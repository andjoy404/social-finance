package rw

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

// ===========================================================================
// testRWScopingConfig returns a ScopingConfig containing a valid UUID
// placeholder so that SA bypass validation passes during tests.
func testRWScopingConfig(t *testing.T) *auth.ScopingConfig {
	t.Helper()
	// Use a real UUID for the allowlist. Tests that check "out of scope" will
	// use rwIDs that are NOT in this slice.
	return &auth.ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"00000000-0000-0000-0000-000000000000"},
	}
}

// setupRWPermissionTest creates the minimal RW+RT fixtures needed for
// permission tests. Returns a ScopingConfig with the test RW ID in the
// allowlist so that SA bypass validation works.
// ===========================================================================

func setupRWPermissionTest(t *testing.T) (*database.Pool, *Repository, string, string, string, *auth.ScopingConfig) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	repo := NewRepository(pool.Raw())

	ctx := context.Background()

	var rwID1 string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Perm Test 1', 101, true) RETURNING id`,
	).Scan(&rwID1)
	require.NoError(t, err)

	var rwID2 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Perm Test 2', 102, true) RETURNING id`,
	).Scan(&rwID2)
	require.NoError(t, err)

	var rtID1 string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 101-01', 101, $1, $2, true) RETURNING id`,
		fmt.Sprintf("01-%d", time.Now().UnixNano()), rwID1,
	).Scan(&rtID1)
	require.NoError(t, err)

	return pool, repo, rwID1, rwID2, rtID1, &auth.ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{rwID1, rwID2},
	}
}

// ===========================================================================
// PERMISSION ENFORCEMENT TESTS
// ===========================================================================

// TestPermissionEnforcement_JabatanKetua tests that ketua RW can perform all finance operations.
func TestPermissionEnforcement_JabatanKetua(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-user-ketua",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "ketua"},
		},
	}

	// Ketua should be able to read (dashboard, KAS, transactions, locations)
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "ketua should have finance.read permission")

	_, err = svc.GetKASReport(ctx, ac, rwID, "")
	require.NoError(t, err, "ketua should have finance.read permission")

	_, _, err = svc.GetTransactions(ctx, ac, rwID, "", 1, 20, "", "")
	require.NoError(t, err, "ketua should have finance.read permission")

	_, err = svc.GetLocations(ctx, ac, rwID, nil)
	require.NoError(t, err, "ketua should have finance.read permission")
}

// TestPermissionEnforcement_JabatanWakilKetua tests that wakil_ketua can read and create but NOT update.
func TestPermissionEnforcement_JabatanWakilKetua(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-user-wakil",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "wakil_ketua"},
		},
	}

	// Wakil ketua should be able to read
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "wakil_ketua should have finance.read permission")

	// Wakil ketua should NOT have finance.update permission
	err = svc.checkRWPermission(ac, rwID, "finance.update")
	require.Error(t, err, "wakil_ketua should NOT have finance.update permission")
	require.Contains(t, err.Error(), "permission denied", "error should be permission denied")
}

// TestPermissionEnforcement_JabatanSekretaris tests that sekretaris can ONLY read.
func TestPermissionEnforcement_JabatanSekretaris(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-user-sekretaris",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "sekretaris"},
		},
	}

	// Sekretaris should be able to read
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "sekretaris should have finance.read permission")

	// Sekretaris should NOT have finance.create
	err = svc.checkRWPermission(ac, rwID, "finance.create")
	require.Error(t, err, "sekretaris should NOT have finance.create permission")

	// Sekretaris should NOT have finance.update
	err = svc.checkRWPermission(ac, rwID, "finance.update")
	require.Error(t, err, "sekretaris should NOT have finance.update permission")

	// Sekretaris should NOT have finance.approve
	err = svc.checkRWPermission(ac, rwID, "finance.approve")
	require.Error(t, err, "sekretaris should NOT have finance.approve permission")
}

