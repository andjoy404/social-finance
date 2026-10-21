package finance

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/storage"
)

// recordingNotifier captures finance notification calls without touching FCM.
type recordingNotifier struct {
	mu               sync.Mutex
	billCreated      []string // billIDs
	paymentSubmitted []paymentSubmittedCall
	paymentVerified  []paymentVerifiedCall
}

type paymentSubmittedCall struct {
	rtID      string
	billID    string
	paymentID string
}

type paymentVerifiedCall struct {
	rtID            string
	billID          string
	paymentID       string
	submitterUserID string
	status          string
}

func (n *recordingNotifier) NotifyBillCreated(ctx context.Context, rtID, billID string, recipientUserIDs []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.billCreated = append(n.billCreated, billID)
}

func (n *recordingNotifier) NotifyPaymentSubmitted(ctx context.Context, rtID, billID, paymentID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.paymentSubmitted = append(n.paymentSubmitted, paymentSubmittedCall{rtID, billID, paymentID})
}

func (n *recordingNotifier) NotifyPaymentVerified(ctx context.Context, rtID, billID, paymentID, submitterUserID, status, rejectionReason string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.paymentVerified = append(n.paymentVerified, paymentVerifiedCall{rtID, billID, paymentID, submitterUserID, status})
}

// setupOfficerUserWithJabatan creates a user whose membership carries both a
// tenant role and a jabatan (e.g. role=pengurus + jabatan=bendahara).
func setupOfficerUserWithJabatan(t *testing.T, pool *database.Pool, email, fullName, phone, rtID, role, jabatan string) (string, string) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()

	var userID string
	c := rtCounter + time.Now().UnixNano()%1000
	uniqueEmail := fmt.Sprintf("%d_%d_%s", time.Now().UnixNano(), c, email)
	err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'dummyhash', $3, true) RETURNING id`,
		uniqueEmail, phone, fullName,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert officer user: %v", err)
	}

	var membershipID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		 VALUES ($1, $2, $3, $4, true) RETURNING id`,
		userID, rtID, role, jabatan,
	).Scan(&membershipID)
	if err != nil {
		t.Fatalf("insert officer membership: %v", err)
	}

	return userID, membershipID
}

// setupOccupancyWithLinkedResident creates an occupancy whose resident record
// is linked to the given user (residents.user_id), mirroring GetOccupancyIDForUser.
func setupOccupancyWithLinkedResident(t *testing.T, pool *database.Pool, rtID, userID string) string {
	t.Helper()
	occID := setupTestOccupancy(t, pool, rtID)
	db := pool.Raw()
	ctx := context.Background()

	var resID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active, user_id)
		 VALUES ($1, 'Officer Resident', $2, true, $3) RETURNING id`,
		rtID, fmt.Sprintf("+62877%d", time.Now().UnixNano()%100000000), userID,
	).Scan(&resID)
	if err != nil {
		t.Fatalf("create linked resident: %v", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, CURRENT_DATE)`,
		resID, occID,
	)
	if err != nil {
		t.Fatalf("create residency period: %v", err)
	}

	return occID
}

// setupOfficerFinanceRouter builds the payments routes with a notifier wired in
// so handler-level notification dispatch can be asserted without FCM.
func setupOfficerFinanceRouter(pool *database.Pool, notifier *recordingNotifier) http.Handler {
	auth.AuthDBPool = pool

	store := storage.NewMockStorage()
	r := chi.NewRouter()
	finSvc := NewService(pool, store)
	finH := NewHandler(pool, finSvc, store)
	finSvc.SetNotifier(notifier)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Post("/api/v1/payments", finH.CreatePayment)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleBendahara, auth.RolePengurus))
		r.Post("/api/v1/payments/{id}/verify", finH.VerifyPayment)
	})

	return r
}

