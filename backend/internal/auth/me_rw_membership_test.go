package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"social-finance/internal/testutil"
)

// seedRWAndRT inserts an RW and its parent RT in the correct order.
func seedRWAndRT(t *testing.T, ctx context.Context, tx *sql.Tx, rwID uuid.UUID, rwName string, rwNumber int, rtID uuid.UUID, rtName, rtCode string) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO rws (id, name, rw_number, is_active)
		VALUES ($1, $2, $3, true)
	`, rwID, rwName, rwNumber)
	if err != nil {
		t.Fatalf("insert rw: %v", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO rts (id, name, rt, rw, rw_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, rtID, rtName, rtCode, rwNumber, rwID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}
}

// makeMeRequest creates a GET /api/v1/auth/me request with the given AuthContext
// injected directly into the request context (bypassing RequireAuth JWT validation).
func makeMeRequest(handler http.Handler, ac *AuthContext) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), AuthContextKey, ac))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestMeEndpointRWMembershipsSingle verifies /api/v1/auth/me returns rw_memberships
// for a user with a single RW membership.
func TestMeEndpointRWMembershipsSingle(t *testing.T) {
	pool := testutil.GetTestPool(t)

	ctx := context.Background()
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	userID := uuid.New()
	rtID := uuid.New()
	rwID := uuid.New()
	membershipID := uuid.New()
	rtCode := fmt.Sprintf("%03d", 900+int(userID[0])%100)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, full_name, email, password_hash, is_active, system_role)
		VALUES ($1, 'Test User', $2, 'hash', true, NULL)
	`, userID, fmt.Sprintf("me.single.%s@test.com", userID))
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	seedRWAndRT(t, ctx, tx, rwID, "RW Test", 2, rtID, "RT Single", rtCode)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'anggota', true)
	`, membershipID, userID, rtID)
	if err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, is_active)
		VALUES ($1, $2, 'warga', 'sekretaris', true)
	`, userID, rwID)
	if err != nil {
		t.Fatalf("insert rw membership: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	ac := &AuthContext{
		UserID:         userID.String(),
		MembershipID:   membershipID.String(),
		RTID:           rtID.String(),
		SystemRole:     "",
		TenantRole:     RoleWarga,
		Jabatan:        JabatanAnggota,
		RWMemberships: []RWMembership{
			{RWID: rwID.String(), Name: "RW Test", Jabatan: "sekretaris"},
		},
	}

	svc := NewService()
	handler := NewHandler(pool, svc)

	rec := makeMeRequest(handler.Me(), ac)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d: %s", rec.Code, rec.Body.String())
	}

	var meResp UserForMe
	if err := json.NewDecoder(rec.Body).Decode(&meResp); err != nil {
		t.Fatalf("failed to decode /me response: %v", err)
	}

	if len(meResp.RWMemberships) != 1 {
		t.Errorf("expected 1 rw_membership, got %d: %+v", len(meResp.RWMemberships), meResp.RWMemberships)
	}
	if meResp.RWMemberships[0].RWID != rwID.String() {
		t.Errorf("expected rw_id %s, got %s", rwID, meResp.RWMemberships[0].RWID)
	}
	if meResp.RWMemberships[0].Name != "RW Test" {
		t.Errorf("expected rw_name 'RW Test', got %s", meResp.RWMemberships[0].Name)
	}
}

