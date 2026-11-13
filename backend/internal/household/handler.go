package household

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
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
		targetRT := ""
		if req.RequestedRTID != nil && strings.TrimSpace(*req.RequestedRTID) != "" {
			bodyRT := strings.TrimSpace(*req.RequestedRTID)
			if ac.RTID != "" && ac.RTID != bodyRT {
				httpx.WriteValidationError(w, "rt_id in body does not match X-RT-ID header", nil)
				return
			}
			targetRT = bodyRT
		} else if ac.RTID != "" {
			targetRT = ac.RTID
		} else {
			reqRT := strings.TrimSpace(r.URL.Query().Get("rt_id"))
			if reqRT != "" {
				targetRT = reqRT
			}
		}

		if targetRT == "" {
			httpx.WriteValidationError(w, "rt_id is required", nil)
			return
		}
		rtID := targetRT
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

	var parsedStart *time.Time
	if req.StartDate != nil && strings.TrimSpace(*req.StartDate) != "" {
		trimmedStart := strings.TrimSpace(*req.StartDate)
		ps, err := time.Parse("2006-01-02", trimmedStart)
		if err != nil {
			httpx.WriteValidationError(w, "start_date must be in YYYY-MM-DD format", nil)
			return
		}
		parsedStart = &ps
		req.StartDate = &trimmedStart
	}
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		trimmedEnd := strings.TrimSpace(*req.EndDate)
		pe, err := time.Parse("2006-01-02", trimmedEnd)
		if err != nil {
			httpx.WriteValidationError(w, "end_date must be in YYYY-MM-DD format", nil)
			return
		}
		if parsedStart != nil && !pe.After(*parsedStart) {
			httpx.WriteValidationError(w, "end_date must be after start_date", nil)
			return
		}
		req.EndDate = &trimmedEnd
	}

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
	json.NewEncoder(w).Encode(redactHouseholdNik(result, ac, ac.RTID))
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
		targetRT := ac.RTID
		if targetRT == "" {
			rtQueryPtr := parseStringQuery(r, "rt_id")
			if rtQueryPtr != nil && strings.TrimSpace(*rtQueryPtr) != "" {
				targetRT = strings.TrimSpace(*rtQueryPtr)
			}
		}

		if targetRT != "" {
			households, err := svc.ListHouseholds(ctx, tx, targetRT, isActive, search, offset, pageSize)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list households")
				return
			}

			total, err := svc.CountHouseholds(ctx, tx, targetRT, isActive, search)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count households")
				return
			}

			if err := tx.Commit(); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list households")
				return
			}

			writePaginated(w, http.StatusOK, redactHouseholdsNik(households, ac, ac.RTID), page, pageSize, total)
			return
		}

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

		writePaginated(w, http.StatusOK, redactHouseholdsNik(households, ac, ac.RTID), page, pageSize, total)
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

	writePaginated(w, http.StatusOK, redactHouseholdsNik(households, ac, ac.RTID), page, pageSize, total)
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
		targetRT := ac.RTID
		if targetRT == "" {
			rtQueryPtr := parseStringQuery(r, "rt_id")
			if rtQueryPtr != nil && strings.TrimSpace(*rtQueryPtr) != "" {
				targetRT = strings.TrimSpace(*rtQueryPtr)
			}
		}

		if targetRT != "" {
			household, err := svc.GetHouseholdByID(ctx, tx, id, targetRT)
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

			writeJSON(w, http.StatusOK, redactHouseholdNik(household, ac, ac.RTID))
			return
		}

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

		writeJSON(w, http.StatusOK, redactHouseholdNik(household, ac, ac.RTID))
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

	writeJSON(w, http.StatusOK, redactHouseholdNik(household, ac, ac.RTID))
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

	var parsedStart *time.Time
	if req.StartDate != nil && strings.TrimSpace(*req.StartDate) != "" {
		trimmedStart := strings.TrimSpace(*req.StartDate)
		ps, err := time.Parse("2006-01-02", trimmedStart)
		if err != nil {
			httpx.WriteValidationError(w, "start_date must be in YYYY-MM-DD format", nil)
			return
		}
		parsedStart = &ps
		req.StartDate = &trimmedStart
	}
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		trimmedEnd := strings.TrimSpace(*req.EndDate)
		pe, err := time.Parse("2006-01-02", trimmedEnd)
		if err != nil {
			httpx.WriteValidationError(w, "end_date must be in YYYY-MM-DD format", nil)
			return
		}
		if parsedStart != nil && !pe.After(*parsedStart) {
			httpx.WriteValidationError(w, "end_date must be after start_date", nil)
			return
		}
		req.EndDate = &trimmedEnd
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

	writeJSON(w, http.StatusOK, redactHouseholdNik(result, ac, ac.RTID))
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
// SUPER_ADMIN with explicit target rt_id (via X-RT-ID header, query param, or JSON body) or target household.
func (h *Handler) CreateResident(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	var req CreateResidentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}

	var rtID string
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		targetRT := ac.RTID
		if req.RequestedRTID != nil && strings.TrimSpace(*req.RequestedRTID) != "" {
			bodyRT := strings.TrimSpace(*req.RequestedRTID)
			if targetRT != "" && targetRT != bodyRT {
				httpx.WriteValidationError(w, "rt_id in body does not match X-RT-ID header", nil)
				return
			}
			targetRT = bodyRT
		} else if targetRT == "" {
			reqRT := strings.TrimSpace(r.URL.Query().Get("rt_id"))
			if reqRT != "" {
				targetRT = reqRT
			}
		}

		if targetRT == "" && req.HouseholdID != nil && strings.TrimSpace(*req.HouseholdID) != "" {
			ctxH, cancelH := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancelH()
			txH, errH := h.pool.BeginTx(ctxH)
			if errH == nil {
				targetRT, _ = HouseholdGetRTByID(ctxH, txH, strings.TrimSpace(*req.HouseholdID))
				_ = txH.Rollback()
			}
		}

		if targetRT == "" {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "tenant context required")
			return
		}

		// Verify target RT exists and is active
		ctxV, cancelV := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancelV()
		txV, errV := h.pool.BeginTx(ctxV)
		if errV != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
			return
		}
		defer txV.Rollback()

		rt, errRT := rtpkg.GetByID(ctxV, txV, targetRT)
		if errRT != nil {
			if errors.Is(errRT, rtpkg.ErrNotFound) {
				httpx.WriteNotFound(w, "target RT not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify target RT")
			return
		}
		if !rt.IsActive {
			httpx.WriteValidationError(w, "cannot create resident under an inactive RT", nil)
			return
		}
		_ = txV.Rollback()

		rtID = targetRT
	} else {
		if req.RequestedRTID != nil && strings.TrimSpace(*req.RequestedRTID) != "" {
			httpx.WriteValidationError(w, "rt_id is not allowed for tenant users", nil)
			return
		}
		if ac.RTID == "" {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "tenant context required")
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

	var parsedStart *time.Time
	if req.StartDate != nil && strings.TrimSpace(*req.StartDate) != "" {
		trimmedStart := strings.TrimSpace(*req.StartDate)
		ps, err := time.Parse("2006-01-02", trimmedStart)
		if err != nil {
			httpx.WriteValidationError(w, "start_date must be in YYYY-MM-DD format", nil)
			return
		}
		parsedStart = &ps
		req.StartDate = &trimmedStart
	}
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		trimmedEnd := strings.TrimSpace(*req.EndDate)
		pe, err := time.Parse("2006-01-02", trimmedEnd)
		if err != nil {
			httpx.WriteValidationError(w, "end_date must be in YYYY-MM-DD format", nil)
			return
		}
		if parsedStart != nil && !pe.After(*parsedStart) {
			httpx.WriteValidationError(w, "end_date must be after start_date", nil)
			return
		}
		req.EndDate = &trimmedEnd
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
	json.NewEncoder(w).Encode(redactResidentNik(resident, ac, ac.RTID))
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
		targetRT := ac.RTID
		if targetRT == "" {
			rtQueryPtr := parseStringQuery(r, "rt_id")
			if rtQueryPtr != nil && strings.TrimSpace(*rtQueryPtr) != "" {
				targetRT = strings.TrimSpace(*rtQueryPtr)
			}
		}

		if targetRT != "" {
			residents, err := svc.ListResidents(ctx, tx, targetRT, householdID, isActive, search, offset, pageSize)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list residents")
				return
			}

			total, err := svc.CountResidents(ctx, tx, targetRT, householdID, isActive)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count residents")
				return
			}

			if err := tx.Commit(); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list residents")
				return
			}

			writePaginated(w, http.StatusOK, redactResidentsNik(residents, ac, ac.RTID), page, pageSize, total)
			return
		}

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

		writePaginated(w, http.StatusOK, redactResidentsNik(residents, ac, ac.RTID), page, pageSize, total)
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

	writePaginated(w, http.StatusOK, redactResidentsNik(residents, ac, ac.RTID), page, pageSize, total)
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
		targetRT := ac.RTID
		if targetRT == "" {
			rtQueryPtr := parseStringQuery(r, "rt_id")
			if rtQueryPtr != nil && strings.TrimSpace(*rtQueryPtr) != "" {
				targetRT = strings.TrimSpace(*rtQueryPtr)
			}
		}

		if targetRT != "" {
			resident, err := svc.GetResidentByID(ctx, tx, id, targetRT)
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

			writeJSON(w, http.StatusOK, redactResidentNik(resident, ac, ac.RTID))
			return
		}

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

		writeJSON(w, http.StatusOK, redactResidentNik(resident, ac, ac.RTID))
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

	writeJSON(w, http.StatusOK, redactResidentNik(resident, ac, ac.RTID))
}

