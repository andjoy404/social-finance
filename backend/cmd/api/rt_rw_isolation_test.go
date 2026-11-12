package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/devseed"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

var rtRwIsoSeq uint64 = uint64(time.Now().UnixNano()%1000000 + 6000000)

func setupRtRwIsoRouter(t *testing.T, rwIDs []string) (*database.Pool, http.Handler) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("ensure devseed: %v", err)
	}

	auth.SigningSecret = []byte("test-secret-key-32-chars-long-ok!!")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	auth.AuthDBPool = pool

	authSvc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: 7 * 24 * time.Hour,
	})
	authHandler := auth.NewHandler(pool, authSvc)
	cfg := auth.ScopingConfig{
		RTIDs: []string{
			"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", // RT 002
			"8dfa1e36-6727-45be-8fea-24d958efc155", // RT 003
		},
		RWIDs: rwIDs,
	}
	router := buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
	return pool, router
}

func createTestRtRwIsoRW(t *testing.T, pool *database.Pool, name string) string {
	t.Helper()
	rwNum := int(atomic.AddUint64(&rtRwIsoSeq, 1))
	var rwID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rws (id, name, rw_number, is_active)
		 VALUES (gen_random_uuid(), $1, $2, true)
		 RETURNING id`,
		name, rwNum,
	).Scan(&rwID)
	if err != nil {
		t.Fatalf("create test rw: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_location_balance_changes WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_transactions WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_fund_transfers WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_fund_locations WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_opening_balances WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})
	return rwID
}

func createTestRtRwIsoRTLocation(t *testing.T, pool *database.Pool, rtID, name, category, initialBal string) string {
	t.Helper()
	var locID string
	var bankName, accNo, accHolder sql.NullString
	if category == "rekening" {
		bankName = sql.NullString{String: "BCA", Valid: true}
		accNo = sql.NullString{String: "111222333", Valid: true}
		accHolder = sql.NullString{String: "Kas RT", Valid: true}
	}
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO fund_locations (id, rt_id, name, category, bank_name, account_number, account_holder, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, true)
		 RETURNING id`,
		rtID, name, category, bankName, accNo, accHolder,
	).Scan(&locID)
	if err != nil {
		t.Fatalf("create test rt location %s: %v", name, err)
	}

	if initialBal != "" && initialBal != "0" {
		_, err = pool.Raw().ExecContext(context.Background(),
			`INSERT INTO location_balance_changes (id, rt_id, location_id, change_type, amount_delta, notes)
			 VALUES (gen_random_uuid(), $1, $2, 'opening_balance', $3::numeric, 'Initial RT opening balance')`,
			rtID, locID, initialBal,
		)
		if err != nil {
			t.Fatalf("insert rt opening balance change for %s: %v", name, err)
		}
	}

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM location_balance_changes WHERE location_id = $1`, locID)
		pool.Raw().ExecContext(ctx, `DELETE FROM fund_locations WHERE id = $1`, locID)
	})

	return locID
}

func createTestRtRwIsoRWLocation(t *testing.T, pool *database.Pool, rwID, name, category, initialBal string) string {
	t.Helper()
	var locID string
	var bankName, accNo, accHolder sql.NullString
	if category == "rekening" {
		bankName = sql.NullString{String: "Mandiri", Valid: true}
		accNo = sql.NullString{String: "888999000", Valid: true}
		accHolder = sql.NullString{String: "Kas RW", Valid: true}
	}
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rw_fund_locations (id, rw_id, name, category, bank_name, account_number, account_holder, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, true)
		 RETURNING id`,
		rwID, name, category, bankName, accNo, accHolder,
	).Scan(&locID)
	if err != nil {
		t.Fatalf("create test rw location %s: %v", name, err)
	}

	if initialBal != "" && initialBal != "0" {
		_, err = pool.Raw().ExecContext(context.Background(),
			`INSERT INTO rw_location_balance_changes (id, rw_id, location_id, change_type, amount_delta, notes)
			 VALUES (gen_random_uuid(), $1, $2, 'opening_balance', $3::numeric, 'Initial RW opening balance')`,
			rwID, locID, initialBal,
		)
		if err != nil {
			t.Fatalf("insert rw opening balance change for %s: %v", name, err)
		}
	}

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_location_balance_changes WHERE location_id = $1`, locID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_fund_locations WHERE id = $1`, locID)
	})

	return locID
}

