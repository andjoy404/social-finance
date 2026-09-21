package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SigningSecret is the HMAC secret used to sign and verify access tokens.
// Loaded from the JWT_SECRET environment variable at startup.
var SigningSecret []byte

// TokenClaims holds the payload for a short-lived access token.
//   - sub: user UUID
//   - mid: membership UUID (empty for system-only auth)
//   - rt_id: RT UUID (empty for system-only auth)
//   - role: tenant role string (e.g. "bendahara", empty for system-only)
//   - sys_role: system role (e.g. "super_admin", empty for tenant-only)
//   - exp, iat, jti: registered claims
type TokenClaims struct {
	UserID  string `json:"sub"`
	MID     string `json:"mid"`
	RTID    string `json:"rt_id"`
	Role    string `json:"role"`
	SysRole string `json:"sys_role"`
	jwt.RegisteredClaims
}

// AccessTokenOptions controls token generation parameters.
type AccessTokenOptions struct {
	Lifetime time.Duration
}

// DefaultAccessTokenLifetime is the default JWT validity period.
var DefaultAccessTokenLifetime = 15 * time.Minute

// GenerateAccessToken creates a signed JWT with the given claims. Lifetime
// defaults to 15 minutes. Returns a compact serialised token string.
func GenerateAccessToken(claims TokenClaims, opts ...AccessTokenOptions) (string, error) {
	lifetime := DefaultAccessTokenLifetime
	for _, o := range opts {
		if o.Lifetime > 0 {
			lifetime = o.Lifetime
		}
	}

	now := time.Now()
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(lifetime))
	claims.IssuedAt = jwt.NewNumericDate(now)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(SigningSecret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// ParseAndVerifyAccessToken parses and verifies a signed JWT. Returns the
// decoded claims or an error describing why verification failed
// (ExpiredToken, InvalidSignature, MalformedToken).
var (
	ErrExpiredToken     = errors.New("access token expired")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrMalformedToken   = errors.New("malformed access token")
)

const (
	ClaimUserID     = "sub"
	ClaimMembership = "mid"
	ClaimRTID       = "rt_id"
	ClaimRole       = "role"
)

func ParseAndVerifyAccessToken(raw string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return SigningSecret, nil
	})
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, ErrExpiredToken
		default:
			return nil, ErrInvalidSignature
		}
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrMalformedToken
	}
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrExpiredToken
	}
	return claims, nil
}
