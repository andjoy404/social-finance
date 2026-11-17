package rw_test

// Laporan KAS RW backend behaviour:
//   - carry-forward opening balance per period (RW opening balance + net posted movements strictly
//     before the window)
//   - window boundaries (period, whole-day inclusive date range, same-day allowed)
//   - posted-only filtering (draft/cancelled excluded everywhere)
//   - legitimate RT setoran income is counted (NOT excluded)
//   - transfers between fund locations are never counted as income/expense
//   - RW isolation
//   - additive type / category_id filters on the transactions endpoint

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	rw "social-finance/internal/rw"
)

func seedRWTxAt(t *testing.T, ctx context.Context, pool *database.Pool, rwID, categoryID, amount, txnType, status, description, occurredAt string, sourceRTID *string) string {
	t.Helper()
	if sourceRTID != nil {
		var txID string
		err := pool.Raw().QueryRowContext(ctx,
			`INSERT INTO rw_transactions (rw_id, category_id, amount, type, status, description, occurred_at, source_rt_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			rwID, categoryID, amount, txnType, status, description, occurredAt, *sourceRTID,
		).Scan(&txID)
		require.NoError(t, err)
		return txID
	}
	var txID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rw_transactions (rw_id, category_id, amount, type, status, description, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		rwID, categoryID, amount, txnType, status, description, occurredAt,
	).Scan(&txID)
	require.NoError(t, err)
	return txID
}

func kasReportCarry(t *testing.T, ctx context.Context, pool *database.Pool, repo *rw.Repository, rwID string, startTime, endTime *time.Time) *rw.KASReport {
	t.Helper()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	report, err := repo.GetKASReportCarry(ctx, tx, rwID, startTime, endTime)
	require.NoError(t, err)
	return report
}

func jakartaTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	require.NoError(t, err)
	tm, err := time.ParseInLocation(layout, value, loc)
	require.NoError(t, err)
	return tm
}

func createReportUser(t *testing.T, ctx context.Context, pool *database.Pool) string {
	t.Helper()
	var userID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'hash', 'Bendahara Kas Report', true) RETURNING id`,
		fmt.Sprintf("kas-report-%s@test.com", uuid.New().String()[:8]),
		fmt.Sprintf("08%d", time.Now().UnixNano()%100000000),
	).Scan(&userID)
	require.NoError(t, err)
	return userID
}

func bendaharaAuth(rwID, userID string) *auth.AuthContext {
	return &auth.AuthContext{
		UserID: userID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "bendahara"},
		},
	}
}

func TestKASReportCarryForward_OpeningAcrossPeriods(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	ctx := context.Background()

	catID := seedCategory(t, ctx, pool, rtID1)
	seedOpeningBalance(t, ctx, pool, rwID1, "1000000.00", "Modal awal RW")

	// September 2026 movements
	seedRWTxAt(t, ctx, pool, rwID1, catID, "250000.00", "income", "posted", "September income", "2026-09-10T09:30:00+07:00", nil)
	seedRWTxAt(t, ctx, pool, rwID1, catID, "50000.00", "expense", "posted", "September expense", "2026-09-20T09:30:00+07:00", nil)

	// --- Period A (September): opening = inception only ---
	sepStart := jakartaTime(t, "2006-01-02", "2026-09-01")
	sepEnd := jakartaTime(t, "2006-01-02", "2026-10-01")
	rSep := kasReportCarry(t, ctx, pool, repo, rwID1, &sepStart, &sepEnd)
	require.Equal(t, "1000000.00", rSep.OpeningBalance)
	require.Equal(t, "250000.00", rSep.TotalIncome)
	require.Equal(t, "50000.00", rSep.TotalExpense)
	require.Equal(t, "1200000.00", rSep.TotalBalance)
	require.EqualValues(t, 2, rSep.TransactionCount)

	// --- Period B (October): opening carries forward September's net (+200000) ---
	octStart := jakartaTime(t, "2006-01-02", "2026-10-01")
	octEnd := jakartaTime(t, "2006-01-02", "2026-11-01")
	rOct := kasReportCarry(t, ctx, pool, repo, rwID1, &octStart, &octEnd)
	require.Equal(t, "1200000.00", rOct.OpeningBalance, "opening for October must include September's posted net movement")
	require.Equal(t, "0.00", rOct.TotalIncome)
	require.Equal(t, "0.00", rOct.TotalExpense)
	require.Equal(t, "1200000.00", rOct.TotalBalance)
	require.EqualValues(t, 0, rOct.TransactionCount)
}

