package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"social-finance/internal/database"
)

// Repository manages persistence for devices and notification delivery logs.
type Repository interface {
	RegisterDevice(ctx context.Context, userID, token, platform string, deviceID *string) error
	UnregisterDevice(ctx context.Context, userID, token string) error
	DeactivateToken(ctx context.Context, token string) error
	GetTokensForUser(ctx context.Context, userID string) ([]string, error)
	GetTokensForPaymentVerifiers(ctx context.Context, rtID string) ([]string, error)
	GetTokensForRT(ctx context.Context, rtID string) ([]string, error)
	GetRecipientUserIDsForBill(ctx context.Context, rtID, billID string) ([]string, error)
	CheckAndRecordDedup(ctx context.Context, rtID, userID, eventType, dedupKey, title, body string, payload map[string]string) (bool, error)

	// Template management
	ListTemplates(ctx context.Context, rtID, eventType string, isActive *bool) ([]NotificationTemplate, error)
	GetTemplateByID(ctx context.Context, rtID, id string) (*NotificationTemplate, error)
	GetActiveTemplateForEvent(ctx context.Context, rtID, eventType string) (*NotificationTemplate, error)
	CreateTemplate(ctx context.Context, rtID string, in CreateTemplateInput, allowedVars []string) (*NotificationTemplate, error)
	UpdateTemplate(ctx context.Context, rtID, id string, in UpdateTemplateInput) (*NotificationTemplate, error)
	ToggleTemplateActive(ctx context.Context, rtID, id string) (*NotificationTemplate, error)
	DeleteTemplate(ctx context.Context, rtID, id string) error

	// Logs listing
	ListLogs(ctx context.Context, rtID, eventType string, offset, limit int) ([]NotificationLogEntry, int, error)
	GetLogByID(ctx context.Context, rtID, id string) (*NotificationLogEntry, error)

	// Tenant resolution fallback for system roles
	GetDefaultRTID(ctx context.Context) (string, error)
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
	query := `
		SELECT DISTINCT ud.fcm_token
		FROM user_devices ud
		JOIN user_rt_memberships urm ON urm.user_id = ud.user_id
		WHERE urm.rt_id = $1
		  AND ud.is_active = true
		  AND urm.is_active = true;
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

func (r *pgRepository) ListLogs(ctx context.Context, rtID, eventType string, offset, limit int) ([]NotificationLogEntry, int, error) {
	countQuery := `SELECT COUNT(*) FROM notification_logs WHERE rt_id = $1`
	countArgs := []any{rtID}
	if strings.TrimSpace(eventType) != "" {
		countQuery += " AND event_type = $2"
		countArgs = append(countArgs, strings.TrimSpace(eventType))
	}

	var total int
	err := r.pool.Raw().QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		if isTableMissingErr(err) {
			return []NotificationLogEntry{}, 0, nil
		}
		return nil, 0, err
	}

	query := `
		SELECT id, rt_id, user_id, event_type, dedup_key, title, body, status, created_at, image_url
		FROM notification_logs
		WHERE rt_id = $1
	`
	args := []any{rtID}
	argIdx := 2
	if strings.TrimSpace(eventType) != "" {
		query += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, strings.TrimSpace(eventType))
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Raw().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []NotificationLogEntry
	for rows.Next() {
		var entry NotificationLogEntry
		if err := rows.Scan(&entry.ID, &entry.RTID, &entry.UserID, &entry.EventType, &entry.DedupKey, &entry.Title, &entry.Body, &entry.Status, &entry.CreatedAt, &entry.ImageURL); err != nil {
			return nil, 0, err
		}
		logs = append(logs, entry)
	}
	if logs == nil {
		logs = []NotificationLogEntry{}
	}
	return logs, total, rows.Err()
}

func (r *pgRepository) GetLogByID(ctx context.Context, rtID, id string) (*NotificationLogEntry, error) {
	query := `
		SELECT id, rt_id, user_id, event_type, dedup_key, title, body, status, created_at, image_url
		FROM notification_logs
		WHERE id = $1 AND rt_id = $2
		LIMIT 1;
	`
	var entry NotificationLogEntry
	err := r.pool.Raw().QueryRowContext(ctx, query, id, rtID).Scan(
		&entry.ID, &entry.RTID, &entry.UserID, &entry.EventType, &entry.DedupKey, &entry.Title, &entry.Body, &entry.Status, &entry.CreatedAt, &entry.ImageURL,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isTableMissingErr(err) {
			return nil, nil
		}
		return nil, err
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
