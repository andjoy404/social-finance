package notification_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-finance/internal/auth"

	"github.com/google/uuid"
)

// attachUserToResidentOccupancy creates physical house -> household -> occupancy -> resident -> residency period.
func attachUserToResidentOccupancy(t *testing.T, db *sql.DB, rtID, userID, email string) {
	t.Helper()
	ctx := context.Background()
	uniq := uuid.New().String()[:8]
	var houseID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, address) VALUES ($1, $2, 'Street') RETURNING id`,
		rtID, "H-"+uniq,
	).Scan(&houseID); err != nil {
		t.Fatalf("create house: %v", err)
	}

	var hhID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, $2) RETURNING id`,
		rtID, "Head "+uniq,
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
		`INSERT INTO residents (rt_id, full_name, email, is_active, user_id)
		 VALUES ($1, $2, $3, true, $4) RETURNING id`,
		rtID, "Res "+uniq, email, userID,
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
}

// waitForCondition polls until cond returns true or timeout expires.
func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out after %v waiting for condition", timeout)
}

// ============================================================================
// Comprehensive Regression Tests for Waste Truck Notifications with Images
// Covers:
// - Scenario A: Keamanan triggers waste truck arrival with image -> dispatch record has image_url;
//   recipient delivery records created with image_url and recipient user_ids.
// - Scenario B: Recipient visibility: Warga A, Warga B, and Ketua RT see exactly 1 received record in ListLogs.
//   Keamanan sees their dispatch in ListWasteTruckHistory and sees 0 dispatch rows in ListLogs (user_id = NULL excluded).
//   No cross-user leakage.
// - Scenario C: Dispatches without image vs with image both follow identical contract.
// - Scenario D: Retry with same Idempotency-Key does NOT duplicate dispatch or recipient rows.
//   Consecutive distinct dispatches DO create new entries.
// - Scenario E: FCM failure/no-device updates status to NO_DEVICE or FAILED while recipient records remain preserved.
// - Scenario F: Authorization: Keamanan with notification.send_waste_truck can send; Keamanan cannot access management history;
//   Ketua RT with notification.manage functions normally.
// ============================================================================

