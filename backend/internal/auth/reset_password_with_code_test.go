package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

func TestResetPasswordWithCode(t *testing.T) {
	pool := setupTestDB(t)

	// Create user
	userID := "11111111-1111-1111-1111-111111111111"
	userEmail := "warga.reset1@example.com"
	initialPass := "InitialPass123"
	initialHash, _ := auth.Hash(initialPass)

	_, err := pool.Raw().Exec(`
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, $3, 'Warga Reset 1', true)
		ON CONFLICT (id) DO UPDATE SET password_hash = $3
	`, userID, userEmail, initialHash)
	if err != nil {
		t.Fatalf("setup user: %v", err)
	}

	// Get existing RT
	var rtID string
	err = pool.Raw().QueryRow(`SELECT id FROM rts LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("query rt: %v", err)
	}

	// Create resident
	resID := "22222222-2222-2222-2222-222222222222"
	_, _ = pool.Raw().Exec(`
		INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		VALUES ($1, $2, 'warga', true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
	`, userID, rtID)
	_, err = pool.Raw().Exec(`
		INSERT INTO residents (id, rt_id, user_id, full_name, email, is_active)
		VALUES ($1, $2, $3, 'Warga Reset 1', $4, true)
		ON CONFLICT (id) DO UPDATE SET user_id = $3
	`, resID, rtID, userID, userEmail)
	if err != nil {
		t.Fatalf("setup resident: %v", err)
	}

	// Insert active 4-digit reset code "0429"
	code := "0429"
	codeHash, _ := auth.HashCode(code)
	codeID := "44444444-4444-4444-4444-444444444444"
	_, err = pool.Raw().Exec(`
		INSERT INTO resident_password_reset_codes (id, user_id, resident_id, rt_id, code_hash, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, now() + interval '24 hours', $2)
		ON CONFLICT (id) DO UPDATE SET code_hash = $5, used_at = NULL, expires_at = now() + interval '24 hours'
	`, codeID, userID, resID, rtID, codeHash)
	if err != nil {
		t.Fatalf("setup code: %v", err)
	}

	svc := auth.NewService()
	h := auth.NewHandler(pool, svc)

	// 1. Wrong code fails
	wrongBody, _ := json.Marshal(map[string]string{
		"identifier":       userEmail,
		"code":             "9999",
		"new_password":     "NewSecurePass123",
		"confirm_password": "NewSecurePass123",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/reset-password-with-code", bytes.NewReader(wrongBody))
	w := httptest.NewRecorder()
	h.ResetPasswordWithCode().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong code, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Mismatched passwords fail
	mismatchBody, _ := json.Marshal(map[string]string{
		"identifier":       userEmail,
		"code":             code,
		"new_password":     "NewSecurePass123",
		"confirm_password": "DifferentPass123",
	})
	req = httptest.NewRequest("POST", "/api/v1/auth/reset-password-with-code", bytes.NewReader(mismatchBody))
	w = httptest.NewRecorder()
	h.ResetPasswordWithCode().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for password mismatch, got %d", w.Code)
	}

	// 3. Valid code succeeds
	validBody, _ := json.Marshal(map[string]string{
		"identifier":       userEmail,
		"code":             code,
		"new_password":     "NewSecurePass123",
		"confirm_password": "NewSecurePass123",
	})
	req = httptest.NewRequest("POST", "/api/v1/auth/reset-password-with-code", bytes.NewReader(validBody))
	w = httptest.NewRecorder()
	h.ResetPasswordWithCode().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid reset, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Verify password was updated by logging in with new password
	tx, _ := pool.BeginTx(context.Background())
	defer tx.Rollback()
	res, err := svc.Login(context.Background(), tx, userEmail, "NewSecurePass123", "device-1")
	if err != nil || res == nil {
		t.Fatalf("expected login with new password to succeed: %v", err)
	}

	// 5. Code is now used; reusing it should fail
	reuseBody, _ := json.Marshal(map[string]string{
		"identifier":       userEmail,
		"code":             code,
		"new_password":     "AnotherNewPass123",
		"confirm_password": "AnotherNewPass123",
	})
	req = httptest.NewRequest("POST", "/api/v1/auth/reset-password-with-code", bytes.NewReader(reuseBody))
	w = httptest.NewRecorder()
	h.ResetPasswordWithCode().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for already used code, got %d", w.Code)
	}
}

func setupTestDB(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}
