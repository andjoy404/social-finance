package security

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSecurity_MultiOfficerSameShift(t *testing.T) {
	_, repo, svc, rtID, officerA, officerB, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Setup Shift Configuration: Shift Pagi (06:00 - 14:00)
	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi Bersama",
		StartTime:                   "06:00",
		EndTime:                     "14:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 120,
	})
	if err != nil {
		t.Fatalf("create shift pagi failed: %v", err)
	}

	// 2. Assign both Officer A and Officer B to Shift Pagi
	schedA, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("assign officer A failed: %v", err)
	}
	schedB, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerB,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("assign officer B failed: %v", err)
	}
	if schedA.ID == schedB.ID {
		t.Fatalf("expected distinct schedule IDs for officers")
	}

	// Helper for generating QR token
	now := time.Now()
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos Utama Test", "Gerbang 1", "hash1", now.Add(time.Hour), now, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	dateD := time.Date(2026, 11, 20, 6, 10, 0, 0, JakartaLoc)

	// Both Officer A and Officer B check in to the same shift on date D
	resInA, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD), "check_in", shiftPagi.ID, "", dateD)
	if err != nil {
		t.Fatalf("officer A check-in failed: %v", err)
	}
	resInB, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD), "check_in", shiftPagi.ID, "", dateD)
	if err != nil {
		t.Fatalf("officer B check-in on same shift failed: %v", err)
	}
	if resInA.Record.ID == resInB.Record.ID {
		t.Fatalf("officer A and B must have distinct attendance records")
	}
	if resInA.Record.Status != StatusHadir || resInB.Record.Status != StatusHadir {
		t.Fatalf("both officers must be hadir")
	}

	// Verify Beranda on duty returns BOTH officers
	duringShift := time.Date(2026, 11, 20, 10, 0, 0, 0, JakartaLoc)
	onDuty, err := repo.GetOnDutyOfficers(ctx, rtID, duringShift)
	if err != nil {
		t.Fatalf("GetOnDutyOfficers failed: %v", err)
	}
	if len(onDuty) != 2 {
		t.Fatalf("expected 2 officers on duty, got %d", len(onDuty))
	}
	uids := map[string]bool{onDuty[0].UserID: true, onDuty[1].UserID: true}
	if !uids[officerA] || !uids[officerB] {
		t.Fatalf("expected officerA and officerB on duty, got %+v", onDuty)
	}

	// Both officers check out
	dateDOut := time.Date(2026, 11, 20, 14, 5, 0, 0, JakartaLoc)
	resOutA, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateDOut), "check_out", "", "", dateDOut)
	if err != nil {
		t.Fatalf("officer A check-out failed: %v", err)
	}
	resOutB, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateDOut), "check_out", "", "", dateDOut)
	if err != nil {
		t.Fatalf("officer B check-out failed: %v", err)
	}
	if resOutA.Record.Status != StatusSudahAbsenPulang || resOutB.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("both officers must be sudah_absen_pulang")
	}
}

