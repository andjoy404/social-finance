package notification_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"social-finance/internal/notification"
)

// TestScheduleConfigHash_DimensionsAndInvariance verifies that:
//  1. Every functional dimension (send_hour, send_minute, send_days, timezone, event_type, template_id, rt_id)
//     changes the configuration hash.
//  2. Metadata changes (name, description, is_active, updated_at, last_executed_at) DO NOT change the hash.
//  3. send_days sorting and deduplication produces invariant hashes.
func TestScheduleConfigHash_DimensionsAndInvariance(t *testing.T) {
	base := notification.NotificationSchedule{
		ID:         "3ecb113a-2580-40e1-b3dd-9159fd5cc9fe",
		RTID:       "a1111111-1111-1111-1111-111111111111",
		EventType:  notification.EventPaymentReminder,
		TemplateID: "b1111111-1111-1111-1111-111111111111",
		Name:       "Jadwal Utama",
		IsActive:   true,
		SendDays:   []int{7},
		SendHour:   15,
		SendMinute: 50,
		Timezone:   "Asia/Jakarta",
	}

	baseHash := notification.ComputeScheduleConfigHashFromSchedule(&base)
	if baseHash == "" {
		t.Fatal("expected non-empty baseHash")
	}

	// 1. Metadata changes -> MUST NOT CHANGE HASH
	metaChanged := base
	metaChanged.Name = "Nama Baru Berubah"
	now := time.Now()
	metaChanged.LastExecutedAt = &now
	status := "success"
	metaChanged.LastExecutedStatus = &status
	metaChanged.IsActive = false
	if h := notification.ComputeScheduleConfigHashFromSchedule(&metaChanged); h != baseHash {
		t.Errorf("metadata-only change changed config hash: got %s, want %s", h, baseHash)
	}

	// 2. send_days order and deduplication -> MUST NOT CHANGE HASH
	order1 := base
	order1.SendDays = []int{5, 15, 25}
	order2 := base
	order2.SendDays = []int{25, 5, 15, 5}
	if h1, h2 := notification.ComputeScheduleConfigHashFromSchedule(&order1), notification.ComputeScheduleConfigHashFromSchedule(&order2); h1 != h2 {
		t.Errorf("send_days sort/dedup invariance failed: %s != %s", h1, h2)
	}

	// 3. Functional dimensions -> MUST CHANGE HASH
	tests := []struct {
		name   string
		mutate func(s *notification.NotificationSchedule)
	}{
		{"send_hour", func(s *notification.NotificationSchedule) { s.SendHour = 16 }},
		{"send_minute", func(s *notification.NotificationSchedule) { s.SendMinute = 51 }},
		{"send_days", func(s *notification.NotificationSchedule) { s.SendDays = []int{8} }},
		{"timezone", func(s *notification.NotificationSchedule) { s.Timezone = "Asia/Makassar" }},
		{"event_type", func(s *notification.NotificationSchedule) { s.EventType = notification.EventOverdue3Months }},
		{"template_id", func(s *notification.NotificationSchedule) { s.TemplateID = "b2222222-2222-2222-2222-222222222222" }},
		{"rt_id", func(s *notification.NotificationSchedule) { s.RTID = "a2222222-2222-2222-2222-222222222222" }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mut := base
			tc.mutate(&mut)
			h := notification.ComputeScheduleConfigHashFromSchedule(&mut)
			if h == baseHash {
				t.Errorf("mutation %s did not change config hash (%s)", tc.name, h)
			}
		})
	}
}

