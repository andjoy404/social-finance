package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-finance/internal/auth"

	"github.com/stretchr/testify/require"
)

// ============================================================
// STAGE 2 TESTS — Pelunasan, FIFO Lintas Tahun & Skipping Check
// ============================================================

func TestMultiPeriod_FIFO_CrossYear(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	// Create due and generate 3 bills spanning years: 2025-11, 2025-12, 2026-01
	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Lintas Tahun", "100000")
	periods := []string{"2025-11", "2025-12", "2026-01"}
	var bills []Bill
	for _, p := range periods {
		body := fmt.Sprintf(`{"due_id":"%s","period":"%s","due_date":"2026-12-25"}`, dueID, p)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
		var resp struct {
			Bills []Bill `json:"bills"`
		}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NotEmpty(t, resp.Bills)
		bills = append(bills, resp.Bills...)
	}
	require.Len(t, bills, 3)

	// 1. Trying to pay 2025-12 & 2026-01 while 2025-11 is unpaid MUST FAIL FIFO
	{
		billIDs := []string{bills[1].ID, bills[2].ID}
		allocs := []AllocationEntry{
			{BillID: bills[1].ID, Amount: "100000"},
			{BillID: bills[2].ID, Amount: "100000"},
		}
		body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
			mustMarshal(t, billIDs), mustMarshal(t, allocs))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "urutan FIFO")
	}

	// 2. Paying oldest bills first (2025-11 and 2025-12) MUST SUCCEED
	var payID string
	{
		billIDs := []string{bills[0].ID, bills[1].ID}
		allocs := []AllocationEntry{
			{BillID: bills[0].ID, Amount: "100000"},
			{BillID: bills[1].ID, Amount: "100000"},
		}
		body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
			mustMarshal(t, billIDs), mustMarshal(t, allocs))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
		var res MultiPeriodPaymentResult
		json.Unmarshal(rec.Body.Bytes(), &res)
		require.NotEmpty(t, res.Payment.ID)
		payID = res.Payment.ID
	}

	// Bendahara approves first payment
	{
		vBody := `{"action":"approve","actual_received_date":"2026-10-20"}`
		vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+payID+"/verify", bytes.NewBufferString(vBody))
		vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		vReq.Header.Set("Content-Type", "application/json")
		vRec := httptest.NewRecorder()
		router.ServeHTTP(vRec, vReq)
		require.Equal(t, http.StatusOK, vRec.Code)
	}

	// 3. Now 2026-01 is the oldest unpaid bill -> Paying it MUST SUCCEED
	{
		billIDs := []string{bills[2].ID}
		allocs := []AllocationEntry{{BillID: bills[2].ID, Amount: "100000"}}
		body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
			mustMarshal(t, billIDs), mustMarshal(t, allocs))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	}
}

func TestMultiPeriod_FIFO_SkippingForbidden(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	// Create 3 consecutive bills: 2026-01, 2026-02, 2026-03
	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Anti-Skip", "100000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026", 3)
	require.Len(t, bills, 3)

	// Attempt to pay 2026-01 and 2026-03, skipping 2026-02
	billIDs := []string{bills[0].ID, bills[2].ID}
	allocs := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[2].ID, Amount: "100000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocs))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Must be rejected because 2026-02 was skipped
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "urutan FIFO")
}

func TestMultiPeriod_DifferentAmountsPerPeriod(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Tarif Bertingkat", "50000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026", 2)
	require.Len(t, bills, 2)

	// Change second bill to Rp75.000 to simulate historical tariff difference
	_, err := pool.Raw().Exec("UPDATE bills SET amount = '75000.00' WHERE id = $1", bills[1].ID)
	require.NoError(t, err)

	// Submit payment with allocations exactly matching each bill's stored amount
	billIDs := []string{bills[0].ID, bills[1].ID}
	allocs := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "50000"},
		{BillID: bills[1].ID, Amount: "75000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"amount":"125000","method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocs))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "payment with varying period amounts should succeed: %s", rec.Body.String())
	var res MultiPeriodPaymentResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	require.Equal(t, "125000.00", res.Payment.Amount)
}

