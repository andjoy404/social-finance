package notification_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/database"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"
)

// These tests exercise the real PostgreSQL recipient queries behind
// notification delivery. They run against the isolated test database only
// (testutil.GetTestPool enforces the "social_finance_test" guard).
//
// Covered authorization policy for payment verifiers (cmd/api/main.go,
// POST /api/v1/payments/{id}/verify):
//   - RequirePermission("finance.approve"): jabatan holders are evaluated
//     against position_permissions; members without jabatan pass through.
//   - RequireAccess(roles=[bendahara, pengurus], jabatans=[ketua, bendahara]).

var dbRTCounter int64 = time.Now().UnixNano() % 100000

func notifTestPool(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}

// ensureFinanceApproveCatalog makes sure the canonical permission catalog
// rows required by the verifier policy exist. Idempotent; matches the
// canonical grants seeded by migration 031 (ketua, bendahara).
func ensureFinanceApproveCatalog(t *testing.T, pool *database.Pool) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO permission_types (code, domain, operation)
		VALUES ('finance.approve', 'finance', 'approve')
		ON CONFLICT (code) DO NOTHING
	`)
	if err != nil {
		t.Fatalf("ensure finance.approve permission type: %v", err)
	}

	for _, position := range []string{"ketua", "bendahara"} {
		_, err = db.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT $1, id FROM permission_types WHERE code = 'finance.approve'
			ON CONFLICT (position, permission_id) DO NOTHING
		`, position)
		if err != nil {
			t.Fatalf("ensure %s finance.approve grant: %v", position, err)
		}
	}
}

func notifTestRT(t *testing.T, pool *database.Pool) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)
	var id string
	err := db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		fmt.Sprintf("Notif Repo Test RT %d-%d", time.Now().UnixNano(), n), 9,
		fmt.Sprintf("NOTIF-%d-%d", time.Now().UnixNano(), n),
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test RT: %v", err)
	}
	return id
}

func notifTestUser(t *testing.T, pool *database.Pool, label string, active bool) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)
	var id string
	err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'dummyhash', $3, $4) RETURNING id`,
		fmt.Sprintf("%d_%d_%s", time.Now().UnixNano(), n, label),
		fmt.Sprintf("+62811%08d", n), label, active,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return id
}

func notifTestMembership(t *testing.T, pool *database.Pool, userID, rtID, role string, jabatan *string, active bool) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, err := db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, rtID, role, jabatan, active,
	)
	if err != nil {
		t.Fatalf("create test membership: %v", err)
	}
}

func notifTestDevice(t *testing.T, pool *database.Pool, userID, token string, active bool) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, err := db.ExecContext(ctx,
		`INSERT INTO user_devices (user_id, fcm_token, platform, is_active, last_seen_at, updated_at)
		 VALUES ($1, $2, 'android', $3, NOW(), NOW())`,
		userID, token, active,
	)
	if err != nil {
		t.Fatalf("create test device: %v", err)
	}
}

func TestGetTokensForWasteTruckArrival_OnlyCurrentResidentsInRT(t *testing.T) {
	pool := notifTestPool(t)
	rtID := notifTestRT(t, pool)
	otherRT := notifTestRT(t, pool)
	n := atomic.AddInt64(&dbRTCounter, 1)

	wargaID := notifTestUser(t, pool, "warga-recipient", true)
	notifTestMembership(t, pool, wargaID, rtID, "warga", nil, true)
	wargaToken := fmt.Sprintf("warga-recipient-%d", n)
	notifTestDevice(t, pool, wargaID, wargaToken, true)
	notifTestOccupancyWithResident(t, pool, rtID, &wargaID)

	pengurusID := notifTestUser(t, pool, "pengurus-recipient", true)
	notifTestMembership(t, pool, pengurusID, rtID, "pengurus", nil, true)
	pengurusToken := fmt.Sprintf("pengurus-recipient-%d", n)
	notifTestDevice(t, pool, pengurusID, pengurusToken, true)
	notifTestOccupancyWithResident(t, pool, rtID, &pengurusID)

	petugasID := notifTestUser(t, pool, "petugas-recipient", true)
	keamanan := "keamanan"
	notifTestMembership(t, pool, petugasID, rtID, "warga", &keamanan, true)
	notifTestDevice(t, pool, petugasID, fmt.Sprintf("petugas-recipient-%d", n), true)

	otherRTWargaID := notifTestUser(t, pool, "other-rt-recipient", true)
	notifTestMembership(t, pool, otherRTWargaID, otherRT, "warga", nil, true)
	notifTestDevice(t, pool, otherRTWargaID, fmt.Sprintf("other-rt-recipient-%d", n), true)
	notifTestOccupancyWithResident(t, pool, otherRT, &otherRTWargaID)

	futureWargaID := notifTestUser(t, pool, "future-warga-recipient", true)
	notifTestMembership(t, pool, futureWargaID, rtID, "warga", nil, true)
	notifTestDevice(t, pool, futureWargaID, fmt.Sprintf("future-warga-recipient-%d", n), true)
	futureOccupancyID := notifTestOccupancyWithResident(t, pool, rtID, &futureWargaID)
	if _, err := pool.Raw().ExecContext(context.Background(),
		`UPDATE residency_periods SET start_date = CURRENT_DATE + 1 WHERE household_occupancy_id = $1`,
		futureOccupancyID,
	); err != nil {
		t.Fatalf("make future residency: %v", err)
	}

	endedTodayID := notifTestUser(t, pool, "ended-today-recipient", true)
	notifTestMembership(t, pool, endedTodayID, rtID, "warga", nil, true)
	notifTestDevice(t, pool, endedTodayID, fmt.Sprintf("ended-today-recipient-%d", n), true)
	endedTodayOccupancyID := notifTestOccupancyWithResident(t, pool, rtID, &endedTodayID)
	if _, err := pool.Raw().ExecContext(context.Background(),
		`UPDATE residency_periods SET start_date = CURRENT_DATE - 1, end_date = CURRENT_DATE WHERE household_occupancy_id = $1`,
		endedTodayOccupancyID,
	); err != nil {
		t.Fatalf("end residency today: %v", err)
	}

	tokens, err := notification.NewRepository(pool).GetTokensForWasteTruckArrival(context.Background(), rtID)
	if err != nil {
		t.Fatalf("GetTokensForWasteTruckArrival: %v", err)
	}
	got := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		got[token] = true
	}
	if len(got) != 2 || !got[wargaToken] || !got[pengurusToken] {
		t.Fatalf("expected only current household resident tokens %q and %q, got %v", wargaToken, pengurusToken, tokens)
	}
	recipients, err := notification.NewRepository(pool).GetWasteTruckArrivalRecipients(context.Background(), rtID)
	if err != nil {
		t.Fatalf("GetWasteTruckArrivalRecipients: %v", err)
	}
	recipientUsers := make(map[string]bool, len(recipients))
	for _, recipient := range recipients {
		recipientUsers[recipient.UserID] = true
	}
	if len(recipientUsers) != 2 || !recipientUsers[wargaID] || !recipientUsers[pengurusID] {
		t.Fatalf("expected only current household resident users (including Pengurus as a resident), got %+v", recipients)
	}
	broadcastRecipients, err := notification.NewRepository(pool).GetRTBroadcastRecipients(context.Background(), rtID)
	if err != nil {
		t.Fatalf("GetRTBroadcastRecipients: %v", err)
	}
	broadcastUsers := make(map[string]bool, len(broadcastRecipients))
	for _, recipient := range broadcastRecipients {
		broadcastUsers[recipient.UserID] = true
	}
	if !broadcastUsers[petugasID] || broadcastUsers[otherRTWargaID] {
		t.Fatalf("general RT broadcasts must include active same-RT security membership without synthetic occupancy and exclude other RT users: %+v", broadcastRecipients)
	}
}

// notifTestOccupancyWithResident creates physical house → household →
// occupancy → resident (optionally linked to a user) → open residency period.
func notifTestOccupancyWithResident(t *testing.T, pool *database.Pool, rtID string, residentUserID *string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	var houseID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, address) VALUES ($1, $2, 'Test Street') RETURNING id`,
		rtID, fmt.Sprintf("NR-%d", n),
	).Scan(&houseID); err != nil {
		t.Fatalf("create house: %v", err)
	}

	var hhID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Notif Test Head') RETURNING id`,
		rtID,
	).Scan(&hhID); err != nil {
		t.Fatalf("create household: %v", err)
	}

	var occID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', CURRENT_DATE) RETURNING id`,
		hhID, houseID,
	).Scan(&occID); err != nil {
		t.Fatalf("create occupancy: %v", err)
	}

	var resID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active, user_id)
		 VALUES ($1, 'Notif Test Resident', $2, true, $3) RETURNING id`,
		rtID, fmt.Sprintf("+62822%08d", n), residentUserID,
	).Scan(&resID); err != nil {
		t.Fatalf("create resident: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, CURRENT_DATE)`,
		resID, occID,
	); err != nil {
		t.Fatalf("create residency period: %v", err)
	}

	return occID
}

