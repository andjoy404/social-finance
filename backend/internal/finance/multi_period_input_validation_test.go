package finance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/database"
)

// setupValidationRT creates RT + warga (with occupancy) + bendahara.
// Returns (rtID, tokenBendahara, tokenWarga).
func setupValidationRT(t *testing.T, pool *database.Pool) (string, string, string) {
	t.Helper()
	rtID := setupTestRT(t, pool, "Validation RT", 1, "VAL")

	phoneW := fmt.Sprintf("0848%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneW)
	userIDW, midW := setupTestUserWithMembership(t, pool, "w@val.test", "Warga VAL", phoneW, rtID, "warga")
	tokenWarga := makeToken(t, auth.TokenClaims{UserID: userIDW, MID: midW, RTID: rtID, Role: string(auth.RoleWarga)})
	_ = midW

	phoneB := fmt.Sprintf("0818%08d", time.Now().UnixNano()%100000000)
	userIDB, midB := setupTestUserWithMembership(t, pool, "b@val.test", "Bendahara VAL", phoneB, rtID, "bendahara")
	tokenBendahara := makeToken(t, auth.TokenClaims{UserID: userIDB, MID: midB, RTID: rtID, Role: string(auth.RoleBendahara)})

	return rtID, tokenBendahara, tokenWarga
}

// createDueWithoutBills creates a due via POST /api/v1/dues without generating any initial bill.
func createDueWithoutBills(t *testing.T, router http.Handler, token string, name string, amount string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"%s","amount":"%s","period_type":"monthly"}`, name, amount)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, "create due failed: %s", rec.Body.String())
	var d Due
	err := json.Unmarshal(rec.Body.Bytes(), &d)
	require.NoError(t, err)
	return d.ID
}

// generateBillsForDue generates `count` bills with sequential zero-padded periods (prefix-01..prefix-N).
func generateBillsForDue(t *testing.T, router http.Handler, token string, dueID string, prefix string, count int) []Bill {
	t.Helper()
	var bills []Bill
	for i := 1; i <= count; i++ {
		period := fmt.Sprintf("%s-%02d", prefix, i)
		body := fmt.Sprintf(`{"due_id":"%s","period":"%s","due_date":"2026-12-25"}`, dueID, period)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code, "generate bill period %s failed: %s", period, rec.Body.String())
		var resp struct {
			Bills []Bill `json:"bills"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Bills, "no bills returned for period %s", period)
		bills = append(bills, resp.Bills...)
	}
	return bills
}

// setupMultiBills creates RT + warga + bendahara, creates a due with the given name/amount,
// and generates `count` bills with clean zero-padded periods without any extraneous control bills.
func setupMultiBills(t *testing.T, pool *database.Pool, router http.Handler, dueName string, amount string, period string, count int) (string, string, string, []Bill) {
	t.Helper()
	rtID, tokenBendahara, tokenWarga := setupValidationRT(t, pool)
	dueID := createDueWithoutBills(t, router, tokenBendahara, dueName, amount)
	bills := generateBillsForDue(t, router, tokenBendahara, dueID, period, count)
	return rtID, tokenBendahara, tokenWarga, bills
}

// setupSingleBill creates RT + warga + bendahara, creates a due and a single bill without extra bills.
func setupSingleBill(t *testing.T, pool *database.Pool, router http.Handler, dueName string, amount string, period string) (string, string, string, Bill) {
	t.Helper()
	rtID, tokenBendahara, tokenWarga, bills := setupMultiBills(t, pool, router, dueName, amount, period, 1)
	return rtID, tokenBendahara, tokenWarga, bills[0]
}

