package security

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

// setupDateIndependentTestEnv creates an isolated test environment with fresh RT and officers.
func setupDateIndependentTestEnv(t *testing.T) (*database.Pool, *Repository, *Service, string, string, string, string) {
	t.Helper()

	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	rtID := uuid.NewString()
	rwNum := int(time.Now().UnixNano() & 0x7fff)
	if rwNum == 0 {
		rwNum = 99
	}

	rtCode := fmt.Sprintf("007-%s", uuid.NewString()[:6])
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO rts (id, rw, rt, name, is_active)
		VALUES ($1, $2, $3, 'RT 007 Date-Independent Shift Test', true)
	`, rtID, rwNum, rtCode)
	if err != nil {
		t.Fatalf("create test rt: %v", err)
	}

	createOfficer := func(name, email string) (string, string) {
		uid := uuid.NewString()
		mid := uuid.NewString()
		_, err := pool.Raw().ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, full_name, is_active)
			VALUES ($1, $2, 'hash', $3, true)
		`, uid, email, name)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		_, err = pool.Raw().ExecContext(ctx, `
			INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
			VALUES ($1, $2, $3, 'pengurus', 'keamanan', true)
		`, mid, uid, rtID)
		if err != nil {
			t.Fatalf("create membership: %v", err)
		}
		return uid, mid
	}

	officerA, _ := createOfficer("Budi Keamanan", fmt.Sprintf("budi_%s@example.com", rtID[:8]))
	officerB, _ := createOfficer("Agus Keamanan", fmt.Sprintf("agus_%s@example.com", rtID[:8]))
	adminID, _ := createOfficer("Ketua RT", fmt.Sprintf("ketua_%s@example.com", rtID[:8]))

	repo := NewRepository(pool)
	svc := NewService(repo)

	return pool, repo, svc, rtID, officerA, officerB, adminID
}

func TestDateIndependent_ContinuousShiftAssignmentCreation(t *testing.T) {
	_, _, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Create Shift Pagi: 06:00 - 14:00
	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                       "Shift Pagi",
		StartTime:                  "06:00",
		EndTime:                    "14:00",
		CrossesMidnight:            false,
		LatenessToleranceMinutes:   15,
		CheckInWindowBeforeMinutes: 60,
		CheckInWindowAfterMinutes:  120,
	})
	if err != nil {
		t.Fatalf("create shift pagi failed: %v", err)
	}

	// 2. Assign Officer A to Shift Pagi WITHOUT ANY DATE (start_date="", end_date="", shift_date="")
	sched, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
		// No StartDate, No EndDate, No ShiftDate!
	})
	if err != nil {
		t.Fatalf("expected date-independent assignment to succeed, got: %v", err)
	}

	if sched.ShiftConfigID != shiftPagi.ID {
		t.Errorf("expected shift config %s, got %s", shiftPagi.ID, sched.ShiftConfigID)
	}
	if !sched.IsActive {
		t.Errorf("expected continuous assignment to be active, got false")
	}
	if sched.ShiftDate != "" {
		t.Errorf("expected shift date to be empty for date-independent assignment, got %s", sched.ShiftDate)
	}

	// 3. Verify duplicate active assignment for same officer and shift is rejected with 409 Conflict
	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err == nil {
		t.Fatalf("expected duplicate active assignment for same shift to be rejected with conflict")
	}

	// 4. Verify ListSchedules returns the active continuous assignment
	list, total, err := svc.ListSchedules(ctx, rtID, ScheduleFilter{
		UserID: officerA,
		Status: ScheduleStatusScheduled,
	})
	if err != nil {
		t.Fatalf("list schedules failed: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected 1 active assignment, got total=%d len=%d", total, len(list))
	}
	if !list[0].IsActive {
		t.Errorf("expected list[0].IsActive = true")
	}
}

