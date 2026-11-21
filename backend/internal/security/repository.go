package security

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"social-finance/internal/database"
)

var (
	ErrNotFound          = errors.New("not_found")
	ErrValidation        = errors.New("validation_error")
	ErrConflict          = errors.New("conflict")
	ErrForbidden         = errors.New("forbidden")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrQRExpired         = errors.New("qr_expired")
	ErrQRAlreadyUsed     = errors.New("qr_already_used")
	ErrQRInvalid         = errors.New("invalid_qr")
	ErrDeviceRevoked     = errors.New("device_revoked")
	ErrCrossRT           = errors.New("cross_rt_denied")
	ErrSelfApproval      = errors.New("self_approval_forbidden")
	ErrCheckoutBeforeIn  = errors.New("checkout_before_checkin")
	ErrAlreadyCheckedIn  = errors.New("already_checked_in")
	ErrAlreadyCheckedOut = errors.New("already_checked_out")
	ErrWindowClosed      = errors.New("check_in_window_closed")
	ErrNoActiveSchedule  = errors.New("no_matching_schedule")
)

type AppError struct {
	Code    string
	Status  int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *AppError) Unwrap() error { return e.Err }

func newAppError(status int, code, msg string, err error) *AppError {
	return &AppError{Status: status, Code: code, Message: msg, Err: err}
}

type DeviceAuthRecord struct {
	DeviceID      string
	RTID          string
	RTName        string
	RTCode        string
	RWID          sql.NullString
	RTActive      bool
	DeviceName    string
	LocationLabel string
	TokenHash     sql.NullString
	IsActive      bool
	RevokedAt     *time.Time
}

type Repository struct {
	pool *database.Pool
}

func NewRepository(pool *database.Pool) *Repository {
	return &Repository{pool: pool}
}

// ── Audit Helper ──────────────────────────────────────────────────────────────

func insertAuditLog(ctx context.Context, tx *sql.Tx, rtID string, userID *string, action, entityType, entityID string, oldVal, newVal any, ip string) error {
	var oldJSON, newJSON []byte
	var err error
	if oldVal != nil {
		if oldJSON, err = json.Marshal(oldVal); err != nil {
			return fmt.Errorf("marshal old audit values: %w", err)
		}
	}
	if newVal != nil {
		if newJSON, err = json.Marshal(newVal); err != nil {
			return fmt.Errorf("marshal new audit values: %w", err)
		}
	}
	var oldNullable, newNullable any
	if len(oldJSON) > 0 {
		oldNullable = string(oldJSON)
	}
	if len(newJSON) > 0 {
		newNullable = string(newJSON)
	}
	var uid any
	if userID != nil && *userID != "" {
		uid = *userID
	}
	var ipVal any
	if ip != "" {
		ipVal = ip
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (rt_id, user_id, action, entity_type, entity_id, old_values, new_values, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8)
	`, rtID, uid, action, entityType, entityID, oldNullable, newNullable, ipVal)
	return err
}

// ── Devices ───────────────────────────────────────────────────────────────────

func (r *Repository) GetDeviceAuthRecord(ctx context.Context, deviceID string) (*DeviceAuthRecord, error) {
	var rec DeviceAuthRecord
	var revoked sql.NullTime
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT d.id, d.rt_id, rt.name, rt.rt, rt.rw_id, rt.is_active,
		       d.device_name, d.location_label, d.token_hash, d.is_active, d.revoked_at
		FROM security_devices d
		JOIN rts rt ON rt.id = d.rt_id
		WHERE d.id = $1
	`, deviceID).Scan(
		&rec.DeviceID, &rec.RTID, &rec.RTName, &rec.RTCode, &rec.RWID, &rec.RTActive,
		&rec.DeviceName, &rec.LocationLabel, &rec.TokenHash, &rec.IsActive, &revoked,
	)
	if err != nil {
		return nil, err
	}
	if revoked.Valid {
		t := revoked.Time
		rec.RevokedAt = &t
	}
	return &rec, nil
}

func (r *Repository) TouchDeviceActivity(ctx context.Context, deviceID string, at time.Time) error {
	_, err := r.pool.Raw().ExecContext(ctx, `
		UPDATE security_devices SET last_active_at = $2, updated_at = $2
		WHERE id = $1 AND is_active = true AND revoked_at IS NULL
	`, deviceID, at)
	return err
}

func (r *Repository) ListDevices(ctx context.Context, rtID string) ([]SecurityDevice, error) {
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT d.id, d.rt_id, rt.name, rt.rt, rt.rw_id, d.device_name, d.location_label,
		       d.is_active, (d.token_hash IS NOT NULL), d.activation_expires_at,
		       d.activated_at, d.last_active_at, d.revoked_at, d.created_by, d.created_at, d.updated_at
		FROM security_devices d
		JOIN rts rt ON rt.id = d.rt_id
		WHERE d.rt_id = $1
		ORDER BY d.is_active DESC, d.created_at DESC
	`, rtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SecurityDevice
	for rows.Next() {
		var d SecurityDevice
		var rwID sql.NullString
		var actExp, actAt, lastAt, revAt sql.NullTime
		if err := rows.Scan(
			&d.ID, &d.RTID, &d.RTName, &d.RTCode, &rwID, &d.DeviceName, &d.LocationLabel,
			&d.IsActive, &d.IsPaired, &actExp, &actAt, &lastAt, &revAt,
			&d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if rwID.Valid {
			v := rwID.String
			d.RWID = &v
		}
		if actExp.Valid {
			t := actExp.Time
			d.ActivationExpiresAt = &t
		}
		if actAt.Valid {
			t := actAt.Time
			d.ActivatedAt = &t
		}
		if lastAt.Valid {
			t := lastAt.Time
			d.LastActiveAt = &t
		}
		if revAt.Valid {
			t := revAt.Time
			d.RevokedAt = &t
		}
		list = append(list, d)
	}
	if list == nil {
		list = []SecurityDevice{}
	}
	return list, rows.Err()
}

func (r *Repository) CreateDevice(ctx context.Context, rtID, createdBy, name, location, pairingHash string, expiresAt, now time.Time, ip string) (*SecurityDevice, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var d SecurityDevice
	var rwID sql.NullString
	var actExp sql.NullTime
	err = tx.QueryRowContext(ctx, `
		INSERT INTO security_devices (rt_id, device_name, location_label, activation_code_hash, activation_expires_at, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING id, rt_id, device_name, location_label, is_active, activation_expires_at, created_by, created_at, updated_at
	`, rtID, name, location, pairingHash, expiresAt, createdBy, now).Scan(
		&d.ID, &d.RTID, &d.DeviceName, &d.LocationLabel, &d.IsActive, &actExp, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if actExp.Valid {
		t := actExp.Time
		d.ActivationExpiresAt = &t
	}
	_ = tx.QueryRowContext(ctx, `SELECT name, rt, rw_id FROM rts WHERE id = $1`, rtID).Scan(&d.RTName, &d.RTCode, &rwID)
	if rwID.Valid {
		v := rwID.String
		d.RWID = &v
	}

	_ = insertAuditLog(ctx, tx, rtID, &createdBy, "security_device_created", "security_device", d.ID, nil, map[string]any{
		"device_name":    d.DeviceName,
		"location_label": d.LocationLabel,
	}, ip)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) RotatePairingCode(ctx context.Context, rtID, deviceID, actorID, pairingHash string, expiresAt, now time.Time, ip string) (*SecurityDevice, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var d SecurityDevice
	var isActive bool
	var revoked sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT id, rt_id, device_name, location_label, is_active, revoked_at, created_by, created_at, updated_at
		FROM security_devices
		WHERE id = $1 AND rt_id = $2
		FOR UPDATE
	`, deviceID, rtID).Scan(&d.ID, &d.RTID, &d.DeviceName, &d.LocationLabel, &isActive, &revoked, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "security device not found", ErrNotFound)
		}
		return nil, err
	}
	if !isActive || revoked.Valid {
		return nil, newAppError(409, "device_revoked", "cannot rotate pairing code for revoked device", ErrDeviceRevoked)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_devices
		SET activation_code_hash = $1, activation_expires_at = $2, updated_at = $3
		WHERE id = $4
	`, pairingHash, expiresAt, now, deviceID)
	if err != nil {
		return nil, err
	}
	d.IsActive = true
	d.ActivationExpiresAt = &expiresAt
	d.UpdatedAt = now

	_ = insertAuditLog(ctx, tx, rtID, &actorID, "security_device_pairing_rotated", "security_device", d.ID, nil, map[string]any{
		"expires_at": expiresAt,
	}, ip)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) ActivateDevice(ctx context.Context, deviceID, providedSecret, tokenHash string, now time.Time, ip string) (*SecurityDevice, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var d SecurityDevice
	var rwID sql.NullString
	var actHash sql.NullString
	var actExp sql.NullTime
	var revAt sql.NullTime
	var rtActive bool
	err = tx.QueryRowContext(ctx, `
		SELECT d.id, d.rt_id, rt.name, rt.rt, rt.rw_id, rt.is_active,
		       d.device_name, d.location_label, d.activation_code_hash, d.activation_expires_at,
		       d.is_active, d.revoked_at, d.created_by, d.created_at
		FROM security_devices d
		JOIN rts rt ON rt.id = d.rt_id
		WHERE d.id = $1
		FOR UPDATE
	`, deviceID).Scan(
		&d.ID, &d.RTID, &d.RTName, &d.RTCode, &rwID, &rtActive,
		&d.DeviceName, &d.LocationLabel, &actHash, &actExp,
		&d.IsActive, &revAt, &d.CreatedBy, &d.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(401, "invalid_pairing", "invalid pairing token", ErrUnauthorized)
		}
		return nil, err
	}
	if rwID.Valid {
		v := rwID.String
		d.RWID = &v
	}
	if !d.IsActive || revAt.Valid || !rtActive {
		return nil, newAppError(403, "device_revoked", "security device or RT is inactive/revoked", ErrDeviceRevoked)
	}
	if !actHash.Valid || actHash.String == "" || !actExp.Valid {
		return nil, newAppError(400, "pairing_already_used", "pairing token already used or unavailable", ErrValidation)
	}
	if now.After(actExp.Time) {
		return nil, newAppError(400, "pairing_expired", "pairing token has expired", ErrQRExpired)
	}
	expectedHash := HashToken(providedSecret)
	if subtle.ConstantTimeCompare([]byte(expectedHash), []byte(actHash.String)) != 1 {
		return nil, newAppError(401, "invalid_pairing", "invalid pairing token", ErrUnauthorized)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_devices
		SET token_hash = $1,
		    activation_code_hash = NULL,
		    activation_expires_at = NULL,
		    activated_at = COALESCE(activated_at, $2),
		    last_active_at = $2,
		    updated_at = $2
		WHERE id = $3
	`, tokenHash, now, deviceID)
	if err != nil {
		return nil, err
	}

	d.IsPaired = true
	d.ActivatedAt = &now
	d.LastActiveAt = &now
	d.UpdatedAt = now

	_ = insertAuditLog(ctx, tx, d.RTID, nil, "security_device_activated", "security_device", d.ID, nil, map[string]any{
		"activated_at": now,
	}, ip)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) RevokeDevice(ctx context.Context, rtID, deviceID, actorID string, now time.Time, ip string) (*SecurityDevice, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var d SecurityDevice
	err = tx.QueryRowContext(ctx, `
		SELECT id, rt_id, device_name, location_label, is_active, created_by, created_at, updated_at
		FROM security_devices
		WHERE id = $1 AND rt_id = $2
		FOR UPDATE
	`, deviceID, rtID).Scan(&d.ID, &d.RTID, &d.DeviceName, &d.LocationLabel, &d.IsActive, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "security device not found", ErrNotFound)
		}
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_devices
		SET is_active = false, revoked_at = COALESCE(revoked_at, $1), revoked_by = COALESCE(revoked_by, $2),
		    token_hash = NULL, activation_code_hash = NULL, activation_expires_at = NULL, updated_at = $1
		WHERE id = $3
	`, now, actorID, deviceID)
	if err != nil {
		return nil, err
	}

	// Invalidate all currently unredeemed QR tokens issued by this device.
	_, err = tx.ExecContext(ctx, `
		UPDATE security_qr_tokens
		SET invalidated_at = $1
		WHERE device_id = $2 AND used_at IS NULL AND invalidated_at IS NULL
	`, now, deviceID)
	if err != nil {
		return nil, err
	}

	d.IsActive = false
	d.IsPaired = false
	d.RevokedAt = &now
	d.UpdatedAt = now

	_ = insertAuditLog(ctx, tx, rtID, &actorID, "security_device_revoked", "security_device", d.ID, nil, map[string]any{
		"revoked_at": now,
	}, ip)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &d, nil
}

// ── Officers ──────────────────────────────────────────────────────────────────

func (r *Repository) ListOfficers(ctx context.Context, rtID string) ([]SecurityOfficer, error) {
	return r.listOfficers(ctx, rtID, false)
}

func (r *Repository) ListOfficersForAttendance(ctx context.Context, rtID string) ([]SecurityOfficer, error) {
	return r.listOfficers(ctx, rtID, true)
}

func (r *Repository) listOfficers(ctx context.Context, rtID string, redactContact bool) ([]SecurityOfficer, error) {
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT u.id, urm.id, urm.rt_id, u.full_name,
		       CASE WHEN $2 THEN '' ELSE u.email END,
		       CASE WHEN $2 THEN NULL ELSE u.phone END,
		       u.is_active,
		       (SELECT r.id FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1),
		       (SELECT r.avatar_path FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1),
		       (SELECT r.avatar_thumb_path FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1)
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE urm.rt_id = $1
		  AND urm.jabatan = 'keamanan'
		  AND urm.is_active = true
		  AND u.is_active = true
		ORDER BY u.full_name ASC
	`, rtID, redactContact)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SecurityOfficer
	for rows.Next() {
		var o SecurityOfficer
		var phone sql.NullString
		var avatarPath sql.NullString
		var avatarThumbPath sql.NullString
		var residentID sql.NullString
		if err := rows.Scan(&o.UserID, &o.MembershipID, &o.RTID, &o.FullName, &o.Email, &phone, &o.IsActive, &residentID, &avatarPath, &avatarThumbPath); err != nil {
			return nil, err
		}
		if residentID.Valid && strings.TrimSpace(residentID.String) != "" {
			v := residentID.String
			o.ResidentID = &v
		}
		if avatarPath.Valid && strings.TrimSpace(avatarPath.String) != "" {
			v := avatarPath.String
			o.AvatarPath = &v
		}
		if avatarThumbPath.Valid && strings.TrimSpace(avatarThumbPath.String) != "" {
			v := avatarThumbPath.String
			o.AvatarThumbPath = &v
		}
		if phone.Valid && strings.TrimSpace(phone.String) != "" {
			v := phone.String
			o.Phone = &v
		}
		list = append(list, o)
	}
	if list == nil {
		list = []SecurityOfficer{}
	}
	return list, rows.Err()
}

func (r *Repository) ValidateActiveOfficer(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, rtID, userID string) (membershipID string, err error) {
	err = q.QueryRowContext(ctx, `
		SELECT urm.id
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE urm.rt_id = $1
		  AND urm.user_id = $2
		  AND urm.jabatan = 'keamanan'
		  AND urm.is_active = true
		  AND u.is_active = true
		LIMIT 1
	`, rtID, userID).Scan(&membershipID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", newAppError(400, "invalid_officer", "user is not an active security officer in this RT", ErrForbidden)
		}
		return "", err
	}
	return membershipID, nil
}

// ── Shift Configs ─────────────────────────────────────────────────────────────

