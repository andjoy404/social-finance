package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// JWT tampering: modifying claims without valid re-signing must be rejected.
// ---------------------------------------------------------------------------

func TestJWTRoleTamperingRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// Step 1: generate a valid warga token
	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatal("expected 3 JWT parts")
	}

	// Step 2: decode the payload, modify role claim
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	// Replace "warga" with "bendahara" in the JSON payload.
	tampered := strings.Replace(string(payloadBytes), `"warga"`, `"bendahara"`, 1)

	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(tampered))
	// Reconstruct token WITHOUT re-signing (keep original signature).
	tamperedToken := parts[0] + "." + tamperedPayload + "." + parts[2]

	_, err = ParseAndVerifyAccessToken(tamperedToken)
	if err == nil {
		t.Fatal("expected error: role-tampered JWT must be rejected by HMAC verification")
	}
}

func TestJWTIDTamperingRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	parts := strings.Split(tokenStr, ".")

	// Decode and modify RTID (tenant-switch attempt).
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	tampered := strings.Replace(string(payloadBytes), `"rt-uuid-1"`, `"rt-uuid-2"`, 1)
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(tampered))

	tamperedToken := parts[0] + "." + tamperedPayload + "." + parts[2]

	_, err = ParseAndVerifyAccessToken(tamperedToken)
	if err == nil {
		t.Fatal("expected error: RTID-tampered JWT must be rejected by HMAC verification")
	}
}

func TestJWTSystemRoleTamperingRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// Start: warga with no system role.
	claims := TokenClaims{
		UserID:  "user-uuid-1",
		MID:     "member-uuid-1",
		RTID:    "rt-uuid-1",
		Role:    "warga",
		SysRole: "",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	parts := strings.Split(tokenStr, ".")

	// Decode and inject sys_role=super_admin.
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	tampered := strings.Replace(string(payloadBytes), `"sys_role":""`, `"sys_role":"super_admin"`, 1)
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(tampered))

	tamperedToken := parts[0] + "." + tamperedPayload + "." + parts[2]

	_, err = ParseAndVerifyAccessToken(tamperedToken)
	if err == nil {
		t.Fatal("expected error: system role tampered JWT must be rejected")
	}
}

func TestJWTMembershipIDTamperingRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "bendahara",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	parts := strings.Split(tokenStr, ".")

	// Decode and modify membership ID.
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	tampered := strings.Replace(string(payloadBytes), `"member-uuid-1"`, `"member-uuid-2"`, 1)
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(tampered))

	tamperedToken := parts[0] + "." + tamperedPayload + "." + parts[2]

	_, err = ParseAndVerifyAccessToken(tamperedToken)
	if err == nil {
		t.Fatal("expected error: MID-tampered JWT must be rejected by HMAC verification")
	}
}

func TestJWTUserIDTamperingRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-b",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "bendahara",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	parts := strings.Split(tokenStr, ".")

	// Decode and modify user ID.
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
	tampered := strings.Replace(string(payloadBytes), `"user-b"`, `"user-a"`, 1)
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(tampered))

	tamperedToken := parts[0] + "." + tamperedPayload + "." + parts[2]

	_, err = ParseAndVerifyAccessToken(tamperedToken)
	if err == nil {
		t.Fatal("expected error: UserID-tampered JWT must be rejected by HMAC verification")
	}
}

func TestJWTMalformedPayloadRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	tokenStr, err := GenerateAccessToken(TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	parts := strings.Split(tokenStr, ".")

	// Replace payload with random bytes.
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte("garbage-payload"))
	badToken := parts[0] + "." + parts[1] + "." + parts[2]

	_, err = ParseAndVerifyAccessToken(badToken)
	if err == nil {
		t.Fatal("expected error for malformed JWT")
	}
}

// ---------------------------------------------------------------------------
// Tenant isolation: authenticated user cannot access another RT's resources
// via client-supplied identifiers.
// ---------------------------------------------------------------------------

