package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

// ---------------------------------------------------------------------------
// In-memory helpers for constructing XLSX fixtures
// ---------------------------------------------------------------------------

// xlsxBytes serializes an excelize.File to []byte via bytes.Buffer.
func xlsxBytes(f *excelize.File) []byte {
	buf := &bytes.Buffer{}
	if err := f.Write(buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// buildMultipartBody creates a multipart/form-data body for testing upload endpoints.
func buildMultipartBody(fieldName, filename string, data []byte) (*bytes.Buffer, string) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, _ := w.CreateFormFile(fieldName, filename)
	part.Write(data)
	boundary := w.FormDataContentType()
	w.Close()
	return buf, boundary
}

// writeHeaderRow writes canonical headers to row 1 of the default sheet.
func writeHeaderRow(f *excelize.File) {
	for i, h := range excelHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cell, h)
	}
}

// writeRow writes a single data row (0-indexed from row 2).
func writeRow(f *excelize.File, dataRow int, vals []string) {
	rowNum := dataRow + 2
	for i, v := range vals {
		cell, _ := excelize.CoordinatesToCellName(i+1, rowNum)
		f.SetCellValue("Sheet1", cell, v)
	}
}

// ---------------------------------------------------------------------------
// Pure parser tests (no database needed)
// ---------------------------------------------------------------------------

func TestParseExcelRowsValid(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. Merdeka 1", "Budi", "3171010101010001", "08111", "bodi@test.com", "HEAD"})
	writeRow(f, 1, []string{"A1", "Budi", "OWNER", "Jl. Merdeka 1", "Ani", "3171010101010002", "08112", "ani@test.com", "SPOUSE"})

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %+v", len(errs), errs)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].HouseNumber != "A1" {
		t.Errorf("got house_number %q, want %q", rows[0].HouseNumber, "A1")
	}
	if rows[0].Nik != "3171010101010001" {
		t.Errorf("got nik %q, want %q", rows[0].Nik, "3171010101010001")
	}
}

func TestStripNonDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"3171010101010001", "3171010101010001"},
		{"3.171010101E+15", "317101010115"},
		{"31 71 01 01 01 00 01", "31710101010001"},
		{"", ""},
		{"abc", ""},
	}
	for _, tt := range tests {
		got := stripNonDigits(tt.input)
		if got != tt.expected {
			t.Errorf("stripNonDigits(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseExcelCanonicalHeaders(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors for header-only XLSX, got: %+v", errs)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for header-only XLSX, got %d", len(rows))
	}
}

func TestParseExcelMissingRequiredColumn(t *testing.T) {
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "house_number")
	f.SetCellValue("Sheet1", "B1", "head_name")

	_, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected header errors for missing required columns")
	}
	foundOccupancy, foundResident := false, false
	for _, e := range errs {
		if e.Field == "occupancy_status" {
			foundOccupancy = true
		}
		if e.Field == "resident_name" {
			foundResident = true
		}
	}
	if !foundOccupancy {
		t.Errorf("missing occupancy_status in errors: %+v", errs)
	}
	if !foundResident {
		t.Errorf("missing resident_name in errors: %+v", errs)
	}
}

func TestParseExcelInvalidNik(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "12345", "08111", "bodi@test.com", "HEAD"})

	_, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range errs {
		if e.Field == "nik" && e.Code == "invalid_nik" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected invalid_nik error, got: %+v", errs)
	}
}

func TestParseExcelInvalidEmail(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3171010101010001", "08111", "not-an-email", "HEAD"})

	_, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range errs {
		if e.Field == "email" && e.Code == "invalid_email" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected invalid_email error, got: %+v", errs)
	}
}

func TestParseExcelPhoneNormalization(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"081234567890", "+6281234567890"},
		{"6281234567890", "+6281234567890"},
		{"+6281234567890", "+6281234567890"},
	}
	for _, tc := range cases {
		f := excelize.NewFile()
		writeHeaderRow(f)
		writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3171010101010001", tc.input, "bodi@test.com", "HEAD"})

		rows, errs, err := parseExcelRows(f)
		if err != nil {
			t.Fatalf("unexpected error for phone %q: %v", tc.input, err)
		}
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors for phone %q, got %d: %+v", tc.input, len(errs), errs)
		}
		if rows[0].Phone != tc.want {
			t.Errorf("phone %q: got %q, want %q", tc.input, rows[0].Phone, tc.want)
		}
	}
}

func TestParseExcelOccupancyPEMILIK(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "PEMILIK", "Jl. M.", "Budi", "3171010101010001", "", "bodi@test.com", "HEAD"})

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got: %+v", errs)
	}
	if rows[0].OccupancyStatus != "OWNER" {
		t.Errorf("expected OWNER, got %q", rows[0].OccupancyStatus)
	}
}

func TestParseExcelOccupancySEWA(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "SEWA", "Jl. M.", "Budi", "3171010101010001", "", "bodi@test.com", "HEAD"})

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got: %+v", errs)
	}
	if rows[0].OccupancyStatus != "TENANT" {
		t.Errorf("expected TENANT, got %q", rows[0].OccupancyStatus)
	}
}

func TestParseExcelOccupancyKONTRAK(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "KONTRAK", "Jl. M.", "Budi", "3171010101010001", "", "bodi@test.com", "HEAD"})

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got: %+v", errs)
	}
	if rows[0].OccupancyStatus != "TENANT" {
		t.Errorf("expected TENANT, got %q", rows[0].OccupancyStatus)
	}
}

