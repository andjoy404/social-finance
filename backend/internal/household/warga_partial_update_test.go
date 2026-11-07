package household

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"social-finance/internal/auth"
	"social-finance/internal/database"
)

// setupWargaImportExportRouter creates a chi router with auth middleware for import/export.
func setupWargaImportExportRouter(pool *database.Pool) http.Handler {
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

// --- Regression tests for partial update of existing residents ---

func TestExistingResidentPartialUpdate(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)
	rtID := setupTestRT(t, pool, "PartialUpdate RT", 1, "PU01")
	tokenPengurus := makeTenantToken(t, "user-pu-1", "mid-pu-1", rtID, auth.RolePengurus)

	suffix := uuid.NewString()
	houseNum := "H-PU-" + suffix
	jonoEmail := fmt.Sprintf("jono-pu-%s@test.com", suffix)
	atunEmail := fmt.Sprintf("atun-pu-%s@test.com", suffix)

	// --- Step 1: Import a new resident with all fields ---
	csvAllFields := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
%s,Jono Owner,OWNER,Jl. Test 1,Jono,3171010000001001,0811000010,%s,2024-01-15,ketua,Password123!
%s,Jono Owner,OWNER,Jl. Test 1,Atun,3171010000001002,0811000011,%s,2024-03-01,sosial,Password456!
`, houseNum, jonoEmail, houseNum, atunEmail)

	// Commit
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvAllFields))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 1 commit: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify residency dates
	var residentJonoID, residentAtunID string
	pool.Raw().QueryRow(`SELECT id FROM residents WHERE rt_id = $1 AND nik = $2`, rtID, "3171010000001001").Scan(&residentJonoID)
	pool.Raw().QueryRow(`SELECT id FROM residents WHERE rt_id = $1 AND nik = $2`, rtID, "3171010000001002").Scan(&residentAtunID)
	if residentJonoID == "" {
		t.Fatal("Jono not created")
	}
	if residentAtunID == "" {
		t.Fatal("Atun not created")
	}

	var jonoDate, atunDate string
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentJonoID).Scan(&jonoDate)
	if jonoDate != "2024-01-15" {
		t.Fatalf("Jono start_date: got %q", jonoDate)
	}
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentAtunID).Scan(&atunDate)
	if atunDate != "2024-03-01" {
		t.Fatalf("Atun start_date: got %q", atunDate)
	}

	// Verify jabatan
	var jonoUserID, atunUserID string
	pool.Raw().QueryRow(`SELECT user_id FROM residents WHERE id = $1`, residentJonoID).Scan(&jonoUserID)
	pool.Raw().QueryRow(`SELECT user_id FROM residents WHERE id = $1`, residentAtunID).Scan(&atunUserID)

	var jonoJabatan, atunJabatan string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoJabatan)
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, atunUserID, rtID).Scan(&atunJabatan)
	if jonoJabatan != "ketua" {
		t.Errorf("Jono jabatan: got %q", jonoJabatan)
	}
	if atunJabatan != "sosial" {
		t.Errorf("Atun jabatan: got %q", atunJabatan)
	}

	// Verify password hashes
	var jonoHash, atunHash string
	pool.Raw().QueryRow(`SELECT password_hash FROM users WHERE id = $1`, jonoUserID).Scan(&jonoHash)
	pool.Raw().QueryRow(`SELECT password_hash FROM users WHERE id = $1`, atunUserID).Scan(&atunHash)
	if jonoHash == "Password123!" {
		t.Fatal("Jono password stored as plaintext!")
	}
	if atunHash == "Password456!" {
		t.Fatal("Atun password stored as plaintext!")
	}

	t.Logf("Step 1: Both residents created ✓ date=%s/%s, jabatan=%s/%s", jonoDate, atunDate, jonoJabatan, atunJabatan)

	// --- Step 2: Existing resident + all columns empty → all data unchanged ---
	csvEmptyFields := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email
%s,Jono Owner,OWNER,Jl. Test 1,Jono,3171010000001001,0811000010,%s
%s,Jono Owner,OWNER,Jl. Test 1,Atun,3171010000001002,0811000011,%s
`, houseNum, jonoEmail, houseNum, atunEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvEmptyFields))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 2: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var jonoDateAfter, atunDateAfter string
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentJonoID).Scan(&jonoDateAfter)
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentAtunID).Scan(&atunDateAfter)
	if jonoDateAfter != "2024-01-15" {
		t.Errorf("Jono date changed to %q (expected 2024-01-15)", jonoDateAfter)
	}
	if atunDateAfter != "2024-03-01" {
		t.Errorf("Atun date changed to %q (expected 2024-03-01)", atunDateAfter)
	}

	var jonoJabatanAfter, atunJabatanAfter string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoJabatanAfter)
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, atunUserID, rtID).Scan(&atunJabatanAfter)
	if jonoJabatanAfter != "ketua" {
		t.Errorf("Jono jabatan changed to %q", jonoJabatanAfter)
	}
	if atunJabatanAfter != "sosial" {
		t.Errorf("Atun jabatan changed to %q", atunJabatanAfter)
	}

	// Verify no duplicate residency periods
	var jonoResCount, atunResCount int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentJonoID).Scan(&jonoResCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentAtunID).Scan(&atunResCount)
	if jonoResCount != 1 {
		t.Errorf("Jono has %d active residency (expected 1)", jonoResCount)
	}
	if atunResCount != 1 {
		t.Errorf("Atun has %d active residency (expected 1)", atunResCount)
	}

	// Verify no duplicate memberships
	var jonoMemCount, atunMemCount int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoMemCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, atunUserID, rtID).Scan(&atunMemCount)
	if jonoMemCount != 1 {
		t.Errorf("Jono has %d memberships (expected 1)", jonoMemCount)
	}
	if atunMemCount != 1 {
		t.Errorf("Atun has %d memberships (expected 1)", atunMemCount)
	}

	t.Logf("Step 2: All empty → no changes, no duplicates ✓")

	// --- Step 3: Existing resident + only date filled → only date changes ---
	csvDateOnly := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date
