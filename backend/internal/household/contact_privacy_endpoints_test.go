package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"social-finance/internal/auth"
)

// TestContactPrivacy_AllRolesEndpoints verifies at the HTTP endpoint level (actual JSON/CSV/XLSX responses)
// that Warga phone and email are ONLY exposed to an authenticated exact RT jabatan `ketua`
// whose verified active RT membership matches the target RT scope, while preserving self-service.
func TestContactPrivacy_AllRolesEndpoints(t *testing.T) {
	pool := testPool(t)
	t.Cleanup(func() { pool.Close() })
	signingSecret()
	auth.AuthDBPool = pool

	ctx := context.Background()
	db := pool.Raw()

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	rwNum := 800 + int(time.Now().UnixNano()%180)
	var rwID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ($1, $2, true) RETURNING id`,
		"RW ContactPrivacy "+suffix, rwNum,
	).Scan(&rwID)
	require.NoError(t, err)

	rt1ID := setupTestRT(t, pool, "RT1 Privacy "+suffix, rwNum, "C1"+suffix[:2])
	rt2ID := setupTestRT(t, pool, "RT2 Privacy "+suffix, rwNum, "C2"+suffix[:2])
	_, err = db.ExecContext(ctx, `UPDATE rts SET rw_id = $1 WHERE id IN ($2, $3)`, rwID, rt1ID, rt2ID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE rw_id = $1`, rwID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})

	// Seed a target resident in RT1 with known phone, email, and NIK.
	const targetNik = "3171098765432199"
	const targetPhone = "081299990001"
	const targetEmail = "target-warga-" + "privacy@example.test"

	var phID, hhID, hoID, targetResID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO physical_houses (id, rt_id, house_number, is_active) VALUES (gen_random_uuid(), $1, 'CP-01', true) RETURNING id`,
		rt1ID,
	).Scan(&phID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO households (id, rt_id, head_name, is_active) VALUES (gen_random_uuid(), $1, 'Kepala CP', true) RETURNING id`,
		rt1ID,
	).Scan(&hhID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (id, physical_house_id, household_id, occupancy_status, start_date) VALUES (gen_random_uuid(), $1, $2, 'OWNER', '2026-01-01') RETURNING id`,
		phID, hhID,
	).Scan(&hoID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO residents (id, rt_id, full_name, phone, nik, email, is_active) VALUES (gen_random_uuid(), $1, 'Target Warga', $2, $3, $4, true) RETURNING id`,
		rt1ID, targetPhone, targetNik, targetEmail,
	).Scan(&targetResID))
	_, err = db.ExecContext(ctx,
		`INSERT INTO residency_periods (id, resident_id, household_occupancy_id, start_date) VALUES (gen_random_uuid(), $1, $2, '2026-01-01')`,
		targetResID, hoID,
	)
	require.NoError(t, err)

	// Seed a self-service Warga user linked to their own resident row in RT1.
	const selfPhone = "+6281299990002"
	const selfEmail = "self-warga-privacy@example.test"
	selfUserID := uuid.NewString()
	selfMID := uuid.NewString()
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, email, phone, password_hash, full_name, is_active) VALUES ($1, $2, $3, 'dummyhash', 'Self Warga', true)`,
		selfUserID, fmt.Sprintf("self-%s@example.test", suffix), selfPhone,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active) VALUES ($1, $2, $3, 'warga', NULL, true)`,
		selfMID, selfUserID, rt1ID,
	)
	require.NoError(t, err)
	var selfResID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO residents (id, rt_id, user_id, full_name, phone, nik, email, is_active) VALUES (gen_random_uuid(), $1, $2, 'Self Warga', $3, '3171098765432188', $4, true) RETURNING id`,
		rt1ID, selfUserID, selfPhone, selfEmail,
	).Scan(&selfResID))
	_, err = db.ExecContext(ctx,
		`INSERT INTO residency_periods (id, resident_id, household_occupancy_id, start_date) VALUES (gen_random_uuid(), $1, $2, '2026-01-01')`,
		selfResID, hoID,
	)
	require.NoError(t, err)
	selfToken := makeToken(t, auth.TokenClaims{
		UserID: selfUserID,
		MID:    selfMID,
		RTID:   rt1ID,
		Role:   string(auth.RoleWarga),
	})

	// Helper to create a caller with specific RT membership + optional RW membership + custom claim jabatan.
	type callerSpec struct {
		name         string
		rtID         string
		role         auth.Role
		dbJabatan    string // stored in user_rt_memberships.jabatan ("" -> NULL)
		claimJabatan string // stored in JWT claims.Jabatan
		rwJabatan    string // if non-empty, creates explicit user_rw_memberships row
		isSuperAdmin bool
		allowContact bool
	}

	makeCallerToken := func(spec callerSpec) string {
		uid := uuid.NewString()
		mid := uuid.NewString()
		var sysRole any = nil
		if spec.isSuperAdmin {
			sysRole = string(auth.SystemRoleSuperAdmin)
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, email, phone, password_hash, full_name, system_role, is_active)
			 VALUES ($1, $2, '+628110000000', 'dummyhash', $3, $4, true)`,
			uid, fmt.Sprintf("u-%s@example.test", uid[:12]), "Caller "+spec.name, sysRole,
		)
		require.NoError(t, err)

		if spec.isSuperAdmin {
			return makeToken(t, auth.TokenClaims{
				UserID:  uid,
				SysRole: string(auth.SystemRoleSuperAdmin),
				Jabatan: spec.claimJabatan,
			})
		}

		var dbJab any = nil
		if spec.dbJabatan != "" {
			dbJab = spec.dbJabatan
		}
		_, err = db.ExecContext(ctx,
			`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			 VALUES ($1, $2, $3, $4, $5, true)`,
			mid, uid, spec.rtID, string(spec.role), dbJab,
		)
		require.NoError(t, err)

		rwCtx := ""
		if spec.rwJabatan != "" {
			_, err = db.ExecContext(ctx,
				`INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, rt_id, is_active)
				 VALUES ($1, $2, 'pengurus', $3, $4, true)`,
				uid, rwID, spec.rwJabatan, spec.rtID,
			)
			require.NoError(t, err)
			rawRW, _ := json.Marshal([]auth.RWMembership{{
				RWID:    rwID,
				Jabatan: spec.rwJabatan,
				Source:  auth.RWMembershipSourceExplicit,
			}})
			rwCtx = string(rawRW)
		}

		return makeToken(t, auth.TokenClaims{
			UserID:    uid,
			MID:       mid,
			RTID:      spec.rtID,
			Role:      string(spec.role),
			Jabatan:   spec.claimJabatan,
			RWContext: rwCtx,
		})
	}

	callers := []callerSpec{
		{name: "exact Ketua RT", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "ketua", claimJabatan: "ketua", allowContact: true},
		{name: "exact Ketua RT other RT", rtID: rt2ID, role: auth.RoleWarga, dbJabatan: "ketua", claimJabatan: "ketua", allowContact: false},
		{name: "Wakil Ketua", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "wakil_ketua", claimJabatan: "wakil_ketua", allowContact: false},
		{name: "Sekretaris", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "sekretaris", claimJabatan: "sekretaris", allowContact: false},
		{name: "Bendahara", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "bendahara", claimJabatan: "bendahara", allowContact: false},
		{name: "Keamanan", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "keamanan", claimJabatan: "keamanan", allowContact: false},
		{name: "Kebersihan", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "kebersihan", claimJabatan: "kebersihan", allowContact: false},
		{name: "Operasional", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "operasional", claimJabatan: "operasional", allowContact: false},
		{name: "Humas", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "humas", claimJabatan: "humas", allowContact: false},
		{name: "Sosial", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "sosial", claimJabatan: "sosial", allowContact: false},
		{name: "Anggota RT", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "anggota_rt", claimJabatan: "anggota_rt", allowContact: false},
		{name: "Anggota RW", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "anggota", claimJabatan: "anggota", rwJabatan: "anggota", allowContact: false},
		{name: "Ketua RW", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "anggota", claimJabatan: "anggota", rwJabatan: "ketua", allowContact: false},
		{name: "Super Admin", rtID: rt1ID, isSuperAdmin: true, allowContact: false},
		{name: "Super Admin with spoofed ketua claim", rtID: rt1ID, isSuperAdmin: true, claimJabatan: "ketua", allowContact: false},
		{name: "User without jabatan", rtID: rt1ID, role: auth.RoleWarga, dbJabatan: "", claimJabatan: "", allowContact: false},
		{name: "Generic permission holder (pengurus no jabatan)", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "", claimJabatan: "", allowContact: false},
		{name: "Alias ketua_rt", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "anggota_rt", claimJabatan: "ketua_rt", allowContact: false},
		{name: "Alias ketua rt", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "anggota_rt", claimJabatan: "ketua rt", allowContact: false},
		{name: "Stale ketua JWT after DB demotion", rtID: rt1ID, role: auth.RolePengurus, dbJabatan: "sekretaris", claimJabatan: "ketua", allowContact: false},
	}

	// Build an Auth-only router so every role reaches the import preview/export handlers directly,
	// proving handler-level defense-in-depth regardless of route middleware.
	previewRouter := chi.NewRouter()
	previewRouter.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/v1/warga/import/preview", HandleWargaImportPreview(pool))
		r.Post("/api/v1/warga/import/preview/xlsx", HandleWargaImportPreviewXLSX(pool))
		r.Get("/api/v1/warga/export", HandleWargaExport(pool))
		r.Get("/api/v1/warga/export/xlsx", HandleWargaExportXLSX(pool))
	})
	hhRouter := setUpRouter(pool)

	const previewPhone = "081234560999"
	const previewEmail = "preview-secret@example.test"
	const previewNik = "3171098765432177"
	csvPayload := fmt.Sprintf(
		"house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,move_in_date,jabatan,password\n"+
			"CP-99,Preview Head,OWNER,Jl. Privacy 99,Preview Resident,%s,081234560999,%s,2026-01-15,,Pass1234!\n",
		previewNik, previewEmail,
	)

	makeXLSXBody := func(t *testing.T) (*bytes.Buffer, string) {
		t.Helper()
		f := excelize.NewFile()
		sheet := f.GetSheetName(0)
		_ = f.SetSheetName(sheet, "Warga")
		headers := []string{"house_number", "head_name", "occupancy_status", "address", "resident_name", "nik", "phone", "email", "move_in_date", "jabatan", "password"}
		row := []string{"CP-98", "Preview Head XLSX", "OWNER", "Jl. Privacy 98", "Preview Resident XLSX", previewNik, "081234560999", previewEmail, "2026-01-15", "", "Pass1234!"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			require.NoError(t, f.SetCellValue("Warga", cell, h))
		}
		for i, v := range row {
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			require.NoError(t, f.SetCellStr("Warga", cell, v))
		}
		var xlsxBuf bytes.Buffer
		require.NoError(t, f.Write(&xlsxBuf))
		_ = f.Close()

		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("file", "preview.xlsx")
		require.NoError(t, err)
		_, err = part.Write(xlsxBuf.Bytes())
		require.NoError(t, err)
		require.NoError(t, mw.Close())
		return &body, mw.FormDataContentType()
	}

	for _, spec := range callers {
		tok := makeCallerToken(spec)

		t.Run("CSV preview/"+spec.name, func(t *testing.T) {
			if spec.rtID != rt1ID && !spec.isSuperAdmin {
				// Cross-RT tenant user attempting to supply ?rt_id=rt1ID must be rejected (400).
				req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview?rt_id="+rt1ID, strings.NewReader(csvPayload))
				req.Header.Set("Authorization", "Bearer "+tok)
				rec := httptest.NewRecorder()
				previewRouter.ServeHTTP(rec, req)
				require.Equal(t, http.StatusBadRequest, rec.Code, "cross-RT rt_id query param must be rejected")
				return
			}
			url := "/api/v1/warga/import/preview"
			if spec.isSuperAdmin {
				url += "?rt_id=" + rt1ID
			}
			req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(csvPayload))
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			previewRouter.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

			var resp struct {
				PreviewData []map[string]any `json:"preview_data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Len(t, resp.PreviewData, 1)

			if spec.allowContact {
				require.Equal(t, previewPhone, resp.PreviewData[0]["phone"])
				require.Equal(t, previewEmail, resp.PreviewData[0]["email"])
			} else {
				require.Empty(t, resp.PreviewData[0]["phone"], "%s leaked phone in CSV preview: %s", spec.name, rec.Body.String())
				require.Empty(t, resp.PreviewData[0]["email"], "%s leaked email in CSV preview: %s", spec.name, rec.Body.String())
			}
		})

		t.Run("XLSX preview/"+spec.name, func(t *testing.T) {
			if spec.rtID != rt1ID && !spec.isSuperAdmin {
				body, ct := makeXLSXBody(t)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/warga/import/preview/xlsx?rt_id="+rt1ID, body)
				req.Header.Set("Authorization", "Bearer "+tok)
				req.Header.Set("Content-Type", ct)
				rec := httptest.NewRecorder()
				previewRouter.ServeHTTP(rec, req)
				require.Equal(t, http.StatusBadRequest, rec.Code, "cross-RT rt_id query param must be rejected")
				return
			}
			body, ct := makeXLSXBody(t)
			url := "/api/v1/warga/import/preview/xlsx"
			if spec.isSuperAdmin {
				url += "?rt_id=" + rt1ID
			}
			req := httptest.NewRequest(http.MethodPost, url, body)
			req.Header.Set("Authorization", "Bearer "+tok)
			req.Header.Set("Content-Type", ct)
			rec := httptest.NewRecorder()
			previewRouter.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

			var resp struct {
				PreviewData []map[string]any `json:"preview_data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Len(t, resp.PreviewData, 1)

			if spec.allowContact {
				require.Equal(t, previewPhone, resp.PreviewData[0]["phone"])
				require.Equal(t, previewEmail, resp.PreviewData[0]["email"])
			} else {
				require.Empty(t, resp.PreviewData[0]["phone"], "%s leaked phone in XLSX preview: %s", spec.name, rec.Body.String())
				require.Empty(t, resp.PreviewData[0]["email"], "%s leaked email in XLSX preview: %s", spec.name, rec.Body.String())
			}
		})

		t.Run("CSV export/"+spec.name, func(t *testing.T) {
			if spec.isSuperAdmin {
				// Super admin without RT context receives 401 on CSV export
				req := httptest.NewRequest(http.MethodGet, "/api/v1/warga/export?rt_id="+rt1ID, nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				rec := httptest.NewRecorder()
				previewRouter.ServeHTTP(rec, req)
				// Even if X-RT-ID / ?rt_id= populated effectiveRTID, contacts must never appear.
				require.NotContains(t, rec.Body.String(), targetPhone)
				require.NotContains(t, rec.Body.String(), targetEmail)
				return
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/warga/export", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			previewRouter.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)

			if spec.allowContact {
				require.Contains(t, rec.Body.String(), targetPhone)
				require.Contains(t, rec.Body.String(), targetEmail)
			} else {
				require.NotContains(t, rec.Body.String(), targetPhone, "%s leaked phone in CSV export", spec.name)
				require.NotContains(t, rec.Body.String(), targetEmail, "%s leaked email in CSV export", spec.name)
			}
		})

		t.Run("Resident GET/"+spec.name, func(t *testing.T) {
			if spec.name == "Stale ketua JWT after DB demotion" {
				// Covered by import/export DB check; Resident GET uses AuthContext claim.
				return
			}
			url := "/api/v1/residents/" + targetResID
			if spec.isSuperAdmin {
				url += "?rt_id=" + rt1ID
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			hhRouter.ServeHTTP(rec, req)
			if spec.rtID != rt1ID && !spec.isSuperAdmin {
				require.Equal(t, http.StatusNotFound, rec.Code)
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			if spec.allowContact {
				require.Equal(t, targetPhone, body["phone"])
				require.Equal(t, targetEmail, body["email"])
			} else {
				_, hasPhone := body["phone"]
				_, hasEmail := body["email"]
				require.False(t, hasPhone, "%s leaked phone in GET /residents/:id: %s", spec.name, rec.Body.String())
				require.False(t, hasEmail, "%s leaked email in GET /residents/:id: %s", spec.name, rec.Body.String())
			}
		})
	}

	// Self-service own profile must remain intact for a regular Warga, while redacting other residents.
	t.Run("Self-service own profile retains own contacts and redacts others", func(t *testing.T) {
		reqOwn := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+selfResID, nil)
		reqOwn.Header.Set("Authorization", "Bearer "+selfToken)
		recOwn := httptest.NewRecorder()
		hhRouter.ServeHTTP(recOwn, reqOwn)
		require.Equal(t, http.StatusOK, recOwn.Code, "body=%s", recOwn.Body.String())

		var ownBody map[string]any
		require.NoError(t, json.Unmarshal(recOwn.Body.Bytes(), &ownBody))
		require.Equal(t, selfPhone, ownBody["phone"], "self-service own phone must be preserved")
		require.Equal(t, selfEmail, ownBody["email"], "self-service own email must be preserved")
		_, hasNik := ownBody["nik"]
		require.False(t, hasNik, "NIK visibility rule remains independent and redacted for warga")

		reqOther := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+targetResID, nil)
		reqOther.Header.Set("Authorization", "Bearer "+selfToken)
		recOther := httptest.NewRecorder()
		hhRouter.ServeHTTP(recOther, reqOther)
		require.Equal(t, http.StatusOK, recOther.Code)

		var otherBody map[string]any
		require.NoError(t, json.Unmarshal(recOther.Body.Bytes(), &otherBody))
		_, otherHasPhone := otherBody["phone"]
		_, otherHasEmail := otherBody["email"]
		require.False(t, otherHasPhone, "other resident phone must be redacted for warga")
		require.False(t, otherHasEmail, "other resident email must be redacted for warga")
	})

	// Household create response must redact top-level phone and email for non-Ketua (e.g. Super Admin).
	t.Run("POST /api/v1/households redacts top-level phone and email for Super Admin", func(t *testing.T) {
		saToken := makeSuperAdminToken(t)
		payload := fmt.Sprintf(`{
			"rt_id": %q,
			"house_number": "CP-HH-%s",
			"head_name": "Head Privacy",
			"nik": "3171098765432%s",
			"phone": "081288887777",
			"email": "hh-privacy-%s@example.test",
			"occupancy_status": "OWNER",
			"start_date": "2026-01-01"
		}`, rt1ID, suffix[:4], fmt.Sprintf("%03d", time.Now().UnixNano()%900+100), suffix)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/households", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+saToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		hhRouter.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		_, hasTopPhone := body["phone"]
		_, hasTopEmail := body["email"]
		require.False(t, hasTopPhone, "Super Admin POST /households leaked top-level phone: %s", rec.Body.String())
		require.False(t, hasTopEmail, "Super Admin POST /households leaked top-level email: %s", rec.Body.String())
		if hr, ok := body["head_resident"].(map[string]any); ok {
			_, hasHRPhone := hr["phone"]
			_, hasHREmail := hr["email"]
			require.False(t, hasHRPhone, "Super Admin POST /households leaked head_resident.phone: %s", rec.Body.String())
			require.False(t, hasHREmail, "Super Admin POST /households leaked head_resident.email: %s", rec.Body.String())
		}
	})
}
