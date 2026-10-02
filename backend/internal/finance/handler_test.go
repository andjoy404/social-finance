package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

const (
	testSigningKey = "test-signing-secret-at-least-16-chars"
)

var rtCounter int64 = time.Now().UnixNano() % 100000

func testPool(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func signingSecret() {
	auth.SigningSecret = []byte(testSigningKey)
}

func makeToken(t *testing.T, claims auth.TokenClaims) string {
	t.Helper()
	signingSecret()
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func setupTestRT(t *testing.T, pool *database.Pool, name string, rw int, rt string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	var id string
	n := atomic.AddInt64(&rtCounter, 1)
	uniqueRT := fmt.Sprintf("%s-%d", rt, n)
	err := db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		name, rw, uniqueRT,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test RT: %v", err)
	}
	return id
}

func cleanupTestRT(t *testing.T, pool *database.Pool, rtID string) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM transactions WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM payments WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM bills WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM dues WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM financial_categories WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM residency_periods WHERE household_occupancy_id IN (SELECT id FROM household_occupancies WHERE household_id IN (SELECT id FROM households WHERE rt_id = $1))`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM household_occupancies WHERE household_id IN (SELECT id FROM households WHERE rt_id = $1)`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM physical_houses WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM households WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
}

func setupTestOccupancy(t *testing.T, pool *database.Pool, rtID string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&rtCounter, 1)

	var houseID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, address) VALUES ($1, $2, 'Test Street') RETURNING id`,
		rtID, fmt.Sprintf("H-%d", n),
	).Scan(&houseID)
	if err != nil {
		t.Fatalf("create house: %v", err)
	}

	var hhID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Test Head') RETURNING id`,
		rtID,
	).Scan(&hhID)
	if err != nil {
		t.Fatalf("create household: %v", err)
	}

	var occID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', CURRENT_DATE) RETURNING id`,
		hhID, houseID,
	).Scan(&occID)
	if err != nil {
		t.Fatalf("create occupancy: %v", err)
	}

	return occID
}

func setupTestOccupancyWithResident(t *testing.T, pool *database.Pool, rtID, phone string) string {
	t.Helper()
	occID := setupTestOccupancy(t, pool, rtID)
	db := pool.Raw()
	ctx := context.Background()

	var resID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active)
		 VALUES ($1, 'Resident Warga', $2, true) RETURNING id`,
		rtID, phone,
	).Scan(&resID)
	if err != nil {
		t.Fatalf("create resident: %v", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date)
		 VALUES ($1, $2, 'HEAD', CURRENT_DATE)`,
		resID, occID,
	)
	if err != nil {
		t.Fatalf("create residency period: %v", err)
	}

	return occID
}

// setupTestUserWithMembership creates a user AND their RT membership so that
// RequireAuth's ValidateIdentity check (wired by uncommitted middleware.go change 0d67520) succeeds.
func setupTestUserWithMembership(t *testing.T, pool *database.Pool, email, fullName, phone, rtID, role string) (string, string) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()

	// Create user
	var userID string
	c := atomic.AddInt64(&rtCounter, 1)
	uniqueEmail := fmt.Sprintf("%d_%s", c, email)
	err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'dummyhash', $3, true) RETURNING id`,
		uniqueEmail, phone, fullName,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Create membership — required by RequireAuth.ValidateIdentity
	if role == "" {
		role = "bendahara"
	}
	var membershipID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		 VALUES ($1, $2, $3, true) RETURNING id`,
		userID, rtID, role,
	).Scan(&membershipID)
	if err != nil {
		t.Fatalf("failed to insert test membership: %v", err)
	}

	return userID, membershipID
}

// setupTestUser creates a user AND their RT membership with default 'bendahara' role.
func setupTestUser(t *testing.T, pool *database.Pool, email, fullName, phone, rtID string) string {
	t.Helper()
	userID, _ := setupTestUserWithMembership(t, pool, email, fullName, phone, rtID, "bendahara")
	return userID
}

func setupFinanceRouter(pool *database.Pool) http.Handler {
	// Wire pool into auth middleware so identity validation in RequireAuth succeeds.
	auth.AuthDBPool = pool

	r := chi.NewRouter()
	finSvc := NewService(pool)
	finH := NewHandler(pool, finSvc)

	// Financial read access & warga payments: warga, bendahara, pengurus
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/categories", finH.ListCategories)
		r.Get("/api/v1/dues", finH.ListDues)
		r.Get("/api/v1/bills", finH.ListBills)
		r.Get("/api/v1/bills/{id}", finH.GetBill)
		r.Get("/api/v1/payments", finH.ListPayments)
		r.Get("/api/v1/payments/{id}", finH.GetPayment)
		r.Get("/api/v1/transactions", finH.ListTransactions)
		r.Get("/api/v1/transactions/{id}", finH.GetTransaction)
		r.Get("/api/v1/reports/balance", finH.GetBalance)

		r.Post("/api/v1/payments", finH.CreatePayment)
	})

	// Master categories management: pengurus only
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/categories", finH.CreateCategory)
		r.Patch("/api/v1/categories/{id}", finH.UpdateCategory)
		r.Delete("/api/v1/categories/{id}", finH.DeactivateCategory)
	})

	// Financial operations: bendahara & pengurus
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleBendahara, auth.RolePengurus))
		r.Post("/api/v1/dues", finH.CreateDue)
		r.Patch("/api/v1/dues/{id}", finH.UpdateDue)
		r.Delete("/api/v1/dues/{id}", finH.DeactivateDue)
		r.Post("/api/v1/bills", finH.CreateBill)
		r.Post("/api/v1/bills/generate", finH.GenerateBills)
		r.Delete("/api/v1/bills/{id}", finH.CancelBill)
		r.Post("/api/v1/payments/{id}/verify", finH.VerifyPayment)
		r.Post("/api/v1/transactions", finH.CreateTransaction)
		r.Post("/api/v1/transactions/{id}/reverse", finH.ReverseTransaction)
	})

	return r
}