func TestSecurity_JumpingShift_Success(t *testing.T) {
	_, repo, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Shift 1: 06:00 - 14:00
	shift1, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift 1 Pagi",
		StartTime:                   "06:00",
		EndTime:                     "14:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift 1 failed: %v", err)
	}

	// 2. Shift 2: 14:00 - 22:00
	shift2, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift 2 Siang",
		StartTime:                   "14:00",
		EndTime:                     "22:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift 2 failed: %v", err)
	}

	// 3. Assign Officer A to BOTH Shift 1 and Shift 2
	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shift1.ID,
	})
	if err != nil {
		t.Fatalf("assign shift 1 failed: %v", err)
	}
	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shift2.ID,
	})
	if err != nil {
		t.Fatalf("assign shift 2 failed: %v", err)
	}

	now := time.Now()
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos Jumping Test", "Gerbang 1", "hash2", now.Add(time.Hour), now, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	// Step A: Check in to Shift 1 at 06:05
	tShift1In := time.Date(2026, 11, 20, 6, 5, 0, 0, JakartaLoc)
	resIn1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(tShift1In), "check_in", shift1.ID, "", tShift1In)
	if err != nil {
		t.Fatalf("shift 1 check-in failed: %v", err)
	}
	if resIn1.Record.Status != StatusHadir {
		t.Fatalf("expected shift 1 status hadir")
	}

	// Step B: Check out of Shift 1 at 14:00
	tShift1Out := time.Date(2026, 11, 20, 14, 0, 0, 0, JakartaLoc)
	resOut1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(tShift1Out), "check_out", shift1.ID, "", tShift1Out)
	if err != nil {
		t.Fatalf("shift 1 check-out failed: %v", err)
	}
	if resOut1.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("expected shift 1 status sudah_absen_pulang")
	}

	// Step C: Immediately check in to Shift 2 at 14:02 (JUMPING SHIFT!)
	// Shift 1 check-out must NEVER block check-in to Shift 2!
	tShift2In := time.Date(2026, 11, 20, 14, 2, 0, 0, JakartaLoc)
	resIn2, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(tShift2In), "check_in", shift2.ID, "", tShift2In)
	if err != nil {
		t.Fatalf("CRITICAL JUMPING SHIFT BLOCKED: check-in to Shift 2 failed after Shift 1 checkout: %v", err)
	}
	if resIn2.Record.ShiftConfigID != shift2.ID {
		t.Fatalf("expected record to belong to Shift 2, got %s", resIn2.Record.ShiftConfigID)
	}
	if resIn2.Record.Status != StatusHadir {
		t.Fatalf("expected Shift 2 status hadir, got %s", resIn2.Record.Status)
	}

	// Step D: Duplicate check-in on Shift 2 while open is rejected
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(tShift2In.Add(5*time.Minute)), "check_in", shift2.ID, "", tShift2In.Add(5*time.Minute))
	if err == nil {
		t.Fatalf("expected duplicate check-in on Shift 2 to be rejected")
	}

	// Step E: Check out of Shift 2 at 22:05
	tShift2Out := time.Date(2026, 11, 20, 22, 5, 0, 0, JakartaLoc)
	resOut2, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(tShift2Out), "check_out", shift2.ID, "", tShift2Out)
	if err != nil {
		t.Fatalf("shift 2 check-out failed: %v", err)
	}
	if resOut2.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("expected shift 2 status sudah_absen_pulang, got %s", resOut2.Record.Status)
	}
}

func TestSecurity_DynamicOccurrences_Minus1_H_Plus1_Plus2(t *testing.T) {
	_, repo, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi Rutin",
		StartTime:                   "07:00",
		EndTime:                     "15:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}

	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("create schedule failed: %v", err)
	}

	// 1. Fetch overview
	overview, err := svc.GetOfficerOverview(ctx, rtID, officerA)
	if err != nil {
		t.Fatalf("GetOfficerOverview failed: %v", err)
	}

	// Verify occurrences
	if len(overview.Occurrences) != 4 {
		t.Fatalf("expected 4 occurrences for 1 shift over [-1, 0, 1, 2], got %d", len(overview.Occurrences))
	}

	labels := []string{"kemarin", "hari_ini", "besok", "lusa"}
	for i, occ := range overview.Occurrences {
		if occ.DateLabel != labels[i] {
			t.Fatalf("occurrence %d expected label %s, got %s", i, labels[i], occ.DateLabel)
		}
		if occ.ShiftConfigID != shiftPagi.ID {
			t.Fatalf("occurrence %d expected shiftConfigID %s, got %s", i, shiftPagi.ID, occ.ShiftConfigID)
		}
		// Future shifts cannot be checked in or corrected
		if occ.DateLabel == "besok" || occ.DateLabel == "lusa" {
			if occ.CanCheckIn {
				t.Fatalf("future shift %s must NOT allow check-in", occ.DateLabel)
			}
			if occ.CanCorrect {
				t.Fatalf("future shift %s must NOT allow correction before starting", occ.DateLabel)
			}
		}
	}

	// Verify idempotency: calling GetOfficerOverview again must return identical occurrences
	overview2, err := svc.GetOfficerOverview(ctx, rtID, officerA)
	if err != nil {
		t.Fatalf("GetOfficerOverview 2 failed: %v", err)
	}
	if len(overview2.Occurrences) != len(overview.Occurrences) {
		t.Fatalf("occurrences count mismatch on reload")
	}

	_ = repo
}

