package finance

import (
	"context"
	"sync"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/storage"
)

// ── helpers ──────────────────────────────────────────────────────────────

// createBillsForFIFO creates two bills for the same (occID, dueID) — oldest
// at period 2026-01 and newer at 2026-02.  Returns (oldestID, newerID).
// Uses payment_rule = 'partial' so these dues also support partial payments.
func createBillsForFIFO(t *testing.T, p *database.Pool, rtID, occID, dueID string) (string, string) {
	t.Helper()
	var oldestID string
	err := p.Raw().QueryRowContext(context.Background(),
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date)
		 VALUES ($1, $2, $3, '2026-01', '50000.00', 'unpaid', '2026-01-15') RETURNING id`,
		rtID, occID, dueID,
	).Scan(&oldestID)
	if err != nil {
		t.Fatalf("create oldest bill: %v", err)
	}
	var newerID string
	err = p.Raw().QueryRowContext(context.Background(),
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date)
		 VALUES ($1, $2, $3, '2026-02', '50000.00', 'unpaid', '2026-02-15') RETURNING id`,
		rtID, occID, dueID,
	).Scan(&newerID)
	if err != nil {
		t.Fatalf("create newer bill: %v", err)
	}
	return oldestID, newerID
}

// createPartialDue creates a due with payment_rule = 'partial' so partial
// payments are allowed, then returns its ID.
func createPartialDue(t *testing.T, p *database.Pool, rtID, name, amount string) string {
	t.Helper()
	var dueID string
	err := p.Raw().QueryRowContext(context.Background(),
		`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, is_active)
		 VALUES ($1, $2, $3, 'monthly', 'partial', false, true) RETURNING id`,
		rtID, name, amount,
	).Scan(&dueID)
	if err != nil {
		t.Fatalf("create partial due: %v", err)
	}
	return dueID
}

