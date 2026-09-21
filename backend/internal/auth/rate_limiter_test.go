package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Login rate limiter: 10 attempts per 5 minutes per IP.
// ---------------------------------------------------------------------------

func TestLoginRateLimiterAllowBelowThreshold(t *testing.T) {
	limiter := LoginRateLimiter()
	if limiter.maxReq != 10 {
		t.Errorf("expected 10 max requests, got %d", limiter.maxReq)
	}
	if limiter.window != 5*time.Minute {
		t.Errorf("expected 5min window, got %v", limiter.window)
	}

	// First 10 requests must be allowed.
	for i := 0; i < 10; i++ {
		if !limiter.Allow("test-ip") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestLoginRateLimiterBlocksAfterThreshold(t *testing.T) {
	limiter := LoginRateLimiter()

	// Exhaust the window.
	for i := 0; i < 10; i++ {
		limiter.Allow("test-ip")
	}
	// 11th request must be blocked.
	if limiter.Allow("test-ip") {
		t.Error("11th request should be blocked")
	}
	if limiter.Allow("test-ip") {
		t.Error("12th request should also be blocked")
	}
}

func TestLoginRateLimiterHTTPHandler(t *testing.T) {
	limiter := LoginRateLimiter()

	callCount := 0
	handler := limiter.LimitFunc(func(r *http.Request) string {
		return "test-ip"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	// Send 10 requests (all allowed).
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	if callCount != 10 {
		t.Errorf("expected 10 handler calls, got %d", callCount)
	}

	// 11th request: should get 429.
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for 11th request, got %d", rec.Code)
	}
	if callCount != 10 {
		t.Errorf("expected 10 handler calls (11th blocked), got %d", callCount)
	}

	// Verify the 429 response uses the standard API error format.
	body, _ := io.ReadAll(rec.Body)
	var resp struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("429 response body is not valid JSON: %s", string(body))
	}
	if resp.Error["code"] != "rate_limited" {
		t.Errorf("expected error.code=rate_limited, got %v", resp.Error["code"])
	}
}

func TestLoginRateLimiterDifferentIPsIndependent(t *testing.T) {
	limiter := LoginRateLimiter()

	// Exhaust IP-A.
	for i := 0; i < 10; i++ {
		limiter.Allow("ip-a")
	}
	// IP-B should still be allowed.
	if !limiter.Allow("ip-b") {
		t.Error("request from different IP should still be allowed")
	}
}

func TestLoginRateLimiterWindowExpires(t *testing.T) {
	// Use a short window for testing.
	limiter := NewRateLimiter(2, 100*time.Millisecond)

	if !limiter.Allow("test") {
		t.Error("1st request should be allowed")
	}
	if !limiter.Allow("test") {
		t.Error("2nd request should be allowed")
	}
	if limiter.Allow("test") {
		t.Error("3rd request should be blocked")
	}

	// Wait for window to expire.
	time.Sleep(150 * time.Millisecond)

	if !limiter.Allow("test") {
		t.Error("request after window expiry should be allowed")
	}
}

// ---------------------------------------------------------------------------
// Refresh rate limiter: 30 attempts per 15 minutes per IP.
// ---------------------------------------------------------------------------

func TestRefreshRateLimiterAllowBelowThreshold(t *testing.T) {
	limiter := RefreshRateLimiter()
	if limiter.maxReq != 30 {
		t.Errorf("expected 30 max requests, got %d", limiter.maxReq)
	}
	if limiter.window != 15*time.Minute {
		t.Errorf("expected 15min window, got %v", limiter.window)
	}

	for i := 0; i < 30; i++ {
		if !limiter.Allow("test-ip") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestRefreshRateLimiterBlocksAfterThreshold(t *testing.T) {
	limiter := RefreshRateLimiter()

	for i := 0; i < 30; i++ {
		limiter.Allow("test-ip")
	}
	if limiter.Allow("test-ip") {
		t.Error("31st request should be blocked")
	}
}

func TestRefreshRateLimiterHTTPHandler(t *testing.T) {
	limiter := RefreshRateLimiter()

	callCount := 0
	handler := limiter.LimitFunc(func(r *http.Request) string {
		return "test-ip"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 30; i++ {
		req, err := http.NewRequest("POST", "/api/v1/auth/refresh", nil)
		if err != nil {
			t.Fatalf("create request %d: %v", i+1, err)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	if callCount != 30 {
		t.Errorf("expected 30 handler calls, got %d", callCount)
	}

	// 31st request: should get 429.
	req, err := http.NewRequest("POST", "/api/v1/auth/refresh", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for 31st request, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	var resp struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("429 response body is not valid JSON: %s", string(body))
	}
	if resp.Error["code"] != "rate_limited" {
		t.Errorf("expected error.code=rate_limited, got %v", resp.Error["code"])
	}
}

// ---------------------------------------------------------------------------
// RemoteAddr vs X-Forwarded-For: verify the limiter uses RemoteAddr.
// ---------------------------------------------------------------------------

func TestLimiterUsesRemoteAddrNotXForwardedFor(t *testing.T) {
	limiter := LoginRateLimiter()

	// Exhaust with a spoofed XFF. The key function strips the port from
	// RemoteAddr to produce a stable per-client key.
	handler := limiter.LimitFunc(func(r *http.Request) string {
		// Production code: auth.ClientIP(r) = stripPort(r.RemoteAddr)
		addr := r.RemoteAddr
		for i := len(addr) - 1; i >= 0; i-- {
			if addr[i] == ':' {
				return addr[:i]
			}
		}
		return addr
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Simulate the first 9 requests with XFF spoofed.
	for i := 0; i < 9; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.Header.Set("X-Forwarded-For", "1.2.3.4") // spoofed
		req.RemoteAddr = "10.0.0.1:12345"            // real remote
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d (XFF should be ignored)", i+1, rec.Code)
		}
	}

	// 10th request: should still be allowed (only 9 from this IP so far).
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("10th request: expected 200, got %d — XFF should not affect the rate limit key", rec.Code)
	}

	// 11th request: should be blocked (10 from that RemoteAddr).
	req = httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "5.6.7.8") // different spoofed IP
	req.RemoteAddr = "10.0.0.1:12345"            // same real IP
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("11th request: expected 429 (blocked by RemoteAddr), got %d", rec.Code)
	}
}

func TestRateLimiterEmptyKeyPassthrough(t *testing.T) {
	limiter := LoginRateLimiter()

	callCount := 0
	handler := limiter.LimitFunc(func(r *http.Request) string {
		return "" // empty key → always allowed
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200 with empty key, got %d", i+1, rec.Code)
		}
	}
	if callCount != 5 {
		t.Errorf("expected 5 handler calls, got %d", callCount)
	}
}

func TestRateLimiterAllowReturnsTrueForFirstRequest(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute)

	// First request for a new key MUST be allowed (sets count=1).
	if !limiter.Allow("new-key") {
		t.Error("first request for a new key must be allowed")
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute)
	done := make(chan bool)

	// Fire 20 goroutines hitting the same key.
	for i := 0; i < 20; i++ {
		go func() {
			done <- limiter.Allow("concurrent-key")
		}()
	}

	allowed := 0
	rejected := 0
	for i := 0; i < 20; i++ {
		if <-done {
			allowed++
		} else {
			rejected++
		}
	}

	if allowed != 5 {
		t.Errorf("expected exactly 5 allowed, got %d (concurrent: %d rejected)", allowed, rejected)
	}
}

// ---------------------------------------------------------------------------
// Helper: parse body status code from handler response.
// ---------------------------------------------------------------------------

func parseStatusCode(t *testing.T, body []byte) int {
	t.Helper()
	var resp struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("body %s is not valid JSON", string(body))
	}
	return int(resp.Error["code"].(float64))
}

func parseErrorBodyCode(t *testing.T, body []byte) string {
	t.Helper()
	var resp struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("body %s is not valid JSON", string(body))
	}
	code, ok := resp.Error["code"].(string)
	if !ok {
		t.Fatalf("expected string error.code in body, got %T", resp.Error["code"])
	}
	return code
}

func bodyString(rec *httptest.ResponseRecorder) string {
	all, _ := io.ReadAll(rec.Body)
	return strconv.Quote(string(all))
}
