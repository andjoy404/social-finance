package notification

import (
	"encoding/json"
	"net/http"

	"social-finance/internal/auth"
	httpx "social-finance/internal/http"
)

// Handler handles HTTP requests for notifications and device token management.
type Handler struct {
	svc    Service
	appEnv string
}

// NewHandler creates a new Handler instance.
func NewHandler(svc Service, appEnv ...string) *Handler {
	env := "development"
	if len(appEnv) > 0 && appEnv[0] != "" {
		env = appEnv[0]
	}
	return &Handler{svc: svc, appEnv: env}
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
	if ac == nil || ac.UserID == "" || ac.RTID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing tenant auth context")
		return
	}

	var in WasteTruckNotificationInput
	_ = json.NewDecoder(r.Body).Decode(&in)

	h.svc.NotifyWasteTruckArrival(r.Context(), ac.RTID, ac.UserID, in.Message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "accepted",
		"message": "Waste truck arrival notification dispatched",
	})
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

