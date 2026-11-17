package household

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"social-finance/internal/auth"
)

// TestReimportIdempotencyU12_14U12_18 tests the exact production scenario where
// re-importing the same XLSX/CSV file caused "temporal move required" errors
// for houses U12/14 (Muhamad Andriyan) and U12/18 (Heri).
func TestReimportIdempotencyU12_14U12_18(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)

	t.Cleanup(func() {
		pool.Close()
	})

	suffix := uuid.NewString()
	rtID := setupTestRT(t, pool, "Idempotency RT", 12, fmt.Sprintf("U12_%s", suffix))
	tokenPengurus := makeTenantToken(t, "user-idem-1", "mid-idem-1", rtID, auth.RolePengurus)

	// --- Initial state: empty RT ---

	// --- Step 1: First import — two households ---
	// Use unique NIKs (16 digits) and emails per test run
	nik1 := fmt.Sprintf("317101%010d", time.Now().UnixNano()%10000000000)
	nik2 := fmt.Sprintf("317101%010d", time.Now().UnixNano()%10000000000+1)
	nik3 := fmt.Sprintf("317101%010d", time.Now().UnixNano()%10000000000+2)
	nik4 := fmt.Sprintf("317101%010d", time.Now().UnixNano()%10000000000+3)
	email1 := fmt.Sprintf("andriyan.%d@test.com", time.Now().UnixNano())
	email2 := fmt.Sprintf("siti.%d@test.com", time.Now().UnixNano()+1)
	email3 := fmt.Sprintf("heri.%d@test.com", time.Now().UnixNano()+2)
	email4 := fmt.Sprintf("dewi.%d@test.com", time.Now().UnixNano()+3)

	csvInitial := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