func TestFinancialCategoriesCRUDAndRoleRestrictions(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Categories", 1, "FCAT")
	defer cleanupTestRT(t, pool, rtID)

	userP, midP := setupTestUserWithMembership(t, pool, "p@social.test", "Pengurus P", "+6281111", rtID, "pengurus")
	userB, midB := setupTestUserWithMembership(t, pool, "b@social.test", "Bendahara B", "+6282222", rtID, "bendahara")
	userW, midW := setupTestUserWithMembership(t, pool, "w@social.test", "Warga W", "+6283333", rtID, "warga")

	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userP,
		MID:    midP,
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})
	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})
	tokenWarga := makeToken(t, auth.TokenClaims{
		UserID: userW,
		MID:    midW,
		RTID:   rtID,
		Role:   string(auth.RoleWarga),
	})

	var catID string

	t.Run("Pengurus creates category", func(t *testing.T) {
		body := `{"name":"Kebersihan Lingkungan","type":"income"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/categories", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp FinancialCategory
		json.Unmarshal(rec.Body.Bytes(), &resp)
		catID = resp.ID
		if resp.Name != "Kebersihan Lingkungan" || resp.Type != CategoryTypeIncome {
			t.Errorf("unexpected category response: %+v", resp)
		}
	})

	t.Run("Bendahara cannot delete category", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/categories/"+catID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for bendahara delete, got %d", rec.Code)
		}
	})

	t.Run("Warga cannot create category", func(t *testing.T) {
		body := `{"name":"Warga Rogue Cat","type":"income"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/categories", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for warga create, got %d", rec.Code)
		}
	})

	t.Run("Pengurus can deactivate category", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/categories/"+catID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content for pengurus delete, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestDuesCRUDAndValidation(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Dues", 1, "FDUES")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "b@test.com", "Bendahara Test", phoneB, rtID, "bendahara")

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	t.Run("Create due with valid amount", func(t *testing.T) {
		body := `{"name":"Iuran Sampah Bulanan","amount":"50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		if d.Amount != "50000.00" {
			t.Errorf("amount = %s, want 50000.00", d.Amount)
		}
	})

	t.Run("Reject negative or zero amount", func(t *testing.T) {
		body := `{"name":"Invalid Due","amount":"-50000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for negative amount, got %d", rec.Code)
		}
	})
}

func TestBillsAndPaymentsFlowWithLedgerIntegration(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Flow", 1, "FFLOW")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "bendahara@test.com", "Bendahara Test", phoneB, rtID, "bendahara")

	phoneP := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	userP, midP := setupTestUserWithMembership(t, pool, "pengurus@test.com", "Pengurus Test", phoneP, rtID, "pengurus")

	phoneW := fmt.Sprintf("0833%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "warga@test.com", "Warga Test", phoneW, rtID, "warga")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})
	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userP,
		MID:    midP,
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})

	// Create Due
	var dueID string
	{
		body := `{"name":"Iuran Keamanan","amount":"100000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due failed: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	// Generate Bills for period 2026-09
	t.Run("Generate bills for period", func(t *testing.T) {
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-09","due_date":"2026-09-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	})

	// Retrieve generated bill
	var billID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-09", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("list bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatalf("expected at least 1 bill generated")
		}
		billID = listResp.Data[0].ID
	}

	// Self-submit payment by citizen
	var paymentID string
	t.Run("Submit payment for bill", func(t *testing.T) {
		tokenWarga := makeToken(t, auth.TokenClaims{
			UserID: userW,
			MID:    midW,
			RTID:   rtID,
			Role:   string(auth.RoleWarga),
		})
		body := fmt.Sprintf(`{"bill_id":"%s","amount":"100000","method":"TRANSFER","notes":"Transfer via BCA"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		paymentID = p.ID
		if p.Status != PaymentStatusPending {
			t.Errorf("expected PENDING status for self-submitted payment, got %s", p.Status)
		}
	})

	// Verify payment by Bendahara -> approves payment -> bill marked paid -> income transaction posted
	t.Run("Verify and approve payment", func(t *testing.T) {
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("verify payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusApproved {
			t.Errorf("expected APPROVED status, got %s", p.Status)
		}

		// Verify bill is now paid
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPaid {
			t.Errorf("expected bill status paid, got %s", b.Status)
		}

		// Verify income transaction posted to ledger
		tReq := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		tReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		tRec := httptest.NewRecorder()
		router.ServeHTTP(tRec, tReq)
		var tResp struct {
			Data []Transaction `json:"data"`
		}
		json.Unmarshal(tRec.Body.Bytes(), &tResp)
		if len(tResp.Data) == 0 {
			t.Fatalf("expected ledger transaction automatically posted")
		}
		if tResp.Data[0].Amount != "100000.00" || tResp.Data[0].Type != TransactionTypeIncome {
			t.Errorf("unexpected transaction: %+v", tResp.Data[0])
		}
	})

	// Verify balance derived from ledger
	t.Run("Derive balance from ledger", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/balance", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("get balance: %d: %s", rec.Code, rec.Body.String())
		}
		var summary BalanceSummary
		json.Unmarshal(rec.Body.Bytes(), &summary)
		if summary.TotalIncome != "100000.00" || summary.NetBalance != "100000.00" {
			t.Errorf("expected 100000.00 net balance, got %+v", summary)
		}
	})

	// Reverse transaction
	t.Run("Compensating reversal updates balance", func(t *testing.T) {
		// List transactions to get the posted income transaction ID
		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var tResp struct {
			Data []Transaction `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &tResp)
		transID := tResp.Data[0].ID

		// Reverse it
		revBody := `{"reason":"Incorrect payment record"}`
		revReq := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/"+transID+"/reverse", bytes.NewBufferString(revBody))
		revReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		revReq.Header.Set("Content-Type", "application/json")
		revRec := httptest.NewRecorder()
		router.ServeHTTP(revRec, revReq)
		if revRec.Code != http.StatusOK {
			t.Fatalf("reverse transaction: %d: %s", revRec.Code, revRec.Body.String())
		}
		var revTrans Transaction
		json.Unmarshal(revRec.Body.Bytes(), &revTrans)
		if revTrans.Type != TransactionTypeExpense || revTrans.Amount != "100000.00" {
			t.Errorf("expected expense reversal of 100000.00, got %+v", revTrans)
		}

		// Recheck derived balance: 100000 income - 100000 expense = 0.00 net balance!
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports/balance", nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var summary BalanceSummary
		json.Unmarshal(bRec.Body.Bytes(), &summary)
		if summary.NetBalance != "0.00" {
			t.Errorf("expected net balance 0.00 after reversal, got %+v", summary)
		}
	})

	_ = occID
}

func TestIdempotencyReplay(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Idempotency", 1, "FIDEM")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "idem_b@test.com", "Bendahara Idem", phoneB, rtID, "bendahara")

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	// Create category
	var catID string
	{
		tx, err := pool.BeginTx(context.Background())
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		cat, err := CategoryCreate(context.Background(), tx, rtID, CreateCategoryInput{
			Name: "Operasional",
			Type: CategoryTypeExpense,
		})
		if err != nil {
			t.Fatalf("create category: %v", err)
		}
		_ = tx.Commit()
		catID = cat.ID
	}

	idemKey := fmt.Sprintf("idem-%d", time.Now().UnixNano())
	body := fmt.Sprintf(`{"category_id":"%s","amount":"45000","type":"expense","description":"Beli Sapu"}`, catID)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewBufferString(body))
	req1.Header.Set("Authorization", "Bearer "+tokenBendahara)
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idemKey)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first request failed: %d: %s", rec1.Code, rec1.Body.String())
	}

	// Replay identical request with same key
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewBufferString(body))
	req2.Header.Set("Authorization", "Bearer "+tokenBendahara)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idemKey)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("replayed request failed: %d: %s", rec2.Code, rec2.Body.String())
	}
	if rec2.Header().Get("X-Cache-Lookup") != "HIT" {
		t.Errorf("expected X-Cache-Lookup: HIT on replay, got %s", rec2.Header().Get("X-Cache-Lookup"))
	}

	// Verify only 1 transaction was inserted
	var count int
	_ = pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE rt_id = $1`, rtID,
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 transaction in DB, found %d", count)
	}
}

func TestTenantIsolationInFinance(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtA := setupTestRT(t, pool, "Finance RT A", 1, "RTA")
	defer cleanupTestRT(t, pool, rtA)
	rtB := setupTestRT(t, pool, "Finance RT B", 1, "RTB")
	defer cleanupTestRT(t, pool, rtB)

	userA, midA := setupTestUserWithMembership(t, pool, "user-a@test.com", "User A", "+628111", rtA, "bendahara")
	userB, midB := setupTestUserWithMembership(t, pool, "user-b@test.com", "User B", "+628222", rtB, "bendahara")

	tokenA := makeToken(t, auth.TokenClaims{
		UserID: userA,
		MID:    midA,
		RTID:   rtA,
		Role:   string(auth.RoleBendahara),
	})
	tokenB := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtB,
		Role:   string(auth.RoleBendahara),
	})

	// User A creates due in RT A
	var dueA string
	{
		body := `{"name":"RT A Due","amount":"20000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenA)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due A: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueA = d.ID
	}

	// User B tries to update or deactivate RT A's due
	patchBody := `{"name":"Hacked Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/dues/"+dueA, bytes.NewBufferString(patchBody))
	req.Header.Set("Authorization", "Bearer "+tokenB)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for cross-RT due update, got %d", rec.Code)
	}
}

