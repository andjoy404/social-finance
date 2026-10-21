package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Service defines notification operations.
type Service interface {
	RegisterDevice(ctx context.Context, userID string, in RegisterDeviceInput) error
	UnregisterDevice(ctx context.Context, userID, token string) error
	NotifyBillCreated(ctx context.Context, rtID, billID string, recipientUserIDs []string)
	NotifyPaymentSubmitted(ctx context.Context, rtID, billID, paymentID string)
	NotifyPaymentVerified(ctx context.Context, rtID, billID, paymentID, submitterUserID, status, rejectionReason string)
	NotifyWasteTruckArrival(ctx context.Context, rtID, senderUserID, message string)
	SendSelfTest(ctx context.Context, userID string) (int, error)
}

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
		payload := NotificationPayload{
			Type:     EventBillCreated,
			RTID:     rtID,
			BillID:   billID,
			DeepLink: fmt.Sprintf("/home/iuran/detail/%s", billID),
			SentAt:   time.Now().UTC().Format(time.RFC3339Nano),
		}

		for _, uid := range targets {
			dedupKey := fmt.Sprintf("bill:%s:%s", billID, uid)
			ok, err := s.repo.CheckAndRecordDedup(bgCtx, rtID, uid, EventBillCreated, dedupKey, title, body, payload.ToMap())
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

			s.sendPushToTokens(bgCtx, tokens, title, body, payload.ToMap())
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
		payload := NotificationPayload{
			Type:      EventPaymentSubmitted,
			RTID:      rtID,
			BillID:    billID,
			PaymentID: paymentID,
			Status:    "PENDING",
			DeepLink:  fmt.Sprintf("/home/iuran/detail/%s", billID),
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
		ok, err := s.repo.CheckAndRecordDedup(bgCtx, rtID, "", EventPaymentSubmitted, dedupKey, title, body, payload.ToMap())
		if err != nil {
			slog.Warn("payment submitted dedup check failed", "payment_id", paymentID, "error", err)
		}
		if !ok {
			return // duplicate event
		}

		s.sendPushToTokens(bgCtx, tokens, title, body, payload.ToMap())
	}()
}

// NotifyPaymentVerified notifies strictly the resident who submitted the payment,
// regardless of whether the submitter also holds a management role — an officer
// paying their own household's bill still receives the verification result.
// Officer notes are NEVER included.
func (s *service) NotifyPaymentVerified(ctx context.Context, rtID, billID, paymentID, submitterUserID, status, rejectionReason string) {
	if submitterUserID == "" {
		return
	}

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

		payload := NotificationPayload{
			Type:      EventPaymentVerified,
			RTID:      rtID,
			BillID:    billID,
			PaymentID: paymentID,
			Status:    status,
			DeepLink:  fmt.Sprintf("/home/iuran/detail/%s?payment_id=%s", billID, paymentID),
		}

		dedupKey := fmt.Sprintf("payment_verified:%s:%s", paymentID, status)

		ok, err := s.repo.CheckAndRecordDedup(bgCtx, rtID, submitterUserID, EventPaymentVerified, dedupKey, title, body, payload.ToMap())
		if err != nil {
			slog.Warn("payment verified dedup check failed", "payment_id", paymentID, "error", err)
		}
		if !ok {
			return // already notified
		}

		tokens, err := s.repo.GetTokensForUser(bgCtx, submitterUserID)
		if err != nil {
			slog.Error("failed to get tokens for payment submitter", "user_id", submitterUserID, "error", err)
			return
		}

		s.sendPushToTokens(bgCtx, tokens, title, body, payload.ToMap())
	}()
}

// NotifyWasteTruckArrival dispatches manual notification to all warga in the sender's RT.
func (s *service) NotifyWasteTruckArrival(ctx context.Context, rtID, senderUserID, customMessage string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		title := "Mobil Pengangkut Sampah Sudah Tiba"
		body := "Mobil pengangkut sampah sudah sampai di depan gerbang RT. Silakan keluarkan sampah Anda."
		if strings.TrimSpace(customMessage) != "" {
			body = strings.TrimSpace(customMessage)
		}

		payload := NotificationPayload{
			Type:     EventWasteTruckArrival,
			RTID:     rtID,
			DeepLink: "/iuran",
		}

		// Cooldown deduplication: one notification per 30 minutes window per RT
		timeSlot := time.Now().Format("2006-01-02-15")
		if time.Now().Minute() >= 30 {
			timeSlot += "-30"
		} else {
			timeSlot += "-00"
		}
		dedupKey := fmt.Sprintf("waste_truck:%s:%s", rtID, timeSlot)

		ok, err := s.repo.CheckAndRecordDedup(bgCtx, rtID, senderUserID, EventWasteTruckArrival, dedupKey, title, body, payload.ToMap())
		if err != nil {
			slog.Warn("waste truck arrival dedup check failed", "error", err)
		}
		if !ok {
			slog.Info("waste truck notification skipped due to cooldown dedup", "rt_id", rtID)
			return
		}

		tokens, err := s.repo.GetTokensForRT(bgCtx, rtID)
		if err != nil {
			slog.Error("failed to get RT tokens for waste truck arrival", "rt_id", rtID, "error", err)
			return
		}

		s.sendPushToTokens(bgCtx, tokens, title, body, payload.ToMap())
	}()
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
		"deep_link": "/home",
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
