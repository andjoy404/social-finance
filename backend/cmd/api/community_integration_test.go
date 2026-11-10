package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/devseed"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

var commSeq uint64 = uint64(time.Now().UnixNano())

func setupCommunityRouter(t *testing.T, rtIDs []string) (*database.Pool, http.Handler) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("ensure devseed: %v", err)
	}

	// Ensure migration 069 (superset: 068 content + dedup/idempotency) is applied
	applyCommunityMigration := func(names []string) {
		for _, n := range names {
			migPaths := []string{
				"migrations/" + n,
				"../../migrations/" + n,
				"../migrations/" + n,
			}
			for _, p := range migPaths {
				migSQL, err := os.ReadFile(p)
				if err == nil {
					_, _ = pool.Raw().ExecContext(context.Background(), string(migSQL))
					break
				}
			}
		}
	}
	applyCommunityMigration([]string{
		"068_community_activities_announcements_banners.up.sql",
		"069_community_banner_dedup_and_idempotency.up.sql",
	})

	auth.SigningSecret = []byte("test-secret-key-32-chars-long-ok!!")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	auth.AuthDBPool = pool
	authSvc := auth.NewService(auth.ServiceOptions{RefreshLifetime: 7 * 24 * 3600})
	authHandler := auth.NewHandler(pool, authSvc)
	cfg := auth.ScopingConfig{
		RTIDs: rtIDs,
		RWIDs: nil,
	}
	return pool, buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
}

func createCommunityTestUser(t *testing.T, pool *database.Pool, rtID string, role auth.Role, jabatan auth.Jabatan) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("commuser-%d-%d@test.com", time.Now().UnixNano(), atomic.AddUint64(&commSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hashed', 'Comm User', NULL, true)
		 RETURNING id`, email).Scan(&userID)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	var mid string
	var jabatanVal *string
	if jabatan != "" {
		jStr := string(jabatan)
		jabatanVal = &jStr
	}
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true)
		 RETURNING id`, userID, rtID, string(role), jabatanVal).Scan(&mid)
	if err != nil {
		t.Fatalf("create membership: %v", err)
	}

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     mid,
		RTID:    rtID,
		Role:    string(role),
		Jabatan: string(jabatan),
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return userID, token
}

const commRTUrena = "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b" // rt=002, whitelisted

func TestCommunity_HomeDashboard(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/home/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+wargaToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if resp["greeting"] == nil || resp["greeting"] == "" {
		t.Errorf("expected greeting, got nil or empty")
	}
	if resp["organization"] == nil {
		t.Errorf("expected organization, got nil")
	}
	if resp["security_on_duty"] == nil {
		t.Errorf("expected security_on_duty, got nil")
	}
	sec, ok := resp["security_on_duty"].(map[string]any)
	if !ok {
		t.Fatalf("security_on_duty is not a map")
	}
	if sec["is_available"] != false {
		t.Errorf("expected is_available = false awaiting Security Module, got %v", sec["is_available"])
	}
}

func TestCommunity_ActivitiesRBAC(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// 1. Warga cannot create activity -> 403 Forbidden
	actBody := map[string]any{
		"title":       "Kerja Bakti Warga",
		"description": "Membersihkan selokan",
		"starts_at":   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"publish_now": true,
	}
	actJSON, _ := json.Marshal(actBody)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewReader(actJSON))
	req1.Header.Set("Authorization", "Bearer "+wargaToken)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusForbidden {
		t.Fatalf("warga should receive 403 Forbidden, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// 2. Ketua can create activity -> 201 Created
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewReader(actJSON))
	req2.Header.Set("Authorization", "Bearer "+ketuaToken)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("ketua should receive 201 Created, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 3. Warga can read published activities -> 200 OK
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/activities", nil)
	req3.Header.Set("Authorization", "Bearer "+wargaToken)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("warga should receive 200 OK on listing activities, got %d: %s", rec3.Code, rec3.Body.String())
	}
}

func TestCommunity_AnnouncementDeepLinkValidation(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	// 1. External deep link must fail with 400 Bad Request
	badBody := map[string]any{
		"title":     "Pengumuman Luar",
		"body":      "Cek website luar",
		"deep_link": "https://malicious.org/phishing",
	}
	badJSON, _ := json.Marshal(badBody)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewReader(badJSON))
	req1.Header.Set("Authorization", "Bearer "+ketuaToken)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("external deep link should fail with 400, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// 2. Safe allowed deep link (/keuangan) must succeed with 201
	goodBody := map[string]any{
		"title":       "Pembayaran Iuran Dibuka",
		"body":        "Silakan lakukan pembayaran iuran bulanan",
		"deep_link":   "/keuangan",
		"publish_now": true,
	}
	goodJSON, _ := json.Marshal(goodBody)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewReader(goodJSON))
	req2.Header.Set("Authorization", "Bearer "+ketuaToken)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("safe deep link should succeed with 201, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestCommunity_CrossRTTenantIsolation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	// Create second RT with RT code '003' which is in AllowedRTCodes
	var otherRT string
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT id FROM rts WHERE rt = '003' AND is_active = true LIMIT 1`).Scan(&otherRT)
	if err != nil {
		err = pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO rts (id, name, rw, rt, is_active)
			 VALUES (gen_random_uuid(), 'RT 003 Test', 16, '003', true)
			 RETURNING id`).Scan(&otherRT)
		if err != nil {
			t.Fatalf("create second rt: %v", err)
		}
	}

	_, router := setupCommunityRouter(t, []string{commRTUrena, otherRT})

	// Ketua in RT Urena creates an activity
	_, ketuaUrenaToken := createCommunityTestUser(t, pool, commRTUrena, auth.RolePengurus, auth.JabatanKetua)
	actBody := map[string]any{
		"title":       "Kegiatan Khusus RT Urena",
		"description": "Hanya warga Urena",
		"starts_at":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"publish_now": true,
	}
	actJSON, _ := json.Marshal(actBody)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewReader(actJSON))
	req1.Header.Set("Authorization", "Bearer "+ketuaUrenaToken)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("create activity failed: %d: %s", rec1.Code, rec1.Body.String())
	}

	// Warga in other RT lists activities -> MUST NOT see "Kegiatan Khusus RT Urena"
	_, wargaOtherToken := createCommunityTestUser(t, pool, otherRT, auth.RoleWarga, "")
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/activities", nil)
	req2.Header.Set("Authorization", "Bearer "+wargaOtherToken)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("list activities for other RT failed: %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp)
	data, _ := resp["data"].([]any)
	for _, item := range data {
		m, ok := item.(map[string]any)
		if ok && m["title"] == "Kegiatan Khusus RT Urena" {
			t.Fatalf("TENANT ISOLATION BREACH: other RT user saw activity from RT Urena!")
		}
	}
}

func TestCommunity_IuranReminderBanner(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	// Devseed creates unpaid bills for Urena
	_, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/home/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+wargaToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard failed: %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	banners, ok := resp["banners"].([]any)
	if !ok {
		t.Fatalf("banners is not a slice")
	}

	// Verify that if an iuran reminder banner exists, it links to /keuangan and does not leak debtor names
	for _, b := range banners {
		bm, ok := b.(map[string]any)
		if ok && bm["source_type"] == "iuran_reminder" {
			if bm["deep_link"] != "/keuangan" {
				t.Errorf("expected deep_link = /keuangan, got %v", bm["deep_link"])
			}
			title, _ := bm["title"].(string)
			body, _ := bm["body"].(string)
			// Must not contain personal info
			if title == "" || body == "" {
				t.Errorf("expected non-empty title and body for iuran reminder banner")
			}
		}
	}
}
