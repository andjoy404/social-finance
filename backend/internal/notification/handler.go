package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	"social-finance/internal/storage"
)

// Handler handles HTTP requests for notifications and device token management.
type Handler struct {
	svc     Service
	pool    *database.Pool
	storage storage.Service
	appEnv  string
}

// NewHandler creates a new Handler instance.
func NewHandler(svc Service, pool *database.Pool, store storage.Service, appEnv ...string) *Handler {
	env := "development"
	if len(appEnv) > 0 && appEnv[0] != "" {
		env = appEnv[0]
	}
	if store == nil {
		store = storage.NewMockStorage()
	}
	return &Handler{svc: svc, pool: pool, storage: store, appEnv: env}
}

// isSecurityOfficer reports whether the authenticated principal holds the
// security officer position (keamanan) within their RT.
func isSecurityOfficer(ac *auth.AuthContext) bool {
	return ac != nil && strings.EqualFold(strings.TrimSpace(string(ac.Jabatan)), string(auth.JabatanKeamanan))
}

func canViewWasteTruckNotifications(ac *auth.AuthContext) bool {
	return isOfficerUser(ac) || isSecurityOfficer(ac)
}

// hasWasteTruckPermission reports whether the security officer holds the
// dedicated notification.send_waste_truck permission.
func (h *Handler) hasWasteTruckPermission(ctx context.Context, ac *auth.AuthContext) (bool, error) {
	if h.pool == nil || ac == nil {
		return false, errors.New("permission database unavailable")
	}
	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	pr := auth.NewPermissionResolver(h.pool)
	return pr.HasPermission(ctx, tx, ac, "notification.send_waste_truck")
}

// canDispatchNotificationEvent enforces per-event authorization for officer
// dispatch actions (upload image and trigger send).
// The waste_truck_arrival event accepts RT officers and security officers
// (keamanan) holding the dedicated notification.send_waste_truck permission.
// Security officers cannot dispatch other events (e.g. manual_push).
func (h *Handler) canDispatchNotificationEvent(ctx context.Context, ac *auth.AuthContext, eventType string) (bool, error) {
	if ac == nil {
		return false, nil
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true, nil
	}
	if isSecurityOfficer(ac) {
		if eventType == EventWasteTruckArrival {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return h.hasWasteTruckPermission(ctx, ac)
		}
		return false, nil
	}
	return isOfficerUser(ac), nil
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

// TriggerWasteTruckArrival allows pengurus or security (keamanan) in an RT to trigger garbage truck arrival alert.
// Security requires the notification.send_waste_truck permission; other officers use their existing officer access.
func (h *Handler) TriggerWasteTruckArrival(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	allowed, authErr := h.canDispatchNotificationEvent(r.Context(), ac, EventWasteTruckArrival)
	if authErr != nil {
		httpx.WriteInternalServerError(w, "failed to check waste truck permission")
		return
	}
	if !allowed {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "hanya officer RT atau petugas keamanan dengan permission notification.send_waste_truck yang berwenang memicu notifikasi waste truck")
		return
	}

	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	var in WasteTruckNotificationInput
	_ = json.NewDecoder(r.Body).Decode(&in)

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idemKey == "" {
		idemKey = strings.TrimSpace(in.IdempotencyKey)
	}

	h.svc.NotifyWasteTruckArrival(r.Context(), rtID, ac.UserID, in.Message, in.ImageURL, idemKey)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "accepted",
		"message": "Waste truck arrival notification dispatched",
	})
}

func parsePositiveQueryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

// ListWasteTruckHistory returns only dispatches created by the authenticated
// security officer in the caller's current RT.
func (h *Handler) ListWasteTruckHistory(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if !isSecurityOfficer(ac) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "only security officers can view their waste truck dispatch history")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", "valid RT context is required")
		return
	}

	page := parsePositiveQueryInt(r, "page", 1)
	pageSize := parsePositiveQueryInt(r, "per_page", 20)
	entries, total, err := h.svc.ListWasteTruckHistory(r.Context(), ac, rtID, page, pageSize)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "not authorized to view dispatch history")
			return
		}
		httpx.WriteInternalServerError(w, "failed to list waste truck dispatch history")
		return
	}
	totalPages := int(math.Ceil(float64(total) / float64(min(pageSize, 100))))
	if totalPages == 0 {
		totalPages = 1
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": entries,
		"pagination": map[string]any{
			"page": page, "per_page": min(pageSize, 100),
			"total": total, "total_pages": totalPages,
		},
	})
}

