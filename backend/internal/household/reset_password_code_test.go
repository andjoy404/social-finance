package household_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/household"
	"social-finance/internal/testutil"
)

func setupTestUser(t *testing.T, pool *database.Pool, id, email, name string) {
	_, err := pool.Raw().Exec(`
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, 'dummyhash', $3, true)
		ON CONFLICT (id) DO UPDATE SET is_active = true, email = EXCLUDED.email
	`, id, email, name)
	if err != nil {
		t.Fatalf("setup test user %s: %v", id, err)
	}
}

func getOrCreateAllowedRT(t *testing.T, pool *database.Pool) string {
	var rtID string
	err := pool.Raw().QueryRow(`
		SELECT id FROM rts WHERE is_active = true AND rt IN ('002', '003') LIMIT 1
	`).Scan(&rtID)
	if err == nil && rtID != "" {
		return rtID
	}

	rtID = "99990002-0002-0002-0002-000000000002"
	_, err = pool.Raw().Exec(`
		INSERT INTO rts (id, name, rw, rt, is_active)
		VALUES ($1, 'RT 002 Test', 1, '002', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true, rt = '002'
	`, rtID)
	if err != nil {
		t.Fatalf("create allowed rt: %v", err)
	}
	return rtID
}

func TestGenerateResetPasswordCode_RBACMatrix(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtID := getOrCreateAllowedRT(t, pool)

	// Setup Warga resident target
	targetUserID := "34343434-3434-3434-3434-343434343434"
	targetEmail := "warga.rbac.target@example.com"
	setupTestUser(t, pool, targetUserID, targetEmail, "Warga Target RBAC")

	_, _ = pool.Raw().Exec(`
		INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		VALUES ($1, $2, 'warga', true)
		ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
	`, targetUserID, rtID)

	resID := "56565656-5656-5656-5656-565656565656"
	_, err := pool.Raw().Exec(`
		INSERT INTO residents (id, rt_id, user_id, full_name, email, is_active)
		VALUES ($1, $2, $3, 'Warga Target RBAC', $4, true)
		ON CONFLICT (id) DO UPDATE SET rt_id = EXCLUDED.rt_id, user_id = EXCLUDED.user_id, full_name = EXCLUDED.full_name, email = EXCLUDED.email, is_active = true
	`, resID, rtID, targetUserID, targetEmail)
	if err != nil {
		t.Fatalf("setup resident: %v", err)
	}

	h := household.NewHandler(pool)
	r := chi.NewRouter()
	r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)

	testCases := []struct {
		name         string
		userID       string
		authContext  *auth.AuthContext
		expectedCode int
	}{
		{
			name:   "SuperAdmin passes",
			userID: "a1a1a1a1-1111-1111-1111-111111111111",
			authContext: &auth.AuthContext{
				UserID:     "a1a1a1a1-1111-1111-1111-111111111111",
				SystemRole: auth.SystemRoleSuperAdmin,
			},
			expectedCode: http.StatusOK,
		},
		{
			name:   "Role Pengurus passes",
			userID: "a2a2a2a2-2222-2222-2222-222222222222",
			authContext: &auth.AuthContext{
				UserID:     "a2a2a2a2-2222-2222-2222-222222222222",
				RTID:       rtID,
				TenantRole: auth.RolePengurus,
			},
			expectedCode: http.StatusOK,
		},
		{
			name:   "Role Bendahara passes",
			userID: "a3a3a3a3-3333-3333-3333-333333333333",
			authContext: &auth.AuthContext{
				UserID:     "a3a3a3a3-3333-3333-3333-333333333333",
				RTID:       rtID,
				TenantRole: auth.RoleBendahara,
			},
			expectedCode: http.StatusOK,
		},
		{
			name:   "Jabatan Ketua RT passes (reproducing production bug fix)",
			userID: "a4a4a4a4-4444-4444-4444-444444444444",
			authContext: &auth.AuthContext{
				UserID:     "a4a4a4a4-4444-4444-4444-444444444444",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
				Jabatan:    auth.JabatanKetua,
			},
			expectedCode: http.StatusOK,
		},
		{
			name:   "Jabatan Wakil Ketua RT passes",
			userID: "a5a5a5a5-5555-5555-5555-555555555555",
			authContext: &auth.AuthContext{
				UserID:     "a5a5a5a5-5555-5555-5555-555555555555",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
				Jabatan:    auth.JabatanWakilKetua,
			},
			expectedCode: http.StatusOK,
		},
		{
			name:   "Jabatan Sekretaris RT passes",
			userID: "a6a6a6a6-6666-6666-6666-666666666666",
			authContext: &auth.AuthContext{
				UserID:     "a6a6a6a6-6666-6666-6666-666666666666",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
				Jabatan:    auth.JabatanSekretaris,
			},
			expectedCode: http.StatusOK,
		},
		{
			name:   "Jabatan Bendahara RT passes",
			userID: "a7a7a7a7-7777-7777-7777-777777777777",
			authContext: &auth.AuthContext{
				UserID:     "a7a7a7a7-7777-7777-7777-777777777777",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
				Jabatan:    auth.JabatanBendahara,
			},
			expectedCode: http.StatusOK,
		},
		{
			name:   "Regular Warga (no jabatan) forbidden",
			userID: "a8a8a8a8-8888-8888-8888-888888888888",
			authContext: &auth.AuthContext{
				UserID:     "a8a8a8a8-8888-8888-8888-888888888888",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
			},
			expectedCode: http.StatusForbidden,
		},
		{
			name:   "Jabatan Keamanan forbidden",
			userID: "a9a9a9a9-9999-9999-9999-999999999999",
			authContext: &auth.AuthContext{
				UserID:     "a9a9a9a9-9999-9999-9999-999999999999",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
				Jabatan:    auth.JabatanKeamanan,
			},
			expectedCode: http.StatusForbidden,
		},
		{
			name:   "Jabatan Kebersihan forbidden",
			userID: "b1b1b1b1-1111-1111-1111-111111111111",
			authContext: &auth.AuthContext{
				UserID:     "b1b1b1b1-1111-1111-1111-111111111111",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
				Jabatan:    auth.JabatanKebersihan,
			},
			expectedCode: http.StatusForbidden,
		},
		{
			name:   "Jabatan Operasional forbidden",
			userID: "b2b2b2b2-2222-2222-2222-222222222222",
			authContext: &auth.AuthContext{
				UserID:     "b2b2b2b2-2222-2222-2222-222222222222",
				RTID:       rtID,
				TenantRole: auth.RoleWarga,
				Jabatan:    auth.JabatanOperasional,
			},
			expectedCode: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupTestUser(t, pool, tc.userID, fmt.Sprintf("%s@test.com", tc.userID), tc.name)

			req := httptest.NewRequest("POST", "/api/v1/residents/"+resID+"/reset-password-code", nil)
			req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, tc.authContext))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tc.expectedCode, w.Code, w.Body.String())
			}

			if tc.expectedCode == http.StatusOK {
				var resData map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &resData); err != nil {
					t.Fatalf("unmarshal json: %v", err)
				}
				code, ok := resData["code"].(string)
				if !ok || len(code) != 4 {
					t.Fatalf("expected 4-digit code string, got %v", resData["code"])
				}
				// Verify code is hashed in db
				var codeHash string
				err = pool.Raw().QueryRowContext(ctx, `
					SELECT code_hash FROM resident_password_reset_codes
					WHERE user_id = $1 AND used_at IS NULL
					ORDER BY created_at DESC LIMIT 1
				`, targetUserID).Scan(&codeHash)
				if err != nil {
					t.Fatalf("lookup code hash: %v", err)
				}
				if err := auth.VerifyCode(code, codeHash); err != nil {
					t.Fatalf("code verification failed: %v", err)
				}
			}
		})
	}
}

