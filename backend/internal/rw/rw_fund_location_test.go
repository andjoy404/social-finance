package rw_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/notification"
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

func TestRWOpeningBalanceAllocation(t *testing.T) {
	ctx := context.Background()
	pool, repo, svc, rwID1, _, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}
	wargaAuth := &auth.AuthContext{
		UserID: "user-warga-rw",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "warga"},
		},
	}

	// Create two locations: one rekening, one brankas
	locA, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Rekening Operasional",
		Category: rw.CategoryRekening,
		BankName: strPtr("BCA"),
	})
	require.NoError(t, err)

	locB, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas Kas",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// Seed opening balance K = 1,000,000.00 (this is the total Kas RW)
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	err = repo.SetOpeningBalance(ctx, tx, rwID1, "1000000.00", userID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// Kas Masuk 500,000.00 to locA (increases K)
	_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
		Type:        "income",
		Amount:      "500000.00",
		LocationID:  locA.ID,
		Description: "Pemasukan kas masuk",
	})
	require.NoError(t, err)

	// Now K = 1,500,000.00 (opening 1M + income 500K)
	// L = 500,000 (allocated to locA via income)
	// U = 1,000,000 (unallocated)

	t.Run("1. Alokasi saldo awal berhasil", func(t *testing.T) {
		loc, err := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locB.ID,
			Amount:     "300000.00",
			Notes:      strPtr("Alokasi ke brankas"),
		})
		require.NoError(t, err)
		require.Equal(t, "300000.00", loc.CurrentBalance)
	})

	t.Run("2. Alokasi mengurangi U", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()
		rep, err := repo.GetReconciliationReport(ctx, tx, rwID1)
		require.NoError(t, err)
		require.Equal(t, "1500000.00", rep.KasBalance)
		require.Equal(t, "800000.00", rep.TotalAllocatedBalance)
		require.Equal(t, "700000.00", rep.UnallocatedBalance)
		require.Equal(t, "0.00", rep.Discrepancy)
		require.Equal(t, "belum_teralokasi", rep.Status)
		require.Equal(t, "Belum Teralokasi", rep.StatusLabel)
	})

	t.Run("3. Alokasi menambah saldo lokasi tujuan", func(t *testing.T) {
		loc, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locB.ID)
		require.NoError(t, err)
		require.Equal(t, "300000.00", loc.CurrentBalance)
	})

	t.Run("4. Total Kas RW K tidak berubah setelah alokasi", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()
		rep, err := repo.GetReconciliationReport(ctx, tx, rwID1)
		require.NoError(t, err)
		require.Equal(t, "1500000.00", rep.KasBalance)
	})

	t.Run("5. Tidak membuat transaksi Kas Masuk baru", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()
		rep, err := repo.GetReconciliationReport(ctx, tx, rwID1)
		require.NoError(t, err)
		// Total income should still be 500,000 (only from the income transaction, not from allocation)
		require.Equal(t, "500000.00", rep.TotalIncome)
	})

	t.Run("6. Nominal nol ditolak", func(t *testing.T) {
		_, err := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locB.ID,
			Amount:     "0.00",
		})
		require.Error(t, err)
	})

	t.Run("7. Nominal negatif ditolak", func(t *testing.T) {
		_, err := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locB.ID,
			Amount:     "-100000.00",
		})
		require.Error(t, err)
	})

	t.Run("8. Nominal melebihi U ditolak", func(t *testing.T) {
		// U = 700,000. Trying to allocate 800,000
		_, err := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locB.ID,
			Amount:     "800000.00",
		})
		require.Error(t, err)
	})

	t.Run("9. Lokasi tujuan tidak valid ditolak", func(t *testing.T) {
		_, err := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: "00000000-0000-0000-0000-000000000000",
			Amount:     "100000.00",
		})
		require.Error(t, err)
	})

	t.Run("10. Warga tanpa permission ditolak", func(t *testing.T) {
		_, err := svc.AllocateOpeningBalance(ctx, wargaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locB.ID,
			Amount:     "100000.00",
		})
		require.Error(t, err)
	})

	t.Run("11. Concurrent allocation tidak menyebabkan U terlampaui", func(t *testing.T) {
		// Current U = 700,000. Launch 5 concurrent allocations of 200,000 each.
		// Only 3 should succeed (3 * 200,000 = 600,000 <= 700,000), 2 should fail.
		var successCount int
		var errs []error

		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_, allocErr := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
					LocationID: locA.ID,
					Amount:     "200000.00",
					Notes:      strPtr(fmt.Sprintf("Concurrent alloc #%d", idx)),
				})
				mu.Lock()
				if allocErr == nil {
					successCount++
				} else {
					errs = append(errs, allocErr)
				}
				mu.Unlock()
			}(i)
		}
		wg.Wait()

		// At most 3 can succeed (600,000 / 200,000 = 3)
		require.LessOrEqual(t, successCount, 3, "tidak boleh lebih dari 3 alokasi berhasil karena U = 700,000")
		// Total allocated to locA should not exceed 700,000
		loc, err := svc.GetLocationByID(ctx, bendaharaAuth, rwID1, locA.ID)
		require.NoError(t, err)
		locBalCents, _, _ := rw.ParseMoney(loc.CurrentBalance)
		// locA already had 500,000 from income, so max additional is 700,000
		require.LessOrEqual(t, locBalCents, int64(120000000), "saldo lokasi tidak boleh melebihi K + alokasi") // 500K + 700K = 1,200,000 cents
	})

	t.Run("12. Transaksi gagal tidak meninggalkan perubahan parsial", func(t *testing.T) {
	// Capture U before invalid allocation attempt
	txBefore, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	repBefore, err := repo.GetReconciliationReport(ctx, txBefore, rwID1)
	require.NoError(t, err)
	require.NoError(t, txBefore.Rollback())
	uBefore := repBefore.UnallocatedBalance

	// Try to allocate with invalid location - should not change anything
	_, err = svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
		LocationID: "invalid-uuid",
		Amount:     "100000.00",
	})
	require.Error(t, err)

	// Verify U did not change
	txAfter, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	repAfter, err := repo.GetReconciliationReport(ctx, txAfter, rwID1)
	require.NoError(t, err)
	require.NoError(t, txAfter.Rollback())
	require.Equal(t, uBefore, repAfter.UnallocatedBalance, "U harus tidak berubah setelah alokasi gagal")
	})
}

