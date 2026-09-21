package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

func setupWargaIORouter(pool *database.Pool) http.Handler {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/warga/export", HandleWargaExport(pool))
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/warga/import/preview", HandleWargaImportPreview(pool))
		r.Post("/api/v1/warga/import/commit", HandleWargaImportCommit(pool))
	})

	return r
}

func TestParseWargaCSVValidation(t *testing.T) {
	t.Run("Valid CSV parsing with OWNER and TENANT", func(t *testing.T) {
		csvContent := `house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
A1,Budi,Pemilik,Jl. Merdeka 1,Budi,3171010101010001,08111,budi@example.com,HEAD
A1,Budi,OWNER,Jl. Merdeka 1,Ani,3171010101010002,08112,ani@example.com,SPOUSE
B2,Citra,Kontrak,Jl. Merdeka 2,Citra,3171010101010003,08222,citra@example.com,HEAD
`
		rows, rowErrors, err := ParseWargaCSV(strings.NewReader(csvContent))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rowErrors) != 0 {
			t.Fatalf("expected 0 row errors, got %d: %+v", len(rowErrors), rowErrors)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		if rows[0].OccupancyStatus != "OWNER" || rows[2].OccupancyStatus != "TENANT" {
			t.Errorf("expected normalized OWNER and TENANT status, got %s and %s", rows[0].OccupancyStatus, rows[2].OccupancyStatus)
		}
	})

	t.Run("Missing required header columns", func(t *testing.T) {
		csvContent := `house_number,head_name,occupancy_status
A1,Budi,OWNER
`
		_, rowErrors, err := ParseWargaCSV(strings.NewReader(csvContent))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rowErrors) == 0 {
			t.Fatalf("expected header errors for missing resident_name")
		}
		foundResidentName := false
		for _, e := range rowErrors {
			if e.Field == "resident_name" {
				foundResidentName = true
			}
		}
		if !foundResidentName {
			t.Errorf("expected missing resident_name error in %+v", rowErrors)
		}
	})

	t.Run("Invalid occupancy status and missing required fields", func(t *testing.T) {
		csvContent := `house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
,,INVALID,Jl. Merdeka 1,,,08111,,HEAD
`
		rows, rowErrors, err := ParseWargaCSV(strings.NewReader(csvContent))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 row parsed")
		}
		if len(rowErrors) < 3 {
			t.Fatalf("expected at least 3 row errors, got %d: %+v", len(rowErrors), rowErrors)
		}
	})

	t.Run("Internal conflicting household house assignment", func(t *testing.T) {
		csvContent := `house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
A1,Budi,OWNER,Jl. Merdeka 1,Budi,3171010101010001,08111,budi@example.com,HEAD
A1,Citra,OWNER,Jl. Merdeka 1,Ani,3171010101010002,08112,ani@example.com,SPOUSE
`
		_, rowErrors, err := ParseWargaCSV(strings.NewReader(csvContent))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		foundConflict := false
		for _, e := range rowErrors {
			if e.Code == "conflicting_head_name" {
				foundConflict = true
				break
			}
		}
		if !foundConflict {
			t.Errorf("expected conflicting_head_name error, got %+v", rowErrors)
		}
	})
}

func TestWargaImportPreviewAndCommitFlow(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupWargaIORouter(pool)

	rtID := setupTestRT(t, pool, "Warga IO RT Flow", 1, "WIO1")

	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: "00000000-0000-0000-0000-000000000001",
		MID:    "mid-p",
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})
	tokenWarga := makeToken(t, auth.TokenClaims{
		UserID: "00000000-0000-0000-0000-000000000002",
		MID:    "mid-w",
		RTID:   rtID,
		Role:   string(auth.RoleWarga),
	})

	nowNano := time.Now().UnixNano()
	house1 := fmt.Sprintf("H-%d-A", nowNano%10000)
	house2 := fmt.Sprintf("H-%d-B", nowNano%10000)

	csvValid := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
