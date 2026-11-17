package auth

import (
	"strings"
	"testing"
)

// TestRWScopePermissionsAudit audits every jabatan entry in rwPermissionMap
// against each expected permission code. This file tests the permission
// resolver logic directly (unit-level) and also ensures the map is exhaustive.
func TestRWScopePermissionsAudit(t *testing.T) {
	// All jabatan values that appear in rwPermissionMap.
	allJabatans := []string{"ketua", "wakil_ketua", "sekretaris", "bendahara"}
	// All permission codes that appear in rwPermissionMap.
	allPermissions := []string{
		"rw.dashboard.read",
		"rw.kas.read",
		"rw.kas.write",
		"rw.fund_location.read",
		"rw.fund_location.write",
		"rw.notification.read",
		"rw.notification.write",
		"finance.read",
		"finance.create",
		"finance.update",
		"finance.approve",
		"notification.read",
		"notification.manage",
	}

	// Build a matrix of expected permissions per jabatan based on the strict final RBAC matrix:
	// - Ketua RW: Beranda READ, KAS WRITE, Lokasi Dana WRITE, Notifikasi WRITE
	// - Bendahara RW: Beranda READ, KAS WRITE, Lokasi Dana WRITE, Notifikasi WRITE
	// - Pengurus RW (wakil_ketua, sekretaris): Beranda READ, KAS READ, Lokasi Dana READ, Notifikasi READ (ALL WRITE DENIED)
	expected := map[string]map[string]bool{
		"ketua": {
			"rw.dashboard.read":     true,
			"rw.kas.read":           true,
			"rw.kas.write":          true,
			"rw.fund_location.read":  true,
			"rw.fund_location.write": true,
			"rw.notification.read":   true,
			"rw.notification.write":  true,
			"finance.read":          true,
			"finance.create":        true,
			"finance.update":        true,
			"finance.approve":       true,
			"notification.read":     true,
			"notification.manage":   true,
		},
		"bendahara": {
			"rw.dashboard.read":     true,
			"rw.kas.read":           true,
			"rw.kas.write":          true,
			"rw.fund_location.read":  true,
			"rw.fund_location.write": true,
			"rw.notification.read":   true,
			"rw.notification.write":  true,
			"finance.read":          true,
			"finance.create":        true,
			"finance.update":        true,
			"finance.approve":       true,
			"notification.read":     true,
			"notification.manage":   true,
		},
		"wakil_ketua": {
			"rw.dashboard.read":     true,
			"rw.kas.read":           true,
			"rw.kas.write":          false, // Pengurus RW cannot WRITE
			"rw.fund_location.read":  true,
			"rw.fund_location.write": false, // Pengurus RW cannot WRITE
			"rw.notification.read":   true,
			"rw.notification.write":  false, // Pengurus RW cannot WRITE
			"finance.read":          true,
			"finance.create":        false, // Pengurus RW cannot WRITE
			"finance.update":        false, // Pengurus RW cannot WRITE
			"finance.approve":       false, // Pengurus RW cannot WRITE
			"notification.read":     true,
			"notification.manage":   false, // Pengurus RW cannot WRITE
		},
		"sekretaris": {
			"rw.dashboard.read":     true,
			"rw.kas.read":           true,
			"rw.kas.write":          false, // Pengurus RW cannot WRITE
			"rw.fund_location.read":  true,
			"rw.fund_location.write": false, // Pengurus RW cannot WRITE
			"rw.notification.read":   true,
			"rw.notification.write":  false, // Pengurus RW cannot WRITE
			"finance.read":          true,
			"finance.create":        false,
			"finance.update":        false,
			"finance.approve":       false,
			"notification.read":     true,
			"notification.manage":   false,
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

	// rwPermissionMap also carries legacy/domain aliases for compatibility;
	// those are allowed when they map to RW-safe domains.
	isAllowedRWCompat := func(p string) bool {
		return strings.HasPrefix(p, "finance.") || strings.HasPrefix(p, "notification.") || strings.HasPrefix(p, "rw.") ||
			strings.HasPrefix(p, "warga.") || strings.HasPrefix(p, "reports.") || strings.HasPrefix(p, "rt.settings.")
	}
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
		// Check for unexpected permissions outside the audited matrix + allowed compat aliases.
		for p := range actualSet {
			if expected[jt][p] {
				continue
			}
			if isAllowedRWCompat(p) {
				continue
			}
			t.Errorf("jabatan %q has unexpected permission %q", jt, p)
		}
	}
}

// TestRWScopePermissionsSecurityGates verifies key security invariants:
//  1. Sekretaris cannot finance.create, finance.update, finance.approve, or any write permissions.
//  2. Wakil Ketua cannot finance.create, finance.update, finance.approve, or any write permissions.
//  3. Bendahara RW and Ketua RW have write permissions on finance/kas, fund location, and notification.
//  4. All jabatan have finance.read and dashboard.read.
//  5. Nonexistent jabatan returns false for every permission.
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
	if hasRWScopePermission("sekretaris", "notification.manage") {
		t.Error("sekretaris must NOT have notification.manage")
	}
	if hasRWScopePermission("sekretaris", "rw.notification.write") {
		t.Error("sekretaris must NOT have rw.notification.write")
	}

	// wakil_ketua: read-only (NO write operations per final matrix).
	if hasRWScopePermission("wakil_ketua", "finance.create") {
		t.Error("wakil_ketua must NOT have finance.create")
	}
	if hasRWScopePermission("wakil_ketua", "finance.update") {
		t.Error("wakil_ketua must NOT have finance.update")
	}
	if hasRWScopePermission("wakil_ketua", "finance.approve") {
		t.Error("wakil_ketua must NOT have finance.approve")
	}
	if hasRWScopePermission("wakil_ketua", "notification.manage") {
		t.Error("wakil_ketua must NOT have notification.manage")
	}
	if hasRWScopePermission("wakil_ketua", "rw.notification.write") {
		t.Error("wakil_ketua must NOT have rw.notification.write")
	}

	// bendahara: has WRITE on kas, fund location, and notification.
	if !hasRWScopePermission("bendahara", "notification.manage") {
		t.Error("bendahara RW MUST have notification.manage")
	}
	if !hasRWScopePermission("bendahara", "rw.notification.write") {
		t.Error("bendahara RW MUST have rw.notification.write")
	}

	// ketua: has WRITE on kas, fund location, and notification.
	if !hasRWScopePermission("ketua", "notification.manage") {
		t.Error("ketua RW MUST have notification.manage")
	}
	if !hasRWScopePermission("ketua", "rw.notification.write") {
		t.Error("ketua RW MUST have rw.notification.write")
	}

	// All jabatan have finance.read.
	for _, jt := range []string{"ketua", "wakil_ketua", "sekretaris", "bendahara"} {
		if !hasRWScopePermission(jt, "finance.read") {
			t.Errorf("%q must have finance.read", jt)
		}
	}

	// Nonexistent jabatan returns false for all.
	for _, perm := range []string{"finance.read", "finance.create", "finance.update", "finance.approve", "user.delete", "rw.kas.read"} {
		if hasRWScopePermission("nonexistent_jabatan", perm) {
			t.Errorf("nonexistent jabatan must return false for %s", perm)
		}
	}
}

