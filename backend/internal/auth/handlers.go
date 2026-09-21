package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

// LoginRequest is the body of a login POST request.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

// Handler handles auth endpoints (login, refresh, logout, me).
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

// handleLogin processes login requests.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "email and password are required")
		return
	}

	// Normalize email: trim whitespace + lowercase for global uniqueness.
	email := strings.ToLower(strings.TrimSpace(req.Email))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	result, err := h.svc.Login(ctx, tx, email, req.Password)
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
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	ac := GetAuthContext(r)
	if ac == nil || ac.MembershipID == "" {
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

	if err := h.svc.Logout(ctx, tx, ac.MembershipID); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "logout failed")
		return
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

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to load profile")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
