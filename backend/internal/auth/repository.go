package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrUserNotFound indicates no user matched the lookup criteria.
var ErrUserNotFound = fmt.Errorf("user not found")

// ErrNotAuthorized indicates the user exists but has no matching active membership,
// or the membership/RT is inactive, or the credential is invalid.
var ErrNotAuthorized = fmt.Errorf("not authorized")

// ErrMultipleMemberships indicates a user has multiple active RT memberships.
// The client should prompt the user to select one.
var ErrMultipleMemberships = fmt.Errorf("user has multiple active memberships")

// ErrInvalidRefreshToken indicates the refresh token is unknown, expired, or revoked.
var ErrInvalidRefreshToken = fmt.Errorf("invalid refresh token")

// UserWithHash is the result of loading a user including the stored password hash
// and system-level role.
type UserWithHash struct {
	ID           string
	SystemRole   SystemRole
	Fullname     string
	Email        string // NOT NULL after migration
	Phone        *string
	PasswordHash string
	IsActive     bool
}

// Membership is a user's role assignment within a specific RT.
type Membership struct {
	ID       string
	UserID   string
	RTID     string
	Role     Role
	IsActive bool
}

// UsersFindByEmail finds a user by normalized email.
// Email is stored as lower(trimmed(original)) for global uniqueness.
// Returns sql.ErrNoRows when not found.
func UsersFindByEmail(ctx context.Context, tx *sql.Tx, email string) (*UserWithHash, error) {
	query := `
		SELECT id, system_role, full_name, email, phone, password_hash, is_active
		FROM users
		WHERE lower(trim(email)) = lower(trim($1))
		LIMIT 1
	`
	var u UserWithHash
	var sysRole sql.NullString
	err := tx.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &sysRole, &u.Fullname, &u.Email, &u.Phone, &u.PasswordHash, &u.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	if sysRole.Valid {
		u.SystemRole = SystemRole(sysRole.String)
	}
	return &u, nil
}

// UsersFindByID finds a user by UUID.
func UsersFindByID(ctx context.Context, tx *sql.Tx, userID string) (*UserWithHash, error) {
	query := `
		SELECT id, system_role, full_name, email, phone, password_hash, is_active
		FROM users
		WHERE id = $1
		LIMIT 1
	`
	var u UserWithHash
	var sysRole sql.NullString
	err := tx.QueryRowContext(ctx, query, userID).Scan(
		&u.ID, &sysRole, &u.Fullname, &u.Email, &u.Phone, &u.PasswordHash, &u.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	if sysRole.Valid {
		u.SystemRole = SystemRole(sysRole.String)
	}
	return &u, nil
}

// RTMembershipsFindByUser returns all memberships for a user.
func RTMembershipsFindByUser(ctx context.Context, tx *sql.Tx, userID string) ([]Membership, error) {
	query := `
		SELECT id, rt_id, role, is_active
		FROM user_rt_memberships
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at ASC
	`
	rows, err := tx.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("find memberships: %w", err)
	}
	defer rows.Close()

	var result []Membership
	for rows.Next() {
		var m Membership
		var roleStr string
		if err := rows.Scan(&m.ID, &m.RTID, &roleStr, &m.IsActive); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		m.Role = Role(roleStr)
		m.UserID = userID
		result = append(result, m)
	}
	return result, rows.Err()
}

// RTMembershipFindByUserAndRT returns the active membership for a user+RT.
// Returns sql.ErrNoRows when not found.
func RTMembershipFindByUserAndRT(ctx context.Context, tx *sql.Tx, userID, rtID string) (*Membership, error) {
	query := `
		SELECT id, user_id, rt_id, role, is_active
		FROM user_rt_memberships
		WHERE user_id = $1 AND rt_id = $2 AND is_active = true
		LIMIT 1
	`
	var m Membership
	var roleStr string
	err := tx.QueryRowContext(ctx, query, userID, rtID).Scan(
		&m.ID, &m.UserID, &m.RTID, &roleStr, &m.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotAuthorized
		}
		return nil, fmt.Errorf("find membership: %w", err)
	}
	m.Role = Role(roleStr)
	return &m, nil
}

// RTMembershipFindByID returns a membership by its UUID.
func RTMembershipFindByID(ctx context.Context, tx *sql.Tx, id string) (*Membership, error) {
	query := `
		SELECT id, user_id, rt_id, role, is_active
		FROM user_rt_memberships
		WHERE id = $1 AND is_active = true
		LIMIT 1
	`
	var m Membership
	var roleStr string
	err := tx.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.UserID, &m.RTID, &roleStr, &m.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotAuthorized
		}
		return nil, fmt.Errorf("find membership by id: %w", err)
	}
	m.Role = Role(roleStr)
	return &m, nil
}

// CreateNewUser inserts a new user and returns the generated ID.
func CreateNewUser(ctx context.Context, tx *sql.Tx, email string, phone *string, passwordHash, fullname, systemRole string) (string, error) {
	query := `
		INSERT INTO users (email, phone, password_hash, full_name, system_role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id string
	err := tx.QueryRowContext(ctx, query, email, phone, passwordHash, fullname, systemRole).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

// CreateMembership creates a user's membership in an RT.
func CreateMembership(ctx context.Context, tx *sql.Tx, userID, rtID string, role Role) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role) VALUES ($1, $2, $3)`,
		userID, rtID, string(role),
	)
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}
	return nil
}

