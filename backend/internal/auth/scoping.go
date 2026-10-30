package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	httpx "social-finance/internal/http"
	"social-finance/internal/database"
)

// ScopingConfig holds the validated allowlists for RT/RW scoping.
// SCOPED_RT_IDS is **mandatory** at startup. If missing or blank the
// application refuses to start (fail-closed).
//
// SCOPED_RW_IDS is **optional**. When empty the application starts normally
// and all RW endpoints remain closed (fail-closed): no SA can access any RW,
// and no RW membership grants implicit access. When non-empty, every UUID
// must be valid and is treated as an allowlist.
type ScopingConfig struct {
	RTIDs []string // validated UUIDs for RT allowlist (mandatory)
	RWIDs []string // validated UUIDs for RW allowlist (optional, may be nil/empty)
}

// isValidUUID checks if a string is a valid UUID v4 format.
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

// parseUUIDList splits a comma-separated string and validates every entry
// as a UUID. Returns a nil/empty slice on blank input (caller decides
// whether that is an error).
func parseUUIDList(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue // skip empty parts between commas
		}
		if !isValidUUID(p) {
			return nil, fmt.Errorf("invalid UUID %q in list", p)
		}
		result = append(result, p)
	}
	return result, nil
}

// LoadScoping reads SCOPED_RT_IDS and SCOPED_RW_IDS from environment
// variables, validates UUID format, and returns a ScopingConfig.
//
// SCOPED_RT_IDS is **mandatory**:
//   - If missing or blank → error (application must refuse to start)
//
// SCOPED_RW_IDS is **optional**:
//   - If missing or blank → RWIDs remains nil/empty; RW endpoints
//     stay closed (fail-closed). No SA gains RW access.
//   - If non-empty but contains invalid UUIDs → error (fail-closed).
//   - If non-empty and all valid → RWIDs is populated as an allowlist.
//
// This ensures fail-closed behaviour regardless of configuration.
func LoadScoping() (ScopingConfig, error) {
	rtRaw := os.Getenv("SCOPED_RT_IDS")
	rwRaw := os.Getenv("SCOPED_RW_IDS")

	rtIDs, err := parseUUIDList(rtRaw)
	if err != nil {
		return ScopingConfig{}, fmt.Errorf("SCOPED_RT_IDS: %w", err)
	}
	if len(rtIDs) == 0 {
		return ScopingConfig{}, fmt.Errorf("SCOPED_RT_IDS is mandatory and must contain at least one valid UUID")
	}

	// RW IDs are optional. An empty or unset value results in an empty
	// allowlist, which makes RequireRWScope reject all RW access (fail-closed).
	rwIDs, err := parseUUIDList(rwRaw)
	if err != nil {
		return ScopingConfig{}, fmt.Errorf("SCOPED_RW_IDS: %w", err)
	}

	return ScopingConfig{RTIDs: rtIDs, RWIDs: rwIDs}, nil
}

