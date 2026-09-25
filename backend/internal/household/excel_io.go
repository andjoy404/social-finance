package household

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xuri/excelize/v2"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

// excelHeaders matches the exact column order and names used in CSV export/import.
var excelHeaders = []string{
	"house_number", "head_name", "occupancy_status",
	"address", "resident_name", "nik", "phone",
	"email", "relationship_to_head",
}

// RequiredColumns are the Excel columns that must be present for import.
var requiredColumns = []string{
	"house_number", "head_name", "occupancy_status", "resident_name",
}

// parseExcelRows reads a .xlsx file into []WargaCSVRow, reusing the same
// validation rules (WargaCSVRow struct, ValidateNik, NormalizePhone, ValidateEmail,
// NormalizeEmail) and cross-row consistency checks as ParseWargaCSV.
func parseExcelRows(f *excelize.File) ([]WargaCSVRow, []RowError, error) {
	sheetNames := f.GetSheetList()
	if len(sheetNames) == 0 {
		return nil, []RowError{{
			Row:     1,
			Field:   "file",
			Code:    "no_sheet",
			Message: "workbook contains no worksheets",
		}}, nil
	}
	// Use the first worksheet.
	sheetName := sheetNames[0]

	rawRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("read worksheet %q: %w", sheetName, err)
	}
	if len(rawRows) == 0 {
		return nil, []RowError{{
			Row:     1,
			Field:   "file",
			Code:    "empty_file",
			Message: "workbook is empty",
		}}, nil
	}

	// Map header names (lowercased, trimmed) → column index.
	colIdx := make(map[string]int)
	for i, h := range rawRows[0] {
		clean := strings.ToLower(strings.TrimSpace(h))
		colIdx[clean] = i
	}

	// Validate required headers.
	var headerErrors []RowError
	for _, req := range requiredColumns {
		if _, ok := colIdx[req]; !ok {
			headerErrors = append(headerErrors, RowError{
				Row:     1,
				Field:   req,
				Code:    "missing_header",
				Message: fmt.Sprintf("required header column %q is missing", req),
			})
		}
	}
	if len(headerErrors) > 0 {
		return nil, headerErrors, nil
	}

	// getVal returns the first non-empty value from the given alias order.
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

	// Data rows start at row 2 (rawRows index 1).
	for i, record := range rawRows[1:] {
		rowNum := i + 2

		// Skip fully empty rows (all cells blank).
		allEmpty := true
		for _, cell := range record {
			if strings.TrimSpace(cell) != "" {
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

		// Read NIK as raw string to prevent Excel from coercing it to a number.
		nikRaw := strings.TrimSpace(getVal(record, "nik", "resident_nik", "ktp"))
		// Excel may store numbers in scientific notation when reading; strip
		// non-digit characters to defensively recover a clean NIK string.
		nikRaw = stripNonDigits(nikRaw)

		row := WargaCSVRow{
			RowNumber:          rowNum,
			HouseNumber:        getVal(record, "house_number", "nomor_rumah"),
			HeadName:           getVal(record, "head_name", "nama_kepala_keluarga", "nama_warga"),
			OccupancyStatus:    normStatus,
			Address:            getVal(record, "address", "alamat"),
			ResidentName:       getVal(record, "resident_name", "nama_anggota"),
			Nik:                nikRaw,
			Phone:              getVal(record, "phone", "resident_phone", "household_phone", "telepon"),
			Email:              getVal(record, "email", "resident_email"),
			RelationshipToHead: getVal(record, "relationship_to_head", "hubungan"),
		}

		// Field-level validation (identical to ParseWargaCSV).
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

	// Cross-row consistency validation (identical logic to ParseWargaCSV).
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
					Message: fmt.Sprintf("house number %q has conflicting head names (%q and %q)", r.HouseNumber, existingHead, r.HeadName),
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
					Message: fmt.Sprintf("house number %q has conflicting occupancy statuses (%q and %q)", r.HouseNumber, existingStatus, r.OccupancyStatus),
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
					Message: fmt.Sprintf("NIK %q is duplicated with row %d", r.Nik, prevRow),
				})
			} else {
				nikToRow[r.Nik] = r.RowNumber
			}
		}
	}

	return rows, rowErrors, nil
}

// stripNonDigits removes all characters that are not 0-9 from the input string.
func stripNonDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteByte(byte(c))
		}
	}
	return b.String()
}

// readXLSXBody opens the uploaded .xlsx file from a multipart form or raw body.
// The Excelize v2 OpenReader API requires io.Reader and reads the whole file into memory.
const maxExcelFileSize = 20 << 20 // 20 MB

func readXLSXBody(r *http.Request) (*excelize.File, error) {
	contentType := r.Header.Get("Content-Type")
	var buf *bytes.Buffer

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxExcelFileSize); err != nil {
			return nil, fmt.Errorf("parse multipart form: %w", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, errors.New("multipart field 'file' is required")
		}
		defer file.Close()
		buf = &bytes.Buffer{}
		if _, err := io.Copy(buf, file); err != nil {
			return nil, fmt.Errorf("read uploaded file: %w", err)
		}
		f, err := excelize.OpenReader(buf)
		if err != nil {
			return nil, fmt.Errorf("read xlsx file: %w", err)
		}
		return f, nil
	}

	limited := io.LimitReader(r.Body, maxExcelFileSize)
	dat, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(dat) == 0 {
		return nil, errors.New("request body is empty")
	}
	f, err := excelize.OpenReader(bytes.NewReader(dat))
	if err != nil {
		return nil, fmt.Errorf("read xlsx file: %w", err)
	}
	return f, nil
}

