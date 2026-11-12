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
		       (SELECT r.avatar_path FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1)
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
		var residentID sql.NullString
		if err := rows.Scan(&o.UserID, &o.MembershipID, &o.RTID, &o.FullName, &o.Email, &phone, &o.IsActive, &residentID, &avatarPath); err != nil {
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
			return newAppError(404, "not_found", "schedule not found", ErrNotFound)
		}
		return err
	}
	if checkedInAt.Valid || (attStatus.Valid && attStatus.String != StatusBelumHadir) {
		return newAppError(409, "attendance_locked", "cannot cancel schedule after attendance has been recorded", ErrConflict)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE security_schedules SET status = 'cancelled', updated_by = $2, updated_at = $3
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
		       s.scheduled_start_at, s.scheduled_end_at, s.notes, s.status,
		       COALESCE(ar.status, 'belum_hadir'), ar.checked_in_at, ar.checked_out_at,
		       s.created_at, s.updated_at
		FROM security_schedules s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
		WHERE %s
		ORDER BY s.shift_date ASC, s.scheduled_start_at ASC, u.full_name ASC
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
		var notes sql.NullString
		var cin, cout sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.AssignmentID, &s.AssignmentStartDate, &s.AssignmentEndDate,
			&s.RTID, &s.UserID, &s.OfficerName, &s.MembershipID, &s.ShiftConfigID, &s.ShiftVersion,
			&s.ShiftDate, &s.ShiftName, &s.StartTime, &s.EndTime,
			&s.CrossesMidnight, &s.LatenessToleranceMinutes,
			&s.CheckInWindowBeforeMinutes, &s.CheckInWindowAfterMinutes,
			&s.AbsentCutoffMinutes, &s.MissingCheckoutCutoffMinutes,
			&s.ScheduledStartAt, &s.ScheduledEndAt, &notes, &s.Status,
			&s.AttendanceStatus, &cin, &cout, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if notes.Valid {
			v := notes.String
			s.Notes = &v
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
		StartAt   time.Time
		EndAt     time.Time
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
	if !reqSched.EndAt.After(now) || !tgtSched.EndAt.After(now) {
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

	// Read start/end bounds for overlap checks after swap.
	var reqStart, reqEnd, tgtStart, tgtEnd time.Time
	_ = tx.QueryRowContext(ctx, `SELECT scheduled_start_at, scheduled_end_at FROM security_schedules WHERE id = $1`, reqSchedID).Scan(&reqStart, &reqEnd)
	_ = tx.QueryRowContext(ctx, `SELECT scheduled_start_at, scheduled_end_at FROM security_schedules WHERE id = $1`, tgtSchedID).Scan(&tgtStart, &tgtEnd)

	// Check requester receiving tgtSched window doesn't overlap other schedules of requester (excluding reqSchedID & tgtSchedID).
	var reqOverlap int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_schedules
		WHERE rt_id = $1 AND user_id = $2 AND status = 'scheduled'
		  AND id NOT IN ($3, $4) AND scheduled_start_at < $6 AND scheduled_end_at > $5
	`, rtID, reqUser, reqSchedID, tgtSchedID, tgtStart, tgtEnd).Scan(&reqOverlap); err != nil {
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
	`, rtID, tgtUser, reqSchedID, tgtSchedID, reqStart, reqEnd).Scan(&tgtOverlap); err != nil {
		return nil, err
	}
	if tgtOverlap > 0 {
		return nil, newAppError(409, "schedule_conflict", "swap creates a schedule conflict for target officer", ErrConflict)
	}

	// Swap owners atomically on both schedules and their attendance records.
	if _, err := tx.ExecContext(ctx, `
		UPDATE security_schedules SET user_id = $2, membership_id = $3, updated_by = $4, updated_at = $5
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
		       sw.requester_schedule_id, to_char(rs.shift_date, 'YYYY-MM-DD'), rs.shift_name_snapshot,
		       sw.target_schedule_id, to_char(ts.shift_date, 'YYYY-MM-DD'), ts.shift_name_snapshot,
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
		       sw.requester_schedule_id, to_char(rs.shift_date, 'YYYY-MM-DD'), rs.shift_name_snapshot,
		       sw.target_schedule_id, to_char(ts.shift_date, 'YYYY-MM-DD'), ts.shift_name_snapshot,
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
	rtID, userID, rawQRToken, action, idemKey string,
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
	windowStartUpper := now.Add(12 * time.Hour)
	windowEndLower := now.Add(-14 * time.Hour)
	rows, err := tx.QueryContext(ctx, `
		SELECT s.id, s.shift_config_id, s.shift_version, to_char(s.shift_date, 'YYYY-MM-DD'),
		       s.shift_name_snapshot, to_char(s.start_time_snapshot, 'HH24:MI'), to_char(s.end_time_snapshot, 'HH24:MI'),
		       s.crosses_midnight_snapshot, s.lateness_tolerance_minutes,
		       s.check_in_window_before_minutes, s.check_in_window_after_minutes,
		       s.absent_cutoff_minutes, s.missing_checkout_cutoff_minutes,
		       s.scheduled_start_at, s.scheduled_end_at,
		       ar.id, ar.status, ar.checked_in_at, ar.checked_out_at
		FROM security_schedules s
		LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
		WHERE s.rt_id = $1
		  AND s.user_id = $2
		  AND s.status = 'scheduled'
		  AND s.scheduled_start_at <= $3
		  AND s.scheduled_end_at >= $4
		ORDER BY s.scheduled_start_at ASC
		FOR UPDATE OF s
	`, rtID, userID, windowStartUpper, windowEndLower)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		scheduleID, shiftConfigID, shiftDate, shiftName, startTime, endTime string
		shiftVersion, tolMin, winBefore, winAfter, absentMin, missOutMin    int
		crossesMidnight                                                     bool
		schedStart, schedEnd                                                time.Time
		attID, attStatus                                                    sql.NullString
		checkedIn, checkedOut                                               sql.NullTime
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(
			&c.scheduleID, &c.shiftConfigID, &c.shiftVersion, &c.shiftDate,
			&c.shiftName, &c.startTime, &c.endTime, &c.crossesMidnight,
			&c.tolMin, &c.winBefore, &c.winAfter, &c.absentMin, &c.missOutMin,
			&c.schedStart, &c.schedEnd,
			&c.attID, &c.attStatus, &c.checkedIn, &c.checkedOut,
		); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	if len(candidates) == 0 {
		return nil, newAppError(400, "no_matching_schedule", "you do not have an active scheduled shift right now", ErrNoActiveSchedule)
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
				if candidates[i].checkedOut.Valid || (candidates[i].attStatus.Valid && candidates[i].attStatus.String == StatusSudahAbsenPulang) {
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
				if c.attStatus.Valid && c.attStatus.String == StatusSudahAbsenPulang {
					return nil, newAppError(409, "already_checked_out", "you have already completed attendance for this shift", ErrAlreadyCheckedOut)
				}
			}
			return nil, newAppError(400, "check_in_window_closed", "current time is outside the allowed attendance window", ErrWindowClosed)
		}
	}

	// Ensure attendance row exists, then lock it directly via FOR UPDATE and re-verify state under READ COMMITTED.
	if !chosen.attID.Valid {
		var insertedID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO security_attendance_records (
				rt_id, schedule_id, user_id, shift_config_id, shift_version, shift_date, status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6::date, 'belum_hadir', $7, $7)
			ON CONFLICT (schedule_id) DO UPDATE SET updated_at = security_attendance_records.updated_at
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
	var cin, cout, absAt, missAt sql.NullTime
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
		&rec.ScheduledStartAt, &rec.ScheduledEndAt,
		&rec.Status, &arrStatus, &rec.LateMinutes,
		&inDevID, &inDevName, &outDevID, &outDevName,
		&cin, &cout, &absAt, &missAt, &rec.CorrectionLocked, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
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
	where := []string{"ar.rt_id = $1", "s.status = 'scheduled'"}
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
		ORDER BY ar.shift_date DESC, s.scheduled_start_at DESC, u.full_name ASC
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
		var cin, cout, absAt, missAt sql.NullTime
		if err := rows.Scan(
			&rec.ID, &rec.RTID, &rec.ScheduleID, &rec.UserID, &rec.OfficerName,
			&rec.ShiftConfigID, &rec.ShiftVersion, &rec.ShiftDate,
			&rec.ShiftName, &rec.StartTime, &rec.EndTime, &rec.CrossesMidnight,
			&rec.ScheduledStartAt, &rec.ScheduledEndAt,
			&rec.Status, &arrStatus, &rec.LateMinutes,
			&inDevID, &inDevName, &outDevID, &outDevName,
			&cin, &cout, &absAt, &missAt, &rec.CorrectionLocked, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, 0, err
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

	var scheduleID, officerUserID, origStatus string
	var origCin, origCout sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT schedule_id, user_id, status, checked_in_at, checked_out_at
		FROM security_attendance_records
		WHERE id = $1 AND rt_id = $2
		FOR UPDATE
	`, req.AttendanceRecordID, rtID).Scan(&scheduleID, &officerUserID, &origStatus, &origCin, &origCout)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, newAppError(404, "not_found", "attendance record not found", ErrNotFound)
		}
		return nil, err
	}
	if !isManager && officerUserID != requesterID {
		return nil, newAppError(403, "forbidden", "you can only request corrections for your own attendance", ErrForbidden)
	}

	var pendingCnt int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM security_attendance_corrections
		WHERE attendance_record_id = $1 AND status = 'pending'
	`, req.AttendanceRecordID).Scan(&pendingCnt); err != nil {
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
	`, rtID, req.AttendanceRecordID, scheduleID, officerUserID, requesterID,
		req.RequestType, origStatus, origCin, origCout,
		req.RequestedStatus, reqCin, reqCout, strings.TrimSpace(req.Reason), now,
	).Scan(&corrID)
	if err != nil {
		return nil, err
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
		SELECT ar.status, ar.checked_in_at, ar.checked_out_at, s.scheduled_start_at, s.lateness_tolerance_minutes
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
	}

	var arrivalStatus sql.NullString
	lateMin := 0
	if reqStatus == StatusHadir {
		arrivalStatus = sql.NullString{String: StatusHadir, Valid: true}
	} else if reqStatus == StatusTerlambat {
		arrivalStatus = sql.NullString{String: StatusTerlambat, Valid: true}
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
		       to_char(s.shift_date, 'YYYY-MM-DD'), s.shift_name_snapshot,
		       c.request_type, c.original_status, c.original_checked_in_at, c.original_checked_out_at,
		       c.requested_status, c.requested_checked_in_at, c.requested_checked_out_at,
		       c.reason, c.status, c.decided_by, du.full_name, c.decided_at, c.decision_note,
		       c.final_status, c.final_checked_in_at, c.final_checked_out_at,
		       c.created_at, c.updated_at
		FROM security_attendance_corrections c
		JOIN security_schedules s ON s.id = c.schedule_id
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
			destination_label, vehicle_plate, notes, checked_in_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $10)
		RETURNING id
	`, rtID, deviceID, strings.TrimSpace(req.GuestName), req.Phone, strings.TrimSpace(req.Purpose),
		req.HostHouseholdID, strings.TrimSpace(req.DestinationLabel), req.VehiclePlate, req.Notes, now).Scan(&id)
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
		       g.purpose, g.host_household_id, g.destination_label, g.vehicle_plate, g.notes,
		       g.checked_in_at, g.checked_out_at, g.checkout_device_id, g.created_at, g.updated_at
		FROM security_guest_books g
		JOIN security_devices d ON d.id = g.device_id
		WHERE %s
		ORDER BY g.checked_in_at DESC
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
		var phone, hostID, plate, notes, outDev sql.NullString
		var cout sql.NullTime
		if err := rows.Scan(
			&g.ID, &g.RTID, &g.DeviceID, &g.DeviceName, &g.GuestName, &phone,
			&g.Purpose, &hostID, &g.DestinationLabel, &plate, &notes,
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
		SELECT s.user_id, u.full_name, s.shift_name_snapshot,
		       to_char(s.start_time_snapshot, 'HH24:MI'), to_char(s.end_time_snapshot, 'HH24:MI'),
		       COALESCE(ar.status, 'belum_hadir'), u.phone,
		       (SELECT r.avatar_path FROM residents r WHERE r.user_id = u.id AND r.is_active = true LIMIT 1)
		FROM security_schedules s
		JOIN users u ON u.id = s.user_id
		JOIN security_attendance_records ar ON ar.schedule_id = s.id
		JOIN user_rt_memberships um ON um.id = s.membership_id AND um.rt_id = s.rt_id AND um.is_active = true
		WHERE s.rt_id = $1
		  AND s.status = 'scheduled'
		  AND s.scheduled_start_at <= $2
		  AND s.scheduled_end_at > $2
		  AND u.is_active = true
		  AND ar.checked_in_at IS NOT NULL
		  AND ar.checked_out_at IS NULL
		  AND ar.status IN ('hadir', 'terlambat', 'belum_absen_pulang')
		ORDER BY ar.checked_in_at ASC NULLS LAST, s.scheduled_start_at ASC, u.full_name ASC
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
		if err := rows.Scan(&item.UserID, &item.FullName, &item.ShiftName, &item.ShiftStart, &item.ShiftEnd, &item.Status, &phone, &avatarPath); err != nil {
			return nil, err
		}
		if avatarPath.Valid && strings.TrimSpace(avatarPath.String) != "" {
			v := avatarPath.String
			item.AvatarPath = &v
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

	// Ensure any active schedule without an attendance row yet gets initialized cleanly first.
	_, err = conn.ExecContext(ctx, `
		INSERT INTO security_attendance_records (
			rt_id, schedule_id, user_id, shift_config_id, shift_version, shift_date, status, created_at, updated_at
		)
		SELECT s.rt_id, s.id, s.user_id, s.shift_config_id, s.shift_version, s.shift_date, 'belum_hadir', $1, $1
		FROM security_schedules s
		LEFT JOIN security_attendance_records ar ON ar.schedule_id = s.id
		WHERE s.status = 'scheduled' AND ar.id IS NULL
		ON CONFLICT (schedule_id) DO NOTHING
	`, now)
	if err != nil {
		return nil, err
	}

	// 1. Mark Tidak Hadir once absent cutoff has elapsed with no check-in.
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
		  AND $1 > (s.scheduled_start_at + (s.absent_cutoff_minutes * INTERVAL '1 minute'))
	`, now)
	if err != nil {
		return nil, err
	}
	if n, err := absRes.RowsAffected(); err == nil {
		res.MarkedAbsentCount = int(n)
	}

	// 2. Mark Belum Absen Pulang once shift end + missing_checkout_cutoff has elapsed with no check-out.
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
		  AND $1 > (s.scheduled_end_at + (s.missing_checkout_cutoff_minutes * INTERVAL '1 minute'))
	`, now)
	if err != nil {
		return nil, err
	}
	if n, err := missRes.RowsAffected(); err == nil {
		res.MarkedMissingCheckout = int(n)
	}

	return res, nil
}
