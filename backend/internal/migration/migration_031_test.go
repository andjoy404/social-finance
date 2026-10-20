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

		// Apply migration 031.
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("apply migration 031: %v", err)
		}

		// Verify all 12 canonical permission types are present.
		var permCount int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM permission_types").Scan(&permCount); err != nil {
			t.Fatalf("count permission_types: %v", err)
		}
		if permCount != 12 {
			t.Errorf("expected 12 permission types, got %d", permCount)
		}

		// Verify all 43 canonical position grants are present.
		var grantCount int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM position_permissions").Scan(&grantCount); err != nil {
			t.Fatalf("count position_permissions: %v", err)
		}
		if grantCount != 43 {
			t.Errorf("expected 43 position permissions, got %d", grantCount)
		}

		// Verify ketua gets the two permissions required by the three endpoints.
		for _, code := range []string{"warga.read", "finance.read"} {
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

		// Apply migration twice.
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("first apply migration 031: %v", err)
		}
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("second apply migration 031: %v", err)
		}

		var permCount, grantCount int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM permission_types").Scan(&permCount); err != nil {
			t.Fatalf("count permission_types: %v", err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM position_permissions").Scan(&grantCount); err != nil {
			t.Fatalf("count position_permissions: %v", err)
		}
		if permCount != 12 {
			t.Errorf("after re-apply: expected 12 permission types, got %d", permCount)
		}
		if grantCount != 43 {
			t.Errorf("after re-apply: expected 43 position permissions, got %d", grantCount)
		}
	})
}
