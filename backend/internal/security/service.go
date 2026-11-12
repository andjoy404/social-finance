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
	start, end, err := normalizeScheduleDates(req.StartDate, req.EndDate, req.ShiftDate)
	if err != nil {
		return nil, err
	}
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
	return s.repo.ConsumeQRAndRecordAttendance(ctx, rtID, userID, req.QRToken, req.Action, idemKey, s.NowUTC())
}

// ── Attendance Queries & Officer Overview ─────────────────────────────────────

func (s *Service) ListAttendance(ctx context.Context, rtID string, f AttendanceFilter) ([]AttendanceRecord, int, error) {
	return s.repo.ListAttendance(ctx, rtID, f)
}

func (s *Service) GetOfficerOverview(ctx context.Context, rtID, userID string) (*OfficerAttendanceOverview, error) {
	now := s.Now()
	todayStr := now.Format("2006-01-02")
	yesterdayStr := now.AddDate(0, 0, -1).Format("2006-01-02")
	nextWeekStr := now.AddDate(0, 0, 7).Format("2006-01-02")

	scheds, _, err := s.repo.ListSchedules(ctx, rtID, ScheduleFilter{
		DateFrom: yesterdayStr,
		DateTo:   nextWeekStr,
		UserID:   userID,
		Page:     1,
		PerPage:  30,
	})
	if err != nil {
		return nil, err
	}

	recent, _, err := s.repo.ListAttendance(ctx, rtID, AttendanceFilter{
		UserID:  userID,
		Page:    1,
		PerPage: 20,
	})
	if err != nil {
		return nil, err
	}

	overview := &OfficerAttendanceOverview{
		ServerTime:     now,
		Timezone:       "Asia/Jakarta",
		UpcomingShifts: []SecuritySchedule{},
		RecentHistory:  recent,
	}

	nowUTC := now.UTC()
	var currentSched *SecuritySchedule
	for i := range scheds {
		sc := &scheds[i]
		// Match an open shift first (checked in but not checked out, or within active check-in window).
		openAt := sc.ScheduledStartAt.Add(-time.Duration(sc.CheckInWindowBeforeMinutes) * time.Minute)
		closeOutAt := sc.ScheduledEndAt.Add(time.Duration(sc.MissingCheckoutCutoffMinutes) * time.Minute)
		inWindow := !nowUTC.Before(openAt) && !nowUTC.After(closeOutAt)
		if currentSched == nil && inWindow && (sc.AttendanceStatus == StatusBelumHadir || sc.AttendanceStatus == StatusHadir || sc.AttendanceStatus == StatusTerlambat) {
			copySc := *sc
			currentSched = &copySc
			continue
		}
		if sc.ShiftDate >= todayStr {
			overview.UpcomingShifts = append(overview.UpcomingShifts, *sc)
		}
	}
	if currentSched != nil {
		overview.CurrentSchedule = currentSched
		for i := range recent {
			if recent[i].ScheduleID == currentSched.ID {
				recCopy := recent[i]
				overview.CurrentRecord = &recCopy
				break
			}
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
	if _, err := uuid.Parse(req.AttendanceRecordID); err != nil {
		return nil, newAppError(400, "validation_error", "invalid attendance_record_id", ErrValidation)
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