// createVoluntaryBill creates a voluntary (donation) bill.
func createVoluntaryBill(t *testing.T, p *database.Pool, rtID, occID, dueID string) string {
	t.Helper()
	var billID string
	err := p.Raw().QueryRowContext(context.Background(),
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date)
		 VALUES ($1, $2, $3, '2026-06', '10000.00', 'unpaid', '2026-06-15') RETURNING id`,
		rtID, occID, dueID,
	).Scan(&billID)
	if err != nil {
		t.Fatalf("create voluntary bill: %v", err)
	}
	return billID
}

// approvePayment helper: begin tx → approve payment → commit. Returns bendahara userID.
func approvePayment(t *testing.T, pool *database.Pool, svc *Service, rtID, paymentID, bendaharaEmail, bendaharaName, bendaharaPhone string, ctx context.Context) {
	t.Helper()
	bendaharaID, _ := setupTestUserWithMembership(t, pool, bendaharaEmail, bendaharaName, bendaharaPhone, rtID, "bendahara")
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	verifyIn := VerifyPaymentInput{Action: "approve"}
	_, err = svc.VerifyPayment(ctx, tx, rtID, bendaharaID, paymentID, verifyIn)
	if err != nil {
		tx.Rollback()
		t.Fatalf("approve: %v", err)
	}
	if err := tx.Commit(); err != nil {
		tx.Rollback()
		t.Fatalf("commit approve: %v", err)
	}
}

// ── Tests ────────────────────────────────────────────────────────────────

// 1. Paying the oldest unpaid bill succeeds.
func TestFIFO_OldestBillPaymentSucceeds(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Oldest", 1, "FO")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6233333333333"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.oldest", "Fargo", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Amanah", "50000.00", false)
	oldestID, _ := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	billIDStr := oldestID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	payment, err := svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create payment on oldest bill: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if payment.Status != PaymentStatusPending {
		t.Fatalf("expected PENDING, got %s", payment.Status)
	}
}

// 2. Paying a later bill while an older bill is unpaid is rejected.
func TestFIFO_LaterBillRejectedWhenOlderHasBalance(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Later Reject", 1, "FL")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6244444444444"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.reject", "Flare", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Pelangi", "50000.00", false)
	_, newerID := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	billIDStr := newerID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err == nil {
		tx.Rollback()
		t.Fatal("expected FIFO rejection for newer bill, got nil")
	}
	expected := "tagihan sebelumnya belum lunas — pembayaran harus mengikuti urutan FIFO"
	if err.Error() != expected {
		tx.Rollback()
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
	tx.Rollback()
}

// 3. Partial payment of the oldest bill is allowed.
func TestFIFO_PartialPaymentOfOldestAllowed(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Partial", 1, "FP")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6255555555555"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.partial", "Fudge", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createPartialDue(t, pool, rtID, "Iuran Cemerlang", "50000.00")
	oldestID, _ := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	billIDStr := oldestID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "30000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	payment, err := svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create partial payment: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if payment.Status != PaymentStatusPending {
		t.Fatalf("expected PENDING, got %s", payment.Status)
	}
}

// 4. After a partial payment on the oldest bill, the next bill is still blocked.
func TestFIFO_PartialDoesNotUnlockNextBill(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO PartialNoUnlock", 1, "FN")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6266666666666"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.nounlock", "Frost", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createPartialDue(t, pool, rtID, "Iuran Sejati", "50000.00")
	oldestID, newerID := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Step 1: partial payment on oldest
	billIDStr := oldestID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "30000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create partial: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit partial: %v", err)
	}

	// Step 2: try to pay newer bill — should FAIL
	billIDStr = newerID
	in.BillID = &billIDStr
	in.Amount = "50000.00"

	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx2, rtID, userID, "warga", in)
	if err == nil {
		tx2.Rollback()
		t.Fatal("expected FIFO rejection after partial, got nil")
	}
	expected := "tagihan sebelumnya belum lunas — pembayaran harus mengikuti urutan FIFO"
	if err.Error() != expected {
		tx2.Rollback()
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
	tx2.Rollback()
}

// 5. After the oldest bill is fully paid, the next bill becomes payable.
func TestFIFO_NextBillPayableAfterOldestFullyPaid(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Unlock", 1, "FU")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6277777777777"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.unlock", "Flame", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Mantap", "50000.00", false)
	oldestID, newerID := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Step 1: pay oldest bill fully
	billIDStr := oldestID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	payment, err := svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create oldest payment: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Approve it
	approvePayment(t, pool, svc, rtID, payment.ID,
		"bendahara@fifo.unlock", "Ben", "+6288888888888", ctx)

	// Step 2: pay newer bill — should SUCCEED now
	billIDStr = newerID
	in.BillID = &billIDStr
	in.Amount = "50000.00"

	tx3, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx3: %v", err)
	}
	payment2, err := svc.CreatePayment(ctx, tx3, rtID, userID, "warga", in)
	if err != nil {
		tx3.Rollback()
		t.Fatalf("create newer bill payment after oldest paid: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("commit newer: %v", err)
	}
	if payment2.Status != PaymentStatusPending {
		t.Fatalf("expected PENDING, got %s", payment2.Status)
	}
}

// 6. FIFO is isolated per due_id — paying a later bill of due B is not
//    blocked by an unpaid earlier bill of due A.
func TestFIFO_IsolationPerDueID(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO DueIso", 1, "FDI")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6299999999999"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.isolate", "Flex", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID1 := createTestDue(t, pool, rtID, "Iuran A", "50000.00", false)
	dueID2 := createTestDue(t, pool, rtID, "Iuran B", "50000.00", false)

	// Due A: 2026-01 unpaid (oldest for dueID1)
	createTestBill(t, pool, rtID, occID, dueID1, "2026-01", "50000.00", "unpaid")

	// Due B: 2026-02 unpaid — this is the ONLY bill for dueID2, so it is the "oldest"
	createTestBill(t, pool, rtID, occID, dueID2, "2026-02", "50000.00", "unpaid")

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Pay dueID2 (period 2026-02) — should SUCCEED because it is the only
	// unpaid bill for dueID2; dueID1's unpaid bill does not block it.
	var due2BillID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM bills WHERE rt_id = $1 AND household_occupancy_id = $2 AND due_id = $3 LIMIT 1`,
		rtID, occID, dueID2,
	).Scan(&due2BillID)
	if err != nil {
		t.Fatalf("get due2 bill: %v", err)
	}

	in := CreatePaymentInput{
		BillID:      &due2BillID,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("paying dueID2 bill should not be blocked by dueID1: %v", err)
	}
	tx.Commit()
}

