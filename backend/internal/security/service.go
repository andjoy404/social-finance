package security

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"social-finance/internal/auth"
)

type Service struct {
	repo  *Repository
	nowFn func() time.Time
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo:  repo,
		nowFn: time.Now,
	}
}

// WithClock overrides the server clock for deterministic unit/integration testing.
func (s *Service) WithClock(fn func() time.Time) {
	if fn != nil {
		s.nowFn = fn
	}
}

func (s *Service) Now() time.Time {
	return NowJakarta(s.nowFn)
}

func (s *Service) NowUTC() time.Time {
	return s.Now().UTC()
}

// ── Tenant Resolution ─────────────────────────────────────────────────────────

func (s *Service) ResolveTenantRT(ctx context.Context, ac *auth.AuthContext, headerRT, queryRT string) (string, error) {
	if ac == nil {
		return "", newAppError(401, "unauthorized", "missing authentication", ErrUnauthorized)
	}
	if ac.RTID != "" {
		return ac.RTID, nil
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		candidate := strings.TrimSpace(headerRT)
		if candidate == "" {
			candidate = strings.TrimSpace(queryRT)
		}
		if candidate != "" {
			if _, err := uuid.Parse(candidate); err != nil {
				return "", newAppError(400, "invalid_rt_id", "invalid target rt_id format", ErrValidation)
			}
			return candidate, nil
		}
		return "", newAppError(400, "missing_tenant_context", "super admin must specify target RT via X-RT-ID header or rt_id query", ErrValidation)
	}
	return "", newAppError(403, "forbidden", "missing active RT membership", ErrForbidden)
}

// ── Devices ───────────────────────────────────────────────────────────────────

func (s *Service) ListDevices(ctx context.Context, rtID string) ([]SecurityDevice, error) {
	return s.repo.ListDevices(ctx, rtID)
}

func (s *Service) CreateDevice(ctx context.Context, rtID, actorID string, req CreateDeviceRequest, ip string) (*DeviceProvisionResult, error) {
	name := strings.TrimSpace(req.DeviceName)
	if len(name) < 2 || len(name) > 100 {
		return nil, newAppError(400, "validation_error", "device_name must be between 2 and 100 characters", ErrValidation)
	}
	loc := strings.TrimSpace(req.LocationLabel)
	if loc == "" {
		loc = "Pos Keamanan Utama"
	}
	if len(loc) > 120 {
		return nil, newAppError(400, "validation_error", "location_label too long", ErrValidation)
	}

	rawSecret, err := GenerateRandomSecret(24)
	if err != nil {
		return nil, err
	}
	now := s.NowUTC()
	exp := now.Add(DefaultPairingTTL)
	dev, err := s.repo.CreateDevice(ctx, rtID, actorID, name, loc, HashToken(rawSecret), exp, now, ip)
	if err != nil {
		return nil, err
	}
	return &DeviceProvisionResult{
		Device:       *dev,
		PairingToken: dev.ID + "." + rawSecret,
		ExpiresAt:    exp.In(JakartaLoc),
	}, nil
}

func (s *Service) RotatePairing(ctx context.Context, rtID, deviceID, actorID, ip string) (*DeviceProvisionResult, error) {
	if _, err := uuid.Parse(deviceID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid device id", ErrValidation)
	}
	rawSecret, err := GenerateRandomSecret(24)
	if err != nil {
		return nil, err
	}
	now := s.NowUTC()
	exp := now.Add(DefaultPairingTTL)
	dev, err := s.repo.RotatePairingCode(ctx, rtID, deviceID, actorID, HashToken(rawSecret), exp, now, ip)
	if err != nil {
		return nil, err
	}
	return &DeviceProvisionResult{
		Device:       *dev,
		PairingToken: dev.ID + "." + rawSecret,
		ExpiresAt:    exp.In(JakartaLoc),
	}, nil
}

func (s *Service) ActivateDevice(ctx context.Context, req ActivateDeviceRequest, ip string) (*DeviceActivationResult, error) {
	deviceID, providedSecret, ok := SplitCompositeToken(req.PairingToken)
	if !ok {
		return nil, newAppError(400, "validation_error", "invalid pairing_token format", ErrValidation)
	}
	permanentSecret, err := GenerateRandomSecret(32)
	if err != nil {
		return nil, err
	}
	dev, err := s.repo.ActivateDevice(ctx, deviceID, providedSecret, HashToken(permanentSecret), s.NowUTC(), ip)
	if err != nil {
		return nil, err
	}
	return &DeviceActivationResult{
		Device:      *dev,
		DeviceToken: dev.ID + "." + permanentSecret,
	}, nil
}

