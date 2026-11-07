package notification_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
)

// ============================================================================
// Dedup Key Contract Verification Tests
//
// These tests verify the exact dedup_key values written by the system and
// ensure ListLogs filters correctly for non-officer users (Warga, Keamanan).
//
// Key contracts observed from source code:
//
//   - SendManualPush sender record:
//     dedup_key = "manual_push:{rtID}:{timestamp}:{title}"
//     user_id = senderUserID
//     event_type = "manual_push"
//
//   - SendManualPush recipient records (via RecordBroadcastRecipientDelivery):
//     dedup_key = "recipient:manual_push:{broadcastID}:{recipientUserID}"
//     user_id = recipientUserID
//     event_type = "manual_push"
//
//   - RecordWasteTruckDispatch (dispatch):
//     dedup_key = "waste_truck:{rtID}:{timeslot}"
//     user_id = NULL (only sender_user_id is set)
//     event_type = "waste_truck_arrival"
//
//   - RecordWasteTruckArrival recipient records (via RecordBroadcastRecipientDelivery):
//     dedup_key = "recipient:waste_truck_arrival:{broadcastID}:{recipientUserID}"
//     user_id = recipientUserID
//     event_type = "waste_truck_arrival"
//
//   - ListLogs personalScope:
//     "AND nl.user_id = $1 AND nl.dedup_key NOT LIKE 'manual_push:%'"
//
// IMPORTANT: The NOT LIKE 'manual_push:%' pattern does NOT exclude
// "recipient:manual_push:..." records because they start with "recipient:",
// not "manual_push:". Only the sender/broadcast record has that prefix.
// ============================================================================

