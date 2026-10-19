package rt

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"social-finance/internal/auth"
)

func TestAdministrationPermissionFollowsCurrentJabatan(t *testing.T) {
	pool := testPool(t)
	rtID := insertRT(t, pool, "Admin Permission RT", 88, "AP01")
	userID, membershipID := uuid.NewString(), uuid.NewString()
	insertAdminRTMembership(t, rtID, userID, membershipID, "pengurus")
	if _, err := pool.Raw().Exec(`UPDATE user_rt_memberships SET jabatan = 'ketua' WHERE id = $1`, membershipID); err != nil {
		t.Fatal(err)
	}

	oldTokenContext := &auth.AuthContext{UserID: userID, MembershipID: membershipID, RTID: rtID, Jabatan: auth.JabatanKetua}
	tx, err := pool.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := auth.NewPermissionResolver(pool).HasPermission(context.Background(), tx, oldTokenContext, "finance.create")
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	tx.Rollback()
	if !allowed {
		t.Fatal("current ketua jabatan should have finance.create")
	}

	if _, err := pool.Raw().Exec(`UPDATE user_rt_memberships SET jabatan = NULL WHERE id = $1`, membershipID); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	allowed, err = auth.NewPermissionResolver(pool).HasPermission(context.Background(), tx, oldTokenContext, "finance.create")
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	tx.Rollback()
	if allowed {
		t.Fatal("stale ketua context must not retain permission after jabatan revocation")
	}
}

func TestAdministrationRevokePreservesResidentAndFinancialHistory(t *testing.T) {
	pool := testPool(t)
	uniqSuffix := uuid.NewString()[:8]
	rtID := insertRT(t, pool, "Admin History RT "+uniqSuffix, 880+int(time.Now().UnixNano()%100), "H"+uniqSuffix)
	t.Cleanup(func() {
		pool.Raw().Exec(`DELETE FROM transactions WHERE rt_id = $1`, rtID)
		pool.Raw().Exec(`DELETE FROM financial_categories WHERE rt_id = $1`, rtID)
		pool.Raw().Exec(`DELETE FROM residents WHERE rt_id = $1`, rtID)
		pool.Raw().Exec(`DELETE FROM user_rt_memberships WHERE rt_id = $1`, rtID)
		pool.Raw().Exec(`DELETE FROM rts WHERE id = $1`, rtID)
	})
	userID, membershipID := uuid.NewString(), uuid.NewString()
	insertAdminRTMembership(t, rtID, userID, membershipID, "pengurus")
	if _, err := pool.Raw().Exec(`UPDATE user_rt_memberships SET jabatan = 'ketua' WHERE id = $1`, membershipID); err != nil {
		t.Fatal(err)
	}
	residentID := uuid.NewString()
	if _, err := pool.Raw().Exec(`INSERT INTO residents (id, rt_id, full_name, phone, is_active) VALUES ($1, $2, 'History Resident', '0812', true)`, residentID, rtID); err != nil {
		t.Fatal(err)
	}
	categoryID := uuid.NewString()
	if _, err := pool.Raw().Exec(`INSERT INTO financial_categories (id, rt_id, name, type, is_active) VALUES ($1, $2, 'History Income', 'income', true)`, categoryID, rtID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(`INSERT INTO transactions (rt_id, category_id, amount, type, status, description, occurred_at) VALUES ($1, $2, 100.00, 'income', 'posted', 'History transaction', CURRENT_TIMESTAMP)`, rtID, categoryID); err != nil {
		t.Fatal(err)
	}

	var residentsBefore, transactionsBefore int
	if err := pool.Raw().QueryRow(`SELECT count(*) FROM residents WHERE rt_id = $1`, rtID).Scan(&residentsBefore); err != nil {
		t.Fatal(err)
	}
	if err := pool.Raw().QueryRow(`SELECT count(*) FROM transactions WHERE rt_id = $1`, rtID).Scan(&transactionsBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(`UPDATE user_rt_memberships SET jabatan = NULL WHERE id = $1`, membershipID); err != nil {
		t.Fatal(err)
	}
	var residentsAfter, transactionsAfter int
	if err := pool.Raw().QueryRow(`SELECT count(*) FROM residents WHERE rt_id = $1`, rtID).Scan(&residentsAfter); err != nil {
		t.Fatal(err)
	}
	if err := pool.Raw().QueryRow(`SELECT count(*) FROM transactions WHERE rt_id = $1`, rtID).Scan(&transactionsAfter); err != nil {
		t.Fatal(err)
	}
	if residentsAfter != residentsBefore || transactionsAfter != transactionsBefore {
		t.Fatalf("history changed after revoke: residents %d->%d transactions %d->%d", residentsBefore, residentsAfter, transactionsBefore, transactionsAfter)
	}
	var jabatan sql.NullString
	if err := pool.Raw().QueryRow(`SELECT jabatan FROM user_rt_memberships WHERE id = $1`, membershipID).Scan(&jabatan); err != nil {
		t.Fatal(err)
	}
	if jabatan.Valid {
		t.Fatalf("jabatan should be null, got %q", jabatan.String)
	}
}
