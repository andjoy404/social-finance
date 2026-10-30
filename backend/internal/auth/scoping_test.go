package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

func TestLoadScopingValidUUIDs(t *testing.T) {
	os.Setenv("SCOPED_RT_IDS", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b,123e4567-e89b-12d3-a456-426614174000")
	os.Setenv("SCOPED_RW_IDS", "aaaa1111-bbbb-cccc-dddd-eeee22222222,bbbb1111-cccc-dddd-eeee-ffff22222222")
	defer os.Unsetenv("SCOPED_RT_IDS")
	defer os.Unsetenv("SCOPED_RW_IDS")

	cfg, err := LoadScoping()
	if err != nil {
		t.Fatalf("LoadScoping() returned error: %v", err)
	}
	if len(cfg.RTIDs) != 2 {
		t.Errorf("expected 2 RT IDs, got %d", len(cfg.RTIDs))
	}
	if len(cfg.RWIDs) != 2 {
		t.Errorf("expected 2 RW IDs, got %d", len(cfg.RWIDs))
	}
	if cfg.RTIDs[0] != "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b" {
		t.Errorf("expected first RT ID, got %s", cfg.RTIDs[0])
	}
	if cfg.RWIDs[0] != "aaaa1111-bbbb-cccc-dddd-eeee22222222" {
		t.Errorf("expected first RW ID, got %s", cfg.RWIDs[0])
	}
}

func TestLoadScopingMissingRT(t *testing.T) {
	os.Setenv("SCOPED_RT_IDS", "")
	os.Setenv("SCOPED_RW_IDS", "aaaa1111-bbbb-cccc-dddd-eeee22222222")
	defer os.Unsetenv("SCOPED_RT_IDS")
	defer os.Unsetenv("SCOPED_RW_IDS")

	_, err := LoadScoping()
	if err == nil {
		t.Fatal("expected error for missing SCOPED_RT_IDS")
	}
}

func TestLoadScopingMissingRW(t *testing.T) {
	// SCOPED_RW_IDS is optional. When missing the application starts
	// with an empty RW allowlist — all RW access remains closed.
	os.Setenv("SCOPED_RT_IDS", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b")
	os.Setenv("SCOPED_RW_IDS", "")
	defer os.Unsetenv("SCOPED_RT_IDS")
	defer os.Unsetenv("SCOPED_RW_IDS")

	cfg, err := LoadScoping()
	if err != nil {
		t.Fatalf("LoadScoping() returned error: %v", err)
	}
	if len(cfg.RTIDs) != 1 {
		t.Errorf("expected 1 RT ID, got %d", len(cfg.RTIDs))
	}
	if len(cfg.RWIDs) != 0 {
		t.Errorf("expected 0 RW IDs (optional, empty allowed), got %d", len(cfg.RWIDs))
	}
}

func TestLoadScopingUnsetRW(t *testing.T) {
	// SCOPED_RW_IDS unset (not in env at all) should also succeed.
	os.Setenv("SCOPED_RT_IDS", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b")
	os.Unsetenv("SCOPED_RW_IDS")
	defer os.Unsetenv("SCOPED_RT_IDS")

	cfg, err := LoadScoping()
	if err != nil {
		t.Fatalf("LoadScoping() returned error: %v", err)
	}
	if len(cfg.RWIDs) != 0 {
		t.Errorf("expected 0 RW IDs (unset), got %d", len(cfg.RWIDs))
	}
}

func TestLoadScopingRTValidRWInvalid(t *testing.T) {
	// Invalid RW UUID should still produce an error.
	os.Setenv("SCOPED_RT_IDS", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b")
	os.Setenv("SCOPED_RW_IDS", "not-a-uuid")
	defer os.Unsetenv("SCOPED_RT_IDS")
	defer os.Unsetenv("SCOPED_RW_IDS")

	_, err := LoadScoping()
	if err == nil {
		t.Fatal("expected error for invalid UUID in SCOPED_RW_IDS")
	}
}

func TestLoadScopingRTOnlyDeployment(t *testing.T) {
	// Simulates RT-only production deployment: RT configured, RW not configured.
	os.Setenv("SCOPED_RT_IDS", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b,123e4567-e89b-12d3-a456-426614174000")
	os.Unsetenv("SCOPED_RW_IDS")
	defer os.Unsetenv("SCOPED_RT_IDS")

	cfg, err := LoadScoping()
	if err != nil {
		t.Fatalf("LoadScoping() returned error: %v", err)
	}
	if len(cfg.RTIDs) != 2 {
		t.Errorf("expected 2 RT IDs, got %d", len(cfg.RTIDs))
	}
	if cfg.RWIDs != nil {
		t.Errorf("expected nil RWIDs for RT-only deployment, got %v", cfg.RWIDs)
	}
}

func TestLoadScopingRTEmpty(t *testing.T) {
	// RT empty → must still fail regardless of RW state.
	os.Unsetenv("SCOPED_RT_IDS")
	os.Setenv("SCOPED_RW_IDS", "aaaa1111-bbbb-cccc-dddd-eeee22222222")
	defer os.Unsetenv("SCOPED_RT_IDS")
	defer os.Unsetenv("SCOPED_RW_IDS")

	_, err := LoadScoping()
	if err == nil {
		t.Fatal("expected error for missing SCOPED_RT_IDS")
	}
}

func TestRequireRWScopeWithEmptyRWIDs(t *testing.T) {
	// Even when SCOPED_RW_IDS is empty, RW access must still be rejected.
	// The middleware rejects SA via hasRW() (always false with empty RWIDs)
	// and rejects regular users via RWMemberships check.
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: nil, // empty scope — RT-only deployment
	}

	// SA with memberships should still be rejected (hasRW returns false).
	ac := &AuthContext{
		UserID:       "user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA rejected with empty RWIDs), got %d, body: %s", rec.Code, rec.Body.String())
	}

	// User with RW membership should also be rejected.
	ac2 := &AuthContext{
		UserID: "user-id",
		RTID:   "rt-uuid-1",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec2 := callRequireRWScopeMiddleware(ac2, "/api/v1/rw/rw-uuid-1/beranda", cfg)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403 (user rejected with empty RWIDs), got %d, body: %s", rec2.Code, rec2.Body.String())
	}
}

func TestRequireRWScopeSAWithMembershipsEmptyScope(t *testing.T) {
	// SA has RW membership but SCOPED_RW_IDS is empty.
	// Should be rejected (no SA has RW access when scope is empty).
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{}, // empty slice, not nil
	}

	ac := &AuthContext{
		UserID:       "user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA rejected with empty RWIDs slice), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestLoadScopingInvalidUUID(t *testing.T) {
	os.Setenv("SCOPED_RT_IDS", "not-a-uuid")
	os.Setenv("SCOPED_RW_IDS", "aaaa1111-bbbb-cccc-dddd-eeee22222222")
	defer os.Unsetenv("SCOPED_RT_IDS")
	defer os.Unsetenv("SCOPED_RW_IDS")

	_, err := LoadScoping()
	if err == nil {
		t.Fatal("expected error for invalid UUID in SCOPED_RT_IDS")
	}
}

func TestScopingHasRT(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"rt-uuid-1", "rt-uuid-2"},
		RWIDs: []string{"rw-uuid-1"},
	}
	if !cfg.hasRT("rt-uuid-1") {
		t.Error("expected hasRT to return true for rt-uuid-1")
	}
	if !cfg.hasRT("rt-uuid-2") {
		t.Error("expected hasRT to return true for rt-uuid-2")
	}
	if cfg.hasRT("rt-uuid-999") {
		t.Error("expected hasRT to return false for rt-uuid-999")
	}
}

func TestScopingHasRW(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}
	if !cfg.hasRW("rw-uuid-1") {
		t.Error("expected hasRW to return true for rw-uuid-1")
	}
	if !cfg.hasRW("rw-uuid-2") {
		t.Error("expected hasRW to return true for rw-uuid-2")
	}
	if cfg.hasRW("rw-uuid-999") {
		t.Error("expected hasRW to return false for rw-uuid-999")
	}
}

func TestExtractRWIDFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/v1/rw/rw-uuid-1/beranda", "rw-uuid-1"},
		{"/rw/rw-uuid-2/kas", "rw-uuid-2"},
		{"/api/v1/rt/rt-uuid-1/users", ""},
		{"/api/v1/transactions", ""},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", tt.path, nil)
		got := extractRWIDFromPath(r)
		if got != tt.expected {
			t.Errorf("extractRWIDFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestHasRWScopePermission(t *testing.T) {
	// ketua: full finance access.
	if !hasRWScopePermission("ketua", "finance.read") {
		t.Error("ketua should have finance.read")
	}
	if !hasRWScopePermission("ketua", "finance.approve") {
		t.Error("ketua should have finance.approve")
	}
	// ketua does NOT have transfer — that's handled by checkRWBendaharaAccess.
	if hasRWScopePermission("ketua", "user.delete") {
		t.Error("ketua should NOT have user.delete")
	}
	// wakil_ketua: NO finance.update — security gate finding.
	if !hasRWScopePermission("wakil_ketua", "finance.read") {
		t.Error("wakil_ketua should have finance.read")
	}
	if hasRWScopePermission("wakil_ketua", "finance.update") {
		t.Error("wakil_ketua must NOT have finance.update (security gate)")
	}
	if !hasRWScopePermission("wakil_ketua", "finance.approve") {
		t.Error("wakil_ketua should have finance.approve")
	}
	// sekretaris: read-only.
	if !hasRWScopePermission("sekretaris", "finance.read") {
		t.Error("sekretaris should have finance.read")
	}
	if hasRWScopePermission("sekretaris", "finance.create") {
		t.Error("sekretaris should NOT have finance.create")
	}
	if hasRWScopePermission("sekretaris", "finance.update") {
		t.Error("sekretaris should NOT have finance.update")
	}
	// bendahara: full.
	if !hasRWScopePermission("bendahara", "finance.update") {
		t.Error("bendahara should have finance.update")
	}
	// nonexistent jabatan.
	if hasRWScopePermission("nonexistent", "finance.read") {
		t.Error("nonexistent jabatan should return false")
	}
}

func TestRequireRWScopeSuperAdminWithMembershipsPoolNil(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID:       "user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	// SA with memberships + pool nil → fail-closed 500, not 200.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (pool nil, fail-closed), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeSAWithMembershipsButNoAllowlist(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-999"}, // rw-uuid-1 NOT in allowlist.
	}

	ac := &AuthContext{
		UserID:       "user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	// SA has membership for rw-uuid-1 but NOT in SCOPED_RW_IDS → 403.
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA membership without scope allowlist), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeUserMultipleMembershipsMatchRWID(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	// User has multiple RW memberships in different order.
	// The middleware must match by rwId, not first element.
	ac := &AuthContext{
		UserID: "user-id",
		RTID:   "rt-uuid-1",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-2", Jabatan: "sekretaris"},
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (rw-uuid-1 found in RWMemberships), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeMultipleMembershipsWrongRW(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID: "user-id",
		RTID:   "rt-uuid-1",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-2", Jabatan: "ketua"},
			{RWID: "rw-uuid-3", Jabatan: "bendahara"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (no membership for rw-uuid-1), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeUserHasRWMembershipButNotInScope(t *testing.T) {
	// This tests: user has RW membership, scope has the RW ID, but
	// the specific rwId in URL doesn't match any membership.
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1"},
	}

	ac := &AuthContext{
		UserID: "user-id",
		RTID:   "rt-uuid-1",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-999", Jabatan: "bendahara"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	// Scope allows rw-uuid-1, but user only has membership for rw-uuid-999.
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (membership rw-uuid-999 doesn't match URL rw-uuid-1), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func callRequireRWScopeMiddleware(ac *AuthContext, rwPath string, cfg ScopingConfig) *httptest.ResponseRecorder {
	return callRequireRWScopeMiddlewareWithPool(ac, rwPath, cfg, nil)
}

func callRequireRWScopeMiddlewareWithPool(ac *AuthContext, rwPath string, cfg ScopingConfig, pool *database.Pool) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", rwPath, nil)
	req = req.WithContext(context.WithValue(req.Context(), AuthContextKey, ac))
	rec := httptest.NewRecorder()

	h := RequireRWScope(pool, cfg)
	// RequireRWScope returns func(http.Handler) http.Handler
	// Wrap a simple handler and call it
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h(next).ServeHTTP(rec, req)
	return rec
}

func TestRequireRWScopeSuperAdminNoMemberships(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID:       "user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	// SA without RW memberships → pool nil → 500 (DB unavailable, fail-closed).
	// In production with a real pool and active RW, this would return 200.
	// The key change: SA no longer requires RW membership (was 403, now bypasses membership check).
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (SA without RW memberships, pool nil), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeSuperAdminWithMembershipsAndScopeAllowlist(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID:       "user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	// SA with memberships + scope allowlist match, but pool nil → 500 (DB unavailable).
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (pool nil, fail-closed), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeUserWithMatchingMembership(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID: "user-id",
		RTID:   "rt-uuid-1",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
			{RWID: "rw-uuid-2", Jabatan: "bendahara"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for matching rwId, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeUserWrongRW(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID: "user-id",
		RTID:   "rt-uuid-1",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-999", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-matching rwId, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeNoRWContext(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1"},
	}

	ac := &AuthContext{
		UserID:          "user-id",
		RTID:            "rt-uuid-1",
		TenantRole:      RoleBendahara,
		RWMemberships:   nil, // no RW memberships
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/beranda", cfg)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for user without RW context, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeInvalidRWPath(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1"},
	}

	ac := &AuthContext{
		UserID: "user-id",
		RTID:   "rt-uuid-1",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/transactions", cfg)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing rwId in path, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestParseUUIDList(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantError bool
	}{
		{"empty", "", 0, false},
		{"single valid", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", 1, false},
		{"two valid", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b,123e4567-e89b-12d3-a456-426614174000", 2, false},
		{"with spaces", " a95c1cb5-2f67-4ea5-9197-bfec537b6d7b , 123e4567-e89b-12d3-a456-426614174000 ", 2, false},
		{"invalid uuid", "not-a-uuid", 0, true},
		{"mixed valid/invalid", "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b,not-uuid", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseUUIDList(tt.input)
			if tt.wantError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d UUIDs, got %d", tt.wantCount, len(result))
			}
		})
	}
}

// ─── Super Admin RW Access Tests ────────────────────────────────────────────
// These tests verify the Super Admin bypass of RW membership requirements
// while still enforcing configured scope boundaries.

func TestRequireRWScopeSuperAdminNoMembershipsRWInScope(t *testing.T) {
	// SA without RW memberships → 500 (pool nil, fail-closed).
	// In production with a real pool and active RW, this would return 200.
	// Key change: SA bypasses membership check entirely; pool nil means DB check fails.
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1"},
	}
	ac := &AuthContext{
		UserID:       "sa-user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}
	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/dashboard", cfg)
	// pool is nil → 500 (cannot verify RW active status).
	// With a real pool and active RW, this would return 200 (allowed).
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (pool nil, SA allowed to proceed), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeSuperAdminNoMembershipsRWScopeNotInList(t *testing.T) {
	// SA without RW memberships, RW NOT in SCOPED_RW_IDS → 403.
	// This test verifies the deployment-level gate still applies to SA.
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1"}, // only rw-uuid-1 is allowed
	}
	ac := &AuthContext{
		UserID:       "sa-user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}
	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-999/dashboard", cfg)
	// 403 because rw-uuid-999 is not in SCOPED_RW_IDS.
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA, RW not in configured scope), got %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "" || rec.Body.String() == "null\n" {
		t.Error("expected error body with message")
	}
}

func TestRequireRWScopeUserWithoutRWMembership(t *testing.T) {
	// Non-SA user without RW membership → 403.
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1"},
	}
	ac := &AuthContext{
		UserID:       "regular-user",
		SystemRole:   "",
		MembershipID: "memb-1",
		RTID:         "rt-uuid-1",
	}
	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/dashboard", cfg)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (non-SA without RW membership), got %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "" || rec.Body.String() == "null\n" {
		t.Error("expected error body with message")
	}
}

func TestRequireRWScopeUserWithWrongMembershipRW(t *testing.T) {
	// User with RW membership for rw-uuid-2, trying rw-uuid-1 → 403.
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}
	ac := &AuthContext{
		UserID:  "user-1",
		RTID:    "rt-uuid-1",
		SystemRole: "",
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-2", Jabatan: "ketua"},
		},
	}
	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/dashboard", cfg)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (non-SA, membership doesn't match requested rw), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireRWScopeSAWithMultipleScopeAccess(t *testing.T) {
	// SA can access any RW in configured scope.
	// Without pool, this returns 500 (DB unavailable). With pool + active RW → 200.
	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2", "rw-uuid-3"},
	}
	ac := &AuthContext{
		UserID:       "sa-user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}

	// Test first RW in scope — pool nil → 500
	rec1 := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-1/dashboard", cfg)
	if rec1.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (pool nil), got %d, body: %s", rec1.Code, rec1.Body.String())
	}

	// Test another RW in scope — same result
	rec2 := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-2/kas", cfg)
	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (pool nil), got %d, body: %s", rec2.Code, rec2.Body.String())
	}

	// RW NOT in scope → 403
	rec3 := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-999/dashboard", cfg)
	if rec3.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA, RW not in scope), got %d, body: %s", rec3.Code, rec3.Body.String())
	}
}

