package main

import (
	"context"
	"os"
	"testing"

	"social-finance/internal/auth"
	"social-finance/internal/devseed"
	"social-finance/internal/testutil"
)

// setBootstrapEnv configures bootstrap env vars and configures the DB to
// use the test database so runBootstrap() operates on the same pool as the test.
func setBootstrapEnv(t *testing.T) {
	t.Helper()
	os.Setenv("DB_NAME", "social_finance_test")
	os.Setenv("BOOTSTRAP_EMAIL", "admin@test.local")
	os.Setenv("BOOTSTRAP_PASSWORD", "TestUser123!")
	os.Setenv("BOOTSTRAP_NAME", "Super Admin")
	t.Cleanup(func() {
		os.Unsetenv("DB_NAME")
		os.Unsetenv("BOOTSTRAP_EMAIL")
		os.Unsetenv("BOOTSTRAP_PASSWORD")
		os.Unsetenv("BOOTSTRAP_NAME")
	})
}

func TestBootstrap_CreatesSuperAdmin(t *testing.T) {
	setBootstrapEnv(t)

	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	err := runBootstrap()
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Verify user exists with correct system_role
	var email string
	var sysRole, fullname string
	var isActive bool
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT email, full_name, system_role, is_active FROM users WHERE email = 'admin@test.local'`,
	).Scan(&email, &fullname, &sysRole, &isActive)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	if email != "admin@test.local" {
		t.Errorf("email = %q; want %q", email, "admin@test.local")
	}
	if fullname != "Super Admin" {
		t.Errorf("fullname = %q; want %q", fullname, "Super Admin")
	}
	if sysRole != "super_admin" {
		t.Errorf("system_role = %q; want %q", sysRole, "super_admin")
	}
	if !isActive {
		t.Error("is_active should be true")
	}

	// Verify password can be verified via existing auth.Verify
	var passwordHash string
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE email = 'admin@test.local'`,
	).Scan(&passwordHash)
	if err != nil {
		t.Fatalf("query password hash: %v", err)
	}
	if passwordHash == "" {
		t.Fatal("password hash should not be empty")
	}
	if err := auth.Verify("TestUser123!", passwordHash); err != nil {
		t.Fatalf("password verification failed: %v", err)
	}
	if err := auth.Verify("wrongpassword", passwordHash); err == nil {
		t.Fatal("expected verification to fail for wrong password")
	}
}

func TestBootstrap_Idempotent_NoDuplicate(t *testing.T) {
	setBootstrapEnv(t)

	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// First run creates user
	err := runBootstrap()
	if err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	// Second run should skip without error (idempotent)
	err = runBootstrap()
	if err != nil {
		t.Fatalf("second bootstrap failed (should skip): %v", err)
	}

	// Verify only one user exists
	var count int
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE system_role = 'super_admin' AND email = 'admin@test.local'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 super_admin, found %d", count)
	}
}

func TestBootstrap_SuperAdminHasCorrectRole(t *testing.T) {
	setBootstrapEnv(t)
	os.Setenv("BOOTSTRAP_EMAIL", "admin-role@test.local")

	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	if err := runBootstrap(); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Verify the user has system_role='super_admin'
	var sysRole string
	err := pool.Raw().QueryRowContext(ctx,
		`SELECT system_role FROM users WHERE email = 'admin-role@test.local'`,
	).Scan(&sysRole)
	if err != nil {
		t.Fatalf("query super_admin role: %v", err)
	}
	if sysRole != "super_admin" {
		t.Errorf("system_role = %q; want %q", sysRole, "super_admin")
	}

	// Verify password via auth.Verify
	var passwordHash string
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE email = 'admin-role@test.local'`,
	).Scan(&passwordHash)
	if err != nil {
		t.Fatalf("find user hash: %v", err)
	}
	if err := auth.Verify("TestUser123!", passwordHash); err != nil {
		t.Fatalf("bootstrap password verification failed: %v", err)
	}

	// Verify active membership in RT 002 exists and matches canonical Urena RT
	var memCount int
	var linkedRTID string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MAX(urm.rt_id::text), '')
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		JOIN rts r ON r.id = urm.rt_id
		WHERE u.email = 'admin-role@test.local' AND r.rt = '002' AND urm.is_active = true
	`).Scan(&memCount, &linkedRTID)
	if err != nil {
		t.Fatalf("query super_admin membership: %v", err)
	}
	if memCount != 1 {
		t.Errorf("expected 1 active membership in RT 002, got %d", memCount)
	}
	if linkedRTID != devseed.CanonicalUrenaRTID {
		t.Errorf("expected membership in canonical RT 002 (%s), got %s", devseed.CanonicalUrenaRTID, linkedRTID)
	}
}