func (r *Repository) ListShifts(ctx context.Context, rtID string, activeOnly bool) ([]ShiftConfig, error) {
	query := `
		SELECT id, rt_id, name, to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'),
		       crosses_midnight, lateness_tolerance_minutes, check_in_window_before_minutes,
		       check_in_window_after_minutes, absent_cutoff_minutes, missing_checkout_cutoff_minutes,
		       version, is_active, created_by, created_at, updated_at
		FROM security_shift_configs
		WHERE rt_id = $1
	`
	if activeOnly {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY is_active DESC, start_time ASC, name ASC`

	rows, err := r.pool.Raw().QueryContext(ctx, query, rtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ShiftConfig
	for rows.Next() {
		var s ShiftConfig
		if err := rows.Scan(
			&s.ID, &s.RTID, &s.Name, &s.StartTime, &s.EndTime, &s.CrossesMidnight,
			&s.LatenessToleranceMinutes, &s.CheckInWindowBeforeMinutes, &s.CheckInWindowAfterMinutes,
			&s.AbsentCutoffMinutes, &s.MissingCheckoutCutoffMinutes,
			&s.Version, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = []ShiftConfig{}
	}
	return list, rows.Err()
}

func (r *Repository) CreateShift(ctx context.Context, rtID, actorID string, req UpsertShiftRequest, startNorm, endNorm string, crosses bool, now time.Time) (*ShiftConfig, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	var s ShiftConfig
	err := r.pool.Raw().QueryRowContext(ctx, `
		INSERT INTO security_shift_configs (
			rt_id, name, start_time, end_time, crosses_midnight,
			lateness_tolerance_minutes, check_in_window_before_minutes, check_in_window_after_minutes,
			absent_cutoff_minutes, missing_checkout_cutoff_minutes, is_active, created_by, created_at, updated_at
		) VALUES ($1, $2, $3::time, $4::time, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)
		RETURNING id, rt_id, name, to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'),
		          crosses_midnight, lateness_tolerance_minutes, check_in_window_before_minutes,
		          check_in_window_after_minutes, absent_cutoff_minutes, missing_checkout_cutoff_minutes,
		          version, is_active, created_by, created_at, updated_at
	`, rtID, strings.TrimSpace(req.Name), startNorm, endNorm, crosses,
		req.LatenessToleranceMinutes, req.CheckInWindowBeforeMinutes, req.CheckInWindowAfterMinutes,
		req.AbsentCutoffMinutes, req.MissingCheckoutCutoffMinutes, isActive, actorID, now,
	).Scan(
		&s.ID, &s.RTID, &s.Name, &s.StartTime, &s.EndTime, &s.CrossesMidnight,
		&s.LatenessToleranceMinutes, &s.CheckInWindowBeforeMinutes, &s.CheckInWindowAfterMinutes,
		&s.AbsentCutoffMinutes, &s.MissingCheckoutCutoffMinutes,
		&s.Version, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, newAppError(409, "duplicate_shift", "an active shift with this name already exists", ErrConflict)
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repository) UpdateShift(ctx context.Context, rtID, shiftID, actorID string, req UpsertShiftRequest, startNorm, endNorm string, crosses bool, now time.Time) (*ShiftConfig, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentActive bool
	err = tx.QueryRowContext(ctx, `
		SELECT is_active FROM security_shift_configs WHERE id = $1 AND rt_id = $2 FOR UPDATE
	`, shiftID, rtID).Scan(&currentActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "shift configuration not found", ErrNotFound)
		}
		return nil, err
	}

	isActive := currentActive
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	var s ShiftConfig
	err = tx.QueryRowContext(ctx, `
		UPDATE security_shift_configs
		SET name = $3,
		    start_time = $4::time,
		    end_time = $5::time,
		    crosses_midnight = $6,
		    lateness_tolerance_minutes = $7,
		    check_in_window_before_minutes = $8,
		    check_in_window_after_minutes = $9,
		    absent_cutoff_minutes = $10,
		    missing_checkout_cutoff_minutes = $11,
		    is_active = $12,
		    version = version + 1,
		    updated_by = $13,
		    updated_at = $14
		WHERE id = $1 AND rt_id = $2
		RETURNING id, rt_id, name, to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'),
		          crosses_midnight, lateness_tolerance_minutes, check_in_window_before_minutes,
		          check_in_window_after_minutes, absent_cutoff_minutes, missing_checkout_cutoff_minutes,
		          version, is_active, created_by, created_at, updated_at
	`, shiftID, rtID, strings.TrimSpace(req.Name), startNorm, endNorm, crosses,
		req.LatenessToleranceMinutes, req.CheckInWindowBeforeMinutes, req.CheckInWindowAfterMinutes,
		req.AbsentCutoffMinutes, req.MissingCheckoutCutoffMinutes, isActive, actorID, now,
	).Scan(
		&s.ID, &s.RTID, &s.Name, &s.StartTime, &s.EndTime, &s.CrossesMidnight,
		&s.LatenessToleranceMinutes, &s.CheckInWindowBeforeMinutes, &s.CheckInWindowAfterMinutes,
		&s.AbsentCutoffMinutes, &s.MissingCheckoutCutoffMinutes,
		&s.Version, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, newAppError(409, "duplicate_shift", "an active shift with this name already exists", ErrConflict)
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &s, nil
}

// ── Schedules ─────────────────────────────────────────────────────────────────

func (r *Repository) createScheduleLegacy(ctx context.Context, rtID, actorID string, req CreateScheduleRequest, shiftDate time.Time, now time.Time) (*SecuritySchedule, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	membershipID, err := r.ValidateActiveOfficer(ctx, tx, rtID, req.UserID)
	if err != nil {
		return nil, err
	}

	var cfg ShiftConfig
	err = tx.QueryRowContext(ctx, `
		SELECT id, rt_id, name, to_char(start_time, 'HH24:MI:SS'), to_char(end_time, 'HH24:MI:SS'),
		       crosses_midnight, lateness_tolerance_minutes, check_in_window_before_minutes,
		       check_in_window_after_minutes, absent_cutoff_minutes, missing_checkout_cutoff_minutes,
		       version, is_active
		FROM security_shift_configs
		WHERE id = $1 AND rt_id = $2
	`, req.ShiftConfigID, rtID).Scan(
		&cfg.ID, &cfg.RTID, &cfg.Name, &cfg.StartTime, &cfg.EndTime, &cfg.CrossesMidnight,
		&cfg.LatenessToleranceMinutes, &cfg.CheckInWindowBeforeMinutes, &cfg.CheckInWindowAfterMinutes,
		&cfg.AbsentCutoffMinutes, &cfg.MissingCheckoutCutoffMinutes, &cfg.Version, &cfg.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "shift_not_found", "shift configuration not found", ErrNotFound)
		}
		return nil, err
	}
	if !cfg.IsActive {
		return nil, newAppError(400, "shift_inactive", "cannot schedule an inactive shift configuration", ErrValidation)
	}

	schedStart, schedEnd, crosses, err := ComputeShiftBounds(shiftDate, cfg.StartTime, cfg.EndTime, cfg.CrossesMidnight)
	if err != nil {
		return nil, newAppError(400, "invalid_shift_time", err.Error(), ErrValidation)
	}

	// Prevent overlapping active schedules for the same officer across adjacent dates / midnight shifts.
	var overlapCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM security_schedules
		WHERE rt_id = $1
		  AND user_id = $2
		  AND status = 'scheduled'
		  AND scheduled_start_at < $4
		  AND scheduled_end_at > $3
	`, rtID, req.UserID, schedStart, schedEnd).Scan(&overlapCount)
	if err != nil {
		return nil, err
	}
	if overlapCount > 0 {
		return nil, newAppError(409, "schedule_conflict", "officer already has an overlapping schedule in that time window", ErrConflict)
	}

	dateStr := shiftDate.In(JakartaLoc).Format("2006-01-02")
	var out SecuritySchedule
	var notes sql.NullString
	err = tx.QueryRowContext(ctx, `
		INSERT INTO security_schedules (
			rt_id, user_id, membership_id, shift_config_id, shift_version, shift_date,
			shift_name_snapshot, start_time_snapshot, end_time_snapshot, crosses_midnight_snapshot,
			lateness_tolerance_minutes, check_in_window_before_minutes, check_in_window_after_minutes,
			absent_cutoff_minutes, missing_checkout_cutoff_minutes,
			scheduled_start_at, scheduled_end_at, notes, status, created_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6::date,
			$7, $8::time, $9::time, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, 'scheduled', $19, $20, $20
		)
		RETURNING id, rt_id, user_id, membership_id, shift_config_id, shift_version,
		          to_char(shift_date, 'YYYY-MM-DD'), shift_name_snapshot,
		          to_char(start_time_snapshot, 'HH24:MI'), to_char(end_time_snapshot, 'HH24:MI'),
		          crosses_midnight_snapshot, lateness_tolerance_minutes,
		          check_in_window_before_minutes, check_in_window_after_minutes,
		          absent_cutoff_minutes, missing_checkout_cutoff_minutes,
		          scheduled_start_at, scheduled_end_at, notes, status, created_at, updated_at
	`, rtID, req.UserID, membershipID, cfg.ID, cfg.Version, dateStr,
		cfg.Name, cfg.StartTime, cfg.EndTime, crosses,
		cfg.LatenessToleranceMinutes, cfg.CheckInWindowBeforeMinutes, cfg.CheckInWindowAfterMinutes,
		cfg.AbsentCutoffMinutes, cfg.MissingCheckoutCutoffMinutes,
		schedStart, schedEnd, req.Notes, actorID, now,
	).Scan(
		&out.ID, &out.RTID, &out.UserID, &out.MembershipID, &out.ShiftConfigID, &out.ShiftVersion,
		&out.ShiftDate, &out.ShiftName, &out.StartTime, &out.EndTime,
		&out.CrossesMidnight, &out.LatenessToleranceMinutes,
		&out.CheckInWindowBeforeMinutes, &out.CheckInWindowAfterMinutes,
		&out.AbsentCutoffMinutes, &out.MissingCheckoutCutoffMinutes,
		&out.ScheduledStartAt, &out.ScheduledEndAt, &notes, &out.Status, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, newAppError(409, "schedule_conflict", "officer already scheduled for this shift and date", ErrConflict)
		}
		return nil, err
	}
	if notes.Valid {
		v := notes.String
		out.Notes = &v
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO security_attendance_records (
			rt_id, schedule_id, user_id, shift_config_id, shift_version, shift_date,
			status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6::date, 'belum_hadir', $7, $7)
	`, rtID, out.ID, out.UserID, out.ShiftConfigID, out.ShiftVersion, dateStr, now)
	if err != nil {
		return nil, err
	}

	_ = tx.QueryRowContext(ctx, `SELECT full_name FROM users WHERE id = $1`, out.UserID).Scan(&out.OfficerName)
	out.AttendanceStatus = StatusBelumHadir
	out.AttendanceStatusLabel = StatusLabel(StatusBelumHadir)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) updateScheduleLegacy(ctx context.Context, rtID, scheduleID, actorID string, req UpdateScheduleRequest, shiftDate time.Time, now time.Time) (*SecuritySchedule, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var curStatus string
	var attStatus sql.NullString
	var checkedInAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT s.status, ar.status, ar.checked_in_at
		FROM security_schedules s
		LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
		WHERE s.id = $1 AND s.rt_id = $2
		FOR UPDATE OF s
	`, scheduleID, rtID).Scan(&curStatus, &attStatus, &checkedInAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "schedule not found", ErrNotFound)
		}
		return nil, err
	}
	if curStatus != ScheduleStatusScheduled {
		return nil, newAppError(409, "schedule_cancelled", "cannot modify a cancelled schedule", ErrConflict)
	}
	if checkedInAt.Valid || (attStatus.Valid && attStatus.String != StatusBelumHadir) {
		return nil, newAppError(409, "attendance_locked", "cannot edit schedule after attendance has been recorded or finalized", ErrConflict)
	}

	membershipID, err := r.ValidateActiveOfficer(ctx, tx, rtID, req.UserID)
	if err != nil {
		return nil, err
	}

	var cfg ShiftConfig
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, to_char(start_time, 'HH24:MI:SS'), to_char(end_time, 'HH24:MI:SS'),
		       crosses_midnight, lateness_tolerance_minutes, check_in_window_before_minutes,
		       check_in_window_after_minutes, absent_cutoff_minutes, missing_checkout_cutoff_minutes,
		       version, is_active
		FROM security_shift_configs
		WHERE id = $1 AND rt_id = $2
	`, req.ShiftConfigID, rtID).Scan(
		&cfg.ID, &cfg.Name, &cfg.StartTime, &cfg.EndTime, &cfg.CrossesMidnight,
		&cfg.LatenessToleranceMinutes, &cfg.CheckInWindowBeforeMinutes, &cfg.CheckInWindowAfterMinutes,
		&cfg.AbsentCutoffMinutes, &cfg.MissingCheckoutCutoffMinutes, &cfg.Version, &cfg.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "shift_not_found", "shift configuration not found", ErrNotFound)
		}
		return nil, err
	}
	if !cfg.IsActive {
		return nil, newAppError(400, "shift_inactive", "cannot assign an inactive shift configuration", ErrValidation)
	}

	schedStart, schedEnd, crosses, err := ComputeShiftBounds(shiftDate, cfg.StartTime, cfg.EndTime, cfg.CrossesMidnight)
	if err != nil {
		return nil, newAppError(400, "invalid_shift_time", err.Error(), ErrValidation)
	}

	var overlapCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM security_schedules
		WHERE rt_id = $1
		  AND user_id = $2
		  AND id <> $3
		  AND status = 'scheduled'
		  AND scheduled_start_at < $5
		  AND scheduled_end_at > $4
	`, rtID, req.UserID, scheduleID, schedStart, schedEnd).Scan(&overlapCount)
	if err != nil {
		return nil, err
	}
	if overlapCount > 0 {
		return nil, newAppError(409, "schedule_conflict", "officer already has an overlapping schedule", ErrConflict)
	}

	dateStr := shiftDate.In(JakartaLoc).Format("2006-01-02")
	var out SecuritySchedule
	var notes sql.NullString
	err = tx.QueryRowContext(ctx, `
		UPDATE security_schedules
		SET user_id = $3,
		    membership_id = $4,
		    shift_config_id = $5,
		    shift_version = $6,
		    shift_date = $7::date,
		    shift_name_snapshot = $8,
		    start_time_snapshot = $9::time,
		    end_time_snapshot = $10::time,
		    crosses_midnight_snapshot = $11,
		    lateness_tolerance_minutes = $12,
		    check_in_window_before_minutes = $13,
		    check_in_window_after_minutes = $14,
		    absent_cutoff_minutes = $15,
		    missing_checkout_cutoff_minutes = $16,
		    scheduled_start_at = $17,
		    scheduled_end_at = $18,
		    notes = $19,
		    updated_by = $20,
		    updated_at = $21
		WHERE id = $1 AND rt_id = $2
		RETURNING id, rt_id, user_id, membership_id, shift_config_id, shift_version,
		          to_char(shift_date, 'YYYY-MM-DD'), shift_name_snapshot,
		          to_char(start_time_snapshot, 'HH24:MI'), to_char(end_time_snapshot, 'HH24:MI'),
		          crosses_midnight_snapshot, lateness_tolerance_minutes,
		          check_in_window_before_minutes, check_in_window_after_minutes,
		          absent_cutoff_minutes, missing_checkout_cutoff_minutes,
		          scheduled_start_at, scheduled_end_at, notes, status, created_at, updated_at
	`, scheduleID, rtID, req.UserID, membershipID, cfg.ID, cfg.Version, dateStr,
		cfg.Name, cfg.StartTime, cfg.EndTime, crosses,
		cfg.LatenessToleranceMinutes, cfg.CheckInWindowBeforeMinutes, cfg.CheckInWindowAfterMinutes,
		cfg.AbsentCutoffMinutes, cfg.MissingCheckoutCutoffMinutes,
		schedStart, schedEnd, req.Notes, actorID, now,
	).Scan(
		&out.ID, &out.RTID, &out.UserID, &out.MembershipID, &out.ShiftConfigID, &out.ShiftVersion,
		&out.ShiftDate, &out.ShiftName, &out.StartTime, &out.EndTime,
		&out.CrossesMidnight, &out.LatenessToleranceMinutes,
		&out.CheckInWindowBeforeMinutes, &out.CheckInWindowAfterMinutes,
		&out.AbsentCutoffMinutes, &out.MissingCheckoutCutoffMinutes,
		&out.ScheduledStartAt, &out.ScheduledEndAt, &notes, &out.Status, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if notes.Valid {
		v := notes.String
		out.Notes = &v
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_attendance_records
		SET user_id = $2, shift_config_id = $3, shift_version = $4, shift_date = $5::date, updated_at = $6
		WHERE schedule_id = $1 AND rt_id = $7
	`, scheduleID, out.UserID, out.ShiftConfigID, out.ShiftVersion, dateStr, now, rtID)
	if err != nil {
		return nil, err
	}

	_ = tx.QueryRowContext(ctx, `SELECT full_name FROM users WHERE id = $1`, out.UserID).Scan(&out.OfficerName)
	out.AttendanceStatus = StatusBelumHadir
	out.AttendanceStatusLabel = StatusLabel(StatusBelumHadir)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) CancelSchedule(ctx context.Context, rtID, scheduleID, actorID string, now time.Time) error {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var curStatus string
	var isDated bool
	var hasOpenSession bool
	var hasRecordedAtt bool
	err = tx.QueryRowContext(ctx, `
		SELECT s.status,
		       s.shift_date IS NOT NULL AS is_dated,
		       EXISTS (
		           SELECT 1 FROM security_attendance_records ar
		           WHERE ar.schedule_id = s.id
		             AND ar.checked_in_at IS NOT NULL
		             AND ar.checked_out_at IS NULL
		       ) AS has_open_session,
		       EXISTS (
		           SELECT 1 FROM security_attendance_records ar
		           WHERE ar.schedule_id = s.id
		             AND (ar.checked_in_at IS NOT NULL OR (ar.status IS NOT NULL AND ar.status != 'belum_hadir'))
		       ) AS has_recorded_att
		FROM security_schedules s
		WHERE s.id = $1 AND s.rt_id = $2
		FOR UPDATE OF s
	`, scheduleID, rtID).Scan(&curStatus, &isDated, &hasOpenSession, &hasRecordedAtt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newAppError(404, "not_found", "schedule not found", ErrNotFound)
		}
		return err
	}
	if hasOpenSession {
		return newAppError(409, "attendance_in_progress", "cannot deactivate schedule while an officer is currently on duty", ErrConflict)
	}
	if isDated && hasRecordedAtt {
		return newAppError(409, "attendance_locked", "cannot cancel schedule after attendance has been recorded", ErrConflict)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_schedules SET status = 'cancelled', is_active = false, updated_by = $2, updated_at = $3
		WHERE id = $1 AND rt_id = $4
	`, scheduleID, actorID, now, rtID)
	if err != nil {
		return err
	}

	// Cancel pending swap requests referencing this schedule.
	_, err = tx.ExecContext(ctx, `
		UPDATE security_shift_swaps
		SET status = 'cancelled', updated_at = $2
		WHERE (requester_schedule_id = $1 OR target_schedule_id = $1)
		  AND status IN ('pending_target', 'pending_ketua')
	`, scheduleID, now)
	if err != nil {
		return err
	}

	return tx.Commit()
}

type ScheduleFilter struct {
	DateFrom      string
	DateTo        string
	UserID        string
	ShiftConfigID string
	Status        string
	Page          int
	PerPage       int
}

