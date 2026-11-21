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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

var opHumasRtCounter int64 = time.Now().UnixNano() % 1000000

func setupTestApp(t *testing.T, pool *database.Pool, rtID string, rwIDs ...string) http.Handler {
	t.Helper()
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	auth.AuthDBPool = pool
	authSvc := auth.NewService(auth.ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	authHandler := auth.NewHandler(pool, authSvc)
	cfg := auth.ScopingConfig{
		RTIDs: []string{rtID},
		RWIDs: rwIDs,
	}
	return buildRouter(pool, authHandler, &cfg, storage.NewMockStorage())
}

func createTestUserToken(t *testing.T, userID, membershipID, rtID string, role auth.Role, jabatan auth.Jabatan) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		MID:     membershipID,
		Role:    string(role),
		RTID:    rtID,
		Jabatan: string(jabatan),
	})
	require.NoError(t, err)
	return token
}

func createTestSAToken(t *testing.T, pool *database.Pool) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM users WHERE system_role = 'super_admin' AND is_active = true LIMIT 1`).Scan(&userID)
	if err != nil {
		err = pool.Raw().QueryRowContext(ctx, `
			INSERT INTO users (id, email, phone, password_hash, full_name, system_role, is_active)
			VALUES (gen_random_uuid(), $1, '08123456789', 'hash', 'Test Super Admin', 'super_admin', true)
			RETURNING id
		`, fmt.Sprintf("sa_%d@test.local", time.Now().UnixNano())).Scan(&userID)
		require.NoError(t, err)
	}
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  userID,
		SysRole: string(auth.SystemRoleSuperAdmin),
	})
	require.NoError(t, err)
	return token
}

func insertTestRTHelper(t *testing.T, pool *database.Pool) (string, string) {
	t.Helper()
	var rtID string
	var rwID string
	ctx := context.Background()
	cnt := atomic.AddInt64(&opHumasRtCounter, 1)
	rwNum := int(700000 + (cnt*37)%100000)
	rtNum := fmt.Sprintf("%03d", cnt%900+1)

	err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rws (id, name, rw_number, is_active)
		VALUES (gen_random_uuid(), $1, $2, true)
		RETURNING id
	`, fmt.Sprintf("RW %d", rwNum), rwNum).Scan(&rwID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rts (id, name, rw, rt, rw_id, is_active)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, true)
		RETURNING id
	`, "RT Test "+rtNum, rwNum, rtNum, rwID).Scan(&rtID)
	require.NoError(t, err)
	return rtID, rwID
}

func TestOperasional_HouseholdAndRWAccess(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID, rwID := insertTestRTHelper(t, pool)
	router := setupTestApp(t, pool, rtID, rwID)

	ctx := context.Background()

	// Create a user with Operasional jabatan who has a valid household
	userID := uuid.NewString()
	membershipID := uuid.NewString()
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, 'hash', 'Petugas Operasional Test', true)
	`, userID, userID+"@local.com")
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'operasional', true)
	`, membershipID, userID, rtID)
	require.NoError(t, err)

	// Setup household chain
	var hhID, phID, occID, resID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'KK Operasional', true) RETURNING id
	`, rtID).Scan(&hhID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'OP-01', true) RETURNING id
	`, rtID).Scan(&phID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status)
		VALUES ($1, $2, 'OWNER') RETURNING id
	`, hhID, phID).Scan(&occID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO residents (rt_id, user_id, full_name, is_active)
		VALUES ($1, $2, 'Petugas Operasional Test', true) RETURNING id
	`, rtID, userID).Scan(&resID)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO residency_periods (resident_id, household_occupancy_id) VALUES ($1, $2)
	`, resID, occID)
	require.NoError(t, err)

	opToken := createTestUserToken(t, userID, membershipID, rtID, auth.RoleWarga, auth.JabatanOperasional)

	// 1. /api/v1/auth/me returns household information (house_number = 'OP-01')
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+opToken)
	recMe := httptest.NewRecorder()
	router.ServeHTTP(recMe, reqMe)
	require.Equal(t, http.StatusOK, recMe.Code)

	var meResp struct {
		HouseNumber *string `json:"house_number"`
		Jabatan     *string `json:"jabatan"`
	}
	require.NoError(t, json.NewDecoder(recMe.Body).Decode(&meResp))
	require.NotNil(t, meResp.HouseNumber)
	require.Equal(t, "OP-01", *meResp.HouseNumber)
	require.NotNil(t, meResp.Jabatan)
	require.Equal(t, "operasional", *meResp.Jabatan)

	// 2. Can access Warga / residents list
	reqWarga := httptest.NewRequest(http.MethodGet, "/api/v1/residents", nil)
	reqWarga.Header.Set("Authorization", "Bearer "+opToken)
	recWarga := httptest.NewRecorder()
	router.ServeHTTP(recWarga, reqWarga)
	require.Equal(t, http.StatusOK, recWarga.Code)

	// 3. RT Kas write / transaction is blocked (403 Forbidden)
	reqKasWrite := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewBufferString(`{}`))
	reqKasWrite.Header.Set("Authorization", "Bearer "+opToken)
	reqKasWrite.Header.Set("Content-Type", "application/json")
	recKasWrite := httptest.NewRecorder()
	router.ServeHTTP(recKasWrite, reqKasWrite)
	require.Equal(t, http.StatusForbidden, recKasWrite.Code)

	// 4. RT Kas read (transactions) is blocked (403 Forbidden)
	reqKasRead := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
	reqKasRead.Header.Set("Authorization", "Bearer "+opToken)
	recKasRead := httptest.NewRecorder()
	router.ServeHTTP(recKasRead, reqKasRead)
	require.Equal(t, http.StatusForbidden, recKasRead.Code)

	// 5. RT Lokasi Dana is blocked (403 Forbidden)
	reqLoc := httptest.NewRequest(http.MethodGet, "/api/v1/fund-locations", nil)
	reqLoc.Header.Set("Authorization", "Bearer "+opToken)
	recLoc := httptest.NewRecorder()
	router.ServeHTTP(recLoc, reqLoc)
	require.Equal(t, http.StatusForbidden, recLoc.Code)

	// 6. Personal household bills are accessible (200 OK)
	reqBills := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	reqBills.Header.Set("Authorization", "Bearer "+opToken)
	recBills := httptest.NewRecorder()
	router.ServeHTTP(recBills, reqBills)
	require.Equal(t, http.StatusOK, recBills.Code)

	// 7. RW Dashboard READ is accessible (200 OK)
	reqRWDash := httptest.NewRequest(http.MethodGet, "/api/v1/rw/"+rwID+"/dashboard", nil)
	reqRWDash.Header.Set("Authorization", "Bearer "+opToken)
	recRWDash := httptest.NewRecorder()
	router.ServeHTTP(recRWDash, reqRWDash)
	require.Equal(t, http.StatusOK, recRWDash.Code)

	// 8. RW Kas READ is accessible (200 OK)
	reqRWKas := httptest.NewRequest(http.MethodGet, "/api/v1/rw/"+rwID+"/kas", nil)
	reqRWKas.Header.Set("Authorization", "Bearer "+opToken)
	recRWKas := httptest.NewRecorder()
	router.ServeHTTP(recRWKas, reqRWKas)
	require.Equal(t, http.StatusOK, recRWKas.Code)

	// 9. RW Kas WRITE is blocked (403 Forbidden)
	reqRWKasWrite := httptest.NewRequest(http.MethodPost, "/api/v1/rw/"+rwID+"/kas", bytes.NewBufferString(`{}`))
	reqRWKasWrite.Header.Set("Authorization", "Bearer "+opToken)
	reqRWKasWrite.Header.Set("Content-Type", "application/json")
	recRWKasWrite := httptest.NewRecorder()
	router.ServeHTTP(recRWKasWrite, reqRWKasWrite)
	require.Equal(t, http.StatusForbidden, recRWKasWrite.Code)

	// 10. RW Lokasi Dana is blocked (403 Forbidden)
	reqRWLoc := httptest.NewRequest(http.MethodGet, "/api/v1/rw/"+rwID+"/locations", nil)
	reqRWLoc.Header.Set("Authorization", "Bearer "+opToken)
	recRWLoc := httptest.NewRecorder()
	router.ServeHTTP(recRWLoc, reqRWLoc)
	require.Equal(t, http.StatusForbidden, recRWLoc.Code)

	// 11. Petugas Keamanan is denied from RW (403 Forbidden)
	secUserID := uuid.NewString()
	secMemID := uuid.NewString()
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, 'hash', 'Petugas Keamanan Test', true)
	`, secUserID, secUserID+"@local.com")
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'keamanan', true)
	`, secMemID, secUserID, rtID)
	require.NoError(t, err)

	secToken := createTestUserToken(t, secUserID, secMemID, rtID, auth.RoleWarga, auth.JabatanKeamanan)
	reqSecRW := httptest.NewRequest(http.MethodGet, "/api/v1/rw/"+rwID+"/dashboard", nil)
	reqSecRW.Header.Set("Authorization", "Bearer "+secToken)
	recSecRW := httptest.NewRecorder()
	router.ServeHTTP(recSecRW, reqSecRW)
	require.Equal(t, http.StatusForbidden, recSecRW.Code)

	// 12. Petugas Kebersihan is denied from RW (403 Forbidden)
	clnUserID := uuid.NewString()
	clnMemID := uuid.NewString()
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, 'hash', 'Petugas Kebersihan Test', true)
	`, clnUserID, clnUserID+"@local.com")
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'kebersihan', true)
	`, clnMemID, clnUserID, rtID)
	require.NoError(t, err)

	clnToken := createTestUserToken(t, clnUserID, clnMemID, rtID, auth.RoleWarga, auth.JabatanKebersihan)
	reqClnRW := httptest.NewRequest(http.MethodGet, "/api/v1/rw/"+rwID+"/dashboard", nil)
	reqClnRW.Header.Set("Authorization", "Bearer "+clnToken)
	recClnRW := httptest.NewRecorder()
	router.ServeHTTP(recClnRW, reqClnRW)
	require.Equal(t, http.StatusForbidden, recClnRW.Code)
}

