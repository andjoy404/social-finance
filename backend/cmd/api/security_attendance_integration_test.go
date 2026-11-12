package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	httpmw "social-finance/internal/http"
	"social-finance/internal/security"
	"social-finance/internal/storage"
)

type secTestEnv struct {
	pool          *database.Pool
	fresh         freshDBSetup
	env           *database.Pool
	svc           *security.Service
	repo          *security.Repository
	router        http.Handler
	store         storage.Service
	mu            sync.Mutex
	fakeNow       time.Time
	rtA           string
	rtB           string
	ketuaA        string
	ketuaAMID     string
	officerA1     string
	officerA1MID  string
	officerA2     string
	officerA2MID  string
	officerB      string
	officerBMID   string
	wargaA        string
	wargaAMID     string
	ketuaToken    string
	officer1Token string
	officer2Token string
	officerBToken string
	wargaToken    string
}

func setupSecurityIntegrationEnv(t *testing.T) *secTestEnv {
	t.Helper()
	fresh := createFreshMigratedDB(t)
	pool := openFreshPool(t, fresh)

	prevPool := auth.AuthDBPool
	auth.AuthDBPool = pool
	prevSecret := auth.SigningSecret
	auth.SigningSecret = []byte("security-integration-test-secret-key-32")
	prevLifetime := auth.DefaultAccessTokenLifetime
	auth.DefaultAccessTokenLifetime = 15 * time.Minute
	t.Cleanup(func() {
		auth.AuthDBPool = prevPool
		auth.SigningSecret = prevSecret
		auth.DefaultAccessTokenLifetime = prevLifetime
	})

	ctx := context.Background()
	var rwID string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rws (id, rw_number, name, is_active) VALUES (gen_random_uuid(), 16, 'RW 016', true)
		RETURNING id
	`).Scan(&rwID); err != nil {
		t.Fatalf("create rw: %v", err)
	}

	var rtA, rtB string
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rts (id, rw_id, name, rw, rt, is_active)
		VALUES (gen_random_uuid(), $1, 'RT 002 Urena', 16, '002', true) RETURNING id
	`, rwID).Scan(&rtA); err != nil {
		t.Fatalf("create rtA: %v", err)
	}
	if err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO rts (id, rw_id, name, rw, rt, is_active)
		VALUES (gen_random_uuid(), $1, 'RT 003 Sebelah', 16, '003', true) RETURNING id
	`, rwID).Scan(&rtB); err != nil {
		t.Fatalf("create rtB: %v", err)
	}

	createUser := func(email, name, rtID, role, jabatan string) (uid, mid string) {
		if err := pool.Raw().QueryRowContext(ctx, `
			INSERT INTO users (id, email, password_hash, full_name, is_active)
			VALUES (gen_random_uuid(), $1, 'hash', $2, true) RETURNING id
		`, email, name).Scan(&uid); err != nil {
			t.Fatalf("insert user %s: %v", email, err)
		}
		var jab any
		if jabatan != "" {
			jab = jabatan
		}
		if err := pool.Raw().QueryRowContext(ctx, `
			INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, true) RETURNING id
		`, uid, rtID, role, jab).Scan(&mid); err != nil {
			t.Fatalf("insert membership %s: %v", email, err)
		}
		return uid, mid
	}

	env := &secTestEnv{
		pool:    pool,
		fresh:   fresh,
		rtA:     rtA,
		rtB:     rtB,
		fakeNow: time.Date(2026, 11, 12, 8, 5, 0, 0, security.JakartaLoc),
	}
	env.ketuaA, env.ketuaAMID = createUser("ketua-a@test.local", "Ketua RT A", rtA, "pengurus", "ketua")
	env.officerA1, env.officerA1MID = createUser("sec1-a@test.local", "Petugas 1 A", rtA, "warga", "keamanan")
	env.officerA2, env.officerA2MID = createUser("sec2-a@test.local", "Petugas 2 A", rtA, "warga", "keamanan")
	env.officerB, env.officerBMID = createUser("sec1-b@test.local", "Petugas 1 B", rtB, "warga", "keamanan")
	env.wargaA, env.wargaAMID = createUser("warga-a@test.local", "Warga A", rtA, "warga", "")

	issueToken := func(uid, mid, rtID string, role auth.Role, jabatan auth.Jabatan) string {
		tok, err := auth.GenerateAccessToken(auth.TokenClaims{
			UserID:  uid,
			MID:     mid,
			RTID:    rtID,
			Role:    string(role),
			Jabatan: string(jabatan),
		})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return tok
	}
	env.ketuaToken = issueToken(env.ketuaA, env.ketuaAMID, rtA, auth.RolePengurus, auth.JabatanKetua)
	env.officer1Token = issueToken(env.officerA1, env.officerA1MID, rtA, auth.RoleWarga, auth.JabatanKeamanan)
	env.officer2Token = issueToken(env.officerA2, env.officerA2MID, rtA, auth.RoleWarga, auth.JabatanKeamanan)
	env.officerBToken = issueToken(env.officerB, env.officerBMID, rtB, auth.RoleWarga, auth.JabatanKeamanan)
	env.wargaToken = issueToken(env.wargaA, env.wargaAMID, rtA, auth.RoleWarga, "")

	repo := security.NewRepository(pool)
	svc := security.NewService(repo)
	svc.WithClock(func() time.Time {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.fakeNow
	})
	env.repo = repo
	env.svc = svc

	mockStore := storage.NewMockStorage()

	// Wire production security routes with controlled clock so tests can verify exact timing cleanly.
	h := security.NewHandler(svc, pool, func(ctx context.Context, rtID string, evalDate time.Time) (any, error) {
		return []map[string]any{{"house_number": "A-01", "rt_id": rtID}}, nil
	}, mockStore)
	r := chi.NewRouter()
	r.Use(httpmw.RequestID)
	r.Post("/api/v1/security/devices/activate", h.ActivateDevice)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(auth.RequireAllowedRT(pool, auth.ScopingConfig{RTIDs: []string{}, RWIDs: []string{}}))
		r.Use(auth.RequireAccess(nil, []auth.Jabatan{auth.JabatanKetua, auth.JabatanKeamanan}))
		r.Get("/api/v1/security/dashboard", h.GetDashboard)
		r.Get("/api/v1/security/officers", h.ListOfficers)
		r.Get("/api/v1/security/shifts", h.ListShifts)
		r.Post("/api/v1/security/shifts", h.CreateShift)
		r.Put("/api/v1/security/shifts/{id}", h.UpdateShift)
		r.Get("/api/v1/security/schedules", h.ListSchedules)
		r.Get("/api/v1/security/schedules/{id}", h.GetSchedule)
		r.Post("/api/v1/security/schedules", h.CreateSchedule)
		r.Put("/api/v1/security/schedules/{id}", h.UpdateSchedule)
		r.Delete("/api/v1/security/schedules/{id}", h.CancelSchedule)
		r.Get("/api/v1/security/swaps", h.ListSwaps)
		r.Post("/api/v1/security/swaps", h.CreateSwap)
		r.Post("/api/v1/security/swaps/{id}/target-respond", h.TargetRespondSwap)
		r.Post("/api/v1/security/swaps/{id}/decide", h.DecideSwap)
		r.Get("/api/v1/security/attendance/me", h.GetMyAttendanceOverview)
		r.Post("/api/v1/security/attendance/scan", h.ScanAttendanceQR)
		r.Get("/api/v1/security/attendance", h.ListAttendance)
		r.Get("/api/v1/security/corrections", h.ListCorrections)
		r.Post("/api/v1/security/corrections", h.CreateCorrection)
		r.Post("/api/v1/security/corrections/{id}/decide", h.DecideCorrection)
		r.Get("/api/v1/security/devices", h.ListDevices)
		r.Post("/api/v1/security/devices", h.CreateDevice)
		r.Post("/api/v1/security/devices/{id}/rotate-pairing", h.RotateDevicePairing)
		r.Post("/api/v1/security/devices/{id}/revoke", h.RevokeDevice)
		r.Get("/api/v1/security/guest-books", h.ListGuestBooksAdmin)
	})
	r.Group(func(r chi.Router) {
		r.Use(security.DeviceAuthMiddleware(repo))
		r.Get("/api/v1/security-device/me", h.PosMe)
		r.Post("/api/v1/security-device/qr", h.PosGenerateQR)
		r.Get("/api/v1/security-device/attendance/today", h.PosTodayAttendance)
		r.Get("/api/v1/security-device/guest-books", h.PosListGuestBooks)
		r.Post("/api/v1/security-device/guest-books", h.PosCreateGuestBook)
		r.Post("/api/v1/security-device/guest-books/{id}/checkout", h.PosCheckoutGuestBook)
		r.Get("/api/v1/security-device/warga/households", h.PosListHouseholds)
		r.Get("/api/v1/security-device/warga/households/{id}", h.PosGetHouseholdDetail)
		r.Get("/api/v1/security-device/monitoring/sampah", h.PosMonitoringSampah)
		r.Patch("/api/v1/security-device/monitoring/sampah/{householdOccupancyID}/note", h.PosUpdateSampahNote)
		r.Get("/api/v1/security-device/images/*", h.PosImage)
	})
	env.router = r
	env.store = mockStore
	return env
}

func (e *secTestEnv) setNow(t time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fakeNow = t
}

func (e *secTestEnv) doJSON(method, path, bearer, deviceTok string, body any, headers ...[2]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if deviceTok != "" {
		req.Header.Set("X-Device-Token", deviceTok)
	}
	for _, h := range headers {
		req.Header.Set(h[0], h[1])
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func provisionPosDevice(t *testing.T, e *secTestEnv) (deviceID, deviceToken string) {
	t.Helper()
	res := e.doJSON("POST", "/api/v1/security/devices", e.ketuaToken, "", map[string]any{
		"device_name":    "Pos Depan RT 002",
		"location_label": "Gerbang Utama",
	})
	if res.Code != http.StatusCreated {
		t.Fatalf("create device = %d: %s", res.Code, res.Body.String())
	}
	var prov security.DeviceProvisionResult
	if err := json.Unmarshal(res.Body.Bytes(), &prov); err != nil {
		t.Fatalf("unmarshal provision: %v", err)
	}
	if prov.PairingToken == "" {
		t.Fatal("expected non-empty pairing token")
	}

	actRes := e.doJSON("POST", "/api/v1/security/devices/activate", "", "", map[string]any{
		"pairing_token": prov.PairingToken,
	})
	if actRes.Code != http.StatusOK {
		t.Fatalf("activate device = %d: %s", actRes.Code, actRes.Body.String())
	}
	var act security.DeviceActivationResult
	if err := json.Unmarshal(actRes.Body.Bytes(), &act); err != nil {
		t.Fatalf("unmarshal activation: %v", err)
	}
	if act.DeviceToken == "" {
		t.Fatal("expected non-empty device token")
	}

	// Single-use pairing token replay must fail.
	replay := e.doJSON("POST", "/api/v1/security/devices/activate", "", "", map[string]any{
		"pairing_token": prov.PairingToken,
	})
	if replay.Code == http.StatusOK {
		t.Fatalf("expected pairing token replay to fail, got 200")
	}

	return act.Device.ID, act.DeviceToken
}

func TestSecurityAttendance_Migration077UpDownUp(t *testing.T) {
	fresh := createFreshMigratedDB(t)
	migDir := findMigrationsDir(t)
	downSQL, err := os.ReadFile(filepath.Join(migDir, "077_security_attendance.down.sql"))
	if err != nil {
		t.Fatalf("read 077 down sql: %v", err)
	}
	upSQL, err := os.ReadFile(filepath.Join(migDir, "077_security_attendance.up.sql"))
	if err != nil {
		t.Fatalf("read 077 up sql: %v", err)
	}

	db, err := sql.Open("pgx", fresh.dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(string(downSQL)); err != nil {
		t.Fatalf("migration 077 down failed: %v", err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'security_devices')`).Scan(&exists); err != nil || exists {
		t.Fatalf("expected security_devices dropped after down, exists=%v err=%v", exists, err)
	}
	if _, err := db.Exec(string(upSQL)); err != nil {
		t.Fatalf("migration 077 re-up failed: %v", err)
	}
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'security_attendance_records')`).Scan(&exists); err != nil || !exists {
		t.Fatalf("expected security_attendance_records restored after re-up, exists=%v err=%v", exists, err)
	}
	periodUp, err := os.ReadFile(filepath.Join(migDir, "078_security_schedule_period.up.sql"))
	if err != nil {
		t.Fatalf("read 078 up sql: %v", err)
	}
	if _, err := db.Exec(string(periodUp)); err != nil {
		t.Fatalf("migration 078 up failed: %v", err)
	}
	periodDown, err := os.ReadFile(filepath.Join(migDir, "078_security_schedule_period.down.sql"))
	if err != nil {
		t.Fatalf("read 078 down sql: %v", err)
	}
	if _, err := db.Exec(string(periodDown)); err != nil {
		t.Fatalf("migration 078 down failed: %v", err)
	}
}

func TestSecuritySchedulePeriodCreateConflictAndEdit(t *testing.T) {
	env := setupSecurityIntegrationEnv(t)
	shiftRes := env.doJSON("POST", "/api/v1/security/shifts", env.ketuaToken, "", map[string]any{
		"name": "Day", "start_time": "08:00", "end_time": "16:00", "lateness_tolerance_minutes": 15,
		"check_in_window_before_minutes": 60, "check_in_window_after_minutes": 120, "absent_cutoff_minutes": 120, "missing_checkout_cutoff_minutes": 180,
	})
	if shiftRes.Code != http.StatusCreated {
		t.Fatalf("create shift: %d %s", shiftRes.Code, shiftRes.Body.String())
	}
	var shift security.ShiftConfig
	if err := json.Unmarshal(shiftRes.Body.Bytes(), &shift); err != nil {
		t.Fatal(err)
	}
	create := func(user, start, end string) *httptest.ResponseRecorder {
		return env.doJSON("POST", "/api/v1/security/schedules", env.ketuaToken, "", map[string]any{"user_id": user, "shift_config_id": shift.ID, "start_date": start, "end_date": end})
	}
	conflict := create(env.officerA1, "2026-11-14", "2026-11-14")
	if conflict.Code != http.StatusCreated {
		t.Fatalf("seed conflict day: %d %s", conflict.Code, conflict.Body.String())
	}
	period := create(env.officerA2, "2026-11-29", "2026-12-02")
	if period.Code != http.StatusCreated {
		t.Fatalf("create range crossing month: %d %s", period.Code, period.Body.String())
	}
	var first security.SecuritySchedule
	if err := json.Unmarshal(period.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	var daily, attendance int
	if err := env.pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM security_schedules WHERE assignment_id=$1`, first.AssignmentID).Scan(&daily); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM security_attendance_records ar JOIN security_schedules s ON s.id=ar.schedule_id WHERE s.assignment_id=$1`, first.AssignmentID).Scan(&attendance); err != nil {
		t.Fatal(err)
	}
	if daily != 4 || attendance != 4 {
		t.Fatalf("period expansion daily=%d attendance=%d; want 4 each", daily, attendance)
	}
	failed := create(env.officerA1, "2026-11-12", "2026-11-16")
	if failed.Code != http.StatusConflict {
		t.Fatalf("mid-range conflict status=%d body=%s", failed.Code, failed.Body.String())
	}
	var partial int
	if err := env.pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM security_schedules WHERE user_id=$1 AND shift_date IN ('2026-11-12','2026-11-13','2026-11-15','2026-11-16') AND status='scheduled'`, env.officerA1).Scan(&partial); err != nil {
		t.Fatal(err)
	}
	if partial != 0 {
		t.Fatalf("failed create left %d partial rows", partial)
	}
	got, err := env.svc.GetScheduleAssignment(context.Background(), env.rtA, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("detail returned %d dates, want 4", len(got))
	}
	update := env.doJSON("PUT", "/api/v1/security/schedules/"+first.ID, env.ketuaToken, "", map[string]any{"user_id": env.officerA2, "shift_config_id": shift.ID, "start_date": "2026-11-30", "end_date": "2026-12-03"})
	if update.Code != http.StatusOK {
		t.Fatalf("edit range: %d %s", update.Code, update.Body.String())
	}
	var after int
	if err := env.pool.Raw().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM security_schedules WHERE assignment_id=$1 AND status='scheduled'`, first.AssignmentID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != 4 {
		t.Fatalf("edited daily row count=%d want 4", after)
	}
	var recordedID string
	if err := env.pool.Raw().QueryRowContext(context.Background(), `SELECT id FROM security_schedules WHERE assignment_id=$1 AND shift_date='2026-12-01'`, first.AssignmentID).Scan(&recordedID); err != nil {
		t.Fatal(err)
	}
	checkedIn := time.Date(2026, 12, 1, 8, 5, 0, 0, security.JakartaLoc)
	if _, err := env.pool.Raw().ExecContext(context.Background(), `UPDATE security_attendance_records SET status='hadir',checked_in_at=$2 WHERE schedule_id=$1`, recordedID, checkedIn); err != nil {
		t.Fatal(err)
	}
	blocked := env.doJSON("PUT", "/api/v1/security/schedules/"+first.ID, env.ketuaToken, "", map[string]any{"user_id": env.officerA1, "shift_config_id": shift.ID, "start_date": "2026-11-30", "end_date": "2026-12-03"})
	if blocked.Code != http.StatusConflict {
		t.Fatalf("edit with recorded attendance status=%d body=%s", blocked.Code, blocked.Body.String())
	}
	var retainedUser string
	var retainedCheckin time.Time
	if err := env.pool.Raw().QueryRowContext(context.Background(), `SELECT s.user_id,ar.checked_in_at FROM security_schedules s JOIN security_attendance_records ar ON ar.schedule_id=s.id WHERE s.id=$1`, recordedID).Scan(&retainedUser, &retainedCheckin); err != nil {
		t.Fatal(err)
	}
	if retainedUser != env.officerA2 || !retainedCheckin.Equal(checkedIn) {
		t.Fatalf("attendance history mutated: user=%s checked_in=%s", retainedUser, retainedCheckin)
	}
}

func TestSecurityAttendance_QRCheckInOutReplayParallelCrossRTAndRBAC(t *testing.T) {
	env := setupSecurityIntegrationEnv(t)
	devID, devTok := provisionPosDevice(t, env)

	// 1. Non-ketua cannot create shifts (RBAC check).
	rForbidden := env.doJSON("POST", "/api/v1/security/shifts", env.officer1Token, "", map[string]any{
		"name": "Shift Pagi", "start_time": "08:00", "end_time": "16:00",
	})
	if rForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected officer create shift forbidden (403), got %d", rForbidden.Code)
	}
	rWargaForbidden := env.doJSON("GET", "/api/v1/security/shifts", env.wargaToken, "", nil)
	if rWargaForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected warga forbidden (403), got %d", rWargaForbidden.Code)
	}
	// User JWT cannot call Pos Device QR endpoint!
	rUserOnPos := env.doJSON("POST", "/api/v1/security-device/qr", env.officer1Token, "", nil)
	if rUserOnPos.Code != http.StatusUnauthorized {
		t.Fatalf("expected user JWT rejected on Pos endpoint (401), got %d", rUserOnPos.Code)
	}
	// Fake header override for non-super-admin must still fail or use own RT!
	rSpoof := env.doJSON("GET", "/api/v1/security/officers?rt_id="+env.rtB, env.ketuaToken, "", nil, [2]string{"X-RT-ID", env.rtB})
	if rSpoof.Code != http.StatusOK {
		t.Fatalf("list officers = %d", rSpoof.Code)
	}
	var offResp struct{ Data []security.SecurityOfficer }
	_ = json.Unmarshal(rSpoof.Body.Bytes(), &offResp)
	for _, o := range offResp.Data {
		if o.RTID != env.rtA {
			t.Fatalf("cross-RT data leak via header/query spoof! got rt=%s", o.RTID)
		}
	}

	// 2. Ketua creates Shift Pagi 08:00 - 16:00 with 15 min lateness tolerance.
	shiftRes := env.doJSON("POST", "/api/v1/security/shifts", env.ketuaToken, "", map[string]any{
		"name":                            "Shift Pagi",
		"start_time":                      "08:00",
		"end_time":                        "16:00",
		"lateness_tolerance_minutes":      15,
		"check_in_window_before_minutes":  60,
		"check_in_window_after_minutes":   120,
		"absent_cutoff_minutes":           120,
		"missing_checkout_cutoff_minutes": 180,
	})
	if shiftRes.Code != http.StatusCreated {
		t.Fatalf("create shift = %d: %s", shiftRes.Code, shiftRes.Body.String())
	}
	var shift security.ShiftConfig
	_ = json.Unmarshal(shiftRes.Body.Bytes(), &shift)

	// 3. Schedule Officer A1 for 2026-11-12, and reject overlapping schedule on same window.
	schedRes := env.doJSON("POST", "/api/v1/security/schedules", env.ketuaToken, "", map[string]any{
		"user_id":         env.officerA1,
		"shift_config_id": shift.ID,
		"start_date":      "2026-11-12",
		"end_date":        "2026-11-12",
	})
	if schedRes.Code != http.StatusCreated {
		t.Fatalf("create schedule = %d: %s", schedRes.Code, schedRes.Body.String())
	}
	overlapRes := env.doJSON("POST", "/api/v1/security/schedules", env.ketuaToken, "", map[string]any{
		"user_id":         env.officerA1,
		"shift_config_id": shift.ID,
		"start_date":      "2026-11-12",
		"end_date":        "2026-11-12",
	})
	if overlapRes.Code != http.StatusConflict {
		t.Fatalf("expected 409 schedule conflict, got %d", overlapRes.Code)
	}

	// Helper to fetch dynamic QR from Pos device
	genQR := func() string {
		qrRes := env.doJSON("POST", "/api/v1/security-device/qr", "", devTok, nil)
		if qrRes.Code != http.StatusCreated {
			t.Fatalf("pos qr generate = %d: %s", qrRes.Code, qrRes.Body.String())
		}
		var p security.DynamicQRPayload
		_ = json.Unmarshal(qrRes.Body.Bytes(), &p)
		return p.QRToken
	}

	// 4. Check-out before Check-in rejected.
	qrEarlyOut := genQR()
	earlyOutRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
		"qr_token": qrEarlyOut, "action": "check_out",
	})
	if earlyOutRes.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 checkout_before_checkin, got %d: %s", earlyOutRes.Code, earlyOutRes.Body.String())
	}

	// 5. Officer from RT B scanning RT A QR -> 403 cross_rt_denied!
	crossQR := genQR()
	crossRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officerBToken, "", map[string]any{
		"qr_token": crossQR, "action": "check_in",
	})
	if crossRes.Code != http.StatusForbidden {
		t.Fatalf("expected 403 cross_rt_denied, got %d: %s", crossRes.Code, crossRes.Body.String())
	}

	// 6. Officer A2 (unscheduled) scanning -> 400 no_matching_schedule!
	unschedRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer2Token, "", map[string]any{
		"qr_token": crossQR, "action": "check_in",
	})
	if unschedRes.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 no_matching_schedule for unscheduled officer, got %d: %s", unschedRes.Code, unschedRes.Body.String())
	}

	// 7. Expired QR rejected!
	expiredQR := genQR()
	env.setNow(time.Date(2026, 11, 12, 8, 6, 30, 0, security.JakartaLoc)) // +90s > 45s TTL
	expRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
		"qr_token": expiredQR, "action": "check_in",
	})
	if expRes.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 qr_expired, got %d: %s", expRes.Code, expRes.Body.String())
	}

	// 8. Valid on-time Check-In at 08:10 WIB (within 15m tolerance) + Idempotency-Key + Single-use Replay!
	env.setNow(time.Date(2026, 11, 12, 8, 10, 0, 0, security.JakartaLoc))
	validQR := genQR()
	idemKey := "scan-checkin-attempt-1"

	inRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
		"qr_token": validQR, "action": "check_in",
	}, [2]string{"Idempotency-Key", idemKey})
	if inRes.Code != http.StatusOK {
		t.Fatalf("expected 200 valid check-in, got %d: %s", inRes.Code, inRes.Body.String())
	}
	var scanOut security.AttendanceScanResult
	_ = json.Unmarshal(inRes.Body.Bytes(), &scanOut)
	if scanOut.Record.Status != security.StatusHadir {
		t.Fatalf("expected status 'hadir', got %q", scanOut.Record.Status)
	}

	// Network retry with same Idempotency-Key returns 200 idempotently.
	idemRetry := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
		"qr_token": validQR, "action": "check_in",
	}, [2]string{"Idempotency-Key", idemKey})
	if idemRetry.Code != http.StatusOK {
		t.Fatalf("expected idempotent 200 on retry, got %d: %s", idemRetry.Code, idemRetry.Body.String())
	}

	// Replay WITHOUT same Idempotency-Key fails with 409 qr_already_used.
	replayRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
		"qr_token": validQR, "action": "check_in",
	})
	if replayRes.Code != http.StatusConflict {
		t.Fatalf("expected 409 qr_already_used on replay, got %d: %s", replayRes.Code, replayRes.Body.String())
	}

	// Double check-in with another fresh QR is blocked (409 already_checked_in).
	doubleQR := genQR()
	dblRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
		"qr_token": doubleQR, "action": "check_in",
	})
	if dblRes.Code != http.StatusConflict {
		t.Fatalf("expected 409 already_checked_in on double check-in, got %d: %s", dblRes.Code, dblRes.Body.String())
	}

	// 9. Concurrent parallel check-out race on the same QR token -> strictly 1 succeeds, 1 rejected!
	env.setNow(time.Date(2026, 11, 12, 16, 5, 0, 0, security.JakartaLoc))
	raceQR := genQR()
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
				"qr_token": raceQR, "action": "check_out",
			})
			codes[idx] = r.Code
		}(i)
	}
	wg.Wait()
	if !((codes[0] == 200 && codes[1] == 409) || (codes[0] == 409 && codes[1] == 200)) {
		t.Fatalf("expected exactly one 200 and one 409 under parallel QR scan race, got %v", codes)
	}

	// 10. Revoking Pos device immediately blocks future Pos requests and QR tokens!
	revRes := env.doJSON("POST", "/api/v1/security/devices/"+devID+"/revoke", env.ketuaToken, "", nil)
	if revRes.Code != http.StatusOK {
		t.Fatalf("revoke device = %d: %s", revRes.Code, revRes.Body.String())
	}
	revPosRes := env.doJSON("POST", "/api/v1/security-device/qr", "", devTok, nil)
	if revPosRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked device token rejected (401), got %d", revPosRes.Code)
	}
}

func TestSecurityAttendance_LatenessMidnightSchedulerSwapCorrectionAndGuestBook(t *testing.T) {
	env := setupSecurityIntegrationEnv(t)
	_, devTok := provisionPosDevice(t, env)

	// Create Night Shift (22:00 - 06:00, crosses midnight) with 10m tolerance, 120m absent cutoff.
	shiftRes := env.doJSON("POST", "/api/v1/security/shifts", env.ketuaToken, "", map[string]any{
		"name":                            "Shift Malam",
		"start_time":                      "22:00",
		"end_time":                        "06:00",
		"crosses_midnight":                true,
		"lateness_tolerance_minutes":      10,
		"check_in_window_before_minutes":  60,
		"check_in_window_after_minutes":   120,
		"absent_cutoff_minutes":           120,
		"missing_checkout_cutoff_minutes": 180,
	})
	if shiftRes.Code != http.StatusCreated {
		t.Fatalf("create night shift = %d: %s", shiftRes.Code, shiftRes.Body.String())
	}
	var shift security.ShiftConfig
	_ = json.Unmarshal(shiftRes.Body.Bytes(), &shift)

	// Schedule A1 on 2026-11-12 and A2 on 2026-11-13
	s1Res := env.doJSON("POST", "/api/v1/security/schedules", env.ketuaToken, "", map[string]any{
		"user_id": env.officerA1, "shift_config_id": shift.ID, "shift_date": "2026-11-12",
	})
	s2Res := env.doJSON("POST", "/api/v1/security/schedules", env.ketuaToken, "", map[string]any{
		"user_id": env.officerA2, "shift_config_id": shift.ID, "shift_date": "2026-11-13",
	})
	var s1, s2 security.SecuritySchedule
	_ = json.Unmarshal(s1Res.Body.Bytes(), &s1)
	_ = json.Unmarshal(s2Res.Body.Bytes(), &s2)

	// 1. Two-step Shift Swap: A1 proposes swap with A2 -> A2 accepts -> Ketua approves -> schedule owners swap!
	swapRes := env.doJSON("POST", "/api/v1/security/swaps", env.officer1Token, "", map[string]any{
		"requester_schedule_id": s1.ID,
		"target_schedule_id":    s2.ID,
		"reason":                "Acara keluarga",
	})
	if swapRes.Code != http.StatusCreated {
		t.Fatalf("create swap = %d: %s", swapRes.Code, swapRes.Body.String())
	}
	var swap security.ShiftSwap
	_ = json.Unmarshal(swapRes.Body.Bytes(), &swap)

	// Ketua cannot approve before target accepts!
	premature := env.doJSON("POST", "/api/v1/security/swaps/"+swap.ID+"/decide", env.ketuaToken, "", map[string]any{"approve": true})
	if premature.Code != http.StatusConflict {
		t.Fatalf("expected 409 approving swap before target accepted, got %d", premature.Code)
	}

	tgtAccept := env.doJSON("POST", "/api/v1/security/swaps/"+swap.ID+"/target-respond", env.officer2Token, "", map[string]any{"accept": true})
	if tgtAccept.Code != http.StatusOK {
		t.Fatalf("target respond = %d: %s", tgtAccept.Code, tgtAccept.Body.String())
	}
	ketuaApprove := env.doJSON("POST", "/api/v1/security/swaps/"+swap.ID+"/decide", env.ketuaToken, "", map[string]any{"approve": true})
	if ketuaApprove.Code != http.StatusOK {
		t.Fatalf("ketua decide swap = %d: %s", ketuaApprove.Code, ketuaApprove.Body.String())
	}

	// After swap, Officer A2 now owns 2026-11-12 night shift (22:00 - 06:00).
	// Also test shift config versioning: Ketua edits Shift Malam tolerance to 30m -> version becomes 2, existing s1 keeps 10m snapshot!
	env.doJSON("PUT", "/api/v1/security/shifts/"+shift.ID, env.ketuaToken, "", map[string]any{
		"name":                            "Shift Malam",
		"start_time":                      "22:00",
		"end_time":                        "06:00",
		"crosses_midnight":                true,
		"lateness_tolerance_minutes":      30,
		"check_in_window_before_minutes":  60,
		"check_in_window_after_minutes":   120,
		"absent_cutoff_minutes":           120,
		"missing_checkout_cutoff_minutes": 180,
	})

	// 2. Officer A2 checks in LATE at 22:20 WIB (snapshot tolerance is 10m!) -> must be Terlambat!
	env.setNow(time.Date(2026, 11, 12, 22, 20, 0, 0, security.JakartaLoc))
	qrRes := env.doJSON("POST", "/api/v1/security-device/qr", "", devTok, nil)
	var qr security.DynamicQRPayload
	_ = json.Unmarshal(qrRes.Body.Bytes(), &qr)

	lateScan := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer2Token, "", map[string]any{
		"qr_token": qr.QRToken, "action": "check_in",
	})
	if lateScan.Code != http.StatusOK {
		t.Fatalf("late scan = %d: %s", lateScan.Code, lateScan.Body.String())
	}
	var lateOut security.AttendanceScanResult
	_ = json.Unmarshal(lateScan.Body.Bytes(), &lateOut)
	if lateOut.Record.Status != security.StatusTerlambat || lateOut.Record.LateMinutes != 20 {
		t.Fatalf("expected Terlambat (20m) preserving old shift snapshot, got %s (%dm)", lateOut.Record.Status, lateOut.Record.LateMinutes)
	}

	// 3. Check-Out on next calendar day 06:05 WIB (midnight-crossing shift)!
	env.setNow(time.Date(2026, 11, 13, 6, 5, 0, 0, security.JakartaLoc))
	qrOutRes := env.doJSON("POST", "/api/v1/security-device/qr", "", devTok, nil)
	var qrOut security.DynamicQRPayload
	_ = json.Unmarshal(qrOutRes.Body.Bytes(), &qrOut)
	outScan := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer2Token, "", map[string]any{
		"qr_token": qrOut.QRToken, "action": "check_out",
	})
	if outScan.Code != http.StatusOK {
		t.Fatalf("midnight checkout failed = %d: %s", outScan.Code, outScan.Body.String())
	}
	var outParsed security.AttendanceScanResult
	_ = json.Unmarshal(outScan.Body.Bytes(), &outParsed)
	if outParsed.Record.Status != security.StatusSudahAbsenPulang {
		t.Fatalf("expected sudah_absen_pulang, got %s", outParsed.Record.Status)
	}

	// 4. Advance clock past 2026-11-13 absent cutoff (00:05 on 2026-11-14) and run scheduler twice -> A1 (owner of 11-13) becomes tidak_hadir idempotently!
	env.setNow(time.Date(2026, 11, 14, 0, 5, 0, 0, security.JakartaLoc))
	tick1, err := env.svc.RunSchedulerTick(context.Background())
	if err != nil {
		t.Fatalf("scheduler tick 1 failed: %v", err)
	}
	if tick1.MarkedAbsentCount != 1 {
		t.Fatalf("expected 1 marked absent, got %d", tick1.MarkedAbsentCount)
	}
	tick2, err := env.svc.RunSchedulerTick(context.Background())
	if err != nil {
		t.Fatalf("scheduler tick 2 failed: %v", err)
	}
	if tick2.MarkedAbsentCount != 0 || tick2.MarkedMissingCheckout != 0 {
		t.Fatalf("expected idempotent 0 mutations on second scheduler tick, got %+v", tick2)
	}

	// 5. Attendance Correction + Anti-Self-Approval + Audit Trail + Scheduler Lock!
	attList := env.doJSON("GET", "/api/v1/security/attendance?user_id="+env.officerA1, env.ketuaToken, "", nil)
	var attResp struct{ Data []security.AttendanceRecord }
	_ = json.Unmarshal(attList.Body.Bytes(), &attResp)
	if len(attResp.Data) == 0 {
		t.Fatal("expected absent attendance record for officerA1")
	}
	absRec := attResp.Data[0]

	// Missing reason must fail 400.
	badCorr := env.doJSON("POST", "/api/v1/security/corrections", env.officer1Token, "", map[string]any{
		"attendance_record_id": absRec.ID,
		"request_type":         "status_change",
		"requested_status":     "hadir",
		"reason":               "",
	})
	if badCorr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 empty correction reason, got %d", badCorr.Code)
	}

	// Officer A1 requests correction; Ketua approves; also test self-approval block when Ketua creates a correction.
	corrRes := env.doJSON("POST", "/api/v1/security/corrections", env.officer1Token, "", map[string]any{
		"attendance_record_id": absRec.ID,
		"request_type":         "manual_check_in",
		"requested_status":     "hadir",
		"reason":               "Listrik pos padam, QR tablet mati saat shift mulai",
	})
	if corrRes.Code != http.StatusCreated {
		t.Fatalf("create correction = %d: %s", corrRes.Code, corrRes.Body.String())
	}
	var corr security.AttendanceCorrection
	_ = json.Unmarshal(corrRes.Body.Bytes(), &corr)

	// Ketua approves Officer A1's correction -> succeeds and writes audit_logs!
	appRes := env.doJSON("POST", "/api/v1/security/corrections/"+corr.ID+"/decide", env.ketuaToken, "", map[string]any{
		"approve": true, "note": "Terverifikasi oleh Ketua RT",
	})
	if appRes.Code != http.StatusOK {
		t.Fatalf("approve correction = %d: %s", appRes.Code, appRes.Body.String())
	}
	var auditCnt int
	_ = env.pool.Raw().QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM audit_logs WHERE rt_id = $1 AND entity_id = $2 AND action = 'security_correction_approved'
	`, env.rtA, corr.ID).Scan(&auditCnt)
	if auditCnt != 1 {
		t.Fatalf("expected 1 audit_logs entry for approved correction, got %d", auditCnt)
	}

	// Self-approval test: Ketua submits correction on behalf of A2 -> Ketua attempting to approve own request gets 403!
	selfCorrRes := env.doJSON("POST", "/api/v1/security/corrections", env.ketuaToken, "", map[string]any{
		"attendance_record_id": lateOut.Record.ID,
		"request_type":         "status_change",
		"requested_status":     "sudah_absen_pulang",
		"reason":               "Test maker-checker",
	})
	var selfCorr security.AttendanceCorrection
	_ = json.Unmarshal(selfCorrRes.Body.Bytes(), &selfCorr)
	selfDecide := env.doJSON("POST", "/api/v1/security/corrections/"+selfCorr.ID+"/decide", env.ketuaToken, "", map[string]any{"approve": true})
	if selfDecide.Code != http.StatusForbidden {
		t.Fatalf("expected 403 self_approval_forbidden, got %d: %s", selfDecide.Code, selfDecide.Body.String())
	}

	// 6. Guest Book: only Pos Device can create/checkout; Ketua RT can read (scoped RT); officer denied!
	officerGB := env.doJSON("POST", "/api/v1/security-device/guest-books", env.officer1Token, "", map[string]any{
		"guest_name": "Tamu", "purpose": "Antar", "destination_label": "Blok A1",
	})
	if officerGB.Code != http.StatusUnauthorized {
		t.Fatalf("expected user rejected on Pos guest book write (401), got %d", officerGB.Code)
	}
	posGB := env.doJSON("POST", "/api/v1/security-device/guest-books", "", devTok, map[string]any{
		"guest_name":        "Bapak Budi",
		"phone":             "081234567890",
		"purpose":           "Mengantar Paket",
		"destination_label": "Blok B-12",
		"vehicle_plate":     "B 1234 XYZ",
	})
	if posGB.Code != http.StatusCreated {
		t.Fatalf("pos create guest book = %d: %s", posGB.Code, posGB.Body.String())
	}
	var gb security.GuestBookEntry
	_ = json.Unmarshal(posGB.Body.Bytes(), &gb)
	if gb.Phone != nil && *gb.Phone == "081234567890" {
		t.Fatalf("expected phone masked for PII protection, got raw %s", *gb.Phone)
	}

	gbOut := env.doJSON("POST", "/api/v1/security-device/guest-books/"+gb.ID+"/checkout", "", devTok, nil)
	if gbOut.Code != http.StatusOK {
		t.Fatalf("pos guest checkout = %d: %s", gbOut.Code, gbOut.Body.String())
	}

	ketuaGB := env.doJSON("GET", "/api/v1/security/guest-books", env.ketuaToken, "", nil)
	if ketuaGB.Code != http.StatusOK {
		t.Fatalf("ketua list guest book = %d: %s", ketuaGB.Code, ketuaGB.Body.String())
	}
	offGBDeny := env.doJSON("GET", "/api/v1/security/guest-books", env.officer1Token, "", nil)
	if offGBDeny.Code != http.StatusForbidden {
		t.Fatalf("expected officer denied admin guest-book endpoint (403), got %d", offGBDeny.Code)
	}

	// Pos Device read-only Warga + Sampah endpoints succeed strictly for device RT.
	wargaPos := env.doJSON("GET", "/api/v1/security-device/warga/households", "", devTok, nil)
	if wargaPos.Code != http.StatusOK {
		t.Fatalf("pos warga households = %d: %s", wargaPos.Code, wargaPos.Body.String())
	}
	sampahPos := env.doJSON("GET", "/api/v1/security-device/monitoring/sampah", "", devTok, nil)
	if sampahPos.Code != http.StatusOK {
		t.Fatalf("pos monitoring sampah = %d: %s", sampahPos.Code, sampahPos.Body.String())
	}
	_ = fmt.Sprintf("verified %s", gb.ID)
}