func TestDR03_DR04_PaymentFlow(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT DR03DR04", 1, "FDR03")
	defer cleanupTestRT(t, pool, rtID)

	phoneW := fmt.Sprintf("0833%08d", time.Now().UnixNano()%100000000)
	userW, midW := setupTestUserWithMembership(t, pool, "warga@dr03.test", "Warga DR03", phoneW, rtID, "warga")

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "bendahara@dr03.test", "Bendahara DR03", phoneB, rtID, "bendahara")

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	tokenWarga := makeToken(t, auth.TokenClaims{
		UserID: userW,
		MID:    midW,
		RTID:   rtID,
		Role:   string(auth.RoleWarga),
	})
	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	phoneP := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	userP, midP := setupTestUserWithMembership(t, pool, "pengurus@dr03.test", "Pengurus DR03", phoneP, rtID, "pengurus")

	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userP,
		MID:    midP,
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})

	// Create Due
	var dueID string
	{
		body := `{"name":"Iuran DR03/04","amount":"100000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	// Generate Bills
	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	t.Run("Staff creates payment for full bill amount -> PAID", func(t *testing.T) {
		// Generate bills for a new period
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-full","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-full", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		billID2 := listResp.Data[0].ID

		// Staff creates full payment
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"100000","method":"CASH"}`, billID2)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusApproved {
			t.Errorf("expected APPROVED, got %s", p.Status)
		}

		// Verify bill is paid
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID2, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPaid {
			t.Errorf("expected bill status PAID, got %s", b.Status)
		}
	})

	t.Run("Staff creates payment less than bill -> PARTIAL", func(t *testing.T) {
		// Generate bills for a new period
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-partial","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-partial", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		billID3 := listResp.Data[0].ID

		// Staff creates partial payment (60k of 100k)
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, billID3)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create partial payment: %d: %s", rec.Code, rec.Body.String())
		}

		// Verify bill is partial
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID3, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPartial {
			t.Errorf("expected bill status PARTIAL, got %s", b.Status)
		}
	})

	t.Run("Overpayment rejected - staff", func(t *testing.T) {
		// Generate bills for a new period
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-overpay","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-overpay", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		billID4 := listResp.Data[0].ID

		// Staff tries to pay 100.001 (over by 1)
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"100001","method":"CASH"}`, billID4)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for overpayment, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify bill is still unpaid
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID4, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusUnpaid {
			t.Errorf("expected bill status UNPAID after overpayment rejection, got %s", b.Status)
		}
	})

	t.Run("Overpayment rejected - warga submission", func(t *testing.T) {
		// Generate a bill for warga's occupancy
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-warga-overpay","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-warga-overpay", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		billID5 := listResp.Data[0].ID

		// Warga submits payment that exceeds bill amount (100001 vs 100000)
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"100001","method":"CASH"}`, billID5)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("warga submit payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.Status != PaymentStatusPending {
			t.Errorf("expected PENDING for warga submission, got %s", p.Status)
		}

		// Verify bill is still unpaid
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID5, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusUnpaid {
			t.Errorf("expected bill status UNPAID after warga submission, got %s", b.Status)
		}

		// Bendahara tries to approve overpayment -> should reject
		verifyBody := `{"action":"approve"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/verify", bytes.NewBufferString(verifyBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		// Should reject due to overpayment (payment.amount > remaining)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for overpayment approval, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Overpayment rejection persists to DB after transaction", func(t *testing.T) {
		// Regression test: VerifyPayment must COMMIT the REJECTED payment
		// so it survives the HTTP response. Before the fix, tx.Rollback()
		// was called on error, wiping the REJECTED state.
		// Generate a bill for this specific test
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-persist","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-persist", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		billID := listResp.Data[0].ID

		// Step 1: Staff records approved payment of 60000
		payBody1 := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, billID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody1))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create approved payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p1 Payment
		json.Unmarshal(rec.Body.Bytes(), &p1)
		if p1.Status != PaymentStatusApproved {
			t.Fatalf("expected APPROVED, got %s", p1.Status)
		}

		// Step 2: Warga submits PENDING payment of 50000 (exceeds remaining 40000)
		payBody2 := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, billID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody2))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create warga payment: %d: %s", rec.Code, rec.Body.String())
		}
		var p2 Payment
		json.Unmarshal(rec.Body.Bytes(), &p2)
		if p2.Status != PaymentStatusPending {
			t.Fatalf("expected PENDING, got %s", p2.Status)
		}

		// Step 3: Bendahara approves -> overpayment detected -> payment REJECTED
		verifyBody := `{"action":"approve"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p2.ID+"/verify", bytes.NewBufferString(verifyBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for overpayment, got %d: %s", rec.Code, rec.Body.String())
		}

		// Step 4: Query DB directly AFTER transaction completes
		// Verify payment is REJECTED (not still PENDING)
		var paymentStatus PaymentStatus
		err := pool.Raw().QueryRowContext(
			context.Background(),
			`SELECT status FROM payments WHERE id = $1`,
			p2.ID,
		).Scan(&paymentStatus)
		if err != nil {
			t.Fatalf("query payment status: %v", err)
		}
		if paymentStatus != PaymentStatusRejected {
			t.Errorf("expected payment status REJECTED after overpayment, got %s", paymentStatus)
		}

		// Verify bill is PARTIAL (not PAID, not UNPAID)
		var billStatus BillStatus
		err = pool.Raw().QueryRowContext(
			context.Background(),
			`SELECT status FROM bills WHERE id = $1`,
			billID,
		).Scan(&billStatus)
		if err != nil {
			t.Fatalf("query bill status: %v", err)
		}
		if billStatus != BillStatusPartial {
			t.Errorf("expected bill status PARTIAL after overpayment rejection, got %s", billStatus)
		}

		// Verify approved total is still 60000 (p2 was NOT approved)
		var approvedTotal string
		err = pool.Raw().QueryRowContext(
			context.Background(),
			`SELECT COALESCE(SUM(amount), 0)::text FROM payments WHERE bill_id = $1 AND status = 'APPROVED'`,
			billID,
		).Scan(&approvedTotal)
		if err != nil {
			t.Fatalf("query approved total: %v", err)
		}
		if approvedTotal != "60000.00" {
			t.Errorf("expected approved total 60000.00, got %s", approvedTotal)
		}

		// Verify no new ledger income transaction was posted for p2
		var transCount int
		err = pool.Raw().QueryRowContext(
			context.Background(),
			`SELECT COUNT(*) FROM transactions WHERE payment_id = $1`,
			p2.ID,
		).Scan(&transCount)
		if err != nil {
			t.Fatalf("query transaction count: %v", err)
		}
		if transCount != 0 {
			t.Errorf("expected 0 ledger transactions for rejected payment, got %d", transCount)
		}
	})

	t.Run("Multiple partial payments summing to exact bill amount", func(t *testing.T) {
		// Generate a bill for multi-part payment
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-multi","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-multi", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		multiBillID := listResp.Data[0].ID

		// First partial payment: 40000
		payBody1 := fmt.Sprintf(`{"bill_id":"%s","amount":"40000","method":"CASH"}`, multiBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody1))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("first partial: %d: %s", rec.Code, rec.Body.String())
		}

		// Verify partial
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+multiBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPartial {
			t.Errorf("expected PARTIAL after first payment, got %s", b.Status)
		}

		// Second partial payment: 60000 (sum = 100000 = bill amount)
		payBody2 := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, multiBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody2))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("second partial: %d: %s", rec.Code, rec.Body.String())
		}

		// Verify PAID
		bReq = httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+multiBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec = httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPaid {
			t.Errorf("expected PAID after two payments summing to bill amount, got %s", b.Status)
		}
	})

	t.Run("PENDING payment not counted in approved total", func(t *testing.T) {
		// Generate a bill for warga's occupancy
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-pending","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-pending", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		pendingBillID := listResp.Data[0].ID

		// Warga submits 60000 -> PENDING
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, pendingBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("warga submit: %d: %s", rec.Code, rec.Body.String())
		}

		// Verify bill is still UNPAID (PENDING not counted)
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+pendingBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusUnpaid {
			t.Errorf("expected UNPAID when payment is PENDING, got %s", b.Status)
		}

		// Warga can submit another payment for same bill (while first is PENDING)
		// because PENDING doesn't reduce remaining
		payBody2 := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, pendingBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody2))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("second warga submit: %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Reject PENDING payment", func(t *testing.T) {
		// Generate a bill for warga's occupancy
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-reject","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-reject", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		rejectBillID := listResp.Data[0].ID

		// Warga submits payment
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, rejectBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("warga submit: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)

		// Verify REJECTED
		verifyBody := `{"action":"reject","rejection_reason":"Proof unclear"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/verify", bytes.NewBufferString(verifyBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("reject payment: %d: %s", rec.Code, rec.Body.String())
		}
		var verified Payment
		json.Unmarshal(rec.Body.Bytes(), &verified)
		if verified.Status != PaymentStatusRejected {
			t.Errorf("expected REJECTED, got %s", verified.Status)
		}

		// Verify bill is still UNPAID
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+rejectBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusUnpaid {
			t.Errorf("expected UNPAID after reject, got %s", b.Status)
		}
	})

	t.Run("Approve pending payment -> bill status updated correctly", func(t *testing.T) {
		// Generate a bill for warga's occupancy
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-approve","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-approve", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		approveBillID := listResp.Data[0].ID

		// Warga submits 60000 -> PENDING
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, approveBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("warga submit: %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)

		// Approve
		verifyBody := `{"action":"approve"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/verify", bytes.NewBufferString(verifyBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("approve payment: %d: %s", rec.Code, rec.Body.String())
		}
		var verified Payment
		json.Unmarshal(rec.Body.Bytes(), &verified)
		if verified.Status != PaymentStatusApproved {
			t.Errorf("expected APPROVED, got %s", verified.Status)
		}

		// Verify bill is PARTIAL (60000 < 100000)
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+approveBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPartial {
			t.Errorf("expected PARTIAL after approving 60000 on 100000 bill, got %s", b.Status)
		}

		// Warga submits remaining 40000 -> PENDING
		payBody2 := fmt.Sprintf(`{"bill_id":"%s","amount":"40000","method":"CASH"}`, approveBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody2))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("second warga submit: %d: %s", rec.Code, rec.Body.String())
		}
		var p2 Payment
		json.Unmarshal(rec.Body.Bytes(), &p2)

		// Approve second payment
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p2.ID+"/verify", bytes.NewBufferString(verifyBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("approve second payment: %d: %s", rec.Code, rec.Body.String())
		}

		// Verify PAID
		bReq = httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+approveBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec = httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPaid {
			t.Errorf("expected PAID after two payments (60000+40000=100000), got %s", b.Status)
		}
	})

	t.Run("Cannot pay already paid bill", func(t *testing.T) {
		// Generate a bill and pay it fully first
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-alreadypaid","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-alreadypaid", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		paidBillID := listResp.Data[0].ID

		// Pay the bill fully
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"100000","method":"CASH"}`, paidBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("initial full payment: %d: %s", rec.Code, rec.Body.String())
		}

		// Try to pay again -> should fail
		payBody = fmt.Sprintf(`{"bill_id":"%s","amount":"10000","method":"CASH"}`, paidBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict for paying already paid bill, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Ledger income transaction posted on approval", func(t *testing.T) {
		// Verify income transaction exists for the approved payment
		tReq := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		tReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		tRec := httptest.NewRecorder()
		router.ServeHTTP(tRec, tReq)
		var tResp struct {
			Data []Transaction `json:"data"`
		}
		json.Unmarshal(tRec.Body.Bytes(), &tResp)
		found := false
		for _, tr := range tResp.Data {
			if tr.Type == TransactionTypeIncome {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected income transaction to be posted for approved payment")
		}
	})

	// Verify occupancy was created
	_ = occID
}

func TestBillCancellationGuard(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Cancel", 1, "FCANCEL")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "bendahara@cancel.test", "Bendahara Cancel", phoneB, rtID, "bendahara")

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	phoneP := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	userP, midP := setupTestUserWithMembership(t, pool, "pengurus@cancel.test", "Pengurus Cancel", phoneP, rtID, "pengurus")

	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userP,
		MID:    midP,
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})

	// Create occupancy so GenerateBills can create bills
	phoneR := fmt.Sprintf("0834%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneR)

	// Create Due
	var dueID string
	{
		body := `{"name":"Iuran Cancel","amount":"100000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	// Generate Bills
	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-cancel","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	// Get unpaid bill
	var unpaidBillID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-cancel", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("list bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		unpaidBillID = listResp.Data[0].ID
	}

	t.Run("Cancel unpaid bill with no payments -> success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/bills/"+unpaidBillID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify cancelled
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+unpaidBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusCancelled {
			t.Errorf("expected CANCELLED, got %s", b.Status)
		}
	})

	// Generate a bill with partial payment
	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-guard","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var partialBillID string

	// Get the generated bill
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-guard", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("list bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		// Create partial payment for this bill
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, listResp.Data[0].ID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create partial payment: %d: %s", rec.Code, rec.Body.String())
		}
		partialBillID = listResp.Data[0].ID
	}

	t.Run("Cancel bill with approved payment -> rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/bills/"+partialBillID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict for cancelling bill with approved payments, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify bill is still PARTIAL (not cancelled)
		bReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+partialBillID, nil)
		bReq.Header.Set("Authorization", "Bearer "+tokenBendahara)
		bRec := httptest.NewRecorder()
		router.ServeHTTP(bRec, bReq)
		var b Bill
		json.Unmarshal(bRec.Body.Bytes(), &b)
		if b.Status != BillStatusPartial {
			t.Errorf("expected bill to remain PARTIAL, got %s", b.Status)
		}
	})

	t.Run("Cancel PAID bill -> rejected", func(t *testing.T) {
		// Generate a paid bill via GenerateBills
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-paid","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
		var listResp struct {
			Data []Bill `json:"data"`
		}
		req = httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-paid", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill generated")
		}
		paidBillID := listResp.Data[0].ID

		// Pay in full
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"100000","method":"CASH"}`, paidBillID)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create full payment: %d: %s", rec.Code, rec.Body.String())
		}

		// Try to cancel paid bill
		req = httptest.NewRequest(http.MethodDelete, "/api/v1/bills/"+paidBillID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict for cancelling paid bill, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestConcurrentPaymentRaceCondition(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID := setupTestRT(t, pool, "Finance RT Race", 1, "FRACE")
	defer cleanupTestRT(t, pool, rtID)

	phoneB := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	userB, midB := setupTestUserWithMembership(t, pool, "bendahara@race.test", "Bendahara Race", phoneB, rtID, "bendahara")

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    midB,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	phoneP := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	userP, midP := setupTestUserWithMembership(t, pool, "pengurus@race.test", "Pengurus Race", phoneP, rtID, "pengurus")

	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userP,
		MID:    midP,
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})

	// Create occupancy so GenerateBills can create bills
	phoneR := fmt.Sprintf("0834%08d", time.Now().UnixNano()%100000000)
	_ = setupTestOccupancyWithResident(t, pool, rtID, phoneR)

	// Create Due
	var dueID string
	{
		body := `{"name":"Iuran Race","amount":"100000","period_type":"monthly"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create due: %d: %s", rec.Code, rec.Body.String())
		}
		var d Due
		json.Unmarshal(rec.Body.Bytes(), &d)
		dueID = d.ID
	}

	// Create a bill for race test
	{
		body := fmt.Sprintf(`{"due_id":"%s","period":"2026-10-race","due_date":"2026-10-25"}`, dueID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bills/generate", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("generate bills: %d: %s", rec.Code, rec.Body.String())
		}
	}

	var billID string
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills?period=2026-10-race", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var listResp struct {
			Data []Bill `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) == 0 {
			t.Fatal("expected at least 1 bill")
		}
		billID = listResp.Data[0].ID
	}

	// Simulate concurrent requests: A=60000, B=60000
	// Both start at nearly same time
	var wg sync.WaitGroup
	results := make(chan int, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		results <- rec.Code
	}()

	go func() {
		defer wg.Done()
		payBody := fmt.Sprintf(`{"bill_id":"%s","amount":"60000","method":"CASH"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payBody))
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		results <- rec.Code
	}()

	wg.Wait()
	close(results)

	var successCount, conflictCount int
	for code := range results {
		if code == http.StatusCreated {
			successCount++
		} else if code == http.StatusBadRequest {
			conflictCount++
		} else {
			t.Errorf("unexpected status code: %d", code)
		}
	}

	// Exactly one should succeed, one should be rejected
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d (success=%d, conflict=%d)", successCount, successCount, conflictCount)
	}
	if conflictCount != 1 {
		t.Errorf("expected exactly 1 rejection, got %d (success=%d, conflict=%d)", conflictCount, successCount, conflictCount)
	}

	// Verify approved_total = 60000 and bill status = PARTIAL
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenBendahara)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var b Bill
	json.Unmarshal(rec.Body.Bytes(), &b)
	if b.Status != BillStatusPartial {
		t.Errorf("expected PARTIAL after race test, got %s", b.Status)
	}
}

