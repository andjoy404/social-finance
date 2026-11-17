package profilephoto

// Resident profile photos.
//
// Store photos in object storage under a dedicated category and persist only
// the storage key on residents.avatar_path. Authorization is enforced by the
// route middleware (resident.profile_photo.manage) for mutations and by the
// community image reference check (community.ImageAccessAllowed) for reads.
//
// Never handles anything except residents rows; never makes storage public.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	"social-finance/internal/storage"
)

// Handler wires profile photo endpoints to the shared storage backend.
type Handler struct {
	pool  *database.Pool
	store storage.Service
}

func NewHandler(pool *database.Pool, store storage.Service) *Handler {
	return &Handler{pool: pool, store: store}
}

type targetRTResolver func(context.Context) (string, error)

// resolveTargetRT resolves the RT scope for the resident id. Tenant users are
// scoped to their authenticated RT (middleware enforces it before the handler
// runs); a system super_admin without RT context resolves the RT from the
// resident itself (mirrors the management flows in households).
func (h *Handler) resolveTargetRT(ctx context.Context, ac *auth.AuthContext, residentID string) (string, error) {
	rtID := strings.TrimSpace(ac.RTID)
	if rtID != "" {
		// Ensure the resident actually lives in the caller's RT.
		var exists bool
		err := h.pool.Raw().QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM residents WHERE id = $1 AND rt_id = $2 AND is_active = true)`,
			residentID, rtID).Scan(&exists)
		if err != nil {
			return "", fmt.Errorf("resolve resident rt: %w", err)
		}
		if !exists {
			return "", fmt.Errorf("resident not found")
		}
		return rtID, nil
	}
	// System super_admin without an active RT — resolve from the resident.
	var target string
	err := h.pool.Raw().QueryRowContext(ctx,
		`SELECT rt_id FROM residents WHERE id = $1 AND is_active = true`,
		residentID).Scan(&target)
	if err != nil {
		return "", err
	}
	return target, nil
}

// loadCurrentAvatar reads the resident's current avatar_path in one query and
// validates RT ownership so a dangling tenant id never leaks another RT's key.
func (h *Handler) loadCurrentAvatar(ctx context.Context, residentID, rtID string) (string, error) {
	var current *string
	err := h.pool.Raw().QueryRowContext(ctx,
		`SELECT avatar_path FROM residents WHERE id = $1 AND rt_id = $2 AND is_active = true`,
		residentID, rtID).Scan(&current)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", nil
	}
	return strings.TrimSpace(*current), nil
}

// auditExecutor is satisfied by *sql.Tx, so the audit entry commits or rolls
// back atomically with the resident update it describes.
type auditExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func auditLog(ctx context.Context, pool auditExecutor, rtID, userID, action, entityID string, oldKey, newKey *string, ip string) error {
	marshal := func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	oldPayload := map[string]any{}
	if oldKey != nil {
		oldPayload["avatar_path"] = *oldKey
	}
	newPayload := map[string]any{}
	if newKey != nil {
		newPayload["avatar_path"] = *newKey
	}
	oldVal, err := marshal(oldPayload)
	if err != nil {
		return err
	}
	newVal, err := marshal(newPayload)
	if err != nil {
		return err
	}
	_, err = pool.ExecContext(ctx, `
		INSERT INTO audit_logs (rt_id, user_id, action, entity_type, entity_id, old_values, new_values, ip_address)
		VALUES ($1, $2, $3, 'resident', $4, $5, $6, $7)
	`, rtID, userID, action, entityID, oldVal, newVal, ip)
	return err
}

func httpReqIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	return r.RemoteAddr
}

// Upload handles POST /api/v1/residents/{id}/photo\n// Authorization: route middleware (RequirePermission resident.profile_photo.manage).
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || (ac.RTID == "" && ac.SystemRole != auth.SystemRoleSuperAdmin) {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "resident id is required")
		return
	}

	ctx := r.Context()
	rtID, err := h.resolveTargetRT(ctx, ac, id)
	if err != nil {
		if err.Error() == "resident not found" {
			httpx.WriteNotFound(w, "resident not found")
			return
		}
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "target resident not accessible")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+1024)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.WriteValidationError(w, "invalid multipart form or file exceeds 5MB limit", nil)
		return
	}
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		httpx.WriteValidationError(w, "file field is required", nil)
		return
	}
	defer file.Close()
	if fileHeader.Size > 5*1024*1024 {
		httpx.WriteValidationError(w, "file size exceeds 5MB limit", nil)
		return
	}
	// Extension sanity check; storage still re-sniffs the magic bytes.
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		httpx.WriteValidationError(w, "only JPG, PNG or WebP images are allowed", nil)
		return
	}

	meta, err := h.store.UploadCommunityImageWithThumb(ctx, rtID, "profile", fileHeader.Filename, file, fileHeader.Size, true)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	// Persist reference and audit atomically.
	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		if meta.ThumbKey != "" {
			h.cleanupObject(ctx, rtID, "delete_new_thumb_after_begin_tx_failure", meta.ThumbKey)
		}
		h.cleanupObject(ctx, rtID, "delete_new_original_after_begin_tx_failure", meta.OriginalKey)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	var currentKey *string
	var currentThumbKey *string
	err = tx.QueryRowContext(ctx,
		`SELECT avatar_path, avatar_thumb_path FROM residents WHERE id = $1 AND rt_id = $2 FOR UPDATE`,
		id, rtID).Scan(&currentKey, &currentThumbKey)
	if err != nil {
		if meta.ThumbKey != "" {
			h.cleanupObject(ctx, rtID, "delete_new_thumb_after_load_failure", meta.ThumbKey)
		}
		h.cleanupObject(ctx, rtID, "delete_new_original_after_load_failure", meta.OriginalKey)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not load resident")
		return
	}
	oldKey := ""
	oldThumbKey := ""
	if currentKey != nil {
		oldKey = strings.TrimSpace(*currentKey)
	}
	if currentThumbKey != nil {
		oldThumbKey = strings.TrimSpace(*currentThumbKey)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE residents SET avatar_path = $1, avatar_thumb_path = $2, updated_at = now() WHERE id = $3`,
		meta.OriginalKey, nullIfEmpty(meta.ThumbKey), id); err != nil {
		if meta.ThumbKey != "" {
			h.cleanupObject(ctx, rtID, "delete_new_thumb_after_update_failure", meta.ThumbKey)
		}
		h.cleanupObject(ctx, rtID, "delete_new_original_after_update_failure", meta.OriginalKey)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not save avatar reference")
		return
	}
	if err := auditLog(ctx, tx, rtID, ac.UserID, "resident.profile_photo.change", id,
		oldKeyPtr(oldKey), &meta.OriginalKey, httpReqIP(r)); err != nil {
		if meta.ThumbKey != "" {
			h.cleanupObject(ctx, rtID, "delete_new_thumb_after_audit_failure", meta.ThumbKey)
		}
		h.cleanupObject(ctx, rtID, "delete_new_original_after_audit_failure", meta.OriginalKey)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not record audit log")
		return
	}
	if err := tx.Commit(); err != nil {
		if meta.ThumbKey != "" {
			h.cleanupObject(ctx, rtID, "delete_new_thumb_after_commit_failure", meta.ThumbKey)
		}
		h.cleanupObject(ctx, rtID, "delete_new_original_after_commit_failure", meta.OriginalKey)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not commit transaction")
		return
	}

	// Best-effort cleanup of superseded keys after commit. Never delete fresh
	// references and never delete keys still referenced by any resident.
	h.cleanupSuperseded(ctx, rtID, "avatar_path", oldKey, meta.OriginalKey)
	h.cleanupSuperseded(ctx, rtID, "avatar_thumb_path", oldThumbKey, meta.ThumbKey)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":            "uploaded",
		"avatar_path":       meta.OriginalKey,
		"avatar_thumb_path": meta.ThumbKey,
		"key":               meta.OriginalKey,
	})
}

