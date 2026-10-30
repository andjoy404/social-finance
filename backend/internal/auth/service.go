package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// loadRWMemberships queries user_rw_memberships for the given user and returns
// a slice of RWMembership. Returns nil (not error) when no memberships exist.
func loadRWMemberships(ctx context.Context, tx *sql.Tx, userID string) ([]RWMembership, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT urm.rw_id, w.name, urm.jabatan FROM user_rw_memberships urm
		 LEFT JOIN rws w ON w.id = urm.rw_id
		 WHERE urm.user_id = $1 AND urm.is_active = true`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("load rw memberships: %w", err)
	}
	defer rows.Close()

	var result []RWMembership
	for rows.Next() {
		var rwID, rwName, jabatan string
		if err := rows.Scan(&rwID, &rwName, &jabatan); err != nil {
			return nil, fmt.Errorf("scan rw membership: %w", err)
		}
		result = append(result, RWMembership{
			RWID:   rwID,
			Name:   rwName,
			Jabatan: jabatan,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rw memberships: %w", err)
	}
	return result, nil
}

// buildRWContextJSON serializes a slice of RWMembership into a compact JSON string.
// Returns an empty string when the slice is nil or empty (backward compatible).
func buildRWContextJSON(memberships []RWMembership) string {
	if len(memberships) == 0 {
		return ""
	}
	b, err := json.Marshal(memberships)
	if err != nil {
		return ""
	}
	return string(b)
}

// Service implements login, token refresh, logout, and profile lookup.
type Service struct {
	refreshLifetime time.Duration
	deviceIDHeader  string
}

// ServiceOptions configures the Service.
type ServiceOptions struct {
	RefreshLifetime time.Duration
	DeviceIDHeader  string
}

// NewService creates an auth service with the given options.
func NewService(opts ...ServiceOptions) *Service {
	lifetime := 90 * 24 * time.Hour // default 90 days
	var deviceIDHeader string
	for _, o := range opts {
		if o.RefreshLifetime > 0 {
			lifetime = o.RefreshLifetime
		}
		if o.DeviceIDHeader != "" {
			deviceIDHeader = o.DeviceIDHeader
		}
	}
	return &Service{refreshLifetime: lifetime, deviceIDHeader: deviceIDHeader}
}

// LoginResult holds the tokens and user info returned after successful login.
type LoginResult struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	DeviceID     string    `json:"device_id,omitempty"`
	User         *UserInfo `json:"user"`
}

// UserInfo is the minimal user+RT info returned in login and /me responses.
type UserInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"full_name"`
	Email      string  `json:"email"`
	SystemRole string  `json:"system_role,omitempty"`
	Role       Role    `json:"role"`
	Jabatan    string  `json:"jabatan,omitempty"`
	RT         *RTInfo `json:"rt,omitempty"`
}

// RTInfo describes the user's active RT.
type RTInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	RT   string `json:"rt,omitempty"`
	RW   int    `json:"rw,omitempty"`
}

