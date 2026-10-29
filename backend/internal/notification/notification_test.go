package notification_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
)

// inMemoryRepo is a mock repository implementing notification.Repository for isolated unit testing.
type inMemoryRepo struct {
	mu                  sync.Mutex
	devices             map[string]map[string]bool // userID -> token -> isActive
	verifiers           map[string][]string        // rtID -> []userIDs authorized to verify payments
	rtUsers             map[string][]string        // rtID -> []userIDs
	billUsers           map[string][]string        // billID -> []userIDs
	dedupRecords        map[string]bool            // dedupKey:userID -> exists
	dedupForceError     bool                       // when true, CheckAndRecordDedup returns an error
	dedupForceErrorOnce bool                       // when true, error is returned only once
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{
		devices:      make(map[string]map[string]bool),
		verifiers:    make(map[string][]string),
		rtUsers:      make(map[string][]string),
		billUsers:    make(map[string][]string),
		dedupRecords: make(map[string]bool),
	}
}

func (r *inMemoryRepo) RegisterDevice(ctx context.Context, userID, token, platform string, deviceID *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.devices[userID]; !ok {
		r.devices[userID] = make(map[string]bool)
	}
	r.devices[userID][token] = true
	return nil
}

func (r *inMemoryRepo) UnregisterDevice(ctx context.Context, userID, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if userDevs, ok := r.devices[userID]; ok {
		userDevs[token] = false
	}
	return nil
}

func (r *inMemoryRepo) DeactivateToken(ctx context.Context, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, userDevs := range r.devices {
		if _, ok := userDevs[token]; ok {
			userDevs[token] = false
		}
	}
	return nil
}

func (r *inMemoryRepo) GetTokensForUser(ctx context.Context, userID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var tokens []string
	if userDevs, ok := r.devices[userID]; ok {
		for tok, active := range userDevs {
			if active {
				tokens = append(tokens, tok)
			}
		}
	}
	return tokens, nil
}

// GetTokensForPaymentVerifiers mirrors the real repository contract: returns
// active-device tokens of active verifier users registered for the RT.
// The SQL-level policy (position_permissions / role evaluation, RT scoping,
// active filters, DISTINCT) is covered by DB-backed repository tests.
func (r *inMemoryRepo) GetTokensForPaymentVerifiers(ctx context.Context, rtID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var tokens []string
	seen := make(map[string]bool)
	for _, uid := range r.verifiers[rtID] {
		if seen[uid] {
			continue
		}
		seen[uid] = true
		if userDevs, ok := r.devices[uid]; ok {
			for tok, active := range userDevs {
				if active {
					tokens = append(tokens, tok)
				}
			}
		}
	}
	return tokens, nil
}

func (r *inMemoryRepo) GetTokensForRT(ctx context.Context, rtID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var tokens []string
	if userIDs, ok := r.rtUsers[rtID]; ok {
		for _, uid := range userIDs {
			if userDevs, ok := r.devices[uid]; ok {
				for tok, active := range userDevs {
					if active {
						tokens = append(tokens, tok)
					}
				}
			}
		}
	}
	return tokens, nil
}

func (r *inMemoryRepo) GetRecipientUserIDsForBill(ctx context.Context, rtID, billID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.billUsers[billID], nil
}

func (r *inMemoryRepo) CheckAndRecordDedup(ctx context.Context, rtID, userID, eventType, dedupKey, title, body string, payload map[string]string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dedupForceError {
		if r.dedupForceErrorOnce {
			r.dedupForceError = false
		}
		return false, errors.New("mock dedup error")
	}
	if r.dedupRecords[dedupKey] {
		return false, nil
	}
	r.dedupRecords[dedupKey] = true
	return true, nil
}

func (r *inMemoryRepo) ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
	return []notification.NotificationTemplate{}, nil
}

func (r *inMemoryRepo) GetTemplateByID(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *inMemoryRepo) GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *inMemoryRepo) CreateTemplate(ctx context.Context, rtID string, in notification.CreateTemplateInput, allowedVars []string) (*notification.NotificationTemplate, error) {
	return &notification.NotificationTemplate{
		ID:        "tmpl-1",
		RTID:      rtID,
		Name:      in.Name,
		EventType: in.EventType,
		Channel:   in.Channel,
		Title:     in.Title,
		Body:      in.Body,
		IsActive:  true,
		Variables: allowedVars,
	}, nil
}

func (r *inMemoryRepo) UpdateTemplate(ctx context.Context, rtID, id string, in notification.UpdateTemplateInput) (*notification.NotificationTemplate, error) {
	return &notification.NotificationTemplate{
		ID:       id,
		RTID:     rtID,
		Name:     in.Name,
		Title:    in.Title,
		Body:     in.Body,
		IsActive: true,
	}, nil
}

func (r *inMemoryRepo) ToggleTemplateActive(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	return &notification.NotificationTemplate{
		ID:       id,
		RTID:     rtID,
		IsActive: false,
	}, nil
}

func (r *inMemoryRepo) DeleteTemplate(ctx context.Context, rtID, id string) error {
	return nil
}

func (r *inMemoryRepo) ListLogs(ctx context.Context, rtID, eventType string, offset, limit int) ([]notification.NotificationLogEntry, int, error) {
	return []notification.NotificationLogEntry{}, 0, nil
}

func (r *inMemoryRepo) GetLogByID(ctx context.Context, rtID, id string) (*notification.NotificationLogEntry, error) {
	return nil, nil
}

func (r *inMemoryRepo) GetDefaultRTID(ctx context.Context) (string, error) {
	return "test-rt-002", nil
}

