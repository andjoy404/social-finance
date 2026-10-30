package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/storage"
)

// ============================================================
// O-12 REGRESSION & VERIFICATION TESTS — ACTUAL RECEIVED DATE
// ============================================================

// 1. Pembayaran diterima tanggal masa lalu (misal 31 Okt) dan disetujui hari ini
//    menghasilkan transaksi KAS bertanggal 31 Okt, bukan tanggal verifikasi.
func TestO12_ActualReceivedDate_PassedToKASTransaction(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Kebersihan O-12", "50000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-O12", 2)
	require.Len(t, bills, 2)

	// Warga submits payment on 2 bills
	subBody := fmt.Sprintf(`{"bill_ids":["%s","%s"],"allocations":[{"bill_id":"%s","amount":"50000"},{"bill_id":"%s","amount":"50000"}],"method":"TRANSFER"}`,
		bills[0].ID, bills[1].ID, bills[0].ID, bills[1].ID)
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(subBody))
	subReq.Header.Set("Authorization", "Bearer "+tokenWarga)
	subReq.Header.Set("Content-Type", "application/json")
	subRec := httptest.NewRecorder()
	router.ServeHTTP(subRec, subReq)
	require.Equal(t, http.StatusCreated, subRec.Code)

	var subRes MultiPeriodPaymentResult
	err := json.Unmarshal(subRec.Body.Bytes(), &subRes)
	require.NoError(t, err)
	paymentID := subRes.Payment.ID

	// Date received: 2026-10-20
	actualDate := "2026-10-20"
	vBody := fmt.Sprintf(`{"action":"approve","actual_received_date":"%s","officer_notes":"Lolos rekonsiliasi bank"}`, actualDate)
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	var vPayment Payment
	err = json.Unmarshal(vRec.Body.Bytes(), &vPayment)
	require.NoError(t, err)
	require.Equal(t, PaymentStatusApproved, vPayment.Status)
	require.NotNil(t, vPayment.ActualReceivedDate)
	require.Equal(t, actualDate, *vPayment.ActualReceivedDate)
	require.NotNil(t, vPayment.VerifiedAt)

	// Verify payment row from DB directly
	var dbActualDate *string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT actual_received_date::text FROM payments WHERE id = $1`, paymentID,
	).Scan(&dbActualDate)
	require.NoError(t, err)
	require.NotNil(t, dbActualDate)
	require.Equal(t, actualDate, *dbActualDate)

	// Verify KAS transaction generated has occurred_at mapped to 2026-10-20 in Asia/Jakarta
	loc, _ := time.LoadLocation("Asia/Jakarta")
	var transOccurredAt time.Time
	var transAmount string
	var transType string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT occurred_at, amount::text, type FROM transactions WHERE payment_id = $1`, paymentID,
	).Scan(&transOccurredAt, &transAmount, &transType)
	require.NoError(t, err)
	require.Equal(t, "100000.00", transAmount)
	require.Equal(t, "income", transType)

	// Verify formatted calendar date matches exactly in Asia/Jakarta
	occurredJakarta := transOccurredAt.In(loc).Format("2006-01-02")
	require.Equal(t, actualDate, occurredJakarta, "KAS transaction occurred_at date must match actual_received_date")
}

