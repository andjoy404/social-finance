package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	httpx "social-finance/internal/http"
	"social-finance/internal/database"
)

const authContextKey = "auth"

// AuthContextKey is the context key used by handlers to retrieve the principal.
var AuthContextKey = authContextKey

// RequireAuth is a Chi middleware that extracts and validates the Bearer token.
// On success, it attaches an AuthContext to the request context and continues.
// Returns 401 for missing, expired, or invalid tokens.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		if raw == "" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
			return
		}

		parts := strings.SplitN(raw, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid authorization scheme")
			return
		}

		claims, err := ParseAndVerifyAccessToken(parts[1])
		if err != nil {
			switch err {
			case ErrExpiredToken:
				httpx.ErrorJSON(w, http.StatusUnauthorized, "token_expired", "access token expired")
			default:
				httpx.ErrorJSON(w, http.StatusUnauthorized, "invalid_token", "authentication failed")
			}
			return
		}

		// Validate the authenticated identity still exists and is active.
		// This closes the stale-jwt gap: JWT signature verification is
		// cryptographic only — it does not check whether the user or their
		// membership still exists in the database.
		validateCtx, validateCancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer validateCancel()

		// Fail-closed: if AuthDBPool is nil, block the request instead of
		// trusting the JWT blindly. In production AuthDBPool is always set
		// during startup before ListenAndServe() begins; in tests that do not
		// wire the pool, this guard prevents a nil-panic.
		if AuthDBPool == nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "identity validation unavailable")
			return
		}

		tx, err := AuthDBPool.BeginTx(validateCtx)
		if err != nil {
			httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
			return
		}
		if err := ValidateIdentity(validateCtx, tx, claims.UserID, claims.MID); err != nil {
			tx.Rollback()
			if err == ErrNotAuthorized {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "identity_invalid", "user not found or inactive")
			} else {
				httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "identity validation failed")
			}
			return
		}
		tx.Rollback()

		// ── RW context ──────────────────────────────────────────────
		// Parse the RWContext JSON string from the token claims.
		// Empty string (or missing claim) → nil memberships (backward compatible).
		// Invalid JSON → treated as empty (do not crash; handler-level middleware
		// enforces 403 on RW endpoints for missing/empty RWContext).
		var rwContextFromToken []RWMembership
		if claims.RWContext != "" {
			if err := json.Unmarshal([]byte(claims.RWContext), &rwContextFromToken); err != nil {
				// Malformed RWContext — treat as empty, not a hard failure.
				rwContextFromToken = nil
			}
		}

		// Per-request validation: ensure every claimed RW membership is still
		// active in the database. This is the revocation-safe layer — if a
		// user's RW membership is deactivated or revoked, subsequent requests
		// with the same stale JWT will silently drop that membership from the
		// context. The user's existing RT access is unaffected.
		var validRWMemberships []RWMembership
		if len(rwContextFromToken) > 0 {
			// Collect all rw_ids claimed in the token for a single query.
			rwIDs := make(map[string]bool, len(rwContextFromToken))
			for _, m := range rwContextFromToken {
				rwIDs[m.RWID] = true
			}
			ids := make([]string, 0, len(rwIDs))
			for id := range rwIDs {
				ids = append(ids, id)
			}

			if len(ids) > 0 && AuthDBPool != nil {
				// Build parameter placeholders: $1, $2, …
				placeholders := make([]string, len(ids))
				args := make([]interface{}, len(ids))
				for i, id := range ids {
					placeholders[i] = fmt.Sprintf("$%d", i+2) // +2 because $1 is user_id
					args[i] = id
				}
				q := fmt.Sprintf(
					`SELECT rw_id, jabatan FROM user_rw_memberships
					 WHERE user_id = $1 AND rw_id IN (%s) AND is_active = true`,
					strings.Join(placeholders, ","),
				)
				args = append([]interface{}{claims.UserID}, args...)

				rows, err := AuthDBPool.Raw().QueryContext(validateCtx, q, args...)
				if err == nil {
					defer rows.Close()
					validSet := make(map[string]string, len(ids))
					for rows.Next() {
						var rwID, jabatan string
						if err := rows.Scan(&rwID, &jabatan); err == nil {
							validSet[rwID] = jabatan
						}
					}
					for _, tc := range rwContextFromToken {
						if j, ok := validSet[tc.RWID]; ok {
							validRWMemberships = append(validRWMemberships, RWMembership{
								RWID:    tc.RWID,
								Jabatan: j,
							})
						}
					}
				}
			}
		}

		effectiveRTID := claims.RTID
		if SystemRole(claims.SysRole) == SystemRoleSuperAdmin && effectiveRTID == "" {
			reqRT := strings.TrimSpace(r.Header.Get("X-RT-ID"))
			if reqRT == "" {
				reqRT = strings.TrimSpace(r.URL.Query().Get("rt_id"))
			}
			if reqRT != "" && AuthDBPool != nil {
				var validID string
				if err := AuthDBPool.Raw().QueryRowContext(validateCtx, "SELECT id FROM rts WHERE id = $1 AND is_active = true", reqRT).Scan(&validID); err == nil {
					effectiveRTID = validID
				}
			}
		}

		ctx := context.WithValue(r.Context(), AuthContextKey, &AuthContext{
			UserID:          claims.UserID,
			MembershipID:    claims.MID,
			RTID:            effectiveRTID,
			SystemRole:      SystemRole(claims.SysRole),
			TenantRole:      Role(claims.Role),
			Jabatan:         Jabatan(claims.Jabatan),
			RWMemberships:   validRWMemberships,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireSystemRole returns a middleware that checks whether the authenticated
// user's system role is in the allowed set. This is for system-level endpoints
// (e.g. RT management) that only SUPER_ADMIN should access.
//
// A user with SystemRole == super_admin but NO active membership is permitted,
// because system administration does not require tenant membership.
//
// Usage:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(RequireSystemRole(auth.SystemRoleSuperAdmin))
//	    r.Get("/api/v1/rt/:id/users", listUsers)
//	})
func RequireSystemRole(allowed ...SystemRole) func(http.Handler) http.Handler {
	allowedMap := make(map[SystemRole]bool, len(allowed))
	for _, r := range allowed {
		allowedMap[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}
			if !allowedMap[ac.SystemRole] {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns a middleware that checks whether the authenticated user's
// tenant role is in the allowed set for their active membership. Returns 403 if
// not authorised.
//
// This middleware enforces strict scope separation:
//   - A system SUPER_ADMIN is allowed to pass through regardless of tenant role,
//     because SUPER_ADMIN has unrestricted application authorization during
//     development phase.
//   - A SUPER_ADMIN with a valid membership is evaluated by their tenant role
//     within that RT.
//
// Usage:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(RequireRole(RoleBendahara, RolePengurus))
//	    r.Get("/transactions", listTransactions)
//	})
func RequireRole(allowed ...Role) func(http.Handler) http.Handler {
	allowedMap := make(map[Role]bool, len(allowed))
	for _, r := range allowed {
		allowedMap[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}
			// System-level SUPER_ADMIN bypass — unrestricted access during development.
			if ac.SystemRole == SystemRoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}
			// Require a valid tenant context (membership). System-only super_admin
			// has no tenant scope and must be denied tenant access.
			if ac.MembershipID == "" || ac.RTID == "" || ac.TenantRole == "" {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}
			if !allowedMap[ac.TenantRole] {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns a middleware that enforces position-based permissions
// for jabatan-based users. Super admin always passes.
//
// Users WITH a jabatan are evaluated against position_permissions via the
// database pool, scoped to the authenticated user's RTID from JWT claims.
// Users WITHOUT a jabatan pass through without DB query — their access is
// enforced solely by RequireRole middleware, which is stacked after
// RequirePermission on each route.
//
// Usage:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(auth.RequireAuth)
//	    r.Use(auth.RequirePermission(pool, "warga.create"))
//	    r.Post("/api/v1/residents", createResident)
//	})
func RequirePermission(pool *database.Pool, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}

			// Super admin always passes.
			if ac.SystemRole == SystemRoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}

			// All users (jabatan-based and role-based without jabatan) are
			// evaluated by PermissionResolver.HasPermission:
			//   - jabatan-based: checked against position_permissions DB table
			//   - no jabatan: passes through (RequireRole enforces role access)
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			tx, err := pool.BeginTx(ctx)
			if err != nil {
				httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
				return
			}
			defer tx.Rollback()

			pr := NewPermissionResolver(pool)
			has, err := pr.HasPermission(ctx, tx, ac, permission)
			if err != nil {
				httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to check permission")
				return
			}

			if !has {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetAuthContext returns the AuthContext from the request, or nil.
func GetAuthContext(r *http.Request) *AuthContext {
	ac, _ := r.Context().Value(AuthContextKey).(*AuthContext)
	return ac
}

// RequireJabatan returns a middleware that checks whether the authenticated user's
// jabatan is in the allowed set for their active membership. Returns 403 if not
// authorised.
//
// This middleware is for jabatan-based access control (e.g. Ketua RT accessing
// import/export endpoints) that RequireRole cannot handle, because jabatan is
// stored in the JWT jabatan claim while RequireRole only checks the role claim.
//
// System-level SUPER_ADMIN always passes through regardless of jabatan.
// Users WITHOUT a jabatan pass through without DB query — their access is
// enforced solely by RequireRole middleware, which is stacked after RequireJabatan.
//
// Usage:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(auth.RequireAuth)
//	    r.Use(auth.RequireJabatan(pool, auth.JabatanKetua))
//	    r.Post("/api/v1/warga/import/preview/xlsx", handlePreview)
//	})
func RequireJabatan(allowed ...Jabatan) func(http.Handler) http.Handler {
	allowedMap := make(map[Jabatan]bool, len(allowed))
	for _, j := range allowed {
		allowedMap[j] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}
			// System-level SUPER_ADMIN bypass — unrestricted access.
			if ac.SystemRole == SystemRoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}
			// Users without a jabatan pass through — their role is enforced by RequireRole middleware.
			if ac.Jabatan == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !allowedMap[ac.Jabatan] {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAccess returns a middleware that checks whether the authenticated user
// has the required role OR jabatan for their active membership. Returns 403
// if neither matches.
//
// This is used for routes that accept both role-based (e.g. Pengurus, Bendahara)
// and jabatan-based (e.g. Ketua RT with role="warga") access control.
//
// Usage:
//
//	r.Use(auth.RequireAccess(
//	    []auth.Role{auth.RolePengurus, auth.RoleBendahara},
//	    []auth.Jabatan{auth.JabatanKetua, auth.JabatanBendahara},
//	))
func RequireAccess(roles []Role, jabatans []Jabatan) func(http.Handler) http.Handler {
	roleMap := make(map[Role]bool, len(roles))
	for _, r := range roles {
		roleMap[r] = true
	}
	jabatMap := make(map[Jabatan]bool, len(jabatans))
	for _, j := range jabatans {
		jabatMap[j] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}
			// System-level SUPER_ADMIN bypass — unrestricted access.
			if ac.SystemRole == SystemRoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}
			// Role-based check
			if roleMap[ac.TenantRole] {
				next.ServeHTTP(w, r)
				return
			}
			// Jabatan-based check
			if jabatMap[ac.Jabatan] {
				next.ServeHTTP(w, r)
				return
			}
			// Neither role nor jabatan matches — forbidden.
			httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		})
	}
}

// RequireNotificationAdmin allows only SUPER_ADMIN or Ketua RT (jabatan "ketua") to access notification management.
func RequireNotificationAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}
			// System-level SUPER_ADMIN bypass — unrestricted access.
			if ac.SystemRole == SystemRoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}
			// Ketua RT position access
			if ac.Jabatan == JabatanKetua || strings.EqualFold(strings.TrimSpace(string(ac.Jabatan)), string(JabatanKetua)) {
				next.ServeHTTP(w, r)
				return
			}
			httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "only super_admin or ketua rt can access notifications")
		})
	}
}

