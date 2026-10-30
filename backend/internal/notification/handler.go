package notification

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	httpx "social-finance/internal/http"
	"social-finance/internal/storage"
)

// Handler handles HTTP requests for notifications and device token management.
type Handler struct {
	svc     Service
	storage storage.Service
	appEnv  string
}

// NewHandler creates a new Handler instance.
func NewHandler(svc Service, store storage.Service, appEnv ...string) *Handler {
	env := "development"
	if len(appEnv) > 0 && appEnv[0] != "" {
		env = appEnv[0]
	}
	if store == nil {
		store = storage.NewMockStorage()
	}
	return &Handler{svc: svc, storage: store, appEnv: env}
}

// RegisterDevice registers an FCM token for the authenticated user.
func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var in RegisterDeviceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	if in.FCMToken == "" {
		httpx.WriteValidationError(w, "fcm_token is required", nil)
		return
	}

	if err := h.svc.RegisterDevice(r.Context(), ac.UserID, in); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UnregisterDevice unregisters an FCM token for the authenticated user.
func (h *Handler) UnregisterDevice(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var in UnregisterDeviceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	if in.FCMToken == "" {
		httpx.WriteValidationError(w, "fcm_token is required", nil)
		return
	}

	if err := h.svc.UnregisterDevice(r.Context(), ac.UserID, in.FCMToken); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TriggerWasteTruckArrival allows pengurus or security in an RT to trigger garbage truck arrival alert.
func (h *Handler) TriggerWasteTruckArrival(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	var in WasteTruckNotificationInput
	_ = json.NewDecoder(r.Body).Decode(&in)

	h.svc.NotifyWasteTruckArrival(r.Context(), rtID, ac.UserID, in.Message, in.ImageURL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "accepted",
		"message": "Waste truck arrival notification dispatched",
	})
}

// TriggerManualPush allows authorized admins to dispatch manual broadcast notifications.
func (h *Handler) TriggerManualPush(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	var in ManualPushInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	if err := h.svc.SendManualPush(r.Context(), rtID, ac.UserID, in); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "accepted",
		"message": "Manual push notification dispatched",
	})
}

// TestPush dispatches an isolated test push notification exclusively to the caller Super Admin's devices.
func (h *Handler) TestPush(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if ac.SystemRole != auth.SystemRoleSuperAdmin {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "hanya super_admin yang berwenang melakukan uji notifikasi")
		return
	}

	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	var in TestPushInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	count, err := h.svc.SendTestPush(r.Context(), rtID, ac.UserID, in)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":           "dispatched",
		"message":          "Test notification dispatched to your registered devices",
		"dispatched_count": count,
	})
}

// UploadImage handles uploading notification images to RustFS.
func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	// 5MB max upload
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+1024)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.WriteValidationError(w, "file too large or invalid multipart form", nil)
		return
	}

	eventType := strings.TrimSpace(r.FormValue("event_type"))
	if eventType == "" {
		httpx.WriteValidationError(w, "event_type is required", nil)
		return
	}
	if eventType != EventWasteTruckArrival && eventType != EventManualPush {
		httpx.WriteValidationError(w, fmt.Sprintf("upload gambar tidak diizinkan untuk event '%s'", eventType), nil)
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

	meta, err := h.storage.UploadNotificationImage(r.Context(), rtID, eventType, fileHeader.Filename, file, fileHeader.Size)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	imageURL := "/api/v1/notifications/images/" + meta.Key
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "uploaded",
		"image_url": imageURL,
		"key":       meta.Key,
	})
}

// GetImage streams a notification image from RustFS.
func (h *Handler) GetImage(w http.ResponseWriter, r *http.Request) {
	rawKey := chi.URLParam(r, "*")
	if rawKey == "" {
		rawKey = strings.TrimPrefix(r.URL.Path, "/api/v1/notifications/images/")
	}
	if strings.Contains(rawKey, "..") || strings.Contains(rawKey, "\\") || strings.Contains(rawKey, "\x00") {
		httpx.WriteValidationError(w, "invalid image key", nil)
		return
	}
	key := strings.TrimPrefix(rawKey, "/")
	if !strings.HasPrefix(key, "notifications/") {
		key = "notifications/" + key
	}

	reader, err := h.storage.GetNotificationImage(r.Context(), key)
	if err != nil {
		httpx.WriteNotFound(w, "image not found")
		return
	}
	defer reader.Body.Close()

	w.Header().Set("Content-Type", reader.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(reader.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, reader.Body)
}

// TestSelf sends an isolated test push notification exclusively to the authenticated user's registered devices.
// Disabled in production environment.
func (h *Handler) TestSelf(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "test endpoint not available in production")
		return
	}

	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	count, err := h.svc.SendSelfTest(r.Context(), ac.UserID)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to dispatch self-test notification")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":           "dispatched",
		"message":          "Test notification dispatched to your registered devices",
		"dispatched_count": count,
	})
}