func (r *Repository) ListSchedules(ctx context.Context, rtID string, f ScheduleFilter) ([]SecuritySchedule, int, error) {
	where := []string{"s.rt_id = $1"}
	args := []any{rtID}
	argIdx := 2

	if f.DateFrom != "" {
		where = append(where, fmt.Sprintf("s.shift_date >= $%d::date", argIdx))
		args = append(args, f.DateFrom)
		argIdx++
	}
	if f.DateTo != "" {
		where = append(where, fmt.Sprintf("s.shift_date <= $%d::date", argIdx))
		args = append(args, f.DateTo)
		argIdx++
	}
	if f.UserID != "" {
		where = append(where, fmt.Sprintf("s.user_id = $%d", argIdx))
		args = append(args, f.UserID)
		argIdx++
	}
	if f.ShiftConfigID != "" {
		where = append(where, fmt.Sprintf("s.shift_config_id = $%d", argIdx))
		args = append(args, f.ShiftConfigID)
		argIdx++
	}
	if f.Status != "" {
		where = append(where, fmt.Sprintf("s.status = $%d", argIdx))
		args = append(args, f.Status)
		argIdx++
	} else {
		where = append(where, "s.status = 'scheduled'")
	}

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM security_schedules s WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.PerPage
	if limit <= 0 {
		limit = 20
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT s.id, s.assignment_id, to_char(s.assignment_start_date, 'YYYY-MM-DD'), to_char(s.assignment_end_date, 'YYYY-MM-DD'),
		       s.rt_id, s.user_id, u.full_name, s.membership_id, s.shift_config_id, s.shift_version,
		       to_char(s.shift_date, 'YYYY-MM-DD'), s.shift_name_snapshot,
		       to_char(s.start_time_snapshot, 'HH24:MI'), to_char(s.end_time_snapshot, 'HH24:MI'),
		       s.crosses_midnight_snapshot, s.lateness_tolerance_minutes,
		       s.check_in_window_before_minutes, s.check_in_window_after_minutes,
		       s.absent_cutoff_minutes, s.missing_checkout_cutoff_minutes,
		       s.scheduled_start_at, s.scheduled_end_at, s.notes, s.status, s.is_active,
		       COALESCE(ar.status, 'belum_hadir'), ar.checked_in_at, ar.checked_out_at,
		       s.created_at, s.updated_at
		FROM security_schedules s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN LATERAL (
			SELECT status, checked_in_at, checked_out_at
			FROM security_attendance_records
			WHERE schedule_id = s.id
			  AND (
			        (s.shift_date IS NOT NULL AND shift_date = s.shift_date)
			        OR (s.shift_date IS NULL AND shift_date = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::date)
			      )
			ORDER BY checked_in_at DESC NULLS LAST, created_at DESC
			LIMIT 1
		) ar ON true
		WHERE %s
		ORDER BY s.is_active DESC, s.created_at DESC, u.full_name ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	rows, err := r.pool.Raw().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []SecuritySchedule
	for rows.Next() {
		var s SecuritySchedule
		var notes, dateNull sql.NullString
		var cin, cout, schedStartNull, schedEndNull sql.NullTime
		var startNull, endNull sql.NullString
		if err := rows.Scan(
			&s.ID, &s.AssignmentID, &startNull, &endNull,
			&s.RTID, &s.UserID, &s.OfficerName, &s.MembershipID, &s.ShiftConfigID, &s.ShiftVersion,
			&dateNull, &s.ShiftName, &s.StartTime, &s.EndTime,
			&s.CrossesMidnight, &s.LatenessToleranceMinutes,
			&s.CheckInWindowBeforeMinutes, &s.CheckInWindowAfterMinutes,
			&s.AbsentCutoffMinutes, &s.MissingCheckoutCutoffMinutes,
			&schedStartNull, &schedEndNull, &notes, &s.Status, &s.IsActive,
			&s.AttendanceStatus, &cin, &cout, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if startNull.Valid {
			s.AssignmentStartDate = &startNull.String
		}
		if endNull.Valid {
			s.AssignmentEndDate = &endNull.String
		}
		if dateNull.Valid {
			s.ShiftDate = dateNull.String
		}
		if notes.Valid {
			v := notes.String
			s.Notes = &v
		}
		if schedStartNull.Valid {
			s.ScheduledStartAt = schedStartNull.Time
		}
		if schedEndNull.Valid {
			s.ScheduledEndAt = schedEndNull.Time
		}
		if cin.Valid {
			t := cin.Time
			s.CheckedInAt = &t
		}
		if cout.Valid {
			t := cout.Time
			s.CheckedOutAt = &t
		}
		s.AttendanceStatusLabel = StatusLabel(s.AttendanceStatus)
		list = append(list, s)
	}
	if list == nil {
		list = []SecuritySchedule{}
	}
	return list, total, rows.Err()
}

// ── Shift Swaps ───────────────────────────────────────────────────────────────

func (r *Repository) CreateSwap(ctx context.Context, rtID, requesterUserID string, req CreateSwapRequest, now time.Time) (*ShiftSwap, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := r.ValidateActiveOfficer(ctx, tx, rtID, requesterUserID); err != nil {
		return nil, err
	}

	type schedInfo struct {
		ID        string
		UserID    string
		Status    string
		AttStatus string
		CheckedIn sql.NullTime
		StartAt   sql.NullTime
		EndAt     sql.NullTime
	}

	loadSched := func(id string) (*schedInfo, error) {
		var sc schedInfo
		var att sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT s.id, s.user_id, s.status, ar.status, ar.checked_in_at, s.scheduled_start_at, s.scheduled_end_at
			FROM security_schedules s
			LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
			WHERE s.id = $1 AND s.rt_id = $2
			FOR UPDATE OF s
		`, id, rtID).Scan(&sc.ID, &sc.UserID, &sc.Status, &att, &sc.CheckedIn, &sc.StartAt, &sc.EndAt)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, newAppError(404, "schedule_not_found", "schedule not found", ErrNotFound)
			}
			return nil, err
		}
		sc.AttStatus = StatusBelumHadir
		if att.Valid {
			sc.AttStatus = att.String
		}
		return &sc, nil
	}

	reqSched, err := loadSched(req.RequesterScheduleID)
	if err != nil {
		return nil, err
	}
	if reqSched.UserID != requesterUserID {
		return nil, newAppError(403, "forbidden", "you can only swap your own schedule", ErrForbidden)
	}
	tgtSched, err := loadSched(req.TargetScheduleID)
	if err != nil {
		return nil, err
	}
	if tgtSched.UserID == requesterUserID {
		return nil, newAppError(400, "invalid_swap", "cannot swap with your own schedule", ErrValidation)
	}
	if reqSched.Status != ScheduleStatusScheduled || tgtSched.Status != ScheduleStatusScheduled {
		return nil, newAppError(409, "schedule_cancelled", "both schedules must be active", ErrConflict)
	}
	if reqSched.CheckedIn.Valid || reqSched.AttStatus != StatusBelumHadir ||
		tgtSched.CheckedIn.Valid || tgtSched.AttStatus != StatusBelumHadir {
		return nil, newAppError(409, "attendance_locked", "cannot swap a schedule that already has recorded attendance or passed cutoff", ErrConflict)
	}
	if (reqSched.EndAt.Valid && !reqSched.EndAt.Time.After(now)) || (tgtSched.EndAt.Valid && !tgtSched.EndAt.Time.After(now)) {
		return nil, newAppError(400, "shift_ended", "cannot swap a shift that has already ended", ErrValidation)
	}

	if _, err := r.ValidateActiveOfficer(ctx, tx, rtID, tgtSched.UserID); err != nil {
		return nil, err
	}

	// Ensure no pending swap already touches either schedule.
	var pendingCnt int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_shift_swaps
		WHERE status IN ('pending_target', 'pending_ketua')
		  AND (requester_schedule_id IN ($1, $2) OR target_schedule_id IN ($1, $2))
	`, reqSched.ID, tgtSched.ID).Scan(&pendingCnt)
	if err != nil {
		return nil, err
	}
	if pendingCnt > 0 {
		return nil, newAppError(409, "swap_already_pending", "one of these schedules already has a pending swap request", ErrConflict)
	}

	var swapID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO security_shift_swaps (
			rt_id, requester_user_id, target_user_id, requester_schedule_id, target_schedule_id,
			reason, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending_target', $7, $7)
		RETURNING id
	`, rtID, requesterUserID, tgtSched.UserID, reqSched.ID, tgtSched.ID, strings.TrimSpace(req.Reason), now).Scan(&swapID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSwapByID(ctx, rtID, swapID)
}

func (r *Repository) TargetRespondSwap(ctx context.Context, rtID, swapID, targetUserID string, accept bool, note *string, now time.Time) (*ShiftSwap, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status, tgtUser string
	err = tx.QueryRowContext(ctx, `
		SELECT status, target_user_id
		FROM security_shift_swaps
		WHERE id = $1 AND rt_id = $2
		FOR UPDATE
	`, swapID, rtID).Scan(&status, &tgtUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "swap request not found", ErrNotFound)
		}
		return nil, err
	}
	if tgtUser != targetUserID {
		return nil, newAppError(403, "forbidden", "only the target officer can respond to this swap", ErrForbidden)
	}
	if status != SwapPendingTarget {
		return nil, newAppError(409, "invalid_swap_state", "swap request is not awaiting target officer response", ErrConflict)
	}

	nextStatus := SwapRejectedTarget
	if accept {
		nextStatus = SwapPendingKetua
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE security_shift_swaps
		SET status = $2, target_responded_at = $3, target_note = $4, updated_at = $3
		WHERE id = $1
	`, swapID, nextStatus, now, note)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSwapByID(ctx, rtID, swapID)
}

func (r *Repository) DecideSwap(ctx context.Context, rtID, swapID, approverID string, approve bool, note *string, now time.Time, ip string) (*ShiftSwap, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status, reqUser, tgtUser, reqSchedID, tgtSchedID string
	err = tx.QueryRowContext(ctx, `
		SELECT status, requester_user_id, target_user_id, requester_schedule_id, target_schedule_id
		FROM security_shift_swaps
		WHERE id = $1 AND rt_id = $2
		FOR UPDATE
	`, swapID, rtID).Scan(&status, &reqUser, &tgtUser, &reqSchedID, &tgtSchedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "swap request not found", ErrNotFound)
		}
		return nil, err
	}
	if status != SwapPendingKetua {
		return nil, newAppError(409, "invalid_swap_state", "swap request must be accepted by target officer first", ErrConflict)
	}

	if !approve {
		_, err = tx.ExecContext(ctx, `
			UPDATE security_shift_swaps
			SET status = 'rejected_ketua', ketua_decided_by = $2, ketua_decided_at = $3, ketua_note = $4, updated_at = $3
			WHERE id = $1
		`, swapID, approverID, now, note)
		if err != nil {
			return nil, err
		}
		_ = insertAuditLog(ctx, tx, rtID, &approverID, "security_swap_rejected", "security_shift_swap", swapID, nil, map[string]any{
			"note": note,
		}, ip)
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetSwapByID(ctx, rtID, swapID)
	}

	// Lock both schedules consistently ordered by ID to avoid deadlocks.
	firstID, secondID := reqSchedID, tgtSchedID
	if secondID < firstID {
		firstID, secondID = secondID, firstID
	}
	for _, id := range []string{firstID, secondID} {
		var sStatus string
		var attStatus sql.NullString
		var checkedIn sql.NullTime
		err := tx.QueryRowContext(ctx, `
			SELECT s.status, ar.status, ar.checked_in_at
			FROM security_schedules s
			LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
			WHERE s.id = $1 AND s.rt_id = $2
			FOR UPDATE OF s
		`, id, rtID).Scan(&sStatus, &attStatus, &checkedIn)
		if err != nil {
			return nil, err
		}
		if sStatus != ScheduleStatusScheduled || checkedIn.Valid || (attStatus.Valid && attStatus.String != StatusBelumHadir) {
			return nil, newAppError(409, "schedule_locked", "schedule cannot be swapped after attendance is recorded", ErrConflict)
		}
	}

	reqMembershipID, err := r.ValidateActiveOfficer(ctx, tx, rtID, reqUser)
	if err != nil {
		return nil, err
	}
	tgtMembershipID, err := r.ValidateActiveOfficer(ctx, tx, rtID, tgtUser)
	if err != nil {
		return nil, err
	}

	// Read start/end bounds for overlap checks after swap if dates were specified.
	var reqStart, reqEnd, tgtStart, tgtEnd sql.NullTime
	_ = tx.QueryRowContext(ctx, `SELECT scheduled_start_at, scheduled_end_at FROM security_schedules WHERE id = $1`, reqSchedID).Scan(&reqStart, &reqEnd)
	_ = tx.QueryRowContext(ctx, `SELECT scheduled_start_at, scheduled_end_at FROM security_schedules WHERE id = $1`, tgtSchedID).Scan(&tgtStart, &tgtEnd)

	// Check requester receiving tgtSched window doesn't overlap other schedules of requester (excluding reqSchedID & tgtSchedID).
	if reqStart.Valid && reqEnd.Valid && tgtStart.Valid && tgtEnd.Valid {
		var reqOverlap int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM security_schedules
			WHERE rt_id = $1 AND user_id = $2 AND status = 'scheduled'
			  AND id NOT IN ($3, $4) AND scheduled_start_at < $6 AND scheduled_end_at > $5
		`, rtID, reqUser, reqSchedID, tgtSchedID, tgtStart.Time, tgtEnd.Time).Scan(&reqOverlap); err != nil {
			return nil, err
		}
		if reqOverlap > 0 {
			return nil, newAppError(409, "schedule_conflict", "swap creates a schedule conflict for requester", ErrConflict)
		}
		var tgtOverlap int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM security_schedules
			WHERE rt_id = $1 AND user_id = $2 AND status = 'scheduled'
			  AND id NOT IN ($3, $4) AND scheduled_start_at < $6 AND scheduled_end_at > $5
		`, rtID, tgtUser, reqSchedID, tgtSchedID, reqStart.Time, reqEnd.Time).Scan(&tgtOverlap); err != nil {
			return nil, err
		}
		if tgtOverlap > 0 {
			return nil, newAppError(409, "schedule_conflict", "swap creates a schedule conflict for target officer", ErrConflict)
		}
	}

	// Swap owners atomically on both schedules and their attendance records.
	// Temporarily set is_active=false on reqSchedID to prevent intermediate collisions on active assignment unique index.
	if _, err := tx.ExecContext(ctx, `UPDATE security_schedules SET is_active = false WHERE id = $1`, reqSchedID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE security_schedules SET user_id = $2, membership_id = $3, updated_by = $4, updated_at = $5
		WHERE id = $1
	`, tgtSchedID, reqUser, reqMembershipID, approverID, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE security_attendance_records SET user_id = $2, updated_at = $3
		WHERE schedule_id = $1
	`, tgtSchedID, reqUser, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE security_schedules SET user_id = $2, membership_id = $3, updated_by = $4, updated_at = $5, is_active = true
		WHERE id = $1
	`, reqSchedID, tgtUser, tgtMembershipID, approverID, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE security_attendance_records SET user_id = $2, updated_at = $3
		WHERE schedule_id = $1
	`, reqSchedID, tgtUser, now); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE security_shift_swaps
		SET status = 'approved', ketua_decided_by = $2, ketua_decided_at = $3, ketua_note = $4, updated_at = $3
		WHERE id = $1
	`, swapID, approverID, now, note); err != nil {
		return nil, err
	}

	_ = insertAuditLog(ctx, tx, rtID, &approverID, "security_swap_approved", "security_shift_swap", swapID,
		map[string]any{"requester_schedule_id": reqSchedID, "requester_user_id": reqUser, "target_schedule_id": tgtSchedID, "target_user_id": tgtUser},
		map[string]any{"requester_schedule_id": reqSchedID, "new_owner": tgtUser, "target_schedule_id": tgtSchedID, "new_owner_target": reqUser},
		ip,
	)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSwapByID(ctx, rtID, swapID)
}

func (r *Repository) GetSwapByID(ctx context.Context, rtID, swapID string) (*ShiftSwap, error) {
	var sw ShiftSwap
	var tgtAt, ketAt sql.NullTime
	var tgtNote, ketBy, ketNote sql.NullString
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT sw.id, sw.rt_id, sw.requester_user_id, ru.full_name,
		       sw.target_user_id, tu.full_name,
		       sw.requester_schedule_id, COALESCE(to_char(rs.shift_date, 'YYYY-MM-DD'), ''), rs.shift_name_snapshot,
		       sw.target_schedule_id, COALESCE(to_char(ts.shift_date, 'YYYY-MM-DD'), ''), ts.shift_name_snapshot,
		       sw.reason, sw.status, sw.target_responded_at, sw.target_note,
		       sw.ketua_decided_by, sw.ketua_decided_at, sw.ketua_note, sw.created_at, sw.updated_at
		FROM security_shift_swaps sw
		JOIN users ru ON ru.id = sw.requester_user_id
		JOIN users tu ON tu.id = sw.target_user_id
		JOIN security_schedules rs ON rs.id = sw.requester_schedule_id
		JOIN security_schedules ts ON ts.id = sw.target_schedule_id
		WHERE sw.id = $1 AND sw.rt_id = $2
	`, swapID, rtID).Scan(
		&sw.ID, &sw.RTID, &sw.RequesterUserID, &sw.RequesterName,
		&sw.TargetUserID, &sw.TargetName,
		&sw.RequesterScheduleID, &sw.RequesterShiftDate, &sw.RequesterShiftName,
		&sw.TargetScheduleID, &sw.TargetShiftDate, &sw.TargetShiftName,
		&sw.Reason, &sw.Status, &tgtAt, &tgtNote, &ketBy, &ketAt, &ketNote, &sw.CreatedAt, &sw.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "swap request not found", ErrNotFound)
		}
		return nil, err
	}
	if tgtAt.Valid {
		t := tgtAt.Time
		sw.TargetRespondedAt = &t
	}
	if tgtNote.Valid {
		v := tgtNote.String
		sw.TargetNote = &v
	}
	if ketBy.Valid {
		v := ketBy.String
		sw.KetuaDecidedBy = &v
	}
	if ketAt.Valid {
		t := ketAt.Time
		sw.KetuaDecidedAt = &t
	}
	if ketNote.Valid {
		v := ketNote.String
		sw.KetuaNote = &v
	}
	return &sw, nil
}

