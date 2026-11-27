package notification_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/storage"
)

func TestBuildFCMMessagePayload_PrivateImageUsesAndroidDataOnlyAndAPNsAlert(t *testing.T) {
	payload := notification.BuildFCMMessagePayload(
		"fcm-token-abc",
		"Pengumuman RT 002",
		"Kerja bakti hari Minggu pukul 07.00.",
		map[string]string{
			"notification_id": "notif-123",
			"type":            notification.EventManualPush,
			"deep_link":       "/home/notifikasi/detail/notif-123",
			"image_url":       "/api/v1/notifications/images/notifications/manual/rt-002/banner.jpg",
		},
	)

	msg, ok := payload["message"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level message map")
	}
	if msg["token"] != "fcm-token-abc" {
		t.Fatalf("unexpected token: %v", msg["token"])
	}

	// Private authenticated image notifications MUST omit top-level notification
	// and android.notification so Android background delivery invokes
	// SocialFinanceMessagingService (authenticated fetch + BigPictureStyle)
	// without Firebase auto-displaying a text-only duplicate.
	if _, hasNotif := msg["notification"]; hasNotif {
		t.Fatalf("private image FCM payload must omit top-level notification block for Android data-only background BigPictureStyle")
	}

	dataBlock, ok := msg["data"].(map[string]string)
	if !ok {
		t.Fatalf("expected data block")
	}
	if dataBlock["title"] != "Pengumuman RT 002" || dataBlock["body"] != "Kerja bakti hari Minggu pukul 07.00." {
		t.Fatalf("expected data title/body populated, got: %+v", dataBlock)
	}
	if dataBlock["image_url"] != "/api/v1/notifications/images/notifications/manual/rt-002/banner.jpg" {
		t.Fatalf("expected data image_url preserved, got: %q", dataBlock["image_url"])
	}
	if dataBlock["channel_id"] != notification.AndroidNotificationChannelID {
		t.Fatalf("expected data channel_id=%q, got %q", notification.AndroidNotificationChannelID, dataBlock["channel_id"])
	}

	androidBlock, ok := msg["android"].(map[string]any)
	if !ok {
		t.Fatalf("expected android block")
	}
	if androidBlock["priority"] != "HIGH" {
		t.Fatalf("expected android.priority=HIGH, got %v", androidBlock["priority"])
	}
	if _, hasAndroidNotif := androidBlock["notification"]; hasAndroidNotif {
		t.Fatalf("private image FCM payload must omit android.notification to avoid unauthenticated/duplicate background display")
	}

	apnsBlock, ok := msg["apns"].(map[string]any)
	if !ok {
		t.Fatalf("expected apns block for iOS")
	}
	apnsHeaders, ok := apnsBlock["headers"].(map[string]string)
	if !ok || apnsHeaders["apns-priority"] != "10" || apnsHeaders["apns-push-type"] != "alert" {
		t.Fatalf("expected apns-priority=10 and apns-push-type=alert, got %+v", apnsBlock["headers"])
	}
}

func TestBuildFCMMessagePayload_TextOnlyKeepsAndroidNotificationBlockAndChannelV2(t *testing.T) {
	payload := notification.BuildFCMMessagePayload(
		"fcm-token-xyz",
		"Info RT 002",
		"Rapat warga pukul 19.30 WIB.",
		map[string]string{
			"notification_id": "notif-456",
			"type":            notification.EventManualPush,
			"deep_link":       "/home/notifikasi/detail/notif-456",
		},
	)

	msg := payload["message"].(map[string]any)
	notifBlock, ok := msg["notification"].(map[string]string)
	if !ok {
		t.Fatalf("expected notification block for text-only push")
	}
	if notifBlock["title"] != "Info RT 002" || notifBlock["body"] != "Rapat warga pukul 19.30 WIB." {
		t.Fatalf("unexpected notification title/body: %+v", notifBlock)
	}
	if _, hasImg := notifBlock["image"]; hasImg {
		t.Fatalf("text-only notification must not include image field")
	}

	androidBlock := msg["android"].(map[string]any)
	if androidBlock["priority"] != "HIGH" {
		t.Fatalf("expected android.priority=HIGH, got %v", androidBlock["priority"])
	}
	androidNotif, ok := androidBlock["notification"].(map[string]any)
	if !ok {
		t.Fatalf("expected android.notification block for text-only push")
	}
	if androidNotif["channel_id"] != notification.AndroidNotificationChannelID {
		t.Fatalf("expected channel_id=%q, got %v", notification.AndroidNotificationChannelID, androidNotif["channel_id"])
	}
	if androidNotif["notification_priority"] != "PRIORITY_MAX" {
		t.Fatalf("expected notification_priority=PRIORITY_MAX, got %v", androidNotif["notification_priority"])
	}
	if androidNotif["default_sound"] != true || androidNotif["default_vibrate_timings"] != true {
		t.Fatalf("expected default_sound & default_vibrate_timings true, got %+v", androidNotif)
	}
	if androidNotif["visibility"] != "PUBLIC" {
		t.Fatalf("expected visibility=PUBLIC, got %v", androidNotif["visibility"])
	}
}

type failingStorageService struct {
	*mockStorageService
	failKey string
}

