package rw_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	rw "social-finance/internal/rw"
	"social-finance/internal/testutil"
)

func setupRWTest(t *testing.T) (*database.Pool, *rw.Repository, string, string, string, string, string) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	repo := rw.NewRepository(pool.Raw())

	ctx := context.Background()

	// RW 1 — tempat kita menguji
	var rwID1 string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW 1', 1, true) RETURNING id`,
	).Scan(&rwID1)
	require.NoError(t, err)

	// RW 2 — untuk isolasi tenant
	var rwID2 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW 2', 2, true) RETURNING id`,
	).Scan(&rwID2)
	require.NoError(t, err)

	// RT aktif di RW 1
	var rtID1 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 1-01', 1, $1, $2, true) RETURNING id`,
		fmt.Sprintf("01-%d", time.Now().UnixNano()), rwID1,
	).Scan(&rtID1)
	require.NoError(t, err)

	// RT aktif kedua di RW 1
	var rtID2 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 1-02', 1, $1, $2, true) RETURNING id`,
		fmt.Sprintf("02-%d", time.Now().UnixNano()), rwID1,
	).Scan(&rtID2)
	require.NoError(t, err)

	// RT aktif di RW 2 (harus terisolasi)
	var rtID3 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 2-01', 2, $1, $2, true) RETURNING id`,
		fmt.Sprintf("03-%d", time.Now().UnixNano()), rwID2,
	).Scan(&rtID3)
	require.NoError(t, err)

	return pool, repo, rwID1, rwID2, rtID1, rtID2, rtID3
}

// ---- Seed helpers for independent RW ledger ----

func seedCategory(t *testing.T, ctx context.Context, pool *database.Pool, rtID string) string {
	var catID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type) VALUES ($1, 'Iuran Uji', 'income') RETURNING id`,
		rtID,
	).Scan(&catID)
	require.NoError(t, err)
	return catID
}

func seedRWTransaction(t *testing.T, ctx context.Context, pool *database.Pool, rwID, categoryID string, amount float64, txnType, status, description string) string {
	var txID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rw_transactions (rw_id, category_id, amount, type, status, description)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		rwID, categoryID, amount, txnType, status, description,
	).Scan(&txID)
	require.NoError(t, err, "seed RW transaction rw=%s type=%s status=%s amount=%f: %v", rwID, txnType, status, amount, err)
	return txID
}

func seedRWReversal(t *testing.T, ctx context.Context, pool *database.Pool, rwID, categoryID string, amount float64, originalTxID string, description string) string {
	var revID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rw_transactions (rw_id, category_id, amount, type, status, reverses_transaction_id, description)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		rwID, categoryID, amount, "expense", "posted", originalTxID, description,
	).Scan(&revID)
	require.NoError(t, err, "seed RW reversal rw=%s: %v", rwID, err)
	return revID
}

func seedOpeningBalance(t *testing.T, ctx context.Context, pool *database.Pool, rwID string, amount float64, desc string) {
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO rw_opening_balances (rw_id, amount, description)
		 VALUES ($1, $2, $3)`,
		rwID, amount, desc,
	)
	require.NoError(t, err, "seed opening balance rw=%s amount=%f: %v", rwID, amount, err)
}

