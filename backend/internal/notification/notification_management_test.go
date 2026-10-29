package notification_test

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

	"social-finance/internal/auth"
	"social-finance/internal/notification"
)

// mockManagementRepo stores templates and logs in-memory for testing management handlers.
type mockManagementRepo struct {
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
	return res, nil
}

func (m *mockManagementRepo) GetTemplateByID(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	t, ok := m.templates[id]
	if !ok || t.RTID != rtID {
		return nil, nil
	}
	return t, nil
}

func (m *mockManagementRepo) GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*notification.NotificationTemplate, error) {
	for _, t := range m.templates {
		if t.RTID == rtID && t.EventType == eventType && t.IsActive {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockManagementRepo) CreateTemplate(ctx context.Context, rtID string, in notification.CreateTemplateInput, allowedVars []string) (*notification.NotificationTemplate, error) {
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
	t, ok := m.templates[id]
	if !ok || t.RTID != rtID {
		return nil, nil
	}
	t.IsActive = !t.IsActive
	return t, nil
}

func (m *mockManagementRepo) DeleteTemplate(ctx context.Context, rtID, id string) error {
	delete(m.templates, id)
	return nil
}

func (m *mockManagementRepo) ListLogs(ctx context.Context, rtID, eventType string, offset, limit int) ([]notification.NotificationLogEntry, int, error) {
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

func setupManagementRouter(repo notification.Repository, svc notification.Service) http.Handler {
	r := chi.NewRouter()
	h := notification.NewHandler(svc, nil, "test")

	r.Get("/api/v1/notifications/images/*", h.GetImage)
	r.Get("/api/v1/notifications/detail/{id}", h.GetNotificationDetail)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireNotificationAdmin())
		r.Get("/api/v1/notifications/system-types", h.GetSystemTypes)
		r.Get("/api/v1/notifications/templates", h.ListTemplates)
		r.Post("/api/v1/notifications/templates", h.CreateTemplate)
		r.Get("/api/v1/notifications/templates/{id}", h.GetTemplate)
		r.Put("/api/v1/notifications/templates/{id}", h.UpdateTemplate)
		r.Patch("/api/v1/notifications/templates/{id}/toggle-active", h.ToggleTemplateActive)
		r.Delete("/api/v1/notifications/templates/{id}", h.DeleteTemplate)
		r.Get("/api/v1/notifications/logs", h.ListLogs)
		r.Post("/api/v1/notifications/manual", h.TriggerManualPush)
		r.Post("/api/v1/notifications/test-push", h.TestPush)
		r.Post("/api/v1/notifications/upload-image", h.UploadImage)
	})
	return r
}

func withAuth(req *http.Request, ac *auth.AuthContext) *http.Request {
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, ac)
	return req.WithContext(ctx)
}

