package http

import (
	"encoding/json"
	"net/http"
)

// APIError represents a standardized API error response body.
type APIError struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []FieldDetail `json:"details,omitempty"`
}

// FieldDetail represents a field-level validation error.
type FieldDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// APIErrorEnvelope wraps APIError to conform to { "error": { "code": "...", "message": "..." } }.
type APIErrorEnvelope struct {
	Error APIError `json:"error"`
}

// WriteError writes a structured JSON error response.
func WriteError(w http.ResponseWriter, statusCode int, code, message string) {
	WriteErrorWithDetails(w, statusCode, code, message, nil)
}

// WriteErrorWithDetails writes a structured JSON error response with optional field details.
func WriteErrorWithDetails(w http.ResponseWriter, statusCode int, code, message string, details []FieldDetail) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIErrorEnvelope{
		Error: APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// WriteValidationError writes a 400 validation error, optionally with field details.
func WriteValidationError(w http.ResponseWriter, message string, details []FieldDetail) {
	WriteErrorWithDetails(w, http.StatusBadRequest, "validation_error", message, details)
}

// WriteUnauthorized writes a 401 unauthorized error.
func WriteUnauthorized(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusUnauthorized, "unauthorized", message)
}

// WriteForbidden writes a 403 forbidden error.
func WriteForbidden(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusForbidden, "forbidden", message)
}

// WriteNotFound writes a 404 not found error.
func WriteNotFound(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusNotFound, "not_found", message)
}

// WriteConflict writes a 409 conflict error.
func WriteConflict(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusConflict, "conflict", message)
}

// WriteRateLimited writes a 429 rate limited error.
func WriteRateLimited(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusTooManyRequests, "rate_limited", message)
}

// WriteInternalServerError writes a 500 internal error.
func WriteInternalServerError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusInternalServerError, "internal_error", message)
}