// ============================================================
// TEST: Maximum 12 bills per multi-period payment
// ============================================================
func TestMultiPeriod_InputValidation_Max12Bills(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	// 13-bill fixture: generate 13 real bills
	rtID13, _, tokenWarga13, bills13 := setupMultiBills(t, pool, router, "Iuran M13", "100000", "2026-12-M13", 13)
	defer cleanupTestRT(t, pool, rtID13)
	require.Len(t, bills13, 13)

	// 13 real bills — should fail with max-12 input validation error (checked before any DB lock/query).
	billIDs13 := make([]string, 13)
	allocations13 := make([]AllocationEntry, 13)
	for i := 0; i < 13; i++ {
		billIDs13[i] = bills13[i].ID
		allocations13[i] = AllocationEntry{BillID: bills13[i].ID, Amount: "100000"}
	}
	body13 := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs13), mustMarshal(t, allocations13))
	req13 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body13))
	req13.Header.Set("Authorization", "Bearer "+tokenWarga13)
	req13.Header.Set("Content-Type", "application/json")
	rec13 := httptest.NewRecorder()
	router.ServeHTTP(rec13, req13)
	require.Equal(t, http.StatusBadRequest, rec13.Code, "13 bills should be rejected: %s", rec13.Body.String())
	require.Contains(t, rec13.Body.String(), "maximum 12 bills", "13 bills should fail with max-12 error, not FIFO or other business rule")

	// 12-bill fixture in separate RT: should not be rejected by max-12 validation
	rtID12, _, tokenWarga12, bills12 := setupMultiBills(t, pool, router, "Iuran M12", "100000", "2026-12-M12", 12)
	defer cleanupTestRT(t, pool, rtID12)
	require.Len(t, bills12, 12)

	billIDs12 := make([]string, 12)
	allocations12 := make([]AllocationEntry, 12)
	for i := 0; i < 12; i++ {
		billIDs12[i] = bills12[i].ID
		allocations12[i] = AllocationEntry{BillID: bills12[i].ID, Amount: "100000"}
	}
	body12 := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs12), mustMarshal(t, allocations12))
	req12 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body12))
	req12.Header.Set("Authorization", "Bearer "+tokenWarga12)
	req12.Header.Set("Content-Type", "application/json")
	rec12 := httptest.NewRecorder()
	router.ServeHTTP(rec12, req12)
	require.NotEqual(t, http.StatusBadRequest, rec12.Code, "12 bills should not hit max-12 validation: %s", rec12.Body.String())
	require.Equal(t, http.StatusCreated, rec12.Code, "12 valid bills should succeed: %s", rec12.Body.String())
}

// ============================================================
// TEST: Empty bill_ids rejected
// ============================================================
func TestMultiPeriod_InputValidation_EmptyBillIDs(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period",
		bytes.NewBufferString(`{"bill_ids":[],"allocations":[{"bill_id":"dummy","amount":"100000"}],"method":"CASH"}`))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "at least one bill must be selected")
}

// ============================================================
// TEST: Duplicate BillIDs in bill_ids array rejected
// ============================================================
func TestMultiPeriod_InputValidation_DuplicateBillIDs(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bill := setupSingleBill(t, pool, router, "Iuran DB", "100000", "2026-12-MDB")
	defer cleanupTestRT(t, pool, rtID)

	dupID := bill.ID
	billIDs := []string{dupID, dupID}
	allocations := []AllocationEntry{
		{BillID: dupID, Amount: "100000"},
		{BillID: dupID, Amount: "100000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocations))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "duplicate bill_id")
}

// ============================================================
// TEST: Duplicate BillID in allocations array rejected
// ============================================================
func TestMultiPeriod_InputValidation_DuplicateAllocationBillIDs(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran DA", "100000", "2026-12-MDA", 2)
	defer cleanupTestRT(t, pool, rtID)

	require.Len(t, bills, 2)

	billIDs := []string{bills[0].ID, bills[1].ID}
	allocations := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[0].ID, Amount: "50000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocations))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "duplicate allocation for bill_id")
}

// ============================================================
// TEST: Allocation mismatch — allocation bill_id not in bill_ids
// ============================================================
func TestMultiPeriod_InputValidation_AllocationMismatch(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran MA", "100000", "2026-12-MMA", 3)
	defer cleanupTestRT(t, pool, rtID)

	require.Len(t, bills, 3)

	// bill_ids has bills[0],bills[1] but allocations has bills[0],bills[2]
	billIDs := []string{bills[0].ID, bills[1].ID}
	allocations := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[2].ID, Amount: "100000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocations))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "has no matching allocation")

	// Extra allocation bill_id not in bill_ids (count mismatch)
	billIDs2 := []string{bills[0].ID}
	allocations2 := []AllocationEntry{
		{BillID: bills[0].ID, Amount: "100000"},
		{BillID: bills[1].ID, Amount: "100000"},
	}
	body2 := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs2), mustMarshal(t, allocations2))
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body2))
	req2.Header.Set("Authorization", "Bearer "+tokenWarga)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	require.Contains(t, rec2.Body.String(), "allocations count must match bill IDs count")
}

