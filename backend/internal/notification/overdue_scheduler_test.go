package notification

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	"social-finance/internal/testutil"
)

var notifTestCounter int64 = time.Now().UnixNano() % 100000

func TestSystemTypesAndDedupKeyFormat(t *testing.T) {
	var foundUpcoming, foundOverdue bool
	for _, st := range SystemNotificationTypes {
		switch st.EventType {
		case EventUpcomingDue3Months:
			foundUpcoming = true
			require.True(t, st.IsConfigurable)
			require.Contains(t, st.AllowedVariables, "nama")
			require.Contains(t, st.AllowedVariables, "total_outstanding")
			require.Contains(t, st.AllowedVariables, "bill_count")
		case EventOverdue3Months:
			foundOverdue = true
			require.True(t, st.IsConfigurable)
			require.Contains(t, st.AllowedVariables, "nama")
			require.Contains(t, st.AllowedVariables, "total_outstanding")
			require.Contains(t, st.AllowedVariables, "bill_count")
		}
	}
	require.True(t, foundUpcoming, "EventUpcomingDue3Months must be registered in SystemNotificationTypes")
	require.True(t, foundOverdue, "EventOverdue3Months must be registered in SystemNotificationTypes")

	// Verify dedup_key format preserves schedule:<schedule_id>:<exec_date>:<user_id> prefix
	key := BuildScheduleDedupKey("sched-1", "2026-10-15", "user-1", EventOverdue3Months, "2026-10", "bill-a+bill-b")
	require.True(t, strings.HasPrefix(key, "schedule:sched-1:2026-10-15:user-1:"))
	require.Equal(t, "schedule:sched-1:2026-10-15:user-1:iuran_overdue_3_months:2026-10:bill-a+bill-b", key)
}

