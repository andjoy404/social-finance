package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Service implements login, token refresh, logout, and profile lookup.
type Service struct {
	refreshLifetime time.Duration
}

// ServiceOptions configures the Service.
type ServiceOptions struct {
	RefreshLifetime time.Duration
}

// NewService creates an auth service with the given options.
func NewService(opts ...ServiceOptions) *Service {
	lifetime := 7 * 24 * time.Hour // default 7 days
	for _, o := range opts {
		if o.RefreshLifetime > 0 {
			lifetime = o.RefreshLifetime
		}
	}
	return &Service{refreshLifetime: lifetime}
}

// LoginResult holds the tokens and user info returned after successful login.
type LoginResult struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	User         *UserInfo `json:"user"`
}

// UserInfo is the minimal user+RT info returned in login and /me responses.
type UserInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"full_name"`
	Email      string  `json:"email"`
	SystemRole string  `json:"system_role,omitempty"`
	Role       Role    `json:"role"`
	RT         *RTInfo `json:"rt,omitempty"`
}

// RTInfo describes the user's active RT.
type RTInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Login authenticates a user by email + password.
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
func (s *Service) Login(ctx context.Context, tx *sql.Tx, email, rawPassword string) (*LoginResult, error) {
	// Step 1 -- find user by normalized email.
	user, err := UsersFindByEmail(ctx, tx, email)
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
		return nil, ErrMultipleMemberships
	}

	// --- Case: member login ---
	if m != nil {
		// Check membership + RT are active.
		if !m.IsActive {
			return nil, ErrNotAuthorized
		}
		rtName, rtActive, err := FindRTByID(ctx, tx, m.RTID)
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
		accessToken, err := GenerateAccessToken(TokenClaims{
			UserID:  user.ID,
			MID:     m.ID,
			RTID:    m.RTID,
			Role:    string(m.Role),
			SysRole: string(user.SystemRole),
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

		if err := CreateRefreshToken(ctx, tx, user.ID, m.ID, refreshHash, expiresAt); err != nil {
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
				RT: &RTInfo{
					ID:   m.RTID,
					Name: rtName,
				},
			},
		}, nil
	}

	// --- Case: system-only SUPER_ADMIN ---
	accessToken, err := GenerateAccessToken(TokenClaims{
		UserID:  user.ID,
		SysRole: string(user.SystemRole),
	})
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: "", // no refresh token for system-only login
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
func (s *Service) Refresh(ctx context.Context, tx *sql.Tx, rawToken string) (*LoginResult, error) {
	hash := rawToken // raw token is looked up by its stored hash (caller hashes before passing)
	_, membershipID, expiresAt, revoked, err := FindRefreshTokenByHash(ctx, tx, hash)
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

	// Load membership for claims.
	m, err := RTMembershipFindByID(ctx, tx, membershipID)
	if err != nil {
		return nil, fmt.Errorf("find membership: %w", err)
	}

	// Load user for system_role.
	user, err := UsersFindByID(ctx, tx, m.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Regenerate access token with both system and tenant roles.
	accessToken, err := GenerateAccessToken(TokenClaims{
		UserID:  m.UserID,
		MID:     m.ID,
		RTID:    m.RTID,
		Role:    string(m.Role),
		SysRole: string(user.SystemRole),
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

	if err := CreateRefreshToken(ctx, tx, m.UserID, m.ID, newHash, newExpires); err != nil {
		return nil, fmt.Errorf("create new refresh token: %w", err)
	}

	// Get RT name.
	rtName, _, err := FindRTByID(ctx, tx, m.RTID)
	if err != nil {
		rtName = ""
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: newToken,
		ExpiresIn:    int(DefaultAccessTokenLifetime.Seconds()),
		User: &UserInfo{
			ID:         m.UserID,
			Name:       "",
			Email:      user.Email,
			SystemRole: string(user.SystemRole),
			Role:       m.Role,
			RT: &RTInfo{
				ID:   m.RTID,
				Name: rtName,
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