func TestGenerateResetPasswordCode_CrossTenantIsolation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Use two distinct RTs from DB, or create with random numbers
	var rtAID, rtBID string
	err := pool.Raw().QueryRowContext(ctx, `
		SELECT a.id, b.id
		FROM rts a
		JOIN rts b ON a.id != b.id
		WHERE a.is_active = true AND b.is_active = true
		LIMIT 1
	`).Scan(&rtAID, &rtBID)
	if err != nil {
		suffix := time.Now().UnixNano() % 100000
		rtAID = fmt.Sprintf("11111111-aaaa-aaaa-aaaa-%012d", suffix)
		rtBID = fmt.Sprintf("22222222-bbbb-bbbb-bbbb-%012d", suffix)
		_, _ = pool.Raw().ExecContext(ctx, `
			INSERT INTO rts (id, name, rw, rt, is_active)
			VALUES ($1, 'RT Isolation A', 9000, $2, true)
		`, rtAID, fmt.Sprintf("%03d", suffix%1000))
		_, _ = pool.Raw().ExecContext(ctx, `
			INSERT INTO rts (id, name, rw, rt, is_active)
			VALUES ($1, 'RT Isolation B', 9001, $2, true)
		`, rtBID, fmt.Sprintf("%03d", (suffix+1)%1000))
	}

	officerUID := "91919191-9191-9191-9191-919191919191"
	setupTestUser(t, pool, officerUID, "officer.rta@example.com", "Officer RT A")

	// Create resident in RT B
	resBID := "bbbb1111-2222-3333-4444-555566667777"
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO residents (id, rt_id, full_name, is_active)
		VALUES ($1, $2, 'Resident in RT B', true)
		ON CONFLICT (id) DO UPDATE SET rt_id = EXCLUDED.rt_id, is_active = true
	`, resBID, rtBID)
	if err != nil {
		t.Fatalf("create resident in RT B: %v", err)
	}

	h := household.NewHandler(pool)
	r := chi.NewRouter()
	r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)

	// Officer of RT A attempts to reset password for resident in RT B
	req := httptest.NewRequest("POST", "/api/v1/residents/"+resBID+"/reset-password-code", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID:     officerUID,
		RTID:       rtAID,
		TenantRole: auth.RolePengurus,
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Must be 404 Not Found (cannot see or mutate other RT's data)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected cross-tenant access to return 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGenerateResetPasswordCode_FullMiddlewareChain(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Set AuthDBPool so RequireAuth validates against pool
	auth.AuthDBPool = pool

	rtID := getOrCreateAllowedRT(t, pool)

	// 1. Setup Ketua RT user with position permission
	ketuaUID := "77777777-7777-7777-7777-777777777777"
	ketuaMID := "88888888-8888-8888-8888-888888888888"
	passHash, _ := auth.Hash("Password123")

	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, 'ketua.full@example.com', $2, 'Pak Ketua RT', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, ketuaUID, passHash)
	if err != nil {
		t.Fatalf("setup ketua user: %v", err)
	}

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'ketua', true)
		ON CONFLICT (id) DO UPDATE SET role = 'warga', jabatan = 'ketua', is_active = true, rt_id = $3
	`, ketuaMID, ketuaUID, rtID)
	if err != nil {
		t.Fatalf("setup ketua membership: %v", err)
	}

	// Ensure position_permissions has warga.update for ketua
	_, _ = pool.Raw().ExecContext(ctx, `
		INSERT INTO position_permissions (rt_id, jabatan, permission)
		VALUES ($1, 'ketua', 'warga.update')
		ON CONFLICT DO NOTHING
	`, rtID)

	// Setup Keamanan user
	keamananUID := "99999999-9999-9999-9999-999999999999"
	keamananMID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, 'keamanan.full@example.com', $2, 'Pak Keamanan', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, keamananUID, passHash)
	if err != nil {
		t.Fatalf("setup keamanan user: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'keamanan', true)
		ON CONFLICT (id) DO UPDATE SET role = 'warga', jabatan = 'keamanan', is_active = true, rt_id = $3
	`, keamananMID, keamananUID, rtID)
	if err != nil {
		t.Fatalf("setup keamanan membership: %v", err)
	}

	// Setup target resident
	targetResID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	targetUID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	_, _ = pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, 'target.middleware@example.com', $2, 'Target Warga', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, targetUID, passHash)
	_, _ = pool.Raw().ExecContext(ctx, `
		INSERT INTO residents (id, rt_id, user_id, full_name, email, is_active)
		VALUES ($1, $2, $3, 'Target Warga', 'target.middleware@example.com', true)
		ON CONFLICT (id) DO UPDATE SET rt_id = EXCLUDED.rt_id, user_id = EXCLUDED.user_id, is_active = true
	`, targetResID, rtID, targetUID)

	// Build router with exact route middleware from main.go
	h := household.NewHandler(pool)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, auth.ScopingConfig{RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", "8dfa1e36-6727-45be-8fea-24d958efc155"}, RWIDs: []string{}}))
		r.Use(auth.RequirePermission(pool, "warga.update"))
		r.Use(auth.RequireAccess(
			[]auth.Role{auth.RolePengurus, auth.RoleBendahara},
			[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanSekretaris, auth.JabatanBendahara},
		))
		r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)
	})

	// 1. Ketua RT JWT token
	ketuaToken, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  ketuaUID,
		MID:     ketuaMID,
		RTID:    rtID,
		Role:    "warga",
		Jabatan: "ketua",
	})
	if err != nil {
		t.Fatalf("generate ketua token: %v", err)
	}

	reqKetua := httptest.NewRequest("POST", "/api/v1/residents/"+targetResID+"/reset-password-code", nil)
	reqKetua.Header.Set("Authorization", "Bearer "+ketuaToken)
	wKetua := httptest.NewRecorder()
	r.ServeHTTP(wKetua, reqKetua)

	if wKetua.Code != http.StatusOK {
		t.Fatalf("expected Ketua RT to pass full middleware chain with 200 OK, got %d: %s", wKetua.Code, wKetua.Body.String())
	}

	// 2. Keamanan JWT token (should be blocked with 403 Forbidden by middleware)
	keamananToken, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  keamananUID,
		MID:     keamananMID,
		RTID:    rtID,
		Role:    "warga",
		Jabatan: "keamanan",
	})
	if err != nil {
		t.Fatalf("generate keamanan token: %v", err)
	}

	reqKeamanan := httptest.NewRequest("POST", "/api/v1/residents/"+targetResID+"/reset-password-code", nil)
	reqKeamanan.Header.Set("Authorization", "Bearer "+keamananToken)
	wKeamanan := httptest.NewRecorder()
	r.ServeHTTP(wKeamanan, reqKeamanan)

	if wKeamanan.Code != http.StatusForbidden {
		t.Fatalf("expected Keamanan to be rejected with 403 Forbidden, got %d: %s", wKeamanan.Code, wKeamanan.Body.String())
	}

	// 3. Inactive membership token
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE user_rt_memberships SET is_active = false WHERE id = $1`, ketuaMID)
	wInactive := httptest.NewRecorder()
	r.ServeHTTP(wInactive, reqKetua)
	if wInactive.Code != http.StatusForbidden && wInactive.Code != http.StatusUnauthorized {
		t.Fatalf("expected inactive membership to be rejected, got %d", wInactive.Code)
	}
	// Restore membership
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE user_rt_memberships SET is_active = true WHERE id = $1`, ketuaMID)
}

func TestGenerateResetPasswordCode_ResidentWithoutUserAutoProvision(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtID := getOrCreateAllowedRT(t, pool)

	officerUID := "51515151-5151-5151-5151-515151515151"
	setupTestUser(t, pool, officerUID, "officer.auto@example.com", "Officer Auto")

	// Create resident without user_id
	orphanResID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	orphanPhone := "081299990001"
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO residents (id, rt_id, user_id, full_name, phone, is_active)
		VALUES ($1, $2, NULL, 'Orphan Resident', $3, true)
		ON CONFLICT (id) DO UPDATE SET rt_id = EXCLUDED.rt_id, user_id = NULL, phone = $3, is_active = true
	`, orphanResID, rtID, orphanPhone)
	if err != nil {
		t.Fatalf("setup orphan resident: %v", err)
	}

	h := household.NewHandler(pool)
	r := chi.NewRouter()
	r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)

	req := httptest.NewRequest("POST", "/api/v1/residents/"+orphanResID+"/reset-password-code", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID:     officerUID,
		RTID:       rtID,
		TenantRole: auth.RolePengurus,
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for resident without user_id, got %d: %s", w.Code, w.Body.String())
	}

	// Check that resident now has user_id linked
	var linkedUID *string
	err = pool.Raw().QueryRowContext(ctx, `SELECT user_id FROM residents WHERE id = $1`, orphanResID).Scan(&linkedUID)
	if err != nil || linkedUID == nil {
		t.Fatalf("expected resident to be linked to a new user account, got %v", linkedUID)
	}
}

