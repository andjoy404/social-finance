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
	"social-finance/internal/devseed"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

var rwNotifSecSeq uint64 = uint64(time.Now().UnixNano()%1000000 + 7000000)

func setupRWNotifRouter(t *testing.T, rwIDs []string) (*database.Pool, http.Handler) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("ensure devseed: %v", err)
	}
	auth.SigningSecret = []byte("test-secret-key-32-chars-long-ok!!")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	auth.AuthDBPool = pool
	authSvc := auth.NewService(auth.ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	authHandler := auth.NewHandler(pool, authSvc)
	cfg := auth.ScopingConfig{
		RTIDs: []string{"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
		RWIDs: rwIDs,
	}
	return pool, buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
}

func createRWForNotifTest(t *testing.T, pool *database.Pool) string {
	t.Helper()
	rwNum := int(atomic.AddUint64(&rwNotifSecSeq, 1))
	var rwID string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rws (id, name, rw_number, is_active) VALUES (gen_random_uuid(), $1, $2, true) RETURNING id`,
		fmt.Sprintf("RW NotifSec %d", rwNum), rwNum).Scan(&rwID); err != nil {
		t.Fatalf("create rw: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM notification_templates WHERE rw_id = $1`, rwID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM notification_logs WHERE rw_id = $1`, rwID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE rw_id = $1`, rwID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})
	return rwID
}

func createRWTemplateForTest(t *testing.T, pool *database.Pool, rwID string) string {
	t.Helper()
	var id string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO notification_templates (rw_id, name, event_type, channel, title, body, is_active, variables, is_default)
		 VALUES ($1, $2, 'manual_push', 'push', 'Title', 'Body', true, '[]'::jsonb, false) RETURNING id`,
		rwID, fmt.Sprintf("T-%d", atomic.AddUint64(&rwNotifSecSeq, 1))).Scan(&id)
	if err != nil {
		t.Fatalf("create rw template: %v", err)
	}
	return id
}

func countRWTemplates(t *testing.T, pool *database.Pool, rwID string) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM notification_templates WHERE rw_id = $1`, rwID).Scan(&n); err != nil {
		t.Fatalf("count templates: %v", err)
	}
	return n
}

func countRWLogs(t *testing.T, pool *database.Pool, rwID string) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM notification_logs WHERE rw_id = $1`, rwID).Scan(&n); err != nil {
		// table may not exist in minimal env; treat as 0 for side-effect comparison only if schema absent
		t.Fatalf("count logs: %v", err)
	}
	return n
}

func templateSnapshot(t *testing.T, pool *database.Pool, id string) (name, title string, active bool) {
	t.Helper()
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT name, title, is_active FROM notification_templates WHERE id = $1`, id).Scan(&name, &title, &active); err != nil {
		t.Fatalf("template snapshot: %v", err)
	}
	return
}

func createRWExplicitUserToken(t *testing.T, pool *database.Pool, rwID string, role auth.Role, jabatan auth.Jabatan) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rwnotif-%s-%d@test.com", jabatan, atomic.AddUint64(&rwNotifSecSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RW Notif User', NULL, true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var mid string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rw_memberships (id, user_id, rw_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`,
		userID, rwID, string(role), string(jabatan)).Scan(&mid); err != nil {
		t.Fatalf("create membership: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE id = $1`, mid)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	rwCtx := fmt.Sprintf(`[{"rw_id":"%s","jabatan":"%s"}]`, rwID, string(jabatan))
	token, err := auth.GenerateAccessToken(auth.TokenClaims{UserID: userID, Role: string(role), Jabatan: string(jabatan), RWContext: rwCtx})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return userID, token
}

func createSAToken(t *testing.T, pool *database.Pool) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rwnotif-sa-%d@test.com", atomic.AddUint64(&rwNotifSecSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'SA Notif', 'super_admin', true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create sa: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Raw().ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })
	token, err := auth.GenerateAccessToken(auth.TokenClaims{UserID: userID, SysRole: string(auth.SystemRoleSuperAdmin)})
	if err != nil {
		t.Fatalf("sa token: %v", err)
	}
	return userID, token
}