// UpdateResident handles PATCH /api/v1/residents/{id} — pengurus only.
func (h *Handler) UpdateResident(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
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

	var parsedStart *time.Time
	if req.StartDate != nil && strings.TrimSpace(*req.StartDate) != "" {
		trimmedStart := strings.TrimSpace(*req.StartDate)
		ps, err := time.Parse("2006-01-02", trimmedStart)
		if err != nil {
			httpx.WriteValidationError(w, "start_date must be in YYYY-MM-DD format", nil)
			return
		}
		parsedStart = &ps
		req.StartDate = &trimmedStart
	}
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		trimmedEnd := strings.TrimSpace(*req.EndDate)
		pe, err := time.Parse("2006-01-02", trimmedEnd)
		if err != nil {
			httpx.WriteValidationError(w, "end_date must be in YYYY-MM-DD format", nil)
			return
		}
		if parsedStart != nil && !pe.After(*parsedStart) {
			httpx.WriteValidationError(w, "end_date must be after start_date", nil)
			return
		}
		req.EndDate = &trimmedEnd
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

	writeJSON(w, http.StatusOK, redactResidentNik(result, ac, ac.RTID))
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
// Special Resident (keamanan, operasional, kebersihan)
// No household chain — user account + membership + household-less resident.
// ────────────────────────────────────────────────────────────────────────

// validSpecialJabatans is the allowed set of jabatan for special residents.
var validSpecialJabatans = map[string]bool{
	"keamanan":    true,
	"operasional": true,
	"kebersihan":  true,
}

// SpecialResidentCreateRequest is the body for POST /api/v1/residents/special.
//
// Password is the officer's initial login password (hashed with the same
// auth.Hash used by every other account). It is required: before it existed
// the handler silently generated a random password nobody knew, so newly
// created officers could never log in.
// IsActive is optional and defaults to true.
type SpecialResidentCreateRequest struct {
	RTID     string `json:"rt_id,omitempty"`
	Jabatan  string `json:"jabatan"`
	FullName string `json:"full_name"`
	Nik      string `json:"nik"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsActive *bool  `json:"is_active,omitempty"`
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
		httpx.WriteValidationError(w, "jabatan must be 'keamanan', 'operasional', or 'kebersihan'", nil)
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
	// Use the auth package's normalizer: it is the exact function the login
	// path applies, so the stored phone and the login lookup can never diverge.
	canonPhone, phoneErr := auth.NormalizePhone(req.Phone)
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
	var passwordHash string
	trimmedPassword := strings.TrimSpace(req.Password)
	if trimmedPassword != "" {
		if len(trimmedPassword) < auth.MinPasswordLength {
			httpx.WriteValidationError(w, fmt.Sprintf("password must be at least %d characters", auth.MinPasswordLength), nil)
			return
		}
		var hashErr error
		passwordHash, hashErr = auth.Hash(trimmedPassword)
		if hashErr != nil {
			httpx.WriteValidationError(w, hashErr.Error(), nil)
			return
		}
	} else {
		// When no password is provided, generate a cryptographically random initial password.
		// Plaintext is never stored, and credentials can be established via authorized reset flow.
		randBytes := make([]byte, 32)
		if _, err := crand.Read(randBytes); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to generate initial password")
			return
		}
		var hashErr error
		passwordHash, hashErr = auth.Hash(base64.StdEncoding.EncodeToString(randBytes))
		if hashErr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to hash initial password")
			return
		}
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

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

	// Never attach an existing account to a new RT or change its role/jabatan.
	// Email is global across tenants, so reusing it could grant access to an
	// existing warga account or move its resident record across RTs.
	existingUser, findErr := auth.UsersFindByEmail(ctx, tx, canonEmail)
	if findErr == nil && existingUser != nil {
		httpx.WriteConflict(w, "email is already registered")
		return
	}
	if !errors.Is(findErr, auth.ErrUserNotFound) {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check registered email")
		return
	}

	// Phone is a login identifier too, and the database has no unique index on
	// it, so reject duplicates explicitly (UsersFindByPhone also matches
	// legacy +62/62 variants and residents.phone, exactly like login does).
	existingByPhone, phoneFindErr := auth.UsersFindByPhone(ctx, tx, canonPhone)
	if phoneFindErr == nil && existingByPhone != nil {
		httpx.WriteConflict(w, "phone number is already registered")
		return
	}
	if !errors.Is(phoneFindErr, auth.ErrUserNotFound) {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check registered phone")
		return
	}

	userID, err := auth.CreateNewUser(ctx, tx, canonEmail, &canonPhone, passwordHash, strings.TrimSpace(req.FullName), "")
	if err != nil {
		if isPGUniqueViolation(err) {
			httpx.WriteConflict(w, "email is already registered")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create user")
		return
	}
	if !isActive {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET is_active = false, updated_at = now() WHERE id = $1`, userID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to set account status")
			return
		}
	}

	// 2. Create a least-privilege warga membership with the special jabatan.
	memQ := `
		INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, $4, $5)
	`
	if _, err := tx.ExecContext(ctx, memQ, userID, rtID, string(auth.RoleWarga), req.Jabatan, isActive); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create membership")
		return
	}

	// 3. Create resident with household_id = NULL.
	var resident Resident
	err = tx.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, user_id, full_name, phone, nik, email, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, rt_id, full_name, phone, nik, email, is_active, created_at, updated_at`,
		rtID, userID, req.FullName, canonPhone, strings.TrimSpace(req.Nik), canonEmail, isActive,
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
	json.NewEncoder(w).Encode(redactResidentNik(&resident, ac, ac.RTID))
}

// SpecialResidentUpdateRequest is the body for PATCH /api/v1/residents/special/:id.
type SpecialResidentUpdateRequest struct {
	FullName *string `json:"full_name,omitempty"`
	Nik      *string `json:"nik,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	Jabatan  *string `json:"jabatan,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
	// Password is optional: absent/empty keeps the current password, a value
	// replaces it (min auth.MinPasswordLength chars, hashed with auth.Hash).
	Password *string `json:"password,omitempty"`
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

	// An empty password means "do not change it".
	var newPasswordHash *string
	if req.Password != nil && strings.TrimSpace(*req.Password) == "" {
		req.Password = nil
	}
	if req.Password != nil {
		if len(*req.Password) < auth.MinPasswordLength {
			httpx.WriteValidationError(w, fmt.Sprintf("password must be at least %d characters", auth.MinPasswordLength), nil)
			return
		}
		hashed, hashErr := auth.Hash(*req.Password)
		if hashErr != nil {
			httpx.WriteValidationError(w, hashErr.Error(), nil)
			return
		}
		newPasswordHash = &hashed
	}

	// At least one field must be provided
	if req.FullName == nil && req.Nik == nil && req.Phone == nil && req.Email == nil && req.Jabatan == nil && req.IsActive == nil && req.Password == nil {
		httpx.WriteValidationError(w, "at least one field must be provided", nil)
		return
	}

	// Validate jabatan if provided
	if req.Jabatan != nil {
		if !validSpecialJabatans[*req.Jabatan] {
			httpx.WriteValidationError(w, "jabatan must be 'keamanan', 'operasional', or 'kebersihan'", nil)
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

	// 1. Find the resident and verify it's a special resident (no household).
	// Inactive officers must stay editable so they can be re-activated.
	var resident Resident
	var userID sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, rt_id, full_name, phone, nik, email, user_id, is_active, created_at, updated_at
		 FROM residents WHERE id = $1`,
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

	// Only genuine special officers may be edited here. Without this guard the
	// password/status fields below could be applied to any household-less
	// resident (e.g. a pengurus record), which is not what this endpoint is for.
	if !userID.Valid {
		httpx.WriteConflict(w, "not a special resident — no linked account")
		return
	}
	var membershipJabatan sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2 LIMIT 1`,
		userID.String, rtID,
	).Scan(&membershipJabatan); err != nil && !errors.Is(err, sql.ErrNoRows) {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify officer membership")
		return
	} else if errors.Is(err, sql.ErrNoRows) || !membershipJabatan.Valid || !validSpecialJabatans[membershipJabatan.String] {
		httpx.WriteConflict(w, "not a special resident — membership is not an officer jabatan")
		return
	}

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
		req.FullName = &trimmed
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
		// Same normalizer as login (auth.NormalizePhone) — see CreateSpecialResident.
		normalized, normErr := auth.NormalizePhone(trimmed)
		if normErr != nil {
			httpx.WriteValidationError(w, normErr.Error(), nil)
			return
		}
		if other, findErr := auth.UsersFindByPhone(ctx, tx, normalized); findErr == nil && other != nil && other.ID != userID.String {
			httpx.WriteConflict(w, "phone number is already registered")
			return
		} else if findErr != nil && !errors.Is(findErr, auth.ErrUserNotFound) {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check registered phone")
			return
		}
		req.Phone = &normalized
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
		req.Nik = &trimmed
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
		normalized := NormalizeEmail(trimmed)
		req.Email = &normalized
		fields = append(fields, updateField{"email", normalized})
		args = append(args, normalized)
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
			` AND rt_id = $` + fmt.Sprint(len(fields)+2)
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

	// 3. Update membership jabatan if provided. The membership is_active flag
	// always follows the account status chosen in the form, so a deactivated
	// officer is blocked at login (membership + user) and a re-activated one
	// can log in again.
	if req.Jabatan != nil {
		memUpdateQ := `
			UPDATE user_rt_memberships SET jabatan = $1, updated_at = now()
			WHERE user_id = $2 AND rt_id = $3
		`
		if _, err := tx.ExecContext(ctx, memUpdateQ, *req.Jabatan, userID.String, rtID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update jabatan")
			return
		}
		hasChanges = true
	}
	if req.IsActive != nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE user_rt_memberships SET is_active = $1, updated_at = now() WHERE user_id = $2 AND rt_id = $3`,
			*req.IsActive, userID.String, rtID,
		); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update membership status")
			return
		}
		hasChanges = true
	}

	if userID.Valid && (req.FullName != nil || req.Phone != nil || req.Email != nil || req.IsActive != nil || newPasswordHash != nil) {
		var fullName, phone, email, passwordHash, isActive any
		if req.FullName != nil {
			fullName = *req.FullName
		}
		if req.Phone != nil {
			phone = *req.Phone
		}
		if req.Email != nil {
			email = *req.Email
		}
		if newPasswordHash != nil {
			passwordHash = *newPasswordHash
		}
		if req.IsActive != nil {
			isActive = *req.IsActive
		}
		result, updateErr := tx.ExecContext(ctx, `
			UPDATE users
			SET full_name = COALESCE($2, full_name),
			    phone = COALESCE($3, phone),
			    email = COALESCE($4, email),
			    password_hash = COALESCE($5, password_hash),
			    is_active = COALESCE($6::boolean, is_active),
			    updated_at = now()
			WHERE id = $1
		`, userID.String, fullName, phone, email, passwordHash, isActive)
		if updateErr != nil {
			if isPGUniqueViolation(updateErr) {
				httpx.WriteConflict(w, "account profile conflicts with an existing user")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to synchronize resident account")
			return
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify resident account update")
			return
		}
		if rows == 0 {
			httpx.WriteNotFound(w, "resident account not found")
			return
		}
		hasChanges = true

		// A new password or a deactivation must cut off sessions that were
		// issued with the old credentials/status.
		if newPasswordHash != nil || (req.IsActive != nil && !*req.IsActive) {
			if err := auth.RevokeRefreshTokensByUserID(ctx, tx, userID.String); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to revoke sessions")
				return
			}
		}
	}

	if !hasChanges {
		httpx.WriteNotFound(w, "no changes to apply")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit")
		return
	}

	if req.FullName != nil {
		resident.FullName = *req.FullName
	}
	if req.Phone != nil {
		phone := *req.Phone
		resident.Phone = &phone
	}
	if req.Nik != nil {
		nik := *req.Nik
		resident.Nik = &nik
	}
	if req.Email != nil {
		email := *req.Email
		resident.Email = &email
	}
	if req.IsActive != nil {
		resident.IsActive = *req.IsActive
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(redactResidentNik(&resident, ac, ac.RTID))
}

// isResetPasswordOfficer checks whether the auth context corresponds to an authorized officer or Super Admin.
func isResetPasswordOfficer(ac *auth.AuthContext) bool {
	if ac == nil {
		return false
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true
	}
	j := strings.ToLower(strings.TrimSpace(string(ac.Jabatan)))
	isJabatanOfficer := j == string(auth.JabatanKetua) ||
		j == "ketua_rt" || j == "ketua rt" ||
		j == string(auth.JabatanWakilKetua) ||
		j == string(auth.JabatanSekretaris) ||
		j == string(auth.JabatanBendahara)
	isRoleOfficer := ac.TenantRole == auth.RolePengurus || ac.TenantRole == auth.RoleBendahara
	return isRoleOfficer || isJabatanOfficer
}

// GenerateResetPasswordCode handles POST /api/v1/residents/{id}/reset-password-code
// Authorized for Pengurus RT and Super Admin. Generates a 4-digit numeric code valid for 24h.
func (h *Handler) GenerateResetPasswordCode(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	if !isResetPasswordOfficer(ac) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "only pengurus or super admin can generate reset codes")
		return
	}

	isSuperAdmin := ac.SystemRole == auth.SystemRoleSuperAdmin

	residentID := chi.URLParam(r, "id")
	if residentID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "resident id is required")
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

	// Query resident
	var resID, resRTID, resFullName string
	var resUserID, resPhone, resEmail sql.NullString
	var query string
	var args []any
	if isSuperAdmin {
		query = `SELECT id, rt_id, user_id, full_name, phone, email FROM residents WHERE id = $1 AND is_active = true`
		args = []any{residentID}
	} else {
		rtID, ok := requireRTID(w, r)
		if !ok {
			return
		}
		query = `SELECT id, rt_id, user_id, full_name, phone, email FROM residents WHERE id = $1 AND rt_id = $2 AND is_active = true`
		args = []any{residentID, rtID}
	}

	err = tx.QueryRowContext(ctx, query, args...).Scan(&resID, &resRTID, &resUserID, &resFullName, &resPhone, &resEmail)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteNotFound(w, "resident not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to lookup resident")
		return
	}

	// If resident has no user_id, create or link user account
	var targetUserID string
	if resUserID.Valid && resUserID.String != "" {
		targetUserID = resUserID.String
	} else {
		// Generate an initial random password to link user account
		dummyPass := fmt.Sprintf("InitPass#%d%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000)
		var pEmail, pPhone *string
		if resEmail.Valid && resEmail.String != "" {
			pEmail = &resEmail.String
		}
		if resPhone.Valid && resPhone.String != "" {
			pPhone = &resPhone.String
		}
		if err := syncResidentUserAccount(ctx, tx, resRTID, resID, resFullName, pPhone, pEmail, &dummyPass); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to initialize resident user account")
			return
		}
		// Fetch newly linked user_id
		err = tx.QueryRowContext(ctx, `SELECT user_id FROM residents WHERE id = $1`, resID).Scan(&targetUserID)
		if err != nil || targetUserID == "" {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to load linked user id")
			return
		}
	}

	// Invalidate any previous unused codes for this user
	_, err = tx.ExecContext(ctx, `
		UPDATE resident_password_reset_codes
		SET used_at = now()
		WHERE user_id = $1 AND used_at IS NULL
	`, targetUserID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to invalidate previous codes")
		return
	}

	// Generate 4-digit code (0000 - 9999) using crypto/rand
	num, err := crand.Int(crand.Reader, big.NewInt(10000))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to generate secure code")
		return
	}
	code := fmt.Sprintf("%04d", num.Int64())

	// Hash code with Argon2id
	codeHash, err := auth.HashCode(code)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to hash reset code")
		return
	}

	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO resident_password_reset_codes (
			user_id, resident_id, rt_id, code_hash, expires_at, created_by
		) VALUES (
			$1, $2, $3, $4, now() + interval '24 hours', $5
		) RETURNING expires_at
	`, targetUserID, resID, resRTID, codeHash, ac.UserID).Scan(&expiresAt)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to save reset code")
		return
	}

	// Determine user identifier (prefer email, else phone, else email on user record)
	identifier := ""
	if resEmail.Valid && resEmail.String != "" {
		identifier = resEmail.String
	} else if resPhone.Valid && resPhone.String != "" {
		identifier = resPhone.String
	} else {
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(email, phone, '') FROM users WHERE id = $1`, targetUserID).Scan(&identifier)
	}

	// Record audit log
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (rt_id, user_id, action, entity_type, entity_id, new_values)
		VALUES ($1, $2, 'generate_password_reset_code', 'resident', $3, $4)
	`, resRTID, ac.UserID, resID, fmt.Sprintf(`{"resident_id": "%s", "expires_at": "%s"}`, resID, expiresAt.Format(time.RFC3339)))

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit transaction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":          code,
		"expires_at":    expiresAt.Format(time.RFC3339),
		"resident_id":   resID,
		"resident_name": resFullName,
		"identifier":    identifier,
	})
}

