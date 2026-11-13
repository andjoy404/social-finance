package notification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Event types supported by the notification system.
const (
	EventBillCreated       = "iuran_bill_created"
	EventPaymentSubmitted  = "iuran_payment_submitted"
	EventPaymentVerified   = "iuran_payment_verified"
	EventWasteTruckArrival = "waste_truck_arrival"
	EventManualPush        = "manual_push"
	// EventPaymentReminder is the auto reminder for unpaid obligatory iuran bills.
	EventPaymentReminder = "iuran_payment_reminder"
	// EventUpcomingDue3Months is the auto reminder sent once when an obligatory bill enters 3 calendar months before due_date.
	EventUpcomingDue3Months = "iuran_upcoming_due_3_months"
	// EventOverdue3Months is the auto reminder sent when an obligatory bill is > 3 calendar months past due_date (and monthly thereafter).
	EventOverdue3Months = "iuran_overdue_3_months"

	// Activity events (kegiatan). Customizable, can be scheduled one-shot via
	// notification_schedules.activity_id + execution_at and fired immediately
	// from the community activity lifecycle.
	EventActivityCreated   = "activity_created"
	EventActivityUpdated   = "activity_updated"
	EventActivityReminder  = "activity_reminder"
	EventActivityCancelled = "activity_cancelled"
	EventActivityCompleted = "activity_completed"

	// RW notification event types
	EventRWCashIn  = "rw_cash_in"
	EventRWCashOut = "rw_cash_out"
)

const (
	HistoryCategorySystem = "system"
	HistoryCategoryMine   = "mine"
)

// Device platforms.
const (
	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformWeb     = "web"
)

// RegisterDeviceInput represents client request to register an FCM device token.
type RegisterDeviceInput struct {
	FCMToken string  `json:"fcm_token"`
	Platform string  `json:"platform"`
	DeviceID *string `json:"device_id,omitempty"`
}

// UnregisterDeviceInput represents client request to unregister an FCM token.
type UnregisterDeviceInput struct {
	FCMToken string `json:"fcm_token"`
}

// UserDevice represents a registered device row in database.
type UserDevice struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	FCMToken   string    `json:"fcm_token"`
	Platform   string    `json:"platform"`
	DeviceID   *string   `json:"device_id,omitempty"`
	IsActive   bool      `json:"is_active"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// WasteTruckNotificationInput represents manual trigger by Pengurus/Security.
type WasteTruckNotificationInput struct {
	Message        string  `json:"message,omitempty"`
	ImageURL       *string `json:"image_url,omitempty"`
	IdempotencyKey string  `json:"idempotency_key,omitempty"`
}

// ManualPushInput represents manual push notification input to broadcast to all RT residents.
type ManualPushInput struct {
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	ImageURL *string `json:"image_url,omitempty"`
	DeepLink string  `json:"deep_link,omitempty"`
}

// TestPushInput represents superadmin-only test push notification input.
type TestPushInput struct {
	EventType string  `json:"event_type"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	ImageURL  *string `json:"image_url,omitempty"`
}

// NotificationPayload encapsulates minimal identifiers sent via FCM data payload.
// NEVER includes amounts, NIK, or confidential officer notes.
type NotificationPayload struct {
	NotificationID string  `json:"notification_id,omitempty"`
	Type           string  `json:"type"`
	RTID           string  `json:"rt_id"`
	Title          string  `json:"title,omitempty"`
	Body           string  `json:"body,omitempty"`
	BillID         string  `json:"bill_id,omitempty"`
	PaymentID      string  `json:"payment_id,omitempty"`
	Status         string  `json:"status,omitempty"`
	DeepLink       string  `json:"deep_link,omitempty"`
	ImageURL       *string `json:"image_url,omitempty"`
	SentAt         string  `json:"sent_at,omitempty"`
}

