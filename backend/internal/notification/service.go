package notification

import (
	"context"
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
	ListLogs(ctx context.Context, rtID, eventType string, page, pageSize int) ([]NotificationLogEntry, int, error)
	GetNotificationDetail(ctx context.Context, ac *auth.AuthContext, rtID, id string) (*NotificationLogEntry, error)
	ResolveTenantRT(ctx context.Context, ac *auth.AuthContext, reqHeaderRT, reqQueryRT string) (string, error)
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

func (s *service) sendPushToTokens(ctx context.Context, tokens []string, title, body string, payload map[string]string) {
	var wg sync.WaitGroup
	for _, tok := range tokens {
		tok := tok
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.provider.Send(ctx, tok, title, body, payload); err != nil {
				slog.Warn("push send failed", "token", maskToken(tok), "error", err)
				if isInvalidTokenError(err) {
					if dErr := s.repo.DeactivateToken(ctx, tok); dErr != nil {
						slog.Warn("failed to deactivate invalid token", "token", maskToken(tok), "error", dErr)
					} else {
						slog.Info("deactivated invalid/unregistered token", "token", maskToken(tok))
					}
				}
			}
		}()
	}
	wg.Wait()
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

func (s *service) ListLogs(ctx context.Context, rtID, eventType string, page, pageSize int) ([]NotificationLogEntry, int, error) {
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
	return s.repo.ListLogs(ctx, rtID, eventType, offset, pageSize)
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

	// Test push notifications are strictly isolated to the Super Admin who triggered them.
	if strings.HasPrefix(entry.DedupKey, "test_push:") {
		if entry.UserID == nil || *entry.UserID != ac.UserID {
			return nil, ErrForbidden
		}
		return entry, nil
	}

	// Broadcast RT notifications (manual_push, waste_truck_arrival) are readable by any authenticated member in the same RT.
	if entry.EventType == EventManualPush || entry.EventType == EventWasteTruckArrival {
		return entry, nil
	}

	// User-targeted or financial notifications require matching user_id or RT officer/admin role.
	isOfficer := ac.SystemRole == auth.SystemRoleSuperAdmin ||
		ac.Jabatan == auth.JabatanKetua ||
		ac.Jabatan == auth.JabatanBendahara ||
		ac.TenantRole == auth.RoleBendahara ||
		ac.TenantRole == auth.RolePengurus
	if entry.UserID != nil && *entry.UserID != "" {
		if *entry.UserID != ac.UserID && !isOfficer {
			return nil, ErrForbidden
		}
		return entry, nil
	}
	if !isOfficer {
		return nil, ErrForbidden
	}
	return entry, nil
}

func (s *service) ResolveTenantRT(ctx context.Context, ac *auth.AuthContext, reqHeaderRT, reqQueryRT string) (string, error) {
	if ac == nil {
		return "", errors.New("unauthorized: missing auth context")
	}
	if ac.RTID != "" {
		return ac.RTID, nil
	}
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		reqRT := strings.TrimSpace(reqHeaderRT)
		if reqRT == "" {
			reqRT = strings.TrimSpace(reqQueryRT)
		}
		if reqRT != "" {
			return "", fmt.Errorf("requested tenant rt (%s) is invalid or inactive", reqRT)
		}
		return s.repo.GetDefaultRTID(ctx)
	}
	return "", errors.New("missing tenant context")
}
