package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"social-finance/internal/finance"
)

// ActivityPayload is the notification-relevant view of an activity (kegiatan).
// It is populated by the community package when firing immediate events, or by
// this package when a one-shot reminder schedule executes.
type ActivityPayload struct {
	ID                string
	RTID              string
	Title             string
	Description       *string
	Location          *string
	Organizer         *string
	StartsAt          time.Time
	EndsAt            *time.Time
	Status            string // 'upcoming', 'ongoing', 'completed', 'cancelled' (dynamic from ends_at)
	PublicationStatus string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IsActivityComplete reports whether the activity is effectively complete:
// explicit 'completed' status or (published) ends_at already in the past.
func (p ActivityPayload) IsActivityComplete(now time.Time) bool {
	if strings.EqualFold(p.Status, "completed") {
		return true
	}
	if p.EndsAt != nil && !p.EndsAt.After(now) {
		return true
	}
	return false
}

func (p ActivityPayload) IsActivityCancelled() bool {
	return strings.EqualFold(p.Status, "cancelled")
}

func (p ActivityPayload) IsActivityTerminated(now time.Time) bool {
	return p.IsActivityComplete(now) || p.IsActivityCancelled()
}

// JakartaLocation for date/time formatting of activity notifications.
func activityJakartaLoc() *time.Location { return finance.JakartaLocation() }

func dateJakarta(t time.Time) string { return t.In(activityJakartaLoc()).Format("02 January 2006") }
func timeJakarta(t time.Time) string { return t.In(activityJakartaLoc()).Format("15:04") }
func derefOr(s *string, fallback string) string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return fallback
	}
	return strings.TrimSpace(*s)
}

func activityDeepLink(activityID string) string { return "/kegiatan/" + activityID }

func activityToMap(p ActivityPayload) map[string]string {
	return map[string]string{
		"nama_kegiatan":      p.Title,
		"deskripsi_kegiatan": derefOr(p.Description, "—"),
		"tanggal_kegiatan":   dateJakarta(p.StartsAt),
		"jam_kegiatan":       timeJakarta(p.StartsAt),
		"lokasi_kegiatan":    derefOr(p.Location, "—"),
		"penyelenggara":      derefOr(p.Organizer, "—"),
		"deep_link":          activityDeepLink(p.ID),
	}
}

// buildActivityDispatch resolves title/body/payload for a broadcast of an activity event.
// If the RT has a customizable active template for the event, it is used; otherwise the
// system default title/body from SystemNotificationTypes is used.
func buildActivityDispatch(ctx context.Context, s *service, rtID, eventType string, p ActivityPayload, userName string) (title, body string, vars map[string]string) {
	tmpl, err := s.repo.GetActiveTemplateForEvent(ctx, rtID, eventType)
	if err == nil && tmpl != nil && tmpl.IsActive {
		title = tmpl.Title
		body = tmpl.Body
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		switch st := findSystemType(eventType); {
		case st != nil:
			if strings.TrimSpace(title) == "" {
				title = st.DefaultTitle
			}
			if strings.TrimSpace(body) == "" {
				body = st.DefaultBody
			}
		default:
			if strings.TrimSpace(title) == "" {
				title = nearestTitleFallback(eventType, p.Title)
			}
			if strings.TrimSpace(body) == "" {
				body = nearestBodyFallback(eventType, p.Title)
			}
		}
	}
	vars = activityToMap(p)
	if strings.TrimSpace(userName) != "" {
		vars["nama_warga"] = strings.TrimSpace(userName)
	} else {
		vars["nama_warga"] = "Warga"
	}
	title = interpolate(title, vars)
	body = interpolate(body, vars)
	return title, body, vars
}

func nearestTitleFallback(eventType, titleHint string) string {
	switch eventType {
	case EventActivityCreated:
		return "Kegiatan Baru: " + titleHint
	case EventActivityUpdated:
		return "Kegiatan Diperbarui: " + titleHint
	case EventActivityReminder:
		return "Pengingat Kegiatan: " + titleHint
	case EventActivityCancelled:
		return "Kegiatan Dibatalkan: " + titleHint
	case EventActivityCompleted:
		return "Kegiatan Selesai: " + titleHint
	default:
		return "Pemberitahuan Kegiatan RT"
	}
}

