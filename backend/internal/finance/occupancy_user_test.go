package finance

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"social-finance/internal/devseed"
)

func TestGetOccupancyIDForUser_PrefixAndEmailMatching(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Seed DB
	if _, err := devseed.Run(ctx, pool, "test"); err != nil {
		t.Fatalf("devseed failed: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Test 1: User with phone 081298918220 and resident with +6281298918220
	// RT 03 id from devseed
	var rtID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt: %v", err)
	}

	// Create test resident and user with different phone prefixes
	var testUserID, testResID, testOccupancyID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		VALUES (gen_random_uuid(), 'testwarga_phone@local.com', '089911223344', 'dummyhash', 'Test Warga Phone', true)
		RETURNING id
	`).Scan(&testUserID)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO residents (id, rt_id, full_name, phone, nik, email, is_active)
		VALUES (gen_random_uuid(), $1, 'Test Warga Phone', '+6289911223344', '3201010101990001', 'resident@local.com', true)
		RETURNING id
	`, rtID).Scan(&testResID)
	if err != nil {
		t.Fatalf("create resident: %v", err)
	}

	// Link resident to household occupancy
	var phID, hhID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO physical_houses (rt_id, house_number, is_active)
		VALUES ($1, 'TEST-99', true) RETURNING id
	`, rtID).Scan(&phID)
	if err != nil {
		t.Fatalf("create physical house: %v", err)
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO households (rt_id, head_name, is_active)
		VALUES ($1, 'Test Warga Phone', true) RETURNING id
	`, rtID).Scan(&hhID)
	if err != nil {
		t.Fatalf("create household: %v", err)
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO household_occupancies (physical_house_id, household_id, occupancy_status, start_date)
		VALUES ($1, $2, 'OWNER', CURRENT_DATE) RETURNING id
	`, phID, hhID).Scan(&testOccupancyID)
	if err != nil {
		t.Fatalf("create occupancy: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		VALUES ($1, $2, CURRENT_DATE)
	`, testResID, testOccupancyID)
	if err != nil {
		t.Fatalf("create residency period: %v", err)
	}

	// Test GetOccupancyIDForUser with phone normalization
	resolvedOccID, err := GetOccupancyIDForUser(ctx, tx, rtID, testUserID)
	if err != nil {
		t.Fatalf("GetOccupancyIDForUser failed: %v", err)
	}
	if resolvedOccID != testOccupancyID {
		t.Errorf("expected occupancy ID %s, got %s", testOccupancyID, resolvedOccID)
	}

	// Check that resident.user_id was backfilled
	var backfilledUserID sql.NullString
	_ = tx.QueryRowContext(ctx, `SELECT user_id FROM residents WHERE id = $1`, testResID).Scan(&backfilledUserID)
	if !backfilledUserID.Valid || backfilledUserID.String != testUserID {
		t.Errorf("expected resident.user_id to be backfilled to %s, got %v", testUserID, backfilledUserID)
	}
}

func TestGetDashboardSummary_NoAbortedTx(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Seed DB
	if _, err := devseed.Run(ctx, pool, "test"); err != nil {
		t.Fatalf("devseed failed: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	var rtID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt: %v", err)
	}

	svc := NewService(pool)
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	currentPeriod := time.Now().In(loc).Format("2006-01")

	summary, err := svc.GetDashboardSummary(ctx, tx, rtID, currentPeriod)
	if err != nil {
		t.Fatalf("GetDashboardSummary failed: %v", err)
	}

	if summary == nil {
		t.Fatal("expected summary, got nil")
	}
	if len(summary.Chart) != 12 {
		t.Errorf("expected 12 chart points, got %d", len(summary.Chart))
	}
}
