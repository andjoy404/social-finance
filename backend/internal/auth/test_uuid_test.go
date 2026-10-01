package auth

import (
	"testing"
)

func TestUUIDDeterministic(t *testing.T) {
	// Same label → same UUID
	u1 := testUUID("user-uuid-test")
	u2 := testUUID("user-uuid-test")
	if u1 != u2 {
		t.Errorf("same label produced different UUIDs: %s vs %s", u1, u2)
	}
}

func TestUUIDDifferentLabels(t *testing.T) {
	labels := []string{
		"user-uuid-test",
		"member-uuid-test",
		"rt-uuid-test",
		"user-uuid-a",
		"member-uuid-a",
		"user-uuid-x",
		"member-uuid-x",
		"rt-a-uuid",
		"user-a-uuid",
		"member-a-uuid",
		"rt-b-uuid",
	}
	seen := make(map[string]bool)
	for _, label := range labels {
		u := testUUID(label)
		if seen[u] {
			t.Errorf("collision: label %q and another label both produced UUID %s", label, u)
		}
		seen[u] = true
	}
}

func TestUUIDValidFormat(t *testing.T) {
	u := testUUID("user-uuid-test")
	// UUID v4 format: xxxxxxxx-xxxx-4xxx-[89ab]xxx-xxxxxxxxxxxx
	if len(u) != 36 {
		t.Fatalf("expected length 36, got %d: %s", len(u), u)
	}
	// Version nibble '4' at position 14 (first hex char of b[6] → group 3 start)
	if u[14] != '4' {
		t.Errorf("expected version nibble '4' at position 14, got %q", u[14])
	}
	// Variant nibble 8/b at position 19 (first hex char of b[8] → group 4 start)
	if u[19] < '8' || u[19] > 'b' {
		t.Errorf("expected variant nibble '8'-'b' at position 19, got %q", u[19])
	}
}
