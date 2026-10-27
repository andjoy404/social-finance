package fundlocation

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

type Handler struct {
	pool *database.Pool
	svc  *Service
}

func NewHandler(pool *database.Pool, svc *Service) *Handler {
	return &Handler{
		pool: pool,
		svc:  svc,
	}
}

func getAuthContext(r *http.Request) (*auth.AuthContext, bool) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		return nil, false
	}
	return ac, true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func parsePagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			pageSize = n
		}
	}
	return page, pageSize
}

func (h *Handler) resolveRTID(r *http.Request, ac *auth.AuthContext) string {
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		if q := r.URL.Query().Get("rt_id"); q != "" {
			return q
		}
	}
	return ac.RTID
}

func (h *Handler) checkWriteAccess(ac *auth.AuthContext) error {
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		return ErrSuperAdminWriteBlocked
	}
	if ac.TenantRole == auth.RoleWarga && ac.Jabatan == "" {
		return ErrWargaBlocked
	}
	return nil
}

func (h *Handler) checkBendaharaAccess(ac *auth.AuthContext) error {
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		return ErrSuperAdminWriteBlocked
	}
	if ac.TenantRole != auth.RoleBendahara && ac.Jabatan != auth.JabatanBendahara {
		return errors.New("only bendahara can execute or reverse fund transfers")
	}
	return nil
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.ErrorJSON(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, ErrDuplicateName):
		httpx.ErrorJSON(w, http.StatusConflict, "duplicate_name", err.Error())
	case errors.Is(err, ErrAlreadyReversed), errors.Is(err, ErrCannotReverseReversal):
		httpx.ErrorJSON(w, http.StatusConflict, "already_reversed", err.Error())
	case errors.Is(err, ErrCannotDeactivateWithBalance):
		httpx.ErrorJSON(w, http.StatusUnprocessableEntity, "cannot_deactivate_with_balance", err.Error())
	case errors.Is(err, ErrInsufficientFunds):
		httpx.ErrorJSON(w, http.StatusUnprocessableEntity, "insufficient_funds", err.Error())
	case errors.Is(err, ErrInsufficientReversal):
		httpx.ErrorJSON(w, http.StatusUnprocessableEntity, "insufficient_reversal_balance", err.Error())
	case errors.Is(err, ErrInsufficientUnallocated):
		httpx.ErrorJSON(w, http.StatusUnprocessableEntity, "insufficient_unallocated_balance", err.Error())
	case errors.Is(err, ErrInactiveLocation):
		httpx.ErrorJSON(w, http.StatusUnprocessableEntity, "inactive_location", err.Error())
	case errors.Is(err, ErrSameLocationTransfer), errors.Is(err, ErrInvalidCategory),
		errors.Is(err, ErrMissingBankName), errors.Is(err, ErrInvalidAmount):
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, ErrSuperAdminWriteBlocked), errors.Is(err, ErrWargaBlocked):
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", err.Error())
	default:
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// GET /api/v1/fund-locations
func (h *Handler) ListLocations(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if ac.TenantRole == auth.RoleWarga && ac.Jabatan == "" {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "warga cannot access fund locations")
		return
	}
	rtID := h.resolveRTID(r, ac)
	if rtID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rt_id is required")
		return
	}

	var cat *string
	if c := r.URL.Query().Get("category"); c != "" {
		cLower := strings.ToLower(strings.TrimSpace(c))
		cat = &cLower
	}
	var isActive *bool
	if a := r.URL.Query().Get("is_active"); a != "" {
		b := a == "true" || a == "1"
		isActive = &b
	}

	list, err := h.svc.ListLocationsByRT(r.Context(), rtID, cat, isActive)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if list == nil {
		list = []*FundLocation{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// POST /api/v1/fund-locations
func (h *Handler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if err := h.checkWriteAccess(ac); err != nil {
		h.handleError(w, err)
		return
	}
	rtID := h.resolveRTID(r, ac)
	if rtID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rt_id is required")
		return
	}

	var in CreateFundLocationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	loc, err := h.svc.CreateLocation(r.Context(), rtID, ac.UserID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": loc})
}

// GET /api/v1/fund-locations/{id}
func (h *Handler) GetLocation(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if ac.TenantRole == auth.RoleWarga && ac.Jabatan == "" {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "warga cannot access fund locations")
		return
	}
	rtID := h.resolveRTID(r, ac)
	id := chi.URLParam(r, "id")

	loc, err := h.svc.GetLocationByID(r.Context(), rtID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": loc})
}

// PATCH /api/v1/fund-locations/{id}
func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if err := h.checkWriteAccess(ac); err != nil {
		h.handleError(w, err)
		return
	}
	rtID := h.resolveRTID(r, ac)
	id := chi.URLParam(r, "id")

	var in UpdateFundLocationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	loc, err := h.svc.UpdateLocation(r.Context(), rtID, ac.UserID, id, in)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": loc})
}

// DELETE /api/v1/fund-locations/{id}
func (h *Handler) DeactivateLocation(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if err := h.checkWriteAccess(ac); err != nil {
		h.handleError(w, err)
		return
	}
	rtID := h.resolveRTID(r, ac)
	id := chi.URLParam(r, "id")

	loc, err := h.svc.DeactivateLocation(r.Context(), rtID, ac.UserID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": loc, "message": "location successfully deactivated"})
}

// GET /api/v1/fund-locations/{id}/history
func (h *Handler) GetLocationHistory(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if ac.TenantRole == auth.RoleWarga && ac.Jabatan == "" {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "warga cannot access fund locations")
		return
	}
	rtID := h.resolveRTID(r, ac)
	id := chi.URLParam(r, "id")
	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	list, total, err := h.svc.GetLocationHistory(r.Context(), rtID, id, offset, pageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if list == nil {
		list = []*LocationBalanceChange{}
	}
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	writeJSON(w, http.StatusOK, map[string]any{
		"data": list,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// PUT /api/v1/fund-locations/{id}/opening-balance
func (h *Handler) SetOpeningBalance(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if err := h.checkWriteAccess(ac); err != nil {
		h.handleError(w, err)
		return
	}
	rtID := h.resolveRTID(r, ac)
	id := chi.URLParam(r, "id")

	var in SetOpeningBalanceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	loc, err := h.svc.SetOpeningBalance(r.Context(), rtID, ac.UserID, id, in)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": loc})
}

// POST /api/v1/fund-transfers
func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if err := h.checkBendaharaAccess(ac); err != nil {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	rtID := h.resolveRTID(r, ac)

	var in CreateFundTransferInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	tr, err := h.svc.CreateTransfer(r.Context(), rtID, ac.UserID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": tr})
}

// GET /api/v1/fund-transfers
func (h *Handler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if ac.TenantRole == auth.RoleWarga && ac.Jabatan == "" {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "warga cannot access fund transfers")
		return
	}
	rtID := h.resolveRTID(r, ac)
	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	list, total, err := h.svc.ListTransfersByRT(r.Context(), rtID, offset, pageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if list == nil {
		list = []*FundTransfer{}
	}
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	writeJSON(w, http.StatusOK, map[string]any{
		"data": list,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GET /api/v1/fund-transfers/{id}
func (h *Handler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if ac.TenantRole == auth.RoleWarga && ac.Jabatan == "" {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "warga cannot access fund transfers")
		return
	}
	rtID := h.resolveRTID(r, ac)
	id := chi.URLParam(r, "id")

	tr, err := h.svc.GetTransferByID(r.Context(), rtID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tr})
}

// POST /api/v1/fund-transfers/{id}/reverse
func (h *Handler) ReverseTransfer(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if err := h.checkBendaharaAccess(ac); err != nil {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	rtID := h.resolveRTID(r, ac)
	id := chi.URLParam(r, "id")

	var in ReverseFundTransferInput
	_ = json.NewDecoder(r.Body).Decode(&in)

	rev, err := h.svc.ReverseTransfer(r.Context(), rtID, ac.UserID, id, in)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rev})
}

// GET /api/v1/reports/fund-locations/reconciliation
func (h *Handler) GetReconciliationReport(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}
	if ac.TenantRole == auth.RoleWarga && ac.Jabatan == "" {
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "warga cannot access fund reconciliation")
		return
	}
	rtID := h.resolveRTID(r, ac)
	if rtID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rt_id is required")
		return
	}

	report, err := h.svc.GetReconciliationReport(r.Context(), rtID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": report})
}