// RequireAllowedRT returns a middleware that verifies the tenant RT is in the allowed
// list (RT 002 and RT 003). Non-allowed RTs receive 403 Forbidden.
// System-only SUPER_ADMIN without tenant context is permitted for platform-wide management.
func RequireAllowedRT(pool *database.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}

			// System-only super admin with no RT context passes through
			if ac.SystemRole == SystemRoleSuperAdmin && ac.RTID == "" {
				next.ServeHTTP(w, r)
				return
			}

			if ac.RTID == "" {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "tenant context required")
				return
			}

			effectivePool := pool
			if effectivePool == nil {
				effectivePool = AuthDBPool
			}
			if effectivePool == nil {
				httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "database pool unavailable")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			var rtCode string
			var isActive bool
			err := effectivePool.Raw().QueryRowContext(ctx,
				`SELECT rt, is_active FROM rts WHERE id = $1 LIMIT 1`,
				ac.RTID,
			).Scan(&rtCode, &isActive)
			if err != nil {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rt not found or invalid")
				return
			}

			if !isActive || !AllowedRTCodes[rtCode] {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rt is not enabled or not allowed")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AllowedRTCodes defines the set of RT codes that are permitted for operational access.
var AllowedRTCodes = map[string]bool{
	"002": true,
	"003": true,
}
