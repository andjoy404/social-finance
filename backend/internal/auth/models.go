package auth

import "social-finance/internal/database"

// RWMembershipSource indicates how an RW membership was granted.
type RWMembershipSource string

const (
	RWMembershipSourceExplicit   RWMembershipSource = "explicit"
	RWMembershipSourceRTInferred RWMembershipSource = "rt_inferred"
)

// RWMembership represents a user's active membership in a Rukun Warga (RW).
// Jabatan uses RT-style values ('ketua', 'wakil_ketua', 'sekretaris', 'bendahara').
// Source indicates whether the membership is explicit (user_rw_memberships) or
// inferred from RT membership (user_rt_memberships + rts.rw_id). The Source field
// is populated by the server during token generation and validated on every request.
//
// NOTE: Source is serialized in the JWT (rw_context claim) so the middleware can
// distinguish explicit from inferred memberships.  Client input is NEVER trusted.
type RWMembership struct {
	RWID     string              `json:"rw_id"`
	RWNumber int                 `json:"rw_number"`
	RWName   string              `json:"rw_name"`
	Jabatan  string              `json:"jabatan"`
	Source   RWMembershipSource  `json:"source"`
}

// SystemRole represents a global-level role (not tied to an RT).
type SystemRole string

const (
	SystemRoleSuperAdmin SystemRole = "super_admin"
)

// ValidSystemRoles maps recognized system roles.
var ValidSystemRoles = map[SystemRole]bool{
	SystemRoleSuperAdmin: true,
}

// Role represents a role within a specific RT (tenant-level).
type Role string

const (
	RolePengurus  Role = "pengurus"
	RoleBendahara Role = "bendahara"
	RoleWarga     Role = "warga"
)

// ValidRoles maps recognized tenant roles.
var ValidRoles = map[Role]bool{
	RolePengurus:  true,
	RoleBendahara: true,
	RoleWarga:     true,
}

// Jabatan represents an RT organizational position (jabatan).
type Jabatan string

const (
	JabatanKetua                 Jabatan = "ketua"
	JabatanWakilKetua            Jabatan = "wakil_ketua"
	JabatanSekretaris            Jabatan = "sekretaris"
	JabatanBendahara             Jabatan = "bendahara"
	JabatanKeamanan              Jabatan = "keamanan"
	JabatanSosial                Jabatan = "sosial"
	JabatanKebersihanPembangunan Jabatan = "kebersihan_pembangunan"
	JabatanAnggota               Jabatan = "anggota"
)

// ValidJabatans maps recognized RT positions.
var ValidJabatans = map[Jabatan]bool{
	JabatanKetua:                 true,
	JabatanWakilKetua:            true,
	JabatanSekretaris:            true,
	JabatanBendahara:             true,
	JabatanKeamanan:              true,
	JabatanSosial:                true,
	JabatanKebersihanPembangunan: true,
	JabatanAnggota:               true,
}

// AllJabatans returns all valid jabatan values in a stable order.
var AllJabatans = []Jabatan{
	JabatanKetua,
	JabatanWakilKetua,
	JabatanSekretaris,
	JabatanBendahara,
	JabatanKeamanan,
	JabatanSosial,
	JabatanKebersihanPembangunan,
	JabatanAnggota,
}

// AuthContext is attached to the request context after successful
// authentication. All authorised handlers derive rtID, userID, etc. from
// this — never from request body or query parameters.
type AuthContext struct {
	UserID         string
	SystemRole     SystemRole // empty if the user has no global authorization
	MembershipID   string     // empty for system-only auth
	RTID           string     // empty for system-only auth
	TenantRole     Role       // empty for system-only auth
	Jabatan        Jabatan    // empty if no position assigned
	RWMemberships  []RWMembership
}

// AuthDBPool is the database pool used by RequireAuth for identity validation.
// It is set once during application startup (before the HTTP server starts).
// This is needed because RequireAuth cannot accept a pool parameter — it is
// wired to all routes as a plain chi middleware (http.Handler).
var AuthDBPool *database.Pool
