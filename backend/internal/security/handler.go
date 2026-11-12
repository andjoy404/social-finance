package security

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/storage"
	httpx "social-finance/internal/http"
)

// MonitoringReaderFunc injects RT-scoped uncollected waste monitoring lookup without coupling packages.
type MonitoringReaderFunc func(ctx context.Context, rtID string, evalDate time.Time) (any, error)

type Handler struct {
	svc           *Service
	pool          *database.Pool
	resolver      *auth.PermissionResolver
	monitorReader MonitoringReaderFunc
	store         storage.Service
}

func NewHandler(svc *Service, pool *database.Pool, monitorReader MonitoringReaderFunc, store storage.Service) *Handler {
	return &Handler{
		svc:           svc,
		pool:          pool,
		resolver:      auth.NewPermissionResolver(pool),
		monitorReader: monitorReader,
		store:         store,
	}
}

func (h *Handler) hasAnyPermission(ctx context.Context, ac *auth.AuthContext, codes ...string) (bool, error) {
	if ac == nil {
		return false, nil
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true, nil
	}
	if ac.Jabatan == "" {
		return false, nil
	}
	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, code := range codes {
		ok, err := h.resolver.HasPermission(ctx, tx, ac, code)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message)
		return
	}
	slog.Error("security handler internal error", "error", err)
	httpx.WriteInternalServerError(w, "internal security module error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writePaginated(w http.ResponseWriter, data any, page, perPage, total int) {
	totalPages := 0
	if perPage > 0 && total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":        page,
			"per_page":    perPage,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (h *Handler) authAndRT(w http.ResponseWriter, r *http.Request) (*auth.AuthContext, string, bool) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteUnauthorized(w, "missing authentication context")
		return nil, "", false
	}
	rtID, err := h.svc.ResolveTenantRT(r.Context(), ac, r.Header.Get("X-RT-ID"), r.URL.Query().Get("rt_id"))
	if err != nil {
		writeServiceError(w, err)
		return nil, "", false
	}
	return ac, rtID, true
}

// ── Dashboard & Officers ──────────────────────────────────────────────────────

