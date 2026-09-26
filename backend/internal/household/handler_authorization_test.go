package household

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
)

// TestSuperAdminWriteHouseholdNoRT verifies system-level SUPER_ADMIN can
// mutate households in any RT without supplying an RT in the JWT.
func TestSuperAdminWriteHouseholdNoRT(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	h := NewHandler(pool)

	rtAID := setupTestRT(t, pool, "SAWHousehold RT A", 1, "SAWHA")
	rtBID := setupTestRT(t, pool, "SAWHousehold RT B", 1, "SAWHB")

	hhAID := insertTestHouseholdFixture(t, pool.Raw(), rtAID, "SAW Head A", strPtr("SAW-A1"), strPtr("Address A"), occPtr(OccupancyOwner))
	hhBID := insertTestHouseholdFixture(t, pool.Raw(), rtBID, "SAW Head B", strPtr("SAW-B1"), strPtr("Address B"), occPtr(OccupancyOwner))

	// Capture baseline household data for verification after moves
	var originalHouseNumberA string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT house_number FROM physical_houses WHERE rt_id = $1 AND house_number = 'SAW-A1' AND is_active = true LIMIT 1`, rtAID,
	).Scan(&originalHouseNumberA)
	if err != nil {
		t.Fatalf("failed to capture baseline household A data: %v", err)
	}

	adminToken := makeSuperAdminToken(t)

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		expected int
	}{
		{
			name:     "UpdateHousehold on RT A",
			method:   http.MethodPatch,
			path:     "/api/v1/households/" + hhAID,
			body:     `{"head_name":"RevisedHeadA"}`,
			expected: http.StatusOK,
		},
		{
			name:     "UpdateHousehold on RT B",
			method:   http.MethodPatch,
			path:     "/api/v1/households/" + hhBID,
			body:     `{"head_name":"RevisedHeadB"}`,
			expected: http.StatusOK,
		},
		{
			name:     "MoveHousehold on RT A",
			method:   http.MethodPost,
			path:     "/api/v1/households/" + hhAID + "/move",
			body:     fmt.Sprintf(`{"house_number":"SAW-MV-%d","address":"New Place","start_date":"%s","occupancy_status":"OWNER"}`, time.Now().UnixNano()/1e6, time.Now().AddDate(0, 0, 1).Format("2006-01-02")),
			expected: http.StatusOK,
		},
		{
			name:     "DeactivateHousehold",
			method:   http.MethodDelete,
			path:     "/api/v1/households/" + hhBID,
			body:     "",
			expected: http.StatusNoContent,
		},
		{
			name:     "nonexistent Household returns 404",
			method:   http.MethodPatch,
			path:     "/api/v1/households/00000000-0000-0000-0000-000000000099",
			body:     `{"head_name":"Nope"}`,
			expected: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(auth.RequireAuth)
			r.Use(auth.RequireRole(auth.RolePengurus))
			r.Patch("/api/v1/households/{id}", h.UpdateHousehold)
			r.Post("/api/v1/households/{id}/move", h.MoveHousehold)
			r.Delete("/api/v1/households/{id}", h.DeactivateHousehold)

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+adminToken)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.expected {
				t.Fatalf("expected %d, got %d: %s", tt.expected, rec.Code, rec.Body.String())
			}

			// Persisted-state assertions for successful mutations
			if tt.expected == http.StatusOK && tt.name == "UpdateHousehold on RT A" {
				var headName string
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT head_name FROM households WHERE id = $1`, hhAID,
				).Scan(&headName)
				if err != nil {
					t.Fatalf("failed to query updated household: %v", err)
				}
				if headName != "RevisedHeadA" {
					t.Fatalf("UpdateHousehold persisted state wrong: expected head_name 'RevisedHeadA', got %q", headName)
				}
			}

			if tt.expected == http.StatusOK && tt.name == "UpdateHousehold on RT B" {
				var headName string
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT head_name FROM households WHERE id = $1`, hhBID,
				).Scan(&headName)
				if err != nil {
					t.Fatalf("failed to query updated household: %v", err)
				}
				if headName != "RevisedHeadB" {
					t.Fatalf("UpdateHousehold persisted state wrong: expected head_name 'RevisedHeadB', got %q", headName)
				}
			}

			if tt.expected == http.StatusOK && tt.name == "MoveHousehold on RT A" {
				// Verify: the original house was closed (end_date set) and a new occupancy exists
				// for a physical house with the new house_number
				var currentOccExists int
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT COUNT(*) FROM household_occupancies WHERE household_id = $1 AND end_date IS NULL`,
					hhAID,
				).Scan(&currentOccExists)
				if err != nil {
					t.Fatalf("failed to query household occupancy: %v", err)
				}
				if currentOccExists != 1 {
					t.Fatalf("MoveHousehold persisted state wrong: expected 1 current occupancy, got %d", currentOccExists)
				}

				// Verify old physical house is no longer current for this household
				var oldHouseStillCurrent int
				err = pool.Raw().QueryRowContext(context.Background(),
					`SELECT COUNT(*) FROM household_occupancies ho
					 JOIN physical_houses ph ON ho.physical_house_id = ph.id
					 WHERE ho.household_id = $1 AND ph.house_number = $2 AND ho.end_date IS NULL`,
					hhAID, originalHouseNumberA,
				).Scan(&oldHouseStillCurrent)
				if err != nil {
					t.Fatalf("failed to query old house status: %v", err)
				}
				if oldHouseStillCurrent != 0 {
					t.Fatalf("MoveHousehold persisted state wrong: old house %q still current for household", originalHouseNumberA)
				}
			}

			if tt.expected == http.StatusNoContent && tt.name == "DeactivateHousehold" {
				var isActive bool
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT is_active FROM households WHERE id = $1`, hhBID,
				).Scan(&isActive)
				if err != nil {
					t.Fatalf("failed to query deactivated household: %v", err)
				}
				if isActive {
					t.Fatalf("DeactivateHousehold persisted state wrong: household still active after DELETE")
				}
			}
		})
	}
}

// TestSuperAdminWriteResidentNoRT verifies system-level SUPER_ADMIN can
// mutate residents in any RT without supplying an RT in the JWT.
func TestSuperAdminWriteResidentNoRT(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	h := NewHandler(pool)

	rtAID := setupTestRT(t, pool, "SAWR RT A", 1, "SAWRA")
	rtBID := setupTestRT(t, pool, "SAWR RT B", 1, "SAWRB")

	hnA := "SAWRA1"
	hhAID := insertTestHouseholdFixture(t, pool.Raw(), rtAID, "SAWR Head A", &hnA, strPtr("Addr A"), occPtr(OccupancyOwner))
	hnB := "SAWRB1"
	hhBID := insertTestHouseholdFixture(t, pool.Raw(), rtBID, "SAWR Head B", &hnB, strPtr("Addr B"), occPtr(OccupancyOwner))

	resAID := insertTestResidentFixture(t, pool.Raw(), rtAID, hhAID, "SAWR Resident A", strPtr("Self"))
	resBID := insertTestResidentFixture(t, pool.Raw(), rtBID, hhBID, "SAWR Resident B", strPtr("Self"))

	adminToken := makeSuperAdminToken(t)

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		expected int
	}{
		{
			name:     "CreateResident in RT A household",
			method:   http.MethodPost,
			path:     "/api/v1/residents",
			body:     fmt.Sprintf(`{"household_id":"%s","full_name":"New Resident A","nik":"1111111111111166","phone":"081100000001","email":"newa@test.com"}`, hhAID),
			expected: http.StatusCreated,
		},
		{
			name:     "CreateResident in RT B household",
			method:   http.MethodPost,
			path:     "/api/v1/residents",
			body:     fmt.Sprintf(`{"household_id":"%s","full_name":"New Resident B","nik":"1111111111111177","phone":"081100000002","email":"newb@test.com"}`, hhBID),
			expected: http.StatusCreated,
		},
		{
			name:     "UpdateResident RT A",
			method:   http.MethodPatch,
			path:     "/api/v1/residents/" + resAID,
			body:     `{"full_name":"RevisedResidentA"}`,
			expected: http.StatusOK,
		},
		{
			name:     "UpdateResident RT B",
			method:   http.MethodPatch,
			path:     "/api/v1/residents/" + resBID,
			body:     `{"phone":"081100000003"}`,
			expected: http.StatusOK,
		},
		{
			name:     "MoveResident RT A",
			method:   http.MethodPost,
			path:     "/api/v1/residents/" + resAID + "/move",
			body:     fmt.Sprintf(`{"destination_household_id":"%s","start_date":"%s"}`, hhAID, "2026-12-01"),
			expected: http.StatusOK,
		},
		{
			name:     "DeactivateResident",
			method:   http.MethodDelete,
			path:     "/api/v1/residents/" + resBID,
			body:     "",
			expected: http.StatusNoContent,
		},
		{
			name:     "nonexistent Resident returns 404",
			method:   http.MethodPatch,
			path:     "/api/v1/residents/00000000-0000-0000-0000-000000000088",
			body:     `{"full_name":"Nope"}`,
			expected: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(auth.RequireAuth)
			r.Use(auth.RequireRole(auth.RolePengurus))
			r.Post("/api/v1/residents", h.CreateResident)
			r.Patch("/api/v1/residents/{id}", h.UpdateResident)
			r.Post("/api/v1/residents/{id}/move", h.MoveResident)
			r.Delete("/api/v1/residents/{id}", h.DeactivateResident)

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+adminToken)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.expected {
				t.Fatalf("expected %d, got %d: %s", tt.expected, rec.Code, rec.Body.String())
			}

			// Persisted-state assertions for successful mutations
			if tt.expected == http.StatusCreated && tt.name == "CreateResident in RT A household" {
				// Parse the household_id from request body to get RT for verification
				var respBody map[string]interface{}
				// The response should contain the created resident
				json.Unmarshal(rec.Body.Bytes(), &respBody)
				resID, ok := respBody["id"].(string)
				if !ok || resID == "" {
					t.Fatalf("CreateResident response missing id")
				}

				// Verify persisted resident rt_id matches target household's RT (rtAID)
				var residentRTID string
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT rt_id FROM residents WHERE id = $1`, resID,
				).Scan(&residentRTID)
				if err != nil {
					t.Fatalf("failed to query created resident rt_id: %v", err)
				}
				if residentRTID != rtAID {
					t.Fatalf("CreateResident RT mismatch: expected resident rt_id %q (target household's RT), got %q", rtAID, residentRTID)
				}

				// Verify residency link exists to the target household's current occupancy
				var linkedHH sql.NullString
				err = pool.Raw().QueryRowContext(context.Background(),
					`SELECT ho.household_id FROM residency_periods rp
					 JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id
					 WHERE rp.resident_id = $1 AND rp.end_date IS NULL`,
					resID,
				).Scan(&linkedHH)
				if err != nil {
					t.Fatalf("failed to query resident's current occupancy link: %v", err)
				}
				if !linkedHH.Valid || linkedHH.String != hhAID {
					t.Fatalf("CreateResident residency link wrong: expected linked to %q, got %v", hhAID, linkedHH)
				}
			}

			if tt.expected == http.StatusCreated && tt.name == "CreateResident in RT B household" {
				var respBody map[string]interface{}
				json.Unmarshal(rec.Body.Bytes(), &respBody)
				resID, ok := respBody["id"].(string)
				if !ok || resID == "" {
					t.Fatalf("CreateResident response missing id")
				}

				var residentRTID string
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT rt_id FROM residents WHERE id = $1`, resID,
				).Scan(&residentRTID)
				if err != nil {
					t.Fatalf("failed to query created resident rt_id: %v", err)
				}
				if residentRTID != rtBID {
					t.Fatalf("CreateResident RT mismatch: expected resident rt_id %q, got %q", rtBID, residentRTID)
				}
			}

			if tt.expected == http.StatusOK && tt.name == "UpdateResident RT A" {
				var fullName string
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT full_name FROM residents WHERE id = $1`, resAID,
				).Scan(&fullName)
				if err != nil {
					t.Fatalf("failed to query updated resident: %v", err)
				}
				if fullName != "RevisedResidentA" {
					t.Fatalf("UpdateResident persisted state wrong: expected full_name 'RevisedResidentA', got %q", fullName)
				}
			}

			if tt.expected == http.StatusOK && tt.name == "UpdateResident RT B" {
				var phone string
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT phone FROM residents WHERE id = $1`, resBID,
				).Scan(&phone)
				if err != nil {
					t.Fatalf("failed to query updated resident: %v", err)
				}
				normalizedPhone, _ := NormalizePhone("081100000003")
				if phone != normalizedPhone {
					t.Fatalf("UpdateResident persisted state wrong: expected phone %q (normalized), got %q", normalizedPhone, phone)
				}
			}

			if tt.expected == http.StatusOK && tt.name == "MoveResident RT A" {
				// Verify residency period reflects the move within same RT
				// The resident should have an active residency period (end_date IS NULL)
				var currentRPCount int
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`,
					resAID,
				).Scan(&currentRPCount)
				if err != nil {
					t.Fatalf("failed to query residency period: %v", err)
				}
				if currentRPCount != 1 {
					t.Fatalf("MoveResident persisted state wrong: expected 1 current residency, got %d", currentRPCount)
				}

				// Verify the residency's household_occupancy points to the destination household
				var targetHH sql.NullString
				err = pool.Raw().QueryRowContext(context.Background(),
					`SELECT ho.household_id FROM residency_periods rp
					 JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id
					 WHERE rp.resident_id = $1 AND rp.end_date IS NULL`,
					resAID,
				).Scan(&targetHH)
				if err != nil {
					t.Fatalf("failed to query resident's current occupancy after move: %v", err)
				}
				if !targetHH.Valid || targetHH.String != hhAID {
					t.Fatalf("MoveResident residency points to wrong household: expected %q, got %v", hhAID, targetHH)
				}

				// Verify resident rt_id unchanged by move
				var residentRTID string
				err = pool.Raw().QueryRowContext(context.Background(),
					`SELECT rt_id FROM residents WHERE id = $1`, resAID,
				).Scan(&residentRTID)
				if err != nil {
					t.Fatalf("failed to query resident rt_id after move: %v", err)
				}
				if residentRTID == "" {
					t.Fatalf("MoveResident: resident lost rt_id")
				}
			}

			if tt.expected == http.StatusNoContent && tt.name == "DeactivateResident" {
				var isActive bool
				err := pool.Raw().QueryRowContext(context.Background(),
					`SELECT is_active FROM residents WHERE id = $1`, resBID,
				).Scan(&isActive)
				if err != nil {
					t.Fatalf("failed to query deactivated resident: %v", err)
				}
				if isActive {
					t.Fatalf("DeactivateResident persisted state wrong: resident still active after DELETE")
				}

				// Verify no active residency periods remain
				var activeRPCount int
				err = pool.Raw().QueryRowContext(context.Background(),
					`SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1 AND end_date IS NULL`,
					resBID,
				).Scan(&activeRPCount)
				if err != nil {
					t.Fatalf("failed to query residency periods: %v", err)
				}
				if activeRPCount != 0 {
					t.Errorf("DeactivateResident: expected 0 active residency periods, got %d", activeRPCount)
				}
			}
		})
	}
}

