package rw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

func TestRWNotification_All4Paths_ImageDistribution_SnapshotPersistence_And_Scope(t *testing.T) {
	pool := testutil.GetTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	runID := time.Now().UnixNano()
	rwNumA := int(runID/1000000)%20000 + 70000
	rwNumB := rwNumA + 1

	var rwAID, rwBID, rtAID, rtBID string
	if err := db.QueryRowContext(ctx, `INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Dist A', $1, true) RETURNING id`, rwNumA).Scan(&rwAID); err != nil {
		t.Fatalf("insert rwA: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Dist B', $1, true) RETURNING id`, rwNumB).Scan(&rwBID); err != nil {
		t.Fatalf("insert rwB: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT Dist A', $1, '001', $2, true) RETURNING id`, rwNumA, rwAID).Scan(&rtAID); err != nil {
		t.Fatalf("insert rtA: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT Dist B', $1, '001', $2, true) RETURNING id`, rwNumB, rwBID).Scan(&rtBID); err != nil {
		t.Fatalf("insert rtB: %v", err)
	}

	ketuaRWA := uuid.NewString()
	ketuaRTA := uuid.NewString()
	ketuaRWB := uuid.NewString()
	for _, u := range []struct {
		id, email, name string
	}{
		{ketuaRWA, fmt.Sprintf("dist.krwa.%d@test.com", runID), "Ketua RW A"},
		{ketuaRTA, fmt.Sprintf("dist.krta.%d@test.com", runID), "Ketua RT A"},
		{ketuaRWB, fmt.Sprintf("dist.krwb.%d@test.com", runID), "Ketua RW B"},
	} {
		if _, err := db.ExecContext(ctx, `INSERT INTO users (id, email, full_name, password_hash, is_active) VALUES ($1, $2, $3, 'hash', true)`, u.id, u.email, u.name); err != nil {
			t.Fatalf("insert user %s: %v", u.name, err)
		}
	}
	_, _ = db.ExecContext(ctx, `INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, is_active) VALUES ($1, $2, 'warga', 'ketua', true)`, ketuaRWA, rwAID)
	_, _ = db.ExecContext(ctx, `INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active) VALUES ($1, $2, 'pengurus', 'ketua', true)`, ketuaRTA, rtAID)
	_, _ = db.ExecContext(ctx, `INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, is_active) VALUES ($1, $2, 'warga', 'ketua', true)`, ketuaRWB, rwBID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM notification_logs WHERE rw_id IN ($1, $2)`, rwAID, rwBID)
		_, _ = db.ExecContext(ctx, `DELETE FROM notification_templates WHERE rw_id IN ($1, $2)`, rwAID, rwBID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_devices WHERE user_id IN ($1, $2, $3)`, ketuaRWA, ketuaRTA, ketuaRWB)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rw_memberships WHERE rw_id IN ($1, $2)`, rwAID, rwBID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id IN ($1, $2)`, rtAID, rtBID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3)`, ketuaRWA, ketuaRTA, ketuaRWB)
		_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id IN ($1, $2)`, rtAID, rtBID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rws WHERE id IN ($1, $2)`, rwAID, rwBID)
	})

	store := storage.NewMockStorage()
	mockPush := notification.NewMockPushProvider()
	notifRepo := notification.NewRepository(pool)
	notifSvc := notification.NewService(notifRepo, mockPush)
	notifH := notification.NewHandler(notifSvc, pool, store, "test")

	// Register mock FCM device token for Ketua RT A so FCM push payload is captured
	fcmTok := fmt.Sprintf("fcm-dist-token-%d", runID)
	if err := notifRepo.RegisterDevice(ctx, ketuaRTA, fcmTok, "android", nil); err != nil {
		t.Fatalf("register device token: %v", err)
	}

	scopingCfg := &auth.ScopingConfig{RTIDs: []string{rtAID, rtBID}, RWIDs: []string{rwAID, rwBID}}
	rwRepo := NewRepository(db)
	rwSvc := NewService(pool, rwRepo, scopingCfg)
	rwSvc.SetNotifier(notifSvc)
	rwH := NewHandler(pool, rwSvc, notifSvc, store)

	// Store real test PNG objects in mock storage for RW A
	pngBytes := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82")
	metaManual, err := store.UploadNotificationImage(ctx, rwAID, "rw_manual", "manual.png", bytes.NewReader(pngBytes), int64(len(pngBytes)))
	if err != nil {
		t.Fatalf("upload manual img: %v", err)
	}
	imgManual := "/api/v1/notifications/images/" + metaManual.Key

	metaKasIn, err := store.UploadNotificationImage(ctx, rwAID, "rw_cash_in", "kas_in.png", bytes.NewReader(pngBytes), int64(len(pngBytes)))
	if err != nil {
		t.Fatalf("upload kas_in img: %v", err)
	}
	imgKasIn := "/api/v1/notifications/images/" + metaKasIn.Key

	metaKasOut, err := store.UploadNotificationImage(ctx, rwAID, "rw_cash_out", "kas_out.png", bytes.NewReader(pngBytes), int64(len(pngBytes)))
	if err != nil {
		t.Fatalf("upload kas_out img: %v", err)
	}
	imgKasOut := "/api/v1/notifications/images/" + metaKasOut.Key

	// Insert RW templates for manual_push, rw_cash_in, rw_cash_out
	var tmplManualID, tmplInID, tmplOutID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rw_id, name, event_type, channel, title, body, is_active, image_url, variables)
		VALUES ($1, 'Manual Img Tmpl', 'manual_push', 'push', 'Judul Manual', 'Isi Manual', true, $2, '[]'::jsonb) RETURNING id`,
		rwAID, imgManual).Scan(&tmplManualID); err != nil {
		t.Fatalf("insert manual template: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rw_id, name, event_type, channel, title, body, is_active, image_url, variables)
		VALUES ($1, 'Kas Masuk Tmpl', 'rw_cash_in', 'push', 'Kas Masuk {{amount}}', 'Ket: {{description}}', true, $2, '["amount","description"]'::jsonb) RETURNING id`,
		rwAID, imgKasIn).Scan(&tmplInID); err != nil {
		t.Fatalf("insert kas in template: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO notification_templates (rw_id, name, event_type, channel, title, body, is_active, image_url, variables)
		VALUES ($1, 'Kas Keluar Tmpl', 'rw_cash_out', 'push', 'Kas Keluar {{amount}}', 'Ket: {{description}}', true, $2, '["amount","description"]'::jsonb) RETURNING id`,
		rwAID, imgKasOut).Scan(&tmplOutID); err != nil {
		t.Fatalf("insert kas out template: %v", err)
	}

	r := chi.NewRouter()
	r.Post("/api/v1/rw/{rwId}/notifications/manual", rwH.SendManualPush)
	r.Post("/api/v1/rw/{rwId}/notifications/templates/{id}/send", rwH.SendRWTemplateNotification)
	r.Get("/api/v1/notifications/logs", notifH.ListLogs)
	r.Get("/api/v1/notifications/detail/{id}", notifH.GetNotificationDetail)
	r.Get("/api/v1/notifications/images/*", notifH.GetImage)

	acKetuaRWA := &auth.AuthContext{
		UserID: ketuaRWA,
		RWMemberships: []auth.RWMembership{
			{RWID: rwAID, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}
	acKetuaRTA := &auth.AuthContext{
		UserID:     ketuaRTA,
		RTID:       rtAID,
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
		RWMemberships: []auth.RWMembership{
			{RWID: rwAID, Jabatan: "ketua", Source: auth.RWMembershipSourceRTInferred},
		},
	}
	acKetuaRWB := &auth.AuthContext{
		UserID: ketuaRWB,
		RTID:   rtBID,
		RWMemberships: []auth.RWMembership{
			{RWID: rwBID, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}

	withAuth := func(req *http.Request, ac *auth.AuthContext) *http.Request {
		if ac == nil {
			return req
		}
		return req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))
	}

	// Path A: Manual push from template (also verify cross-RW image_url rejected with 403)
	badBody, _ := json.Marshal(map[string]any{
		"title": "Bad", "body": "Cross RW",
		"image_url": "/api/v1/notifications/images/notifications/rw-manual/" + rwBID + "/stolen.png",
	})
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/rw/"+rwAID+"/notifications/manual", bytes.NewReader(badBody)), acKetuaRWA))
	if recBad.Code != http.StatusForbidden {
		t.Fatalf("cross-RW image_url in SendManualPush expected 403, got %d: %s", recBad.Code, recBad.Body.String())
	}

	pathABody, _ := json.Marshal(map[string]any{"template_id": tmplManualID, "target": "all_pengurus"})
	recA := httptest.NewRecorder()
	r.ServeHTTP(recA, withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/rw/"+rwAID+"/notifications/manual", bytes.NewReader(pathABody)), acKetuaRWA))
	if recA.Code != http.StatusOK {
		t.Fatalf("Path A SendManualPush expected 200, got %d: %s", recA.Code, recA.Body.String())
	}

	// Path B: Send from template endpoint
	pathBBody, _ := json.Marshal(map[string]any{"target": "all_pengurus"})
	recB := httptest.NewRecorder()
	r.ServeHTTP(recB, withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/rw/"+rwAID+"/notifications/templates/"+tmplManualID+"/send", bytes.NewReader(pathBBody)), acKetuaRWA))
	if recB.Code != http.StatusOK {
		t.Fatalf("Path B SendRWTemplateNotification expected 200, got %d: %s", recB.Code, recB.Body.String())
	}

	// Path C: Automatic Kas Masuk RW
	notifSvc.NotifyRWCashIn(ctx, rwAID, uuid.NewString(), "250000.00", "Iuran Bulanan", "Kas Utama")
	// Path D: Automatic Kas Keluar RW
	notifSvc.NotifyRWCashOut(ctx, rwAID, uuid.NewString(), "100000.00", "Beli Lampu", "Operasional", "Kas Utama")

	// Wait briefly for async Kas notifications to persist
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var cnt int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE rw_id = $1 AND user_id = $2`, rwAID, ketuaRTA).Scan(&cnt)
		if cnt >= 4 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Delete ALL 3 templates NOW to prove sent logs keep their own image_url snapshot!
	_, err = db.ExecContext(ctx, `DELETE FROM notification_templates WHERE id IN ($1, $2, $3)`, tmplManualID, tmplInID, tmplOutID)
	if err != nil {
		t.Fatalf("delete templates: %v", err)
	}

	// Verify recipient (Ketua RT A) sees all 4 notifications in GET /api/v1/notifications/logs with non-empty image_url
	recLogs := httptest.NewRecorder()
	r.ServeHTTP(recLogs, withAuth(httptest.NewRequest(http.MethodGet, "/api/v1/notifications/logs?page=1&page_size=20", nil), acKetuaRTA))
	if recLogs.Code != http.StatusOK {
		t.Fatalf("ListLogs expected 200, got %d: %s", recLogs.Code, recLogs.Body.String())
	}
	var logsResp struct {
		Data []struct {
			ID        string            `json:"id"`
			EventType string            `json:"event_type"`
			ImageURL  *string           `json:"image_url"`
			Payload   map[string]string `json:"payload"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recLogs.Body.Bytes(), &logsResp); err != nil {
		t.Fatalf("decode logsResp: %v", err)
	}

	seenManual, seenIn, seenOut := 0, 0, 0
	for _, entry := range logsResp.Data {
		if entry.ImageURL == nil || *entry.ImageURL == "" {
			t.Errorf("notification %s (%s) missing top-level image_url after template deletion", entry.ID, entry.EventType)
			continue
		}
		if entry.Payload["image_url"] != *entry.ImageURL {
			t.Errorf("notification %s payload.image_url=%q != image_url=%q", entry.ID, entry.Payload["image_url"], *entry.ImageURL)
		}
		switch entry.EventType {
		case "manual_push":
			if *entry.ImageURL == imgManual {
				seenManual++
			}
		case "rw_cash_in":
			if *entry.ImageURL == imgKasIn {
				seenIn++
			}
		case "rw_cash_out":
			if *entry.ImageURL == imgKasOut {
				seenOut++
			}
		}

		// Verify Detail API returns image_url too
		recDetail := httptest.NewRecorder()
		r.ServeHTTP(recDetail, withAuth(httptest.NewRequest(http.MethodGet, "/api/v1/notifications/detail/"+entry.ID, nil), acKetuaRTA))
		if recDetail.Code != http.StatusOK {
			t.Fatalf("GetNotificationDetail(%s) expected 200, got %d: %s", entry.ID, recDetail.Code, recDetail.Body.String())
		}
		var detailResp struct {
			Data struct {
				ImageURL *string `json:"image_url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recDetail.Body.Bytes(), &detailResp); err != nil {
			t.Fatalf("decode detailResp: %v", err)
		}
		if detailResp.Data.ImageURL == nil || *detailResp.Data.ImageURL != *entry.ImageURL {
			t.Errorf("Detail API image_url mismatch for %s", entry.ID)
		}
	}

	if seenManual < 2 || seenIn < 1 || seenOut < 1 {
		t.Fatalf("expected all 4 paths with preserved images (manual>=2, in>=1, out>=1), got manual=%d in=%d out=%d", seenManual, seenIn, seenOut)
	}

	// Verify FCM mock messages carried image_url in data payload
	fcmImgs := map[string]bool{}
	for _, m := range mockPush.GetMessages() {
		if v := m.Data["image_url"]; v != "" {
			fcmImgs[v] = true
		}
	}
	for _, want := range []string{imgManual, imgKasIn, imgKasOut} {
		if !fcmImgs[want] {
			t.Errorf("expected FCM payload to carry image_url %q", want)
		}
	}

	// Verify authenticated image retrieval: 200 for authorized recipient, 403 for cross-RW user, 401 for unauth
	for _, url := range []string{imgManual, imgKasIn, imgKasOut} {
		recOK := httptest.NewRecorder()
		r.ServeHTTP(recOK, withAuth(httptest.NewRequest(http.MethodGet, url, nil), acKetuaRTA))
		if recOK.Code != http.StatusOK {
			t.Fatalf("authorized recipient GET %s expected 200, got %d", url, recOK.Code)
		}

		recForbidden := httptest.NewRecorder()
		r.ServeHTTP(recForbidden, withAuth(httptest.NewRequest(http.MethodGet, url, nil), acKetuaRWB))
		if recForbidden.Code != http.StatusForbidden {
			t.Fatalf("cross-RW user GET %s expected 403, got %d", url, recForbidden.Code)
		}

		recUnauth := httptest.NewRecorder()
		r.ServeHTTP(recUnauth, httptest.NewRequest(http.MethodGet, url, nil))
		if recUnauth.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated GET %s expected 401, got %d", url, recUnauth.Code)
		}
	}
}
