package community

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"social-finance/internal/auth"
)

// Image access model:
//
//   - Bearer mode (API clients, Flutter via Dio): the caller is authenticated
//     through the normal JWT pipeline, must hold a valid RT context in an
//     allowed+active RT, the key must live under community/{rtID}/, and the
//     key must be referenced by content the caller may see (drafts only for
//     content managers, published/active in-window content for everyone else).
//   - Signed-URL mode (Web <img> tags cannot send Authorization headers):
//     the URL is issued by SignImageURL after the SAME authorization check
//     that applies to warga. The signature binds the key and a short expiry
//     (15 minutes). On read, the handler REVALIDATES against the current
//     content state at WARGA visibility level — a signed URL can never
//     serve draft content, no matter who issued it, so a leaked manager
//     URL confers no manager privilege to the holder. Archived/cancelled/
//     expired content stops serving immediately, even before the URL
//     expires. Verification still enforces prefix/traversal rules.
//     Content managers viewing draft/archived content use the Bearer mode
//     (apiFetchCommunityImageBlob on the frontend), never signed URLs.
//
// In both modes, unreferenced keys and keys of other RTs are rejected, and
// the storage layer refuses anything outside the community/ prefix.

// 15 minutes: long enough for <img> rendering and short-lived lists, but
// short enough that a leaked URL expires quickly. The signed URL is a
// bearer credential and cannot be individually revoked; the effective
// window is bounded by this TTL plus content revalidation at read time.
const imageSignatureLifetime = 15 * time.Minute

var imageAllowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// validateCommunityImageKey ensures key is a well-formed community image key
// for rtID: community/{rtID}/{category}/{file}{image-ext}. It rejects path
// traversal, backslashes, NUL bytes, leftover URL-encoding, and non-image
// extensions so the storage layer is never handed a hostile key.
func validateCommunityImageKey(key, rtID string) bool {
	if key == "" || rtID == "" {
		return false
	}
	if strings.ContainsAny(key, "\x00\\%") {
		return false
	}
	// No traversal or empty/double segments; Clean must be a no-op.
	if path.Clean(key) != key {
		return false
	}
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		return false
	}
	if parts[0] != "community" || parts[1] != rtID || parts[2] == "" || parts[3] == "" {
		return false
	}
	if strings.Contains(parts[2], "..") || strings.Contains(parts[3], "..") {
		return false
	}
	ext := strings.ToLower(path.Ext(parts[3]))
	return imageAllowedExtensions[ext]
}

