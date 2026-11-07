package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"social-finance/internal/database"
	"social-finance/internal/finance"
)

// Repository manages persistence for devices and notification delivery logs.
type Repository interface {
	RegisterDevice(ctx context.Context, userID, token, platform string, deviceID *string) error
	UnregisterDevice(ctx context.Context, userID, token string) error
	DeactivateToken(ctx context.Context, token string) error
	GetTokensForUser(ctx context.Context, userID string) ([]string, error)
	GetTokensForPaymentVerifiers(ctx context.Context, rtID string) ([]string, error)
	GetTokensForRT(ctx context.Context, rtID string) ([]string, error)
	GetTokensForWasteTruckArrival(ctx context.Context, rtID string) ([]string, error)
	GetWasteTruckArrivalRecipients(ctx context.Context, rtID string) ([]WasteTruckArrivalRecipient, error)
	GetRTBroadcastRecipients(ctx context.Context, rtID string) ([]WasteTruckArrivalRecipient, error)
	GetRecipientUserIDsForBill(ctx context.Context, rtID, billID string) ([]string, error)
	CheckAndRecordDedup(ctx context.Context, rtID, userID, eventType, dedupKey, title, body string, payload map[string]string) (bool, error)
	RecordWasteTruckDispatch(ctx context.Context, rtID, senderUserID, dedupKey, title, body string, payload map[string]string) (string, bool, error)
	RecordBroadcastRecipientDelivery(ctx context.Context, rtID, senderUserID, recipientUserID, eventType, broadcastID, title, body string, payload map[string]string, status string) error
	UpdateWasteTruckDispatch(ctx context.Context, rtID, senderUserID, id, status, failureReason string) error
	ListWasteTruckDispatches(ctx context.Context, rtID, senderUserID string, offset, limit int) ([]WasteTruckDispatchEntry, int, error)

	// Template management
	ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]NotificationTemplate, error)
	GetTemplateByID(ctx context.Context, rtID, id string) (*NotificationTemplate, error)
	GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*NotificationTemplate, error)
	CreateTemplate(ctx context.Context, rtID string, in CreateTemplateInput, allowedVars []string) (*NotificationTemplate, error)
	UpdateTemplate(ctx context.Context, rtID, id string, in UpdateTemplateInput) (*NotificationTemplate, error)
	ToggleTemplateActive(ctx context.Context, rtID, id string) (*NotificationTemplate, error)
	DeleteTemplate(ctx context.Context, rtID, id string) error

	// Schedule management
	CreateSchedule(ctx context.Context, rtID, userID string, in CreateScheduleInput) (*NotificationSchedule, error)
	GetSchedule(ctx context.Context, rtID, id string) (*NotificationSchedule, error)
	ListSchedules(ctx context.Context, rtID, eventType string, isActive *bool) ([]*NotificationSchedule, error)
	UpdateSchedule(ctx context.Context, rtID, id, userID string, in UpdateScheduleInput) (*NotificationSchedule, error)
	DeleteSchedule(ctx context.Context, rtID, id string) error
	ToggleScheduleActive(ctx context.Context, rtID, id, userID string) (*NotificationSchedule, error)
	// GetDueSchedules returns active schedules whose send days/hour/minute match
	// the given local time (computed in each schedule's own timezone).
	GetDueSchedules(ctx context.Context, now time.Time) ([]*NotificationSchedule, error)
	// GetIuranReminderRecipients returns obligatory, active, unpaid/partial bills
	// whose occupants are eligible for the iuran payment reminder as of `asOf`.
	GetIuranReminderRecipients(ctx context.Context, rtID string, asOf time.Time) ([]*IuranReminderRecipient, error)
	// UpdateScheduleExecuted records the last execution time/status of a schedule.
	UpdateScheduleExecuted(ctx context.Context, scheduleID, rtID string, at time.Time, status string) error
	// UpdateScheduleExecutedDetailed records execution time, status, safe error detail, execution summary metrics,
	// and persists the configuration version execution record.
	UpdateScheduleExecutedDetailed(ctx context.Context, scheduleID, rtID string, at time.Time, status string, errorDetail *string, summary *ScheduleExecutionSummary, configHash ...string) error
	// RecordScheduleExecution persists an execution instance in notification_schedule_executions
	// with unique constraint (schedule_id, exec_date, config_hash).
	RecordScheduleExecution(ctx context.Context, scheduleID, rtID string, execDate time.Time, configHash, status string, errorDetail *string, summary *ScheduleExecutionSummary) error
	// IsScheduleExecutionDone reports whether (schedule_id, exec_date, config_hash) has completed.
	IsScheduleExecutionDone(ctx context.Context, scheduleID, execDate, configHash string) (bool, error)
	// IsRecipientAlreadySentToday reports whether a recipient delivery log for a schedule execution already succeeded today.
	IsRecipientAlreadySentToday(ctx context.Context, dedupKey string) (bool, error)
	// IsScheduleExecutedToday reports whether a schedule already has a completed
	// (SENT/FAILED) delivery log for the given local execution date and optional config hash.
	IsScheduleExecutedToday(ctx context.Context, scheduleID, execDate string, configHash ...string) (bool, error)
	// RecordScheduleDelivery inserts a per-recipient delivery log with provider
	// evidence, guarded by the UNIQUE dedup_key. Returns the log id.
	RecordScheduleDelivery(ctx context.Context, rtID, userID, eventType, dedupKey, title, body string, payload map[string]string, status, failureReason string) (string, error)

	// ListSchedulerHistory returns aggregated scheduler execution history for
	// the tenant RT. One row per (schedule, execution date, config_hash).
	// Only logs with dedup_key prefixed "schedule:" are included.
	ListSchedulerHistory(ctx context.Context, rtID string, page, pageSize int) ([]*NotificationSchedulerHistory, int, error)

	// GetSchedulerRecipientDetail returns individual recipient delivery results
	// for a single scheduler execution identified by schedule ID, exec date, and optional config hash.
	GetSchedulerRecipientDetail(ctx context.Context, rtID, scheduleID, execDate string, configHash ...string) ([]*NotificationSchedulerRecipientDetail, error)

	// Transactional helpers used by the scheduler to guarantee at-most-once
	// execution across multiple backend instances. A dedicated connection is
	// acquired and a session-level Postgres advisory lock is held for the whole
	// execution (including the FCM network call) so that only one instance
	// delivers a given schedule on a given execution date. The database
	// transaction itself is NOT held during the network call.
	AcquireConn(ctx context.Context) (*sql.Conn, error)
	TryAdvisoryLockConn(conn *sql.Conn, ctx context.Context, scheduleID, execDate string) (bool, error)
	ReleaseAdvisoryLockConn(conn *sql.Conn, ctx context.Context, scheduleID, execDate string) error

	// Logs listing & read receipts
	ListLogs(ctx context.Context, rtID, userID, eventType, status string, isOfficer bool, offset, limit int) ([]NotificationLogEntry, int, error)
	ListManagementHistory(ctx context.Context, rtID, userID, category string, offset, limit int) ([]NotificationLogEntry, int, error)
	GetLogByID(ctx context.Context, rtID, id string) (*NotificationLogEntry, error)
	GetBroadcastRecipientLog(ctx context.Context, rtID, userID, eventType, broadcastID string) (*NotificationLogEntry, error)
	MarkAsRead(ctx context.Context, rtID, userID, id string, isOfficer bool) error
	MarkAllAsRead(ctx context.Context, rtID, userID string, isOfficer bool) (int, error)
	GetUnreadCount(ctx context.Context, rtID, userID string, isOfficer bool) (int, error)

	// Tenant resolution fallback for system roles
	GetDefaultRTID(ctx context.Context) (string, error)
}

// OverdueReminderRepository is an optional extension of Repository that evaluates
// upcoming due (3 months before due_date) and overdue (3+ months after due_date)
// bills using the shared finance calculation and checks period-level deduplication.
type OverdueReminderRepository interface {
	GetRTUserBillsForEvaluation(ctx context.Context, rtID string) ([]finance.BillEvaluationInput, error)
	IsReminderAlreadySentForPeriod(ctx context.Context, rtID, userID, eventType, periodKey, billID string) (bool, error)
}

type pgRepository struct {
	pool *database.Pool
}

// NewRepository creates a new PostgreSQL notification repository.
func NewRepository(pool *database.Pool) Repository {
	return &pgRepository{pool: pool}
}

// isTableMissingErr checks if PostgreSQL returned 42P01 (relation does not exist).
func isTableMissingErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "42P01") || strings.Contains(err.Error(), "does not exist")
}

func (r *pgRepository) RegisterDevice(ctx context.Context, userID, token, platform string, deviceID *string) error {
	deactivateOther := `
		UPDATE user_devices
		SET is_active = false, updated_at = NOW()
		WHERE fcm_token = $1 AND user_id != $2 AND is_active = true;
	`
	_, _ = r.pool.Raw().ExecContext(ctx, deactivateOther, token, userID)

	query := `
		INSERT INTO user_devices (user_id, fcm_token, platform, device_id, is_active, last_seen_at, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW(), NOW())
		ON CONFLICT (user_id, fcm_token)
		DO UPDATE SET
			platform = EXCLUDED.platform,
			device_id = EXCLUDED.device_id,
			is_active = true,
			last_seen_at = NOW(),
			updated_at = NOW();
	`
	_, err := r.pool.Raw().ExecContext(ctx, query, userID, token, platform, deviceID)
	if isTableMissingErr(err) {
		slog.Warn("user_devices table does not exist yet (migration 022 pending)", "error", err)
		return nil
	}
	return err
}

func (r *pgRepository) UnregisterDevice(ctx context.Context, userID, token string) error {
	query := `
		UPDATE user_devices
		SET is_active = false, updated_at = NOW()
		WHERE user_id = $1 AND fcm_token = $2;
	`
	_, err := r.pool.Raw().ExecContext(ctx, query, userID, token)
	if isTableMissingErr(err) {
		slog.Warn("user_devices table does not exist yet (migration 022 pending)", "error", err)
		return nil
	}
	return err
}

