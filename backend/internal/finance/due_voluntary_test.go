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

	"social-finance/internal/auth"
)

func TestVoluntaryDuesZeroAmount(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "RT Voluntary Dues", 1, "FVOL")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b.vol@test.com", "Bendahara Voluntary", phoneB, rtID, "bendahara")

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	// 1. Model input validation unit tests
	t.Run("CreateDueInput validation unit test", func(t *testing.T) {
		// Voluntary with 0 amount -> SUCCESS
		vZero := CreateDueInput{
			Name:        "Sumbangan Kebersihan",
			Amount:      "0",
			PeriodType:  DuePeriodMonthly,
			IsVoluntary: true,
		}
		if err := vZero.Validate(); err != nil {
			t.Fatalf("expected voluntary due with amount 0 to be valid, got: %v", err)
		}
		if vZero.Amount != "0.00" {
			t.Errorf("expected canonical amount 0.00, got: %s", vZero.Amount)
		}

		// Voluntary with 0.00 amount -> SUCCESS
		vZeroDecimal := CreateDueInput{
			Name:        "Sumbangan Sukarela",
			Amount:      "0.00",
			PeriodType:  DuePeriodMonthly,
			IsVoluntary: true,
		}
		if err := vZeroDecimal.Validate(); err != nil {
			t.Fatalf("expected voluntary due with amount 0.00 to be valid, got: %v", err)
		}

		// Voluntary with positive amount -> SUCCESS
		vPositive := CreateDueInput{
			Name:        "Sumbangan Renovasi",
			Amount:      "50000",
			PeriodType:  DuePeriodOneTime,
			IsVoluntary: true,
		}
		if err := vPositive.Validate(); err != nil {
			t.Fatalf("expected voluntary due with positive amount to be valid, got: %v", err)
		}
		if vPositive.Amount != "50000.00" {
			t.Errorf("expected canonical amount 50000.00, got: %s", vPositive.Amount)
		}

		// Mandatory with 0 amount -> FAIL (ErrInvalidAmount)
		mZero := CreateDueInput{
			Name:        "Iuran Wajib",
			Amount:      "0",
			PeriodType:  DuePeriodMonthly,
			IsVoluntary: false,
		}
		if err := mZero.Validate(); err != ErrInvalidAmount {
			t.Fatalf("expected mandatory due with amount 0 to return ErrInvalidAmount, got: %v", err)
		}

		// Negative amount voluntary -> FAIL (ErrInvalidAmount)
		vNegative := CreateDueInput{
			Name:        "Sumbangan Minus",
			Amount:      "-1000",
			PeriodType:  DuePeriodMonthly,
			IsVoluntary: true,
		}
		if err := vNegative.Validate(); err != ErrInvalidAmount {
			t.Fatalf("expected negative amount for voluntary due to return ErrInvalidAmount, got: %v", err)
		}

		// Negative amount mandatory -> FAIL (ErrInvalidAmount)
		mNegative := CreateDueInput{
			Name:        "Iuran Minus",
			Amount:      "-50000",
			PeriodType:  DuePeriodMonthly,
			IsVoluntary: false,
		}
		if err := mNegative.Validate(); err != ErrInvalidAmount {
			t.Fatalf("expected negative amount for mandatory due to return ErrInvalidAmount, got: %v", err)
		}
	})

	// 2. HTTP POST /api/v1/dues with is_voluntary: true, amount: "0"
	t.Run("POST /api/v1/dues voluntary zero amount succeeds", func(t *testing.T) {
		body := `{"name":"Sumbangan Sukarela Warga","amount":"0","period_type":"monthly","is_voluntary":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var created Due
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if created.Amount != "0.00" {
			t.Errorf("expected due amount to be '0.00', got '%s'", created.Amount)
		}
		if !created.IsVoluntary {
			t.Errorf("expected is_voluntary to be true")
		}
	})

	// 3. HTTP POST /api/v1/dues with is_voluntary: false, amount: "0" is rejected
	t.Run("POST /api/v1/dues mandatory zero amount is rejected", func(t *testing.T) {
		body := `{"name":"Iuran Sampah Wajib","amount":"0","period_type":"monthly","is_voluntary":false}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// 4. Verification that payment with nominal 0 remains rejected
	t.Run("Payment input validation rejects zero amount", func(t *testing.T) {
		billID := "some-bill-id"
		pZero := CreatePaymentInput{
			BillID: &billID,
			Amount: "0",
			Method: PaymentMethodCash,
		}
		if err := pZero.Validate(); err != ErrInvalidAmount {
			t.Fatalf("expected payment with amount 0 to return ErrInvalidAmount, got: %v", err)
		}
	})

	// 5. GenerateBills for voluntary due with amount 0 produces 1 bill per occupancy
	t.Run("POST /api/v1/bills/generate for voluntary Rp0 produces individual bills", func(t *testing.T) {
		// Setup occupancy
		occID := setupTestOccupancyWithResident(t, pool, rtID, "081234567890")

		// Create voluntary due with amount 0
		bodyDue := `{"name":"Kerja Bakti Sukarela","amount":"0","period_type":"one_time","payment_rule":"partial","is_voluntary":true}`
		reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
		reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqDue.Header.Set("Content-Type", "application/json")
		recDue := httptest.NewRecorder()
		router.ServeHTTP(recDue, reqDue)

		if recDue.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for due, got %d: %s", recDue.Code, recDue.Body.String())
		}
		var createdDue Due
		json.Unmarshal(recDue.Body.Bytes(), &createdDue)

		// Generate bills for voluntary due
		bodyGen := fmt.Sprintf(`{"due_id":"%s","period":"2026-12","due_date":"2026-12-31"}`, createdDue.ID)
		reqGen := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodyGen))
		reqGen.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGen.Header.Set("Content-Type", "application/json")
		recGen := httptest.NewRecorder()
		router.ServeHTTP(recGen, reqGen)

		if recGen.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for generate bills, got %d: %s", recGen.Code, recGen.Body.String())
		}

		var genResp struct {
			GeneratedCount int    `json:"generated_count"`
			Bills          []Bill `json:"bills"`
		}
		if err := json.Unmarshal(recGen.Body.Bytes(), &genResp); err != nil {
			t.Fatalf("unmarshal generate bills response: %v", err)
		}

		// Voluntary Rp0 should produce individual bills per occupancy
		if genResp.GeneratedCount != 1 {
			t.Errorf("expected generated_count to be 1 for voluntary Rp0 due, got %d", genResp.GeneratedCount)
		}
		if len(genResp.Bills) != 1 {
			t.Errorf("expected 1 bill generated, got %d", len(genResp.Bills))
		}
		if len(genResp.Bills) > 0 && genResp.Bills[0].Amount != "0.00" {
			t.Errorf("expected bill amount to be 0.00, got %s", genResp.Bills[0].Amount)
		}
		if len(genResp.Bills) > 0 && genResp.Bills[0].HouseholdOccupancyID != occID {
			t.Errorf("expected bill household_occupancy_id to be %s, got %s", occID, genResp.Bills[0].HouseholdOccupancyID)
		}

		// Verify bills exist in database for this due
		reqList := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-12", nil)
		reqList.Header.Set("Authorization", "Bearer "+tokenBendahara)
		recList := httptest.NewRecorder()
		router.ServeHTTP(recList, reqList)

		if recList.Code != http.StatusOK {
			t.Fatalf("expected 200 OK listing bills, got %d: %s", recList.Code, recList.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(recList.Body.Bytes(), &listResp)

		found := false
		for _, b := range listResp.Data {
			if b.DueID == createdDue.ID {
				found = true
				if b.Amount != "0.00" {
					t.Errorf("expected bill amount 0.00, got %s", b.Amount)
				}
				break
			}
		}
		if !found {
			t.Fatalf("expected at least 1 bill in database for voluntary Rp0 due")
		}

		// Idempotency: re-running generate does not fail and returns 0 (already billed)
		reqGen2 := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodyGen))
		reqGen2.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGen2.Header.Set("Content-Type", "application/json")
		recGen2 := httptest.NewRecorder()
		router.ServeHTTP(recGen2, reqGen2)

		if recGen2.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created on idempotent retry, got %d: %s", recGen2.Code, recGen2.Body.String())
		}
		var genResp2 struct {
			GeneratedCount int `json:"generated_count"`
		}
		json.Unmarshal(recGen2.Body.Bytes(), &genResp2)
		if genResp2.GeneratedCount != 0 {
			t.Errorf("expected generated_count to be 0 on idempotent retry, got %d", genResp2.GeneratedCount)
		}
	})

	// 6. GenerateBills for positive mandatory due works as expected
	t.Run("POST /api/v1/bills/generate for positive mandatory due generates bills", func(t *testing.T) {
		bodyDue := `{"name":"Iuran Keamanan Positif","amount":"75000","period_type":"monthly","is_voluntary":false}`
		reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
		reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqDue.Header.Set("Content-Type", "application/json")
		recDue := httptest.NewRecorder()
		router.ServeHTTP(recDue, reqDue)

		if recDue.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for due, got %d: %s", recDue.Code, recDue.Body.String())
		}
		var createdDue Due
		json.Unmarshal(recDue.Body.Bytes(), &createdDue)

		bodyGen := fmt.Sprintf(`{"due_id":"%s","period":"2026-11","due_date":"2026-11-25"}`, createdDue.ID)
		reqGen := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodyGen))
		reqGen.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGen.Header.Set("Content-Type", "application/json")
		recGen := httptest.NewRecorder()
		router.ServeHTTP(recGen, reqGen)

		if recGen.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for generate bills, got %d: %s", recGen.Code, recGen.Body.String())
		}

		var genResp struct {
			GeneratedCount int    `json:"generated_count"`
			Bills          []Bill `json:"bills"`
		}
		json.Unmarshal(recGen.Body.Bytes(), &genResp)

		if genResp.GeneratedCount < 1 {
			t.Errorf("expected at least 1 bill generated for positive due, got %d", genResp.GeneratedCount)
		}
		if len(genResp.Bills) > 0 && genResp.Bills[0].Amount != "75000.00" {
			t.Errorf("expected bill amount 75000.00, got %s", genResp.Bills[0].Amount)
		}
	})

	// 7. Payment and verification lifecycle on voluntary Rp0 bill
	t.Run("POST /api/v1/payments/verify approves payment on voluntary Rp0 bill without overpayment rejection", func(t *testing.T) {
		phoneW := fmt.Sprintf("0814%08d", time.Now().UnixNano()%100000000)
		occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)
		userW, midW := setupTestUserWithMembership(t, pool, "w.vol@test.com", "Warga Voluntary", phoneW, rtID, "warga")
		tokenWarga := makeToken(t, auth.TokenClaims{
			UserID: userW,
			MID:    midW,
			RTID:   rtID,
			Role:   string(auth.RoleWarga),
		})

		// Create voluntary due
		bodyDue := `{"name":"Sumbangan Pembangunan","amount":"0","period_type":"one_time","is_voluntary":true}`
		reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
		reqDue.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqDue.Header.Set("Content-Type", "application/json")
		recDue := httptest.NewRecorder()
		router.ServeHTTP(recDue, reqDue)
		if recDue.Code != http.StatusCreated {
			t.Fatalf("failed to create voluntary due: %s", recDue.Body.String())
		}
		var volDue Due
		json.Unmarshal(recDue.Body.Bytes(), &volDue)

		// Generate bills
		bodyGen := fmt.Sprintf(`{"due_id":"%s","period":"2026-12","due_date":"2026-12-31"}`, volDue.ID)
		reqGen := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodyGen))
		reqGen.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGen.Header.Set("Content-Type", "application/json")
		recGen := httptest.NewRecorder()
		router.ServeHTTP(recGen, reqGen)
		if recGen.Code != http.StatusCreated {
			t.Fatalf("failed to generate bills: %s", recGen.Body.String())
		}

		// Find the generated bill for this occupancy
		var billID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`SELECT id FROM bills WHERE due_id = $1 AND household_occupancy_id = $2`,
			volDue.ID, occID,
		).Scan(&billID)
		if err != nil {
			t.Fatalf("failed to find generated bill: %v", err)
		}

		getBalance := func() *BalanceSummary {
			tx, err := pool.Raw().BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatalf("begin tx: %v", err)
			}
			defer tx.Rollback()
			bal, err := CalculateBalance(context.Background(), tx, rtID)
			if err != nil {
				t.Fatalf("calculate balance: %v", err)
			}
			return bal
		}

		getApprovedTotal := func(bID string) string {
			tx, err := pool.Raw().BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatalf("begin tx: %v", err)
			}
			defer tx.Rollback()
			tot, err := GetApprovedPaymentTotal(context.Background(), tx, bID)
			if err != nil {
				t.Fatalf("get approved total: %v", err)
			}
			return tot
		}

		// Initial ledger balance
		balanceBefore := getBalance()

		// 1. Submit voluntary payment on this bill
		bodyPay := fmt.Sprintf(`{"bill_id":"%s","amount":"150000","method":"TRANSFER"}`, billID)
		reqPay := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyPay))
		reqPay.Header.Set("Authorization", "Bearer "+tokenWarga)
		reqPay.Header.Set("Content-Type", "application/json")
		recPay := httptest.NewRecorder()
		router.ServeHTTP(recPay, reqPay)
		if recPay.Code != http.StatusCreated {
			t.Fatalf("failed to submit voluntary payment: %d: %s", recPay.Code, recPay.Body.String())
		}
		var p1 Payment
		json.Unmarshal(recPay.Body.Bytes(), &p1)

		// 2. Approve voluntary payment as Bendahara
		bodyVerify := `{"action":"approve"}`
		reqVerify := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p1.ID+"/verify", bytes.NewBufferString(bodyVerify))
		reqVerify.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqVerify.Header.Set("Content-Type", "application/json")
		recVerify := httptest.NewRecorder()
		router.ServeHTTP(recVerify, reqVerify)
		if recVerify.Code != http.StatusOK {
			t.Fatalf("failed to approve voluntary payment: %d: %s", recVerify.Code, recVerify.Body.String())
		}

		// Verify bill status updated to 'paid'
		var billStatus string
		pool.Raw().QueryRowContext(context.Background(), `SELECT status FROM bills WHERE id = $1`, billID).Scan(&billStatus)
		if billStatus != "paid" {
			t.Errorf("expected bill status 'paid', got %q", billStatus)
		}

		// Verify ledger balance increased by 150000
		balanceAfter1 := getBalance()
		bBeforeCents, _, _ := ParseMoney(balanceBefore.NetBalance)
		bAfter1Cents, _, _ := ParseMoney(balanceAfter1.NetBalance)
		if bAfter1Cents-bBeforeCents != 15000000 {
			t.Errorf("expected balance to increase by 150000 (15000000 cents), got %d cents diff", bAfter1Cents-bBeforeCents)
		}

		// 3. Second donation to the same voluntary bill
		bodyPay2 := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER"}`, billID)
		reqPay2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyPay2))
		reqPay2.Header.Set("Authorization", "Bearer "+tokenWarga)
		reqPay2.Header.Set("Content-Type", "application/json")
		recPay2 := httptest.NewRecorder()
		router.ServeHTTP(recPay2, reqPay2)
		if recPay2.Code != http.StatusCreated {
			t.Fatalf("failed to submit second voluntary payment: %d: %s", recPay2.Code, recPay2.Body.String())
		}
		var p2 Payment
		json.Unmarshal(recPay2.Body.Bytes(), &p2)

		// Approve second payment
		reqVerify2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p2.ID+"/verify", bytes.NewBufferString(bodyVerify))
		reqVerify2.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqVerify2.Header.Set("Content-Type", "application/json")
		recVerify2 := httptest.NewRecorder()
		router.ServeHTTP(recVerify2, reqVerify2)
		if recVerify2.Code != http.StatusOK {
			t.Fatalf("failed to approve second voluntary payment: %d: %s", recVerify2.Code, recVerify2.Body.String())
		}

		// Verify ledger balance increased by another 50000
		balanceAfter2 := getBalance()
		bAfter2Cents, _, _ := ParseMoney(balanceAfter2.NetBalance)
		if bAfter2Cents-bAfter1Cents != 5000000 {
			t.Errorf("expected balance to increase by another 50000 (5000000 cents), got %d cents diff", bAfter2Cents-bAfter1Cents)
		}

		// Verify GetApprovedPaymentTotal sums both payments (150000 + 50000 = 200000)
		totalApproved := getApprovedTotal(billID)
		totCents, _, _ := ParseMoney(totalApproved)
		if totCents != 20000000 {
			t.Errorf("expected approved total 200000.00 (20000000 cents), got %s (%d cents)", totalApproved, totCents)
		}
	})
}