// TestScenarioF_CalendarMonthEndAndEdgeCases tests days 29, 30, 31 edge cases
// and timezone boundary evaluation.
func TestScenarioF_CalendarMonthEndAndEdgeCases(t *testing.T) {
	locJakarta, _ := time.LoadLocation("Asia/Jakarta")

	// 1. February in non-leap year (e.g. 2025, 28 days)
	// A schedule set to run on day 31 must run on Feb 28.
	feb28 := time.Date(2025, 2, 28, 10, 0, 0, 0, locJakarta)
	if !notification.IsDayDueForTesting([]int{31}, feb28) {
		t.Errorf("day 31 should be due on Feb 28 in non-leap year")
	}

	// But on Feb 27, day 31 must NOT be due.
	feb27 := time.Date(2025, 2, 27, 10, 0, 0, 0, locJakarta)
	if notification.IsDayDueForTesting([]int{31}, feb27) {
		t.Errorf("day 31 should NOT be due on Feb 27")
	}

	// 2. April (30 days): day 31 must run on April 30.
	apr30 := time.Date(2026, 4, 30, 10, 0, 0, 0, locJakarta)
	if !notification.IsDayDueForTesting([]int{31}, apr30) {
		t.Errorf("day 31 should be due on April 30")
	}

	// 3. Timezone boundary test:
	// 2026-11-07 23:30 UTC = 2026-11-08 06:30 Asia/Jakarta (+7).
	// A schedule set for day 8 at 06:30 Asia/Jakarta must match nowUTC.
	nowUTC := time.Date(2026, 11, 7, 23, 30, 0, 0, time.UTC)
	localJakarta := nowUTC.In(locJakarta)
	if localJakarta.Day() != 8 || localJakarta.Hour() != 6 || localJakarta.Minute() != 30 {
		t.Fatalf("unexpected local conversion: %v", localJakarta)
	}
	if !notification.IsDayDueForTesting([]int{8}, localJakarta) {
		t.Errorf("day 8 should be due when nowUTC is converted to Asia/Jakarta")
	}
}

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }
func ptrDays(d []int) *[]int  { return &d }

