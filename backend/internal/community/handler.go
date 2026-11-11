package community

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	"social-finance/internal/storage"
)

// Handler handles HTTP requests for community features.
type Handler struct {
	svc     *Service
	pool    *database.Pool
	storage storage.Service
}

// NewHandler creates a new community Handler.
func NewHandler(svc *Service, pool *database.Pool, store storage.Service) *Handler {
	if store == nil {
		store = storage.NewMockStorage()
	}
	return &Handler{
		svc:     svc,
		pool:    pool,
		storage: store,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// hasPermission is the single permission check used by community handlers.
// Super admin bypasses (official bypass, still RT-scoped by handlers).
// Jabatan-based users are resolved against position_permissions; users
// without a jabatan never gain community management/contact rights here
// (their role-based access is enforced separately by route middleware).
func (h *Handler) hasPermission(ctx context.Context, ac *auth.AuthContext, code string) bool {
	if ac == nil {
		return false
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true
	}
	if ac.Jabatan == "" || h.pool == nil {
		return false
	}
	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback()
	pr := auth.NewPermissionResolver(h.pool)
	allowed, _ := pr.HasPermission(ctx, tx, ac, code)
	return allowed
}

// canManageActivities reports whether the caller may see draft activities
// (needs create or update permission). Replaces the former role/jabatan
// based isOfficer check, which treated every pengurus/bendahara role and
// jabatan ketua/sekretaris/bendahara as draft-capable regardless of grants.
func (h *Handler) canManageActivities(ctx context.Context, ac *auth.AuthContext) bool {
	if ac == nil {
		return false
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true
	}
	return h.hasPermission(ctx, ac, "activities.create") || h.hasPermission(ctx, ac, "activities.update")
}

// canManageAnnouncements reports whether the caller may see draft/scheduled
// announcements (needs create or update permission).
func (h *Handler) canManageAnnouncements(ctx context.Context, ac *auth.AuthContext) bool {
	if ac == nil {
		return false
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true
	}
	return h.hasPermission(ctx, ac, "announcements.create") || h.hasPermission(ctx, ac, "announcements.update")
}

// hasContactPermission reports whether the caller may see pengurus phone
// numbers: only the organization.contact.read permission (super-admin
// bypass included). There is deliberately no hardcoded jabatan shortcut —
// the database grant is the single source of truth.
func (h *Handler) hasContactPermission(ctx context.Context, ac *auth.AuthContext) bool {
	if ac == nil {
		return false
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true
	}
	return h.hasPermission(ctx, ac, "organization.contact.read")
}

func (h *Handler) getUserFullName(ctx context.Context, userID string) string {
	if h.pool == nil || userID == "" {
		return "Warga"
	}
	var name string
	err := h.pool.Raw().QueryRowContext(ctx, `SELECT full_name FROM users WHERE id = $1`, userID).Scan(&name)
	if err != nil || strings.TrimSpace(name) == "" {
		return "Warga"
	}
	return name
}

// GetHomeDashboard returns composite data for Beranda.
func (h *Handler) GetHomeDashboard(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context or RT scope")
		return
	}

	fullName := h.getUserFullName(r.Context(), ac.UserID)
	role := string(ac.TenantRole)
	if role == "" && ac.SystemRole != "" {
		role = string(ac.SystemRole)
	}
	jabatan := string(ac.Jabatan)
	canContact := h.hasContactPermission(r.Context(), ac)

	data, err := h.svc.GetHomeDashboard(r.Context(), ac.RTID, fullName, role, jabatan, canContact)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

// -----------------------------------------------------------------------------
// Activities Handlers
// -----------------------------------------------------------------------------

// ListActivities lists activities with pagination and status filters.
func (h *Handler) ListActivities(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	pagination := httpx.ParsePagination(r)
	offset := (pagination.Page - 1) * pagination.PerPage

	pubStatus := r.URL.Query().Get("publication_status")
	status := r.URL.Query().Get("status")
	isOfficer := h.canManageActivities(r.Context(), ac)

	activities, total, err := h.svc.ListActivities(r.Context(), ac.RTID, pubStatus, status, isOfficer, pagination.PerPage, offset)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = (total + pagination.PerPage - 1) / pagination.PerPage
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": activities,
		"pagination": map[string]any{
			"page":        pagination.Page,
			"per_page":    pagination.PerPage,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetActivity returns an activity by ID.
func (h *Handler) GetActivity(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "activity id is required", nil)
		return
	}

	activity, err := h.svc.GetActivityByID(r.Context(), ac.RTID, id, h.canManageActivities(r.Context(), ac))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "activity not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, activity)
}

// CreateActivity creates a new activity.
func (h *Handler) CreateActivity(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req CreateActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	activity, err := h.svc.CreateActivity(r.Context(), ac.RTID, ac.UserID, req)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, activity)
}

// UpdateActivity updates an existing activity.
func (h *Handler) UpdateActivity(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "activity id is required", nil)
		return
	}

	var req UpdateActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	activity, err := h.svc.UpdateActivity(r.Context(), ac.RTID, id, ac.UserID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "activity not found")
			return
		}
		if errors.Is(err, ErrValidation) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, activity)
}

