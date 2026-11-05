package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"social-finance/internal/auth"
)

// Service defines notification operations.
type Service interface {
	RegisterDevice(ctx context.Context, userID string, in RegisterDeviceInput) error
	UnregisterDevice(ctx context.Context, userID, token string) error
	NotifyBillCreated(ctx context.Context, rtID, billID string, recipientUserIDs []string)
	NotifyPaymentSubmitted(ctx context.Context, rtID, billID, paymentID string)
	NotifyPaymentVerified(ctx context.Context, rtID, billID, paymentID, submitterUserID, status, rejectionReason string)
	NotifyWasteTruckArrival(ctx context.Context, rtID, senderUserID, message string, imageURL ...*string)
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
	GetNotificationDetail(ctx context.Context, ac *auth.AuthContext, rtID, id string) (*NotificationLogEntry, error)
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
	GetSchedulerRecipientDetail(ctx context.Context, rtID, scheduleID, execDate string) ([]*NotificationSchedulerRecipientDetail, error)
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

		s.sendPushToTokens(bgCtx, tokens, title, body, payloadMap)
	}()
}

// NotifyPaymentVerified notifies the payment submitter and, if required by business rules, also notifies authorized verifiers.
// When submitterUserID is empty, only the verifier group is notified (no submitter path).
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
        }

        // Submitter path: only when submitterUserID is non-empty
        if submitterUserID != "" {
            // Deduplication for the submitter
            submitterDedupKey := fmt.Sprintf("payment_verified:%s:%s", paymentID, status)
            okSubmitter, dedupErr := s.repo.CheckAndRecordDedup(bgCtx, rtID, submitterUserID, EventPaymentVerified, submitterDedupKey, title, body, payload.ToMap())
            if dedupErr != nil {
                slog.Warn("payment verified dedup check failed for submitter", "payment_id", paymentID, "user_id", submitterUserID, "error", dedupErr)
            }

            var submitterTokens []string
            if dedupErr == nil && okSubmitter {
                var err error
                submitterTokens, err = s.repo.GetTokensForUser(bgCtx, submitterUserID)
                if err != nil {
                    slog.Error("failed to get tokens for payment submitter", "user_id", submitterUserID, "error", err)
                } else if len(submitterTokens) > 0 {
                    s.sendPushToTokens(bgCtx, submitterTokens, title, body, payload.ToMap())
                }
            }

            // Notify other authorized verifiers
            verifierTokens, err := s.repo.GetTokensForPaymentVerifiers(bgCtx, rtID)
            if err != nil {
                slog.Error("failed to get verifier tokens for payment verified", "rt_id", rtID, "error", err)
            } else if len(verifierTokens) > 0 {
                verifierDedupKey := fmt.Sprintf("payment_verified_verifier:%s:%s", paymentID, status)
                okVerifier, verifierDedupErr := s.repo.CheckAndRecordDedup(bgCtx, rtID, "", EventPaymentVerified, verifierDedupKey, title, body, payload.ToMap())
                if verifierDedupErr != nil {
                    slog.Warn("verifier payment verified dedup check failed", "payment_id", paymentID, "error", verifierDedupErr)
                }
                if verifierDedupErr == nil && okVerifier {
                    // Remove any tokens already sent to submitter to avoid duplicate pushes
                    sent := make(map[string]struct{}, len(submitterTokens))
                    for _, t := range submitterTokens {
                        sent[t] = struct{}{}
                    }
                    var filtered []string
                    for _, vt := range verifierTokens {
                        if _, exists := sent[vt]; !exists {
                            filtered = append(filtered, vt)
                        }
                    }
                    if len(filtered) > 0 {
                        s.sendPushToTokens(bgCtx, filtered, title, body, payload.ToMap())
                    }
                }
            }
        } else {
            // No submitter — still notify authorized verifiers if they exist
            verifierTokens, err := s.repo.GetTokensForPaymentVerifiers(bgCtx, rtID)
            if err != nil {
                slog.Error("failed to get verifier tokens for payment verified", "rt_id", rtID, "error", err)
            } else if len(verifierTokens) > 0 {
                verifierDedupKey := fmt.Sprintf("payment_verified_verifier:%s:%s", paymentID, status)
                okVerifier, verifierDedupErr := s.repo.CheckAndRecordDedup(bgCtx, rtID, "", EventPaymentVerified, verifierDedupKey, title, body, payload.ToMap())
                if verifierDedupErr != nil {
                    slog.Warn("verifier payment verified dedup check failed", "payment_id", paymentID, "error", verifierDedupErr)
                }
                if verifierDedupErr == nil && okVerifier {
                    s.sendPushToTokens(bgCtx, verifierTokens, title, body, payload.ToMap())
                }
            }
        }
    }()
}

