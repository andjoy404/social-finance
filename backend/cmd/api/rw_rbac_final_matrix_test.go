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

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/devseed"
	"social-finance/internal/notification"
	"social-finance/internal/rw"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

var rbacMatrixSeq uint64 = uint64(time.Now().UnixNano()%1000000 + 8000000)

func setupRBACMatrixRouter(t *testing.T, rwIDs []string) (*database.Pool, http.Handler) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("ensure devseed: %v", err)
	}

	// Ensure canonical permission types from migration 064 exist
	_, _ = pool.Raw().ExecContext(context.Background(), `
		INSERT INTO permission_types (id, code, description, module)
		VALUES
			(gen_random_uuid(), 'rw.dashboard.read', 'Lihat beranda ringkasan keuangan RW', 'rw'),
			(gen_random_uuid(), 'rw.kas.read', 'Lihat buku kas dan transaksi RW', 'rw'),
			(gen_random_uuid(), 'rw.kas.write', 'Kelola buku kas, catat transaksi kas dan saldo awal RW', 'rw'),
			(gen_random_uuid(), 'rw.fund_location.read', 'Lihat lokasi dana dan mutasi RW', 'rw'),
			(gen_random_uuid(), 'rw.fund_location.write', 'Kelola lokasi dana, transfer dan rekonsiliasi RW', 'rw'),
			(gen_random_uuid(), 'rw.notification.read', 'Lihat riwayat dan template notifikasi RW', 'rw'),
			(gen_random_uuid(), 'rw.notification.write', 'Kelola template dan kirim notifikasi manual RW', 'rw')
		ON CONFLICT (code) DO NOTHING;
	`)

	_, err := pool.Raw().ExecContext(context.Background(), `
		ALTER TABLE user_rw_memberships DROP CONSTRAINT IF EXISTS user_rw_memberships_jabatan_check;
		ALTER TABLE user_rw_memberships ADD CONSTRAINT user_rw_memberships_jabatan_check
			CHECK (jabatan IS NULL OR jabatan IN ('ketua', 'wakil_ketua', 'sekretaris', 'bendahara', 'anggota', 'pengurus'));

		-- Ensure migration 061 & 063 schema for RW notifications
		ALTER TABLE notification_logs ALTER COLUMN rt_id DROP NOT NULL;
		ALTER TABLE notification_logs ADD COLUMN IF NOT EXISTS rw_id UUID REFERENCES rws(id) ON DELETE CASCADE;
		CREATE INDEX IF NOT EXISTS idx_notification_logs_rw_id ON notification_logs(rw_id) WHERE rw_id IS NOT NULL;

		ALTER TABLE notification_templates ALTER COLUMN rt_id DROP NOT NULL;
		ALTER TABLE notification_templates ADD COLUMN IF NOT EXISTS rw_id UUID;
		ALTER TABLE notification_templates DROP CONSTRAINT IF EXISTS fk_notification_templates_rw_id;
		ALTER TABLE notification_templates ADD CONSTRAINT fk_notification_templates_rw_id FOREIGN KEY (rw_id) REFERENCES rws(id) ON DELETE CASCADE;
		CREATE INDEX IF NOT EXISTS idx_notification_templates_rw_id ON notification_templates(rw_id);
		ALTER TABLE notification_templates DROP CONSTRAINT IF EXISTS chk_notification_templates_rt_or_rw;
		ALTER TABLE notification_templates ADD CONSTRAINT chk_notification_templates_rt_or_rw CHECK (
			(rt_id IS NOT NULL AND rw_id IS NULL) OR
			(rt_id IS NULL AND rw_id IS NOT NULL)
		);
	`)
	require.NoError(t, err)

	auth.SigningSecret = []byte("test-secret-key-32-chars-long-ok!!")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	auth.AuthDBPool = pool

	authSvc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: 7 * 24 * time.Hour,
	})
	authHandler := auth.NewHandler(pool, authSvc)
	cfg := auth.ScopingConfig{
		RTIDs: []string{
			"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", // RT 002
			"8dfa1e36-6727-45be-8fea-24d958efc155", // RT 003
		},
		RWIDs: rwIDs,
	}
	router := buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
	return pool, router
}

