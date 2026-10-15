package finance

import (
	"context"
	"testing"

	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

// testRepo returns the test database pool for repository-level tests.
func testRepo(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}

// --- SubmitterName Regression Tests ---

func testSetup(t *testing.T, pool *database.Pool, ctx context.Context, rtLabel string) (rtID, occID string) {
	t.Helper()
	rtID = setupTestRT(t, pool, rtLabel, 1, rtLabel)
	occID = setupTestOccupancy(t, pool, rtID)
	return
}

// TestSubmitterName_TwoUsersCorrectNames verifies that two different users
// produce two different, correct submitter_name values.
func TestSubmitterName_TwoUsersCorrectNames(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "TwoUsers")
	defer cleanupTestRT(t, pool, rtID)

	_, _ = setupTestUserWithMembership(t, pool, "alice@test.local", "Alice Saputra", "+6281111111", rtID, "warga")
	userBob, _ := setupTestUserWithMembership(t, pool, "bob@test.local", "Budi Santoso", "+6282222222", rtID, "bendahara")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userBob, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}
	if p.SubmitterName == nil {
		t.Fatalf("expected SubmitterName not nil, got nil")
	}
	if *p.SubmitterName != "Budi Santoso" {
		t.Errorf("expected 'Budi Santoso', got %q", *p.SubmitterName)
	}

	// Verify via PaymentGetByID
	p2, err := PaymentGetByID(ctx, tx, p.ID, rtID)
	if err != nil {
		t.Fatalf("PaymentGetByID: %v", err)
	}
	if p2.SubmitterName == nil || *p2.SubmitterName != "Budi Santoso" {
		t.Errorf("PaymentGetByID: expected 'Budi Santoso', got %v", p2.SubmitterName)
	}
	tx.Commit()
}

// TestSubmitterName_EmptyOrNullUserNamesFallback verifies that an empty or NULL
// full_name in the users table produces the fallback '—'.
func TestSubmitterName_EmptyOrNullUserNamesFallback(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "NullNames")
	defer cleanupTestRT(t, pool, rtID)

	// Create user with empty full_name — setupTestUserWithMembership returns userID (valid UUID)
	emptyUserID, _ := setupTestUserWithMembership(t, pool, "empty@test.local", "", "+6281111111", rtID, "warga")
	// Also create a normal user so we can confirm they work too
	normalUserID, _ := setupTestUserWithMembership(t, pool, "normal@test.local", "Normal User", "+6282222222", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	// Payment by user with empty full_name → NULLIF('', '') = NULL → COALESCE → '—'
	inEmpty := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &emptyUserID, Amount: "10000", Method: PaymentMethodTransfer}
	pEmpty, err := PaymentCreate(ctx, tx, rtID, inEmpty, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate empty full_name: %v", err)
	}
	pEmpty, err = PaymentGetByID(ctx, tx, pEmpty.ID, rtID)
	if err != nil {
		t.Fatalf("PaymentGetByID: %v", err)
	}
	if pEmpty.SubmitterName == nil || *pEmpty.SubmitterName != "—" {
		t.Errorf("expected fallback '—' for empty full_name, got %v", pEmpty.SubmitterName)
	}

	// Payment by user with normal name → should get real name
	inNormal := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &normalUserID, Amount: "10000", Method: PaymentMethodTransfer}
	pNormal, _ := PaymentCreate(ctx, tx, rtID, inNormal, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	pNormal, _ = PaymentGetByID(ctx, tx, pNormal.ID, rtID)
	if pNormal.SubmitterName == nil || *pNormal.SubmitterName != "Normal User" {
		t.Errorf("expected 'Normal User', got %v", pNormal.SubmitterName)
	}
	tx.Commit()
}// TestPaymentCreate_SubmitterName verifies PaymentCreate returns correct SubmitterName.
func TestPaymentCreate_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "CreateSN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "create@test.local", "Create User", "+6281111111", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}
	if p.SubmitterName == nil {
		t.Fatalf("expected SubmitterName not nil, got nil")
	}
	if *p.SubmitterName != "Create User" {
		t.Errorf("expected 'Create User', got %q", *p.SubmitterName)
	}
	tx.Commit()
}

// TestPaymentGetByID_SubmitterName verifies PaymentGetByID returns correct SubmitterName.
func TestPaymentGetByID_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "GetByIDSN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "getbyid@test.local", "GetByID User", "+6281111111", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}

	p2, err := PaymentGetByID(ctx, tx, p.ID, rtID)
	if err != nil {
		t.Fatalf("PaymentGetByID: %v", err)
	}
	if p2.SubmitterName == nil || *p2.SubmitterName != "GetByID User" {
		t.Errorf("expected 'GetByID User', got %v", p2.SubmitterName)
	}
	tx.Commit()
}

