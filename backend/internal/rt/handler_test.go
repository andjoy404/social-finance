package rt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpx "social-finance/internal/http"
	"social-finance/internal/testutil"
)

const (
	testSigningKey = "test-signing-secret-at-least-16-chars"
)

// superAdminToken creates a signed JWT token for a super_admin user.
func superAdminToken(t *testing.T) string {
	t.Helper()
	auth.SigningSecret = []byte(testSigningKey)
	claims := auth.TokenClaims{
		UserID:  "test-user-uuid-12345",
		SysRole: "super_admin",
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

// regularToken creates a signed JWT token for a non-super-admin user.
func regularToken(t *testing.T) string {
	t.Helper()
	auth.SigningSecret = []byte(testSigningKey)
	claims := auth.TokenClaims{
		UserID: "test-user-uuid-67890",
		Role:   "warga",
		RTID:   "test-rt-uuid-12345",
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func testPool(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}

// insertRT inserts a test RT into the database and returns its ID.
func insertRT(t *testing.T, pool *database.Pool, name string, rw int, rt string) string {
	t.Helper()
	db := pool.Raw()

	// Pre-test stale cleanup: remove fixtures from a previous interrupted run
	db.Exec("DELETE FROM rts WHERE name = $1", name)

	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, $3, $4, $5, true)
		 RETURNING id`,
		name, rw, rt, "Test Address", "Test Head",
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test RT: %v", err)
	}

	t.Cleanup(func() {
		db.Exec("DELETE FROM rts WHERE id = $1", id)
	})

	return id
}

// buildRouter creates a chi router with RT routes.
func buildRouter(pool *database.Pool) *chi.Mux {
	svc := NewService()
	h := NewHandler(svc, pool)

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.PanicRecovery)
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))

	r.Post("/api/v1/rts", h.Create)
	r.Get("/api/v1/rts/{id}", h.GetByID)
	r.Get("/api/v1/rts", h.ListAll)
	r.Patch("/api/v1/rts/{id}", h.Update)
	r.Patch("/api/v1/rts/{id}/deactivate", h.Deactivate)

	return r
}

func setupEnv() {
	auth.SigningSecret = []byte(testSigningKey)
}

func TestCreateRT(t *testing.T) {
	pool := testPool(t)

	r := buildRouter(pool)

	setupEnv()
	token := superAdminToken(t)

	// Pre-test stale cleanup: tests via HTTP POST, not insertRT
	pool.Raw().Exec("DELETE FROM rts WHERE name LIKE 'CreateTest RT%'")

	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rts WHERE name LIKE 'CreateTest RT%'")
		pool.Close()
	})

	t.Run("valid input", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"CreateTest RT01","rw":11,"rt":"C01"}`)
		req := httptest.NewRequest("POST", "/api/v1/rts", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var result RT
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if result.Name != "CreateTest RT01" {
			t.Errorf("expected name 'RT 01', got %q", result.Name)
		}
		if result.RW != 11 {
			t.Errorf("expected rw 1, got %d", result.RW)
		}
		if result.RT != "C01" {
			t.Errorf("expected rt '01', got %q", result.RT)
		}
		if result.ID == "" {
			t.Error("expected non-empty ID")
		}
		if !result.IsActive {
			t.Error("expected is_active to be true")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		body := bytes.NewBufferString(`{"rw":11,"rt":"C02"}`)
		req := httptest.NewRequest("POST", "/api/v1/rts", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing rt", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"CreateTest RT02","rw":11}`)
		req := httptest.NewRequest("POST", "/api/v1/rts", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestListRTs(t *testing.T) {
	pool := testPool(t)

	r := buildRouter(pool)

	setupEnv()
	token := superAdminToken(t)

	_ = insertRT(t, pool, "ListTest RT 1", 10, "L01")
	_ = insertRT(t, pool, "ListTest RT 2", 20, "L02")

	t.Cleanup(func() { pool.Close() })

	req := httptest.NewRequest("GET", "/api/v1/rts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("expected 'data' to be an array, got %T", resp["data"])
	}
	if len(data) < 2 {
		t.Errorf("expected at least 2 RTs, got %d", len(data))
	}

	pagination, ok := resp["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'pagination' to be an object, got %T", resp["pagination"])
	}
	if pagination["page"] != float64(1) {
		t.Errorf("expected page 1, got %v", pagination["page"])
	}
	if pagination["page_size"] != float64(20) {
		t.Errorf("expected page_size 20, got %v", pagination["page_size"])
	}
}

func TestListRTsPagination(t *testing.T) {
	pool := testPool(t)

	r := buildRouter(pool)

	setupEnv()
	token := superAdminToken(t)

	for i := 1; i <= 5; i++ {
		_ = insertRT(t, pool, fmt.Sprintf("PagTest RT %d", i), 1, fmt.Sprintf("Pg%02d", i))
	}

	t.Cleanup(func() { pool.Close() })

	// page=1, page_size=2
	req := httptest.NewRequest("GET", "/api/v1/rts?page=1&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	pagination := resp["pagination"].(map[string]any)
	if pagination["page_size"] != float64(2) {
		t.Errorf("expected page_size 2, got %v", pagination["page_size"])
	}
	data := resp["data"].([]any)
	if len(data) != 2 {
		t.Errorf("expected 2 items on page 1, got %d", len(data))
	}

	// page=2, page_size=2
	req2 := httptest.NewRequest("GET", "/api/v1/rts?page=2&page_size=2", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp2 map[string]any
	json.Unmarshal(rec2.Body.Bytes(), &resp2)
	data2 := resp2["data"].([]any)
	if len(data2) != 2 {
		t.Errorf("expected 2 items on page 2, got %d", len(data2))
	}
}

func TestGetByID(t *testing.T) {
	pool := testPool(t)

	r := buildRouter(pool)

	setupEnv()
	token := superAdminToken(t)

	rtID := insertRT(t, pool, "GetByIDTest RT", 1, "GB99")

	t.Cleanup(func() { pool.Close() })

	t.Run("found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/rts/"+rtID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var result RT
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.ID != rtID {
			t.Errorf("expected id %q, got %q", rtID, result.ID)
		}
		if result.Name != "GetByIDTest RT" {
			t.Errorf("expected name 'GetByIDTest RT', got %q", result.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/rts/00000000-0000-0000-0000-000000000000", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestUpdateRT(t *testing.T) {
	pool := testPool(t)

	r := buildRouter(pool)

	setupEnv()
	token := superAdminToken(t)

	rtID := insertRT(t, pool, "UpdateTest RT", 1, "UT88")

	t.Cleanup(func() { pool.Close() })

	t.Run("partial update", func(t *testing.T) {
		body := bytes.NewBufferString(`{"rw":5}`)
		req := httptest.NewRequest("PATCH", "/api/v1/rts/"+rtID, body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var result RT
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.RW != 5 {
			t.Errorf("expected rw 5, got %d", result.RW)
		}
		if result.Name != "UpdateTest RT" {
			t.Errorf("expected name to stay unchanged, got %q", result.Name)
		}
	})

	t.Run("update not found", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"Ghost RT"}`)
		req := httptest.NewRequest("PATCH", "/api/v1/rts/00000000-0000-0000-0000-000000000000", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestDeactivateRT(t *testing.T) {
	pool := testPool(t)

	r := buildRouter(pool)

	setupEnv()
	token := superAdminToken(t)

	rtID := insertRT(t, pool, "DeactivTest RT", 1, "DT77")

	t.Cleanup(func() { pool.Close() })

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/api/v1/rts/"+rtID+"/deactivate", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify the RT is now inactive
		var isActive bool
		err := pool.Raw().QueryRow("SELECT is_active FROM rts WHERE id = $1", rtID).Scan(&isActive)
		if err != nil {
			t.Fatalf("failed to query RT: %v", err)
		}
		if isActive {
			t.Error("expected is_active to be false after deactivation")
		}
	})

	t.Run("deactivate not found", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/api/v1/rts/00000000-0000-0000-0000-000000000000/deactivate", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("double deactivate returns 404", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/api/v1/rts/"+rtID+"/deactivate", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 (already deactivated), got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestUnauthorized(t *testing.T) {
	pool := testPool(t)

	r := buildRouter(pool)

	t.Cleanup(func() { pool.Close() })

	req := httptest.NewRequest("POST", "/api/v1/rts", bytes.NewBufferString(`{"name":"No Auth","rw":1,"rt":"00"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth token, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test with invalid token
	req2 := httptest.NewRequest("POST", "/api/v1/rts", bytes.NewBufferString(`{"name":"Bad Token","rw":1,"rt":"00"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer invalid-token-here")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d: %s", rec2.Code, rec2.Body.String())
	}
}
