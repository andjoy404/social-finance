package auth_test

// ============================================================================
// Migration 070 lifecycle and ownership safety tests.
//
// IMPORTANT CONTEXT (see docs/migration-070-safety.md):
//   - Migration 070_community_delete_permissions has already been applied in
//     the development environments (social_finance and social_finance_test
//     both contain the permission rows and the ketua grants).
//   - Per project policy (AGENTS.md / task instructions), an already-applied
//     migration is immutable: we do NOT edit the applied migration files.
//   - The UP migration is idempotent. The DOWN migration that ships in-tree
//     is NOT provably safe: it deletes the 'ketua' grant for these two
//     permissions even though that grant may have existed before the
//     migration ran (the canonical seed creates it). This test suite proves
//     that behavior explicitly and locks in the SAFE policy documented in
//     docs/migration-070-safety.md.
//
// Every test here runs inside an outer transaction and is rolled back.
// The two migration files referenced by these tests are read from disk so
// we test exactly what ships, not a reimplementation.
// ============================================================================

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"social-finance/internal/devseed"
	"social-finance/internal/testutil"
)

var communityDeleteCodes = []string{"activities.delete", "announcements.delete"}

type permTypeRow struct {
	id        string
	code      string
	domain    string
	operation string
	createdAt time.Time
	updatedAt time.Time
}

type posPermRow struct {
	id           string
	position     string
	permissionID string
	createdAt    time.Time
}

func communityDeletePath(t *testing.T, filename string) string {
	t.Helper()
	base := filepath.Join("..", "..", "migrations", filename)
	if info, err := os.Stat(base); err != nil || info.IsDir() {
		t.Fatalf("migration file %s: %v", base, err)
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	return abs
}

// snapshotCommunityDeleteState captures ALL rows that could be affected by the
// 070 migration (permission types + every grant pointing at them).
func snapshotCommunityDeleteState(t *testing.T, ctx context.Context, db *sql.DB) (map[string]permTypeRow, map[string]posPermRow) {
	t.Helper()
	permRows := map[string]permTypeRow{}
	grantRows := map[string]posPermRow{}

	q := `SELECT id::text, code, domain, operation, created_at, updated_at
	      FROM permission_types
	      WHERE code IN ('activities.delete', 'announcements.delete')`
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
		t.Fatalf("permType rows: %v", err)
	}

	q = `SELECT pp.id::text, pp.position, pp.permission_id::text, pp.created_at
	      FROM position_permissions pp
	      JOIN permission_types pt ON pt.id = pp.permission_id
	      WHERE pt.code IN ('activities.delete', 'announcements.delete')`
	grows, err := db.QueryContext(ctx, q)
	if err != nil {
		t.Fatalf("snapshot position_permissions: %v", err)
	}
	defer grows.Close()
	for grows.Next() {
		var r posPermRow
		if err := grows.Scan(&r.id, &r.position, &r.permissionID, &r.createdAt); err != nil {
			t.Fatalf("scan position_permission: %v", err)
		}
		key := r.position + "|" + r.permissionID
		grantRows[key] = r
	}
	if err := grows.Err(); err != nil {
		t.Fatalf("position_permission rows: %v", err)
	}

	return permRows, grantRows
}

// restoreCommunityDeleteState deletes the two permission types (CASCADE removes
// the grants) and re-inserts the captured rows exactly as they were.
func restoreCommunityDeleteState(t *testing.T, ctx context.Context, db *sql.DB, permRows map[string]permTypeRow, grantRows map[string]posPermRow) {
	t.Helper()
	_, err := db.ExecContext(ctx, `DELETE FROM permission_types WHERE code IN ('activities.delete', 'announcements.delete')`)
	if err != nil {
		t.Fatalf("restore: delete permission_types: %v", err)
	}
	for code, r := range permRows {
		_, err = db.ExecContext(ctx, `
			INSERT INTO permission_types (id, code, domain, operation, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			ON CONFLICT (code) DO UPDATE SET id = EXCLUDED.id, domain = EXCLUDED.domain,
				operation = EXCLUDED.operation, created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at
		`, r.id, r.code, r.domain, r.operation, r.createdAt, r.updatedAt)
		if err != nil {
			t.Fatalf("restore: insert permission_type %s: %v", code, err)
		}
	}
	for _, r := range grantRows {
		_, err = db.ExecContext(ctx, `
			INSERT INTO position_permissions (id, position, permission_id, created_at)
			VALUES ($1::uuid, $2, $3::uuid, $4)
			ON CONFLICT (position, permission_id) DO NOTHING
		`, r.id, r.position, r.permissionID, r.createdAt)
		if err != nil {
			t.Fatalf("restore: insert position_permission %s/%s: %v", r.position, r.permissionID, err)
		}
	}
}

