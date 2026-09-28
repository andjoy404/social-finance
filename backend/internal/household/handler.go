package household

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	rtpkg "social-finance/internal/rt"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// Handler handles HTTP requests for households and residents.
type Handler struct {
	pool *database.Pool
}

// NewHandler creates a new handler.
func NewHandler(pool *database.Pool) *Handler {
	return &Handler{pool: pool}
}

// parsePagination extracts and validates page/page_size from query parameters.
func parsePagination(r *http.Request) (int, int) {
	page := defaultPage
	pageSize := defaultPageSize

	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 1 {
			page = n
		}
	}

	ps := r.URL.Query().Get("page_size")
	if ps == "" {
		ps = r.URL.Query().Get("per_page")
	}
	if ps == "" {
		ps = r.URL.Query().Get("limit")
	}
	if ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 {
			if n > maxPageSize {
				n = maxPageSize
			}
			pageSize = n
		}
	}

	return page, pageSize
}

// parseBoolQuery parses an optional bool query param, returning nil if absent.
func parseBoolQuery(r *http.Request, key string) *bool {
	val := r.URL.Query().Get(key)
	if val == "" {
		return nil
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return nil
	}
	return &b
}

// parseStringQuery returns the query param value or nil if absent/empty.
func parseStringQuery(r *http.Request, key string) *string {
	val := r.URL.Query().Get(key)
	if val == "" {
		return nil
	}
	return &val
}

// getRTID extracts the tenant ID from auth context.
func getRTID(r *http.Request) (string, bool) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		return "", false
	}
	return ac.RTID, true
}

// resolveRTID resolves the target RT for existing-resource mutations.
// For tenant users: returns authenticated RT from JWT claims.
// For system super_admin without RTID: resolves from the given resource lookup.
// For unauthenticated or insufficiently-privileged users: writes error and returns ("", false).
func (h *Handler) resolveRTID(w http.ResponseWriter, r *http.Request,
	getResourceRT func(context.Context, *sql.Tx) (string, error),
) (string, bool) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return "", false
	}

	// Tenant user with authenticated RT
	if ac.RTID != "" {
		return ac.RTID, true
	}

	// System super admin without tenant RT: resolve from resource
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		tx, err := h.pool.BeginTx(ctx)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
			return "", false
		}
		defer tx.Rollback()

		rtID, err := getResourceRT(ctx, tx)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteNotFound(w, "resource not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve target RT")
			}
			return "", false
		}
		return rtID, true
	}

	httpx.WriteError(w, http.StatusForbidden, "forbidden", "tenant context required")
	return "", false
}

func requireRTID(w http.ResponseWriter, r *http.Request) (string, bool) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return "", false
	}
	if ac.RTID == "" {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "tenant context required")
		return "", false
	}
	return ac.RTID, true
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// writePaginated writes a paginated JSON response.
func writePaginated(w http.ResponseWriter, code int, data any, page, pageSize, total int) {
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	resp := map[string]any{
		"data": data,
		"pagination": map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	}
	writeJSON(w, code, resp)
}

// ─── Household Handlers ─────────────────────────────────────────────────────