func (s *Service) RevokeDevice(ctx context.Context, rtID, deviceID, actorID, ip string) (*SecurityDevice, error) {
	if _, err := uuid.Parse(deviceID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid device id", ErrValidation)
	}
	return s.repo.RevokeDevice(ctx, rtID, deviceID, actorID, s.NowUTC(), ip)
}

// ── Officers ──────────────────────────────────────────────────────────────────

func (s *Service) ListOfficers(ctx context.Context, rtID string) ([]SecurityOfficer, error) {
	return s.repo.ListOfficers(ctx, rtID)
}

func (s *Service) ListOfficersForAttendance(ctx context.Context, rtID string) ([]SecurityOfficer, error) {
	return s.repo.ListOfficersForAttendance(ctx, rtID)
}

// ── Shift Configs ─────────────────────────────────────────────────────────────

func validateShiftRequest(req UpsertShiftRequest) (startNorm, endNorm string, crosses bool, err error) {
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 80 {
		return "", "", false, newAppError(400, "validation_error", "shift name must be between 2 and 80 characters", ErrValidation)
	}
	sh, sm, sNorm, err := ParseHHMM(req.StartTime)
	if err != nil {
		return "", "", false, newAppError(400, "validation_error", err.Error(), ErrValidation)
	}
	eh, em, eNorm, err := ParseHHMM(req.EndTime)
	if err != nil {
		return "", "", false, newAppError(400, "validation_error", err.Error(), ErrValidation)
	}
	if sh == eh && sm == em {
		return "", "", false, newAppError(400, "validation_error", "shift start_time and end_time cannot be identical", ErrValidation)
	}
	if req.LatenessToleranceMinutes < 0 || req.LatenessToleranceMinutes > 180 {
		return "", "", false, newAppError(400, "validation_error", "lateness_tolerance_minutes must be 0..180", ErrValidation)
	}
	if req.CheckInWindowBeforeMinutes < 0 || req.CheckInWindowBeforeMinutes > 360 {
		return "", "", false, newAppError(400, "validation_error", "check_in_window_before_minutes must be 0..360", ErrValidation)
	}
	if req.CheckInWindowAfterMinutes < 0 || req.CheckInWindowAfterMinutes > 480 {
		return "", "", false, newAppError(400, "validation_error", "check_in_window_after_minutes must be 0..480", ErrValidation)
	}
	if req.AbsentCutoffMinutes < 0 || req.AbsentCutoffMinutes > 480 {
		return "", "", false, newAppError(400, "validation_error", "absent_cutoff_minutes must be 0..480", ErrValidation)
	}
	if req.MissingCheckoutCutoffMinutes < 0 || req.MissingCheckoutCutoffMinutes > 720 {
		return "", "", false, newAppError(400, "validation_error", "missing_checkout_cutoff_minutes must be 0..720", ErrValidation)
	}
	startMin := sh*60 + sm
	endMin := eh*60 + em
	crosses = req.CrossesMidnight || endMin <= startMin
	return sNorm, eNorm, crosses, nil
}

func (s *Service) ListShifts(ctx context.Context, rtID string, activeOnly bool) ([]ShiftConfig, error) {
	return s.repo.ListShifts(ctx, rtID, activeOnly)
}

func (s *Service) CreateShift(ctx context.Context, rtID, actorID string, req UpsertShiftRequest) (*ShiftConfig, error) {
	sNorm, eNorm, crosses, err := validateShiftRequest(req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateShift(ctx, rtID, actorID, req, sNorm, eNorm, crosses, s.NowUTC())
}

func (s *Service) UpdateShift(ctx context.Context, rtID, shiftID, actorID string, req UpsertShiftRequest) (*ShiftConfig, error) {
	if _, err := uuid.Parse(shiftID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid shift id", ErrValidation)
	}
	sNorm, eNorm, crosses, err := validateShiftRequest(req)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateShift(ctx, rtID, shiftID, actorID, req, sNorm, eNorm, crosses, s.NowUTC())
}

// ── Schedules ─────────────────────────────────────────────────────────────────

func parseShiftDate(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, newAppError(400, "validation_error", "shift_date is required (YYYY-MM-DD)", ErrValidation)
	}
	t, err := time.ParseInLocation("2006-01-02", trimmed, JakartaLoc)
	if err != nil {
		return time.Time{}, newAppError(400, "validation_error", "invalid shift_date format, expected YYYY-MM-DD", ErrValidation)
	}
	return t, nil
}

func (s *Service) ListSchedules(ctx context.Context, rtID string, f ScheduleFilter) ([]SecuritySchedule, int, error) {
	return s.repo.ListSchedules(ctx, rtID, f)
}

