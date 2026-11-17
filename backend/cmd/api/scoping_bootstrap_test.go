package main

// ============================================================================
// SCOPED_RT_IDS ↔ auto-provision integration scenarios.
//
// Verifies the startup chain (auto-provision → bootstrap → LoadScoping) against
// fresh and existing databases, covering:
//   A. Fresh DB + SCOPED_RT_IDS blank          → auto-derived scope, SA created.
//   B. Fresh DB + explicit valid scope         → scope follows operator config.
//   C. Existing DB + explicit scope on restart → scope preserved, no overwrite.
//   D. Restart (blank scope)                   → no duplicates, scope stable,
//                                                SA not re-created/password kept.
//   E. Invalid scope                           → LoadScoping fails closed with an
//                                                informative error, empty scope.
// ============================================================================

import (
	"context"
	"os"
	"strings"
	"testing"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

// saveScopedEnv snapshots SCOPED_RT_IDS / SCOPED_RW_IDS and restores them on
// cleanup. Tests that operate on the startup chain must leave them untouched
// for other tests.
func saveScopedEnv(t *testing.T) {
	t.Helper()
	prevRT, hadRT := os.LookupEnv("SCOPED_RT_IDS")
	prevRW, hadRW := os.LookupEnv("SCOPED_RW_IDS")
	t.Cleanup(func() {
		if hadRT {
			_ = os.Setenv("SCOPED_RT_IDS", prevRT)
		} else {
			os.Unsetenv("SCOPED_RT_IDS")
		}
		if hadRW {
			_ = os.Setenv("SCOPED_RW_IDS", prevRW)
		} else {
			os.Unsetenv("SCOPED_RW_IDS")
		}
	})
}

// clearScopedEnv sets SCOPED_RT_IDS / SCOPED_RW_IDS to the given values.
func clearScopedEnv(t *testing.T, rt, rw string) {
	t.Helper()
	os.Setenv("SCOPED_RT_IDS", rt)
	os.Setenv("SCOPED_RW_IDS", rw)
}

func countSuperAdminMembership(t *testing.T, pool *database.Pool) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM user_rt_memberships urm
		 JOIN users u ON u.id = urm.user_id
		 WHERE u.system_role = 'super_admin' AND urm.is_active = true`).Scan(&n); err != nil {
		t.Fatalf("count super admin membership: %v", err)
	}
	return n
}

func TestScopeBootstrap_FreshInstall_EmptyScope(t *testing.T) {
	if os.Getenv("SKIP_FRESH_BOOTSTRAP_TEST") != "" {
		t.Skip("fresh-DB scoping tests disabled by SKIP_FRESH_BOOTSTRAP_TEST")
	}

	saveScopedEnv(t)
	clearScopedEnv(t, "", "")

	fresh := createFreshMigratedDB(t)
	setFreshBootstrapEnv(t, fresh, "scope-a@test.local")

	checkPool := openFreshPool(t, fresh)
	ctx := context.Background()

	// Startup chain: auto-provision → bootstrap → LoadScoping.
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("runAutoProvision (empty scope): %v", err)
	}

	var resolvedRT string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rts WHERE rt = '002' AND rw = 16 AND is_active = true LIMIT 1`).Scan(&resolvedRT); err != nil {
		t.Fatalf("resolve RT 002: %v", err)
	}

	// Scope must be auto-derived to the provisioned RT UUID.
	scope := os.Getenv("SCOPED_RT_IDS")
	if scope != resolvedRT {
		t.Fatalf("auto-derived scope = %q, want %q", scope, resolvedRT)
	}

	if err := runBootstrapWithPool(ctx, checkPool); err != nil {
		t.Fatalf("runBootstrapWithPool: %v", err)
	}

	cfg, err := auth.LoadScoping()
	if err != nil {
		t.Fatalf("LoadScoping after auto-derived provision: %v", err)
	}
	if len(cfg.RTIDs) != 1 || cfg.RTIDs[0] != resolvedRT {
		t.Fatalf("scoping config = %+v, want RTIDs=[%s]", cfg.RTIDs, resolvedRT)
	}
	t.Logf("startup chain complete: LoadScoping RTIDs=%v", cfg.RTIDs)

	var email string
	var active bool
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT email, is_active FROM users WHERE system_role = 'super_admin' AND email = 'scope-a@test.local'`).Scan(&email, &active); err != nil {
		t.Fatalf("lookup super admin: %v", err)
	}
	if email != "scope-a@test.local" || !active {
		t.Fatalf("super admin invalid: email=%s active=%v", email, active)
	}

	if m := countSuperAdminMembership(t, checkPool); m != 1 {
		t.Fatalf("super admin memberships = %d, want 1", m)
	}
}

func TestScopeBootstrap_FreshInstall_ExplicitScope(t *testing.T) {
	if os.Getenv("SKIP_FRESH_BOOTSTRAP_TEST") != "" {
		t.Skip("fresh-DB scoping tests disabled by SKIP_FRESH_BOOTSTRAP_TEST")
	}

	saveScopedEnv(t)
	explicit := "11111111-2222-3333-4444-555555555555,aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	clearScopedEnv(t, explicit, "")

	fresh := createFreshMigratedDB(t)
	setFreshBootstrapEnv(t, fresh, "scope-b@test.local")

	checkPool := openFreshPool(t, fresh)
	ctx := context.Background()

	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("runAutoProvision (explicit scope): %v", err)
	}

	var resolvedRT string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rts WHERE rt = '002' AND rw = 16 AND is_active = true LIMIT 1`).Scan(&resolvedRT); err != nil {
		t.Fatalf("resolve RT 002: %v", err)
	}

	// The operator-configured list must remain untouched.
	if got := os.Getenv("SCOPED_RT_IDS"); got != explicit {
		t.Fatalf("explicit scope overwritten: got %q, want %q", got, explicit)
	}
	cfg, err := auth.LoadScoping()
	if err != nil {
		t.Fatalf("LoadScoping: %v", err)
	}
	if len(cfg.RTIDs) != 2 || cfg.RTIDs[0] != "11111111-2222-3333-4444-555555555555" || cfg.RTIDs[1] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("scoping config = %+v, want the two operator-configured UUIDs in order", cfg.RTIDs)
	}
	t.Logf("explicit scope preserved: %v", cfg.RTIDs)

	// Bootstrap must still work and link the super admin to the provisioned RT
	// (not to the scope list).
	if err := runBootstrapWithPool(ctx, checkPool); err != nil {
		t.Fatalf("runBootstrapWithPool: %v", err)
	}
	var linkRT string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT urm.rt_id::text FROM user_rt_memberships urm
		 JOIN users u ON u.id = urm.user_id
		 WHERE u.email = 'scope-b@test.local'`).Scan(&linkRT); err != nil {
		t.Fatalf("lookup membership: %v", err)
	}
	if linkRT != resolvedRT {
		t.Fatalf("membership linked to %s, want provisioned RT %s", linkRT, resolvedRT)
	}
	t.Logf("bootstrap super admin linked to provisioned RT %s (scope = operator config)", linkRT)
}

func TestScopeBootstrap_Restart_ReusedRT_ExplicitScope(t *testing.T) {
	if os.Getenv("SKIP_FRESH_BOOTSTRAP_TEST") != "" {
		t.Skip("fresh-DB scoping tests disabled by SKIP_FRESH_BOOTSTRAP_TEST")
	}

	saveScopedEnv(t)
	clearScopedEnv(t, "", "")

	fresh := createFreshMigratedDB(t)
	setFreshBootstrapEnv(t, fresh, "scope-c@test.local")

	checkPool := openFreshPool(t, fresh)
	ctx := context.Background()

	// First boot with blank scope → provision new RT.
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("first runAutoProvision: %v", err)
	}
	var resolvedRT string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rts WHERE rt = '002' AND rw = 16 AND is_active = true LIMIT 1`).Scan(&resolvedRT); err != nil {
		t.Fatalf("resolve RT: %v", err)
	}

	// Restart with an explicit, different scope — must NOT be overwritten.
	explicit := "bbbbbbbb-cccc-dddd-eeee-ffffffffffff,12345678-90ab-cdef-1234-567890abcdef"
	clearScopedEnv(t, explicit, "")
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("restart runAutoProvision: %v", err)
	}
	if got := os.Getenv("SCOPED_RT_IDS"); got != explicit {
		t.Fatalf("explicit scope overwritten on restart: got %q, want %q", got, explicit)
	}

	var rtCount, rwCount int
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&rtCount); err != nil {
		t.Fatalf("count rts: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rws").Scan(&rwCount); err != nil {
		t.Fatalf("count rws: %v", err)
	}
	if rtCount != 1 || rwCount != 1 {
		t.Fatalf("restart created duplicates: rws=%d rts=%d", rwCount, rtCount)
	}
	cfg, err := auth.LoadScoping()
	if err != nil {
		t.Fatalf("LoadScoping: %v", err)
	}
	if len(cfg.RTIDs) != 2 {
		t.Fatalf("scoping config = %+v, want 2 UUIDs", cfg.RTIDs)
	}
	t.Logf("restart with explicit scope: RT reused (%s…), scope intact", shortUUID(resolvedRT))
}