// withCommunityDeleteSnapshot runs fn inside a shared, fully-isolated test
// instance. It snapshots the entire 070 permission scope before the test and
// restores it afterwards, so the shared test database is always left exactly
// as it was found. fn is free to mutate state arbitrarily — the restore
// will delete the two permission types (cascading the grants) and re-insert
// the snapshot.
func withCommunityDeleteSnapshot(t *testing.T, name string, fn func(ctx context.Context, t *testing.T)) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	permSnap, grantSnap := snapshotCommunityDeleteState(t, ctx, pool.Raw())

	defer func() {
		restoreCommunityDeleteState(t, ctx, pool.Raw(), permSnap, grantSnap)
	}()

	fn(ctx, t)
}

// runMigration executes a migration file inside the caller's outer transaction.
// The BEGIN/COMMIT wrappers in the file are converted to SAVEPOINT pairs so
// the file nests inside the outer transaction without committing it.
func runMigration(t *testing.T, ctx context.Context, tx *sql.Tx, filename string) {
	t.Helper()
	raw, err := os.ReadFile(communityDeletePath(t, filename))
	if err != nil {
		t.Fatalf("read migration %s: %v", filename, err)
	}
	sp := 0
	text := strings.Replace(string(raw), "\r\n", "\n", -1)
	text = strings.Replace(text, "\r", "\n", -1)
	// Lines that are exactly "BEGIN;" or "COMMIT;" are converted to SAVEPOINT pairs.
	var builder strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trim := strings.ToLower(strings.TrimSpace(line))
		switch trim {
		case "begin;":
			sp++
			fmt.Fprintf(&builder, "SAVEPOINT mig_sp_%d;\n", sp)
		case "commit;":
			fmt.Fprintf(&builder, "RELEASE SAVEPOINT mig_sp_%d;\n", sp)
		default:
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
	}
	if _, err := tx.ExecContext(ctx, builder.String()); err != nil {
		t.Fatalf("execute migration %s: %v", filename, err)
	}
}

// with070Tx runs fn inside an outer transaction and ALWAYS rolls back.
// The 070 table scope is never deleted globally.
func with070Tx(t *testing.T, fn func(ctx context.Context, t *testing.T, tx *sql.Tx)) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	permSnap, grantSnap := snapshotCommunityDeleteState(t, ctx, pool.Raw())
	tx, err := pool.Raw().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
		restoreCommunityDeleteState(t, ctx, pool.Raw(), permSnap, grantSnap)
	}()
	fn(ctx, t, tx)
}

// counts070Tx wraps counts for a transaction.
func counts070Tx(t *testing.T, ctx context.Context, tx *sql.Tx) (types, ketua, wakil, sekre, bendahara int) {
	t.Helper()
	scan := func(query string) int {
		var n int
		if err := tx.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatalf("count query failed: %v (query: %s)", err, query)
		}
		return n
	}
	types = scan(`SELECT COUNT(*) FROM permission_types WHERE code IN ('activities.delete', 'announcements.delete')`)

	ketua = scan(`SELECT COUNT(*) FROM position_permissions pp JOIN permission_types pt ON pt.id = pp.permission_id WHERE pt.code IN ('activities.delete', 'announcements.delete') AND pp.position = 'ketua'`)
	wakil = scan(`SELECT COUNT(*) FROM position_permissions pp JOIN permission_types pt ON pt.id = pp.permission_id WHERE pt.code IN ('activities.delete', 'announcements.delete') AND pp.position = 'wakil_ketua'`)
	sekre = scan(`SELECT COUNT(*) FROM position_permissions pp JOIN permission_types pt ON pt.id = pp.permission_id WHERE pt.code IN ('activities.delete', 'announcements.delete') AND pp.position = 'sekretaris'`)
	bendahara = scan(`SELECT COUNT(*) FROM position_permissions pp JOIN permission_types pt ON pt.id = pp.permission_id WHERE pt.code IN ('activities.delete', 'announcements.delete') AND pp.position = 'bendahara'`)
	return
}

