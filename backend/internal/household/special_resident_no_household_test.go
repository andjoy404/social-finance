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
	"social-finance/internal/auth"
)

// TestSpecialResidentCreationWithoutHousehold verifies that special position
// officers (keamanan, operasional, kebersihan) can be created without a
// household / household_occupancy / residency period, and are therefore not
// registered as household occupants (warga) of any physical house.
func TestSpecialResidentCreationWithoutHousehold(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()
	rtID := setupTestRT(t, pool, "RT Special No Household", 1, "SNH")

	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Post("/api/v1/residents/special", h.CreateSpecialResident)

	pengurusToken := makeTenantToken(t, "test-pengurus-special-nh", "test-pengurus-mem-special-nh", rtID, auth.RolePengurus)
	runID := time.Now().UnixNano()

	for idx, jab := range []string{"keamanan", "operasional", "kebersihan"} {
		body := fmt.Sprintf(`{"full_name":"Petugas %s","nik":"31730000000000%02d","phone":"0812999900%02d","email":"petugas.%s.%d.nh@test.com","jabatan":"%s"}`, jab, idx, idx, jab, runID, jab)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pengurusToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("CreateSpecialResident(%q) expected 201, got %d: %s", jab, rec.Code, rec.Body.String())
		}

		var created Resident
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("unmarshal created special resident: %v", err)
		}

		// The officer must not be linked to any household occupancy/residency.
		var occCount int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM residency_periods WHERE resident_id = $1`, created.ID).Scan(&occCount); err != nil {
			t.Fatalf("count residency_periods for %s: %v", created.ID, err)
		}
		if occCount != 0 {
			t.Fatalf("special officer %q must not have household residency, got %d rows", jab, occCount)
		}

		var userID, role, membershipJabatan string
		if err := db.QueryRowContext(ctx, `SELECT user_id FROM residents WHERE id = $1`, created.ID).Scan(&userID); err != nil {
			t.Fatalf("load special officer user id: %v", err)
		}
		if err := db.QueryRowContext(ctx,
			`SELECT role, jabatan FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2 AND is_active = true`,
			userID, rtID,
		).Scan(&role, &membershipJabatan); err != nil {
			t.Fatalf("load special officer membership: %v", err)
		}
		if role != string(auth.RoleWarga) || membershipJabatan != jab {
			t.Fatalf("special officer membership must be warga/%s, got %s/%s", jab, role, membershipJabatan)
		}

		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id = $1`, created.ID)
			_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, userID)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		})
	}
}

func TestCreateSpecialResidentRejectsExistingAccountFromAnotherRT(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()
	rtA := setupTestRT(t, pool, "RT Existing Account A", 1, "EAA")
	rtB := setupTestRT(t, pool, "RT Existing Account B", 1, "EAB")

	email := fmt.Sprintf("existing.%d@test.com", time.Now().UnixNano())
	var existingUserID, existingResidentID, existingMembershipID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'dummyhash', 'Existing RT A User', true) RETURNING id`,
		email, fmt.Sprintf("0812%08d", time.Now().UnixNano()%100000000),
	).Scan(&existingUserID); err != nil {
		t.Fatalf("create existing user: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		 VALUES ($1, $2, 'warga', 'keamanan', true) RETURNING id`,
		existingUserID, rtA,
	).Scan(&existingMembershipID); err != nil {
		t.Fatalf("create existing membership: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, user_id, full_name, phone, nik, email, is_active)
		 VALUES ($1, $2, 'Existing RT A User', '081200000001', '3173000000000991', $3, true)
		 RETURNING id`,
		rtA, existingUserID, email,
	).Scan(&existingResidentID); err != nil {
		t.Fatalf("create existing resident: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id = $1`, existingResidentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, existingMembershipID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, existingUserID)
	})

	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Post("/api/v1/residents/special", h.CreateSpecialResident)
	token := makeTenantToken(t, "test-special-existing-target", "test-special-existing-target-mem", rtB, auth.RolePengurus)
	body := fmt.Sprintf(`{"rt_id":%q,"jabatan":"operasional","full_name":"Attempted RT B Petugas","nik":"3173000000000992","phone":"081299990099","email":%q}`, rtB, email)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for an email already registered in another RT, got %d: %s", rec.Code, rec.Body.String())
	}

	var targetMemberships, residentCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_rt_memberships WHERE user_id = $1 AND rt_id = $2`,
		existingUserID, rtB,
	).Scan(&targetMemberships); err != nil {
		t.Fatalf("count target RT memberships: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM residents WHERE user_id = $1 AND rt_id = $2`,
		existingUserID, rtA,
	).Scan(&residentCount); err != nil {
		t.Fatalf("count original RT residents: %v", err)
	}
	if targetMemberships != 0 || residentCount != 1 {
		t.Fatalf("existing account changed across RTs: target memberships=%d original residents=%d", targetMemberships, residentCount)
	}
	var role, jabatan string
	if err := db.QueryRowContext(ctx,
		`SELECT role, jabatan FROM user_rt_memberships WHERE id = $1`,
		existingMembershipID,
	).Scan(&role, &jabatan); err != nil {
		t.Fatalf("reload original membership: %v", err)
	}
	if role != "warga" || jabatan != "keamanan" {
		t.Fatalf("existing role/jabatan changed: got %s/%s", role, jabatan)
	}
}