func setupNotifTestRT(t *testing.T, pool *database.Pool, suffix string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&notifTestCounter, 1)
	var rtID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ($1, 1, $2, true) RETURNING id`,
		"RT Notif "+suffix, fmt.Sprintf("N%s-%d", suffix, n),
	).Scan(&rtID)
	require.NoError(t, err)
	return rtID
}

func cleanupNotifTestRT(t *testing.T, pool *database.Pool, rtID string) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM notification_logs WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM notification_schedules WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM notification_templates WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM payments WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM bills WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM dues WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM financial_categories WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM residency_periods WHERE household_occupancy_id IN (SELECT id FROM household_occupancies WHERE household_id IN (SELECT id FROM households WHERE rt_id = $1))`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM household_occupancies WHERE household_id IN (SELECT id FROM households WHERE rt_id = $1)`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM physical_houses WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM households WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
}

func TestOverdueAndUpcomingSchedulerEndToEnd(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	db := pool.Raw()

	rtA := setupNotifTestRT(t, pool, "A")
	rtB := setupNotifTestRT(t, pool, "B")
	defer cleanupNotifTestRT(t, pool, rtA)
	defer cleanupNotifTestRT(t, pool, rtB)

	n := atomic.AddInt64(&notifTestCounter, 1)

	createResidentUser := func(rtID, name, suffix, fcmToken string) (userID, occID string) {
		phone := fmt.Sprintf("0813%04d%s", n%10000, suffix)
		var houseID, resID, hhID string
		err := db.QueryRowContext(ctx,
			`INSERT INTO physical_houses (rt_id, house_number, address) VALUES ($1, $2, 'Jl Uji') RETURNING id`,
			rtID, fmt.Sprintf("HN-%s-%d", suffix, n),
		).Scan(&houseID)
		require.NoError(t, err)

		err = db.QueryRowContext(ctx,
			`INSERT INTO households (rt_id, head_name) VALUES ($1, $2) RETURNING id`,
			rtID, name,
		).Scan(&hhID)
		require.NoError(t, err)

		err = db.QueryRowContext(ctx,
			`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
			 VALUES ($1, $2, 'OWNER', '2025-01-01'::date) RETURNING id`,
			hhID, houseID,
		).Scan(&occID)
		require.NoError(t, err)

		err = db.QueryRowContext(ctx,
			`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
			rtID, name, phone,
		).Scan(&resID)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx,
			`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
			 VALUES ($1, $2, '2025-01-01'::date)`,
			resID, occID,
		)
		require.NoError(t, err)

		err = db.QueryRowContext(ctx,
			`INSERT INTO users (full_name, email, phone, password_hash, is_active)
			 VALUES ($1, $2, $3, 'hash', true) RETURNING id`,
			name, fmt.Sprintf("notif-%s-%d@test.local", suffix, n), phone,
		).Scan(&userID)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx,
			`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active) VALUES ($1, $2, 'warga', true)`,
			userID, rtID,
		)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx,
			`INSERT INTO user_devices (user_id, fcm_token, platform, is_active) VALUES ($1, $2, 'android', true)`,
			userID, fcmToken,
		)
		require.NoError(t, err)

		return userID, occID
	}

	userA1, occA1 := createResidentUser(rtA, "Budi Santoso", "11", fmt.Sprintf("tok-a1-%d", n))
	userA2, occA2 := createResidentUser(rtA, "Siti Aminah", "22", fmt.Sprintf("tok-a2-%d", n))
	userB1, occB1 := createResidentUser(rtB, "Andi Wijaya", "33", fmt.Sprintf("tok-b1-%d", n))

	defer func() {
		for _, uid := range []string{userA1, userA2, userB1} {
			_, _ = db.ExecContext(ctx, `DELETE FROM user_devices WHERE user_id = $1`, uid)
			_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, uid)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, uid)
		}
	}()

	createDue := func(rtID string) string {
		var dueID string
		err := db.QueryRowContext(ctx,
			`INSERT INTO dues (rt_id, name, amount, period_type, is_active, is_voluntary)
			 VALUES ($1, 'Iuran Bulanan', 150000.00, 'monthly', true, false) RETURNING id`,
			rtID,
		).Scan(&dueID)
		require.NoError(t, err)
		return dueID
	}

	dueA := createDue(rtA)
	dueB := createDue(rtB)
	// Clear any bills auto-generated by the dues trigger so test assertions are deterministic
	_, err := db.ExecContext(ctx, `DELETE FROM bills WHERE rt_id IN ($1, $2)`, rtA, rtB)
	require.NoError(t, err)

	// Evaluation date: 2026-10-15 08:00 WIB
	evalOct15 := time.Date(2026, 10, 15, 8, 0, 0, 0, finance.JakartaLocation())

	// 1. User A1 has TWO bills overdue >= 3 months:
	//    - Bill 1: due 2026-06-15 (4 months overdue), Rp 150.000, partial paid Rp 50.000 -> remaining Rp 100.000
	//    - Bill 2: due 2026-07-15 (exact 3 months overdue), Rp 150.000 unpaid -> remaining Rp 150.000
	//    - Bill 3: due 2026-08-15 (2 months overdue -> NOT >= 3 months, must be ignored)
	var billA1Jun, billA1Jul string
	err = db.QueryRowContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2026-06', 150000.00, 'partial', '2026-06-15'::date, true, true) RETURNING id`,
		rtA, occA1, dueA,
	).Scan(&billA1Jun)
	require.NoError(t, err)

	var payID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, submitted_by)
		 VALUES ($1, $2, 50000.00, 'TRANSFER', 'SELF_SUBMITTED', 'APPROVED', $3) RETURNING id`,
		rtA, billA1Jun, userA1,
	).Scan(&payID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO payment_allocations (payment_id, bill_id, amount) VALUES ($1, $2, 50000.00)`,
		payID, billA1Jun,
	)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2026-07', 150000.00, 'unpaid', '2026-07-15'::date, true, true) RETURNING id`,
		rtA, occA1, dueA,
	).Scan(&billA1Jul)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2026-08', 150000.00, 'unpaid', '2026-08-15'::date, true, true)`,
		rtA, occA1, dueA,
	)
	require.NoError(t, err)

	// 2. User A2 has ONE bill upcoming in 3 months (due 2027-01-15 -> trigger date is 2026-10-15)
	var billA2Jan string
	err = db.QueryRowContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2027-01', 150000.00, 'unpaid', '2027-01-15'::date, true, true) RETURNING id`,
		rtA, occA2, dueA,
	).Scan(&billA2Jan)
	require.NoError(t, err)

	// 3. User B1 (in RT B) has an overdue >= 3 months bill (must NOT receive notifications from RT A's schedule)
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2026-06', 150000.00, 'unpaid', '2026-06-15'::date, true, true)`,
		rtB, occB1, dueB,
	)
	require.NoError(t, err)

	repo := NewRepository(pool)
	mockPush := NewMockPushProvider()
	svc := NewService(repo, mockPush)

	// Create templates & schedules for RT A
	tmplUpcoming, err := svc.CreateTemplate(ctx, rtA, CreateTemplateInput{
		Name:      "Pengingat 3 Bulan Sebelum Jatuh Tempo",
		EventType: EventUpcomingDue3Months,
		Title:     "Pengingat Pembayaran Iuran",
		Body:      "Halo {{nama}}, kami mengingatkan bahwa tagihan iuran Anda ({{period}}) akan jatuh tempo dalam waktu 3 bulan.",
		Channel:   "push",
	})
	require.NoError(t, err)

	tmplOverdue, err := svc.CreateTemplate(ctx, rtA, CreateTemplateInput{
		Name:      "Pengingat Tunggakan 3 Bulan+",
		EventType: EventOverdue3Months,
		Title:     "Pengingat Tunggakan Iuran",
		Body:      "Halo {{nama}}, terdapat {{bill_count}} tagihan iuran Anda yang telah menunggak selama 3 bulan atau lebih dengan total {{total_outstanding}}.",
		Channel:   "push",
	})
	require.NoError(t, err)

	schedUpcoming, err := svc.CreateSchedule(ctx, rtA, userA1, CreateScheduleInput{
		EventType:  EventUpcomingDue3Months,
		TemplateID: tmplUpcoming.ID,
		Name:       "Jadwal Pengingat H-3 Bulan",
		SendDays:   []int{15},
		SendHour:   8,
		SendMinute: 0,
		Timezone:   "Asia/Jakarta",
	})
	require.NoError(t, err)

	schedOverdue, err := svc.CreateSchedule(ctx, rtA, userA1, CreateScheduleInput{
		EventType:  EventOverdue3Months,
		TemplateID: tmplOverdue.ID,
		Name:       "Jadwal Pengingat Tunggakan 3 Bulan+",
		SendDays:   []int{15},
		SendHour:   8,
		SendMinute: 0,
		Timezone:   "Asia/Jakarta",
	})
	require.NoError(t, err)

	// ── Execution 1: Upcoming Due 3 Months on 2026-10-15 ──
	sentUpc, err := svc.ExecuteSchedule(ctx, schedUpcoming, evalOct15)
	require.NoError(t, err)
	require.Equal(t, 1, sentUpc, "Siti Aminah (userA2) should receive 1 upcoming reminder")

	msgs := mockPush.GetMessages()
	require.Len(t, msgs, 1)
	require.Equal(t, "Pengingat Pembayaran Iuran", msgs[0].Title)
	require.Contains(t, msgs[0].Body, "Halo Siti Aminah")
	require.Contains(t, msgs[0].Body, "2027-01")
	require.Equal(t, "/home/iuran/detail/"+billA2Jan, msgs[0].Data["deep_link"])

	// Re-running upcoming schedule on next month (2026-11-15) must NOT resend for the same bill!
	mockPush.Clear()
	evalNov15 := time.Date(2026, 11, 15, 8, 0, 0, 0, finance.JakartaLocation())
	sentUpcRepeat, err := svc.ExecuteSchedule(ctx, schedUpcoming, evalNov15)
	require.NoError(t, err)
	require.Equal(t, 0, sentUpcRepeat, "upcoming reminder must only be sent once per bill")
	require.Empty(t, mockPush.GetMessages())

	// ── Execution 2: Overdue 3 Months+ on 2026-10-15 (Grouped notification for Budi Santoso's 2 bills) ──
	mockPush.Clear()
	sentOver, err := svc.ExecuteSchedule(ctx, schedOverdue, evalOct15)
	require.NoError(t, err)
	require.Equal(t, 1, sentOver, "Budi Santoso has 2 overdue >=3m bills and must receive 1 grouped push notification")

	msgs = mockPush.GetMessages()
	require.Len(t, msgs, 1)
	require.Equal(t, "Pengingat Tunggakan Iuran", msgs[0].Title)
	require.Contains(t, msgs[0].Body, "Halo Budi Santoso")
	require.Contains(t, msgs[0].Body, "2 tagihan")
	require.Contains(t, msgs[0].Body, "Rp 250.000") // 100.000 + 150.000
	require.Equal(t, "/home/iuran", msgs[0].Data["deep_link"], "grouped multi-bill notification deep-links to /home/iuran")
	require.Equal(t, "2", msgs[0].Data["bill_count"])

	// Re-running overdue schedule later in the same month (2026-10-20) must NOT resend
	mockPush.Clear()
	evalOct20 := time.Date(2026, 10, 20, 8, 0, 0, 0, finance.JakartaLocation())
	sentOverSameMonth, err := svc.ExecuteSchedule(ctx, schedOverdue, evalOct20)
	require.NoError(t, err)
	require.Equal(t, 0, sentOverSameMonth, "overdue reminder must not resend in the same YYYY-MM period")

	// Customize template by Pengurus before November run
	activeTrue := true
	_, err = svc.UpdateTemplate(ctx, rtA, tmplOverdue.ID, UpdateTemplateInput{
		Name:     tmplOverdue.Name,
		Title:    "Peringatan Akhir Tunggakan RT",
		Body:     "Yth {{nama}}, mohon segera melunasi {{bill_count}} tagihan sebesar {{total_outstanding}}.",
		IsActive: &activeTrue,
	})
	require.NoError(t, err)

	// Running overdue schedule in the NEXT month (2026-11-15):
	// Now 2026-08 bill ALSO reaches 3 months overdue (2026-08-15 + 3m = 2026-11-15), so Budi has 3 bills!
	mockPush.Clear()
	sentOverNov, err := svc.ExecuteSchedule(ctx, schedOverdue, evalNov15)
	require.NoError(t, err)
	require.Equal(t, 1, sentOverNov, "overdue reminder must send again in the next calendar month")
	msgs = mockPush.GetMessages()
	require.Len(t, msgs, 1)
	require.Equal(t, "Peringatan Akhir Tunggakan RT", msgs[0].Title)
	require.Contains(t, msgs[0].Body, "Yth Budi Santoso")
	require.Contains(t, msgs[0].Body, "3 tagihan")
	require.Contains(t, msgs[0].Body, "Rp 400.000") // 100.000 + 150.000 + 150.000

	// ── Execution 3: Paying off all overdue bills stops future reminders ──
	_, err = db.ExecContext(ctx,
		`UPDATE bills SET status = 'paid' WHERE household_occupancy_id = $1`,
		occA1,
	)
	require.NoError(t, err)

	// Run in December (2026-12-15): since all bills are paid, 0 notifications must be sent
	mockPush.Clear()
	evalDec15 := time.Date(2026, 12, 15, 8, 0, 0, 0, finance.JakartaLocation())
	sentOverDec, err := svc.ExecuteSchedule(ctx, schedOverdue, evalDec15)
	require.NoError(t, err)
	require.Equal(t, 0, sentOverDec, "must stop sending reminders once bills are paid")

	// Verify Scheduler Execution History & Recipient Detail compatibility
	history, total, err := svc.ListSchedulerHistory(ctx, rtA, 1, 20)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 3)
	require.NotEmpty(t, history)

	details, err := svc.GetSchedulerRecipientDetail(ctx, rtA, schedOverdue.ID, "2026-10-15")
	require.NoError(t, err)
	require.Len(t, details, 1)
	require.Equal(t, userA1, details[0].UserID)
	require.Equal(t, "SENT", details[0].Status)
}

