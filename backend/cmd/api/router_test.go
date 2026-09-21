package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"social-finance/internal/health"
)

func TestRouterHasHealthRoute(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/health", health.Handler(&healthTestPinger{}))

	// Healthy case
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthy: status = %d; want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	if body["database"] != "healthy" {
		t.Errorf("database = %q; want healthy", body["database"])
	}
}

func TestRouter404(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/health", health.Handler(&healthTestPinger{}))

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", w.Code)
	}
}

// healthTestPinger is a minimal Pinger that always returns nil.
type healthTestPinger struct{}

func (healthTestPinger) Ping(_ context.Context) error { return nil }