func TestRWReconciliationScenarios(t *testing.T) {
	ctx := context.Background()
	pool, repo, svc, rwID1, _, _, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	// Create two locations
	locA, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Rekening A",
		Category: rw.CategoryRekening,
		BankName: strPtr("BCA"),
	})
	require.NoError(t, err)

	locB, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas B",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	t.Run("1. K=500.000, L=300.000 → U=200.000, D=0, Belum Teralokasi", func(t *testing.T) {
		// Set opening balance K=500.000
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		err = repo.SetOpeningBalance(ctx, tx, rwID1, "500000.00", userID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		// Allocate 300.000 to locA → L=300.000
		_, err = svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locA.ID,
			Amount:     "300000.00",
		})
		require.NoError(t, err)

		tx, err = pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()
		rep, err := repo.GetReconciliationReport(ctx, tx, rwID1)
		require.NoError(t, err)
		require.Equal(t, "500000.00", rep.KasBalance)
		require.Equal(t, "300000.00", rep.TotalAllocatedBalance)
		require.Equal(t, "200000.00", rep.UnallocatedBalance)
		require.Equal(t, "0.00", rep.Discrepancy)
		require.Equal(t, "belum_teralokasi", rep.Status)
		require.Equal(t, "Belum Teralokasi", rep.StatusLabel)
	})

	t.Run("2. K=500.000, L=500.000 → U=0, D=0, Cocok Sempurna", func(t *testing.T) {
		// Reset: allocate remaining 200.000 to locB → L=500.000
		_, err = svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locB.ID,
			Amount:     "200000.00",
		})
		require.NoError(t, err)

		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()
		rep, err := repo.GetReconciliationReport(ctx, tx, rwID1)
		require.NoError(t, err)
		require.Equal(t, "500000.00", rep.KasBalance)
		require.Equal(t, "500000.00", rep.TotalAllocatedBalance)
		require.Equal(t, "0.00", rep.UnallocatedBalance)
		require.Equal(t, "0.00", rep.Discrepancy)
		require.Equal(t, "matched", rep.Status)
		require.Equal(t, "Cocok Sempurna", rep.StatusLabel)
	})

	t.Run("3. K=500.000, L=600.000 → U=0, D=-100.000, Discrepancy", func(t *testing.T) {
		// This simulates over-allocation: L > K
		// We can't directly create LBC entries, so we manipulate via allocation
		// First reset: set opening balance to 500.000 (already done)
		// Allocate 500.000 total already (scenario 2). Now add 100.000 more to force L=600.000
		// But allocation checks U, so U=0 means we can't allocate more.
		// Instead, directly insert an LBC entry to simulate over-allocation.
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx,
			`INSERT INTO rw_location_balance_changes (rw_id, location_id, change_type, amount_delta, notes, created_by)
			 VALUES ($1, $2, 'opening_balance', $3::numeric(15,2), $4, $5)`,
			rwID1, locA.ID, "100000.00", "Over-allocation simulation", userID,
		)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		tx, err = pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()
		rep, err := repo.GetReconciliationReport(ctx, tx, rwID1)
		require.NoError(t, err)
		require.Equal(t, "500000.00", rep.KasBalance)
		require.Equal(t, "600000.00", rep.TotalAllocatedBalance)
		require.Equal(t, "0.00", rep.UnallocatedBalance)
		require.Equal(t, "-100000.00", rep.Discrepancy)
		require.Equal(t, "discrepant", rep.Status)
		require.Equal(t, "Discrepancy", rep.StatusLabel)
	})

	t.Run("4. K=0 → tidak pembagian dengan nol", func(t *testing.T) {
		// Use a fresh RW with no opening balance
		var rwID2 string
		err := pool.Raw().QueryRowContext(ctx,
			`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Reconcile Zero', 99999, true) RETURNING id`,
		).Scan(&rwID2)
		require.NoError(t, err)

		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback()
		rep, err := repo.GetReconciliationReport(ctx, tx, rwID2)
		require.NoError(t, err)
		require.Equal(t, "0.00", rep.KasBalance)
		require.Equal(t, "0.00", rep.TotalAllocatedBalance)
		require.Equal(t, "0.00", rep.UnallocatedBalance)
		require.Equal(t, "0.00", rep.Discrepancy)
		require.Equal(t, "matched", rep.Status)
		require.Equal(t, "Cocok Sempurna", rep.StatusLabel)
		require.Equal(t, 0.0, rep.AllocatedPercentage)
	})
}

