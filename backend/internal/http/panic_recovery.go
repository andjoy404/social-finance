package http

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// PanicRecovery returns a middleware that recovers from panics, logs the
// stack trace using slog, and returns a structured 500 JSON error response.
func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
					"method", r.Method,
					"request_id", GetRequestID(r.Context()),
				)
				WriteInternalError(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// WriteInternalError writes a generic 500 JSON error without exposing internal details.
func WriteInternalError(w http.ResponseWriter) {
	WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}