// 7. FIFO is isolated per occupancy — another household's unpaid bills
//    don't block a different occupancy's bill payment.
func TestFIFO_IsolationPerOccupancy(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO OccIso", 1, "FOI")
	defer cleanupTestRT(t, pool, rtID)

	// Occupancy 1 — user we test with
	phone1 := "+6210000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.occ", "Flint", phone1, rtID, "warga")
	occID1 := setupTestOccupancyWithResident(t, pool, rtID, phone1)

	// Occupancy 2 — separate household
	phone2 := "+6210000000001"
	userID2, _ := setupTestUserWithMembership(t, pool, "warga2@fifo.occ", "Flint2", phone2, rtID, "warga")
	occID2 := setupTestOccupancyWithResident(t, pool, rtID, phone2)

	dueID := createTestDue(t, pool, rtID, "Iuran Bersatu", "50000.00", false)

	// Occupancy 1: both 2026-01 and 2026-02 unpaid
	_, _ = createBillsForFIFO(t, pool, rtID, occID1, dueID)

	// Occupancy 2: both 2026-01 and 2026-02 unpaid
	createBillsForFIFO(t, pool, rtID, occID2, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Pay occID1's NEWER bill — should FAIL because occID1's oldest is unpaid.
	// This confirms FIFO applies within the same occupancy.
	var occ1NewerID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM bills WHERE rt_id = $1 AND household_occupancy_id = $2 AND period = '2026-02' LIMIT 1`,
		rtID, occID1,
	).Scan(&occ1NewerID)
	if err != nil {
		t.Fatalf("get occ1 newer: %v", err)
	}

	in := CreatePaymentInput{
		BillID:      &occ1NewerID,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err == nil {
		tx.Rollback()
		t.Fatal("expected FIFO rejection for newer bill within same occupancy, got nil")
	}
	tx.Rollback()

	// Now pay occID2's OLDER bill as user2 — should SUCCEED.
	// occID2's older is the only bill it can pay, and FIFO does not cross-occupancy.
	var occ2OlderID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM bills WHERE rt_id = $1 AND household_occupancy_id = $2 AND period = '2026-01' LIMIT 1`,
		rtID, occID2,
	).Scan(&occ2OlderID)
	if err != nil {
		t.Fatalf("get occ2 older: %v", err)
	}

	in2 := CreatePaymentInput{
		BillID:      &occ2OlderID,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID2,
	}

	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx2, rtID, userID2, "warga", in2)
	if err != nil {
		tx2.Rollback()
		t.Fatalf("pay occID2 oldest should succeed: %v", err)
	}
	tx2.Commit()
}

// 8. Multi-period payment of multiple bills from the same queue succeeds
//    when oldest bill is included.
func TestFIFO_MultiPeriodPayment(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO MultiPd", 1, "FMP")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6220000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.multipd", "Flick", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Terang", "50000.00", false)
	oldestID, newerID := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Pay both bills at once (oldest included) — should SUCCEED
	in := CreateMultiPeriodPaymentInput{
		BillIDs: []string{oldestID, newerID},
		Allocations: []AllocationEntry{
			{BillID: oldestID, Amount: "50000.00"},
			{BillID: newerID, Amount: "50000.00"},
		},
		Method: PaymentMethodTransfer,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	result, err := svc.CreateMultiPeriodPayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("multi-period FIFO (oldest included): %v", err)
	}
	if result.Payment.Status != PaymentStatusPending {
		t.Fatalf("expected PENDING, got %s", result.Payment.Status)
	}
	tx.Commit()
}

