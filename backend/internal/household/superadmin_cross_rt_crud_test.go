package household

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"social-finance/internal/auth"
)

func TestSuperAdmin_CrossRT_ResidentCRUD_And_AnggotaRW(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()

	// 1. Seed two distinct RTs under an RW
	runID := time.Now().UnixNano()
	rwNum := int(runID/1000000)%50000 + 40000
	var rwID, rt1ID, rt2ID string

	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Cross RT', $1, true) RETURNING id`, rwNum,
	).Scan(&rwID))

	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 001 Test', 16, '001', $1, true) RETURNING id`, rwID,
	).Scan(&rt1ID))

	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 002 Test', 16, '002', $1, true) RETURNING id`, rwID,
	).Scan(&rt2ID))

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE rt_id IN ($1, $2)`, rt1ID, rt2ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id IN ($1, $2)`, rt1ID, rt2ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id IN ($1, $2)`, rt1ID, rt2ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})

	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Get("/api/v1/residents", h.ListResidents)
	r.Post("/api/v1/residents", h.CreateResident)
	r.Get("/api/v1/residents/{id}", h.GetResident)
	r.Patch("/api/v1/residents/{id}", h.UpdateResident)
	r.Delete("/api/v1/residents/{id}", h.DeactivateResident)
	r.Post("/api/v1/residents/special", h.CreateSpecialResident)
	r.Patch("/api/v1/residents/special/{id}", h.UpdateSpecialResident)
	r.Post("/api/v1/residents/{id}/reset-password-code", h.GenerateResetPasswordCode)

	// Super Admin token pinned to RT2 in claims
	superAdminUserID := uuid.NewString()
	_, _ = db.ExecContext(ctx, `INSERT INTO users (id, email, full_name, system_role, password_hash, is_active) VALUES ($1, $2, 'Super Admin User', 'super_admin', 'hash', true)`,
		superAdminUserID, fmt.Sprintf("sa.%d@test.com", runID))
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, superAdminUserID)
	})
	superAdminToken := makeToken(t, auth.TokenClaims{
		UserID:  superAdminUserID,
		SysRole: "super_admin",
		RTID:    rt2ID,
	})

	// Ketua RT 001 token
	ketua1UserID := uuid.NewString()
	ketua1MemID := uuid.NewString()
	ketua1Token := makeTenantToken(t, ketua1UserID, ketua1MemID, rt1ID, auth.RolePengurus)
	_, _ = db.ExecContext(ctx, `UPDATE user_rt_memberships SET jabatan = 'ketua' WHERE id = $1`, ketua1MemID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, ketua1UserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, ketua1UserID)
	})

	// ── Workstream 1: SuperAdmin CRUD Warga in RT 001 without 404 ──
	t.Run("SuperAdmin creates and views resident in RT 001", func(t *testing.T) {
		resNik := fmt.Sprintf("3173%012d", (runID/10+1)%1000000000000)
		resPhone := fmt.Sprintf("08%010d", (runID/10+1)%10000000000)
		resEmail := fmt.Sprintf("warga.rt1.%d@test.com", runID)

		createBody := map[string]any{
			"rt_id":     rt1ID,
			"full_name": "Warga RT 001 Alpha",
			"nik":       resNik,
			"phone":     resPhone,
			"email":     resEmail,
		}
		b, _ := json.Marshal(createBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+superAdminToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code, "create response: %s", rec.Body.String())

		var created Resident
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
		require.Equal(t, rt1ID, created.RTID, "resident must be created in RT 001")

		// SuperAdmin views resident detail without ?rt_id= query -> MUST NOT 404
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+created.ID, nil)
		getReq.Header.Set("Authorization", "Bearer "+superAdminToken)
		getRec := httptest.NewRecorder()
		r.ServeHTTP(getRec, getReq)
		require.Equal(t, http.StatusOK, getRec.Code, "get detail response: %s", getRec.Body.String())

		var fetched Resident
		require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &fetched))
		require.Equal(t, created.ID, fetched.ID)
		require.Equal(t, rt1ID, fetched.RTID)

		// Privacy check: Phone & Email must be redacted for SuperAdmin on ordinary endpoints
		require.Nil(t, fetched.Phone, "phone must be redacted for SuperAdmin (non-ketua)")
		require.Nil(t, fetched.Email, "email must be redacted for SuperAdmin (non-ketua)")

		// SuperAdmin updates resident in RT 001
		newName := "Warga RT 001 Alpha Updated"
		updateBody := map[string]any{
			"full_name": newName,
		}
		ub, _ := json.Marshal(updateBody)
		patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+created.ID, bytes.NewReader(ub))
		patchReq.Header.Set("Content-Type", "application/json")
		patchReq.Header.Set("Authorization", "Bearer "+superAdminToken)
		patchRec := httptest.NewRecorder()
		r.ServeHTTP(patchRec, patchReq)
		require.Equal(t, http.StatusOK, patchRec.Code, "patch response: %s", patchRec.Body.String())

		// SuperAdmin can generate reset password code for resident in RT 001
		resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/residents/"+created.ID+"/reset-password-code", nil)
		resetReq.Header.Set("Authorization", "Bearer "+superAdminToken)
		resetRec := httptest.NewRecorder()
		r.ServeHTTP(resetRec, resetReq)
		require.Equal(t, http.StatusOK, resetRec.Code, "reset response: %s", resetRec.Body.String())
	})

	// ── Workstream 2: Anggota RW created by SuperAdmin appears in RT 001 List ──
	t.Run("SuperAdmin and Ketua RT create Anggota RW in RT 001", func(t *testing.T) {
		// 1. SuperAdmin creates Anggota RW specifying RT 001
		saAnggotaNik := fmt.Sprintf("3173%012d", (runID/10+2)%1000000000000)
		saAnggotaPhone := fmt.Sprintf("08%010d", (runID/10+2)%10000000000)
		saAnggotaEmail := fmt.Sprintf("anggota.rw.sa.%d@test.com", runID)

		saBody := map[string]any{
			"rt_id":     rt1ID,
			"jabatan":   "anggota_rw",
			"full_name": "Anggota RW By SA",
			"nik":       saAnggotaNik,
			"phone":     saAnggotaPhone,
			"email":     saAnggotaEmail,
		}
		b1, _ := json.Marshal(saBody)
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewReader(b1))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Authorization", "Bearer "+superAdminToken)
		rec1 := httptest.NewRecorder()
		r.ServeHTTP(rec1, req1)
		require.Equal(t, http.StatusCreated, rec1.Code, "SA create special resident: %s", rec1.Body.String())

		var saCreated Resident
		require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &saCreated))
		require.Equal(t, rt1ID, saCreated.RTID, "must be created in RT 001, NOT overwritten by SA token's RT 002!")

		// 2. Ketua RT 001 creates Anggota RW
		k1AnggotaNik := fmt.Sprintf("3173%012d", (runID/10+3)%1000000000000)
		k1AnggotaPhone := fmt.Sprintf("08%010d", (runID/10+3)%10000000000)
		k1AnggotaEmail := fmt.Sprintf("anggota.rw.k1.%d@test.com", runID)

		k1Body := map[string]any{
			"jabatan":   "anggota_rw",
			"full_name": "Anggota RW By Ketua",
			"nik":       k1AnggotaNik,
			"phone":     k1AnggotaPhone,
			"email":     k1AnggotaEmail,
		}
		b2, _ := json.Marshal(k1Body)
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewReader(b2))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+ketua1Token)
		rec2 := httptest.NewRecorder()
		r.ServeHTTP(rec2, req2)
		require.Equal(t, http.StatusCreated, rec2.Code, "Ketua create special resident: %s", rec2.Body.String())

		var k1Created Resident
		require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &k1Created))
		require.Equal(t, rt1ID, k1Created.RTID)

		// 3. Query resident list for RT 001 -> BOTH must appear in the list!
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/residents?rt_id="+rt1ID, nil)
		listReq.Header.Set("Authorization", "Bearer "+superAdminToken)
		listRec := httptest.NewRecorder()
		r.ServeHTTP(listRec, listReq)
		require.Equal(t, http.StatusOK, listRec.Code, "list response: %s", listRec.Body.String())

		var listResp struct {
			Data []Resident `json:"data"`
		}
		require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))

		foundSA, foundK1 := false, false
		for _, res := range listResp.Data {
			if res.ID == saCreated.ID {
				foundSA = true
			}
			if res.ID == k1Created.ID {
				foundK1 = true
			}
		}
		require.True(t, foundSA, "Anggota RW created by SuperAdmin MUST appear in RT 001 resident list")
		require.True(t, foundK1, "Anggota RW created by Ketua RT 001 MUST appear in RT 001 resident list")

		// 4. SuperAdmin updates the Anggota RW in RT 001 via PATCH /api/v1/residents/special/{id}
		patchSpecialBody := map[string]any{
			"full_name": "Anggota RW By SA Updated",
		}
		psb, _ := json.Marshal(patchSpecialBody)
		patchSpecialReq := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/special/"+saCreated.ID, bytes.NewReader(psb))
		patchSpecialReq.Header.Set("Content-Type", "application/json")
		patchSpecialReq.Header.Set("Authorization", "Bearer "+superAdminToken)
		patchSpecialRec := httptest.NewRecorder()
		r.ServeHTTP(patchSpecialRec, patchSpecialReq)
		require.Equal(t, http.StatusOK, patchSpecialRec.Code, "SuperAdmin patch special resident MUST NOT 404: %s", patchSpecialRec.Body.String())
	})
}

func TestSuperAdmin_CrossRT_SecurityScopeMatrix_And_AnggotaRWParity(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()

	runID := time.Now().UnixNano()
	rwNumIn := int(runID/1000000)%40000 + 50000
	rwNumOut := rwNumIn + 1

	var rwInID, rwOutID, rt1ID, rt2ID, rtOutID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Scope In', $1, true) RETURNING id`, rwNumIn,
	).Scan(&rwInID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Scope Out', $1, true) RETURNING id`, rwNumOut,
	).Scan(&rwOutID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 001 In', $1, '001', $2, true) RETURNING id`, rwNumIn, rwInID,
	).Scan(&rt1ID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 002 In', $1, '002', $2, true) RETURNING id`, rwNumIn, rwInID,
	).Scan(&rt2ID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT 999 Out', $1, '999', $2, true) RETURNING id`, rwNumOut, rwOutID,
	).Scan(&rtOutID))

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE rt_id IN ($1, $2, $3)`, rt1ID, rt2ID, rtOutID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id IN ($1, $2, $3)`, rt1ID, rt2ID, rtOutID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id IN ($1, $2, $3)`, rt1ID, rt2ID, rtOutID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rws WHERE id IN ($1, $2)`, rwInID, rwOutID)
	})

	// Administrative scope: RT 002 directly + RW In (which includes RT 001 & RT 002). RT 999 Out is OUTSIDE scope.
	scopingCfg := auth.ScopingConfig{
		RTIDs: []string{rt2ID},
		RWIDs: []string{rwInID},
	}

	h := NewHandlerWithScope(pool, scopingCfg)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireAllowedRTForWarga(pool, scopingCfg))
	r.Get("/api/v1/residents", h.ListResidents)
	r.Post("/api/v1/residents", h.CreateResident)
	r.Get("/api/v1/residents/{id}", h.GetResident)
	r.Patch("/api/v1/residents/{id}", h.UpdateResident)
	r.Delete("/api/v1/residents/{id}", h.DeactivateResident)
	r.Post("/api/v1/residents/special", h.CreateSpecialResident)
	r.Patch("/api/v1/residents/special/{id}", h.UpdateSpecialResident)

	// SuperAdmin pinned to RT 002
	saUserID := uuid.NewString()
	_, err := db.ExecContext(ctx, `INSERT INTO users (id, email, full_name, system_role, password_hash, is_active) VALUES ($1, $2, 'SA Scoped', 'super_admin', 'hash', true)`,
		saUserID, fmt.Sprintf("sa.scope.%d@test.com", runID))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, saUserID) })
	saToken := makeToken(t, auth.TokenClaims{UserID: saUserID, SysRole: "super_admin", RTID: rt2ID})

	// Exact Ketua RT 001
	ketua1UserID := uuid.NewString()
	ketua1MemID := uuid.NewString()
	ketua1Token := makeTenantToken(t, ketua1UserID, ketua1MemID, rt1ID, auth.RolePengurus)
	_, err = db.ExecContext(ctx, `UPDATE user_rt_memberships SET jabatan = 'ketua' WHERE id = $1`, ketua1MemID)
	require.NoError(t, err)
	ketua1Token = makeToken(t, auth.TokenClaims{UserID: ketua1UserID, MID: ketua1MemID, RTID: rt1ID, Role: "pengurus", Jabatan: "ketua"})
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, ketua1UserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, ketua1UserID)
	})

	// Non-ketua pengurus RT 001 (sekretaris)
	sek1UserID := uuid.NewString()
	sek1MemID := uuid.NewString()
	_ = makeTenantToken(t, sek1UserID, sek1MemID, rt1ID, auth.RolePengurus)
	_, err = db.ExecContext(ctx, `UPDATE user_rt_memberships SET jabatan = 'sekretaris' WHERE id = $1`, sek1MemID)
	require.NoError(t, err)
	sek1Token := makeToken(t, auth.TokenClaims{UserID: sek1UserID, MID: sek1MemID, RTID: rt1ID, Role: "pengurus", Jabatan: "sekretaris"})
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, sek1UserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, sek1UserID)
	})

	// Seed a regular resident and special resident directly in RT_OUTSIDE (outside SuperAdmin scope)
	outResID := uuid.NewString()
	outNik := fmt.Sprintf("3173%012d", (runID/10+50)%1000000000000)
	_, err = db.ExecContext(ctx,
		`INSERT INTO residents (id, rt_id, full_name, phone, nik, email, is_active) VALUES ($1, $2, 'Out Scope Warga', '081299990001', $3, 'out.warga@test.com', true)`,
		outResID, rtOutID, outNik)
	require.NoError(t, err)

	outSpecUserID := uuid.NewString()
	outSpecResID := uuid.NewString()
	outSpecNik := fmt.Sprintf("3173%012d", (runID/10+51)%1000000000000)
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, email, full_name, password_hash, is_active) VALUES ($1, $2, 'Out Spec', 'hash', true)`,
		outSpecUserID, fmt.Sprintf("out.spec.%d@test.com", runID))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, outSpecUserID) })
	_, err = db.ExecContext(ctx, `INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active) VALUES ($1, $2, 'warga', 'anggota_rw', true)`,
		outSpecUserID, rtOutID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO residents (id, rt_id, user_id, full_name, phone, nik, email, is_active) VALUES ($1, $2, $3, 'Out Spec Anggota', '081299990002', $4, $5, true)`,
		outSpecResID, rtOutID, outSpecUserID, outSpecNik, fmt.Sprintf("out.spec.%d@test.com", runID))
	require.NoError(t, err)

	// Seed resident in RT 002 for cross-RT isolation test
	rt2ResID := uuid.NewString()
	rt2Nik := fmt.Sprintf("3173%012d", (runID/10+52)%1000000000000)
	_, err = db.ExecContext(ctx,
		`INSERT INTO residents (id, rt_id, full_name, phone, nik, email, is_active) VALUES ($1, $2, 'RT2 Resident', '081288880002', $3, 'rt2.res@test.com', true)`,
		rt2ResID, rt2ID, rt2Nik)
	require.NoError(t, err)

	t.Run("SuperAdmin denied 403 for RT outside administrative scope on all CRUD endpoints", func(t *testing.T) {
		// CreateResident out of scope
		bCreate, _ := json.Marshal(map[string]any{
			"rt_id": rtOutID, "full_name": "Should Fail",
			"nik": fmt.Sprintf("3173%012d", (runID/10+60)%1000000000000),
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents", bytes.NewReader(bCreate))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+saToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code, "CreateResident out of scope must be 403: %s", rec.Body.String())

		// CreateSpecialResident out of scope
		bSpec, _ := json.Marshal(map[string]any{
			"rt_id": rtOutID, "jabatan": "anggota_rw", "full_name": "Out Anggota",
			"nik": fmt.Sprintf("3173%012d", (runID/10+61)%1000000000000),
			"phone": "081299990061", "email": fmt.Sprintf("out.arw.%d@test.com", runID), "password": "Password123!",
		})
		reqSpec := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewReader(bSpec))
		reqSpec.Header.Set("Content-Type", "application/json")
		reqSpec.Header.Set("Authorization", "Bearer "+saToken)
		recSpec := httptest.NewRecorder()
		r.ServeHTTP(recSpec, reqSpec)
		require.Equal(t, http.StatusForbidden, recSpec.Code, "CreateSpecialResident out of scope must be 403: %s", recSpec.Body.String())

		// ListResidents out of scope
		reqList := httptest.NewRequest(http.MethodGet, "/api/v1/residents?rt_id="+rtOutID, nil)
		reqList.Header.Set("Authorization", "Bearer "+saToken)
		recList := httptest.NewRecorder()
		r.ServeHTTP(recList, reqList)
		require.Equal(t, http.StatusForbidden, recList.Code, "ListResidents out of scope must be 403: %s", recList.Body.String())

		// GetResident out of scope (even without ?rt_id= fallback to GetAllResidentByID)
		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+outResID, nil)
		reqGet.Header.Set("Authorization", "Bearer "+saToken)
		recGet := httptest.NewRecorder()
		r.ServeHTTP(recGet, reqGet)
		require.Equal(t, http.StatusForbidden, recGet.Code, "GetResident out of scope must be 403: %s", recGet.Body.String())

		// UpdateResident out of scope
		reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+outResID, bytes.NewBufferString(`{"full_name":"Hijack"}`))
		reqPatch.Header.Set("Content-Type", "application/json")
		reqPatch.Header.Set("Authorization", "Bearer "+saToken)
		recPatch := httptest.NewRecorder()
		r.ServeHTTP(recPatch, reqPatch)
		require.Equal(t, http.StatusForbidden, recPatch.Code, "UpdateResident out of scope must be 403: %s", recPatch.Body.String())

		// DeactivateResident out of scope
		reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/residents/"+outResID, nil)
		reqDel.Header.Set("Authorization", "Bearer "+saToken)
		recDel := httptest.NewRecorder()
		r.ServeHTTP(recDel, reqDel)
		require.Equal(t, http.StatusForbidden, recDel.Code, "DeleteResident out of scope must be 403: %s", recDel.Body.String())

		// UpdateSpecialResident out of scope
		reqPatchSpec := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/special/"+outSpecResID, bytes.NewBufferString(`{"full_name":"Hijack Spec"}`))
		reqPatchSpec.Header.Set("Content-Type", "application/json")
		reqPatchSpec.Header.Set("Authorization", "Bearer "+saToken)
		recPatchSpec := httptest.NewRecorder()
		r.ServeHTTP(recPatchSpec, reqPatchSpec)
		require.Equal(t, http.StatusForbidden, recPatchSpec.Code, "UpdateSpecialResident out of scope must be 403: %s", recPatchSpec.Body.String())
	})

	t.Run("Tenant Pengurus RT 001 cannot access RT 002 via rt_id query, X-RT-ID, body, or GetAllResidentByID", func(t *testing.T) {
		// GET RT 002 resident with ?rt_id=rt2 and X-RT-ID: rt2
		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+rt2ResID+"?rt_id="+rt2ID, nil)
		reqGet.Header.Set("Authorization", "Bearer "+ketua1Token)
		reqGet.Header.Set("X-RT-ID", rt2ID)
		recGet := httptest.NewRecorder()
		r.ServeHTTP(recGet, reqGet)
		require.Equal(t, http.StatusNotFound, recGet.Code, "RT1 pengurus must NOT read RT2 resident: %s", recGet.Body.String())

		// PATCH RT 002 resident
		reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/"+rt2ResID, bytes.NewBufferString(`{"full_name":"CrossRT"}`))
		reqPatch.Header.Set("Content-Type", "application/json")
		reqPatch.Header.Set("Authorization", "Bearer "+ketua1Token)
		reqPatch.Header.Set("X-RT-ID", rt2ID)
		recPatch := httptest.NewRecorder()
		r.ServeHTTP(recPatch, reqPatch)
		require.Equal(t, http.StatusNotFound, recPatch.Code, "RT1 pengurus must NOT update RT2 resident")

		// DELETE RT 002 resident
		reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/residents/"+rt2ResID, nil)
		reqDel.Header.Set("Authorization", "Bearer "+ketua1Token)
		reqDel.Header.Set("X-RT-ID", rt2ID)
		recDel := httptest.NewRecorder()
		r.ServeHTTP(recDel, reqDel)
		require.Equal(t, http.StatusNotFound, recDel.Code, "RT1 pengurus must NOT delete RT2 resident")

		// POST /residents/special attempting foreign rt_id=rt2
		bSpec, _ := json.Marshal(map[string]any{
			"rt_id": rt2ID, "jabatan": "anggota_rw", "full_name": "CrossRT Anggota",
			"nik": fmt.Sprintf("3173%012d", (runID/10+62)%1000000000000),
			"phone": "081299990062", "email": fmt.Sprintf("cross.arw.%d@test.com", runID), "password": "Password123!",
		})
		reqSpec := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewReader(bSpec))
		reqSpec.Header.Set("Content-Type", "application/json")
		reqSpec.Header.Set("Authorization", "Bearer "+ketua1Token)
		recSpec := httptest.NewRecorder()
		r.ServeHTTP(recSpec, reqSpec)
		require.Equal(t, http.StatusForbidden, recSpec.Code, "RT1 pengurus must NOT create special resident in RT2")
	})

	t.Run("Anggota RW parity (SuperAdmin vs Ketua RT) + contact privacy exact ketua", func(t *testing.T) {
		saEmail := fmt.Sprintf("parity.sa.%d@test.com", runID)
		saPhone := fmt.Sprintf("0813%08d", (runID/10+70)%100000000)
		saNik := fmt.Sprintf("3173%012d", (runID/10+70)%1000000000000)
		bSA, _ := json.Marshal(map[string]any{
			"rt_id": rt1ID, "jabatan": "anggota_rw", "full_name": "Anggota RW Parity SA",
			"nik": saNik, "phone": saPhone, "email": saEmail, "password": "Password123!",
		})
		reqSA := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewReader(bSA))
		reqSA.Header.Set("Content-Type", "application/json")
		reqSA.Header.Set("Authorization", "Bearer "+saToken)
		recSA := httptest.NewRecorder()
		r.ServeHTTP(recSA, reqSA)
		require.Equal(t, http.StatusCreated, recSA.Code, "SA create Anggota RW: %s", recSA.Body.String())

		var createdSA Resident
		require.NoError(t, json.Unmarshal(recSA.Body.Bytes(), &createdSA))
		require.Equal(t, rt1ID, createdSA.RTID)
		require.NotNil(t, createdSA.UserID)
		require.NotNil(t, createdSA.Jabatan)
		require.Equal(t, "anggota_rw", *createdSA.Jabatan)
		require.Nil(t, createdSA.Phone, "SA must NOT receive phone")
		require.Nil(t, createdSA.Email, "SA must NOT receive email")
		t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, *createdSA.UserID) })

		// Verify DB state: linked user, active membership with jabatan=anggota_rw, zero residency_periods (no household)
		var memJab string
		var memActive bool
		require.NoError(t, db.QueryRowContext(ctx,
			`SELECT jabatan, is_active FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`,
			*createdSA.UserID, rt1ID,
		).Scan(&memJab, &memActive))
		require.Equal(t, "anggota_rw", memJab)
		require.True(t, memActive)

		var rpCount int
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1`, createdSA.ID).Scan(&rpCount))
		require.Equal(t, 0, rpCount, "household/residency_periods must not be required for anggota_rw")

		// Duplicate email rejected with 409 (no duplicate user/resident)
		reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewReader(bSA))
		reqDup.Header.Set("Content-Type", "application/json")
		reqDup.Header.Set("Authorization", "Bearer "+saToken)
		recDup := httptest.NewRecorder()
		r.ServeHTTP(recDup, reqDup)
		require.Equal(t, http.StatusConflict, recDup.Code)

		var userCount int
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = LOWER($1)`, saEmail).Scan(&userCount))
		require.Equal(t, 1, userCount, "no duplicate user row")

		// Exact Ketua RT 001 sees phone & email; Sekretaris RT 001 & SuperAdmin do NOT
		reqK1 := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+createdSA.ID, nil)
		reqK1.Header.Set("Authorization", "Bearer "+ketua1Token)
		recK1 := httptest.NewRecorder()
		r.ServeHTTP(recK1, reqK1)
		require.Equal(t, http.StatusOK, recK1.Code)
		var detailK1 Resident
		require.NoError(t, json.Unmarshal(recK1.Body.Bytes(), &detailK1))
		require.NotNil(t, detailK1.Phone, "exact ketua RT 001 must see phone")
		require.NotNil(t, detailK1.Email, "exact ketua RT 001 must see email")

		reqSek := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+createdSA.ID, nil)
		reqSek.Header.Set("Authorization", "Bearer "+sek1Token)
		recSek := httptest.NewRecorder()
		r.ServeHTTP(recSek, reqSek)
		require.Equal(t, http.StatusOK, recSek.Code)
		var detailSek Resident
		require.NoError(t, json.Unmarshal(recSek.Body.Bytes(), &detailSek))
		require.Nil(t, detailSek.Phone, "sekretaris must NOT see phone")
		require.Nil(t, detailSek.Email, "sekretaris must NOT see email")

		// SuperAdmin detail & list return identical record ID + redacted contact
		reqSADetail := httptest.NewRequest(http.MethodGet, "/api/v1/residents/"+createdSA.ID, nil)
		reqSADetail.Header.Set("Authorization", "Bearer "+saToken)
		recSADetail := httptest.NewRecorder()
		r.ServeHTTP(recSADetail, reqSADetail)
		require.Equal(t, http.StatusOK, recSADetail.Code)
		var detailSA Resident
		require.NoError(t, json.Unmarshal(recSADetail.Body.Bytes(), &detailSA))
		require.Equal(t, createdSA.ID, detailSA.ID)
		require.Nil(t, detailSA.Phone)
		require.Nil(t, detailSA.Email)
	})
}
