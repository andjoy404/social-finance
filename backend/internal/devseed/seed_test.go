package devseed_test

import (
	"context"
	"testing"

	"social-finance/internal/devseed"
	"social-finance/internal/testutil"
)

func TestDevSeed_ExecutionAndIdempotency(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// 1. First run: should populate all canonical seed data
	summary1, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("first seed run failed: %v", err)
	}

	if summary1.RTsCount != 5 {
		t.Errorf("expected 5 RTs, got %d", summary1.RTsCount)
	}
	if summary1.PhysicalHousesCount != 20 {
		t.Errorf("expected 20 physical houses, got %d", summary1.PhysicalHousesCount)
	}
	if summary1.HouseholdsCount != 20 {
		t.Errorf("expected 20 households, got %d", summary1.HouseholdsCount)
	}
	if summary1.HouseholdOccupanciesCount != 20 {
		t.Errorf("expected 20 occupancies, got %d", summary1.HouseholdOccupanciesCount)
	}
	if summary1.ResidentsCount != 20 {
		t.Errorf("expected 20 residents, got %d", summary1.ResidentsCount)
	}
	if summary1.ResidencyPeriodsCount != 26 {
		t.Errorf("expected 26 residency periods, got %d", summary1.ResidencyPeriodsCount)
	}
	if summary1.UsersCount != 21 {
		t.Errorf("expected 21 users, got %d", summary1.UsersCount)
	}
	if summary1.MembershipsCount != 21 {
		t.Errorf("expected 21 memberships, got %d", summary1.MembershipsCount)
	}

	if summary1.PositionUsersCount != 35 {
		t.Errorf("expected 35 position users, got %d", summary1.PositionUsersCount)
	}
	if summary1.PositionMembershipsCount != 35 {
		t.Errorf("expected 35 position memberships, got %d", summary1.PositionMembershipsCount)
	}
	if summary1.PermissionTypesCount != 12 {
		t.Errorf("expected 12 permission types, got %d", summary1.PermissionTypesCount)
	}
	if summary1.PositionPermissionsCount != 36 {
		t.Errorf("expected 36 position permissions, got %d", summary1.PositionPermissionsCount)
	}

	// 2. Second run: idempotency test - should succeed without error or duplicating rows
	summary2, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("second seed run failed: %v", err)
	}

	// Verify row counts in the database did not double
	var count int
	if err := pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts WHERE id IN ('8dfa1e36-6727-45be-8fea-24d958efc155', 'efb70405-09f5-48a0-ad14-34802cf09e3b')").Scan(&count); err != nil {
		t.Fatalf("query rts count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 RTs from canonical set, found %d", count)
	}

	if err := pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM physical_houses WHERE id::text LIKE 'bbbb%'").Scan(&count); err != nil {
		t.Fatalf("query physical_houses count: %v", err)
	}
	if count != 18 {
		t.Errorf("expected 18 bbbb physical houses, found %d", count)
	}

	if summary2.UsersCount != summary1.UsersCount {
		t.Errorf("expected idempotent user count %d, got %d", summary1.UsersCount, summary2.UsersCount)
	}
	if summary2.PositionUsersCount != summary1.PositionUsersCount {
		t.Errorf("expected idempotent position user count %d, got %d", summary1.PositionUsersCount, summary2.PositionUsersCount)
	}
}

func TestDevSeed_SafetyGuard_RejectsProduction(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	_, err := devseed.Run(ctx, pool, "production")
	if err == nil {
		t.Fatalf("expected devseed to fail for APP_ENV=production, but it succeeded")
	}
	if err := pool.Raw().PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestDevSeed_NeverCreatesSuperAdmin(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	_, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	var superAdminCount int
	q := `SELECT COUNT(*) FROM users WHERE system_role = 'super_admin' AND email LIKE '%@example.%'`
	if err := pool.Raw().QueryRowContext(ctx, q).Scan(&superAdminCount); err != nil {
		t.Fatalf("query super_admin count: %v", err)
	}
	if superAdminCount != 0 {
		t.Errorf("devseed must NEVER create super_admin, found %d", superAdminCount)
	}
}
