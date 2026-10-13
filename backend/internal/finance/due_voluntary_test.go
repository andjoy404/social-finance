package finance

import (
	"bytes"
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
		pZero := CreatePaymentInput{
			BillID: "some-bill-id",
			Amount: "0",
			Method: PaymentMethodCash,
		}
		if err := pZero.Validate(); err != ErrInvalidAmount {
			t.Fatalf("expected payment with amount 0 to return ErrInvalidAmount, got: %v", err)
		}
	})

	// 5. GenerateBills for voluntary due with amount 0 succeeds with 0 bills
	t.Run("POST /api/v1/bills/generate for voluntary Rp0 produces 0 bills and does not fail", func(t *testing.T) {
		// Setup occupancy
		_ = setupTestOccupancyWithResident(t, pool, rtID, "081234567890")

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

		if genResp.GeneratedCount != 0 {
			t.Errorf("expected generated_count to be 0 for voluntary Rp0 due, got %d", genResp.GeneratedCount)
		}
		if len(genResp.Bills) != 0 {
			t.Errorf("expected 0 bills generated, got %d", len(genResp.Bills))
		}

		// Verify no bills exist in database for this due
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

		for _, b := range listResp.Data {
			if b.DueID == createdDue.ID {
				t.Fatalf("unexpected bill found for voluntary Rp0 due: %+v", b)
			}
		}

		// Idempotency: re-running generate does not fail and still returns 0
		reqGen2 := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(bodyGen))
		reqGen2.Header.Set("Authorization", "Bearer "+tokenBendahara)
		reqGen2.Header.Set("Content-Type", "application/json")
		recGen2 := httptest.NewRecorder()
		router.ServeHTTP(recGen2, reqGen2)

		if recGen2.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created on idempotent retry, got %d: %s", recGen2.Code, recGen2.Body.String())
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
}