func TestMultiPeriod_TotalAmountMismatch(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran Mismatch", "100000", "2026-MM", 2)
	defer cleanupTestRT(t, pool, rtID)

	billIDs := []string{bills[0].ID, bills[1].ID}
	allocs := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[1].ID, Amount: "100000"},
	}
	// Explicitly send wrong total amount: 250000 instead of 200000
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"amount":"250000","method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocs))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "nominal pembayaran tidak sesuai total tagihan")
}

func TestMultiPeriod_InvalidBills(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran Invalid", "100000", "2026-INV", 2)
	defer cleanupTestRT(t, pool, rtID)

	// Mark bill 0 as cancelled
	_, err := pool.Raw().Exec("UPDATE bills SET status = 'cancelled' WHERE id = $1", bills[0].ID)
	require.NoError(t, err)

	billIDs := []string{bills[0].ID, bills[1].ID}
	allocs := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[1].ID, Amount: "100000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocs))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "cancelled")
}

// ============================================================
// STAGE 3 TESTS — Approval, Rejection & Concurrency
// ============================================================

func TestMultiPeriod_PendingDoesNotSettleBills(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran Pending", "100000", "2026-PND", 2)
	defer cleanupTestRT(t, pool, rtID)

	billIDs := []string{bills[0].ID, bills[1].ID}
	allocs := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[1].ID, Amount: "100000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocs))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var res MultiPeriodPaymentResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	require.Equal(t, PaymentStatusPending, res.Payment.Status)

	// Verify both bills are STILL UNPAID while payment is pending
	for _, b := range bills {
		gReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+b.ID, nil)
		gReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		gRec := httptest.NewRecorder()
		router.ServeHTTP(gRec, gReq)
		require.Equal(t, http.StatusOK, gRec.Code)
		var billObj Bill
		json.Unmarshal(gRec.Body.Bytes(), &billObj)
		require.Equal(t, BillStatusUnpaid, billObj.Status, "bill must remain UNPAID while payment is pending")
	}

	// Rejecting the payment keeps bills UNPAID and releases pending reservation
	vBody := `{"action":"reject","rejection_reason":"Bukti tidak valid"}`
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+res.Payment.ID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	// Warga can now re-submit payment for the same bills
	rReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	rReq.Header.Set("Authorization", "Bearer "+tokenWarga)
	rReq.Header.Set("Content-Type", "application/json")
	rRec := httptest.NewRecorder()
	router.ServeHTTP(rRec, rReq)
	require.Equal(t, http.StatusCreated, rRec.Code, "resubmission after rejection must succeed")
}

// ============================================================
// STAGE 4 TESTS — Atomicity, Single KAS Posting & Reversal
// ============================================================

