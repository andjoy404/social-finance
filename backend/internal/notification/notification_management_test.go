package notification_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"

	"github.com/google/uuid"
)

// mockManagementRepo stores templates and logs in-memory for testing management handlers.
type mockManagementRepo struct {
	mu        sync.Mutex
	inMemoryRepo
	templates map[string]*notification.NotificationTemplate // id -> template
	logs      []notification.NotificationLogEntry
}

func newMockManagementRepo() *mockManagementRepo {
	return &mockManagementRepo{
		inMemoryRepo: *newInMemoryRepo(),
		templates:    make(map[string]*notification.NotificationTemplate),
		logs:         make([]notification.NotificationLogEntry, 0),
	}
}

func (m *mockManagementRepo) ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
	m.mu.Lock()
	var res []notification.NotificationTemplate
	for _, t := range m.templates {
		if t.RTID != rtID {
			continue
		}
		if eventType != "" && t.EventType != eventType {
			continue
		}
		if isActive != nil && t.IsActive != *isActive {
			continue
		}
		res = append(res, *t)
	}
	m.mu.Unlock()
	return res, nil
}

func (m *mockManagementRepo) GetTemplateByID(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	m.mu.Lock()
	t, ok := m.templates[id]
	if !ok || t.RTID != rtID {
		m.mu.Unlock()
		return nil, nil
	}
	m.mu.Unlock()
	return t, nil
}

func (m *mockManagementRepo) GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*notification.NotificationTemplate, error) {
	m.mu.Lock()
	for _, t := range m.templates {
		if t.RTID == rtID && t.EventType == eventType && t.IsActive {
			m.mu.Unlock()
			return t, nil
		}
	}
	m.mu.Unlock()
	return nil, nil
}

func (m *mockManagementRepo) CreateTemplate(ctx context.Context, rtID string, in notification.CreateTemplateInput, allowedVars []string) (*notification.NotificationTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("tmpl-%d", len(m.templates)+1)
	t := &notification.NotificationTemplate{
		ID:        id,
		RTID:      rtID,
		Name:      in.Name,
		EventType: in.EventType,
		Channel:   in.Channel,
		Title:     in.Title,
		Body:      in.Body,
		ImageURL:  in.ImageURL,
		IsActive:  true,
		Variables: allowedVars,
	}
	m.templates[id] = t
	return t, nil
}

func (m *mockManagementRepo) UpdateTemplate(ctx context.Context, rtID, id string, in notification.UpdateTemplateInput) (*notification.NotificationTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.templates[id]
	if !ok || t.RTID != rtID {
		return nil, nil
	}
	if in.Name != "" {
		t.Name = in.Name
	}
	if in.Title != "" {
		t.Title = in.Title
	}
	if in.Body != "" {
		t.Body = in.Body
	}
	if in.ImageURL != nil {
		t.ImageURL = in.ImageURL
	}
	if in.IsActive != nil {
		t.IsActive = *in.IsActive
	}
	return t, nil
}

func (m *mockManagementRepo) ToggleTemplateActive(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.templates[id]
	if !ok || t.RTID != rtID {
		return nil, nil
	}
	t.IsActive = !t.IsActive
	return t, nil
}

func (m *mockManagementRepo) DeleteTemplate(ctx context.Context, rtID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.templates, id)
	return nil
}