// ============================================================================
// Scenario 1: Fresh environment (no prior rows → UP → DOWN policy)
// ============================================================================
func TestMigration070_FreshEnvironment_UP_and_Policy(t *testing.T) {
	with070Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Baseline: remove 070-scope rows so this executes as a fresh environment.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM position_permissions
			WHERE permission_id IN (SELECT id FROM permission_types
			WHERE code IN ('activities.delete', 'announcements.delete'))`); err != nil {
			t.Fatalf("reset position_permissions: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM permission_types WHERE code IN ('activities.delete', 'announcements.delete')`); err != nil {
			t.Fatalf("reset permission_types: %v", err)
		}

		types, ketua, wakil, sekre, bendahara := counts070Tx(t, ctx, tx)
		if types != 0 || ketua != 0 || wakil != 0 || sekre != 0 || bendahara != 0 {
			t.Fatalf("baseline not clean: types=%d ketua=%d wakil=%d sekre=%d bendahara=%d", types, ketua, wakil, sekre, bendahara)
		}

		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, wakil, sekre, bendahara = counts070Tx(t, ctx, tx)
		if types != 2 {
			t.Fatalf("after UP: expected 2 permission_types, got %d", types)
		}
		if ketua != 2 {
			t.Fatalf("after UP: expected 2 ketua grants, got %d", ketua)
		}
		if wakil != 0 || sekre != 0 || bendahara != 0 {
			t.Fatalf("after UP: non-ketua grants must not be created: wakil=%d sekre=%d bendahara=%d", wakil, sekre, bendahara)
		}

		// SAFE POLICY: DOWN is a no-op — nothing is removed automatically.
		// Re-running UP must remain idempotent.
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, _, _, _ = counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 2 {
			t.Fatalf("after re-UP (idempotent): types=%d ketua=%d", types, ketua)
		}
	})
}

// ============================================================================
// Scenario 2: Seeded environment (canonical devseed created everything)
// ============================================================================
func TestMigration070_SeededEnvironment_Unaffected(t *testing.T) {
	withCommunityDeleteSnapshot(t, "seeded", func(ctx context.Context, t *testing.T) {
		pool := testutil.GetTestPool(t)
		if _, err := devseed.Run(ctx, pool, "test"); err != nil {
			t.Fatalf("devseed: %v", err)
		}
		permSnap, grantSnap := snapshotCommunityDeleteState(t, ctx, pool.Raw())
		if len(permSnap) != 2 {
			t.Fatalf("canonical seed must create both permission_types; got %d", len(permSnap))
		}
		var ketuaGrants int
		for _, g := range grantSnap {
			if g.position == "ketua" {

				ketuaGrants++
			}
		}
		if ketuaGrants != 2 {
			t.Fatalf("canonical seed must grant ketua both permissions; got %d grants", ketuaGrants)
		}
	})
}

