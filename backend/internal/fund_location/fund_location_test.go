package fundlocation_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/database"
	fundlocation "social-finance/internal/fund_location"
	"social-finance/internal/testutil"
)

func setupTest(t *testing.T) (*database.Pool, *fundlocation.Service, *fundlocation.Repository, string, string, string, string) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	repo := fundlocation.NewRepository(pool.Raw())
	svc := fundlocation.NewService(pool, repo)

	// Create test RTs
	ctx := context.Background()
	var rtID, rtID2 string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ('RT Test', 2, $1, true) RETURNING id`,
		fmt.Sprintf("001-%d", time.Now().UnixNano()),
	).Scan(&rtID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ('RT Other', 2, $1, true) RETURNING id`,
		fmt.Sprintf("002-%d", time.Now().UnixNano()),
	).Scan(&rtID2)
	require.NoError(t, err)

	var uID1, uID2 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'hash', 'User 1', true) RETURNING id`,
		fmt.Sprintf("u1-%d@test.com", time.Now().UnixNano()),
		fmt.Sprintf("081%d", time.Now().UnixNano()%100000000),
	).Scan(&uID1)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'hash', 'User 2', true) RETURNING id`,
		fmt.Sprintf("u2-%d@test.com", time.Now().UnixNano()),
		fmt.Sprintf("082%d", time.Now().UnixNano()%100000000),
	).Scan(&uID2)
	require.NoError(t, err)

	return pool, svc, repo, rtID, rtID2, uID1, uID2
}

func TestFundLocationCRUDAndTenantIsolation(t *testing.T) {
	_, svc, _, rt1, rt2, u1, u2 := setupTest(t)
	ctx := context.Background()

	// 1. Create Location in RT1
	loc1, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "Brankas Utama",
		Category: fundlocation.CategoryBrankas,
	})
	require.NoError(t, err)
	require.NotEmpty(t, loc1.ID)
	require.Equal(t, "Brankas Utama", loc1.Name)
	require.Equal(t, fundlocation.CategoryBrankas, loc1.Category)
	require.True(t, loc1.IsActive)

	// 2. Multi-tenant isolation: RT2 cannot get RT1's location
	_, err = svc.GetLocationByID(ctx, rt2, loc1.ID)
	require.Error(t, err)

	// 3. Duplicate name check in RT1 (case-insensitive)
	_, err = svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "brankas utama",
		Category: fundlocation.CategoryBrankas,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, fundlocation.ErrDuplicateName)

	// 4. Same name in RT2 is allowed
	loc2, err := svc.CreateLocation(ctx, rt2, u2, fundlocation.CreateFundLocationInput{
		Name:     "Brankas Utama",
		Category: fundlocation.CategoryBrankas,
	})
	require.NoError(t, err)
	require.Equal(t, "Brankas Utama", loc2.Name)

	// 5. Update Location in RT1
	newName := "Brankas Utama Revisi"
	updated, err := svc.UpdateLocation(ctx, rt1, u1, loc1.ID, fundlocation.UpdateFundLocationInput{
		Name: &newName,
	})
	require.NoError(t, err)
	require.Equal(t, "Brankas Utama Revisi", updated.Name)

	// 6. List Locations in RT1
	list, err := svc.ListLocationsByRT(ctx, rt1, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "0.00", list[0].CurrentBalance)

	// 7. Deactivate Location with 0 balance
	deactivated, err := svc.DeactivateLocation(ctx, rt1, u1, loc1.ID)
	require.NoError(t, err)
	require.False(t, deactivated.IsActive)

	getDeactivated, err := svc.GetLocationByID(ctx, rt1, loc1.ID)
	require.NoError(t, err)
	require.False(t, getDeactivated.IsActive)
}

