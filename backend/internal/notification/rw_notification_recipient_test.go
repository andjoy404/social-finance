package notification_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/database"
	"social-finance/internal/notification"
)

// TestGetRWAndRTPengurusUserIDs_JabatanAllowlist verifies that
// GetRWAndRTPengurusUserIDs strictly enforces the 5-allowed-jabatan allowlist.
//
// Allowed: ketua, wakil_ketua, bendahara, sekretaris, anggota
// Denied: keamanan, sosial, operasional, kebersihan
//
// Critical: users with role='pengurus' AND jabatan='keamanan' must NOT receive.
// The role-based fallback only applies when jabatan IS NULL.
func TestGetRWAndRTPengurusUserIDs_JabatanAllowlist(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	// Create RW
	rwID := createTestRW(t, pool, fmt.Sprintf("RW-Test-%d", n))

	// Create RT within the RW
	rtID := createTestRTInRW(t, pool, rwID, fmt.Sprintf("RT-Test-%d", n))

	// Create users for each jabatan
	type testCase struct {
		jabatan *string
		role    string
		shouldReceive bool
		label     string
	}

	testCases := []testCase{
		// Allowed jabatan-based users:
		// Per specification: ketua, wakil_ketua, bendahara, sekretaris, anggota_rt, anggota_rw, operasional, and sosial.
		// ONLY 'keamanan' and 'kebersihan' are strictly excluded.
		{strPtr("ketua"), "warga", true, "ketua"},
		{strPtr("wakil_ketua"), "warga", true, "wakil_ketua"},
		{strPtr("bendahara"), "warga", true, "bendahara_jabatan"},
		{strPtr("sekretaris"), "warga", true, "sekretaris"},
		{strPtr("anggota_rt"), "warga", true, "anggota_rt"},
		{strPtr("anggota_rw"), "warga", true, "anggota_rw"},
		{strPtr("operasional"), "warga", true, "operasional_warga"},
		{strPtr("operasional"), "pengurus", true, "operasional_pengurus"},
		{strPtr("sosial"), "warga", true, "sosial_warga"},
		{strPtr("sosial"), "pengurus", true, "sosial_pengurus"},

		// Denied jabatan-based users — CRITICAL: strictly excluded
		{strPtr("keamanan"), "warga", false, "keamanan_warga"},
		{strPtr("kebersihan"), "warga", false, "kebersihan_warga"},

		// CRITICAL: role='pengurus' with denied jabatan must NOT receive
		{strPtr("keamanan"), "pengurus", false, "keamanan_pengurus"},
		{strPtr("kebersihan"), "pengurus", false, "kebersihan_pengurus"},

		// CRITICAL: role='bendahara' with denied jabatan must NOT receive
		{strPtr("keamanan"), "bendahara", false, "keamanan_bendahara"},

		// Legacy role-based fallback — jabatan IS NULL
		{nil, "pengurus", true, "legacy_pengurus"},
		{nil, "bendahara", true, "legacy_bendahara"},
		{nil, "warga", false, "legacy_warga_no_jabatan"},

		// Active jabatan-based allowed users with pengurus role
		{strPtr("ketua"), "pengurus", true, "ketua_pengurus"},
		{strPtr("sekretaris"), "pengurus", true, "sekretaris_pengurus"},
	}

	userIDs := make(map[string]string) // label -> userID

	for _, tc := range testCases {
		userID := notifTestUser(t, pool, fmt.Sprintf("%d-%s", n, tc.label), true)
		notifTestMembership(t, pool, userID, rtID, tc.role, tc.jabatan, true)
		userIDs[tc.label] = userID
	}

	// Also create a user in an RT outside the RW — must NOT receive
	otherRW := createTestRW(t, pool, fmt.Sprintf("RW-Other-%d", n))
	otherRT := createTestRTInRW(t, pool, otherRW, fmt.Sprintf("RT-Other-%d", n))
	outsideUserID := notifTestUser(t, pool, fmt.Sprintf("%d-outside-ketua", n), true)
	ketuaJabatan := "ketua"
	notifTestMembership(t, pool, outsideUserID, otherRT, "warga", &ketuaJabatan, true)

	// Create an inactive user — must NOT receive
	inactiveUserID := notifTestUser(t, pool, fmt.Sprintf("%d-inactive-ketua", n), true)
	notifTestMembership(t, pool, inactiveUserID, rtID, "warga", &ketuaJabatan, false)

	// Query recipients
	repo := notification.NewRepository(pool)
	recipientIDs, err := repo.GetRWAndRTPengurusUserIDs(ctx, rwID)
	if err != nil {
		t.Fatalf("GetRWAndRTPengurusUserIDs: %v", err)
	}

	recipientSet := make(map[string]bool)
	for _, id := range recipientIDs {
		recipientSet[id] = true
	}

	// Verify each test case
	for _, tc := range testCases {
		userID := userIDs[tc.label]
		got := recipientSet[userID]
		if got != tc.shouldReceive {
			t.Errorf("%s (jabatan=%v, role=%s): expected receive=%v, got=%v",
				tc.label, tc.jabatan, tc.role, tc.shouldReceive, got)
		}
	}

	// Verify outside RT user is NOT included
	if recipientSet[outsideUserID] {
		t.Error("user in RT outside RW should NOT receive notifications")
	}

	// Verify inactive user is NOT included
	if recipientSet[inactiveUserID] {
		t.Error("inactive membership user should NOT receive notifications")
	}

	// Print summary for debugging
	t.Logf("Total recipients: %d", len(recipientIDs))
	received := []string{}
	for label, userID := range userIDs {
		if recipientSet[userID] {
			received = append(received, label)
		}
	}
	sort.Strings(received)
	t.Logf("Received: %v", received)
}

