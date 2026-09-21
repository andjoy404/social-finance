package household

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

// WargaCSVRow models a parsed single line from the Warga import template.
type WargaCSVRow struct {
	RowNumber          int    `json:"row_number"`
	HouseNumber        string `json:"house_number"`
	HeadName           string `json:"head_name"`
	OccupancyStatus    string `json:"occupancy_status"`
	Address            string `json:"address"`
	ResidentName       string `json:"resident_name"`
	Nik                string `json:"nik"`
	Phone              string `json:"phone"`
	Email              string `json:"email"`
	RelationshipToHead string `json:"relationship_to_head"`
}

// RowError captures an error on a specific row and field.
type RowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ImportPreviewResponse returns machine-readable summary of validation.
type ImportPreviewResponse struct {
	TotalRows   int           `json:"total_rows"`
	ValidRows   int           `json:"valid_rows"`
	InvalidRows int           `json:"invalid_rows"`
	Errors      []RowError    `json:"errors"`
	PreviewData []WargaCSVRow `json:"preview_data"`
}

// ImportCommitResponse returns summary after committing import.
type ImportCommitResponse struct {
	Status             string `json:"status"`
	ImportedHouseholds int    `json:"imported_households"`
	ImportedResidents  int    `json:"imported_residents"`
}

// ParseWargaCSV reads and parses CSV input into WargaCSVRow items.
func ParseWargaCSV(r io.Reader) ([]WargaCSVRow, []RowError, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, []RowError{{Row: 1, Field: "file", Code: "empty_file", Message: "file is empty"}}, nil
		}
		return nil, nil, fmt.Errorf("read csv header: %w", err)
	}

	colIdx := make(map[string]int)
	for i, h := range headers {
		clean := strings.ToLower(strings.TrimSpace(h))
		colIdx[clean] = i
	}

	requiredCols := []string{
		"house_number", "head_name", "occupancy_status", "resident_name",
	}
	var headerErrors []RowError
	for _, req := range requiredCols {
		if _, ok := colIdx[req]; !ok {
			headerErrors = append(headerErrors, RowError{
				Row:     1,
				Field:   req,
				Code:    "missing_header",
				Message: fmt.Sprintf("required header column '%s' is missing", req),
			})
		}
	}
	if len(headerErrors) > 0 {
		return nil, headerErrors, nil
	}

	getVal := func(record []string, cols ...string) string {
		for _, col := range cols {
			if idx, ok := colIdx[col]; ok && idx < len(record) {
				val := strings.TrimSpace(record[idx])
				if val != "" {
					return val
				}
			}
		}
		return ""
	}

	var rows []WargaCSVRow
	var rowErrors []RowError
	rowNum := 1

	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			rowNum++
			rowErrors = append(rowErrors, RowError{
				Row:     rowNum,
				Field:   "row",
				Code:    "csv_parse_error",
				Message: err.Error(),
			})
			continue
		}
		rowNum++

		// Check if empty line
		allEmpty := true
		for _, f := range record {
			if strings.TrimSpace(f) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		rawStatus := getVal(record, "occupancy_status")
		normStatus := strings.ToUpper(rawStatus)
		if normStatus == "PEMILIK" {
			normStatus = "OWNER"
		} else if normStatus == "KONTRAK" || normStatus == "SEWA" {
			normStatus = "TENANT"
		}

		row := WargaCSVRow{
			RowNumber:          rowNum,
			HouseNumber:        getVal(record, "house_number", "nomor_rumah"),
			HeadName:           getVal(record, "head_name", "nama_kepala_keluarga", "nama_warga"),
			OccupancyStatus:    normStatus,
			Address:            getVal(record, "address", "alamat"),
			ResidentName:       getVal(record, "resident_name", "nama_anggota"),
			Nik:                getVal(record, "nik", "resident_nik", "ktp"),
			Phone:              getVal(record, "phone", "resident_phone", "household_phone", "telepon"),
			Email:              getVal(record, "email", "resident_email"),
			RelationshipToHead: getVal(record, "relationship_to_head", "hubungan"),
		}

		// Field validations
		if row.HouseNumber == "" {
			rowErrors = append(rowErrors, RowError{
				Row:     rowNum,
				Field:   "house_number",
				Code:    "required_field",
				Message: "house_number is required",
			})
		}
		if row.HeadName == "" {
			rowErrors = append(rowErrors, RowError{
				Row:     rowNum,
				Field:   "head_name",
				Code:    "required_field",
				Message: "head_name is required",
			})
		}
		if row.OccupancyStatus != "OWNER" && row.OccupancyStatus != "TENANT" {
			rowErrors = append(rowErrors, RowError{
				Row:     rowNum,
				Field:   "occupancy_status",
				Code:    "invalid_occupancy_status",
				Message: "occupancy_status must be OWNER or TENANT",
			})
		}
		if row.ResidentName == "" {
			rowErrors = append(rowErrors, RowError{
				Row:     rowNum,
				Field:   "resident_name",
				Code:    "required_field",
				Message: "resident_name is required",
			})
		}
		if row.Nik != "" {
			if err := ValidateNik(row.Nik); err != nil {
				rowErrors = append(rowErrors, RowError{
					Row:     rowNum,
					Field:   "nik",
					Code:    "invalid_nik",
					Message: err.Error(),
				})
			}
		}
		if row.Phone != "" {
			canonPhone, err := NormalizePhone(row.Phone)
			if err != nil {
				rowErrors = append(rowErrors, RowError{
					Row:     rowNum,
					Field:   "phone",
					Code:    "invalid_phone",
					Message: err.Error(),
				})
			} else {
				row.Phone = canonPhone
			}
		}
		if row.Email != "" {
			if err := ValidateEmail(row.Email); err != nil {
				rowErrors = append(rowErrors, RowError{
					Row:     rowNum,
					Field:   "email",
					Code:    "invalid_email",
					Message: err.Error(),
				})
			} else {
				row.Email = NormalizeEmail(row.Email)
			}
		}

		rows = append(rows, row)
	}

	// Internal cross-row consistency validation
	houseToHead := make(map[string]string)
	houseToStatus := make(map[string]string)
	nikToRow := make(map[string]int)

	for _, r := range rows {
		if r.HouseNumber == "" {
			continue
		}
		hnKey := strings.ToLower(r.HouseNumber)

		if existingHead, exists := houseToHead[hnKey]; exists {
			if !strings.EqualFold(existingHead, r.HeadName) {
				rowErrors = append(rowErrors, RowError{
					Row:     r.RowNumber,
					Field:   "head_name",
					Code:    "conflicting_head_name",
					Message: fmt.Sprintf("house number '%s' has conflicting head names ('%s' and '%s')", r.HouseNumber, existingHead, r.HeadName),
				})
			}
		} else {
			houseToHead[hnKey] = r.HeadName
		}

		if existingStatus, exists := houseToStatus[hnKey]; exists {
			if existingStatus != r.OccupancyStatus {
				rowErrors = append(rowErrors, RowError{
					Row:     r.RowNumber,
					Field:   "occupancy_status",
					Code:    "conflicting_occupancy_status",
					Message: fmt.Sprintf("house number '%s' has conflicting occupancy statuses ('%s' and '%s')", r.HouseNumber, existingStatus, r.OccupancyStatus),
				})
			}
		} else {
			houseToStatus[hnKey] = r.OccupancyStatus
		}

		if r.Nik != "" {
			if prevRow, exists := nikToRow[r.Nik]; exists {
				rowErrors = append(rowErrors, RowError{
					Row:     r.RowNumber,
					Field:   "nik",
					Code:    "duplicate_nik",
					Message: fmt.Sprintf("NIK '%s' is duplicated with row %d", r.Nik, prevRow),
				})
			} else {
				nikToRow[r.Nik] = r.RowNumber
			}
		}
	}

	return rows, rowErrors, nil
}

