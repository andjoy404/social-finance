package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequestID(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	mw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", w.Code, http.StatusOK)
	}

	id := w.Header().Get("X-Request-Id")
	if id == "" {
		t.Error("X-Request-Id header is empty")
	}
}

func TestPanicRecoveryMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	mw := PanicRecovery(handler)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	mw.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := Logging(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	mw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", w.Code, http.StatusOK)
	}
}