func (r *pgRepository) DeactivateToken(ctx context.Context, token string) error {
	query := `
		UPDATE user_devices
		SET is_active = false, updated_at = NOW()
		WHERE fcm_token = $1;
	`
	_, err := r.pool.Raw().ExecContext(ctx, query, token)
	if isTableMissingErr(err) {
		slog.Warn("user_devices table does not exist yet (migration 022 pending)", "error", err)
		return nil
	}
	return err
}

func (r *pgRepository) GetTokensForUser(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT fcm_token
		FROM user_devices
		WHERE user_id = $1 AND is_active = true;
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query, userID)
	if err != nil {
		if isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}
	return tokens, rows.Err()
}

// GetTokensForPaymentVerifiers returns active device tokens of every active
// user in the RT who is authorized to verify payments via
// POST /api/v1/payments/{id}/verify (cmd/api/main.go):
//
//   - RequirePermission("finance.approve"): members holding a jabatan are
//     evaluated against position_permissions (e.g. ketua with role=warga,
//     bendahara with role=pengurus); members without a jabatan pass through.
//   - RequireAccess(roles=[bendahara, pengurus], jabatans=[ketua, bendahara]):
//     tenant role OR jabatan must match.
//
// The notification recipient set therefore mirrors the actual authorization
// policy instead of a hard-coded role/job-title allowlist, and includes cases
// such as role=pengurus + jabatan=bendahara (finance.approve via
// position_permissions) that the previous role='bendahara'/jabatan='ketua'
// allowlist missed. Inactive users, inactive memberships, and inactive
// devices are excluded; DISTINCT prevents duplicate tokens when a member
// matches multiple conditions.
//
// Note: role=ketua or role=wakil_ketua WITHOUT a jabatan are intentionally
// excluded from the role-based fallback because RequireAccess does not list
// them in the role map (only jabatan=ketua/bendahara grants access).
// Users with jabatan=ketua or jabatan=bendahara still qualify via the
// position_permissions branch.
func (r *pgRepository) GetTokensForPaymentVerifiers(ctx context.Context, rtID string) ([]string, error) {
	query := `
		SELECT DISTINCT ud.fcm_token
		FROM user_devices ud
		JOIN user_rt_memberships urm ON urm.user_id = ud.user_id
		JOIN users u ON u.id = ud.user_id
		WHERE urm.rt_id = $1
		  AND ud.is_active = true
		  AND urm.is_active = true
		  AND u.is_active = true
		  AND (
		      -- Jabatan-based verifier: the membership position grants
		      -- finance.approve via position_permissions (e.g. ketua,
		      -- bendahara regardless of tenant role).
		      (urm.jabatan IS NOT NULL AND EXISTS (
		          SELECT 1
		          FROM position_permissions pp
		          JOIN permission_types pt ON pt.id = pp.permission_id
		          WHERE pp.position = urm.jabatan
		            AND pt.code = 'finance.approve'
		      ))
		      -- Role-based verifier without jabatan: RequirePermission passes
		      -- members that hold no jabatan, and RequireAccess allows the
		      -- bendahara and pengurus tenant roles.
		      OR (
		          (urm.jabatan IS NULL OR btrim(urm.jabatan) = '')
		          AND urm.role IN ('bendahara', 'pengurus')
		      )
		  );
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query, rtID)
	if err != nil {
		if isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}
	return tokens, rows.Err()
}

func (r *pgRepository) GetTokensForRT(ctx context.Context, rtID string) ([]string, error) {
	recipients, err := r.GetRTBroadcastRecipients(ctx, rtID)
	if err != nil {
		return nil, err
	}
	var tokens []string
	for _, recipient := range recipients {
		tokens = append(tokens, recipient.Tokens...)
	}
	return tokens, nil
}

func (r *pgRepository) GetTokensForWasteTruckArrival(ctx context.Context, rtID string) ([]string, error) {
	recipients, err := r.GetWasteTruckArrivalRecipients(ctx, rtID)
	if err != nil {
		return nil, err
	}
	var tokens []string
	for _, recipient := range recipients {
		tokens = append(tokens, recipient.Tokens...)
	}
	return tokens, nil
}

func (r *pgRepository) GetWasteTruckArrivalRecipients(ctx context.Context, rtID string) ([]WasteTruckArrivalRecipient, error) {
	query := `
		SELECT DISTINCT u.id::text, ud.fcm_token
		FROM users u
		JOIN user_rt_memberships urm ON urm.user_id = u.id
		  AND urm.rt_id = $1 AND urm.is_active = true
		JOIN residents res ON (res.user_id = u.id OR (
			res.user_id IS NULL AND res.email IS NOT NULL AND lower(trim(u.email)) = lower(trim(res.email))
		))
		  AND res.rt_id = $1 AND res.is_active = true
		JOIN residency_periods rp ON rp.resident_id = res.id
		  AND (rp.start_date IS NULL OR rp.start_date <= CURRENT_DATE)
		  AND (rp.end_date IS NULL OR rp.end_date > CURRENT_DATE)
		JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
		  AND (ho.start_date IS NULL OR ho.start_date <= CURRENT_DATE)
		  AND ho.end_date IS NULL
		JOIN households hh ON hh.id = ho.household_id
		  AND hh.rt_id = $1 AND hh.is_active = true
		LEFT JOIN user_devices ud ON ud.user_id = u.id AND ud.is_active = true
		WHERE res.rt_id = $1
		  AND u.is_active = true
		ORDER BY u.id::text, ud.fcm_token
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query, rtID)
	if err != nil {
		if isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	return scanNotificationRecipients(rows)
}

func (r *pgRepository) GetRTBroadcastRecipients(ctx context.Context, rtID string) ([]WasteTruckArrivalRecipient, error) {
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT DISTINCT u.id::text, ud.fcm_token
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id AND u.is_active = true
		LEFT JOIN user_devices ud ON ud.user_id = u.id AND ud.is_active = true
		WHERE urm.rt_id = $1 AND urm.is_active = true
		ORDER BY u.id::text, ud.fcm_token
	`, rtID)
	if err != nil {
		if isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	return scanNotificationRecipients(rows)
}

func scanNotificationRecipients(rows *sql.Rows) ([]WasteTruckArrivalRecipient, error) {
	recipients := make([]WasteTruckArrivalRecipient, 0)
	byUser := make(map[string]int)
	for rows.Next() {
		var userID string
		var token sql.NullString
		if err := rows.Scan(&userID, &token); err != nil {
			return nil, err
		}
		index, ok := byUser[userID]
		if !ok {
			index = len(recipients)
			byUser[userID] = index
			recipients = append(recipients, WasteTruckArrivalRecipient{UserID: userID})
		}
		if token.Valid {
			recipients[index].Tokens = append(recipients[index].Tokens, token.String)
		}
	}
	return recipients, rows.Err()
}

// GetRecipientUserIDsForBill returns the users living in the household the
// bill is addressed to. Eligibility follows the residency model only:
// open residency period, active resident, active user. Membership roles are
// intentionally NOT considered, so residents who also hold management roles
// (pengurus, bendahara, ...) keep receiving notifications about their own
// household's bills. DISTINCT prevents duplicates when a person matches
// multiple recipient conditions.
func (r *pgRepository) GetRecipientUserIDsForBill(ctx context.Context, rtID, billID string) ([]string, error) {
	query := `
		SELECT DISTINCT u.id
		FROM bills b
		JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
		JOIN residency_periods rp ON ho.id = rp.household_occupancy_id
		JOIN residents res ON rp.resident_id = res.id
		JOIN users u ON (u.id = res.user_id OR (res.user_id IS NULL AND res.email IS NOT NULL AND lower(trim(u.email)) = lower(trim(res.email))))
		WHERE b.id = $1 AND b.rt_id = $2
		  AND res.rt_id = $2
		  AND res.is_active = true
		  AND u.is_active = true
		  AND (rp.end_date IS NULL OR rp.end_date >= CURRENT_DATE);
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query, billID, rtID)
	if err != nil {
		if isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, uid)
	}
	return userIDs, rows.Err()
}

func (r *pgRepository) CheckAndRecordDedup(
	ctx context.Context,
	rtID, userID, eventType, dedupKey, title, body string,
	payload map[string]string,
) (bool, error) {
	payloadJSON, _ := json.Marshal(payload)
	var uidParam *string
	if strings.TrimSpace(userID) != "" {
		trimmed := strings.TrimSpace(userID)
		uidParam = &trimmed
	}
	var imageURL *string
	if u, ok := payload["image_url"]; ok && strings.TrimSpace(u) != "" {
		trimmed := strings.TrimSpace(u)
		imageURL = &trimmed
	}
	query := `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, payload, status, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'SENT', $8)
		ON CONFLICT (dedup_key) DO NOTHING
		RETURNING id;
	`
	var id string
	err := r.pool.Raw().QueryRowContext(ctx, query, rtID, uidParam, eventType, dedupKey, title, body, payloadJSON, imageURL).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Row already exists -> deduplicated!
			return false, nil
		}
		if isTableMissingErr(err) {
			return false, fmt.Errorf("notification_logs table does not exist (migration pending): %w", err)
		}
		return false, err
	}
	if payload != nil && id != "" {
		payload["notification_id"] = id
	}
	return true, nil
}

func (r *pgRepository) RecordWasteTruckDispatch(
	ctx context.Context,
	rtID, senderUserID, dedupKey, title, body string,
	payload map[string]string,
) (string, bool, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}
	var imageURL *string
	if image := strings.TrimSpace(payload["image_url"]); image != "" {
		imageURL = &image
	}

	var id string
	err = r.pool.Raw().QueryRowContext(ctx, `
		INSERT INTO notification_logs
			(rt_id, sender_user_id, event_type, dedup_key, title, body, payload, status, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'PENDING', $8)
		ON CONFLICT (dedup_key) DO NOTHING
		RETURNING id
	`, rtID, senderUserID, EventWasteTruckArrival, dedupKey, title, body, payloadJSON, imageURL).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	if payload != nil {
		payload["notification_id"] = id
	}
	return id, true, nil
}

