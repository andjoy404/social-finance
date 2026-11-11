package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/finance"
)

// Service defines notification operations.
type Service interface {
	RegisterDevice(ctx context.Context, userID string, in RegisterDeviceInput) error
	UnregisterDevice(ctx context.Context, userID, token string) error
	NotifyBillCreated(ctx context.Context, rtID, billID string, recipientUserIDs []string)
	NotifyPaymentSubmitted(ctx context.Context, rtID, billID, paymentID string)
	NotifyPaymentVerified(ctx context.Context, rtID, billID, paymentID, submitterUserID, status, rejectionReason string)
	NotifyWasteTruckArrival(ctx context.Context, rtID, senderUserID, message string, imageURL *string, idempotencyKey ...string)
	ListWasteTruckHistory(ctx context.Context, ac *auth.AuthContext, rtID string, page, pageSize int) ([]WasteTruckDispatchEntry, int, error)
	SendManualPush(ctx context.Context, rtID, senderUserID string, in ManualPushInput) error
	SendTestPush(ctx context.Context, rtID, superAdminUserID string, in TestPushInput) (int, error)
	SendSelfTest(ctx context.Context, userID string) (int, error)

	// Notification Management
	GetSystemTypes() []SystemNotificationType
	ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]NotificationTemplate, error)
	GetTemplate(ctx context.Context, rtID, id string) (*NotificationTemplate, error)
	CreateTemplate(ctx context.Context, rtID string, in CreateTemplateInput) (*NotificationTemplate, error)
	UpdateTemplate(ctx context.Context, rtID, id string, in UpdateTemplateInput) (*NotificationTemplate, error)
	ToggleTemplateActive(ctx context.Context, rtID, id string) (*NotificationTemplate, error)
	DeleteTemplate(ctx context.Context, rtID, id string) error
	ListLogs(ctx context.Context, ac *auth.AuthContext, rtID, eventType, status string, page, pageSize int) ([]NotificationLogEntry, int, error)
	ListManagementHistory(ctx context.Context, ac *auth.AuthContext, rtID, category string, page, pageSize int) ([]NotificationLogEntry, int, error)
	GetNotificationDetail(ctx context.Context, ac *auth.AuthContext, rtID, id string, canManage bool) (*NotificationLogEntry, error)
	MarkAsRead(ctx context.Context, ac *auth.AuthContext, rtID, id string) error
	MarkAllAsRead(ctx context.Context, ac *auth.AuthContext, rtID string) (int, error)
	GetUnreadCount(ctx context.Context, ac *auth.AuthContext, rtID string) (int, error)
	ResolveTenantRT(ctx context.Context, ac *auth.AuthContext, reqHeaderRT, reqQueryRT string) (string, error)

	// Schedule management
	ListSchedules(ctx context.Context, rtID, eventType string, isActive *bool) ([]*NotificationSchedule, error)
	GetSchedule(ctx context.Context, rtID, id string) (*NotificationSchedule, error)
	CreateSchedule(ctx context.Context, rtID, userID string, in CreateScheduleInput) (*NotificationSchedule, error)
	UpdateSchedule(ctx context.Context, rtID, id, userID string, in UpdateScheduleInput) (*NotificationSchedule, error)
	DeleteSchedule(ctx context.Context, rtID, id string) error
	ToggleScheduleActive(ctx context.Context, rtID, id, userID string) (*NotificationSchedule, error)
	// GetDueSchedules returns active schedules due for execution at the given time.
	GetDueSchedules(ctx context.Context, now time.Time) ([]*NotificationSchedule, error)
	// ExecuteSchedule runs a single schedule: resolves recipients, sends pushes,
	// and records per-recipient delivery logs with idempotency.
	ExecuteSchedule(ctx context.Context, schedule *NotificationSchedule, now time.Time) (int, error)

	// Scheduler history (aggregated per execution, for pengurus/Super Admin)
	ListSchedulerHistory(ctx context.Context, rtID string, page, pageSize int) ([]*NotificationSchedulerHistory, int, error)
	GetSchedulerRecipientDetail(ctx context.Context, rtID, scheduleID, execDate string, configHash ...string) ([]*NotificationSchedulerRecipientDetail, error)

	// RW notifications
	NotifyRWCashIn(ctx context.Context, rwID, txID, amount, description, locationName string)
	NotifyRWCashOut(ctx context.Context, rwID, txID, amount, description, category, locationName string)
	SendRWManualPush(ctx context.Context, rwID, senderUserID string, title, body string, target ...string) (int, error)
	ListRWLogs(ctx context.Context, rwID, eventType string, page, pageSize int) ([]NotificationLogEntry, int, error)

	// RW notification templates
	ListRWTemplates(ctx context.Context, rwID, eventType string, isActive *bool) ([]NotificationTemplate, error)
	GetRWTemplateByID(ctx context.Context, rwID, id string) (*NotificationTemplate, error)
	CreateRWTemplate(ctx context.Context, rwID string, in CreateTemplateInput) (*NotificationTemplate, error)
	UpdateRWTemplate(ctx context.Context, rwID, id string, in UpdateTemplateInput) (*NotificationTemplate, error)
	ToggleRWTemplateActive(ctx context.Context, rwID, id string) (*NotificationTemplate, error)
	DeleteRWTemplate(ctx context.Context, rwID, id string) error
}

var ErrForbidden = errors.New("forbidden: akses detail notifikasi ditolak")

type service struct {
	repo     Repository
	provider PushProvider
}

// NewService creates a new notification service.
func NewService(repo Repository, provider PushProvider) Service {
	if provider == nil {
		provider = NewMockPushProvider()
	}
	return &service{
		repo:     repo,
		provider: provider,
	}
}

func (s *service) RegisterDevice(ctx context.Context, userID string, in RegisterDeviceInput) error {
	token := strings.TrimSpace(in.FCMToken)
	if token == "" {
		return fmt.Errorf("fcm_token is required")
	}
	platform := strings.ToLower(strings.TrimSpace(in.Platform))
	if platform != PlatformAndroid && platform != PlatformIOS && platform != PlatformWeb {
		return fmt.Errorf("invalid platform: must be android, ios, or web")
	}
	return s.repo.RegisterDevice(ctx, userID, token, platform, in.DeviceID)
}

func (s *service) UnregisterDevice(ctx context.Context, userID, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("fcm_token is required")
	}
	return s.repo.UnregisterDevice(ctx, userID, token)
}

// NotifyBillCreated sends notification to residents associated with a newly created bill.
// Executed asynchronously and decoupled from database transactions.
func (s *service) NotifyBillCreated(ctx context.Context, rtID, billID string, recipientUserIDs []string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var targets []string
		if len(recipientUserIDs) > 0 {
			targets = dedupeUserIDs(recipientUserIDs)
		} else {
			var err error
			targets, err = s.repo.GetRecipientUserIDsForBill(bgCtx, rtID, billID)
			if err != nil {
				slog.Error("failed to find recipients for bill notification", "bill_id", billID, "error", err)
				return
			}
			targets = dedupeUserIDs(targets)
		}

		title := "Tagihan Iuran Baru"
		body := "Tagihan iuran baru telah tersedia. Silakan periksa rincian tagihan Anda."
		if tmpl, err := s.repo.GetActiveTemplateForEvent(bgCtx, rtID, EventBillCreated); err == nil && tmpl != nil && tmpl.IsActive {
			vars := map[string]string{
				"bill_id": billID,
			}
			title = interpolate(tmpl.Title, vars)
			body = interpolate(tmpl.Body, vars)
		}

		payload := NotificationPayload{
			Type:     EventBillCreated,
			RTID:     rtID,
			Title:    title,
			Body:     body,
			BillID:   billID,
			DeepLink: fmt.Sprintf("/home/iuran/detail/%s", billID),
			SentAt:   time.Now().UTC().Format(time.RFC3339Nano),
		}

		for _, uid := range targets {
			dedupKey := fmt.Sprintf("bill:%s:%s", billID, uid)
			payloadMap := payload.ToMap()
			ok, err := s.repo.CheckAndRecordDedup(bgCtx, rtID, uid, EventBillCreated, dedupKey, title, body, payloadMap)
			if err != nil {
				slog.Warn("notification dedup check failed", "user_id", uid, "error", err)
			}
			if !ok {
				continue // deduplicated
			}

			tokens, err := s.repo.GetTokensForUser(bgCtx, uid)
			if err != nil {
				slog.Error("failed to get tokens for bill recipient", "user_id", uid, "error", err)
				continue
			}

			s.sendPushToTokens(bgCtx, tokens, title, body, payloadMap)
		}
	}()
}

// NotifyPaymentSubmitted notifies every active user in the affected RT who is
// authorized to verify payments (POST /api/v1/payments/{id}/verify), resolved
// from the actual authorization policy: members whose jabatan grants
// finance.approve via position_permissions, plus members without jabatan
// holding the bendahara/pengurus tenant role. The payment submitter is never
// excluded here — a management officer paying their own household's bill must
// still be notified as an authorized verifier (self-approval remains blocked
// at verification time by the finance service guard).
func (s *service) NotifyPaymentSubmitted(ctx context.Context, rtID, billID, paymentID string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		title := "Pembayaran Iuran Masuk"
		body := "Ada pembayaran iuran baru yang menunggu verifikasi."
		if tmpl, err := s.repo.GetActiveTemplateForEvent(bgCtx, rtID, EventPaymentSubmitted); err == nil && tmpl != nil && tmpl.IsActive {
			vars := map[string]string{
				"bill_id":    billID,
				"payment_id": paymentID,
			}
			title = interpolate(tmpl.Title, vars)
			body = interpolate(tmpl.Body, vars)
		}

		payload := NotificationPayload{
			Type:      EventPaymentSubmitted,
			RTID:      rtID,
			Title:     title,
			Body:      body,
			BillID:    billID,
			PaymentID: paymentID,
			Status:    "PENDING",
			DeepLink:  fmt.Sprintf("/home/iuran/detail/%s?payment_id=%s", billID, paymentID),
			SentAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}

		dedupKey := fmt.Sprintf("payment_submitted:%s", paymentID)

		// Verifier tokens are already DISTINCT per token, so a member matching
		// multiple recipient conditions (e.g. role=pengurus + jabatan=bendahara)
		// never receives duplicates.
		tokens, err := s.repo.GetTokensForPaymentVerifiers(bgCtx, rtID)
		if err != nil {
			slog.Error("failed to get payment verifier tokens for payment submission", "rt_id", rtID, "error", err)
			return
		}

		// Also check deduplication for the RT event log
		payloadMap := payload.ToMap()
		ok, err := s.repo.CheckAndRecordDedup(bgCtx, rtID, "", EventPaymentSubmitted, dedupKey, title, body, payloadMap)
		if err != nil {
			slog.Warn("payment submitted dedup check failed", "payment_id", paymentID, "error", err)
		}
		if !ok {
			return // duplicate event
		}

		// Record personal logs for each authorized payment verifier so they
		// see the submission notification in their personal Riwayat Notifikasi.
		if verifierUserIDs, err := s.repo.GetPaymentVerifierUserIDs(bgCtx, rtID); err == nil {
			for _, vid := range verifierUserIDs {
				verifierDedupKey := fmt.Sprintf("payment_submitted:%s:%s", paymentID, vid)
				_, _ = s.repo.CheckAndRecordDedup(bgCtx, rtID, vid, EventPaymentSubmitted, verifierDedupKey, title, body, payloadMap)
			}
		} else {
			slog.Error("failed to get payment verifier user IDs for personal delivery logs", "rt_id", rtID, "error", err)
		}

		s.sendPushToTokens(bgCtx, tokens, title, body, payloadMap)
	}()
}

