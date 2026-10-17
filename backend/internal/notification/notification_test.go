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
	mu           sync.Mutex
	devices      map[string]map[string]bool // userID -> token -> isActive
	roles        map[string]map[string][]string // rtID -> role -> []userIDs
	rtUsers      map[string][]string // rtID -> []userIDs
	billUsers    map[string][]string // billID -> []userIDs
	dedupRecords map[string]bool // dedupKey:userID -> exists
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{
		devices:      make(map[string]map[string]bool),
		roles:        make(map[string]map[string][]string),
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

func (r *inMemoryRepo) GetTokensForRTAndRole(ctx context.Context, rtID, role string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var tokens []string
	if roleMap, ok := r.roles[rtID]; ok {
		if userIDs, ok := roleMap[role]; ok {
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
	if r.dedupRecords[dedupKey] {
		return false, nil
	}
	r.dedupRecords[dedupKey] = true
	return true, nil
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

	// Setup bendahara device and RT role
	_ = repo.RegisterDevice(ctx, bendaharaUser, bendaharaToken, "android", nil)
	repo.roles[rtID] = map[string][]string{
		"bendahara": {bendaharaUser},
	}

	// Another RT bendahara that must NOT receive notification
	otherRT := "rt-02"
	otherBendahara := "user-bendahara-other"
	otherToken := "tok-bendahara-other"
	_ = repo.RegisterDevice(ctx, otherBendahara, otherToken, "android", nil)
	repo.roles[otherRT] = map[string][]string{
		"bendahara": {otherBendahara},
	}

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
	if provider.ProjectID() != "social-finance-andjoy" {
		t.Fatalf("expected project ID 'social-finance-andjoy', got '%s'", provider.ProjectID())
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
	handler := notification.NewHandler(svc, "development")

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
	handler := notification.NewHandler(svc, "production")

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
	handler := notification.NewHandler(svc, "development")

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
	handler := notification.NewHandler(svc, "development")

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
