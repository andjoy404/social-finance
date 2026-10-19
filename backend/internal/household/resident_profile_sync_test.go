package household

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/devseed"
)

// TestResidentUpdateSyncsToUserProfile verifies that when a Pengurus updates a
// resident's phone number, the linked user account (users.phone) is also updated
// so that the profile endpoint (/auth/me) reflects the new phone without
// requiring logout/login.
//
// Root cause: ResidentUpdate only wrote to the `residents` table, while
// /auth/me (GetProfile) reads from `users`. The two drifted out of sync.
func TestResidentUpdateSyncsToUserProfile(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if _, err := devseed.Run(ctx, pool, "test"); err != nil {
		t.Fatalf("devseed run failed: %v", err)
	}

	auth.SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute

	const (
		rt03ID      = "8dfa1e36-6727-45be-8fea-24d958efc155" // RT 003
		wargaUserID = "22220001-0000-4000-8000-000000000001" // warga.rt03@local.com
		wargaEmail  = "warga.rt03@local.com"
		wargaPass   = devseed.DefaultSeedPassword
		oldPhone    = "+6281300000030"
		newPhone    = "+6281399999999"
	)

	residentID := toUUID("test-resident-profile-sync")

	// 1. Create a resident linked to the warga user account (user_id set).
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO residents (id, rt_id, user_id, full_name, phone, nik, email, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			full_name = EXCLUDED.full_name,
			phone = EXCLUDED.phone,
			nik = EXCLUDED.nik,
			email = EXCLUDED.email,
			is_active = true
	`, residentID, rt03ID, wargaUserID, "Profile Sync Warga", oldPhone, "3173999999999999", "profilesync@test.com")
	if err != nil {
		t.Fatalf("create test resident: %v", err)
	}

	// 2. Create a pengurus user + membership in RT 003 to perform the update.
	pengurusUserID := toUUID("test-pengurus-profile-sync")
	pengurusMemID := toUUID("test-pengurus-mem-profile-sync")
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		VALUES ($1, $2, '08123456789', 'dummyhash', 'Test Pengurus', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, pengurusUserID, "pengurus.sync@test.com")
	if err != nil {
		t.Fatalf("insert pengurus user: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active)
		VALUES ($1, $2, $3, 'pengurus', true)
		ON CONFLICT (id) DO UPDATE SET role = 'pengurus', is_active = true, rt_id = EXCLUDED.rt_id
	`, pengurusMemID, pengurusUserID, rt03ID)
	if err != nil {
		t.Fatalf("insert pengurus membership: %v", err)
	}

	// 3. Pengurus token (RequireAuth + RequireRole(pengurus) validate via JWT + DB).
	pengurusToken := makeTenantToken(t, pengurusUserID, pengurusMemID, rt03ID, auth.RolePengurus)

	// --- Auth handler for login + /me (wrapped with RequireAuth like the real router) ---
	svc := auth.NewService(auth.ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	authHandler := auth.NewHandler(pool, svc)
	meHandler := auth.RequireAuth(authHandler.Me())

	// --- Helper: login as warga and return /me phone ---
	loginAndMe := func() (*string, error) {
		loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			bytes.NewBufferString(`{"email":"`+wargaEmail+`","password":"`+wargaPass+`"}`))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()
		authHandler.Login().ServeHTTP(loginRec, loginReq)
		if loginRec.Code != http.StatusOK {
			return nil, &errStr{msg: "warga login failed: " + loginRec.Body.String()}
		}
		var loginResp struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
			return nil, err
		}
		// Call /auth/me to fetch the canonical profile.
		meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		meReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
		meRec := httptest.NewRecorder()
		meHandler.ServeHTTP(meRec, meReq)
		if meRec.Code != http.StatusOK {
			return nil, &errStr{msg: "/auth/me failed: " + meRec.Body.String()}
		}
		var meResp struct {
			Phone *string `json:"phone"`
		}
		if err := json.Unmarshal(meRec.Body.Bytes(), &meResp); err != nil {
			return nil, err
		}
		return meResp.Phone, nil
	}

	// 4. Profile BEFORE update shows OLD phone.
	phoneBefore, err := loginAndMe()
	if err != nil {
		t.Fatalf("pre-update profile: %v", err)
	}
	if phoneBefore == nil || *phoneBefore != oldPhone {
		t.Fatalf("expected profile phone %q before update, got %v", oldPhone, phoneBefore)
	}

	// 5. Pengurus updates the resident's phone via existing PATCH endpoint.
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireRole(auth.RolePengurus))
	r.Patch("/api/v1/residents/{id}", NewHandler(pool).UpdateResident)

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+residentID,
		bytes.NewBufferString(`{"phone":"`+newPhone+`"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+pengurusToken)
	updateRec := httptest.NewRecorder()
	r.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("resident update failed: %d %s", updateRec.Code, updateRec.Body.String())
	}

	// 6. Verify the resident row itself was updated.
	var residentPhone string
	if err := pool.Raw().QueryRowContext(ctx, `SELECT phone FROM residents WHERE id = $1`, residentID).Scan(&residentPhone); err != nil {
		t.Fatalf("query updated resident phone: %v", err)
	}
	if residentPhone != newPhone {
		t.Fatalf("expected residents.phone %q, got %q", newPhone, residentPhone)
	}

	// 7. Profile AFTER update (no logout/login) shows NEW phone via /auth/me.
	phoneAfter, err := loginAndMe()
	if err != nil {
		t.Fatalf("post-update profile: %v", err)
	}
	if phoneAfter == nil || *phoneAfter != newPhone {
		t.Fatalf("expected profile phone %q after update, got %v", newPhone, phoneAfter)
	}

	// 8. Other user fields (full_name, email) remain correct / unchanged.
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+func() string {
		loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			bytes.NewBufferString(`{"email":"`+wargaEmail+`","password":"`+wargaPass+`"}`))
		loginReq.Header.Set("Content-Type", "application/json")
		loginRec := httptest.NewRecorder()
		authHandler.Login().ServeHTTP(loginRec, loginReq)
		var resp struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.Unmarshal(loginRec.Body.Bytes(), &resp)
		return resp.AccessToken
	}())
	meRec := httptest.NewRecorder()
	meHandler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("/auth/me (fields) failed: %d %s", meRec.Code, meRec.Body.String())
	}
	var meFull struct {
		Fullname string  `json:"full_name"`
		Phone    *string `json:"phone"`
		Email    string  `json:"email"`
		Role     string  `json:"role"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &meFull); err != nil {
		t.Fatalf("unmarshal /auth/me: %v", err)
	}
	if meFull.Fullname != "Warga RT 03" {
		t.Errorf("expected full_name unchanged 'Warga RT 03', got %q", meFull.Fullname)
	}
	if meFull.Email != wargaEmail {
		t.Errorf("expected email unchanged %q, got %q", wargaEmail, meFull.Email)
	}
	if meFull.Role != "warga" {
		t.Errorf("expected role unchanged 'warga', got %q", meFull.Role)
	}
}

// errStr is a small error wrapper used by the test helper.
type errStr struct{ msg string }

func (e *errStr) Error() string { return e.msg }