package notification

import "time"

// Event types supported by the notification system.
const (
	EventBillCreated       = "iuran_bill_created"
	EventPaymentSubmitted  = "iuran_payment_submitted"
	EventPaymentVerified   = "iuran_payment_verified"
	EventWasteTruckArrival = "waste_truck_arrival"
	EventManualPush        = "manual_push"
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
	Message  string  `json:"message,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
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
	RTID      string    `json:"rt_id"`
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
	ID        string    `json:"id"`
	RTID      string    `json:"rt_id"`
	UserID    *string   `json:"user_id,omitempty"`
	EventType string    `json:"event_type"`
	DedupKey  string    `json:"dedup_key"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	ImageURL  *string   `json:"image_url,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