// ToMap converts NotificationPayload to string map for FCM Data payload.
func (p NotificationPayload) ToMap() map[string]string {
	m := map[string]string{
		"type":  p.Type,
		"rt_id": p.RTID,
	}
	if p.NotificationID != "" {
		m["notification_id"] = p.NotificationID
	}
	if p.Title != "" {
		m["title"] = p.Title
	}
	if p.Body != "" {
		m["body"] = p.Body
	}
	if p.BillID != "" {
		m["bill_id"] = p.BillID
	}
	if p.PaymentID != "" {
		m["payment_id"] = p.PaymentID
	}
	if p.Status != "" {
		m["status"] = p.Status
	}
	if p.DeepLink != "" {
		m["deep_link"] = p.DeepLink
	}
	if p.ImageURL != nil && *p.ImageURL != "" {
		m["image_url"] = *p.ImageURL
	}
	if p.SentAt != "" {
		m["sent_at"] = p.SentAt
	}
	return m
}

// SystemNotificationType represents a documented system event type.
type SystemNotificationType struct {
	EventType        string   `json:"event_type"`
	Name             string   `json:"name"`
	Trigger          string   `json:"trigger"`
	Channel          string   `json:"channel"`
	RecipientsRule   string   `json:"recipients_rule"`
	DefaultTitle     string   `json:"default_title"`
	DefaultBody      string   `json:"default_body"`
	AllowedVariables []string `json:"allowed_variables"`
	IsConfigurable   bool     `json:"is_configurable"`
	SupportsImage    bool     `json:"supports_image"`
	SupportsTest     bool     `json:"supports_test"`
}