func createRBACMatrixRW(t *testing.T, pool *database.Pool, name string) string {
	t.Helper()
	rwNum := int(atomic.AddUint64(&rbacMatrixSeq, 1))
	var rwID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rws (id, name, rw_number, is_active)
		 VALUES (gen_random_uuid(), $1, $2, true)
		 RETURNING id`,
		name, rwNum,
	).Scan(&rwID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_notification_templates WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_location_balance_changes WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_transactions WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_fund_transfers WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_fund_locations WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rw_opening_balances WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE rw_id = $1`, rwID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})
	return rwID
}

func createRBACMatrixRT(t *testing.T, pool *database.Pool, rwID, name string) string {
	t.Helper()
	rtNum := fmt.Sprintf("%03d", atomic.AddUint64(&rbacMatrixSeq, 1)%900+100)
	var rtID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rts (id, name, rw, rt, rw_id, is_active)
		 VALUES (gen_random_uuid(), $1, 100, $2, $3, true)
		 RETURNING id`,
		name, rtNum, rwID,
	).Scan(&rtID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id = $1`, rtID)
		pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
	})
	return rtID
}

func createRBACMatrixRWUserToken(t *testing.T, pool *database.Pool, rwID string, role auth.Role, jabatan string) string {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rw-user-%s-%d@test.com", jabatan, atomic.AddUint64(&rbacMatrixSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', $2, NULL, true)
		 RETURNING id`,
		email, "RW "+jabatan,
	).Scan(&userID)
	require.NoError(t, err)

	var membershipID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rw_memberships (id, user_id, rw_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true)
		 RETURNING id`,
		userID, rwID, string(role), jabatan,
	).Scan(&membershipID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE id = $1`, membershipID)
		pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	rwContextJSON := fmt.Sprintf(`[{"rw_id":"%s","jabatan":"%s"}]`, rwID, jabatan)
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:    userID,
		Role:      string(role),
		Jabatan:   jabatan,
		RWContext: rwContextJSON,
	})
	require.NoError(t, err)
	return token
}

func createRBACMatrixRTUserToken(t *testing.T, pool *database.Pool, rtID string, role auth.Role, jabatan string) string {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rt-user-%s-%d@test.com", jabatan, atomic.AddUint64(&rbacMatrixSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', $2, NULL, true)
		 RETURNING id`,
		email, "RT "+jabatan,
	).Scan(&userID)
	require.NoError(t, err)

	var membershipID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true)
		 RETURNING id`,
		userID, rtID, string(role), jabatan,
	).Scan(&membershipID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, membershipID)
		pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     membershipID,
		RTID:    rtID,
		Role:    string(role),
		Jabatan: jabatan,
	})
	require.NoError(t, err)
	return token
}

func createRBACMatrixSuperAdminToken(t *testing.T, pool *database.Pool) string {
	t.Helper()
	var userID string
	email := fmt.Sprintf("sa-%d@test.com", atomic.AddUint64(&rbacMatrixSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'Super Admin', 'super_admin', true)
		 RETURNING id`,
		email,
	).Scan(&userID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		SysRole: "super_admin",
		Role:    "super_admin",
	})
	require.NoError(t, err)
	return token
}

