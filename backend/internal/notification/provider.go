package notification

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// ServiceAccountCredentials represents a Google Cloud service account JSON structure.
type ServiceAccountCredentials struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// FCMProvider handles Firebase Cloud Messaging HTTP v1 API.
// In development/test environments, it safely falls back to MockPushProvider if credentials are not configured.
// In production, fallback to mock is strictly prohibited to prevent silently masking unconfigured notifications.
type FCMProvider struct {
	appEnv            string
	credentialsSource string
	projectID         string
	fallback          *MockPushProvider
	httpClient        *http.Client

	mu          sync.RWMutex
	sa          *ServiceAccountCredentials
	parsedKey   *rsa.PrivateKey
	accessToken string
	tokenExpiry time.Time
	initErr     error
}

// NewFCMProvider initializes an FCMProvider with credential path or raw JSON.
// An optional appEnv argument can be passed; defaults to os.Getenv("APP_ENV") or "development".
func NewFCMProvider(credentialsSource string, appEnv ...string) *FCMProvider {
	env := ""
	if len(appEnv) > 0 && appEnv[0] != "" {
		env = appEnv[0]
	} else {
		env = os.Getenv("APP_ENV")
	}
	if env == "" {
		env = "development"
	}

	p := &FCMProvider{
		appEnv:            env,
		credentialsSource: strings.TrimSpace(credentialsSource),
		fallback:          NewMockPushProvider(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	p.initCredentials()
	return p
}

// SetProjectID explicitly sets or overrides the target Firebase project ID.
func (f *FCMProvider) SetProjectID(projectID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := strings.TrimSpace(projectID)
	if p != "" {
		f.projectID = p
		if f.sa != nil {
			f.sa.ProjectID = p
		}
	}
}

// ProjectID returns the active Firebase project ID.
func (f *FCMProvider) ProjectID() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.projectID != "" {
		return f.projectID
	}
	if f.sa != nil && f.sa.ProjectID != "" {
		return f.sa.ProjectID
	}
	return "social-finance-andjoy"
}

// Fallback returns the underlying MockPushProvider for testing.
func (f *FCMProvider) Fallback() *MockPushProvider {
	return f.fallback
}

func (f *FCMProvider) initCredentials() {
	if f.credentialsSource == "" {
		if f.appEnv == "production" {
			f.initErr = errors.New("fcm: credentials are required in production environment")
			slog.Error("FCM credentials missing in production environment")
		} else {
			slog.Debug("FCM credentials not configured, operating in mock mode")
		}
		return
	}

	var rawBytes []byte
	if strings.HasPrefix(f.credentialsSource, "{") {
		rawBytes = []byte(f.credentialsSource)
	} else {
		data, err := os.ReadFile(f.credentialsSource)
		if err != nil {
			// Check standard container secret mount if default relative secrets path is used
			if strings.HasSuffix(f.credentialsSource, "secrets/fcm-service-account.json") {
				if containerData, cErr := os.ReadFile("/run/secrets/fcm-service-account.json"); cErr == nil {
					rawBytes = containerData
				} else {
					f.initErr = fmt.Errorf("read credentials file: %w", err)
					slog.Warn("failed to read FCM credentials file", "path", f.credentialsSource, "env", f.appEnv, "error", err)
					return
				}
			} else {
				f.initErr = fmt.Errorf("read credentials file: %w", err)
				slog.Warn("failed to read FCM credentials file", "path", f.credentialsSource, "env", f.appEnv, "error", err)
				return
			}
		} else {
			rawBytes = data
		}
	}

	var sa ServiceAccountCredentials
	if err := json.Unmarshal(rawBytes, &sa); err != nil {
		f.initErr = fmt.Errorf("unmarshal credentials json: %w", err)
		slog.Warn("failed to parse FCM credentials JSON", "env", f.appEnv, "error", err)
		return
	}

	if sa.ProjectID == "" && f.projectID != "" {
		sa.ProjectID = f.projectID
	}
	if sa.ProjectID == "" {
		sa.ProjectID = os.Getenv("FCM_PROJECT_ID")
	}

	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		f.initErr = errors.New("incomplete FCM service account credentials (missing project_id, client_email, or private_key)")
		slog.Warn("incomplete FCM service account credentials", "env", f.appEnv)
		return
	}

	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(sa.PrivateKey))
	if err != nil {
		f.initErr = fmt.Errorf("parse private key PEM: %w", err)
		slog.Warn("failed to parse FCM private key PEM", "env", f.appEnv, "error", err)
		return
	}

	f.sa = &sa
	f.parsedKey = parsedKey
	slog.Info("FCM service account credentials loaded successfully", "project_id", sa.ProjectID, "client_email", sa.ClientEmail)
}

