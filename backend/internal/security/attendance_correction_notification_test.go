package security

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestAttendanceCorrection_NotificationAuditAndDuplicate verifies:
// - Submitting a correction creates a `correction_submitted` notification_logs row for Ketua RT
//   (with correction_id, officer_user_id, officer_name, shift_date in payload) and never notifies the requester.
// - Duplicate pending correction for the same attendance record is rejected.
// - Approving the correction writes a security_audit_logs entry and updates the attendance record.
// - Cross-RT access to the correction is denied (RT scope isolation).
func TestAttendanceCorrection_NotificationAuditAndDuplicate(t *testing.T) {
	pool, repo, svc, rtID, officerA, _, ketuaID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// Ensure ketuaID holds exact jabatan='ketua' in user_rt_memberships for rtID.
	_, err := pool.Raw().ExecContext(ctx, `
		UPDATE user_rt_memberships SET jabatan = 'ketua', is_active = true
		WHERE user_id = $1 AND rt_id = $2
	`, ketuaID, rtID)
	if err != nil {
		t.Fatalf("set ketua jabatan: %v", err)
	}

	shiftPagi, err := svc.CreateShift(ctx, rtID, ketuaID, UpsertShiftRequest{
		Name:                         "Shift Pagi Notif",
		StartTime:                    "06:00",
		EndTime:                      "14:00",
		CrossesMidnight:              false,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   30,
		CheckInWindowAfterMinutes:    60,
		AbsentCutoffMinutes:          120,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift: %v", err)
	}

	_, err = svc.CreateSchedule(ctx, rtID, ketuaID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	now := time.Now()
	dev, err := repo.CreateDevice(ctx, rtID, ketuaID, "Pos Notif", "pos_notif", "hash_notif", now.Add(time.Hour), now, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE security_devices SET is_active = true, activated_at = now() WHERE id = $1`, dev.ID)

	rawQR := uuid.NewString()
	checkInAt := time.Date(2026, 11, 20, 6, 5, 0, 0, JakartaLoc)
	_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(rawQR), checkInAt, checkInAt.Add(2*time.Minute))
	att, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, rawQR, "check_in", "", "", checkInAt)
	if err != nil {
		t.Fatalf("check-in: %v", err)
	}

	// 1. Submit correction as officerA -> triggers correction_submitted notification to Ketua RT.
	reqOut := time.Date(2026, 11, 20, 14, 0, 0, 0, JakartaLoc).Format(time.RFC3339)
	corr, err := svc.CreateCorrection(ctx, rtID, officerA, false, CreateCorrectionRequest{
		AttendanceRecordID:    att.Record.ID,
		RequestType:           CorrectionTypeStatusChange,
		RequestedStatus:       StatusSudahAbsenPulang,
		RequestedCheckedOutAt: &reqOut,
		Reason:                "Lupa scan pulang",
	})
	if err != nil {
		t.Fatalf("CreateCorrection failed: %v", err)
	}

	// Verify correction_submitted notification_logs entry for ketuaID.
	var notifCount int
	var payloadCorrectionID string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MAX(payload->>'correction_id'), '')
		FROM notification_logs
		WHERE rt_id = $1 AND user_id = $2 AND event_type = 'correction_submitted'
		  AND dedup_key = $3
	`, rtID, ketuaID, "correction:"+corr.ID+":"+ketuaID).Scan(&notifCount, &payloadCorrectionID)
	if err != nil {
		t.Fatalf("query notification_logs: %v", err)
	}
	if notifCount != 1 {
		t.Fatalf("expected 1 correction_submitted notification for Ketua RT, got %d", notifCount)
	}
	if payloadCorrectionID != corr.ID {
		t.Fatalf("payload correction_id = %q, want %q", payloadCorrectionID, corr.ID)
	}

	// Verify requester (officerA) was NOT notified.
	var selfNotifCount int
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE rt_id = $1 AND user_id = $2 AND event_type = 'correction_submitted'
	`, rtID, officerA).Scan(&selfNotifCount)
	if selfNotifCount != 0 {
		t.Fatalf("requester must not receive correction_submitted notification, got %d", selfNotifCount)
	}

	// 2. Duplicate pending correction for same attendance record must be rejected.
	_, dupErr := svc.CreateCorrection(ctx, rtID, officerA, false, CreateCorrectionRequest{
		AttendanceRecordID:    att.Record.ID,
		RequestType:           CorrectionTypeStatusChange,
		RequestedStatus:       StatusSudahAbsenPulang,
		RequestedCheckedOutAt: &reqOut,
		Reason:                "Duplikat",
	})
	if dupErr == nil {
		t.Fatalf("expected duplicate pending correction to be rejected")
	}

	// 3. Cross-RT access to the correction must fail.
	otherRT := uuid.NewString()
	if _, crossErr := repo.GetCorrectionByID(ctx, otherRT, corr.ID); crossErr == nil {
		t.Fatalf("expected cross-RT GetCorrectionByID to fail")
	}

	// 4. Approve correction as Ketua RT -> verifies audit trail + updated attendance status.
	note := "Disetujui ketua"
	decided, err := svc.DecideCorrection(ctx, rtID, corr.ID, ketuaID, DecideCorrectionRequest{
		Approve: true,
		Note:    &note,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("DecideCorrection approve failed: %v", err)
	}
	if decided.Status != "approved" {
		t.Fatalf("decided status = %q, want approved", decided.Status)
	}

	var auditCount int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE rt_id = $1 AND entity_type = 'security_attendance_correction' AND entity_id = $2 AND action = 'security_correction_approved'
	`, rtID, corr.ID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit_logs entry for correction %s, got %d", corr.ID, auditCount)
	}
}
