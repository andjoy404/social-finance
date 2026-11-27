package security

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"social-finance/internal/auth"
)

func TestPOSQR_AuthValidationLifecycleAndReconciliation(t *testing.T) {
	pool, repo, svc, rtID, officerA, officerB, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// Create a second RT (RT B) for cross-RT isolation checks
	rtBID := uuid.NewString()
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO rts (id, rw, rt, name, is_active) VALUES ($1, 99, $2, 'RT Cross Test', true)`,
		rtBID, "992-"+uuid.NewString()[:5],
	)
	if err != nil {
		t.Fatalf("create second RT failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtBID)
	})

	// Create Shift Pagi (06:00 - 14:00, missing checkout cutoff 60m -> expires 15:00)
	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                         "Shift Pagi Hotfix",
		StartTime:                    "06:00",
		EndTime:                      "14:00",
		CrossesMidnight:              false,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   30,
		CheckInWindowAfterMinutes:    60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift pagi failed: %v", err)
	}
	// Create Shift Siang (14:00 - 22:00) for concurrent active open session test
	shiftSiang, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                         "Shift Siang Hotfix",
		StartTime:                    "14:00",
		EndTime:                      "22:00",
		CrossesMidnight:              false,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   30,
		CheckInWindowAfterMinutes:    60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift siang failed: %v", err)
	}

	// Assign Officer A to Shift Pagi & Shift Siang; leave Officer B unscheduled
	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("assign officer A to shift pagi failed: %v", err)
	}
	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftSiang.ID,
	})
	if err != nil {
		t.Fatalf("assign officer A to shift siang failed: %v", err)
	}

	// 1. Device auth lifecycle: provision, activate, valid token, invalid token, revoked device
	prov, err := svc.CreateDevice(ctx, rtID, adminID, CreateDeviceRequest{
		DeviceName:    "Pos Gerbang Hotfix",
		LocationLabel: "Gerbang Utara",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	activated, err := svc.ActivateDevice(ctx, ActivateDeviceRequest{PairingToken: prov.PairingToken}, "127.0.0.1")
	if err != nil {
		t.Fatalf("activate device failed: %v", err)
	}

	h := NewHandler(svc, pool, nil, nil)
	posRouter := chi.NewRouter()
	posRouter.Use(DeviceAuthMiddleware(repo))
	posRouter.Post("/api/v1/pos/qr/generate", h.PosGenerateQR)

	// 1a. Valid device token -> 201 Created
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pos/qr/generate", nil)
	req.Header.Set("X-Device-Token", activated.DeviceToken)
	rec := httptest.NewRecorder()
	posRouter.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 from PosGenerateQR with valid device token, got %d: %s", rec.Code, rec.Body.String())
	}

	// 1b. Invalid device token -> 401
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/pos/qr/generate", nil)
	reqBad.Header.Set("X-Device-Token", "invalid-device-token")
	recBad := httptest.NewRecorder()
	posRouter.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid device token, got %d", recBad.Code)
	}

	// 1c. Revoked device -> 401
	provRevoked, err := svc.CreateDevice(ctx, rtID, adminID, CreateDeviceRequest{
		DeviceName:    "Pos Revoked",
		LocationLabel: "Gerbang Selatan",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("create revoked device failed: %v", err)
	}
	actRevoked, err := svc.ActivateDevice(ctx, ActivateDeviceRequest{PairingToken: provRevoked.PairingToken}, "127.0.0.1")
	if err != nil {
		t.Fatalf("activate revoked device failed: %v", err)
	}
	if _, err := svc.RevokeDevice(ctx, rtID, provRevoked.Device.ID, adminID, "127.0.0.1"); err != nil {
		t.Fatalf("revoke device failed: %v", err)
	}
	reqRev := httptest.NewRequest(http.MethodPost, "/api/v1/pos/qr/generate", nil)
	reqRev.Header.Set("X-Device-Token", actRevoked.DeviceToken)
	recRev := httptest.NewRecorder()
	posRouter.ServeHTTP(recRev, reqRev)
	if recRev.Code != http.StatusUnauthorized && recRev.Code != http.StatusForbidden {
		t.Fatalf("expected 401/403 for revoked device, got %d", recRev.Code)
	}

	makeQR := func(targetRT, devID string, at time.Time, ttl time.Duration) string {
		raw := uuid.NewString()
		_, err := repo.CreateQRToken(ctx, targetRT, devID, HashToken(raw), at, at.Add(ttl))
		if err != nil {
			t.Fatalf("CreateQRToken failed: %v", err)
		}
		return raw
	}

	// Create a device in RT B for cross-RT QR check
	devB, err := repo.CreateDevice(ctx, rtBID, adminID, "Pos RT B", "Gate B", HashToken("pair-b"), time.Now().Add(time.Hour), time.Now(), "127.0.0.1")
	if err != nil {
		t.Fatalf("create RT B device failed: %v", err)
	}

	day1CheckIn := time.Date(2026, 11, 21, 6, 5, 0, 0, JakartaLoc)

	// 2. Expired QR -> 400 qr_expired
	expiredRaw := makeQR(rtID, prov.Device.ID, day1CheckIn.Add(-10*time.Minute), 1*time.Minute)
	if _, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, expiredRaw, "auto", "", "", day1CheckIn); err == nil {
		t.Fatal("expected error for expired QR")
	} else if appErr, ok := err.(*AppError); !ok || appErr.Status != http.StatusBadRequest {
		t.Fatalf("expected 400 AppError for expired QR, got %T %v", err, err)
	}

	// 3. Wrong RT QR -> 403 cross_rt_denied
	wrongRTRaw := makeQR(rtBID, devB.ID, day1CheckIn, 2*time.Minute)
	if _, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, wrongRTRaw, "auto", "", "", day1CheckIn); err == nil {
		t.Fatal("expected error for cross-RT QR")
	} else if appErr, ok := err.(*AppError); !ok || appErr.Status != http.StatusForbidden {
		t.Fatalf("expected 403 AppError for cross-RT QR, got %T %v", err, err)
	}

	// 4. Unscheduled officer -> 404/400 (NOT 500)
	unschedRaw := makeQR(rtID, prov.Device.ID, day1CheckIn, 2*time.Minute)
	if _, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, unschedRaw, "auto", "", "", day1CheckIn); err == nil {
		t.Fatal("expected error for unscheduled user")
	} else if appErr, ok := err.(*AppError); !ok || appErr.Status >= 500 {
		t.Fatalf("expected 4xx AppError for unscheduled user, got %T %v", err, err)
	}

	// 5. Valid check-in + Idempotency replay + Used QR rejection
	day1QR := makeQR(rtID, prov.Device.ID, day1CheckIn, 2*time.Minute)
	idemKey := "idem-qr-scan-" + uuid.NewString()
	res1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, day1QR, "auto", shiftPagi.ID, idemKey, day1CheckIn)
	if err != nil {
		t.Fatalf("valid Day 1 check-in failed: %v", err)
	}
	if res1.Record.Status != StatusHadir {
		t.Fatalf("expected hadir, got %s", res1.Record.Status)
	}

	// Idempotent replay returns same record without duplicate
	resReplay, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, day1QR, "auto", shiftPagi.ID, idemKey, day1CheckIn)
	if err != nil {
		t.Fatalf("idempotent replay failed: %v", err)
	}
	if resReplay.Record.ID != res1.Record.ID {
		t.Fatalf("expected same record ID %s on idempotent replay, got %s", res1.Record.ID, resReplay.Record.ID)
	}

	// Used QR with a DIFFERENT idempotency key -> 409/400 (NOT 500)
	if _, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, day1QR, "auto", shiftPagi.ID, "other-idem-"+uuid.NewString(), day1CheckIn); err == nil {
		t.Fatal("expected error when reusing consumed QR token")
	} else if appErr, ok := err.(*AppError); !ok || appErr.Status >= 500 {
		t.Fatalf("expected 4xx AppError for reused QR, got %T %v", err, err)
	}

	// 6. Concurrent ACTIVE open session (same day, Shift Pagi not yet expired) -> 409 Conflict (NOT 500!)
	// Shift Pagi ends 14:00 (cutoff 15:00); at 13:45 Shift Siang (14:00) check-in window is open while Shift Pagi is still open.
	concurrentTime := time.Date(2026, 11, 21, 13, 45, 0, 0, JakartaLoc)
	concurrentQR := makeQR(rtID, prov.Device.ID, concurrentTime, 2*time.Minute)
	if _, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, concurrentQR, "check_in", shiftSiang.ID, "", concurrentTime); err == nil {
		t.Fatal("expected 409 Conflict for concurrent active open session")
	} else if appErr, ok := err.(*AppError); !ok || appErr.Status != http.StatusConflict {
		t.Fatalf("expected 409 AppError for concurrent active open session, got %T %v", err, err)
	}

	// 7. CRITICAL REGRESSION: Mark Day 1 open session as correction_locked = true (simulating approved check-in-only correction).
	// On Day 2 (2026-11-22 06:05), Day 1 (cutoff 2026-11-21 15:00) is expired.
	// Previously this crashed with PG 23505 uniq_security_attendance_user_open_session -> HTTP 500 "internal security module error"!
	if _, err := pool.Raw().ExecContext(ctx,
		`UPDATE security_attendance_records SET correction_locked = true WHERE id = $1`,
		res1.Record.ID,
	); err != nil {
		t.Fatalf("lock Day 1 record failed: %v", err)
	}

	day2CheckIn := time.Date(2026, 11, 22, 6, 5, 0, 0, JakartaLoc)
	day2QR := makeQR(rtID, prov.Device.ID, day2CheckIn, 2*time.Minute)
	resDay2, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, day2QR, "auto", shiftPagi.ID, "", day2CheckIn)
	if err != nil {
		t.Fatalf("REGRESSION: Day 2 QR check-in after expired correction_locked open session MUST succeed, got: %v", err)
	}
	if resDay2.Record.Status != StatusHadir {
		t.Fatalf("expected Day 2 status hadir, got %s", resDay2.Record.Status)
	}

	// Verify Day 1 expired record was cleanly reconciled to belum_absen_pulang
	var day1Status string
	if err := pool.Raw().QueryRowContext(ctx,
		`SELECT status FROM security_attendance_records WHERE id = $1`,
		res1.Record.ID,
	).Scan(&day1Status); err != nil {
		t.Fatalf("read reconciled Day 1 status failed: %v", err)
	}
	if day1Status != StatusBelumAbsenPulang {
		t.Fatalf("expected expired Day 1 session reconciled to %s, got %s", StatusBelumAbsenPulang, day1Status)
	}

	// 8. Day 2 valid check-out succeeds
	day2CheckOut := time.Date(2026, 11, 22, 14, 5, 0, 0, JakartaLoc)
	day2OutQR := makeQR(rtID, prov.Device.ID, day2CheckOut, 2*time.Minute)
	resDay2Out, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, day2OutQR, "auto", "", "", day2CheckOut)
	if err != nil {
		t.Fatalf("Day 2 check-out failed: %v", err)
	}
	if resDay2Out.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("expected sudah_absen_pulang, got %s", resDay2Out.Record.Status)
	}
}

func TestCorrectionApproval_RBACNotesAndReconciliation(t *testing.T) {
	pool, repo, svc, rtID, officerA, _, ketuaID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// Create helper users in RT A: Humas, Sekretaris (to test dynamic permission), Warga (no permission)
	createUserInRT := func(targetRT, email, name, role, jabatan string) (string, string) {
		uid := uuid.NewString()
		mid := uuid.NewString()
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, full_name, is_active)
			VALUES ($1, $2, 'hash', $3, true)
		`, uid, email, name)
		if err != nil {
			t.Fatalf("create user %s failed: %v", jabatan, err)
		}
		_, err = pool.Raw().ExecContext(ctx, `
			INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), true)
		`, mid, uid, targetRT, role, jabatan)
		if err != nil {
			t.Fatalf("create membership %s failed: %v", jabatan, err)
		}
		t.Cleanup(func() {
			_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE id = $1`, mid)
			_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM users WHERE id = $1`, uid)
		})
		return uid, mid
	}

	ketuaRTID, ketuaRTMid := createUserInRT(rtID, "ketua-hotfix-"+uuid.NewString()+"@test.local", "Ketua RT", "pengurus", "ketua")
	humasID, humasMid := createUserInRT(rtID, "humas-hotfix-"+uuid.NewString()+"@test.local", "Humas RT", "pengurus", "humas")
	sekretarisID, sekretarisMid := createUserInRT(rtID, "sek-hotfix-"+uuid.NewString()+"@test.local", "Sekretaris RT", "pengurus", "sekretaris")
	wargaID, wargaMid := createUserInRT(rtID, "warga-hotfix-"+uuid.NewString()+"@test.local", "Warga RT", "warga", "")

	// Create RT B + Ketua RT B for cross-RT denial test
	rtBID := uuid.NewString()
	_, err := pool.Raw().ExecContext(ctx,
		`INSERT INTO rts (id, rw, rt, name, is_active) VALUES ($1, 99, $2, 'RT Cross Corr', true)`,
		rtBID, "993-"+uuid.NewString()[:5],
	)
	if err != nil {
		t.Fatalf("create RT B failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtBID)
	})
	ketuaRTBID, ketuaRTBMid := createUserInRT(rtBID, "ketua-rtb-"+uuid.NewString()+"@test.local", "Ketua RT B", "pengurus", "ketua")

	// Dynamically grant `security.correction` to `sekretaris` (proving non-hardcoded jabatan works!)
	var hadSekPerm bool
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM position_permissions pp
			JOIN permission_types pt ON pt.id = pp.permission_id
			WHERE pp.position = 'sekretaris' AND pt.code = 'security.correction'
		)
	`).Scan(&hadSekPerm)
	if !hadSekPerm {
		_, err = pool.Raw().ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT 'sekretaris', id FROM permission_types WHERE code = 'security.correction'
			ON CONFLICT DO NOTHING
		`)
		if err != nil {
			t.Fatalf("grant security.correction to sekretaris failed: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Raw().ExecContext(ctx, `
				DELETE FROM position_permissions
				WHERE position = 'sekretaris'
				  AND permission_id = (SELECT id FROM permission_types WHERE code = 'security.correction')
			`)
		})
	}

	// Setup Shift + Schedule
	shiftPagi, err := svc.CreateShift(ctx, rtID, ketuaID, UpsertShiftRequest{
		Name:                         "Shift Pagi Koreksi",
		StartTime:                    "06:00",
		EndTime:                      "14:00",
		CrossesMidnight:              false,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   30,
		CheckInWindowAfterMinutes:    60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}
	sched, err := svc.CreateSchedule(ctx, rtID, ketuaID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("create schedule failed: %v", err)
	}

	// Insert an EXPIRED open attendance session from Day 1 with correction_locked = true
	day1In := time.Date(2026, 11, 15, 6, 0, 0, 0, JakartaLoc)
	day1RecID := uuid.NewString()
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO security_attendance_records
			(id, rt_id, schedule_id, shift_config_id, user_id, shift_date, status, checked_in_at, correction_locked)
		VALUES ($1, $2, $3, $4, $5, '2026-11-15', 'hadir', $6, true)
	`, day1RecID, rtID, sched.ID, shiftPagi.ID, officerA, day1In)
	if err != nil {
		t.Fatalf("insert Day 1 locked open record failed: %v", err)
	}

	// Helper to insert a Day N attendance record (tidak_hadir) and a pending correction
	makeCorrection := func(shiftDate, reason string, reqStatus string, reqIn *time.Time) string {
		recID := uuid.NewString()
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO security_attendance_records
				(id, rt_id, schedule_id, shift_config_id, user_id, shift_date, status, correction_locked)
			VALUES ($1, $2, $3, $4, $5, $6, 'tidak_hadir', false)
		`, recID, rtID, sched.ID, shiftPagi.ID, officerA, shiftDate)
		if err != nil {
			t.Fatalf("insert attendance record for %s failed: %v", shiftDate, err)
		}
		var reqInStr *string
		if reqIn != nil {
			s := reqIn.Format(time.RFC3339)
			reqInStr = &s
		}
		corr, err := svc.CreateCorrection(ctx, rtID, officerA, false, CreateCorrectionRequest{
			AttendanceRecordID:   recID,
			RequestType:          "status_change",
			RequestedStatus:      reqStatus,
			RequestedCheckedInAt: reqInStr,
			Reason:               reason,
		})
		if err != nil {
			t.Fatalf("CreateCorrection failed: %v", err)
		}
		return corr.ID
	}

	// Mount HTTP handler exactly as in main.go (NO hardcoded jabatan RequireAccess!)
	h := NewHandler(svc, pool, nil, nil)
	r := chi.NewRouter()
	r.Post("/api/v1/security/corrections/{id}/decide", h.DecideCorrection)
	r.Get("/api/v1/security/corrections", h.ListCorrections)

	callDecide := func(ac *auth.AuthContext, corrID string, approve bool, note string) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(map[string]any{
			"approve": approve,
			"note":    note,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/security/corrections/"+corrID+"/decide", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.AuthContextKey, ac))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// A. CRITICAL REGRESSION: Humas approves Day 2 correction (setting open `hadir`) even while expired Day 1 locked row exists!
	day2In := time.Date(2026, 11, 16, 6, 10, 0, 0, JakartaLoc)
	corrHumasApprove := makeCorrection("2026-11-16", "HP mati saat scan pagi", StatusHadir, &day2In)
	humasAC := &auth.AuthContext{UserID: humasID, MembershipID: humasMid, TenantRole: auth.RolePengurus, Jabatan: auth.JabatanHumas, RTID: rtID}
	rec := callDecide(humasAC, corrHumasApprove, true, "Terverifikasi oleh Humas via CCTV")
	if rec.Code != http.StatusOK {
		t.Fatalf("REGRESSION: Humas approve MUST return 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify already-decided correction returns 409 Conflict
	recDup := callDecide(humasAC, corrHumasApprove, true, "Coba ulang")
	if recDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 for already-decided correction, got %d: %s", recDup.Code, recDup.Body.String())
	}

	// B. Humas rejects a correction with approver note -> 200
	corrHumasReject := makeCorrection("2026-11-17", "Lupa absen siang", StatusHadir, nil)
	recRej := callDecide(humasAC, corrHumasReject, false, "Tidak ditemukan di pos saat jam shift")
	if recRej.Code != http.StatusOK {
		t.Fatalf("Humas reject MUST return 200, got %d: %s", recRej.Code, recRej.Body.String())
	}

	// C. Ketua RT approves a correction -> 200
	day3In := time.Date(2026, 11, 18, 6, 5, 0, 0, JakartaLoc)
	corrKetuaApprove := makeCorrection("2026-11-18", "QR pos sedang restart", StatusHadir, &day3In)
	ketuaAC := &auth.AuthContext{UserID: ketuaRTID, MembershipID: ketuaRTMid, TenantRole: auth.RolePengurus, Jabatan: auth.JabatanKetua, RTID: rtID}
	recKetua := callDecide(ketuaAC, corrKetuaApprove, true, "Disetujui Ketua RT")
	if recKetua.Code != http.StatusOK {
		t.Fatalf("Ketua RT approve MUST return 200, got %d: %s", recKetua.Code, recKetua.Body.String())
	}

	// D. Dynamic permission: Sekretaris (granted security.correction) approves a correction -> 200
	day4In := time.Date(2026, 11, 19, 6, 3, 0, 0, JakartaLoc)
	corrSekApprove := makeCorrection("2026-11-19", "Koneksi jaringan pos terputus", StatusHadir, &day4In)
	sekAC := &auth.AuthContext{UserID: sekretarisID, MembershipID: sekretarisMid, TenantRole: auth.RolePengurus, Jabatan: auth.JabatanSekretaris, RTID: rtID}
	recSek := callDecide(sekAC, corrSekApprove, true, "Diverifikasi Sekretaris")
	if recSek.Code != http.StatusOK {
		t.Fatalf("Sekretaris with security.correction MUST return 200, got %d: %s", recSek.Code, recSek.Body.String())
	}

	// E. Unauthorized user (Warga without permission) -> 403 Forbidden
	corrUnauth := makeCorrection("2026-11-20", "Alasan warga test", StatusHadir, nil)
	wargaAC := &auth.AuthContext{UserID: wargaID, MembershipID: wargaMid, TenantRole: auth.RoleWarga, RTID: rtID}
	recWarga := callDecide(wargaAC, corrUnauth, true, "Tidak boleh")
	if recWarga.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized warga, got %d: %s", recWarga.Code, recWarga.Body.String())
	}

	// F. Cross-RT user (Ketua RT B) -> 404 (not found in RT B scope)
	ketuaRTBAC := &auth.AuthContext{UserID: ketuaRTBID, MembershipID: ketuaRTBMid, TenantRole: auth.RolePengurus, Jabatan: auth.JabatanKetua, RTID: rtBID}
	recCross := callDecide(ketuaRTBAC, corrUnauth, true, "Cross RT")
	if recCross.Code != http.StatusNotFound && recCross.Code != http.StatusForbidden {
		t.Fatalf("expected 404/403 for cross-RT decide, got %d: %s", recCross.Code, recCross.Body.String())
	}

	// G. Verify requester `reason` AND approver `decision_note` are BOTH persisted and returned by ListCorrections
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/corrections", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.AuthContextKey, humasAC))
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListCorrections failed: %d %s", listRec.Code, listRec.Body.String())
	}
	var listResp struct {
		Data []AttendanceCorrection `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode ListCorrections failed: %v", err)
	}

	byID := map[string]AttendanceCorrection{}
	for _, c := range listResp.Data {
		byID[c.ID] = c
	}
	gotApproved, ok := byID[corrHumasApprove]
	if !ok {
		t.Fatalf("approved correction %s missing from ListCorrections", corrHumasApprove)
	}
	if gotApproved.Reason != "HP mati saat scan pagi" {
		t.Fatalf("expected requester reason preserved, got %q", gotApproved.Reason)
	}
	if gotApproved.DecisionNote == nil || *gotApproved.DecisionNote != "Terverifikasi oleh Humas via CCTV" {
		t.Fatalf("expected approver decision_note preserved, got %v", gotApproved.DecisionNote)
	}
	gotRejected, ok := byID[corrHumasReject]
	if !ok {
		t.Fatalf("rejected correction %s missing from ListCorrections", corrHumasReject)
	}
	if gotRejected.DecisionNote == nil || *gotRejected.DecisionNote != "Tidak ditemukan di pos saat jam shift" {
		t.Fatalf("expected rejected decision_note preserved, got %v", gotRejected.DecisionNote)
	}

	// H. Verify audit_logs entries recorded for both approve and reject
	var auditCount int
	if err := pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE rt_id = $1
		  AND action IN ('security_correction_approved', 'security_correction_rejected')
		  AND entity_id IN ($2, $3)
	`, rtID, corrHumasApprove, corrHumasReject).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_logs failed: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("expected 2 audit_logs rows for approve & reject, got %d", auditCount)
	}
	_ = repo
}