// canViewNik returns true only if the requester is Ketua RT in the active context.
// Non-ketua pengurus (e.g. Sekretaris, Bendahara, Keamanan), regular warga, and RW officers
// MUST NOT see unmasked NIK.
//
// Ketua RT in production has role=warga with jabatan=ketua (see migration 031),
// so the check must NOT require TenantRole==pengurus — it must rely on jabatan
// plus RT isolation: the user's active RT (ac.RTID) must match the requested rtID.
//
// If rtID is provided, the user's active RT must equal rtID and jabatan must be ketua.
// If rtID is empty, it falls back to checking the jabatan claim only.
//
// IMPORTANT: Super Admin must NOT see NIK — strict Ketua RT-only access, regardless of rtID provided.
func canViewNik(ac *auth.AuthContext, rtID string) bool {
	if ac == nil {
		return false
	}
	// Super Admin never sees NIK — strict Ketua RT-only access
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return false
	}
	j := strings.ToLower(strings.TrimSpace(string(ac.Jabatan)))
	isKetua := j == "ketua" || j == "ketua_rt" || j == "ketua rt"
	if !isKetua {
		return false
	}
	// If rtID is provided, enforce RT isolation: ketua only in their own active RT.
	if rtID != "" {
		if ac.RTID == "" || ac.RTID != rtID {
			return false
		}
	}
	return true
}

