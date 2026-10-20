package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-finance/internal/auth"
)

func TestProfileAndUpdatePassword(t *testing.T) {
	handler, pool := setupEffectivePermissionTest(t)
	t.Cleanup(func() {
		resetHash, _ := auth.Hash("123qweasdzxc")
		_, _ = pool.Raw().ExecContext(context.Background(), `UPDATE users SET email = 'warga.rt03@local.com', password_hash = $1 WHERE id = '22220001-0000-4000-8000-000000000001'`, resetHash)
	})

	// 1. Login as warga to get access token and context
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "warga.rt03@local.com",
		"password": "123qweasdzxc",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	handler.Login().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	var loginResp auth.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}

	// Helper to make authenticated request
	makeAuthedReq := func(method, path string, body any, handlerFunc http.Handler) *httptest.ResponseRecorder {
		var bodyReader *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(b)
		} else {
			bodyReader = bytes.NewReader([]byte{})
		}
		r := httptest.NewRequest(method, path, bodyReader)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)

		// Run through RequireAuth middleware
		rec := httptest.NewRecorder()
		auth.RequireAuth(handlerFunc).ServeHTTP(rec, r)
		return rec
	}

	// 2. Test PUT /api/v1/auth/profile
	t.Run("UpdateProfile_Success", func(t *testing.T) {
		newPhone := "081298918220"
		newEmail := "warga.rt03.updated@local.com"
		updateBody := auth.UpdateProfileRequest{
			FullName: "Warga RT03 Updated",
			Phone:    &newPhone,
			Email:    &newEmail,
		}
		resp := makeAuthedReq(http.MethodPut, "/api/v1/auth/profile", updateBody, handler.UpdateProfile())
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var updatedProfile auth.UserForMe
		if err := json.Unmarshal(resp.Body.Bytes(), &updatedProfile); err != nil {
			t.Fatalf("unmarshal updated profile: %v", err)
		}
		if updatedProfile.Fullname != "Warga RT03 Updated" {
			t.Errorf("expected full_name 'Warga RT03 Updated', got %q", updatedProfile.Fullname)
		}
	})

	// 3. Test PUT /api/v1/auth/password with wrong current password
	t.Run("ChangePassword_WrongCurrentPassword", func(t *testing.T) {
		changeBody := auth.ChangePasswordRequest{
			CurrentPassword: "WrongPassword123!",
			NewPassword:     "NewPassword123!",
		}
		resp := makeAuthedReq(http.MethodPut, "/api/v1/auth/password", changeBody, handler.ChangePassword())
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for wrong current password, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	// 4. Test PUT /api/v1/auth/password with too short new password
	t.Run("ChangePassword_ShortNewPassword", func(t *testing.T) {
		changeBody := auth.ChangePasswordRequest{
			CurrentPassword: "123qweasdzxc",
			NewPassword:     "short",
		}
		resp := makeAuthedReq(http.MethodPut, "/api/v1/auth/password", changeBody, handler.ChangePassword())
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for short password, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	// 5. Test PUT /api/v1/auth/password success and relogin
	t.Run("ChangePassword_SuccessAndRelogin", func(t *testing.T) {
		changeBody := auth.ChangePasswordRequest{
			CurrentPassword: "123qweasdzxc",
			NewPassword:     "NewPassword123!",
		}
		resp := makeAuthedReq(http.MethodPut, "/api/v1/auth/password", changeBody, handler.ChangePassword())
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		// Login with old password must now fail
		oldLogin, _ := json.Marshal(map[string]string{
			"email":    "warga.rt03@local.com",
			"password": "123qweasdzxc",
		})
		rOld := httptest.NewRecorder()
		handler.Login().ServeHTTP(rOld, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(oldLogin)))
		if rOld.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 when logging in with old password, got %d", rOld.Code)
		}

		// Login with new password must succeed
		newLogin, _ := json.Marshal(map[string]string{
			"email":    "warga.rt03.updated@local.com",
			"password": "NewPassword123!",
		})
		rNew := httptest.NewRecorder()
		handler.Login().ServeHTTP(rNew, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(newLogin)))
		if rNew.Code != http.StatusOK {
			t.Fatalf("expected 200 when logging in with new password, got %d: %s", rNew.Code, rNew.Body.String())
		}

		// Restore original password for test idempotency
		_ = pool
		var resetHash string
		resetHash, _ = auth.Hash("123qweasdzxc")
		_, _ = pool.Raw().ExecContext(context.Background(), `UPDATE users SET password_hash = $1 WHERE email = 'warga.rt03@local.com'`, resetHash)
	})
}
