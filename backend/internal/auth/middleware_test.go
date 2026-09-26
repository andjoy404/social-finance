package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestRequireAuthMissingAuthorizationHeader(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	var receivedAuthContext *AuthContext
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthContext = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if receivedAuthContext != nil {
		t.Error("expected no AuthContext when no header is provided")
	}
}

func TestRequireAuthMalformedBearerFormat(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	var receivedAuthContext *AuthContext
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthContext = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	// Test: missing "Bearer " prefix
	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "NotBearer some-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for malformed scheme, got %d", rec.Code)
	}
	if receivedAuthContext != nil {
		t.Error("expected no AuthContext for malformed scheme")
	}

	// Test: empty token after Bearer
	req2 := httptest.NewRequest("GET", "/api/v1/me", nil)
	req2.Header.Set("Authorization", "Bearer ")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty token, got %d", rec2.Code)
	}

	// Test: bare token without Bearer prefix
	req3 := httptest.NewRequest("GET", "/api/v1/me", nil)
	req3.Header.Set("Authorization", "some-random-token")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bare token, got %d", rec3.Code)
	}
}

func TestRequireAuthValidTokenSetsContext(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "bendahara",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	var receivedAuthContext *AuthContext
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthContext = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if receivedAuthContext == nil {
		t.Fatal("expected AuthContext to be set for valid token")
	}
	if receivedAuthContext.UserID != "user-uuid-test" {
		t.Errorf("expected UserID=user-uuid-test, got %s", receivedAuthContext.UserID)
	}
	if receivedAuthContext.MembershipID != "member-uuid-test" {
		t.Errorf("expected MembershipID=member-uuid-test, got %s", receivedAuthContext.MembershipID)
	}
	if receivedAuthContext.RTID != "rt-uuid-test" {
		t.Errorf("expected RTID=rt-uuid-test, got %s", receivedAuthContext.RTID)
	}
	if receivedAuthContext.TenantRole != Role("bendahara") {
		t.Errorf("expected TenantRole=bendahara, got %s", receivedAuthContext.TenantRole)
	}
	if receivedAuthContext.SystemRole != "" {
		t.Errorf("expected empty SystemRole for tenant-only user, got %s", receivedAuthContext.SystemRole)
	}
}

func TestRequireAuthExpiredTokenRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "warga",
	}

	expiredClaims := claims
	expiredClaims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	tokenStr, err := token.SignedString(SigningSecret)
	if err != nil {
		t.Fatalf("Failed to sign expired token: %v", err)
	}

	var receivedAuthContext *AuthContext
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthContext = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", rec.Code)
	}
	if receivedAuthContext != nil {
		t.Error("expected no AuthContext for expired token")
	}
}

func TestRequireAuthInvalidSignatureRejected(t *testing.T) {
	SigningSecret = []byte("correct-signing-secret-value!")

	wrongKey := []byte("wrong-signing-secret-value!")
	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "warga",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(wrongKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	var receivedAuthContext *AuthContext
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthContext = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong signature, got %d", rec.Code)
	}
	if receivedAuthContext != nil {
		t.Error("expected no AuthContext for wrong signature")
	}
}

func TestRequireRolePermittedRole(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "bendahara",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	finalHandler := RequireAuth(RequireRole(RoleBendahara)(inner))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	finalHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when role is permitted (%s), got %d", claims.Role, rec.Code)
	}
}

func TestRequireRoleForbiddenRole(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "warga",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	finalHandler := RequireAuth(RequireRole(RoleBendahara)(inner))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	finalHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for forbidden role, got %d", rec.Code)
	}
}

func TestRequireRoleUnauthenticated(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	rec := httptest.NewRecorder()

	handler := RequireRole(RoleBendahara)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request with RequireRole, got %d", rec.Code)
	}
}

func TestRequireRoleMultipleRoles(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	for _, role := range []Role{RolePengurus, RoleBendahara} {
		claims := TokenClaims{
			UserID: "user-uuid-test",
			MID:    "member-uuid-test",
			RTID:   "rt-uuid-test",
			Role:   string(role),
		}
		tokenStr, err := GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("GenerateAccessToken(%s) failed: %v", role, err)
		}

		received := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received = true
			w.WriteHeader(http.StatusOK)
		})

		finalHandler := RequireAuth(RequireRole(RolePengurus, RoleBendahara)(inner))

		req := httptest.NewRequest("GET", "/api/v1/bills", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		rec := httptest.NewRecorder()
		finalHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for role %s (allowed), got %d", role, rec.Code)
		}
		if !received {
			t.Errorf("handler not reached for role %s", role)
		}
	}

	// Test non-authorized role is rejected
	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "warga",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken(warga) failed: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	finalHandler := RequireAuth(RequireRole(RolePengurus, RoleBendahara)(inner))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	finalHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for warga in [pengurus, bendahara] whitelist, got %d", rec.Code)
	}
}