// 9. Multi-period payment without the oldest bill is rejected.
func TestFIFO_MultiPeriodPaymentRejectedOutOfOrder(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO MultiRej", 1, "FMR")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6221000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.mreject", "Flux", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Gelap", "50000.00", false)
	_, newerID := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Multi-period with only the newer bill — should FAIL
	in := CreateMultiPeriodPaymentInput{
		BillIDs: []string{newerID},
		Allocations: []AllocationEntry{
			{BillID: newerID, Amount: "50000.00"},
		},
		Method: PaymentMethodTransfer,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreateMultiPeriodPayment(ctx, tx, rtID, userID, "warga", in)
	if err == nil {
		tx.Rollback()
		t.Fatal("expected FIFO rejection for multi-period skipping oldest, got nil")
	}
	expected := "tagihan sebelumnya belum lunas — pembayaran harus mengikuti urutan FIFO"
	if err.Error() != expected {
		tx.Rollback()
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
	tx.Rollback()
}

// 10. Voluntary donations bypass FIFO even when regular bills are unpaid.
func TestFIFO_VoluntaryBillsBypassFIFO(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Voluntary", 1, "FV")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6222000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.vol", "Flare2", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	// Regular (non-voluntary) due with unpaid bill
	regDueID := createTestDue(t, pool, rtID, "Iuran Wajib", "50000.00", false)
	createTestBill(t, pool, rtID, occID, regDueID, "2026-01", "50000.00", "unpaid")

	// Voluntary due — paying this should NOT be blocked by unpaid regular bill
	volDueID := createTestDue(t, pool, rtID, "Sumbangan Sukarela", "0", true)
	volBillID := createVoluntaryBill(t, pool, rtID, occID, volDueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	billIDStr := volBillID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "10000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("voluntary payment should bypass FIFO: %v", err)
	}
	tx.Commit()
}

// 11. Regular arrears remain payable and subject to FIFO even when dues are inactive.
func TestFIFO_RegularArrearsPayableWhenDuesInactive(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Inactive", 1, "FIX")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6223000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.inactive", "Flicker", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Mati", "50000.00", false)
	_, err := pool.Raw().ExecContext(context.Background(),
		`UPDATE dues SET is_active = false WHERE id = $1`, dueID,
	)
	if err != nil {
		t.Fatalf("deactivate due: %v", err)
	}

	oldestID, newerID := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Pay oldest — should succeed even though due is inactive
	billIDStr := oldestID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("payment on inactive due should still respect FIFO: %v", err)
	}
	tx.Commit()

	// Pay newer — should still be blocked by FIFO
	billIDStr = newerID
	in.BillID = &billIDStr

	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	_, err = svc.CreatePayment(ctx, tx2, rtID, userID, "warga", in)
	if err == nil {
		tx2.Rollback()
		t.Fatal("expected FIFO rejection, got nil")
	}
	tx2.Rollback()
}

// 12. Concurrent requests cannot violate FIFO — exactly one payment succeeds.
func TestFIFO_ConcurrentRequestsCannotViolate(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Conc", 1, "FC")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6224000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.cx", "Conc1", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Konkuren", "50000.00", false)
	oldestID, _ := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	const n = 5
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			billIDStr := oldestID
			in := CreatePaymentInput{
				BillID:      &billIDStr,
				Amount:      "50000.00",
				Method:      PaymentMethodCash,
				SubmittedBy: &userID,
			}
			tx, err := pool.BeginTx(ctx)
			if err != nil {
				return
			}
			_, err = svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
			if err == nil {
				if commitErr := tx.Commit(); commitErr == nil {
					mu.Lock()
					successes++
					mu.Unlock()
				}
			} else {
				tx.Rollback()
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("expected exactly 1 concurrent payment to succeed, got %d", successes)
	}
}

// 13. Self-approval prevention still works alongside FIFO.
func TestFIFO_SelfApprovalStillWorks(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO SAP", 1, "FSA")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6226000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.sap", "Flint3", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID := createTestDue(t, pool, rtID, "Iuran Kombinasi", "50000.00", false)
	oldestID, _ := createBillsForFIFO(t, pool, rtID, occID, dueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	billIDStr := oldestID
	in := CreatePaymentInput{
		BillID:      &billIDStr,
		Amount:      "50000.00",
		Method:      PaymentMethodCash,
		SubmittedBy: &userID,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	payment, err := svc.CreatePayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create payment: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Self-approval should fail
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	verifyIn := VerifyPaymentInput{Action: "approve"}
	_, err = svc.VerifyPayment(ctx, tx2, rtID, userID, payment.ID, verifyIn)
	if err == nil {
		tx2.Rollback()
		t.Fatal("expected self-approval rejection, got nil")
	}
	if err.Error() != "cannot verify your own payment" {
		tx2.Rollback()
		t.Fatalf("expected 'cannot verify your own payment', got %q", err.Error())
	}
	tx2.Rollback()

	// Different verifier can approve
	verifierID, _ := setupTestUserWithMembership(t, pool, "verifier@fifo.sap", "Veri", "+6226100000000", rtID, "bendahara")
	tx3, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx3: %v", err)
	}
	_, err = svc.VerifyPayment(ctx, tx3, rtID, verifierID, payment.ID, verifyIn)
	if err != nil {
		tx3.Rollback()
		t.Fatalf("different verifier approval: %v", err)
	}
	tx3.Commit()
}

// 14. Multi-period voluntary-only bypasses FIFO.
func TestFIFO_MultiPeriodVoluntaryOnly(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO MVol", 1, "FVM")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6227000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.mvol", "Flare3", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	volDueID := createTestDue(t, pool, rtID, "Sumbangan", "0", true)
	volBillID := createVoluntaryBill(t, pool, rtID, occID, volDueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	in := CreateMultiPeriodPaymentInput{
		BillIDs: []string{volBillID},
		Allocations: []AllocationEntry{
			{BillID: volBillID, Amount: "10000.00"},
		},
		Method: PaymentMethodTransfer,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreateMultiPeriodPayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("voluntary-only multi-period should bypass FIFO: %v", err)
	}
	tx.Commit()
}

// 15. Multi-period mixing regular oldest + voluntary succeeds.
func TestFIFO_MultiPeriodMixedVoluntaryAndRegular(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO Mixed", 1, "FMIX")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6229000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.mixed", "Flex2", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	regDueID := createTestDue(t, pool, rtID, "Iuran Campur", "50000.00", false)
	oldestID, _ := createBillsForFIFO(t, pool, rtID, occID, regDueID)

	volDueID := createTestDue(t, pool, rtID, "Sumbangan Campur", "0", true)
	volBillID := createVoluntaryBill(t, pool, rtID, occID, volDueID)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Oldest regular + voluntary — should SUCCEED (oldest regular is first)
	in := CreateMultiPeriodPaymentInput{
		BillIDs: []string{oldestID, volBillID},
		Allocations: []AllocationEntry{
			{BillID: oldestID, Amount: "50000.00"},
			{BillID: volBillID, Amount: "10000.00"},
		},
		Method: PaymentMethodTransfer,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreateMultiPeriodPayment(ctx, tx, rtID, userID, "warga", in)
	if err != nil {
		tx.Rollback()
		t.Fatalf("mixed regular+voluntary multi-period: %v", err)
	}
	tx.Commit()
}

// 16. Multi-period per-occID+dueID: paying newer bill of due A is blocked
//     even though due B has no unpaid bills.
func TestFIFO_MultiPeriodPerOccDueQueue(t *testing.T) {
	signingSecret()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool := testPool(t)
	defer pool.Close()
	auth.AuthDBPool = pool

	rtID := setupTestRT(t, pool, "FIFO MQueue", 1, "FMQ")
	defer cleanupTestRT(t, pool, rtID)

	phone := "+6230000000000"
	userID, _ := setupTestUserWithMembership(t, pool, "warga@fifo.mq", "Fusion", phone, rtID, "warga")
	occID := setupTestOccupancyWithResident(t, pool, rtID, phone)

	dueID1 := createTestDue(t, pool, rtID, "Iuran A", "50000.00", false)
	dueID2 := createTestDue(t, pool, rtID, "Iuran B", "50000.00", false)

	// Both dues have 2026-01 and 2026-02 bills
	_, _ = createBillsForFIFO(t, pool, rtID, occID, dueID1)
	_, _ = createBillsForFIFO(t, pool, rtID, occID, dueID2)

	store := storage.NewMockStorage()
	svc := NewService(pool, store)

	// Try paying only the NEWER bill of dueID1 — should FAIL
	// because dueID1's oldest is unpaid (FIFO is per occID+dueID)
	var newerDue1ID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM bills WHERE rt_id = $1 AND household_occupancy_id = $2 AND due_id = $3 AND period = '2026-02' LIMIT 1`,
		rtID, occID, dueID1,
	).Scan(&newerDue1ID)
	if err != nil {
		t.Fatalf("get newer due1: %v", err)
	}

	in := CreateMultiPeriodPaymentInput{
		BillIDs: []string{newerDue1ID},
		Allocations: []AllocationEntry{
			{BillID: newerDue1ID, Amount: "50000.00"},
		},
		Method: PaymentMethodTransfer,
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = svc.CreateMultiPeriodPayment(ctx, tx, rtID, userID, "warga", in)
	if err == nil {
		tx.Rollback()
		t.Fatal("expected FIFO rejection for newer bill of due1, got nil")
	}
	tx.Rollback()
}
