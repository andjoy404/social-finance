package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"social-finance/internal/auth"
	"social-finance/internal/testutil"
)

// TestRWRBAC_AnggotaRT_AllEndpoints tests that RT committee members with
// position 'anggota' or 'anggota_rt' can read RW Dashboard and RW KAS,
// but are strictly forbidden from mutation/write operations, fund locations,
// and cross-RW data.
func TestRWRBAC_AnggotaRT_AllEndpoints(t *testing.T) {
	pool := testutil.GetTestPool(t)

	rwAID := createRBACMatrixRW(t, pool, "RW Alpha Anggota")
	rwBID := createRBACMatrixRW(t, pool, "RW Beta Anggota")
	rtAID := createRBACMatrixRT(t, pool, rwAID, "RT Alpha Anggota")

	_, router := setupRBACMatrixRouter(t, []string{rwAID, rwBID})

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

	testCases := []struct {
		name    string
		role    auth.Role
		jabatan string
	}{
		{
			name:    "Pengurus RT with jabatan anggota",
			role:    auth.RolePengurus,
			jabatan: "anggota",
		},
		{
			name:    "Pengurus RT with jabatan anggota_rt",
			role:    auth.RolePengurus,
			jabatan: "anggota_rt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := createRBACMatrixRTUserToken(t, pool, rtAID, tc.role, tc.jabatan)

			// 1. RW Dashboard: READ berhasil (200 OK)
			recDash := sendReq("GET", "/api/v1/rw/"+rwAID+"/dashboard", token, nil)
			assert.Equal(t, 200, recDash.Code, "RW dashboard must return 200 for %s, got: %s", tc.name, recDash.Body.String())

			// 2. RW KAS report: READ berhasil (200 OK)
			recReport := sendReq("GET", "/api/v1/rw/"+rwAID+"/kas/report", token, nil)
			assert.Equal(t, 200, recReport.Code, "RW KAS report must return 200 for %s, got: %s", tc.name, recReport.Body.String())

			// 3. RW KAS transactions: READ berhasil (200 OK)
			recTxs := sendReq("GET", "/api/v1/rw/"+rwAID+"/kas/transactions", token, nil)
			assert.Equal(t, 200, recTxs.Code, "RW KAS transactions must return 200 for %s, got: %s", tc.name, recTxs.Body.String())

			// 4. RW KAS create transaction: Ditolak (403 Forbidden)
			bodyKas := map[string]any{
				"type":        "income",
				"amount":      "100000.00",
				"description": "Donasi Liar",
			}
			recCreateKas := sendReq("POST", "/api/v1/rw/"+rwAID+"/kas", token, bodyKas)
			assert.Equal(t, 403, recCreateKas.Code, "RW KAS create must be forbidden (403), got: %d", recCreateKas.Code)

			// 5. RW KAS create transaction via /transactions: Ditolak (403 Forbidden)
			recCreateKasTx := sendReq("POST", "/api/v1/rw/"+rwAID+"/kas/transactions", token, bodyKas)
			assert.Equal(t, 403, recCreateKasTx.Code, "RW KAS transactions create must be forbidden (403), got: %d", recCreateKasTx.Code)

			// 6. RW Fund Location list: Ditolak (403 Forbidden for rt_inferred)
			recLocs := sendReq("GET", "/api/v1/rw/"+rwAID+"/locations", token, nil)
			assert.Equal(t, 403, recLocs.Code, "RW locations list must be forbidden (403), got: %d", recLocs.Code)

			// 7. RW Fund Location create: Ditolak (403 Forbidden)
			bodyLoc := map[string]any{
				"name":     "Rekening Baru",
				"category": "rekening",
			}
			recCreateLoc := sendReq("POST", "/api/v1/rw/"+rwAID+"/locations", token, bodyLoc)
			assert.Equal(t, 403, recCreateLoc.Code, "RW locations create must be forbidden (403), got: %d", recCreateLoc.Code)

			// 8. Cross-RW: Ditolak (403 Forbidden)
			recCrossDash := sendReq("GET", "/api/v1/rw/"+rwBID+"/dashboard", token, nil)
			assert.Equal(t, 403, recCrossDash.Code, "Cross-RW dashboard must be forbidden (403), got: %d", recCrossDash.Code)

			recCrossKas := sendReq("GET", "/api/v1/rw/"+rwBID+"/kas/report", token, nil)
			assert.Equal(t, 403, recCrossKas.Code, "Cross-RW kas report must be forbidden (403), got: %d", recCrossKas.Code)
		})
	}
}

// TestRWRBAC_WargaWithAnggotaJabatan_DeniedAccess verifies that a resident
// with role 'warga' and jabatan 'anggota' is NOT inferred as an RT official
// and receives 403 Forbidden on all RW endpoints.
func TestRWRBAC_WargaWithAnggotaJabatan_DeniedAccess(t *testing.T) {
	pool := testutil.GetTestPool(t)

	rwAID := createRBACMatrixRW(t, pool, "RW Alpha Warga Denied")
	rtAID := createRBACMatrixRT(t, pool, rwAID, "RT Alpha Warga Denied")

	_, router := setupRBACMatrixRouter(t, []string{rwAID})

	sendReq := func(method, path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	wargaToken := createRBACMatrixRTUserToken(t, pool, rtAID, auth.RoleWarga, "anggota")

	recDash := sendReq("GET", "/api/v1/rw/"+rwAID+"/dashboard", wargaToken)
	require.Equal(t, 403, recDash.Code, "Warga with jabatan anggota must be forbidden from RW dashboard")

	recKas := sendReq("GET", "/api/v1/rw/"+rwAID+"/kas/report", wargaToken)
	require.Equal(t, 403, recKas.Code, "Warga with jabatan anggota must be forbidden from RW kas report")
}
