package rt

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"social-finance/internal/finance"
)

func TestAdministrationAuditFailureRollsBackJabatan(t *testing.T) {
	pool := testPool(t)
	r := buildRouter(pool)
	rtID := insertRT(t, pool, "Admin Audit Rollback RT", 88, "AR01")
	userID, membershipID := uuid.NewString(), uuid.NewString()
	insertAdminRTMembership(t, rtID, userID, membershipID, "pengurus")
	token := superAdminToken(t)

	finance.AuditLogInsertHook = func(_ context.Context, _ *sql.Tx, _ finance.AuditLog) error {
		return errors.New("injected audit failure")
	}
	t.Cleanup(func() { finance.AuditLogInsertHook = nil })

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rts/"+rtID+"/memberships/"+userID+"/jabatan", bytes.NewBufferString(`{"jabatan":"ketua"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}

	var jabatan string
	if err := pool.Raw().QueryRow(`SELECT COALESCE(jabatan, '') FROM user_rt_memberships WHERE id = $1`, membershipID).Scan(&jabatan); err != nil {
		t.Fatal(err)
	}
	if jabatan != "" {
		t.Fatalf("audit failure must rollback jabatan, got %q", jabatan)
	}
	var logs int
	if err := pool.Raw().QueryRow(`SELECT count(*) FROM audit_logs WHERE rt_id = $1 AND entity_id = $2`, rtID, membershipID).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if logs != 0 {
		t.Fatalf("audit failure must not leave audit log, got %d", logs)
	}
}