// NotifyPaymentVerified notifies the warga who owns/submitted the payment.
// Verifiers (and Bendahara) must NOT receive this notification.
func (s *service) NotifyPaymentVerified(ctx context.Context, rtID, billID, paymentID, submitterUserID, status, rejectionReason string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var title, body string
		if status == "APPROVED" {
			title = "Pembayaran Iuran Diverifikasi"
			body = "Pembayaran iuran Anda telah disetujui dan dicatat dalam pembukuan."
		} else {
			title = "Pembayaran Iuran Ditolak"
			if strings.TrimSpace(rejectionReason) != "" {
				body = fmt.Sprintf("Pembayaran iuran Anda ditolak: %s", rejectionReason)
			} else {
				body = "Pembayaran iuran Anda ditolak oleh bendahara. Silakan periksa kembali bukti transfer Anda."
			}
		}

		if tmpl, err := s.repo.GetActiveTemplateForEvent(bgCtx, rtID, EventPaymentVerified); err == nil && tmpl != nil && tmpl.IsActive {
			vars := map[string]string{
				"bill_id":          billID,
				"payment_id":       paymentID,
				"status":           status,
				"rejection_reason": rejectionReason,
			}
			title = interpolate(tmpl.Title, vars)
			body = interpolate(tmpl.Body, vars)
		}

		payload := NotificationPayload{
			Type:      EventPaymentVerified,
			RTID:      rtID,
			BillID:    billID,
			PaymentID: paymentID,
			Status:    status,
			DeepLink:  fmt.Sprintf("/home/iuran/detail/%s?payment_id=%s", billID, paymentID),
			SentAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}

		var targets []string
		if submitterUserID != "" {
			targets = []string{submitterUserID}
		} else {
			var err error
			targets, err = s.repo.GetRecipientUserIDsForBill(bgCtx, rtID, billID)
			if err != nil {
				slog.Error("failed to get recipient user IDs for bill on payment verified", "bill_id", billID, "error", err)
			}
			targets = dedupeUserIDs(targets)
		}

		for _, targetUID := range targets {
			submitterDedupKey := fmt.Sprintf("payment_verified:%s:%s:%s", paymentID, status, targetUID)
			okSubmitter, dedupErr := s.repo.CheckAndRecordDedup(bgCtx, rtID, targetUID, EventPaymentVerified, submitterDedupKey, title, body, payload.ToMap())
			if dedupErr != nil {
				slog.Warn("payment verified dedup check failed for submitter", "payment_id", paymentID, "user_id", targetUID, "error", dedupErr)
			}
			if !okSubmitter || dedupErr != nil {
				continue
			}

			submitterTokens, err := s.repo.GetTokensForUser(bgCtx, targetUID)
			if err != nil {
				slog.Error("failed to get tokens for payment submitter", "user_id", targetUID, "error", err)
				continue
			}
			if len(submitterTokens) > 0 {
				s.sendPushToTokens(bgCtx, submitterTokens, title, body, payload.ToMap())
			}
		}
	}()
}

// NotifyWasteTruckArrival dispatches manual notification to all warga in the sender's RT.
func (s *service) NotifyWasteTruckArrival(ctx context.Context, rtID, senderUserID, customMessage string, imageURL *string, idempotencyKey ...string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		title := "Mobil Pengangkut Sampah Sudah Tiba"
		body := "Mobil pengangkut sampah sudah sampai di depan gerbang RT. Silakan keluarkan sampah Anda."
		var tmplImg *string
		if tmpl, err := s.repo.GetActiveTemplateForEvent(bgCtx, rtID, EventWasteTruckArrival); err == nil && tmpl != nil && tmpl.IsActive {
			title = tmpl.Title
			body = tmpl.Body
			tmplImg = tmpl.ImageURL
		}
		if strings.TrimSpace(customMessage) != "" {
			body = strings.TrimSpace(customMessage)
		}

		var finalImg *string
		if imageURL != nil && strings.TrimSpace(*imageURL) != "" {
			trimmed := strings.TrimSpace(*imageURL)
			finalImg = &trimmed
		} else if tmplImg != nil && strings.TrimSpace(*tmplImg) != "" {
			finalImg = tmplImg
		}

		payload := NotificationPayload{
			Type:     EventWasteTruckArrival,
			RTID:     rtID,
			Title:    title,
			Body:     body,
			DeepLink: "/home/notifikasi/detail",
			ImageURL: finalImg,
			SentAt:   time.Now().UTC().Format(time.RFC3339Nano),
		}

		var idemKey string
		if len(idempotencyKey) > 0 && strings.TrimSpace(idempotencyKey[0]) != "" {
			idemKey = strings.TrimSpace(idempotencyKey[0])
		}

		var dedupKey string
		if idemKey != "" {
			dedupKey = fmt.Sprintf("waste_truck:%s:idemp:%s", rtID, idemKey)
		} else {
			dedupKey = fmt.Sprintf("waste_truck:%s:%d:%s", rtID, time.Now().UnixNano(), senderUserID)
		}

		payloadMap := payload.ToMap()
		notifID, ok, err := s.repo.RecordWasteTruckDispatch(bgCtx, rtID, senderUserID, dedupKey, title, body, payloadMap)
		if err != nil {
			slog.Warn("waste truck arrival dedup check failed", "error", err)
		}
		if !ok {
			slog.Info("waste truck notification skipped due to idempotency dedup", "rt_id", rtID, "dedup_key", dedupKey)
			return
		}
		if notifID == "" {
			slog.Error("waste truck dispatch log was created without an identifier", "rt_id", rtID)
			return
		}
		updateDispatchStatus := func(status, reason string) {
			updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer updateCancel()
			if updateErr := s.repo.UpdateWasteTruckDispatch(updateCtx, rtID, senderUserID, notifID, status, reason); updateErr != nil {
				slog.Error("failed to record waste truck dispatch result", "rt_id", rtID, "error", updateErr)
			}
		}
		if notifID := payloadMap["notification_id"]; notifID != "" {
			payloadMap["deep_link"] = "/home/notifikasi/detail/" + notifID
		}

		recipients, err := s.repo.GetWasteTruckArrivalRecipients(bgCtx, rtID)
		if err != nil {
			slog.Error("failed to get RT recipients for waste truck arrival", "rt_id", rtID, "error", err)
			updateDispatchStatus("UNKNOWN", "Recipient delivery results could not be loaded")
			return
		}

		recipientStatuses := make([]string, 0, len(recipients))
		for _, recipient := range recipients {
			recipientSent, recipientFailed := s.sendPushToTokens(
				bgCtx, recipient.Tokens, title, body, payloadMap,
			)
			status := recipientDeliveryStatus(recipientSent, recipientFailed, len(recipient.Tokens))
			recipientStatuses = append(recipientStatuses, status)
			if err := s.repo.RecordBroadcastRecipientDelivery(
				bgCtx, rtID, senderUserID, recipient.UserID, EventWasteTruckArrival, notifID,
				title, body, payloadMap, status,
			); err != nil {
				slog.Warn("failed to record waste truck recipient history",
					"rt_id", rtID, "error", err)
			}
		}
		status := aggregateDeliveryStatus(recipientStatuses)
		failureReason := ""
		switch status {
		case "PARTIAL":
			failureReason = "Some recipient deliveries failed or could not be confirmed"
		case "FAILED":
			failureReason = "Provider reported failure for all recipient deliveries"
		case "UNKNOWN":
			failureReason = "Recipient delivery results could not be confirmed"
		}
		updateDispatchStatus(status, failureReason)
	}()
}

