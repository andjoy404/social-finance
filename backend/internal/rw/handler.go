package rw

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	"social-finance/internal/notification"
	"social-finance/internal/storage"
)

// Handler provides HTTP handlers for RW operational endpoints.
type Handler struct {
	pool    *database.Pool
	svc     *Service
	nsvc    notification.Service
	storage storage.Service
}

func NewHandler(pool *database.Pool, svc *Service, nsvc notification.Service, store ...storage.Service) *Handler {
	var s storage.Service
	if len(store) > 0 {
		s = store[0]
	}
	return &Handler{
		pool:    pool,
		svc:     svc,
		nsvc:    nsvc,
		storage: s,
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

// UpdateOpeningBalance handles POST /api/v1/rw/{rwId}/opening-balance
func (h *Handler) UpdateOpeningBalance(w http.ResponseWriter, r *http.Request) {
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

	var in UpdateOpeningBalanceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if in.OpeningBalance == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "opening_balance is required")
		return
	}

	if err := h.svc.SetOpeningBalance(r.Context(), ac, rwID, in.OpeningBalance); err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "opening balance updated successfully",
	})
}

// AllocateOpeningBalance handles POST /api/v1/rw/{rwId}/locations/{id}/opening-balance
func (h *Handler) AllocateOpeningBalance(w http.ResponseWriter, r *http.Request) {
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

	locID := chi.URLParam(r, "id")
	if locID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "location id is required")
		return
	}

	var in AllocateOpeningBalanceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}
	in.LocationID = locID

	loc, err := h.svc.AllocateOpeningBalance(r.Context(), ac, rwID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": loc,
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

	search := r.URL.Query().Get("search")
	page, pageSize := parsePagination(r)

	list, total, err := h.svc.GetTransactions(r.Context(), ac, rwID, period, page, pageSize, startDate, endDate, search)
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

// UploadProof handles POST /api/v1/rw/{rwId}/kas/transactions/{id}/proof
func (h *Handler) UploadProof(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	txID := chi.URLParam(r, "id")
	if rwID == "" || txID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId and id are required")
		return
	}

	if h.storage == nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "storage service unavailable")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+1024)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.WriteValidationError(w, "invalid multipart form or file too large (max 5MB)", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("proof")
	}
	if err != nil {
		httpx.WriteValidationError(w, "missing file in 'file' or 'proof' field", nil)
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		httpx.WriteValidationError(w, "uploaded file is empty", nil)
		return
	}
	if header.Size > 5*1024*1024 {
		httpx.WriteValidationError(w, "file size exceeds 5MB limit", nil)
		return
	}

	meta, err := h.storage.UploadProof(r.Context(), rwID, txID, header.Filename, file, header.Size)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidFormat) || errors.Is(err, storage.ErrFileTooLarge) || errors.Is(err, storage.ErrEmptyFile) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to upload proof")
		return
	}

	if err := h.svc.AttachTransactionProof(r.Context(), ac, rwID, txID, meta); err != nil {
		_ = h.storage.DeleteProof(r.Context(), meta.Key)
		h.handleError(w, err)
		return
	}

	proofURL := fmt.Sprintf("/api/v1/rw/%s/kas/transactions/%s/proof", rwID, txID)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "uploaded",
		"proof_url":  proofURL,
		"proof_path": meta.Key,
	})
}

// GetProof handles GET /api/v1/rw/{rwId}/kas/transactions/{id}/proof
func (h *Handler) GetProof(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	txID := chi.URLParam(r, "id")
	if rwID == "" || txID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId and id are required")
		return
	}

	if h.storage == nil {
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "storage service unavailable")
		return
	}

	proofPath, mimeType, err := h.svc.GetTransactionProofInfo(r.Context(), ac, rwID, txID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	reader, err := h.storage.GetProof(r.Context(), proofPath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "proof image not found")
			return
		}
		httpx.ErrorJSON(w, http.StatusInternalServerError, "internal_error", "failed to fetch proof image")
		return
	}
	defer reader.Body.Close()

	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	} else if reader.ContentType != "" {
		w.Header().Set("Content-Type", reader.ContentType)
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}

	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, reader.Body)
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

