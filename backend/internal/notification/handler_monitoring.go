package notification

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	httpx "social-finance/internal/http"
)

// MonitoringHousehold represents a household with overdue 3+ months obligatory bills,
// scoped to the caller's RT.
type MonitoringHousehold struct {
	HouseholdOccupancyID string `json:"household_occupancy_id"`
	HouseNumber          string `json:"house_number"`
	Note                 string `json:"note,omitempty"`
	NoteCreatedAt        string `json:"note_created_at,omitempty"`
	NoteCreatedBy        string `json:"note_created_by,omitempty"`
	NoteUpdatedAt        string `json:"note_updated_at,omitempty"`
}

// MonitoringListResponse is the response for GET /api/v1/rw/{rwId}/monitoring.
type MonitoringListResponse struct {
	Households []MonitoringHousehold `json:"households"`
	Total      int                   `json:"total"`
}

// MonitoringNoteRequest is the body for creating/updating a monitoring note.
type MonitoringNoteRequest struct {
	Note string `json:"note"`
}

// MonitoringNoteResponse is the response for note CRUD.
type MonitoringNoteResponse struct {
	HouseholdOccupancyID string `json:"household_occupancy_id"`
	HouseNumber          string `json:"house_number"`
	Note                 string `json:"note"`
	CreatedBy            string `json:"created_by,omitempty"`
	UpdatedBy            string `json:"updated_by,omitempty"`
	CreatedAt            string `json:"created_at,omitempty"`
	UpdatedAt            string `json:"updated_at,omitempty"`
}

// ListMonitoringHouseholds returns households with obligatory bills overdue 3+ months
// for the caller's RT.
func (h *Handler) ListMonitoringHouseholds(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	evalDate := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	households, err := GetMonitoringHouseholds(ctx, h.pool, rtID, evalDate)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to load monitoring list")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":      households,
		"total":     len(households),
		"eval_date": evalDate.Format("2006-01-02"),
	})
}

// CreateMonitoringNote creates a field note for a household.
func (h *Handler) CreateMonitoringNote(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	householdOccupancyID := chi.URLParam(r, "household_occupancy_id")
	if householdOccupancyID == "" {
		httpx.WriteValidationError(w, "household_occupancy_id is required", nil)
		return
	}

	var req MonitoringNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	if strings.TrimSpace(req.Note) == "" {
		httpx.WriteValidationError(w, "note is required", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	note, err := CreateOrUpdateMonitoringNote(ctx, h.pool, rtID, householdOccupancyID, ac.UserID, req.Note)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to save note")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": note,
	})
}

// ListMonitoringNotes returns all notes for a household occupancy.
func (h *Handler) ListMonitoringNotes(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	householdOccupancyID := chi.URLParam(r, "household_occupancy_id")
	if householdOccupancyID == "" {
		httpx.WriteValidationError(w, "household_occupancy_id is required", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	notes, err := GetMonitoringNotes(ctx, h.pool, rtID, householdOccupancyID)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to load notes")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": notes,
	})
}

// UpdateMonitoringNote updates an existing note.
func (h *Handler) UpdateMonitoringNote(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	householdOccupancyID := chi.URLParam(r, "household_occupancy_id")
	if householdOccupancyID == "" {
		httpx.WriteValidationError(w, "household_occupancy_id is required", nil)
		return
	}

	var req MonitoringNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteValidationError(w, "invalid request body", nil)
		return
	}
	if strings.TrimSpace(req.Note) == "" {
		httpx.WriteValidationError(w, "note is required", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	note, err := CreateOrUpdateMonitoringNote(ctx, h.pool, rtID, householdOccupancyID, ac.UserID, req.Note)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to update note")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": note,
	})
}

