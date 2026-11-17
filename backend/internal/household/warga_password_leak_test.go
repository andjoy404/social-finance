package household

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"social-finance/internal/auth"
	"social-finance/internal/database"
)

// setupWargaPreviewRouter creates a chi router with auth middleware for preview only.
func setupWargaPreviewRouter(pool *database.Pool) http.Handler {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/warga/import/preview", HandleWargaImportPreview(pool))
	})

	return r
}

// TestPreviewPasswordNeverLeaked verifies that plaintext passwords never appear
// in preview responses, error responses, or any nested JSON field.
func TestPreviewPasswordNeverLeaked(t *testing.T) {
	pool := testPool(t)
	router := setupWargaPreviewRouter(pool)
	rtID := setupTestRT(t, pool, "LeakTest RT", 1, "LT1")
	tokenPengurus := makeTenantToken(t, "user-lt", "mid-lt", rtID, auth.RolePengurus)

	suffix := uuid.NewString()
	secretPassword := "SuperSecret123!"

	t.Run("CSV preview does not contain password", func(t *testing.T) {
		csvContent := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-%s,Head Test,OWNER,Jl. Test,Test User,3171010000001001,0811,%s@test.com,2024-01-15,ketua,%s
`, suffix, suffix, secretPassword)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", strings.NewReader(csvContent))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if strings.Contains(body, secretPassword) {
			t.Errorf("CRITICAL: password %q found in preview response body", secretPassword)
		}

		// Verify response structure: preview_data should contain PasswordHint, not Password.
		var preview ImportPreviewResponse
		if err := json.NewDecoder(strings.NewReader(body)).Decode(&preview); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if len(preview.PreviewData) != 1 {
			t.Fatalf("expected 1 preview row, got %d", len(preview.PreviewData))
		}

		row := preview.PreviewData[0]
		if row.PasswordHint != "password will be set" {
			t.Errorf("expected PasswordHint 'password will be set', got %q", row.PasswordHint)
		}

		rowJSON, _ := json.Marshal(row)
		rowJSONStr := string(rowJSON)
		// password_hint is expected — check only that actual password value is not present.
		if strings.Contains(rowJSONStr, secretPassword) {
			t.Errorf("actual password value leaked in preview row: %s", rowJSONStr)
		}
	})

	t.Run("CSV preview with empty password shows correct hint", func(t *testing.T) {
		csvContent := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-%s,Head Test,OWNER,Jl. Test,Test User,3171010000001002,0811,%s@test.com,2024-01-15,ketua,
`, suffix, suffix)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", strings.NewReader(csvContent))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var preview ImportPreviewResponse
		if err := json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&preview); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if len(preview.PreviewData) != 1 {
			t.Fatalf("expected 1 preview row, got %d", len(preview.PreviewData))
		}

		row := preview.PreviewData[0]
		if row.PasswordHint != "password unchanged" {
			t.Errorf("expected PasswordHint 'password unchanged', got %q", row.PasswordHint)
		}
	})

	t.Run("Excel preview does not contain password", func(t *testing.T) {
		// Build an xlsx with data including password in column K.
		f, err := buildTemplateXLSX()
		if err != nil {
			t.Fatalf("build template: %v", err)
		}

		sheetName := "Template"
		dataRow := []string{
			"H-1", "Test Head", "OWNER", "Jl. Test",
			"Excel User", "3171010000001003", "0811",
			"excel@test.com", "2024-06-01", "ketua", secretPassword,
		}
		for ci, val := range dataRow {
			cell, _ := excelize.CoordinatesToCellName(ci+1, 2)
			if err := f.SetCellValue(sheetName, cell, val); err != nil {
				t.Fatalf("set cell %s: %v", cell, err)
			}
		}

		// Write xlsx to buffer.
		xlsxBuf := &bytes.Buffer{}
		if err := f.Write(xlsxBuf); err != nil {
			t.Fatalf("write xlsx: %v", err)
		}
		f.Close()

		// Wrap in multipart form.
		formBuf := &bytes.Buffer{}
		writer := multipart.NewWriter(formBuf)
		part, err := writer.CreateFormFile("file", "warga_test.xlsx")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		part.Write(xlsxBuf.Bytes())
		writer.Close()

		r := chi.NewRouter()
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth)
			r.Use(auth.RequireRole(auth.RolePengurus))
			r.Post("/api/v1/warga/import/preview", HandleWargaImportPreviewXLSX(pool))
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", formBuf)
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if strings.Contains(body, secretPassword) {
			t.Errorf("CRITICAL: password %q found in Excel preview response body", secretPassword)
		}

		// Verify no nested "Password" field in JSON.
		if strings.Contains(strings.ToLower(body), `"password":`) {
			t.Errorf("CRITICAL: raw password field found in Excel preview JSON")
		}

		var preview ImportPreviewResponse
		if err := json.NewDecoder(strings.NewReader(body)).Decode(&preview); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if len(preview.PreviewData) != 1 {
			t.Fatalf("expected 1 preview row, got %d", len(preview.PreviewData))
		}

		row := preview.PreviewData[0]
		if row.PasswordHint != "password will be set" {
			t.Errorf("expected PasswordHint 'password will be set', got %q", row.PasswordHint)
		}
	})

	t.Run("Validation error response does not contain password", func(t *testing.T) {
		// Upload a CSV with an invalid password (too short) to trigger a validation error.
		csvContent := "house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password\nH-ERR,Head,OWNER,Jl. Test,Err User,3171010000001004,0811,err@test.com,2024-01-15,ketua,short\n"

		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", strings.NewReader(csvContent))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Should succeed with validation errors returned.
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 (with errors), got %d: %s", rec.Code, rec.Body.String())
		}

		var preview ImportPreviewResponse
		if err := json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&preview); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		// The error code mentions "password_too_short" but the error MESSAGE does NOT
		// contain the actual password value.
		for _, e := range preview.Errors {
			if e.Code == "password_too_short" && strings.Contains(e.Message, "short") {
				t.Errorf("CRITICAL: short password value leaked in validation error message: %s", e.Message)
			}
		}
	})
}