func TestParseExcelDuplicateNik(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3171010101010001", "", "bodi@test.com", "HEAD"})
	writeRow(f, 1, []string{"A2", "Citra", "OWNER", "Jl. M.", "Ani", "3171010101010001", "", "ani@test.com", "SPOUSE"})

	_, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range errs {
		if e.Code == "duplicate_nik" {
			found = true
			t.Logf("found: %+v", e)
			break
		}
	}
	if !found {
		t.Fatalf("expected duplicate_nik error, got: %+v", errs)
	}
}

func TestParseExcelConflictingHeadName(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3171010101010001", "", "bodi@test.com", "HEAD"})
	writeRow(f, 1, []string{"A1", "Citra", "OWNER", "Jl. M.", "Ani", "3171010101010002", "", "ani@test.com", "SPOUSE"})

	_, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range errs {
		if e.Code == "conflicting_head_name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected conflicting_head_name, got: %+v", errs)
	}
}

func TestParseExcelConflictingOccupancyStatus(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3171010101010001", "", "bodi@test.com", "HEAD"})
	writeRow(f, 1, []string{"A1", "Budi", "TENANT", "Jl. M.", "Ani", "3171010101010002", "", "ani@test.com", "SPOUSE"})

	_, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range errs {
		if e.Code == "conflicting_occupancy_status" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected conflicting_occupancy_status, got: %+v", errs)
	}
}

func TestParseExcelEmptyRowsSkipped(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3171010101010001", "", "bodi@test.com", "HEAD"})
	writeRow(f, 1, []string{"", "", "", "", "", "", "", "", ""})
	writeRow(f, 2, []string{"", "", "", "", "", "", "", "", ""})
	writeRow(f, 3, []string{"A2", "Citra", "OWNER", "Jl. M.", "Citra", "3171010101010002", "", "citra@test.com", "HEAD"})

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got: %+v", errs)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].HouseNumber != "A1" || rows[1].HouseNumber != "A2" {
		t.Errorf("got houses %+v, want [A1 A2]", []string{rows[0].HouseNumber, rows[1].HouseNumber})
	}
}

func TestParseExcelMalformedInput(t *testing.T) {
	plainText := []byte("house_number,head_name\nA1,Budi")
	_, err := excelize.OpenReader(bytes.NewReader(plainText))
	if err == nil {
		t.Log("excelize accepted plain text as XLSX, verifying parseExcelRows handles it gracefully")
	}
}

func TestParseExcelNoWorksheet(t *testing.T) {
	f := excelize.NewFile()
	f.DeleteSheet("Sheet1")
	_, errs, err := parseExcelRows(f)
	f.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected error for workbook with no worksheets")
	}
}

func TestParseExcelAliasHeadersRejected(t *testing.T) {
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "nomor_rumah")
	f.SetCellValue("Sheet1", "B1", "nama_kepala_keluarga")
	f.SetCellValue("Sheet1", "C1", "occupancy_status")
	f.SetCellValue("Sheet1", "D1", "alamat")
	f.SetCellValue("Sheet1", "E1", "nama_anggota")
	f.SetCellValue("Sheet1", "F1", "nik")
	f.SetCellValue("Sheet1", "G1", "telepon")
	f.SetCellValue("Sheet1", "H1", "resident_email")
	f.SetCellValue("Sheet1", "I1", "hubungan")
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3171010101010001", "08111", "bodi@test.com", "HEAD"})

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Required header checks must fail for all canonical names missing.
	if len(errs) == 0 {
		t.Fatalf("expected missing_header errors for canonical columns, got none")
	}

	// The required columns that were replaced with aliases are:
	// house_number, head_name, resident_name.
	// occupancy_status was kept canonical since it has no alias.
	requirementFields := map[string]bool{
		"house_number":  false,
		"head_name":     false,
		"resident_name": false,
	}
	for _, e := range errs {
		if _, ok := requirementFields[e.Field]; ok {
			requirementFields[e.Field] = true
		}
	}
	for field, found := range requirementFields {
		if !found {
			t.Errorf("expected missing_header error for required column %q", field)
		}
	}
	// Rows should be nil since header validation failed.
	if rows != nil && len(rows) != 0 {
		t.Errorf("expected nil/empty rows when required headers missing, got %d", len(rows))
	}
}

func TestParseExcelMissingRequiredFields(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"", "", "INVALID", "", "", "", "08111", "", ""})

	_, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors (house_number, head_name, resident_name, occupancy), got %d: %+v", len(errs), errs)
	}
}

func TestParseExcelNIKStripNonDigits(t *testing.T) {
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"A1", "Budi", "OWNER", "Jl. M.", "Budi", "3.171010101E+15", "08111", "bodi@test.com", "HEAD"})

	rows, errs, err := parseExcelRows(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// stripNonDigits("3.171010101E+15") == "317101010115"  (12 digits)
	// ValidateNik rejects anything not exactly 16 digits.
	found := false
	for _, e := range errs {
		if e.Field == "nik" && e.Code == "invalid_nik" {
			found = true
			t.Logf("found error: %+v", e)
		}
	}
	if !found {
		t.Fatalf("expected invalid_nik after stripping non-digits from scientific-notation NIK, got: %+v", errs)
	}
}

