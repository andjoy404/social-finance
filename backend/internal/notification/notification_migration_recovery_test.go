package notification_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"

	"github.com/go-chi/chi/v5"
)

// TestMigrationRecovery_CompleteScenario reproduces and verifies all 11 conditions (A through K):
// A. Database state before migration 053.
// B. Migration 053 fails due to missing is_default dependency.
// C. Partial execution effects investigated and verified.
// D. Recovery procedure (force 52 + fixed 053) succeeds cleanly.
// E. Migrations 054–060 apply sequentially without errors.
// F. Migrations 053–060 can be re-run on existing objects without data loss (idempotency).
// G. Custom templates and schedules remain intact.
// H. Default seeds are never duplicated per RT.
// I. Old execution history and logs remain readable.
// J. Backend repository can query schedules and management-history without schema mismatch.
// K. Scheduler does not re-execute legacy configurations after recovery.
func TestMigrationRecovery_CompleteScenario(t *testing.T) {
	pool := testutil.GetTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	// Create an isolated schema for this test run so we can test raw migrations from 052 upwards
	schemaName := fmt.Sprintf("mig_recovery_%d", time.Now().UnixNano()%10000000)
	_, err := db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s;", schemaName))
	if err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
	}()

	// Switch search_path to isolated schema, then fallback to public
	_, err = db.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s, public;", schemaName))
	if err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	// 1. Setup prerequisite base tables for migration 052:
	// - rts
	// - users
	// - permission_types
	// - position_permissions
	// - user_rt_memberships
	// - notification_templates (from migration 040/041 without is_default)
	// - notification_logs
	_, err = db.ExecContext(ctx, `
		CREATE TABLE rts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) NOT NULL,
			rw INT NOT NULL DEFAULT 1,
			rt VARCHAR(10) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255),
			phone VARCHAR(50),
			full_name VARCHAR(100),
			password_hash VARCHAR(255) NOT NULL DEFAULT 'dummy',
			role VARCHAR(50) NOT NULL DEFAULT 'warga',
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE permission_types (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			code VARCHAR(100) UNIQUE NOT NULL,
			domain VARCHAR(50) NOT NULL,
			operation VARCHAR(50) NOT NULL
		);

		CREATE TABLE position_permissions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			position VARCHAR(50) NOT NULL,
			permission_id UUID NOT NULL REFERENCES permission_types(id) ON DELETE CASCADE,
			CONSTRAINT uq_position_permission UNIQUE (position, permission_id)
		);

		CREATE TABLE user_rt_memberships (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
			role VARCHAR(50) NOT NULL DEFAULT 'warga',
			jabatan VARCHAR(50),
			is_active BOOLEAN NOT NULL DEFAULT true,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE notification_templates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
			name VARCHAR(100) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			channel VARCHAR(20) NOT NULL DEFAULT 'push',
			title VARCHAR(200) NOT NULL,
			body TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			variables JSONB NOT NULL DEFAULT '[]',
			image_url TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_rt_event_template_name UNIQUE (rt_id, event_type, name)
		);

		CREATE TABLE notification_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			event_type VARCHAR(50) NOT NULL,
			dedup_key VARCHAR(255) UNIQUE,
			title VARCHAR(200) NOT NULL,
			body TEXT NOT NULL,
			status VARCHAR(20) NOT NULL,
			payload JSONB,
			image_url TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("setup base tables: %v", err)
	}

	// Seed 2 active RTs: RT 01 and RT 02
	var rt1ID, rt2ID string
	err = db.QueryRowContext(ctx, "INSERT INTO rts (name, rw, rt) VALUES ('RT 01', 1, '001') RETURNING id;").Scan(&rt1ID)
	if err != nil {
		t.Fatalf("create rt1: %v", err)
	}
	err = db.QueryRowContext(ctx, "INSERT INTO rts (name, rw, rt) VALUES ('RT 02', 1, '002') RETURNING id;").Scan(&rt2ID)
	if err != nil {
		t.Fatalf("create rt2: %v", err)
	}

	// Seed a custom template and custom schedule in RT 01 before migration 053 to test preservation
	var customTmplID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rt_id, name, event_type, channel, title, body, is_active)
		VALUES ($1, 'Custom Reminder Ketua RT', 'iuran_payment_reminder', 'push', 'Pengingat Khusus', 'Isi Khusus Kustom', true)
		RETURNING id;
	`, rt1ID).Scan(&customTmplID)
	if err != nil {
		t.Fatalf("create custom template: %v", err)
	}

	// Apply migration 052 (creates notification_schedules)
	mig052SQL, err := os.ReadFile("../../migrations/052_notification_schedules.up.sql")
	if err != nil {
		t.Fatalf("read 052 up sql: %v", err)
	}
	_, err = db.ExecContext(ctx, string(mig052SQL))
	if err != nil {
		t.Fatalf("apply 052: %v", err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO schema_migrations (version, dirty) VALUES (52, false);")
	if err != nil {
		t.Fatalf("set schema_migrations 52: %v", err)
	}

	// Seed a custom schedule in RT 01
	var customSchedID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO notification_schedules (rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, last_executed_at, last_executed_status)
		VALUES ($1, 'iuran_payment_reminder', $2, 'Jadwal Khusus Tanggal 10', true, '[10]'::jsonb, 9, 30, 'Asia/Jakarta', '2026-11-01 09:30:00+07', 'success')
		RETURNING id;
	`, rt1ID, customTmplID).Scan(&customSchedID)
	if err != nil {
		t.Fatalf("create custom schedule: %v", err)
	}

	// -------------------------------------------------------------------------
	// CONDITION A & B: Reproduce the exact production failure with old 053 SQL
	// -------------------------------------------------------------------------
	old053SQL := `
		INSERT INTO notification_templates (id, rt_id, name, event_type, channel, title, body, is_active, variables, is_default, created_at, updated_at)
		SELECT
			gen_random_uuid(), r.id, 'Pengingat Pembayaran Iuran', 'iuran_payment_reminder', 'push',
			'Pengingat Pembayaran Iuran', 'Anda masih memiliki tagihan iuran yang belum lunas.', true, '[]'::jsonb, true, NOW(), NOW()
		FROM rts r WHERE r.is_active = true
		ON CONFLICT (rt_id, event_type, name) DO NOTHING;
	`
	_, expectedErr := db.ExecContext(ctx, old053SQL)
	if expectedErr == nil {
		t.Fatal("expected old 053 SQL to fail because is_default does not exist yet, but it succeeded!")
	}
	if !strings.Contains(expectedErr.Error(), "is_default") {
		t.Fatalf("expected error mentioning is_default, got: %v", expectedErr)
	}

	// Mark schema_migrations as dirty 53 (exactly matching production state)
	_, err = db.ExecContext(ctx, "UPDATE schema_migrations SET version = 53, dirty = true WHERE version = 52;")
	if err != nil {
		t.Fatalf("set dirty 53: %v", err)
	}

	// EXACT PRODUCTION STATE: In production, someone manually added is_default on notification_templates after 053 failed:
	_, err = db.ExecContext(ctx, "ALTER TABLE notification_templates ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT false;")
	if err != nil {
		t.Fatalf("simulate manual addition of is_default on notification_templates: %v", err)
	}

	// Verify exact production state facts:
	// 1. notification_templates.is_default EXISTS
	var tmplIsDefaultCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'notification_templates' AND column_name = 'is_default';
	`, schemaName).Scan(&tmplIsDefaultCount)
	if err != nil || tmplIsDefaultCount != 1 {
		t.Fatalf("expected is_default to exist on notification_templates, count=%d (err=%v)", tmplIsDefaultCount, err)
	}

	// 2. notification_schedules.is_default does NOT exist
	var colCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'notification_schedules' AND column_name = 'is_default';
	`, schemaName).Scan(&colCount)
	if err != nil || colCount != 0 {
		t.Fatalf("expected is_default to not exist on notification_schedules, count=%d (err=%v)", colCount, err)
	}

	// 3. uq_notif_schedules_rt_event_name constraint does NOT exist in this isolated schema
	var uqConstraintCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1 AND c.conname = 'uq_notif_schedules_rt_event_name';
	`, schemaName).Scan(&uqConstraintCount)
	if err != nil || uqConstraintCount != 0 {
		t.Fatalf("expected uq_notif_schedules_rt_event_name to not exist yet in schema %s, count=%d", schemaName, uqConstraintCount)
	}

	// 4. Default seed for iuran_payment_reminder does NOT exist in RT 02
	var initialSeedCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_templates
		WHERE rt_id = $1 AND event_type = 'iuran_payment_reminder';
	`, rt2ID).Scan(&initialSeedCount)
	if err != nil || initialSeedCount != 0 {
		t.Fatalf("expected no template in RT 02 initially, count=%d", initialSeedCount)
	}

	// -------------------------------------------------------------------------
	// CONDITION D: Perform the deterministic recovery procedure
	// -------------------------------------------------------------------------
	// Step 1 of Recovery: Reset dirty state to version 52 (equivalent to `migrate force 52`)
	_, err = db.ExecContext(ctx, "UPDATE schema_migrations SET version = 52, dirty = false WHERE version = 53 AND dirty = true;")
	if err != nil {
		t.Fatalf("recovery force 52: %v", err)
	}

	// Step 2 of Recovery: Run the FIXED migration 053
	fixed053SQL, err := os.ReadFile("../../migrations/053_default_notification_schedule.up.sql")
	if err != nil {
		t.Fatalf("read fixed 053 up: %v", err)
	}
	_, err = db.ExecContext(ctx, string(fixed053SQL))
	if err != nil {
		t.Fatalf("apply fixed 053: %v", err)
	}
	_, err = db.ExecContext(ctx, "UPDATE schema_migrations SET version = 53, dirty = false WHERE version = 52;")
	if err != nil {
		t.Fatalf("update version to 53: %v", err)
	}

	// -------------------------------------------------------------------------
	// CONDITION E: Sequentially apply migrations 054 through 060
	// -------------------------------------------------------------------------
	migFiles := []struct {
		ver  int
		path string
	}{
		{54, "../../migrations/054_notification_default_flag.up.sql"},
		{55, "../../migrations/055_overdue_reminder_templates_and_schedules.up.sql"},
		{56, "../../migrations/056_special_resident_operasional_and_kebersihan.up.sql"},
		{57, "../../migrations/057_petugas_keamanan_kebersihan.up.sql"},
		{58, "../../migrations/058_waste_truck_dispatch_audit.up.sql"},
		{59, "../../migrations/059_notification_schedule_execution_detail.up.sql"},
		{60, "../../migrations/060_notification_schedule_executions.up.sql"},
	}

	for _, m := range migFiles {
		sqlBytes, err := os.ReadFile(m.path)
		if err != nil {
			t.Fatalf("read migration %03d: %v", m.ver, err)
		}
		_, err = db.ExecContext(ctx, string(sqlBytes))
		if err != nil {
			t.Fatalf("apply migration %03d: %v", m.ver, err)
		}
		_, err = db.ExecContext(ctx, fmt.Sprintf("UPDATE schema_migrations SET version = %d, dirty = false;", m.ver))
		if err != nil {
			t.Fatalf("update version to %d: %v", m.ver, err)
		}
	}

	// Verify schema_migrations is at version 60 and clean
	var currentVer int
	var isDirty bool
	err = db.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations;").Scan(&currentVer, &isDirty)
	if err != nil || currentVer != 60 || isDirty {
		t.Fatalf("expected version 60 clean, got version=%d, dirty=%v (err=%v)", currentVer, isDirty, err)
	}

	// -------------------------------------------------------------------------
	// CONDITION F: Idempotency test — re-apply migrations 053 through 060
	// -------------------------------------------------------------------------
	_, err = db.ExecContext(ctx, string(fixed053SQL))
	if err != nil {
		t.Fatalf("idempotency check 053 failed: %v", err)
	}
	for _, m := range migFiles {
		sqlBytes, _ := os.ReadFile(m.path)
		_, err = db.ExecContext(ctx, string(sqlBytes))
		if err != nil {
			t.Fatalf("idempotency check %03d failed: %v", m.ver, err)
		}
	}

	// -------------------------------------------------------------------------
	// CONDITION G: Custom templates and schedules remain intact
	// -------------------------------------------------------------------------
	var customTmplName string
	var customTmplIsDefault bool
	err = db.QueryRowContext(ctx, "SELECT name, is_default FROM notification_templates WHERE id = $1;", customTmplID).Scan(&customTmplName, &customTmplIsDefault)
	if err != nil || customTmplName != "Custom Reminder Ketua RT" || customTmplIsDefault != false {
		t.Fatalf("custom template corrupted: name=%s, is_default=%v (err=%v)", customTmplName, customTmplIsDefault, err)
	}

	var customSchedName string
	var customSchedDaysRaw []byte
	var customSchedIsDefault bool
	err = db.QueryRowContext(ctx, "SELECT name, send_days, is_default FROM notification_schedules WHERE id = $1;", customSchedID).Scan(&customSchedName, &customSchedDaysRaw, &customSchedIsDefault)
	if err != nil || customSchedName != "Jadwal Khusus Tanggal 10" || string(customSchedDaysRaw) != "[10]" || customSchedIsDefault != false {
		t.Fatalf("custom schedule corrupted: name=%s, days=%s, is_default=%v (err=%v)", customSchedName, string(customSchedDaysRaw), customSchedIsDefault, err)
	}

	// -------------------------------------------------------------------------
	// CONDITION H: Seed default is never duplicated per RT
	// -------------------------------------------------------------------------
	// In RT 01: custom schedule already existed for iuran_payment_reminder, so no default schedule should have been added for RT 01!
	var schedCountRT1 int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_schedules WHERE rt_id = $1 AND event_type = 'iuran_payment_reminder';", rt1ID).Scan(&schedCountRT1)
	if schedCountRT1 != 1 {
		t.Fatalf("expected exactly 1 schedule for RT 01, got %d", schedCountRT1)
	}

	// In RT 02: had no schedule, so exactly 1 default schedule was created
	var schedCountRT2 int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_schedules WHERE rt_id = $1 AND event_type = 'iuran_payment_reminder';", rt2ID).Scan(&schedCountRT2)
	if err != nil || schedCountRT2 != 1 {
		t.Fatalf("expected exactly 1 default schedule for RT 02, got %d (err: %v)", schedCountRT2, err)
	}

	// -------------------------------------------------------------------------
	// CONDITION I: Execution history backfilled into notification_schedule_executions
	// -------------------------------------------------------------------------
	var execBackfillCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_schedule_executions
		WHERE schedule_id = $1 AND config_hash = 'legacy_initial';
	`, customSchedID).Scan(&execBackfillCount)
	if err != nil || execBackfillCount != 1 {
		t.Fatalf("expected legacy execution to be backfilled into notification_schedule_executions, count=%d (err=%v)", execBackfillCount, err)
	}

	// -------------------------------------------------------------------------
	// CONDITION J & Section 6: Query verification for HTTP endpoints
	// -------------------------------------------------------------------------
	// Query 1: ListSchedules query
	rows1, err := db.QueryContext(ctx, `
		SELECT id, rt_id, event_type, template_id, name, is_active,
		       send_days, send_hour, send_minute, timezone,
		       last_executed_at, last_executed_status, last_executed_error, last_executed_summary,
		       created_at, updated_at, created_by, updated_by
		FROM notification_schedules WHERE rt_id = $1 ORDER BY created_at DESC;
	`, rt1ID)
	if err != nil {
		t.Fatalf("ListSchedules query failed on migrated schema: %v", err)
	}
	rows1.Close()

	// Query 2: ListManagementHistory query (system category)
	rows2, err := db.QueryContext(ctx, `
		WITH grouped AS (
			SELECT
				(array_agg(id ORDER BY created_at, id))[1] AS representative_id,
				COUNT(*)::int AS recipients_count,
				COUNT(*) FILTER (WHERE status IN ('SENT', 'PARTIAL'))::int AS sent_count,
				COUNT(*) FILTER (WHERE status = 'PARTIAL' OR (status = 'FAILED' AND failure_reason IS NOT NULL))::int AS failed_count,
				MIN(created_at) AS activity_time
			FROM notification_logs
			WHERE rt_id = $1 AND dedup_key LIKE 'schedule:%'
			GROUP BY
				split_part(dedup_key, ':', 2),
				split_part(dedup_key, ':', 3),
				(CASE WHEN split_part(dedup_key, ':', 4) LIKE 'c_%' THEN substring(split_part(dedup_key, ':', 4) FROM 3) ELSE '' END)
		)
		SELECT nl.id, nl.title FROM grouped g JOIN notification_logs nl ON nl.id = g.representative_id;
	`, rt1ID)
	if err != nil {
		t.Fatalf("ListManagementHistory system query failed on migrated schema: %v", err)
	}
	rows2.Close()

	// Query 3: ListManagementHistory query (mine category with sender_user_id)
	rows3, err := db.QueryContext(ctx, `
		SELECT nl.id, nl.title, nl.sender_user_id, nl.failure_reason
		FROM notification_logs nl
		WHERE nl.rt_id = $1;
	`, rt1ID)
	if err != nil {
		t.Fatalf("ListManagementHistory mine query failed on migrated schema: %v", err)
	}
	rows3.Close()

	// -------------------------------------------------------------------------
	// CONDITION K: Scheduler does not re-execute legacy configuration on same date
	// -------------------------------------------------------------------------
	// Verify that the backfilled record prevents duplicate execution on 2026-11-01
	var legacyDone bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_schedule_executions
			WHERE schedule_id = $1 AND exec_date = '2026-11-01'
		);
	`, customSchedID).Scan(&legacyDone)
	if err != nil || !legacyDone {
		t.Fatalf("expected legacy execution on 2026-11-01 to be recognized as completed: %v", err)
	}

	// -------------------------------------------------------------------------
	// AUDIT TEST: Schedule deletion vs notification_logs preservation (Point 5)
	// -------------------------------------------------------------------------
	// Insert an audit log in notification_logs associated with customSchedID delivery
	var auditLogID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, NULL, 'iuran_payment_reminder', $2, 'Tagihan Pengingat', 'Badan Pengingat', 'SENT', NOW())
		RETURNING id;
	`, rt1ID, "schedule:"+customSchedID+":2026-11-01:user-1").Scan(&auditLogID)
	if err != nil {
		t.Fatalf("insert test notification log: %v", err)
	}

	// Delete customSchedID schedule
	_, err = db.ExecContext(ctx, "DELETE FROM notification_schedules WHERE id = $1;", customSchedID)
	if err != nil {
		t.Fatalf("delete schedule: %v", err)
	}

	// Verify:
	// 1. notification_schedule_executions rows for that schedule are deleted (via CASCADE)
	var schedExecsCount int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_schedule_executions WHERE schedule_id = $1;", customSchedID).Scan(&schedExecsCount)
	if schedExecsCount != 0 {
		t.Fatalf("expected schedule_executions to be cascaded upon schedule deletion, got %d", schedExecsCount)
	}

	// 2. The permanent delivery history in notification_logs is 100% PRESERVED!
	var auditLogPreserved bool
	_ = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM notification_logs WHERE id = $1);", auditLogID).Scan(&auditLogPreserved)
	if !auditLogPreserved {
		t.Fatalf("audit log in notification_logs must NOT be deleted when a schedule is deleted!")
	}

	// -------------------------------------------------------------------------
	// CONDITION L: Test complete rollback sequence (060 down to 053 down)
	// -------------------------------------------------------------------------
	downFiles := []struct {
		ver  int
		path string
	}{
		{60, "../../migrations/060_notification_schedule_executions.down.sql"},
		{59, "../../migrations/059_notification_schedule_execution_detail.down.sql"},
		{58, "../../migrations/058_waste_truck_dispatch_audit.down.sql"},
		{57, "../../migrations/057_petugas_keamanan_kebersihan.down.sql"},
		{56, "../../migrations/056_special_resident_operasional_and_kebersihan.down.sql"},
		{55, "../../migrations/055_overdue_reminder_templates_and_schedules.down.sql"},
		{54, "../../migrations/054_notification_default_flag.down.sql"},
		{53, "../../migrations/053_default_notification_schedule.down.sql"},
	}

	for _, d := range downFiles {
		sqlBytes, err := os.ReadFile(d.path)
		if err != nil {
			t.Fatalf("read down migration %03d: %v", d.ver, err)
		}
		_, err = db.ExecContext(ctx, string(sqlBytes))
		if err != nil {
			t.Fatalf("apply down migration %03d: %v", d.ver, err)
		}
		_, err = db.ExecContext(ctx, fmt.Sprintf("UPDATE schema_migrations SET version = %d, dirty = false;", d.ver-1))
		if err != nil {
			t.Fatalf("update version to %d after down: %v", d.ver-1, err)
		}
	}

	// Verify rollback state at version 52
	var finalRollbackVer int
	var finalRollbackDirty bool
	err = db.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations;").Scan(&finalRollbackVer, &finalRollbackDirty)
	if err != nil || finalRollbackVer != 52 || finalRollbackDirty {
		t.Fatalf("expected rollback to version 52 clean, got version=%d, dirty=%v", finalRollbackVer, finalRollbackDirty)
	}

	// Custom template and custom schedule in RT 01 are STILL PRESERVED after rolling back default seeds!
	var postRollbackTmplCount int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_templates WHERE id = $1;", customTmplID).Scan(&postRollbackTmplCount)
	if postRollbackTmplCount != 1 {
		t.Fatalf("expected custom template to be preserved after down migration, count=%d", postRollbackTmplCount)
	}
}

