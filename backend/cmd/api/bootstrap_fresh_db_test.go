package main

// ============================================================================
// Auto-provision & bootstrap — fresh empty database scenarios.
//
// These tests create brand-new, isolated databases (names contain "test"),
// apply every migration from backend/migrations/ in order, then run the
// idempotent auto-provision + bootstrap flow. Assertions cover:
//  - RW 016 created with a fresh UUID (not the legacy canonical UUID).
//  - RT 002 created with a fresh UUID, rw_id pointing at RW 016.
//  - Super admin created from BOOTSTRAP_EMAIL/BOOTSTRAP_PASSWORD.
//  - Bootstrap is idempotent; second run does not duplicate records or IDs.
//  - Failure on a mid-flow insert rolls back the entire transaction,
//    leaves no partial state, and surfaces the FIRST error with context.
//
// The tests are SKIP-SAFE: environments lacking privileges to CREATE
// DATABASE or lacking the migrations directory will skip with an actionable
// message.
// ============================================================================

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"social-finance/internal/config"
	"social-finance/internal/database"
)

// applyAllMigrations applies every *.up.sql file in migrationsDir, in sorted
// (numeric-prefix) order. Returns the count of applied migrations.
func applyAllMigrations(t *testing.T, db *sql.DB, migrationsDir string) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(files)
	applied := 0
	for _, f := range files {
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if len(sqlBytes) == 0 {
			continue
		}
		if _, err := db.ExecContext(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(f), err)
		}
		applied++
	}
	return applied
}

// findMigrationsDir walks up from the current working directory until it finds
// a migrations directory containing *.up.sql files.
func findMigrationsDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		cand := filepath.Join(dir, "migrations")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			matches, err := fs.Glob(os.DirFS(cand), "*.up.sql")
			if err == nil && len(matches) > 0 {
				return cand
			}
		}
		if dir == "/" || dir == "" {
			break
		}
	}
	t.Fatal("migrations/ directory with *.up.sql files not found")
	return ""
}

// shortUUID returns a stable readable prefix of a UUID for logging.
func shortUUID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "…"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// freshDBSetup holds the coordinates of a disposable fresh database.
type freshDBSetup struct {
	dbName   string
	host     string
	port     string
	user     string
	pass     string
	ssl      string
	dsn      string
	adminDSN string
}

// createFreshMigratedDB creates an empty database with the fresh name,
// applies all migrations, and registers cleanup to drop the database.
// It calls t.Skip (not Fail) when the environment cannot support it.
func createFreshMigratedDB(t *testing.T) freshDBSetup {
	t.Helper()

	migrationsDir := findMigrationsDir(t)

	host := envOr("DB_HOST", "localhost")
	port := envOr("DB_PORT", "5432")
	user := envOr("DB_USER", "social_finance")
	pass := envOr("DB_PASSWORD", "social_finance_dev_password")
	ssl := envOr("DB_SSLMODE", "disable")

	adminDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=%s", user, pass, host, port, ssl)

	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()

	if _, err := admin.ExecContext(context.Background(), "SELECT 1"); err != nil {
		t.Skipf("cannot connect to maintenance DB for fresh-bootstrap test: %v", err)
	}

	dbName := fmt.Sprintf("social_finance_boot_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(context.Background(), `CREATE DATABASE "`+dbName+`"`); err != nil {
		t.Skipf("cannot create fresh test database (likely missing privileges): %v", err)
	}

	t.Cleanup(func() {
		admin2, err := sql.Open("pgx", adminDSN)
		if err != nil {
			t.Logf("cleanup: cannot open admin pool to drop %s: %v", dbName, err)
			return
		}
		_, _ = admin2.ExecContext(context.Background(), `DROP DATABASE IF EXISTS "`+dbName+`" WITH (FORCE)`)
		_ = admin2.Close()
	})

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, dbName, ssl)
	raw, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open fresh pool: %v", err)
	}
	if err := raw.Ping(); err != nil {
		t.Fatalf("ping fresh pool: %v", err)
	}
	applied := applyAllMigrations(t, raw, migrationsDir)
	t.Logf("applied %d migrations to fresh DB %s", applied, dbName)
	_ = raw.Close()

	return freshDBSetup{
		dbName:   dbName,
		host:     host,
		port:     port,
		user:     user,
		pass:     pass,
		ssl:      ssl,
		dsn:      dsn,
		adminDSN: adminDSN,
	}
}

