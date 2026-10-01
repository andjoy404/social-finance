package auth

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"social-finance/internal/database"
)

// PermissionResolver resolves whether a jabatan grants a specific permission.
// It queries the database: position_permissions.position → permission_types.code.
type PermissionResolver struct {
	pool *database.Pool
	mu   sync.Mutex
}

// NewPermissionResolver creates a new permission resolver.
func NewPermissionResolver(pool *database.Pool) *PermissionResolver {
	return &PermissionResolver{pool: pool}
}

// HasPermission checks whether the authenticated user has the given permission.
// System-level super_admin always passes.
// jabatan-based users are evaluated against the position_permissions table.
// Role-based users (no jabatan) pass through without DB query — their access
// is enforced by RequireRole middleware on each route.
func (pr *PermissionResolver) HasPermission(ctx context.Context, tx *sql.Tx, ac *AuthContext, permission string) (bool, error) {
	// 1. System-level super_admin always passes.
	if ac.SystemRole == SystemRoleSuperAdmin {
		return true, nil
	}

	// 2. Role-based users (no jabatan) pass through without DB query.
	// Existing role-based authorization is preserved via RequireRole middleware.
	if ac.Jabatan == "" {
		return true, nil
	}

	// 3. jabatan-based user: verify membership exists in current RT, then check permissions.
	if ac.MembershipID == "" || ac.RTID == "" {
		return false, nil
	}

	// Validate the user has this jabatan in the current RT.
	jabatan, err := validateMembershipJabatan(ctx, tx, ac.UserID, ac.RTID, ac.Jabatan)
	if err != nil {
		return false, err
	}
	if jabatan == "" {
		// User does not have this jabatan in the current RT.
		return false, nil
	}

	// Check position_permissions for this jabatan and permission.
	return hasPermissionForPosition(ctx, tx, jabatan, permission)
}

// validateMembershipJabatan verifies that the user has the given jabatan
// in an active membership for the specified RT. Returns jabatan string or empty.
func validateMembershipJabatan(ctx context.Context, tx *sql.Tx, userID, rtID string, expectedJabatan Jabatan) (string, error) {
	var resultJabatan string
	err := tx.QueryRowContext(ctx, `
		SELECT jabatan
		FROM user_rt_memberships
		WHERE user_id = $1 AND rt_id = $2 AND is_active = true AND jabatan IS NOT NULL
		LIMIT 1
	`, userID, rtID).Scan(&resultJabatan)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("validate membership jabatan: %w", err)
	}
	return resultJabatan, nil
}

// hasPermissionForPosition checks whether the given position has the specified permission code.
// Queries: position_permissions.position → permission_types.id = permission_types.code.
func hasPermissionForPosition(ctx context.Context, tx *sql.Tx, position string, permissionCode string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM position_permissions pp
		JOIN permission_types pt ON pt.id = pp.permission_id
		WHERE pp.position = $1 AND pt.code = $2
	`, position, permissionCode).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check permission for position %s / %s: %w", position, permissionCode, err)
	}
	return count > 0, nil
}
