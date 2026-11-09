package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-finance/internal/auth"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestNIKPrivacy_RBAC(t *testing.T) {
	pool := testPool(t)
	t.Cleanup(func() { pool.Close() })

	r := setUpRouter(pool)
	signingSecret()

	rtID := setupTestRT(t, pool, "NIKPrivacy RT", 1, "NIKP")
	db := pool.Raw()
	ctx := context.Background()

	// Create test resident with known NIK, household, physical house, and residency period
	const secretNik = "3171098765432109"
	var phID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO physical_houses (id, rt_id, house_number, is_active)
		VALUES (gen_random_uuid(), $1, 'NIK-01', true)
		RETURNING id
	`, rtID).Scan(&phID)
	require.NoError(t, err)

	var hhID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO households (id, rt_id, head_name, is_active)
		VALUES (gen_random_uuid(), $1, 'Kepala NIK', true)
		RETURNING id
	`, rtID).Scan(&hhID)
	require.NoError(t, err)

	var hoID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO household_occupancies (id, physical_house_id, household_id, occupancy_status, start_date)
		VALUES (gen_random_uuid(), $1, $2, 'OWNER', '2026-01-01')
		RETURNING id
	`, phID, hhID).Scan(&hoID)
	require.NoError(t, err)

	var residentID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO residents (id, rt_id, full_name, phone, nik, is_active)
		VALUES (gen_random_uuid(), $1, 'Warga Rahasia', '081299991111', $2, true)
		RETURNING id
	`, rtID, secretNik).Scan(&residentID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO residency_periods (id, resident_id, household_occupancy_id, start_date)
		VALUES (gen_random_uuid(), $1, $2, '2026-01-01')
	`, residentID, hoID)
	require.NoError(t, err)

	// Helper to make token with explicit jabatan
	makeTokenWithJabatan := func(uid, j string, role auth.Role) string {
		realUID := toUUID(uid)
		realMID := toUUID("mid-" + uid)
		_, _ = db.ExecContext(ctx,
			`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
			 VALUES ($1, $2, '08123456789', 'dummyhash', $3, true)
			 ON CONFLICT (id) DO UPDATE SET is_active = true`,
			realUID, fmt.Sprintf("%s@test.com", realUID), "User "+uid,
		)
		var jabatanVal any = nil
		if j != "" {
			jabatanVal = j
		}
		_, _ = db.ExecContext(ctx,
			`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			 VALUES ($1, $2, $3, $4, $5, true)
			 ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, jabatan = EXCLUDED.jabatan, is_active = true`,
			realMID, realUID, rtID, string(role), jabatanVal,
		)
		return makeToken(t, auth.TokenClaims{
			UserID:  realUID,
			MID:     realMID,
			RTID:    rtID,
			Role:    string(role),
			Jabatan: j,
		})
	}

	ketuaToken := makeTokenWithJabatan("user-ketua-nik", "ketua", auth.RolePengurus)
	sekretarisToken := makeTokenWithJabatan("user-sekretaris-nik", "sekretaris", auth.RolePengurus)
	bendaharaToken := makeTokenWithJabatan("user-bendahara-nik", "bendahara", auth.RolePengurus)
	keamananToken := makeTokenWithJabatan("user-keamanan-nik", "keamanan", auth.RolePengurus)
	wargaToken := makeTokenWithJabatan("user-warga-nik", "", auth.RoleWarga)
	superAdminTok := makeSuperAdminToken(t)

	// 1. Ketua RT can see NIK in GET /api/v1/residents/:id
	t.Run("Ketua RT can read NIK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents/"+residentID, nil)
		req.Header.Set("Authorization", "Bearer "+ketuaToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var res Resident
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.NotNil(t, res.Nik, "Ketua RT must receive NIK")
		require.Equal(t, secretNik, *res.Nik)
	})

	// 2. Super Admin can see NIK in GET /api/v1/residents/:id?rt_id=...
	t.Run("Super Admin can read NIK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents/"+residentID+"?rt_id="+rtID, nil)
		req.Header.Set("Authorization", "Bearer "+superAdminTok)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var res Resident
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.NotNil(t, res.Nik, "Super Admin must receive NIK")
		require.Equal(t, secretNik, *res.Nik)
	})

	// 3. Sekretaris cannot see NIK (redacted to nil)
	t.Run("Sekretaris receives nil NIK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents/"+residentID, nil)
		req.Header.Set("Authorization", "Bearer "+sekretarisToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var res Resident
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.Nil(t, res.Nik, "Sekretaris must NOT receive NIK")
	})

	// 4. Bendahara cannot see NIK (redacted to nil)
	t.Run("Bendahara receives nil NIK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents/"+residentID, nil)
		req.Header.Set("Authorization", "Bearer "+bendaharaToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var res Resident
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.Nil(t, res.Nik, "Bendahara must NOT receive NIK")
	})

	// 5. Keamanan cannot see NIK
	t.Run("Keamanan receives nil NIK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents/"+residentID, nil)
		req.Header.Set("Authorization", "Bearer "+keamananToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var res Resident
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.Nil(t, res.Nik, "Keamanan must NOT receive NIK")
	})

	// 6. Warga cannot see NIK in list
	t.Run("Warga receives nil NIK in list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents?page=1&page_size=10", nil)
		req.Header.Set("Authorization", "Bearer "+wargaToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var listResp struct {
			Data []Resident `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
		for _, item := range listResp.Data {
			require.Nil(t, item.Nik, "Warga must NOT receive NIK for any resident")
		}
	})

	// 7. Non-ketua export XLSX does not contain raw NIK
	t.Run("Non-ketua export XLSX redacts NIK", func(t *testing.T) {
		exportRouter := setupWargaImportExportRouter(pool)
		req := httptest.NewRequest("GET", "/api/v1/warga/export/xlsx", nil)
		req.Header.Set("Authorization", "Bearer "+bendaharaToken)
		rec := httptest.NewRecorder()
		exportRouter.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
		require.NoError(t, err)
		defer f.Close()

		rows, err := f.GetRows("Warga")
		require.NoError(t, err)
		for _, row := range rows {
			for _, cell := range row {
				require.False(t, strings.Contains(cell, secretNik), "Export must NOT leak secret NIK to bendahara")
			}
		}
	})

	// 8. Ketua export XLSX retains NIK
	t.Run("Ketua export XLSX includes NIK", func(t *testing.T) {
		exportRouter := setupWargaImportExportRouter(pool)
		req := httptest.NewRequest("GET", "/api/v1/warga/export/xlsx", nil)
		req.Header.Set("Authorization", "Bearer "+ketuaToken)
		rec := httptest.NewRecorder()
		exportRouter.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
		require.NoError(t, err)
		defer f.Close()

		rows, err := f.GetRows("Warga")
		require.NoError(t, err)
		var found bool
		for _, row := range rows {
			for _, cell := range row {
				if cell == secretNik {
					found = true
				}
			}
		}
		require.True(t, found, "Export for Ketua RT must include NIK")
	})
}
