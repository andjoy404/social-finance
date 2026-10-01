package auth

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"social-finance/internal/testutil"
)

var tenantIDCounter int64

// setupTenantIdentity creates a test RT, user, and membership in the test
// PostgreSQL database using the provided human-readable labels.  Each label is
// deterministically mapped to a valid UUID v4 via testUUID(), so the database
// and JWT claims carry the same identifier.
//
// It sets AuthDBPool so that RequireAuth identity validation succeeds and
// restores the previous value (or nil) on test cleanup.
//
// The email is made unique per test invocation to avoid the unique index on
// lower(trim(email)).
func setupTenantIdentity(t *testing.T, userID, membershipID, rtID, email string, role Role) {
	t.Helper()
	pool := testutil.GetTestPool(t)

	counter := atomic.AddInt64(&tenantIDCounter, 1)
	rtName := fmt.Sprintf("test-tenant-identity-%d", counter)
	rtCode := fmt.Sprintf("test-%d", counter)

	// Convert human-readable labels to valid UUIDs.
	uuidUser := testUUID(userID)
	uuidMember := testUUID(membershipID)
	uuidRT := testUUID(rtID)

	db := pool.Raw()

	// Insert RT (explicit UUID, skip if already present).
	var rtIDActual string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, 99, $3, 'Test Address', 'Test Head', true)
		 ON CONFLICT (id) DO UPDATE SET is_active = true
		 RETURNING id`,
		uuidRT, rtName, rtCode,
	).Scan(&rtIDActual)
	if err != nil {
		t.Fatalf("insert test RT: %v", err)
	}

	// Insert user (explicit UUID, skip if already present).
	// IMPORTANT: system_role is NOT reset on conflict — it stays whatever it
	// was last set to.  Tenant users must always have system_role = NULL.
	var userIDActual string
	err = db.QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, phone, password_hash, full_name, system_role, is_active)
		 VALUES ($1, $2, NULL, 'dummy', 'Test User', NULL, true)
		 ON CONFLICT (id) DO UPDATE SET
			 system_role = NULL,
			 is_active = true
		 RETURNING id`,
		uuidUser, email,
	).Scan(&userIDActual)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	// Insert membership (explicit UUID, skip if already present).
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active)
		 VALUES ($1, $2, $3, $4, true)
		 ON CONFLICT (id) DO UPDATE SET role = $4, is_active = true`,
		uuidMember, userIDActual, rtIDActual, role,
	)
	if err != nil {
		t.Fatalf("insert test membership: %v", err)
	}

	// Wire the pool into RequireAuth for identity validation.
	oldPool := AuthDBPool
	AuthDBPool = pool
	t.Cleanup(func() {
		AuthDBPool = oldPool
	})
}

// setupSystemOnlyIdentity creates a test user with no RT membership in the test
// database using the provided human-readable label.  The label is deterministically
// mapped to a valid UUID v4 via testUUID().
//
// It sets AuthDBPool so that RequireAuth identity validation succeeds and
// restores the previous value (or nil) on test cleanup.
//
// For system-only users ValidateIdentity only checks users.is_active; no RT or
// membership rows are created.
func setupSystemOnlyIdentity(t *testing.T, userID, email string) {
	t.Helper()
	pool := testutil.GetTestPool(t)

	// Convert human-readable label to valid UUID.
	uuidUser := testUUID(userID)

	db := pool.Raw()

	// Insert user with system_role = 'super_admin' (upsert to handle re-use).
	var userIDActual string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, phone, password_hash, full_name, system_role, is_active)
		 VALUES ($1, $2, NULL, 'dummy', 'Test Admin', 'super_admin', true)
		 ON CONFLICT (id) DO UPDATE SET
			 system_role = 'super_admin',
			 is_active = true
		 RETURNING id`,
		uuidUser, email,
	).Scan(&userIDActual)
	if err != nil {
		t.Fatalf("insert test system-only user: %v", err)
	}

	// Wire the pool into RequireAuth for identity validation.
	oldPool := AuthDBPool
	AuthDBPool = pool
	t.Cleanup(func() {
		AuthDBPool = oldPool
	})
}
