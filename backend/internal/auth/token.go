package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const (
	// RefreshTokenEntropy is the number of random bytes in an opaque token.
	RefreshTokenEntropy = 32

	// TokenURLSafeBase64 produces ~43 characters for 32 bytes.
)

// GenerateRefreshToken creates a cryptographically random token suitable for
// storage as a hash and comparison against the raw value presented by the client.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, RefreshTokenEntropy)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
