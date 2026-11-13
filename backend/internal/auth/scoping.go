package auth

import (
	"context"
	"database/sql"
	"errors"
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

// Sentinel errors for SA scope validation. Middleware uses errors.Is() to
// distinguish authorization denials (403) from infrastructure failures (500).
var (
	// ErrRWScopeInvalid means the request was denied for authorization reasons:
	// config unavailable, RW not in allowlist, RW not found, or RW inactive.
	ErrRWScopeInvalid = errors.New("rw access denied")

	// ErrRWInfrastructure means the validation could not proceed due to an
	// internal problem (nil pool, database query failure).
	ErrRWInfrastructure = errors.New("rw access denied: infrastructure error")
)

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
// System-level SUPER_ADMIN bypass:
//   - A user with SystemRole == super_admin is allowed to access any RW
//     that is in the configured SCOPED_RW_IDS allowlist, regardless of
//     whether they have an RW membership record.
//   - The RW ID must still exist and be active in the database.
//
// For users WITHOUT super_admin system role:
//   - Must have an active RW membership whose rwId matches the requested scope.
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

			// ── System-level SUPER_ADMIN bypass ────────────────────────
			// Super Admin does NOT require RW membership to access RW endpoints.
			// They are authorized by their system role alone, subject to:
			//   1. The RW ID must be in the configured SCOPED_RW_IDS (checked above).
			//   2. The RW must exist and be active in the database.
			if ac.SystemRole == SystemRoleSuperAdmin {
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				if err := ValidateSAForRW(ctx, &cfg, pool, rwID); err != nil {
					// Distinguish authorization denials (403) from
					// infrastructure/database failures (500) using typed errors.
					if errors.Is(err, ErrRWInfrastructure) {
						httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "database unavailable")
						return
					}
					httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw not found or inactive")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// ── Users WITHOUT super_admin system role ──────────────────
			// Must have an active RW membership whose rwId matches the requested scope.
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

// ValidateSAForRW validates Super Admin access to a specific RW.
// It checks: (1) SA identity, (2) rwID in SCOPED_RW_IDS, (3) RW exists in DB,
// (4) RW is active. Returns nil if all checks pass, error otherwise.
//
// This function is shared between the RequireRWScope middleware and rw.Service
// to prevent divergent validation logic between HTTP and direct service calls.
//
// Parameters:
//   - ctx: request context for timeout/cancellation.
//   - cfg: ScopingConfig holding the SCOPED_RW_IDS allowlist. If nil or empty,
//     no SA can access any RW (fail-closed).
//   - pool: database connection pool. If nil, returns ErrRWInfrastructure.
//   - rwID: the RW identifier to validate against.
//
// Errors:
//   - ErrRWScopeInvalid: config nil/empty, rwID not in SCOPED_RW_IDS,
//     rwID not found in database, or rwID exists but is_active = false.
//   - ErrRWInfrastructure: pool is nil or database query fails.
func ValidateSAForRW(ctx context.Context, cfg *ScopingConfig, pool *database.Pool, rwID string) error {
	// Fail-closed: no config means no RW access.
	if cfg == nil {
		return ErrRWScopeInvalid
	}
	if !cfg.hasRW(rwID) {
		return ErrRWScopeInvalid
	}
	if pool == nil {
		return fmt.Errorf("%w: database unavailable", ErrRWInfrastructure)
	}
	var isActive bool
	err := pool.Raw().QueryRowContext(ctx, `SELECT is_active FROM rws WHERE id = $1 LIMIT 1`, rwID).Scan(&isActive)
	if err == sql.ErrNoRows {
		return ErrRWScopeInvalid
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRWInfrastructure, err)
	}
	if !isActive {
		return ErrRWScopeInvalid
	}
	return nil
}

// RW feature permission codes
const (
	PermissionRWDashboardRead     = "rw.dashboard.read"
	PermissionRWKasRead           = "rw.kas.read"
	PermissionRWKasWrite          = "rw.kas.write"
	PermissionRWLocationRead      = "rw.fund_location.read"
	PermissionRWLocationWrite     = "rw.fund_location.write"
	PermissionRWNotificationRead  = "rw.notification.read"
	PermissionRWNotificationWrite = "rw.notification.write"
)

// rwPermissionMapReadOnly defines RW permissions for RT officials accessing RW
// through inferred membership (rt_inferred). In accordance with the final RBAC matrix:
//   - Beranda RW: READ
//   - KAS RW: NO
//   - Lokasi Dana RW: NO
//   - Notifikasi RW: NO
var rwPermissionMapReadOnly = map[string][]string{
	"ketua":       {PermissionRWDashboardRead},
	"wakil_ketua": {PermissionRWDashboardRead},
	"sekretaris":  {PermissionRWDashboardRead},
	"bendahara":   {PermissionRWDashboardRead},
	"anggota":     {PermissionRWDashboardRead},
	"pengurus":    {PermissionRWDashboardRead},
	"perangkat":   {PermissionRWDashboardRead},
}

// rwPermissionMap defines RW-specific permissions mapped by explicit RW jabatan.
// Strict Final RBAC Matrix:
//   - Ketua RW: Beranda READ, KAS WRITE, Lokasi Dana WRITE, Notifikasi WRITE
//   - Bendahara RW: Beranda READ, KAS WRITE, Lokasi Dana WRITE, Notifikasi WRITE
//   - Pengurus RW (wakil_ketua, sekretaris, etc.): Beranda READ, KAS READ, Lokasi Dana READ, Notifikasi READ (ALL WRITE DENIED)
var rwPermissionMap = map[string][]string{
	"ketua": {
		PermissionRWDashboardRead,
		PermissionRWKasRead,
		PermissionRWKasWrite,
		PermissionRWLocationRead,
		PermissionRWLocationWrite,
		PermissionRWNotificationRead,
		PermissionRWNotificationWrite,
		// Domain/legacy aliases
		"warga.read",
		"warga.create",
		"warga.update",
		"warga.import",
		"warga.export",
		"finance.read",
		"finance.create",
		"finance.update",
		"finance.approve",
		"notification.read",
		"notification.manage",
		"reports.read",
		"rt.settings.read",
		"rt.settings.manage",
	},
	"bendahara": {
		PermissionRWDashboardRead,
		PermissionRWKasRead,
		PermissionRWKasWrite,
		PermissionRWLocationRead,
		PermissionRWLocationWrite,
		PermissionRWNotificationRead,
		PermissionRWNotificationWrite,
		// Domain/legacy aliases
		"finance.read",
		"finance.create",
		"finance.update",
		"finance.approve",
		"notification.read",
		"notification.manage",
	},
	"wakil_ketua": {
		PermissionRWDashboardRead,
		PermissionRWKasRead,
		PermissionRWLocationRead,
		PermissionRWNotificationRead,
		// Domain/legacy aliases (READ ONLY)
		"finance.read",
		"notification.read",
	},
	"sekretaris": {
		PermissionRWDashboardRead,
		PermissionRWKasRead,
		PermissionRWLocationRead,
		PermissionRWNotificationRead,
		// Domain/legacy aliases (READ ONLY)
		"finance.read",
		"notification.read",
	},
	"anggota": {
		PermissionRWDashboardRead,
		PermissionRWKasRead,
		PermissionRWLocationRead,
		PermissionRWNotificationRead,
		// Domain/legacy aliases (READ ONLY)
		"finance.read",
		"notification.read",
	},
	"pengurus": {
		PermissionRWDashboardRead,
		PermissionRWKasRead,
		PermissionRWLocationRead,
		PermissionRWNotificationRead,
		// Domain/legacy aliases (READ ONLY)
		"finance.read",
		"notification.read",
	},
}

// HasRWScopePermission checks whether the given RW jabatan and source
// grants the specified permission code.
//
// Source must be one of:
//   - RWMembershipSourceExplicit  — from user_rw_memberships (RW officials).
//   - RWMembershipSourceRTInferred — from user_rt_memberships + rts.rw_id (RT officials).
//
// RT officials (rt_inferred) may ONLY read Beranda RW (rw.dashboard.read).
// RW officials (explicit): Ketua and Bendahara have full write permissions;
// other RW officials have read-only access across all RW features.
//
// Exported as HasRWScopePermission for use by external packages (e.g., internal/rw).
func HasRWScopePermission(jabatan string, source RWMembershipSource, permissionCode string) bool {
	cleanJabatan := strings.ToLower(strings.TrimSpace(jabatan))
	if cleanJabatan == "" {
		return false
	}
	normalizedCode := normalizeRWPermissionCode(permissionCode)

	if source == RWMembershipSourceRTInferred {
		// RT officials get ONLY Beranda RW read access.
		return hasRWScopePermissionMap(cleanJabatan, normalizedCode, rwPermissionMapReadOnly)
	}
	// RW officials (explicit) use the full permission map.
	perms, ok := rwPermissionMap[cleanJabatan]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == normalizedCode || p == permissionCode {
			return true
		}
	}
	return false
}

func normalizeRWPermissionCode(code string) string {
	switch code {
	case "dashboard.read", "beranda.read":
		return PermissionRWDashboardRead
	case "kas.read":
		return PermissionRWKasRead
	case "kas.write":
		return PermissionRWKasWrite
	case "fund_location.read", "locations.read", "rw.locations.read":
		return PermissionRWLocationRead
	case "fund_location.write", "locations.write", "rw.locations.write":
		return PermissionRWLocationWrite
	case "rw.notification.manage":
		return PermissionRWNotificationWrite
	default:
		return code
	}
}

// hasRWScopePermission checks whether the given RW jabatan grants the specified
// permission code against rwPermissionMap. Returns false if no entry exists.
// This is kept for backward compatibility with existing tests and internal use.
func hasRWScopePermission(jabatan string, permissionCode string) bool {
	return HasRWScopePermission(jabatan, RWMembershipSourceExplicit, permissionCode)
}

// hasRWScopePermissionMap checks permissions against a given permission map.
func hasRWScopePermissionMap(jabatan string, permissionCode string, permMap map[string][]string) bool {
	perms, ok := permMap[jabatan]
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