func TestSecurity_Correction_PastOccurrenceWithoutCheckin(t *testing.T) {
	pool, repo, svc, rtID, officerA, officerB, adminID := setupDateIndependentTestEnv(t)
	_ = repo
	ctx := context.Background()

	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi Koreksi",
		StartTime:                   "08:00",
		EndTime:                     "16:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
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

	nowJkt := time.Now().In(JakartaLoc)
	yesterdayStr := nowJkt.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrowStr := nowJkt.AddDate(0, 0, 1).Format("2006-01-02")

	// 1. Officer A forgot check-in on yesterday's shift. Submits correction with schedule_id and shift_date = yesterday!
	cinStr := fmt.Sprintf("%sT08:00:00+07:00", yesterdayStr)
	coutStr := fmt.Sprintf("%sT16:00:00+07:00", yesterdayStr)
	corrReq := CreateCorrectionRequest{
		ScheduleID:            schedA.ID,
		ShiftDate:             yesterdayStr,
		RequestType:           CorrectionTypeManualCheckIn,
		RequestedStatus:       StatusHadir,
		RequestedCheckedInAt:  &cinStr,
		RequestedCheckedOutAt: &coutStr,
		Reason:                "Lupa scan QR kemarin karena HP kehabisan baterai",
	}

	corr, err := svc.CreateCorrection(ctx, rtID, officerA, false, corrReq)
	if err != nil {
		t.Fatalf("create correction for past occurrence without prior check-in failed: %v", err)
	}
	if corr.Status != CorrectionPending {
		t.Fatalf("expected pending status, got %s", corr.Status)
	}
	if corr.ShiftDate != yesterdayStr {
		t.Fatalf("expected correction shift date to be yesterday %s, got %s", yesterdayStr, corr.ShiftDate)
	}

	// 2. Submitting duplicate correction while one is pending is rejected with 409
	_, err = svc.CreateCorrection(ctx, rtID, officerA, false, corrReq)
	if err == nil {
		t.Fatalf("expected 409 conflict for duplicate pending correction")
	}

	// 3. Submitting correction for a FUTURE shift (tomorrow / +1) must be rejected!
	futureReq := CreateCorrectionRequest{
		ScheduleID:            schedA.ID,
		ShiftDate:             tomorrowStr,
		RequestType:           CorrectionTypeManualCheckIn,
		RequestedStatus:       StatusHadir,
		RequestedCheckedInAt:  &cinStr,
		RequestedCheckedOutAt: &coutStr,
		Reason:                "Mencoba koreksi shift masa depan",
	}
	_, err = svc.CreateCorrection(ctx, rtID, officerA, false, futureReq)
	if err == nil {
		t.Fatalf("expected future shift +1 correction to be rejected")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "cannot_correct_future_shift" {
		t.Fatalf("expected cannot_correct_future_shift error for +1, got %v", err)
	}

	// 4. Submitting correction for a FUTURE shift (day after tomorrow / +2) must also be rejected!
	dayAfterTomorrowStr := nowJkt.AddDate(0, 0, 2).Format("2006-01-02")
	cinStr2 := fmt.Sprintf("%sT08:00:00+07:00", dayAfterTomorrowStr)
	coutStr2 := fmt.Sprintf("%sT16:00:00+07:00", dayAfterTomorrowStr)
	futureReq2 := CreateCorrectionRequest{
		ScheduleID:            schedA.ID,
		ShiftDate:             dayAfterTomorrowStr,
		RequestType:           CorrectionTypeManualCheckIn,
		RequestedStatus:       StatusHadir,
		RequestedCheckedInAt:  &cinStr2,
		RequestedCheckedOutAt: &coutStr2,
		Reason:                "Mencoba koreksi shift lusa (+2)",
	}
	_, err = svc.CreateCorrection(ctx, rtID, officerA, false, futureReq2)
	if err == nil {
		t.Fatalf("expected future shift +2 correction to be rejected")
	}
	if !errors.As(err, &appErr) || appErr.Code != "cannot_correct_future_shift" {
		t.Fatalf("expected cannot_correct_future_shift error for +2, got %v", err)
	}

	// 5. Officer B cannot correct Officer A's schedule
	_, err = svc.CreateCorrection(ctx, rtID, officerB, false, corrReq)
	if err == nil {
		t.Fatalf("expected officer B correction of officer A schedule to be rejected with 403")
	}

	// 6. Admin approves correction
	approved, err := svc.DecideCorrection(ctx, rtID, corr.ID, adminID, DecideCorrectionRequest{
		Approve: true,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("decide correction failed: %v", err)
	}
	if approved.Status != CorrectionApproved {
		t.Fatalf("expected approved status, got %s", approved.Status)
	}

	// Verify attendance record was updated
	var attStatus string
	err = pool.Raw().QueryRowContext(ctx, `SELECT status FROM security_attendance_records WHERE id = $1`, corr.AttendanceRecordID).Scan(&attStatus)
	if err != nil {
		t.Fatalf("get attendance record failed: %v", err)
	}
	if attStatus != StatusHadir {
		t.Fatalf("expected attendance record status hadir, got %s", attStatus)
	}
}

func TestSecurity_CrossMidnight_ShiftDatePreserved(t *testing.T) {
	_, repo, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// Shift 22:00 - 06:00 (crosses midnight)
	shiftMalam, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Malam Lintas Tengah Malam",
		StartTime:                   "22:00",
		EndTime:                     "06:00",
		CrossesMidnight:             true,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 120,
	})
	if err != nil {
		t.Fatalf("create shift malam failed: %v", err)
	}

	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftMalam.ID,
	})
	if err != nil {
		t.Fatalf("create schedule failed: %v", err)
	}

	now := time.Now()
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos Malam Test", "Gerbang 1", "hash3", now.Add(time.Hour), now, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	// Thursday night 22:10 check-in
	tIn := time.Date(2026, 11, 19, 22, 10, 0, 0, JakartaLoc)
	resIn, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(tIn), "check_in", shiftMalam.ID, "", tIn)
	if err != nil {
		t.Fatalf("cross-midnight check-in failed: %v", err)
	}
	if resIn.Record.ShiftDate != "2026-11-19" {
		t.Fatalf("expected shift date to be start date 2026-11-19, got %s", resIn.Record.ShiftDate)
	}

	// Friday morning 06:05 check-out
	tOut := time.Date(2026, 11, 20, 6, 5, 0, 0, JakartaLoc)
	resOut, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(tOut), "check_out", shiftMalam.ID, "", tOut)
	if err != nil {
		t.Fatalf("cross-midnight check-out failed: %v", err)
	}
	if resOut.Record.ShiftDate != "2026-11-19" {
		t.Fatalf("expected shift date after checkout to stay start date 2026-11-19, got %s", resOut.Record.ShiftDate)
	}
	if resOut.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("expected status sudah_absen_pulang, got %s", resOut.Record.Status)
	}
}

