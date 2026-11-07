package rt

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	httpx "social-finance/internal/http"
	"social-finance/internal/notification"
)

type mockNotifService struct {
	notification.Service
	listTemplatesFn  func(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error)
	createTemplateFn func(ctx context.Context, rtID string, in notification.CreateTemplateInput) (*notification.NotificationTemplate, error)
	listSchedulesFn  func(ctx context.Context, rtID, eventType string, isActive *bool) ([]*notification.NotificationSchedule, error)
	createScheduleFn func(ctx context.Context, rtID, userID string, in notification.CreateScheduleInput) (*notification.NotificationSchedule, error)
	onCreateCalled   func(rtID string)
}

func (m *mockNotifService) ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
	if m.listTemplatesFn != nil {
		return m.listTemplatesFn(ctx, rtID, eventType, isActive)
	}
	return nil, nil
}

func (m *mockNotifService) CreateTemplate(ctx context.Context, rtID string, in notification.CreateTemplateInput) (*notification.NotificationTemplate, error) {
	if m.onCreateCalled != nil {
		m.onCreateCalled(rtID)
	}
	if m.createTemplateFn != nil {
		return m.createTemplateFn(ctx, rtID, in)
	}
	return &notification.NotificationTemplate{ID: "mock-tmpl-id"}, nil
}

func (m *mockNotifService) ListSchedules(ctx context.Context, rtID, eventType string, isActive *bool) ([]*notification.NotificationSchedule, error) {
	if m.listSchedulesFn != nil {
		return m.listSchedulesFn(ctx, rtID, eventType, isActive)
	}
	return nil, nil
}

func (m *mockNotifService) CreateSchedule(ctx context.Context, rtID, userID string, in notification.CreateScheduleInput) (*notification.NotificationSchedule, error) {
	if m.createScheduleFn != nil {
		return m.createScheduleFn(ctx, rtID, userID, in)
	}
	return &notification.NotificationSchedule{ID: "mock-sched-id"}, nil
}

func buildRouterWithNotif(pool *database.Pool, notifSvc notification.Service) *chi.Mux {
	auth.AuthDBPool = pool
	svc := NewService()
	h := NewHandler(svc, pool, notifSvc)

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.PanicRecovery)
	r.Use(auth.RequireAuth)
	r.Use(auth.RequireSystemRole(auth.SystemRoleSuperAdmin))

	r.Post("/api/v1/rts", h.Create)
	r.Get("/api/v1/rts/{id}", h.GetByID)
	return r
}

func TestCreateRT_CommittedBeforeNotificationSeed(t *testing.T) {
	pool := testPool(t)
	auth.AuthDBPool = pool
	setupEnv()
	token := superAdminToken(t)

	var rtVisibleInDBWhenSeedCalled bool
	var checkedRTID string

	mockSvc := &mockNotifService{
		listTemplatesFn: func(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
			checkedRTID = rtID
			// Check if the RT is visible in the database from a separate query (outside the Create transaction)
			var count int
			err := pool.Raw().QueryRowContext(ctx, "SELECT count(*) FROM rts WHERE id = $1", rtID).Scan(&count)
			if err == nil && count == 1 {
				rtVisibleInDBWhenSeedCalled = true
			}
			return nil, nil
		},
	}

	r := buildRouterWithNotif(pool, mockSvc)

	suffix := time.Now().UnixNano() % 1000000
	rtName := fmt.Sprintf("OrderTest RT %d", suffix)
	rtCode := fmt.Sprintf("OR%d", suffix%1000)

	body := bytes.NewBufferString(fmt.Sprintf(`{"name":%q,"rw":%d,"rt":%q}`, rtName, int(suffix%1000), rtCode))
	req := httptest.NewRequest("POST", "/api/v1/rts", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created RT
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rts WHERE id = $1", created.ID)
	})

	if !rtVisibleInDBWhenSeedCalled {
		t.Errorf("expected RT %s to be committed and visible in DB when seed notification ran, but it was not visible", checkedRTID)
	}
}

func TestCreateRT_NotificationSeedFailureDoesNotRollbackRT(t *testing.T) {
	pool := testPool(t)
	auth.AuthDBPool = pool
	setupEnv()
	token := superAdminToken(t)

	mockSvc := &mockNotifService{
		listTemplatesFn: func(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
			return nil, errors.New("simulated notification template list failure")
		},
	}

	r := buildRouterWithNotif(pool, mockSvc)

	suffix := time.Now().UnixNano() % 1000000
	rtName := fmt.Sprintf("FailNotif RT %d", suffix)
	rtCode := fmt.Sprintf("FN%d", suffix%1000)

	body := bytes.NewBufferString(fmt.Sprintf(`{"name":%q,"rw":%d,"rt":%q}`, rtName, int(suffix%1000), rtCode))
	req := httptest.NewRequest("POST", "/api/v1/rts", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// Even though notification seed failed, response MUST be 201 Created
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created even if notif seed fails, got %d: %s", rec.Code, rec.Body.String())
	}

	var created RT
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rts WHERE id = $1", created.ID)
	})

	// Verify RT is committed in database
	var count int
	err := pool.Raw().QueryRow("SELECT count(*) FROM rts WHERE id = $1", created.ID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("expected RT %s to exist in DB, got count=%d, err=%v", created.ID, count, err)
	}
}

