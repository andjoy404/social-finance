package rt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound  = errors.New("rt not found")
	ErrDuplicate = errors.New("rt with same rw and rt code already exists")
)

// isPGUniqueViolation checks for PostgreSQL unique constraint violation
// via the typed SQLSTATE code 23505 (unique_violation).
// This avoids reliance on English error-message text.
func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}

type updateField struct {
	column string
	value  interface{}
}

func setClause(fields []updateField, baseIdx int) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = fmt.Sprintf("%s = $%d", f.column, baseIdx+i+1)
	}
	return strings.Join(parts, ", ")
}

// Create inserts a new RT and returns it with generated fields.
func Create(ctx context.Context, tx *sql.Tx, in CreateRTInput) (*RT, error) {
	var item RT
	err := tx.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, address, head_name)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, name, rw, rt, address, head_name, is_active, created_at, updated_at`,
		in.Name, in.RW, in.RT, in.Address, in.HeadName,
	).Scan(
		&item.ID, &item.Name, &item.RW, &item.RT,
		&item.Address, &item.HeadName, &item.IsActive,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if isPGUniqueViolation(err) {
			return nil, fmt.Errorf("create rt: %w", ErrDuplicate)
		}
		return nil, fmt.Errorf("create rt: %w", err)
	}
	return &item, nil
}

// GetByID finds an RT by UUID. Returns ErrNotFound when not found.
func GetByID(ctx context.Context, tx *sql.Tx, id string) (*RT, error) {
	var item RT
	err := tx.QueryRowContext(ctx,
		`SELECT id, name, rw, rt, address, head_name, is_active, created_at, updated_at
		 FROM rts WHERE id = $1 LIMIT 1`,
		id,
	).Scan(
		&item.ID, &item.Name, &item.RW, &item.RT,
		&item.Address, &item.HeadName, &item.IsActive,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get rt by id: %w", err)
	}
	return &item, nil
}

// ListAll returns RTs with optional active filter, search, and pagination.
func ListAll(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, offset, limit int) ([]*RT, error) {
	query := `SELECT id, name, rw, rt, address, head_name, is_active, created_at, updated_at
		FROM rts`

	var args []interface{}
	argIndex := 1
	where := make([]string, 0, 2)

	if isActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *isActive)
		argIndex++
	}
	if search != nil && strings.TrimSpace(*search) != "" {
		term := "%" + strings.ToLower(*search) + "%"
		where = append(where, "(LOWER(name) LIKE $"+fmt.Sprint(argIndex)+" OR LOWER(rt) LIKE $"+fmt.Sprint(argIndex)+" OR LOWER(head_name) LIKE $"+fmt.Sprint(argIndex)+")")
		args = append(args, term)
		argIndex++
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY created_at ASC OFFSET $%d LIMIT $%d", argIndex, argIndex+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list rts: %w", err)
	}
	defer rows.Close()

	var result []*RT
	for rows.Next() {
		var item RT
		if err := rows.Scan(
			&item.ID, &item.Name, &item.RW, &item.RT,
			&item.Address, &item.HeadName, &item.IsActive,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan rt: %w", err)
		}
		result = append(result, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rts: %w", err)
	}
	return result, nil
}

// Count returns the number of RTs matching the active filter and search.
func Count(ctx context.Context, tx *sql.Tx, isActive *bool, search *string) (int, error) {
	query := `SELECT COUNT(*) FROM rts`

	var args []interface{}
	argIndex := 1
	where := make([]string, 0, 2)

	if isActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *isActive)
		argIndex++
	}
	if search != nil && strings.TrimSpace(*search) != "" {
		term := "%" + strings.ToLower(*search) + "%"
		where = append(where, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER(rt) LIKE $%d OR LOWER(head_name) LIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, term)
		argIndex++
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count rts: %w", err)
	}
	return count, nil
}

// Update partially updates an RT. Nil fields in the input are skipped.
// Returns the updated RT or ErrNotFound.
func Update(ctx context.Context, tx *sql.Tx, id string, in *UpdateRTInput) (*RT, error) {
	var fields []updateField
	var args []interface{}
	argIndex := 1

	if in.Name != nil {
		fields = append(fields, updateField{"name", *in.Name})
		args = append(args, *in.Name)
		argIndex++
	}
	if in.RW != nil {
		fields = append(fields, updateField{"rw", *in.RW})
		args = append(args, *in.RW)
		argIndex++
	}
	if in.RT != nil {
		fields = append(fields, updateField{"rt", *in.RT})
		args = append(args, *in.RT)
		argIndex++
	}
	if in.Address != nil {
		fields = append(fields, updateField{"address", *in.Address})
		args = append(args, *in.Address)
		argIndex++
	}
	if in.HeadName != nil {
		fields = append(fields, updateField{"head_name", *in.HeadName})
		args = append(args, *in.HeadName)
		argIndex++
	}
	if in.IsActive != nil {
		fields = append(fields, updateField{"is_active", *in.IsActive})
		args = append(args, *in.IsActive)
		argIndex++
	}

	if len(fields) == 0 {
		return nil, ErrNotFound
	}

	args = append(args, id)

	var item RT
	err := tx.QueryRowContext(ctx,
		`UPDATE rts SET `+setClause(fields, 0)+
			`, updated_at = now() WHERE id = $`+fmt.Sprint(len(fields)+1)+
			` RETURNING id, name, rw, rt, address, head_name, is_active, created_at, updated_at`,
		args...,
	).Scan(
		&item.ID, &item.Name, &item.RW, &item.RT,
		&item.Address, &item.HeadName, &item.IsActive,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update rt: %w", err)
	}
	return &item, nil
}

// Deactivates an RT (soft delete). Returns ErrNotFound if not found.
func Deactivate(ctx context.Context, tx *sql.Tx, id string) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE rts SET is_active = false, updated_at = now()
		 WHERE id = $1 AND is_active = true`,
		id,
	)
	if err != nil {
		return fmt.Errorf("deactivate rt: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deactivate rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