// TriggerManualPush allows authorized admins to dispatch manual broadcast notifications.
func (h *Handler) TriggerManualPush(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	allowed, authErr := h.canDispatchNotificationEvent(r.Context(), ac, EventManualPush)
	if authErr != nil {
		httpx.WriteInternalServerError(w, "failed to check notification permission")
		return
	}
	if !allowed {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "hanya officer RT yang berwenang mengirim notifikasi manual")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
	allowed, authErr := h.canDispatchNotificationEvent(r.Context(), ac, eventType)
	if authErr != nil {
		httpx.WriteInternalServerError(w, "failed to check notification upload permission")
		return
	}
	if !allowed {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "hanya officer RT atau petugas keamanan dengan permission notification.send_waste_truck yang berwenang mengunggah gambar notifikasi")
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
	// Derive effective RTID: header RT overrides for Super Admin,
	// otherwise use JWT RTID. Then delegate to service for validation.
	// NOTE: The RequireAllowedRT middleware (if present) already validated
	// the RTID. If middleware is missing, ResolveTenantRT will still
	// enforce that the resolved RTID is valid — it will NOT silently fall
	// back to a default RT.
	headerRT := r.Header.Get("X-RT-ID")
	return h.svc.ResolveTenantRT(r.Context(), ac, headerRT, r.URL.Query().Get("rt_id"))
}

// GetWasteTruckTemplate returns the active waste_truck_arrival template for
// an authorized officer or security officer in the caller's RT. Reading the
// template does not grant permission to dispatch it.
func (h *Handler) GetWasteTruckTemplate(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !canViewWasteTruckNotifications(ac) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "hanya officer RT atau petugas keamanan yang dapat melihat notifikasi truk sampah")
		return
	}

	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	isActive := true
	templates, err := h.svc.ListTemplates(r.Context(), rtID, EventWasteTruckArrival, &isActive)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to load template")
		return
	}
	if len(templates) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "template waste_truck_arrival tidak tersedia untuk RT ini")
		return
	}
	tmpl := templates[0]

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"id":         tmpl.ID,
			"event_type": tmpl.EventType,
			"name":       tmpl.Name,
			"title":      tmpl.Title,
			"body":       tmpl.Body,
			"image_url":  tmpl.ImageURL,
			"is_active":  tmpl.IsActive,
			"variables":  tmpl.Variables,
		},
		"message": "Template waste_truck_arrival untuk RT",
	})
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
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
	status := r.URL.Query().Get("status")

	logs, total, err := h.svc.ListLogs(r.Context(), ac, rtID, eventType, status, page, pageSize)
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

// ListManagementHistory returns categorized, server-scoped notification activity
// for notification-management users.
func (h *Handler) ListManagementHistory(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category != HistoryCategorySystem && category != HistoryCategoryMine {
		httpx.WriteValidationError(w, "category must be system or mine", nil)
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		message := "invalid tenant context"
		if err != nil {
			message = err.Error()
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", message)
		return
	}

	page := parsePositiveQueryInt(r, "page", 1)
	pageSize := parsePositiveQueryInt(r, "page_size", 20)
	entries, total, err := h.svc.ListManagementHistory(r.Context(), ac, rtID, category, page, pageSize)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "notification management access is required")
			return
		}
		httpx.WriteInternalServerError(w, "failed to list categorized notification history")
		return
	}
	if entries == nil {
		entries = []NotificationLogEntry{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": entries,
		"pagination": map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetUnreadCount returns unread notifications count for the caller.
func (h *Handler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	count, err := h.svc.GetUnreadCount(r.Context(), ac, rtID)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to get unread count")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"unread_count": count,
	})
}

// MarkAsRead marks a single notification as read.
func (h *Handler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteValidationError(w, "notification id is required", nil)
		return
	}

	if err := h.svc.MarkAsRead(r.Context(), ac, rtID, id); err != nil {
		if errors.Is(err, ErrForbidden) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(w, "notification not found")
			return
		}
		httpx.WriteInternalServerError(w, "failed to mark notification as read")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	})
}