// NotifyWasteTruckArrival dispatches manual notification to all warga in the sender's RT.
func (s *service) NotifyWasteTruckArrival(ctx context.Context, rtID, senderUserID, customMessage string, imageURL ...*string) {
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
		if len(imageURL) > 0 && imageURL[0] != nil && strings.TrimSpace(*imageURL[0]) != "" {
			finalImg = imageURL[0]
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

		// Cooldown deduplication: one notification per 30 minutes window per RT
		timeSlot := time.Now().Format("2006-01-02-15")
		if time.Now().Minute() >= 30 {
			timeSlot += "-30"
		} else {
			timeSlot += "-00"
		}
		dedupKey := fmt.Sprintf("waste_truck:%s:%s", rtID, timeSlot)

		payloadMap := payload.ToMap()
		ok, err := s.repo.CheckAndRecordDedup(bgCtx, rtID, senderUserID, EventWasteTruckArrival, dedupKey, title, body, payloadMap)
		if err != nil {
			slog.Warn("waste truck arrival dedup check failed", "error", err)
		}
		if !ok {
			slog.Info("waste truck notification skipped due to cooldown dedup", "rt_id", rtID)
			return
		}
		if notifID := payloadMap["notification_id"]; notifID != "" {
			payloadMap["deep_link"] = "/home/notifikasi/detail/" + notifID
		}

		tokens, err := s.repo.GetTokensForRT(bgCtx, rtID)
		if err != nil {
			slog.Error("failed to get RT tokens for waste truck arrival", "rt_id", rtID, "error", err)
			return
		}

		s.sendPushToTokens(bgCtx, tokens, title, body, payloadMap)
	}()
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

	tokens, err := s.repo.GetTokensForRT(ctx, rtID)
	if err != nil {
		return fmt.Errorf("gagal mendapatkan target perangkat RT: %w", err)
	}

	s.sendPushToTokens(ctx, tokens, title, body, payloadMap)
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
		RecipientsRule:   "Warga yang menempati KK terkait tagihan",
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
		AllowedVariables: []string{"bill_id", "period", "amount", "due_date", "deep_link"},
		IsConfigurable:   true,
		SupportsImage:    false,
		SupportsTest:     false,
	},
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
	return s.repo.ListTemplates(ctx, rtID, eventType, isActive)
}

func (s *service) GetTemplate(ctx context.Context, rtID, id string) (*NotificationTemplate, error) {
	return s.repo.GetTemplateByID(ctx, rtID, id)
}

func (s *service) CreateTemplate(ctx context.Context, rtID string, in CreateTemplateInput) (*NotificationTemplate, error) {
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
	return s.repo.ToggleTemplateActive(ctx, rtID, id)
}

func (s *service) DeleteTemplate(ctx context.Context, rtID, id string) error {
	return s.repo.DeleteTemplate(ctx, rtID, id)
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
	isOfficer := false
	if ac != nil {
		userID = ac.UserID
		isOfficer = isOfficerUser(ac)
	}
	return s.repo.ListLogs(ctx, rtID, userID, eventType, status, isOfficer, offset, pageSize)
}

func (s *service) MarkAsRead(ctx context.Context, ac *auth.AuthContext, rtID, id string) error {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return errors.New("unauthorized: missing auth context")
	}
	return s.repo.MarkAsRead(ctx, rtID, ac.UserID, id, isOfficerUser(ac))
}