func (r *Repository) ListSwaps(ctx context.Context, rtID, userFilter, statusFilter string) ([]ShiftSwap, error) {
	where := []string{"sw.rt_id = $1"}
	args := []any{rtID}
	idx := 2
	if userFilter != "" {
		where = append(where, fmt.Sprintf("(sw.requester_user_id = $%d OR sw.target_user_id = $%d)", idx, idx))
		args = append(args, userFilter)
		idx++
	}
	if statusFilter != "" {
		where = append(where, fmt.Sprintf("sw.status = $%d", idx))
		args = append(args, statusFilter)
		idx++
	}

	q := fmt.Sprintf(`
		SELECT sw.id, sw.rt_id, sw.requester_user_id, ru.full_name,
		       sw.target_user_id, tu.full_name,
		       sw.requester_schedule_id, COALESCE(to_char(rs.shift_date, 'YYYY-MM-DD'), ''), rs.shift_name_snapshot,
		       sw.target_schedule_id, COALESCE(to_char(ts.shift_date, 'YYYY-MM-DD'), ''), ts.shift_name_snapshot,
		       sw.reason, sw.status, sw.target_responded_at, sw.target_note,
		       sw.ketua_decided_by, sw.ketua_decided_at, sw.ketua_note, sw.created_at, sw.updated_at
		FROM security_shift_swaps sw
		JOIN users ru ON ru.id = sw.requester_user_id
		JOIN users tu ON tu.id = sw.target_user_id
		JOIN security_schedules rs ON rs.id = sw.requester_schedule_id
		JOIN security_schedules ts ON ts.id = sw.target_schedule_id
		WHERE %s
		ORDER BY sw.created_at DESC
	`, strings.Join(where, " AND "))

	rows, err := r.pool.Raw().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ShiftSwap
	for rows.Next() {
		var sw ShiftSwap
		var tgtAt, ketAt sql.NullTime
		var tgtNote, ketBy, ketNote sql.NullString
		if err := rows.Scan(
			&sw.ID, &sw.RTID, &sw.RequesterUserID, &sw.RequesterName,
			&sw.TargetUserID, &sw.TargetName,
			&sw.RequesterScheduleID, &sw.RequesterShiftDate, &sw.RequesterShiftName,
			&sw.TargetScheduleID, &sw.TargetShiftDate, &sw.TargetShiftName,
			&sw.Reason, &sw.Status, &tgtAt, &tgtNote, &ketBy, &ketAt, &ketNote, &sw.CreatedAt, &sw.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if tgtAt.Valid {
			t := tgtAt.Time
			sw.TargetRespondedAt = &t
		}
		if tgtNote.Valid {
			v := tgtNote.String
			sw.TargetNote = &v
		}
		if ketBy.Valid {
			v := ketBy.String
			sw.KetuaDecidedBy = &v
		}
		if ketAt.Valid {
			t := ketAt.Time
			sw.KetuaDecidedAt = &t
		}
		if ketNote.Valid {
			v := ketNote.String
			sw.KetuaNote = &v
		}
		list = append(list, sw)
	}
	if list == nil {
		list = []ShiftSwap{}
	}
	return list, rows.Err()
}

// ── QR Tokens & Atomic Attendance Scan ────────────────────────────────────────

func (r *Repository) CreateQRToken(ctx context.Context, rtID, deviceID, tokenHash string, issuedAt, expiresAt time.Time) (string, error) {
	var tokenID string
	err := r.pool.Raw().QueryRowContext(ctx, `
		INSERT INTO security_qr_tokens (rt_id, device_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, rtID, deviceID, tokenHash, expiresAt, issuedAt).Scan(&tokenID)
	return tokenID, err
}

func (r *Repository) ConsumeQRAndRecordAttendance(
	ctx context.Context,
	rtID, userID, rawQRToken, action, shiftConfigID, idemKey string,
	now time.Time,
) (*AttendanceScanResult, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Idempotency replay guard for transient mobile network retries.
	var scopedIdemKey string
	if strings.TrimSpace(idemKey) != "" {
		scopedIdemKey = "secscan:" + userID + ":" + strings.TrimSpace(idemKey)
		var respCode int
		var respBody string
		err := tx.QueryRowContext(ctx, `
			SELECT response_code, response_body
			FROM idempotency_keys
			WHERE rt_id = $1 AND key = $2
		`, rtID, scopedIdemKey).Scan(&respCode, &respBody)
		if err == nil && respCode >= 200 && respCode < 300 {
			var cached AttendanceScanResult
			if uErr := json.Unmarshal([]byte(respBody), &cached); uErr == nil {
				return &cached, nil
			}
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	if _, err := r.ValidateActiveOfficer(ctx, tx, rtID, userID); err != nil {
		return nil, err
	}

	// Lock QR token row atomically for single-use anti-replay protection.
	tokenHash := HashToken(strings.TrimSpace(rawQRToken))
	var qrID, qrRTID, deviceID, deviceName string
	var deviceActive bool
	var deviceRevoked, qrUsedAt, qrInvalidated sql.NullTime
	var qrExpiresAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT q.id, q.rt_id, q.device_id, d.device_name, d.is_active, d.revoked_at,
		       q.expires_at, q.used_at, q.invalidated_at
		FROM security_qr_tokens q
		JOIN security_devices d ON d.id = q.device_id
		WHERE q.token_hash = $1
		FOR UPDATE OF q
	`, tokenHash).Scan(&qrID, &qrRTID, &deviceID, &deviceName, &deviceActive, &deviceRevoked, &qrExpiresAt, &qrUsedAt, &qrInvalidated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(400, "invalid_qr", "invalid QR code", ErrQRInvalid)
		}
		return nil, err
	}

	if qrRTID != rtID {
		return nil, newAppError(403, "cross_rt_denied", "QR code belongs to another RT", ErrCrossRT)
	}
	if !deviceActive || deviceRevoked.Valid {
		return nil, newAppError(401, "device_revoked", "security pos device has been revoked or deactivated", ErrDeviceRevoked)
	}
	if qrUsedAt.Valid {
		return nil, newAppError(409, "qr_already_used", "QR code has already been used", ErrQRAlreadyUsed)
	}
	if qrInvalidated.Valid {
		return nil, newAppError(400, "invalid_qr", "QR code is no longer valid", ErrQRInvalid)
	}
	if now.After(qrExpiresAt) {
		return nil, newAppError(400, "qr_expired", "QR code has expired", ErrQRExpired)
	}

	// Find candidate active schedules for this officer near server `now`.
	nowJkt := now.In(JakartaLoc)
	todayStr := nowJkt.Format("2006-01-02")
	yestStr := nowJkt.AddDate(0, 0, -1).Format("2006-01-02")
	windowStartUpper := now.Add(12 * time.Hour)
	windowEndLower := now.Add(-14 * time.Hour)
	rows, err := tx.QueryContext(ctx, `
		SELECT s.id, s.shift_config_id, s.shift_version, to_char(s.shift_date, 'YYYY-MM-DD'), to_char(ar.shift_date, 'YYYY-MM-DD'),
		       s.shift_name_snapshot, to_char(s.start_time_snapshot, 'HH24:MI'), to_char(s.end_time_snapshot, 'HH24:MI'),
		       s.crosses_midnight_snapshot, s.lateness_tolerance_minutes,
		       s.check_in_window_before_minutes, s.check_in_window_after_minutes,
		       s.absent_cutoff_minutes, s.missing_checkout_cutoff_minutes,
		       s.scheduled_start_at, s.scheduled_end_at,
		       ar.id, ar.status, ar.checked_in_at, ar.checked_out_at
		FROM security_schedules s
		LEFT JOIN LATERAL (
			SELECT id, shift_date, status, checked_in_at, checked_out_at
			FROM security_attendance_records
			WHERE schedule_id = s.id
			  AND (
			        (s.shift_date IS NOT NULL AND shift_date = s.shift_date)
			        OR (s.shift_date IS NULL AND shift_date IN ($5::date, $6::date))
			      )
			ORDER BY checked_in_at DESC NULLS LAST, created_at DESC
			LIMIT 1
		) ar ON true
		WHERE s.rt_id = $1
		  AND s.user_id = $2
		  AND s.status = 'scheduled'
		  AND s.is_active = true
		  AND (
		        s.scheduled_start_at IS NULL
		        OR (s.scheduled_start_at <= $3 AND s.scheduled_end_at >= $4)
		      )
		ORDER BY s.created_at ASC
		FOR UPDATE OF s
	`, rtID, userID, windowStartUpper, windowEndLower, todayStr, yestStr)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		scheduleID, shiftConfigID, shiftDate, shiftName, startTime, endTime string
		shiftVersion, tolMin, winBefore, winAfter, absentMin, missOutMin    int
		crossesMidnight, isContinuous                                       bool
		schedStart, schedEnd                                                time.Time
		attID, attStatus                                                    sql.NullString
		checkedIn, checkedOut                                               sql.NullTime
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		var schedDateNull, arDateNull sql.NullString
		var schedStartNull, schedEndNull sql.NullTime
		if err := rows.Scan(
			&c.scheduleID, &c.shiftConfigID, &c.shiftVersion, &schedDateNull, &arDateNull,
			&c.shiftName, &c.startTime, &c.endTime, &c.crossesMidnight,
			&c.tolMin, &c.winBefore, &c.winAfter, &c.absentMin, &c.missOutMin,
			&schedStartNull, &schedEndNull,
			&c.attID, &c.attStatus, &c.checkedIn, &c.checkedOut,
		); err != nil {
			rows.Close()
			return nil, err
		}
		c.isContinuous = !schedDateNull.Valid
		if schedDateNull.Valid {
			c.shiftDate = schedDateNull.String
		} else if arDateNull.Valid {
			c.shiftDate = arDateNull.String
		}
		if schedStartNull.Valid && schedEndNull.Valid {
			c.schedStart = schedStartNull.Time
			c.schedEnd = schedEndNull.Time
		} else {
			// Compute dynamic wall-clock bounds around `now` (Asia/Jakarta)
			nowJkt := now.In(JakartaLoc)
			sh, sm, _, err1 := ParseHHMM(c.startTime)
			eh, em, _, err2 := ParseHHMM(c.endTime)
			if err1 == nil && err2 == nil {
				tTodayStart := time.Date(nowJkt.Year(), nowJkt.Month(), nowJkt.Day(), sh, sm, 0, 0, JakartaLoc)
				tTodayEnd := time.Date(nowJkt.Year(), nowJkt.Month(), nowJkt.Day(), eh, em, 0, 0, JakartaLoc)
				if c.crossesMidnight || !tTodayEnd.After(tTodayStart) {
					tTodayEnd = tTodayEnd.AddDate(0, 0, 1)
				}
				tYestStart := tTodayStart.AddDate(0, 0, -1)
				tYestEnd := tTodayEnd.AddDate(0, 0, -1)

				// For cross-midnight shift: if officer has an open check-in during yesterday's window, or current time is in yesterday's checkout window
				if c.crossesMidnight && c.checkedIn.Valid && !c.checkedOut.Valid && !c.checkedIn.Time.Before(tYestStart.Add(-time.Duration(c.winBefore)*time.Minute)) && c.checkedIn.Time.Before(tTodayStart.Add(-time.Duration(c.winBefore)*time.Minute)) {
					c.schedStart = tYestStart.UTC()
					c.schedEnd = tYestEnd.UTC()
					c.shiftDate = tYestStart.Format("2006-01-02")
				} else if c.crossesMidnight && nowJkt.Before(tTodayStart.Add(-time.Duration(c.winBefore)*time.Minute)) && !nowJkt.After(tYestEnd.Add(time.Duration(c.missOutMin)*time.Minute)) {
					c.schedStart = tYestStart.UTC()
					c.schedEnd = tYestEnd.UTC()
					c.shiftDate = tYestStart.Format("2006-01-02")
				} else {
					c.schedStart = tTodayStart.UTC()
					c.schedEnd = tTodayEnd.UTC()
					c.shiftDate = tTodayStart.Format("2006-01-02")
				}
			}
		}

		// In continuous date-independent shifts, a previous completed/closed session from an earlier calendar date
		// must not block attendance on a new date.
		if c.isContinuous && arDateNull.Valid && arDateNull.String != c.shiftDate {
			c.attID = sql.NullString{}
			c.attStatus = sql.NullString{}
			c.checkedIn = sql.NullTime{}
			c.checkedOut = sql.NullTime{}
		}

		candidates = append(candidates, c)
	}
	rows.Close()
	if len(candidates) == 0 {
		return nil, newAppError(400, "no_matching_schedule", "you do not have an active scheduled shift right now", ErrNoActiveSchedule)
	}

	targetShiftConfigID := strings.TrimSpace(shiftConfigID)
	if targetShiftConfigID != "" {
		var filtered []candidate
		for _, c := range candidates {
			if c.shiftConfigID == targetShiftConfigID || c.scheduleID == targetShiftConfigID {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
	}

	reqAction := strings.ToLower(strings.TrimSpace(action))
	if reqAction == "" {
		reqAction = "auto"
	}
	if reqAction != "auto" && reqAction != "check_in" && reqAction != "check_out" {
		return nil, newAppError(400, "validation_error", "action must be check_in, check_out, or auto", ErrValidation)
	}

	// Select best candidate matching the requested action.
	var chosen *candidate
	resolvedAction := reqAction

	findOpenForCheckout := func() *candidate {
		for i := range candidates {
			c := &candidates[i]
			st := StatusBelumHadir
			if c.attStatus.Valid {
				st = c.attStatus.String
			}
			if c.checkedIn.Valid && !c.checkedOut.Valid && (st == StatusHadir || st == StatusTerlambat) {
				maxOut := c.schedEnd.Add(time.Duration(c.missOutMin) * time.Minute)
				if !now.After(maxOut) {
					return c
				}
			}
		}
		return nil
	}

	findOpenForCheckin := func() *candidate {
		for i := range candidates {
			c := &candidates[i]
			st := StatusBelumHadir
			if c.attStatus.Valid {
				st = c.attStatus.String
			}
			if c.checkedIn.Valid || st != StatusBelumHadir {
				continue
			}
			openAt := c.schedStart.Add(-time.Duration(c.winBefore) * time.Minute)
			closeAt := c.schedStart.Add(time.Duration(c.winAfter) * time.Minute)
			if !now.Before(openAt) && !now.After(closeAt) {
				return c
			}
		}
		return nil
	}

	switch reqAction {
	case "check_out":
		chosen = findOpenForCheckout()
		if chosen == nil {
			// Give specific reason if already checked out vs not yet checked in.
			for i := range candidates {
				c := &candidates[i]
				if c.checkedOut.Valid || (c.attStatus.Valid && c.attStatus.String == StatusSudahAbsenPulang) {
					return nil, newAppError(409, "already_checked_out", "you have already checked out for this shift", ErrAlreadyCheckedOut)
				}
			}
			return nil, newAppError(400, "checkout_before_checkin", "cannot check out before checking in", ErrCheckoutBeforeIn)
		}
	case "check_in":
		chosen = findOpenForCheckin()
		if chosen == nil {
			for i := range candidates {
				c := &candidates[i]
				openAt := c.schedStart.Add(-time.Duration(c.winBefore) * time.Minute)
				closeAt := c.schedStart.Add(time.Duration(c.winAfter) * time.Minute)
				inWindow := !now.Before(openAt) && !now.After(closeAt)
				if inWindow {
					if c.checkedIn.Valid && !c.checkedOut.Valid {
						return nil, newAppError(409, "already_checked_in", "you have already checked in for this shift", ErrAlreadyCheckedIn)
					}
					if c.attStatus.Valid && c.attStatus.String == StatusSudahAbsenPulang {
						return nil, newAppError(409, "already_checked_out", "shift attendance is already completed", ErrAlreadyCheckedOut)
					}
					if c.attStatus.Valid && c.attStatus.String == StatusTidakHadir {
						return nil, newAppError(400, "check_in_window_closed", "shift was already marked absent", ErrWindowClosed)
					}
				}
			}
			return nil, newAppError(400, "check_in_window_closed", "current time is outside your shift check-in window", ErrWindowClosed)
		}
	case "auto":
		if c := findOpenForCheckout(); c != nil {
			chosen = c
			resolvedAction = "check_out"
		} else if c := findOpenForCheckin(); c != nil {
			chosen = c
			resolvedAction = "check_in"
		} else {
			for i := range candidates {
				c := &candidates[i]
				openAt := c.schedStart.Add(-time.Duration(c.winBefore) * time.Minute)
				closeAt := c.schedStart.Add(time.Duration(c.winAfter) * time.Minute)
				inWindow := !now.Before(openAt) && !now.After(closeAt)
				if inWindow && c.attStatus.Valid && c.attStatus.String == StatusSudahAbsenPulang {
					return nil, newAppError(409, "already_checked_out", "you have already completed attendance for this shift", ErrAlreadyCheckedOut)
				}
			}
			return nil, newAppError(400, "check_in_window_closed", "current time is outside the allowed attendance window", ErrWindowClosed)
		}
	}

	// Serialize concurrent attempts for this officer's schedule and date using transaction advisory lock
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1 || ':' || $2 || ':' || $3))`, rtID, chosen.scheduleID, chosen.shiftDate); err != nil {
		return nil, err
	}

	// Ensure attendance row exists, then lock it directly via FOR UPDATE and re-verify state under READ COMMITTED.
	if !chosen.attID.Valid {
		var insertedID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO security_attendance_records (
				rt_id, schedule_id, user_id, shift_config_id, shift_version, shift_date, status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6::date, 'belum_hadir', $7, $7)
			ON CONFLICT (schedule_id, shift_date) DO UPDATE SET updated_at = security_attendance_records.updated_at
			RETURNING id
		`, rtID, chosen.scheduleID, userID, chosen.shiftConfigID, chosen.shiftVersion, chosen.shiftDate, now).Scan(&insertedID)
		if err != nil {
			return nil, err
		}
		chosen.attID = sql.NullString{String: insertedID, Valid: true}
	}

	var lockedStatus string
	var lockedCin, lockedCout sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT status, checked_in_at, checked_out_at
		FROM security_attendance_records
		WHERE id = $1
		FOR UPDATE
	`, chosen.attID.String).Scan(&lockedStatus, &lockedCin, &lockedCout); err != nil {
		return nil, err
	}
	if resolvedAction == "check_in" {
		if lockedCin.Valid || lockedStatus != StatusBelumHadir {
			return nil, newAppError(409, "already_checked_in", "you have already checked in for this shift", ErrAlreadyCheckedIn)
		}
	} else {
		if !lockedCin.Valid || (lockedStatus != StatusHadir && lockedStatus != StatusTerlambat) {
			return nil, newAppError(400, "checkout_before_checkin", "cannot check out before checking in", ErrCheckoutBeforeIn)
		}
		if lockedCout.Valid {
			return nil, newAppError(409, "already_checked_out", "you have already checked out for this shift", ErrAlreadyCheckedOut)
		}
	}

	if resolvedAction == "check_in" {
		deadline := chosen.schedStart.Add(time.Duration(chosen.tolMin) * time.Minute)
		status := StatusHadir
		arrival := StatusHadir
		lateMinutes := 0
		if now.After(deadline) {
			status = StatusTerlambat
			arrival = StatusTerlambat
			lateMinutes = int(now.Sub(chosen.schedStart) / time.Minute)
			if lateMinutes < 1 {
				lateMinutes = 1
			}
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE security_attendance_records
			SET status = $2,
			    arrival_status = $3,
			    late_minutes = $4,
			    check_in_device_id = $5,
			    check_in_qr_token_id = $6,
			    checked_in_at = $7,
			    updated_at = $7
			WHERE id = $1
		`, chosen.attID.String, status, arrival, lateMinutes, deviceID, qrID, now); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE security_attendance_records
			SET status = 'sudah_absen_pulang',
			    check_out_device_id = $2,
			    check_out_qr_token_id = $3,
			    checked_out_at = $4,
			    updated_at = $4
			WHERE id = $1
		`, chosen.attID.String, deviceID, qrID, now); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE security_qr_tokens
		SET used_at = $2, used_by_user_id = $3, used_for_action = $4
		WHERE id = $1
	`, qrID, now, userID, resolvedAction); err != nil {
		return nil, err
	}

	rec, err := r.getAttendanceByIDTx(ctx, tx, rtID, chosen.attID.String)
	if err != nil {
		return nil, err
	}

	msg := "Check-in berhasil dicatat"
	if resolvedAction == "check_out" {
		msg = "Check-out berhasil dicatat"
	} else if rec.Status == StatusTerlambat {
		msg = fmt.Sprintf("Check-in tercatat (Terlambat %d menit)", rec.LateMinutes)
	}

	res := &AttendanceScanResult{
		Action:     resolvedAction,
		Record:     *rec,
		ServerTime: now.In(JakartaLoc),
		DeviceID:   deviceID,
		DeviceName: deviceName,
		Message:    msg,
	}

	if scopedIdemKey != "" {
		if bodyBytes, mErr := json.Marshal(res); mErr == nil {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO idempotency_keys (rt_id, key, user_id, request_path, request_hash, response_code, response_body)
				VALUES ($1, $2, $3, '/api/v1/security/attendance/scan', $4, 200, $5)
				ON CONFLICT (rt_id, key) DO NOTHING
			`, rtID, scopedIdemKey, userID, tokenHash, string(bodyBytes)); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *Repository) getAttendanceByIDTx(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, rtID, id string) (*AttendanceRecord, error) {
	var rec AttendanceRecord
	var arrStatus, inDevID, inDevName, outDevID, outDevName sql.NullString
	var cin, cout, absAt, missAt, schedStartNull, schedEndNull sql.NullTime
	err := q.QueryRowContext(ctx, `
		SELECT ar.id, ar.rt_id, ar.schedule_id, ar.user_id, u.full_name,
		       ar.shift_config_id, ar.shift_version, to_char(ar.shift_date, 'YYYY-MM-DD'),
		       s.shift_name_snapshot, to_char(s.start_time_snapshot, 'HH24:MI'), to_char(s.end_time_snapshot, 'HH24:MI'),
		       s.crosses_midnight_snapshot, s.scheduled_start_at, s.scheduled_end_at,
		       ar.status, ar.arrival_status, ar.late_minutes,
		       ar.check_in_device_id, ind.device_name,
		       ar.check_out_device_id, outd.device_name,
		       ar.checked_in_at, ar.checked_out_at, ar.marked_absent_at, ar.marked_missing_checkout_at,
		       ar.correction_locked, ar.created_at, ar.updated_at
		FROM security_attendance_records ar
		JOIN security_schedules s ON s.id = ar.schedule_id
		JOIN users u ON u.id = ar.user_id
		LEFT JOIN security_devices ind ON ind.id = ar.check_in_device_id
		LEFT JOIN security_devices outd ON outd.id = ar.check_out_device_id
		WHERE ar.id = $1 AND ar.rt_id = $2
	`, id, rtID).Scan(
		&rec.ID, &rec.RTID, &rec.ScheduleID, &rec.UserID, &rec.OfficerName,
		&rec.ShiftConfigID, &rec.ShiftVersion, &rec.ShiftDate,
		&rec.ShiftName, &rec.StartTime, &rec.EndTime, &rec.CrossesMidnight,
		&schedStartNull, &schedEndNull,
		&rec.Status, &arrStatus, &rec.LateMinutes,
		&inDevID, &inDevName, &outDevID, &outDevName,
		&cin, &cout, &absAt, &missAt, &rec.CorrectionLocked, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if schedStartNull.Valid {
		rec.ScheduledStartAt = schedStartNull.Time
	}
	if schedEndNull.Valid {
		rec.ScheduledEndAt = schedEndNull.Time
	}
	assignAttendanceOptionals(&rec, arrStatus, inDevID, inDevName, outDevID, outDevName, cin, cout, absAt, missAt)
	return &rec, nil
}

func assignAttendanceOptionals(
	rec *AttendanceRecord,
	arrStatus, inDevID, inDevName, outDevID, outDevName sql.NullString,
	cin, cout, absAt, missAt sql.NullTime,
) {
	rec.StatusLabel = StatusLabel(rec.Status)
	if arrStatus.Valid {
		v := arrStatus.String
		rec.ArrivalStatus = &v
	}
	if inDevID.Valid {
		v := inDevID.String
		rec.CheckInDeviceID = &v
	}
	if inDevName.Valid {
		v := inDevName.String
		rec.CheckInDeviceName = &v
	}
	if outDevID.Valid {
		v := outDevID.String
		rec.CheckOutDeviceID = &v
	}
	if outDevName.Valid {
		v := outDevName.String
		rec.CheckOutDeviceName = &v
	}
	if cin.Valid {
		t := cin.Time
		rec.CheckedInAt = &t
	}
	if cout.Valid {
		t := cout.Time
		rec.CheckedOutAt = &t
	}
	if absAt.Valid {
		t := absAt.Time
		rec.MarkedAbsentAt = &t
	}
	if missAt.Valid {
		t := missAt.Time
		rec.MarkedMissingCheckoutAt = &t
	}
}

type AttendanceFilter struct {
	DateFrom      string
	DateTo        string
	UserID        string
	ShiftConfigID string
	Status        string
	Page          int
	PerPage       int
}

func (r *Repository) ListAttendance(ctx context.Context, rtID string, f AttendanceFilter) ([]AttendanceRecord, int, error) {
	where := []string{"ar.rt_id = $1"}
	args := []any{rtID}
	idx := 2

	if f.DateFrom != "" {
		where = append(where, fmt.Sprintf("ar.shift_date >= $%d::date", idx))
		args = append(args, f.DateFrom)
		idx++
	}
	if f.DateTo != "" {
		where = append(where, fmt.Sprintf("ar.shift_date <= $%d::date", idx))
		args = append(args, f.DateTo)
		idx++
	}
	if f.UserID != "" {
		where = append(where, fmt.Sprintf("ar.user_id = $%d", idx))
		args = append(args, f.UserID)
		idx++
	}
	if f.ShiftConfigID != "" {
		where = append(where, fmt.Sprintf("ar.shift_config_id = $%d", idx))
		args = append(args, f.ShiftConfigID)
		idx++
	}
	if f.Status != "" {
		where = append(where, fmt.Sprintf("ar.status = $%d", idx))
		args = append(args, f.Status)
		idx++
	}

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_attendance_records ar
		JOIN security_schedules s ON s.id = ar.schedule_id
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.PerPage
	if limit <= 0 {
		limit = 20
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT ar.id, ar.rt_id, ar.schedule_id, ar.user_id, u.full_name,
		       ar.shift_config_id, ar.shift_version, to_char(ar.shift_date, 'YYYY-MM-DD'),
		       s.shift_name_snapshot, to_char(s.start_time_snapshot, 'HH24:MI'), to_char(s.end_time_snapshot, 'HH24:MI'),
		       s.crosses_midnight_snapshot, s.scheduled_start_at, s.scheduled_end_at,
		       ar.status, ar.arrival_status, ar.late_minutes,
		       ar.check_in_device_id, ind.device_name,
		       ar.check_out_device_id, outd.device_name,
		       ar.checked_in_at, ar.checked_out_at, ar.marked_absent_at, ar.marked_missing_checkout_at,
		       ar.correction_locked, ar.created_at, ar.updated_at
		FROM security_attendance_records ar
		JOIN security_schedules s ON s.id = ar.schedule_id
		JOIN users u ON u.id = ar.user_id
		LEFT JOIN security_devices ind ON ind.id = ar.check_in_device_id
		LEFT JOIN security_devices outd ON outd.id = ar.check_out_device_id
		WHERE %s
		ORDER BY ar.shift_date ASC, s.scheduled_start_at ASC, ar.id ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, idx, idx+1)

	rows, err := r.pool.Raw().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []AttendanceRecord
	for rows.Next() {
		var rec AttendanceRecord
		var arrStatus, inDevID, inDevName, outDevID, outDevName sql.NullString
		var cin, cout, absAt, missAt, schedStartNull, schedEndNull sql.NullTime
		if err := rows.Scan(
			&rec.ID, &rec.RTID, &rec.ScheduleID, &rec.UserID, &rec.OfficerName,
			&rec.ShiftConfigID, &rec.ShiftVersion, &rec.ShiftDate,
			&rec.ShiftName, &rec.StartTime, &rec.EndTime, &rec.CrossesMidnight,
			&schedStartNull, &schedEndNull,
			&rec.Status, &arrStatus, &rec.LateMinutes,
			&inDevID, &inDevName, &outDevID, &outDevName,
			&cin, &cout, &absAt, &missAt, &rec.CorrectionLocked, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if schedStartNull.Valid {
			rec.ScheduledStartAt = schedStartNull.Time
		}
		if schedEndNull.Valid {
			rec.ScheduledEndAt = schedEndNull.Time
		}
		assignAttendanceOptionals(&rec, arrStatus, inDevID, inDevName, outDevID, outDevName, cin, cout, absAt, missAt)
		list = append(list, rec)
	}
	if list == nil {
		list = []AttendanceRecord{}
	}
	return list, total, rows.Err()
}

// ── Corrections & Audit Trail ─────────────────────────────────────────────────

func (r *Repository) CreateCorrection(
	ctx context.Context,
	rtID, requesterID string,
	isManager bool,
	req CreateCorrectionRequest,
	reqCin, reqCout *time.Time,
	now time.Time,
) (*AttendanceCorrection, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var attRecordID string
	var scheduleID, officerUserID, origStatus string
	var origCin, origCout sql.NullTime
	var shiftDateStr, startTimeStr string
	var winBefore int

	if strings.TrimSpace(req.AttendanceRecordID) != "" {
		attRecordID = strings.TrimSpace(req.AttendanceRecordID)
		err = tx.QueryRowContext(ctx, `
			SELECT ar.schedule_id, ar.user_id, ar.status, ar.checked_in_at, ar.checked_out_at,
			       to_char(ar.shift_date, 'YYYY-MM-DD'),
			       to_char(COALESCE(s.start_time_snapshot, '00:00'::time), 'HH24:MI'),
			       COALESCE(s.check_in_window_before_minutes, 15)
			FROM security_attendance_records ar
			LEFT JOIN security_schedules s ON s.id = ar.schedule_id
			WHERE ar.id = $1 AND ar.rt_id = $2
			FOR UPDATE OF ar
		`, attRecordID, rtID).Scan(&scheduleID, &officerUserID, &origStatus, &origCin, &origCout, &shiftDateStr, &startTimeStr, &winBefore)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, newAppError(404, "not_found", "attendance record not found", ErrNotFound)
			}
			return nil, err
		}
	} else if strings.TrimSpace(req.ScheduleID) != "" && strings.TrimSpace(req.ShiftDate) != "" {
		scheduleID = strings.TrimSpace(req.ScheduleID)
		shiftDateStr = strings.TrimSpace(req.ShiftDate)
		var shiftConfigID string
		var shiftVersion int
		err = tx.QueryRowContext(ctx, `
			SELECT s.user_id, s.shift_config_id, s.shift_version,
			       to_char(COALESCE(s.start_time_snapshot, '00:00'::time), 'HH24:MI'),
			       COALESCE(s.check_in_window_before_minutes, 15)
			FROM security_schedules s
			WHERE s.id = $1 AND s.rt_id = $2 AND s.is_active = true
			FOR UPDATE
		`, scheduleID, rtID).Scan(&officerUserID, &shiftConfigID, &shiftVersion, &startTimeStr, &winBefore)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, newAppError(404, "not_found", "schedule assignment not found", ErrNotFound)
			}
			return nil, err
		}

		// Idempotently ensure the attendance row exists for (schedule_id, shift_date)
		err = tx.QueryRowContext(ctx, `
			INSERT INTO security_attendance_records (
				rt_id, schedule_id, user_id, shift_config_id, shift_version, shift_date, status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6::date, 'belum_hadir', $7, $7)
			ON CONFLICT (schedule_id, shift_date) DO UPDATE SET updated_at = security_attendance_records.updated_at
			RETURNING id, status, checked_in_at, checked_out_at
		`, rtID, scheduleID, officerUserID, shiftConfigID, shiftVersion, shiftDateStr, now).
			Scan(&attRecordID, &origStatus, &origCin, &origCout)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, newAppError(400, "validation_error", "attendance_record_id or (schedule_id and shift_date) is required", ErrValidation)
	}

	if !isManager && officerUserID != requesterID {
		return nil, newAppError(403, "forbidden", "you can only request corrections for your own attendance", ErrForbidden)
	}

	// Verify occurrence has started (cannot correct future shifts before they start)
	if !origCin.Valid && shiftDateStr != "" && startTimeStr != "" {
		sh, sm, _, err1 := ParseHHMM(startTimeStr)
		if err1 == nil {
			tShiftDate, err2 := time.Parse("2006-01-02", shiftDateStr)
			if err2 == nil {
				occStart := time.Date(tShiftDate.Year(), tShiftDate.Month(), tShiftDate.Day(), sh, sm, 0, 0, JakartaLoc)
				allowedEarliest := occStart.Add(-time.Duration(winBefore) * time.Minute)
				nowJkt := now.In(JakartaLoc)
				if nowJkt.Before(allowedEarliest) {
					return nil, newAppError(400, "cannot_correct_future_shift", "cannot request correction for a future shift that has not started yet", ErrValidation)
				}
			}
		}
	}

	var pendingCnt int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_attendance_corrections
		WHERE attendance_record_id = $1 AND status = 'pending'
	`, attRecordID).Scan(&pendingCnt); err != nil {
		return nil, err
	}
	if pendingCnt > 0 {
		return nil, newAppError(409, "correction_already_pending", "this attendance record already has a pending correction", ErrConflict)
	}

	var corrID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO security_attendance_corrections (
			rt_id, attendance_record_id, schedule_id, officer_user_id, requested_by,
			request_type, original_status, original_checked_in_at, original_checked_out_at,
			requested_status, requested_checked_in_at, requested_checked_out_at,
			reason, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'pending', $14, $14)
		RETURNING id
	`, rtID, attRecordID, scheduleID, officerUserID, requesterID,
		req.RequestType, origStatus, origCin, origCout,
		req.RequestedStatus, reqCin, reqCout, strings.TrimSpace(req.Reason), now,
	).Scan(&corrID)
	if err != nil {
		return nil, err
	}

	// Notify Ketua RT of the new correction request.
	var officerName string
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(full_name, 'Petugas Keamanan') FROM users WHERE id = $1
	`, officerUserID).Scan(&officerName)
	notifyRows, nErr := tx.QueryContext(ctx, `
		SELECT DISTINCT u.id
		FROM users u
		JOIN user_rt_memberships urm ON urm.user_id = u.id
		WHERE urm.rt_id = $1 AND urm.is_active = true AND urm.jabatan = 'ketua'
	`, rtID)
	if nErr == nil {
		for notifyRows.Next() {
			var ketuaID string
			if err := notifyRows.Scan(&ketuaID); err == nil {
				// Skip notifying the requester themselves.
				if ketuaID == requesterID {
					continue
				}
				dedup := fmt.Sprintf("correction:%s:%s", corrID, ketuaID)
				_, _ = tx.ExecContext(ctx, `
					INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at, payload)
					VALUES ($1::uuid, $2::uuid, 'correction_submitted', $3, 'Pengajuan Koreksi Absensi', $4, 'SENT', $5, json_build_object(
						'correction_id', $6::text,
						'officer_user_id', $7::text,
						'officer_name', $8::text,
						'shift_date', $9::text
					)::jsonb)
					ON CONFLICT (dedup_key) DO NOTHING
				`, rtID, ketuaID, dedup,
					fmt.Sprintf("Petugas %s mengajukan koreksi absensi untuk shift %s. Silakan tinjau pengajuan.", officerName, shiftDateStr),
					now, corrID, officerUserID, officerName, shiftDateStr)
			}
		}
		notifyRows.Close()
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCorrectionByID(ctx, rtID, corrID)
}