// canViewContact returns true only if the requester is Ketua RT in the active context.
// Non-ketua pengurus (e.g. Sekretaris, Bendahara, Keamanan), regular warga, and RW officers
// MUST NOT see unmasked contact data (phone, email).
//
// Like canViewNik, this must not require TenantRole==pengurus because the real
// Ketua RT has role=warga with jabatan=ketua. RT isolation is enforced by
// requiring ac.RTID == rtID when rtID is provided.
//
// If rtID is empty, it falls back to checking the jabatan claim only.
//
// IMPORTANT: Super Admin must NOT see contact data — strict Ketua RT-only access,
// regardless of rtID provided.
func canViewContact(ac *auth.AuthContext, rtID string) bool {
	if ac == nil {
		return false
	}
	// Super Admin never sees contact data — strict Ketua RT-only access
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return false
	}
	j := strings.ToLower(strings.TrimSpace(string(ac.Jabatan)))
	isKetua := j == "ketua" || j == "ketua_rt" || j == "ketua rt"
	if !isKetua {
		return false
	}
	if rtID != "" {
		if ac.RTID == "" || ac.RTID != rtID {
			return false
		}
	}
	return true
}

func redactResidentsNik(residents []*Resident, ac *auth.AuthContext, rtID string) []*Resident {
	if canViewNik(ac, rtID) {
		return residents
	}
	for _, r := range residents {
		if r != nil {
			r.Nik = nil
		}
	}
	return residents
}

func redactResidentNik(resident *Resident, ac *auth.AuthContext, rtID string) *Resident {
	if resident == nil || canViewNik(ac, rtID) {
		return resident
	}
	resident.Nik = nil
	return resident
}

func redactHouseholdsNik(households []*Household, ac *auth.AuthContext, rtID string) []*Household {
	if canViewNik(ac, rtID) {
		return households
	}
	for _, h := range households {
		if h != nil {
			h.Nik = nil
			if h.HeadResident != nil {
				h.HeadResident.Nik = nil
			}
		}
	}
	return households
}

func redactHouseholdNik(household *Household, ac *auth.AuthContext, rtID string) *Household {
	if household == nil || canViewNik(ac, rtID) {
		return household
	}
	household.Nik = nil
	if household.HeadResident != nil {
		household.HeadResident.Nik = nil
	}
	return household
}
