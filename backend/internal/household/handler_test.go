package household

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
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

var rtCounter int64 = time.Now().UnixNano() % 100000

func testPool(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func signingSecret() {
	auth.SigningSecret = []byte(testSigningKey)
}

func makeToken(t *testing.T, claims auth.TokenClaims) string {
	t.Helper()
	signingSecret()
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func makeTenantToken(t *testing.T, userID, membershipID, rtID string, role auth.Role) string {
	return makeToken(t, auth.TokenClaims{
		UserID: userID,
		MID:    membershipID,
		RTID:   rtID,
		Role:   string(role),
	})
}

func makeSuperAdminToken(t *testing.T) string {
	return makeToken(t, auth.TokenClaims{
		UserID:  "super-admin-user-uuid",
		SysRole: "super_admin",
	})
}

func setUpRouter(pool *database.Pool) http.Handler {
	h := NewHandler(pool)

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.PanicRecovery)

	// Write access: pengurus only
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RolePengurus))
		r.Post("/api/v1/households", h.CreateHousehold)
		r.Post("/api/v1/households/{id}/move", h.MoveHousehold)
		r.Patch("/api/v1/households/{id}", h.UpdateHousehold)
		r.Delete("/api/v1/households/{id}", h.DeactivateHousehold)
		r.Post("/api/v1/residents", h.CreateResident)
		r.Post("/api/v1/residents/{id}/move", h.MoveResident)
		r.Patch("/api/v1/residents/{id}", h.UpdateResident)
		r.Delete("/api/v1/residents/{id}", h.DeactivateResident)
	})

	// Read access: warga, bendahara, pengurus
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireRole(auth.RoleWarga, auth.RoleBendahara, auth.RolePengurus))
		r.Get("/api/v1/households", h.ListHouseholds)
		r.Get("/api/v1/households/{id}", h.GetHousehold)
		r.Get("/api/v1/residents", h.ListResidents)
		r.Get("/api/v1/residents/{id}", h.GetResident)
	})

	return r
}

func setupTestRT(t *testing.T, pool *database.Pool, name string, rw int, rtStr string) string {
	t.Helper()
	db := pool.Raw()

	c := atomic.AddInt64(&rtCounter, 1)
	uniqueName := fmt.Sprintf("%s_%d", name, c)
	uniqueRT := fmt.Sprintf("%s_%d", rtStr, c)

	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
		uniqueName, rw, uniqueRT, "Test Address", "Test Head",
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test RT: %v", err)
	}

	t.Cleanup(func() {
		db.Exec("DELETE FROM residency_periods WHERE resident_id IN (SELECT id FROM residents WHERE rt_id = $1)", id)
		db.Exec("DELETE FROM household_occupancies WHERE household_id IN (SELECT id FROM households WHERE rt_id = $1) OR physical_house_id IN (SELECT id FROM physical_houses WHERE rt_id = $1)", id)
		db.Exec("DELETE FROM physical_houses WHERE rt_id = $1", id)
		db.Exec("DELETE FROM residents WHERE rt_id = $1", id)
		db.Exec("DELETE FROM households WHERE rt_id = $1", id)
		db.Exec("DELETE FROM rts WHERE id = $1", id)
	})

	return id
}

func insertTestHouseholdFixture(t *testing.T, db *sql.DB, rtID, headName string, houseNumber *string, address *string, occStatus *OccupancyStatus) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, $2, true) RETURNING id`,
		rtID, headName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test household: %v", err)
	}

	if houseNumber != nil && *houseNumber != "" {
		var phID string
		err = db.QueryRowContext(context.Background(),
			`SELECT id FROM physical_houses WHERE rt_id = $1 AND house_number = $2 AND is_active = true LIMIT 1`,
			rtID, *houseNumber,
		).Scan(&phID)
		if err != nil {
			err = db.QueryRowContext(context.Background(),
				`INSERT INTO physical_houses (rt_id, house_number, address, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
				rtID, *houseNumber, address,
			).Scan(&phID)
			if err != nil {
				t.Fatalf("failed to insert test physical house: %v", err)
			}
		}

		st := "OWNER"
		if occStatus != nil && *occStatus != "" {
			st = string(*occStatus)
		}
		_, err = db.ExecContext(context.Background(),
			`INSERT INTO household_occupancies (physical_house_id, household_id, occupancy_status, start_date, end_date)
			 VALUES ($1, $2, $3, CURRENT_DATE, NULL)`,
			phID, id, st,
		)
		if err != nil {
			t.Fatalf("failed to insert test household occupancy: %v", err)
		}
	}
	return id
}

