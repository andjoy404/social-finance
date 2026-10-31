package rw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Test handleError: ErrRWAccessDenied → HTTP 403 ---

func TestHandleErrorAccessDenied(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	h.handleError(rec, fmt.Errorf("GetDashboard: %w", ErrRWAccessDenied))

	if rec.Code != http.StatusForbidden {
		t.Errorf("handleError(ErrRWAccessDenied) status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("response body has no error object: %v", result)
	}

	if errObj["code"] != "forbidden" {
		t.Errorf("error.code = %q, want 'forbidden'", errObj["code"])
	}

	if errObj["message"] != "rw access denied" {
		t.Errorf("error.message = %q, want 'rw access denied'", errObj["message"])
	}
}

func TestHandleErrorAccessDeniedWrapped(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	wrapped := fmt.Errorf("service layer: %w", fmt.Errorf("GetDashboard: %w", ErrRWAccessDenied))
	h.handleError(rec, wrapped)

	if rec.Code != http.StatusForbidden {
		t.Errorf("handleError(wrapped ErrRWAccessDenied) status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// --- Test handleError: bad request → HTTP 400 ---

func TestHandleErrorBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	wrapped := fmt.Errorf("kas report: validate period: %w", ErrValidation)
	h.handleError(rec, wrapped)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("handleError(validation error) status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("response body has no error object: %v", result)
	}

	if errObj["code"] != "bad_request" {
		t.Errorf("error.code = %q, want 'bad_request'", errObj["code"])
	}
}

// --- Test handleError: internal error → HTTP 500, tanpa detail bocor ---

func TestHandleErrorInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	h.handleError(rec, fmt.Errorf("pq: unable to execute SELECT: no such column: nonexistent_column"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("handleError(internal) status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("response body has no error object: %v", result)
	}

	if errObj["code"] != "internal_error" {
		t.Errorf("error.code = %q, want 'internal_error'", errObj["code"])
	}

	if errObj["message"] != "internal server error" {
		t.Errorf("error.message = %q, want 'internal server error' (not leaking query details)", errObj["message"])
	}

	// Verify the raw body does NOT contain the query detail
	body := rec.Body.String()
	if len(body) > 0 && body != "" && contains(body, "nonexistent_column") {
		t.Errorf("internal error details leaked to client: %s", body)
	}
}

// --- Test handleError: generic error → HTTP 500 ---

func TestHandleErrorGeneric(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	h.handleError(rec, fmt.Errorf("something unexpected happened"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("handleError(generic) status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("response body has no error object: %v", result)
	}

	if errObj["code"] != "internal_error" {
		t.Errorf("error.code = %q, want 'internal_error'", errObj["code"])
	}
	if errObj["message"] != "internal server error" {
		t.Errorf("error.message = %q, want 'internal server error'", errObj["message"])
	}
}

// --- Test handleError: non-bad_request service error does NOT become 400 ---

func TestHandleErrorNonBadRequestNot400(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	h.handleError(rec, fmt.Errorf("database connection failed"))

	if rec.Code == http.StatusBadRequest {
		t.Errorf("handleError(database error) should NOT return 400, got %d", rec.Code)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("handleError(database error) status = %d, want 500", rec.Code)
	}
}

// --- Test handleError: validation error preserves original message ---

func TestHandleErrorValidationErrorPreservesMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	// Simulate the full chain: repo → service → handler
	repoErr := fmt.Errorf("validate period: %w", ErrValidation)
	svcErr := fmt.Errorf("kas report: %w", repoErr)
	h.handleError(rec, svcErr)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("handleError(deep-wrapped validation) status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("response body has no error object: %v", result)
	}

	// Message should contain the original validation detail (wrapped message)
	msg, ok := errObj["message"].(string)
	if !ok {
		t.Fatalf("error.message is not a string: %v", errObj["message"])
	}
	if msg != "kas report: validate period: validation error" {
		t.Errorf("error.message = %q, want %q", msg, "kas report: validate period: validation error")
	}
}

// --- Test handleError: date validation error also → 400 ---

func TestHandleErrorDateValidation(t *testing.T) {
	rec := httptest.NewRecorder()
	h := &Handler{}
	err := fmt.Errorf("get transactions: validate start_date: %w", ErrValidation)
	h.handleError(rec, err)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("handleError(date validation) status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	errObj, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("response body has no error object: %v", result)
	}

	if errObj["code"] != "bad_request" {
		t.Errorf("error.code = %q, want 'bad_request'", errObj["code"])
	}
}

// --- Helper for body content check ---

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
