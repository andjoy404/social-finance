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

	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	if err := runBootstrap(); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Verify the user has system_role='super_admin'
	var sysRole string
	err := pool.Raw().QueryRowContext(ctx,
		`SELECT system_role FROM users WHERE email = 'admin@test.local'`,
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
		`SELECT password_hash FROM users WHERE email = 'admin@test.local'`,
	).Scan(&passwordHash)
	if err != nil {
		t.Fatalf("find user hash: %v", err)
	}
	if err := auth.Verify("TestUser123!", passwordHash); err != nil {
		t.Fatalf("bootstrap password verification failed: %v", err)
	}
}

func TestBootstrap_DevseedNeverCreatesSuperAdmin(t *testing.T) {
	// Ensure no bootstrap env vars are set so bootstrap won't interfere
	os.Unsetenv("BOOTSTRAP_EMAIL")
	os.Unsetenv("BOOTSTRAP_PASSWORD")
	os.Unsetenv("BOOTSTRAP_NAME")

	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Run devseed
	summary, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("devseed failed: %v", err)
	}
	if summary.UsersCount != 29 {
		t.Errorf("expected 29 users, got %d", summary.UsersCount)
	}

	// Verify zero super_admin users exist
	var superAdminCount int
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE system_role = 'super_admin' AND email LIKE '%@example.%'`,
	).Scan(&superAdminCount)
	if err != nil {
		t.Fatalf("query super_admin count: %v", err)
	}
	if superAdminCount != 0 {
		t.Errorf("devseed must NEVER create super_admin, found %d", superAdminCount)
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
