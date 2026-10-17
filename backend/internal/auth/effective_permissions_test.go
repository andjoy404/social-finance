package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/devseed"
	"social-finance/internal/testutil"
)

func setupEffectivePermissionTest(t *testing.T) (*auth.Handler, *database.Pool) {
	t.Helper()
	handler := setupTestDBWithSeed(t)
	pool := testutil.GetTestPool(t)
	if _, err := devseed.Run(context.Background(), pool, "test"); err != nil {
		t.Fatalf("ensure canonical permission seed: %v", err)
	}

	previousPool := auth.AuthDBPool
	auth.AuthDBPool = pool
	t.Cleanup(func() { auth.AuthDBPool = previousPool })
	return handler, pool
}

func TestMeReturnsEffectiveFinancePermissions(t *testing.T) {
	handler, pool := setupEffectivePermissionTest(t)

	tests := []struct {
		email       string
		wantRole    string
		wantJabatan string
		want        []string
	}{
		{
			email:       "ketua-003@local.com",
			wantRole:    "pengurus",
			wantJabatan: "ketua",
			want:        []string{"finance.read", "finance.create", "finance.update", "finance.approve"},
		},
		{
			email:       "wakil-ketua-003@local.com",
			wantRole:    "pengurus",
			wantJabatan: "wakil_ketua",
			want:        []string{"finance.read"},
		},
		{
			email:    "warga.rt03@local.com",
			wantRole: "warga",
			want:     []string{"finance.read"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.email, func(t *testing.T) {
			loginBody, _ := json.Marshal(map[string]string{
				"email":    tc.email,
				"password": devseed.DefaultSeedPassword,
			})
			loginResponse := httptest.NewRecorder()
			handler.Login().ServeHTTP(loginResponse, httptest.NewRequest(
				http.MethodPost,
				"/api/v1/auth/login",
				bytes.NewReader(loginBody),
			))
			if loginResponse.Code != http.StatusOK {
				t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
			}

			var tokens struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.Unmarshal(loginResponse.Body.Bytes(), &tokens); err != nil {
				t.Fatalf("decode login: %v", err)
			}

			meResponse := httptest.NewRecorder()
			meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
			meRequest.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
			auth.RequireAuth(handler.Me()).ServeHTTP(meResponse, meRequest)
			if meResponse.Code != http.StatusOK {
				t.Fatalf("/auth/me status = %d, body = %s", meResponse.Code, meResponse.Body.String())
			}

			var profile struct {
				Email                string   `json:"email"`
				Role                 string   `json:"role"`
				Jabatan              *string  `json:"jabatan"`
				RTID                 string   `json:"rt_id"`
				EffectivePermissions []string `json:"effective_permissions"`
			}
			if err := json.Unmarshal(meResponse.Body.Bytes(), &profile); err != nil {
				t.Fatalf("decode /auth/me: %v", err)
			}
			if profile.Email != tc.email || profile.Role != tc.wantRole || profile.RTID == "" {
				t.Fatalf("legacy profile fields changed: %+v", profile)
			}
			if tc.wantJabatan != "" &&
				(profile.Jabatan == nil || *profile.Jabatan != tc.wantJabatan) {
				t.Fatalf("jabatan = %v, want %q", profile.Jabatan, tc.wantJabatan)
			}
			if !reflect.DeepEqual(profile.EffectivePermissions, tc.want) {
				t.Errorf("effective_permissions = %v, want %v", profile.EffectivePermissions, tc.want)
			}
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin transaction for super admin resolver test: %v", err)
	}
	defer tx.Rollback()
	got, err := auth.NewPermissionResolver(nil).HasPermission(
		ctx,
		tx,
		&auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin},
		"finance.update",
	)
	if err != nil || !got {
		t.Fatalf("super admin existing resolver bypass: allowed=%t, err=%v", got, err)
	}
}

func TestFinanceManagementPermissionMiddlewareRejectsReadOnlyPosition(t *testing.T) {
	_, pool := setupEffectivePermissionTest(t)

	tests := []struct {
		email       string
		wantStatus  int
		wantJabatan string
	}{
		{email: "ketua-003@local.com", wantStatus: http.StatusNoContent, wantJabatan: "ketua"},
		{email: "wakil-ketua-003@local.com", wantStatus: http.StatusForbidden, wantJabatan: "wakil_ketua"},
	}
	for _, tc := range tests {
		t.Run(tc.email, func(t *testing.T) {
			var ac auth.AuthContext
			err := pool.Raw().QueryRowContext(
				context.Background(),
				`SELECT u.id, urm.id, urm.rt_id, urm.role, urm.jabatan
				 FROM users u
				 JOIN user_rt_memberships urm ON urm.user_id = u.id AND urm.is_active = true
				 WHERE u.email = $1 AND urm.jabatan = $2`,
				tc.email,
				tc.wantJabatan,
			).Scan(&ac.UserID, &ac.MembershipID, &ac.RTID, &ac.TenantRole, &ac.Jabatan)
			if err != nil {
				t.Fatalf("load permission fixture: %v", err)
			}

			handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			handler = auth.RequireRole(auth.RoleBendahara, auth.RolePengurus)(handler)
			handler = auth.RequirePermission(pool, "finance.approve")(handler)
			handler = auth.RequirePermission(pool, "finance.update")(handler)
			handler = auth.RequirePermission(pool, "finance.create")(handler)

			request := httptest.NewRequest(http.MethodPost, "/api/v1/dues", nil)
			request = request.WithContext(context.WithValue(
				request.Context(),
				auth.AuthContextKey,
				&ac,
			))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.wantStatus, response.Body.String())
			}
		})
	}
}

func TestMeSystemSuperAdminUsesExistingPermissionBypass(t *testing.T) {
	handler, pool := setupEffectivePermissionTest(t)
	var userID string
	if err := pool.Raw().QueryRowContext(
		context.Background(),
		`INSERT INTO users (id, email, phone, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, NULL, 'dummy', 'Test Admin', 'super_admin', true)
		 RETURNING id`,
		fmt.Sprintf("effective-permission-system-admin-%d@local.com", time.Now().UnixNano()),
	).Scan(&userID); err != nil {
		t.Fatalf("insert system-only super admin fixture: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request = request.WithContext(context.WithValue(
		request.Context(),
		auth.AuthContextKey,
		&auth.AuthContext{
			UserID:     userID,
			SystemRole: auth.SystemRoleSuperAdmin,
		},
	))
	response := httptest.NewRecorder()
	handler.Me().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("/auth/me status = %d, body = %s", response.Code, response.Body.String())
	}

	var profile struct {
		EffectivePermissions []string `json:"effective_permissions"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode /auth/me response: %v", err)
	}
	want := []string{"finance.read", "finance.create", "finance.update", "finance.approve"}
	if !reflect.DeepEqual(profile.EffectivePermissions, want) {
		t.Fatalf("super admin effective_permissions = %v, want %v", profile.EffectivePermissions, want)
	}
}
