package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	query := `
		INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, payload, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'SENT')
		ON CONFLICT (dedup_key) DO NOTHING
		RETURNING id;
	`
	var id string
	err := r.pool.Raw().QueryRowContext(ctx, query, rtID, uidParam, eventType, dedupKey, title, body, payloadJSON).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Row already exists -> deduplicated!
			return false, nil
		}
		if isTableMissingErr(err) {
			slog.Warn("notification_logs table does not exist yet, skipping dedup record", "error", err)
			return true, nil
		}
		return false, err
	}
	return true, nil
}