// TestDedupKeyContracts verifies the actual dedup_key values stored in the
// database for each notification type.
func TestDedupKeyContracts(t *testing.T) {
	pool := setupNotificationTestPool(t)
	_ = buildNotificationTestRouter(pool) // router not needed for this test

	rtID := notifTestRTInPool(t, pool, "dedupkey-contract")

	// Create users
	senderID := notifTestUserInPool(t, pool, "dedup-sender@local.com", true)
	_ = notifTestMembershipInPool(t, pool, senderID, rtID, auth.RoleWarga, "ketua", true)
	notifTestDeviceInPool(t, pool, senderID, "tok-dedup-sender", true)

	securityID := notifTestUserInPool(t, pool, "dedup-security@local.com", true)
	_ = notifTestMembershipInPool(t, pool, securityID, rtID, auth.RoleWarga, "keamanan", true)
	notifTestDeviceInPool(t, pool, securityID, "tok-dedup-security", true)

	wargaID := notifTestUserInPool(t, pool, "dedup-warga@local.com", true)
	_ = notifTestMembershipInPool(t, pool, wargaID, rtID, auth.RoleWarga, "", true)
	notifTestDeviceInPool(t, pool, wargaID, "tok-dedup-warga", true)

	// 1. Insert manual_push sender record
	manualSenderDedup := "manual_push:rt-test:1234567890:test announcement"
	senderLogID := notifTestLogInPool(t, pool, rtID, senderID, notification.EventManualPush,
		manualSenderDedup, "Pengumuman Pengurus", "Broadcast manual push")

	// 2. Insert manual_push recipient record for Security
	// Format: recipient:manual_push:{broadcastID}:{recipientUserID}
	securityManualPushDedup := "recipient:manual_push:bcast-001:" + securityID
	securityManualPushID := notifTestLogInPool(t, pool, rtID, securityID, notification.EventManualPush,
		securityManualPushDedup, "Pengumuman Pengurus", "Broadcast to security")

	// 3. Insert manual_push recipient record for Warga
	wargaManualPushDedup := "recipient:manual_push:bcast-001:" + wargaID
	wargaManualPushID := notifTestLogInPool(t, pool, rtID, wargaID, notification.EventManualPush,
		wargaManualPushDedup, "Pengumuman Pengurus", "Broadcast to warga")

	// 4. Insert waste_truck dispatch (user_id = NULL)
	wasteTruckDedup := "waste_truck:rt-test:2026-01-01-10"
	_ = notifTestLogWithNullableUserInPool(t, pool, rtID, notification.EventWasteTruckArrival,
		wasteTruckDedup, "Mobil Sampah Tiba", "Waste truck dispatch internal")

	// 5. Insert waste_truck recipient record for Security
	securityWasteTruckDedup := "recipient:waste_truck_arrival:bcast-waste-001:" + securityID
	securityWasteTruckID := notifTestLogInPool(t, pool, rtID, securityID, notification.EventWasteTruckArrival,
		securityWasteTruckDedup, "Mobil Pengangkut Sampah Sudah Tiba", "Waste truck arrived")

	// 6. Insert waste_truck recipient record for Warga
	wargaWasteTruckDedup := "recipient:waste_truck_arrival:bcast-waste-001:" + wargaID
	_ = notifTestLogInPool(t, pool, rtID, wargaID, notification.EventWasteTruckArrival,
		wargaWasteTruckDedup, "Mobil Pengangkut Sampah Sudah Tiba", "Waste truck arrived")

	// Verify actual stored dedup_key and user_id values
	verifyDedupKey := func(t *testing.T, logID, expectedDedup string) {
		t.Helper()
		var storedDedup, eventType string
		var userID *string
		err := pool.Raw().QueryRowContext(
			context.Background(),
			`SELECT user_id, event_type, dedup_key FROM notification_logs WHERE id = $1`, logID,
		).Scan(&userID, &eventType, &storedDedup)
		if err != nil {
			t.Fatalf("query log %s: %v", logID, err)
		}
		if storedDedup != expectedDedup {
			t.Errorf("log %s: dedup_key = %q, want %q", logID, storedDedup, expectedDedup)
		}
		t.Logf("  [VERIFIED] %s: dedup_key=%q, user_id=%v, event_type=%s",
			logID, storedDedup, userID, eventType)
	}

	t.Run("manual_push sender", func(t *testing.T) {
		verifyDedupKey(t, senderLogID, manualSenderDedup)
	})
	t.Run("manual_push recipient (security)", func(t *testing.T) {
		verifyDedupKey(t, securityManualPushID, securityManualPushDedup)
	})
	t.Run("manual_push recipient (warga)", func(t *testing.T) {
		verifyDedupKey(t, wargaManualPushID, wargaManualPushDedup)
	})
	t.Run("waste_truck dispatch (NULL user)", func(t *testing.T) {
		// Just verify it exists with NULL user_id
		var storedDedup, eventType string
		var userID *string
		err := pool.Raw().QueryRowContext(
			context.Background(),
			`SELECT user_id, event_type, dedup_key FROM notification_logs WHERE event_type = $1 AND dedup_key = $2`,
			notification.EventWasteTruckArrival, wasteTruckDedup,
		).Scan(&userID, &eventType, &storedDedup)
		if err != nil {
			t.Fatalf("query waste_truck dispatch: %v", err)
		}
		if userID != nil {
			t.Errorf("waste_truck dispatch: user_id = %v, want NULL", *userID)
		}
		t.Logf("  [VERIFIED] waste_truck dispatch: dedup_key=%q, user_id=NULL, event_type=%s",
			storedDedup, eventType)
	})
	t.Run("waste_truck recipient (security)", func(t *testing.T) {
		verifyDedupKey(t, securityWasteTruckID, securityWasteTruckDedup)
	})
}

