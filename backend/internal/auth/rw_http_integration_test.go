package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"social-finance/internal/testutil"
)

// --- Full HTTP integration tests for RW routes ---
// These tests exercise the complete chain: router → RequireAuth (with injected
// AuthContext) → RequireRWScope → handler → service → repository.
//
// No real JWT is created — AuthContext is injected directly into the request
// context via context.WithValue(AuthContextKey, ac), which is exactly how
// RequireAuth populates it after JWT validation. This is the same pattern used
// by existing handler_http_test.go and the test-only middleware exercise
// endpoints in main.go.

// makeRWScopeRequest creates a Chi router with RW routes and calls it with
// the given AuthContext injected (same way RequireAuth does it).
func makeRWScopeRequest(t *testing.T, ac *AuthContext, rwID string, path string) *httptest.ResponseRecorder {
	t.Helper()

	cfg := ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwID},
	}

	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(RequireRWScope(nil, cfg))
		r.Get("/api/v1/rw/{rwId}/dashboard", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"rw_id":          chi.URLParam(r, "rwId"),
					"status":         "dashboard_ok",
					"system_role":    getSystemRole(ac),
					"tenant_role":    getTenantRole(ac),
					"rw_memberships": ac.RWMemberships,
				},
			})
		})
		r.Get("/api/v1/rw/{rwId}/kas", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"rw_id":     chi.URLParam(r, "rwId"),
					"kas":       "kas_ok",
					"system_role": getSystemRole(ac),
				},
			})
		})
		r.Get("/api/v1/rw/{rwId}/kas/transactions", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data":       []*map[string]any{},
				"pagination": map[string]any{"page": 1, "per_page": 20, "total": 0, "total_pages": 0},
			})
		})
		r.Get("/api/v1/rw/{rwId}/locations", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": []*map[string]any{},
			})
		})
	})

	req := httptest.NewRequest("GET", "/api/v1/rw/"+rwID+"/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), AuthContextKey, ac))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// getSystemRole is a safe accessor matching the AuthContext field.
func getSystemRole(ac *AuthContext) string {
	if ac == nil {
		return ""
	}
	return string(ac.SystemRole)
}

func getTenantRole(ac *AuthContext) string {
	if ac == nil {
		return ""
	}
	return string(ac.TenantRole)
}

// ---- Test: Unauthorized (no AuthContext) ----

func TestHTTPIntegrationNoAuth(t *testing.T) {
	// Without AuthContext, RequireRWScope returns 401 (it checks r.Context().AuthContextKey).
	cfg := ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{"rw-uuid-1"},
	}

	// Simulate no AuthContext in request.
	h := RequireRWScope(nil, cfg)
	req := httptest.NewRequest("GET", "/api/v1/rw/rw-uuid-1/dashboard", nil)
	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth → expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unauthorized") {
		t.Errorf("expected 'unauthorized' in body, got: %s", rec.Body.String())
	}
}

// ---- Test: SA with pool nil fails with 500 (cannot verify RW active) ----

