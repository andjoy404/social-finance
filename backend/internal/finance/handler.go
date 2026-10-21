package finance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	"social-finance/internal/storage"
)

var (
	periodRegex = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	dateRegex   = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`)
)

type Handler struct {
	pool    *database.Pool
	svc     *Service
	storage storage.Service
}

func NewHandler(pool *database.Pool, svc *Service, opts ...storage.Service) *Handler {
	var store storage.Service
	if len(opts) > 0 && opts[0] != nil {
		store = opts[0]
	} else if svc != nil && svc.Storage() != nil {
		store = svc.Storage()
	} else {
		store = storage.NewMockStorage()
	}
	return &Handler{
		pool:    pool,
		svc:     svc,
		storage: store,
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

// requireTenantContext returns true if the user has a tenant membership.
// When false, it writes a 403 error response and returns false — the caller
// must abort handling.  This guards financial write endpoints from receiving
// an empty RTID that would cause "invalid input syntax for type uuid: \"\""
// in PostgreSQL.
func requireTenantContext(ac *auth.AuthContext, w http.ResponseWriter) bool {
	if ac.RTID == "" {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "User harus memiliki membership RT untuk melakukan operasi keuangan")
		return false
	}
	return true
}

// --- Categories Handlers ---

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var isActive *bool
	if a := r.URL.Query().Get("is_active"); a != "" {
		b := a == "true" || a == "1"
		isActive = &b
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	// System-only SUPER_ADMIN: query across all RTs
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		list, err := CategoryListAll(r.Context(), tx, isActive)
		if err != nil {
			httpx.WriteInternalServerError(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
		return
	}

	list, err := CategoryList(r.Context(), tx, ac.RTID, isActive)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	var in CreateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	cat, err := h.svc.CreateCategory(r.Context(), tx, ac.RTID, ac.UserID, in)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}
	writeJSON(w, http.StatusCreated, cat)
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	var in UpdateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	cat, err := h.svc.UpdateCategory(r.Context(), tx, id, ac.RTID, ac.UserID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "category not found")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

func (h *Handler) DeactivateCategory(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	if err := h.svc.DeactivateCategory(r.Context(), tx, id, ac.RTID, ac.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "category not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Dues Handlers ---

func (h *Handler) ListDues(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var isActive *bool
	if a := r.URL.Query().Get("is_active"); a != "" {
		b := a == "true" || a == "1"
		isActive = &b
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	// System-only SUPER_ADMIN: query across all RTs
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		list, err := DueListAll(r.Context(), tx, isActive)
		if err != nil {
			httpx.WriteInternalServerError(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
		return
	}

	list, err := DueList(r.Context(), tx, ac.RTID, isActive)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) CreateDue(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	var in CreateDueInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	due, err := h.svc.CreateDue(r.Context(), tx, ac.RTID, ac.UserID, in)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}
	writeJSON(w, http.StatusCreated, due)
}

func (h *Handler) UpdateDue(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	var in UpdateDueInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	due, err := h.svc.UpdateDue(r.Context(), tx, id, ac.RTID, ac.UserID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "due not found")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}
	writeJSON(w, http.StatusOK, due)
}

func (h *Handler) DeactivateDue(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	if err := h.svc.DeactivateDue(r.Context(), tx, id, ac.RTID, ac.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "due not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// hasStaffFinancePermission determines whether the authenticated user has staff-level
// authorization for a specific finance permission (e.g. "finance.read", "finance.update"),
// as opposed to an ordinary citizen whose access is restricted to their own household.
//
// Access is granted if:
//  1. System-level super_admin.
//  2. The user has an assigned jabatan (such as ketua, bendahara, sekretaris) that
//     possesses the specified permission in the position_permissions table.
//  3. The user has tenant role pengurus or bendahara without a conflicting jabatan.
func (h *Handler) hasStaffFinancePermission(ctx context.Context, tx *sql.Tx, ac *auth.AuthContext, permission string) (bool, error) {
	if ac == nil {
		return false, nil
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true, nil
	}
	if ac.RTID == "" {
		return false, nil
	}

	if tx == nil && h.pool != nil {
		t, err := h.pool.BeginTx(ctx)
		if err != nil {
			return false, err
		}
		defer t.Rollback()
		tx = t
	}

	// Users with a jabatan: evaluate against position_permissions via PermissionResolver
	if ac.Jabatan != "" {
		pr := auth.NewPermissionResolver(h.pool)
		return pr.HasPermission(ctx, tx, ac, permission)
	}

	// Role-based users without jabatan: pengurus and bendahara roles have staff finance permissions
	if ac.TenantRole == auth.RolePengurus || ac.TenantRole == auth.RoleBendahara {
		return true, nil
	}

	return false, nil
}

// hasStaffFinanceRead determines whether the authenticated user has staff-level
// authorization to read all RT financial data (e.g. List Iuran Warga).
func (h *Handler) hasStaffFinanceRead(ctx context.Context, tx *sql.Tx, ac *auth.AuthContext) (bool, error) {
	return h.hasStaffFinancePermission(ctx, tx, ac, "finance.read")
}

// --- Bills Handlers ---

func (h *Handler) ListBills(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	// System-only SUPER_ADMIN: query across all RTs with optional rt_id filter
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		// Check if ?rt_id= query param is present → use tenant-scoped query
		rtQuery := r.URL.Query().Get("rt_id")

		var dueID *string
		if d := r.URL.Query().Get("due_id"); d != "" {
			dueID = &d
		}
		var status *string
		if s := r.URL.Query().Get("status"); s != "" {
			upper := strings.ToUpper(strings.TrimSpace(s))
			status = &upper
		}
		var paymentStatus *string
		if ps := r.URL.Query().Get("payment_status"); ps != "" {
			upper := strings.ToUpper(strings.TrimSpace(ps))
			paymentStatus = &upper
		}
		var period *string
		if p := r.URL.Query().Get("period"); p != "" {
			period = &p
		}
		var search *string
		if s := r.URL.Query().Get("search"); s != "" {
			search = &s
		} else if q := r.URL.Query().Get("q"); q != "" {
			search = &q
		}
		var dueName *string
		if dn := r.URL.Query().Get("due_name"); dn != "" {
			dueName = &dn
		}

		tx, err := h.pool.BeginTx(r.Context())
		if err != nil {
			httpx.WriteInternalServerError(w, "transaction error")
			return
		}
		defer tx.Rollback()

		var occupancyID *string
		if o := r.URL.Query().Get("household_occupancy_id"); o != "" {
			occupancyID = &o
		}

		page, pageSize := parsePagination(r)
		offset := (page - 1) * pageSize

		var total int
		var bills []*Bill

		if rtQuery != "" {
			// Tenant-scoped query for specific RT (cross-tenant admin filtering)
			total, err = BillCount(r.Context(), tx, rtQuery, occupancyID, dueID, status, paymentStatus, period, search, dueName)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
			bills, err = BillList(r.Context(), tx, rtQuery, occupancyID, dueID, status, paymentStatus, period, search, dueName, offset, pageSize)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
		} else {
			// Cross-tenant query (admin viewing all RTs)
			total, err = BillCountAll(r.Context(), tx, occupancyID, dueID, status, paymentStatus, period, search, dueName)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
			bills, err = BillListAll(r.Context(), tx, occupancyID, dueID, status, paymentStatus, period, search, dueName, offset, pageSize)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
		}

		totalPages := 0
		if total > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": bills,
			"pagination": map[string]any{
				"page":        page,
				"per_page":    pageSize,
				"total":       total,
				"total_pages": totalPages,
			},
		})
		return
	}

	// Tenant user path (unchanged)
	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	isStaff, err := h.hasStaffFinanceRead(r.Context(), tx, ac)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to check permissions")
		return
	}

	var occupancyID *string
	// If not staff (ordinary citizen) or explicit ?my_bills=true, restrict strictly to user's own active household occupancy!
	if !isStaff || r.URL.Query().Get("my_bills") == "true" {
		occID, err := GetOccupancyIDForUser(r.Context(), tx, ac.RTID, ac.UserID)
		if err != nil || occID == "" {
			// No occupancy tied to warga -> empty collection
			writeJSON(w, http.StatusOK, map[string]any{
				"data": []*Bill{},
				"pagination": map[string]any{
					"page": 1, "per_page": 20, "total": 0, "total_pages": 0,
				},
			})
			return
		}
		occupancyID = &occID
	} else if o := r.URL.Query().Get("household_occupancy_id"); o != "" {
		occupancyID = &o
	}

	var dueID *string
	if d := r.URL.Query().Get("due_id"); d != "" {
		dueID = &d
	}
	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}
	var paymentStatus *string
	if ps := r.URL.Query().Get("payment_status"); ps != "" {
		upper := strings.ToUpper(strings.TrimSpace(ps))
		paymentStatus = &upper
	}
	var period *string
	if p := r.URL.Query().Get("period"); p != "" {
		period = &p
	}
	var search *string
	if s := r.URL.Query().Get("search"); s != "" {
		search = &s
	} else if q := r.URL.Query().Get("q"); q != "" {
		search = &q
	}
	var dueName *string
	if dn := r.URL.Query().Get("due_name"); dn != "" {
		dueName = &dn
	}

	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	total, err := BillCount(r.Context(), tx, ac.RTID, occupancyID, dueID, status, paymentStatus, period, search, dueName)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	bills, err := BillList(r.Context(), tx, ac.RTID, occupancyID, dueID, status, paymentStatus, period, search, dueName, offset, pageSize)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	_ = tx.Commit()

	writeJSON(w, http.StatusOK, map[string]any{
		"data": bills,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (h *Handler) GetBill(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	// System-only SUPER_ADMIN: query across all RTs
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		bill, err := BillGetByIDAll(r.Context(), tx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteNotFound(w, "bill not found")
				return
			}
			httpx.WriteInternalServerError(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, bill)
		return
	}

	bill, err := BillGetByID(r.Context(), tx, id, ac.RTID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "bill not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	isStaff, err := h.hasStaffFinanceRead(r.Context(), tx, ac)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to check permissions")
		return
	}

	if !isStaff {
		occID, _ := GetOccupancyIDForUser(r.Context(), tx, ac.RTID, ac.UserID)
		if occID != bill.HouseholdOccupancyID {
			httpx.WriteNotFound(w, "bill not found")
			return
		}
	}

	writeJSON(w, http.StatusOK, bill)
}

func (h *Handler) GenerateBills(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	handled, rec, idem := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer idem.Finish()

	targetW := w
	if rec != nil {
		targetW = rec
	}

	var in GenerateBillsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(targetW, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(targetW, "transaction error")
		return
	}
	defer tx.Rollback()

	bills, err := h.svc.GenerateBills(r.Context(), tx, ac.RTID, ac.UserID, in)
	if err != nil {
		httpx.WriteValidationError(targetW, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("GenerateBills commit failed", "error", err, "rt_id", ac.RTID, "due_id", in.DueID, "period", in.Period)
		httpx.WriteInternalServerError(targetW, "commit error")
		return
	}

	if h.svc.Notifier() != nil {
		for _, b := range bills {
			h.svc.Notifier().NotifyBillCreated(r.Context(), ac.RTID, b.ID, nil)
		}
	}

	writeJSON(targetW, http.StatusCreated, map[string]any{
		"generated_count": len(bills),
		"bills":           bills,
	})
}

func (h *Handler) CreateBill(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	var in CreateBillInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	bill, err := h.svc.CreateBill(r.Context(), tx, ac.RTID, ac.UserID, in)
	if err != nil {
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}

	if h.svc.Notifier() != nil && bill != nil {
		h.svc.Notifier().NotifyBillCreated(r.Context(), ac.RTID, bill.ID, nil)
	}

	writeJSON(w, http.StatusCreated, bill)
}

// --- Payments Handlers ---

func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	// System-only SUPER_ADMIN: query across all RTs with optional rt_id filter
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		// Check if ?rt_id= query param is present → use tenant-scoped query
		rtQuery := r.URL.Query().Get("rt_id")

		var billID *string
		if b := r.URL.Query().Get("bill_id"); b != "" {
			billID = &b
		}
		var dueID *string
		if d := r.URL.Query().Get("due_id"); d != "" {
			dueID = &d
		}
		var status *string
		if s := r.URL.Query().Get("status"); s != "" {
			upper := strings.ToUpper(strings.TrimSpace(s))
			status = &upper
		}

		tx, err := h.pool.BeginTx(r.Context())
		if err != nil {
			httpx.WriteInternalServerError(w, "transaction error")
			return
		}
		defer tx.Rollback()

		page, pageSize := parsePagination(r)
		offset := (page - 1) * pageSize

		var total int
		var payments []*Payment

		if rtQuery != "" {
			// Tenant-scoped query for specific RT (cross-tenant admin filtering)
			total, err = PaymentCount(r.Context(), tx, rtQuery, billID, dueID, status, nil, nil)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
			payments, err = PaymentList(r.Context(), tx, rtQuery, billID, dueID, status, nil, nil, offset, pageSize)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
		} else {
			// Cross-tenant query (admin viewing all RTs)
			total, err = PaymentCountAll(r.Context(), tx, billID, dueID, status)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
			payments, err = PaymentListAll(r.Context(), tx, billID, dueID, status, offset, pageSize)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
		}

		totalPages := 0
		if total > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": payments,
			"pagination": map[string]any{
				"page":        page,
				"per_page":    pageSize,
				"total":       total,
				"total_pages": totalPages,
			},
		})
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	var billID *string
	if b := r.URL.Query().Get("bill_id"); b != "" {
		billID = &b
	}
	var dueID *string
	if d := r.URL.Query().Get("due_id"); d != "" {
		dueID = &d
	}
	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}
	isStaff, err := h.hasStaffFinanceRead(r.Context(), tx, ac)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to check permissions")
		return
	}

	var submittedBy, occupancyID *string
	if !isStaff {
		submittedBy = &ac.UserID
		occID, err := GetOccupancyIDForUser(r.Context(), tx, ac.RTID, ac.UserID)
		if err != nil {
			httpx.WriteInternalServerError(w, "error verifying warga occupancy")
			return
		}
		if occID != "" {
			occupancyID = &occID
		}
	}

	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	total, err := PaymentCount(r.Context(), tx, ac.RTID, billID, dueID, status, submittedBy, occupancyID)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	payments, err := PaymentList(r.Context(), tx, ac.RTID, billID, dueID, status, submittedBy, occupancyID, offset, pageSize)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": payments,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	// System-only SUPER_ADMIN: query across all RTs
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		p, err := PaymentGetByIDAll(r.Context(), tx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteNotFound(w, "payment not found")
				return
			}
			httpx.WriteInternalServerError(w, err.Error())
			return
		}
		if p == nil {
			httpx.WriteNotFound(w, "payment not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
		return
	}

	isStaff, err := h.hasStaffFinanceRead(r.Context(), tx, ac)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to check permissions")
		return
	}

	var p *Payment
	if !isStaff {
		var occupancyID *string
		occID, occErr := GetOccupancyIDForUser(r.Context(), tx, ac.RTID, ac.UserID)
		if occErr != nil {
			httpx.WriteInternalServerError(w, "error verifying warga occupancy")
			return
		}
		if occID != "" {
			occupancyID = &occID
		}
		p, err = PaymentGetByIDForUser(r.Context(), tx, id, ac.RTID, ac.UserID, occupancyID)
	} else {
		p, err = PaymentGetByID(r.Context(), tx, id, ac.RTID)
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "payment not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}
	if p == nil {
		httpx.WriteNotFound(w, "payment not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	handled, rec, idem := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer idem.Finish()

	targetW := w
	if rec != nil {
		targetW = rec
	}

	var in CreatePaymentInput
	var uploadedKey string

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			httpx.WriteValidationError(targetW, "invalid multipart form or file too large (max 5MB)", nil)
			return
		}
		if billID := strings.TrimSpace(r.FormValue("bill_id")); billID != "" {
			in.BillID = &billID
		}
		if dueID := strings.TrimSpace(r.FormValue("due_id")); dueID != "" {
			in.DueID = &dueID
		}
		in.Amount = r.FormValue("amount")
		in.Method = PaymentMethod(r.FormValue("method"))
		if notes := r.FormValue("notes"); notes != "" {
			in.Notes = &notes
		}
		if err := in.Validate(); err != nil {
			httpx.WriteValidationError(targetW, err.Error(), nil)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			file, header, err = r.FormFile("proof")
		}
		if err == nil {
			defer file.Close()
			if header.Size > 0 {
				proofTarget := ""
				if in.BillID != nil {
					proofTarget = *in.BillID
				} else if in.DueID != nil {
					proofTarget = "donation-" + *in.DueID
				}
				meta, err := h.storage.UploadProof(r.Context(), ac.RTID, proofTarget, header.Filename, file, header.Size)
				if err != nil {
					httpx.WriteValidationError(targetW, err.Error(), nil)
					return
				}
				uploadedKey = meta.Key
				in.ProofPath = &meta.Key
				in.ProofOriginalName = &meta.OriginalName
				in.ProofMimeType = &meta.ContentType
				in.ProofSizeBytes = &meta.Size
				in.ProofUploadedBy = &ac.UserID
			}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			httpx.WriteValidationError(targetW, "invalid request body", nil)
			return
		}
	}

	tx, err := idem.BeginTx(r.Context(), h.pool)
	if err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteProof(r.Context(), uploadedKey)
		}
		httpx.WriteInternalServerError(targetW, "transaction error")
		return
	}
	defer tx.Rollback()

	p, err := h.svc.CreatePayment(r.Context(), tx, ac.RTID, ac.UserID, string(ac.TenantRole), in)
	if err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteProof(r.Context(), uploadedKey)
		}
		if errors.Is(err, ErrBillAlreadyPaid) {
			httpx.WriteConflict(targetW, "bill is already paid")
			return
		}
		if errors.Is(err, ErrBillCancelled) {
			httpx.WriteConflict(targetW, "bill is cancelled")
			return
		}
		httpx.WriteValidationError(targetW, err.Error(), nil)
		return
	}

	if idem != nil {
		writeJSON(targetW, http.StatusCreated, p)
		if err := idem.Save(tx); err != nil {
			rec.replace(0, "")
			httpx.WriteInternalServerError(targetW, "could not persist idempotency response")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteProof(r.Context(), uploadedKey)
		}
		if idem != nil {
			rec.replace(0, "")
		}
		httpx.WriteInternalServerError(targetW, "commit error")
		return
	}

	if h.svc.Notifier() != nil && p != nil {
		billID := ""
		if p.BillID != nil {
			billID = *p.BillID
		}
		h.svc.Notifier().NotifyPaymentSubmitted(r.Context(), ac.RTID, billID, p.ID)
	}

	if idem == nil {
		writeJSON(targetW, http.StatusCreated, p)
	}
}

func (h *Handler) CreateMultiPeriodPayment(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	handled, rec, idem := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer idem.Finish()

	targetW := w
	if rec != nil {
		targetW = rec
	}

	var in CreateMultiPeriodPaymentInput
	var uploadedKey string

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			httpx.WriteValidationError(targetW, "invalid multipart form or file too large (max 5MB)", nil)
			return
		}

		billIDsStr := r.FormValue("bill_ids")
		if billIDsStr != "" {
			_ = json.Unmarshal([]byte(billIDsStr), &in.BillIDs)
		}

		allocationsStr := r.FormValue("allocations")
		if allocationsStr != "" {
			_ = json.Unmarshal([]byte(allocationsStr), &in.Allocations)
		}

		in.Method = PaymentMethod(r.FormValue("method"))
		if notes := r.FormValue("notes"); notes != "" {
			in.Notes = &notes
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			file, header, err = r.FormFile("proof")
		}
		if err == nil {
			defer file.Close()
			if header.Size > 0 {
				meta, err := h.storage.UploadProof(r.Context(), ac.RTID, "multi", header.Filename, file, header.Size)
				if err != nil {
					httpx.WriteValidationError(targetW, err.Error(), nil)
					return
				}
				uploadedKey = meta.Key
				in.ProofPath = &meta.Key
				in.ProofOriginalName = &meta.OriginalName
				in.ProofMimeType = &meta.ContentType
				in.ProofSizeBytes = &meta.Size
				in.ProofUploadedBy = &ac.UserID
			}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			httpx.WriteValidationError(targetW, "invalid request body", nil)
			return
		}
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteProof(r.Context(), uploadedKey)
		}
		httpx.WriteInternalServerError(targetW, "transaction error")
		return
	}
	defer tx.Rollback()

	p, err := h.svc.CreateMultiPeriodPayment(r.Context(), tx, ac.RTID, ac.UserID, string(ac.TenantRole), in)
	if err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteProof(r.Context(), uploadedKey)
		}
		httpx.WriteValidationError(targetW, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteProof(r.Context(), uploadedKey)
		}
		httpx.WriteInternalServerError(targetW, "commit error")
		return
	}

	if h.svc.Notifier() != nil && p != nil && p.Payment != nil {
		firstBillID := ""
		if len(in.BillIDs) > 0 {
			firstBillID = in.BillIDs[0]
		}
		h.svc.Notifier().NotifyPaymentSubmitted(r.Context(), ac.RTID, firstBillID, p.Payment.ID)
	}

	writeJSON(targetW, http.StatusCreated, p)
}

func (h *Handler) UploadPaymentProof(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
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

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	canStaffUpdate, err := h.hasStaffFinancePermission(r.Context(), tx, ac, "finance.update")
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to check permissions")
		return
	}

	var payment *Payment
	if !canStaffUpdate {
		var occupancyID *string
		occID, occErr := GetOccupancyIDForUser(r.Context(), tx, ac.RTID, ac.UserID)
		if occErr != nil {
			httpx.WriteInternalServerError(w, "error verifying warga occupancy")
			return
		}
		if occID != "" {
			occupancyID = &occID
		}
		payment, err = PaymentGetByIDForUser(r.Context(), tx, paymentID, ac.RTID, ac.UserID, occupancyID)
	} else {
		payment, err = PaymentGetByID(r.Context(), tx, paymentID, ac.RTID)
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "payment not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}
	if payment == nil {
		httpx.WriteNotFound(w, "payment not found")
		return
	}

	if payment.Status == PaymentStatusRejected {
		httpx.WriteValidationError(w, "cannot upload proof for a rejected payment", nil)
		return
	}

	meta, err := h.storage.UploadProof(r.Context(), ac.RTID, paymentID, header.Filename, file, header.Size)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidFormat) || errors.Is(err, storage.ErrFileTooLarge) || errors.Is(err, storage.ErrEmptyFile) {
			httpx.WriteValidationError(w, err.Error(), nil)
			return
		}
		httpx.WriteInternalServerError(w, "failed to upload proof to storage")
		return
	}

	callerRole := string(ac.TenantRole)
	if canStaffUpdate {
		callerRole = string(auth.RolePengurus)
	}
	updatedPayment, err := h.svc.AttachPaymentProof(r.Context(), tx, ac.RTID, ac.UserID, callerRole, paymentID, meta)
	if err != nil {
		_ = h.storage.DeleteProof(r.Context(), meta.Key)
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		_ = h.storage.DeleteProof(r.Context(), meta.Key)
		httpx.WriteInternalServerError(w, "commit error")
		return
	}

	writeJSON(w, http.StatusOK, updatedPayment)
}

func (h *Handler) GetPaymentProof(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	var p *Payment
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		p, err = PaymentGetByIDAll(r.Context(), tx, paymentID)
	} else {
		if !requireTenantContext(ac, w) {
			return
		}
		isStaff, err := h.hasStaffFinanceRead(r.Context(), tx, ac)
		if err != nil {
			httpx.WriteInternalServerError(w, "failed to check permissions")
			return
		}

		if !isStaff {
			var occupancyID *string
			occID, lookupErr := GetOccupancyIDForUser(r.Context(), tx, ac.RTID, ac.UserID)
			if lookupErr != nil {
				httpx.WriteInternalServerError(w, "error verifying warga occupancy")
				return
			}
			if occID != "" {
				occupancyID = &occID
			}
			p, err = PaymentGetByIDForUser(r.Context(), tx, paymentID, ac.RTID, ac.UserID, occupancyID)
		} else {
			p, err = PaymentGetByID(r.Context(), tx, paymentID, ac.RTID)
		}
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "payment not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}
	if p == nil {
		httpx.WriteNotFound(w, "payment not found")
		return
	}

	if p.ProofPath == nil || *p.ProofPath == "" {
		httpx.WriteNotFound(w, "payment proof not found")
		return
	}

	reader, err := h.storage.GetProof(r.Context(), *p.ProofPath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			httpx.WriteNotFound(w, "proof file not found in storage")
			return
		}
		httpx.WriteInternalServerError(w, "failed to retrieve proof file")
		return
	}
	defer reader.Body.Close()

	w.Header().Set("Content-Type", reader.ContentType)
	if reader.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(reader.Size, 10))
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	_, _ = io.Copy(w, reader.Body)
}

func (h *Handler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	var in VerifyPaymentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	res, err := h.svc.VerifyPayment(r.Context(), tx, ac.RTID, ac.UserID, id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "payment not found")
			return
		}
		if errors.Is(err, ErrBillAlreadyPaid) {
			httpx.WriteConflict(w, "bill is already paid")
			return
		}
		if errors.Is(err, ErrBillCancelled) {
			httpx.WriteConflict(w, "bill is cancelled")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if res.OverpaymentRejected {
		if err := tx.Commit(); err != nil {
			httpx.WriteInternalServerError(w, "commit error")
			return
		}
		httpx.WriteValidationError(w, "payment amount exceeds remaining balance", nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}

	if h.svc.Notifier() != nil && res.Payment != nil {
		submitterID := ""
		if res.Payment.SubmittedBy != nil {
			submitterID = *res.Payment.SubmittedBy
		}
		billID := ""
		if res.Payment.BillID != nil {
			billID = *res.Payment.BillID
		}
		rejectionReason := ""
		if in.Action == "reject" && in.RejectionReason != nil {
			rejectionReason = *in.RejectionReason
		}
		status := "APPROVED"
		if in.Action == "reject" {
			status = "REJECTED"
		}
		h.svc.Notifier().NotifyPaymentVerified(r.Context(), ac.RTID, billID, res.Payment.ID, submitterID, status, rejectionReason)
	}

	writeJSON(w, http.StatusOK, res.Payment)
}

// --- Transactions & Balance Handlers ---

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	// System-only SUPER_ADMIN: query across all RTs with optional rt_id filter
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		// Check if ?rt_id= query param is present → use tenant-scoped query
		rtQuery := r.URL.Query().Get("rt_id")

		var categoryID, transType, status, startDate, endDate *string
		if c := r.URL.Query().Get("category_id"); c != "" {
			categoryID = &c
		}
		if t := r.URL.Query().Get("type"); t != "" {
			transType = &t
		}
		if s := r.URL.Query().Get("status"); s != "" {
			status = &s
		}
		if sd := r.URL.Query().Get("start_date"); sd != "" {
			startDate = &sd
		}
		if ed := r.URL.Query().Get("end_date"); ed != "" {
			endDate = &ed
		}
		if p := r.URL.Query().Get("period"); p != "" && startDate == nil && endDate == nil {
			if !periodRegex.MatchString(p) {
				httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid period format, expected YYYY-MM")
				return
			}
			loc, _ := time.LoadLocation("Asia/Jakarta")
			if loc == nil {
				loc = time.FixedZone("WIB", 7*3600)
			}
			t, err := time.ParseInLocation("2006-01", p, loc)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid period format, expected YYYY-MM")
				return
			}
			st := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
			et := st.AddDate(0, 1, 0).Add(-time.Microsecond)
			sdStr := st.Format(time.RFC3339)
			edStr := et.Format(time.RFC3339)
			startDate = &sdStr
			endDate = &edStr
		}

		tx, err := h.pool.BeginTx(r.Context())
		if err != nil {
			httpx.WriteInternalServerError(w, "transaction error")
			return
		}
		defer tx.Rollback()

		page, pageSize := parsePagination(r)
		offset := (page - 1) * pageSize

		var total int
		var transactions []*Transaction

		if rtQuery != "" {
			// Tenant-scoped query for specific RT (cross-tenant admin filtering)
			total, err = TransactionCount(r.Context(), tx, rtQuery, categoryID, transType, status, startDate, endDate)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
			transactions, err = TransactionList(r.Context(), tx, rtQuery, categoryID, transType, status, startDate, endDate, offset, pageSize)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
		} else {
			// Cross-tenant query (admin viewing all RTs)
			total, err = TransactionCountAll(r.Context(), tx, categoryID, transType, status, startDate, endDate)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
			transactions, err = TransactionListAll(r.Context(), tx, categoryID, transType, status, startDate, endDate, offset, pageSize)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
		}

		totalPages := 0
		if total > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": transactions,
			"pagination": map[string]any{
				"page":        page,
				"per_page":    pageSize,
				"total":       total,
				"total_pages": totalPages,
			},
		})
		return
	}

	var categoryID, transType, status, startDate, endDate *string
	if c := r.URL.Query().Get("category_id"); c != "" {
		categoryID = &c
	}
	if t := r.URL.Query().Get("type"); t != "" {
		transType = &t
	}
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}
	if sd := r.URL.Query().Get("start_date"); sd != "" {
		startDate = &sd
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		endDate = &ed
	}
	if p := r.URL.Query().Get("period"); p != "" && startDate == nil && endDate == nil {
		if !periodRegex.MatchString(p) {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid period format, expected YYYY-MM")
			return
		}
		loc, _ := time.LoadLocation("Asia/Jakarta")
		if loc == nil {
			loc = time.FixedZone("WIB", 7*3600)
		}
		t, err := time.ParseInLocation("2006-01", p, loc)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid period format, expected YYYY-MM")
			return
		}
		st := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		et := st.AddDate(0, 1, 0).Add(-time.Microsecond)
		sdStr := st.Format(time.RFC3339)
		edStr := et.Format(time.RFC3339)
		startDate = &sdStr
		endDate = &edStr
	}

	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	total, err := TransactionCount(r.Context(), tx, ac.RTID, categoryID, transType, status, startDate, endDate)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	transactions, err := TransactionList(r.Context(), tx, ac.RTID, categoryID, transType, status, startDate, endDate, offset, pageSize)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": transactions,
		"pagination": map[string]any{
			"page":        page,
			"per_page":    pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	// System-only SUPER_ADMIN: query across all RTs
	if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
		trans, err := TransactionGetByIDAll(r.Context(), tx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteNotFound(w, "transaction not found")
				return
			}
			httpx.WriteInternalServerError(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, trans)
		return
	}

	trans, err := TransactionGetByID(r.Context(), tx, id, ac.RTID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "transaction not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, trans)
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	handled, rec, idem := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer idem.Finish()

	targetW := w
	if rec != nil {
		targetW = rec
	}

	var in CreateTransactionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(targetW, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(targetW, "transaction error")
		return
	}
	defer tx.Rollback()

	trans, err := h.svc.CreateManualTransaction(r.Context(), tx, ac.RTID, ac.UserID, in)
	if err != nil {
		httpx.WriteValidationError(targetW, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(targetW, "commit error")
		return
	}

	writeJSON(targetW, http.StatusCreated, trans)
}

func (h *Handler) ReverseTransaction(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	handled, rec, idem := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer idem.Finish()

	targetW := w
	if rec != nil {
		targetW = rec
	}

	var in ReverseTransactionInput
	_ = json.NewDecoder(r.Body).Decode(&in)

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(targetW, "transaction error")
		return
	}
	defer tx.Rollback()

	reversal, err := h.svc.ReverseTransaction(r.Context(), tx, ac.RTID, ac.UserID, id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(targetW, "transaction not found")
			return
		}
		if errors.Is(err, ErrAlreadyReversed) || errors.Is(err, ErrCannotReverseNonPost) {
			httpx.WriteConflict(targetW, err.Error())
			return
		}
		httpx.WriteValidationError(targetW, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(targetW, "commit error")
		return
	}

	writeJSON(targetW, http.StatusOK, reversal)
}

func parsePeriodOrDateRange(r *http.Request) (periodStr string, startTime, endTime *time.Time, errCode int, errMsg string) {
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	startDate := strings.TrimSpace(r.URL.Query().Get("start_date"))
	endDate := strings.TrimSpace(r.URL.Query().Get("end_date"))

	if period == "" && startDate == "" && endDate == "" {
		return "", nil, nil, 0, ""
	}

	if period != "" && (startDate != "" || endDate != "") {
		return "", nil, nil, http.StatusBadRequest, "cannot combine period with start_date or end_date"
	}

	if (startDate != "" && endDate == "") || (startDate == "" && endDate != "") {
		return "", nil, nil, http.StatusBadRequest, "start_date and end_date must be provided together"
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	if period != "" {
		if !periodRegex.MatchString(period) {
			return "", nil, nil, http.StatusBadRequest, "invalid period format, expected YYYY-MM"
		}
		t, err := time.ParseInLocation("2006-01", period, loc)
		if err != nil {
			return "", nil, nil, http.StatusBadRequest, "invalid period format, expected YYYY-MM"
		}
		st := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		et := st.AddDate(0, 1, 0)
		return period, &st, &et, 0, ""
	}

	// Explicit startDate and endDate
	if !dateRegex.MatchString(startDate) || !dateRegex.MatchString(endDate) {
		return "", nil, nil, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD"
	}
	st, err := time.ParseInLocation("2006-01-02", startDate, loc)
	if err != nil {
		return "", nil, nil, http.StatusBadRequest, "invalid start_date format, expected YYYY-MM-DD"
	}
	et, err := time.ParseInLocation("2006-01-02", endDate, loc)
	if err != nil {
		return "", nil, nil, http.StatusBadRequest, "invalid end_date format, expected YYYY-MM-DD"
	}
	if !st.Before(et) {
		return "", nil, nil, http.StatusBadRequest, "start_date must be before end_date"
	}

	return "", &st, &et, 0, ""
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	periodStr, startTime, endTime, errCode, errMsg := parsePeriodOrDateRange(r)
	if errCode != 0 {
		httpx.WriteError(w, errCode, "bad_request", errMsg)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	// If no period or date range specified: backward-compatible cumulative balance
	if startTime == nil || endTime == nil {
		// System-only SUPER_ADMIN: query across all RTs
		if ac.SystemRole == auth.SystemRoleSuperAdmin && ac.RTID == "" {
			summary, err := CalculateBalanceAll(r.Context(), tx)
			if err != nil {
				httpx.WriteInternalServerError(w, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, summary)
			return
		}

		summary, err := CalculateBalance(r.Context(), tx, ac.RTID)
		if err != nil {
			httpx.WriteInternalServerError(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, summary)
		return
	}

	// Period balance requested
	if ac.RTID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "rt_id is required for period balance")
		return
	}

	summary, err := CalculatePeriodBalance(r.Context(), tx, ac.RTID, periodStr, *startTime, *endTime)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) UpdateOpeningBalance(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if !requireTenantContext(ac, w) {
		return
	}

	period := chi.URLParam(r, "period")
	if period == "" || !periodRegex.MatchString(period) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid period format, expected YYYY-MM")
		return
	}

	var input UpdateOpeningBalanceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if _, _, err := ParseMoney(input.OpeningBalance); err != nil {
		httpx.WriteValidationError(w, "invalid opening_balance: must be a positive decimal number with up to 2 decimal places", nil)
		return
	}

	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		httpx.WriteValidationError(w, "reason is required", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	resp, err := SetPeriodOpeningBalance(r.Context(), tx, ac.RTID, period, input.OpeningBalance, input.Reason, ac.UserID)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CancelBill(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	if err := h.svc.CancelBill(r.Context(), tx, id, ac.RTID, ac.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "bill not found")
			return
		}
		if errors.Is(err, ErrBillCannotCancel) {
			httpx.WriteConflict(w, "bill cannot be cancelled: has approved payments")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	var in UpdateTransactionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	trans, err := h.svc.UpdateManualTransaction(r.Context(), tx, ac.RTID, ac.UserID, id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "transaction not found")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}

	writeJSON(w, http.StatusOK, trans)
}

func (h *Handler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if !requireTenantContext(ac, w) {
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	period := r.URL.Query().Get("period")
	summary, err := h.svc.GetDashboardSummary(r.Context(), tx, ac.RTID, period)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	_ = tx.Commit()

	writeJSON(w, http.StatusOK, summary)
}

