package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/testutil"
)

func TestSuperAdmin_RT002FullAccessAndContext(t *testing.T) {
	pool := testutil.GetTestPool(t)
	auth.AuthDBPool = pool

	ctx := context.Background()
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Ensure RT 002 exists in database
	rt002ID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
	_, err = tx.ExecContext(ctx, `
		INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
		VALUES ($1, 'Urena', 16, '002', 'Urena Street', 'Ketua RT 002', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, rt002ID)
	if err != nil {
		t.Fatalf("upsert RT 002: %v", err)
	}

	// Create a super_admin user with NO RT membership (system-only global admin)
	saUserID := "b69669cb-7145-4991-8192-8b017b12f79e"
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES ($1, 'admin-test@local.com', 'dummyhash', 'Super Admin Urena', 'super_admin', true)
		ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true
	`, saUserID)
	if err != nil {
		t.Fatalf("upsert super_admin user: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit setup: %v", err)
	}

	// Generate super_admin system token (SysRole: super_admin, no RTID)
	tokenStr, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  saUserID,
		SysRole: string(auth.SystemRoleSuperAdmin),
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	// 1. Verify RequireAuth resolves effective RTID to RT 002 for super_admin when passing X-RT-ID
	var capturedAC *auth.AuthContext
	testHandler := auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAC = auth.GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("X-RT-ID", rt002ID)
	rec := httptest.NewRecorder()
	testHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedAC == nil {
		t.Fatal("expected AuthContext to be populated")
	}
	if capturedAC.SystemRole != auth.SystemRoleSuperAdmin {
		t.Errorf("expected SystemRole super_admin, got %s", capturedAC.SystemRole)
	}
	if capturedAC.RTID != rt002ID {
		t.Errorf("expected RTID to resolve to RT 002 (%s), got %s", rt002ID, capturedAC.RTID)
	}

	// 1b. Verify query parameter rt_id also resolves for super_admin
	capturedAC = nil
	reqQuery := httptest.NewRequest("GET", "/api/v1/test?rt_id="+rt002ID, nil)
	reqQuery.Header.Set("Authorization", "Bearer "+tokenStr)
	recQuery := httptest.NewRecorder()
	testHandler.ServeHTTP(recQuery, reqQuery)

	if recQuery.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recQuery.Code)
	}
	if capturedAC == nil || capturedAC.RTID != rt002ID {
		t.Errorf("expected query param rt_id to resolve to RT 002, got %+v", capturedAC)
	}

	// 2. Verify /api/v1/auth/me populates RT 002 details for super_admin
	svc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: 7 * 24 * time.Hour,
	})
	handler := auth.NewHandler(pool, svc)

	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+tokenStr)
	meRec := httptest.NewRecorder()

	meHandler := auth.RequireAuth(handler.Me())
	meHandler.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d: %s", meRec.Code, meRec.Body.String())
	}

	var meResp auth.UserForMe
	if err := json.NewDecoder(meRec.Body).Decode(&meResp); err != nil {
		t.Fatalf("failed to decode /me response: %v", err)
	}

	if meResp.SystemRole != "super_admin" {
		t.Errorf("expected system_role super_admin, got %s", meResp.SystemRole)
	}
	if meResp.RTID != rt002ID {
		t.Errorf("expected /me rt_id to be RT 002 (%s), got %s", rt002ID, meResp.RTID)
	}
	if meResp.RT_rt != "002" {
		t.Errorf("expected /me rt_rt to be '002', got %s", meResp.RT_rt)
	}
	if meResp.RTName != "Urena" {
		t.Errorf("expected /me rt_name to be 'Urena', got %s", meResp.RTName)
	}
}