func (s *service) ListWasteTruckHistory(
	ctx context.Context,
	ac *auth.AuthContext,
	rtID string,
	page, pageSize int,
) ([]WasteTruckDispatchEntry, int, error) {
	if ac == nil || ac.UserID == "" ||
		!strings.EqualFold(strings.TrimSpace(string(ac.Jabatan)), string(auth.JabatanKeamanan)) ||
		ac.RTID == "" || ac.RTID != rtID {
		return nil, 0, ErrForbidden
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	return s.repo.ListWasteTruckDispatches(ctx, rtID, ac.UserID, offset, pageSize)
}

// SendManualPush dispatches manual broadcast notification to all active residents in the RT.
func (s *service) SendManualPush(ctx context.Context, rtID, senderUserID string, in ManualPushInput) error {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	var imgURL *string
	if in.ImageURL != nil && strings.TrimSpace(*in.ImageURL) != "" {
		trimmed := strings.TrimSpace(*in.ImageURL)
		imgURL = &trimmed
	}

	if title == "" || body == "" {
		if tmpl, err := s.repo.GetActiveTemplateForEvent(ctx, rtID, EventManualPush); err == nil && tmpl != nil && tmpl.IsActive {
			if title == "" {
				title = tmpl.Title
			}
			if body == "" {
				body = tmpl.Body
			}
			if imgURL == nil && tmpl.ImageURL != nil && strings.TrimSpace(*tmpl.ImageURL) != "" {
				imgURL = tmpl.ImageURL
			}
		}
	}

	if title == "" {
		return errors.New("judul notifikasi wajib diisi")
	}
	if body == "" {
		return errors.New("isi pesan notifikasi wajib diisi")
	}

	deepLink := strings.TrimSpace(in.DeepLink)
	if deepLink == "" || deepLink == "/notifications" {
		deepLink = "/home/notifikasi/detail"
	}

	payload := NotificationPayload{
		Type:     EventManualPush,
		RTID:     rtID,
		Title:    title,
		Body:     body,
		DeepLink: deepLink,
		ImageURL: imgURL,
		SentAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	dedupKey := fmt.Sprintf("manual_push:%s:%d:%s", rtID, time.Now().UnixNano(), title)
	payloadMap := payload.ToMap()
	ok, err := s.repo.CheckAndRecordDedup(ctx, rtID, senderUserID, EventManualPush, dedupKey, title, body, payloadMap)
	if err != nil {
		slog.Warn("manual push dedup recording failed", "error", err)
	}
	if !ok {
		return errors.New("notifikasi duplikat ditolak")
	}
	if notifID := payloadMap["notification_id"]; notifID != "" && payloadMap["deep_link"] == "/home/notifikasi/detail" {
		payloadMap["deep_link"] = "/home/notifikasi/detail/" + notifID
	}

	recipients, err := s.repo.GetRTBroadcastRecipients(ctx, rtID)
	if err != nil {
		return fmt.Errorf("gagal mendapatkan target perangkat RT: %w", err)
	}

	broadcastID := payloadMap["notification_id"]
	for _, recipient := range recipients {
		sent, failed := s.sendPushToTokens(ctx, recipient.Tokens, title, body, payloadMap)
		status := recipientDeliveryStatus(sent, failed, len(recipient.Tokens))
		if err := s.repo.RecordBroadcastRecipientDelivery(
			ctx, rtID, senderUserID, recipient.UserID, EventManualPush,
			broadcastID, title, body, payloadMap, status,
		); err != nil {
			slog.Warn("failed to record manual push recipient history",
				"rt_id", rtID, "error", err)
		}
	}
	return nil
}

// SendTestPush sends an isolated test notification strictly to the authenticated superadmin's device tokens.
func (s *service) SendTestPush(ctx context.Context, rtID, superAdminUserID string, in TestPushInput) (int, error) {
	if in.EventType != EventWasteTruckArrival && in.EventType != EventManualPush {
		return 0, fmt.Errorf("event '%s' tidak mendukung pengujian notifikasi manual", in.EventType)
	}

	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	var imgURL *string
	if in.ImageURL != nil && strings.TrimSpace(*in.ImageURL) != "" {
		trimmed := strings.TrimSpace(*in.ImageURL)
		imgURL = &trimmed
	}

	if title == "" || body == "" {
		if tmpl, err := s.repo.GetActiveTemplateForEvent(ctx, rtID, in.EventType); err == nil && tmpl != nil && tmpl.IsActive {
			if title == "" {
				title = tmpl.Title
			}
			if body == "" {
				body = tmpl.Body
			}
			if imgURL == nil && tmpl.ImageURL != nil && strings.TrimSpace(*tmpl.ImageURL) != "" {
				imgURL = tmpl.ImageURL
			}
		}
	}

	if title == "" {
		title = fmt.Sprintf("[TEST] Notifikasi %s", in.EventType)
	}
	if body == "" {
		body = "Ini adalah pengujian notifikasi untuk perangkat Super Admin."
	}

	payload := NotificationPayload{
		Type:     in.EventType,
		RTID:     rtID,
		Title:    title,
		Body:     body,
		DeepLink: "/home/notifikasi/detail",
		ImageURL: imgURL,
		SentAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Strictly get tokens ONLY for the Super Admin
	tokens, err := s.repo.GetTokensForUser(ctx, superAdminUserID)
	if err != nil {
		return 0, fmt.Errorf("gagal mendapatkan token perangkat admin: %w", err)
	}
	if len(tokens) == 0 {
		return 0, nil
	}

	// Dedup key specific to test, does not trigger production cooldown side-effects
	dedupKey := fmt.Sprintf("test_push:%s:%s:%d", rtID, superAdminUserID, time.Now().UnixNano())
	payloadMap := payload.ToMap()
	_, _ = s.repo.CheckAndRecordDedup(ctx, rtID, superAdminUserID, in.EventType, dedupKey, title, body, payloadMap)
	if notifID := payloadMap["notification_id"]; notifID != "" {
		payloadMap["deep_link"] = "/home/notifikasi/detail/" + notifID
	}

	s.sendPushToTokens(ctx, tokens, title, body, payloadMap)
	return len(tokens), nil
}

// SendSelfTest dispatches an isolated test notification exclusively to the specified user's active devices.
// Used for development connectivity verification without mutating bills, payments, or ledger state.
func (s *service) SendSelfTest(ctx context.Context, userID string) (int, error) {
	tokens, err := s.repo.GetTokensForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get user devices: %w", err)
	}
	if len(tokens) == 0 {
		return 0, nil
	}

	title := "Uji Notifikasi SocialFinance"
	body := "Ini adalah pesan uji konektivitas notifikasi FCM SocialFinance."
	payload := map[string]string{
		"type":      "test_self",
		"title":     title,
		"body":      body,
		"deep_link": "/home/notifikasi/detail",
		"sent_at":   time.Now().UTC().Format(time.RFC3339Nano),
	}

	s.sendPushToTokens(ctx, tokens, title, body, payload)
	return len(tokens), nil
}

// dedupeUserIDs trims, drops empty entries, and removes duplicates while
// preserving order, ensuring one person never receives the same notification
// twice when they match multiple recipient conditions.
func dedupeUserIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *service) sendPushToTokens(ctx context.Context, tokens []string, title, body string, payload map[string]string) (sent, failed int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, tok := range tokens {
		tok := tok
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.provider.Send(ctx, tok, title, body, payload); err != nil {
				mu.Lock()
				failed++
				slog.Warn("push send failed", "token", maskToken(tok), "error", err)
				if isInvalidTokenError(err) {
					if dErr := s.repo.DeactivateToken(ctx, tok); dErr != nil {
						slog.Warn("failed to deactivate invalid token", "token", maskToken(tok), "error", dErr)
					} else {
						slog.Info("deactivated invalid/unregistered token", "token", maskToken(tok))
					}
				}
				mu.Unlock()
			} else {
				mu.Lock()
				sent++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return sent, failed
}

func deliveryStatus(sent, failed int) string {
	switch {
	case sent > 0 && failed > 0:
		return "PARTIAL"
	case sent > 0:
		return "SENT"
	case failed > 0:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// recipientDeliveryStatus distinguishes known lack of a delivery endpoint from
// provider failure or an unavailable result. SENT means accepted by FCM, not
// confirmed displayed on a device.
func recipientDeliveryStatus(sent, failed, activeTokenCount int) string {
	if activeTokenCount == 0 {
		return "NO_DEVICE"
	}
	return deliveryStatus(sent, failed)
}

func deliveryFailureReason(status string) string {
	switch status {
	case "FAILED":
		return "Provider reported failure for all recipient devices"
	case "PARTIAL":
		return "Provider reported failure for one or more recipient devices"
	default:
		return ""
	}
}

func aggregateDeliveryStatus(statuses []string) string {
	sent, failed, noDevice, unknown := false, false, false, false
	for _, status := range statuses {
		switch status {
		case "SENT":
			sent = true
		case "FAILED":
			failed = true
		case "PARTIAL":
			sent, failed = true, true
		case "NO_DEVICE":
			noDevice = true
		default:
			unknown = true
		}
	}
	if sent && failed {
		return "PARTIAL"
	}
	if sent {
		return "SENT"
	}
	if failed {
		return "FAILED"
	}
	if unknown {
		return "UNKNOWN"
	}
	if noDevice {
		return "NO_DEVICE"
	}
	return "UNKNOWN"
}

func isInvalidTokenError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTokenInvalid) || errors.Is(err, ErrTokenUnregistered) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unregistered") ||
		strings.Contains(lower, "invalid device token") ||
		strings.Contains(lower, "invalid registration token")
}

func interpolate(text string, vars map[string]string) string {
	res := text
	for k, v := range vars {
		res = strings.ReplaceAll(res, "{{"+k+"}}", v)
	}
	return res
}

// SystemNotificationTypes lists the standard recognized notification events.
var SystemNotificationTypes = []SystemNotificationType{
	{
		EventType:        EventBillCreated,
		Name:             "Tagihan Iuran Baru",
		Trigger:          "Penerbitan tagihan iuran baru bulanan/tahunan",
		Channel:          "push",
		RecipientsRule:   "Warga yang menempati rumah tangga terkait tagihan",
		DefaultTitle:     "Tagihan Iuran Baru",
		DefaultBody:      "Tagihan iuran baru telah tersedia. Silakan periksa rincian tagihan Anda.",
		AllowedVariables: []string{"bill_id"},
		IsConfigurable:   true,
		SupportsImage:    false,
		SupportsTest:     false,
	},
	{
		EventType:        EventPaymentSubmitted,
		Name:             "Pembayaran Iuran Masuk",
		Trigger:          "Warga mengunggah bukti pembayaran iuran baru",
		Channel:          "push",
		RecipientsRule:   "Pengurus / Bendahara berwenang verifikasi pembayaran",
		DefaultTitle:     "Pembayaran Iuran Masuk",
		DefaultBody:      "Ada pembayaran iuran baru yang menunggu verifikasi.",
		AllowedVariables: []string{"bill_id", "payment_id"},
		IsConfigurable:   true,
		SupportsImage:    false,
		SupportsTest:     false,
	},
	{
		EventType:        EventPaymentVerified,
		Name:             "Verifikasi Pembayaran Iuran",
		Trigger:          "Verifikasi persetujuan atau penolakan pembayaran oleh Bendahara",
		Channel:          "push",
		RecipientsRule:   "Warga pengunggah pembayaran dan verifikator RT",
		DefaultTitle:     "Pembayaran Iuran Diverifikasi",
		DefaultBody:      "Pembayaran iuran Anda telah disetujui dan dicatat dalam pembukuan.",
		AllowedVariables: []string{"bill_id", "payment_id", "status", "rejection_reason"},
		IsConfigurable:   true,
		SupportsImage:    false,
		SupportsTest:     false,
	},
	{
		EventType:        EventWasteTruckArrival,
		Name:             "Kedatangan Mobil Pengangkut Sampah",
		Trigger:          "Pemicu manual oleh Pengurus atau Petugas Keamanan RT",
		Channel:          "push",
		RecipientsRule:   "Seluruh warga terdaftar di RT pengirim",
		DefaultTitle:     "Mobil Pengangkut Sampah Sudah Tiba",
		DefaultBody:      "Mobil pengangkut sampah sudah sampai di depan gerbang RT. Silakan keluarkan sampah Anda.",
		AllowedVariables: []string{},
		IsConfigurable:   true,
		SupportsImage:    true,
		SupportsTest:     true,
	},
	{
		EventType:        EventManualPush,
		Name:             "Pengumuman Warga Manual",
		Trigger:          "Pengiriman pesan siaran manual oleh Pengurus RT / Superadmin",
		Channel:          "push",
		RecipientsRule:   "Seluruh warga terdaftar di RT pengirim",
		DefaultTitle:     "Pengumuman Pengurus RT",
		DefaultBody:      "Pemberitahuan penting untuk seluruh warga.",
		AllowedVariables: []string{},
		IsConfigurable:   true,
		SupportsImage:    true,
		SupportsTest:     true,
	},
	{
		EventType:        EventPaymentReminder,
		Name:             "Pengingat Pembayaran Iuran",
		Trigger:          "Jadwal otomatis bulanan (tanggal dan jam dapat diatur per RT)",
		Channel:          "push",
		RecipientsRule:   "Warga yang masih memiliki kewajiban pembayaran tagihan iuran yang belum lunas",
		DefaultTitle:     "Pengingat Pembayaran Iuran",
		DefaultBody:      "Anda masih memiliki tagihan iuran yang belum lunas. Segera lakukan pembayaran sebelum jatuh tempo.",
		AllowedVariables: []string{"nama", "bill_id", "period", "amount", "due_date", "deep_link"},
		IsConfigurable:   true,
		SupportsImage:    false,
		SupportsTest:     false,
	},
	{
		EventType:        EventUpcomingDue3Months,
		Name:             "Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)",
		Trigger:          "Otomatis dikirim satu kali saat tagihan iuran wajib memasuki 3 bulan kalender sebelum tanggal jatuh tempo",
		Channel:          "push",
		RecipientsRule:   "Warga penghuni aktif yang memiliki tagihan iuran wajib belum lunas yang memasuki 3 bulan kalender sebelum jatuh tempo",
		DefaultTitle:     "Pengingat Pembayaran Iuran",
		DefaultBody:      "Halo {{nama}}, kami mengingatkan bahwa tagihan iuran Anda akan jatuh tempo dalam waktu 3 bulan. Silakan periksa tagihan Anda dan lakukan pembayaran sesuai ketentuan. Terima kasih.",
		AllowedVariables: []string{"nama", "bill_id", "period", "amount", "total_outstanding", "bill_count", "due_date", "deep_link"},
		IsConfigurable:   true,
		SupportsImage:    false,
		SupportsTest:     false,
	},
	{
		EventType:        EventOverdue3Months,
		Name:             "Pengingat Tunggakan Iuran (3 Bulan+)",
		Trigger:          "Otomatis dikirim saat tagihan iuran wajib menunggak minimal 3 bulan kalender setelah jatuh tempo, dan diulang setiap bulan selama belum lunas",
		Channel:          "push",
		RecipientsRule:   "Warga penghuni aktif yang memiliki tagihan iuran wajib belum lunas dengan tunggakan minimal 3 bulan kalender",
		DefaultTitle:     "Pengingat Tunggakan Iuran",
		DefaultBody:      "Halo {{nama}}, terdapat tagihan iuran Anda yang telah menunggak selama 3 bulan atau lebih. Mohon segera lakukan pembayaran untuk menyelesaikan kewajiban iuran Anda. Terima kasih.",
		AllowedVariables: []string{"nama", "bill_id", "period", "amount", "total_outstanding", "bill_count", "due_date", "deep_link"},
		IsConfigurable:   true,
		SupportsImage:    false,
		SupportsTest:     false,
	},
}

// RWSystemNotificationTypes lists system notification types specific to RW level.
var RWSystemNotificationTypes = []SystemNotificationType{
	{
		EventType:        EventRWCashIn,
		Name:             "Kas Masuk RW",
		Trigger:          "Penerimaan dana kas masuk tingkat RW",
		Channel:          "push",
		RecipientsRule:   "Seluruh pengurus RW aktif",
		DefaultTitle:     "Kas Masuk RW Diterima",
		DefaultBody:      "Penerimaan kas masuk RW sebesar {{amount}} telah dicatat.",
		AllowedVariables: []string{"amount", "description", "location"},
		IsConfigurable:   false,
		SupportsImage:    false,
		SupportsTest:     false,
	},
	{
		EventType:        EventRWCashOut,
		Name:             "Kas Keluar RW",
		Trigger:          "Pengeluaran dana kas keluar tingkat RW",
		Channel:          "push",
		RecipientsRule:   "Seluruh pengurus RW aktif",
		DefaultTitle:     "Kas Keluar RW Dicatat",
		DefaultBody:      "Pengeluaran kas RW sebesar {{amount}} untuk {{description}} telah dicatat.",
		AllowedVariables: []string{"amount", "description", "category", "location"},
		IsConfigurable:   false,
		SupportsImage:    false,
		SupportsTest:     false,
	},
}

// IsFixedSystemEvent reports whether eventType is one of the permanent system events
// that cannot be created as custom templates or scheduled.
func IsFixedSystemEvent(eventType string) bool {
	return eventType == EventBillCreated || eventType == EventPaymentSubmitted || eventType == EventPaymentVerified
}

func (s *service) GetSystemTypes() []SystemNotificationType {
	return SystemNotificationTypes
}

func findSystemType(eventType string) *SystemNotificationType {
	for _, t := range SystemNotificationTypes {
		if t.EventType == eventType {
			return &t
		}
	}
	return nil
}

func extractVariables(text string) []string {
	var vars []string
	seen := make(map[string]struct{})
	start := 0
	for {
		open := strings.Index(text[start:], "{{")
		if open == -1 {
			break
		}
		openIdx := start + open
		close := strings.Index(text[openIdx+2:], "}}")
		if close == -1 {
			break
		}
		closeIdx := openIdx + 2 + close
		varName := strings.TrimSpace(text[openIdx+2 : closeIdx])
		if varName != "" {
			if _, exists := seen[varName]; !exists {
				seen[varName] = struct{}{}
				vars = append(vars, varName)
			}
		}
		start = closeIdx + 2
	}
	return vars
}

func validateTemplateVariables(text string, allowed []string) error {
	allowedMap := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowedMap[a] = true
	}
	used := extractVariables(text)
	for _, u := range used {
		if !allowedMap[u] {
			return fmt.Errorf("variabel {{%s}} tidak diizinkan untuk jenis notifikasi ini", u)
		}
	}
	return nil
}

func (s *service) ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]NotificationTemplate, error) {
	tmpls, err := s.repo.ListTemplates(ctx, rtID, eventType, isActive)
	if err != nil {
		return nil, err
	}
	if eventType == "" {
		filtered := make([]NotificationTemplate, 0, len(tmpls))
		for _, t := range tmpls {
			if !IsFixedSystemEvent(t.EventType) {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}
	return tmpls, nil
}

func (s *service) GetTemplate(ctx context.Context, rtID, id string) (*NotificationTemplate, error) {
	return s.repo.GetTemplateByID(ctx, rtID, id)
}

func (s *service) CreateTemplate(ctx context.Context, rtID string, in CreateTemplateInput) (*NotificationTemplate, error) {
	if IsFixedSystemEvent(in.EventType) {
		return nil, errors.New("event ini merupakan notifikasi sistem tetap dan tidak dapat dibuat sebagai template kustom")
	}
	if in.EventType == EventRWCashIn || in.EventType == EventRWCashOut {
		return nil, errors.New("event kas RW tidak dapat digunakan pada template notifikasi tingkat RT")
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("nama template wajib diisi")
	}
	if strings.TrimSpace(in.EventType) == "" {
		return nil, errors.New("jenis event wajib dipilih")
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("judul notifikasi wajib diisi")
	}
	if strings.TrimSpace(in.Body) == "" {
		return nil, errors.New("isi pesan notifikasi wajib diisi")
	}

	sysType := findSystemType(in.EventType)
	if sysType == nil {
		return nil, fmt.Errorf("jenis event '%s' tidak valid atau tidak didukung", in.EventType)
	}

	if in.ImageURL != nil && strings.TrimSpace(*in.ImageURL) != "" {
		if !sysType.SupportsImage {
			return nil, fmt.Errorf("gambar tidak diizinkan untuk event '%s'", in.EventType)
		}
	}

	if err := validateTemplateVariables(in.Title, sysType.AllowedVariables); err != nil {
		return nil, err
	}
	if err := validateTemplateVariables(in.Body, sysType.AllowedVariables); err != nil {
		return nil, err
	}

	return s.repo.CreateTemplate(ctx, rtID, in, sysType.AllowedVariables)
}

func (s *service) UpdateTemplate(ctx context.Context, rtID, id string, in UpdateTemplateInput) (*NotificationTemplate, error) {
	existing, err := s.repo.GetTemplateByID(ctx, rtID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("template tidak ditemukan")
	}

	sysType := findSystemType(existing.EventType)
	if sysType == nil {
		return nil, errors.New("jenis event tidak valid")
	}

	if in.ImageURL != nil && strings.TrimSpace(*in.ImageURL) != "" {
		if !sysType.SupportsImage {
			return nil, fmt.Errorf("gambar tidak diizinkan untuk event '%s'", existing.EventType)
		}
	}

	if in.Name != "" && strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("nama template tidak boleh kosong")
	}
	if in.Title != "" && strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("judul notifikasi tidak boleh kosong")
	}
	if in.Body != "" && strings.TrimSpace(in.Body) == "" {
		return nil, errors.New("isi pesan notifikasi tidak boleh kosong")
	}

	if in.Title != "" {
		if err := validateTemplateVariables(in.Title, sysType.AllowedVariables); err != nil {
			return nil, err
		}
	}
	if in.Body != "" {
		if err := validateTemplateVariables(in.Body, sysType.AllowedVariables); err != nil {
			return nil, err
		}
	}

	return s.repo.UpdateTemplate(ctx, rtID, id, in)
}

