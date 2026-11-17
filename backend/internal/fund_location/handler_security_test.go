package fundlocation

import (
	"testing"

	"social-finance/internal/auth"
)

func TestCheckRWBendaharaAccessSecurity(t *testing.T) {
	h := &Handler{}

	t.Run("SA passes through (validated by RequireRWScope middleware)", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID:     "sa-id",
			SystemRole: auth.SystemRoleSuperAdmin,
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-1", Jabatan: "ketua"},
			},
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		if err != nil {
			t.Errorf("expected nil for SA, got: %v", err)
		}
	})

	t.Run("Bendahara RW membership for matching rwId passes", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "user-id",
			RTID:   "rt-uuid-1",
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-1", Jabatan: "bendahara"},
				{RWID: "rw-uuid-2", Jabatan: "ketua"},
			},
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		if err != nil {
			t.Errorf("expected nil for RW bendahara matching rwId, got: %v", err)
		}
	})

	t.Run("Bendahara RW membership does NOT cross rwId", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "user-id",
			RTID:   "rt-uuid-1",
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-2", Jabatan: "bendahara"},
				{RWID: "rw-uuid-1", Jabatan: "sekretaris"},
			},
		}
		// Requesting rw-uuid-1 but only has bendahara on rw-uuid-2.
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		if err == nil {
			t.Error("expected error: bendahara on rw-uuid-2 should NOT authorize rw-uuid-1")
		}
	})

	t.Run("Ketua RW is blocked", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "user-id",
			RTID:   "rt-uuid-1",
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-1", Jabatan: "ketua"},
			},
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		if err == nil {
			t.Error("expected error: ketua should not be allowed to transfer")
		}
	})

	t.Run("Wakil Ketua RW is blocked", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "user-id",
			RTID:   "rt-uuid-1",
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-1", Jabatan: "wakil_ketua"},
			},
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		if err == nil {
			t.Error("expected error: wakil_ketua should not be allowed to transfer")
		}
	})

	t.Run("Sekretaris RW is blocked", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID: "user-id",
			RTID:   "rt-uuid-1",
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-1", Jabatan: "sekretaris"},
			},
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		if err == nil {
			t.Error("expected error: sekretaris should not be allowed to transfer")
		}
	})

	t.Run("No RW memberships → rejected", func(t *testing.T) {
		ac := &auth.AuthContext{
			UserID:        "user-id",
			RTID:          "rt-uuid-1",
			RWMemberships: nil,
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		if err == nil {
			t.Error("expected error: no RW memberships should be rejected")
		}
	})

	t.Run("SA without SCOPED_RW_IDS in env should be blocked by RequireRWScope", func(t *testing.T) {
		// This test verifies checkRWBendaharaAccess behavior.
		// SA is allowed through this handler, but RequireRWScope middleware
		// should block SA if SCOPED_RW_IDS is not configured.
		// checkRWBendaharaAccess itself allows SA through because the
		// middleware (RequireRWScope) is responsible for scope validation.
		ac := &auth.AuthContext{
			UserID:       "sa-id",
			SystemRole:   auth.SystemRoleSuperAdmin,
			MembershipID: "",
			RTID:         "",
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-1", Jabatan: "ketua"},
			},
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-1")
		// checkRWBendaharaAccess passes SA through — this is correct.
		// The actual blocking happens in RequireRWScope middleware.
		if err != nil {
			t.Errorf("expected nil for SA (RequireRWScope does scope validation), got: %v", err)
		}
	})

	t.Run("SA with RW membership but no scope should be blocked by RequireRWScope middleware", func(t *testing.T) {
		// Same as above: checkRWBendaharaAccess passes SA, but RequireRWScope
		// validates the SA's membership against SCOPED_RW_IDS allowlist.
		// This is NOT a test for checkRWBendaharaAccess — it's a test for
		// RequireRWScope (tested in scoping_test.go).
		// Just confirm checkRWBendaharaAccess allows SA through.
		ac := &auth.AuthContext{
			UserID:       "sa-id",
			SystemRole:   auth.SystemRoleSuperAdmin,
			MembershipID: "",
			RTID:         "",
			RWMemberships: []auth.RWMembership{
				{RWID: "rw-uuid-999", Jabatan: "ketua"},
			},
		}
		err := h.checkRWBendaharaAccess(ac, "rw-uuid-999")
		// SA passes through checkRWBendaharaAccess.
		// RequireRWScope middleware will block based on SCOPED_RW_IDS.
		if err != nil {
			t.Errorf("expected nil for SA, got: %v", err)
		}
	})
}