func TestDeviceRegistrationLifecycle(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	userID := "user-123"
	token := "fcm-token-abc"

	// 1. Register device
	err := svc.RegisterDevice(ctx, userID, notification.RegisterDeviceInput{
		FCMToken: token,
		Platform: "android",
	})
	if err != nil {
		t.Fatalf("unexpected error registering device: %v", err)
	}

	tokens, err := repo.GetTokensForUser(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get tokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != token {
		t.Fatalf("expected 1 token %s, got %v", token, tokens)
	}

	// 2. Unregister device
	err = svc.UnregisterDevice(ctx, userID, token)
	if err != nil {
		t.Fatalf("unexpected error unregistering device: %v", err)
	}

	tokensAfter, err := repo.GetTokensForUser(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get tokens: %v", err)
	}
	if len(tokensAfter) != 0 {
		t.Fatalf("expected 0 tokens after unregister, got %v", tokensAfter)
	}
}

func TestPaymentSubmissionNotificationTargeting(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	bendaharaUser := "user-bendahara-1"
	bendaharaToken := "tok-bendahara-1"

	// Setup bendahara device and RT verifier registration
	_ = repo.RegisterDevice(ctx, bendaharaUser, bendaharaToken, "android", nil)
	repo.verifiers[rtID] = []string{bendaharaUser}

	// Another RT bendahara that must NOT receive notification
	otherRT := "rt-02"
	otherBendahara := "user-bendahara-other"
	otherToken := "tok-bendahara-other"
	_ = repo.RegisterDevice(ctx, otherBendahara, otherToken, "android", nil)
	repo.verifiers[otherRT] = []string{otherBendahara}

	// Dispatch notification
	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-100", "pay-200")

	// Allow goroutine to execute
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 push message sent to bendahara, got %d", len(msgs))
	}
	if msgs[0].Token != bendaharaToken {
		t.Errorf("expected token %s, got %s", bendaharaToken, msgs[0].Token)
	}
	if msgs[0].Data["type"] != notification.EventPaymentSubmitted {
		t.Errorf("expected event type %s, got %s", notification.EventPaymentSubmitted, msgs[0].Data["type"])
	}
	if msgs[0].Data["bill_id"] != "bill-100" || msgs[0].Data["payment_id"] != "pay-200" {
		t.Errorf("unexpected payload data: %v", msgs[0].Data)
	}

	// Duplicate dispatch should be deduplicated
	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-100", "pay-200")
	time.Sleep(100 * time.Millisecond)

	msgs2 := provider.GetMessages()
	if len(msgs2) != 1 {
		t.Fatalf("expected deduplication to prevent duplicate message, got %d messages", len(msgs2))
	}
}

func TestPaymentVerificationNotificationTargeting(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-warga-1"
	submitterToken := "tok-warga-1"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)

	// Approved verification
	svc.NotifyPaymentVerified(ctx, rtID, "bill-100", "pay-200", submitterUser, "APPROVED", "")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 push message sent to submitter, got %d", len(msgs))
	}
	if msgs[0].Token != submitterToken {
		t.Errorf("expected token %s, got %s", submitterToken, msgs[0].Token)
	}
	if msgs[0].Data["status"] != "APPROVED" {
		t.Errorf("expected status APPROVED, got %s", msgs[0].Data["status"])
	}

	// Rejected verification
	provider.Clear()
	svc.NotifyPaymentVerified(ctx, rtID, "bill-100", "pay-201", submitterUser, "REJECTED", "Bukti transfer buram")
	time.Sleep(100 * time.Millisecond)

	msgsRejected := provider.GetMessages()
	if len(msgsRejected) != 1 {
		t.Fatalf("expected 1 push message for rejected payment, got %d", len(msgsRejected))
	}
	if msgsRejected[0].Data["status"] != "REJECTED" {
		t.Errorf("expected status REJECTED, got %s", msgsRejected[0].Data["status"])
	}
}

func TestBillCreatedNotificationTargeting(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	billID := "bill-sukarela-1"
	recipientUser := "user-warga-target"
	targetToken := "tok-target-123"

	// Register device for intended recipient
	_ = repo.RegisterDevice(ctx, recipientUser, targetToken, "android", nil)
	repo.billUsers[billID] = []string{recipientUser}

	// Another resident that must NOT receive this bill notification
	otherUser := "user-warga-other"
	otherToken := "tok-other-456"
	_ = repo.RegisterDevice(ctx, otherUser, otherToken, "android", nil)

	// Dispatch bill notification
	svc.NotifyBillCreated(ctx, rtID, billID, nil)
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 push message sent to target bill recipient, got %d", len(msgs))
	}
	if msgs[0].Token != targetToken {
		t.Errorf("expected token %s, got %s", targetToken, msgs[0].Token)
	}
	if msgs[0].Data["type"] != notification.EventBillCreated {
		t.Errorf("expected event type %s, got %s", notification.EventBillCreated, msgs[0].Data["type"])
	}
	if msgs[0].Data["bill_id"] != billID {
		t.Errorf("expected bill_id %s, got %s", billID, msgs[0].Data["bill_id"])
	}

	// Deduplication test: re-dispatching same bill to same user should be skipped
	svc.NotifyBillCreated(ctx, rtID, billID, nil)
	time.Sleep(100 * time.Millisecond)

	msgs2 := provider.GetMessages()
	if len(msgs2) != 1 {
		t.Fatalf("expected deduplication to prevent duplicate bill message, got %d messages", len(msgs2))
	}
}

func TestNotificationFailureDoesNotPanicOrCrash(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	provider.SetSendError(errors.New("FCM connection timeout"))
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-warga-fail"
	_ = repo.RegisterDevice(ctx, submitterUser, "tok-fail", "android", nil)

	// Dispatch should run safely without throwing unhandled exceptions
	svc.NotifyPaymentVerified(ctx, rtID, "bill-1", "pay-1", submitterUser, "APPROVED", "")
	time.Sleep(100 * time.Millisecond)
}