// CreateHousehold handles POST /api/v1/households — pengurus only, SUPER_ADMIN with explicit target rt_id.
func (h *Handler) CreateHousehold(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)

	var req CreateHouseholdInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	if ac != nil && ac.SystemRole == auth.SystemRoleSuperAdmin {
		if req.RequestedRTID == nil {
			httpx.WriteValidationError(w, "rt_id is required", nil)
			return
		}
		trimmedRTID := strings.TrimSpace(*req.RequestedRTID)
		if trimmedRTID == "" {
			httpx.WriteValidationError(w, "rt_id cannot be empty", nil)
			return
		}
		rtID := trimmedRTID
		rt, err := rtpkg.GetByID(ctx, tx, rtID)
		if err != nil {
			if errors.Is(err, rtpkg.ErrNotFound) {
				httpx.WriteNotFound(w, "target RT not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify target RT")
			return
		}
		if !rt.IsActive {
			httpx.WriteValidationError(w, "cannot create household under an inactive RT", nil)
			return
		}
		req.RTID = rtID
	} else {
		if req.RequestedRTID != nil {
			httpx.WriteValidationError(w, "rt_id is not allowed for tenant users", nil)
			return
		}
		rtID, ok := getRTID(r)
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}
		req.RTID = rtID
	}

	if err := req.TrimAndValidateHouseNumber(); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	if req.HeadName == "" {
		if req.FullName != nil && strings.TrimSpace(*req.FullName) != "" {
			req.HeadName = strings.TrimSpace(*req.FullName)
		} else {
			httpx.WriteValidationError(w, "head_name is required", nil)
			return
		}
	}
	if req.OccupancyStatus == "" {
		httpx.WriteValidationError(w, "occupancy_status is required", nil)
		return
	}
	if err := req.OccupancyStatus.Validate(); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if req.Nik == nil || strings.TrimSpace(*req.Nik) == "" {
		httpx.WriteValidationError(w, "nik is required", nil)
		return
	}
	trimmedNik := strings.TrimSpace(*req.Nik)
	if err := ValidateNik(trimmedNik); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	req.Nik = &trimmedNik

	if req.Phone == nil || strings.TrimSpace(*req.Phone) == "" {
		httpx.WriteValidationError(w, "phone is required", nil)
		return
	}
	canonPhone, err := NormalizePhone(*req.Phone)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	req.Phone = &canonPhone

	if req.Email == nil || strings.TrimSpace(*req.Email) == "" {
		httpx.WriteValidationError(w, "email is required", nil)
		return
	}
	if err := ValidateEmail(*req.Email); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	canonEmail := NormalizeEmail(*req.Email)
	req.Email = &canonEmail

	if req.IsActive == nil {
		trueVal := true
		req.IsActive = &trueVal
	}

	result, err := svc.CreateHousehold(ctx, tx, req)
	if err != nil {
		if errors.Is(err, ErrDuplicateHouseNumber) {
			httpx.WriteConflict(w, "duplicate house_number within RT")
			return
		}
		if errors.Is(err, ErrOccupiedHouse) {
			httpx.WriteConflict(w, "physical house is already occupied")
			return
		}
		if errors.Is(err, ErrDuplicateNik) {
			httpx.WriteConflict(w, "duplicate NIK within RT")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create household")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create household")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// ListHouseholds handles GET /api/v1/households — all tenant roles, system read for SUPER_ADMIN.
func (h *Handler) ListHouseholds(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)

	page, pageSize := parsePagination(r)
	isActive := parseBoolQuery(r, "is_active")
	search := parseStringQuery(r, "search")

	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	if ac != nil && ac.SystemRole == auth.SystemRoleSuperAdmin {
		households, err := svc.ListAllHouseholds(ctx, tx, isActive, search, offset, pageSize)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list households")
			return
		}

		total, err := svc.CountAllHouseholds(ctx, tx, isActive, search)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count households")
			return
		}

		if err := tx.Commit(); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list households")
			return
		}

		writePaginated(w, http.StatusOK, households, page, pageSize, total)
		return
	}

	rtID, ok := getRTID(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	households, err := svc.ListHouseholds(ctx, tx, rtID, isActive, search, offset, pageSize)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list households")
		return
	}

	total, err := svc.CountHouseholds(ctx, tx, rtID, isActive, search)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count households")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list households")
		return
	}

	writePaginated(w, http.StatusOK, households, page, pageSize, total)
}

// GetHousehold handles GET /api/v1/households/{id} — all tenant roles, system read for SUPER_ADMIN.
func (h *Handler) GetHousehold(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	if ac != nil && ac.SystemRole == auth.SystemRoleSuperAdmin {
		household, err := svc.GetAllHouseholdByID(ctx, tx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteNotFound(w, "household not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load household")
			return
		}

		if err := tx.Commit(); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load household")
			return
		}

		writeJSON(w, http.StatusOK, household)
		return
	}

	rtID, ok := getRTID(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	household, err := svc.GetHouseholdByID(ctx, tx, id, rtID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "household not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load household")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load household")
		return
	}

	writeJSON(w, http.StatusOK, household)
}

