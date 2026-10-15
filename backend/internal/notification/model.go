package notification

import "time"

// Event types supported by the notification system.
const (
	EventBillCreated        = "iuran_bill_created"
	EventPaymentSubmitted   = "iuran_payment_submitted"
	EventPaymentVerified    = "iuran_payment_verified"
	EventWasteTruckArrival  = "waste_truck_arrival"
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
	Message string `json:"message,omitempty"`
}

// NotificationPayload encapsulates minimal identifiers sent via FCM data payload.
// NEVER includes amounts, NIK, or confidential officer notes.
type NotificationPayload struct {
	Type      string `json:"type"`
	RTID      string `json:"rt_id"`
	BillID    string `json:"bill_id,omitempty"`
	PaymentID string `json:"payment_id,omitempty"`
	Status    string `json:"status,omitempty"`
	DeepLink  string `json:"deep_link,omitempty"`
}

// ToMap converts NotificationPayload to string map for FCM Data payload.
func (p NotificationPayload) ToMap() map[string]string {
	m := map[string]string{
		"type":  p.Type,
		"rt_id": p.RTID,
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
	return m
}

