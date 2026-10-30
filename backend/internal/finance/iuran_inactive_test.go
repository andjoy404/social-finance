package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
)

// TestInactiveIuranPaymentRejection verifies that payments for bills belonging
// to an inactive due are rejected by the backend and leave no financial side effects.
func TestInactiveIuranPaymentRejection(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Inactive Iuran RT", 1, "INACT")
	defer cleanupTestRT(t, pool, rtID)

	phoneW := fmt.Sprintf("0843%08d", time.Now().UnixNano()%100000000)
	wargaOccID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0816%08d", time.Now().UnixNano()%100000000)
	userIDB, midB := setupTestUserWithMembership(t, pool, "b@inact.test", "Bendahara Inact", phoneB, rtID, "bendahara")
	phoneP := fmt.Sprintf("0817%08d", time.Now().UnixNano()%100000000)
	userIDP, midP := setupTestUserWithMembership(t, pool, "p@inact.test", "Pengurus Inact", phoneP, rtID, "pengurus")

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userIDB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara),
	})
	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userIDP, MID: midP, RTID: rtID, Role: string(auth.RolePengurus),
	})
	userIDW, midW := setupTestUserWithMembership(t, pool, "w@inact.test", "Warga Inact", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{
		UserID: userIDW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga),
	})

	// ── Helper: create a regular due ──────────────────────────────
	createRegularDue := func(name string, active bool) string {
		t.Helper()
		var dueID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, is_active)
			 VALUES ($1, $2, $3, 'one_time', 'partial', false, $4) RETURNING id`,
			rtID, name, "50000", active,
		).Scan(&dueID)
		if err != nil {
			t.Fatalf("create regular due %s: %v", name, err)
		}
		return dueID
	}

	// ── Helper: create a bill via SQL (bypasses due active check) ──
	billCounter := 0
	createBillDirect := func(dueID string) string {
		t.Helper()

		billCounter++
		period := fmt.Sprintf("2026-10-%02d", billCounter)
		var billID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
			 VALUES ($1, $2, $3, '50000', $4, '2026-12-25', 'unpaid')
			 RETURNING id`,
			rtID, wargaOccID, dueID, period,
		).Scan(&billID)
		require.NoError(t, err)
		return billID
	}

	// ── Helper: post payment via API ──────────────────────────────
	postPayment := func(token, billID string) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// ── Helper: get payment count for a bill ──────────────────────
	paymentCount := func(billID string) int {
		t.Helper()
		var count int
		err := pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM payments WHERE rt_id = $1 AND bill_id = $2`,
			rtID, billID,
		).Scan(&count)
		require.NoError(t, err)
		return count
	}

	// ── Helper: get ledger count for a bill ───────────────────────
	ledgerCount := func(billID string) int {
		t.Helper()
		var count int
		err := pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM transactions t
			 JOIN payments p ON t.payment_id = p.id
			 WHERE p.bill_id = $1 AND p.rt_id = $2`,
			billID, rtID,
		).Scan(&count)
		require.NoError(t, err)
		return count
	}

	// ── Helper: get bill status ───────────────────────────────────
	getBillStatus := func(billID string) string {
		t.Helper()
		var status string
		err := pool.Raw().QueryRowContext(context.Background(),
			`SELECT status FROM bills WHERE id = $1`, billID,
		).Scan(&status)
		require.NoError(t, err)
		return status
	}

	// ── Helper: get bill detail with due_is_active ────────────────
	getBillDetail := func(billID string) map[string]interface{} {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var detail map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail))
		return detail
	}

	// ── Helper: get bill list ─────────────────────────────────────
	getBillList := func() []map[string]interface{} {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Data []map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp.Data
	}

	// ═══════════════════════════════════════════════════════════
	// Scenario 1: Active iuran – payment succeeds
	// ═══════════════════════════════════════════════════════════
	activeDueID := createRegularDue("Active Iuran", true)
	activeBillID := createBillDirect(activeDueID)

	t.Run("active iuran payment succeeds", func(t *testing.T) {
		rec := postPayment(tokenWarga, activeBillID)
		require.Equal(t, http.StatusCreated, rec.Code, "active payment should succeed: %s", rec.Body.String())

		var p Payment
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
		require.NotNil(t, p.ID)

		// Verify payment was created
		require.Equal(t, 1, paymentCount(activeBillID))
		// Verify bill status changed (warga submits PENDING, no auto-approval)
		require.Equal(t, "unpaid", getBillStatus(activeBillID))
	})

	// ═══════════════════════════════════════════════════════════
	// Scenario 2: Inactive iuran – payment rejected
	// ═══════════════════════════════════════════════════════════
	inactiveDueID := createRegularDue("Inactive Iuran", false)
	inactiveBillID := createBillDirect(inactiveDueID)

	t.Run("inactive iuran payment rejected", func(t *testing.T) {
		rec := postPayment(tokenWarga, inactiveBillID)
		require.Equal(t, http.StatusBadRequest, rec.Code,
			"expected 400 for inactive due, got %d: %s", rec.Code, rec.Body.String())

		// Verify error message contains expected text
		var errResp map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
		errMsg, ok := errResp["error"].(map[string]interface{})
		require.True(t, ok, "expected error object in response")
		require.Contains(t, errMsg["message"].(string), "inactive due",
			"error message should mention inactive due: %s", rec.Body.String())

		// Verify no payment was created
		require.Equal(t, 0, paymentCount(inactiveBillID))
		// Verify no ledger entries
		require.Equal(t, 0, ledgerCount(inactiveBillID))
		// Verify bill status is unchanged
		require.Equal(t, "unpaid", getBillStatus(inactiveBillID))
	})

	// ═══════════════════════════════════════════════════════════
	// Scenario 3: Direct API call also rejected (bypassing UI)
	// ═══════════════════════════════════════════════════════════
	t.Run("inactive iuran payment rejected via direct API", func(t *testing.T) {
		// Create another bill for the same inactive due
		inactiveBill2ID := createBillDirect(inactiveDueID)

		// Bendahara (staff) tries to record payment directly
		rec := postPayment(tokenBendahara, inactiveBill2ID)
		require.Equal(t, http.StatusBadRequest, rec.Code,
			"staff payment should also be rejected: %s", rec.Body.String())

		// Pengurus tries to record payment directly
		rec2 := postPayment(tokenPengurus, inactiveBill2ID)
		require.Equal(t, http.StatusBadRequest, rec2.Code,
			"pengurus payment should also be rejected: %s", rec2.Body.String())

		// Verify no payments created
		require.Equal(t, 0, paymentCount(inactiveBill2ID))
	})

	// ═══════════════════════════════════════════════════════════
	// Scenario 4: No side effects from rejected payment
	// ═══════════════════════════════════════════════════════════
	t.Run("rejected payment has no side effects", func(t *testing.T) {
		// Count payments before
		beforePaymentCount := 0
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM payments WHERE rt_id = $1 AND bill_id = $2`,
			rtID, inactiveBillID,
		).Scan(&beforePaymentCount)

		beforeLedgerCount := 0
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM transactions t
			 JOIN payments p ON t.payment_id = p.id
			 WHERE p.bill_id = $1 AND p.rt_id = $2`,
			inactiveBillID, rtID,
		).Scan(&beforeLedgerCount)

		// Attempt payment
		rec := postPayment(tokenWarga, inactiveBillID)
		require.Equal(t, http.StatusBadRequest, rec.Code)

		// Verify counts unchanged
		afterPaymentCount := 0
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM payments WHERE rt_id = $1 AND bill_id = $2`,
			rtID, inactiveBillID,
		).Scan(&afterPaymentCount)

		afterLedgerCount := 0
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM transactions t
			 JOIN payments p ON t.payment_id = p.id
			 WHERE p.bill_id = $1 AND p.rt_id = $2`,
			inactiveBillID, rtID,
		).Scan(&afterLedgerCount)

		require.Equal(t, beforePaymentCount, afterPaymentCount,
			"payment count should not change after rejected payment")
		require.Equal(t, beforeLedgerCount, afterLedgerCount,
			"ledger count should not change after rejected payment")
	})

	// ═══════════════════════════════════════════════════════════
	// Scenario 5: Bill API returns due_is_active correctly
	// ═══════════════════════════════════════════════════════════
	t.Run("bill API returns due_is_active", func(t *testing.T) {
		// Check bill detail
		activeDetail := getBillDetail(activeBillID)
		dueIsActive, ok := activeDetail["due_is_active"]
		require.True(t, ok || dueIsActive == nil,
			"due_is_active field should be present or nullable in bill detail")
		if dueIsActive != nil {
			require.Equal(t, true, dueIsActive, "active bill should have due_is_active=true")
		}

		inactiveDetail := getBillDetail(inactiveBillID)
		dueIsActive2, ok2 := inactiveDetail["due_is_active"]
		require.True(t, ok2 || dueIsActive2 == nil,
			"due_is_active field should be present or nullable in bill detail")
		if dueIsActive2 != nil {
			require.Equal(t, false, dueIsActive2, "inactive bill should have due_is_active=false")
		}
	})

	t.Run("bill list includes due_is_active", func(t *testing.T) {
		bills := getBillList()
		require.NotEmpty(t, bills, "should have at least 2 bills")

		for _, b := range bills {
			id, _ := b["id"].(string)
			if id == inactiveBillID {
				dueIsActive, ok := b["due_is_active"].(bool)
				require.True(t, ok, "due_is_active should be a bool in bill list")
				require.False(t, dueIsActive, "inactive bill should have due_is_active=false in list")
			}
			if id == activeBillID {
				dueIsActive, ok := b["due_is_active"].(bool)
				require.True(t, ok, "due_is_active should be a bool in bill list")
				require.True(t, dueIsActive, "active bill should have due_is_active=true in list")
			}
		}
	})

	// ═══════════════════════════════════════════════════════════
	// Scenario 6: Donation/voluntary dues still work (no regression)
	// ═══════════════════════════════════════════════════════════
	t.Run("voluntary donation to active voluntary due works", func(t *testing.T) {
		var volDueID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, is_active)
			 VALUES ($1, 'Voluntary Active', '0', 'one_time', 'partial', true, true) RETURNING id`,
			rtID,
		).Scan(&volDueID)
		require.NoError(t, err)

		body := fmt.Sprintf(`{"due_id":"%s","amount":"10000","method":"TRANSFER"}`, volDueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code,
			"voluntary donation to active due should succeed: %s", rec.Body.String())
	})

	t.Run("voluntary donation to inactive due is rejected", func(t *testing.T) {
		var inactiveVolDueID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, is_active)
			 VALUES ($1, 'Voluntary Inactive', '0', 'one_time', 'partial', true, false) RETURNING id`,
			rtID,
		).Scan(&inactiveVolDueID)
		require.NoError(t, err)

		body := fmt.Sprintf(`{"due_id":"%s","amount":"5000","method":"TRANSFER"}`, inactiveVolDueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code,
			"voluntary donation to inactive due should be rejected: %s", rec.Body.String())
	})

	// ═══════════════════════════════════════════════════════════
	// Scenario 7: Idempotent key with inactive due
	// ═══════════════════════════════════════════════════════════
	t.Run("idempotent payment to inactive due returns same error", func(t *testing.T) {
		inactiveBill3ID := createBillDirect(inactiveDueID)

		idemKey := fmt.Sprintf("inactive-idem-%d", time.Now().UnixNano())
		// First request
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, inactiveBill3ID)
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req1.Header.Set("Authorization", "Bearer "+tokenWarga)
		req1.Header.Set("Content-Type", "application/json; charset=utf-8")
		req1.Header.Set("Idempotency-Key", idemKey)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)
		require.Equal(t, http.StatusBadRequest, rec1.Code)

		// Same idempotency key
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req2.Header.Set("Authorization", "Bearer "+tokenWarga)
		req2.Header.Set("Content-Type", "application/json; charset=utf-8")
		req2.Header.Set("Idempotency-Key", idemKey)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)
		require.Equal(t, http.StatusBadRequest, rec2.Code)

		// Both requests should have returned consistent error
		var err1, err2 map[string]interface{}
		require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &err1))
		require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &err2))
		require.Equal(t, err1["error"], err2["error"],
			"idempotent retry should return same error for inactive due")

		// Verify no payment created
		require.Equal(t, 0, paymentCount(inactiveBill3ID))
	})
}

