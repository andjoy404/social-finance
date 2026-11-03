package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndVerifyToken(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "bendahara",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken() returned error: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("GenerateAccessToken() returned empty token")
	}

	parsed, err := ParseAndVerifyAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ParseAndVerifyAccessToken() returned error: %v", err)
	}

	if parsed.UserID != "user-uuid-1" {
		t.Errorf("expected UserID=user-uuid-1, got %s", parsed.UserID)
	}
	if parsed.MID != "member-uuid-1" {
		t.Errorf("expected MID=member-uuid-1, got %s", parsed.MID)
	}
	if parsed.RTID != "rt-uuid-1" {
		t.Errorf("expected RTID=rt-uuid-1, got %s", parsed.RTID)
	}
	if parsed.Role != "bendahara" {
		t.Errorf("expected Role=bendahara, got %s", parsed.Role)
	}
}

func TestTokenHasExpiry(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken() returned error: %v", err)
	}

	parsed, err := ParseAndVerifyAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ParseAndVerifyAccessToken() returned error: %v", err)
	}

	if parsed.ExpiresAt == nil {
		t.Fatal("Expected token to have ExpiresAt")
	}

	// ExpiresAt should be ~15 minutes from now
	if parsed.ExpiresAt.Time.Before(time.Now()) {
		t.Fatal("ExpiredAt is in the past")
	}

	// Should be within 16 minutes of now (allowing for test timing variance)
	if parsed.ExpiresAt.Time.Sub(time.Now()) > 16*time.Minute {
		t.Fatalf("Expected expiry ~15min, got %v", parsed.ExpiresAt.Time.Sub(time.Now()))
	}
}

func TestTokenExpiredIsRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// Create an expired token manually by setting registered claims
	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(SigningSecret)
	if err != nil {
		t.Fatalf("Failed to sign expired token: %v", err)
	}

	_, err = ParseAndVerifyAccessToken(tokenStr)
	if err == nil {
		t.Fatal("Expected expired token to be rejected")
	}
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got %v", err)
	}
}

func TestInvalidSignatureRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// Create a token signed with a different key
	wrongKey := []byte("different-signing-secret-key!!")
	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(wrongKey)
	if err != nil {
		t.Fatalf("Failed to sign token with wrong key: %v", err)
	}

	_, err = ParseAndVerifyAccessToken(tokenStr)
	if err == nil {
		t.Fatal("Expected invalid signature to be rejected")
	}
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got %v", err)
	}
}

func TestMalformedTokenRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	_, err := ParseAndVerifyAccessToken("not-a-valid-jwt")
	if err == nil {
		t.Fatal("Expected malformed token to be rejected")
	}
}

func TestEmptyTokenRejected(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	_, err := ParseAndVerifyAccessToken("")
	if err == nil {
		t.Fatal("Expected empty token to be rejected")
	}
}

func TestDifferentRolesPreserved(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	roles := []string{"super_admin", "pengurus", "bendahara", "warga"}
	for _, role := range roles {
		claims := TokenClaims{
			UserID: "user-uuid-1",
			MID:    "member-uuid-1",
			RTID:   "rt-uuid-1",
			Role:   role,
		}

		tokenStr, err := GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("GenerateAccessToken() for role %s returned error: %v", role, err)
		}

		parsed, err := ParseAndVerifyAccessToken(tokenStr)
		if err != nil {
			t.Fatalf("ParseAndVerifyAccessToken() for role %s returned error: %v", role, err)
		}

		if parsed.Role != role {
			t.Errorf("Expected role %s, got %s", role, parsed.Role)
		}
	}
}

func TestCustomLifetime(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
	}

	tokenStr, err := GenerateAccessToken(claims, AccessTokenOptions{
		Lifetime: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken() with custom lifetime returned error: %v", err)
	}

	parsed, err := ParseAndVerifyAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ParseAndVerifyAccessToken() returned error: %v", err)
	}

	duration := parsed.ExpiresAt.Time.Sub(time.Now())
	if duration > 5*time.Minute+10*time.Second || duration < 5*time.Minute-10*time.Second {
		t.Errorf("Expected ~5min lifetime, got %v", duration)
	}
}