func (s *Service) GetScheduleAssignment(ctx context.Context, rtID, scheduleID string) ([]SecuritySchedule, error) {
	if _, err := uuid.Parse(scheduleID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid schedule id", ErrValidation)
	}
	return s.repo.GetScheduleAssignment(ctx, rtID, scheduleID)
}

func (s *Service) CreateSchedule(ctx context.Context, rtID, actorID string, req CreateScheduleRequest) (*SecuritySchedule, error) {
	if _, err := uuid.Parse(req.UserID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid user_id", ErrValidation)
	}
	if _, err := uuid.Parse(req.ShiftConfigID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid shift_config_id", ErrValidation)
	}
	start, end, err := normalizeScheduleDates(req.StartDate, req.EndDate, req.ShiftDate)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateSchedulePeriod(ctx, rtID, actorID, req, start, end, s.NowUTC())
}

func (s *Service) UpdateSchedule(ctx context.Context, rtID, scheduleID, actorID string, req UpdateScheduleRequest) (*SecuritySchedule, error) {
	if _, err := uuid.Parse(scheduleID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid schedule id", ErrValidation)
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid user_id", ErrValidation)
	}
	if _, err := uuid.Parse(req.ShiftConfigID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid shift_config_id", ErrValidation)
	}
	// Resolve dates: nil pointer means "preserve existing"; dereference to string for normalizeScheduleDates.
	startRaw := ""
	endRaw := ""
	if req.StartDate != nil {
		startRaw = *req.StartDate
	}
	if req.EndDate != nil {
		endRaw = *req.EndDate
	}
	start, end, err := normalizeScheduleDates(startRaw, endRaw, req.ShiftDate)
	if err != nil {
		return nil, err
	}
	// Pass whether the client explicitly sent date fields so the repository can preserve old values.
	return s.repo.UpdateSchedulePeriod(ctx, rtID, scheduleID, actorID, req, start, end, s.NowUTC())
}

func (s *Service) CancelSchedule(ctx context.Context, rtID, scheduleID, actorID string) error {
	if _, err := uuid.Parse(scheduleID); err != nil {
		return newAppError(400, "validation_error", "invalid schedule id", ErrValidation)
	}
	return s.repo.CancelSchedule(ctx, rtID, scheduleID, actorID, s.NowUTC())
}

// ── Swaps ─────────────────────────────────────────────────────────────────────

func (s *Service) ListSwaps(ctx context.Context, rtID, userFilter, statusFilter string) ([]ShiftSwap, error) {
	return s.repo.ListSwaps(ctx, rtID, userFilter, statusFilter)
}

func (s *Service) CreateSwap(ctx context.Context, rtID, requesterUserID string, req CreateSwapRequest) (*ShiftSwap, error) {
	if _, err := uuid.Parse(req.RequesterScheduleID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid requester_schedule_id", ErrValidation)
	}
	if _, err := uuid.Parse(req.TargetScheduleID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid target_schedule_id", ErrValidation)
	}
	if len(strings.TrimSpace(req.Reason)) < 3 {
		return nil, newAppError(400, "validation_error", "reason must be at least 3 characters", ErrValidation)
	}
	return s.repo.CreateSwap(ctx, rtID, requesterUserID, req, s.NowUTC())
}

func (s *Service) TargetRespondSwap(ctx context.Context, rtID, swapID, targetUserID string, req SwapTargetRespondRequest) (*ShiftSwap, error) {
	if _, err := uuid.Parse(swapID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid swap id", ErrValidation)
	}
	return s.repo.TargetRespondSwap(ctx, rtID, swapID, targetUserID, req.Accept, req.Note, s.NowUTC())
}

func (s *Service) DecideSwap(ctx context.Context, rtID, swapID, approverID string, req SwapDecideRequest, ip string) (*ShiftSwap, error) {
	if _, err := uuid.Parse(swapID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid swap id", ErrValidation)
	}
	return s.repo.DecideSwap(ctx, rtID, swapID, approverID, req.Approve, req.Note, s.NowUTC(), ip)
}

// ── QR Generation & Scanning ──────────────────────────────────────────────────

func (s *Service) GeneratePosQR(ctx context.Context, dc *DeviceContext) (*DynamicQRPayload, error) {
	rawToken, err := GenerateRandomSecret(24)
	if err != nil {
		return nil, err
	}
	qrString := "SFQR:" + rawToken
	issuedAt := s.NowUTC()
	expiresAt := issuedAt.Add(DefaultQRTTL)
	tokenID, err := s.repo.CreateQRToken(ctx, dc.RTID, dc.DeviceID, HashToken(qrString), issuedAt, expiresAt)
	if err != nil {
		return nil, err
	}
	return &DynamicQRPayload{
		TokenID:          tokenID,
		QRToken:          qrString,
		DeviceID:         dc.DeviceID,
		DeviceName:       dc.DeviceName,
		RTID:             dc.RTID,
		IssuedAt:         issuedAt.In(JakartaLoc),
		ExpiresAt:        expiresAt.In(JakartaLoc),
		ExpiresInSeconds: int(DefaultQRTTL.Seconds()),
	}, nil
}

