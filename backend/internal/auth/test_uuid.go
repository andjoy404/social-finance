package auth

import (
	"crypto/sha256"
	"fmt"
)

// testUUID maps a human-readable label to a deterministic UUID v4.
//
// It hashes the label with SHA-256, takes the first 16 bytes, forces the
// version to 4 (random) and the variant to RFC 4122, then returns the
// resulting UUID as a string in standard dash-separated notation.
//
// This function is test-only infrastructure. It is safe to call from
// both fixture helpers (which write to PostgreSQL) and test code that
// builds JWT claims, ensuring the database and the token carry the
// identical UUID for a given label.
//
// Examples:
//
//	testUUID("user-uuid-test") → "6c8b... (deterministic UUID v4)"
//	testUUID("member-uuid-test") → "3f2a... (deterministic UUID v4)"
//	testUUID("user-uuid-test") == testUUID("user-uuid-test") // always equal
//	testUUID("a") != testUUID("b") // different labels → different UUIDs
func testUUID(label string) string {
	h := sha256.Sum256([]byte(label))
	b := make([]byte, 16)
	copy(b, h[:])

	// Force UUID v4 variant.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3],
		b[4], b[5],
		b[6], b[7],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15],
	)
}
