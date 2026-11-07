package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

var saResidentSeq uint64 = uint64(time.Now().UnixNano())

func createTestSuperAdminUser(t *testing.T, pool *database.Pool) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("sa-resident-%d@test.com", atomic.AddUint64(&saResidentSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'Super Admin Resident Test', 'super_admin', true)
		 RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("create super admin user: %v", err)
	}
	t.Cleanup(func() {
		pool.Raw().ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		SysRole: string(auth.SystemRoleSuperAdmin),
	})
	if err != nil {
		t.Fatalf("generate SA token: %v", err)
	}
	return userID, token
}

func getTestRTs(t *testing.T, pool *database.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	var rt002ID, rt003ID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '002' AND is_active = true LIMIT 1`).Scan(&rt002ID)
	if err != nil {
		t.Fatalf("get rt 002: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' AND is_active = true LIMIT 1`).Scan(&rt003ID)
	if err != nil {
		t.Fatalf("get rt 003: %v", err)
	}
	return rt002ID, rt003ID
}

// 1. Super Admin without user_rt_memberships can GET /api/v1/residents with valid X-RT-ID (HTTP 200).
func TestSuperAdminResident_GetResidents_SuccessWithXRTID(t *testing.T) {
	pool, router := setupRouterTest(t)
	_, saToken := createTestSuperAdminUser(t, pool)
	rt002ID, _ := getTestRTs(t, pool)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/residents", nil)
	req.Header.Set("Authorization", "Bearer "+saToken)
	req.Header.Set("X-RT-ID", rt002ID)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/residents expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
}

// 2 & 3. Super Admin without membership can POST /api/v1/residents with valid X-RT-ID (HTTP 201)
// and resident created is properly scoped to the chosen RT.
func TestSuperAdminResident_PostResident_SuccessWithXRTID(t *testing.T) {
	pool, router := setupRouterTest(t)
	_, saToken := createTestSuperAdminUser(t, pool)
	rt002ID, _ := getTestRTs(t, pool)

	seq := atomic.AddUint64(&saResidentSeq, 1)
	residentName := fmt.Sprintf("Warga SA Test %d", seq)
	nik := fmt.Sprintf("317101%010d", seq%10000000000)
	email := fmt.Sprintf("warga-sa-%d@test.com", seq)
	phone := fmt.Sprintf("+62812%07d", seq%10000000)

	reqBody, _ := json.Marshal(map[string]any{
		"full_name": residentName,
		"nik":       nik,
		"email":     email,
		"phone":     phone,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+saToken)
	req.Header.Set("X-RT-ID", rt002ID)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/residents expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID   string `json:"id"`
		RTID string `json:"rt_id"`
		Name string `json:"full_name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created resident: %v", err)
	}

	if created.ID == "" {
		t.Fatalf("expected non-empty resident ID")
	}
	if created.RTID != rt002ID {
		t.Fatalf("resident scoped RT ID = %q; want %q", created.RTID, rt002ID)
	}

	// Verify in database that rt_id matches
	var dbRTID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT rt_id FROM residents WHERE id = $1`, created.ID).Scan(&dbRTID)
	if err != nil {
		t.Fatalf("query db for resident: %v", err)
	}
	if dbRTID != rt002ID {
		t.Fatalf("db rt_id = %q; want %q", dbRTID, rt002ID)
	}
}