func (s *service) ToggleTemplateActive(ctx context.Context, rtID, id string) (*NotificationTemplate, error) {
	existing, err := s.repo.GetTemplateByID(ctx, rtID, id)
	if err != nil {
		return nil, err
	}
	if existing != nil && IsFixedSystemEvent(existing.EventType) {
		return nil, errors.New("template notifikasi sistem tetap tidak dapat dinonaktifkan")
	}
	return s.repo.ToggleTemplateActive(ctx, rtID, id)
}

func (s *service) DeleteTemplate(ctx context.Context, rtID, id string) error {
	existing, err := s.repo.GetTemplateByID(ctx, rtID, id)
	if err != nil {
		return err
	}
	if existing != nil && IsFixedSystemEvent(existing.EventType) {
		return errors.New("template notifikasi sistem tetap tidak dapat dihapus")
	}
	return s.repo.DeleteTemplate(ctx, rtID, id)
}

// ListRWTemplates lists notification templates scoped to an RW.
func (s *service) ListRWTemplates(ctx context.Context, rwID, eventType string, isActive *bool) ([]NotificationTemplate, error) {
	return s.repo.ListRWTemplates(ctx, rwID, eventType, isActive)
}

// GetRWTemplateByID gets an RW-scoped template by ID.
func (s *service) GetRWTemplateByID(ctx context.Context, rwID, id string) (*NotificationTemplate, error) {
	return s.repo.GetRWTemplateByID(ctx, rwID, id)
}

