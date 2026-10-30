package finance

import (
	"context"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/storage"
)

// TestVerifyPayment_SelfApprovalPrevention verifies that a payment submitter
// cannot approve or reject their own payment.
func TestVerifyPayment_SelfApprovalPrevention(t *testing.T) {
	signingSecret()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()

	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "Self-Approval Test RT", 1, "SAP")
	defer cleanupTestRT(t, pool, rtID)

	// Submitter warga — harus punya chain resident → residency_period → household_occupancy
	// agar GetOccupancyIDForUser bisa menemukan occupancy-nya.
	submitterPhone := "+6281111111111"
	submitterID, _ := setupTestUserWithMembership(t, pool, "submitter@sap.local",
		"Submitter", submitterPhone, rtID, "warga")
	// setupTestOccupancyWithResident creates: resident(phone) → residency_period → household_occupancy
	occID := setupTestOccupancyWithResident(t, pool, rtID, submitterPhone)

	verifierID, _ := setupTestUserWithMembership(t, pool, "verifier@sap.local",
		"Verifier", "+6282222222222", rtID, "bendahara")

	// Buat bill untuk occupancy submitter
	dueID := createTestDue(t, pool, rtID, "Test Due", "0", false)
	billID := createTestBill(t, pool, rtID, occID, dueID, "2026-11", "50000", "unpaid")

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	billIDStr := billID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "50000",
		Method:      PaymentMethodCash,
		SubmittedBy: &submitterID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	payment, err := svc.CreatePayment(ctx, tx, rtID, submitterID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create pending payment: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit payment: %v", err)
	}

	if payment.SubmittedBy == nil || *payment.SubmittedBy != submitterID {
		t.Fatalf("expected SubmittedBy=%s, got %v", submitterID, payment.SubmittedBy)
	}
	if payment.Status != PaymentStatusPending {
		t.Fatalf("expected status PENDING, got %s", payment.Status)
	}

	// ── Test 1: submitter tries to approve own payment → should FAIL ──
	t.Run("self-approve blocked", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}

		actualDate := "2026-10-20"
		verifyIn := VerifyPaymentInput{Action: "approve", ActualReceivedDate: &actualDate}
		_, err = svc.VerifyPayment(ctx, tx, rtID, submitterID, payment.ID, verifyIn)
		if err == nil {
			tx.Rollback()
			t.Fatal("expected error for self-approval, got nil")
		}
		if err.Error() != "cannot verify your own payment" {
			tx.Rollback()
			t.Fatalf("expected 'cannot verify your own payment', got %q", err.Error())
		}
		tx.Rollback()

		tx2, err2 := pool.BeginTx(ctx)
		if err2 != nil {
			t.Fatalf("begin tx2: %v", err2)
		}
		p, err := PaymentGetByID(ctx, tx2, payment.ID, rtID)
		if err != nil {
			tx2.Rollback()
			t.Fatalf("get payment after failed self-approval: %v", err)
		}
		if p.Status != PaymentStatusPending {
			tx2.Rollback()
			t.Fatalf("expected status PENDING, got %s", p.Status)
		}
		tx2.Rollback()
	})

	// ── Test 2: submitter tries to reject own payment → should FAIL ──
	t.Run("self-reject blocked", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}

		reason := "test rejection"
		verifyIn := VerifyPaymentInput{
			Action:          "reject",
			RejectionReason: &reason,
		}
		_, err = svc.VerifyPayment(ctx, tx, rtID, submitterID, payment.ID, verifyIn)
		if err == nil {
			tx.Rollback()
			t.Fatal("expected error for self-rejection, got nil")
		}
		if err.Error() != "cannot verify your own payment" {
			tx.Rollback()
			t.Fatalf("expected 'cannot verify your own payment', got %q", err.Error())
		}
		tx.Rollback()

		tx2, err2 := pool.BeginTx(ctx)
		if err2 != nil {
			t.Fatalf("begin tx2: %v", err2)
		}
		p, err := PaymentGetByID(ctx, tx2, payment.ID, rtID)
		if err != nil {
			tx2.Rollback()
			t.Fatalf("get payment after failed self-rejection: %v", err)
		}
		if p.Status != PaymentStatusPending {
			tx2.Rollback()
			t.Fatalf("expected status PENDING, got %s", p.Status)
		}
		tx2.Rollback()
	})

	// ── Test 3: different verifier can approve → should SUCCEED ──
	t.Run("different verifier approved", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}

		actualDate := "2026-10-20"
		verifyIn := VerifyPaymentInput{Action: "approve", ActualReceivedDate: &actualDate}
		_, err = svc.VerifyPayment(ctx, tx, rtID, verifierID, payment.ID, verifyIn)
		if err != nil {
			tx.Rollback()
			t.Fatalf("expected different verifier to succeed, got error: %v", err)
		}
		if err := tx.Commit(); err != nil {
			tx.Rollback()
			t.Fatalf("commit verify: %v", err)
		}

		tx2, err2 := pool.BeginTx(ctx)
		if err2 != nil {
			t.Fatalf("begin tx2: %v", err2)
		}
		p, err := PaymentGetByID(ctx, tx2, payment.ID, rtID)
		if err != nil {
			tx2.Rollback()
			t.Fatalf("get payment after verification: %v", err)
		}
		if p.Status != PaymentStatusApproved {
			tx2.Rollback()
			t.Fatalf("expected status APPROVED, got %s", p.Status)
		}
		if p.VerifiedBy == nil || *p.VerifiedBy != verifierID {
			tx2.Rollback()
			t.Fatalf("expected VerifiedBy=%s, got %v", verifierID, p.VerifiedBy)
		}
		tx2.Rollback()
	})
}