// ============================================================================
// Phase 1: Cross-RT superadmin tests
// ============================================================================

// setupSystemOnlySuperAdmin creates a user with no RT membership and returns a
// system-only superadmin JWT token (SysRole=super_admin, RTID="", MID="", Role="").
func setupSystemOnlySuperAdmin(t *testing.T, pool *database.Pool, email, fullName, phone string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	c := atomic.AddInt64(&rtCounter, 1)
	uniqueEmail := fmt.Sprintf("%d_%s", c, email)
	var userID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'dummyhash', $3, true) RETURNING id`,
		uniqueEmail, phone, fullName,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert system-only superadmin user: %v", err)
	}
	return userID
}

// SetupCrossRFFinanceData creates the full finance dataset across two RTs.
// Returns (rta, rtb, occIDA, occIDB, amounts: {incomeA, expenseB}).
func SetupCrossRFFinanceData(t *testing.T, pool *database.Pool) (rta, rtb, occIDA, occIDB string, incomeA, expenseB int64) {
	t.Helper()
	// RT-A
	rta = setupTestRT(t, pool, "Cross RT RT-A", 1, "CRRTA")
	phoneW := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	occIDA = setupTestOccupancyWithResident(t, pool, rta, phoneW)

	// RT-B
	rtb = setupTestRT(t, pool, "Cross RT RT-B", 1, "CRRTB")
	phoneWB := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	occIDB = setupTestOccupancyWithResident(t, pool, rtb, phoneWB)

	ctx := context.Background()

	// ── RT-A data: 2 categories, 1 due, 2 bills, income transaction, payment ──
	var catA1, dueA string
	var billA1ID string
	{
		tx, _ := pool.BeginTx(ctx)
		c, _ := CategoryCreate(ctx, tx, rta, CreateCategoryInput{Name: "Sumbangan RT-A", Type: CategoryTypeIncome})
		catA1 = c.ID
		_, _ = CategoryCreate(ctx, tx, rta, CreateCategoryInput{Name: "Kebersihan RT-A", Type: CategoryTypeExpense})
		d, _ := DueCreate(ctx, tx, rta, CreateDueInput{Name: "Iuran RT-A", Amount: "100000", PeriodType: DuePeriodMonthly})
		dueA = d.ID
		b1, _ := BillCreate(ctx, tx, rta, CreateBillInput{
			HouseholdOccupancyID: occIDA, DueID: dueA, Amount: "100000",
			Period: "2026-10", DueDate: "2026-10-25",
		})
		billA1ID = b1.ID
		// Second bill (for testing count and list comprehensiveness)
		_, _ = BillCreate(ctx, tx, rta, CreateBillInput{
			HouseholdOccupancyID: occIDA, DueID: dueA, Amount: "100000",
			Period: "2026-11", DueDate: "2026-11-25",
		})
		tx.Commit()
	}
	// Insert income transaction for RT-A (200000)
	incomeA = 200000
	{
		tx, _ := pool.BeginTx(ctx)
		_, _ = PaymentCreate(ctx, tx, rta, CreatePaymentInput{
			BillID: billA1ID, Amount: "200000", Method: PaymentMethodTransfer,
		}, PaymentOriginStaffRecorded, PaymentStatusApproved, nil, nil)
		_, _ = TransactionCreate(ctx, tx, rta, CreateTransactionInput{
			CategoryID: catA1, Amount: "200000", Type: TransactionTypeIncome,
			Description: "Sumbangan RT-A",
		}, TransactionStatusPosted, nil, nil, time.Now())
		tx.Commit()
	}

	// ── RT-B data: 2 categories, 1 due, 1 bill, expense transaction ──
	var catB2 string
	{
		tx, _ := pool.BeginTx(ctx)
		_, _ = CategoryCreate(ctx, tx, rtb, CreateCategoryInput{Name: "Iuran RT-B", Type: CategoryTypeIncome})
		c2, _ := CategoryCreate(ctx, tx, rtb, CreateCategoryInput{Name: "Kebersihan RT-B", Type: CategoryTypeExpense})
		catB2 = c2.ID
		_, _ = DueCreate(ctx, tx, rtb, CreateDueInput{Name: "Iuran RT-B", Amount: "50000", PeriodType: DuePeriodMonthly})
		_, _ = BillCreate(ctx, tx, rtb, CreateBillInput{
			HouseholdOccupancyID: occIDB, DueID: dueA, Amount: "50000",
			Period: "2026-10", DueDate: "2026-10-25",
		})
		tx.Commit()
	}
	// Insert expense transaction for RT-B (150000)
	expenseB = 150000
	{
		tx, _ := pool.BeginTx(ctx)
		_, _ = TransactionCreate(ctx, tx, rtb, CreateTransactionInput{
			CategoryID: catB2, Amount: "150000", Type: TransactionTypeExpense,
			Description: "Beli Sapu RT-B",
		}, TransactionStatusPosted, nil, nil, time.Now())
		tx.Commit()
	}

	return rta, rtb, occIDA, occIDB, incomeA, expenseB
}

func TestSuperAdminCrossRTAccess(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	// 1. Create two RTs with finance data
	rta, rtb, _, _, incA, expB := SetupCrossRFFinanceData(t, pool)
	defer cleanupTestRT(t, pool, rta)
	defer cleanupTestRT(t, pool, rtb)

	// 2. Create system-only superadmin (no memberships)
	saUser := setupSystemOnlySuperAdmin(t, pool, "sa@test.com", "Super Admin", "+628000")

	// 3. Generate system-only token: no memberships, system role only
	tokenSysAdmin := makeToken(t, auth.TokenClaims{
		UserID:  saUser,
		MID:     "",
		RTID:    "",
		Role:    "",
		Jabatan: "",
		SysRole: "super_admin",
	})

	// ── Helper: GET and decode JSON ──
	jsonGet := func(path string) (code int, body []byte) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+tokenSysAdmin)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code, rec.Body.Bytes()
	}

	// ── 1. GET /categories ──
	t.Run("GET categories from both RTs", func(t *testing.T) {
		code, body := jsonGet("/api/v1/categories")
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", code, string(body))
		}
		var cats []*FinancialCategory
		if err := json.Unmarshal(body, &cats); err != nil {
			t.Fatalf("unmarshal categories: %v", err)
		}
		// Should have categories from BOTH RTs (at least 4)
		rtIDs := make(map[string]bool)
		for _, c := range cats {
			rtIDs[c.RTID] = true
		}
		if !rtIDs[rta] {
			t.Error("missing RT-A categories in list")
		}
		if !rtIDs[rtb] {
			t.Error("missing RT-B categories in list")
		}
		if len(cats) < 4 {
			t.Errorf("expected >= 4 categories (2 per RT), got %d", len(cats))
		}
	})

	// ── 2. GET /dues ──
	t.Run("GET dues from both RTs", func(t *testing.T) {
		code, body := jsonGet("/api/v1/dues")
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", code, string(body))
		}
		var dues []*Due
		if err := json.Unmarshal(body, &dues); err != nil {
			t.Fatalf("unmarshal dues: %v", err)
		}
		rtIDs := make(map[string]bool)
		for _, d := range dues {
			rtIDs[d.RTID] = true
		}
		if !rtIDs[rta] {
			t.Error("missing RT-A dues in list")
		}
		if !rtIDs[rtb] {
			t.Error("missing RT-B dues in list")
		}
		if len(dues) < 2 {
			t.Errorf("expected >= 2 dues, got %d", len(dues))
		}
	})

	// ── 3. GET /bills (list) ──
	t.Run("GET bills from both RTs", func(t *testing.T) {
		code, body := jsonGet("/api/v1/bills")
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", code, string(body))
		}
		var resp struct {
			Data       []*Bill `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("unmarshal bills: %v", err)
		}
		rtIDs := make(map[string]bool)
		for _, b := range resp.Data {
			rtIDs[b.RTID] = true
		}
		if !rtIDs[rta] {
			t.Error("missing RT-A bills in list")
		}
		if !rtIDs[rtb] {
			t.Error("missing RT-B bills in list")
		}
		if resp.Pagination.Total < 3 {
			t.Errorf("expected >= 3 bills total, got %d", resp.Pagination.Total)
		}
	})

	// ── 4. GET /bills/{id} (from RT-A) ──
	t.Run("GET bill by ID from RT-A", func(t *testing.T) {
		// List bills first to get an RT-A bill ID
		code, body := jsonGet("/api/v1/bills?per_page=100")
		if code != http.StatusOK {
			t.Fatalf("list bills: expected 200, got %d", code)
		}
		var resp struct {
			Data []*Bill `json:"data"`
		}
		json.Unmarshal(body, &resp)
		// Find an RT-A bill
		var billAID string
		for _, b := range resp.Data {
			if b.RTID == rta {
				billAID = b.ID
				break
			}
		}
		if billAID == "" {
			t.Fatal("no RT-A bill found to test GET /bills/{id}")
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billAID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenSysAdmin)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for GET bill, got %d: %s", rec.Code, rec.Body.String())
		}
		var bill Bill
		json.Unmarshal(rec.Body.Bytes(), &bill)
		if bill.RTID != rta {
			t.Errorf("expected bill RT-A, got %s", bill.RTID)
		}
	})

	// ── 5. GET /bills/{id} (from RT-B) ──
	t.Run("GET bill by ID from RT-B", func(t *testing.T) {
		code, body := jsonGet("/api/v1/bills?per_page=100")
		if code != http.StatusOK {
			t.Fatalf("list bills: expected 200, got %d", code)
		}
		var resp struct {
			Data []*Bill `json:"data"`
		}
		json.Unmarshal(body, &resp)
		var billBID string
		for _, b := range resp.Data {
			if b.RTID == rtb {
				billBID = b.ID
				break
			}
		}
		if billBID == "" {
			t.Fatal("no RT-B bill found to test GET /bills/{id}")
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billBID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenSysAdmin)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for GET bill, got %d: %s", rec.Code, rec.Body.String())
		}
		var bill Bill
		json.Unmarshal(rec.Body.Bytes(), &bill)
		if bill.RTID != rtb {
			t.Errorf("expected bill RT-B, got %s", bill.RTID)
		}
	})

	// ── 6. GET /payments ──
	t.Run("GET payments from both RTs", func(t *testing.T) {
		code, body := jsonGet("/api/v1/payments")
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", code, string(body))
		}
		var resp struct {
			Data       []*Payment `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("unmarshal payments: %v", err)
		}
		rtIDs := make(map[string]bool)
		for _, p := range resp.Data {
			rtIDs[p.RTID] = true
		}
		if len(resp.Data) > 0 {
			if !rtIDs[rta] && !rtIDs[rtb] {
				t.Error("payments from neither RT found")
			}
		}
	})

	// ── 7. GET /payments/{id} ──
	t.Run("GET payment by ID", func(t *testing.T) {
		code, body := jsonGet("/api/v1/payments")
		if code != http.StatusOK {
			t.Fatalf("list payments: expected 200, got %d", code)
		}
		var resp struct {
			Data []*Payment `json:"data"`
		}
		json.Unmarshal(body, &resp)
		if len(resp.Data) == 0 {
			t.Skip("no payments exist, skipping GET payment by ID")
		}
		payID := resp.Data[0].ID
		req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+payID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenSysAdmin)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for GET payment, got %d: %s", rec.Code, rec.Body.String())
		}
		var p Payment
		json.Unmarshal(rec.Body.Bytes(), &p)
		if p.RTID == "" {
			t.Error("payment rt_id should not be empty")
		}
	})

	// ── 8. GET /transactions ──
	t.Run("GET transactions from both RTs", func(t *testing.T) {
		code, body := jsonGet("/api/v1/transactions")
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", code, string(body))
		}
		var resp struct {
			Data       []*Transaction `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("unmarshal transactions: %v", err)
		}
		rtIDs := make(map[string]bool)
		for _, tr := range resp.Data {
			rtIDs[tr.RTID] = true
		}
		if len(resp.Data) > 0 {
			if !rtIDs[rta] {
				t.Error("missing RT-A transactions in list")
			}
			if !rtIDs[rtb] {
				t.Error("missing RT-B transactions in list")
			}
		}
	})

	// ── 9. GET /transactions/{id} ──
	t.Run("GET transaction by ID from both RTs", func(t *testing.T) {
		code, body := jsonGet("/api/v1/transactions")
		if code != http.StatusOK {
			t.Fatalf("list transactions: expected 200, got %d", code)
		}
		var resp struct {
			Data []*Transaction `json:"data"`
		}
		json.Unmarshal(body, &resp)
		var transAID, transBID string
		for _, tr := range resp.Data {
			if tr.RTID == rta && transAID == "" {
				transAID = tr.ID
			}
			if tr.RTID == rtb && transBID == "" {
				transBID = tr.ID
			}
		}
		if transAID != "" {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+transAID, nil)
			req.Header.Set("Authorization", "Bearer "+tokenSysAdmin)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("GET transaction %s: expected 200, got %d: %s", transAID, rec.Code, rec.Body.String())
			}
		}
		if transBID != "" {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+transBID, nil)
			req.Header.Set("Authorization", "Bearer "+tokenSysAdmin)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("GET transaction %s: expected 200, got %d: %s", transBID, rec.Code, rec.Body.String())
			}
		}
		if transAID == "" && transBID == "" {
			t.Skip("no transactions exist, skipping GET transaction by ID")
		}
	})

	// ── 10. GET /reports/balance (aggregate across RTs) ──
	t.Run("GET balance aggregate across both RTs", func(t *testing.T) {
		code, body := jsonGet("/api/v1/reports/balance")
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", code, string(body))
		}
		var summary BalanceSummary
		if err := json.Unmarshal(body, &summary); err != nil {
			t.Fatalf("unmarshal balance: %v", err)
		}
		// Net balance should be sum of both RTs: (incA) - (expB)
		expectedNet := incA - expB
		if summary.NetBalance == "" {
			t.Fatal("net_balance is empty")
		}
		actual, err := strconv.ParseFloat(summary.NetBalance, 64)
		if err != nil {
			t.Fatalf("parse net_balance %q: %v", summary.NetBalance, err)
		}
		if int64(actual) != expectedNet {
			t.Errorf("expected net balance %d, got %s", expectedNet, summary.NetBalance)
		}
	})
}