func TestKASReportCarryForward_WindowBoundaries_WholeDaysInclusive(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	ctx := context.Background()

	catID := seedCategory(t, ctx, pool, rtID1)

	start := jakartaTime(t, "2006-01-02", "2026-10-05")
	// Same instant as start boundary (inclusive).
	seedRWTxAt(t, ctx, pool, rwID1, catID, "100000.00", "income", "posted", "Boundary start", "2026-10-05T00:00:00+07:00", nil)
	// Middle of the window.
	seedRWTxAt(t, ctx, pool, rwID1, catID, "200000.00", "income", "posted", "Mid window", "2026-10-05T14:30:00+07:00", nil)
	// Just before end boundary (exclusive at next day 00:00).
	seedRWTxAt(t, ctx, pool, rwID1, catID, "400000.00", "income", "posted", "Last instant", "2026-10-05T23:30:00+07:00", nil)
	// On the following day — must be EXCLUDED from the same-day window.
	seedRWTxAt(t, ctx, pool, rwID1, catID, "800000.00", "income", "posted", "Next day", "2026-10-06T00:00:00+07:00", nil)

	end := jakartaTime(t, "2006-01-02", "2026-10-06") // window [Oct 5 00:00, Oct 6 00:00)
	r := kasReportCarry(t, ctx, pool, repo, rwID1, &start, &end)
	require.Equal(t, "700000.00", r.TotalIncome, "window must include the start boundary and all of Oct 5, exclude Oct 6")

	// Whole-day end_date semantics: window [Oct 5, Oct 6] (inclusive both ends) means end bound = Oct 7.
	r2 := kasReportCarry(t, ctx, pool, repo, rwID1, &start, func() *time.Time {
		et := jakartaTime(t, "2006-01-02", "2026-10-07")
		return &et
	}())
	require.Equal(t, "1500000.00", r2.TotalIncome, "inclusive whole-day window must cover Oct 5 and Oct 6")

	// Transaction strictly before window contributes to opening, not to window income.
	prior := jakartaTime(t, "2006-01-02", "2026-10-01")
	priorEnd := jakartaTime(t, "2006-01-02", "2026-10-05")
	_ = kasReportCarry(t, ctx, pool, repo, rwID1, &prior, &priorEnd) // seed state unchanged; nothing asserted here other than it computes

	octEnd := jakartaTime(t, "2006-01-02", "2026-11-01")
	// Full-month window: opening carries prior (inception=0 + Oct-01..Oct-04 window net = 0), income = whole month.
	r3 := kasReportCarry(t, ctx, pool, repo, rwID1, &prior, &octEnd)
	require.Equal(t, "1500000.00", r3.TotalIncome)
	require.Equal(t, "0.00", r3.OpeningBalance)
}

func TestKASReportCarryForward_PostedOnly(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	ctx := context.Background()

	catID := seedCategory(t, ctx, pool, rtID1)
	seedOpeningBalance(t, ctx, pool, rwID1, "100000.00", "Modal awal")

	// Posted: counts everywhere.
	seedRWTxAt(t, ctx, pool, rwID1, catID, "300000.00", "income", "posted", "Posted income", "2026-08-10T09:00:00+07:00", nil)
	// Draft/cancelled: ignored everywhere, including carry-forward opening.
	seedRWTxAt(t, ctx, pool, rwID1, catID, "999000.00", "income", "draft", "Draft income", "2026-08-11T09:00:00+07:00", nil)
	seedRWTxAt(t, ctx, pool, rwID1, catID, "888000.00", "income", "cancelled", "Cancelled income", "2026-08-12T09:00:00+07:00", nil)
	seedRWTxAt(t, ctx, pool, rwID1, catID, "777000.00", "expense", "draft", "Draft expense", "2026-08-13T09:00:00+07:00", nil)

	start := jakartaTime(t, "2006-01-02", "2026-09-01")
	end := jakartaTime(t, "2006-01-02", "2026-10-01")
	r := kasReportCarry(t, ctx, pool, repo, rwID1, &start, &end)

	require.Equal(t, "400000.00", r.OpeningBalance, "carry-forward opening must include only posted movements (100000 + 300000)")
	require.Equal(t, "0.00", r.TotalIncome, "no posted transactions in September")
	require.Equal(t, "0.00", r.TotalExpense)
	require.EqualValues(t, 0, r.TransactionCount)
}

