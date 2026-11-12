package security

import (
	"fmt"
	"strings"
	"time"
)

const (
	// Attendance statuses
	StatusBelumHadir       = "belum_hadir"
	StatusHadir            = "hadir"
	StatusTerlambat        = "terlambat"
	StatusSudahAbsenPulang = "sudah_absen_pulang"
	StatusTidakHadir       = "tidak_hadir"
	StatusBelumAbsenPulang = "belum_absen_pulang"

	// Shift swap workflow statuses
	SwapPendingTarget  = "pending_target"
	SwapRejectedTarget = "rejected_target"
	SwapPendingKetua   = "pending_ketua"
	SwapApproved       = "approved"
	SwapRejectedKetua  = "rejected_ketua"
	SwapCancelled      = "cancelled"

	// Correction statuses
	CorrectionPending  = "pending"
	CorrectionApproved = "approved"
	CorrectionRejected = "rejected"

	// Correction request types
	CorrectionTypeStatusChange   = "status_change"
	CorrectionTypeTimeAdjustment = "time_adjustment"
	CorrectionTypeManualCheckIn  = "manual_check_in"
	CorrectionTypeManualCheckOut = "manual_check_out"

	// Schedule statuses
	ScheduleStatusScheduled = "scheduled"
	ScheduleStatusCancelled = "cancelled"

	// Token TTLs
	DefaultQRTTL      = 45 * time.Second
	DefaultPairingTTL = 24 * time.Hour

	// Scheduler advisory lock key (unique arbitrary int64)
	SchedulerAdvisoryLockID int64 = 87451209
)

// JakartaLoc is the canonical timezone for all security attendance time calculations.
var JakartaLoc = loadJakartaLocation()

func loadJakartaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

// NowJakarta returns the current time in Asia/Jakarta.
func NowJakarta(nowFn func() time.Time) time.Time {
	if nowFn != nil {
		return nowFn().In(JakartaLoc)
	}
	return time.Now().In(JakartaLoc)
}

// StatusLabel returns the human-readable Indonesian label for an attendance status.
func StatusLabel(status string) string {
	switch status {
	case StatusBelumHadir:
		return "Belum Hadir"
	case StatusHadir:
		return "Hadir"
	case StatusTerlambat:
		return "Terlambat"
	case StatusSudahAbsenPulang:
		return "Sudah Absen Pulang"
	case StatusTidakHadir:
		return "Tidak Hadir"
	case StatusBelumAbsenPulang:
		return "Belum Absen Pulang"
	default:
		return status
	}
}

// ValidAttendanceStatus checks if a string is a recognized attendance status.
func ValidAttendanceStatus(s string) bool {
	switch s {
	case StatusBelumHadir, StatusHadir, StatusTerlambat,
		StatusSudahAbsenPulang, StatusTidakHadir, StatusBelumAbsenPulang:
		return true
	default:
		return false
	}
}

// ParseHHMM validates and normalizes a wall-clock time string ("HH:MM" or "HH:MM:SS") to "HH:MM:SS".
func ParseHHMM(raw string) (hour int, minute int, normalized string, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, 0, "", fmt.Errorf("time is required")
	}
	var sec int
	var parsed time.Time
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, parseErr := time.Parse(layout, trimmed); parseErr == nil {
			parsed = t
			hour = parsed.Hour()
			minute = parsed.Minute()
			sec = parsed.Second()
			return hour, minute, fmt.Sprintf("%02d:%02d:%02d", hour, minute, sec), nil
		}
	}
	return 0, 0, "", fmt.Errorf("invalid time format %q, expected HH:MM", raw)
}

// ComputeShiftBounds computes exact Asia/Jakarta UTC instants for a shift date and wall-clock boundaries.
// When crossesMidnight is true (or end <= start), scheduledEndAt advances to the following calendar day.
func ComputeShiftBounds(shiftDate time.Time, startHHMM, endHHMM string, crossesMidnight bool) (scheduledStartAt, scheduledEndAt time.Time, effectiveCrossesMidnight bool, err error) {
	sh, sm, _, err := ParseHHMM(startHHMM)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("start_time: %w", err)
	}
	eh, em, _, err := ParseHHMM(endHHMM)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("end_time: %w", err)
	}

	y, m, d := shiftDate.In(JakartaLoc).Date()
	startAt := time.Date(y, m, d, sh, sm, 0, 0, JakartaLoc)
	endAt := time.Date(y, m, d, eh, em, 0, 0, JakartaLoc)

	effectiveCrossesMidnight = crossesMidnight || !endAt.After(startAt)
	if !endAt.After(startAt) {
		endAt = endAt.AddDate(0, 0, 1)
	}
	if !endAt.After(startAt) {
		return time.Time{}, time.Time{}, false, fmt.Errorf("shift end time must be after start time")
	}
	return startAt.UTC(), endAt.UTC(), effectiveCrossesMidnight, nil
}