func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.read")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.read permission")
		return
	}
	sum, err := h.svc.GetDashboardSummary(r.Context(), rtID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (h *Handler) ListOfficers(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.read", "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden")
		return
	}
	canManage, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	var list []SecurityOfficer
	if canManage {
		list, err = h.svc.ListOfficers(r.Context(), rtID)
	} else {
		list, err = h.svc.ListOfficersForAttendance(r.Context(), rtID)
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// ── Shift Configs ─────────────────────────────────────────────────────────────

func (h *Handler) ListShifts(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.read", "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden")
		return
	}
	activeOnly := r.URL.Query().Get("active_only") == "true"
	list, err := h.svc.ListShifts(r.Context(), rtID, activeOnly)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) CreateShift(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.write permission")
		return
	}
	var req UpsertShiftRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	created, err := h.svc.CreateShift(r.Context(), rtID, ac.UserID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateShift(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.write permission")
		return
	}
	shiftID := chi.URLParam(r, "id")
	var req UpsertShiftRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	updated, err := h.svc.UpdateShift(r.Context(), rtID, shiftID, ac.UserID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// ── Schedules ─────────────────────────────────────────────────────────────────

func (h *Handler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.read", "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden")
		return
	}
	canManage, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	pg := httpx.ParsePagination(r)
	q := r.URL.Query()
	userFilter := strings.TrimSpace(q.Get("user_id"))
	if !canManage && ac.Jabatan == auth.JabatanKeamanan {
		userFilter = ac.UserID
	}
	list, total, err := h.svc.ListSchedules(r.Context(), rtID, ScheduleFilter{
		DateFrom:      strings.TrimSpace(q.Get("date_from")),
		DateTo:        strings.TrimSpace(q.Get("date_to")),
		UserID:        userFilter,
		ShiftConfigID: strings.TrimSpace(q.Get("shift_config_id")),
		Status:        strings.TrimSpace(q.Get("status")),
		Page:          pg.Page,
		PerPage:       pg.PerPage,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writePaginated(w, list, pg.Page, pg.PerPage, total)
}

func (h *Handler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.read", "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden")
		return
	}
	if ac.Jabatan == auth.JabatanKeamanan {
		requestedID := chi.URLParam(r, "id")
		selfSchedules, _, listErr := h.svc.ListSchedules(r.Context(), rtID, ScheduleFilter{UserID: ac.UserID, Page: 1, PerPage: 1000})
		if listErr != nil {
			writeServiceError(w, listErr)
			return
		}
		allowed := false
		for _, schedule := range selfSchedules {
			if schedule.ID == requestedID {
				allowed = true
				break
			}
		}
		if !allowed {
			httpx.WriteNotFound(w, "schedule not found")
			return
		}
	}
	items, err := h.svc.GetScheduleAssignment(r.Context(), rtID, chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.write permission")
		return
	}
	var req CreateScheduleRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	created, err := h.svc.CreateSchedule(r.Context(), rtID, ac.UserID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.write permission")
		return
	}
	scheduleID := chi.URLParam(r, "id")
	var req UpdateScheduleRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	updated, err := h.svc.UpdateSchedule(r.Context(), rtID, scheduleID, ac.UserID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) CancelSchedule(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.write permission")
		return
	}
	scheduleID := chi.URLParam(r, "id")
	if err := h.svc.CancelSchedule(r.Context(), rtID, scheduleID, ac.UserID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Shift Swaps ───────────────────────────────────────────────────────────────

func (h *Handler) ListSwaps(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	canManage, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	userFilter := ""
	if !canManage {
		canAttend, err := h.hasAnyPermission(r.Context(), ac, "security.attendance")
		if err != nil {
			httpx.WriteInternalServerError(w, "permission check failed")
			return
		}
		if !canAttend {
			httpx.WriteForbidden(w, "forbidden")
			return
		}
		userFilter = ac.UserID
	} else {
		userFilter = strings.TrimSpace(r.URL.Query().Get("user_id"))
	}

	list, err := h.svc.ListSwaps(r.Context(), rtID, userFilter, strings.TrimSpace(r.URL.Query().Get("status")))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) CreateSwap(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed || ac.SystemRole == auth.SystemRoleSuperAdmin {
		// Only active security officers propose swaps on their own shifts.
		if !allowed {
			httpx.WriteForbidden(w, "forbidden: only security officers can submit shift swaps")
			return
		}
	}
	var req CreateSwapRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	created, err := h.svc.CreateSwap(r.Context(), rtID, ac.UserID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) TargetRespondSwap(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.attendance permission")
		return
	}
	swapID := chi.URLParam(r, "id")
	var req SwapTargetRespondRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	updated, err := h.svc.TargetRespondSwap(r.Context(), rtID, swapID, ac.UserID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) DecideSwap(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.write permission")
		return
	}
	swapID := chi.URLParam(r, "id")
	var req SwapDecideRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	updated, err := h.svc.DecideSwap(r.Context(), rtID, swapID, ac.UserID, req, auth.ClientIP(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// ── Attendance Scan & Monitoring ──────────────────────────────────────────────

func (h *Handler) GetMyAttendanceOverview(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: only security officers can access personal attendance")
		return
	}
	overview, err := h.svc.GetOfficerOverview(r.Context(), rtID, ac.UserID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *Handler) ScanAttendanceQR(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.attendance")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed || ac.SystemRole == auth.SystemRoleSuperAdmin && ac.Jabatan != auth.JabatanKeamanan {
		httpx.WriteForbidden(w, "forbidden: only active security officers can scan attendance QR")
		return
	}
	var req ScanQRRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	res, err := h.svc.ScanQR(r.Context(), rtID, ac.UserID, req, idemKey)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) ListAttendance(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.read")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.read permission")
		return
	}
	canManage, _ := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	q := r.URL.Query()
	userFilter := strings.TrimSpace(q.Get("user_id"))
	if !canManage && ac.Jabatan == auth.JabatanKeamanan {
		userFilter = ac.UserID
	}
	pg := httpx.ParsePagination(r)
	list, total, err := h.svc.ListAttendance(r.Context(), rtID, AttendanceFilter{
		DateFrom:      strings.TrimSpace(q.Get("date_from")),
		DateTo:        strings.TrimSpace(q.Get("date_to")),
		UserID:        userFilter,
		ShiftConfigID: strings.TrimSpace(q.Get("shift_config_id")),
		Status:        strings.TrimSpace(q.Get("status")),
		Page:          pg.Page,
		PerPage:       pg.PerPage,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writePaginated(w, list, pg.Page, pg.PerPage, total)
}

func (h *Handler) ExportAttendanceCSV(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.schedule.write")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.schedule.write permission")
		return
	}
	q := r.URL.Query()
	data, err := h.svc.ExportAttendanceCSV(r.Context(), rtID, AttendanceFilter{
		DateFrom:      strings.TrimSpace(q.Get("date_from")),
		DateTo:        strings.TrimSpace(q.Get("date_to")),
		UserID:        strings.TrimSpace(q.Get("user_id")),
		ShiftConfigID: strings.TrimSpace(q.Get("shift_config_id")),
		Status:        strings.TrimSpace(q.Get("status")),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="absensi-keamanan.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ── Corrections ───────────────────────────────────────────────────────────────

func (h *Handler) ListCorrections(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	isManager, err := h.hasAnyPermission(r.Context(), ac, "security.correction")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	officerFilter := strings.TrimSpace(r.URL.Query().Get("officer_user_id"))
	if !isManager {
		canAttend, err := h.hasAnyPermission(r.Context(), ac, "security.attendance")
		if err != nil {
			httpx.WriteInternalServerError(w, "permission check failed")
			return
		}
		if !canAttend {
			httpx.WriteForbidden(w, "forbidden")
			return
		}
		officerFilter = ac.UserID
	}
	list, err := h.svc.ListCorrections(r.Context(), rtID, officerFilter, strings.TrimSpace(r.URL.Query().Get("status")))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) CreateCorrection(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	isManager, err := h.hasAnyPermission(r.Context(), ac, "security.correction")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !isManager {
		canAttend, err := h.hasAnyPermission(r.Context(), ac, "security.attendance")
		if err != nil {
			httpx.WriteInternalServerError(w, "permission check failed")
			return
		}
		if !canAttend {
			httpx.WriteForbidden(w, "forbidden")
			return
		}
	}
	var req CreateCorrectionRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	created, err := h.svc.CreateCorrection(r.Context(), rtID, ac.UserID, isManager, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) DecideCorrection(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.correction")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.correction permission")
		return
	}
	correctionID := chi.URLParam(r, "id")
	var req DecideCorrectionRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	updated, err := h.svc.DecideCorrection(r.Context(), rtID, correctionID, ac.UserID, req, auth.ClientIP(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// ── Devices Management (Ketua RT / Super Admin) ───────────────────────────────

func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.device.manage")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.device.manage permission")
		return
	}
	list, err := h.svc.ListDevices(r.Context(), rtID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.device.manage")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.device.manage permission")
		return
	}
	var req CreateDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	res, err := h.svc.CreateDevice(r.Context(), rtID, ac.UserID, req, auth.ClientIP(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *Handler) RotateDevicePairing(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.device.manage")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.device.manage permission")
		return
	}
	deviceID := chi.URLParam(r, "id")
	res, err := h.svc.RotatePairing(r.Context(), rtID, deviceID, ac.UserID, auth.ClientIP(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.device.manage")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.device.manage permission")
		return
	}
	deviceID := chi.URLParam(r, "id")
	dev, err := h.svc.RevokeDevice(r.Context(), rtID, deviceID, ac.UserID, auth.ClientIP(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

// ActivateDevice exchanges a one-time pairing credential for a persistent device token (public, rate-limited).
func (h *Handler) ActivateDevice(w http.ResponseWriter, r *http.Request) {
	var req ActivateDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	res, err := h.svc.ActivateDevice(r.Context(), req, auth.ClientIP(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ── Guest Book (Ketua RT Monitoring) ──────────────────────────────────────────

func (h *Handler) ListGuestBooksAdmin(w http.ResponseWriter, r *http.Request) {
	ac, rtID, ok := h.authAndRT(w, r)
	if !ok {
		return
	}
	allowed, err := h.hasAnyPermission(r.Context(), ac, "security.guest_book")
	if err != nil {
		httpx.WriteInternalServerError(w, "permission check failed")
		return
	}
	if !allowed {
		httpx.WriteForbidden(w, "forbidden: missing security.guest_book permission")
		return
	}
	pg := httpx.ParsePagination(r)
	q := r.URL.Query()
	list, total, err := h.svc.ListGuestBooks(r.Context(), rtID, GuestBookFilter{
		Status:   strings.TrimSpace(q.Get("status")),
		DateFrom: strings.TrimSpace(q.Get("date_from")),
		DateTo:   strings.TrimSpace(q.Get("date_to")),
		Search:   strings.TrimSpace(q.Get("search")),
		Page:     pg.Page,
		PerPage:  pg.PerPage,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writePaginated(w, list, pg.Page, pg.PerPage, total)
}

// ── Pos Device Endpoints (/api/v1/security-device/*) ──────────────────────────

func (h *Handler) PosMe(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":      dc.DeviceID,
		"rt_id":          dc.RTID,
		"rt_name":        dc.RTName,
		"rt_code":        dc.RTCode,
		"rw_id":          dc.RWID,
		"device_name":    dc.DeviceName,
		"location_label": dc.LocationLabel,
		"server_time":    h.svc.Now(),
	})
}

func (h *Handler) PosGenerateQR(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	payload, err := h.svc.GeneratePosQR(r.Context(), dc)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

func (h *Handler) PosTodayAttendance(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	today := h.svc.Now().Format("2006-01-02")
	yesterday := h.svc.Now().AddDate(0, 0, -1).Format("2006-01-02")
	list, _, err := h.svc.ListAttendance(r.Context(), dc.RTID, AttendanceFilter{
		DateFrom: yesterday,
		DateTo:   today,
		Page:     1,
		PerPage:  50,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"server_time": h.svc.Now(),
		"data":        list,
	})
}

func (h *Handler) PosListGuestBooks(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	pg := httpx.ParsePagination(r)
	q := r.URL.Query()
	list, total, err := h.svc.ListGuestBooks(r.Context(), dc.RTID, GuestBookFilter{
		Status:   strings.TrimSpace(q.Get("status")),
		DateFrom: strings.TrimSpace(q.Get("date_from")),
		DateTo:   strings.TrimSpace(q.Get("date_to")),
		Search:   strings.TrimSpace(q.Get("search")),
		Page:     pg.Page,
		PerPage:  pg.PerPage,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writePaginated(w, list, pg.Page, pg.PerPage, total)
}

func (h *Handler) PosCreateGuestBook(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	var req CreateGuestBookRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	created, err := h.svc.CreateGuestBook(r.Context(), dc.RTID, dc.DeviceID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) PosCheckoutGuestBook(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	entryID := chi.URLParam(r, "id")
	updated, err := h.svc.CheckoutGuestBook(r.Context(), dc.RTID, dc.DeviceID, entryID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) PosListHouseholds(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	pg := httpx.ParsePagination(r)
	list, total, err := h.svc.ListPosHouseholds(r.Context(), dc.RTID, r.URL.Query().Get("search"), pg.Page, pg.PerPage)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writePaginated(w, list, pg.Page, pg.PerPage, total)
}

func (h *Handler) PosGetHouseholdDetail(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	id := chi.URLParam(r, "id")
	detail, err := h.svc.GetPosHouseholdDetail(r.Context(), dc.RTID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) PosMonitoringSampah(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	if h.monitorReader == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}, "total": 0})
		return
	}
	evalDate := h.svc.Now()
	items, err := h.monitorReader(r.Context(), dc.RTID, evalDate)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":      items,
		"eval_date": evalDate.Format("2006-01-02"),
	})
}
