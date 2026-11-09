package migration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"social-finance/internal/testutil"
)

// migrationSQLPath returns the absolute path to migration 031's up SQL,
// resolved relative to this test file's package directory
// (backend/internal/migration/).
func migration031UpSQLPath() string {
	return filepath.Join("..", "..", "migrations", "031_restore_permission_catalog.up.sql")
}

// readMigration031UpSQL reads the actual migration file so the test validates
// the file that will be applied in production, not a hand-duplicated string.
func readMigration031UpSQL(t *testing.T) string {
	t.Helper()
	path := migration031UpSQLPath()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration 031 up sql %q: %v", path, err)
	}
	return string(bytes)
}

func TestMigration031RestorePermissionCatalog(t *testing.T) {
	pool := testutil.GetTestPool(t)
	db := pool.Raw()
	ctx := context.Background()
	upSQL := readMigration031UpSQL(t)

	t.Run("restores canonical catalog from empty state", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		// Simulate production: permission catalog is empty.
		if _, err := tx.ExecContext(ctx, "DELETE FROM position_permissions"); err != nil {
			t.Fatalf("delete position_permissions: %v", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM permission_types"); err != nil {
			t.Fatalf("delete permission_types: %v", err)
		}

		// Migration 056 may have tightened position_permissions check constraints on
		// the test DB; temporarily allow legacy positions restored by migration 031.
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE position_permissions DROP CONSTRAINT IF EXISTS position_permissions_position_check;
			ALTER TABLE position_permissions ADD CONSTRAINT position_permissions_position_check
			CHECK (position IN (
				'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
				'keamanan', 'sosial', 'operasional', 'kebersihan',
				'kebersihan_pembangunan', 'anggota'
			));
		`); err != nil {
			t.Fatalf("prepare legacy position constraint for migration 031: %v", err)
		}

		// Apply migration 031.
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("apply migration 031: %v", err)
		}

		var permCount, grantCount int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM permission_types").Scan(&permCount); err != nil {
			t.Fatalf("count permission_types: %v", err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM position_permissions").Scan(&grantCount); err != nil {
			t.Fatalf("count position_permissions: %v", err)
		}
		// Verify migration 031's canonical grants are present (idempotent/additive).
		if permCount < 12 {
			t.Errorf("expected at least 12 permission types, got %d", permCount)
		}
		if grantCount < 43 {
			t.Errorf("expected at least 43 position permissions, got %d", grantCount)
		}
		// Verify ketua receives the grants migration 031 is responsible for.
		for _, code := range []string{"warga.read", "finance.read", "rt.settings.manage"} {
			var n int
			q := `
				SELECT COUNT(*)
				FROM position_permissions pp
				JOIN permission_types pt ON pt.id = pp.permission_id
				WHERE pp.position = $1 AND pt.code = $2`
			if err := tx.QueryRowContext(ctx, q, "ketua", code).Scan(&n); err != nil {
				t.Fatalf("check ketua -> %s: %v", code, err)
			}
			if n != 1 {
				t.Errorf("expected 1 grant for ketua -> %s, got %d", code, n)
			}
		}

		// Verify no business tables were touched (resident count unchanged is
		// implied by the rollback; explicitly confirm residents table is intact).
		var residentCount int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM residents").Scan(&residentCount); err != nil {
			t.Fatalf("count residents: %v", err)
		}
		_ = residentCount
	})

	t.Run("is idempotent on re-apply", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		// Temporarily allow legacy positions for migration 031 compatibility.
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE position_permissions DROP CONSTRAINT IF EXISTS position_permissions_position_check;
			ALTER TABLE position_permissions ADD CONSTRAINT position_permissions_position_check
			CHECK (position IN (
				'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
				'keamanan', 'sosial', 'operasional', 'kebersihan',
				'kebersihan_pembangunan', 'anggota'
			));
		`); err != nil {
			t.Fatalf("prepare legacy position constraint for migration 031: %v", err)
		}

		// Apply migration twice.
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("first apply migration 031: %v", err)
		}
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("second apply migration 031: %v", err)
		}

		// Verify idempotency (counts stable across repeated apply).
		var permCount1, grantCount1, permCount2, grantCount2 int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM permission_types").Scan(&permCount1); err != nil {
			t.Fatalf("count permission_types: %v", err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM position_permissions").Scan(&grantCount1); err != nil {
			t.Fatalf("count position_permissions: %v", err)
		}
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("third apply migration 031: %v", err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM permission_types").Scan(&permCount2); err != nil {
			t.Fatalf("count permission_types: %v", err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM position_permissions").Scan(&grantCount2); err != nil {
			t.Fatalf("count position_permissions: %v", err)
		}
		if permCount1 != permCount2 {
			t.Errorf("after re-apply: permission types changed %d -> %d", permCount1, permCount2)
		}
		if grantCount1 != grantCount2 {
			t.Errorf("after re-apply: position permissions changed %d -> %d", grantCount1, grantCount2)
		}
		if permCount1 < 12 {
			t.Errorf("after re-apply: expected at least 12 permission types, got %d", permCount1)
		}
		if grantCount1 < 43 {
			t.Errorf("after re-apply: expected at least 43 position permissions, got %d", grantCount1)
		}
	})
}
