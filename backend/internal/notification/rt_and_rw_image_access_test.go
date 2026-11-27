package notification_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/storage"
)

func TestGetImage_RTAndRWScopeAuthorization(t *testing.T) {
	pool := setupNotificationTestPool(t)
	ctx := context.Background()

	// Create RW + RT A (in admin scope) + RT B (out of admin scope)
	rwID := uuid.NewString()
	rtAID := uuid.NewString()
	rtBID := uuid.NewString()
	rwNum := 800 + int(uuid.New().ID()%90)
	_, err := pool.Raw().ExecContext(ctx, `INSERT INTO rws (id, rw_number, name, is_active) VALUES ($1, $2, $3, true)`, rwID, rwNum, "RW ImgTest "+rwID[:6])
	if err != nil {
		t.Fatalf("insert RW failed: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx, `INSERT INTO rts (id, rw_id, rw, rt, name, is_active) VALUES ($1, $2, $3, $4, 'RT ImgTest A', true)`, rtAID, rwID, rwNum, "881-"+rtAID[:5])
	if err != nil {
		t.Fatalf("insert RT A failed: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx, `INSERT INTO rts (id, rw, rt, name, is_active) VALUES ($1, $2, $3, 'RT OutOfScope B', true)`, rtBID, rwNum+1, "891-"+rtBID[:5])
	if err != nil {
		t.Fatalf("insert RT B failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id IN ($1, $2)`, rtAID, rtBID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
	})

	store := newMockStorageService()
	pngBytes := []byte("\x89PNG\r\n\x1a\nfake-image-payload")
	rtAKey := "notifications/waste-truck/" + rtAID + "/truck.png"
	rtBKey := "notifications/manual/" + rtBID + "/manual.png"
	rwKey := "notifications/rw-manual/" + rwID + "/rw_banner.png"
	for _, k := range []string{rtAKey, rtBKey, rwKey} {
		store.objects[k] = pngBytes
		store.meta[k] = storage.ObjectMetadata{Key: k, ContentType: "image/png", Size: int64(len(pngBytes))}
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

	// 1. Unauthenticated -> 401
	if rec := doGet(rtAKey, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthenticated for RT image, got %d", rec.Code)
	}

	// 2. Authorized RT A user -> 200 OK
	rtAUser := &auth.AuthContext{UserID: uuid.NewString(), TenantRole: auth.RoleWarga, RTID: rtAID}
	rec := doGet(rtAKey, rtAUser, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized RT A user, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "private, max-age=3600" {
		t.Fatalf("expected private cache control, got %q", rec.Header().Get("Cache-Control"))
	}

	// 3. Cross-RT user (RT B requesting RT A image) -> 403 Forbidden
	rtBUser := &auth.AuthContext{UserID: uuid.NewString(), TenantRole: auth.RoleWarga, RTID: rtBID}
	if rec := doGet(rtAKey, rtBUser, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-RT user, got %d", rec.Code)
	}

	// 4. SuperAdmin WITH X-RT-ID matching RT A -> 200 OK
	sa := &auth.AuthContext{UserID: uuid.NewString(), SystemRole: auth.SystemRoleSuperAdmin}
	if rec := doGet(rtAKey, sa, rtAID); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SuperAdmin with X-RT-ID, got %d", rec.Code)
	}

	// 5. SuperAdmin WITHOUT X-RT-ID for RT A (inside admin scope) -> 200 OK (fixes Android RT notification image for SuperAdmin!)
	if rec := doGet(rtAKey, sa, ""); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SuperAdmin in-scope RT without X-RT-ID, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. SuperAdmin WITHOUT X-RT-ID for RT B (OUTSIDE admin scope) -> 403 Forbidden
	if rec := doGet(rtBKey, sa, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for SuperAdmin out-of-scope RT, got %d", rec.Code)
	}

	// 7. RW notification image still works for RT A user (whose RT belongs to RW 88) & SuperAdmin, denied for RT B user!
	if rec := doGet(rwKey, rtAUser, ""); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for RW image to RT A member in RW 88, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doGet(rwKey, sa, ""); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for RW image to SuperAdmin, got %d", rec.Code)
	}
	if rec := doGet(rwKey, rtBUser, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for RW image to outside RT B user, got %d", rec.Code)
	}
}