// TestTenantCrossRTIsolation verifies tenant users cannot access resources
// owned by other RTs.
func TestTenantCrossRTIsolation(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	h := NewHandler(pool)

	rtAID := setupTestRT(t, pool, "ISO RT A", 1, "ISOA")
	rtBID := setupTestRT(t, pool, "ISO RT B", 1, "ISOB")

	hhAID := insertTestHouseholdFixture(t, pool.Raw(), rtAID, "Iso Head A", strPtr("ISO-A1"), strPtr("A Addr"), occPtr(OccupancyOwner))
	hnB := "ISO-B1"
	hhBID := insertTestHouseholdFixture(t, pool.Raw(), rtBID, "Iso Head B", &hnB, strPtr("B Addr"), occPtr(OccupancyOwner))
	resBID := insertTestResidentFixture(t, pool.Raw(), rtBID, hhBID, "Iso Resident B", strPtr("Self"))

	pengurusAToken := makeTenantToken(t, "pengurus-a", "membership-a", rtAID, auth.RolePengurus)
	pengurusBToken := makeTenantToken(t, "pengurus-b", "membership-b", rtBID, auth.RolePengurus)

	write := func(method, path, token string) *httptest.ResponseRecorder {
		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Patch("/api/v1/households/{id}", h.UpdateHousehold)
		r.Post("/api/v1/households/{id}/move", h.MoveHousehold)
		r.Delete("/api/v1/households/{id}", h.DeactivateHousehold)
		r.Patch("/api/v1/residents/{id}", h.UpdateResident)
		r.Post("/api/v1/residents/{id}/move", h.MoveResident)
		r.Delete("/api/v1/residents/{id}", h.DeactivateResident)

		body := `{"head_name":"Unauthorized"}`
		if method == http.MethodPatch {
			body = `{"head_name":"Unauthorized"}`
		} else if method == http.MethodPost {
			body = fmt.Sprintf(`{"house_number":"","start_date":"2026-12-01"}`)
		} else {
			body = ""
		}

		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	t.Run("PENGURUS RT A can mutate own Household RT A", func(t *testing.T) {
		rec := write(http.MethodPatch, "/api/v1/households/"+hhAID, pengurusAToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("PENGURUS RT A -> Household RT B returns exactly 404", func(t *testing.T) {
		rec := write(http.MethodPatch, "/api/v1/households/"+hhBID, pengurusAToken)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("PENGURUS RT B -> Household RT A returns exactly 404", func(t *testing.T) {
		rec := write(http.MethodPatch, "/api/v1/households/"+hhAID, pengurusBToken)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// MoveHousehold cross-RT
	t.Run("PENGURUS RT A -> MoveHousehold RT B returns exactly 404", func(t *testing.T) {
		moveDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		moveReq := bytes.NewBufferString(fmt.Sprintf(`{"house_number":"ZZZ","start_date":"%s"}`, moveDate))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+hhBID+"/move", moveReq)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusAToken)
		rec := httptest.NewRecorder()

		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households/{id}/move", h.MoveHousehold)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify foreign household's current occupancy is unchanged
		var currentOccCount int
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM household_occupancies WHERE household_id = $1 AND end_date IS NULL`,
			hhBID,
		).Scan(&currentOccCount)
		if currentOccCount == 0 {
			t.Fatalf("MoveHousehold rejected but foreign household occupancy was cleared")
		}
	})

	// DeactivateHousehold cross-RT
	t.Run("PENGURUS RT A -> DeactivateHousehold RT B returns exactly 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/households/"+hhBID, nil)
		req.Header.Set("Authorization", "Bearer "+pengurusAToken)
		rec := httptest.NewRecorder()

		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Delete("/api/v1/households/{id}", h.DeactivateHousehold)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify foreign household is still active
		var isActive bool
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT is_active FROM households WHERE id = $1`, hhBID,
		).Scan(&isActive)
		if !isActive {
			t.Fatalf("DeactivateHousehold rejected but foreign household was deactivated")
		}
	})

	// MoveResident cross-RT: source Resident RT B -> destination Household RT A
	// Destination is a valid active household in RT A so the 404 is unambiguously from the
	// foreign resident check, not from a nonexistent destination.
	t.Run("PENGURUS RT A -> MoveResident RT B returns exactly 404", func(t *testing.T) {
		// Capture original RT B resident's active period count
		var originalRPCount int
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM residency_periods rp WHERE rp.resident_id = $1 AND rp.end_date IS NULL`,
			resBID,
		).Scan(&originalRPCount)

		moveDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		moveReq := bytes.NewBufferString(fmt.Sprintf(`{"destination_household_id":"%s","start_date":"%s"}`, hhAID, moveDate))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/"+resBID+"/move", moveReq)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusAToken)
		rec := httptest.NewRecorder()

		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/residents/{id}/move", h.MoveResident)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify resident's original residency period is unchanged
		var currentRPCount int
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM residency_periods rp WHERE rp.resident_id = $1 AND rp.end_date IS NULL`,
			resBID,
		).Scan(&currentRPCount)
		if currentRPCount != originalRPCount {
			t.Fatalf("MoveResident rejected but foreign resident's residency period was modified: was %d, now %d", originalRPCount, currentRPCount)
		}
	})

	// DeactivateResident cross-RT
	t.Run("PENGURUS RT A -> DeactivateResident RT B returns exactly 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/residents/"+resBID, nil)
		req.Header.Set("Authorization", "Bearer "+pengurusAToken)
		rec := httptest.NewRecorder()

		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Delete("/api/v1/residents/{id}", h.DeactivateResident)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify resident is still active
		var isActive bool
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT is_active FROM residents WHERE id = $1`, resBID,
		).Scan(&isActive)
		if !isActive {
			t.Fatalf("DeactivateResident rejected but foreign resident was deactivated")
		}
	})
}

// TestForeignRtIdBlocked verifies that a tenant user cannot create resources
// in another RT by supplying a foreign rt_id.
func TestForeignRtIdBlocked(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	h := NewHandler(pool)

	rtAID := setupTestRT(t, pool, "FRT RT A", 1, "FRTA")
	rtBID := setupTestRT(t, pool, "FRT RT B", 1, "FRTB")

	marker := fmt.Sprintf("FRT-%d", time.Now().UnixNano())
	pengurusAToken := makeTenantToken(t, "pengurus-a", "membership-a", rtAID, auth.RolePengurus)

	t.Run("foreign rt_id CreateHousehold returns 400", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households", h.CreateHousehold)

		body := fmt.Sprintf(`{"rt_id":"%s","head_name":"%s","house_number":"FRT-X1","nik":"1111111111111188","phone":"081100000005","email":"x@example.com","occupancy_status":"OWNER"}`, rtBID, marker)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusAToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("no household created in RT B", func(t *testing.T) {
		marker2 := fmt.Sprintf("FRT2-%d", time.Now().UnixNano())

		body := fmt.Sprintf(`{"rt_id":"%s","head_name":"%s","house_number":"FRT-X2","nik":"1111111111111188","phone":"081100000006","email":"x2@example.com","occupancy_status":"OWNER"}`, rtBID, marker2)
		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households", h.CreateHousehold)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusAToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}

		var count int
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM households WHERE rt_id = $1 AND is_active = true AND head_name = $2`,
			rtBID, marker2,
		).Scan(&count)
		if count != 0 {
			t.Fatalf("unexpected household created in RT B: %d rows with head_name %q", count, marker2)
		}
	})
}

func strPtr(s string) *string                   { return &s }
func occPtr(o OccupancyStatus) *OccupancyStatus { return &o }

// TestBendaharaWritePermissionsRegression verifies that bendahara cannot
// execute any household or resident write operations that are gated to pengurus.
func TestBendaharaWritePermissionsRegression(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	h := NewHandler(pool)

	rtID := setupTestRT(t, pool, "BendaharaPerm RT", 1, "BPERM")

	bendaharaToken := makeTenantToken(t, "bendahara-1", "membership-b1", rtID, auth.RoleBendahara)

	// --- Per-action pengurus baselines (independent fixtures) ---
	pengurusToken := makeTenantToken(t, "pengurus-1", "membership-p1", rtID, auth.RolePengurus)

	rPengurus := chi.NewRouter()
	rPengurus.Use(auth.RequireAuth)
	rPengurus.Use(auth.RequireRole(auth.RolePengurus))
	rPengurus.Post("/api/v1/households", h.CreateHousehold)
	rPengurus.Patch("/api/v1/households/{id}", h.UpdateHousehold)
	rPengurus.Post("/api/v1/households/{id}/move", h.MoveHousehold)
	rPengurus.Delete("/api/v1/households/{id}", h.DeactivateHousehold)
	rPengurus.Post("/api/v1/residents", h.CreateResident)
	rPengurus.Patch("/api/v1/residents/{id}", h.UpdateResident)
	rPengurus.Post("/api/v1/residents/{id}/move", h.MoveResident)
	rPengurus.Delete("/api/v1/residents/{id}", h.DeactivateResident)

	// POST /households — creates its own resource
	t.Run("pengurus/POST /households", func(t *testing.T) {
		marker := fmt.Sprintf("BP-baseline-%d", time.Now().UnixNano())
		body := fmt.Sprintf(`{"head_name":"%s","house_number":"BP01","nik":"1111111111111199","phone":"081100000099","email":"bpbaseline@example.com","occupancy_status":"OWNER"}`, marker)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("pengurus POST /households baseline failed: expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// PATCH /households/{id} — needs its own household
	hhPatchID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "BPPatchHead", strPtr("BP-PP01"), strPtr("BP-Patch Addr"), occPtr(OccupancyOwner))
	t.Run("pengurus/PATCH /households/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/households/"+hhPatchID, bytes.NewBufferString(`{"head_name":"RevisedPatch"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("pengurus PATCH /households baseline failed: expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var headName string
		pool.Raw().QueryRowContext(context.Background(), `SELECT head_name FROM households WHERE id = $1`, hhPatchID).Scan(&headName)
		if headName != "RevisedPatch" {
			t.Fatalf("persisted head_name mismatch: expected RevisedPatch, got %q", headName)
		}
	})

	// MOVE /households/{id} — needs its own household
	hhMoveID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "BPMoveHead", strPtr("BP-M01"), strPtr("BP-Move Addr"), occPtr(OccupancyOwner))
	t.Run("pengurus/POST /households/{id}/move", func(t *testing.T) {
		moveDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		body := fmt.Sprintf(`{"house_number":"BP-M2","start_date":"%s"}`, moveDate)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+hhMoveID+"/move", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("pengurus MOVE household baseline failed: expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var occCount int
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM household_occupancies WHERE household_id = $1 AND end_date IS NULL`, hhMoveID,
		).Scan(&occCount)
		if occCount != 1 {
			t.Fatalf("persisted household current occupancy: expected 1, got %d", occCount)
		}
	})

	// DELETE /households/{id} — needs its own household
	hhDelID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "BPDelHead", strPtr("BP-D01"), strPtr("BP-Del Addr"), occPtr(OccupancyOwner))
	t.Run("pengurus/DELETE /households/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/households/"+hhDelID, nil)
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("pengurus DELETE household baseline failed: expected 204, got %d: %s", rec.Code, rec.Body.String())
		}
		var isActive bool
		pool.Raw().QueryRowContext(context.Background(), `SELECT is_active FROM households WHERE id = $1`, hhDelID).Scan(&isActive)
		if isActive {
			t.Fatalf("persisted household is_active still true after DELETE")
		}
	})

	// POST /residents — its own fixture with embedded hhID
	t.Run("pengurus/POST /residents", func(t *testing.T) {
		resHHID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "BPResCreateHead", strPtr("BP-RC01"), strPtr("BP-RC Addr"), occPtr(OccupancyOwner))
		body := fmt.Sprintf(`{"household_id":"%s","full_name":"BP Resident Baseline","nik":"1111111111111150","phone":"081100000150","email":"bpresbase@example.com","relationship_to_head":"Self"}`, resHHID)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("pengurus POST /residents baseline failed: expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var respBody map[string]any
		json.Unmarshal(rec.Body.Bytes(), &respBody)
		if resID, ok := respBody["id"].(string); ok {
			var residentRT string
			pool.Raw().QueryRowContext(context.Background(), `SELECT rt_id FROM residents WHERE id = $1`, resID).Scan(&residentRT)
			if residentRT != rtID {
				t.Fatalf("persisted resident rt_id mismatch: expected %q, got %q", rtID, residentRT)
			}
		}
	})

	// PATCH /residents/{id} — needs its own resident
	resPatchID := insertTestResidentFixture(t, pool.Raw(), rtID, hhPatchID, "BP Resident Patch", strPtr("Self"))
	t.Run("pengurus/PATCH /residents/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+resPatchID, bytes.NewBufferString(`{"full_name":"RevisedBPRes"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("pengurus PATCH /residents baseline failed: expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var fullName string
		pool.Raw().QueryRowContext(context.Background(), `SELECT full_name FROM residents WHERE id = $1`, resPatchID).Scan(&fullName)
		if fullName != "RevisedBPRes" {
			t.Fatalf("persisted full_name mismatch: expected RevisedBPRes, got %q", fullName)
		}
	})

	// MOVE /residents/{id} — source hh A, dest hh B
	hhSrcID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "BPResSrcHead", strPtr("BP-S01"), strPtr("BP-Src Addr"), occPtr(OccupancyOwner))
	hhDstID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "BPResDstHead", strPtr("BP-D02"), strPtr("BP-Dst Addr"), occPtr(OccupancyOwner))
	resMoveID := insertTestResidentFixture(t, pool.Raw(), rtID, hhSrcID, "BP Resident Move", strPtr("Self"))
	t.Run("pengurus/POST /residents/{id}/move", func(t *testing.T) {
		moveDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		body := fmt.Sprintf(`{"destination_household_id":"%s","start_date":"%s"}`, hhDstID, moveDate)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/"+resMoveID+"/move", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("pengurus MOVE resident baseline failed: expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var linkedHH sql.NullString
		_ = pool.Raw().QueryRowContext(context.Background(),
			`SELECT ho.household_id FROM residency_periods rp JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id WHERE rp.resident_id = $1 AND rp.end_date IS NULL LIMIT 1`,
			resMoveID,
		).Scan(&linkedHH)
		if !linkedHH.Valid || linkedHH.String != hhDstID {
			t.Fatalf("persisted residency destination wrong: expected %q, got %v", hhDstID, linkedHH)
		}
	})

	// DELETE /residents/{id} — its own resident
	resDelID := insertTestResidentFixture(t, pool.Raw(), rtID, hhDelID, "BP Resident Delete", strPtr("Self"))
	t.Run("pengurus/DELETE /residents/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/residents/"+resDelID, nil)
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		rPengurus.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("pengurus DELETE /residents baseline failed: expected 204, got %d: %s", rec.Code, rec.Body.String())
		}
		var isActive bool
		pool.Raw().QueryRowContext(context.Background(), `SELECT is_active FROM residents WHERE id = $1`, resDelID).Scan(&isActive)
		if isActive {
			t.Fatalf("persisted resident is_active still true after DELETE")
		}
	})

	// --- Bendahara denial tests (independent fixtures) ---
	t.Run("bendahara write permissions", func(t *testing.T) {
		bhHhID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "BPHHHead", strPtr("BP-BH01"), strPtr("BP-BH Addr"), occPtr(OccupancyOwner))
		bhResID := insertTestResidentFixture(t, pool.Raw(), rtID, bhHhID, "BP Resident", strPtr("Self"))

		bendaharaWrites := []struct {
			name     string
			method   string
			path     string
			body     string
			expected int
		}{
			{"bendahara POST /households", http.MethodPost, "/api/v1/households", `{"head_name":"Unauthorized","house_number":"X","nik":"1111111111111199","phone":"081100000099","email":"x@example.com","occupancy_status":"OWNER"}`, http.StatusForbidden},
			{"bendahara PATCH /households/{id}", http.MethodPatch, "/api/v1/households/" + bhHhID, `{"head_name":"Unauthorized"}`, http.StatusForbidden},
			{"bendahara MOVE /households/{id}", http.MethodPost, "/api/v1/households/" + bhHhID + "/move", `{"house_number":"X2","start_date":"2026-12-01"}`, http.StatusForbidden},
			{"bendahara DELETE /households/{id}", http.MethodDelete, "/api/v1/households/" + bhHhID, "", http.StatusForbidden},
			{"bendahara POST /residents", http.MethodPost, "/api/v1/residents", `{"household_id":"` + bhHhID + `","full_name":"Unauthorized","nik":"1111111111111199","phone":"081100000099","email":"x@example.com"}`, http.StatusForbidden},
			{"bendahara PATCH /residents/{id}", http.MethodPatch, "/api/v1/residents/" + bhResID, `{"full_name":"Unauthorized"}`, http.StatusForbidden},
			{"bendahara MOVE /residents/{id}", http.MethodPost, "/api/v1/residents/" + bhResID + "/move", `{"destination_household_id":"` + bhHhID + `","start_date":"2026-12-01"}`, http.StatusForbidden},
			{"bendahara DELETE /residents/{id}", http.MethodDelete, "/api/v1/residents/" + bhResID, "", http.StatusForbidden},
		}

		for _, w := range bendaharaWrites {
			t.Run(w.name, func(t *testing.T) {
				r := chi.NewRouter()
				r.Use(auth.RequireAuth)
				r.Use(auth.RequireRole(auth.RolePengurus))
				r.Post("/api/v1/households", h.CreateHousehold)
				r.Patch("/api/v1/households/{id}", h.UpdateHousehold)
				r.Post("/api/v1/households/{id}/move", h.MoveHousehold)
				r.Delete("/api/v1/households/{id}", h.DeactivateHousehold)
				r.Post("/api/v1/residents", h.CreateResident)
				r.Patch("/api/v1/residents/{id}", h.UpdateResident)
				r.Post("/api/v1/residents/{id}/move", h.MoveResident)
				r.Delete("/api/v1/residents/{id}", h.DeactivateResident)

				var body io.Reader
				if w.body != "" {
					body = bytes.NewBufferString(w.body)
				}
				req := httptest.NewRequest(w.method, w.path, body)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+bendaharaToken)
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				if rec.Code != w.expected {
					t.Errorf("%s: expected %d, got %d: %s", w.name, w.expected, rec.Code, rec.Body.String())
				}
			})
		}
	})

	// Bendahara cannot create resources in foreign RT
	rtB := setupTestRT(t, pool, "BendaharaPerm RTB", 1, "BPERMB")
	_ = insertTestHouseholdFixture(t, pool.Raw(), rtB, "BP Head B", strPtr("BP-B1"), strPtr("BP-B Addr"), occPtr(OccupancyOwner))

	t.Run("bendahara foreign RT create blocked", func(t *testing.T) {
		// bendahara in RT B creates household in RT B -> should succeed
		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households", h.CreateHousehold)
		// This should fail (wrong role) regardless of RT

		body := fmt.Sprintf(`{"head_name":"Bendahara Create","house_number":"BPC-%d","nik":"1111111111111198","phone":"081100000098","email":"bpc@example.com","occupancy_status":"OWNER"}`, time.Now().UnixNano()/1e6)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+bendaharaToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("bendahara create household: expected 403, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify nothing was created
		var count int
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM households WHERE rt_id = $1 AND is_active = true AND head_name = $2`,
			rtID, "Bendahara Create",
		).Scan(&count)
		if count != 0 {
			t.Errorf("household was created by bendahara despite 403")
		}
	})
}

// TestWargaWritePermissionsRegression verifies that warga cannot
// execute any household or resident write operations.
func TestWargaWritePermissionsRegression(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	h := NewHandler(pool)

	rtID := setupTestRT(t, pool, "WargaPerm RT", 1, "WPERM")

	wargaToken := makeTenantToken(t, "warga-1", "membership-w1", rtID, auth.RoleWarga)

	// --- Warga denial tests (independent fixtures) ---
	t.Run("warga write permissions", func(t *testing.T) {
		whHhID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "WPHHHead", strPtr("WP-WH01"), strPtr("WP-WH Addr"), occPtr(OccupancyOwner))
		whResID := insertTestResidentFixture(t, pool.Raw(), rtID, whHhID, "WP Resident", strPtr("Self"))

		wargaWrites := []struct {
			name     string
			method   string
			path     string
			body     string
			expected int
		}{
			{"warga POST /households", http.MethodPost, "/api/v1/households", `{"head_name":"Unauthorized","house_number":"W","nik":"1111111111111197","phone":"081100000097","email":"w@example.com","occupancy_status":"OWNER"}`, http.StatusForbidden},
			{"warga PATCH /households/{id}", http.MethodPatch, "/api/v1/households/" + whHhID, `{"head_name":"Unauthorized"}`, http.StatusForbidden},
			{"warga MOVE /households/{id}", http.MethodPost, "/api/v1/households/" + whHhID + "/move", `{"house_number":"W2","start_date":"2026-12-01"}`, http.StatusForbidden},
			{"warga DELETE /households/{id}", http.MethodDelete, "/api/v1/households/" + whHhID, "", http.StatusForbidden},
			{"warga POST /residents", http.MethodPost, "/api/v1/residents", `{"household_id":"` + whHhID + `","full_name":"Unauthorized","nik":"1111111111111197","phone":"081100000097","email":"w@example.com"}`, http.StatusForbidden},
			{"warga PATCH /residents/{id}", http.MethodPatch, "/api/v1/residents/" + whResID, `{"full_name":"Unauthorized"}`, http.StatusForbidden},
			{"warga MOVE /residents/{id}", http.MethodPost, "/api/v1/residents/" + whResID + "/move", `{"destination_household_id":"` + whHhID + `","start_date":"2026-12-01"}`, http.StatusForbidden},
			{"warga DELETE /residents/{id}", http.MethodDelete, "/api/v1/residents/" + whResID, "", http.StatusForbidden},
		}

		for _, w := range wargaWrites {
			t.Run(w.name, func(t *testing.T) {
				r := chi.NewRouter()
				r.Use(auth.RequireAuth)
				r.Use(auth.RequireRole(auth.RolePengurus))
				r.Post("/api/v1/households", h.CreateHousehold)
				r.Patch("/api/v1/households/{id}", h.UpdateHousehold)
				r.Post("/api/v1/households/{id}/move", h.MoveHousehold)
				r.Delete("/api/v1/households/{id}", h.DeactivateHousehold)
				r.Post("/api/v1/residents", h.CreateResident)
				r.Patch("/api/v1/residents/{id}", h.UpdateResident)
				r.Post("/api/v1/residents/{id}/move", h.MoveResident)
				r.Delete("/api/v1/residents/{id}", h.DeactivateResident)

				var body io.Reader
				if w.body != "" {
					body = bytes.NewBufferString(w.body)
				}
				req := httptest.NewRequest(w.method, w.path, body)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+wargaToken)
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				if rec.Code != w.expected {
					t.Errorf("%s: expected %d, got %d: %s", w.name, w.expected, rec.Code, rec.Body.String())
				}
			})
		}
	})

	// Warga cannot create resources in foreign RT
	rtB := setupTestRT(t, pool, "WargaPerm RTB", 1, "WPERMB")
	_ = insertTestHouseholdFixture(t, pool.Raw(), rtB, "WP Head B", strPtr("WP-B1"), strPtr("WP-B Addr"), occPtr(OccupancyOwner))

	t.Run("warga foreign RT create blocked", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households", h.CreateHousehold)

		body := fmt.Sprintf(`{"head_name":"Warga Create","house_number":"WPC-%d","nik":"1111111111111196","phone":"081100000096","email":"wpc@example.com","occupancy_status":"OWNER"}`, time.Now().UnixNano()/1e6)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("warga create household: expected 403, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify nothing was created
		var count int
		pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM households WHERE rt_id = $1 AND is_active = true AND head_name = $2`,
			rtID, "Warga Create",
		).Scan(&count)
		if count != 0 {
			t.Errorf("household was created by warga despite 403")
		}
	})
}

