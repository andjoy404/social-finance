package auth_test

// ============================================================================
// Migration 071 regression tests — Bendahara loses notification.manage.
//
// Migration 071 is immutable and already applied in the development database.
// These tests prove the required outcome WITHOUT re-applying the migration to
// the shared test database. They work on a snapshot-restoring isolated copy of
// the position_permissions scope, so the shared state is always preserved.
// ============================================================================

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"social-finance/internal/devseed"
	"social-finance/internal/testutil"
)

func snapshotNotificationManageState(t *testing.T, ctx context.Context, db *sql.DB) (permRows map[string]permTypeRow, grantRows map[string]posPermRow) {
	t.Helper()
	permRows = map[string]permTypeRow{}
	grantRows = map[string]posPermRow{}

	q := `SELECT id::text, code, domain, operation, created_at, updated_at
	      FROM permission_types WHERE code IN ('notification.manage','notification.read','finance.approve')`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		t.Fatalf("snapshot permission_types: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r permTypeRow
		if err := rows.Scan(&r.id, &r.code, &r.domain, &r.operation, &r.createdAt, &r.updatedAt); err != nil {
			t.Fatalf("scan permType: %v", err)
		}
		permRows[r.code] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("permTypes rows: %v", err)
	}

	q = `SELECT pp.id::text, pp.position, pp.permission_id::text, pp.created_at
	      FROM position_permissions pp
	      JOIN permission_types pt ON pt.id = pp.permission_id
	      WHERE pt.code IN ('notification.manage','notification.read','finance.approve')`
	grows, err := db.QueryContext(ctx, q)
	if err != nil {
		t.Fatalf("snapshot position_permissions: %v", err)
	}
	defer grows.Close()
	for grows.Next() {
		var r posPermRow
		if err := grows.Scan(&r.id, &r.position, &r.permissionID, &r.createdAt); err != nil {
			t.Fatalf("scan grant: %v", err)
		}
		grantRows[r.position+"|"+r.permissionID] = r
	}
	if err := grows.Err(); err != nil {
		t.Fatalf("position_permissions rows: %v", err)
	}
	return permRows, grantRows
}

func restoreNotificationManageState(t *testing.T, ctx context.Context, db *sql.DB, permRows map[string]permTypeRow, grantRows map[string]posPermRow) {
	t.Helper()
	_, err := db.ExecContext(ctx, `DELETE FROM position_permissions
		WHERE permission_id IN (SELECT id FROM permission_types
		WHERE code IN ('notification.manage','notification.read','finance.approve'))`)
	if err != nil {
		t.Fatalf("restore: delete position_permissions: %v", err)
	}
	_, err = db.ExecContext(ctx, `DELETE FROM permission_types WHERE code IN ('notification.manage','notification.read','finance.approve')`)
	if err != nil {
		t.Fatalf("restore: delete permission_types: %v", err)
	}
	for code, r := range permRows {
		_, err = db.ExecContext(ctx, `
			INSERT INTO permission_types (id, code, domain, operation, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)`, r.id, r.code, r.domain, r.operation, r.createdAt, r.updatedAt)
		if err != nil {
			t.Fatalf("restore: insert permission_type %s: %v", code, err)
		}
	}
	for _, r := range grantRows {
		_, err = db.ExecContext(ctx, `
			INSERT INTO position_permissions (id, position, permission_id, created_at)
			VALUES ($1::uuid, $2, $3::uuid, $4)
			ON CONFLICT (position, permission_id) DO NOTHING`, r.id, r.position, r.permissionID, r.createdAt)
		if err != nil {
			t.Fatalf("restore: insert position_permission %s/%s: %v", r.position, r.permissionID, err)
		}
	}
}

func with071Tx(t *testing.T, fn func(ctx context.Context, t *testing.T, tx *sql.Tx)) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	permSnap, grantSnap := snapshotNotificationManageState(t, ctx, pool.Raw())
	tx, err := pool.Raw().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
		restoreNotificationManageState(t, ctx, pool.Raw(), permSnap, grantSnap)
	}()
	fn(ctx, t, tx)
}

