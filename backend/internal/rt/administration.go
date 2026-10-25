package rt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"social-finance/internal/auth"
)

var (
	ErrMembershipNotFound = errors.New("active membership not found in RT")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidJabatan     = errors.New("invalid jabatan")
	ErrDuplicateJabatan   = errors.New("jabatan ketua is already assigned in RT")
)

type RTMembership struct {
	ID       string  `json:"membership_id"`
	UserID   string  `json:"user_id"`
	FullName string  `json:"full_name"`
	Email    *string `json:"email,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Role     string  `json:"role"`
	Jabatan  *string `json:"jabatan,omitempty"`
	IsActive bool    `json:"is_active"`
}

func ListMemberships(ctx context.Context, tx *sql.Tx, rtID string) ([]RTMembership, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT urm.id, u.id, u.full_name, u.email, u.phone, urm.role, urm.jabatan, urm.is_active
		FROM user_rt_memberships urm
		JOIN users u ON u.id = urm.user_id
		WHERE urm.rt_id = $1 AND urm.is_active = true AND u.is_active = true
		ORDER BY u.full_name ASC, u.id ASC`, rtID)
	if err != nil {
		return nil, fmt.Errorf("list RT memberships: %w", err)
	}
	defer rows.Close()
	result := make([]RTMembership, 0)
	for rows.Next() {
		var item RTMembership
		if err := rows.Scan(&item.ID, &item.UserID, &item.FullName, &item.Email, &item.Phone, &item.Role, &item.Jabatan, &item.IsActive); err != nil {
			return nil, fmt.Errorf("scan RT membership: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate RT memberships: %w", err)
	}
	return result, nil
}

func SetJabatan(ctx context.Context, tx *sql.Tx, rtID, userID string, jabatan auth.Jabatan) (*RTMembership, *string, error) {
	if !auth.ValidJabatans[jabatan] {
		return nil, nil, ErrInvalidJabatan
	}
	// Lock the organization row so concurrent assignments cannot both pass the
	// single-Ketua check. The membership itself is updated under this lock.
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM rts WHERE id = $1 FOR UPDATE`, rtID).Scan(&active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("lock RT: %w", err)
	}
	if !active {
		return nil, nil, ErrNotFound
	}

	var item RTMembership
	var old *string
	var oldValue sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT urm.id, u.id, u.full_name, u.email, u.phone, urm.role, urm.jabatan, urm.is_active
		FROM user_rt_memberships urm JOIN users u ON u.id = urm.user_id
		WHERE urm.rt_id = $1 AND urm.user_id = $2 AND urm.is_active = true AND u.is_active = true
		FOR UPDATE`, rtID, userID).Scan(&item.ID, &item.UserID, &item.FullName, &item.Email, &item.Phone, &item.Role, &oldValue, &item.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrMembershipNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load membership: %w", err)
	}
	if oldValue.Valid {
		v := oldValue.String
		old = &v
	}
	if old != nil && *old == string(jabatan) {
		item.Jabatan = old
		return &item, old, nil
	}

	if jabatan == auth.JabatanKetua || jabatan == auth.JabatanWakilKetua {
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT id FROM user_rt_memberships WHERE rt_id = $1 AND is_active = true AND jabatan = $2 AND user_id <> $3 LIMIT 1`, rtID, string(jabatan), userID).Scan(&existing)
		if err == nil {
			return nil, nil, ErrDuplicateJabatan
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("check existing jabatan: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_rt_memberships SET jabatan = $1, updated_at = now() WHERE id = $2`, string(jabatan), item.ID); err != nil {
		return nil, nil, fmt.Errorf("set jabatan: %w", err)
	}
	v := string(jabatan)
	item.Jabatan = &v
	return &item, old, nil
}

func RemoveJabatan(ctx context.Context, tx *sql.Tx, rtID, userID string) (*RTMembership, *string, error) {
	var item RTMembership
	var oldValue sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT urm.id, u.id, u.full_name, u.email, u.phone, urm.role, urm.jabatan, urm.is_active
		FROM user_rt_memberships urm JOIN users u ON u.id = urm.user_id
		WHERE urm.user_id = $1 AND urm.rt_id = $2 AND urm.is_active = true AND u.is_active = true AND urm.jabatan IS NOT NULL
		FOR UPDATE`, userID, rtID).Scan(&item.ID, &item.UserID, &item.FullName, &item.Email, &item.Phone, &item.Role, &oldValue, &item.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrMembershipNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load jabatan: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_rt_memberships SET jabatan = NULL, updated_at = now() WHERE id = $1`, item.ID); err != nil {
		return nil, nil, fmt.Errorf("remove jabatan: %w", err)
	}
	v := oldValue.String
	return &item, &v, nil
}

func normalizeJabatan(raw string) auth.Jabatan { return auth.Jabatan(strings.TrimSpace(raw)) }