// PublishActivity marks an activity as published.
func (h *Handler) PublishActivity(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "activity id is required", nil)
		return
	}

	err := h.svc.PublishActivity(r.Context(), ac.RTID, id, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "activity not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

// CancelActivity cancels an activity.
func (h *Handler) CancelActivity(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "activity id is required", nil)
		return
	}

	err := h.svc.CancelActivity(r.Context(), ac.RTID, id, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "activity not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// DeleteActivity deletes a draft activity.
func (h *Handler) DeleteActivity(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !h.hasPermission(r.Context(), ac, "activities.delete") && !h.hasPermission(r.Context(), ac, "activities.create") {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "activity id is required", nil)
		return
	}

	err := h.svc.DeleteActivity(r.Context(), ac.RTID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "activity not found")
			return
		}
		if errors.Is(err, ErrValidation) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// -----------------------------------------------------------------------------
// Announcements Handlers
// -----------------------------------------------------------------------------

// ListAnnouncements lists announcements.
func (h *Handler) ListAnnouncements(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	pagination := httpx.ParsePagination(r)
	offset := (pagination.Page - 1) * pagination.PerPage

	status := r.URL.Query().Get("status")
	canManage := h.canManageAnnouncements(r.Context(), ac)

	announcements, total, err := h.svc.ListAnnouncements(r.Context(), ac.RTID, status, canManage, pagination.PerPage, offset)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = (total + pagination.PerPage - 1) / pagination.PerPage
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": announcements,
		"pagination": map[string]any{
			"page":        pagination.Page,
			"per_page":    pagination.PerPage,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetAnnouncement returns an announcement by ID.
func (h *Handler) GetAnnouncement(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "announcement id is required", nil)
		return
	}

	announcement, err := h.svc.GetAnnouncementByID(r.Context(), ac.RTID, id, h.canManageAnnouncements(r.Context(), ac))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "announcement not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, announcement)
}

// CreateAnnouncement creates a new announcement.
func (h *Handler) CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req CreateAnnouncementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	announcement, err := h.svc.CreateAnnouncement(r.Context(), ac.RTID, ac.UserID, req)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			httpx.WriteNotFound(w, "template not found")
			return
		}
		if errors.Is(err, ErrValidation) || errors.Is(err, ErrInvalidDeepLink) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, announcement)
}

// UpdateAnnouncement updates an existing announcement.
func (h *Handler) UpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "announcement id is required", nil)
		return
	}

	var req UpdateAnnouncementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	announcement, err := h.svc.UpdateAnnouncement(r.Context(), ac.RTID, id, ac.UserID, req)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			httpx.WriteNotFound(w, "template not found")
			return
		}
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "announcement not found")
			return
		}
		if errors.Is(err, ErrValidation) || errors.Is(err, ErrInvalidDeepLink) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, announcement)
}

// PublishAnnouncement marks an announcement as published.
func (h *Handler) PublishAnnouncement(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "announcement id is required", nil)
		return
	}

	err := h.svc.PublishAnnouncement(r.Context(), ac.RTID, id, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "announcement not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

// ArchiveAnnouncement archives an announcement.
func (h *Handler) ArchiveAnnouncement(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "announcement id is required", nil)
		return
	}

	err := h.svc.ArchiveAnnouncement(r.Context(), ac.RTID, id, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "announcement not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// DeleteAnnouncement deletes an announcement.
func (h *Handler) DeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !h.hasPermission(r.Context(), ac, "announcements.delete") && !h.hasPermission(r.Context(), ac, "announcements.create") {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "announcement id is required", nil)
		return
	}

	err := h.svc.DeleteAnnouncement(r.Context(), ac.RTID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "announcement not found")
			return
		}
		if errors.Is(err, ErrValidation) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// -----------------------------------------------------------------------------
