package notification_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
)

// ============================================================================
// Handler-level security tests for notification device and notification
// detail endpoints.
//
// These tests verify cross-user isolation at the HTTP handler level, using
// an in-memory repository and the production handler code path.
// ============================================================================

// deviceAwareRepo extends inMemoryRepo with notification detail tracking
// needed for handler-level isolation tests.
type deviceAwareRepo struct {
	*inMemoryRepo
	logsMu    sync.Mutex
	logs      []notification.NotificationLogEntry
	nextLogID int
}

func newDeviceAwareRepo() *deviceAwareRepo {
	return &deviceAwareRepo{
		inMemoryRepo: newInMemoryRepo(),
		logs:         make([]notification.NotificationLogEntry, 0),
	}
}

func (r *deviceAwareRepo) AddLog(entry notification.NotificationLogEntry) {
	r.logsMu.Lock()
	defer r.logsMu.Unlock()
	r.nextLogID++
	entry.ID = "log-" + strconv.Itoa(r.nextLogID)
if entry.CreatedAt.IsZero() {
			entry.CreatedAt = *nowPtr()
	}
	r.logs = append(r.logs, entry)
}

func (r *deviceAwareRepo) GetLogByID(ctx context.Context, rtID, id string) (*notification.NotificationLogEntry, error) {
	r.logsMu.Lock()
	defer r.logsMu.Unlock()
	for _, l := range r.logs {
		if l.ID == id && l.RTID == rtID {
			copied := l
			return &copied, nil
		}
	}
	return nil, nil
}

func (r *deviceAwareRepo) ListLogs(ctx context.Context, rtID, userID, eventType, status string, isOfficer bool, offset, limit int) ([]notification.NotificationLogEntry, int, error) {
	r.logsMu.Lock()
	defer r.logsMu.Unlock()
	var filtered []notification.NotificationLogEntry
	for _, l := range r.logs {
		if l.RTID != rtID {
			continue
		}
		if !isOfficer && l.UserID != nil && *l.UserID != userID {
			continue
		}
		if eventType != "" && l.EventType != eventType {
			continue
		}
		if status != "" && l.Status != status {
			continue
		}
		filtered = append(filtered, l)
	}
	total := len(filtered)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
}

func (r *deviceAwareRepo) MarkAsRead(ctx context.Context, rtID, userID, id string, isOfficer bool) error {
	r.logsMu.Lock()
	defer r.logsMu.Unlock()
	for i, l := range r.logs {
		if l.ID == id && l.RTID == rtID {
			if !isOfficer && l.UserID != nil && *l.UserID != userID {
				return notification.ErrForbidden
			}
			r.logs[i].Status = "READ"
			r.logs[i].ReadAt = nowPtr()
			return nil
		}
	}
	return sql.ErrNoRows
}

func (r *deviceAwareRepo) MarkAllAsRead(ctx context.Context, rtID, userID string, isOfficer bool) (int, error) {
	r.logsMu.Lock()
	defer r.logsMu.Unlock()
	count := 0
	for i := range r.logs {
		if r.logs[i].RTID != rtID {
			continue
		}
		if !isOfficer && r.logs[i].UserID != nil && *r.logs[i].UserID != userID {
			continue
		}
		if r.logs[i].Status != "READ" {
			r.logs[i].Status = "READ"
			r.logs[i].ReadAt = nowPtr()
			count++
		}
	}
	return count, nil
}

func (r *deviceAwareRepo) GetUnreadCount(ctx context.Context, rtID, userID string, isOfficer bool) (int, error) {
	r.logsMu.Lock()
	defer r.logsMu.Unlock()
	count := 0
	for _, l := range r.logs {
		if l.RTID != rtID {
			continue
		}
		if l.Status == "READ" {
			continue
		}
		if !isOfficer && l.UserID != nil && *l.UserID != userID {
			continue
		}
		count++
	}
	return count, nil
}

func (r *deviceAwareRepo) ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
	return []notification.NotificationTemplate{}, nil
}

func (r *deviceAwareRepo) GetTemplateByID(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *deviceAwareRepo) GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *deviceAwareRepo) CreateTemplate(ctx context.Context, rtID string, in notification.CreateTemplateInput, allowedVars []string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *deviceAwareRepo) UpdateTemplate(ctx context.Context, rtID, id string, in notification.UpdateTemplateInput) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *deviceAwareRepo) ToggleTemplateActive(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *deviceAwareRepo) DeleteTemplate(ctx context.Context, rtID, id string) error {
	return nil
}

func (r *deviceAwareRepo) GetDefaultRTID(ctx context.Context) (string, error) {
	return "test-rt-002", nil
}

