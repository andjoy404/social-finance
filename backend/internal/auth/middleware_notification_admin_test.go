package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

// setupNotificationAdminTestPool returns a test database pool seeded with:
//   - One RT
//   - Memberships: ketua, wakil_ketua, sekretaris
//   - Permission "notification.manage" granted to position "ketua" only
//
// Returns pool, rtID, ketuaUserID, ketuaMembershipID, wakilMembershipID, sekretarisMembershipID.
func setupNotificationAdminTestPool(t *testing.T) (*database.Pool, string, string, string, string, string) {
	t.Helper()
	pool := testutil.GetTestPool(t)
	db := pool.Raw()
	AuthDBPool = pool

	// Create RT
	var rtID string
	counter := atomic.AddInt64(&rtCounter, 1)
	timestamp := time.Now().UnixNano()
	uniqueRT := fmt.Sprintf("001_%d_%d", counter, timestamp)
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO rts (name, rw, rt, address, head_name, is_active)
		 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
		"Test RT for Notification Admin", 1, uniqueRT, "Test Address", "Test Head",
	).Scan(&rtID)
	if err != nil {
		t.Fatalf("create test RT: %v", err)
	}

	// Insert permission "notification.manage" if not present
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO permission_types (code, domain, operation)
		 VALUES ('notification.manage', 'notification', 'manage')
		 ON CONFLICT (code) DO NOTHING`,
	)
	if err != nil {
		t.Fatalf("insert permission_types: %v", err)
	}

	// Get permission id
	var permissionID string
	err = db.QueryRowContext(context.Background(),
		`SELECT id FROM permission_types WHERE code = 'notification.manage'`,
	).Scan(&permissionID)
	if err != nil {
		t.Fatalf("get permission id: %v", err)
	}

	// Grant permission to position "ketua"
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO position_permissions (position, permission_id)
		 VALUES ($1, $2)
		 ON CONFLICT (position, permission_id) DO NOTHING`,
		"ketua", permissionID,
	)
	if err != nil {
		t.Fatalf("insert position_permissions: %v", err)
	}

	ctx := context.Background()

	// Helper to create a user + membership with a given jabatan.
	// Returns the real userID and real membershipID.
	createMember := func(userIDLabel, membershipIDLabel, jabatan string) (string, string) {
		realUserID := uuid.Must(uuid.NewUUID()).String()
		realMID := uuid.Must(uuid.NewUUID()).String()
		email := fmt.Sprintf("%s-%s-%s@test.com", userIDLabel, membershipIDLabel, realUserID[:8])
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
			 VALUES ($1, $2, '08123456789', 'dummyhash', $3, true)
			 ON CONFLICT (id) DO UPDATE SET is_active = true`,
			realUserID, email, "Test User "+userIDLabel,
		)
		if err != nil {
			t.Fatalf("create test user %s: %v", userIDLabel, err)
		}
		_, err = db.ExecContext(ctx,
			`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			 VALUES ($1, $2, $3, $4, $5, true)
			 ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, is_active = true, jabatan = EXCLUDED.jabatan, rt_id = EXCLUDED.rt_id`,
			realMID, realUserID, rtID, "warga", jabatan,
		)
		if err != nil {
			t.Fatalf("create membership %s: %v", membershipIDLabel, err)
		}
		return realUserID, realMID
	}

	// Create memberships and capture their real UUIDs
	var ketuaMembershipID, wakilMembershipID, sekretarisMembershipID string
	var ketuaUserID string
	ketuaUserID, ketuaMembershipID = createMember("ketua-user", "ketua-membership", "ketua")
	_, wakilMembershipID = createMember("wakil-user", "wakil-membership", "wakil_ketua")
	_, sekretarisMembershipID = createMember("sekretaris-user", "sekretaris-membership", "sekretaris")

	return pool, rtID, ketuaUserID, ketuaMembershipID, wakilMembershipID, sekretarisMembershipID
}

// makeNotificationAdminToken creates a JWT access token for a tenant user.
func makeNotificationAdminToken(userID, membershipID, rtID string, role Role, jabatan Jabatan) string {
	SigningSecret = []byte("test-signing-secret-notification-admin")
	tokenStr, err := GenerateAccessToken(TokenClaims{
		UserID:    userID,
		MID:       membershipID,
		RTID:      rtID,
		Role:      string(role),
		Jabatan:   string(jabatan),
	})
	if err != nil {
		panic(fmt.Sprintf("GenerateAccessToken failed: %v", err))
	}
	return tokenStr
}

// TestRequireNotificationAdminSuperAdmin verifies that SUPER_ADMIN always passes.
func TestRequireNotificationAdminSuperAdmin(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	SigningSecret = []byte("test-signing-secret-notification-admin")

	userID := uuid.Must(uuid.NewUUID()).String()
	claims := TokenClaims{
		UserID:  userID,
		SysRole: string(SystemRoleSuperAdmin),
	}
	tokenStr, err := GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	var reached bool
	handler := RequireNotificationAdmin(pool)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("GET", "/api/v1/notifications/schedules", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req = requestWithAuthContext(req, &AuthContext{
		UserID:    userID,
		SystemRole: SystemRoleSuperAdmin,
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", rec.Code)
	}
	if !reached {
		t.Error("handler was not reached for super admin")
	}
}

// TestRequireNotificationAdminKetuaRT verifies that Ketua RT with notification.manage permission passes.
func TestRequireNotificationAdminKetuaRT(t *testing.T) {
	pool, rtID, ketuaUserID, ketuaMembershipID, _, _ := setupNotificationAdminTestPool(t)

	SigningSecret = []byte("test-signing-secret-notification-admin")

	tokenStr := makeNotificationAdminToken(
		ketuaUserID, ketuaMembershipID, rtID, RoleWarga, JabatanKetua,
	)

	var reached bool
	handler := RequireNotificationAdmin(pool)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("GET", "/api/v1/notifications/schedules", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req = requestWithAuthContext(req, &AuthContext{
		UserID:       ketuaUserID,
		MembershipID: ketuaMembershipID,
		RTID:         rtID,
		TenantRole:   RoleWarga,
		Jabatan:      JabatanKetua,
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for Ketua RT, got %d", rec.Code)
	}
	if !reached {
		t.Error("handler was not reached for Ketua RT")
	}
}

// TestRequireNotificationAdminJabatanWithoutPermission verifies that a jabatan
// without notification.manage permission gets 403.
func TestRequireNotificationAdminJabatanWithoutPermission(t *testing.T) {
	pool, rtID, _, _, wakilMembershipID, _ := setupNotificationAdminTestPool(t)

	SigningSecret = []byte("test-signing-secret-notification-admin")

	tokenStr := makeNotificationAdminToken(
		uuid.Must(uuid.NewUUID()).String(), wakilMembershipID, rtID, RoleWarga, JabatanWakilKetua,
	)

	var reached bool
	handler := RequireNotificationAdmin(pool)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("GET", "/api/v1/notifications/schedules", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req = requestWithAuthContext(req, &AuthContext{
		UserID:       uuid.Must(uuid.NewUUID()).String(),
		MembershipID: wakilMembershipID,
		RTID:        rtID,
		TenantRole:   RoleWarga,
		Jabatan:      JabatanWakilKetua,
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for jabatan without permission, got %d", rec.Code)
	}
	if reached {
		t.Error("handler was reached for jabatan without permission")
	}
}

// TestRequireNotificationAdminUserWithoutJabatan verifies that a user without
// a jabatan (e.g. plain warga) gets 403.
func TestRequireNotificationAdminUserWithoutJabatan(t *testing.T) {
	pool, rtID, _, _, _, _ := setupNotificationAdminTestPool(t)

	SigningSecret = []byte("test-signing-secret-notification-admin")

	// Create a plain warga user without jabatan
	var plainMembershipID string
	realUserID := uuid.Must(uuid.NewUUID()).String()
	db := pool.Raw()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, '08123456789', 'dummyhash', $3, true)
		 ON CONFLICT (id) DO UPDATE SET is_active = true`,
		realUserID, fmt.Sprintf("%s@test.com", realUserID), "Test Warga User",
	)
	if err != nil {
		t.Fatalf("create test warga user: %v", err)
	}
	err = db.QueryRowContext(context.Background(),
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
		 VALUES ($1, $2, $3, true) RETURNING id`,
		realUserID, rtID, "warga",
	).Scan(&plainMembershipID)
	if err != nil {
		t.Fatalf("create warga membership: %v", err)
	}

	tokenStr := makeNotificationAdminToken(
		realUserID, plainMembershipID, rtID, RoleWarga, "",
	)

	var reached bool
	handler := RequireNotificationAdmin(pool)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("GET", "/api/v1/notifications/schedules", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req = requestWithAuthContext(req, &AuthContext{
		UserID:       uuid.Must(uuid.NewUUID()).String(),
		MembershipID: plainMembershipID,
		RTID:        rtID,
		TenantRole:   RoleWarga,
		Jabatan:      "",
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for user without jabatan, got %d", rec.Code)
	}
	if reached {
		t.Error("handler was reached for user without jabatan")
	}
}

// TestRequireNotificationAdminMissingAuthContext verifies 401 when no auth context.
func TestRequireNotificationAdminMissingAuthContext(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()

	var reached bool
	handler := RequireNotificationAdmin(pool)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("GET", "/api/v1/notifications/schedules", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}
	if reached {
		t.Error("handler was reached without auth context")
	}
}

// requestWithAuthContext attaches an AuthContext to the request context.
func requestWithAuthContext(req *http.Request, ac *AuthContext) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), AuthContextKey, ac))
}

// ensure test util references are used
var _ = sql.ErrNoRows
var _ = database.Pool{}