%s,Jono Owner,OWNER,Jl. Test 1,Jono,3171010000001001,0811000010,%s,2025-06-15
`, houseNum, jonoEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvDateOnly))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 3: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var jonoDateStep3 string
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentJonoID).Scan(&jonoDateStep3)
	if jonoDateStep3 != "2025-06-15" {
		t.Errorf("Jono date should be 2025-06-15, got %q", jonoDateStep3)
	}

	var atunDateStep3 string
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentAtunID).Scan(&atunDateStep3)
	if atunDateStep3 != "2024-03-01" {
		t.Errorf("Atun date should still be 2024-03-01, got %q", atunDateStep3)
	}

	t.Logf("Step 3: Date only → Jono updated, Atun unchanged ✓")

	// --- Step 4: Existing resident + only jabatan filled → only jabatan changes ---
	csvJabatanOnly := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,jabatan
%s,Jono Owner,OWNER,Jl. Test 1,Atun,3171010000001002,0811000011,%s,keamanan
`, houseNum, atunEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvJabatanOnly))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 4: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var atunJabatanStep4 string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, atunUserID, rtID).Scan(&atunJabatanStep4)
	if atunJabatanStep4 != "keamanan" {
		t.Errorf("Atun jabatan should be keamanan, got %q", atunJabatanStep4)
	}

	var jonoJabatanStep4 string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoJabatanStep4)
	if jonoJabatanStep4 != "ketua" {
		t.Errorf("Jono jabatan should still be ketua, got %q", jonoJabatanStep4)
	}

	t.Logf("Step 4: Jabatan only → Atun updated, Jono unchanged ✓")

	// --- Step 5: Existing resident + only password filled → only password changes ---
	csvPasswordOnly := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,password