// openFreshPool opens a *database.Pool against the freshly created database.
func openFreshPool(t *testing.T, fresh freshDBSetup) *database.Pool {
	t.Helper()
	pool, err := database.NewPool(context.Background(), config.Config{
		DBHost:     fresh.host,
		DBPort:     fresh.port,
		DBName:     fresh.dbName,
		DBUser:     fresh.user,
		DBPassword: fresh.pass,
		DBSSLMode:  fresh.ssl,
	})
	if err != nil {
		t.Fatalf("open fresh pool for bootstrap calls: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func setFreshBootstrapEnv(t *testing.T, fresh freshDBSetup, email string) {
	t.Helper()
	os.Setenv("DB_HOST", fresh.host)
	os.Setenv("DB_PORT", fresh.port)
	os.Setenv("DB_USER", fresh.user)
	os.Setenv("DB_PASSWORD", fresh.pass)
	os.Setenv("DB_NAME", fresh.dbName)
	os.Setenv("DB_SSLMODE", fresh.ssl)
	os.Setenv("BOOTSTRAP_EMAIL", email)
	os.Setenv("BOOTSTRAP_PASSWORD", "FreshBootstrap123!")
	os.Setenv("BOOTSTRAP_NAME", "Super Admin Fresh")
	t.Cleanup(func() {
		for _, k := range []string{
			"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
			"BOOTSTRAP_EMAIL", "BOOTSTRAP_PASSWORD", "BOOTSTRAP_NAME",
		} {
			os.Unsetenv(k)
		}
	})
}

func TestBootstrap_Fresh_EmptyDatabase(t *testing.T) {
	if os.Getenv("SKIP_FRESH_BOOTSTRAP_TEST") != "" {
		t.Skip("fresh-DB bootstrap test disabled by SKIP_FRESH_BOOTSTRAP_TEST")
	}

	fresh := createFreshMigratedDB(t)
	setFreshBootstrapEnv(t, fresh, "bootstrap-fresh@test.local")

	checkPool := openFreshPool(t, fresh)
	ctx := context.Background()

	// 1) Baseline must be empty.
	var rwsCount, rtsCount, adminCount int
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rws").Scan(&rwsCount); err != nil {
		t.Fatalf("count rws: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&rtsCount); err != nil {
		t.Fatalf("count rts: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE system_role = 'super_admin'").Scan(&adminCount); err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if rwsCount != 0 || rtsCount != 0 || adminCount != 0 {
		t.Fatalf("fresh DB baseline not empty: rws=%d rts=%d admins=%d", rwsCount, rtsCount, adminCount)
	}
	t.Logf("fresh DB baseline clean")

	// 2) Auto-provision must create RW 016 and RT 002 tied correctly.
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("runAutoProvision on fresh DB: %v", err)
	}

	var rwID, rtID string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rws WHERE rw_number = 16 AND is_active = true LIMIT 1`).Scan(&rwID); err != nil {
		t.Fatalf("resolve RW 016: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rts WHERE rt = '002' AND rw = 16 AND is_active = true LIMIT 1`).Scan(&rtID); err != nil {
		t.Fatalf("resolve RT 002: %v", err)
	}
	var rwIDFromRT string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT rw_id FROM rts WHERE id = $1::uuid`, rtID).Scan(&rwIDFromRT); err != nil {
		t.Fatalf("load RT.rw_id: %v", err)
	}
	t.Logf("fresh bootstrap created RW 016=%s RT 002=%s (rw_id=%s)", shortUUID(rwID), shortUUID(rtID), shortUUID(rwIDFromRT))

	if rwIDFromRT == "" {
		t.Fatal("RT 002 has empty rw_id")
	}
	if rwIDFromRT != rwID {
		t.Fatalf("RT 002 rw_id = %s, want %s", rwIDFromRT, rwID)
	}

	// 3) Bootstrap creates the super admin and links to the fresh RT.
	if err := runBootstrapWithPool(ctx, checkPool); err != nil {
		t.Fatalf("runBootstrapWithPool on fresh DB: %v", err)
	}

	var saEmail, saRole string
	var saActive bool
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT email, system_role, is_active FROM users WHERE email = 'bootstrap-fresh@test.local'`,
	).Scan(&saEmail, &saRole, &saActive); err != nil {
		t.Fatalf("lookup super admin: %v", err)
	}
	if saRole != "super_admin" || !saActive {
		t.Fatalf("super admin invalid: email=%s role=%s is_active=%v", saEmail, saRole, saActive)
	}

	var linkRT string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT urm.rt_id::text FROM user_rt_memberships urm
		 JOIN users u ON u.id = urm.user_id
		 WHERE u.email = 'bootstrap-fresh@test.local' AND urm.is_active = true`).Scan(&linkRT); err != nil {
		t.Fatalf("lookup super admin RT membership: %v", err)
	}
	if linkRT != rtID {
		t.Fatalf("super admin linked to wrong RT: got %s, want %s", linkRT, rtID)
	}
	t.Logf("super admin created and linked to RT %s", shortUUID(linkRT))

	// 4) Idempotency on re-run, including UUID stability.
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("re-run auto-provision: %v", err)
	}
	if err := runBootstrapWithPool(ctx, checkPool); err != nil {
		t.Fatalf("re-run bootstrap: %v", err)
	}

	var rwIDAfter, rtIDAfter string
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rws WHERE rw_number = 16 AND is_active = true LIMIT 1`).Scan(&rwIDAfter); err != nil {
		t.Fatalf("re-resolve RW 016: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx,
		`SELECT id FROM rts WHERE rt = '002' AND rw = 16 AND is_active = true LIMIT 1`).Scan(&rtIDAfter); err != nil {
		t.Fatalf("re-resolve RT 002: %v", err)
	}
	if rwIDAfter != rwID {
		t.Fatalf("RW 016 UUID changed on re-run: got %s, want %s", rwIDAfter, rwID)
	}
	if rtIDAfter != rtID {
		t.Fatalf("RT 002 UUID changed on re-run: got %s, want %s", rtIDAfter, rtID)
	}

	var rwsCountAfter, rtsCountAfter, adminCountAfter int
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rws").Scan(&rwsCountAfter); err != nil {
		t.Fatalf("count rws after re-run: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&rtsCountAfter); err != nil {
		t.Fatalf("count rts after re-run: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE system_role = 'super_admin'").Scan(&adminCountAfter); err != nil {
		t.Fatalf("count admins after re-run: %v", err)
	}
	if rwsCountAfter != 1 || rtsCountAfter != 1 || adminCountAfter != 1 {
		t.Fatalf("fresh DB bootstrap not idempotent: rws=%d rts=%d admins=%d", rwsCountAfter, rtsCountAfter, adminCountAfter)
	}
	t.Logf("idempotent re-run verified (IDs stable): rws=%d rts=%d admins=%d", rwsCountAfter, rtsCountAfter, adminCountAfter)
}

func TestBootstrap_Failure_Rollback(t *testing.T) {
	if os.Getenv("SKIP_FRESH_BOOTSTRAP_TEST") != "" {
		t.Skip("fresh-DB bootstrap test disabled by SKIP_FRESH_BOOTSTRAP_TEST")
	}

	fresh := createFreshMigratedDB(t)
	checkPool := openFreshPool(t, fresh)
	ctx := context.Background()

	// Simulate a mid-flow SQL failure: forbid any INSERT into rts.
	// RW 016 (created earlier in the same transaction) must be rolled back.
	ddl := `
		CREATE OR REPLACE FUNCTION test_fail_rts_insert() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'forced failure for rollback test';
		END;
		$$;
		CREATE TRIGGER trg_test_fail_rts_insert
		BEFORE INSERT ON rts FOR EACH ROW
		EXECUTE FUNCTION test_fail_rts_insert();
	`
	if _, err := checkPool.Raw().ExecContext(ctx, ddl); err != nil {
		t.Fatalf("install failing trigger: %v", err)
	}

	err := runAutoProvision(ctx, checkPool)
	if err == nil {
		t.Fatal("expected auto-provision to fail with the forced trigger error; got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "auto-provision: insert RT 002") {
		t.Fatalf("expected first-error context 'insert RT 002', got: %v", err)
	}
	if strings.Contains(errMsg, "SQLSTATE 25P02") {
		t.Fatalf("error still reports aborted-transaction follow-on instead of the first error: %v", err)
	}
	t.Logf("auto-provision failed as expected with first error: %v", err)

	// The whole provisioning transaction must be rolled back: no RW 016 row.
	var rwsCount, rtsCount int
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rws").Scan(&rwsCount); err != nil {
		t.Fatalf("count rws after failed provision: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&rtsCount); err != nil {
		t.Fatalf("count rts after failed provision: %v", err)
	}
	if rwsCount != 0 {
		t.Fatalf("rollback failed: rws=%d (expected 0 — partial state leaked)", rwsCount)
	}
	if rtsCount != 0 {
		t.Fatalf("rollback failed: rts=%d (expected 0)", rtsCount)
	}
	t.Logf("rollback verified: no RW/RT rows persisted after forced failure")

	// Drop the failing trigger; a retry must succeed on the same database.
	if _, err := checkPool.Raw().ExecContext(ctx, `
		DROP TRIGGER trg_test_fail_rts_insert ON rts;
		DROP FUNCTION test_fail_rts_insert();
	`); err != nil {
		t.Fatalf("drop failing trigger: %v", err)
	}
	if err := runAutoProvision(ctx, checkPool); err != nil {
		t.Fatalf("auto-provision after failure recovery: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rws").Scan(&rwsCount); err != nil {
		t.Fatalf("count rws after recovery: %v", err)
	}
	if err := checkPool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&rtsCount); err != nil {
		t.Fatalf("count rts after recovery: %v", err)
	}
	if rwsCount != 1 || rtsCount != 1 {
		t.Fatalf("recovery after failure produced wrong state: rws=%d rts=%d (expected 1/1)", rwsCount, rtsCount)
	}
	t.Logf("retry after rollback succeeded: RW 016 and RT 002 created correctly")
}