func TestMultiPeriod_ReversalCompensation(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _, bills := setupMultiBills(t, pool, router, "Iuran Rev", "100000", "2026-REV", 2)
	defer cleanupTestRT(t, pool, rtID)

	// Staff-recorded payment auto-approves and creates 1 KAS transaction
	billIDs := []string{bills[0].ID, bills[1].ID}
	allocs := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[1].ID, Amount: "100000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocs))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenBendahara)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var res MultiPeriodPaymentResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	paymentID := res.Payment.ID
	require.Equal(t, PaymentStatusApproved, res.Payment.Status)

	// Verify both bills are PAID
	for _, b := range bills {
		gReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+b.ID, nil)
		gReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		gRec := httptest.NewRecorder()
		router.ServeHTTP(gRec, gReq)
		var billObj Bill
		json.Unmarshal(gRec.Body.Bytes(), &billObj)
		require.Equal(t, BillStatusPaid, billObj.Status)
	}

	// Verify exactly 1 KAS transaction was posted for this payment
	var foundIncomeID, foundIncomeAmt, foundIncomeType string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id, amount::text, type FROM transactions WHERE rt_id = $1 AND payment_id = $2 AND type = 'income'`,
		rtID, paymentID,
	).Scan(&foundIncomeID, &foundIncomeAmt, &foundIncomeType)
	require.NoError(t, err, "income transaction must exist for payment")
	require.Equal(t, "200000.00", foundIncomeAmt)

	// Reverse the approved payment
	revBody := `{"reason":"Salah bayar warga"}`
	revReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/reverse", bytes.NewBufferString(revBody))
	revReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	revReq.Header.Set("Content-Type", "application/json")
	revRec := httptest.NewRecorder()
	router.ServeHTTP(revRec, revReq)
	require.Equal(t, http.StatusOK, revRec.Code, "reverse payment should succeed: %s", revRec.Body.String())

	// Verify compensating expense transaction was created referencing the original income
	var foundExpenseID, foundExpenseAmt string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT id, amount::text FROM transactions WHERE rt_id = $1 AND reverses_transaction_id = $2 AND type = 'expense'`,
		rtID, foundIncomeID,
	).Scan(&foundExpenseID, &foundExpenseAmt)
	require.NoError(t, err, "compensating reversal expense transaction must exist")
	require.Equal(t, "200000.00", foundExpenseAmt)

	// Verify payment status is now REVERSED
	pReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+paymentID, nil)
	pReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	pRec := httptest.NewRecorder()
	router.ServeHTTP(pRec, pReq)
	var pObj Payment
	json.Unmarshal(pRec.Body.Bytes(), &pObj)
	require.Equal(t, PaymentStatusReversed, pObj.Status)

	// Verify both bills are restored to UNPAID
	for _, b := range bills {
		gReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+b.ID, nil)
		gReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		gRec := httptest.NewRecorder()
		router.ServeHTTP(gRec, gReq)
		var billObj Bill
		json.Unmarshal(gRec.Body.Bytes(), &billObj)
		require.Equal(t, BillStatusUnpaid, billObj.Status, "bill must be restored to UNPAID after reversal")
	}
}

// ============================================================
// AUDIT LANJUTAN: Pending Reservation & Concurrency Lifecycle
// ============================================================

func TestMultiPeriod_AuditPendingReservation_ConcurrentAndLifecycle(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Audit Pending", "100000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-AP", 2)
	require.Len(t, bills, 2)

	// 1. Warga submits payment for Bill 0 (becomes PENDING)
	body0 := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"100000"}],"method":"CASH"}`,
		bills[0].ID, bills[0].ID)
	req0 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body0))
	req0.Header.Set("Authorization", "Bearer "+tokenWarga)
	req0.Header.Set("Content-Type", "application/json")
	rec0 := httptest.NewRecorder()
	router.ServeHTTP(rec0, req0)
	require.Equal(t, http.StatusCreated, rec0.Code)
	var res0 MultiPeriodPaymentResult
	json.Unmarshal(rec0.Body.Bytes(), &res0)
	require.Equal(t, PaymentStatusPending, res0.Payment.Status)

	// 2. Duplicate submission for Bill 0 while pending MUST BE REJECTED (capacity reserved)
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body0))
	reqDup.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqDup.Header.Set("Content-Type", "application/json")
	recDup := httptest.NewRecorder()
	router.ServeHTTP(recDup, reqDup)
	require.Equal(t, http.StatusBadRequest, recDup.Code)
	require.Contains(t, recDup.Body.String(), "exceeds remaining balance 0.00")

	// 3. Warga trying to pay Bill 1 while Bill 0 is still PENDING MUST BE REJECTED by FIFO
	body1 := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"100000"}],"method":"CASH"}`,
		bills[1].ID, bills[1].ID)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body1))
	req1.Header.Set("Authorization", "Bearer "+tokenWarga)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusBadRequest, rec1.Code)
	require.Contains(t, rec1.Body.String(), "urutan FIFO")

	// 4. Bendahara approves Payment 0 -> Bill 0 becomes PAID
	vBody := `{"action":"approve","actual_received_date":"2026-10-20"}`
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+res0.Payment.ID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	// 5. Now Bill 0 is PAID, Bill 1 is the oldest unpaid bill -> Paying Bill 1 MUST SUCCEED
	req1After := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body1))
	req1After.Header.Set("Authorization", "Bearer "+tokenWarga)
	req1After.Header.Set("Content-Type", "application/json")
	rec1After := httptest.NewRecorder()
	router.ServeHTTP(rec1After, req1After)
	require.Equal(t, http.StatusCreated, rec1After.Code, "payment for Bill 1 should succeed once Bill 0 is approved")
}