// Login authenticates a user by email, phone, or identifier + password.
//
// Rules enforced during login:
//   - user exists and is active
//   - password matches
//   - user has exactly one active membership, OR no member­ship (SUPER_ADMIN)
//   - that membership is active, and the RT is active
//
// For SUPER_ADMIN users with no active membership:
//
//	login succeeds with system_role in claims, no RT context.
//
// For users with multiple active memberships:
//
//	ErrMultipleMemberships is returned.
func (s *Service) Login(ctx context.Context, tx *sql.Tx, identifier, rawPassword, deviceID string) (*LoginResult, error) {
	// Step 1 -- find user by identifier (email or phone).
	user, err := UsersFindByIdentifier(ctx, tx, identifier)
	if err != nil {
		return nil, err
	}

	// Step 2 -- verify password (constant-time).
	if err := Verify(rawPassword, user.PasswordHash); err != nil {
		return nil, ErrInvalidPassword
	}

	// Step 3 -- user must be active.
	if !user.IsActive {
		return nil, ErrNotAuthorized
	}

	// Step 4 -- find active memberships.
	memberships, err := RTMembershipsFindByUser(ctx, tx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("find memberships: %w", err)
	}

	// --- Case: user has memberships ---
	var m *Membership
	if len(memberships) == 0 {
		// No memberships. Only SUPER_ADMIN can proceed.
		if user.SystemRole != SystemRoleSuperAdmin {
			return nil, ErrNotAuthorized
		}
		// System-only super_admin: no RT context.
	} else if len(memberships) == 1 {
		m = &memberships[0]
	} else {
		// Multiple memberships — determine which RT to use.
		// For SUPER_ADMIN: prefer the RT whose RW matches one of the user's
		// RW memberships (if any), falling back to the first membership.
		// This avoids hard-coding any specific RT UUID and works regardless
		// of how many RTs/RWs the user is attached to.
		if user.SystemRole == SystemRoleSuperAdmin {
			// Load RW memberships to build an allowlist of RW IDs.
			rwMemberships, err := loadRWMemberships(ctx, tx, user.ID)
			if err == nil && len(rwMemberships) > 0 {
				rwIDs := make(map[string]struct{}, len(rwMemberships))
				for _, rm := range rwMemberships {
					rwIDs[rm.RWID] = struct{}{}
				}
				// Query each RT's rw_id and find a match.
				for i := range memberships {
					var rtRWID string
					err := tx.QueryRowContext(ctx,
						`SELECT rw_id FROM rts WHERE id = $1 LIMIT 1`,
						memberships[i].RTID,
					).Scan(&rtRWID)
					if err == nil {
						if _, ok := rwIDs[rtRWID]; ok {
							m = &memberships[i]
							break
						}
					}
				}
			}
			if m == nil {
				m = &memberships[0]
			}
		} else {
			return nil, ErrMultipleMemberships
		}
	}

	// --- Case: member login ---
	if m != nil {
		// Check membership + RT are active.
		if !m.IsActive {
			return nil, ErrNotAuthorized
		}
		rtName, rtCode, rtRW, rtActive, err := FindRTDetailsByID(ctx, tx, m.RTID)
		if err != nil {
			return nil, ErrNotAuthorized
		}
		if !rtActive {
			return nil, ErrNotAuthorized
		}

		// Generate tokens with both system role and tenant context.
		// system_role is included in claims regardless of whether the user
		// has a tenant membership; this avoids granting system access to
		// a super_admin who has never used the system before (the login
		// has already proven identity).
		//
		// RW memberships are loaded for all users (tenant and system-only)
		// so that super_admins with RW access carry their RW context in the JWT.
		var rwMemberships []RWMembership
		if userRW, err := loadRWMemberships(ctx, tx, user.ID); err == nil {
			rwMemberships = userRW
		}
		rwContext := buildRWContextJSON(rwMemberships)

		accessToken, err := GenerateAccessToken(TokenClaims{
			UserID:    user.ID,
			MID:       m.ID,
			RTID:      m.RTID,
			Role:      string(m.Role),
			Jabatan:   string(m.Jabatan),
			SysRole:   string(user.SystemRole),
			RWContext: rwContext,
		})
		if err != nil {
			return nil, fmt.Errorf("generate access token: %w", err)
		}

		refreshToken, err := GenerateRefreshToken()
		if err != nil {
			return nil, fmt.Errorf("generate refresh token: %w", err)
		}
		refreshHash := refreshToken
		expiresAt := time.Now().Add(s.refreshLifetime)

		if err := CreateRefreshToken(ctx, tx, user.ID, &m.ID, refreshHash, expiresAt, deviceID); err != nil {
			return nil, fmt.Errorf("create refresh token: %w", err)
		}

		return &LoginResult{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    int(DefaultAccessTokenLifetime.Seconds()),
			User: &UserInfo{
				ID:         user.ID,
				Name:       user.Fullname,
				Email:      user.Email,
				SystemRole: string(user.SystemRole),
				Role:       m.Role,
				Jabatan:    string(m.Jabatan),
				RT: &RTInfo{
					ID:   m.RTID,
					Name: rtName,
					RT:   rtCode,
					RW:   rtRW,
				},
			},
		}, nil
	}

	// --- Case: system-only SUPER_ADMIN ---
	// Load RW memberships for system-only super_admins too.
	var rwMemberships []RWMembership
	if userRW, err := loadRWMemberships(ctx, tx, user.ID); err == nil {
		rwMemberships = userRW
	}
	rwContext := buildRWContextJSON(rwMemberships)

	accessToken, err := GenerateAccessToken(TokenClaims{
		UserID:    user.ID,
		SysRole:   string(user.SystemRole),
		RWContext: rwContext,
	})
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshHash := refreshToken
	expiresAt := time.Now().Add(s.refreshLifetime)

	if err := CreateRefreshToken(ctx, tx, user.ID, nil, refreshHash, expiresAt, deviceID); err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(DefaultAccessTokenLifetime.Seconds()),
		User: &UserInfo{
			ID:         user.ID,
			Name:       user.Fullname,
			Email:      user.Email,
			SystemRole: string(user.SystemRole),
			Role:       "", // no tenant role
		},
	}, nil
}