func TestSecurity_ConcurrentCheckin_TwoOfficers(t *testing.T) {
	_, repo, svc, rtID, officerA, officerB, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi Konkuren",
		StartTime:                   "07:00",
		EndTime:                     "15:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}

	_, _ = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{UserID: officerA, ShiftConfigID: shiftPagi.ID})
	_, _ = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{UserID: officerB, ShiftConfigID: shiftPagi.ID})

	now := time.Now()
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos Konkuren", "Gerbang", "hash_conc", now.Add(time.Hour), now, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device failed: %v", err)
	}
	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	dateD := time.Date(2026, 11, 20, 7, 5, 0, 0, JakartaLoc)

	// Concurrently check in both officers
	type resHolder struct {
		officer string
		res     *AttendanceScanResult
		err     error
	}
	ch := make(chan resHolder, 2)
	go func() {
		r, e := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD), "check_in", shiftPagi.ID, "", dateD)
		ch <- resHolder{officer: officerA, res: r, err: e}
	}()
	go func() {
		r, e := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD), "check_in", shiftPagi.ID, "", dateD)
		ch <- resHolder{officer: officerB, res: r, err: e}
	}()

	for i := 0; i < 2; i++ {
		item := <-ch
		if item.err != nil {
			t.Fatalf("concurrent check-in failed for officer %s: %v", item.officer, item.err)
		}
		if item.res.Record.Status != StatusHadir {
			t.Fatalf("officer %s status must be hadir, got %s", item.officer, item.res.Record.Status)
		}
	}
}

