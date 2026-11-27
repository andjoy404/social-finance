package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"social-finance/internal/testutil"
)

// TestMultiDevice_FindRefreshTokenDeviceIDAndScopedLogout verifies:
//  1. FindRefreshTokenByHash returns the real device_id (not membership_id) for member users.
//  2. Logging out on Web (passing Web's refresh_token) revokes ONLY Web's session
//     and keeps Android's persistent refresh token valid and rotatable.
//  3. Refresh preserves stored device_id when the client omits device_id.
//  4. Rotating Android's refresh token immediately revokes the old token (single-use).
func TestMultiDevice_FindRefreshTokenDeviceIDAndScopedLogout(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	suffix := time.Now().UnixNano()

	rtID := testUUID(fmt.Sprintf("multidev-rt-%d", suffix))
	userID := testUUID(fmt.Sprintf("multidev-user-%d", suffix))
	mID := testUUID(fmt.Sprintf("multidev-mid-%d", suffix))
	email := fmt.Sprintf("ketua-multidev-%d@local.com", suffix)
	password := "StrongPass123!"

	pwHash, err := Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin setup tx: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO rts (id, name, rw, rt, address, head_name, is_active) VALUES ($1, 'MultiDev RT', 99, $2, 'Addr', 'Ketua', true)`,
		rtID, fmt.Sprintf("M%d", suffix%100000),
	); err != nil {
		tx.Rollback()
		t.Fatalf("insert rt: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, is_active) VALUES ($1, $2, $3, 'Ketua MultiDev', true)`,
		userID, email, pwHash,
	); err != nil {
		tx.Rollback()
		t.Fatalf("insert user: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active) VALUES ($1, $2, $3, 'pengurus', 'ketua', true)`,
		mID, userID, rtID,
	); err != nil {
		tx.Rollback()
		t.Fatalf("insert membership: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit setup: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(`DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
		_, _ = pool.Raw().Exec(`DELETE FROM user_rt_memberships WHERE id = $1`, mID)
		_, _ = pool.Raw().Exec(`DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Raw().Exec(`DELETE FROM rts WHERE id = $1`, rtID)
	})

	SigningSecret = []byte("test-secret-for-multidevice-auth-32bytes!!")
	svc := NewService(ServiceOptions{RefreshLifetime: 30 * 24 * time.Hour})

	// 1. Login on Android with explicit device_id
	txAndroid, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin android login tx: %v", err)
	}
	androidLogin, err := svc.Login(ctx, txAndroid, email, password, "android-device-uuid-001")
	if err != nil {
		txAndroid.Rollback()
		t.Fatalf("android login failed: %v", err)
	}
	if err := txAndroid.Commit(); err != nil {
		t.Fatalf("commit android login: %v", err)
	}

	// Verify FindRefreshTokenByHash returns real device_id, NOT membership_id!
	txCheck, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin check tx: %v", err)
	}
	_, foundMID, _, revoked, foundDeviceID, err := FindRefreshTokenByHash(ctx, txCheck, androidLogin.RefreshToken)
	txCheck.Rollback()
	if err != nil {
		t.Fatalf("FindRefreshTokenByHash failed: %v", err)
	}
	if foundMID == nil || *foundMID != mID {
		t.Fatalf("expected membershipID=%s, got %v", mID, foundMID)
	}
	if revoked {
		t.Fatal("expected newly issued android refresh token not revoked")
	}
	if foundDeviceID != "android-device-uuid-001" {
		t.Fatalf("expected deviceID=android-device-uuid-001, got %q (must not return membership_id!)", foundDeviceID)
	}

	// 2. Login on Web with same Ketua RT account
	txWeb, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin web login tx: %v", err)
	}
	webLogin, err := svc.Login(ctx, txWeb, email, password, "")
	if err != nil {
		txWeb.Rollback()
		t.Fatalf("web login failed: %v", err)
	}
	if err := txWeb.Commit(); err != nil {
		t.Fatalf("commit web login: %v", err)
	}

	// 3. Web logs out sending its own refresh_token -> ONLY Web session revoked;
	// Android session MUST remain active!
	txWebLogout, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin web logout tx: %v", err)
	}
	if err := svc.LogoutSession(ctx, txWebLogout, userID, mID, webLogin.RefreshToken, ""); err != nil {
		txWebLogout.Rollback()
		t.Fatalf("web scoped logout failed: %v", err)
	}
	if err := txWebLogout.Commit(); err != nil {
		t.Fatalf("commit web logout: %v", err)
	}

	// Web's refresh token must now be rejected.
	txWebRef, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin web refresh tx: %v", err)
	}
	if _, err := svc.Refresh(ctx, txWebRef, webLogin.RefreshToken, ""); err != ErrInvalidRefreshToken {
		txWebRef.Rollback()
		t.Fatalf("expected ErrInvalidRefreshToken for logged-out web token, got %v", err)
	}
	txWebRef.Rollback()

	// Android's refresh token MUST still succeed and preserve its device_id!
	txAndRef, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin android refresh tx: %v", err)
	}
	rotated, err := svc.Refresh(ctx, txAndRef, androidLogin.RefreshToken, "")
	if err != nil {
		txAndRef.Rollback()
		t.Fatalf("expected android refresh to succeed after web logout, got: %v", err)
	}
	if err := txAndRef.Commit(); err != nil {
		t.Fatalf("commit android refresh: %v", err)
	}
	if rotated.DeviceID != "android-device-uuid-001" {
		t.Errorf("expected rotated token to preserve device_id=android-device-uuid-001, got %q", rotated.DeviceID)
	}
	if rotated.RefreshToken == "" || rotated.RefreshToken == androidLogin.RefreshToken {
		t.Errorf("expected single-use rotation to issue a brand new refresh token")
	}

	// Old Android refresh token must now be rejected (single-use rotation).
	txOld, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin old token check tx: %v", err)
	}
	if _, err := svc.Refresh(ctx, txOld, androidLogin.RefreshToken, "android-device-uuid-001"); err != ErrInvalidRefreshToken {
		txOld.Rollback()
		t.Fatalf("expected ErrInvalidRefreshToken for reused old refresh token, got %v", err)
	}
	txOld.Rollback()
}