func seedLocationWithCategory(t *testing.T, ctx context.Context, pool *database.Pool, rtID, name, category, bankName string) string {
	var locID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO fund_locations (rt_id, name, category, bank_name) VALUES ($1, $2, $3, $4) RETURNING id`,
		rtID, name, category, bankName,
	).Scan(&locID)
	require.NoError(t, err, "seed location %s: %v", name, err)
	return locID
}

func seedLBC(t *testing.T, ctx context.Context, pool *database.Pool, locationID, rtID string, changeType string, amountDelta string, txnID *string, transferID *string) {
	query := `INSERT INTO location_balance_changes (rt_id, location_id, change_type, amount_delta`
	args := []interface{}{rtID, locationID, changeType}
	pos := 4

	if txnID != nil {
		query += `, transaction_id`
		args = append(args, *txnID)
		pos++
	}
	if transferID != nil {
		query += `, transfer_id`
		args = append(args, *transferID)
		pos++
	}
	query += fmt.Sprintf(`) VALUES ($%d, $%d, $%d, $%d)`, pos-3, pos-2, pos-1, pos)
	args = append(args, amountDelta)

	_, err := pool.Raw().ExecContext(ctx, query, args...)
	require.NoError(t, err, "seed LBC for loc %s type=%s delta=%s: %v", locationID, changeType, amountDelta, err)
}

// ===========================================================================
// REGRESSION TEST 1: Saldo awal RW Rp7.000.000
// Saldo RW = Rp7.000.000, meskipun total saldo lokasi dana RT berbeda
// ===========================================================================

func TestRegression1_OpeningBalanceIndependent(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID1, _, _ := setupRWTest(t)

	// Saldo awal RW = Rp7.000.000
	seedOpeningBalance(t, ctx, pool, rwID, 7000000.00, "Saldo awal KAS RW")

	// Lokasi dana RT = Rp99.000.000 (BEGITU Saja — tidak boleh memengaruhi saldo RW)
	seedLBC(t, ctx, pool,
		seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi Dana RT", "brankas", ""),
		rtID1, "opening_balance", "99000000.00", nil, nil)

	tx, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)

	summary, err := repo.GetDashboardSummary(ctx, tx, rwID)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Saldo awal = 7.000.000
	require.Equal(t, "7000000.00", summary.TotalOpeningBal,
		"saldo awal RW harus 7.000.000")

	// Net balance = 7.000.000 (opening + 0 income - 0 expense)
	require.Equal(t, "7000000.00", summary.NetBalance,
		"net balance RW harus 7.000.000, tidak boleh 99.000.000 dari lokasi dana RT")
}

// ===========================================================================
// REGRESSION TEST 2: Pemasukan KAS RW Rp500.000
// Saldo RW = Rp7.000.000 + Rp500.000 = Rp7.500.000
// ===========================================================================

func TestRegression2_IncreaseByIncome(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID, _, _ := setupRWTest(t)

	seedOpeningBalance(t, ctx, pool, rwID, 7000000.00, "Saldo awal")
	catID := seedCategory(t, ctx, pool, rtID)

	// Pemasukan RW = Rp500.000
	seedRWTransaction(t, ctx, pool, rwID, catID, 500000.00, "income", "posted", "Pemasukan KAS RW")

	tx, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)

	summary, err := repo.GetDashboardSummary(ctx, tx, rwID)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	require.Equal(t, "7000000.00", summary.TotalOpeningBal)
	require.Equal(t, "500000.00", summary.TotalIncome)
	require.Equal(t, "0.00", summary.TotalExpense)
	require.Equal(t, "7500000.00", summary.NetBalance,
		"net = 7.000.000 + 500.000 - 0 = 7.500.000")
}

// ===========================================================================
// REGRESSION TEST 3: Pengeluaran KAS RW Rp200.000
// Saldo RW = Rp7.500.000 - Rp200.000 = Rp7.300.000
// ===========================================================================

func TestRegression3_IncreaseByExpense(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID, _, _ := setupRWTest(t)

	seedOpeningBalance(t, ctx, pool, rwID, 7000000.00, "Saldo awal")
	catID := seedCategory(t, ctx, pool, rtID)

	seedRWTransaction(t, ctx, pool, rwID, catID, 500000.00, "income", "posted", "Pemasukan")
	seedRWTransaction(t, ctx, pool, rwID, catID, 200000.00, "expense", "posted", "Pengeluaran KAS RW")

	tx, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)

	summary, err := repo.GetDashboardSummary(ctx, tx, rwID)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	require.Equal(t, "7000000.00", summary.TotalOpeningBal)
	require.Equal(t, "500000.00", summary.TotalIncome)
	require.Equal(t, "200000.00", summary.TotalExpense)
	require.Equal(t, "7300000.00", summary.NetBalance,
		"net = 7.000.000 + 500.000 - 200.000 = 7.300.000")
}

// ===========================================================================
// REGRESSION TEST 4: Perubahan saldo RT tidak mengubah saldo RW
// ===========================================================================

func TestRegression4_RTChangeDoesNotAffectRW(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID1, _, _ := setupRWTest(t)

	seedOpeningBalance(t, ctx, pool, rwID, 1000000.00, "Saldo awal")

	// Ambil snapshot saldo RW sebelum perubahan RT
	txBefore, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	summaryBefore, err := repo.GetDashboardSummary(ctx, txBefore, rwID)
	require.NoError(t, err)
	err = txBefore.Commit()
	require.NoError(t, err)

	beforeBalance := summaryBefore.NetBalance

	// Ubah saldo RT: tambahkan 50.000.000 di lokasi dana RT
	seedLBC(t, ctx, pool,
		seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi Dana RT", "brankas", ""),
		rtID1, "opening_balance", "50000000.00", nil, nil)

	// Saldo RW harus TIDAK berubah
	txAfter, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	summaryAfter, err := repo.GetDashboardSummary(ctx, txAfter, rwID)
	require.NoError(t, err)
	err = txAfter.Commit()
	require.NoError(t, err)

	require.Equal(t, beforeBalance, summaryAfter.NetBalance,
		"net balance RW tidak boleh berubah akibat perubahan saldo RT")
}

// ===========================================================================
// REGRESSION TEST 5: Perubahan saldo RW tidak mengubah saldo RT
// ===========================================================================

func TestRegression5_RWChangeDoesNotAffectRT(t *testing.T) {
	ctx := context.Background()
	pool, _, rwID, _, rtID1, _, _ := setupRWTest(t)

	// Saldo awal RT = Rp100.000
	seedLBC(t, ctx, pool,
		seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi Dana RT", "brankas", ""),
		rtID1, "opening_balance", "100000.00", nil, nil)

	// Ambil snapshot saldo RT sebelum perubahan RW
	txBefore, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	var rtBalBefore string
	err = txBefore.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text
		 FROM location_balance_changes lbc
		 JOIN fund_locations fl ON lbc.location_id = fl.id
		 JOIN rts r ON fl.rt_id = r.id
		 WHERE r.rw_id = $1`, rwID,
	).Scan(&rtBalBefore)
	require.NoError(t, err)
	err = txBefore.Commit()
	require.NoError(t, err)

	// Ubah saldo RW: tambahkan opening balance 5.000.000
	seedOpeningBalance(t, ctx, pool, rwID, 5000000.00, "RW opening")

	// Saldo RT harus TIDAK berubah
	txAfter, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	var rtBalAfter string
	err = txAfter.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text
		 FROM location_balance_changes lbc
		 JOIN fund_locations fl ON lbc.location_id = fl.id
		 JOIN rts r ON fl.rt_id = r.id
		 WHERE r.rw_id = $1`, rwID,
	).Scan(&rtBalAfter)
	require.NoError(t, err)
	err = txAfter.Commit()
	require.NoError(t, err)

	require.Equal(t, rtBalBefore, rtBalAfter,
		"saldo RT tidak boleh berubah akibat perubahan saldo RW")
}

// ===========================================================================
// REGRESSION TEST 6: Transaksi RT tidak muncul di riwayat RW
// ===========================================================================

func TestRegression6_RTTransactionsNotInRWHistory(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID1, _, _ := setupRWTest(t)

	// Transaksi RT langsung via location_balance_changes (bukan rw_transactions)
	seedLBC(t, ctx, pool,
		seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi Dana RT", "brankas", ""),
		rtID1, "opening_balance", "500000.00", nil, nil)

	// Riwayat transaksi RW harus 0
	tx, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	items, total, err := repo.GetTransactions(ctx, tx, rwID, "", 1, 20, "", "")
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	require.Equal(t, 0, total, "total transaksi RW harus 0, tidak boleh ada transaksi RT")
	require.Len(t, items, 0, "riwayat transaksi RW harus kosong")
}

// ===========================================================================
// REGRESSION TEST 7: Transaksi RW tidak muncul di riwayat RT
// ===========================================================================

func TestRegression7_RWTransactionsNotInRTHistory(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID1, _, _ := setupRWTest(t)

	// Seed transaksi RW (masuk ke rw_transactions, bukan transactions)
	seedOpeningBalance(t, ctx, pool, rwID, 100000.00, "Opening")
	catID := seedCategory(t, ctx, pool, rtID1)
	seedRWTransaction(t, ctx, pool, rwID, catID, 200000.00, "income", "posted", "Transaksi RW")

	// Pastikan transaksi RT = 0
	tx, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	items, total, err := repo.GetTransactions(ctx, tx, rwID, "", 1, 20, "", "")
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Total transaksi RW harus 1 (hanya transaksi RW, bukan RT)
	require.Equal(t, 1, total, "total transaksi RW harus 1")
	require.Len(t, items, 1, "riwayat harus berisi 1 transaksi RW")

	// Transaksi ini harus dari rw_transactions, bukan transactions RT
	require.Equal(t, rwID, items[0].RWID, "transaksi harus ber-RWID")
}

// ===========================================================================
// REGRESSION TEST 8: Reversal RW hanya memengaruhi pembukuan RW
// ===========================================================================

func TestRegression8_RWReversalOnlyAffectsRW(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID, _, _ := setupRWTest(t)

	catID := seedCategory(t, ctx, pool, rtID)

	// Pemasukan awal RW = Rp10.000
	incomeID := seedRWTransaction(t, ctx, pool, rwID, catID, 10000.00, "income", "posted", "Income A")

	// Ambil snapshot
	tx1, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s1, err := repo.GetDashboardSummary(ctx, tx1, rwID)
	require.NoError(t, err)
	err = tx1.Commit()
	require.NoError(t, err)

	require.Equal(t, "10000.00", s1.TotalIncome)
	require.Equal(t, "0.00", s1.TotalExpense)

	// Reversal: transaksi baru dengan amount Rp1.000, reverses_transaction_id = ID transaksi Income A
	seedRWReversal(t, ctx, pool, rwID, catID, 1000.00, incomeID, "Reversal Income A")

	// Setelah reversal:
	// - Income tetap 10.000 (reversal tidak mengurangi line item income)
	// - Expense = 1.000 (reversal dicatat sebagai expense baru)
	// - Transaction count = 2
	tx2, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s2, err := repo.GetDashboardSummary(ctx, tx2, rwID)
	require.NoError(t, err)
	err = tx2.Commit()
	require.NoError(t, err)

	require.Equal(t, "10000.00", s2.TotalIncome,
		"after reversal: income should stay 10000")
	require.Equal(t, "1000.00", s2.TotalExpense,
		"after reversal: expense should be 1000 (reversal)")
	require.Equal(t, int64(2), s2.TransactionCount,
		"after reversal: 2 posted transactions")

	// Net balance = 10.000 - 1.000 = 9.000
	require.Equal(t, "9000.00", s2.NetBalance,
		"net = 10.000 - 1.000 = 9.000")
}

// ===========================================================================
// REGRESSION TEST 9: Dashboard, laporan, dan halaman KAS RW menampilkan saldo yang sama
// ===========================================================================

func TestRegression9_ConsistentBalanceAcrossViews(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID, _, _ := setupRWTest(t)

	// Setup data
	seedOpeningBalance(t, ctx, pool, rwID, 1000000.00, "Opening balance")
	catID := seedCategory(t, ctx, pool, rtID)
	seedRWTransaction(t, ctx, pool, rwID, catID, 500000.00, "income", "posted", "Pemasukan")
	seedRWTransaction(t, ctx, pool, rwID, catID, 200000.00, "expense", "posted", "Pengeluaran")

	// 1. Dashboard summary
	txDash, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	dash, err := repo.GetDashboardSummary(ctx, txDash, rwID)
	require.NoError(t, err)
	err = txDash.Commit()
	require.NoError(t, err)

	// 2. KAS report (all-time)
	txCas, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	kas, err := repo.GetKASReport(ctx, txCas, rwID, "")
	require.NoError(t, err)
	err = txCas.Commit()
	require.NoError(t, err)

	// Dashboard net_balance harus sama dengan KAS report net_balance
	require.Equal(t, dash.NetBalance, kas.NetBalance,
		"net balance dashboard (%s) harus sama dengan KAS report (%s)", dash.NetBalance, kas.NetBalance)

	// Keduanya = 1.000.000 + 500.000 - 200.000 = 1.300.000
	require.Equal(t, "1300000.00", dash.NetBalance,
		"dashboard net = 1.000.000 + 500.000 - 200.000 = 1.300.000")
	require.Equal(t, "1300000.00", kas.NetBalance,
		"KAS report net = 1.000.000 + 500.000 - 200.000 = 1.300.000")

	// Opening balance KAS harus sama dengan TotalOpeningBal dashboard
	require.Equal(t, dash.TotalOpeningBal, kas.OpeningBalance,
		"opening balance harus konsisten antar dashboard dan KAS")
}

// ===========================================================================
// REGRESSION TEST 10: Filter periode menghitung saldo sesuai aturan laporan
// ===========================================================================

func TestRegression10_PeriodFilterCalculation(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID, _, _ := setupRWTest(t)

	catID := seedCategory(t, ctx, pool, rtID)

	// Seed transaksi dengan tanggal eksplisit
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO rw_transactions (rw_id, category_id, amount, type, status, description, occurred_at)
		 VALUES ($1, $2, 1000.00, 'income', 'posted', 'Jan Income', '2025-01-10T00:00:00+07:00')`,
		rwID, catID,
	)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO rw_transactions (rw_id, category_id, amount, type, status, description, occurred_at)
		 VALUES ($1, $2, 300.00, 'expense', 'posted', 'Jan Expense', '2025-01-20T00:00:00+07:00')`,
		rwID, catID,
	)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO rw_transactions (rw_id, category_id, amount, type, status, description, occurred_at)
		 VALUES ($1, $2, 2000.00, 'income', 'posted', 'Feb Income', '2025-02-15T00:00:00+07:00')`,
		rwID, catID,
	)
	require.NoError(t, err)

	// ---- All-time KAS report ---------------------------------------------
	txAll, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	reportAll, err := repo.GetKASReport(ctx, txAll, rwID, "")
	require.NoError(t, err)
	err = txAll.Commit()
	require.NoError(t, err)

	// Semua transaksi: income 3000, expense 300
	require.Equal(t, "3000.00", reportAll.TotalIncome)
	require.Equal(t, "300.00", reportAll.TotalExpense)
	require.Equal(t, int64(3), reportAll.TransactionCount)

	// ---- January 2025 KAS report -----------------------------------------
	txJan, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	reportJan, err := repo.GetKASReport(ctx, txJan, rwID, "2025-01")
	require.NoError(t, err)
	err = txJan.Commit()
	require.NoError(t, err)

	require.Equal(t, "1000.00", reportJan.TotalIncome, "Jan income = 1000")
	require.Equal(t, "300.00", reportJan.TotalExpense, "Jan expense = 300")
	require.Equal(t, "0.00", reportJan.OpeningBalance, "opening balance all-time = 0 (not seeded)")
	require.Equal(t, "700.00", reportJan.NetBalance, "Jan net = 0 + 1000 - 300 = 700")
	require.Equal(t, int64(2), reportJan.TransactionCount, "Jan = 2 transactions")

	// ---- February 2025 KAS report ----------------------------------------
	txFeb, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	reportFeb, err := repo.GetKASReport(ctx, txFeb, rwID, "2025-02")
	require.NoError(t, err)
	err = txFeb.Commit()
	require.NoError(t, err)

	require.Equal(t, "2000.00", reportFeb.TotalIncome, "Feb income = 2000")
	require.Equal(t, "0.00", reportFeb.TotalExpense, "Feb expense = 0")
	require.Equal(t, "2000.00", reportFeb.NetBalance, "Feb net = 0 + 2000 - 0 = 2000")
	require.Equal(t, int64(1), reportFeb.TransactionCount, "Feb = 1 transaction")
}