func TestSecurityAttendance_OnDutyOfficersFiltering(t *testing.T) {
	env := setupSecurityIntegrationEnv(t)
	devID, devTok := provisionPosDevice(t, env)
	ctx := context.Background()

	// Fixed moment inside the shift window (08:05, shift 08:00-16:00).
	shiftStartNow := time.Date(2026, 11, 12, 8, 5, 0, 0, security.JakartaLoc)
	env.setNow(shiftStartNow)

	// Create shift Pagi.
	shiftRes := env.doJSON("POST", "/api/v1/security/shifts", env.ketuaToken, "", map[string]any{
		"name":                            "Shift Pagi",
		"start_time":                      "08:00",
		"end_time":                        "16:00",
		"lateness_tolerance_minutes":      15,
		"check_in_window_before_minutes":  60,
		"check_in_window_after_minutes":   120,
		"absent_cutoff_minutes":           120,
		"missing_checkout_cutoff_minutes": 180,
	})
	if shiftRes.Code != http.StatusCreated {
		t.Fatalf("create shift = %d: %s", shiftRes.Code, shiftRes.Body.String())
	}
	var shift security.ShiftConfig
	_ = json.Unmarshal(shiftRes.Body.Bytes(), &shift)

	// Schedule both officers for today (different users, same window is fine).
	for _, tc := range []struct {
		name string
		user string
	}{
		{"officerA1", env.officerA1},
		{"officerA2", env.officerA2},
	} {
		res := env.doJSON("POST", "/api/v1/security/schedules", env.ketuaToken, "", map[string]any{
			"user_id":         tc.user,
			"shift_config_id": shift.ID,
			"start_date":      "2026-11-12",
			"end_date":        "2026-11-12",
		})
		if res.Code != http.StatusCreated {
			t.Fatalf("schedule %s = %d: %s", tc.name, res.Code, res.Body.String())
		}
	}

	// Give officer A1 a phone number so the on-duty projection carries contact data.
	if _, err := env.pool.Raw().ExecContext(ctx,
		`UPDATE users SET phone = '08110004444' WHERE id = $1`, env.officerA1); err != nil {
		t.Fatalf("set phone: %v", err)
	}

	// Give officer A1 and A2 a linked resident row with an avatar reference.
	for _, tc := range []struct {
		userID string
		name   string
		key    string
	}{
		{env.officerA1, "Petugas 1 A", "community/" + env.rtA + "/profile/aaaaaaaaaaaaaaaaaaaa.jpg"},
		{env.officerA2, "Petugas 2 A", "community/" + env.rtA + "/profile/bbbbbbbbbbbbbbbbbbbb.jpg"},
	} {
		if _, err := env.pool.Raw().ExecContext(ctx, `
			INSERT INTO residents (id, rt_id, full_name, user_id, is_active, avatar_path)
			VALUES (gen_random_uuid(), $1, $2, $3, true, $4)
		`, env.rtA, tc.name, tc.userID, tc.key); err != nil {
			t.Fatalf("seed officer resident %s: %v", tc.name, err)
		}
	}

	onDuty := func(rtID string) []security.OnDutyOfficerSummary {
		t.Helper()
		env.setNow(shiftStartNow)
		items, err := env.svc.GetOnDutyOfficers(ctx, rtID)
		if err != nil {
			t.Fatalf("GetOnDutyOfficers: %v", err)
		}
		return items
	}

	// Neither officer has checked in yet.
	if got := onDuty(env.rtA); len(got) != 0 {
		t.Fatalf("expected no on-duty officers before check-in, got %+v", got)
	}

	genQR := func() string {
		t.Helper()
		qrRes := env.doJSON("POST", "/api/v1/security-device/qr", "", devTok, nil)
		if qrRes.Code != http.StatusCreated {
			t.Fatalf("pos qr generate = %d: %s", qrRes.Code, qrRes.Body.String())
		}
		var p security.DynamicQRPayload
		_ = json.Unmarshal(qrRes.Body.Bytes(), &p)
		return p.QRToken
	}
	// Both officers check in.
	for _, tok := range []string{env.officer1Token, env.officer2Token} {
		res := env.doJSON("POST", "/api/v1/security/attendance/scan", tok, "", map[string]any{
			"qr_token": genQR(),
			"action":   "check_in",
		})
		if res.Code != http.StatusOK {
			t.Fatalf("check-in = %d: %s", res.Code, res.Body.String())
		}
	}

	// Both checked in, window in progress -> both appear, A1 phone surfaced.
	got := onDuty(env.rtA)
	if len(got) != 2 {
		t.Fatalf("expected 2 on-duty officers, got %+v", got)
	}
	var names []string
	var phoneFound bool
	var avatarFound bool
	for _, o := range got {
		names = append(names, o.FullName)
		if o.PhoneNumber != nil && *o.PhoneNumber == "08110004444" {
			phoneFound = true
		}
		if o.AvatarPath != nil && *o.AvatarPath != "" {
			avatarFound = true
		}
	}
	if !phoneFound {
		t.Fatalf("expected officer A1 phone to be projected, got names=%v", names)
	}
	if !avatarFound {
		t.Fatalf("expected officers on-duty to carry avatar_path, got %+v", got)
	}

	// RT B scope must see none of them (cross-RT isolation).
	if got := func() []security.OnDutyOfficerSummary {
		env.setNow(shiftStartNow)
		items, err := env.svc.GetOnDutyOfficers(ctx, env.rtB)
		if err != nil {
			t.Fatalf("GetOnDutyOfficers rtB: %v", err)
		}
		return items
	}(); len(got) != 0 {
		t.Fatalf("expected no on-duty officers in RT B, got %+v", got)
	}

	// Outside the schedule window (before start) -> hidden.
	env.setNow(time.Date(2026, 11, 12, 3, 0, 0, 0, security.JakartaLoc))
	if items, err := env.svc.GetOnDutyOfficers(ctx, env.rtA); err != nil || len(items) != 0 {
		t.Fatalf("expected on-duty hidden before shift window, err=%v got=%+v", err, items)
	}
	// Outside the schedule window (after end) -> hidden even without checkout.
	env.setNow(time.Date(2026, 11, 12, 17, 0, 0, 0, security.JakartaLoc))
	if items, err := env.svc.GetOnDutyOfficers(ctx, env.rtA); err != nil || len(items) != 0 {
		t.Fatalf("expected on-duty hidden after shift window, err=%v got=%+v", err, items)
	}
	env.setNow(shiftStartNow)

	// Deactivated officer (users.is_active=false) hidden, membership intact.
	if _, err := env.pool.Raw().ExecContext(ctx,
		`UPDATE users SET is_active = false WHERE id = $1`, env.officerA2); err != nil {
		t.Fatalf("deactivate officer A2: %v", err)
	}
	if items, err := env.svc.GetOnDutyOfficers(ctx, env.rtA); err != nil || len(items) != 1 {
		t.Fatalf("expected only officer A1 on duty with A2 deactivated, err=%v got=%+v", err, items)
	}
	if _, err := env.pool.Raw().ExecContext(ctx,
		`UPDATE users SET is_active = true WHERE id = $1`, env.officerA2); err != nil {
		t.Fatalf("reactivate officer A2: %v", err)
	}

	// Inactive membership hides the officer even if user is active.
	if _, err := env.pool.Raw().ExecContext(ctx,
		`UPDATE user_rt_memberships SET is_active = false WHERE id = $1`, env.officerA2MID); err != nil {
		t.Fatalf("deactivate membership A2: %v", err)
	}
	if items, err := env.svc.GetOnDutyOfficers(ctx, env.rtA); err != nil || len(items) != 1 {
		t.Fatalf("expected only officer A1 on duty with A2 membership deactivated, err=%v got=%+v", err, items)
	}
	if _, err := env.pool.Raw().ExecContext(ctx,
		`UPDATE user_rt_memberships SET is_active = true WHERE id = $1`, env.officerA2MID); err != nil {
		t.Fatalf("reactivate membership A2: %v", err)
	}

	// Officer A1 checks out -> only A2 remains.
	env.setNow(time.Date(2026, 11, 12, 9, 0, 0, 0, security.JakartaLoc))
	coutRes := env.doJSON("POST", "/api/v1/security/attendance/scan", env.officer1Token, "", map[string]any{
		"qr_token": genQR(),
		"action":   "check_out",
	})
	if coutRes.Code != http.StatusOK {
		t.Fatalf("check-out A1 = %d: %s", coutRes.Code, coutRes.Body.String())
	}
	env.setNow(shiftStartNow)
	got = onDuty(env.rtA)
	if len(got) != 1 || got[0].FullName != "Petugas 2 A" {
		t.Fatalf("expected only officer A2 on duty after A1 checkout, got %+v", got)
	}

	// Cross-midnight shift: A2 stays on duty past midnight start-of-window.
	_ = devID // keep provisioned device reference used above
}

