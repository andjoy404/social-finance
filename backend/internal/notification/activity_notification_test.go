package notification

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

// insertActivity inserts a published activity starting tomorrow and lasting ~2h.
//
//	overrides maps column name -> literal SQL expression, e.g. map[string]string{
//	  "status": "'cancelled'",
//	  "ends_at": "NOW() - INTERVAL '1 hour'",
//	}.
func insertActivity(t *testing.T, pool *database.Pool, rtID, title string, overrides ...map[string]string) string {
	t.Helper()
	ctx := context.Background()

	cols := []string{
		"rt_id", "title", "description", "starts_at", "ends_at",
		"location", "organizer", "status", "publication_status", "show_on_banner",
	}
	vals := map[string]string{
		"description":        "'Deskripsi kegiatan'",
		"starts_at":          "NOW() + INTERVAL '1 day'",
		"ends_at":            "NOW() + INTERVAL '1 day 2 hours'",
		"location":           "'Balai RT'",
		"organizer":          "'Pengurus RT'",
		"status":             "'upcoming'",
		"publication_status": "'published'",
		"show_on_banner":     "false",
	}
	for _, o := range overrides {
		if o == nil {
			continue
		}
		for k, v := range o {
			vals[k] = v
		}
	}

	strLit := strings.ReplaceAll(title, "'", "''")
	placeVals := make([]string, 0, len(cols))
	args := []any{}
	for _, col := range cols {
		if col == "rt_id" {
			placeVals = append(placeVals, fmt.Sprintf("$%d", len(args)+1))
			args = append(args, rtID)
			continue
		}
		if col == "title" {
			placeVals = append(placeVals, fmt.Sprintf("$%d", len(args)+1))
			args = append(args, strLit)
			continue
		}
		placeVals = append(placeVals, vals[col])
	}

	query := fmt.Sprintf(`
		INSERT INTO activities (%s)
		VALUES (%s)
		RETURNING id`, strings.Join(cols, ", "), strings.Join(placeVals, ", "))

	var id string
	err := pool.Raw().QueryRowContext(ctx, query, args...).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertActivityTemplate seeds a template for a given event_type.
func insertActivityTemplate(t *testing.T, pool *database.Pool, rtID, eventType, name, title, body string) string {
	t.Helper()
	var id string
	err := pool.Raw().QueryRowContext(context.Background(), `
		INSERT INTO notification_templates (rt_id, event_type, name, title, body, is_active, channel, variables, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, 'push', '[]'::jsonb, NOW(), NOW())
		RETURNING id`, rtID, eventType, name, title, body).Scan(&id)
	require.NoError(t, err)
	return id
}

// createActivitySchedule makes a one-shot reminder schedule tied to an activity and returns its id.
func createActivitySchedule(t *testing.T, pool *database.Pool, rtID, templateID, name, activityID string, execAt time.Time) string {
	t.Helper()
	var id string
	err := pool.Raw().QueryRowContext(context.Background(), `
		INSERT INTO notification_schedules (rt_id, event_type, template_id, name, is_active,
		                                    send_days, send_hour, send_minute, timezone,
		                                    activity_id, execution_at)
		VALUES ($1, 'activity_reminder', $2, $3, true,
		        '[]'::jsonb, 8, 0, 'Asia/Jakarta',
		        $4, $5)
		RETURNING id`, rtID, templateID, name, activityID, execAt).Scan(&id)
	require.NoError(t, err)
	return id
}

// TestActivityEventTypesRegistered covers spec scenario #1.
func TestActivityEventTypesRegistered(t *testing.T) {
	pool := testutil.GetTestPool(t)
	svc := NewService(NewRepository(pool), NewMockPushProvider())
	types := svc.GetSystemTypes()

	actEv := map[string]SystemNotificationType{}
	for _, tp := range types {
		actEv[tp.EventType] = tp
	}
	// By-System events are permanent and cannot be created/scheduled/toggled/deleted
	// by default Pengurus actions. Activity reminder (terjadwal) stays configurable.
	expectFixed := map[string]bool{
		EventActivityCreated:   true,
		EventActivityUpdated:   true,
		EventActivityReminder:  false,
		EventActivityCancelled: true,
		EventActivityCompleted: true,
	}
	for _, ev := range []string{
		EventActivityCreated, EventActivityUpdated, EventActivityReminder,
		EventActivityCancelled, EventActivityCompleted,
	} {
		st, ok := actEv[ev]
		require.True(t, ok, "event %s must be registered in SystemNotificationTypes", ev)
		require.True(t, st.IsConfigurable, "event %s must be configurable", ev)
		for _, v := range []string{
			"nama_kegiatan", "deskripsi_kegiatan", "tanggal_kegiatan",
			"jam_kegiatan", "lokasi_kegiatan", "penyelenggara", "nama_warga",
		} {
			require.Contains(t, st.AllowedVariables, v, "event %s must expose variable %s", ev, v)
		}
		require.Equal(t, expectFixed[ev], IsFixedSystemEvent(ev), "IsFixedSystemEvent(%s)", ev)
	}
}

// TestOneTemplateManyActivities covers spec scenario #2.
func TestOneTemplateManyActivities(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T2")
	defer cleanupNotifTestRT(t, pool, rtID)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Kegiatan Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	_, _ = pool.Raw().ExecContext(context.Background(), // users required by broadcast fanout queries
		`INSERT INTO users (full_name, email, phone, password_hash, is_active)
		 SELECT 'Test Broadcast User', 'broadcast-'||$2::text||'@local', '081200001234', 'hash', true
		 WHERE NOT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower('broadcast-'||$2::text||'@local'))`,
		rtID, fmt.Sprint(time.Now().UnixNano()))

	actA := insertActivity(t, pool, rtID, "Kegiatan Alpha")
	actB := insertActivity(t, pool, rtID, "Kegiatan Beta")
	execAt := time.Now().Add(24 * time.Hour)

	schedA := createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actA, execAt)
	schedB := createActivitySchedule(t, pool, rtID, templateID, "Reminder B", actB, execAt)

	var count int
	err := pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM notification_schedules WHERE template_id = $1 AND rt_id = $2",
		templateID, rtID).Scan(&count)
	require.NoError(t, err)
	_ = schedA
	_ = schedB
	require.Equal(t, 2, count, "same template may back multiple activities' schedules")
}

