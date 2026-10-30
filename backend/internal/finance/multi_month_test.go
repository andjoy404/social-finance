package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
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

func TestMultiMonth_ActiveInactiveManagementAndFIFO(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	targetYear := 2029
	bodyMM := fmt.Sprintf(`{"name":"Iuran Sampah 2029","amount":"30000","due_type":"multi_month","year":%d,"due_date":"%d-01-15"}`, targetYear, targetYear)
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &due))

	// 1. Check GET /api/v1/dues/:id/months returns 12 months, all active
	reqMonths := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/dues/%s/months", due.ID), nil)
	reqMonths.Header.Set("Authorization", "Bearer "+tokenBendahara)
	recMonths := httptest.NewRecorder()
	router.ServeHTTP(recMonths, reqMonths)
	require.Equal(t, http.StatusOK, recMonths.Code)

	var monthsResp struct {
		Data []DueMonthSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recMonths.Body.Bytes(), &monthsResp))
	require.Len(t, monthsResp.Data, 12)
	for i, m := range monthsResp.Data {
		expectedPeriod := fmt.Sprintf("%d-%02d", targetYear, i+1)
		require.Equal(t, expectedPeriod, m.Period)
		require.True(t, m.IsActive, "month %s should be active by default", m.Period)
		require.True(t, m.CanDeactivate, "unpaid month %s can be deactivated", m.Period)
	}

	// 2. Fetch bills for Jan and Feb
	reqBills := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/bills?due_id=%s&per_page=20", due.ID), nil)
	reqBills.Header.Set("Authorization", "Bearer "+tokenBendahara)
	recBills := httptest.NewRecorder()
	router.ServeHTTP(recBills, reqBills)
	require.Equal(t, http.StatusOK, recBills.Code)

	var billsResp struct {
		Data []*Bill `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recBills.Body.Bytes(), &billsResp))
	require.GreaterOrEqual(t, len(billsResp.Data), 12)

	var janBill, febBill, marBill *Bill
	for _, b := range billsResp.Data {
		if b.Period == fmt.Sprintf("%d-01", targetYear) {
			janBill = b
		} else if b.Period == fmt.Sprintf("%d-02", targetYear) {
			febBill = b
		} else if b.Period == fmt.Sprintf("%d-03", targetYear) {
			marBill = b
		}
	}
	require.NotNil(t, janBill)
	require.NotNil(t, febBill)
	require.NotNil(t, marBill)
	require.True(t, janBill.IsActive)
	require.True(t, febBill.IsActive)

	// 3. Warga cannot deactivate a period or a bill (403 Forbidden)
	deactBody := `{"is_active": false}`
	reqWargaDeact := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/bills/%s/status", janBill.ID), bytes.NewBufferString(deactBody))
	reqWargaDeact.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqWargaDeact.Header.Set("Content-Type", "application/json")
	recWargaDeact := httptest.NewRecorder()
	router.ServeHTTP(recWargaDeact, reqWargaDeact)
	require.Equal(t, http.StatusForbidden, recWargaDeact.Code)

	// 4. Bendahara deactivates Jan period via PATCH /api/v1/dues/:id/periods/:period/status
	janPeriod := fmt.Sprintf("%d-01", targetYear)
	reqDeactPeriod := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/dues/%s/periods/%s/status", due.ID, janPeriod), bytes.NewBufferString(deactBody))
	reqDeactPeriod.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDeactPeriod.Header.Set("Content-Type", "application/json")
	recDeactPeriod := httptest.NewRecorder()
	router.ServeHTTP(recDeactPeriod, reqDeactPeriod)
	require.Equal(t, http.StatusOK, recDeactPeriod.Code)

	// Verify Jan bill is now inactive in DB
	var janIsActive bool
	err := pool.Raw().QueryRowContext(context.Background(), "SELECT is_active FROM bills WHERE id = $1", janBill.ID).Scan(&janIsActive)
	require.NoError(t, err)
	require.False(t, janIsActive)

	// 5. Attempting to pay inactive Jan bill directly is REJECTED
	payJanBody := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "30000.00"}],
		"method": "CASH"
	}`, janBill.ID, janBill.ID)
	reqPayJan := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payJanBody))
	reqPayJan.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqPayJan.Header.Set("Content-Type", "application/json")
	recPayJan := httptest.NewRecorder()
	router.ServeHTTP(recPayJan, reqPayJan)
	require.Equal(t, http.StatusBadRequest, recPayJan.Code)

	// 6. FIFO skips inactive Jan! Feb is now oldest active unpaid bill and CAN be paid!
	payFebBody := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "30000.00"}],
		"method": "CASH"
	}`, febBill.ID, febBill.ID)
	reqPayFeb := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payFebBody))
	reqPayFeb.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqPayFeb.Header.Set("Content-Type", "application/json")
	recPayFeb := httptest.NewRecorder()
	router.ServeHTTP(recPayFeb, reqPayFeb)
	require.Equal(t, http.StatusCreated, recPayFeb.Code, "Feb bill should be payable when Jan is inactive")

	var febPayRes MultiPeriodPaymentResult
	require.NoError(t, json.Unmarshal(recPayFeb.Body.Bytes(), &febPayRes))
	febPaymentID := febPayRes.Payment.ID

	// Bendahara approves Feb payment
	todayStr := time.Now().Format("2006-01-02")
	verifyFebBody := fmt.Sprintf(`{"action": "APPROVE", "actual_received_date": "%s"}`, todayStr)
	reqApproveFeb := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/payments/%s/verify", febPaymentID), bytes.NewBufferString(verifyFebBody))
	reqApproveFeb.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqApproveFeb.Header.Set("Content-Type", "application/json")
	recApproveFeb := httptest.NewRecorder()
	router.ServeHTTP(recApproveFeb, reqApproveFeb)
	require.Equal(t, http.StatusOK, recApproveFeb.Code)

	// 7. Feb bill is now PAID. Deactivating a paid bill MUST fail!
	reqDeactFeb := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/bills/%s/status", febBill.ID), bytes.NewBufferString(deactBody))
	reqDeactFeb.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDeactFeb.Header.Set("Content-Type", "application/json")
	recDeactFeb := httptest.NewRecorder()
	router.ServeHTTP(recDeactFeb, reqDeactFeb)
	require.Equal(t, http.StatusBadRequest, recDeactFeb.Code, "Paid bill cannot be deactivated: "+recDeactFeb.Body.String())

	// 8. Reactivate Jan period via PATCH /api/v1/bills/:id/status
	reactBody := `{"is_active": true}`
	reqReactJan := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/bills/%s/status", janBill.ID), bytes.NewBufferString(reactBody))
	reqReactJan.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqReactJan.Header.Set("Content-Type", "application/json")
	recReactJan := httptest.NewRecorder()
	router.ServeHTTP(recReactJan, reqReactJan)
	require.Equal(t, http.StatusOK, recReactJan.Code)

	// Now Jan is active again! Paying Mar directly should FAIL FIFO because Jan is older and active!
	payMarBody := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "30000.00"}],
		"method": "CASH"
	}`, marBill.ID, marBill.ID)
	reqPayMar := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payMarBody))
	reqPayMar.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqPayMar.Header.Set("Content-Type", "application/json")
	recPayMar := httptest.NewRecorder()
	router.ServeHTTP(recPayMar, reqPayMar)
	require.Equal(t, http.StatusBadRequest, recPayMar.Code, "Mar should be blocked by reactivated Jan")

	// 9. Pending payment on bill: create pending on Jan, then deactivate Jan, then approval must FAIL cleanly
	payJanPending := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "30000.00"}],
		"method": "TRANSFER"
	}`, janBill.ID, janBill.ID)
	reqPayPending := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payJanPending))
	reqPayPending.Header.Set("Authorization", "Bearer "+tokenWarga)
	reqPayPending.Header.Set("Content-Type", "application/json")
	recPayPending := httptest.NewRecorder()
	router.ServeHTTP(recPayPending, reqPayPending)
	require.Equal(t, http.StatusCreated, recPayPending.Code)

	var janPendingRes MultiPeriodPaymentResult
	require.NoError(t, json.Unmarshal(recPayPending.Body.Bytes(), &janPendingRes))
	janPendingPaymentID := janPendingRes.Payment.ID

	// Now deactivate Jan bill while payment is pending
	reqDeactJanAgain := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/bills/%s/status", janBill.ID), bytes.NewBufferString(deactBody))
	reqDeactJanAgain.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDeactJanAgain.Header.Set("Content-Type", "application/json")
	recDeactJanAgain := httptest.NewRecorder()
	router.ServeHTTP(recDeactJanAgain, reqDeactJanAgain)
	require.Equal(t, http.StatusOK, recDeactJanAgain.Code)

	// Approval of payment on deactivated bill must be REJECTED atomically
	verifyPendingBody := fmt.Sprintf(`{"action": "APPROVE", "actual_received_date": "%s"}`, todayStr)
	reqApprovePending := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/payments/%s/verify", janPendingPaymentID), bytes.NewBufferString(verifyPendingBody))
	reqApprovePending.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqApprovePending.Header.Set("Content-Type", "application/json")
	recApprovePending := httptest.NewRecorder()
	router.ServeHTTP(recApprovePending, reqApprovePending)
	require.Equal(t, http.StatusBadRequest, recApprovePending.Code, "Approval must fail if bill is inactive")

	// Ensure no KAS transaction was created for this rejected approval
	var kasCount int
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM transactions WHERE payment_id = $1", janPendingPaymentID).Scan(&kasCount)
	require.NoError(t, err)
	require.Equal(t, 0, kasCount, "No KAS transaction may be created for rejected approval")
}

func TestMultiMonth_OccupancyMidYear_GeneratesAll12Bills(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "MidYear Occupancy RT", 1, "MYO")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0815%08d", time.Now().UnixNano()%100000000)
	userIDB, midB := setupTestUserWithMembership(t, pool, "b@myo.test", "Bendahara MYO", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userIDB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	// Insert an occupancy with start_date in September 2026
	ctx := context.Background()
	var hhID, houseID, occID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'KK September') RETURNING id`, rtID).Scan(&hhID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'A-01') RETURNING id`, rtID).Scan(&houseID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2026-09-20') RETURNING id`, hhID, houseID).Scan(&occID)
	require.NoError(t, err)

	phoneW := fmt.Sprintf("0845%08d", time.Now().UnixNano()%100000000)
	var resID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, 'Warga Sep', $2, true) RETURNING id`, rtID, phoneW).Scan(&resID)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, '2026-09-20')`, resID, occID)
	require.NoError(t, err)
	userIDW, midW := setupTestUserWithMembership(t, pool, "w@myo.test", "Warga MYO", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userIDW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})

	// Create Multi-Month due for 2026
	bodyMM := `{"name":"IPL 2026 Full","amount":"100000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`
	reqMM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyMM))
	reqMM.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqMM.Header.Set("Content-Type", "application/json")
	recMM := httptest.NewRecorder()
	router.ServeHTTP(recMM, reqMM)
	require.Equal(t, http.StatusCreated, recMM.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recMM.Body.Bytes(), &due))

	// Verify all 12 bills (2026-01 through 2026-12) exist in database
	rows, err := pool.Raw().QueryContext(ctx,
		`SELECT period FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 ORDER BY period ASC`,
		due.ID, occID)
	require.NoError(t, err)
	defer rows.Close()

	var periods []string
	for rows.Next() {
		var p string
		require.NoError(t, rows.Scan(&p))
		periods = append(periods, p)
	}

	require.Len(t, periods, 12, "Occupancy starting in September must still receive all 12 bills for 2026")
	expected := []string{
		"2026-01", "2026-02", "2026-03", "2026-04",
		"2026-05", "2026-06", "2026-07", "2026-08",
		"2026-09", "2026-10", "2026-11", "2026-12",
	}
	require.Equal(t, expected, periods)

	// Verify GET /api/v1/bills?my_bills=true returns bills in chronological order (Jan to Dec)
	reqMyBills := httptest.NewRequest(http.MethodGet, "/api/v1/bills?my_bills=true", nil)
	reqMyBills.Header.Set("Authorization", "Bearer "+tokenWarga)
	recMyBills := httptest.NewRecorder()
	router.ServeHTTP(recMyBills, reqMyBills)
	require.Equal(t, http.StatusOK, recMyBills.Code)

	var myBillsResp struct {
		Data []*Bill `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recMyBills.Body.Bytes(), &myBillsResp))
	require.Len(t, myBillsResp.Data, 12)
	for i, b := range myBillsResp.Data {
		require.Equal(t, expected[i], b.Period, "Warga bills must be ordered chronologically")
	}
}

func TestMultiMonth_CreateDue_Idempotency(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	idemKey := fmt.Sprintf("idem-due-%d", time.Now().UnixNano())
	body := `{"name":"IPL 2026 Unique","amount":"100000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
	req1.Header.Set("Authorization", "Bearer "+tokenBendahara)
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idemKey)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusCreated, rec1.Code)

	var due1 Due
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &due1))

	// Second request with same idempotency key (simulating retry or double-click)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
	req2.Header.Set("Authorization", "Bearer "+tokenBendahara)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idemKey)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusCreated, rec2.Code)

	var due2 Due
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &due2))
	require.Equal(t, due1.ID, due2.ID, "Idempotent request must return the exact same due")

	// Ensure only 1 due was created in the database
	var count int
	err := pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM dues WHERE rt_id = $1 AND name = 'IPL 2026 Unique'", rtID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "Must not create duplicate dues on retry")
}