func createRtRwIsoRTUserToken(t *testing.T, pool *database.Pool, rtID string, role auth.Role, jabatan auth.Jabatan) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rt-user-%s-%d@test.com", jabatan, atomic.AddUint64(&rtRwIsoSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RT User', NULL, true)
		 RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("create rt user: %v", err)
	}

	var membershipID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true)
		 RETURNING id`,
		userID, rtID, string(role), string(jabatan),
	).Scan(&membershipID)
	if err != nil {
		t.Fatalf("create rt membership: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, membershipID)
		pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     membershipID,
		RTID:    rtID,
		Role:    string(role),
		Jabatan: string(jabatan),
	})
	if err != nil {
		t.Fatalf("generate rt token: %v", err)
	}
	return userID, token
}

func createRtRwIsoRWUserToken(t *testing.T, pool *database.Pool, rwID string, role auth.Role, jabatan auth.Jabatan) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rw-user-%s-%d@test.com", jabatan, atomic.AddUint64(&rtRwIsoSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RW User', NULL, true)
		 RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("create rw user: %v", err)
	}

	var membershipID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rw_memberships (id, user_id, rw_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true)
		 RETURNING id`,
		userID, rwID, string(role), string(jabatan),
	).Scan(&membershipID)
	if err != nil {
		t.Fatalf("create rw membership: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE id = $1`, membershipID)
		pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	rwContextJSON := fmt.Sprintf(`[{"rw_id":"%s","jabatan":"%s"}]`, rwID, string(jabatan))
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:    userID,
		Role:      string(role),
		Jabatan:   string(jabatan),
		RWContext: rwContextJSON,
	})
	if err != nil {
		t.Fatalf("generate rw token: %v", err)
	}
	return userID, token
}

// -----------------------------------------------------------------------------
// Test 1 & 2: RT A tidak dapat membaca atau mengubah lokasi RT B
// -----------------------------------------------------------------------------
func TestIsolation_RT_CannotReadOrModifyOtherRTLocation(t *testing.T) {
	pool, router := setupRtRwIsoRouter(t, nil)
	rtAID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b" // RT 002
	rtBID := "8dfa1e36-6727-45be-8fea-24d958efc155" // RT 003

	_, tokenRTA := createRtRwIsoRTUserToken(t, pool, rtAID, auth.RoleBendahara, auth.JabatanBendahara)
	locBID := createTestRtRwIsoRTLocation(t, pool, rtBID, "Brankas RT B", "brankas", "1000000.00")

	// 1. RT A membaca lokasi RT B -> 404 Not Found (tenant-isolated query)
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/fund-locations/%s", locBID), nil)
	reqGet.Header.Set("Authorization", "Bearer "+tokenRTA)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RT A reads RT B location, got %d", recGet.Code)
	}

	// 2. RT A mencoba mengubah nama lokasi RT B -> 404 Not Found
	updateBody, _ := json.Marshal(map[string]any{"name": "Hacked Location Name"})
	reqPatch := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/fund-locations/%s", locBID), bytes.NewReader(updateBody))
	reqPatch.Header.Set("Authorization", "Bearer "+tokenRTA)
	reqPatch.Header.Set("Content-Type", "application/json")
	recPatch := httptest.NewRecorder()
	router.ServeHTTP(recPatch, reqPatch)
	if recPatch.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RT A updates RT B location, got %d", recPatch.Code)
	}

	// Verify database integrity: name must remain unchanged
	var currentName string
	err := pool.Raw().QueryRowContext(context.Background(), `SELECT name FROM fund_locations WHERE id = $1`, locBID).Scan(&currentName)
	if err != nil {
		t.Fatalf("get location name: %v", err)
	}
	if currentName != "Brankas RT B" {
		t.Fatalf("RT B location was modified by RT A! Got %s", currentName)
	}
}

