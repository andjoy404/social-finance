package security

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ScheduleExcelHeaders defines the standard columns for security schedule Excel files.
var ScheduleExcelHeaders = []string{
	"ID Jadwal",
	"Nama Petugas",
	"Jenis Shift",
	"Tanggal Mulai",
	"Tanggal Selesai",
	"Informasi Jadwal",
}

type ScheduleExcelRow struct {
	RowNumber     int      `json:"row_number"`
	ScheduleID    string   `json:"schedule_id"`
	OfficerName   string   `json:"officer_name"`
	OfficerUserID string   `json:"officer_user_id"`
	ShiftName     string   `json:"shift_name"`
	ShiftConfigID string   `json:"shift_config_id"`
	StartDate     string   `json:"start_date"`
	EndDate       string   `json:"end_date"`
	Notes         string   `json:"notes"`
	Action        string   `json:"action"` // "create" | "update"
	IsValid       bool     `json:"is_valid"`
	Errors        []string `json:"errors"`
}

type ScheduleImportPreviewResponse struct {
	TotalRows    int                `json:"total_rows"`
	ValidRows    int                `json:"valid_rows"`
	RejectedRows int                `json:"rejected_rows"`
	EmptyRows    int                `json:"empty_rows"`
	Rows         []ScheduleExcelRow `json:"rows"`
}

type ScheduleImportCommitRequest struct {
	Rows []ScheduleExcelRow `json:"rows"`
}

type ScheduleImportCommitResponse struct {
	Message        string `json:"message"`
	CreatedCount   int    `json:"created_count"`
	UpdatedCount   int    `json:"updated_count"`
	TotalCommitted int    `json:"total_committed"`
}

// parseExcelDate parses dates formatted as YYYY-MM-DD or standard Excel serials/formats.
func parseExcelDate(s string) (time.Time, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, "", errors.New("tanggal wajib diisi")
	}

	// Support Excel float/integer serial dates (e.g. 45678)
	if fVal, err := strconv.ParseFloat(s, 64); err == nil && fVal > 1000 && fVal < 100000 {
		t, err := excelize.ExcelDateToTime(fVal, false)
		if err == nil {
			return t, t.Format("2006-01-02"), nil
		}
	}

	// Try canonical YYYY-MM-DD
	if len(s) >= 10 {
		datePart := s[:10]
		if t, err := time.Parse("2006-01-02", datePart); err == nil {
			return t, t.Format("2006-01-02"), nil
		}
	}

	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"02-01-2006",
		"02/01/2006",
		"2006-1-2",
		"2006/1/2",
		"2-1-2006",
		"2/1/2006",
		"02-Jan-2006",
		"2-Jan-2006",
		"02 Jan 2006",
		"2 Jan 2006",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, t.Format("2006-01-02"), nil
		}
	}

	return time.Time{}, "", fmt.Errorf("format tanggal tidak valid: %q (gunakan format YYYY-MM-DD)", s)
}

const (
	maxExcelScheduleFileSize = 20 << 20 // 20 MB
	MaxImportScheduleRows    = 500
)

func readScheduleXLSX(r *http.Request) (*excelize.File, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxExcelScheduleFileSize); err != nil {
			return nil, fmt.Errorf("parse multipart form: %w", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, errors.New("multipart field 'file' wajib diisi")
		}
		defer file.Close()
		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, file); err != nil {
			return nil, fmt.Errorf("read uploaded file: %w", err)
		}
		f, err := excelize.OpenReader(buf)
		if err != nil {
			return nil, fmt.Errorf("buka file excel: %w", err)
		}
		return f, nil
	}

	limited := io.LimitReader(r.Body, maxExcelScheduleFileSize)
	dat, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(dat) == 0 {
		return nil, errors.New("body request kosong")
	}
	f, err := excelize.OpenReader(bytes.NewReader(dat))
	if err != nil {
		return nil, fmt.Errorf("buka file excel: %w", err)
	}
	return f, nil
}