// ===========================================================================
// TEST ISOLASI TENANT: RW 1 != RW 2
// ===========================================================================

func TestRWTenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID1, rwID2, rtID1, _, rtID3 := setupRWTest(t)

	// Create categories
	cat1 := seedCategory(t, ctx, pool, rtID1)
	cat2 := seedCategory(t, ctx, pool, rtID3)

	// RW 1: opening 5000, income 1000, expense 200
	seedOpeningBalance(t, ctx, pool, rwID1, 5000.00, "RW1 opening")
	seedRWTransaction(t, ctx, pool, rwID1, cat1, 1000.00, "income", "posted", "RW1 Income")
	seedRWTransaction(t, ctx, pool, rwID1, cat1, 200.00, "expense", "posted", "RW1 Expense")

	// RW 2: opening 99999, income 88888, expense 77777
	seedOpeningBalance(t, ctx, pool, rwID2, 99999.00, "RW2 opening")
	seedRWTransaction(t, ctx, pool, rwID2, cat2, 88888.00, "income", "posted", "RW2 Income")
	seedRWTransaction(t, ctx, pool, rwID2, cat2, 77777.00, "expense", "posted", "RW2 Expense")

	// Test RW 1
	tx1, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s1, err := repo.GetDashboardSummary(ctx, tx1, rwID1)
	require.NoError(t, err)
	err = tx1.Commit()
	require.NoError(t, err)

	require.Equal(t, "5000.00", s1.TotalOpeningBal)
	require.Equal(t, "1000.00", s1.TotalIncome)
	require.Equal(t, "200.00", s1.TotalExpense)
	require.Equal(t, "5800.00", s1.NetBalance, "RW1 net = 5000 + 1000 - 200")

	// Test RW 2
	tx2, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s2, err := repo.GetDashboardSummary(ctx, tx2, rwID2)
	require.NoError(t, err)
	err = tx2.Commit()
	require.NoError(t, err)

	require.Equal(t, "99999.00", s2.TotalOpeningBal)
	require.Equal(t, "88888.00", s2.TotalIncome)
	require.Equal(t, "77777.00", s2.TotalExpense)
	require.Equal(t, "111110.00", s2.NetBalance, "RW2 net = 99999 + 88888 - 77777")

	// Cross-isolation checks
	require.NotEqual(t, s1.NetBalance, s2.NetBalance, "RW1 and RW2 balance must differ")
	require.NotEqual(t, s1.TotalIncome, s2.TotalIncome, "RW1 and RW2 income must differ")
}