// CreateRefreshToken stores a new refresh token hash.
func CreateRefreshToken(ctx context.Context, tx *sql.Tx, userID, membershipID string, tokenHash string, expiresAt time.Time) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, membership_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		userID, membershipID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

// FindRefreshTokenByHash looks up a refresh token by its hash.
// Returns the DB row or sql.ErrNoRows.
func FindRefreshTokenByHash(ctx context.Context, tx *sql.Tx, tokenHash string) (userID, membershipID string, expiresAt time.Time, revoked bool, err error) {
	query := `
		SELECT user_id, membership_id, expires_at,
			CASE WHEN revoked_at IS NOT NULL THEN true ELSE false END
		FROM refresh_tokens
		WHERE token_hash = $1
		LIMIT 1
	`
	err = tx.QueryRowContext(ctx, query, tokenHash).Scan(&userID, &membershipID, &expiresAt, &revoked)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", time.Time{}, false, sql.ErrNoRows
		}
		return "", "", time.Time{}, false, fmt.Errorf("find refresh token: %w", err)
	}
	return userID, membershipID, expiresAt, revoked, nil
}

// RevokeRefreshTokenByHash marks a refresh token as revoked.
// Also revokes any token that has already replaced it (cascading).
func RevokeRefreshTokenByHash(ctx context.Context, tx *sql.Tx, tokenHash string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE token_hash = $1`,
		tokenHash,
	); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE replaced_by_hash = $1`,
		tokenHash,
	); err != nil {
		// Best-effort -- the primary token is already revoked.
	}
	return nil
}

// RevokeRefreshTokensByMembership revokes all active refresh tokens for a membership.
func RevokeRefreshTokensByMembership(ctx context.Context, tx *sql.Tx, membershipID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE membership_id = $1 AND revoked_at IS NULL`,
		membershipID,
	)
	if err != nil {
		return fmt.Errorf("revoke tokens for membership: %w", err)
	}
	return nil
}

// FindRTByID finds an RT by UUID.
// Returns sql.ErrNoRows when not found.
func FindRTByID(ctx context.Context, tx *sql.Tx, rtID string) (name string, isActive bool, err error) {
	query := `SELECT name, is_active FROM rts WHERE id = $1 LIMIT 1`
	err = tx.QueryRowContext(ctx, query, rtID).Scan(&name, &isActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, ErrNotAuthorized
		}
		return "", false, fmt.Errorf("find rt: %w", err)
	}
	return name, isActive, nil
}

// UserForMe returns user info plus their active RT details for the /me endpoint.
type UserForMe struct {
	ID         string  `json:"id"`
	Fullname   string  `json:"full_name"`
	Email      string  `json:"email"` // NOT NULL after migration
	Phone      *string `json:"phone,omitempty"`
	TenantRole Role    `json:"role"`
	SystemRole string  `json:"system_role,omitempty"` // "super_admin" if present
	RTID       string  `json:"rt_id"`
	RTName     string  `json:"rt_name"`
	RT_rw      int     `json:"rt_rw"`
	RT_rt      string  `json:"rt_rt"`
}

// GetProfile returns user info plus their active RT details for the /me endpoint.
func GetProfile(ctx context.Context, tx *sql.Tx, userID, membershipID string) (*UserForMe, error) {
	query := `
		SELECT u.id, u.full_name, u.email, u.phone, u.system_role, urm.role,
			rt.id, rt.name, rt.rw, rt.rt
		FROM users u
		JOIN user_rt_memberships urm ON urm.id = $2
		JOIN rts rt ON rt.id = urm.rt_id
		WHERE u.id = $1 AND u.is_active = true
		AND urm.is_active = true AND rt.is_active = true
		LIMIT 1
	`
	var u UserForMe
	var sysRole sql.NullString
	var roleStr string
	err := tx.QueryRowContext(ctx, query, userID, membershipID).Scan(
		&u.ID, &u.Fullname, &u.Email, &u.Phone, &sysRole,
		&roleStr, &u.RTID, &u.RTName, &u.RT_rw, &u.RT_rt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotAuthorized
		}
		return nil, fmt.Errorf("get user for me: %w", err)
	}
	if sysRole.Valid {
		u.SystemRole = sysRole.String
	}
	u.TenantRole = Role(roleStr)
	return &u, nil
}

// GetSystemProfile returns user profile for system-only SUPER_ADMIN (no RT membership).
func GetSystemProfile(ctx context.Context, tx *sql.Tx, userID string) (*UserForMe, error) {
	query := `
		SELECT id, full_name, email, phone, system_role
		FROM users
		WHERE id = $1 AND is_active = true
		LIMIT 1
	`
	var u UserForMe
	var sysRole sql.NullString
	err := tx.QueryRowContext(ctx, query, userID).Scan(
		&u.ID, &u.Fullname, &u.Email, &u.Phone, &sysRole,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get system profile: %w", err)
	}
	if sysRole.Valid {
		u.SystemRole = sysRole.String
	}
	return &u, nil
}