// TestPaymentGetByIDForUpdate_SubmitterName verifies PaymentGetByIDForUpdate returns correct SubmitterName.
func TestPaymentGetByIDForUpdate_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "ForUpdateSN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "forupdate@test.local", "ForUpdate User", "+6281111111", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}

	p2, err := PaymentGetByIDForUpdate(ctx, tx, p.ID, rtID)
	if err != nil {
		t.Fatalf("PaymentGetByIDForUpdate: %v", err)
	}
	if p2.SubmitterName == nil || *p2.SubmitterName != "ForUpdate User" {
		t.Errorf("expected 'ForUpdate User', got %v", p2.SubmitterName)
	}
	tx.Commit()
}

// TestPaymentList_SubmitterName verifies PaymentList returns correct SubmitterName.
func TestPaymentList_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "ListSN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "list@test.local", "List User", "+6281111111", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}

	list, err := PaymentList(ctx, tx, rtID, nil, nil, nil, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("PaymentList: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one payment in list")
	}
	found := false
	for _, item := range list {
		if item.ID == p.ID {
			found = true
			if item.SubmitterName == nil {
				t.Fatal("expected SubmitterName not nil in list")
			}
			if *item.SubmitterName != "List User" {
				t.Errorf("expected 'List User' in list, got %q", *item.SubmitterName)
			}
			break
		}
	}
	if !found {
		t.Error("created payment not found in PaymentList")
	}
	tx.Commit()
}

// TestPaymentVerify_SubmitterName verifies PaymentVerify returns correct SubmitterName.
func TestPaymentVerify_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "VerifySN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "verify@test.local", "Verify User", "+6281111111", rtID, "warga")
	bendaharaID, _ := setupTestUserWithMembership(t, pool, "ben@test.local", "Bendahara Name", "+6282222222", rtID, "bendahara")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}

	verified, err := PaymentVerify(ctx, tx, p.ID, rtID, bendaharaID, PaymentStatusApproved, "", "")
	if err != nil {
		t.Fatalf("PaymentVerify: %v", err)
	}
	if verified.SubmitterName == nil {
		t.Fatalf("expected SubmitterName not nil after verify")
	}
	if *verified.SubmitterName != "Verify User" {
		t.Errorf("expected 'Verify User' after verify, got %q", *verified.SubmitterName)
	}
	tx.Commit()
}

// TestPaymentAttachProof_SubmitterName verifies PaymentAttachProof returns correct SubmitterName.
func TestPaymentAttachProof_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "AttachProofSN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "proof@test.local", "Proof User", "+6281111111", rtID, "warga")
	staffID, _ := setupTestUserWithMembership(t, pool, "staff@test.local", "Staff Recorder", "+6282222222", rtID, "bendahara")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}

	proofPath := "/uploads/test.jpg"
	p2, err := PaymentAttachProof(ctx, tx, p.ID, rtID, proofPath, "test.jpg", "image/jpeg", 1024, staffID)
	if err != nil {
		t.Fatalf("PaymentAttachProof: %v", err)
	}
	if p2.SubmitterName == nil {
		t.Fatalf("expected SubmitterName not nil after attach proof")
	}
	if *p2.SubmitterName != "Proof User" {
		t.Errorf("expected 'Proof User' after attach proof, got %q", *p2.SubmitterName)
	}
	tx.Commit()
}

// TestPaymentGetByIDAll_SubmitterName verifies PaymentGetByIDAll returns correct SubmitterName.
func TestPaymentGetByIDAll_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "ByIDAllSN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "all@test.local", "All User", "+6281111111", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}

	p2, err := PaymentGetByIDAll(ctx, tx, p.ID)
	if err != nil {
		t.Fatalf("PaymentGetByIDAll: %v", err)
	}
	if p2.SubmitterName == nil {
		t.Fatalf("expected SubmitterName not nil in GetByIDAll")
	}
	if *p2.SubmitterName != "All User" {
		t.Errorf("expected 'All User' in GetByIDAll, got %q", *p2.SubmitterName)
	}
	tx.Commit()
}