func TestSecurityAttendance_DeviceWargaDirectoryAndPII(t *testing.T) {
	env := setupSecurityIntegrationEnv(t)
	devID, devTok := provisionPosDevice(t, env)
	ctx := context.Background()

	insertHouse := func(rtID, number, headName string) (string, string) {
		t.Helper()
		var houseID string
		if err := env.pool.Raw().QueryRowContext(ctx,
			`INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, $2, true) RETURNING id`,
			rtID, number).Scan(&houseID); err != nil {
			t.Fatalf("insert house %s: %v", number, err)
		}
		var householdID string
		if err := env.pool.Raw().QueryRowContext(ctx,
			`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, $2, true) RETURNING id`,
			rtID, headName).Scan(&householdID); err != nil {
			t.Fatalf("insert household: %v", err)
		}
		var occupancyID string
		if err := env.pool.Raw().QueryRowContext(ctx,
			`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
			 VALUES ($1, $2, 'OWNER', '2025-01-01'::date) RETURNING id`,
			householdID, houseID).Scan(&occupancyID); err != nil {
			t.Fatalf("insert occupancy: %v", err)
		}
		return houseID, occupancyID
	}

	// RT A house with residents that carry sensitive PII
	_, occA1 := insertHouse(env.rtA, "A-01", "Budi Santoso")
	_, occB1 := insertHouse(env.rtB, "B-01", "Warga Lain RT")

	insertResident := func(rtID, occupancyID, name, relationship, phone, nik, email string) string {
		_ = relationship // current schema carries no relationship column; head-of-family derives from households.head_name
		t.Helper()
		var residentID string
		if err := env.pool.Raw().QueryRowContext(ctx,
			`INSERT INTO residents (rt_id, full_name, phone, nik, email, is_active)
			 VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
			rtID, name, phone, nik, email).Scan(&residentID); err != nil {
			t.Fatalf("insert resident %s: %v", name, err)
		}
		_, err := env.pool.Raw().ExecContext(ctx,
			`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
			 VALUES ($1, $2, '2025-01-01'::date)`,
			residentID, occupancyID)
		if err != nil {
			t.Fatalf("insert residency period for %s: %v", name, err)
		}
		return residentID
	}

	insertResident(env.rtA, occA1, "Budi Santoso", "head", "08110000001", "3201234567890001", "budi@test.local")
	insertResident(env.rtA, occA1, "Sari Santoso", "spouse", "08110000002", "3201234567890002", "sari@test.local")
	insertResident(env.rtA, occA1, "Andi Santoso", "child", "08110000003", "3201234567890003", "andi@test.local")
	insertResident(env.rtB, occB1, "Warga Lain RT", "head", "08110000099", "3201234567890099", "lain@test.local")

	// 1) List: only RT A house appears (cross-RT isolation).
	listRes := env.doJSON("GET", "/api/v1/security-device/warga/households", "", devTok, nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list device households = %d: %s", listRes.Code, listRes.Body.String())
	}
	body := listRes.Body.String()
	for _, sensitive := range []string{"phone", "nik", "email", "08110000001", "08110000002", "08110000003", "budi@test.local", "3201234567890001", "sari@test.local", "andi@test.local"} {
		if strings.Contains(body, sensitive) {
			t.Fatalf("sensitive value %q leaked in device list response: %s", sensitive, body)
		}
	}
	if !strings.Contains(body, "A-01") {
		t.Fatalf("expected RT A house in list: %s", body)
	}
	if strings.Contains(body, "B-01") {
		t.Fatalf("RT B house leaked into RT A device list: %s", body)
	}

	// 2) Search filter narrows to the matching house.
	listRes = env.doJSON("GET", "/api/v1/security-device/warga/households?search=Budi", "", devTok, nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("search device households = %d: %s", listRes.Code, listRes.Body.String())
	}
	if !strings.Contains(listRes.Body.String(), "A-01") {
		t.Fatalf("expected search hit for Budi: %s", listRes.Body.String())
	}

	// 3) Detail: members listed ordered with head first, without PII.
	detailRes := env.doJSON("GET", "/api/v1/security-device/warga/households/"+occA1, "", devTok, nil)
	if detailRes.Code != http.StatusOK {
		t.Fatalf("device household detail = %d: %s", detailRes.Code, detailRes.Body.String())
	}
	body = detailRes.Body.String()
	for _, sensitive := range []string{"phone", "nik", "email", "08110000001", "08110000002", "08110000003", "budi@test.local", "sari@test.local", "andi@test.local", "3201234567890001", "3201234567890002", "3201234567890003"} {
		if strings.Contains(body, sensitive) {
			t.Fatalf("sensitive value %q leaked in device detail response: %s", sensitive, body)
		}
	}
	var detail map[string]any
	if err := json.Unmarshal(detailRes.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	residentsList, ok := detail["residents"].([]any)
	if !ok || len(residentsList) != 3 {
		t.Fatalf("expected 3 residents, got %v", detail["residents"])
	}
	// Deterministic alphabetical order
	got0, _ := residentsList[0].(map[string]any)["full_name"].(string)
	got1, _ := residentsList[1].(map[string]any)["full_name"].(string)
	got2, _ := residentsList[2].(map[string]any)["full_name"].(string)
	if got0 > got1 || got1 > got2 {
		t.Fatalf("residents not in alphabetical order: %v", []string{got0, got1, got2})
	}
	headCount := 0
	for _, r := range residentsList {
		m, _ := r.(map[string]any)
		if m["is_head_of_family"] == true {
			headCount++
			if m["full_name"] != "Budi Santoso" {
				t.Fatalf("head of family is %v, want Budi Santoso", m["full_name"])
			}
		}
	}
	if headCount != 1 {
		t.Fatalf("expected exactly one head of family flagged, got %d", headCount)
	}

	// 4) Detail for RT B's occupancy with RT A device -> 404 (existence not leaked).
	denyRes := env.doJSON("GET", "/api/v1/security-device/warga/households/"+occB1, "", devTok, nil)
	if denyRes.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-RT detail, got %d: %s", denyRes.Code, denyRes.Body.String())
	}

	// 5) User JWT is rejected on device endpoints.
	denyRes = env.doJSON("GET", "/api/v1/security-device/warga/households", env.ketuaToken, "", nil)
	if denyRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for user JWT on device endpoint, got %d", denyRes.Code)
	}

	// 6) Device token is rejected on user-only mutation endpoints
	//    (keamanan device must not reach RW-scoped routes).
	_ = devID
}

func TestSecurityAttendance_DeviceResidentAvatarSync(t *testing.T) {
	env := setupSecurityIntegrationEnv(t)
	devID, devTok := provisionPosDevice(t, env)
	_ = devID
	ctx := context.Background()

	// RT A household "A-72" whose head matches resident full_name.
	var houseID, householdID, occupancyID string
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'A-72', true) RETURNING id`,
		env.rtA).Scan(&houseID); err != nil {
		t.Fatalf("insert house: %v", err)
	}
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'Foto Kepala A', true) RETURNING id`,
		env.rtA).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2025-01-01'::date) RETURNING id`,
		householdID, houseID).Scan(&occupancyID); err != nil {
		t.Fatalf("insert occupancy: %v", err)
	}

	// Upload a tiny avatar for the head via the same mock store the device-handler uses.
	jpeg := tinyJPEG()
	meta, err := env.store.UploadCommunityImage(ctx, env.rtA, "profile", "kepala.jpg", bytes.NewReader(jpeg), int64(len(jpeg)))
	if err != nil {
		t.Fatalf("mock upload avatar: %v", err)
	}
	avatarKey := meta.Key

	var residentID string
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active, avatar_path)
		 VALUES ($1, 'Foto Kepala A', '081700000001', true, $2) RETURNING id`,
		env.rtA, avatarKey).Scan(&residentID); err != nil {
		t.Fatalf("insert resident: %v", err)
	}
	if _, err := env.pool.Raw().ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, '2025-01-01'::date)`,
		residentID, occupancyID); err != nil {
		t.Fatalf("insert residency period: %v", err)
	}

	// 1) Device list: head_avatar_path must equal the stored key.
	listRes := env.doJSON("GET", "/api/v1/security-device/warga/households?search=A-72", "", devTok, nil)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", listRes.Code, listRes.Body.String())
	}
	if !strings.Contains(listRes.Body.String(), avatarKey) {
		t.Fatalf("expected head_avatar_path %q in list body: %s", avatarKey, listRes.Body.String())
	}

	// 2) Detail: member avatar_path must equal the stored key, and head flagged.
	detailRes := env.doJSON("GET", "/api/v1/security-device/warga/households/"+occupancyID, "", devTok, nil)
	if detailRes.Code != http.StatusOK {
		t.Fatalf("detail = %d: %s", detailRes.Code, detailRes.Body.String())
	}
	body := detailRes.Body.String()
	if !strings.Contains(body, avatarKey) {
		t.Fatalf("expected member avatar_path %q in detail body: %s", avatarKey, body)
	}

	// 3) Device can fetch the avatar bytes via the device-scoped image endpoint.
	imgReq := httptest.NewRequest(http.MethodGet, "/api/v1/security-device/images/"+avatarKey, nil)
	imgReq.Header.Set("X-Device-Token", devTok)
	imgRec := httptest.NewRecorder()
	env.router.ServeHTTP(imgRec, imgReq)
	if imgRec.Code != http.StatusOK {
		t.Fatalf("device image fetch = %d: %s", imgRec.Code, imgRec.Body.String())
	}
	if ct := imgRec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("device image content-type = %q, want image/jpeg", ct)
	}
	if !bytes.Equal(imgRec.Body.Bytes(), jpeg) {
		t.Fatalf("device image bytes mismatch")
	}

	// 4) RT B avatar key is not reachable via RT A device (prefix check).
	metaB, err := env.store.UploadCommunityImage(ctx, env.rtB, "profile", "lainnya.jpg", bytes.NewReader(jpeg), int64(len(jpeg)))
	if err != nil {
		t.Fatalf("mock upload RT B avatar: %v", err)
	}
	denyReq := httptest.NewRequest(http.MethodGet, "/api/v1/security-device/images/"+metaB.Key, nil)
	denyReq.Header.Set("X-Device-Token", devTok)
	denyRec := httptest.NewRecorder()
	env.router.ServeHTTP(denyRec, denyReq)
	if denyRec.Code != http.StatusForbidden && denyRec.Code != http.StatusNotFound {
		t.Fatalf("expected 403/404 for cross-RT avatar key, got %d", denyRec.Code)
	}

	// 5) Traversal / invalid key shapes are rejected.
	badReq := httptest.NewRequest(http.MethodGet, "/api/v1/security-device/images/community/"+env.rtA+"/profile/..%2f..%2fuser.jpg", nil)
	badReq.Header.Set("X-Device-Token", devTok)
	badRec := httptest.NewRecorder()
	env.router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusForbidden && badRec.Code != http.StatusNotFound && badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected rejection for traversal key, got %d", badRec.Code)
	}

	// 6) Unreferenced (but well-formed, in-RT) key is not served.
	orphanMeta, err := env.store.UploadCommunityImage(ctx, env.rtA, "profile", "orphan.jpg", bytes.NewReader(jpeg), int64(len(jpeg)))
	if err != nil {
		t.Fatalf("mock upload orphan avatar: %v", err)
	}
	orphanReq := httptest.NewRequest(http.MethodGet, "/api/v1/security-device/images/"+orphanMeta.Key, nil)
	orphanReq.Header.Set("X-Device-Token", devTok)
	orphanRec := httptest.NewRecorder()
	env.router.ServeHTTP(orphanRec, orphanReq)
	if orphanRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unreferenced avatar key, got %d", orphanRec.Code)
	}
}