// ExportSchedulesXLSX builds an Excel workbook containing current schedules and instructions.
func (s *Service) ExportSchedulesXLSX(ctx context.Context, rtID string, f ScheduleFilter) (*excelize.File, error) {
	officers, err := s.repo.ListOfficers(ctx, rtID)
	if err != nil {
		return nil, fmt.Errorf("list officers: %w", err)
	}

	shifts, err := s.repo.ListShifts(ctx, rtID, true)
	if err != nil {
		return nil, fmt.Errorf("list shifts: %w", err)
	}

	// Query distinct assignment periods matching filter
	where := []string{"s.rt_id = $1", "s.status = 'scheduled'"}
	args := []any{rtID}
	idx := 2

	if f.DateFrom != "" {
		where = append(where, fmt.Sprintf("s.shift_date >= $%d::date", idx))
		args = append(args, f.DateFrom)
		idx++
	}
	if f.DateTo != "" {
		where = append(where, fmt.Sprintf("s.shift_date <= $%d::date", idx))
		args = append(args, f.DateTo)
		idx++
	}
	if f.UserID != "" {
		where = append(where, fmt.Sprintf("s.user_id = $%d", idx))
		args = append(args, f.UserID)
		idx++
	}
	if f.ShiftConfigID != "" {
		where = append(where, fmt.Sprintf("s.shift_config_id = $%d", idx))
		args = append(args, f.ShiftConfigID)
		idx++
	}

	q := fmt.Sprintf(`
		SELECT DISTINCT ON (s.assignment_id)
			COALESCE(s.assignment_id::text, s.id::text) AS sched_id,
			u.full_name AS officer_name,
			s.shift_name_snapshot AS shift_name,
			to_char(s.assignment_start_date, 'YYYY-MM-DD') AS start_date,
			to_char(s.assignment_end_date, 'YYYY-MM-DD') AS end_date,
			COALESCE(s.notes, '') AS notes,
			s.assignment_start_date
		FROM security_schedules s
		JOIN users u ON u.id = s.user_id
		WHERE %s
		ORDER BY s.assignment_id, s.shift_date ASC
	`, strings.Join(where, " AND "))

	rows, err := s.repo.pool.Raw().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query schedules for export: %w", err)
	}
	defer rows.Close()

	type exportRow struct {
		id, officer, shift, start, end, notes string
		startDate                             time.Time
	}
	var dataRows []exportRow
	for rows.Next() {
		var r exportRow
		if err := rows.Scan(&r.id, &r.officer, &r.shift, &r.start, &r.end, &r.notes, &r.startDate); err != nil {
			return nil, fmt.Errorf("scan export row: %w", err)
		}
		dataRows = append(dataRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate export rows: %w", err)
	}

	file := excelize.NewFile()

	// 1. Create sheet "Jadwal"
	schedSheet := "Jadwal"
	sIdx, err := file.NewSheet(schedSheet)
	if err != nil {
		file.Close()
		return nil, err
	}
	file.SetActiveSheet(sIdx)
	_ = file.DeleteSheet("Sheet1")

	// Header style
	hdrStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11, Family: "Calibri"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#1E3A8A"}, Pattern: 1},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#000000", Style: 2},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
		},
	})
	if err != nil {
		file.Close()
		return nil, err
	}

	// Write headers
	for ci, h := range ScheduleExcelHeaders {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		_ = file.SetCellValue(schedSheet, cell, h)
	}
	_ = file.SetRowStyle(schedSheet, 1, 1, hdrStyle)
	_ = file.SetRowHeight(schedSheet, 1, 28)

	// Write data rows
	rowNum := 2
	for _, d := range dataRows {
		vals := []string{d.id, d.officer, d.shift, d.start, d.end, d.notes}
		for ci, val := range vals {
			cell, _ := excelize.CoordinatesToCellName(ci+1, rowNum)
			_ = file.SetCellStr(schedSheet, cell, val)
		}
		rowNum++
	}

	// Freeze header pane & autofilter
	_ = file.SetPanes(schedSheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
	_ = file.AutoFilter(schedSheet, fmt.Sprintf("A1:F%d", max(rowNum-1, 1)), []excelize.AutoFilterOptions{})

	// Column widths
	colWidths := map[string]float64{
		"A": 38, // ID Jadwal (UUID)
		"B": 26, // Nama Petugas
		"C": 22, // Jenis Shift
		"D": 16, // Tanggal Mulai
		"E": 16, // Tanggal Selesai
		"F": 32, // Informasi Jadwal
	}
	for col, w := range colWidths {
		_ = file.SetColWidth(schedSheet, col, col, w)
	}

	// 2. Create sheet "Petunjuk"
	guideSheet := "Petunjuk"
	_, err = file.NewSheet(guideSheet)
	if err != nil {
		file.Close()
		return nil, err
	}

	guideTitleStyle, _ := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Color: "#1E3A8A"},
	})
	sectionHdrStyle, _ := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "#FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#2563EB"}, Pattern: 1},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	_ = file.SetCellValue(guideSheet, "A1", "PANDUAN EKSPOR & IMPOR JADWAL PETUGAS KEAMANAN")
	_ = file.SetCellStyle(guideSheet, "A1", "A1", guideTitleStyle)

	instructions := [][]string{
		{"Kolom", "Aturan Pengisian", "Keterangan"},
		{"ID Jadwal", "KOSONGKAN untuk jadwal baru. JANGAN UBAH jika ingin memperbarui jadwal lama.", "UUID unik penugasan dari sistem"},
		{"Nama Petugas", "Wajib diisi sesuai daftar nama petugas aktif yang tersedia di bawah.", "Pilih dari dropdown atau ketik persis"},
		{"Jenis Shift", "Wajib diisi sesuai daftar nama shift yang aktif di bawah.", "Pilih dari dropdown atau ketik persis"},
		{"Tanggal Mulai", "Wajib diisi dalam format YYYY-MM-DD (contoh: 2026-11-15).", "Tanggal awal periode jadwal"},
		{"Tanggal Selesai", "Wajib diisi dalam format YYYY-MM-DD (harus sama atau setelah Tanggal Mulai).", "Tanggal akhir periode jadwal (maks 366 hari)"},
		{"Informasi Jadwal", "Opsional (maksimal 255 karakter).", "Catatan tugas atau keterangan tambahan"},
	}

	for rIdx, row := range instructions {
		for cIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(cIdx+1, rIdx+3)
			_ = file.SetCellValue(guideSheet, cell, val)
		}
	}
	_ = file.SetRowStyle(guideSheet, 3, 3, sectionHdrStyle)

	// Master Petugas table
	offHdrRow := 12
	_ = file.SetCellValue(guideSheet, "A11", "DAFTAR PETUGAS KEAMANAN AKTIF")
	_ = file.SetCellStyle(guideSheet, "A11", "A11", guideTitleStyle)
	_ = file.SetCellValue(guideSheet, "A12", "No")
	_ = file.SetCellValue(guideSheet, "B12", "Nama Lengkap Petugas")
	_ = file.SetCellValue(guideSheet, "C12", "No. Telepon / Email")
	_ = file.SetRowStyle(guideSheet, offHdrRow, offHdrRow, sectionHdrStyle)

	for i, off := range officers {
		r := offHdrRow + 1 + i
		_ = file.SetCellValue(guideSheet, fmt.Sprintf("A%d", r), i+1)
		_ = file.SetCellValue(guideSheet, fmt.Sprintf("B%d", r), off.FullName)
		contact := off.Email
		if off.Phone != nil && *off.Phone != "" {
			contact = *off.Phone
		}
		_ = file.SetCellValue(guideSheet, fmt.Sprintf("C%d", r), contact)
	}

	// Master Shift table
	shiftHdrRow := offHdrRow + len(officers) + 3
	_ = file.SetCellValue(guideSheet, fmt.Sprintf("A%d", shiftHdrRow-1), "DAFTAR SHIFT KEAMANAN AKTIF")
	_ = file.SetCellStyle(guideSheet, fmt.Sprintf("A%d", shiftHdrRow-1), fmt.Sprintf("A%d", shiftHdrRow-1), guideTitleStyle)
	_ = file.SetCellValue(guideSheet, fmt.Sprintf("A%d", shiftHdrRow), "No")
	_ = file.SetCellValue(guideSheet, fmt.Sprintf("B%d", shiftHdrRow), "Nama Shift")
	_ = file.SetCellValue(guideSheet, fmt.Sprintf("C%d", shiftHdrRow), "Jam Operasional")
	_ = file.SetRowStyle(guideSheet, shiftHdrRow, shiftHdrRow, sectionHdrStyle)

	for i, sh := range shifts {
		r := shiftHdrRow + 1 + i
		_ = file.SetCellValue(guideSheet, fmt.Sprintf("A%d", r), i+1)
		_ = file.SetCellValue(guideSheet, fmt.Sprintf("B%d", r), sh.Name)
		_ = file.SetCellValue(guideSheet, fmt.Sprintf("C%d", r), fmt.Sprintf("%s - %s", sh.StartTime, sh.EndTime))
	}

	_ = file.SetColWidth(guideSheet, "A", "A", 18)
	_ = file.SetColWidth(guideSheet, "B", "B", 35)
	_ = file.SetColWidth(guideSheet, "C", "C", 40)

	// 3. Dropdown Data Validation for column B (Nama Petugas) & column C (Jenis Shift)
	maxDataRow := max(rowNum+50, 100)
	if len(officers) > 0 {
		offStartRow := offHdrRow + 1
		offEndRow := offHdrRow + len(officers)
		dvOfficer := excelize.NewDataValidation(false)
		dvOfficer.Sqref = fmt.Sprintf("B2:B%d", maxDataRow)
		dvOfficer.Type = "list"
		dvOfficer.Formula1 = fmt.Sprintf("'%s'!$B$%d:$B$%d", guideSheet, offStartRow, offEndRow)
		dvOfficer.AllowBlank = true
		_ = file.AddDataValidation(schedSheet, dvOfficer)
	}

	if len(shifts) > 0 {
		shStartRow := shiftHdrRow + 1
		shEndRow := shiftHdrRow + len(shifts)
		dvShift := excelize.NewDataValidation(false)
		dvShift.Sqref = fmt.Sprintf("C2:C%d", maxDataRow)
		dvShift.Type = "list"
		dvShift.Formula1 = fmt.Sprintf("'%s'!$B$%d:$B$%d", guideSheet, shStartRow, shEndRow)
		dvShift.AllowBlank = true
		_ = file.AddDataValidation(schedSheet, dvShift)
	}

	return file, nil
}