func TestRequireRoleNeverReadsClientRTID(t *testing.T) {
	// The RequireRole middleware ONLY reads AuthContext, which was populated
	// solely from the JWT.  It does NOT read URL path params, query strings,
	// or JSON body.
	//
	// To prove: even if a client sends rt_id in path/query/body on a test
	// endpoint, RequireRole ignores it entirely.  The AuthContext.RTID comes
	// from the JWT — a cryptographically signed field the client cannot
	// forge.

	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// Authenticated principal: user in RT A with bendahara role.
	claims := TokenClaims{
		UserID: "user-uuid-a",
		MID:    "member-uuid-a",
		RTID:   "rt-a-uuid",
		Role:   "bendahara",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	// Simulate client sending a different RT in various places.
	testCases := []struct {
		name   string
		path   string
		query  string
		header map[string]string
	}{
		{
			name: "path-param RT override",
			path: "/api/v1/rt-uuid-a/transactions",
		},
		{
			name: "query-param RT override",
			path: "/api/v1/transactions?rt_id=rt-b-uuid",
		},
		{
			name:   "header RT override",
			path:   "/api/v1/transactions",
			header: map[string]string{"X-RT-ID": "some-other-rt-uuid"},
		},
		{
			name:  "both path and query RT override",
			path:  "/api/v1/transactions",
			query: "?rt_id=fake-rt&membership_id=fake-mid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path+tc.query, nil)
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			req.Header.Set("Authorization", "Bearer "+tokenStr)

			receivedRTID := ""
			receivedMID := ""
			handler := RequireAuth(RequireRole(RoleBendahara)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ac := GetAuthContext(r)
				if ac != nil {
					receivedRTID = ac.RTID
					receivedMID = ac.MembershipID
				}
				w.WriteHeader(http.StatusOK)
			})))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
			// The RTID must come from the JWT, never from client request.
			if receivedRTID != "rt-a-uuid" {
				t.Errorf("expected RTID=rt-a-uuid (from JWT), got %s", receivedRTID)
			}
			if receivedMID != "member-uuid-a" {
				t.Errorf("expected MID=member-uuid-a (from JWT), got %s", receivedMID)
			}
		})
	}
}

func TestAuthContextTenantFieldsAreIsolatedFromRequest(t *testing.T) {
	// Even if a handler reads query params or body for rt_id, AuthContext
	// values are derived solely from the JWT token.
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-x",
		MID:    "member-uuid-x",
		RTID:   "actual-rt-uuid",
		Role:   "warga",
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/data?rt_id=fake-rt-uuid&membership_id=fake-mid", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	var acRTID, acMID, acUserID string

	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := GetAuthContext(r)
		if ac != nil {
			acRTID = ac.RTID
			acMID = ac.MembershipID
			acUserID = ac.UserID
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if acRTID != "actual-rt-uuid" {
		t.Errorf("AuthContext.RTID was overridden by query param: expected actual-rt-uuid, got %s", acRTID)
	}
	if acMID != "member-uuid-x" {
		t.Errorf("AuthContext.MembershipID was overridden by query param: expected member-uuid-x, got %s", acMID)
	}
	if acUserID != "user-uuid-x" {
		t.Errorf("AuthContext.UserID was overridden by query param: expected user-uuid-x, got %s", acUserID)
	}
}

func TestCrossRTTenantIsolation(t *testing.T) {
	// User A is in RT A.  User B is in RT B.
	// If User A tries to access RT B's resources by modifying their JWT
	// (tampering), HMAC verification catches the forgery (tested above).
	// This test confirms RequireRole correctly evaluates the AuthContext
	// which only contains the legitimate RT from verified JWT.

	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	rtABendaharaClaims := TokenClaims{
		UserID: "user-a-uuid",
		MID:    "member-a-uuid",
		RTID:   "rt-a-uuid",
		Role:   "bendahara", // RT A: bendahara
	}
	rtAWargaClaims := TokenClaims{
		UserID: "user-a-uuid",
		MID:    "member-a-uuid",
		RTID:   "rt-a-uuid",
		Role:   "warga", // RT A: warga
	}

	tokenBendahara, err := GenerateAccessToken(rtABendaharaClaims)
	if err != nil {
		t.Fatalf("generate bendahara token: %v", err)
	}
	tokenWarga, err := GenerateAccessToken(rtAWargaClaims)
	if err != nil {
		t.Fatalf("generate warga token: %v", err)
	}

	handler := RequireAuth(RequireRole(RoleBendahara)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	// Bentahara token → 200.
	reqAuth := httptest.NewRequest("GET", "/api/v1/bills", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+tokenBendahara)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, reqAuth)
	if rec.Code != http.StatusOK {
		t.Errorf("bendahara: expected 200, got %d", rec.Code)
	}

	// Warga token → 403 (same RT, wrong tenant role).
	reqWarga := httptest.NewRequest("GET", "/api/v1/bills", nil)
	reqWarga.Header.Set("Authorization", "Bearer "+tokenWarga)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, reqWarga)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("warga: expected 403, got %d", rec2.Code)
	}
}