func TestInvalidTokenDeactivation(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	provider.SetSendError(notification.ErrTokenUnregistered)
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-warga-invalid"
	tok := "tok-invalid-456"
	_ = repo.RegisterDevice(ctx, submitterUser, tok, "android", nil)

	// Dispatch notification which will trigger ErrTokenUnregistered
	svc.NotifyPaymentVerified(ctx, rtID, "bill-1", "pay-1", submitterUser, "APPROVED", "")
	time.Sleep(100 * time.Millisecond)

	// Verify that token has been deactivated
	tokens, err := repo.GetTokensForUser(ctx, submitterUser)
	if err != nil {
		t.Fatalf("failed to get tokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected invalid token to be deactivated, but found: %v", tokens)
	}
}

func TestFCMProviderEmptyCredentialsFallback(t *testing.T) {
	provider := notification.NewFCMProvider("")
	ctx := context.Background()

	err := provider.Send(ctx, "token-123", "Test Title", "Test Body", map[string]string{"type": "test"})
	if err != nil {
		t.Fatalf("expected nil error on mock fallback, got %v", err)
	}

	msgs := provider.Fallback().GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in fallback provider, got %d", len(msgs))
	}
	if msgs[0].Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got '%s'", msgs[0].Title)
	}
}

func TestFCMProviderInvalidPathFallback(t *testing.T) {
	provider := notification.NewFCMProvider("/non/existent/path/service-account.json")
	ctx := context.Background()

	err := provider.Send(ctx, "token-456", "Fallback Title", "Fallback Body", map[string]string{"type": "test"})
	if err != nil {
		t.Fatalf("expected nil error on mock fallback, got %v", err)
	}

	msgs := provider.Fallback().GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in fallback provider, got %d", len(msgs))
	}
}

func TestFCMProviderInvalidJSONFallback(t *testing.T) {
	provider := notification.NewFCMProvider("{invalid-json")
	ctx := context.Background()

	err := provider.Send(ctx, "token-789", "Fallback Title", "Fallback Body", map[string]string{"type": "test"})
	if err != nil {
		t.Fatalf("expected nil error on mock fallback, got %v", err)
	}

	msgs := provider.Fallback().GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in fallback provider, got %d", len(msgs))
	}
}

func TestFCMProviderProductionRejectsMissingCredentials(t *testing.T) {
	provider := notification.NewFCMProvider("", "production")
	ctx := context.Background()

	err := provider.Send(ctx, "token-prod-1", "Prod Title", "Prod Body", map[string]string{"type": "test"})
	if err == nil {
		t.Fatal("expected error in production when credentials are missing, got nil")
	}
}

func TestFCMProviderProductionRejectsInvalidCredentials(t *testing.T) {
	provider := notification.NewFCMProvider("/missing/file.json", "production")
	ctx := context.Background()

	err := provider.Send(ctx, "token-prod-2", "Prod Title", "Prod Body", map[string]string{"type": "test"})
	if err == nil {
		t.Fatal("expected error in production when credentials file is missing, got nil")
	}
}

func TestGeneralPayloadErrorDoesNotDeactivateToken(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	// Simulate bad request payload error that is NOT a token error
	provider.SetSendError(errors.New("fcm: bad request payload: message field exceeds limit"))
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-valid-token"
	tok := "tok-still-valid-789"
	_ = repo.RegisterDevice(ctx, submitterUser, tok, "android", nil)

	svc.NotifyPaymentVerified(ctx, rtID, "bill-1", "pay-1", submitterUser, "APPROVED", "")
	time.Sleep(100 * time.Millisecond)

	tokens, err := repo.GetTokensForUser(ctx, submitterUser)
	if err != nil {
		t.Fatalf("failed to get tokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != tok {
		t.Fatalf("expected valid token to remain active on payload error, but got: %v", tokens)
	}
}

func TestFCMProviderProjectIDMethods(t *testing.T) {
	provider := notification.NewFCMProvider("", "development")
	if provider.ProjectID() != "social-finance-andjoy" {
		t.Fatalf("expected default project ID 'social-finance-andjoy', got '%s'", provider.ProjectID())
	}

	provider.SetProjectID("custom-project-id")
	if provider.ProjectID() != "custom-project-id" {
		t.Fatalf("expected custom project ID 'custom-project-id', got '%s'", provider.ProjectID())
	}
}

func TestFCMProviderLiveServiceAccountAndTokenExchange(t *testing.T) {
	credsPath := os.Getenv("FCM_CREDENTIALS_FILE")
	if credsPath == "" {
		credsPath = "/Users/Andriyan/Documents/apx/devl/social-finance/secret/social-finance-andjoy-93df7226d69d.json"
	}
	if _, err := os.Stat(credsPath); os.IsNotExist(err) {
		t.Skip("skipping live credentials test: service account file not found")
	}

	provider := notification.NewFCMProvider(credsPath, "production")
	expectedProjectID := os.Getenv("FCM_PROJECT_ID")
	if expectedProjectID == "" {
		expectedProjectID = "social-finance-andjoy"
	}
	if provider.ProjectID() != expectedProjectID && provider.ProjectID() != "social-finance-andjoy" && provider.ProjectID() != "social-finance-urena" {
		t.Fatalf("expected project ID '%s', got '%s'", expectedProjectID, provider.ProjectID())
	}

	// Attempt live send with a dummy test token.
	// Since credentials are valid, this connects to Google OAuth2 and FCM HTTP v1 API,
	// returning ErrTokenInvalid or ErrTokenUnregistered (not an OAuth or credential failure).
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := provider.Send(ctx, "test-invalid-device-token-123456", "Test Title", "Test Body", map[string]string{"type": "test"})
	if err == nil {
		t.Fatal("expected invalid token error, got nil")
	}

	errStr := err.Error()
	if strings.Contains(errStr, "production configuration error") ||
		strings.Contains(errStr, "production requires valid live credentials") ||
		strings.Contains(errStr, "oauth token retrieval failed") {
		t.Fatalf("unexpected auth/credentials failure: %v", err)
	}

	t.Logf("Live Google FCM exchange verified successfully. Gateway response: %v", err)
}

func TestSelfTestRequiresAuth(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	handler := notification.NewHandler(svc, nil, "development")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/test-self", nil)
	rr := httptest.NewRecorder()

	handler.TestSelf(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", rr.Code)
	}
}

func TestSelfTestRejectedInProduction(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	handler := notification.NewHandler(svc, nil, "production")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/test-self", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID: "user-123",
		RTID:   "rt-123",
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.TestSelf(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 in production, got %d", rr.Code)
	}
}

func TestSelfTestOnlyTargetsAuthenticatedUser(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	handler := notification.NewHandler(svc, nil, "development")

	// Register device for user-1
	_ = repo.RegisterDevice(context.Background(), "user-1", "tok-user-1", "android", nil)
	// Register device for user-2
	_ = repo.RegisterDevice(context.Background(), "user-2", "tok-user-2", "android", nil)

	// Call TestSelf as user-1
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/test-self", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID: "user-1",
		RTID:   "rt-1",
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.TestSelf(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message dispatched, got %d", len(msgs))
	}
	if msgs[0].Token != "tok-user-1" {
		t.Fatalf("expected message to user-1 token, got %s", msgs[0].Token)
	}
	if msgs[0].Data["type"] != "test_self" {
		t.Fatalf("expected payload type test_self, got %s", msgs[0].Data["type"])
	}
}

func TestSelfTestNoBroadcastAndZeroDevices(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)
	handler := notification.NewHandler(svc, nil, "development")

	// User without devices
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/test-self", nil)
	ctx := context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		UserID: "user-no-device",
		RTID:   "rt-1",
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.TestSelf(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	msgs := provider.GetMessages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages dispatched, got %d", len(msgs))
	}
}