// 2. Validasi Tanggal: Empty, Malformed, Impossible Calendar Date, dan Future Date
func TestO12_ActualReceivedDate_ValidationRules(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Validasi O-12", "50000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-VAL", 1)
	require.Len(t, bills, 1)

	// Warga submits payment
	subBody := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"50000"}],"method":"TRANSFER"}`,
		bills[0].ID, bills[0].ID)
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(subBody))
	subReq.Header.Set("Authorization", "Bearer "+tokenWarga)
	subReq.Header.Set("Content-Type", "application/json")
	subRec := httptest.NewRecorder()
	router.ServeHTTP(subRec, subReq)
	require.Equal(t, http.StatusCreated, subRec.Code)

	var subRes MultiPeriodPaymentResult
	json.Unmarshal(subRec.Body.Bytes(), &subRes)
	paymentID := subRes.Payment.ID

	testCases := []struct {
		name       string
		actualDate string
		expSubstr  string
	}{
		{"Empty string", "", "actual_received_date) wajib diisi"},
		{"Malformed text", "invalid-date", "format actual_received_date harus YYYY-MM-DD"},
		{"Slash delimiter", "2026/10/20", "format actual_received_date harus YYYY-MM-DD"},
		{"Impossible February 30", "2026-02-30", "tidak valid"},
		{"Impossible Month 13", "2026-13-01", "tidak valid"},
		{"Future date", "2099-01-01", "tidak boleh berada di masa depan"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vBody := fmt.Sprintf(`{"action":"approve","actual_received_date":"%s"}`, tc.actualDate)
			vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(vBody))
			vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
			vReq.Header.Set("Content-Type", "application/json")
			vRec := httptest.NewRecorder()
			router.ServeHTTP(vRec, vReq)
			require.Equal(t, http.StatusBadRequest, vRec.Code)
			require.Contains(t, vRec.Body.String(), tc.expSubstr)
		})
	}
}

// 3. Pembayaran satu bulan (legacy / single endpoint) menggunakan actual_received_date
func TestO12_SinglePeriodPayment_ActualReceivedDate(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Single O-12", "75000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-SN", 1)
	require.Len(t, bills, 1)

	// Warga submits via single payment endpoint POST /api/v1/payments
	subBody := fmt.Sprintf(`{"bill_id":"%s","amount":"75000","method":"TRANSFER"}`, bills[0].ID)
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(subBody))
	subReq.Header.Set("Authorization", "Bearer "+tokenWarga)
	subReq.Header.Set("Content-Type", "application/json")
	subRec := httptest.NewRecorder()
	router.ServeHTTP(subRec, subReq)
	require.Equal(t, http.StatusCreated, subRec.Code)

	var p Payment
	json.Unmarshal(subRec.Body.Bytes(), &p)

	// Approve with actual_received_date
	actualDate := "2026-10-15"
	vBody := fmt.Sprintf(`{"action":"approve","actual_received_date":"%s"}`, actualDate)
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	// Check KAS transaction occurred_at
	loc, _ := time.LoadLocation("Asia/Jakarta")
	var transOccurredAt time.Time
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT occurred_at FROM transactions WHERE payment_id = $1`, p.ID,
	).Scan(&transOccurredAt)
	require.NoError(t, err)
	require.Equal(t, actualDate, transOccurredAt.In(loc).Format("2006-01-02"))
}