// Banners & Templates Handlers
// -----------------------------------------------------------------------------

// ListBannerTemplates returns available banner templates.
func (h *Handler) ListBannerTemplates(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	templates, err := h.svc.ListBannerTemplates(r.Context(), ac.RTID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": templates})
}

// GetBannerTemplate returns a banner template by ID.
func (h *Handler) GetBannerTemplate(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteValidationError(w, "template id is required", nil)
		return
	}

	tmpl, err := h.svc.GetBannerTemplateByID(r.Context(), ac.RTID, id)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			httpx.WriteNotFound(w, "template not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, tmpl)
}

// ListBanners returns active banners for the RT.
func (h *Handler) ListBanners(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	banners, err := h.svc.ListActiveBanners(r.Context(), ac.RTID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": banners})
}

// GetOrganizationStructure returns the organizational hierarchy for the RT.
func (h *Handler) GetOrganizationStructure(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	canContact := h.hasContactPermission(r.Context(), ac)
	org, err := h.svc.repo.GetOrganizationStructure(r.Context(), nil, ac.RTID, canContact)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// UploadImage handles image uploads for activities and announcements.
func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	// 5MB max upload limit
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+1024)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.WriteValidationError(w, "invalid multipart form or file exceeds 5MB limit", nil)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		file, fileHeader, err = r.FormFile("image")
	}
	if err != nil {
		httpx.WriteValidationError(w, "file field is required", nil)
		return
	}
	defer file.Close()

	if fileHeader.Size > 5*1024*1024 {
		httpx.WriteValidationError(w, "file size exceeds 5MB limit", nil)
		return
	}

	category := r.FormValue("category")
	if category == "" {
		category = "general"
	}

	// Upload authorization is category-scoped: read-only users must not be
	// able to upload posters/banners. Route middleware only checks the
	// jabatan gate; this enforces the specific create permission.
	switch category {
	case "activity":
		if !h.hasPermission(r.Context(), ac, "activities.create") {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
	case "announcement":
		if !h.hasPermission(r.Context(), ac, "announcements.create") {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
	default:
		// banner/general images back activity or announcement content.
		if !h.hasPermission(r.Context(), ac, "activities.create") && !h.hasPermission(r.Context(), ac, "announcements.create") {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
	}

	meta, err := h.storage.UploadCommunityImage(r.Context(), ac.RTID, category, fileHeader.Filename, file, fileHeader.Size)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	imageURL := "/api/v1/community/images/" + meta.Key
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "uploaded",
		"image_url": imageURL,
		"key":       meta.Key,
	})
}

// imageKeySuffix normalizes raw key input from the wildcard route into the
// storage key form community/{rtID}/{category}/{file}.
func imageKeySuffix(raw string) string {
	key := strings.TrimPrefix(raw, "/")
	if !strings.HasPrefix(key, "community/") {
		key = "community/" + key
	}
	return key
}

// serveImage streams an already-authorized storage key to the client.
func (h *Handler) serveImage(w http.ResponseWriter, r *http.Request, key string) {
	reader, err := h.storage.GetCommunityImage(r.Context(), key)
	if err != nil {
		httpx.WriteNotFound(w, "image not found")
		return
	}
	defer reader.Body.Close()

	if reader.ContentType != "" {
		w.Header().Set("Content-Type", reader.ContentType)
	} else {
		lower := strings.ToLower(key)
		switch {
		case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
			w.Header().Set("Content-Type", "image/jpeg")
		case strings.HasSuffix(lower, ".png"):
			w.Header().Set("Content-Type", "image/png")
		case strings.HasSuffix(lower, ".webp"):
			w.Header().Set("Content-Type", "image/webp")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, reader.Body)
}

// GetImage serves a community image. Two authorization modes are supported:
//
//  1. Signed URL — ?exp=<unix>&sig=<hmac>; signature is verified against the
//     signing secret and the key format is re-validated. Used for short-lived
//     read access issued by SignImageURL (e.g. <img> tags).
//  2. Bearer — Authorization: Bearer <jwt>; the caller must hold a valid RT
//     context in an allowed+active RT, the key must live under the caller's
//     RT prefix, and the key must be referenced by content the caller may
//     see (drafts only for content managers).
//
// Everything else (unreferenced keys, other RTs, traversal, invalid
// signatures, other image prefixes) is rejected.
func (h *Handler) GetImage(w http.ResponseWriter, r *http.Request) {
	// Community images are authorization-gated per request; intermediary and
	// browser caches must never store them, for any response on this path
	// (including errors).
	w.Header().Set("Cache-Control", "no-store")
	rawKey := chi.URLParam(r, "*")
	if rawKey == "" {
		rawKey = strings.TrimPrefix(r.URL.Path, "/api/v1/community/images/")
	}
	key := imageKeySuffix(rawKey)

	// Mode 1: signed URL. The signature binds the key and a short expiry.
	// Content access is revalidated against CURRENT state at WARGA
	// visibility level — signed URLs never serve draft/private content to
	// anyone holding them. Archived/cancelled/expired content stops
	// serving images immediately.
	if exp := r.URL.Query().Get("exp"); exp != "" {
		if !verifyImageSignature(key, exp, r.URL.Query().Get("sig")) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "invalid or expired image signature")
			return
		}
		parts := strings.SplitN(key, "/", 3)
		if len(parts) < 3 || !validateCommunityImageKey(key, parts[1]) {
			httpx.WriteValidationError(w, "invalid image key", nil)
			return
		}
		rtID := parts[1]
		allowed, err := h.svc.repo.ImageAccessAllowed(r.Context(), nil, rtID, key, false, false)
		if err != nil || !allowed {
			httpx.WriteNotFound(w, "image not found")
			return
		}
		h.serveImage(w, r, key)
		return
	}

	// Mode 2: Bearer auth (full identity + allowed-RT validation).
	ac, err := h.authenticateImageBearer(r)
	if err != nil {
		if errors.Is(err, errImageForbidden) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "image access forbidden")
			return
		}
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if !strings.HasPrefix(key, "community/"+ac.RTID+"/") || !validateCommunityImageKey(key, ac.RTID) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "image not accessible")
		return
	}
	canAct, canAnn := h.canManageContent(r.Context(), ac)
	allowed, err := h.svc.repo.ImageAccessAllowed(r.Context(), nil, ac.RTID, key, canAct, canAnn)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "image access check failed")
		return
	}
	if !allowed {
		// Allow authenticated content managers to preview freshly uploaded
		// images in their own RT before the key is referenced/persisted on
		// draft content. This path is Bearer-only (not signed URLs), scoped to
		// the caller RT, and limited to upload categories the caller may manage.
		// Signed/warga-visible access still requires referenced visible content.
		if !h.canPreviewUnreferencedCommunityImage(r.Context(), ac, key, canAct, canAnn) {
			// Do not reveal whether the key exists outside the caller's access.
			httpx.WriteNotFound(w, "image not found")
			return
		}
	}
	h.serveImage(w, r, key)
}