func (s *Service) ScanQR(ctx context.Context, rtID, userID string, req ScanQRRequest, idemKey string) (*AttendanceScanResult, error) {
	if strings.TrimSpace(req.QRToken) == "" {
		return nil, newAppError(400, "validation_error", "qr_token is required", ErrValidation)
	}
	return s.repo.ConsumeQRAndRecordAttendance(ctx, rtID, userID, req.QRToken, req.Action, req.ShiftConfigID, idemKey, s.NowUTC())
}

// ── Attendance Queries & Officer Overview ─────────────────────────────────────

func (s *Service) ListAttendance(ctx context.Context, rtID string, f AttendanceFilter) ([]AttendanceRecord, int, error) {
	return s.repo.ListAttendance(ctx, rtID, f)
}

func (s *Service) GetOfficerOverview(ctx context.Context, rtID, userID string) (*OfficerAttendanceOverview, error) {
	now := s.Now()
	nowJkt := now.In(JakartaLoc)

	// Fetch officer's active shift assignments without calendar date dependency
	scheds, _, err := s.repo.ListSchedules(ctx, rtID, ScheduleFilter{
		UserID:  userID,
		Status:  ScheduleStatusScheduled,
		Page:    1,
		PerPage: 30,
	})
	if err != nil {
		return nil, err
	}

	todayZero := time.Date(nowJkt.Year(), nowJkt.Month(), nowJkt.Day(), 0, 0, 0, 0, JakartaLoc)
	startDateStr := todayZero.AddDate(0, 0, -2).Format("2006-01-02")
	endDateStr := todayZero.AddDate(0, 0, 3).Format("2006-01-02")

	// Fetch attendance records around current window [-2, +3] for this officer
	recordsInWindow, _, _ := s.repo.ListAttendance(ctx, rtID, AttendanceFilter{
		UserID:   userID,
		DateFrom: startDateStr,
		DateTo:   endDateStr,
		Page:     1,
		PerPage:  50,
	})

	recent, _, err := s.repo.ListAttendance(ctx, rtID, AttendanceFilter{
		UserID:  userID,
		Page:    1,
		PerPage: 20,
	})
	if err != nil {
		return nil, err
	}

	overview := &OfficerAttendanceOverview{
		ServerTime:     nowJkt,
		Timezone:       "Asia/Jakarta",
		UpcomingShifts: []SecuritySchedule{},
		RecentHistory:  recent,
		Occurrences:    []OfficerShiftOccurrence{},
	}

	// Build dynamic occurrences for offsets -1 (kemarin), 0 (hari_ini), 1 (besok), 2 (lusa)
	offsets := []struct {
		d       int
		label   string
		labelID string
	}{
		{-1, "kemarin", "Kemarin"},
		{0, "hari_ini", "Hari Ini"},
		{1, "besok", "Besok"},
		{2, "lusa", "Lusa"},
	}

	recordMap := make(map[string]*AttendanceRecord)
	for i := range recordsInWindow {
		r := &recordsInWindow[i]
		key := fmt.Sprintf("%s:%s", r.ScheduleID, r.ShiftDate)
		recordMap[key] = r
		cfgKey := fmt.Sprintf("%s:%s", r.ShiftConfigID, r.ShiftDate)
		if _, exists := recordMap[cfgKey]; !exists {
			recordMap[cfgKey] = r
		}
	}

	var occurrences []OfficerShiftOccurrence
	for _, off := range offsets {
		targetDate := todayZero.AddDate(0, 0, off.d)
		targetDateStr := targetDate.Format("2006-01-02")

		for i := range scheds {
			sc := &scheds[i]
			sh, sm, _, err1 := ParseHHMM(sc.StartTime)
			eh, em, _, err2 := ParseHHMM(sc.EndTime)
			if err1 != nil || err2 != nil {
				continue
			}

			occStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), sh, sm, 0, 0, JakartaLoc)
			occEnd := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), eh, em, 0, 0, JakartaLoc)
			if sc.CrossesMidnight || !occEnd.After(occStart) {
				occEnd = occEnd.AddDate(0, 0, 1)
			}

			winBefore := sc.CheckInWindowBeforeMinutes
			if winBefore <= 0 {
				winBefore = 15
			}
			winAfter := sc.CheckInWindowAfterMinutes
			if winAfter <= 0 {
				winAfter = 60
			}
			missOutMin := sc.MissingCheckoutCutoffMinutes
			if missOutMin <= 0 {
				missOutMin = 120
			}

			openAt := occStart.Add(-time.Duration(winBefore) * time.Minute)
			closeAt := occStart.Add(time.Duration(winAfter) * time.Minute)
			maxOut := occEnd.Add(time.Duration(missOutMin) * time.Minute)

			// Look up matching record
			key := fmt.Sprintf("%s:%s", sc.ID, targetDateStr)
			rec := recordMap[key]
			if rec == nil {
				cfgKey := fmt.Sprintf("%s:%s", sc.ShiftConfigID, targetDateStr)
				rec = recordMap[cfgKey]
			}

			st := StatusBelumHadir
			stLabel := "Belum Hadir"
			var attID string
			var cin, cout *time.Time
			if rec != nil {
				st = rec.Status
				stLabel = rec.StatusLabel
				attID = rec.ID
				cin = rec.CheckedInAt
				cout = rec.CheckedOutAt
			} else {
				// If past occurrence without record, mark as absent
				if off.d < 0 || (off.d == 0 && nowJkt.After(closeAt)) {
					st = StatusTidakHadir
					stLabel = "Tidak Hadir"
				}
			}

			canCheckIn := false
			canCheckOut := false
			canCorrect := false

			// Check-in condition
			if cin == nil && (st == StatusBelumHadir || st == StatusTidakHadir) {
				if !nowJkt.Before(openAt) && !nowJkt.After(closeAt) {
					canCheckIn = true
				}
			}

			// Check-out condition
			if cin != nil && cout == nil && (st == StatusHadir || st == StatusTerlambat) {
				if !nowJkt.After(maxOut) {
					canCheckOut = true
				}
			}

			// Correction condition:
			// Can request correction for past/today shifts once the shift has started.
			// Future shifts (+1, +2) cannot be corrected before start.
			if !nowJkt.Before(openAt) {
				canCorrect = true
			}
			if rec != nil && rec.CorrectionLocked {
				canCorrect = false
			}

			occurrences = append(occurrences, OfficerShiftOccurrence{
				DateLabel:       off.label,
				DateLabelID:     off.labelID,
				ShiftDate:       targetDateStr,
				ScheduleID:      sc.ID,
				ShiftConfigID:   sc.ShiftConfigID,
				ShiftName:       sc.ShiftName,
				StartTime:       sc.StartTime,
				EndTime:         sc.EndTime,
				CrossesMidnight: sc.CrossesMidnight,
				Status:          st,
				StatusLabel:     stLabel,
				AttendanceID:    attID,
				CheckedInAt:     cin,
				CheckedOutAt:    cout,
				CanCheckIn:      canCheckIn,
				CanCheckOut:     canCheckOut,
				CanCorrect:      canCorrect,
			})
		}
	}

	overview.Occurrences = occurrences

	// Determine CurrentSchedule & CurrentRecord
	var currentSched *SecuritySchedule
	var currentRec *AttendanceRecord

	// 1. Priority 1: Any occurrence with CanCheckOut (officer is currently on duty)
	for _, occ := range occurrences {
		if occ.CanCheckOut {
			for j := range scheds {
				if scheds[j].ID == occ.ScheduleID {
					sCopy := scheds[j]
					currentSched = &sCopy
					break
				}
			}
			if occ.AttendanceID != "" {
				for j := range recent {
					if recent[j].ID == occ.AttendanceID {
						rCopy := recent[j]
						currentRec = &rCopy
						break
					}
				}
			}
			break
		}
	}

	// 2. Priority 2: Any occurrence with CanCheckIn (ready to check in right now)
	if currentSched == nil {
		for _, occ := range occurrences {
			if occ.CanCheckIn {
				for j := range scheds {
					if scheds[j].ID == occ.ScheduleID {
						sCopy := scheds[j]
						currentSched = &sCopy
						break
					}
				}
				break
			}
		}
	}

	// 3. Priority 3: Today's occurrence that is not yet completed
	if currentSched == nil {
		for _, occ := range occurrences {
			if occ.DateLabel == "hari_ini" && occ.Status != StatusSudahAbsenPulang {
				for j := range scheds {
					if scheds[j].ID == occ.ScheduleID {
						sCopy := scheds[j]
						currentSched = &sCopy
						break
					}
				}
				if occ.AttendanceID != "" {
					for j := range recent {
						if recent[j].ID == occ.AttendanceID {
							rCopy := recent[j]
							currentRec = &rCopy
							break
						}
					}
				}
				break
			}
		}
	}

	// 4. Fallback: First today occurrence or first schedule
	if currentSched == nil && len(occurrences) > 0 {
		for _, occ := range occurrences {
			if occ.DateLabel == "hari_ini" {
				for j := range scheds {
					if scheds[j].ID == occ.ScheduleID {
						sCopy := scheds[j]
						currentSched = &sCopy
						break
					}
				}
				if occ.AttendanceID != "" {
					for j := range recent {
						if recent[j].ID == occ.AttendanceID {
							rCopy := recent[j]
							currentRec = &rCopy
							break
						}
					}
				}
				break
			}
		}
	}
	if currentSched == nil && len(scheds) > 0 {
		sCopy := scheds[0]
		currentSched = &sCopy
	}

	overview.CurrentSchedule = currentSched
	overview.CurrentRecord = currentRec

	// Populate upcoming shifts (other active assignments)
	for i := range scheds {
		if currentSched == nil || scheds[i].ID != currentSched.ID {
			overview.UpcomingShifts = append(overview.UpcomingShifts, scheds[i])
		}
	}

	return overview, nil
}