func (r *Repository) DecideCorrection(ctx context.Context, rtID, correctionID, approverID string, approve bool, note *string, now time.Time, ip string) (*AttendanceCorrection, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status, attID, officerID, reqBy, reqStatus, origStatus string
	var reqCin, reqCout, origCin, origCout sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT status, attendance_record_id, officer_user_id, requested_by,
		       requested_status, requested_checked_in_at, requested_checked_out_at,
		       original_status, original_checked_in_at, original_checked_out_at
		FROM security_attendance_corrections
		WHERE id = $1 AND rt_id = $2
		FOR UPDATE
	`, correctionID, rtID).Scan(&status, &attID, &officerID, &reqBy, &reqStatus, &reqCin, &reqCout, &origStatus, &origCin, &origCout)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "attendance correction not found", ErrNotFound)
		}
		return nil, err
	}
	if status != CorrectionPending {
		return nil, newAppError(409, "invalid_correction_state", "correction request has already been decided", ErrConflict)
	}
	// Strict maker-checker anti-self-approval guard.
	if approverID == reqBy || approverID == officerID {
		return nil, newAppError(403, "self_approval_forbidden", "cannot approve or decide your own correction request", ErrSelfApproval)
	}

	if !approve {
		_, err = tx.ExecContext(ctx, `
			UPDATE security_attendance_corrections
			SET status = 'rejected', decided_by = $2, decided_at = $3, decision_note = $4, updated_at = $3
			WHERE id = $1
		`, correctionID, approverID, now, note)
		if err != nil {
			return nil, err
		}
		if !origCin.Valid && origStatus == StatusBelumHadir {
			_, _ = tx.ExecContext(ctx, `
				UPDATE security_attendance_records
				SET status = 'tidak_hadir',
				    marked_absent_at = COALESCE(marked_absent_at, $2),
				    correction_locked = true,
				    updated_at = $2
				WHERE id = $1 AND checked_in_at IS NULL AND status = 'belum_hadir'
			`, attID, now)
		}
		_ = insertAuditLog(ctx, tx, rtID, &approverID, "security_correction_rejected", "security_attendance_correction", correctionID,
			map[string]any{"status": origStatus, "checked_in_at": origCin.Time},
			map[string]any{"decision": "rejected", "note": note}, ip)
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetCorrectionByID(ctx, rtID, correctionID)
	}

	// Load current attendance record and schedule to compute final values cleanly.
	var curStatus string
	var curCin, curCout sql.NullTime
	var schedStart time.Time
	var tolMin int
	err = tx.QueryRowContext(ctx, `
		SELECT ar.status, ar.checked_in_at, ar.checked_out_at,
		       COALESCE(s.scheduled_start_at, (ar.shift_date + s.start_time_snapshot) AT TIME ZONE 'Asia/Jakarta'),
		       s.lateness_tolerance_minutes
		FROM security_attendance_records ar
		JOIN security_schedules s ON s.id = ar.schedule_id
		WHERE ar.id = $1 AND ar.rt_id = $2
		FOR UPDATE OF ar
	`, attID, rtID).Scan(&curStatus, &curCin, &curCout, &schedStart, &tolMin)
	if err != nil {
		return nil, err
	}

	finalCin := curCin
	if reqCin.Valid {
		finalCin = reqCin
	}
	finalCout := curCout
	if reqCout.Valid {
		finalCout = reqCout
	} else if reqStatus == StatusHadir || reqStatus == StatusTerlambat {
		finalCout = sql.NullTime{Valid: false}
	}

	var arrivalStatus sql.NullString
	lateMin := 0
	if reqStatus == StatusHadir {
		arrivalStatus = sql.NullString{String: StatusHadir, Valid: true}
		if !finalCin.Valid {
			finalCin = sql.NullTime{Time: schedStart, Valid: true}
		}
	} else if reqStatus == StatusTerlambat {
		arrivalStatus = sql.NullString{String: StatusTerlambat, Valid: true}
		if !finalCin.Valid {
			finalCin = sql.NullTime{Time: schedStart.Add(time.Duration(tolMin+1) * time.Minute), Valid: true}
		}
		if finalCin.Valid && finalCin.Time.After(schedStart) {
			lateMin = int(finalCin.Time.Sub(schedStart) / time.Minute)
			if lateMin < 1 {
				lateMin = 1
			}
		}
	} else if reqStatus == StatusSudahAbsenPulang && finalCin.Valid {
		if finalCin.Time.After(schedStart.Add(time.Duration(tolMin) * time.Minute)) {
			arrivalStatus = sql.NullString{String: StatusTerlambat, Valid: true}
			lateMin = int(finalCin.Time.Sub(schedStart) / time.Minute)
			if lateMin < 1 {
				lateMin = 1
			}
		} else {
			arrivalStatus = sql.NullString{String: StatusHadir, Valid: true}
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_attendance_records
		SET status = $2,
		    arrival_status = COALESCE($3, arrival_status),
		    late_minutes = $4,
		    checked_in_at = $5,
		    checked_out_at = $6,
		    correction_locked = true,
		    updated_at = $7
		WHERE id = $1
	`, attID, reqStatus, arrivalStatus, lateMin, finalCin, finalCout, now)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_attendance_corrections
		SET status = 'approved', decided_by = $2, decided_at = $3, decision_note = $4,
		    final_status = $5, final_checked_in_at = $6, final_checked_out_at = $7, updated_at = $3
		WHERE id = $1
	`, correctionID, approverID, now, note, reqStatus, finalCin, finalCout)
	if err != nil {
		return nil, err
	}

	_ = insertAuditLog(ctx, tx, rtID, &approverID, "security_correction_approved", "security_attendance_correction", correctionID,
		map[string]any{"status": curStatus, "checked_in_at": nullableTimePtr(curCin), "checked_out_at": nullableTimePtr(curCout)},
		map[string]any{"status": reqStatus, "checked_in_at": nullableTimePtr(finalCin), "checked_out_at": nullableTimePtr(finalCout), "attendance_record_id": attID},
		ip,
	)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCorrectionByID(ctx, rtID, correctionID)
}

func nullableTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time
	return &t
}

func (r *Repository) GetCorrectionByID(ctx context.Context, rtID, correctionID string) (*AttendanceCorrection, error) {
	rows, err := r.listCorrectionsQuery(ctx, rtID, "", "", correctionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, newAppError(404, "not_found", "attendance correction not found", ErrNotFound)
	}
	return &rows[0], nil
}

func (r *Repository) ListCorrections(ctx context.Context, rtID, officerFilter, statusFilter string) ([]AttendanceCorrection, error) {
	return r.listCorrectionsQuery(ctx, rtID, officerFilter, statusFilter, "")
}

func (r *Repository) listCorrectionsQuery(ctx context.Context, rtID, officerFilter, statusFilter, idFilter string) ([]AttendanceCorrection, error) {
	where := []string{"c.rt_id = $1"}
	args := []any{rtID}
	idx := 2
	if idFilter != "" {
		where = append(where, fmt.Sprintf("c.id = $%d", idx))
		args = append(args, idFilter)
		idx++
	}
	if officerFilter != "" {
		where = append(where, fmt.Sprintf("c.officer_user_id = $%d", idx))
		args = append(args, officerFilter)
		idx++
	}
	if statusFilter != "" {
		where = append(where, fmt.Sprintf("c.status = $%d", idx))
		args = append(args, statusFilter)
		idx++
	}

	q := fmt.Sprintf(`
		SELECT c.id, c.rt_id, c.attendance_record_id, c.schedule_id,
		       c.officer_user_id, ou.full_name,
		       c.requested_by, ru.full_name,
		       COALESCE(to_char(ar.shift_date, 'YYYY-MM-DD'), to_char(s.shift_date, 'YYYY-MM-DD'), ''), s.shift_name_snapshot,
		       c.request_type, c.original_status, c.original_checked_in_at, c.original_checked_out_at,
		       c.requested_status, c.requested_checked_in_at, c.requested_checked_out_at,
		       c.reason, c.status, c.decided_by, du.full_name, c.decided_at, c.decision_note,
		       c.final_status, c.final_checked_in_at, c.final_checked_out_at,
		       c.created_at, c.updated_at
		FROM security_attendance_corrections c
		JOIN security_schedules s ON s.id = c.schedule_id
		LEFT JOIN security_attendance_records ar ON ar.id = c.attendance_record_id
		JOIN users ou ON ou.id = c.officer_user_id
		JOIN users ru ON ru.id = c.requested_by
		LEFT JOIN users du ON du.id = c.decided_by
		WHERE %s
		ORDER BY c.created_at DESC
	`, strings.Join(where, " AND "))

	rows, err := r.pool.Raw().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AttendanceCorrection
	for rows.Next() {
		var c AttendanceCorrection
		var oCin, oCout, rCin, rCout, dAt, fCin, fCout sql.NullTime
		var dBy, dName, dNote, fStatus sql.NullString
		if err := rows.Scan(
			&c.ID, &c.RTID, &c.AttendanceRecordID, &c.ScheduleID,
			&c.OfficerUserID, &c.OfficerName, &c.RequestedBy, &c.RequestedByName,
			&c.ShiftDate, &c.ShiftName,
			&c.RequestType, &c.OriginalStatus, &oCin, &oCout,
			&c.RequestedStatus, &rCin, &rCout,
			&c.Reason, &c.Status, &dBy, &dName, &dAt, &dNote,
			&fStatus, &fCin, &fCout, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c.OriginalCheckedInAt = nullableTimePtr(oCin)
		c.OriginalCheckedOutAt = nullableTimePtr(oCout)
		c.RequestedCheckedInAt = nullableTimePtr(rCin)
		c.RequestedCheckedOutAt = nullableTimePtr(rCout)
		c.DecidedAt = nullableTimePtr(dAt)
		c.FinalCheckedInAt = nullableTimePtr(fCin)
		c.FinalCheckedOutAt = nullableTimePtr(fCout)
		if dBy.Valid {
			v := dBy.String
			c.DecidedBy = &v
		}
		if dName.Valid {
			v := dName.String
			c.DecidedByName = &v
		}
		if dNote.Valid {
			v := dNote.String
			c.DecisionNote = &v
		}
		if fStatus.Valid {
			v := fStatus.String
			c.FinalStatus = &v
		}
		list = append(list, c)
	}
	if list == nil {
		list = []AttendanceCorrection{}
	}
	return list, rows.Err()
}

// ── Guest Book ────────────────────────────────────────────────────────────────

func (r *Repository) CreateGuestBook(ctx context.Context, rtID, deviceID string, req CreateGuestBookRequest, now time.Time) (*GuestBookEntry, error) {
	// If host_household_id provided, verify it belongs to the same RT.
	if req.HostHouseholdID != nil && strings.TrimSpace(*req.HostHouseholdID) != "" {
		var cnt int
		if err := r.pool.Raw().QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM household_occupancies ho
			JOIN households h ON h.id = ho.household_id
			WHERE ho.id = $1 AND h.rt_id = $2
		`, strings.TrimSpace(*req.HostHouseholdID), rtID).Scan(&cnt); err != nil {
			return nil, err
		}
		if cnt == 0 {
			return nil, newAppError(400, "invalid_household", "host household does not exist in this RT", ErrValidation)
		}
	}

	var id string
	err := r.pool.Raw().QueryRowContext(ctx, `
		INSERT INTO security_guest_books (
			rt_id, device_id, guest_name, phone, purpose, host_household_id,
			destination_label, vehicle_plate, notes, photo_key, checked_in_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11, $11)
		RETURNING id
	`, rtID, deviceID, strings.TrimSpace(req.GuestName), req.Phone, strings.TrimSpace(req.Purpose),
		req.HostHouseholdID, strings.TrimSpace(req.DestinationLabel), req.VehiclePlate, req.Notes, req.PhotoKey, now).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetGuestBookByID(ctx, rtID, id)
}