// UpdateHousehold handles PATCH /api/v1/households/{id} — pengurus only,
// system-level SUPER_ADMIN derives target RT from existing household.
func (h *Handler) UpdateHousehold(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the household itself,
	// allowing updates to inactive households.
	rtID, ok := h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
		return HouseholdGetRTByIDAnyStatus(ctx, tx, id)
	})
	if !ok {
		return
	}

	var req UpdateHouseholdInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if req.HouseNumber != nil {
		trimmed := strings.TrimSpace(*req.HouseNumber)
		if trimmed == "" {
			httpx.WriteValidationError(w, "house_number cannot be empty", nil)
			return
		}
		*req.HouseNumber = trimmed
	}
	if req.OccupancyStatus != nil && (*req.OccupancyStatus).Validate() != nil {
		httpx.WriteValidationError(w, "occupancy_status must be OWNER or TENANT", nil)
		return
	}
	if req.HeadName != nil {
		trimmed := strings.TrimSpace(*req.HeadName)
		if trimmed == "" {
			httpx.WriteValidationError(w, "head_name cannot be empty", nil)
			return
		}
		*req.HeadName = trimmed
	}
	if req.FullName != nil {
		trimmed := strings.TrimSpace(*req.FullName)
		if trimmed == "" {
			httpx.WriteValidationError(w, "full_name cannot be empty", nil)
			return
		}
		*req.FullName = trimmed
	}
	if req.Nik != nil {
		trimmed := strings.TrimSpace(*req.Nik)
		if trimmed == "" {
			httpx.WriteValidationError(w, "nik cannot be empty", nil)
			return
		}
		if err := ValidateNik(trimmed); err != nil {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		req.Nik = &trimmed
	}
	if req.Phone != nil {
		trimmed := strings.TrimSpace(*req.Phone)
		if trimmed == "" {
			httpx.WriteValidationError(w, "phone cannot be empty", nil)
			return
		}
		canonPhone, err := NormalizePhone(trimmed)
		if err != nil {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		req.Phone = &canonPhone
	}
	if req.Email != nil {
		trimmed := strings.TrimSpace(*req.Email)
		if trimmed == "" {
			httpx.WriteValidationError(w, "email cannot be empty", nil)
			return
		}
		if err := ValidateEmail(trimmed); err != nil {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		canonEmail := NormalizeEmail(trimmed)
		req.Email = &canonEmail
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	result, err := svc.UpdateHousehold(ctx, tx, id, rtID, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "household not found or no fields provided")
			return
		}
		if errors.Is(err, ErrDuplicateHouseNumber) {
			httpx.WriteConflict(w, "duplicate house_number within RT")
			return
		}
		if errors.Is(err, ErrDuplicateNik) {
			httpx.WriteConflict(w, "duplicate NIK within RT")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update household")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update household")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// MoveHousehold handles POST /api/v1/households/{id}/move — pengurus only,
// system-level SUPER_ADMIN derives target RT from existing household.
func (h *Handler) MoveHousehold(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the household itself.
	rtID, ok := h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
		return HouseholdGetRTByID(ctx, tx, id)
	})
	if !ok {
		return
	}

	var req MoveHouseholdInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	trimmed := strings.TrimSpace(req.HouseNumber)
	if trimmed == "" {
		httpx.WriteValidationError(w, "house_number is required", nil)
		return
	}
	req.HouseNumber = trimmed

	if strings.TrimSpace(req.StartDate) == "" {
		httpx.WriteValidationError(w, "start_date is required", nil)
		return
	}
	if _, err := time.Parse("2006-01-02", req.StartDate); err != nil {
		httpx.WriteValidationError(w, "start_date must be in YYYY-MM-DD format", nil)
		return
	}
	if req.OccupancyStatus != nil && (*req.OccupancyStatus).Validate() != nil {
		httpx.WriteValidationError(w, "invalid occupancy_status", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)
	res, err := svc.MoveHousehold(ctx, tx, id, rtID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoCurrentOccupancy) {
			httpx.WriteNotFound(w, "household not found or has no current occupancy")
			return
		}
		if errors.Is(err, ErrOccupiedHouse) {
			httpx.WriteConflict(w, "target house is already occupied")
			return
		}
		if errors.Is(err, ErrDuplicateHouseNumber) {
			httpx.WriteConflict(w, "duplicate house_number within RT")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit household move")
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// DeactivateHousehold handles DELETE /api/v1/households/{id} — pengurus only,
// system-level SUPER_ADMIN derives target RT from existing household.
func (h *Handler) DeactivateHousehold(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the household itself.
	rtID, ok := h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
		return HouseholdGetRTByID(ctx, tx, id)
	})
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	if err := svc.DeactivateHousehold(ctx, tx, id, rtID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "household not found or already deactivated")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate household")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate household")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Resident Handlers ──────────────────────────────────────────────────────

// CreateResident handles POST /api/v1/residents — pengurus only,
// system-level SUPER_ADMIN derives target RT from target household.
func (h *Handler) CreateResident(w http.ResponseWriter, r *http.Request) {
	var req CreateResidentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the target household (if provided).
	// For special residents (no household_id), RT comes from auth context only.
	var rtID string
	var ok bool
	if req.HouseholdID != nil && *req.HouseholdID != "" {
		rtID, ok = h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
			return HouseholdGetRTByID(ctx, tx, *req.HouseholdID)
		})
		if !ok {
			return
		}
	} else {
		// Special resident: RT must come from auth context (not from client).
		ac := auth.GetAuthContext(r)
		if ac == nil || ac.RTID == "" {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
			return
		}
		rtID = ac.RTID
	}

	if req.FullName == "" {
		httpx.WriteValidationError(w, "full_name is required", nil)
		return
	}
	if req.Nik == nil || strings.TrimSpace(*req.Nik) == "" {
		httpx.WriteValidationError(w, "nik is required", nil)
		return
	}
	trimmedNik := strings.TrimSpace(*req.Nik)
	if err := ValidateNik(trimmedNik); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	req.Nik = &trimmedNik

	if req.Phone == nil || strings.TrimSpace(*req.Phone) == "" {
		httpx.WriteValidationError(w, "phone is required", nil)
		return
	}
	canonPhone, phoneErr := NormalizePhone(*req.Phone)
	if phoneErr != nil {
		httpx.WriteValidationError(w, phoneErr.Error(), nil)
		return
	}
	req.Phone = &canonPhone

	if req.Email == nil || strings.TrimSpace(*req.Email) == "" {
		httpx.WriteValidationError(w, "email is required", nil)
		return
	}
	if err := ValidateEmail(*req.Email); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	canonEmail := NormalizeEmail(*req.Email)
	req.Email = &canonEmail

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)
	resident, err := svc.CreateResident(ctx, tx, req, rtID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "household not found in this RT")
			return
		}
		if errors.Is(err, ErrDuplicateNik) {
			httpx.WriteConflict(w, "duplicate NIK within RT")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create resident")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create resident")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resident)
}