// TestRequireRWScopeSuperAdminWithPool verifies that a SUPER_ADMIN user without
// RW membership can access RW endpoints when a database pool is available
// and the RW exists and is active.
//
// This test uses an isolated test database pool (via testutil.GetTestPool) to
// avoid the pool nil → 500 path and actually verify the SA bypass works end-to-end.
func TestRequireRWScopeSuperAdminWithPool(t *testing.T) {
	pool := testutil.GetTestPool(t)

	ctx := context.Background()
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	rwID := uuid.New()
	rtID := uuid.New()
	// Generate a random RT code to avoid uniqueness conflicts with existing test data
	rtCode := fmt.Sprintf("%03d", 900+int(rwID[0]))

	// Insert active RW into the test database
	_, err = tx.ExecContext(ctx, `
		INSERT INTO rws (id, name, rw_number, is_active)
		VALUES ($1, 'RW Test', 16, true)
	`, rwID)
	if err != nil {
		t.Fatalf("insert rw: %v", err)
	}

	// Insert associated RT
	_, err = tx.ExecContext(ctx, `
		INSERT INTO rts (id, name, rt, rw, rw_id, is_active)
		VALUES ($1, 'RT Test', $2, 16, $3, true)
	`, rtID, rtCode, rwID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	cfg := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{rwID.String()}, // RW in configured scope
	}

	// SA without RW membership, pool provided → should pass (200)
	ac := &AuthContext{
		UserID:       "sa-user-id",
		SystemRole:   SystemRoleSuperAdmin,
		MembershipID: "",
		RTID:         "",
	}

	rec := callRequireRWScopeMiddlewareWithPool(ac, "/api/v1/rw/"+rwID.String()+"/dashboard", cfg, pool)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (SA with pool, RW active), got %d, body: %s", rec.Code, rec.Body.String())
	}

	// SA without membership, RW NOT in configured scope → 403
	offScopeRW := uuid.New()
	recOffScope := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+offScopeRW.String()+"/dashboard", cfg)
	if recOffScope.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA, RW not in configured scope), got %d, body: %s", recOffScope.Code, recOffScope.Body.String())
	}

	// SA without membership, RW exists but inactive → 403
	inactiveRW := uuid.New()
	tx2, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	_, err = tx2.ExecContext(ctx, `
		INSERT INTO rws (id, name, rw_number, is_active)
		VALUES ($1, 'RW Inactive', 99, false)
	`, inactiveRW)
	if err != nil {
		t.Fatalf("insert inactive rw: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit tx2: %v", err)
	}

	cfgInactive := ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{inactiveRW.String()},
	}
	recInactive := callRequireRWScopeMiddlewareWithPool(ac, "/api/v1/rw/"+inactiveRW.String()+"/dashboard", cfgInactive, pool)
	if recInactive.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA, RW inactive), got %d, body: %s", recInactive.Code, recInactive.Body.String())
	}
}