func (r *pgRepository) RecordBroadcastRecipientDelivery(
	ctx context.Context,
	rtID, senderUserID, recipientUserID, eventType, broadcastID, title, body string,
	payload map[string]string,
	status string,
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var imageURL *string
	if image := strings.TrimSpace(payload["image_url"]); image != "" {
		imageURL = &image
	}
	_, err = r.pool.Raw().ExecContext(ctx, `
		INSERT INTO notification_logs
			(rt_id, user_id, sender_user_id, event_type, dedup_key, title, body, payload, status, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (dedup_key) DO NOTHING
	`, rtID, recipientUserID, senderUserID, eventType,
		"recipient:"+eventType+":"+broadcastID+":"+recipientUserID,
		title, body, payloadJSON, status, imageURL)
	return err
}

func (r *pgRepository) UpdateWasteTruckDispatch(
	ctx context.Context,
	rtID, senderUserID, id, status, failureReason string,
) error {
	result, err := r.pool.Raw().ExecContext(ctx, `
		UPDATE notification_logs
		SET status = $1, failure_reason = NULLIF($2, '')
		WHERE id = $3 AND rt_id = $4 AND sender_user_id = $5
		  AND event_type = $6
	`, status, failureReason, id, rtID, senderUserID, EventWasteTruckArrival)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *pgRepository) ListWasteTruckDispatches(
	ctx context.Context,
	rtID, senderUserID string,
	offset, limit int,
) ([]WasteTruckDispatchEntry, int, error) {
	const where = `
		FROM notification_logs
		WHERE rt_id = $1 AND sender_user_id = $2
		  AND event_type = $3
		  AND user_id IS NULL
	`
	var total int
	if err := r.pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) `+where,
		rtID, senderUserID, EventWasteTruckArrival,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT id, event_type, title, body, image_url, status, failure_reason, created_at
	`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $4 OFFSET $5
	`, rtID, senderUserID, EventWasteTruckArrival, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := make([]WasteTruckDispatchEntry, 0)
	for rows.Next() {
		var entry WasteTruckDispatchEntry
		if err := rows.Scan(
			&entry.ID, &entry.EventType, &entry.Title, &entry.Body, &entry.ImageURL,
			&entry.Status, &entry.FailureReason, &entry.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		entries = append(entries, entry)
	}
	return entries, total, rows.Err()
}

func (r *pgRepository) ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]NotificationTemplate, error) {
	query := `
		SELECT id, rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at, image_url
		FROM notification_templates
		WHERE rt_id = $1
	`
	args := []any{rtID}
	argIdx := 2

	if strings.TrimSpace(eventType) != "" {
		query += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, strings.TrimSpace(eventType))
		argIdx++
	}

	if isActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *isActive)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Raw().QueryContext(ctx, query, args...)
	if err != nil {
		if isTableMissingErr(err) {
			return []NotificationTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var templates []NotificationTemplate
	for rows.Next() {
		var t NotificationTemplate
		var varsRaw []byte
		if err := rows.Scan(&t.ID, &t.RTID, &t.Name, &t.EventType, &t.Channel, &t.Title, &t.Body, &t.IsActive, &varsRaw, &t.CreatedAt, &t.UpdatedAt, &t.ImageURL); err != nil {
			return nil, err
		}
		if len(varsRaw) > 0 {
			_ = json.Unmarshal(varsRaw, &t.Variables)
		}
		if t.Variables == nil {
			t.Variables = []string{}
		}
		templates = append(templates, t)
	}
	if templates == nil {
		templates = []NotificationTemplate{}
	}
	return templates, rows.Err()
}

func (r *pgRepository) GetTemplateByID(ctx context.Context, rtID, id string) (*NotificationTemplate, error) {
	query := `
		SELECT id, rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at, image_url
		FROM notification_templates
		WHERE id = $1 AND rt_id = $2
		LIMIT 1;
	`
	var t NotificationTemplate
	var varsRaw []byte
	err := r.pool.Raw().QueryRowContext(ctx, query, id, rtID).Scan(
		&t.ID, &t.RTID, &t.Name, &t.EventType, &t.Channel, &t.Title, &t.Body, &t.IsActive, &varsRaw, &t.CreatedAt, &t.UpdatedAt, &t.ImageURL,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(varsRaw) > 0 {
		_ = json.Unmarshal(varsRaw, &t.Variables)
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}
	return &t, nil
}

func (r *pgRepository) GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*NotificationTemplate, error) {
	query := `
		SELECT id, rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at, image_url
		FROM notification_templates
		WHERE rt_id = $1 AND event_type = $2 AND is_active = true
		ORDER BY updated_at DESC
		LIMIT 1;
	`
	var t NotificationTemplate
	var varsRaw []byte
	err := r.pool.Raw().QueryRowContext(ctx, query, rtID, eventType).Scan(
		&t.ID, &t.RTID, &t.Name, &t.EventType, &t.Channel, &t.Title, &t.Body, &t.IsActive, &varsRaw, &t.CreatedAt, &t.UpdatedAt, &t.ImageURL,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(varsRaw) > 0 {
		_ = json.Unmarshal(varsRaw, &t.Variables)
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}
	return &t, nil
}

func (r *pgRepository) CreateTemplate(ctx context.Context, rtID string, in CreateTemplateInput, allowedVars []string) (*NotificationTemplate, error) {
	channel := "push"
	if strings.TrimSpace(in.Channel) != "" {
		channel = strings.TrimSpace(in.Channel)
	}
	varsJSON, _ := json.Marshal(allowedVars)

	var imageURL *string
	if in.ImageURL != nil && strings.TrimSpace(*in.ImageURL) != "" {
		trimmed := strings.TrimSpace(*in.ImageURL)
		imageURL = &trimmed
	}

	query := `
		INSERT INTO notification_templates (rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7, NOW(), NOW(), $8)
		RETURNING id, rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at, image_url;
	`
	var t NotificationTemplate
	var varsRaw []byte
	err := r.pool.Raw().QueryRowContext(ctx, query, rtID, in.Name, in.EventType, channel, in.Title, in.Body, varsJSON, imageURL).Scan(
		&t.ID, &t.RTID, &t.Name, &t.EventType, &t.Channel, &t.Title, &t.Body, &t.IsActive, &varsRaw, &t.CreatedAt, &t.UpdatedAt, &t.ImageURL,
	)
	if err != nil {
		return nil, err
	}
	if len(varsRaw) > 0 {
		_ = json.Unmarshal(varsRaw, &t.Variables)
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}
	return &t, nil
}

func (r *pgRepository) UpdateTemplate(ctx context.Context, rtID, id string, in UpdateTemplateInput) (*NotificationTemplate, error) {
	// First fetch existing
	existing, err := r.GetTemplateByID(ctx, rtID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	name := existing.Name
	if strings.TrimSpace(in.Name) != "" {
		name = strings.TrimSpace(in.Name)
	}
	title := existing.Title
	if strings.TrimSpace(in.Title) != "" {
		title = strings.TrimSpace(in.Title)
	}
	body := existing.Body
	if strings.TrimSpace(in.Body) != "" {
		body = strings.TrimSpace(in.Body)
	}
	isActive := existing.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	imageURL := existing.ImageURL
	if in.ImageURL != nil {
		if strings.TrimSpace(*in.ImageURL) == "" {
			imageURL = nil
		} else {
			trimmed := strings.TrimSpace(*in.ImageURL)
			imageURL = &trimmed
		}
	}

	query := `
		UPDATE notification_templates
		SET name = $1, title = $2, body = $3, is_active = $4, image_url = $5, updated_at = NOW()
		WHERE id = $6 AND rt_id = $7
		RETURNING id, rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at, image_url;
	`
	var t NotificationTemplate
	var varsRaw []byte
	err = r.pool.Raw().QueryRowContext(ctx, query, name, title, body, isActive, imageURL, id, rtID).Scan(
		&t.ID, &t.RTID, &t.Name, &t.EventType, &t.Channel, &t.Title, &t.Body, &t.IsActive, &varsRaw, &t.CreatedAt, &t.UpdatedAt, &t.ImageURL,
	)
	if err != nil {
		return nil, err
	}
	if len(varsRaw) > 0 {
		_ = json.Unmarshal(varsRaw, &t.Variables)
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}
	return &t, nil
}

func (r *pgRepository) ToggleTemplateActive(ctx context.Context, rtID, id string) (*NotificationTemplate, error) {
	query := `
		UPDATE notification_templates
		SET is_active = NOT is_active, updated_at = NOW()
		WHERE id = $1 AND rt_id = $2
		RETURNING id, rt_id, name, event_type, channel, title, body, is_active, variables, created_at, updated_at, image_url;
	`
	var t NotificationTemplate
	var varsRaw []byte
	err := r.pool.Raw().QueryRowContext(ctx, query, id, rtID).Scan(
		&t.ID, &t.RTID, &t.Name, &t.EventType, &t.Channel, &t.Title, &t.Body, &t.IsActive, &varsRaw, &t.CreatedAt, &t.UpdatedAt, &t.ImageURL,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(varsRaw) > 0 {
		_ = json.Unmarshal(varsRaw, &t.Variables)
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}
	return &t, nil
}

func (r *pgRepository) DeleteTemplate(ctx context.Context, rtID, id string) error {
	query := `DELETE FROM notification_templates WHERE id = $1 AND rt_id = $2;`
	res, err := r.pool.Raw().ExecContext(ctx, query, id, rtID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *pgRepository) CreateSchedule(ctx context.Context, rtID, userID string, in CreateScheduleInput) (*NotificationSchedule, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("nama jadwal wajib diisi")
	}
	if strings.TrimSpace(in.EventType) == "" {
		return nil, errors.New("jenis event wajib diisi")
	}
	if strings.TrimSpace(in.TemplateID) == "" {
		return nil, errors.New("template wajib dipilih")
	}
	if len(in.SendDays) == 0 {
		return nil, errors.New("minimal satu tanggal pengiriman wajib diisi")
	}
	for _, d := range in.SendDays {
		if d < 1 || d > 31 {
			return nil, fmt.Errorf("tanggal pengiriman %d tidak valid (harus 1-31)", d)
		}
	}
	if in.SendHour < 0 || in.SendHour > 23 {
		return nil, errors.New("jam pengiriman harus antara 0 dan 23")
	}
	if in.SendMinute < 0 || in.SendMinute > 59 {
		return nil, errors.New("menit pengiriman harus antara 0 dan 59")
	}
	if strings.TrimSpace(in.Timezone) == "" {
		in.Timezone = "Asia/Jakarta"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return nil, fmt.Errorf("timezone '%s' tidak valid", in.Timezone)
	}

	daysJSON, err := json.Marshal(in.SendDays)
	if err != nil {
		return nil, fmt.Errorf("marshal send_days: %w", err)
	}

	var s NotificationSchedule
	var daysRaw []byte
	var lastExecutedStatus *string
	var lastExecutedError *string
	var summaryRaw []byte
	var createdBy, updatedBy *string
	query := `
		INSERT INTO notification_schedules
			(rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, created_by, updated_by)
		VALUES ($1, $2, $3, $4, true, $5::jsonb, $6, $7, $8, $9, $10)
		RETURNING id, rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, last_executed_at, last_executed_status, last_executed_error, last_executed_summary, created_at, updated_at, created_by, updated_by
	`
	var userParam any
	if strings.TrimSpace(userID) != "" {
		userParam = strings.TrimSpace(userID)
	}
	err = r.pool.Raw().QueryRowContext(ctx, query, rtID, in.EventType, in.TemplateID, in.Name, daysJSON, in.SendHour, in.SendMinute, in.Timezone, userParam, userParam).Scan(
		&s.ID, &s.RTID, &s.EventType, &s.TemplateID, &s.Name, &s.IsActive, &daysRaw, &s.SendHour, &s.SendMinute, &s.Timezone, &s.LastExecutedAt, &lastExecutedStatus, &lastExecutedError, &summaryRaw, &s.CreatedAt, &s.UpdatedAt, &createdBy, &updatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	if lastExecutedStatus != nil {
		s.LastExecutedStatus = lastExecutedStatus
	}
	if lastExecutedError != nil {
		s.LastExecutedError = lastExecutedError
	}
	if len(summaryRaw) > 0 {
		var sum ScheduleExecutionSummary
		if err := json.Unmarshal(summaryRaw, &sum); err == nil {
			s.LastExecutedSummary = &sum
		}
	}
	if createdBy != nil {
		s.CreatedBy = *createdBy
	}
	if updatedBy != nil {
		s.UpdatedBy = *updatedBy
	}
	s.SendDays = in.SendDays
	return &s, nil
}

func (r *pgRepository) GetSchedule(ctx context.Context, rtID, id string) (*NotificationSchedule, error) {
	query := `
		SELECT id, rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, last_executed_at, last_executed_status, last_executed_error, last_executed_summary, created_at, updated_at, created_by, updated_by
		FROM notification_schedules WHERE id = $1 AND rt_id = $2 LIMIT 1
	`
	return r.scanSchedule(ctx, r.pool.Raw().QueryRowContext(ctx, query, id, rtID))
}

func (r *pgRepository) ListSchedules(ctx context.Context, rtID, eventType string, isActive *bool) ([]*NotificationSchedule, error) {
	query := `
		SELECT id, rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, last_executed_at, last_executed_status, last_executed_error, last_executed_summary, created_at, updated_at, created_by, updated_by
		FROM notification_schedules WHERE rt_id = $1
	`
	args := []any{rtID}
	argIdx := 2
	if strings.TrimSpace(eventType) != "" {
		query += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, strings.TrimSpace(eventType))
		argIdx++
	}
	if isActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *isActive)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Raw().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()

	var list []*NotificationSchedule
	for rows.Next() {
		s, err := r.scanSchedule(ctx, rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *pgRepository) UpdateSchedule(ctx context.Context, rtID, id, userID string, in UpdateScheduleInput) (*NotificationSchedule, error) {
	existing, err := r.GetSchedule(ctx, rtID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, sql.ErrNoRows
	}

	name := existing.Name
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		name = strings.TrimSpace(*in.Name)
	}
	isActive := existing.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	sendDays := existing.SendDays
	if in.SendDays != nil {
		if len(*in.SendDays) == 0 {
			return nil, errors.New("minimal satu tanggal pengiriman wajib diisi")
		}
		for _, d := range *in.SendDays {
			if d < 1 || d > 31 {
				return nil, fmt.Errorf("tanggal pengiriman %d tidak valid (harus 1-31)", d)
			}
		}
		sendDays = *in.SendDays
	}
	sendHour := existing.SendHour
	if in.SendHour != nil {
		if *in.SendHour < 0 || *in.SendHour > 23 {
			return nil, errors.New("jam pengiriman harus antara 0 dan 23")
		}
		sendHour = *in.SendHour
	}
	sendMinute := existing.SendMinute
	if in.SendMinute != nil {
		if *in.SendMinute < 0 || *in.SendMinute > 59 {
			return nil, errors.New("menit pengiriman harus antara 0 dan 59")
		}
		sendMinute = *in.SendMinute
	}
	timezone := existing.Timezone
	if in.Timezone != nil && strings.TrimSpace(*in.Timezone) != "" {
		if _, err := time.LoadLocation(*in.Timezone); err != nil {
			return nil, fmt.Errorf("timezone '%s' tidak valid", *in.Timezone)
		}
		timezone = *in.Timezone
	}

	daysJSON, err := json.Marshal(sendDays)
	if err != nil {
		return nil, fmt.Errorf("marshal send_days: %w", err)
	}

	var s NotificationSchedule
	var daysRaw []byte
	var lastExecutedStatus *string
	var lastExecutedError *string
	var summaryRaw []byte
	var createdBy, updatedBy *string
	query := `
		UPDATE notification_schedules
		SET name = $1, is_active = $2, send_days = $3::jsonb, send_hour = $4, send_minute = $5, timezone = $6, updated_at = CURRENT_TIMESTAMP, updated_by = $7
		WHERE id = $8 AND rt_id = $9
		RETURNING id, rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, last_executed_at, last_executed_status, last_executed_error, last_executed_summary, created_at, updated_at, created_by, updated_by
	`
	var userParam any
	if strings.TrimSpace(userID) != "" {
		userParam = strings.TrimSpace(userID)
	}
	err = r.pool.Raw().QueryRowContext(ctx, query, name, isActive, daysJSON, sendHour, sendMinute, timezone, userParam, id, rtID).Scan(
		&s.ID, &s.RTID, &s.EventType, &s.TemplateID, &s.Name, &s.IsActive, &daysRaw, &s.SendHour, &s.SendMinute, &s.Timezone, &s.LastExecutedAt, &lastExecutedStatus, &lastExecutedError, &summaryRaw, &s.CreatedAt, &s.UpdatedAt, &createdBy, &updatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("update schedule: %w", err)
	}
	if len(daysRaw) > 0 {
		_ = json.Unmarshal(daysRaw, &s.SendDays)
	}
	if s.SendDays == nil {
		s.SendDays = sendDays
	}
	if lastExecutedStatus != nil {
		s.LastExecutedStatus = lastExecutedStatus
	}
	if lastExecutedError != nil {
		s.LastExecutedError = lastExecutedError
	}
	if len(summaryRaw) > 0 {
		var sum ScheduleExecutionSummary
		if err := json.Unmarshal(summaryRaw, &sum); err == nil {
			s.LastExecutedSummary = &sum
		}
	}
	if createdBy != nil {
		s.CreatedBy = *createdBy
	}
	if updatedBy != nil {
		s.UpdatedBy = *updatedBy
	}
	return &s, nil
}

func (r *pgRepository) DeleteSchedule(ctx context.Context, rtID, id string) error {
	query := `DELETE FROM notification_schedules WHERE id = $1 AND rt_id = $2;`
	res, err := r.pool.Raw().ExecContext(ctx, query, id, rtID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *pgRepository) ToggleScheduleActive(ctx context.Context, rtID, id, userID string) (*NotificationSchedule, error) {
	query := `
		UPDATE notification_schedules
		SET is_active = NOT is_active, updated_at = CURRENT_TIMESTAMP, updated_by = $1
		WHERE id = $2 AND rt_id = $3
		RETURNING id, rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, last_executed_at, last_executed_status, last_executed_error, last_executed_summary, created_at, updated_at, created_by, updated_by
	`
	var s NotificationSchedule
	var daysRaw []byte
	var lastExecutedStatus *string
	var lastExecutedError *string
	var summaryRaw []byte
	var createdBy, updatedBy *string
	var userParam any
	if strings.TrimSpace(userID) != "" {
		userParam = strings.TrimSpace(userID)
	}
	err := r.pool.Raw().QueryRowContext(ctx, query, userParam, id, rtID).Scan(
		&s.ID, &s.RTID, &s.EventType, &s.TemplateID, &s.Name, &s.IsActive, &daysRaw, &s.SendHour, &s.SendMinute, &s.Timezone, &s.LastExecutedAt, &lastExecutedStatus, &lastExecutedError, &summaryRaw, &s.CreatedAt, &s.UpdatedAt, &createdBy, &updatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("toggle schedule: %w", err)
	}
	if len(daysRaw) > 0 {
		_ = json.Unmarshal(daysRaw, &s.SendDays)
	}
	if s.SendDays == nil {
		s.SendDays = []int{}
	}
	if lastExecutedStatus != nil {
		s.LastExecutedStatus = lastExecutedStatus
	}
	if lastExecutedError != nil {
		s.LastExecutedError = lastExecutedError
	}
	if len(summaryRaw) > 0 {
		var sum ScheduleExecutionSummary
		if err := json.Unmarshal(summaryRaw, &sum); err == nil {
			s.LastExecutedSummary = &sum
		}
	}
	if createdBy != nil {
		s.CreatedBy = *createdBy
	}
	if updatedBy != nil {
		s.UpdatedBy = *updatedBy
	}
	return &s, nil
}

func (r *pgRepository) GetDueSchedules(ctx context.Context, now time.Time) ([]*NotificationSchedule, error) {
	query := `
		SELECT id, rt_id, event_type, template_id, name, is_active, send_days, send_hour, send_minute, timezone, last_executed_at, last_executed_status, last_executed_error, last_executed_summary, created_at, updated_at, created_by, updated_by
		FROM notification_schedules WHERE is_active = true
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get due schedules: %w", err)
	}
	defer rows.Close()

	// Pre-load completed executions across recent days (window of 2 days covers any timezone differences)
	fromDate := now.AddDate(0, 0, -2).Format("2006-01-02")
	executedSet := make(map[string]bool)

	execRows, execErr := r.pool.Raw().QueryContext(ctx, `
		SELECT schedule_id, to_char(exec_date, 'YYYY-MM-DD'), config_hash
		FROM notification_schedule_executions
		WHERE exec_date >= $1::date
		  AND status IN ('success', 'partial', 'no_recipients', 'no_device', 'failed')
	`, fromDate)
	if execErr == nil {
		defer execRows.Close()
		for execRows.Next() {
			var sid, dt, hash string
			if err := execRows.Scan(&sid, &dt, &hash); err == nil {
				executedSet[sid+":"+dt+":"+hash] = true
			}
		}
	}

	// Also index recent delivery logs with c_<config_hash>
	logRows, logErr := r.pool.Raw().QueryContext(ctx, `
		SELECT DISTINCT
			split_part(dedup_key, ':', 2) AS schedule_id,
			split_part(dedup_key, ':', 3) AS exec_date,
			CASE
				WHEN split_part(dedup_key, ':', 4) LIKE 'c_%'
				THEN substring(split_part(dedup_key, ':', 4) FROM 3)
				ELSE ''
			END AS config_hash
		FROM notification_logs
		WHERE dedup_key LIKE 'schedule:%'
		  AND created_at >= NOW() - INTERVAL '3 days'
	`)
	if logErr == nil {
		defer logRows.Close()
		for logRows.Next() {
			var sid, dt, hash string
			if err := logRows.Scan(&sid, &dt, &hash); err == nil && hash != "" {
				executedSet[sid+":"+dt+":"+hash] = true
			}
		}
	}

	var list []*NotificationSchedule
	for rows.Next() {
		s, err := r.scanSchedule(ctx, rows)
		if err != nil {
			return nil, err
		}
		if r.shouldExecuteNow(s, now, executedSet) {
			list = append(list, s)
		}
	}
	return list, rows.Err()
}