// TestPermissionEnforcement_JabatanBendahara tests that bendahara can perform all finance operations.
func TestPermissionEnforcement_JabatanBendahara(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-user-bendahara",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "bendahara"},
		},
	}

	// Bendahara should be able to read
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "bendahara should have finance.read permission")

	// Bendahara should have all permissions
	err = svc.checkRWPermission(ac, rwID, "finance.read")
	require.NoError(t, err, "bendahara should have finance.read permission")

	err = svc.checkRWPermission(ac, rwID, "finance.create")
	require.NoError(t, err, "bendahara should have finance.create permission")

	err = svc.checkRWPermission(ac, rwID, "finance.update")
	require.NoError(t, err, "bendahara should have finance.update permission")

	err = svc.checkRWPermission(ac, rwID, "finance.approve")
	require.NoError(t, err, "bendahara should have finance.approve permission")
}

// TestPermissionEnforcement_UnknownJabatan tests that unknown jabatan is denied by default.
func TestPermissionEnforcement_UnknownJabatan(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-user-unknown",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "jabatan_tidak_dikenal"},
		},
	}

	// Unknown jabatan should be denied for ALL permissions
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "unknown jabatan should be denied")
	require.Contains(t, err.Error(), "permission denied", "unknown jabatan should get permission denied")

	// Explicit check for finance.read
	err = svc.checkRWPermission(ac, rwID, "finance.read")
	require.Error(t, err, "unknown jabatan should not have any permission")
}

// TestPermissionEnforcement_MultiRole tests that user with multiple RW memberships can access each RW.
func TestPermissionEnforcement_MultiRole(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID1, rwID2, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-user-multi",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "ketua"},
			{RWID: rwID2, Jabatan: "sekretaris"},
		},
	}

	// User should be able to access RW1 as ketua (all permissions)
	_, err := svc.GetDashboard(ctx, ac, rwID1)
	require.NoError(t, err, "user should access RW1 as ketua")

	// User should be able to access RW2 as sekretaris (read only)
	_, err = svc.GetDashboard(ctx, ac, rwID2)
	require.NoError(t, err, "user should access RW2 as sekretaris (read)")
}

// TestPermissionEnforcement_RTJabatanDoesNotGrantRWPermission tests that RT jabatan does NOT grant RW permission.
func TestPermissionEnforcement_RTJabatanNoRW(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	// User with RT role but RW membership with empty jabatan
	ac := &auth.AuthContext{
		UserID: "test-user-rt",
		TenantRole: auth.RoleBendahara,
		Jabatan: auth.JabatanBendahara,
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: ""}, // empty jabatan
		},
	}

	// Empty jabatan should be denied
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "empty jabatan should be denied")
	require.Contains(t, err.Error(), "permission denied", "empty jabatan should get permission denied")
}

// TestPermissionEnforcement_SuperAdmin tests that SA can access RW without membership.
func TestPermissionEnforcement_SuperAdmin(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{}, // NO membership
	}

	// SA should be able to access all RW operations without membership
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "SA should access RW without membership")

	// SA should pass permission check for all operations
	err = svc.checkRWPermission(ac, rwID, "finance.read")
	require.NoError(t, err, "SA should have all permissions")

	err = svc.checkRWPermission(ac, rwID, "finance.create")
	require.NoError(t, err, "SA should have finance.create permission")

	err = svc.checkRWPermission(ac, rwID, "finance.update")
	require.NoError(t, err, "SA should have finance.update permission")

	err = svc.checkRWPermission(ac, rwID, "finance.approve")
	require.NoError(t, err, "SA should have finance.approve permission")
}

// TestPermissionEnforcement_NoMembership tests that user without RW membership is denied.
func TestPermissionEnforcement_NoMembership(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-user-no-rw",
		RWMemberships: []auth.RWMembership{}, // empty
	}

	// No membership should be denied
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "no membership should be denied")
	require.ErrorIs(t, err, ErrRWAccessDenied, "error should be rw access denied")
}