func doJSON(router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func requireDeniedNoTemplateChange(t *testing.T, pool *database.Pool, rwID, templateID string, beforeCount int, beforeName, beforeTitle string, beforeActive bool, w *httptest.ResponseRecorder, label string) {
	t.Helper()
	if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Fatalf("%s: expected 401/403, got %d body=%s", label, w.Code, w.Body.String())
	}
	afterCount := countRWTemplates(t, pool, rwID)
	if afterCount != beforeCount {
		t.Fatalf("%s: template count changed %d -> %d", label, beforeCount, afterCount)
	}
	name, title, active := templateSnapshot(t, pool, templateID)
	if name != beforeName || title != beforeTitle || active != beforeActive {
		t.Fatalf("%s: template mutated: before=(%q,%q,%v) after=(%q,%q,%v)", label, beforeName, beforeTitle, beforeActive, name, title, active)
	}
}

// Route-actual RW notification mutation authorization + DB no-side-effect.
// Uses explicit RW membership (DB) + matching JWT claims; RequireRWScope + RW service enforce permissions.
func TestRWNotification_ReadOnlyExplicitDeniedMutations_NoSideEffects(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	// explicit sekretaris = read in RW matrix, no rw.notification.write
	_, tokenRO := createRWExplicitUserToken(t, pool, rwID, auth.RoleWarga, auth.JabatanSekretaris)
	templateID := createRWTemplateForTest(t, pool, rwID)

	beforeCount := countRWTemplates(t, pool, rwID)
	beforeName, beforeTitle, beforeActive := templateSnapshot(t, pool, templateID)
	beforeLogs := countRWLogs(t, pool, rwID)

	w := doJSON(router, http.MethodPost, "/api/v1/rw/"+rwID+"/notifications/manual", tokenRO, map[string]any{"title": "x", "body": "y"})
	if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Fatalf("manual push RO: got %d body=%s", w.Code, w.Body.String())
	}
	if after := countRWLogs(t, pool, rwID); after != beforeLogs {
		t.Fatalf("manual push side effect: logs %d -> %d", beforeLogs, after)
	}

	w = doJSON(router, http.MethodPost, "/api/v1/rw/"+rwID+"/notifications/templates", tokenRO, map[string]any{"name": "N", "event_type": "manual_push", "channel": "push", "title": "T", "body": "B"})
	requireDeniedNoTemplateChange(t, pool, rwID, templateID, beforeCount, beforeName, beforeTitle, beforeActive, w, "create template RO")

	w = doJSON(router, http.MethodPut, "/api/v1/rw/"+rwID+"/notifications/templates/"+templateID, tokenRO, map[string]any{"title": "Changed"})
	requireDeniedNoTemplateChange(t, pool, rwID, templateID, beforeCount, beforeName, beforeTitle, beforeActive, w, "update template RO")

	w = doJSON(router, http.MethodPatch, "/api/v1/rw/"+rwID+"/notifications/templates/"+templateID+"/toggle", tokenRO, nil)
	requireDeniedNoTemplateChange(t, pool, rwID, templateID, beforeCount, beforeName, beforeTitle, beforeActive, w, "toggle template RO")

	w = doJSON(router, http.MethodDelete, "/api/v1/rw/"+rwID+"/notifications/templates/"+templateID, tokenRO, nil)
	requireDeniedNoTemplateChange(t, pool, rwID, templateID, beforeCount, beforeName, beforeTitle, beforeActive, w, "delete template RO")

	// read should still be allowed for explicit read-only
	w = doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/notifications", tokenRO, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("explicit read-only list notifications: got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRWNotification_KetuaBendaharaExplicitAllowedMutations(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})
	_, tokenKetua := createRWExplicitUserToken(t, pool, rwID, auth.RolePengurus, auth.JabatanKetua)
	templateID := createRWTemplateForTest(t, pool, rwID)

	beforeCount := countRWTemplates(t, pool, rwID)
	w := doJSON(router, http.MethodPost, "/api/v1/rw/"+rwID+"/notifications/templates", tokenKetua, map[string]any{"name": "New", "event_type": "manual_push", "channel": "push", "title": "T2", "body": "B2"})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("ketua create template: got %d body=%s", w.Code, w.Body.String())
	}
	if after := countRWTemplates(t, pool, rwID); after != beforeCount+1 {
		t.Fatalf("expected template count +1, got %d -> %d", beforeCount, after)
	}

	w = doJSON(router, http.MethodPatch, "/api/v1/rw/"+rwID+"/notifications/templates/"+templateID+"/toggle", tokenKetua, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("ketua toggle: got %d body=%s", w.Code, w.Body.String())
	}

	_, tokenBend := createRWExplicitUserToken(t, pool, rwID, auth.RoleBendahara, auth.JabatanBendahara)
	w = doJSON(router, http.MethodPost, "/api/v1/rw/"+rwID+"/notifications/manual", tokenBend, map[string]any{"title": "Manual", "body": "Body"})
	if w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized {
		t.Fatalf("bendahara manual push denied: %d body=%s", w.Code, w.Body.String())
	}
}

