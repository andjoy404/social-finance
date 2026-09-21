package auth

import (
	"context"
	"net/http"
	"strings"

	httpx "social-finance/internal/http"
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

		ctx := context.WithValue(r.Context(), AuthContextKey, &AuthContext{
			UserID:       claims.UserID,
			MembershipID: claims.MID,
			RTID:         claims.RTID,
			SystemRole:   SystemRole(claims.SysRole),
			TenantRole:   Role(claims.Role),
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

// GetAuthContext returns the AuthContext from the request, or nil.
func GetAuthContext(r *http.Request) *AuthContext {
	ac, _ := r.Context().Value(AuthContextKey).(*AuthContext)
	return ac
}