func TestWasteTruckImage_EndToEnd_DispatchAndRecipientDelivery(t *testing.T) {
	pool := setupNotificationTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	// Ensure permission notification.send_waste_truck exists and is granted to keamanan
	var permID string
	_ = db.QueryRowContext(ctx, `SELECT id FROM permission_types WHERE code = 'notification.send_waste_truck'`).Scan(&permID)
	if permID == "" {
		_ = db.QueryRowContext(ctx, `INSERT INTO permission_types (code, domain, operation) VALUES ('notification.send_waste_truck', 'notification', 'send_waste_truck') RETURNING id`).Scan(&permID)
	}
	if permID != "" {
		_, _ = db.ExecContext(ctx, `INSERT INTO position_permissions (position, permission_id) VALUES ('keamanan', $1) ON CONFLICT DO NOTHING`, permID)
	}

	router := buildNotificationTestRouter(pool)

	// Setup RT
	rtID := notifTestRTInPool(t, pool, "wt-img-rt")

	// Setup Keamanan user
	keamananUID := notifTestUserInPool(t, pool, "keamanan-img@local.com", true)
	keamananMID := notifTestMembershipInPool(t, pool, keamananUID, rtID, auth.RoleWarga, "keamanan", true)
	keamananToken := makeNotificationToken(t, keamananUID, keamananMID, rtID, auth.RoleWarga, auth.JabatanKeamanan)

	// Setup Ketua RT user
	ketuaUID := notifTestUserInPool(t, pool, "ketua-img@local.com", true)
	ketuaMID := notifTestMembershipInPool(t, pool, ketuaUID, rtID, auth.RolePengurus, "ketua", true)
	ketuaToken := makeNotificationToken(t, ketuaUID, ketuaMID, rtID, auth.RolePengurus, auth.JabatanKetua)
	notifTestDeviceInPool(t, pool, ketuaUID, "fcm-ketua-token", true)

	// Setup Warga A user
	wargaAUID := notifTestUserInPool(t, pool, "warga-a@local.com", true)
	wargaAMID := notifTestMembershipInPool(t, pool, wargaAUID, rtID, auth.RoleWarga, "", true)
	wargaAToken := makeNotificationToken(t, wargaAUID, wargaAMID, rtID, auth.RoleWarga, "")
	notifTestDeviceInPool(t, pool, wargaAUID, "fcm-warga-a-token", true)

	// Setup Warga B user (no device registered)
	wargaBUID := notifTestUserInPool(t, pool, "warga-b@local.com", true)
	wargaBMID := notifTestMembershipInPool(t, pool, wargaBUID, rtID, auth.RoleWarga, "", true)
	wargaBToken := makeNotificationToken(t, wargaBUID, wargaBMID, rtID, auth.RoleWarga, "")

	// Attach ketua, wargaA, and wargaB to active households in this RT
	attachUserToResidentOccupancy(t, db, rtID, ketuaUID, "ketua-img@local.com")
	attachUserToResidentOccupancy(t, db, rtID, wargaAUID, "warga-a@local.com")
	attachUserToResidentOccupancy(t, db, rtID, wargaBUID, "warga-b@local.com")

	// SCENARIO A: Keamanan triggers waste truck arrival with image
	t.Run("Scenario A & B: Dispatch with image and recipient delivery visibility", func(t *testing.T) {
		imageURL := "https://social-finance.local/api/v1/notifications/images/notifications/waste_truck_arrival/test.png"
		payload := map[string]interface{}{
			"message":   "Truk sampah tiba di gerbang utama.",
			"image_url": imageURL,
		}
		bodyBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+keamananToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
		}

		// Wait for async dispatch to record in DB
		waitForCondition(t, 2*time.Second, func() bool {
			var dispatchCount int
			_ = db.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM notification_logs
				WHERE rt_id = $1 AND event_type = 'waste_truck_arrival' AND sender_user_id = $2 AND user_id IS NULL
			`, rtID, keamananUID).Scan(&dispatchCount)
			return dispatchCount == 1
		})

		var dbImageURL string
		err := db.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(image_url), '')
			FROM notification_logs
			WHERE rt_id = $1 AND event_type = 'waste_truck_arrival' AND sender_user_id = $2 AND user_id IS NULL
		`, rtID, keamananUID).Scan(&dbImageURL)
		if err != nil {
			t.Fatalf("query dispatch image_url: %v", err)
		}
		if dbImageURL != imageURL {
			t.Fatalf("expected dispatch image_url %q, got %q", imageURL, dbImageURL)
		}

		// Wait for recipient delivery records in DB (3 active residents in RT)
		waitForCondition(t, 2*time.Second, func() bool {
			var recipientCount int
			_ = db.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM notification_logs
				WHERE rt_id = $1 AND event_type = 'waste_truck_arrival' AND user_id IS NOT NULL
			`, rtID).Scan(&recipientCount)
			return recipientCount == 3
		})

		// Verify image_url is propagated to recipient records
		var recipientsWithImage int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM notification_logs
			WHERE rt_id = $1 AND event_type = 'waste_truck_arrival' AND user_id IS NOT NULL AND image_url = $2
		`, rtID, imageURL).Scan(&recipientsWithImage)
		if err != nil {
			t.Fatalf("query recipients with image: %v", err)
		}
		if recipientsWithImage != 3 {
			t.Fatalf("expected all 3 recipient records to have image_url, got %d", recipientsWithImage)
		}

		// SCENARIO B: Check API visibility for each actor
		// 1. Keamanan history check (/api/v1/notifications/waste-truck/history)
		reqH := httptest.NewRequest("GET", "/api/v1/notifications/waste-truck/history", nil)
		reqH.Header.Set("Authorization", "Bearer "+keamananToken)
		recH := httptest.NewRecorder()
		router.ServeHTTP(recH, reqH)
		if recH.Code != http.StatusOK {
			t.Fatalf("expected 200 for keamanan waste truck history, got %d: %s", recH.Code, recH.Body.String())
		}
		var historyResp struct {
			Data []struct {
				ID           string  `json:"id"`
				SenderUserID *string `json:"sender_user_id"`
				ImageURL     *string `json:"image_url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recH.Body.Bytes(), &historyResp); err != nil {
			t.Fatalf("unmarshal history resp: %v", err)
		}
		if len(historyResp.Data) != 1 {
			t.Fatalf("expected 1 history item for keamanan, got %d", len(historyResp.Data))
		}
		if historyResp.Data[0].ImageURL == nil || *historyResp.Data[0].ImageURL != imageURL {
			t.Fatalf("expected history image_url %q, got %v", imageURL, historyResp.Data[0].ImageURL)
		}

		// 2. Keamanan should see 0 rows in personal notification logs (/api/v1/notifications/logs)
		reqL := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
		reqL.Header.Set("Authorization", "Bearer "+keamananToken)
		recL := httptest.NewRecorder()
		router.ServeHTTP(recL, reqL)
		if recL.Code != http.StatusOK {
			t.Fatalf("expected 200 for keamanan logs, got %d: %s", recL.Code, recL.Body.String())
		}
		var keamLogsResp struct {
			Data []map[string]interface{} `json:"data"`
		}
		_ = json.Unmarshal(recL.Body.Bytes(), &keamLogsResp)
		if len(keamLogsResp.Data) != 0 {
			t.Fatalf("expected 0 personal logs for keamanan, got %d (leakage detected)", len(keamLogsResp.Data))
		}

		// 3. Warga A should see exactly 1 log in personal notification logs
		reqWA := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
		reqWA.Header.Set("Authorization", "Bearer "+wargaAToken)
		recWA := httptest.NewRecorder()
		router.ServeHTTP(recWA, reqWA)
		if recWA.Code != http.StatusOK {
			t.Fatalf("expected 200 for warga A logs, got %d: %s", recWA.Code, recWA.Body.String())
		}
		var wargaALogsResp struct {
			Data []struct {
				ID        string  `json:"id"`
				UserID    *string `json:"user_id"`
				ImageURL  *string `json:"image_url"`
				EventType string  `json:"event_type"`
			} `json:"data"`
		}
		_ = json.Unmarshal(recWA.Body.Bytes(), &wargaALogsResp)
		if len(wargaALogsResp.Data) != 1 {
			t.Fatalf("expected exactly 1 personal log for warga A, got %d", len(wargaALogsResp.Data))
		}
		if wargaALogsResp.Data[0].ImageURL == nil || *wargaALogsResp.Data[0].ImageURL != imageURL {
			t.Fatalf("expected warga A log to have image_url %q, got %v", imageURL, wargaALogsResp.Data[0].ImageURL)
		}

		// 4. Warga B (no device) should also see exactly 1 log preserved in personal notification logs
		reqWB := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
		reqWB.Header.Set("Authorization", "Bearer "+wargaBToken)
		recWB := httptest.NewRecorder()
		router.ServeHTTP(recWB, reqWB)
		if recWB.Code != http.StatusOK {
			t.Fatalf("expected 200 for warga B logs, got %d: %s", recWB.Code, recWB.Body.String())
		}
		var wargaBLogsResp struct {
			Data []struct {
				ID        string  `json:"id"`
				UserID    *string `json:"user_id"`
				ImageURL  *string `json:"image_url"`
				Status    string  `json:"status"`
				EventType string  `json:"event_type"`
			} `json:"data"`
		}
		_ = json.Unmarshal(recWB.Body.Bytes(), &wargaBLogsResp)
		if len(wargaBLogsResp.Data) != 1 {
			t.Fatalf("expected exactly 1 personal log for warga B, got %d", len(wargaBLogsResp.Data))
		}
		if wargaBLogsResp.Data[0].Status == "" {
			t.Fatalf("expected status to be non-empty")
		}

		// 5. Ketua RT should also see exactly 1 log in personal logs
		reqKetua := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
		reqKetua.Header.Set("Authorization", "Bearer "+ketuaToken)
		recKetua := httptest.NewRecorder()
		router.ServeHTTP(recKetua, reqKetua)
		if recKetua.Code != http.StatusOK {
			t.Fatalf("expected 200 for ketua logs, got %d: %s", recKetua.Code, recKetua.Body.String())
		}
		var ketuaLogsResp struct {
			Data []struct {
				ID       string  `json:"id"`
				ImageURL *string `json:"image_url"`
			} `json:"data"`
		}
		_ = json.Unmarshal(recKetua.Body.Bytes(), &ketuaLogsResp)
		if len(ketuaLogsResp.Data) != 1 {
			t.Fatalf("expected exactly 1 personal log for ketua, got %d", len(ketuaLogsResp.Data))
		}
	})

	// SCENARIO C: Dispatches without image follow identical contract
	t.Run("Scenario C: Dispatch without image follows identical contract", func(t *testing.T) {
		payload := map[string]interface{}{
			"message":   "Truk sampah telah meninggalkan lingkungan.",
			"image_url": nil,
		}
		bodyBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+keamananToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
		}

		// Wait for 2nd dispatch to be recorded
		waitForCondition(t, 2*time.Second, func() bool {
			var dispatchCount int
			_ = db.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM notification_logs
				WHERE rt_id = $1 AND event_type = 'waste_truck_arrival' AND sender_user_id = $2 AND user_id IS NULL
			`, rtID, keamananUID).Scan(&dispatchCount)
			return dispatchCount == 2
		})

		// Wait for recipients to have 6 entries (3 from first dispatch + 3 from second)
		waitForCondition(t, 2*time.Second, func() bool {
			var recipientCount int
			_ = db.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM notification_logs
				WHERE rt_id = $1 AND event_type = 'waste_truck_arrival' AND user_id IS NOT NULL
			`, rtID).Scan(&recipientCount)
			return recipientCount == 6
		})

		// Keamanan history should now have 2 items
		reqH := httptest.NewRequest("GET", "/api/v1/notifications/waste-truck/history", nil)
		reqH.Header.Set("Authorization", "Bearer "+keamananToken)
		recH := httptest.NewRecorder()
		router.ServeHTTP(recH, reqH)
		if recH.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recH.Code)
		}
		var historyResp struct {
			Data []struct {
				ID       string  `json:"id"`
				ImageURL *string `json:"image_url"`
			} `json:"data"`
		}
		_ = json.Unmarshal(recH.Body.Bytes(), &historyResp)
		if len(historyResp.Data) != 2 {
			t.Fatalf("expected 2 history items, got %d", len(historyResp.Data))
		}
		// The newest item (first) should have nil image_url
		if historyResp.Data[0].ImageURL != nil {
			t.Fatalf("expected nil image_url for dispatch without image, got %v", *historyResp.Data[0].ImageURL)
		}

		// Warga A logs should now have 2 items
		reqWA := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
		reqWA.Header.Set("Authorization", "Bearer "+wargaAToken)
		recWA := httptest.NewRecorder()
		router.ServeHTTP(recWA, reqWA)
		var wargaALogsResp struct {
			Data []struct {
				ID       string  `json:"id"`
				ImageURL *string `json:"image_url"`
			} `json:"data"`
		}
		_ = json.Unmarshal(recWA.Body.Bytes(), &wargaALogsResp)
		if len(wargaALogsResp.Data) != 2 {
			t.Fatalf("expected 2 personal logs for warga A, got %d", len(wargaALogsResp.Data))
		}
	})

	// SCENARIO D: Retry with same Idempotency-Key does NOT duplicate dispatch or recipient rows
	t.Run("Scenario D: Idempotency-Key deduplication prevents duplicate dispatch and recipient logs", func(t *testing.T) {
		var dispatchCountBefore, recipientCountBefore int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND sender_user_id = $2 AND user_id IS NULL`, rtID, keamananUID).Scan(&dispatchCountBefore)
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND user_id IS NOT NULL`, rtID).Scan(&recipientCountBefore)

		idempotencyKey := "idemp-test-uuid-" + uuid.New().String()
		payload := map[string]interface{}{
			"message":   "Truk sampah sedang memuat sampah di blok C.",
			"image_url": "https://social-finance.local/images/block_c.png",
		}
		bodyBytes, _ := json.Marshal(payload)

		// First call with Idempotency-Key
		req1 := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewBuffer(bodyBytes))
		req1.Header.Set("Authorization", "Bearer "+keamananToken)
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", idempotencyKey)
		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusAccepted {
			t.Fatalf("expected 202 on first call, got %d: %s", rec1.Code, rec1.Body.String())
		}

		// Wait for first call to record
		waitForCondition(t, 2*time.Second, func() bool {
			var dCount int
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND sender_user_id = $2 AND user_id IS NULL`, rtID, keamananUID).Scan(&dCount)
			return dCount == dispatchCountBefore+1
		})
		waitForCondition(t, 2*time.Second, func() bool {
			var rCount int
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND user_id IS NOT NULL`, rtID).Scan(&rCount)
			return rCount == recipientCountBefore+3
		})

		// Second call with EXACT SAME Idempotency-Key (retry)
		req2 := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewBuffer(bodyBytes))
		req2.Header.Set("Authorization", "Bearer "+keamananToken)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", idempotencyKey)
		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusAccepted {
			t.Fatalf("expected 202 on idempotent retry, got %d: %s", rec2.Code, rec2.Body.String())
		}

		// Wait a moment and ensure count has NOT increased
		time.Sleep(100 * time.Millisecond)

		var dispatchCountAfter, recipientCountAfter int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND sender_user_id = $2 AND user_id IS NULL`, rtID, keamananUID).Scan(&dispatchCountAfter)
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND user_id IS NOT NULL`, rtID).Scan(&recipientCountAfter)

		if dispatchCountAfter != dispatchCountBefore+1 {
			t.Fatalf("idempotent retry created duplicate dispatch log! Expected: %d, Got: %d", dispatchCountBefore+1, dispatchCountAfter)
		}
		if recipientCountAfter != recipientCountBefore+3 {
			t.Fatalf("idempotent retry created duplicate recipient logs! Expected: %d, Got: %d", recipientCountBefore+3, recipientCountAfter)
		}
	})

	// SCENARIO E: Recipient delivery status preservation (device vs no device)
	t.Run("Scenario E: Recipient logs are preserved regardless of device status", func(t *testing.T) {
		// Query Warga A (has device) status
		var wargaAStatus string
		err := db.QueryRowContext(ctx, `
			SELECT status FROM notification_logs
			WHERE rt_id = $1 AND user_id = $2
			ORDER BY created_at DESC LIMIT 1
		`, rtID, wargaAUID).Scan(&wargaAStatus)
		if err != nil {
			t.Fatalf("query warga A status: %v", err)
		}
		if wargaAStatus != "SENT" {
			t.Fatalf("expected warga A status SENT, got %q", wargaAStatus)
		}

		// Query Warga B (no device) status
		var wargaBStatus string
		err = db.QueryRowContext(ctx, `
			SELECT status FROM notification_logs
			WHERE rt_id = $1 AND user_id = $2
			ORDER BY created_at DESC LIMIT 1
		`, rtID, wargaBUID).Scan(&wargaBStatus)
		if err != nil {
			t.Fatalf("query warga B status: %v", err)
		}
		if wargaBStatus != "NO_DEVICE" {
			t.Fatalf("expected warga B status NO_DEVICE, got %q", wargaBStatus)
		}

		// Both logs exist and are visible to users
		var countA, countB int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND user_id = $2`, rtID, wargaAUID).Scan(&countA)
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1 AND user_id = $2`, rtID, wargaBUID).Scan(&countB)
		if countA == 0 {
			t.Fatalf("expected warga A log to exist")
		}
		if countB == 0 {
			t.Fatalf("expected warga B log to exist")
		}
	})

	// SCENARIO F: Authorization boundaries
	t.Run("Scenario F: Role and permission enforcement", func(t *testing.T) {
		// Keamanan cannot access notification management endpoints
		reqMgmt := httptest.NewRequest("GET", "/api/v1/notifications/system-types", nil)
		reqMgmt.Header.Set("Authorization", "Bearer "+keamananToken)
		recMgmt := httptest.NewRecorder()
		router.ServeHTTP(recMgmt, reqMgmt)
		if recMgmt.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for Keamanan accessing system-types, got %d", recMgmt.Code)
		}

		// Warga cannot trigger waste truck notifications
		reqWargaSend := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewBufferString(`{"message":"test"}`))
		reqWargaSend.Header.Set("Authorization", "Bearer "+wargaAToken)
		reqWargaSend.Header.Set("Content-Type", "application/json")
		recWargaSend := httptest.NewRecorder()
		router.ServeHTTP(recWargaSend, reqWargaSend)
		if recWargaSend.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for Warga sending waste truck notif, got %d", recWargaSend.Code)
		}

		// Ketua RT can access system-types
		reqKetuaMgmt := httptest.NewRequest("GET", "/api/v1/notifications/system-types", nil)
		reqKetuaMgmt.Header.Set("Authorization", "Bearer "+ketuaToken)
		recKetuaMgmt := httptest.NewRecorder()
		router.ServeHTTP(recKetuaMgmt, reqKetuaMgmt)
		if recKetuaMgmt.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for Ketua RT accessing system-types, got %d: %s", recKetuaMgmt.Code, recKetuaMgmt.Body.String())
		}
	})
}
