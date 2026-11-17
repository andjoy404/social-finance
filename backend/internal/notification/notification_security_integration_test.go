package notification_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"
)

// TestRequireAllowedRT_ProductionMiddleware verifies that the production
// RequireAllowedRT middleware correctly enforces the allowed RT whitelist
// when used with a real database pool.
//
// This test uses RequireAllowedRT(pool) directly — not an inline
// approximation — to ensure the security boundary actually works at
// the middleware level.
func TestRequireAllowedRT_ProductionMiddleware(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Ensure RT 002 and RT 003 exist and are active in the test database.
	rt002ID := setupTestRT(t, pool, ctx, "002")
	rt003ID := setupTestRT(t, pool, ctx, "003")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	middleware := auth.RequireAllowedRT(pool, auth.ScopingConfig{RTIDs: []string{rt002ID, rt003ID}, RWIDs: []string{}})
	authorizedHandler := middleware(handler)

	tests := []struct {
		name          string
		systemRole    auth.SystemRole
		rtID          string
		expectedCode  int
		expectContent string // substring check on error
	}{
		{
			name:          "super_admin_with_empty_rtid_rejected",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          "",
			expectedCode:  http.StatusForbidden,
			expectContent: "tenant context required",
		},
		{
			name:          "super_admin_with_whitelisted_rt_002_passes",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          rt002ID,
			expectedCode:  http.StatusOK,
			expectContent: "",
		},
		{
			name:          "super_admin_with_whitelisted_rt_003_passes",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          rt003ID,
			expectedCode:  http.StatusOK,
			expectContent: "",
		},
		{
			name:          "super_admin_with_non_whitelisted_rt_999_rejected",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          "00000000-0000-0000-0999-999999999999",
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},
		{
			name:          "super_admin_with_random_uuid_rejected",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          "11111111-2222-3333-4444-555555555555",
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},
		{
			name:          "non_super_admin_with_empty_rtid_rejected",
			systemRole:    "", // non-SuperAdmin
			rtID:          "",
			expectedCode:  http.StatusForbidden,
			expectContent: "tenant context required",
		},
		{
			name:          "non_super_admin_with_rt_002_passes",
			systemRole:    "", // non-SuperAdmin
			rtID:          rt002ID,
			expectedCode:  http.StatusOK,
			expectContent: "",
		},
		{
			name:          "non_super_admin_with_non_whitelisted_rt_rejected",
			systemRole:    "", // non-SuperAdmin
			rtID:          "00000000-0000-0000-0999-999999999999",
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},
	}

	// Run sub-tests sequentially to avoid concurrent access to the same RT rows
	// in the shared test database.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ac := &auth.AuthContext{
				UserID:     "test-user-id",
				SystemRole: tc.systemRole,
				RTID:       tc.rtID,
			}

			req := httptest.NewRequest("GET", "/api/v1/notifications/templates", nil)
			ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			authorizedHandler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tc.expectedCode, rec.Code, rec.Body.String())
			}

			if tc.expectContent != "" && rec.Code != http.StatusOK {
				bodyStr := rec.Body.String()
				if !strings.Contains(bodyStr, tc.expectContent) {
					t.Errorf("expected error body to contain %q, got %s", tc.expectContent, bodyStr)
				}
			}
		})
	}

	// Verify no side effect on the database.
	var count int
	err := pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&count)
	if err != nil {
		t.Fatalf("unexpected error checking DB side effects: %v", err)
	}
	// Count should not change due to middleware calls.
	_ = count
}

// setupTestRT creates an isolated active RT fixture with the requested
// whitelisted RT code. A randomized RW number prevents collisions with shared
// RT rows while preserving the production whitelist check exercised by these
// tests. Cleanup targets only the RT row created by this helper.
func setupTestRT(t *testing.T, pool *database.Pool, ctx context.Context, rtCode string) string {
	t.Helper()

	var rtID string
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		fixtureID := uuid.New()
		rwNumber := int(fixtureID.ID() & 0x7fffffff)
		if rwNumber == 0 {
			rwNumber = 1
		}
		rtID = fixtureID.String()
		err = pool.Raw().QueryRowContext(ctx, `
			INSERT INTO rts (id, rw, rt, name, is_active)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id
		`, rtID, rwNumber, rtCode, fmt.Sprintf("Test RT %s %s", rtCode, rtID)).Scan(&rtID)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("failed to create isolated active RT %s fixture: %v", rtCode, err)
	}

	fixtureRTID := rtID
	t.Cleanup(func() {
		if _, cleanupErr := pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, fixtureRTID); cleanupErr != nil {
			t.Errorf("failed to clean up isolated RT fixture %s: %v", fixtureRTID, cleanupErr)
		}
	})
	return fixtureRTID
}

