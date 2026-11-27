package notification_test

import (
	"context"
	"sync"
	"testing"

	"social-finance/internal/notification"
)

// TestNotificationReadReceipt_ConcurrentAndIdempotent verifies directly against PostgreSQL
// that notification_reads is the authoritative read state:
// - New notification starts unread.
// - Repeated and concurrent MarkAsRead requests succeed idempotently without duplicate rows.
// - Reading one notification does not clear unrelated unread notifications.
// - Cross-user MarkAsRead attempts return ErrForbidden and create zero rows.
func TestNotificationReadReceipt_ConcurrentAndIdempotent(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userA := notifTestUser(t, pool, "rr-user-a", true)
	notifTestMembership(t, pool, userA, rtID, "warga", nil, true)
	userB := notifTestUser(t, pool, "rr-user-b", true)
	notifTestMembership(t, pool, userB, rtID, "warga", nil, true)

	var notif1, notif2 string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status)
		VALUES ($1, $2, 'paket_received', $3, 'Paket Tiba', 'Paket 1 di pos', 'SENT')
		RETURNING id::text
	`, rtID, userA, "rr_conc:1:"+userA).Scan(&notif1); err != nil {
		t.Fatalf("insert notif1: %v", err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status)
		VALUES ($1, $2, 'correction_submitted', $3, 'Koreksi Absen', 'Koreksi diajukan', 'SENT')
		RETURNING id::text
	`, rtID, userA, "rr_conc:2:"+userA).Scan(&notif2); err != nil {
		t.Fatalf("insert notif2: %v", err)
	}

	// 1. Both start unread.
	unread, err := repo.GetUnreadCount(ctx, rtID, userA, false)
	if err != nil {
		t.Fatalf("initial unread count: %v", err)
	}
	if unread != 2 {
		t.Fatalf("initial unread = %d, want 2", unread)
	}

	// 2. Cross-user read attempt is forbidden and creates no receipt.
	if err := repo.MarkAsRead(ctx, rtID, userB, notif1, false); err != notification.ErrForbidden {
		t.Fatalf("cross-user MarkAsRead = %v, want ErrForbidden", err)
	}
	var crossCount int
	if err := pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notification_reads WHERE notification_id = $1`, notif1,
	).Scan(&crossCount); err != nil {
		t.Fatalf("count receipts after forbidden: %v", err)
	}
	if crossCount != 0 {
		t.Fatalf("forbidden read created %d rows, want 0", crossCount)
	}

	// 3. Concurrent MarkAsRead requests for notif1 create exactly 1 row without errors.
	const workers = 10
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.MarkAsRead(ctx, rtID, userA, notif1, false)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent MarkAsRead failed: %v", err)
		}
	}

	// 4. Repeated sequential call is also idempotent.
	if err := repo.MarkAsRead(ctx, rtID, userA, notif1, false); err != nil {
		t.Fatalf("repeated MarkAsRead failed: %v", err)
	}

	var receiptRows int
	if err := pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notification_reads WHERE notification_id = $1 AND user_id = $2`,
		notif1, userA,
	).Scan(&receiptRows); err != nil {
		t.Fatalf("count notification_reads: %v", err)
	}
	if receiptRows != 1 {
		t.Fatalf("notification_reads rows = %d, want 1", receiptRows)
	}

	// 5. Reading notif1 must leave notif2 unread (unread count == 1).
	unreadAfter, err := repo.GetUnreadCount(ctx, rtID, userA, false)
	if err != nil {
		t.Fatalf("unread count after read: %v", err)
	}
	if unreadAfter != 1 {
		t.Fatalf("unread after marking 1 of 2 = %d, want 1", unreadAfter)
	}
}