// 4. Pembayaran Sukarela (Donation) menggunakan actual_received_date pada KAS
func TestO12_VoluntaryDonation_ActualReceivedDate(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	// Create voluntary due
	dueBody := `{"name":"Sumbangan O-12","amount":"0","period_type":"monthly","payment_rule":"full","is_voluntary":true}`
	dueReq := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(dueBody))
	dueReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	dueReq.Header.Set("Content-Type", "application/json")
	dueRec := httptest.NewRecorder()
	router.ServeHTTP(dueRec, dueReq)
	require.Equal(t, http.StatusCreated, dueRec.Code)

	var due Due
	json.Unmarshal(dueRec.Body.Bytes(), &due)

	// Warga donates
	donBody := fmt.Sprintf(`{"due_id":"%s","amount":"250000","method":"TRANSFER"}`, due.ID)
	donReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(donBody))
	donReq.Header.Set("Authorization", "Bearer "+tokenWarga)
	donReq.Header.Set("Content-Type", "application/json")
	donRec := httptest.NewRecorder()
	router.ServeHTTP(donRec, donReq)
	require.Equal(t, http.StatusCreated, donRec.Code)

	var p Payment
	json.Unmarshal(donRec.Body.Bytes(), &p)

	// Approve voluntary donation with actual received date
	actualDate := "2026-10-18"
	vBody := fmt.Sprintf(`{"action":"approve","actual_received_date":"%s"}`, actualDate)
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	loc, _ := time.LoadLocation("Asia/Jakarta")
	var transOccurredAt time.Time
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT occurred_at FROM transactions WHERE payment_id = $1`, p.ID,
	).Scan(&transOccurredAt)
	require.NoError(t, err)
	require.Equal(t, actualDate, transOccurredAt.In(loc).Format("2006-01-02"))
}

// 5. Staff Recording: Petugas dapat menyertakan actual_received_date secara langsung
func TestO12_StaffRecorded_ActualReceivedDate(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Tunai Staf", "50000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-ST", 1)
	require.Len(t, bills, 1)

	// Staff records payment on the spot with explicit actual_received_date = 2026-10-10
	staffDate := "2026-10-10"
	body := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"50000"}],"method":"CASH","actual_received_date":"%s"}`,
		bills[0].ID, bills[0].ID, staffDate)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenBendahara)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var res MultiPeriodPaymentResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	require.NotNil(t, res.Payment.ActualReceivedDate)
	require.Equal(t, staffDate, *res.Payment.ActualReceivedDate)

	loc, _ := time.LoadLocation("Asia/Jakarta")
	var transOccurredAt time.Time
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT occurred_at FROM transactions WHERE payment_id = $1`, res.Payment.ID,
	).Scan(&transOccurredAt)
	require.NoError(t, err)
	require.Equal(t, staffDate, transOccurredAt.In(loc).Format("2006-01-02"))
}

// 6 & 7. Pembayaran pending dan reject tidak membuat transaksi KAS
func TestO12_PendingAndReject_NoKASTransaction(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Pending/Reject", "50000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-PR", 1)
	require.Len(t, bills, 1)

	// Step 1: Submit payment -> status PENDING
	body := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"50000"}],"method":"TRANSFER"}`,
		bills[0].ID, bills[0].ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var res MultiPeriodPaymentResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	paymentID := res.Payment.ID

	// Verify bill is still UNPAID
	var billStatus string
	err := pool.Raw().QueryRowContext(context.Background(), `SELECT status FROM bills WHERE id = $1`, bills[0].ID).Scan(&billStatus)
	require.NoError(t, err)
	require.Equal(t, "unpaid", billStatus)

	// Verify ZERO KAS transactions created for pending payment
	var txCount int
	err = pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM transactions WHERE payment_id = $1`, paymentID).Scan(&txCount)
	require.NoError(t, err)
	require.Equal(t, 0, txCount, "Pending payment must not post to KAS")

	// Step 2: Reject payment
	rejBody := `{"action":"reject","rejection_reason":"Bukti transfer buram","officer_notes":"Mohon upload ulang"}`
	rejReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(rejBody))
	rejReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	rejReq.Header.Set("Content-Type", "application/json")
	rejRec := httptest.NewRecorder()
	router.ServeHTTP(rejRec, rejReq)
	require.Equal(t, http.StatusOK, rejRec.Code)

	// Verify ZERO KAS transactions created after reject
	err = pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM transactions WHERE payment_id = $1`, paymentID).Scan(&txCount)
	require.NoError(t, err)
	require.Equal(t, 0, txCount, "Rejected payment must not post to KAS")

	// Verify bill remains UNPAID
	err = pool.Raw().QueryRowContext(context.Background(), `SELECT status FROM bills WHERE id = $1`, bills[0].ID).Scan(&billStatus)
	require.NoError(t, err)
	require.Equal(t, "unpaid", billStatus)
}