func (s *service) MarkAllAsRead(ctx context.Context, ac *auth.AuthContext, rtID string) (int, error) {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return 0, errors.New("unauthorized: missing auth context")
	}
	return s.repo.MarkAllAsRead(ctx, rtID, ac.UserID, isOfficerUser(ac))
}

func (s *service) GetUnreadCount(ctx context.Context, ac *auth.AuthContext, rtID string) (int, error) {
	if ac == nil || strings.TrimSpace(ac.UserID) == "" {
		return 0, errors.New("unauthorized: missing auth context")
	}
	return s.repo.GetUnreadCount(ctx, rtID, ac.UserID, isOfficerUser(ac))
}

func (s *service) GetNotificationDetail(ctx context.Context, ac *auth.AuthContext, rtID, id string) (*NotificationLogEntry, error) {
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

	isOfficer := isOfficerUser(ac)

	// Test push notifications are strictly isolated to the Super Admin who triggered them.
	if strings.HasPrefix(entry.DedupKey, "test_push:") {
		if entry.UserID == nil || *entry.UserID != ac.UserID {
			return nil, ErrForbidden
		}
		_ = s.repo.MarkAsRead(ctx, rtID, ac.UserID, id, isOfficer)
		entry.IsRead = true
		now := time.Now()
		entry.ReadAt = &now
		return entry, nil
	}

	// Broadcast RT notifications (manual_push, waste_truck_arrival) are readable by any authenticated member in the same RT.
	if entry.EventType == EventManualPush || entry.EventType == EventWasteTruckArrival {
		_ = s.repo.MarkAsRead(ctx, rtID, ac.UserID, id, isOfficer)
		entry.IsRead = true
		now := time.Now()
		entry.ReadAt = &now
		return entry, nil
	}

	// User-targeted or financial notifications require matching user_id or RT officer/admin role.
	if entry.UserID != nil && *entry.UserID != "" {
		if *entry.UserID != ac.UserID && !isOfficer {
			return nil, ErrForbidden
		}
		_ = s.repo.MarkAsRead(ctx, rtID, ac.UserID, id, isOfficer)
		entry.IsRead = true
		now := time.Now()
		entry.ReadAt = &now
		return entry, nil
	}
	if !isOfficer {
		return nil, ErrForbidden
	}
	_ = s.repo.MarkAsRead(ctx, rtID, ac.UserID, id, isOfficer)
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
	return s.repo.ToggleScheduleActive(ctx, rtID, id, userID)
}

func (s *service) GetDueSchedules(ctx context.Context, now time.Time) ([]*NotificationSchedule, error) {
	return s.repo.GetDueSchedules(ctx, now)
}