func (s *Service) ExportAttendanceCSV(ctx context.Context, rtID string, f AttendanceFilter) ([]byte, error) {
	f.Page = 1
	f.PerPage = 1000
	rows, _, err := s.repo.ListAttendance(ctx, rtID, f)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{
		"Tanggal Shift", "Nama Petugas", "Shift", "Jam Mulai", "Jam Selesai",
		"Status", "Terlambat (Menit)", "Check-In (WIB)", "Pos Check-In", "Check-Out (WIB)", "Pos Check-Out",
	})
	for _, r := range rows {
		cin := "-"
		if r.CheckedInAt != nil {
			cin = r.CheckedInAt.In(JakartaLoc).Format("2006-01-02 15:04:05")
		}
		cout := "-"
		if r.CheckedOutAt != nil {
			cout = r.CheckedOutAt.In(JakartaLoc).Format("2006-01-02 15:04:05")
		}
		inDev := "-"
		if r.CheckInDeviceName != nil {
			inDev = *r.CheckInDeviceName
		}
		outDev := "-"
		if r.CheckOutDeviceName != nil {
			outDev = *r.CheckOutDeviceName
		}
		_ = w.Write([]string{
			r.ShiftDate, r.OfficerName, r.ShiftName, r.StartTime, r.EndTime,
			r.StatusLabel, fmt.Sprintf("%d", r.LateMinutes), cin, inDev, cout, outDev,
		})
	}
	w.Flush()
	return []byte(sb.String()), w.Error()
}