// ParseAndValidateScheduleXLSX validates an uploaded Excel file and returns preview information.
func (s *Service) ParseAndValidateScheduleXLSX(ctx context.Context, rtID string, file *excelize.File) (*ScheduleImportPreviewResponse, error) {
	sheetList := file.GetSheetList()
	if len(sheetList) == 0 {
		return nil, newAppError(http.StatusBadRequest, "no_sheet", "Workbook Excel tidak memiliki lembar kerja", ErrValidation)
	}

	sheetName := sheetList[0]
	for _, name := range sheetList {
		if strings.EqualFold(name, "Jadwal") || strings.EqualFold(name, "Schedule") {
			sheetName = name
			break
		}
	}

	rawRows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, newAppError(http.StatusBadRequest, "invalid_file", fmt.Sprintf("Gagal membaca lembar kerja %q: %v", sheetName, err), ErrValidation)
	}
	if len(rawRows) == 0 {
		return nil, newAppError(http.StatusBadRequest, "empty_file", "File Excel kosong", ErrValidation)
	}
	if len(rawRows)-1 > MaxImportScheduleRows {
		return nil, newAppError(http.StatusBadRequest, "too_many_rows", fmt.Sprintf("Jumlah baris Excel (%d) melebihi batas maksimal %d baris per import", len(rawRows)-1, MaxImportScheduleRows), ErrValidation)
	}

	// Map headers
	headerMap := make(map[string]int)
	for ci, h := range rawRows[0] {
		clean := strings.ToLower(strings.TrimSpace(h))
		headerMap[clean] = ci
	}

	findCol := func(names ...string) (int, bool) {
		for _, n := range names {
			clean := strings.ToLower(strings.TrimSpace(n))
			if idx, ok := headerMap[clean]; ok {
				return idx, true
			}
		}
		return -1, false
	}

	colOfficer, hasOfficer := findCol("Nama Petugas", "officer_name", "nama", "petugas")
	colShift, hasShift := findCol("Jenis Shift", "shift_name", "shift", "jenis_shift")
	colStart, hasStart := findCol("Tanggal Mulai", "start_date", "tanggal_mulai", "mulai")
	colEnd, hasEnd := findCol("Tanggal Selesai", "end_date", "tanggal_selesai", "selesai")
	colID, _ := findCol("ID Jadwal", "schedule_id", "assignment_id", "id", "id_jadwal")
	colNotes, _ := findCol("Informasi Jadwal", "notes", "catatan", "keterangan", "informasi_jadwal")

	if !hasOfficer || !hasShift || !hasStart || !hasEnd {
		return nil, newAppError(http.StatusBadRequest, "missing_headers", "Kolom wajib tidak lengkap: Nama Petugas, Jenis Shift, Tanggal Mulai, Tanggal Selesai harus ada", ErrValidation)
	}

	// Load officers & shifts
	officers, err := s.repo.ListOfficers(ctx, rtID)
	if err != nil {
		return nil, err
	}
	officerByName := make(map[string]SecurityOfficer)
	for _, o := range officers {
		officerByName[strings.ToLower(strings.TrimSpace(o.FullName))] = o
	}

	shifts, err := s.repo.ListShifts(ctx, rtID, false)
	if err != nil {
		return nil, err
	}
	shiftByName := make(map[string]ShiftConfig)
	for _, sh := range shifts {
		shiftByName[strings.ToLower(strings.TrimSpace(sh.Name))] = sh
	}

	getCell := func(row []string, idx int) string {
		if idx >= 0 && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	type fileScheduleSlot struct {
		rowNumber int
		userID    string
		startAt   time.Time
		endAt     time.Time
	}
	var intraSlots []fileScheduleSlot
	seenIDs := make(map[string]int)

	resp := &ScheduleImportPreviewResponse{
		Rows: make([]ScheduleExcelRow, 0, len(rawRows)-1),
	}

	now := s.NowUTC()
	today := now.In(JakartaLoc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, JakartaLoc)

	for rIdx := 1; rIdx < len(rawRows); rIdx++ {
		rowRecord := rawRows[rIdx]
		idRaw := getCell(rowRecord, colID)
		officerRaw := getCell(rowRecord, colOfficer)
		shiftRaw := getCell(rowRecord, colShift)
		startRaw := getCell(rowRecord, colStart)
		endRaw := getCell(rowRecord, colEnd)
		notesRaw := getCell(rowRecord, colNotes)

		// Check if entirely empty
		if idRaw == "" && officerRaw == "" && shiftRaw == "" && startRaw == "" && endRaw == "" && notesRaw == "" {
			resp.EmptyRows++
			continue
		}

		resp.TotalRows++
		rowNumber := rIdx + 1
		excelRow := ScheduleExcelRow{
			RowNumber:   rowNumber,
			ScheduleID:  idRaw,
			OfficerName: officerRaw,
			ShiftName:   shiftRaw,
			Notes:       notesRaw,
			Action:      "create",
			Errors:      []string{},
		}
		if len(excelRow.Notes) > 255 {
			excelRow.Notes = excelRow.Notes[:255]
		}

		// 1. Validate Officer
		var chosenOfficer SecurityOfficer
		if officerRaw == "" {
			excelRow.Errors = append(excelRow.Errors, "Nama Petugas wajib diisi")
		} else {
			off, found := officerByName[strings.ToLower(officerRaw)]
			if !found {
				excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Petugas %q tidak ditemukan dalam data RT ini", officerRaw))
			} else if !off.IsActive {
				excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Petugas %q sudah tidak aktif", officerRaw))
			} else {
				chosenOfficer = off
				excelRow.OfficerUserID = off.UserID
				excelRow.OfficerName = off.FullName
			}
		}

		// 2. Validate Shift
		var chosenShift ShiftConfig
		if shiftRaw == "" {
			excelRow.Errors = append(excelRow.Errors, "Jenis Shift wajib diisi")
		} else {
			sh, found := shiftByName[strings.ToLower(shiftRaw)]
			if !found {
				excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Jenis Shift %q tidak ditemukan", shiftRaw))
			} else if !sh.IsActive {
				excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Jenis Shift %q sudah tidak aktif", shiftRaw))
			} else {
				chosenShift = sh
				excelRow.ShiftConfigID = sh.ID
				excelRow.ShiftName = sh.Name
			}
		}

		// 3. Validate Dates
		var startDate, endDate time.Time
		if startRaw == "" {
			excelRow.Errors = append(excelRow.Errors, "Tanggal Mulai wajib diisi")
		} else {
			t, norm, err := parseExcelDate(startRaw)
			if err != nil {
				excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Tanggal Mulai: %v", err))
			} else {
				startDate = t
				excelRow.StartDate = norm
			}
		}

		if endRaw == "" {
			excelRow.Errors = append(excelRow.Errors, "Tanggal Selesai wajib diisi")
		} else {
			t, norm, err := parseExcelDate(endRaw)
			if err != nil {
				excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Tanggal Selesai: %v", err))
			} else {
				endDate = t
				excelRow.EndDate = norm
			}
		}

		if !startDate.IsZero() && !endDate.IsZero() {
			if endDate.Before(startDate) {
				excelRow.Errors = append(excelRow.Errors, "Tanggal Selesai tidak boleh sebelum Tanggal Mulai")
			} else if int(endDate.Sub(startDate).Hours()/24)+1 > maxSchedulePeriodDays {
				excelRow.Errors = append(excelRow.Errors, "Rentang tanggal jadwal maksimal 366 hari")
			}
		}

		// 4. Validate Schedule ID (if provided)
		var existingAssignmentID string
		if idRaw != "" {
			excelRow.Action = "update"
			if _, err := uuid.Parse(idRaw); err != nil {
				excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Format ID Jadwal %q tidak valid", idRaw))
			} else {
				// Check duplicate ID in the same file
				if prevRow, dup := seenIDs[idRaw]; dup {
					excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("ID Jadwal duplikat dengan baris %d", prevRow))
				} else {
					seenIDs[idRaw] = rowNumber
				}

				// Query existing schedule assignment
				var currentUser string
				var oldStart, oldEnd time.Time
				err := s.repo.pool.Raw().QueryRowContext(ctx, `
					SELECT assignment_id, user_id, assignment_start_date, assignment_end_date
					FROM security_schedules
					WHERE (id = $1::uuid OR assignment_id = $1::uuid) AND rt_id = $2
					ORDER BY shift_date LIMIT 1
				`, idRaw, rtID).Scan(&existingAssignmentID, &currentUser, &oldStart, &oldEnd)

				if errors.Is(err, sql.ErrNoRows) {
					excelRow.Errors = append(excelRow.Errors, "ID Jadwal tidak ditemukan atau bukan milik RT ini")
				} else if err != nil {
					excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Cek database ID Jadwal: %v", err))
				} else {
					// Check attendance lock for dates with recorded attendance
					var lockedCount int
					_ = s.repo.pool.Raw().QueryRowContext(ctx, `
						SELECT COUNT(*)
						FROM security_schedules s
						LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
						WHERE s.rt_id = $1 AND s.assignment_id = $2::uuid
						  AND s.shift_date >= $3::date
						  AND (ar.checked_in_at IS NOT NULL OR ar.checked_out_at IS NOT NULL OR (ar.status IS NOT NULL AND ar.status <> 'belum_hadir'))
					`, rtID, existingAssignmentID, today.Format("2006-01-02")).Scan(&lockedCount)

					if lockedCount > 0 {
						excelRow.Errors = append(excelRow.Errors, "Jadwal memiliki riwayat absensi aktif dan tidak dapat diubah")
					}
				}
			}
		}

		// 5. Intra-file conflict check & Database conflict check
		if len(excelRow.Errors) == 0 && chosenOfficer.UserID != "" && chosenShift.ID != "" && !startDate.IsZero() && !endDate.IsZero() {
			// Compute slots for each day in range
			hasIntraConflict := false
			for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
				bs, be, _, e := ComputeShiftBounds(day, chosenShift.StartTime, chosenShift.EndTime, chosenShift.CrossesMidnight)
				if e != nil {
					excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Waktu shift tidak valid: %v", e))
					break
				}

				// Intra-file conflict
				for _, slot := range intraSlots {
					if slot.userID == chosenOfficer.UserID {
						if bs.Before(slot.endAt) && be.After(slot.startAt) {
							excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Bentrok jadwal petugas pada tanggal %s dengan baris %d dalam file ini", day.Format("2006-01-02"), slot.rowNumber))
							hasIntraConflict = true
							break
						}
					}
				}
				if hasIntraConflict {
					break
				}

				// Database conflict check
				var conflictDate string
				var errConf error
				if existingAssignmentID != "" {
					errConf = s.repo.pool.Raw().QueryRowContext(ctx, `
						SELECT to_char(shift_date, 'YYYY-MM-DD')
						FROM security_schedules
						WHERE rt_id = $1 AND user_id = $2 AND status = 'scheduled'
						  AND assignment_id <> $3::uuid
						  AND scheduled_start_at < $4 AND scheduled_end_at > $5
						ORDER BY shift_date LIMIT 1
					`, rtID, chosenOfficer.UserID, existingAssignmentID, be, bs).Scan(&conflictDate)
				} else {
					errConf = s.repo.pool.Raw().QueryRowContext(ctx, `
						SELECT to_char(shift_date, 'YYYY-MM-DD')
						FROM security_schedules
						WHERE rt_id = $1 AND user_id = $2 AND status = 'scheduled'
						  AND scheduled_start_at < $3 AND scheduled_end_at > $4
						ORDER BY shift_date LIMIT 1
					`, rtID, chosenOfficer.UserID, be, bs).Scan(&conflictDate)
				}

				if errConf == nil {
					excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Jadwal bentrok pada sistem untuk tanggal %s", conflictDate))
					break
				} else if !errors.Is(errConf, sql.ErrNoRows) {
					excelRow.Errors = append(excelRow.Errors, fmt.Sprintf("Cek bentrok jadwal database: %v", errConf))
					break
				}
			}

			// Add to intraSlots if no errors so far
			if len(excelRow.Errors) == 0 {
				for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
					bs, be, _, _ := ComputeShiftBounds(day, chosenShift.StartTime, chosenShift.EndTime, chosenShift.CrossesMidnight)
					intraSlots = append(intraSlots, fileScheduleSlot{
						rowNumber: rowNumber,
						userID:    chosenOfficer.UserID,
						startAt:   bs,
						endAt:     be,
					})
				}
			}
		}

		if len(excelRow.Errors) == 0 {
			excelRow.IsValid = true
			resp.ValidRows++
		} else {
			excelRow.IsValid = false
			resp.RejectedRows++
		}

		resp.Rows = append(resp.Rows, excelRow)
	}

	return resp, nil
}