// ListResidents handles GET /api/v1/residents — all tenant roles, system read for SUPER_ADMIN.
func (h *Handler) ListResidents(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)

	page, pageSize := parsePagination(r)
	householdID := parseStringQuery(r, "household_id")
	isActive := parseBoolQuery(r, "is_active")
	search := parseStringQuery(r, "search")

	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	if ac != nil && ac.SystemRole == auth.SystemRoleSuperAdmin {
		residents, err := svc.ListAllResidents(ctx, tx, householdID, isActive, search, offset, pageSize)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list residents")
			return
		}

		total, err := svc.CountAllResidents(ctx, tx, householdID, isActive)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count residents")
			return
		}

		if err := tx.Commit(); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list residents")
			return
		}

		writePaginated(w, http.StatusOK, residents, page, pageSize, total)
		return
	}

	rtID, ok := requireRTID(w, r)
	if !ok {
		return
	}

	residents, err := svc.ListResidents(ctx, tx, rtID, householdID, isActive, search, offset, pageSize)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list residents")
		return
	}

	total, err := svc.CountResidents(ctx, tx, rtID, householdID, isActive)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count residents")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list residents")
		return
	}

	writePaginated(w, http.StatusOK, residents, page, pageSize, total)
}

// GetResident handles GET /api/v1/residents/{id} — all tenant roles & super_admin.
func (h *Handler) GetResident(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	if ac != nil && ac.SystemRole == auth.SystemRoleSuperAdmin {
		resident, err := svc.GetAllResidentByID(ctx, tx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteNotFound(w, "resident not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load resident")
			return
		}

		if err := tx.Commit(); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load resident")
			return
		}

		writeJSON(w, http.StatusOK, resident)
		return
	}

	rtID, ok := requireRTID(w, r)
	if !ok {
		return
	}

	resident, err := svc.GetResidentByID(ctx, tx, id, rtID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "resident not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load resident")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load resident")
		return
	}

	writeJSON(w, http.StatusOK, resident)
}