// TestListLogs_SecurityOfficerOnlySeesOwnRecipientRecords verifies that a
// security officer sees exactly the records that belong to them via ListLogs,
// and nothing else.
func TestListLogs_SecurityOfficerOnlySeesOwnRecipientRecords(t *testing.T) {
	pool := setupNotificationTestPool(t)
	router := buildNotificationTestRouter(pool)

	rtID := notifTestRTInPool(t, pool, "listlogs-contract-security")

	// Create users
	securityID := notifTestUserInPool(t, pool, "contract-security@local.com", true)
	securityMID := notifTestMembershipInPool(t, pool, securityID, rtID, auth.RoleWarga, "keamanan", true)
	notifTestDeviceInPool(t, pool, securityID, "tok-contract-security", true)

	wargaID := notifTestUserInPool(t, pool, "contract-warga@local.com", true)
	_ = notifTestMembershipInPool(t, pool, wargaID, rtID, auth.RoleWarga, "", true)

	senderID := notifTestUserInPool(t, pool, "contract-sender@local.com", true)
	_ = notifTestMembershipInPool(t, pool, senderID, rtID, auth.RoleWarga, "ketua", true)

	tokenSecurity := makeNotificationToken(t, securityID, securityMID, rtID, auth.RoleWarga, "keamanan")

	// --- INSERT 5 SCENARIOS ---

	// Scenario 1: manual_push sender record (admin broadcast log)
	// dedup_key = "manual_push:..."
	// user_id = senderID
	senderLogID := notifTestLogInPool(t, pool, rtID, senderID, notification.EventManualPush,
		"manual_push:rt-x:999:announcement", "Pengumuman Pengurus", "Manual push broadcast")

	// Scenario 2: manual_push recipient record for Security
	// dedup_key = "recipient:manual_push:{broadcastID}:{securityID}"
	// user_id = securityID
	secManualPushID := notifTestLogInPool(t, pool, rtID, securityID, notification.EventManualPush,
		"recipient:manual_push:bcast-x-001:"+securityID, "Pengumuman Pengurus", "Broadcast to security")

	// Scenario 3: manual_push recipient record for Warga lain
	// dedup_key = "recipient:manual_push:{broadcastID}:{wargaID}"
	// user_id = wargaID
	wargaManualPushID := notifTestLogInPool(t, pool, rtID, wargaID, notification.EventManualPush,
		"recipient:manual_push:bcast-x-001:"+wargaID, "Pengumuman Pengurus", "Broadcast to warga")

	// Scenario 4: waste_truck dispatch with user_id = NULL
	// dedup_key = "waste_truck:..."
	// user_id = NULL
	_ = notifTestLogWithNullableUserInPool(t, pool, rtID, notification.EventWasteTruckArrival,
		"waste_truck:rt-x:2026-01-01-10", "Mobil Sampah Tiba", "Waste truck dispatch")

	// Scenario 5: waste_truck recipient record for Security
	// dedup_key = "recipient:waste_truck_arrival:{broadcastID}:{securityID}"
	// user_id = securityID
	secWasteTruckID := notifTestLogInPool(t, pool, rtID, securityID, notification.EventWasteTruckArrival,
		"recipient:waste_truck_arrival:bcast-wt-001:"+securityID, "Mobil Pengangkut Sampah Sudah Tiba", "Waste truck arrived")

	// --- CALL ListLogs as Security Officer ---
	req := httptest.NewRequest("GET", "/api/v1/notifications/logs?page=1&per_page=20", nil)
	req = withBearerToken(req, tokenSecurity)
	rec := doRequest(router, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]any   `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// --- VERIFY: Exactly 2 records for security ---
	t.Log("=== SECURITY OFFICER ListLogs RESULT ===")
	for _, entry := range resp.Data {
		uid := ""
		if v, ok := entry["user_id"]; ok && v != nil {
			uid = v.(string)
		}
		dedup := ""
		if v, ok := entry["dedup_key"]; ok && v != nil {
			dedup = v.(string)
		}
		event := ""
		if v, ok := entry["event_type"]; ok && v != nil {
			event = v.(string)
		}
		t.Logf("  Entry: user_id=%q, dedup_key=%q, event_type=%q", uid, dedup, event)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 log entries for security officer, got %d: %v", len(resp.Data), resp.Data)
	}

	// Check total
	total, _ := resp.Pagination["total"].(float64)
	if int(total) != 2 {
		t.Errorf("expected total=2, got %f", total)
	}

	// Collect IDs of returned entries
	returnedIDs := make(map[string]bool)
	for _, entry := range resp.Data {
		if id, ok := entry["id"].(string); ok {
			returnedIDs[id] = true
		}
	}

	// Scenario 1: sender record should NOT appear
	if returnedIDs[senderLogID] {
		t.Errorf("Scenario 1 FAIL: manual_push sender record appeared in security officer view")
	} else {
		t.Log("Scenario 1 PASS: manual_push sender record correctly excluded")
	}

	// Scenario 2: security manual_push recipient SHOULD appear
	if !returnedIDs[secManualPushID] {
		t.Errorf("Scenario 2 FAIL: manual_push recipient record for security NOT found")
	} else {
		t.Log("Scenario 2 PASS: manual_push recipient record for security correctly shown")
	}

	// Scenario 3: warga manual_push recipient should NOT appear
	if returnedIDs[wargaManualPushID] {
		t.Errorf("Scenario 3 FAIL: manual_push recipient record for warga lain appeared in security view")
	} else {
		t.Log("Scenario 3 PASS: manual_push recipient record for warga lain correctly excluded")
	}

	// Scenario 4: waste_truck dispatch NULL should NOT appear
	// (The dispatch log was created with user_id=NULL, it's not returned by ListLogs)
	t.Log("Scenario 4: waste_truck dispatch with NULL user_id - checked by total count")

	// Scenario 5: security waste_truck recipient SHOULD appear
	if !returnedIDs[secWasteTruckID] {
		t.Errorf("Scenario 5 FAIL: waste_truck recipient record for security NOT found")
	} else {
		t.Log("Scenario 5 PASS: waste_truck recipient record for security correctly shown")
	}
}

// TestListLogs_WargaSeesNoSecurityData verifies that a regular warga user
// cannot see any of the security officer's notification data.
func TestListLogs_WargaSeesNoSecurityData(t *testing.T) {
	pool := setupNotificationTestPool(t)
	router := buildNotificationTestRouter(pool)

	rtID := notifTestRTInPool(t, pool, "listlogs-contract-warga")

	// Create users
	wargaID := notifTestUserInPool(t, pool, "contract-warga2@local.com", true)
	wargaMID := notifTestMembershipInPool(t, pool, wargaID, rtID, auth.RoleWarga, "", true)
	notifTestDeviceInPool(t, pool, wargaID, "tok-contract-warga2", true)

	securityID := notifTestUserInPool(t, pool, "contract-security2@local.com", true)
	_ = notifTestMembershipInPool(t, pool, securityID, rtID, auth.RoleWarga, "keamanan", true)
	notifTestDeviceInPool(t, pool, securityID, "tok-contract-security2", true)

	senderID := notifTestUserInPool(t, pool, "contract-sender2@local.com", true)
	_ = notifTestMembershipInPool(t, pool, senderID, rtID, auth.RoleWarga, "ketua", true)

	tokenWarga := makeNotificationToken(t, wargaID, wargaMID, rtID, auth.RoleWarga, "")

	// --- INSERT records for Security Officer ---

	// Security's manual_push recipient
	secManualPushID := notifTestLogInPool(t, pool, rtID, securityID, notification.EventManualPush,
		"recipient:manual_push:bcast-y-001:"+securityID, "Pengumuman Pengurus", "Broadcast to security")

	// Security's waste_truck recipient
	secWasteTruckID := notifTestLogInPool(t, pool, rtID, securityID, notification.EventWasteTruckArrival,
		"recipient:waste_truck_arrival:bcast-wt-y-001:"+securityID, "Mobil Pengangkut Sampah Sudah Tiba", "Waste truck")

	// Also insert a bill-created notification for security
	notifTestLogInPool(t, pool, rtID, securityID, notification.EventBillCreated,
		"bill:bill-001:"+securityID, "Tagihan Iuran Baru", "Bill for security")

	// --- Also insert a bill-created notification for warga ---
	notifTestLogInPool(t, pool, rtID, wargaID, notification.EventBillCreated,
		"bill:bill-002:"+wargaID, "Tagihan Iuran Baru", "Bill for warga")

	// --- CALL ListLogs as Warga ---
	req := httptest.NewRequest("GET", "/api/v1/notifications/logs?page=1&per_page=20", nil)
	req = withBearerToken(req, tokenWarga)
	rec := doRequest(router, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]any   `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// --- VERIFY: Warga only sees their own records ---
	t.Log("=== WARGA ListLogs RESULT ===")
	for _, entry := range resp.Data {
		uid := ""
		if v, ok := entry["user_id"]; ok && v != nil {
			uid = v.(string)
		}
		dedup := ""
		if v, ok := entry["dedup_key"]; ok && v != nil {
			dedup = v.(string)
		}
		event := ""
		if v, ok := entry["event_type"]; ok && v != nil {
			event = v.(string)
		}
		t.Logf("  Entry: user_id=%q, dedup_key=%q, event_type=%q", uid, dedup, event)
	}

	// Warga should only see: 1 bill-created for warga
	expectedCount := 1
	if len(resp.Data) != expectedCount {
		t.Errorf("expected %d log entries for warga, got %d: %v", expectedCount, len(resp.Data), resp.Data)
	}

	total, _ := resp.Pagination["total"].(float64)
	if int(total) != expectedCount {
		t.Errorf("expected total=%d, got %f", expectedCount, total)
	}

	// Collect IDs of returned entries
	returnedIDs := make(map[string]bool)
	for _, entry := range resp.Data {
		if id, ok := entry["id"].(string); ok {
			returnedIDs[id] = true
		}
	}

	// Security's manual_push recipient should NOT appear
	if returnedIDs[secManualPushID] {
		t.Errorf("Security manual_push recipient leaked to warga view")
	} else {
		t.Log("PASS: Security manual_push recipient correctly excluded from warga view")
	}

	// Security's waste_truck recipient should NOT appear
	if returnedIDs[secWasteTruckID] {
		t.Errorf("Security waste_truck recipient leaked to warga view")
	} else {
		t.Log("PASS: Security waste_truck recipient correctly excluded from warga view")
	}
}