func runMigration071(t *testing.T, ctx context.Context, tx *sql.Tx, filename string) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "migrations", filename))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read migration %s: %v", filename, err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	var b strings.Builder
	sp := 0
	for _, line := range strings.Split(text, "\n") {
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "begin;":
			sp++
			b.WriteString(fmt.Sprintf("SAVEPOINT mig_sp_%d;\n", sp))
		case "commit;":
			b.WriteString(fmt.Sprintf("RELEASE SAVEPOINT mig_sp_%d;\n", sp))
		default:
			b.WriteString(line + "\n")
		}
	}
	if _, err := tx.ExecContext(ctx, b.String()); err != nil {
		t.Fatalf("execute migration %s: %v", filename, err)
	}
}

func count071(t *testing.T, ctx context.Context, tx *sql.Tx, code, position string) int {
	t.Helper()
	var n int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM position_permissions pp
		JOIN permission_types pt ON pt.id = pp.permission_id
		WHERE pt.code = $1 AND pp.position = $2`, code, position).Scan(&n); err != nil {
		t.Fatalf("count(%s,%s): %v", code, position, err)
	}
	return n
}

// TestMigration071_RollbackGrantIsUnconditional documents that the shipped
// migration 071 down unconditionally re-grants notification.manage to
// bendahara. This is an unqualified rollback; it is recorded here as the
// documented rollback contract rather than as an invitation to execute it.
func TestMigration071_RollbackGrantIsUnconditional(t *testing.T) {
	with071Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Remove existing grant if present, then apply 071 down.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM position_permissions
			WHERE permission_id = (SELECT id FROM permission_types WHERE code = 'notification.manage')
			  AND position = 'bendahara'`); err != nil {
			t.Fatalf("remove bendahara manage: %v", err)
		}
		if got := count071(t, ctx, tx, "notification.manage", "bendahara"); got != 0 {
			t.Fatalf("precondition: bendahara has manage before down, got %d", got)
		}

		runMigration071(t, ctx, tx, "071_notification_manage_revoke_bendahara.down.sql")

		if got := count071(t, ctx, tx, "notification.manage", "bendahara"); got != 1 {
			t.Fatalf("071 down must re-grant notification.manage to bendahara, got %d", got)
		}

		// Re-applying down is idempotent.
		runMigration071(t, ctx, tx, "071_notification_manage_revoke_bendahara.down.sql")
		if got := count071(t, ctx, tx, "notification.manage", "bendahara"); got != 1 {
			t.Fatalf("071 down idempotency: got %d", got)
		}

		// Re-applying up must revoke again.
		runMigration071(t, ctx, tx, "071_notification_manage_revoke_bendahara.up.sql")
		if got := count071(t, ctx, tx, "notification.manage", "bendahara"); got != 0 {
			t.Fatalf("071 up must revoke manage from bendahara, got %d", got)
		}
	})
}

// TestMigration071_UpPreservesOtherPermissions asserts the up migration only
// removes notification.manage from bendahara and touches no other grant.
func TestMigration071_UpPreservesOtherPermissions(t *testing.T) {
	with071Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Ensure a bendahara manage grant exists for the exercise — this cannot
		// occur in a migrated database (071 already ran), but the test is
		// independent of the shared state.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO permission_types (code, domain, operation)
			VALUES ('notification.manage', 'notification', 'manage')
			ON CONFLICT (code) DO NOTHING`); err != nil {
			t.Fatalf("insert notif manage perm: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT 'bendahara', id FROM permission_types WHERE code = 'notification.manage'
			ON CONFLICT DO NOTHING`); err != nil {
			t.Fatalf("insert bendahara manage: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO permission_types (code, domain, operation) VALUES
				('notification.read', 'notification', 'read'),
				('finance.approve', 'finance', 'approve')
			ON CONFLICT (code) DO NOTHING`); err != nil {
			t.Fatalf("insert notif read / finance approve: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT 'bendahara', id FROM permission_types
			WHERE code IN ('notification.read','finance.approve')
			ON CONFLICT DO NOTHING`); err != nil {
			t.Fatalf("insert bendahara notif read / finance approve: %v", err)
		}

		beforeFinance := count071(t, ctx, tx, "finance.approve", "bendahara")
		beforeNotifRead := count071(t, ctx, tx, "notification.read", "bendahara")

		runMigration071(t, ctx, tx, "071_notification_manage_revoke_bendahara.up.sql")

		if got := count071(t, ctx, tx, "notification.manage", "bendahara"); got != 0 {
			t.Fatalf("071 up must revoke bendahara notification.manage; got %d", got)
		}
		if got := count071(t, ctx, tx, "finance.approve", "bendahara"); got != beforeFinance {
			t.Fatalf("071 up changed bendahara finance.approve: before=%d after=%d", beforeFinance, got)
		}
		if got := count071(t, ctx, tx, "notification.read", "bendahara"); got != beforeNotifRead {
			t.Fatalf("071 up changed bendahara notification.read: before=%d after=%d", beforeNotifRead, got)
		}
	})
}