// 8. Retry / Concurrent Approval tidak membuat posting ganda
func TestO12_RetryApproval_Blocked_NoDuplicateKAS(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Retry", "50000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-RT", 1)
	require.Len(t, bills, 1)

	body := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"50000"}],"method":"TRANSFER"}`,
		bills[0].ID, bills[0].ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var res MultiPeriodPaymentResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	paymentID := res.Payment.ID

	// First approval: succeeds
	vBody := `{"action":"approve","actual_received_date":"2026-10-20"}`
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	// Verify exactly 1 KAS transaction exists
	var txCount int
	err := pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM transactions WHERE payment_id = $1`, paymentID).Scan(&txCount)
	require.NoError(t, err)
	require.Equal(t, 1, txCount)

	// Second approval attempt (retry): must FAIL
	vReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(vBody))
	vReq2.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq2.Header.Set("Content-Type", "application/json")
	vRec2 := httptest.NewRecorder()
	router.ServeHTTP(vRec2, vReq2)
	require.Equal(t, http.StatusBadRequest, vRec2.Code)
	require.Contains(t, vRec2.Body.String(), "only pending payments can be verified")

	// Still exactly 1 KAS transaction
	err = pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM transactions WHERE payment_id = $1`, paymentID).Scan(&txCount)
	require.NoError(t, err)
	require.Equal(t, 1, txCount, "Retry approval must not create duplicate KAS transaction")
}

// 9. Pembayaran historis dengan actual_received_date = NULL tetap dapat dibaca dengan aman
func TestO12_HistoricalPayment_NullActualReceivedDate(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran Historis", "50000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-HIST", 1)
	require.Len(t, bills, 1)

	// Insert historical payment directly into DB with actual_received_date NULL
	var histPaymentID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, paid_at, actual_received_date)
		 VALUES ($1, $2, 50000.00, 'TRANSFER', 'SELF_SUBMITTED', 'PENDING', '2026-09-15 10:00:00+07', NULL)
		 RETURNING id`,
		rtID, bills[0].ID,
	).Scan(&histPaymentID)
	require.NoError(t, err)

	// Fetch via API GET /api/v1/payments/:id
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+histPaymentID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenBendahara)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var p Payment
	err = json.Unmarshal(rec.Body.Bytes(), &p)
	require.NoError(t, err)
	require.Nil(t, p.ActualReceivedDate, "Historical payment with NULL date must remain nil in JSON")

	// Historical pending payment can be approved by supplying actual_received_date
	vBody := `{"action":"approve","actual_received_date":"2026-09-15"}`
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+histPaymentID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	var approvedP Payment
	json.Unmarshal(vRec.Body.Bytes(), &approvedP)
	require.NotNil(t, approvedP.ActualReceivedDate)
	require.Equal(t, "2026-09-15", *approvedP.ActualReceivedDate)
}

// 1 & 10. Pembayaran Oktober disetujui November masuk laporan KAS Oktober, bukan November
func TestO12_MonthlyKAS_PlacesTransactionInActualReceivedMonth(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	dueID := createDueWithoutBills(t, router, tokenBendahara, "Iuran KAS Cross-Month", "150000")
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, "2026-10", 1)
	require.Len(t, bills, 1)

	// Warga submits payment
	subBody := fmt.Sprintf(`{"bill_ids":["%s"],"allocations":[{"bill_id":"%s","amount":"150000"}],"method":"TRANSFER"}`,
		bills[0].ID, bills[0].ID)
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(subBody))
	subReq.Header.Set("Authorization", "Bearer "+tokenWarga)
	subReq.Header.Set("Content-Type", "application/json")
	subRec := httptest.NewRecorder()
	router.ServeHTTP(subRec, subReq)
	require.Equal(t, http.StatusCreated, subRec.Code)

	var subRes MultiPeriodPaymentResult
	json.Unmarshal(subRec.Body.Bytes(), &subRes)
	paymentID := subRes.Payment.ID

	// Actual date received is 30 September 2026
	actualDate := "2026-09-30"
	vBody := fmt.Sprintf(`{"action":"approve","actual_received_date":"%s"}`, actualDate)
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(vBody))
	vReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	vReq.Header.Set("Content-Type", "application/json")
	vRec := httptest.NewRecorder()
	router.ServeHTTP(vRec, vReq)
	require.Equal(t, http.StatusOK, vRec.Code)

	// Query dashboard summary for September 2026 via service
	store := storage.NewMockStorage()
	svc := NewService(pool, store)
	tx, err := pool.BeginTx(context.Background())
	require.NoError(t, err)
	defer tx.Rollback()

	summarySep, err := svc.GetDashboardSummary(context.Background(), tx, rtID, "2026-09")
	require.NoError(t, err)
	require.Equal(t, "150000.00", summarySep.MonthlyIncome, "September KAS report must include transaction based on actual_received_date 2026-09-30")

	// Query dashboard summary for October 2026: must be 0
	summaryOct, err := svc.GetDashboardSummary(context.Background(), tx, rtID, "2026-10")
	require.NoError(t, err)
	require.Equal(t, "0.00", summaryOct.MonthlyIncome, "October KAS report must NOT include transaction received in September")
}