func createRBACMatrixUnmappedUserToken(t *testing.T, pool *database.Pool) string {
	t.Helper()
	var userID string
	email := fmt.Sprintf("unmapped-%d@test.com", atomic.AddUint64(&rbacMatrixSeq, 1))
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'Unmapped User', NULL, true)
		 RETURNING id`,
		email,
	).Scan(&userID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID: userID,
		Role:   "warga",
	})
	require.NoError(t, err)
	return token
}

// -----------------------------------------------------------------------------
// COMPREHENSIVE FINAL MATRIX RBAC TEST
// -----------------------------------------------------------------------------
// Verifies all 10 personas across all 4 RW features:
// A. Beranda RW (dashboard)
// B. KAS RW (report & transaction write)
// C. Lokasi Dana RW (locations & location write)
// D. Notifikasi RW (logs & manual-push / template write)
// -----------------------------------------------------------------------------
func TestRWRBAC_FinalMatrix_AllPersonasAllFeatures(t *testing.T) {
	pool := testutil.GetTestPool(t)

	rwAID := createRBACMatrixRW(t, pool, "RW Alpha Matrix")
	rwBID := createRBACMatrixRW(t, pool, "RW Beta Matrix")
	rtAID := createRBACMatrixRT(t, pool, rwAID, "RT Alpha Matrix")

	_, router := setupRBACMatrixRouter(t, []string{rwAID, rwBID})

	// Create an initial fund location in RW A so that kas transactions can reference it
	var locAID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rw_fund_locations (id, rw_id, name, category, bank_name, is_active)
		 VALUES (gen_random_uuid(), $1, 'Rekening Kas RW Alpha', 'rekening', 'BCA', true)
		 RETURNING id`,
		rwAID,
	).Scan(&locAID)
	require.NoError(t, err)

	// 10 Personas
	personaTokens := map[string]string{
		"1. Ketua RW A":              createRBACMatrixRWUserToken(t, pool, rwAID, auth.RolePengurus, "ketua"),
		"2. Bendahara RW A":          createRBACMatrixRWUserToken(t, pool, rwAID, auth.RoleBendahara, "bendahara"),
		"3. Wakil Ketua RW A":        createRBACMatrixRWUserToken(t, pool, rwAID, auth.RolePengurus, "wakil_ketua"),
		"4. Sekretaris RW A":         createRBACMatrixRWUserToken(t, pool, rwAID, auth.RolePengurus, "sekretaris"),
		"5. Pengurus RW A (Anggota)": createRBACMatrixRWUserToken(t, pool, rwAID, auth.RolePengurus, "anggota"),
		"6. RT Warga under RW A":     createRBACMatrixRTUserToken(t, pool, rtAID, auth.RoleWarga, "anggota"),
		"7. Pengurus RT (Inferred)":  createRBACMatrixRTUserToken(t, pool, rtAID, auth.RolePengurus, "ketua"),
		"8. User Without RW/RT":      createRBACMatrixUnmappedUserToken(t, pool),
		"9. Ketua RW B (Cross-RW)":   createRBACMatrixRWUserToken(t, pool, rwBID, auth.RolePengurus, "ketua"),
		"10. Super Admin":            createRBACMatrixSuperAdminToken(t, pool),
	}

	// Helper to send HTTP requests
	sendReq := func(method, path, token string, body any) *httptest.ResponseRecorder {
		var reqBody *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			reqBody = bytes.NewReader(b)
		} else {
			reqBody = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, path, reqBody)
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// =========================================================================
	// FEATURE A: BERANDA RW (Dashboard)
	// =========================================================================
	t.Run("Feature A - Beranda RW", func(t *testing.T) {
		dashboardPath := fmt.Sprintf("/api/v1/rw/%s/dashboard", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				rec := sendReq("GET", dashboardPath, token, nil)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "3. Wakil Ketua RW A",
					"4. Sekretaris RW A", "5. Pengurus RW A (Anggota)",
					"7. Pengurus RT (Inferred)", "10. Super Admin":
					require.Equal(t, http.StatusOK, rec.Code, "should have read access to Beranda RW")
				case "6. RT Warga under RW A", "8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "should be forbidden from Beranda RW")
				}
			})
		}
	})

	// =========================================================================
	// FEATURE B: KAS RW (Read & Write)
	// =========================================================================
	t.Run("Feature B - KAS RW Read", func(t *testing.T) {
		kasPath := fmt.Sprintf("/api/v1/rw/%s/kas", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				rec := sendReq("GET", kasPath, token, nil)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "3. Wakil Ketua RW A",
					"4. Sekretaris RW A", "5. Pengurus RW A (Anggota)", "10. Super Admin":
					require.Equal(t, http.StatusOK, rec.Code, "explicit RW managers and SA should have read access to KAS")
				case "6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "Pengurus RT and unauthorized users must be forbidden from KAS")
				}
			})
		}
	})

	t.Run("Feature B - KAS RW Write (Create Income)", func(t *testing.T) {
		kasPath := fmt.Sprintf("/api/v1/rw/%s/kas", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				input := rw.CreateTransactionInput{
					Type:        "income",
					Amount:      "500000.00",
					LocationID:  locAID,
					Description: "Kas masuk test from " + name,
				}
				rec := sendReq("POST", kasPath, token, input)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "10. Super Admin":
					require.Equal(t, http.StatusCreated, rec.Code, "Ketua, Bendahara, and SA must be able to write to KAS")
				case "3. Wakil Ketua RW A", "4. Sekretaris RW A", "5. Pengurus RW A (Anggota)",
					"6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "only Ketua and Bendahara may write to KAS; others must get 403")
				}
			})
		}
	})

	// =========================================================================
	// FEATURE C: LOKASI DANA RW (Read & Write)
	// =========================================================================
	t.Run("Feature C - Lokasi Dana RW Read", func(t *testing.T) {
		locationsPath := fmt.Sprintf("/api/v1/rw/%s/locations", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				rec := sendReq("GET", locationsPath, token, nil)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "3. Wakil Ketua RW A",
					"4. Sekretaris RW A", "5. Pengurus RW A (Anggota)", "10. Super Admin":
					require.Equal(t, http.StatusOK, rec.Code, "explicit RW managers and SA should have read access to Lokasi Dana")
				case "6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "Pengurus RT and unauthorized users must be forbidden from Lokasi Dana")
				}
			})
		}
	})

	t.Run("Feature C - Lokasi Dana RW Write (Create Location)", func(t *testing.T) {
		locationsPath := fmt.Sprintf("/api/v1/rw/%s/locations", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				locName := fmt.Sprintf("Brankas %s %d", name, time.Now().UnixNano()%10000)
				input := rw.CreateRWFundLocationInput{
					Name:     locName,
					Category: rw.CategoryBrankas,
				}
				rec := sendReq("POST", locationsPath, token, input)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "10. Super Admin":
					require.Equal(t, http.StatusCreated, rec.Code, "Ketua, Bendahara, and SA must be able to create fund location")
				case "3. Wakil Ketua RW A", "4. Sekretaris RW A", "5. Pengurus RW A (Anggota)",
					"6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "only Ketua and Bendahara may write fund locations; others must get 403")
				}
			})
		}
	})

	// =========================================================================
	// FEATURE D: NOTIFIKASI RW (Read & Write)
	// =========================================================================
	t.Run("Feature D - Notifikasi RW Read", func(t *testing.T) {
		notifPath := fmt.Sprintf("/api/v1/rw/%s/notifications", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				rec := sendReq("GET", notifPath, token, nil)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "3. Wakil Ketua RW A",
					"4. Sekretaris RW A", "5. Pengurus RW A (Anggota)", "10. Super Admin":
					require.Equal(t, http.StatusOK, rec.Code, "explicit RW managers and SA should have read access to Notifikasi RW")
				case "6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "Pengurus RT and unauthorized users must be forbidden from Notifikasi RW")
				}
			})
		}
	})

	t.Run("Feature D - Notifikasi RW Write (Manual Push)", func(t *testing.T) {
		manualPushPath := fmt.Sprintf("/api/v1/rw/%s/notifications/manual", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				input := rw.RWManualPushRequest{
					Title: "Pengumuman Warga",
					Body:  "Pesan manual push test from " + name,
				}
				rec := sendReq("POST", manualPushPath, token, input)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "10. Super Admin":
					require.Equal(t, http.StatusOK, rec.Code, "Ketua, Bendahara, and SA must be able to send manual push")
				case "3. Wakil Ketua RW A", "4. Sekretaris RW A", "5. Pengurus RW A (Anggota)",
					"6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "only Ketua and Bendahara may send manual push; others must get 403")
				}
			})
		}
	})

	t.Run("Feature D - Notifikasi RW Write (Create Template)", func(t *testing.T) {
		templatePath := fmt.Sprintf("/api/v1/rw/%s/notifications/templates", rwAID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				tmplName := fmt.Sprintf("Template %s %d", name, time.Now().UnixNano()%10000)
				input := notification.CreateTemplateInput{
					Name:      tmplName,
					EventType: "manual_push",
					Channel:   "push",
					Title:     "Pengingat Iuran",
					Body:      "Mohon selesaikan pembayaran",
				}
				rec := sendReq("POST", templatePath, token, input)
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "10. Super Admin":
					require.Equal(t, http.StatusCreated, rec.Code, "Ketua, Bendahara, and SA must be able to create template; got body: %s", rec.Body.String())
				case "3. Wakil Ketua RW A", "4. Sekretaris RW A", "5. Pengurus RW A (Anggota)",
					"6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "only Ketua and Bendahara may write templates; others must get 403; got body: %s", rec.Body.String())
				}
			})
		}
	})

	// Setup a dedicated template in RW A for Preview and Send testing
	var testTmplID string
	err = pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO notification_templates (id, rw_id, name, event_type, channel, title, body, is_active)
		 VALUES (gen_random_uuid(), $1, 'Template Iuran Preview', 'manual_push', 'push', 'Pengumuman {{rw_name}}', 'Halo {{nama}}, iuran {{period}} sebesar {{amount}} telah terbit.', true)
		 RETURNING id`,
		rwAID,
	).Scan(&testTmplID)
	require.NoError(t, err)

	t.Run("Feature D - Notifikasi RW Template Preview (Read Permission)", func(t *testing.T) {
		previewPath := fmt.Sprintf("/api/v1/rw/%s/notifications/templates/%s/preview", rwAID, testTmplID)

		// Check log count before preview
		var initialLogCount int
		_ = pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM notification_logs WHERE rw_id = $1`, rwAID).Scan(&initialLogCount)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				rec := sendReq("POST", previewPath, token, map[string]any{
					"variables": map[string]string{
						"rw_name": "RW 05 Maju",
					},
				})
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "3. Wakil Ketua RW A",
					"4. Sekretaris RW A", "5. Pengurus RW A (Anggota)", "10. Super Admin":
					require.Equal(t, http.StatusOK, rec.Code, "explicit RW members and SA must be able to preview template")
					var resp struct {
						Data struct {
							RenderedTitle string   `json:"rendered_title"`
							RenderedBody  string   `json:"rendered_body"`
							Placeholders  []string `json:"placeholders"`
						} `json:"data"`
					}
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
					require.Equal(t, "Pengumuman RW 05 Maju", resp.Data.RenderedTitle)
					require.Contains(t, resp.Data.RenderedBody, "Budi Santoso")
					require.Contains(t, resp.Data.RenderedBody, "November 2026")
					require.Contains(t, resp.Data.RenderedBody, "Rp 50.000")
				case "6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "unauthorized users and Pengurus RT must be forbidden from preview")
				}
			})
		}

		// Verify preview did NOT create any notification log
		var finalLogCount int
		_ = pool.Raw().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM notification_logs WHERE rw_id = $1`, rwAID).Scan(&finalLogCount)
		require.Equal(t, initialLogCount, finalLogCount, "preview template MUST NOT create notification logs")
	})

	t.Run("Feature D - Notifikasi RW Template Send (Write Permission)", func(t *testing.T) {
		sendPath := fmt.Sprintf("/api/v1/rw/%s/notifications/templates/%s/send", rwAID, testTmplID)

		for name, token := range personaTokens {
			t.Run(name, func(t *testing.T) {
				rec := sendReq("POST", sendPath, token, map[string]any{
					"target": "all_pengurus",
					"variables": map[string]string{
						"rw_name": "RW 05 Maju",
					},
				})
				switch name {
				case "1. Ketua RW A", "2. Bendahara RW A", "10. Super Admin":
					require.Equal(t, http.StatusOK, rec.Code, "Ketua, Bendahara, and SA must be able to send via template; body: %s", rec.Body.String())
					var resp struct {
						Data struct {
							SentCount int    `json:"sent_count"`
							Title     string `json:"title"`
							Body      string `json:"body"`
						} `json:"data"`
					}
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
					require.Equal(t, "Pengumuman RW 05 Maju", resp.Data.Title)
				case "3. Wakil Ketua RW A", "4. Sekretaris RW A", "5. Pengurus RW A (Anggota)",
					"6. RT Warga under RW A", "7. Pengurus RT (Inferred)",
					"8. User Without RW/RT", "9. Ketua RW B (Cross-RW)":
					require.Equal(t, http.StatusForbidden, rec.Code, "only Ketua, Bendahara, and SA may send via template; body: %s", rec.Body.String())
				}
			})
		}
	})

	t.Run("Feature D - Cross-RW Template Isolation", func(t *testing.T) {
		// Ketua RW B tries to preview RW A's template via RW B path
		crossPath := fmt.Sprintf("/api/v1/rw/%s/notifications/templates/%s/preview", rwBID, testTmplID)
		rec := sendReq("POST", crossPath, personaTokens["9. Ketua RW B (Cross-RW)"], nil)
		require.Equal(t, http.StatusNotFound, rec.Code, "template belonging to RW A must not be accessible in RW B")
	})
}