// -----------------------------------------------------------------------------
// Test 3 & 4: RT tidak dapat membaca atau mengubah lokasi RW
// -----------------------------------------------------------------------------
func TestIsolation_RT_CannotReadOrModifyRWLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createTestRtRwIsoRW(t, pool, "RW Isolated")
	_, router := setupRtRwIsoRouter(t, []string{rwID})

	rtAID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
	_, tokenRTA := createRtRwIsoRTUserToken(t, pool, rtAID, auth.RoleBendahara, auth.JabatanBendahara)
	rwLocID := createTestRtRwIsoRWLocation(t, pool, rwID, "Kas Utama RW", "coh", "2000000.00")

	// 3. RT A mencoba membaca lokasi RW via endpoint RT -> 404 Not Found
	reqGetRT := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/fund-locations/%s", rwLocID), nil)
	reqGetRT.Header.Set("Authorization", "Bearer "+tokenRTA)
	recGetRT := httptest.NewRecorder()
	router.ServeHTTP(recGetRT, reqGetRT)
	if recGetRT.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RT reads RW location via RT endpoint, got %d", recGetRT.Code)
	}

	// RT A mencoba membaca lokasi RW via endpoint RW -> 403 Forbidden (no RW membership)
	reqGetRW := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwID, rwLocID), nil)
	reqGetRW.Header.Set("Authorization", "Bearer "+tokenRTA)
	recGetRW := httptest.NewRecorder()
	router.ServeHTTP(recGetRW, reqGetRW)
	if recGetRW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when RT accesses RW endpoint, got %d", recGetRW.Code)
	}

	// 4. RT A mencoba mengubah lokasi RW via endpoint RT -> 404 Not Found
	updateBody, _ := json.Marshal(map[string]any{"name": "Tampered RW Location"})
	reqPatchRT := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/fund-locations/%s", rwLocID), bytes.NewReader(updateBody))
	reqPatchRT.Header.Set("Authorization", "Bearer "+tokenRTA)
	reqPatchRT.Header.Set("Content-Type", "application/json")
	recPatchRT := httptest.NewRecorder()
	router.ServeHTTP(recPatchRT, reqPatchRT)
	if recPatchRT.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RT modifies RW location via RT endpoint, got %d", recPatchRT.Code)
	}

	// RT A mencoba mengubah lokasi RW via endpoint RW -> 403 Forbidden
	reqPutRW := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwID, rwLocID), bytes.NewReader(updateBody))
	reqPutRW.Header.Set("Authorization", "Bearer "+tokenRTA)
	reqPutRW.Header.Set("Content-Type", "application/json")
	recPutRW := httptest.NewRecorder()
	router.ServeHTTP(recPutRW, reqPutRW)
	if recPutRW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when RT modifies RW location via RW endpoint, got %d", recPutRW.Code)
	}

	// Verify database: RW location remains intact
	var currentName string
	err := pool.Raw().QueryRowContext(context.Background(), `SELECT name FROM rw_fund_locations WHERE id = $1`, rwLocID).Scan(&currentName)
	if err != nil {
		t.Fatalf("get rw location name: %v", err)
	}
	if currentName != "Kas Utama RW" {
		t.Fatalf("RW location was altered! Got %s", currentName)
	}
}