// SignImageURL issues a short-lived signed URL for a community image key.
// Web <img> tags cannot send Authorization headers, so the frontend calls
// this endpoint first and renders the signed URL. The signed URL always has
// warga visibility level: it never unlocks draft content, so a leaked URL
// confers no manager privilege. Content managers viewing draft/archived
// images must use the Bearer mode of GetImage (e.g. via an authenticated
// fetch), never signed URLs. The signature carries its own expiry; no
// unsigned access is granted.
func (h *Handler) SignImageURL(w http.ResponseWriter, r *http.Request) {
	// Response embeds a credential; never cached by intermediaries.
	w.Header().Set("Cache-Control", "no-store")
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	key := strings.TrimSpace(req.Key)
	key = imageKeySuffix(strings.TrimPrefix(key, "/api/v1/community/images/"))
	if !strings.HasPrefix(key, "community/"+ac.RTID+"/") || !validateCommunityImageKey(key, ac.RTID) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "image not accessible")
		return
	}
	// Signed URLs are warga-visible content only — even a content manager
	// cannot mint a signed URL that unlocks draft content.
	allowed, err := h.svc.repo.ImageAccessAllowed(r.Context(), nil, ac.RTID, key, false, false)
	if err != nil || !allowed {
		httpx.WriteNotFound(w, "image not found")
		return
	}
	signed, err := signImageURL(key)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "image signing unavailable")
		return
	}
	exp := time.Now().Add(imageSignatureLifetime).Unix()
	writeJSON(w, http.StatusOK, map[string]any{
		"url":        signed,
		"expires_at": exp,
	})
}