func isDayDue(sendDays []int, local time.Time) bool {
	day := local.Day()
	lastDayOfMonth := time.Date(local.Year(), local.Month()+1, 0, 0, 0, 0, 0, local.Location()).Day()

	for _, d := range sendDays {
		if d == day {
			return true
		}
		if d > lastDayOfMonth && day == lastDayOfMonth {
			return true
		}
	}
	return false
}

// IsDayDueForTesting exports isDayDue for automated scheduler regression test scenarios.
func IsDayDueForTesting(sendDays []int, local time.Time) bool {
	return isDayDue(sendDays, local)
}

func (r *pgRepository) shouldExecuteNow(s *NotificationSchedule, now time.Time, executedSet ...map[string]bool) bool {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		loc = time.FixedZone(s.Timezone, 7*3600) // default WIB
	}
	local := now.In(loc)
	localDate := local.Format("2006-01-02")
	configHash := ComputeScheduleConfigHashFromSchedule(s)

	var set map[string]bool
	if len(executedSet) > 0 {
		set = executedSet[0]
	}

	if set != nil {
		if set[s.ID+":"+localDate+":"+configHash] {
			return false
		}
	} else if r != nil && r.pool != nil {
		done, _ := r.IsScheduleExecutionDone(context.Background(), s.ID, localDate, configHash)
		if done {
			return false
		}
	}

	if !isDayDue(s.SendDays, local) {
		return false
	}

	// The scheduled moment today in the schedule's own timezone. If that moment
	// has already passed (or is now), the schedule is due for execution today.
	// Backend restarts after the scheduled time (catch-up) are therefore covered:
	// a 08:00 schedule will still fire when the backend comes back at 08:15.
	scheduledToday := time.Date(local.Year(), local.Month(), local.Day(),
		s.SendHour, s.SendMinute, 0, 0, loc)

	return !local.Before(scheduledToday)
}