func TestGetRWAndRTPengurusUserIDs_RWMembership(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	rwID := createTestRW(t, pool, fmt.Sprintf("RW-RWMember-%d", n))

	// RW membership should always receive regardless of role/jabatan
	rwMemberID := notifTestUser(t, pool, fmt.Sprintf("%d-rw-member", n), true)
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO user_rw_memberships (user_id, rw_id, role, is_active)
		 VALUES ($1, $2, 'pengurus', true)`,
		rwMemberID, rwID,
	)
	if err != nil {
		t.Fatalf("create RW membership: %v", err)
	}

	repo := notification.NewRepository(pool)
	recipientIDs, err := repo.GetRWAndRTPengurusUserIDs(ctx, rwID)
	if err != nil {
		t.Fatalf("GetRWAndRTPengurusUserIDs: %v", err)
	}

	recipientSet := make(map[string]bool)
	for _, id := range recipientIDs {
		recipientSet[id] = true
	}

	if !recipientSet[rwMemberID] {
		t.Error("RW membership user should receive notifications")
	}
}

func createTestRW(t *testing.T, pool *database.Pool, name string) string {
	t.Helper()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1) + 1000
	var id string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (rw_number, name, is_active) VALUES ($1, $2, true) RETURNING id`,
		n, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test RW: %v", err)
	}
	return id
}

