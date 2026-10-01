package auth

import "social-finance/internal/database"

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
	JabatanKetua               Jabatan = "ketua"
	JabatanWakilKetua          Jabatan = "wakil_ketua"
	JabatanSekretaris          Jabatan = "sekretaris"
	JabatanBendahara           Jabatan = "bendahara"
	JabatanKeamanan            Jabatan = "keamanan"
	JabatanSosial              Jabatan = "sosial"
	JabatanKebersihanPembangunan Jabatan = "kebersihan_pembangunan"
)

// ValidJabatans maps recognized RT positions.
var ValidJabatans = map[Jabatan]bool{
	JabatanKetua:               true,
	JabatanWakilKetua:          true,
	JabatanSekretaris:          true,
	JabatanBendahara:           true,
	JabatanKeamanan:            true,
	JabatanSosial:              true,
	JabatanKebersihanPembangunan: true,
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
}

// AuthContext is attached to the request context after successful
// authentication. All authorised handlers derive rtID, userID, etc. from
// this — never from request body or query parameters.
type AuthContext struct {
	UserID       string
	SystemRole   SystemRole // empty if the user has no global authorization
	MembershipID string     // empty for system-only auth
	RTID         string     // empty for system-only auth
	TenantRole   Role       // empty for system-only auth
	Jabatan      Jabatan    // empty if no position assigned
}

// AuthDBPool is the database pool used by RequireAuth for identity validation.
// It is set once during application startup (before the HTTP server starts).
// This is needed because RequireAuth cannot accept a pool parameter — it is
// wired to all routes as a plain chi middleware (http.Handler).
var AuthDBPool *database.Pool