// CreateRWTemplate creates a new RW-scoped notification template.
func (s *service) CreateRWTemplate(ctx context.Context, rwID string, in CreateTemplateInput) (*NotificationTemplate, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("nama template wajib diisi")
	}
	if strings.TrimSpace(in.EventType) == "" {
		return nil, errors.New("jenis event wajib dipilih")
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("judul notifikasi wajib diisi")
	}
	if strings.TrimSpace(in.Body) == "" {
		return nil, errors.New("isi pesan notifikasi wajib diisi")
	}

	sysType := findSystemType(in.EventType)
	if sysType == nil {
		return nil, fmt.Errorf("jenis event '%s' tidak valid atau tidak didukung", in.EventType)
	}

	if in.ImageURL != nil && strings.TrimSpace(*in.ImageURL) != "" {
		if !sysType.SupportsImage {
			return nil, fmt.Errorf("gambar tidak diizinkan untuk event '%s'", in.EventType)
		}
	}

	if err := validateTemplateVariables(in.Title, sysType.AllowedVariables); err != nil {
		return nil, err
	}
	if err := validateTemplateVariables(in.Body, sysType.AllowedVariables); err != nil {
		return nil, err
	}

	return s.repo.CreateRWTemplate(ctx, rwID, in, sysType.AllowedVariables)
}

// UpdateRWTemplate updates an existing RW-scoped notification template.
func (s *service) UpdateRWTemplate(ctx context.Context, rwID, id string, in UpdateTemplateInput) (*NotificationTemplate, error) {
	existing, err := s.repo.GetRWTemplateByID(ctx, rwID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("template tidak ditemukan")
	}

	sysType := findSystemType(existing.EventType)
	if sysType == nil {
		return nil, errors.New("jenis event tidak valid")
	}

	if in.ImageURL != nil && strings.TrimSpace(*in.ImageURL) != "" {
		if !sysType.SupportsImage {
			return nil, fmt.Errorf("gambar tidak diizinkan untuk event '%s'", existing.EventType)
		}
	}

	if in.Name != "" && strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("nama template tidak boleh kosong")
	}
	if in.Title != "" && strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("judul notifikasi tidak boleh kosong")
	}
	if in.Body != "" && strings.TrimSpace(in.Body) == "" {
		return nil, errors.New("isi pesan notifikasi tidak boleh kosong")
	}

	if in.Title != "" {
		if err := validateTemplateVariables(in.Title, sysType.AllowedVariables); err != nil {
			return nil, err
		}
	}
	if in.Body != "" {
		if err := validateTemplateVariables(in.Body, sysType.AllowedVariables); err != nil {
			return nil, err
		}
	}

	return s.repo.UpdateRWTemplate(ctx, rwID, id, in)
}

// ToggleRWTemplateActive toggles the is_active flag on an RW-scoped template.
func (s *service) ToggleRWTemplateActive(ctx context.Context, rwID, id string) (*NotificationTemplate, error) {
	existing, err := s.repo.GetRWTemplateByID(ctx, rwID, id)
	if err != nil {
		return nil, err
	}
	if existing != nil && IsFixedSystemEvent(existing.EventType) {
		return nil, errors.New("template notifikasi sistem tetap tidak dapat dinonaktifkan")
	}
	return s.repo.ToggleRWTemplateActive(ctx, rwID, id)
}

// DeleteRWTemplate deletes an RW-scoped notification template.
func (s *service) DeleteRWTemplate(ctx context.Context, rwID, id string) error {
	existing, err := s.repo.GetRWTemplateByID(ctx, rwID, id)
	if err != nil {
		return err
	}
	if existing != nil && IsFixedSystemEvent(existing.EventType) {
		return errors.New("template notifikasi sistem tetap tidak dapat dihapus")
	}
	return s.repo.DeleteRWTemplate(ctx, rwID, id)
}

func isOfficerUser(ac *auth.AuthContext) bool {
	if ac == nil {
		return false
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return true
	}
	j := strings.ToLower(strings.TrimSpace(string(ac.Jabatan)))
	if j == "ketua" || j == "ketua_rt" || j == "ketua rt" || j == "bendahara" || j == "wakil_ketua" || j == "sekretaris" {
		return true
	}
	r := strings.ToLower(strings.TrimSpace(string(ac.TenantRole)))
	return r == "bendahara" || r == "pengurus"
}

func (s *service) ListLogs(ctx context.Context, ac *auth.AuthContext, rtID, eventType, status string, page, pageSize int) ([]NotificationLogEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	userID := ""
	if ac != nil {
		userID = ac.UserID
	}
	return s.repo.ListLogs(ctx, rtID, userID, eventType, status, false, offset, pageSize)
}

func (s *service) ListManagementHistory(
	ctx context.Context,
	ac *auth.AuthContext,
	rtID, category string,
	page, pageSize int,
) ([]NotificationLogEntry, int, error) {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return nil, 0, errors.New("unauthorized: missing auth context")
	}
	if !isOfficerUser(ac) {
		return nil, 0, ErrForbidden
	}
	// Security officers (keamanan) do NOT have notification.manage permission
	// and must NOT see system/management aggregated logs or other residents' data.
	if isSecurityOfficer(ac) {
		return nil, 0, ErrForbidden
	}
	if category != HistoryCategorySystem && category != HistoryCategoryMine {
		return nil, 0, errors.New("invalid notification history category")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	return s.repo.ListManagementHistory(ctx, rtID, ac.UserID, category, offset, pageSize)
}

func (s *service) MarkAsRead(ctx context.Context, ac *auth.AuthContext, rtID, id string) error {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return errors.New("unauthorized: missing auth context")
	}
	return s.repo.MarkAsRead(ctx, rtID, ac.UserID, id, false)
}

func (s *service) MarkAllAsRead(ctx context.Context, ac *auth.AuthContext, rtID string) (int, error) {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return 0, errors.New("unauthorized: missing auth context")
	}
	return s.repo.MarkAllAsRead(ctx, rtID, ac.UserID, false)
}

func (s *service) GetUnreadCount(ctx context.Context, ac *auth.AuthContext, rtID string) (int, error) {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return 0, errors.New("unauthorized: missing auth context")
	}
	return s.repo.GetUnreadCount(ctx, rtID, ac.UserID, false)
}

func (s *service) GetNotificationDetail(ctx context.Context, ac *auth.AuthContext, rtID, id string, canManage bool) (*NotificationLogEntry, error) {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return nil, errors.New("unauthorized: missing auth context")
	}
	rtID = strings.TrimSpace(rtID)
	id = strings.TrimSpace(id)
	if rtID == "" || id == "" {
		return nil, nil
	}

	entry, err := s.repo.GetLogByID(ctx, rtID, id)
	if err != nil || entry == nil {
		return entry, err
	}
	if entry.RTID != rtID {
		return nil, nil
	}

	// Test push notifications are strictly isolated to the Super Admin who triggered them.
	if strings.HasPrefix(entry.DedupKey, "test_push:") {
		if entry.UserID == nil || *entry.UserID != ac.UserID {
			return nil, ErrForbidden
		}
		_ = s.repo.MarkAsRead(ctx, rtID, ac.UserID, id, false)
		entry.IsRead = true
		now := time.Now()
		entry.ReadAt = &now
		return entry, nil
	}

	isOwner := entry.UserID != nil && *entry.UserID == ac.UserID
	isSender := entry.SenderUserID != nil && *entry.SenderUserID == ac.UserID

	// Dispatch records remain readable by their sender or notification managers.
	if entry.EventType == EventManualPush || entry.EventType == EventWasteTruckArrival {
		if !isOwner && !isSender && !canManage {
			recipientEntry, recipientErr := s.repo.GetBroadcastRecipientLog(ctx, rtID, ac.UserID, entry.EventType, entry.ID)
			if recipientErr != nil {
				return nil, recipientErr
			}
			if recipientEntry == nil {
				return nil, ErrForbidden
			}
			entry = recipientEntry
			isOwner = entry.UserID != nil && *entry.UserID == ac.UserID
			isSender = entry.SenderUserID != nil && *entry.SenderUserID == ac.UserID
		}
	}

	if !isOwner && !isSender && !canManage {
		return nil, ErrForbidden
	}
	if entry.UserID != nil && *entry.UserID != "" {
		if *entry.UserID != ac.UserID && !canManage {
			return nil, ErrForbidden
		}
		_ = s.repo.MarkAsRead(ctx, rtID, ac.UserID, entry.ID, false)
		entry.IsRead = true
		now := time.Now()
		entry.ReadAt = &now
		return entry, nil
	}
	if !canManage {
		return nil, ErrForbidden
	}
	_ = s.repo.MarkAsRead(ctx, rtID, ac.UserID, entry.ID, false)
	entry.IsRead = true
	now := time.Now()
	entry.ReadAt = &now
	return entry, nil
}

