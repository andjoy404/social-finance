package notification_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"social-finance/internal/finance"
	"social-finance/internal/notification"
)

// schedulerRegressionRepo is a mock repository implementing Repository and OverdueReminderRepository
// for scheduler regression tests covering all 17 required scenarios.
type schedulerRegressionRepo struct {
	templates         map[string]*notification.NotificationTemplate
	schedules         map[string]*notification.NotificationSchedule
	devices           map[string][]string // userID -> tokens
	logs              []notification.NotificationLogEntry
	iuranRecipients   map[string][]*notification.IuranReminderRecipient // rtID -> recipients
	evalBills         map[string][]finance.BillEvaluationInput          // rtID -> bills
	executedSchedules map[string]struct {
		at      time.Time
		status  string
		errDesc string
		summary *notification.ScheduleExecutionSummary
	}
	configExecutions map[string]struct {
		at      time.Time
		status  string
		errDesc string
		summary *notification.ScheduleExecutionSummary
	}
}

func newSchedulerRegressionRepo() *schedulerRegressionRepo {
	return &schedulerRegressionRepo{
		templates:       make(map[string]*notification.NotificationTemplate),
		schedules:       make(map[string]*notification.NotificationSchedule),
		devices:         make(map[string][]string),
		logs:            make([]notification.NotificationLogEntry, 0),
		iuranRecipients: make(map[string][]*notification.IuranReminderRecipient),
		evalBills:       make(map[string][]finance.BillEvaluationInput),
		executedSchedules: make(map[string]struct {
			at      time.Time
			status  string
			errDesc string
			summary *notification.ScheduleExecutionSummary
		}),
		configExecutions: make(map[string]struct {
			at      time.Time
			status  string
			errDesc string
			summary *notification.ScheduleExecutionSummary
		}),
	}
}

func (r *schedulerRegressionRepo) RegisterDevice(ctx context.Context, userID, token, platform string, deviceID *string) error {
	r.devices[userID] = append(r.devices[userID], token)
	return nil
}

func (r *schedulerRegressionRepo) UnregisterDevice(ctx context.Context, userID, token string) error {
	return nil
}

func (r *schedulerRegressionRepo) DeactivateToken(ctx context.Context, token string) error {
	for u, toks := range r.devices {
		var kept []string
		for _, t := range toks {
			if t != token {
				kept = append(kept, t)
			}
		}
		r.devices[u] = kept
	}
	return nil
}

func (r *schedulerRegressionRepo) GetTokensForUser(ctx context.Context, userID string) ([]string, error) {
	return r.devices[userID], nil
}