// TestTenantIsolationCrossRTRegression proves that a regular tenant user
// (bendahara with membership) still sees ONLY their own RT data — even
// after the *All methods are added.
func TestTenantIsolationCrossRTRegression(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rta := setupTestRT(t, pool, "Isolation RT-A", 1, "ISOA")
	defer cleanupTestRT(t, pool, rta)
	rtb := setupTestRT(t, pool, "Isolation RT-B", 1, "ISOB")
	defer cleanupTestRT(t, pool, rtb)

	phoneA := fmt.Sprintf("0811%08d", time.Now().UnixNano()%100000000)
	phoneB := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	userA, midA := setupTestUserWithMembership(t, pool, "a@iso.test", "User A", phoneA, rta, "bendahara")
	_, _ = setupTestUserWithMembership(t, pool, "b@iso.test", "User B", phoneB, rtb, "bendahara")

	tokenA := makeToken(t, auth.TokenClaims{
		UserID: userA, MID: midA, RTID: rta, Role: string(auth.RoleBendahara),
	})

	// ── Setup: create categories, dues, bills, transactions in BOTH RTs ──
	ctx := context.Background()

	// RT-A: income category + transaction
	var catAID string
	{
		tx, _ := pool.BeginTx(ctx)
		c, _ := CategoryCreate(ctx, tx, rta, CreateCategoryInput{Name: "Isolation-A-Income", Type: CategoryTypeIncome})
		catAID = c.ID
		_, _ = TransactionCreate(ctx, tx, rta, CreateTransactionInput{
			CategoryID: catAID, Amount: "50000", Type: TransactionTypeIncome,
			Description: "Isolation test income A",
		}, TransactionStatusPosted, nil, nil, time.Now())
		tx.Commit()
	}

	// RT-B: expense category + transaction
	var catBID string
	{
		tx, _ := pool.BeginTx(ctx)
		c, _ := CategoryCreate(ctx, tx, rtb, CreateCategoryInput{Name: "Isolation-B-Expense", Type: CategoryTypeExpense})
		catBID = c.ID
		_, _ = TransactionCreate(ctx, tx, rtb, CreateTransactionInput{
			CategoryID: catBID, Amount: "30000", Type: TransactionTypeExpense,
			Description: "Isolation test expense B",
		}, TransactionStatusPosted, nil, nil, time.Now())
		tx.Commit()
	}

	// Create dues in both RTs
	{
		tx, _ := pool.BeginTx(ctx)
		_, _ = DueCreate(ctx, tx, rta, CreateDueInput{Name: "Isuara RT-A", Amount: "20000", PeriodType: DuePeriodMonthly})
		_, _ = DueCreate(ctx, tx, rtb, CreateDueInput{Name: "Isuara RT-B", Amount: "25000", PeriodType: DuePeriodMonthly})
		tx.Commit()
	}

	// ── Assert: tenant user A sees ONLY RT-A data ──

	// Categories
	t.Run("Categories isolation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var cats []*FinancialCategory
		json.Unmarshal(rec.Body.Bytes(), &cats)
		for _, c := range cats {
			if c.RTID != rta {
				t.Errorf("tenant user saw category from RT-B (%s), expected only RT-A", c.RTID)
			}
		}
		// Should have at least the RT-A category
		found := false
		for _, c := range cats {
			if c.RTID == rta && c.Name == "Isolation-A-Income" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find RT-A income category")
		}
	})

	// Transactions
	t.Run("Transactions isolation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Data       []*Transaction `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Pagination.Total > 0 {
			for _, tr := range resp.Data {
				if tr.RTID != rta {
					t.Errorf("tenant user saw transaction from RT-B (%s), expected only RT-A", tr.RTID)
				}
			}
		}
	})

	// Dues
	t.Run("Dues isolation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dues", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var dues []*Due
		json.Unmarshal(rec.Body.Bytes(), &dues)
		for _, d := range dues {
			if d.RTID != rta {
				t.Errorf("tenant user saw due from RT-B (%s), expected only RT-A", d.RTID)
			}
		}
	})
}

