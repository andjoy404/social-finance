package rw

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/testutil"
)

func TestRWNotificationPermissionAliasMatrix(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	repo := NewRepository(pool.Raw())
	svc := NewService(pool, repo, nil)

	rwA := "rw-a"
	rwB := "rw-b"

	cases := []struct {
		name      string
		ac        *auth.AuthContext
		rwID      string
		perm      string
		wantAllow bool
	}{
		{"ketua rw explicit read canonical", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit}}}, rwA, "rw.notification.read", true},
		{"ketua rw explicit read alias", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit}}}, rwA, "notification.read", true},
		{"ketua rw explicit write canonical", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit}}}, rwA, "rw.notification.write", true},
		{"ketua rw explicit write alias manage", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit}}}, rwA, "notification.manage", true},
		{"ketua rw explicit write alias write", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit}}}, rwA, "notification.write", true},
		{"bendahara rw explicit write", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "bendahara", Source: auth.RWMembershipSourceExplicit}}}, rwA, "rw.notification.write", true},
		{"anggota rw explicit read only", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "anggota", Source: auth.RWMembershipSourceExplicit}}}, rwA, "rw.notification.read", true},
		{"anggota rw explicit cannot write", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "anggota", Source: auth.RWMembershipSourceExplicit}}}, rwA, "rw.notification.write", false},
		{"anggota rw explicit cannot manage alias", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "anggota", Source: auth.RWMembershipSourceExplicit}}}, rwA, "notification.manage", false},
		{"sekretaris rw explicit cannot write", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "sekretaris", Source: auth.RWMembershipSourceExplicit}}}, rwA, "notification.write", false},
		{"rt inferred cannot read rw notification", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "ketua", Source: auth.RWMembershipSourceRTInferred}}}, rwA, "rw.notification.read", false},
		{"rt inferred cannot write rw notification", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwA, Jabatan: "bendahara", Source: auth.RWMembershipSourceRTInferred}}}, rwA, "notification.manage", false},
		{"cross rw denied", &auth.AuthContext{SystemRole: "user", RWMemberships: []auth.RWMembership{{RWID: rwB, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit}}}, rwA, "rw.notification.read", false},
		{"super admin bypasses permission map but still needs RW scope", &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, RWMemberships: nil}, rwA, "rw.notification.write", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.checkRWPermission(tc.ac, tc.rwID, tc.perm)
			if tc.wantAllow {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}

	// SA scope rule: permission map bypass only after scope/access passes through RequireRWScope+membership checks;
	// checkRWPermission alone allows SA, but checkRWAccess/RequirePermission enforces scope validation.
	sa := &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin}
	require.NoError(t, svc.checkRWPermission(sa, rwA, "rw.notification.write"))
	err := svc.RequirePermission(ctx, sa, rwA, "rw.notification.write")
	require.Error(t, err, "super admin without valid RW scope must not gain access")
}