func createTestRTInRW(t *testing.T, pool *database.Pool, rwID string, name string) string {
	t.Helper()
	ctx := context.Background()
	counter := atomic.AddInt64(&dbRTCounter, 1)
	rwVal := int((time.Now().UnixNano()/1000000)%90000) + 10000
	rtVal := fmt.Sprintf("%06d", (time.Now().UnixNano()/1000)%900000+counter%100000)
	var id string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rw_id, rt, is_active) VALUES ($1, $2, $3, $4, true) RETURNING id`,
		name, rwVal, rwID, rtVal,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test RT in RW: %v", err)
	}
	return id
}

func TestRecipientLogs_CombinedRTAndRW(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	rwID := createTestRW(t, pool, fmt.Sprintf("RW-Comb-%d", n))
	rtID := createTestRTInRW(t, pool, rwID, fmt.Sprintf("RT-Comb-%d", n))

	userID := notifTestUser(t, pool, fmt.Sprintf("%d-comb-recipient", n), true)
	ketuaJabatan := "ketua"
	notifTestMembership(t, pool, userID, rtID, "warga", &ketuaJabatan, true)

	repo := notification.NewRepository(pool)

	// 1. Insert RT notification
	rtDedup := fmt.Sprintf("rt_test:%d", n)
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, $2, 'iuran_unpaid_reminder', $3, 'Tagihan RT', 'Isi tagihan RT', 'SENT', NOW() - INTERVAL '1 minute')
	`, rtID, userID, rtDedup)
	if err != nil {
		t.Fatalf("insert RT log: %v", err)
	}

	// 2. Insert RW notification
	rwDedup := fmt.Sprintf("rw_manual:%s:%d:%s", rwID, n, userID)
	senderID := notifTestUser(t, pool, fmt.Sprintf("%d-rw-sender", n), true)
	ok, err := repo.RecordRWDedupWithStatus(ctx, rwID, userID, "manual_push", rwDedup, "Pengumuman RW", "Isi pengumuman RW", "SENT", map[string]string{"type": "manual_push"}, &senderID)
	if err != nil || !ok {
		t.Fatalf("insert RW log: err=%v, ok=%v", err, ok)
	}

	// 3. Query ListLogs as the recipient
	logs, total, err := repo.ListLogs(ctx, rtID, userID, "", "", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}
	if total != 2 || len(logs) != 2 {
		t.Fatalf("expected 2 combined logs, got total=%d, len=%d", total, len(logs))
	}

	foundRT := false
	foundRW := false
	for _, l := range logs {
		if l.Title == "Tagihan RT" {
			foundRT = true
			if l.Source != "rt" {
				t.Errorf("expected RT log source 'rt', got %q", l.Source)
			}
		}
		if l.Title == "Pengumuman RW" {
			foundRW = true
			if l.Source != "rw" {
				t.Errorf("expected RW log source 'rw', got %q", l.Source)
			}
			if l.RWID == nil || *l.RWID != rwID {
				t.Errorf("expected RWID=%s, got %v", rwID, l.RWID)
			}
		}
	}
	if !foundRT || !foundRW {
		t.Errorf("expected both RT and RW logs, foundRT=%v, foundRW=%v", foundRT, foundRW)
	}

	// 4. Verify unread count and read operations
	unread, err := repo.GetUnreadCount(ctx, rtID, userID, false)
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}
	if unread != 2 {
		t.Fatalf("expected 2 unread, got %d", unread)
	}

	// Find RW log ID
	var rwLogEntry notification.NotificationLogEntry
	for _, l := range logs {
		if l.Title == "Pengumuman RW" {
			rwLogEntry = l
			break
		}
	}

	// Mark RW log as read
	if err := repo.MarkAsRead(ctx, rtID, userID, rwLogEntry.ID, false); err != nil {
		t.Fatalf("MarkAsRead for RW notification: %v", err)
	}

	unreadAfter, err := repo.GetUnreadCount(ctx, rtID, userID, false)
	if err != nil {
		t.Fatalf("GetUnreadCount after mark read: %v", err)
	}
	if unreadAfter != 1 {
		t.Fatalf("expected 1 unread, got %d", unreadAfter)
	}

	// Mark all as read
	marked, err := repo.MarkAllAsRead(ctx, rtID, userID, false)
	if err != nil {
		t.Fatalf("MarkAllAsRead: %v", err)
	}
	if marked != 1 {
		t.Fatalf("expected 1 remaining marked, got %d", marked)
	}

	unreadFinal, err := repo.GetUnreadCount(ctx, rtID, userID, false)
	if err != nil {
		t.Fatalf("GetUnreadCount final: %v", err)
	}
	if unreadFinal != 0 {
		t.Fatalf("expected 0 unread, got %d", unreadFinal)
	}
}

func TestListRWMyLogs_GroupingAndCounts(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	rwID := createTestRW(t, pool, fmt.Sprintf("RW-MyLogs-%d", n))
	senderID := notifTestUser(t, pool, fmt.Sprintf("%d-sender-ketua", n), true)
	rec1 := notifTestUser(t, pool, fmt.Sprintf("%d-rec1", n), true)
	rec2 := notifTestUser(t, pool, fmt.Sprintf("%d-rec2", n), true)
	rec3 := notifTestUser(t, pool, fmt.Sprintf("%d-rec3", n), true)

	repo := notification.NewRepository(pool)

	batchID := fmt.Sprintf("rw_manual:%s:%d", rwID, n)
	_, _ = repo.RecordRWDedupWithStatus(ctx, rwID, rec1, "manual_push", fmt.Sprintf("%s:%s", batchID, rec1), "Edaran RW", "Isi edaran", "SENT", nil, &senderID)
	_, _ = repo.RecordRWDedupWithStatus(ctx, rwID, rec2, "manual_push", fmt.Sprintf("%s:%s", batchID, rec2), "Edaran RW", "Isi edaran", "SENT", nil, &senderID)
	_, _ = repo.RecordRWDedupWithStatus(ctx, rwID, rec3, "manual_push", fmt.Sprintf("%s:%s", batchID, rec3), "Edaran RW", "Isi edaran", "FAILED", nil, &senderID)

	myLogs, total, err := repo.ListRWMyLogs(ctx, rwID, senderID, "", 0, 10)
	if err != nil {
		t.Fatalf("ListRWMyLogs: %v", err)
	}
	if total != 1 || len(myLogs) != 1 {
		t.Fatalf("expected 1 grouped batch, got total=%d, len=%d", total, len(myLogs))
	}

	entry := myLogs[0]
	if entry.RecipientsCount != 3 {
		t.Errorf("expected RecipientsCount=3, got %d", entry.RecipientsCount)
	}
	if entry.SentCount != 2 {
		t.Errorf("expected SentCount=2, got %d", entry.SentCount)
	}
	if entry.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", entry.FailedCount)
	}
	if entry.Status != "PARTIAL" {
		t.Errorf("expected status PARTIAL, got %s", entry.Status)
	}
	if entry.Source != "rw" {
		t.Errorf("expected Source=rw, got %s", entry.Source)
	}
	if entry.ActivityRelation != "sent_by_me" {
		t.Errorf("expected ActivityRelation=sent_by_me, got %s", entry.ActivityRelation)
	}
}