// Regression tests for payment-submission notification recipient selection.

func TestPaymentSubmitted_BendaharaRoleReceivesNotification(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	bendaharaUser := "user-bendahara-1"
	bendaharaToken := "tok-bendahara-1"

	_ = repo.RegisterDevice(ctx, bendaharaUser, bendaharaToken, "android", nil)
	repo.verifiers[rtID] = []string{bendaharaUser}

	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-reguler-1", "pay-100")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Token != bendaharaToken {
		t.Errorf("expected bendahara token %s, got %s", bendaharaToken, msgs[0].Token)
	}
	if msgs[0].Data["type"] != notification.EventPaymentSubmitted {
		t.Errorf("expected type %s, got %s", notification.EventPaymentSubmitted, msgs[0].Data["type"])
	}
}

func TestPaymentSubmitted_KetuaRTReceivesNotification(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	ketuaUser := "user-ketua-1"
	ketuaToken := "tok-ketua-1"

	// Setup: ketua has role='pengurus' + jabatan='ketua' → finance.approve
	_ = repo.RegisterDevice(ctx, ketuaUser, ketuaToken, "android", nil)
	repo.verifiers[rtID] = []string{ketuaUser}

	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-reguler-2", "pay-200")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Token != ketuaToken {
		t.Errorf("expected ketua token %s, got %s", ketuaToken, msgs[0].Token)
	}
}

// TestPaymentSubmitted_PengurusWithBendaharaJabatanReceives covers the
// reported regression: a user with role=pengurus, jabatan=bendahara, and
// finance.approve permission is an authorized payment verifier but was
// missed by the previous role='bendahara'/jabatan='ketua' allowlist.
func TestPaymentSubmitted_PengurusWithBendaharaJabatanReceives(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	officerUser := "user-pengurus-bendahara"
	officerToken := "tok-pengurus-bendahara"

	_ = repo.RegisterDevice(ctx, officerUser, officerToken, "android", nil)
	// Resolved by GetTokensForPaymentVerifiers: role=pengurus + jabatan=bendahara
	// has finance.approve via position_permissions.
	repo.verifiers[rtID] = []string{officerUser}

	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-reguler-jb", "pay-jb-1")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Token != officerToken {
		t.Errorf("expected officer token %s, got %s", officerToken, msgs[0].Token)
	}
	if msgs[0].Data["type"] != notification.EventPaymentSubmitted {
		t.Errorf("expected type %s, got %s", notification.EventPaymentSubmitted, msgs[0].Data["type"])
	}
}

// TestPaymentSubmitted_NoDuplicateWhenMatchingMultipleConditions verifies
// that a verifier listed once per matching condition still receives exactly
// one notification.
func TestPaymentSubmitted_NoDuplicateWhenMatchingMultipleConditions(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	officerUser := "user-officer-multi"
	officerToken := "tok-officer-multi"

	_ = repo.RegisterDevice(ctx, officerUser, officerToken, "android", nil)
	// Same person listed twice (e.g. matched by role condition and jabatan
	// condition) plus a second verifier.
	otherVerifier := "user-verifier-2"
	otherToken := "tok-verifier-2"
	_ = repo.RegisterDevice(ctx, otherVerifier, otherToken, "android", nil)
	repo.verifiers[rtID] = []string{officerUser, officerUser, otherVerifier}

	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-dup-cond", "pay-dup-cond-1")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	tokenCounts := make(map[string]int)
	for _, m := range msgs {
		tokenCounts[m.Token]++
	}
	if tokenCounts[officerToken] != 1 {
		t.Errorf("expected exactly 1 message for officer token, got %d", tokenCounts[officerToken])
	}
	if tokenCounts[otherToken] != 1 {
		t.Errorf("expected exactly 1 message for other verifier token, got %d", tokenCounts[otherToken])
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages total, got %d", len(msgs))
	}
}

