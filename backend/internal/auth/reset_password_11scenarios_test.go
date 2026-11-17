package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/household"
)

// TestResetPassword11Scenarios validates all required scenarios end-to-end:
// 1. Generate code & reset using that code
// 2. Multi-generate invalidates previous code (Code 1 rejected with code_used)
// 3. Reset using latest code (Code 2) succeeds
// 4. Old password rejected after successful reset
// 5. New password succeeds for login
// 6. Code cannot be used twice (second reset attempt returns code_used)
// 7. Reset works with both email and phone identifier (with phone canonicalization)
// 8. Cross-client: Reset via Web endpoint, login with Android device headers/DeviceID
// 9. Cross-client: Reset with Android DeviceID, login via Web client
// 10. Rejection: Wrong code returns invalid_code; Expired code returns code_expired
// 11. Security: Password unchanged when reset attempt fails
// 12. UX Guard: 4-digit reset code entered in login password field returns reset_code_entered
func TestResetPassword11Scenarios(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()

	// 1. Setup RT
	var rtID string
	err := pool.Raw().QueryRow(`SELECT id FROM rts LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("query rt: %v", err)
	}

	// 2. Setup Pengurus Officer
	pengurusID := "a1111111-1111-1111-1111-111111111111"
	pengurusEmail := "pengurus.scenarios@example.com"
	passHash, _ := auth.Hash("PengurusPass123!")
	_, _ = pool.Raw().Exec(`
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, $3, 'Pengurus Skenario', true)
		ON CONFLICT (id) DO UPDATE SET password_hash = $3
	`, pengurusID, pengurusEmail, passHash)
	_, _ = pool.Raw().Exec(`
		INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		VALUES ($1, $2, 'pengurus', true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
	`, pengurusID, rtID)

	// 3. Setup Resident User A (Email based: warga.a@example.com)
	userAID := "a2222222-2222-2222-2222-222222222222"
	userAEmail := "warga.a@example.com"
	userAOldPass := "OldPassUserA123"
	userAOldHash, _ := auth.Hash(userAOldPass)
	_, _ = pool.Raw().Exec(`
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, $3, 'Warga Skenario A', true)
		ON CONFLICT (id) DO UPDATE SET password_hash = $3
	`, userAID, userAEmail, userAOldHash)
	_, _ = pool.Raw().Exec(`
		INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		VALUES ($1, $2, 'warga', true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
	`, userAID, rtID)

	resAID := "a3333333-3333-3333-3333-333333333333"
	_, err = pool.Raw().Exec(`
		INSERT INTO residents (id, rt_id, user_id, full_name, email, is_active)
		VALUES ($1, $2, $3, 'Warga Skenario A', $4, true)
		ON CONFLICT (id) DO UPDATE SET user_id = $3
	`, resAID, rtID, userAID, userAEmail)
	if err != nil {
		t.Fatalf("setup resident A: %v", err)
	}

	// 4. Setup Resident User B (Phone based: 081299998888 -> stored canon +6281299998888)
	userBID := "b2222222-2222-2222-2222-222222222222"
	userBEmail := "warga.b@example.com"
	userBPhoneCanon := "+6281299998888"
	userBPhoneLocal := "081299998888"
	userBOldPass := "OldPassUserB123"
	userBOldHash, _ := auth.Hash(userBOldPass)
	_, _ = pool.Raw().Exec(`
		INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		VALUES ($1, $2, $3, $4, 'Warga Skenario B', true)
		ON CONFLICT (id) DO UPDATE SET password_hash = $4, phone = $3
	`, userBID, userBEmail, userBPhoneCanon, userBOldHash)
	_, _ = pool.Raw().Exec(`
		INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		VALUES ($1, $2, 'warga', true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
	`, userBID, rtID)

	resBID := "b3333333-3333-3333-3333-333333333333"
	_, err = pool.Raw().Exec(`
		INSERT INTO residents (id, rt_id, user_id, full_name, phone, is_active)
		VALUES ($1, $2, $3, 'Warga Skenario B', $4, true)
		ON CONFLICT (id) DO UPDATE SET user_id = $3, phone = $4
	`, resBID, rtID, userBID, userBPhoneCanon)
	if err != nil {
		t.Fatalf("setup resident B: %v", err)
	}

	// Handlers & Router setup
	authSvc := auth.NewService()
	authH := auth.NewHandler(pool, authSvc)
	hhH := household.NewHandler(pool)

	r := chi.NewRouter()
	r.Post("/api/v1/residents/{id}/reset-password-code", hhH.GenerateResetPasswordCode)
	r.Method(http.MethodPost, "/api/v1/auth/reset-password-with-code", authH.ResetPasswordWithCode())
	r.Method(http.MethodPost, "/api/v1/auth/login", authH.Login())

	// Helper: Officer generates code for resident
	generateCode := func(targetResidentID string) string {
		req := httptest.NewRequest("POST", "/api/v1/residents/"+targetResidentID+"/reset-password-code", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
			UserID:     pengurusID,
			RTID:       rtID,
			TenantRole: auth.RolePengurus,
		}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("failed to generate code for resident %s: %d %s", targetResidentID, w.Code, w.Body.String())
		}
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		code, ok := resp["code"].(string)
		if !ok || len(code) != 4 {
			t.Fatalf("invalid code returned: %v", resp["code"])
		}
		return code
	}

	// Helper: Post reset password
	postReset := func(identifier, code, newPass, confirmPass string) (int, map[string]any) {
		body, _ := json.Marshal(map[string]string{
			"identifier":       identifier,
			"code":             code,
			"new_password":     newPass,
			"confirm_password": confirmPass,
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/reset-password-with-code", bytes.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}

	// Helper: Post login
	postLogin := func(identifier, password, deviceID string) (int, map[string]any) {
		body, _ := json.Marshal(map[string]string{
			"identifier": identifier,
			"password":   password,
			"device_id":  deviceID,
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		if deviceID != "" {
			req.Header.Set("X-Device-Id", deviceID)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}

	// =========================================================================
	// SCENARIO 1 & 2 & 3: Multi-generate invalidates code 1; code 2 succeeds
	// =========================================================================
	t.Run("Scenario 1, 2, 3: Multi-generate invalidation and successful reset with latest code", func(t *testing.T) {
		code1 := generateCode(resAID)
		time.Sleep(10 * time.Millisecond)
		code2 := generateCode(resAID)

		if code1 == code2 {
			t.Logf("Notice: randomly generated codes happened to match: %s", code1)
		}

		// Scenario 2: Code 1 must be rejected with code_used
		status1, resp1 := postReset(userAEmail, code1, "BrandNewPass123!", "BrandNewPass123!")
		if status1 != http.StatusBadRequest {
			t.Fatalf("expected code 1 to be rejected with 400, got %d", status1)
		}
		errObj, _ := resp1["error"].(map[string]any)
		if errObj["code"] != "code_used" {
			t.Fatalf("expected error code 'code_used' for invalidated code 1, got %v", errObj["code"])
		}

		// Scenario 3: Code 2 must succeed
		status2, _ := postReset(userAEmail, code2, "BrandNewPass123!", "BrandNewPass123!")
		if status2 != http.StatusOK {
			t.Fatalf("expected code 2 to succeed with 200, got %d", status2)
		}
	})

	// =========================================================================
	// SCENARIO 4 & 5: Old password rejected, New password succeeds
	// =========================================================================
	t.Run("Scenario 4 & 5: Old password rejected and new password succeeds", func(t *testing.T) {
		// Scenario 4: Old password rejected
		statusOld, _ := postLogin(userAEmail, userAOldPass, "web-client-1")
		if statusOld != http.StatusUnauthorized {
			t.Fatalf("expected old password to be rejected with 401, got %d", statusOld)
		}

		// Scenario 5: New password succeeds
		statusNew, respNew := postLogin(userAEmail, "BrandNewPass123!", "web-client-1")
		if statusNew != http.StatusOK {
			t.Fatalf("expected new password login to succeed with 200, got %d", statusNew)
		}
		if respNew["access_token"] == nil || respNew["access_token"] == "" {
			t.Fatalf("expected access_token in login response")
		}
	})

	// =========================================================================
	// SCENARIO 6: Code cannot be used twice (second reset attempt returns code_used)
	// =========================================================================
	t.Run("Scenario 6: Code cannot be used twice", func(t *testing.T) {
		// Generate fresh code for User A
		codeFresh := generateCode(resAID)

		// First use succeeds
		st1, _ := postReset(userAEmail, codeFresh, "NextPass456!", "NextPass456!")
		if st1 != http.StatusOK {
			t.Fatalf("expected first reset to succeed, got %d", st1)
		}

		// Second use fails with code_used
		st2, resp2 := postReset(userAEmail, codeFresh, "AnotherPass789!", "AnotherPass789!")
		if st2 != http.StatusBadRequest {
			t.Fatalf("expected second reset attempt to fail with 400, got %d", st2)
		}
		errObj, _ := resp2["error"].(map[string]any)
		if errObj["code"] != "code_used" {
			t.Fatalf("expected error code 'code_used', got %v", errObj["code"])
		}
	})

	// =========================================================================
	// SCENARIO 7: Reset with Phone Identifier (local format 0812... vs canon +62812...)
	// =========================================================================
	t.Run("Scenario 7: Reset with local phone format and canonical phone matching", func(t *testing.T) {
		codeB := generateCode(resBID)

		// Submit reset using 081299998888 (local phone format)
		stReset, _ := postReset(userBPhoneLocal, codeB, "PhonePass123!", "PhonePass123!")
		if stReset != http.StatusOK {
			t.Fatalf("expected phone reset with 0812... to succeed, got %d", stReset)
		}

		// Login with canonical phone (+6281299998888) using new password
		stLogin, respLogin := postLogin(userBPhoneCanon, "PhonePass123!", "android-device-phone")
		if stLogin != http.StatusOK {
			t.Fatalf("expected login with new password and canonical phone to succeed, got %d", stLogin)
		}
		if respLogin["access_token"] == nil {
			t.Fatalf("missing token on phone login")
		}
	})

	// =========================================================================
	// SCENARIO 8: Cross-client Web reset -> Android login
	// =========================================================================
	t.Run("Scenario 8: Cross-client: Reset via Web and login with Android device headers", func(t *testing.T) {
		code := generateCode(resAID)

		// Web reset
		stReset, _ := postReset(userAEmail, code, "CrossWebToAndroid123!", "CrossWebToAndroid123!")
		if stReset != http.StatusOK {
			t.Fatalf("reset failed: %d", stReset)
		}

		// Android login with device ID
		stLogin, respLogin := postLogin(userAEmail, "CrossWebToAndroid123!", "android-fcm-device-1234")
		if stLogin != http.StatusOK {
			t.Fatalf("expected android login to succeed, got %d", stLogin)
		}
		if respLogin["access_token"] == nil {
			t.Fatalf("missing access token")
		}
	})

	// =========================================================================
	// SCENARIO 9: Cross-client Android reset -> Web login
	// =========================================================================
	t.Run("Scenario 9: Cross-client: Reset with Android DeviceID and login via Web client", func(t *testing.T) {
		code := generateCode(resAID)

		// Android reset request (with device headers)
		body, _ := json.Marshal(map[string]string{
			"identifier":       userAEmail,
			"code":             code,
			"new_password":     "CrossAndroidToWeb123!",
			"confirm_password": "CrossAndroidToWeb123!",
		})
		req := httptest.NewRequest("POST", "/api/v1/auth/reset-password-with-code", bytes.NewReader(body))
		req.Header.Set("X-Device-Id", "android-physical-device-id")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected reset with android device to succeed, got %d", w.Code)
		}

		// Web login (no device headers)
		stLogin, respLogin := postLogin(userAEmail, "CrossAndroidToWeb123!", "")
		if stLogin != http.StatusOK {
			t.Fatalf("expected web login to succeed, got %d", stLogin)
		}
		if respLogin["access_token"] == nil {
			t.Fatalf("missing access token on web login")
		}
	})

	// =========================================================================
	// SCENARIO 10: Wrong code rejected (invalid_code) & Expired code rejected (code_expired)
	// =========================================================================
	t.Run("Scenario 10: Rejections: Wrong code and expired code", func(t *testing.T) {
		code := generateCode(resAID)

		// 1. Wrong 4-digit code
		wrongCode := "0000"
		if code == "0000" {
			wrongCode = "9999"
		}
		stWrong, respWrong := postReset(userAEmail, wrongCode, "ShouldFailPass123!", "ShouldFailPass123!")
		if stWrong != http.StatusBadRequest {
			t.Fatalf("expected 400 for wrong code, got %d", stWrong)
		}
		errWrong, _ := respWrong["error"].(map[string]any)
		if errWrong["code"] != "invalid_code" {
			t.Fatalf("expected error code 'invalid_code', got %v", errWrong["code"])
		}

		// 2. Expire the code manually in DB
		_, err = pool.Raw().ExecContext(ctx, `
			UPDATE resident_password_reset_codes
			SET expires_at = now() - interval '1 hour'
			WHERE user_id = $1 AND used_at IS NULL
		`, userAID)
		if err != nil {
			t.Fatalf("expire code in db: %v", err)
		}

		// Attempt reset with the expired code
		stExp, respExp := postReset(userAEmail, code, "ShouldFailPass123!", "ShouldFailPass123!")
		if stExp != http.StatusBadRequest {
			t.Fatalf("expected 400 for expired code, got %d", stExp)
		}
		errExp, _ := respExp["error"].(map[string]any)
		if errExp["code"] != "code_expired" {
			t.Fatalf("expected error code 'code_expired', got %v", errExp["code"])
		}
	})

	// =========================================================================
	// SCENARIO 11: Security: Password remains unchanged when reset attempt fails
	// =========================================================================
	t.Run("Scenario 11: Password remains unchanged if reset fails", func(t *testing.T) {
		// User A current working password is "CrossAndroidToWeb123!" from scenario 9
		activePass := "CrossAndroidToWeb123!"

		// Failed reset with wrong identifier
		stBadIdent, _ := postReset("nonexistent@example.com", "1234", "MaliciousPass123!", "MaliciousPass123!")
		if stBadIdent != http.StatusBadRequest {
			t.Fatalf("expected bad request for nonexistent identifier, got %d", stBadIdent)
		}

		// Verify original active password still works
		stLogin, _ := postLogin(userAEmail, activePass, "security-audit")
		if stLogin != http.StatusOK {
			t.Fatalf("expected user password to remain intact and login to succeed, got %d", stLogin)
		}

		// Verify candidate password fails
		stCandidate, _ := postLogin(userAEmail, "MaliciousPass123!", "security-audit")
		if stCandidate != http.StatusUnauthorized {
			t.Fatalf("expected candidate password to be rejected, got %d", stCandidate)
		}
	})

	// =========================================================================
	// SCENARIO 12: UX Guard: 4-digit reset code in login password returns reset_code_entered
	// =========================================================================
	t.Run("Scenario 12: UX Guard returns reset_code_entered when user enters 4-digit code in login", func(t *testing.T) {
		codeGuard := generateCode(resAID)

		// User accidentally enters 4-digit reset code into standard login password box
		stLogin, respLogin := postLogin(userAEmail, codeGuard, "device-accident")
		if stLogin != http.StatusBadRequest {
			t.Fatalf("expected 400 for reset_code_entered, got %d", stLogin)
		}
		errObj, _ := respLogin["error"].(map[string]any)
		if errObj["code"] != "reset_code_entered" {
			t.Fatalf("expected error code 'reset_code_entered', got %v", errObj["code"])
		}
		msg, _ := errObj["message"].(string)
		if msg == "" {
			t.Fatalf("expected informative guidance message in error")
		}
		t.Logf("Friendly guidance returned: %s", msg)
	})

	t.Run("Scenario 13: Jabatan-based Ketua RT generates code and resident successfully resets password", func(t *testing.T) {
		ketuaUID := "98989898-9898-9898-9898-989898989898"
		_, _ = pool.Raw().Exec(`
			INSERT INTO users (id, email, password_hash, full_name, is_active)
			VALUES ($1, 'ketua.rt.reset@example.com', $2, 'Pak Ketua RT', true)
			ON CONFLICT (id) DO UPDATE SET is_active = true
		`, ketuaUID, passHash)

		req := httptest.NewRequest("POST", "/api/v1/residents/"+resAID+"/reset-password-code", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
			UserID:     ketuaUID,
			RTID:       rtID,
			TenantRole: auth.RoleWarga,
			Jabatan:    auth.JabatanKetua,
		}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK when Ketua RT generates code, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		ketuaCode, ok := resp["code"].(string)
		if !ok || len(ketuaCode) != 4 {
			t.Fatalf("expected 4-digit code from Ketua RT, got %v", resp["code"])
		}

		newPass := "KetuaPass123!"
		stReset, _ := postReset(userAEmail, ketuaCode, newPass, newPass)
		if stReset != http.StatusOK {
			t.Fatalf("expected 200 OK on reset with code generated by Ketua RT, got %d", stReset)
		}

		stLogin, respLogin := postLogin(userAEmail, newPass, "device-ketua-verify")
		if stLogin != http.StatusOK {
			t.Fatalf("expected login success with new password set via Ketua code, got %d", stLogin)
		}
		if respLogin["access_token"] == nil {
			t.Fatalf("expected access_token returned")
		}
	})
}