var rePlaceholder = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)

func extractPlaceholders(texts ...string) []string {
	seen := make(map[string]bool)
	var list []string
	for _, text := range texts {
		matches := rePlaceholder.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) > 1 && !seen[m[1]] {
				seen[m[1]] = true
				list = append(list, m[1])
			}
		}
	}
	if list == nil {
		list = []string{}
	}
	return list
}

func defaultSampleData() map[string]string {
	return map[string]string{
		"nama":              "Budi Santoso",
		"bill_id":           "BILL-2026-001",
		"period":            "November 2026",
		"amount":            "Rp 50.000",
		"total_outstanding": "Rp 150.000",
		"bill_count":        "3",
		"due_date":          "10 November 2026",
		"rw_name":           "RW 05",
		"rt_name":           "RT 02",
		"category":          "Operasional",
		"location":          "Kas Utama RW",
		"status":            "Diverifikasi",
		"rejection_reason":  "Bukti transfer tidak terbaca",
		"deep_link":         "https://socialfinance.id",
	}
}

func renderTemplateWithSamples(title, body string, overrides map[string]string) (string, string, []string, map[string]string) {
	placeholders := extractPlaceholders(title, body)
	samples := defaultSampleData()
	usedSamples := make(map[string]string)
	for _, p := range placeholders {
		if val, ok := overrides[p]; ok && strings.TrimSpace(val) != "" {
			usedSamples[p] = val
		} else if val, ok := samples[p]; ok {
			usedSamples[p] = val
		} else {
			usedSamples[p] = fmt.Sprintf("[%s]", p)
		}
	}

	renderedTitle := rePlaceholder.ReplaceAllStringFunc(title, func(m string) string {
		varName := strings.TrimSuffix(strings.TrimPrefix(m, "{{"), "}}")
		if v, ok := usedSamples[varName]; ok {
			return v
		}
		return m
	})
	renderedBody := rePlaceholder.ReplaceAllStringFunc(body, func(m string) string {
		varName := strings.TrimSuffix(strings.TrimPrefix(m, "{{"), "}}")
		if v, ok := usedSamples[varName]; ok {
			return v
		}
		return m
	})
	return renderedTitle, renderedBody, placeholders, usedSamples
}

// RWManualPushRequest is the payload for sending a manual push notification to RW and RT officials.
type RWManualPushRequest struct {
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	TemplateID string            `json:"template_id,omitempty"`
	Target     string            `json:"target,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"`
}

// SendManualPush handles POST /api/v1/rw/{rwId}/notifications/manual
func (h *Handler) SendManualPush(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req RWManualPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)

	if req.TemplateID != "" && (title == "" || body == "") {
		tmpl, err := h.nsvc.GetRWTemplateByID(r.Context(), rwID, req.TemplateID)
		if err != nil {
			h.handleError(w, err)
			return
		}
		if tmpl == nil {
			httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "template not found")
			return
		}
		if !tmpl.IsActive {
			httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "template sedang tidak aktif")
			return
		}
		rTitle, rBody, _, _ := renderTemplateWithSamples(tmpl.Title, tmpl.Body, req.Variables)
		if title == "" {
			title = rTitle
		}
		if body == "" {
			body = rBody
		}
	}

	count, err := h.svc.SendManualPush(r.Context(), ac, rwID, title, body, req.Target)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"sent_count": count,
		},
	})
}