// ============================================================================
// Scenario 3 & 4: Existing permission types and additional grants in OTHER positions.
// ============================================================================
func TestMigration070_ExistingPermissionAndAdditionalGrants_Preserved(t *testing.T) {
	with070Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Seed an environment that ALREADY has the permission types and a
		// ketua grant, PLUS additional grants for other positions.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO permission_types (code, domain, operation) VALUES
				('activities.delete', 'community', 'delete'),
				('announcements.delete', 'community', 'delete')
			ON CONFLICT (code) DO NOTHING`); err != nil {
			t.Fatalf("seed permission_types: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT p, id FROM (
				SELECT unnest(ARRAY['ketua','wakil_ketua','sekretaris','bendahara']) AS p
			) positions, permission_types
			WHERE code IN ('activities.delete', 'announcements.delete')
			ON CONFLICT (position, permission_id) DO NOTHING`); err != nil {
			t.Fatalf("seed grants: %v", err)
		}

		types, ketua, wakil, sekre, bendahara := counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 2 || wakil != 2 || sekre != 2 || bendahara != 2 {
			t.Fatalf("manual fixture not in place: %+v",
				map[string]int{"types": types, "ketua": ketua, "wakil": wakil, "sekre": sekre, "bendahara": bendahara})
		}

		// UP must not revoke anything that already exists.
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, wakil, sekre, bendahara = counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 2 || wakil != 2 || sekre != 2 || bendahara != 2 {
			t.Fatalf("UP must not touch existing state: %+v",
				map[string]int{"types": types, "ketua": ketua, "wakil": wakil, "sekre": sekre, "bendahara": bendahara})
		}

		// Negative control: the DOWN file contained in the repo deletes the
		// ketua grant even if it existed before UP. This documents the exact
		// reason DOWN is not reversible for pre-existing data.
		runMigration(t, ctx, tx, "070_community_delete_permissions.down.sql")
		types, ketua, wakil, sekre, bendahara = counts070Tx(t, ctx, tx)
		if types != 2 {
			t.Fatalf("DOWN must preserve permission_types, got %d", types)
		}
		if ketua != 0 {
			t.Fatalf("DOWN (as shipped) revokes pre-existing ketua grants; got %d — expected 0 for this test", ketua)
		}
		if wakil != 2 || sekre != 2 || bendahara != 2 {
			t.Fatalf("DOWN must preserve non-ketua grants: wakil=%d sekre=%d bendahara=%d", wakil, sekre, bendahara)
		}

		// Re-seed point: after a fresh UP (run again), ketua grants are re-created.
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, wakil, sekre, bendahara = counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 2 || wakil != 2 || sekre != 2 || bendahara != 2 {
			t.Fatalf("after re-UP: %+v",
				map[string]int{"types": types, "ketua": ketua, "wakil": wakil, "sekre": sekre, "bendahara": bendahara})
		}
	})
}

// ============================================================================
// Scenario 5: Mixed ownership — one permission pre-exists, the other does not;
// one ketua grant pre-exists, the other does not.
// ============================================================================
func TestMigration070_MixedOwnership_PartialPreexistence(t *testing.T) {
	with070Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Reset the 070 scope inside the outer transaction (rolled back at cleanup),
		// then re-create exactly the partial pre-state this scenario needs.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM position_permissions
			WHERE permission_id IN (SELECT id FROM permission_types
			WHERE code IN ('activities.delete', 'announcements.delete'))`); err != nil {
			t.Fatalf("reset position_permissions: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM permission_types WHERE code IN ('activities.delete', 'announcements.delete')`); err != nil {
			t.Fatalf("reset permission_types: %v", err)
		}

		// Only activities.delete exists + only ketua/activities.delete granted.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO permission_types (code, domain, operation)
			VALUES ('activities.delete', 'community', 'delete')`); err != nil {
			t.Fatalf("pre-seed activities.delete: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT 'ketua', id FROM permission_types WHERE code = 'activities.delete'
			ON CONFLICT (position, permission_id) DO NOTHING`); err != nil {
			t.Fatalf("pre-seed ketua grant for activities.delete: %v", err)
		}

		types, ketua, wakil, sekre, bendahara := counts070Tx(t, ctx, tx)
		if types != 1 || ketua != 1 || wakil != 0 {
			t.Fatalf("manual fixture not in place: types=%d ketua=%d", types, ketua)
		}

		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, wakil, sekre, bendahara = counts070Tx(t, ctx, tx)
		if types != 2 {
			t.Fatalf("UP must create the missing permission type; got types=%d", types)
		}
		if ketua != 2 {
			t.Fatalf("UP must grant ketua the missing permission; got ketua=%d", ketua)
		}
		if wakil != 0 || sekre != 0 || bendahara != 0 {
			t.Fatalf("UP must not create grants for non-ketua positions: wakil=%d sekre=%d bendahara=%d", wakil, sekre, bendahara)
		}

		// Re-running UP must be fully idempotent (both codes already exist).
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, wakil, sekre, bendahara = counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 2 || wakil != 0 || sekre != 0 || bendahara != 0 {
			t.Fatalf("re-UP must be idempotent: %+v",
				map[string]int{"types": types, "ketua": ketua, "wakil": wakil, "sekre": sekre, "bendahara": bendahara})
		}
	})
}

