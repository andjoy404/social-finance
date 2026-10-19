package rt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrRWMembershipNotFound = errors.New("active RW membership not found")
	ErrInvalidRWJabatan     = errors.New("invalid RW jabatan")
	ErrDuplicateRWJabatan   = errors.New("jabatan already assigned in RW")
	ErrRWInactive           = errors.New("rw is inactive")
	ErrRTNotInRW            = errors.New("rt is not in this rw")
	ErrUserNotRTMember      = errors.New("user is not an active member of the selected RT")
)

// AllowedRWJabatans defines jabatan allowed for RW.
var AllowedRWJabatans = map[string]bool{
	"ketua":       true,
	"wakil_ketua": true,
	"sekretaris":  true,
	"bendahara":   true,
}

type RWMembership struct {
	ID       string  `json:"membership_id"`
	UserID   string  `json:"user_id"`
	FullName string  `json:"full_name"`
	Email    *string `json:"email,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Role     string  `json:"role"`
	Jabatan  *string `json:"jabatan,omitempty"`
	RTID     *string `json:"rt_id,omitempty"`
	RTName   *string `json:"rt_name,omitempty"`
	IsActive bool    `json:"is_active"`
}

func ListRWMemberships(ctx context.Context, tx *sql.Tx, rwID string) ([]RWMembership, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT urwm.id, u.id, u.full_name, u.email, u.phone, urwm.role, urwm.jabatan, urwm.rt_id, r.name, urwm.is_active
		FROM user_rw_memberships urwm
		JOIN users u ON u.id = urwm.user_id
		LEFT JOIN rts r ON r.id = urwm.rt_id
		WHERE urwm.rw_id = $1 AND urwm.is_active = true AND u.is_active = true
		ORDER BY u.full_name ASC, u.id ASC`, rwID)
	if err != nil {
		return nil, fmt.Errorf("list RW memberships: %w", err)
	}
	defer rows.Close()
	result := make([]RWMembership, 0)
	for rows.Next() {
		var item RWMembership
		var rtID, rtName sql.NullString
		if err := rows.Scan(&item.ID, &item.UserID, &item.FullName, &item.Email, &item.Phone, &item.Role, &item.Jabatan, &rtID, &rtName, &item.IsActive); err != nil {
			return nil, fmt.Errorf("scan RW membership: %w", err)
		}
		if rtID.Valid {
			v := rtID.String
			item.RTID = &v
		}
		if rtName.Valid {
			v := rtName.String
			item.RTName = &v
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate RW memberships: %w", err)
	}
	return result, nil
}