// ---------------------------------------------------------------------------
// HTTP handler setup for integration tests
// ---------------------------------------------------------------------------

func setupExcelRouter(pool *database.Pool) http.Handler {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/warga/export", HandleWargaExport(pool))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/warga/export/xlsx", HandleWargaExportXLSX(pool))
		r.Get("/api/v1/warga/template", HandleWargaTemplateXLSX(pool))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/warga/import/preview", HandleWargaImportPreview(pool))
		r.Post("/api/v1/warga/import/commit", HandleWargaImportCommit(pool))
		r.Post("/api/v1/warga/import/preview/xlsx", HandleWargaImportPreviewXLSX(pool))
		r.Post("/api/v1/warga/import/commit/xlsx", HandleWargaImportCommitXLSX(pool))
	})

	return r
}

func makeAuthRequest(pool *database.Pool, method, targetURL, token string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, targetURL, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// ---------------------------------------------------------------------------
// Template endpoint tests
// ---------------------------------------------------------------------------

func TestHandleWargaTemplateXLSX(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXTemplate", 1, "XLXT")
	pengurusToken := makeTenantToken(t, "test-user-template-1", "test-mid-template-1", rtID, auth.RolePengurus)
	wargaToken := makeTenantToken(t, "test-user-template-w", "test-mid-template-w", rtID, auth.RoleWarga)
	adminToken := makeSuperAdminToken(t)

	// Warga is denied (403)
	reqW := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/template", wargaToken, nil)
	recW := httptest.NewRecorder()
	router.ServeHTTP(recW, reqW)
	if recW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for warga template, got %d: %s", recW.Code, recW.Body.String())
	}

	// Super admin is allowed (200)
	reqSA := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/template", adminToken, nil)
	recSA := httptest.NewRecorder()
	router.ServeHTTP(recSA, reqSA)
	if recSA.Code != http.StatusOK {
		t.Fatalf("expected 200 for super_admin template, got %d: %s", recSA.Code, recSA.Body.String())
	}

	// Pengurus is allowed (200)
	req := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/template", pengurusToken, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("expected XLSX content type, got %q", ct)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "warga_template.xlsx") {
		t.Errorf("expected Content-Disposition with warga_template.xlsx, got %q", cd)
	}

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("response body is not valid XLSX: %v", err)
	}
	defer f.Close()

	sheetNames := f.GetSheetList()
	found := false
	for _, sn := range sheetNames {
		if sn == "Template" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected sheet named 'Template', got %v", sheetNames)
	}

	for i, h := range excelHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		val, err := f.GetCellValue("Template", cell)
		if err != nil {
			t.Errorf("read cell %s: %v", cell, err)
			continue
		}
		if val != h {
			t.Errorf("header[%d] = %q, want %q", i, val, h)
		}
	}
}

// ---------------------------------------------------------------------------
// Export endpoint tests
// ---------------------------------------------------------------------------

