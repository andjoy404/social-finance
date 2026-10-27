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
)

// ============================================================
// MULTI-MONTH DUES INTEGRATION & REGRESSION TESTS
// ============================================================

func TestMultiMonth_DueTypeStorageAndDistinct(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	// 1. Create Regular due
	bodyReg := `{"name":"Iuran Reguler Test","amount":"25000","period_type":"monthly","due_type":"regular"}`
	reqReg := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyReg))
	reqReg.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	router.ServeHTTP(recReg, reqReg)
	require.Equal(t, http.StatusCreated, recReg.Code)

	var dueReg Due
	require.NoError(t, json.Unmarshal(recReg.Body.Bytes(), &dueReg))
	require.Equal(t, "regular", dueReg.DueType)
	require.False(t, dueReg.IsVoluntary)

	// 2. Create Voluntary due
	bodyVol := `{"name":"Iuran Sukarela Test","amount":"0","period_type":"one_time","due_type":"voluntary"}`
	reqVol := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyVol))
	reqVol.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqVol.Header.Set("Content-Type", "application/json")
	recVol := httptest.NewRecorder()
	router.ServeHTTP(recVol, reqVol)
	require.Equal(t, http.StatusCreated, recVol.Code)

	var dueVol Due
	require.NoError(t, json.Unmarshal(recVol.Body.Bytes(), &dueVol))
	require.Equal(t, "voluntary", dueVol.DueType)
	require.True(t, dueVol.IsVoluntary)

	// 3. Create Multi-Month due
	bodyMM := `{"name":"Iuran Multi-Bulan 2027","amount":"50000","due_type":"multi_month","year":2027,"due_date":"2027-01-10"}`
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var dueMM Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &dueMM))
	require.Equal(t, "multi_month", dueMM.DueType)
	require.False(t, dueMM.IsVoluntary)
	require.NotNil(t, dueMM.Year)
	require.Equal(t, 2027, *dueMM.Year)

	// 4. List dues and check all three types exist and are distinct
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/dues", nil)
	reqList.Header.Set("Authorization", "Bearer "+tokenBendahara)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)

	var list []*Due
	require.NoError(t, json.Unmarshal(recList.Body.Bytes(), &list))
	typeMap := make(map[string]string)
	for _, d := range list {
		typeMap[d.ID] = d.DueType
	}
	require.Equal(t, "regular", typeMap[dueReg.ID])
	require.Equal(t, "voluntary", typeMap[dueVol.ID])
	require.Equal(t, "multi_month", typeMap[dueMM.ID])
}

func TestMultiMonth_12BillsCalendarYear_JanToDec(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	targetYear := 2028
	bodyMM := fmt.Sprintf(`{"name":"Iuran Kebersihan 2028","amount":"60000","due_type":"multi_month","year":%d,"due_date":"%d-01-15"}`, targetYear, targetYear)
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &due))
	require.Equal(t, "multi_month", due.DueType)

	// Query database directly for generated bills
	ctx := context.Background()
	rows, err := pool.Raw().QueryContext(ctx,
		`SELECT period, amount, to_char(due_date, 'YYYY-MM-DD') FROM bills WHERE due_id = $1 ORDER BY period ASC`,
		due.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	var periods []string
	for rows.Next() {
		var p, amt, dd string
		require.NoError(t, rows.Scan(&p, &amt, &dd))
		periods = append(periods, p)
		require.Equal(t, "60000.00", amt)
	}

	// Active occupancies in setupValidationRT is 1
	require.Len(t, periods, 12, "Must generate exactly 12 bills for 1 occupancy")
	expectedPeriods := []string{
		"2028-01", "2028-02", "2028-03", "2028-04",
		"2028-05", "2028-06", "2028-07", "2028-08",
		"2028-09", "2028-10", "2028-11", "2028-12",
	}
	require.Equal(t, expectedPeriods, periods, "Periods must run Jan through Dec of the chosen year")
}