// RequireRWScope is a middleware that checks whether the authenticated user
// has an active RW membership for the specified rwId (from URL path).
//
// Deployment-level gate:
//   - If SCOPED_RW_IDS is empty, ALL RW requests are rejected (fail-closed).
//   - This applies to every user including system-level SUPER_ADMIN.
//
// For users WITH RWContext in their JWT: validates rwId is in AuthContext.RWMemberships.
// For users WITHOUT RWContext: returns 403 Forbidden (fail-closed).
// For system-only super_admin: additionally checks DB is_active status.
//
// Usage:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(auth.RequireRWScope(pool, scopingConfig))
//	    r.Get("/rw/{rwId}/beranda", getBeranda)
//	})
func RequireRWScope(pool *database.Pool, cfg ScopingConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(AuthContextKey).(*AuthContext)
			if !ok || ac == nil {
				httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
				return
			}

			// Extract rwId from URL path — must come from URL, not client body.
			// We use raw URL path parsing to avoid chi-specific deps in this middleware.
			// Expected pattern: /api/v1/rw/{rwId}/...
			rwID := extractRWIDFromPath(r)
			if rwID == "" {
				httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required in URL path")
				return
			}

			// ── Deployment-level gate ─────────────────────────────────
			// If SCOPED_RW_IDS is empty (RT-only deployment), no RW access.
			// This check applies to ALL users including SA.
			if !cfg.hasRW(rwID) {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw not in configured scope")
				return
			}

			// ── System-only SUPER_ADMIN ───────────────────────────────
			if ac.SystemRole == SystemRoleSuperAdmin && ac.RTID == "" && ac.MembershipID == "" {
				// SA must have at least one RW membership in their JWT context.
				if len(ac.RWMemberships) == 0 {
					httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access requires rw membership")
					return
				}

				// Verify RW exists and is active in DB.
				// If pool is nil, fail closed — cannot verify RW existence.
				if pool == nil {
					httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "database unavailable")
					return
				}
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				tx, err := pool.BeginTx(ctx)
				if err != nil {
					httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
					return
				}
				var isActive bool
				err = tx.QueryRowContext(ctx, `SELECT is_active FROM rws WHERE id = $1 LIMIT 1`, rwID).Scan(&isActive)
				tx.Rollback()
				if err == sql.ErrNoRows || !isActive {
					httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw not found or inactive")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// ── Users WITH RW memberships ─────────────────────────────
			// Must have at least one RW membership AND the specific rwId must match.
			if len(ac.RWMemberships) == 0 {
				httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access requires active rw membership")
				return
			}

			for _, rm := range ac.RWMemberships {
				if rm.RWID == rwID {
					next.ServeHTTP(w, r)
					return
				}
			}

			// rwId not in validated memberships.
			httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access denied")
		})
	}
}

// hasRW checks if the given rwId is in the SCOPED_RW_IDS allowlist.
func (c ScopingConfig) hasRW(rwID string) bool {
	for _, id := range c.RWIDs {
		if id == rwID {
			return true
		}
	}
	return false
}

// hasRT checks if the given rtId is in the SCOPED_RT_IDS allowlist.
func (c ScopingConfig) hasRT(rtID string) bool {
	for _, id := range c.RTIDs {
		if id == rtID {
			return true
		}
	}
	return false
}

// extractRWIDFromPath extracts the rwId segment from the URL path.
// Handles paths like /api/v1/rw/{rwId}/... or just /rw/{rwId}/...
func extractRWIDFromPath(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Handle /api/v1/rw/{rwId}/...
	for i, seg := range strings.Split(path, "/") {
		if seg == "rw" && i+1 < len(strings.Split(path, "/")) {
			return strings.Split(path, "/")[i+1]
		}
	}
	return ""
}

// rwPermissionMap defines RW-specific permissions mapped by jabatan.
// Jabatan uses RT-style values ('ketua', 'wakil_ketua', 'sekretaris', 'bendahara').
// Permission codes use the existing 'finance' domain.
//
// This map is HARD-CODED to avoid modifying the database or position_permissions
// table (which has a strict CHECK constraint with only 8 RT jabatan values).
var rwPermissionMap = map[string][]string{
	"ketua": {
		"finance.read",
		"finance.create",
		"finance.update",
		"finance.approve",
	},
	"wakil_ketua": {
		"finance.read",
		"finance.create",
		// TIDAK punya finance.update — tidak bisa transfer, tidak bisa update transaksi
		"finance.approve",
	},
	"sekretaris": {
		"finance.read",
	},
	"bendahara": {
		"finance.read",
		"finance.create",
		"finance.update",
		"finance.approve",
	},
}

// hasRWScopePermission checks whether the given RW jabatan grants the specified
// permission code. Returns false if the jabatan has no entry in rwPermissionMap.
func hasRWScopePermission(jabatan string, permissionCode string) bool {
	perms, ok := rwPermissionMap[jabatan]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == permissionCode {
			return true
		}
	}
	return false
}