// ValidateAgainstDatabase checks existing DB records for potential conflicts.
func ValidateAgainstDatabase(ctx context.Context, pool *database.Pool, rtID string, rows []WargaCSVRow, existingErrors []RowError) ([]RowError, error) {
	errorsList := append([]RowError{}, existingErrors...)
	db := pool.Raw()

	for _, r := range rows {
		if r.HouseNumber == "" {
			continue
		}

		// Check if house_number currently occupied by a different household in the same RT
		var currentOccupantHead string
		err := db.QueryRowContext(ctx,
			`SELECT h.head_name
			 FROM physical_houses ph
			 JOIN household_occupancies ho ON ho.physical_house_id = ph.id AND ho.end_date IS NULL
			 JOIN households h ON ho.household_id = h.id AND h.is_active = true
			 WHERE ph.rt_id = $1 AND ph.house_number = $2 AND ph.is_active = true
			 LIMIT 1`,
			rtID, r.HouseNumber,
		).Scan(&currentOccupantHead)
		if err == nil {
			if !strings.EqualFold(currentOccupantHead, r.HeadName) {
				errorsList = append(errorsList, RowError{
					Row:     r.RowNumber,
					Field:   "house_number",
					Code:    "house_already_occupied",
					Message: fmt.Sprintf("house number '%s' is currently occupied by household head '%s'; temporal move required", r.HouseNumber, currentOccupantHead),
				})
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("check house occupancy: %w", err)
		}

		// Check if NIK already exists in residents in the same RT
		if r.Nik != "" {
			var existingResidentName string
			err = db.QueryRowContext(ctx,
				`SELECT r.full_name
				 FROM residents r
				 WHERE r.rt_id = $1 AND r.nik = $2 AND r.is_active = true
				 LIMIT 1`,
				rtID, r.Nik,
			).Scan(&existingResidentName)
			if err == nil {
				if !strings.EqualFold(existingResidentName, r.ResidentName) {
					errorsList = append(errorsList, RowError{
						Row:     r.RowNumber,
						Field:   "nik",
						Code:    "duplicate_nik",
						Message: fmt.Sprintf("NIK '%s' is already registered to resident '%s'", r.Nik, existingResidentName),
					})
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("check resident NIK: %w", err)
			}
		}
	}

	return errorsList, nil
}

// CommitWargaImport processes and inserts valid rows into the database atomically.
func CommitWargaImport(ctx context.Context, pool *database.Pool, rtID, userID string, rows []WargaCSVRow) (*ImportCommitResponse, error) {
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Group rows by HouseNumber
	type householdGroup struct {
		HouseNumber     string
		HeadName        string
		OccupancyStatus string
		Address         string
		Residents       []WargaCSVRow
	}

	groups := make(map[string]*householdGroup)
	var groupOrder []string
	for _, r := range rows {
		hnKey := strings.ToLower(r.HouseNumber)
		grp, ok := groups[hnKey]
		if !ok {
			grp = &householdGroup{
				HouseNumber:     r.HouseNumber,
				HeadName:        r.HeadName,
				OccupancyStatus: r.OccupancyStatus,
				Address:         r.Address,
			}
			groups[hnKey] = grp
			groupOrder = append(groupOrder, hnKey)
		}
		grp.Residents = append(grp.Residents, r)
	}

	importedHouseholds := 0
	importedResidents := 0

	for _, hnKey := range groupOrder {
		grp := groups[hnKey]

		// A. Find or create Physical House
		var houseID string
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM physical_houses WHERE rt_id = $1 AND house_number = $2 AND is_active = true LIMIT 1`,
			rtID, grp.HouseNumber,
		).Scan(&houseID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				var addr *string
				if grp.Address != "" {
					addr = &grp.Address
				}
				err = tx.QueryRowContext(ctx,
					`INSERT INTO physical_houses (rt_id, house_number, address, is_active)
					 VALUES ($1, $2, $3, true) RETURNING id`,
					rtID, grp.HouseNumber, addr,
				).Scan(&houseID)
				if err != nil {
					return nil, fmt.Errorf("create physical house %s: %w", grp.HouseNumber, err)
				}
			} else {
				return nil, fmt.Errorf("query physical house %s: %w", grp.HouseNumber, err)
			}
		}

		// B. Find or create Household
		var householdID string
		var hoID string
		err = tx.QueryRowContext(ctx,
			`SELECT h.id, ho.id
			 FROM households h
			 JOIN household_occupancies ho ON ho.household_id = h.id AND ho.end_date IS NULL
			 WHERE ho.physical_house_id = $1 AND h.rt_id = $2 AND h.is_active = true LIMIT 1`,
			houseID, rtID,
		).Scan(&householdID, &hoID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				err = tx.QueryRowContext(ctx,
					`INSERT INTO households (rt_id, head_name, is_active)
					 VALUES ($1, $2, true) RETURNING id`,
					rtID, grp.HeadName,
				).Scan(&householdID)
				if err != nil {
					return nil, fmt.Errorf("create household %s: %w", grp.HouseNumber, err)
				}
				importedHouseholds++

				// Create current occupancy starting today
				err = tx.QueryRowContext(ctx,
					`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
					 VALUES ($1, $2, $3, CURRENT_DATE) RETURNING id`,
					householdID, houseID, grp.OccupancyStatus,
				).Scan(&hoID)
				if err != nil {
					return nil, fmt.Errorf("create household occupancy for %s: %w", grp.HouseNumber, err)
				}
			} else {
				return nil, fmt.Errorf("query household occupancy for %s: %w", grp.HouseNumber, err)
			}
		}

		// C. Create residents and current residency periods
		for _, res := range grp.Residents {
			var existingResID string
			err := tx.QueryRowContext(ctx,
				`SELECT r.id
				 FROM residents r
				 JOIN residency_periods rp ON rp.resident_id = r.id AND rp.household_occupancy_id = $1 AND rp.end_date IS NULL
				 WHERE r.rt_id = $2 AND (r.full_name = $3 OR ($4 <> '' AND r.nik IS NOT NULL AND r.nik = $4))
				 LIMIT 1`,
				hoID, rtID, res.ResidentName, res.Nik,
			).Scan(&existingResID)
			if err == nil {
				// Already exists in current occupancy, skip
				continue
			}

			var nikPtr, phonePtr, emailPtr *string
			if res.Nik != "" {
				nikPtr = &res.Nik
			}
			if res.Phone != "" {
				phonePtr = &res.Phone
			}
			if res.Email != "" {
				emailPtr = &res.Email
			}

			var resID string
			err = tx.QueryRowContext(ctx,
				`INSERT INTO residents (rt_id, full_name, phone, nik, email, is_active)
				 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
				rtID, res.ResidentName, phonePtr, nikPtr, emailPtr,
			).Scan(&resID)
			if err != nil {
				return nil, fmt.Errorf("insert resident %s: %w", res.ResidentName, err)
			}

			rel := res.RelationshipToHead
			if rel == "" && strings.EqualFold(res.ResidentName, grp.HeadName) {
				rel = RelationshipHead
			}
			var relPtr *string
			if rel != "" {
				relPtr = &rel
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date)
				 VALUES ($1, $2, $3, CURRENT_DATE)`,
				resID, hoID, relPtr,
			)
			if err != nil {
				return nil, fmt.Errorf("insert residency period for %s: %w", res.ResidentName, err)
			}
			importedResidents++
		}
	}

	// Record audit log
	auditData, _ := json.Marshal(map[string]any{
		"imported_households": importedHouseholds,
		"imported_residents":  importedResidents,
		"total_rows":          len(rows),
	})
	var validUser *string
	if userID != "" {
		var exists bool
		_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id::text = $1)`, userID).Scan(&exists)
		if exists {
			validUser = &userID
		}
	}
	_, _ = tx.ExecContext(ctx,
		`INSERT INTO audit_logs (rt_id, user_id, action, entity_type, entity_id, new_values)
		 VALUES ($1, $2, 'warga_import', 'household', gen_random_uuid()::text, $3)`,
		rtID, validUser, auditData,
	)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit warga import: %w", err)
	}

	return &ImportCommitResponse{
		Status:             "committed",
		ImportedHouseholds: importedHouseholds,
		ImportedResidents:  importedResidents,
	}, nil
}

