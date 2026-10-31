package rw

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

// Handler provides HTTP handlers for RW operational endpoints.
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

// GetDashboard handles GET /api/v1/rw/{rwId}/dashboard
func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	if rwID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required")
		return
	}

	summary, err := h.svc.GetDashboard(r.Context(), ac, rwID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": summary,
	})
}

// GetKAS handles GET /api/v1/rw/{rwId}/kas
func (h *Handler) GetKAS(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	if rwID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required")
		return
	}

	period := r.URL.Query().Get("period")
	if period != "" {
		if _, err := ValidatePeriod(period); err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}

	report, err := h.svc.GetKASReport(r.Context(), ac, rwID, period)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": report,
	})
}

// GetKASTransactions handles GET /api/v1/rw/{rwId}/kas/transactions
func (h *Handler) GetKASTransactions(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	if rwID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required")
		return
	}

	period := r.URL.Query().Get("period")
	if period != "" {
		if _, err := ValidatePeriod(period); err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}

	startDate := r.URL.Query().Get("start_date")
	if startDate != "" {
		if _, err := ValidateDate(startDate); err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}

	endDate := r.URL.Query().Get("end_date")
	if endDate != "" {
		if _, err := ValidateDate(endDate); err != nil {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}

	page, pageSize := parsePagination(r)

	list, total, err := h.svc.GetTransactions(r.Context(), ac, rwID, period, page, pageSize, startDate, endDate)
	if err != nil {
		h.handleError(w, err)
		return
	}

	if list == nil {
		list = []*TransactionEntry{}
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

// GetKASTransactionByID handles GET /api/v1/rw/{rwId}/kas/transactions/{id}
func (h *Handler) GetKASTransactionByID(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	id := chi.URLParam(r, "id")

	entry, err := h.svc.GetTransactionByID(r.Context(), ac, rwID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": entry,
	})
}

// CreateKASTransaction handles POST /api/v1/rw/{rwId}/kas and POST /api/v1/rw/{rwId}/kas/transactions
func (h *Handler) CreateKASTransaction(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	if rwID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required")
		return
	}

	var in CreateTransactionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	entry, err := h.svc.CreateTransaction(r.Context(), ac, rwID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"data": entry,
	})
}

// GetLocations handles GET /api/v1/rw/{rwId}/locations
func (h *Handler) GetLocations(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	if rwID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required")
		return
	}

	var isActive *bool
	if a := r.URL.Query().Get("is_active"); a != "" {
		b := a == "true" || a == "1"
		isActive = &b
	}

	locations, err := h.svc.GetLocations(r.Context(), ac, rwID, isActive)
	if err != nil {
		h.handleError(w, err)
		return
	}

	if locations == nil {
		locations = []*LocationSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": locations,
	})
}

// GetLocationByID handles GET /api/v1/rw/{rwId}/locations/{id}
func (h *Handler) GetLocationByID(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	locID := chi.URLParam(r, "id")

	loc, err := h.svc.GetLocationByID(r.Context(), ac, rwID, locID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": loc,
	})
}

// CreateLocation handles POST /api/v1/rw/{rwId}/locations
func (h *Handler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	if rwID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required")
		return
	}

	var in CreateRWFundLocationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	loc, err := h.svc.CreateLocation(r.Context(), ac, rwID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"data": loc,
	})
}

// UpdateLocation handles PUT /api/v1/rw/{rwId}/locations/{id}
func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	locID := chi.URLParam(r, "id")

	var in UpdateRWFundLocationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	loc, err := h.svc.UpdateLocation(r.Context(), ac, rwID, locID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": loc,
	})
}

// ToggleLocationStatus handles PATCH /api/v1/rw/{rwId}/locations/{id}/status
func (h *Handler) ToggleLocationStatus(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	locID := chi.URLParam(r, "id")

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	loc, err := h.svc.ToggleLocationStatus(r.Context(), ac, rwID, locID, body.IsActive)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": loc,
	})
}

// GetLocationMutations handles GET /api/v1/rw/{rwId}/locations/{id}/mutations
func (h *Handler) GetLocationMutations(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	locID := chi.URLParam(r, "id")
	page, pageSize := parsePagination(r)

	mutations, total, err := h.svc.GetLocationMutations(r.Context(), ac, rwID, locID, page, pageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}

	if mutations == nil {
		mutations = []LocationBalanceChange{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	writeJSON(w, http.StatusOK, map[string]any{
		"data": mutations,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetReconciliation handles GET /api/v1/rw/{rwId}/locations/reconcile
func (h *Handler) GetReconciliation(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")

	rep, err := h.svc.GetReconciliation(r.Context(), ac, rwID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": rep,
	})
}

// CreateTransfer handles POST /api/v1/rw/{rwId}/locations/transfers
func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")

	var in CreateFundTransferInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	tr, err := h.svc.CreateTransfer(r.Context(), ac, rwID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"data": tr,
	})
}

// ReverseTransfer handles POST /api/v1/rw/{rwId}/locations/transfers/{id}/reverse
func (h *Handler) ReverseTransfer(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	transferID := chi.URLParam(r, "id")

	var in ReverseFundTransferInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	rev, err := h.svc.ReverseTransfer(r.Context(), ac, rwID, transferID, in.Reason)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": rev,
	})
}

// GetTransfers handles GET /api/v1/rw/{rwId}/locations/transfers
func (h *Handler) GetTransfers(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	page, pageSize := parsePagination(r)

	transfers, total, err := h.svc.GetTransfers(r.Context(), ac, rwID, page, pageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}

	if transfers == nil {
		transfers = []FundTransfer{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	writeJSON(w, http.StatusOK, map[string]any{
		"data": transfers,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// ---- Helper functions ----

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

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRWAccessDenied):
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "rw access denied")
	case errors.Is(err, ErrPermissionDenied):
		httpx.ErrorJSON(w, http.StatusForbidden, "forbidden", "insufficient permissions")
	case errors.Is(err, ErrNotFound):
		httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, ErrDuplicateName), errors.Is(err, ErrAlreadyReversed):
		httpx.ErrorJSON(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, ErrValidation),
		errors.Is(err, ErrCannotDeactivateWithBalance),
		errors.Is(err, ErrSameLocationTransfer),
		errors.Is(err, ErrCrossRWTransfer),
		errors.Is(err, ErrInsufficientFunds),
		errors.Is(err, ErrInsufficientReversal),
		errors.Is(err, ErrCannotReverseReversal),
		errors.Is(err, ErrInactiveLocation),
		errors.Is(err, ErrInvalidCategory),
		errors.Is(err, ErrMissingBankName),
		errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrInvalidMethodCategoryMatch):
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", err.Error())
	default:
		slog.Error("rw internal error", "error", err.Error())
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
