package main

import (
	"context"
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

const flRTUrena = "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b" // rt=002, whitelisted

var flSeq uint64 = uint64(9000000)

// setupFLRouter membangun router aktual (buildRouter) dengan middleware
// RequireAuth -> RequireAllowedRT, tanpa RequirePermission pada route fund location.
func setupFLRouter(t *testing.T, rtIDs []string) (*database.Pool, http.Handler) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("ensure devseed: %v", err)
	}
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

// createRTUser membuat user dengan user_rt_memberships pada rtID dan JWT token
// yang membawa RTID + MID + role + jabatan (dari claim JWT, bukan dari body).
func createRTUser(t *testing.T, pool *database.Pool, rtID string, role auth.Role, jabatan auth.Jabatan) (string, string) {
	t.Helper()
	var userID string
	email := fmt.Sprintf("fluser-%d-%d@test.com", time.Now().UnixNano(), atomic.AddUint64(&flSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'FL User', NULL, true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var mid string
	jabatanVal := string(jabatan)
	if jabatanVal == "" {
		if err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			 VALUES (gen_random_uuid(), $1, $2, $3, NULL, true) RETURNING id`,
			userID, rtID, string(role)).Scan(&mid); err != nil {
			t.Fatalf("create rt membership: %v", err)
		}
	} else {
		if err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			 VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`,
			userID, rtID, string(role), jabatanVal).Scan(&mid); err != nil {
			t.Fatalf("create rt membership: %v", err)
		}
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
		Role:     string(role),
		Jabatan:  string(jabatan),
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return userID, token
}

// createSATokenWithRT membuat SA token dengan RTID (RequireAllowedRT wajib RTID bahkan untuk SA).
func createSATokenWithRT(t *testing.T, pool *database.Pool, rtID string) string {
	t.Helper()
	var userID string
	email := fmt.Sprintf("flsa-%d-%d@test.com", time.Now().UnixNano(), atomic.AddUint64(&flSeq, 1))
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hash', 'FL SA', 'super_admin', true) RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("create SA: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID: userID,
		SysRole: string(auth.SystemRoleSuperAdmin),
		RTID:   rtID,
	})
	if err != nil {
		t.Fatalf("SA token: %v", err)
	}
	return token
}

// createFLLocationSeed langsung insert fund_location ke DB untuk kebutuhan cross-RT / read test.
func createFLLocationSeed(t *testing.T, pool *database.Pool, rtID, name, category string) string {
	t.Helper()
	var id string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO fund_locations (rt_id, name, category, is_active)
		 VALUES ($1, $2, $3, true) RETURNING id`,
		rtID, name, category).Scan(&id); err != nil {
		t.Fatalf("seed fund location: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM location_balance_changes WHERE location_id = $1 AND rt_id = $2`, id, rtID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM fund_transfers WHERE from_location_id = $1 OR to_location_id = $1`, id)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM fund_locations WHERE id = $1 AND rt_id = $2`, id, rtID)
	})
	return id
}

// ---- Snapshot helpers (state DB sebelum dan sesudah mutasi yang ditolak) ----

func countFLLocations(t *testing.T, pool *database.Pool, rtID string) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM fund_locations WHERE rt_id = $1`, rtID).Scan(&n); err != nil {
		t.Fatalf("count fund_locations: %v", err)
	}
	return n
}

func countFLTransfers(t *testing.T, pool *database.Pool, rtID string) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM fund_transfers WHERE rt_id = $1`, rtID).Scan(&n); err != nil {
		t.Fatalf("count fund_transfers: %v", err)
	}
	return n
}

func countFLBalanceChanges(t *testing.T, pool *database.Pool, rtID string) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM location_balance_changes WHERE rt_id = $1`, rtID).Scan(&n); err != nil {
		t.Fatalf("count location_balance_changes: %v", err)
	}
	return n
}

func countFLAuditLogs(t *testing.T, pool *database.Pool, rtID string, actionLike string) int {
	t.Helper()
	var n int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE rt_id = $1 AND action LIKE $2`,
		rtID, actionLike).Scan(&n); err != nil {
		t.Fatalf("count audit_logs: %v", err)
	}
	return n
}