// ExportWargaCSV writes out all active residents and their household info to CSV format.
func ExportWargaCSV(ctx context.Context, pool *database.Pool, rtID string, w io.Writer) error {
	db := pool.Raw()

	rows, err := db.QueryContext(ctx,
		`SELECT
		   COALESCE(ph.house_number, '') as house_number,
		   h.head_name,
		   COALESCE(ho.occupancy_status, 'OWNER') as occupancy_status,
		   COALESCE(ph.address, '') as address,
		   r.full_name as resident_name,
		   COALESCE(r.nik, '') as nik,
		   COALESCE(r.phone, '') as phone,
		   COALESCE(r.email, '') as email,
		   COALESCE(rp.relationship_to_head, '') as relationship_to_head
		 FROM residents r
		 JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
		 JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id AND ho.end_date IS NULL
		 JOIN households h ON ho.household_id = h.id AND h.is_active = true
		 JOIN physical_houses ph ON ho.physical_house_id = ph.id AND ph.is_active = true
		 WHERE r.rt_id = $1 AND r.is_active = true
		 ORDER BY ph.house_number, r.full_name`,
		rtID,
	)
	if err != nil {
		return fmt.Errorf("query warga export: %w", err)
	}
	defer rows.Close()

	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{
		"house_number", "head_name", "occupancy_status",
		"address", "resident_name", "nik", "phone",
		"email", "relationship_to_head",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for rows.Next() {
		var houseNum, headName, occStatus, addr, resName, nik, phone, email, rel string
		if err := rows.Scan(
			&houseNum, &headName, &occStatus, &addr,
			&resName, &nik, &phone, &email, &rel,
		); err != nil {
			return fmt.Errorf("scan export row: %w", err)
		}

		record := []string{
			houseNum, headName, occStatus, addr,
			resName, nik, phone, email, rel,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	return rows.Err()
}

// --- HTTP Handlers for Warga Import/Export ---

// HandleWargaExport exports tenant-scoped warga data as CSV.
func HandleWargaExport(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid or missing tenant context")
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="warga_export.csv"`)
		w.WriteHeader(http.StatusOK)

		if err := ExportWargaCSV(r.Context(), pool, ac.RTID, w); err != nil {
			// Headers already sent, log error
			return
		}
	}
}

// readCSVBody reads the CSV data from multipart form file or raw body.
func readCSVBody(r *http.Request) (io.Reader, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(10 << 20) // 10MB limit
		if err != nil {
			return nil, fmt.Errorf("parse multipart form: %w", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, errors.New("multipart field 'file' is required")
		}
		return file, nil
	}

	// Plain text or CSV in raw body
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(bodyBytes) == 0 {
		return nil, errors.New("request body is empty")
	}
	return bytes.NewReader(bodyBytes), nil
}

// HandleWargaImportPreview parses and validates CSV input without committing to DB.
func HandleWargaImportPreview(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid or missing tenant context")
			return
		}

		reader, err := readCSVBody(r)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		rows, parseErrors, err := ParseWargaCSV(reader)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "csv_parse_error", err.Error())
			return
		}

		allErrors, err := ValidateAgainstDatabase(r.Context(), pool, ac.RTID, rows, parseErrors)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		invalidCount := 0
		rowWithErrors := make(map[int]bool)
		for _, e := range allErrors {
			rowWithErrors[e.Row] = true
		}
		invalidCount = len(rowWithErrors)
		validCount := len(rows) - invalidCount
		if validCount < 0 {
			validCount = 0
		}

		resp := ImportPreviewResponse{
			TotalRows:   len(rows),
			ValidRows:   validCount,
			InvalidRows: invalidCount,
			Errors:      allErrors,
			PreviewData: rows,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// HandleWargaImportCommit parses, validates, and commits CSV data to DB in a transaction.
func HandleWargaImportCommit(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid or missing tenant context")
			return
		}

		reader, err := readCSVBody(r)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		rows, parseErrors, err := ParseWargaCSV(reader)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "csv_parse_error", err.Error())
			return
		}

		allErrors, err := ValidateAgainstDatabase(r.Context(), pool, ac.RTID, rows, parseErrors)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		if len(allErrors) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "validation_error",
					"message": "CSV data contains validation errors",
					"details": allErrors,
				},
			})
			return
		}

		result, err := CommitWargaImport(r.Context(), pool, ac.RTID, ac.UserID, rows)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "import_failed", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(result)
	}
}