// ============================================================
// TEST: bill_ids count mismatches allocations count
// ============================================================
func TestMultiPeriod_InputValidation_CountMismatch(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran CM", "100000", "2026-12-MCM", 2)
	defer cleanupTestRT(t, pool, rtID)

	require.Len(t, bills, 2)

	billIDs := []string{bills[0].ID, bills[1].ID}
	allocations := []AllocationEntry{{BillID: bills[0].ID, Amount: "100000"}}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocations))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "allocations count must match bill IDs count")
}

// ============================================================
// TEST: Empty allocations array rejected
// ============================================================
func TestMultiPeriod_InputValidation_EmptyAllocations(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period",
		bytes.NewBufferString(`{"bill_ids":["dummy"],"allocations":[],"method":"CASH"}`))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "at least one allocation must be specified")
}

// ============================================================
// TEST: Successful multi-period with exactly 12 bills passes input validation
// ============================================================
func TestMultiPeriod_InputValidation_12BillsSuccess(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga, bills := setupMultiBills(t, pool, router, "Iuran V12", "100000", "2026-12-MV12", 12)
	defer cleanupTestRT(t, pool, rtID)

	require.Len(t, bills, 12)

	billIDs := make([]string, 12)
	allocations := make([]AllocationEntry, 12)
	for i := 0; i < 12; i++ {
		billIDs[i] = bills[i].ID
		allocations[i] = AllocationEntry{BillID: bills[i].ID, Amount: "100000"}
	}

	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocations))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Must succeed with 201 Created and return valid payment result
	require.Equal(t, http.StatusCreated, rec.Code, "12 bills should succeed: %s", rec.Body.String())
	var result MultiPeriodPaymentResult
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	require.NotEmpty(t, result.Payment.ID, "expected payment ID in response")
	require.Len(t, result.Allocations, 12, "expected 12 allocations in response")
	require.Equal(t, PaymentStatusPending, result.Payment.Status, "expected PENDING status for warga payment")
}

// ============================================================
// TEST: Duplicate BillIDs rejected before lock (cheap failure)
// ============================================================
func TestMultiPeriod_InputValidation_DuplicateBeforeLock(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, tokenWarga, bill := setupSingleBill(t, pool, router, "Iuran PL", "100000", "2026-12-MPL")
	defer cleanupTestRT(t, pool, rtID)

	require.Equal(t, BillStatusUnpaid, bill.Status)

	dupID := bill.ID
	billIDs := []string{dupID, dupID}
	allocations := []AllocationEntry{
		{BillID: dupID, Amount: "100000"},
		{BillID: dupID, Amount: "100000"},
	}
	body := fmt.Sprintf(`{"bill_ids":%s,"allocations":%s,"method":"CASH"}`,
		mustMarshal(t, billIDs), mustMarshal(t, allocations))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "duplicate bill_id")

	// Verify bill is still unpaid (no side effects)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+dupID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenBendahara)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var b Bill
	err := json.Unmarshal(rec.Body.Bytes(), &b)
	require.NoError(t, err)
	require.Equal(t, BillStatusUnpaid, b.Status, "bill should still be UNPAID after rejected request")
}

// ============================================================
// TEST: Empty bill_ids + empty allocations
// ============================================================
func TestMultiPeriod_InputValidation_EmptyBillIDsAndAllocations(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, _, tokenWarga := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/multi-period",
		bytes.NewBufferString(`{"bill_ids":[],"allocations":[],"method":"CASH"}`))
	req.Header.Set("Authorization", "Bearer "+tokenWarga)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "at least one bill must be selected")
}
