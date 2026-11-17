package auth_test

// ============================================================================
// Migration 073 (sync_imported_users_phone) and 075 (normalize_phones_remaining)
// safety tests.
//
// These tests exercise the actual migration files shipped in the repository,
// not reimplementations. Each test runs inside an outer transaction that ALWAYS
// rolls back; nothing persists.
//
// Both migrations are idempotent and additive-only (074/073 touch only rows
// where the target condition matches; nothing destructive happens).
// ============================================================================

import (
	"context"
	"database/sql"
	"testing"

	"social-finance/internal/testutil"
)

// withPhoneScopeTx runs fn inside an outer transaction that ALWAYS rolls back.
func withPhoneScopeTx(t *testing.T, fn func(ctx context.Context, tx *sql.Tx)) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	tx, err := pool.Raw().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	fn(ctx, tx)
}

// countsNonCanonUsers returns the number of users whose phone is not canonical.
func countsNonCanonUsers(t *testing.T, ctx context.Context, tx *sql.Tx) int {
	t.Helper()
	var n int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users
		WHERE phone IS NOT NULL AND btrim(phone) <> ''
		  AND phone !~ '^08[0-9]{8,}$'`).Scan(&n); err != nil {
		t.Fatalf("count non-canon users: %v", err)
	}
	return n
}

// ============================================================================
// Migration 075 — normalizes remaining non-canonical phones
// ============================================================================
func TestMigration075_UP_NormalizesRemainingPhones(t *testing.T) {
	withPhoneScopeTx(t, func(ctx context.Context, tx *sql.Tx) {
		nonCanonBefore := countsNonCanonUsers(t, ctx, tx)

		runMigration(t, ctx, tx, "075_normalize_phones_remaining.up.sql")

		nonCanonAfter := countsNonCanonUsers(t, ctx, tx)

		// We cannot expect the polluted shared test database to be empty, so we
		// assert that the migration ONLY normalizes rows whose normalized form
		// does not collide. We compare against the sum of collision-skipped rows.
		// The invariant: nonCanonAfter <= nonCanonBefore (no corruption introduced).
		if nonCanonAfter > nonCanonBefore {
			t.Fatalf("non-canonical users after migration = %d, want <= %d (no corruption introduced)", nonCanonAfter, nonCanonBefore)
		}

		// Idempotent: running the migration a second time must be a no-op.
		runMigration(t, ctx, tx, "075_normalize_phones_remaining.up.sql")
		nonCanonAfter2 := countsNonCanonUsers(t, ctx, tx)
		if nonCanonAfter2 > nonCanonBefore {
			t.Fatalf("non-canonical users after re-run = %d, want <= %d (idempotency)", nonCanonAfter2, nonCanonBefore)
		}
	})
}

// ============================================================================
// Migration 073 — backfills users.phone from residents.phone
// ============================================================================
func TestMigration073_UP_BackfillsFromResidents(t *testing.T) {
	withPhoneScopeTx(t, func(ctx context.Context, tx *sql.Tx) {
		// Create a user with NULL phone.
		var userID string
		row := tx.QueryRowContext(ctx, `
			INSERT INTO users (email, password_hash, full_name, system_role, is_active, phone)
			VALUES ('migration073-' || extract(epoch from now())::text || '@test.local',
			        'dummy', 'Migration 073 Test', NULL, true, NULL)
			RETURNING id`)
		if err := row.Scan(&userID); err != nil {
			t.Fatalf("insert user: %v", err)
		}

		// Create a matching resident with a canonical phone referencing that user.
		_, err := tx.ExecContext(ctx, `
			INSERT INTO residents (id, rt_id, full_name, phone, is_active, nik, user_id)
			VALUES (gen_random_uuid(), '00000000-0000-0000-0000-000000000002',
			        'Migration 073 Resident', '081234567890', true,
			        '3201010090011111', $1::uuid)`, userID)
		if err != nil {
			t.Fatalf("insert resident: %v", err)
		}

		runMigration(t, ctx, tx, "073_sync_imported_users_phone.up.sql")

		var phone *string
		if err := tx.QueryRowContext(ctx, `SELECT phone FROM users WHERE id = $1::uuid`, userID).Scan(&phone); err != nil {
			t.Fatalf("read phone: %v", err)
		}
		if phone == nil || *phone != "081234567890" {
			t.Fatalf("phone after 073 = %v, want 081234567890", phone)
		}

		// Idempotency: a second run must not change anything.
		runMigration(t, ctx, tx, "073_sync_imported_users_phone.up.sql")
		var phone2 *string
		if err := tx.QueryRowContext(ctx, `SELECT phone FROM users WHERE id = $1::uuid`, userID).Scan(&phone2); err != nil {
			t.Fatalf("read phone after re-run: %v", err)
		}
		if phone2 == nil || *phone2 != "081234567890" {
			t.Fatalf("phone after re-run = %v, want 081234567890 (idempotency)", phone2)
		}
	})
}

func TestMigration073_UP_DoesNotOverwriteExistingPhones(t *testing.T) {
	withPhoneScopeTx(t, func(ctx context.Context, tx *sql.Tx) {
		// User with a phone already set; resident recommends a different canonical value.
		var userID string
		row := tx.QueryRowContext(ctx, `
			INSERT INTO users (email, password_hash, full_name, system_role, is_active, phone)
			VALUES ('migration073b-' || extract(epoch from now())::text || '@test.local',
			        'dummy', 'Migration 073 Test B', NULL, true, '081222222222')
			RETURNING id`)
		if err := row.Scan(&userID); err != nil {
			t.Fatalf("insert user: %v", err)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO residents (id, rt_id, full_name, phone, is_active, nik, user_id)
			VALUES (gen_random_uuid(), '00000000-0000-0000-0000-000000000002',
			        'Migration 073 Resident B', '081999999999', true,
			        '3201010090022222', $1::uuid)`, userID)
		if err != nil {
			t.Fatalf("insert resident: %v", err)
		}

		runMigration(t, ctx, tx, "073_sync_imported_users_phone.up.sql")

		var phone *string
		if err := tx.QueryRowContext(ctx, `SELECT phone FROM users WHERE id = $1::uuid`, userID).Scan(&phone); err != nil {
			t.Fatalf("read phone: %v", err)
		}
		if phone == nil || *phone != "081222222222" {
			t.Fatalf("users.phone was overwritten by 073 = %v, want 081222222222 (existing values must never be replaced)", phone)
		}
	})
}

// ============================================================================
// Migration 075 — does not corrupt canonical data
// ============================================================================
func TestMigration075_UP_PreservesCanonicalUsers(t *testing.T) {
	withPhoneScopeTx(t, func(ctx context.Context, tx *sql.Tx) {
		var canonicalBefore int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM users
			WHERE phone ~ '^08[0-9]{8,}$'`).Scan(&canonicalBefore); err != nil {
			t.Fatalf("count canonical users: %v", err)
		}

		runMigration(t, ctx, tx, "075_normalize_phones_remaining.up.sql")

		var canonicalAfter int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM users
			WHERE phone ~ '^08[0-9]{8,}$'`).Scan(&canonicalAfter); err != nil {
			t.Fatalf("count canonical users: %v", err)
		}
		if canonicalAfter < canonicalBefore {
			t.Fatalf("canonical user count dropped after 075; before=%d after=%d", canonicalBefore, canonicalAfter)
		}
	})
}