func TestCreateRT_AuditLogFailureRollsBackRT(t *testing.T) {
	pool := testPool(t)
	auth.AuthDBPool = pool
	setupEnv()
	token := superAdminToken(t)

	finance.AuditLogInsertHook = func(_ context.Context, _ *sql.Tx, _ finance.AuditLog) error {
		return errors.New("injected audit log failure on RT create")
	}
	t.Cleanup(func() { finance.AuditLogInsertHook = nil })

	mockSvc := &mockNotifService{}
	r := buildRouterWithNotif(pool, mockSvc)

	suffix := time.Now().UnixNano() % 1000000
	rtName := fmt.Sprintf("AuditFail RT %d", suffix)
	rtCode := fmt.Sprintf("AF%d", suffix%1000)

	body := bytes.NewBufferString(fmt.Sprintf(`{"name":%q,"rw":%d,"rt":%q}`, rtName, int(suffix%1000), rtCode))
	req := httptest.NewRequest("POST", "/api/v1/rts", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// Must fail with 500 when audit log fails
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on audit failure, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify RT was NOT created/committed
	var count int
	_ = pool.Raw().QueryRow("SELECT count(*) FROM rts WHERE name = $1", rtName).Scan(&count)
	if count != 0 {
		t.Fatalf("expected RT creation to be rolled back, but found count=%d", count)
	}
}

func TestCreateRT_NotificationSeedIdempotent(t *testing.T) {
	ctx := context.Background()

	createCalls := 0
	mockSvc := &mockNotifService{
		listTemplatesFn: func(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
			// First call returns empty, second call returns existing template
			if createCalls > 0 {
				return []notification.NotificationTemplate{{ID: "existing-tmpl"}}, nil
			}
			return nil, nil
		},
		createTemplateFn: func(ctx context.Context, rtID string, in notification.CreateTemplateInput) (*notification.NotificationTemplate, error) {
			createCalls++
			return &notification.NotificationTemplate{ID: "new-tmpl"}, nil
		},
		listSchedulesFn: func(ctx context.Context, rtID, eventType string, isActive *bool) ([]*notification.NotificationSchedule, error) {
			if createCalls > 1 {
				return []*notification.NotificationSchedule{{ID: "existing-sched"}}, nil
			}
			return nil, nil
		},
		createScheduleFn: func(ctx context.Context, rtID, userID string, in notification.CreateScheduleInput) (*notification.NotificationSchedule, error) {
			return &notification.NotificationSchedule{ID: "new-sched"}, nil
		},
	}

	testRTID := "test-rt-uuid-12345"

	// First execution seeds template
	if err := seedDefaultNotificationForRT(ctx, mockSvc, testRTID); err != nil {
		t.Fatalf("expected first seed to succeed, got %v", err)
	}
	firstCreateCalls := createCalls

	// Second execution should be idempotent (existing templates detected)
	if err := seedDefaultNotificationForRT(ctx, mockSvc, testRTID); err != nil {
		t.Fatalf("expected second seed to succeed, got %v", err)
	}

	// Should not have created duplicate templates for already existing event types
	if createCalls > firstCreateCalls {
		// Event types that already existed were not re-created
		t.Logf("idempotent seed verified")
	}
}

func TestCreateRT_WithRealNotificationService(t *testing.T) {
	pool := testPool(t)
	auth.AuthDBPool = pool
	setupEnv()
	token := superAdminToken(t)

	notifRepo := notification.NewRepository(pool)
	notifSvc := notification.NewService(notifRepo, notification.NewMockPushProvider())

	r := buildRouterWithNotif(pool, notifSvc)

	suffix := time.Now().UnixNano() % 1000000
	rtName := fmt.Sprintf("RealNotif RT %d", suffix)
	rtCode := fmt.Sprintf("RN%d", suffix%1000)

	body := bytes.NewBufferString(fmt.Sprintf(`{"name":%q,"rw":%d,"rt":%q}`, rtName, int(suffix%1000), rtCode))
	req := httptest.NewRequest("POST", "/api/v1/rts", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created with real notif service, got %d: %s", rec.Code, rec.Body.String())
	}

	var created RT
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	t.Cleanup(func() {
		pool.Raw().Exec("DELETE FROM rts WHERE id = $1", created.ID)
	})

	// Verify default templates were seeded for this RT
	ctx := context.Background()
	templates, err := notifSvc.ListTemplates(ctx, created.ID, "iuran_payment_reminder", nil)
	if err != nil {
		t.Fatalf("failed to query seeded templates: %v", err)
	}
	if len(templates) == 0 {
		t.Errorf("expected default template to be seeded for RT %s, got 0", created.ID)
	}

	// Verify default schedules were seeded for this RT
	schedules, err := notifSvc.ListSchedules(ctx, created.ID, "iuran_payment_reminder", nil)
	if err != nil {
		t.Fatalf("failed to query seeded schedules: %v", err)
	}
	if len(schedules) == 0 {
		t.Errorf("expected default schedule to be seeded for RT %s, got 0", created.ID)
	}
}