// --- HTTP Handlers ---

// HandleWargaExportXLSX exports tenant-scoped warga data as Excel (.xlsx).
func HandleWargaExportXLSX(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid or missing tenant context")
			return
		}

		f, err := exportWargaToXLSX(r.Context(), pool, ac.RTID)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="warga_export.xlsx"`)
		w.WriteHeader(http.StatusOK)

		if err := f.Write(w); err != nil {
			// Headers already sent; caller may receive a partial file.
			return
		}
	}
}

// HandleWargaTemplateXLSX downloads a pre-formatted Excel template for warga import.
func HandleWargaTemplateXLSX(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid or missing tenant context")
			return
		}

		f, err := buildTemplateXLSX()
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="warga_template.xlsx"`)
		w.WriteHeader(http.StatusOK)

		if err := f.Write(w); err != nil {
			return
		}
	}
}

// HandleWargaImportXLSXPreview parses and validates an Excel file without committing to DB.
func HandleWargaImportPreviewXLSX(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid or missing tenant context")
			return
		}

		f, err := readXLSXBody(r)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		defer f.Close()

		rows, parseErrors, err := parseExcelRows(f)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "xlsx_parse_error", err.Error())
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
		// json.NewEncoder is available since warga_io.go in this package uses it.
		encodeResponse(w, resp)
	}
}

// HandleWargaImportXLSXCommit parses, validates, and commits Excel data to DB.
func HandleWargaImportCommitXLSX(pool *database.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid or missing tenant context")
			return
		}

		f, err := readXLSXBody(r)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		defer f.Close()

		rows, parseErrors, err := parseExcelRows(f)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "xlsx_parse_error", err.Error())
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
			encodeErrorResponse(w, "validation_error", "Excel data contains validation errors", allErrors)
			return
		}

		result, err := CommitWargaImport(r.Context(), pool, ac.RTID, ac.UserID, rows)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "import_failed", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		encodeResponse(w, result)
	}
}

// --- Export Helpers ---

// exportWargaToXLSX queries tenant residents and writes them to a new .xlsx file.
func exportWargaToXLSX(ctx context.Context, pool *database.Pool, rtID string) (*excelize.File, error) {
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
		return nil, fmt.Errorf("query warga export: %w", err)
	}
	defer rows.Close()

	f := excelize.NewFile()
	sheetIdx, err := f.NewSheet("Warga")
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("new sheet: %w", err)
	}
	f.SetActiveSheet(sheetIdx)

	// Write header row.
	for ci, col := range excelHeaders {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		if err := f.SetCellValue("Warga", cell, col); err != nil {
			f.Close()
			return nil, fmt.Errorf("set header cell %s: %w", cell, err)
		}
	}

	rowNum := 2
	for rows.Next() {
		var houseNum, headName, occStatus, addr, resName, nik, phone, email, rel string
		if err := rows.Scan(
			&houseNum, &headName, &occStatus, &addr,
			&resName, &nik, &phone, &email, &rel,
		); err != nil {
			f.Close()
			return nil, fmt.Errorf("scan export row: %w", err)
		}

		var cell string
		for ci, val := range []string{
			houseNum, headName, occStatus, addr,
			resName, nik, phone, email, rel,
		} {
			cell, _ = excelize.CoordinatesToCellName(ci+1, rowNum)
			if err := f.SetCellValue("Warga", cell, val); err != nil {
				f.Close()
				return nil, fmt.Errorf("set export cell %s: %w", cell, err)
			}
		}

		rowNum++
	}
	if err := rows.Err(); err != nil {
		f.Close()
		return nil, fmt.Errorf("iterate export rows: %w", err)
	}

	// Set column widths for readability.
	columns := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	widths := []float64{14, 24, 16, 30, 24, 20, 18, 28, 20}
	for i, col := range columns {
		if i < len(widths) {
			f.SetColWidth("Warga", col, col, widths[i])
		}
	}

	return f, nil
}

// buildTemplateXLSX creates a blank Excel file with the canonical warga headers.
func buildTemplateXLSX() (*excelize.File, error) {
	f := excelize.NewFile()
	sheetIdx, err := f.NewSheet("Template")
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("new sheet: %w", err)
	}
	f.SetActiveSheet(sheetIdx)

	for ci, col := range excelHeaders {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		if err := f.SetCellValue("Template", cell, col); err != nil {
			f.Close()
			return nil, fmt.Errorf("set template cell %s: %w", cell, err)
		}
	}

	// Set column widths matching export.
	columns := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	widths := []float64{14, 24, 16, 30, 24, 20, 18, 28, 20}
	for i, col := range columns {
		if i < len(widths) {
			f.SetColWidth("Template", col, col, widths[i])
		}
	}

	return f, nil
}

// --- JSON encoding helpers ---

func encodeResponse(w http.ResponseWriter, v any) {
	json.NewEncoder(w).Encode(v)
}

func encodeErrorResponse(w http.ResponseWriter, code, message string, details any) {
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": details,
		},
	})
}