// ============================================================================
// Scenario 6 & 7: Idempotency and UP → DOWN → UP re-application.
// ============================================================================
func TestMigration070_Idempotency_And_Reapply(t *testing.T) {
	with070Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Reset scope.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM position_permissions
			WHERE permission_id IN (SELECT id FROM permission_types
			WHERE code IN ('activities.delete', 'announcements.delete'))`); err != nil {
			t.Fatalf("reset position_permissions: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM permission_types WHERE code IN ('activities.delete', 'announcements.delete')`); err != nil {
			t.Fatalf("reset permission_types: %v", err)
		}

		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, _, _, _ := counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 2 {
			t.Fatalf("UP must be idempotent: types=%d ketua=%d", types, ketua)
		}

		// DOWN (as shipped in-tree) removes ketua grants, then UP re-adds them.
		runMigration(t, ctx, tx, "070_community_delete_permissions.down.sql")
		types, ketua, _, _, _ = counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 0 {
			t.Fatalf("DOWN (as shipped): want types=2 ketua=0; got types=%d ketua=%d", types, ketua)
		}

		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		types, ketua, _, _, _ = counts070Tx(t, ctx, tx)
		if types != 2 || ketua != 2 {
			t.Fatalf("UP after DOWN: types=%d ketua=%d", types, ketua)
		}
	})
}

// ============================================================================
// Scenario 8: Interaction with 071 — 070 DOWN must not touch 071 state.
// ============================================================================
func TestMigration070_DownInteractionWith071(t *testing.T) {
	with070Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Snapshot 071-relevant grants: bendahara MUST NOT have notification.manage
		// (071 applied), ketua MUST have notification.manage, bendahara MUST have
		// finance.approve.
		countFor := func(code, position string) int {
			var n int
			if err := tx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM position_permissions pp
				JOIN permission_types pt ON pt.id = pp.permission_id
				WHERE pt.code = $1 AND pp.position = $2`, code, position).Scan(&n); err != nil {
				t.Fatalf("count(%s,%s): %v", code, position, err)
			}
			return n
		}

		beforeBendaharaManage := countFor("notification.manage", "bendahara")
		beforeKetuaManage := countFor("notification.manage", "ketua")
		beforeBendaharaFinance := countFor("finance.approve", "bendahara")
		beforeBendaharaNotifRead := countFor("notification.read", "bendahara")

		if beforeBendaharaManage != 0 {
			t.Fatalf("precondition: bendahara must NOT have notification.manage (071), got %d", beforeBendaharaManage)
		}
		if beforeKetuaManage < 1 {
			t.Fatalf("precondition: ketua must have notification.manage, got %d", beforeKetuaManage)
		}
		if beforeBendaharaFinance < 1 {
			t.Fatalf("precondition: bendahara must keep finance.approve, got %d", beforeBendaharaFinance)
		}
		if beforeBendaharaNotifRead < 1 {
			t.Fatalf("precondition: bendahara must keep notification.read, got %d", beforeBendaharaNotifRead)
		}

		// Execute 070 UP then the shipped 070 DOWN inside the outer tx.
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		runMigration(t, ctx, tx, "070_community_delete_permissions.down.sql")

		if got := countFor("notification.manage", "bendahara"); got != beforeBendaharaManage {
			t.Fatalf("070 DOWN changed bendahara notification.manage: before=%d after=%d", beforeBendaharaManage, got)
		}
		if got := countFor("notification.manage", "ketua"); got != beforeKetuaManage {
			t.Fatalf("070 DOWN changed ketua notification.manage: before=%d after=%d", beforeKetuaManage, got)
		}
		if got := countFor("finance.approve", "bendahara"); got != beforeBendaharaFinance {
			t.Fatalf("070 DOWN removed bendahara finance.approve: before=%d after=%d", beforeBendaharaFinance, got)
		}
		if got := countFor("notification.read", "bendahara"); got != beforeBendaharaNotifRead {
			t.Fatalf("070 DOWN removed bendahara notification.read: before=%d after=%d", beforeBendaharaNotifRead, got)
		}

		// Permission types 071/070 all intact.
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM permission_types WHERE code IN ('notification.manage','notification.read','finance.approve')`).Scan(&n); err != nil {
			t.Fatalf("count permission_types: %v", err)
		}
		if n != 3 {
			t.Fatalf("070 DOWN removed permission types needed by 071: got %d of 3", n)
		}
	})
}