// TestOfficerOwnPayment_NotificationsAndSelfApprovalGuard verifies that:
//  1. A management officer paying their own household's bill still triggers
//     the payment-submission notification (handler wiring).
//  2. The same officer can never verify their own pending payment, despite
//     holding a management role/jabatan.
//  3. A different authorized verifier can still verify that payment, and the
//     submitter-directed verification-result notification targets the officer.
func TestOfficerOwnPayment_NotificationsAndSelfApprovalGuard(t *testing.T) {
	signingSecret()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()

	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "Officer Own Payment RT", 1, "OOP")
	defer cleanupTestRT(t, pool, rtID)

	// Reported profile: role=pengurus, jabatan=bendahara (finance.approve via
	// position_permissions).
	officerID, officerMid := setupOfficerUserWithJabatan(t, pool,
		"officer@oop.local", "Officer Bendahara", "+628123000001", rtID, "pengurus", "bendahara")

	verifierID, verifierMid := setupTestUserWithMembership(t, pool,
		"verifier@oop.local", "Other Verifier", "+628123000002", rtID, "bendahara")

	occID := setupOccupancyWithLinkedResident(t, pool, rtID, officerID)

	notifier := &recordingNotifier{}
	router := setupOfficerFinanceRouter(pool, notifier)

	officerToken := makeToken(t, auth.TokenClaims{
		UserID: officerID,
		MID:    officerMid,
		RTID:   rtID,
		Role:   string(auth.RolePengurus),
	})
	verifierToken := makeToken(t, auth.TokenClaims{
		UserID: verifierID,
		MID:    verifierMid,
		RTID:   rtID,
		Role:   string(auth.RoleBendahara),
	})

	// ── 1. Officer pays their own household's bill ──
	t.Run("officer pays own bill triggers submission notification", func(t *testing.T) {
		dueID := createTestDue(t, pool, rtID, "OOP Due", "50000", false)
		billID := createTestBill(t, pool, rtID, occID, dueID, "2028-01", "50000", "unpaid")

		body := fmt.Sprintf(`{"bill_id":"%s","amount":"50000","method":"CASH"}`, billID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+officerToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 for officer own payment, got %d: %s", rec.Code, rec.Body.String())
		}

		notifier.mu.Lock()
		submitted := make([]paymentSubmittedCall, len(notifier.paymentSubmitted))
		copy(submitted, notifier.paymentSubmitted)
		notifier.mu.Unlock()

		if len(submitted) != 1 {
			t.Fatalf("expected 1 payment-submitted notification, got %d", len(submitted))
		}
		if submitted[0].rtID != rtID || submitted[0].billID != billID || submitted[0].paymentID == "" {
			t.Fatalf("unexpected submission notification payload: %+v", submitted[0])
		}
	})

	// ── 2 & 3. Pending payment submitted by the officer ──
	dueID2 := createTestDue(t, pool, rtID, "OOP Due 2", "50000", false)
	billID2 := createTestBill(t, pool, rtID, occID, dueID2, "2028-02", "50000", "unpaid")

	var pendingPaymentID string
	{
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		billID2Str := billID2
		in := CreatePaymentInput{
			BillID:      &billID2Str,
			Amount:      "50000",
			Method:      PaymentMethodCash,
			SubmittedBy: &officerID,
		}
		p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
		if err != nil {
			tx.Rollback()
			t.Fatalf("create pending self-submitted payment: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit pending payment: %v", err)
		}
		pendingPaymentID = p.ID
	}

	// ── 2. Officer cannot verify their own payment despite management role ──
	t.Run("officer cannot verify own payment", func(t *testing.T) {
		verifyBody := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+pendingPaymentID+"/verify", bytes.NewBufferString(verifyBody))
		req.Header.Set("Authorization", "Bearer "+officerToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for self-verification, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "cannot verify your own payment") {
			t.Fatalf("expected self-verification error, got: %s", rec.Body.String())
		}

		notifier.mu.Lock()
		verifiedCount := len(notifier.paymentVerified)
		notifier.mu.Unlock()
		if verifiedCount != 0 {
			t.Fatalf("expected no verification-result notification for blocked self-verification, got %d", verifiedCount)
		}

		// Payment must remain PENDING.
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		p, err := PaymentGetByID(ctx, tx, pendingPaymentID, rtID)
		tx.Rollback()
		if err != nil {
			t.Fatalf("get payment after blocked self-verification: %v", err)
		}
		if p.Status != PaymentStatusPending {
			t.Fatalf("expected PENDING after blocked self-verification, got %s", p.Status)
		}
	})

	// ── 3. A different authorized verifier can verify it ──
	t.Run("different verifier approves officer payment", func(t *testing.T) {
		verifyBody := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+pendingPaymentID+"/verify", bytes.NewBufferString(verifyBody))
		req.Header.Set("Authorization", "Bearer "+verifierToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for different verifier, got %d: %s", rec.Code, rec.Body.String())
		}

		notifier.mu.Lock()
		verified := make([]paymentVerifiedCall, len(notifier.paymentVerified))
		copy(verified, notifier.paymentVerified)
		notifier.mu.Unlock()

		if len(verified) != 1 {
			t.Fatalf("expected 1 verification-result notification, got %d", len(verified))
		}
		if verified[0].submitterUserID != officerID {
			t.Fatalf("expected verification-result targeted at officer submitter %s, got %s", officerID, verified[0].submitterUserID)
		}
		if verified[0].status != "APPROVED" {
			t.Fatalf("expected APPROVED status, got %s", verified[0].status)
		}
		if verified[0].paymentID != pendingPaymentID {
			t.Fatalf("expected payment %s, got %s", pendingPaymentID, verified[0].paymentID)
		}
	})
}