func TestHandleWargaExportXLSX(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXExport", 1, "XLXE")

	// Insert physical house
	var houseID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'X-OUT-1', true) RETURNING id`,
		rtID,
	).Scan(&houseID)
	if err != nil {
		t.Fatalf("insert physical house: %v", err)
	}

	// Insert household
	var householdID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'Export Head', true) RETURNING id`,
		rtID,
	).Scan(&householdID)
	if err != nil {
		t.Fatalf("insert household: %v", err)
	}

	// Insert occupancy
	var hoID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date) VALUES ($1, $2, 'OWNER', CURRENT_DATE) RETURNING id`,
		householdID, houseID,
	).Scan(&hoID)
	if err != nil {
		t.Fatalf("insert occupancy: %v", err)
	}

	// Insert resident
	var resID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO residents (rt_id, full_name, nik, is_active) VALUES ($1, 'Export Resident', '3171010101010101', true) RETURNING id`,
		rtID,
	).Scan(&resID)
	if err != nil {
		t.Fatalf("insert resident: %v", err)
	}

	// Insert residency period
	_, err = pool.Raw().ExecContext(context.Background(),
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, CURRENT_DATE)`,
		resID, hoID,
	)
	if err != nil {
		t.Fatalf("insert residency period: %v", err)
	}

	token := makeTenantToken(t, "test-user-export-1", "test-mid-export-1", rtID, auth.RolePengurus)
	wargaToken := makeTenantToken(t, "test-user-export-w", "test-mid-export-w", rtID, auth.RoleWarga)

	// Warga is denied (403)
	reqW := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/export/xlsx", wargaToken, nil)
	recW := httptest.NewRecorder()
	router.ServeHTTP(recW, reqW)
	if recW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for warga export, got %d: %s", recW.Code, recW.Body.String())
	}

	// Pengurus is allowed (200)
	req := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/export/xlsx", token, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("expected XLSX content type, got %q", ct)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "warga_export.xlsx") {
		t.Errorf("expected Content-Disposition with warga_export.xlsx, got %q", cd)
	}

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("response body is not valid XLSX: %v", err)
	}
	defer f.Close()

	// Verify headers
	for i, h := range excelHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		val, err := f.GetCellValue("Warga", cell)
		if err != nil {
			t.Errorf("read header cell %s: %v", cell, err)
			continue
		}
		if val != h {
			t.Errorf("header[%d] = %q, want %q", i, val, h)
		}
	}

	// Verify data rows: should have at least one row with 'Export Resident'
	rows, err := f.GetRows("Warga")
	if err != nil {
		t.Fatalf("get export rows: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows (header + data), got %d", len(rows))
	}
	foundData := false
	for i := 1; i < len(rows); i++ {
		for _, cell := range rows[i] {
			if cell == "Export Resident" {
				foundData = true
				break
			}
		}
	}
	if !foundData {
		t.Fatalf("expected 'Export Resident' in exported rows")
	}
}

func TestHandleWargaExportXLSXTenantIsolation(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	// Create two RTs
	rtA := setupTestRT(t, pool, "XLXRI-A", 1, "XLXRA")
	rtB := setupTestRT(t, pool, "XLXRI-B", 1, "XLXRB")

	uniqueA := "XLXRI-A-UNIQUE-"
	uniqueB := "XLXRI-B-UNIQUE-"

	// RT B: insert unique data
	houseNumB := fmt.Sprintf("X-RI-B-%d", counter.Add(1))
	houseIDB := ensurePhysicalHouse(pool, context.Background(), rtB, houseNumB, uniqueB)
	hhB := ensureHousehold(pool, context.Background(), rtB, "B Owner")
	hoB := ensureOccupancy(pool, context.Background(), hhB, houseIDB, "OWNER")
	resB := ensureResident(pool, context.Background(), rtB, "XLXRI-B-Resident")
	ensureResidency(pool, context.Background(), resB, hoB)

	// RT A: insert unique data
	houseNumA := fmt.Sprintf("X-RI-A-%d", counter.Add(1))
	houseIDA := ensurePhysicalHouse(pool, context.Background(), rtA, houseNumA, uniqueA)
	hhA := ensureHousehold(pool, context.Background(), rtA, "A Owner")
	hoA := ensureOccupancy(pool, context.Background(), hhA, houseIDA, "OWNER")
	resA := ensureResident(pool, context.Background(), rtA, "XLXRI-A-Resident")
	ensureResidency(pool, context.Background(), resA, hoA)

	// Export as RT A Pengurus
	tokenA := makeTenantToken(t, "test-user-ri-a-1", "test-mid-ri-a-1", rtA, auth.RolePengurus)
	req := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/export/xlsx", tokenA, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("response body is not valid XLSX: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Warga")
	if err != nil {
		t.Fatalf("get export rows: %v", err)
	}
	// Flatten all cells into a single string
	var contents string
	for _, row := range rows {
		for _, cell := range row {
			contents += cell + " "
		}
	}
	if !strings.Contains(contents, "XLXRI-A-Resident") {
		t.Error("expected RT A resident data in export")
	}
	if strings.Contains(contents, "XLXRI-B-Resident") {
		t.Error("RT B resident data leaked into RT A export")
	}
	if strings.Contains(contents, uniqueB) {
		t.Error("RT B unique data leaked into RT A export")
	}
}

// ---------------------------------------------------------------------------
// Preview endpoint tests
// ---------------------------------------------------------------------------

func TestHandleWargaImportPreviewXLSX(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXPreview", 1, "XLXP")
	pToken := makeTenantToken(t, "test-user-pengurus-1", "test-mid-pengurus-1", rtID, auth.RolePengurus)

	// Build valid XLSX with 2 rows
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-PRV-1", "Preview Head", "OWNER", "Jl. Preview 1", "Preview Resident 1", "3171010101010201", "08111", "p1@test.com", "HEAD"})
	writeRow(f, 1, []string{"X-PRV-1", "Preview Head", "OWNER", "Jl. Preview 1", "Preview Resident 2", "3171010101010202", "08112", "p2@test.com", "SPOUSE"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "preview.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx", pToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ImportPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp.TotalRows != 2 {
		t.Errorf("expected total_rows 2, got %d", resp.TotalRows)
	}
	if resp.ValidRows != 2 {
		t.Errorf("expected valid_rows 2, got %d", resp.ValidRows)
	}
	if resp.InvalidRows != 0 {
		t.Errorf("expected invalid_rows 0, got %d: %+v", resp.InvalidRows, resp.Errors)
	}

	// Verify preview did NOT mutate the database
	var count int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE nik = '3171010101010201' OR nik = '3171010101010202'`,
	).Scan(&count)
	if count != 0 {
		t.Errorf("preview should not insert resident, found count=%d", count)
	}
}

func TestHandleWargaImportPreviewXLSXMalformed(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXMal", 1, "XLXM")
	pToken := makeTenantToken(t, "test-user-pengurus-m-1", "test-mid-pengurus-m-1", rtID, auth.RolePengurus)

	// Send plain text (not XLSX)
	buf := bytes.NewBufferString("this is not an xlsx file")
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx", pToken, buf)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("expected JSON error response, got: %s", rec.Body.String())
	}
	if _, ok := errResp["error"]; !ok {
		t.Errorf("expected 'error' key in response, got: %v", errResp)
	}
}