func (s *service) ResolveTenantRT(ctx context.Context, ac *auth.AuthContext, reqHeaderRT, reqQueryRT string) (string, error) {
	if ac == nil {
		return "", errors.New("unauthorized: missing auth context")
	}
	if ac.RTID != "" {
		return ac.RTID, nil
	}
	// At this point ac.RTID is empty. The RequireAllowedRT middleware
	// (if present) already enforced a valid RTID for the request.
	// For handlers that rely on this fallback without middleware,
	// reject with a clear error rather than silently falling back to a
	// default RT — that bypasses tenant isolation and the whitelist.
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return "", fmt.Errorf("super admin must specify a target RT via X-RT-ID header or JWT membership")
	}
	return "", fmt.Errorf("missing tenant context: user has no active RT membership")
}

// Schedule management

func (s *service) ListSchedules(ctx context.Context, rtID, eventType string, isActive *bool) ([]*NotificationSchedule, error) {
	return s.repo.ListSchedules(ctx, rtID, eventType, isActive)
}

func (s *service) GetSchedule(ctx context.Context, rtID, id string) (*NotificationSchedule, error) {
	return s.repo.GetSchedule(ctx, rtID, id)
}

func (s *service) CreateSchedule(ctx context.Context, rtID, userID string, in CreateScheduleInput) (*NotificationSchedule, error) {
	if IsFixedSystemEvent(in.EventType) {
		return nil, errors.New("event ini merupakan notifikasi sistem tetap dan tidak dapat dijadwalkan")
	}
	if in.EventType == EventRWCashIn || in.EventType == EventRWCashOut {
		return nil, errors.New("event kas RW tidak dapat digunakan pada jadwal notifikasi tingkat RT")
	}
	if strings.TrimSpace(in.EventType) == "" {
		return nil, errors.New("jenis event wajib diisi")
	}
	if strings.TrimSpace(in.TemplateID) == "" {
		return nil, errors.New("template wajib dipilih")
	}
	sysType := findSystemType(in.EventType)
	if sysType == nil {
		return nil, fmt.Errorf("jenis event '%s' tidak valid atau tidak didukung", in.EventType)
	}
	// Template must exist, belong to this RT, match the event type, and be active.
	tmpl, err := s.repo.GetTemplateByID(ctx, rtID, in.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("template tidak ditemukan: %w", err)
	}
	if tmpl == nil {
		return nil, errors.New("template tidak ditemukan")
	}
	if tmpl.EventType != in.EventType {
		return nil, errors.New("template tidak cocok dengan jenis event yang dipilih")
	}
	if !tmpl.IsActive {
		return nil, errors.New("template tidak aktif, tidak dapat digunakan dalam jadwal")
	}
	return s.repo.CreateSchedule(ctx, rtID, userID, in)
}

func (s *service) UpdateSchedule(ctx context.Context, rtID, id, userID string, in UpdateScheduleInput) (*NotificationSchedule, error) {
	return s.repo.UpdateSchedule(ctx, rtID, id, userID, in)
}

func (s *service) DeleteSchedule(ctx context.Context, rtID, id string) error {
	return s.repo.DeleteSchedule(ctx, rtID, id)
}

func (s *service) ToggleScheduleActive(ctx context.Context, rtID, id, userID string) (*NotificationSchedule, error) {
	existing, err := s.repo.GetSchedule(ctx, rtID, id)
	if err != nil {
		return nil, err
	}
	if existing != nil && IsFixedSystemEvent(existing.EventType) {
		return nil, errors.New("event ini merupakan notifikasi sistem tetap dan tidak dapat dijadwalkan")
	}
	return s.repo.ToggleScheduleActive(ctx, rtID, id, userID)
}

func (s *service) GetDueSchedules(ctx context.Context, now time.Time) ([]*NotificationSchedule, error) {
	return s.repo.GetDueSchedules(ctx, now)
}

// BuildScheduleDedupKey constructs a deterministic deduplication key for a scheduled notification delivery.
// It preserves the prefix format `schedule:<schedule_id>:<exec_date>:<user_id>...` (or with `:c_<config_hash>`)
// required by ListSchedulerHistory and GetSchedulerRecipientDetail while incorporating eventType, periodKey,
// and billGroupKey when provided.
func BuildScheduleDedupKey(scheduleID, execDate, userID, eventType, periodKey, billGroupKey string, configHash ...string) string {
	cfg := ""
	if len(configHash) > 0 && configHash[0] != "" {
		cfg = ":c_" + configHash[0]
	}
	if eventType == "" && periodKey == "" && billGroupKey == "" {
		return fmt.Sprintf("schedule:%s:%s%s:%s", scheduleID, execDate, cfg, userID)
	}
	return fmt.Sprintf("schedule:%s:%s%s:%s:%s:%s:%s", scheduleID, execDate, cfg, userID, eventType, periodKey, billGroupKey)
}

type scheduledRecipientGroup struct {
	userID                string
	userFullName          string
	rtID                  string
	primaryBillID         string
	periodDisplay         string
	amountDisplay         string
	totalOutstandingCents int64
	billCount             int
	dueDateDisplay        string
	deepLink              string
	periodKey             string
	billGroupKey          string
	dedupKey              string
}

func (s *service) resolveOverdueOrUpcomingRecipientGroups(
	ctx context.Context,
	schedule *NotificationSchedule,
	localNow time.Time,
	execDate string,
	configHash string,
) ([]scheduledRecipientGroup, error) {
	extRepo, ok := s.repo.(OverdueReminderRepository)
	if !ok {
		return nil, nil
	}

	bills, err := extRepo.GetRTUserBillsForEvaluation(ctx, schedule.RTID)
	if err != nil {
		return nil, fmt.Errorf("get rt user bills for evaluation: %w", err)
	}
	if len(bills) == 0 {
		return nil, nil
	}

	periodKey := "upcoming_3m"
	if schedule.EventType == EventOverdue3Months {
		periodKey = finance.NormalizeDateJakarta(localNow).Format("2006-01")
	}

	byUser := make(map[string][]finance.EligibleBillResult)
	var userOrder []string

	for _, b := range bills {
		var (
			res      finance.EligibleBillResult
			eligible bool
		)
		switch schedule.EventType {
		case EventUpcomingDue3Months:
			res, eligible = finance.EvaluateBillForUpcomingDue3Months(b, localNow)
		case EventOverdue3Months:
			res, eligible = finance.EvaluateBillForOverdue3Months(b, localNow)
		default:
			continue
		}
		if !eligible {
			continue
		}

		alreadySent, err := extRepo.IsReminderAlreadySentForPeriod(ctx, schedule.RTID, b.UserID, schedule.EventType, periodKey, b.BillID)
		if err != nil {
			return nil, err
		}
		if alreadySent {
			continue
		}

		if _, exists := byUser[b.UserID]; !exists {
			userOrder = append(userOrder, b.UserID)
		}
		byUser[b.UserID] = append(byUser[b.UserID], res)
	}

	sort.Strings(userOrder)
	groups := make([]scheduledRecipientGroup, 0, len(userOrder))

	for _, uid := range userOrder {
		userBills := byUser[uid]
		if len(userBills) == 0 {
			continue
		}
		sort.Slice(userBills, func(i, j int) bool {
			di, dj := time.Time{}, time.Time{}
			if userBills[i].Bill.DueDate != nil {
				di = *userBills[i].Bill.DueDate
			}
			if userBills[j].Bill.DueDate != nil {
				dj = *userBills[j].Bill.DueDate
			}
			if !di.Equal(dj) {
				return di.Before(dj)
			}
			if userBills[i].Bill.Period != userBills[j].Bill.Period {
				return userBills[i].Bill.Period < userBills[j].Bill.Period
			}
			return userBills[i].Bill.BillID < userBills[j].Bill.BillID
		})

		first := userBills[0]
		var totalCents int64
		billIDs := make([]string, 0, len(userBills))
		periods := make([]string, 0, len(userBills))
		seenPeriods := make(map[string]bool)

		for _, ub := range userBills {
			totalCents += ub.OutstandingCents
			billIDs = append(billIDs, ub.Bill.BillID)
			if ub.Bill.Period != "" && !seenPeriods[ub.Bill.Period] {
				seenPeriods[ub.Bill.Period] = true
				periods = append(periods, ub.Bill.Period)
			}
		}

		billGroupKey := strings.Join(billIDs, ",")
		periodDisplay := first.Bill.Period
		if len(periods) > 1 {
			periodDisplay = strings.Join(periods, ", ")
		}

		fullName := strings.TrimSpace(first.Bill.UserFullName)
		if fullName == "" {
			fullName = "Warga"
		}

		deepLink := fmt.Sprintf("/home/iuran/detail/%s", first.Bill.BillID)
		if len(userBills) > 1 {
			deepLink = "/home/iuran"
		}

		dedupKey := BuildScheduleDedupKey(schedule.ID, execDate, uid, schedule.EventType, periodKey, billGroupKey, configHash)

		groups = append(groups, scheduledRecipientGroup{
			userID:                uid,
			userFullName:          fullName,
			rtID:                  schedule.RTID,
			primaryBillID:         first.Bill.BillID,
			periodDisplay:         periodDisplay,
			amountDisplay:         finance.FormatRupiahDisplay(totalCents),
			totalOutstandingCents: totalCents,
			billCount:             len(userBills),
			dueDateDisplay:        first.DueDateFormatted,
			deepLink:              deepLink,
			periodKey:             periodKey,
			billGroupKey:          billGroupKey,
			dedupKey:              dedupKey,
		})
	}

	return groups, nil
}