func (r *Repository) CheckoutGuestBook(ctx context.Context, rtID, deviceID, entryID string, now time.Time) (*GuestBookEntry, error) {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var checkedOut sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT checked_out_at FROM security_guest_books
		WHERE id = $1 AND rt_id = $2
		FOR UPDATE
	`, entryID, rtID).Scan(&checkedOut)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "guest book entry not found", ErrNotFound)
		}
		return nil, err
	}
	if checkedOut.Valid {
		return nil, newAppError(409, "already_checked_out", "guest has already checked out", ErrConflict)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_guest_books
		SET checked_out_at = $2, checkout_device_id = $3, updated_at = $2
		WHERE id = $1
	`, entryID, now, deviceID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetGuestBookByID(ctx, rtID, entryID)
}

func (r *Repository) GetGuestBookByID(ctx context.Context, rtID, entryID string) (*GuestBookEntry, error) {
	rows, _, err := r.ListGuestBooks(ctx, rtID, GuestBookFilter{ID: entryID, Page: 1, PerPage: 1})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, newAppError(404, "not_found", "guest book entry not found", ErrNotFound)
	}
	return &rows[0], nil
}

type GuestBookFilter struct {
	ID       string
	Status   string // "active", "checked_out", ""
	DateFrom string
	DateTo   string
	Search   string
	Page     int
	PerPage  int
}

func (r *Repository) ListGuestBooks(ctx context.Context, rtID string, f GuestBookFilter) ([]GuestBookEntry, int, error) {
	where := []string{"g.rt_id = $1"}
	args := []any{rtID}
	idx := 2

	if f.ID != "" {
		where = append(where, fmt.Sprintf("g.id = $%d", idx))
		args = append(args, f.ID)
		idx++
	}
	if f.Status == "active" {
		where = append(where, "g.checked_out_at IS NULL")
	} else if f.Status == "checked_out" {
		where = append(where, "g.checked_out_at IS NOT NULL")
	}
	if f.DateFrom != "" {
		where = append(where, fmt.Sprintf("(g.checked_in_at AT TIME ZONE 'Asia/Jakarta')::date >= $%d::date", idx))
		args = append(args, f.DateFrom)
		idx++
	}
	if f.DateTo != "" {
		where = append(where, fmt.Sprintf("(g.checked_in_at AT TIME ZONE 'Asia/Jakarta')::date <= $%d::date", idx))
		args = append(args, f.DateTo)
		idx++
	}
	if strings.TrimSpace(f.Search) != "" {
		where = append(where, fmt.Sprintf("(g.guest_name ILIKE $%d OR g.destination_label ILIKE $%d OR g.purpose ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+strings.TrimSpace(f.Search)+"%")
		idx++
	}

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM security_guest_books g WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.PerPage
	if limit <= 0 {
		limit = 20
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT g.id, g.rt_id, g.device_id, d.device_name, g.guest_name, g.phone,
		       g.purpose, g.host_household_id, g.destination_label, g.vehicle_plate, g.notes, g.photo_key,
		       g.checked_in_at, g.checked_out_at, g.checkout_device_id, g.created_at, g.updated_at
		FROM security_guest_books g
		JOIN security_devices d ON d.id = g.device_id
		WHERE %s
		ORDER BY g.checked_in_at ASC, g.id ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, idx, idx+1)

	rows, err := r.pool.Raw().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []GuestBookEntry
	for rows.Next() {
		var g GuestBookEntry
		var phone, hostID, plate, notes, photoKey, outDev sql.NullString
		var cout sql.NullTime
		if err := rows.Scan(
			&g.ID, &g.RTID, &g.DeviceID, &g.DeviceName, &g.GuestName, &phone,
			&g.Purpose, &hostID, &g.DestinationLabel, &plate, &notes, &photoKey,
			&g.CheckedInAt, &cout, &outDev, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if phone.Valid && strings.TrimSpace(phone.String) != "" {
			rawPhone := phone.String
			g.PhoneMasked = MaskPhone(&rawPhone)
			// Return masked phone in Phone as well to minimize PII exposure in lists/monitoring.
			g.Phone = g.PhoneMasked
		}
		if hostID.Valid {
			v := hostID.String
			g.HostHouseholdID = &v
		}
		if plate.Valid {
			v := plate.String
			g.VehiclePlate = &v
		}
 	if notes.Valid {
			v := notes.String
			g.Notes = &v
		}
		if photoKey.Valid {
			v := photoKey.String
			g.PhotoKey = &v
		}
		if outDev.Valid {
			v := outDev.String
			g.CheckoutDeviceID = &v
		}
		g.CheckedOutAt = nullableTimePtr(cout)
		list = append(list, g)
	}
	if list == nil {
		list = []GuestBookEntry{}
	}
	return list, total, rows.Err()
}

// ── Pos Device Read-Only Warga Helpers ────────────────────────────────────────

func (r *Repository) ListPosHouseholds(ctx context.Context, rtID, search string, page, perPage int) ([]PosHouseholdSummary, int, error) {
	where := []string{"ph.rt_id = $1", "ho.end_date IS NULL", "ph.is_active = true", "h.is_active = true"}
	args := []any{rtID}
	idx := 2
	if strings.TrimSpace(search) != "" {
		where = append(where, fmt.Sprintf(`(ph.house_number ILIKE $%d OR h.head_name ILIKE $%d OR EXISTS (
			SELECT 1
			FROM residency_periods rp
			JOIN residents res ON res.id = rp.resident_id
			WHERE rp.household_occupancy_id = ho.id AND rp.end_date IS NULL AND res.is_active = true AND res.full_name ILIKE $%d
		))`, idx, idx, idx))
		args = append(args, "%"+strings.TrimSpace(search)+"%")
		idx++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM household_occupancies ho
		JOIN physical_houses ph ON ph.id = ho.physical_house_id
		JOIN households h ON h.id = ho.household_id
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := perPage
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT ho.id, ph.house_number, ph.address, ho.occupancy_status, h.head_name,
		       (SELECT COUNT(*)
		        FROM residency_periods rp2
		        JOIN residents r2 ON r2.id = rp2.resident_id
		        WHERE rp2.household_occupancy_id = ho.id AND rp2.end_date IS NULL AND r2.is_active = true),
		       (SELECT r3.avatar_path
		        FROM residents r3
		        WHERE r3.rt_id = ph.rt_id AND r3.is_active = true AND r3.avatar_path IS NOT NULL
		          AND lower(trim(r3.full_name)) = lower(trim(h.head_name))
		        ORDER BY r3.updated_at DESC
		        LIMIT 1) AS head_avatar_path
		FROM household_occupancies ho
		JOIN physical_houses ph ON ph.id = ho.physical_house_id
		JOIN households h ON h.id = ho.household_id
		WHERE %s
		ORDER BY ph.house_number ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, idx, idx+1)

	rows, err := r.pool.Raw().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []PosHouseholdSummary
	for rows.Next() {
		var item PosHouseholdSummary
		var addr sql.NullString
		var avatar sql.NullString
		if err := rows.Scan(&item.HouseholdOccupancyID, &item.HouseNumber, &addr, &item.OccupancyType, &item.HeadOfFamilyName, &item.ResidentCount, &avatar); err != nil {
			return nil, 0, err
		}
		if addr.Valid {
			v := addr.String
			item.Block = &v
		}
		if avatar.Valid {
			v := avatar.String
			item.HeadAvatarPath = &v
		}
		list = append(list, item)
	}
	if list == nil {
		list = []PosHouseholdSummary{}
	}
	return list, total, rows.Err()
}

func (r *Repository) GetPosHouseholdDetail(ctx context.Context, rtID, occupancyID string) (*PosHouseholdDetail, error) {
	var d PosHouseholdDetail
	var addr sql.NullString
	var headAvatar sql.NullString
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT ho.id, ph.house_number, ph.address, ho.occupancy_status, h.head_name,
		       (SELECT COUNT(*)
		        FROM residency_periods rp2
		        JOIN residents r2 ON r2.id = rp2.resident_id
		        WHERE rp2.household_occupancy_id = ho.id AND rp2.end_date IS NULL AND r2.is_active = true),
		       (SELECT r3.avatar_path
		        FROM residents r3
		        WHERE r3.rt_id = ph.rt_id AND r3.is_active = true AND r3.avatar_path IS NOT NULL
		          AND lower(trim(r3.full_name)) = lower(trim(h.head_name))
		        ORDER BY r3.updated_at DESC
		        LIMIT 1) AS head_avatar_path
		FROM household_occupancies ho
		JOIN physical_houses ph ON ph.id = ho.physical_house_id
		JOIN households h ON h.id = ho.household_id
		WHERE ho.id = $1 AND ph.rt_id = $2 AND ho.end_date IS NULL AND ph.is_active = true AND h.is_active = true
	`, occupancyID, rtID).Scan(&d.HouseholdOccupancyID, &d.HouseNumber, &addr, &d.OccupancyType, &d.HeadOfFamilyName, &d.ResidentCount, &headAvatar)
	if headAvatar.Valid {
		v := headAvatar.String
		d.HeadAvatarPath = &v
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "household not found", ErrNotFound)
		}
		return nil, err
	}
	if addr.Valid {
		v := addr.String
		d.Block = &v
	}

	// Note: household_formation/008 dropped residents.relationship_to_head I
	// (30_remove_relationship_to_head.up.sql dropped the residency-period
	// column too), so the current domain model has no relationship linkage.
	// The household's head is expressed by households.head_name; we derive an
	// "is head of family" flag by exact (case-insensitive) name match.
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT r.id, r.full_name, r.avatar_path
		FROM residency_periods rp
		JOIN residents r ON r.id = rp.resident_id
		JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
		JOIN physical_houses ph ON ph.id = ho.physical_house_id
		WHERE rp.household_occupancy_id = $1 AND ph.rt_id = $2 AND rp.end_date IS NULL AND r.is_active = true
		ORDER BY r.full_name ASC
	`, occupancyID, rtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	d.Residents = []PosResidentSummary{}
	for rows.Next() {
		var rs PosResidentSummary
		var avatar sql.NullString
		if err := rows.Scan(&rs.ID, &rs.FullName, &avatar); err != nil {
			return nil, err
		}
		if avatar.Valid {
			v := avatar.String
			rs.AvatarPath = &v
		}
		rs.Relationship = "Anggota"
		rs.IsHeadOfFamily = strings.EqualFold(strings.TrimSpace(rs.FullName), strings.TrimSpace(d.HeadOfFamilyName))
		rs.ResidencyStatus = "tetap"
		d.Residents = append(d.Residents, rs)
	}
	return &d, rows.Err()
}

// ── Dashboard & Beranda Integration ───────────────────────────────────────────

func (r *Repository) GetDashboardSummary(ctx context.Context, rtID string, now time.Time) (*SecurityDashboardSummary, error) {
	todayStr := now.In(JakartaLoc).Format("2006-01-02")
	sum := &SecurityDashboardSummary{
		ServerTime: now.In(JakartaLoc),
		TodayDate:  todayStr,
	}

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE urm.rt_id = $1 AND urm.jabatan = 'keamanan' AND urm.is_active = true AND u.is_active = true
	`, rtID).Scan(&sum.TotalOfficers)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_shift_configs WHERE rt_id = $1 AND is_active = true
	`, rtID).Scan(&sum.ActiveShiftsCount)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_shift_swaps WHERE rt_id = $1 AND status = 'pending_ketua'
	`, rtID).Scan(&sum.PendingSwapsCount)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_attendance_corrections WHERE rt_id = $1 AND status = 'pending'
	`, rtID).Scan(&sum.PendingCorrections)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_devices WHERE rt_id = $1 AND is_active = true AND revoked_at IS NULL
	`, rtID).Scan(&sum.ActiveDevicesCount)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_guest_books WHERE rt_id = $1 AND checked_out_at IS NULL
	`, rtID).Scan(&sum.ActiveGuestsCount)

	todayAtt, _, err := r.ListAttendance(ctx, rtID, AttendanceFilter{
		DateFrom: todayStr,
		DateTo:   todayStr,
		Page:     1,
		PerPage:  100,
	})
	if err != nil {
		return nil, err
	}
	sum.TodayAttendance = todayAtt
	sum.ScheduledTodayCount = len(todayAtt)
	for _, rec := range todayAtt {
		switch rec.Status {
		case StatusHadir:
			sum.PresentTodayCount++
		case StatusTerlambat:
			sum.LateTodayCount++
		case StatusSudahAbsenPulang:
			sum.CheckedOutTodayCount++
		case StatusTidakHadir:
			sum.AbsentTodayCount++
		case StatusBelumAbsenPulang:
			sum.MissingCheckoutCount++
		}
	}
	return sum, nil
}

// GetOnDutyOfficers returns officers who are actually on duty now (checked in,
// not checked out, present attendance status) for Beranda integration.
// Officers only scheduled-but-not-checked-in or already checked out are excluded.
func (r *Repository) GetOnDutyOfficers(ctx context.Context, rtID string, now time.Time) ([]OnDutyOfficerSummary, error) {
	// now is the authoritative instant used to decide whether a schedule is
	// currently in progress (Asia/Jakarta wall-clock semantics come from the
	// stored TIMESTAMPTZ instants themselves). The half-open interval keeps
	// cross-midnight shifts working: at 02:00 the schedule that started
	// "yesterday" 20:00 and ends "today" 08:00 still contains now.
	rows, err := r.pool.Raw().QueryContext(ctx, `
		WITH duty AS (
			SELECT s.user_id, u.full_name, s.shift_name_snapshot,
			       to_char(s.start_time_snapshot, 'HH24:MI') AS shift_start,
			       to_char(s.end_time_snapshot, 'HH24:MI') AS shift_end,
			       COALESCE(ar.status, 'belum_hadir') AS status,
			       COALESCE(u.phone, (SELECT r.phone FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1)) AS phone,
			       (SELECT r.avatar_path FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1) AS avatar_path,
			       (SELECT r.avatar_thumb_path FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1) AS avatar_thumb_path,
			       ar.checked_in_at, s.scheduled_start_at,
			       ROW_NUMBER() OVER (PARTITION BY s.user_id ORDER BY ar.checked_in_at ASC NULLS LAST, s.scheduled_start_at ASC) as rn
			FROM security_schedules s
			JOIN users u ON u.id = s.user_id
			JOIN security_attendance_records ar ON ar.schedule_id = s.id
			JOIN user_rt_memberships um ON (um.id = s.membership_id OR (um.user_id = s.user_id AND um.rt_id = s.rt_id)) AND um.is_active = true
			WHERE s.rt_id = $1
			  AND s.status = 'scheduled'
			  AND s.is_active = true
			  AND (
			        (s.scheduled_start_at IS NOT NULL AND (
			            (s.scheduled_start_at <= $2 AND s.scheduled_end_at > $2)
			            OR (
			                 ar.checked_in_at IS NOT NULL
			                 AND ar.checked_in_at <= $2
			                 AND s.scheduled_end_at > $2
			                 AND $2 >= (s.scheduled_start_at - (s.check_in_window_before_minutes || ' minutes')::interval)
			               )
			        ))
			        OR
			        (s.scheduled_start_at IS NULL AND (
			            ar.checked_in_at IS NOT NULL
			            AND ar.checked_in_at <= $2
			            AND $2 < CASE
			                WHEN s.crosses_midnight_snapshot OR s.end_time_snapshot <= s.start_time_snapshot
			                THEN (ar.shift_date + interval '1 day' + s.end_time_snapshot + (COALESCE(s.missing_checkout_cutoff_minutes, 60) || ' minutes')::interval) AT TIME ZONE 'Asia/Jakarta'
			                ELSE (ar.shift_date + s.end_time_snapshot + (COALESCE(s.missing_checkout_cutoff_minutes, 60) || ' minutes')::interval) AT TIME ZONE 'Asia/Jakarta'
			            END
			        ))
			      )
			  AND u.is_active = true
			  AND (ar.checked_in_at IS NOT NULL OR LOWER(ar.status) IN ('hadir', 'terlambat'))
			  AND ar.checked_out_at IS NULL
			  AND LOWER(ar.status) IN ('hadir', 'terlambat')
			  AND ar.marked_missing_checkout_at IS NULL
		)
		SELECT user_id, full_name, shift_name_snapshot, shift_start, shift_end, status, phone, avatar_path, avatar_thumb_path
		FROM duty
		WHERE rn = 1
		ORDER BY checked_in_at ASC NULLS LAST, scheduled_start_at ASC, full_name ASC
	`, rtID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []OnDutyOfficerSummary
	for rows.Next() {
		var item OnDutyOfficerSummary
		var phone sql.NullString
		var avatarPath sql.NullString
		var avatarThumbPath sql.NullString
		if err := rows.Scan(&item.UserID, &item.FullName, &item.ShiftName, &item.ShiftStart, &item.ShiftEnd, &item.Status, &phone, &avatarPath, &avatarThumbPath); err != nil {
			return nil, err
		}
		if avatarPath.Valid && strings.TrimSpace(avatarPath.String) != "" {
			v := avatarPath.String
			item.AvatarPath = &v
		}
		if avatarThumbPath.Valid && strings.TrimSpace(avatarThumbPath.String) != "" {
			v := avatarThumbPath.String
			item.AvatarThumbPath = &v
		}
		if phone.Valid && strings.TrimSpace(phone.String) != "" {
			v := phone.String
			item.PhoneNumber = &v
		}
		list = append(list, item)
	}
	if list == nil {
		list = []OnDutyOfficerSummary{}
	}
	return list, rows.Err()
}

// ── Idempotent Background Scheduler ───────────────────────────────────────────

type SchedulerTickResult struct {
	AcquiredLock          bool
	MarkedAbsentCount     int
	MarkedMissingCheckout int
}

func (r *Repository) RunSchedulerTick(ctx context.Context, now time.Time) (*SchedulerTickResult, error) {
	conn, err := r.pool.Raw().Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, SchedulerAdvisoryLockID).Scan(&acquired); err != nil {
		return nil, err
	}
	if !acquired {
		return &SchedulerTickResult{AcquiredLock: false}, nil
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, SchedulerAdvisoryLockID)
	}()

	res := &SchedulerTickResult{AcquiredLock: true}

	// Ensure any active legacy schedule with explicit shift_date without an attendance row yet gets initialized cleanly.
	// For date-independent shifts (shift_date IS NULL), attendance rows are NEVER pre-inserted by scheduler.
	_, err = conn.ExecContext(ctx, `
		INSERT INTO security_attendance_records (
			rt_id, schedule_id, user_id, shift_config_id, shift_version, shift_date, status, created_at, updated_at
		)
		SELECT s.rt_id, s.id, s.user_id, s.shift_config_id, s.shift_version, s.shift_date, 'belum_hadir', $1, $1
		FROM security_schedules s
		LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
		WHERE s.status = 'scheduled' AND ar.id IS NULL AND s.shift_date IS NOT NULL
		ON CONFLICT (schedule_id, shift_date) DO NOTHING
	`, now)
	if err != nil {
		return nil, err
	}

	// 1. Mark Tidak Hadir once absent cutoff has elapsed with no check-in (only for legacy date-specified schedules).
	absRes, err := conn.ExecContext(ctx, `
		UPDATE security_attendance_records ar
		SET status = 'tidak_hadir',
		    marked_absent_at = COALESCE(ar.marked_absent_at, $1),
		    updated_at = $1
		FROM security_schedules s
		WHERE ar.schedule_id = s.id
		  AND s.status = 'scheduled'
		  AND ar.correction_locked = false
		  AND ar.status = 'belum_hadir'
		  AND ar.checked_in_at IS NULL
		  AND s.shift_date IS NOT NULL
		  AND s.scheduled_start_at IS NOT NULL
		  AND $1 > (s.scheduled_start_at + (s.absent_cutoff_minutes * INTERVAL '1 minute'))
	`, now)
	if err != nil {
		return nil, err
	}
	if n, err := absRes.RowsAffected(); err == nil {
		res.MarkedAbsentCount = int(n)
	}

	// 2. Mark Belum Absen Pulang once shift end + missing_checkout_cutoff has elapsed with no check-out.
	// Supports both legacy scheduled_end_at and date-independent dynamic wall-clock end.
	missRes, err := conn.ExecContext(ctx, `
		UPDATE security_attendance_records ar
		SET status = 'belum_absen_pulang',
		    marked_missing_checkout_at = COALESCE(ar.marked_missing_checkout_at, $1),
		    updated_at = $1
		FROM security_schedules s
		WHERE ar.schedule_id = s.id
		  AND s.status = 'scheduled'
		  AND ar.correction_locked = false
		  AND ar.status IN ('hadir', 'terlambat')
		  AND ar.checked_out_at IS NULL
		  AND $1 > CASE
		        WHEN s.scheduled_end_at IS NOT NULL THEN (s.scheduled_end_at + (s.missing_checkout_cutoff_minutes * INTERVAL '1 minute'))
		        ELSE (
		            CASE
		                WHEN s.crosses_midnight_snapshot OR s.end_time_snapshot <= s.start_time_snapshot
		                THEN (ar.shift_date + interval '1 day' + s.end_time_snapshot) AT TIME ZONE 'Asia/Jakarta'
		                ELSE (ar.shift_date + s.end_time_snapshot) AT TIME ZONE 'Asia/Jakarta'
		            END + (s.missing_checkout_cutoff_minutes * INTERVAL '1 minute')
		        )
		  END
	`, now)
	if err != nil {
		return nil, err
	}
	if n, err := missRes.RowsAffected(); err == nil {
		res.MarkedMissingCheckout = int(n)
	}

	return res, nil
}