// -----------------------------------------------------------------------------
// Test 5 & 6: RW A tidak dapat membaca atau mengubah lokasi RW B
// -----------------------------------------------------------------------------
func TestIsolation_RW_CannotReadOrModifyOtherRWLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwAID := createTestRtRwIsoRW(t, pool, "RW A Scope")
	rwBID := createTestRtRwIsoRW(t, pool, "RW B Scope")

	_, router := setupRtRwIsoRouter(t, []string{rwAID, rwBID})

	_, tokenRWA := createRtRwIsoRWUserToken(t, pool, rwAID, auth.RoleBendahara, auth.JabatanBendahara)
	locBID := createTestRtRwIsoRWLocation(t, pool, rwBID, "Brankas RW B", "brankas", "3000000.00")

	// 5. RW A mencoba membaca lokasi RW B via endpoint RW B -> 403 Forbidden (membership mismatch)
	reqGetDirect := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwBID, locBID), nil)
	reqGetDirect.Header.Set("Authorization", "Bearer "+tokenRWA)
	recGetDirect := httptest.NewRecorder()
	router.ServeHTTP(recGetDirect, reqGetDirect)
	if recGetDirect.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when RW A reads via RW B path, got %d", recGetDirect.Code)
	}

	// RW A mencoba membaca ID lokasi RW B via endpoint RW A -> 404 Not Found (tenant-isolated query)
	reqGetCross := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwAID, locBID), nil)
	reqGetCross.Header.Set("Authorization", "Bearer "+tokenRWA)
	recGetCross := httptest.NewRecorder()
	router.ServeHTTP(recGetCross, reqGetCross)
	if recGetCross.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RW A queries foreign location ID, got %d", recGetCross.Code)
	}

	// 6. RW A mencoba mengubah lokasi RW B via endpoint RW B -> 403 Forbidden
	updateBody, _ := json.Marshal(map[string]any{"name": "Hacked RW B"})
	reqPutDirect := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwBID, locBID), bytes.NewReader(updateBody))
	reqPutDirect.Header.Set("Authorization", "Bearer "+tokenRWA)
	reqPutDirect.Header.Set("Content-Type", "application/json")
	recPutDirect := httptest.NewRecorder()
	router.ServeHTTP(recPutDirect, reqPutDirect)
	if recPutDirect.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when RW A updates via RW B path, got %d", recPutDirect.Code)
	}

	// RW A mencoba mengubah ID lokasi RW B via endpoint RW A -> 404 Not Found
	reqPutCross := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwAID, locBID), bytes.NewReader(updateBody))
	reqPutCross.Header.Set("Authorization", "Bearer "+tokenRWA)
	reqPutCross.Header.Set("Content-Type", "application/json")
	recPutCross := httptest.NewRecorder()
	router.ServeHTTP(recPutCross, reqPutCross)
	if recPutCross.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RW A updates foreign location ID, got %d", recPutCross.Code)
	}

	// Verify database: location B unchanged
	var currentName string
	err := pool.Raw().QueryRowContext(context.Background(), `SELECT name FROM rw_fund_locations WHERE id = $1`, locBID).Scan(&currentName)
	if err != nil {
		t.Fatalf("get location name: %v", err)
	}
	if currentName != "Brankas RW B" {
		t.Fatalf("RW B location was modified! Got %s", currentName)
	}
}

// -----------------------------------------------------------------------------
// Test 7: RW tidak dapat membaca atau mengubah lokasi RT
// -----------------------------------------------------------------------------
func TestIsolation_RW_CannotReadOrModifyRTLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createTestRtRwIsoRW(t, pool, "RW Test Isolation")
	_, router := setupRtRwIsoRouter(t, []string{rwID})

	rtID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
	locRTID := createTestRtRwIsoRTLocation(t, pool, rtID, "Kas Operasional RT", "coh", "500000.00")
	_, tokenRW := createRtRwIsoRWUserToken(t, pool, rwID, auth.RoleBendahara, auth.JabatanBendahara)

	// RW mencoba membaca ID lokasi RT melalui endpoint RW -> 404 Not Found
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwID, locRTID), nil)
	reqGet.Header.Set("Authorization", "Bearer "+tokenRW)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RW queries RT location ID, got %d", recGet.Code)
	}

	// RW mencoba mengubah ID lokasi RT melalui endpoint RW -> 404 Not Found
	updateBody, _ := json.Marshal(map[string]any{"name": "Hacked RT From RW"})
	reqPut := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/rw/%s/locations/%s", rwID, locRTID), bytes.NewReader(updateBody))
	reqPut.Header.Set("Authorization", "Bearer "+tokenRW)
	reqPut.Header.Set("Content-Type", "application/json")
	recPut := httptest.NewRecorder()
	router.ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RW updates RT location ID, got %d", recPut.Code)
	}

	// Verify RT location in fund_locations remains unchanged
	var currentName string
	err := pool.Raw().QueryRowContext(context.Background(), `SELECT name FROM fund_locations WHERE id = $1`, locRTID).Scan(&currentName)
	if err != nil {
		t.Fatalf("get location name: %v", err)
	}
	if currentName != "Kas Operasional RT" {
		t.Fatalf("RT location was altered by RW! Got %s", currentName)
	}
}