func (f *failingStorageService) GetNotificationImage(ctx context.Context, key string) (*storage.ObjectReader, error) {
	if key == f.failKey {
		return nil, errors.New("simulated rustfs connection reset")
	}
	return f.mockStorageService.GetNotificationImage(ctx, key)
}

func TestGetImage_FullAuthScopingAndContentTypeMatrix(t *testing.T) {
	pool := setupNotificationTestPool(t)
	ctx := context.Background()

	rwID := uuid.NewString()
	rtAID := uuid.NewString()
	rtBID := uuid.NewString()
	rwNum := 810 + int(uuid.New().ID()%80)
	_, err := pool.Raw().ExecContext(ctx, `INSERT INTO rws (id, rw_number, name, is_active) VALUES ($1, $2, $3, true)`, rwID, rwNum, "RW Matrix "+rwID[:6])
	if err != nil {
		t.Fatalf("insert RW failed: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx, `INSERT INTO rts (id, rw_id, rw, rt, name, is_active) VALUES ($1, $2, $3, $4, 'RT Matrix A', true)`, rtAID, rwID, rwNum, "871-"+rtAID[:5])
	if err != nil {
		t.Fatalf("insert RT A failed: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx, `INSERT INTO rts (id, rw, rt, name, is_active) VALUES ($1, $2, $3, 'RT Matrix B', true)`, rtBID, rwNum+1, "872-"+rtBID[:5])
	if err != nil {
		t.Fatalf("insert RT B failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id IN ($1, $2)`, rtAID, rtBID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})

	baseStore := newMockStorageService()
	jpegBytes := []byte("\xFF\xD8\xFF\xE0fake-jpeg-payload")
	rtAKey := "notifications/manual/" + rtAID + "/photo.jpg"
	rtAFailKey := "notifications/manual/" + rtAID + "/broken_io.jpg"
	rwKey := "notifications/rw-manual/" + rwID + "/rw_photo.webp"

	// Simulate object stored with generic application/octet-stream Content-Type to verify extension inference.
	baseStore.objects[rtAKey] = jpegBytes
	baseStore.meta[rtAKey] = storage.ObjectMetadata{Key: rtAKey, ContentType: "application/octet-stream", Size: int64(len(jpegBytes))}
	baseStore.objects[rwKey] = jpegBytes
	baseStore.meta[rwKey] = storage.ObjectMetadata{Key: rwKey, ContentType: "", Size: int64(len(jpegBytes))}

	store := &failingStorageService{
		mockStorageService: baseStore,
		failKey:            rtAFailKey,
	}

	h := notification.NewHandler(nil, pool, store, "test")
	h.SetScopingConfig(auth.ScopingConfig{
		RTIDs: []string{rtAID},
		RWIDs: []string{rwID},
	})

	router := chi.NewRouter()
	router.Get("/api/v1/notifications/images/*", h.GetImage)

	doGet := func(key string, ac *auth.AuthContext, headerRT string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/images/"+key, nil)
		if headerRT != "" {
			req.Header.Set("X-RT-ID", headerRT)
		}
		if ac != nil {
			req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// 1. Authorized RT A user gets 200 and inferred image/jpeg content-type
	rtAUser := &auth.AuthContext{UserID: uuid.NewString(), TenantRole: auth.RoleWarga, RTID: rtAID}
	rec := doGet(rtAKey, rtAUser, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for RT A user, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("expected inferred Content-Type image/jpeg, got %q", rec.Header().Get("Content-Type"))
	}

	// 2. Cross-RT user spoofing X-RT-ID header for RT A must STILL get 403 Forbidden
	rtBUser := &auth.AuthContext{UserID: uuid.NewString(), TenantRole: auth.RoleWarga, RTID: rtBID}
	if rec := doGet(rtAKey, rtBUser, rtAID); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-RT user spoofing X-RT-ID, got %d", rec.Code)
	}

	// 3. RW-only user (empty RTID, valid RWMemberships) accessing RW image -> 200 OK with inferred image/webp
	rwOnlyUser := &auth.AuthContext{
		UserID:        uuid.NewString(),
		RWMemberships: []auth.RWMembership{{RWID: rwID, Jabatan: string(auth.JabatanKetua)}},
	}
	rec = doGet(rwKey, rwOnlyUser, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for RW-only member accessing RW image, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/webp" {
		t.Fatalf("expected inferred Content-Type image/webp, got %q", rec.Header().Get("Content-Type"))
	}

	// 4. SuperAdmin with empty RTID accessing out-of-scope RW image -> 403 Forbidden
	sa := &auth.AuthContext{UserID: uuid.NewString(), SystemRole: auth.SystemRoleSuperAdmin}
	outOfScopeRWKey := "notifications/rw-manual/" + uuid.NewString() + "/other.png"
	if rec := doGet(outOfScopeRWKey, sa, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for SuperAdmin accessing out-of-scope RW image, got %d", rec.Code)
	}

	// 5. Missing key in authorized RT -> 404 Not Found
	missingKey := "notifications/manual/" + rtAID + "/does_not_exist.jpg"
	if rec := doGet(missingKey, rtAUser, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing object, got %d", rec.Code)
	}

	// 6. Storage I/O failure -> 502 Bad Gateway (not masked as 404)
	if rec := doGet(rtAFailKey, rtAUser, ""); rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for storage I/O error, got %d", rec.Code)
	}
}