// PreviewRWTemplate handles POST and GET /api/v1/rw/{rwId}/notifications/templates/{id}/preview
func (h *Handler) PreviewRWTemplate(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	id := chi.URLParam(r, "id")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.read"); err != nil {
		h.handleError(w, err)
		return
	}

	tmpl, err := h.nsvc.GetRWTemplateByID(r.Context(), rwID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if tmpl == nil {
		httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "template not found")
		return
	}

	var overrides map[string]string
	if r.Method == http.MethodPost && r.Body != nil {
		var req struct {
			Variables map[string]string `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		overrides = req.Variables
	}

	renderedTitle, renderedBody, placeholders, sampleData := renderTemplateWithSamples(tmpl.Title, tmpl.Body, overrides)

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"template_id":    tmpl.ID,
			"name":           tmpl.Name,
			"event_type":     tmpl.EventType,
			"original_title": tmpl.Title,
			"original_body":  tmpl.Body,
			"rendered_title": renderedTitle,
			"rendered_body":  renderedBody,
			"placeholders":   placeholders,
			"sample_data":    sampleData,
			"is_active":      tmpl.IsActive,
		},
	})
}

// PreviewRWTemplateDirect handles POST /api/v1/rw/{rwId}/notifications/templates/preview
// It allows previewing template title and body directly without saving.
func (h *Handler) PreviewRWTemplateDirect(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.read"); err != nil {
		h.handleError(w, err)
		return
	}

	var req struct {
		Name      string            `json:"name"`
		EventType string            `json:"event_type"`
		Title     string            `json:"title"`
		Body      string            `json:"body"`
		Variables map[string]string `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	renderedTitle, renderedBody, placeholders, sampleData := renderTemplateWithSamples(req.Title, req.Body, req.Variables)

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"name":           req.Name,
			"event_type":     req.EventType,
			"original_title": req.Title,
			"original_body":  req.Body,
			"rendered_title": renderedTitle,
			"rendered_body":  renderedBody,
			"placeholders":   placeholders,
			"sample_data":    sampleData,
		},
	})
}