func (f *FCMProvider) getAccessToken(ctx context.Context) (string, error) {
	f.mu.RLock()
	if f.accessToken != "" && time.Now().Before(f.tokenExpiry.Add(-2*time.Minute)) {
		token := f.accessToken
		f.mu.RUnlock()
		return token, nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check under write lock
	if f.accessToken != "" && time.Now().Before(f.tokenExpiry.Add(-2*time.Minute)) {
		return f.accessToken, nil
	}

	if f.sa == nil || f.parsedKey == nil {
		return "", errors.New("fcm: service account not initialized")
	}

	now := time.Now()
	tokenURI := f.sa.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}

	claims := jwt.MapClaims{
		"iss":   f.sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   tokenURI,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedJWT, err := token.SignedString(f.parsedKey)
	if err != nil {
		return "", fmt.Errorf("fcm: sign oauth jwt: %w", err)
	}

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {signedJWT},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("fcm: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fcm: request oauth token: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fcm: oauth token endpoint returned %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("fcm: parse token response: %w", err)
	}

	f.accessToken = tokenResp.AccessToken
	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 3600 * time.Second
	}
	f.tokenExpiry = now.Add(expiresIn)

	return f.accessToken, nil
}

// Send dispatches to FCM HTTP v1 or fallback mock if allowed.
// In production, fallback to mock is rejected to guarantee notifications are not falsely reported as delivered.
func (f *FCMProvider) Send(ctx context.Context, token string, title, body string, data map[string]string) error {
	f.mu.RLock()
	hasLiveCreds := f.sa != nil && f.parsedKey != nil
	initErr := f.initErr
	isProd := f.appEnv == "production"
	f.mu.RUnlock()

	if isProd {
		if initErr != nil {
			return fmt.Errorf("fcm: production configuration error: %w", initErr)
		}
		if !hasLiveCreds {
			return errors.New("fcm: production requires valid live credentials")
		}
	} else if !hasLiveCreds {
		slog.Debug("FCM credentials absent in non-production, dispatching via mock recorder", "token", maskToken(token))
		return f.fallback.Send(ctx, token, title, body, data)
	}

	accessToken, err := f.getAccessToken(ctx)
	if err != nil {
		if isProd {
			return fmt.Errorf("fcm: oauth token retrieval failed: %w", err)
		}
		slog.Warn("failed to obtain FCM oauth token, falling back to mock send", "error", err)
		return f.fallback.Send(ctx, token, title, body, data)
	}

	projectID := f.sa.ProjectID
	if f.projectID != "" {
		projectID = f.projectID
	}
	if projectID == "" {
		projectID = "social-finance-andjoy"
	}
	destURL := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)

	notifBlock := map[string]string{
		"title": title,
		"body":  body,
	}
	androidNotifBlock := map[string]any{
		"channel_id":            "social_finance_notifications",
		"sound":                 "default",
		"notification_priority": "PRIORITY_MAX",
	}
	if rawImg := strings.TrimSpace(data["image_url"]); rawImg != "" {
		resolvedImg := rawImg
		if strings.HasPrefix(resolvedImg, "/") {
			pubBase := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
			if pubBase == "" {
				pubBase = strings.TrimSpace(os.Getenv("API_PUBLIC_URL"))
			}
			if pubBase != "" {
				resolvedImg = strings.TrimRight(pubBase, "/") + resolvedImg
			}
		}
		if (strings.HasPrefix(resolvedImg, "http://") || strings.HasPrefix(resolvedImg, "https://")) && !strings.Contains(resolvedImg, "..") {
			notifBlock["image"] = resolvedImg
			androidNotifBlock["image"] = resolvedImg
		}
	}

	messagePayload := map[string]any{
		"message": map[string]any{
			"token":        token,
			"notification": notifBlock,
			"data":         data,
			"android": map[string]any{
				"priority":     "HIGH",
				"notification": androidNotifBlock,
			},
		},
	}

	payloadBytes, err := json.Marshal(messagePayload)
	if err != nil {
		return fmt.Errorf("fcm: marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("fcm: create push request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fcm: dispatch network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		slog.Info("notification sent (fcm v1)", "token", maskToken(token), "title", title)
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	// Check for unregistered device token
	if resp.StatusCode == http.StatusNotFound || strings.Contains(respStr, "UNREGISTERED") {
		return ErrTokenUnregistered
	}

	// Only return ErrTokenInvalid if the error detail specifically targets the device registration token.
	// General payload validation errors (e.g. malformed data key) must NOT deactivate user tokens.
	if resp.StatusCode == http.StatusBadRequest {
		lowerResp := strings.ToLower(respStr)
		if strings.Contains(lowerResp, "registration token") ||
			(strings.Contains(respStr, "INVALID_ARGUMENT") && strings.Contains(lowerResp, "token")) {
			return ErrTokenInvalid
		}
		return fmt.Errorf("fcm: bad request payload: %s", respStr)
	}

	return fmt.Errorf("fcm: gateway responded with status %d", resp.StatusCode)
}