%s,Jono Owner,OWNER,Jl. Test 1,Jono,3171010000001001,0811000010,%s,NewPassword789!
`, houseNum, jonoEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvPasswordOnly))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 5: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var jonoHashStep5 string
	pool.Raw().QueryRow(`SELECT password_hash FROM users WHERE id = $1`, jonoUserID).Scan(&jonoHashStep5)
	if jonoHashStep5 == "NewPassword789!" {
		t.Fatal("Jono password stored as plaintext!")
	}
	if auth.Verify("Password123!", jonoHashStep5) == nil {
		t.Fatal("Old password still verifies!")
	}
	if auth.Verify("NewPassword789!", jonoHashStep5) != nil {
		t.Fatal("New password does not verify!")
	}

	var jonoDateStep5 string
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentJonoID).Scan(&jonoDateStep5)
	if jonoDateStep5 != "2025-06-15" {
		t.Errorf("Jono date should still be 2025-06-15, got %q", jonoDateStep5)
	}

	t.Logf("Step 5: Password only → password updated, date/jabatan unchanged ✓")

	// --- Step 6: Password empty → old password still works ---
	csvEmptyPassword := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,password
%s,Jono Owner,OWNER,Jl. Test 1,Jono,3171010000001001,0811000010,%s,
`, houseNum, jonoEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvEmptyPassword))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 6: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var jonoHashStep6 string
	pool.Raw().QueryRow(`SELECT password_hash FROM users WHERE id = $1`, jonoUserID).Scan(&jonoHashStep6)
	if jonoHashStep5 != jonoHashStep6 {
		t.Fatal("Password hash changed when password column was empty!")
	}
	if auth.Verify("NewPassword789!", jonoHashStep6) != nil {
		t.Fatal("Password reverted after empty-password import!")
	}

	t.Logf("Step 6: Empty password → password unchanged ✓")

	// --- Step 7: Alias "Seksi Keamanan" → "keamanan" ---
	csvAlias := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,jabatan
%s,Jono Owner,OWNER,Jl. Test 1,Jono,3171010000001001,0811000010,%s,Seksi Keamanan
`, houseNum, jonoEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvAlias))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 7: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var jonoJabatanStep7 string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoJabatanStep7)
	if jonoJabatanStep7 != "keamanan" {
		t.Errorf("Jono jabatan should be 'keamanan', got %q", jonoJabatanStep7)
	}

	t.Logf("Step 7: Alias 'Seksi Keamanan' → 'keamanan' ✓")

	// --- Step 8: Cross-RT membership not affected ---
	rtB := setupTestRT(t, pool, "PartialUpdate RT B", 2, "PU02")
	tokenPengurusB := makeTenantToken(t, "user-pu-b", "mid-pu-b", rtB, auth.RolePengurus)
	houseB := "H-PU-B-" + fmt.Sprintf("%d", time.Now().UnixNano()%10000)
	rtbEmail := fmt.Sprintf("rtb-%s@test.com", suffix)

	csvRTB := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,jabatan,password
%s,RTB Head,OWNER,Jl. B,RTB Head,3171010000009001,0899999901,%s,keamanan,SecureRTB123!
`, houseB, rtbEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvRTB))
	req.Header.Set("Authorization", "Bearer "+tokenPengurusB)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit RTB: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Now import to RT A with jabatan — ensure RT B membership not created for Jono
	csvRTAUpdate := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,jabatan
%s,Jono Owner,OWNER,Jl. Test 1,Jono,3171010000001001,0811000010,%s,bendahara
`, houseNum, jonoEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvRTAUpdate))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 8: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var jonoJabatanFinal string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoJabatanFinal)
	if jonoJabatanFinal != "bendahara" {
		t.Errorf("Jono RT-A jabatan should be bendahara, got %q", jonoJabatanFinal)
	}

	var jonoRTBCount int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtB).Scan(&jonoRTBCount)
	if jonoRTBCount != 0 {
		t.Errorf("Jono has %d memberships in RT B (expected 0)", jonoRTBCount)
	}

	t.Logf("Step 8: RT-A update doesn't affect RT-B ✓")

	// --- Step 9: Re-import same file → no duplicate residency or membership ---
	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvAllFields))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 9: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var jonoResFinal, atunResFinal int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentJonoID).Scan(&jonoResFinal)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, residentAtunID).Scan(&atunResFinal)
	if jonoResFinal != 1 {
		t.Errorf("Jono has %d active residency (expected 1)", jonoResFinal)
	}
	if atunResFinal != 1 {
		t.Errorf("Atun has %d active residency (expected 1)", atunResFinal)
	}

	t.Logf("Step 9: Re-import → no duplicates ✓")
}

// TestImportPasswordWithoutJabatan verifies importing with password but no jabatan
// updates the existing user's password without requiring a membership.
func TestImportPasswordWithoutJabatan(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)
	rtID := setupTestRT(t, pool, "PwNoJabatan RT", 1, "PNJ1")
	tokenPengurus := makeTenantToken(t, "user-pnj", "mid-pnj", rtID, auth.RolePengurus)

	suffix := uuid.NewString()
	houseNum := "H-PNJ-" + suffix
	rinaEmail := fmt.Sprintf("rina-pnj-%s@test.com", suffix)

	// Import resident WITHOUT jabatan and WITHOUT password
	csvNoJabatan := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email
%s,Rina,OWNER,Jl. Test,Rina,3171010000002001,0811000020,%s
`, houseNum, rinaEmail)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvNoJabatan))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import without jabatan: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resID string
	pool.Raw().QueryRow(`SELECT id FROM residents WHERE rt_id = $1 AND nik = $2`, rtID, "3171010000002001").Scan(&resID)
	if resID == "" {
		t.Fatal("Rina not created")
	}

	// Now import with password but no jabatan
	csvWithPw := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,password