func TestListManagementHistory_CombinedRTAndRW(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	rwID := createTestRW(t, pool, fmt.Sprintf("RW-Mgmt-%d", n))
	rtID := createTestRTInRW(t, pool, rwID, fmt.Sprintf("RT-Mgmt-%d", n))

	userID := notifTestUser(t, pool, fmt.Sprintf("%d-mgmt-ketua", n), true)
	ketuaJabatan := "ketua"
	notifTestMembership(t, pool, userID, rtID, "pengurus", &ketuaJabatan, true)

	repo := notification.NewRepository(pool)

	// 1. Insert RT notification to user
	rtDedup := fmt.Sprintf("rt_mgmt_test:%d", n)
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, $2, 'iuran_unpaid_reminder', $3, 'Tagihan RT Untuk Pengurus', 'Isi tagihan', 'SENT', NOW() - INTERVAL '2 minute')
	`, rtID, userID, rtDedup)
	if err != nil {
		t.Fatalf("insert RT log: %v", err)
	}

	// 2. Insert RW notification sent to this RT user
	rwDedup := fmt.Sprintf("rw_manual:%s:%d:%s", rwID, n, userID)
	senderID := notifTestUser(t, pool, fmt.Sprintf("%d-rw-mgmt-sender", n), true)
	ok, err := repo.RecordRWDedupWithStatus(ctx, rwID, userID, "manual_push", rwDedup, "Pengumuman RW Untuk RT", "Isi RW", "SENT", map[string]string{"type": "manual_push"}, &senderID)
	if err != nil || !ok {
		t.Fatalf("insert RW log: err=%v, ok=%v", err, ok)
	}

	// 3. Query ListManagementHistory(mine) as the RT user
	entries, total, err := repo.ListManagementHistory(ctx, rtID, userID, notification.HistoryCategoryMine, 0, 10)
	if err != nil {
		t.Fatalf("ListManagementHistory mine: %v", err)
	}
	if total < 2 || len(entries) < 2 {
		t.Fatalf("expected at least 2 entries in mine history, got total=%d, len=%d", total, len(entries))
	}

	foundRT := false
	foundRW := false
	var rwEntryID string
	for _, e := range entries {
		if e.Title == "Tagihan RT Untuk Pengurus" {
			foundRT = true
			if e.Source != "rt" {
				t.Errorf("expected RT source 'rt', got %q", e.Source)
			}
		}
		if e.Title == "Pengumuman RW Untuk RT" {
			foundRW = true
			rwEntryID = e.ID
			if e.Source != "rw" {
				t.Errorf("expected RW source 'rw', got %q", e.Source)
			}
			if e.IsRead {
				t.Errorf("expected unread initially")
			}
		}
	}
	if !foundRT {
		t.Errorf("did not find RT entry in ListManagementHistory(mine)")
	}
	if !foundRW {
		t.Errorf("did not find RW entry in ListManagementHistory(mine)")
	}

	// 4. Mark RW notification as read and verify
	if rwEntryID != "" {
		if err := repo.MarkAsRead(ctx, rtID, userID, rwEntryID, true); err != nil {
			t.Fatalf("MarkAsRead: %v", err)
		}

		entriesAfterRead, _, err := repo.ListManagementHistory(ctx, rtID, userID, notification.HistoryCategoryMine, 0, 10)
		if err != nil {
			t.Fatalf("ListManagementHistory after read: %v", err)
		}
		for _, e := range entriesAfterRead {
			if e.ID == rwEntryID {
				if !e.IsRead {
					t.Errorf("expected RW notification to be marked as read, got isRead=false")
				}
				if e.ReadAt == nil {
					t.Errorf("expected ReadAt timestamp to be set")
				}
			}
		}
	}
}

func TestAnggotaRW_NotificationFlow_EndToEnd(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	rwID := createTestRW(t, pool, fmt.Sprintf("RW-AnggotaRW-%d", n))
	rtID := createTestRTInRW(t, pool, rwID, fmt.Sprintf("RT-AnggotaRW-%d", n))

	repo := notification.NewRepository(pool)

	// 1. Create Anggota RW, Keamanan, and Other users in this RT
	anggotaRWUser := notifTestUser(t, pool, fmt.Sprintf("%d-anggotarw", n), true)
	notifTestMembership(t, pool, anggotaRWUser, rtID, "warga", strPtr("anggota_rw"), true)

	keamananUser := notifTestUser(t, pool, fmt.Sprintf("%d-keamanan", n), true)
	notifTestMembership(t, pool, keamananUser, rtID, "warga", strPtr("keamanan"), true)

	otherUser := notifTestUser(t, pool, fmt.Sprintf("%d-other", n), true)
	notifTestMembership(t, pool, otherUser, rtID, "warga", nil, true)

	// 2. Test GetRWAndRTPengurusUserIDs: anggota_rw MUST be included, keamanan MUST NOT be included
	recipients, err := repo.GetRWAndRTPengurusUserIDs(ctx, rwID)
	if err != nil {
		t.Fatalf("GetRWAndRTPengurusUserIDs: %v", err)
	}
	recipMap := make(map[string]bool)
	for _, id := range recipients {
		recipMap[id] = true
	}
	if !recipMap[anggotaRWUser] {
		t.Errorf("expected anggota_rw (%s) to be in recipient list, got %v", anggotaRWUser, recipients)
	}
	if recipMap[keamananUser] {
		t.Errorf("keamanan (%s) must NOT be in RW recipient list", keamananUser)
	}

	// 3. Send RW broadcast notification to anggota_rw
	senderID := notifTestUser(t, pool, fmt.Sprintf("%d-rw-broadcaster", n), true)
	dedupKey := fmt.Sprintf("rw_manual:%s:%d:%s", rwID, n, anggotaRWUser)
	ok, err := repo.RecordRWDedupWithStatus(ctx, rwID, anggotaRWUser, "manual_push", dedupKey, "Pengumuman RW Spesial", "Pesan dari Ketua RW", "SENT", map[string]string{"type": "manual_push"}, &senderID)
	if err != nil || !ok {
		t.Fatalf("RecordRWDedupWithStatus: ok=%v, err=%v", ok, err)
	}

	// 4. anggota_rw queries Riwayat Saya (ListLogs)
	logs, total, err := repo.ListLogs(ctx, rtID, anggotaRWUser, "", "sent", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs for anggota_rw: %v", err)
	}
	if total < 1 || len(logs) < 1 {
		t.Fatalf("expected at least 1 log in Riwayat Saya for anggota_rw, got total=%d, len=%d", total, len(logs))
	}

	var targetLogID string
	for _, l := range logs {
		if l.Title == "Pengumuman RW Spesial" {
			targetLogID = l.ID
			if l.IsRead {
				t.Errorf("expected is_read=false initially")
			}
		}
	}
	if targetLogID == "" {
		t.Fatalf("did not find target notification in anggota_rw logs")
	}

	// 5. Another user cannot mark anggota_rw's notification as read
	otherErr := repo.MarkAsRead(ctx, rtID, otherUser, targetLogID, false)
	if !errors.Is(otherErr, notification.ErrForbidden) && !errors.Is(otherErr, sql.ErrNoRows) {
		t.Errorf("expected ErrForbidden or ErrNoRows when other user marks notification, got %v", otherErr)
	}

	// 6. anggota_rw marks it as read
	if err := repo.MarkAsRead(ctx, rtID, anggotaRWUser, targetLogID, false); err != nil {
		t.Fatalf("MarkAsRead by anggota_rw: %v", err)
	}

	// 7. Verify status updated to is_read=true
	logsAfter, _, err := repo.ListLogs(ctx, rtID, anggotaRWUser, "", "sent", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs after read: %v", err)
	}
	for _, l := range logsAfter {
		if l.ID == targetLogID {
			if !l.IsRead {
				t.Errorf("expected is_read=true after MarkAsRead, got false")
			}
			if l.ReadAt == nil {
				t.Errorf("expected read_at timestamp to be non-nil")
			}
		}
	}

	// 8. Verify Keamanan receives their own special notification normally
	keamananDedup := fmt.Sprintf("patrol_alert:%s:%d", keamananUser, n)
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at)
		VALUES ($1, $2, 'patrol_alert', $3, 'Laporan Keamanan Baru', 'Patroli selesai', 'SENT', NOW())
	`, rtID, keamananUser, keamananDedup)
	if err != nil {
		t.Fatalf("insert security notification: %v", err)
	}

	keamananLogs, kTotal, err := repo.ListLogs(ctx, rtID, keamananUser, "", "sent", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs for keamanan: %v", err)
	}
	if kTotal < 1 || len(keamananLogs) < 1 {
		t.Errorf("keamanan must still receive their own notifications, got total=%d", kTotal)
	}
}