func TestGetAuthContextMissing(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	ac := GetAuthContext(req)
	if ac != nil {
		t.Error("expected nil AuthContext when none is set")
	}
}

// TestRequireAuthValidTokenSucceeds checks that a valid token reaches the handler.
func TestRequireAuthValidTokenSucceeds(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "warga",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	var received bool
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 response body, got %d", rec.Code)
	}
	if !received {
		t.Error("handler was not reached for valid token")
	}
}

func TestRequireSystemRolePermitted(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID:  "user-uuid-test",
		SysRole: "super_admin",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireSystemRole(SystemRoleSuperAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/rt", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for super_admin, got %d", rec.Code)
	}
	if !received {
		t.Error("handler not reached for super_admin")
	}
}

func TestRequireSystemRoleForbiddenForTenantOnlyUser(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID:  "user-uuid-test",
		MID:     "member-uuid-test",
		RTID:    "rt-uuid-test",
		Role:    "pengurus",
		SysRole: "",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireSystemRole(SystemRoleSuperAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/rt", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for tenant-only user on system endpoint, got %d", rec.Code)
	}
	if received {
		t.Error("handler should not be reached for non-super_admin")
	}
}

func TestRequireSystemRoleUnauthenticated(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/rt", nil)
	rec := httptest.NewRecorder()

	handler := RequireSystemRole(SystemRoleSuperAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request with RequireSystemRole, got %d", rec.Code)
	}
}

// TestRequireRoleSystemOnlyAdminPasses verifies that a system-only super_admin
// (no membership) is ALLOWED through tenant RequireRole authorization.
func TestRequireRoleSystemOnlyAdminPasses(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID:  "user-uuid-test",
		SysRole: "super_admin",
		// No MID, RTID, or Role — system-only user
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireRole(RoleBendahara)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200: system-only super_admin bypasses RequireRole(bendahara), got %d", rec.Code)
	}
	if !received {
		t.Error("handler should be reached: system-only super_admin bypasses RequireRole")
	}
}

// TestRequireRoleAdminWithMembershipAsWarga verifies that a super_admin
// who also has a warga membership IS ALLOWED throughRequireRole(bendahara)
// because SystemRole takes precedence.
func TestRequireRoleAdminWithMembershipAsWarga(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID:  "user-uuid-test",
		MID:     "member-uuid-test",
		RTID:    "rt-uuid-test",
		Role:    "warga",
		SysRole: "super_admin",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireRole(RoleBendahara)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200: super_admin with warga membership passes RequireRole(bendahara) via SystemRole bypass, got %d", rec.Code)
	}
	if !received {
		t.Error("handler should be reached: super_admin bypasses tenant role matching")
	}
}

// TestRequireRoleAdminWithMembershipAsBendahara verifies that a super_admin
// who has a bendahara membership IS granted access to bendahara endpoints.
func TestRequireRoleAdminWithMembershipAsBendahara(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID:  "user-uuid-test",
		MID:     "member-uuid-test",
		RTID:    "rt-uuid-test",
		Role:    "bendahara",
		SysRole: "super_admin",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireRole(RoleBendahara)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200: super_admin with bendahara membership allowed, got %d", rec.Code)
	}
	if !received {
		t.Error("handler should be reached for super_admin with bendahara membership")
	}
}

// TestRequireSystemRoleTenantAdminDenied verifies a tenant admin (pengurus)
// with also-super-admin system role cannot use RequireSystemRole as a bendahara.
func TestRequireSystemRoleTenantAdminDeniedSystemAccess(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// Token for a pure tenant-user (pengurus), NOT super_admin
	claims := TokenClaims{
		UserID:  "user-uuid-test",
		MID:     "member-uuid-test",
		RTID:    "rt-uuid-test",
		Role:    "pengurus",
		SysRole: "",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireSystemRole(SystemRoleSuperAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/rt", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403: tenant-only user denied system endpoint, got %d", rec.Code)
	}
	if received {
		t.Error("handler should NOT be reached")
	}
}