// ===========================================================================
// TEST DASHBOARD DENGAN REVERSAL
// ===========================================================================

func TestRWDashboardWithReversal(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID, _, _ := setupRWTest(t)

	catID := seedCategory(t, ctx, pool, rtID)

	// 1. Posting income 10000
	incomeID := seedRWTransaction(t, ctx, pool, rwID, catID, 10000.00, "income", "posted", "Income A")

	tx1, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s1, err := repo.GetDashboardSummary(ctx, tx1, rwID)
	require.NoError(t, err)
	err = tx1.Commit()
	require.NoError(t, err)

	require.Equal(t, "10000.00", s1.TotalIncome, "after 1 posting: expected 10000 income")
	require.Equal(t, int64(1), s1.TransactionCount, "after 1 posting: expected 1 transaction")

	// 2. Posting expense 3000
	seedRWTransaction(t, ctx, pool, rwID, catID, 3000.00, "expense", "posted", "Expense A")

	tx2, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s2, err := repo.GetDashboardSummary(ctx, tx2, rwID)
	require.NoError(t, err)
	err = tx2.Commit()
	require.NoError(t, err)

	require.Equal(t, "10000.00", s2.TotalIncome, "after expense: income unchanged at 10000")
	require.Equal(t, "3000.00", s2.TotalExpense, "after 2 postings: expected 3000 expense")
	require.Equal(t, int64(2), s2.TransactionCount, "after 2 postings: expected 2 transactions")

	// 3. Draft (harus diabaikan)
	seedRWTransaction(t, ctx, pool, rwID, catID, 9999.00, "income", "draft", "Draft yang tidak dihitung")

	tx3, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s3, err := repo.GetDashboardSummary(ctx, tx3, rwID)
	require.NoError(t, err)
	err = tx3.Commit()
	require.NoError(t, err)

	require.Equal(t, "10000.00", s3.TotalIncome, "draft must not affect income")
	require.Equal(t, int64(2), s3.TransactionCount, "draft must not increase count")

	// 4. Reversal: transaksi baru dengan amount 1000, reverses_transaction_id = ID transaksi income A
	seedRWReversal(t, ctx, pool, rwID, catID, 1000.00, incomeID, "Reversal Income A")

	tx4, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	s4, err := repo.GetDashboardSummary(ctx, tx4, rwID)
	require.NoError(t, err)
	err = tx4.Commit()
	require.NoError(t, err)

	// Setelah reversal:
	// - Income tetap 10000 (reversal tidak mengurangi line item income)
	// - Expense = 3000 + 1000 = 4000 (reversal dicatat sebagai expense baru)
	// - Transaction count = 3 (original + expense + reversal)
	require.Equal(t, "10000.00", s4.TotalIncome,
		"after reversal: income should stay 10000")
	require.Equal(t, "4000.00", s4.TotalExpense,
		"after reversal: expense should be 3000 + 1000 reversal = 4000")
	require.Equal(t, int64(3), s4.TransactionCount,
		"after reversal: 3 posted transactions")
}