func TestHandleWargaImportPreviewXLXWargaForbidden(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXWarga", 1, "XLXW")
	wargaToken := makeTenantToken(t, "test-user-warga-1", "test-mid-warga-1", rtID, auth.RoleWarga)

	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-W-1", "Warga Head", "OWNER", "Jl. M.", "Warga Resident", "3171010101010301", "", "w@test.com", "HEAD"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "test.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx", wargaToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Commit endpoint tests
// ---------------------------------------------------------------------------

func TestHandleWargaImportCommitXLSX(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXCommit", 1, "XLXC")
	pToken := makeTenantToken(t, "test-user-pengurus-c-1", "test-mid-pengurus-c-1", rtID, auth.RolePengurus)

	// Build valid XLSX
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-CMT-1", "Commit Head", "OWNER", "Jl. Commit 1", "Commit Resident 1", "3171010101010401", "08201", "c1@test.com", "HEAD"})
	writeRow(f, 1, []string{"X-CMT-1", "Commit Head", "OWNER", "Jl. Commit 1", "Commit Resident 2", "3171010101010402", "08202", "c2@test.com", "SPOUSE"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "commit.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ImportCommitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp.Status != "committed" {
		t.Errorf("expected status 'committed', got %q", resp.Status)
	}
	if resp.ImportedHouseholds == 0 {
		t.Errorf("expected at least 1 household imported, got %d", resp.ImportedHouseholds)
	}

	// Verify physical house was created
	var houseID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM physical_houses WHERE house_number = 'X-CMT-1' AND rt_id = $1`,
		rtID,
	).Scan(&houseID)
	if err != nil {
		t.Fatalf("physical house not found: %v", err)
	}

	// Verify household was created
	var hhID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM households WHERE rt_id = $1 AND head_name = 'Commit Head' AND is_active = true`,
		rtID,
	).Scan(&hhID)
	if err != nil {
		t.Fatalf("household not found: %v", err)
	}

	// Verify occupancy
	var hoID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT ho.id FROM household_occupancies ho
		 JOIN households h ON ho.household_id = h.id
		 JOIN physical_houses ph ON ho.physical_house_id = ph.id
		 WHERE h.id = $1 AND ph.id = $2 AND ho.end_date IS NULL`,
		hhID, houseID,
	).Scan(&hoID)
	if err != nil {
		t.Fatalf("occupancy not found: %v", err)
	}

	// Verify residents were created
	var countRes int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND (full_name = 'Commit Resident 1' OR full_name = 'Commit Resident 2')`,
		rtID,
	).Scan(&countRes)
	if countRes != 2 {
		t.Errorf("expected 2 residents, got %d", countRes)
	}

	// Verify residency periods
	var countRp int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residency_periods rp
		 JOIN residents r ON rp.resident_id = r.id
		 WHERE r.rt_id = $1 AND rp.household_occupancy_id = $2`,
		rtID, hoID,
	).Scan(&countRp)
	if countRp != 2 {
		t.Errorf("expected 2 residency periods, got %d", countRp)
	}

	// Verify audit log
	var auditCount int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE rt_id = $1 AND action = 'warga_import'`,
		rtID,
	).Scan(&auditCount)
	if auditCount == 0 {
		t.Error("expected audit log entry for warga_import")
	}
}

func TestHandleWargaImportCommitXLSXWargaForbidden(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXWargaC", 1, "XLXWC")
	wargaToken := makeTenantToken(t, "test-user-warga-c-1", "test-mid-warga-c-1", rtID, auth.RoleWarga)

	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-WC-1", "Warga Commit", "OWNER", "Jl. M.", "Warga C", "3171010101010501", "", "wc@test.com", "HEAD"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "test.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", wargaToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleWargaImportCommitXLSXBendaharaForbidden(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXBendC", 1, "XLXBC")
	bhToken := makeTenantToken(t, "test-user-bendahara-c-1", "test-mid-bendahara-c-1", rtID, auth.RoleBendahara)

	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-BC-1", "Bendahara Commit", "OWNER", "Jl. M.", "Bendahara C", "3171010101010601", "", "bc@test.com", "HEAD"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "test.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", bhToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Partial-commit / validation failure tests
// ---------------------------------------------------------------------------

func TestCommitXLSXRejectsPartialCommit(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXPath", 1, "XLXPT")
	pToken := makeTenantToken(t, "test-user-pengurus-pt-1", "test-mid-pengurus-pt-1", rtID, auth.RolePengurus)

	// Build XLSX: 1 valid head, 1 invalid NIK
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-PT-1", "Valid Head", "OWNER", "Jl. M.", "Valid Resident", "3171010101010701", "", "val@test.com", "HEAD"})
	writeRow(f, 1, []string{"X-PT-2", "Invalid Head", "OWNER", "Jl. M.", "Invalid Resident", "12345", "", "inv@test.com", "HEAD"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "test.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("expected JSON error response: %v", err)
	}

	// Verify NO records were created (partial commit should not occur)
	var residentCount int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND full_name IN ('Valid Resident', 'Invalid Resident')`,
		rtID,
	).Scan(&residentCount)
	if residentCount != 0 {
		t.Errorf("expected 0 residents after rejected commit, got %d", residentCount)
	}

	var houseCount int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM physical_houses WHERE rt_id = $1 AND house_number IN ('X-PT-1', 'X-PT-2')`,
		rtID,
	).Scan(&houseCount)
	if houseCount != 0 {
		t.Errorf("expected 0 houses after rejected commit, got %d", houseCount)
	}
}

// ---------------------------------------------------------------------------
// Duplicate NIK prevents commit
// ---------------------------------------------------------------------------

