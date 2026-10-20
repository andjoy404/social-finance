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
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/finance"
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
	RWID     *string `json:"rw_id,omitempty"`
	RW       int     `json:"rw"`
	RT       string  `json:"rt"`
	Address  *string `json:"address,omitempty"`
	HeadName *string `json:"head_name,omitempty"`
}

// UpdateRequest is the body of an Update RT PATCH request.
type UpdateRequest struct {
	Name     *string `json:"name,omitempty"`
	RWID     *string `json:"rw_id,omitempty"`
	RW       *int    `json:"rw,omitempty"`
	RT       *string `json:"rt,omitempty"`
	Address  *string `json:"address,omitempty"`
	HeadName *string `json:"head_name,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// CreateRWRequest is the body of a Create RW POST request.
type CreateRWRequest struct {
	RWNumber int     `json:"rw_number"`
	Name     string  `json:"name"`
	HeadName *string `json:"head_name,omitempty"`
	Address  *string `json:"address,omitempty"`
}

// UpdateRWRequest is the body of an Update RW PATCH request.
type UpdateRWRequest struct {
	RWNumber *int    `json:"rw_number,omitempty"`
	Name     *string `json:"name,omitempty"`
	HeadName *string `json:"head_name,omitempty"`
	Address  *string `json:"address,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

type setJabatanRequest struct {
	Jabatan string `json:"jabatan"`
}

type setRWJabatanRequest struct {
	Jabatan string `json:"jabatan"`
	RTID    string `json:"rt_id"`
}

func auditIP(r *http.Request) *string {
	ip := auth.ClientIP(r)
	if ip == "" {
		return nil
	}
	return &ip
}

func (h *Handler) ListMemberships(w http.ResponseWriter, r *http.Request) {
	rtID := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteInternalServerError(w, "could not begin transaction")
		return
	}
	defer tx.Rollback()
	if _, err := h.svc.GetByID(ctx, tx, rtID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "RT not found")
		} else {
			httpx.WriteInternalServerError(w, "failed to load RT")
		}
		return
	}
	items, err := ListMemberships(ctx, tx, rtID)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to list RT memberships")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "failed to list RT memberships")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) SetJabatan(w http.ResponseWriter, r *http.Request) {
	rtID, userID := chi.URLParam(r, "id"), chi.URLParam(r, "userId")
	var req setJabatanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	jabatan := normalizeJabatan(req.Jabatan)
	if !auth.ValidJabatans[jabatan] {
		httpx.WriteValidationError(w, "jabatan is invalid", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteInternalServerError(w, "could not begin transaction")
		return
	}
	defer tx.Rollback()
	item, old, err := SetJabatan(ctx, tx, rtID, userID, jabatan)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	ac := auth.GetAuthContext(r)
	actor := ac.UserID
	oldJSON, _ := json.Marshal(map[string]any{"jabatan": old})
	newJSON, _ := json.Marshal(map[string]string{"jabatan": string(jabatan)})
	if old != nil && *old == string(jabatan) {
		if err := tx.Commit(); err != nil {
			httpx.WriteInternalServerError(w, "failed to commit jabatan")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if err := finance.AuditLogInsert(ctx, tx, finance.AuditLog{RTID: rtID, UserID: &actor, Action: "update", EntityType: "user_rt_membership.jabatan", EntityID: item.ID, OldValues: oldJSON, NewValues: newJSON, IPAddress: auditIP(r)}); err != nil {
		httpx.WriteInternalServerError(w, "failed to write audit log")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "failed to commit jabatan")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) RemoveJabatan(w http.ResponseWriter, r *http.Request) {
	rtID, userID := chi.URLParam(r, "id"), chi.URLParam(r, "userId")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteInternalServerError(w, "could not begin transaction")
		return
	}
	defer tx.Rollback()
	item, old, err := RemoveJabatan(ctx, tx, rtID, userID)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	ac := auth.GetAuthContext(r)
	actor := ac.UserID
	oldJSON, _ := json.Marshal(map[string]any{"jabatan": old})
	newJSON, _ := json.Marshal(map[string]any{"jabatan": nil})
	if err := finance.AuditLogInsert(ctx, tx, finance.AuditLog{RTID: rtID, UserID: &actor, Action: "revoke", EntityType: "user_rt_membership.jabatan", EntityID: item.ID, OldValues: oldJSON, NewValues: newJSON, IPAddress: auditIP(r)}); err != nil {
		httpx.WriteInternalServerError(w, "failed to write audit log")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "failed to commit jabatan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListRWMemberships(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "id")
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication context is missing")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteInternalServerError(w, "could not begin transaction")
		return
	}
	defer tx.Rollback()
	if _, err := h.svc.GetRWByID(ctx, tx, rwID); err != nil {
		if errors.Is(err, ErrRWNotFound) {
			httpx.WriteNotFound(w, "RW not found")
		} else {
			httpx.WriteInternalServerError(w, "failed to load RW")
		}
		return
	}
	items, err := ListRWMemberships(ctx, tx, rwID)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to list RW memberships")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "failed to list RW memberships")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) SetRWJabatan(w http.ResponseWriter, r *http.Request) {
	rwID := chi.URLParam(r, "id")
	var req setRWJabatanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	jabatan := normalizeJabatan(req.Jabatan)
	jabatanStr := string(jabatan)
	if !AllowedRWJabatans[jabatanStr] {
		httpx.WriteValidationError(w, "jabatan is invalid", nil)
		return
	}
	if strings.TrimSpace(req.RTID) == "" {
		httpx.WriteValidationError(w, "rt_id is required", nil)
		return
	}
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication context is missing")
		return
	}
	actor := ac.UserID

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteInternalServerError(w, "could not begin transaction")
		return
	}
	defer tx.Rollback()
	item, old, err := SetRWJabatan(ctx, tx, rwID, chi.URLParam(r, "userId"), req.RTID, actor, jabatanStr)
	if err != nil {
		h.writeRWAdministrationError(w, err)
		return
	}
	oldJSON, _ := json.Marshal(map[string]any{"jabatan": old})
	newJSON, _ := json.Marshal(map[string]string{"jabatan": jabatanStr})
	if old != nil && *old == jabatanStr {
		if err := tx.Commit(); err != nil {
			httpx.WriteInternalServerError(w, "failed to commit RW jabatan")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if err := finance.AuditLogInsert(ctx, tx, finance.AuditLog{RTID: req.RTID, UserID: &actor, Action: "update", EntityType: "user_rw_membership.jabatan", EntityID: item.ID, OldValues: oldJSON, NewValues: newJSON, IPAddress: auditIP(r)}); err != nil {
		httpx.WriteInternalServerError(w, "failed to write audit log")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "failed to commit RW jabatan")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) RemoveRWJabatan(w http.ResponseWriter, r *http.Request) {
	rwID, userID := chi.URLParam(r, "id"), chi.URLParam(r, "userId")
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication context is missing")
		return
	}
	actor := ac.UserID

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteInternalServerError(w, "could not begin transaction")
		return
	}
	defer tx.Rollback()
	item, old, err := RemoveRWJabatan(ctx, tx, rwID, userID, actor)
	if err != nil {
		h.writeRWAdministrationError(w, err)
		return
	}
	oldJSON, _ := json.Marshal(map[string]any{"jabatan": old})
	newJSON, _ := json.Marshal(map[string]any{"jabatan": nil})

	var rtID string
	if item.RTID != nil {
		rtID = *item.RTID
	} else if ac.RTID != "" {
		rtID = ac.RTID
	}

	// RW memberships may legitimately lack an associated RT.
	// Only write the audit log when a valid RT is available; the
	// business-logic layer already records history in
	// user_rw_membership_history regardless.
	if rtID != "" {
		if err := finance.AuditLogInsert(ctx, tx, finance.AuditLog{RTID: rtID, UserID: &actor, Action: "revoke", EntityType: "user_rw_membership.jabatan", EntityID: item.ID, OldValues: oldJSON, NewValues: newJSON, IPAddress: auditIP(r)}); err != nil {
			httpx.WriteInternalServerError(w, "failed to write audit log")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "failed to commit RW jabatan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeRWAdministrationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRWNotFound), errors.Is(err, ErrRWMembershipNotFound):
		httpx.WriteNotFound(w, err.Error())
	case errors.Is(err, ErrDuplicateRWJabatan):
		httpx.WriteConflict(w, err.Error())
	case errors.Is(err, ErrInvalidRWJabatan), errors.Is(err, ErrRTNotInRW), errors.Is(err, ErrUserNotRTMember), errors.Is(err, ErrRWInactive):
		httpx.WriteValidationError(w, err.Error(), nil)
	default:
		httpx.WriteInternalServerError(w, "failed to update RW jabatan")
	}
}