func TestGenerateResetPasswordCode_PIIAndSecretSafety(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtID := getOrCreateAllowedRT(t, pool)

	officerUID := "61616161-6161-6161-6161-616161616161"
	setupTestUser(t, pool, officerUID, "officer.pii@example.com", "Officer PII")

	safeResID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	safeUID := "00000000-0000-0000-0000-000000000001"
	passHash, _ := auth.Hash("SecretPass#999")
	nikVal := "3201010101010001"

	_, _ = pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, 'pii.safe@example.com', $2, 'PII Safe Resident', true)
		ON CONFLICT (id) DO UPDATE SET is_active = true
	`, safeUID, passHash)

	_, _ = pool.Raw().ExecContext(ctx, `
		INSERT INTO residents (id, rt_id, user_id, full_name, email, nik, is_active)
		VALUES ($1, $2, $3, 'PII Safe Resident', 'pii.safe@example.com', $4, true)
		ON CONFLICT (id) DO UPDATE SET rt_id = EXCLUDED.rt_id, nik = $4, user_id = $3, is_active = true
	`, safeResID, rtID, safeUID, nikVal)

	h := household.NewHandler(pool)
	r := chi.NewRouter()
	r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)

	req := httptest.NewRequest("POST", "/api/v1/residents/"+safeResID+"/reset-password-code", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID:     officerUID,
		RTID:       rtID,
		TenantRole: auth.RolePengurus,
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	bodyStr := w.Body.String()
	// Must NOT contain password, password hash, or NIK
	if contains(bodyStr, "SecretPass") || contains(bodyStr, passHash) {
		t.Fatalf("response leaks password or password hash: %s", bodyStr)
	}
	if contains(bodyStr, nikVal) {
		t.Fatalf("response leaks full NIK: %s", bodyStr)
	}

	var resData map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resData)
	expiresAtStr, _ := resData["expires_at"].(string)
	expTime, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		t.Fatalf("invalid expires_at format: %v", expiresAtStr)
	}
	// Validity should be ~24 hours from now
	if expTime.Before(time.Now().Add(23 * time.Hour)) {
		t.Fatalf("expires_at should be roughly 24 hours in future, got %v", expTime)
	}
}

func TestGenerateResetPasswordCode_InvalidationAndReissue(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtID := getOrCreateAllowedRT(t, pool)

	targetUID := "71717171-7171-7171-7171-717171717171"
	targetEmail := "reissue.target@example.com"
	setupTestUser(t, pool, targetUID, targetEmail, "Reissue Target")

	resID := "81818181-8181-8181-8181-818181818181"
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO residents (id, rt_id, user_id, full_name, email, is_active)
		VALUES ($1, $2, $3, 'Reissue Target', $4, true)
		ON CONFLICT (id) DO UPDATE SET rt_id = EXCLUDED.rt_id, user_id = EXCLUDED.user_id, is_active = true
	`, resID, rtID, targetUID, targetEmail)
	if err != nil {
		t.Fatalf("setup resident: %v", err)
	}

	officerUID := "92929292-9292-9292-9292-929292929292"
	setupTestUser(t, pool, officerUID, "officer.reissue@example.com", "Officer Reissue")

	h := household.NewHandler(pool)
	r := chi.NewRouter()
	r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)

	reqGen := func() string {
		req := httptest.NewRequest("POST", "/api/v1/residents/"+resID+"/reset-password-code", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
			UserID:     officerUID,
			RTID:       rtID,
			TenantRole: auth.RolePengurus,
		}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("generate code failed: %d: %s", w.Code, w.Body.String())
		}
		var d map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &d)
		return d["code"].(string)
	}

	code1 := reqGen()
	if len(code1) != 4 {
		t.Fatalf("expected 4-digit code, got %s", code1)
	}

	// Verify 1 active code
	var activeCount int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM resident_password_reset_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, targetUID).Scan(&activeCount)
	if err != nil || activeCount != 1 {
		t.Fatalf("expected 1 active code, got %d (err: %v)", activeCount, err)
	}

	// Re-issue code
	code2 := reqGen()
	if len(code2) != 4 {
		t.Fatalf("expected 4-digit code, got %s", code2)
	}

	// Verify only 1 code remains active (code 1 invalidated)
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM resident_password_reset_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, targetUID).Scan(&activeCount)
	if err != nil || activeCount != 1 {
		t.Fatalf("expected exactly 1 active code after reissue, got %d (err: %v)", activeCount, err)
	}

	// Verify code 1 is marked used
	var usedCount int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM resident_password_reset_codes
		WHERE user_id = $1 AND used_at IS NOT NULL
	`, targetUID).Scan(&usedCount)
	if err != nil || usedCount < 1 {
		t.Fatalf("expected previous code to be marked used, got count %d", usedCount)
	}

	// Verify code 2 hash matches active code in DB
	var activeHash string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT code_hash FROM resident_password_reset_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, targetUID).Scan(&activeHash)
	if err != nil {
		t.Fatalf("query active hash: %v", err)
	}
	if err := auth.VerifyCode(code2, activeHash); err != nil {
		t.Fatalf("expected code 2 to match active hash: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && fmt.Sprintf("%v", s) != "" && (len(substr) == 0 || (len(s) >= len(substr) && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