// TestScenarioA_SameDayConfigChangeAndReExecution simulates the exact target case:
// - Date: 7 November 2026 (Asia/Jakarta)
// - Schedule ID: 3ecb113a-2580-40e1-b3dd-9159fd5cc9fe
// - Config A: 08:05 WIB executes successfully at 08:05.
// - At 15:48 WIB, pengurus updates schedule to 15:50 WIB (Config B).
// - At 15:49 WIB, scheduler checks: not due yet.
// - At 15:50 WIB, scheduler checks: due! Config B executes successfully.
// - At 15:51 WIB, scheduler checks: not due (already executed for Config B today).
func TestScenarioA_SameDayConfigChangeAndReExecution(t *testing.T) {
	pool := notifTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	adminUserID := notifTestUser(t, pool, "Admin A", true)
	schedID := "3ecb113a-2580-40e1-b3dd-9159fd5cc9fe"

	// Clean up any existing data with this schedule ID from previous test runs
	_, _ = db.ExecContext(ctx, "DELETE FROM notification_schedule_executions WHERE schedule_id = $1", schedID)
	_, _ = db.ExecContext(ctx, "DELETE FROM notification_logs WHERE dedup_key LIKE $1", "schedule:"+schedID+":%")
	_, _ = db.ExecContext(ctx, "DELETE FROM notification_schedules WHERE id = $1", schedID)

	// Create active template
	var tmplID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rt_id, event_type, name, title, body, is_active, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', 'Template Reminder', 'Tagihan Iuran', 'Harap segera membayar tagihan', true, NOW(), NOW())
		RETURNING id
	`, rtID).Scan(&tmplID)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	// Create schedule with Config A: 08:05 WIB
	_, err = db.ExecContext(ctx, `
		INSERT INTO notification_schedules (id, rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, created_at, updated_at)
		VALUES ($1, $2, 'iuran_payment_reminder', $3, 'Pengingat Pembayaran Iuran', true, '[7]'::jsonb, 8, 5, 'Asia/Jakarta', NOW(), NOW())
	`, schedID, rtID, tmplID)
	if err != nil {
		t.Fatalf("create schedule Config A: %v", err)
	}

	locJakarta, _ := time.LoadLocation("Asia/Jakarta")
	time0805 := time.Date(2026, 11, 7, 8, 5, 0, 0, locJakarta)

	repo := notification.NewRepository(pool)
	mockProvider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, mockProvider)

	// 1. At 08:05, schedule is due for Config A
	due, err := repo.GetDueSchedules(ctx, time0805)
	if err != nil {
		t.Fatalf("GetDueSchedules at 08:05: %v", err)
	}
	var found *notification.NotificationSchedule
	for _, s := range due {
		if s.ID == schedID {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatalf("expected schedule %s to be due at 08:05 Config A", schedID)
	}

	// Execute Config A
	_, err = svc.ExecuteSchedule(ctx, found, time0805)
	if err != nil {
		t.Fatalf("ExecuteSchedule Config A: %v", err)
	}

	// Verify Config A execution is recorded
	hashA := notification.ComputeScheduleConfigHashFromSchedule(found)
	doneA, err := repo.IsScheduleExecutionDone(ctx, schedID, "2026-11-07", hashA)
	if err != nil || !doneA {
		t.Fatalf("expected Config A execution to be marked done: done=%v, err=%v", doneA, err)
	}

	// 2. Poll at 08:06 WIB -> NOT due
	time0806 := time.Date(2026, 11, 7, 8, 6, 0, 0, locJakarta)
	due0806, err := repo.GetDueSchedules(ctx, time0806)
	if err != nil {
		t.Fatalf("GetDueSchedules at 08:06: %v", err)
	}
	for _, s := range due0806 {
		if s.ID == schedID {
			t.Fatalf("schedule %s should NOT be due at 08:06 without config change", schedID)
		}
	}

	// 3. At 15:48 WIB, pengurus updates schedule to 15:50 (Config B)
	updateInput := notification.UpdateScheduleInput{
		Name:       ptrStr("Pengingat Pembayaran Iuran (Diubah)"),
		SendDays:   ptrDays([]int{7}),
		SendHour:   ptrInt(15),
		SendMinute: ptrInt(50),
		Timezone:   ptrStr("Asia/Jakarta"),
	}
	updatedSchedule, err := svc.UpdateSchedule(ctx, rtID, schedID, adminUserID, updateInput)
	if err != nil {
		t.Fatalf("UpdateSchedule to Config B: %v", err)
	}

	hashB := notification.ComputeScheduleConfigHashFromSchedule(updatedSchedule)
	if hashB == hashA {
		t.Fatalf("expected Config B hash to differ from Config A: %s == %s", hashB, hashA)
	}

	// 4. At 15:49 WIB, scheduler checks -> NOT due yet (minute 49 < 50)
	time1549 := time.Date(2026, 11, 7, 15, 49, 0, 0, locJakarta)
	due1549, err := repo.GetDueSchedules(ctx, time1549)
	if err != nil {
		t.Fatalf("GetDueSchedules at 15:49: %v", err)
	}
	for _, s := range due1549 {
		if s.ID == schedID {
			t.Fatalf("schedule %s should NOT be due at 15:49 (scheduled for 15:50)", schedID)
		}
	}

	// 5. At 15:50 WIB, scheduler checks -> MUST BE DUE!
	time1550 := time.Date(2026, 11, 7, 15, 50, 0, 0, locJakarta)
	due1550, err := repo.GetDueSchedules(ctx, time1550)
	if err != nil {
		t.Fatalf("GetDueSchedules at 15:50: %v", err)
	}
	var foundB *notification.NotificationSchedule
	for _, s := range due1550 {
		if s.ID == schedID {
			foundB = s
			break
		}
	}
	if foundB == nil {
		t.Fatalf("CRITICAL BUG: Schedule %s was NOT due at 15:50 on same day after config update!", schedID)
	}

	// Execute Config B
	_, err = svc.ExecuteSchedule(ctx, foundB, time1550)
	if err != nil {
		t.Fatalf("ExecuteSchedule Config B: %v", err)
	}

	// Verify Config B execution is recorded
	doneB, err := repo.IsScheduleExecutionDone(ctx, schedID, "2026-11-07", hashB)
	if err != nil || !doneB {
		t.Fatalf("expected Config B execution to be marked done: done=%v, err=%v", doneB, err)
	}

	// 6. At 15:51 WIB, scheduler checks -> NOT due (already executed for Config B)
	time1551 := time.Date(2026, 11, 7, 15, 51, 0, 0, locJakarta)
	due1551, err := repo.GetDueSchedules(ctx, time1551)
	if err != nil {
		t.Fatalf("GetDueSchedules at 15:51: %v", err)
	}
	for _, s := range due1551 {
		if s.ID == schedID {
			t.Fatalf("schedule %s should NOT be due at 15:51 after executing Config B", schedID)
		}
	}
}

// TestScenarioB_NoConfigChange_NeverReExecutes verifies that when a schedule has executed,
// subsequent scheduler ticks throughout the same day do not re-execute it.
func TestScenarioB_NoConfigChange_NeverReExecutes(t *testing.T) {
	pool := notifTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	rtID := notifTestRT(t, pool)

	var tmplID, schedID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rt_id, event_type, name, title, body, is_active, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', 'Template B', 'Tagihan', 'Isi', true, NOW(), NOW())
		RETURNING id
	`, rtID).Scan(&tmplID)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO notification_schedules (rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', $2, 'Jadwal B', true, '[7]'::jsonb, 8, 5, 'Asia/Jakarta', NOW(), NOW())
		RETURNING id
	`, rtID, tmplID).Scan(&schedID)
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	locJakarta, _ := time.LoadLocation("Asia/Jakarta")
	time0805 := time.Date(2026, 11, 7, 8, 5, 0, 0, locJakarta)

	repo := notification.NewRepository(pool)
	svc := notification.NewService(repo, notification.NewMockPushProvider())

	due, err := repo.GetDueSchedules(ctx, time0805)
	if err != nil || len(due) == 0 {
		t.Fatalf("expected schedule to be due at 08:05: %v", err)
	}
	var target *notification.NotificationSchedule
	for _, s := range due {
		if s.ID == schedID {
			target = s
			break
		}
	}
	if target == nil {
		t.Fatalf("schedule %s not found in due list at 08:05", schedID)
	}

	_, err = svc.ExecuteSchedule(ctx, target, time0805)
	if err != nil {
		t.Fatalf("execute schedule: %v", err)
	}

	// Polls throughout the day
	testTimes := []time.Time{
		time.Date(2026, 11, 7, 8, 6, 0, 0, locJakarta),
		time.Date(2026, 11, 7, 9, 0, 0, 0, locJakarta),
		time.Date(2026, 11, 7, 12, 30, 0, 0, locJakarta),
		time.Date(2026, 11, 7, 15, 50, 0, 0, locJakarta),
		time.Date(2026, 11, 7, 23, 59, 0, 0, locJakarta),
	}

	for _, checkTime := range testTimes {
		dueLater, err := repo.GetDueSchedules(ctx, checkTime)
		if err != nil {
			t.Fatalf("GetDueSchedules at %v: %v", checkTime, err)
		}
		for _, s := range dueLater {
			if s.ID == schedID {
				t.Fatalf("schedule %s should never re-execute at %v without config change", schedID, checkTime)
			}
		}
	}
}

// TestScenarioC_MetadataOnlyChange_DoesNotReExecute verifies that modifying
// non-scheduling metadata (name, description) does NOT cause a same-day re-execution.
func TestScenarioC_MetadataOnlyChange_DoesNotReExecute(t *testing.T) {
	pool := notifTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	adminUserID := notifTestUser(t, pool, "Admin C", true)

	var tmplID, schedID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rt_id, event_type, name, title, body, is_active, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', 'Template C', 'Tagihan', 'Isi', true, NOW(), NOW())
		RETURNING id
	`, rtID).Scan(&tmplID)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO notification_schedules (rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', $2, 'Jadwal C Lama', true, '[7]'::jsonb, 8, 5, 'Asia/Jakarta', NOW(), NOW())
		RETURNING id
	`, rtID, tmplID).Scan(&schedID)
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	locJakarta, _ := time.LoadLocation("Asia/Jakarta")
	time0805 := time.Date(2026, 11, 7, 8, 5, 0, 0, locJakarta)

	repo := notification.NewRepository(pool)
	svc := notification.NewService(repo, notification.NewMockPushProvider())

	due, _ := repo.GetDueSchedules(ctx, time0805)
	var target *notification.NotificationSchedule
	for _, s := range due {
		if s.ID == schedID {
			target = s
			break
		}
	}
	if target == nil {
		t.Fatalf("schedule %s not found in due list at 08:05", schedID)
	}
	_, _ = svc.ExecuteSchedule(ctx, target, time0805)

	// Update metadata only: name changes, time and days remain identical
	_, err = svc.UpdateSchedule(ctx, rtID, schedID, adminUserID, notification.UpdateScheduleInput{
		Name:       ptrStr("Jadwal C Nama Baru Yang Berbeda"),
		SendDays:   ptrDays([]int{7}),
		SendHour:   ptrInt(8),
		SendMinute: ptrInt(5),
		Timezone:   ptrStr("Asia/Jakarta"),
	})
	if err != nil {
		t.Fatalf("update schedule metadata: %v", err)
	}

	// Poll at 10:00 -> MUST NOT BE DUE
	time1000 := time.Date(2026, 11, 7, 10, 0, 0, 0, locJakarta)
	due1000, err := repo.GetDueSchedules(ctx, time1000)
	if err != nil {
		t.Fatalf("GetDueSchedules at 10:00: %v", err)
	}
	for _, s := range due1000 {
		if s.ID == schedID {
			t.Fatalf("metadata-only change should not cause re-execution for schedule %s", schedID)
		}
	}
}

// TestScenarioE_DeactivationReactivation_DoesNotReExecute verifies that toggling
// a schedule inactive then active does not bypass the same-day execution check.
func TestScenarioE_DeactivationReactivation_DoesNotReExecute(t *testing.T) {
	pool := notifTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	adminUserID := notifTestUser(t, pool, "Admin E", true)

	var tmplID, schedID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rt_id, event_type, name, title, body, is_active, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', 'Template E', 'Tagihan', 'Isi', true, NOW(), NOW())
		RETURNING id
	`, rtID).Scan(&tmplID)
	if err != nil {
		t.Fatalf("create template E: %v", err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO notification_schedules (rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', $2, 'Jadwal E', true, '[7]'::jsonb, 8, 5, 'Asia/Jakarta', NOW(), NOW())
		RETURNING id
	`, rtID, tmplID).Scan(&schedID)
	if err != nil {
		t.Fatalf("create schedule E: %v", err)
	}

	locJakarta, _ := time.LoadLocation("Asia/Jakarta")
	time0805 := time.Date(2026, 11, 7, 8, 5, 0, 0, locJakarta)

	repo := notification.NewRepository(pool)
	svc := notification.NewService(repo, notification.NewMockPushProvider())

	due, _ := repo.GetDueSchedules(ctx, time0805)
	var target *notification.NotificationSchedule
	for _, s := range due {
		if s.ID == schedID {
			target = s
			break
		}
	}
	if target == nil {
		t.Fatalf("schedule %s not found in due list at 08:05", schedID)
	}
	_, _ = svc.ExecuteSchedule(ctx, target, time0805)

	// Deactivate
	_, err = svc.ToggleScheduleActive(ctx, rtID, schedID, adminUserID)
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	// Reactivate
	_, err = svc.ToggleScheduleActive(ctx, rtID, schedID, adminUserID)
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}

	// Poll again at 09:00 -> MUST NOT BE DUE
	time0900 := time.Date(2026, 11, 7, 9, 0, 0, 0, locJakarta)
	due0900, err := repo.GetDueSchedules(ctx, time0900)
	if err != nil {
		t.Fatalf("GetDueSchedules at 09:00: %v", err)
	}
	for _, s := range due0900 {
		if s.ID == schedID {
			t.Fatalf("deactivation-reactivation toggle should not trigger re-execution for schedule %s", schedID)
		}
	}
}

// TestScenarioG_AdvisoryLockAndUniqueConstraint verifies concurrency protection:
//  1. Simultaneous ExecuteSchedule calls for the same schedule and config version
//     do not double-send or create conflicting executions.
//  2. The PostgreSQL table notification_schedule_executions rejects duplicate
//     (schedule_id, exec_date, config_hash) inserts at the database level.
func TestScenarioG_AdvisoryLockAndUniqueConstraint(t *testing.T) {
	pool := notifTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	rtID := notifTestRT(t, pool)

	var tmplID, schedID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rt_id, event_type, name, title, body, is_active, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', 'Template G', 'Tagihan G', 'Isi', true, NOW(), NOW())
		RETURNING id
	`, rtID).Scan(&tmplID)
	if err != nil {
		t.Fatalf("create template G: %v", err)
	}

	err = db.QueryRowContext(ctx, `
		INSERT INTO notification_schedules (rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', $2, 'Jadwal G', true, '[7]'::jsonb, 10, 0, 'Asia/Jakarta', NOW(), NOW())
		RETURNING id
	`, rtID, tmplID).Scan(&schedID)
	if err != nil {
		t.Fatalf("create schedule G: %v", err)
	}

	locJakarta, _ := time.LoadLocation("Asia/Jakarta")
	execTime := time.Date(2026, 11, 7, 10, 0, 0, 0, locJakarta)

	repo := notification.NewRepository(pool)
	svc := notification.NewService(repo, notification.NewMockPushProvider())

	schedule, err := repo.GetSchedule(ctx, rtID, schedID)
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}

	// 1. Database-level UNIQUE constraint test on notification_schedule_executions
	hash := notification.ComputeScheduleConfigHashFromSchedule(schedule)
	err1 := repo.RecordScheduleExecution(ctx, schedID, rtID, execTime, hash, "success", nil, nil)
	if err1 != nil {
		t.Fatalf("first RecordScheduleExecution failed: %v", err1)
	}

	// Direct INSERT with duplicate (schedule_id, exec_date, config_hash) must violate uq_schedule_execution
	_, err2 := db.ExecContext(ctx, `
		INSERT INTO notification_schedule_executions (schedule_id, rt_id, exec_date, config_hash, status)
		VALUES ($1, $2, $3::date, $4, 'success')
	`, schedID, rtID, "2026-11-07", hash)
	if err2 == nil {
		t.Fatal("expected unique constraint violation on duplicate execution insert, got nil")
	}

	// 2. Concurrency test: 5 concurrent workers trying to ExecuteSchedule
	var wg sync.WaitGroup
	workers := 5
	results := make([]error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, results[idx] = svc.ExecuteSchedule(ctx, schedule, execTime)
		}(i)
	}
	wg.Wait()

	for i, rErr := range results {
		if rErr != nil {
			t.Errorf("worker %d returned error: %v", i, rErr)
		}
	}

	// Verify only 1 execution row exists in notification_schedule_executions for this config
	var count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_schedule_executions
		WHERE schedule_id = $1 AND exec_date = '2026-11-07' AND config_hash = $2
	`, schedID, hash).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected exactly 1 execution record, got %d (err: %v)", count, err)
	}
}

// TestScenarioH_HistoryDistinctRowsAndRecipientDetail verifies that:
// 1. Two different config executions on the same day appear as TWO distinct rows in ListSchedulerHistory.
// 2. Each row contains its respective config_hash.
// 3. GetSchedulerRecipientDetail filtered by config_hash returns only the recipients for that execution.
func TestScenarioH_HistoryDistinctRowsAndRecipientDetail(t *testing.T) {
	pool := notifTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	rtID := notifTestRT(t, pool)

	var tmplID, schedID string
	_ = db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rt_id, event_type, name, title, body, is_active, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', 'Template H', 'Tagihan H', 'Isi H', true, NOW(), NOW())
		RETURNING id
	`, rtID).Scan(&tmplID)

	_ = db.QueryRowContext(ctx, `
		INSERT INTO notification_schedules (rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, created_at, updated_at)
		VALUES ($1, 'iuran_payment_reminder', $2, 'Jadwal H', true, '[7]'::jsonb, 8, 5, 'Asia/Jakarta', NOW(), NOW())
		RETURNING id
	`, rtID, tmplID).Scan(&schedID)

	locJakarta, _ := time.LoadLocation("Asia/Jakarta")
	dateJakarta := "2026-11-07"
	configHashA := "hash_version_a_12345"
	configHashB := "hash_version_b_67890"

	repo := notification.NewRepository(pool)

	user1 := notifTestUser(t, pool, "User H1", true)
	user2 := notifTestUser(t, pool, "User H2", true)
	user3 := notifTestUser(t, pool, "User H3", true)

	dedupA1 := notification.BuildScheduleDedupKey(schedID, dateJakarta, user1, "", "", "", configHashA)
	dedupA2 := notification.BuildScheduleDedupKey(schedID, dateJakarta, user2, "", "", "", configHashA)
	_, err := repo.RecordScheduleDelivery(ctx, rtID, user1, notification.EventPaymentReminder, dedupA1, "Title A", "Body A", nil, "SENT", "")
	if err != nil {
		t.Fatalf("record delivery A1: %v", err)
	}
	_, err = repo.RecordScheduleDelivery(ctx, rtID, user2, notification.EventPaymentReminder, dedupA2, "Title A", "Body A", nil, "SENT", "")
	if err != nil {
		t.Fatalf("record delivery A2: %v", err)
	}

	// Insert execution logs for Config B (1 recipient: user-3)
	dedupB1 := notification.BuildScheduleDedupKey(schedID, dateJakarta, user3, "", "", "", configHashB)
	_, err = repo.RecordScheduleDelivery(ctx, rtID, user3, notification.EventPaymentReminder, dedupB1, "Title B", "Body B", nil, "SENT", "")
	if err != nil {
		t.Fatalf("record delivery B1: %v", err)
	}

	// Record execution metadata in notification_schedule_executions
	time0805 := time.Date(2026, 11, 7, 8, 5, 0, 0, locJakarta)
	time1550 := time.Date(2026, 11, 7, 15, 50, 0, 0, locJakarta)
	_ = repo.RecordScheduleExecution(ctx, schedID, rtID, time0805, configHashA, "success", nil, &notification.ScheduleExecutionSummary{
		TotalRecipients: 2, SentCount: 2, FailedCount: 0,
	})
	_ = repo.RecordScheduleExecution(ctx, schedID, rtID, time1550, configHashB, "success", nil, &notification.ScheduleExecutionSummary{
		TotalRecipients: 1, SentCount: 1, FailedCount: 0,
	})

	// 1. Verify ListSchedulerHistory produces TWO distinct entries
	history, total, err := repo.ListSchedulerHistory(ctx, rtID, 1, 20)
	if err != nil {
		t.Fatalf("ListSchedulerHistory: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected at least 2 history entries for same day different configs, got %d", total)
	}

	var entryA, entryB *notification.NotificationSchedulerHistory
	for _, h := range history {
		if h.ScheduleID == schedID {
			if h.ConfigHash == configHashA {
				entryA = h
			} else if h.ConfigHash == configHashB {
				entryB = h
			}
		}
	}

	if entryA == nil || entryB == nil {
		t.Fatalf("expected distinct entries for configHashA and configHashB: entryA=%v, entryB=%v", entryA, entryB)
	}
	if entryA.TotalRecipients != 2 || entryA.SentCount != 2 {
		t.Errorf("entryA counts wrong: got total=%d, sent=%d, want 2/2", entryA.TotalRecipients, entryA.SentCount)
	}
	if entryB.TotalRecipients != 1 || entryB.SentCount != 1 {
		t.Errorf("entryB counts wrong: got total=%d, sent=%d, want 1/1", entryB.TotalRecipients, entryB.SentCount)
	}

	// 2. Verify GetSchedulerRecipientDetail with config_hash returns ONLY that version's recipients
	recipientsA, err := repo.GetSchedulerRecipientDetail(ctx, rtID, schedID, dateJakarta, configHashA)
	if err != nil {
		t.Fatalf("GetSchedulerRecipientDetail A: %v", err)
	}
	if len(recipientsA) != 2 {
		t.Fatalf("expected 2 recipients for config A, got %d", len(recipientsA))
	}
	for _, r := range recipientsA {
		if r.UserID != user1 && r.UserID != user2 {
			t.Errorf("unexpected user in config A detail: %s", r.UserID)
		}
	}

	recipientsB, err := repo.GetSchedulerRecipientDetail(ctx, rtID, schedID, dateJakarta, configHashB)
	if err != nil {
		t.Fatalf("GetSchedulerRecipientDetail B: %v", err)
	}
	if len(recipientsB) != 1 {
		t.Fatalf("expected 1 recipient for config B, got %d", len(recipientsB))
	}
	if recipientsB[0].UserID != user3 {
		t.Errorf("unexpected user in config B detail: %s (want %s)", recipientsB[0].UserID, user3)
	}
}