func notifTestBill(t *testing.T, pool *database.Pool, rtID, occID, period string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	var dueID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO dues (rt_id, name, amount, period_type, is_active)
		 VALUES ($1, $2, 50000, 'monthly', true) RETURNING id`,
		rtID, fmt.Sprintf("Notif Due %d", n),
	).Scan(&dueID); err != nil {
		t.Fatalf("create due: %v", err)
	}

	var billID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
		 VALUES ($1, $2, $3, 50000, $4, CURRENT_DATE + INTERVAL '7 days', 'unpaid') RETURNING id`,
		rtID, occID, dueID, period,
	).Scan(&billID); err != nil {
		t.Fatalf("create bill: %v", err)
	}

	return billID
}

func TestNotificationRepository_ManagementHistoryClassification(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	otherRTID := notifTestRT(t, pool)
	actorA := notifTestUser(t, pool, "history-actor-a", true)
	actorB := notifTestUser(t, pool, "history-actor-b", true)
	recipient := notifTestUser(t, pool, "history-recipient", true)
	defer func() {
		cleanupNotifTestRT(t, pool, rtID)
		cleanupNotifTestRT(t, pool, otherRTID)
		for _, userID := range []string{actorA, actorB, recipient} {
			_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		}
	}()
	testKey := fmt.Sprintf("%d", atomic.AddInt64(&dbRTCounter, 1))
	ketuaJabatan := "ketua"
	notifTestMembership(t, pool, actorA, rtID, "pengurus", &ketuaJabatan, true)
	notifTestMembership(t, pool, actorB, rtID, "warga", nil, true)

	occupancyID := notifTestOccupancyWithResident(t, pool, rtID, &recipient)
	approvedBillID := notifTestBill(t, pool, rtID, occupancyID, fmt.Sprintf("history-approved-%d", atomic.AddInt64(&dbRTCounter, 1)))
	rejectedBillID := notifTestBill(t, pool, rtID, occupancyID, fmt.Sprintf("history-rejected-%d", atomic.AddInt64(&dbRTCounter, 1)))
	var approvedPaymentID, rejectedPaymentID string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, verified_by)
		VALUES ($1, $2, 50000, 'TRANSFER', 'SELF_SUBMITTED', 'APPROVED', $3)
		RETURNING id::text
	`, rtID, approvedBillID, actorA).Scan(&approvedPaymentID); err != nil {
		t.Fatalf("create approved payment: %v", err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, verified_by)
		VALUES ($1, $2, 50000, 'TRANSFER', 'SELF_SUBMITTED', 'REJECTED', $3)
		RETURNING id::text
	`, rtID, rejectedBillID, actorB).Scan(&rejectedPaymentID); err != nil {
		t.Fatalf("create rejected payment: %v", err)
	}

	insertHistoryLog := func(logRT, userID, eventType, dedupKey, status, paymentID string) {
		t.Helper()
		payload := map[string]string{}
		if paymentID != "" {
			payload["payment_id"] = paymentID
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal log payload: %v", err)
		}
		_, err = pool.Raw().ExecContext(ctx, `
			INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, payload, status, failure_reason)
			VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8,
				CASE WHEN $8 = 'FAILED' THEN 'provider reported failure' ELSE NULL END)
		`, logRT, userID, eventType, dedupKey, eventType, "history test", payloadJSON, status)
		if err != nil {
			t.Fatalf("insert history log %q: %v", dedupKey, err)
		}
	}

	insertHistoryLog(rtID, "", notification.EventUpcomingDue3Months, "schedule:"+testKey+":2026-11-06:"+recipient, "SENT", "")
	insertHistoryLog(rtID, "", notification.EventUpcomingDue3Months, "schedule:"+testKey+":2026-11-06:"+actorA, "FAILED", "")
	insertHistoryLog(rtID, "", notification.EventUpcomingDue3Months, "schedule:"+testKey+":2026-11-07:"+actorA, "PENDING", "")
	insertHistoryLog(rtID, actorA, notification.EventManualPush, "recipient:manual_push:"+testKey+":"+actorA, "SENT", "")
	insertHistoryLog(otherRTID, actorA, notification.EventManualPush, "recipient:manual_push:other-"+testKey+":"+actorA, "SENT", "")
	insertHistoryLog(rtID, actorB, notification.EventManualPush, "recipient:manual_push:"+testKey+":"+actorB, "SENT", "")
	insertHistoryLog(rtID, actorB, notification.EventManualPush, "recipient:manual_push:mismatch-"+testKey+":"+actorA, "SENT", "") // mismatched user id/key is not a receipt for actor B
	insertHistoryLog(rtID, recipient, notification.EventOverdue3Months, "schedule:"+testKey+":2026-11-09:"+recipient, "SENT", "")
	insertHistoryLog(rtID, actorA, notification.EventOverdue3Months, "schedule:"+testKey+":2026-11-09:"+actorA, "UNKNOWN", "")
	insertHistoryLog(otherRTID, "", notification.EventUpcomingDue3Months, "schedule:other-"+testKey+":2026-11-06:"+recipient, "SENT", "")
	insertHistoryLog(rtID, actorA, notification.EventManualPush, "manual_push:"+testKey, "SENT", "")
	insertHistoryLog(rtID, recipient, notification.EventPaymentVerified, "payment-approved:"+testKey+":recipient", "SENT", approvedPaymentID)
	insertHistoryLog(rtID, "", notification.EventPaymentVerified, "payment-approved:"+testKey+":verifier", "FAILED", approvedPaymentID)
	insertHistoryLog(rtID, recipient, notification.EventPaymentVerified, "payment-rejected:"+testKey+":recipient", "SENT", rejectedPaymentID)
	insertHistoryLog(rtID, "", notification.EventPaymentVerified, "payment-rejected:"+testKey+":verifier", "SENT", rejectedPaymentID)
	insertHistoryLog(rtID, actorA, notification.EventWasteTruckArrival, "recipient:waste_truck_arrival:"+testKey+":"+actorA, "SENT", "")
	insertHistoryLog(rtID, actorB, notification.EventWasteTruckArrival, "recipient:waste_truck_arrival:"+testKey+":"+actorB, "SENT", "")
	insertHistoryLog(rtID, actorB, notification.EventWasteTruckArrival, "recipient:waste_truck_arrival:mismatch-"+testKey+":"+actorA, "SENT", "") // mismatched user id/key is not a receipt for actor B
	insertHistoryLog(otherRTID, actorA, notification.EventWasteTruckArrival, "recipient:waste_truck_arrival:other:"+actorA, "SENT", "")
	insertHistoryLog(rtID, "", notification.EventBillCreated, "bill:unknown:"+testKey, "SENT", "")
	legacyDedupKey := "schedule:" + testKey + ":2026-11-08:" + recipient
	if _, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, payload, status)
		VALUES ($1, $2, $3, $4, 'Legacy reminder', 'Legacy body', '{}'::jsonb, 'FAILED')
	`, rtID, recipient, notification.EventOverdue3Months, legacyDedupKey); err != nil {
		t.Fatalf("insert legacy delivery without provider evidence: %v", err)
	}

	systemHistory, systemTotal, err := repo.ListManagementHistory(ctx, rtID, actorA, notification.HistoryCategorySystem, 0, 20)
	if err != nil {
		t.Fatalf("ListManagementHistory system: %v", err)
	}
	if systemTotal != 4 || len(systemHistory) != 4 {
		t.Fatalf("expected four in-RT schedule activities, total=%d entries=%d: %+v", systemTotal, len(systemHistory), systemHistory)
	}
	foundPartial, foundUnknown, foundLegacyUnknown, foundSentWithUnknown := false, false, false, false
	for _, entry := range systemHistory {
		if strings.Contains(entry.DedupKey, ":2026-11-06:") {
			foundPartial = entry.RecipientsCount == 2 && entry.SentCount == 1 &&
				entry.FailedCount == 1 && entry.Status == "PARTIAL"
		}
		if strings.Contains(entry.DedupKey, ":2026-11-07:") {
			foundUnknown = entry.RecipientsCount == 1 && entry.SentCount == 0 &&
				entry.FailedCount == 0 && entry.Status == "UNKNOWN"
		}
		if entry.DedupKey == legacyDedupKey {
			foundLegacyUnknown = entry.Status == "UNKNOWN"
		}
		if strings.Contains(entry.DedupKey, ":2026-11-09:") {
			foundSentWithUnknown = entry.Status == "SENT" &&
				entry.RecipientsCount == 2 && entry.SentCount == 1 && entry.FailedCount == 0
		}
	}
	if !foundPartial || !foundUnknown || !foundLegacyUnknown || !foundSentWithUnknown {
		t.Fatalf("unexpected grouped scheduler delivery statuses: %+v", systemHistory)
	}
	wargaHistory, _, err := repo.ListLogs(ctx, rtID, recipient, "", "", false, 0, 20)
	if err != nil {
		t.Fatalf("ListLogs for legacy reminder recipient: %v", err)
	}
	foundLegacyUnknown, foundRecipientSent := false, false
	for _, entry := range wargaHistory {
		if entry.DedupKey == legacyDedupKey {
			foundLegacyUnknown = entry.Status == "UNKNOWN"
		}
		if strings.Contains(entry.DedupKey, ":2026-11-09:") {
			foundRecipientSent = entry.Status == "SENT"
		}
	}
	if !foundLegacyUnknown || !foundRecipientSent {
		t.Fatalf("recipient must retain provider SENT while legacy FAILED without evidence remains UNKNOWN: %+v", wargaHistory)
	}

	mineA, mineATotal, err := repo.ListManagementHistory(ctx, rtID, actorA, notification.HistoryCategoryMine, 0, 20)
	if err != nil {
		t.Fatalf("ListManagementHistory actor A: %v", err)
	}
	if mineATotal != 5 || len(mineA) != 5 {
		t.Fatalf("expected sent activity, actual received notifications, and payment action for actor A, got total=%d entries=%+v", mineATotal, mineA)
	}
	foundManual, foundPayment, foundReceivedBlast, foundReceivedManual, foundReceivedReminder := false, false, false, false, false
	for _, entry := range mineA {
		if entry.EventType == notification.EventManualPush {
			foundManual = foundManual || (entry.Status == "RECORDED" && !entry.DeliveryCountsAvailable && entry.ActivityRelation == "sent_by_me")
		}
		if entry.DedupKey == "payment_action:"+approvedPaymentID+":APPROVED" {
			foundPayment = entry.Status == "APPROVED" && !entry.DeliveryCountsAvailable && entry.ActivityRelation == "performed_by_me"
		}
		if strings.HasPrefix(entry.DedupKey, "recipient:waste_truck_arrival:") {
			foundReceivedBlast = entry.Status == "SENT" && entry.ActivityRelation == "received_by_me"
		}
		if strings.HasPrefix(entry.DedupKey, "recipient:manual_push:") {
			foundReceivedManual = entry.Status == "SENT" && entry.ActivityRelation == "received_by_me"
		}
		if strings.HasPrefix(entry.DedupKey, "schedule:") && strings.HasSuffix(entry.DedupKey, ":"+actorA) {
			foundReceivedReminder = entry.Status == "UNKNOWN" && entry.ActivityRelation == "received_by_me"
		}
		if entry.ID == "" || entry.HistoryCategory != notification.HistoryCategoryMine {
			t.Errorf("unexpected history metadata: %+v", entry)
		}
	}
	if !foundManual || !foundPayment || !foundReceivedBlast || !foundReceivedManual || !foundReceivedReminder {
		t.Fatalf("actor A history did not include sent activity and actual recipient records with their own statuses: %+v", mineA)
	}
	for _, entry := range mineA {
		if entry.RTID != rtID {
			t.Fatalf("actor A history contains cross-RT delivery: %+v", entry)
		}
	}

	mineB, mineBTotal, err := repo.ListManagementHistory(ctx, rtID, actorB, notification.HistoryCategoryMine, 0, 20)
	if err != nil {
		t.Fatalf("ListManagementHistory actor B: %v", err)
	}
	if mineBTotal != 3 || len(mineB) != 3 {
		t.Fatalf("actor B should see two actual broadcast receipts and their payment action, got total=%d: %+v", mineBTotal, mineB)
	}
	foundActorBManual, foundActorBWasteTruck := false, false
	for _, entry := range mineB {
		if entry.ActivityRelation != "received_by_me" || entry.UserID == nil || *entry.UserID != actorB {
			continue
		}
		if entry.EventType == notification.EventManualPush {
			foundActorBManual = entry.Status == "SENT" && strings.HasSuffix(entry.DedupKey, ":"+actorB)
		}
		if entry.EventType == notification.EventWasteTruckArrival {
			foundActorBWasteTruck = entry.Status == "SENT" && strings.HasSuffix(entry.DedupKey, ":"+actorB)
		}
	}
	if !foundActorBManual || !foundActorBWasteTruck {
		t.Fatalf("actor B should see only the actual manual and waste-truck receipts: %+v", mineB)
	}
	for _, entry := range mineB {
		if strings.HasSuffix(entry.DedupKey, ":"+actorA) {
			t.Fatalf("actor B received an activity recorded for actor A: %+v", entry)
		}
	}
}

func TestManagementHistoryIncludesPersistedManualAndWasteTruckRecipientDeliveries(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()
	rtID := notifTestRT(t, pool)
	senderID := notifTestUser(t, pool, "history-sender", true)
	recipientID := notifTestUser(t, pool, "history-recipient-actual", true)
	ketuaJabatan := "ketua"
	notifTestMembership(t, pool, senderID, rtID, "pengurus", &ketuaJabatan, true)
	notifTestMembership(t, pool, recipientID, rtID, "warga", nil, true)
	defer func() {
		cleanupNotifTestRT(t, pool, rtID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, senderID, recipientID)
	}()

	recipients, err := repo.GetRTBroadcastRecipients(ctx, rtID)
	if err != nil {
		t.Fatalf("load actual active RT broadcast recipients: %v", err)
	}
	actualRecipients := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		actualRecipients[recipient.UserID] = struct{}{}
	}
	if _, ok := actualRecipients[recipientID]; !ok {
		t.Fatalf("test recipient %s was not selected from active RT memberships: %+v", recipientID, recipients)
	}

	manualPayload := map[string]string{"title": "Manual history test", "body": "Recipient-scoped manual broadcast"}
	manualKey := fmt.Sprintf("manual_push:%s:%d:test", rtID, atomic.AddInt64(&dbRTCounter, 1))
	created, err := repo.CheckAndRecordDedup(ctx, rtID, senderID, notification.EventManualPush, manualKey, manualPayload["title"], manualPayload["body"], manualPayload)
	if err != nil || !created {
		t.Fatalf("record manual broadcast parent: created=%t err=%v", created, err)
	}
	manualBroadcastID := manualPayload["notification_id"]
	if err := repo.RecordBroadcastRecipientDelivery(ctx, rtID, senderID, recipientID, notification.EventManualPush, manualBroadcastID, manualPayload["title"], manualPayload["body"], manualPayload, "SENT"); err != nil {
		t.Fatalf("persist actual manual recipient delivery: %v", err)
	}

	truckPayload := map[string]string{"title": "Waste truck history test", "body": "Recipient-scoped truck dispatch"}
	truckKey := fmt.Sprintf("waste_truck:%s:test:%d", rtID, atomic.AddInt64(&dbRTCounter, 1))
	truckID, created, err := repo.RecordWasteTruckDispatch(ctx, rtID, senderID, truckKey, truckPayload["title"], truckPayload["body"], truckPayload)
	if err != nil || !created {
		t.Fatalf("record waste-truck dispatch parent: id=%s created=%t err=%v", truckID, created, err)
	}
	if err := repo.RecordBroadcastRecipientDelivery(ctx, rtID, senderID, recipientID, notification.EventWasteTruckArrival, truckID, truckPayload["title"], truckPayload["body"], truckPayload, "SENT"); err != nil {
		t.Fatalf("persist actual waste-truck recipient delivery: %v", err)
	}

	mine, total, err := repo.ListManagementHistory(ctx, rtID, recipientID, notification.HistoryCategoryMine, 0, 20)
	if err != nil {
		t.Fatalf("load recipient personal history: %v", err)
	}
	if total != 2 || len(mine) != 2 {
		t.Fatalf("actual recipient should see one delivery for each broadcast event; got total=%d entries=%+v", total, mine)
	}
	foundManual, foundTruck := false, false
	for _, entry := range mine {
		if entry.RTID != rtID || entry.UserID == nil || *entry.UserID != recipientID || entry.ActivityRelation != "received_by_me" {
			t.Fatalf("recipient history contains a non-recipient or cross-RT row: %+v", entry)
		}
		switch entry.EventType {
		case notification.EventManualPush:
			foundManual = entry.Status == "SENT" && entry.ID != manualBroadcastID
		case notification.EventWasteTruckArrival:
			foundTruck = entry.Status == "SENT" && entry.ID != truckID
		}
	}
	if !foundManual || !foundTruck {
		t.Fatalf("recipient history omitted persisted manual or waste-truck deliveries: %+v", mine)
	}

	senderHistory, senderTotal, err := repo.ListManagementHistory(ctx, rtID, senderID, notification.HistoryCategoryMine, 0, 20)
	if err != nil {
		t.Fatalf("load sender personal history: %v", err)
	}
	if senderTotal < 2 {
		t.Fatalf("sender history lost manual/truck dispatch activity: %+v", senderHistory)
	}
}

func cleanupNotifTestRT(t *testing.T, pool *database.Pool, rtID string) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM notification_logs WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM transactions WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM payments WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM bills WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM dues WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM financial_categories WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `
		DELETE FROM residency_periods WHERE household_occupancy_id IN (
			SELECT id FROM household_occupancies WHERE household_id IN (
				SELECT id FROM households WHERE rt_id = $1))`, rtID)
	_, _ = db.ExecContext(ctx, `
		DELETE FROM household_occupancies WHERE household_id IN (
			SELECT id FROM households WHERE rt_id = $1)`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM physical_houses WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM households WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
}

// --- GetTokensForPaymentVerifiers ---

// Reported regression case: role=pengurus, jabatan=bendahara with
// finance.approve must receive payment-submission notifications.
func TestGetTokensForPaymentVerifiers_PengurusWithBendaharaJabatan(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "pengurus-bendahara", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, userID, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-pengurus-bendahara", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "tok-pengurus-bendahara" {
		t.Fatalf("expected [tok-pengurus-bendahara], got %v", tokens)
	}
}

// Documented pattern from migration 031: Ketua RT has role=warga + jabatan=ketua.
func TestGetTokensForPaymentVerifiers_KetuaWithWargaRole(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "warga-ketua", true)
	jabatan := "ketua"
	notifTestMembership(t, pool, userID, rtID, "warga", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-warga-ketua", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "tok-warga-ketua" {
		t.Fatalf("expected [tok-warga-ketua], got %v", tokens)
	}
}

// Role-based verifier without jabatan passes RequirePermission by pass-through
// and RequireAccess by role → must be notified.
func TestGetTokensForPaymentVerifiers_PengurusWithoutJabatan(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "pengurus-no-jabatan", true)
	notifTestMembership(t, pool, userID, rtID, "pengurus", nil, true)
	notifTestDevice(t, pool, userID, "tok-pengurus-no-jabatan", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "tok-pengurus-no-jabatan" {
		t.Fatalf("expected [tok-pengurus-no-jabatan], got %v", tokens)
	}
}

// Ordinary warga without jabatan cannot verify → must NOT be notified.
func TestGetTokensForPaymentVerifiers_WargaExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "warga-no-jabatan", true)
	notifTestMembership(t, pool, userID, rtID, "warga", nil, true)
	notifTestDevice(t, pool, userID, "tok-warga", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens for warga, got %v", tokens)
	}
}

// A pengurus whose jabatan lacks finance.approve (e.g. anggota) is blocked by
// RequirePermission on the verify route → must NOT be notified.
func TestGetTokensForPaymentVerifiers_JabatanWithoutFinanceApproveExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "pengurus-anggota", true)
	jabatan := "anggota"
	notifTestMembership(t, pool, userID, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-pengurus-anggota", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens for jabatan without finance.approve, got %v", tokens)
	}
}

// An authorized verifier in another RT must not receive the notification.
func TestGetTokensForPaymentVerifiers_OtherRTExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	targetRT := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, targetRT)
	otherRT := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, otherRT)

	otherUserID := notifTestUser(t, pool, "other-rt-verifier", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, otherUserID, otherRT, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, otherUserID, "tok-other-rt", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, targetRT)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens from other RT, got %v", tokens)
	}
}

// Inactive users, inactive memberships, and inactive devices are excluded.
func TestGetTokensForPaymentVerifiers_InactiveRowsExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	jabatan := "bendahara"

	// Inactive user with active membership and device.
	inactiveUser := notifTestUser(t, pool, "inactive-user", false)
	notifTestMembership(t, pool, inactiveUser, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, inactiveUser, "tok-inactive-user", true)

	// Active user with inactive membership.
	noMembershipUser := notifTestUser(t, pool, "inactive-membership", true)
	notifTestMembership(t, pool, noMembershipUser, rtID, "pengurus", &jabatan, false)
	notifTestDevice(t, pool, noMembershipUser, "tok-inactive-membership", true)

	// Active user and membership but inactive device.
	noDeviceUser := notifTestUser(t, pool, "inactive-device", true)
	notifTestMembership(t, pool, noDeviceUser, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, noDeviceUser, "tok-inactive-device", false)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens (all inactive), got %v", tokens)
	}
}

// A member matching multiple recipient conditions (role=pengurus AND
// jabatan=bendahara) with several devices must yield each token exactly once.
func TestGetTokensForPaymentVerifiers_NoDuplicateTokensForMultipleMatches(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "multi-match", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, userID, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-multi-a", true)
	notifTestDevice(t, pool, userID, "tok-multi-b", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	counts := make(map[string]int)
	for _, tok := range tokens {
		counts[tok]++
	}
	if len(tokens) != 2 {
		t.Fatalf("expected exactly 2 unique tokens, got %d: %v", len(tokens), tokens)
	}
	if counts["tok-multi-a"] != 1 || counts["tok-multi-b"] != 1 {
		t.Fatalf("expected each token exactly once, got %v", counts)
	}
}

// --- GetRecipientUserIDsForBill ---

func TestGetRecipientUserIDsForBill_IncludesOrdinaryResident(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "resident-ordinary", true)
	notifTestMembership(t, pool, userID, rtID, "warga", nil, true)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &userID)
	billID := notifTestBill(t, pool, rtID, occID, "2027-01")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 1 || ids[0] != userID {
		t.Fatalf("expected [%s], got %v", userID, ids)
	}
}

// A resident who also holds a management role (pengurus) must keep receiving
// resident-addressed notifications for their own household.
func TestGetRecipientUserIDsForBill_IncludesPengurusResident(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	officerID := notifTestUser(t, pool, "resident-pengurus", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, officerID, rtID, "pengurus", &jabatan, true)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &officerID)
	billID := notifTestBill(t, pool, rtID, occID, "2027-02")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 1 || ids[0] != officerID {
		t.Fatalf("expected pengurus resident [%s] to receive bill notification, got %v", officerID, ids)
	}
}

func TestGetRecipientUserIDsForBill_ExcludesInactiveUser(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	inactiveID := notifTestUser(t, pool, "resident-inactive", false)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &inactiveID)
	billID := notifTestBill(t, pool, rtID, occID, "2027-03")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no recipients for inactive user, got %v", ids)
	}
}

// One person matching multiple recipient conditions (same user resolved via
// user_id and via email fallback) must appear only once.
func TestGetRecipientUserIDsForBill_NoDuplicateRecipients(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "resident-dup", true)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &userID)

	// Second resident row for the SAME user (email fallback path would also
	// resolve to this user); the unique user_id index prevents duplicate
	// user_id rows, so simulate the multi-condition match via an email-only
	// resident whose email equals the user's email.
	db := pool.Raw()
	var email string
	if err := db.QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
		t.Fatalf("load user email: %v", err)
	}
	var resID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active, email)
		 VALUES ($1, 'Email Fallback Resident', '+628999999999', true, $2) RETURNING id`,
		rtID, email,
	).Scan(&resID); err != nil {
		t.Fatalf("create email-fallback resident: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, CURRENT_DATE)`,
		resID, occID,
	); err != nil {
		t.Fatalf("create email-fallback residency period: %v", err)
	}

	billID := notifTestBill(t, pool, rtID, occID, "2027-04")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 1 || ids[0] != userID {
		t.Fatalf("expected exactly one recipient [%s], got %v", userID, ids)
	}
}