// -----------------------------------------------------------------------------
// Test 8: Transfer RT lintas-RT ditolak
// -----------------------------------------------------------------------------
func TestIsolation_CrossRT_TransferRejected(t *testing.T) {
	pool, router := setupRtRwIsoRouter(t, nil)
	rtAID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
	rtBID := "8dfa1e36-6727-45be-8fea-24d958efc155"

	_, tokenRTA := createRtRwIsoRTUserToken(t, pool, rtAID, auth.RoleBendahara, auth.JabatanBendahara)
	locA := createTestRtRwIsoRTLocation(t, pool, rtAID, "Kas RT A", "coh", "1000000.00")
	locB := createTestRtRwIsoRTLocation(t, pool, rtBID, "Kas RT B", "coh", "1000000.00")

	transferBody, _ := json.Marshal(map[string]any{
		"from_location_id": locA,
		"to_location_id":   locB,
		"amount":           "200000.00",
		"notes":            "Illegal Cross-RT Transfer",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/fund-transfers", bytes.NewReader(transferBody))
	req.Header.Set("Authorization", "Bearer "+tokenRTA)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
		t.Fatalf("expected cross-RT transfer to be rejected, got %d", rec.Code)
	}

	// Verify database: zero balance changes, zero transfers created
	var countTransfers int
	pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM fund_transfers WHERE rt_id = $1`, rtAID).Scan(&countTransfers)
	if countTransfers != 0 {
		t.Fatalf("expected 0 transfers, found %d", countTransfers)
	}
}

// -----------------------------------------------------------------------------
// Test 9: Transfer RW lintas-RW ditolak
// -----------------------------------------------------------------------------
func TestIsolation_CrossRW_TransferRejected(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwAID := createTestRtRwIsoRW(t, pool, "RW A Transfer Scope")
	rwBID := createTestRtRwIsoRW(t, pool, "RW B Transfer Scope")

	_, router := setupRtRwIsoRouter(t, []string{rwAID, rwBID})

	_, tokenRWA := createRtRwIsoRWUserToken(t, pool, rwAID, auth.RoleBendahara, auth.JabatanBendahara)
	locA := createTestRtRwIsoRWLocation(t, pool, rwAID, "Kas RW A", "coh", "2000000.00")
	locB := createTestRtRwIsoRWLocation(t, pool, rwBID, "Kas RW B", "coh", "1000000.00")

	transferBody, _ := json.Marshal(map[string]any{
		"from_location_id": locA,
		"to_location_id":   locB,
		"amount":           "500000.00",
		"notes":            "Illegal Cross-RW Transfer",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rw/%s/locations/transfers", rwAID), bytes.NewReader(transferBody))
	req.Header.Set("Authorization", "Bearer "+tokenRWA)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest && rec.Code != http.StatusForbidden {
		t.Fatalf("expected cross-RW transfer to be rejected with 404/400/403, got %d", rec.Code)
	}

	// Verify database: zero transfers in RW A or RW B
	var countA, countB int
	pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM rw_fund_transfers WHERE rw_id = $1`, rwAID).Scan(&countA)
	pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM rw_fund_transfers WHERE rw_id = $1`, rwBID).Scan(&countB)
	if countA != 0 || countB != 0 {
		t.Fatalf("expected 0 transfers, found A=%d B=%d", countA, countB)
	}
}

// -----------------------------------------------------------------------------
// Test 10: Transfer RT ke RW melalui endpoint transfer lokasi ditolak
// -----------------------------------------------------------------------------
func TestIsolation_RTtoRW_DirectTransferRejected(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createTestRtRwIsoRW(t, pool, "RW Direct Transfer Scope")
	_, router := setupRtRwIsoRouter(t, []string{rwID})

	rtID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
	_, tokenRT := createRtRwIsoRTUserToken(t, pool, rtID, auth.RoleBendahara, auth.JabatanBendahara)
	_, tokenRW := createRtRwIsoRWUserToken(t, pool, rwID, auth.RoleBendahara, auth.JabatanBendahara)

	locRT := createTestRtRwIsoRTLocation(t, pool, rtID, "Kas RT Asal", "coh", "1000000.00")
	locRW := createTestRtRwIsoRWLocation(t, pool, rwID, "Kas RW Tujuan", "coh", "1000000.00")

	// 1. Coba transfer via endpoint transfer RT: from locRT -> to locRW
	bodyRT, _ := json.Marshal(map[string]any{
		"from_location_id": locRT,
		"to_location_id":   locRW,
		"amount":           "100000.00",
		"notes":            "Illegal Direct RT to RW Transfer via RT Endpoint",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/fund-transfers", bytes.NewReader(bodyRT))
	req1.Header.Set("Authorization", "Bearer "+tokenRT)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code == http.StatusOK || rec1.Code == http.StatusCreated {
		t.Fatalf("expected RT-to-RW transfer via RT endpoint to fail, got %d", rec1.Code)
	}

	// 2. Coba transfer via endpoint transfer RW: from locRT -> to locRW
	bodyRW, _ := json.Marshal(map[string]any{
		"from_location_id": locRT,
		"to_location_id":   locRW,
		"amount":           "100000.00",
		"notes":            "Illegal Direct RT to RW Transfer via RW Endpoint",
	})
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rw/%s/locations/transfers", rwID), bytes.NewReader(bodyRW))
	req2.Header.Set("Authorization", "Bearer "+tokenRW)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusOK || rec2.Code == http.StatusCreated {
		t.Fatalf("expected RT-to-RW transfer via RW endpoint to fail, got %d", rec2.Code)
	}

	// Verify database: zero transfers in both tables
	var countRTTransfers, countRWTransfers int
	pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM fund_transfers WHERE rt_id = $1`, rtID).Scan(&countRTTransfers)
	pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM rw_fund_transfers WHERE rw_id = $1`, rwID).Scan(&countRWTransfers)
	if countRTTransfers != 0 || countRWTransfers != 0 {
		t.Fatalf("expected 0 transfers, found RT=%d RW=%d", countRTTransfers, countRWTransfers)
	}
}

// -----------------------------------------------------------------------------
// Test 11: Penerimaan dana RT ke RW tercatat sebagai transaksi KAS Masuk RW
//
//	dengan source_rt_id yang benar
//
// -----------------------------------------------------------------------------
func TestIsolation_RTtoRW_LegitimateKasIncome(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createTestRtRwIsoRW(t, pool, "RW Kas Legitimate Scope")
	_, router := setupRtRwIsoRouter(t, []string{rwID})

	rtID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
	var prevRwID sql.NullString
	_ = pool.Raw().QueryRowContext(context.Background(), `SELECT rw_id FROM rts WHERE id = $1`, rtID).Scan(&prevRwID)
	_, _ = pool.Raw().ExecContext(context.Background(), `UPDATE rts SET rw_id = $1 WHERE id = $2`, rwID, rtID)
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(context.Background(), `UPDATE rts SET rw_id = $1 WHERE id = $2`, prevRwID, rtID)
	})
	_, tokenRW := createRtRwIsoRWUserToken(t, pool, rwID, auth.RoleBendahara, auth.JabatanBendahara)
	locRW := createTestRtRwIsoRWLocation(t, pool, rwID, "Kas Tunai RW Resmi", "coh", "500000.00")

	kasBody, _ := json.Marshal(map[string]any{
		"type":           "income",
		"amount":         "250000.00",
		"location_id":    locRW,
		"source_rt_id":   rtID,
		"description":    "Setoran iuran warga dari RT 002 ke Kas RW",
		"payment_method": "CASH",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rw/%s/kas/transactions", rwID), bytes.NewReader(kasBody))
	req.Header.Set("Authorization", "Bearer "+tokenRW)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for legitimate RT to RW Kas Income, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// Verify database: rw_transactions has source_rt_id recorded
	var savedSourceRTID sql.NullString
	var savedAmount string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT source_rt_id, amount::text FROM rw_transactions WHERE rw_id = $1 AND location_id = $2`,
		rwID, locRW,
	).Scan(&savedSourceRTID, &savedAmount)
	if err != nil {
		t.Fatalf("query rw_transactions: %v", err)
	}
	if !savedSourceRTID.Valid || savedSourceRTID.String != rtID {
		t.Fatalf("expected source_rt_id %s, got %v", rtID, savedSourceRTID)
	}
	if savedAmount != "250000.00" {
		t.Fatalf("expected amount 250000.00, got %s", savedAmount)
	}

	// Verify balance mutation: RW balance increased by 250000.00, RT remained untouched
	var rwBal string
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COALESCE(SUM(amount_delta), 0)::text FROM rw_location_balance_changes WHERE location_id = $1`,
		locRW,
	).Scan(&rwBal)
	if rwBal != "750000.00" { // 500,000 opening + 250,000 income
		t.Fatalf("expected RW balance 750000.00, got %s", rwBal)
	}
}

// -----------------------------------------------------------------------------
// Test 12: Saldo dan histori RT serta RW tetap independen (Zero Fallback)
// -----------------------------------------------------------------------------
func TestIsolation_RT_RW_LedgersAndLocationsStrictlyIndependent(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createTestRtRwIsoRW(t, pool, "RW Pure Independent")
	_, router := setupRtRwIsoRouter(t, []string{rwID})

	rtID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
	_, tokenRW := createRtRwIsoRWUserToken(t, pool, rwID, auth.RoleBendahara, auth.JabatanBendahara)

	// RW currently has 0 locations. Check GET locations returns empty array (NO fallback to RT locations)
	reqLocs := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rw/%s/locations", rwID), nil)
	reqLocs.Header.Set("Authorization", "Bearer "+tokenRW)
	recLocs := httptest.NewRecorder()
	router.ServeHTTP(recLocs, reqLocs)
	if recLocs.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recLocs.Code)
	}

	var locResp struct {
		Data []any `json:"data"`
	}
	if err := json.NewDecoder(recLocs.Body).Decode(&locResp); err != nil {
		t.Fatalf("decode locations response: %v", err)
	}
	if len(locResp.Data) != 0 {
		t.Fatalf("expected 0 locations in newly created RW (zero fallback to RT), got %d: %+v", len(locResp.Data), locResp.Data)
	}

	// Check GET dashboard total_locations is 0 (zero fallback to RT)
	reqDash := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rw/%s/dashboard", rwID), nil)
	reqDash.Header.Set("Authorization", "Bearer "+tokenRW)
	recDash := httptest.NewRecorder()
	router.ServeHTTP(recDash, reqDash)
	if recDash.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for dashboard, got %d", recDash.Code)
	}
	var dashResp struct {
		Data struct {
			TotalLocations int `json:"total_locations"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recDash.Body).Decode(&dashResp); err != nil {
		t.Fatalf("decode dashboard response: %v", err)
	}
	if dashResp.Data.TotalLocations != 0 {
		t.Fatalf("expected total_locations 0 in RW dashboard (no RT leakage), got %d", dashResp.Data.TotalLocations)
	}

	// Create RT location in RT 002 with 1,000,000 balance
	locRT := createTestRtRwIsoRTLocation(t, pool, rtID, "Kas RT 002 Mandiri", "coh", "1000000.00")

	// RW dashboard must STILL report 0 locations
	recDash2 := httptest.NewRecorder()
	router.ServeHTTP(recDash2, reqDash)
	var dashResp2 struct {
		Data struct {
			TotalLocations int `json:"total_locations"`
		} `json:"data"`
	}
	json.NewDecoder(recDash2.Body).Decode(&dashResp2)
	if dashResp2.Data.TotalLocations != 0 {
		t.Fatalf("RT location leaked into RW dashboard! total_locations=%d", dashResp2.Data.TotalLocations)
	}

	// Verify RT location balance remains 1,000,000
	var rtBal string
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COALESCE(SUM(amount_delta), 0)::text FROM location_balance_changes WHERE location_id = $1`,
		locRT,
	).Scan(&rtBal)
	if rtBal != "1000000.00" {
		t.Fatalf("expected RT balance 1000000.00, got %s", rtBal)
	}
}
