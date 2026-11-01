package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
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
	DeviceID   string `json:"device_id,omitempty"`
}

// LoginResponse is the body of a login 200 response.
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	DeviceID     string   `json:"device_id,omitempty"`
	User         UserInfo `json:"user"`
}

// refreshRequest is the body of a refresh POST request.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID   string `json:"device_id,omitempty"`
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

// ResetPasswordWithCodeRequest defines the body for resetting password with 4-digit code.
type ResetPasswordWithCodeRequest struct {
	Identifier      string `json:"identifier"`
	Code            string `json:"code"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Handler handles auth endpoints (login, refresh, logout, me, profile, password).
type Handler struct {
	svc        *Service
	p          *database.Pool
	scopingCfg *ScopingConfig
}

// NewHandler creates a new auth handler with optional scoping configuration.
func NewHandler(pool *database.Pool, svc *Service, scopingCfg ...*ScopingConfig) *Handler {
	var sc *ScopingConfig
	if len(scopingCfg) > 0 {
		sc = scopingCfg[0]
	}
	return &Handler{svc: svc, p: pool, scopingCfg: sc}
}

// SetScopingConfig updates the scoping configuration on the handler.
func (h *Handler) SetScopingConfig(cfg *ScopingConfig) {
	h.scopingCfg = cfg
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

// ResetPasswordWithCode returns an HTTP handler for POST /api/v1/auth/reset-password-with-code
func (h *Handler) ResetPasswordWithCode() http.Handler {
	return http.HandlerFunc(h.handleResetPasswordWithCode)
}

func (h *Handler) handleResetPasswordWithCode(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordWithCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "format data tidak valid")
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	code := strings.TrimSpace(req.Code)
	newPass := strings.TrimSpace(req.NewPassword)
	confirmPass := strings.TrimSpace(req.ConfirmPassword)

	if identifier == "" || code == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "email/nomor telepon dan kode reset wajib diisi")
		return
	}

	if len(code) != 4 || !regexp.MustCompile(`^[0-9]{4}$`).MatchString(code) {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "kode reset harus berupa 4 digit angka")
		return
	}

	if len(newPass) < 8 {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "kata sandi baru minimal 8 karakter")
		return
	}

	if newPass != confirmPass {
		httpx.ErrorJSON(w, http.StatusBadRequest, "validation_error", "konfirmasi kata sandi tidak cocok")
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

	// Normalize identifier: lower for email or normalized phone
	lowerEmail := strings.ToLower(identifier)
	canonPhone := ""
	if cp, err := NormalizePhone(identifier); err == nil {
		canonPhone = cp
	}

	var userID string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM users
		WHERE (lower(email) = $1 OR ($2 != '' AND phone = $2)) AND is_active = true
		LIMIT 1
	`, lowerEmail, canonPhone).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.ErrorJSON(w, http.StatusBadRequest, "invalid_credentials", "identitas atau kode reset tidak valid")
		return
	}
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	// Fetch latest active reset code for this user
	var codeID, codeHash, codeRTID string
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT id, code_hash, rt_id, expires_at
		FROM resident_password_reset_codes
		WHERE user_id = $1 AND used_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, userID).Scan(&codeID, &codeHash, &codeRTID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.ErrorJSON(w, http.StatusBadRequest, "invalid_code", "kode reset tidak ditemukan atau sudah kadaluarsa")
		return
	}
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	// Verify code
	if err := VerifyCode(code, codeHash); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "invalid_code", "kode reset tidak sesuai")
		return
	}

	// Hash new password
	newHash, err := Hash(newPass)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to hash password")
		return
	}

	// Update user password
	if err := UpdateUserPassword(ctx, tx, userID, newHash); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to update password")
		return
	}

	// Mark code as used
	_, err = tx.ExecContext(ctx, `
		UPDATE resident_password_reset_codes
		SET used_at = now()
		WHERE id = $1
	`, codeID)
	if err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to mark code used")
		return
	}

	// Revoke refresh tokens
	_, err = tx.ExecContext(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	if err != nil {
		slog.Error("revoke tokens err", "error", err)
	}

	// Audit log
	clientIP := ClientIP(r)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (rt_id, user_id, action, entity_type, entity_id, new_values, ip_address)
		VALUES ($1, $2, 'reset_password_with_code', 'user', $3, '{"method": "4_digit_code"}', $4)
	`, codeRTID, userID, userID, clientIP)
	if err != nil {
		slog.Error("audit log err", "error", err)
	}

	if err := tx.Commit(); err != nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Kata sandi berhasil diubah. Silakan masuk dengan kata sandi baru.",
	})
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

	result, err := h.svc.Login(ctx, tx, identifier, req.Password, req.DeviceID)
	if err != nil {
		slog.Error("login failed", "error", err, "identifier", identifier, "has_password", req.Password != "")
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

	result, err := h.svc.Refresh(ctx, tx, req.RefreshToken, req.DeviceID)
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
		if err == nil && profile != nil && profile.SystemRole == string(SystemRoleSuperAdmin) {
			targetRTID := ac.RTID
			var rtID, rtName, rtCode string
			var rtRW int
			var rwID, rwName sql.NullString
			var errRT error

			if targetRTID != "" {
				errRT = tx.QueryRowContext(ctx, `
					SELECT r.id, r.name, r.rw, r.rt, r.rw_id, w.name AS rw_name
					FROM rts r
					LEFT JOIN rws w ON w.id = r.rw_id
					WHERE r.id = $1 AND r.is_active = true
				`, targetRTID).Scan(&rtID, &rtName, &rtRW, &rtCode, &rwID, &rwName)
			} else {
				errRT = tx.QueryRowContext(ctx, `
					SELECT r.id, r.name, r.rw, r.rt, r.rw_id, w.name AS rw_name
					FROM rts r
					LEFT JOIN rws w ON w.id = r.rw_id
					WHERE r.is_active = true AND (r.id = 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b' OR lower(r.name) = 'urena' OR (r.name ILIKE '%urena%' AND r.rt = '002'))
					ORDER BY
						CASE
							WHEN r.id = 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b' THEN 1
							WHEN lower(r.name) = 'urena' AND r.rt = '002' THEN 2
							WHEN lower(r.name) = 'urena' THEN 3
							WHEN r.name ILIKE '%urena%' AND r.rt = '002' THEN 4
							ELSE 5
						END ASC
					LIMIT 1
				`).Scan(&rtID, &rtName, &rtRW, &rtCode, &rwID, &rwName)
			}

			if errRT == nil {
				profile.RTID = rtID
				profile.RTName = rtName
				profile.RT_rw = rtRW
				profile.RT_rt = rtCode
				profile.TenantRole = Role(SystemRoleSuperAdmin)
				if rwID.Valid {
					profile.RWID = &rwID.String
				}
				if rwName.Valid {
					profile.RWName = &rwName.String
				}
			}
		}
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

	// Populate scoped RW memberships for Super Admin.
	// Super Admin does not hold user_rw_memberships rows; their access
	// is governed by SCOPED_RW_IDS. Active RWs in scope are exposed so that
	// Super Admin can navigate and switch contexts among authorized RWs.
	if profile != nil && profile.SystemRole == string(SystemRoleSuperAdmin) {
		var scopedRWIDs []string
		if h.scopingCfg != nil && len(h.scopingCfg.RWIDs) > 0 {
			scopedRWIDs = h.scopingCfg.RWIDs
		} else if rawEnv := os.Getenv("SCOPED_RW_IDS"); rawEnv != "" {
			if parsed, errParse := parseUUIDList(rawEnv); errParse == nil {
				scopedRWIDs = parsed
			}
		}

		if len(scopedRWIDs) > 0 {
			placeholders := make([]string, len(scopedRWIDs))
			args := make([]interface{}, len(scopedRWIDs))
			for i, id := range scopedRWIDs {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
				args[i] = id
			}
			q := fmt.Sprintf(`
				SELECT id, rw_number, name
				FROM rws
				WHERE id IN (%s) AND is_active = true
				ORDER BY rw_number ASC, name ASC
			`, strings.Join(placeholders, ","))

			rows, errScoped := tx.QueryContext(ctx, q, args...)
			if errScoped == nil {
				defer rows.Close()
				var saMemberships []RWMembership
				for rows.Next() {
					var sID string
					var sNumber int
					var sName string
					if errScan := rows.Scan(&sID, &sNumber, &sName); errScan == nil {
						saMemberships = append(saMemberships, RWMembership{
							RWID:     sID,
							RWNumber: sNumber,
							RWName:   sName,
							Jabatan:  "super_admin",
						})
					}
				}
				if len(saMemberships) > 0 {
					profile.RWMemberships = saMemberships
					found := false
					if profile.RWID != nil && *profile.RWID != "" {
						for _, m := range saMemberships {
							if m.RWID == *profile.RWID {
								found = true
								break
							}
						}
					}
					if !found {
						profile.RWID = &saMemberships[0].RWID
						profile.RWName = &saMemberships[0].RWName
					}
				}
			}
		}
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
		if err == nil && profile != nil && profile.SystemRole == string(SystemRoleSuperAdmin) {
			targetRTID := ac.RTID
			var rtID, rtName, rtCode string
			var rtRW int
			var rwID, rwName sql.NullString
			var errRT error

			if targetRTID != "" {
				errRT = tx.QueryRowContext(ctx, `
					SELECT r.id, r.name, r.rw, r.rt, r.rw_id, w.name AS rw_name
					FROM rts r
					LEFT JOIN rws w ON w.id = r.rw_id
					WHERE r.id = $1 AND r.is_active = true
				`, targetRTID).Scan(&rtID, &rtName, &rtRW, &rtCode, &rwID, &rwName)
			} else {
				errRT = tx.QueryRowContext(ctx, `
					SELECT r.id, r.name, r.rw, r.rt, r.rw_id, w.name AS rw_name
					FROM rts r
					LEFT JOIN rws w ON w.id = r.rw_id
					WHERE r.is_active = true AND (r.id = 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b' OR lower(r.name) = 'urena' OR (r.name ILIKE '%urena%' AND r.rt = '002'))
					ORDER BY
						CASE
							WHEN r.id = 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b' THEN 1
							WHEN lower(r.name) = 'urena' AND r.rt = '002' THEN 2
							WHEN lower(r.name) = 'urena' THEN 3
							WHEN r.name ILIKE '%urena%' AND r.rt = '002' THEN 4
							ELSE 5
						END ASC
					LIMIT 1
				`).Scan(&rtID, &rtName, &rtRW, &rtCode, &rwID, &rwName)
			}

			if errRT == nil {
				profile.RTID = rtID
				profile.RTName = rtName
				profile.RT_rw = rtRW
				profile.RT_rt = rtCode
				profile.TenantRole = Role(SystemRoleSuperAdmin)
				if rwID.Valid {
					profile.RWID = &rwID.String
				}
				if rwName.Valid {
					profile.RWName = &rwName.String
				}
			}
		}
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

	// Populate scoped RW memberships for Super Admin.
	// Super Admin does not hold user_rw_memberships rows; their access
	// is governed by SCOPED_RW_IDS. Active RWs in scope are exposed so that
	// Super Admin can navigate and switch contexts among authorized RWs.
	if profile != nil && profile.SystemRole == string(SystemRoleSuperAdmin) {
		var scopedRWIDs []string
		if h.scopingCfg != nil && len(h.scopingCfg.RWIDs) > 0 {
			scopedRWIDs = h.scopingCfg.RWIDs
		} else if rawEnv := os.Getenv("SCOPED_RW_IDS"); rawEnv != "" {
			if parsed, errParse := parseUUIDList(rawEnv); errParse == nil {
				scopedRWIDs = parsed
			}
		}

		if len(scopedRWIDs) > 0 {
			placeholders := make([]string, len(scopedRWIDs))
			args := make([]interface{}, len(scopedRWIDs))
			for i, id := range scopedRWIDs {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
				args[i] = id
			}
			q := fmt.Sprintf(`
				SELECT id, rw_number, name
				FROM rws
				WHERE id IN (%s) AND is_active = true
				ORDER BY rw_number ASC, name ASC
			`, strings.Join(placeholders, ","))

			rows, errScoped := tx.QueryContext(ctx, q, args...)
			if errScoped == nil {
				defer rows.Close()
				var saMemberships []RWMembership
				for rows.Next() {
					var sID string
					var sNumber int
					var sName string
					if errScan := rows.Scan(&sID, &sNumber, &sName); errScan == nil {
						saMemberships = append(saMemberships, RWMembership{
							RWID:     sID,
							RWNumber: sNumber,
							RWName:   sName,
							Jabatan:  "super_admin",
						})
					}
				}
				if len(saMemberships) > 0 {
					profile.RWMemberships = saMemberships
					found := false
					if profile.RWID != nil && *profile.RWID != "" {
						for _, m := range saMemberships {
							if m.RWID == *profile.RWID {
								found = true
								break
							}
						}
					}
					if !found {
						profile.RWID = &saMemberships[0].RWID
						profile.RWName = &saMemberships[0].RWName
					}
				}
			}
		}
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
		ac.Jabatan == JabatanWakilKetua ||
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