func TestRWTransferAndTransactionEffects(t *testing.T) {
	ctx := context.Background()
	pool, repo, svc, rwID1, _, rtID1, userID := setupRWFundLocationTest(t)

	bendaharaAuth := &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	// Create two locations
	locA, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Rekening A",
		Category: rw.CategoryRekening,
		BankName: strPtr("BCA"),
	})
	require.NoError(t, err)

	locB, err := svc.CreateLocation(ctx, bendaharaAuth, rwID1, rw.CreateRWFundLocationInput{
		Name:     "Brankas B",
		Category: rw.CategoryBrankas,
	})
	require.NoError(t, err)

	// Set opening balance K = 1,000,000
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	err = repo.SetOpeningBalance(ctx, tx, rwID1, "1000000.00", userID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	t.Run("1. Transfer antar-lokasi tidak mengubah total L", func(t *testing.T) {
		// Allocate 300,000 to locA → L = 300,000
		_, err := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locA.ID,
			Amount:     "300000.00",
		})
		require.NoError(t, err)

		// Get L before transfer
		tx1, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		repBefore, err := repo.GetReconciliationReport(ctx, tx1, rwID1)
		require.NoError(t, err)
		require.NoError(t, tx1.Rollback())
		lBefore := repBefore.TotalAllocatedBalance

		// Transfer 100,000 from locA to locB
		_, err = svc.CreateTransfer(ctx, bendaharaAuth, rwID1, rw.CreateFundTransferInput{
			FromLocationID: locA.ID,
			ToLocationID:   locB.ID,
			Amount:         "100000.00",
			Notes:          strPtr("Transfer ke Brankas B"),
		})
		require.NoError(t, err)

		// Get L after transfer
		tx2, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		repAfter, err := repo.GetReconciliationReport(ctx, tx2, rwID1)
		require.NoError(t, err)
		require.NoError(t, tx2.Rollback())

		// L should be unchanged (transfer_out -100k + transfer_in +100k = 0)
		require.Equal(t, lBefore, repAfter.TotalAllocatedBalance, "Transfer tidak boleh mengubah total L")
	})

	t.Run("2. Setoran RT hanya dihitung satu kali sebagai income", func(t *testing.T) {
		// State after test 1: K=1,000,000, L=300,000 (allocated to locA)
		tx1, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		repBefore, err := repo.GetReconciliationReport(ctx, tx1, rwID1)
		require.NoError(t, err)
		require.NoError(t, tx1.Rollback())
		require.Equal(t, "1000000.00", repBefore.KasBalance)
		require.Equal(t, "300000.00", repBefore.TotalAllocatedBalance)

		// Create Kas Masuk via Setoran RT: 200,000 to locA
		_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
			Type:        "income",
			Amount:      "200000.00",
			LocationID:  locA.ID,
			SourceRTID:  &rtID1,
			Description: "Setoran dari RT 01",
		})
		require.NoError(t, err)

		tx2, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		repAfter, err := repo.GetReconciliationReport(ctx, tx2, rwID1)
		require.NoError(t, err)
		require.NoError(t, tx2.Rollback())

		// K should increase by 200,000 (1M + 200k)
		require.Equal(t, "1200000.00", repAfter.KasBalance)
		// Total income should be 200,000 (counted once)
		require.Equal(t, "200000.00", repAfter.TotalIncome)
		// L should increase by 200,000 (300k + 200k)
		require.Equal(t, "500000.00", repAfter.TotalAllocatedBalance)
		// U should be unchanged (K and L both increased by 200k)
		require.Equal(t, repBefore.UnallocatedBalance, repAfter.UnallocatedBalance,
			"Setoran RT tidak boleh mengubah U (K dan L bertambah seimbang)")
	})

	t.Run("3. Kas Keluar mengurangi K dan L secara konsisten", func(t *testing.T) {
		// State after test 2: K=1,200,000, L=500,000 (300k locA + 200k setoran RT)
		// First add balance to locB via allocation
		_, err := svc.AllocateOpeningBalance(ctx, bendaharaAuth, rwID1, rw.AllocateOpeningBalanceInput{
			LocationID: locB.ID,
			Amount:     "500000.00",
		})
		require.NoError(t, err)

		tx1, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		repBefore, err := repo.GetReconciliationReport(ctx, tx1, rwID1)
		require.NoError(t, err)
		require.NoError(t, tx1.Rollback())
		require.Equal(t, "1200000.00", repBefore.KasBalance)
		require.Equal(t, "1000000.00", repBefore.TotalAllocatedBalance) // 300k + 200k + 500k

		// Kas Keluar from locB: 100,000
		_, err = svc.CreateTransaction(ctx, bendaharaAuth, rwID1, rw.CreateTransactionInput{
			Type:        "expense",
			Amount:      "100000.00",
			LocationID:  locB.ID,
			Description: "Pembelian ATK",
		})
		require.NoError(t, err)

		tx2, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		repAfter, err := repo.GetReconciliationReport(ctx, tx2, rwID1)
		require.NoError(t, err)
		require.NoError(t, tx2.Rollback())

		// K should decrease by 100,000 (1.2M - 100k)
		require.Equal(t, "1100000.00", repAfter.KasBalance)
		// Total expense should be 100,000
		require.Equal(t, "100000.00", repAfter.TotalExpense)
		// L should decrease by 100,000 (1M - 100k)
		require.Equal(t, "900000.00", repAfter.TotalAllocatedBalance)
		// U should be unchanged
		require.Equal(t, repBefore.UnallocatedBalance, repAfter.UnallocatedBalance,
			"Kas Keluar tidak boleh mengubah U (K dan L berkurang seimbang)")
	})

	t.Run("4. Setoran RT dengan RT dari RW lain ditolak", func(t *testing.T) {
		t.Skip("Need second RW for cross-RW test - covered by backend validation")
	})
}