// type, renders title/body from the linked template, sends pushes with
// per-recipient idempotency, and updates the schedule's last execution info.
//
// Execution guarantees (at-most-once delivery, no duplicate pushes):
//
//  1. A dedicated DB connection is acquired and a SESSION-LEVEL Postgres
//     advisory lock (pg_try_advisory_lock) keyed by (schedule ID, execution
//     date) is held for the WHOLE execution, including the FCM network call.
//     Only one backend instance can hold this lock for a given
//     (schedule, execDate) at a time, so concurrent instances skip immediately.
//     The lock is released when the connection is closed (also on crash), so
//     there is no deadlock risk.
//
//  2. The database transaction is NOT held during the network call: only a
//     short tx is used at the end to insert delivery logs (with actual
//     SENT/FAILED status) and update the schedule's last execution info.
//
//  3. The UNIQUE dedup_key on notification_logs
//     (schedule:<id>:<local-date>:<userId>) is the final backstop: even if two
//     instances somehow both send, only one log row sticks per recipient per
//     date, and the log status reflects the actual delivery result.
//
// Logs distinguish provider outcomes from unknown/no-attempt outcomes. A
// missing token is UNKNOWN, not FAILED; only provider-reported failures are
// FAILED. Partial provider results are recorded per recipient and aggregated.
func (s *service) ExecuteSchedule(ctx context.Context, schedule *NotificationSchedule, now time.Time) (int, error) {
	if schedule == nil {
		return 0, errors.New("schedule is required")
	}

	rtID := schedule.RTID

	// Execution date in the schedule's own timezone (for dedup key + lock).
	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		loc = time.FixedZone(schedule.Timezone, 7*3600) // default WIB
	}
	execDate := now.In(loc).Format("2006-01-02")
	configHash := ComputeScheduleConfigHashFromSchedule(schedule)

	// Acquire a dedicated connection and hold a session-level advisory lock for
	// the whole execution (network call included). This guarantees at-most-once
	// execution across multiple backend instances without holding a DB tx open
	// during the network call. In tests (mock repo) conn may be nil; the lock
	// is then skipped because tests run a single instance per schedule.
	var conn *sql.Conn
	conn, err = s.repo.AcquireConn(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire conn: %w", err)
	}
	if conn != nil {
		defer conn.Close()
		lockKey := fmt.Sprintf("%s:c_%s", execDate, configHash)
		claimed, err := s.repo.TryAdvisoryLockConn(conn, ctx, schedule.ID, lockKey)
		if err != nil {
			return 0, fmt.Errorf("try advisory lock: %w", err)
		}
		if !claimed {
			slog.Info("schedule execution claimed by another instance", "schedule_id", schedule.ID, "exec_date", execDate, "config_hash", configHash)
			return 0, nil
		}
		defer s.repo.ReleaseAdvisoryLockConn(conn, ctx, schedule.ID, lockKey)
	}

	// Defensive schedule-level dedup: already completed today (SENT/FAILED) for this configuration version?
	already, err := s.repo.IsScheduleExecutedToday(ctx, schedule.ID, execDate, configHash)
	if err != nil {
		return 0, fmt.Errorf("check executed today: %w", err)
	}
	if already {
		slog.Info("schedule already executed today for this config version", "schedule_id", schedule.ID, "exec_date", execDate, "config_hash", configHash)
		return 0, nil
	}

	bgCtx := ctx

	// Resolve the active template: prefer the schedule's assigned template if valid and active,
	// otherwise fallback to the active template for this event type + RT.
	var tmpl *NotificationTemplate
	if schedule.TemplateID != "" {
		t, err := s.repo.GetTemplateByID(bgCtx, rtID, schedule.TemplateID)
		if err == nil && t != nil && t.IsActive {
			tmpl = t
		}
	}
	if tmpl == nil {
		t, err := s.repo.GetActiveTemplateForEvent(bgCtx, rtID, schedule.EventType)
		if err == nil && t != nil && t.IsActive {
			tmpl = t
		}
	}
	if tmpl == nil {
		errMsg := "Template not found or inactive"
		_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, "failed", &errMsg, &ScheduleExecutionSummary{
			TotalRecipients: 0,
			SentCount:       0,
			FailedCount:     0,
			NoDeviceCount:   0,
			FailureReason:   errMsg,
		}, configHash)
		slog.Warn("no active template for reminder schedule", "rt_id", rtID, "schedule_id", schedule.ID, "event_type", schedule.EventType)
		return 0, fmt.Errorf("active template not found or inactive for schedule %s (event_type: %s)", schedule.ID, schedule.EventType)
	}

	title := tmpl.Title
	body := tmpl.Body

	type delivery struct {
		userID        string
		dedupKey      string
		title         string
		body          string
		payload       map[string]string
		billID        string
		status        string
		failureReason string
	}
	var deliveries []delivery
	recipientStatuses := make([]string, 0)
	sent := 0
	switch schedule.EventType {
	case EventPaymentReminder:
		recipients, err := s.repo.GetIuranReminderRecipients(bgCtx, rtID, now)
		if err != nil {
			errMsg := fmt.Sprintf("failed to resolve iuran reminder recipients: %v", err)
			_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, "failed", &errMsg, &ScheduleExecutionSummary{
				FailureReason: errMsg,
			}, configHash)
			slog.Error("failed to resolve iuran reminder recipients", "rt_id", rtID, "error", err)
			return 0, err
		}
		if len(recipients) == 0 {
			status := "no_recipients"
			reason := "Tidak ada tagihan yang memenuhi kriteria penerima"
			_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, status, &reason, &ScheduleExecutionSummary{
				TotalRecipients: 0,
				SentCount:       0,
				FailedCount:     0,
				NoDeviceCount:   0,
				FailureReason:   reason,
			}, configHash)
			slog.Info("payment reminder schedule executed with zero recipients", "schedule_id", schedule.ID)
			return 0, nil
		}
		deliveries = make([]delivery, 0, len(recipients))

		for _, rec := range recipients {
			userName := strings.TrimSpace(rec.UserFullName)
			if userName == "" {
				userName = "Warga"
			}
			vars := map[string]string{
				"nama":      userName,
				"bill_id":   rec.BillID,
				"period":    rec.Period,
				"amount":    rec.Amount,
				"due_date":  rec.DueDate,
				"deep_link": fmt.Sprintf("/home/iuran/detail/%s", rec.BillID),
			}
			t := interpolate(title, vars)
			b := interpolate(body, vars)

			payload := NotificationPayload{
				Type:     schedule.EventType,
				RTID:     rec.RTID,
				Title:    t,
				Body:     b,
				BillID:   rec.BillID,
				DeepLink: vars["deep_link"],
				SentAt:   now.UTC().Format(time.RFC3339Nano),
			}
			payloadMap := payload.ToMap()
			dedupKey := BuildScheduleDedupKey(schedule.ID, execDate, rec.UserID, "", "", "", configHash)

			alreadySent, _ := s.repo.IsRecipientAlreadySentToday(bgCtx, dedupKey)
			if alreadySent {
				deliveries = append(deliveries, delivery{
					userID: rec.UserID, dedupKey: dedupKey, title: t, body: b,
					payload: payloadMap, billID: rec.BillID, status: "SENT",
				})
				recipientStatuses = append(recipientStatuses, "SENT")
				sent++
				continue
			}

			tokens, err := s.repo.GetTokensForUser(bgCtx, rec.UserID)
			if err != nil {
				slog.Error("failed to get tokens for reminder recipient", "user_id", rec.UserID, "error", err)
				deliveries = append(deliveries, delivery{
					userID: rec.UserID, dedupKey: dedupKey, title: t, body: b,
					payload: payloadMap, billID: rec.BillID, status: "UNKNOWN",
				})
				recipientStatuses = append(recipientStatuses, "UNKNOWN")
				continue
			}
			if len(tokens) == 0 {
				deliveries = append(deliveries, delivery{
					userID: rec.UserID, dedupKey: dedupKey, title: t, body: b,
					payload: payloadMap, billID: rec.BillID, status: "NO_DEVICE",
				})
				recipientStatuses = append(recipientStatuses, "NO_DEVICE")
				continue
			}

			sentN, failedN := s.sendPushToTokens(bgCtx, tokens, t, b, payloadMap)
			sent += sentN
			status := recipientDeliveryStatus(sentN, failedN, len(tokens))
			deliveries = append(deliveries, delivery{
				userID: rec.UserID, dedupKey: dedupKey, title: t, body: b,
				payload: payloadMap, billID: rec.BillID, status: status,
				failureReason: deliveryFailureReason(status),
			})
			recipientStatuses = append(recipientStatuses, status)
		}

	case EventUpcomingDue3Months, EventOverdue3Months:
		groups, err := s.resolveOverdueOrUpcomingRecipientGroups(bgCtx, schedule, now.In(loc), execDate, configHash)
		if err != nil {
			errMsg := fmt.Sprintf("failed to resolve upcoming/overdue iuran recipients: %v", err)
			_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, "failed", &errMsg, &ScheduleExecutionSummary{
				FailureReason: errMsg,
			}, configHash)
			slog.Error("failed to resolve upcoming/overdue iuran recipients", "rt_id", rtID, "event_type", schedule.EventType, "error", err)
			return 0, err
		}
		if len(groups) == 0 {
			status := "no_recipients"
			reason := "Tidak ada tagihan yang memenuhi kriteria penerima"
			_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, status, &reason, &ScheduleExecutionSummary{
				TotalRecipients: 0,
				SentCount:       0,
				FailedCount:     0,
				NoDeviceCount:   0,
				FailureReason:   reason,
			}, configHash)
			slog.Info("overdue/upcoming schedule executed with zero recipient groups", "schedule_id", schedule.ID)
			return 0, nil
		}
		deliveries = make([]delivery, 0, len(groups))

		for _, grp := range groups {
			vars := map[string]string{
				"nama":              grp.userFullName,
				"bill_id":           grp.primaryBillID,
				"period":            grp.periodDisplay,
				"amount":            grp.amountDisplay,
				"total_outstanding": grp.amountDisplay,
				"bill_count":        strconv.Itoa(grp.billCount),
				"due_date":          grp.dueDateDisplay,
				"deep_link":         grp.deepLink,
			}
			t := interpolate(title, vars)
			b := interpolate(body, vars)

			payload := NotificationPayload{
				Type:     schedule.EventType,
				RTID:     grp.rtID,
				Title:    t,
				Body:     b,
				BillID:   grp.primaryBillID,
				DeepLink: grp.deepLink,
				SentAt:   now.UTC().Format(time.RFC3339Nano),
			}
			payloadMap := payload.ToMap()
			payloadMap["reminder_period"] = grp.periodKey
			payloadMap["bill_ids"] = grp.billGroupKey
			payloadMap["bill_count"] = strconv.Itoa(grp.billCount)
			payloadMap["total_outstanding"] = strconv.FormatInt(grp.totalOutstandingCents/100, 10)

			alreadySent, _ := s.repo.IsRecipientAlreadySentToday(bgCtx, grp.dedupKey)
			if alreadySent {
				deliveries = append(deliveries, delivery{
					userID: grp.userID, dedupKey: grp.dedupKey, title: t, body: b,
					payload: payloadMap, billID: grp.primaryBillID, status: "SENT",
				})
				recipientStatuses = append(recipientStatuses, "SENT")
				sent++
				continue
			}

			tokens, err := s.repo.GetTokensForUser(bgCtx, grp.userID)
			if err != nil {
				slog.Error("failed to get tokens for upcoming/overdue recipient", "user_id", grp.userID, "error", err)
				deliveries = append(deliveries, delivery{
					userID: grp.userID, dedupKey: grp.dedupKey, title: t, body: b,
					payload: payloadMap, billID: grp.primaryBillID, status: "UNKNOWN",
				})
				recipientStatuses = append(recipientStatuses, "UNKNOWN")
				continue
			}
			if len(tokens) == 0 {
				deliveries = append(deliveries, delivery{
					userID: grp.userID, dedupKey: grp.dedupKey, title: t, body: b,
					payload: payloadMap, billID: grp.primaryBillID, status: "NO_DEVICE",
				})
				recipientStatuses = append(recipientStatuses, "NO_DEVICE")
				continue
			}

			sentN, failedN := s.sendPushToTokens(bgCtx, tokens, t, b, payloadMap)
			sent += sentN
			status := recipientDeliveryStatus(sentN, failedN, len(tokens))
			deliveries = append(deliveries, delivery{
				userID: grp.userID, dedupKey: grp.dedupKey, title: t, body: b,
				payload: payloadMap, billID: grp.primaryBillID, status: status,
				failureReason: deliveryFailureReason(status),
			})
			recipientStatuses = append(recipientStatuses, status)
		}

	default:
		errMsg := fmt.Sprintf("unsupported event type for scheduled execution: %s", schedule.EventType)
		_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, "failed", &errMsg, &ScheduleExecutionSummary{
			FailureReason: errMsg,
		}, configHash)
		return 0, fmt.Errorf("%s", errMsg)
	}

	// Record per-recipient delivery logs with ACTUAL status and update the
	// schedule's last execution info. The network call is already finished;
	// the advisory lock is still held. Each log insert is idempotent
	// (UNIQUE dedup_key, ON CONFLICT DO NOTHING), so retries are safe.
	for _, d := range deliveries {
		if _, err := s.repo.RecordScheduleDelivery(
			ctx, rtID, d.userID, schedule.EventType, d.dedupKey,
			d.title, d.body, d.payload, d.status, d.failureReason,
		); err != nil {
			slog.Warn("failed to record schedule delivery log", "user_id", d.userID, "error", err)
		}
	}

	summary := ScheduleExecutionSummary{
		TotalRecipients: len(deliveries),
	}
	for _, d := range deliveries {
		switch d.status {
		case "SENT":
			summary.SentCount++
		case "PARTIAL":
			summary.SentCount++
			summary.FailedCount++
		case "FAILED":
			summary.FailedCount++
		case "NO_DEVICE":
			summary.NoDeviceCount++
		}
	}

	var (
		finalStatus   string
		execErrDetail *string
	)
	if summary.SentCount > 0 && summary.FailedCount == 0 {
		finalStatus = "success"
	} else if summary.SentCount > 0 && summary.FailedCount > 0 {
		finalStatus = "partial"
		msg := "Sebagian perangkat gagal menerima push notifikasi"
		execErrDetail = &msg
		summary.FailureReason = msg
	} else if summary.SentCount == 0 && summary.FailedCount > 0 {
		finalStatus = "failed"
		msg := "Pengiriman push ke perangkat gagal untuk seluruh penerima"
		execErrDetail = &msg
		summary.FailureReason = msg
	} else if summary.SentCount == 0 && summary.FailedCount == 0 && summary.NoDeviceCount > 0 {
		finalStatus = "no_device"
		msg := "Seluruh penerima tidak memiliki perangkat terdaftar (notifikasi in-app tetap tersimpan)"
		execErrDetail = &msg
		summary.FailureReason = msg
	} else {
		finalStatus = "success"
	}

	if err := s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, finalStatus, execErrDetail, &summary, configHash); err != nil {
		slog.Warn("failed to update schedule last executed detailed", "schedule_id", schedule.ID, "error", err)
	}

	return sent, nil
}

