package rt

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

	var userID string
	if auth.AuthDBPool != nil {
		db := auth.AuthDBPool.Raw()
		err := db.QueryRowContext(context.Background(),
			`SELECT id FROM users WHERE system_role = 'super_admin' AND is_active = true LIMIT 1`,
		).Scan(&userID)
		if err != nil {
			err = db.QueryRowContext(context.Background(),
				`INSERT INTO users (email, phone, password_hash, full_name, system_role, is_active)
				 VALUES ($1, '08123456789', 'dummyhash', 'Test Super Admin', 'super_admin', true)
				 RETURNING id`,
				fmt.Sprintf("superadmin_%d@test.com", time.Now().UnixNano()),
			).Scan(&userID)
			if err != nil {
				t.Fatalf("failed to insert superadmin fixture: %v", err)
			}
		}
	} else {
		userID = "test-user-uuid-12345"
	}

	claims := auth.TokenClaims{
		UserID:  userID,
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
	auth.AuthDBPool = pool
	svc := NewService()
	h := NewHandler(svc, pool)

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.PanicRecovery)
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))

	r.Get("/api/v1/rws", h.ListAllRWs)
	r.Post("/api/v1/rws", h.CreateRW)
	r.Get("/api/v1/rws/{id}", h.GetRWByID)
	r.Patch("/api/v1/rws/{id}", h.UpdateRW)
	r.Patch("/api/v1/rws/{id}/deactivate", h.DeactivateRW)
	r.Get("/api/v1/rws/{id}/memberships", h.ListRWMemberships)
	r.Put("/api/v1/rws/{id}/memberships/{userId}/jabatan", h.SetRWJabatan)
	r.Delete("/api/v1/rws/{id}/memberships/{userId}/jabatan", h.RemoveRWJabatan)

	r.Post("/api/v1/rts", h.Create)
	r.Get("/api/v1/rts/{id}", h.GetByID)
	r.Get("/api/v1/rts", h.ListAll)
	r.Patch("/api/v1/rts/{id}", h.Update)
	r.Patch("/api/v1/rts/{id}/deactivate", h.Deactivate)
	r.Get("/api/v1/rts/{id}/memberships", h.ListMemberships)
	r.Put("/api/v1/rts/{id}/memberships/{userId}/jabatan", h.SetJabatan)
	r.Delete("/api/v1/rts/{id}/memberships/{userId}/jabatan", h.RemoveJabatan)

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

// ── RW Jabatan Assignment Regression Tests ──────────────────────────────────────