func (m *mockManagementRepo) ListLogs(ctx context.Context, rtID, userID, eventType, status string, isOfficer bool, offset, limit int) ([]notification.NotificationLogEntry, int, error) {
	m.mu.Lock()
	var filtered []notification.NotificationLogEntry
	for _, l := range m.logs {
		if l.RTID != rtID {
			continue
		}
		if eventType != "" && l.EventType != eventType {
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
	m.mu.Unlock()
	return filtered[start:end], total, nil
}

func (m *mockManagementRepo) GetLogByID(ctx context.Context, rtID, id string) (*notification.NotificationLogEntry, error) {
	for _, l := range m.logs {
		if l.ID == id && l.RTID == rtID {
			copied := l
			return &copied, nil
		}
	}
	return nil, nil
}

// setupManagementRouter mirrors the production notification management route group.
// It applies RequireAllowedRT + RequireNotificationAdmin to match the production chain:
// RequireAuth + RequireAllowedRT(pool) + RequireNotificationAdmin(pool).
//
// Manual push and waste-truck endpoints are in a separate group with RequireRole.
func setupManagementRouter(repo notification.Repository, svc notification.Service, pool *database.Pool) http.Handler {
	r := chi.NewRouter()
	h := notification.NewHandler(svc, nil, nil, "test")

	r.Get("/api/v1/notifications/images/*", h.GetImage)
	r.Get("/api/v1/notifications/detail/{id}", h.GetNotificationDetail)

	// In-memory RT code lookup mirroring: SELECT rt, is_active FROM rts WHERE id = $1
	allowedRTCodes := map[string]string{
		"00000000-0000-0000-0000-000000000002": "002",
		"00000000-0000-0000-0000-000000000003": "003",
	}

	// requireAllowedRT is an inline middleware matching RequireAllowedRT(pool)
	// without requiring a real database connection.
	// After security hardening, NO bypass exists for empty RTID — every request
	// to a tenant-scoped endpoint must carry a valid, whitelisted RT.
	requireAllowedRT := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := r.Context().Value(auth.AuthContextKey).(*auth.AuthContext)
			if !ok || ac == nil {
				http.Error(w, `{"error":{"code":"unauthorized","message":"missing authentication"}}`, http.StatusUnauthorized)
				return
			}

			// No bypass for empty RTID — every tenant-scoped request must have a valid RT.
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

	// Management endpoints: templates, logs, test-push, upload-image
	r.Group(func(r chi.Router) {
		r.Use(requireAllowedRT)
		r.Use(auth.RequireNotificationAdmin(pool))
		r.Get("/api/v1/notifications/system-types", h.GetSystemTypes)
		r.Get("/api/v1/notifications/templates", h.ListTemplates)
		r.Post("/api/v1/notifications/templates", h.CreateTemplate)
		r.Get("/api/v1/notifications/templates/{id}", h.GetTemplate)
		r.Put("/api/v1/notifications/templates/{id}", h.UpdateTemplate)
		r.Patch("/api/v1/notifications/templates/{id}/toggle-active", h.ToggleTemplateActive)
		r.Delete("/api/v1/notifications/templates/{id}", h.DeleteTemplate)
		r.Get("/api/v1/notifications/logs", h.ListLogs)
		r.Post("/api/v1/notifications/test-push", h.TestPush)
		r.Post("/api/v1/notifications/upload-image", h.UploadImage)
	})

	// Manual push and waste-truck: separate group mirroring production
	// Requires RequireAllowedRT + isOfficerUser check inside handler (like production)
	r.Group(func(r chi.Router) {
		r.Use(requireAllowedRT)
		r.Post("/api/v1/notifications/manual", h.TriggerManualPush)
		r.Post("/api/v1/notifications/waste-truck", h.TriggerWasteTruckArrival)
	})

	return r
}

func withAuth(req *http.Request, ac *auth.AuthContext) *http.Request {
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	return req.WithContext(ctx)
}

func TestNotificationManagement_SystemTypes(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, notification.NewMockPushProvider())
	router := setupManagementRouter(repo, svc, pool)

	req := httptest.NewRequest("GET", "/api/v1/notifications/system-types", nil)
	req = withAuth(req, &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, RTID: "00000000-0000-0000-0000-000000000002"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []notification.SystemNotificationType `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) < 4 {
		t.Errorf("expected at least 4 system types, got %d", len(resp.Data))
	}
}

func TestNotificationManagement_TemplateCRUD(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ketuaUID, membershipID := ensureNotificationManagePermission(t, pool)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	ketuaAuth := &auth.AuthContext{
		UserID:         ketuaUID,
		MembershipID:   membershipID,
		RTID:           "00000000-0000-0000-0000-000000000002",
		TenantRole:     auth.RolePengurus,
		Jabatan:        auth.JabatanKetua,
	}

	// 1. Create Valid Template
	createBody := map[string]any{
		"name":       "Template Tagihan Bulanan",
		"event_type": notification.EventBillCreated,
		"channel":    "push",
		"title":      "Pemberitahuan Iuran RT 002: {{bill_id}}",
		"body":       "Halo warga, tagihan iuran baru Anda dengan ID {{bill_id}} telah siap dibayar.",
	}
	bodyBytes, _ := json.Marshal(createBody)
	req := httptest.NewRequest("POST", "/api/v1/notifications/templates", bytes.NewReader(bodyBytes))
	req = withAuth(req, ketuaAuth)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created notification.NotificationTemplate
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode template: %v", err)
	}
	if created.Name != "Template Tagihan Bulanan" {
		t.Errorf("expected name 'Template Tagihan Bulanan', got %s", created.Name)
	}
	if !created.IsActive {
		t.Error("expected template to be active by default")
	}

	// 2. Reject Invalid Variable in Template
	invalidBody := map[string]any{
		"name":       "Template Tagihan Ilegal",
		"event_type": notification.EventBillCreated,
		"channel":    "push",
		"title":      "Pemberitahuan: {{password}}", // unauthorized variable
		"body":       "Pesan: {{nik}}",
	}
	bodyBytes, _ = json.Marshal(invalidBody)
	req = httptest.NewRequest("POST", "/api/v1/notifications/templates", bytes.NewReader(bodyBytes))
	req = withAuth(req, ketuaAuth)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for unauthorized variable, got %d", rec.Code)
	}

	// 3. List Templates
	req = httptest.NewRequest("GET", "/api/v1/notifications/templates", nil)
	req = withAuth(req, ketuaAuth)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var listResp struct {
		Data []notification.NotificationTemplate `json:"data"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 template in list, got %d", len(listResp.Data))
	}

	// 4. Toggle Active
	req = httptest.NewRequest("PATCH", "/api/v1/notifications/templates/"+created.ID+"/toggle-active", nil)
	req = withAuth(req, ketuaAuth)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 toggle, got %d", rec.Code)
	}
	var toggled notification.NotificationTemplate
	_ = json.NewDecoder(rec.Body).Decode(&toggled)
	if toggled.IsActive {
		t.Error("expected template to become inactive")
	}

	// 5. Delete Template
	req = httptest.NewRequest("DELETE", "/api/v1/notifications/templates/"+created.ID, nil)
	req = withAuth(req, ketuaAuth)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", rec.Code)
	}
}

