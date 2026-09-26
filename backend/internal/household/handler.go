package household

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	if req.HouseholdID == "" {
		httpx.WriteValidationError(w, "household_id is required", nil)
		return
	}

	// Resolve target RT: tenant users use authenticated RT;
	// system super_admin without RT resolves it from the target household.
	rtID, ok := h.resolveRTID(w, r, func(ctx context.Context, tx *sql.Tx) (string, error) {
		return HouseholdGetRTByID(ctx, tx, req.HouseholdID)
	})
	if !ok {
		return
	}

	if req.FullName == "" {
		httpx.WriteValidationError(w, "full_name is required", nil)
		return
	}
	if req.HouseholdID == "" {
		httpx.WriteValidationError(w, "household_id is required", nil)
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