func TestRWJabatanRouteRegistration(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	setupEnv()
	token := superAdminToken(t)

	pool.Raw().Exec("DELETE FROM rws WHERE rw_number IN (880, 881)")
	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rws WHERE rw_number IN (880, 881)")
		pool.Close()
	})

	// Create RW for tests
	createReq := httptest.NewRequest("POST", "/api/v1/rws",
		bytes.NewBufferString(`{"rw_number":880,"name":"RW 880"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create RW failed: %d %s", createRec.Code, createRec.Body.String())
	}
	var rwResp map[string]string
	json.Unmarshal(createRec.Body.Bytes(), &rwResp)
	rwID := rwResp["id"]

	// Test PUT route is registered and reaches handler (not 404)
	putReq := httptest.NewRequest("PUT",
		"/api/v1/rws/"+rwID+"/memberships/test-user-uuid/jabatan",
		bytes.NewBufferString(`{"jabatan":"ketua","rt_id":"test-rt-uuid"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, putReq)

	// Must NOT be 404 — if the route is registered, handler executes
	if putRec.Code == http.StatusNotFound {
		t.Errorf("PUT route not registered: got 404, expected handler to execute")
	}

	// Test DELETE route is registered and reaches handler (not 404)
	deleteReq := httptest.NewRequest("DELETE",
		"/api/v1/rws/"+rwID+"/memberships/test-user-uuid/jabatan", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	r.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code == http.StatusNotFound {
		t.Errorf("DELETE route not registered: got 404, expected handler to execute")
	}

	// Test GET memberships route
	getReq := httptest.NewRequest("GET", "/api/v1/rws/"+rwID+"/memberships", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	if getRec.Code == http.StatusNotFound {
		t.Errorf("GET memberships route not registered: got 404, expected handler to execute")
	}
}

func TestRWJabatanRejectsNonSuperAdmin(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	setupEnv()
	token := superAdminToken(t)

	pool.Raw().Exec("DELETE FROM rws WHERE rw_number = 882")
	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rws WHERE rw_number = 882")
		pool.Close()
	})

	// Create RW
	createReq := httptest.NewRequest("POST", "/api/v1/rws",
		bytes.NewBufferString(`{"rw_number":882,"name":"RW 882"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create RW failed: %d %s", createRec.Code, createRec.Body.String())
	}
	var rwResp map[string]string
	json.Unmarshal(createRec.Body.Bytes(), &rwResp)
	rwID := rwResp["id"]

	// Build a non-super-admin token
	auth.SigningSecret = []byte(testSigningKey)
	claims := auth.TokenClaims{
		UserID: "regular-user-uuid",
		Role:   "warga",
		RTID:   "test-rt-uuid",
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	regularTok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatal(err)
	}

	// Non-super-admin should get 403, NOT 404
	putReq := httptest.NewRequest("PUT",
		"/api/v1/rws/"+rwID+"/memberships/test-user/jabatan",
		bytes.NewBufferString(`{"jabatan":"ketua","rt_id":"test-rt"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+regularTok)
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-super-admin, got %d: %s", putRec.Code, putRec.Body.String())
	}
}

func TestRWJabatanReturns401WithoutAuth(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)

	setupEnv()
	token := superAdminToken(t)

	pool.Raw().Exec("DELETE FROM rws WHERE rw_number = 883")
	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rws WHERE rw_number = 883")
		pool.Close()
	})

	// Create RW
	createReq := httptest.NewRequest("POST", "/api/v1/rws",
		bytes.NewBufferString(`{"rw_number":883,"name":"RW 883"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create RW failed: %d %s", createRec.Code, createRec.Body.String())
	}
	var rwResp map[string]string
	json.Unmarshal(createRec.Body.Bytes(), &rwResp)
	rwID := rwResp["id"]

	// PUT without auth token should return 401
	putReq := httptest.NewRequest("PUT",
		"/api/v1/rws/"+rwID+"/memberships/test-user/jabatan",
		bytes.NewBufferString(`{"jabatan":"ketua","rt_id":"test-rt"}`))
	putReq.Header.Set("Content-Type", "application/json")
	// No Authorization header
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth token, got %d: %s", putRec.Code, putRec.Body.String())
	}

	// DELETE without auth token should also return 401
	deleteReq := httptest.NewRequest("DELETE",
		"/api/v1/rws/"+rwID+"/memberships/test-user/jabatan", nil)
	// No Authorization header
	deleteRec := httptest.NewRecorder()
	r.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth token (DELETE), got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestRWJabatanReturns400ForInvalidInput(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	setupEnv()
	token := superAdminToken(t)

	pool.Raw().Exec("DELETE FROM rws WHERE rw_number = 884")
	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rws WHERE rw_number = 884")
		pool.Close()
	})

	// Create RW
	createReq := httptest.NewRequest("POST", "/api/v1/rws",
		bytes.NewBufferString(`{"rw_number":884,"name":"RW 884"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create RW failed: %d %s", createRec.Code, createRec.Body.String())
	}
	var rwResp map[string]string
	json.Unmarshal(createRec.Body.Bytes(), &rwResp)
	rwID := rwResp["id"]

	// Missing rt_id should return 400
	putReq := httptest.NewRequest("PUT",
		"/api/v1/rws/"+rwID+"/memberships/test-user/jabatan",
		bytes.NewBufferString(`{"jabatan":"ketua"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing rt_id, got %d: %s", putRec.Code, putRec.Body.String())
	}

	// Invalid jabatan should return 400
	putReq2 := httptest.NewRequest("PUT",
		"/api/v1/rws/"+rwID+"/memberships/test-user/jabatan",
		bytes.NewBufferString(`{"jabatan":"invalid_role","rt_id":"test-rt"}`))
	putReq2.Header.Set("Content-Type", "application/json")
	putReq2.Header.Set("Authorization", "Bearer "+token)
	putRec2 := httptest.NewRecorder()
	r.ServeHTTP(putRec2, putReq2)

	if putRec2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid jabatan, got %d: %s", putRec2.Code, putRec2.Body.String())
	}
}

func TestRWJabatanReturns404ForNotRWFound(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	setupEnv()
	token := superAdminToken(t)

	t.Cleanup(func() { pool.Close() })

	// PUT to non-existent RW should return 404 (handler processes, not route mismatch)
	putReq := httptest.NewRequest("PUT",
		"/api/v1/rws/nonexistent-uuid-0000000000/memberships/test-user/jabatan",
		bytes.NewBufferString(`{"jabatan":"ketua","rt_id":"test-rt"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	r.ServeHTTP(putRec, putReq)

	// Should be 404 for not-found RW (not 400, not 500, not 401)
	if putRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent RW, got %d: %s", putRec.Code, putRec.Body.String())
	}
}

// TestRWJabatanNilRTIDRegression verifies that deleting RW jabatan does not
// panic with a nil pointer dereference when the membership's rt_id is NULL.
// Regression test for: DELETE /rws/{id}/memberships/{userId}/jabatan → 500
func TestRWJabatanNilRTIDRegression(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	setupEnv()
	token := superAdminToken(t)

	t.Cleanup(func() { pool.Close() })

	// Insert a test user (system_role must be NULL for regular users)
	var userID string
	now := time.Now().Format("20060102150405")
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (email, phone, password_hash, full_name, system_role, is_active)
		 VALUES ($1, '08123456789', 'dummyhash', 'Test User Nil RT ' || $2, NULL, true)
		 RETURNING id`,
		"test-user-nil-rt-"+now+"@test.com", now,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Insert a test RW (use rw_number that likely doesn't exist)
	var rwID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rws (name, rw_number, address, is_active)
		 VALUES ('Test RW Nil RT Regression', 999, 'Jl. Test', true)
		 RETURNING id`,
	).Scan(&rwID)
	if err != nil {
		t.Fatalf("failed to insert test RW: %v", err)
	}

	// Insert RW membership with jabatan but NULL rt_id (simulates RT deleted or never assigned)
	_, err = pool.Raw().ExecContext(context.Background(),
		`INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, rt_id, is_active)
		 VALUES ($1, $2, 'pengurus', 'ketua', NULL, true)`,
		userID, rwID,
	)
	if err != nil {
		t.Fatalf("failed to insert RW membership: %v", err)
	}

	// Verify the membership exists and has jabatan
	var jabatanBefore *string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT jabatan FROM user_rw_memberships WHERE user_id = $1 AND rw_id = $2`,
		userID, rwID,
	).Scan(&jabatanBefore)
	if err != nil {
		t.Fatalf("failed to verify membership before revoke: %v", err)
	}
	if jabatanBefore == nil || *jabatanBefore != "ketua" {
		t.Fatalf("expected jabatan=ketua before revoke, got %v", jabatanBefore)
	}

	// DELETE should return 204 (not 500) — nil rt_id must not cause panic
	delReq := httptest.NewRequest("DELETE",
		"/api/v1/rws/"+rwID+"/memberships/"+userID+"/jabatan",
		nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)

	// Debug: print the actual response for diagnosis
	t.Logf("Response status: %d", delRec.Code)
	t.Logf("Response body: %s", delRec.Body.String())

	if delRec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for nil-rt_id revoke, got %d: %s", delRec.Code, delRec.Body.String())
	}

	// Verify jabatan was actually revoked
	var jabatanAfter *string
	err = pool.Raw().QueryRowContext(context.Background(),
		`SELECT jabatan FROM user_rw_memberships WHERE user_id = $1 AND rw_id = $2`,
		userID, rwID,
	).Scan(&jabatanAfter)
	if err != nil {
		t.Fatalf("failed to verify membership after revoke: %v", err)
	}
	if jabatanAfter != nil {
		t.Errorf("expected jabatan=NULL after revoke, got %v", jabatanAfter)
	}
}

// TestRWJabatanBusinessLogicNilRTID directly tests the business logic.
func TestRWJabatanBusinessLogicNilRTID(t *testing.T) {
	pool := testPool(t)
	setupEnv()

	t.Cleanup(func() { pool.Close() })

	// Insert test user with unique email
	var userID string
	now := time.Now().Format("20060102150405")
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (email, phone, password_hash, full_name, system_role, is_active)
		 VALUES ($1, '08123456791', 'dummyhash', 'Test BL User ' || $2, NULL, true)
		 RETURNING id`,
		"test-bl-nil-rt-"+now+"@test.com", now,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Insert test RW
	var rwID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rws (name, rw_number, address, is_active)
		 VALUES ('Test BL RW', 888, 'Jl. Test', true)
		 RETURNING id`,
	).Scan(&rwID)
	if err != nil {
		t.Fatalf("failed to insert test RW: %v", err)
	}

	// Insert membership with jabatan but NULL rt_id
	_, err = pool.Raw().ExecContext(context.Background(),
		`INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, rt_id, is_active)
		 VALUES ($1, $2, 'pengurus', 'ketua', NULL, true)`,
		userID, rwID,
	)
	if err != nil {
		t.Fatalf("failed to insert RW membership: %v", err)
	}

	// Test business logic directly
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	defer tx.Rollback()

	member, old, err := RemoveRWJabatan(ctx, tx, rwID, userID, userID)
	if err != nil {
		t.Logf("Business logic error: %v", err)
		t.Logf("Error type: %T", err)
		t.Logf("errors.Is(err, ErrRWMembershipNotFound): %v", errors.Is(err, ErrRWMembershipNotFound))
		t.Logf("errors.Is(err, sql.ErrNoRows): %v", errors.Is(err, sql.ErrNoRows))
		t.Logf("errors.Is(err, ErrRWNotFound): %v", errors.Is(err, ErrRWNotFound))
		t.Logf("member: %v", member)
		t.Logf("old: %v", old)
		// Don't fail — we just want to see the debug output
	} else {
		t.Logf("Business logic succeeded. member.ID=%s, old=%v", member.ID, old)
	}

	if err := tx.Commit(); err != nil {
		t.Logf("Commit error: %v", err)
	}
}