// TestCancelledActivityDeactivatesPendingSchedules covers spec scenario #3.
func TestCancelledActivityDeactivatesPendingSchedules(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T3")
	defer cleanupNotifTestRT(t, pool, rtID)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actID := insertActivity(t, pool, rtID, "Kegiatan Dibatalkan")
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(24*time.Hour))

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	require.NoError(t, svc.CancelActivityPendingSchedules(context.Background(), rtID, actID))
	active := 0
	err := pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM notification_schedules WHERE activity_id = $1 AND is_active = true", actID).Scan(&active)
	require.NoError(t, err)
	require.Equal(t, 0, active, "pending schedules must be deactivated after cancellation")
}

// TestCompletedActivityDeactivatesPendingSchedules covers spec scenario #4.
func TestCompletedActivityDeactivatesPendingSchedules(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T4")
	defer cleanupNotifTestRT(t, pool, rtID)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actID := insertActivity(t, pool, rtID, "Kegiatan Selesai",
		map[string]string{"ends_at": "NOW() - INTERVAL '1 hour'"})

	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(-time.Hour))

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	require.NoError(t, svc.RunActivityMaintenance(context.Background()))

	active := 0
	err := pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM notification_schedules WHERE activity_id = $1 AND is_active = true", actID).Scan(&active)
	require.NoError(t, err)
	require.Equal(t, 0, active)
}