func TestScopeBootstrap_Restart_BlankScope_NoDuplicates(t *testing.T) {
	if os.Getenv("SKIP_FRESH_BOOTSTRAP_TEST") != "" {
		t.Skip("fresh-DB scoping tests disabled by SKIP_FRESH_BOOTSTRAP_TEST")
	}

	saveScopedEnv(t)
	clearScopedEnv(t, "", "")

	fresh := createFreshMigratedDB(t)
	setFreshBootstrapEnv(t, fresh, "scope-d@test.local")

	checkPool := openFreshPool(t, fresh)
	ctx := context.Background()

	// --- Simulated first process ---
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("first provision: %v", err)
	}
	if err := runBootstrapWithPool(ctx, checkPool); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	var resolvedRT string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rts WHERE rt = '002' AND rw = 16 AND is_active = true LIMIT 1`).Scan(&resolvedRT); err != nil {
		t.Fatalf("resolve RT: %v", err)
	}
	var hash1 string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE email = 'scope-d@test.local'`).Scan(&hash1); err != nil {
		t.Fatalf("hash after first bootstrap: %v", err)
	}

	// --- Simulated restart: docker default env resets to blank ---
	clearScopedEnv(t, "", "")

	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("restart provision: %v", err)
	}
	if err := runBootstrapWithPool(ctx, checkPool); err != nil {
		t.Fatalf("restart bootstrap: %v", err)
	}

	// Scope must be auto-derived to the SAME RT UUID.
	if got := os.Getenv("SCOPED_RT_IDS"); got != resolvedRT {
		t.Fatalf("auto-derived scope on restart = %q, want stable RT %q", got, resolvedRT)
	}

	var rtCount, rwCount, saCount, membershipCount int
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&rtCount); err != nil {
		t.Fatalf("count rts: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rws").Scan(&rwCount); err != nil {
		t.Fatalf("count rws: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE system_role = 'super_admin'`).Scan(&saCount); err != nil {
		t.Fatalf("count super admins: %v", err)
	}
	if membershipCount = countSuperAdminMembership(t, checkPool); membershipCount != 1 {
		t.Fatalf("super admin memberships = %d, want 1", membershipCount)
	}
	if rtCount != 1 || rwCount != 1 || saCount != 1 {
		t.Fatalf("restart created duplicates: rws=%d rts=%d super_admins=%d", rwCount, rtCount, saCount)
	}

	var hash2 string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE email = 'scope-d@test.local'`).Scan(&hash2); err != nil {
		t.Fatalf("hash after restart bootstrap: %v", err)
	}
	if hash1 != hash2 {
		t.Fatal("super admin password hash changed across restart (password was reset)")
	}
	t.Logf("restart stable: RT=%s… scope stable, hash unchanged, counts 1/1/1", shortUUID(resolvedRT))
}

func TestScopeBootstrap_InvalidScope_FailsClosed(t *testing.T) {
	if os.Getenv("SKIP_FRESH_BOOTSTRAP_TEST") != "" {
		t.Skip("fresh-DB scoping tests disabled by SKIP_FRESH_BOOTSTRAP_TEST")
	}

	saveScopedEnv(t)
	clearScopedEnv(t, "", "")

	fresh := createFreshMigratedDB(t)
	setFreshBootstrapEnv(t, fresh, "scope-e@test.local")

	checkPool := openFreshPool(t, fresh)
	ctx := context.Background()

	// First bootstrap the database with a working blank scope so an existing
	// RT is available; then restart with invalid operator config.
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("first provision: %v", err)
	}
	originalRT := os.Getenv("SCOPED_RT_IDS")

	cases := []struct {
		name    string
		scoped  string
		rw      string
		wantErr string
	}{
		{"single invalid uuid", "not-a-uuid", "", "SCOPED_RT_IDS"},
		{"mixed valid and invalid", originalRT + ",not-a-uuid", "", "SCOPED_RT_IDS"},
		{"empty entry between commas", originalRT + ",," + originalRT, "", ""}, // blank entries are skipped by parser
		{"invalid RW scope", originalRT, "oops-not-a-uuid", "SCOPED_RW_IDS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("SCOPED_RT_IDS", tc.scoped)
			os.Setenv("SCOPED_RW_IDS", tc.rw)

			// Startup path: provision (explicit config must not be rewritten),
			// then LoadScoping must refuse to boot safely.
			if err := runAutoProvision(ctx, checkPool); err != nil {
				t.Fatalf("runAutoProvision with invalid scope %q: %v", tc.scoped, err)
			}
			if got := os.Getenv("SCOPED_RT_IDS"); got != tc.scoped {
				t.Fatalf("invalid explicit scope was rewritten: got %q, want %q", got, tc.scoped)
			}

			cfg, err := auth.LoadScoping()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error for blank entries, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected fail-closed error mentioning %s; got nil (scope=%+v)", tc.wantErr, cfg)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error should mention %s, got: %v", tc.wantErr, err)
			}
			if len(cfg.RTIDs) != 0 {
				t.Fatalf("fail-closed violated: cfg.RTIDs=%v (must be empty)", cfg.RTIDs)
			}
			t.Logf("fail-closed as expected: %v", err)
		})
	}

	// Restore blank scope so subsequent tests aren't surprised.
	os.Setenv("SCOPED_RT_IDS", originalRT)
	os.Setenv("SCOPED_RW_IDS", "")
}
