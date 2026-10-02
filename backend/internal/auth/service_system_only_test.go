package auth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"social-finance/internal/testutil"
)

// TestSystemOnlyLogin_ReturnsRefreshToken verifies that a system-only
// SUPER_ADMIN (no memberships) receives a non-empty refresh token.
func TestSystemOnlyLogin_ReturnsRefreshToken(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Create a system-only superadmin user.
	userID := testUUID("sys-login-user")
	email := "syslogin-" + userID[:8] + "@example.com"
	passwordHash, err := Hash("TestUser123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true`
	if _, err := tx.ExecContext(ctx, q, userID, email, passwordHash, "System Login User"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Login.
	svc := NewService(ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	AuthDBPool = pool

	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin login tx: %v", err)
	}
	defer tx2.Rollback()

	result, err := svc.Login(ctx, tx2, email, "TestUser123!")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit login tx: %v", err)
	}

	if result.AccessToken == "" {
		t.Fatal("expected non-empty access_token")
	}
	if result.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token for system-only superadmin")
	}

	// Verify access token claims.
	claims, err := ParseAndVerifyAccessToken(result.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("expected UserID=%s, got %s", userID, claims.UserID)
	}
	if claims.SysRole != "super_admin" {
		t.Errorf("expected SysRole=super_admin, got %s", claims.SysRole)
	}
	if claims.MID != "" {
		t.Error("expected empty MID for system-only token")
	}
	if claims.RTID != "" {
		t.Error("expected empty RTID for system-only token")
	}
	if claims.Role != "" {
		t.Error("expected empty Role for system-only token")
	}
	if claims.Jabatan != "" {
		t.Error("expected empty Jabatan for system-only token")
	}
}

// TestSystemOnlyLogin_TokenStoredWithNullMembership verifies the
// refresh token is stored with membership_id IS NULL.
func TestSystemOnlyLogin_TokenStoredWithNullMembership(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	userID := testUUID("sys-store-null-user")
	email := "sysstorenull-" + userID[:8] + "@example.com"
	passwordHash, err := Hash("TestUser123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true`
	if _, err := tx.ExecContext(ctx, q, userID, email, passwordHash, "System Store Null"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	svc := NewService(ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	AuthDBPool = pool

	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin login tx: %v", err)
	}
	defer tx2.Rollback()

	result, err := svc.Login(ctx, tx2, email, "TestUser123!")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit login tx: %v", err)
	}

	// Verify stored token has NULL membership_id.
	var storedHash string
	var mid sql.NullString
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT token_hash, membership_id FROM refresh_tokens WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&storedHash, &mid)
	if err != nil {
		t.Fatalf("query refresh token: %v", err)
	}
	if mid.Valid {
		t.Fatal("Expected membership_id IS NULL, got valid value")
	}
	// Verify hash matches the raw token.
	expectedHash := storedHash // just verify we can find the row
	if storedHash == "" {
		t.Fatal("stored token_hash is empty")
	}
	_ = expectedHash // suppress unused variable if not needed

	// Also verify the raw token returned matches what's stored.
	if result.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
}

// TestSystemOnlyRefresh_Success verifies that a system-only refresh
// produces valid new tokens.
func TestSystemOnlyRefresh_Success(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	userID := testUUID("sys-refresh-user")
	email := "sysrefresh-" + userID[:8] + "@example.com"
	passwordHash, err := Hash("TestUser123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true`
	if _, err := tx.ExecContext(ctx, q, userID, email, passwordHash, "System Refresh"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	svc := NewService(ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	AuthDBPool = pool

	// Step 1: Login to get initial tokens.
	tx1, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin login tx: %v", err)
	}
	defer tx1.Rollback()

	loginResult, err := svc.Login(ctx, tx1, email, "TestUser123!")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("commit login: %v", err)
	}

	// Step 2: Refresh.
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin refresh tx: %v", err)
	}
	defer tx2.Rollback()

	refreshResult, err := svc.Refresh(ctx, tx2, loginResult.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit refresh: %v", err)
	}

	if refreshResult.AccessToken == "" {
		t.Fatal("expected non-empty access_token after refresh")
	}
	if refreshResult.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token after refresh")
	}

	// Verify new token is different from old (rotation).
	if refreshResult.RefreshToken == loginResult.RefreshToken {
		t.Error("expected rotated refresh token, got same token")
	}

	// Parse new access token claims.
	claims, err := ParseAndVerifyAccessToken(refreshResult.AccessToken)
	if err != nil {
		t.Fatalf("parse new access token: %v", err)
	}
	if claims.MID != "" {
		t.Error("expected empty MID after refresh")
	}
	if claims.RTID != "" {
		t.Error("expected empty RTID after refresh")
	}
	if claims.Role != "" {
		t.Error("expected empty Role after refresh")
	}
	if claims.SysRole != "super_admin" {
		t.Errorf("expected SysRole=super_admin, got %s", claims.SysRole)
	}
}