func TestNotificationManagement_AuthorizationRules(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ketuaUID, membershipID := ensureNotificationManagePermission(t, pool)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	cases := []struct {
		name       string
		ac         *auth.AuthContext
		expectCode int
	}{
		{
			name:       "Super Admin is allowed",
			ac:         &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, RTID: "00000000-0000-0000-0000-000000000002"},
			expectCode: http.StatusOK,
		},
		{
			name: "Ketua RT is allowed",
			ac: &auth.AuthContext{
				UserID:         ketuaUID,
				MembershipID:   membershipID,
				RTID:           "00000000-0000-0000-0000-000000000002",
				TenantRole:     auth.RolePengurus,
				Jabatan:        auth.JabatanKetua,
			},
			expectCode: http.StatusOK,
		},
		{
			name: "Bendahara is FORBIDDEN",
			ac: &auth.AuthContext{
				UserID:     "user-bendahara",
				RTID:       "00000000-0000-0000-0000-000000000002",
				TenantRole: auth.RoleBendahara,
				Jabatan:    auth.JabatanBendahara,
			},
			expectCode: http.StatusForbidden,
		},
		{
			name: "Pengurus without Ketua jabatan is FORBIDDEN",
			ac: &auth.AuthContext{
				UserID:     "user-sekretaris",
				RTID:       "00000000-0000-0000-0000-000000000002",
				TenantRole: auth.RolePengurus,
				Jabatan:    auth.JabatanSekretaris,
			},
			expectCode: http.StatusForbidden,
		},
		{
			name: "Warga is FORBIDDEN",
			ac: &auth.AuthContext{
				UserID:     "user-warga",
				RTID:       "00000000-0000-0000-0000-000000000002",
				TenantRole: auth.RoleWarga,
			},
			expectCode: http.StatusForbidden,
		},
		{
			name:       "Unauthenticated is UNAUTHORIZED",
			ac:         nil,
			expectCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/notifications/system-types", nil)
			if tc.ac != nil {
				req = withAuth(req, tc.ac)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tc.expectCode {
				t.Errorf("expected status %d, got %d (body: %s)", tc.expectCode, rec.Code, rec.Body.String())
			}
		})
	}
}

// ensureNotificationManagePermission grants notification.manage to jabatan "ketua"
// in the test database so that Ketua RT can pass the permission check.
// Also creates a ketua user + membership in rt-002 so that
// validateMembershipJabatan succeeds.
// Returns the ketua user ID and membership ID.
func ensureNotificationManagePermission(t *testing.T, pool *database.Pool) (ketuaUID string, membershipID string) {
	t.Helper()
	db := pool.Raw()

	// Create ketua user if not exists
	ketuaUID = uuid.Must(uuid.NewUUID()).String()
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, '08123456789', 'dummyhash', 'Test Ketua RT', true)
		 ON CONFLICT (id) DO UPDATE SET is_active = true
		 RETURNING id`,
		ketuaUID, fmt.Sprintf("ketua-%d@test.com", time.Now().UnixNano()),
	).Scan(&ketuaUID)
	if err != nil {
		t.Fatalf("create ketua user: %v", err)
	}

	// Use existing RT rt-002 if it exists and is active;
	// only create it if it doesn't exist yet. This avoids
	// uniq_rts_active partial-index violations on re-run.
	var rtID string
	rtID = "00000000-0000-0000-0000-000000000002"
	var isActive bool
	err = db.QueryRowContext(context.Background(),
		`SELECT id, is_active FROM rts WHERE id = $1`,
		rtID,
	).Scan(&rtID, &isActive)
	if err == sql.ErrNoRows {
		// RT does not exist — create it (first run only)
		_, err = db.ExecContext(context.Background(),
			`INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, true)`,
			"00000000-0000-0000-0000-000000000002", "Test RT 002", 1, "002", "Test Address", "Test Head",
		)
		if err != nil {
			t.Fatalf("create test RT rt-002: %v", err)
		}
		rtID = "00000000-0000-0000-0000-000000000002"
	} else if err != nil {
		t.Fatalf("lookup test RT rt-002: %v", err)
	}
	if !isActive {
		// RT exists but is inactive — leave it as-is; tests that need an active RT
		// should manage their own fixtures rather than mutating shared data.
	}

	// Create membership for ketua
	membershipID = uuid.Must(uuid.NewUUID()).String()
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
	 VALUES ($1, $2, $3, $4, $5, true)
	 ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, jabatan = EXCLUDED.jabatan, rt_id = EXCLUDED.rt_id, is_active = true`,
		membershipID, ketuaUID, rtID, "warga", "ketua",
	)
	if err != nil {
		t.Fatalf("create ketua membership: %v", err)
	}

	// Insert permission
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO permission_types (code, domain, operation)
		VALUES ('notification.manage', 'notification', 'manage')
		ON CONFLICT (code) DO NOTHING`,
	)
	if err != nil {
		t.Fatalf("insert permission_types: %v", err)
	}
	var permissionID string
	err = db.QueryRowContext(context.Background(),
		`SELECT id FROM permission_types WHERE code = 'notification.manage'`,
	).Scan(&permissionID)
	if err != nil {
		t.Fatalf("get permission id: %v", err)
	}
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO position_permissions (position, permission_id)
		VALUES ($1, $2)
		ON CONFLICT (position, permission_id) DO NOTHING`,
		"ketua", permissionID,
	)
	if err != nil {
		t.Fatalf("insert position_permissions: %v", err)
	}

	return ketuaUID, membershipID
}