func TestHumas_LifecycleAndSecurityPermissions(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID, rwID := insertTestRTHelper(t, pool)
	router := setupTestApp(t, pool, rtID, rwID)

	ctx := context.Background()

	// 1. Create a resident user with a valid household
	userID := uuid.NewString()
	membershipID := uuid.NewString()
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, 'hash', 'Warga Humas Candidate', true)
	`, userID, userID+"@local.com")
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active)
		VALUES ($1, $2, $3, 'warga', true)
	`, membershipID, userID, rtID)
	require.NoError(t, err)

	var hhID, phID, occID, resID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'KK Humas', true) RETURNING id
	`, rtID).Scan(&hhID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'HM-01', true) RETURNING id
	`, rtID).Scan(&phID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status)
		VALUES ($1, $2, 'OWNER') RETURNING id
	`, hhID, phID).Scan(&occID)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO residents (rt_id, user_id, full_name, is_active)
		VALUES ($1, $2, 'Warga Humas Candidate', true) RETURNING id
	`, rtID, userID).Scan(&resID)
	require.NoError(t, err)

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO residency_periods (resident_id, household_occupancy_id) VALUES ($1, $2)
	`, resID, occID)
	require.NoError(t, err)

	// 2. Set jabatan to 'humas' via SetJabatan (as super_admin)
	saToken := createTestSAToken(t, pool)
	setReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/rts/%s/memberships/%s/jabatan", rtID, userID), bytes.NewBufferString(`{"jabatan":"humas"}`))
	setReq.Header.Set("Authorization", "Bearer "+saToken)
	setReq.Header.Set("Content-Type", "application/json")
	setRec := httptest.NewRecorder()
	router.ServeHTTP(setRec, setReq)
	require.Equal(t, http.StatusOK, setRec.Code)

	// Verify DB has jabatan = 'humas'
	var dbJabatan string
	err = pool.Raw().QueryRowContext(ctx, `SELECT jabatan FROM user_rt_memberships WHERE id = $1`, membershipID).Scan(&dbJabatan)
	require.NoError(t, err)
	require.Equal(t, "humas", dbJabatan)

	// 3. Make token for Humas
	humasToken := createTestUserToken(t, userID, membershipID, rtID, auth.RoleWarga, auth.JabatanHumas)

	// A. Can access /api/v1/security/dashboard
	secDashReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/dashboard", nil)
	secDashReq.Header.Set("Authorization", "Bearer "+humasToken)
	secDashRec := httptest.NewRecorder()
	router.ServeHTTP(secDashRec, secDashReq)
	require.Equal(t, http.StatusOK, secDashRec.Code)

	// B. Can access /api/v1/security/shifts (GET)
	secShiftsReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/shifts", nil)
	secShiftsReq.Header.Set("Authorization", "Bearer "+humasToken)
	secShiftsRec := httptest.NewRecorder()
	router.ServeHTTP(secShiftsRec, secShiftsReq)
	require.Equal(t, http.StatusOK, secShiftsRec.Code)

	// C. Can access /api/v1/security/schedules (GET)
	secSchedReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/schedules", nil)
	secSchedReq.Header.Set("Authorization", "Bearer "+humasToken)
	secSchedRec := httptest.NewRecorder()
	router.ServeHTTP(secSchedRec, secSchedReq)
	require.Equal(t, http.StatusOK, secSchedRec.Code)

	// D. Can access /api/v1/security/corrections (GET)
	secCorrReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/corrections", nil)
	secCorrReq.Header.Set("Authorization", "Bearer "+humasToken)
	secCorrRec := httptest.NewRecorder()
	router.ServeHTTP(secCorrRec, secCorrReq)
	require.Equal(t, http.StatusOK, secCorrRec.Code)

	// E. Can access /api/v1/security/devices (GET)
	secDevReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/devices", nil)
	secDevReq.Header.Set("Authorization", "Bearer "+humasToken)
	secDevRec := httptest.NewRecorder()
	router.ServeHTTP(secDevRec, secDevReq)
	require.Equal(t, http.StatusOK, secDevRec.Code)

	// F. Cannot scan QR attendance (only security officer with security.attendance can scan)
	scanReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/attendance/scan", bytes.NewBufferString(`{}`))
	scanReq.Header.Set("Authorization", "Bearer "+humasToken)
	scanReq.Header.Set("Content-Type", "application/json")
	scanRec := httptest.NewRecorder()
	router.ServeHTTP(scanRec, scanReq)
	require.Equal(t, http.StatusForbidden, scanRec.Code)

	// G. Cannot perform RT Kas mutation
	kasReq := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewBufferString(`{}`))
	kasReq.Header.Set("Authorization", "Bearer "+humasToken)
	kasReq.Header.Set("Content-Type", "application/json")
	kasRec := httptest.NewRecorder()
	router.ServeHTTP(kasRec, kasReq)
	require.Equal(t, http.StatusForbidden, kasRec.Code)

	// H. Cannot read RT Kas transactions
	kasReadReq := httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
	kasReadReq.Header.Set("Authorization", "Bearer "+humasToken)
	kasReadRec := httptest.NewRecorder()
	router.ServeHTTP(kasReadRec, kasReadReq)
	require.Equal(t, http.StatusForbidden, kasReadRec.Code)

	// I. Cannot read RT Lokasi Dana
	locReq := httptest.NewRequest(http.MethodGet, "/api/v1/fund-locations", nil)
	locReq.Header.Set("Authorization", "Bearer "+humasToken)
	locRec := httptest.NewRecorder()
	router.ServeHTTP(locRec, locReq)
	require.Equal(t, http.StatusForbidden, locRec.Code)

	// J. Cannot access Buku Tamu admin (security.guest_book missing)
	gbReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/guest-books", nil)
	gbReq.Header.Set("Authorization", "Bearer "+humasToken)
	gbRec := httptest.NewRecorder()
	router.ServeHTTP(gbRec, gbReq)
	require.Equal(t, http.StatusForbidden, gbRec.Code)

	// K. Can read personal household bills
	billsReq := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	billsReq.Header.Set("Authorization", "Bearer "+humasToken)
	billsRec := httptest.NewRecorder()
	router.ServeHTTP(billsRec, billsReq)
	require.Equal(t, http.StatusOK, billsRec.Code)

	// 4. Revoke Humas
	delReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/rts/%s/memberships/%s/jabatan", rtID, userID), nil)
	delReq.Header.Set("Authorization", "Bearer "+saToken)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	require.Equal(t, http.StatusNoContent, delRec.Code)

	// Verify DB has NULL jabatan
	var postJab *string
	err = pool.Raw().QueryRowContext(ctx, `SELECT jabatan FROM user_rt_memberships WHERE id = $1`, membershipID).Scan(&postJab)
	require.NoError(t, err)
	require.Nil(t, postJab)

	// 5. Revoked user immediately loses access to Kelola Keamanan
	revokedToken := createTestUserToken(t, userID, membershipID, rtID, auth.RoleWarga, "")
	secRevokedReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/dashboard", nil)
	secRevokedReq.Header.Set("Authorization", "Bearer "+revokedToken)
	secRevokedRec := httptest.NewRecorder()
	router.ServeHTTP(secRevokedRec, secRevokedReq)
	require.Equal(t, http.StatusForbidden, secRevokedRec.Code)

	// Verify resident & household remain intact
	var cnt int
	err = pool.Raw().QueryRowContext(ctx, `SELECT count(*) FROM residents WHERE id = $1 AND is_active = true`, resID).Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 1, cnt)
}