func TestRWNotification_SuperAdminBypass(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})
	_, saToken := createSAToken(t, pool)
	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/notifications", saToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("SA list notifications: got %d body=%s", w.Code, w.Body.String())
	}
	templateID := createRWTemplateForTest(t, pool, rwID)
	w = doJSON(router, http.MethodDelete, "/api/v1/rw/"+rwID+"/notifications/templates/"+templateID, saToken, nil)
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("SA delete template: got %d body=%s", w.Code, w.Body.String())
	}
}

// ============================================================================
// RW Monitoring Security Tests
// Endpoint: /api/v1/rw/{rwId}/monitoring (RT-scoped, {rwId} not used for auth)
// Middleware stack: RequireAuth -> RequireAllowedRT -> RequireAccess(JabatanKebersihan)
//                  -> RequirePermission(rw.monitoring.{read|notes.write|export})
// ============================================================================

var rwMonSeq uint64 = uint64(time.Now().UnixNano()%1000000 + 8000000)

const rwMonRTUrena = "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b" // rt=002, whitelisted in scoping config

func insertKebersihanMonitoringPerms(t *testing.T, pool *database.Pool, codes ...string) {
	insertJabatanMonitoringPerms(t, pool, "kebersihan", codes...)
}

func insertJabatanMonitoringPerms(t *testing.T, pool *database.Pool, position string, codes ...string) {
	t.Helper()
	for _, code := range codes {
		_, err := pool.Raw().ExecContext(context.Background(), `
			INSERT INTO position_permissions (position, permission_id)
			SELECT $1, id FROM permission_types WHERE code = $2
			ON CONFLICT (position, permission_id) DO NOTHING
		`, position, code)
		if err != nil {
			t.Fatalf("insert position_permission %s -> %s: %v", position, code, err)
		}
	}
	t.Cleanup(func() {
		for _, code := range codes {
			_, _ = pool.Raw().ExecContext(context.Background(), `
				DELETE FROM position_permissions
				WHERE position = $1
				AND permission_id = (SELECT id FROM permission_types WHERE code = $2)
			`, position, code)
		}
	})
}