// Delete handles DELETE /api/v1/residents/{id}/photo
// Authorization: route middleware (RequirePermission resident.profile_photo.manage).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || (ac.RTID == "" && ac.SystemRole != auth.SystemRoleSuperAdmin) {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "resident id is required")
		return
	}

	ctx := r.Context()
	rtID, err := h.resolveTargetRT(ctx, ac, id)
	if err != nil {
		if err.Error() == "resident not found" {
			httpx.WriteNotFound(w, "resident not found")
			return
		}
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "target resident not accessible")
		return
	}

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()
	var oldKey *string
	var oldThumbKey *string
	err = tx.QueryRowContext(ctx,
		`SELECT avatar_path, avatar_thumb_path FROM residents WHERE id = $1 AND rt_id = $2 FOR UPDATE`,
		id, rtID).Scan(&oldKey, &oldThumbKey)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not load resident")
		return
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE residents SET avatar_path = NULL, avatar_thumb_path = NULL, updated_at = now() WHERE id = $1`, id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not clear avatar reference")
		return
	}
	old := ""
	oldThumb := ""
	if oldKey != nil {
		old = strings.TrimSpace(*oldKey)
	}
	if oldThumbKey != nil {
		oldThumb = strings.TrimSpace(*oldThumbKey)
	}
	if err := auditLog(ctx, tx, rtID, ac.UserID, "resident.profile_photo.clear", id,
		oldKeyPtr(old), nil, httpReqIP(r)); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not record audit log")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not commit transaction")
		return
	}

	if old != "" {
		h.cleanupSuperseded(ctx, rtID, "avatar_path", old, "")
	}
	if oldThumb != "" {
		h.cleanupSuperseded(ctx, rtID, "avatar_thumb_path", oldThumb, "")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "cleared"})
}

func oldKeyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (h *Handler) cleanupObject(ctx context.Context, rtID, operation, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if err := h.store.DeleteCommunityImage(ctx, key); err != nil {
		slog.Warn("profile photo cleanup failed",
			"operation", operation,
			"object_key", key,
			"rt_id", rtID,
			"error", err,
		)
	}
}

func (h *Handler) cleanupSuperseded(ctx context.Context, rtID, column, oldKey, keepKey string) {
	oldKey = strings.TrimSpace(oldKey)
	keepKey = strings.TrimSpace(keepKey)
	if oldKey == "" || (keepKey != "" && oldKey == keepKey) {
		return
	}
	if column != "avatar_path" && column != "avatar_thumb_path" {
		return
	}
	var otherRefs int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM residents WHERE %s = $1`, column)
	if err := h.pool.Raw().QueryRowContext(ctx, query, oldKey).Scan(&otherRefs); err != nil {
		slog.Warn("profile photo cleanup reference check failed",
			"operation", "check_superseded_refs",
			"object_key", oldKey,
			"rt_id", rtID,
			"column", column,
			"error", err,
		)
		return
	}
	if otherRefs == 0 {
		h.cleanupObject(ctx, rtID, "delete_superseded_"+column, oldKey)
	}
}