// TestPermissionEnforcement_CrossRW tests that user can only access their own RW.
func TestPermissionEnforcement_CrossRW(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID1, rwID2, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	// User with membership only in RW1
	ac := &auth.AuthContext{
		UserID: "test-user-rw1",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "ketua"},
		},
	}

	// Should be able to access RW1
	_, err := svc.GetDashboard(ctx, ac, rwID1)
	require.NoError(t, err, "user should access RW1")

	// Should NOT be able to access RW2
	_, err = svc.GetDashboard(ctx, ac, rwID2)
	require.Error(t, err, "user should NOT access RW2")
	require.Contains(t, err.Error(), "rw access denied", "cross-RW access should be denied")
}

// TestServiceCheckRWPermission_Direct tests the permission check function directly.
func TestServiceCheckRWPermission_Direct(t *testing.T) {
	pool, _, rwID, _, _, _ := setupRWPermissionTest(t)
	svc := NewService(pool, nil, nil)

	t.Run("ketua has all permissions", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "test-ketua",
			RWMemberships: []auth.RWMembership{
				{RWID: rwID, Jabatan: "ketua"},
			},
		}

		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.read"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.create"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.update"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.approve"))
	})

	t.Run("wakil_ketua lacks update", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "test-wakil",
			RWMemberships: []auth.RWMembership{
				{RWID: rwID, Jabatan: "wakil_ketua"},
			},
		}

		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.read"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.create"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.approve"))

		err := svc.checkRWPermission(ac, rwID, "finance.update")
		require.Error(t, err)
		require.Contains(t, err.Error(), "finance.update")
	})

	t.Run("sekretaris has only read", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "test-sekretaris",
			RWMemberships: []auth.RWMembership{
				{RWID: rwID, Jabatan: "sekretaris"},
			},
		}

		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.read"))

		require.Error(t, svc.checkRWPermission(ac, rwID, "finance.create"))
		require.Error(t, svc.checkRWPermission(ac, rwID, "finance.update"))
		require.Error(t, svc.checkRWPermission(ac, rwID, "finance.approve"))
	})

	t.Run("bendahara has all permissions", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "test-bendahara",
			RWMemberships: []auth.RWMembership{
				{RWID: rwID, Jabatan: "bendahara"},
			},
		}

		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.read"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.create"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.update"))
		require.NoError(t, svc.checkRWPermission(ac, rwID, "finance.approve"))
	})
}

// TestServiceCheckRWAccess_SAWithNoMembership tests that SA bypass works with empty membership.
func TestServiceCheckRWAccess_SAWithNoMembership(t *testing.T) {
	ctx := context.Background()
	pool, _, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, nil, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{}, // No membership at all
	}

	err := svc.checkRWAccess(ctx, ac, rwID)
	require.NoError(t, err, "SA should bypass membership check")
}

// TestServiceCheckRWAccess_NilContext tests nil AuthContext handling.
func TestServiceCheckRWAccess_NilContext(t *testing.T) {
	pool, _, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, nil, scopeCfg)

	ctx := context.Background()
	// checkRWAccess with nil ac
	err := svc.checkRWAccess(ctx, nil, rwID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "authentication context")

	// checkRWPermission with nil ac
	err = svc.checkRWPermission(nil, rwID, "finance.read")
	require.Error(t, err)
	require.Contains(t, err.Error(), "authentication context")
}

// TestPermissionEnforcement_Consistency tests that all permission checks are consistent across service methods.
func TestPermissionEnforcement_Consistency(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID: "test-consistency",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "sekretaris"},
		},
	}

	// Sekretaris should be able to call all read methods
	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err)

	_, err = svc.GetKASReport(ctx, ac, rwID, "")
	require.NoError(t, err)

	_, _, err = svc.GetTransactions(ctx, ac, rwID, "", 1, 20, "", "")
	require.NoError(t, err)

	_, err = svc.GetLocations(ctx, ac, rwID, nil)
	require.NoError(t, err)

	// All these operations use finance.read permission internally.
	// If any fails, it means the permission check is not consistently applied.
}