// TestMeEndpointRWMembershipsMultiple verifies /api/v1/auth/me returns multiple rw_memberships.
func TestMeEndpointRWMembershipsMultiple(t *testing.T) {
	pool := testutil.GetTestPool(t)

	ctx := context.Background()
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	userID := uuid.New()
	rtID := uuid.New()
	rwID1 := uuid.New()
	rwID2 := uuid.New()
	membershipID := uuid.New()
	rtCode := fmt.Sprintf("%03d", 900+int(userID[0])%100)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, full_name, email, password_hash, is_active, system_role)
		VALUES ($1, 'Multi User', $2, 'hash', true, NULL)
	`, userID, fmt.Sprintf("me.multi.%s@test.com", userID))
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	seedRWAndRT(t, ctx, tx, rwID1, "RW Test", 1, rtID, "RT Multi", rtCode)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO rws (id, name, rw_number, is_active)
		VALUES ($1, 'RW 002', 2, true)
	`, rwID2)
	if err != nil {
		t.Fatalf("insert rw2: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'anggota', true)
	`, membershipID, userID, rtID)
	if err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, is_active)
		VALUES ($1, $2, 'warga', 'sekretaris', true), ($1, $3, 'warga', 'bendahara', true)
	`, userID, rwID1, rwID2)
	if err != nil {
		t.Fatalf("insert rw memberships: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	ac := &AuthContext{
		UserID:         userID.String(),
		MembershipID:   membershipID.String(),
		RTID:           rtID.String(),
		SystemRole:     "",
		TenantRole:     RoleWarga,
		Jabatan:        JabatanAnggota,
		RWMemberships: []RWMembership{
			{RWID: rwID1.String(), Name: "RW Test", Jabatan: "sekretaris"},
			{RWID: rwID2.String(), Name: "RW Test 2", Jabatan: "bendahara"},
		},
	}

	svc := NewService()
	handler := NewHandler(pool, svc)

	rec := makeMeRequest(handler.Me(), ac)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d: %s", rec.Code, rec.Body.String())
	}

	var meResp UserForMe
	if err := json.NewDecoder(rec.Body).Decode(&meResp); err != nil {
		t.Fatalf("failed to decode /me response: %v", err)
	}

	if len(meResp.RWMemberships) != 2 {
		t.Errorf("expected 2 rw_memberships, got %d: %+v", len(meResp.RWMemberships), meResp.RWMemberships)
	}
}

// TestMeEndpointNoRWMembership verifies /api/v1/auth/me returns empty rw_memberships
// for a user without any RW membership.
func TestMeEndpointNoRWMembership(t *testing.T) {
	pool := testutil.GetTestPool(t)

	ctx := context.Background()
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	userID := uuid.New()
	rtID := uuid.New()
	membershipID := uuid.New()
	rtCode := fmt.Sprintf("%03d", 900+int(userID[0])%100)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, full_name, email, password_hash, is_active, system_role)
		VALUES ($1, 'No RW User', $2, 'hash', true, NULL)
	`, userID, fmt.Sprintf("me.none.%s@test.com", userID))
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO rts (id, name, rt, rw, rw_id, is_active)
		VALUES ($1, 'RT None', $2, 998, NULL, true)
	`, rtID, rtCode)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'anggota', true)
	`, membershipID, userID, rtID)
	if err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	ac := &AuthContext{
		UserID:         userID.String(),
		MembershipID:   membershipID.String(),
		RTID:           rtID.String(),
		SystemRole:     "",
		TenantRole:     RoleWarga,
		Jabatan:        JabatanAnggota,
		RWMemberships:  nil,
	}

	svc := NewService()
	handler := NewHandler(pool, svc)

	rec := makeMeRequest(handler.Me(), ac)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d: %s", rec.Code, rec.Body.String())
	}

	var meResp UserForMe
	if err := json.NewDecoder(rec.Body).Decode(&meResp); err != nil {
		t.Fatalf("failed to decode /me response: %v", err)
	}

	if len(meResp.RWMemberships) != 0 {
		t.Errorf("expected 0 rw_memberships, got %d: %+v", len(meResp.RWMemberships), meResp.RWMemberships)
	}
}

// TestRequireRWScopeRejectsOutOfScopeRW verifies that RequireRWScope rejects
// an rwId that is not in the user's memberships.
func TestRequireRWScopeRejectsOutOfScopeRW(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleBendahara,
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Name: "RW 1", Jabatan: "ketua"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-2/beranda", cfg)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for out-of-scope rwId, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireRWScopeAcceptsInScopeRW verifies that RequireRWScope accepts
// an rwId that is in the user's memberships.
func TestRequireRWScopeAcceptsInScopeRW(t *testing.T) {
	cfg := ScopingConfig{
		RTIDs: []string{"rt-uuid-1"},
		RWIDs: []string{"rw-uuid-1", "rw-uuid-2"},
	}

	ac := &AuthContext{
		UserID:     "user-id",
		RTID:       "rt-uuid-1",
		TenantRole: RoleBendahara,
		RWMemberships: []RWMembership{
			{RWID: "rw-uuid-1", Name: "RW 1", Jabatan: "ketua"},
			{RWID: "rw-uuid-2", Name: "RW 2", Jabatan: "bendahara"},
		},
	}

	rec := callRequireRWScopeMiddleware(ac, "/api/v1/rw/rw-uuid-2/beranda", cfg)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for in-scope rwId, got %d, body: %s", rec.Code, rec.Body.String())
	}
}