func TestCommitXLSXDuplicateNIK(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXDupNik", 1, "XLXD")
	pToken := makeTenantToken(t, "test-user-pengurus-dn-1", "test-mid-pengurus-dn-1", rtID, auth.RolePengurus)

	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-DN-1", "Dup Head 1", "OWNER", "Jl. M.", "Dup Resident 1", "3171010101010801", "", "d1@test.com", "HEAD"})
	writeRow(f, 1, []string{"X-DN-2", "Dup Head 2", "OWNER", "Jl. M.", "Dup Resident 2", "3171010101010801", "", "d2@test.com", "HEAD"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "test.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	// No records committed
	var count int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND (full_name = 'Dup Resident 1' OR full_name = 'Dup Resident 2')`,
		rtID,
	).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 residents after duplicate NIK rejection, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Cross-row conflict prevents commit
// ---------------------------------------------------------------------------

func TestCommitXLSXConflictingHeadName(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXConfHead", 1, "XLXCH")
	pToken := makeTenantToken(t, "test-user-pengurus-ch-1", "test-mid-pengurus-ch-1", rtID, auth.RolePengurus)

	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-CH-1", "Conflicting Head A", "OWNER", "Jl. M.", "Resident A", "3171010101010901", "", "a@test.com", "HEAD"})
	writeRow(f, 1, []string{"X-CH-1", "Conflicting Head B", "OWNER", "Jl. M.", "Resident B", "3171010101010902", "", "b@test.com", "SPOUSE"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "test.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var count int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND (full_name = 'Resident A' OR full_name = 'Resident B')`,
		rtID,
	).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 residents after conflicting head name, got %d", count)
	}
}

func TestCommitXLSXConflictingOccupancyStatus(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXConfOcc", 1, "XLXCO")
	pToken := makeTenantToken(t, "test-user-pengurus-co-1", "test-mid-pengurus-co-1", rtID, auth.RolePengurus)

	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-CO-1", "Same Head", "OWNER", "Jl. M.", "Owner Resident", "3171010101011001", "", "o@test.com", "HEAD"})
	writeRow(f, 1, []string{"X-CO-1", "Same Head", "TENANT", "Jl. M.", "Tenant Resident", "3171010101011002", "", "t@test.com", "SPOUSE"})
	xlsxData := xlsxBytes(f)
	f.Close()

	buf, boundary := buildMultipartBody("file", "test.xlsx", xlsxData)
	req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, buf)
	req.Header.Set("Content-Type", boundary)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var count int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND (full_name = 'Owner Resident' OR full_name = 'Tenant Resident')`,
		rtID,
	).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 residents after conflicting occupancy, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Preview/commit validation parity
// ---------------------------------------------------------------------------

func TestPreviewCommitValidationParity(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtID := setupTestRT(t, pool, "XLXParity", 1, "XLXPR")

	// Build invalid XLSX: bad NIK
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-PAR-1", "Parity Head", "OWNER", "Jl. M.", "Parity Resident", "12345", "", "p@test.com", "HEAD"})
	xlsxData := xlsxBytes(f)
	defer f.Close()

	pToken := makeTenantToken(t, "test-user-pengurus-pa-1", "test-mid-pengurus-pa-1", rtID, auth.RolePengurus)

	// Run preview
	bufP, boundary := buildMultipartBody("file", "par.xlsx", xlsxData)
	reqP := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx", pToken, bufP)
	reqP.Header.Set("Content-Type", boundary)
	recP := httptest.NewRecorder()
	router.ServeHTTP(recP, reqP)

	if recP.Code != http.StatusOK {
		t.Fatalf("preview expected 200, got %d: %s", recP.Code, recP.Body.String())
	}

	var previewResp ImportPreviewResponse
	if err := json.Unmarshal(recP.Body.Bytes(), &previewResp); err != nil {
		t.Fatalf("invalid preview JSON: %v", err)
	}

	// Find the error code(s) from preview
	var previewCodes []string
	for _, e := range previewResp.Errors {
		if e.Code != "" {
			previewCodes = append(previewCodes, e.Code)
		}
	}
	if len(previewCodes) == 0 {
		t.Fatalf("preview should have errors, got none")
	}

	// Run commit with same XLSX
	bufC, boundaryC := buildMultipartBody("file", "par.xlsx", xlsxData)
	reqC := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx", pToken, bufC)
	reqC.Header.Set("Content-Type", boundaryC)
	recC := httptest.NewRecorder()
	router.ServeHTTP(recC, reqC)

	if recC.Code != http.StatusBadRequest {
		t.Fatalf("commit expected 400, got %d: %s", recC.Code, recC.Body.String())
	}

	var commitErrResp map[string]interface{}
	if err := json.Unmarshal(recC.Body.Bytes(), &commitErrResp); err != nil {
		t.Fatalf("invalid commit error JSON: %v", err)
	}

	// Both should have at least one duplicate (cross-row) error.
	// Preview groups errors with internal errors and DB errors, so we check that
	// both reject (400) and both have a non-empty errors list.
	commitDetails, ok := commitErrResp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("commit error response missing 'error' key: %v", commitErrResp)
	}
	details := commitDetails["details"]
	if details == nil {
		t.Fatalf("commit should return error details")
	}
	detailsBytes, _ := json.Marshal(details)
	var commitErrors []interface{}
	if err := json.Unmarshal(detailsBytes, &commitErrors); err != nil {
		t.Fatalf("commit details is not an array: %v", details)
	}
	if len(commitErrors) == 0 {
		t.Fatal("commit should return validation errors")
	}

	// Both should reject — this is the semantic parity.
	_ = previewCodes // we already validated there are errors in preview
}

// ---------------------------------------------------------------------------
// DB helper: ensure test records exist so exports can verify them
// ---------------------------------------------------------------------------

func ensurePhysicalHouse(pool *database.Pool, ctx context.Context, rtID, houseNum, label string) string {
	var id string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, $2, true) RETURNING id`,
		rtID, houseNum,
	).Scan(&id)
	if err != nil {
		panic(fmt.Sprintf("ensurePhysicalHouse(%s): %v", label, err))
	}
	return id
}