// NotificationTemplate represents a configurable template row in database.
type NotificationTemplate struct {
	ID        string    `json:"id"`
	RTID      string    `json:"rt_id,omitempty"`
	RWID      string    `json:"rw_id,omitempty"`
	Name      string    `json:"name"`
	EventType string    `json:"event_type"`
	Channel   string    `json:"channel"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	ImageURL  *string   `json:"image_url,omitempty"`
	IsActive  bool      `json:"is_active"`
	Variables []string  `json:"variables"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateTemplateInput represents payload to create a new template.
type CreateTemplateInput struct {
	Name      string  `json:"name"`
	EventType string  `json:"event_type"`
	Channel   string  `json:"channel"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	ImageURL  *string `json:"image_url,omitempty"`
}

// UpdateTemplateInput represents payload to edit an existing template.
type UpdateTemplateInput struct {
	Name     string  `json:"name"`
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	ImageURL *string `json:"image_url,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// NotificationLogEntry represents a delivery log row.
type NotificationLogEntry struct {
	ID                      string         `json:"id"`
	RTID                    string         `json:"rt_id,omitempty"`
	RWID                    *string        `json:"rw_id,omitempty"`
	UserID                  *string        `json:"user_id,omitempty"`
	SenderUserID            *string        `json:"-"`
	EventType               string         `json:"event_type"`
	DedupKey                string         `json:"dedup_key"`
	Title                   string         `json:"title"`
	Body                    string         `json:"body"`
	Payload                 map[string]any `json:"payload,omitempty"`
	ImageURL                *string        `json:"image_url,omitempty"`
	Status                  string         `json:"status"`
	CreatedAt               time.Time      `json:"created_at"`
	IsRead                  bool           `json:"is_read"`
	ReadAt                  *time.Time     `json:"read_at,omitempty"`
	HistoryCategory         string         `json:"history_category,omitempty"`
	ActivityRelation        string         `json:"activity_relation,omitempty"`
	RecipientsCount         int            `json:"recipients_count,omitempty"`
	SentCount               int            `json:"sent_count,omitempty"`
	FailedCount             int            `json:"failed_count,omitempty"`
	DeliveryCountsAvailable bool           `json:"delivery_counts_available"`
}

// WasteTruckArrivalRecipient contains a current household resident and their
// active devices. Residents without a device are retained for history.
type WasteTruckArrivalRecipient struct {
	UserID       string
	UserFullName string
	Tokens       []string
}

// WasteTruckDispatchEntry is an audit record for a sender's waste-truck dispatch.
type WasteTruckDispatchEntry struct {
	ID            string    `json:"id"`
	EventType     string    `json:"event_type"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	ImageURL      *string   `json:"image_url,omitempty"`
	Status        string    `json:"status"`
	FailureReason *string   `json:"failure_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ScheduleExecutionSummary captures aggregate recipient counts and failure reasons for a schedule execution.
type ScheduleExecutionSummary struct {
	TotalRecipients int    `json:"total_recipients"`
	SentCount       int    `json:"sent_count"`
	FailedCount     int    `json:"failed_count"`
	NoDeviceCount   int    `json:"no_device_count"`
	FailureReason   string `json:"failure_reason,omitempty"`
}

// NotificationSchedule represents a configurable notification schedule row.
type NotificationSchedule struct {
	ID         string `json:"id"`
	RTID       string `json:"rt_id"`
	EventType  string `json:"event_type"`
	TemplateID string `json:"template_id"`
	Name       string `json:"name"`
	IsActive   bool   `json:"is_active"`
	SendDays   []int  `json:"send_days"`
	SendHour   int    `json:"send_hour"`
	SendMinute int    `json:"send_minute"`
	Timezone   string `json:"timezone"`
	// ActivityID — when set, this schedule is a one-shot reminder tied to a specific activity (kegiatan).
	ActivityID *string `json:"activity_id,omitempty"`
	// ExecutionAt — absolute time at which a one-shot activity reminder should fire (null for recurring schedules).
	ExecutionAt         *time.Time                `json:"execution_at,omitempty"`
	LastExecutedAt      *time.Time                `json:"last_executed_at,omitempty"`
	LastExecutedStatus  *string                   `json:"last_executed_status,omitempty"`
	LastExecutedError   *string                   `json:"last_executed_error,omitempty"`
	LastExecutedSummary *ScheduleExecutionSummary `json:"last_executed_summary,omitempty"`
	CreatedBy           string                    `json:"created_by,omitempty"`
	UpdatedBy           string                    `json:"updated_by,omitempty"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
}

// CreateScheduleInput represents payload to create a new schedule.
type CreateScheduleInput struct {
	EventType  string `json:"event_type"`
	TemplateID string `json:"template_id"`
	Name       string `json:"name"`
	// SendDays is a list of day-of-month (1-31).
	SendDays   []int  `json:"send_days"`
	SendHour   int    `json:"send_hour"`
	SendMinute int    `json:"send_minute"`
	Timezone   string `json:"timezone"`
	// ActivityID is only valid for the activity_reminder event (one-shot reminder tied to an activity).
	ActivityID *string `json:"activity_id,omitempty"`
	// ExecutionAt is only valid for the activity_reminder event — absolute time the reminder fires.
	ExecutionAt *time.Time `json:"execution_at,omitempty"`
}

// UpdateScheduleInput represents payload to edit an existing schedule.
type UpdateScheduleInput struct {
	Name        *string    `json:"name,omitempty"`
	IsActive    *bool      `json:"is_active,omitempty"`
	SendDays    *[]int     `json:"send_days,omitempty"`
	SendHour    *int       `json:"send_hour,omitempty"`
	SendMinute  *int       `json:"send_minute,omitempty"`
	Timezone    *string    `json:"timezone,omitempty"`
	ActivityID  *string    `json:"activity_id,omitempty"`
	ExecutionAt *time.Time `json:"execution_at,omitempty"`
}

// IuranReminderRecipient represents a warga + bill that should receive the
// iuran payment reminder for a single execution date.
type IuranReminderRecipient struct {
	BillID       string `json:"bill_id"`
	UserID       string `json:"user_id"`
	UserFullName string `json:"user_full_name,omitempty"`
	RTID         string `json:"rt_id"`
	Period       string `json:"period"`
	Amount       string `json:"amount"`
	DueDate      string `json:"due_date"`
}

// NotificationSchedulerHistory represents an aggregated view of one scheduler
// execution (one schedule + one execution date), used by pengurus/Super Admin
// in the Riwayat Notifikasi tab. One row = one execution, not one row per
// recipient.
type NotificationSchedulerHistory struct {
	ScheduleID      string     `json:"schedule_id"`
	ScheduleName    string     `json:"schedule_name"`
	EventType       string     `json:"event_type"`
	ExecDate        string     `json:"exec_date"` // YYYY-MM-DD in schedule's timezone
	ConfigHash      string     `json:"config_hash,omitempty"`
	TotalRecipients int        `json:"total_recipients"`
	SentCount       int        `json:"sent_count"`
	FailedCount     int        `json:"failed_count"`
	ExecutionTime   *time.Time `json:"execution_time,omitempty"`
}

// NotificationSchedulerRecipientDetail represents an individual recipient's
// delivery result within a single scheduler execution.
type NotificationSchedulerRecipientDetail struct {
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"` // SENT or FAILED
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// ComputeScheduleConfigHash computes a deterministic, canonical SHA-256 hash
// representing the delivery configuration of a notification schedule.
//
// Included delivery configuration fields:
//   - rt_id
//   - event_type
//   - template_id
//   - send_days (canonical: sorted ascending, duplicates removed)
//   - send_hour
//   - send_minute
//   - timezone (normalized)
//
// Explicitly excluded (metadata / non-delivery fields):
//   - name
//   - description
//   - is_active
//   - created_at, updated_at, created_by, updated_by
//   - last_executed_*
func ComputeScheduleConfigHash(
	rtID, eventType, templateID string,
	sendDays []int,
	sendHour, sendMinute int,
	timezone string,
	extra ...string,
) string {
	rt := strings.TrimSpace(strings.ToLower(rtID))
	ev := strings.TrimSpace(eventType)
	tmpl := strings.TrimSpace(strings.ToLower(templateID))

	uniqueDays := make([]int, 0, len(sendDays))
	seen := make(map[int]bool)
	for _, d := range sendDays {
		if !seen[d] {
			seen[d] = true
			uniqueDays = append(uniqueDays, d)
		}
	}
	sort.Ints(uniqueDays)

	tz := strings.TrimSpace(timezone)
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		tz = loc.String()
	}

	daysStr := make([]string, len(uniqueDays))
	for i, d := range uniqueDays {
		daysStr[i] = strconv.Itoa(d)
	}

	canonical := fmt.Sprintf(
		"rt_id=%s|event_type=%s|template_id=%s|send_days=[%s]|send_hour=%d|send_minute=%d|timezone=%s",
		rt, ev, tmpl, strings.Join(daysStr, ","), sendHour, sendMinute, tz,
	)
	for _, e := range extra {
		if strings.TrimSpace(e) != "" {
			canonical += "|activity=" + strings.TrimSpace(e)
		}
	}

	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// ComputeScheduleConfigHashFromSchedule computes the hash directly from a NotificationSchedule.
func ComputeScheduleConfigHashFromSchedule(s *NotificationSchedule) string {
	if s == nil {
		return ""
	}
	extra := []string{}
	if s.ActivityID != nil && strings.TrimSpace(*s.ActivityID) != "" {
		extra = append(extra, "activity_id:"+strings.TrimSpace(*s.ActivityID))
	}
	if s.ExecutionAt != nil {
		extra = append(extra, "execution_at:"+s.ExecutionAt.UTC().Format(time.RFC3339))
	}
	return ComputeScheduleConfigHash(
		s.RTID, s.EventType, s.TemplateID,
		s.SendDays, s.SendHour, s.SendMinute, s.Timezone,
		extra...,
	)
}

// ScheduleExecutionRecord represents a persistent record in notification_schedule_executions.
type ScheduleExecutionRecord struct {
	ID          string                    `json:"id"`
	ScheduleID  string                    `json:"schedule_id"`
	RTID        string                    `json:"rt_id"`
	ExecDate    string                    `json:"exec_date"`
	ConfigHash  string                    `json:"config_hash"`
	Status      string                    `json:"status"`
	ErrorDetail *string                   `json:"error_detail,omitempty"`
	Summary     *ScheduleExecutionSummary `json:"summary,omitempty"`
	ExecutedAt  time.Time                 `json:"executed_at"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
}