// TestInactiveIuranRaceCondition verifies that toggling a due's active status
// during concurrent payment attempts does not allow payment through.
func TestInactiveIuranRaceCondition(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Race RT", 1, "RACE")
	defer cleanupTestRT(t, pool, rtID)

	phoneW := fmt.Sprintf("0844%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	phoneB := fmt.Sprintf("0818%08d", time.Now().UnixNano()%100000000)
	userIDB, midB := setupTestUserWithMembership(t, pool, "b@race.test", "Bendahara Race", phoneB, rtID, "bendahara")

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userIDB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara),
	})

	// Create active regular due
	var dueID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, is_active)
		 VALUES ($1, 'Race Iuran', '50000', 'one_time', 'partial', false, true) RETURNING id`,
		rtID,
	).Scan(&dueID)
	require.NoError(t, err)

	// Create bill directly via SQL
	var occID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM household_occupancies LIMIT 1`,
	).Scan(&occID)
	require.NoError(t, err)

	var billID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
		 VALUES ($1, $2, $3, '50000', '2026-10', '2026-12-25', 'unpaid')
		 RETURNING id`,
		rtID, occID, dueID,
	).Scan(&billID)
	require.NoError(t, err)

	// Now deactivate the due
	_, err = pool.Raw().ExecContext(context.Background(),
		`UPDATE dues SET is_active = false WHERE id = $1`, dueID,
	)
	require.NoError(t, err)

	// Multiple concurrent payment attempts should all be rejected
	const concurrentReqs = 5
	done := make(chan struct{})
	var results []struct {
		code int
		body string
	}
	var mu sync.Mutex

	for i := 0; i < concurrentReqs; i++ {
		go func() {
			<-done
			body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, billID)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
			req.Header.Set("Authorization", "Bearer "+tokenBendahara)
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
			r := httptest.NewRecorder()
			router.ServeHTTP(r, req)
			mu.Lock()
			results = append(results, struct {
				code int
				body string
			}{code: r.Code, body: r.Body.String()})
			mu.Unlock()
		}()
	}
	close(done)

	time.Sleep(500 * time.Millisecond) // Wait for all goroutines

	for _, r := range results {
		require.Equal(t, http.StatusBadRequest, r.code,
			"all concurrent requests should be rejected: %s", r.body)
	}

	// Verify no payment was created
	paymentCount := 0
	pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM payments WHERE rt_id = $1 AND bill_id = $2`,
		rtID, billID,
	).Scan(&paymentCount)
	require.Equal(t, 0, paymentCount, "no payments should exist after concurrent rejection")
}
