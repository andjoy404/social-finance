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
	"time"

	"github.com/google/uuid"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

// WargaCSVRow models a parsed single line from the Warga import template.
type WargaCSVRow struct {
	RowNumber      int    `json:"row_number"`
	HouseNumber    string `json:"house_number"`
	HeadName       string `json:"head_name"`
	OccupancyStatus string `json:"occupancy_status"`
	Address        string `json:"address"`
	ResidentName   string `json:"resident_name"`
	Nik            string `json:"nik"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	MoveInDate     string `json:"move_in_date"`
	Jabatan        string `json:"jabatan"`
	Password       string `json:"password"`
}

// WargaPreviewRow is the safe, sanitized preview row returned to the client.
// It is a strict subset of WargaCSVRow with the Password field omitted.
type WargaPreviewRow struct {
	RowNumber      int    `json:"row_number"`
	HouseNumber    string `json:"house_number"`
	HeadName       string `json:"head_name"`
	OccupancyStatus string `json:"occupancy_status"`
	Address        string `json:"address"`
	ResidentName   string `json:"resident_name"`
	Nik            string `json:"nik"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	MoveInDate     string `json:"move_in_date"`
	Jabatan        string `json:"jabatan"`
	// Password field intentionally omitted to prevent password leak in preview responses.
	PasswordHint string `json:"password_hint,omitempty"`
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
	TotalRows   int              `json:"total_rows"`
	ValidRows   int              `json:"valid_rows"`
	InvalidRows int              `json:"invalid_rows"`
	Errors      []RowError       `json:"errors"`
	PreviewData []WargaPreviewRow `json:"preview_data"`
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
			HeadName:           getVal(record, "head_name", "nama_warga", "nama_penanggung_jawab"),
			OccupancyStatus:    normStatus,
			Address:            getVal(record, "address", "alamat"),
			ResidentName:       getVal(record, "resident_name", "nama_anggota"),
			Nik:                getVal(record, "nik", "resident_nik", "ktp"),
			Phone:              getVal(record, "phone", "resident_phone", "household_phone", "telepon"),
			Email:              getVal(record, "email", "resident_email"),
			MoveInDate:         getVal(record, "move_in_date", "tanggal_mulai", "move_in"),
			Jabatan:            getVal(record, "jabatan", "position", "jabatan_warga"),
			Password:           getVal(record, "password", "kata_sandi"),
		}

		// Normalize "kosong" (Indonesian for "empty") to empty string for optional
		// fields. This allows import files to explicitly mark a field as "no value"
		// without being rejected as an invalid jabatan or too-short password.
		if strings.EqualFold(row.Jabatan, "kosong") {
			row.Jabatan = ""
		}
		if strings.EqualFold(row.Password, "kosong") {
			row.Password = ""
		}
		if strings.EqualFold(row.MoveInDate, "kosong") {
			row.MoveInDate = ""
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

		// Validate move_in_date (optional, YYYY-MM-DD format)
		if row.MoveInDate != "" {
			if _, err := parseMoveInDate(row.MoveInDate); err != nil {
				rowErrors = append(rowErrors, RowError{
					Row:     rowNum,
					Field:   "move_in_date",
					Code:    "invalid_date_format",
					Message: "move_in_date must be in YYYY-MM-DD format",
				})
			} else {
				row.MoveInDate = normalizeDate(row.MoveInDate)
			}
		}

		// Validate jabatan (optional, must be a recognized position)
		if row.Jabatan != "" {
			if err := validateJabatan(row.Jabatan); err != nil {
				rowErrors = append(rowErrors, RowError{
					Row:     rowNum,
					Field:   "jabatan",
					Code:    "invalid_jabatan",
					Message: err.Error(),
				})
			} else {
				row.Jabatan = normalizeJabatan(row.Jabatan)
			}
		}

		// Validate password (optional, non-empty passwords must be at least 8 chars)
		if row.Password != "" {
			if len(strings.TrimSpace(row.Password)) < 8 {
				rowErrors = append(rowErrors, RowError{
					Row:     rowNum,
					Field:   "password",
					Code:    "password_too_short",
					Message: "password must be at least 8 characters",
				})
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

	// Build a set of house numbers that are already validated within this import
	// (from the same file) so that cross-row conflicts are caught but self-reference
	// is allowed.
	houseNumbersInFile := make(map[string]bool)

	for _, r := range rows {
		if r.HouseNumber == "" {
			continue
		}

		// Cross-row consistency: same house number in the file must have the same
		// head_name. This catches file-level data issues early.
		hnKey := strings.ToLower(r.HouseNumber)
		if already, exists := houseNumbersInFile[hnKey]; exists {
			if !already {
				// Already flagged as conflicting in internal validation
				continue
			}
		}
		houseNumbersInFile[hnKey] = true

		// Check if house_number is currently occupied by a different household in the same RT.
		// We need a more robust matching strategy to support re-imports:
		// 1. If NIK is provided and matches a resident in the currently occupied household → same household, no error.
		// 2. If head_name matches (case-insensitive) → same household, no error.
		// 3. Otherwise → potential conflict, flag it.
		var currentOccupantHead string
		var currentOccupantHouseholdID string
		err := db.QueryRowContext(ctx,
			`SELECT h.id, h.head_name
			 FROM physical_houses ph
			 JOIN household_occupancies ho ON ho.physical_house_id = ph.id AND ho.end_date IS NULL
			 JOIN households h ON ho.household_id = h.id AND h.is_active = true
			 WHERE ph.rt_id = $1 AND ph.house_number = $2 AND ph.is_active = true
			 LIMIT 1`,
			rtID, r.HouseNumber,
		).Scan(&currentOccupantHouseholdID, &currentOccupantHead)
		if err == nil {
			// Check if this import row's head_name matches the current occupant (case-insensitive).
			// If so, it's a re-import of the same household.
			headNameMatch := strings.EqualFold(currentOccupantHead, r.HeadName)

			// If head_name doesn't match, check if any NIK in the same import file
			// matches a resident in the current household. This handles cases where
			// the head_name might have been typo'd or abbreviated differently.
			nikMatch := false
			if !headNameMatch && r.Nik != "" {
				err = db.QueryRowContext(ctx,
					`SELECT COUNT(*) > 0
					 FROM residents re
					 JOIN residency_periods rpp ON rpp.resident_id = re.id AND rpp.end_date IS NULL
					 WHERE re.rt_id = $1 AND re.nik = $2 AND rpp.household_occupancy_id = (
					 	SELECT ho2.id
					 	 FROM household_occupancies ho2
					 	 WHERE ho2.household_id = $3 AND ho2.end_date IS NULL
					 	 LIMIT 1
					 )`,
					rtID, r.Nik, currentOccupantHouseholdID,
				).Scan(&nikMatch)
				if err != nil {
					// If we can't check NIK, fall through to head_name-based logic
					nikMatch = false
				}
			}

			if !headNameMatch && !nikMatch {
				// Neither head_name nor NIK matches — this is a genuine conflict
				// indicating a different household occupying the same house.
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

		// Check if NIK already exists in residents in the same RT.
		// A resident with the same NIK and same name is considered the same person
		// and is a no-op for import. Different name = possible re-import with name
		// format change, OR a duplicate identity conflict. We distinguish these:
		//   - NIK exists in SAME house_number → same household being re-imported
		//     (name format may differ), so allow.
		//   - NIK exists in DIFFERENT house_number → duplicate identity conflict,
		//     reject.
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
				if strings.EqualFold(existingResidentName, r.ResidentName) {
					// Same NIK + same name → identical resident, allow (no-op).
				} else {
					// Names differ — check if this NIK already belongs to a resident
					// in the SAME house_number. If yes, it's a re-import (name format
					// changed) and should be allowed. If no, it's a duplicate identity.
					var isInSameHouse bool
					err = db.QueryRowContext(ctx,
						`SELECT EXISTS(
							SELECT 1 FROM residents re
							JOIN residency_periods rp ON rp.resident_id = re.id AND rp.end_date IS NULL
							JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id AND ho.end_date IS NULL
							JOIN physical_houses ph ON ph.id = ho.physical_house_id AND ph.is_active = true
							WHERE re.rt_id = $1 AND re.nik = $2 AND ph.house_number = $3
						)`,
						rtID, r.Nik, r.HouseNumber,
					).Scan(&isInSameHouse)
					if err != nil && !errors.Is(err, sql.ErrNoRows) {
						return nil, fmt.Errorf("check NIK house: %w", err)
					}
					if !isInSameHouse {
						errorsList = append(errorsList, RowError{
							Row:     r.RowNumber,
							Field:   "nik",
							Code:    "duplicate_nik",
							Message: fmt.Sprintf("NIK '%s' is already registered to resident '%s'", r.Nik, existingResidentName),
						})
					}
					// If isInSameHouse: allow (re-import with name change in same house).
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("check resident NIK: %w", err)
			}
		}
	}

	// Check for email duplicate within the import file itself (internal conflict).
	// Cross-user email conflicts are handled by the DB unique index on
	// lower(trim(email)) — if the import changes a user's email to one that
	// another user already owns, the DB will reject it as a unique violation.
	// We only pre-check for rows within the same import file requesting the
	// same email (which would unambiguously be a conflict).
	emailOwner := make(map[string]string) // normalizedEmail → row number
	for _, r := range rows {
		if r.Email == "" {
			continue
		}

		normalizedEmail := NormalizeEmail(r.Email)
		if normalizedEmail == "" {
			continue
		}

		if existingRow, exists := emailOwner[normalizedEmail]; exists {
			errorsList = append(errorsList, RowError{
				Row:     r.RowNumber,
				Field:   "email",
				Code:    "duplicate_email",
				Message: fmt.Sprintf("email '%s' is also used on row %s", normalizedEmail, existingRow),
			})
		} else {
			emailOwner[normalizedEmail] = fmt.Sprintf("%d", r.RowNumber)
		}
	}

	// For rows that will create a user (have both jabatan and email),
	// check if the email is already taken by a user not linked to any
	// resident in this import file.
	for _, r := range rows {
		if r.Email == "" || r.Jabatan == "" {
			continue
		}
		normalizedEmail := NormalizeEmail(r.Email)

		var owningUserID string
		if err := db.QueryRowContext(ctx,
			`SELECT u.id FROM users u WHERE lower(trim(u.email)) = $1`,
			normalizedEmail,
		).Scan(&owningUserID); err != nil || owningUserID == "" {
			continue // email not taken by any user
		}

		// Email is taken. Determine whether to flag it as a conflict:
		// 1. If the owner has a linked resident in the import RT →
		//    check if it's the same person (same NIK). Same person
		//    = re-import (allow). Different person = email stealing (reject).
		// 2. If the owner has linked residents but ONLY in other RTs →
		//    allow (different RT; DB unique constraint handles cross-RT
		//    conflicts at commit time if they ever arise).
		// 3. If the owner has NO linked resident at all (standalone
		//    user) → reject because email is globally unique and already
		//    taken by that user.
		var hasLinkedInImportRT, hasLinkedAnyRT bool
		row := db.QueryRowContext(ctx,
			`SELECT
				EXISTS(SELECT 1 FROM residents re WHERE re.user_id = $1 AND re.rt_id = $2),
				EXISTS(SELECT 1 FROM residents re WHERE re.user_id = $1)`,
			owningUserID, rtID,
		)
		if err := row.Scan(&hasLinkedInImportRT, &hasLinkedAnyRT); err != nil {
			continue // can't determine — allow
		}

		if hasLinkedInImportRT {
			// Owner has a linked resident in this RT. Check if it's
			// the same person (re-import) or a different person.
			var isSelf bool
			for _, other := range rows {
				if other.Nik == "" {
					continue
				}
				var resUserID sql.NullString
				if err := db.QueryRowContext(ctx,
					`SELECT user_id FROM residents WHERE rt_id = $1 AND nik = $2`,
					rtID, other.Nik,
				).Scan(&resUserID); err == nil && resUserID.Valid && resUserID.String == owningUserID {
					isSelf = true
					break
				}
			}

			if !isSelf {
				errorsList = append(errorsList, RowError{
					Row:     r.RowNumber,
					Field:   "email",
					Code:    "duplicate_email",
					Message: fmt.Sprintf("email '%s' is already used by another user", normalizedEmail),
				})
			}
		} else {
			// Email is taken by a user (with or without linked residents in any RT).
			// Email is globally unique — reject regardless of which RT owns it.
			errorsList = append(errorsList, RowError{
				Row:     r.RowNumber,
				Field:   "email",
				Code:    "duplicate_email",
				Message: fmt.Sprintf("email '%s' is already used by another user", normalizedEmail),
			})
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

	// Unique import suffix to ensure user emails are unique per import run
	importSuffix := uuid.NewString()

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

		// Try to find existing household by physical house + active occupancy.
		err = tx.QueryRowContext(ctx,
			`SELECT h.id, ho.id
			 FROM households h
			 JOIN household_occupancies ho ON ho.household_id = h.id AND ho.end_date IS NULL
			 WHERE ho.physical_house_id = $1 AND h.rt_id = $2 AND h.is_active = true LIMIT 1`,
			houseID, rtID,
		).Scan(&householdID, &hoID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// First, check if this house_number had a valid row (passed validation)
				// which means no conflicting household is occupying it.
				// However, we should still try to match by head_name to support
				// re-imports where the physical_house exists but household was not found
				// (unlikely edge case, but safety check).
				//
				// Also try matching by NIK: if any resident in this import group has a NIK
				// that matches a resident in a different household at a different house,
				// it's still a new household at this house.
				//
				// If no match found, create new household.
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

		// C. Process residents — create new or partially update existing
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
				// Existing resident in current occupancy — perform partial update per column
				err = processExistingResident(ctx, tx, rtID, hoID, existingResID, res, importSuffix)
				if err != nil {
					return nil, fmt.Errorf("update resident %s: %w", res.ResidentName, err)
				}
				continue
			}

			// New resident — full creation flow
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

			// Determine start_date for residency: use move_in_date if provided, else CURRENT_DATE
			_, err = tx.ExecContext(ctx,
				`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
				 VALUES ($1, $2, COALESCE(NULLIF($3, '')::date, CURRENT_DATE))`,
				resID, hoID, res.MoveInDate,
			)
			if err != nil {
				return nil, fmt.Errorf("insert residency period for %s: %w", res.ResidentName, err)
			}
			importedResidents++

			// Handle user and membership: create user if jabatan or password provided
			if res.Jabatan != "" || res.Password != "" {
				var pwHash *string
				if res.Password != "" {
					h, err := auth.Hash(res.Password)
					if err != nil {
						return nil, fmt.Errorf("hash password for %s: %w", res.ResidentName, err)
					}
					pwHash = &h
				}
				err = createResidentUserAndMembership(ctx, tx, resID, rtID, res.ResidentName, res.Email, pwHash, res.Jabatan, importSuffix)
				if err != nil {
					return nil, fmt.Errorf("create user+membership for %s: %w", res.ResidentName, err)
				}
			}

			triggerSyncBills(ctx, tx, rtID, hoID)
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

// processExistingResident handles partial updates for a resident that already exists.
// Each column is handled independently: empty means "keep existing value".
func processExistingResident(
	ctx context.Context,
	tx *sql.Tx,
	rtID, hoID, resID string,
	res WargaCSVRow,
	importSuffix string,
) error {
	// --- 1. Move-in date: update current residency period start_date if provided ---
	if res.MoveInDate != "" {
		_, err := tx.ExecContext(ctx,
			`UPDATE residency_periods
			 SET start_date = $1, updated_at = now()
			 WHERE resident_id = $2 AND household_occupancy_id = $3 AND end_date IS NULL`,
			res.MoveInDate, resID, hoID,
		)
		if err != nil {
			return fmt.Errorf("update residency start_date for %s: %w", res.ResidentName, err)
		}
		// Trigger bill recalculation since start_date has changed
		triggerSyncBills(ctx, tx, rtID, hoID)
	}

	// --- 2. Jabatan: upsert membership if provided, do nothing if empty ---
	if res.Jabatan != "" {
		var userID sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT user_id FROM residents WHERE id = $1`, resID,
		).Scan(&userID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query user_id for resident %s: %w", res.ResidentName, err)
		}

		if userID.Valid {
			// User exists — upsert membership jabatan
			// Since idx_memberships_user_rt_active is a partial index (not a constraint),
			// we use INSERT + UPDATE pattern instead of ON CONFLICT.
			_, err = tx.ExecContext(ctx,
				`INSERT INTO user_rt_memberships (user_id, rt_id, jabatan, is_active)
				 SELECT $1, $2, $3, true
				 WHERE NOT EXISTS (
				   SELECT 1 FROM user_rt_memberships
				   WHERE user_id = $1 AND rt_id = $2 AND is_active = true
				 )`,
				userID.String, rtID, res.Jabatan,
			)
			if err != nil {
				return fmt.Errorf("insert membership jabatan for %s: %w", res.ResidentName, err)
			}
			// Always update jabatan (insert or update)
			_, err = tx.ExecContext(ctx,
				`UPDATE user_rt_memberships
				 SET jabatan = $3, updated_at = now()
				 WHERE user_id = $1 AND rt_id = $2 AND is_active = true`,
				userID.String, rtID, res.Jabatan,
			)
			if err != nil {
				return fmt.Errorf("update membership jabatan for %s: %w", res.ResidentName, err)
			}
		} else if res.Password != "" {
			// No existing user — create user with password and membership
			pwHash, err := auth.Hash(res.Password)
			if err != nil {
				return fmt.Errorf("hash password for %s: %w", res.ResidentName, err)
			}

			var newUserID string
			err = tx.QueryRowContext(ctx,
				`INSERT INTO users (email, full_name, password_hash, system_role, created_at, updated_at)
				 VALUES ($1, $2, $3, NULL, now(), now()) RETURNING id`,
				fmt.Sprintf("%s-%s@import.local", strings.ToLower(res.ResidentName), importSuffix), res.ResidentName, pwHash,
			).Scan(&newUserID)
			if err != nil {
				return fmt.Errorf("create user for %s: %w", res.ResidentName, err)
			}

			// Link resident to new user
			_, err = tx.ExecContext(ctx,
				`UPDATE residents SET user_id = $1 WHERE id = $2`,
				newUserID, resID,
			)
			if err != nil {
				return fmt.Errorf("link user to resident %s: %w", res.ResidentName, err)
			}

			// Create RT membership with jabatan
			_, err = tx.ExecContext(ctx,
				`INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
				 VALUES ($1, $2, 'warga', $3, true)`,
				newUserID, rtID, res.Jabatan,
			)
			if err != nil {
				return fmt.Errorf("create RT membership for %s: %w", res.ResidentName, err)
			}
		}
	} else if res.Password != "" {
		// Password provided but no jabatan
		var existingUID sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT user_id FROM residents WHERE id = $1`, resID).Scan(&existingUID)
		if err == nil && existingUID.Valid {
			// User exists — update password
			err = updateResidentPassword(ctx, tx, resID, res.Password)
			if err != nil {
				return fmt.Errorf("update password for %s: %w", res.ResidentName, err)
			}
		} else {
			// Resident has no linked user — create user with password and link
			pwHash, err := auth.Hash(res.Password)
			if err != nil {
				return fmt.Errorf("hash password for %s: %w", res.ResidentName, err)
			}
			userEmail := fmt.Sprintf("%s-%s@import.local", strings.ToLower(res.ResidentName), importSuffix)
			if res.Email != "" {
				userEmail = NormalizeEmail(res.Email)
			}
			var newUserID string
			err = tx.QueryRowContext(ctx,
				`INSERT INTO users (email, full_name, password_hash, system_role, created_at, updated_at)
				 VALUES ($1, $2, $3, NULL, now(), now()) RETURNING id`,
				userEmail, res.ResidentName, pwHash,
			).Scan(&newUserID)
			if err != nil {
				return fmt.Errorf("create user for %s: %w", res.ResidentName, err)
			}
			_, err = tx.ExecContext(ctx, `UPDATE residents SET user_id = $1 WHERE id = $2`, newUserID, resID)
			if err != nil {
				return fmt.Errorf("link user to resident %s: %w", res.ResidentName, err)
			}
			_, err = tx.ExecContext(ctx,
				`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
				 VALUES ($1, $2, 'warga', true)`,
				newUserID, rtID,
			)
			if err != nil {
				return fmt.Errorf("create RT membership for %s: %w", res.ResidentName, err)
			}
		}
	}

	// --- 3. Email: update users.email for existing resident if provided ---
	if res.Email != "" {
		normalizedEmail := NormalizeEmail(res.Email)

		var userID sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT user_id FROM residents WHERE id = $1`, resID,
		).Scan(&userID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query user_id for resident %s: %w", res.ResidentName, err)
		}

		if userID.Valid {
			// Check if the target email is already in use by ANOTHER user.
			// If it's owned by this same user, it's a no-op. If owned by another
			// user, return an error — email is globally unique.
			var owningUserID string
			err = tx.QueryRowContext(ctx,
				`SELECT u.id FROM users u WHERE lower(trim(u.email)) = $1`,
				normalizedEmail,
			).Scan(&owningUserID)
			if err == nil && owningUserID != "" && owningUserID != userID.String {
				// Email already belongs to a different user — return error.
				return fmt.Errorf(
					"email '%s' is already used by another user; cannot update resident %s",
					normalizedEmail, res.ResidentName,
				)
			} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("check email existence for resident %s: %w", res.ResidentName, err)
			}
			// Safe to update — either email is free or belongs to this user.
			_, err = tx.ExecContext(ctx,
				`UPDATE users
				 SET email = $1, updated_at = now()
				 WHERE id = $2`,
				normalizedEmail, userID.String,
			)
			if err != nil {
				return fmt.Errorf("update email for resident %s: %w", res.ResidentName, err)
			}
		} else {
			// No linked user for this resident — cannot update email.
			return fmt.Errorf("resident %s has no user account; email update requires an existing linked user", res.ResidentName)
		}
	}

	return nil
}

// createResidentUserAndMembership creates a new user account and RT membership for a new resident.
func createResidentUserAndMembership(
	ctx context.Context,
	tx *sql.Tx,
	resID, rtID, residentName, email string,
	pwHash *string, // nil if no password
	jabatan string,
	importSuffix string,
) error {
	var newUserID string
	var err error
	var userEmail string

	// Determine email: check if provided email already exists globally.
	normalizedEmail := NormalizeEmail(email)
	if normalizedEmail != "" {
		// Email must be globally unique. If already taken by any user,
		// return an error — do NOT fall back to a synthetic @import.local address.
		var exists bool
		err = tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM users WHERE lower(trim(email)) = $1)`,
			normalizedEmail,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check email existence for %s: %w", residentName, err)
		}
		if exists {
			return fmt.Errorf(
				"email '%s' is already used by another user; cannot create account for %s",
				normalizedEmail, residentName,
			)
		}
		userEmail = normalizedEmail
	}

	actualHash := ""
	if pwHash != nil {
		actualHash = *pwHash
	} else {
		h, err := auth.Hash(uuid.NewString())
		if err != nil {
			return fmt.Errorf("hash initial password for %s: %w", residentName, err)
		}
		actualHash = h
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (email, full_name, password_hash, system_role, created_at, updated_at)
		 VALUES ($1, $2, $3, NULL, now(), now()) RETURNING id`,
		userEmail, residentName, actualHash,
	).Scan(&newUserID)
	if err != nil {
		return fmt.Errorf("create user for %s: %w", residentName, err)
	}

	// Link resident to new user
	_, err = tx.ExecContext(ctx,
		`UPDATE residents SET user_id = $1 WHERE id = $2`,
		newUserID, resID,
	)
	if err != nil {
		return fmt.Errorf("link user to resident %s: %w", residentName, err)
	}

	// Create RT membership with jabatan (null if empty)
	var jabVal any
	if strings.TrimSpace(jabatan) != "" {
		jabVal = strings.TrimSpace(jabatan)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		 VALUES ($1, $2, 'warga', $3, true)`,
		newUserID, rtID, jabVal,
	)
	if err != nil {
		return fmt.Errorf("create RT membership for %s: %w", residentName, err)
	}

	return nil
}

// createUserInTx inserts a new user into the users table within a transaction.
// pwHash is nil for users without passwords (imported without password).
func createUserInTx(
	ctx context.Context,
	tx *sql.Tx,
	email, fullName string,
	pwHash *string,
) error {
	actualHash := ""
	if pwHash != nil {
		actualHash = *pwHash
	} else {
		h, err := auth.Hash(uuid.NewString())
		if err != nil {
			return err
		}
		actualHash = h
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO users (email, full_name, password_hash, system_role, created_at, updated_at)
		 VALUES ($1, $2, $3, NULL, now(), now())`,
		email, fullName, actualHash,
	)
	return err
}

// updateResidentPassword hashes and updates the password for an existing resident's user account.
func updateResidentPassword(ctx context.Context, tx *sql.Tx, resID, password string) error {
	var userID sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT user_id FROM residents WHERE id = $1`, resID,
	).Scan(&userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query user_id for resident %s: %w", resID, err)
	}
	if !userID.Valid {
		// No user account linked — skip password update silently
		return nil
	}

	pwHash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password for resident %s: %w", resID, err)
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now()
		 WHERE id = $2`,
		pwHash, userID.String,
	)
	if err != nil {
		return fmt.Errorf("update password for resident %s: %w", resID, err)
	}
	return nil
}

// ExportWargaCSV writes out all active residents and their household info to CSV format.
func ExportWargaCSV(ctx context.Context, pool *database.Pool, rtID string, w io.Writer, acOpt ...*auth.AuthContext) error {
	var ac *auth.AuthContext
	if len(acOpt) > 0 {
		ac = acOpt[0]
	}
	db := pool.Raw()

	var rows *sql.Rows
	var err error
	rows, err = db.QueryContext(ctx,
		`SELECT
		   COALESCE(ph.house_number, '') as house_number,
		   h.head_name,
		   COALESCE(ho.occupancy_status, 'OWNER') as occupancy_status,
		   COALESCE(ph.address, '') as address,
		   r.full_name as resident_name,
		   COALESCE(r.nik, '') as nik,
		   COALESCE(r.phone, '') as phone,
		   COALESCE(r.email, '') as email,
		   COALESCE(rp.start_date::text, '') as move_in_date,
		   COALESCE(urm.jabatan, '') as jabatan,
		   '' as password
		 FROM residents r
		 JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
		 LEFT JOIN user_rt_memberships urm ON urm.user_id = r.user_id
		   AND urm.rt_id = $1 AND urm.is_active = true AND urm.jabatan IS NOT NULL
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
		"email", "move_in_date", "jabatan", "password",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for rows.Next() {
		var houseNum, headName, occStatus, addr, resName, nik, phone, email string
		var moveInDate, jabatan, password string
		if err := rows.Scan(
			&houseNum, &headName, &occStatus, &addr,
			&resName, &nik, &phone, &email,
			&moveInDate, &jabatan, &password,
		); err != nil {
			return fmt.Errorf("scan export row: %w", err)
		}

		nikVal := nik
		if !canViewNik(ac, rtID) {
			nikVal = ""
		}
		phoneVal := phone
		emailVal := email
		if !canViewContact(ac, rtID) {
			phoneVal = ""
			emailVal = ""
		} else if phoneVal != "" {
			if norm, err := NormalizePhone(phoneVal); err == nil {
				phoneVal = norm
			}
		}

		record := []string{
			houseNum, headName, occStatus, addr,
			resName, nikVal, phoneVal, emailVal,
			moveInDate, jabatan, password,
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

		if err := ExportWargaCSV(r.Context(), pool, ac.RTID, w, ac); err != nil {
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
		if ac == nil {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
			return
		}

		targetRT, err := resolveImportRTID(r.Context(), pool, r, ac, true)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "target RT not found")
				return
			}
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
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

		allErrors, err := ValidateAgainstDatabase(r.Context(), pool, targetRT, rows, parseErrors)
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
			PreviewData: rowsToPreview(rows, ac),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// rowsToPreview converts WargaCSVRow (input model) to WargaPreviewRow (safe response DTO),
// stripping the password, adding a hint about password update status, and redacting NIK if unauthorized.
func rowsToPreview(rows []WargaCSVRow, acOpt ...*auth.AuthContext) []WargaPreviewRow {
	var ac *auth.AuthContext
	if len(acOpt) > 0 {
		ac = acOpt[0]
	}
	allowNik := canViewNik(ac, "")

	preview := make([]WargaPreviewRow, len(rows))
	for i, r := range rows {
		nikVal := r.Nik
		if !allowNik {
			nikVal = ""
		}
		preview[i] = WargaPreviewRow{
			RowNumber:       r.RowNumber,
			HouseNumber:     r.HouseNumber,
			HeadName:        r.HeadName,
			OccupancyStatus: r.OccupancyStatus,
			Address:         r.Address,
			ResidentName:    r.ResidentName,
			Nik:             nikVal,
			Phone:           r.Phone,
			Email:           r.Email,
			MoveInDate:      r.MoveInDate,
			Jabatan:         r.Jabatan,
		}
		if r.Password != "" {
			preview[i].PasswordHint = "password will be set"
		} else {
			preview[i].PasswordHint = "password unchanged"
		}
	}
	return preview
}

// HandleWargaImportCommit parses, validates, and commits CSV data to DB in a transaction.
func HandleWargaImportCommit(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
			return
		}

		targetRT, err := resolveImportRTID(r.Context(), pool, r, ac, true)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "target RT not found")
				return
			}
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
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

		allErrors, err := ValidateAgainstDatabase(r.Context(), pool, targetRT, rows, parseErrors)
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

		result, err := CommitWargaImport(r.Context(), pool, targetRT, ac.UserID, rows)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "import_failed", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(result)
	}
}

// --- Date helpers ---

// parseMoveInDate parses a YYYY-MM-DD date string. Returns error if not valid.
func parseMoveInDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("move_in_date is empty")
	}
	if len(s) < 10 {
		return "", fmt.Errorf("move_in_date is too short")
	}
	// Accept YYYY-MM-DD, strip leading/trailing spaces
	dateStr := s[:10]
	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", fmt.Errorf("move_in_date must be in YYYY-MM-DD format, got %q", dateStr)
	}
	return dateStr, nil
}