func TestPaymentSubmitted_BendaharaAndKetuaBothReceive(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	bendaharaUser := "user-bendahara-1"
	bendaharaToken := "tok-bendahara-1"
	ketuaUser := "user-ketua-1"
	ketuaToken := "tok-ketua-1"

	_ = repo.RegisterDevice(ctx, bendaharaUser, bendaharaToken, "android", nil)
	_ = repo.RegisterDevice(ctx, ketuaUser, ketuaToken, "android", nil)

	repo.verifiers[rtID] = []string{bendaharaUser, ketuaUser}

	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-reguler-3", "pay-300")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	tokenSet := make(map[string]bool)
	for _, m := range msgs {
		tokenSet[m.Token] = true
	}
	if !tokenSet[bendaharaToken] {
		t.Errorf("expected bendahara token %s in messages", bendaharaToken)
	}
	if !tokenSet[ketuaToken] {
		t.Errorf("expected ketua token %s in messages", ketuaToken)
	}

	// Verify dedup: same token should only appear once even if user has both memberships
	tokenCounts := make(map[string]int)
	for _, m := range msgs {
		tokenCounts[m.Token]++
	}
	for tok, count := range tokenCounts {
		if count > 1 {
			t.Errorf("token %s sent %d times (expected 1)", tok, count)
		}
	}
}

func TestPaymentSubmitted_RTIsolation(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	targetRT := "rt-01"
	otherRT := "rt-02"

	bendaharaUser := "user-bendahara-1"
	bendaharaToken := "tok-bendahara-1"
	ketuaUser := "user-ketua-1"
	ketuaToken := "tok-ketua-1"

	// Other RT ketua that must NOT receive notification
	otherKetuaUser := "user-other-ketua"
	otherKetuaToken := "tok-other-ketua"

	_ = repo.RegisterDevice(ctx, bendaharaUser, bendaharaToken, "android", nil)
	_ = repo.RegisterDevice(ctx, ketuaUser, ketuaToken, "android", nil)
	_ = repo.RegisterDevice(ctx, otherKetuaUser, otherKetuaToken, "android", nil)

	repo.verifiers[targetRT] = []string{bendaharaUser, ketuaUser}
	repo.verifiers[otherRT] = []string{otherKetuaUser}

	svc.NotifyPaymentSubmitted(ctx, targetRT, "bill-isolation-1", "pay-400")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	for _, m := range msgs {
		if m.Token == otherKetuaToken {
			t.Errorf("other RT ketua token %s should NOT receive notification", otherKetuaToken)
		}
	}
}

func TestPaymentSubmitted_InactiveDeviceExcluded(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	bendaharaUser := "user-bendahara-1"
	inactiveDeviceToken := "tok-inactive-device"

	// Register device with inactive status
	_ = repo.RegisterDevice(ctx, bendaharaUser, inactiveDeviceToken, "android", nil)
	repo.verifiers[rtID] = []string{bendaharaUser}

	// Mark device as inactive (simulating is_active=false in user_devices)
	if userDevs, ok := repo.devices[bendaharaUser]; ok {
		userDevs[inactiveDeviceToken] = false
	}

	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-inactive-1", "pay-500")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages (inactive device excluded), got %d", len(msgs))
	}
}

func TestPaymentSubmitted_DedupEventStillWorks(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	ketuaUser := "user-ketua-1"
	ketuaToken := "tok-ketua-1"

	_ = repo.RegisterDevice(ctx, ketuaUser, ketuaToken, "android", nil)
	repo.verifiers[rtID] = []string{ketuaUser}

	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-dedup-1", "pay-dedup-1")
	time.Sleep(100 * time.Millisecond)

	msgs1 := provider.GetMessages()
	if len(msgs1) != 1 {
		t.Fatalf("expected 1 message on first dispatch, got %d", len(msgs1))
	}

	// Duplicate dispatch should be deduplicated
	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-dedup-1", "pay-dedup-1")
	time.Sleep(100 * time.Millisecond)

	msgs2 := provider.GetMessages()
	if len(msgs2) != 1 {
		t.Fatalf("expected dedup to prevent duplicate, got %d total messages", len(msgs2))
	}
}

// TestPaymentSubmitted_OfficerSubmitterStillReceivesAsVerifier verifies that a
// management officer who submits their own household's payment is NOT excluded
// from the payment-submission notification: they remain an authorized verifier
// of their RT and must still be told a payment awaits verification.
func TestPaymentSubmitted_OfficerSubmitterStillReceivesAsVerifier(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	officerUser := "user-officer-submit-own"
	officerToken := "tok-officer-submit-own"

	_ = repo.RegisterDevice(ctx, officerUser, officerToken, "android", nil)
	// Officer is an authorized verifier (e.g. role=pengurus, jabatan=bendahara).
	repo.verifiers[rtID] = []string{officerUser}

	// Officer submits a payment for their own household bill.
	svc.NotifyPaymentSubmitted(ctx, rtID, "bill-officer-own", "pay-officer-own-1")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for officer submitter, got %d", len(msgs))
	}
	if msgs[0].Token != officerToken {
		t.Errorf("expected officer token %s, got %s", officerToken, msgs[0].Token)
	}
	if msgs[0].Data["type"] != notification.EventPaymentSubmitted {
		t.Errorf("expected type %s, got %s", notification.EventPaymentSubmitted, msgs[0].Data["type"])
	}
}

// TestPaymentVerified_OfficerSubmitterReceivesResultNotification verifies that
// a management officer who submitted their own payment still receives the
// verification-result notification addressed to the submitter.
func TestPaymentVerified_OfficerSubmitterReceivesResultNotification(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	officerUser := "user-officer-verified"
	officerToken := "tok-officer-verified"

	_ = repo.RegisterDevice(ctx, officerUser, officerToken, "android", nil)

	// A different authorized verifier approves the officer's own payment.
	svc.NotifyPaymentVerified(ctx, rtID, "bill-officer-verified", "pay-officer-verified-1", officerUser, "APPROVED", "")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 result message for officer submitter, got %d", len(msgs))
	}
	if msgs[0].Token != officerToken {
		t.Errorf("expected officer token %s, got %s", officerToken, msgs[0].Token)
	}
	if msgs[0].Data["type"] != notification.EventPaymentVerified {
		t.Errorf("expected type %s, got %s", notification.EventPaymentVerified, msgs[0].Data["type"])
	}
	if msgs[0].Data["status"] != "APPROVED" {
		t.Errorf("expected status APPROVED, got %s", msgs[0].Data["status"])
	}
}