// TestCSVImportSuperAdminViaResolveRTID verifies that resolveImportRTID is used
// for CSV preview and commit, supporting super_admin via rt_id query param
// and blocking tenant tampering.
func TestCSVImportSuperAdminViaResolveRTID(t *testing.T) {
	pool := testPool(t)
	router := setupWargaPreviewRouter(pool)
	rtA := setupTestRT(t, pool, "SuperAdmin CSV RT A", 1, "SA1")
	rtB := setupTestRT(t, pool, "SuperAdmin CSV RT B", 2, "SA2")

	suffix := uuid.NewString()

	t.Run("Tenant user rejecting rt_id query parameter returns 400", func(t *testing.T) {
		tokenPengurus := makeTenantToken(t, "user-tenant", "mid-tenant", rtA, auth.RolePengurus)
		csvContent := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email
H-%s,Head,OWNER,Jl. Test,Tenant User,3171010000002001,0811,%s@test.com
`, suffix, suffix)

		// Tamper: add rt_id query param to target a different RT.
		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview?rt_id="+rtB, strings.NewReader(csvContent))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Must reject — tenant users cannot supply rt_id.
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for tenant tampering, got %d: %s", rec.Code, rec.Body.String())
		} else {
			body := rec.Body.String()
			if !strings.Contains(strings.ToLower(body), "rt_id") {
				t.Errorf("expected error mentioning rt_id, got: %s", body)
			}
		}
	})

	t.Run("Super admin preview via rt_id query param works", func(t *testing.T) {
		adminToken := makeSuperAdminToken(t)

		csvContent := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email
H-%s,SA Head,OWNER,Jl. Test,SA User,3171010000002002,0811,%s@test.com
`, suffix, suffix)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview?rt_id="+rtA, strings.NewReader(csvContent))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Should succeed — super_admin can target valid RT via query param.
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for super_admin preview, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Super admin preview rejects invalid rt_id", func(t *testing.T) {
		adminToken := makeSuperAdminToken(t)
		csvContent := "house_number,head_name,occupancy_status,address,resident_name,nik,phone,email\nH-X,Head,OWNER,Jl. Test,User,3171010000002003,0811,x@test.com\n"

		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview?rt_id=00000000-0000-0000-0000-000000000000", strings.NewReader(csvContent))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for invalid rt_id, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestCommitPasswordNeverLeaked verifies that commit response never contains password.
func TestCommitPasswordNeverLeaked(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)
	rtID := setupTestRT(t, pool, "CommitLeakTest RT", 1, "CLT1")
	tokenPengurus := makeTenantToken(t, "user-clt", "mid-clt", rtID, auth.RolePengurus)

	suffix := uuid.NewString()
	secretPassword := "SecretCommit1!"

	csvContent := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-%s,Head,OWNER,Jl. Test,Commit User,3171010000003001,0811,%s@test.com,2024-01-15,bendahara,%s
`, suffix, suffix, secretPassword)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvContent))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, secretPassword) {
		t.Errorf("CRITICAL: password %q found in commit response body", secretPassword)
	}

	var resp ImportCommitResponse
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
		t.Fatalf("decode commit response: %v", err)
	}

	if resp.Status != "committed" {
		t.Errorf("expected status 'committed', got %q", resp.Status)
	}
}

// TestParseWargaCSVStillAcceptsPasswordColumn verifies that parser still reads
// the password column (for internal use) even though it is not returned in responses.
func TestParseWargaCSVStillAcceptsPasswordColumn(t *testing.T) {
	csvContent := `house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-1,Head,OWNER,Jl. Test,Test User,3171010000004001,0811,test@test.com,2024-01-15,ketua,TestPassword123!
`

	rows, errs, err := ParseWargaCSV(strings.NewReader(csvContent))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("parse errors: %+v", errs)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Password != "TestPassword123!" {
		t.Errorf("parser should still read password for internal use: got %q", rows[0].Password)
	}

	// But preview conversion must NOT include it.
	preview := rowsToPreview(rows)
	if len(preview) != 1 {
		t.Fatalf("expected 1 preview row, got %d", len(preview))
	}
	previewJSON, _ := json.Marshal(preview[0])
	previewJSONStr := string(previewJSON)

	if strings.Contains(previewJSONStr, "TestPassword123!") {
		t.Errorf("password leaked in preview: %s", previewJSONStr)
	}
	if !strings.Contains(previewJSONStr, "password will be set") {
		t.Errorf("expected PasswordHint in preview: %s", previewJSONStr)
	}
}
