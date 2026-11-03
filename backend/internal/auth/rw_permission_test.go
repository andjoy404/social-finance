package auth

import (
	"testing"
)

// TestRWScopePermissionsAudit audits every jabatan entry in rwPermissionMap
// against each expected permission code. This file tests the permission
// resolver logic directly (unit-level) and also ensures the map is exhaustive.
func TestRWScopePermissionsAudit(t *testing.T) {
	// All jabatan values that appear in rwPermissionMap.
	allJabatans := []string{"ketua", "wakil_ketua", "sekretaris", "bendahara"}
	// All permission codes that appear in rwPermissionMap.
	allPermissions := []string{"finance.read", "finance.create", "finance.update", "finance.approve"}

	// Build a matrix of expected permissions per jabatan.
	expected := map[string]map[string]bool{
		"ketua": {
			"finance.read":     true,
			"finance.create":   true,
			"finance.update":   true,
			"finance.approve":  true,
		},
		"wakil_ketua": {
			"finance.read":     true,
			"finance.create":   true,
			"finance.update":   false, // SECURITY GATE: no update/transfer
			"finance.approve":  true,
		},
		"sekretaris": {
			"finance.read":     true,
			"finance.create":   false,
			"finance.update":   false,
			"finance.approve":  false,
		},
		"bendahara": {
			"finance.read":     true,
			"finance.create":   true,
			"finance.update":   true,
			"finance.approve":  true,
		},
	}

	// Verify every jabatan in the map matches the expected matrix.
	for _, jt := range allJabatans {
		for _, perm := range allPermissions {
			t.Run(jt+"_"+perm, func(t *testing.T) {
				got := hasRWScopePermission(jt, perm)
				want := expected[jt][perm]
				if got != want {
					t.Errorf("hasRWScopePermission(%q, %q) = %v, want %v", jt, perm, got, want)
				}
			})
		}
	}

	// Verify no jabatan in the map has unexpected permissions.
	// Iterate over the actual map entries.
	for _, jt := range allJabatans {
		perms := rwPermissionMap[jt]
		actualSet := make(map[string]bool)
		for _, p := range perms {
			actualSet[p] = true
		}
		for _, p := range allPermissions {
			want := expected[jt][p]
			if got := actualSet[p]; got != want {
				t.Errorf("jabatan %q has %s = %v, want %v", jt, p, got, want)
			}
		}
		// Also check for unexpected extra permissions.
		for p := range actualSet {
			if !expected[jt][p] {
				t.Errorf("jabatan %q has unexpected permission %q", jt, p)
			}
		}
	}
}

// TestRWScopePermissionsSecurityGates verifies key security invariants:
//  1. Sekretaris cannot finance.create, finance.update, or finance.approve.
//  2. Wakil Ketua cannot finance.update.
//  3. All jabatan have finance.read (read-only is the minimum).
//  4. Nonexistent jabatan returns false for every permission.
func TestRWScopePermissionsSecurityGates(t *testing.T) {
	// sekretaris: read-only.
	if hasRWScopePermission("sekretaris", "finance.create") {
		t.Error("sekretaris must NOT have finance.create")
	}
	if hasRWScopePermission("sekretaris", "finance.update") {
		t.Error("sekretaris must NOT have finance.update")
	}
	if hasRWScopePermission("sekretaris", "finance.approve") {
		t.Error("sekretaris must NOT have finance.approve")
	}

	// wakil_ketua: no finance.update.
	if hasRWScopePermission("wakil_ketua", "finance.update") {
		t.Error("wakil_ketua must NOT have finance.update")
	}

	// All jabatan have finance.read.
	for _, jt := range []string{"ketua", "wakil_ketua", "sekretaris", "bendahara"} {
		if !hasRWScopePermission(jt, "finance.read") {
			t.Errorf("%q must have finance.read", jt)
		}
	}

	// Nonexistent jabatan returns false for all.
	for _, perm := range []string{"finance.read", "finance.create", "finance.update", "finance.approve", "user.delete"} {
		if hasRWScopePermission("nonexistent_jabatan", perm) {
			t.Errorf("nonexistent jabatan must return false for %s", perm)
		}
	}
}

// TestRWScopePermissionsCrossDomain ensures RW permissions only cover finance.*
// and never accidentally expose user, rw, or other domain permissions.
func TestRWScopePermissionsCrossDomain(t *testing.T) {
	crossDomainPerms := []string{
		"user.read", "user.delete", "user.create",
		"rw.read", "rw.update", "rw.delete",
		"household.read", "household.create",
		"rt.read", "rt.delete",
	}

	for _, jt := range []string{"ketua", "wakil_ketua", "sekretaris", "bendahara"} {
		for _, perm := range crossDomainPerms {
			if hasRWScopePermission(jt, perm) {
				t.Errorf("jabatan %q must NOT have cross-domain permission %q", jt, perm)
			}
		}
	}
}

// TestRWScopePermissionCompleteness verifies that all four RW-specific
// jabatan values are covered and each has at least one permission.
func TestRWScopePermissionCompleteness(t *testing.T) {
	// rwPermissionMap must have exactly 4 entries.
	if len(rwPermissionMap) != 4 {
		t.Fatalf("expected 4 jabatan entries in rwPermissionMap, got %d", len(rwPermissionMap))
	}

	for _, jt := range []string{"ketua", "wakil_ketua", "sekretaris", "bendahara"} {
		perms, ok := rwPermissionMap[jt]
		if !ok {
			t.Fatalf("jabatan %q missing from rwPermissionMap", jt)
		}
		if len(perms) == 0 {
			t.Errorf("jabatan %q has no permissions assigned", jt)
		}
		// Every permission in the map must start with "finance.".
		for _, p := range perms {
			if len(p) < 8 || p[:8] != "finance." {
				t.Errorf("jabatan %q has non-finance permission %q", jt, p)
			}
		}
	}
}