%s,Budi Santoso,OWNER,Jl. Melati No. 1,Budi Santoso,3171010000000001,0811000001,budi@test.com,HEAD
%s,Budi Santoso,OWNER,Jl. Melati No. 1,Siti Rahayu,3171010000000002,0811000002,siti@test.com,SPOUSE
%s,Ahmad Dahlan,TENANT,Jl. Mawar No. 2,Ahmad Dahlan,3171010000000003,0822000001,ahmad@test.com,HEAD
`, house1, house1, house2)

	// 1. Authorization: Warga cannot preview or commit
	t.Run("Warga cannot preview import", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", strings.NewReader(csvValid))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for Warga, got %d", rec.Code)
		}
	})

	// 2. Pengurus preview valid CSV
	t.Run("Pengurus previews valid CSV", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", strings.NewReader(csvValid))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var preview ImportPreviewResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
			t.Fatalf("unmarshal preview: %v", err)
		}
		if preview.TotalRows != 3 || preview.ValidRows != 3 || preview.InvalidRows != 0 {
			t.Errorf("unexpected preview counts: %+v", preview)
		}
		if len(preview.Errors) != 0 {
			t.Errorf("expected 0 errors, got: %+v", preview.Errors)
		}
	})

	// 3. Pengurus commits valid CSV
	t.Run("Pengurus commits valid CSV", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvValid))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}
		var commitResp ImportCommitResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &commitResp); err != nil {
			t.Fatalf("unmarshal commit resp: %v", err)
		}
		if commitResp.ImportedHouseholds != 2 || commitResp.ImportedResidents != 3 {
			t.Errorf("unexpected commit counts: %+v", commitResp)
		}
	})

	// 4. Export CSV and verify contents
	t.Run("Export CSV contains imported warga", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/warga/export", nil)
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for export, got %d: %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, house1) || !strings.Contains(body, "Budi Santoso") || !strings.Contains(body, "Siti Rahayu") {
			t.Errorf("export missing expected records: %s", body)
		}
		if !strings.Contains(body, house2) || !strings.Contains(body, "Ahmad Dahlan") {
			t.Errorf("export missing expected records: %s", body)
		}
	})

	// 5. Duplicate occupied house conflict detection on next import
	t.Run("Detects conflict when house is occupied by another household", func(t *testing.T) {
		csvConflict := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
%s,New Head,OWNER,Jl. Melati No. 1,New Resident,3171010000000009,08999999,new@test.com,HEAD
`, house1)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", strings.NewReader(csvConflict))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var preview ImportPreviewResponse
		json.Unmarshal(rec.Body.Bytes(), &preview)
		if preview.InvalidRows == 0 {
			t.Errorf("expected conflict on occupied house %s", house1)
		}
		foundOccupied := false
		for _, e := range preview.Errors {
			if e.Code == "house_already_occupied" {
				foundOccupied = true
				break
			}
		}
		if !foundOccupied {
			t.Errorf("expected house_already_occupied error code, got %+v", preview.Errors)
		}
	})
}

func TestWargaImportMultipartForm(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupWargaIORouter(pool)

	rtID := setupTestRT(t, pool, "Warga IO Multipart RT", 1, "WIOMP")

	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: "00000000-0000-0000-0000-000000000001",
		MID:    "mid-p",
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})

	nowNano := time.Now().UnixNano()
	csvData := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
H-MP-%d,Doni,OWNER,Jl. Melati,Doni,3171010000000010,0812,doni@test.com,HEAD
`, nowNano%10000)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "warga.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte(csvData))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview", body)
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for multipart preview, got %d: %s", rec.Code, rec.Body.String())
	}
	var preview ImportPreviewResponse
	json.Unmarshal(rec.Body.Bytes(), &preview)
	if preview.ValidRows != 1 || preview.TotalRows != 1 {
		t.Errorf("unexpected preview counts: %+v", preview)
	}
}

func TestWargaExportTenantIsolation(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupWargaIORouter(pool)

	rtA := setupTestRT(t, pool, "Export RT A", 1, "EXPA")
	rtB := setupTestRT(t, pool, "Export RT B", 2, "EXPB")

	// Seed resident in RT A
	nowNano := time.Now().UnixNano()
	csvA := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
H-A-%d,Warga RT A,OWNER,Jl. A,Warga RT A,3171010000000021,0811,wargaa@test.com,HEAD
`, nowNano%10000)

	// Seed resident in RT B
	csvB := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
H-B-%d,Warga RT B,OWNER,Jl. B,Warga RT B,3171010000000022,0822,wargab@test.com,HEAD
`, nowNano%10000)

	rowsA, _, _ := ParseWargaCSV(strings.NewReader(csvA))
	_, err := CommitWargaImport(context.Background(), pool, rtA, "00000000-0000-0000-0000-000000000001", rowsA)
	if err != nil {
		t.Fatalf("commit RT A: %v", err)
	}

	rowsB, _, _ := ParseWargaCSV(strings.NewReader(csvB))
	_, err = CommitWargaImport(context.Background(), pool, rtB, "00000000-0000-0000-0000-000000000002", rowsB)
	if err != nil {
		t.Fatalf("commit RT B: %v", err)
	}

	tokenPengurusA := makeToken(t, auth.TokenClaims{
		UserID: "00000000-0000-0000-0000-000000000001",
		MID:    "mid-a",
		RTID:   rtA,
		Role:   string(auth.RolePengurus),
	})

	// RT A exports
	req := httptest.NewRequest(http.MethodGet, "/api/v1/warga/export", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPengurusA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("export RT A failed: %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Warga RT A") {
		t.Errorf("expected RT A warga in export, got: %s", body)
	}
	if strings.Contains(body, "Warga RT B") {
		t.Errorf("CRITICAL: RT B warga leaked in RT A export! Body: %s", body)
	}
}