func TestDateIndependent_CrossMidnightShiftAttendanceSingleRecord(t *testing.T) {
	pool, repo, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Create Shift Malam (22:00 - 06:00, crosses midnight)
	shiftMalam, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                         "Shift Malam",
		StartTime:                    "22:00",
		EndTime:                      "06:00",
		CrossesMidnight:              true,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   60,
		CheckInWindowAfterMinutes:    120,
		MissingCheckoutCutoffMinutes: 180,
	})
	if err != nil {
		t.Fatalf("create shift malam failed: %v", err)
	}

	// 2. Assign Officer A to Shift Malam (continuous, no date)
	sched, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftMalam.ID,
	})
	if err != nil {
		t.Fatalf("create schedule assignment failed: %v", err)
	}

	// 3. Create POS device & pairing token for check-in / check-out
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos Gerbang", "Pos 1", "hash", time.Now().Add(time.Hour), time.Now(), "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	// Activate pos device directly in test
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE security_devices SET is_active = true, activated_at = now() WHERE id = $1`, dev.ID)

	// Simulate Check-In on Day 1 (Nov 17 at 22:15 WIB)
	checkInTime := time.Date(2026, 11, 17, 22, 15, 0, 0, JakartaLoc)
	rawQRIn := uuid.NewString()
	tokenHashIn := HashToken(rawQRIn)
	_, err = repo.CreateQRToken(ctx, rtID, dev.ID, tokenHashIn, checkInTime, checkInTime.Add(time.Minute))
	if err != nil {
		t.Fatalf("create qr in failed: %v", err)
	}

	inRes, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, rawQRIn, "check_in", "", checkInTime)
	if err != nil {
		t.Fatalf("cross-midnight check-in failed: %v", err)
	}
	if inRes.Record.Status != StatusHadir {
		t.Errorf("expected status 'hadir', got %s", inRes.Record.Status)
	}
	if inRes.Record.ShiftDate != "2026-11-17" {
		t.Errorf("expected shift_date derived as '2026-11-17', got %s", inRes.Record.ShiftDate)
	}

	// 4. Verify "Sedang Bertugas" at 03:00 WIB on Day 2 (Nov 18)
	duringShiftTime := time.Date(2026, 11, 18, 3, 0, 0, 0, JakartaLoc)
	onDuty, err := repo.GetOnDutyOfficers(ctx, rtID, duringShiftTime)
	if err != nil {
		t.Fatalf("get on duty failed: %v", err)
	}
	if len(onDuty) != 1 || onDuty[0].UserID != officerA {
		t.Fatalf("expected officerA to be on duty at 03:00 Nov 18, got %+v", onDuty)
	}

	// 5. Simulate Check-Out on Day 2 (Nov 18 at 06:05 WIB)
	checkOutTime := time.Date(2026, 11, 18, 6, 5, 0, 0, JakartaLoc)
	rawQROut := uuid.NewString()
	tokenHashOut := HashToken(rawQROut)
	_, err = repo.CreateQRToken(ctx, rtID, dev.ID, tokenHashOut, checkOutTime, checkOutTime.Add(time.Minute))
	if err != nil {
		t.Fatalf("create qr out failed: %v", err)
	}

	outRes, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, rawQROut, "check_out", "", checkOutTime)
	if err != nil {
		t.Fatalf("cross-midnight check-out failed: %v", err)
	}

	// PROOF: It must be the EXACT SAME attendance record, not a new record or calendar lookup error!
	if outRes.Record.ID != inRes.Record.ID {
		t.Fatalf("expected identical record ID for check-in and check-out, got in=%s out=%s", inRes.Record.ID, outRes.Record.ID)
	}
	if outRes.Record.Status != StatusSudahAbsenPulang {
		t.Errorf("expected status 'sudah_absen_pulang', got %s", outRes.Record.Status)
	}
	if outRes.Record.CheckedInAt == nil || outRes.Record.CheckedOutAt == nil {
		t.Fatalf("expected both checked_in_at and checked_out_at to be present, got %+v", outRes.Record)
	}

	// 6. After check-out, officer is no longer "Sedang Bertugas"
	afterShiftTime := time.Date(2026, 11, 18, 7, 0, 0, 0, JakartaLoc)
	onDutyAfter, err := repo.GetOnDutyOfficers(ctx, rtID, afterShiftTime)
	if err != nil {
		t.Fatalf("get on duty after failed: %v", err)
	}
	if len(onDutyAfter) != 0 {
		t.Errorf("expected 0 officers on duty after checkout, got %d", len(onDutyAfter))
	}

	// Suppress unused warning
	_ = sched
}

func TestDateIndependent_ShiftSwapBetweenOfficers(t *testing.T) {
	_, repo, svc, rtID, officerA, officerB, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Create Shift Pagi and Shift Malam
	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:      "Shift Pagi",
		StartTime: "06:00",
		EndTime:   "14:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	shiftMalam, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:            "Shift Malam",
		StartTime:       "22:00",
		EndTime:         "06:00",
		CrossesMidnight: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Assign Officer A -> Shift Pagi; Officer B -> Shift Malam
	schedA, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	schedB, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerB,
		ShiftConfigID: shiftMalam.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Officer A proposes shift swap with Officer B
	swap, err := svc.CreateSwap(ctx, rtID, officerA, CreateSwapRequest{
		RequesterScheduleID: schedA.ID,
		TargetScheduleID:    schedB.ID,
		Reason:              "Tukar shift minggu ini",
	})
	if err != nil {
		t.Fatalf("create swap failed: %v", err)
	}
	if swap.Status != SwapPendingTarget {
		t.Errorf("expected pending_target, got %s", swap.Status)
	}

	// 4. Officer B accepts
	swap, err = svc.TargetRespondSwap(ctx, rtID, swap.ID, officerB, SwapTargetRespondRequest{
		Accept: true,
	})
	if err != nil {
		t.Fatalf("target accept failed: %v", err)
	}
	if swap.Status != SwapPendingKetua {
		t.Errorf("expected pending_ketua, got %s", swap.Status)
	}

	// 5. Ketua RT approves
	decidedSwap, err := repo.DecideSwap(ctx, rtID, swap.ID, adminID, true, nil, time.Now(), "127.0.0.1")
	if err != nil {
		t.Fatalf("ketua approve swap failed: %v", err)
	}
	if decidedSwap.Status != SwapApproved {
		t.Errorf("expected approved, got %s", decidedSwap.Status)
	}

	// 6. Verify that Officer A now owns Shift Malam and Officer B now owns Shift Pagi
	schedsA, _, err := svc.ListSchedules(ctx, rtID, ScheduleFilter{UserID: officerA, Status: "scheduled"})
	if err != nil || len(schedsA) != 1 {
		t.Fatalf("list schedules officer A failed: %v", err)
	}
	if schedsA[0].ShiftConfigID != shiftMalam.ID {
		t.Errorf("expected Officer A to now hold Shift Malam (%s), got %s", shiftMalam.ID, schedsA[0].ShiftConfigID)
	}

	schedsB, _, err := svc.ListSchedules(ctx, rtID, ScheduleFilter{UserID: officerB, Status: "scheduled"})
	if err != nil || len(schedsB) != 1 {
		t.Fatalf("list schedules officer B failed: %v", err)
	}
	if schedsB[0].ShiftConfigID != shiftPagi.ID {
		t.Errorf("expected Officer B to now hold Shift Pagi (%s), got %s", shiftPagi.ID, schedsB[0].ShiftConfigID)
	}
}

func TestDateIndependent_ScenariosAudit1to10(t *testing.T) {
	pool, repo, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// ── Scenario 1: Petugas memiliki satu penugasan shift aktif tanpa tanggal ──
	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                         "Shift Pagi Audit",
		StartTime:                    "06:00",
		EndTime:                      "14:00",
		CrossesMidnight:              false,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   60,
		CheckInWindowAfterMinutes:    120,
		MissingCheckoutCutoffMinutes: 120,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}

	schedA, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("create schedule failed: %v", err)
	}
	if !schedA.IsActive || schedA.ShiftDate != "" {
		t.Fatalf("scenario 1 failed: expected active continuous schedule without date, got active=%v date=%s", schedA.IsActive, schedA.ShiftDate)
	}

	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos 1 Audit", "Pos 1", "hash", time.Now().Add(time.Hour), time.Now(), "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE security_devices SET is_active = true, activated_at = now() WHERE id = $1`, dev.ID)

	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(time.Minute))
		return raw
	}

	// ── Scenario 2: Petugas check-in dan memperoleh timestamp aktual ──
	day1In := time.Date(2026, 11, 18, 6, 10, 0, 0, JakartaLoc)
	resIn1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day1In), "check_in", "", day1In)
	if err != nil {
		t.Fatalf("scenario 2 check-in failed: %v", err)
	}
	if resIn1.Record.Status != StatusHadir || resIn1.Record.CheckedInAt == nil {
		t.Fatalf("scenario 2 failed: expected status hadir with actual check-in timestamp, got %+v", resIn1.Record)
	}

	// ── Scenario 5: Petugas tidak dapat memiliki dua sesi absensi terbuka yang tidak valid ──
	// Attempt check-in again while Day 1 is still open -> rejected with 409
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day1In.Add(10*time.Minute)), "check_in", "", day1In.Add(10*time.Minute))
	if err == nil {
		t.Fatalf("scenario 5 failed: expected second open check-in to be rejected")
	}

	// ── Scenario 3: Petugas check-out dan sesi yang sama diperbarui ──
	day1Out := time.Date(2026, 11, 18, 14, 5, 0, 0, JakartaLoc)
	resOut1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day1Out), "check_out", "", day1Out)
	if err != nil {
		t.Fatalf("scenario 3 check-out failed: %v", err)
	}
	if resOut1.Record.ID != resIn1.Record.ID {
		t.Fatalf("scenario 3 failed: expected same record to be updated, got in=%s out=%s", resIn1.Record.ID, resOut1.Record.ID)
	}
	if resOut1.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("scenario 3 failed: expected sudah_absen_pulang, got %s", resOut1.Record.Status)
	}

	// ── Scenario 4: Petugas dapat melakukan sesi berikutnya pada shift berikutnya tanpa penugasan baru ──
	day2In := time.Date(2026, 11, 19, 6, 8, 0, 0, JakartaLoc)
	resIn2, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day2In), "check_in", "", day2In)
	if err != nil {
		t.Fatalf("scenario 4 next session check-in failed: %v", err)
	}
	if resIn2.Record.ID == resIn1.Record.ID {
		t.Fatalf("scenario 4 failed: next session must create a distinct attendance session")
	}
	if resIn2.Record.ScheduleID != schedA.ID {
		t.Fatalf("scenario 4 failed: both sessions must belong to the exact same continuous schedule %s", schedA.ID)
	}
	day2Out := time.Date(2026, 11, 19, 14, 2, 0, 0, JakartaLoc)
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(day2Out), "check_out", "", day2Out)
	if err != nil {
		t.Fatalf("scenario 4 next session check-out failed: %v", err)
	}

	// ── Scenario 7: Pergantian tanggal pukul 00:00 tidak menginisialisasi ulang penugasan ──
	schedAfterDays, err := svc.GetScheduleAssignment(ctx, rtID, schedA.ID)
	if err != nil {
		t.Fatalf("scenario 7 failed to fetch schedule: %v", err)
	}
	if len(schedAfterDays) == 0 || !schedAfterDays[0].IsActive || schedAfterDays[0].Status != ScheduleStatusScheduled {
		t.Fatalf("scenario 7 failed: schedule altered across midnight, got %+v", schedAfterDays)
	}

	// ── Scenario 8: Penugasan dinonaktifkan tanpa menghapus histori absensi ──
	if err := svc.CancelSchedule(ctx, rtID, schedA.ID, adminID); err != nil {
		t.Fatalf("scenario 8 deactivation failed: %v", err)
	}
	schedCancelled, err := svc.GetScheduleAssignment(ctx, rtID, schedA.ID)
	if err != nil {
		t.Fatalf("scenario 8 fetch cancelled failed: %v", err)
	}
	if len(schedCancelled) == 0 || schedCancelled[0].IsActive || schedCancelled[0].Status != ScheduleStatusCancelled {
		t.Fatalf("scenario 8 failed: expected deactivated schedule, got %+v", schedCancelled)
	}
	// Verify history is 100% intact!
	attList, totalAtt, err := svc.ListAttendance(ctx, rtID, AttendanceFilter{UserID: officerA, Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("scenario 8 list attendance failed: %v", err)
	}
	if totalAtt != 2 || len(attList) != 2 {
		t.Fatalf("scenario 8 failed: attendance history lost! Expected 2 records, got total=%d len=%d", totalAtt, len(attList))
	}

	// ── Scenario 9: Koreksi mengubah data yang benar-benar ada dan tetap menyimpan audit trail ──
	recToCorrect := attList[0]
	reasonStr := "Koreksi catatan waktu absensi"
	cinStr := recToCorrect.CheckedInAt.Format(time.RFC3339)
	coutStr := recToCorrect.CheckedOutAt.Format(time.RFC3339)
	reqCorrection, err := svc.CreateCorrection(ctx, rtID, officerA, false, CreateCorrectionRequest{
		AttendanceRecordID:    recToCorrect.ID,
		RequestType:           CorrectionTypeTimeAdjustment,
		RequestedStatus:       StatusSudahAbsenPulang,
		RequestedCheckedInAt:  &cinStr,
		RequestedCheckedOutAt: &coutStr,
		Reason:                reasonStr,
	})
	if err != nil {
		t.Fatalf("scenario 9 create correction failed: %v", err)
	}
	if reqCorrection.OriginalStatus != recToCorrect.Status {
		t.Fatalf("scenario 9 audit trail original status mismatch: expected %s, got %s", recToCorrect.Status, reqCorrection.OriginalStatus)
	}
	noteStr := "Disetujui Ketua RT"
	decidedCorrection, err := svc.DecideCorrection(ctx, rtID, reqCorrection.ID, adminID, DecideCorrectionRequest{
		Approve: true,
		Note:    &noteStr,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("scenario 9 decide correction failed: %v", err)
	}
	if decidedCorrection.Status != "approved" || decidedCorrection.DecidedBy == nil || *decidedCorrection.DecidedBy != adminID {
		t.Fatalf("scenario 9 audit trail failed: expected approved by adminID, got status=%s decided_by=%v", decidedCorrection.Status, decidedCorrection.DecidedBy)
	}

	// ── Scenario 10: Koreksi shift malam tidak salah menghubungkan sesi dengan tanggal kalender yang berbeda ──
	shiftMalam, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                         "Shift Malam Audit",
		StartTime:                    "22:00",
		EndTime:                      "06:00",
		CrossesMidnight:              true,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   60,
		CheckInWindowAfterMinutes:    120,
		MissingCheckoutCutoffMinutes: 180,
	})
	if err != nil {
		t.Fatalf("create shift malam failed: %v", err)
	}
	schedMalam, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftMalam.ID,
	})
	if err != nil {
		t.Fatalf("create schedule malam failed: %v", err)
	}
	midnightIn := time.Date(2026, 11, 20, 22, 15, 0, 0, JakartaLoc)
	resMidnightIn, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(midnightIn), "check_in", "", midnightIn)
	if err != nil {
		t.Fatalf("midnight checkin failed: %v", err)
	}
	midnightOut := time.Date(2026, 11, 21, 6, 5, 0, 0, JakartaLoc)
	resMidnightOut, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(midnightOut), "check_out", "", midnightOut)
	if err != nil {
		t.Fatalf("midnight checkout failed: %v", err)
	}
	if resMidnightIn.Record.ID != resMidnightOut.Record.ID {
		t.Fatalf("scenario 6/10 failed: expected single session for cross-midnight")
	}
	corrReason := "Koreksi shift malam"
	corrMidnight, err := svc.CreateCorrection(ctx, rtID, officerA, false, CreateCorrectionRequest{
		AttendanceRecordID: resMidnightOut.Record.ID,
		RequestType:        CorrectionTypeStatusChange,
		RequestedStatus:    StatusSudahAbsenPulang,
		Reason:             corrReason,
	})
	if err != nil {
		t.Fatalf("scenario 10 create correction failed: %v", err)
	}
	if corrMidnight.ShiftDate != "2026-11-20" {
		t.Fatalf("scenario 10 failed: expected correction shift date to be 2026-11-20, got %s", corrMidnight.ShiftDate)
	}
	_ = schedMalam
}