func nearestBodyFallback(eventType, titleHint string) string {
	switch eventType {
	case EventActivityCreated:
		return "Agenda kegiatan baru telah diterbitkan. Silakan cek menu Kegiatan untuk rinciannya."
	case EventActivityUpdated:
		return "Informasi kegiatan telah diperbarui. Silakan cek menu Kegiatan untuk rincian terbaru."
	case EventActivityReminder:
		return "Kegiatan ini akan segera dilaksanakan. Silakan cek menu Kegiatan untuk rinciannya."
	case EventActivityCancelled:
		return "Mohon maaf, kegiatan ini telah dibatalkan oleh Pengurus RT."
	case EventActivityCompleted:
		return "Kegiatan ini telah selesai dilaksanakan. Terima kasih atas partisipasi warga."
	default:
		return "Pemberitahuan kegiatan RT."
	}
}

// BroadcastActivityEventParams is the parameter set for BroadcastActivityEvent.
// RecipientsTemplate is the broadcast ID prefix used in dedup keys.
type BroadcastActivityEventParams struct {
	RTID            string
	ActivityID      string
	EventType       string
	SenderUserID    string // blank means system/automatic
	ScheduleID      string // blank means immediate dispatch
	ExecutionDate   string // blank means immediate dispatch
	DedupSalt       string
	LegacyBroadcast string // ignored if empty; kept for forward compatibility
}

// broadcastActivityEvent fans out an activity event to all active members of
// the RT, rendering the template (if any) per recipient so {{nama_warga}} is
// personalized. It is a broadcast — all members receive the same event.
func (s *service) broadcastActivityEvent(ctx context.Context, p ActivityPayload, eventType, senderUserID, broadcastID string) error {
	rtID := p.RTID
	if strings.TrimSpace(rtID) == "" {
		return errors.New("rt_id is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		// Defensive default — not required but harmless for caller convenience.
		_ = ctx
	}

	recipients, err := s.repo.GetRTBroadcastRecipients(ctx, rtID)
	if err != nil {
		return fmt.Errorf("gagal mendapatkan target perangkat RT: %w", err)
	}

	for _, recipient := range recipients {
		userName := strings.TrimSpace(recipient.UserFullName)
		title, body, vars := buildActivityDispatch(ctx, s, rtID, eventType, p, userName)

		payload := NotificationPayload{
			Type:     eventType,
			RTID:     rtID,
			Title:    title,
			Body:     body,
			DeepLink: vars["deep_link"],
			SentAt:   time.Now().UTC().Format(time.RFC3339Nano),
		}
		payloadMap := payload.ToMap()
		payloadMap["activity_id"] = p.ID
		payloadMap["activity_type"] = eventType
		payloadMap["tanggal_kegiatan"] = vars["tanggal_kegiatan"]
		payloadMap["jam_kegiatan"] = vars["jam_kegiatan"]
		payloadMap["lokasi_kegiatan"] = vars["lokasi_kegiatan"]

		sentN, failedN := 0, 0
		if len(recipient.Tokens) > 0 {
			sentN, failedN = s.sendPushToTokens(ctx, recipient.Tokens, title, body, payloadMap)
		}
		status := recipientDeliveryStatus(sentN, failedN, len(recipient.Tokens))
		if err := s.repo.RecordBroadcastRecipientDelivery(
			ctx, rtID, senderUserID, recipient.UserID, eventType,
			broadcastID, title, body, payloadMap, status,
		); err != nil {
			// Log only — deliveries must stay best-effort fan-out.
			slog.Warn("failed to record activity broadcast delivery",
				"rt_id", rtID, "activity_id", p.ID, "event_type", eventType,
				"recipient_user_id", recipient.UserID, "error", err)
		}
	}
	return nil
}

// NotifyActivityEvent fires an immediate activity lifecycle event to all RT members.
// The recipientBroadcastID is a freshly generated unique id; dedup is not enforced
// for immediate events (fresh nanosecond) because they cannot be retried by callers.
func (s *service) NotifyActivityEvent(ctx context.Context, p ActivityPayload, eventType string) error {
	if strings.TrimSpace(eventType) == "" {
		return errors.New("event type is required")
	}
	if !strings.HasPrefix(eventType, "activity_") {
		return fmt.Errorf("unsupported activity event %q", eventType)
	}
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.RTID) == "" {
		return errors.New("rt_id and activity id are required")
	}
	// Idempotency backstop for the sweep:
	broadcastID := fmt.Sprintf("%s:%d", eventType, time.Now().UnixNano())
	if err := s.broadcastActivityEvent(ctx, p, eventType, "", broadcastID); err != nil {
		return err
	}
	return nil
}

// CancelActivityPendingSchedules deactivates all pending one-shot schedules
// attached to a specific activity.
func (s *service) CancelActivityPendingSchedules(ctx context.Context, rtID, activityID string) error {
	_, err := s.repo.CancelSchedulesForActivity(ctx, rtID, activityID)
	return err
}