func TestKASReportCarryForward_SetoranRTCountedAsIncome(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	ctx := context.Background()

	catID := seedCategory(t, ctx, pool, rtID1)
	seedOpeningBalance(t, ctx, pool, rwID1, "1000000.00", "Modal awal")

	// Legitimate setoran RT -> RW income (source_rt_id IS NOT NULL). Must be counted.
	rtID := rtID1
	seedRWTxAt(t, ctx, pool, rwID1, catID, "250000.00", "income", "posted",
		"Setoran iuran warga dari RT 001 ke Kas RW", "2026-10-05T09:00:00+07:00", &rtID)

	start := jakartaTime(t, "2006-01-02", "2026-10-01")
	end := jakartaTime(t, "2006-01-02", "2026-11-01")
	r := kasReportCarry(t, ctx, pool, repo, rwID1, &start, &end)
	require.Equal(t, "250000.00", r.TotalIncome, "setoran RT yang sah tetap dihitung sebagai penerimaan KAS RW")
	require.Equal(t, "250000.00", r.NetBalance)
	require.Equal(t, "1250000.00", r.TotalBalance)
	require.EqualValues(t, 1, r.TransactionCount)
}

func TestKASReportCarryForward_TransfersNotCountedAsMovements(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	ctx := context.Background()

	svc := rw.NewService(pool, repo, &auth.ScopingConfig{RWIDs: []string{rwID1}})
	userID := createReportUser(t, ctx, pool)
	bauth := bendaharaAuth(rwID1, userID)
	catID := seedCategory(t, ctx, pool, rtID1)

	// Fund the rekening location with a legitimate income tx, mirroring the
	// side effects of a real deposit (LBC row).
	incomeID := seedRWTxAt(t, ctx, pool, rwID1, catID, "500000.00", "income", "posted", "Saldo kas", "2026-10-01T09:00:00+07:00", nil)
	locRekening := seedRWFundLocation(t, ctx, pool, rwID1, "Rekening Operasional", "rekening", "BCA")
	locBrankas := seedRWFundLocation(t, ctx, pool, rwID1, "Brankas RW", "brankas", "")
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (rw_id, location_id, change_type, amount_delta, transaction_id)
		 VALUES ($1, $2, 'income', '500000.00', $3)`, rwID1, locRekening, incomeID)
	require.NoError(t, err)

	start := jakartaTime(t, "2006-01-02", "2026-10-01")
	end := jakartaTime(t, "2006-01-02", "2026-11-01")

	before := kasReportCarry(t, ctx, pool, repo, rwID1, &start, &end)
	require.Equal(t, "500000.00", before.TotalIncome)

	// Move money between fund locations. This is a transfer, NOT income/expense.
	_, err = svc.CreateTransfer(ctx, bauth, rwID1, rw.CreateFundTransferInput{
		FromLocationID: locRekening,
		ToLocationID:   locBrankas,
		Amount:         "200000.00",
	})
	require.NoError(t, err)

	after := kasReportCarry(t, ctx, pool, repo, rwID1, &start, &end)
	require.Equal(t, before.TotalIncome, after.TotalIncome, "transfer antar lokasi tidak boleh menambah penerimaan")
	require.Equal(t, before.TotalExpense, after.TotalExpense, "transfer antar lokasi tidak boleh menambah pengeluaran")
	require.Equal(t, before.TotalBalance, after.TotalBalance)
	require.Equal(t, before.TransactionCount, after.TransactionCount)
}

func TestKASReportCarryForward_IsolationBetweenRWs(t *testing.T) {
	pool, repo, rwID1, rwID2, rtID1, _, _ := setupRWTest(t)
	ctx := context.Background()

	catID := seedCategory(t, ctx, pool, rtID1)
	seedOpeningBalance(t, ctx, pool, rwID1, "100000.00", "Opening RW1")
	seedRWTxAt(t, ctx, pool, rwID1, catID, "250000.00", "income", "posted", "RW1 income", "2026-10-05T09:00:00+07:00", nil)
	// RW2 untouched: nothing seeded here.

	start := jakartaTime(t, "2006-01-02", "2026-10-01")
	end := jakartaTime(t, "2006-01-02", "2026-11-01")
	r1 := kasReportCarry(t, ctx, pool, repo, rwID1, &start, &end)
	r2 := kasReportCarry(t, ctx, pool, repo, rwID2, &start, &end)
	require.Equal(t, "350000.00", r1.TotalBalance)
	require.Equal(t, "0.00", r2.OpeningBalance)
	require.Equal(t, "0.00", r2.TotalIncome)
	require.Equal(t, "0.00", r2.TotalBalance)
	require.EqualValues(t, 0, r2.TransactionCount)
}

func TestKASReportCarryForward_AllTime(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	ctx := context.Background()

	catID := seedCategory(t, ctx, pool, rtID1)
	seedOpeningBalance(t, ctx, pool, rwID1, "700000.00", "Opening")
	seedRWTxAt(t, ctx, pool, rwID1, catID, "100000.00", "income", "posted", "Old income", "2025-01-02T09:00:00+07:00", nil)
	seedRWTxAt(t, ctx, pool, rwID1, catID, "40000.00", "expense", "posted", "Old expense", "2025-01-03T09:00:00+07:00", nil)

	// No window bounds: cumulative all-time.
	r := kasReportCarry(t, ctx, pool, repo, rwID1, nil, nil)
	require.Equal(t, "700000.00", r.OpeningBalance)
	require.Equal(t, "100000.00", r.TotalIncome)
	require.Equal(t, "40000.00", r.TotalExpense)
	require.Equal(t, "760000.00", r.TotalBalance)
	require.EqualValues(t, 2, r.TransactionCount)
}

func TestGetTransactions_TypeAndCategoryFilters(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	svc := rw.NewService(pool, repo, &auth.ScopingConfig{RWIDs: []string{rwID1}})
	ctx := context.Background()
	userID := createReportUser(t, ctx, pool)
	bauth := bendaharaAuth(rwID1, userID)

	catIncome := seedCategory(t, ctx, pool, rtID1)
	var catExpense string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type) VALUES ($1, 'Operasional Uji', 'expense') RETURNING id`, rtID1).Scan(&catExpense)
	require.NoError(t, err)

	seedRWTxAt(t, ctx, pool, rwID1, catIncome, "111000.00", "income", "posted", "Pemasukan A", "2026-10-01T09:00:00+07:00", nil)
	seedRWTxAt(t, ctx, pool, rwID1, catExpense, "222000.00", "expense", "posted", "Pengeluaran A", "2026-10-02T09:00:00+07:00", nil)

	// All (legacy contract): returns both rows.
	all, total, err := svc.GetTransactions(ctx, bauth, rwID1, "", 1, 20, "", "", "")
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, all, 2)

	// type=expense only
	expOnly, total, err := svc.GetTransactions(ctx, bauth, rwID1, "", 1, 20, "", "", "", rw.TransactionFilter{Type: "expense"})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, expOnly, 1)
	require.Equal(t, "expense", expOnly[0].Type)

	// type=income only
	incOnly, total, err := svc.GetTransactions(ctx, bauth, rwID1, "", 1, 20, "", "", "", rw.TransactionFilter{Type: "income"})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, incOnly, 1)
	require.Equal(t, "income", incOnly[0].Type)

	// category filter
	catOnly, total, err := svc.GetTransactions(ctx, bauth, rwID1, "", 1, 20, "", "", "", rw.TransactionFilter{CategoryID: catExpense})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, catOnly, 1)
	require.Equal(t, catExpense, derefStr(catOnly[0].CategoryID))
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestGetKASReportDetail_ValidationAndWindow(t *testing.T) {
	pool, repo, rwID1, _, rtID1, _, _ := setupRWTest(t)
	svc := rw.NewService(pool, repo, &auth.ScopingConfig{RWIDs: []string{rwID1}})
	ctx := context.Background()
	userID := createReportUser(t, ctx, pool)
	bauth := bendaharaAuth(rwID1, userID)

	catID := seedCategory(t, ctx, pool, rtID1)
	seedRWTxAt(t, ctx, pool, rwID1, catID, "123000.00", "income", "posted", "Some income", "2026-10-05T09:00:00+07:00", nil)

	// period combined with dates → error
	_, err := svc.GetKASReportDetail(ctx, bauth, rwID1, "2026-10", "2026-10-01", "2026-10-31")
	require.Error(t, err)

	// only one of start_date/end_date → error
	_, err = svc.GetKASReportDetail(ctx, bauth, rwID1, "", "2026-10-01", "")
	require.Error(t, err)

	// invalid period format → error
	_, err = svc.GetKASReportDetail(ctx, bauth, rwID1, "2026-13", "", "")
	require.Error(t, err)

	// start_date after end_date → error
	_, err = svc.GetKASReportDetail(ctx, bauth, rwID1, "", "2026-10-31", "2026-10-01")
	require.Error(t, err)

	// Same-day window covers the whole day.
	detail, err := svc.GetKASReportDetail(ctx, bauth, rwID1, "", "2026-10-05", "2026-10-05")
	require.NoError(t, err)
	require.Equal(t, "123000.00", detail.TotalIncome)
	require.Equal(t, "2026-10-05", detail.StartDate)
	require.Equal(t, "2026-10-05", detail.EndDate)

	// Period query returns the month.
	detail2, err := svc.GetKASReportDetail(ctx, bauth, rwID1, "2026-10", "", "")
	require.NoError(t, err)
	require.Equal(t, "2026-10", detail2.Period)
	require.Equal(t, "123000.00", detail2.TotalIncome)
}