func createRTKeamananUser(t *testing.T, pool *database.Pool, rtID string) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rtkeam-%d@test.com", atomic.AddUint64(&rwMonSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RT Keamanan User', NULL, true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var mid string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`,
		userID, rtID, string(auth.RolePengurus), string(auth.JabatanKeamanan)).Scan(&mid); err != nil {
		t.Fatalf("create rt membership: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, mid)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     mid,
		RTID:    rtID,
		Role:    string(auth.RolePengurus),
		Jabatan: string(auth.JabatanKeamanan),
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return userID, token
}

func createRTKetuaUser(t *testing.T, pool *database.Pool, rtID string) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rtketua-%d@test.com", atomic.AddUint64(&rwMonSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RT Ketua User', NULL, true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var mid string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`,
		userID, rtID, string(auth.RolePengurus), string(auth.JabatanKetua)).Scan(&mid); err != nil {
		t.Fatalf("create rt membership: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, mid)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     mid,
		RTID:    rtID,
		Role:    string(auth.RolePengurus),
		Jabatan: string(auth.JabatanKetua),
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return userID, token
}

func createRTBendaharaUser(t *testing.T, pool *database.Pool, rtID string) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rtbend-%d@test.com", atomic.AddUint64(&rwMonSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RT Bendahara User', NULL, true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var mid string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`,
		userID, rtID, string(auth.RoleBendahara), string(auth.JabatanBendahara)).Scan(&mid); err != nil {
		t.Fatalf("create rt membership: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, mid)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     mid,
		RTID:    rtID,
		Role:    string(auth.RoleBendahara),
		Jabatan: string(auth.JabatanBendahara),
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return userID, token
}

func createRTKebersihanUser(t *testing.T, pool *database.Pool, rtID string) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rtkeb-%d@test.com", atomic.AddUint64(&rwMonSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RT Kebersihan User', NULL, true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var mid string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`,
		userID, rtID, string(auth.RolePengurus), string(auth.JabatanKebersihan)).Scan(&mid); err != nil {
		t.Fatalf("create rt membership: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, mid)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:   userID,
		MID:      mid,
		RTID:     rtID,
		Role:     string(auth.RolePengurus),
		Jabatan:  string(auth.JabatanKebersihan),
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return userID, token
}
func createWargaUser(t *testing.T, pool *database.Pool, rtID string) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("rtwarga-%d@test.com", atomic.AddUint64(&rwMonSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'RT Warga User', NULL, true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var mid string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		 VALUES (gen_random_uuid(), $1, $2, $3, NULL, true) RETURNING id`,
		userID, rtID, string(auth.RoleWarga)).Scan(&mid); err != nil {
		t.Fatalf("create rt membership: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, mid)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID: userID,
		MID:    mid,
		RTID:   rtID,
		Role:   string(auth.RoleWarga),
		Jabatan: "",
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return userID, token
}

func createSecondRT(t *testing.T, pool *database.Pool) string {
	t.Helper()
	var rtID string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
		fmt.Sprintf("Cross RT %d", atomic.AddUint64(&rwMonSeq, 1)), 3, "003", "Address", "Head").Scan(&rtID); err != nil {
		t.Fatalf("create second RT: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
	})
	return rtID
}

func setupMonitoringData(t *testing.T, pool *database.Pool, rtID, createdBy string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddUint64(&rwMonSeq, 1)

	var houseID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, $2, true) RETURNING id`,
		rtID, fmt.Sprintf("MON-%d", n)).Scan(&houseID); err != nil {
		t.Fatalf("create physical house: %v", err)
	}

	var householdID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, $2, true) RETURNING id`,
		rtID, fmt.Sprintf("Monitoring Household %d", n)).Scan(&householdID); err != nil {
		t.Fatalf("create household: %v", err)
	}

	var occupancyID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2025-01-01'::date) RETURNING id`,
		householdID, houseID).Scan(&occupancyID); err != nil {
		t.Fatalf("create occupancy: %v", err)
	}

	var dueID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO dues (rt_id, name, amount, period_type, is_active, is_voluntary)
		 VALUES ($1, $2, 100000, 'monthly', true, false) RETURNING id`,
		rtID, fmt.Sprintf("Obligatory %d", n)).Scan(&dueID); err != nil {
		t.Fatalf("create due: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status, is_active, is_obligatory)
		 VALUES ($1, $2, $3, 100000, '2026-05', '2026-05-01'::date, 'unpaid', true, true)`,
		rtID, occupancyID, dueID); err != nil {
		t.Fatalf("create bill: %v", err)
	}

	var createdByVal interface{} = createdBy
	if createdBy == "" {
		createdByVal = nil
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO rw_monitoring_notes (rt_id, household_occupancy_id, note, created_by, updated_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4, NOW(), NOW())`,
		rtID, occupancyID, fmt.Sprintf("Existing note %d", n), createdByVal); err != nil {
		t.Fatalf("create monitoring note: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM rw_monitoring_notes WHERE rt_id = $1`, rtID)
		_, _ = db.ExecContext(ctx, `DELETE FROM bills WHERE rt_id = $1`, rtID)
		_, _ = db.ExecContext(ctx, `DELETE FROM dues WHERE rt_id = $1`, rtID)
		_, _ = db.ExecContext(ctx, `DELETE FROM household_occupancies WHERE household_id IN (SELECT id FROM households WHERE rt_id = $1)`, rtID)
		_, _ = db.ExecContext(ctx, `DELETE FROM physical_houses WHERE rt_id = $1`, rtID)
		_, _ = db.ExecContext(ctx, `DELETE FROM households WHERE rt_id = $1`, rtID)
	})

	return occupancyID
}