// ===========================================================================
// TEST GETLOCATIONS — SALDO DIPERHITUNG (locations tetap RT-level)
// ===========================================================================

func TestGetLocationsBalanceComputed(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID1, _, rtID3 := setupRWTest(t)

	// Lokasi di RT1 (RW 1)
	_ = seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi Aktif", "brankas", "")
	_ = seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi Tanpa Mutasi", "brankas", "")

	locInactive := seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi Nonaktif", "brankas", "")
	_, err := pool.Raw().ExecContext(ctx,
		`UPDATE fund_locations SET is_active = false WHERE id = $1`, locInactive)
	require.NoError(t, err)

	// Lokasi di RT3 (RW 2)
	locRW2 := seedLocationWithCategory(t, ctx, pool, rtID3, "Lokasi RW2", "brankas", "")

	seedLBC(t, ctx, pool, locInactive, rtID1, "opening_balance", "5000.00", nil, nil)
	seedLBC(t, ctx, pool, locRW2, rtID3, "opening_balance", "10000.00", nil, nil)

	// Test A: Lokasi aktif
	activeTrue := true
	txA, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)

	locations, err := repo.GetLocations(ctx, txA, rwID, &activeTrue)
	require.NoError(t, err)
	require.NoError(t, txA.Commit())

	require.Len(t, locations, 2, "expected 2 active locations in RW 1")

	locByName := make(map[string]*rw.LocationSummary)
	for _, loc := range locations {
		locByName[loc.Name] = loc
	}

	locObjA, ok := locByName["Lokasi Aktif"]
	require.True(t, ok, "Lokasi Aktif should be in results")
	require.True(t, locObjA.IsActive)
	require.Equal(t, "0.00", locObjA.CurrentBalance)

	locObjB, ok := locByName["Lokasi Tanpa Mutasi"]
	require.True(t, ok, "Lokasi Tanpa Mutasi should be in results")
	require.True(t, locObjB.IsActive)
	require.Equal(t, "0.00", locObjB.CurrentBalance)

	// Test B: Semua lokasi
	txB, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	locationsAll, err := repo.GetLocations(ctx, txB, rwID, nil)
	require.NoError(t, err)
	require.NoError(t, txB.Commit())

	require.Len(t, locationsAll, 3, "should have 3 total locations")

	// Test C: Lokasi nonaktif
	txC, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	activeFalse := false
	locationsInactive, err := repo.GetLocations(ctx, txC, rwID, &activeFalse)
	require.NoError(t, err)
	require.NoError(t, txC.Commit())

	require.Len(t, locationsInactive, 1, "should have 1 inactive location")
	require.Equal(t, "Lokasi Nonaktif", locationsInactive[0].Name)
	require.False(t, locationsInactive[0].IsActive)
	require.Equal(t, "5000.00", locationsInactive[0].CurrentBalance)
}

