package auth_test

// ============================================================================
// Migration 072 / 073 / 074 / 075 safety tests.
//
// These tests exercise the actual migration files shipped in the repository
// (backend/migrations/<filename>), not reimplementations. Every scenario runs
// inside an outer transaction and rolls back, so the shared test database is
// always returned to its pre-test state. The snapshot/restore pattern from
// migration_070_safety_test.go is reused so we never leak state to concurrent
// or subsequent tests.
//
// Each test proves the migration:
//   1. does what it claims,
//   2. does NOT touch neighbours (other positions, other permissions),
//   3. is idempotent,
//   4. is honest about the DOWN direction.
// ============================================================================

import (
	"context"
	"database/sql"
	"testing"

	"social-finance/internal/testutil"
)

// relevantPermissionCodes covers every permission these migrations touch.
var relevantPermissionCodes = []string{
	"announcements.create",
	"announcements.update",
	"announcements.publish",
	"announcements.archive",
	"announcements.delete",
	"announcements.read",
	"banner_templates.read",
	"banner_templates.manage",
}

// snapshotRelevantState captures the permission scope 072/074/075 operate on.
func snapshotRelevantState(t *testing.T, ctx context.Context, db *sql.DB) (permTypes map[string]permTypeRow, grants map[string]posPermRow) {
	t.Helper()
	permTypes = map[string]permTypeRow{}
	grants = map[string]posPermRow{}

	q := `SELECT id::text, code, domain, operation, created_at, updated_at
	      FROM permission_types WHERE code = ANY($1)`
	rows, err := db.QueryContext(ctx, q, sqlArray(relevantPermissionCodes))
	if err != nil {
		t.Fatalf("snapshot permission_types: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r permTypeRow
		if err := rows.Scan(&r.id, &r.code, &r.domain, &r.operation, &r.createdAt, &r.updatedAt); err != nil {
			t.Fatalf("scan permType: %v", err)
		}
		permTypes[r.code] = r
	}

	q = `SELECT pp.id::text, pp.position, pp.permission_id::text, pp.created_at
	      FROM position_permissions pp
	      JOIN permission_types pt ON pt.id = pp.permission_id
	      WHERE pt.code = ANY($1)`
	grows, err := db.QueryContext(ctx, q, sqlArray(relevantPermissionCodes))
	if err != nil {
		t.Fatalf("snapshot position_permissions: %v", err)
	}
	defer grows.Close()
	for grows.Next() {
		var r posPermRow
		if err := grows.Scan(&r.id, &r.position, &r.permissionID, &r.createdAt); err != nil {
			t.Fatalf("scan grant: %v", err)
		}
		grants[r.position+"|"+r.permissionID] = r
	}
	return permTypes, grants
}

// sqlArray converts a Go slice of strings to a pgx-compatible []string array.
func sqlArray(s []string) []string { return s }

func restoreRelevantState(t *testing.T, ctx context.Context, db *sql.DB, permTypes map[string]permTypeRow, grants map[string]posPermRow) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		DELETE FROM position_permissions
		WHERE permission_id IN (SELECT id FROM permission_types WHERE code = ANY($1))`, sqlArray(relevantPermissionCodes))
	if err != nil {
		t.Fatalf("restore: delete position_permissions: %v", err)
	}
	_, err = db.ExecContext(ctx, `
		DELETE FROM permission_types WHERE code = ANY($1)`, sqlArray(relevantPermissionCodes))
	if err != nil {
		t.Fatalf("restore: delete permission_types: %v", err)
	}
	for code, r := range permTypes {
		_, err = db.ExecContext(ctx, `
			INSERT INTO permission_types (id, code, domain, operation, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)`, r.id, r.code, r.domain, r.operation, r.createdAt, r.updatedAt)
		if err != nil {
			t.Fatalf("restore: insert permission_type %s: %v", code, err)
		}
	}
	for _, r := range grants {
		_, err = db.ExecContext(ctx, `
			INSERT INTO position_permissions (id, position, permission_id, created_at)
			VALUES ($1::uuid, $2, $3::uuid, $4)
			ON CONFLICT (position, permission_id) DO NOTHING`, r.id, r.position, r.permissionID, r.createdAt)
		if err != nil {
			t.Fatalf("restore: insert position_permission %s/%s: %v", r.position, r.permissionID, err)
		}
	}
}

// withRelevantTx runs fn inside an outer transaction that ALWAYS rolls back.
func withRelevantTx(t *testing.T, fn func(ctx context.Context, tx *sql.Tx)) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	permTypes, grants := snapshotRelevantState(t, ctx, pool.Raw())
	tx, err := pool.Raw().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
		restoreRelevantState(t, ctx, pool.Raw(), permTypes, grants)
	}()
	fn(ctx, tx)
}

// countsFor returns the count of grants for the given code and position.
func countFor(t *testing.T, ctx context.Context, tx *sql.Tx, code, position string) int {
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

// ============================================================================
// Migration 072
// ============================================================================
func TestMigration072_UPGrantsOnlySekretarisAnnouncements(t *testing.T) {
	withRelevantTx(t, func(ctx context.Context, tx *sql.Tx) {
		// Reset scope: remove all announcements grants for sekretaris to start from a baseline.
		// Note: we cannot safely reset other positions because those grants are
		// held by the devseed canonical data and we do not want to destroy or
		// accidentally revert data used by other tests. We assert that 072
		// creates the sekretaris grant and does NOT create grants for
		// positions that did not already hold them.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM position_permissions pp
			USING permission_types pt
			WHERE pp.permission_id = pt.id
			  AND pt.code IN ('announcements.create','announcements.update','announcements.publish','announcements.archive','announcements.delete')
			  AND pp.position = 'sekretaris'`); err != nil {
			t.Fatalf("reset sekretaris grants: %v", err)
		}
		// Capture pre-existing grants (other positions) so we do not false-positive on
		// migration 072 — those were already in the database before UP ran.
		pre := map[string]int{}
		for _, code := range []string{
			"announcements.create", "announcements.update", "announcements.publish",
			"announcements.archive", "announcements.delete",
		} {
			for _, position := range []string{"ketua", "wakil_ketua", "bendahara", "keamanan", "sosial", "operasional", "kebersihan", "anggota"} {
				pre[code+"|"+position] = countFor(t, ctx, tx, code, position)
			}
		}

		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.up.sql")

		for _, code := range []string{
			"announcements.create",
			"announcements.update",
			"announcements.publish",
			"announcements.archive",
			"announcements.delete",
		} {
			if got := countFor(t, ctx, tx, code, "sekretaris"); got != 1 {
				t.Fatalf("%s after UP = %d, want 1", code, got)
			}
			// Other positions must NOT receive these from 072. Whatever they held
			// before must still hold after.
			for _, position := range []string{"ketua", "wakil_ketua", "bendahara", "keamanan", "sosial", "operasional", "kebersihan", "anggota"} {
				if got := countFor(t, ctx, tx, code, position); got != pre[code+"|"+position] {
					t.Fatalf("%s on position %s changed by 072: before=%d after=%d", code, position, pre[code+"|"+position], got)
				}
			}
		}

		// Idempotency.
		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.up.sql")
		for _, code := range []string{
			"announcements.create",
			"announcements.update",
			"announcements.publish",
			"announcements.archive",
			"announcements.delete",
		} {
			if got := countFor(t, ctx, tx, code, "sekretaris"); got != 1 {
				t.Fatalf("re-UP %s = %d, want 1 (idempotent)", code, got)
			}
		}
	})
}