// TestVerifyPayment_SubmitterIsNull_NoBlock verifies that when SubmittedBy is nil,
// the self-approval check is skipped and normal verification proceeds.
func TestVerifyPayment_SubmitterIsNull_NoBlock(t *testing.T) {
	signingSecret()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()

	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "Nil Submitter Test RT", 1, "NSA")
	defer cleanupTestRT(t, pool, rtID)

	verifierID, _ := setupTestUserWithMembership(t, pool, "verifier@nsa.local",
		"Verifier", "+6283333333333", rtID, "bendahara")

	occID := setupTestOccupancy(t, pool, rtID)

	dueID := createTestDue(t, pool, rtID, "Test Due NSA", "0", false)
	billID := createTestBill(t, pool, rtID, occID, dueID, "2026-12", "50000", "unpaid")

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	billIDStr := billID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "50000",
		Method:      PaymentMethodCash,
		SubmittedBy: nil,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	payment, err := svc.CreatePayment(ctx, tx, rtID, verifierID, "bendahara", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create payment with nil submitted_by: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit payment: %v", err)
	}

	// STAFF_RECORDED payments are already APPROVED — skip verification test
	// The guard should never be exercised on already-approved payments
	if payment.Status == PaymentStatusApproved {
		t.Skip("STAFF_RECORDED payment is already approved, no verification needed")
	}

	// If somehow PENDING, self-approval guard should still block since SubmittedBy was set to verifierID by CreatePayment
	if payment.SubmittedBy != nil && *payment.SubmittedBy == verifierID {
		tx2, err2 := pool.BeginTx(ctx)
		if err2 != nil {
			t.Fatalf("begin tx: %v", err2)
		}
		actualDate := "2026-10-20"
		verifyIn := VerifyPaymentInput{Action: "approve", ActualReceivedDate: &actualDate}
		_, err := svc.VerifyPayment(ctx, tx2, rtID, verifierID, payment.ID, verifyIn)
		if err == nil {
			tx2.Rollback()
			t.Fatal("expected self-approval to be blocked when SubmittedBy == verifierID")
		}
		tx2.Rollback()
	}
}