func TestMultiMonth_SyncBills_NewHouseholdMidYear_Comprehensive(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	ctx := context.Background()

	// 1. Setup KK 1 (Existing Household in RT)
	var hh1ID, house1ID, occ1ID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'KK Lama 1') RETURNING id`, rtID).Scan(&hh1ID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'A-01') RETURNING id`, rtID).Scan(&house1ID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2025-01-01') RETURNING id`, hh1ID, house1ID).Scan(&occ1ID)
	require.NoError(t, err)

	// 2. Create Multi-Month Due 2026
	bodyDue := `{"name":"IPL 2026 Annual","amount":"75000","due_type":"multi_month","year":2026,"due_date":"2026-01-10"}`
	reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
	reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDue.Header.Set("Content-Type", "application/json")
	recDue := httptest.NewRecorder()
	router.ServeHTTP(recDue, reqDue)
	require.Equal(t, http.StatusCreated, recDue.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recDue.Body.Bytes(), &due))

	// Verify KK 1 got 12 bills
	var countBills1 int
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bills WHERE due_id = $1 AND household_occupancy_id = $2`, due.ID, occ1ID).Scan(&countBills1)
	require.NoError(t, err)
	require.Equal(t, 12, countBills1)

	// 3. Bendahara manually deactivates period 2026-05 for KK 1
	var billMayID string
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT id FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 AND period = '2026-05'`, due.ID, occ1ID).Scan(&billMayID)
	require.NoError(t, err)

	patchReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/bills/%s/status", billMayID), bytes.NewBufferString(`{"is_active":false}`))
	patchReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
	patchReq.Header.Set("Content-Type", "application/json")
	recPatch := httptest.NewRecorder()
	router.ServeHTTP(recPatch, patchReq)
	require.Equal(t, http.StatusOK, recPatch.Code)

	// Verify it is inactive
	var isMayActive bool
	err = pool.Raw().QueryRowContext(ctx, `SELECT is_active FROM bills WHERE id = $1`, billMayID).Scan(&isMayActive)
	require.NoError(t, err)
	require.False(t, isMayActive, "Period 2026-05 for KK 1 must be inactive")

	// 4. Now, in October 2026, KK 2 is registered as a new household
	var hh2ID, house2ID, occ2ID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'KK Baru Oktober') RETURNING id`, rtID).Scan(&hh2ID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'B-02') RETURNING id`, rtID).Scan(&house2ID)
	require.NoError(t, err)
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'TENANT', '2026-10-15') RETURNING id`, hh2ID, house2ID).Scan(&occ2ID)
	require.NoError(t, err)

	// Create resident & user for KK 2
	phoneW2 := fmt.Sprintf("0877%08d", time.Now().UnixNano()%100000000)
	var res2ID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, 'Warga Baru Oct', $2, true) RETURNING id`, rtID, phoneW2).Scan(&res2ID)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date) VALUES ($1, $2, '2026-10-15')`, res2ID, occ2ID)
	require.NoError(t, err)
	user2ID, mid2 := setupTestUserWithMembership(t, pool, "w2@oct.test", "Warga Baru Oct", phoneW2, rtID, "warga")
	tokenWarga2 := makeToken(t, auth.TokenClaims{UserID: user2ID, MID: mid2, RTID: rtID, Role: string(auth.RoleWarga)})

	// Verify before sync: KK 2 has 0 bills
	var countBefore int
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bills WHERE due_id = $1 AND household_occupancy_id = $2`, due.ID, occ2ID).Scan(&countBefore)
	require.NoError(t, err)
	require.Equal(t, 0, countBefore, "KK Baru must have 0 bills before sync")

	// 5. Non-privileged user (Warga) tries to sync bills -> Forbidden (403)
	bodySync := fmt.Sprintf(`{"due_id":"%s","period":"2026"}`, due.ID)
	reqSyncWarga := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodySync))
	reqSyncWarga.Header.Set("Authorization", "Bearer "+tokenWarga2)
	reqSyncWarga.Header.Set("Content-Type", "application/json")
	recSyncWarga := httptest.NewRecorder()
	router.ServeHTTP(recSyncWarga, reqSyncWarga)
	require.Equal(t, http.StatusForbidden, recSyncWarga.Code, "Warga cannot trigger bill generation")

	// 6. Bendahara triggers Sync Bills (POST /api/v1/bills/generate)
	reqSync := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodySync))
	reqSync.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqSync.Header.Set("Content-Type", "application/json")
	recSync := httptest.NewRecorder()
	router.ServeHTTP(recSync, reqSync)
	require.Equal(t, http.StatusCreated, recSync.Code)

	var syncResp struct {
		GeneratedCount int     `json:"generated_count"`
		Bills          []*Bill `json:"bills"`
	}
	require.NoError(t, json.Unmarshal(recSync.Body.Bytes(), &syncResp))
	require.Equal(t, 12, syncResp.GeneratedCount, "Exactly 12 new bills should be generated for KK 2")
	require.Len(t, syncResp.Bills, 12)

	// Verify KK 2 now has all 12 bills (2026-01 through 2026-12)
	rows2, err := pool.Raw().QueryContext(ctx,
		`SELECT period, is_active FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 ORDER BY period ASC`,
		due.ID, occ2ID)
	require.NoError(t, err)
	defer rows2.Close()

	var periods2 []string
	for rows2.Next() {
		var p string
		var act bool
		require.NoError(t, rows2.Scan(&p, &act))
		periods2 = append(periods2, p)
		require.True(t, act, "New bills must be active by default")
	}
	expectedPeriods := []string{
		"2026-01", "2026-02", "2026-03", "2026-04",
		"2026-05", "2026-06", "2026-07", "2026-08",
		"2026-09", "2026-10", "2026-11", "2026-12",
	}
	require.Equal(t, expectedPeriods, periods2)

	// Verify KK 1 STILL has exactly 12 bills (no duplicates)
	var countBills1After int
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bills WHERE due_id = $1 AND household_occupancy_id = $2`, due.ID, occ1ID).Scan(&countBills1After)
	require.NoError(t, err)
	require.Equal(t, 12, countBills1After, "KK 1 must not receive duplicate bills")

	// Verify KK 1 May bill is STILL inactive (not overwritten!)
	var isMayActiveAfter bool
	err = pool.Raw().QueryRowContext(ctx, `SELECT is_active FROM bills WHERE id = $1`, billMayID).Scan(&isMayActiveAfter)
	require.NoError(t, err)
	require.False(t, isMayActiveAfter, "KK 1 May bill must remain inactive after sync")

	// 7. Second Sync run -> Idempotent, 0 new bills
	reqSync2 := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodySync))
	reqSync2.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqSync2.Header.Set("Content-Type", "application/json")
	recSync2 := httptest.NewRecorder()
	router.ServeHTTP(recSync2, reqSync2)
	require.Equal(t, http.StatusCreated, recSync2.Code)

	var syncResp2 struct {
		GeneratedCount int `json:"generated_count"`
	}
	require.NoError(t, json.Unmarshal(recSync2.Body.Bytes(), &syncResp2))
	require.Equal(t, 0, syncResp2.GeneratedCount, "Subsequent sync must generate 0 bills")

	// 8. FIFO Enforcement on KK 2:
	// KK 2 tries to pay October 2026 directly without paying Jan–Sep -> MUST BE REJECTED!
	var octBillID string
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT id FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 AND period = '2026-10'`,
		due.ID, occ2ID).Scan(&octBillID)
	require.NoError(t, err)

	bodyPayOct := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "75000.00"}],
		"method": "TRANSFER"
	}`, octBillID, octBillID)
	reqPayOct := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(bodyPayOct))
	reqPayOct.Header.Set("Authorization", "Bearer "+tokenWarga2)
	reqPayOct.Header.Set("Content-Type", "application/json")
	recPayOct := httptest.NewRecorder()
	router.ServeHTTP(recPayOct, reqPayOct)

	require.Equal(t, http.StatusBadRequest, recPayOct.Code, "Paying October directly must be rejected by FIFO")
	require.Contains(t, recPayOct.Body.String(), "FIFO", "Error message must indicate FIFO violation")

	// 9. Valid FIFO payment for KK 2 (Jan to Oct in sequential order):
	var janToOctIDs []string
	var allocParts []string
	rowsJanToOct, err := pool.Raw().QueryContext(ctx,
		`SELECT id FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 AND period <= '2026-10' ORDER BY period ASC`,
		due.ID, occ2ID)
	require.NoError(t, err)
	defer rowsJanToOct.Close()
	for rowsJanToOct.Next() {
		var id string
		require.NoError(t, rowsJanToOct.Scan(&id))
		janToOctIDs = append(janToOctIDs, fmt.Sprintf(`"%s"`, id))
		allocParts = append(allocParts, fmt.Sprintf(`{"bill_id":"%s","amount":"75000.00"}`, id))
	}
	require.Len(t, janToOctIDs, 10)

	bodyPayFIFO := fmt.Sprintf(`{
		"bill_ids": [%s],
		"allocations": [%s],
		"method": "TRANSFER"
	}`, strings.Join(janToOctIDs, ","), strings.Join(allocParts, ","))
	reqPayFIFO := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(bodyPayFIFO))
	reqPayFIFO.Header.Set("Authorization", "Bearer "+tokenWarga2)
	reqPayFIFO.Header.Set("Content-Type", "application/json")
	recPayFIFO := httptest.NewRecorder()
	router.ServeHTTP(recPayFIFO, reqPayFIFO)
	require.Equal(t, http.StatusCreated, recPayFIFO.Code, "Sequential Jan–Oct payment must succeed under FIFO")
}