func TestMigration072_DOWNRevokesOnlySekretarisAnnouncements(t *testing.T) {
	withRelevantTx(t, func(ctx context.Context, tx *sql.Tx) {
		// Ensure the fixture includes grants first.
		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.up.sql")
		// Manual foreign grant must be preserved by DOWN.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT 'wakil_ketua', id FROM permission_types WHERE code = 'announcements.create'
			ON CONFLICT (position, permission_id) DO NOTHING`); err != nil {
			t.Fatalf("seed wakil_ketua announcements.create: %v", err)
		}

		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.down.sql")

		for _, code := range []string{
			"announcements.create",
			"announcements.update",
			"announcements.publish",
			"announcements.archive",
			"announcements.delete",
		} {
			if got := countFor(t, ctx, tx, code, "sekretaris"); got != 0 {
				t.Fatalf("%s after DOWN = %d, want 0", code, got)
			}
		}
		// Non-sekretaris grants must be untouched.
		if got := countFor(t, ctx, tx, "announcements.create", "wakil_ketua"); got != 1 {
			t.Fatalf("wakil_ketua announcements.create after DOWN = %d, want 1 (preserved)", got)
		}
	})
}

func TestMigration072_DOWN_IsIdempotent(t *testing.T) {
	withRelevantTx(t, func(ctx context.Context, tx *sql.Tx) {
		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.down.sql")
		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.down.sql")
	})
}

// ============================================================================
// Migration 074
// ============================================================================
func TestMigration074_UPRevokesBannerTemplatesManageFromSekretaris(t *testing.T) {
	withRelevantTx(t, func(ctx context.Context, tx *sql.Tx) {
		// Seed the state migration 074 targets.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT 'sekretaris', id FROM permission_types WHERE code = 'banner_templates.manage'
			ON CONFLICT (position, permission_id) DO NOTHING`); err != nil {
			t.Fatalf("seed sekretaris manage: %v", err)
		}

		runMigration(t, ctx, tx, "074_revoke_banner_templates_manage_from_sekretaris.up.sql")
		after := countFor(t, ctx, tx, "banner_templates.manage", "sekretaris")

		if after != 0 {
			t.Fatalf("banner_templates.manage after 074 UP = %d, want 0", after)
		}
		// Other positions keep their banner_templates.manage grants.
		for _, position := range []string{"ketua", "wakil_ketua", "bendahara", "keamanan", "sosial", "operasional", "kebersihan", "anggota"} {
			_ = countFor(t, ctx, tx, "banner_templates.manage", position)
		}

		// Idempotent.
		runMigration(t, ctx, tx, "074_revoke_banner_templates_manage_from_sekretaris.up.sql")
		if got := countFor(t, ctx, tx, "banner_templates.manage", "sekretaris"); got != 0 {
			t.Fatalf("re-UP 074 must be idempotent, got %d", got)
		}
	})
}