// ── Corrections ───────────────────────────────────────────────────────────────

func (s *Service) ListCorrections(ctx context.Context, rtID, officerFilter, statusFilter string) ([]AttendanceCorrection, error) {
	return s.repo.ListCorrections(ctx, rtID, officerFilter, statusFilter)
}

func (s *Service) CreateCorrection(ctx context.Context, rtID, requesterID string, isManager bool, req CreateCorrectionRequest) (*AttendanceCorrection, error) {
	if strings.TrimSpace(req.AttendanceRecordID) != "" {
		if _, err := uuid.Parse(req.AttendanceRecordID); err != nil {
			return nil, newAppError(400, "validation_error", "invalid attendance_record_id", ErrValidation)
		}
	} else if strings.TrimSpace(req.ScheduleID) != "" && strings.TrimSpace(req.ShiftDate) != "" {
		if _, err := uuid.Parse(req.ScheduleID); err != nil {
			return nil, newAppError(400, "validation_error", "invalid schedule_id", ErrValidation)
		}
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(req.ShiftDate)); err != nil {
			return nil, newAppError(400, "validation_error", "invalid shift_date, must be YYYY-MM-DD", ErrValidation)
		}
	} else {
		return nil, newAppError(400, "validation_error", "attendance_record_id or (schedule_id and shift_date) is required", ErrValidation)
	}
	if !ValidAttendanceStatus(req.RequestedStatus) {
		return nil, newAppError(400, "validation_error", "invalid requested_status", ErrValidation)
	}
	switch req.RequestType {
	case CorrectionTypeStatusChange, CorrectionTypeTimeAdjustment, CorrectionTypeManualCheckIn, CorrectionTypeManualCheckOut:
	default:
		return nil, newAppError(400, "validation_error", "invalid request_type", ErrValidation)
	}
	if len(strings.TrimSpace(req.Reason)) < 3 {
		return nil, newAppError(400, "validation_error", "reason must be at least 3 characters", ErrValidation)
	}

	var reqCin, reqCout *time.Time
	if req.RequestedCheckedInAt != nil && strings.TrimSpace(*req.RequestedCheckedInAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.RequestedCheckedInAt))
		if err != nil {
			return nil, newAppError(400, "validation_error", "requested_checked_in_at must be valid RFC3339", ErrValidation)
		}
		u := t.UTC()
		reqCin = &u
	}
	if req.RequestedCheckedOutAt != nil && strings.TrimSpace(*req.RequestedCheckedOutAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.RequestedCheckedOutAt))
		if err != nil {
			return nil, newAppError(400, "validation_error", "requested_checked_out_at must be valid RFC3339", ErrValidation)
		}
		u := t.UTC()
		reqCout = &u
	}
	if reqCin != nil && reqCout != nil && reqCout.Before(*reqCin) {
		return nil, newAppError(400, "validation_error", "requested_checked_out_at cannot be earlier than check-in", ErrValidation)
	}

	return s.repo.CreateCorrection(ctx, rtID, requesterID, isManager, req, reqCin, reqCout, s.NowUTC())
}