func insertTestResidentFixture(t *testing.T, db *sql.DB, rtID, householdID, fullName string, relationshipToHead *string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO residents (rt_id, full_name, is_active) VALUES ($1, $2, true) RETURNING id`,
		rtID, fullName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test resident: %v", err)
	}

	if householdID != "" {
		var hoID string
		err = db.QueryRowContext(context.Background(),
			`SELECT id FROM household_occupancies WHERE household_id = $1 AND end_date IS NULL LIMIT 1`,
			householdID,
		).Scan(&hoID)
		if err == nil {
			_, err = db.ExecContext(context.Background(),
				`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
				 VALUES ($1, $2, $3, CURRENT_DATE, NULL)`,
				id, hoID, relationshipToHead,
			)
			if err != nil {
				t.Fatalf("failed to insert test residency period: %v", err)
			}
		}
	}
	return id
}

// ─── Household Tests ────────────────────────────────────────────────────────

func TestCreateHousehold(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "CreateHouseholdTest RT", 1, "CH01")
	t.Cleanup(func() { pool.Close() })

	role := auth.RolePengurus
	token := makeTenantToken(t, "user-1", "membership-1", rtID, role)

	t.Run("valid input", func(t *testing.T) {
		body := `{"head_name":"John Doe","house_number":"U01","occupancy_status":"OWNER","nik":"3171010101010001","phone":"081234567890","email":"john@example.com"}`
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.HeadName != "John Doe" {
			t.Errorf("expected head_name 'John Doe', got %q", result.HeadName)
		}
		if result.RTID != rtID {
			t.Errorf("expected rt_id %q, got %q", rtID, result.RTID)
		}
		if result.ID == "" {
			t.Error("expected non-empty ID")
		}
		if !result.IsActive {
			t.Error("expected is_active=true")
		}
		if result.HouseNumber == nil || *result.HouseNumber != "U01" {
			t.Errorf("expected house_number 'U01', got %v", result.HouseNumber)
		}
		if result.OccupancyStatus == nil || *result.OccupancyStatus != OccupancyOwner {
			t.Errorf("expected occupancy_status 'OWNER', got %v", result.OccupancyStatus)
		}
		if result.Nik == nil || *result.Nik != "3171010101010001" {
			t.Errorf("expected nik '3171010101010001', got %v", result.Nik)
		}
		if result.Phone == nil || *result.Phone != "+6281234567890" {
			t.Errorf("expected phone '+6281234567890', got %v", result.Phone)
		}
		if result.Email == nil || *result.Email != "john@example.com" {
			t.Errorf("expected email 'john@example.com', got %v", result.Email)
		}
		if result.HeadResident == nil {
			t.Fatal("expected head_resident to be populated")
		}
		if result.HeadResident.FullName != "John Doe" {
			t.Errorf("expected head resident full_name 'John Doe', got %q", result.HeadResident.FullName)
		}
	})

	t.Run("missing head name", func(t *testing.T) {
		body := `{"house_number":"U02","occupancy_status":"OWNER","nik":"3171010101010002","phone":"081234567891","email":"test@example.com"}`
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid nik format", func(t *testing.T) {
		body := `{"head_name":"Bad NIK","house_number":"U03","occupancy_status":"OWNER","nik":"12345","phone":"081234567892","email":"badnik@example.com"}`
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid NIK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("with optional address", func(t *testing.T) {
		addr := "Jl. Test No. 123"
		phone := "+6281234567893"
		nik := fmt.Sprintf("317101%010d", time.Now().UnixNano()%10000000000)
		body := fmt.Sprintf(`{"head_name":"Jane Doe","house_number":"A12","address":"Jl. Test No. 123","phone":"081234567893","email":"jane@example.com","nik":"%s","occupancy_status":"TENANT"}`, nik)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.Address == nil || *result.Address != addr {
			t.Errorf("expected address %q, got %v", addr, result.Address)
		}
		if result.Phone == nil || *result.Phone != phone {
			t.Errorf("expected phone %q, got %v", phone, result.Phone)
		}
		if result.Nik == nil || *result.Nik != nik {
			t.Errorf("expected nik %q, got %v", nik, result.Nik)
		}
	})

	t.Run("duplicate nik within RT returns 409", func(t *testing.T) {
		dupNik := fmt.Sprintf("317102%010d", time.Now().UnixNano()%10000000000)
		firstBody := fmt.Sprintf(`{"head_name":"Dup Test","house_number":"DUP-01","nik":"%s","phone":"0812000001","email":"dup1@example.com","occupancy_status":"OWNER"}`, dupNik)
		req1 := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(firstBody))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Authorization", "Bearer "+token)
		rec1 := httptest.NewRecorder()
		r.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusCreated {
			t.Fatalf("expected 201 for first, got %d: %s", rec1.Code, rec1.Body.String())
		}

		secondBody := fmt.Sprintf(`{"head_name":"Dup Test 2","house_number":"DUP-02","nik":"%s","phone":"0812000002","email":"dup2@example.com","occupancy_status":"TENANT"}`, dupNik)
		req2 := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(secondBody))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+token)
		rec2 := httptest.NewRecorder()
		r.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusConflict {
			t.Errorf("expected 409 conflict for duplicate NIK, got %d: %s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("same nik in different RT allows creation", func(t *testing.T) {
		dupNik := fmt.Sprintf("317103%010d", time.Now().UnixNano()%10000000000)
		body1 := fmt.Sprintf(`{"head_name":"RT1 Head","house_number":"RT1-01","nik":"%s","phone":"0812000011","email":"rt1@example.com","occupancy_status":"OWNER"}`, dupNik)
		req1 := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body1))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Authorization", "Bearer "+token)
		rec1 := httptest.NewRecorder()
		r.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusCreated {
			t.Fatalf("expected 201 in RT1, got %d: %s", rec1.Code, rec1.Body.String())
		}

		// Create RT 2
		rtID2 := setupTestRT(t, pool, "CreateHouseholdTest RT 2", 2, "CH02")
		token2 := makeTenantToken(t, "user-2", "membership-2", rtID2, role)

		body2 := fmt.Sprintf(`{"head_name":"RT2 Head","house_number":"RT2-01","nik":"%s","phone":"0812000012","email":"rt2@example.com","occupancy_status":"OWNER"}`, dupNik)
		req2 := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body2))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+token2)
		rec2 := httptest.NewRecorder()
		r.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusCreated {
			t.Fatalf("expected 201 in RT2 with same NIK, got %d: %s", rec2.Code, rec2.Body.String())
		}
	})
}

func TestListHouseholds(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "ListHouseholdTest RT", 1, "LH01")
	t.Cleanup(func() { pool.Close() })

	role := auth.RoleWarga
	token := makeTenantToken(t, "user-1", "membership-1", rtID, role)

	// Create test households
	for i := 1; i <= 5; i++ {
		hn := fmt.Sprintf("HN-%d", i)
		addr := fmt.Sprintf("Address %d", i)
		occ := OccupancyOwner
		insertTestHouseholdFixture(t, pool.Raw(), rtID, fmt.Sprintf("Head %d", i), &hn, &addr, &occ)
	}

	// Inactive household
	inactID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Inactive Head", nil, nil, nil)
	pool.Raw().Exec("UPDATE households SET is_active = false WHERE id = $1", inactID)

	t.Run("list with pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households?page=1&page_size=2", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		data := resp["data"].([]any)
		if len(data) != 2 {
			t.Errorf("expected 2 items, got %d", len(data))
		}
	})

	t.Run("list with active filter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households?is_active=false", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		data := resp["data"].([]any)
		if len(data) != 1 {
			t.Errorf("expected 1 inactive household, got %d", len(data))
		}
	})

	t.Run("list with search", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households?search=Address%203", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		data := resp["data"].([]any)
		if len(data) != 1 {
			t.Errorf("expected 1 result for search, got %d", len(data))
		}
	})

	t.Run("list with search by NIK phone email and pagination sync", func(t *testing.T) {
		createBody := `{"head_name":"Unique Head","house_number":"UNQ-99","occupancy_status":"OWNER","nik":"3171099999999999","phone":"081299998888","email":"unique_head@test.com"}`
		createReq := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(createBody))
		createReq.Header.Set("Content-Type", "application/json")
		pengurusToken := makeTenantToken(t, "user-p", "membership-p", rtID, auth.RolePengurus)
		createReq.Header.Set("Authorization", "Bearer "+pengurusToken)
		createRec := httptest.NewRecorder()
		r.ServeHTTP(createRec, createReq)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("failed to create unique household: %d: %s", createRec.Code, createRec.Body.String())
		}

		// 1. Search by NIK
		nikReq := httptest.NewRequest("GET", "/api/v1/households?search=3171099999999999", nil)
		nikReq.Header.Set("Authorization", "Bearer "+token)
		nikRec := httptest.NewRecorder()
		r.ServeHTTP(nikRec, nikReq)
		if nikRec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", nikRec.Code)
		}
		var nikResp map[string]any
		json.Unmarshal(nikRec.Body.Bytes(), &nikResp)
		dataNik := nikResp["data"].([]any)
		paginationNik := nikResp["pagination"].(map[string]any)
		if len(dataNik) != 1 || paginationNik["total"].(float64) != 1 {
			t.Errorf("expected 1 result and total=1 for NIK search, got data=%d, total=%v", len(dataNik), paginationNik["total"])
		}

		// 2. Search by phone
		phoneReq := httptest.NewRequest("GET", "/api/v1/households?search=81299998888", nil)
		phoneReq.Header.Set("Authorization", "Bearer "+token)
		phoneRec := httptest.NewRecorder()
		r.ServeHTTP(phoneRec, phoneReq)
		var phoneResp map[string]any
		json.Unmarshal(phoneRec.Body.Bytes(), &phoneResp)
		dataPhone := phoneResp["data"].([]any)
		paginationPhone := phoneResp["pagination"].(map[string]any)
		if len(dataPhone) != 1 || paginationPhone["total"].(float64) != 1 {
			t.Errorf("expected 1 result and total=1 for phone search, got data=%d, total=%v", len(dataPhone), paginationPhone["total"])
		}

		// 3. Search by email
		emailReq := httptest.NewRequest("GET", "/api/v1/households?search=unique_head@test.com", nil)
		emailReq.Header.Set("Authorization", "Bearer "+token)
		emailRec := httptest.NewRecorder()
		r.ServeHTTP(emailRec, emailReq)
		var emailResp map[string]any
		json.Unmarshal(emailRec.Body.Bytes(), &emailResp)
		dataEmail := emailResp["data"].([]any)
		paginationEmail := emailResp["pagination"].(map[string]any)
		if len(dataEmail) != 1 || paginationEmail["total"].(float64) != 1 {
			t.Errorf("expected 1 result and total=1 for email search, got data=%d, total=%v", len(dataEmail), paginationEmail["total"])
		}
	})

	t.Run("search nonmatching returns data=[] not null", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households?search=zzzznonexistentzzzz", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		data, ok := body["data"]
		if !ok {
			t.Fatal("response missing 'data' field")
		}
		if data == nil {
			t.Fatal("response 'data' is null (must be empty array [])")
		}
		arr, ok := data.([]any)
		if !ok {
			t.Fatalf("response 'data' is not an array, got %T", data)
		}
		if len(arr) != 0 {
			t.Errorf("expected 0 results, got %d", len(arr))
		}
	})

	t.Run("resident projection includes RT details", func(t *testing.T) {
		var expectedRT, expectedRTName string
		var expectedRW int
		if err := pool.Raw().QueryRow("SELECT rt, rw, name FROM rts WHERE id = $1", rtID).Scan(&expectedRT, &expectedRW, &expectedRTName); err != nil {
			t.Fatalf("failed to query test RT: %v", err)
		}

		headRel := "HEAD"

		// Create active household with head resident
		hn := "HN-RT-TEST"
		addr := "Address RT Test"
		occ := OccupancyOwner
		testHHID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Head RT Test", &hn, &addr, &occ)
		insertTestResidentFixture(t, pool.Raw(), rtID, testHHID, "Head RT Test Resident", &headRel)

		// 1. Check GET /api/v1/households
		req := httptest.NewRequest("GET", "/api/v1/households?search=HN-RT-TEST", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		data := resp["data"].([]any)
		if len(data) == 0 {
			t.Fatal("expected at least 1 household")
		}
		first := data[0].(map[string]any)
		headRes, ok := first["head_resident"].(map[string]any)
		if !ok || headRes == nil {
			t.Fatal("expected head_resident in household response")
		}
		if headRes["rt_number"] != expectedRT {
			t.Errorf("expected rt_number '%s', got %v", expectedRT, headRes["rt_number"])
		}
		if headRes["rw"] != float64(expectedRW) {
			t.Errorf("expected rw %d, got %v", expectedRW, headRes["rw"])
		}
		if headRes["rt_name"] != expectedRTName {
			t.Errorf("expected rt_name '%s', got %v", expectedRTName, headRes["rt_name"])
		}

		// 2. Check GET /api/v1/residents
		reqRes := httptest.NewRequest("GET", "/api/v1/residents?search=Head%20RT%20Test%20Resident", nil)
		reqRes.Header.Set("Authorization", "Bearer "+token)
		recRes := httptest.NewRecorder()
		r.ServeHTTP(recRes, reqRes)

		if recRes.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recRes.Code, recRes.Body.String())
		}
		var respRes map[string]any
		if err := json.Unmarshal(recRes.Body.Bytes(), &respRes); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		resData := respRes["data"].([]any)
		if len(resData) == 0 {
			t.Fatal("expected at least 1 resident")
		}
		resItem := resData[0].(map[string]any)
		if resItem["rt_number"] != expectedRT {
			t.Errorf("expected resident rt_number '%s', got %v", expectedRT, resItem["rt_number"])
		}
		if resItem["rw"] != float64(expectedRW) {
			t.Errorf("expected resident rw %d, got %v", expectedRW, resItem["rw"])
		}
		if resItem["rt_name"] != expectedRTName {
			t.Errorf("expected resident rt_name '%s', got %v", expectedRTName, resItem["rt_name"])
		}
	})
}

func TestGetHousehold(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "GetHouseholdTest RT", 1, "GH01")
	t.Cleanup(func() { pool.Close() })

	role := auth.RoleBendahara
	token := makeTenantToken(t, "user-1", "membership-1", rtID, role)

	hn := "GH-01"
	addr := "Test Address"
	occ := OccupancyOwner
	hhID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Test Head", &hn, &addr, &occ)

	t.Run("found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households/"+hhID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.ID != hhID {
			t.Errorf("expected id %q, got %q", hhID, result.ID)
		}
		if result.HeadName != "Test Head" {
			t.Errorf("expected head_name 'Test Head', got %q", result.HeadName)
		}
		if result.HouseNumber == nil || *result.HouseNumber != "GH-01" {
			t.Errorf("expected house_number 'GH-01', got %v", result.HouseNumber)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households/00000000-0000-0000-0000-000000000001", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestUpdateHousehold(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "UpdateTest RT", 5, "UT88")
	t.Cleanup(func() { pool.Close() })

	role := auth.RolePengurus
	token := makeTenantToken(t, "user-1", "membership-1", rtID, role)

	hn := "UT-01"
	addr := "Old Address"
	occ := OccupancyOwner
	hhID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Old Head", &hn, &addr, &occ)

	t.Run("partial update", func(t *testing.T) {
		body := `{"head_name":"New Head"}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hhID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.HeadName != "New Head" {
			t.Errorf("expected head_name 'New Head', got %q", result.HeadName)
		}
		if result.Address == nil || *result.Address != "Old Address" {
			t.Errorf("expected address unchanged, got %v", result.Address)
		}
	})

	t.Run("no fields provided", func(t *testing.T) {
		body := `{}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hhID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for no fields, got %d: %s", rec.Code, rec.Body.String())
		}
	})

		t.Run("update deactivated household succeeds", func(t *testing.T) {
		deactivateReq := httptest.NewRequest("DELETE", "/api/v1/households/"+hhID, nil)
		deactivateReq.Header.Set("Authorization", "Bearer "+token)
		deactivateRec := httptest.NewRecorder()
		r.ServeHTTP(deactivateRec, deactivateReq)
		if deactivateRec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 for deactivate, got %d: %s", deactivateRec.Code, deactivateRec.Body.String())
		}

		body := `{"head_name":"Reactivated Head"}`
		updateReq := httptest.NewRequest("PATCH", "/api/v1/households/"+hhID, bytes.NewBufferString(body))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("Authorization", "Bearer "+token)
		updateRec := httptest.NewRecorder()
		r.ServeHTTP(updateRec, updateReq)

		if updateRec.Code != http.StatusOK {
			t.Errorf("expected 200 for deactivated household patch, got %d: %s", updateRec.Code, updateRec.Body.String())
		}
		var result Household
		if err := json.Unmarshal(updateRec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		// head_name should be updated but is_active should remain false
		if result.HeadName != "Reactivated Head" {
			t.Errorf("expected head_name 'Reactivated Head', got %q", result.HeadName)
		}
		if result.IsActive {
			t.Error("expected is_active=false after patch without is_active field")
		}
	})

	t.Run("update head resident fields and sync head_name", func(t *testing.T) {
		cBody := `{"head_name":"Original Head","house_number":"UP-HEAD-01","occupancy_status":"OWNER","nik":"3171088888888881","phone":"081288888881","email":"orig@test.com"}`
		cReq := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(cBody))
		cReq.Header.Set("Content-Type", "application/json")
		cReq.Header.Set("Authorization", "Bearer "+token)
		cRec := httptest.NewRecorder()
		r.ServeHTTP(cRec, cReq)
		if cRec.Code != http.StatusCreated {
			t.Fatalf("create failed: %d: %s", cRec.Code, cRec.Body.String())
		}
		var created Household
		json.Unmarshal(cRec.Body.Bytes(), &created)

		patchBody := `{"head_name":"Updated Head Name","nik":"3171088888888882","phone":"081288888882","email":"updated@test.com"}`
		pReq := httptest.NewRequest("PATCH", "/api/v1/households/"+created.ID, bytes.NewBufferString(patchBody))
		pReq.Header.Set("Content-Type", "application/json")
		pReq.Header.Set("Authorization", "Bearer "+token)
		pRec := httptest.NewRecorder()
		r.ServeHTTP(pRec, pReq)
		if pRec.Code != http.StatusOK {
			t.Fatalf("patch failed: %d: %s", pRec.Code, pRec.Body.String())
		}
		var patched Household
		json.Unmarshal(pRec.Body.Bytes(), &patched)
		if patched.HeadName != "Updated Head Name" {
			t.Errorf("expected head_name 'Updated Head Name', got %q", patched.HeadName)
		}
		if patched.Nik == nil || *patched.Nik != "3171088888888882" {
			t.Errorf("expected nik '3171088888888882', got %v", patched.Nik)
		}
		if patched.Phone == nil || *patched.Phone != "+6281288888882" {
			t.Errorf("expected phone '+6281288888882', got %v", patched.Phone)
		}
		if patched.Email == nil || *patched.Email != "updated@test.com" {
			t.Errorf("expected email 'updated@test.com', got %v", patched.Email)
		}
		if patched.HeadResident == nil || patched.HeadResident.FullName != "Updated Head Name" {
			t.Errorf("expected head_resident full_name to be 'Updated Head Name', got %+v", patched.HeadResident)
		}
	})

	t.Run("duplicate nik on patch returns 409 conflict", func(t *testing.T) {
		cBody1 := `{"head_name":"Target Head 1","house_number":"UP-NIK-01","occupancy_status":"OWNER","nik":"3171088888888891","phone":"081288888891","email":"nik1@test.com"}`
		cReq1 := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(cBody1))
		cReq1.Header.Set("Content-Type", "application/json")
		cReq1.Header.Set("Authorization", "Bearer "+token)
		cRec1 := httptest.NewRecorder()
		r.ServeHTTP(cRec1, cReq1)
		if cRec1.Code != http.StatusCreated {
			t.Fatalf("create 1 failed: %d: %s", cRec1.Code, cRec1.Body.String())
		}

		cBody2 := `{"head_name":"Target Head 2","house_number":"UP-NIK-02","occupancy_status":"OWNER","nik":"3171088888888892","phone":"081288888892","email":"nik2@test.com"}`
		cReq2 := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(cBody2))
		cReq2.Header.Set("Content-Type", "application/json")
		cReq2.Header.Set("Authorization", "Bearer "+token)
		cRec2 := httptest.NewRecorder()
		r.ServeHTTP(cRec2, cReq2)
		if cRec2.Code != http.StatusCreated {
			t.Fatalf("create 2 failed: %d: %s", cRec2.Code, cRec2.Body.String())
		}
		var h2 Household
		json.Unmarshal(cRec2.Body.Bytes(), &h2)

		conflictBody := `{"nik":"3171088888888891"}`
		pReq := httptest.NewRequest("PATCH", "/api/v1/households/"+h2.ID, bytes.NewBufferString(conflictBody))
		pReq.Header.Set("Content-Type", "application/json")
		pReq.Header.Set("Authorization", "Bearer "+token)
		pRec := httptest.NewRecorder()
		r.ServeHTTP(pRec, pReq)
		if pRec.Code != http.StatusConflict {
			t.Errorf("expected 409 conflict, got %d: %s", pRec.Code, pRec.Body.String())
		}
	})

	t.Run("supplied empty NIK rejected", func(t *testing.T) {
		body := `{"nik":""}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hhID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty nik, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("supplied empty phone rejected", func(t *testing.T) {
		body := `{"phone":"   "}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hhID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty phone, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("supplied empty email rejected", func(t *testing.T) {
		body := `{"email":""}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hhID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty email, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("omitted optional fields preserve legitimate PATCH behavior", func(t *testing.T) {
		activeHN := "OMIT-01"
		activeAddr := "Old Address"
		activeOcc := OccupancyOwner
		activeHHID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Omit Head", &activeHN, &activeAddr, &activeOcc)

		body := `{"address":"New Only Address"}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+activeHHID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("repair legacy household without head resident", func(t *testing.T) {
		legacyHN := "LEGACY-01"
		legacyAddr := "Legacy Street 1"
		legacyOcc := OccupancyOwner
		legacyHHID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Legacy Head", &legacyHN, &legacyAddr, &legacyOcc)

		patchBody := `{"nik":"3171088888888877","phone":"081288888877","email":"legacy@test.com"}`
		pReq := httptest.NewRequest("PATCH", "/api/v1/households/"+legacyHHID, bytes.NewBufferString(patchBody))
		pReq.Header.Set("Content-Type", "application/json")
		pReq.Header.Set("Authorization", "Bearer "+token)
		pRec := httptest.NewRecorder()
		r.ServeHTTP(pRec, pReq)
		if pRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for legacy repair, got %d: %s", pRec.Code, pRec.Body.String())
		}
		var repaired Household
		json.Unmarshal(pRec.Body.Bytes(), &repaired)
		if repaired.Nik == nil || *repaired.Nik != "3171088888888877" {
			t.Errorf("expected nik '3171088888888877', got %v", repaired.Nik)
		}
		if repaired.HeadResident == nil {
			t.Fatal("expected repaired household to have head_resident populated")
		}
		if repaired.HeadResident.FullName != "Legacy Head" {
			t.Errorf("expected head_resident full_name 'Legacy Head', got %q", repaired.HeadResident.FullName)
		}
	})
}

func TestHouseholdMove(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "MoveTest RT", 1, "MV01")
	t.Cleanup(func() { pool.Close() })

	role := auth.RolePengurus
	token := makeTenantToken(t, "user-1", "membership-1", rtID, role)

	hn := "MV-01"
	addr := "Origin Address"
	occ := OccupancyOwner
	hhID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Moving Head", &hn, &addr, &occ)

	// Add a resident
	rel := "Spouse"
	resID := insertTestResidentFixture(t, pool.Raw(), rtID, hhID, "Moving Resident", &rel)

	t.Run("valid household move with explicit start_date", func(t *testing.T) {
		moveDate := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
		body := fmt.Sprintf(`{"house_number":"MV-02","address":"Destination Address","start_date":"%s","occupancy_status":"TENANT"}`, moveDate)
		req := httptest.NewRequest("POST", "/api/v1/households/"+hhID+"/move", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.HouseNumber == nil || *result.HouseNumber != "MV-02" {
			t.Errorf("expected new house_number 'MV-02', got %v", result.HouseNumber)
		}
		if result.Address == nil || *result.Address != "Destination Address" {
			t.Errorf("expected new address 'Destination Address', got %v", result.Address)
		}
		if result.OccupancyStatus == nil || *result.OccupancyStatus != OccupancyTenant {
			t.Errorf("expected new occupancy_status 'TENANT', got %v", result.OccupancyStatus)
		}

		// Verify resident residency period was moved
		var resRel, currHH sql.NullString
		err := pool.Raw().QueryRowContext(context.Background(),
			`SELECT rp.relationship_to_head, ho.household_id
			 FROM residency_periods rp
			 JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id
			 WHERE rp.resident_id = $1 AND rp.end_date IS NULL`,
			resID,
		).Scan(&resRel, &currHH)
		if err != nil {
			t.Fatalf("failed to query resident new period: %v", err)
		}
		if currHH.String != hhID {
			t.Errorf("expected resident still in household %s, got %s", hhID, currHH.String)
		}
	})

	t.Run("move missing start_date returns 400", func(t *testing.T) {
		body := `{"house_number":"MV-03"}`
		req := httptest.NewRequest("POST", "/api/v1/households/"+hhID+"/move", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing start_date, got %d", rec.Code)
		}
	})
}

func TestResidentMove(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "ResidentMoveTest RT", 1, "RM01")
	t.Cleanup(func() { pool.Close() })

	role := auth.RolePengurus
	token := makeTenantToken(t, "user-1", "membership-1", rtID, role)

	hn1 := "RM-01"
	hn2 := "RM-02"
	occ := OccupancyOwner
	hh1ID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Head 1", &hn1, nil, &occ)
	hh2ID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Head 2", &hn2, nil, &occ)

	rel := "Child"
	resID := insertTestResidentFixture(t, pool.Raw(), rtID, hh1ID, "Moving Child", &rel)

	t.Run("valid resident move", func(t *testing.T) {
		moveDate := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
		body := fmt.Sprintf(`{"destination_household_id":"%s","start_date":"%s","relationship_to_head":"Relative"}`, hh2ID, moveDate)
		req := httptest.NewRequest("POST", "/api/v1/residents/"+resID+"/move", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result Resident
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.HouseholdID == nil || *result.HouseholdID != hh2ID {
			t.Errorf("expected household_id %s, got %v", hh2ID, result.HouseholdID)
		}
		if result.RelationshipToHead == nil || *result.RelationshipToHead != "Relative" {
			t.Errorf("expected relationship 'Relative', got %v", result.RelationshipToHead)
		}
	})

	t.Run("move to cross-RT household returns 404", func(t *testing.T) {
		rtBRID := setupTestRT(t, pool, "ResidentMoveTest RT B", 2, "RM02B")
		hnB := "RMB-01"
		hhBRID := insertTestHouseholdFixture(t, pool.Raw(), rtBRID, "Head B", &hnB, nil, &occ)

		moveDate := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
		body := fmt.Sprintf(`{"destination_household_id":"%s","start_date":"%s"}`, hhBRID, moveDate)
		req := httptest.NewRequest("POST", "/api/v1/residents/"+resID+"/move", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for cross-RT move, got %d", rec.Code)
		}
	})
}

func TestCreateResident(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "CreateResidentTest RT", 1, "CR01")
	t.Cleanup(func() { pool.Close() })

	role := auth.RolePengurus
	token := makeTenantToken(t, "user-1", "membership-1", rtID, role)

	hn := "CR-01"
	occ := OccupancyOwner
	hhID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Head Name", &hn, nil, &occ)

	rtBRID := setupTestRT(t, pool, "CreateResidentTest RT B", 2, "CR02")
	hnB := "CR-02"
	hhBRID := insertTestHouseholdFixture(t, pool.Raw(), rtBRID, "RT B Head", &hnB, nil, &occ)

	t.Run("valid input", func(t *testing.T) {
		body := fmt.Sprintf(`{"household_id":"%s","full_name":"Jane Doe","nik":"3201010101990099","phone":"081234567890","email":"jane@example.com","relationship_to_head":"Spouse"}`, hhID)
		req := httptest.NewRequest("POST", "/api/v1/residents", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var result Resident
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if result.FullName != "Jane Doe" {
			t.Errorf("expected full_name 'Jane Doe', got %q", result.FullName)
		}
		if result.RTID != rtID {
			t.Errorf("expected rt_id %q, got %q", rtID, result.RTID)
		}
		if result.HouseholdID == nil || *result.HouseholdID != hhID {
			t.Errorf("expected household_id %q, got %v", hhID, result.HouseholdID)
		}
	})

	t.Run("missing full name", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/residents", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("household in different RT returns 404", func(t *testing.T) {
		body := fmt.Sprintf(`{"household_id":"%s","full_name":"Other RT Person","nik":"3201010101990098","phone":"081234567891","email":"other@example.com"}`, hhBRID)
		req := httptest.NewRequest("POST", "/api/v1/residents", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for cross-RT household, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestTenantIsolationHouseholds(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtAID := setupTestRT(t, pool, "TenantIsolationRT A", 1, "TI01")
	rtBID := setupTestRT(t, pool, "TenantIsolationRT B", 1, "TI02")
	t.Cleanup(func() { pool.Close() })

	hnA := "TI-A"
	hnB := "TI-B"
	occ := OccupancyOwner
	hhAID := insertTestHouseholdFixture(t, pool.Raw(), rtAID, "RT A Head", &hnA, nil, &occ)
	hhBID := insertTestHouseholdFixture(t, pool.Raw(), rtBID, "RT B Head", &hnB, nil, &occ)

	tokenA := makeTenantToken(t, "user-a-1", "membership-a-1", rtAID, auth.RoleWarga)
	tokenAPengurus := makeTenantToken(t, "user-a-pengurus", "membership-a-pengurus", rtAID, auth.RolePengurus)

	t.Run("RT A can GET RT A household", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households/"+hhAID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for own RT, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("RT A cannot GET RT B household → 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households/"+hhBID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for cross-RT, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("RT A cannot PATCH RT B household → 404", func(t *testing.T) {
		body := `{"head_name":"Hacked"}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hhBID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenAPengurus)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for cross-RT PATCH, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("RT A cannot DELETE RT B household → 404", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/households/"+hhBID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenAPengurus)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for cross-RT DELETE, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestSuperAdminGlobalReadHouseholds(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtAID := setupTestRT(t, pool, "SuperAdminRead RT A", 1, "SARM-A")
	rtBID := setupTestRT(t, pool, "SuperAdminRead RT B", 1, "SARM-B")
	t.Cleanup(func() { pool.Close() })

	marker := fmt.Sprintf("SARM-READ-%d", time.Now().UnixNano())
	hnA := "SA-HN-A"
	addrA := "Super Admin Read Address A"
	occ := OccupancyOwner
	hhAID := insertTestHouseholdFixture(t, pool.Raw(), rtAID, marker+"-A", &hnA, &addrA, &occ)

	hnB := "SA-HN-B"
	addrB := "Super Admin Read Address B"
	hhBID := insertTestHouseholdFixture(t, pool.Raw(), rtBID, marker+"-B", &hnB, &addrB, &occ)

	adminToken := makeSuperAdminToken(t)

	t.Run("global list returns households from multiple RTs", func(t *testing.T) {
		searchParam := url.QueryEscape(marker)
		req := httptest.NewRequest("GET", "/api/v1/households?search="+searchParam, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		data, ok := resp["data"].([]any)
		if !ok {
			t.Fatalf("expected data to be an array")
		}

		foundA := false
		foundB := false
		var rtIDFromA, rtIDFromB string
		for _, raw := range data {
			item := raw.(map[string]any)
			id := item["id"].(string)
			if id == hhAID {
				foundA = true
				rtIDFromA = item["rt_id"].(string)
			}
			if id == hhBID {
				foundB = true
				rtIDFromB = item["rt_id"].(string)
			}
		}
		if !foundA {
			t.Errorf("expected household from RT A (id=%s) in response", hhAID)
		}
		if !foundB {
			t.Errorf("expected household from RT B (id=%s) in response", hhBID)
		}
		if foundA && rtIDFromA != rtAID {
			t.Errorf("expected rt_id=%s for RT A household, got %s", rtAID, rtIDFromA)
		}
		if foundB && rtIDFromB != rtBID {
			t.Errorf("expected rt_id=%s for RT B household, got %s", rtBID, rtIDFromB)
		}
	})

	t.Run("global detail returns household from any RT", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households/"+hhBID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if result.ID != hhBID {
			t.Errorf("expected id %s, got %s", hhBID, result.ID)
		}
		if result.RTID != rtBID {
			t.Errorf("expected rt_id %s, got %s", rtBID, result.RTID)
		}
		if result.HeadName != marker+"-B" {
			t.Errorf("expected head_name %q, got %q", marker+"-B", result.HeadName)
		}
	})
}

func TestUnauthorizedAccess(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()
	t.Cleanup(func() { pool.Close() })

	t.Run("no auth header → 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid token → 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households", nil)
		req.Header.Set("Authorization", "Bearer invalid-token-here")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestHouseNumberCorrectionAndCollision(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "CorrectionTest RT", 1, "CORR01")
	t.Cleanup(func() { pool.Close() })

	token := makeTenantToken(t, "user-1", "membership-1", rtID, auth.RolePengurus)

	hn1 := "CORR-A"
	hn2 := "CORR-B"
	occ := OccupancyOwner
	hh1ID := insertTestHouseholdFixture(t, pool.Raw(), rtID, "Household 1", &hn1, nil, &occ)
	_ = insertTestHouseholdFixture(t, pool.Raw(), rtID, "Household 2", &hn2, nil, &occ)

	t.Run("successful house_number correction", func(t *testing.T) {
		body := `{"house_number":"CORR-A-FIXED"}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hh1ID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if result.HouseNumber == nil || *result.HouseNumber != "CORR-A-FIXED" {
			t.Errorf("expected house_number CORR-A-FIXED, got %v", result.HouseNumber)
		}
	})

	t.Run("collision with another active house in same RT returns 409", func(t *testing.T) {
		body := `{"house_number":"CORR-B"}`
		req := httptest.NewRequest("PATCH", "/api/v1/households/"+hh1ID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 conflict for house number collision, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestLegacyNullTemporalRowsReadable(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "LegacyTest RT", 1, "LEG01")
	t.Cleanup(func() { pool.Close() })

	token := makeTenantToken(t, "user-1", "membership-1", rtID, auth.RoleWarga)

	// Insert legacy household with NO occupancy row
	var legacyHHID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'Legacy No Occ', true) RETURNING id`,
		rtID,
	).Scan(&legacyHHID)
	if err != nil {
		t.Fatalf("failed to insert legacy household: %v", err)
	}

	// Insert legacy resident with NO residency period
	var legacyResID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO residents (rt_id, full_name, is_active) VALUES ($1, 'Legacy No Period', true) RETURNING id`,
		rtID,
	).Scan(&legacyResID)
	if err != nil {
		t.Fatalf("failed to insert legacy resident: %v", err)
	}

	t.Run("read household with no current occupancy returns null fields without error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/households/"+legacyHHID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if result.HouseNumber != nil {
			t.Errorf("expected nil house_number, got %v", result.HouseNumber)
		}
		if result.OccupancyStatus != nil {
			t.Errorf("expected nil occupancy_status, got %v", result.OccupancyStatus)
		}
	})

	t.Run("read resident with no current residency period returns null household_id without error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents/"+legacyResID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var result Resident
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if result.HouseholdID != nil {
			t.Errorf("expected nil household_id, got %v", result.HouseholdID)
		}
	})
}

func TestSuperAdminCreateHousehold(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	activeRT := setupTestRT(t, pool, "SACreate RT", 1, "SACT01")
	t.Cleanup(func() { pool.Close() })

	marker := fmt.Sprintf("SACREATE-%d", time.Now().UnixNano())

	// inactive RT: insert then deactivate
	var inactiveRT string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
		"SACreateInactive", 1, "SACTI", "Inactive Addr", "Inactive Head",
	).Scan(&inactiveRT)
	if err != nil {
		t.Fatalf("failed to insert inactive RT: %v", err)
	}
	pool.Raw().Exec("UPDATE rts SET is_active = false WHERE id = $1", inactiveRT)

	adminToken := makeSuperAdminToken(t)
	nonExistentRTID := "00000000-0000-0000-0000-000000000001"

	t.Run("valid active rt_id returns 201", func(t *testing.T) {
		body := fmt.Sprintf(`{"rt_id":"%s","head_name":"%s","house_number":"SA-A1","nik":"3201010101990097","phone":"081234567897","email":"headsa1@example.com","occupancy_status":"OWNER"}`, activeRT, marker)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if result.RTID != activeRT {
			t.Errorf("expected rt_id %s, got %s", activeRT, result.RTID)
		}
		if result.HeadName != marker {
			t.Errorf("expected head_name %q, got %q", marker, result.HeadName)
		}
	})

	t.Run("missing rt_id returns 400", func(t *testing.T) {
		body := fmt.Sprintf(`{"head_name":"%s","house_number":"SA-A2","nik":"3201010101990097","phone":"081234567897","email":"headsa1@example.com","occupancy_status":"OWNER"}`, marker)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("whitespace-only rt_id returns 400", func(t *testing.T) {
		whitespaceRT := "   "
		body := fmt.Sprintf(`{"rt_id":"%s","head_name":"%s","house_number":"SA-A3","nik":"3201010101990097","phone":"081234567897","email":"headsa1@example.com","occupancy_status":"OWNER"}`, whitespaceRT, marker)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("nonexistent rt_id returns 404", func(t *testing.T) {
		body := fmt.Sprintf(`{"rt_id":"%s","head_name":"%s","house_number":"SA-A4","nik":"3201010101990097","phone":"081234567897","email":"headsa1@example.com","occupancy_status":"OWNER"}`, nonExistentRTID, marker)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("inactive rt_id returns 400", func(t *testing.T) {
		body := fmt.Sprintf(`{"rt_id":"%s","head_name":"%s","house_number":"SA-A5","nik":"3201010101990097","phone":"081234567897","email":"headsa1@example.com","occupancy_status":"OWNER"}`, inactiveRT, marker)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestTenantCreateHouseholdSecurity(t *testing.T) {
	pool := testPool(t)
	r := setUpRouter(pool)
	signingSecret()

	rtA := setupTestRT(t, pool, "TenantSec RTA", 1, "TS-A")
	rtB := setupTestRT(t, pool, "TenantSec RTB", 1, "TS-B")
	t.Cleanup(func() { pool.Close() })

	marker := fmt.Sprintf("TENANTSEC-%d", time.Now().UnixNano())

	pengurusTokenA := makeTenantToken(t, "pengurus-a", "membership-a", rtA, auth.RolePengurus)

	t.Run("ordinary PENGURUS without rt_id succeeds", func(t *testing.T) {
		body := fmt.Sprintf(`{"head_name":"%s","house_number":"TS-P1","nik":"3201010101990096","phone":"081234567896","email":"headtsp1@example.com","occupancy_status":"OWNER"}`, marker)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusTokenA)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		var result Household
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if result.RTID != rtA {
			t.Errorf("expected rt_id %s (authenticated RT), got %s", rtA, result.RTID)
		}
	})

	t.Run("PENGURUS supplying rt_id rejected with 400", func(t *testing.T) {
		body := fmt.Sprintf(`{"rt_id":"%s","head_name":"%s","house_number":"TS-P2","nik":"3201010101990096","phone":"081234567896","email":"headtsp1@example.com","occupancy_status":"OWNER"}`, rtB, marker)
		req := httptest.NewRequest("POST", "/api/v1/households", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusTokenA)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