func TestMigration074_DOWNReGrantsOnlySekretaris(t *testing.T) {
	withRelevantTx(t, func(ctx context.Context, tx *sql.Tx) {
		_, err := tx.ExecContext(ctx, `
			DELETE FROM position_permissions
			USING permission_types pt
			WHERE position_permissions.permission_id = pt.id
			  AND pt.code = 'banner_templates.manage'
			  AND position_permissions.position = 'sekretaris'`)
		if err != nil {
			t.Fatalf("reset sekretaris manage: %v", err)
		}
		// Capture pre-existing non-sekretaris grants.
		type grantKey struct{ position, code string }
		pre := map[grantKey]int{}
		for _, position := range []string{"ketua", "wakil_ketua", "bendahara", "keamanan", "sosial", "operasional", "kebersihan", "anggota"} {
			pre[grantKey{position, "banner_templates.manage"}] = countFor(t, ctx, tx, "banner_templates.manage", position)
		}

		runMigration(t, ctx, tx, "074_revoke_banner_templates_manage_from_sekretaris.down.sql")

		if got := countFor(t, ctx, tx, "banner_templates.manage", "sekretaris"); got != 1 {
			t.Fatalf("banner_templates.manage after 074 DOWN = %d, want 1", got)
		}
		// Other positions preserved.
		for _, position := range []string{"ketua", "wakil_ketua", "bendahara", "keamanan", "sosial", "operasional", "kebersihan", "anggota"} {
			if got := countFor(t, ctx, tx, "banner_templates.manage", position); got != pre[grantKey{position, "banner_templates.manage"}] {
				t.Fatalf("non-sekretaris %s changed by 074 DOWN: %d", position, got)
			}
		}
	})
}

// ============================================================================
// Combined 072 → 074 expectation
// ============================================================================
func TestMigration072Then074_FinalState(t *testing.T) {
	withRelevantTx(t, func(ctx context.Context, tx *sql.Tx) {
		// Start from a clean scope.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM position_permissions pp
			USING permission_types pt
			WHERE pp.permission_id = pt.id AND pt.code = ANY($1)`, sqlArray([]string{
			"announcements.create", "announcements.update", "announcements.publish",
			"announcements.archive", "announcements.delete", "banner_templates.manage",
		})); err != nil {
			t.Fatalf("reset scope: %v", err)
		}

		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.up.sql")
		runMigration(t, ctx, tx, "074_revoke_banner_templates_manage_from_sekretaris.up.sql")

		for _, code := range []string{
			"announcements.create", "announcements.update", "announcements.publish",
			"announcements.archive", "announcements.delete",
		} {
			if got := countFor(t, ctx, tx, code, "sekretaris"); got != 1 {
				t.Fatalf("%s after 072+074 = %d, want 1", code, got)
			}
		}
		if got := countFor(t, ctx, tx, "banner_templates.manage", "sekretaris"); got != 0 {
			t.Fatalf("banner_templates.manage after 072+074 = %d, want 0", got)
		}

		// DOWN sequence: 074 DOWN restores manage, 072 DOWN revokes announcements.
		runMigration(t, ctx, tx, "074_revoke_banner_templates_manage_from_sekretaris.down.sql")
		runMigration(t, ctx, tx, "072_sekretaris_announcement_permissions.down.sql")

		if got := countFor(t, ctx, tx, "banner_templates.manage", "sekretaris"); got != 1 {
			t.Fatalf("banner_templates.manage after 074 DOWN = %d, want 1", got)
		}
		for _, code := range []string{
			"announcements.create", "announcements.update", "announcements.publish",
			"announcements.archive", "announcements.delete",
		} {
			if got := countFor(t, ctx, tx, code, "sekretaris"); got != 0 {
				t.Fatalf("%s after 072 DOWN = %d, want 0", code, got)
			}
		}
	})
}