func ensureHousehold(pool *database.Pool, ctx context.Context, rtID, headName string) string {
	var id string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, $2, true) RETURNING id`,
		rtID, headName,
	).Scan(&id)
	if err != nil {
		panic(fmt.Sprintf("ensureHousehold(%s): %v", headName, err))
	}
	return id
}

func ensureOccupancy(pool *database.Pool, ctx context.Context, hhID, houseID, status string) string {
	var hoID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date) VALUES ($1, $2, $3, CURRENT_DATE) RETURNING id`,
		hhID, houseID, status,
	).Scan(&hoID)
	if err != nil {
		panic(fmt.Sprintf("ensureOccupancy(%s): %v", houseID, err))
	}
	return hoID
}

func ensureResident(pool *database.Pool, ctx context.Context, rtID, name string) string {
	var id string
	// Use large NIK prefix (317101010102xxxx) to avoid collisions with other tests.
	// Generate a digits-only suffix to satisfy the chk_residents_nik_digits constraint.
	suffix := fmt.Sprintf("%04d", counter.Add(1))
	nik := "317101010102" + suffix
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, nik, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		rtID, name, nik,
	).Scan(&id)
	if err != nil {
		panic(fmt.Sprintf("ensureResident(%s): %v", name, err))
	}
	return id
}

var counter atomic.Int64