func TestOpeningBalanceAndReconciliation(t *testing.T) {
	pool, svc, _, rt1, _, u1, _ := setupTest(t)
	ctx := context.Background()

	// Create location
	bank := "BCA"
	loc, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "Rekening BCA",
		Category: fundlocation.CategoryRekening,
		BankName: &bank,
	})
	require.NoError(t, err)

	// Currently K=0, L=0, U=0. Attempting opening balance of 500000 must fail because 500000 > U
	_, err = svc.SetOpeningBalance(ctx, rt1, u1, loc.ID, fundlocation.SetOpeningBalanceInput{
		Amount: "500000.00",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, fundlocation.ErrInsufficientUnallocated)

	// Seed historical transaction without location to generate U = 1,000,000
	var catID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type) VALUES ($1, 'Iuran Warga', 'income') RETURNING id`,
		rt1,
	).Scan(&catID)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO transactions (rt_id, category_id, amount, type, status, description)
		 VALUES ($1, $2, 1000000.00, 'income', 'posted', 'Historical income without location')`,
		rt1, catID,
	)
	require.NoError(t, err)

	// Now U = 1,000,000. Allocate 400,000 to Rekening BCA
	_, err = svc.SetOpeningBalance(ctx, rt1, u1, loc.ID, fundlocation.SetOpeningBalanceInput{
		Amount: "400000.00",
	})
	require.NoError(t, err)

	// Verify balance is 400,000
	locWithBal, err := svc.GetLocationByID(ctx, rt1, loc.ID)
	require.NoError(t, err)
	require.Equal(t, "400000.00", locWithBal.CurrentBalance)

	// Check reconciliation: K=1,000,000, L=400,000, U=600,000, Discrepancy=0
	rec, err := svc.GetReconciliationReport(ctx, rt1)
	require.NoError(t, err)
	require.Equal(t, "1000000.00", rec.KasBalance)
	require.Equal(t, "400000.00", rec.TotalAllocatedBalance)
	require.Equal(t, "600000.00", rec.UnallocatedBalance)
	require.Equal(t, "0.00", rec.Discrepancy)
	require.Equal(t, "MATCH_WITH_NOTES", rec.Status) // U != 0 -> Matched with note
	require.Empty(t, rec.AnomaliesDetected)

	// Cannot deactivate location with balance > 0
	_, err = svc.DeactivateLocation(ctx, rt1, u1, loc.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, fundlocation.ErrCannotDeactivateWithBalance)
}

