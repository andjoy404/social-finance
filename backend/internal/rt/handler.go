package rt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
)

// Handler handles RT HTTP endpoints.
type Handler struct {
	svc *RTManager
	p   *database.Pool
}

// NewHandler creates a new RT handler.
func NewHandler(svc *RTManager, pool *database.Pool) *Handler {
	return &Handler{svc: svc, p: pool}
}

// CreateRequest is the body of a Create RT POST request.
type CreateRequest struct {
	Name     string  `json:"name"`
	RW       int     `json:"rw"`
	RT       string  `json:"rt"`
	Address  *string `json:"address,omitempty"`
	HeadName *string `json:"head_name,omitempty"`
}

// UpdateRequest is the body of an Update RT PATCH request.
type UpdateRequest struct {
	Name     *string `json:"name,omitempty"`
	RW       *int    `json:"rw,omitempty"`
	RT       *string `json:"rt,omitempty"`
	Address  *string `json:"address,omitempty"`
	HeadName *string `json:"head_name,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// Create handles POST /api/v1/rts — creates a new RT.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.WriteValidationError(w, "name is required", nil)
		return
	}
	if req.RW < 0 {
		httpx.WriteValidationError(w, "rw must be >= 0", nil)
		return
	}
	if strings.TrimSpace(req.RT) == "" {
		httpx.WriteValidationError(w, "rt name is required", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	result, err := h.svc.Create(ctx, tx, CreateRTInput{
		Name:     req.Name,
		RW:       req.RW,
		RT:       req.RT,
		Address:  req.Address,
		HeadName: req.HeadName,
	})
	if err != nil {
		if errors.Is(err, ErrDuplicate) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "RT with this RW and RT code already exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create RT")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create RT")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// GetByID handles GET /api/v1/rts/{id} — returns a single RT.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	item, err := h.svc.GetByID(ctx, tx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "RT not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load RT")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load RT")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// ListAll handles GET /api/v1/rts — returns paginated list of RTs.
func (h *Handler) ListAll(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	var isActive *bool
	if val := r.URL.Query().Get("is_active"); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			isActive = &b
		}
	}

	var search *string
	if val := r.URL.Query().Get("search"); strings.TrimSpace(val) != "" {
		search = &val
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	items, err := h.svc.ListAll(ctx, tx, isActive, search, offset, pageSize)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list RTs")
		return
	}

	total, err := h.svc.Count(ctx, tx, isActive, search)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count RTs")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list RTs")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	resp := map[string]any{
		"data": items,
		"pagination": map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Update handles PATCH /api/v1/rts/{id} — partially updates an RT.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	updateInput := &UpdateRTInput{
		Name:     req.Name,
		RW:       req.RW,
		RT:       req.RT,
		Address:  req.Address,
		HeadName: req.HeadName,
		IsActive: req.IsActive,
	}

	result, err := h.svc.Update(ctx, tx, id, updateInput)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "RT not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update RT")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update RT")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Deactivate handles PATCH /api/v1/rts/{id}/deactivate — deactivates an RT.
func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	if err := h.svc.Deactivate(ctx, tx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "RT not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate RT")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate RT")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