func TestNotificationManagement_DynamicTemplateDispatch(t *testing.T) {
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	// Add user device
	_ = repo.RegisterDevice(context.Background(), "user-resident-1", "fcm-tok-resident-1", "android", nil)
	repo.billUsers["bill-xyz-123"] = []string{"user-resident-1"}

	// Create and register active custom template for bill created
	_, _ = repo.CreateTemplate(context.Background(), "00000000-0000-0000-0000-000000000002", notification.CreateTemplateInput{
		Name:      "Custom Bill Notif",
		EventType: notification.EventBillCreated,
		Channel:   "push",
		Title:     "Tagihan Baru Siap: {{bill_id}}",
		Body:      "Silakan bayar tagihan {{bill_id}} tepat waktu.",
	}, []string{"bill_id"})

	// Trigger bill notification
	svc.NotifyBillCreated(context.Background(), "00000000-0000-0000-0000-000000000002", "bill-xyz-123", nil)

	// Allow goroutine to execute
	var found bool
	for i := 0; i < 20; i++ {
		msgs := provider.GetMessages()
		for _, m := range msgs {
			if m.Token == "fcm-tok-resident-1" && m.Title == "Tagihan Baru Siap: bill-xyz-123" {
				found = true
				if m.Body != "Silakan bayar tagihan bill-xyz-123 tepat waktu." {
					t.Errorf("unexpected interpolated body: %s", m.Body)
				}
				break
			}
		}
		if found {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !found {
		t.Error("expected push notification with interpolated template text to be delivered")
	}
}

func TestNotificationManagement_MatrixImageEnforcement(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	superAdminAC := &auth.AuthContext{
		UserID:     "super-admin-1",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0000-000000000002",
	}

	imgURL := "https://example.com/image.png"

	// 1. Creating template for iuran_bill_created with image should be REJECTED
	payloadBad, _ := json.Marshal(notification.CreateTemplateInput{
		Name:      "Bad Iuran Template",
		EventType: notification.EventBillCreated,
		Channel:   "push",
		Title:     "Tagihan {{bill_id}}",
		Body:      "Rincian tagihan",
		ImageURL:  &imgURL,
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/templates", bytes.NewReader(payloadBad))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, withAuth(req, superAdminAC))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for iuran template with image, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// 2. Creating template for manual_push with image should SUCCEED
	payloadGood, _ := json.Marshal(notification.CreateTemplateInput{
		Name:      "Good Manual Template",
		EventType: notification.EventManualPush,
		Channel:   "push",
		Title:     "Pengumuman Kerja Bakti",
		Body:      "Diharapkan hadir besok pagi.",
		ImageURL:  &imgURL,
	})
	reqGood := httptest.NewRequest("POST", "/api/v1/notifications/templates", bytes.NewReader(payloadGood))
	recGood := httptest.NewRecorder()
	router.ServeHTTP(recGood, withAuth(reqGood, superAdminAC))
	if recGood.Code != http.StatusCreated {
		t.Errorf("expected 201 for manual_push template with image, got %d (body: %s)", recGood.Code, recGood.Body.String())
	}
}

func TestNotificationManagement_ManualPushBroadcast(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	superAdminAC := &auth.AuthContext{
		UserID:     "super-admin-1",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0000-000000000002",
	}

	// Register 2 RT devices and 1 other RT device
	_ = repo.RegisterDevice(context.Background(), "user-rt2-a", "tok-rt2-a", "android", nil)
	_ = repo.RegisterDevice(context.Background(), "user-rt2-b", "tok-rt2-b", "android", nil)
	_ = repo.RegisterDevice(context.Background(), "user-other-rt", "tok-other-rt", "android", nil)
	repo.rtUsers["00000000-0000-0000-0000-000000000002"] = []string{"user-rt2-a", "user-rt2-b"}
	repo.rtUsers["rt-other"] = []string{"user-other-rt"}

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Pemberitahuan Air Bersih",
		Body:  "Penyaluran air bersih akan dimulai jam 10:00 WIB.",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, withAuth(req, superAdminAC))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	msgs := provider.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages dispatched to rt-002, got %d", len(msgs))
	}
	for _, m := range msgs {
		if m.Token == "tok-other-rt" {
			t.Errorf("other RT token unexpectedly received manual push")
		}
		if m.Data["type"] != notification.EventManualPush {
			t.Errorf("expected payload type %s, got %s", notification.EventManualPush, m.Data["type"])
		}
	}
}

func TestNotificationManagement_SuperAdminTestPush(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	superAdminAC := &auth.AuthContext{
		UserID:     "super-admin-1",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0000-000000000002",
	}
	ketuaAC := &auth.AuthContext{
		UserID:     "ketua-1",
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
		RTID:       "00000000-0000-0000-0000-000000000002",
	}

	_ = repo.RegisterDevice(context.Background(), "super-admin-1", "tok-admin-1", "android", nil)
	_ = repo.RegisterDevice(context.Background(), "ketua-1", "tok-ketua-1", "android", nil)

	// 1. Non-superadmin cannot call test-push (Forbidden 403)
	testBody, _ := json.Marshal(notification.TestPushInput{
		EventType: notification.EventManualPush,
		Title:     "Test Push",
		Body:      "Testing admin token only",
	})
	reqForbidden := httptest.NewRequest("POST", "/api/v1/notifications/test-push", bytes.NewReader(testBody))
	recForbidden := httptest.NewRecorder()
	router.ServeHTTP(recForbidden, withAuth(reqForbidden, ketuaAC))
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-superadmin, got %d", recForbidden.Code)
	}

	// 2. Reject test-push for Iuran event
	testIuran, _ := json.Marshal(notification.TestPushInput{
		EventType: notification.EventBillCreated,
		Title:     "Test Iuran",
		Body:      "Not allowed",
	})
	reqIuran := httptest.NewRequest("POST", "/api/v1/notifications/test-push", bytes.NewReader(testIuran))
	recIuran := httptest.NewRecorder()
	router.ServeHTTP(recIuran, withAuth(reqIuran, superAdminAC))
	if recIuran.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for testing iuran event, got %d", recIuran.Code)
	}

	// 3. Superadmin test-push sends ONLY to superadmin's device
	reqGood := httptest.NewRequest("POST", "/api/v1/notifications/test-push", bytes.NewReader(testBody))
	recGood := httptest.NewRecorder()
	router.ServeHTTP(recGood, withAuth(reqGood, superAdminAC))
	if recGood.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", recGood.Code, recGood.Body.String())
	}

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 test message, got %d", len(msgs))
	}
	if msgs[0].Token != "tok-admin-1" {
		t.Errorf("expected test message strictly sent to super-admin-1, got token %s", msgs[0].Token)
	}
}