func TestSecurity_DuplicateCheckinAndCheckout_Rejected(t *testing.T) {
	_, repo, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi Duplikat",
		StartTime:                   "08:00",
		EndTime:                     "16:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}
	_, _ = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{UserID: officerA, ShiftConfigID: shiftPagi.ID})

	now := time.Now()
	dev, _ := repo.CreateDevice(ctx, rtID, adminID, "Pos Dup", "Gerbang", "hash_dup", now.Add(time.Hour), now, "127.0.0.1")
	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	dateD := time.Date(2026, 11, 20, 8, 5, 0, 0, JakartaLoc)

	// First check-in succeeds
	res1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD), "check_in", shiftPagi.ID, "", dateD)
	if err != nil {
		t.Fatalf("first check-in failed: %v", err)
	}
	if res1.Record.Status != StatusHadir {
		t.Fatalf("expected status hadir")
	}

	// Duplicate check-in is rejected
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD.Add(time.Minute)), "check_in", shiftPagi.ID, "", dateD.Add(time.Minute))
	if err == nil {
		t.Fatalf("expected duplicate check-in to be rejected")
	}

	// First checkout succeeds
	dateOut := time.Date(2026, 11, 20, 16, 5, 0, 0, JakartaLoc)
	resOut, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateOut), "check_out", shiftPagi.ID, "", dateOut)
	if err != nil {
		t.Fatalf("first checkout failed: %v", err)
	}
	if resOut.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("expected status sudah_absen_pulang")
	}

	// Duplicate checkout is rejected
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateOut.Add(time.Minute)), "check_out", shiftPagi.ID, "", dateOut.Add(time.Minute))
	if err == nil {
		t.Fatalf("expected duplicate checkout to be rejected")
	}
}

func TestSecurity_UnassignedOfficer_CheckinRejected(t *testing.T) {
	_, repo, svc, rtID, _, officerB, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	shiftPagi, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi Khusus",
		StartTime:                   "08:00",
		EndTime:                     "16:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}
	// Officer B is NOT assigned to shiftPagi!

	now := time.Now()
	dev, _ := repo.CreateDevice(ctx, rtID, adminID, "Pos Unassigned", "Gerbang", "hash_unassigned", now.Add(time.Hour), now, "127.0.0.1")
	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	dateD := time.Date(2026, 11, 20, 8, 5, 0, 0, JakartaLoc)
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD), "check_in", shiftPagi.ID, "", dateD)
	if err == nil {
		t.Fatalf("expected unassigned officer check-in to be rejected")
	}
}