func (h *Handler) writeAdministrationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrMembershipNotFound), errors.Is(err, ErrUserNotFound):
		httpx.WriteNotFound(w, err.Error())
	case errors.Is(err, ErrDuplicateJabatan):
		httpx.WriteConflict(w, err.Error())
	case errors.Is(err, ErrInvalidJabatan):
		httpx.WriteValidationError(w, err.Error(), nil)
	default:
		httpx.WriteInternalServerError(w, "failed to update jabatan")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

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
	hasRWID := req.RWID != nil && strings.TrimSpace(*req.RWID) != ""
	if !hasRWID && req.RW < 0 {
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
		Name: req.Name, RWID: req.RWID, RW: req.RW, RT: req.RT, Address: req.Address, HeadName: req.HeadName,
	})
	if err != nil {
		if errors.Is(err, ErrRWNotFound) {
			httpx.WriteNotFound(w, "RW not found")
			return
		}
		if errors.Is(err, ErrDuplicate) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "RT with this RW and RT code already exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create RT")
		return
	}
	ac := auth.GetAuthContext(r)
	actor := ac.UserID
	newValues, _ := json.Marshal(result)
	if err := finance.AuditLogInsert(ctx, tx, finance.AuditLog{RTID: result.ID, UserID: &actor, Action: "create", EntityType: "rt", EntityID: result.ID, NewValues: newValues, IPAddress: auditIP(r)}); err != nil {
		httpx.WriteInternalServerError(w, "failed to write audit log")
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

	var rwID *string
	if val := strings.TrimSpace(r.URL.Query().Get("rw_id")); val != "" {
		rwID = &val
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	items, err := h.svc.ListAllWithFilter(ctx, tx, isActive, search, rwID, offset, pageSize)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list RTs")
		return
	}

	total, err := h.svc.CountWithFilter(ctx, tx, isActive, search, rwID)
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

	old, err := h.svc.GetByID(ctx, tx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "RT not found")
		} else {
			httpx.WriteInternalServerError(w, "failed to load RT")
		}
		return
	}

	updateInput := &UpdateRTInput{
		Name:     req.Name,
		RWID:     req.RWID,
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
		if errors.Is(err, ErrRWNotFound) {
			httpx.WriteNotFound(w, "RW not found")
			return
		}
		if errors.Is(err, ErrDuplicate) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "RT with this RW and RT code already exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update RT")
		return
	}

	ac := auth.GetAuthContext(r)
	actor := ac.UserID
	oldJSON, _ := json.Marshal(old)
	newJSON, _ := json.Marshal(result)
	if err := finance.AuditLogInsert(ctx, tx, finance.AuditLog{RTID: id, UserID: &actor, Action: "update", EntityType: "rt", EntityID: id, OldValues: oldJSON, NewValues: newJSON, IPAddress: auditIP(r)}); err != nil {
		httpx.WriteInternalServerError(w, "failed to write audit log")
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

	old, err := h.svc.GetByID(ctx, tx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "RT not found")
		} else {
			httpx.WriteInternalServerError(w, "failed to load RT")
		}
		return
	}

	if err := h.svc.Deactivate(ctx, tx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "RT not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate RT")
		return
	}

	ac := auth.GetAuthContext(r)
	actor := ac.UserID
	oldJSON, _ := json.Marshal(old)
	newJSON, _ := json.Marshal(map[string]any{"is_active": false})
	if err := finance.AuditLogInsert(ctx, tx, finance.AuditLog{RTID: id, UserID: &actor, Action: "deactivate", EntityType: "rt", EntityID: id, OldValues: oldJSON, NewValues: newJSON, IPAddress: auditIP(r)}); err != nil {
		httpx.WriteInternalServerError(w, "failed to write audit log")
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate RT")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateRW handles POST /api/v1/rws — creates a new RW organization.
func (h *Handler) CreateRW(w http.ResponseWriter, r *http.Request) {
	var req CreateRWRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if req.RWNumber <= 0 {
		httpx.WriteValidationError(w, "rw_number must be > 0", nil)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "RW " + strconv.Itoa(req.RWNumber)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.p.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	result, err := h.svc.CreateRW(ctx, tx, CreateRWInput{
		RWNumber: req.RWNumber,
		Name:     strings.TrimSpace(req.Name),
		HeadName: req.HeadName,
		Address:  req.Address,
	})
	if err != nil {
		if errors.Is(err, ErrRWDuplicate) {
			httpx.WriteError(w, http.StatusConflict, "conflict", "RW with this number already exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create RW")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create RW")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// GetRWByID handles GET /api/v1/rws/{id} — returns a single RW.
func (h *Handler) GetRWByID(w http.ResponseWriter, r *http.Request) {
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

	item, err := h.svc.GetRWByID(ctx, tx, id)
	if err != nil {
		if errors.Is(err, ErrRWNotFound) {
			httpx.WriteNotFound(w, "RW not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load RW")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load RW")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// ListAllRWs handles GET /api/v1/rws — returns paginated list of RWs.
func (h *Handler) ListAllRWs(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 50
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

	items, err := h.svc.ListAllRWs(ctx, tx, isActive, search, offset, pageSize)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list RWs")
		return
	}

	total, err := h.svc.CountRWs(ctx, tx, isActive, search)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count RWs")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list RWs")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": items,
		"pagination": map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// UpdateRW handles PATCH /api/v1/rws/{id} — updates an RW.
func (h *Handler) UpdateRW(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	var req UpdateRWRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if req.RWNumber != nil && *req.RWNumber <= 0 {
		httpx.WriteValidationError(w, "rw_number must be > 0", nil)
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

	result, err := h.svc.UpdateRW(ctx, tx, id, &UpdateRWInput{
		RWNumber: req.RWNumber,
		Name:     req.Name,
		HeadName: req.HeadName,
		Address:  req.Address,
		IsActive: req.IsActive,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrRWNotFound):
			httpx.WriteNotFound(w, "RW not found")
		case errors.Is(err, ErrRWDuplicate), errors.Is(err, ErrDuplicate):
			httpx.WriteConflict(w, err.Error())
		case errors.Is(err, ErrRWHasActiveRTs):
			httpx.WriteConflict(w, err.Error())
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update RW")
		}
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update RW")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeactivateRW handles PATCH /api/v1/rws/{id}/deactivate — deactivates an RW.
func (h *Handler) DeactivateRW(w http.ResponseWriter, r *http.Request) {
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

	if err := h.svc.DeactivateRW(ctx, tx, id); err != nil {
		switch {
		case errors.Is(err, ErrRWNotFound):
			httpx.WriteNotFound(w, "RW not found")
		case errors.Is(err, ErrRWHasActiveRTs):
			httpx.WriteConflict(w, err.Error())
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate RW")
		}
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate RW")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
