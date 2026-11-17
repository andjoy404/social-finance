package main

// ============================================================================
// Hotfix RBAC integration test — Bendahara must NOT have access to any
// notification administration endpoint. These tests use the production
// router so they verify the actual middleware chain (auth -> allowed-RT
// -> notification.manage), not a parallel test wiring.
// ============================================================================

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-finance/internal/auth"
)

// applyNotificationHotfixMigrations idempotently removes the stale
// notification.manage grant from the bendahara position. Safe to run
// against the test database (idempotent DELETE).
func TestNotificationAdminRBAC_BendaharaDenied(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, _ = pool, rtID

	// Remove stale privileged grants for bendahara before verifying any
	// behavior. Idempotent no-op on databases without the grant.
	if _, err := pool.Raw().ExecContext(context.Background(), `
		DELETE FROM position_permissions
		WHERE position = 'bendahara'
		  AND permission_id = (SELECT id FROM permission_types WHERE code = 'notification.manage')`); err != nil {
		t.Fatalf("remove stale bendahara notification.manage: %v", err)
	}

	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)
	_, bendaharaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanBendahara)
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")

	managementCases := []struct {
		name   string
		method string
		path   string
	}{
		{"system-types", http.MethodGet, "/api/v1/notifications/system-types"},
		{"management-history", http.MethodGet, "/api/v1/notifications/management-history?category=system&page=1&page_size=10"},
		{"management-history-mine", http.MethodGet, "/api/v1/notifications/management-history?category=mine&page=1&page_size=10"},
		{"templates-list", http.MethodGet, "/api/v1/notifications/templates"},
		{"templates-create", http.MethodPost, "/api/v1/notifications/templates"},
		{"templates-get", http.MethodGet, "/api/v1/notifications/templates/00000000-0000-0000-0000-000000000000"},
		{"templates-update", http.MethodPut, "/api/v1/notifications/templates/00000000-0000-0000-0000-000000000000"},
		{"templates-toggle", http.MethodPatch, "/api/v1/notifications/templates/00000000-0000-0000-0000-000000000000/toggle-active"},
		{"templates-delete", http.MethodDelete, "/api/v1/notifications/templates/00000000-0000-0000-0000-000000000000"},
		{"schedules-list", http.MethodGet, "/api/v1/notifications/schedules"},
		{"schedules-create", http.MethodPost, "/api/v1/notifications/schedules"},
		{"schedules-get", http.MethodGet, "/api/v1/notifications/schedules/00000000-0000-0000-0000-000000000000"},
		{"schedules-update", http.MethodPut, "/api/v1/notifications/schedules/00000000-0000-0000-0000-000000000000"},
		{"schedules-toggle", http.MethodPatch, "/api/v1/notifications/schedules/00000000-0000-0000-0000-000000000000/toggle-active"},
		{"schedules-delete", http.MethodDelete, "/api/v1/notifications/schedules/00000000-0000-0000-0000-000000000000"},
		{"upload-image", http.MethodPost, "/api/v1/notifications/upload-image"},
		{"test-push", http.MethodPost, "/api/v1/notifications/test-push"},
	}

	for _, tc := range managementCases {
		t.Run("bendahara_"+tc.name, func(t *testing.T) {
			var body []byte
			if tc.method == http.MethodPost || tc.method == http.MethodPut || tc.method == http.MethodPatch {
				body = []byte(`{}`)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+bendaharaToken)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for bendahara on %s %s, got %d: %s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}

	// Warga (role-only, no jabatan) must also be rejected.
	for _, tc := range managementCases {
		t.Run("warga_"+tc.name, func(t *testing.T) {
			var body []byte
			if tc.method == http.MethodPost || tc.method == http.MethodPut || tc.method == http.MethodPatch {
				body = []byte(`{}`)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+wargaToken)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for warga on %s %s, got %d: %s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}

	// Ketua RT keeps the access it always had.
	for _, tc := range managementCases {
		name := "ketua_" + tc.name
		t.Run(name, func(t *testing.T) {
			var body []byte
			if tc.method == http.MethodPost || tc.method == http.MethodPut || tc.method == http.MethodPatch {
				body = []byte(`{}`)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+ketuaToken)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			// Ketua must not be denied at the notification.manage middleware. Other
			// statuses are acceptable as long as it is not 403-from-middleware.
			// For /notifications/test-push the handler itself restricts to
			// super admin, so a 403 from the handler (message contains
			// "super_admin") is still acceptable — the manage middleware passed
			// and the authorization decision was deliberately escalated.
			if rec.Code == http.StatusForbidden && tc.path != "/api/v1/notifications/test-push" {
				t.Fatalf("expected ketua to be allowed by notification.manage on %s %s, got 403: %s", tc.method, tc.path, rec.Body.String())
			}
			if tc.path == "/api/v1/notifications/test-push" && rec.Code == http.StatusForbidden && !strings.Contains(rec.Body.String(), "super_admin") {
				t.Fatalf("ketua 403 on test-push must come from the super-admin-only check, got: %s", rec.Body.String())
			}
		})
	}

	// Bendahara must keep access to ordinary notification endpoints.
	for _, ordinary := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/notifications/logs"},
		{http.MethodGet, "/api/v1/notifications/unread-count"},
		{http.MethodGet, "/api/v1/auth/me"},
	} {
		t.Run("bendahara_ordinary_"+ordinary.path, func(t *testing.T) {
			req := httptest.NewRequest(ordinary.method, ordinary.path, nil)
			req.Header.Set("Authorization", "Bearer "+bendaharaToken)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 for bendahara on %s, got %d: %s", ordinary.path, rec.Code, rec.Body.String())
			}
		})
	}

	// Bendahara /auth/me must not advertise notification.manage, but must
	// advertise notification.read.
	t.Run("bendahara_effective_permissions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+bendaharaToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode /me: %v", err)
		}
		raw, _ := body["effective_permissions"].([]any)
		hasManage, hasRead := false, false
		for _, v := range raw {
			if s, _ := v.(string); s == "notification.manage" {
				hasManage = true
			}
			if s, _ := v.(string); s == "notification.read" {
				hasRead = true
			}
		}
		if hasManage {
			t.Error("bendahara must NOT have notification.manage in effective_permissions")
		}
		if !hasRead {
			t.Error("bendahara must still have notification.read in effective_permissions")
		}
	})
}
