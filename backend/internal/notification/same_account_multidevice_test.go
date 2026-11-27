package notification_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"
)

// TestSameAccountMultiDevice_WebSendToAndroid verifies all critical scenarios
// when the same Ketua RT account is logged in on Web and Android (and/or
// multiple active devices) and sends notifications from Web:
//
//  1. Sender Ketua RT (Web) -> target includes Ketua RT -> Ketua RT's Android
//     device receives the push notification (never excluded by sender_user_id).
//  2. Same Ketua RT with multiple active devices -> all active devices receive
//     the push, with zero duplicates per token.
//  3. Different account (e.g. Wakil Ketua / Bendahara) sends -> Ketua RT's
//     Android device also receives push.
//  4. Non-recipient user (other RT or inactive member) does NOT receive push.
//  5. Notification history, read status, and unread count remain accurate for
//     same-account sender/recipient.
//  6. RW manual push when sender is also in target recipients delivers push to
//     sender's active Android device without duplicates.
func TestSameAccountMultiDevice_WebSendToAndroid(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	db := pool.Raw()
	suffix := time.Now().UnixNano()

	// Create isolated RT A (target RT) and RT B (other RT for isolation check).
	var rwID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO rws (rw_number, name, is_active)
		VALUES ($1, $2, true) RETURNING id
	`, int(500+(suffix%400)), fmt.Sprintf("RW MultiDev %d", suffix)).Scan(&rwID)
	if err != nil {
		t.Fatalf("create rw: %v", err)
	}

	var rtA, rtB string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO rts (name, rw, rt, address, head_name, is_active, rw_id)
		VALUES ($1, 99, $2, 'Addr A', 'Ketua A', true, $3) RETURNING id
	`, fmt.Sprintf("RT-A-%d", suffix), fmt.Sprintf("A%d", suffix%100000), rwID).Scan(&rtA); err != nil {
		t.Fatalf("create rtA: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO rts (name, rw, rt, address, head_name, is_active, rw_id)
		VALUES ($1, 99, $2, 'Addr B', 'Ketua B', true, $3) RETURNING id
	`, fmt.Sprintf("RT-B-%d", suffix), fmt.Sprintf("B%d", suffix%100000), rwID).Scan(&rtB); err != nil {
		t.Fatalf("create rtB: %v", err)
	}

	ketuaID := notifTestUserInPool(t, pool, fmt.Sprintf("ketua-multi-%d@local.com", suffix), true)
	wakilID := notifTestUserInPool(t, pool, fmt.Sprintf("wakil-multi-%d@local.com", suffix), true)
	wargaAID := notifTestUserInPool(t, pool, fmt.Sprintf("warga-a-multi-%d@local.com", suffix), true)
	inactiveAID := notifTestUserInPool(t, pool, fmt.Sprintf("inactive-a-multi-%d@local.com", suffix), true)
	otherRTUserID := notifTestUserInPool(t, pool, fmt.Sprintf("warga-b-multi-%d@local.com", suffix), true)

	_ = notifTestMembershipInPool(t, pool, ketuaID, rtA, auth.RolePengurus, "ketua", true)
	_ = notifTestMembershipInPool(t, pool, wakilID, rtA, auth.RolePengurus, "wakil_ketua", true)
	_ = notifTestMembershipInPool(t, pool, wargaAID, rtA, auth.RoleWarga, "", true)
	_ = notifTestMembershipInPool(t, pool, inactiveAID, rtA, auth.RoleWarga, "", false)
	_ = notifTestMembershipInPool(t, pool, otherRTUserID, rtB, auth.RoleWarga, "", true)

	// Register Ketua RT with two active mobile devices (Android + second device),
	// Warga A with Android, inactive user with token, and other-RT user with token.
	ketuaAndroidTok := fmt.Sprintf("tok-ketua-android-%d", suffix)
	ketuaSecondaryTok := fmt.Sprintf("tok-ketua-ios-%d", suffix)
	wargaATok := fmt.Sprintf("tok-warga-a-%d", suffix)
	inactiveATok := fmt.Sprintf("tok-inactive-a-%d", suffix)
	otherRTTok := fmt.Sprintf("tok-other-rt-%d", suffix)

	repo := notification.NewRepository(pool)
	mockPush := notification.NewMockPushProvider()
	svc := notification.NewService(repo, mockPush)

	androidDevID := "android-dev-01"
	iosDevID := "ios-dev-02"
	if err := svc.RegisterDevice(ctx, ketuaID, notification.RegisterDeviceInput{
		FCMToken: ketuaAndroidTok,
		Platform: notification.PlatformAndroid,
		DeviceID: &androidDevID,
	}); err != nil {
		t.Fatalf("register ketua android device: %v", err)
	}
	if err := svc.RegisterDevice(ctx, ketuaID, notification.RegisterDeviceInput{
		FCMToken: ketuaSecondaryTok,
		Platform: notification.PlatformIOS,
		DeviceID: &iosDevID,
	}); err != nil {
		t.Fatalf("register ketua ios device: %v", err)
	}
	if err := svc.RegisterDevice(ctx, wargaAID, notification.RegisterDeviceInput{
		FCMToken: wargaATok,
		Platform: notification.PlatformAndroid,
	}); err != nil {
		t.Fatalf("register wargaA device: %v", err)
	}
	if err := svc.RegisterDevice(ctx, inactiveAID, notification.RegisterDeviceInput{
		FCMToken: inactiveATok,
		Platform: notification.PlatformAndroid,
	}); err != nil {
		t.Fatalf("register inactive device: %v", err)
	}
	if err := svc.RegisterDevice(ctx, otherRTUserID, notification.RegisterDeviceInput{
		FCMToken: otherRTTok,
		Platform: notification.PlatformAndroid,
	}); err != nil {
		t.Fatalf("register other RT device: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM notification_reads WHERE user_id IN ($1,$2,$3,$4,$5)`, ketuaID, wakilID, wargaAID, inactiveAID, otherRTUserID)
		_, _ = db.Exec(`DELETE FROM notification_logs WHERE rt_id IN ($1,$2) OR rw_id = $3`, rtA, rtB, rwID)
		_, _ = db.Exec(`DELETE FROM user_devices WHERE user_id IN ($1,$2,$3,$4,$5)`, ketuaID, wakilID, wargaAID, inactiveAID, otherRTUserID)
		_, _ = db.Exec(`DELETE FROM user_rt_memberships WHERE rt_id IN ($1,$2)`, rtA, rtB)
		_, _ = db.Exec(`DELETE FROM users WHERE id IN ($1,$2,$3,$4,$5)`, ketuaID, wakilID, wargaAID, inactiveAID, otherRTUserID)
		_, _ = db.Exec(`DELETE FROM rts WHERE id IN ($1,$2)`, rtA, rtB)
		_, _ = db.Exec(`DELETE FROM rws WHERE id = $1`, rwID)
	})

	// 1 & 2 & 4: Ketua RT sends manual push from Web -> Ketua RT's Android + iOS
	// devices AND Warga A receive push exactly once each; inactive member and
	// other-RT user do NOT receive push.
	t.Run("same_account_web_send_delivers_to_android_and_all_active_devices_without_duplicates", func(t *testing.T) {
		mockPush.Clear()
		err := svc.SendManualPush(ctx, rtA, ketuaID, notification.ManualPushInput{
			Title: "Pengumuman Rapat RT",
			Body:  "Rapat rutin besok pukul 19.00 WIB.",
		})
		if err != nil {
			t.Fatalf("SendManualPush failed: %v", err)
		}

		msgs := mockPush.GetMessages()
		counts := map[string]int{}
		for _, m := range msgs {
			counts[m.Token]++
		}

		if counts[ketuaAndroidTok] != 1 {
			t.Errorf("expected Ketua RT Android token to receive exactly 1 push, got %d", counts[ketuaAndroidTok])
		}
		if counts[ketuaSecondaryTok] != 1 {
			t.Errorf("expected Ketua RT secondary device token to receive exactly 1 push, got %d", counts[ketuaSecondaryTok])
		}
		if counts[wargaATok] != 1 {
			t.Errorf("expected Warga A token to receive exactly 1 push, got %d", counts[wargaATok])
		}
		if counts[inactiveATok] != 0 {
			t.Errorf("inactive member must NOT receive push, got %d", counts[inactiveATok])
		}
		if counts[otherRTTok] != 0 {
			t.Errorf("other RT user must NOT receive push, got %d", counts[otherRTTok])
		}
	})

	// 3: Different account (Wakil Ketua) sends -> Ketua RT's Android device still receives push.
	t.Run("different_account_send_delivers_to_ketua_android", func(t *testing.T) {
		mockPush.Clear()
		err := svc.SendManualPush(ctx, rtA, wakilID, notification.ManualPushInput{
			Title: "Kerja Bakti Minggu",
			Body:  "Kerja bakti dimulai jam 07.00 WIB.",
		})
		if err != nil {
			t.Fatalf("SendManualPush by wakil failed: %v", err)
		}

		counts := map[string]int{}
		for _, m := range mockPush.GetMessages() {
			counts[m.Token]++
		}
		if counts[ketuaAndroidTok] != 1 {
			t.Errorf("expected Ketua RT Android token to receive 1 push from wakil, got %d", counts[ketuaAndroidTok])
		}
		if counts[otherRTTok] != 0 {
			t.Errorf("other RT user must NOT receive push, got %d", counts[otherRTTok])
		}
	})

	// 5: History, unread count, and mark-as-read remain consistent for Ketua RT.
	t.Run("history_unread_and_read_status_consistent_for_same_account", func(t *testing.T) {
		ketuaAC := &auth.AuthContext{
			UserID:     ketuaID,
			RTID:       rtA,
			TenantRole: auth.RolePengurus,
			Jabatan:    auth.JabatanKetua,
		}

		unreadBefore, err := svc.GetUnreadCount(ctx, ketuaAC, rtA)
		if err != nil {
			t.Fatalf("GetUnreadCount failed: %v", err)
		}
		if unreadBefore != 2 {
			t.Errorf("expected 2 unread personal notifications (1 self-sent + 1 from wakil), got %d", unreadBefore)
		}

		logs, total, err := svc.ListLogs(ctx, ketuaAC, rtA, "", "", 1, 20)
		if err != nil {
			t.Fatalf("ListLogs failed: %v", err)
		}
		if total != 2 || len(logs) != 2 {
			t.Fatalf("expected 2 personal log entries, got total=%d len=%d", total, len(logs))
		}

		if err := svc.MarkAsRead(ctx, ketuaAC, rtA, logs[0].ID); err != nil {
			t.Fatalf("MarkAsRead failed: %v", err)
		}
		unreadAfter, err := svc.GetUnreadCount(ctx, ketuaAC, rtA)
		if err != nil {
			t.Fatalf("GetUnreadCount after read failed: %v", err)
		}
		if unreadAfter != 1 {
			t.Errorf("expected unread count 1 after reading 1 entry, got %d", unreadAfter)
		}
	})

	// 6: RW manual push when sender Ketua RT is in target recipients also delivers
	// push to Ketua RT's Android device without duplicates.
	t.Run("rw_manual_push_same_account_delivers_without_duplicates", func(t *testing.T) {
		mockPush.Clear()
		sentCount, err := svc.SendRWManualPush(ctx, rwID, ketuaID, "Info RW 016", "Rapat koordinasi RW.")
		if err != nil {
			t.Fatalf("SendRWManualPush failed: %v", err)
		}
		if sentCount < 1 {
			t.Errorf("expected sentCount >= 1, got %d", sentCount)
		}

		counts := map[string]int{}
		for _, m := range mockPush.GetMessages() {
			counts[m.Token]++
		}
		if counts[ketuaAndroidTok] != 1 {
			t.Errorf("expected Ketua RT Android token to receive 1 RW push, got %d", counts[ketuaAndroidTok])
		}
		if counts[ketuaSecondaryTok] != 1 {
			t.Errorf("expected Ketua RT secondary token to receive 1 RW push, got %d", counts[ketuaSecondaryTok])
		}
		// Plain warga is not an RW/RT pengurus target
		if counts[wargaATok] != 0 {
			t.Errorf("plain warga must NOT receive RW pengurus push, got %d", counts[wargaATok])
		}
	})
}