func (s *Service) DecideCorrection(ctx context.Context, rtID, correctionID, approverID string, req DecideCorrectionRequest, ip string) (*AttendanceCorrection, error) {
	if _, err := uuid.Parse(correctionID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid correction id", ErrValidation)
	}
	return s.repo.DecideCorrection(ctx, rtID, correctionID, approverID, req.Approve, req.Note, s.NowUTC(), ip)
}

// ── Guest Book ────────────────────────────────────────────────────────────────

func (s *Service) CreateGuestBook(ctx context.Context, rtID, deviceID string, req CreateGuestBookRequest) (*GuestBookEntry, error) {
	if len(strings.TrimSpace(req.GuestName)) < 2 || len(req.GuestName) > 120 {
		return nil, newAppError(400, "validation_error", "guest_name must be 2..120 characters", ErrValidation)
	}
	if len(strings.TrimSpace(req.Purpose)) < 2 || len(req.Purpose) > 250 {
		return nil, newAppError(400, "validation_error", "purpose must be 2..250 characters", ErrValidation)
	}
	if len(strings.TrimSpace(req.DestinationLabel)) < 1 || len(req.DestinationLabel) > 120 {
		return nil, newAppError(400, "validation_error", "destination_label is required", ErrValidation)
	}
	if req.Phone != nil {
		p := strings.TrimSpace(*req.Phone)
		if p == "" {
			req.Phone = nil
		} else if len(p) > 25 {
			return nil, newAppError(400, "validation_error", "phone number too long", ErrValidation)
		} else {
			req.Phone = &p
		}
	}
	if req.HostHouseholdID != nil {
		h := strings.TrimSpace(*req.HostHouseholdID)
		if h == "" {
			req.HostHouseholdID = nil
		} else if _, err := uuid.Parse(h); err != nil {
			return nil, newAppError(400, "validation_error", "invalid host_household_id", ErrValidation)
		} else {
			req.HostHouseholdID = &h
		}
	}
	return s.repo.CreateGuestBook(ctx, rtID, deviceID, req, s.NowUTC())
}

func (s *Service) CheckoutGuestBook(ctx context.Context, rtID, deviceID, entryID string) (*GuestBookEntry, error) {
	if _, err := uuid.Parse(entryID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid guest book entry id", ErrValidation)
	}
	return s.repo.CheckoutGuestBook(ctx, rtID, deviceID, entryID, s.NowUTC())
}

func (s *Service) ListGuestBooks(ctx context.Context, rtID string, f GuestBookFilter) ([]GuestBookEntry, int, error) {
	return s.repo.ListGuestBooks(ctx, rtID, f)
}

// ── Pos Device Helpers ────────────────────────────────────────────────────────

func (s *Service) ListPosHouseholds(ctx context.Context, rtID, search string, page, perPage int) ([]PosHouseholdSummary, int, error) {
	return s.repo.ListPosHouseholds(ctx, rtID, search, page, perPage)
}

func (s *Service) GetPosHouseholdDetail(ctx context.Context, rtID, occupancyID string) (*PosHouseholdDetail, error) {
	if _, err := uuid.Parse(occupancyID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid household occupancy id", ErrValidation)
	}
	return s.repo.GetPosHouseholdDetail(ctx, rtID, occupancyID)
}

// ── Dashboard & Scheduler ─────────────────────────────────────────────────────

func (s *Service) GetDashboardSummary(ctx context.Context, rtID string) (*SecurityDashboardSummary, error) {
	return s.repo.GetDashboardSummary(ctx, rtID, s.NowUTC())
}

func (s *Service) GetOnDutyOfficers(ctx context.Context, rtID string) ([]OnDutyOfficerSummary, error) {
	return s.repo.GetOnDutyOfficers(ctx, rtID, s.NowUTC())
}

func (s *Service) RunSchedulerTick(ctx context.Context) (*SchedulerTickResult, error) {
	return s.repo.RunSchedulerTick(ctx, s.NowUTC())
}