func TestNotifyRWCashIn_And_CashOut_EndToEnd(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	rwID := createTestRW(t, pool, fmt.Sprintf("RW-KasNotif-%d", n))
	rtID := createTestRTInRW(t, pool, rwID, fmt.Sprintf("RT-KasNotif-%d", n))

	// 1. Create Ketua RT (allowed recipient) and Keamanan (excluded)
	ketuaUser := notifTestUser(t, pool, fmt.Sprintf("%d-kas-ketua", n), true)
	notifTestMembership(t, pool, ketuaUser, rtID, "warga", strPtr("ketua"), true)

	keamananUser := notifTestUser(t, pool, fmt.Sprintf("%d-kas-keamanan", n), true)
	notifTestMembership(t, pool, keamananUser, rtID, "warga", strPtr("keamanan"), true)

	// 2. Insert active template for rw_cash_in with image_url and variables
	testImageURL := "/api/v1/notifications/images/notifications/rw-kas/" + rwID + "/test_in.png"
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO notification_templates (
			id, rw_id, name, event_type, channel, title, body, is_active, is_default, image_url, variables, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, 'Kas RW Masuk Aktif', 'rw_cash_in', 'push',
			'Kas Masuk {{amount}}', 'Diterima {{amount}} pada {{location}}: {{description}}',
			true, false, $2, '["amount", "location", "description"]'::jsonb, NOW(), NOW()
		)
	`, rwID, testImageURL)
	if err != nil {
		t.Fatalf("insert active rw_cash_in template: %v", err)
	}

	repo := notification.NewRepository(pool)
	fcm := notification.NewMockPushProvider()
	svc := notification.NewService(repo, fcm)

	txID := fmt.Sprintf("tx-cash-in-%d", n)

	// 3. Call NotifyRWCashIn
	svc.NotifyRWCashIn(ctx, rwID, txID, "Rp 1.000.000", "Iuran RT Sukses", "Kas Utama RW")
	time.Sleep(200 * time.Millisecond)

	// 4. Verify Ketua received notification with interpolated values, image_url, and deep_link
	logs, total, err := repo.ListLogs(ctx, rtID, ketuaUser, "", "sent", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs for ketua: %v", err)
	}
	if total < 1 || len(logs) < 1 {
		t.Fatalf("expected at least 1 log for ketua, got %d", total)
	}

	var foundLog *notification.NotificationLogEntry
	for _, l := range logs {
		if l.Title == "Kas Masuk Rp 1.000.000" {
			foundLog = &l
			break
		}
	}
	if foundLog == nil {
		t.Fatalf("did not find expected Kas Masuk log in ketua logs: %+v", logs)
	}
	if !strings.Contains(foundLog.Body, "Diterima Rp 1.000.000 pada Kas Utama RW: Iuran RT Sukses") {
		t.Errorf("unexpected body interpolation: %q", foundLog.Body)
	}
	expectedDeepLink := fmt.Sprintf("/rw/%s/kas?tx_id=%s", rwID, txID)
	if foundLog.Payload["deep_link"] != expectedDeepLink {
		t.Errorf("expected deep_link %q, got %q", expectedDeepLink, foundLog.Payload["deep_link"])
	}
	if foundLog.Payload["image_url"] != testImageURL {
		t.Errorf("expected image_url %q, got %q", testImageURL, foundLog.Payload["image_url"])
	}

	// 5. Verify Keamanan did NOT receive the notification
	kLogs, _, err := repo.ListLogs(ctx, rtID, keamananUser, "", "sent", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs for keamanan: %v", err)
	}
	for _, l := range kLogs {
		if strings.Contains(l.Title, "Kas Masuk") {
			t.Errorf("keamanan must NOT receive Kas RW Masuk notification: %+v", l)
		}
	}

	// 6. Test Idempotency: Calling NotifyRWCashIn with same txID does NOT create duplicate log
	svc.NotifyRWCashIn(ctx, rwID, txID, "Rp 1.000.000", "Duplicate Attempt", "Kas Utama RW")
	time.Sleep(200 * time.Millisecond)
	logsAfterDup, _, err := repo.ListLogs(ctx, rtID, ketuaUser, "", "sent", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs after dup: %v", err)
	}
	count := 0
	for _, l := range logsAfterDup {
		if l.Title == "Kas Masuk Rp 1.000.000" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 log due to dedup, got %d", count)
	}

	// 7. Test NotifyRWCashOut (using fallback default template since no custom template active)
	txOutID := fmt.Sprintf("tx-cash-out-%d", n)
	svc.NotifyRWCashOut(ctx, rwID, txOutID, "Rp 250.000", "Pembelian Lampu Jalan", "Kas Operasional RW", "Operasional Lingkungan")
	time.Sleep(200 * time.Millisecond)

	logsOut, _, err := repo.ListLogs(ctx, rtID, ketuaUser, "", "sent", false, 0, 10)
	if err != nil {
		t.Fatalf("ListLogs for cash out: %v", err)
	}
	var foundOutLog *notification.NotificationLogEntry
	for _, l := range logsOut {
		if strings.Contains(l.Title, "Kas Keluar") || strings.Contains(l.Title, "Kas RW") {
			foundOutLog = &l
			break
		}
	}
	if foundOutLog == nil {
		t.Fatalf("did not find expected Kas RW Keluar log: %+v", logsOut)
	}
	expectedOutLink := fmt.Sprintf("/rw/%s/kas?tx_id=%s", rwID, txOutID)
	if foundOutLog.Payload["deep_link"] != expectedOutLink {
		t.Errorf("expected cash out deep link %q, got %q", expectedOutLink, foundOutLog.Payload["deep_link"])
	}
}

// TestRWKasNotificationRecipients_ExactSpecification tests the full recipient matrix:
// - Keamanan does NOT receive
// - Kebersihan does NOT receive
// - Sosial DOES receive
// - Anggota RW DOES receive
// - Anggota RW who is also Ketua RT on a non-SF RT DOES receive
// - No recipients from other RW
// - No duplicate recipients
func TestRWKasNotificationRecipients_ExactSpecification(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	// RW A (Target) and RW B (Other RW)
	rwA := createTestRW(t, pool, fmt.Sprintf("RW-SpecA-%d", n))
	rwB := createTestRW(t, pool, fmt.Sprintf("RW-SpecB-%d", n))

	// RT in RW A
	rtA1 := createTestRTInRW(t, pool, rwA, fmt.Sprintf("RT-SpecA1-%d", n))
	// RT in RW B
	rtB1 := createTestRTInRW(t, pool, rwB, fmt.Sprintf("RT-SpecB1-%d", n))
	// Non-SF RT (no RW affiliation, rw_id IS NULL)
	nonSFRT := createTestRTWithoutRW(t, pool, fmt.Sprintf("RT-NonSF-%d", n))

	// 1. Keamanan in RT A1 (Must NOT receive)
	keamananUser := notifTestUser(t, pool, fmt.Sprintf("%d-keamanan-spec", n), true)
	keamananJab := "keamanan"
	notifTestMembership(t, pool, keamananUser, rtA1, "warga", &keamananJab, true)

	// 2. Kebersihan in RT A1 (Must NOT receive)
	kebersihanUser := notifTestUser(t, pool, fmt.Sprintf("%d-kebersihan-spec", n), true)
	kebersihanJab := "kebersihan"
	notifTestMembership(t, pool, kebersihanUser, rtA1, "warga", &kebersihanJab, true)

	// 3. Sosial in RT A1 (MUST receive)
	sosialUser := notifTestUser(t, pool, fmt.Sprintf("%d-sosial-spec", n), true)
	sosialJab := "sosial"
	notifTestMembership(t, pool, sosialUser, rtA1, "warga", &sosialJab, true)

	// 4. Ketua RT A1 (MUST receive)
	ketuaUser := notifTestUser(t, pool, fmt.Sprintf("%d-ketua-spec", n), true)
	ketuaJab := "ketua"
	notifTestMembership(t, pool, ketuaUser, rtA1, "warga", &ketuaJab, true)

	// 5. Anggota RW (in user_rw_memberships for RW A) (MUST receive)
	anggotaRWUser := notifTestUser(t, pool, fmt.Sprintf("%d-anggotarw-spec", n), true)
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, is_active)
		 VALUES ($1, $2, 'pengurus', 'anggota', true)`,
		anggotaRWUser, rwA,
	)
	if err != nil {
		t.Fatalf("create anggota_rw membership: %v", err)
	}

	// 6. Anggota RW who is also Ketua RT on a Non-SF RT (MUST receive because RW A membership is valid)
	dualUser := notifTestUser(t, pool, fmt.Sprintf("%d-dual-spec", n), true)
	_, err = pool.Raw().ExecContext(ctx,
		`INSERT INTO user_rw_memberships (user_id, rw_id, role, is_active)
		 VALUES ($1, $2, 'pengurus', true)`,
		dualUser, rwA,
	)
	if err != nil {
		t.Fatalf("create dual user RW membership: %v", err)
	}
	// Also Ketua in non-SF RT
	notifTestMembership(t, pool, dualUser, nonSFRT, "warga", &ketuaJab, true)

	// 7. Pengurus in RW B (Must NOT receive)
	rwBOfficial := notifTestUser(t, pool, fmt.Sprintf("%d-rwB-official", n), true)
	notifTestMembership(t, pool, rwBOfficial, rtB1, "warga", &ketuaJab, true)

	// Query recipients for RW A
	repo := notification.NewRepository(pool)
	recipients, err := repo.GetRWAndRTPengurusUserIDs(ctx, rwA)
	if err != nil {
		t.Fatalf("GetRWAndRTPengurusUserIDs: %v", err)
	}

	recipientMap := make(map[string]bool)
	hasDuplicates := false
	for _, id := range recipients {
		if recipientMap[id] {
			hasDuplicates = true
		}
		recipientMap[id] = true
	}

	// Verification 1: Keamanan tidak menerima
	if recipientMap[keamananUser] {
		t.Errorf("Keamanan user %s MUST NOT receive notification", keamananUser)
	}

	// Verification 2: Kebersihan tidak menerima
	if recipientMap[kebersihanUser] {
		t.Errorf("Kebersihan user %s MUST NOT receive notification", kebersihanUser)
	}

	// Verification 3: Sosial menerima
	if !recipientMap[sosialUser] {
		t.Errorf("Sosial user %s MUST receive notification", sosialUser)
	}

	// Verification 4: Anggota RW menerima
	if !recipientMap[anggotaRWUser] {
		t.Errorf("Anggota RW user %s MUST receive notification", anggotaRWUser)
	}

	// Verification 5: Anggota RW yang juga Ketua RT non-SF menerima
	if !recipientMap[dualUser] {
		t.Errorf("Anggota RW with non-SF RT user %s MUST receive notification", dualUser)
	}

	// Verification 6: Tidak ada penerima dari RW lain
	if recipientMap[rwBOfficial] {
		t.Errorf("Official from RW B %s MUST NOT receive notification for RW A", rwBOfficial)
	}

	// Verification 7: Tidak ada duplikasi penerima
	if hasDuplicates || len(recipients) != len(recipientMap) {
		t.Errorf("Recipients contains duplicates: total %d vs unique %d", len(recipients), len(recipientMap))
	}
}

// createTestRTWithoutRW creates an RT without RW affiliation (rw_id IS NULL)
func createTestRTWithoutRW(t *testing.T, pool *database.Pool, name string) string {
	t.Helper()
	ctx := context.Background()
	counter := atomic.AddInt64(&dbRTCounter, 1)
	rwVal := int((time.Now().UnixNano()/1000000)%90000) + 10000
	rtVal := fmt.Sprintf("%06d", (time.Now().UnixNano()/1000)%900000+counter%100000)
	var id string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		name, rwVal, rtVal,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test RT without RW: %v", err)
	}
	return id
}
