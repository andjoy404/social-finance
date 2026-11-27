package notification_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/notification"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRTNotificationIsolation_RT002DoesNotReachRT001OrRT003 verifies the critical P0 requirement:
// When RT 002 sends a notification (manual push, waste truck, etc.), residents of RT 001 and RT 003
// in the same RW must NEVER receive device tokens, FCM dispatches, or personal notification logs.
func TestRTNotificationIsolation_RT002DoesNotReachRT001OrRT003(t *testing.T) {
	pool := notifTestPool(t)
	ctx := context.Background()

	// 1. Create a shared RW
	rwNumber := int(time.Now().UnixNano() % 100000000)
	var rwID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (rw_number, name, is_active) VALUES ($1, $2, true) RETURNING id`,
		rwNumber, fmt.Sprintf("RW Isolation Test %d", rwNumber),
	).Scan(&rwID)
	require.NoError(t, err)

	// 2. Create three RTs in the same RW: RT 001, RT 002, RT 003
	createRT := func(code, name string) string {
		var id string
		err := pool.Raw().QueryRowContext(ctx,
			`INSERT INTO rts (id, name, rw, rw_id, rt, is_active)
			 VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id`,
			name, rwNumber, rwID, code,
		).Scan(&id)
		require.NoError(t, err)
		return id
	}

	rt001ID := createRT("001", "RT 001")
	rt002ID := createRT("002", "RT 002")
	rt003ID := createRT("003", "RT 003")

	// 3. Create users in each RT with active devices
	// RT 001 Warga
	u1ID := notifTestUserInPool(t, pool, fmt.Sprintf("warga_rt001_%d@local.com", rwNumber), true)
	notifTestMembershipInPool(t, pool, u1ID, rt001ID, auth.RoleWarga, "anggota_rt", true)
	notifTestDeviceInPool(t, pool, u1ID, "token-rt001-warga", true)

	// RT 002 Ketua (Sender) and Warga
	u2SenderID := notifTestUserInPool(t, pool, fmt.Sprintf("ketua_rt002_%d@local.com", rwNumber), true)
	notifTestMembershipInPool(t, pool, u2SenderID, rt002ID, auth.RolePengurus, "ketua", true)
	notifTestDeviceInPool(t, pool, u2SenderID, "token-rt002-sender", true)

	u2WargaID := notifTestUserInPool(t, pool, fmt.Sprintf("warga_rt002_%d@local.com", rwNumber), true)
	notifTestMembershipInPool(t, pool, u2WargaID, rt002ID, auth.RoleWarga, "anggota_rt", true)
	notifTestDeviceInPool(t, pool, u2WargaID, "token-rt002-warga", true)

	// RT 003 Warga
	u3ID := notifTestUserInPool(t, pool, fmt.Sprintf("warga_rt003_%d@local.com", rwNumber), true)
	notifTestMembershipInPool(t, pool, u3ID, rt003ID, auth.RoleWarga, "anggota_rt", true)
	notifTestDeviceInPool(t, pool, u3ID, "token-rt003-warga", true)

	repo := notification.NewRepository(pool)

	// 4. Test GetRTBroadcastRecipients for RT 002
	recipients, err := repo.GetRTBroadcastRecipients(ctx, rt002ID)
	require.NoError(t, err)

	// Assertions:
	// - RT 002 recipients MUST contain u2SenderID and u2WargaID
	// - RT 002 recipients MUST NOT contain u1ID or u3ID!
	recipientMap := make(map[string]bool)
	var collectedTokens []string
	for _, r := range recipients {
		recipientMap[r.UserID] = true
		for _, tok := range r.Tokens {
			if tok != "" {
				collectedTokens = append(collectedTokens, tok)
			}
		}
	}

	assert.True(t, recipientMap[u2SenderID], "RT 002 sender must be in recipients")
	assert.True(t, recipientMap[u2WargaID], "RT 002 warga must be in recipients")
	assert.False(t, recipientMap[u1ID], "CRITICAL: RT 001 resident MUST NOT be in RT 002 broadcast recipients!")
	assert.False(t, recipientMap[u3ID], "CRITICAL: RT 003 resident MUST NOT be in RT 002 broadcast recipients!")

	assert.Contains(t, collectedTokens, "token-rt002-warga")
	assert.NotContains(t, collectedTokens, "token-rt001-warga", "CRITICAL: RT 001 token must not be included")
	assert.NotContains(t, collectedTokens, "token-rt003-warga", "CRITICAL: RT 003 token must not be included")

	// 5. Verify Personal Log Isolation: ListLogs
	// Simulate manual push parent and recipient-specific delivery for RT 002 warga
	manualPayload := map[string]string{"title": "Pengumuman RT 002", "body": "Ini pesan khusus RT 002"}
	manualKey := fmt.Sprintf("manual_push:%s:%d:test", rt002ID, atomic.AddInt64(&dbRTCounter, 1))
	created, err := repo.CheckAndRecordDedup(ctx, rt002ID, u2SenderID, notification.EventManualPush, manualKey, manualPayload["title"], manualPayload["body"], manualPayload)
	require.NoError(t, err)
	require.True(t, created)
	manualBroadcastID := manualPayload["notification_id"]
	require.NotEmpty(t, manualBroadcastID)

	err = repo.RecordBroadcastRecipientDelivery(ctx, rt002ID, u2SenderID, u2WargaID, notification.EventManualPush, manualBroadcastID, manualPayload["title"], manualPayload["body"], manualPayload, "SENT")
	require.NoError(t, err)

	// Check RT 001 user's log
	logs1, _, err := repo.ListLogs(ctx, rt001ID, u1ID, "", "", false, 0, 10)
	require.NoError(t, err)
	for _, l := range logs1 {
		assert.NotEqual(t, manualBroadcastID, l.ID, "RT 001 resident must not see RT 002 notification log")
	}

	// Check RT 003 user's log
	logs3, _, err := repo.ListLogs(ctx, rt003ID, u3ID, "", "", false, 0, 10)
	require.NoError(t, err)
	for _, l := range logs3 {
		assert.NotEqual(t, manualBroadcastID, l.ID, "RT 003 resident must not see RT 002 notification log")
	}

	// Check RT 002 user's log
	logs2, _, err := repo.ListLogs(ctx, rt002ID, u2WargaID, "", "", false, 0, 10)
	require.NoError(t, err)
	found := false
	for _, l := range logs2 {
		if l.Title == "Pengumuman RT 002" {
			found = true
			break
		}
	}
	assert.True(t, found, "RT 002 resident must see RT 002 notification log in their personal inbox")

	// 6. Verify Additional Task 1: ListManagementHistory HistoryCategoryMine Read status
	// When sender views their own sent manual push in HistoryCategoryMine
	mineLogs, total, err := repo.ListManagementHistory(ctx, rt002ID, u2SenderID, notification.HistoryCategoryMine, 0, 10)
	require.NoError(t, err)
	assert.True(t, total >= 1)

	var targetLog *notification.NotificationLogEntry
	for _, ml := range mineLogs {
		if ml.ID == manualBroadcastID {
			targetLog = &ml
			break
		}
	}
	require.NotNil(t, targetLog, "Sender must see their own RECORDED notification in HistoryCategoryMine")
	assert.False(t, targetLog.IsRead, "Initially must be unread (is_read=false)")

	// Now mark as read
	err = repo.MarkAsRead(ctx, rt002ID, u2SenderID, manualBroadcastID, true)
	require.NoError(t, err)

	// Fetch again from ListManagementHistory
	mineLogsAfter, _, err := repo.ListManagementHistory(ctx, rt002ID, u2SenderID, notification.HistoryCategoryMine, 0, 10)
	require.NoError(t, err)
	var targetLogAfter *notification.NotificationLogEntry
	for _, ml := range mineLogsAfter {
		if ml.ID == manualBroadcastID {
			targetLogAfter = &ml
			break
		}
	}
	require.NotNil(t, targetLogAfter)
	assert.True(t, targetLogAfter.IsRead, "CRITICAL: After mark as read, RECORDED notification in HistoryCategoryMine must reflect is_read=true!")
}