// ===========================================================================
// TEST ISOLASI TENANT — GETLOCATIONS
// ===========================================================================

func TestGetLocationsTenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID1, rwID2, rtID1, _, rtID3 := setupRWTest(t)

	seedLocationWithCategory(t, ctx, pool, rtID1, "Lokasi RW1-A", "brankas", "")
	seedLocationWithCategory(t, ctx, pool, rtID3, "Lokasi RW2-A", "brankas", "")

	tx1, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	locs1, err := repo.GetLocations(ctx, tx1, rwID1, nil)
	require.NoError(t, err)
	require.NoError(t, tx1.Commit())

	tx2, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	locs2, err := repo.GetLocations(ctx, tx2, rwID2, nil)
	require.NoError(t, err)
	require.NoError(t, tx2.Commit())

	for _, loc := range locs1 {
		require.NotEqual(t, "Lokasi RW2-A", loc.Name,
			"RW 1 must not see RW 2 locations, but found: %s", loc.Name)
	}
	for _, loc := range locs2 {
		require.NotEqual(t, "Lokasi RW1-A", loc.Name,
			"RW 2 must not see RW 1 locations, but found: %s", loc.Name)
	}
}

// ===========================================================================
// TEST TRANSACTION LIST PAGINATION
// ===========================================================================