func (r *pgRepository) GetIuranReminderRecipients(ctx context.Context, rtID string, asOf time.Time) ([]*IuranReminderRecipient, error) {
	query := `
		WITH eligible AS (
		    SELECT b.id AS bill_id, b.amount::text AS amount, b.period, b.due_date, b.rt_id,
		           u.id AS user_id, COALESCE(u.full_name, '') AS user_full_name
		    FROM bills b
		    JOIN household_occupancies ho ON b.household_occupancy_id = ho.id AND ho.end_date IS NULL
		    JOIN residency_periods rp ON rp.household_occupancy_id = ho.id AND rp.end_date IS NULL
		    JOIN residents r ON r.id = rp.resident_id AND r.is_active = true
		    JOIN users u ON u.id = r.user_id AND u.is_active = true
		    JOIN user_rt_memberships urm ON urm.user_id = u.id AND urm.rt_id = b.rt_id AND urm.is_active = true
		    JOIN dues d ON b.due_id = d.id AND d.is_active = true AND d.is_voluntary = false
		    WHERE b.rt_id = $1
		      AND b.is_active = true
		      AND b.is_obligatory = true
		      AND b.status IN ('unpaid', 'partial')
		      AND ho.start_date <= $2::date
		      AND (b.due_date IS NULL OR ho.start_date <= b.due_date)
		      AND (LENGTH(b.period) != 7 OR to_char(ho.start_date, 'YYYY-MM') <= b.period)
		      AND (b.amount::numeric - COALESCE((
		            SELECT SUM(COALESCE(pa.amount, p.amount))
		            FROM payments p
		            LEFT JOIN payment_allocations pa ON pa.payment_id = p.id AND pa.bill_id = b.id
		            WHERE (p.bill_id = b.id OR pa.bill_id = b.id) AND p.status = 'APPROVED'
		        ), 0)) > 0
		)
		SELECT DISTINCT ON (user_id) bill_id, user_id, user_full_name, rt_id, period, amount, due_date
		FROM eligible
		ORDER BY user_id, due_date ASC NULLS LAST, bill_id
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query, rtID, asOf.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("get iuran reminder recipients: %w", err)
	}
	defer rows.Close()

	var list []*IuranReminderRecipient
	for rows.Next() {
		var rec IuranReminderRecipient
		if err := rows.Scan(&rec.BillID, &rec.UserID, &rec.UserFullName, &rec.RTID, &rec.Period, &rec.Amount, &rec.DueDate); err != nil {
			return nil, err
		}
		list = append(list, &rec)
	}
	return list, rows.Err()
}

func (r *pgRepository) GetRTUserBillsForEvaluation(ctx context.Context, rtID string) ([]finance.BillEvaluationInput, error) {
	return finance.GetRTUserBillsForEvaluation(ctx, r.pool.Raw(), rtID)
}

func (r *pgRepository) IsReminderAlreadySentForPeriod(ctx context.Context, rtID, userID, eventType, periodKey, billID string) (bool, error) {
	var exists bool
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM notification_logs
			WHERE rt_id = $1
			  AND user_id = $2
			  AND event_type = $3
			  AND status = 'SENT'
			  AND (
				payload->>'reminder_period' = $4
				OR split_part(dedup_key, ':', 6) = $4
				OR split_part(dedup_key, ':', 7) = $4
			  )
			  AND (
				payload->>'bill_id' = $5
				OR ($5 = ANY(string_to_array(COALESCE(payload->>'bill_ids', ''), ',')))
				OR ($5 = ANY(string_to_array(split_part(dedup_key, ':', 7), ',')))
				OR ($5 = ANY(string_to_array(split_part(dedup_key, ':', 8), ',')))
			  )
			LIMIT 1
		)
	`, rtID, userID, eventType, periodKey, billID).Scan(&exists)
	if err != nil {
		if isTableMissingErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("check reminder already sent for period: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) UpdateScheduleExecuted(ctx context.Context, scheduleID, rtID string, at time.Time, status string) error {
	return r.UpdateScheduleExecutedDetailed(ctx, scheduleID, rtID, at, status, nil, nil)
}

func (r *pgRepository) RecordScheduleExecution(
	ctx context.Context,
	scheduleID, rtID string,
	execDate time.Time,
	configHash, status string,
	errorDetail *string,
	summary *ScheduleExecutionSummary,
) error {
	var summaryJSON []byte
	if summary != nil {
		summaryJSON, _ = json.Marshal(summary)
	} else {
		summaryJSON = []byte("{}")
	}

	query := `
		INSERT INTO notification_schedule_executions (
			schedule_id, rt_id, exec_date, config_hash, status, error_detail, summary, executed_at
		) VALUES (
			$1, $2, $3::date, $4, $5, $6, $7, $8
		)
		ON CONFLICT (schedule_id, exec_date, config_hash)
		DO UPDATE SET
			status = EXCLUDED.status,
			error_detail = EXCLUDED.error_detail,
			summary = EXCLUDED.summary,
			executed_at = EXCLUDED.executed_at,
			updated_at = NOW();
	`
	_, err := r.pool.Raw().ExecContext(
		ctx, query,
		scheduleID, rtID, execDate.Format("2006-01-02"), configHash,
		status, errorDetail, summaryJSON, execDate,
	)
	if err != nil {
		if isTableMissingErr(err) {
			return nil
		}
		return fmt.Errorf("record schedule execution: %w", err)
	}
	return nil
}

func (r *pgRepository) IsScheduleExecutionDone(
	ctx context.Context,
	scheduleID, execDate, configHash string,
) (bool, error) {
	var exists bool
	// Check notification_schedule_executions table first
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_schedule_executions
			WHERE schedule_id = $1 AND exec_date = $2::date AND config_hash = $3
			  AND status IN ('success', 'partial', 'no_recipients', 'no_device', 'failed')
			LIMIT 1
		)
	`, scheduleID, execDate, configHash).Scan(&exists)
	if err == nil && exists {
		return true, nil
	}

	// Fallback check against notification_logs with c_<config_hash>
	pattern := fmt.Sprintf("schedule:%s:%s:c_%s:%%", scheduleID, execDate, configHash)
	err = r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_logs
			WHERE dedup_key LIKE $1 LIMIT 1
		)
	`, pattern).Scan(&exists)
	if err != nil {
		if isTableMissingErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("check schedule execution done: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) UpdateScheduleExecutedDetailed(
	ctx context.Context,
	scheduleID, rtID string,
	at time.Time,
	status string,
	errorDetail *string,
	summary *ScheduleExecutionSummary,
	configHash ...string,
) error {
	var summaryJSON []byte
	if summary != nil {
		summaryJSON, _ = json.Marshal(summary)
	}
	query := `
		UPDATE notification_schedules
		SET last_executed_at = $1,
		    last_executed_status = $2,
		    last_executed_error = $3,
		    last_executed_summary = $4,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $5 AND rt_id = $6
	`
	res, err := r.pool.Raw().ExecContext(ctx, query, at, status, errorDetail, summaryJSON, scheduleID, rtID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	if len(configHash) > 0 && configHash[0] != "" {
		_ = r.RecordScheduleExecution(ctx, scheduleID, rtID, at, configHash[0], status, errorDetail, summary)
	}
	return nil
}

func (r *pgRepository) IsRecipientAlreadySentToday(ctx context.Context, dedupKey string) (bool, error) {
	var exists bool
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_logs
			WHERE dedup_key = $1 AND status = 'SENT'
			LIMIT 1
		)
	`, dedupKey).Scan(&exists)
	if err != nil {
		if isTableMissingErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("check recipient already sent today: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) AcquireConn(ctx context.Context) (*sql.Conn, error) {
	return r.pool.Acquire(ctx)
}

func (r *pgRepository) TryAdvisoryLockConn(conn *sql.Conn, ctx context.Context, scheduleID, execDate string) (bool, error) {
	var ok bool
	err := conn.QueryRowContext(ctx,
		`SELECT pg_try_advisory_lock(hashtext($1), hashtext($2))`,
		scheduleID, execDate,
	).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("try advisory lock: %w", err)
	}
	return ok, nil
}

func (r *pgRepository) ReleaseAdvisoryLockConn(conn *sql.Conn, ctx context.Context, scheduleID, execDate string) error {
	_, err := conn.ExecContext(ctx,
		`SELECT pg_advisory_unlock(hashtext($1), hashtext($2))`,
		scheduleID, execDate,
	)
	if err != nil {
		return fmt.Errorf("release advisory lock: %w", err)
	}
	return nil
}

func (r *pgRepository) IsScheduleExecutedToday(ctx context.Context, scheduleID, execDate string, configHash ...string) (bool, error) {
	if len(configHash) > 0 && configHash[0] != "" {
		return r.IsScheduleExecutionDone(ctx, scheduleID, execDate, configHash[0])
	}

	var exists bool
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_logs
			WHERE dedup_key LIKE $1 LIMIT 1
		)`,
		"schedule:"+scheduleID+":"+execDate+":%",
	).Scan(&exists)
	if err != nil {
		if isTableMissingErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("check schedule executed today: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) RecordScheduleDelivery(ctx context.Context, rtID, userID, eventType, dedupKey, title, body string, payload map[string]string, status, failureReason string) (string, error) {
	payloadJSON, _ := json.Marshal(payload)
	var uidParam *string
	if strings.TrimSpace(userID) != "" {
		trimmed := strings.TrimSpace(userID)
		uidParam = &trimmed
	}
	var imageURL *string
	if u, ok := payload["image_url"]; ok && strings.TrimSpace(u) != "" {
		trimmed := strings.TrimSpace(u)
		imageURL = &trimmed
	}
	query := `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, payload, status, image_url, failure_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''))
		ON CONFLICT (dedup_key) DO UPDATE
		SET status = EXCLUDED.status,
		    failure_reason = EXCLUDED.failure_reason,
		    title = EXCLUDED.title,
		    body = EXCLUDED.body,
		    payload = EXCLUDED.payload
		WHERE notification_logs.status != 'SENT' AND EXCLUDED.status = 'SENT'
		RETURNING id;
	`
	var id string
	err := r.pool.Raw().QueryRowContext(ctx, query, rtID, uidParam, eventType, dedupKey, title, body, payloadJSON, status, imageURL, failureReason).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Row already exists -> deduplicated, no id to return.
			return "", nil
		}
		if isTableMissingErr(err) {
			return "", fmt.Errorf("notification_logs table does not exist (migration pending): %w", err)
		}
		return "", err
	}
	return id, nil
}