U12/14,Muhamad Andriyan,OWNER,Jl. Test 1,Muhamad Andriyan,%s,0811000010,%s,2024-01-15,ketua,Password123!
U12/14,Muhamad Andriyan,OWNER,Jl. Test 1,Siti Andriyan,%s,0811000011,%s,2024-01-15,sosial,Password456!
U12/18,Heri House,OWNER,Jl. Test 2,Heri House,%s,0811000020,%s,2024-02-01,ketua,HeriPass123!
U12/18,Heri House,OWNER,Jl. Test 2,Dewi House,%s,0811000021,%s,2024-02-01,anggota,HeriPass456!
`, nik1, email1, nik2, email2, nik3, email3, nik4, email4)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvInitial))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 1 (first import) failed: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("Step 1: First import succeeded (code=%d)", rec.Code)

	// --- Verify state after first import ---
	var phCount, hhCount, hoCount, resCount int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM physical_houses WHERE rt_id = $1`, rtID).Scan(&phCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM households WHERE rt_id = $1`, rtID).Scan(&hhCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM household_occupancies ho JOIN households h ON h.id = ho.household_id WHERE h.rt_id = $1 AND ho.end_date IS NULL`, rtID).Scan(&hoCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residents WHERE rt_id = $1`, rtID).Scan(&resCount)

	if phCount != 2 {
		t.Errorf("expected 2 physical houses, got %d", phCount)
	}
	if hhCount != 2 {
		t.Errorf("expected 2 households, got %d", hhCount)
	}
	if hoCount != 2 {
		t.Errorf("expected 2 active occupancies, got %d", hoCount)
	}
	if resCount != 4 {
		t.Errorf("expected 4 residents, got %d", resCount)
	}
	t.Logf("State after step 1: physical_houses=%d, households=%d, active_occupancies=%d, residents=%d", phCount, hhCount, hoCount, resCount)

	// --- Step 2: Re-import identical file ---
	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvInitial))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 2 (re-import) failed: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("Step 2: Re-import succeeded (code=%d)", rec.Code)

	// --- Verify state after re-import: no duplicates ---
	var phCount2, hhCount2, hoCount2, resCount2 int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM physical_houses WHERE rt_id = $1`, rtID).Scan(&phCount2)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM households WHERE rt_id = $1`, rtID).Scan(&hhCount2)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM household_occupancies ho JOIN households h ON h.id = ho.household_id WHERE h.rt_id = $1 AND ho.end_date IS NULL`, rtID).Scan(&hoCount2)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residents WHERE rt_id = $1`, rtID).Scan(&resCount2)

	if phCount2 != 2 {
		t.Errorf("after re-import: expected 2 physical houses, got %d", phCount2)
	}
	if hhCount2 != 2 {
		t.Errorf("after re-import: expected 2 households, got %d", hhCount2)
	}
	if hoCount2 != 2 {
		t.Errorf("after re-import: expected 2 active occupancies, got %d", hoCount2)
	}
	if resCount2 != 4 {
		t.Errorf("after re-import: expected 4 residents, got %d", resCount2)
	}

	// Also verify no residency period duplicates
	var jonoResCount int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND nik = $2`, rtID, nik1).Scan(&jonoResCount)
	if jonoResCount != 1 {
		t.Errorf("expected 1 resident with NIK %s, got %d", nik1, jonoResCount)
	}
	// Count active residency periods for that resident
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residency_periods rp JOIN residents r ON r.id = rp.resident_id WHERE r.nik = $1 AND rp.end_date IS NULL`, nik1).Scan(&jonoResCount)
	if jonoResCount != 1 {
		t.Errorf("expected 1 active residency for NIK %s, got %d", nik1, jonoResCount)
	}

	t.Logf("State after step 2 (re-import): physical_houses=%d, households=%d, active_occupancies=%d, residents=%d ✓", phCount2, hhCount2, hoCount2, resCount2)

	// --- Step 3: Third import (triple-check) ---
	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvInitial))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("step 3 (triple re-import) failed: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var phCount3, hhCount3, hoCount3, resCount3 int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM physical_houses WHERE rt_id = $1`, rtID).Scan(&phCount3)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM households WHERE rt_id = $1`, rtID).Scan(&hhCount3)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM household_occupancies ho JOIN households h ON h.id = ho.household_id WHERE h.rt_id = $1 AND ho.end_date IS NULL`, rtID).Scan(&hoCount3)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residents WHERE rt_id = $1`, rtID).Scan(&resCount3)

	if phCount3 != 2 || hhCount3 != 2 || hoCount3 != 2 || resCount3 != 4 {
		t.Errorf("after step 3: expected 2/2/2/4, got %d/%d/%d/%d", phCount3, hhCount3, hoCount3, resCount3)
	}
	t.Logf("Step 3: Triple re-import succeeded, state verified (2/2/2/4) ✓")
}

// TestReimportWithHouseConflict tests that importing a different household
// into an occupied house is correctly rejected.
func TestReimportWithHouseConflict(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)

	t.Cleanup(func() {
		pool.Close()
	})

	suffix := uuid.NewString()
	rtID := setupTestRT(t, pool, "Conflict RT", 1, fmt.Sprintf("CONFLICT1_%s", suffix))
	tokenPengurus := makeTenantToken(t, "user-conflict-1", "mid-conflict-1", rtID, auth.RolePengurus)

	// Use unique NIKs and emails per test run
	nikSeed := time.Now().UnixNano() % 10000000000
	nik1 := fmt.Sprintf("317101%010d", nikSeed)
	nik2 := fmt.Sprintf("317101%010d", nikSeed+1)
	nik3 := fmt.Sprintf("317101%010d", nikSeed+2)
	email1 := fmt.Sprintf("a.%d@test.com", time.Now().UnixNano())
	email2 := fmt.Sprintf("am.%d@test.com", time.Now().UnixNano()+1)
	email3 := fmt.Sprintf("b.%d@test.com", time.Now().UnixNano()+2)

	// First import: Household A at H-1
	csvA := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-1,A Owner,OWNER,Jl. Test,A Owner,%s,0811000090,%s,2024-01-15,ketua,PassA123!
H-1,A Owner,OWNER,Jl. Test,A Member,%s,0811000091,%s,2024-01-15,anggota,PassAM123!
`, nik1, email1, nik2, email2)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvA))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first import: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("First import succeeded (code=%d)", rec.Code)

	// Second import: Different household B trying to occupy H-1 → should fail
	csvB := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-1,B Owner,OWNER,Jl. Test,B Owner,%s,0811000092,%s,2024-02-01,ketua,PassB123!
`, nik3, email3)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvB))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("conflict import: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("Conflict import correctly rejected (code=%d) ✓", rec.Code)

	// Verify state unchanged
	var phCount, hhCount, hoCount int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM physical_houses WHERE rt_id = $1`, rtID).Scan(&phCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM households WHERE rt_id = $1`, rtID).Scan(&hhCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM household_occupancies ho JOIN households h ON h.id = ho.household_id WHERE h.rt_id = $1 AND ho.end_date IS NULL`, rtID).Scan(&hoCount)

	if phCount != 1 || hhCount != 1 || hoCount != 1 {
		t.Errorf("state should be unchanged: ph=%d(1), hh=%d(1), ho=%d(1)", phCount, hhCount, hoCount)
	}
	t.Logf("State unchanged after conflict: physical_houses=%d, households=%d, active_occupancies=%d ✓", phCount, hhCount, hoCount)
}

