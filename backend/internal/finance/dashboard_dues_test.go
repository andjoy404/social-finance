package finance

import (
	"context"
	"testing"

	"social-finance/internal/database"
)

// Helper to create a test due
func createTestDue(t *testing.T, db *database.Pool, rtID, name, amount string, isVoluntary bool) string {
	t.Helper()
	var dueID string
	err := db.Raw().QueryRowContext(context.Background(),
		`INSERT INTO dues (rt_id, name, amount, period_type, is_voluntary)
		 VALUES ($1, $2, $3, 'monthly', $4) RETURNING id`,
		rtID, name, amount, isVoluntary,
	).Scan(&dueID)
	if err != nil {
		t.Fatalf("create test due: %v", err)
	}
	return dueID
}

// Helper to create a test bill
func createTestBill(t *testing.T, db *database.Pool, rtID, occID, dueID, period, amount, status string) string {
	t.Helper()
	var billID string
	err := db.Raw().QueryRowContext(context.Background(),
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date)
		 VALUES ($1, $2, $3, $4, $5, $6, CURRENT_DATE) RETURNING id`,
		rtID, occID, dueID, period, amount, status,
	).Scan(&billID)
	if err != nil {
		t.Fatalf("create test bill: %v", err)
	}
	return billID
}

func TestDashboardDuesProgress(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	ctx := context.Background()
	svc := NewService(pool)

	testPeriod := "2026-11"

	// Setup primary RT
	rt1ID := setupTestRT(t, pool, "RT Dues Progress 1", 1, "DP1")
	defer cleanupTestRT(t, pool, rt1ID)

	// Setup secondary RT for cross-RT testing
	rt2ID := setupTestRT(t, pool, "RT Dues Progress 2", 2, "DP2")
	defer cleanupTestRT(t, pool, rt2ID)

	// Scenario 4: Target nol atau data kosong (0 target -> 0%)
	t.Run("Scenario 4: Target nol atau data kosong", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		summary, err := svc.GetDashboardSummary(ctx, tx, rt1ID, testPeriod)
		if err != nil {
			t.Fatalf("GetDashboardSummary: %v", err)
		}
		if summary.DuesProgress.TotalHouseholds != 0 {
			t.Errorf("expected 0 total households, got %d", summary.DuesProgress.TotalHouseholds)
		}
		if summary.DuesProgress.PaidHouseholds != 0 {
			t.Errorf("expected 0 paid households, got %d", summary.DuesProgress.PaidHouseholds)
		}
		if summary.DuesProgress.Percentage != 0 {
			t.Errorf("expected 0 percentage, got %d", summary.DuesProgress.Percentage)
		}
	})

	// Create 2 households in RT1
	occ1 := setupTestOccupancy(t, pool, rt1ID)
	occ2 := setupTestOccupancy(t, pool, rt1ID)

	// Create regular and voluntary dues in RT1
	regDue1 := createTestDue(t, pool, rt1ID, "Iuran Keamanan", "50000.00", false)
	regDue2 := createTestDue(t, pool, rt1ID, "Iuran Kebersihan", "30000.00", false)
	volDue := createTestDue(t, pool, rt1ID, "Sumbangan HUT", "0.00", true)

	// Scenario 1: Belum ada pembayaran yang disetujui (0 paid out of 2)
	t.Run("Scenario 1: Belum ada pembayaran yang disetujui", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		period := "2026-01"
		createTestBill(t, pool, rt1ID, occ1, regDue1, period, "50000.00", "unpaid")
		createTestBill(t, pool, rt1ID, occ2, regDue1, period, "50000.00", "unpaid")

		summary, err := svc.GetDashboardSummary(ctx, tx, rt1ID, period)
		if err != nil {
			t.Fatalf("GetDashboardSummary: %v", err)
		}
		if summary.DuesProgress.TotalHouseholds != 2 {
			t.Errorf("expected 2 total households, got %d", summary.DuesProgress.TotalHouseholds)
		}
		if summary.DuesProgress.PaidHouseholds != 0 {
			t.Errorf("expected 0 paid households, got %d", summary.DuesProgress.PaidHouseholds)
		}
		if summary.DuesProgress.Percentage != 0 {
			t.Errorf("expected 0 percentage, got %d", summary.DuesProgress.Percentage)
		}
	})

	// Scenario 2: Ada pembayaran yang disetujui sebagian
	t.Run("Scenario 2: Ada pembayaran yang disetujui sebagian", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		period := "2026-02"
		// HH1: paid fully (1 of 1 bill paid)
		createTestBill(t, pool, rt1ID, occ1, regDue1, period, "50000.00", "paid")
		// HH2: has 2 regular bills, only 1 is paid, other is unpaid
		createTestBill(t, pool, rt1ID, occ2, regDue1, period, "50000.00", "paid")
		createTestBill(t, pool, rt1ID, occ2, regDue2, period, "30000.00", "unpaid")

		summary, err := svc.GetDashboardSummary(ctx, tx, rt1ID, period)
		if err != nil {
			t.Fatalf("GetDashboardSummary: %v", err)
		}
		if summary.DuesProgress.TotalHouseholds != 2 {
			t.Errorf("expected 2 total households, got %d", summary.DuesProgress.TotalHouseholds)
		}
		// Only HH1 is fully paid. HH2 is partially paid, so not counted as paid household.
		if summary.DuesProgress.PaidHouseholds != 1 {
			t.Errorf("expected 1 paid household, got %d", summary.DuesProgress.PaidHouseholds)
		}
		if summary.DuesProgress.Percentage != 50 {
			t.Errorf("expected 50 percentage, got %d", summary.DuesProgress.Percentage)
		}
	})

	// Scenario 3: Semua target sudah terkumpul (100%)
	t.Run("Scenario 3: Semua target sudah terkumpul", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		period := "2026-03"
		createTestBill(t, pool, rt1ID, occ1, regDue1, period, "50000.00", "paid")
		createTestBill(t, pool, rt1ID, occ2, regDue1, period, "50000.00", "paid")

		summary, err := svc.GetDashboardSummary(ctx, tx, rt1ID, period)
		if err != nil {
			t.Fatalf("GetDashboardSummary: %v", err)
		}
		if summary.DuesProgress.TotalHouseholds != 2 {
			t.Errorf("expected 2 total households, got %d", summary.DuesProgress.TotalHouseholds)
		}
		if summary.DuesProgress.PaidHouseholds != 2 {
			t.Errorf("expected 2 paid households, got %d", summary.DuesProgress.PaidHouseholds)
		}
		if summary.DuesProgress.Percentage != 100 {
			t.Errorf("expected 100 percentage, got %d", summary.DuesProgress.Percentage)
		}
	})

	// Scenario 5: Pembayaran pending/ditolak tidak masuk ke total terkumpul
	t.Run("Scenario 5: Pembayaran pending atau ditolak tidak masuk ke paid", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		period := "2026-04"
		// Bill with status 'partial' or 'unpaid' (pending/rejected payments leave bill unpaid/partial)
		createTestBill(t, pool, rt1ID, occ1, regDue1, period, "50000.00", "partial")
		createTestBill(t, pool, rt1ID, occ2, regDue1, period, "50000.00", "unpaid")

		summary, err := svc.GetDashboardSummary(ctx, tx, rt1ID, period)
		if err != nil {
			t.Fatalf("GetDashboardSummary: %v", err)
		}
		if summary.DuesProgress.TotalHouseholds != 2 {
			t.Errorf("expected 2 total households, got %d", summary.DuesProgress.TotalHouseholds)
		}
		if summary.DuesProgress.PaidHouseholds != 0 {
			t.Errorf("expected 0 paid households for partial/unpaid bills, got %d", summary.DuesProgress.PaidHouseholds)
		}
		if summary.DuesProgress.Percentage != 0 {
			t.Errorf("expected 0 percentage, got %d", summary.DuesProgress.Percentage)
		}
	})

	// Scenario 6: Scope RT/RW tidak tercampur
	t.Run("Scenario 6: Cross-RT isolation", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		period := "2026-05"
		// In RT2: create an occupancy and a paid bill
		occRT2 := setupTestOccupancy(t, pool, rt2ID)
		dueRT2 := createTestDue(t, pool, rt2ID, "Iuran RT2", "50000.00", false)
		createTestBill(t, pool, rt2ID, occRT2, dueRT2, period, "50000.00", "paid")

		// Query RT1 for same period (which has no bills in period 2026-05)
		summaryRT1, err := svc.GetDashboardSummary(ctx, tx, rt1ID, period)
		if err != nil {
			t.Fatalf("GetDashboardSummary RT1: %v", err)
		}
		if summaryRT1.DuesProgress.TotalHouseholds != 0 {
			t.Errorf("expected 0 total households in RT1, got %d", summaryRT1.DuesProgress.TotalHouseholds)
		}
		if summaryRT1.DuesProgress.PaidHouseholds != 0 {
			t.Errorf("expected 0 paid households in RT1, got %d", summaryRT1.DuesProgress.PaidHouseholds)
		}

		// Query RT2: should see its own 1 paid household
		summaryRT2, err := svc.GetDashboardSummary(ctx, tx, rt2ID, period)
		if err != nil {
			t.Fatalf("GetDashboardSummary RT2: %v", err)
		}
		if summaryRT2.DuesProgress.TotalHouseholds != 1 || summaryRT2.DuesProgress.PaidHouseholds != 1 {
			t.Errorf("expected 1/1 in RT2, got %d/%d", summaryRT2.DuesProgress.PaidHouseholds, summaryRT2.DuesProgress.TotalHouseholds)
		}
	})

	// Scenario 7: Tagihan reguler dan sukarela ditangani sesuai aturan bisnis (sukarela excluded)
	t.Run("Scenario 7: Voluntary dues excluded from target and progress", func(t *testing.T) {
		tx, err := pool.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		period := "2026-06"
		// HH1: paid regular bill, but unpaid voluntary bill -> should still count as paid!
		createTestBill(t, pool, rt1ID, occ1, regDue1, period, "50000.00", "paid")
		createTestBill(t, pool, rt1ID, occ1, volDue, period, "0.00", "unpaid")

		// HH2: ONLY has a voluntary bill (paid) -> should NOT be included in regular target
		createTestBill(t, pool, rt1ID, occ2, volDue, period, "0.00", "paid")

		summary, err := svc.GetDashboardSummary(ctx, tx, rt1ID, period)
		if err != nil {
			t.Fatalf("GetDashboardSummary: %v", err)
		}
		// Only HH1 has regular bills, so total target is 1.
		if summary.DuesProgress.TotalHouseholds != 1 {
			t.Errorf("expected 1 total household (excluding voluntary-only HH), got %d", summary.DuesProgress.TotalHouseholds)
		}
		// HH1 paid its regular bill, so paid is 1.
		if summary.DuesProgress.PaidHouseholds != 1 {
			t.Errorf("expected 1 paid household, got %d", summary.DuesProgress.PaidHouseholds)
		}
		if summary.DuesProgress.Percentage != 100 {
			t.Errorf("expected 100 percentage, got %d", summary.DuesProgress.Percentage)
		}
	})
}
