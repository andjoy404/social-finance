package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
	req := httptest.NewRequest("GET", rwPath, nil)
	req = req.WithContext(context.WithValue(req.Context(), AuthContextKey, ac))
	rec := httptest.NewRecorder()

	h := RequireRWScope(nil, cfg)
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

	// SA without RW memberships → 403 (not authorized to access RW).
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (SA without RW memberships), got %d, body: %s", rec.Code, rec.Body.String())
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
