package auth

import (
	"fmt"

	"github.com/alexedwards/argon2id"
)

// ErrInvalidPassword is returned when a password does not match the stored hash.
var ErrInvalidPassword = fmt.Errorf("invalid password")

// Password policy: minimum 12, maximum 128 characters.
// No complexity requirements -- length is the primary defense.
const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
)

// Hash encrypts password and returns a self-describing Argon2id encoded hash.
// The hash contains version, memory, iterations, parallelism, salt, and hash.
func Hash(password string) (string, error) {
	if len(password) < MinPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return "", fmt.Errorf("password must be at most %d characters", MaxPasswordLength)
	}

	h, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return h, nil
}

// Verify compares password against an encoded hash. Uses constant-time
// comparison internally via the encoding package.
func Verify(password, hash string) error {
	if len(password) > MaxPasswordLength {
		return ErrInvalidPassword
	}
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !match {
		return ErrInvalidPassword
	}
	return nil
}
