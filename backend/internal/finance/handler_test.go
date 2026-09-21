package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func setupTestUser(t *testing.T, pool *database.Pool, email, fullName, phone string) string {
	t.Helper()
	db := pool.Raw()
	var id string
	c := atomic.AddInt64(&rtCounter, 1)
	uniqueEmail := fmt.Sprintf("%d_%s", c, email)
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'dummyhash', $3, true) RETURNING id`,
		uniqueEmail, phone, fullName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	return id
}

func setupFinanceRouter(pool *database.Pool) http.Handler {
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

	userP := setupTestUser(t, pool, "p@social.test", "Pengurus P", "+6281111")
	userB := setupTestUser(t, pool, "b@social.test", "Bendahara B", "+6282222")
	userW := setupTestUser(t, pool, "w@social.test", "Warga W", "+6283333")

	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userP,
		MID:    "mid-p",
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})
	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    "mid-b",
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})
	tokenWarga := makeToken(t, auth.TokenClaims{
		UserID: userW,
		MID:    "mid-w",
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

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: "user-b",
		MID:    "mid-b",
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
	userB := setupTestUser(t, pool, "bendahara@test.com", "Bendahara Test", phoneB)

	phoneP := fmt.Sprintf("0822%08d", time.Now().UnixNano()%100000000)
	userP := setupTestUser(t, pool, "pengurus@test.com", "Pengurus Test", phoneP)

	phoneW := fmt.Sprintf("0833%08d", time.Now().UnixNano()%100000000)
	userW := setupTestUser(t, pool, "warga@test.com", "Warga Test", phoneW)

	occID := setupTestOccupancyWithResident(t, pool, rtID, phoneW)

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    "mid-b",
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})
	tokenPengurus := makeToken(t, auth.TokenClaims{
		UserID: userP,
		MID:    "mid-p",
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
			MID:    "mid-w",
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
	userB := setupTestUser(t, pool, "idem_b@test.com", "Bendahara Idem", phoneB)

	tokenBendahara := makeToken(t, auth.TokenClaims{
		UserID: userB,
		MID:    "mid-b",
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

	tokenA := makeToken(t, auth.TokenClaims{
		UserID: "user-a",
		MID:    "mid-a",
		RTID:   rtA,
		Role:   string(auth.RoleBendahara),
	})
	tokenB := makeToken(t, auth.TokenClaims{
		UserID: "user-b",
		MID:    "mid-b",
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