// TestCompletedActivityKeepsGlobalTemplateActive covers spec scenario #5.
func TestCompletedActivityKeepsGlobalTemplateActive(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T5")
	defer cleanupNotifTestRT(t, pool, rtID)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actID := insertActivity(t, pool, rtID, "Kegiatan Selesai X",
		map[string]string{"ends_at": "NOW() - INTERVAL '1 hour'"})
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(-time.Hour))

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	require.NoError(t, svc.RunActivityMaintenance(context.Background()))

	tmplActive := false
	err := pool.Raw().QueryRowContext(context.Background(),
		"SELECT is_active FROM notification_templates WHERE id = $1", templateID).Scan(&tmplActive)
	require.NoError(t, err)
	require.True(t, tmplActive, "template global tetap aktif setelah kegiatan selesai")
}

// TestCancelledActivityPreservesSentHistory covers spec scenario #6.
// A reminder that already went out keeps its delivery rows intact after cancellation.
func TestCancelledActivityPreservesSentHistory(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T6")
	defer cleanupNotifTestRT(t, pool, rtID)

	var userID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (full_name, email, phone, password_hash, is_active)
		 VALUES ('Uji T6', $1, '081200006666', 'hash', true) RETURNING id`,
		fmt.Sprintf("t6-%d@test.local", time.Now().UnixNano())).Scan(&userID)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(context.Background(),
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, 'warga', true)`, userID, rtID)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(context.Background(),
		`INSERT INTO user_devices (user_id, fcm_token, platform, is_active) VALUES ($1, 'tok-t6-history', 'android', true)`, userID)
	require.NoError(t, err)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	// Activity is currently upcoming with ends_at in future — reminder genuinely "fires" before cancellation.
	actID := insertActivity(t, pool, rtID, "Kegiatan History")
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(-time.Minute))

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	repo := NewRepository(pool)
	list, err := repo.ListSchedules(context.Background(), rtID, "activity_reminder", nil)
	require.NoError(t, err)
	require.Len(t, list, 1)

	_, err = svc.ExecuteSchedule(context.Background(), list[0], time.Now())
	require.NoError(t, err)

	sentBefore := 0
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND status = 'SENT'", rtID).Scan(&sentBefore)
	require.NoError(t, err)
	require.GreaterOrEqual(t, sentBefore, 1, "reminder already went out — sent rows must exist")

	// Cancel via direct status change (mirrors community.CancelActivity persistence path).
	_, err = pool.Raw().ExecContext(context.Background(),
		"UPDATE activities SET status = 'cancelled' WHERE id = $1", actID)
	require.NoError(t, err)
	_ = svc.CancelActivityPendingSchedules(context.Background(), rtID, actID)

	// Sweep fires 'activity_cancelled' — evidence must include SENT log rows intact.
	_ = svc.RunActivityMaintenance(context.Background())

	var sentAfter int
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND status = 'SENT'", rtID).Scan(&sentAfter)
	require.NoError(t, err)
	require.GreaterOrEqual(t, sentAfter, sentBefore, "cancellation must never delete prior sent rows")

	var cancelLogCount int
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND event_type = 'activity_cancelled'", rtID).Scan(&cancelLogCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, cancelLogCount, 1, "cancellation event must produce a log record")
}