func TestSecurity_Correction_RejectionLifecycle_And_Isolation(t *testing.T) {
	pool, _, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	shift1, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift 1 Pagi",
		StartTime:                   "06:00",
		EndTime:                     "14:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift 1 failed: %v", err)
	}
	shift2, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift 2 Siang",
		StartTime:                   "14:00",
		EndTime:                     "22:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift 2 failed: %v", err)
	}

	sched1, _ := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{UserID: officerA, ShiftConfigID: shift1.ID})
	sched2, _ := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{UserID: officerA, ShiftConfigID: shift2.ID})

	nowJkt := time.Now().In(JakartaLoc)
	yesterdayStr := nowJkt.AddDate(0, 0, -1).Format("2006-01-02")

	// 1. Submit correction on Shift 1
	cinStr := fmt.Sprintf("%sT06:00:00+07:00", yesterdayStr)
	coutStr := fmt.Sprintf("%sT14:00:00+07:00", yesterdayStr)
	corrReq := CreateCorrectionRequest{
		ScheduleID:            sched1.ID,
		ShiftDate:             yesterdayStr,
		RequestType:           CorrectionTypeManualCheckIn,
		RequestedStatus:       StatusHadir,
		RequestedCheckedInAt:  &cinStr,
		RequestedCheckedOutAt: &coutStr,
		Reason:                "Koreksi Shift 1 yang akan ditolak",
	}
	corr, err := svc.CreateCorrection(ctx, rtID, officerA, false, corrReq)
	if err != nil {
		t.Fatalf("create correction failed: %v", err)
	}

	// 2. Ketua RT rejects correction
	note := "Bukti tidak valid"
	decided, err := svc.DecideCorrection(ctx, rtID, corr.ID, adminID, DecideCorrectionRequest{
		Approve: false,
		Note:    &note,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("decide correction reject failed: %v", err)
	}
	if decided.Status != CorrectionRejected {
		t.Fatalf("expected rejected status, got %s", decided.Status)
	}

	// 3. Verify record status is finalized to tidak_hadir and correction_locked = true
	var recStatus string
	var locked bool
	err = pool.Raw().QueryRowContext(ctx, `SELECT status, correction_locked FROM security_attendance_records WHERE id = $1`, corr.AttendanceRecordID).Scan(&recStatus, &locked)
	if err != nil {
		t.Fatalf("query record failed: %v", err)
	}
	if recStatus != StatusTidakHadir {
		t.Fatalf("expected record to be tidak_hadir after rejection, got %s", recStatus)
	}
	if !locked {
		t.Fatalf("expected record to be correction_locked")
	}

	// 4. Verify Shift 2 is completely unaffected (no record, no cross-contamination)
	var countShift2 int
	_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM security_attendance_records WHERE schedule_id = $1 AND shift_date = $2::date`, sched2.ID, yesterdayStr).Scan(&countShift2)
	if countShift2 != 0 {
		t.Fatalf("Shift 2 must have no records, got %d", countShift2)
	}
}

func TestSecurity_ConcurrentAttendanceAndCorrection_NoDuplicates(t *testing.T) {
	_, _, svc, rtID, officerA, _, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	shift1, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Pagi Konkuren Koreksi",
		StartTime:                   "08:00",
		EndTime:                     "16:00",
		CrossesMidnight:             false,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift failed: %v", err)
	}
	sched1, _ := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{UserID: officerA, ShiftConfigID: shift1.ID})

	nowJkt := time.Now().In(JakartaLoc)
	yesterdayStr := nowJkt.AddDate(0, 0, -1).Format("2006-01-02")

	// Submit 2 identical correction requests concurrently for the exact same occurrence
	cinStr := fmt.Sprintf("%sT08:00:00+07:00", yesterdayStr)
	coutStr := fmt.Sprintf("%sT16:00:00+07:00", yesterdayStr)
	req := CreateCorrectionRequest{
		ScheduleID:            sched1.ID,
		ShiftDate:             yesterdayStr,
		RequestType:           CorrectionTypeManualCheckIn,
		RequestedStatus:       StatusHadir,
		RequestedCheckedInAt:  &cinStr,
		RequestedCheckedOutAt: &coutStr,
		Reason:                "Koreksi serentak",
	}

	type corrRes struct {
		corr *AttendanceCorrection
		err  error
	}
	ch := make(chan corrRes, 2)
	for i := 0; i < 2; i++ {
		go func() {
			c, e := svc.CreateCorrection(ctx, rtID, officerA, false, req)
			ch <- corrRes{corr: c, err: e}
		}()
	}

	var successes, conflicts int
	for i := 0; i < 2; i++ {
		res := <-ch
		if res.err == nil {
			successes++
		} else {
			var appErr *AppError
			if errors.As(res.err, &appErr) && appErr.Code == "correction_already_pending" {
				conflicts++
			}
		}
	}

	// Exactly ONE must succeed and the other must be rejected with conflict
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected exactly 1 success and 1 conflict, got %d successes and %d conflicts", successes, conflicts)
	}
}
