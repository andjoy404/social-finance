package notification_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