// TestUpdatedActivityReconcilesFutureReminder covers spec scenario #7.
func TestUpdatedActivityReconcilesFutureReminder(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T7")
	defer cleanupNotifTestRT(t, pool, rtID)

	t7email := fmt.Sprintf("t7-%d@test.local", time.Now().UnixNano())
	_, err := pool.Raw().ExecContext(context.Background(),
		`INSERT INTO users (full_name, email, phone, password_hash, is_active)
		 VALUES ('Uji T7', $1, '081300001', 'hash', true)`, t7email)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(context.Background(),
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		 SELECT id, $1, 'warga', true FROM users WHERE email = $2`, rtID, t7email)
	require.NoError(t, err)
	_, err = pool.Raw().ExecContext(context.Background(),
		`INSERT INTO user_devices (user_id, fcm_token, platform, is_active)
		 SELECT id, 'tok-t7', 'android', true FROM users WHERE email = $1`, t7email)
	require.NoError(t, err)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actID := insertActivity(t, pool, rtID, "Kegiatan Asal")
	execAt := time.Now().Add(2 * time.Hour)
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, execAt)

	_, err = pool.Raw().ExecContext(context.Background(),
		"UPDATE activities SET title = 'Kegiatan Baru' WHERE id = $1", actID)
	require.NoError(t, err)

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	_, err = svc.GetDueSchedules(context.Background(), execAt.Add(time.Second))
	require.NoError(t, err)

	// Schedule one-shot execution manually for determinism
	var dueSchedules []*NotificationSchedule
	repo := NewRepository(pool)
	list, err := repo.ListSchedules(context.Background(), rtID, "activity_reminder", nil)
	require.NoError(t, err)
	for _, s := range list {
		if s.ActivityID != nil && *s.ActivityID == actID && s.IsActive {
			dueSchedules = append(dueSchedules, s)
		}
	}
	require.Len(t, dueSchedules, 1)
	_, err = svc.ExecuteSchedule(context.Background(), dueSchedules[0], execAt.Add(time.Second))
	require.NoError(t, err)

	// The sent notification body should reference the updated title
	var body string
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT body FROM notification_logs WHERE rt_id = $1 AND event_type = 'activity_reminder' ORDER BY created_at DESC LIMIT 1",
		rtID).Scan(&body)
	require.NoError(t, err)
	require.Contains(t, body, "Kegiatan Baru")
}

// TestReactivationDoesNotAutoEnableSchedules covers spec scenario #8.
func TestReactivationDoesNotAutoEnableSchedules(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T8")
	defer cleanupNotifTestRT(t, pool, rtID)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")
	actID := insertActivity(t, pool, rtID, "Kegiatan Aktif Kembali",
		map[string]string{"status": "'cancelled'"})
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(24*time.Hour))

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	_ = svc.CancelActivityPendingSchedules(context.Background(), rtID, actID)
	_, err := pool.Raw().ExecContext(context.Background(), // reactivate the activity
		"UPDATE activities SET status = 'upcoming' WHERE id = $1", actID)
	require.NoError(t, err)

	require.NoError(t, svc.RunActivityMaintenance(context.Background()))
	// After reactivation, the schedule remains inactive.
	isActive := false
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT is_active FROM notification_schedules WHERE activity_id = $1", actID).Scan(&isActive)
	require.NoError(t, err)
	require.False(t, isActive, "cancelled schedules must not auto-enable on reactivation")
}

// TestInactiveTemplateSuppressesDelivery covers spec scenario #9.
func TestInactiveTemplateSuppressesDelivery(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T9")
	defer cleanupNotifTestRT(t, pool, rtID)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	_, _ = pool.Raw().ExecContext(context.Background(),
		`UPDATE notification_templates SET is_active = false WHERE id = $1`, templateID)

	actID := insertActivity(t, pool, rtID, "Kegiatan Template Off")
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(2*time.Hour))

	mock := NewMockPushProvider()
	svc := NewService(NewRepository(pool), mock)
	repo := NewRepository(pool)
	list, err := repo.ListSchedules(context.Background(), rtID, "activity_reminder", nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	_, err = svc.ExecuteSchedule(context.Background(), list[0], time.Now().Add(2*time.Hour))

	require.Error(t, err, "execution with inactive template must fail — no notification dispatch")
	require.Len(t, mock.GetMessages(), 0, "no push messages may be sent while template is inactive")

	// Execution should be recorded as skipped and schedule deactivated.
	isActive := true
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT is_active FROM notification_schedules WHERE activity_id = $1", actID).Scan(&isActive)
	require.NoError(t, err)
	require.False(t, isActive)
}

// TestRetryDoesNotDuplicateDelivery covers spec scenario #10.
func TestRetryDoesNotDuplicateDelivery(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T10")
	defer cleanupNotifTestRT(t, pool, rtID)

	insertMember := func(name string) string {
		var userID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO users (full_name, email, phone, password_hash, is_active)
			 VALUES ($1, $2, '081211112222', 'hash', true) RETURNING id`,
			name, fmt.Sprintf("t10-%s-%d@test.local", name, time.Now().UnixNano())).Scan(&userID)
		require.NoError(t, err)
		_, err = pool.Raw().ExecContext(context.Background(),
			`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, 'warga', true)`, userID, rtID)
		require.NoError(t, err)
		_, err = pool.Raw().ExecContext(context.Background(),
			`INSERT INTO user_devices (user_id, fcm_token, platform, is_active) VALUES ($1, $2, 'android', true)`, userID, "tok-t10")
		require.NoError(t, err)
		return userID
	}
	insertMember("Retry Uji")

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actID := insertActivity(t, pool, rtID, "Kegiatan Retry")
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(-time.Minute))

	mock := NewMockPushProvider()
	svc := NewService(NewRepository(pool), mock)
	repo := NewRepository(pool)
	list, err := repo.ListSchedules(context.Background(), rtID, "activity_reminder", nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	_, err = svc.ExecuteSchedule(context.Background(), list[0], time.Now())
	require.NoError(t, err)
	firstCount := len(mock.GetMessages())

	// Second call after the first must not re-send — schedule was auto-deactivated after first delivery.
	_, err = svc.ExecuteSchedule(context.Background(), list[0], time.Now())
	// Either error (inactive rejected by caller path) or no new messages — both acceptable and safe.
	require.Len(t, mock.GetMessages(), firstCount, "retry must not duplicate messages")
}

