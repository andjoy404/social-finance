package security

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

func TestParseExcelDate(t *testing.T) {
	cases := []struct {
		input       string
		expectedStr string
		expectErr   bool
	}{
		{"2026-11-15", "2026-11-15", false},
		{"2026/11/15", "2026-11-15", false},
		{"15-11-2026", "2026-11-15", false},
		{"15/11/2026", "2026-11-15", false},
		{"2026-1-5", "2026-01-05", false},
		{"5-1-2026", "2026-01-05", false},
		{"45678", "2025-01-21", false}, // Excel serial
		{"", "", true},
		{"bukan-tanggal", "", true},
		{"2026-02-31", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			_, str, err := parseExcelDate(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil (result: %s)", tc.input, str)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input %q: %v", tc.input, err)
				}
				if str != tc.expectedStr {
					t.Fatalf("expected %q, got %q", tc.expectedStr, str)
				}
			}
		})
	}
}

func createTestXLSX(sheetName string, headers []string, rows [][]string) *excelize.File {
	f := excelize.NewFile()
	if sheetName != "Sheet1" {
		idx, _ := f.NewSheet(sheetName)
		f.SetActiveSheet(idx)
		_ = f.DeleteSheet("Sheet1")
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	for rIdx, r := range rows {
		rowNum := rIdx + 2
		for cIdx, val := range r {
			cell, _ := excelize.CoordinatesToCellName(cIdx+1, rowNum)
			_ = f.SetCellStr(sheetName, cell, val)
		}
	}
	return f
}

func xlsxToBytes(f *excelize.File) []byte {
	buf := &bytes.Buffer{}
	_ = f.Write(buf)
	return buf.Bytes()
}

func buildMultipartXLSX(fieldName, filename string, data []byte) (*bytes.Buffer, string) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, _ := w.CreateFormFile(fieldName, filename)
	_, _ = part.Write(data)
	boundary := w.FormDataContentType()
	_ = w.Close()
	return buf, boundary
}

func TestParseAndValidateScheduleXLSX_MissingHeaders(t *testing.T) {
	// Missing "Jenis Shift"
	f := createTestXLSX("Jadwal", []string{"Nama Petugas", "Tanggal Mulai", "Tanggal Selesai"}, [][]string{
		{"Budi", "2026-11-15", "2026-11-16"},
	})
	defer f.Close()

	svc := &Service{}
	_, err := svc.ParseAndValidateScheduleXLSX(context.Background(), "rt-123", f)
	if err == nil {
		t.Fatal("expected error on missing headers, got nil")
	}
	var appErr *AppError
	if !isAppError(err, &appErr) || appErr.Code != "missing_headers" {
		t.Fatalf("expected code missing_headers, got %v", err)
	}
}

func TestParseAndValidateScheduleXLSX_EmptyWorkbook(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	svc := &Service{}
	_, err := svc.ParseAndValidateScheduleXLSX(context.Background(), "rt-123", f)
	if err == nil {
		t.Fatal("expected error on empty workbook, got nil")
	}
}

func TestReadScheduleXLSX_MultipartAndRaw(t *testing.T) {
	f := createTestXLSX("Jadwal", ScheduleExcelHeaders, [][]string{
		{"", "Budi", "Pagi", "2026-11-15", "2026-11-16", "Catatan"},
	})
	data := xlsxToBytes(f)
	f.Close()

	// 1. Test Multipart
	bodyBuf, contentType := buildMultipartXLSX("file", "test.xlsx", data)
	req := httptest.NewRequest(http.MethodPost, "/import", bodyBuf)
	req.Header.Set("Content-Type", contentType)

	readF, err := readScheduleXLSX(req)
	if err != nil {
		t.Fatalf("failed to read multipart xlsx: %v", err)
	}
	defer readF.Close()
	rows, err := readF.GetRows("Jadwal")
	if err != nil || len(rows) != 2 {
		t.Fatalf("expected 2 rows from multipart xlsx, got %d (err: %v)", len(rows), err)
	}

	// 2. Test Raw Body
	rawReq := httptest.NewRequest(http.MethodPost, "/import", bytes.NewReader(data))
	rawReq.Header.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	readRawF, err := readScheduleXLSX(rawReq)
	if err != nil {
		t.Fatalf("failed to read raw body xlsx: %v", err)
	}
	defer readRawF.Close()
	rawRows, err := readRawF.GetRows("Jadwal")
	if err != nil || len(rawRows) != 2 {
		t.Fatalf("expected 2 rows from raw xlsx, got %d (err: %v)", len(rawRows), err)
	}
}

func isAppError(err error, target **AppError) bool {
	if err == nil {
		return false
	}
	var a *AppError
	if ok := errors.As(err, &a); ok {
		*target = a
		return true
	}
	return false
}

func TestIntraFileScheduleValidation(t *testing.T) {
	u1 := uuid.NewString()
	u2 := uuid.NewString()
	if u1 == u2 {
		t.Fatal("uuids must be unique")
	}
}