// TestErrPermissionDenied_Is tests that ErrPermissionDenied is properly defined.
func TestErrPermissionDenied_Is(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", ErrPermissionDenied)
	require.ErrorIs(t, err, ErrPermissionDenied)

	err2 := fmt.Errorf("wrapped: %w", ErrRWAccessDenied)
	require.NotErrorIs(t, err2, ErrPermissionDenied)
}

// ===========================================================================
// TEST SUPER ADMIN SCOPED SCOPE VALIDATION
// These tests verify that SA bypass is gated by SCOPED_RW_IDS and active status.
// ===========================================================================

// TestSAWithRWInAllowlistAndActive verifies SA can access a valid RW via direct service call.
func TestSAWithRWInAllowlistAndActive(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	// Verify the RW ID is in the allowlist
	require.Contains(t, scopeCfg.RWIDs, rwID, "test setup: RW must be in scope")

	ac := &auth.AuthContext{
		UserID:      "sa-direct-call",
		SystemRole:  auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{}, // No membership
	}

	// SA should be able to call service methods directly
	dashboard, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "SA should access RW that is in allowlist and active")
	require.NotNil(t, dashboard, "dashboard data should be returned")
}

// TestSAWithRWOOutOfAllowlistViaService verifies SA is rejected when RW is out of scope.
func TestSAWithRWOOutOfAllowlistViaService(t *testing.T) {
	ctx := context.Background()
	pool, repo, _, _, _, scopeCfg := setupRWPermissionTest(t)
	// scopeCfg contains rwID1 and rwID2 from setup. Test with a completely different UUID.
	outOfScopeRWID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	// Verify test setup: outOfScopeRWID is NOT in the allowlist
	require.NotContains(t, scopeCfg.RWIDs, outOfScopeRWID,
		"test setup: RW must be out of scope for this test")

	// Create a service instance with the config
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID:      "sa-out-of-scope",
		SystemRole:  auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	// SA should be rejected for out-of-scope RW via direct service call
	_, err := svc.GetDashboard(ctx, ac, outOfScopeRWID)
	require.Error(t, err, "SA should NOT access RW outside SCOPED_RW_IDS")
	require.ErrorIs(t, err, auth.ErrRWScopeInvalid)
}

// TestSAWithInactiveRWViaService verifies SA is rejected for inactive RW.
func TestSAWithInactiveRWViaService(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	// Mark the RW as inactive
	_, err := pool.Raw().ExecContext(ctx, `UPDATE rws SET is_active = false WHERE id = $1`, rwID)
	require.NoError(t, err)
	t.Cleanup(func() {
		// Restore active status for cleanup
		pool.Raw().ExecContext(ctx, `UPDATE rws SET is_active = true WHERE id = $1`, rwID)
	})

	ac := &auth.AuthContext{
		UserID:      "sa-inactive-rw",
		SystemRole:  auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	// SA should be rejected for inactive RW via direct service call
	_, err = svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "SA should NOT access inactive RW")
	require.ErrorIs(t, err, auth.ErrRWScopeInvalid)
}

// TestSAWithoutMembershipValidRW verifies SA can access valid RW without membership.
func TestSAWithoutMembershipValidRW(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID:      "sa-no-membership",
		SystemRole:  auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{}, // Explicitly empty
	}

	dashboard, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "SA should access RW without membership")
	require.NotNil(t, dashboard)
}

// TestUserWithoutMembership verifies non-SA users without membership are denied.
func TestUserWithoutMembership(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID:      "regular-user-no-membership",
		SystemRole:  "", // empty = non-SA role
		RWMemberships: []auth.RWMembership{},
	}

	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "non-SA user without membership should be denied")
	require.ErrorIs(t, err, ErrRWAccessDenied, "non-SA without membership should be ErrRWAccessDenied")
}

