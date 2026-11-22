package rw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

// minimal valid 1x1 PNG bytes
var validTestPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

func makeMultipartImageReq(t *testing.T, url, eventType, filename string, content []byte, ac *auth.AuthContext) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if eventType != "" {
		require.NoError(t, mw.WriteField("event_type", eventType))
	}
	if filename != "" {
		part, err := mw.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if ac != nil {
		req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))
	}
	return req
}

func TestRWNotificationImageUpload_AuthRBACScopeValidationAndRetrieval(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Seed two isolated active RWs
	baseNum := int(time.Now().UnixNano()/1000000)%50000 + 30000
	var rw1ID, rw2ID string
	require.NoError(t, pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Img 1', $1, true) RETURNING id`, baseNum,
	).Scan(&rw1ID))
	require.NoError(t, pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Img 2', $1, true) RETURNING id`, baseNum+1,
	).Scan(&rw2ID))
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id IN ($1, $2)`, rw1ID, rw2ID)
	})

	store := storage.NewMockStorage()
	svc := NewService(pool, NewRepository(pool.Raw()), &auth.ScopingConfig{RWIDs: []string{rw1ID, rw2ID}})
	rwH := NewHandler(pool, svc, nil, store)
	notifH := notification.NewHandler(nil, pool, store)

	r := chi.NewRouter()
	r.Post("/api/v1/rw/{rwId}/notifications/upload-image", rwH.UploadNotificationImage)
	r.Get("/api/v1/notifications/images/*", notifH.GetImage)

	ketuaRW1 := &auth.AuthContext{
		UserID: uuid.NewString(),
		RWMemberships: []auth.RWMembership{
			{RWID: rw1ID, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}
	sekretarisRW1 := &auth.AuthContext{
		UserID: uuid.NewString(),
		RWMemberships: []auth.RWMembership{
			{RWID: rw1ID, Jabatan: "sekretaris", Source: auth.RWMembershipSourceExplicit},
		},
	}
	inferredRW1 := &auth.AuthContext{
		UserID: uuid.NewString(),
		RWMemberships: []auth.RWMembership{
			{RWID: rw1ID, Jabatan: "ketua", Source: auth.RWMembershipSourceRTInferred},
		},
	}
	ketuaRW2 := &auth.AuthContext{
		UserID: uuid.NewString(),
		RWMemberships: []auth.RWMembership{
			{RWID: rw2ID, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}
	superAdmin := &auth.AuthContext{
		UserID:     uuid.NewString(),
		SystemRole: auth.SystemRoleSuperAdmin,
	}

	// 1. Unauthenticated upload -> 401
	t.Run("unauthenticated upload returns 401", func(t *testing.T) {
		req := makeMultipartImageReq(t, "/api/v1/rw/"+rw1ID+"/notifications/upload-image", "manual_push", "photo.png", validTestPNG, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// 2. Read-only pengurus RW & rt_inferred -> 403
	for name, ac := range map[string]*auth.AuthContext{
		"sekretaris read-only": sekretarisRW1,
		"rt_inferred":          inferredRW1,
		"cross-RW ketua":       ketuaRW2,
	} {
		t.Run("forbidden upload for "+name, func(t *testing.T) {
			req := makeMultipartImageReq(t, "/api/v1/rw/"+rw1ID+"/notifications/upload-image", "manual_push", "photo.png", validTestPNG, ac)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusForbidden, rec.Code, "body=%s", rec.Body.String())
		})
	}

	// 3. Disallowed event types -> 400
	for _, ev := range []string{"iuran_payment_reminder", "waste_truck_arrival", "unknown_event"} {
		t.Run("disallowed event rejected: "+ev, func(t *testing.T) {
			req := makeMultipartImageReq(t, "/api/v1/rw/"+rw1ID+"/notifications/upload-image", ev, "photo.png", validTestPNG, ketuaRW1)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
		})
	}

	// 4. Invalid file type / empty file -> 400
	t.Run("non-image file rejected", func(t *testing.T) {
		req := makeMultipartImageReq(t, "/api/v1/rw/"+rw1ID+"/notifications/upload-image", "manual_push", "evil.txt", []byte("not an image"), ketuaRW1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	// 5. Allowed RW events (manual_push, rw_cash_in, rw_cash_out) succeed for Ketua RW1 & Super Admin,
	//    and authenticated GetImage enforces strict RW scope (same RW 200, cross-RW 403, unauth 401).
	for _, tc := range []struct {
		eventType        string
		expectedCategory string
		caller           *auth.AuthContext
	}{
		{"manual_push", "rw-manual", ketuaRW1},
		{"rw_cash_in", "rw-kas", ketuaRW1},
		{"rw_cash_out", "rw-kas", superAdmin},
	} {
		t.Run("upload and scoped retrieval for "+tc.eventType, func(t *testing.T) {
			req := makeMultipartImageReq(t, "/api/v1/rw/"+rw1ID+"/notifications/upload-image", tc.eventType, "camera.png", validTestPNG, tc.caller)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

			var resp struct {
				Status   string `json:"status"`
				ImageURL string `json:"image_url"`
				Key      string `json:"key"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, "uploaded", resp.Status)
			require.True(t, strings.HasPrefix(resp.Key, "notifications/"+tc.expectedCategory+"/"+rw1ID+"/"), "unexpected key: %s", resp.Key)
			require.Equal(t, "/api/v1/notifications/images/"+resp.Key, resp.ImageURL)

			// Unauthenticated image GET -> 401
			getUnauth := httptest.NewRequest(http.MethodGet, resp.ImageURL, nil)
			recUnauth := httptest.NewRecorder()
			r.ServeHTTP(recUnauth, getUnauth)
			require.Equal(t, http.StatusUnauthorized, recUnauth.Code)

			// Cross-RW member image GET -> 403
			getCross := httptest.NewRequest(http.MethodGet, resp.ImageURL, nil)
			getCross = getCross.WithContext(context.WithValue(getCross.Context(), auth.AuthContextKey, ketuaRW2))
			recCross := httptest.NewRecorder()
			r.ServeHTTP(recCross, getCross)
			require.Equal(t, http.StatusForbidden, recCross.Code)

			// Same-RW member (even read-only sekretaris RW1) image GET -> 200
			getSame := httptest.NewRequest(http.MethodGet, resp.ImageURL, nil)
			getSame = getSame.WithContext(context.WithValue(getSame.Context(), auth.AuthContextKey, sekretarisRW1))
			recSame := httptest.NewRecorder()
			r.ServeHTTP(recSame, getSame)
			require.Equal(t, http.StatusOK, recSame.Code)
			require.Equal(t, validTestPNG, recSame.Body.Bytes())
		})
	}
}

