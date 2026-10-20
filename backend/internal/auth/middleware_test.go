package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/database"
	"social-finance/internal/testutil"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var rtCounter int64

// testPool returns the shared test PostgreSQL connection pool.
func testPool(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}

// toUUID maps a human-readable label to a deterministic UUID v4.
func toUUID(s string) string {
	return testUUID(s)
}

// makeToken generates a JWT access token from the given claims using the
// SigningSecret configured for tests.
func makeToken(claims TokenClaims) string {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		panic(fmt.Sprintf("GenerateAccessToken failed: %v", err))
	}
	return tokenStr
}

// setupTestRT creates a test RT row in the database and returns its UUID.
// Uses a timestamp suffix to ensure uniqueness across test runs.
func setupTestRT(t *testing.T, pool *database.Pool, name string, rw int, rtStr string) string {
	t.Helper()
	AuthDBPool = pool
	db := pool.Raw()
	c := atomic.AddInt64(&rtCounter, 1)
	timestamp := time.Now().UnixNano()
	uniqueName := fmt.Sprintf("%s_%d_%d", name, c, timestamp)
	uniqueRT := fmt.Sprintf("%s_%d_%d", rtStr, c, timestamp)

	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
		uniqueName, rw, uniqueRT, "Test Address", "Test Head",
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test RT: %v", err)
	}
	return id
}

// makeTenantToken creates a JWT for a tenant user and upserts the corresponding
// user and membership rows so that RequireAuth identity validation succeeds.
func makeTenantToken(userID, membershipID, rtID string, role Role) string {
	realUserID := toUUID(userID)
	realMID := toUUID(membershipID)

	if AuthDBPool != nil {
		db := AuthDBPool.Raw()
		ctx := context.Background()
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
			 VALUES ($1, $2, '08123456789', 'dummyhash', $3, true)
			 ON CONFLICT (id) DO UPDATE SET is_active = true`,
			realUserID, fmt.Sprintf("%s@test.com", realUserID), "Test User "+userID,
		)
		if err != nil {
			panic(fmt.Sprintf("failed to insert test user %s: %v", userID, err))
		}
		if _, err := uuid.Parse(rtID); err == nil {
			_, err = db.ExecContext(ctx,
				`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active)
				 VALUES ($1, $2, $3, $4, true)
				 ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, is_active = true, rt_id = EXCLUDED.rt_id`,
				realMID, realUserID, rtID, string(role),
			)
			if err != nil {
				panic(fmt.Sprintf("failed to insert test membership %s: %v", membershipID, err))
			}
		}
	}

	return makeToken(TokenClaims{
		UserID: realUserID,
		MID:    realMID,
		RTID:   rtID,
		Role:   string(role),
	})
}

// makeTenantTokenWithJabatan creates a JWT for a tenant user with a jabatan
// claim, and upserts the corresponding user, membership (with jabatan), and RT rows.
func makeTenantTokenWithJabatan(userID, membershipID, rtID string, role Role, jabatan Jabatan) string {
	realUserID := toUUID(userID)
	realMID := toUUID(membershipID)

	if AuthDBPool != nil {
		db := AuthDBPool.Raw()
		ctx := context.Background()
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
			 VALUES ($1, $2, '08123456789', 'dummyhash', $3, true)
			 ON CONFLICT (id) DO UPDATE SET is_active = true`,
			realUserID, fmt.Sprintf("%s@test.com", realUserID), "Test User "+userID,
		)
		if err != nil {
			panic(fmt.Sprintf("failed to insert test user %s: %v", userID, err))
		}
		if _, err := uuid.Parse(rtID); err == nil {
			_, err = db.ExecContext(ctx,
				`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
				 VALUES ($1, $2, $3, $4, $5, true)
				 ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, jabatan = EXCLUDED.jabatan, is_active = true, rt_id = EXCLUDED.rt_id`,
				realMID, realUserID, rtID, string(role), string(jabatan),
			)
			if err != nil {
				panic(fmt.Sprintf("failed to insert test membership with jabatan %s: %v", membershipID, err))
			}
		}
	}

	return makeToken(TokenClaims{
		UserID:  realUserID,
		MID:     realMID,
		RTID:    rtID,
		Role:    string(role),
		Jabatan: string(jabatan),
	})
}