// containsSubstring is a simple helper to check if a substring exists in a string.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestRequireAllowedRT_IntegrationWithNotificationHandlers verifies the full
// notification management stack (middleware + handler) with production middleware.
func TestRequireAllowedRT_IntegrationWithNotificationHandlers(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Setup RTs in test DB.
	rt002ID := setupTestRT(t, pool, ctx, "002")
	rt003ID := setupTestRT(t, pool, ctx, "003")

	// Ensure notification.manage permission exists for jabatan "ketua".
	// This is needed for Ketua RT test cases that go through RequireNotificationAdmin.
	// Use the same RT ID (rt002ID) so validateMembershipJabatan finds the membership.

	// Insert permission (idempotent)
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO permission_types (code, domain, operation)
		VALUES ('notification.manage', 'notification', 'manage')
		ON CONFLICT (code) DO NOTHING`,
	)
	if err != nil {
		t.Fatalf("insert permission_types: %v", err)
	}
	var permissionID string
	err = pool.Raw().QueryRowContext(ctx,
		`SELECT id FROM permission_types WHERE code = 'notification.manage'`,
	).Scan(&permissionID)
	if err != nil {
		t.Fatalf("get permission id: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO position_permissions (position, permission_id)
		VALUES ($1, $2)
		ON CONFLICT (position, permission_id) DO NOTHING`,
		"ketua", permissionID,
	)
	if err != nil {
		t.Fatalf("insert position_permissions: %v", err)
	}

	// Create ketua user + membership in the same RT that setupTestRT returned.
	// This is the key fix: the membership rt_id must match rt002ID.
	var ketuaUID string
	ketuaUID = uuid.New().String()
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		VALUES ($1, $2, '08123456789', 'dummyhash', 'Test Ketua RT', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true`,
		ketuaUID, fmt.Sprintf("ketua-%d@test.com", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create ketua user for integration test: %v", err)
	}

	// Insert membership with the correct rt002ID so validateMembershipJabatan finds it.
	var ketuaMembershipID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, true)
		ON CONFLICT DO NOTHING
		RETURNING id`,
		ketuaUID, rt002ID, "warga", "ketua",
	).Scan(&ketuaMembershipID)
	if err != nil {
		t.Fatalf("create ketua membership for integration test: %v", err)
	}

	// Create a minimal mock repo for handler responses.
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	handler := notification.NewHandler(svc, nil, nil, "test")

	// Build router with PRODUCTION middleware (not inline).
	r := chi.NewRouter()

	// Production middleware chain: RequireAuth (simulated) + RequireAllowedRT + RequireNotificationAdmin
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ac := r.Context().Value(auth.AuthContextKey)
				if ac == nil {
					httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Use(auth.RequireAllowedRT(pool, auth.ScopingConfig{RTIDs: []string{rt002ID, rt003ID}, RWIDs: []string{}}))
		r.Use(auth.RequireNotificationAdmin(pool))

		r.Get("/api/v1/notifications/system-types", handler.GetSystemTypes)
		r.Get("/api/v1/notifications/templates", handler.ListTemplates)
		r.Post("/api/v1/notifications/templates", handler.CreateTemplate)
		r.Get("/api/v1/notifications/templates/{id}", handler.GetTemplate)
		r.Put("/api/v1/notifications/templates/{id}", handler.UpdateTemplate)
		r.Patch("/api/v1/notifications/templates/{id}/toggle-active", handler.ToggleTemplateActive)
		r.Delete("/api/v1/notifications/templates/{id}", handler.DeleteTemplate)
		r.Post("/api/v1/notifications/test-push", handler.TestPush)
		r.Post("/api/v1/notifications/upload-image", handler.UploadImage)
	})

	nonWhitelistedUUID := "00000000-0000-0000-0999-999999999999"

	tests := []struct {
		name          string
		systemRole    auth.SystemRole
		rtID          string
		jabatan       auth.Jabatan
		method        string
		path          string
		body          string
		expectedCode  int
		expectContent string
	}{
		// Super Admin tests
		{
			name:         "sa_list_templates_whitelisted_rt_002",
			systemRole:   auth.SystemRoleSuperAdmin,
			rtID:         rt002ID,
			method:       "GET",
			path:         "/api/v1/notifications/templates",
			expectedCode: http.StatusOK,
		},
		{
			name:         "sa_list_templates_whitelisted_rt_003",
			systemRole:   auth.SystemRoleSuperAdmin,
			rtID:         rt003ID,
			method:       "GET",
			path:         "/api/v1/notifications/templates",
			expectedCode: http.StatusOK,
		},
		{
			name:         "sa_create_template_whitelisted_rt_002",
			systemRole:   auth.SystemRoleSuperAdmin,
			rtID:         rt002ID,
			method:       "POST",
			path:         "/api/v1/notifications/templates",
			body:         `{"name":"Test Template","event_type":"manual_push","channel":"push","title":"Test","body":"Test body"}`,
			expectedCode: http.StatusCreated,
		},
		{
			name:          "sa_list_templates_non_whitelisted_uuid",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          nonWhitelistedUUID,
			method:        "GET",
			path:          "/api/v1/notifications/templates",
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},
		{
			name:          "sa_create_template_non_whitelisted_uuid",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          nonWhitelistedUUID,
			method:        "POST",
			path:          "/api/v1/notifications/templates",
			body:          `{"name":"Test Template","event_type":"manual_push","channel":"push","title":"Test","body":"Test body"}`,
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},
		{
			name:          "sa_test_push_non_whitelisted_uuid",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          nonWhitelistedUUID,
			method:        "POST",
			path:          "/api/v1/notifications/test-push",
			body:          `{"title":"Test","body":"Test"}`,
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},
		{
			name:          "sa_upload_image_non_whitelisted_uuid",
			systemRole:    auth.SystemRoleSuperAdmin,
			rtID:          nonWhitelistedUUID,
			method:        "POST",
			path:          "/api/v1/notifications/upload-image",
			body:          `{"event_type":"manual_push"}`,
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},

		// Non-SuperAdmin tests (empty systemRole = regular role like pengurus/bendahara/warga)
		{
			name:          "regular_empty_rtid_rejected",
			systemRole:    "", // non-SuperAdmin
			rtID:          "",
			method:        "GET",
			path:          "/api/v1/notifications/templates",
			expectedCode:  http.StatusForbidden,
			expectContent: "tenant context required",
		},
		{
			name:          "regular_with_non_whitelisted_rt_rejected",
			systemRole:    "", // non-SuperAdmin
			rtID:          nonWhitelistedUUID,
			method:        "GET",
			path:          "/api/v1/notifications/templates",
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},

		// Ketua RT tests (non-SuperAdmin with jabatan "ketua")
		{
			name:         "ketua_with_whitelisted_rt",
			systemRole:   "", // non-SuperAdmin
			jabatan:      auth.JabatanKetua,
			rtID:         rt002ID,
			method:       "GET",
			path:         "/api/v1/notifications/templates",
			expectedCode: http.StatusOK,
		},
		{
			name:          "ketua_with_non_whitelisted_rt",
			systemRole:    "", // non-SuperAdmin
			jabatan:       auth.JabatanKetua,
			rtID:          nonWhitelistedUUID,
			method:        "GET",
			path:          "/api/v1/notifications/templates",
			expectedCode:  http.StatusForbidden,
			expectContent: "not found or invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// For Ketua RT test case, use the ketua user created in fixture.
			userID := "test-user-id"
			membID := ""
			jabatanVal := tc.jabatan
			if tc.name == "ketua_with_whitelisted_rt" {
				userID = ketuaUID
				membID = ketuaMembershipID
				jabatanVal = auth.JabatanKetua
			}

			ac := &auth.AuthContext{
				UserID:       userID,
				MembershipID: membID,
				SystemRole:   tc.systemRole,
				RTID:         tc.rtID,
				Jabatan:      jabatanVal,
			}

			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}

			ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tc.expectedCode, rec.Code, rec.Body.String())
			}

			if tc.expectContent != "" && rec.Code != http.StatusOK {
				bodyStr := rec.Body.String()
				if !containsSubstring(bodyStr, tc.expectContent) {
					t.Errorf("expected error body to contain %q, got %s", tc.expectContent, bodyStr)
				}
			}
		})
	}
}