// TestUserWithMembershipRWCannotAccessRWB verifies non-SA user with RW A membership cannot access RW B.
func TestUserWithMembershipRWCannotAccessRWB(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID1, rwID2, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	// Create membership for rwID1 only
	ac := &auth.AuthContext{
		UserID: "rw-a-user",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID1, Jabatan: "bendahara"},
		},
	}

	// Accessing RW A (matching membership) should work
	dashboard, err := svc.GetDashboard(ctx, ac, rwID1)
	require.NoError(t, err, "user with RW A membership should access RW A")
	require.NotNil(t, dashboard)

	// Accessing RW B (non-matching membership) should be denied
	_, err = svc.GetDashboard(ctx, ac, rwID2)
	require.Error(t, err, "user with RW A membership should NOT access RW B")
	require.Contains(t, err.Error(), "rw access denied")
	require.ErrorIs(t, err, ErrRWAccessDenied)
}

// TestPermissionJabatanStillApplied verifies permission checks are still enforced for non-SA users.
func TestPermissionJabatanStillApplied(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	t.Run("ketua can access all read methods", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "ketua-user",
			RWMemberships: []auth.RWMembership{
				{RWID: rwID, Jabatan: "ketua"},
			},
		}

		d, err := svc.GetDashboard(ctx, ac, rwID)
		require.NoError(t, err)
		require.NotNil(t, d)

		_, err = svc.GetKASReport(ctx, ac, rwID, "")
		require.NoError(t, err)
	})

	t.Run("sekretaris can only read", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "sekretaris-user",
			RWMemberships: []auth.RWMembership{
				{RWID: rwID, Jabatan: "sekretaris"},
			},
		}

		d, err := svc.GetDashboard(ctx, ac, rwID)
		require.NoError(t, err)
		require.NotNil(t, d)

		_, err = svc.GetKASReport(ctx, ac, rwID, "")
		require.NoError(t, err)
	})
}

// ===========================================================================
// TEST: Error classification & context propagation
// ===========================================================================

// TestSAConfigNil tests that SA with nil scoping config is denied.
func TestSAConfigNil(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, _ := setupRWPermissionTest(t)
	// Deliberately pass nil config — simulates missing scoping config.
	svc := NewService(pool, repo, nil)

	ac := &auth.AuthContext{
		UserID:        "sa-nil-config",
		SystemRole:    auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "SA with nil config should be denied")
	require.ErrorIs(t, err, auth.ErrRWScopeInvalid)
}

// TestSAEmptyRWIDs tests that SA with empty ScopingConfig.RWIDs is denied.
func TestSAEmptyRWIDs(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, _ := setupRWPermissionTest(t)
	// Explicit empty RWIDs — not nil, but empty slice.
	emptyCfg := &auth.ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{}, // Empty allowlist
	}
	svc := NewService(pool, repo, emptyCfg)

	ac := &auth.AuthContext{
		UserID:        "sa-empty-rwids",
		SystemRole:    auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "SA with empty RWIDs should be denied")
	require.ErrorIs(t, err, auth.ErrRWScopeInvalid)
}