func (r *schedulerRegressionRepo) GetTokensForRT(ctx context.Context, rtID string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetTokensForPaymentVerifiers(ctx context.Context, rtID string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetPaymentVerifierUserIDs(ctx context.Context, rtID string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetTokensForWasteTruckArrival(ctx context.Context, rtID string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetWasteTruckArrivalRecipients(ctx context.Context, rtID string) ([]notification.WasteTruckArrivalRecipient, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetRTBroadcastRecipients(ctx context.Context, rtID string) ([]notification.WasteTruckArrivalRecipient, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetRecipientUserIDsForBill(ctx context.Context, rtID, billID string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetTokensForRTRoles(ctx context.Context, rtID string, roles []string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) CheckAndRecordDedup(ctx context.Context, rtID, userID, eventType, dedupKey, title, body string, payload map[string]string) (bool, error) {
	return true, nil
}

func (r *schedulerRegressionRepo) RecordBroadcastRecipientDelivery(ctx context.Context, rtID, senderUserID, recipientUserID, eventType, broadcastID, title, body string, payload map[string]string, status string) error {
	return nil
}

func (r *schedulerRegressionRepo) RecordWasteTruckDispatch(ctx context.Context, rtID, senderUserID, dedupKey, title, body string, payload map[string]string) (string, bool, error) {
	return "", true, nil
}

func (r *schedulerRegressionRepo) UpdateWasteTruckDispatch(ctx context.Context, rtID, senderUserID, id, status, failureReason string) error {
	return nil
}

func (r *schedulerRegressionRepo) ListWasteTruckDispatches(ctx context.Context, rtID, senderUserID string, offset, limit int) ([]notification.WasteTruckDispatchEntry, int, error) {
	return nil, 0, nil
}

func (r *schedulerRegressionRepo) ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetTemplateByID(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	for _, t := range r.templates {
		if t.ID == id && t.RTID == rtID {
			return t, nil
		}
	}
	return nil, nil
}

func (r *schedulerRegressionRepo) GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*notification.NotificationTemplate, error) {
	for _, t := range r.templates {
		if t.RTID == rtID && t.EventType == eventType && t.IsActive {
			return t, nil
		}
	}
	return nil, nil
}

func (r *schedulerRegressionRepo) CreateTemplate(ctx context.Context, rtID string, in notification.CreateTemplateInput, allowedVars []string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) UpdateTemplate(ctx context.Context, rtID, id string, in notification.UpdateTemplateInput) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) ToggleTemplateActive(ctx context.Context, rtID, id string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) DeleteTemplate(ctx context.Context, rtID, id string) error {
	return nil
}

func (r *schedulerRegressionRepo) ListRWTemplates(ctx context.Context, rwID, eventType string, isActive *bool) ([]notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetRWTemplateByID(ctx context.Context, rwID, id string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) CreateRWTemplate(ctx context.Context, rwID string, in notification.CreateTemplateInput, allowedVars []string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) UpdateRWTemplate(ctx context.Context, rwID, id string, in notification.UpdateTemplateInput) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) ToggleRWTemplateActive(ctx context.Context, rwID, id string) (*notification.NotificationTemplate, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) DeleteRWTemplate(ctx context.Context, rwID, id string) error {
	return nil
}

func (r *schedulerRegressionRepo) CreateSchedule(ctx context.Context, rtID, userID string, in notification.CreateScheduleInput) (*notification.NotificationSchedule, error) {
	s := &notification.NotificationSchedule{
		ID:         "sched-" + in.Name,
		RTID:       rtID,
		EventType:  in.EventType,
		TemplateID: in.TemplateID,
		Name:       in.Name,
		IsActive:   true,
		SendDays:   in.SendDays,
		SendHour:   in.SendHour,
		SendMinute: in.SendMinute,
		Timezone:   in.Timezone,
	}
	r.schedules[s.ID] = s
	return s, nil
}

func (r *schedulerRegressionRepo) GetSchedule(ctx context.Context, rtID, id string) (*notification.NotificationSchedule, error) {
	s, ok := r.schedules[id]
	if !ok || s.RTID != rtID {
		return nil, nil
	}
	return s, nil
}

func (r *schedulerRegressionRepo) ListSchedules(ctx context.Context, rtID, eventType string, isActive *bool) ([]*notification.NotificationSchedule, error) {
	var list []*notification.NotificationSchedule
	for _, s := range r.schedules {
		if s.RTID == rtID {
			list = append(list, s)
		}
	}
	return list, nil
}

func (r *schedulerRegressionRepo) UpdateSchedule(ctx context.Context, rtID, id, userID string, in notification.UpdateScheduleInput) (*notification.NotificationSchedule, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) DeleteSchedule(ctx context.Context, rtID, id string) error {
	delete(r.schedules, id)
	return nil
}

func (r *schedulerRegressionRepo) ToggleScheduleActive(ctx context.Context, rtID, id, userID string) (*notification.NotificationSchedule, error) {
	s, ok := r.schedules[id]
	if !ok || s.RTID != rtID {
		return nil, sql.ErrNoRows
	}
	s.IsActive = !s.IsActive
	return s, nil
}

func (r *schedulerRegressionRepo) GetDueSchedules(ctx context.Context, now time.Time) ([]*notification.NotificationSchedule, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetIuranReminderRecipients(ctx context.Context, rtID string, asOf time.Time) ([]*notification.IuranReminderRecipient, error) {
	return r.iuranRecipients[rtID], nil
}

func (r *schedulerRegressionRepo) UpdateScheduleExecuted(ctx context.Context, scheduleID, rtID string, at time.Time, status string) error {
	return r.UpdateScheduleExecutedDetailed(ctx, scheduleID, rtID, at, status, nil, nil)
}

func (r *schedulerRegressionRepo) UpdateScheduleExecutedDetailed(ctx context.Context, scheduleID, rtID string, at time.Time, status string, errorDetail *string, summary *notification.ScheduleExecutionSummary, configHash ...string) error {
	s, ok := r.schedules[scheduleID]
	if !ok || s.RTID != rtID {
		return sql.ErrNoRows
	}
	s.LastExecutedAt = &at
	s.LastExecutedStatus = &status
	s.LastExecutedError = errorDetail
	s.LastExecutedSummary = summary
	s.UpdatedAt = at

	errStr := ""
	if errorDetail != nil {
		errStr = *errorDetail
	}
	r.executedSchedules[scheduleID] = struct {
		at      time.Time
		status  string
		errDesc string
		summary *notification.ScheduleExecutionSummary
	}{at: at, status: status, errDesc: errStr, summary: summary}

	cfg := ""
	if len(configHash) > 0 && configHash[0] != "" {
		cfg = configHash[0]
	} else {
		cfg = notification.ComputeScheduleConfigHashFromSchedule(s)
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		loc = time.FixedZone(s.Timezone, 7*3600)
	}
	execDate := at.In(loc).Format("2006-01-02")
	k := scheduleID + ":" + execDate + ":" + cfg
	r.configExecutions[k] = struct {
		at      time.Time
		status  string
		errDesc string
		summary *notification.ScheduleExecutionSummary
	}{at: at, status: status, errDesc: errStr, summary: summary}

	return nil
}

func (r *schedulerRegressionRepo) RecordScheduleExecution(ctx context.Context, scheduleID, rtID string, execDate time.Time, configHash, status string, errorDetail *string, summary *notification.ScheduleExecutionSummary) error {
	dateStr := execDate.Format("2006-01-02")
	k := scheduleID + ":" + dateStr + ":" + configHash
	errStr := ""
	if errorDetail != nil {
		errStr = *errorDetail
	}
	r.configExecutions[k] = struct {
		at      time.Time
		status  string
		errDesc string
		summary *notification.ScheduleExecutionSummary
	}{at: execDate, status: status, errDesc: errStr, summary: summary}
	return nil
}

func (r *schedulerRegressionRepo) IsScheduleExecutionDone(ctx context.Context, scheduleID, execDate, configHash string) (bool, error) {
	k := scheduleID + ":" + execDate + ":" + configHash
	if rec, ok := r.configExecutions[k]; ok && rec.status != "" {
		return true, nil
	}
	cfgPrefix := "schedule:" + scheduleID + ":" + execDate + ":c_" + configHash + ":"
	for _, l := range r.logs {
		if strings.HasPrefix(l.DedupKey, cfgPrefix) && (l.Status == "SENT" || l.Status == "FAILED" || l.Status == "NO_DEVICE") {
			return true, nil
		}
	}
	return false, nil
}

func (r *schedulerRegressionRepo) AcquireConn(ctx context.Context) (*sql.Conn, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) TryAdvisoryLockConn(conn *sql.Conn, ctx context.Context, scheduleID, execDate string) (bool, error) {
	return true, nil
}

func (r *schedulerRegressionRepo) ReleaseAdvisoryLockConn(conn *sql.Conn, ctx context.Context, scheduleID, execDate string) error {
	return nil
}

func (r *schedulerRegressionRepo) IsScheduleExecutedToday(ctx context.Context, scheduleID, execDate string, configHash ...string) (bool, error) {
	if len(configHash) > 0 && configHash[0] != "" {
		k := scheduleID + ":" + execDate + ":" + configHash[0]
		if _, ok := r.configExecutions[k]; ok {
			return true, nil
		}
		prefix := "schedule:" + scheduleID + ":" + execDate + ":c_" + configHash[0] + ":"
		for _, l := range r.logs {
			if strings.HasPrefix(l.DedupKey, prefix) && (l.Status == "SENT" || l.Status == "FAILED" || l.Status == "NO_DEVICE") {
				return true, nil
			}
		}
		return false, nil
	}
	prefix := "schedule:" + scheduleID + ":" + execDate + ":"
	for _, l := range r.logs {
		if strings.HasPrefix(l.DedupKey, prefix) && l.Status == "SENT" {
			return true, nil
		}
	}
	return false, nil
}

func (r *schedulerRegressionRepo) IsRecipientAlreadySentToday(ctx context.Context, dedupKey string) (bool, error) {
	for _, l := range r.logs {
		if l.DedupKey == dedupKey && l.Status == "SENT" {
			return true, nil
		}
	}
	return false, nil
}

func (r *schedulerRegressionRepo) RecordScheduleDelivery(ctx context.Context, rtID, userID, eventType, dedupKey, title, body string, payload map[string]string, status, failureReason string) (string, error) {
	for i, l := range r.logs {
		if l.DedupKey == dedupKey {
			if l.Status != "SENT" && status == "SENT" {
				r.logs[i].Status = "SENT"
			}
			return l.ID, nil
		}
	}
	uid := userID
	entry := notification.NotificationLogEntry{
		ID:        "log-" + dedupKey,
		RTID:      rtID,
		UserID:    &uid,
		EventType: eventType,
		DedupKey:  dedupKey,
		Title:     title,
		Body:      body,
		Status:    status,
		CreatedAt: time.Now(),
	}
	r.logs = append(r.logs, entry)
	return entry.ID, nil
}

func (r *schedulerRegressionRepo) ListSchedulerHistory(ctx context.Context, rtID string, page, pageSize int) ([]*notification.NotificationSchedulerHistory, int, error) {
	return nil, 0, nil
}

func (r *schedulerRegressionRepo) GetSchedulerRecipientDetail(ctx context.Context, rtID, scheduleID, execDate string, configHash ...string) ([]*notification.NotificationSchedulerRecipientDetail, error) {
	prefix := "schedule:" + scheduleID + ":" + execDate + ":"
	if len(configHash) > 0 && configHash[0] != "" {
		prefix = "schedule:" + scheduleID + ":" + execDate + ":c_" + configHash[0] + ":"
	}
	var details []*notification.NotificationSchedulerRecipientDetail
	for _, l := range r.logs {
		if strings.HasPrefix(l.DedupKey, prefix) {
			details = append(details, &notification.NotificationSchedulerRecipientDetail{
				UserID:    *l.UserID,
				Status:    l.Status,
				Title:     l.Title,
				CreatedAt: l.CreatedAt,
			})
		}
	}
	return details, nil
}

func (r *schedulerRegressionRepo) ListLogs(ctx context.Context, rtID, userID, eventType, status string, isOfficer bool, offset, limit int) ([]notification.NotificationLogEntry, int, error) {
	return nil, 0, nil
}

func (r *schedulerRegressionRepo) GetLogByID(ctx context.Context, rtID, id string) (*notification.NotificationLogEntry, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) ListManagementHistory(ctx context.Context, rtID, userID, category string, offset, limit int) ([]notification.NotificationLogEntry, int, error) {
	return nil, 0, nil
}

func (r *schedulerRegressionRepo) GetBroadcastRecipientLog(ctx context.Context, rtID, userID, eventType, broadcastID string) (*notification.NotificationLogEntry, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) MarkAsRead(ctx context.Context, rtID, userID, id string, isOfficer bool) error {
	return nil
}

func (r *schedulerRegressionRepo) MarkAllAsRead(ctx context.Context, rtID, userID string, isOfficer bool) (int, error) {
	return 0, nil
}

func (r *schedulerRegressionRepo) GetUnreadCount(ctx context.Context, rtID, userID string, isOfficer bool) (int, error) {
	return 0, nil
}

func (r *schedulerRegressionRepo) GetDefaultRTID(ctx context.Context) (string, error) {
	return "test-rt-002", nil
}

func (r *schedulerRegressionRepo) GetRWPengurusUserIDs(ctx context.Context, rwID string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) GetRWAndRTPengurusUserIDs(ctx context.Context, rwID string) ([]string, error) {
	return nil, nil
}

func (r *schedulerRegressionRepo) RecordRWDedup(ctx context.Context, rwID, userID, eventType, dedupKey, title, body string, payload map[string]string, senderUserID *string) (bool, error) {
	return true, nil
}

func (r *schedulerRegressionRepo) ListRWLogs(ctx context.Context, rwID, eventType string, offset, limit int) ([]notification.NotificationLogEntry, int, error) {
	return nil, 0, nil
}

func (r *schedulerRegressionRepo) GetRTUserBillsForEvaluation(ctx context.Context, rtID string) ([]finance.BillEvaluationInput, error) {
	return r.evalBills[rtID], nil
}

func (r *schedulerRegressionRepo) IsReminderAlreadySentForPeriod(ctx context.Context, rtID, userID, eventType, periodKey, billID string) (bool, error) {
	for _, l := range r.logs {
		if l.RTID == rtID && l.UserID != nil && *l.UserID == userID && l.EventType == eventType && l.Status == "SENT" {
			if strings.Contains(l.DedupKey, periodKey) && strings.Contains(l.DedupKey, billID) {
				return true, nil
			}
		}
	}
	return false, nil
}

// Custom mock push provider to simulate selective FCM failures.
type configurableMockPushProvider struct {
	failAll    bool
	failTokens map[string]error
	messages   []string
}

func newConfigurablePushProvider() *configurableMockPushProvider {
	return &configurableMockPushProvider{
		failTokens: make(map[string]error),
		messages:   make([]string, 0),
	}
}

func (p *configurableMockPushProvider) Send(ctx context.Context, token, title, body string, data map[string]string) error {
	if p.failAll {
		return errors.New("fcm provider unavailable")
	}
	if err, ok := p.failTokens[token]; ok {
		return err
	}
	p.messages = append(p.messages, token)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// REGRESSION SUITE: 17 MANDATORY SCENARIOS
// ─────────────────────────────────────────────────────────────────────────────

func TestNotificationScheduler_17_Mandatory_Regression_Scenarios(t *testing.T) {
	const testRT = "rt-test-sched-17"
	ctx := context.Background()

	setupService := func() (*schedulerRegressionRepo, *configurableMockPushProvider, notification.Service) {
		repo := newSchedulerRegressionRepo()
		provider := newConfigurablePushProvider()
		svc := notification.NewService(repo, provider)
		return repo, provider, svc
	}

	// -------------------------------------------------------------------------
	// Scenario 1: Jadwal pembayaran iuran 3 bulan sebelum jatuh tempo
	// -------------------------------------------------------------------------
	t.Run("Scenario 1: Upcoming due 3 months schedule execution", func(t *testing.T) {
		repo, provider, svc := setupService()
		evalDate := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC) // 3 calendar months before Nov 2026

		repo.templates["tmpl-upc"] = &notification.NotificationTemplate{
			ID: "tmpl-upc", RTID: testRT, EventType: notification.EventUpcomingDue3Months,
			Title: "Upcoming 3M {{nama}}", Body: "Jatuh tempo {{due_date}}", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-upc-1", RTID: testRT, EventType: notification.EventUpcomingDue3Months,
			TemplateID: "tmpl-upc", Name: "Upcoming 3M", IsActive: true,
			SendDays: []int{15}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		dueDate := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
		moveInDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		repo.evalBills[testRT] = []finance.BillEvaluationInput{
			{
				BillID: "bill-upc-1", UserID: "user-upc-1", UserFullName: "Warga Upcoming",
				Period: "2026-11", DueDate: &dueDate, AmountCents: 5000000,
				IsActive: true, IsObligatory: true, Status: "unpaid", DueIsActive: true,
				MoveInDate: &moveInDate,
			},
		}
		repo.devices["user-upc-1"] = []string{"token-upc-1"}

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 1 {
			t.Fatalf("expected 1 sent push, got %d", sent)
		}
		if len(provider.messages) != 1 {
			t.Fatalf("expected provider to receive 1 message, got %d", len(provider.messages))
		}
		if len(repo.logs) != 1 || repo.logs[0].Status != "SENT" {
			t.Fatalf("expected 1 log with status SENT, got %+v", repo.logs)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "success" {
			t.Fatalf("expected schedule status success, got %s", rec.status)
		}
		if rec.summary == nil || rec.summary.SentCount != 1 || rec.summary.TotalRecipients != 1 {
			t.Fatalf("invalid execution summary: %+v", rec.summary)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 2: Jadwal tunggakan 3 bulan atau lebih
	// -------------------------------------------------------------------------
	t.Run("Scenario 2: Overdue 3+ months schedule execution", func(t *testing.T) {
		repo, provider, svc := setupService()
		evalDate := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC) // 3+ months after July 2026

		repo.templates["tmpl-over"] = &notification.NotificationTemplate{
			ID: "tmpl-over", RTID: testRT, EventType: notification.EventOverdue3Months,
			Title: "Tunggakan {{nama}}", Body: "Total {{total_outstanding}}", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-over-2", RTID: testRT, EventType: notification.EventOverdue3Months,
			TemplateID: "tmpl-over", Name: "Tunggakan 3M+", IsActive: true,
			SendDays: []int{10}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		pastDueDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
		moveInDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		repo.evalBills[testRT] = []finance.BillEvaluationInput{
			{
				BillID: "bill-over-1", UserID: "user-over-1", UserFullName: "Warga Overdue",
				Period: "2026-07", DueDate: &pastDueDate, AmountCents: 10000000,
				IsActive: true, IsObligatory: true, Status: "unpaid", DueIsActive: true,
				MoveInDate: &moveInDate,
			},
		}
		repo.devices["user-over-1"] = []string{"token-over-1"}

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 1 {
			t.Fatalf("expected 1 sent push, got %d", sent)
		}
		if len(provider.messages) != 1 {
			t.Fatalf("expected provider to receive 1 message, got %d", len(provider.messages))
		}
		if len(repo.logs) != 1 || repo.logs[0].Status != "SENT" {
			t.Fatalf("expected 1 log with status SENT, got %+v", repo.logs)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "success" {
			t.Fatalf("expected schedule status success, got %s", rec.status)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 2B: Strict boundary overdue > 3m (exact 3m is false, 3m+1d is true)
	// -------------------------------------------------------------------------
	t.Run("Scenario 2B: Strict boundary overdue > 3m (exact 3m is false, 3m+1d is true)", func(t *testing.T) {
		repo, provider, svc := setupService()
		// Due date: 2026-07-15.
		// On 2026-10-15 (exact 3 months): MUST NOT be sent!
		// On 2026-10-16 (3 months + 1 day): MUST be sent!
		repo.templates["tmpl-over-b"] = &notification.NotificationTemplate{
			ID: "tmpl-over-b", RTID: testRT, EventType: notification.EventOverdue3Months,
			Title: "Tunggakan {{nama}}", Body: "Total {{total_outstanding}}", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-over-b", RTID: testRT, EventType: notification.EventOverdue3Months,
			TemplateID: "tmpl-over-b", Name: "Tunggakan Strict Boundary", IsActive: true,
			SendDays: []int{15, 16}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		dueDate := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
		moveInDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		repo.evalBills[testRT] = []finance.BillEvaluationInput{
			{
				BillID: "bill-over-b", UserID: "user-over-b", UserFullName: "Warga Boundary",
				Period: "2026-07", DueDate: &dueDate, AmountCents: 10000000,
				IsActive: true, IsObligatory: true, Status: "unpaid", DueIsActive: true,
				MoveInDate: &moveInDate,
			},
		}
		repo.devices["user-over-b"] = []string{"token-over-b"}

		// Test Day 1: Exact 3 months (2026-10-15) -> 0 sent, status no_recipients
		evalOct15 := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
		sentExact, err := svc.ExecuteSchedule(ctx, sched, evalOct15)
		if err != nil {
			t.Fatalf("ExecuteSchedule Oct 15: %v", err)
		}
		if sentExact != 0 {
			t.Fatalf("expected 0 sent on exact 3 months, got %d", sentExact)
		}
		if len(provider.messages) != 0 {
			t.Fatalf("expected provider to receive 0 messages on exact 3 months, got %d", len(provider.messages))
		}

		// Test Day 2: 3 months + 1 day (2026-10-16) -> 1 sent, status success
		evalOct16 := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
		sentOver, err := svc.ExecuteSchedule(ctx, sched, evalOct16)
		if err != nil {
			t.Fatalf("ExecuteSchedule Oct 16: %v", err)
		}
		if sentOver != 1 {
			t.Fatalf("expected 1 sent on 3 months + 1 day, got %d", sentOver)
		}
		if len(provider.messages) != 1 {
			t.Fatalf("expected provider to receive 1 message on 3m+1d, got %d", len(provider.messages))
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 3: Jadwal pembayaran iuran biasa
	// -------------------------------------------------------------------------
	t.Run("Scenario 3: Standard iuran payment reminder schedule execution", func(t *testing.T) {
		repo, provider, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-remind"] = &notification.NotificationTemplate{
			ID: "tmpl-remind", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Pengingat {{nama}}", Body: "Iuran {{period}}", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-remind-3", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-remind", Name: "Reminder Rutin", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{
			{BillID: "bill-std-1", UserID: "user-std-1", UserFullName: "Budi Santoso", RTID: testRT, Period: "2026-11", Amount: "50000", DueDate: "2026-11-20"},
		}
		repo.devices["user-std-1"] = []string{"token-std-1"}

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 1 {
			t.Fatalf("expected 1 sent push, got %d", sent)
		}
		if len(provider.messages) != 1 {
			t.Fatalf("expected provider to receive 1 message, got %d", len(provider.messages))
		}
		if len(repo.logs) != 1 || repo.logs[0].Status != "SENT" {
			t.Fatalf("expected log with status SENT, got %+v", repo.logs)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "success" {
			t.Fatalf("expected status success, got %s", rec.status)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 4: Template tidak ditemukan
	// -------------------------------------------------------------------------
	t.Run("Scenario 4: Template not found records failed status with clear reason", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		sched := &notification.NotificationSchedule{
			ID: "sched-no-tmpl-4", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "non-existent-tmpl", Name: "No Template Schedule", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err == nil {
			t.Fatalf("expected error when template not found, got nil")
		}
		if sent != 0 {
			t.Fatalf("expected 0 sends, got %d", sent)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "failed" {
			t.Fatalf("expected schedule status failed, got %s", rec.status)
		}
		if !strings.Contains(rec.errDesc, "Template not found") {
			t.Fatalf("expected error description about template not found, got %q", rec.errDesc)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 5: Template tidak aktif
	// -------------------------------------------------------------------------
	t.Run("Scenario 5: Inactive template records failed status with clear reason", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-inactive"] = &notification.NotificationTemplate{
			ID: "tmpl-inactive", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test", Body: "Body", IsActive: false, // inactive!
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-inact-5", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-inactive", Name: "Inactive Template Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err == nil {
			t.Fatalf("expected error when template inactive, got nil")
		}
		if sent != 0 {
			t.Fatalf("expected 0 sends, got %d", sent)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "failed" {
			t.Fatalf("expected status failed, got %s", rec.status)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 6: Notification type tidak dikenal
	// -------------------------------------------------------------------------
	t.Run("Scenario 6: Unknown notification type records failed status", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-weird"] = &notification.NotificationTemplate{
			ID: "tmpl-weird", RTID: testRT, EventType: "unknown_weird_event",
			Title: "Weird", Body: "Weird", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-weird-6", RTID: testRT, EventType: "unknown_weird_event",
			TemplateID: "tmpl-weird", Name: "Weird Event Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err == nil {
			t.Fatalf("expected error for unknown event type, got nil")
		}
		if sent != 0 {
			t.Fatalf("expected 0 sends, got %d", sent)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "failed" {
			t.Fatalf("expected status failed, got %s", rec.status)
		}
		if !strings.Contains(rec.errDesc, "unsupported event type") {
			t.Fatalf("expected error description about unsupported event, got %q", rec.errDesc)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 7: Recipient resolver menghasilkan nol penerima
	// -------------------------------------------------------------------------
	t.Run("Scenario 7: Zero recipients explicitly records no_recipients status without error", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-zero"] = &notification.NotificationTemplate{
			ID: "tmpl-zero", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test", Body: "Body", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-zero-7", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-zero", Name: "Zero Recipients Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched
		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{} // 0 recipients

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule should not error on 0 recipients, got: %v", err)
		}
		if sent != 0 {
			t.Fatalf("expected 0 sends, got %d", sent)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "no_recipients" {
			t.Fatalf("expected status 'no_recipients', got %s", rec.status)
		}
		if rec.summary == nil || rec.summary.TotalRecipients != 0 {
			t.Fatalf("expected summary with 0 total recipients, got %+v", rec.summary)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 8: Sebagian penerima tidak memiliki device token
	// -------------------------------------------------------------------------
	t.Run("Scenario 8: Some recipients have no device tokens (NO_DEVICE) -> aggregate status is SUCCESS not PARTIAL", func(t *testing.T) {
		repo, provider, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-part-dev"] = &notification.NotificationTemplate{
			ID: "tmpl-part-dev", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test {{nama}}", Body: "Body", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-nodev-8", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-part-dev", Name: "Mixed Devices Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		// 3 recipients: user-1 has device, user-2 has no device, user-3 has device
		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{
			{BillID: "b-1", UserID: "u-1", UserFullName: "Warga Satu", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
			{BillID: "b-2", UserID: "u-2", UserFullName: "Warga Dua", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
			{BillID: "b-3", UserID: "u-3", UserFullName: "Warga Tiga", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.devices["u-1"] = []string{"tok-1"}
		repo.devices["u-2"] = []string{} // NO DEVICE
		repo.devices["u-3"] = []string{"tok-3"}

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 2 {
			t.Fatalf("expected 2 successful push sends, got %d", sent)
		}
		if len(provider.messages) != 2 {
			t.Fatalf("expected provider to receive 2 messages, got %d", len(provider.messages))
		}
		// Check recipient log statuses
		var u2Log *notification.NotificationLogEntry
		for _, l := range repo.logs {
			if l.UserID != nil && *l.UserID == "u-2" {
				u2Log = &l
				break
			}
		}
		if u2Log == nil || u2Log.Status != "NO_DEVICE" {
			t.Fatalf("expected u-2 log to have status NO_DEVICE, got %+v", u2Log)
		}
		// Crucial verification: Since no FCM push failed, schedule status MUST BE success, NOT partial!
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "success" {
			t.Fatalf("expected schedule status 'success' when attempted sends succeeded, got %s", rec.status)
		}
		if rec.summary == nil || rec.summary.SentCount != 2 || rec.summary.NoDeviceCount != 1 || rec.summary.FailedCount != 0 {
			t.Fatalf("expected summary (Sent: 2, NoDevice: 1, Failed: 0), got %+v", rec.summary)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 9: FCM berhasil untuk semua penerima
	// -------------------------------------------------------------------------
	t.Run("Scenario 9: FCM succeeds for all recipients -> success status", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-all-ok"] = &notification.NotificationTemplate{
			ID: "tmpl-all-ok", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test {{nama}}", Body: "Body", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-all-ok-9", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-all-ok", Name: "All OK Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched
		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{
			{BillID: "b-10", UserID: "u-10", UserFullName: "A", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
			{BillID: "b-11", UserID: "u-11", UserFullName: "B", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.devices["u-10"] = []string{"tok-10"}
		repo.devices["u-11"] = []string{"tok-11"}

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 2 {
			t.Fatalf("expected 2 sent, got %d", sent)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "success" {
			t.Fatalf("expected status success, got %s", rec.status)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 10: FCM gagal untuk sebagian penerima
	// -------------------------------------------------------------------------
	t.Run("Scenario 10: FCM fails for some recipients -> PARTIAL status with failure reason", func(t *testing.T) {
		repo, provider, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-fcm-part"] = &notification.NotificationTemplate{
			ID: "tmpl-fcm-part", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test {{nama}}", Body: "Body", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-fcm-part-10", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-fcm-part", Name: "FCM Partial Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{
			{BillID: "b-20", UserID: "u-20", UserFullName: "Success User", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
			{BillID: "b-21", UserID: "u-21", UserFullName: "Fail User", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.devices["u-20"] = []string{"tok-ok-20"}
		repo.devices["u-21"] = []string{"tok-fail-21"}
		provider.failTokens["tok-fail-21"] = errors.New("simulated FCM device offline")

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 1 {
			t.Fatalf("expected 1 sent, got %d", sent)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "partial" {
			t.Fatalf("expected schedule status partial, got %s", rec.status)
		}
		if rec.summary == nil || rec.summary.SentCount != 1 || rec.summary.FailedCount != 1 {
			t.Fatalf("expected summary (Sent: 1, Failed: 1), got %+v", rec.summary)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 11: FCM gagal untuk seluruh penerima
	// -------------------------------------------------------------------------
	t.Run("Scenario 11: FCM fails for all recipients -> FAILED status", func(t *testing.T) {
		repo, provider, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-all-fail"] = &notification.NotificationTemplate{
			ID: "tmpl-all-fail", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test {{nama}}", Body: "Body", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-all-fail-11", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-all-fail", Name: "All Fail Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{
			{BillID: "b-30", UserID: "u-30", UserFullName: "Fail 1", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.devices["u-30"] = []string{"tok-30"}
		provider.failAll = true

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 0 {
			t.Fatalf("expected 0 sent, got %d", sent)
		}
		rec := repo.executedSchedules[sched.ID]
		if rec.status != "failed" {
			t.Fatalf("expected schedule status failed, got %s", rec.status)
		}
		if rec.summary == nil || rec.summary.FailedCount != 1 {
			t.Fatalf("expected summary with 1 failed, got %+v", rec.summary)
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 12: Retry eksekusi yang sama tidak menghasilkan push ganda
	// -------------------------------------------------------------------------
	t.Run("Scenario 12: Retry of executed schedule skips already-sent recipients without duplicate pushes", func(t *testing.T) {
		repo, provider, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-retry"] = &notification.NotificationTemplate{
			ID: "tmpl-retry", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test", Body: "Body", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-retry-12", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-retry", Name: "Retry Sched", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{
			{BillID: "b-40", UserID: "u-40", UserFullName: "Retry Target", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.devices["u-40"] = []string{"tok-40"}

		// Run 1: Succeeds
		sent1, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil || sent1 != 1 {
			t.Fatalf("Run 1 failed: sent=%d err=%v", sent1, err)
		}
		if len(provider.messages) != 1 {
			t.Fatalf("Run 1 provider messages = %d, want 1", len(provider.messages))
		}

		// Run 2 (Retry on same date): Must NOT send another push to u-40!
		sent2, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("Run 2 failed: %v", err)
		}
		if sent2 != 0 {
			t.Fatalf("expected 0 new sends on retry, got %d", sent2)
		}
		if len(provider.messages) != 1 {
			t.Fatalf("provider received duplicate push! total messages = %d, want 1", len(provider.messages))
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 13: Dua jadwal berbeda berjalan pada waktu yang sama
	// -------------------------------------------------------------------------
	t.Run("Scenario 13: Two distinct schedules at same time do not block each other", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-multi"] = &notification.NotificationTemplate{
			ID: "tmpl-multi", RTID: testRT, EventType: notification.EventPaymentReminder,
			Title: "Test", Body: "Body", IsActive: true,
		}
		schedA := &notification.NotificationSchedule{
			ID: "sched-13A", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-multi", Name: "Sched A", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		schedB := &notification.NotificationSchedule{
			ID: "sched-13B", RTID: testRT, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-multi", Name: "Sched B", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[schedA.ID] = schedA
		repo.schedules[schedB.ID] = schedB

		repo.iuranRecipients[testRT] = []*notification.IuranReminderRecipient{
			{BillID: "b-50", UserID: "u-50", UserFullName: "User A", RTID: testRT, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.devices["u-50"] = []string{"tok-50"}

		sentA, errA := svc.ExecuteSchedule(ctx, schedA, evalDate)
		sentB, errB := svc.ExecuteSchedule(ctx, schedB, evalDate)
		if errA != nil || errB != nil {
			t.Fatalf("errA: %v, errB: %v", errA, errB)
		}
		if sentA != 1 || sentB != 1 {
			t.Fatalf("sentA=%d, sentB=%d, both should execute independently", sentA, sentB)
		}
		if repo.executedSchedules[schedA.ID].status != "success" || repo.executedSchedules[schedB.ID].status != "success" {
			t.Fatalf("both schedules must reach status success")
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 14: Jadwal tanggal 29, 30, dan 31 pada bulan yang tidak memiliki tanggal tersebut
	// -------------------------------------------------------------------------
	t.Run("Scenario 14: Day 29, 30, 31 edge case correctly matches last day of shorter months", func(t *testing.T) {
		// Test February 2026 (non-leap year: 28 days)
		loc, _ := time.LoadLocation("Asia/Jakarta")
		feb27 := time.Date(2026, 2, 27, 8, 0, 0, 0, loc)
		feb28 := time.Date(2026, 2, 28, 8, 0, 0, 0, loc) // last day of Feb 2026

		sched31 := &notification.NotificationSchedule{
			ID: "s-31", SendDays: []int{31}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		sched30 := &notification.NotificationSchedule{
			ID: "s-30", SendDays: []int{30}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		sched29 := &notification.NotificationSchedule{
			ID: "s-29", SendDays: []int{29}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}

		// On Feb 27: none should match
		if notification.IsDayDueForTesting(sched31.SendDays, feb27) ||
			notification.IsDayDueForTesting(sched30.SendDays, feb27) ||
			notification.IsDayDueForTesting(sched29.SendDays, feb27) {
			t.Fatalf("days 29, 30, 31 must not match on Feb 27")
		}

		// On Feb 28 (last day): ALL 29, 30, 31 must match!
		if !notification.IsDayDueForTesting(sched31.SendDays, feb28) {
			t.Fatalf("day 31 must match on Feb 28 (last day of month)")
		}
		if !notification.IsDayDueForTesting(sched30.SendDays, feb28) {
			t.Fatalf("day 30 must match on Feb 28 (last day of month)")
		}
		if !notification.IsDayDueForTesting(sched29.SendDays, feb28) {
			t.Fatalf("day 29 must match on Feb 28 (last day of month)")
		}

		// Test November 2026 (30 days)
		nov29 := time.Date(2026, 11, 29, 8, 0, 0, 0, loc)
		nov30 := time.Date(2026, 11, 30, 8, 0, 0, 0, loc) // last day of Nov

		if notification.IsDayDueForTesting(sched31.SendDays, nov29) {
			t.Fatalf("day 31 must not match on Nov 29")
		}
		if !notification.IsDayDueForTesting(sched31.SendDays, nov30) {
			t.Fatalf("day 31 must match on Nov 30 (last day of 30-day month)")
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 15: Timezone Asia/Jakarta dan pergantian tanggal
	// -------------------------------------------------------------------------
	t.Run("Scenario 15: Timezone Asia/Jakarta midnight and date boundaries", func(t *testing.T) {
		// UTC 2026-11-04 17:30 is 2026-11-05 00:30 in Asia/Jakarta (UTC+7)
		utcTime := time.Date(2026, 11, 4, 17, 30, 0, 0, time.UTC)
		loc, _ := time.LoadLocation("Asia/Jakarta")
		localTime := utcTime.In(loc)

		if localTime.Day() != 5 {
			t.Fatalf("expected local day to be 5 in Asia/Jakarta, got %d", localTime.Day())
		}
		if localTime.Hour() != 0 || localTime.Minute() != 30 {
			t.Fatalf("expected 00:30 in Asia/Jakarta, got %02d:%02d", localTime.Hour(), localTime.Minute())
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 16: Isolasi RT dan permission admin
	// -------------------------------------------------------------------------
	t.Run("Scenario 16: RT isolation prevents cross-tenant recipient leakage", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

		const rtA = "rt-alpha"
		const rtB = "rt-beta"

		repo.templates["tmpl-iso"] = &notification.NotificationTemplate{
			ID: "tmpl-iso", RTID: rtA, EventType: notification.EventPaymentReminder,
			Title: "Test", Body: "Body", IsActive: true,
		}
		schedA := &notification.NotificationSchedule{
			ID: "sched-iso-A", RTID: rtA, EventType: notification.EventPaymentReminder,
			TemplateID: "tmpl-iso", Name: "Iso Sched A", IsActive: true,
			SendDays: []int{5}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[schedA.ID] = schedA

		// RT A has 1 resident, RT B has 1 resident
		repo.iuranRecipients[rtA] = []*notification.IuranReminderRecipient{
			{BillID: "b-rtA", UserID: "u-rtA", UserFullName: "User RT A", RTID: rtA, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.iuranRecipients[rtB] = []*notification.IuranReminderRecipient{
			{BillID: "b-rtB", UserID: "u-rtB", UserFullName: "User RT B", RTID: rtB, Period: "2026-11", Amount: "10000", DueDate: "2026-11-20"},
		}
		repo.devices["u-rtA"] = []string{"tok-rtA"}
		repo.devices["u-rtB"] = []string{"tok-rtB"}

		sent, err := svc.ExecuteSchedule(ctx, schedA, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 1 {
			t.Fatalf("expected 1 sent for RT A, got %d", sent)
		}
		// Confirm ONLY RT A resident was logged
		for _, l := range repo.logs {
			if l.RTID != rtA {
				t.Fatalf("cross-tenant leak! Found log with RTID %s, want %s", l.RTID, rtA)
			}
			if l.UserID != nil && *l.UserID == "u-rtB" {
				t.Fatalf("cross-tenant leak! User from RT B received RT A notification!")
			}
		}
	})

	// -------------------------------------------------------------------------
	// Scenario 17: Validasi aturan move_in_date dan is_obligatory
	// -------------------------------------------------------------------------
	t.Run("Scenario 17: move_in_date and is_obligatory correctly enforced", func(t *testing.T) {
		repo, _, svc := setupService()
		evalDate := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)

		repo.templates["tmpl-mv"] = &notification.NotificationTemplate{
			ID: "tmpl-mv", RTID: testRT, EventType: notification.EventOverdue3Months,
			Title: "Test", Body: "Body", IsActive: true,
		}
		sched := &notification.NotificationSchedule{
			ID: "sched-mv-17", RTID: testRT, EventType: notification.EventOverdue3Months,
			TemplateID: "tmpl-mv", Name: "Move In Date Sched", IsActive: true,
			SendDays: []int{10}, SendHour: 8, SendMinute: 0, Timezone: "Asia/Jakarta",
		}
		repo.schedules[sched.ID] = sched

		pastDueDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
		moveInDateAfterBill := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) // Moved in AFTER July 2026!
		moveInDateBeforeBill := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		repo.evalBills[testRT] = []finance.BillEvaluationInput{
			// Bill 1: Non-obligatory -> Must be excluded
			{
				BillID: "b-non-ob", UserID: "u-non-ob", UserFullName: "Voluntary Warga",
				Period: "2026-07", DueDate: &pastDueDate, AmountCents: 1000000,
				IsActive: true, IsObligatory: false, Status: "unpaid", DueIsActive: true,
				MoveInDate: &moveInDateBeforeBill,
			},
			// Bill 2: Resident moved in AFTER bill period -> Must be excluded
			{
				BillID: "b-new-res", UserID: "u-new-res", UserFullName: "New Resident",
				Period: "2026-07", DueDate: &pastDueDate, AmountCents: 1000000,
				IsActive: true, IsObligatory: true, Status: "unpaid", DueIsActive: true,
				MoveInDate: &moveInDateAfterBill,
			},
			// Bill 3: Obligatory & resident lived there during bill period -> Must be included
			{
				BillID: "b-valid", UserID: "u-valid", UserFullName: "Valid Resident",
				Period: "2026-07", DueDate: &pastDueDate, AmountCents: 1000000,
				IsActive: true, IsObligatory: true, Status: "unpaid", DueIsActive: true,
				MoveInDate: &moveInDateBeforeBill,
			},
		}
		repo.devices["u-valid"] = []string{"tok-valid"}

		sent, err := svc.ExecuteSchedule(ctx, sched, evalDate)
		if err != nil {
			t.Fatalf("ExecuteSchedule: %v", err)
		}
		if sent != 1 {
			t.Fatalf("expected exactly 1 eligible recipient, got %d", sent)
		}
		if len(repo.logs) != 1 || repo.logs[0].UserID == nil || *repo.logs[0].UserID != "u-valid" {
			t.Fatalf("expected only u-valid to receive notification, got logs: %+v", repo.logs)
		}
	})
}

func (r *schedulerRegressionRepo) DeactivateSchedule(ctx context.Context, rtID, id string) error {
	if s, ok := r.schedules[id]; ok && s.RTID == rtID {
		s.IsActive = false
	}
	return nil
}

func (r *schedulerRegressionRepo) CancelSchedulesForActivity(ctx context.Context, rtID, activityID string) (int64, error) {
	var n int64
	for _, s := range r.schedules {
		if s.RTID == rtID && s.ActivityID != nil && *s.ActivityID == activityID && s.IsActive {
			s.IsActive = false
			n++
		}
	}
	return n, nil
}

func (r *schedulerRegressionRepo) ActivityInfoFetch(ctx context.Context, rtID, activityID string) (*notification.ActivityPayload, error) {
	return nil, sql.ErrNoRows
}

func (r *schedulerRegressionRepo) ListActivityStateCandidates(ctx context.Context, now time.Time) ([]notification.ActivityPayload, error) {
	return nil, nil
}
