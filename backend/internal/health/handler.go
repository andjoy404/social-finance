package health

import (
	"context"
	"encoding/json"
	"net/http"
)

// Pinger defines the minimal interface for checking database connectivity.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler returns an HTTP handler that checks application and database health.
func Handler(p Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		resp := map[string]string{
			"status": "ok",
		}

		if err := p.Ping(ctx); err != nil {
			resp["status"] = "unhealthy"
			resp["database"] = "unhealthy"
			writeJSON(w, http.StatusServiceUnavailable, resp)
			return
		}

		resp["database"] = "healthy"
		writeJSON(w, http.StatusOK, resp)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
