package auth

import (
	"encoding/base64"
	"testing"
)

func TestGenerateRefreshTokenIsLongEnough(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() returned error: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateRefreshToken() returned empty token")
	}

	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("Generated token is not valid base64: %v", err)
	}
	if len(decoded) < RefreshTokenEntropy {
		t.Errorf("Token has too few bytes: %d, want >= %d", len(decoded), RefreshTokenEntropy)
	}
}

func TestDifferentRefreshTokensEachTime(t *testing.T) {
	t1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken 1 error: %v", err)
	}
	t2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken 2 error: %v", err)
	}
	t3, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken 3 error: %v", err)
	}
	if t1 == t2 || t2 == t3 || t1 == t3 {
		t.Error("Generated refresh tokens should be unique, got duplicates")
	}
}

func TestGenerateRefreshTokenNotPredictable(t *testing.T) {
	t1, _ := GenerateRefreshToken()
	t2, _ := GenerateRefreshToken()

	// Tokens should share very few same characters due to high entropy
	// This is a probabilistic check - if tokens were generated without
	// randomness, they would match significantly
	if t1 == t2 {
		t.Fatal("Token generation is not random")
	}

	// Tokens should not look like sequential values or weak randomness
	// by checking they don't have repeated prefixes
	if len(t1) > 5 && t1[:5] == t2[:5] {
		t.Logf("Warning: tokens share prefix %s, low entropy detected", t1[:5])
	}
}