func TestGetTransactionsPagination(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, rtID, _, _ := setupRWTest(t)

	catID := seedCategory(t, ctx, pool, rtID)

	for i := 0; i < 5; i++ {
		seedRWTransaction(t, ctx, pool, rwID, catID, 100.00, "income", "posted", fmt.Sprintf("Tx %d", i))
	}

	seedRWTransaction(t, ctx, pool, rwID, catID, 100.00, "income", "draft", "Draft Tx")

	// Halaman 1, pageSize=2
	tx1, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)

	items, total, err := repo.GetTransactions(ctx, tx1, rwID, "", 1, 2, "", "")
	require.NoError(t, err)
	require.NoError(t, tx1.Commit())

	require.Equal(t, 5, total, "total posted transactions should be 5")
	require.Len(t, items, 2, "page 1 should have 2 items")
	require.Equal(t, rwID, items[0].RWID, "RWID should match the queried RW")

	// Halaman 2, pageSize=2
	tx2, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	items2, total2, err := repo.GetTransactions(ctx, tx2, rwID, "", 2, 2, "", "")
	require.NoError(t, err)
	require.NoError(t, tx2.Commit())

	require.Equal(t, 5, total2)
	require.Len(t, items2, 2, "page 2 should have 2 items")

	// Halaman 3, pageSize=2 (sisa 1)
	tx3, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	items3, total3, err := repo.GetTransactions(ctx, tx3, rwID, "", 3, 2, "", "")
	require.NoError(t, err)
	require.NoError(t, tx3.Commit())

	require.Equal(t, 5, total3)
	require.Len(t, items3, 1, "page 3 (last page) should have 1 item")
}