func TestFundTransfersAndProtections(t *testing.T) {
	pool, svc, _, rt1, _, u1, _ := setupTest(t)
	ctx := context.Background()

	// Create 2 locations
	bank := "Mandiri"
	fromLoc, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "Rekening Mandiri",
		Category: fundlocation.CategoryRekening,
		BankName: &bank,
	})
	require.NoError(t, err)

	toLoc, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "Brankas Kasir",
		Category: fundlocation.CategoryBrankas,
	})
	require.NoError(t, err)

	// Seed historical K and opening balance for fromLoc
	var catID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type) VALUES ($1, 'Kas Awal', 'income') RETURNING id`,
		rt1,
	).Scan(&catID)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO transactions (rt_id, category_id, amount, type, status, description)
		 VALUES ($1, $2, 500000.00, 'income', 'posted', 'Seed')`,
		rt1, catID,
	)
	require.NoError(t, err)

	_, err = svc.SetOpeningBalance(ctx, rt1, u1, fromLoc.ID, fundlocation.SetOpeningBalanceInput{
		Amount: "500000.00",
	})
	require.NoError(t, err)

	// 1. Transfer more than balance must fail (ErrInsufficientFunds)
	_, err = svc.CreateTransfer(ctx, rt1, u1, fundlocation.CreateFundTransferInput{
		FromLocationID: fromLoc.ID,
		ToLocationID:   toLoc.ID,
		Amount:         "600000.00",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, fundlocation.ErrInsufficientFunds)

	// 2. Transfer between same location must fail
	_, err = svc.CreateTransfer(ctx, rt1, u1, fundlocation.CreateFundTransferInput{
		FromLocationID: fromLoc.ID,
		ToLocationID:   fromLoc.ID,
		Amount:         "100000.00",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, fundlocation.ErrSameLocationTransfer)

	// 3. Valid transfer of 200,000
	note := "Tarik tunai untuk operasional brankas"
	transfer, err := svc.CreateTransfer(ctx, rt1, u1, fundlocation.CreateFundTransferInput{
		FromLocationID: fromLoc.ID,
		ToLocationID:   toLoc.ID,
		Amount:         "200000.00",
		Notes:          &note,
	})
	require.NoError(t, err)
	require.NotEmpty(t, transfer.ID)

	// Check balances: fromLoc = 300,000, toLoc = 200,000
	fromAfter, err := svc.GetLocationByID(ctx, rt1, fromLoc.ID)
	require.NoError(t, err)
	require.Equal(t, "300000.00", fromAfter.CurrentBalance)

	toAfter, err := svc.GetLocationByID(ctx, rt1, toLoc.ID)
	require.NoError(t, err)
	require.Equal(t, "200000.00", toAfter.CurrentBalance)

	// 4. Reverse transfer
	revTransfer, err := svc.ReverseTransfer(ctx, rt1, u1, transfer.ID, fundlocation.ReverseFundTransferInput{
		Reason: "Salah nominal",
	})
	require.NoError(t, err)
	require.NotEmpty(t, revTransfer.ID)
	require.Equal(t, &transfer.ID, revTransfer.ReversesTransferID)

	// Balances restored: fromLoc = 500,000, toLoc = 0
	fromRestored, err := svc.GetLocationByID(ctx, rt1, fromLoc.ID)
	require.NoError(t, err)
	require.Equal(t, "500000.00", fromRestored.CurrentBalance)

	toRestored, err := svc.GetLocationByID(ctx, rt1, toLoc.ID)
	require.NoError(t, err)
	require.Equal(t, "0.00", toRestored.CurrentBalance)

	// 5. Cannot reverse an already reversed transfer
	_, err = svc.ReverseTransfer(ctx, rt1, u1, transfer.ID, fundlocation.ReverseFundTransferInput{
		Reason: "Double reverse",
	})
	require.Error(t, err)
}

func TestConcurrentOpposingTransfersNoDeadlock(t *testing.T) {
	pool, svc, _, rt1, _, u1, u2 := setupTest(t)
	ctx := context.Background()

	bankA := "BCA"
	locA, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "Akun A",
		Category: fundlocation.CategoryRekening,
		BankName: &bankA,
	})
	require.NoError(t, err)

	locB, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "Akun B",
		Category: fundlocation.CategoryBrankas,
	})
	require.NoError(t, err)

	// Seed K and opening balances of 1,000,000 each
	var catID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type) VALUES ($1, 'Modal', 'income') RETURNING id`,
		rt1,
	).Scan(&catID)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO transactions (rt_id, category_id, amount, type, status, description)
		 VALUES ($1, $2, 2000000.00, 'income', 'posted', 'Seed Concurrency')`,
		rt1, catID,
	)
	require.NoError(t, err)

	_, err = svc.SetOpeningBalance(ctx, rt1, u1, locA.ID, fundlocation.SetOpeningBalanceInput{Amount: "1000000.00"})
	require.NoError(t, err)
	_, err = svc.SetOpeningBalance(ctx, rt1, u1, locB.ID, fundlocation.SetOpeningBalanceInput{Amount: "1000000.00"})
	require.NoError(t, err)

	// Concurrently transfer A -> B and B -> A 20 times simultaneously
	var wg sync.WaitGroup
	errCh := make(chan error, 40)

	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := svc.CreateTransfer(ctx, rt1, u1, fundlocation.CreateFundTransferInput{
				FromLocationID: locA.ID,
				ToLocationID:   locB.ID,
				Amount:         "10000.00",
			})
			if err != nil {
				errCh <- err
			}
		}()

		go func() {
			defer wg.Done()
			_, err := svc.CreateTransfer(ctx, rt1, u2, fundlocation.CreateFundTransferInput{
				FromLocationID: locB.ID,
				ToLocationID:   locA.ID,
				Amount:         "10000.00",
			})
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err, "No transfer should fail with deadlock or error")
	}

	// Final balances should still total 2,000,000.00
	rec, err := svc.GetReconciliationReport(ctx, rt1)
	require.NoError(t, err)
	require.Equal(t, "2000000.00", rec.TotalAllocatedBalance)
	require.Equal(t, "2000000.00", rec.KasBalance)
	require.Equal(t, "0.00", rec.Discrepancy)
}

