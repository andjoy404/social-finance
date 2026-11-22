package profilephoto

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"social-finance/internal/auth"
	"social-finance/internal/storage"
	"social-finance/internal/testutil"
)

func makeTinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 80, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestProfilePhoto_ScopeConsistency_SuperAdminAndTenant(t *testing.T) {
	pool := testutil.GetTestPool(t)
	db := pool.Raw()
	ctx := context.Background()

	runID := time.Now().UnixNano()
	rwNumIn := int(runID/1000000)%30000 + 60000
	rwNumOut := rwNumIn + 1

	var rwInID, rwOutID, rt1ID, rt2ID, rtOutID string
	if err := db.QueryRowContext(ctx, `INSERT INTO rws (name, rw_number, is_active) VALUES ('PP RW In', $1, true) RETURNING id`, rwNumIn).Scan(&rwInID); err != nil {
		t.Fatalf("insert rwIn: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO rws (name, rw_number, is_active) VALUES ('PP RW Out', $1, true) RETURNING id`, rwNumOut).Scan(&rwOutID); err != nil {
		t.Fatalf("insert rwOut: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('PP RT1', $1, '001', $2, true) RETURNING id`, rwNumIn, rwInID).Scan(&rt1ID); err != nil {
		t.Fatalf("insert rt1: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('PP RT2', $1, '002', $2, true) RETURNING id`, rwNumIn, rwInID).Scan(&rt2ID); err != nil {
		t.Fatalf("insert rt2: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO rts (name, rw, rt, rw_id, is_active) VALUES ('PP RT Out', $1, '999', $2, true) RETURNING id`, rwNumOut, rwOutID).Scan(&rtOutID); err != nil {
		t.Fatalf("insert rtOut: %v", err)
	}

	resInID := uuid.NewString()
	resOutID := uuid.NewString()
	_, err := db.ExecContext(ctx, `INSERT INTO residents (id, rt_id, full_name, nik, is_active) VALUES ($1, $2, 'In Scope Res', $3, true)`,
		resInID, rt1ID, fmt.Sprintf("3173%012d", (runID/10+80)%1000000000000))
	if err != nil {
		t.Fatalf("insert resIn: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO residents (id, rt_id, full_name, nik, is_active) VALUES ($1, $2, 'Out Scope Res', $3, true)`,
		resOutID, rtOutID, fmt.Sprintf("3173%012d", (runID/10+81)%1000000000000))
	if err != nil {
		t.Fatalf("insert resOut: %v", err)
	}

	saUserID := uuid.NewString()
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, email, full_name, system_role, password_hash, is_active) VALUES ($1, $2, 'SA PP', 'super_admin', 'hash', true)`,
		saUserID, fmt.Sprintf("sa.pp.%d@test.com", runID))
	if err != nil {
		t.Fatalf("insert sa user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE rt_id IN ($1, $2, $3)`, rt1ID, rt2ID, rtOutID)
		_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE id IN ($1, $2)`, resInID, resOutID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, saUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id IN ($1, $2, $3)`, rt1ID, rt2ID, rtOutID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rws WHERE id IN ($1, $2)`, rwInID, rwOutID)
	})

	cfg := auth.ScopingConfig{
		RTIDs: []string{rt2ID},
		RWIDs: []string{rwInID},
	}
	store := storage.NewMockStorage()
	h := NewHandlerWithScope(pool, store, cfg)

	newUploadReq := func(residentID string, ac *auth.AuthContext) *http.Request {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("file", "avatar.png")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		_, _ = part.Write(makeTinyPNG(t))
		_ = mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/residents/"+residentID+"/photo", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", residentID)
		ctxWith := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		if ac != nil {
			ctxWith = context.WithValue(ctxWith, auth.AuthContextKey, ac)
		}
		return req.WithContext(ctxWith)
	}

	// 1. SuperAdmin (pinned to RT2) CAN upload photo for resident in scoped RT1 (via RW scope)
	saAC := &auth.AuthContext{UserID: saUserID, SystemRole: auth.SystemRoleSuperAdmin, RTID: rt2ID}
	rec1 := httptest.NewRecorder()
	h.Upload(rec1, newUploadReq(resInID, saAC))
	if rec1.Code != http.StatusCreated {
		t.Fatalf("SuperAdmin scoped RT1 photo upload expected 201, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// 2. SuperAdmin CANNOT upload photo for resident in out-of-scope RT
	rec2 := httptest.NewRecorder()
	h.Upload(rec2, newUploadReq(resOutID, saAC))
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("SuperAdmin out-of-scope RT photo upload expected 403, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 3. Non-SA tenant user from RT2 CANNOT upload photo for resident in RT1
	tenantRT2 := &auth.AuthContext{UserID: uuid.NewString(), RTID: rt2ID, TenantRole: auth.RolePengurus, Jabatan: auth.JabatanKetua}
	rec3 := httptest.NewRecorder()
	h.Upload(rec3, newUploadReq(resInID, tenantRT2))
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("Tenant RT2 uploading RT1 resident photo expected 404, got %d: %s", rec3.Code, rec3.Body.String())
	}

	// 4. Non-SA user with empty RTID fails closed in resolveTargetRT (no unscoped fallback!)
	emptyTenant := &auth.AuthContext{UserID: uuid.NewString(), RTID: "", TenantRole: auth.RoleWarga}
	if _, err := h.resolveTargetRT(ctx, emptyTenant, resInID); err == nil {
		t.Fatalf("expected non-SA with empty RTID to fail closed in resolveTargetRT, got nil error")
	}
}
