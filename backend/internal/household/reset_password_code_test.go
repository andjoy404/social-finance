package household_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/household"
	"social-finance/internal/testutil"
)

func TestGenerateResetPasswordCode(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Setup RT
	var rtID string
	err := pool.Raw().QueryRow(`SELECT id FROM rts LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("query rt: %v", err)
	}

	// Setup Pengurus user
	pengurusID := "12121212-1212-1212-1212-121212121212"
	pengurusEmail := "pengurus.reset@example.com"
	passHash, _ := auth.Hash("Password123")
	_, _ = pool.Raw().Exec(`
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, $3, 'Pak Pengurus', true)
		ON CONFLICT (id) DO UPDATE SET password_hash = $3
	`, pengurusID, pengurusEmail, passHash)
	_, _ = pool.Raw().Exec(`
		INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		VALUES ($1, $2, 'pengurus', true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
	`, pengurusID, rtID)

	// Setup Warga user
	wargaUserID := "34343434-3434-3434-3434-343434343434"
	wargaEmail := "warga.target@example.com"
	_, _ = pool.Raw().Exec(`
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, $3, 'Warga Target', true)
		ON CONFLICT (id) DO UPDATE SET password_hash = $3
	`, wargaUserID, wargaEmail, passHash)
	_, _ = pool.Raw().Exec(`
		INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		VALUES ($1, $2, 'warga', true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
	`, wargaUserID, rtID)

	// Setup resident record
	resID := "56565656-5656-5656-5656-565656565656"
	_, err = pool.Raw().Exec(`
		INSERT INTO residents (id, rt_id, user_id, full_name, email, is_active)
		VALUES ($1, $2, $3, 'Warga Target', $4, true)
		ON CONFLICT (id) DO UPDATE SET user_id = $3
	`, resID, rtID, wargaUserID, wargaEmail)
	if err != nil {
		t.Fatalf("setup resident: %v", err)
	}

	h := household.NewHandler(pool)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Injected by test
			next.ServeHTTP(w, req)
		})
	})
	r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)

	// 1. Pengurus generates reset code
	req := httptest.NewRequest("POST", "/api/v1/residents/"+resID+"/reset-password-code", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID:     pengurusID,
		RTID:       rtID,
		TenantRole: auth.RolePengurus,
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resData map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resData); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	code1, ok := resData["code"].(string)
	if !ok || len(code1) != 4 {
		t.Fatalf("expected 4-digit string code, got: %v", resData["code"])
	}

	// Verify code is hashed in database
	var codeHash string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT code_hash FROM resident_password_reset_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, wargaUserID).Scan(&codeHash)
	if err != nil {
		t.Fatalf("lookup code hash: %v", err)
	}
	if err := auth.VerifyCode(code1, codeHash); err != nil {
		t.Fatalf("expected code hash to match: %v", err)
	}

	// 2. Generate again: invalidates previous code
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	var resData2 map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &resData2)
	code2 := resData2["code"].(string)

	// Verify old code was marked used/invalidated
	var activeCount int
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM resident_password_reset_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, wargaUserID).Scan(&activeCount)
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active code, got %d", activeCount)
	}

	// 3. Warga cannot generate reset code (Forbidden)
	wargaReq := httptest.NewRequest("POST", "/api/v1/residents/"+resID+"/reset-password-code", nil)
	wargaReq = wargaReq.WithContext(context.WithValue(wargaReq.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID:     wargaUserID,
		RTID:       rtID,
		TenantRole: auth.RoleWarga,
	}))
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, wargaReq)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for warga role, got %d", w3.Code)
	}

	_ = code2
}