// normalizeDate normalizes a date to YYYY-MM-DD.
func normalizeDate(s string) string {
	// Already validated via parseMoveInDate; just return the YYYY-MM-DD part.
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// --- Jabatan helpers ---

// validRTJabatans defines all valid jabatan values for RT memberships.
var validRTJabatans = map[string]bool{
	"ketua":       true,
	"wakil_ketua": true,
	"sekretaris":  true,
	"bendahara":   true,
	"keamanan":    true,
	"sosial":      true,
	"operasional": true,
	"kebersihan":  true,
	"anggota":     true,
}

// canonicalJabatan maps display aliases to the canonical lower-case jabatan value.
var canonicalJabatan = map[string]string{
	"ketua":          "ketua",
	"ketua_rt":       "ketua",
	"ketua rt":       "ketua",
	"wakil_ketua":    "wakil_ketua",
	"wakil ketua":    "wakil_ketua",
	"sekretaris":     "sekretaris",
	"bendahara":      "bendahara",
	"keamanan":       "keamanan",
	"seksi keamanan": "keamanan",
	"sosial":         "sosial",
	"seksi sosial":   "sosial",
	"operasional":    "operasional",
	"kebersihan":     "kebersihan",
	"anggota":        "anggota",
}

// validateJabatan checks that the jabatan string is valid.
func validateJabatan(jabatan string) error {
	normed := normalizeJabatan(jabatan)
	if !validRTJabatans[normed] {
		validList := []string{
			"ketua", "wakil_ketua", "sekretaris", "bendahara",
			"keamanan", "sosial", "operasional", "kebersihan", "anggota",
		}
		return fmt.Errorf(
			"jabatan %q is not valid; must be one of: %s",
			jabatan, strings.Join(validList, ", "),
		)
	}
	return nil
}

// normalizeJabatan maps a display string to the canonical lower-case jabatan.
// It handles aliases like "Seksi Keamanan" -> "keamanan".
func normalizeJabatan(jabatan string) string {
	l := strings.ToLower(strings.TrimSpace(jabatan))
	if val, ok := canonicalJabatan[l]; ok {
		return val
	}
	// If already a canonical value, return as-is
	if validRTJabatans[l] {
		return l
	}
	return l
}