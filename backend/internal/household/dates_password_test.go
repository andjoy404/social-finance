package household

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
)

func TestHouseholdDatesAndResidentPassword(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	rtID := setupTestRT(t, pool, "Dates RT", 1, "09")
	token := makeTenantToken(t, "user-dates-test", "mid-dates-test", rtID, auth.RolePengurus)

	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireRole(auth.RolePengurus))
	r.Post("/api/v1/households", h.CreateHousehold)
	r.Patch("/api/v1/households/{id}", h.UpdateHousehold)
	r.Get("/api/v1/households/{id}", h.GetHousehold)
	r.Post("/api/v1/residents", h.CreateResident)
	r.Patch("/api/v1/residents/{id}", h.UpdateResident)
	r.Get("/api/v1/residents/{id}", h.GetResident)

	suffix := time.Now().UnixNano()
	testEmail := fmt.Sprintf("budi-%d@example.com", suffix)
	testNik := fmt.Sprintf("320101%010d", suffix%10000000000)
	testPhone := fmt.Sprintf("081234569%05d", suffix%100000)

	// 1. Validation: end_date before start_date fails
	bodyInvalid := map[string]interface{}{
		"house_number":     "A-10",
		"head_name":        "Budi Santoso",
		"nik":              testNik,
		"phone":            testPhone,
		"email":            testEmail,
		"occupancy_status": "OWNER",
		"start_date":       "2026-06-01",
		"end_date":         "2026-05-01",
	}
	buf, _ := json.Marshal(bodyInvalid)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for end_date before start_date, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Success: Create household with start_date, end_date, and password
	bodyValid := map[string]interface{}{
		"house_number":     "A-10",
		"head_name":        "Budi Santoso",
		"nik":              testNik,
		"phone":            testPhone,
		"email":            testEmail,
		"occupancy_status": "OWNER",
		"start_date":       "2026-06-01",
		"end_date":         "2027-06-01",
		"password":         "Password123!",
	}
	buf, _ = json.Marshal(bodyValid)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var createdHH struct {
		ID        string  `json:"id"`
		StartDate *string `json:"start_date"`
		EndDate   *string `json:"end_date"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &createdHH)
	if createdHH.StartDate == nil || *createdHH.StartDate != "2026-06-01" {
		t.Fatalf("expected start_date 2026-06-01, got %v", createdHH.StartDate)
	}
	if createdHH.EndDate == nil || *createdHH.EndDate != "2027-06-01" {
		t.Fatalf("expected end_date 2027-06-01, got %v", createdHH.EndDate)
	}

	// Verify user account was provisioned with hashed password
	var passHash string
	err := pool.Raw().QueryRow(`SELECT password_hash FROM users WHERE email = $1`, testEmail).Scan(&passHash)
	if err != nil {
		t.Fatalf("expected user account created for %s, got %v", testEmail, err)
	}
	if passHash == "Password123!" || auth.Verify("Password123!", passHash) != nil {
		t.Fatalf("expected password hash to verify against Password123!")
	}

	// 3. Update household end_date to new value
	bodyUpdate := map[string]interface{}{
		"end_date": "2028-01-01",
		"password": "NewSecretPassword456!",
	}
	buf, _ = json.Marshal(bodyUpdate)
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/households/%s", createdHH.ID), bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on update, got %d: %s", rec.Code, rec.Body.String())
	}

	// Get household to verify reload
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/households/%s", createdHH.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var reloadedHH struct {
		StartDate *string `json:"start_date"`
		EndDate   *string `json:"end_date"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &reloadedHH)
	if reloadedHH.EndDate == nil || *reloadedHH.EndDate != "2028-01-01" {
		t.Fatalf("expected reloaded end_date 2028-01-01, got %v", reloadedHH.EndDate)
	}

	// Verify updated password hash
	_ = pool.Raw().QueryRow(`SELECT password_hash FROM users WHERE email = $1`, testEmail).Scan(&passHash)
	if auth.Verify("NewSecretPassword456!", passHash) != nil {
		t.Fatalf("expected updated password hash to verify against NewSecretPassword456!")
	}
}

// TestSyncResidentUserAccountEmailConflict verifies that creating a second
// resident with an email already linked to another resident returns an error
// instead of silently skipping or replacing the existing link.
func TestSyncResidentUserAccountEmailConflict(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	rtID := setupTestRT(t, pool, "Conflict RT", 1, "09")
	token := makeTenantToken(t, "user-conflict", "mid-conflict", rtID, auth.RolePengurus)

	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireRole(auth.RolePengurus))
	r.Post("/api/v1/households", h.CreateHousehold)

	suffix := time.Now().UnixNano()
	testEmail := fmt.Sprintf("conflict-%d@example.com", suffix)
	testPhone := fmt.Sprintf("081234568%05d", suffix%100000)
	testNik := fmt.Sprintf("320102%010d", suffix%10000000000)

	// Create first household
	body1 := map[string]interface{}{
		"house_number":     "C-1",
		"head_name":        "Resident One",
		"nik":              testNik,
		"phone":            testPhone,
		"email":            testEmail,
		"occupancy_status": "OWNER",
		"start_date":       "2026-01-01",
		"password":         "Password123!",
	}
	buf, _ := json.Marshal(body1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first household, got %d: %s", rec.Code, rec.Body.String())
	}

	// Try second household with same email — must fail with conflict
	testNik2 := fmt.Sprintf("320103%010d", suffix%10000000000)
	body2 := map[string]interface{}{
		"house_number":     "C-2",
		"head_name":        "Resident Two",
		"nik":              testNik2,
		"phone":            fmt.Sprintf("081234568%05d", (suffix+1)%100000),
		"email":            testEmail,
		"occupancy_status": "TENANT",
		"start_date":       "2026-01-01",
		"password":         "Password123!",
	}
	buf2, _ := json.Marshal(body2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewReader(buf2))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusCreated {
		t.Fatalf("expected non-201 for duplicate email-linked resident, got 201")
	}
}