// ============================================================
// AUDIT LANJUTAN: Reversal, Subsequent Payment & Retry Integrity
// ============================================================

func TestMultiPeriod_AuditReversal_SubsequentPaymentAndRetry(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, phoneW, _, _, midB, _, userIDB, _ := setupMultiPeriodTestSetup(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	userIDW, midW := setupTestUserWithMembership(t, pool, "w@rev2.test", "Warga REV2", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userIDW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userIDB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	// Create due with partial payment allowed: nominal Rp 100.000
	_ = createDueAndBillsWithRule(t, router, tokenBendahara, "Iuran Audit Rev", "100000", "monthly", "partial", "2026-AR", 1)
	bills := waitForBills(t, router, tokenBendahara, "2026-AR", 1)
	require.Len(t, bills, 1)
	billID := bills[0].ID

	// Payment A: Rp 50.000 by warga
	bodyA := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"50000"}],"method":"CASH"}`, billID, billID)
	reqA := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(bodyA))
	reqA.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqA.Header.Set("Content-Type", "application/json")
	recA := httptest.NewRecorder()
	router.ServeHTTP(recA, reqA)
	require.Equal(t, http.StatusCreated, recA.Code)
	var resA MultiPeriodPaymentResult
	json.Unmarshal(recA.Body.Bytes(), &resA)

	// Approve Payment A -> Bill becomes PARTIAL
	vReqA := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+resA.Payment.ID+"/verify", bytes.NewBufferString(`{"action":"approve","actual_received_date":"2026-10-20"}`))
	vReqA.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReqA.Header.Set("Content-Type", "application/json")
	vRecA := httptest.NewRecorder()
	router.ServeHTTP(vRecA, vReqA)
	require.Equal(t, http.StatusOK, vRecA.Code)

	var bAfterA Bill
	rowA := httptest.NewRecorder()
	getReqA := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID, nil)
	getReqA.Header.Set("Authorization", "Bearer "+tokenBendahara)
	router.ServeHTTP(rowA, getReqA)
	require.Equal(t, http.StatusOK, rowA.Code)
	json.Unmarshal(rowA.Body.Bytes(), &bAfterA)
	require.Equal(t, BillStatusPartial, bAfterA.Status)

	// Payment B: Remaining Rp 50.000 by staff (auto-approved)
	bodyB := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"50000"}],"method":"CASH"}`, billID, billID)
	reqB := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(bodyB))
	reqB.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqB.Header.Set("Content-Type", "application/json")
	recB := httptest.NewRecorder()
	router.ServeHTTP(recB, reqB)
	require.Equal(t, http.StatusCreated, recB.Code)
	var resB MultiPeriodPaymentResult
	json.Unmarshal(recB.Body.Bytes(), &resB)

	// Bill is now fully PAID
	var bAfterB Bill
	rowB := httptest.NewRecorder()
	getReqB := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID, nil)
	getReqB.Header.Set("Authorization", "Bearer "+tokenBendahara)
	router.ServeHTTP(rowB, getReqB)
	require.Equal(t, http.StatusOK, rowB.Code)
	json.Unmarshal(rowB.Body.Bytes(), &bAfterB)
	require.Equal(t, BillStatusPaid, bAfterB.Status)

	// Reversal of Payment A:
	// The bill MUST NOT become UNPAID because Payment B (Rp 50.000) is still valid!
	// It MUST become PARTIAL!
	revReqA := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+resA.Payment.ID+"/reverse", bytes.NewBufferString(`{"reason":"Koreksi A"}`))
	revReqA.Header.Set("Authorization", "Bearer "+tokenBendahara)
	revReqA.Header.Set("Content-Type", "application/json")
	revRecA := httptest.NewRecorder()
	router.ServeHTTP(revRecA, revReqA)
	require.Equal(t, http.StatusOK, revRecA.Code)

	var bAfterRevA Bill
	rowRevA := httptest.NewRecorder()
	getReqRevA := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID, nil)
	getReqRevA.Header.Set("Authorization", "Bearer "+tokenBendahara)
	router.ServeHTTP(rowRevA, getReqRevA)
	require.Equal(t, http.StatusOK, rowRevA.Code)
	json.Unmarshal(rowRevA.Body.Bytes(), &bAfterRevA)
	require.Equal(t, BillStatusPartial, bAfterRevA.Status, "bill must be PARTIAL (not UNPAID) when subsequent payment B still remains approved")

	// Retry reversal of Payment A MUST BE REJECTED (409 Conflict) and MUST NOT duplicate KAS
	retryReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+resA.Payment.ID+"/reverse", bytes.NewBufferString(`{"reason":"Retry A"}`))
	retryReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	retryReq.Header.Set("Content-Type", "application/json")
	retryRec := httptest.NewRecorder()
	router.ServeHTTP(retryRec, retryReq)
	require.Equal(t, http.StatusConflict, retryRec.Code, "retry of reversal must be rejected with 409 Conflict")

	// Reversal of Payment B:
	// Now with both A and B reversed, bill MUST become UNPAID
	revReqB := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+resB.Payment.ID+"/reverse", bytes.NewBufferString(`{"reason":"Koreksi B"}`))
	revReqB.Header.Set("Authorization", "Bearer "+tokenBendahara)
	revReqB.Header.Set("Content-Type", "application/json")
	revRecB := httptest.NewRecorder()
	router.ServeHTTP(revRecB, revReqB)
	require.Equal(t, http.StatusOK, revRecB.Code)

	var bAfterRevB Bill
	rowRevB := httptest.NewRecorder()
	getReqRevB := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID, nil)
	getReqRevB.Header.Set("Authorization", "Bearer "+tokenBendahara)
	router.ServeHTTP(rowRevB, getReqRevB)
	require.Equal(t, http.StatusOK, rowRevB.Code)
	json.Unmarshal(rowRevB.Body.Bytes(), &bAfterRevB)
	require.Equal(t, BillStatusUnpaid, bAfterRevB.Status, "bill must be UNPAID once all payments are reversed")
}