// UpdateResident handles PATCH /api/v1/residents/{id} — pengurus only.
func (h *Handler) UpdateResident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the resident itself.
	rtID, ok := h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
		return ResidentGetRTByID(ctx, tx, id)
	})
	if !ok {
		return
	}

	var req UpdateResidentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	if req.FullName != nil {
		trimmed := strings.TrimSpace(*req.FullName)
		if trimmed == "" {
			httpx.WriteValidationError(w, "full_name cannot be empty", nil)
			return
		}
		*req.FullName = trimmed
	}
	if req.Phone != nil {
		trimmed := strings.TrimSpace(*req.Phone)
		if trimmed == "" {
			httpx.WriteValidationError(w, "phone cannot be empty", nil)
			return
		}
		normalizedPhone, phoneNormErr := NormalizePhone(trimmed)
		if phoneNormErr != nil {
			httpx.WriteValidationError(w, phoneNormErr.Error(), nil)
			return
		}
		req.Phone = &normalizedPhone
	}
	if req.Nik != nil {
		trimmed := strings.TrimSpace(*req.Nik)
		if trimmed == "" {
			httpx.WriteValidationError(w, "nik cannot be empty", nil)
			return
		}
		if err := ValidateNik(trimmed); err != nil {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		req.Nik = &trimmed
	}
	if req.Email != nil {
		trimmed := strings.TrimSpace(*req.Email)
		if trimmed == "" {
			httpx.WriteValidationError(w, "email cannot be empty", nil)
			return
		}
		if err := ValidateEmail(trimmed); err != nil {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		canonEmail := NormalizeEmail(trimmed)
		req.Email = &canonEmail
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	result, err := svc.UpdateResident(ctx, tx, id, rtID, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "resident not found or no fields provided")
			return
		}
		if errors.Is(err, ErrDuplicateNik) {
			httpx.WriteConflict(w, "duplicate NIK within RT")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update resident")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update resident")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// MoveResident handles POST /api/v1/residents/{id}/move — pengurus only.
func (h *Handler) MoveResident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the resident itself.
	rtID, ok := h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
		return ResidentGetRTByID(ctx, tx, id)
	})
	if !ok {
		return
	}

	var req MoveResidentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if strings.TrimSpace(req.DestinationHouseholdID) == "" {
		httpx.WriteValidationError(w, "destination_household_id is required", nil)
		return
	}
	if strings.TrimSpace(req.StartDate) == "" {
		httpx.WriteValidationError(w, "start_date is required", nil)
		return
	}
	if _, err := time.Parse("2006-01-02", req.StartDate); err != nil {
		httpx.WriteValidationError(w, "start_date must be in YYYY-MM-DD format", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)
	res, err := svc.MoveResident(ctx, tx, id, rtID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoCurrentOccupancy) {
			httpx.WriteNotFound(w, "resident or destination household not found in this RT")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit resident move")
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// DeactivateResident handles DELETE /api/v1/residents/{id} — pengurus only,
// system-level SUPER_ADMIN derives target RT from existing resident.
func (h *Handler) DeactivateResident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the resident itself.
	rtID, ok := h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
		return ResidentGetRTByID(ctx, tx, id)
	})
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	svc := NewHouseholdManager(h.pool)

	if err := svc.DeactivateResident(ctx, tx, id, rtID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "resident not found or already deactivated")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate resident")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate resident")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ────────────────────────────────────────────────────────────────────────
// Special Resident (keamanan, kebersihan_pembangunan)
// No household chain — user account + membership + household-less resident.
// ────────────────────────────────────────────────────────────────────────

// validSpecialJabatans is the allowed set of jabatan for special residents.
var validSpecialJabatans = map[string]bool{
	"keamanan":                    true,
	"kebersihan_pembangunan": true,
}