// 4. Super Admin can access resident detail GET /api/v1/residents/{id} with valid X-RT-ID (HTTP 200).
func TestSuperAdminResident_GetResidentDetail_SuccessWithXRTID(t *testing.T) {
	pool, router := setupRouterTest(t)
	_, saToken := createTestSuperAdminUser(t, pool)
	rt002ID, _ := getTestRTs(t, pool)

	// Create resident in RT 002
	var resID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO residents (id, rt_id, full_name, phone, is_active)
		 VALUES (gen_random_uuid(), $1, 'Detail Warga Test', $2, true)
		 RETURNING id`,
		rt002ID, fmt.Sprintf("+628%09d", time.Now().UnixNano()%1000000000),
	).Scan(&resID)
	if err != nil {
		t.Fatalf("insert resident: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+resID, nil)
	req.Header.Set("Authorization", "Bearer "+saToken)
	req.Header.Set("X-RT-ID", rt002ID)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/residents/%s expected 200, got %d: %s", resID, rec.Code, rec.Body.String())
	}
}

// 5. Super Admin without RT context receives HTTP 403 ("tenant context required").
func TestSuperAdminResident_WithoutRTContext_Returns403(t *testing.T) {
	pool, router := setupRouterTest(t)
	_, saToken := createTestSuperAdminUser(t, pool)

	// 5a. GET /api/v1/residents without X-RT-ID or ?rt_id
	req := httptest.NewRequest(http.MethodGet, "/api/v1/residents", nil)
	req.Header.Set("Authorization", "Bearer "+saToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /api/v1/residents without RT context expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5b. POST /api/v1/residents without X-RT-ID or ?rt_id or body rt_id
	reqBody, _ := json.Marshal(map[string]any{
		"full_name": "No Context Warga",
	})
	reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(reqBody))
	reqPost.Header.Set("Authorization", "Bearer "+saToken)
	reqPost.Header.Set("Content-Type", "application/json")

	recPost := httptest.NewRecorder()
	router.ServeHTTP(recPost, reqPost)

	if recPost.Code != http.StatusForbidden {
		t.Fatalf("POST /api/v1/residents without RT context expected 403, got %d: %s", recPost.Code, recPost.Body.String())
	}
}

// 6. Request with invalid RT ID is rejected (HTTP 403).
func TestSuperAdminResident_WithInvalidRTID_Returns403(t *testing.T) {
	pool, router := setupRouterTest(t)
	_, saToken := createTestSuperAdminUser(t, pool)

	invalidRT := "00000000-0000-0000-0000-000000000999"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/residents", nil)
	req.Header.Set("Authorization", "Bearer "+saToken)
	req.Header.Set("X-RT-ID", invalidRT)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /api/v1/residents with invalid RT expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 7. Regular user without permission gets HTTP 403.
func TestSuperAdminResident_RegularUserWithoutPermission_Returns403(t *testing.T) {
	pool, router := setupRouterTest(t)
	rt002ID, _ := getTestRTs(t, pool)

	// Create regular warga user with role=warga, no jabatan (no warga.create permission)
	var wargaUserID, wargaMID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'Warga Polos', NULL, true)
		 RETURNING id`,
		fmt.Sprintf("warga-polos-%d@test.com", atomic.AddUint64(&saResidentSeq, 1)),
	).Scan(&wargaUserID)
	if err != nil {
		t.Fatalf("insert warga user: %v", err)
	}
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, 'warga', NULL, true)
		 RETURNING id`,
		wargaUserID, rt002ID,
	).Scan(&wargaMID)
	if err != nil {
		t.Fatalf("insert warga membership: %v", err)
	}

	wargaToken := makeTestToken(t, wargaUserID, wargaMID, rt002ID, auth.RoleWarga, "")

	reqBody, _ := json.Marshal(map[string]any{
		"full_name": "Unauthorized Attempt",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+wargaToken)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /api/v1/residents as regular warga expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 8. Pengurus with valid permission succeeds as before.
func TestSuperAdminResident_PengurusWithValidPermission_Succeeds(t *testing.T) {
	pool, router := setupRouterTest(t)
	rt002ID, _ := getTestRTs(t, pool)

	// Create ketua RT user
	var ketuaUserID, ketuaMID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'Ketua RT Valid', NULL, true)
		 RETURNING id`,
		fmt.Sprintf("ketua-valid-%d@test.com", atomic.AddUint64(&saResidentSeq, 1)),
	).Scan(&ketuaUserID)
	if err != nil {
		t.Fatalf("insert ketua user: %v", err)
	}
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, 'warga', 'ketua', true)
		 RETURNING id`,
		ketuaUserID, rt002ID,
	).Scan(&ketuaMID)
	if err != nil {
		t.Fatalf("insert ketua membership: %v", err)
	}

	ketuaToken := makeTestToken(t, ketuaUserID, ketuaMID, rt002ID, auth.RoleWarga, auth.JabatanKetua)

	// Ketua can GET residents
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/residents", nil)
	reqGet.Header.Set("Authorization", "Bearer "+ketuaToken)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/residents as ketua expected 200, got %d: %s", recGet.Code, recGet.Body.String())
	}

	// Ketua can POST resident
	seq := atomic.AddUint64(&saResidentSeq, 1)
	reqBody, _ := json.Marshal(map[string]any{
		"full_name": fmt.Sprintf("Warga By Ketua %d", seq),
		"nik":       fmt.Sprintf("317102%010d", seq%10000000000),
		"email":     fmt.Sprintf("warga-ketua-%d@test.com", seq),
		"phone":     fmt.Sprintf("+62813%07d", seq%10000000),
	})
	reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(reqBody))
	reqPost.Header.Set("Authorization", "Bearer "+ketuaToken)
	reqPost.Header.Set("Content-Type", "application/json")
	recPost := httptest.NewRecorder()
	router.ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/residents as ketua expected 201, got %d: %s", recPost.Code, recPost.Body.String())
	}
}

// 9. No cross-tenant data leakage (accessing resident of RT A using RT B context yields 404).
func TestSuperAdminResident_CrossTenantIsolation_Returns404(t *testing.T) {
	pool, router := setupRouterTest(t)
	_, saToken := createTestSuperAdminUser(t, pool)
	rt002ID, rt003ID := getTestRTs(t, pool)

	// Create resident strictly in RT 002
	var resID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO residents (id, rt_id, full_name, phone, is_active)
		 VALUES (gen_random_uuid(), $1, 'RT 002 Isolated Resident', $2, true)
		 RETURNING id`,
		rt002ID, fmt.Sprintf("+628%09d", time.Now().UnixNano()%1000000000),
	).Scan(&resID)
	if err != nil {
		t.Fatalf("insert resident in RT 002: %v", err)
	}

	// Try to access resident of RT 002 while Super Admin has RT 003 context in X-RT-ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+resID, nil)
	req.Header.Set("Authorization", "Bearer "+saToken)
	req.Header.Set("X-RT-ID", rt003ID)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/residents/%s with wrong RT context expected 404, got %d: %s", resID, rec.Code, rec.Body.String())
	}
}

// 10. Modul RT/RW authorization is unaffected.
func TestSuperAdminResident_RTRWAuthorizationUnaffected(t *testing.T) {
	pool, router := setupRouterTest(t)
	_, saToken := createTestSuperAdminUser(t, pool)

	// Super Admin can access RT list without X-RT-ID
	reqRT := httptest.NewRequest(http.MethodGet, "/api/v1/rts", nil)
	reqRT.Header.Set("Authorization", "Bearer "+saToken)
	recRT := httptest.NewRecorder()
	router.ServeHTTP(recRT, reqRT)
	if recRT.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/rts as SA expected 200, got %d: %s", recRT.Code, recRT.Body.String())
	}

	// Super Admin can access RW list without X-RT-ID
	reqRW := httptest.NewRequest(http.MethodGet, "/api/v1/rws", nil)
	reqRW.Header.Set("Authorization", "Bearer "+saToken)
	recRW := httptest.NewRecorder()
	router.ServeHTTP(recRW, reqRW)
	if recRW.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/rws as SA expected 200, got %d: %s", recRW.Code, recRW.Body.String())
	}
}