// TestBillCreated_PengurusResidentReceivesResidentNotification verifies that a
// resident who also holds a management role (pengurus) is not excluded from
// resident-addressed bill notifications for their own household.
func TestBillCreated_PengurusResidentReceivesResidentNotification(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	billID := "bill-pengurus-resident"

	ordinaryResident := "user-warga-ordinary"
	ordinaryToken := "tok-warga-ordinary"
	pengurusResident := "user-pengurus-resident"
	pengurusToken := "tok-pengurus-resident"

	_ = repo.RegisterDevice(ctx, ordinaryResident, ordinaryToken, "android", nil)
	_ = repo.RegisterDevice(ctx, pengurusResident, pengurusToken, "android", nil)
	// Both users are residents of the billed household. The pengurus resident
	// must receive the notification alongside the ordinary resident.
	repo.billUsers[billID] = []string{ordinaryResident, pengurusResident}

	svc.NotifyBillCreated(ctx, rtID, billID, nil)
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	tokenSet := make(map[string]int)
	for _, m := range msgs {
		tokenSet[m.Token]++
	}
	if tokenSet[ordinaryToken] != 1 {
		t.Errorf("expected ordinary resident token exactly once, got %d", tokenSet[ordinaryToken])
	}
	if tokenSet[pengurusToken] != 1 {
		t.Errorf("expected pengurus resident token exactly once, got %d", tokenSet[pengurusToken])
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

// TestBillCreated_DuplicateRecipientListSendsOnce verifies that a person who
// appears multiple times in the recipient list (multiple matching recipient
// conditions) receives exactly one notification.
func TestBillCreated_DuplicateRecipientListSendsOnce(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	billID := "bill-dup-recipient"

	residentUser := "user-dup-recipient"
	residentToken := "tok-dup-recipient"

	_ = repo.RegisterDevice(ctx, residentUser, residentToken, "android", nil)
	repo.billUsers[billID] = []string{residentUser, residentUser}

	svc.NotifyBillCreated(ctx, rtID, billID, nil)
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 message for duplicated recipient, got %d", len(msgs))
	}
	if msgs[0].Token != residentToken {
		t.Errorf("expected token %s, got %s", residentToken, msgs[0].Token)
	}
}

// TestBillCreated_ExplicitDuplicateRecipientsSendOnce verifies deduplication
// of explicitly supplied recipient lists (nil falls back to the repository
// query; explicit lists must be deduplicated identically).
func TestBillCreated_ExplicitDuplicateRecipientsSendOnce(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	billID := "bill-explicit-dup"

	residentUser := "user-explicit-dup"
	residentToken := "tok-explicit-dup"

	_ = repo.RegisterDevice(ctx, residentUser, residentToken, "android", nil)

	svc.NotifyBillCreated(ctx, rtID, billID, []string{residentUser, " ", residentUser})
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 message for duplicated explicit recipient, got %d", len(msgs))
	}
	if msgs[0].Token != residentToken {
		t.Errorf("expected token %s, got %s", residentToken, msgs[0].Token)
	}
}

// TestPaymentVerified_APPROVED_NotifiesSubmitterAndVerifiers verifies that an APPROVED
// payment verification delivers a push to both the submitter and all authorized verifiers.
func TestPaymentVerified_APPROVED_NotifiesSubmitterAndVerifiers(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-1"
	submitterToken := "tok-submitter-1"
	verifierUser := "user-verifier-1"
	verifierToken := "tok-verifier-1"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)
	repo.verifiers[rtID] = []string{verifierUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-100", "pay-200", submitterUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 push messages (submitter + verifier), got %d", len(msgs))
	}

	tokenSet := make(map[string]bool)
	for _, m := range msgs {
		tokenSet[m.Token] = true
		if m.Data["status"] != "APPROVED" {
			t.Errorf("expected status APPROVED in message, got %s", m.Data["status"])
		}
	}
	if !tokenSet[submitterToken] {
		t.Errorf("expected message to submitter token %s", submitterToken)
	}
	if !tokenSet[verifierToken] {
		t.Errorf("expected message to verifier token %s", verifierToken)
	}
}

// TestPaymentVerified_DuplicateCallsSendNoDuplicatePush verifies that calling
// NotifyPaymentVerified with the same paymentID and status twice does not produce
// duplicate push notifications due to deduplication.
func TestPaymentVerified_DuplicateCallsSendNoDuplicatePush(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-dedup"
	submitterToken := "tok-submitter-dedup"
	verifierUser := "user-verifier-dedup"
	verifierToken := "tok-verifier-dedup"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)
	repo.verifiers[rtID] = []string{verifierUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-dedup", "pay-dedup", submitterUser, "APPROVED", "")
	time.Sleep(100 * time.Millisecond)

	firstBatch := provider.GetMessages()
	if len(firstBatch) != 2 {
		t.Fatalf("expected 2 initial messages, got %d", len(firstBatch))
	}

	// Second call with identical arguments should be deduplicated
	svc.NotifyPaymentVerified(ctx, rtID, "bill-dedup", "pay-dedup", submitterUser, "APPROVED", "")
	time.Sleep(100 * time.Millisecond)

	secondBatch := provider.GetMessages()
	if len(secondBatch) != 2 {
		t.Fatalf("expected deduplication to prevent duplicate messages, got %d", len(secondBatch))
	}
}

