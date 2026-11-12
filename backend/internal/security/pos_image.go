package security

import (
	"io"
	"net/http"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	httpx "social-finance/internal/http"
)

// Device-scoped resident avatar access.
//
// The Pos device kiosk needs to render resident profile photos (same source
// as every other Warga menu) without a user JWT. The device identity is bound
// to exactly one RT, so access is narrower than the user-image pipeline:
//   - key must live under `community/{deviceRT}/profile/`;
//   - key must reference an active resident avatar in that RT;
//   - read-only, warga-visibility only (no other categories).

// validPosAvatarKey enforces a strict, filesystem-safe key shape.
func validPosAvatarKey(key string) bool {
	segs := strings.Split(key, "/")
	if len(segs) != 4 {
		return false
	}
	if segs[0] != "community" || segs[2] != "profile" {
		return false
	}
	if _, err := uuid.Parse(segs[1]); err != nil {
		return false
	}
	for _, seg := range segs {
		if seg == "" {
			return false
		}
		for _, r := range seg {
			ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
			if !ok {
				return false
			}
		}
	}
	return true
}

// deviceImageContentType mirrors the extension-derived fallback used elsewhere.
func deviceImageContentType(key, fallback string) string {
	if fallback != "" {
		return fallback
	}
	lower := strings.ToLower(key)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// PosImage serves resident avatar bytes to an authenticated Pos device,
// strictly scoped to the device's bound RT and to resident avatar keys.
func (h *Handler) PosImage(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	key := chi.URLParam(r, "*")
	if key == "" {
		key = strings.TrimPrefix(r.URL.Path, "/api/v1/security-device/images/")
	}
	key = strings.TrimPrefix(key, "/")
	prefix := "community/" + dc.RTID + "/profile/"
	if !strings.HasPrefix(key, prefix) || !validPosAvatarKey(key) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "image not accessible")
		return
	}
	if h.store == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "service_unavailable", "image store unavailable")
		return
	}

	var referenced int
	ctx := r.Context()
	if err := h.pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM residents WHERE rt_id = $1 AND is_active = true AND avatar_path = $2`,
		dc.RTID, key,
	).Scan(&referenced); err != nil || referenced == 0 {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "image not found")
		return
	}

	reader, err := h.store.GetCommunityImage(ctx, key)
	if err != nil || reader == nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	defer func() { _ = reader.Body.Close() }()

	w.Header().Set("Content-Type", deviceImageContentType(key, reader.ContentType))
	_, _ = io.Copy(w, reader.Body)
}