func (s *service) ListSchedulerHistory(ctx context.Context, rtID string, page, pageSize int) ([]*NotificationSchedulerHistory, int, error) {
	return s.repo.ListSchedulerHistory(ctx, rtID, page, pageSize)
}

func (s *service) GetSchedulerRecipientDetail(ctx context.Context, rtID, scheduleID, execDate string, configHash ...string) ([]*NotificationSchedulerRecipientDetail, error) {
	return s.repo.GetSchedulerRecipientDetail(ctx, rtID, scheduleID, execDate, configHash...)
}

func (s *service) NotifyRWCashIn(ctx context.Context, rwID, txID, amount, description, locationName string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		targets, err := s.repo.GetRWPengurusUserIDs(bgCtx, rwID)
		if err != nil {
			slog.Error("failed to find recipients for rw_cash_in", "rw_id", rwID, "error", err)
			return
		}

		title := "Kas Masuk RW Diterima"
		body := fmt.Sprintf("Penerimaan kas masuk RW sebesar Rp %s telah dicatat.", amount)
		if strings.TrimSpace(description) != "" {
			body = fmt.Sprintf("Penerimaan kas masuk RW sebesar Rp %s (%s) telah dicatat.", amount, description)
		}

		payload := map[string]string{
			"type":        EventRWCashIn,
			"rw_id":       rwID,
			"tx_id":       txID,
			"amount":      amount,
			"description": description,
			"location":    locationName,
			"sent_at":     time.Now().UTC().Format(time.RFC3339Nano),
		}

		for _, uid := range targets {
			dedupKey := fmt.Sprintf("rw_kas:%s:income:%s", txID, uid)
			ok, err := s.repo.RecordRWDedup(bgCtx, rwID, uid, EventRWCashIn, dedupKey, title, body, payload, nil)
			if err != nil {
				slog.Warn("rw_cash_in dedup check failed", "user_id", uid, "error", err)
			}
			if !ok {
				continue
			}
			tokens, _ := s.repo.GetTokensForUser(bgCtx, uid)
			s.sendPushToTokens(bgCtx, tokens, title, body, payload)
		}
	}()
}

func (s *service) NotifyRWCashOut(ctx context.Context, rwID, txID, amount, description, category, locationName string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		targets, err := s.repo.GetRWPengurusUserIDs(bgCtx, rwID)
		if err != nil {
			slog.Error("failed to find recipients for rw_cash_out", "rw_id", rwID, "error", err)
			return
		}

		title := "Kas Keluar RW Dicatat"
		body := fmt.Sprintf("Pengeluaran kas RW sebesar Rp %s untuk %s telah dicatat.", amount, description)

		payload := map[string]string{
			"type":        EventRWCashOut,
			"rw_id":       rwID,
			"tx_id":       txID,
			"amount":      amount,
			"description": description,
			"category":    category,
			"location":    locationName,
			"sent_at":     time.Now().UTC().Format(time.RFC3339Nano),
		}

		for _, uid := range targets {
			dedupKey := fmt.Sprintf("rw_kas:%s:expense:%s", txID, uid)
			ok, err := s.repo.RecordRWDedup(bgCtx, rwID, uid, EventRWCashOut, dedupKey, title, body, payload, nil)
			if err != nil {
				slog.Warn("rw_cash_out dedup check failed", "user_id", uid, "error", err)
			}
			if !ok {
				continue
			}
			tokens, _ := s.repo.GetTokensForUser(bgCtx, uid)
			s.sendPushToTokens(bgCtx, tokens, title, body, payload)
		}
	}()
}

func (s *service) SendRWManualPush(ctx context.Context, rwID, senderUserID, title, body string, target ...string) (int, error) {
	var targets []string
	var err error
	if len(target) > 0 && strings.TrimSpace(target[0]) == "rw_only" {
		targets, err = s.repo.GetRWPengurusUserIDs(ctx, rwID)
	} else {
		targets, err = s.repo.GetRWAndRTPengurusUserIDs(ctx, rwID)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get RW manual push recipients: %w", err)
	}
	if len(targets) == 0 {
		return 0, nil
	}

	payload := map[string]string{
		"type":           EventManualPush,
		"rw_id":          rwID,
		"title":          title,
		"body":           body,
		"sender_user_id": senderUserID,
		"sent_at":        time.Now().UTC().Format(time.RFC3339Nano),
	}

	batchID := fmt.Sprintf("rw_manual:%s:%d", rwID, time.Now().UnixNano())
	sentCount := 0
	senderPtr := &senderUserID
	for _, uid := range targets {
		dedupKey := fmt.Sprintf("%s:%s", batchID, uid)
		ok, err := s.repo.RecordRWDedup(ctx, rwID, uid, EventManualPush, dedupKey, title, body, payload, senderPtr)
		if err != nil {
			slog.Warn("rw manual push dedup failed", "user_id", uid, "error", err)
		}
		if !ok {
			continue
		}
		tokens, err := s.repo.GetTokensForUser(ctx, uid)
		if err == nil && len(tokens) > 0 {
			s.sendPushToTokens(ctx, tokens, title, body, payload)
		}
		sentCount++
	}
	return sentCount, nil
}

func (s *service) ListRWLogs(ctx context.Context, rwID, eventType string, page, pageSize int) ([]NotificationLogEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.ListRWLogs(ctx, rwID, eventType, offset, pageSize)
}