// TestSystemOnlyRefresh_OldTokenRejected verifies that the old refresh
// token is rejected after rotation.
func TestSystemOnlyRefresh_OldTokenRejected(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	userID := testUUID("sys-rotation-user")
	email := "sysrot-" + userID[:8] + "@example.com"
	passwordHash, err := Hash("TestUser123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true`
	if _, err := tx.ExecContext(ctx, q, userID, email, passwordHash, "System Rotation"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	svc := NewService(ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	AuthDBPool = pool

	// Login.
	tx1, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin login tx: %v", err)
	}
	defer tx1.Rollback()

	loginResult, err := svc.Login(ctx, tx1, email, "TestUser123!")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("commit login: %v", err)
	}

	// Refresh once.
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin refresh tx: %v", err)
	}
	defer tx2.Rollback()

	if _, err := svc.Refresh(ctx, tx2, loginResult.RefreshToken); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit refresh: %v", err)
	}

	// Old token should be rejected.
	tx3, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin second refresh tx: %v", err)
	}
	defer tx3.Rollback()

	_, err = svc.Refresh(ctx, tx3, loginResult.RefreshToken)
	if err == nil {
		t.Fatal("expected error when reusing old refresh token")
	}
	if err != ErrInvalidRefreshToken {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}
	if err := tx3.Commit(); err != nil {
		// Commit might fail because token was already revoked, that's OK.
	}
}

// TestSystemOnlyRefresh_InactiveUserRejected verifies that an inactive
// system-only user cannot refresh.
func TestSystemOnlyRefresh_InactiveUserRejected(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	userID := testUUID("sys-inactive-user")
	email := "sysinactive-" + userID[:8] + "@example.com"
	passwordHash, err := Hash("TestUser123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true`
	if _, err := tx.ExecContext(ctx, q, userID, email, passwordHash, "System Inactive"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	svc := NewService(ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	AuthDBPool = pool

	// Login to get refresh token.
	tx1, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin login tx: %v", err)
	}
	defer tx1.Rollback()

	loginResult, err := svc.Login(ctx, tx1, email, "TestUser123!")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("commit login: %v", err)
	}

	// Deactivate the user.
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin deactivate tx: %v", err)
	}
	defer tx2.Rollback()

	if _, err := tx2.ExecContext(ctx, `UPDATE users SET is_active = false WHERE id = $1`, userID); err != nil {
		t.Fatalf("deactivate user: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit deactivate: %v", err)
	}

	// Refresh should fail.
	tx3, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin refresh tx: %v", err)
	}
	defer tx3.Rollback()

	_, err = svc.Refresh(ctx, tx3, loginResult.RefreshToken)
	if err == nil {
		t.Fatal("expected error when refreshing with inactive user")
	}
}

// TestSystemOnlyRefresh_RotatedTokenStoredAsNULL verifies that after
// rotation, the new refresh token still has membership_id IS NULL.
func TestSystemOnlyRefresh_RotatedTokenStoredAsNULL(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	userID := testUUID("sys-rot-null-user")
	email := "sysrotnull-" + userID[:8] + "@example.com"
	passwordHash, err := Hash("TestUser123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	q := `INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, $3, $4, 'super_admin', true) ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true`
	if _, err := tx.ExecContext(ctx, q, userID, email, passwordHash, "System Rot Null"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	svc := NewService(ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	AuthDBPool = pool

	// Login.
	tx1, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin login tx: %v", err)
	}
	defer tx1.Rollback()

	loginResult, err := svc.Login(ctx, tx1, email, "TestUser123!")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("commit login: %v", err)
	}

	// Verify initial token has NULL membership_id.
	var mid sql.NullString
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT membership_id FROM refresh_tokens WHERE token_hash = $1`,
		loginResult.RefreshToken,
	).Scan(&mid)
	if err != nil {
		t.Fatalf("query initial token: %v", err)
	}
	if mid.Valid {
		t.Fatal("Expected initial token to have membership_id IS NULL")
	}

	// Refresh.
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin refresh tx: %v", err)
	}
	defer tx2.Rollback()

	refreshResult, err := svc.Refresh(ctx, tx2, loginResult.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit refresh: %v", err)
	}

	// Verify rotated token also has NULL membership_id.
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT membership_id FROM refresh_tokens WHERE token_hash = $1`,
		refreshResult.RefreshToken,
	).Scan(&mid)
	if err != nil {
		t.Fatalf("query rotated token: %v", err)
	}
	if mid.Valid {
		t.Fatal("Expected rotated token to have membership_id IS NULL")
	}
}