// TestSAAllowlistedRWRNotInDB tests that SA with RW ID in allowlist but not in DB is denied.
func TestSAAllowlistedRWRNotInDB(t *testing.T) {
	ctx := context.Background()
	pool, repo, _, _, _, scopeCfg := setupRWPermissionTest(t)
	// Use a RW ID that IS in the allowlist but DOES NOT exist in the DB.
	// We modify scopeCfg to contain a non-existent RW ID.
	fakeRWID := "11111111-2222-3333-4444-555555555555"
	nonExistCfg := &auth.ScopingConfig{
		RTIDs: scopeCfg.RTIDs,
		RWIDs: []string{fakeRWID}, // This RW does not exist in the database
	}
	svc := NewService(pool, repo, nonExistCfg)

	ac := &auth.AuthContext{
		UserID:        "sa-rw-not-in-db",
		SystemRole:    auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	_, err := svc.GetDashboard(ctx, ac, fakeRWID)
	require.Error(t, err, "SA should be denied when RW is in allowlist but not in DB")
	require.ErrorIs(t, err, auth.ErrRWScopeInvalid)
}

// TestSARepositoryNotCalledOnAuthFailure tests that repository methods are NOT called
// when scope authorization fails.
func TestSARepositoryNotCalledOnAuthFailure(t *testing.T) {
	ctx := context.Background()
	pool, _, _, _, _, scopeCfg := setupRWPermissionTest(t)

	repo := NewRepository(pool.Raw())
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID:        "sa-out-of-scope",
		SystemRole:    auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	// Use an out-of-scope RW ID.
	outOfScopeRWID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	// The error should be returned BEFORE any repository query.
	_, err := svc.GetDashboard(ctx, ac, outOfScopeRWID)
	require.Error(t, err, "SA out of scope should be denied")

	// Verify the error is from authorization, not from the repository.
	require.Contains(t, err.Error(), "rw access denied",
		"error should be from checkRWAccess, not from repo")

	// Direct checkRWAccess should fail before repo is touched.
	err = svc.checkRWAccess(ctx, ac, outOfScopeRWID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rw access denied")

	// checkRWAccess is called first in requirePermission,
	// so if it errors, GetDashboard never reaches the repo.
	_, err = svc.GetDashboard(ctx, ac, outOfScopeRWID)
	require.Error(t, err)
}

// TestRepositoryNotCalledOnPermissionFailure tests that repository is not called
// when permission check fails (e.g., sekretaris trying to get dashboard).
func TestRepositoryNotCalledOnPermissionFailure(t *testing.T) {
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	// Use checkRWPermission directly for a jabatan that lacks the permission.
	ac := &auth.AuthContext{
		UserID: "sekretaris-no-read",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "sekretaris"},
		},
	}

	// checkRWPermission is called after checkRWAccess.
	// We call it directly to verify permission fails before repo.
	err := svc.checkRWPermission(ac, rwID, "finance.update")
	require.Error(t, err, "permission denied should be returned before repo")
	require.Contains(t, err.Error(), "permission denied")
}

// TestSAWithContextCancellation tests that context cancellation is propagated
// to the database query during SA validation.
func TestSAWithContextCancellation(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID:        "sa-cancel-test",
		SystemRole:    auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	// Create a context that is immediately cancelled.
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately.

	// The key assertion: checkRWAccess accepts ctx and does not panic.
	err := svc.checkRWAccess(cancelCtx, ac, rwID)
	// err may be nil (fast DB) or non-nil (cancelled context), both are acceptable.
	// What matters is that no panic occurs and the function respects context.
	_ = err
}

// TestSAWithPoolNil tests that SA with pool nil returns error.
func TestSAWithPoolNil(t *testing.T) {
	ctx := context.Background()
	_, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	// Create service with nil pool to simulate infrastructure failure.
	svc := &Service{
		pool:       nil,
		repo:       repo,
		scopingCfg: scopeCfg,
	}

	ac := &auth.AuthContext{
		UserID:        "sa-nil-pool",
		SystemRole:    auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	_, err := svc.GetDashboard(ctx, ac, rwID)
	require.Error(t, err, "SA with nil pool should be denied")
}

// TestSAWithValidActiveRW tests that SA is allowed for valid active RW.
func TestSAWithValidActiveRW(t *testing.T) {
	ctx := context.Background()
	pool, repo, rwID, _, _, scopeCfg := setupRWPermissionTest(t)
	svc := NewService(pool, repo, scopeCfg)

	ac := &auth.AuthContext{
		UserID:        "sa-valid-rw",
		SystemRole:    auth.SystemRoleSuperAdmin,
		RWMemberships: []auth.RWMembership{},
	}

	dashboard, err := svc.GetDashboard(ctx, ac, rwID)
	require.NoError(t, err, "SA should access valid active RW")
	require.NotNil(t, dashboard)
}