// TestRWScopePermissionsCrossDomain ensures RW permissions only cover finance.*, notification.*, and rw.*
// and never accidentally expose user, rt, or other domain permissions.
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

// TestRWScopePermissionCompleteness verifies that all canonical RW-specific
// jabatan values are covered and each has at least one permission.
func TestRWScopePermissionCompleteness(t *testing.T) {
	if len(rwPermissionMap) < 4 {
		t.Fatalf("expected at least 4 jabatan entries in rwPermissionMap, got %d", len(rwPermissionMap))
	}

	for _, jt := range []string{"ketua", "wakil_ketua", "sekretaris", "bendahara", "anggota", "pengurus"} {
		perms, ok := rwPermissionMap[jt]
		if !ok {
			t.Fatalf("jabatan %q missing from rwPermissionMap", jt)
		}
		if len(perms) == 0 {
			t.Errorf("jabatan %q has no permissions assigned", jt)
		}
		// Every permission in the map must be in an allowed RW-compatible domain.
		for _, p := range perms {
			if !strings.HasPrefix(p, "finance.") && !strings.HasPrefix(p, "notification.") && !strings.HasPrefix(p, "rw.") &&
				!strings.HasPrefix(p, "warga.") && !strings.HasPrefix(p, "reports.") && !strings.HasPrefix(p, "rt.settings.") {
				t.Errorf("jabatan %q has invalid permission %q", jt, p)
			}
		}
	}
}