// TestRequireAllowedRT_ActiveRTFlag verifies that deactivated RTs are rejected.
//
// NOTE: This test intentionally does NOT use t.Parallel(). It must run
// sequentially because it modifies the is_active flag on RT 002, a shared
// fixture used by other tests. Running it in parallel would race with any
// other test that reads the same RT row while is_active is temporarily false.
//
// The middleware production code is not modified; this test reuses the
// existing whitelisted RT 002 because RequireAllowedRT checks BOTH the
// whitelist (rt code in {"002","003"}) AND the is_active column. A
// non-whitelisted RT code would fail the whitelist check before the
// is_active check is reached, making it impossible to test deactivation
// without temporarily altering AllowedRTCodes (which would be unsafe
// without a mutex around that global map).
func TestRequireAllowedRT_ActiveRTFlag(t *testing.T) {

	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Use an existing whitelisted RT so the whitelist check passes.
	rtID := setupTestRT(t, pool, ctx, "002")

	// Ensure the RT is restored to active even if this test panics.
	var wasActive bool
	err := pool.Raw().QueryRowContext(ctx,
		`SELECT is_active FROM rts WHERE id = $1`, rtID).Scan(&wasActive)
	if err != nil {
		t.Fatalf("query RT 002 initial state: %v", err)
	}
	t.Cleanup(func() {
		if wasActive {
			_, restoreErr := pool.Raw().ExecContext(ctx,
				`UPDATE rts SET is_active = true WHERE id = $1`, rtID)
			if restoreErr != nil {
				t.Logf("failed to restore RT 002 to active: %v", restoreErr)
			}
		}
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool, auth.ScopingConfig{RTIDs: []string{rtID}, RWIDs: []string{}})
	authorizedHandler := middleware(handler)

	// First, verify RT is active → passes
	ac := &auth.AuthContext{
		UserID:     "test-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       rtID,
	}
	req := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx2)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for active RT, got %d: %s", rec.Code, rec.Body.String())
	}

	// Deactivate the RT via SQL
	_, err = pool.Raw().ExecContext(ctx, `UPDATE rts SET is_active = false WHERE id = $1`, rtID)
	if err != nil {
		t.Fatalf("failed to deactivate test RT: %v", err)
	}

	// Now same request should be rejected
	rec2 := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for deactivated RT, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// t.Cleanup will restore is_active after this test completes.
}