func TestMultiMonth_RetryDoesNotDuplicateBills(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	targetYear := 2029
	bodyMM := fmt.Sprintf(`{"name":"Iuran Keamanan 2029","amount":"75000","due_type":"multi_month","year":%d,"due_date":"%d-01-10"}`, targetYear, targetYear)
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &due))

	// Re-run GenerateBills via API for the same year
	genBody := fmt.Sprintf(`{"due_id":"%s","period":"%d","due_date":"%d-01-10"}`, due.ID, targetYear, targetYear)
	reqGen := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(genBody))
	reqGen.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqGen.Header.Set("Content-Type", "application/json")
	recGen := httptest.NewRecorder()
	router.ServeHTTP(recGen, reqGen)
	require.Equal(t, http.StatusCreated, recGen.Code)

	// Ensure still exactly 12 bills exist, no duplicates
	var count int
	err := pool.Raw().QueryRowContext(context.Background(), `SELECT count(*) FROM bills WHERE due_id = $1`, due.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 12, count, "Retrying bill generation must not duplicate bills")
}

func TestMultiMonth_Payment1Month_2Months_FIFOAndSingleKAS(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	targetYear := 2030
	bodyMM := fmt.Sprintf(`{"name":"Iuran Sampah 2030","amount":"40000","due_type":"multi_month","year":%d,"due_date":"%d-01-10"}`, targetYear, targetYear)
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &due))

	// Fetch bills in order
	ctx := context.Background()
	rows, err := pool.Raw().QueryContext(ctx,
		`SELECT id, period, amount::text FROM bills WHERE due_id = $1 ORDER BY period ASC`,
		due.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	type billItem struct {
		id     string
		period string
		amount string
	}
	var bills []billItem
	for rows.Next() {
		var b billItem
		require.NoError(t, rows.Scan(&b.id, &b.period, &b.amount))
		bills = append(bills, b)
	}
	require.Len(t, bills, 12)

	// 1. Warga pays 2 months (January and February)
	payBody := fmt.Sprintf(`{
		"bill_ids": ["%s", "%s"],
		"allocations": [
			{"bill_id": "%s", "amount": "40000.00"},
			{"bill_id": "%s", "amount": "40000.00"}
		],
		"method": "TRANSFER"
	}`, bills[0].id, bills[1].id, bills[0].id, bills[1].id)

	reqPay := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payBody))
	reqPay.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqPay.Header.Set("Content-Type", "application/json")
	recPay := httptest.NewRecorder()
	router.ServeHTTP(recPay, reqPay)
	require.Equal(t, http.StatusCreated, recPay.Code)

	var payRes MultiPeriodPaymentResult
	require.NoError(t, json.Unmarshal(recPay.Body.Bytes(), &payRes))
	paymentID := payRes.Payment.ID

	// Verify before approval: bills are still UNPAID, KAS is 0
	var kasCountBefore int
	err = pool.Raw().QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE payment_id = $1`, paymentID).Scan(&kasCountBefore)
	require.NoError(t, err)
	require.Equal(t, 0, kasCountBefore, "Pending payment must not generate KAS transactions")

	// 2. Bendahara approves payment with actual_received_date (O-12)
	actualDate := time.Now().Format("2006-01-02")
	approveBody := fmt.Sprintf(`{"action":"approve","actual_received_date":"%s"}`, actualDate)
	reqApprove := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/payments/%s/verify", paymentID), bytes.NewBufferString(approveBody))
	reqApprove.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqApprove.Header.Set("Content-Type", "application/json")
	recApprove := httptest.NewRecorder()
	router.ServeHTTP(recApprove, reqApprove)
	require.Equal(t, http.StatusOK, recApprove.Code)

	// 3. Verify exactly 1 KAS transaction was created
	var kasCountAfter int
	var kasAmount string
	var occurredAt time.Time
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT count(*), max(amount)::text, max(occurred_at) FROM transactions WHERE payment_id = $1`,
		paymentID,
	).Scan(&kasCountAfter, &kasAmount, &occurredAt)
	require.NoError(t, err)
	require.Equal(t, 1, kasCountAfter, "Multi-month payment must generate EXACTLY ONE KAS transaction")
	require.Equal(t, "80000.00", kasAmount, "KAS transaction amount must equal the total sum paid")
	require.Equal(t, actualDate, occurredAt.Format("2006-01-02"), "KAS transaction occurred_at must match actual_received_date")

	// 4. Verify bills status: Jan & Feb are PAID, Mar..Dec are UNPAID
	var statusJan, statusFeb, statusMar string
	err = pool.Raw().QueryRowContext(ctx, `SELECT status FROM bills WHERE id = $1`, bills[0].id).Scan(&statusJan)
	require.NoError(t, err)
	require.Equal(t, "paid", statusJan)

	err = pool.Raw().QueryRowContext(ctx, `SELECT status FROM bills WHERE id = $1`, bills[1].id).Scan(&statusFeb)
	require.NoError(t, err)
	require.Equal(t, "paid", statusFeb)

	err = pool.Raw().QueryRowContext(ctx, `SELECT status FROM bills WHERE id = $1`, bills[2].id).Scan(&statusMar)
	require.NoError(t, err)
	require.Equal(t, "unpaid", statusMar)
}