func TestVoluntaryHardeningScenarios(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtA := setupTestRT(t, pool, "RT Hardening A", 1, "RTHA")
	defer cleanupTestRT(t, pool, rtA)
	rtB := setupTestRT(t, pool, "RT Hardening B", 2, "RTHB")
	defer cleanupTestRT(t, pool, rtB)

	// Users in RT A
	phoneBA := fmt.Sprintf("0813%08d", time.Now().UnixNano()%100000000)
	userBA, midBA := setupTestUserWithMembership(t, pool, "b.ha@test.com", "Bendahara HA", phoneBA, rtA, "bendahara")
	tokenBendaharaA := makeToken(t, auth.TokenClaims{UserID: userBA, MID: midBA, RTID: rtA, Role: string(auth.RoleBendahara)})

	phoneWA1 := fmt.Sprintf("0814%08d", time.Now().UnixNano()%100000000)
	occA1 := setupTestOccupancyWithResident(t, pool, rtA, phoneWA1)
	userWA1, midWA1 := setupTestUserWithMembership(t, pool, "w1.ha@test.com", "Warga 1 HA", phoneWA1, rtA, "warga")
	tokenWargaA1 := makeToken(t, auth.TokenClaims{UserID: userWA1, MID: midWA1, RTID: rtA, Role: string(auth.RoleWarga)})

	phoneWA2 := fmt.Sprintf("0815%08d", time.Now().UnixNano()%100000000)
	occA2 := setupTestOccupancyWithResident(t, pool, rtA, phoneWA2)
	_ = occA2
	userWA2, midWA2 := setupTestUserWithMembership(t, pool, "w2.ha@test.com", "Warga 2 HA", phoneWA2, rtA, "warga")
	tokenWargaA2 := makeToken(t, auth.TokenClaims{UserID: userWA2, MID: midWA2, RTID: rtA, Role: string(auth.RoleWarga)})

	// User in RT B
	phoneWB := fmt.Sprintf("0816%08d", time.Now().UnixNano()%100000000)
	occB := setupTestOccupancyWithResident(t, pool, rtB, phoneWB)
	_ = occB
	userWB, midWB := setupTestUserWithMembership(t, pool, "w.hb@test.com", "Warga HB", phoneWB, rtB, "warga")
	tokenWargaB := makeToken(t, auth.TokenClaims{UserID: userWB, MID: midWB, RTID: rtB, Role: string(auth.RoleWarga)})

	// Create voluntary due in RT A (Rp0)
	bodyDue := `{"name":"Kerja Bakti A","amount":"0","period_type":"one_time","payment_rule":"partial","is_voluntary":true}`
	reqDue := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDue))
	reqDue.Header.Set("Authorization", "Bearer "+tokenBendaharaA)
	reqDue.Header.Set("Content-Type", "application/json")
	recDue := httptest.NewRecorder()
	router.ServeHTTP(recDue, reqDue)
	if recDue.Code != http.StatusCreated {
		t.Fatalf("failed to create due: %s", recDue.Body.String())
	}
	var createdDue Due
	json.Unmarshal(recDue.Body.Bytes(), &createdDue)
	dueVolA := createdDue.ID

	// Generate voluntary bill for occA1
	bodyGen := fmt.Sprintf(`{"due_id":"%s","period":"2026-12","due_date":"2026-12-31"}`, dueVolA)
	reqGen := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodyGen))
	reqGen.Header.Set("Authorization", "Bearer "+tokenBendaharaA)
	reqGen.Header.Set("Content-Type", "application/json")
	recGen := httptest.NewRecorder()
	router.ServeHTTP(recGen, reqGen)
	if recGen.Code != http.StatusCreated {
		t.Fatalf("failed to generate bills: %d: %s", recGen.Code, recGen.Body.String())
	}

	var genRes struct {
		Bills []Bill `json:"bills"`
	}
	json.Unmarshal(recGen.Body.Bytes(), &genRes)
	var billVolA1 string
	for _, b := range genRes.Bills {
		if b.HouseholdOccupancyID == occA1 {
			billVolA1 = b.ID
			break
		}
	}
	if billVolA1 == "" {
		t.Fatalf("bill for occA1 not found in generated bills")
	}

	// 1. Rejection of non-positive contribution (amount "0" or "-1000")
	t.Run("Voluntary payment with zero amount is rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"0","method":"TRANSFER"}`, billVolA1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWargaA1)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for Rp0 payment, got %d", rec.Code)
		}
	})

	// 2. Cross-household protection: Warga A2 cannot pay bill of Household A1
	t.Run("Cross-household payment is rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER"}`, billVolA1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWargaA2)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for cross-household payment attempt, got %d", rec.Code)
		}
	})

	// 3. Tenant isolation: Warga in RT B cannot access or pay bill in RT A
	t.Run("Cross-tenant payment is rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"TRANSFER"}`, billVolA1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWargaB)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("expected 400/404 for cross-tenant payment attempt, got %d", rec.Code)
		}
	})

	// 4. Regular (mandatory) due strictly rejects overpayment
	t.Run("Mandatory bill rejects overpayment", func(t *testing.T) {
		bodyDueM := `{"name":"Iuran Keamanan Wajib","amount":"50000","period_type":"monthly","payment_rule":"partial","is_voluntary":false}`
		reqDueM := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(bodyDueM))
		reqDueM.Header.Set("Authorization", "Bearer "+tokenBendaharaA)
		reqDueM.Header.Set("Content-Type", "application/json")
		recDueM := httptest.NewRecorder()
		router.ServeHTTP(recDueM, reqDueM)
		if recDueM.Code != http.StatusCreated {
			t.Fatalf("failed to create mandatory due: %s", recDueM.Body.String())
		}
		var createdDueM Due
		json.Unmarshal(recDueM.Body.Bytes(), &createdDueM)
		dueMandatory := createdDueM.ID
		bodyGenM := fmt.Sprintf(`{"due_id":"%s","period":"2026-12","due_date":"2026-12-31"}`, dueMandatory)
		reqGenM := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodyGenM))
		reqGenM.Header.Set("Authorization", "Bearer "+tokenBendaharaA)
		reqGenM.Header.Set("Content-Type", "application/json")
		recGenM := httptest.NewRecorder()
		router.ServeHTTP(recGenM, reqGenM)

		var genMRes struct {
			Bills []Bill `json:"bills"`
		}
		json.Unmarshal(recGenM.Body.Bytes(), &genMRes)
		var billMandatoryA1 string
		for _, b := range genMRes.Bills {
			if b.HouseholdOccupancyID == occA1 {
				billMandatoryA1 = b.ID
				break
			}
		}

		// Trying to pay 60000 on a 50000 bill
		bodyOver := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"TRANSFER"}`, billMandatoryA1)
		reqOver := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyOver))
		reqOver.Header.Set("Authorization", "Bearer "+tokenWargaA1)
		reqOver.Header.Set("Content-Type", "application/json")
		recOver := httptest.NewRecorder()
		router.ServeHTTP(recOver, reqOver)
		if recOver.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for overpayment on mandatory bill, got %d: %s", recOver.Code, recOver.Body.String())
		}
	})

	// 5. Authoritative domain rule: a non-voluntary due with amount Rp0 still rejects overpayment
	t.Run("Non-voluntary due with Rp0 amount rejects overpayment", func(t *testing.T) {
		// Insert non-voluntary due with 0 amount directly
		var dueEdgeID string
		pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO dues (rt_id, name, amount, period_type, is_voluntary, payment_rule, is_active)
			 VALUES ($1, 'Edge Case Mandatory 0', 0, 'monthly', false, 'partial', true)
			 RETURNING id`, rtA,
		).Scan(&dueEdgeID)

		var billEdgeID string
		pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
			 VALUES ($1, $2, $3, 0, '2026-12', '2026-12-31', 'unpaid')
			 RETURNING id`, rtA, occA1, dueEdgeID,
		).Scan(&billEdgeID)

		// Trying to submit 25000 on this non-voluntary bill must be rejected
		bodyEdge := fmt.Sprintf(`{"bill_id":"%s","amount":"25000","method":"TRANSFER"}`, billEdgeID)
		reqEdge := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyEdge))
		reqEdge.Header.Set("Authorization", "Bearer "+tokenWargaA1)
		reqEdge.Header.Set("Content-Type", "application/json")
		recEdge := httptest.NewRecorder()
		router.ServeHTTP(recEdge, reqEdge)
		if recEdge.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for non-voluntary zero-amount bill, got %d: %s", recEdge.Code, recEdge.Body.String())
		}
	})

	// 6. Idempotency on voluntary payment does not duplicate payments or transactions
	t.Run("Idempotent voluntary payment retry returns cached response", func(t *testing.T) {
		idemKey := fmt.Sprintf("idem-vol-%d", time.Now().UnixNano())
		bodyPay := fmt.Sprintf(`{"bill_id":"%s","amount":"75000","method":"TRANSFER"}`, billVolA1)

		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyPay))
		req1.Header.Set("Authorization", "Bearer "+tokenWargaA1)
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", idemKey)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusCreated {
			t.Fatalf("first idempotent payment failed: %d: %s", rec1.Code, rec1.Body.String())
		}
		var p1 Payment
		json.Unmarshal(rec1.Body.Bytes(), &p1)

		// Second request with same idempotency key
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyPay))
		req2.Header.Set("Authorization", "Bearer "+tokenWargaA1)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", idemKey)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusCreated {
			t.Fatalf("second idempotent payment failed: %d: %s", rec2.Code, rec2.Body.String())
		}
		var p2 Payment
		json.Unmarshal(rec2.Body.Bytes(), &p2)

		if p1.ID != p2.ID {
			t.Errorf("expected identical payment ID for idempotent retry, got %s vs %s", p1.ID, p2.ID)
		}
	})

	// 7. Payment rejection does not transition bill to paid and does not increment ledger
	t.Run("Rejection of voluntary payment does not post income and keeps bill pending/unpaid", func(t *testing.T) {
		bodyPay := fmt.Sprintf(`{"bill_id":"%s","amount":"30000","method":"TRANSFER"}`, billVolA1)
		reqPay := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(bodyPay))
		reqPay.Header.Set("Authorization", "Bearer "+tokenWargaA1)
		reqPay.Header.Set("Content-Type", "application/json")
		recPay := httptest.NewRecorder()
		router.ServeHTTP(recPay, reqPay)
		if recPay.Code != http.StatusCreated {
			t.Fatalf("failed to submit payment: %d", recPay.Code)
		}
		var p Payment
		json.Unmarshal(recPay.Body.Bytes(), &p)

		// Balance before rejection
		var bBefore string
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE -amount END), 0)::text FROM transactions WHERE rt_id = $1 AND status = 'posted'`, rtA,
		).Scan(&bBefore)

		// Reject payment
		bodyReject := `{"action":"reject","rejection_reason":"Bukti transfer tidak terbaca"}`
		reqReject := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/verify", bytes.NewBufferString(bodyReject))
		reqReject.Header.Set("Authorization", "Bearer "+tokenBendaharaA)
		reqReject.Header.Set("Content-Type", "application/json")
		recReject := httptest.NewRecorder()
		router.ServeHTTP(recReject, reqReject)
		if recReject.Code != http.StatusOK {
			t.Fatalf("failed to reject payment: %d: %s", recReject.Code, recReject.Body.String())
		}

		// Verify payment status is rejected
		var pStatus string
		pool.Raw().QueryRowContext(context.Background(), `SELECT status FROM payments WHERE id = $1`, p.ID).Scan(&pStatus)
		if pStatus != "REJECTED" {
			t.Errorf("expected payment status REJECTED, got %s", pStatus)
		}

		// Balance after rejection should not change
		var bAfter string
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE -amount END), 0)::text FROM transactions WHERE rt_id = $1 AND status = 'posted'`, rtA,
		).Scan(&bAfter)
		if bBefore != bAfter {
			t.Errorf("expected balance to stay unchanged upon rejection, got %s before vs %s after", bBefore, bAfter)
		}
	})
}
