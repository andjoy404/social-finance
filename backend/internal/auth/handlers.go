package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

// LoginRequest is the body of a login POST request.
// Accepts identifier via email, phone, or identifier fields.
type LoginRequest struct {
	Email      string `json:"email"`
	Phone      string `json:"phone,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	Password   string `json:"password"`
}

// LoginResponse is the body of a login 200 response.
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	User         UserInfo `json:"user"`
}

// refreshRequest is the body of a refresh POST request.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// logoutRequest is the body of a logout POST request.
type logoutRequest struct {
	MembershipID string `json:"membership_id"`
}

// UpdateProfileRequest is the body of a profile update PUT request.
type UpdateProfileRequest struct {
	FullName string  `json:"full_name"`
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
}

// ChangePasswordRequest is the body of a password change PUT request.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Handler handles auth endpoints (login, refresh, logout, me, profile, password).
type Handler struct {
	svc *Service
	p   *database.Pool
}

// NewHandler creates a new auth handler.
func NewHandler(pool *database.Pool, svc *Service) *Handler {
	return &Handler{svc: svc, p: pool}
}

// Register mounts the auth routes on the given router group.
func (h *Handler) Register(r authRouter) {
	r.POST("/login", h.handleLogin)
	r.POST("/refresh", h.handleRefresh)
	r.POST("/logout", h.handleLogout)
	r.GET("/me", h.handleMe)
	r.PUT("/profile", h.handleUpdateProfile)
	r.PUT("/password", h.handleChangePassword)
}

// Login returns an HTTP handler for the /login endpoint.
func (h *Handler) Login() http.Handler {
	return http.HandlerFunc(h.handleLogin)
}

// Refresh returns an HTTP handler for the /refresh endpoint.
func (h *Handler) Refresh() http.Handler {
	return http.HandlerFunc(h.handleRefresh)
}

// Logout returns an HTTP handler for the /logout endpoint.
func (h *Handler) Logout() http.Handler {
	return http.HandlerFunc(h.handleLogout)
}

// Me returns an HTTP handler for the /me endpoint.
func (h *Handler) Me() http.Handler {
	return http.HandlerFunc(h.handleMe)
}

// UpdateProfile returns an HTTP handler for the /profile PUT endpoint.
func (h *Handler) UpdateProfile() http.Handler {
	return http.HandlerFunc(h.handleUpdateProfile)
}

// ChangePassword returns an HTTP handler for the /password PUT endpoint.
func (h *Handler) ChangePassword() http.Handler {
	return http.HandlerFunc(h.handleChangePassword)
}

// handleLogin processes login requests.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(req.Phone)
	}
	if identifier == "" || req.Password == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "email or phone and password are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	result, err := h.svc.Login(ctx, tx, identifier, req.Password)
	if err != nil {
		switch err {
		case ErrInvalidPassword, ErrUserNotFound:
			httpx.ErrorJSON(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		case ErrNotAuthorized:
			httpx.ErrorJSON(w, http.StatusUnauthorized, "not_authorized", "account is not authorized to login")
		case ErrMultipleMemberships:
			httpx.ErrorJSON(w, http.StatusMultipleChoices, "multiple_memberships", "user has multiple active memberships")
		default:
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "login failed")
		}
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    result.ExpiresIn,
		User:         *result.User,
	})
}

// handleRefresh processes token refresh requests.
func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "refresh_token is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	result, err := h.svc.Refresh(ctx, tx, req.RefreshToken)
	if err != nil {
		switch err {
		case ErrInvalidRefreshToken:
			httpx.ErrorJSON(w, http.StatusUnauthorized, "invalid_token", "invalid or expired refresh token")
		default:
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "refresh failed")
		}
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "refresh failed")
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    result.ExpiresIn,
		User:         *result.User,
	})
}

// handleLogout processes logout requests.
// For member users (membershipID != "") it revokes tokens scoped to that membership.
// For system-only SUPER_ADMIN (membershipID == "") it revokes all tokens for the user.
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	ac := GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	if ac.MembershipID != "" {
		if err := h.svc.Logout(ctx, tx, ac.MembershipID); err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "logout failed")
			return
		}
	} else {
		if err := h.svc.SystemLogout(ctx, tx, ac.UserID); err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "logout failed")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "logout failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleMe returns the authenticated user's profile.
func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	ac := GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	var profile *UserForMe
	if ac.MembershipID != "" {
		profile, err = h.svc.GetProfile(ctx, tx, ac.UserID, ac.MembershipID)
	} else {
		profile, err = h.svc.GetSystemProfile(ctx, tx, ac.UserID)
	}
	if err != nil {
		switch err {
		case ErrNotAuthorized:
			httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "user profile not found")
		case ErrUserNotFound:
			httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "user profile not found")
		default:
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to load profile")
		}
		return
	}

	profile.EffectivePermissions, err = effectiveIuranPermissions(ctx, tx, h.p, ac)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to load profile permissions")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// handleUpdateProfile updates the user's name, phone, and/or email.
func (h *Handler) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	ac := GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	if strings.TrimSpace(req.FullName) == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "full_name cannot be empty")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	if err := h.svc.UpdateProfile(ctx, tx, ac.UserID, req.FullName, req.Phone, req.Email); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	var profile *UserForMe
	if ac.MembershipID != "" {
		profile, err = h.svc.GetProfile(ctx, tx, ac.UserID, ac.MembershipID)
	} else {
		profile, err = h.svc.GetSystemProfile(ctx, tx, ac.UserID)
	}
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to reload profile")
		return
	}

	profile.EffectivePermissions, err = effectiveIuranPermissions(ctx, tx, h.p, ac)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to load profile permissions")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// handleChangePassword verifies current password and updates to new password.
func (h *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	ac := GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	if strings.TrimSpace(req.CurrentPassword) == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "current_password is required")
		return
	}
	if len(req.NewPassword) < 8 {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "new_password must be at least 8 characters")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	if err := h.svc.ChangePassword(ctx, tx, ac.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "invalid_credentials", "Kata sandi saat ini tidak sesuai")
			return
		}
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Password updated successfully",
	})
}

func effectiveIuranPermissions(ctx context.Context, tx *sql.Tx, pool *database.Pool, ac *AuthContext) ([]string, error) {
	permissions := make([]string, 0, 4)
	roleAllowsRead := ac.TenantRole == RoleWarga ||
		ac.TenantRole == RoleBendahara ||
		ac.TenantRole == RolePengurus
	roleAllowsManagement := ac.TenantRole == RoleBendahara ||
		ac.TenantRole == RolePengurus ||
		ac.Jabatan == JabatanKetua ||
		ac.Jabatan == JabatanBendahara ||
		ac.Jabatan == JabatanSekretaris

	checks := []struct {
		code       string
		roleAllows bool
	}{
		{code: "finance.read", roleAllows: roleAllowsRead},
		{code: "finance.create", roleAllows: roleAllowsManagement},
		{code: "finance.update", roleAllows: roleAllowsManagement},
		{code: "finance.approve", roleAllows: roleAllowsManagement},
	}

	resolver := NewPermissionResolver(pool)
	for _, check := range checks {
		// Jabatan-based users always go through the PermissionResolver to verify
		// explicit position_permissions grants. Role-based users without jabatan
		// are gated by roleAllows and then pass through the resolver (which
		// returns true for role-based users without jabatan).
		if ac.SystemRole != SystemRoleSuperAdmin && !check.roleAllows && ac.Jabatan == "" {
			continue
		}
		allowed, err := resolver.HasPermission(ctx, tx, ac, check.code)
		if err != nil {
			return nil, fmt.Errorf("resolve effective permission %s: %w", check.code, err)
		}
		if allowed {
			permissions = append(permissions, check.code)
		}
	}
	return permissions, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