func TestBootstrap_MultiTenantSameRTNumberRegression(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// 1. Create a non-canonical fixture tenant with the same RT number "002"
	fixtureRTID := "c1110002-9999-4a00-8000-000000000002"
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
		VALUES ($1, 'Fixture RT 002 Other Tenant', 88, '002', 'Jalan Fixture Lain', 'Ketua Fixture', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, fixtureRTID)
	if err != nil {
		t.Fatalf("create fixture RT 002: %v", err)
	}

	// 2. Ensure canonical RT 002 (Urena) exists
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
		VALUES ($1, 'Urena', 16, '002', 'GNI Urena', 'Atom', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, devseed.CanonicalUrenaRTID)
	if err != nil {
		t.Fatalf("ensure canonical RT 002: %v", err)
	}

	// 3. Configure bootstrap for a distinct regression test admin
	regEmail := "admin-multi-rt-regression@test.local"
	os.Setenv("DB_NAME", "social_finance_test")
	os.Setenv("BOOTSTRAP_EMAIL", regEmail)
	os.Setenv("BOOTSTRAP_PASSWORD", "TestUser123!")
	os.Setenv("BOOTSTRAP_NAME", "Super Admin Regression")
	defer func() {
		os.Unsetenv("BOOTSTRAP_EMAIL")
		os.Unsetenv("BOOTSTRAP_PASSWORD")
		os.Unsetenv("BOOTSTRAP_NAME")
	}()

	// 4. Run bootstrap first time
	if err := runBootstrap(); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// 5. Run bootstrap second time (verify idempotency)
	if err := runBootstrap(); err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	// 6. Verify:
	// a) Total active memberships across all RTs for this admin is EXACTLY 1
	var totalMemberships int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE u.email = $1 AND urm.is_active = true
	`, regEmail).Scan(&totalMemberships)
	if err != nil {
		t.Fatalf("query total memberships: %v", err)
	}
	if totalMemberships != 1 {
		t.Errorf("expected exactly 1 active membership, got %d", totalMemberships)
	}

	// b) The single active membership points to Canonical Urena RT, NOT the fixture RT
	var actualRTID string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT urm.rt_id::text
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE u.email = $1 AND urm.is_active = true
	`, regEmail).Scan(&actualRTID)
	if err != nil {
		t.Fatalf("query actual rt_id: %v", err)
	}
	if actualRTID != devseed.CanonicalUrenaRTID {
		t.Errorf("expected canonical RT %s, got %s", devseed.CanonicalUrenaRTID, actualRTID)
	}

	// c) The fixture RT 002 has ZERO memberships for this admin
	var fixtureCount int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE u.email = $1 AND urm.rt_id = $2
	`, regEmail, fixtureRTID).Scan(&fixtureCount)
	if err != nil {
		t.Fatalf("query fixture rt count: %v", err)
	}
	if fixtureCount != 0 {
		t.Errorf("expected 0 memberships in fixture RT, got %d", fixtureCount)
	}
}

func TestBootstrap_DevseedNeverCreatesSuperAdmin(t *testing.T) {
	// Ensure no bootstrap env vars are set so bootstrap won't interfere
	os.Unsetenv("BOOTSTRAP_EMAIL")
	os.Unsetenv("BOOTSTRAP_PASSWORD")
	os.Unsetenv("BOOTSTRAP_NAME")

	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	var usersBefore, adminsBefore int
	if err := pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&usersBefore); err != nil {
		t.Fatalf("query baseline users: %v", err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE system_role = 'super_admin'`).Scan(&adminsBefore); err != nil {
		t.Fatalf("query baseline super_admin count: %v", err)
	}

	// Run devseed
	summary, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("devseed failed: %v", err)
	}
	if summary.UsersCount != 8 {
		t.Errorf("expected canonical user upsert count 8, got %d", summary.UsersCount)
	}

	var usersAfter, adminsAfter int
	if err := pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&usersAfter); err != nil {
		t.Fatalf("query post-seed users: %v", err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE system_role = 'super_admin'`).Scan(&adminsAfter); err != nil {
		t.Fatalf("query post-seed super_admin count: %v", err)
	}
	if adminsAfter != adminsBefore {
		t.Errorf("devseed changed super_admin count: before=%d after=%d", adminsBefore, adminsAfter)
	}
	if usersAfter < usersBefore {
		t.Errorf("devseed reduced user count: before=%d after=%d", usersBefore, usersAfter)
	}

	// Verify bootstrap doesn't create when BOOTSTRAP_EMAIL is not set
	// (runBootstrap will return error)
	os.Setenv("BOOTSTRAP_EMAIL", "")
	os.Setenv("BOOTSTRAP_PASSWORD", "TestUser123!")
	err = runBootstrap()
	if err == nil {
		t.Fatal("expected bootstrap to fail when BOOTSTRAP_EMAIL is empty")
	}
}