func TestNotificationManagement_SystemTypes(t *testing.T) {
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc)

	req := httptest.NewRequest("GET", "/api/v1/notifications/system-types", nil)
	req = withAuth(req, &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, RTID: "rt-002"})
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
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc)

	ketuaAuth := &auth.AuthContext{
		UserID:     "user-ketua",
		RTID:       "rt-002",
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
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
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc)

	cases := []struct {
		name       string
		ac         *auth.AuthContext
		expectCode int
	}{
		{
			name:       "Super Admin is allowed",
			ac:         &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, RTID: "rt-002"},
			expectCode: http.StatusOK,
		},
		{
			name: "Ketua RT is allowed",
			ac: &auth.AuthContext{
				UserID:     "user-ketua",
				RTID:       "rt-002",
				TenantRole: auth.RolePengurus,
				Jabatan:    auth.JabatanKetua,
			},
			expectCode: http.StatusOK,
		},
		{
			name: "Bendahara is FORBIDDEN",
			ac: &auth.AuthContext{
				UserID:     "user-bendahara",
				RTID:       "rt-002",
				TenantRole: auth.RoleBendahara,
				Jabatan:    auth.JabatanBendahara,
			},
			expectCode: http.StatusForbidden,
		},
		{
			name: "Pengurus without Ketua jabatan is FORBIDDEN",
			ac: &auth.AuthContext{
				UserID:     "user-sekretaris",
				RTID:       "rt-002",
				TenantRole: auth.RolePengurus,
				Jabatan:    auth.JabatanSekretaris,
			},
			expectCode: http.StatusForbidden,
		},
		{
			name: "Warga is FORBIDDEN",
			ac: &auth.AuthContext{
				UserID:     "user-warga",
				RTID:       "rt-002",
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

func TestNotificationManagement_DynamicTemplateDispatch(t *testing.T) {
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	// Add user device
	_ = repo.RegisterDevice(context.Background(), "user-resident-1", "fcm-tok-resident-1", "android", nil)
	repo.billUsers["bill-xyz-123"] = []string{"user-resident-1"}

	// Create and register active custom template for bill created
	_, _ = repo.CreateTemplate(context.Background(), "rt-002", notification.CreateTemplateInput{
		Name:      "Custom Bill Notif",
		EventType: notification.EventBillCreated,
		Channel:   "push",
		Title:     "Tagihan Baru Siap: {{bill_id}}",
		Body:      "Silakan bayar tagihan {{bill_id}} tepat waktu.",
	}, []string{"bill_id"})

	// Trigger bill notification
	svc.NotifyBillCreated(context.Background(), "rt-002", "bill-xyz-123", nil)

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
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc)

	superAdminAC := &auth.AuthContext{
		UserID:     "super-admin-1",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "rt-002",
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
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc)

	superAdminAC := &auth.AuthContext{
		UserID:     "super-admin-1",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "rt-002",
	}

	// Register 2 RT devices and 1 other RT device
	_ = repo.RegisterDevice(context.Background(), "user-rt2-a", "tok-rt2-a", "android", nil)
	_ = repo.RegisterDevice(context.Background(), "user-rt2-b", "tok-rt2-b", "android", nil)
	_ = repo.RegisterDevice(context.Background(), "user-other-rt", "tok-other-rt", "android", nil)
	repo.rtUsers["rt-002"] = []string{"user-rt2-a", "user-rt2-b"}
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
	repo := newMockManagementRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	router := setupManagementRouter(repo, svc)

	superAdminAC := &auth.AuthContext{
		UserID:     "super-admin-1",
		SystemRole: auth.SystemRoleSuperAdmin,
		RTID:       "rt-002",
	}
	ketuaAC := &auth.AuthContext{
		UserID:     "ketua-1",
		TenantRole: auth.RolePengurus,
		Jabatan:    auth.JabatanKetua,
		RTID:       "rt-002",
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
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc)

	imgURL := "/api/v1/notifications/images/notifications/manual/rt-002/test.png"
	senderID := "user-ketua-rt2"
	superAdminID := "super-admin-1"
	wargaA := "user-warga-a"
	wargaB := "user-warga-b"

	repo.logs = []notification.NotificationLogEntry{
		{
			ID:        "log-manual-rt2",
			RTID:      "rt-002",
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
			RTID:      "rt-002",
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
			RTID:      "rt-002",
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
		RTID:       "rt-002",
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
		RTID:       "rt-003",
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
		RTID:       "rt-002",
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
		RTID:       "rt-002",
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
		RTID:       "rt-002",
		TenantRole: auth.RoleWarga,
	}))
	if rec5.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-owner accessing test_push log, got %d", rec5.Code)
	}
}

func TestNotificationManagement_ImagePathTraversalProtection(t *testing.T) {
	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc)

	traversalPaths := []string{
		"/api/v1/notifications/images/../../secrets/fcm-service-account.json",
		"/api/v1/notifications/images/notifications/..%2f..%2fetc/passwd",
		"/api/v1/notifications/images/notifications\\..\\secret",
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