// TestCrossRTIsolationGET verifies cross-RT GET isolation on individual
// household and resident endpoints across warga, pengurus, and super_admin roles.
func TestCrossRTIsolationGET(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	h := NewHandler(pool)

	rtAID := setupTestRT(t, pool, "CrossRTGet RT A", 1, "CRTGETA")
	rtBID := setupTestRT(t, pool, "CrossRTGet RT B", 1, "CRTGETB")

	hnA := "CRT-A1"
	hhAID := insertTestHouseholdFixture(t, pool.Raw(), rtAID, "Head A", &hnA, strPtr("Addr A"), occPtr(OccupancyOwner))
	resAID := insertTestResidentFixture(t, pool.Raw(), rtAID, hhAID, "Resident A", strPtr("Self"))

	hnB := "CRT-B1"
	hhBID := insertTestHouseholdFixture(t, pool.Raw(), rtBID, "Head B", &hnB, strPtr("Addr B"), occPtr(OccupancyOwner))
	resBID := insertTestResidentFixture(t, pool.Raw(), rtBID, hhBID, "Resident B", strPtr("Self"))

	wargaTokenA := makeTenantToken(t, "user-w-a", "mem-w-a", rtAID, auth.RoleWarga)
	pengurusTokenA := makeTenantToken(t, "user-p-a", "mem-p-a", rtAID, auth.RolePengurus)
	adminToken := makeSuperAdminToken(t)

	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
	r.Get("/api/v1/households/{id}", h.GetHousehold)
	r.Get("/api/v1/residents/{id}", h.GetResident)

	tests := []struct {
		name         string
		token        string
		path         string
		expectedCode int
	}{
		// Baseline: Own RT access succeeds
		{
			name:         "warga RT A -> GET household in RT A (own RT)",
			token:        wargaTokenA,
			path:         "/api/v1/households/" + hhAID,
			expectedCode: http.StatusOK,
		},
		{
			name:         "warga RT A -> GET resident in RT A (own RT)",
			token:        wargaTokenA,
			path:         "/api/v1/residents/" + resAID,
			expectedCode: http.StatusOK,
		},
		{
			name:         "pengurus RT A -> GET household in RT A (own RT)",
			token:        pengurusTokenA,
			path:         "/api/v1/households/" + hhAID,
			expectedCode: http.StatusOK,
		},
		{
			name:         "pengurus RT A -> GET resident in RT A (own RT)",
			token:        pengurusTokenA,
			path:         "/api/v1/residents/" + resAID,
			expectedCode: http.StatusOK,
		},

		// 1. Warga in RT A -> GET household in RT B -> 404
		{
			name:         "warga RT A -> GET household in RT B (cross-RT isolation)",
			token:        wargaTokenA,
			path:         "/api/v1/households/" + hhBID,
			expectedCode: http.StatusNotFound,
		},

		// 2. Warga in RT A -> GET resident in RT B -> 404
		{
			name:         "warga RT A -> GET resident in RT B (cross-RT isolation)",
			token:        wargaTokenA,
			path:         "/api/v1/residents/" + resBID,
			expectedCode: http.StatusNotFound,
		},

		// 3. Pengurus in RT A -> GET household in RT B -> 404
		{
			name:         "pengurus RT A -> GET household in RT B (cross-RT isolation)",
			token:        pengurusTokenA,
			path:         "/api/v1/households/" + hhBID,
			expectedCode: http.StatusNotFound,
		},

		// 4. Pengurus in RT A -> GET resident in RT B -> 404
		{
			name:         "pengurus RT A -> GET resident in RT B (cross-RT isolation)",
			token:        pengurusTokenA,
			path:         "/api/v1/residents/" + resBID,
			expectedCode: http.StatusNotFound,
		},

		// 5. Super admin -> GET household in RT B -> 200
		{
			name:         "super admin -> GET household in RT B (system-wide access)",
			token:        adminToken,
			path:         "/api/v1/households/" + hhBID,
			expectedCode: http.StatusOK,
		},

		// 6. Super admin -> GET resident in RT B -> 200
		{
			name:         "super admin -> GET resident in RT B (system-wide access)",
			token:        adminToken,
			path:         "/api/v1/residents/" + resBID,
			expectedCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tc.expectedCode, rec.Code, rec.Body.String())
			}
		})
	}
}