// requireFLDeniedNoChange memeriksa response 403/404/401 dan snapshot DB tidak berubah.
func requireFLDeniedNoChange(t *testing.T, pool *database.Pool, rtID string,
	beforeLoc, beforeTr, beforeLBC, beforeAudit int, w *httptest.ResponseRecorder, label string) {
	t.Helper()
	if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound && w.Code != http.StatusUnauthorized {
		t.Fatalf("%s: expected 403/404/401, got %d body=%s", label, w.Code, w.Body.String())
	}
	if after := countFLLocations(t, pool, rtID); after != beforeLoc {
		t.Fatalf("%s: fund_locations changed %d -> %d", label, beforeLoc, after)
	}
	if after := countFLTransfers(t, pool, rtID); after != beforeTr {
		t.Fatalf("%s: fund_transfers changed %d -> %d", label, beforeTr, after)
	}
	if after := countFLBalanceChanges(t, pool, rtID); after != beforeLBC {
		t.Fatalf("%s: location_balance_changes changed %d -> %d", label, beforeLBC, after)
	}
	if after := countFLAuditLogs(t, pool, rtID, "%fund_location%"); after != beforeAudit {
		t.Fatalf("%s: audit_logs changed %d -> %d", label, beforeAudit, after)
	}
}

// requireFLDeniedNoChangeTransfer sama tetapi untuk action fund_transfer.
func requireFLDeniedNoChangeTransfer(t *testing.T, pool *database.Pool, rtID string,
	beforeLoc, beforeTr, beforeLBC, beforeAudit int, w *httptest.ResponseRecorder, label string) {
	t.Helper()
	if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound && w.Code != http.StatusUnauthorized {
		t.Fatalf("%s: expected 403/404/401, got %d body=%s", label, w.Code, w.Body.String())
	}
	if after := countFLLocations(t, pool, rtID); after != beforeLoc {
		t.Fatalf("%s: fund_locations changed %d -> %d", label, beforeLoc, after)
	}
	if after := countFLTransfers(t, pool, rtID); after != beforeTr {
		t.Fatalf("%s: fund_transfers changed %d -> %d", label, beforeTr, after)
	}
	if after := countFLBalanceChanges(t, pool, rtID); after != beforeLBC {
		t.Fatalf("%s: location_balance_changes changed %d -> %d", label, beforeLBC, after)
	}
	if after := countFLAuditLogs(t, pool, rtID, "%fund_transfer%"); after != beforeAudit {
		rows, err := pool.Raw().QueryContext(context.Background(),
			`SELECT action, entity_id, created_at FROM audit_logs WHERE rt_id = $1 AND action LIKE $2 ORDER BY created_at DESC LIMIT 10`,
			rtID, "%fund_transfer%")
		if err == nil {
			defer rows.Close()
			var actions []string
			for rows.Next() {
				var a, e, c string
				_ = rows.Scan(&a, &e, &c)
				actions = append(actions, a)
			}
			t.Fatalf("%s: audit_logs(fund_transfer) changed %d -> %d; recent actions=%v", label, beforeAudit, after, actions)
		}
		t.Fatalf("%s: audit_logs(fund_transfer) changed %d -> %d", label, beforeAudit, after)
	}
}

// Req 1 & 2: Warga / no-jabatan ditolak melakukan mutasi (Create, Update, Deactivate, SetOpeningBalance).
// Setiap mutasi yang ditolak dibandingkan snapshot DB sebelum dan sesudah.
func TestFL_WargaDeniedCreateLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, token := createRTUser(t, pool, flRTUrena, auth.RoleWarga, "")

	beforeLoc := countFLLocations(t, pool, flRTUrena)
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_location%")

	w := doJSON(router, http.MethodPost, "/api/v1/fund-locations", token,
		map[string]any{"name": "Brankas Warga", "category": "brankas"})
	requireFLDeniedNoChange(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "warga create location")
}

func TestFL_WargaDeniedUpdateLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, token := createRTUser(t, pool, flRTUrena, auth.RoleWarga, "")
	locID := createFLLocationSeed(t, pool, flRTUrena, "Lokasi Target", "brankas")

	beforeLoc := countFLLocations(t, pool, flRTUrena)
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_location%")

	w := doJSON(router, http.MethodPatch, "/api/v1/fund-locations/"+locID, token,
		map[string]any{"name": "Diubah Warga"})
	requireFLDeniedNoChange(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "warga update location")
}

func TestFL_WargaDeniedDeactivateLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, token := createRTUser(t, pool, flRTUrena, auth.RoleWarga, "")
	locID := createFLLocationSeed(t, pool, flRTUrena, "Lokasi Target", "brankas")

	beforeLoc := countFLLocations(t, pool, flRTUrena)
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_location%")

	w := doJSON(router, http.MethodDelete, "/api/v1/fund-locations/"+locID, token, nil)
	requireFLDeniedNoChange(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "warga deactivate location")
}

func TestFL_WargaDeniedSetOpeningBalance(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, token := createRTUser(t, pool, flRTUrena, auth.RoleWarga, "")
	locID := createFLLocationSeed(t, pool, flRTUrena, "Lokasi Target", "brankas")

	beforeLoc := countFLLocations(t, pool, flRTUrena)
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_location%")

	w := doJSON(router, http.MethodPut, "/api/v1/fund-locations/"+locID+"/opening-balance", token,
		map[string]any{"amount": "100000.00"})
	requireFLDeniedNoChange(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "warga set opening balance")
}

func TestFL_WargaDeniedCreateTransfer(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, token := createRTUser(t, pool, flRTUrena, auth.RoleWarga, "")

	beforeLoc := countFLLocations(t, pool, flRTUrena)
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_transfer%")

	w := doJSON(router, http.MethodPost, "/api/v1/fund-transfers", token,
		map[string]any{"from_location_id": "00000000-0000-0000-0000-000000000001", "to_location_id": "00000000-0000-0000-0000-000000000002", "amount": "100000.00"})
	requireFLDeniedNoChangeTransfer(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "warga create transfer")
}

func TestFL_WargaDeniedReverseTransfer(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, token := createRTUser(t, pool, flRTUrena, auth.RoleWarga, "")

	beforeLoc := countFLLocations(t, pool, flRTUrena)
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_transfer%")

	w := doJSON(router, http.MethodPost, "/api/v1/fund-transfers/00000000-0000-0000-0000-000000000001/reverse", token,
		map[string]any{"reason": "coba"})
	requireFLDeniedNoChangeTransfer(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "warga reverse transfer")
}

// Req 3: Cross-RT tidak dapat membaca atau mengubah data di luar scope.
func createFLSecondRT(t *testing.T, pool *database.Pool) string {
	t.Helper()
	var rtID string
	if err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
		fmt.Sprintf("FL Cross RT %d", atomic.AddUint64(&flSeq, 1)), 3, "003", "Address", "Head").Scan(&rtID); err != nil {
		t.Fatalf("create second RT: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
	})
	return rtID
}

