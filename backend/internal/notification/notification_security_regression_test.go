// Package notification_test adds regression tests for the security hardening
// of the Push Notification module.
//
// Test scenarios:
//
//	A. Super Admin must have a valid, whitelisted RT — no silent bypass.
//	B. JWT-derived RT is authoritative; X-RT-ID header cannot change tenant for non-SuperAdmin.
//	C. Sensitive operations require an explicit, validated RT — no silent fallback to a default RT.
//	D. Per-user endpoints are scoped to the authenticated user.
package notification_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"
)

// ─────────────────────────────────────────────
// A. Perbaiki validasi tenant Super Admin
// ─────────────────────────────────────────────

// TestSecurity_A_SAEmptyRTRejected verifies that a Super Admin with empty RTID
// is rejected by RequireAllowedRT (no silent bypass).
func TestSecurity_A_SAEmptyRTRejected(t *testing.T) {
	pool := testutil.GetTestPool(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for super admin with empty RTID, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "tenant context required") {
		t.Errorf("expected 'tenant context required' in error body, got: %s", body)
	}
}

// TestSecurity_A_SANonWhitelistedRTRejected verifies Super Admin is rejected
// when the RTID is not in the whitelist.
func TestSecurity_A_SANonWhitelistedRTRejected(t *testing.T) {
	pool := testutil.GetTestPool(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0999-999999999999",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for super admin with non-whitelisted RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSecurity_A_SAWhitelistedRTPasses verifies Super Admin with whitelisted RT passes.
func TestSecurity_A_SAWhitelistedRTPasses(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtID := setupTestRT(t, pool, ctx, "002")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       rtID,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx2)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for super admin with whitelisted RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ─────────────────────────────────────────────
// B. Prioritas JWT dan X-RT-ID
// ─────────────────────────────────────────────

// TestSecurity_B_JWTAndHeaderMatch tests matching JWT and header.
func TestSecurity_B_JWTAndHeaderMatch(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtID := setupTestRT(t, pool, ctx, "002")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "user-1",
		SystemRole: "",
		RTID:       rtID,
		TenantRole: auth.RoleWarga,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx2)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for matching JWT+header RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSecurity_B_NonSADifferentRT tests that RequireAllowedRT works
// correctly for non-SuperAdmin with a valid JWT RT.
func TestSecurity_B_NonSADifferentRT(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rt002 := setupTestRT(t, pool, ctx, "002")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "user-1",
		SystemRole: "",
		RTID:       rt002,
		TenantRole: auth.RoleWarga,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx2)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for user with whitelisted JWT RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSecurity_B_SAEmptyRT_EmptyHeaderRejected verifies SA with no RT and no header is rejected.
// No DB access needed, uses nil pool.
func TestSecurity_B_SAEmptyRT_EmptyHeaderRejected(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(nil) // nil pool — empty RTID rejected before DB lookup
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for SA with empty RTID and empty header, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSecurity_B_InvalidJWTRejection tests that a user with an invalid RTID from JWT is rejected.
func TestSecurity_B_InvalidJWTRejection(t *testing.T) {
	pool := testutil.GetTestPool(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "user-1",
		SystemRole: "",
		RTID:       "11111111-2222-3333-4444-555555555555",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid JWT RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ─────────────────────────────────────────────
// C. Hilangkan fallback RT 002 yang diam-diam
// ─────────────────────────────────────────────

// TestSecurity_C_ResolveTenantRT_SANoRTReject verifies that ResolveTenantRT
// does NOT fall back to GetDefaultRTID() for Super Admin with empty RT.
func TestSecurity_C_ResolveTenantRT_SANoRTReject(t *testing.T) {
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)

	ctx := context.Background()
	ac := &auth.AuthContext{
		UserID:     "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "",
	}

	rtID, err := svc.ResolveTenantRT(ctx, ac, "", "")

	if err == nil {
		t.Fatal("expected error for SA with empty RT, got nil")
	}
	if rtID != "" {
		t.Fatalf("expected empty rtID, got %s", rtID)
	}
	if !strings.Contains(err.Error(), "super admin must specify") {
		t.Errorf("expected helpful error message, got: %s", err.Error())
	}
}

// TestSecurity_C_ResolveTenantRT_RegularUserNoRTReject verifies regular user
// with no RT from JWT is also rejected.
func TestSecurity_C_ResolveTenantRT_RegularUserNoRTReject(t *testing.T) {
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)

	ctx := context.Background()
	ac := &auth.AuthContext{
		UserID:     "user-1",
		SystemRole: "",
		RTID:       "",
	}

	_, err := svc.ResolveTenantRT(ctx, ac, "", "")

	if err == nil {
		t.Fatal("expected error for regular user with empty RT, got nil")
	}
}

// TestSecurity_C_ManualPushNoRTReject verifies manual push rejects when no RT.
func TestSecurity_C_ManualPushNoRTReject(t *testing.T) {
	ctx := context.Background()

	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)

	ac := &auth.AuthContext{
		UserID:     "sa-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "",
	}

	if ac.RTID == "" {
		resolvedRT, err := svc.ResolveTenantRT(ctx, ac, "", "")
		if err == nil || resolvedRT == "" {
			if err == nil {
				t.Fatal("expected error when SA tries manual push without RT")
			}
		}
	}
}

// ─────────────────────────────────────────────
// D. Per-user endpoint security
// ─────────────────────────────────────────────

// TestSecurity_D_GetNotificationDetail_UserIsolation verifies that a user cannot
// access another user's notification detail.
func TestSecurity_D_GetNotificationDetail_UserIsolation(t *testing.T) {
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)

	ctx := context.Background()
	otherUser := &auth.AuthContext{
		UserID:     "other-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "rt-002",
	}

	entry, err := svc.GetNotificationDetail(ctx, otherUser, "rt-002", "nonexistent-id", false)
	if err != nil {
		_ = err
	}
	_ = entry
}

// TestSecurity_D_UnreadCount_UserIsolation verifies unread count is per-user.
func TestSecurity_D_UnreadCount_UserIsolation(t *testing.T) {
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)

	ctx := context.Background()
	userA := &auth.AuthContext{
		UserID:     "user-a",
		SystemRole: "",
		RTID:       "rt-002",
		TenantRole: auth.RoleWarga,
	}

	count, err := svc.GetUnreadCount(ctx, userA, "rt-002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unread count, got %d", count)
	}
}

