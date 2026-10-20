package rt

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"social-finance/internal/auth"
	"social-finance/internal/testutil"
)

func insertAdminRTMembership(t *testing.T, rtID, userID, membershipID string, role string) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	_, err := pool.Raw().Exec(`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active) VALUES ($1, $2, 'dummy', $3, NULL, true) ON CONFLICT (id) DO NOTHING`, userID, userID+"@test.local", "Test Member")
	if err != nil {
		t.Fatalf("insert administration user: %v", err)
	}
	_, err = pool.Raw().Exec(`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES ($1, $2, $3, $4, true)`, membershipID, userID, rtID, role)
	if err != nil {
		t.Fatalf("insert administration membership: %v", err)
	}
}

func TestAdministrationJabatanLifecycleAndAudit(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	rtID := insertRT(t, pool, "Admin Lifecycle RT", 88, "AL01")
	userID := uuid.NewString()
	membershipID := uuid.NewString()
	insertAdminRTMembership(t, rtID, userID, membershipID, string(auth.RolePengurus))
	token := superAdminToken(t)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(http.MethodPut, "/api/v1/rts/"+rtID+"/memberships/"+userID+"/jabatan", `{"jabatan":"anggota"}`); rec.Code != http.StatusOK {
		t.Fatalf("assign anggota: got %d %s", rec.Code, rec.Body.String())
	}
	if rec := request(http.MethodPut, "/api/v1/rts/"+rtID+"/memberships/"+userID+"/jabatan", `{"jabatan":"sekretaris"}`); rec.Code != http.StatusOK {
		t.Fatalf("replace jabatan: got %d %s", rec.Code, rec.Body.String())
	}
	if rec := request(http.MethodDelete, "/api/v1/rts/"+rtID+"/memberships/"+userID+"/jabatan", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke jabatan: got %d %s", rec.Code, rec.Body.String())
	}

	var count int
	if err := pool.Raw().QueryRow(`SELECT count(*) FROM audit_logs WHERE rt_id = $1 AND entity_type = 'user_rt_membership.jabatan'`, rtID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected exactly 3 jabatan audit logs, got %d", count)
	}
	var remaining string
	if err := pool.Raw().QueryRow(`SELECT coalesce(jabatan, '') FROM user_rt_memberships WHERE id = $1`, membershipID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != "" {
		t.Fatalf("expected jabatan to be revoked, got %q", remaining)
	}
}

func TestAdministrationJabatanRejectsNonSuperAdmin(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	rtID := insertRT(t, pool, "Admin Auth RT", 88, "AA01")
	userID, membershipID := uuid.NewString(), uuid.NewString()
	insertAdminRTMembership(t, rtID, userID, membershipID, string(auth.RoleWarga))
	auth.SigningSecret = []byte(testSigningKey)
	claims := auth.TokenClaims{UserID: userID, MID: membershipID, RTID: rtID, Role: string(auth.RoleWarga)}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/rts/"+rtID+"/memberships/"+userID+"/jabatan", bytes.NewBufferString(`{"jabatan":"ketua"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected non-super admin to be rejected with 403, got %d", rec.Code)
	}
}

func TestAdministrationConcurrentKetuaAssignmentHasSingleWinner(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	rtID := insertRT(t, pool, "Admin Concurrent RT", 88, "AC01")
	userA, userB := uuid.NewString(), uuid.NewString()
	insertAdminRTMembership(t, rtID, userA, uuid.NewString(), string(auth.RolePengurus))
	insertAdminRTMembership(t, rtID, userB, uuid.NewString(), string(auth.RolePengurus))
	token := superAdminToken(t)
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for _, userID := range []string{userA, userB} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/rts/"+rtID+"/memberships/"+id+"/jabatan", bytes.NewBufferString(`{"jabatan":"ketua"}`))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			results <- rec.Code
		}(userID)
	}
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for code := range results {
		if code == http.StatusOK {
			ok++
		}
		if code == http.StatusConflict {
			conflict++
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("expected one successful Ketua and one conflict, got success=%d conflict=%d", ok, conflict)
	}
	var count int
	if err := pool.Raw().QueryRowContext(context.Background(), `SELECT count(*) FROM user_rt_memberships WHERE rt_id = $1 AND jabatan = 'ketua'`, rtID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one Ketua, got %d", count)
	}
}
