package security

import (
	"net/http"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	httpx "social-finance/internal/http"
)

// Device-scoped access to kebersihan monitoring notes.
//
// The Pos kiosk is bound to exactly one RT, and its user cannot escalate to a
// different RT. Devices may update the note for any occupancy owned by their
// own RT; they cannot read other tables or other RTs.

// PosUpdateSampahNote handles
//
//	PATCH /api/v1/security-device/monitoring/sampah/{householdOccupancyID}/note
//
// Body: {"note": "..."}
func (h *Handler) PosUpdateSampahNote(w http.ResponseWriter, r *http.Request) {
	dc := GetDeviceContext(r)
	if dc == nil {
		httpx.WriteUnauthorized(w, "missing security device context")
		return
	}
	hid := chi.URLParam(r, "householdOccupancyID")
	if _, err := uuid.Parse(hid); err != nil {
		httpx.WriteValidationError(w, "invalid household occupancy id", nil)
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		httpx.WriteValidationError(w, "note is required", nil)
		return
	}
	if len(note) > 2000 {
		httpx.WriteValidationError(w, "note must be 2000 characters or fewer", nil)
		return
	}

	// Occupancy must exist, be active, and belong to the device's bound RT.
	var ok bool
	err := h.pool.Raw().QueryRowContext(r.Context(),
		`SELECT EXISTS (
			SELECT 1
			FROM household_occupancies ho
			JOIN physical_houses ph ON ph.id = ho.physical_house_id
			JOIN households hs ON hs.id = ho.household_id
			WHERE ho.id = $1
			  AND ph.rt_id = $2
			  AND ho.end_date IS NULL
			  AND ph.is_active = true
			  AND hs.is_active = true
		)`,
		hid, dc.RTID,
	).Scan(&ok)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to verify household occupancy")
		return
	}
	if !ok {
		httpx.WriteNotFound(w, "household occupancy not found")
		return
	}

	now := NowJakarta(nil).UTC()
	_, err = h.pool.Raw().ExecContext(r.Context(),
		`INSERT INTO rw_monitoring_notes (rt_id, household_occupancy_id, note, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)
		 ON CONFLICT (household_occupancy_id)
		 DO UPDATE SET note = EXCLUDED.note, updated_at = EXCLUDED.updated_at`,
		dc.RTID, hid, note, now,
	)
	if err != nil {
		httpx.WriteInternalServerError(w, "failed to save monitoring note")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"household_occupancy_id": hid,
		"rt_id":                  dc.RTID,
		"note":                   note,
		"updated_at":             now.Format("2006-01-02T15:04:05Z07:00"),
	})
}