func SetRWJabatan(ctx context.Context, tx *sql.Tx, rwID, userID, rtID, actorID string, jabatan string) (*RWMembership, *string, error) {
	if !AllowedRWJabatans[jabatan] {
		return nil, nil, ErrInvalidRWJabatan
	}

	// Lock the RW row
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM rws WHERE id = $1 FOR UPDATE`, rwID).Scan(&active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrRWNotFound
		}
		return nil, nil, fmt.Errorf("lock RW: %w", err)
	}
	if !active {
		return nil, nil, ErrRWInactive
	}

	// Verify RT belongs to this RW
	var rtRWID *string
	err := tx.QueryRowContext(ctx, `SELECT rw_id FROM rts WHERE id = $1 AND is_active = true`, rtID).Scan(&rtRWID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrRTNotInRW
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load RT: %w", err)
	}
	if rtRWID == nil || *rtRWID != rwID {
		return nil, nil, ErrRTNotInRW
	}

	// Verify user is an active member of this RT
	var membershipID string
	var oldJabatan sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT urm.id, urm.jabatan
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE urm.rt_id = $1 AND urm.user_id = $2 AND urm.is_active = true AND u.is_active = true
		FOR UPDATE`, rtID, userID).Scan(&membershipID, &oldJabatan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrUserNotRTMember
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load RT membership: %w", err)
	}

	var old *string
	if oldJabatan.Valid {
		v := oldJabatan.String
		old = &v
	}

	// Idempotent: if same jabatan, return
	if old != nil && *old == jabatan {
		// Load full membership for response
		var item RWMembership
		_ = tx.QueryRowContext(ctx, `
			SELECT urwm.id, u.id, u.full_name, u.email, u.phone, urwm.role, urwm.jabatan, urwm.rt_id, r.name, urwm.is_active
			FROM user_rw_memberships urwm JOIN users u ON u.id = urwm.user_id
			LEFT JOIN rts r ON r.id = urwm.rt_id
			WHERE urwm.rw_id = $1 AND urwm.user_id = $2 AND urwm.is_active = true`, rwID, userID).Scan(
			&item.ID, &item.UserID, &item.FullName, &item.Email, &item.Phone, &item.Role, &item.Jabatan, &item.RTID, &item.RTName, &item.IsActive)
		return &item, old, nil
	}

	// Single-holder check for this jabatan in this RW
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id FROM user_rw_memberships WHERE rw_id = $1 AND is_active = true AND jabatan = $2 AND user_id <> $3 LIMIT 1`, rwID, jabatan, userID).Scan(&existing)
	if err == nil {
		return nil, nil, ErrDuplicateRWJabatan
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("check existing RW jabatan: %w", err)
	}

	// Upsert: update existing or insert new
	var item RWMembership
	var itemID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO user_rw_memberships (user_id, rw_id, role, jabatan, rt_id, is_active)
		VALUES ($1, $2, 'pengurus', $3, $4, true)
		ON CONFLICT (user_id, rw_id) WHERE is_active = true
		DO UPDATE SET jabatan = $3, rt_id = $4, updated_at = now()
		RETURNING id`, userID, rwID, jabatan, rtID).Scan(&itemID)
	if err != nil {
		return nil, nil, fmt.Errorf("upsert RW membership: %w", err)
	}

	// Load full response
	_ = tx.QueryRowContext(ctx, `
		SELECT urwm.id, u.id, u.full_name, u.email, u.phone, urwm.role, urwm.jabatan, urwm.rt_id, r.name, urwm.is_active
		FROM user_rw_memberships urwm JOIN users u ON u.id = urwm.user_id
		LEFT JOIN rts r ON r.id = urwm.rt_id
		WHERE urwm.id = $1`, itemID).Scan(
		&item.ID, &item.UserID, &item.FullName, &item.Email, &item.Phone, &item.Role, &item.Jabatan, &item.RTID, &item.RTName, &item.IsActive)

	// Record history
	var actorPtr *string
	if actorID != "" {
		actorPtr = &actorID
	}
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO user_rw_membership_history (rw_membership_id, user_id, rw_id, rt_id, jabatan_old, jabatan_new, action, actor_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		itemID, userID, rwID, rtID, old, jabatan, "assign", actorPtr)

	return &item, old, nil
}

func RemoveRWJabatan(ctx context.Context, tx *sql.Tx, rwID, userID, actorID string) (*RWMembership, *string, error) {
	var item RWMembership
	var oldJabatan sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT urwm.id, u.id, u.full_name, u.email, u.phone, urwm.role, urwm.jabatan, urwm.rt_id, r.name, urwm.is_active
		FROM user_rw_memberships urwm JOIN users u ON u.id = urwm.user_id
		LEFT JOIN rts r ON r.id = urwm.rt_id
		WHERE urwm.rw_id = $1 AND urwm.user_id = $2 AND urwm.is_active = true AND u.is_active = true AND urwm.jabatan IS NOT NULL
		FOR UPDATE OF urwm`, rwID, userID).Scan(&item.ID, &item.UserID, &item.FullName, &item.Email, &item.Phone, &item.Role, &oldJabatan, &item.RTID, &item.RTName, &item.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrRWMembershipNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load RW jabatan: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_rw_memberships SET jabatan = NULL, updated_at = now() WHERE id = $1`, item.ID); err != nil {
		return nil, nil, fmt.Errorf("remove RW jabatan: %w", err)
	}
	var old *string
	if oldJabatan.Valid {
		v := oldJabatan.String
		old = &v
	}
	item.Jabatan = nil

	var actorPtr *string
	if actorID != "" {
		actorPtr = &actorID
	}
	// Record history
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO user_rw_membership_history (rw_membership_id, user_id, rw_id, rt_id, jabatan_old, jabatan_new, action, actor_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		item.ID, userID, rwID, item.RTID, old, nil, "revoke", actorPtr)

	return &item, old, nil
}
