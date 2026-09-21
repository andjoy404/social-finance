package auth

import (
	"regexp"
	"testing"
)

func TestHashSucceeds(t *testing.T) {
	hash, err := Hash("longpassword12345")
	if err != nil {
		t.Fatalf("Hash() returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("Hash() returned empty hash")
	}
}

func TestHashProducesArgon2idOutput(t *testing.T) {
	hash, err := Hash("longpassword12345")
	if err != nil {
		t.Fatalf("Hash() returned error: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("Hash() returned empty hash")
	}
	var foundArgon2id bool
	if foundArgon2id = regexp.MustCompile(`^\$argon2id\$`).MatchString(hash); !foundArgon2id {
		t.Fatalf("Hash does not look like argon2id output: %s", hash)
	}
}

func TestHashMinLengthEnforced(t *testing.T) {
	_, err := Hash("short")
	if err == nil {
		t.Fatal("Hash() should reject <12 char passwords")
	}
}

func TestHashMaxLengthEnforced(t *testing.T) {
	long := ""
	for i := 0; i < 129; i++ {
		long += "a"
	}
	_, err := Hash(long)
	if err == nil {
		t.Fatal("Hash() should reject >128 char passwords")
	}
}

func TestHashDifferentSaltProduceDifferentHashes(t *testing.T) {
	h1, err := Hash("password12345678")
	if err != nil {
		t.Fatalf("Hash 1 failed: %v", err)
	}
	h2, err := Hash("password12345678")
	if err != nil {
		t.Fatalf("Hash 2 failed: %v", err)
	}
	if h1 == h2 {
		t.Fatal("Same password hashed to identical strings (salt not varying)")
	}
}

func TestHashMaxlengthBoundary(t *testing.T) {
	long := ""
	for i := 0; i < 128; i++ {
		long += "a"
	}
	_, err := Hash(long)
	if err != nil {
		t.Fatalf("Hash() should accept 128 char password: %v", err)
	}
}

func TestHashMinlengthBoundary(t *testing.T) {
	_, err := Hash("123456789012")
	if err != nil {
		t.Fatalf("Hash() should accept 12 char password: %v", err)
	}
}

func TestHashEmtpyPassword(t *testing.T) {
	_, err := Hash("")
	if err == nil {
		t.Fatal("Hash() should reject empty password")
	}
}
