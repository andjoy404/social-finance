package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/devseed"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

func setupRouterTest(t *testing.T) (*database.Pool, http.Handler) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("ensure devseed: %v", err)
	}

	auth.SigningSecret = []byte("test-secret-key-32-chars-long-ok!!")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	auth.AuthDBPool = pool

	authSvc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: 7 * 24 * time.Hour,
	})
	authHandler := auth.NewHandler(pool, authSvc)
	// Scope must contain both canonical seeded RTs (a95c1cb5 = RT 002 Urena,
	// 8dfa1e36 = RT 001 Wisma Rukun Tunggal) so super-admins pinned to either
	// pass the fail-closed scope check. RWIDs empty is fine for RT-only tests.
	cfg := auth.ScopingConfig{
		RTIDs: []string{
			"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b",
			"8dfa1e36-6727-45be-8fea-24d958efc155",
		},
		RWIDs: []string{},
	}
	router := buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
	return pool, router
}

func makeTestToken(t *testing.T, userID, membershipID, rtID string, role auth.Role, jabatan auth.Jabatan) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     membershipID,
		RTID:    rtID,
		Role:    string(role),
		Jabatan: string(jabatan),
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

func TestKetuaRTCanUpdateResidentWithoutWargaCreate(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	// 1. Get an existing resident and its allowed RT from seed
	var rtID, residentID string
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT res.id, r.id
		FROM residents res
		JOIN rts r ON r.id = res.rt_id
		WHERE r.rt IN ('002', '003') AND res.is_active = true AND r.is_active = true
		LIMIT 1
	`).Scan(&residentID, &rtID)
	if err != nil {
		t.Fatalf("get resident: %v", err)
	}

	// 2. Create a Ketua RT with role="warga" and jabatan="ketua"
	var ketuaUserID, ketuaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Ketua RT Warga', NULL, true)
		RETURNING id
	`, fmt.Sprintf("ketua-rt-%d@local.com", time.Now().UnixNano())).Scan(&ketuaUserID)
	if err != nil {
		t.Fatalf("create ketua user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'ketua', true)
		RETURNING id
	`, ketuaUserID, rtID).Scan(&ketuaMID)
	if err != nil {
		t.Fatalf("create ketua membership: %v", err)
	}

	token := makeTestToken(t, ketuaUserID, ketuaMID, rtID, auth.RoleWarga, auth.JabatanKetua)

	// 3. Perform PATCH /api/v1/residents/{id}
	updateBody, _ := json.Marshal(map[string]any{
		"full_name": "Updated By Ketua RT",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+residentID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for Ketua RT updating resident, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify update in DB
	var updatedName string
	err = pool.Raw().QueryRowContext(ctx, `SELECT full_name FROM residents WHERE id = $1`, residentID).Scan(&updatedName)
	if err != nil {
		t.Fatalf("query updated resident: %v", err)
	}
	if updatedName != "Updated By Ketua RT" {
		t.Fatalf("resident name not updated, got: %s", updatedName)
	}
}

func TestWargaWithoutJabatanCannotUpdateResident(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID, residentID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id, rt_id FROM residents WHERE is_active = true LIMIT 1`).Scan(&residentID, &rtID)
	if err != nil {
		t.Fatalf("get resident: %v", err)
	}

	// Regular warga without jabatan
	var wargaUserID, wargaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Warga Biasa', NULL, true)
		RETURNING id
	`, fmt.Sprintf("warga-biasa-%d@local.com", time.Now().UnixNano())).Scan(&wargaUserID)
	if err != nil {
		t.Fatalf("create warga user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', NULL, true)
		RETURNING id
	`, wargaUserID, rtID).Scan(&wargaMID)
	if err != nil {
		t.Fatalf("create warga membership: %v", err)
	}

	token := makeTestToken(t, wargaUserID, wargaMID, rtID, auth.RoleWarga, "")

	updateBody, _ := json.Marshal(map[string]any{
		"full_name": "Should Fail",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+residentID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for regular warga updating resident, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBendaharaCannotDeleteFinancialCategories(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID, categoryID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id, rt_id FROM financial_categories WHERE is_active = true LIMIT 1`).Scan(&categoryID, &rtID)
	if err != nil {
		err = pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE is_active = true LIMIT 1`).Scan(&rtID)
		if err != nil {
			t.Fatalf("get rt: %v", err)
		}
		err = pool.Raw().QueryRowContext(ctx, `
			INSERT INTO financial_categories (id, rt_id, name, type, is_active)
			VALUES (gen_random_uuid(), $1, 'Kategori Test', 'expense', true)
			RETURNING id
		`, rtID).Scan(&categoryID)
		if err != nil {
			t.Fatalf("create category: %v", err)
		}
	}

	// Bendahara membership
	var bendaharaUserID, bendaharaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Bendahara RT', NULL, true)
		RETURNING id
	`, fmt.Sprintf("bendahara-rt-%d@local.com", time.Now().UnixNano())).Scan(&bendaharaUserID)
	if err != nil {
		t.Fatalf("create bendahara user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'bendahara', 'bendahara', true)
		RETURNING id
	`, bendaharaUserID, rtID).Scan(&bendaharaMID)
	if err != nil {
		t.Fatalf("create bendahara membership: %v", err)
	}

	token := makeTestToken(t, bendaharaUserID, bendaharaMID, rtID, auth.RoleBendahara, auth.JabatanBendahara)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/categories/"+categoryID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for bendahara deleting category, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestKetuaRTCanManageFinancialOperations(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt IN ('002', '003') AND is_active = true LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt: %v", err)
	}

	var ketuaUserID, ketuaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Ketua RT Ops', NULL, true)
		RETURNING id
	`, fmt.Sprintf("ketua-ops-%d@local.com", time.Now().UnixNano())).Scan(&ketuaUserID)
	if err != nil {
		t.Fatalf("create ketua user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'ketua', true)
		RETURNING id
	`, ketuaUserID, rtID).Scan(&ketuaMID)
	if err != nil {
		t.Fatalf("create ketua membership: %v", err)
	}

	token := makeTestToken(t, ketuaUserID, ketuaMID, rtID, auth.RoleWarga, auth.JabatanKetua)

	// Create due (financial operation)
	dueBody, _ := json.Marshal(map[string]any{
		"name":         "Iuran Kebersihan RT",
		"amount":       "25000",
		"period_type":  "monthly",
		"payment_rule": "partial",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewReader(dueBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201 Created for Ketua RT creating due, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResidentCreationRequiresWargaCreatePermission(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt IN ('002', '003') AND is_active = true LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt: %v", err)
	}

	// Create a user with a position that has warga.read but NOT warga.create (e.g. keamanan)
	var keamananUserID, keamananMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Petugas Keamanan', NULL, true)
		RETURNING id
	`, fmt.Sprintf("keamanan-%d@local.com", time.Now().UnixNano())).Scan(&keamananUserID)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'keamanan', true)
		RETURNING id
	`, keamananUserID, rtID).Scan(&keamananMID)
	if err != nil {
		t.Fatalf("create membership: %v", err)
	}

	token := makeTestToken(t, keamananUserID, keamananMID, rtID, auth.RoleWarga, auth.JabatanKeamanan)

	createBody, _ := json.Marshal(map[string]any{
		"full_name": "Penduduk Baru",
		"nik":       "3201010101019999",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for user without warga.create, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSekretarisRTCannotCreateOrUpdateWarga(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	// 1. Get an existing RT and resident
	var rtID, residentID string
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT res.rt_id, res.id
		FROM residents res
		JOIN rts r ON r.id = res.rt_id
		WHERE res.is_active = true AND r.is_active = true
		LIMIT 1
	`).Scan(&rtID, &residentID)
	if err != nil {
		t.Fatalf("find existing resident: %v", err)
	}

	// 2. Create a user with jabatan = sekretaris
	var sekUserID, sekMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Sekretaris RT Test', NULL, true)
		RETURNING id
	`, fmt.Sprintf("sekretaris-test-%d@local.com", time.Now().UnixNano())).Scan(&sekUserID)
	if err != nil {
		t.Fatalf("create sekretaris user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'pengurus', 'sekretaris', true)
		RETURNING id
	`, sekUserID, rtID).Scan(&sekMID)
	if err != nil {
		t.Fatalf("create sekretaris membership: %v", err)
	}

	token := makeTestToken(t, sekUserID, sekMID, rtID, auth.RolePengurus, auth.JabatanSekretaris)

	// A. Create resident -> 403 Forbidden
	createResidentBody, _ := json.Marshal(map[string]any{
		"full_name": "Penduduk Baru Oleh Sekretaris",
		"nik":       "3201010101018888",
	})
	reqCreateRes := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(createResidentBody))
	reqCreateRes.Header.Set("Authorization", "Bearer "+token)
	reqCreateRes.Header.Set("Content-Type", "application/json")
	recCreateRes := httptest.NewRecorder()
	router.ServeHTTP(recCreateRes, reqCreateRes)
	if recCreateRes.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Sekretaris POST /api/v1/residents, got %d: %s", recCreateRes.Code, recCreateRes.Body.String())
	}

	// B. Create household -> 403 Forbidden
	createHhBody, _ := json.Marshal(map[string]any{
		"house_number": "A-999",
	})
	reqCreateHh := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewReader(createHhBody))
	reqCreateHh.Header.Set("Authorization", "Bearer "+token)
	reqCreateHh.Header.Set("Content-Type", "application/json")
	recCreateHh := httptest.NewRecorder()
	router.ServeHTTP(recCreateHh, reqCreateHh)
	if recCreateHh.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Sekretaris POST /api/v1/households, got %d: %s", recCreateHh.Code, recCreateHh.Body.String())
	}

	// C. Update resident -> 403 Forbidden
	updateResBody, _ := json.Marshal(map[string]any{
		"full_name": "Nama Diubah Oleh Sekretaris",
	})
	reqUpdateRes := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+residentID, bytes.NewReader(updateResBody))
	reqUpdateRes.Header.Set("Authorization", "Bearer "+token)
	reqUpdateRes.Header.Set("Content-Type", "application/json")
	recUpdateRes := httptest.NewRecorder()
	router.ServeHTTP(recUpdateRes, reqUpdateRes)
	if recUpdateRes.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Sekretaris PATCH /api/v1/residents/{id}, got %d: %s", recUpdateRes.Code, recUpdateRes.Body.String())
	}

	// D. Move resident -> 403 Forbidden
	moveResBody, _ := json.Marshal(map[string]any{
		"move_date": "2026-11-15",
	})
	reqMoveRes := httptest.NewRequest(http.MethodPost, "/api/v1/residents/"+residentID+"/move", bytes.NewReader(moveResBody))
	reqMoveRes.Header.Set("Authorization", "Bearer "+token)
	reqMoveRes.Header.Set("Content-Type", "application/json")
	recMoveRes := httptest.NewRecorder()
	router.ServeHTTP(recMoveRes, reqMoveRes)
	if recMoveRes.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Sekretaris POST /api/v1/residents/{id}/move, got %d: %s", recMoveRes.Code, recMoveRes.Body.String())
	}

	// E. List residents -> 200 OK (warga.read is preserved!)
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/residents", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Sekretaris GET /api/v1/residents (warga.read preserved), got %d: %s", recList.Code, recList.Body.String())
	}
}

func TestKetuaRTCannotUpdateResidentInAnotherRT(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	// 1. Get an existing resident and its RT
	var residentID, residentRTID string
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT res.id, res.rt_id 
		FROM residents res 
		JOIN rts r ON r.id = res.rt_id 
		WHERE r.rt IN ('002', '003') AND res.is_active = true AND r.is_active = true 
		LIMIT 1
	`).Scan(&residentID, &residentRTID)
	if err != nil {
		t.Fatalf("get resident: %v", err)
	}

	// 2. Find an other RT in AllowedRTCodes
	var otherRTID string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT id 
		FROM rts 
		WHERE id != $1 AND is_active = true AND rt IN ('002', '003') 
		LIMIT 1
	`, residentRTID).Scan(&otherRTID)
	if err != nil {
		t.Fatalf("get other rt: %v", err)
	}

	// Create Ketua RT in the other RT
	var ketuaUserID, ketuaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Ketua Other RT', NULL, true)
		RETURNING id
	`, fmt.Sprintf("ketua-other-%d@local.com", time.Now().UnixNano())).Scan(&ketuaUserID)
	if err != nil {
		t.Fatalf("create ketua user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'ketua', true)
		RETURNING id
	`, ketuaUserID, otherRTID).Scan(&ketuaMID)
	if err != nil {
		t.Fatalf("create ketua membership: %v", err)
	}

	token := makeTestToken(t, ketuaUserID, ketuaMID, otherRTID, auth.RoleWarga, auth.JabatanKetua)

	// Attempt to PATCH resident in original RT using other RT's Ketua credentials
	updateBody, _ := json.Marshal(map[string]any{
		"full_name": "Hacked Cross RT",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+residentID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Cross-RT update must be rejected (404 Not Found within tenant's RT scope or 403)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 404 or 403 for cross-RT resident update, got %d: %s", rec.Code, rec.Body.String())
	}
}
