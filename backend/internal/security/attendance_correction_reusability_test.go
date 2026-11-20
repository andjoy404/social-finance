package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAttendanceCorrection_ReusableShifts_All10Scenarios(t *testing.T) {
	pool, repo, svc, rtID, officerA, officerB, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Setup Shift Configuration: Shift Pagi (06:00 - 14:00)
	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi",
		StartTime:                   "06:00",
		EndTime:                     "14:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		AbsentCutoffMinutes:         120,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift pagi: %v", err)
	}

	// 2. Assign continuous Shift Pagi to Officer A
	schedA, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("assign schedA: %v", err)
	}

	// 3. Provision Pos Device
	now := time.Now()
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos Induk Koreksi", "pos_induk_corr", "hash_corr", now.Add(time.Hour), now, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE security_devices SET is_active = true, activated_at = now() WHERE id = $1`, dev.ID)

	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	// ── Day 1 (2026-11-20): Officer A performs and completes Shift Pagi ──
	day1In := time.Date(2026, 11, 20, 6, 10, 0, 0, JakartaLoc)
	resInDay1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day1In), "check_in", "", day1In)
	if err != nil {
		t.Fatalf("day 1 check-in failed: %v", err)
	}
	day1Out := time.Date(2026, 11, 20, 14, 5, 0, 0, JakartaLoc)
	resOutDay1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day1Out), "check_out", "", day1Out)
	if err != nil {
		t.Fatalf("day 1 check-out failed: %v", err)
	}
	if resOutDay1.Record.Status != StatusSudahAbsenPulang || resOutDay1.Record.ShiftDate != "2026-11-20" {
		t.Fatalf("day 1 expected sudah_absen_pulang on 2026-11-20, got %+v", resOutDay1.Record)
	}

	// ── Day 2 (2026-11-21): Officer A performs Shift Pagi, but marked 'terlambat' or forgets check-out ──
	day2In := time.Date(2026, 11, 21, 6, 35, 0, 0, JakartaLoc) // late check-in
	resInDay2, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day2In), "check_in", "", day2In)
	if err != nil {
		t.Fatalf("day 2 check-in failed: %v", err)
	}
	if resInDay2.Record.Status != StatusTerlambat || resInDay2.Record.ShiftDate != "2026-11-21" {
		t.Fatalf("day 2 expected terlambat on 2026-11-21, got %+v", resInDay2.Record)
	}

	// ── SCENARIO 1: Security officer can submit a valid manual correction request ──
	// ── SCENARIO 2: Officer can request correction for a later date even after completing the same shift on an earlier date ──
	// ── SCENARIO 3: Correction request references the correct attendance record/date ──
	reasonDay2 := "Lupa check-out karena ada insiden gerbang darurat"
	corrReqDay2 := CreateCorrectionRequest{
		AttendanceRecordID: resInDay2.Record.ID,
		RequestType:        CorrectionTypeStatusChange,
		RequestedStatus:    StatusSudahAbsenPulang,
		Reason:             reasonDay2,
	}
	corrDay2, err := svc.CreateCorrection(ctx, rtID, officerA, false, corrReqDay2)
	if err != nil {
		t.Fatalf("scenario 1 & 2 failed: CreateCorrection for Day 2 returned error: %v", err)
	}
	if corrDay2.AttendanceRecordID != resInDay2.Record.ID {
		t.Fatalf("scenario 3 failed: expected attendance_record_id %s, got %s", resInDay2.Record.ID, corrDay2.AttendanceRecordID)
	}
	if corrDay2.ShiftDate != "2026-11-21" {
		t.Fatalf("scenario 3 failed: expected shift_date 2026-11-21, got %s", corrDay2.ShiftDate)
	}
	if corrDay2.Status != CorrectionPending {
		t.Fatalf("scenario 1 failed: expected pending status, got %s", corrDay2.Status)
	}

	// ── SCENARIO 6: Duplicate correction requests follow existing duplicate policy (409 conflict) ──
	_, err = svc.CreateCorrection(ctx, rtID, officerA, false, corrReqDay2)
	if err == nil {
		t.Fatalf("scenario 6 failed: expected duplicate correction request to be rejected with conflict")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 409 || appErr.Code != "correction_already_pending" {
		t.Fatalf("scenario 6 failed: expected 409 correction_already_pending, got: %v", err)
	}

	// ── SCENARIO 7: Unauthorized officer cannot request correction for another officer ──
	_, err = svc.CreateCorrection(ctx, rtID, officerB, false, CreateCorrectionRequest{
		AttendanceRecordID: resInDay2.Record.ID,
		RequestType:        CorrectionTypeStatusChange,
		RequestedStatus:    StatusSudahAbsenPulang,
		Reason:             "Mencoba koreksi absensi petugas lain",
	})
	if err == nil {
		t.Fatalf("scenario 7 failed: officer B should not be able to request correction for officer A")
	}
	if !errors.As(err, &appErr) || appErr.Status != 403 {
		t.Fatalf("scenario 7 failed: expected 403 forbidden, got: %v", err)
	}

	// ── SCENARIO 8: Cross-RT correction requests are rejected ──
	otherRTID := uuid.NewString()
	_, _ = pool.Raw().ExecContext(ctx, `INSERT INTO rts (id, rw, rt, name, is_active) VALUES ($1, 999, '099', 'Other RT', true)`, otherRTID)
	_, err = svc.CreateCorrection(ctx, otherRTID, officerA, false, corrReqDay2)
	if err == nil {
		t.Fatalf("scenario 8 failed: cross-RT correction should be rejected")
	}
	if !errors.As(err, &appErr) || appErr.Status != 404 {
		t.Fatalf("scenario 8 failed: expected 404 not_found across RTs, got: %v", err)
	}

	// ── SCENARIO 9: Existing approval and rejection workflows continue to work ──
	decideNote := "Disetujui setelah konfirmasi saksi"
	approvedCorr, err := svc.DecideCorrection(ctx, rtID, corrDay2.ID, adminID, DecideCorrectionRequest{
		Approve: true,
		Note:    &decideNote,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("scenario 9 approval failed: %v", err)
	}
	if approvedCorr.Status != CorrectionApproved {
		t.Fatalf("scenario 9 failed: expected approved status, got %s", approvedCorr.Status)
	}

	// Verify Day 2 record was updated in database
	var day2UpdatedStatus string
	var day2Locked bool
	err = pool.Raw().QueryRowContext(ctx, `SELECT status, correction_locked FROM security_attendance_records WHERE id = $1`, resInDay2.Record.ID).Scan(&day2UpdatedStatus, &day2Locked)
	if err != nil {
		t.Fatalf("scenario 9 fetch updated day 2 record failed: %v", err)
	}
	if day2UpdatedStatus != StatusSudahAbsenPulang || !day2Locked {
		t.Fatalf("scenario 9 failed: expected day 2 status sudah_absen_pulang with correction_locked=true, got status=%s locked=%v", day2UpdatedStatus, day2Locked)
	}

	// ── SCENARIO 4: Correction request does not overwrite another day's attendance ──
	var day1StatusAfter string
	var day1DateAfter string
	err = pool.Raw().QueryRowContext(ctx, `SELECT status, to_char(shift_date, 'YYYY-MM-DD') FROM security_attendance_records WHERE id = $1`, resInDay1.Record.ID).Scan(&day1StatusAfter, &day1DateAfter)
	if err != nil {
		t.Fatalf("scenario 4 fetch day 1 record failed: %v", err)
	}
	if day1StatusAfter != StatusSudahAbsenPulang || day1DateAfter != "2026-11-20" {
		t.Fatalf("scenario 4 failed: day 1 attendance was overwritten or corrupted! status=%s date=%s", day1StatusAfter, day1DateAfter)
	}

	// ── SCENARIO 10: Correction audit history remains accurate after the reusable shift changes ──
	var auditCount int
	var auditAction, auditEntityID string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*), action, entity_id
		FROM audit_logs
		WHERE rt_id = $1 AND entity_type = 'security_attendance_correction' AND entity_id = $2
		GROUP BY action, entity_id
	`, rtID, corrDay2.ID).Scan(&auditCount, &auditAction, &auditEntityID)
	if err != nil {
		t.Fatalf("scenario 10 fetch audit log failed: %v", err)
	}
	if auditCount < 1 || auditAction != "security_correction_approved" || auditEntityID != corrDay2.ID {
		t.Fatalf("scenario 10 failed: expected audit log for approved correction, got count=%d action=%s entityID=%s", auditCount, auditAction, auditEntityID)
	}

	// ── SCENARIO 5: Correction request does not consume the next day's normal shift slot ──
	// Day 3 (2026-11-22): Officer A must be able to perform normal attendance on Shift Pagi without being blocked
	day3In := time.Date(2026, 11, 22, 6, 8, 0, 0, JakartaLoc)
	resInDay3, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day3In), "check_in", "", day3In)
	if err != nil {
		t.Fatalf("scenario 5 failed: Day 3 normal attendance check-in failed after Day 2 correction: %v", err)
	}
	if resInDay3.Record.Status != StatusHadir || resInDay3.Record.ShiftDate != "2026-11-22" {
		t.Fatalf("scenario 5 failed: expected Day 3 normal attendance status hadir on 2026-11-22, got %+v", resInDay3.Record)
	}
	if resInDay3.Record.ScheduleID != schedA.ID {
		t.Fatalf("scenario 5 failed: expected same continuous schedule %s, got %s", schedA.ID, resInDay3.Record.ScheduleID)
	}

	// Day 3 Check out
	day3Out := time.Date(2026, 11, 22, 14, 2, 0, 0, JakartaLoc)
	resOutDay3, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day3Out), "check_out", "", day3Out)
	if err != nil {
		t.Fatalf("scenario 5 Day 3 check-out failed: %v", err)
	}
	if resOutDay3.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("scenario 5 expected Day 3 status sudah_absen_pulang, got %s", resOutDay3.Record.Status)
	}

	// ── Test Rejection Workflow on Day 3 ──
	// Create another correction request for Day 3 and verify rejection
	corrReqDay3 := CreateCorrectionRequest{
		AttendanceRecordID: resInDay3.Record.ID,
		RequestType:        CorrectionTypeStatusChange,
		RequestedStatus:    StatusHadir,
		Reason:             "Minta ubah status hadir saja",
	}
	corrDay3, err := svc.CreateCorrection(ctx, rtID, officerA, false, corrReqDay3)
	if err != nil {
		t.Fatalf("create correction day 3 failed: %v", err)
	}
	rejectNote := "Ditolak karena data log sudah sesuai"
	rejectedCorr, err := svc.DecideCorrection(ctx, rtID, corrDay3.ID, adminID, DecideCorrectionRequest{
		Approve: false,
		Note:    &rejectNote,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("decide rejection failed: %v", err)
	}
	if rejectedCorr.Status != CorrectionRejected {
		t.Fatalf("expected rejected status, got %s", rejectedCorr.Status)
	}

	// Verify Day 3 record remained unchanged after rejection
	var day3StatusAfterReject string
	err = pool.Raw().QueryRowContext(ctx, `SELECT status FROM security_attendance_records WHERE id = $1`, resInDay3.Record.ID).Scan(&day3StatusAfterReject)
	if err != nil {
		t.Fatalf("fetch day 3 record after rejection failed: %v", err)
	}
	if day3StatusAfterReject != StatusSudahAbsenPulang {
		t.Fatalf("expected day 3 status to remain sudah_absen_pulang, got %s", day3StatusAfterReject)
	}
}