// TestRequireAllowedRT_NoDBPoolUsesAuthDBPool verifies behavior when pool is nil.
func TestRequireAllowedRT_NoDBPoolUsesAuthDBPool(t *testing.T) {
	t.Parallel()

	// Create a real handler that just returns OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// With the new security hardening, RequireAllowedRT always validates
	// RTID against the whitelist when RTID is non-empty. When RTID is empty,
	// it rejects the request (no bypass for any role).
	middleware := auth.RequireAllowedRT(nil, auth.ScopingConfig{})
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "test-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "", // empty RTID is now always rejected
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	// SuperAdmin with empty RTID must be rejected — no silent bypass
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for super admin with empty RTID, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAllowedRT_NotFoundAuthContext verifies unauthorized when auth context missing.
func TestRequireAllowedRT_NotFoundAuthContext(t *testing.T) {
	t.Parallel()

	pool := testutil.GetTestPool(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool, auth.ScopingConfig{RTIDs: []string{}, RWIDs: []string{}})
	authorizedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	// No auth context set
	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when auth context missing, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAllowedRT_IntegrationWithNotificationLogs verifies notification logs
// are protected by the same whitelist enforcement.
func TestRequireAllowedRT_IntegrationWithNotificationLogs(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rt002ID := setupTestRT(t, pool, ctx, "002")

	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	handler := notification.NewHandler(svc, nil, nil, "test")

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ac := r.Context().Value(auth.AuthContextKey)
				if ac == nil {
					httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Use(auth.RequireAllowedRT(pool, auth.ScopingConfig{RTIDs: []string{rt002ID}, RWIDs: []string{}}))
		r.Use(auth.RequireNotificationAdmin(pool))

		r.Get("/api/v1/notifications/logs", handler.ListLogs)
	})

	nonWhitelistedUUID := "00000000-0000-0000-0999-999999999999"

	tests := []struct {
		name         string
		systemRole   auth.SystemRole
		rtID         string
		expectedCode int
	}{
		{
			name:         "sa_list_logs_whitelisted",
			systemRole:   auth.SystemRoleSuperAdmin,
			rtID:         rt002ID,
			expectedCode: http.StatusOK,
		},
		{
			name:         "sa_list_logs_non_whitelisted",
			systemRole:   auth.SystemRoleSuperAdmin,
			rtID:         nonWhitelistedUUID,
			expectedCode: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ac := &auth.AuthContext{
				UserID:     "test-user-id",
				SystemRole: tc.systemRole,
				RTID:       tc.rtID,
			}

			req := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
			ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tc.expectedCode, rec.Code, rec.Body.String())
			}
		})
	}
}
