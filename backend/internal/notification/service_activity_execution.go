package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// executeActivityReminder runs a one-shot scheduled reminder bound to an activity.
// It mirrors ExecuteSchedule's locking / dedup semantics for the activity path.
func (s *service) executeActivityReminder(ctx context.Context, schedule *NotificationSchedule, now time.Time) error {
	if schedule == nil || schedule.ActivityID == nil {
		return errors.New("activity schedule requires activity_id")
	}
	rtID := schedule.RTID

	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		loc = activityJakartaLoc()
	}
	execDate := now.In(loc).Format("2006-01-02")
	if schedule.ExecutionAt != nil {
		execDate = schedule.ExecutionAt.In(loc).Format("2006-01-02")
	}
	configHash := ComputeScheduleConfigHashFromSchedule(schedule)

	// Same advisory lock pattern as the recurring path.
	var conn *sql.Conn
	conn, err = s.repo.AcquireConn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	if conn != nil {
		defer conn.Close()
		lockKey := fmt.Sprintf("%s:c_%s", execDate, configHash)
		claimed, lerr := s.repo.TryAdvisoryLockConn(conn, ctx, schedule.ID, lockKey)
		if lerr != nil {
			return fmt.Errorf("try advisory lock: %w", lerr)
		}
		if !claimed {
			slog.Info("activity reminder already claimed by another instance", "schedule_id", schedule.ID, "exec_date", execDate)
			return nil
		}
		defer s.repo.ReleaseAdvisoryLockConn(conn, ctx, schedule.ID, lockKey)
	}

	eventType := schedule.EventType
	if eventType == "" {
		eventType = EventActivityReminder
	}

	// Hard-fail template resolution (same as the recurring path).
	var tmpl *NotificationTemplate
	if schedule.TemplateID != "" {
		t, terr := s.repo.GetTemplateByID(ctx, rtID, schedule.TemplateID)
		if terr == nil && t != nil && t.IsActive {
			tmpl = t
		}
	}
	if tmpl == nil {
		t, terr := s.repo.GetActiveTemplateForEvent(ctx, rtID, eventType)
		if terr == nil && t != nil && t.IsActive {
			tmpl = t
		}
	}
	if tmpl == nil {
		reason := "Template not found or inactive"
		status := "skipped"
		_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, status, &reason, &ScheduleExecutionSummary{FailureReason: reason}, configHash)
		_ = s.repo.DeactivateSchedule(ctx, rtID, schedule.ID)
		return fmt.Errorf("active template not found or inactive for schedule %s (event_type %s)", schedule.ID, eventType)
	}

	// Pull the activity; an activity that is cancelled/completed/terminated must NOT get a reminder.
	activity, aerr := s.activityInfoByID(ctx, rtID, *schedule.ActivityID)
	if aerr != nil {
		reason := fmt.Sprintf("activity tidak dapat dimuat: %v", aerr)
		status := "skipped"
		_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, status, &reason, &ScheduleExecutionSummary{FailureReason: reason}, configHash)
		_ = s.repo.DeactivateSchedule(ctx, rtID, schedule.ID)
		return fmt.Errorf("activity %s unreadable: %w", *schedule.ActivityID, aerr)
	}
	if activity.IsActivityCancelled() || activity.IsActivityComplete(now) {
		reason := "Kegiatan sudah dibatalkan atau selesai — pengingat tidak dikirim"
		status := "skipped"
		_ = s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, status, &reason, &ScheduleExecutionSummary{FailureReason: reason}, configHash)
		_ = s.repo.DeactivateSchedule(ctx, rtID, schedule.ID)
		slog.Info("activity reminder skipped because activity is terminated", "schedule_id", schedule.ID, "activity_id", activity.ID)
		return nil
	}

	broadcastID := fmt.Sprintf("activity_reminder:%s:%s:%s", schedule.ID, execDate, configHash)

	// Idempotency backstop — this exact schedule + date + config must deliver at most once.
	already, ierr := s.repo.IsScheduleExecutedToday(ctx, schedule.ID, execDate, configHash)
	if ierr == nil && already {
		slog.Info("activity reminder already executed today", "schedule_id", schedule.ID, "exec_date", execDate)
		// Deactivate one-shot schedule so it does not keep retrying same-day.
		_ = s.repo.DeactivateSchedule(ctx, rtID, schedule.ID)
		return nil
	}

	// Fan-out delivery per recipient (with per-recipient personalization).
	sent, failed, noDevice := 0, 0, 0
	recipients, rerr := s.repo.GetRTBroadcastRecipients(ctx, rtID)
	if rerr != nil {
		_ = rerr
	}
	for _, recipient := range recipients {
		userName := trimUserName(recipient.UserFullName)
		title, body, vars := buildActivityDispatch(ctx, s, rtID, eventType, *activity, userName)

		payload := NotificationPayload{
			Type:     eventType,
			RTID:     rtID,
			Title:    title,
			Body:     body,
			DeepLink: vars["deep_link"],
			SentAt:   now.UTC().Format(time.RFC3339Nano),
		}
		payloadMap := payload.ToMap()
		payloadMap["activity_id"] = activity.ID
		payloadMap["schedule_id"] = schedule.ID

		sentN, failedN := 0, 0
		if len(recipient.Tokens) > 0 {
			sentN, failedN = s.sendPushToTokens(ctx, recipient.Tokens, title, body, payloadMap)
		}
		switch {
		case len(recipient.Tokens) == 0:
			noDevice++
		case sentN > 0:
			sent += sentN
		default:
			failed += failedN
		}

		status := recipientDeliveryStatus(sentN, failedN, len(recipient.Tokens))
		_ = s.repo.RecordBroadcastRecipientDelivery(
			ctx, rtID, "", recipient.UserID, eventType,
			broadcastID, title, body, payloadMap, status,
		)
	}

	total := len(recipients)
	summary := &ScheduleExecutionSummary{
		TotalRecipients: total,
		SentCount:       sent,
		FailedCount:     failed,
		NoDeviceCount:   noDevice,
	}
	finalStatus := "success"
	if total == 0 {
		finalStatus = "no_recipients"
	} else if sent == 0 && failed > 0 {
		finalStatus = "failed"
	} else if sent > 0 && failed > 0 {
		finalStatus = "partial"
	}

	if err := s.repo.UpdateScheduleExecutedDetailed(ctx, schedule.ID, rtID, now, finalStatus, nil, summary, configHash); err != nil {
		return fmt.Errorf("record activity reminder execution: %w", err)
	}
	// One-shot safety: deactivate after the first successful delivery so it
	// cannot be sent again even if execution_at is moved forward manually.
	if err := s.repo.DeactivateSchedule(ctx, rtID, schedule.ID); err != nil {
		slog.Warn("failed to deactivate one-shot activity schedule after execution", "schedule_id", schedule.ID, "error", err)
	}
	return nil
}

func trimUserName(name string) string {
	n := len(name)
	for n > 0 && (name[n-1] == ' ' || name[n-1] == '\t' || name[n-1] == '\n') {
		n--
	}
	if n == 0 {
		return "Warga"
	}
	return name[:n]
}

// activityInfoByID loads a minimal view of an activity for lifecycle checks and payload rendering.
func (s *service) activityInfoByID(ctx context.Context, rtID, activityID string) (*ActivityPayload, error) {
	if f, ok := s.repo.(interface {
		fetchActivityFull(ctx context.Context, rtID, activityID string) (*ActivityPayload, error)
	}); ok {
		return f.fetchActivityFull(ctx, rtID, activityID)
	}
	// Fallback — interface-free path (e.g. test mocks): read via ActivityInfoFetch.
	return s.repo.ActivityInfoFetch(ctx, rtID, activityID)
}