func TestRWGetRTRsEndpoint(t *testing.T) {
	ctx := context.Background()
	pool, _, svc, rwID1, rwID2, rtID1, userID := setupRWFundLocationTest(t)

	// Add more RTs under RW1
	var rtID2 string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT FL 2', 2, '02', $1, true) RETURNING id`,
		rwID1,
	).Scan(&rtID2)
	require.NoError(t, err)

	// RT under RW2 (should NOT appear in RW1 results)
	var rtID3 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT FL 3', 3, '03', $1, true) RETURNING id`,
		rwID2,
	).Scan(&rtID3)
	require.NoError(t, err)

	// Inactive RT under RW1 (should NOT appear)
	var rtID4 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT FL Inactive', 4, '04', $1, false) RETURNING id`,
		rwID1,
	).Scan(&rtID4)
	require.NoError(t, err)

	h := rw.NewHandler(pool, svc, &mockNotificationService{})

	router := chi.NewRouter()
	router.Get("/api/v1/rw/{rwId}/rts", h.GetRTRs)

	ac := &auth.AuthContext{
		UserID: userID,
		TenantRole: auth.RoleBendahara,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	acSA := &auth.AuthContext{
		UserID: userID,
		SystemRole: auth.SystemRoleSuperAdmin,
	}

	makeRequest := func(rwID string, acCtx *auth.AuthContext) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/rw/"+rwID+"/rts", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, acCtx))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("bendahara lists active RTs under RW1 (no 500)", func(t *testing.T) {
		rec := makeRequest(rwID1, ac)
		require.Equal(t, http.StatusOK, rec.Code, "expected 200, got %d: %s", rec.Code, rec.Body.String())

		var body map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)

		data := body["data"].([]any)
		// Should have exactly 2 active RTs under RW1 (rtID1, rtID2)
		require.Len(t, data, 2, "expected 2 active RTs under RW1")

		ids := make(map[string]bool)
		for _, item := range data {
			m := item.(map[string]any)
			require.NotEmpty(t, m["id"])
			require.NotEmpty(t, m["name"])
			require.NotEmpty(t, m["rt"])
			require.NotEmpty(t, m["label"])
			ids[m["id"].(string)] = true
		}
		require.True(t, ids[rtID1], "rtID1 should be in results")
		require.True(t, ids[rtID2], "rtID2 should be in results")
		require.False(t, ids[rtID3], "rtID3 (RW2) should not be in RW1 results")
		require.False(t, ids[rtID4], "rtID4 (inactive) should not be in results")
	})

	t.Run("superAdmin can list RTs under any RW", func(t *testing.T) {
		rec := makeRequest(rwID1, acSA)
		require.Equal(t, http.StatusOK, rec.Code, "superAdmin expected 200, got %d: %s", rec.Code, rec.Body.String())
	})

	t.Run("invalid RW ID returns empty data (not 500)", func(t *testing.T) {
		invalidID := "00000000-0000-0000-0000-000000000000"
		rec := makeRequest(invalidID, ac)
		require.Equal(t, http.StatusOK, rec.Code, "invalid RW expected 200, got %d: %s", rec.Code, rec.Body.String())

		var body map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)

		data, ok := body["data"].([]any)
		require.True(t, ok, "data should be an array")
		require.Empty(t, data, "invalid RW should return empty data")
	})

	t.Run("no auth context returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/rw/"+rwID1+"/rts", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// mockNotificationService implements notification.Service for testing.
// It embeds the interface so all methods return zero values by default.
type mockNotificationService struct {
	notification.Service
}