// ExecuteSchedule runs a single schedule: resolves recipients based on the event
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
// Logs are written with the ACTUAL delivery status (SENT/FAILED), never
// "SENT" before a successful push. An empty FCM token list is treated as a
// delivery failure (not success). Partial failures are recorded per recipient
// and aggregated into the schedule-level status.
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
		claimed, err := s.repo.TryAdvisoryLockConn(conn, ctx, schedule.ID, execDate)
		if err != nil {
			return 0, fmt.Errorf("try advisory lock: %w", err)
		}
		if !claimed {
			slog.Info("schedule execution claimed by another instance", "schedule_id", schedule.ID, "exec_date", execDate)
			return 0, nil
		}
		defer s.repo.ReleaseAdvisoryLockConn(conn, ctx, schedule.ID, execDate)
	}

	// Defensive schedule-level dedup: already completed today (SENT/FAILED)?
	already, err := s.repo.IsScheduleExecutedToday(ctx, schedule.ID, execDate)
	if err != nil {
		return 0, fmt.Errorf("check executed today: %w", err)
	}
	if already {
		slog.Info("schedule already executed today", "schedule_id", schedule.ID, "exec_date", execDate)
		return 0, nil
	}

	bgCtx := ctx

	// Resolve the active template for this event type + RT (read-only).
	tmpl, err := s.repo.GetActiveTemplateForEvent(bgCtx, rtID, schedule.EventType)
	if err != nil {
		slog.Warn("no active template for reminder schedule", "rt_id", rtID, "event_type", schedule.EventType, "error", err)
		return 0, nil
	}
	if tmpl == nil || !tmpl.IsActive {
		return 0, nil
	}

	title := tmpl.Title
	body := tmpl.Body

	// Resolve recipients based on event type (read-only).
	var recipients []*IuranReminderRecipient
	if schedule.EventType == EventPaymentReminder {
		recs, err := s.repo.GetIuranReminderRecipients(bgCtx, rtID, now)
		if err != nil {
			slog.Error("failed to resolve iuran reminder recipients", "rt_id", rtID, "error", err)
			return 0, err
		}
		recipients = recs
	} else {
		return 0, fmt.Errorf("unsupported event type for scheduled execution: %s", schedule.EventType)
	}

	if len(recipients) == 0 {
		return 0, nil
	}

	type delivery struct {
		userID   string
		dedupKey string
		title    string
		body     string
		payload  map[string]string
		billID   string
		ok       bool // true if push delivered successfully
	}
	deliveries := make([]delivery, 0, len(recipients))

	sent := 0
	failed := 0
	for _, rec := range recipients {
		vars := map[string]string{
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
			// Deep link uses the bill ID (the navigable entity), not the
			// notification ID, which is only generated at log insert time.
			DeepLink: vars["deep_link"],
			SentAt:   now.UTC().Format(time.RFC3339Nano),
		}
		payloadMap := payload.ToMap()

		dedupKey := fmt.Sprintf("schedule:%s:%s:%s", schedule.ID, execDate, rec.UserID)

		// Empty token list => cannot deliver => failure, not success.
		tokens, err := s.repo.GetTokensForUser(bgCtx, rec.UserID)
		if err != nil {
			slog.Error("failed to get tokens for reminder recipient", "user_id", rec.UserID, "error", err)
			deliveries = append(deliveries, delivery{rec.UserID, dedupKey, t, b, payloadMap, rec.BillID, false})
			failed++
			continue
		}
		if len(tokens) == 0 {
			deliveries = append(deliveries, delivery{rec.UserID, dedupKey, t, b, payloadMap, rec.BillID, false})
			failed++
			continue
		}

		sentN, failedN := s.sendPushToTokens(bgCtx, tokens, t, b, payloadMap)
		sent += sentN
		failed += failedN
		deliveries = append(deliveries, delivery{rec.UserID, dedupKey, t, b, payloadMap, rec.BillID, sentN > 0})
	}

	// Record per-recipient delivery logs with ACTUAL status and update the
	// schedule's last execution info. The network call is already finished;
	// the advisory lock is still held. Each log insert is idempotent
	// (UNIQUE dedup_key, ON CONFLICT DO NOTHING), so retries are safe.
	for _, d := range deliveries {
		status := "SENT"
		if !d.ok {
			status = "FAILED"
		}
		if _, err := s.repo.RecordScheduleDelivery(ctx, rtID, d.userID, schedule.EventType, d.dedupKey, d.title, d.body, d.payload, status); err != nil {
			slog.Warn("failed to record schedule delivery log", "user_id", d.userID, "error", err)
		}
	}

	// Schedule-level status reflects real delivery outcomes.
	status := "success"
	total := len(recipients)
	if sent == 0 && total > 0 {
		status = "failed"
	} else if sent > 0 && sent < total {
		status = "partial"
	}
	if err := s.repo.UpdateScheduleExecuted(ctx, schedule.ID, rtID, now, status); err != nil {
		slog.Warn("failed to update schedule last executed", "schedule_id", schedule.ID, "error", err)
	}

	return sent, nil
}

func (s *service) ListSchedulerHistory(ctx context.Context, rtID string, page, pageSize int) ([]*NotificationSchedulerHistory, int, error) {
	return s.repo.ListSchedulerHistory(ctx, rtID, page, pageSize)
}

func (s *service) GetSchedulerRecipientDetail(ctx context.Context, rtID, scheduleID, execDate string) ([]*NotificationSchedulerRecipientDetail, error) {
	return s.repo.GetSchedulerRecipientDetail(ctx, rtID, scheduleID, execDate)
}