// ---------------------------------------------------------------------------
// Role tampering with wrong signing key (attacker does not know secret).
// ---------------------------------------------------------------------------

func TestTamperingWithWrongKeyRejectsAccess(t *testing.T) {
	SigningSecret = []byte("correct-signing-key-value-12345")

	// Attacker crafts a token with higher role using a DIFFERENT key.
	attackerKey := []byte("wrong-signing-key-value-9999")
	claims := TokenClaims{
		UserID:  "attacker-user",
		MID:     "fake-member-id",
		RTID:    "fake-rt-id",
		Role:    "super_admin", // attacker tries to escalate
		SysRole: "super_admin",
	}
	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(attackerKey)
	if err != nil {
		t.Fatalf("sign attacker token: %v", err)
	}

	_, err = ParseAndVerifyAccessToken(tokenStr)
	if err == nil {
		t.Fatal("token signed with wrong key must be rejected")
	}
}

func TestJWTNoneAlgorithmRejected(t *testing.T) {
	// Attacker attempts to use alg=none to create an unsigned token.
	// The HMAC verifier in ParseAndVerifyAccessToken checks that the signing
	// method is HMAC and rejects any other (including alg=none).
	//
	// We craft the token manually since jwt.NewWithClaims(jwt.SigningMethodNone,
	// ...) is blocked by the library itself.

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"attacker-user","mid":"fake-member-id","rt_id":"fake-rt-id","role":"bendahara","sys_role":"super_admin","exp":9999999999}`))
	// alg=none: empty signature, just the period.
	noneToken := headerB64 + "." + payloadB64 + "."

	_, err := ParseAndVerifyAccessToken(noneToken)
	if err == nil {
		t.Fatal("alg=none token must be rejected by HMAC verifier")
	}
}

// ---------------------------------------------------------------------------
// Membership validity: documented trade-offs.
// ---------------------------------------------------------------------------

// TestMembershipValidityAccessDeniedDuringRefresh documents the trade-off
// where an issued access token remains valid until expiry even if the
// user/membership/RT becomes inactive after token issuance.
//
// Login and Refresh endpoints re-validate against the database:
// - Login (service.go:82-84): rejects inactive users
// - Login (service.go:109-111): rejects inactive memberships
// - Login (service.go:112-118): rejects inactive RTs
// - Refresh (service.go:202): rejects revoked/expired refresh tokens
// - Refresh (service.go:213): loads membership (is_active checked in SQL)
//
// Access tokens (~15 min) contain immutable claims. Once issued, they
// cannot be invalidated without a global blacklist (not implemented in P2).
// The ~15 min window is an acceptable trade-off:
// - Login must reject (re-authenticates on each login)
// - Refresh must reject (refresh flow re-checks DB)
// - Read-only endpoints tolerate the ~15 min stale window
func TestMembershipValidityAccessTokenLifetime(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-test",
		MID:    "member-uuid-test",
		RTID:   "rt-uuid-test",
		Role:   "warga",
	}

	// Expired tokens are always rejected.
	expiredClaims := claims
	expiredClaims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	str, _ := token.SignedString(SigningSecret)

	_, err := ParseAndVerifyAccessToken(str)
	if err == nil {
		t.Fatal("expected expired token to be rejected")
	}
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}

	// Token with far-future expiry is valid (until real expiry).
	validClaims := claims
	validClaims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	}
	token2 := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)
	str2, _ := token2.SignedString(SigningSecret)

	_, err = ParseAndVerifyAccessToken(str2)
	if err != nil {
		t.Fatalf("expected valid token to pass, got: %v", err)
	}
}
