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
	ErrNotFound       = errors.New("rt not found")
	ErrDuplicate      = errors.New("rt with same rw and rt code already exists")
	ErrRWNotFound     = errors.New("rw not found")
	ErrRWDuplicate    = errors.New("rw with same number already exists")
	ErrRWHasActiveRTs = errors.New("cannot deactivate rw while active rts are attached")
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

// ensureRWForNumber finds an active RW by rw_number or creates one if not present.
func ensureRWForNumber(ctx context.Context, tx *sql.Tx, rwNumber int) (*RW, error) {
	if rwNumber <= 0 {
		return nil, nil
	}
	rw, err := GetRWByNumber(ctx, tx, rwNumber)
	if err == nil {
		return rw, nil
	}
	if !errors.Is(err, ErrRWNotFound) {
		return nil, err
	}
	return CreateRW(ctx, tx, CreateRWInput{
		RWNumber: rwNumber,
		Name:     fmt.Sprintf("RW %03d", rwNumber),
	})
}

// Create inserts a new RT and returns it with generated fields.
func Create(ctx context.Context, tx *sql.Tx, in CreateRTInput) (*RT, error) {
	var resolvedRWID *string
	var resolvedRWName *string
	resolvedRWNum := in.RW

	if in.RWID != nil && strings.TrimSpace(*in.RWID) != "" {
		rwID := strings.TrimSpace(*in.RWID)
		rwObj, err := GetRWByID(ctx, tx, rwID)
		if err != nil {
			return nil, err
		}
		resolvedRWID = &rwObj.ID
		resolvedRWName = &rwObj.Name
		resolvedRWNum = rwObj.RWNumber
	} else if in.RW > 0 {
		rwObj, err := ensureRWForNumber(ctx, tx, in.RW)
		if err != nil {
			return nil, err
		}
		if rwObj != nil {
			resolvedRWID = &rwObj.ID
			resolvedRWName = &rwObj.Name
		}
	}

	var item RT
	err := tx.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw_id, rw, rt, address, head_name)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, rw_id, name, rw, rt, address, head_name, is_active, created_at, updated_at`,
		in.Name, resolvedRWID, resolvedRWNum, in.RT, in.Address, in.HeadName,
	).Scan(
		&item.ID, &item.RWID, &item.Name, &item.RW, &item.RT,
		&item.Address, &item.HeadName, &item.IsActive,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if isPGUniqueViolation(err) {
			return nil, fmt.Errorf("create rt: %w", ErrDuplicate)
		}
		return nil, fmt.Errorf("create rt: %w", err)
	}
	item.RWName = resolvedRWName
	return &item, nil
}

// GetByID finds an RT by UUID. Returns ErrNotFound when not found.
func GetByID(ctx context.Context, tx *sql.Tx, id string) (*RT, error) {
	var item RT
	err := tx.QueryRowContext(ctx,
		`SELECT r.id, r.rw_id, w.name AS rw_name, r.name, r.rw, r.rt, r.address, r.head_name, r.is_active, r.created_at, r.updated_at
		 FROM rts r
		 LEFT JOIN rws w ON w.id = r.rw_id
		 WHERE r.id = $1 LIMIT 1`,
		id,
	).Scan(
		&item.ID, &item.RWID, &item.RWName, &item.Name, &item.RW, &item.RT,
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
	return ListAllWithFilter(ctx, tx, isActive, search, nil, offset, limit)
}

// ListAllWithFilter returns RTs with optional active filter, search, rw_id filter, and pagination.
func ListAllWithFilter(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, rwID *string, offset, limit int) ([]*RT, error) {
	query := `SELECT r.id, r.rw_id, w.name AS rw_name, r.name, r.rw, r.rt, r.address, r.head_name, r.is_active, r.created_at, r.updated_at
		FROM rts r
		LEFT JOIN rws w ON w.id = r.rw_id`

	var args []interface{}
	argIndex := 1
	where := make([]string, 0, 3)

	if isActive != nil {
		where = append(where, fmt.Sprintf("r.is_active = $%d", argIndex))
		args = append(args, *isActive)
		argIndex++
	}
	if rwID != nil && strings.TrimSpace(*rwID) != "" {
		where = append(where, fmt.Sprintf("r.rw_id = $%d", argIndex))
		args = append(args, strings.TrimSpace(*rwID))
		argIndex++
	}
	if search != nil && strings.TrimSpace(*search) != "" {
		term := "%" + strings.ToLower(*search) + "%"
		where = append(where, "(LOWER(r.name) LIKE $"+fmt.Sprint(argIndex)+" OR LOWER(r.rt) LIKE $"+fmt.Sprint(argIndex)+" OR LOWER(r.head_name) LIKE $"+fmt.Sprint(argIndex)+")")
		args = append(args, term)
		argIndex++
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY r.created_at ASC OFFSET $%d LIMIT $%d", argIndex, argIndex+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list rts: %w", err)
	}
	defer rows.Close()

	result := make([]*RT, 0)
	for rows.Next() {
		var item RT
		if err := rows.Scan(
			&item.ID, &item.RWID, &item.RWName, &item.Name, &item.RW, &item.RT,
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
	return CountWithFilter(ctx, tx, isActive, search, nil)
}

// CountWithFilter returns the number of RTs matching the active filter, search, and rw_id.
func CountWithFilter(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, rwID *string) (int, error) {
	query := `SELECT COUNT(*) FROM rts r`

	var args []interface{}
	argIndex := 1
	where := make([]string, 0, 3)

	if isActive != nil {
		where = append(where, fmt.Sprintf("r.is_active = $%d", argIndex))
		args = append(args, *isActive)
		argIndex++
	}
	if rwID != nil && strings.TrimSpace(*rwID) != "" {
		where = append(where, fmt.Sprintf("r.rw_id = $%d", argIndex))
		args = append(args, strings.TrimSpace(*rwID))
		argIndex++
	}
	if search != nil && strings.TrimSpace(*search) != "" {
		term := "%" + strings.ToLower(*search) + "%"
		where = append(where, fmt.Sprintf("(LOWER(r.name) LIKE $%d OR LOWER(r.rt) LIKE $%d OR LOWER(r.head_name) LIKE $%d)", argIndex, argIndex, argIndex))
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
	if in.RWID != nil && strings.TrimSpace(*in.RWID) != "" {
		rwObj, err := GetRWByID(ctx, tx, strings.TrimSpace(*in.RWID))
		if err != nil {
			return nil, err
		}
		fields = append(fields, updateField{"rw_id", rwObj.ID})
		args = append(args, rwObj.ID)
		argIndex++
		fields = append(fields, updateField{"rw", rwObj.RWNumber})
		args = append(args, rwObj.RWNumber)
		argIndex++
	} else if in.RW != nil {
		fields = append(fields, updateField{"rw", *in.RW})
		args = append(args, *in.RW)
		argIndex++
		if *in.RW > 0 {
			rwObj, err := ensureRWForNumber(ctx, tx, *in.RW)
			if err != nil {
				return nil, err
			}
			if rwObj != nil {
				fields = append(fields, updateField{"rw_id", rwObj.ID})
				args = append(args, rwObj.ID)
				argIndex++
			}
		}
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

	var updatedID string
	err := tx.QueryRowContext(ctx,
		`UPDATE rts SET `+setClause(fields, 0)+
			`, updated_at = now() WHERE id = $`+fmt.Sprint(len(fields)+1)+
			` RETURNING id`,
		args...,
	).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isPGUniqueViolation(err) {
			return nil, fmt.Errorf("update rt: %w", ErrDuplicate)
		}
		return nil, fmt.Errorf("update rt: %w", err)
	}
	return GetByID(ctx, tx, updatedID)
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

// CreateRW inserts a new RW and returns it.
func CreateRW(ctx context.Context, tx *sql.Tx, in CreateRWInput) (*RW, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = fmt.Sprintf("RW %03d", in.RWNumber)
	}
	var item RW
	err := tx.QueryRowContext(ctx,
		`INSERT INTO rws (rw_number, name, head_name, address)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, rw_number, name, head_name, address, is_active, created_at, updated_at`,
		in.RWNumber, name, in.HeadName, in.Address,
	).Scan(
		&item.ID, &item.RWNumber, &item.Name, &item.HeadName,
		&item.Address, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if isPGUniqueViolation(err) {
			return nil, fmt.Errorf("create rw: %w", ErrRWDuplicate)
		}
		return nil, fmt.Errorf("create rw: %w", err)
	}
	return &item, nil
}

// GetRWByID finds an RW by UUID. Returns ErrRWNotFound when not found.
func GetRWByID(ctx context.Context, tx *sql.Tx, id string) (*RW, error) {
	var item RW
	err := tx.QueryRowContext(ctx,
		`SELECT w.id, w.rw_number, w.name, w.head_name, w.address,
		        (SELECT COUNT(*) FROM rts r WHERE r.rw_id = w.id AND r.is_active = true) AS rt_count,
		        w.is_active, w.created_at, w.updated_at
		 FROM rws w
		 WHERE w.id = $1 LIMIT 1`,
		id,
	).Scan(
		&item.ID, &item.RWNumber, &item.Name, &item.HeadName, &item.Address,
		&item.RTCount, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRWNotFound
		}
		return nil, fmt.Errorf("get rw by id: %w", err)
	}
	return &item, nil
}

// GetRWByNumber finds an active RW by rw_number.
func GetRWByNumber(ctx context.Context, tx *sql.Tx, rwNumber int) (*RW, error) {
	var item RW
	err := tx.QueryRowContext(ctx,
		`SELECT w.id, w.rw_number, w.name, w.head_name, w.address,
		        (SELECT COUNT(*) FROM rts r WHERE r.rw_id = w.id AND r.is_active = true) AS rt_count,
		        w.is_active, w.created_at, w.updated_at
		 FROM rws w
		 WHERE w.rw_number = $1 AND w.is_active = true
		 LIMIT 1`,
		rwNumber,
	).Scan(
		&item.ID, &item.RWNumber, &item.Name, &item.HeadName, &item.Address,
		&item.RTCount, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRWNotFound
		}
		return nil, fmt.Errorf("get rw by number: %w", err)
	}
	return &item, nil
}

// ListAllRWs returns RWs with optional active filter, search, and pagination.
func ListAllRWs(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, offset, limit int) ([]*RW, error) {
	query := `SELECT w.id, w.rw_number, w.name, w.head_name, w.address,
	                 (SELECT COUNT(*) FROM rts r WHERE r.rw_id = w.id AND r.is_active = true) AS rt_count,
	                 w.is_active, w.created_at, w.updated_at
	          FROM rws w`

	var args []interface{}
	argIndex := 1
	where := make([]string, 0, 2)

	if isActive != nil {
		where = append(where, fmt.Sprintf("w.is_active = $%d", argIndex))
		args = append(args, *isActive)
		argIndex++
	}
	if search != nil && strings.TrimSpace(*search) != "" {
		term := "%" + strings.ToLower(*search) + "%"
		where = append(where, fmt.Sprintf("(LOWER(w.name) LIKE $%d OR LOWER(COALESCE(w.head_name, '')) LIKE $%d OR CAST(w.rw_number AS text) LIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, term)
		argIndex++
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY w.rw_number ASC, w.created_at ASC OFFSET $%d LIMIT $%d", argIndex, argIndex+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list rws: %w", err)
	}
	defer rows.Close()

	result := make([]*RW, 0)
	for rows.Next() {
		var item RW
		if err := rows.Scan(
			&item.ID, &item.RWNumber, &item.Name, &item.HeadName, &item.Address,
			&item.RTCount, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan rw: %w", err)
		}
		result = append(result, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rws: %w", err)
	}
	return result, nil
}

// CountRWs returns the number of RWs matching the active filter and search.
func CountRWs(ctx context.Context, tx *sql.Tx, isActive *bool, search *string) (int, error) {
	query := `SELECT COUNT(*) FROM rws w`

	var args []interface{}
	argIndex := 1
	where := make([]string, 0, 2)

	if isActive != nil {
		where = append(where, fmt.Sprintf("w.is_active = $%d", argIndex))
		args = append(args, *isActive)
		argIndex++
	}
	if search != nil && strings.TrimSpace(*search) != "" {
		term := "%" + strings.ToLower(*search) + "%"
		where = append(where, fmt.Sprintf("(LOWER(w.name) LIKE $%d OR LOWER(COALESCE(w.head_name, '')) LIKE $%d OR CAST(w.rw_number AS text) LIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, term)
		argIndex++
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count rws: %w", err)
	}
	return count, nil
}

// UpdateRW partially updates an RW.
func UpdateRW(ctx context.Context, tx *sql.Tx, id string, in *UpdateRWInput) (*RW, error) {
	var fields []updateField
	var args []interface{}

	if in.RWNumber != nil {
		fields = append(fields, updateField{"rw_number", *in.RWNumber})
		args = append(args, *in.RWNumber)
	}
	if in.Name != nil {
		fields = append(fields, updateField{"name", *in.Name})
		args = append(args, *in.Name)
	}
	if in.HeadName != nil {
		fields = append(fields, updateField{"head_name", *in.HeadName})
		args = append(args, *in.HeadName)
	}
	if in.Address != nil {
		fields = append(fields, updateField{"address", *in.Address})
		args = append(args, *in.Address)
	}
	if in.IsActive != nil {
		if !*in.IsActive {
			var activeRTs int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rts WHERE rw_id = $1 AND is_active = true`, id).Scan(&activeRTs); err != nil {
				return nil, fmt.Errorf("check active rts for rw: %w", err)
			}
			if activeRTs > 0 {
				return nil, ErrRWHasActiveRTs
			}
		}
		fields = append(fields, updateField{"is_active", *in.IsActive})
		args = append(args, *in.IsActive)
	}

	if len(fields) == 0 {
		return nil, ErrRWNotFound
	}

	args = append(args, id)
	var updatedID string
	err := tx.QueryRowContext(ctx,
		`UPDATE rws SET `+setClause(fields, 0)+
			`, updated_at = now() WHERE id = $`+fmt.Sprint(len(fields)+1)+
			` RETURNING id`,
		args...,
	).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRWNotFound
		}
		if isPGUniqueViolation(err) {
			return nil, fmt.Errorf("update rw: %w", ErrRWDuplicate)
		}
		return nil, fmt.Errorf("update rw: %w", err)
	}

	if in.RWNumber != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE rts SET rw = $1, updated_at = now() WHERE rw_id = $2`, *in.RWNumber, id); err != nil {
			if isPGUniqueViolation(err) {
				return nil, fmt.Errorf("sync rts rw number: %w", ErrDuplicate)
			}
			return nil, fmt.Errorf("sync rts rw number: %w", err)
		}
	}

	return GetRWByID(ctx, tx, updatedID)
}

// DeactivateRW soft-deletes an RW if no active RTs are attached.
func DeactivateRW(ctx context.Context, tx *sql.Tx, id string) error {
	var activeRTs int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rts WHERE rw_id = $1 AND is_active = true`, id).Scan(&activeRTs); err != nil {
		return fmt.Errorf("check active rts for rw: %w", err)
	}
	if activeRTs > 0 {
		return ErrRWHasActiveRTs
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE rws SET is_active = false, updated_at = now()
		 WHERE id = $1 AND is_active = true`,
		id,
	)
	if err != nil {
		return fmt.Errorf("deactivate rw: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deactivate rw rows: %w", err)
	}
	if rows == 0 {
		return ErrRWNotFound
	}
	return nil
}