// ListSchedulerHistory returns aggregated scheduler execution history for the
// tenant RT. One row per (schedule, execution date), not per recipient.
// Only logs with dedup_key prefixed "schedule:" are included.
// Dedup key format: schedule:<schedule_id>:<exec_date>:<user_id>
func (r *pgRepository) ListSchedulerHistory(ctx context.Context, rtID string, page, pageSize int) ([]*NotificationSchedulerHistory, int, error) {
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

	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT 1
			FROM notification_logs
			WHERE rt_id = $1
			  AND dedup_key LIKE 'schedule:%'
			GROUP BY
				split_part(dedup_key, ':', 2),
				split_part(dedup_key, ':', 3),
				(CASE WHEN split_part(dedup_key, ':', 4) LIKE 'c_%' THEN substring(split_part(dedup_key, ':', 4) FROM 3) ELSE '' END)
		) sub
	`
	var total int
	err := r.pool.Raw().QueryRowContext(ctx, countQuery, rtID).Scan(&total)
	if err != nil {
		if isTableMissingErr(err) {
			return []*NotificationSchedulerHistory{}, 0, nil
		}
		return nil, 0, err
	}

	query := `
		WITH parsed AS (
			SELECT
				user_id,
				status,
				created_at,
				event_type,
				split_part(dedup_key, ':', 2) AS schedule_id,
				split_part(dedup_key, ':', 3) AS exec_date,
				CASE
					WHEN split_part(dedup_key, ':', 4) LIKE 'c_%'
					THEN substring(split_part(dedup_key, ':', 4) FROM 3)
					ELSE ''
				END AS config_hash
			FROM notification_logs
			WHERE rt_id = $1
			  AND dedup_key LIKE 'schedule:%'
		),
		grouped AS (
			SELECT
				schedule_id,
				exec_date,
				config_hash,
				COUNT(DISTINCT user_id) AS total_recipients,
				COUNT(*) FILTER (WHERE status = 'SENT') AS sent_count,
				COUNT(*) FILTER (WHERE status = 'FAILED') AS failed_count,
				MIN(created_at) AS execution_time,
				MAX(event_type) AS event_type
			FROM parsed
			GROUP BY schedule_id, exec_date, config_hash
		)
		SELECT
			g.schedule_id,
			g.exec_date,
			g.config_hash,
			g.total_recipients,
			g.sent_count,
			g.failed_count,
			g.execution_time,
			COALESCE(s.name, '') AS schedule_name,
			COALESCE(s.event_type, g.event_type) AS event_type
		FROM grouped g
		LEFT JOIN notification_schedules s
			ON s.id::text = g.schedule_id AND s.rt_id = $1
		ORDER BY g.execution_time DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query, rtID, pageSize, offset)
	if err != nil {
		if isTableMissingErr(err) {
			return []*NotificationSchedulerHistory{}, 0, nil
		}
		return nil, 0, err
	}
	defer rows.Close()

	var list []*NotificationSchedulerHistory
	for rows.Next() {
		var h NotificationSchedulerHistory
		if err := rows.Scan(
			&h.ScheduleID,
			&h.ExecDate,
			&h.ConfigHash,
			&h.TotalRecipients,
			&h.SentCount,
			&h.FailedCount,
			&h.ExecutionTime,
			&h.ScheduleName,
			&h.EventType,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, &h)
	}
	if list == nil {
		list = []*NotificationSchedulerHistory{}
	}
	return list, total, rows.Err()
}