func TestTokenWithRWContextIsParsed(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	memberships := []RWMembership{
		{RWID: "rw-uuid-1", Jabatan: "ketua"},
		{RWID: "rw-uuid-2", Jabatan: "bendahara"},
	}
	rawCtx, err := json.Marshal(memberships)
	if err != nil {
		t.Fatalf("marshal RWContext: %v", err)
	}

	claims := TokenClaims{
		UserID:    "user-uuid-1",
		MID:       "member-uuid-1",
		RTID:      "rt-uuid-1",
		Role:      "bendahara",
		Jabatan:   "ketua",
		RWContext: string(rawCtx),
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken() returned error: %v", err)
	}

	parsed, err := ParseAndVerifyAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ParseAndVerifyAccessToken() returned error: %v", err)
	}

	if parsed.RWContext == "" {
		t.Fatal("expected non-empty RWContext")
	}

	var parsedMemberships []RWMembership
	if err := json.Unmarshal([]byte(parsed.RWContext), &parsedMemberships); err != nil {
		t.Fatalf("unmarshal RWContext: %v", err)
	}
	if len(parsedMemberships) != 2 {
		t.Fatalf("expected 2 RW memberships, got %d", len(parsedMemberships))
	}
	if parsedMemberships[0].RWID != "rw-uuid-1" {
		t.Errorf("expected RWID rw-uuid-1, got %s", parsedMemberships[0].RWID)
	}
	if parsedMemberships[0].Jabatan != "ketua" {
		t.Errorf("expected jabatan ketua, got %s", parsedMemberships[0].Jabatan)
	}
	if parsedMemberships[1].RWID != "rw-uuid-2" {
		t.Errorf("expected RWID rw-uuid-2, got %s", parsedMemberships[1].RWID)
	}
	if parsedMemberships[1].Jabatan != "bendahara" {
		t.Errorf("expected jabatan bendahara, got %s", parsedMemberships[1].Jabatan)
	}
}

func TestTokenWithoutRWContextIsBackwardCompatible(t *testing.T) {
	SigningSecret = []byte("test-signing-secret-at-least-16-chars")

	// Token generated by old client (no RWContext field at all).
	claims := TokenClaims{
		UserID: "user-uuid-1",
		MID:    "member-uuid-1",
		RTID:   "rt-uuid-1",
		Role:   "warga",
		// RWContext intentionally omitted — should be empty string.
	}

	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken() returned error: %v", err)
	}

	parsed, err := ParseAndVerifyAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ParseAndVerifyAccessToken() returned error: %v", err)
	}

	if parsed.RWContext != "" {
		t.Errorf("expected empty RWContext for backward compat, got %q", parsed.RWContext)
	}

	// Other claims must still parse correctly.
	if parsed.UserID != "user-uuid-1" {
		t.Errorf("expected UserID=user-uuid-1, got %s", parsed.UserID)
	}
	if parsed.Role != "warga" {
		t.Errorf("expected Role=warga, got %s", parsed.Role)
	}
}

func TestBuildRWContextJSON(t *testing.T) {
	// Empty/nil slice → empty string.
	if got := buildRWContextJSON(nil); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
	if got := buildRWContextJSON([]RWMembership{}); got != "" {
		t.Errorf("expected empty string for empty slice, got %q", got)
	}

	// Single membership.
	single := []RWMembership{{RWID: "abc-123", Jabatan: "ketua"}}
	got := buildRWContextJSON(single)
	var out []RWMembership
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 1 || out[0].RWID != "abc-123" || out[0].Jabatan != "ketua" {
		t.Errorf("unexpected result: %+v", out)
	}
}