func TestMigration055UpAndDownSafety(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	db := pool.Raw()

	upSQL, err := os.ReadFile("../../migrations/055_overdue_reminder_templates_and_schedules.up.sql")
	require.NoError(t, err)
	downSQL, err := os.ReadFile("../../migrations/055_overdue_reminder_templates_and_schedules.down.sql")
	require.NoError(t, err)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	// Create two test RTs inside the transaction
	var rtDefault, rtCustom string
	err = tx.QueryRowContext(ctx, `INSERT INTO rts (name, rw, rt, is_active) VALUES ('RT M055 Def', 1, 'M55D', true) RETURNING id`).Scan(&rtDefault)
	require.NoError(t, err)
	err = tx.QueryRowContext(ctx, `INSERT INTO rts (name, rw, rt, is_active) VALUES ('RT M055 Cus', 1, 'M55C', true) RETURNING id`).Scan(&rtCustom)
	require.NoError(t, err)

	// 1. Run UP twice (verify idempotency and no duplicates)
	_, err = tx.ExecContext(ctx, string(upSQL))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(upSQL))
	require.NoError(t, err)

	var tmplCount, schedCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_templates WHERE rt_id = $1 AND event_type IN ($2, $3)`, rtDefault, EventUpcomingDue3Months, EventOverdue3Months).Scan(&tmplCount)
	require.NoError(t, err)
	require.Equal(t, 2, tmplCount)

	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_schedules WHERE rt_id = $1 AND event_type IN ($2, $3)`, rtDefault, EventUpcomingDue3Months, EventOverdue3Months).Scan(&schedCount)
	require.NoError(t, err)
	require.Equal(t, 2, schedCount)

	// 2. Customize the overdue template and upcoming schedule on rtCustom
	_, err = tx.ExecContext(ctx, `UPDATE notification_templates SET body = 'Custom Body Pengurus' WHERE rt_id = $1 AND event_type = $2`, rtCustom, EventOverdue3Months)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE notification_schedules SET send_hour = 9 WHERE rt_id = $1 AND event_type = $2`, rtCustom, EventUpcomingDue3Months)
	require.NoError(t, err)

	// 3. Run DOWN migration
	_, err = tx.ExecContext(ctx, string(downSQL))
	require.NoError(t, err)

	// Verify rtDefault had all 2 default templates and 2 default schedules removed
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_templates WHERE rt_id = $1 AND event_type IN ($2, $3)`, rtDefault, EventUpcomingDue3Months, EventOverdue3Months).Scan(&tmplCount)
	require.NoError(t, err)
	require.Equal(t, 0, tmplCount, "unmodified default templates must be deleted on DOWN")

	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_schedules WHERE rt_id = $1 AND event_type IN ($2, $3)`, rtDefault, EventUpcomingDue3Months, EventOverdue3Months).Scan(&schedCount)
	require.NoError(t, err)
	require.Equal(t, 0, schedCount, "unmodified default schedules must be deleted on DOWN")

	// Verify rtCustom preserved its customized overdue template AND its customized upcoming schedule (plus the template referenced by that customized upcoming schedule)
	var customOverdueTmplCount, customUpcomingSchedCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_templates WHERE rt_id = $1 AND event_type = $2`, rtCustom, EventOverdue3Months).Scan(&customOverdueTmplCount)
	require.NoError(t, err)
	require.Equal(t, 1, customOverdueTmplCount, "customized template must NOT be deleted on DOWN")

	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_schedules WHERE rt_id = $1 AND event_type = $2`, rtCustom, EventUpcomingDue3Months).Scan(&customUpcomingSchedCount)
	require.NoError(t, err)
	require.Equal(t, 1, customUpcomingSchedCount, "customized schedule must NOT be deleted on DOWN")
}
