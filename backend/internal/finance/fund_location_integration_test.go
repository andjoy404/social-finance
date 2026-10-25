package finance_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	fundlocation "social-finance/internal/fund_location"
	"social-finance/internal/testutil"
)

func setupFinanceTest(t *testing.T) (*database.Pool, *finance.Service, *fundlocation.Service, string, string, string) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	finSvc := finance.NewService(pool)
	locRepo := fundlocation.NewRepository(pool.Raw())
	locSvc := fundlocation.NewService(pool, locRepo)

	ctx := context.Background()
	var rtID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ('RT FinTest', 1, $1, true) RETURNING id`,
		fmt.Sprintf("001-%d", time.Now().UnixNano()),
	).Scan(&rtID)
	require.NoError(t, err)

	var uID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'hash', 'Treasurer', true) RETURNING id`,
		fmt.Sprintf("t-%d@test.com", time.Now().UnixNano()),
		fmt.Sprintf("081%d", time.Now().UnixNano()%100000000),
	).Scan(&uID)
	require.NoError(t, err)

	var vID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'hash', 'Verifier', true) RETURNING id`,
		fmt.Sprintf("v-%d@test.com", time.Now().UnixNano()),
		fmt.Sprintf("089%d", time.Now().UnixNano()%100000000),
	).Scan(&vID)
	require.NoError(t, err)

	return pool, finSvc, locSvc, rtID, uID, vID
}

func TestPostedTransactionImmutability(t *testing.T) {
	pool, finSvc, _, rtID, uID, _ := setupFinanceTest(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	// 1. Create a manual posted transaction
	trans, err := finSvc.CreateManualTransaction(ctx, tx, rtID, uID, finance.CreateTransactionInput{
		Amount:      "150000.00",
		Type:        finance.TransactionTypeIncome,
		Description: "Iuran warga manual",
	})
	require.NoError(t, err)
	require.Equal(t, finance.TransactionStatusPosted, trans.Status)

	// 2. Attempting to update the posted transaction must fail with ErrTransactionPosted
	newDesc := "Deskripsi baru"
	_, err = finSvc.UpdateManualTransaction(ctx, tx, rtID, uID, trans.ID, finance.UpdateTransactionInput{
		Description: &newDesc,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, finance.ErrTransactionPosted)
}

func TestVerifyPaymentGAPBAndReversePaymentGAPD(t *testing.T) {
	pool, finSvc, locSvc, rtID, uID, vID := setupFinanceTest(t)
	ctx := context.Background()

	// Create locations
	bank := "BCA"
	rekening, err := locSvc.CreateLocation(ctx, rtID, uID, fundlocation.CreateFundLocationInput{
		Name:     "BCA Operasional",
		Category: fundlocation.CategoryRekening,
		BankName: &bank,
	})
	require.NoError(t, err)

	brankas, err := locSvc.CreateLocation(ctx, rtID, uID, fundlocation.CreateFundLocationInput{
		Name:     "Brankas Kasir",
		Category: fundlocation.CategoryBrankas,
	})
	require.NoError(t, err)

	// Create due & bill
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	due, err := finSvc.CreateDue(ctx, tx, rtID, uID, finance.CreateDueInput{
		Name:       "Iuran Keamanan",
		Amount:     "100000.00",
		PeriodType: "monthly",
	})
	require.NoError(t, err)

	// Create household occupancy & link warga uID
	var houseID, hhID, occID, resID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, address) VALUES ($1, 'A-10', 'Jl Merak') RETURNING id`,
		rtID,
	).Scan(&houseID)
	require.NoError(t, err)

	err = tx.QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Test Head') RETURNING id`,
		rtID,
	).Scan(&hhID)
	require.NoError(t, err)

	err = tx.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', CURRENT_DATE) RETURNING id`,
		hhID, houseID,
	).Scan(&occID)
	require.NoError(t, err)

	err = tx.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, user_id, full_name, is_active)
		 VALUES ($1, $2, 'Resident Warga', true) RETURNING id`,
		rtID, uID,
	).Scan(&resID)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, CURRENT_DATE)`,
		resID, occID,
	)
	require.NoError(t, err)

	bill, err := finSvc.CreateBill(ctx, tx, rtID, vID, finance.CreateBillInput{
		HouseholdOccupancyID: occID,
		DueID:                due.ID,
		Period:               "2026-10",
		Amount:               "100000.00",
		DueDate:              "2026-10-31",
	})
	require.NoError(t, err)

	// 1. Submit payment with CASH as warga uID
	cashPayment, err := finSvc.CreatePayment(ctx, tx, rtID, uID, "warga", finance.CreatePaymentInput{
		BillID: &bill.ID,
		Amount: "100000.00",
		Method: finance.PaymentMethodCash,
	})
	require.NoError(t, err)

	// GAP B: Attempting to approve CASH payment into rekening must FAIL
	approveAction := "approve"
	_, err = finSvc.VerifyPayment(ctx, tx, rtID, vID, cashPayment.ID, finance.VerifyPaymentInput{
		Action:     approveAction,
		LocationID: &rekening.ID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "metode tunai (CASH) hanya dapat disetorkan ke Brankas atau COH")

	// GAP B: Approving CASH payment into brankas must SUCCEED
	res, err := finSvc.VerifyPayment(ctx, tx, rtID, vID, cashPayment.ID, finance.VerifyPaymentInput{
		Action:     approveAction,
		LocationID: &brankas.ID,
	})
	require.NoError(t, err)
	require.Equal(t, finance.PaymentStatusApproved, res.Payment.Status)

	// Check bill status is paid
	updatedBill, err := finance.BillGetByID(ctx, tx, bill.ID, rtID)
	require.NoError(t, err)
	require.Equal(t, finance.BillStatusPaid, updatedBill.Status)

	// Check brankas balance is 100,000
	var brankasBal string
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_delta), 0)::text FROM location_balance_changes WHERE location_id = $1`,
		brankas.ID,
	).Scan(&brankasBal)
	require.NoError(t, err)
	require.Equal(t, "100000.00", brankasBal)

	// Commit verification
	require.NoError(t, tx.Commit())

	// 2. GAP D: Reversal of approved payment
	txRev, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer txRev.Rollback()

	revTx, err := finSvc.ReverseApprovedPayment(ctx, txRev, rtID, vID, cashPayment.ID, "Salah verifikasi warga")
	require.NoError(t, err)
	require.NotEmpty(t, revTx.ID)
	require.Equal(t, finance.TransactionTypeExpense, revTx.Type)

	// Check payment status is now REVERSED
	revPayment, err := finance.PaymentGetByID(ctx, txRev, cashPayment.ID, rtID)
	require.NoError(t, err)
	require.Equal(t, finance.PaymentStatusReversed, revPayment.Status)

	// Check bill status reverted to UNPAID
	revertedBill, err := finance.BillGetByID(ctx, txRev, bill.ID, rtID)
	require.NoError(t, err)
	require.Equal(t, finance.BillStatusUnpaid, revertedBill.Status)

	// Check brankas balance reverted to 0.00
	err = txRev.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_delta), 0)::text FROM location_balance_changes WHERE location_id = $1`,
		brankas.ID,
	).Scan(&brankasBal)
	require.NoError(t, err)
	_, canonBal, _ := finance.ParseMoney(brankasBal)
	require.Equal(t, "0.00", canonBal)

	// Cannot reverse again
	_, err = finSvc.ReverseApprovedPayment(ctx, txRev, rtID, vID, cashPayment.ID, "Double reverse")
	require.Error(t, err)
}