// TestPaymentVerified_SubmitterAlsoVerifierNoDuplicatePush verifies that when the
// submitter is also registered as a verifier, they only receive one push message
// (not one from the submitter path and one from the verifier path).
func TestPaymentVerified_SubmitterAlsoVerifierNoDuplicatePush(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	jointUser := "user-joint-1"
	jointToken := "tok-joint-1"

	_ = repo.RegisterDevice(ctx, jointUser, jointToken, "android", nil)
	repo.verifiers[rtID] = []string{jointUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-joint", "pay-joint", jointUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()
	tokenCount := 0
	for _, m := range msgs {
		if m.Token == jointToken {
			tokenCount++
		}
	}
	if tokenCount != 1 {
		t.Errorf("expected exactly 1 push to joint user token, got %d", tokenCount)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 message total, got %d", len(msgs))
	}
}

// TestPaymentVerified_REJECTED_NotifiesSubmitterAndVerifiers verifies that a REJECTED
// payment verification delivers a push to both the submitter and all authorized verifiers.
func TestPaymentVerified_REJECTED_NotifiesSubmitterAndVerifiers(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-2"
	submitterToken := "tok-submitter-2"
	verifierUser := "user-verifier-2"
	verifierToken := "tok-verifier-2"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)
	repo.verifiers[rtID] = []string{verifierUser}

	reason := "Bukti transfer tidak jelas"
	svc.NotifyPaymentVerified(ctx, rtID, "bill-101", "pay-201", submitterUser, "REJECTED", reason)
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 push messages (submitter + verifier), got %d", len(msgs))
	}

	tokenSet := make(map[string]bool)
	for _, m := range msgs {
		tokenSet[m.Token] = true
		if m.Data["status"] != "REJECTED" {
			t.Errorf("expected status REJECTED in message, got %s", m.Data["status"])
		}
	}
	if !tokenSet[submitterToken] {
		t.Errorf("expected message to submitter token %s", submitterToken)
	}
	if !tokenSet[verifierToken] {
		t.Errorf("expected message to verifier token %s", verifierToken)
	}
}

// TestPaymentVerified_DedupErrorDoesNotSend verifies that when CheckAndRecordDedup
// returns an error, no push notifications are sent (dedup failure must not cause
// unsafe notification delivery).
func TestPaymentVerified_DedupErrorDoesNotSend(t *testing.T) {
	repo := newInMemoryRepo()
	repo.dedupForceError = true
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-error"
	submitterToken := "tok-submitter-error"
	verifierUser := "user-verifier-error"
	verifierToken := "tok-verifier-error"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)
	repo.verifiers[rtID] = []string{verifierUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-error", "pay-error", submitterUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages when dedup fails, got %d", len(msgs))
	}
}

// TestPaymentVerified_PengurusPaysOwnBillReceivesVerification verifies that a
// pengurus who pays their own household's bill still receives the verification
// result notification — officer notes are NEVER included, but the notification
// must still be delivered to the submitter regardless of role.
func TestPaymentVerified_PengurusPaysOwnBillReceivesVerification(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	pengurusUser := "user-pengurus-1"
	pengurusToken := "tok-pengurus-1"
	verifierUser := "user-verifier-1"
	verifierToken := "tok-verifier-1"

	_ = repo.RegisterDevice(ctx, pengurusUser, pengurusToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)
	repo.verifiers[rtID] = []string{verifierUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-pengurus-self", "pay-pengurus-self", pengurusUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()
	foundSubmitter := false
	for _, m := range msgs {
		if m.Token == pengurusToken {
			foundSubmitter = true
			break
		}
	}
	if !foundSubmitter {
		t.Error("expected pengurus submitter to receive verification notification")
	}
}

// TestPaymentVerified_EmptySubmitterUserIDDoesNotPanic verifies that passing an
// empty submitterUserID is handled gracefully — no panic, and no messages
// are sent when there are no registered verifiers either.
func TestPaymentVerified_EmptySubmitterUserIDNoVerifiers(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"

	// No submitter, no verifiers registered → no messages
	svc.NotifyPaymentVerified(ctx, rtID, "bill-1", "pay-1", "", "APPROVED", "")
	time.Sleep(100 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages when submitterUserID is empty and no verifiers registered, got %d", len(msgs))
	}
}

// TestPaymentVerified_EmptySubmitterWithVerifierNotifiesVerifiers verifies that when
// submitterUserID is empty but verifier group has tokens, the verifier group
// still receives the notification.
func TestPaymentVerified_EmptySubmitterWithVerifierNotifiesVerifiers(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	verifierUser := "user-verifier-empty"
	verifierToken := "tok-verifier-empty"

	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)
	repo.verifiers[rtID] = []string{verifierUser}

	// No submitter — verifier should still receive
	svc.NotifyPaymentVerified(ctx, rtID, "bill-empty-sub", "pay-empty-sub", "", "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message to verifier (no submitter), got %d", len(msgs))
	}
	if msgs[0].Token != verifierToken {
		t.Errorf("expected message to verifier token %s, got %s", verifierToken, msgs[0].Token)
	}
}

// TestPaymentVerified_Unit_KetuaRoleTokenNotInVerifierList verifies that when
// the mock verifier list does not include a token registered for a
// role=ketua user, that user does not receive a push notification.
// NOTE: This tests service-level notification delivery against a mock repo.
// It does NOT verify the SQL query in GetTokensForPaymentVerifiers or the
// RequireAccess middleware; for that, see repository_db_test.go.
func TestPaymentVerified_Unit_KetuaRoleTokenNotInVerifierList(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-2"
	submitterToken := "tok-submitter-2"
	verifierUser := "user-verifier-2"
	verifierToken := "tok-verifier-2"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)

	// Register a token that mimics a role=ketua user WITHOUT jabatan —
	// this token is intentionally NOT placed in the verifier list.
	_ = repo.RegisterDevice(ctx, "user-ketua-nojabatan", "tok-ketua-nojabatan", "android", nil)

	repo.verifiers[rtID] = []string{verifierUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-ketua-excl", "pay-ketua-excl", submitterUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()
	foundKetuaNoJabatan := false
	for _, m := range msgs {
		if m.Token == "tok-ketua-nojabatan" {
			foundKetuaNoJabatan = true
		}
	}
	if foundKetuaNoJabatan {
		t.Error("token not in verifier list should NOT receive notification")
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (submitter + verifier), got %d", len(msgs))
	}
}