// TestRequireSystemRoleMultiRole verifies that multiple allowed system roles
// work correctly.
func TestRequireSystemRoleMultiRole(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	for _, sysRole := range []SystemRole{SystemRoleSuperAdmin} {
		claims := TokenClaims{
			UserID:  "user-uuid-test",
			SysRole: string(sysRole),
		}

		tokenStr, err := GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("GenerateAccessToken(%s) failed: %v", sysRole, err)
		}

		received := false
		handler := RequireAuth(RequireSystemRole(SystemRoleSuperAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received = true
			w.WriteHeader(http.StatusOK)
		})))

		req := httptest.NewRequest("GET", "/api/v1/rt", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for system role %s, got %d", sysRole, rec.Code)
		}
		if !received {
			t.Errorf("handler not reached for system role %s", sysRole)
		}
	}
}

// TestSystemRoleAndTenantRoleAreDistinct verifies that a token with both a system
// role and a tenant role preserves both scopes independently.
func TestSystemRoleAndTenantRoleAreDistinct(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID:  "user-uuid-test",
		MID:     "member-uuid-test",
		RTID:    "rt-uuid-test",
		Role:    "bendahara",
		SysRole: "super_admin",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	var receivedAuthContext *AuthContext
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthContext = GetAuthContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if receivedAuthContext == nil {
		t.Fatal("expected AuthContext to be set for valid token")
	}
	if receivedAuthContext.TenantRole != Role("bendahara") {
		t.Errorf("expected TenantRole=bendahara, got %s", receivedAuthContext.TenantRole)
	}
	if receivedAuthContext.SystemRole != SystemRoleSuperAdmin {
		t.Errorf("expected SystemRole=%s, got %s", SystemRoleSuperAdmin, receivedAuthContext.SystemRole)
	}
}

// TestRequireRoleEmptyContextFields tests all three missing membership context
// fields produce 403, except when SystemRole is super_admin.
func TestRequireRoleEmptyContextFields(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	tests := []struct {
		name     string
		claim    TokenClaims
		expected int
		recv     bool
	}{
		{
			name: "empty MembershipID",
			claim: TokenClaims{
				UserID: "user-uuid",
				MID:    "",
				RTID:   "rt-uuid",
				Role:   "bendahara",
			},
			expected: http.StatusForbidden,
			recv:     false,
		},
		{
			name: "empty RTID",
			claim: TokenClaims{
				UserID: "user-uuid",
				MID:    "member-uuid",
				RTID:   "",
				Role:   "bendahara",
			},
			expected: http.StatusForbidden,
			recv:     false,
		},
		{
			name: "empty TenantRole",
			claim: TokenClaims{
				UserID:  "user-uuid",
				SysRole: "super_admin",
				MID:     "member-uuid",
				RTID:    "rt-uuid",
				Role:    "",
			},
			expected: http.StatusOK,
			recv:     true,
		},
		{
			name:     "all empty",
			claim:    TokenClaims{UserID: "user-uuid"},
			expected: http.StatusForbidden,
			recv:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenStr, err := GenerateAccessToken(tc.claim)
			if err != nil {
				t.Fatalf("GenerateAccessToken failed: %v", err)
			}

			received := false
			handler := RequireAuth(RequireRole(RoleBendahara)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received = true
				w.WriteHeader(http.StatusOK)
			})))

			req := httptest.NewRequest("GET", "/api/v1/bills", nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expected {
				t.Errorf("expected %d when %s, got %d", tc.expected, tc.name, rec.Code)
			}
			if received != tc.recv {
				t.Errorf("handler received=%v when %s, expected %v", received, tc.name, tc.recv)
			}
		})
	}
}

// TestRequireSystemRoleAndRequireRoleTogether verify that BOTH middleware can
// be stacked and each enforces its own scope.
func TestRequireSystemRoleAndRequireRoleTogether(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// A tenant-user (pengurus, NOT super_admin) hitting BOTH checks
	claims := TokenClaims{
		UserID:  "user-uuid-test",
		MID:     "member-uuid-test",
		RTID:    "rt-uuid-test",
		Role:    "pengurus",
		SysRole: "",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(
		RequireSystemRole(SystemRoleSuperAdmin)(
			RequireRole(RolePengurus)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received = true
				w.WriteHeader(http.StatusOK)
			}))),
	)

	req := httptest.NewRequest("GET", "/api/v1/rt/1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// System role check runs first (inner-most in stack) — should fail
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 (system role check first), got %d", rec.Code)
	}
	if received {
		t.Error("handler should not be reached for tenant-only user on system-role required route")
	}
}

// TestRequireRoleEmptyAllowedList tests that an empty allowed list rejects all.
func TestRequireRoleEmptyAllowedList(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "pengurus",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireRole()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 with empty allowed list, got %d", rec.Code)
	}
	if received {
		t.Error("handler should not be reached with empty role list")
	}
}