// GetSchedulerRecipientDetail returns individual recipient delivery results
// for a single scheduler execution identified by schedule ID, exec date, and optional config hash.
// Dedup key format: schedule:<schedule_id>:<exec_date>:c_<config_hash>:<user_id> (or legacy schedule:<schedule_id>:<exec_date>:<user_id>)
func (r *pgRepository) GetSchedulerRecipientDetail(ctx context.Context, rtID, scheduleID, execDate string, configHash ...string) ([]*NotificationSchedulerRecipientDetail, error) {
	var prefix string
	if len(configHash) > 0 && configHash[0] != "" {
		prefix = "schedule:" + scheduleID + ":" + execDate + ":c_" + configHash[0] + ":"
	} else if strings.Contains(execDate, ":c_") {
		prefix = "schedule:" + scheduleID + ":" + execDate + ":"
	} else {
		prefix = "schedule:" + scheduleID + ":" + execDate + ":"
	}
	query := `
		SELECT user_id, status, title, created_at
		FROM notification_logs
		WHERE rt_id = $1
		  AND dedup_key LIKE $2
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Raw().QueryContext(ctx, query, rtID, prefix+"%")
	if err != nil {
		if isTableMissingErr(err) {
			return []*NotificationSchedulerRecipientDetail{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var list []*NotificationSchedulerRecipientDetail
	for rows.Next() {
		var d NotificationSchedulerRecipientDetail
		if err := rows.Scan(&d.UserID, &d.Status, &d.Title, &d.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &d)
	}
	if list == nil {
		list = []*NotificationSchedulerRecipientDetail{}
	}
	return list, rows.Err()
}

func (r *pgRepository) scanSchedule(ctx context.Context, row scannable) (*NotificationSchedule, error) {
	var s NotificationSchedule
	var daysRaw []byte
	var lastExecutedStatus *string
	var lastExecutedError *string
	var summaryRaw []byte
	var createdBy, updatedBy *string
	err := row.Scan(
		&s.ID, &s.RTID, &s.EventType, &s.TemplateID, &s.Name, &s.IsActive,
		&daysRaw, &s.SendHour, &s.SendMinute, &s.Timezone,
		&s.LastExecutedAt, &lastExecutedStatus,
		&lastExecutedError, &summaryRaw,
		&s.CreatedAt, &s.UpdatedAt, &createdBy, &updatedBy,
	)
	if err != nil {
		return nil, err
	}
	if len(daysRaw) > 0 {
		_ = json.Unmarshal(daysRaw, &s.SendDays)
	}
	if s.SendDays == nil {
		s.SendDays = []int{}
	}
	if lastExecutedStatus != nil {
		s.LastExecutedStatus = lastExecutedStatus
	}
	if lastExecutedError != nil {
		s.LastExecutedError = lastExecutedError
	}
	if len(summaryRaw) > 0 {
		var sum ScheduleExecutionSummary
		if err := json.Unmarshal(summaryRaw, &sum); err == nil {
			s.LastExecutedSummary = &sum
		}
	}
	if createdBy != nil {
		s.CreatedBy = *createdBy
	}
	if updatedBy != nil {
		s.UpdatedBy = *updatedBy
	}
	return &s, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func (r *pgRepository) ListLogs(ctx context.Context, rtID, userID, eventType, status string, isOfficer bool, offset, limit int) ([]NotificationLogEntry, int, error) {
	var countQuery strings.Builder
	var query strings.Builder
	var args []any

	countQuery.WriteString(`
		SELECT COUNT(*)
		FROM notification_logs nl
		LEFT JOIN notification_reads nr ON nr.notification_id = nl.id AND nr.user_id = $1
		WHERE nl.rt_id = $2
	`)
	query.WriteString(`
		SELECT
			nl.id, nl.rt_id, nl.user_id, nl.event_type, nl.dedup_key, nl.title, nl.body,
			CASE
				WHEN nl.dedup_key LIKE 'schedule:%' AND nl.status = 'FAILED' AND nl.failure_reason IS NULL THEN 'UNKNOWN'
				WHEN nl.dedup_key LIKE 'schedule:%' AND nl.status = 'PENDING' THEN 'UNKNOWN'
				ELSE nl.status
			END AS status,
			nl.created_at, nl.image_url,
			(nr.read_at IS NOT NULL) AS is_read,
			nr.read_at, nl.payload
		FROM notification_logs nl
		LEFT JOIN notification_reads nr ON nr.notification_id = nl.id AND nr.user_id = $1
		WHERE nl.rt_id = $2
	`)
	args = append(args, userID, rtID)
	argIdx := 3

	personalScope := ` AND nl.user_id = $1 AND nl.user_id IS NOT NULL AND (nl.event_type NOT IN ('manual_push', 'waste_truck_arrival') OR nl.dedup_key LIKE 'recipient:%') AND nl.dedup_key NOT LIKE 'manual_push:%' AND nl.dedup_key NOT LIKE 'waste_truck:%'`
	countQuery.WriteString(personalScope)
	query.WriteString(personalScope)

	if strings.TrimSpace(eventType) != "" {
		countQuery.WriteString(fmt.Sprintf(" AND nl.event_type = $%d", argIdx))
		query.WriteString(fmt.Sprintf(" AND nl.event_type = $%d", argIdx))
		args = append(args, strings.TrimSpace(eventType))
		argIdx++
	}

	status = strings.ToLower(strings.TrimSpace(status))
	if status == "unread" {
		countQuery.WriteString(" AND nr.read_at IS NULL")
		query.WriteString(" AND nr.read_at IS NULL")
	} else if status == "read" {
		countQuery.WriteString(" AND nr.read_at IS NOT NULL")
		query.WriteString(" AND nr.read_at IS NOT NULL")
	}

	var total int
	err := r.pool.Raw().QueryRowContext(ctx, countQuery.String(), args...).Scan(&total)
	if err != nil {
		if isTableMissingErr(err) {
			return []NotificationLogEntry{}, 0, nil
		}
		return nil, 0, err
	}

	query.WriteString(fmt.Sprintf(" ORDER BY nl.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1))
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.pool.Raw().QueryContext(ctx, query.String(), queryArgs...)
	if err != nil {
		if isTableMissingErr(err) {
			return []NotificationLogEntry{}, 0, nil
		}
		return nil, 0, err
	}
	defer rows.Close()

	var logs []NotificationLogEntry
	for rows.Next() {
		var entry NotificationLogEntry
		var isRead sql.NullBool
		var readAt sql.NullTime
		var payloadBytes []byte
		if err := rows.Scan(
			&entry.ID, &entry.RTID, &entry.UserID, &entry.EventType, &entry.DedupKey,
			&entry.Title, &entry.Body, &entry.Status, &entry.CreatedAt, &entry.ImageURL,
			&isRead, &readAt, &payloadBytes,
		); err != nil {
			return nil, 0, err
		}
		entry.IsRead = isRead.Valid && isRead.Bool
		if readAt.Valid {
			t := readAt.Time
			entry.ReadAt = &t
		}
		if len(payloadBytes) > 0 {
			_ = json.Unmarshal(payloadBytes, &entry.Payload)
		}
		entry.ActivityRelation = "received_by_me"
		logs = append(logs, entry)
	}
	if logs == nil {
		logs = []NotificationLogEntry{}
	}
	return logs, total, rows.Err()
}

func (r *pgRepository) ListManagementHistory(
	ctx context.Context,
	rtID, userID, category string,
	offset, limit int,
) ([]NotificationLogEntry, int, error) {
	var activityQuery string
	var countArgs []any
	var queryArgs []any

	switch category {
	case HistoryCategorySystem:
		activityQuery = `
			WITH grouped AS (
				SELECT
					(array_agg(id ORDER BY created_at, id))[1] AS representative_id,
					COUNT(*)::int AS recipients_count,
					COUNT(*) FILTER (WHERE status IN ('SENT', 'PARTIAL'))::int AS sent_count,
					COUNT(*) FILTER (WHERE status = 'PARTIAL' OR
						(status = 'FAILED' AND failure_reason IS NOT NULL))::int AS failed_count,
					MIN(created_at) AS activity_time
				FROM notification_logs
				WHERE rt_id = $1 AND dedup_key LIKE 'schedule:%'
				GROUP BY
					split_part(dedup_key, ':', 2),
					split_part(dedup_key, ':', 3),
					(CASE WHEN split_part(dedup_key, ':', 4) LIKE 'c_%' THEN substring(split_part(dedup_key, ':', 4) FROM 3) ELSE '' END)
			)
			SELECT
				nl.id, nl.rt_id, NULL::uuid, nl.event_type, nl.dedup_key, nl.title, nl.body,
				CASE
					WHEN g.sent_count > 0 AND g.failed_count > 0 THEN 'PARTIAL'
					WHEN g.sent_count > 0 THEN 'SENT'
					WHEN g.failed_count > 0 THEN 'FAILED'
					ELSE 'UNKNOWN'
				END,
				g.activity_time AS created_at, nl.image_url, FALSE, NULL::timestamptz, nl.payload,
				'system', 'system', g.recipients_count, g.sent_count, g.failed_count, TRUE
			FROM grouped g
			JOIN notification_logs nl ON nl.id = g.representative_id
		`
		countArgs = []any{rtID}
		queryArgs = []any{rtID, limit, offset}
	case HistoryCategoryMine:
		activityQuery = `
			WITH manual AS (
				SELECT
					nl.id, nl.rt_id, NULL::uuid AS user_id, nl.event_type, nl.dedup_key,
					nl.title, nl.body, 'RECORDED'::text AS status, nl.created_at, nl.image_url, nl.payload,
					0::int AS recipients_count,
					0::int AS sent_count,
					0::int AS failed_count,
					FALSE AS delivery_counts_available
				FROM notification_logs nl
				WHERE nl.rt_id = $1
				  AND nl.user_id = $2
				  AND nl.event_type = 'manual_push'
				  AND nl.dedup_key LIKE 'manual_push:%'
			),
			waste_truck_sent AS (
				SELECT
				  nl.id, nl.rt_id, NULL::uuid AS user_id, nl.event_type, nl.dedup_key,
				  nl.title, nl.body, nl.status, nl.created_at, nl.image_url, nl.payload,
				  0::int AS recipients_count, 0::int AS sent_count, 0::int AS failed_count,
				  FALSE AS delivery_counts_available
				FROM notification_logs nl
				WHERE nl.rt_id = $1
				  AND nl.sender_user_id = $2
				  AND nl.event_type = 'waste_truck_arrival'
				  AND nl.dedup_key LIKE 'waste_truck:%'
			),
			received_notifications AS (
				SELECT
				  nl.id, nl.rt_id, nl.user_id, nl.event_type, nl.dedup_key,
				  nl.title, nl.body, nl.status, nl.created_at, nl.image_url, nl.payload,
				  0::int AS recipients_count, 0::int AS sent_count, 0::int AS failed_count,
				  FALSE AS delivery_counts_available
				FROM notification_logs nl
				WHERE nl.rt_id = $1
				  AND nl.user_id = $2
				  AND (
				    (
				      nl.event_type IN ('manual_push', 'waste_truck_arrival')
				      AND left(nl.dedup_key, length('recipient:' || nl.event_type || ':')) =
				          'recipient:' || nl.event_type || ':'
				      AND right(nl.dedup_key, length($2::text) + 1) = ':' || $2::text
				    )
				    OR (
				      nl.event_type NOT IN ('manual_push', 'waste_truck_arrival')
				      AND nl.dedup_key NOT LIKE 'manual_push:%'
				      AND nl.dedup_key NOT LIKE 'waste_truck:%'
				    )
				  )
			),
			payment_groups AS (
				SELECT
					(array_agg(nl.id ORDER BY nl.created_at, nl.id))[1] AS representative_id,
					p.id AS payment_id,
					p.status AS payment_status,
					MIN(nl.created_at) AS activity_time
				FROM notification_logs nl
				JOIN payments p
				  ON p.id::text = nl.payload->>'payment_id'
				 AND p.rt_id = nl.rt_id
				WHERE nl.rt_id = $1
				  AND nl.event_type = 'iuran_payment_verified'
				  AND p.verified_by = $2
				GROUP BY p.id, p.status
			),
			payment_activity AS (
				SELECT
					nl.id, nl.rt_id, NULL::uuid AS user_id, nl.event_type,
					'payment_action:' || pg.payment_id::text || ':' || pg.payment_status AS dedup_key,
					nl.title, nl.body,
					pg.payment_status AS status,
					pg.activity_time AS created_at, nl.image_url, nl.payload,
					0::int AS recipients_count, 0::int AS sent_count, 0::int AS failed_count,
					FALSE AS delivery_counts_available
				FROM payment_groups pg
				JOIN notification_logs nl ON nl.id = pg.representative_id
			)
			SELECT
				id, rt_id, user_id, event_type, dedup_key, title, body, status, created_at,
				image_url, FALSE, NULL::timestamptz, payload, 'mine', 'sent_by_me',
				recipients_count, sent_count, failed_count, delivery_counts_available
			FROM manual
			UNION ALL
			SELECT
				id, rt_id, user_id, event_type, dedup_key, title, body, status, created_at,
				image_url, FALSE, NULL::timestamptz, payload, 'mine', 'sent_by_me',
				recipients_count, sent_count, failed_count, delivery_counts_available
			FROM waste_truck_sent
			UNION ALL
			SELECT
				id, rt_id, user_id, event_type, dedup_key, title, body, status, created_at,
				image_url, FALSE, NULL::timestamptz, payload, 'mine', 'received_by_me',
				recipients_count, sent_count, failed_count, delivery_counts_available
			FROM received_notifications
			UNION ALL
			SELECT
				id, rt_id, user_id, event_type, dedup_key, title, body, status, created_at,
				image_url, FALSE, NULL::timestamptz, payload, 'mine', 'performed_by_me',
				recipients_count, sent_count, failed_count, delivery_counts_available
			FROM payment_activity
		`
		countArgs = []any{rtID, userID}
		queryArgs = []any{rtID, userID, limit, offset}
	default:
		return nil, 0, fmt.Errorf("unsupported notification history category %q", category)
	}

	var total int
	if err := r.pool.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (`+activityQuery+`) AS activities`, countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Raw().QueryContext(ctx,
		`SELECT * FROM (`+activityQuery+`) AS activities ORDER BY created_at DESC, id DESC LIMIT $`+
			fmt.Sprint(len(countArgs)+1)+` OFFSET $`+fmt.Sprint(len(countArgs)+2),
		queryArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := make([]NotificationLogEntry, 0)
	for rows.Next() {
		var entry NotificationLogEntry
		var userID sql.NullString
		var isRead sql.NullBool
		var readAt sql.NullTime
		var payload []byte
		if err := rows.Scan(
			&entry.ID, &entry.RTID, &userID, &entry.EventType, &entry.DedupKey,
			&entry.Title, &entry.Body, &entry.Status, &entry.CreatedAt, &entry.ImageURL,
			&isRead, &readAt, &payload, &entry.HistoryCategory, &entry.ActivityRelation,
			&entry.RecipientsCount, &entry.SentCount, &entry.FailedCount,
			&entry.DeliveryCountsAvailable,
		); err != nil {
			return nil, 0, err
		}
		if userID.Valid {
			entry.UserID = &userID.String
		}
		entry.IsRead = isRead.Valid && isRead.Bool
		if readAt.Valid {
			readAtValue := readAt.Time
			entry.ReadAt = &readAtValue
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &entry.Payload); err != nil {
				return nil, 0, fmt.Errorf("decode notification history payload: %w", err)
			}
		}
		entries = append(entries, entry)
	}
	return entries, total, rows.Err()
}