// TestTwoWorkersDoNotClaimSameReminder covers spec scenario #11.
func TestTwoWorkersDoNotClaimSameReminder(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T11")
	defer cleanupNotifTestRT(t, pool, rtID)

	insertMember := func(name string) string {
		var userID string
		err := pool.Raw().QueryRowContext(context.Background(),
			`INSERT INTO users (full_name, email, phone, password_hash, is_active)
			 VALUES ($1, $2, '081311113333', 'hash', true) RETURNING id`,
			name, fmt.Sprintf("t11-%s-%d@test.local", name, time.Now().UnixNano())).Scan(&userID)
		require.NoError(t, err)
		_, err = pool.Raw().ExecContext(context.Background(),
			`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, 'warga', true)`, userID, rtID)
		require.NoError(t, err)
		_, err = pool.Raw().ExecContext(context.Background(),
			`INSERT INTO user_devices (user_id, fcm_token, platform, is_active) VALUES ($1, $2, 'android', true)`, userID,
			fmt.Sprintf("tok-t11-%s", name))
		require.NoError(t, err)
		return userID
	}
	insertMember("Worker Uji")

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actID := insertActivity(t, pool, rtID, "Kegiatan Worker")
	createActivitySchedule(t, pool, rtID, templateID, "Reminder A", actID, time.Now().Add(-time.Minute))

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	repo := NewRepository(pool)
	list, err := repo.ListSchedules(context.Background(), rtID, "activity_reminder", nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	sched := list[0]

	var wg sync.WaitGroup
	run := func() {
		defer wg.Done()
		_, _ = svc.ExecuteSchedule(context.Background(), sched, time.Now())
	}

	wg.Add(2)
	go run()
	go run()
	wg.Wait()

	// Exactly one execution row must exist per (schedule, exec_date, config_hash) (unique constraint).
	var count int
	err = pool.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM notification_schedule_executions WHERE schedule_id = $1", sched.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "only one of the two workers can claim the same execution")
}

// TestSchedulerSkipsTerminatedActivityReminder covers spec scenario #12.
func TestSchedulerSkipsTerminatedActivityReminder(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T12")
	defer cleanupNotifTestRT(t, pool, rtID)

	templateID := insertActivityTemplate(t, pool, rtID, EventActivityReminder,
		"Reminder Test", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actCancelled := insertActivity(t, pool, rtID, "Kegiatan Cancelled",
		map[string]string{"status": "'cancelled'"})
	createActivitySchedule(t, pool, rtID, templateID, "Reminder Cancelled", actCancelled, time.Now().Add(time.Hour))

	actCompleted := insertActivity(t, pool, rtID, "Kegiatan Completed",
		map[string]string{"ends_at": "NOW() - INTERVAL '1 hour'"})
	createActivitySchedule(t, pool, rtID, templateID, "Reminder Completed", actCompleted, time.Now().Add(time.Hour))

	mock := NewMockPushProvider()
	svc := NewService(NewRepository(pool), mock)
	repo := NewRepository(pool)
	list, err := repo.ListSchedules(context.Background(), rtID, "activity_reminder", nil)
	require.NoError(t, err)

	for _, s := range list {
		before := len(mock.GetMessages())
		_, _ = svc.ExecuteSchedule(context.Background(), s, time.Now().Add(time.Hour))
		// Must not send — zero new messages after this execution
		after := len(mock.GetMessages())
		require.Equal(t, before, after, "deliveries for terminated activity must not happen")
	}
}

// TestCrossRTIsolation covers spec scenario #13.
func TestCrossRTIsolation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtA := setupNotifTestRT(t, pool, "TRA")
	rtB := setupNotifTestRT(t, pool, "TRB")
	defer cleanupNotifTestRT(t, pool, rtA)
	defer cleanupNotifTestRT(t, pool, rtB)

	templateID := insertActivityTemplate(t, pool, rtA, EventActivityReminder,
		"Reminder A", "Pengingat Kegiatan",
		"Kegiatan {{nama_kegiatan}} akan dilaksanakan pada {{tanggal_kegiatan}}")

	actID := insertActivity(t, pool, rtA, "Kegiatan A")
	createActivitySchedule(t, pool, rtA, templateID, "Reminder A", actID, time.Now().Add(24*time.Hour))

	repo := NewRepository(pool)
	listB, err := repo.ListSchedules(context.Background(), rtB, "activity_reminder", nil)
	require.NoError(t, err)
	require.Empty(t, listB, "RT B must not see RT A's schedules")

	// RT isolation for templates — RT B cannot see RT A's template row.
	tmplB, err := repo.GetActiveTemplateForEvent(context.Background(), rtB, EventActivityReminder)
	if tmplB != nil {
		require.NotEqual(t, rtA, tmplB.RTID, "RT B lookup must never return RT A's template")
	}
	_ = err

}

// TestActivityEventTemplateValidationRejectsUnknownVariables covers spec scenario #14.
func TestActivityEventTemplateValidationRejectsUnknownVariables(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "T14")
	defer cleanupNotifTestRT(t, pool, rtID)

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	_, err := svc.CreateTemplate(context.Background(), rtID, CreateTemplateInput{
		Name:      "Invalid Variable Template",
		EventType: EventActivityReminder,
		Channel:   "push",
		Title:     "{{nama_kegiatan}}",
		Body:      "Halo {{not_a_known_variable}} there",
	})
	require.Error(t, err, "template with unknown variable must be rejected")

	// By System trigger events (activity_created) are permanent system events:
	// pengurus cannot create custom templates for them even with valid variables.
	_, err = svc.CreateTemplate(context.Background(), rtID, CreateTemplateInput{
		Name:      "Custom Forbidden",
		EventType: EventActivityCreated,
		Channel:   "push",
		Title:     "{{nama_kegiatan}}",
		Body:      "Kegiatan {{nama_kegiatan}}",
	})
	require.Error(t, err, "by_system events cannot be customized via custom template creation")

	// A valid template with allowed activity reminder variables should succeed.
	created, err := svc.CreateTemplate(context.Background(), rtID, CreateTemplateInput{
		Name:      "Valid Activity Template",
		EventType: EventActivityReminder,
		Channel:   "push",
		Title:     "{{nama_kegiatan}}",
		Body:      "Kegiatan {{nama_kegiatan}} pada {{tanggal_kegiatan}} di {{lokasi_kegiatan}}",
	})
	require.NoError(t, err)
	_ = created
}