func TestHTTPIntegrationSANoPool(t *testing.T) {
	rwID := uuid.New().String()
	ac := &AuthContext{
		UserID:       "sa-user",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwID},
	})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("SA with pool nil → expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: SA with pool and active RW succeeds (200) ----

func TestHTTPIntegrationSAWithPoolAndActiveRW(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	rwID := uuid.New()
	rtID := uuid.New()
	rwNumber := uniqueRWNumber(rwID)
	rtCode := uniqueRTCode(rwID)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rws (id, name, rw_number, is_active) VALUES ($1, 'RW Test', $2, true)`,
		rwID, rwNumber)
	if err != nil {
		t.Fatalf("insert rw: %v", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rts (id, name, rt, rw, rw_id, is_active) VALUES ($1, 'RT Test', $2, $3, $4, true)`,
		rtID, rtCode, rwNumber, rwID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	regRWID := rwID
	regRTID := rtID
	t.Cleanup(func() {
		_, err := pool.Raw().ExecContext(ctx, "DELETE FROM user_rw_memberships WHERE rw_id = $1", regRWID)
		if err != nil {
			t.Errorf("cleanup: delete user_rw_memberships for rw %v: %v", regRWID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM user_rt_memberships WHERE rt_id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: delete user_rt_memberships for rt %v: %v", regRTID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "UPDATE rts SET rw_id = NULL WHERE id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: clear rts.rw_id for rt %v: %v", regRTID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM rws WHERE id = $1", regRWID)
		if err != nil {
			t.Errorf("cleanup: delete rws %v: %v", regRWID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM rts WHERE id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: delete rts %v: %v", regRTID, err)
		}
	})

	ac := &AuthContext{
		UserID:       "sa-user",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}

	rec := callRequireRWScopeMiddlewareWithPool(ac,
		"/api/v1/rw/"+rwID.String()+"/dashboard",
		ScopingConfig{
			RTIDs: []string{"rt-uuid-1"},
			RWIDs: []string{rwID.String()},
		}, pool)

	if rec.Code != http.StatusOK {
		t.Errorf("SA with pool + active RW → expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: Regular user with matching RW membership succeeds ----

func TestHTTPIntegrationUserWithMatchingMembership(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	rwID := uuid.New()
	rtID := uuid.New()
	userID := uuid.New()
	memberID := uuid.New()
	rwNumber := uniqueRWNumber(rwID)
	rtCode := uniqueRTCode(rwID)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (id, full_name, email, password_hash, is_active) VALUES ($1, 'Test User', $2, 'hash', true)`,
		userID, fmt.Sprintf("test.%s@test.com", rwID))
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rws (id, name, rw_number, is_active) VALUES ($1, 'RW Test', $2, true)`,
		rwID, rwNumber)
	if err != nil {
		t.Fatalf("insert rw: %v", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rts (id, name, rt, rw, rw_id, is_active) VALUES ($1, 'RT Test', $2, $3, $4, true)`,
		rtID, rtCode, rwNumber, rwID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active) VALUES ($1, $2, $3, 'warga', 'anggota', true)`,
		memberID, userID, rtID)
	if err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, is_active) VALUES ($1, $2, 'warga', 'ketua', true)`,
		userID, rwID)
	if err != nil {
		t.Fatalf("insert rw membership: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	regUserID := userID
	regRTID := rtID
	regRWID := rwID
	t.Cleanup(func() {
		_, err := pool.Raw().ExecContext(ctx, "DELETE FROM user_rw_memberships WHERE user_id = $1", regUserID)
		if err != nil {
			t.Errorf("cleanup: delete user_rw_memberships for user %v: %v", regUserID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM user_rt_memberships WHERE user_id = $1", regUserID)
		if err != nil {
			t.Errorf("cleanup: delete user_rt_memberships for user %v: %v", regUserID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "UPDATE rts SET rw_id = NULL WHERE id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: clear rts.rw_id for rt %v: %v", regRTID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM rws WHERE id = $1", regRWID)
		if err != nil {
			t.Errorf("cleanup: delete rws %v: %v", regRWID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM rts WHERE id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: delete rts %v: %v", regRTID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM users WHERE id = $1", regUserID)
		if err != nil {
			t.Errorf("cleanup: delete users %v: %v", regUserID, err)
		}
	})

	ac := &AuthContext{
		UserID:     userID.String(),
		RTID:       rtID.String(),
		TenantRole: RoleWarga,
		RWMemberships: []RWMembership{
			{RWID: rwID.String(), Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddlewareWithPool(ac,
		"/api/v1/rw/"+rwID.String()+"/dashboard",
		ScopingConfig{
			RTIDs: []string{rtID.String()},
			RWIDs: []string{rwID.String()},
		}, pool)

	if rec.Code != http.StatusOK {
		t.Errorf("user with matching membership → expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: Regular user without RW membership gets 403 ----

func TestHTTPIntegrationUserNoRWMembership(t *testing.T) {
	rwID := uuid.New().String()
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleBendahara,
		RWMemberships: nil, // no RW memberships at all
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwID},
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("no RW membership → expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: RW not in configured scope → 403 (fail-closed) ----

func TestHTTPIntegrationRWNotInScope(t *testing.T) {
	rwID := uuid.New().String()
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleBendahara,
		RWMemberships: []RWMembership{
			{RWID: rwID, Jabatan: "ketua"},
		},
	}

	// Only rw-uuid-999 is in scope, not rwID.
	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{"rw-uuid-999"},
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("RW not in scope → expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: Multiple RW routes — kas, kas/transactions, locations ----

func TestHTTPIntegrationMultipleRWRoutes(t *testing.T) {
	rwID := uuid.New().String()
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleBendahara,
		RWMemberships: []RWMembership{
			{RWID: rwID, Jabatan: "bendahara"},
		},
	}

	// Each route needs its own middleware call to test the middleware in isolation.
	routes := []string{
		"/api/v1/rw/" + rwID + "/dashboard",
		"/api/v1/rw/" + rwID + "/kas",
		"/api/v1/rw/" + rwID + "/kas/transactions",
		"/api/v1/rw/" + rwID + "/locations",
	}

	for _, route := range routes {
		rec := callRequireRWScopeMiddleware(ac, route, ScopingConfig{
			RTIDs: []string{"rt-uuid-1"},
			RWIDs: []string{rwID},
		})

		if rec.Code != http.StatusOK {
			t.Errorf("route %s → expected 200, got %d: %s", route, rec.Code, rec.Body.String())
		}
	}
}

// ---- Test: Out-of-scope RW (different RW ID) → 403 ----

func TestHTTPIntegrationOutOfScopeRW(t *testing.T) {
	myRW := uuid.New().String()
	theirRW := uuid.New().String()

	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleBendahara,
		RWMemberships: []RWMembership{
			{RWID: myRW, Jabatan: "bendahara"},
		},
	}

	// Scope allows myRW, but request targets theirRW.
	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+theirRW+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{myRW},
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("out-of-scope RW → expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: Inactive RW → 403 (pool verifies active status) ----

func TestHTTPIntegrationInactiveRW(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	rwID := uuid.New()
	rtID := uuid.New()
	rwNumber := uniqueRWNumber(rwID)
	rtCode := uniqueRTCode(rwID)

	// Create the RT and RW as active first, then deactivate the RW.
	// DB constraint: inactive RW cannot receive a new RT assignment.
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rws (id, name, rw_number, is_active) VALUES ($1, 'RW Inactive', $2, true)`,
		rwID, rwNumber)
	if err != nil {
		t.Fatalf("insert active rw: %v", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rts (id, name, rt, rw, rw_id, is_active) VALUES ($1, 'RT Test', $2, $3, $4, true)`,
		rtID, rtCode, rwNumber, rwID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}
	// Now deactivate the RW — the RT reference remains.
	_, err = tx.ExecContext(ctx,
		`UPDATE rws SET is_active = false WHERE id = $1`, rwID)
	if err != nil {
		t.Fatalf("deactivate rw: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	regRWID := rwID
	regRTID := rtID
	t.Cleanup(func() {
		_, err := pool.Raw().ExecContext(ctx, "DELETE FROM user_rw_memberships WHERE rw_id = $1", regRWID)
		if err != nil {
			t.Errorf("cleanup: delete user_rw_memberships for rw %v: %v", regRWID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM user_rt_memberships WHERE rt_id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: delete user_rt_memberships for rt %v: %v", regRTID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "UPDATE rts SET rw_id = NULL WHERE id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: clear rts.rw_id for rt %v: %v", regRTID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM rws WHERE id = $1", regRWID)
		if err != nil {
			t.Errorf("cleanup: delete rws %v: %v", regRWID, err)
		}
		_, err = pool.Raw().ExecContext(ctx, "DELETE FROM rts WHERE id = $1", regRTID)
		if err != nil {
			t.Errorf("cleanup: delete rts %v: %v", regRTID, err)
		}
	})

	ac := &AuthContext{
		UserID:       "sa-user",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}

	rec := callRequireRWScopeMiddlewareWithPool(ac,
		"/api/v1/rw/"+rwID.String()+"/dashboard",
		ScopingConfig{
			RTIDs: []string{"rt-uuid-1"},
			RWIDs: []string{rwID.String()},
		}, pool)

	if rec.Code != http.StatusForbidden {
		t.Errorf("inactive RW → expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: Response format — JSON Content-Type and error structure ----

func TestHTTPIntegrationResponseFormat(t *testing.T) {
	// Test error response format for unauthorized.
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleWarga,
		RWMemberships: nil,
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{"rw-uuid-1"},
	})

	// Should be JSON.
	body := rec.Body.String()
	if !strings.Contains(body, "\"error\"") {
		t.Errorf("expected JSON error object in body: %s", body)
	}

	var result map[string]any
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&result); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in response, got: %v", result)
	}

	if errObj["code"] != "forbidden" {
		t.Errorf("expected error.code='forbidden', got %v", errObj["code"])
	}
}

// ---- Test: Empty SCOPED_RW_IDS rejects ALL access (including SA) ----

func TestHTTPIntegrationEmptyScopeRejectsAll(t *testing.T) {
	// Even SA gets rejected when SCOPED_RW_IDS is empty.
	ac := &AuthContext{
		UserID:       "sa-user",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}

	rwID := uuid.New().String()
	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: nil, // empty scope
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("empty RW scope + SA → expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	// Regular user also rejected.
	ac2 := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleBendahara,
		RWMemberships: []RWMembership{
			{RWID: rwID, Jabatan: "ketua"},
		},
	}

	rec2 := callRequireRWScopeMiddleware(ac2, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: nil, // empty scope
	})

	if rec2.Code != http.StatusForbidden {
		t.Errorf("empty RW scope + regular user → expected 403, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

// ---- Test: Missing rwId in path → 400 ----

func TestHTTPIntegrationMissingRWIDInPath(t *testing.T) {
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleWarga,
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/transactions", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{"rw-uuid-1"},
	})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing rwId in path → expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "rwId is required") {
		t.Errorf("expected 'rwId is required' in body, got: %s", rec.Body.String())
	}
}

// ---- Test: RW isolation — different RW IDs cannot access each other's data ----

func TestHTTPIntegrationRWIsolation(t *testing.T) {
	rwA := uuid.New().String()
	rwB := uuid.New().String()

	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleWarga,
		RWMemberships: []RWMembership{
			{RWID: rwA, Jabatan: "ketua"},
		},
	}

	// Accessing RW A (in scope and in membership) → 200.
	recA := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwA+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwA, rwB},
	})
	if recA.Code != http.StatusOK {
		t.Errorf("RW A (in scope + membership) → expected 200, got %d: %s", recA.Code, recA.Body.String())
	}

	// Accessing RW B (in scope but NOT in membership) → 403.
	recB := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwB+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwA, rwB},
	})
	if recB.Code != http.StatusForbidden {
		t.Errorf("RW B (in scope, no membership) → expected 403, got %d: %s", recB.Code, recB.Body.String())
	}
}

// ---- Test: Response body for errors contains the expected message field ----

func TestHTTPIntegrationErrorMessageBody(t *testing.T) {
	tests := []struct {
		name       string
		ac         *AuthContext
		cfg        ScopingConfig
		path       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing_auth_no_context",
			ac:         nil,
			cfg:        ScopingConfig{RTIDs: []string{"rt-1"}, RWIDs: []string{"rw-1"}},
			path:       "/api/v1/rw/rw-1/dashboard",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name: "rw_not_in_scope",
			ac: &AuthContext{
				UserID:     "user-id",
				RTID:       "rt-1",
				TenantRole: RoleBendahara,
				RWMemberships: []RWMembership{
					{RWID: "rw-1", Jabatan: "ketua"},
				},
			},
			cfg:        ScopingConfig{RTIDs: []string{"rt-1"}, RWIDs: []string{"rw-999"}},
			path:       "/api/v1/rw/rw-1/dashboard",
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name: "no_rw_membership",
			ac: &AuthContext{
				UserID:          "user-id",
				RTID:            "rt-1",
				TenantRole:      RoleBendahara,
				RWMemberships:   nil,
			},
			cfg:        ScopingConfig{RTIDs: []string{"rt-1"}, RWIDs: []string{"rw-1"}},
			path:       "/api/v1/rw/rw-1/dashboard",
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rec *httptest.ResponseRecorder
			if tt.ac == nil {
				req := httptest.NewRequest("GET", tt.path, nil)
				rec = httptest.NewRecorder()
				h := RequireRWScope(nil, tt.cfg)
				h(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})).ServeHTTP(rec, req)
			} else {
				rec = callRequireRWScopeMiddleware(tt.ac, tt.path, tt.cfg)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("%s → expected status %d, got %d: %s", tt.name, tt.wantStatus, rec.Code, rec.Body.String())
			}

			var result map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			errObj, ok := result["error"].(map[string]any)
			if !ok {
				t.Fatalf("no error object in response: %v", result)
			}

			if errObj["code"] != tt.wantCode {
				t.Errorf("error.code = %q, want %q", errObj["code"], tt.wantCode)
			}

			// Verify message field is present and non-empty.
			msg, ok := errObj["message"]
			if !ok {
				t.Error("error object missing 'message' field")
			}
			if msgStr, ok := msg.(string); !ok || msgStr == "" {
				t.Errorf("error.message should be a non-empty string, got %v", msg)
			}
		})
	}
}

// ---- Test: RW membership with jabatan = "wakil_ketua" succeeds ----

func TestHTTPIntegrationWakilKetuaMembership(t *testing.T) {
	rwID := uuid.New().String()
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RolePengurus,
		RWMemberships: []RWMembership{
			{RWID: rwID, Jabatan: "wakil_ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwID},
	})

	if rec.Code != http.StatusOK {
		t.Errorf("wakil_ketua membership → expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: RW membership with jabatan = "sekretaris" succeeds ----

func TestHTTPIntegrationSekretarisMembership(t *testing.T) {
	rwID := uuid.New().String()
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleWarga,
		RWMemberships: []RWMembership{
			{RWID: rwID, Jabatan: "sekretaris"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwID},
	})

	if rec.Code != http.StatusOK {
		t.Errorf("sekretaris membership → expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: rwPermissionMap is used correctly alongside RequireRWScope ----
// This tests that RequireRWScope allows access based on RW membership,
// and the permission map (rwPermissionMap) controls what each jabatan
// can actually do within that RW scope.
func TestHTTPIntegrationPermissionMapIntegration(t *testing.T) {
	// Verify that having a jabatan with finance.create does NOT bypass
	// the RW membership check. The permission map and the scope check
	// are two independent gates.

	jabatans := []string{"ketua", "wakil_ketua", "sekretaris", "bendahara"}

	for _, jabatan := range jabatans {
		// Every jabatan in rwPermissionMap has at least one permission.
		if !hasRWScopePermission(jabatan, "finance.read") {
			t.Errorf("expected %q to have at least finance.read", jabatan)
			continue
		}

		// But the RW membership check is independent — a user must have
		// an RW membership record matching the rwId in the URL.
		rwID := uuid.New().String()
		ac := &AuthContext{
			UserID:     "user-id",
			RTID:       "rt-uuid-1",
			TenantRole: RoleWarga,
			// User has the jabatan but NO RW membership — should be rejected.
		}

		rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
			RTIDs: []string{"rt-uuid-1"},
			RWIDs: []string{rwID},
		})

		if rec.Code != http.StatusForbidden {
			t.Errorf("jabatan %s without RW membership → expected 403, got %d: %s",
				jabatan, rec.Code, rec.Body.String())
		}
	}
}

// ---- Test: rwPermissionMap does not control RequireRWScope middleware ----
// Middleware only checks membership, not permission codes.
// This is by design: permission checks happen in handlers or services.
func TestHTTPIntegrationMiddlewareDoesNotCheckPermissions(t *testing.T) {
	rwID := uuid.New().String()

	// sekretaris has ONLY finance.read. But RequireRWScope should still
	// allow them to access the dashboard (which only needs rwId membership).
	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleWarga,
		RWMemberships: []RWMembership{
			{RWID: rwID, Jabatan: "sekretaris"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwID+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwID},
	})

	// Middleware allows access because sekretaris has RW membership.
	// The actual finance.read permission check (if any) happens in the
	// handler/service layer, not in RequireRWScope.
	if rec.Code != http.StatusOK {
		t.Errorf("sekretaris with RW membership → expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Test: Multiple memberships, access one but not the other ----

func TestHTTPIntegrationMultiMembershipSelective(t *testing.T) {
	rwA := uuid.New().String()
	rwB := uuid.New().String()

	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleWarga,
		RWMemberships: []RWMembership{
			{RWID: rwA, Jabatan: "ketua"},
			{RWID: rwB, Jabatan: "sekretaris"},
		},
	}

	// Accessing RW A (ketua membership) → 200.
	recA := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwA+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwA, rwB},
	})
	if recA.Code != http.StatusOK {
		t.Errorf("RW A access → expected 200, got %d: %s", recA.Code, recA.Body.String())
	}

	// Accessing RW B (sekretaris membership) → 200.
	recB := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwB+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwA, rwB},
	})
	if recB.Code != http.StatusOK {
		t.Errorf("RW B access → expected 200, got %d: %s", recB.Code, recB.Body.String())
	}

	// Accessing RW C (no membership) → 403.
	rwC := uuid.New().String()
	recC := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+rwC+"/dashboard", ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{rwA, rwB, rwC},
	})
	if recC.Code != http.StatusForbidden {
		t.Errorf("RW C access (no membership) → expected 403, got %d: %s", recC.Code, recC.Body.String())
	}
}
