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

var rwSecuritySeq uint64 = uint64(time.Now().UnixNano()%1000000 + 5000000)

func setupRWSecurityRouter(t *testing.T, rwIDs []string) (*database.Pool, http.Handler) {
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
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: rwIDs,
	}
	router := buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
	return pool, router
}

func createTestRWSecurityRW(t *testing.T, pool *database.Pool, name string) string {
	t.Helper()
	rwNum := int(atomic.AddUint64(&rwSecuritySeq, 1))
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

func createTestRWSecurityLocation(t *testing.T, pool *database.Pool, rwID, name, category, initialBal string) string {
	t.Helper()
	var locID string
	var bankName, accNo, accHolder sql.NullString
	if category == "rekening" {
		bankName = sql.NullString{String: "BCA", Valid: true}
		accNo = sql.NullString{String: "1234567890", Valid: true}
		accHolder = sql.NullString{String: "Kas RW", Valid: true}
	}
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rw_fund_locations (id, rw_id, name, category, bank_name, account_number, account_holder, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, true)
		 RETURNING id`,
		rwID, name, category, bankName, accNo, accHolder,
	).Scan(&locID)
	if err != nil {
		t.Fatalf("create test location %s: %v", name, err)
	}

	if initialBal != "" && initialBal != "0" {
		_, err = pool.Raw().ExecContext(context.Background(),
			`INSERT INTO rw_location_balance_changes (id, rw_id, location_id, change_type, amount_delta, notes)
			 VALUES (gen_random_uuid(), $1, $2, 'opening_balance', $3::numeric, 'Initial opening balance')`,
			rwID, locID, initialBal,
		)
		if err != nil {
			t.Fatalf("insert opening balance change for %s: %v", name, err)
		}
	}
	return locID
}

func createTestRWSecurityTransfer(t *testing.T, pool *database.Pool, rwID, fromLocID, toLocID, amount, userID string) string {
	t.Helper()
	ctx := context.Background()
	var transferID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rw_fund_transfers (id, rw_id, from_location_id, to_location_id, amount, transfer_date, notes, created_by)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4::numeric, CURRENT_DATE, 'Initial test transfer', $5)
		 RETURNING id`,
		rwID, fromLocID, toLocID, amount, userID,
	).Scan(&transferID)
	if err != nil {
		t.Fatalf("insert test transfer: %v", err)
	}

	// Two mutation records in rw_location_balance_changes: transfer_out (-), transfer_in (+)
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (id, rw_id, location_id, transfer_id, change_type, amount_delta, notes, created_by)
		 VALUES (gen_random_uuid(), $1, $2, $3, 'transfer_out', -1 * $4::numeric, 'Transfer keluar', $5)`,
		rwID, fromLocID, transferID, amount, userID,
	)
	if err != nil {
		t.Fatalf("insert transfer_out mutation: %v", err)
	}

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (id, rw_id, location_id, transfer_id, change_type, amount_delta, notes, created_by)
		 VALUES (gen_random_uuid(), $1, $2, $3, 'transfer_in', $4::numeric, 'Transfer masuk', $5)`,
		rwID, toLocID, transferID, amount, userID,
	)
	if err != nil {
		t.Fatalf("insert transfer_in mutation: %v", err)
	}

	return transferID
}

func createRWSecuritySuperAdminToken(t *testing.T, pool *database.Pool) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("sa-sec-%d@test.com", atomic.AddUint64(&rwSecuritySeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'Super Admin Security Test', 'super_admin', true)
		 RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("create super admin user: %v", err)
	}
	t.Cleanup(func() {
		pool.Raw().ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		SysRole: string(auth.SystemRoleSuperAdmin),
	})
	if err != nil {
		t.Fatalf("generate SA token: %v", err)
	}
	return userID, token
}

func createRWSecurityUserToken(t *testing.T, pool *database.Pool, rwID string, role auth.Role, jabatan auth.Jabatan) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rw-user-%s-%d@test.com", jabatan, atomic.AddUint64(&rwSecuritySeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RW Security User', NULL, true)
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
		t.Fatalf("generate rw user token: %v", err)
	}
	return userID, token
}

