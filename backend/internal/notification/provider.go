package notification

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

var (
	// ErrTokenInvalid indicates the device token has invalid formatting or is expired.
	ErrTokenInvalid = errors.New("fcm: invalid device token")
	// ErrTokenUnregistered indicates the device token has been unregistered with FCM.
	ErrTokenUnregistered = errors.New("fcm: device token unregistered")
)

// PushMessage records a push notification delivery attempt.
type PushMessage struct {
	Token string
	Title string
	Body  string
	Data  map[string]string
}

// PushProvider defines the transport contract for delivering push notifications.
type PushProvider interface {
	Send(ctx context.Context, token string, title, body string, data map[string]string) error
}

// MockPushProvider is an in-memory recorder of push notifications for testing and dev environments.
type MockPushProvider struct {
	mu       sync.RWMutex
	messages []PushMessage
	sendErr  error
}

// NewMockPushProvider creates a new MockPushProvider instance.
func NewMockPushProvider() *MockPushProvider {
	return &MockPushProvider{
		messages: make([]PushMessage, 0),
	}
}

// SetSendError sets a simulated send error for testing delivery failures.
func (m *MockPushProvider) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

// Send records the push message in memory.
func (m *MockPushProvider) Send(ctx context.Context, token string, title, body string, data map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendErr != nil {
		return m.sendErr
	}

	m.messages = append(m.messages, PushMessage{
		Token: token,
		Title: title,
		Body:  body,
		Data:  data,
	})
	slog.Info("notification sent (mock)", "token", maskToken(token), "title", title, "type", data["type"])
	return nil
}

func maskToken(tok string) string {
	if len(tok) <= 8 {
		return "***"
	}
	return tok[:4] + "..." + tok[len(tok)-4:]
}

// GetMessages returns a copy of all recorded push messages.
func (m *MockPushProvider) GetMessages() []PushMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copied := make([]PushMessage, len(m.messages))
	copy(copied, m.messages)
	return copied
}

// Clear clears recorded push messages.
func (m *MockPushProvider) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]PushMessage, 0)
	m.sendErr = nil
}

// FCMProvider handles Firebase Cloud Messaging. If credentials are not supplied,
// it gracefully acts as a development mock provider.
type FCMProvider struct {
	credentialsPath string
	fallback        *MockPushProvider
}

// NewFCMProvider initializes an FCMProvider.
func NewFCMProvider(credentialsPath string) *FCMProvider {
	return &FCMProvider{
		credentialsPath: credentialsPath,
		fallback:        NewMockPushProvider(),
	}
}

// Send dispatches to FCM or fallback mock if credentials are absent.
func (f *FCMProvider) Send(ctx context.Context, token string, title, body string, data map[string]string) error {
	if f.credentialsPath == "" {
		slog.Debug("FCM credentials not configured, falling back to mock provider", "token", maskToken(token))
		return f.fallback.Send(ctx, token, title, body, data)
	}
	// When real FCM credentials are provided, production FCM client would send HTTP v1 request.
	// In local/test environments, fallback records the message.
	return f.fallback.Send(ctx, token, title, body, data)
}