// ── Patrol & Paket Service Methods ──────────────────────────────────────────

func (s *Service) ListPatrolTemplates(ctx context.Context, rtID string) ([]PatrolTemplate, error) {
	return s.repo.ListPatrolTemplates(ctx, rtID)
}

func (s *Service) CreatePatrolTemplate(ctx context.Context, rtID, name, locationLabel, createdBy string) (*PatrolTemplate, error) {
	if strings.TrimSpace(name) == "" {
		return nil, newAppError(400, "empty_name", "nama template patroli wajib diisi", ErrValidation)
	}
	if strings.TrimSpace(locationLabel) == "" {
		return nil, newAppError(400, "empty_location", "lokasi patroli wajib diisi", ErrValidation)
	}
	return s.repo.CreatePatrolTemplate(ctx, rtID, name, locationLabel, createdBy)
}

func (s *Service) UpdatePatrolTemplate(ctx context.Context, rtID, templateID, name, locationLabel string, isActive *bool) (*PatrolTemplate, error) {
	return s.repo.UpdatePatrolTemplate(ctx, rtID, templateID, name, locationLabel, isActive)
}

func (s *Service) DeletePatrolTemplate(ctx context.Context, rtID, templateID string) error {
	return s.repo.DeletePatrolTemplate(ctx, rtID, templateID)
}

func (s *Service) CreatePatrolBundle(ctx context.Context, rtID, createdBy string, notes *string, photos []CreatePatrolPhoto) (*PatrolBundle, error) {
	return s.repo.CreatePatrolBundle(ctx, rtID, createdBy, notes, photos)
}

func (s *Service) ListPatrolBundles(ctx context.Context, rtID string, limit, offset int) ([]PatrolBundle, int, error) {
	return s.repo.ListPatrolBundles(ctx, rtID, limit, offset)
}

func (s *Service) GetPatrolBundle(ctx context.Context, rtID, bundleID string) (*PatrolBundle, error) {
	return s.repo.GetPatrolBundle(ctx, rtID, bundleID)
}

func (s *Service) CreatePaketReport(ctx context.Context, rtID, createdBy, receiverWargaID string, items []CreatePaketItem) (*PaketReport, error) {
	if strings.TrimSpace(receiverWargaID) == "" {
		return nil, newAppError(400, "empty_receiver", "warga penerima wajib dipilih", ErrValidation)
	}
	return s.repo.CreatePaketReport(ctx, rtID, createdBy, receiverWargaID, items)
}

func (s *Service) ListPaketReports(ctx context.Context, rtID string, limit, offset int) ([]PaketReport, int, error) {
	return s.repo.ListPaketReports(ctx, rtID, limit, offset)
}

func (s *Service) GetPaketReport(ctx context.Context, rtID, reportID string) (*PaketReport, error) {
	return s.repo.GetPaketReport(ctx, rtID, reportID)
}

func (s *Service) UpdatePaketItemStatus(ctx context.Context, rtID, reportID, itemID, newStatus, userID string, proofImageKey *string) error {
	return s.repo.UpdatePaketItemStatus(ctx, rtID, reportID, itemID, newStatus, userID, proofImageKey)
}

func (s *Service) SearchWargaMasked(ctx context.Context, rtID, query string, limit int) ([]MaskedWargaSearchResult, error) {
	return s.repo.SearchWargaMasked(ctx, rtID, query, limit)
}

func (s *Service) GetSecurityAlerts(ctx context.Context, rtID, userID string) (*SecurityAlertsResult, error) {
	return s.repo.GetSecurityAlerts(ctx, rtID, userID)
}

func (s *Service) MarkReadReceipt(ctx context.Context, userID, rtID, domain string, itemIDs []string) error {
	return s.repo.MarkReadReceipt(ctx, userID, rtID, domain, itemIDs)
}

func (s *Service) GetPatrolPhotoKey(ctx context.Context, rtID, photoID string) (string, error) {
	return s.repo.GetPatrolPhotoKey(ctx, rtID, photoID)
}

func (s *Service) GetPaketPhotoKey(ctx context.Context, rtID, photoID string) (string, error) {
	return s.repo.GetPaketPhotoKey(ctx, rtID, photoID)
}

func (s *Service) IsPaketReceiver(ctx context.Context, rtID, reportID, userID string) (bool, error) {
	return s.repo.IsPaketReceiver(ctx, rtID, reportID, userID)
}

func (s *Service) IsPaketPhotoReceiver(ctx context.Context, rtID, photoID, userID string) (bool, error) {
	return s.repo.IsPaketPhotoReceiver(ctx, rtID, photoID, userID)
}