func TestFL_CrossRTReadDenied(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, tokenRT1 := createRTUser(t, pool, flRTUrena, auth.RoleBendahara, "")
	rtID2 := createFLSecondRT(t, pool)
	locID2 := createFLLocationSeed(t, pool, rtID2, "Lokasi RT Lain", "brankas")
	w := doJSON(router, http.MethodGet, "/api/v1/fund-locations/"+locID2, tokenRT1, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-RT read: expected 404, got %d body=%s", w.Code, w.Body.String())
	}
	w = doJSON(router, http.MethodGet, "/api/v1/fund-locations", tokenRT1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cross-RT list own RT: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestFL_CrossRTWriteDeniedDBUnchanged(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, tokenRT1 := createRTUser(t, pool, flRTUrena, auth.RoleBendahara, "")
	rtID2 := createFLSecondRT(t, pool)
	locID2 := createFLLocationSeed(t, pool, rtID2, "Lokasi RT Lain", "brankas")
	beforeLoc := countFLLocations(t, pool, flRTUrena)
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_location%")
	w := doJSON(router, http.MethodPatch, "/api/v1/fund-locations/"+locID2, tokenRT1,
		map[string]any{"name": "Diubah Lintas RT"})
	requireFLDeniedNoChange(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "cross-RT update")
	w = doJSON(router, http.MethodPut, "/api/v1/fund-locations/"+locID2+"/opening-balance", tokenRT1,
		map[string]any{"amount": "100000.00"})
	requireFLDeniedNoChange(t, pool, flRTUrena, beforeLoc, beforeTr, beforeLBC, beforeAudit, w, "cross-RT opening balance")
}

// Req 4: Bendahara hanya dapat menjalankan aksi yang diizinkan business rules.
func TestFL_PengurusDeniedTransfer(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, tokenPengurus := createRTUser(t, pool, flRTUrena, auth.RolePengurus, "")
	beforeTr := countFLTransfers(t, pool, flRTUrena)
	beforeLBC := countFLBalanceChanges(t, pool, flRTUrena)
	beforeAudit := countFLAuditLogs(t, pool, flRTUrena, "%fund_transfer%")
	w := doJSON(router, http.MethodPost, "/api/v1/fund-transfers", tokenPengurus,
		map[string]any{"from_location_id": "00000000-0000-0000-0000-000000000001", "to_location_id": "00000000-0000-0000-0000-000000000002", "amount": "100000.00"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("pengurus transfer: expected 403, got %d body=%s", w.Code, w.Body.String())
	}
	if after := countFLTransfers(t, pool, flRTUrena); after != beforeTr {
		t.Fatalf("pengurus transfer changed fund_transfers %d -> %d", beforeTr, after)
	}
	if after := countFLBalanceChanges(t, pool, flRTUrena); after != beforeLBC {
		t.Fatalf("pengurus transfer changed location_balance_changes %d -> %d", beforeLBC, after)
	}
	if after := countFLAuditLogs(t, pool, flRTUrena, "%fund_transfer%"); after != beforeAudit {
		t.Fatalf("pengurus transfer changed audit_logs %d -> %d", beforeAudit, after)
	}
}

func TestFL_BendaharaAllowedCreateLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, tokenBend := createRTUser(t, pool, flRTUrena, auth.RoleBendahara, "")
	beforeLoc := countFLLocations(t, pool, flRTUrena)
	locName := fmt.Sprintf("Brankas Bendahara %d", time.Now().UnixNano())
	w := doJSON(router, http.MethodPost, "/api/v1/fund-locations", tokenBend,
		map[string]any{"name": locName, "category": "brankas"})
	if w.Code != http.StatusCreated {
		t.Fatalf("bendahara create location: expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if after := countFLLocations(t, pool, flRTUrena); after != beforeLoc+1 {
		t.Fatalf("expected location count +1, got %d -> %d", beforeLoc, after)
	}
}

func TestFL_PengurusAllowedCreateLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	_, tokenPeng := createRTUser(t, pool, flRTUrena, auth.RolePengurus, "")
	beforeLoc := countFLLocations(t, pool, flRTUrena)
	locName := fmt.Sprintf("Rekening Pengurus %d", time.Now().UnixNano())
	w := doJSON(router, http.MethodPost, "/api/v1/fund-locations", tokenPeng,
		map[string]any{"name": locName, "category": "rekening", "bank_name": "BCA"})
	if w.Code != http.StatusCreated {
		t.Fatalf("pengurus create location: expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if after := countFLLocations(t, pool, flRTUrena); after != beforeLoc+1 {
		t.Fatalf("expected location count +1, got %d -> %d", beforeLoc, after)
	}
}

// Req 5: Super Admin bypass (checkWriteAccess / checkBendaharaAccess mengizinkan SA).
func TestFL_SuperAdminBypass(t *testing.T) {
	pool := testutil.GetTestPool(t)
	_, router := setupFLRouter(t, []string{flRTUrena})
	saToken := createSATokenWithRT(t, pool, flRTUrena)
	beforeLoc := countFLLocations(t, pool, flRTUrena)
	locName := fmt.Sprintf("Brankas SA %d", time.Now().UnixNano())
	w := doJSON(router, http.MethodPost, "/api/v1/fund-locations", saToken,
		map[string]any{"name": locName, "category": "brankas"})
	if w.Code != http.StatusCreated {
		t.Fatalf("SA create location: expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if after := countFLLocations(t, pool, flRTUrena); after != beforeLoc+1 {
		t.Fatalf("expected location count +1, got %d -> %d", beforeLoc, after)
	}
	w = doJSON(router, http.MethodGet, "/api/v1/reports/fund-locations/reconciliation", saToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("SA reconciliation: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}