// makeSuperAdminToken creates a JWT for a system-level SUPER_ADMIN user.
func makeSuperAdminToken() string {
	realUserID := toUUID("super-admin-user-uuid")

	if AuthDBPool != nil {
		db := AuthDBPool.Raw()
		ctx := context.Background()
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, email, phone, password_hash, full_name, system_role, is_active)
			 VALUES ($1, $2, '08123456789', 'dummyhash', 'Test Super Admin', 'super_admin', true)
			 ON CONFLICT (id) DO UPDATE SET system_role = 'super_admin', is_active = true`,
			realUserID, fmt.Sprintf("%s@test.com", realUserID),
		)
		if err != nil {
			panic(fmt.Sprintf("failed to insert test super_admin user: %v", err))
		}
	}

	return makeToken(TokenClaims{
		UserID:  realUserID,
		SysRole: "super_admin",
	})
}

// makeAuthRequest creates an HTTP request with the given method, path, and Bearer token.
func makeAuthRequest(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-0@example.com", RoleBendahara)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID: userID,
		MID:    memberID,
		RTID:   rtID,
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
	if receivedAuthContext.UserID != userID {
		t.Errorf("expected UserID=%s, got %s", userID, receivedAuthContext.UserID)
	}
	if receivedAuthContext.MembershipID != memberID {
		t.Errorf("expected MembershipID=%s, got %s", memberID, receivedAuthContext.MembershipID)
	}
	if receivedAuthContext.RTID != rtID {
		t.Errorf("expected RTID=%s, got %s", rtID, receivedAuthContext.RTID)
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-2@example.com", RoleBendahara)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID: userID,
		MID:    memberID,
		RTID:   rtID,
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-3@example.com", RoleWarga)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID: userID,
		MID:    memberID,
		RTID:   rtID,
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-4@example.com", RolePengurus)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	for _, role := range []Role{RolePengurus, RoleBendahara} {
		claims := TokenClaims{
			UserID: userID,
			MID:    memberID,
			RTID:   rtID,
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
		UserID: userID,
		MID:    memberID,
		RTID:   rtID,
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
	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-1@example.com", RoleWarga)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID: userID,
		MID:    memberID,
		RTID:   rtID,
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

	setupSystemOnlyIdentity(t, "user-uuid-test", "test-13@example.com")

	userID := testUUID("user-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-5@example.com", RolePengurus)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
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

	setupSystemOnlyIdentity(t, "user-uuid-test", "test-14@example.com")

	userID := testUUID("user-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-6@example.com", RoleWarga)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-7@example.com", RoleBendahara)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
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
	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-8@example.com", RolePengurus)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
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

	setupSystemOnlyIdentity(t, "user-uuid-test", "test-15@example.com")

	userID := testUUID("user-uuid-test")

	for _, sysRole := range []SystemRole{SystemRoleSuperAdmin} {
		claims := TokenClaims{
			UserID:  userID,
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-9@example.com", RoleBendahara)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
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
		setup    func(t *testing.T) TokenClaims
		expected int
		recv     bool
	}{
		{
			name: "empty MembershipID",
			setup: func(t *testing.T) TokenClaims {
				setupSystemOnlyIdentity(t, "user-uuid", "test-12a@example.com")
				userID := testUUID("user-uuid")
				return TokenClaims{
					UserID: userID,
					MID:    "",
					RTID:   "rt-uuid",
					Role:   "bendahara",
				}
			},
			expected: http.StatusForbidden,
			recv:     false,
		},
		{
			name: "empty RTID",
			setup: func(t *testing.T) TokenClaims {
				setupTenantIdentity(t, "user-uuid", "member-uuid", "rt-uuid", "test-12b@example.com", RolePengurus)
				userID := testUUID("user-uuid")
				memberID := testUUID("member-uuid")
				return TokenClaims{
					UserID: userID,
					MID:    memberID,
					RTID:   "",
					Role:   "bendahara",
				}
			},
			expected: http.StatusForbidden,
			recv:     false,
		},
		{
			name: "empty TenantRole",
			setup: func(t *testing.T) TokenClaims {
				setupTenantIdentity(t, "user-uuid", "member-uuid", "rt-uuid", "test-12c@example.com", RolePengurus)
				userID := testUUID("user-uuid")
				memberID := testUUID("member-uuid")
				rtID := testUUID("rt-uuid")
				return TokenClaims{
					UserID:  userID,
					SysRole: "super_admin",
					MID:     memberID,
					RTID:    rtID,
					Role:    "",
				}
			},
			expected: http.StatusOK,
			recv:     true,
		},
		{
			name:     "all empty",
			setup: func(t *testing.T) TokenClaims {
				setupSystemOnlyIdentity(t, "user-uuid", "test-12d@example.com")
				userID := testUUID("user-uuid")
				return TokenClaims{
					UserID: userID,
					MID:    "",
					RTID:   "",
					Role:   "",
				}
			},
			expected: http.StatusForbidden,
			recv:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claim := tc.setup(t)
			tokenStr, err := GenerateAccessToken(claim)
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
	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-10@example.com", RolePengurus)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
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

	setupTenantIdentity(t, "user-uuid-test", "member-uuid-test", "rt-uuid-test", "test-11@example.com", RolePengurus)

	userID := testUUID("user-uuid-test")
	memberID := testUUID("member-uuid-test")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID: userID,
		MID:    memberID,
		RTID:   rtID,
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

func setupTenantIdentityWithRTCode(t *testing.T, userID, membershipID, rtID, email, rtCode string, role Role, isActive bool) {
	t.Helper()
	pool := testutil.GetTestPool(t)

	counter := atomic.AddInt64(&tenantIDCounter, 1)
	rtName := fmt.Sprintf("test-tenant-rt-%d", counter)

	uuidUser := testUUID(userID)
	uuidMember := testUUID(membershipID)
	uuidRT := testUUID(rtID)

	db := pool.Raw()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, 99, $3, 'Test Address', 'Test Head', $4)
		 ON CONFLICT (id) DO UPDATE SET is_active = $4, rt = $3`,
		uuidRT, rtName, rtCode, isActive,
	)
	if err != nil {
		t.Fatalf("insert test RT: %v", err)
	}

	_, err = db.ExecContext(context.Background(),
		`INSERT INTO users (id, email, phone, password_hash, full_name, system_role, is_active)
		 VALUES ($1, $2, NULL, 'dummy', 'Test User', NULL, true)
		 ON CONFLICT (id) DO UPDATE SET system_role = NULL, is_active = true`,
		uuidUser, email,
	)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	_, err = db.ExecContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active)
		 VALUES ($1, $2, $3, $4, true)
		 ON CONFLICT (id) DO UPDATE SET role = $4, is_active = true`,
		uuidMember, uuidUser, uuidRT, string(role),
	)
	if err != nil {
		t.Fatalf("insert test membership: %v", err)
	}
}

// TestRequireAllowedRT verifies that only allowed RTs (002 and 003) can access operational routes.
func TestRequireAllowedRT(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	pool := testutil.GetTestPool(t)
	oldPool := AuthDBPool
	AuthDBPool = pool
	t.Cleanup(func() {
		AuthDBPool = oldPool
	})

	// 1. RT 002 allowed
	setupTenantIdentityWithRTCode(t, "user-rt002", "member-rt002", "rt-uuid-002", "test-rt002@example.com", "002", RoleBendahara, true)
	claims002 := TokenClaims{
		UserID: testUUID("user-rt002"),
		MID:    testUUID("member-rt002"),
		RTID:   testUUID("rt-uuid-002"),
		Role:   "bendahara",
	}
	token002, err := GenerateAccessToken(claims002)
	if err != nil {
		t.Fatalf("token002 err: %v", err)
	}

	handler := RequireAuth(RequireAllowedRT(pool)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+token002)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for RT 002, got %d", rec.Code)
	}

	// 2. RT 003 allowed
	setupTenantIdentityWithRTCode(t, "user-rt003", "member-rt003", "rt-uuid-003", "test-rt003@example.com", "003", RoleBendahara, true)
	claims003 := TokenClaims{
		UserID: testUUID("user-rt003"),
		MID:    testUUID("member-rt003"),
		RTID:   testUUID("rt-uuid-003"),
		Role:   "bendahara",
	}
	token003, err := GenerateAccessToken(claims003)
	if err != nil {
		t.Fatalf("token003 err: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+token003)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for RT 003, got %d", rec.Code)
	}

	// 3. RT 004 NOT allowed -> 403
	setupTenantIdentityWithRTCode(t, "user-rt004", "member-rt004", "rt-uuid-004", "test-rt004@example.com", "004", RoleBendahara, true)
	claims004 := TokenClaims{
		UserID: testUUID("user-rt004"),
		MID:    testUUID("member-rt004"),
		RTID:   testUUID("rt-uuid-004"),
		Role:   "bendahara",
	}
	token004, err := GenerateAccessToken(claims004)
	if err != nil {
		t.Fatalf("token004 err: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+token004)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for RT 004, got %d", rec.Code)
	}

	// 4. System-only super admin with no RTID passes through
	setupSystemOnlyIdentity(t, "user-superadmin-rt", "test-superadmin-rt@example.com")
	claimsSA := TokenClaims{
		UserID:  testUUID("user-superadmin-rt"),
		SysRole: "super_admin",
	}
	tokenSA, err := GenerateAccessToken(claimsSA)
	if err != nil {
		t.Fatalf("tokenSA err: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenSA)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for system-only super admin, got %d", rec.Code)
	}
}

// TestRequireJabatanKetuaPasses verifies that a Ketua RT user passes RequireJabatan(JabatanKetua).
func TestRequireJabatanKetuaPasses(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	setupTenantIdentity(t, "user-uuid-ketua", "member-uuid-ketua", "rt-uuid-test", "ketua@example.com", RoleWarga)

	userID := testUUID("user-uuid-ketua")
	memberID := testUUID("member-uuid-ketua")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
		Role:    "warga",
		Jabatan: "ketua",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireJabatan(JabatanKetua)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/warga/import/preview", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for jabatan=ketua, got %d", rec.Code)
	}
	if !received {
		t.Error("handler should be reached for jabatan=ketua")
	}
}

// TestRequireJabatanSekretarisRejected verifies that a non-allowed jabatan is rejected.
func TestRequireJabatanSekretarisRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	setupTenantIdentity(t, "user-uuid-sek", "member-uuid-sek", "rt-uuid-test", "sekretaris@example.com", RoleWarga)

	userID := testUUID("user-uuid-sek")
	memberID := testUUID("member-uuid-sek")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
		Role:    "warga",
		Jabatan: "sekretaris",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireJabatan(JabatanKetua)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/warga/import/preview", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for jabatan=sekretaris, got %d", rec.Code)
	}
	if received {
		t.Error("handler should NOT be reached for non-allowed jabatan")
	}
}

// TestRequireJabatanNoJabatanPasses verifies that a user without jabatan passes through.
func TestRequireJabatanNoJabatanPasses(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	setupTenantIdentity(t, "user-uuid-pg", "member-uuid-pg", "rt-uuid-test", "pengurus@example.com", RolePengurus)

	userID := testUUID("user-uuid-pg")
	memberID := testUUID("member-uuid-pg")
	rtID := testUUID("rt-uuid-test")

	claims := TokenClaims{
		UserID:  userID,
		MID:     memberID,
		RTID:    rtID,
		Role:    "pengurus",
		Jabatan: "",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireJabatan(JabatanKetua)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/warga/import/preview", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for no jabatan, got %d", rec.Code)
	}
	if !received {
		t.Error("handler should be reached for user without jabatan")
	}
}

// TestRequireJabatanSystemAdminPasses verifies super admin bypasses RequireJabatan.
func TestRequireJabatanSystemAdminPasses(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	setupSystemOnlyIdentity(t, "user-uuid-sa", "sa@example.com")

	userID := testUUID("user-uuid-sa")

	claims := TokenClaims{
		UserID:  userID,
		SysRole: "super_admin",
		Jabatan: "keamanan", // arbitrary jabatan — should not matter
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	received := false
	handler := RequireAuth(RequireJabatan(JabatanKetua)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/warga/import/preview", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for super admin, got %d", rec.Code)
	}
	if !received {
		t.Error("handler should be reached for super admin")
	}
}

// TestRequireJabatanUnauthenticated verifies unauthenticated requests get 401.
func TestRequireJabatanUnauthenticated(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/warga/import/preview", nil)
	rec := httptest.NewRecorder()

	handler := RequireJabatan(JabatanKetua)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request with RequireJabatan, got %d", rec.Code)
	}
}

// TestRequireAccessRolePengurusPasses verifies role-based Pengurus passes RequireAccess.
func TestRequireAccessRolePengurusPasses(t *testing.T) {
	rtID := setupTestRT(t, testPool(t), "RAAcc-P", 1, "RACCP")
	pToken := makeTenantToken("user-p-a", "mid-p-a", rtID, RolePengurus)
	req := makeAuthRequest(http.MethodGet, "/test", pToken)

	rec := httptest.NewRecorder()
	handler := RequireAuth(RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for role-based Pengurus, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAccessRoleBendaharaPasses verifies role-based Bendahara passes RequireAccess.
func TestRequireAccessRoleBendaharaPasses(t *testing.T) {
	rtID := setupTestRT(t, testPool(t), "RAAcc-B", 1, "RACCB")
	bToken := makeTenantToken("user-b-a", "mid-b-a", rtID, RoleBendahara)
	req := makeAuthRequest(http.MethodGet, "/test", bToken)

	rec := httptest.NewRecorder()
	handler := RequireAuth(RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for role-based Bendahara, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAccessJabatanKetuaPasses verifies jabatan-based Ketua with role="warga" passes.
func TestRequireAccessJabatanKetuaPasses(t *testing.T) {
	rtID := setupTestRT(t, testPool(t), "RAAcc-K", 1, "RACKK")
	kToken := makeTenantTokenWithJabatan("user-k-a", "mid-k-a", rtID, RoleWarga, JabatanKetua)
	req := makeAuthRequest(http.MethodGet, "/test", kToken)

	rec := httptest.NewRecorder()
	handler := RequireAuth(RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for jabatan-based Ketua with role=warga, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAccessJabatanBendaharaPasses verifies jabatan-based Bendahara passes.
func TestRequireAccessJabatanBendaharaPasses(t *testing.T) {
	rtID := setupTestRT(t, testPool(t), "RAAcc-JB", 1, "RACJB")
	jbToken := makeTenantTokenWithJabatan("user-jb-a", "mid-jb-a", rtID, RolePengurus, JabatanBendahara)
	req := makeAuthRequest(http.MethodGet, "/test", jbToken)

	rec := httptest.NewRecorder()
	handler := RequireAuth(RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for jabatan-based Bendahara, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAccessRoleWargaBlocked verifies role-based Warga is denied.
func TestRequireAccessRoleWargaBlocked(t *testing.T) {
	rtID := setupTestRT(t, testPool(t), "RAAcc-W", 1, "RACWW")
	wToken := makeTenantToken("user-w-a", "mid-w-a", rtID, RoleWarga)
	req := makeAuthRequest(http.MethodGet, "/test", wToken)

	rec := httptest.NewRecorder()
	handler := RequireAuth(RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for role-based Warga, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAccessJabatanSekretarisBlocked verifies jabatan-based Sekretaris without matching role is denied.
func TestRequireAccessJabatanSekretarisBlocked(t *testing.T) {
	rtID := setupTestRT(t, testPool(t), "RAAcc-S", 1, "RACSS")
	sToken := makeTenantTokenWithJabatan("user-s-a", "mid-s-a", rtID, RoleWarga, JabatanSekretaris)
	req := makeAuthRequest(http.MethodGet, "/test", sToken)

	rec := httptest.NewRecorder()
	handler := RequireAuth(RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for jabatan-based Sekretaris with role=warga, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAccessSystemAdminPasses verifies SUPER_ADMIN bypasses RequireAccess.
func TestRequireAccessSystemAdminPasses(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	setupSystemOnlyIdentity(t, "user-uuid-sa", "sa-test@example.com")
	saToken := makeSuperAdminToken()
	req := makeAuthRequest(http.MethodGet, "/test", saToken)

	rec := httptest.NewRecorder()
	handler := RequireAuth(RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for SUPER_ADMIN, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAccessUnauthenticated verifies unauthenticated requests are rejected.
func TestRequireAccessUnauthenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	rec := httptest.NewRecorder()
	handler := RequireAccess(
		[]Role{RolePengurus, RoleBendahara},
		[]Jabatan{JabatanKetua, JabatanBendahara},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request with RequireAccess, got %d", rec.Code)
	}
}

// setupKetuaRTTestIdentity creates an RT with an allowed code ("002"), a user,
// and an active membership with role="warga" and jabatan="ketua". It returns
// the JWT token and the RT ID. The RT code "002" is in AllowedRTCodes so
// RequireAllowedRT passes.
func setupKetuaRTTestIdentity(t *testing.T, userID, membershipID, rtID string) (string, string) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	realUserID := testUUID(userID)
	realMID := testUUID(membershipID)

	// Reuse an existing active RT with an allowed code ("002") so we do not
	// conflict with the uniq_rts_active constraint left by prior test runs.
	// Insert one only if none exists.
	var realRTID string
	err := db.QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '002' AND is_active = true LIMIT 1`).Scan(&realRTID)
	if err != nil {
		realRTID = testUUID(rtID)
		_, err = db.ExecContext(ctx, `
			INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
			VALUES ($1, $2, 16, '002', 'Test Address', 'Test Head', true)
		`, realRTID)
		if err != nil {
			t.Fatalf("insert test RT: %v", err)
		}
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		VALUES ($1, $2, '08123456789', 'dummyhash', 'Test Ketua', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, realUserID, fmt.Sprintf("%s@test.com", userID))
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, jabatan = EXCLUDED.jabatan, is_active = true, rt_id = EXCLUDED.rt_id
	`, realMID, realUserID, realRTID, string(RoleWarga), string(JabatanKetua))
	if err != nil {
		t.Fatalf("insert test membership: %v", err)
	}

	token := makeToken(TokenClaims{
		UserID:  realUserID,
		MID:     realMID,
		RTID:    realRTID,
		Role:    string(RoleWarga),
		Jabatan: string(JabatanKetua),
	})
	return token, realRTID
}

// readChain composes the exact middleware chain used by GET /api/v1/residents,
// /api/v1/bills, and /api/v1/transactions so tests exercise the real protection.
func readChain(pool *database.Pool, permission string, inner http.Handler) http.Handler {
	return RequireAuth(RequireAllowedRT(pool)(RequirePermission(pool, permission)(RequireRole(RoleWarga, RoleBendahara, RolePengurus)(inner))))
}

func TestKetuaRTReadAccessChain(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	pool := testPool(t)
	oldPool := AuthDBPool
	AuthDBPool = pool
	t.Cleanup(func() { AuthDBPool = oldPool })

	token, _ := setupKetuaRTTestIdentity(t, "user-krt-chain", "member-krt-chain", "rt-krt-chain")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Ketua RT reads residents (warga.read)", func(t *testing.T) {
		handler := readChain(pool, "warga.read", inner)
		req := makeAuthRequest(http.MethodGet, "/api/v1/residents", token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for Ketua RT reading residents, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Ketua RT reads bills (finance.read)", func(t *testing.T) {
		handler := readChain(pool, "finance.read", inner)
		req := makeAuthRequest(http.MethodGet, "/api/v1/bills", token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for Ketua RT reading bills, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Ketua RT reads transactions (finance.read)", func(t *testing.T) {
		handler := readChain(pool, "finance.read", inner)
		req := makeAuthRequest(http.MethodGet, "/api/v1/transactions", token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for Ketua RT reading transactions, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestKetuaRTWithoutRequiredPermissionDenied(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	pool := testPool(t)
	oldPool := AuthDBPool
	AuthDBPool = pool
	t.Cleanup(func() { AuthDBPool = oldPool })

	token, _ := setupKetuaRTTestIdentity(t, "user-krt-noperm", "member-krt-noperm", "rt-krt-noperm")

	// Remove the ketua -> warga.read position permission so the permission
	// check fails, then restore it afterwards.
	ctx := context.Background()
	_, err := pool.Raw().ExecContext(ctx, `
		DELETE FROM position_permissions pp
		USING permission_types pt
		WHERE pp.permission_id = pt.id AND pp.position = $1 AND pt.code = $2
	`, string(JabatanKetua), "warga.read")
	if err != nil {
		t.Fatalf("delete position permission: %v", err)
	}
	t.Cleanup(func() {
		pool.Raw().ExecContext(context.Background(), `
			INSERT INTO position_permissions (position, permission_id)
			SELECT $1, id FROM permission_types WHERE code = $2
			ON CONFLICT (position, permission_id) DO NOTHING
		`, string(JabatanKetua), "warga.read")
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := readChain(pool, "warga.read", inner)

	req := makeAuthRequest(http.MethodGet, "/api/v1/residents", token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for Ketua RT without warga.read permission, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReadChainDisallowedTenantRole(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	pool := testPool(t)
	oldPool := AuthDBPool
	AuthDBPool = pool
	t.Cleanup(func() { AuthDBPool = oldPool })

	// Reuse an existing Ketua RT identity (membership role=warga is DB-valid;
	// the JWT tenant role is what RequireRole evaluates).
	token, rtID := setupKetuaRTTestIdentity(t, "user-krt-norole", "member-krt-norole", "rt-krt-norole")

	// Override the JWT tenant role to "keamanan" — not in the read whitelist.
	token = makeToken(TokenClaims{
		UserID:  testUUID("user-krt-norole"),
		MID:     testUUID("member-krt-norole"),
		RTID:    rtID,
		Role:    "keamanan", // not in {warga, bendahara, pengurus}
		Jabatan: string(JabatanKetua),
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := readChain(pool, "warga.read", inner)

	req := makeAuthRequest(http.MethodGet, "/api/v1/residents", token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed tenant role keamanan, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSuperAdminReadAccessNoRegression(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	pool := testPool(t)
	oldPool := AuthDBPool
	AuthDBPool = pool
	t.Cleanup(func() { AuthDBPool = oldPool })

	setupSystemOnlyIdentity(t, "user-sa-read", "sa-read@example.com")
	token := makeToken(TokenClaims{
		UserID:  testUUID("user-sa-read"),
		SysRole: "super_admin",
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, permission := range []string{"warga.read", "finance.read"} {
		handler := readChain(pool, permission, inner)
		for _, path := range []string{"/api/v1/residents", "/api/v1/bills", "/api/v1/transactions"} {
			req := makeAuthRequest(http.MethodGet, path, token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 for SUPER_ADMIN on %s (%s), got %d: %s", path, permission, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestUnauthenticatedReadAccessDenied(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	pool := testutil.GetTestPool(t)
	handler := readChain(pool, "warga.read", inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/residents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request, got %d", rec.Code)
	}
}
