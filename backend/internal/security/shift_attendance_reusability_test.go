package security

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestShiftAttendance_ReusableDailyShifts_12Scenarios(t *testing.T) {
	pool, repo, svc, rtID, officerA, officerB, adminID := setupDateIndependentTestEnv(t)
	ctx := context.Background()

	// 1. Create Shift Configurations: Shift Pagi (06:00 - 14:00) and Shift Malam (18:00 - 02:00 next day)
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

	shiftMalam, err := svc.CreateShift(ctx, rtID, adminID, UpsertShiftRequest{
		Name:                        "Shift Malam",
		StartTime:                   "18:00",
		EndTime:                     "02:00",
		CrossesMidnight:             true,
		LatenessToleranceMinutes:    15,
		CheckInWindowBeforeMinutes:  30,
		CheckInWindowAfterMinutes:   60,
		AbsentCutoffMinutes:         120,
		MissingCheckoutCutoffMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create shift malam: %v", err)
	}

	// 2. Assign continuous shifts:
	// Officer A: Shift Pagi & Shift Malam
	// Officer B: Shift Pagi & Shift Malam
	schedAPagi, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("assign A pagi: %v", err)
	}
	schedAMalam, err := svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerA,
		ShiftConfigID: shiftMalam.ID,
	})
	if err != nil {
		t.Fatalf("assign A malam: %v", err)
	}

	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerB,
		ShiftConfigID: shiftPagi.ID,
	})
	if err != nil {
		t.Fatalf("assign B pagi: %v", err)
	}
	_, err = svc.CreateSchedule(ctx, rtID, adminID, CreateScheduleRequest{
		UserID:        officerB,
		ShiftConfigID: shiftMalam.ID,
	})
	if err != nil {
		t.Fatalf("assign B malam: %v", err)
	}

	// Create Pos Security Device for QR codes
	now := time.Now()
	dev, err := repo.CreateDevice(ctx, rtID, adminID, "Pos Induk", "pos_induk", "hash", now.Add(time.Hour), now, "127.0.0.1")
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	_, _ = pool.Raw().ExecContext(ctx, `UPDATE security_devices SET is_active = true, activated_at = now() WHERE id = $1`, dev.ID)

	makeQR := func(at time.Time) string {
		raw := uuid.NewString()
		_, _ = repo.CreateQRToken(ctx, rtID, dev.ID, HashToken(raw), at, at.Add(2*time.Minute))
		return raw
	}

	// ── SCENARIO 1: Officer A successfully attends Shift Pagi on date D ──
	dateD := time.Date(2026, 11, 20, 6, 10, 0, 0, JakartaLoc)
	resInA, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD), "check_in", "", dateD)
	if err != nil {
		t.Fatalf("scenario 1 check-in failed: %v", err)
	}
	if resInA.Record.Status != StatusHadir || resInA.Record.ShiftDate != "2026-11-20" {
		t.Fatalf("scenario 1 expected status hadir on 2026-11-20, got %+v", resInA.Record)
	}

	dateDOut := time.Date(2026, 11, 20, 14, 5, 0, 0, JakartaLoc)
	resOutA, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateDOut), "check_out", "", dateDOut)
	if err != nil {
		t.Fatalf("scenario 1 check-out failed: %v", err)
	}
	if resOutA.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("scenario 1 expected sudah_absen_pulang, got %s", resOutA.Record.Status)
	}

	// ── SCENARIO 4: Officer B cannot claim Shift Pagi on date D after Officer A has claimed it ──
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD), "check_in", "", dateD)
	if err == nil {
		t.Fatalf("scenario 4 failed: Officer B should have been rejected for already claimed shift on date D")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != 409 || appErr.Code != "shift_already_claimed" {
		t.Fatalf("scenario 4 expected 409 shift_already_claimed, got: %v", err)
	}

	// ── SCENARIO 3: Officer A successfully attends Shift Malam on date D after Shift Pagi ──
	dateDMalamIn := time.Date(2026, 11, 20, 18, 5, 0, 0, JakartaLoc)
	resInAMalam, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateDMalamIn), "check_in", "", dateDMalamIn)
	if err != nil {
		t.Fatalf("scenario 3 malam check-in failed: %v", err)
	}
	if resInAMalam.Record.ShiftConfigID != shiftMalam.ID {
		t.Fatalf("scenario 3 expected shift malam, got %s", resInAMalam.Record.ShiftConfigID)
	}
	dateDMalamOut := time.Date(2026, 11, 21, 2, 5, 0, 0, JakartaLoc)
	resOutAMalam, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateDMalamOut), "check_out", "", dateDMalamOut)
	if err != nil {
		t.Fatalf("scenario 3 malam check-out failed: %v", err)
	}
	if resOutAMalam.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("scenario 3 expected status sudah_absen_pulang, got %s", resOutAMalam.Record.Status)
	}

	// ── SCENARIO 2: Officer A successfully attends Shift Pagi again on date D+1 (reusable daily shift) ──
	dateD1In := time.Date(2026, 11, 21, 6, 12, 0, 0, JakartaLoc)
	resInAD1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD1In), "check_in", "", dateD1In)
	if err != nil {
		t.Fatalf("scenario 2 check-in on D+1 failed: %v", err)
	}
	if resInAD1.Record.Status != StatusHadir || resInAD1.Record.ShiftDate != "2026-11-21" {
		t.Fatalf("scenario 2 expected status hadir on 2026-11-21, got %+v", resInAD1.Record)
	}
	dateD1Out := time.Date(2026, 11, 21, 14, 2, 0, 0, JakartaLoc)
	resOutAD1, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD1Out), "check_out", "", dateD1Out)
	if err != nil {
		t.Fatalf("scenario 2 check-out on D+1 failed: %v", err)
	}
	if resOutAD1.Record.Status != StatusSudahAbsenPulang {
		t.Fatalf("scenario 2 expected status sudah_absen_pulang, got %s", resOutAD1.Record.Status)
	}

	// ── SCENARIO 6: Historical attendance on D remains intact after attendance on D+1 ──
	var histStatus, histDate string
	err = pool.Raw().QueryRowContext(ctx, `SELECT status, to_char(shift_date, 'YYYY-MM-DD') FROM security_attendance_records WHERE id = $1`, resInA.Record.ID).Scan(&histStatus, &histDate)
	if err != nil {
		t.Fatalf("scenario 6 fetch historical D failed: %v", err)
	}
	if histStatus != StatusSudahAbsenPulang || histDate != "2026-11-20" {
		t.Fatalf("scenario 6 historical record on D modified: status=%s date=%s", histStatus, histDate)
	}

	// ── SCENARIO 5: Officer B can claim Shift Pagi on date D+2 if that slot is available ──
	dateD2In := time.Date(2026, 11, 22, 6, 8, 0, 0, JakartaLoc)
	resInBD2, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD2In), "check_in", "", dateD2In)
	if err != nil {
		t.Fatalf("scenario 5 Officer B claim on D+2 failed: %v", err)
	}
	if resInBD2.Record.UserID != officerB || resInBD2.Record.ShiftDate != "2026-11-22" {
		t.Fatalf("scenario 5 expected Officer B on 2026-11-22, got %+v", resInBD2.Record)
	}
	dateD2Out := time.Date(2026, 11, 22, 14, 1, 0, 0, JakartaLoc)
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD2Out), "check_out", "", dateD2Out)
	if err != nil {
		t.Fatalf("scenario 5 Officer B check-out on D+2 failed: %v", err)
	}

	// ── SCENARIO 8: Both officers can attend different shifts on the same date ──
	dateD3Pagi := time.Date(2026, 11, 23, 6, 15, 0, 0, JakartaLoc)
	dateD3Malam := time.Date(2026, 11, 23, 18, 15, 0, 0, JakartaLoc)
	resA_D3, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD3Pagi), "check_in", "", dateD3Pagi)
	if err != nil {
		t.Fatalf("scenario 8 Officer A Pagi failed: %v", err)
	}
	resB_D3, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD3Malam), "check_in", "", dateD3Malam)
	if err != nil {
		t.Fatalf("scenario 8 Officer B Malam failed: %v", err)
	}
	if resA_D3.Record.ShiftConfigID == resB_D3.Record.ShiftConfigID {
		t.Fatalf("scenario 8 expected different shift configs")
	}
	// Check out both officers from date D3
	dateD3PagiOut := time.Date(2026, 11, 23, 14, 0, 0, 0, JakartaLoc)
	_, _ = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD3PagiOut), "check_out", "", dateD3PagiOut)
	dateD3MalamOut := time.Date(2026, 11, 24, 2, 0, 0, 0, JakartaLoc)
	_, _ = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD3MalamOut), "check_out", "", dateD3MalamOut)

	// ── SCENARIO 7: Two concurrent officers attempting the same RT/date/shift result in exactly one winner ──
	dateD4In := time.Date(2026, 11, 24, 6, 5, 0, 0, JakartaLoc)
	var wg sync.WaitGroup
	var successCount, conflictCount int
	var mu sync.Mutex

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD4In), "check_in", "", dateD4In)
		mu.Lock()
		if err == nil {
			successCount++
		} else {
			var aErr *AppError
			if errors.As(err, &aErr) && aErr.Status == 409 && aErr.Code == "shift_already_claimed" {
				conflictCount++
			}
		}
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		_, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD4In), "check_in", "", dateD4In)
		mu.Lock()
		if err == nil {
			successCount++
		} else {
			var aErr *AppError
			if errors.As(err, &aErr) && aErr.Status == 409 && aErr.Code == "shift_already_claimed" {
				conflictCount++
			}
		}
		mu.Unlock()
	}()
	wg.Wait()

	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("scenario 7 concurrent claim failed: expected 1 winner and 1 409 conflict, got success=%d conflict=%d", successCount, conflictCount)
	}

	// Check out the winner of date D4 so no open session remains
	dateD4Out := time.Date(2026, 11, 24, 14, 0, 0, 0, JakartaLoc)
	_, _ = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateD4Out), "check_out", "", dateD4Out)
	_, _ = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerB, makeQR(dateD4Out), "check_out", "", dateD4Out)

	// ── SCENARIO 9: Officers belonging to another RT cannot claim the shift ──
	otherRT := uuid.NewString()
	_, _ = pool.Raw().ExecContext(ctx, `INSERT INTO rts (id, rw, rt, name, is_active) VALUES ($1, 99, '099', 'Other RT', true)`, otherRT)
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, otherRT, officerA, makeQR(dateD), "check_in", "", dateD)
	if err == nil {
		t.Fatalf("scenario 9 failed: cross-RT attendance should be rejected")
	}

	// ── SCENARIO 10: Unauthorized roles cannot perform Keamanan attendance ──
	wargaUID := uuid.NewString()
	_, _ = pool.Raw().ExecContext(ctx, `INSERT INTO users (id, email, password_hash, full_name, is_active) VALUES ($1, 'warga@test.com', 'h', 'Warga Biasa', true)`, wargaUID)
	_, _ = pool.Raw().ExecContext(ctx, `INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES (gen_random_uuid(), $1, $2, 'warga', true)`, wargaUID, rtID)
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, wargaUID, makeQR(dateD), "check_in", "", dateD)
	if err == nil {
		t.Fatalf("scenario 10 failed: warga without schedule cannot perform attendance")
	}

	// ── SCENARIO 11: A failed attendance transaction leaves no partial attendance record ──
	dateInvalid := time.Date(2026, 11, 25, 3, 0, 0, 0, JakartaLoc) // Middle of night, no shift active
	_, err = repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateInvalid), "check_in", "", dateInvalid)
	if err == nil {
		t.Fatalf("scenario 11 expected failure outside window")
	}
	var count int
	_ = pool.Raw().QueryRowContext(ctx, `SELECT count(*) FROM security_attendance_records WHERE shift_date = '2026-11-25' AND user_id = $1`, officerA).Scan(&count)
	if count != 0 {
		t.Fatalf("scenario 11 failed: partial attendance record created on failure")
	}

	// ── SCENARIO 12: A retry after a legitimate failure works correctly ──
	// Officer A failed earlier due to expired/invalid QR or time window
	// Now Officer A provides a valid QR during allowed window
	dateValidRetry := time.Date(2026, 11, 25, 6, 10, 0, 0, JakartaLoc)
	resRetry, err := repo.ConsumeQRAndRecordAttendance(ctx, rtID, officerA, makeQR(dateValidRetry), "check_in", "", dateValidRetry)
	if err != nil {
		t.Fatalf("scenario 12 retry failed: %v", err)
	}
	if resRetry.Record.Status != StatusHadir {
		t.Fatalf("scenario 12 expected status hadir on retry, got %s", resRetry.Record.Status)
	}

	_ = schedAPagi
	_ = schedAMalam
}