// RefreshRotates a refresh token: verifies it, revokes the old token,
// creates a new access token + refresh token, and returns both.
// The old token is immediately invalid (rotation).
func (s *Service) Refresh(ctx context.Context, tx *sql.Tx, rawToken, deviceID string) (*LoginResult, error) {
	hash := rawToken // raw token is looked up by its stored hash (caller hashes before passing)
	userID, membershipIDPtr, expiresAt, revoked, _, err := FindRefreshTokenByHash(ctx, tx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}

	// Must not be revoked or expired.
	if revoked || time.Now().After(expiresAt) {
		return nil, ErrInvalidRefreshToken
	}

	// Revoke old token and invalidate any replacements.
	if err := RevokeRefreshTokenByHash(ctx, tx, hash); err != nil {
		return nil, fmt.Errorf("revoke old token: %w", err)
	}

	// Load user for system_role (common to both paths).
	user, err := UsersFindByID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Branch based on whether the refresh token has a membership.
	if membershipIDPtr == nil {
		// --- System-only path: no membership claims ---
		if err := ValidateIdentity(ctx, tx, userID, ""); err != nil {
			return nil, fmt.Errorf("identity invalid during refresh: %w", err)
		}
		// Load RW memberships for system-only super_admin during refresh.
		var rwMemberships []RWMembership
		if userRW, err := loadRWMemberships(ctx, tx, userID); err == nil {
			rwMemberships = userRW
		}
		rwContext := buildRWContextJSON(rwMemberships)

		accessToken, err := GenerateAccessToken(TokenClaims{
			UserID:    userID,
			SysRole:   string(user.SystemRole),
			RWContext: rwContext,
		})
		if err != nil {
			return nil, fmt.Errorf("generate access token: %w", err)
		}

		newToken, err := GenerateRefreshToken()
		if err != nil {
			return nil, fmt.Errorf("generate refresh token: %w", err)
		}
		newHash := newToken
		newExpires := time.Now().Add(s.refreshLifetime)

		// Update replaced_by pointer on the old token (best-effort).
		tx.ExecContext(ctx, `UPDATE refresh_tokens SET replaced_by_hash = $1 WHERE token_hash = $2`, newHash, hash)

		if err := CreateRefreshToken(ctx, tx, userID, nil, newHash, newExpires, deviceID); err != nil {
			return nil, fmt.Errorf("create new refresh token: %w", err)
		}

		return &LoginResult{
			AccessToken:  accessToken,
			RefreshToken: newToken,
			ExpiresIn:    int(DefaultAccessTokenLifetime.Seconds()),
			DeviceID:     deviceID,
			User: &UserInfo{
				ID:         userID,
				Name:       user.Fullname,
				Email:      user.Email,
				SystemRole: string(user.SystemRole),
				Role:       "", // no tenant role
			},
		}, nil
	}

	// --- Member path: existing behavior unchanged ---
	membershipID := *membershipIDPtr
	m, err := RTMembershipFindByID(ctx, tx, membershipID)
	if err != nil {
		return nil, fmt.Errorf("find membership: %w", err)
	}

	// Validate the authenticated identity still exists and is active.
	// This prevents stale refresh tokens from issuing new access tokens
	// for deactivated users or memberships.
	if err := ValidateIdentity(ctx, tx, m.UserID, membershipID); err != nil {
		return nil, fmt.Errorf("identity invalid during refresh: %w", err)
	}

	// Load user for system_role.
	user, err = UsersFindByID(ctx, tx, m.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Regenerate access token with both system and tenant roles.
	// Also include RW memberships loaded from user_rw_memberships.
	var rwMemberships []RWMembership
	if userRW, err := loadRWMemberships(ctx, tx, m.UserID); err == nil {
		rwMemberships = userRW
	}
	rwContext := buildRWContextJSON(rwMemberships)

	accessToken, err := GenerateAccessToken(TokenClaims{
		UserID:    m.UserID,
		MID:       m.ID,
		RTID:      m.RTID,
		Role:      string(m.Role),
		Jabatan:   string(m.Jabatan),
		SysRole:   string(user.SystemRole),
		RWContext: rwContext,
	})
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// New refresh token.
	newToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	newHash := newToken
	newExpires := time.Now().Add(s.refreshLifetime)

	// Update replaced_by pointer on the old token (best-effort).
	tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET replaced_by_hash = $1 WHERE token_hash = $2`,
		newHash, hash,
	)

	if err := CreateRefreshToken(ctx, tx, m.UserID, &m.ID, newHash, newExpires, deviceID); err != nil {
		return nil, fmt.Errorf("create new refresh token: %w", err)
	}

	// Get RT name and details.
	rtName, rtCode, rtRW, _, err := FindRTDetailsByID(ctx, tx, m.RTID)
	if err != nil {
		rtName = ""
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: newToken,
		ExpiresIn:    int(DefaultAccessTokenLifetime.Seconds()),
		DeviceID:     deviceID,
		User: &UserInfo{
			ID:         m.UserID,
			Name:       "",
			Email:      user.Email,
			SystemRole: string(user.SystemRole),
			Role:       m.Role,
			Jabatan:    string(m.Jabatan),
			RT: &RTInfo{
				ID:   m.RTID,
				Name: rtName,
				RT:   rtCode,
				RW:   rtRW,
			},
		},
	}, nil
}

// Logout revokes all refresh tokens for the given membership.
func (s *Service) Logout(ctx context.Context, tx *sql.Tx, membershipID string) error {
	if err := RevokeRefreshTokensByMembership(ctx, tx, membershipID); err != nil {
		return fmt.Errorf("revoke tokens for membership: %w", err)
	}
	return nil
}

// SystemLogout revokes all refresh tokens for a system-only user (no RT membership).
// It revokes tokens by user_id, covering both member and system-only tokens.
func (s *Service) SystemLogout(ctx context.Context, tx *sql.Tx, userID string) error {
	if err := RevokeRefreshTokensByUserID(ctx, tx, userID); err != nil {
		return fmt.Errorf("revoke tokens for user: %w", err)
	}
	return nil
}

// GetProfile returns the authenticated user's profile for the /me endpoint.
func (s *Service) GetProfile(ctx context.Context, tx *sql.Tx, userID, membershipID string) (*UserForMe, error) {
	result, err := GetProfile(ctx, tx, userID, membershipID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetSystemProfile returns the profile for a system-only SUPER_ADMIN without RT membership.
func (s *Service) GetSystemProfile(ctx context.Context, tx *sql.Tx, userID string) (*UserForMe, error) {
	result, err := GetSystemProfile(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateProfile updates the profile of the authenticated user.
func (s *Service) UpdateProfile(ctx context.Context, tx *sql.Tx, userID, fullName string, rawPhone, rawEmail *string) error {
	trimmedName := strings.TrimSpace(fullName)
	var canonPhone *string
	if rawPhone != nil && strings.TrimSpace(*rawPhone) != "" {
		p, err := NormalizePhone(*rawPhone)
		if err != nil {
			return fmt.Errorf("invalid phone number: %w", err)
		}
		canonPhone = &p
	}

	var canonEmail *string
	if rawEmail != nil && strings.TrimSpace(*rawEmail) != "" {
		em := strings.ToLower(strings.TrimSpace(*rawEmail))
		canonEmail = &em
	}

	return UpdateUserProfile(ctx, tx, userID, trimmedName, canonPhone, canonEmail)
}

// ChangePassword changes the password of the authenticated user after verifying current password.
func (s *Service) ChangePassword(ctx context.Context, tx *sql.Tx, userID, currentPassword, newPassword string) error {
	if strings.TrimSpace(currentPassword) == "" {
		return fmt.Errorf("current password is required")
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	currentHash, err := GetUserPasswordHash(ctx, tx, userID)
	if err != nil {
		return err
	}

	if err := Verify(currentPassword, currentHash); err != nil {
		return ErrInvalidPassword
	}

	newHash, err := Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return UpdateUserPassword(ctx, tx, userID, newHash)
}