func TestAutoCloseVsActualCheckout_AuditSectionB(t *testing.T) {
	pool, repo, svc, rtID, officerID, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Create shift: Pagi (08:00 - 16:00), missing_checkout_cutoff: 60 minutes
	shift, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                         "Shift Pagi Section B",
		StartTime:                    "08:00",
		EndTime:                      "16:00",
		CrossesMidnight:              false,
		LatenessToleranceMinutes:     15,
		CheckInWindowBeforeMinutes:   30,
		CheckInWindowAfterMinutes:    60,
		AbsentCutoffMinutes:          120,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}

	// 2. Create continuous schedule assignment
	schedNotes := "Continuous Shift Test Section B"
	sched, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerID,
		ShiftConfigID: shift.ID,
		Notes:         &schedNotes,
	})
	if err != nil {
		t.Fatalf("create schedule failed: %v", err)
	}

	// 3. Create POS device & activate
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos B", "Pos Section B", "hashB", time.Now().Add(time.Hour), time.Now(), "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE security_devices SET is_active = true, activated_at = now() WHERE id = $1`, dev.ID)

	makeQR := func(tAt time.Time) string {
		tok := uuid.NewString()
		_, err := repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(tok), tAt, tAt.Add(time.Minute))
		if err != nil {
			t.Fatalf("create qr token failed: %v", err)
		}
		return tok
	}

	// ── Requirement B.1: Check-in Day 1 (Actual check-in) ──
	day1In := time.Date(2026, 11, 25, 7, 55, 0, 0, JakartaLoc)
	resIn, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerID, makeQR(day1In), "check_in", "", day1In)
	if err != nil {
		t.Fatalf("day 1 check-in failed: %v", err)
	}
	if resIn.Record.CheckedInAt == nil {
		t.Fatalf("expected checked_in_at to be recorded, got nil")
	}
	if resIn.Record.CheckedOutAt != nil {
		t.Fatalf("expected checked_out_at to be nil on check-in, got %v", resIn.Record.CheckedOutAt)
	}

	// While shift is active: officer is on duty
	duringShift := time.Date(2026, 11, 25, 12, 0, 0, 0, JakartaLoc)
	onDutyDuring, err := repo.GetOnDutyOfficers(ctx, rtID, duringShift)
	if err != nil {
		t.Fatalf("get on duty failed: %v", err)
	}
	if len(onDutyDuring) != 1 {
		t.Fatalf("expected 1 officer on duty during shift, got %d", len(onDutyDuring))
	}

	// ── Requirement B.2: Officer DOES NOT scan checkout. Advance clock past cutoff (16:00 + 60m = 17:00 -> 17:05) ──
	autoCloseTime := time.Date(2026, 11, 25, 17, 5, 0, 0, JakartaLoc)
	tickRes, err := repo.RunSchedulerTick(ctx, autoCloseTime.UTC())
	if err != nil {
		t.Fatalf("scheduler tick failed: %v", err)
	}
	if tickRes.MarkedMissingCheckout < 1 {
		t.Fatalf("expected at least 1 record auto-closed as missing checkout, got %d", tickRes.MarkedMissingCheckout)
	}

	// ── Requirement B.2 & B.3: Verify auto-close does NOT produce fake checked_out_at ──
	list, _, err := repo.ListAttendance(ctx, rtID, AttendanceFilter{
		UserID:   officerID,
		DateFrom: "2026-11-25",
		DateTo:   "2026-11-25",
		Page:     1,
		PerPage:  10,
	})
	if err != nil {
		t.Fatalf("list attendance failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 attendance record, got %d", len(list))
	}
	recDay1 := list[0]
	if recDay1.Status != StatusBelumAbsenPulang {
		t.Fatalf("expected status %s, got %s", StatusBelumAbsenPulang, recDay1.Status)
	}
	if recDay1.CheckedOutAt != nil {
		t.Fatalf("CRITICAL: checked_out_at MUST BE NIL on auto-closure (no fake checkout time), got %v", recDay1.CheckedOutAt)
	}
	if recDay1.MarkedMissingCheckoutAt == nil {
		t.Fatalf("CRITICAL: marked_missing_checkout_at MUST be set by auto-closure, got nil")
	}

	// ── Requirement B.4: GetOnDutyOfficers MUST NOT consider auto-closed session on duty ──
	onDutyAfterAutoClose, err := repo.GetOnDutyOfficers(ctx, rtID, autoCloseTime)
	if err != nil {
		t.Fatalf("get on duty after auto close failed: %v", err)
	}
	if len(onDutyAfterAutoClose) != 0 {
		t.Fatalf("CRITICAL: auto-closed session must NOT be counted as on duty, got %d officers", len(onDutyAfterAutoClose))
	}

	// ── Requirement B.4: Attendance correction can be submitted and preserves pre-correction state ──
	corrReason := "Lupa scan checkout karena dinas mendadak"
	corrReq, err := svc.CreateCorrection(ctx, rtID, officerID, false, CreateCorrectionRequest{
		AttendanceRecordID: recDay1.ID,
		RequestType:        CorrectionTypeStatusChange,
		RequestedStatus:    StatusSudahAbsenPulang,
		Reason:             corrReason,
	})
	if err != nil {
		t.Fatalf("create correction for auto-closed session failed: %v", err)
	}
	if corrReq.OriginalStatus != StatusBelumAbsenPulang {
		t.Fatalf("expected original_status to preserve %s, got %s", StatusBelumAbsenPulang, corrReq.OriginalStatus)
	}

	// Approve correction by Ketua RT
	noteDecide := "Disetujui setelah verifikasi dinas"
	decided, err := svc.DecideCorrection(ctx, rtID, corrReq.ID, adminID, DecideCorrectionRequest{
		Approve: true,
		Note:    &noteDecide,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("approve correction failed: %v", err)
	}
	if decided.Status != "approved" {
		t.Fatalf("expected correction status approved, got %s", decided.Status)
	}

	// ── Requirement B.5: Auto-close DOES NOT cause session to stay open forever ──
	// Day 2: Officer checks in on the continuous schedule without error!
	day2In := time.Date(2026, 11, 26, 7, 50, 0, 0, JakartaLoc)
	resDay2In, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerID, makeQR(day2In), "check_in", "", day2In)
	if err != nil {
		t.Fatalf("CRITICAL: Day 2 check-in after auto-closure failed: %v", err)
	}
	if resDay2In.Record.ID == recDay1.ID {
		t.Fatalf("expected new attendance record on Day 2, got same record ID")
	}

	// ── Requirement B.1: Actual QR checkout fills checked_out_at ──
	day2Out := time.Date(2026, 11, 26, 16, 10, 0, 0, JakartaLoc)
	resDay2Out, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerID, makeQR(day2Out), "check_out", "", day2Out)
	if err != nil {
		t.Fatalf("Day 2 check-out failed: %v", err)
	}
	if resDay2Out.Record.CheckedOutAt == nil {
		t.Fatalf("CRITICAL: actual scan checkout MUST fill checked_out_at, got nil")
	}
	if resDay2Out.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("expected status %s on actual checkout, got %s", StatusSudahAbsenPulang, resDay2Out.Record.Status)
	}

	_ = sched
}