// CommitScheduleImport atomically commits validated schedule rows.
func (s *Service) CommitScheduleImport(ctx context.Context, rtID, actorID string, req ScheduleImportCommitRequest) (*ScheduleImportCommitResponse, error) {
	if len(req.Rows) == 0 {
		return nil, newAppError(http.StatusBadRequest, "empty_rows", "Tidak ada data jadwal untuk disimpan", ErrValidation)
	}
	if len(req.Rows) > MaxImportScheduleRows {
		return nil, newAppError(http.StatusBadRequest, "too_many_rows", fmt.Sprintf("Jumlah baris (%d) melebihi batas maksimal %d baris per import", len(req.Rows), MaxImportScheduleRows), ErrValidation)
	}

	tx, err := s.repo.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := s.NowUTC()
	createdCount := 0
	updatedCount := 0
	seenScheduleIDs := make(map[string]int)

	for _, r := range req.Rows {
		// Defensive validation: commit independently re-validates all identifiers and dates
		if r.ScheduleID != "" {
			if _, err := uuid.Parse(r.ScheduleID); err != nil {
				return nil, newAppError(http.StatusBadRequest, "invalid_uuid", fmt.Sprintf("baris %d: format ID Jadwal tidak valid", r.RowNumber), ErrValidation)
			}
			if prevRow, dup := seenScheduleIDs[r.ScheduleID]; dup {
				return nil, newAppError(http.StatusBadRequest, "duplicate_id", fmt.Sprintf("baris %d: duplikat ID Jadwal dengan baris %d dalam file", r.RowNumber, prevRow), ErrValidation)
			}
			seenScheduleIDs[r.ScheduleID] = r.RowNumber
		}

		if _, err := uuid.Parse(r.OfficerUserID); err != nil {
			return nil, newAppError(http.StatusBadRequest, "invalid_uuid", fmt.Sprintf("baris %d: ID Petugas tidak valid", r.RowNumber), ErrValidation)
		}
		if _, err := uuid.Parse(r.ShiftConfigID); err != nil {
			return nil, newAppError(http.StatusBadRequest, "invalid_uuid", fmt.Sprintf("baris %d: ID Shift tidak valid", r.RowNumber), ErrValidation)
		}

		start, end, err := normalizeScheduleDates(r.StartDate, r.EndDate, "")
		if err != nil {
			return nil, fmt.Errorf("baris %d: tanggal tidak valid: %w", r.RowNumber, err)
		}

		var notesPtr *string
		if r.Notes != "" {
			trimmed := strings.TrimSpace(r.Notes)
			if len(trimmed) > 255 {
				trimmed = trimmed[:255]
			}
			notesPtr = &trimmed
		}

		if r.Action == "create" || r.ScheduleID == "" {
			createReq := CreateScheduleRequest{
				UserID:        r.OfficerUserID,
				ShiftConfigID: r.ShiftConfigID,
				StartDate:     r.StartDate,
				EndDate:       r.EndDate,
				Notes:         notesPtr,
			}
			if _, err := s.repo.createSchedulePeriodTx(ctx, tx, rtID, actorID, createReq, start, end, now); err != nil {
				return nil, fmt.Errorf("baris %d: gagal membuat jadwal: %w", r.RowNumber, err)
			}
			createdCount++
		} else {
			updateReq := UpdateScheduleRequest{
				UserID:        r.OfficerUserID,
				ShiftConfigID: r.ShiftConfigID,
				StartDate:     r.StartDate,
				EndDate:       r.EndDate,
				Notes:         notesPtr,
			}
			if _, err := s.repo.updateSchedulePeriodTx(ctx, tx, rtID, r.ScheduleID, actorID, updateReq, start, end, now); err != nil {
				return nil, fmt.Errorf("baris %d: gagal memperbarui jadwal %s: %w", r.RowNumber, r.ScheduleID, err)
			}
			updatedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &ScheduleImportCommitResponse{
		Message:        fmt.Sprintf("Berhasil memproses %d jadwal (%d dibuat, %d diperbarui)", createdCount+updatedCount, createdCount, updatedCount),
		CreatedCount:   createdCount,
		UpdatedCount:   updatedCount,
		TotalCommitted: createdCount + updatedCount,
	}, nil
}