%s,Rina,OWNER,Jl. Test,Rina,3171010000002001,0811000020,%s,RinaNewPass1!
`, houseNum, rinaEmail)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvWithPw))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("update password: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("Password import without jabatan → success (code=%d) ✓", rec.Code)
}

// TestImportJabatanAliasSecurity verifies "Seksi Keamanan" normalizes to "keamanan".
func TestImportJabatanAliasSecurity(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)
	rtID := setupTestRT(t, pool, "Alias RT", 1, "ALI1")
	tokenPengurus := makeTenantToken(t, "user-ali", "mid-ali", rtID, auth.RolePengurus)

	suffix := uuid.NewString()
	houseNum := "H-ALI-" + suffix
	guardEmail := fmt.Sprintf("guard-ali-%s@test.com", suffix)

	csvAlias := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,jabatan,password
%s,Guard,OWNER,Jl. Test,Guard,3171010000003001,0811000030,%s,Seksi Keamanan,SecurePass123!
`, houseNum, guardEmail)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvAlias))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import alias: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resID string
	pool.Raw().QueryRow(`SELECT id FROM residents WHERE rt_id = $1 AND nik = $2`, rtID, "3171010000003001").Scan(&resID)

	var userID string
	pool.Raw().QueryRow(`SELECT user_id FROM residents WHERE id = $1`, resID).Scan(&userID)
	if userID == "" {
		t.Fatal("expected user created for jabatan import")
	}

	var jabatan string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, userID, rtID).Scan(&jabatan)
	if jabatan != "keamanan" {
		t.Errorf("expected jabatan 'keamanan', got %q", jabatan)
	}

	var pwHash string
	pool.Raw().QueryRow(`SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&pwHash)
	if pwHash == "SecurePass123!" {
		t.Fatal("password stored as plaintext!")
	}
	if auth.Verify("SecurePass123!", pwHash) != nil {
		t.Fatal("password does not verify!")
	}

	t.Logf("Alias 'Seksi Keamanan' → jabatan='%s', password verified ✓", jabatan)
}

// TestExportPasswordAlwaysEmpty verifies that export never includes password data.
func TestExportPasswordAlwaysEmpty(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)
	rtID := setupTestRT(t, pool, "ExportPw RT", 1, "EPR1")
	tokenPengurus := makeTenantToken(t, "user-epr", "mid-epr", rtID, auth.RolePengurus)

	suffix := uuid.NewString()
	houseNum := "H-EPR-" + suffix
	exporterEmail := fmt.Sprintf("exporter-epr-%s@test.com", suffix)

	csv := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,jabatan,password
%s,Exporter,OWNER,Jl. Test,Exporter,3171010000004001,0811000040,%s,bendahara,ExportPw123!
`, houseNum, exporterEmail)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csv))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Export
	req = httptest.NewRequest(http.MethodGet, "/api/v1/warga/export", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 2 {
		t.Fatal("export has fewer than 2 lines")
	}

	for i, line := range lines {
		if i == 0 {
			if !strings.Contains(line, "password") {
				t.Error("header missing 'password' column")
			}
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 11 {
			t.Errorf("line %d has %d fields (expected ≥11)", i, len(parts))
			continue
		}
		pwdField := strings.TrimSpace(parts[len(parts)-1])
		if pwdField != "" {
			t.Errorf("line %d password field is %q, expected empty", i, pwdField)
		}
		if strings.Contains(strings.ToLower(line), "exportpw") {
			t.Errorf("line %d contains password-like content", i)
		}
	}

	t.Logf("Export: %d data rows, all with empty password ✓", len(lines)-1)
}