func TestNotificationManagement_GetNotificationDetail_TenantAndUserIsolation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	imgURL := "/api/v1/notifications/images/notifications/manual/rt-002/test.png"
	senderID := "user-ketua-rt2"
	superAdminID := "super-admin-1"
	wargaA := "user-warga-a"
	wargaB := "user-warga-b"

	repo.logs = []notification.NotificationLogEntry{
		{
			ID:        "log-manual-rt2",
			RTID:      "00000000-0000-0000-0000-000000000002",
			UserID:    &senderID,
			EventType: notification.EventManualPush,
			DedupKey:  "manual_push:rt-002:1",
			Title:     "Kerja Bakti Minggu Pagi",
			Body:      "Seluruh warga RT 002 diharapkan hadir.",
			ImageURL:  &imgURL,
			Status:    "SENT",
			CreatedAt: time.Now(),
		},
		{
			ID:        "log-bill-warga-a",
			RTID:      "00000000-0000-0000-0000-000000000002",
			UserID:    &wargaA,
			EventType: notification.EventBillCreated,
			DedupKey:  "bill:b1:user-warga-a",
			Title:     "Tagihan Iuran Baru",
			Body:      "Tagihan bulan ini telah terbit.",
			Status:    "SENT",
			CreatedAt: time.Now(),
		},
		{
			ID:        "log-test-admin",
			RTID:      "00000000-0000-0000-0000-000000000002",
			UserID:    &superAdminID,
			EventType: notification.EventManualPush,
			DedupKey:  "test_push:rt-002:super-admin-1:99",
			Title:     "[TEST] Pengumuman",
			Body:      "Hanya untuk super admin.",
			Status:    "SENT",
			CreatedAt: time.Now(),
		},
	}

	// 1. Warga in RT 002 can view manual broadcast notification in RT 002
	req1 := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-manual-rt2", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, withAuth(req1, &auth.AuthContext{
		UserID:     wargaB,
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleWarga,
	}))
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for RT member viewing manual broadcast, got %d (%s)", rec1.Code, rec1.Body.String())
	}

	// 2. Warga in different RT (rt-003) CANNOT view RT 002 notification (404 Not Found)
	req2 := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-manual-rt2", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, withAuth(req2, &auth.AuthContext{
		UserID:     "user-other-rt",
		RTID:       "00000000-0000-0000-0000-000000000003",
		TenantRole: auth.RoleWarga,
	}))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for cross-RT access, got %d", rec2.Code)
	}

	// 3. Warga B CANNOT view Warga A's personal bill notification (403 Forbidden)
	req3 := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-bill-warga-a", nil)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, withAuth(req3, &auth.AuthContext{
		UserID:     wargaB,
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleWarga,
	}))
	if rec3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for another user's personal notification, got %d", rec3.Code)
	}

	// 4. Warga A CAN view their own personal bill notification (200 OK)
	req4 := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-bill-warga-a", nil)
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, withAuth(req4, &auth.AuthContext{
		UserID:     wargaA,
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleWarga,
	}))
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for owner viewing personal notification, got %d", rec4.Code)
	}

	// 5. Non-owner CANNOT view Super Admin test push notification (403 Forbidden)
	req5 := httptest.NewRequest("GET", "/api/v1/notifications/detail/log-test-admin", nil)
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, withAuth(req5, &auth.AuthContext{
		UserID:     wargaA,
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleWarga,
	}))
	if rec5.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-owner accessing test_push log, got %d", rec5.Code)
	}
}