func (r *pgRepository) MarkAsRead(ctx context.Context, rtID, userID, id string, isOfficer bool) error {
	var ownerID *string
	var eventType, dedupKey string
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT user_id, event_type, dedup_key
		FROM notification_logs
		WHERE id = $1 AND rt_id = $2
	`, id, rtID).Scan(&ownerID, &eventType, &dedupKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isTableMissingErr(err) {
			return sql.ErrNoRows
		}
		return err
	}

	if strings.HasPrefix(dedupKey, "test_push:") {
		if ownerID == nil || *ownerID != userID {
			return ErrForbidden
		}
	} else if !isOfficer {
		isOwn := ownerID != nil && *ownerID == userID
		if !isOwn {
			return ErrForbidden
		}
	}

	targetUserID := userID
	if isOfficer && ownerID != nil && *ownerID != "" {
		targetUserID = *ownerID
	}

	insertQuery := `
		INSERT INTO notification_reads (notification_id, user_id, read_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (notification_id, user_id) DO NOTHING;
	`
	_, err = r.pool.Raw().ExecContext(ctx, insertQuery, id, targetUserID)
	if err != nil && !isTableMissingErr(err) {
		return err
	}
	if targetUserID != userID {
		_, _ = r.pool.Raw().ExecContext(ctx, insertQuery, id, userID)
	}
	return nil
}

func (r *pgRepository) MarkAllAsRead(ctx context.Context, rtID, userID string, isOfficer bool) (int, error) {
	var insertQuery string
	if !isOfficer {
		insertQuery = `
			INSERT INTO notification_reads (notification_id, user_id, read_at)
			SELECT nl.id, $1, NOW()
			FROM notification_logs nl
			LEFT JOIN notification_reads nr ON nr.notification_id = nl.id AND nr.user_id = $1
			WHERE nl.rt_id = $2
			  AND nl.user_id = $1
			  AND nl.dedup_key NOT LIKE 'manual_push:%'
			  AND nr.read_at IS NULL
			ON CONFLICT (notification_id, user_id) DO NOTHING;
		`
	} else {
		insertQuery = `
			INSERT INTO notification_reads (notification_id, user_id, read_at)
			SELECT nl.id, $1, NOW()
			FROM notification_logs nl
			LEFT JOIN notification_reads nr ON nr.notification_id = nl.id AND nr.user_id = $1
			WHERE nl.rt_id = $2
			  AND NOT (nl.dedup_key LIKE 'test_push:%')
			  AND nr.read_at IS NULL
			ON CONFLICT (notification_id, user_id) DO NOTHING;
		`
	}
	res, err := r.pool.Raw().ExecContext(ctx, insertQuery, userID, rtID)
	if err != nil {
		if isTableMissingErr(err) {
			return 0, nil
		}
		return 0, err
	}
	rows, _ := res.RowsAffected()
	return int(rows), nil
}

func (r *pgRepository) GetUnreadCount(ctx context.Context, rtID, userID string, isOfficer bool) (int, error) {
	var countQuery string
	if !isOfficer {
		countQuery = `
			SELECT COUNT(*)
			FROM notification_logs nl
			LEFT JOIN notification_reads nr ON nr.notification_id = nl.id AND nr.user_id = $1
			WHERE nl.rt_id = $2
			  AND nl.user_id = $1
			  AND nl.dedup_key NOT LIKE 'manual_push:%'
			  AND nr.read_at IS NULL
		`
	} else {
		countQuery = `
			SELECT COUNT(*)
			FROM notification_logs nl
			LEFT JOIN notification_reads nr ON nr.notification_id = nl.id AND nr.user_id = $1
			WHERE nl.rt_id = $2
			  AND NOT (nl.dedup_key LIKE 'test_push:%')
			  AND nr.read_at IS NULL
		`
	}
	var count int
	err := r.pool.Raw().QueryRowContext(ctx, countQuery, userID, rtID).Scan(&count)
	if err != nil {
		if isTableMissingErr(err) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

func (r *pgRepository) GetLogByID(ctx context.Context, rtID, id string) (*NotificationLogEntry, error) {
	query := `
		SELECT id, rt_id, user_id, sender_user_id, event_type, dedup_key, title, body, status, created_at, image_url, payload
		FROM notification_logs
		WHERE id = $1 AND rt_id = $2
		LIMIT 1;
	`
	var entry NotificationLogEntry
	var payloadBytes []byte
	err := r.pool.Raw().QueryRowContext(ctx, query, id, rtID).Scan(
		&entry.ID, &entry.RTID, &entry.UserID, &entry.SenderUserID, &entry.EventType, &entry.DedupKey, &entry.Title, &entry.Body, &entry.Status, &entry.CreatedAt, &entry.ImageURL, &payloadBytes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &entry.Payload)
	}
	return &entry, nil
}

func (r *pgRepository) GetBroadcastRecipientLog(
	ctx context.Context,
	rtID, userID, eventType, broadcastID string,
) (*NotificationLogEntry, error) {
	var entry NotificationLogEntry
	var payloadBytes []byte
	var isRead sql.NullBool
	var readAt sql.NullTime
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT nl.id, nl.rt_id, nl.user_id, nl.sender_user_id, nl.event_type,
			nl.dedup_key, nl.title, nl.body, nl.status, nl.created_at, nl.image_url,
			nl.payload, (nr.read_at IS NOT NULL), nr.read_at
		FROM notification_logs nl
		LEFT JOIN notification_reads nr ON nr.notification_id = nl.id AND nr.user_id = $2
		WHERE nl.rt_id = $1 AND nl.user_id = $2 AND nl.event_type = $3
		  AND nl.dedup_key = 'recipient:' || $3 || ':' || $4 || ':' || $2
		LIMIT 1
	`, rtID, userID, eventType, broadcastID).Scan(
		&entry.ID, &entry.RTID, &entry.UserID, &entry.SenderUserID, &entry.EventType,
		&entry.DedupKey, &entry.Title, &entry.Body, &entry.Status, &entry.CreatedAt,
		&entry.ImageURL, &payloadBytes, &isRead, &readAt,
	)
	if errors.Is(err, sql.ErrNoRows) || isTableMissingErr(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entry.IsRead = isRead.Valid && isRead.Bool
	if readAt.Valid {
		readAtValue := readAt.Time
		entry.ReadAt = &readAtValue
	}
	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &entry.Payload); err != nil {
			return nil, fmt.Errorf("decode notification recipient payload: %w", err)
		}
	}
	return &entry, nil
}

func (r *pgRepository) GetDefaultRTID(ctx context.Context) (string, error) {
	var rtID string
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT id FROM rts
		WHERE is_active = true AND (id = 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b' OR lower(name) = 'urena' OR (name ILIKE '%urena%' AND rt = '002'))
		ORDER BY
			CASE
				WHEN id = 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b' THEN 1
				WHEN lower(name) = 'urena' AND rt = '002' THEN 2
				WHEN lower(name) = 'urena' THEN 3
				WHEN name ILIKE '%urena%' AND rt = '002' THEN 4
				ELSE 5
			END ASC
		LIMIT 1
	`).Scan(&rtID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("default rt (rt 002/urena) not found or inactive")
		}
		return "", err
	}
	return rtID, nil
}