func countMonitoringNotes(t *testing.T, pool *database.Pool, rtID string) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM rw_monitoring_notes WHERE rt_id = $1`, rtID).Scan(&n); err != nil {
		t.Fatalf("count monitoring notes: %v", err)
	}
	return n
}

func monitoringNoteSnapshot(t *testing.T, pool *database.Pool, rtID, occupancyID string) string {
	t.Helper()
	var note string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT note FROM rw_monitoring_notes WHERE rt_id = $1 AND household_occupancy_id = $2`,
		rtID, occupancyID).Scan(&note); err != nil {
		t.Fatalf("snapshot monitoring note: %v", err)
	}
	return note
}
// Req 1: Kebersihan dengan permission rw.monitoring.read dapat membaca data RT sendiri.
func TestRWMonitoring_KebersihanReadOwnRT(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.read")

	_, token := createRTKebersihanUser(t, pool, rwMonRTUrena)
	setupMonitoringData(t, pool, rwMonRTUrena, "")

	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("kebersihan read monitoring: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// Req 2: Kebersihan tanpa permission rw.monitoring.notes.write ditolak, DB tidak berubah.
func TestRWMonitoring_KebersihanWithoutWritePermDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.read")

	_, token := createRTKebersihanUser(t, pool, rwMonRTUrena)
	occupancyID := setupMonitoringData(t, pool, rwMonRTUrena, "")

	beforeCount := countMonitoringNotes(t, pool, rwMonRTUrena)
	beforeNote := monitoringNoteSnapshot(t, pool, rwMonRTUrena, occupancyID)

	w := doJSON(router, http.MethodPost,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID,
		token, map[string]any{"note": "Unauthorized note"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("kebersihan without write perm: expected 403, got %d body=%s", w.Code, w.Body.String())
	}

	// DB snapshot: unchanged
	if afterCount := countMonitoringNotes(t, pool, rwMonRTUrena); afterCount != beforeCount {
		t.Fatalf("note count changed %d -> %d", beforeCount, afterCount)
	}
	if afterNote := monitoringNoteSnapshot(t, pool, rwMonRTUrena, occupancyID); afterNote != beforeNote {
		t.Fatalf("note mutated: before=%q after=%q", beforeNote, afterNote)
	}
}

// Req 3: Warga dan role tidak berhak (no jabatan) ditolak.
func TestRWMonitoring_WargaRoleDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.read")

	_, token := createWargaUser(t, pool, rwMonRTUrena)

	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("warga read monitoring: expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// Req 4: Cross-RT read ditolak.
func TestRWMonitoring_CrossRTReadDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.read")

	// Kebersihan user in RT Urena (002)
	_, tokenRTUrena := createRTKebersihanUser(t, pool, rwMonRTUrena)

	// Second RT (003) with its own monitoring data
	rtID2 := createSecondRT(t, pool)
	occupancyID2 := setupMonitoringData(t, pool, rtID2, "")

	// Try to read RT2's monitoring note using RT Urena's kebersihan token.
	// Handler resolves RT from JWT (RT Urena), so RT2's occupancy won't match.
	// Cross-RT isolation: response is 200 with empty data (no notes for this RT).
	w := doJSON(router, http.MethodGet,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID2,
		tokenRTUrena, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cross-RT read: expected 200 (empty), got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("cross-RT read: invalid JSON: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("cross-RT read: expected empty data, got %d items", len(resp.Data))
	}
}

// Req 5: Cross-RT create/update notes ditolak dan database tidak berubah.
func TestRWMonitoring_CrossRTWriteDeniedDBUnchanged(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.read", "rw.monitoring.notes.write")

	// Kebersihan user in RT Urena (002) — has write permission but wrong RT
	_, tokenRTUrena := createRTKebersihanUser(t, pool, rwMonRTUrena)

	// Second RT (003) with its own monitoring data
	rtID2 := createSecondRT(t, pool)
	occupancyID2 := setupMonitoringData(t, pool, rtID2, "")

	beforeCountRT2 := countMonitoringNotes(t, pool, rtID2)
	beforeNoteRT2 := monitoringNoteSnapshot(t, pool, rtID2, occupancyID2)

	// Try to create RT2's note using RT Urena's kebersihan token — cross-RT, must be denied
	w := doJSON(router, http.MethodPost,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID2,
		tokenRTUrena, map[string]any{"note": "Cross-RT note"})
	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		t.Fatalf("cross-RT write: expected denial, got %d body=%s", w.Code, w.Body.String())
	}

	// Try PATCH as well
	w = doJSON(router, http.MethodPatch,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID2,
		tokenRTUrena, map[string]any{"note": "Cross-RT patch"})
	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		t.Fatalf("cross-RT patch: expected denial, got %d body=%s", w.Code, w.Body.String())
	}

	// DB snapshot for RT2: unchanged
	if afterCount := countMonitoringNotes(t, pool, rtID2); afterCount != beforeCountRT2 {
		t.Fatalf("RT2 note count changed %d -> %d", beforeCountRT2, afterCount)
	}
	if afterNote := monitoringNoteSnapshot(t, pool, rtID2, occupancyID2); afterNote != beforeNoteRT2 {
		t.Fatalf("RT2 note mutated: before=%q after=%q", beforeNoteRT2, afterNote)
	}
}
// Req 6a: Permission rw.monitoring.notes.write diperiksa pada endpoint notes (POST/PATCH).
func TestRWMonitoring_NotesWritePermissionAllowed(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.notes.write")
	userIDWrite, tokenWrite := createRTKebersihanUser(t, pool, rwMonRTUrena)
	occupancyID := setupMonitoringData(t, pool, rwMonRTUrena, userIDWrite)

	w := doJSON(router, http.MethodPost,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID,
		tokenWrite, map[string]any{"note": "Authorized note"})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("kebersihan with notes.write: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}
	if after := countMonitoringNotes(t, pool, rwMonRTUrena); after < 1 {
		t.Fatalf("expected at least 1 note after write, got %d", after)
	}
}

func TestRWMonitoring_NotesWritePermissionDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	// Hanya read, tanpa notes.write
	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.read")
	_, tokenNoWrite := createRTKebersihanUser(t, pool, rwMonRTUrena)
	occupancyID := setupMonitoringData(t, pool, rwMonRTUrena, "")

	before := countMonitoringNotes(t, pool, rwMonRTUrena)
	w := doJSON(router, http.MethodPost,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID,
		tokenNoWrite, map[string]any{"note": "No write perm"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("kebersihan without notes.write: expected 403, got %d body=%s", w.Code, w.Body.String())
	}
	if after := countMonitoringNotes(t, pool, rwMonRTUrena); after != before {
		t.Fatalf("note count changed %d -> %d (write without perm)", before, after)
	}
}

// Req 6b: Permission rw.monitoring.export diperiksa pada endpoint export.
func TestRWMonitoring_ExportPermissionAllowed(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.export")
	_, tokenExport := createRTKebersihanUser(t, pool, rwMonRTUrena)

	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring/export/excel", tokenExport, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("kebersihan export excel: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	w = doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring/export/pdf", tokenExport, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("kebersihan export pdf: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRWMonitoring_ExportPermissionDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	// Tanpa export perm (insert read saja untuk lolos RequireAccess+jabatan check,
	// tapi RequirePermission export akan menolak)
	insertKebersihanMonitoringPerms(t, pool, "rw.monitoring.read")
	_, tokenNoExport := createRTKebersihanUser(t, pool, rwMonRTUrena)

	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring/export/excel", tokenNoExport, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("kebersihan without export: expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// Req 7: Super Admin mengikuti bypass yang berlaku.
func TestRWMonitoring_SuperAdminBypass(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	// Super Admin token WITH RTID (RequireAllowedRT requires RTID even for SA)
	var saUserID string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'SA Monitor', 'super_admin', true) RETURNING id`,
		fmt.Sprintf("sa-monitor-%d@test.com", atomic.AddUint64(&rwMonSeq, 1))).Scan(&saUserID); err != nil {
		t.Fatalf("create SA: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, saUserID)
	})
	saToken, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID: saUserID,
		SysRole: string(auth.SystemRoleSuperAdmin),
		RTID:   rwMonRTUrena,
	})
	if err != nil {
		t.Fatalf("SA token: %v", err)
	}

	// SA can read monitoring (position_permission bypass)
	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring", saToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("SA read monitoring: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// Req 8 (subset): Every denied mutation includes before/after DB snapshot.
// Covered in TestRWMonitoring_KebersihanWithoutWritePermDenied and TestRWMonitoring_CrossRTWriteDeniedDBUnchanged.

// TestRWMonitoring_KeamananAccess verifies Keamanan with permissions can access monitoring.
func TestRWMonitoring_KeamananAccess(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertJabatanMonitoringPerms(t, pool, "keamanan", "rw.monitoring.read", "rw.monitoring.notes.write", "rw.monitoring.export")

	userID, token := createRTKeamananUser(t, pool, rwMonRTUrena)
	occupancyID := setupMonitoringData(t, pool, rwMonRTUrena, userID)

	// 1. Read list
	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("keamanan read monitoring: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 2. Write note
	w = doJSON(router, http.MethodPost,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID,
		token, map[string]any{"note": "Catatan Keamanan"})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("keamanan post note: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}

	// 3. Export
	w = doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring/export/excel", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("keamanan export excel: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRWMonitoring_KetuaAccess verifies Ketua RT with permissions can access monitoring.
func TestRWMonitoring_KetuaAccess(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertJabatanMonitoringPerms(t, pool, "ketua", "rw.monitoring.read", "rw.monitoring.notes.write", "rw.monitoring.export")

	userID, token := createRTKetuaUser(t, pool, rwMonRTUrena)
	occupancyID := setupMonitoringData(t, pool, rwMonRTUrena, userID)

	// 1. Read list
	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("ketua read monitoring: expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// 2. Write note
	w = doJSON(router, http.MethodPost,
		"/api/v1/rw/"+rwID+"/monitoring/notes/"+occupancyID,
		token, map[string]any{"note": "Catatan Ketua RT"})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("ketua post note: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}

	// 3. Export
	w = doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring/export/excel", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("ketua export excel: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRWMonitoring_BendaharaDenied verifies non-permitted positions like Bendahara cannot access monitoring.
func TestRWMonitoring_BendaharaDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	// Even if position_permissions had the code injected, RequireAccess blocks Bendahara
	insertJabatanMonitoringPerms(t, pool, "bendahara", "rw.monitoring.read")

	_, token := createRTBendaharaUser(t, pool, rwMonRTUrena)

	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring", token, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bendahara read monitoring: expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRWMonitoring_InactiveMembershipDenied verifies that when a user's membership is deactivated, access is forbidden.
func TestRWMonitoring_InactiveMembershipDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rwID := createRWForNotifTest(t, pool)
	_, router := setupRWNotifRouter(t, []string{rwID})

	insertJabatanMonitoringPerms(t, pool, "keamanan", "rw.monitoring.read")

	userID, token := createRTKeamananUser(t, pool, rwMonRTUrena)

	// Deactivate membership in DB
	_, err := pool.Raw().ExecContext(context.Background(),
		`UPDATE user_rt_memberships SET is_active = false WHERE user_id = $1 AND rt_id = $2`,
		userID, rwMonRTUrena)
	if err != nil {
		t.Fatalf("deactivate membership: %v", err)
	}

	w := doJSON(router, http.MethodGet, "/api/v1/rw/"+rwID+"/monitoring", token, nil)
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("inactive membership read monitoring: expected 401 or 403, got %d body=%s", w.Code, w.Body.String())
	}
}