func TestMultiMonth_FIFOBypassRejected(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	targetYear := 2031
	bodyMM := fmt.Sprintf(`{"name":"Iuran FIFO Test 2031","amount":"30000","due_type":"multi_month","year":%d,"due_date":"%d-01-10"}`, targetYear, targetYear)
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &due))

	ctx := context.Background()
	rows, err := pool.Raw().QueryContext(ctx,
		`SELECT id, period FROM bills WHERE due_id = $1 ORDER BY period ASC`,
		due.ID,
	)
	require.NoError(t, err)
	defer rows.Close()

	var billIDs []string
	for rows.Next() {
		var id, p string
		require.NoError(t, rows.Scan(&id, &p))
		billIDs = append(billIDs, id)
	}
	require.True(t, len(billIDs) >= 3)

	// Attempt to pay February and March while January (billIDs[0]) is unpaid
	bypassBody := fmt.Sprintf(`{
		"bill_ids": ["%s", "%s"],
		"allocations": [
			{"bill_id": "%s", "amount": "30000.00"},
			{"bill_id": "%s", "amount": "30000.00"}
		],
		"method": "TRANSFER"
	}`, billIDs[1], billIDs[2], billIDs[1], billIDs[2])

	reqBypass := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(bypassBody))
	reqBypass.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqBypass.Header.Set("Content-Type", "application/json")
	recBypass := httptest.NewRecorder()
	router.ServeHTTP(recBypass, reqBypass)

	require.Equal(t, http.StatusBadRequest, recBypass.Code, "Skipping oldest unpaid bill must be rejected (FIFO violation)")
}

func TestMultiMonth_PendingBlocksDuplicateAllocation(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	targetYear := 2032
	bodyMM := fmt.Sprintf(`{"name":"Iuran Pending Lock 2032","amount":"50000","due_type":"multi_month","year":%d,"due_date":"%d-01-10"}`, targetYear, targetYear)
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &due))

	var billID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM bills WHERE due_id = $1 ORDER BY period ASC LIMIT 1`,
		due.ID,
	).Scan(&billID)
	require.NoError(t, err)

	// First payment reservation
	payBody1 := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "50000.00"}],
		"method": "TRANSFER"
	}`, billID, billID)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payBody1))
	req1.Header.Set("Authorization", "Bearer "+tokenWarga)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusCreated, rec1.Code)

	// Second payment on same reserved bill
	payBody2 := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "50000.00"}],
		"method": "TRANSFER"
	}`, billID, billID)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payBody2))
	req2.Header.Set("Authorization", "Bearer "+tokenWarga)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusBadRequest, rec2.Code, "Pending payment reservation must block overlapping allocation")
}