func TestReconciliationWithPeriodOpeningBalanceAndNetMovement(t *testing.T) {
	pool, svc, _, rt1, _, u1, _ := setupTest(t)
	ctx := context.Background()

	locTime, _ := time.LoadLocation("Asia/Jakarta")
	if locTime == nil {
		locTime = time.FixedZone("WIB", 7*3600)
	}
	currentPeriod := time.Now().In(locTime).Format("2006-01")

	// 1. Set Saldo Awal KAS = 1,000,000.00 in period_opening_balances
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO period_opening_balances (rt_id, period, opening_balance, source, reason, updated_by)
		 VALUES ($1, $2, 1000000.00, 'manual', 'Saldo awal kas kasir', $3)`,
		rt1, currentPeriod, u1,
	)
	require.NoError(t, err)

	// 2. Insert transaction in current month: income 325,000.00 (Net movement = 325,000.00)
	var catID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type) VALUES ($1, 'Iuran Kas Oktober', 'income') RETURNING id`,
		rt1,
	).Scan(&catID)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO transactions (rt_id, category_id, amount, type, status, description)
		 VALUES ($1, $2, 325000.00, 'income', 'posted', 'Net movement Oktober')`,
		rt1, catID,
	)
	require.NoError(t, err)

	// 3. Create location: Rekening BCA
	bank := "BCA"
	locBCA, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "BCA RT",
		Category: fundlocation.CategoryRekening,
		BankName: &bank,
	})
	require.NoError(t, err)

	// 4. Initial reconciliation: K = 1,325,000.00, L = 0, U = 1,325,000.00, D = 0
	rec, err := svc.GetReconciliationReport(ctx, rt1)
	require.NoError(t, err)
	require.Equal(t, "1325000.00", rec.KasBalance)
	require.Equal(t, "0.00", rec.TotalAllocatedBalance)
	require.Equal(t, "1325000.00", rec.UnallocatedBalance)
	require.Equal(t, "0.00", rec.Discrepancy)
	require.Equal(t, "MATCH_WITH_NOTES", rec.Status)

	// 5. Allocate 1,000,000.00 to BCA
	_, err = svc.SetOpeningBalance(ctx, rt1, u1, locBCA.ID, fundlocation.SetOpeningBalanceInput{
		Amount: "1000000.00",
	})
	require.NoError(t, err)

	// 6. Reconciliation after 1,000,000 allocation:
	// K = 1,325,000.00, L = 1,000,000.00, U = 325,000.00 (exact unallocated reported!), D = 0
	rec, err = svc.GetReconciliationReport(ctx, rt1)
	require.NoError(t, err)
	require.Equal(t, "1325000.00", rec.KasBalance)
	require.Equal(t, "1000000.00", rec.TotalAllocatedBalance)
	require.Equal(t, "325000.00", rec.UnallocatedBalance)
	require.Equal(t, "0.00", rec.Discrepancy)
	require.Equal(t, "MATCH_WITH_NOTES", rec.Status)

	// 7. Create Brankas and allocate remaining 325,000.00
	locBrankas, err := svc.CreateLocation(ctx, rt1, u1, fundlocation.CreateFundLocationInput{
		Name:     "Brankas Kantor",
		Category: fundlocation.CategoryBrankas,
	})
	require.NoError(t, err)

	_, err = svc.SetOpeningBalance(ctx, rt1, u1, locBrankas.ID, fundlocation.SetOpeningBalanceInput{
		Amount: "325000.00",
	})
	require.NoError(t, err)

	// 8. Final reconciliation: K = 1,325,000.00, L = 1,325,000.00, U = 0.00, D = 0.00, Status = MATCH
	rec, err = svc.GetReconciliationReport(ctx, rt1)
	require.NoError(t, err)
	require.Equal(t, "1325000.00", rec.KasBalance)
	require.Equal(t, "1325000.00", rec.TotalAllocatedBalance)
	require.Equal(t, "0.00", rec.UnallocatedBalance)
	require.Equal(t, "0.00", rec.Discrepancy)
	require.Equal(t, "MATCH", rec.Status)
}