// ExportMonitoringExcel exports the monitoring list to Excel.
func (h *Handler) ExportMonitoringExcel(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	evalDate := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rtNumber, err := getRTNumber(ctx, h.pool, rtID)
	if err != nil {
		rtNumber = rtID
	}

	households, err := GetMonitoringHouseholds(ctx, h.pool, rtID, evalDate)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to load monitoring list")
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	headers := []string{"No.", "RT", "No. Rumah", "Catatan"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for i, h := range households {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), rtNumber)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), h.HouseNumber)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), h.Note)
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="monitoring_kebersihan_`+evalDate.Format("20060102")+`.xlsx"`)
	if err := f.Write(w); err != nil {
		httpx.WriteInternalServerError(w, "failed to write excel")
		return
	}
}

// ExportMonitoringPDF exports the monitoring list to PDF.
func (h *Handler) ExportMonitoringPDF(w http.ResponseWriter, r *http.Request) {
	ac := auth.GetAuthContext(r)
	if ac == nil || ac.UserID == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	rtID, err := h.resolveTenantRT(r, ac)
	if err != nil || rtID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tenant_context", err.Error())
		return
	}

	evalDate := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rtNumber, err := getRTNumber(ctx, h.pool, rtID)
	if err != nil {
		rtNumber = rtID
	}

	households, err := GetMonitoringHouseholds(ctx, h.pool, rtID, evalDate)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to load monitoring list")
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "LAPORAN MONITORING KEBERSIHAN")
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 6, "Periode: "+evalDate.Format("02 Januari 2006"))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(10, 7, "No.")
	pdf.Cell(20, 7, "RT")
	pdf.Cell(30, 7, "No. Rumah")
	pdf.Cell(60, 7, "Catatan")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 10)

	for i, h := range households {
		pdf.Cell(10, 7, fmt.Sprintf("%d", i+1))
		pdf.Cell(20, 7, rtNumber)
		pdf.Cell(30, 7, h.HouseNumber)
		pdf.MultiCell(60, 7, h.Note, "", "", false)
		pdf.Ln(-1)
	}

	var pdfBuf bytes.Buffer
	err = pdf.Output(&pdfBuf)
	pdfBytes := pdfBuf.Bytes()
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to generate pdf")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="monitoring_kebersihan_`+evalDate.Format("20060102")+`.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	if _, err := w.Write(pdfBytes); err != nil {
		httpx.WriteInternalServerError(w, "failed to write pdf")
		return
	}
}

// GetMonitoringHouseholds returns households with obligatory bills overdue 3+ months
// for the given RT and evaluation date.
func GetMonitoringHouseholds(ctx context.Context, pool *database.Pool, rtID string, evalDate time.Time) ([]MonitoringHousehold, error) {
	if strings.TrimSpace(rtID) == "" {
		return nil, nil
	}

	// Threshold date: bills due on or before this date are considered overdue 3+ months.
	thresholdDate := finance.AddCalendarMonths(evalDate, -3)

	query := `
		WITH eligible_bills AS (
		    SELECT DISTINCT ON (b.household_occupancy_id)
		        b.id AS bill_id,
		        b.household_occupancy_id,
		        ph.house_number,
		        b.due_date,
		        b.amount::text AS bill_amount,
		        COALESCE((
		            SELECT SUM(v.amount)
		            FROM (
		                SELECT p.amount
		                FROM payments p
		                WHERE p.bill_id = b.id AND p.status = 'APPROVED'
		                  AND NOT EXISTS (
		                      SELECT 1 FROM payment_allocations pa
		                      WHERE pa.payment_id = p.id AND pa.bill_id = b.id
		                  )
		                UNION ALL
		                SELECT pa.amount
		                FROM payment_allocations pa
		                JOIN payments p ON p.id = pa.payment_id
		                WHERE pa.bill_id = b.id AND p.status = 'APPROVED'
		            ) v
		        ), 0)::text AS paid_amount,
		        (b.amount::numeric - COALESCE((
		            SELECT SUM(v.amount)
		            FROM (
		                SELECT p.amount
		                FROM payments p
		                WHERE p.bill_id = b.id AND p.status = 'APPROVED'
		                  AND NOT EXISTS (
		                      SELECT 1 FROM payment_allocations pa
		                      WHERE pa.payment_id = p.id AND pa.bill_id = b.id
		                  )
		                UNION ALL
		                SELECT pa.amount
		                FROM payment_allocations pa
		                JOIN payments p ON p.id = pa.payment_id
		                WHERE pa.bill_id = b.id AND p.status = 'APPROVED'
		            ) v
		            ), 0))::text AS outstanding,
		            rn.note,
		            rn.created_at AS note_created_at,
		            u.full_name AS note_created_by,
		            rn.updated_at AS note_updated_at
		    FROM bills b
		    JOIN household_occupancies ho ON ho.id = b.household_occupancy_id
		        AND ho.end_date IS NULL
		        AND (ho.start_date IS NULL OR ho.start_date <= $3::date)
		    JOIN households hh ON hh.id = ho.household_id
		        AND hh.rt_id = b.rt_id AND hh.is_active = true
		    JOIN physical_houses ph ON ph.id = ho.physical_house_id
		        AND ph.rt_id = b.rt_id AND ph.is_active = true
		    JOIN dues d ON d.id = b.due_id AND d.is_active = true AND d.is_voluntary = false
		    LEFT JOIN rw_monitoring_notes rn ON rn.household_occupancy_id = ho.id AND rn.rt_id = b.rt_id
		    LEFT JOIN users u ON u.id = rn.created_by
		    WHERE b.rt_id = $1
		      AND b.is_active = true
		      AND b.is_obligatory = true
		      AND b.status IN ('unpaid', 'partial')
		      AND b.due_date < $2::date
		      AND (ho.start_date IS NULL OR b.period >= to_char(ho.start_date, 'YYYY-MM'))
		      AND (b.amount::numeric - COALESCE((
		            SELECT SUM(v.amount)
		            FROM (
		                SELECT p.amount
		                FROM payments p
		                WHERE p.bill_id = b.id AND p.status = 'APPROVED'
		                  AND NOT EXISTS (
		                      SELECT 1 FROM payment_allocations pa
		                      WHERE pa.payment_id = p.id AND pa.bill_id = b.id
		                  )
		                UNION ALL
		                SELECT pa.amount
		                FROM payment_allocations pa
		                JOIN payments p ON p.id = pa.payment_id
		                WHERE pa.bill_id = b.id AND p.status = 'APPROVED'
		            ) v
		            ), 0)) > 0
		    ORDER BY b.household_occupancy_id, rn.created_at DESC NULLS LAST
		)
		SELECT
		    household_occupancy_id,
		    house_number,
		    bill_amount,
		    paid_amount,
		    outstanding,
		    due_date,
		    note,
		    note_created_at,
		    note_created_by,
		    note_updated_at
		FROM eligible_bills
		ORDER BY house_number
	`

	rows, err := pool.Raw().QueryContext(
		ctx,
		query,
		rtID,
		thresholdDate.Format("2006-01-02"),
		evalDate.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("query monitoring households: %w", err)
	}
	defer rows.Close()

	var list []MonitoringHousehold
	for rows.Next() {
		var h MonitoringHousehold
		var billAmount, paidAmount, outstanding, dueDate string
		var note, noteCreatedAt, noteCreatedBy, noteUpdatedAt sql.NullString
		if err := rows.Scan(
			&h.HouseholdOccupancyID,
			&h.HouseNumber,
			&billAmount,
			&paidAmount,
			&outstanding,
			&dueDate,
			&note,
			&noteCreatedAt,
			&noteCreatedBy,
			&noteUpdatedAt,
		); err != nil {
			return nil, err
		}
		if note.Valid {
			h.Note = note.String
		}
		if noteCreatedAt.Valid {
			h.NoteCreatedAt = noteCreatedAt.String
		}
		if noteCreatedBy.Valid {
			h.NoteCreatedBy = noteCreatedBy.String
		}
		if noteUpdatedAt.Valid {
			h.NoteUpdatedAt = noteUpdatedAt.String
		}
		list = append(list, h)
	}
	if list == nil {
		list = []MonitoringHousehold{}
	}
	return list, rows.Err()
}

// CreateOrUpdateMonitoringNote creates or updates a field note for a household occupancy.
func CreateOrUpdateMonitoringNote(ctx context.Context, pool *database.Pool, rtID, householdOccupancyID, userID, note string) (*MonitoringNoteResponse, error) {
	if strings.TrimSpace(rtID) == "" || strings.TrimSpace(householdOccupancyID) == "" || strings.TrimSpace(userID) == "" {
		return nil, errors.New("rt_id, household_occupancy_id, and user_id are required")
	}

	// Verify the household occupancy belongs to this RT and has an overdue obligatory bill.
	var exists bool
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM household_occupancies ho
		    JOIN bills b ON b.household_occupancy_id = ho.id
		    WHERE ho.id = $1 AND b.rt_id = $2 AND b.is_active = true AND b.is_obligatory = true
		       AND b.status IN ('unpaid', 'partial')
		       AND b.due_date < $3
		       AND (b.amount::numeric - COALESCE((
		                SELECT SUM(v.amount)
		                FROM (
		                    SELECT p.amount
		                    FROM payments p
		                    WHERE p.bill_id = b.id AND p.status = 'APPROVED'
		                      AND NOT EXISTS (
		                          SELECT 1 FROM payment_allocations pa
		                          WHERE pa.payment_id = p.id AND pa.bill_id = b.id
		                      )
		                    UNION ALL
		                    SELECT pa.amount
		                    FROM payment_allocations pa
		                    JOIN payments p ON p.id = pa.payment_id
		                    WHERE pa.bill_id = b.id AND p.status = 'APPROVED'
		                ) v
		            ), 0)) > 0
		)`, householdOccupancyID, rtID, finance.AddCalendarMonths(time.Now(), -3).Format("2006-01-02")).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("validate household occupancy: %w", err)
	}
	if !exists {
		return nil, errors.New("household occupancy not found or does not have an overdue obligatory bill in this RT")
	}

	// Upsert the note.
	var noteID, createdBy, updatedBy string
	var createdAt, updatedAt time.Time
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rw_monitoring_notes (rt_id, household_occupancy_id, note, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4, NOW(), NOW())
		ON CONFLICT (household_occupancy_id)
		DO UPDATE SET note = EXCLUDED.note, updated_by = EXCLUDED.updated_by, updated_at = NOW()
		RETURNING id, created_by, updated_by, created_at, updated_at
	`, rtID, householdOccupancyID, note, userID).Scan(&noteID, &createdBy, &updatedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert monitoring note: %w", err)
	}

	houseNumber, err := getHouseNumberForOccupancy(ctx, pool, rtID, householdOccupancyID)
	if err != nil {
		houseNumber = ""
	}

	return &MonitoringNoteResponse{
		HouseholdOccupancyID: householdOccupancyID,
		HouseNumber:          houseNumber,
		Note:                 note,
		CreatedBy:            createdBy,
		UpdatedBy:            updatedBy,
		CreatedAt:            createdAt.Format(time.RFC3339),
		UpdatedAt:            updatedAt.Format(time.RFC3339),
	}, nil
}

// GetMonitoringNotes returns all notes for a household occupancy.
func GetMonitoringNotes(ctx context.Context, pool *database.Pool, rtID, householdOccupancyID string) ([]MonitoringNoteResponse, error) {
	rows, err := pool.Raw().QueryContext(ctx, `
		SELECT household_occupancy_id, note, created_by, updated_by, created_at, updated_at
		FROM rw_monitoring_notes
		WHERE household_occupancy_id = $1 AND rt_id = $2
		ORDER BY created_at DESC
	`, householdOccupancyID, rtID)
	if err != nil {
		return nil, fmt.Errorf("query monitoring notes: %w", err)
	}
	defer rows.Close()

	var list []MonitoringNoteResponse
	for rows.Next() {
		var n MonitoringNoteResponse
		n.HouseholdOccupancyID = householdOccupancyID
		if err := rows.Scan(&n.Note, &n.CreatedBy, &n.UpdatedBy, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		hn, err := getHouseNumberForOccupancy(ctx, pool, rtID, householdOccupancyID)
		if err == nil {
			n.HouseNumber = hn
		}
		list = append(list, n)
	}
	if list == nil {
		list = []MonitoringNoteResponse{}
	}
	return list, rows.Err()
}

func getHouseNumberForOccupancy(ctx context.Context, pool *database.Pool, rtID, householdOccupancyID string) (string, error) {
	var houseNumber string
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT ph.house_number
		FROM household_occupancies ho
		JOIN physical_houses ph ON ph.id = ho.physical_house_id
		WHERE ho.id = $1 AND ho.rt_id = $2
	`, householdOccupancyID, rtID).Scan(&houseNumber)
	if err != nil {
		return "", err
	}
	return houseNumber, nil
}

func getRTNumber(ctx context.Context, pool *database.Pool, rtID string) (string, error) {
	var rtNumber string
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT rt FROM rts WHERE id = $1 AND is_active = true
	`, rtID).Scan(&rtNumber)
	if err != nil {
		return "", err
	}
	return rtNumber, nil
}
