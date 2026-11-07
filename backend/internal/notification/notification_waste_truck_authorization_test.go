package notification_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"

	"github.com/google/uuid"
)

// TestManualPush_WasteTruck_KeamananAuthorization verifies the waste truck
// arrival authorization rules for security officers (keamanan):
//   - keamanan holding the dedicated notification.send_waste_truck permission
//     may trigger the waste truck arrival notification (202).
//   - keamanan without that permission is rejected (403).
func TestManualPush_WasteTruck_KeamananAuthorization(t *testing.T) {
	pool := testutil.GetTestPool(t)
	db := pool.Raw()
	ctx := context.Background()
	rtID := "00000000-0000-0000-0000-000000000002"

	// Ensure the dedicated permission exists.
	var permissionTypeExisted bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM permission_types WHERE code = 'notification.send_waste_truck')`,
	).Scan(&permissionTypeExisted); err != nil {
		t.Fatalf("check permission type: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO permission_types (code, domain, operation)
		VALUES ('notification.send_waste_truck', 'notification', 'send_waste_truck')
		ON CONFLICT (code) DO NOTHING`,
	); err != nil {
		t.Fatalf("insert permission_types: %v", err)
	}
	var permID string
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM permission_types WHERE code = 'notification.send_waste_truck'`,
	).Scan(&permID); err != nil {
		t.Fatalf("get permission id: %v", err)
	}
	var permissionGrantExisted bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM position_permissions WHERE position = 'keamanan' AND permission_id = $1)`,
		permID,
	).Scan(&permissionGrantExisted); err != nil {
		t.Fatalf("check permission grant: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO position_permissions (position, permission_id)
		VALUES ('keamanan', $1)
		ON CONFLICT (position, permission_id) DO NOTHING`,
		permID,
	); err != nil {
		t.Fatalf("insert position_permissions: %v", err)
	}
	t.Cleanup(func() {
		if permissionGrantExisted {
			_, _ = db.ExecContext(ctx,
				`INSERT INTO position_permissions (position, permission_id) VALUES ('keamanan', $1) ON CONFLICT DO NOTHING`,
				permID,
			)
		} else {
			_, _ = db.ExecContext(ctx,
				`DELETE FROM position_permissions WHERE position = 'keamanan' AND permission_id = $1`,
				permID,
			)
		}
		if !permissionTypeExisted {
			_, _ = db.ExecContext(ctx, `DELETE FROM permission_types WHERE id = $1`, permID)
		}
	})

	repo := newMockManagementRepo()
	svc := notification.NewService(repo, nil)
	router := setupManagementRouter(repo, svc, pool)

	keamananUID := uuid.New().String()
	membershipID := uuid.New().String()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, email, phone, password_hash, full_name, is_active)
		VALUES ($1, $2, '08123456789', 'dummyhash', $3, true)
		ON CONFLICT (id) DO UPDATE SET is_active = true`,
		keamananUID, fmt.Sprintf("keamanan-wt-%d@test.com", time.Now().UnixNano()), "Test Keamanan WT",
	); err != nil {
		t.Fatalf("create keamanan user: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, 'warga', 'keamanan', true)
		ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, jabatan = EXCLUDED.jabatan, is_active = true`,
		membershipID, keamananUID, rtID,
	); err != nil {
		t.Fatalf("create keamanan membership: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, membershipID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, keamananUID)
	})

	sendWasteTruck := func(ac *auth.AuthContext) *httptest.ResponseRecorder {
		body, _ := json.Marshal(notification.WasteTruckNotificationInput{
			Message:  "Mobil pengangkut sampah sudah tiba.",
			ImageURL: nil,
		})
		req := httptest.NewRequest("POST", "/api/v1/notifications/waste-truck", bytes.NewBufferString(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req = withAuth(req, ac)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	getWasteTruckTemplate := func(ac *auth.AuthContext) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/waste-truck/template", nil)
		req = withAuth(req, ac)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	getWasteTruckHistory := func(ac *auth.AuthContext) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/waste-truck/history", nil)
		req = withAuth(req, ac)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	ac := &auth.AuthContext{
		UserID:        keamananUID,
		MembershipID:  membershipID,
		RTID:          rtID,
		TenantRole:    auth.RoleWarga,
		Jabatan:       auth.JabatanKeamanan,
		RWMemberships: []auth.RWMembership{{RWID: "rw-002", Jabatan: "keamanan"}},
	}

	// Security officer with the dedicated permission may trigger waste truck.
	rec := sendWasteTruck(ac)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for keamanan with notification.send_waste_truck, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = getWasteTruckTemplate(ac)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected permitted template lookup to reach RT-scoped template query (404 when unset), got %d: %s", rec.Code, rec.Body.String())
	}
	rec = getWasteTruckHistory(ac)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected permitted security officer history lookup, got %d: %s", rec.Code, rec.Body.String())
	}

	otherOfficer := *ac
	otherOfficer.UserID = uuid.NewString()
	otherOfficer.MembershipID = ""
	otherOfficer.Jabatan = ""
	otherOfficer.RWMemberships = nil
	rec = getWasteTruckHistory(&otherOfficer)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected user without an active security membership to be forbidden, got %d: %s", rec.Code, rec.Body.String())
	}
	warga := *ac
	warga.Jabatan = auth.Jabatan("")
	rec = getWasteTruckTemplate(&warga)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected non-security user to be forbidden from template read, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = getWasteTruckHistory(&warga)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected non-security user to be forbidden from dispatch history, got %d: %s", rec.Code, rec.Body.String())
	}

	superAdmin := &auth.AuthContext{
		UserID:     uuid.NewString(),
		RTID:       rtID,
		SystemRole: auth.SystemRoleSuperAdmin,
	}
	rec = getWasteTruckTemplate(superAdmin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected super admin template lookup to use the selected RT context, got %d: %s", rec.Code, rec.Body.String())
	}

	uploadImage := func(eventType string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("event_type", eventType); err != nil {
			t.Fatalf("write event_type field: %v", err)
		}
		file, err := writer.CreateFormFile("file", "waste-truck.png")
		if err != nil {
			t.Fatalf("create multipart image field: %v", err)
		}
		if _, err := file.Write([]byte("\x89PNG\r\n\x1a\n")); err != nil {
			t.Fatalf("write multipart image: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close multipart form: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/waste-truck/upload-image", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = withAuth(req, ac)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	rec = uploadImage(notification.EventWasteTruckArrival)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for permitted waste_truck_arrival image upload, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = uploadImage(notification.EventManualPush)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for keamanan uploading a manual_push image, got %d: %s", rec.Code, rec.Body.String())
	}

	// Revoke the position permission: reading remains allowed, dispatch stays denied.
	if _, err := db.ExecContext(ctx, `DELETE FROM position_permissions WHERE position = 'keamanan' AND permission_id = $1`, permID); err != nil {
		t.Fatalf("delete position_permissions: %v", err)
	}
	rec = sendWasteTruck(ac)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for keamanan without notification.send_waste_truck, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = getWasteTruckTemplate(ac)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected template read access without dispatch permission to reach RT-scoped template query (404 when unset), got %d: %s", rec.Code, rec.Body.String())
	}
	rec = getWasteTruckHistory(ac)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected history read access without dispatch permission, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = uploadImage(notification.EventWasteTruckArrival)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for keamanan without permission uploading an image, got %d: %s", rec.Code, rec.Body.String())
	}

}