// jabatan=ketua (with finance.approve from migration 031) MUST be included
// in the verifier list via the position_permissions branch.  This test
// confirms the positive path: when jabatan=ketua has the permission grant,
// the user appears regardless of their tenant role.
func TestGetTokensForPaymentVerifiers_JabatanKetuaWithFinanceApproveIncluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool) // migration 031 already grants this, but be explicit
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	// role=warga with jabatan=ketua and finance.approve should be included
	// via the position_permissions branch of the query.
	jabatanKetuaID := notifTestUser(t, pool, "jabatan-ketua-with-perm", true)
	jabatan := "ketua"
	notifTestMembership(t, pool, jabatanKetuaID, rtID, "warga", &jabatan, true)
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO user_devices (user_id, fcm_token, platform, is_active, last_seen_at, updated_at)
		 VALUES ($1, $2, 'android', true, NOW(), NOW())`,
		jabatanKetuaID, "tok-jabatan-ketua-with-perm",
	)
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}

	found := false
	for _, tok := range tokens {
		if tok == "tok-jabatan-ketua-with-perm" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("jabatan=ketua WITH finance.approve SHOULD be a payment verifier, but token was not returned")
	}
}

func TestNotificationRepository_ReadReceiptsAndScoping(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	otherRTID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, otherRTID)

	wargaAID := notifTestUser(t, pool, "warga-a", true)
	notifTestMembership(t, pool, wargaAID, rtID, "warga", nil, true)

	wargaBID := notifTestUser(t, pool, "warga-b", true)
	notifTestMembership(t, pool, wargaBID, rtID, "warga", nil, true)

	// Insert:
	// 1. Broadcast parent plus an actual recipient delivery for Warga A in rtID
	// 2. Personal bill notification for Warga A in rtID
	// 3. Personal bill notification for Warga B in rtID
	// 4. Internal notification with user_id=NULL (NOT public announcement) in rtID
	// 5. Cross-tenant notification in otherRTID
	var notifAnnounceParentID, notifAnnounceID, notifAnnounceBID, notifBillAID, notifBillBID, notifInternalNullID, notifOtherRTID string
	err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, NULL, 'manual_push', 'test_db:manual:1', 'Pengumuman Kerja Bakti', 'Minggu jam 7', 'SENT', NOW() - INTERVAL '5 minutes')
		RETURNING id::text
	`, rtID).Scan(&notifAnnounceParentID)
	if err != nil {
		t.Fatalf("insert announcement: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs
			(rt_id, user_id, sender_user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, $2, $3, 'manual_push', $4, 'Pengumuman Kerja Bakti', 'Minggu jam 7', 'SENT', NOW() - INTERVAL '5 minutes')
		RETURNING id::text
	`, rtID, wargaAID, wargaBID, "recipient:manual_push:"+notifAnnounceParentID+":"+wargaAID).Scan(&notifAnnounceID)
	if err != nil {
		t.Fatalf("insert announcement recipient delivery: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs
			(rt_id, user_id, sender_user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, $2, $3, 'manual_push', $4, 'Pengumuman Kerja Bakti', 'Minggu jam 7', 'SENT', NOW() - INTERVAL '5 minutes')
		RETURNING id::text
	`, rtID, wargaBID, wargaAID, "recipient:manual_push:"+notifAnnounceParentID+":"+wargaBID).Scan(&notifAnnounceBID)
	if err != nil {
		t.Fatalf("insert announcement recipient delivery for Warga B: %v", err)
	}

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, $2, 'iuran_bill_created', 'test_db:bill_a:1', 'Tagihan Warga A', 'IPL Rp 50.000', 'SENT', NOW() - INTERVAL '3 minutes')
		RETURNING id::text
	`, rtID, wargaAID).Scan(&notifBillAID)
	if err != nil {
		t.Fatalf("insert bill A: %v", err)
	}

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, $2, 'iuran_bill_created', 'test_db:bill_b:1', 'Tagihan Warga B', 'IPL Rp 50.000', 'SENT', NOW() - INTERVAL '1 minute')
		RETURNING id::text
	`, rtID, wargaBID).Scan(&notifBillBID)
	if err != nil {
		t.Fatalf("insert bill B: %v", err)
	}

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, NULL, 'system_maintenance', 'test_db:internal:1', 'Maintenance Server', 'Internal officer note', 'SENT', NOW() - INTERVAL '30 seconds')
		RETURNING id::text
	`, rtID).Scan(&notifInternalNullID)
	if err != nil {
		t.Fatalf("insert internal null log: %v", err)
	}

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, NULL, 'manual_push', 'test_db:other_rt:1', 'Pengumuman RT Lain', 'Khusus RT lain', 'SENT', NOW())
		RETURNING id::text
	`, otherRTID).Scan(&notifOtherRTID)
	if err != nil {
		t.Fatalf("insert other RT log: %v", err)
	}

	// 1. Warga A lists logs -> sees only their recorded delivery and own bill.
	logsA, totalA, err := repo.ListLogs(ctx, rtID, wargaAID, "", "", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs for Warga A: %v", err)
	}
	if totalA != 2 || len(logsA) != 2 {
		t.Fatalf("expected Warga A to see exactly 2 logs, got total=%d len=%d", totalA, len(logsA))
	}
	for _, l := range logsA {
		if l.ID == notifBillBID {
			t.Errorf("Warga A must not see Warga B's personal notification")
		}
		if l.ID == notifInternalNullID {
			t.Errorf("Warga A must not see internal system notification with user_id=NULL")
		}
		if l.ID == notifOtherRTID {
			t.Errorf("Warga A must not see notification from another RT")
		}
		if l.IsRead {
			t.Errorf("expected initial notification to be unread")
		}
	}

	// 2. Unread count check: exactly 2 for Warga A and 2 for Warga B
	unreadA, err := repo.GetUnreadCount(ctx, rtID, wargaAID, false)
	if err != nil {
		t.Fatalf("GetUnreadCount A: %v", err)
	}
	if unreadA != 2 {
		t.Errorf("expected 2 unread for Warga A, got %d", unreadA)
	}

	unreadB, err := repo.GetUnreadCount(ctx, rtID, wargaBID, false)
	if err != nil {
		t.Fatalf("GetUnreadCount B: %v", err)
	}
	if unreadB != 2 {
		t.Errorf("expected 2 unread for Warga B, got %d", unreadB)
	}

	// 3. Warga A marks own bill as read -> SUCCESS
	if err := repo.MarkAsRead(ctx, rtID, wargaAID, notifBillAID, false); err != nil {
		t.Fatalf("MarkAsRead for Warga A: %v", err)
	}

	unreadA2, _ := repo.GetUnreadCount(ctx, rtID, wargaAID, false)
	if unreadA2 != 1 {
		t.Errorf("expected 1 unread for Warga A after marking read, got %d", unreadA2)
	}

	// User isolation: Warga B's unread count MUST remain 2
	unreadB2, _ := repo.GetUnreadCount(ctx, rtID, wargaBID, false)
	if unreadB2 != 2 {
		t.Errorf("expected 2 unread for Warga B (unchanged), got %d", unreadB2)
	}

	// 4. Status filter test
	unreadLogsA, _, err := repo.ListLogs(ctx, rtID, wargaAID, "", "unread", false, 0, 10)
	if err != nil || len(unreadLogsA) != 1 || unreadLogsA[0].ID != notifAnnounceID {
		t.Fatalf("expected 1 unread log (announcement) for Warga A, got %v", unreadLogsA)
	}

	readLogsA, _, err := repo.ListLogs(ctx, rtID, wargaAID, "", "read", false, 0, 10)
	if err != nil || len(readLogsA) != 1 || readLogsA[0].ID != notifBillAID {
		t.Fatalf("expected 1 read log (own bill) for Warga A, got %v", readLogsA)
	}

	// 5a. Warga A attempts to mark Warga B's personal notification as read -> ErrForbidden
	if err := repo.MarkAsRead(ctx, rtID, wargaAID, notifBillBID, false); err != notification.ErrForbidden {
		t.Errorf("expected ErrForbidden when Warga A attempts to mark Warga B's bill as read, got %v", err)
	}

	// 5b. Warga A attempts to mark internal notification (user_id=NULL, not public) as read -> ErrForbidden
	if err := repo.MarkAsRead(ctx, rtID, wargaAID, notifInternalNullID, false); err != notification.ErrForbidden {
		t.Errorf("expected ErrForbidden when Warga A attempts to mark internal notification as read, got %v", err)
	}

	// 5c. Warga A attempts to mark other RT notification as read -> sql.ErrNoRows (tenant isolation)
	if err := repo.MarkAsRead(ctx, rtID, wargaAID, notifOtherRTID, false); err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows when accessing notification from another RT, got %v", err)
	}

	// 6. Warga A marks all as read -> marks only the 1 remaining accessible log (announcement)
	marked, err := repo.MarkAllAsRead(ctx, rtID, wargaAID, false)
	if err != nil {
		t.Fatalf("MarkAllAsRead for Warga A: %v", err)
	}
	if marked != 1 {
		t.Errorf("expected 1 remaining unread log marked as read, got %d", marked)
	}

	unreadA3, _ := repo.GetUnreadCount(ctx, rtID, wargaAID, false)
	if unreadA3 != 0 {
		t.Errorf("expected 0 unread for Warga A, got %d", unreadA3)
	}

	// Warga B still has 2 unread!
	unreadB3, _ := repo.GetUnreadCount(ctx, rtID, wargaBID, false)
	if unreadB3 != 2 {
		t.Errorf("expected Warga B to still have 2 unread, got %d", unreadB3)
	}

	// 7. Officer listing through the personal logs endpoint sees only records addressed to them.
	officerID := notifTestUser(t, pool, "officer", false)
	notifTestMembership(t, pool, officerID, rtID, "pengurus", nil, true)
	officerLogs, totalOff, err := repo.ListLogs(ctx, rtID, officerID, "", "", true, 0, 10)
	if err != nil {
		t.Fatalf("Officer ListLogs: %v", err)
	}
	if totalOff != 0 || len(officerLogs) != 0 {
		t.Errorf("expected officer personal history to exclude other recipients and RT-wide parent logs, got total=%d len=%d", totalOff, len(officerLogs))
	}
}
