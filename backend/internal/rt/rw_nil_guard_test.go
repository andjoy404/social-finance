package rt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	chi "github.com/go-chi/chi/v5"
	"social-finance/internal/auth"
)

func TestRWJabatanNilAuthGuard(t *testing.T) {
	// Build a router with RW membership routes (no DB pool needed for nil guard)
	r := chi.NewRouter()
	r.Put("/api/v1/rws/{id}/memberships/{userId}/jabatan", NewHandler(nil, nil).SetRWJabatan)
	r.Delete("/api/v1/rws/{id}/memberships/{userId}/jabatan", NewHandler(nil, nil).RemoveRWJabatan)
	r.Get("/api/v1/rws/{id}/memberships", NewHandler(nil, nil).ListRWMemberships)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{"PUT", "/api/v1/rws/test-rw/memberships/test-user/jabatan", `{"jabatan":"ketua","rt_id":"test-rt"}`},
		{"DELETE", "/api/v1/rws/test-rw/memberships/test-user/jabatan", ""},
		{"GET", "/api/v1/rws/test-rw/memberships", ""},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			// No auth context in request (simulates missing auth)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRWJabatanValidActor(t *testing.T) {
	// This test verifies that with a valid auth context, the handler proceeds
	// past the nil guard. Since it requires a DB pool, it's marked as
	// needing integration test infrastructure.
	t.Skip("Requires database pool - integration test")
}

var _ = json.Marshal
var _ = auth.GetAuthContext