// setupNotificationRouter builds a router mirroring the production
// notification device/detail/read routes chain:
//   RequireAuth + RequireAllowedRT(pool)
//
// Since RequireAllowedRT takes *database.Pool, we use an inline middleware
// matching RequireAllowedRT(pool)'s whitelist logic against an in-memory
// RT code map.
func setupNotificationRouter(repo notification.Repository, svc notification.Service) http.Handler {
	r := chi.NewRouter()

	allowedRTCodes := map[string]string{
		"rt-002": "002",
		"rt-003": "003",
	}

	requireAllowedRT := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(auth.AuthContextKey).(*auth.AuthContext)
			if !ok || ac == nil {
				http.Error(w, `{"error":{"code":"unauthorized","message":"missing authentication"}}`, http.StatusUnauthorized)
				return
			}
			if ac.RTID == "" {
				http.Error(w, `{"error":{"code":"forbidden","message":"tenant context required"}}`, http.StatusForbidden)
				return
			}
			rtCode, found := allowedRTCodes[ac.RTID]
			if !found || !auth.AllowedRTCodes[rtCode] {
				http.Error(w, `{"error":{"code":"forbidden","message":"rt is not enabled or not allowed"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// Middleware must be defined BEFORE routes in chi
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac := r.Context().Value(auth.AuthContextKey)
			if ac == nil {
				http.Error(w, `{"error":{"code":"unauthorized","message":"missing authentication"}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(requireAllowedRT)

	h := notification.NewHandler(svc, nil, nil, "development")

	r.Get("/api/v1/notifications/detail/{id}", h.GetNotificationDetail)
	r.Get("/api/v1/notifications/logs", h.ListLogs)
	r.Get("/api/v1/notifications/unread-count", h.GetUnreadCount)
	r.Post("/api/v1/notifications/logs/{id}/read", h.MarkAsRead)
	r.Patch("/api/v1/notifications/logs/{id}/read", h.MarkAsRead)
	r.Post("/api/v1/notifications/read/{id}", h.MarkAsRead)
	r.Patch("/api/v1/notifications/read/{id}", h.MarkAsRead)
	r.Post("/api/v1/notifications/logs/read-all", h.MarkAllAsRead)
	r.Patch("/api/v1/notifications/logs/read-all", h.MarkAllAsRead)
	r.Post("/api/v1/notifications/read-all", h.MarkAllAsRead)
	r.Post("/api/v1/notifications/devices", h.RegisterDevice)
	r.Delete("/api/v1/notifications/devices", h.UnregisterDevice)

	return r
}

func withAuthNotification(req *http.Request, ac *auth.AuthContext) *http.Request {
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	// Send RT code as header so handler.resolveTenantRT can short-circuit
	// without needing database round-trips (maps UUID-style RTIDs to codes).
	if ac.RTID != "" {
		req.Header.Set("X-RT-ID", ac.RTID)
	}
	return req.WithContext(ctx)
}

// ============================================================================
// Device isolation tests
// ============================================================================

func TestHandler_Device_RegisterTiesToAuthenticatedUser(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	req := httptest.NewRequest("POST", "/api/v1/notifications/devices", strings.NewReader(`{"fcm_token":"tok-user-a","platform":"android"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for device registration, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify device is tied to user-a in the repo
	tokens, err := repo.GetTokensForUser(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("failed to get tokens for user-a: %v", err)
	}
	found := false
	for _, tok := range tokens {
		if tok == "tok-user-a" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected device token tok-user-a to be registered for user-a")
	}

	// Verify user-b does NOT have the token
	tokensB, err := repo.GetTokensForUser(context.Background(), "user-b")
	if err != nil {
		t.Fatalf("failed to get tokens for user-b: %v", err)
	}
	if len(tokensB) != 0 {
		t.Errorf("expected user-b to have 0 tokens, got %v", tokensB)
	}
}

func TestHandler_Device_UnregisterOwnDevice(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	// Register device first
	_ = repo.RegisterDevice(context.Background(), "user-a", "tok-user-a", "android", nil)

	// Unregister own device
	req := httptest.NewRequest("DELETE", "/api/v1/notifications/devices", strings.NewReader(`{"fcm_token":"tok-user-a"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for unregister own device, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify device is deactivated
	tokens, err := repo.GetTokensForUser(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("failed to get tokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 active tokens after unregister, got %v", tokens)
	}
}

func TestHandler_Device_CannotUnregisterOtherUserDevice(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	// Register device for user-b
	_ = repo.RegisterDevice(context.Background(), "user-b", "tok-user-b", "android", nil)

	// User A tries to unregister user-b's device
	req := httptest.NewRequest("DELETE", "/api/v1/notifications/devices", strings.NewReader(`{"fcm_token":"tok-user-b"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// The handler uses ac.UserID (user-a) to look up the token.
	// tok-user-b doesn't exist under user-a, so it's a no-op returning 204.
	// The important thing is user-b's device is still active.
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for no-op unregister, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify device-b is still active
	tokens, err := repo.GetTokensForUser(context.Background(), "user-b")
	if err != nil {
		t.Fatalf("failed to get tokens: %v", err)
	}
	found := false
	for _, tok := range tokens {
		if tok == "tok-user-b" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected device tok-user-b to still be active for user-b")
	}
}

// ============================================================================
// Notification detail isolation tests
// ============================================================================

func TestHandler_NotificationDetail_OwnNotification(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	// Add a personal notification for user-a
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-a"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b1:user-a",
		Title:     "Tagihan Baru",
		Body:      "Tagihan bulan ini telah terbit.",
		Status:    "SENT",
	})

	req := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-1", nil)
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for own notification, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_NotificationDetail_CannotReadOtherUser(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	// Add a personal notification for user-b
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-b"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b2:user-b",
		Title:     "Tagihan User B",
		Body:      "Tagihan user B bulan ini.",
		Status:    "SENT",
	})

	// User A tries to read user-b's notification
	req := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-1", nil)
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for user-a reading user-b's notification, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_NotificationDetail_CannotReadOtherRT(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	// Add notification in rt-003
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-003",
		UserID:    strPtr("user-b"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b3:user-b",
		Title:     "Tagihan RT 003",
		Body:      "Tagihan di RT 003.",
		Status:    "SENT",
	})

	// User A (rt-002) tries to read notification in rt-003
	req := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-1", nil)
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Should be 404 (not found) since notification doesn't exist in rt-002
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("expected 404 or 403 for cross-RT access, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_NotificationDetail_OfficerCanReadInSameRT(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	ketua := &auth.AuthContext{
		UserID:     "user-ketua",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
	}

	// Add notification for user-a in rt-002
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-a"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b4:user-a",
		Title:     "Tagihan User A",
		Body:      "Tagihan user A.",
		Status:    "SENT",
	})

	// Ketua can read user-a's notification in same RT
	req := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-1", nil)
	req = withAuthNotification(req, ketua)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for ketua reading same-RT notification, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_NotificationDetail_OfficerCannotReadOtherRT(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	ketuaRT002 := &auth.AuthContext{
		UserID:     "user-ketua-rt002",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
	}

	// Add notification in rt-003
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-003",
		UserID:    strPtr("user-b"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b5:user-b",
		Title:     "Tagihan RT 003",
		Body:      "Tagihan di RT 003.",
		Status:    "SENT",
	})

	// Ketua rt-002 tries to read notification in rt-003
	req := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-1", nil)
	req = withAuthNotification(req, ketuaRT002)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Should be 404 since notification doesn't exist in rt-002 context
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("expected 404 or 403 for cross-RT officer access, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ============================================================================
// Read operations isolation tests
// ============================================================================

func TestHandler_MarkAsRead_CannotMarkOtherUserNotification(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	// Add unread notification for user-a
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-a"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b6:user-a",
		Title:     "Tagihan Belum Dibaca",
		Body:      "Tagihan belum dibaca.",
		Status:    "SENT",
	})

	// Another user (user-c) tries to mark user-a's notification as read
	userC := &auth.AuthContext{
		UserID: "user-c",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	req := httptest.NewRequest("POST", "/api/v1/notifications/logs/log-1/read", nil)
	req = withAuthNotification(req, userC)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for user-c marking user-a's notification as read, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify notification is still unread (status unchanged)
	entry, err := repo.GetLogByID(context.Background(), "rt-002", "log-1")
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}
	if entry == nil {
		t.Fatal("expected log-1 to exist")
	}
	if entry.Status == "READ" {
		t.Error("expected notification to remain unread after rejected mark-as-read")
	}
}

func TestHandler_MarkAllAsRead_CannotMarkOtherUserNotifications(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	// Add unread notifications for user-a
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-a"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b7:user-a-1",
		Title:     "Tagihan 1",
		Body:      "Tagihan 1.",
		Status:    "SENT",
	})
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-a"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b7:user-a-2",
		Title:     "Tagihan 2",
		Body:      "Tagihan 2.",
		Status:    "SENT",
	})

	// User C tries to read-all for user-a
	userC := &auth.AuthContext{
		UserID: "user-c",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}
	req := httptest.NewRequest("POST", "/api/v1/notifications/logs/read-all", nil)
	req = withAuthNotification(req, userC)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for read-all, got %d: %s", rec.Code, rec.Body.String())
	}

	// User-c's read-all affects user-c's notifications (none), count is 0
	var resp struct {
		UpdatedCount int `json:"updated_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.UpdatedCount != 0 {
		t.Errorf("expected updated_count 0 for user-c's read-all, got %d", resp.UpdatedCount)
	}

	// Verify both notifications are still unread
	for _, l := range []string{"log-1", "log-2"} {
		entry, err := repo.GetLogByID(context.Background(), "rt-002", l)
		if err != nil {
			t.Fatalf("failed to get log %s: %v", l, err)
		}
		if entry == nil {
			t.Fatalf("expected log %s to exist", l)
		}
		if entry.Status == "READ" {
			t.Errorf("expected notification %s to remain unread", l)
		}
	}
}

func TestHandler_UnreadCount_ExcludesOtherUser(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	// Add unread notifications for user-a and user-b in same RT
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-a"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b8:user-a",
		Title:     "Tagihan A",
		Body:      "Tagihan A.",
		Status:    "SENT",
	})
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-b"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b8:user-b",
		Title:     "Tagihan B",
		Body:      "Tagihan B.",
		Status:    "SENT",
	})

	// User A's unread count should be 1 (only their own)
	req := httptest.NewRequest("GET", "/api/v1/notifications/unread-count", nil)
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unread count, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UnreadCount int `json:"unread_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.UnreadCount != 1 {
		t.Errorf("expected unread count 1 for user-a, got %d", resp.UnreadCount)
	}
}

func TestHandler_ListLogs_ExcludesOtherUser(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	userA := &auth.AuthContext{
		UserID: "user-a",
		RTID:   "rt-002",
		TenantRole: auth.RoleWarga,
	}

	// Add notifications for user-a and user-b in same RT
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-a"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b9:user-a",
		Title:     "Tagihan A",
		Body:      "Tagihan A.",
		Status:    "SENT",
	})
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    strPtr("user-b"),
		EventType: notification.EventBillCreated,
		DedupKey:  "bill:b9:user-b",
		Title:     "Tagihan B",
		Body:      "Tagihan B.",
		Status:    "SENT",
	})

	// User A lists logs - should only see their own
	req := httptest.NewRequest("GET", "/api/v1/notifications/logs", nil)
	req = withAuthNotification(req, userA)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for list logs, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []notification.NotificationLogEntry `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Pagination.Total != 1 {
		t.Errorf("expected total 1 log for user-a, got %d", resp.Pagination.Total)
	}
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 log in data for user-a, got %d", len(resp.Data))
	}
	if resp.Data[0].UserID == nil || *resp.Data[0].UserID != "user-a" {
		t.Errorf("expected log owned by user-a, got %v", resp.Data[0].UserID)
	}
}

// ============================================================================
// Broadcast notification access tests
// ============================================================================

func TestHandler_BroadcastNotification_OfficerCanReadInSameRT(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	ketua := &auth.AuthContext{
		UserID:     "user-ketua",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
	}

	// Add broadcast notification in rt-002
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-002",
		UserID:    nil, // broadcast - no specific user
		EventType: notification.EventManualPush,
		DedupKey:  "manual_push:rt-002:1",
		Title:     "Kerja Bakti",
		Body:      "Seluruh warga RT 002 hadir.",
		Status:    "SENT",
	})

	// Ketua can read broadcast notification in same RT
	req := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-1", nil)
	req = withAuthNotification(req, ketua)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for ketua reading broadcast notification, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_BroadcastNotification_OfficerCannotReadOtherRT(t *testing.T) {
	repo := newDeviceAwareRepo()
	svc := notification.NewService(repo, nil)
	router := setupNotificationRouter(repo, svc)

	ketuaRT002 := &auth.AuthContext{
		UserID:     "user-ketua-rt002",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
	}

	// Add broadcast notification in rt-003
	repo.AddLog(notification.NotificationLogEntry{
		RTID:      "rt-003",
		UserID:    nil,
		EventType: notification.EventManualPush,
		DedupKey:  "manual_push:rt-003:1",
		Title:     "Kerja Bakti RT 003",
		Body:      "Seluruh warga RT 003 hadir.",
		Status:    "SENT",
	})

	// Ketua rt-002 tries to read broadcast in rt-003
	req := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-1", nil)
	req = withAuthNotification(req, ketuaRT002)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Should be 404 since notification doesn't exist in rt-002 context
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("expected 404 or 403 for officer cross-RT broadcast access, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Helper functions

func strPtr(s string) *string {
	return &s
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}