// ── Domain Models ────────────────────────────────────────────────────────────

type SecurityDevice struct {
	ID                  string     `json:"id"`
	RTID                string     `json:"rt_id"`
	RTName              string     `json:"rt_name,omitempty"`
	RTCode              string     `json:"rt_code,omitempty"`
	RWID                *string    `json:"rw_id,omitempty"`
	DeviceName          string     `json:"device_name"`
	LocationLabel       string     `json:"location_label"`
	IsActive            bool       `json:"is_active"`
	IsPaired            bool       `json:"is_paired"`
	ActivationExpiresAt *time.Time `json:"activation_expires_at,omitempty"`
	ActivatedAt         *time.Time `json:"activated_at,omitempty"`
	LastActiveAt        *time.Time `json:"last_active_at,omitempty"`
	RevokedAt           *time.Time `json:"revoked_at,omitempty"`
	CreatedBy           string     `json:"created_by"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type DeviceContext struct {
	DeviceID      string
	RTID          string
	RTName        string
	RTCode        string
	RWID          string
	DeviceName    string
	LocationLabel string
}

type DeviceProvisionResult struct {
	Device       SecurityDevice `json:"device"`
	PairingToken string         `json:"pairing_token"`
	ExpiresAt    time.Time      `json:"expires_at"`
}

type DeviceActivationResult struct {
	Device      SecurityDevice `json:"device"`
	DeviceToken string         `json:"device_token"`
}

type ShiftConfig struct {
	ID                           string    `json:"id"`
	RTID                         string    `json:"rt_id"`
	Name                         string    `json:"name"`
	StartTime                    string    `json:"start_time"`
	EndTime                      string    `json:"end_time"`
	CrossesMidnight              bool      `json:"crosses_midnight"`
	LatenessToleranceMinutes     int       `json:"lateness_tolerance_minutes"`
	CheckInWindowBeforeMinutes   int       `json:"check_in_window_before_minutes"`
	CheckInWindowAfterMinutes    int       `json:"check_in_window_after_minutes"`
	AbsentCutoffMinutes          int       `json:"absent_cutoff_minutes"`
	MissingCheckoutCutoffMinutes int       `json:"missing_checkout_cutoff_minutes"`
	Version                      int       `json:"version"`
	IsActive                     bool      `json:"is_active"`
	CreatedBy                    string    `json:"created_by"`
	CreatedAt                    time.Time `json:"created_at"`
	UpdatedAt                    time.Time `json:"updated_at"`
}

type SecurityOfficer struct {
	UserID       string  `json:"user_id"`
	MembershipID string  `json:"membership_id"`
	RTID         string  `json:"rt_id"`
	ResidentID   *string `json:"resident_id,omitempty"`
	FullName     string  `json:"full_name"`
	Email        string  `json:"email"`
	Phone        *string `json:"phone,omitempty"`
	IsActive     bool    `json:"is_active"`
	AvatarPath   *string `json:"avatar_path,omitempty"`
}

type SecuritySchedule struct {
	ID                           string     `json:"id"`
	AssignmentID                 string     `json:"assignment_id"`
	AssignmentStartDate          string     `json:"assignment_start_date"`
	AssignmentEndDate            string     `json:"assignment_end_date"`
	RTID                         string     `json:"rt_id"`
	UserID                       string     `json:"user_id"`
	OfficerName                  string     `json:"officer_name,omitempty"`
	MembershipID                 string     `json:"membership_id"`
	ShiftConfigID                string     `json:"shift_config_id"`
	ShiftVersion                 int        `json:"shift_version"`
	ShiftDate                    string     `json:"shift_date"`
	ShiftName                    string     `json:"shift_name"`
	StartTime                    string     `json:"start_time"`
	EndTime                      string     `json:"end_time"`
	CrossesMidnight              bool       `json:"crosses_midnight"`
	LatenessToleranceMinutes     int        `json:"lateness_tolerance_minutes"`
	CheckInWindowBeforeMinutes   int        `json:"check_in_window_before_minutes"`
	CheckInWindowAfterMinutes    int        `json:"check_in_window_after_minutes"`
	AbsentCutoffMinutes          int        `json:"absent_cutoff_minutes"`
	MissingCheckoutCutoffMinutes int        `json:"missing_checkout_cutoff_minutes"`
	ScheduledStartAt             time.Time  `json:"scheduled_start_at"`
	ScheduledEndAt               time.Time  `json:"scheduled_end_at"`
	Notes                        *string    `json:"notes,omitempty"`
	Status                       string     `json:"status"`
	AttendanceStatus             string     `json:"attendance_status,omitempty"`
	AttendanceStatusLabel        string     `json:"attendance_status_label,omitempty"`
	CheckedInAt                  *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt                 *time.Time `json:"checked_out_at,omitempty"`
	CreatedAt                    time.Time  `json:"created_at"`
	UpdatedAt                    time.Time  `json:"updated_at"`
}

type ShiftSwap struct {
	ID                  string     `json:"id"`
	RTID                string     `json:"rt_id"`
	RequesterUserID     string     `json:"requester_user_id"`
	RequesterName       string     `json:"requester_name,omitempty"`
	TargetUserID        string     `json:"target_user_id"`
	TargetName          string     `json:"target_name,omitempty"`
	RequesterScheduleID string     `json:"requester_schedule_id"`
	RequesterShiftDate  string     `json:"requester_shift_date,omitempty"`
	RequesterShiftName  string     `json:"requester_shift_name,omitempty"`
	TargetScheduleID    string     `json:"target_schedule_id"`
	TargetShiftDate     string     `json:"target_shift_date,omitempty"`
	TargetShiftName     string     `json:"target_shift_name,omitempty"`
	Reason              string     `json:"reason"`
	Status              string     `json:"status"`
	TargetRespondedAt   *time.Time `json:"target_responded_at,omitempty"`
	TargetNote          *string    `json:"target_note,omitempty"`
	KetuaDecidedBy      *string    `json:"ketua_decided_by,omitempty"`
	KetuaDecidedAt      *time.Time `json:"ketua_decided_at,omitempty"`
	KetuaNote           *string    `json:"ketua_note,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type DynamicQRPayload struct {
	TokenID          string    `json:"token_id"`
	QRToken          string    `json:"qr_token"`
	DeviceID         string    `json:"device_id"`
	DeviceName       string    `json:"device_name"`
	RTID             string    `json:"rt_id"`
	IssuedAt         time.Time `json:"issued_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	ExpiresInSeconds int       `json:"expires_in_seconds"`
}

type AttendanceRecord struct {
	ID                      string     `json:"id"`
	RTID                    string     `json:"rt_id"`
	ScheduleID              string     `json:"schedule_id"`
	UserID                  string     `json:"user_id"`
	OfficerName             string     `json:"officer_name,omitempty"`
	ShiftConfigID           string     `json:"shift_config_id"`
	ShiftVersion            int        `json:"shift_version"`
	ShiftDate               string     `json:"shift_date"`
	ShiftName               string     `json:"shift_name"`
	StartTime               string     `json:"start_time"`
	EndTime                 string     `json:"end_time"`
	CrossesMidnight         bool       `json:"crosses_midnight"`
	ScheduledStartAt        time.Time  `json:"scheduled_start_at"`
	ScheduledEndAt          time.Time  `json:"scheduled_end_at"`
	Status                  string     `json:"status"`
	StatusLabel             string     `json:"status_label"`
	ArrivalStatus           *string    `json:"arrival_status,omitempty"`
	LateMinutes             int        `json:"late_minutes"`
	CheckInDeviceID         *string    `json:"check_in_device_id,omitempty"`
	CheckInDeviceName       *string    `json:"check_in_device_name,omitempty"`
	CheckOutDeviceID        *string    `json:"check_out_device_id,omitempty"`
	CheckOutDeviceName      *string    `json:"check_out_device_name,omitempty"`
	CheckedInAt             *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt            *time.Time `json:"checked_out_at,omitempty"`
	MarkedAbsentAt          *time.Time `json:"marked_absent_at,omitempty"`
	MarkedMissingCheckoutAt *time.Time `json:"marked_missing_checkout_at,omitempty"`
	CorrectionLocked        bool       `json:"correction_locked"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type OfficerAttendanceOverview struct {
	ServerTime      time.Time          `json:"server_time"`
	Timezone        string             `json:"timezone"`
	CurrentSchedule *SecuritySchedule  `json:"current_schedule,omitempty"`
	CurrentRecord   *AttendanceRecord  `json:"current_record,omitempty"`
	UpcomingShifts  []SecuritySchedule `json:"upcoming_shifts"`
	RecentHistory   []AttendanceRecord `json:"recent_history"`
}

type AttendanceScanResult struct {
	Action     string           `json:"action"`
	Record     AttendanceRecord `json:"record"`
	ServerTime time.Time        `json:"server_time"`
	DeviceID   string           `json:"device_id"`
	DeviceName string           `json:"device_name"`
	Message    string           `json:"message"`
}

type AttendanceCorrection struct {
	ID                    string     `json:"id"`
	RTID                  string     `json:"rt_id"`
	AttendanceRecordID    string     `json:"attendance_record_id"`
	ScheduleID            string     `json:"schedule_id"`
	OfficerUserID         string     `json:"officer_user_id"`
	OfficerName           string     `json:"officer_name,omitempty"`
	RequestedBy           string     `json:"requested_by"`
	RequestedByName       string     `json:"requested_by_name,omitempty"`
	ShiftDate             string     `json:"shift_date,omitempty"`
	ShiftName             string     `json:"shift_name,omitempty"`
	RequestType           string     `json:"request_type"`
	OriginalStatus        string     `json:"original_status"`
	OriginalCheckedInAt   *time.Time `json:"original_checked_in_at,omitempty"`
	OriginalCheckedOutAt  *time.Time `json:"original_checked_out_at,omitempty"`
	RequestedStatus       string     `json:"requested_status"`
	RequestedCheckedInAt  *time.Time `json:"requested_checked_in_at,omitempty"`
	RequestedCheckedOutAt *time.Time `json:"requested_checked_out_at,omitempty"`
	Reason                string     `json:"reason"`
	Status                string     `json:"status"`
	DecidedBy             *string    `json:"decided_by,omitempty"`
	DecidedByName         *string    `json:"decided_by_name,omitempty"`
	DecidedAt             *time.Time `json:"decided_at,omitempty"`
	DecisionNote          *string    `json:"decision_note,omitempty"`
	FinalStatus           *string    `json:"final_status,omitempty"`
	FinalCheckedInAt      *time.Time `json:"final_checked_in_at,omitempty"`
	FinalCheckedOutAt     *time.Time `json:"final_checked_out_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type GuestBookEntry struct {
	ID               string     `json:"id"`
	RTID             string     `json:"rt_id"`
	DeviceID         string     `json:"device_id"`
	DeviceName       string     `json:"device_name,omitempty"`
	GuestName        string     `json:"guest_name"`
	Phone            *string    `json:"phone,omitempty"`
	PhoneMasked      *string    `json:"phone_masked,omitempty"`
	Purpose          string     `json:"purpose"`
	HostHouseholdID  *string    `json:"host_household_id,omitempty"`
	DestinationLabel string     `json:"destination_label"`
	VehiclePlate     *string    `json:"vehicle_plate,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
	CheckedInAt      time.Time  `json:"checked_in_at"`
	CheckedOutAt     *time.Time `json:"checked_out_at,omitempty"`
	CheckoutDeviceID *string    `json:"checkout_device_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type PosHouseholdSummary struct {
	HouseholdOccupancyID string  `json:"household_occupancy_id"`
	HouseNumber          string  `json:"house_number"`
	Block                *string `json:"block,omitempty"`
	HeadOfFamilyName     string  `json:"head_of_family_name"`
	OccupancyType        string  `json:"occupancy_type"`
	ResidentCount        int     `json:"resident_count"`
	// Avatar of the resident whose full_name matches the head of family, else empty.
	// Opaque object-storage key; resolvable only via the device-scoped image endpoint.
	HeadAvatarPath *string `json:"head_avatar_path,omitempty"`
}

type PosResidentSummary struct {
	ID              string `json:"id"`
	FullName        string `json:"full_name"`
	Relationship    string `json:"relationship"`
	IsHeadOfFamily  bool   `json:"is_head_of_family"`
	ResidencyStatus string `json:"residency_status"`
	// Opaque object-storage key; resolvable only via the device-scoped image endpoint.
	AvatarPath *string `json:"avatar_path,omitempty"`
}

type PosHouseholdDetail struct {
	PosHouseholdSummary
	Residents []PosResidentSummary `json:"residents"`
}

type SecurityDashboardSummary struct {
	ServerTime           time.Time          `json:"server_time"`
	TodayDate            string             `json:"today_date"`
	TotalOfficers        int                `json:"total_officers"`
	ActiveShiftsCount    int                `json:"active_shifts_count"`
	ScheduledTodayCount  int                `json:"scheduled_today_count"`
	PresentTodayCount    int                `json:"present_today_count"`
	LateTodayCount       int                `json:"late_today_count"`
	CheckedOutTodayCount int                `json:"checked_out_today_count"`
	AbsentTodayCount     int                `json:"absent_today_count"`
	MissingCheckoutCount int                `json:"missing_checkout_count"`
	PendingSwapsCount    int                `json:"pending_swaps_count"`
	PendingCorrections   int                `json:"pending_corrections"`
	ActiveDevicesCount   int                `json:"active_devices_count"`
	ActiveGuestsCount    int                `json:"active_guests_count"`
	TodayAttendance      []AttendanceRecord `json:"today_attendance"`
}

// OnDutyOfficerSummary is used by Beranda HomeDashboard integration without coupling modules.
type OnDutyOfficerSummary struct {
	UserID      string
	FullName    string
	ShiftName   string
	ShiftStart  string
	ShiftEnd    string
	Status      string
	PhoneNumber *string
	AvatarPath  *string `json:"avatar_path,omitempty"`
}

// ── Request Payloads ──────────────────────────────────────────────────────────

type CreateDeviceRequest struct {
	DeviceName    string `json:"device_name"`
	LocationLabel string `json:"location_label"`
}

type ActivateDeviceRequest struct {
	PairingToken string `json:"pairing_token"`
}

type UpsertShiftRequest struct {
	Name                         string `json:"name"`
	StartTime                    string `json:"start_time"`
	EndTime                      string `json:"end_time"`
	CrossesMidnight              bool   `json:"crosses_midnight"`
	LatenessToleranceMinutes     int    `json:"lateness_tolerance_minutes"`
	CheckInWindowBeforeMinutes   int    `json:"check_in_window_before_minutes"`
	CheckInWindowAfterMinutes    int    `json:"check_in_window_after_minutes"`
	AbsentCutoffMinutes          int    `json:"absent_cutoff_minutes"`
	MissingCheckoutCutoffMinutes int    `json:"missing_checkout_cutoff_minutes"`
	IsActive                     *bool  `json:"is_active"`
}

type CreateScheduleRequest struct {
	UserID        string  `json:"user_id"`
	ShiftConfigID string  `json:"shift_config_id"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	ShiftDate     string  `json:"shift_date,omitempty"` // Legacy one-day client compatibility.
	Notes         *string `json:"notes"`
}

type UpdateScheduleRequest struct {
	UserID        string  `json:"user_id"`
	ShiftConfigID string  `json:"shift_config_id"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	ShiftDate     string  `json:"shift_date,omitempty"` // Legacy one-day client compatibility.
	Notes         *string `json:"notes"`
}

type CreateSwapRequest struct {
	RequesterScheduleID string `json:"requester_schedule_id"`
	TargetScheduleID    string `json:"target_schedule_id"`
	Reason              string `json:"reason"`
}

type SwapTargetRespondRequest struct {
	Accept bool    `json:"accept"`
	Note   *string `json:"note"`
}

type SwapDecideRequest struct {
	Approve bool    `json:"approve"`
	Note    *string `json:"note"`
}

type ScanQRRequest struct {
	QRToken string `json:"qr_token"`
	Action  string `json:"action"` // "check_in", "check_out", or "auto"/empty
}

type CreateCorrectionRequest struct {
	AttendanceRecordID    string  `json:"attendance_record_id"`
	RequestType           string  `json:"request_type"`
	RequestedStatus       string  `json:"requested_status"`
	RequestedCheckedInAt  *string `json:"requested_checked_in_at,omitempty"`
	RequestedCheckedOutAt *string `json:"requested_checked_out_at,omitempty"`
	Reason                string  `json:"reason"`
}

type DecideCorrectionRequest struct {
	Approve bool    `json:"approve"`
	Note    *string `json:"note"`
}

type CreateGuestBookRequest struct {
	GuestName        string  `json:"guest_name"`
	Phone            *string `json:"phone,omitempty"`
	Purpose          string  `json:"purpose"`
	HostHouseholdID  *string `json:"host_household_id,omitempty"`
	DestinationLabel string  `json:"destination_label"`
	VehiclePlate     *string `json:"vehicle_plate,omitempty"`
	Notes            *string `json:"notes,omitempty"`
}

// MaskPhone protects PII by masking middle digits of phone numbers.
func MaskPhone(raw *string) *string {
	if raw == nil {
		return nil
	}
	v := strings.TrimSpace(*raw)
	if v == "" {
		return nil
	}
	runes := []rune(v)
	if len(runes) <= 4 {
		masked := strings.Repeat("*", len(runes))
		return &masked
	}
	prefix := string(runes[:3])
	suffix := string(runes[len(runes)-3:])
	mid := strings.Repeat("*", len(runes)-6)
	if len(runes) <= 6 {
		prefix = string(runes[:2])
		suffix = string(runes[len(runes)-2:])
		mid = strings.Repeat("*", len(runes)-4)
	}
	res := prefix + mid + suffix
	return &res
}