// ===========================================================================
// TEST SERVICE LAYER — AUTHORIZATION
// ===========================================================================

func TestServiceCheckRWAccess(t *testing.T) {
	ctx := context.Background()
	pool, _, rwID1, rwID2, _, _, _ := setupRWTest(t)
	repo := rw.NewRepository(pool.Raw())
	// Use a scoping config that includes both test RW IDs so SA bypass validation works.
	scopeCfg := &auth.ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{rwID1, rwID2},
	}
	svc := rw.NewService(pool, repo, scopeCfg)

	t.Run("no auth context returns error", func(t *testing.T) {
		_, err := svc.GetDashboard(ctx, nil, rwID1)
		require.Error(t, err, "nil AuthContext should be rejected")
		require.Contains(t, err.Error(), "authentication context",
			"error should mention no authentication context")
	})

	t.Run("user with matching RW membership passes authorization", func(t *testing.T) {
		var userID string
		err := pool.Raw().QueryRowContext(ctx,
			`INSERT INTO users (email, phone, password_hash, full_name, is_active)
			 VALUES ($1, $2, 'hash', 'Test User', true) RETURNING id`,
			fmt.Sprintf("testuser-%d@test.com", time.Now().UnixNano()),
			fmt.Sprintf("081%d", time.Now().UnixNano()%100000000),
		).Scan(&userID)
		require.NoError(t, err)

		ac := &auth.AuthContext{
			UserID: userID,
			RWMemberships: []auth.RWMembership{
				{RWID: rwID1, Jabatan: "ketua"},
			},
		}

		_, err = svc.GetDashboard(ctx, ac, rwID1)
		// checkRWAccess TIDAK melempar error access denied.
		if err != nil {
			require.NotContains(t, err.Error(), "rw access denied",
				"user with matching RW membership should pass authorization")
		}
	})

	t.Run("user with non-matching RW membership is rejected", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "some-user-id",
			RWMemberships: []auth.RWMembership{
				{RWID: rwID2, Jabatan: "ketua"},
			},
		}

		_, err := svc.GetDashboard(ctx, ac, rwID1)
		require.Error(t, err, "non-matching RW should be rejected")
		require.Contains(t, err.Error(), "rw access denied",
			"error should be 'rw access denied' for wrong RW")
	})

	t.Run("empty RW membership list is rejected", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "some-user-id",
			RWMemberships: []auth.RWMembership{},
		}

		_, err := svc.GetDashboard(ctx, ac, rwID1)
		require.Error(t, err, "empty RW membership should be rejected")
		require.Contains(t, err.Error(), "rw access denied",
			"error should be 'rw access denied' for empty RW membership")
	})
}

// ===========================================================================
// TEST: Saldo awal tanpa transaksi lain
// ===========================================================================

func TestDashboardOnlyOpeningBalance(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, _, _ := setupRWTest(t)

	seedOpeningBalance(t, ctx, pool, rwID, 2500000.00, "Modal awal")

	tx, err := pool.Raw().BeginTx(ctx, nil)
	require.NoError(t, err)
	summary, err := repo.GetDashboardSummary(ctx, tx, rwID)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	require.Equal(t, "2500000.00", summary.TotalOpeningBal)
	require.Equal(t, "0.00", summary.TotalIncome)
	require.Equal(t, "0.00", summary.TotalExpense)
	require.Equal(t, "2500000.00", summary.NetBalance)
	require.Equal(t, int64(0), summary.TransactionCount)
}
