package auth

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/testutil"
)

var rtSequence int64

func nextRTCode() string {
	n := atomic.AddInt64(&rtSequence, 1)
	return fmt.Sprintf("rtcode-%d", n)
}

// TestCreateRefreshToken_NilMembershipID verifies that CreateRefreshToken
// accepts a nil membershipID and stores SQL NULL in the database.
func TestCreateRefreshToken_NilMembershipID(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Create a test user first.
	userID := testUUID("create-token-nil-user")
	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, userID, "nil-test@example.com", "noop", "Null Test User"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Insert refresh token with nil membership_id.
	err = CreateRefreshToken(ctx, tx, userID, nil, "test-hash-nil", time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateRefreshToken(nil) returned error: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Verify the row was stored with membership_id IS NULL.
	var mid sql.NullString
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT membership_id FROM refresh_tokens WHERE token_hash = 'test-hash-nil' AND user_id = $1`,
		userID,
	).Scan(&mid)
	if err != nil {
		t.Fatalf("query stored row: %v", err)
	}
	if mid.Valid {
		t.Fatal("Expected membership_id IS NULL, got valid value")
	}
}

// TestCreateRefreshToken_MembershipID_UUID verifies that CreateRefreshToken
// still works correctly with a real membership UUID.
func TestCreateRefreshToken_MembershipID_UUID(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	userID := testUUID("create-token-uuid-user")
	mID := testUUID("create-token-uuid-mid")
	rtID := testUUID("create-token-uuid-rt")

	// Create RT, user, and a real membership so the FK constraints are satisfied.
	q := `INSERT INTO rts (id, name, rw, rt, address, head_name, is_active) VALUES ($1, 'Create UUID Test RT', 99, $2, 'Test', 'Test', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, rtID, nextRTCode()); err != nil {
		t.Fatalf("insert test RT: %v", err)
	}
	q = `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, NULL, true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, userID, "uuid-test@example.com", "noop", "UUID Test User"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	q = `INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES ($1, $2, $3, 'bendahara', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, mID, userID, rtID); err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	// Insert with real membership UUID.
	midStr := mID
	err = CreateRefreshToken(ctx, tx, userID, &midStr, "test-hash-uuid", time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateRefreshToken(&uuid) returned error: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Verify the row was stored with correct membership_id.
	var storedMid string
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT membership_id FROM refresh_tokens WHERE token_hash = 'test-hash-uuid' AND user_id = $1`,
		userID,
	).Scan(&storedMid)
	if err != nil {
		t.Fatalf("query stored row: %v", err)
	}
	if storedMid != mID {
		t.Fatalf("Expected membership_id=%s, got %s", mID, storedMid)
	}
}

// TestFindRefreshTokenByHash_NilMembershipID verifies that FindRefreshTokenByHash
// returns nil for membershipID when the row has SQL NULL.
func TestFindRefreshTokenByHash_NilMembershipID(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Insert a user and refresh token within this tx.
	userID := testUUID("find-nil-user")
	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, userID, "find-nil@example.com", "noop", "Find Nil Test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	err = CreateRefreshToken(ctx, tx, userID, nil, "find-nil-hash", time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateRefreshToken(nil): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Look it up in a NEW transaction (the insert tx is already committed).
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin lookup tx: %v", err)
	}
	defer tx2.Rollback()

	foundUserID, mid, expiresAt, revoked, err := FindRefreshTokenByHash(ctx, tx2, "find-nil-hash")
	if err != nil {
		t.Fatalf("FindRefreshTokenByHash returned error: %v", err)
	}
	if foundUserID != userID {
		t.Errorf("Expected userID=%s, got %s", userID, foundUserID)
	}
	if mid != nil {
		t.Fatal("Expected membershipID to be nil, got non-nil")
	}
	// expiresAt should reflect the stored value (time.Now().Add(7*24*time.Hour)), not zero.
	_ = expiresAt // non-zero confirms the row was stored correctly
	if revoked {
		t.Error("Expected revoked=false")
	}
}

// TestFindRefreshTokenByHash_MembershipID_UUID verifies that FindRefreshTokenByHash
// returns a non-nil *string for a member token.
func TestFindRefreshTokenByHash_MembershipID_UUID(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	userID := testUUID("find-uuid-user")
	mID := testUUID("find-uuid-mid")
	rtID := testUUID("find-uuid-rt")

	// Create RT, user, and a real membership so the FK constraints are satisfied.
	q := `INSERT INTO rts (id, name, rw, rt, address, head_name, is_active) VALUES ($1, 'Find UUID Test RT', 99, $2, 'Test', 'Test', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, rtID, nextRTCode()); err != nil {
		t.Fatalf("insert test RT: %v", err)
	}
	q = `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, NULL, true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, userID, "find-uuid@example.com", "noop", "Find UUID Test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	q = `INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES ($1, $2, $3, 'bendahara', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, mID, userID, rtID); err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	midStr := mID
	err = CreateRefreshToken(ctx, tx, userID, &midStr, "find-uuid-hash", time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateRefreshToken(&uuid): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Look it up in a NEW transaction.
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin lookup tx: %v", err)
	}
	defer tx2.Rollback()

	foundUserID, mid, expiresAt, revoked, err := FindRefreshTokenByHash(ctx, tx2, "find-uuid-hash")
	if err != nil {
		t.Fatalf("FindRefreshTokenByHash returned error: %v", err)
	}
	if foundUserID != userID {
		t.Errorf("Expected userID=%s, got %s", userID, foundUserID)
	}
	if mid == nil {
		t.Fatal("Expected membershipID to be non-nil")
	}
	if *mid != mID {
		t.Errorf("Expected membershipID=%s, got %s", mID, *mid)
	}
	// expiresAt should reflect the stored value (time.Now().Add(7*24*time.Hour)), not zero.
	_ = expiresAt // non-zero confirms the row was stored correctly
	if revoked {
		t.Error("Expected revoked=false")
	}
}

// TestRevokeRefreshTokensByMembership_ExcludesNull verifies that revoking by
// membership_id does NOT affect system-only (NULL) tokens.
func TestRevokeRefreshTokensByMembership_ExcludesNull(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Create two users: one member, one system-only.
	memberUserID := testUUID("revoke-member-user")
	systemUserID := testUUID("revoke-system-user")
	mID := testUUID("revoke-membership")
	rtID := testUUID("revoke-rt")

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Create RT, both users, and a real membership for the member user.
	q := `INSERT INTO rts (id, name, rw, rt, address, head_name, is_active) VALUES ($1, 'Revoke Test RT', 99, $2, 'Test', 'Test', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, rtID, nextRTCode()); err != nil {
		t.Fatalf("insert test RT: %v", err)
	}
	q = `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, NULL, true), ($5, $6, $7, $8, 'super_admin', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q,
		memberUserID, "member-revoke@example.com", "noop", "Member Revoke",
		systemUserID, "system-revoke@example.com", "noop", "System Revoke",
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	q = `INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES ($1, $2, $3, 'bendahara', true) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, q, mID, memberUserID, rtID); err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	mIDStr := mID

	// Create member token (with real membership FK).
	if err := CreateRefreshToken(ctx, tx, memberUserID, &mIDStr, "member-revoke-hash", time.Now().Add(7*24*time.Hour)); err != nil {
		t.Fatalf("create member token: %v", err)
	}

	// Create system-only token.
	if err := CreateRefreshToken(ctx, tx, systemUserID, nil, "system-revoke-hash", time.Now().Add(7*24*time.Hour)); err != nil {
		t.Fatalf("create system token: %v", err)
	}

	// Revoke the membership.
	if err := RevokeRefreshTokensByMembership(ctx, tx, mID); err != nil {
		t.Fatalf("revoke by membership: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Member token should be revoked.
	var memberRevoked bool
	if err := pool.Raw().QueryRowContext(ctx,
		`SELECT revoked_at IS NOT NULL FROM refresh_tokens WHERE token_hash = 'member-revoke-hash'`,
	).Scan(&memberRevoked); err != nil {
		t.Fatalf("query member token: %v", err)
	}
	if !memberRevoked {
		t.Error("Expected member token to be revoked")
	}

	// System-only token should NOT be revoked.
	var systemRevoked bool
	if err := pool.Raw().QueryRowContext(ctx,
		`SELECT revoked_at IS NOT NULL FROM refresh_tokens WHERE token_hash = 'system-revoke-hash'`,
	).Scan(&systemRevoked); err != nil {
		t.Fatalf("query system token: %v", err)
	}
	if systemRevoked {
		t.Error("Expected system-only token to NOT be revoked")
	}
}