// SendRWTemplateNotification handles POST /api/v1/rw/{rwId}/notifications/templates/{id}/send
func (h *Handler) SendRWTemplateNotification(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	id := chi.URLParam(r, "id")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.write"); err != nil {
		h.handleError(w, err)
		return
	}

	tmpl, err := h.nsvc.GetRWTemplateByID(r.Context(), rwID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if tmpl == nil {
		httpx.ErrorJSON(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if !tmpl.IsActive {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "template sedang tidak aktif")
		return
	}

	var req struct {
		Target    string            `json:"target"`
		Variables map[string]string `json:"variables"`
		Title     string            `json:"title"`
		Body      string            `json:"body"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	renderedTitle, renderedBody, _, _ := renderTemplateWithSamples(tmpl.Title, tmpl.Body, req.Variables)
	if strings.TrimSpace(req.Title) != "" {
		renderedTitle = strings.TrimSpace(req.Title)
	}
	if strings.TrimSpace(req.Body) != "" {
		renderedBody = strings.TrimSpace(req.Body)
	}

	target := strings.TrimSpace(req.Target)
	count, err := h.svc.SendManualPush(r.Context(), ac, rwID, renderedTitle, renderedBody, target)
	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"sent_count":    count,
			"template_id":   tmpl.ID,
			"template_name": tmpl.Name,
			"title":         renderedTitle,
			"body":          renderedBody,
			"target":        target,
		},
	})
}

// ListRWTemplates handles GET /api/v1/rw/{rwId}/notifications/templates
func (h *Handler) ListRWTemplates(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.read"); err != nil {
		h.handleError(w, err)
		return
	}
	eventType := r.URL.Query().Get("event_type")

	templates, err := h.nsvc.ListRWTemplates(r.Context(), rwID, eventType, nil)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if templates == nil {
		templates = []notification.NotificationTemplate{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": templates,
	})
}

// GetRWTemplateByID handles GET /api/v1/rw/{rwId}/notifications/templates/{id}
func (h *Handler) GetRWTemplateByID(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	id := chi.URLParam(r, "id")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.read"); err != nil {
		h.handleError(w, err)
		return
	}

	tmpl, err := h.nsvc.GetRWTemplateByID(r.Context(), rwID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if tmpl == nil {
		httpx.WriteNotFound(w, "template not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": tmpl,
	})
}

// CreateRWTemplate handles POST /api/v1/rw/{rwId}/notifications/templates
func (h *Handler) CreateRWTemplate(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.write"); err != nil {
		h.handleError(w, err)
		return
	}
	var in notification.CreateTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	tmpl, err := h.nsvc.CreateRWTemplate(r.Context(), rwID, in)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"data": tmpl,
	})
}

// UpdateRWTemplate handles PUT /api/v1/rw/{rwId}/notifications/templates/{id}
func (h *Handler) UpdateRWTemplate(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	id := chi.URLParam(r, "id")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.write"); err != nil {
		h.handleError(w, err)
		return
	}
	var in notification.UpdateTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	tmpl, err := h.nsvc.UpdateRWTemplate(r.Context(), rwID, id, in)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": tmpl,
	})
}

// ToggleRWTemplateActive handles PATCH /api/v1/rw/{rwId}/notifications/templates/{id}/toggle
func (h *Handler) ToggleRWTemplateActive(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	id := chi.URLParam(r, "id")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.write"); err != nil {
		h.handleError(w, err)
		return
	}

	tmpl, err := h.nsvc.ToggleRWTemplateActive(r.Context(), rwID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if tmpl == nil {
		httpx.WriteNotFound(w, "template not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": tmpl,
	})
}

// DeleteRWTemplate handles DELETE /api/v1/rw/{rwId}/notifications/templates/{id}
func (h *Handler) DeleteRWTemplate(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	id := chi.URLParam(r, "id")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RequirePermission(r.Context(), ac, rwID, "rw.notification.write"); err != nil {
		h.handleError(w, err)
		return
	}

	err := h.nsvc.DeleteRWTemplate(r.Context(), rwID, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "template deleted successfully",
	})
}

// ListNotifications handles GET /api/v1/rw/{rwId}/notifications
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "rwId")
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	page, pageSize := parsePagination(r)
	eventType := r.URL.Query().Get("event_type")

	logs, total, err := h.svc.ListNotifications(r.Context(), ac, rwID, eventType, page, pageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if logs == nil {
		logs = []notification.NotificationLogEntry{}
	}
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages < 1 {
		totalPages = 1
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": logs,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
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

// GetRTRs handles GET /api/v1/rw/{rwId}/rts — lists RTs under an RW for dropdown selection.
func (h *Handler) GetRTRs(w http.ResponseWriter, r *http.Request) {
	if _, ok := getAuthContext(r); !ok {
		httpx.ErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	rwID := chi.URLParam(r, "rwId")
	if rwID == "" {
		httpx.ErrorJSON(w, http.StatusBadRequest, "bad_request", "rwId is required")
		return
	}

	rows, err := h.pool.Raw().QueryContext(r.Context(),
		`SELECT id, name, rt, rw_id
		 FROM rts
		 WHERE rw_id = $1 AND is_active = true
		 ORDER BY rt ASC`,
		rwID,
	)
	if err != nil {
		h.handleError(w, err)
		return
	}
	defer rows.Close()

	type rtItem struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		RT     string `json:"rt"`
		RWID   string `json:"rw_id"`
		Label  string `json:"label"`
	}

	var rts []rtItem
	for rows.Next() {
		var item rtItem
		if err := rows.Scan(&item.ID, &item.Name, &item.RT, &item.RWID); err != nil {
			h.handleError(w, err)
			return
		}
		item.Label = fmt.Sprintf("RT %s — %s", item.RT, item.Name)
		rts = append(rts, item)
	}
	if rts == nil {
		rts = []rtItem{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": rts,
	})
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {	switch {
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