// ── Patrol & Paket Repository Methods ───────────────────────────────────────

func (r *Repository) ListPatrolTemplates(ctx context.Context, rtID string) ([]PatrolTemplate, error) {
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT id, rt_id, name, location_label, is_active, created_by, created_at, updated_at
		FROM patrol_templates
		WHERE rt_id = $1 AND is_active = true
		ORDER BY name ASC
	`, rtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []PatrolTemplate
	for rows.Next() {
		var t PatrolTemplate
		var createdBy sql.NullString
		if err := rows.Scan(&t.ID, &t.RTID, &t.Name, &t.LocationLabel, &t.IsActive, &createdBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if createdBy.Valid {
			t.CreatedBy = &createdBy.String
		}
		templates = append(templates, t)
	}
	if templates == nil {
		templates = []PatrolTemplate{}
	}
	return templates, nil
}

func (r *Repository) CreatePatrolTemplate(ctx context.Context, rtID, name, locationLabel, createdBy string) (*PatrolTemplate, error) {
	var id string
	now := time.Now()
	err := r.pool.Raw().QueryRowContext(ctx, `
		INSERT INTO patrol_templates (rt_id, name, location_label, is_active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, true, $4, $5, $5)
		RETURNING id
	`, rtID, strings.TrimSpace(name), strings.TrimSpace(locationLabel), createdBy, now).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &PatrolTemplate{
		ID:            id,
		RTID:          rtID,
		Name:          strings.TrimSpace(name),
		LocationLabel: strings.TrimSpace(locationLabel),
		IsActive:      true,
		CreatedBy:     &createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (r *Repository) UpdatePatrolTemplate(ctx context.Context, rtID, templateID, name, locationLabel string, isActive *bool) (*PatrolTemplate, error) {
	var current PatrolTemplate
	var createdBy sql.NullString
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT id, rt_id, name, location_label, is_active, created_by, created_at, updated_at
		FROM patrol_templates
		WHERE id = $1 AND rt_id = $2
	`, templateID, rtID).Scan(&current.ID, &current.RTID, &current.Name, &current.LocationLabel, &current.IsActive, &createdBy, &current.CreatedAt, &current.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "template gang patroli tidak ditemukan", ErrNotFound)
		}
		return nil, err
	}
	if createdBy.Valid {
		current.CreatedBy = &createdBy.String
	}

	name = strings.TrimSpace(name)
	if name != "" {
		current.Name = name
	}
	locationLabel = strings.TrimSpace(locationLabel)
	if locationLabel != "" {
		current.LocationLabel = locationLabel
	}
	if isActive != nil {
		current.IsActive = *isActive
	}
	now := time.Now()
	_, err = r.pool.Raw().ExecContext(ctx, `
		UPDATE patrol_templates
		SET name = $1, location_label = $2, is_active = $3, updated_at = $4
		WHERE id = $5 AND rt_id = $6
	`, current.Name, current.LocationLabel, current.IsActive, now, templateID, rtID)
	if err != nil {
		return nil, err
	}
	current.UpdatedAt = now
	return &current, nil
}

func (r *Repository) DeletePatrolTemplate(ctx context.Context, rtID, templateID string) error {
	var exists bool
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM patrol_templates WHERE id = $1 AND rt_id = $2)
	`, templateID, rtID).Scan(&exists)
	if err != nil || !exists {
		return newAppError(404, "not_found", "template gang patroli tidak ditemukan", ErrNotFound)
	}

	var count int
	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)::int FROM patrol_photos WHERE patrol_template_id = $1
	`, templateID).Scan(&count)

	if count > 0 {
		_, err = r.pool.Raw().ExecContext(ctx, `
			UPDATE patrol_templates SET is_active = false, updated_at = NOW() WHERE id = $1 AND rt_id = $2
		`, templateID, rtID)
		return err
	}

	_, err = r.pool.Raw().ExecContext(ctx, `
		DELETE FROM patrol_templates WHERE id = $1 AND rt_id = $2
	`, templateID, rtID)
	return err
}

func (r *Repository) CreatePatrolBundle(ctx context.Context, rtID, createdBy string, notes *string, photos []CreatePatrolPhoto) (*PatrolBundle, error) {
	if len(photos) == 0 {
		return nil, newAppError(400, "empty_photos", "minimal satu foto patroli harus disertakan", ErrValidation)
	}

	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var bundleID string
	now := time.Now()
	err = tx.QueryRowContext(ctx, `
		INSERT INTO patrol_bundles (rt_id, created_by, submitted_at, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $3, $3)
		RETURNING id
	`, rtID, createdBy, now, notes).Scan(&bundleID)
	if err != nil {
		return nil, err
	}

	for _, p := range photos {
		if strings.HasPrefix(p.ImageKey, "community/") && !strings.HasPrefix(p.ImageKey, "community/"+rtID+"/") {
			return nil, newAppError(403, "forbidden", "foto milik RT lain tidak dapat digunakan", ErrForbidden)
		}

		var photoID string
		captured := p.CapturedAt
		if captured.IsZero() {
			captured = now
		}
		var tName, tLoc sql.NullString
		err = tx.QueryRowContext(ctx, `
			SELECT name, location_label FROM patrol_templates WHERE id = $1 AND rt_id = $2
		`, p.PatrolTemplateID, rtID).Scan(&tName, &tLoc)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, newAppError(400, "invalid_template", "template patroli tidak valid atau milik RT lain", ErrValidation)
			}
			return nil, err
		}

		err = tx.QueryRowContext(ctx, `
			INSERT INTO patrol_photos (patrol_bundle_id, patrol_template_id, template_name, location_label, image_key, captured_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`, bundleID, p.PatrolTemplateID, tName.String, tLoc.String, p.ImageKey, captured, now).Scan(&photoID)
		if err != nil {
			return nil, err
		}
	}

	// Insert notifications for Ketua RT only
	var officerIDs []string
	oRows, err := tx.QueryContext(ctx, `
		SELECT u.id FROM users u
		JOIN user_rt_memberships urm ON urm.user_id = u.id
		WHERE urm.rt_id = $1 AND urm.is_active = true AND urm.jabatan = 'ketua'
	`, rtID)
	if err == nil {
		for oRows.Next() {
			var oid string
			if err := oRows.Scan(&oid); err == nil {
				officerIDs = append(officerIDs, oid)
			}
		}
		oRows.Close()
		for _, oid := range officerIDs {
			dedupKey := fmt.Sprintf("patrol:%s:%s", bundleID, oid)
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at, payload)
				VALUES ($1, $2, 'patrol_bundle_submitted', $3, 'Laporan Patroli Baru', 'Petugas telah menyelesaikan dan mengirim laporan patroli wilayah.', 'SENT', $4, json_build_object('bundle_id', $5::text))
				ON CONFLICT DO NOTHING
			`, rtID, oid, dedupKey, now, bundleID)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetPatrolBundle(ctx, rtID, bundleID)
}

func (r *Repository) ListPatrolBundles(ctx context.Context, rtID string, limit, offset int) ([]PatrolBundle, int, error) {
	var total int
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM patrol_bundles WHERE rt_id = $1
	`, rtID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT pb.id, pb.rt_id, pb.created_by, COALESCE(u.full_name, ''), pb.submitted_at, pb.notes,
		       COUNT(pp.id)::int AS photo_count, pb.created_at, pb.updated_at
		FROM patrol_bundles pb
		LEFT JOIN users u ON u.id = pb.created_by
		LEFT JOIN patrol_photos pp ON pp.patrol_bundle_id = pb.id
		WHERE pb.rt_id = $1
		GROUP BY pb.id, u.full_name
		ORDER BY pb.submitted_at DESC
		LIMIT $2 OFFSET $3
	`, rtID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bundles []PatrolBundle
	bundleIndexMap := make(map[string]int)
	for rows.Next() {
		var b PatrolBundle
		var notes sql.NullString
		if err := rows.Scan(&b.ID, &b.RTID, &b.CreatedBy, &b.CreatedByName, &b.SubmittedAt, &notes, &b.PhotoCount, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if notes.Valid {
			b.Notes = &notes.String
		}
		b.Photos = []PatrolPhoto{}
		bundleIndexMap[b.ID] = len(bundles)
		bundles = append(bundles, b)
	}

	if len(bundles) > 0 {
		photoRows, pErr := r.pool.Raw().QueryContext(ctx, `
			SELECT pp.id, pp.patrol_bundle_id, COALESCE(pp.patrol_template_id::text, ''),
			       COALESCE(NULLIF(pp.template_name, ''), pt.name, ''),
			       COALESCE(NULLIF(pp.location_label, ''), pt.location_label, ''),
			       pp.image_key, pp.captured_at, pp.created_at
			FROM patrol_photos pp
			LEFT JOIN patrol_templates pt ON pt.id = pp.patrol_template_id
			WHERE pp.patrol_bundle_id IN (
				SELECT pb2.id FROM patrol_bundles pb2
				WHERE pb2.rt_id = $1
				ORDER BY pb2.submitted_at DESC
				LIMIT $2 OFFSET $3
			)
			ORDER BY pp.created_at ASC
		`, rtID, limit, offset)
		if pErr == nil {
			defer photoRows.Close()
			for photoRows.Next() {
				var p PatrolPhoto
				if err := photoRows.Scan(&p.ID, &p.PatrolBundleID, &p.PatrolTemplateID, &p.TemplateName, &p.LocationLabel, &p.ImageKey, &p.CapturedAt, &p.CreatedAt); err == nil {
					p.ImageURL = "/api/v1/security/patrol/photos/" + p.ID
					if idx, ok := bundleIndexMap[p.PatrolBundleID]; ok {
						bundles[idx].Photos = append(bundles[idx].Photos, p)
					}
				}
			}
		}
	}

	if bundles == nil {
		bundles = []PatrolBundle{}
	}
	return bundles, total, nil
}

func (r *Repository) GetPatrolBundle(ctx context.Context, rtID, bundleID string) (*PatrolBundle, error) {
	var b PatrolBundle
	var notes sql.NullString
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT pb.id, pb.rt_id, pb.created_by, COALESCE(u.full_name, ''), pb.submitted_at, pb.notes, pb.created_at, pb.updated_at
		FROM patrol_bundles pb
		LEFT JOIN users u ON u.id = pb.created_by
		WHERE pb.id = $1 AND pb.rt_id = $2
	`, bundleID, rtID).Scan(&b.ID, &b.RTID, &b.CreatedBy, &b.CreatedByName, &b.SubmittedAt, &notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "patrol bundle tidak ditemukan", ErrNotFound)
		}
		return nil, err
	}
	if notes.Valid {
		b.Notes = &notes.String
	}

	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT pp.id, pp.patrol_bundle_id, COALESCE(pp.patrol_template_id::text, ''),
		       COALESCE(NULLIF(pp.template_name, ''), pt.name, ''),
		       COALESCE(NULLIF(pp.location_label, ''), pt.location_label, ''),
		       pp.image_key, pp.captured_at, pp.created_at
		FROM patrol_photos pp
		LEFT JOIN patrol_templates pt ON pt.id = pp.patrol_template_id
		WHERE pp.patrol_bundle_id = $1
		ORDER BY pp.created_at ASC
	`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p PatrolPhoto
		if err := rows.Scan(&p.ID, &p.PatrolBundleID, &p.PatrolTemplateID, &p.TemplateName, &p.LocationLabel, &p.ImageKey, &p.CapturedAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.ImageURL = "/api/v1/security/patrol/photos/" + p.ID
		b.Photos = append(b.Photos, p)
	}
	b.PhotoCount = len(b.Photos)
	return &b, nil
}

func (r *Repository) CreatePaketReport(ctx context.Context, rtID, createdBy, receiverWargaID string, items []CreatePaketItem) (*PaketReport, error) {
	if len(items) == 0 {
		return nil, newAppError(400, "empty_items", "minimal satu item paket harus disertakan", ErrValidation)
	}

	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var residentExists bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM residents WHERE id = $1 AND rt_id = $2)
	`, receiverWargaID, rtID).Scan(&residentExists)
	if err != nil || !residentExists {
		return nil, newAppError(400, "invalid_receiver", "penerima warga tidak valid atau bukan warga RT ini", ErrValidation)
	}

	var reportID string
	now := time.Now()
	err = tx.QueryRowContext(ctx, `
		INSERT INTO paket_reports (rt_id, receiver_warga_id, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id
	`, rtID, receiverWargaID, createdBy, now).Scan(&reportID)
	if err != nil {
		return nil, err
	}

	for _, it := range items {
		var itemID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO paket_items (paket_report_id, description, status, created_at, updated_at)
			VALUES ($1, $2, 'dititipkan', $3, $3)
			RETURNING id
		`, reportID, it.Description, now).Scan(&itemID)
		if err != nil {
			return nil, err
		}

		for _, k := range it.ImageKeys {
			if strings.HasPrefix(k, "community/") && !strings.HasPrefix(k, "community/"+rtID+"/") {
				return nil, newAppError(403, "forbidden", "foto milik RT lain tidak dapat digunakan", ErrForbidden)
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO paket_photos (paket_item_id, paket_report_id, image_key, photo_type, captured_at, created_at)
				VALUES ($1, $2, $3, 'paket', $4, $4)
			`, itemID, reportID, k, now)
			if err != nil {
				return nil, err
			}
		}
	}

	// Notify receiver warga if linked to user account
	var receiverUserID sql.NullString
	_ = tx.QueryRowContext(ctx, `
		SELECT user_id FROM residents WHERE id = $1 AND rt_id = $2
	`, receiverWargaID, rtID).Scan(&receiverUserID)
	if receiverUserID.Valid && receiverUserID.String != "" {
		dedup := fmt.Sprintf("paket_recv:%s:%s", reportID, receiverUserID.String)
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at, payload)
			VALUES ($1, $2, 'paket_received', $3, 'Paket Baru Dititipkan', 'Ada paket baru yang dititipkan di pos keamanan untuk Anda.', 'SENT', $4, json_build_object('report_id', $5::text, 'paket_id', $5::text, 'package_id', $5::text))
			ON CONFLICT DO NOTHING
		`, rtID, receiverUserID.String, dedup, now, reportID)
	}

	// Notify Ketua RT
	var ketuaIDs []string
	kRows, err := tx.QueryContext(ctx, `
		SELECT u.id FROM users u
		JOIN user_rt_memberships urm ON urm.user_id = u.id
		WHERE urm.rt_id = $1 AND urm.is_active = true AND urm.jabatan = 'ketua'
	`, rtID)
	if err == nil {
		for kRows.Next() {
			var kid string
			if err := kRows.Scan(&kid); err == nil {
				ketuaIDs = append(ketuaIDs, kid)
			}
		}
		kRows.Close()
		for _, kid := range ketuaIDs {
			dedup := fmt.Sprintf("paket_ketua:%s:%s", reportID, kid)
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at, payload)
				VALUES ($1, $2, 'paket_received', $3, 'Laporan Paket Masuk', 'Ada paket titipan baru di pos keamanan.', 'SENT', $4, json_build_object('report_id', $5::text, 'paket_id', $5::text, 'package_id', $5::text))
				ON CONFLICT DO NOTHING
			`, rtID, kid, dedup, now, reportID)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetPaketReport(ctx, rtID, reportID)
}