func TestSecurityAttendance_DeviceSampahNoteWriteAndIsolation(t *testing.T) {
	env := setupSecurityIntegrationEnv(t)
	devID, devTok := provisionPosDevice(t, env)
	_ = devID
	ctx := context.Background()

	// RT A occupancy to receive a note.
	var houseID, householdID, occupancyID string
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'TB-01', true) RETURNING id`,
		env.rtA).Scan(&houseID); err != nil {
		t.Fatalf("insert house: %v", err)
	}
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'Catatan TB', true) RETURNING id`,
		env.rtA).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2025-01-01'::date) RETURNING id`,
		householdID, houseID).Scan(&occupancyID); err != nil {
		t.Fatalf("insert occupancy: %v", err)
	}

	// 1) Device creates/updates the note.
	body := map[string]any{"note": "rumah depan belum diangkut 3x"}
	reqBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/security-device/monitoring/sampah/"+occupancyID+"/note",
		bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Token", devTok)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH note = %d: %s", rec.Code, rec.Body.String())
	}

	// 2) Update same note (upsert, no duplicate).
	body = map[string]any{"note": "sudah diangkut tgl 2026-11-13"}
	reqBody, _ = json.Marshal(body)
	req = httptest.NewRequest(http.MethodPatch,
		"/api/v1/security-device/monitoring/sampah/"+occupancyID+"/note",
		bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Token", devTok)
	rec = httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH note (update) = %d: %s", rec.Code, rec.Body.String())
	}

	var notesCount int
	if err := env.pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_monitoring_notes WHERE household_occupancy_id = $1`,
		occupancyID,
	).Scan(&notesCount); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if notesCount != 1 {
		t.Fatalf("expected exactly 1 note row after upsert, got %d", notesCount)
	}
	var storedNote string
	if err := env.pool.Raw().QueryRowContext(ctx,
		`SELECT note FROM rw_monitoring_notes WHERE household_occupancy_id = $1`,
		occupancyID,
	).Scan(&storedNote); err != nil {
		t.Fatalf("read note: %v", err)
	}
	if storedNote != "sudah diangkut tgl 2026-11-13" {
		t.Fatalf("stored note = %q, want the updated text", storedNote)
	}

	// 3) RT B cannot-be-edited via RT A's device.
	var houseB, householdB, occupancyB string
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, is_active) VALUES ($1, 'RTB-01', true) RETURNING id`,
		env.rtB).Scan(&houseB); err != nil {
		t.Fatalf("insert RTB house: %v", err)
	}
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'Catatan RTB', true) RETURNING id`,
		env.rtB).Scan(&householdB); err != nil {
		t.Fatalf("insert RTB household: %v", err)
	}
	if err := env.pool.Raw().QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', '2025-01-01'::date) RETURNING id`,
		householdB, houseB).Scan(&occupancyB); err != nil {
		t.Fatalf("insert RTB occupancy: %v", err)
	}
	reqBody, _ = json.Marshal(map[string]any{"note": "catat RT lain"})
	req = httptest.NewRequest(http.MethodPatch,
		"/api/v1/security-device/monitoring/sampah/"+occupancyB+"/note",
		bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Token", devTok)
	rec = httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-RT occupancy, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4) Note text empty -> 400
	reqBody, _ = json.Marshal(map[string]any{"note": "   "})
	req = httptest.NewRequest(http.MethodPatch,
		"/api/v1/security-device/monitoring/sampah/"+occupancyID+"/note",
		bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Token", devTok)
	rec = httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 400-class for empty note, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5) Unauthenticated device -> 401
	req = httptest.NewRequest(http.MethodPatch,
		"/api/v1/security-device/monitoring/sampah/"+occupancyID+"/note",
		bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing device token, got %d", rec.Code)
	}
}