// TestMigration071_CleanPermissionSeed signifies what the canonical seed keeps
// after migrations 070 + 071: bendahara retains finance.approve and
// notification.read but never receives notification.manage.
func TestMigration071_CleanPermissionSeed(t *testing.T) {
	pool := testutil.GetTestPool(t)

	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("devseed: %v", err)
	}

	var notifManage, notifRead, financeApprove int
	row := pool.Raw().QueryRowContext(context.Background(),
		`SELECT
			COALESCE(SUM(CASE WHEN pt.code = 'notification.manage' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN pt.code = 'notification.read' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN pt.code = 'finance.approve' THEN 1 ELSE 0 END), 0)
		FROM position_permissions pp
		JOIN permission_types pt ON pt.id = pp.permission_id
		WHERE pp.position = 'bendahara'`)
	_ = row.Scan(&notifManage, &notifRead, &financeApprove)
	t.Logf("bendahara grants — manage=%d read=%d finance.approve=%d", notifManage, notifRead, financeApprove)
	if notifManage != 0 {
		t.Fatalf("bendahara must not have notification.manage after seed, got %d", notifManage)
	}
	if notifRead < 1 {
		t.Fatalf("bendahara must keep notification.read after seed, got %d", notifRead)
	}
	if financeApprove < 1 {
		t.Fatalf("bendahara must keep finance.approve after seed, got %d", financeApprove)
	}
}

// TestMigration071_KetuaKeepsManage guards against regression of the previous
// allowance: ketua retains notification.manage.
func TestMigration071_KetuaKeepsManage(t *testing.T) {
	pool := testutil.GetTestPool(t)

	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("devseed: %v", err)
	}

	var got int
	if err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM position_permissions pp
		JOIN permission_types pt ON pt.id = pp.permission_id
		WHERE pp.position = 'ketua' AND pt.code = 'notification.manage'`).Scan(&got); err != nil {
		t.Fatalf("count ketua manage: %v", err)
	}
	if got != 1 {
		t.Fatalf("ketua must keep notification.manage, got %d", got)
	}
}

// TestMigration071_OtherJabatanKeepManageAndPermissions guards that no position
// beyond bendahara suffers notification.manage removal and that the other
// grants are still present for every position.
func TestMigration071_OtherJabatanKeepManageAndPermissions(t *testing.T) {
	pool := testutil.GetTestPool(t)

	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("devseed: %v", err)
	}

	want := map[string][]string{
		"ketua":       {"notification.manage", "notification.read", "finance.approve"},
		"wakil_ketua": {"notification.read", "finance.approve"},
		"sekretaris":  {"notification.read"},
	}
	for position, codes := range want {
		for _, code := range codes {
			var n int
			if err := pool.Raw().QueryRowContext(context.Background(),
				`SELECT COUNT(*) FROM position_permissions pp
				JOIN permission_types pt ON pt.id = pp.permission_id
				WHERE pp.position = $1 AND pt.code = $2`, position, code).Scan(&n); err != nil {
				t.Fatalf("count(%s,%s): %v", position, code, err)
			}
			if n < 1 {
				t.Fatalf("%s lost %s — migration 071 must not touch other positions' grants", position, code)
			}
		}
	}
}