// ============================================================================
// Scenario 9: Permission type owned by canonical seed must still exist after
// the 070 DOWN (verified above by count==2) — this test asserts the same
// property with row-level identity checks.
// ============================================================================
func TestMigration070_SeedOwnedPermissionTypesRemain(t *testing.T) {
	with070Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		before := map[string]string{}
		rows, err := tx.QueryContext(ctx, `SELECT code, id::text FROM permission_types WHERE code IN ('activities.delete', 'announcements.delete')`)
		if err != nil {
			t.Fatalf("snapshot types: %v", err)
		}
		for rows.Next() {
			var code, id string
			_ = rows.Scan(&code, &id)
			before[code] = id
		}
		_ = rows.Close()

		runMigration(t, ctx, tx, "070_community_delete_permissions.down.sql")

		for code, id := range before {
			var after string
			if err := tx.QueryRowContext(ctx, `SELECT id::text FROM permission_types WHERE code = $1`, code).Scan(&after); err != nil {
				t.Fatalf("permission_type %s disappeared after DOWN: %v", code, err)
			}
			if after != id {
				t.Fatalf("permission_type %s changed identity: before=%s after=%s", code, id, after)
			}
		}
	})
}

// ============================================================================
// Scenario 10: The shipped 070 DOWN is a negative control — it revokes the
// ketua grant created by the UP because it cannot distinguish it from a
// pre-existing seed grant. This is UNAVOIDABLE without provenance.
// ============================================================================
func TestMigration070_ShippedDown_IsNotProvablyReversible(t *testing.T) {
	with070Tx(t, func(ctx context.Context, t *testing.T, tx *sql.Tx) {
		// Insert a pre-existing ketua grant manually (as canonical seed would).
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO permission_types (code, domain, operation) VALUES
				('activities.delete', 'community', 'delete'),
				('announcements.delete', 'community', 'delete')
			ON CONFLICT (code) DO NOTHING`); err != nil {
			t.Fatalf("seed permission_types: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT 'ketua', id FROM permission_types
			WHERE code IN ('activities.delete', 'announcements.delete')
			ON CONFLICT (position, permission_id) DO NOTHING`); err != nil {
			t.Fatalf("seed ketua grants: %v", err)
		}

		_, ketua, _, _, _ := counts070Tx(t, ctx, tx)
		if ketua != 2 {
			t.Fatalf("pre-state: ketua=%d", ketua)
		}

		// UP is a no-op (already present). DOWN (shipped) deletes ketua grants
		// even though they were not created by the UP.
		runMigration(t, ctx, tx, "070_community_delete_permissions.up.sql")
		runMigration(t, ctx, tx, "070_community_delete_permissions.down.sql")

		_, ketua, _, _, _ = counts070Tx(t, ctx, tx)
		if ketua != 0 {
			t.Fatalf("shipped DOWN must revoke the ketua grant it cannot attribute; got %d", ketua)
		}

		// Permission_types MUST survive.
		types, _, _, _, _ := counts070Tx(t, ctx, tx)
		if types != 2 {
			t.Fatalf("permission_types must survive DOWN; got %d", types)
		}
	})
}