// TestPaymentListAll_SubmitterName verifies PaymentListAll returns correct SubmitterName.
func TestPaymentListAll_SubmitterName(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "ListAllSN")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "alllist@test.local", "AllList User", "+6281111111", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	pay, _ := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)

	list, err := PaymentListAll(ctx, tx, nil, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("PaymentListAll: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one payment in PaymentListAll")
	}
	found := false
	for _, item := range list {
		if item.ID == pay.ID {
			found = true
			if item.SubmitterName == nil {
				t.Fatal("expected SubmitterName not nil in PaymentListAll")
			}
			if *item.SubmitterName != "AllList User" {
				t.Errorf("expected 'AllList User' in PaymentListAll, got %q", *item.SubmitterName)
			}
			break
		}
	}
	if !found {
		t.Error("created payment not found in PaymentListAll")
	}
	tx.Commit()
}

// TestPaymentBillPaymentIdentityCorrect verifies that a regular bill payment
// maintains correct identity (BillID set, status transitions work, SubmitterName preserved).
func TestPaymentBillPaymentIdentityCorrect(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "BillIdentity")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "billpay@test.local", "Bill Pay User", "+6281111111", rtID, "warga")
	bendaharaID, _ := setupTestUserWithMembership(t, pool, "benbill@test.local", "Bendahara Bill", "+6282222222", rtID, "bendahara")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate: %v", err)
	}

	in := CreatePaymentInput{BillID: &bill.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	p, err := PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)
	if err != nil {
		t.Fatalf("PaymentCreate: %v", err)
	}
	if p.BillID == nil || *p.BillID != bill.ID {
		t.Errorf("expected bill_id=%s, got %v", bill.ID, p.BillID)
	}
	if p.Status != PaymentStatusPending {
		t.Errorf("expected status PENDING, got %s", p.Status)
	}

	verified, err := PaymentVerify(ctx, tx, p.ID, rtID, bendaharaID, PaymentStatusApproved, "", "")
	if err != nil {
		t.Fatalf("PaymentVerify: %v", err)
	}
	if verified.Status != PaymentStatusApproved {
		t.Errorf("expected APPROVED, got %s", verified.Status)
	}
	if verified.BillID == nil || *verified.BillID != bill.ID {
		t.Errorf("expected bill_id unchanged after verify, got %v", verified.BillID)
	}
	if verified.SubmitterName == nil || *verified.SubmitterName != "Bill Pay User" {
		t.Errorf("expected 'Bill Pay User', got %v", verified.SubmitterName)
	}
	tx.Commit()
}

// TestPaymentJoinNoDuplicates verifies that LEFT JOIN users does not duplicate rows.
func TestPaymentJoinNoDuplicates(t *testing.T) {
	pool := testRepo(t)
	ctx := context.Background()
	rtID, occID := testSetup(t, pool, ctx, "NoDup")
	defer cleanupTestRT(t, pool, rtID)

	userID, _ := setupTestUserWithMembership(t, pool, "nodup@test.local", "No Dup User", "+6281111111", rtID, "warga")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	due, err := DueCreate(ctx, tx, rtID, CreateDueInput{Name: "Iuran RT", Amount: "10000", PeriodType: DuePeriodMonthly, PaymentRule: PaymentRuleFull})
	if err != nil {
		t.Fatalf("DueCreate: %v", err)
	}
	bill1, err := BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-10", DueDate: "2026-10-25"})
	if err != nil {
		t.Fatalf("BillCreate bill1: %v", err)
	}
	_, _ = BillCreate(ctx, tx, rtID, CreateBillInput{HouseholdOccupancyID: occID, DueID: due.ID, Amount: "10000", Period: "2026-11", DueDate: "2026-11-25"})

	// Create a single payment
	in := CreatePaymentInput{BillID: &bill1.ID, SubmittedBy: &userID, Amount: "10000", Method: PaymentMethodTransfer}
	_, _ = PaymentCreate(ctx, tx, rtID, in, PaymentOriginSelfSubmitted, PaymentStatusPending, nil, nil)

	// PaymentListAll with billID filter should return exactly 1 payment for this bill,
	// proving LEFT JOIN users does not duplicate rows even with stale data in DB.
	billID := bill1.ID
	list, err := PaymentListAll(ctx, tx, &billID, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("PaymentListAll: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected exactly 1 payment, got %d", len(list))
	}
	if len(list) > 0 && list[0].SubmitterName == nil {
		t.Fatal("expected SubmitterName not nil")
	}
	if len(list) > 0 && list[0].SubmitterName != nil && *list[0].SubmitterName != "No Dup User" {
		t.Errorf("expected 'No Dup User', got %q", *list[0].SubmitterName)
	}
	tx.Commit()
}