// SpecialResidentCreateRequest is the body for POST /api/v1/residents/special.
type SpecialResidentCreateRequest struct {
	RTID     string `json:"rt_id,omitempty"`
	Jabatan  string `json:"jabatan"`
	FullName string `json:"full_name"`
	Nik      string `json:"nik"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

// CreateSpecialResident handles POST /api/v1/residents/special.
// Creates: user → membership (pengurus + jabatan) → resident (household_id=NULL).
func (h *Handler) CreateSpecialResident(w http.ResponseWriter, r *http.Request) {
	var req SpecialResidentCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	if req.Jabatan == "" {
		httpx.WriteValidationError(w, "jabatan is required", nil)
		return
	}
	if !validSpecialJabatans[req.Jabatan] {
		httpx.WriteValidationError(w, "jabatan must be 'keamanan' or 'kebersihan_pembangunan'", nil)
		return
	}
	if req.FullName == "" {
		httpx.WriteValidationError(w, "full_name is required", nil)
		return
	}
	if req.Nik == "" {
		httpx.WriteValidationError(w, "nik is required", nil)
		return
	}
	trimmedNik := strings.TrimSpace(req.Nik)
	if err := ValidateNik(trimmedNik); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	if req.Phone == "" {
		httpx.WriteValidationError(w, "phone is required", nil)
		return
	}
	canonPhone, phoneErr := NormalizePhone(req.Phone)
	if phoneErr != nil {
		httpx.WriteValidationError(w, phoneErr.Error(), nil)
		return
	}
	if req.Email == "" {
		httpx.WriteValidationError(w, "email is required", nil)
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}
	canonEmail := NormalizeEmail(req.Email)

	// Resolve RTID: non-super-admin uses their authenticated RT.
	// Super-admin without RTID must provide rt_id in the body.
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.RTID == "" {
		if ac == nil || ac.SystemRole != auth.SystemRoleSuperAdmin {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
			return
		}
		// System-level super admin: resolve from body rt_id
		if req.RTID == "" {
			httpx.WriteValidationError(w, "rt_id is required for super admin", nil)
			return
		}
		// Validate the RT exists
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		tx, err := h.pool.BeginTx(ctx)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
			return
		}
		defer tx.Rollback()
		var rtName string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM rts WHERE id = $1 AND is_active = true`, req.RTID).Scan(&rtName); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				httpx.WriteNotFound(w, "RT not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify RT")
			return
		}
		if err := tx.Commit(); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not verify RT")
			return
		}
		ac.RTID = req.RTID
	} else {
		// Tenant users: enforce their own RT
		req.RTID = ac.RTID
	}
	rtID := ac.RTID

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	// 1. Create user (upsert by normalized email)
	passwordHash, err := auth.Hash("SpecialResident123!")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to hash password")
		return
	}
	userID, err := auth.CreateNewUser(ctx, tx, canonEmail, &canonPhone, passwordHash, strings.TrimSpace(req.FullName), "")
	if err != nil {
		// User may already exist — query by normalized email to get the existing ID
		existingUser, userErr := auth.UsersFindByEmail(ctx, tx, canonEmail)
		if userErr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create user")
			return
		}
		userID = existingUser.ID
	}

	// 2. Upsert membership (pengurus + jabatan)
	memQ := `
		INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, $4, true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO UPDATE SET
			role = EXCLUDED.role,
			jabatan = EXCLUDED.jabatan,
			updated_at = now()
	`
	if _, err := tx.ExecContext(ctx, memQ, userID, rtID, string(auth.RolePengurus), req.Jabatan); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create membership")
		return
	}

	// 3. Create resident with household_id = NULL
	var resident Resident
	err = tx.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, user_id, full_name, phone, nik, email, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, true)
		 RETURNING id, rt_id, full_name, phone, nik, email, is_active, created_at, updated_at`,
		rtID, userID, req.FullName, canonPhone, strings.TrimSpace(req.Nik), canonEmail,
	).Scan(
		&resident.ID, &resident.RTID, &resident.FullName, &resident.Phone,
		&resident.Nik, &resident.Email, &resident.IsActive, &resident.CreatedAt, &resident.UpdatedAt,
	)
	if err != nil {
		if isPGNikViolation(err) {
			httpx.WriteConflict(w, "duplicate NIK within RT")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create resident")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resident)
}

// SpecialResidentUpdateRequest is the body for PATCH /api/v1/residents/special/:id.
type SpecialResidentUpdateRequest struct {
	FullName *string `json:"full_name,omitempty"`
	Nik      *string `json:"nik,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	Jabatan  *string `json:"jabatan,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// UpdateSpecialResident handles PATCH /api/v1/residents/special/:id.