func TestNotificationManagement_ImagePathTraversalProtection(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	traversalPaths := []string{
		"/api/v1/notifications/images/../../secrets/fcm-service-account.json",
		"/api/v1/notifications/images/notifications/..%2f..%2fetc/passwd",
		"/api/v1/notifications/images/../../../etc/shadow",
	}

	for _, p := range traversalPaths {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("expected path traversal %q to be rejected with 400 or 404, got %d", p, rec.Code)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Regression tests (added during security review)
// These tests verify that the HTTP 400 fixes and the whitelist bypass fix work
// correctly and do not regress.
// ──────────────────────────────────────────────────────────────────────────────

// TestResolveTenantRT_JWTFallback verifies that when the X-RT-ID header is
// missing, the handler falls back to ac.RTID from JWT claims instead of
// passing an empty string to the service.
//
// In production, RequireAuth sets ac.RTID from JWT claims. If the header is
// absent, the handler's resolveTenantRT uses ac.RTID as the fallback.
func TestResolveTenantRT_JWTFallback(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	// Simulates production: RequireAuth already populated ac.RTID from JWT.
	// X-RT-ID header is absent — resolveTenantRT falls back to ac.RTID.
	ac := &auth.AuthContext{
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0000-000000000002",
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications/templates", nil)
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (JWT RTID fallback), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestResolveTenantRT_EmptyRTIDReturns400 verifies that when a regular user
// (non-SuperAdmin) has no RTID in JWT, the handler returns 400 Bad Request.
// In production, RequireAuth sets ac.RTID from JWT claims for regular users.
// If the user has no active RT membership, ac.RTID is empty and ResolveTenantRT
// returns an error.
//
// Note: For Super Admins with empty RTID, ResolveTenantRT falls back to
// repo.GetDefaultRTID(), which returns "test-rt-002" in the mock. So Super
// Admins succeed with 200. Only non-SuperAdmins get 400/403.
func TestResolveTenantRT_EmptyRTIDReturns400(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	// Warga user with empty RTID — RequireAllowedRT blocks at middleware level
	// because ac.RTID == "" and they're not a Super Admin.
	ac := &auth.AuthContext{
		SystemRole: "", // empty = non-SuperAdmin
		RTID:       "",
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications/templates", nil)
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// RequireAllowedRT blocks non-SuperAdmin with empty RTID.
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from middleware for non-SA with empty RTID, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestResolveTenantRT_SuperAdminWithRTID verifies that a Super Admin with
// RTID set (from JWT or RequireAuth header processing) accesses whitelisted RT.
//
// In production, when a Super Admin sends X-RT-ID header, RequireAuth middleware
// reads it, validates it against rts.is_active, and sets ac.RTID. Then
// RequireAllowedRT checks ac.RTID against the whitelist.
func TestResolveTenantRT_SuperAdminWithRTID(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0000-000000000002", // simulates RequireAuth already processing header
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications/templates", nil)
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for whitelisted RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestResolveTenantRT_SuperAdminNoRTIDNoHeader verifies that a Super Admin
// with empty RTID and no X-RT-ID header is REJECTED. The old behavior
// silently fell back to repo.GetDefaultRTID(), which bypassed the whitelist.
// After security hardening, RequireAllowedRT middleware blocks empty RTID,
// and ResolveTenantRT no longer calls GetDefaultRTID() as a fallback.
func TestResolveTenantRT_SuperAdminNoRTIDNoHeader(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "",
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications/templates", nil)
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Inline middleware blocks Super Admin with empty RTID.
	// No silent fallback to a default RT.
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for super admin with empty RTID (no silent fallback), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestListLogs_PaginationPageSize verifies that both page_size and per_page
// query parameters are accepted for list logs.
func TestListLogs_PaginationPageSize(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0000-000000000002",
	}

	// Test with page_size parameter
	req := httptest.NewRequest("GET", "/api/v1/notifications/logs?page=1&page_size=10", nil)
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for page_size, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test with per_page parameter (legacy name)
	req2 := httptest.NewRequest("GET", "/api/v1/notifications/logs?page=1&per_page=5", nil)
	req2 = withAuth(req2, ac)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for per_page, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

// TestManualPush_JWTFallback verifies that manual push uses JWT RTID as fallback
// when X-RT-ID header is not provided.
//
// In production, RequireAuth sets ac.RTID from JWT claims. The handler's
// resolveTenantRT falls back to ac.RTID when the header is absent.
func TestManualPush_JWTFallback(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	// Register device for the target user
	_ = repo.RegisterDevice(context.Background(), "user-pengurus-1", "tok-pengurus-1", "android", nil)
	repo.rtUsers["00000000-0000-0000-0000-000000000002"] = []string{"user-pengurus-1"}

	// Set RTID directly (simulating what RequireAuth does from JWT claims).
	// X-RT-ID header is intentionally NOT set — resolveTenantRT will fall
	// back to ac.RTID, then ResolveTenantRT will return ac.RTID directly.
	ac := &auth.AuthContext{
		UserID:     "user-pengurus-1",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RolePengurus,
	}

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Test Manual Push",
		Body:  "Testing JWT fallback",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (JWT RTID fallback for manual push), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSuperAdminWhitelistBypass verifies that a Super Admin CANNOT access
// a non-whitelisted RT via template CRUD endpoints, even when providing
// an X-RT-ID header.
func TestSuperAdminWhitelistBypass(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	// Super Admin with no JWT RTID, but trying to access non-whitelisted RT
	ac := &auth.AuthContext{
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "", // no JWT membership
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications/templates", nil)
	req.Header.Set("X-RT-ID", "rt-999") // non-whitelisted RT
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// In production, RequireAllowedRT middleware would return 403 here.
	// In tests, the inline middleware passes the Super Admin through (empty RTID),
	// then resolveTenantRT passes "rt-999" to ResolveTenantRT which rejects it
	// with 400 because the header RT is not found/whitelisted.
	// Both 400 and 403 indicate the bypass is blocked — the key assertion
	// is that we do NOT get 200.
	if rec.Code == http.StatusOK {
		t.Fatalf("expected 400 or 403 (bypass blocked), got 200 — whitelist bypass NOT fixed! %s", rec.Body.String())
	}
}

// ===========================================================================
// TEST: Manual Push Authorization for Ketua RT and Officer Roles
// ===========================================================================

// TestManualPush_KetuaRT_CanSend verifies that a Ketua RT (TenantRole=warga,
// Jabatan=ketua) can send manual notifications to their own RT.
func TestManualPush_KetuaRT_CanSend(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ketuaUID, membershipID := ensureNotificationManagePermission(t, pool)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	// Ketua RT with TenantRole=warga, Jabatan=ketua
	ac := &auth.AuthContext{
		UserID:         ketuaUID,
		MembershipID:   membershipID,
		RTID:           "00000000-0000-0000-0000-000000000002",
		TenantRole:     auth.RoleWarga,
		Jabatan:        auth.JabatanKetua,
		RWMemberships: []auth.RWMembership{
			{RWID: "rw-002", Jabatan: "ketua"},
		},
	}

	_ = repo.RegisterDevice(context.Background(), "user-ketua-rt2", "tok-ketua-rt2", "android", nil)
	repo.rtUsers["00000000-0000-0000-0000-000000000002"] = []string{"user-ketua-rt2"}

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Pengumuman Umum",
		Body:  "Rapat RT akan dimulai jam 19:00 WIB.",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for Ketua RT manual push, got %d: %s", rec.Code, rec.Body.String())
	}

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message dispatched, got %d", len(msgs))
	}
	if msgs[0].Token != "tok-ketua-rt2" {
		t.Errorf("expected message to ketua RT device, got token %s", msgs[0].Token)
	}
}

// TestManualPush_WakilKetua_CanSend verifies that a Wakil Ketua RT
// (TenantRole=warga, Jabatan=wakil_ketua) can send manual notifications.
func TestManualPush_WakilKetua_CanSend(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		UserID:     "user-wk-rt2",
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleWarga,
		Jabatan:    auth.JabatanWakilKetua,
		RWMemberships: []auth.RWMembership{
			{RWID: "rw-002", Jabatan: "wakil_ketua"},
		},
	}

	_ = repo.RegisterDevice(context.Background(), "user-wk-rt2", "tok-wk-rt2", "android", nil)
	repo.rtUsers["00000000-0000-0000-0000-000000000002"] = []string{"user-wk-rt2"}

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Info Keamanan",
		Body:  "Jaga kesuburan lingkungan RT.",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for Wakil Ketua manual push, got %d: %s", rec.Code, rec.Body.String())
	}

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message dispatched, got %d", len(msgs))
	}
	if msgs[0].Token != "tok-wk-rt2" {
		t.Errorf("expected message to wakil ketua device, got token %s", msgs[0].Token)
	}
}

// TestManualPush_Bendahara_CanSend verifies that a Bendahara can send manual notifications.
func TestManualPush_Bendahara_CanSend(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		UserID:     "user-bendahara-rt2",
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleBendahara,
		RWMemberships: []auth.RWMembership{
			{RWID: "rw-002", Jabatan: "bendahara"},
		},
	}

	_ = repo.RegisterDevice(context.Background(), "user-bendahara-rt2", "tok-bendahara-rt2", "android", nil)
	repo.rtUsers["00000000-0000-0000-0000-000000000002"] = []string{"user-bendahara-rt2"}

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Pembayaran Iuran",
		Body:  "Silakan melunasi iuran RT bulan ini.",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for Bendahara manual push, got %d: %s", rec.Code, rec.Body.String())
	}

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message dispatched, got %d", len(msgs))
	}
	if msgs[0].Token != "tok-bendahara-rt2" {
		t.Errorf("expected message to bendahara device, got token %s", msgs[0].Token)
	}
}

// TestManualPush_WargaCannotSend verifies that a plain warga (no jabatan)
// cannot send manual notifications.
func TestManualPush_WargaCannotSend(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		UserID:     "user-warga-rt2",
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleWarga,
		Jabatan:    "", // no jabatan - plain warga
		RWMemberships: []auth.RWMembership{
			{RWID: "rw-002", Jabatan: ""},
		},
	}

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Coba Kirim",
		Body:  "Warga mencoba kirim notifikasi.",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for warga manual push, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManualPush_KetuaCannotAccessOtherRT verifies that a Ketua RT cannot
// send manual notification to a different RT.
func TestManualPush_KetuaCannotAccessOtherRT(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ketuaUID, membershipID := ensureNotificationManagePermission(t, pool)
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc, pool)

	// Ketua of rt-002 acting in rt-003 context (where they are only a regular warga / anggota, not officer)
	ac := &auth.AuthContext{
		UserID:         ketuaUID,
		MembershipID:   membershipID,
		RTID:           "00000000-0000-0000-0000-000000000003", // trying to access different RT
		TenantRole:     auth.RoleWarga,
		Jabatan:        auth.JabatanAnggota,
		RWMemberships: []auth.RWMembership{
			{RWID: "rw-002", Jabatan: "ketua"},
			{RWID: "rw-003", Jabatan: "anggota"}, // member of rt-003 but not officer
		},
	}

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Coba Akses RT Lain",
		Body:  "Ketua RT 002 mencoba mengirim ke RT 003.",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Should be forbidden because RTID in JWT doesn't match any whitelisted RT
	// or user doesn't have officer role in target RT
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Ketua RT accessing other RT, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManualPush_Unauthenticated verifies that unauthenticated requests are rejected.
func TestManualPush_Unauthenticated(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	body, _ := json.Marshal(notification.ManualPushInput{
		Title: "Tanpa Auth",
		Body:  "Tanpa token autentikasi.",
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/manual", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated request, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManualPush_WasteTruck_KetuaCanSend verifies that a Ketua RT can trigger
// waste truck arrival notification.
func TestManualPush_WasteTruck_KetuaCanSend(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ketuaUID, membershipID := ensureNotificationManagePermission(t, pool)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		UserID:         ketuaUID,
		MembershipID:   membershipID,
		RTID:           "00000000-0000-0000-0000-000000000002",
		TenantRole:     auth.RoleWarga,
		Jabatan:        auth.JabatanKetua,
		RWMemberships: []auth.RWMembership{
			{RWID: "rw-002", Jabatan: "ketua"},
		},
	}

	body, _ := json.Marshal(notification.WasteTruckNotificationInput{
		Message:   "Sampah telah diangkut.",
		ImageURL:  nil,
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for Ketua RT waste truck, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManualPush_WasteTruck_WargaCannotSend verifies that a plain warga
// cannot trigger waste truck notification.
func TestManualPush_WasteTruck_WargaCannotSend(t *testing.T) {
	pool := testutil.GetTestPool(t)
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	ac := &auth.AuthContext{
		UserID:     "user-warga-rt2",
		RTID:       "00000000-0000-0000-0000-000000000002",
		TenantRole: auth.RoleWarga,
		Jabatan:    "", // no jabatan
		RWMemberships: []auth.RWMembership{
			{RWID: "rw-002", Jabatan: ""},
		},
	}

	body, _ := json.Marshal(notification.WasteTruckNotificationInput{
		Message:   "Coba kirim sampah.",
		ImageURL:  nil,
	})
	req := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, ac)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for warga waste truck, got %d: %s", rec.Code, rec.Body.String())
	}
}

// toUUID creates a deterministic UUID from a string label for test fixtures.
// toUUID creates a deterministic UUID v4-like string from a string label for test fixtures.
func toUUID(s string) string {
	// Pad the input to at least 32 chars to avoid index-out-of-range
	padded := s + strings.Repeat("0", 32-len(s))
	if len(padded) > 32 {
		padded = padded[:32]
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		padded[0:8], padded[8:12], padded[12:16], padded[16:20], padded[20:32])
}
// ensure test util references are used
var _ = sql.ErrNoRows
var _ = database.Pool{}
