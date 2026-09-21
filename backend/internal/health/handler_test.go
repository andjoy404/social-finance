package health_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-finance/internal/health"
)

// mockPool implements health.Pinger for testing.
type mockPool struct {
	err error
}

func (m *mockPool) Ping(_ context.Context) error {
	return m.err
}

func TestHealthHandlerHealthy(t *testing.T) {
	pool := &mockPool{}
	h := health.Handler(pool)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	if body["database"] != "healthy" {
		t.Errorf("database = %q; want healthy", body["database"])
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q; want ok", body["status"])
	}
}

func TestHealthHandlerUnhealthy(t *testing.T) {
	pool := &mockPool{err: context.DeadlineExceeded}
	h := health.Handler(pool)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d; want %d", w.Code, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	if body["database"] != "unhealthy" {
		t.Errorf("database = %q; want unhealthy", body["database"])
	}
	if body["status"] != "unhealthy" {
		t.Errorf("status = %q; want unhealthy", body["status"])
	}
}