// Updates resident personal fields + membership jabatan in one transaction.
func (h *Handler) UpdateSpecialResident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	var req SpecialResidentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	// At least one field must be provided
	if req.FullName == nil && req.Nik == nil && req.Phone == nil && req.Email == nil && req.Jabatan == nil && req.IsActive == nil {
		httpx.WriteValidationError(w, "at least one field must be provided", nil)
		return
	}

	// Validate jabatan if provided
	if req.Jabatan != nil {
		if !validSpecialJabatans[*req.Jabatan] {
			httpx.WriteValidationError(w, "jabatan must be 'keamanan' or 'kebersihan_pembangunan'", nil)
			return
		}
	}

	ac := auth.GetAuthContext(r)
	if ac == nil || (ac.RTID == "" && ac.SystemRole != auth.SystemRoleSuperAdmin) {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	// 1. Find the resident and verify it's a special resident (no household)
	var resident Resident
	var userID sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, rt_id, full_name, phone, nik, email, user_id, is_active, created_at, updated_at
		 FROM residents WHERE id = $1 AND is_active = true`,
		id,
	).Scan(
		&resident.ID, &resident.RTID, &resident.FullName, &resident.Phone,
		&resident.Nik, &resident.Email, &userID, &resident.IsActive,
		&resident.CreatedAt, &resident.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteNotFound(w, "special resident not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to find resident")
		return
	}

	// Verify the resident has no active residency period (special residents have none).
	var activePeriodCount int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM residency_periods
		 WHERE resident_id = $1 AND end_date IS NULL`, id,
	).Scan(&activePeriodCount)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify resident type")
		return
	}
	if activePeriodCount > 0 {
		httpx.WriteConflict(w, "not a special resident — has active residency period")
		return
	}

	// Verify RT access: tenant users can only update their own RT
	if ac.RTID != "" && resident.RTID != ac.RTID {
		httpx.WriteNotFound(w, "special resident not found")
		return
	}
	rtID := resident.RTID

	// 2. Update resident fields
	hasChanges := false
	var fields []updateField
	var args []interface{}
	argIdx := 1

	if req.FullName != nil {
		trimmed := strings.TrimSpace(*req.FullName)
		if trimmed == "" {
			httpx.WriteValidationError(w, "full_name cannot be empty", nil)
			return
		}
		fields = append(fields, updateField{"full_name", trimmed})
		args = append(args, trimmed)
		argIdx++
	}
	if req.Phone != nil {
		trimmed := strings.TrimSpace(*req.Phone)
		if trimmed == "" {
			httpx.WriteValidationError(w, "phone cannot be empty", nil)
			return
		}
		normalized, normErr := NormalizePhone(trimmed)
		if normErr != nil {
			httpx.WriteValidationError(w, normErr.Error(), nil)
			return
		}
		fields = append(fields, updateField{"phone", normalized})
		args = append(args, normalized)
		argIdx++
	}
	if req.Nik != nil {
		trimmed := strings.TrimSpace(*req.Nik)
		if trimmed == "" {
			httpx.WriteValidationError(w, "nik cannot be empty", nil)
			return
		}
		if err := ValidateNik(trimmed); err != nil {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		fields = append(fields, updateField{"nik", trimmed})
		args = append(args, trimmed)
		argIdx++
	}
	if req.Email != nil {
		trimmed := strings.TrimSpace(*req.Email)
		if trimmed == "" {
			httpx.WriteValidationError(w, "email cannot be empty", nil)
			return
		}
		if err := ValidateEmail(trimmed); err != nil {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		fields = append(fields, updateField{"email", NormalizeEmail(trimmed)})
		args = append(args, NormalizeEmail(trimmed))
		argIdx++
	}
	if req.IsActive != nil {
		fields = append(fields, updateField{"is_active", *req.IsActive})
		args = append(args, *req.IsActive)
		argIdx++
	}

	if len(fields) > 0 {
		query := `UPDATE residents SET ` + setClause(fields, 0) +
			`, updated_at = now() WHERE id = $` + fmt.Sprint(len(fields)+1) +
			` AND rt_id = $` + fmt.Sprint(len(fields)+2) + ` AND is_active = true`
		args = append(args, id, rtID)
		result, execErr := tx.ExecContext(ctx, query, args...)
		if execErr != nil {
			if isPGNikViolation(execErr) {
				httpx.WriteConflict(w, "duplicate NIK within RT")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update resident")
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			httpx.WriteNotFound(w, "special resident not found")
			return
		}
		hasChanges = true
	}

	// 3. Update membership jabatan if provided
	if req.Jabatan != nil {
		memUpdateQ := `
			UPDATE user_rt_memberships SET jabatan = $1, updated_at = now()
			WHERE user_id = $2 AND rt_id = $3 AND is_active = true
		`
		if _, err := tx.ExecContext(ctx, memUpdateQ, *req.Jabatan, userID.String, rtID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update jabatan")
			return
		}
		hasChanges = true
	}

	if !hasChanges {
		httpx.WriteNotFound(w, "no changes to apply")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resident)
}