func (r *Repository) ListPaketReports(ctx context.Context, rtID string, limit, offset int) ([]PaketReport, int, error) {
	var total int
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM paket_reports WHERE rt_id = $1
	`, rtID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT pr.id, pr.rt_id, pr.receiver_warga_id, COALESCE(res.full_name, ''), COALESCE(h.house_number, ''),
		       pr.created_by, COALESCE(u.full_name, ''), pr.created_at, pr.updated_at
		FROM paket_reports pr
		LEFT JOIN residents res ON res.id = pr.receiver_warga_id
		LEFT JOIN LATERAL (
			SELECT ph.house_number
			FROM residency_periods rp
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
			JOIN physical_houses ph ON ph.id = ho.physical_house_id
			WHERE rp.resident_id = res.id AND rp.end_date IS NULL
			LIMIT 1
		) h ON true
		LEFT JOIN users u ON u.id = pr.created_by
		WHERE pr.rt_id = $1
		ORDER BY pr.created_at DESC
		LIMIT $2 OFFSET $3
	`, rtID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []PaketReport
	for rows.Next() {
		var rep PaketReport
		if err := rows.Scan(&rep.ID, &rep.RTID, &rep.ReceiverWargaID, &rep.ReceiverName, &rep.ReceiverHouseNumber,
			&rep.CreatedBy, &rep.CreatedByName, &rep.CreatedAt, &rep.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items, err := r.getPaketItemsForReport(ctx, rep.ID)
		if err == nil {
			rep.Items = items
		}
		enrichPaketReportAudit(&rep)
		reports = append(reports, rep)
	}
	if reports == nil {
		reports = []PaketReport{}
	}
	return reports, total, nil
}

func enrichPaketReportAudit(rep *PaketReport) {
	rep.OverallStatus = computeOverallPaketStatus(rep.Items)
	rep.TotalItems = len(rep.Items)
	handedOverCount := 0
	pendingCount := 0
	officerMap := make(map[string]string) // userID -> name
	for _, it := range rep.Items {
		if it.Status == "sudah_diambil" || it.HandedOverAt != nil {
			handedOverCount++
			if it.HandedOverBy != nil && *it.HandedOverBy != "" {
				officerMap[*it.HandedOverBy] = it.HandedOverByName
			}
			if rep.HandedOverAt == nil || (it.HandedOverAt != nil && it.HandedOverAt.After(*rep.HandedOverAt)) {
				rep.HandedOverAt = it.HandedOverAt
			}
		} else {
			pendingCount++
		}
	}
	rep.HandedOverItemsCount = handedOverCount
	rep.PendingItemsCount = pendingCount

	if handedOverCount == len(rep.Items) && len(rep.Items) > 0 {
		if len(officerMap) == 1 {
			for uid, name := range officerMap {
				uidCopy := uid
				rep.HandedOverBy = &uidCopy
				rep.HandedOverByName = name
			}
		} else if len(officerMap) > 1 {
			rep.HandedOverBy = nil
			rep.HandedOverByName = "Beberapa Petugas"
		} else {
			rep.HandedOverBy = nil
			rep.HandedOverByName = ""
		}
	} else if handedOverCount > 0 {
		rep.HandedOverBy = nil
		rep.HandedOverByName = "Sebagian Diserahkan"
	} else {
		rep.HandedOverBy = nil
		rep.HandedOverByName = ""
		rep.HandedOverAt = nil
	}
}

func (r *Repository) GetPaketReport(ctx context.Context, rtID, reportID string) (*PaketReport, error) {
	var rep PaketReport
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT pr.id, pr.rt_id, pr.receiver_warga_id, COALESCE(res.full_name, ''), COALESCE(h.house_number, ''),
		       pr.created_by, COALESCE(u.full_name, ''), pr.created_at, pr.updated_at
		FROM paket_reports pr
		LEFT JOIN residents res ON res.id = pr.receiver_warga_id
		LEFT JOIN LATERAL (
			SELECT ph.house_number
			FROM residency_periods rp
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
			JOIN physical_houses ph ON ph.id = ho.physical_house_id
			WHERE rp.resident_id = res.id AND rp.end_date IS NULL
			LIMIT 1
		) h ON true
		LEFT JOIN users u ON u.id = pr.created_by
		WHERE (pr.id = $1 OR pr.id IN (SELECT pi.paket_report_id FROM paket_items pi WHERE pi.id = $1))
		  AND pr.rt_id = $2
		LIMIT 1
	`, reportID, rtID).Scan(&rep.ID, &rep.RTID, &rep.ReceiverWargaID, &rep.ReceiverName, &rep.ReceiverHouseNumber,
		&rep.CreatedBy, &rep.CreatedByName, &rep.CreatedAt, &rep.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "laporan paket tidak ditemukan", ErrNotFound)
		}
		return nil, err
	}

	items, err := r.getPaketItemsForReport(ctx, rep.ID)
	if err != nil {
		return nil, err
	}
	rep.Items = items
	enrichPaketReportAudit(&rep)
	return &rep, nil
}

func (r *Repository) getPaketItemsForReport(ctx context.Context, reportID string) ([]PaketItem, error) {
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT pi.id, pi.paket_report_id, pi.description, pi.status, pi.status_changed_at, pi.status_changed_by,
		       COALESCE(u.full_name, ''), pi.created_at, pi.updated_at,
		       pi.handed_over_at, pi.handed_over_by_user_id, COALESCE(hu.full_name, '')
		FROM paket_items pi
		LEFT JOIN users u ON u.id = pi.status_changed_by
		LEFT JOIN users hu ON hu.id = pi.handed_over_by_user_id
		WHERE pi.paket_report_id = $1
		ORDER BY pi.created_at ASC, pi.id ASC
	`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PaketItem
	for rows.Next() {
		var it PaketItem
		var desc, changedBy, changedByName, handedOverBy, handedOverByName sql.NullString
		var changedAt, handedOverAt sql.NullTime
		if err := rows.Scan(
			&it.ID, &it.PaketReportID, &desc, &it.Status, &changedAt, &changedBy, &changedByName, &it.CreatedAt, &it.UpdatedAt,
			&handedOverAt, &handedOverBy, &handedOverByName,
		); err != nil {
			return nil, err
		}
		if desc.Valid {
			it.Description = &desc.String
		}
		if changedAt.Valid {
			it.StatusChangedAt = &changedAt.Time
		}
		if changedBy.Valid {
			it.StatusChangedBy = &changedBy.String
		}
		if changedByName.Valid {
			it.ChangedByName = changedByName.String
		}
		if handedOverAt.Valid {
			it.HandedOverAt = &handedOverAt.Time
		}
		if handedOverBy.Valid {
			it.HandedOverBy = &handedOverBy.String
		}
		if handedOverByName.Valid {
			it.HandedOverByName = handedOverByName.String
		}

		pRows, err := r.pool.Raw().QueryContext(ctx, `
			SELECT id, paket_item_id, paket_report_id, image_key, photo_type, captured_at, created_at
			FROM paket_photos
			WHERE paket_item_id = $1
			ORDER BY created_at ASC
		`, it.ID)
		if err == nil {
			for pRows.Next() {
				var p PaketPhoto
				if err := pRows.Scan(&p.ID, &p.PaketItemID, &p.PaketReportID, &p.ImageKey, &p.PhotoType, &p.CapturedAt, &p.CreatedAt); err == nil {
					p.ImageURL = "/api/v1/security/paket/photos/" + p.ID
					it.Photos = append(it.Photos, p)
				}
			}
			pRows.Close()
		}
		items = append(items, it)
	}
	if items == nil {
		items = []PaketItem{}
	}
	return items, nil
}

func computeOverallPaketStatus(items []PaketItem) string {
	if len(items) == 0 {
		return "dititipkan"
	}
	hasPending := false
	hasDelivered := false
	for _, it := range items {
		if it.Status == "dititipkan" {
			hasPending = true
		} else if it.Status == "sudah_diambil" {
			hasDelivered = true
		}
	}
	if hasPending {
		return "dititipkan"
	}
	if hasDelivered {
		return "sudah_diambil"
	}
	return "dibatalkan"
}

func (r *Repository) UpdatePaketItemStatus(ctx context.Context, rtID, reportID, itemID, newStatus, userID string, proofImageKey *string) error {
	if newStatus != "sudah_diambil" && newStatus != "dibatalkan" && newStatus != "dititipkan" {
		return newAppError(400, "invalid_status", "status tidak valid", ErrValidation)
	}

	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var currentStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT pi.status
		FROM paket_items pi
		JOIN paket_reports pr ON pr.id = pi.paket_report_id
		WHERE pi.id = $1 AND pr.rt_id = $2 AND pr.id = $3
		FOR UPDATE
	`, itemID, rtID, reportID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newAppError(404, "not_found", "item paket tidak ditemukan pada laporan ini", ErrNotFound)
		}
		return err
	}

	if currentStatus == "sudah_diambil" && newStatus == "sudah_diambil" {
		// Idempotent: already marked as picked up
		return nil
	}
	if currentStatus == "dibatalkan" {
		return newAppError(400, "invalid_transition", "paket yang sudah dibatalkan tidak dapat diubah statusnya", ErrValidation)
	}
	if currentStatus == "sudah_diambil" && newStatus != "sudah_diambil" {
		return newAppError(400, "invalid_transition", "paket yang sudah diambil tidak dapat dikembalikan ke status sebelumnya", ErrValidation)
	}

	if newStatus == "sudah_diambil" && (proofImageKey == nil || *proofImageKey == "") {
		return newAppError(400, "proof_required", "foto bukti pengambilan wajib disertakan saat pengambilan paket", ErrValidation)
	}

	if proofImageKey != nil && *proofImageKey != "" {
		if strings.HasPrefix(*proofImageKey, "community/") && !strings.HasPrefix(*proofImageKey, "community/"+rtID+"/") {
			return newAppError(403, "forbidden", "foto bukti milik RT lain tidak dapat digunakan", ErrForbidden)
		}
	}

	now := time.Now()
	if newStatus == "sudah_diambil" {
		_, err = tx.ExecContext(ctx, `
			UPDATE paket_items
			SET status = $1, status_changed_at = $2, status_changed_by = $3,
			    handed_over_at = $2, handed_over_by_user_id = $3, updated_at = $2
			WHERE id = $4
		`, newStatus, now, userID, itemID)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE paket_items
			SET status = $1, status_changed_at = $2, status_changed_by = $3, updated_at = $2
			WHERE id = $4
		`, newStatus, now, userID, itemID)
	}
	if err != nil {
		return err
	}

	if proofImageKey != nil && *proofImageKey != "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO paket_photos (paket_item_id, paket_report_id, image_key, photo_type, captured_at, created_at)
			VALUES ($1, $2, $3, 'bukti_pengambilan', $4, $4)
		`, itemID, reportID, *proofImageKey, now)
		if err != nil {
			return err
		}
	}

	if newStatus == "sudah_diambil" {
		var officerName string
		_ = tx.QueryRowContext(ctx, `
			SELECT COALESCE(full_name, 'Petugas Keamanan') FROM users WHERE id = $1
		`, userID).Scan(&officerName)
		if officerName == "" {
			officerName = "Petugas Keamanan"
		}

		var receiverUserID sql.NullString
		var residentName, itemDesc string
		_ = tx.QueryRowContext(ctx, `
			SELECT r.user_id, COALESCE(r.full_name, 'Warga'), COALESCE(pi.description, 'Barang titipan')
			FROM paket_items pi
			JOIN paket_reports pr ON pr.id = pi.paket_report_id
			JOIN residents r ON r.id = pr.receiver_warga_id
			WHERE pi.id = $1 AND pr.rt_id = $2
		`, itemID, rtID).Scan(&receiverUserID, &residentName, &itemDesc)

		loc, errLoc := time.LoadLocation("Asia/Jakarta")
		if errLoc != nil {
			loc = time.FixedZone("WIB", 7*3600)
		}
		timeStr := now.In(loc).Format("02 Jan 2006, 15:04 WIB")

		// 1. Notify recipient Warga
		if receiverUserID.Valid && receiverUserID.String != "" {
			dedup := fmt.Sprintf("paket_pickup:%s:%s", itemID, receiverUserID.String)
			body := fmt.Sprintf("Paket Anda (%s) telah diambil pada %s dengan bukti penyerahan oleh %s.", itemDesc, timeStr, officerName)
			_, err = tx.ExecContext(ctx, `
				INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at, payload)
				VALUES ($1::uuid, $2::uuid, 'paket_picked_up', $3, 'Paket Telah Diambil', $4, 'SENT', $5, json_build_object(
					'report_id', $6::text,
					'paket_id', $6::text,
					'package_id', $6::text,
					'item_id', $7::text,
					'recipient_user_id', $2::text,
					'handed_over_by', $8::text,
					'handed_over_at', $5::timestamptz
				)::jsonb)
				ON CONFLICT (dedup_key) DO NOTHING
			`, rtID, receiverUserID.String, dedup, body, now, reportID, itemID, officerName)
			if err != nil {
				return fmt.Errorf("failed to insert warga pickup notification: %w", err)
			}
		}

		// 2. Notify Ketua RT and all RT staff with notification.manage permission in the same RT
		var managerUserIDs []string
		mRows, err := tx.QueryContext(ctx, `
			SELECT DISTINCT u.id
			FROM users u
			JOIN user_rt_memberships urm ON urm.user_id = u.id
			LEFT JOIN position_permissions pp ON pp.position = urm.jabatan
			LEFT JOIN permission_types pt ON pt.id = pp.permission_id AND pt.code = 'notification.manage'
			WHERE urm.rt_id = $1 AND urm.is_active = true
			  AND (urm.jabatan = 'ketua' OR pt.id IS NOT NULL)
		`, rtID)
		if err == nil {
			for mRows.Next() {
				var mid string
				if err := mRows.Scan(&mid); err == nil {
					if receiverUserID.Valid && receiverUserID.String == mid {
						continue
					}
					managerUserIDs = append(managerUserIDs, mid)
				}
			}
			mRows.Close()

			managerBody := fmt.Sprintf("Paket untuk %s (%s) telah diserahkan pada %s oleh %s.", residentName, itemDesc, timeStr, officerName)
			for _, mid := range managerUserIDs {
				mDedup := fmt.Sprintf("paket_diserahkan:%s:%s", itemID, mid)
				recipientUID := ""
				if receiverUserID.Valid {
					recipientUID = receiverUserID.String
				}
				_, err = tx.ExecContext(ctx, `
					INSERT INTO notification_logs (rt_id, user_id, event_type, dedup_key, title, body, status, created_at, payload)
					VALUES ($1::uuid, $2::uuid, 'paket_picked_up', $3, 'Paket Telah Diserahkan', $4, 'SENT', $5, json_build_object(
						'report_id', $6::text,
						'paket_id', $6::text,
						'package_id', $6::text,
						'item_id', $7::text,
						'recipient_user_id', $8::text,
						'handed_over_by', $9::text,
						'handed_over_at', $5::timestamptz
					)::jsonb)
					ON CONFLICT (dedup_key) DO NOTHING
				`, rtID, mid, mDedup, managerBody, now, reportID, itemID, recipientUID, officerName)
				if err != nil {
					return fmt.Errorf("failed to insert manager handover notification: %w", err)
				}
			}
		}
	}

	return tx.Commit()
}

func (r *Repository) SearchWargaMasked(ctx context.Context, rtID, query string, limit int) ([]MaskedWargaSearchResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT r.id, r.full_name, COALESCE(h.house_number, '')
		FROM residents r
		LEFT JOIN LATERAL (
			SELECT ph.house_number
			FROM residency_periods rp
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
			JOIN physical_houses ph ON ph.id = ho.physical_house_id
			WHERE rp.resident_id = r.id AND rp.end_date IS NULL
			LIMIT 1
		) h ON true
		WHERE r.rt_id = $1 AND r.is_active = true
		  AND (LOWER(r.full_name) LIKE $2 OR LOWER(COALESCE(h.house_number, '')) LIKE $2)
		ORDER BY r.full_name ASC
		LIMIT $3
	`, rtID, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MaskedWargaSearchResult
	for rows.Next() {
		var res MaskedWargaSearchResult
		if err := rows.Scan(&res.ID, &res.FullName, &res.HouseNumber); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	if results == nil {
		results = []MaskedWargaSearchResult{}
	}
	return results, nil
}

func (r *Repository) MarkReadReceipt(ctx context.Context, userID, rtID, domain string, itemIDs []string) error {
	domain = strings.TrimSpace(domain)
	if domain != "correction" && domain != "patrol" && domain != "paket" {
		return newAppError(400, "invalid_domain", "domain must be correction, patrol, or paket", ErrValidation)
	}

	if len(itemIDs) > 0 {
		for _, id := range itemIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			_, _ = r.pool.Raw().ExecContext(ctx, `
				INSERT INTO security_read_receipts (user_id, rt_id, domain, item_id, read_at)
				VALUES ($1, $2, $3, $4, NOW())
				ON CONFLICT (user_id, domain, item_id) DO UPDATE SET read_at = NOW()
			`, userID, rtID, domain, id)
		}
		return nil
	}

	switch domain {
	case "correction":
		_, err := r.pool.Raw().ExecContext(ctx, `
			INSERT INTO security_read_receipts (user_id, rt_id, domain, item_id, read_at)
			SELECT $1, $2, 'correction', id, NOW()
			FROM security_attendance_corrections
			WHERE rt_id = $2 AND status = 'pending'
			ON CONFLICT (user_id, domain, item_id) DO NOTHING
		`, userID, rtID)
		return err
	case "patrol":
		_, err := r.pool.Raw().ExecContext(ctx, `
			INSERT INTO security_read_receipts (user_id, rt_id, domain, item_id, read_at)
			SELECT $1, $2, 'patrol', id, NOW()
			FROM patrol_bundles
			WHERE rt_id = $2
			ON CONFLICT (user_id, domain, item_id) DO NOTHING
		`, userID, rtID)
		return err
	case "paket":
		_, err := r.pool.Raw().ExecContext(ctx, `
			INSERT INTO security_read_receipts (user_id, rt_id, domain, item_id, read_at)
			SELECT $1, $2, 'paket', id, NOW()
			FROM paket_reports
			WHERE rt_id = $2
			ON CONFLICT (user_id, domain, item_id) DO NOTHING
		`, userID, rtID)
		return err
	}
	return nil
}

func (r *Repository) GetSecurityAlerts(ctx context.Context, rtID, userID string) (*SecurityAlertsResult, error) {
	var res SecurityAlertsResult

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM security_attendance_corrections
		WHERE rt_id = $1 AND status = 'pending'
	`, rtID).Scan(&res.PendingCorrections)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM patrol_bundles
		WHERE rt_id = $1 AND submitted_at >= NOW() - INTERVAL '24 hours'
	`, rtID).Scan(&res.RecentPatrols)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM security_attendance_corrections c
		WHERE c.rt_id = $1 AND c.status = 'pending'
		  AND NOT EXISTS (
			  SELECT 1 FROM security_read_receipts r
			  WHERE r.user_id = $2 AND r.domain = 'correction' AND r.item_id = c.id
		  )
	`, rtID, userID).Scan(&res.PendingCorrectionsUnread)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM patrol_bundles pb
		WHERE pb.rt_id = $1 AND pb.created_by != $2
		  AND pb.submitted_at >= NOW() - INTERVAL '7 days'
		  AND NOT EXISTS (
			  SELECT 1 FROM security_read_receipts r
			  WHERE r.user_id = $2 AND r.domain = 'patrol' AND r.item_id = pb.id
		  )
	`, rtID, userID).Scan(&res.PatrolUnread)

	_ = r.pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM paket_reports pr
		WHERE pr.rt_id = $1
		  AND pr.created_at >= NOW() - INTERVAL '7 days'
		  AND NOT EXISTS (
			  SELECT 1 FROM security_read_receipts r
			  WHERE r.user_id = $2 AND r.domain = 'paket' AND r.item_id = pr.id
		  )
	`, rtID, userID).Scan(&res.PaketUnread)

	res.HasAlert = res.PendingCorrectionsUnread > 0 || res.PatrolUnread > 0 || res.PaketUnread > 0
	return &res, nil
}

func (r *Repository) GetPatrolPhotoKey(ctx context.Context, rtID, photoID string) (string, error) {
	var key string
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT pp.image_key
		FROM patrol_photos pp
		JOIN patrol_bundles pb ON pb.id = pp.patrol_bundle_id
		WHERE pp.id = $1 AND pb.rt_id = $2
	`, photoID, rtID).Scan(&key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", newAppError(404, "not_found", "patrol photo not found", ErrNotFound)
		}
		return "", err
	}
	return key, nil
}

func (r *Repository) GetPaketPhotoKey(ctx context.Context, rtID, photoID string) (string, error) {
	var key string
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT pp.image_key
		FROM paket_photos pp
		JOIN paket_reports pr ON pr.id = pp.paket_report_id
		WHERE pp.id = $1 AND pr.rt_id = $2
	`, photoID, rtID).Scan(&key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", newAppError(404, "not_found", "paket photo not found", ErrNotFound)
		}
		return "", err
	}
	return key, nil
}

func (r *Repository) IsPaketReceiver(ctx context.Context, rtID, reportID, userID string) (bool, error) {
	var exists bool
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM paket_reports pr
			JOIN residents res ON res.id = pr.receiver_warga_id
			WHERE (pr.id = $1 OR pr.id IN (SELECT pi.paket_report_id FROM paket_items pi WHERE pi.id = $1))
			  AND pr.rt_id = $2
			  AND res.rt_id = $2
			  AND res.user_id = $3
		)
	`, reportID, rtID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *Repository) IsPaketPhotoReceiver(ctx context.Context, rtID, photoID, userID string) (bool, error) {
	var exists bool
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM paket_photos pp
			JOIN paket_reports pr ON pr.id = pp.paket_report_id
			JOIN residents res ON res.id = pr.receiver_warga_id
			WHERE pp.id = $1 AND pr.rt_id = $2 AND res.rt_id = $2 AND res.user_id = $3
		)
	`, photoID, rtID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
