package auth

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

// AuthContext is attached to the request context after successful
// authentication. All authorised handlers derive rtID, userID, etc. from
// this — never from request body or query parameters.
type AuthContext struct {
	UserID       string
	SystemRole   SystemRole // empty if the user has no global authorization
	MembershipID string     // empty for system-only users
	RTID         string     // empty for system-only users
	TenantRole   Role       // empty for system-only users
}