func TestCreateSpecialResidentRollsBackOnResidentConflict(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()
	rtID := setupTestRT(t, pool, "RT Special Rollback", 1, "SRB")
	const duplicateNik = "3173000000000994"
	var existingResidentID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, nik, is_active)
		 VALUES ($1, 'Existing Resident', $2, true) RETURNING id`,
		rtID, duplicateNik,
	).Scan(&existingResidentID); err != nil {
		t.Fatalf("create resident with duplicate NIK: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id = $1`, existingResidentID)
	})

	email := fmt.Sprintf("rollback.%d@test.com", time.Now().UnixNano())
	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Post("/api/v1/residents/special", h.CreateSpecialResident)
	token := makeTenantToken(t, "special-rollback-pengurus", "special-rollback-membership", rtID, auth.RolePengurus)
	body := fmt.Sprintf(`{"jabatan":"operasional","full_name":"Rollback Petugas","nik":%q,"phone":"081299990094","email":%q}`, duplicateNik, email)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate NIK, got %d: %s", rec.Code, rec.Body.String())
	}

	var userCount, membershipCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE lower(email) = lower($1)`, email).Scan(&userCount); err != nil {
		t.Fatalf("count partially created users: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_rt_memberships urm JOIN users u ON u.id = urm.user_id WHERE lower(u.email) = lower($1)`,
		email,
	).Scan(&membershipCount); err != nil {
		t.Fatalf("count partially created memberships: %v", err)
	}
	if userCount != 0 || membershipCount != 0 {
		t.Fatalf("failed creation left partial records: users=%d memberships=%d", userCount, membershipCount)
	}
}

func TestSpecialResidentRoutesEnforceTenantScope(t *testing.T) {
	pool := testPool(t)
	db := pool.Raw()
	ctx := context.Background()
	rtA := setupTestRT(t, pool, "RT Special Scope A", 1, "SSA")
	rtB := setupTestRT(t, pool, "RT Special Scope B", 1, "SSB")

	h := NewHandler(pool)
	r := chi.NewRouter()
	r.Use(auth.RequireAuth)
	r.Post("/api/v1/residents/special", h.CreateSpecialResident)
	r.Patch("/api/v1/residents/special/{id}", h.UpdateSpecialResident)
	tokenA := makeTenantToken(t, "special-scope-a", "special-scope-a-membership", rtA, auth.RolePengurus)
	tokenB := makeTenantToken(t, "special-scope-b", "special-scope-b-membership", rtB, auth.RolePengurus)

	body := fmt.Sprintf(`{"rt_id":%q,"jabatan":"keamanan","full_name":"RT A Security Officer","nik":"3173000000000993","phone":"081299990093","email":"special.scope.%d@test.com"}`, rtB, time.Now().UnixNano())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating special resident in RT A, got %d: %s", rec.Code, rec.Body.String())
	}

	var created Resident
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created special resident: %v", err)
	}
	if created.RTID != rtA {
		t.Fatalf("client-supplied rt_id changed tenant scope: expected %s, got %s", rtA, created.RTID)
	}
	var userID string
	if err := db.QueryRowContext(ctx, `SELECT user_id FROM residents WHERE id = $1`, created.ID).Scan(&userID); err != nil {
		t.Fatalf("load created resident user id: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id = $1`, created.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/residents/special/"+created.ID, bytes.NewBufferString(`{"full_name":"Cross RT Update"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+tokenB)
	updateRec := httptest.NewRecorder()
	r.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when RT B updates RT A resident, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	var fullName string
	if err := db.QueryRowContext(ctx, `SELECT full_name FROM residents WHERE id = $1`, created.ID).Scan(&fullName); err != nil {
		t.Fatalf("load resident after cross-RT update: %v", err)
	}
	if fullName != "RT A Security Officer" {
		t.Fatalf("cross-RT update changed resident name to %q", fullName)
	}
}

func TestSpecialResidentCreateAccessRoles(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := auth.RequireAccess(
		[]auth.Role{auth.RolePengurus, auth.RoleBendahara},
		[]auth.Jabatan{auth.JabatanKetua, auth.JabatanWakilKetua, auth.JabatanSekretaris, auth.JabatanBendahara},
	)(next)

	tests := []struct {
		name string
		ac   *auth.AuthContext
		want int
	}{
		{"super admin", &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin}, http.StatusNoContent},
		{"ketua", &auth.AuthContext{TenantRole: auth.RoleWarga, Jabatan: auth.JabatanKetua}, http.StatusNoContent},
		{"bendahara role", &auth.AuthContext{TenantRole: auth.RoleBendahara}, http.StatusNoContent},
		{"warga", &auth.AuthContext{TenantRole: auth.RoleWarga}, http.StatusForbidden},
		{"petugas keamanan", &auth.AuthContext{TenantRole: auth.RoleWarga, Jabatan: auth.JabatanKeamanan}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/special", nil)
			req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, tt.ac))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("expected status %d, got %d: %s", tt.want, rec.Code, rec.Body.String())
			}
		})
	}
}