// ===========================================================================
// TEST: Error Classification
// ===========================================================================

func TestValidateSAForRW_ErrorClassification(t *testing.T) {
	ctx := context.Background()

	t.Run("cfg nil returns ErrRWScopeInvalid", func(t *testing.T) {
		err := ValidateSAForRW(ctx, nil, nil, "some-rw-id")
		require.ErrorIs(t, err, ErrRWScopeInvalid)
	})

	t.Run("cfg with empty RWIDs returns ErrRWScopeInvalid", func(t *testing.T) {
		cfg := &ScopingConfig{RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"}, RWIDs: []string{}}
		err := ValidateSAForRW(ctx, cfg, nil, "some-rw-id")
		require.ErrorIs(t, err, ErrRWScopeInvalid)
	})

	t.Run("cfg non-nil but RW not in allowlist returns ErrRWScopeInvalid", func(t *testing.T) {
		cfg := &ScopingConfig{RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"}, RWIDs: []string{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}}
		err := ValidateSAForRW(ctx, cfg, nil, "ffffaaaa-1111-2222-3333-444455556666")
		require.ErrorIs(t, err, ErrRWScopeInvalid)
	})

	t.Run("pool nil returns ErrRWInfrastructure", func(t *testing.T) {
		cfg := &ScopingConfig{RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"}, RWIDs: []string{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}}
		err := ValidateSAForRW(ctx, cfg, nil, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
		require.ErrorIs(t, err, ErrRWInfrastructure)
		require.NotErrorIs(t, err, ErrRWScopeInvalid)
	})
}

func TestMiddlewareErrorClassification(t *testing.T) {
	pool := testutil.GetTestPool(t)

	// Create a test RW that is active.
	var activeRWID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('Error Classification Test', 999, true) RETURNING id`,
	).Scan(&activeRWID)
	require.NoError(t, err)

	// Create a scope config containing this RW.
	cfg := &ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{activeRWID},
	}

	t.Run("pool nil → 500 for SA", func(t *testing.T) {
		ac := &AuthContext{
			SystemRole:    SystemRoleSuperAdmin,
			RWMemberships: []RWMembership{},
		}
		rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/"+activeRWID+"/dashboard", *cfg)
		require.Equal(t, http.StatusInternalServerError, rec.Code,
			"pool nil should return 500, got %d: %s", rec.Code, rec.Body.String())
	})

	t.Run("pool not nil + SA + valid RW → 200", func(t *testing.T) {
		ac := &AuthContext{
			SystemRole:    SystemRoleSuperAdmin,
			RWMemberships: []RWMembership{},
		}
		rec := callRequireRWScopeMiddlewareWithPool(ac, "/api/v1/rw/"+activeRWID+"/dashboard", *cfg, pool)
		require.Equal(t, http.StatusOK, rec.Code,
			"valid SA + valid RW should return 200, got %d: %s", rec.Code, rec.Body.String())
	})

	t.Run("pool not nil + SA + inactive RW → 403", func(t *testing.T) {
		// Mark RW inactive.
		pool.Raw().ExecContext(context.Background(), `UPDATE rws SET is_active = false WHERE id = $1`, activeRWID)
		defer pool.Raw().ExecContext(context.Background(), `UPDATE rws SET is_active = true WHERE id = $1`, activeRWID)

		ac := &AuthContext{
			SystemRole:    SystemRoleSuperAdmin,
			RWMemberships: []RWMembership{},
		}
		rec := callRequireRWScopeMiddlewareWithPool(ac, "/api/v1/rw/"+activeRWID+"/dashboard", *cfg, pool)
		require.Equal(t, http.StatusForbidden, rec.Code,
			"SA + inactive RW should return 403, got %d: %s", rec.Code, rec.Body.String())
	})

	t.Run("pool not nil + SA + RW not in allowlist → 403", func(t *testing.T) {
		rwNotInList := uuid.New().String()
		ac := &AuthContext{
			SystemRole:    SystemRoleSuperAdmin,
			RWMemberships: []RWMembership{},
		}
		rec := callRequireRWScopeMiddlewareWithPool(ac, "/api/v1/rw/"+rwNotInList+"/dashboard", *cfg, pool)
		require.Equal(t, http.StatusForbidden, rec.Code,
			"SA + RW not in allowlist should return 403, got %d: %s", rec.Code, rec.Body.String())
	})

	t.Run("pool not nil + SA + RW does not exist in DB → 403", func(t *testing.T) {
		rwNotInDB := uuid.New().String()
		// Modify cfg to include this non-existent RW.
		cfgWithFake := &ScopingConfig{
			RTIDs: cfg.RTIDs,
			RWIDs: []string{rwNotInDB},
		}
		ac := &AuthContext{
			SystemRole:    SystemRoleSuperAdmin,
			RWMemberships: []RWMembership{},
		}
		rec := callRequireRWScopeMiddlewareWithPool(ac, "/api/v1/rw/"+rwNotInDB+"/dashboard", *cfgWithFake, pool)
		require.Equal(t, http.StatusForbidden, rec.Code,
			"SA + RW not in DB should return 403, got %d: %s", rec.Code, rec.Body.String())
	})
}

func TestValidateSAForRW_NoSensitiveDataLeak(t *testing.T) {
	ctx := context.Background()
	pool := testutil.GetTestPool(t)

	// Create a test RW.
	var rwID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('Leak Test', 888, true) RETURNING id`,
	).Scan(&rwID)
	require.NoError(t, err)

	cfg := &ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: []string{rwID},
	}

	// Valid access → no error.
	err = ValidateSAForRW(ctx, cfg, pool, rwID)
	require.NoError(t, err)

	// Verify that error messages do NOT leak internal implementation details
	// such as SQL column names, table names, or query text.
	t.Run("cfg nil error does not leak internals", func(t *testing.T) {
		err := ValidateSAForRW(ctx, nil, nil, rwID)
		require.Error(t, err)
		s := err.Error()
		require.NotContains(t, s, "SELECT", "error should not leak SQL query")
		require.NotContains(t, s, "is_active", "error should not leak column names")
		require.NotContains(t, s, "rws", "error should not leak table names")
	})

	t.Run("invalid RW error does not leak internals", func(t *testing.T) {
		err := ValidateSAForRW(ctx, cfg, pool, "bbbbbbbb-cccc-dddd-eeee-ffffffffffff")
		require.ErrorIs(t, err, ErrRWScopeInvalid)
		s := err.Error()
		require.NotContains(t, s, "SELECT", "error should not leak SQL query")
		require.NotContains(t, s, "rws", "error should not leak table names")
	})
}