type rwSecurityDBState struct {
	txCount       int
	transferCount int
	changeCount   int
	totalBalance  string
	locationBals  map[string]string
}

func getRWSecurityDBState(t *testing.T, pool *database.Pool, rwID string, locIDs ...string) rwSecurityDBState {
	t.Helper()
	ctx := context.Background()
	var state rwSecurityDBState
	state.locationBals = make(map[string]string)

	err := pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_transactions WHERE rw_id = $1`, rwID,
	).Scan(&state.txCount)
	if err != nil {
		t.Fatalf("get txCount: %v", err)
	}

	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_fund_transfers WHERE rw_id = $1`, rwID,
	).Scan(&state.transferCount)
	if err != nil {
		t.Fatalf("get transferCount: %v", err)
	}

	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_location_balance_changes WHERE rw_id = $1`, rwID,
	).Scan(&state.changeCount)
	if err != nil {
		t.Fatalf("get changeCount: %v", err)
	}

	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_delta), 0)::numeric(15,2)::text
		 FROM rw_location_balance_changes WHERE rw_id = $1`, rwID,
	).Scan(&state.totalBalance)
	if err != nil {
		t.Fatalf("get totalBalance: %v", err)
	}

	for _, locID := range locIDs {
		var bal string
		err = pool.Raw().QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount_delta), 0)::numeric(15,2)::text
			 FROM rw_location_balance_changes WHERE rw_id = $1 AND location_id = $2`,
			rwID, locID,
		).Scan(&bal)
		if err != nil {
			t.Fatalf("get loc bal for %s: %v", locID, err)
		}
		state.locationBals[locID] = bal
	}

	return state
}

// -----------------------------------------------------------------------------
// Test 1: Super Admin Out-of-Scope Mutation Blocked
// Scenario:
//   - Super Admin is configured with scope only for RW A (SCOPED_RW_IDS).
//   - Super Admin sends HTTP POST mutation to RW B's transfer and transaction endpoints.
//   - Both requests MUST return 403 Forbidden.
//   - RW B's database state (transactions, transfers, mutations, balances) remains untouched.
// -----------------------------------------------------------------------------
func TestSecurity_SuperAdminOutOfScopeMutationBlocked(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwAID := createTestRWSecurityRW(t, pool, "RW A Allowed Scope")
	rwBID := createTestRWSecurityRW(t, pool, "RW B Out of Scope")

	// Super Admin is scoped ONLY to RW A
	_, router := setupRWSecurityRouter(t, []string{rwAID})

	// Setup locations with balances in RW B
	locB1 := createTestRWSecurityLocation(t, pool, rwBID, "Kas Tunai RW B", "coh", "1000000.00")
	locB2 := createTestRWSecurityLocation(t, pool, rwBID, "Rekening BCA RW B", "rekening", "500000.00")

	_, saToken := createRWSecuritySuperAdminToken(t, pool)

	// Snapshot state of RW B before requests
	stateBefore := getRWSecurityDBState(t, pool, rwBID, locB1, locB2)

	// 1. Attempt transfer mutation on RW B
	transferPayload, _ := json.Marshal(map[string]any{
		"from_location_id": locB1,
		"to_location_id":   locB2,
		"amount":           "200000.00",
		"notes":            "Illegal transfer across out-of-scope RW",
	})
	reqTransfer := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rw/%s/locations/transfers", rwBID), bytes.NewReader(transferPayload))
	reqTransfer.Header.Set("Authorization", "Bearer "+saToken)
	reqTransfer.Header.Set("Content-Type", "application/json")
	recTransfer := httptest.NewRecorder()
	router.ServeHTTP(recTransfer, reqTransfer)

	if recTransfer.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for out-of-scope SA transfer, got %d. Body: %s", recTransfer.Code, recTransfer.Body.String())
	}

	// 2. Attempt kas transaction mutation on RW B
	txPayload, _ := json.Marshal(map[string]any{
		"type":           "income",
		"amount":         "150000.00",
		"location_id":    locB1,
		"description":    "Illegal kas income across out-of-scope RW",
		"payment_method": "CASH",
	})
	reqTx := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rw/%s/kas/transactions", rwBID), bytes.NewReader(txPayload))
	reqTx.Header.Set("Authorization", "Bearer "+saToken)
	reqTx.Header.Set("Content-Type", "application/json")
	recTx := httptest.NewRecorder()
	router.ServeHTTP(recTx, reqTx)

	if recTx.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for out-of-scope SA kas transaction, got %d. Body: %s", recTx.Code, recTx.Body.String())
	}

	// Verify zero side effects on RW B
	stateAfter := getRWSecurityDBState(t, pool, rwBID, locB1, locB2)
	if stateAfter.txCount != stateBefore.txCount {
		t.Fatalf("side effect detected: txCount changed from %d to %d", stateBefore.txCount, stateAfter.txCount)
	}
	if stateAfter.transferCount != stateBefore.transferCount {
		t.Fatalf("side effect detected: transferCount changed from %d to %d", stateBefore.transferCount, stateAfter.transferCount)
	}
	if stateAfter.changeCount != stateBefore.changeCount {
		t.Fatalf("side effect detected: changeCount changed from %d to %d", stateBefore.changeCount, stateAfter.changeCount)
	}
	if stateAfter.totalBalance != stateBefore.totalBalance {
		t.Fatalf("side effect detected: totalBalance changed from %s to %s", stateBefore.totalBalance, stateAfter.totalBalance)
	}
	for locID, beforeBal := range stateBefore.locationBals {
		if afterBal := stateAfter.locationBals[locID]; afterBal != beforeBal {
			t.Fatalf("side effect detected: loc %s balance changed from %s to %s", locID, beforeBal, afterBal)
		}
	}
}

// -----------------------------------------------------------------------------
// Test 2: Cross-RW Reversal Blocked
// Scenario:
//   - Bendahara belongs to RW A.
//   - A valid transfer exists in RW B.
//   - Bendahara sends HTTP POST reversal to RW B's router endpoint -> 403 Forbidden.
//   - Bendahara sends HTTP POST reversal to RW A's endpoint using RW B's transfer ID -> 404 Not Found.
//   - RW B's original transfer, mutations, and location balances remain strictly unchanged.
// -----------------------------------------------------------------------------
func TestSecurity_CrossRWReversalBlocked(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwAID := createTestRWSecurityRW(t, pool, "RW A Bendahara Home")
	rwBID := createTestRWSecurityRW(t, pool, "RW B Target Reversal")

	// Router scoping allows both RWs at deployment level so membership gate is tested
	_, router := setupRWSecurityRouter(t, []string{rwAID, rwBID})

	// Bendahara in RW A
	_, bendaharaTokenA := createRWSecurityUserToken(t, pool, rwAID, auth.RoleBendahara, auth.JabatanBendahara)

	// Valid transfer setup in RW B
	locB1 := createTestRWSecurityLocation(t, pool, rwBID, "Kas Utama RW B", "coh", "2000000.00")
	locB2 := createTestRWSecurityLocation(t, pool, rwBID, "Rekening RW B", "rekening", "1000000.00")
	userBID, _ := createRWSecurityUserToken(t, pool, rwBID, auth.RoleBendahara, auth.JabatanBendahara)
	transferBID := createTestRWSecurityTransfer(t, pool, rwBID, locB1, locB2, "500000.00", userBID)

	stateBefore := getRWSecurityDBState(t, pool, rwBID, locB1, locB2)

	// 1. Bendahara A attempts reversal via RW B's endpoint directly
	revPayload, _ := json.Marshal(map[string]any{
		"reason": "Cross-RW illegal reversal attempt by Bendahara A",
	})
	reqDirect := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/rw/%s/locations/transfers/%s/reverse", rwBID, transferBID),
		bytes.NewReader(revPayload),
	)
	reqDirect.Header.Set("Authorization", "Bearer "+bendaharaTokenA)
	reqDirect.Header.Set("Content-Type", "application/json")
	recDirect := httptest.NewRecorder()
	router.ServeHTTP(recDirect, reqDirect)

	if recDirect.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when Bendahara A accesses RW B endpoint, got %d. Body: %s", recDirect.Code, recDirect.Body.String())
	}

	// 2. Bendahara A attempts reversal via RW A's endpoint targeting RW B's transfer ID
	reqCrossID := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/rw/%s/locations/transfers/%s/reverse", rwAID, transferBID),
		bytes.NewReader(revPayload),
	)
	reqCrossID.Header.Set("Authorization", "Bearer "+bendaharaTokenA)
	reqCrossID.Header.Set("Content-Type", "application/json")
	recCrossID := httptest.NewRecorder()
	router.ServeHTTP(recCrossID, reqCrossID)

	if recCrossID.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found when referencing foreign transfer ID in own RW, got %d. Body: %s", recCrossID.Code, recCrossID.Body.String())
	}

	// Verify zero side effects on RW B
	stateAfter := getRWSecurityDBState(t, pool, rwBID, locB1, locB2)
	if stateAfter.transferCount != stateBefore.transferCount {
		t.Fatalf("side effect detected: transferCount changed from %d to %d", stateBefore.transferCount, stateAfter.transferCount)
	}
	if stateAfter.changeCount != stateBefore.changeCount {
		t.Fatalf("side effect detected: changeCount changed from %d to %d", stateBefore.changeCount, stateAfter.changeCount)
	}
	if stateAfter.totalBalance != stateBefore.totalBalance {
		t.Fatalf("side effect detected: totalBalance changed from %s to %s", stateBefore.totalBalance, stateAfter.totalBalance)
	}
	for locID, beforeBal := range stateBefore.locationBals {
		if afterBal := stateAfter.locationBals[locID]; afterBal != beforeBal {
			t.Fatalf("side effect detected: loc %s balance changed from %s to %s", locID, beforeBal, afterBal)
		}
	}
}

// -----------------------------------------------------------------------------
// Test 3: Wakil Ketua Direct HTTP Mutation Blocked
// Scenario:
//   - Wakil Ketua RW has finance.create and finance.approve, but lacks finance.update.
//   - Sends HTTP POST transfer -> 403 Forbidden.
//   - Sends HTTP POST reversal -> 403 Forbidden.
//   - Zero side effects: no transfers created, no reversal records inserted.
//   - Positive control: Wakil Ketua sends HTTP POST kas transaction (finance.create) -> 201 Created.
// -----------------------------------------------------------------------------
func TestSecurity_WakilKetuaDirectHTTPMutationBlocked(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwAID := createTestRWSecurityRW(t, pool, "RW A Wakil Ketua Scope")

	_, router := setupRWSecurityRouter(t, []string{rwAID})

	// Wakil Ketua in RW A
	_, wakilKetuaToken := createRWSecurityUserToken(t, pool, rwAID, auth.RoleWarga, auth.JabatanWakilKetua)

	// Locations and existing transfer created by bendahara
	locA1 := createTestRWSecurityLocation(t, pool, rwAID, "Kas Brankas RW A", "brankas", "1500000.00")
	locA2 := createTestRWSecurityLocation(t, pool, rwAID, "Kas Tunai RW A", "coh", "800000.00")
	bendaharaID, _ := createRWSecurityUserToken(t, pool, rwAID, auth.RoleBendahara, auth.JabatanBendahara)
	transferAID := createTestRWSecurityTransfer(t, pool, rwAID, locA1, locA2, "200000.00", bendaharaID)

	stateBefore := getRWSecurityDBState(t, pool, rwAID, locA1, locA2)

	// 1. Wakil Ketua sends HTTP POST transfer (requires finance.update)
	transferPayload, _ := json.Marshal(map[string]any{
		"from_location_id": locA1,
		"to_location_id":   locA2,
		"amount":           "100000.00",
		"notes":            "Attempted transfer by Wakil Ketua lacking finance.update",
	})
	reqTransfer := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/rw/%s/locations/transfers", rwAID),
		bytes.NewReader(transferPayload),
	)
	reqTransfer.Header.Set("Authorization", "Bearer "+wakilKetuaToken)
	reqTransfer.Header.Set("Content-Type", "application/json")
	recTransfer := httptest.NewRecorder()
	router.ServeHTTP(recTransfer, reqTransfer)

	if recTransfer.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Wakil Ketua creating transfer, got %d. Body: %s", recTransfer.Code, recTransfer.Body.String())
	}

	// 2. Wakil Ketua sends HTTP POST reversal (requires finance.update)
	revPayload, _ := json.Marshal(map[string]any{
		"reason": "Attempted reversal by Wakil Ketua lacking finance.update",
	})
	reqRev := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/rw/%s/locations/transfers/%s/reverse", rwAID, transferAID),
		bytes.NewReader(revPayload),
	)
	reqRev.Header.Set("Authorization", "Bearer "+wakilKetuaToken)
	reqRev.Header.Set("Content-Type", "application/json")
	recRev := httptest.NewRecorder()
	router.ServeHTTP(recRev, reqRev)

	if recRev.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Wakil Ketua reversing transfer, got %d. Body: %s", recRev.Code, recRev.Body.String())
	}

	// Verify zero side effects from the two blocked operations
	stateMid := getRWSecurityDBState(t, pool, rwAID, locA1, locA2)
	if stateMid.transferCount != stateBefore.transferCount {
		t.Fatalf("side effect detected: transferCount changed from %d to %d", stateBefore.transferCount, stateMid.transferCount)
	}
	if stateMid.changeCount != stateBefore.changeCount {
		t.Fatalf("side effect detected: changeCount changed from %d to %d", stateBefore.changeCount, stateMid.changeCount)
	}
	if stateMid.totalBalance != stateBefore.totalBalance {
		t.Fatalf("side effect detected: totalBalance changed from %s to %s", stateBefore.totalBalance, stateMid.totalBalance)
	}
	for locID, beforeBal := range stateBefore.locationBals {
		if midBal := stateMid.locationBals[locID]; midBal != beforeBal {
			t.Fatalf("side effect detected: loc %s balance changed from %s to %s", locID, beforeBal, midBal)
		}
	}

	// 3. Positive control: Wakil Ketua sends HTTP POST kas transaction (finance.create allowed)
	txPayload, _ := json.Marshal(map[string]any{
		"type":           "income",
		"amount":         "50000.00",
		"location_id":    locA1,
		"description":    "Kas Masuk Sah oleh Wakil Ketua",
		"payment_method": "CASH",
	})
	reqTx := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/rw/%s/kas/transactions", rwAID),
		bytes.NewReader(txPayload),
	)
	reqTx.Header.Set("Authorization", "Bearer "+wakilKetuaToken)
	reqTx.Header.Set("Content-Type", "application/json")
	recTx := httptest.NewRecorder()
	router.ServeHTTP(recTx, reqTx)

	// Per Final RBAC Matrix: Wakil Ketua (Pengurus RW) is strictly read-only and blocked from creating kas transactions
	if recTx.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Wakil Ketua creating kas transaction per Final RBAC matrix, got %d. Body: %s", recTx.Code, recTx.Body.String())
	}

	// Verify Wakil Ketua cannot mutate: txCount and changeCount must remain unchanged
	stateAfter := getRWSecurityDBState(t, pool, rwAID, locA1, locA2)
	if stateAfter.txCount != stateBefore.txCount {
		t.Fatalf("expected txCount to remain unchanged, got %d (was %d)", stateAfter.txCount, stateBefore.txCount)
	}
	if stateAfter.transferCount != stateBefore.transferCount {
		t.Fatalf("transferCount unexpectedly changed: got %d (was %d)", stateAfter.transferCount, stateBefore.transferCount)
	}
	if stateAfter.changeCount != stateBefore.changeCount {
		t.Fatalf("expected changeCount to remain unchanged, got %d (was %d)", stateAfter.changeCount, stateBefore.changeCount)
	}
}