func (h *Handler) resolveTenantRT(r *http.Request, ac *auth.AuthContext) (string, error) {
	if ac == nil {
		return "", errors.New("unauthorized: missing auth context")
	}
	return h.svc.ResolveTenantRT(r.Context(), ac, r.Header.Get("X-RT-ID"), r.URL.Query().Get("rt_id"))
}

// GetSystemTypes returns all recognized notification system events.
func (h *Handler) GetSystemTypes(w http.ResponseWriter, r *http.Request) {
	types := h.svc.GetSystemTypes()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": types,
	})
}

// ListTemplates lists notification templates for the tenant RT.
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	eventType := r.URL.Query().Get("event_type")
	var isActive *bool
	if activeStr := r.URL.Query().Get("is_active"); activeStr != "" {
		val := activeStr == "true" || activeStr == "1"
		isActive = &val
	}

	templates, err := h.svc.ListTemplates(r.Context(), rtID, eventType, isActive)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to list notification templates")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": templates,
	})
}

// GetTemplate returns detail of a single notification template.
func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		httpx.WriteValidationError(w, "template id is required", nil)
		return
	}

	tmpl, err := h.svc.GetTemplate(r.Context(), rtID, id)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to get notification template")
		return
	}
	if tmpl == nil {
		httpx.WriteNotFound(w, "template not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tmpl)
}

// CreateTemplate creates a new notification template.
func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	var in CreateTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tmpl, err := h.svc.CreateTemplate(r.Context(), rtID, in)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(tmpl)
}

// UpdateTemplate updates an existing notification template.
func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		httpx.WriteValidationError(w, "template id is required", nil)
		return
	}

	var in UpdateTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tmpl, err := h.svc.UpdateTemplate(r.Context(), rtID, id, in)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	if tmpl == nil {
		httpx.WriteNotFound(w, "template not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tmpl)
}

// ToggleTemplateActive toggles the is_active state of a template.
func (h *Handler) ToggleTemplateActive(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		httpx.WriteValidationError(w, "template id is required", nil)
		return
	}

	tmpl, err := h.svc.ToggleTemplateActive(r.Context(), rtID, id)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to toggle template status")
		return
	}
	if tmpl == nil {
		httpx.WriteNotFound(w, "template not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tmpl)
}

// DeleteTemplate deletes a notification template.
func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		httpx.WriteValidationError(w, "template id is required", nil)
		return
	}

	if err := h.svc.DeleteTemplate(r.Context(), rtID, id); err != nil {
		httpx.WriteNotFound(w, "template not found or already deleted")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListLogs returns paginated delivery logs from notification_logs.
func (h *Handler) ListLogs(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	pageSize := 20
	if ps := r.URL.Query().Get("per_page"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil && val > 0 {
			pageSize = val
		}
	} else if ps := r.URL.Query().Get("page_size"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil && val > 0 {
			pageSize = val
		}
	}

	eventType := r.URL.Query().Get("event_type")

	logs, total, err := h.svc.ListLogs(r.Context(), rtID, eventType, page, pageSize)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to list notification logs")
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": logs,
		"pagination": map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetNotificationDetail returns detail of a single notification log entry for an authenticated RT member.
func (h *Handler) GetNotificationDetail(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "invalid or missing tenant context")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteValidationError(w, "notification id is required", nil)
		return
	}

	entry, err := h.svc.GetNotificationDetail(r.Context(), ac, rtID, id)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		httpx.WriteInternalServerError(w, "failed to get notification detail")
		return
	}
	if entry == nil {
		httpx.WriteNotFound(w, "notification not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": entry,
	})
}

