package finance

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
	if ac == nil || ac.RTID == "" {
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

// --- Bills Handlers ---

func (h *Handler) ListBills(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	var occupancyID *string
	// If warga, restrict strictly to citizen's own active household occupancy!
	if ac.TenantRole == auth.RoleWarga {
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
	var period *string
	if p := r.URL.Query().Get("period"); p != "" {
		period = &p
	}

	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	total, err := BillCount(r.Context(), tx, ac.RTID, occupancyID, dueID, status, period)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	bills, err := BillList(r.Context(), tx, ac.RTID, occupancyID, dueID, status, period, offset, pageSize)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
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

	bill, err := BillGetByID(r.Context(), tx, id, ac.RTID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "bill not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	if ac.TenantRole == auth.RoleWarga {
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

	handled, rec, finish := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer finish()

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
		httpx.WriteInternalServerError(targetW, "commit error")
		return
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
	writeJSON(w, http.StatusCreated, bill)
}

// --- Payments Handlers ---

func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
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
	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}

	page, pageSize := parsePagination(r)
	offset := (page - 1) * pageSize

	total, err := PaymentCount(r.Context(), tx, ac.RTID, billID, status)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	payments, err := PaymentList(r.Context(), tx, ac.RTID, billID, status, offset, pageSize)
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

	p, err := PaymentGetByID(r.Context(), tx, id, ac.RTID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "payment not found")
			return
		}
		httpx.WriteInternalServerError(w, err.Error())
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

	handled, rec, finish := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer finish()

	targetW := w
	if rec != nil {
		targetW = rec
	}

	var in CreatePaymentInput
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

	p, err := h.svc.CreatePayment(r.Context(), tx, ac.RTID, ac.UserID, string(ac.TenantRole), in)
	if err != nil {
		httpx.WriteValidationError(targetW, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(targetW, "commit error")
		return
	}

	writeJSON(targetW, http.StatusCreated, p)
}

func (h *Handler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
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

	p, err := h.svc.VerifyPayment(r.Context(), tx, ac.RTID, ac.UserID, id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteNotFound(w, "payment not found")
			return
		}
		httpx.WriteValidationError(w, err.Error(), nil)
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteInternalServerError(w, "commit error")
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// --- Transactions & Balance Handlers ---

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
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

	handled, rec, finish := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer finish()

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
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}

	handled, rec, finish := HandleIdempotentRequest(r.Context(), h.pool, ac.RTID, ac.UserID, w, r)
	if handled {
		return
	}
	defer finish()

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

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ac, ok := getAuthContext(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	tx, err := h.pool.BeginTx(r.Context())
	if err != nil {
		httpx.WriteInternalServerError(w, "transaction error")
		return
	}
	defer tx.Rollback()

	summary, err := CalculateBalance(r.Context(), tx, ac.RTID)
	if err != nil {
		httpx.WriteInternalServerError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}
