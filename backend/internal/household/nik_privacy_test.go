package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	const secretPhone = "081299991111"
	const secretEmail = "warga-rahasia@test.com"
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
		INSERT INTO residents (id, rt_id, full_name, phone, nik, email, is_active)
		VALUES (gen_random_uuid(), $1, 'Warga Rahasia', $3, $2, $4, true)
		RETURNING id
	`, rtID, secretNik, secretPhone, secretEmail).Scan(&residentID)
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

	// Ketua RT in production has role=warga + jabatan=ketua (see migration 031).
	// The export privacy check must rely on jabatan, not on TenantRole==pengurus.
	ketuaToken := makeTokenWithJabatan("user-ketua-nik", "ketua", auth.RoleWarga)
	wakilKetuaToken := makeTokenWithJabatan("user-wakil-nik", "wakil_ketua", auth.RolePengurus)
	sekretarisToken := makeTokenWithJabatan("user-sekretaris-nik", "sekretaris", auth.RolePengurus)
	bendaharaToken := makeTokenWithJabatan("user-bendahara-nik", "bendahara", auth.RolePengurus)
	keamananToken := makeTokenWithJabatan("user-keamanan-nik", "keamanan", auth.RoleWarga)
	sosialToken := makeTokenWithJabatan("user-sosial-nik", "sosial", auth.RolePengurus)
	operasionalToken := makeTokenWithJabatan("user-operasional-nik", "operasional", auth.RoleWarga)
	kebersihanToken := makeTokenWithJabatan("user-kebersihan-nik", "kebersihan", auth.RoleWarga)
	anggotaToken := makeTokenWithJabatan("user-anggota-nik", "anggota", auth.RoleWarga)
	anggotaRTToken := makeTokenWithJabatan("user-anggotart-nik", "anggota_rt", auth.RolePengurus)
	// ketua_rt claim variant: membership in DB is "anggota_rt", but token claim is "ketua_rt"
	ketuaRTToken := makeToken(t, auth.TokenClaims{
		UserID:  toUUID("user-anggotart-nik"),
		MID:     toUUID("mid-user-anggotart-nik"),
		RTID:    rtID,
		Role:    string(auth.RolePengurus),
		Jabatan: "ketua_rt",
	})
	wargaToken := makeTokenWithJabatan("user-warga-nik", "", auth.RoleWarga)
	superAdminTok := makeSuperAdminToken(t)

	// 1. Ketua RT can see NIK, phone, and email in GET /api/v1/residents/:id
	t.Run("Ketua RT can read NIK, phone, and email", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/residents/"+residentID, nil)
		req.Header.Set("Authorization", "Bearer "+ketuaToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var res Resident
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		require.NotNil(t, res.Nik, "Ketua RT must receive NIK")
		require.Equal(t, secretNik, *res.Nik)
		require.NotNil(t, res.Phone, "Ketua RT must receive phone")
		require.Equal(t, secretPhone, *res.Phone)
		require.NotNil(t, res.Email, "Ketua RT must receive email")
		require.Equal(t, secretEmail, *res.Email)
	})

	// Matrix testing: all non-ketua roles MUST have NIK, phone, and email redacted to nil
	nonKetuaResidentTokens := map[string]string{
		"Super Admin": superAdminTok,
		"Sekretaris":  sekretarisToken,
		"Bendahara":   bendaharaToken,
		"Wakil Ketua": wakilKetuaToken,
		"Keamanan":    keamananToken,
		"Sosial":      sosialToken,
		"Operasional": operasionalToken,
		"Kebersihan":  kebersihanToken,
		"Anggota":     anggotaToken,
		"Anggota RT":  anggotaRTToken,
		"Ketua RT (variant ketua_rt)": ketuaRTToken,
		"Warga":       wargaToken,
	}

	for roleName, tok := range nonKetuaResidentTokens {
		t.Run("Non-ketua receives nil NIK and nil contacts/"+roleName, func(t *testing.T) {
			targetURL := "/api/v1/residents/" + residentID
			if roleName == "Super Admin" {
				targetURL += "?rt_id=" + rtID
			}
			req := httptest.NewRequest("GET", targetURL, nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)

			var res Resident
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
			require.Nil(t, res.Nik, "%s must NOT receive NIK", roleName)
			require.Nil(t, res.Phone, "%s must NOT receive phone", roleName)
			require.Nil(t, res.Email, "%s must NOT receive email", roleName)
		})
	}

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

	// 7. Non-ketua export XLSX does not contain raw NIK, phone, or email
	nonKetuaExports := map[string]string{
		"Bendahara":   bendaharaToken,
		"Sekretaris":  sekretarisToken,
		"Wakil Ketua": wakilKetuaToken,
		"Keamanan":    keamananToken,
		"Sosial":      sosialToken,
		"Operasional": operasionalToken,
		"Kebersihan":  kebersihanToken,
		"Anggota":     anggotaToken,
		"Anggota RT":  anggotaRTToken,
		"Ketua RT (variant)": ketuaRTToken,
		"Warga":       wargaToken,
		"Super Admin": superAdminTok,
	}
	for name, tok := range nonKetuaExports {
		t.Run("Non-ketua export XLSX redacts NIK phone email/"+name, func(t *testing.T) {
			exportRouter := setupWargaImportExportRouter(pool)
			req := httptest.NewRequest("GET", "/api/v1/warga/export/xlsx", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			exportRouter.ServeHTTP(rec, req)
			// Super Admin export may return 200 with redacted data; tenant roles must be 200 too.
			require.Equal(t, http.StatusOK, rec.Code, "export for %s failed: %s", name, rec.Body.String())

			f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
			require.NoError(t, err)
			defer f.Close()

			rows, err := f.GetRows("Warga")
			require.NoError(t, err)
			for _, row := range rows {
				for _, cell := range row {
					require.NotContains(t, cell, secretNik, "Export must NOT leak secret NIK to %s", name)
					require.NotContains(t, cell, secretPhone, "Export must NOT leak secret phone to %s", name)
					require.NotContains(t, cell, secretEmail, "Export must NOT leak secret email to %s", name)
				}
			}
		})
	}

	// 8. Ketua export XLSX retains NIK, phone, and email (role=warga + jabatan=ketua, as in production)
	t.Run("Ketua export XLSX includes NIK phone email", func(t *testing.T) {
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
		var foundNik, foundPhone, foundEmail bool
		for _, row := range rows {
			for _, cell := range row {
				if cell == secretNik {
					foundNik = true
				}
				if cell == secretPhone {
					foundPhone = true
				}
				if cell == secretEmail {
					foundEmail = true
				}
			}
		}
		require.True(t, foundNik, "Export for Ketua RT must include NIK")
		require.True(t, foundPhone, "Export for Ketua RT must include phone")
		require.True(t, foundEmail, "Export for Ketua RT must include email")
	})
}