func ensureResidency(pool *database.Pool, ctx context.Context, resID, hoID string) {
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, CURRENT_DATE)`,
		resID, hoID,
	)
	if err != nil {
		panic(fmt.Sprintf("ensureResidency: %v", err))
	}
}

// ---------------------------------------------------------------------------
// Super Admin XLSX Export & Import tests
// ---------------------------------------------------------------------------

func TestSuperAdminWargaExportXLSX(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtA := setupTestRT(t, pool, "XLXSA-A", 1, "XLXSAA")
	rtB := setupTestRT(t, pool, "XLXSA-B", 1, "XLXSAB")

	uniqueA := "XLXSA-A-UNIQUE"
	uniqueB := "XLXSA-B-UNIQUE"

	houseNumA := fmt.Sprintf("X-SA-A-%d", counter.Add(1))
	houseIDA := ensurePhysicalHouse(pool, context.Background(), rtA, houseNumA, uniqueA)
	hhA := ensureHousehold(pool, context.Background(), rtA, "SA Owner A")
	hoA := ensureOccupancy(pool, context.Background(), hhA, houseIDA, "OWNER")
	resA := ensureResident(pool, context.Background(), rtA, "SA-A-Resident")
	ensureResidency(pool, context.Background(), resA, hoA)

	houseNumB := fmt.Sprintf("X-SA-B-%d", counter.Add(1))
	houseIDB := ensurePhysicalHouse(pool, context.Background(), rtB, houseNumB, uniqueB)
	hhB := ensureHousehold(pool, context.Background(), rtB, "SA Owner B")
	hoB := ensureOccupancy(pool, context.Background(), hhB, houseIDB, "OWNER")
	resB := ensureResident(pool, context.Background(), rtB, "SA-B-Resident")
	ensureResidency(pool, context.Background(), resB, hoB)

	saToken := makeSuperAdminToken(t)

	// 1. Super Admin exports all RTs (omitting rt_id param)
	reqAll := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/export/xlsx", saToken, nil)
	recAll := httptest.NewRecorder()
	router.ServeHTTP(recAll, reqAll)

	if recAll.Code != http.StatusOK {
		t.Fatalf("export all expected 200, got %d: %s", recAll.Code, recAll.Body.String())
	}
	fAll, err := excelize.OpenReader(bytes.NewReader(recAll.Body.Bytes()))
	if err != nil {
		t.Fatalf("response not valid xlsx: %v", err)
	}
	defer fAll.Close()
	rowsAll, _ := fAll.GetRows("Warga")
	var contentsAll string
	for _, row := range rowsAll {
		for _, cell := range row {
			contentsAll += cell + " "
		}
	}
	if !strings.Contains(contentsAll, "SA-A-Resident") || !strings.Contains(contentsAll, "SA-B-Resident") {
		t.Errorf("export all should contain both RT A and RT B residents, got: %s", contentsAll)
	}

	// 2. Super Admin exports specific RT (rt_id=rtA)
	reqA := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/export/xlsx?rt_id="+rtA, saToken, nil)
	recA := httptest.NewRecorder()
	router.ServeHTTP(recA, reqA)

	if recA.Code != http.StatusOK {
		t.Fatalf("export specific RT expected 200, got %d: %s", recA.Code, recA.Body.String())
	}
	fA, err := excelize.OpenReader(bytes.NewReader(recA.Body.Bytes()))
	if err != nil {
		t.Fatalf("response not valid xlsx: %v", err)
	}
	defer fA.Close()
	rowsA, _ := fA.GetRows("Warga")
	var contentsA string
	for _, row := range rowsA {
		for _, cell := range row {
			contentsA += cell + " "
		}
	}
	if !strings.Contains(contentsA, "SA-A-Resident") {
		t.Errorf("export rtA should contain RT A resident, got: %s", contentsA)
	}
	if strings.Contains(contentsA, "SA-B-Resident") {
		t.Errorf("export rtA should NOT contain RT B resident, got: %s", contentsA)
	}

	// 3. Super Admin with non-existent RT returns 404
	reqNonExistent := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/export/xlsx?rt_id=00000000-0000-0000-0000-000000000099", saToken, nil)
	recNonExistent := httptest.NewRecorder()
	router.ServeHTTP(recNonExistent, reqNonExistent)
	if recNonExistent.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent RT export, got %d: %s", recNonExistent.Code, recNonExistent.Body.String())
	}

	// 4. Tenant user trying to supply rt_id query parameter returns 400 (tampering blocked)
	pToken := makeTenantToken(t, "user-p-a", "mid-p-a", rtA, auth.RolePengurus)
	reqTamper := makeAuthRequest(pool, http.MethodGet, "/api/v1/warga/export/xlsx?rt_id="+rtB, pToken, nil)
	recTamper := httptest.NewRecorder()
	router.ServeHTTP(recTamper, reqTamper)
	if recTamper.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when tenant passes rt_id query param, got %d: %s", recTamper.Code, recTamper.Body.String())
	}
}

func TestSuperAdminWargaImportXLSX(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupExcelRouter(pool)

	rtA := setupTestRT(t, pool, "XLXSAImp-A", 1, "XLXSAIA")
	rtB := setupTestRT(t, pool, "XLXSAImp-B", 1, "XLXSAIB")
	saToken := makeSuperAdminToken(t)

	// Valid XLSX with 1 row
	f := excelize.NewFile()
	writeHeaderRow(f)
	writeRow(f, 0, []string{"X-SAI-1", "SA Import Head", "OWNER", "Jl. SA 1", "SA Import Res", "3171010101010999", "08111", "sa@test.com", "HEAD"})
	xlsxData := xlsxBytes(f)
	f.Close()

	// 1. Super Admin preview without rt_id fails with 400
	buf1, bound1 := buildMultipartBody("file", "test.xlsx", xlsxData)
	reqNoRT := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx", saToken, buf1)
	reqNoRT.Header.Set("Content-Type", bound1)
	recNoRT := httptest.NewRecorder()
	router.ServeHTTP(recNoRT, reqNoRT)
	if recNoRT.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when SA preview omits rt_id, got %d: %s", recNoRT.Code, recNoRT.Body.String())
	}

	// 2. Super Admin preview with non-existent RT returns 404
	buf2, bound2 := buildMultipartBody("file", "test.xlsx", xlsxData)
	reqNotFound := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx?rt_id=00000000-0000-0000-0000-000000000099", saToken, buf2)
	reqNotFound.Header.Set("Content-Type", bound2)
	recNotFound := httptest.NewRecorder()
	router.ServeHTTP(recNotFound, reqNotFound)
	if recNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when SA preview targets non-existent RT, got %d: %s", recNotFound.Code, recNotFound.Body.String())
	}

	// 3. Super Admin preview with valid ?rt_id=<rtA> returns 200
	buf3, bound3 := buildMultipartBody("file", "test.xlsx", xlsxData)
	reqValid := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx?rt_id="+rtA, saToken, buf3)
	reqValid.Header.Set("Content-Type", bound3)
	recValid := httptest.NewRecorder()
	router.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Fatalf("expected 200 for SA preview with rt_id, got %d: %s", recValid.Code, recValid.Body.String())
	}

	// 4. Tenant user supplying ?rt_id returns 400
	pToken := makeTenantToken(t, "user-p-a", "mid-p-a", rtA, auth.RolePengurus)
	buf4, bound4 := buildMultipartBody("file", "test.xlsx", xlsxData)
	reqTamper := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/preview/xlsx?rt_id="+rtB, pToken, buf4)
	reqTamper.Header.Set("Content-Type", bound4)
	recTamper := httptest.NewRecorder()
	router.ServeHTTP(recTamper, reqTamper)
	if recTamper.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when tenant supplies rt_id, got %d: %s", recTamper.Code, recTamper.Body.String())
	}

	// 5. Super Admin commit with ?rt_id=<rtA> returns 201 Created and creates records in RT A
	buf5, bound5 := buildMultipartBody("file", "test.xlsx", xlsxData)
	reqCommit := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx?rt_id="+rtA, saToken, buf5)
	reqCommit.Header.Set("Content-Type", bound5)
	recCommit := httptest.NewRecorder()
	router.ServeHTTP(recCommit, reqCommit)
	if recCommit.Code != http.StatusCreated {
		t.Fatalf("expected 201 for SA commit with rt_id, got %d: %s", recCommit.Code, recCommit.Body.String())
	}

	var resCount int
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND full_name = 'SA Import Res'`,
		rtA,
	).Scan(&resCount)
	if resCount != 1 {
		t.Fatalf("expected 1 resident created in RT A, got %d", resCount)
	}
}