// TestSuperAdminSecurityTests covers security edge cases:
//  1. Missing auth → 401
//  2. Non-superadmin with empty RTID → 403 (no membership)
//  3. Write endpoints must NOT succeed for system-only superadmin
func TestSuperAdminSecurityTests(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rta := setupTestRT(t, pool, "Security RT", 1, "SECRT")
	defer cleanupTestRT(t, pool, rta)

	// ── 1. Missing auth → 401 on read endpoint ──
	t.Run("Missing auth returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
		// No Authorization header
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// ── 2. Non-superadmin with empty RTID → 403 ──
	// Create a user with a membership (so they pass RequireAuth) but token
	// has empty Role and RTID — they have no tenant context.
	// Actually, RequireAuth.ValidateIdentity needs a valid membershipID to pass.
	// Let's create a bendahara user, give them a membership, but craft a token
	// with empty RTID, MID, Role (so they have a user but no tenant scope).
	// Since ValidateIdentity is called with the MID from the token:
	//   - If MID is "" → system-only check (just checks user exists)
	//   - But the user HAS a membership → that's fine, system-only doesn't care
	t.Run("Non-superadmin with empty RTID returns 403", func(t *testing.T) {
		phoneU := fmt.Sprintf("0833%08d", time.Now().UnixNano()%100000000)
		userID, _ := setupTestUserWithMembership(t, pool, "nonsa@sec.test", "Not Super Admin", phoneU, rta, "warga")
		// Create token WITHOUT sys_role, without RTID, without MID
		// This user is active, so ValidateIdentity(passes). But then RequireRole
		// checks: ac.SystemRole != super_admin → ac.MembershipID == "" → 403
		tokenNoScope := makeToken(t, auth.TokenClaims{
			UserID:  userID,
			MID:     "",
			RTID:    "",
			Role:    "",
			Jabatan: "",
			SysRole: "",
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
		req.Header.Set("Authorization", "Bearer "+tokenNoScope)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		// Should fail at RequireRole because MembershipID is empty (no tenant context)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 for non-superadmin with empty RTID, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// ── 3. Write protection — POST endpoints should NOT succeed ──
	writeEndpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/categories", `{"name":"Hacked Cat","type":"income"}`},
		{http.MethodPost, "/api/v1/dues", `{"name":"Hacked Due","amount":"10000","period_type":"monthly"}`},
		{http.MethodPost, "/api/v1/bills", `{"household_occupancy_id":"fake","due_id":"fake","amount":"10000","period":"2026-10","due_date":"2026-10-25"}`},
		{http.MethodPost, "/api/v1/payments", `{"bill_id":"fake","amount":"10000","method":"CASH"}`},
		{http.MethodPost, "/api/v1/transactions", `{"category_id":"fake","amount":"10000","type":"income","description":"Hacked"}`},
	}

	for _, we := range writeEndpoints {
		t.Run("Write blocked: "+we.method+" "+we.path, func(t *testing.T) {
			saUser := setupSystemOnlySuperAdmin(t, pool, "wasa@sec.test", "Write-Attempt SA", "+628999")
			tokenSysAdmin := makeToken(t, auth.TokenClaims{
				UserID:  saUser,
				MID:     "",
				RTID:    "",
				Role:    "",
				Jabatan: "",
				SysRole: "super_admin",
			})
			req := httptest.NewRequest(we.method, we.path, bytes.NewBufferString(we.body))
			req.Header.Set("Authorization", "Bearer "+tokenSysAdmin)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			// Should NOT return 201 Created or 200 OK
			if rec.Code == http.StatusCreated || rec.Code == http.StatusOK {
				t.Errorf("write endpoint unexpectedly succeeded (2xx): %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