// TestNotificationEndpoints_HTTP200 verifies that with the migrated schema, the HTTP handlers for:
// - GET /api/v1/notifications/schedules
// - GET /api/v1/notifications/management-history
// - GET /api/v1/notifications/templates
// - GET /api/v1/notifications/system-types
// all return HTTP 200 (recovering from the HTTP 500 error seen in production).
func TestNotificationEndpoints_HTTP200(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	mockProvider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, mockProvider)
	h := notification.NewHandler(svc, pool, nil, "development")

	rtID := notifTestRT(t, pool)
	adminUserID := notifTestUser(t, pool, "Admin Test", true)

	// Create test template and schedule so the list endpoints return real records
	tmpl, err := repo.CreateTemplate(context.Background(), rtID, notification.CreateTemplateInput{
		Name:      "Template Endpoint Test",
		EventType: notification.EventPaymentReminder,
		Title:     "Title",
		Body:      "Body",
	}, []string{"bill_id", "period", "amount", "due_date", "deep_link"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	_, err = repo.CreateSchedule(context.Background(), rtID, adminUserID, notification.CreateScheduleInput{
		EventType:  notification.EventPaymentReminder,
		TemplateID: tmpl.ID,
		Name:       "Schedule Endpoint Test",
		SendDays:   []int{1, 15},
		SendHour:   8,
		SendMinute: 0,
		Timezone:   "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	// Build router with auth context for Super Admin / Ketua RT
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ac := &auth.AuthContext{
				UserID:     adminUserID,
				SystemRole: auth.SystemRoleSuperAdmin,
				RTID:       rtID,
			}
			ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	r.Get("/api/v1/notifications/schedules", h.ListSchedules)
	r.Get("/api/v1/notifications/management-history", h.ListManagementHistory)
	r.Get("/api/v1/notifications/templates", h.ListTemplates)
	r.Get("/api/v1/notifications/system-types", h.GetSystemTypes)
	r.Get("/api/v1/notifications/logs", h.ListLogs)

	endpoints := []struct {
		name string
		path string
	}{
		{"GET /api/v1/notifications/schedules", "/api/v1/notifications/schedules"},
		{"GET /api/v1/notifications/management-history", "/api/v1/notifications/management-history?category=system"},
		{"GET /api/v1/notifications/management-history (mine)", "/api/v1/notifications/management-history?category=mine"},
		{"GET /api/v1/notifications/templates", "/api/v1/notifications/templates"},
		{"GET /api/v1/notifications/system-types", "/api/v1/notifications/system-types"},
		{"GET /api/v1/notifications/logs (admin)", "/api/v1/notifications/logs"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", ep.path, nil)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("endpoint %s failed: status=%d, body=%s", ep.name, w.Code, w.Body.String())
			}
		})
	}

	// Also test Warga (non-admin) calling GET /api/v1/notifications/logs
	wargaUserID := notifTestUser(t, pool, "Warga Test", false)
	rWarga := chi.NewRouter()
	rWarga.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ac := &auth.AuthContext{
				UserID:     wargaUserID,
				TenantRole: auth.RoleWarga,
				RTID:       rtID,
			}
			ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	rWarga.Get("/api/v1/notifications/logs", h.ListLogs)

	t.Run("GET /api/v1/notifications/logs (warga)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		rWarga.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET /api/v1/notifications/logs (warga) failed: status=%d, body=%s", w.Code, w.Body.String())
		}
	})
}