// MarkAllAsRead marks all eligible unread notifications as read for current user.
func (h *Handler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	updated, err := h.svc.MarkAllAsRead(r.Context(), ac, rtID)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to mark all notifications as read")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":       true,
		"updated_count": updated,
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteValidationError(w, "notification id is required", nil)
		return
	}

	canManage, err := auth.HasNotificationManage(r.Context(), h.pool, ac)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to check notification permission")
		return
	}
	entry, err := h.svc.GetNotificationDetail(r.Context(), ac, rtID, id, canManage)
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

// ListSchedules returns the notification schedules for the tenant RT.
func (h *Handler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	eventType := r.URL.Query().Get("event_type")
	var isActive *bool
	if activeStr := r.URL.Query().Get("is_active"); activeStr != "" {
		val := activeStr == "true" || activeStr == "1"
		isActive = &val
	}

	schedules, err := h.svc.ListSchedules(r.Context(), rtID, eventType, isActive)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to list notification schedules")
		return
	}
	if schedules == nil {
		schedules = []*NotificationSchedule{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": schedules,
	})
}

// ListSchedulerHistory returns aggregated scheduler execution history for the
// tenant RT. One row per (schedule, execution date). Accessible to
// notification.manage users (Ketua RT / Super Admin).
func (h *Handler) ListSchedulerHistory(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	pageSize := 20
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil && val > 0 {
			pageSize = val
		}
	} else if ps := r.URL.Query().Get("page"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil && val > 0 {
			pageSize = val
		}
	}

	history, total, err := h.svc.ListSchedulerHistory(r.Context(), rtID, page, pageSize)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to list scheduler history")
		return
	}
	if history == nil {
		history = []*NotificationSchedulerHistory{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": history,
		"pagination": map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetSchedulerRecipientDetail returns individual recipient delivery results for
// a single scheduler execution. Accessible to notification.manage users.
func (h *Handler) GetSchedulerRecipientDetail(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	scheduleID := strings.TrimSpace(chi.URLParam(r, "schedule_id"))
	if scheduleID == "" {
		httpx.WriteValidationError(w, "schedule id is required", nil)
		return
	}

	execDate := r.URL.Query().Get("exec_date")
	if execDate == "" {
		httpx.WriteValidationError(w, "exec_date is required", nil)
		return
	}

	configHash := strings.TrimSpace(r.URL.Query().Get("config_hash"))
	details, err := h.svc.GetSchedulerRecipientDetail(r.Context(), rtID, scheduleID, execDate, configHash)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to get scheduler recipient detail")
		return
	}
	if details == nil {
		details = []*NotificationSchedulerRecipientDetail{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": details,
	})
}

// GetSchedule returns detail of a single notification schedule.
func (h *Handler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteValidationError(w, "schedule id is required", nil)
		return
	}

	schedule, err := h.svc.GetSchedule(r.Context(), rtID, id)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to get notification schedule")
		return
	}
	if schedule == nil {
		httpx.WriteNotFound(w, "schedule not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": schedule,
	})
}

// CreateSchedule creates a new notification schedule.
func (h *Handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	var in CreateScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	schedule, err := h.svc.CreateSchedule(r.Context(), rtID, ac.UserID, in)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": schedule,
	})
}

// UpdateSchedule updates an existing notification schedule.
func (h *Handler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteValidationError(w, "schedule id is required", nil)
		return
	}

	var in UpdateScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	schedule, err := h.svc.UpdateSchedule(r.Context(), rtID, id, ac.UserID, in)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(w, "schedule not found")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": schedule,
	})
}

// DeleteSchedule deletes a notification schedule.
func (h *Handler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteValidationError(w, "schedule id is required", nil)
		return
	}

	if err := h.svc.DeleteSchedule(r.Context(), rtID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(w, "schedule not found")
			return
		}
		httpx.WriteInternalServerError(w, "failed to delete notification schedule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ToggleScheduleActive toggles the is_active state of a notification schedule.
func (h *Handler) ToggleScheduleActive(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httpx.WriteValidationError(w, "schedule id is required", nil)
		return
	}

	schedule, err := h.svc.ToggleScheduleActive(r.Context(), rtID, id, ac.UserID)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to toggle schedule status")
		return
	}
	if schedule == nil {
		httpx.WriteNotFound(w, "schedule not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": schedule,
	})
}