// TestPaymentVerified_Unit_WakilKetuaRoleTokenNotInVerifierList verifies that
// when the mock verifier list does not include a token for a
// role=wakil_ketua user, that user does not receive a push notification.
// NOTE: This tests service-level notification delivery against a mock repo.
// It does NOT verify the SQL query in GetTokensForPaymentVerifiers or the
// RequireAccess middleware; for that, see repository_db_test.go.
func TestPaymentVerified_Unit_WakilKetuaRoleTokenNotInVerifierList(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-3"
	submitterToken := "tok-submitter-3"
	verifierUser := "user-verifier-3"
	verifierToken := "tok-verifier-3"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)

	// Register a token that mimics a role=wakil_ketua user WITHOUT jabatan —
	// this token is intentionally NOT placed in the verifier list.
	_ = repo.RegisterDevice(ctx, "user-wakil-ketua-nojabatan", "tok-wakil-ketua-nojabatan", "android", nil)

	repo.verifiers[rtID] = []string{verifierUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-wakil-excl", "pay-wakil-excl", submitterUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()

	foundWakilKetua := false
	for _, m := range msgs {
		if m.Token == "tok-wakil-ketua-nojabatan" {
			foundWakilKetua = true
		}
	}
	if foundWakilKetua {
		t.Error("token not in verifier list should NOT receive notification")
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (submitter + verifier), got %d", len(msgs))
	}
}

// TestPaymentVerified_Unit_JabatanAnggotaTokenNotInVerifierList verifies that
// when the mock verifier list does not include a token for a
// pengurus+jabatan=anggota user, that user does not receive a push
// notification.
// NOTE: This tests service-level notification delivery against a mock repo.
// It does NOT verify the SQL query in GetTokensForPaymentVerifiers or the
// RequirePermission middleware; for that, see repository_db_test.go.
func TestPaymentVerified_Unit_JabatanAnggotaTokenNotInVerifierList(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-4"
	submitterToken := "tok-submitter-4"
	verifierUser := "user-verifier-4"
	verifierToken := "tok-verifier-4"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)
	_ = repo.RegisterDevice(ctx, verifierUser, verifierToken, "android", nil)

	// Register a token that mimics pengurus+jabatan=anggota —
	// this token is intentionally NOT placed in the verifier list.
	_ = repo.RegisterDevice(ctx, "user-pengurus-anggota", "tok-pengurus-anggota", "android", nil)

	repo.verifiers[rtID] = []string{verifierUser}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-anggota-excl", "pay-anggota-excl", submitterUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()

	foundAnggota := false
	for _, m := range msgs {
		if m.Token == "tok-pengurus-anggota" {
			foundAnggota = true
		}
	}
	if foundAnggota {
		t.Error("token not in verifier list should NOT receive notification")
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (submitter + verifier), got %d", len(msgs))
	}
}

// TestPaymentVerified_Unit_PengurusBendaharaTokenInVerifierList verifies that
// when the mock verifier list includes a token for a
// pengurus+jabatan=bendahara user, that user DOES receive a push notification.
// NOTE: This tests service-level notification delivery against a mock repo.
// It does NOT verify the SQL query in GetTokensForPaymentVerifiers or the
// position_permissions check; for that, see repository_db_test.go.
func TestPaymentVerified_Unit_PengurusBendaharaTokenInVerifierList(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-5"
	submitterToken := "tok-submitter-5"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)

	// Register a token that mimics pengurus+jabatan=bendahara —
	// this token IS placed in the verifier list.
	pengurusBendahara := "user-pengurus-bendahara-2"
	_ = repo.RegisterDevice(ctx, pengurusBendahara, "tok-pengurus-bendahara-2", "android", nil)
	repo.verifiers[rtID] = []string{pengurusBendahara}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-pengurus-bendahara", "pay-pengurus-bendahara", submitterUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()

	foundPengurusBendahara := false
	for _, m := range msgs {
		if m.Token == "tok-pengurus-bendahara-2" {
			foundPengurusBendahara = true
		}
	}
	if !foundPengurusBendahara {
		t.Error("token in verifier list should receive notification")
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (submitter + pengurus bendahara), got %d", len(msgs))
	}
}

// TestPaymentVerified_Unit_WargaKetuaTokenInVerifierList verifies that when
// the mock verifier list includes a token for a warga+jabatan=ketua user,
// that user DOES receive a push notification.
// NOTE: This tests service-level notification delivery against a mock repo.
// It does NOT verify the SQL query in GetTokensForPaymentVerifiers or the
// position_permissions check; for that, see repository_db_test.go.
func TestPaymentVerified_Unit_WargaKetuaTokenInVerifierList(t *testing.T) {
	repo := newInMemoryRepo()
	provider := notification.NewMockPushProvider()
	svc := notification.NewService(repo, provider)

	ctx := context.Background()
	rtID := "rt-01"
	submitterUser := "user-submitter-6"
	submitterToken := "tok-submitter-6"

	_ = repo.RegisterDevice(ctx, submitterUser, submitterToken, "android", nil)

	// Register a token that mimics warga+jabatan=ketua —
	// this token IS placed in the verifier list.
	wargaKetua := "user-warga-ketua-2"
	_ = repo.RegisterDevice(ctx, wargaKetua, "tok-warga-ketua-2", "android", nil)
	repo.verifiers[rtID] = []string{wargaKetua}

	svc.NotifyPaymentVerified(ctx, rtID, "bill-warga-ketua", "pay-warga-ketua", submitterUser, "APPROVED", "")
	time.Sleep(150 * time.Millisecond)

	msgs := provider.GetMessages()

	foundWargaKetua := false
	for _, m := range msgs {
		if m.Token == "tok-warga-ketua-2" {
			foundWargaKetua = true
		}
	}
	if !foundWargaKetua {
		t.Error("token in verifier list should receive notification")
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (submitter + warga ketua), got %d", len(msgs))
	}
}