func TestRWNotificationImageUpload_2_1MB_And_5MB_Limits(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	baseNum := int(time.Now().UnixNano()/1000000)%50000 + 45000
	var rwID string
	require.NoError(t, pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Sizing Test', $1, true) RETURNING id`, baseNum,
	).Scan(&rwID))
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})

	store := storage.NewMockStorage()
	svc := NewService(pool, NewRepository(pool.Raw()), &auth.ScopingConfig{RWIDs: []string{rwID}})
	rwH := NewHandler(pool, svc, nil, store)

	r := chi.NewRouter()
	r.Post("/api/v1/rw/{rwId}/notifications/upload-image", rwH.UploadNotificationImage)

	caller := &auth.AuthContext{
		UserID: uuid.NewString(),
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}

	// 1. 2.1 MB file upload -> MUST SUCCEED (201 Created)
	t.Run("upload 2.1 MB file succeeds", func(t *testing.T) {
		size2_1MB := 2202010
		data2_1MB := make([]byte, size2_1MB)
		copy(data2_1MB, validTestPNG)

		req := makeMultipartImageReq(t, "/api/v1/rw/"+rwID+"/notifications/upload-image", "manual_push", "large-2.1mb.png", data2_1MB, caller)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code, "2.1 MB upload should return 201: %s", rec.Body.String())

		var resp struct {
			Status   string `json:"status"`
			ImageURL string `json:"image_url"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "uploaded", resp.Status)
		require.NotEmpty(t, resp.ImageURL)
	})

	// 2. Exactly 5 MiB file upload -> MUST SUCCEED (201 Created)
	t.Run("upload exact 5 MiB file succeeds", func(t *testing.T) {
		size5MiB := 5 * 1024 * 1024
		data5MiB := make([]byte, size5MiB)
		copy(data5MiB, validTestPNG)

		req := makeMultipartImageReq(t, "/api/v1/rw/"+rwID+"/notifications/upload-image", "manual_push", "exact-5mib.png", data5MiB, caller)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code, "exact 5 MiB upload should return 201: %s", rec.Body.String())
	})

	// 3. 5.1 MB file upload -> MUST BE REJECTED with 413 Payload Too Large
	t.Run("upload >5 MiB file rejected with 413", func(t *testing.T) {
		size5_1MB := 5347738
		data5_1MB := make([]byte, size5_1MB)
		copy(data5_1MB, validTestPNG)

		req := makeMultipartImageReq(t, "/api/v1/rw/"+rwID+"/notifications/upload-image", "manual_push", "over-5mib.png", data5_1MB, caller)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, "over 5 MiB upload should return 413: %s", rec.Body.String())
	})
}

func TestRWNotification_Template_And_ManualPush_ImageDistribution(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	baseNum := int(time.Now().UnixNano()/1000000)%50000 + 48000
	var rwID, rtID string
	require.NoError(t, pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Distribution Test', $1, true) RETURNING id`, baseNum,
	).Scan(&rwID))
	require.NoError(t, pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('RT Test Dist', 16, '001', $1, true) RETURNING id`, rwID,
	).Scan(&rtID))

	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM notification_logs WHERE rw_id = $1`, rwID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM notification_templates WHERE rw_id = $1`, rwID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id = $1`, rtID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})

	// Create recipient user (Ketua RT under this RW)
	recipientUID := uuid.NewString()
	_, _ = pool.Raw().ExecContext(ctx, `INSERT INTO users (id, email, full_name, password_hash, is_active) VALUES ($1, $2, 'Ketua RT Dist', 'hash', true)`,
		recipientUID, fmt.Sprintf("ketua.dist.%d@test.com", baseNum))
	_, _ = pool.Raw().ExecContext(ctx, `INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active) VALUES ($1, $2, 'pengurus', 'ketua', true)`,
		recipientUID, rtID)
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, recipientUID)
	})

	store := storage.NewMockStorage()
	notifRepo := notification.NewRepository(pool)
	notifSvc := notification.NewService(notifRepo, notification.NewMockPushProvider())
	rwRepo := NewRepository(pool.Raw())
	rwSvc := NewService(pool, rwRepo, &auth.ScopingConfig{RWIDs: []string{rwID}})
	rwSvc.SetNotifier(notifSvc)
	rwH := NewHandler(pool, rwSvc, notifSvc, store)

	r := chi.NewRouter()
	r.Post("/api/v1/rw/{rwId}/notifications/manual", rwH.SendManualPush)
	r.Post("/api/v1/rw/{rwId}/notifications/templates", rwH.CreateRWTemplate)
	r.Post("/api/v1/rw/{rwId}/notifications/templates/{id}/send", rwH.SendRWTemplateNotification)
	r.Get("/api/v1/rw/{rwId}/notifications", rwH.ListNotifications)
	r.Get("/api/v1/rw/{rwId}/notifications/mine", rwH.ListMyNotifications)

	senderCaller := &auth.AuthContext{
		UserID: uuid.NewString(),
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}
	_, _ = pool.Raw().ExecContext(ctx, `INSERT INTO users (id, email, full_name, password_hash, is_active) VALUES ($1, $2, 'Sender Ketua RW', 'hash', true)`,
		senderCaller.UserID, fmt.Sprintf("sender.%d@test.com", baseNum))
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, senderCaller.UserID)
	})

	// 1. Send Manual Push with Image -> Image URL must be distributed to notification logs
	t.Run("manual push with image persists image_url in log and payload", func(t *testing.T) {
		testImgURL := "/api/v1/notifications/images/rw-manual/" + rwID + "/manual-photo.png"
		manualBody := map[string]any{
			"title":     "Pengumuman Kerja Bakti",
			"body":      "Diharapkan kehadiran pengurus RW dan RT.",
			"target":    "all_pengurus",
			"image_url": testImgURL,
		}
		b, _ := json.Marshal(manualBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/rw/"+rwID+"/notifications/manual", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, senderCaller))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "manual push send response: %s", rec.Body.String())

		// Verify notification log has image_url
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/rw/"+rwID+"/notifications", nil)
		listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.AuthContextKey, senderCaller))
		listRec := httptest.NewRecorder()
		r.ServeHTTP(listRec, listReq)
		require.Equal(t, http.StatusOK, listRec.Code, "list response: %s", listRec.Body.String())

		var listResp struct {
			Data []notification.NotificationLogEntry `json:"data"`
		}
		require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
		require.NotEmpty(t, listResp.Data)

		found := false
		for _, entry := range listResp.Data {
			if entry.Title == "Pengumuman Kerja Bakti" {
				require.NotNil(t, entry.ImageURL, "entry.ImageURL must not be nil")
				require.Equal(t, testImgURL, *entry.ImageURL)
				require.Equal(t, testImgURL, entry.Payload["image_url"], "payload['image_url'] must match")
				found = true
				break
			}
		}
		require.True(t, found, "sent manual notification must be found in logs with image_url")
	})

	// 2. Send RW Template with Image -> Template's image_url must be forwarded and received
	t.Run("send template notification with image propagates image_url to recipient", func(t *testing.T) {
		tmplImgURL := "/api/v1/notifications/images/rw-manual/" + rwID + "/template-photo.png"
		tmplReqBody := map[string]any{
			"name":       "Template Gotong Royong",
			"event_type": "manual_push",
			"title":      "Jadwal Gotong Royong",
			"body":       "Pemberitahuan gotong royong tingkat RW.",
			"is_active":  true,
			"image_url":  tmplImgURL,
		}
		tb, _ := json.Marshal(tmplReqBody)
		createTmplReq := httptest.NewRequest(http.MethodPost, "/api/v1/rw/"+rwID+"/notifications/templates", bytes.NewReader(tb))
		createTmplReq.Header.Set("Content-Type", "application/json")
		createTmplReq = createTmplReq.WithContext(context.WithValue(createTmplReq.Context(), auth.AuthContextKey, senderCaller))
		createTmplRec := httptest.NewRecorder()
		r.ServeHTTP(createTmplRec, createTmplReq)
		require.Equal(t, http.StatusCreated, createTmplRec.Code, "create template response: %s", createTmplRec.Body.String())

		var tmplResp struct {
			Data notification.NotificationTemplate `json:"data"`
		}
		require.NoError(t, json.Unmarshal(createTmplRec.Body.Bytes(), &tmplResp))
		tmplID := tmplResp.Data.ID

		// Send template notification
		sendReq := httptest.NewRequest(http.MethodPost, "/api/v1/rw/"+rwID+"/notifications/templates/"+tmplID+"/send", bytes.NewReader([]byte(`{"target":"all_pengurus"}`)))
		sendReq.Header.Set("Content-Type", "application/json")
		sendReq = sendReq.WithContext(context.WithValue(sendReq.Context(), auth.AuthContextKey, senderCaller))
		sendRec := httptest.NewRecorder()
		r.ServeHTTP(sendRec, sendReq)
		require.Equal(t, http.StatusOK, sendRec.Code, "send template response: %s", sendRec.Body.String())

		// Verify in My Notifications (ListRWMyLogs)
		myReq := httptest.NewRequest(http.MethodGet, "/api/v1/rw/"+rwID+"/notifications/mine", nil)
		myReq = myReq.WithContext(context.WithValue(myReq.Context(), auth.AuthContextKey, senderCaller))
		myRec := httptest.NewRecorder()
		r.ServeHTTP(myRec, myReq)
		require.Equal(t, http.StatusOK, myRec.Code, "my logs response: %s", myRec.Body.String())

		var myResp struct {
			Data []notification.NotificationLogEntry `json:"data"`
		}
		require.NoError(t, json.Unmarshal(myRec.Body.Bytes(), &myResp))
		require.NotEmpty(t, myResp.Data)

		foundTmplLog := false
		for _, entry := range myResp.Data {
			if entry.Title == "Jadwal Gotong Royong" {
				require.NotNil(t, entry.ImageURL, "entry.ImageURL must not be nil for template send")
				require.Equal(t, tmplImgURL, *entry.ImageURL)
				foundTmplLog = true
				break
			}
		}
		require.True(t, foundTmplLog, "template notification log must have image_url")
	})
}