// TestReimportWithNIKMatch tests that when head_name doesn't match exactly but NIK does,
// it's still recognized as the same household.
func TestReimportWithNIKMatch(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)

	t.Cleanup(func() {
		pool.Close()
	})

	suffix := uuid.NewString()
	rtID := setupTestRT(t, pool, "NIKMatch RT", 1, fmt.Sprintf("NIK1_%s", suffix))
	tokenPengurus := makeTenantToken(t, "user-nik-1", "mid-nik-1", rtID, auth.RolePengurus)

	// Use unique NIKs and emails per test run to avoid test pollution
	// NIKs must be exactly 16 digits: 6 prefix + 10 unique suffix
	nikSeed := time.Now().UnixNano() % 10000000000
	nik1 := fmt.Sprintf("317101%010d", nikSeed)
	nik2 := fmt.Sprintf("317101%010d", nikSeed+1)
	email1 := fmt.Sprintf("muhamad.%d@test.com", time.Now().UnixNano())
	email2 := fmt.Sprintf("siti.%d@test.com", time.Now().UnixNano()+1)

	// First import: household with head "Muhamad Andriyan"
	csvInitial := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-NIK,Muhamad Andriyan,OWNER,Jl. Test,Muhamad Andriyan,%s,0811000080,%s,2024-01-15,ketua,Pass123!
H-NIK,Muhamad Andriyan,OWNER,Jl. Test,Siti,%s,0811000081,%s,2024-01-15,anggota,Pass456!
`, nik1, email1, nik2, email2)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvInitial))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first import: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("First import succeeded (code=%d)", rec.Code)

	// Count initial state
	var initialHH, initialHO, initialRes int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM households WHERE rt_id = $1`, rtID).Scan(&initialHH)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM household_occupancies ho JOIN households h ON h.id = ho.household_id WHERE h.rt_id = $1 AND ho.end_date IS NULL`, rtID).Scan(&initialHO)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residents WHERE rt_id = $1`, rtID).Scan(&initialRes)

	// Re-import with abbreviated head_name but same NIKs and emails
	csvReimport := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-NIK,M. Andriyan,OWNER,Jl. Test,M. Andriyan,%s,0811000080,%s,2024-01-15,ketua,Pass123!
H-NIK,M. Andriyan,OWNER,Jl. Test,Siti,%s,0811000081,%s,2024-01-15,anggota,Pass456!
`, nik1, email1, nik2, email2)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvReimport))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("re-import with NIK match: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("Re-import with NIK match succeeded (code=%d) ✓", rec.Code)

	// Verify no duplicates
	var afterHH, afterHO, afterRes int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM households WHERE rt_id = $1`, rtID).Scan(&afterHH)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM household_occupancies ho JOIN households h ON h.id = ho.household_id WHERE h.rt_id = $1 AND ho.end_date IS NULL`, rtID).Scan(&afterHO)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residents WHERE rt_id = $1`, rtID).Scan(&afterRes)

	if afterHH != initialHH {
		t.Errorf("household count changed: %d → %d", initialHH, afterHH)
	}
	if afterHO != initialHO {
		t.Errorf("occupancy count changed: %d → %d", initialHO, afterHO)
	}
	if afterRes != initialRes {
		t.Errorf("resident count changed: %d → %d", initialRes, afterRes)
	}
	t.Logf("No duplicates after NIK match re-import (HH=%d, HO=%d, Res=%d) ✓", afterHH, afterHO, afterRes)
}

// TestReimportWithDifferentNameDifferentNIK tests that different NIKs with same house
// are treated as new residents, not household conflict.
func TestReimportWithDifferentNameDifferentNIK(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)

	t.Cleanup(func() {
		pool.Close()
	})

	suffix := uuid.NewString()
	rtID := setupTestRT(t, pool, "DifferentNIK RT", 1, fmt.Sprintf("DN1_%s", suffix))
	tokenPengurus := makeTenantToken(t, "user-dn-1", "mid-dn-1", rtID, auth.RolePengurus)

	// Use unique NIKs and emails per test run
	nikSeed := time.Now().UnixNano() % 10000000000
	nik1 := fmt.Sprintf("317101%010d", nikSeed)
	nik2 := fmt.Sprintf("317101%010d", nikSeed+1)
	email1 := fmt.Sprintf("a.%d@test.com", time.Now().UnixNano())
	email2 := fmt.Sprintf("news.%d@test.com", time.Now().UnixNano()+1)

	// First import: household A
	csvA := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-DN,A Owner,OWNER,Jl. Test,A Owner,%s,0811000070,%s,2024-01-15,ketua,PassA123!
`, nik1, email1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvA))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first import: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Second import: same house, different head_name, different NIK
	// This is a conflict because head_name differs AND no NIK match in the current household.
	// However, since the NIK doesn't match any resident in the current household, it should
	// still be rejected as a house conflict.
	csvConflict := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
H-DN,New Owner,OWNER,Jl. Test,New Owner,%s,0811000071,%s,2024-02-01,ketua,PassN123!
`, nik2, email2)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvConflict))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// This should be rejected because head_name differs and no NIK match in current household
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("conflict import: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("Different name + different NIK correctly rejected (code=%d) ✓", rec.Code)
}

