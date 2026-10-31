package rw_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/rw"
	"social-finance/internal/testutil"
)

func setupRWFundLocationTest(t *testing.T) (*database.Pool, *rw.Repository, *rw.Service, string, string, string, string) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	repo := rw.NewRepository(pool.Raw())

	rwUUID1 := uuid.New()
	rwUUID2 := uuid.New()
	ctx := context.Background()

	var rwID1, rwID2 string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW FL 1', $1, true) RETURNING id`,
		2000000+int(rwUUID1[0])*65536+int(rwUUID1[1])*256+int(rwUUID1[2]),
	).Scan(&rwID1)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW FL 2', $1, true) RETURNING id`,
		2000000+int(rwUUID2[0])*65536+int(rwUUID2[1])*256+int(rwUUID2[2]),
	).Scan(&rwID2)
	require.NoError(t, err)

	// RT under RW 1 (for source RT testing)
	var rtID1 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT FL 1', 1, '01', $1, true) RETURNING id`,
		rwID1,
	).Scan(&rtID1)
	require.NoError(t, err)

	var userID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'hash', 'Test Bendahara RW', true) RETURNING id`,
		fmt.Sprintf("bendahara-fl-%s@test.com", uuid.New().String()[:8]),
		fmt.Sprintf("081%d", time.Now().UnixNano()%100000000),
	).Scan(&userID)
	require.NoError(t, err)

	cfg := &auth.ScopingConfig{
		RWIDs: []string{rwID1, rwID2},
	}
	svc := rw.NewService(pool, repo, cfg)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_location_balance_changes WHERE rw_id IN ($1, $2)`, rwID1, rwID2)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_transactions WHERE rw_id IN ($1, $2)`, rwID1, rwID2)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_fund_transfers WHERE rw_id IN ($1, $2)`, rwID1, rwID2)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_fund_locations WHERE rw_id IN ($1, $2)`, rwID1, rwID2)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_opening_balances WHERE rw_id IN ($1, $2)`, rwID1, rwID2)
		pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID1)
		pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id IN ($1, $2)`, rwID1, rwID2)
		pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	return pool, repo, svc, rwID1, rwID2, rtID1, userID
}

func TestRWFundLocations_CRUDAndValidation(t *testing.T) {
	ctx := context.Background()
	_, repo, svc, rwID1, _, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}
	wargaAuth := &auth.AuthContext{
		UserID: "user-warga-rw",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "anggota"}, // no finance permissions
		},
	}

	// 1. Warga cannot create location
	bankNameBCA := "BCA"
	accNo := "1234567890"
	accHolder := "Kas RW"
	_, err := svc.CreateLocation(ctx, wargaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:          "Rekening Operasional RW",
		Category:      rw.CategoryRekening,
		BankName:      &bankNameBCA,
		AccountNumber: &accNo,
		AccountHolder: &accHolder,
	})
	require.ErrorIs(t, err, rw.ErrPermissionDenied)

	// 2. Bendahara creates valid Rekening
	locRekening, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:          "Rekening Operasional RW",
		Category:      rw.CategoryRekening,
		BankName:      &bankNameBCA,
		AccountNumber: &accNo,
		AccountHolder: &accHolder,
	})
	require.NoError(t, err)
	require.NotEmpty(t, locRekening.ID)
	require.Equal(t, rwID1, locRekening.RWID)
	require.Equal(t, "Rekening Operasional RW", locRekening.Name)
	require.Equal(t, rw.CategoryRekening, locRekening.Category)
	require.Equal(t, "0.00", locRekening.CurrentBalance)

	// 3. Duplicate name in same RW is rejected (409 Conflict)
	_, err = svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:          "  rekening operasional rw  ", // case-insensitive trimmed
		Category:      rw.CategoryRekening,
		BankName:      &bankNameBCA,
	})
	require.ErrorIs(t, err, rw.ErrDuplicateName)

	// 4. Create Brankas and COH
	locBrankas, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas Utama RW",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	locCOH, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Kas Di Tangan Bendahara",
		Category: rw.CategoryCOH,
	})
	require.NoError(t, err)

	// 5. List locations (should have 3 dedicated locations)
	locs, err := svc.GetLocations(ctx, bendaharaAuth, rwID1, nil)
	require.NoError(t, err)
	require.Len(t, locs, 3)

	// 6. Update location
	newName := "Rekening Operasional RW (BCA)"
	updated, err := svc.UpdateLocation(ctx, bendaharaAuth, rwID1, locRekening.ID, rw.UpdateRWFundLocationInput{
		Name: &newName,
	})
	require.NoError(t, err)
	require.Equal(t, newName, updated.Name)

	// 7. Deactivate with 0 balance succeeds
	deactivated, err := svc.ToggleLocationStatus(ctx, bendaharaAuth, rwID1, locBrankas.ID, false)
	require.NoError(t, err)
	require.False(t, deactivated.IsActive)

	// Reactivate
	reactivated, err := svc.ToggleLocationStatus(ctx, bendaharaAuth, rwID1, locBrankas.ID, true)
	require.NoError(t, err)
	require.True(t, reactivated.IsActive)

	_ = repo
	_ = locCOH
}

func TestRWFundLocations_TransferAndReversal(t *testing.T) {
	ctx := context.Background()
	pool, _, svc, rwID1, rwID2, rtID1, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	// 1. Create two locations in RW 1
	bankBCA := "BCA"
	accNo := "111"
	accHolder := "RW"
	locRekening, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:          "Rekening RW 1",
		Category:      rw.CategoryRekening,
		BankName:      &bankBCA,
		AccountNumber: &accNo,
		AccountHolder: &accHolder,
	})
	require.NoError(t, err)

	locBrankas, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas RW 1",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// 2. Fund the Rekening with a KAS Masuk (e.g., from RT 1 or external)
	kasMasuk, err := svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "income",
		Amount:      "500000.00",
		LocationID:  locRekening.ID,
		SourceRTID:  &rtID1,
		Description: "Penerimaan Iuran dari RT 1 ke RW",
	})
	require.NoError(t, err)
	require.Equal(t, "income", kasMasuk.Type)
	require.Equal(t, "500000.00", kasMasuk.Amount)

	// Check Rekening balance is now 500,000.00
	rekeningDetail, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locRekening.ID)
	require.NoError(t, err)
	require.Equal(t, "500000.00", rekeningDetail.CurrentBalance)

	// 3. Attempt transfer with insufficient funds (> 500,000)
	_, err = svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: locRekening.ID,
		ToLocationID:   locBrankas.ID,
		Amount:         "600000.00",
	})
	require.ErrorIs(t, err, rw.ErrInsufficientFunds)

	// 4. Attempt transfer to same location
	_, err = svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: locRekening.ID,
		ToLocationID:   locRekening.ID,
		Amount:         "100000.00",
	})
	require.ErrorIs(t, err, rw.ErrSameLocationTransfer)

	// 5. Attempt Cross-RW transfer (from RW 1 to RW 2 location)
	bendaharaRW2Auth := &auth.AuthContext{
		UserID: "user-bendahara-rw2",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID2, Jabatan: "bendahara"},
		},
	}
	locRW2, err := svc.CreateLocation(ctx, bendaharaRW2Auth, rwID2, rw.CreateRWFundLocationInput{
		Name:     "Brankas RW 2",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	_, err = svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: locRekening.ID,
		ToLocationID:   locRW2.ID, // Different RW!
		Amount:         "100000.00",
	})
	require.Error(t, err) // Cross-RW rejected!

	// 6. Valid Transfer: 200,000.00 from Rekening to Brankas
	notes := "Pencairan operasional ke brankas"
	tr, err := svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: locRekening.ID,
		ToLocationID:   locBrankas.ID,
		Amount:         "200000.00",
		Notes:          &notes,
	})
	require.NoError(t, err)
	require.NotEmpty(t, tr.ID)
	require.Equal(t, "200000.00", tr.Amount)

	// Verify balances after transfer:
	// Rekening: 500,000 - 200,000 = 300,000.00
	// Brankas: 0 + 200,000 = 200,000.00
	rekeningAfter, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locRekening.ID)
	require.NoError(t, err)
	require.Equal(t, "300000.00", rekeningAfter.CurrentBalance)

	brankasAfter, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locBrankas.ID)
	require.NoError(t, err)
	require.Equal(t, "200000.00", brankasAfter.CurrentBalance)

	// Total RW dashboard net balance should still be 500,000.00 (transfers don't alter net kas)
	dash, err := svc.GetDashboard(ctx, bendaharaAuth, rwID1)
	require.NoError(t, err)
	require.Equal(t, "500000.00", dash.NetBalance)

	// 7. Attempt deactivation of location with positive balance -> rejected
	_, err = svc.ToggleLocationStatus(ctx, bendaharaAuth, rwID1, locRekening.ID, false)
	require.ErrorIs(t, err, rw.ErrCannotDeactivateWithBalance)

	// 8. Reversal of transfer
	// Case A: If Brankas has spent money and has insufficient balance to refund -> reversal rejected
	expenseTx, err := svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "expense",
		Amount:      "150000.00",
		LocationID:  locBrankas.ID,
		Description: "Pengeluaran darurat",
	})
	require.NoError(t, err)

	// Now Brankas balance is 200,000 - 150,000 = 50,000. Reversing 200,000 should fail!
	_, err = svc.ReverseTransfer(ctx, bendaharaAuth, rwID1, tr.ID, "Salah nominal transfer")
	require.ErrorIs(t, err, rw.ErrInsufficientReversal)

	// Clean up that expense so Brankas has full 200,000 again
	pool.Raw().ExecContext(ctx, `DELETE FROM rw_location_balance_changes WHERE transaction_id = $1`, expenseTx.ID)
	pool.Raw().ExecContext(ctx, `DELETE FROM rw_transactions WHERE id = $1`, expenseTx.ID)

	// Case B: Successful reversal
	rev, err := svc.ReverseTransfer(ctx, bendaharaAuth, rwID1, tr.ID, "Batal pencairan")
	require.NoError(t, err)
	require.NotEmpty(t, rev.ID)

	// Check balances after reversal:
	// Rekening restored to 500,000.00
	// Brankas restored to 0.00
	rekeningFinal, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locRekening.ID)
	require.NoError(t, err)
	require.Equal(t, "500000.00", rekeningFinal.CurrentBalance)

	brankasFinal, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locBrankas.ID)
	require.NoError(t, err)
	require.Equal(t, "0.00", brankasFinal.CurrentBalance)

	// Cannot reverse already reversed transfer
	_, err = svc.ReverseTransfer(ctx, bendaharaAuth, rwID1, tr.ID, "Ulang lagi")
	require.ErrorIs(t, err, rw.ErrAlreadyReversed)

	// Cannot reverse a reversal
	_, err = svc.ReverseTransfer(ctx, bendaharaAuth, rwID1, rev.ID, "Reverse the reversal")
	require.ErrorIs(t, err, rw.ErrCannotReverseReversal)

	// Check reconciliation
	recon, err := svc.GetReconciliation(ctx, bendaharaAuth, rwID1)
	require.NoError(t, err)
	require.Equal(t, "matched", recon.Status)
	require.Equal(t, "500000.00", recon.KasBalance)
	require.Equal(t, "500000.00", recon.TotalAllocatedBalance)
	require.Equal(t, "0.00", recon.Discrepancy)
}

func TestRWFundLocations_InactiveLocationCannotTransact(t *testing.T) {
	ctx := context.Background()
	_, _, svc, rwID1, _, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	// 1. Create a location and deactivate it while balance is 0
	loc, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas Nonaktif",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	_, err = svc.ToggleLocationStatus(ctx, bendaharaAuth, rwID1, loc.ID, false)
	require.NoError(t, err)

	// 2. KAS Masuk to inactive location must fail
	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "income",
		Amount:      "100000.00",
		LocationID:  loc.ID,
		Description: "Setoran ke lokasi nonaktif",
	})
	require.ErrorIs(t, err, rw.ErrInactiveLocation)

	// 3. KAS Keluar from inactive location must fail
	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "expense",
		Amount:      "50000.00",
		LocationID:  loc.ID,
		Description: "Pengeluaran dari lokasi nonaktif",
	})
	require.ErrorIs(t, err, rw.ErrInactiveLocation)

	// 4. Create another active location and attempt transfer to/from inactive location
	activeLoc, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas Aktif",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// Transfer FROM inactive location
	_, err = svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: loc.ID,
		ToLocationID:   activeLoc.ID,
		Amount:         "10000.00",
	})
	require.ErrorIs(t, err, rw.ErrInactiveLocation)

	// Transfer TO inactive location
	_, err = svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: activeLoc.ID,
		ToLocationID:   loc.ID,
		Amount:         "10000.00",
	})
	require.ErrorIs(t, err, rw.ErrInactiveLocation)
}

func TestRWFundLocations_CrossRWDirectIDAccessBlocked(t *testing.T) {
	ctx := context.Background()
	_, _, svc, rwID1, rwID2, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth1 := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}
	bendaharaAuth2 := &auth.AuthContext{
		UserID: "user-bendahara-rw2",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID2, Jabatan: "bendahara"},
		},
	}

	// Create location in RW 2
	locRW2, err := svc.CreateLocation(ctx, bendaharaAuth2, rwID2, rw.CreateRWFundLocationInput{
		Name:     "Brankas Rahasia RW 2",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// Bendahara RW 1 attempts to access RW 2's location using locRW2.ID under RW 1 scope -> ErrNotFound (tenant isolated)
	_, err = svc.GetLocationByID(ctx, bendaharaAuth1, rwID1, locRW2.ID)
	require.ErrorIs(t, err, rw.ErrNotFound)

	// Bendahara RW 1 attempts to pass rwID2 in scope -> ErrRWAccessDenied (no membership in RW 2)
	_, err = svc.GetLocationByID(ctx, bendaharaAuth1, rwID2, locRW2.ID)
	require.ErrorIs(t, err, rw.ErrRWAccessDenied)
}

func TestRWFundLocations_ConcurrentTransactionsConsistent(t *testing.T) {
	ctx := context.Background()
	_, _, svc, rwID1, _, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	loc, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas Concurrency",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// Seed with 1,000,000.00
	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "income",
		Amount:      "1000000.00",
		LocationID:  loc.ID,
		Description: "Modal awal concurrency test",
	})
	require.NoError(t, err)

	// Run 10 concurrent expenses of 100,000 each in parallel goroutines
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, txErr := svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
				Type:        "expense",
				Amount:      "100000.00",
				LocationID:  loc.ID,
				Description: fmt.Sprintf("Pengeluaran concurrent #%d", idx),
			})
			if txErr != nil {
				errs <- txErr
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for txErr := range errs {
		require.NoError(t, txErr)
	}

	// Total final balance must be exactly 0.00 (1,000,000 - 10 * 100,000)
	locDetail, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, loc.ID)
	require.NoError(t, err)
	require.Equal(t, "0.00", locDetail.CurrentBalance)

	// An 11th expense must be rejected with ErrInsufficientFunds
	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "expense",
		Amount:      "1000.00",
		LocationID:  loc.ID,
		Description: "Pengeluaran ke-11 melebihi saldo",
	})
	require.ErrorIs(t, err, rw.ErrInsufficientFunds)
}

func TestRWFundLocations_ConcurrentOpposingTransfersNoDeadlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, _, svc, rwID1, _, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	locA, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Lokasi Opposing A",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	locB, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Lokasi Opposing B",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// Seed both with 1,000,000.00
	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "income",
		Amount:      "1000000.00",
		LocationID:  locA.ID,
		Description: "Modal A",
	})
	require.NoError(t, err)

	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "income",
		Amount:      "1000000.00",
		LocationID:  locB.ID,
		Description: "Modal B",
	})
	require.NoError(t, err)

	// Launch 10 transfers A -> B and 10 transfers B -> A in parallel
	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			notes := fmt.Sprintf("Transfer A ke B #%d", idx)
			_, txErr := svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
				FromLocationID: locA.ID,
				ToLocationID:   locB.ID,
				Amount:         "50000.00",
				Notes:          &notes,
			})
			if txErr != nil {
				errs <- txErr
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			notes := fmt.Sprintf("Transfer B ke A #%d", idx)
			_, txErr := svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
				FromLocationID: locB.ID,
				ToLocationID:   locA.ID,
				Amount:         "50000.00",
				Notes:          &notes,
			})
			if txErr != nil {
				errs <- txErr
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for txErr := range errs {
		require.NoError(t, txErr)
	}

	// Because 10 * 50,000 went A->B and 10 * 50,000 went B->A, final balances should be exactly 1,000,000.00 each
	detailA, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locA.ID)
	require.NoError(t, err)
	require.Equal(t, "1000000.00", detailA.CurrentBalance)

	detailB, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locB.ID)
	require.NoError(t, err)
	require.Equal(t, "1000000.00", detailB.CurrentBalance)
}

func TestRWFundLocations_ConcurrentReversalsSingleSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, _, svc, rwID1, _, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	locA, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Lokasi Rev A",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	locB, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Lokasi Rev B",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// Seed A with 500,000.00
	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "income",
		Amount:      "500000.00",
		LocationID:  locA.ID,
		Description: "Modal A",
	})
	require.NoError(t, err)

	// Transfer 200,000 from A to B
	tr, err := svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: locA.ID,
		ToLocationID:   locB.ID,
		Amount:         "200000.00",
	})
	require.NoError(t, err)

	// Launch 2 concurrent goroutines attempting to reverse the exact same transfer simultaneously
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, revErr := svc.ReverseTransfer(ctx, bendaharaAuth, rwID1, tr.ID, fmt.Sprintf("Reversal attempt #%d", idx))
			errs <- revErr
		}(i)
	}

	wg.Wait()
	close(errs)

	var successCount, alreadyReversedCount int
	for revErr := range errs {
		if revErr == nil {
			successCount++
		} else if revErr == rw.ErrAlreadyReversed {
			alreadyReversedCount++
		} else {
			t.Fatalf("unexpected error: %v", revErr)
		}
	}

	require.Equal(t, 1, successCount, "exactly one reversal must succeed")
	require.Equal(t, 1, alreadyReversedCount, "the second concurrent reversal must fail with ErrAlreadyReversed")

	// Balances must be restored accurately once: A = 500,000, B = 0.00
	detailA, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locA.ID)
	require.NoError(t, err)
	require.Equal(t, "500000.00", detailA.CurrentBalance)

	detailB, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locB.ID)
	require.NoError(t, err)
	require.Equal(t, "0.00", detailB.CurrentBalance)
}
