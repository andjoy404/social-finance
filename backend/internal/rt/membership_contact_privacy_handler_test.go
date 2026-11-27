package rt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"social-finance/internal/auth"
)

func TestMembershipContactPrivacy_HTTPJSONResponses(t *testing.T) {
	pool := testPool(t)
	router := buildRouter(pool)
	setupEnv()
	token := superAdminToken(t)
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	email := fmt.Sprintf("privacy-%s@example.test", suffix)
	phone := "+628123456789"
	var userID, rwID, rtID string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (email, phone, password_hash, full_name, system_role, is_active)
		VALUES ($1, $2, 'test-hash', $3, NULL, true) RETURNING id`,
		email, phone, "Privacy Test "+suffix).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rws (name, rw_number, is_active) VALUES ($1, $2, true) RETURNING id`,
		"Privacy RW "+suffix, 500+time.Now().Nanosecond()%400).Scan(&rwID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ($1, 500, $2, $3, true) RETURNING id`,
		"Privacy RT "+suffix, suffix, rwID).Scan(&rtID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, 'warga', 'anggota', true)`, userID, rtID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, rt_id, is_active)
		VALUES ($1, $2, 'pengurus', 'anggota', $3, true)`, userID, rwID, rtID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rws WHERE id = $1`, rwID)
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		pool.Close()
	})

	for _, tc := range []struct {
		name string
		path string
	}{
		{"RT memberships", "/api/v1/rts/" + rtID + "/memberships"},
		{"RW memberships retains account contacts", "/api/v1/rws/" + rwID + "/memberships"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET %s status=%d body=%s", tc.path, recorder.Code, recorder.Body.String())
			}
			var body struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Data) != 1 {
				t.Fatalf("response data length=%d, want 1: %s", len(body.Data), recorder.Body.String())
			}
			if tc.name == "RT memberships" {
				for _, field := range []string{"email", "phone"} {
					if _, exists := body.Data[0][field]; exists {
						t.Fatalf("Super Admin response leaked resident %s: %s", field, recorder.Body.String())
					}
				}
				return
			}
			if body.Data[0]["email"] != email || body.Data[0]["phone"] != phone {
				t.Fatalf("RW account contact fields were unexpectedly removed: %s", recorder.Body.String())
			}
		})
	}

	if canViewMembershipContacts(&auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, Jabatan: auth.JabatanKetua, RTID: rtID}, rtID) {
		t.Fatal("Super Admin must not receive resident contacts through Ketua claim")
	}
}