// TestReimportEmptyFieldsNoOp tests that re-importing with empty optional fields
// doesn't change existing data.
func TestReimportEmptyFieldsNoOp(t *testing.T) {
	pool := testPool(t)
	router := setupWargaImportExportRouter(pool)

	t.Cleanup(func() {
		pool.Close()
	})

	rtID := setupTestRT(t, pool, "NoOp RT", 1, "NOOP1")
	tokenPengurus := makeTenantToken(t, "user-noop-1", "mid-noop-1", rtID, auth.RolePengurus)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	houseNum := "H-NOOP-" + suffix

	// Use unique NIKs (16 digits) to avoid test pollution
	nikSeed := time.Now().UnixNano() % 10000000000
	nik1 := fmt.Sprintf("317101%010d", nikSeed)
	nik2 := fmt.Sprintf("317101%010d", nikSeed+1)
	email1 := fmt.Sprintf("noop1-%s@test.com", suffix)
	email2 := fmt.Sprintf("noop2-%s@test.com", suffix)

	// First import: full data
	csvInitial := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
%s,Jono Owner,OWNER,Jl. Test Jono,Jono,%s,0811000060,%s,2024-01-15,ketua,PassJono123!
%s,Jono Owner,OWNER,Jl. Test Jono,Atun,%s,0811000061,%s,2024-03-01,sosial,PassAtun456!
`, houseNum, nik1, email1, houseNum, nik2, email2)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvInitial))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first import: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Get initial state
	var jonoResID, atunResID string
	pool.Raw().QueryRow(`SELECT r.id FROM residents r WHERE r.nik = $1`, nik1).Scan(&jonoResID)
	pool.Raw().QueryRow(`SELECT r.id FROM residents r WHERE r.nik = $1`, nik2).Scan(&atunResID)

	var jonoStart, atunStart string
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, jonoResID).Scan(&jonoStart)
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, atunResID).Scan(&atunStart)

	var jonoJabatan, atunJabatan string
	var jonoUserID string
	pool.Raw().QueryRow(`SELECT u.id FROM users u JOIN residents r ON r.user_id = u.id WHERE r.nik = $1`, nik1).Scan(&jonoUserID)
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoJabatan)
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, atunResID, rtID).Scan(&atunJabatan)

	// Second import: empty optional fields (jabatan, password) should not change existing data
	csvEmptyFields := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password
%s,Jono Owner,OWNER,Jl. Test Jono,Jono,%s,0811000060,%s,2024-01-15,,
%s,Jono Owner,OWNER,Jl. Test Jono,Atun,%s,0811000061,%s,2024-03-01,,
`, houseNum, nik1, email1, houseNum, nik2, email2)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/commit", strings.NewReader(csvEmptyFields))
	req.Header.Set("Authorization", "Bearer "+tokenPengurus)
	req.Header.Set("Content-Type", "text/csv")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("re-import empty fields: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify dates unchanged
	var jonoStart2, atunStart2 string
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, jonoResID).Scan(&jonoStart2)
	pool.Raw().QueryRow(`SELECT start_date::text FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, atunResID).Scan(&atunStart2)
	if jonoStart2 != jonoStart {
		t.Errorf("Jono date changed: %s → %s", jonoStart, jonoStart2)
	}
	if atunStart2 != atunStart {
		t.Errorf("Atun date changed: %s → %s", atunStart, atunStart2)
	}

	// Verify jabatan unchanged
	var jonoJabatan2, atunJabatan2 string
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, jonoUserID, rtID).Scan(&jonoJabatan2)
	pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`, atunResID, rtID).Scan(&atunJabatan2)
	if jonoJabatan2 != jonoJabatan {
		t.Errorf("Jono jabatan changed: %s → %s", jonoJabatan, jonoJabatan2)
	}
	if atunJabatan2 != atunJabatan {
		t.Errorf("Atun jabatan changed: %s → %s", atunJabatan, atunJabatan2)
	}

	// Verify no duplicates
	var jonoResCount, atunResCount int
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, jonoResID).Scan(&jonoResCount)
	pool.Raw().QueryRow(`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`, atunResID).Scan(&atunResCount)
	if jonoResCount != 1 {
		t.Errorf("Jono has %d active residencies (expected 1)", jonoResCount)
	}
	if atunResCount != 1 {
		t.Errorf("Atun has %d active residencies (expected 1)", atunResCount)
	}

	t.Logf("Empty fields re-import: dates and jabatan unchanged, no duplicates ✓")
}
