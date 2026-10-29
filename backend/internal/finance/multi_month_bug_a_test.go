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
	"social-finance/internal/auth"
)

func TestMultiMonth_BugA_PaymentDoesNotDeactivatePeriodOrOtherResidents(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	ctx := context.Background()

	// Setup Warga A
	phoneA := fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000)
	occAID := setupTestOccupancyWithResident(t, pool, rtID, phoneA)
	userA, midA := setupTestUserWithMembership(t, pool, "warga.a.test@test.com", "Warga A", phoneA, rtID, "warga")
	tokenWargaA := makeToken(t, auth.TokenClaims{UserID: userA, MID: midA, RTID: rtID, Role: string(auth.RoleWarga)})
	_, err := pool.Raw().ExecContext(ctx, "UPDATE residents SET user_id = $1 WHERE rt_id = $2 AND phone = $3", userA, rtID, phoneA)
	require.NoError(t, err)

	// Setup Warga B
	phoneB := fmt.Sprintf("0813%08d", time.Now().UnixNano()%100000000)
	occBID := setupTestOccupancyWithResident(t, pool, rtID, phoneB)
	userB, midB := setupTestUserWithMembership(t, pool, "warga.b.test@test.com", "Warga B", phoneB, rtID, "warga")
	tokenWargaB := makeToken(t, auth.TokenClaims{UserID: userB, MID: midB, RTID: rtID, Role: string(auth.RoleWarga)})
	_, err = pool.Raw().ExecContext(ctx, "UPDATE residents SET user_id = $1 WHERE rt_id = $2 AND phone = $3", userB, rtID, phoneB)
	require.NoError(t, err)

	targetYear := 2026

	// 1. Bendahara creates Multi-Month Due for 2026
	createDueBody := fmt.Sprintf(`{
		"name": "IPL 2026 Multi Test",
		"amount": "50000.00",
		"due_type": "multi_month",
		"period_type": "monthly",
		"payment_rule": "full",
		"year": %d
	}`, targetYear)
	reqCreateDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(createDueBody))
	reqCreateDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqCreateDue.Header.Set("Content-Type", "application/json")
	recCreateDue := httptest.NewRecorder()
	router.ServeHTTP(recCreateDue, reqCreateDue)
	require.Equal(t, http.StatusCreated, recCreateDue.Code)

	var due Due
	require.NoError(t, json.Unmarshal(recCreateDue.Body.Bytes(), &due))

	// Verify all 12 periods initially active in DueGetMonths
	reqMonths := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/dues/%s/months", due.ID), nil)
	reqMonths.Header.Set("Authorization", "Bearer "+tokenBendahara)
	recMonths := httptest.NewRecorder()
	router.ServeHTTP(recMonths, reqMonths)
	require.Equal(t, http.StatusOK, recMonths.Code)

	var monthsRes struct {
		Data []DueMonthSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recMonths.Body.Bytes(), &monthsRes))
	months := monthsRes.Data
	require.Len(t, months, 12)
	for _, m := range months {
		require.True(t, m.IsActive, "period %s should be active", m.Period)
		require.True(t, m.CanDeactivate, "period %s should be deactivatable", m.Period)
		require.Equal(t, 0, m.PaidBills)
		require.Greater(t, m.TotalBills, 0)
	}

	// Find Jan and Feb bills for Warga A and Warga B
	var billAJan, billAFeb *Bill
	var billBJan, billBFeb, billBMar *Bill

	findBills := func(occID string) (*Bill, *Bill, *Bill) {
		rows, err := pool.Raw().QueryContext(ctx,
			`SELECT id, household_occupancy_id, due_id, amount::text, period, due_date, status, is_active
			 FROM bills WHERE due_id = $1 AND household_occupancy_id = $2 ORDER BY period ASC`,
			due.ID, occID)
		require.NoError(t, err)
		defer rows.Close()

		var jan, feb, mar *Bill
		for rows.Next() {
			var b Bill
			var dueDate time.Time
			require.NoError(t, rows.Scan(&b.ID, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.IsActive))
			b.DueDate = dueDate.Format("2006-01-02")
			switch b.Period {
			case fmt.Sprintf("%d-01", targetYear):
				cpy := b
				jan = &cpy
			case fmt.Sprintf("%d-02", targetYear):
				cpy := b
				feb = &cpy
			case fmt.Sprintf("%d-03", targetYear):
				cpy := b
				mar = &cpy
			}
		}
		return jan, feb, mar
	}

	billAJan, billAFeb, _ = findBills(occAID)
	billBJan, billBFeb, billBMar = findBills(occBID)
	require.NotNil(t, billAJan)
	require.NotNil(t, billBJan)
	require.True(t, billAJan.IsActive)
	require.True(t, billBJan.IsActive)

	// 2. Warga A pays Januari & Februari
	payBodyA := fmt.Sprintf(`{
		"bill_ids": ["%s", "%s"],
		"allocations": [
			{"bill_id": "%s", "amount": "50000.00"},
			{"bill_id": "%s", "amount": "50000.00"}
		],
		"method": "CASH"
	}`, billAJan.ID, billAFeb.ID, billAJan.ID, billAFeb.ID)
	reqPayA := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payBodyA))
	reqPayA.Header.Set("Authorization", "Bearer "+tokenWargaA)
	reqPayA.Header.Set("Content-Type", "application/json")
	recPayA := httptest.NewRecorder()
	router.ServeHTTP(recPayA, reqPayA)
	require.Equal(t, http.StatusCreated, recPayA.Code)

	var payResA MultiPeriodPaymentResult
	require.NoError(t, json.Unmarshal(recPayA.Body.Bytes(), &payResA))

	// Bendahara approves Warga A's payment
	todayStr := time.Now().Format("2006-01-02")
	approveBody := fmt.Sprintf(`{"action": "APPROVE", "actual_received_date": "%s"}`, todayStr)
	reqApprove := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/payments/%s/verify", payResA.Payment.ID), bytes.NewBufferString(approveBody))
	reqApprove.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqApprove.Header.Set("Content-Type", "application/json")
	recApprove := httptest.NewRecorder()
	router.ServeHTTP(recApprove, reqApprove)
	require.Equal(t, http.StatusOK, recApprove.Code)

	// 3. VERIFY BUG A FIX:
	// - Warga A's Jan & Feb bills are now status = 'paid'
	// - Warga B's Jan & Feb bills are STILL status = 'unpaid' AND is_active = true!
	var bJanActive, bFebActive bool
	var bJanStatus, bFebStatus string
	err = pool.Raw().QueryRowContext(ctx, "SELECT is_active, status FROM bills WHERE id = $1", billBJan.ID).Scan(&bJanActive, &bJanStatus)
	require.NoError(t, err)
	require.True(t, bJanActive, "Warga B's Jan bill must remain active after Warga A pays")
	require.Equal(t, "unpaid", bJanStatus)

	err = pool.Raw().QueryRowContext(ctx, "SELECT is_active, status FROM bills WHERE id = $1", billBFeb.ID).Scan(&bFebActive, &bFebStatus)
	require.NoError(t, err)
	require.True(t, bFebActive, "Warga B's Feb bill must remain active after Warga A pays")
	require.Equal(t, "unpaid", bFebStatus)

	// - Periods Jan and Feb in DueGetMonths MUST STILL BE ACTIVE!
	reqMonthsAfterPay := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/dues/%s/months", due.ID), nil)
	reqMonthsAfterPay.Header.Set("Authorization", "Bearer "+tokenBendahara)
	recMonthsAfterPay := httptest.NewRecorder()
	router.ServeHTTP(recMonthsAfterPay, reqMonthsAfterPay)
	require.Equal(t, http.StatusOK, recMonthsAfterPay.Code)

	var monthsAfterPayRes struct {
		Data []DueMonthSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recMonthsAfterPay.Body.Bytes(), &monthsAfterPayRes))
	monthsAfterPay := monthsAfterPayRes.Data
	janSummary := monthsAfterPay[0]
	febSummary := monthsAfterPay[1]
	require.True(t, janSummary.IsActive, "Jan period must remain active after Warga A pays")
	require.True(t, janSummary.CanDeactivate, "Jan period must still be deactivatable because Warga B is unpaid")
	require.Equal(t, 1, janSummary.PaidBills)
	require.Greater(t, janSummary.UnpaidBills, 0)

	require.True(t, febSummary.IsActive, "Feb period must remain active after Warga A pays")
	require.True(t, febSummary.CanDeactivate, "Feb period must still be deactivatable because Warga B is unpaid")
	require.Equal(t, 1, febSummary.PaidBills)

	// 4. VERIFY BUG B FIX (Deactivate Jan period with paid bill):
	// Bendahara explicitly deactivates Jan period
	deactBody := `{"is_active": false}`
	janPeriod := fmt.Sprintf("%d-01", targetYear)
	reqDeactJan := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/dues/%s/periods/%s/status", due.ID, janPeriod), bytes.NewBufferString(deactBody))
	reqDeactJan.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqDeactJan.Header.Set("Content-Type", "application/json")
	recDeactJan := httptest.NewRecorder()
	router.ServeHTTP(recDeactJan, reqDeactJan)
	require.Equal(t, http.StatusOK, recDeactJan.Code)

	var monthsAfterDeactRes struct {
		Data []DueMonthSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recDeactJan.Body.Bytes(), &monthsAfterDeactRes))
	janDeactSummary := monthsAfterDeactRes.Data[0]
	require.False(t, janDeactSummary.IsActive, "Jan period must be reported as inactive after bendahara deactivates it")
	require.False(t, janDeactSummary.CanDeactivate, "Jan period cannot be deactivated again while inactive")
	require.Equal(t, 0, janDeactSummary.ActiveBills, "ActiveBills must be 0 when period is deactivated")

	// Warga B's Jan bill is now inactive
	err = pool.Raw().QueryRowContext(ctx, "SELECT is_active FROM bills WHERE id = $1", billBJan.ID).Scan(&bJanActive)
	require.NoError(t, err)
	require.False(t, bJanActive, "Warga B's Jan bill must be inactive after period deactivation")

	// Warga B trying to pay inactive Jan bill directly fails
	payJanB := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "50000.00"}],
		"method": "CASH"
	}`, billBJan.ID, billBJan.ID)
	reqPayJanB := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payJanB))
	reqPayJanB.Header.Set("Authorization", "Bearer "+tokenWargaB)
	reqPayJanB.Header.Set("Content-Type", "application/json")
	recPayJanB := httptest.NewRecorder()
	router.ServeHTTP(recPayJanB, reqPayJanB)
	require.Equal(t, http.StatusBadRequest, recPayJanB.Code, "Inactive bill must be rejected")

	// FIFO for Warga B: Since Jan is inactive, Feb is oldest active and CAN be paid!
	payFebB := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "50000.00"}],
		"method": "CASH"
	}`, billBFeb.ID, billBFeb.ID)
	reqPayFebB := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payFebB))
	reqPayFebB.Header.Set("Authorization", "Bearer "+tokenWargaB)
	reqPayFebB.Header.Set("Content-Type", "application/json")
	recPayFebB := httptest.NewRecorder()
	router.ServeHTTP(recPayFebB, reqPayFebB)
	require.Equal(t, http.StatusCreated, recPayFebB.Code, "Warga B can pay Feb when Jan is inactive")

	// 5. REACTIVATE Jan period:
	reactBody := `{"is_active": true}`
	reqReactJan := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/dues/%s/periods/%s/status", due.ID, janPeriod), bytes.NewBufferString(reactBody))
	reqReactJan.Header.Set("Authorization", "Bearer "+tokenBendahara)
	reqReactJan.Header.Set("Content-Type", "application/json")
	recReactJan := httptest.NewRecorder()
	router.ServeHTTP(recReactJan, reqReactJan)
	require.Equal(t, http.StatusOK, recReactJan.Code)

	var monthsAfterReactRes struct {
		Data []DueMonthSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recReactJan.Body.Bytes(), &monthsAfterReactRes))
	janReactSummary := monthsAfterReactRes.Data[0]
	require.True(t, janReactSummary.IsActive, "Jan period must be active after reactivation")
	require.True(t, janReactSummary.CanDeactivate, "Jan period can be deactivated again")

	// Now Warga B cannot bypass Jan to pay Mar (FIFO violation)
	payMarB := fmt.Sprintf(`{
		"bill_ids": ["%s"],
		"allocations": [{"bill_id": "%s", "amount": "50000.00"}],
		"method": "CASH"
	}`, billBMar.ID, billBMar.ID)
	reqPayMarB := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(payMarB))
	reqPayMarB.Header.Set("Authorization", "Bearer "+tokenWargaB)
	reqPayMarB.Header.Set("Content-Type", "application/json")
	recPayMarB := httptest.NewRecorder()
	router.ServeHTTP(recPayMarB, reqPayMarB)
	require.Equal(t, http.StatusBadRequest, recPayMarB.Code, "Skipping reactivated Jan to pay Mar must be rejected by FIFO")
}