// imageSignature binds the key and expiry only. Signed URLs are ALWAYS
// evaluated at warga visibility level; there is deliberately no privilege
// channel embedded in the URL — a leaked URL grants exactly what warga
// receives, never manager rights.
func imageSignature(key string, exp int64) string {
	mac := hmac.New(sha256.New, auth.SigningSecret)
	mac.Write([]byte(key + "." + strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// signImageURL builds a short-lived signed URL for an already-authorized key.
func signImageURL(key string) (string, error) {
	if len(auth.SigningSecret) == 0 {
		return "", errors.New("image signing unavailable")
	}
	exp := time.Now().Add(imageSignatureLifetime).Unix()
	sig := imageSignature(key, exp)
	return "/api/v1/community/images/" + key + "?exp=" + strconv.FormatInt(exp, 10) + "&sig=" + sig, nil
}

// verifyImageSignature checks a signed-URL query pair for key.
func verifyImageSignature(key, expStr, sig string) bool {
	return verifyImageSignatureAt(key, expStr, sig, time.Now())
}

// verifyImageSignatureAt is the core of verifyImageSignature with an
// injected clock, so expiry boundaries can be tested deterministically.
// A signature is valid only while strictly before its expiry: at or after
// exp the URL is rejected.
func verifyImageSignatureAt(key, expStr, sig string, now time.Time) bool {
	if len(auth.SigningSecret) == 0 || key == "" || expStr == "" || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if now.Unix() >= exp {
		return false
	}
	expected := imageSignature(key, exp)
	return hmac.Equal([]byte(expected), []byte(sig))
}

var errImageForbidden = errors.New("image access forbidden")

// authenticateImageBearer replicates the core of the RequireAuth +
// RequireAllowedRT pipeline for the Bearer mode of GetImage, which lives
// outside middleware groups so that signed URLs remain reachable by <img>
// tags. It validates the JWT, the user's identity/membership in the DB, and
// that the effective RT is active and allowed (codes 002/003), returning
// the AuthContext on success.
func (h *Handler) authenticateImageBearer(r *http.Request) (*auth.AuthContext, error) {
	raw := r.Header.Get("Authorization")
	if raw == "" {
		return nil, errors.New("missing authorization header")
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, errors.New("invalid authorization scheme")
	}
	claims, err := auth.ParseAndVerifyAccessToken(parts[1])
	if err != nil {
		return nil, err
	}
	if h.pool == nil {
		return nil, errors.New("identity validation unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	validateErr := auth.ValidateIdentity(ctx, tx, claims.UserID, claims.MID)
	_ = tx.Rollback()
	if validateErr != nil {
		return nil, validateErr
	}

	rtID := claims.RTID
	if auth.SystemRole(claims.SysRole) == auth.SystemRoleSuperAdmin && rtID == "" {
		// Same effective-RT logic as RequireAuth for system super admins.
		reqRT := strings.TrimSpace(r.Header.Get("X-RT-ID"))
		if reqRT == "" {
			reqRT = strings.TrimSpace(r.URL.Query().Get("rt_id"))
		}
		if reqRT != "" {
			var validID string
			if err := h.pool.Raw().QueryRowContext(ctx, "SELECT id FROM rts WHERE id = $1 AND is_active = true", reqRT).Scan(&validID); err == nil {
				rtID = validID
			}
		}
	}
	if rtID == "" {
		return nil, errImageForbidden
	}

	// RT must be active and in the allowed codes set (same rule as
	// RequireAllowedRT).
	var rtCode string
	var isActive bool
	err = h.pool.Raw().QueryRowContext(ctx, `SELECT rt, is_active FROM rts WHERE id = $1 LIMIT 1`, rtID).Scan(&rtCode, &isActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errImageForbidden
		}
		return nil, err
	}
	if !isActive || !auth.AllowedRTCodes[rtCode] {
		return nil, errImageForbidden
	}

	return &auth.AuthContext{
		UserID:       claims.UserID,
		MembershipID: claims.MID,
		RTID:         rtID,
		SystemRole:   auth.SystemRole(claims.SysRole),
		TenantRole:   auth.Role(claims.Role),
		Jabatan:      auth.Jabatan(claims.Jabatan),
	}, nil
}

// canViewImageContent resolves whether the caller may see draft content
// images for activities and announcements respectively (permission based,
// super-admin bypass). Used by both Bearer and signed-URL issuance checks.
func (h *Handler) canManageContent(ctx context.Context, ac *auth.AuthContext) (canActivities, canAnnouncements bool) {
	if ac == nil {
		return false, false
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true, true
	}
	if h.pool == nil {
		return false, false
	}
	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		return false, false
	}
	defer tx.Rollback()
	pr := auth.NewPermissionResolver(h.pool)
	create, _ := pr.HasPermission(ctx, tx, ac, "activities.create")
	update, _ := pr.HasPermission(ctx, tx, ac, "activities.update")
	canActivities = create || update
	create, _ = pr.HasPermission(ctx, tx, ac, "announcements.create")
	update, _ = pr.HasPermission(ctx, tx, ac, "announcements.update")
	canAnnouncements = create || update
	return canActivities, canAnnouncements
}

// canPreviewUnreferencedCommunityImage permits authenticated managers to
// preview a freshly uploaded image key in their own RT before it is referenced
// by saved content. This is Bearer-only and never extends signed/warga access.
func (h *Handler) canPreviewUnreferencedCommunityImage(ctx context.Context, ac *auth.AuthContext, key string, canAct, canAnn bool) bool {
	if ac == nil || ac.RTID == "" || key == "" || h.pool == nil {
		return false
	}
	parts := strings.Split(key, "/")
	if len(parts) != 4 || parts[0] != "community" || parts[1] != ac.RTID {
		return false
	}
	category := parts[2]
	switch category {
	case "activity":
		return canAct
	case "announcement":
		return canAnn
	case "general":
		return canAct || canAnn
	default:
		return false
	}
}