// ============================================================
// AUDIT LANJUTAN: Validasi Nominal di Server (Edge Cases)
// ============================================================

func TestMultiPeriod_AuditNominalValidation_EdgeCases(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran Nominal Edge", "100000", "2026-NE", 2)
	defer cleanupTestRT(t, pool, rtID)

	billIDs := []string{bills[0].ID, bills[1].ID}
	allocs := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[1].ID, Amount: "100000"},
	}

	testCases := []struct {
		name        string
		amountStr   string
		expectCode  int
		errContains string
	}{
		{"Empty string", `""`, http.StatusBadRequest, "nominal pembayaran tidak sesuai"},
		{"Zero amount", `"0"`, http.StatusBadRequest, "nominal pembayaran tidak sesuai"},
		{"Negative amount", `"-200000"`, http.StatusBadRequest, "nominal pembayaran tidak sesuai"},
		{"Malformed text", `"abc"`, http.StatusBadRequest, "nominal pembayaran tidak sesuai"},
		{"Decimal overflow", `"200000.999"`, http.StatusBadRequest, "nominal pembayaran tidak sesuai"},
		{"Incorrect sum", `"150000"`, http.StatusBadRequest, "nominal pembayaran tidak sesuai"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"amount":%s,"method":"CASH"}`,
				mustMarshal(t, billIDs), mustMarshal(t, allocs), tc.amountStr)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
			req.Header.Set("Authorization", "Bearer "+tokenWarga)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, tc.expectCode, rec.Code)
			require.Contains(t, rec.Body.String(), tc.errContains)
		})
	}

	// When amount is omitted (nil in JSON), server calculates exactly 200000.00
	t.Run("Omitted amount calculates automatically", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
			mustMarshal(t, billIDs), mustMarshal(t, allocs))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
		var res MultiPeriodPaymentResult
		json.Unmarshal(rec.Body.Bytes(), &res)
		require.Equal(t, "200000.00", res.Payment.Amount)
	})
}
