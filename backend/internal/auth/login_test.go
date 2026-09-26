package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/devseed"
	"social-finance/internal/testutil"
)

func setupTestDBWithSeed(t *testing.T) *auth.Handler {
	t.Helper()
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Seed the canonical dev data into test database
	if _, err := devseed.Run(ctx, pool, "test"); err != nil {
		t.Fatalf("devseed run failed: %v", err)
	}

	auth.SigningSecret = []byte("test-signing-secret-at-least-16-chars")
	auth.DefaultAccessTokenLifetime = 15 * time.Minute

	svc := auth.NewService(auth.ServiceOptions{
		RefreshLifetime: 7 * 24 * time.Hour,
	})

	return auth.NewHandler(pool, svc)
}

func TestLogin_EmailAndPhoneVariants(t *testing.T) {
	handler := setupTestDBWithSeed(t)

	tests := []struct {
		name       string
		body       map[string]string
		wantStatus int
		wantEmail  string
		wantRole   string
	}{
		{
			name: "Pengurus RT 03 login by email",
			body: map[string]string{
				"email":    "pengurus.rt03@example.com",
				"password": devseed.DefaultSeedPassword,
			},
			wantStatus: http.StatusOK,
			wantEmail:  "pengurus.rt03@example.com",
			wantRole:   "pengurus",
		},
		{
			name: "Pengurus RT 03 login by canonical phone (+62)",
			body: map[string]string{
				"phone":    "+6281300000003",
				"password": devseed.DefaultSeedPassword,
			},
			wantStatus: http.StatusOK,
			wantEmail:  "pengurus.rt03@example.com",
			wantRole:   "pengurus",
		},
		{
			name: "Pengurus RT 03 login by local phone (08...)",
			body: map[string]string{
				"phone":    "081300000003",
				"password": devseed.DefaultSeedPassword,
			},
			wantStatus: http.StatusOK,
			wantEmail:  "pengurus.rt03@example.com",
			wantRole:   "pengurus",
		},
		{
			name: "Pengurus RT 03 login by identifier field",
			body: map[string]string{
				"identifier": "081300000003",
				"password":   devseed.DefaultSeedPassword,
			},
			wantStatus: http.StatusOK,
			wantEmail:  "pengurus.rt03@example.com",
			wantRole:   "pengurus",
		},
		{
			name: "Warga RT 04 login by email",
			body: map[string]string{
				"email":    "warga.rt04@example.com",
				"password": devseed.DefaultSeedPassword,
			},
			wantStatus: http.StatusOK,
			wantEmail:  "warga.rt04@example.com",
			wantRole:   "warga",
		},
		{
			name: "Warga RT 04 login by local phone",
			body: map[string]string{
				"phone":    "081400000040",
				"password": devseed.DefaultSeedPassword,
			},
			wantStatus: http.StatusOK,
			wantEmail:  "warga.rt04@example.com",
			wantRole:   "warga",
		},
		{
			name: "Invalid password rejected",
			body: map[string]string{
				"email":    "pengurus.rt03@example.com",
				"password": "WrongPassword123!",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "Non-existent user rejected",
			body: map[string]string{
				"email":    "nonexistent@example.com",
				"password": devseed.DefaultSeedPassword,
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login().ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d. Body: %s", tc.wantStatus, w.Code, w.Body.String())
			}

			if tc.wantStatus == http.StatusOK {
				var resp struct {
					User struct {
						Email string `json:"email"`
						Role  string `json:"role"`
					} `json:"user"`
					AccessToken  string `json:"access_token"`
					RefreshToken string `json:"refresh_token"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if resp.User.Email != tc.wantEmail {
					t.Errorf("expected email %q, got %q", tc.wantEmail, resp.User.Email)
				}
				if resp.User.Role != tc.wantRole {
					t.Errorf("expected role %q, got %q", tc.wantRole, resp.User.Role)
				}
				if resp.AccessToken == "" {
					t.Error("expected non-empty access_token")
				}
				if resp.RefreshToken == "" {
					t.Error("expected non-empty refresh_token")
				}
			}
		})
	}
}