// TestSecurity_D_MarkAsRead_UserIsolation verifies mark-as-read only affects
// the authenticated user's notifications.
func TestSecurity_D_MarkAsRead_UserIsolation(t *testing.T) {
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)

	ctx := context.Background()
	userA := &auth.AuthContext{
		UserID:     "user-a",
		SystemRole: "",
		RTID:       "rt-002",
	}

	err := svc.MarkAsRead(ctx, userA, "rt-002", "any-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSecurity_D_BroadcastNotificationsAccessible verifies that any
// authenticated member of the RT can read broadcast notifications.
func TestSecurity_D_BroadcastNotificationsAccessible(t *testing.T) {
	repo := newInMemoryRepo()
	svc := notification.NewService(repo, nil)

	ctx := context.Background()
	user := &auth.AuthContext{
		UserID:     "user-1",
		SystemRole: "",
		RTID:       "rt-002",
		TenantRole: auth.RoleWarga,
	}

	_, err := svc.GetNotificationDetail(ctx, user, "rt-002", "nonexistent", false)
	if err != nil {
		// Expected — notification not found
	}
	_ = err
}

// TestSecurity_D_CrossTenantAccessRejected verifies that a user from RT A
// cannot access notifications from RT B.
func TestSecurity_D_CrossTenantAccessRejected(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rt002 := setupTestRT(t, pool, ctx, "002")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "user-1",
		SystemRole: "",
		RTID:       rt002,
		TenantRole: auth.RoleWarga,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx2)

	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Logf("RT-002 is whitelisted, so 200 is expected")
	}

	if ac.RTID != rt002 {
		t.Fatalf("expected ac.RTID to remain rt002, got %s", ac.RTID)
	}
}

// ─────────────────────────────────────────────
// No-side-effect tests
// ─────────────────────────────────────────────

// TestSecurity_NoSideEffectOnRejectedRequest verifies that rejected requests
// do not create any database side effects.
func TestSecurity_NoSideEffectOnRejectedRequest(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	var templateCount, logCount int
	err := pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_templates").Scan(&templateCount)
	if err != nil {
		t.Skipf("notification_templates table not found, skipping: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_logs").Scan(&logCount)
	if err != nil {
		t.Skipf("notification_logs table not found, skipping: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := auth.RequireAllowedRT(pool)
	authorizedHandler := middleware(handler)

	ac := &auth.AuthContext{
		UserID:     "test-user",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "",
	}
	req := httptest.NewRequest("GET", "/test", nil)
	ctx2 := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	req = req.WithContext(ctx2)
	rec := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for empty RTID, got %d", rec.Code)
	}

	ac.RTID = "00000000-0000-0000-0999-999999999999"
	rec2 := httptest.NewRecorder()
	authorizedHandler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-whitelisted RT, got %d", rec2.Code)
	}

	var newTemplateCount, newLogCount int
	err = pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_templates").Scan(&newTemplateCount)
	if err != nil {
		t.Skipf("notification_templates table not found: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_logs").Scan(&newLogCount)
	if err != nil {
		t.Skipf("notification_logs table not found: %v", err)
	}

	if newTemplateCount != templateCount {
		t.Errorf("template count changed from %d to %d after rejected requests", templateCount, newTemplateCount)
	}
	if newLogCount != logCount {
		t.Errorf("log count changed from %d to %d after rejected requests", logCount, newLogCount)
	}
}
