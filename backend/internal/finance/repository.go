package finance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// --- Financial Categories Repository ---

func CategoryCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateCategoryInput) (*FinancialCategory, error) {
	var c FinancialCategory
	err := tx.QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type, is_active)
		 VALUES ($1, $2, $3, true)
		 RETURNING id, rt_id, name, type, is_active, created_at, updated_at`,
		rtID, in.Name, in.Type,
	).Scan(&c.ID, &c.RTID, &c.Name, &c.Type, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create category: %w", err)
	}
	return &c, nil
}

func CategoryGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*FinancialCategory, error) {
	var c FinancialCategory
	err := tx.QueryRowContext(ctx,
		`SELECT id, rt_id, name, type, is_active, created_at, updated_at
		 FROM financial_categories
		 WHERE id = $1 AND rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&c.ID, &c.RTID, &c.Name, &c.Type, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get category by id: %w", err)
	}
	return &c, nil
}

func CategoryList(ctx context.Context, tx *sql.Tx, rtID string, isActive *bool) ([]*FinancialCategory, error) {
	query := `SELECT id, rt_id, name, type, is_active, created_at, updated_at
	          FROM financial_categories WHERE rt_id = $1`
	var args []interface{}
	args = append(args, rtID)
	if isActive != nil {
		query += " AND is_active = $2"
		args = append(args, *isActive)
	}
	query += " ORDER BY name ASC"

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var list []*FinancialCategory
	for rows.Next() {
		var c FinancialCategory
		if err := rows.Scan(&c.ID, &c.RTID, &c.Name, &c.Type, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		list = append(list, &c)
	}
	return list, nil
}

func CategoryUpdate(ctx context.Context, tx *sql.Tx, id, rtID string, in UpdateCategoryInput) (*FinancialCategory, error) {
	existing, err := CategoryGetByID(ctx, tx, id, rtID)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if in.Name != nil && *in.Name != "" {
		name = *in.Name
	}
	isActive := existing.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	var c FinancialCategory
	err = tx.QueryRowContext(ctx,
		`UPDATE financial_categories
		 SET name = $1, is_active = $2, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $3 AND rt_id = $4
		 RETURNING id, rt_id, name, type, is_active, created_at, updated_at`,
		name, isActive, id, rtID,
	).Scan(&c.ID, &c.RTID, &c.Name, &c.Type, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update category: %w", err)
	}
	return &c, nil
}

func CategoryDeactivate(ctx context.Context, tx *sql.Tx, id, rtID string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE financial_categories SET is_active = false, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND rt_id = $2 AND is_active = true`,
		id, rtID,
	)
	if err != nil {
		return fmt.Errorf("deactivate category: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deactivate category rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Dues Repository ---

func DueCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateDueInput) (*Due, error) {
	var d Due
	var validUntil sql.NullString
	var year sql.NullInt64
	dueType := in.DueType
	if dueType == "" {
		if in.IsVoluntary {
			dueType = DueTypeVoluntary
		} else {
			dueType = DueTypeRegular
		}
	}
	err := tx.QueryRowContext(ctx,
		`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, due_type, year, is_active, valid_until)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, $9)
		 RETURNING id, rt_id, (SELECT rts.rt FROM rts WHERE rts.id = $1) as rt_code, name, amount::text, period_type, payment_rule, is_voluntary, due_type, year, is_active, to_char(valid_until, 'YYYY-MM-DD'), created_at, updated_at`,
		rtID, in.Name, in.Amount, in.PeriodType, in.PaymentRule, in.IsVoluntary, dueType, in.Year, in.ValidUntil,
	).Scan(&d.ID, &d.RTID, &d.RTCode, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.DueType, &year, &d.IsActive, &validUntil, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create dues: %w", err)
	}
	if year.Valid {
		y := int(year.Int64)
		d.Year = &y
	}
	setDueValidUntil(&d, validUntil)
	return &d, nil
}

func DueGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Due, error) {
	var d Due
	var validUntil sql.NullString
	var year sql.NullInt64
	err := tx.QueryRowContext(ctx,
		`SELECT id, rt_id, (SELECT rts.rt FROM rts WHERE rts.id = $2) as rt_code, name, amount::text, period_type, payment_rule, is_voluntary, due_type, year, is_active, to_char(valid_until, 'YYYY-MM-DD'), created_at, updated_at
		 FROM dues WHERE id = $1 AND rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&d.ID, &d.RTID, &d.RTCode, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.DueType, &year, &d.IsActive, &validUntil, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get due by id: %w", err)
	}
	if year.Valid {
		y := int(year.Int64)
		d.Year = &y
	}
	setDueValidUntil(&d, validUntil)
	return &d, nil
}

func DueGetByIDForUpdate(ctx context.Context, tx *sql.Tx, id, rtID string) (*Due, error) {
	var d Due
	var validUntil sql.NullString
	var year sql.NullInt64
	err := tx.QueryRowContext(ctx,
		`SELECT id, rt_id, (SELECT rts.rt FROM rts WHERE rts.id = $2) as rt_code, name, amount::text, period_type, payment_rule, is_voluntary, due_type, year, is_active, to_char(valid_until, 'YYYY-MM-DD'), created_at, updated_at
		 FROM dues WHERE id = $1 AND rt_id = $2 LIMIT 1 FOR UPDATE`,
		id, rtID,
	).Scan(&d.ID, &d.RTID, &d.RTCode, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.DueType, &year, &d.IsActive, &validUntil, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get due by id for update: %w", err)
	}
	if year.Valid {
		y := int(year.Int64)
		d.Year = &y
	}
	setDueValidUntil(&d, validUntil)
	return &d, nil
}

func DueList(ctx context.Context, tx *sql.Tx, rtID string, isActive *bool) ([]*Due, error) {
	query := `SELECT d.id, d.rt_id, rts.rt as rt_code, d.name, d.amount::text, d.period_type, d.payment_rule, d.is_voluntary, d.due_type, d.year, d.is_active, to_char(d.valid_until, 'YYYY-MM-DD'),
			COALESCE(ARRAY_AGG(DISTINCT b.period) FILTER (WHERE b.period IS NOT NULL), '{}') as periods,
			d.created_at, d.updated_at
		  FROM dues d JOIN rts ON rts.id = d.rt_id
		  LEFT JOIN bills b ON b.due_id = d.id AND b.rt_id = d.rt_id
		  WHERE d.rt_id = $1`
	var args []interface{}
	args = append(args, rtID)
	pos := 2
	if isActive != nil {
		query += fmt.Sprintf(" AND d.is_active = $%d", pos)
		args = append(args, *isActive)
		pos++
	}
	query += " GROUP BY d.id, rts.rt ORDER BY d.name ASC"

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dues: %w", err)
	}
	defer rows.Close()

	var list []*Due
	for rows.Next() {
		var d Due
		var validUntil sql.NullString
		var year sql.NullInt64
		var periodStr string
		if err := rows.Scan(&d.ID, &d.RTID, &d.RTCode, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.DueType, &year, &d.IsActive, &validUntil, &periodStr, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan due: %w", err)
		}
		if year.Valid {
			y := int(year.Int64)
			d.Year = &y
		}
		setDueValidUntil(&d, validUntil)
		d.Periods = parsePGArray(periodStr)
		list = append(list, &d)
	}
	return list, nil
}

// parsePGArray parses a PostgreSQL array string like '{"2026-01","2026-02"}' into a Go []string.
// Returns nil for empty/NULL arrays ('{}').
func parsePGArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "{}" || s == "" {
		return nil
	}
	// Remove outer braces
	if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return nil
	}
	// Split by comma, handling quoted elements
	var result []string
	var current strings.Builder
	inQuotes := false
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch == '"' {
			if inQuotes && i+1 < len(s) && s[i+1] == '"' {
				// escaped quote
				current.WriteByte('"')
				i += 2
				continue
			}
			inQuotes = !inQuotes
		} else if ch == ',' && !inQuotes {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
		i++
	}
	// last element
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}
	// filter empty strings
	filtered := make([]string, 0, len(result))
	for _, v := range result {
		if v != "" {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func setDueValidUntil(d *Due, value sql.NullString) {
	if value.Valid {
		s := value.String
		d.ValidUntil = &s
	} else {
		d.ValidUntil = nil
	}
}

func DueUpdate(ctx context.Context, tx *sql.Tx, id, rtID string, in UpdateDueInput) (*Due, error) {
	existing, err := DueGetByID(ctx, tx, id, rtID)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if in.Name != nil && *in.Name != "" {
		name = *in.Name
	}
	amount := existing.Amount
	if in.Amount != nil && *in.Amount != "" {
		amount = *in.Amount
	}
	periodType := existing.PeriodType
	if in.PeriodType != nil {
		periodType = *in.PeriodType
	}
	paymentRule := existing.PaymentRule
	if in.PaymentRule != nil && *in.PaymentRule != "" {
		paymentRule = *in.PaymentRule
	}
	isVoluntary := existing.IsVoluntary
	if in.IsVoluntary != nil {
		isVoluntary = *in.IsVoluntary
	}
	isActive := existing.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	validUntil := existing.ValidUntil
	if in.ValidUntil != nil {
		trimmed := strings.TrimSpace(*in.ValidUntil)
		if trimmed == "" {
			validUntil = nil
		} else {
			validUntil = &trimmed
		}
	}
	// Regular dues never carry a validity deadline.
	if !isVoluntary {
		validUntil = nil
	}

	dueType := existing.DueType
	if in.DueType != nil && *in.DueType != "" {
		dueType = *in.DueType
	}
	year := existing.Year
	if in.Year != nil {
		year = in.Year
	}

	var d Due
	var storedValidUntil sql.NullString
	var storedYear sql.NullInt64
	err = tx.QueryRowContext(ctx,
		`UPDATE dues
		 SET name = $1, amount = $2, period_type = $3, payment_rule = $4, is_voluntary = $5, due_type = $6, year = $7, is_active = $8, valid_until = $9, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $10 AND rt_id = $11
		 RETURNING id, rt_id, name, amount::text, period_type, payment_rule, is_voluntary, due_type, year, is_active, to_char(valid_until, 'YYYY-MM-DD'), created_at, updated_at`,
		name, amount, periodType, paymentRule, isVoluntary, dueType, year, isActive, validUntil, id, rtID,
	).Scan(&d.ID, &d.RTID, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.DueType, &storedYear, &d.IsActive, &storedValidUntil, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update due: %w", err)
	}
	if storedYear.Valid {
		y := int(storedYear.Int64)
		d.Year = &y
	}
	setDueValidUntil(&d, storedValidUntil)
	return &d, nil
}

func DueDeactivate(ctx context.Context, tx *sql.Tx, id, rtID string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE dues SET is_active = false, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND rt_id = $2 AND is_active = true`,
		id, rtID,
	)
	if err != nil {
		return fmt.Errorf("deactivate due: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deactivate due rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Bills Repository ---

func BillCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateBillInput) (*Bill, error) {
	var b Bill
	var dueDate time.Time
	err := tx.QueryRowContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, 'unpaid', true)
		 ON CONFLICT (household_occupancy_id, due_id, period) DO NOTHING
		 RETURNING id, rt_id, (SELECT rts.rt FROM rts WHERE id = $1) as rt_code, household_occupancy_id, due_id, amount::text, period, due_date, status, is_active, created_at, updated_at`,
		rtID, in.HouseholdOccupancyID, in.DueID, in.Amount, in.Period, in.DueDate,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.IsActive, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("create bill: %w", err)
	}
	b.DueDate = dueDate.Format("2006-01-02")
	return &b, nil
}

func BillGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Bill, error) {
	var b Bill
	var dueDate time.Time
	var dueName, houseNum, headName, dueType sql.NullString
	var dueYear sql.NullInt32
	var dueActive bool

	err := tx.QueryRowContext(ctx,
		`SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.is_active, b.created_at, b.updated_at,
		        d.name as due_name, d.is_active as due_is_active, ph.house_number, h.head_name, d.due_type, d.year
		 FROM bills b
		 JOIN dues d ON b.due_id = d.id
		 JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
		 JOIN households h ON ho.household_id = h.id
		 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
		 LEFT JOIN rts rt ON b.rt_id = rt.id
		 WHERE b.id = $1 AND b.rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
		&dueName, &dueActive, &houseNum, &headName, &dueType, &dueYear)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get bill by id: %w", err)
	}
	b.DueDate = dueDate.Format("2006-01-02")
	if dueName.Valid {
		b.DueName = &dueName.String
	}
	b.DueIsActive = &dueActive
	if houseNum.Valid {
		b.HouseNumber = &houseNum.String
	}
	if headName.Valid {
		b.HeadName = &headName.String
	}
	if dueType.Valid {
		b.DueType = &dueType.String
		if dueType.String == string(DueTypeVoluntary) {
			b.IsVoluntary = true
		}
	}
	if dueYear.Valid {
		yr := int(dueYear.Int32)
		b.DueYear = &yr
	}
	return &b, nil
}

func BillList(ctx context.Context, tx *sql.Tx, rtID string, occupancyID, dueID, status, paymentStatus, period, search, dueName *string, isActive *bool, offset, limit int, order ...string) ([]*Bill, error) {
	query := `SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.is_active, b.created_at, b.updated_at,
	                 d.name as due_name, d.is_active as due_is_active, ph.house_number, h.head_name, d.is_voluntary,
	                 COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.bill_id = b.id AND p.status = 'APPROVED'), 0)::text as paid_amount,
	                 COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.bill_id = b.id AND p.status = 'PENDING'), 0)::text as pending_amount,
	                 d.due_type, d.year
	          FROM bills b
	          JOIN dues d ON b.due_id = d.id
	          JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
	          JOIN households h ON ho.household_id = h.id
	          LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
	          LEFT JOIN rts rt ON b.rt_id = rt.id
	          WHERE b.rt_id = $1`

	args := []interface{}{rtID}
	pos := 2

	if occupancyID != nil && *occupancyID != "" {
		query += fmt.Sprintf(" AND b.household_occupancy_id = $%d", pos)
		args = append(args, *occupancyID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		query += fmt.Sprintf(" AND b.due_id = $%d", pos)
		args = append(args, *dueID)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND b.status = $%d", pos)
		args = append(args, *status)
		pos++
	}
	if isActive != nil {
		query += fmt.Sprintf(" AND b.is_active = $%d", pos)
		args = append(args, *isActive)
		pos++
	}
	if paymentStatus != nil && *paymentStatus != "" {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM payments p WHERE p.bill_id = b.id AND p.rt_id = b.rt_id AND p.status = $%d)", pos)
		args = append(args, *paymentStatus)
		pos++
	}
	if period != nil && *period != "" {
		query += fmt.Sprintf(" AND b.period LIKE $%d || '%%'", pos)
		args = append(args, *period)
		pos++
	}
	if search != nil && *search != "" {
		query += fmt.Sprintf(" AND (d.name ILIKE $%d OR h.head_name ILIKE $%d OR ph.house_number ILIKE $%d)", pos, pos, pos)
		args = append(args, "%"+*search+"%")
		pos++
	}
	if dueName != nil && *dueName != "" {
		query += fmt.Sprintf(" AND d.name = $%d", pos)
		args = append(args, *dueName)
		pos++
	}

	orderType := ""
	if len(order) > 0 {
		orderType = order[0]
	}
	if orderType == "unpaid_first" {
		query += fmt.Sprintf(" ORDER BY CASE WHEN b.status = 'paid' THEN 1 ELSE 0 END ASC, CASE WHEN b.status != 'paid' THEN b.period END ASC, CASE WHEN b.status = 'paid' THEN b.period END DESC, b.created_at ASC, b.id ASC OFFSET $%d LIMIT $%d", pos, pos+1)
	} else if occupancyID != nil && *occupancyID != "" {
		query += fmt.Sprintf(" ORDER BY b.period ASC, b.created_at ASC, b.id ASC OFFSET $%d LIMIT $%d", pos, pos+1)
	} else {
		query += fmt.Sprintf(" ORDER BY b.created_at DESC, b.id ASC OFFSET $%d LIMIT $%d", pos, pos+1)
	}
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list bills: %w", err)
	}
	defer rows.Close()

	var list []*Bill
	for rows.Next() {
		var b Bill
		var dueDate time.Time
		var dueName, houseNum, headName, paidAmt, pendingAmt, dueType sql.NullString
		var dueYear sql.NullInt32
		var dueActive bool
		if err := rows.Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
			&dueName, &dueActive, &houseNum, &headName, &b.IsVoluntary, &paidAmt, &pendingAmt, &dueType, &dueYear); err != nil {
			return nil, fmt.Errorf("scan bill: %w", err)
		}
		b.DueDate = dueDate.Format("2006-01-02")
		if dueName.Valid {
			b.DueName = &dueName.String
		}
		b.DueIsActive = &dueActive
		if houseNum.Valid {
			b.HouseNumber = &houseNum.String
		}
		if headName.Valid {
			b.HeadName = &headName.String
		}
		if paidAmt.Valid {
			b.PaidAmount = &paidAmt.String
		}
		if pendingAmt.Valid {
			b.PendingAmount = &pendingAmt.String
		}
		if dueType.Valid {
			b.DueType = &dueType.String
			if dueType.String == string(DueTypeVoluntary) {
				b.IsVoluntary = true
			}
		}
		if dueYear.Valid {
			yr := int(dueYear.Int32)
			b.DueYear = &yr
		}
		list = append(list, &b)
	}
	return list, nil
}

func BillCount(ctx context.Context, tx *sql.Tx, rtID string, occupancyID, dueID, status, paymentStatus, period, search, dueName *string, isActive *bool) (int, error) {
	query := `SELECT COUNT(*) FROM bills b
JOIN dues d ON b.due_id = d.id
JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
JOIN households h ON ho.household_id = h.id
LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
WHERE b.rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if occupancyID != nil && *occupancyID != "" {
		query += fmt.Sprintf(" AND b.household_occupancy_id = $%d", pos)
		args = append(args, *occupancyID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		query += fmt.Sprintf(" AND b.due_id = $%d", pos)
		args = append(args, *dueID)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND b.status = $%d", pos)
		args = append(args, *status)
		pos++
	}
	if isActive != nil {
		query += fmt.Sprintf(" AND b.is_active = $%d", pos)
		args = append(args, *isActive)
		pos++
	}
	if paymentStatus != nil && *paymentStatus != "" {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM payments p WHERE p.bill_id = b.id AND p.rt_id = b.rt_id AND p.status = $%d)", pos)
		args = append(args, *paymentStatus)
		pos++
	}
	if period != nil && *period != "" {
		query += fmt.Sprintf(" AND b.period LIKE $%d || '%%'", pos)
		args = append(args, *period)
		pos++
	}
	if search != nil && *search != "" {
		query += fmt.Sprintf(" AND (d.name ILIKE $%d OR h.head_name ILIKE $%d OR ph.house_number ILIKE $%d)", pos, pos, pos)
		args = append(args, "%"+*search+"%")
		pos++
	}
	if dueName != nil && *dueName != "" {
		query += fmt.Sprintf(" AND d.name = $%d", pos)
		args = append(args, *dueName)
		pos++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func BillUpdateStatus(ctx context.Context, tx *sql.Tx, id, rtID string, status BillStatus) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE bills SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND rt_id = $3`,
		status, id, rtID,
	)
	if err != nil {
		return fmt.Errorf("update bill status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update bill status rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// BillGetByIDForUpdate retrieves a bill and locks the row FOR UPDATE.
// Use this when the caller intends to modify the bill or compute derived values
// under a transaction (e.g., payment approval with overpayment validation).
func BillGetByIDForUpdate(ctx context.Context, tx *sql.Tx, id, rtID string) (*Bill, error) {
	var b Bill
	var dueDate time.Time

	err := tx.QueryRowContext(ctx,
		`SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.is_active, b.created_at, b.updated_at
		 FROM bills b
		 JOIN dues d ON b.due_id = d.id
		 JOIN rts rt ON b.rt_id = rt.id
		 WHERE b.id = $1 AND b.rt_id = $2 LIMIT 1 FOR UPDATE`,
		id, rtID,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.IsActive, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get bill by id for update: %w", err)
	}
	b.DueDate = dueDate.Format("2006-01-02")
	return &b, nil
}

func BillUpdateActiveStatus(ctx context.Context, tx *sql.Tx, id, rtID string, isActive bool) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE bills SET is_active = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND rt_id = $3`,
		isActive, id, rtID,
	)
	if err != nil {
		return fmt.Errorf("update bill active status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update bill active status rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func DueGetMonths(ctx context.Context, tx *sql.Tx, dueID, rtID string, year int) ([]DueMonthSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT b.period,
		        COUNT(b.id) as total_bills,
		        COUNT(CASE WHEN b.is_active THEN 1 END) as active_bills,
		        COUNT(CASE WHEN b.is_active AND b.status != 'paid' THEN 1 END) as active_unpaid_bills,
		        COUNT(CASE WHEN b.status = 'paid' THEN 1 END) as paid_bills,
		        COUNT(CASE WHEN b.status != 'paid' THEN 1 END) as unpaid_bills
		 FROM bills b
		 WHERE b.due_id = $1 AND b.rt_id = $2
		 GROUP BY b.period
		 ORDER BY b.period ASC`,
		dueID, rtID,
	)
	if err != nil {
		return nil, fmt.Errorf("query due months: %w", err)
	}
	defer rows.Close()

	monthMap := make(map[string]DueMonthSummary)
	for rows.Next() {
		var period string
		var total, active, activeUnpaid, paid, unpaid int
		if err := rows.Scan(&period, &total, &active, &activeUnpaid, &paid, &unpaid); err != nil {
			return nil, fmt.Errorf("scan due month: %w", err)
		}

		// A period's active status is determined by its unpaid bills.
		// - If there are unpaid bills, the period is active iff at least one unpaid bill is active (activeUnpaid > 0).
		// - If all bills in the period are paid (unpaid == 0 and total > 0), the period was fulfilled while active.
		// - If total == 0, defaults to true.
		isActive := true
		if total > 0 {
			if unpaid > 0 {
				isActive = activeUnpaid > 0
			} else {
				isActive = true
			}
		}

		activeBillsCount := activeUnpaid
		if isActive && unpaid == 0 {
			activeBillsCount = total
		} else if isActive {
			activeBillsCount = activeUnpaid + paid
		} else {
			activeBillsCount = 0
		}

		monthMap[period] = DueMonthSummary{
			Period:        period,
			TotalBills:    total,
			ActiveBills:   activeBillsCount,
			PaidBills:     paid,
			UnpaidBills:   unpaid,
			IsActive:      isActive,
			CanDeactivate: unpaid > 0 && activeUnpaid > 0,
		}
	}

	summaries := make([]DueMonthSummary, 12)
	for m := 1; m <= 12; m++ {
		period := fmt.Sprintf("%04d-%02d", year, m)
		if s, ok := monthMap[period]; ok {
			s.Month = m
			s.Year = year
			summaries[m-1] = s
		} else {
			summaries[m-1] = DueMonthSummary{
				Period:        period,
				Month:         m,
				Year:          year,
				TotalBills:    0,
				ActiveBills:   0,
				PaidBills:     0,
				UnpaidBills:   0,
				IsActive:      true,
				CanDeactivate: false,
			}
		}
	}
	return summaries, nil
}

func DueUpdatePeriodActiveStatus(ctx context.Context, tx *sql.Tx, dueID, rtID, period string, isActive bool) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE bills
		 SET is_active = $1, updated_at = CURRENT_TIMESTAMP
		 WHERE due_id = $2 AND rt_id = $3 AND period = $4 AND status != 'paid'`,
		isActive, dueID, rtID, period,
	)
	if err != nil {
		return 0, fmt.Errorf("update due period active status: %w", err)
	}
	return res.RowsAffected()
}

// GetApprovedPaymentTotal returns the total approved payment amount for a bill.
// A payment targets a bill either directly via payments.bill_id OR through
// payment_allocations (multi-period payments).
//
// For payments that have an allocation for this bill, the allocation amount
// is used (not the full payments.amount). For legacy single-bill payments
// without allocations, payments.amount is used.
//
// Returns "0" when no approved payments exist for this bill.
func GetApprovedPaymentTotal(ctx context.Context, tx *sql.Tx, billID string) (string, error) {
	var totalStr string
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0)::text FROM (
			-- Path 1: Single-bill payments (or fallback for multi-period without allocation)
			SELECT p.amount
			FROM payments p
			WHERE p.bill_id = $1 AND p.status = 'APPROVED'
			  AND NOT EXISTS (
				SELECT 1 FROM payment_allocations pa WHERE pa.payment_id = p.id AND pa.bill_id = $1
			  )
			UNION ALL
			-- Path 2: Multi-period allocations
			SELECT pa.amount
			FROM payment_allocations pa
			JOIN payments p ON p.id = pa.payment_id
			WHERE pa.bill_id = $1 AND p.status = 'APPROVED'
		) v`,
		billID,
	).Scan(&totalStr)
	if err != nil {
		return "0", fmt.Errorf("get approved payment total: %w", err)
	}
	return totalStr, nil
}

// GetApprovedPaymentCount returns the number of DISTINCT APPROVED payments visible for a bill.
// A payment is visible if it targets this bill directly via payments.bill_id OR
// through payment_allocations (multi-period payments).
// Returns 0 when no approved payments exist.
func GetApprovedPaymentCount(ctx context.Context, tx *sql.Tx, billID string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT payment_id) FROM (
			-- Path 1: Single-bill payments
			SELECT p.id AS payment_id
			FROM payments p
			WHERE p.bill_id = $1 AND p.status = 'APPROVED'
			  AND NOT EXISTS (
				SELECT 1 FROM payment_allocations pa WHERE pa.payment_id = p.id AND pa.bill_id = $1
			  )
			UNION
			-- Path 2: Multi-period allocations
			SELECT pa.payment_id
			FROM payment_allocations pa
			JOIN payments p ON p.id = pa.payment_id
			WHERE pa.bill_id = $1 AND p.status = 'APPROVED'
		) v`,
		billID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get approved payment count: %w", err)
	}
	return count, nil
}

// GetOldestUnpaidBillID returns the ID of the oldest unpaid bill for a given
// household occupancy and due_id, ordered by bill period ASC.
// A bill is considered unpaid when status != 'cancelled' and
// (bill amount - approved payments) > 0.
// Returns ErrNotFound when all bills for this (occID, due_id) are paid or cancelled.
func GetOldestUnpaidBillID(ctx context.Context, tx *sql.Tx, occID, dueID string) (string, error) {
	var billID string
	row := tx.QueryRowContext(ctx,
		`SELECT b.id FROM bills b
		 WHERE b.household_occupancy_id = $1 AND b.due_id = $2
		   AND b.status != 'cancelled'
		   AND b.is_active = true
		   AND (
			   SELECT COALESCE(SUM(v.amount), 0)
			   FROM (
					SELECT p.amount
					FROM payments p
					WHERE p.bill_id = b.id AND p.status = 'APPROVED'
					  AND NOT EXISTS (
						SELECT 1 FROM payment_allocations pa
						WHERE pa.payment_id = p.id AND pa.bill_id = b.id
					  )
					UNION ALL
					SELECT pa.amount
					FROM payment_allocations pa
					JOIN payments p ON p.id = pa.payment_id
					WHERE pa.bill_id = b.id AND p.status = 'APPROVED'
			   ) v
		   ) < b.amount
		 ORDER BY b.period ASC LIMIT 1`,
		occID, dueID,
	)
	if err := row.Scan(&billID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("scan oldest unpaid bill: %w", err)
	}
	return billID, nil
}

// GetOldestUnpaidBillIDs returns the IDs of the oldest unpaid bills up to limit for a given
// household occupancy and due_id, ordered by bill period ASC.
func GetOldestUnpaidBillIDs(ctx context.Context, tx *sql.Tx, occID, dueID string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT b.id FROM bills b
		 WHERE b.household_occupancy_id = $1 AND b.due_id = $2
		   AND b.status != 'cancelled'
		   AND b.is_active = true
		   AND (
			   SELECT COALESCE(SUM(v.amount), 0)
			   FROM (
					SELECT p.amount
					FROM payments p
					WHERE p.bill_id = b.id AND p.status = 'APPROVED'
					  AND NOT EXISTS (
						SELECT 1 FROM payment_allocations pa
						WHERE pa.payment_id = p.id AND pa.bill_id = b.id
					  )
					UNION ALL
					SELECT pa.amount
					FROM payment_allocations pa
					JOIN payments p ON p.id = pa.payment_id
					WHERE pa.bill_id = b.id AND p.status = 'APPROVED'
			   ) v
		   ) < b.amount
		 ORDER BY b.period ASC LIMIT $3`,
		occID, dueID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query oldest unpaid bills: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan oldest unpaid bill: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oldest unpaid bills: %w", err)
	}
	if len(ids) == 0 {
		return nil, ErrNotFound
	}
	return ids, nil
}

// GetPendingPaymentTotal returns the total pending payment amount for a bill.
// A payment targets a bill either directly via payments.bill_id OR through
// payment_allocations (multi-period payments).
//
// For payments that have an allocation for this bill, the allocation amount
// is used (not the full payments.amount). For legacy single-bill payments
// without allocations, payments.amount is used.
//
// Returns "0" when no pending payments exist for this bill.
func GetPendingPaymentTotal(ctx context.Context, tx *sql.Tx, billID string) (string, error) {
	var totalStr string
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0)::text FROM (
			-- Path 1: Single-bill payments (or fallback for multi-period without allocation)
			SELECT p.amount
			FROM payments p
			WHERE p.bill_id = $1 AND p.status = 'PENDING'
			  AND NOT EXISTS (
				SELECT 1 FROM payment_allocations pa WHERE pa.payment_id = p.id AND pa.bill_id = $1
			  )
			UNION ALL
			-- Path 2: Multi-period allocations
			SELECT pa.amount
			FROM payment_allocations pa
			JOIN payments p ON p.id = pa.payment_id
			WHERE pa.bill_id = $1 AND p.status = 'PENDING'
		) v`,
		billID,
	).Scan(&totalStr)
	if err != nil {
		return "0", fmt.Errorf("get pending payment total: %w", err)
	}
	return totalStr, nil
}

// GetActiveCurrentOccupancyIDs returns all active current household occupancies for an RT.
func GetActiveCurrentOccupancyIDs(ctx context.Context, tx *sql.Tx, rtID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT ho.id
		 FROM household_occupancies ho
		 JOIN households h ON ho.household_id = h.id
		 WHERE h.rt_id = $1 AND h.is_active = true AND ho.end_date IS NULL`,
		rtID,
	)
	if err != nil {
		return nil, fmt.Errorf("get active occupancy ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan occupancy id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type OccupancyEligibility struct {
	ID        string
	StartDate *string
	EndDate   *string
}

func GetActiveOccupanciesWithDates(ctx context.Context, tx *sql.Tx, rtID string) ([]OccupancyEligibility, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT ho.id, to_char(ho.start_date, 'YYYY-MM-DD'), to_char(ho.end_date, 'YYYY-MM-DD')
		 FROM household_occupancies ho
		 JOIN households h ON ho.household_id = h.id
		 WHERE h.rt_id = $1 AND h.is_active = true AND ho.end_date IS NULL`,
		rtID,
	)
	if err != nil {
		return nil, fmt.Errorf("get active occupancies: %w", err)
	}
	defer rows.Close()

	var result []OccupancyEligibility
	for rows.Next() {
		var o OccupancyEligibility
		var start, end sql.NullString
		if err := rows.Scan(&o.ID, &start, &end); err != nil {
			return nil, fmt.Errorf("scan occupancy eligibility: %w", err)
		}
		if start.Valid {
			s := start.String
			o.StartDate = &s
		}
		if end.Valid {
			e := end.String
			o.EndDate = &e
		}
		result = append(result, o)
	}
	return result, nil
}

// GetOccupancyIDForUser resolves the current active household occupancy for a warga user.
func GetOccupancyIDForUser(ctx context.Context, tx *sql.Tx, rtID, userID string) (string, error) {
	var occupancyID string
	var residentID string
	err := tx.QueryRowContext(ctx,
		`SELECT ho.id, r.id
		 FROM users u
		 JOIN residents r ON (
			r.user_id = u.id
			OR (u.email IS NOT NULL AND u.email != '' AND lower(trim(r.email)) = lower(trim(u.email)))
			OR (u.phone IS NOT NULL AND u.phone != '' AND (
				r.phone = u.phone
				OR regexp_replace(r.phone, '^(\+62|62|0)', '') = regexp_replace(u.phone, '^(\+62|62|0)', '')
			))
		 ) AND r.rt_id = $1 AND r.is_active = true
		 JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
		 JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id AND ho.end_date IS NULL
		 WHERE u.id = $2
		 ORDER BY CASE WHEN r.user_id = u.id THEN 0 ELSE 1 END, ho.created_at DESC
		 LIMIT 1`,
		rtID, userID,
	).Scan(&occupancyID, &residentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Fallback: user tidak terdaftar sebagai resident (misalnya staff/jabatan).
			// Coba cari via phone number: users.phone → households.phone → household_occupancies.
			var userPhone string
			err2 := tx.QueryRowContext(ctx,
				`SELECT phone FROM users WHERE id = $1`, userID,
			).Scan(&userPhone)
			if err2 == nil && userPhone != "" {
				normalizedPhone := regexp.MustCompile(`^(\+62|62|0)`).ReplaceAllString(userPhone, "")
				err3 := tx.QueryRowContext(ctx,
					`SELECT ho.id
					 FROM households h
					 JOIN household_occupancies ho ON ho.household_id = h.id AND ho.end_date IS NULL
					 WHERE h.rt_id = $1 AND h.is_active = true AND h.phone IS NOT NULL
					 ORDER BY CASE WHEN regexp_replace(h.phone, '^(\+62|62|0)', '') = $2 THEN 0 ELSE 1 END
					 LIMIT 1`,
					rtID, normalizedPhone,
				).Scan(&occupancyID)
				if err3 != nil && !errors.Is(err3, sql.ErrNoRows) {
					return "", fmt.Errorf("fallback lookup user occupancy: %w", err3)
				}
			}
			return "", nil // No occupancy tied to user
		}
		return "", fmt.Errorf("lookup user occupancy: %w", err)
	}

	// Backfill user_id on resident if not yet linked
	if residentID != "" {
		_, _ = tx.ExecContext(ctx, `UPDATE residents SET user_id = $1 WHERE id = $2 AND user_id IS NULL`, userID, residentID)
	}

	return occupancyID, nil
}

// --- Payments Repository ---

// paymentColumns contains only columns from the payments table (no JOINs).
// Use this for INSERT...RETURNING and UPDATE...RETURNING where CTEs handle the JOIN.
const paymentColumns = `id, rt_id, bill_id, due_id, submitted_by,
	amount::text, method, origin, status, proof_path, paid_at,
	verified_by, verified_at, verified_by_name, rejection_reason, notes, officer_notes, payment_no, created_at, updated_at,
	proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by, actual_received_date::text`

// paymentJoinedColumns includes enriched columns derived from LEFT JOINs
// on users.full_name (submitter_name) and bills.period.
// Only use with queries that have:
//
//	FROM payments p LEFT JOIN users u ON u.id = p.submitted_by
//	LEFT JOIN bills b ON b.id = p.bill_id
const paymentJoinedColumns = `p.id, p.rt_id, p.bill_id, p.due_id, p.submitted_by,
	COALESCE(NULLIF(u.full_name, ''), '—') AS submitter_name,
	p.amount::text, p.method, p.origin, p.status, p.proof_path, p.paid_at,
	p.verified_by, p.verified_at, p.verified_by_name, p.rejection_reason, p.notes, p.officer_notes, p.payment_no, p.created_at, p.updated_at,
	p.proof_original_name, p.proof_mime_type, p.proof_size_bytes, p.proof_uploaded_at, p.proof_uploaded_by, p.actual_received_date::text,
	b.period,
	COALESCE(t.location_id::text, '') AS location_id,
	COALESCE(fl.name, '') AS location_name`

type paymentScanner interface {
	Scan(dest ...any) error
}

func scanPayment(row paymentScanner) (*Payment, error) {
	var p Payment
	var actualRecDate sql.NullString
	err := row.Scan(&p.ID, &p.RTID, &p.BillID, &p.DueID, &p.SubmittedBy, &p.SubmitterName, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
		&p.VerifiedBy, &p.VerifiedAt, &p.VerifiedByName, &p.RejectionReason, &p.Notes, &p.OfficerNotes, &p.PaymentNo, &p.CreatedAt, &p.UpdatedAt,
		&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy, &actualRecDate)
	if err == nil && actualRecDate.Valid && actualRecDate.String != "" {
		p.ActualReceivedDate = &actualRecDate.String
	}
	return &p, err
}

func scanPaymentJoined(row paymentScanner) (*Payment, error) {
	var p Payment
	var actualRecDate sql.NullString
	var period sql.NullString
	var locID, locName string
	err := row.Scan(&p.ID, &p.RTID, &p.BillID, &p.DueID, &p.SubmittedBy, &p.SubmitterName, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
		&p.VerifiedBy, &p.VerifiedAt, &p.VerifiedByName, &p.RejectionReason, &p.Notes, &p.OfficerNotes, &p.PaymentNo, &p.CreatedAt, &p.UpdatedAt,
		&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy, &actualRecDate, &period, &locID, &locName)
	if err == nil && actualRecDate.Valid && actualRecDate.String != "" {
		p.ActualReceivedDate = &actualRecDate.String
	}
	if err == nil && period.Valid && period.String != "" {
		p.Period = &period.String
	}
	if err == nil && locID != "" {
		p.LocationID = &locID
	}
	if err == nil && locName != "" {
		p.LocationName = &locName
	}
	return &p, err
}

func PaymentCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreatePaymentInput, origin PaymentOrigin, status PaymentStatus, verifiedBy *string, verifiedAt *time.Time, actualReceivedDate ...*string) (*Payment, error) {
	var ard *string
	if len(actualReceivedDate) > 0 {
		ard = actualReceivedDate[0]
	}

	var uploadedAt *time.Time
	if in.ProofPath != nil {
		now := time.Now()
		uploadedAt = &now
	}

	p, err := scanPayment(tx.QueryRowContext(ctx,
		`WITH ins AS (
			INSERT INTO payments (rt_id, bill_id, due_id, submitted_by, amount, method, origin, status, proof_path, notes, officer_notes, verified_by, verified_at, verified_by_name, payment_no,
			                      proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by, actual_received_date)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
			 RETURNING `+paymentColumns+`
		)
		SELECT ins.id, ins.rt_id, ins.bill_id, ins.due_id, ins.submitted_by,
			COALESCE(NULLIF(u.full_name, ''), '—') AS submitter_name,
			ins.amount::text, ins.method, ins.origin, ins.status, ins.proof_path, ins.paid_at,
			ins.verified_by, ins.verified_at, ins.verified_by_name, ins.rejection_reason, ins.notes, ins.officer_notes, ins.payment_no, ins.created_at, ins.updated_at,
			ins.proof_original_name, ins.proof_mime_type, ins.proof_size_bytes, ins.proof_uploaded_at, ins.proof_uploaded_by, ins.actual_received_date::text
		FROM ins LEFT JOIN users u ON u.id = ins.submitted_by`,
		rtID, in.BillID, in.DueID, in.SubmittedBy, in.Amount, in.Method, origin, status, in.ProofPath, in.Notes, nil, verifiedBy, verifiedAt, nil, nil,
		in.ProofOriginalName, in.ProofMimeType, in.ProofSizeBytes, uploadedAt, in.ProofUploadedBy, ard,
	))
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return p, nil
}

func PaymentGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Payment, error) {
	p, err := scanPaymentJoined(tx.QueryRowContext(ctx,
		`SELECT `+paymentJoinedColumns+` FROM payments p LEFT JOIN users u ON u.id = p.submitted_by LEFT JOIN bills b ON b.id = p.bill_id LEFT JOIN transactions t ON t.payment_id = p.id LEFT JOIN fund_locations fl ON t.location_id = fl.id WHERE p.id = $1 AND p.rt_id = $2 LIMIT 1`,
		id, rtID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get payment by id: %w", err)
	}

	// Enrich: fund location from the KAS transaction created during approval
	// (income transaction, not reversed, for this payment)
	var locID, locName sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT fl.id::text, fl.name
		 FROM transactions t
		 LEFT JOIN fund_locations fl ON t.location_id = fl.id
		 WHERE t.payment_id = $1 AND t.type = 'income' AND t.reverses_transaction_id IS NULL
		 ORDER BY t.created_at DESC LIMIT 1`,
		id).Scan(&locID, &locName); err == nil {
		if locID.Valid && locID.String != "" {
			p.LocationID = &locID.String
		}
		if locName.Valid && locName.String != "" {
			p.LocationName = &locName.String
		}
	}

	return p, nil
}

// paymentForUpdateColumns is identical to paymentColumns but includes a scalar subquery
// for submitter_name. Using a scalar subquery (instead of LEFT JOIN) allows FOR UPDATE
// to be applied without the "nullable side of an outer join" error.
const paymentForUpdateColumns = `id, rt_id, bill_id, due_id, submitted_by,
	(SELECT COALESCE(NULLIF(u.full_name, ''), '—') FROM users u WHERE u.id = p.submitted_by) AS submitter_name,
	amount::text, method, origin, status, proof_path, paid_at,
	verified_by, verified_at, verified_by_name, rejection_reason, notes, officer_notes, payment_no, created_at, updated_at,
	proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by, actual_received_date::text`

func PaymentGetByIDForUpdate(ctx context.Context, tx *sql.Tx, id, rtID string) (*Payment, error) {
	p, err := scanPayment(tx.QueryRowContext(ctx,
		`SELECT `+paymentForUpdateColumns+` FROM payments p WHERE p.id = $1 AND p.rt_id = $2 FOR UPDATE`,
		id, rtID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get payment for update: %w", err)
	}
	return p, nil
}

func PaymentGetByIDForUser(ctx context.Context, tx *sql.Tx, id, rtID, userID string, occupancyID *string) (*Payment, error) {
	query := `SELECT ` + paymentJoinedColumns + `
	          FROM payments p LEFT JOIN users u ON u.id = p.submitted_by LEFT JOIN bills b ON b.id = p.bill_id LEFT JOIN transactions t ON t.payment_id = p.id LEFT JOIN fund_locations fl ON t.location_id = fl.id
	          WHERE p.id = $1 AND p.rt_id = $2
	          AND (p.submitted_by = $3`
	args := []interface{}{id, rtID, userID}
	if occupancyID != nil && *occupancyID != "" {
		query += ` OR EXISTS (
			SELECT 1 FROM bills b
			WHERE b.id = p.bill_id AND b.rt_id = p.rt_id AND b.household_occupancy_id = $4
		)`
		args = append(args, *occupancyID)
	}
	query += `)`
	p, err := scanPaymentJoined(tx.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get payment for user: %w", err)
	}

	// Enrich: fund location from the KAS transaction created during approval
	var locID, locName sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT fl.id::text, fl.name
		 FROM transactions t
		 LEFT JOIN fund_locations fl ON t.location_id = fl.id
		 WHERE t.payment_id = $1 AND t.type = 'income' AND t.reverses_transaction_id IS NULL
		 ORDER BY t.created_at DESC LIMIT 1`,
		id).Scan(&locID, &locName); err == nil {
		if locID.Valid && locID.String != "" {
			p.LocationID = &locID.String
		}
		if locName.Valid && locName.String != "" {
			p.LocationName = &locName.String
		}
	}

	return p, nil
}

func PaymentList(ctx context.Context, tx *sql.Tx, rtID string, billID, dueID, status, submittedBy, occupancyID *string, offset, limit int) ([]*Payment, error) {
	query := `SELECT ` + paymentJoinedColumns + `
	          FROM payments p LEFT JOIN users u ON u.id = p.submitted_by LEFT JOIN bills b ON b.id = p.bill_id LEFT JOIN transactions t ON t.payment_id = p.id LEFT JOIN fund_locations fl ON t.location_id = fl.id
	          WHERE p.rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND p.bill_id = $%d", pos)
		args = append(args, *billID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		query += fmt.Sprintf(" AND p.due_id = $%d", pos)
		args = append(args, *dueID)
		pos++
	}
	if submittedBy != nil {
		query += fmt.Sprintf(" AND (p.submitted_by = $%d", pos)
		args = append(args, *submittedBy)
		pos++
		if occupancyID != nil && *occupancyID != "" {
			query += fmt.Sprintf(" OR EXISTS (SELECT 1 FROM bills b WHERE b.id = p.bill_id AND b.rt_id = p.rt_id AND b.household_occupancy_id = $%d)", pos)
			args = append(args, *occupancyID)
			pos++
		}
		query += ")"
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND p.status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY p.created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	list := make([]*Payment, 0)
	for rows.Next() {
		p, err := scanPaymentJoined(rows)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		list = append(list, p)
	}
	return list, nil
}

func PaymentCount(ctx context.Context, tx *sql.Tx, rtID string, billID, dueID, status, submittedBy, occupancyID *string) (int, error) {
	query := `SELECT COUNT(*) FROM payments p WHERE p.rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND p.bill_id = $%d", pos)
		args = append(args, *billID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		query += fmt.Sprintf(" AND p.due_id = $%d", pos)
		args = append(args, *dueID)
		pos++
	}
	if submittedBy != nil {
		query += fmt.Sprintf(" AND (p.submitted_by = $%d", pos)
		args = append(args, *submittedBy)
		pos++
		if occupancyID != nil && *occupancyID != "" {
			query += fmt.Sprintf(" OR EXISTS (SELECT 1 FROM bills b WHERE b.id = p.bill_id AND b.rt_id = p.rt_id AND b.household_occupancy_id = $%d)", pos)
			args = append(args, *occupancyID)
			pos++
		}
		query += ")"
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND p.status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func PaymentVerify(ctx context.Context, tx *sql.Tx, id, rtID, verifierID string, status PaymentStatus, reason, officerNotes string, actualReceivedDate ...*string) (*Payment, error) {
	var ard *string
	if len(actualReceivedDate) > 0 {
		ard = actualReceivedDate[0]
	}

	var p Payment
	// 1. Lookup verifier name
	var verifierName string
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(NULLIF(u.full_name, ''), 'Unknown') FROM users u WHERE u.id = $1`,
		verifierID,
	).Scan(&verifierName)
	if err != nil {
		verifierName = "Unknown"
	}

	// 2. Generate payment_no if not yet set.
	// Lock the payment row FOR UPDATE first so that concurrent verifies
	// serialize on the same row, eliminating the TOCTOU race between
	// the COUNT(*) read and the UPDATE below.
	var existingNo string
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(payment_no, '') FROM payments WHERE id = $1 AND rt_id = $2 FOR UPDATE`,
		id, rtID,
	).Scan(&existingNo)
	if err == nil && existingNo == "" {
		dateStr := time.Now().Format("20060102")
		var seq int64
		err = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(CAST(SUBSTRING(payment_no FROM 14) AS INTEGER)), 0)
			 FROM payments
			 WHERE payment_no LIKE 'PAY-' || $1 || '-%'`,
			dateStr,
		).Scan(&seq)
		if err != nil {
			seq = 0
		}
		seq++
		v := fmt.Sprintf("PAY-%s-%05d", dateStr, seq)
		p.PaymentNo = &v
	} else if err == nil && existingNo != "" {
		v := existingNo
		p.PaymentNo = &v
	}

	// Convert empty strings to nil so that NULL is stored in the DB
	// (empty string "" is not the same as NULL in PostgreSQL)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	var officerNotesPtr *string
	if officerNotes != "" {
		officerNotesPtr = &officerNotes
	}

	verifiedPayment, err := scanPayment(tx.QueryRowContext(ctx,
		`WITH upd AS (
			UPDATE payments
			SET status = $1, verified_by = $2, verified_at = CURRENT_TIMESTAMP, rejection_reason = $3,
			    officer_notes = $4, verified_by_name = $5, payment_no = $6,
			    actual_received_date = COALESCE($7, actual_received_date),
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $8 AND rt_id = $9
			RETURNING `+paymentColumns+`
		)
		SELECT upd.id, upd.rt_id, upd.bill_id, upd.due_id, upd.submitted_by,
			COALESCE(NULLIF(u.full_name, ''), '—') AS submitter_name,
			upd.amount::text, upd.method, upd.origin, upd.status, upd.proof_path, upd.paid_at,
			upd.verified_by, upd.verified_at, upd.verified_by_name, upd.rejection_reason, upd.notes, upd.officer_notes, upd.payment_no, upd.created_at, upd.updated_at,
			upd.proof_original_name, upd.proof_mime_type, upd.proof_size_bytes, upd.proof_uploaded_at, upd.proof_uploaded_by, upd.actual_received_date::text
		FROM upd LEFT JOIN users u ON u.id = upd.submitted_by`,
		status, verifierID, reasonPtr, officerNotesPtr, verifierName, p.PaymentNo, ard, id, rtID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("verify payment: %w", err)
	}
	return verifiedPayment, nil
}

func PaymentAttachProof(ctx context.Context, tx *sql.Tx, paymentID, rtID, proofPath, originalName, mimeType string, size int64, uploadedBy string) (*Payment, error) {
	p, err := scanPayment(tx.QueryRowContext(ctx,
		`WITH upd AS (
			UPDATE payments
			SET proof_path = $1, proof_original_name = $2, proof_mime_type = $3, proof_size_bytes = $4,
			    proof_uploaded_at = CURRENT_TIMESTAMP, proof_uploaded_by = $5, updated_at = CURRENT_TIMESTAMP
			WHERE id = $6 AND rt_id = $7
			RETURNING `+paymentColumns+`
		)
		SELECT upd.id, upd.rt_id, upd.bill_id, upd.due_id, upd.submitted_by,
			COALESCE(NULLIF(u.full_name, ''), '—') AS submitter_name,
			upd.amount::text, upd.method, upd.origin, upd.status, upd.proof_path, upd.paid_at,
			upd.verified_by, upd.verified_at, upd.verified_by_name, upd.rejection_reason, upd.notes, upd.officer_notes, upd.payment_no, upd.created_at, upd.updated_at,
			upd.proof_original_name, upd.proof_mime_type, upd.proof_size_bytes, upd.proof_uploaded_at, upd.proof_uploaded_by, upd.actual_received_date::text
		FROM upd LEFT JOIN users u ON u.id = upd.submitted_by`,
		proofPath, originalName, mimeType, size, uploadedBy, paymentID, rtID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("attach proof to payment: %w", err)
	}
	return p, nil
}

// --- Transactions (Immutable Ledger) Repository ---

func TransactionCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateTransactionInput, status TransactionStatus, reversesID, paymentID, createdBy *string, occurredAt time.Time) (*Transaction, error) {
	var t Transaction
	var payMethod *string
	if in.PaymentMethod != nil && *in.PaymentMethod != "" {
		m := strings.ToUpper(strings.TrimSpace(*in.PaymentMethod))
		payMethod = &m
	}

	var refNo string
	if paymentID != nil && strings.TrimSpace(*paymentID) != "" && reversesID == nil {
		_ = tx.QueryRowContext(ctx,
			`SELECT COALESCE(payment_no, '') FROM payments WHERE id = $1`,
			*paymentID,
		).Scan(&refNo)
	}
	if refNo == "" {
		dateStr := time.Now().Format("20060102")
		_, _ = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('kas_ref_' || $1))`, dateStr)
		var seq int64
		_ = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(CAST(SUBSTRING(reference_no FROM 14) AS INTEGER)), 0)
			 FROM transactions
			 WHERE reference_no LIKE 'KAS-' || $1 || '-%'`,
			dateStr,
		).Scan(&seq)
		seq++
		refNo = fmt.Sprintf("KAS-%s-%05d", dateStr, seq)
	}

	var payMethodStr, createdByStr, refNoStr, locIDStr sql.NullString
	var locID *string
	if in.LocationID != nil && strings.TrimSpace(*in.LocationID) != "" {
		trimmed := strings.TrimSpace(*in.LocationID)
		locID = &trimmed
	}

	err := tx.QueryRowContext(ctx,
		`INSERT INTO transactions (rt_id, category_id, amount, type, status, reverses_transaction_id, payment_id, reference_no, description, occurred_at, payment_method, created_by, location_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, rt_id, category_id, amount::text, type, status, reverses_transaction_id, payment_id, reference_no, description, occurred_at, created_at, updated_at, payment_method, created_by::text, location_id::text`,
		rtID, in.CategoryID, in.Amount, in.Type, status, reversesID, paymentID, refNo, in.Description, occurredAt, payMethod, createdBy, locID,
	).Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status, &t.ReversesTransactionID, &t.PaymentID, &refNoStr, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt, &payMethodStr, &createdByStr, &locIDStr)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	if refNoStr.Valid && refNoStr.String != "" {
		t.ReferenceNo = &refNoStr.String
		t.PaymentNo = &refNoStr.String
	}
	if payMethodStr.Valid {
		t.PaymentMethod = &payMethodStr.String
	}
	if createdByStr.Valid {
		t.CreatedBy = &createdByStr.String
	}
	if locIDStr.Valid && locIDStr.String != "" {
		t.LocationID = &locIDStr.String
	}

	// Insert into location_balance_changes if location_id is provided and status is posted
	if locID != nil && status == TransactionStatusPosted {
		var changeType string
		var amtDelta string
		if reversesID != nil {
			if in.Type == TransactionTypeIncome {
				changeType = "reversal_expense"
				amtDelta = in.Amount
			} else {
				changeType = "reversal_income"
				amtDelta = "-" + in.Amount
			}
		} else {
			if in.Type == TransactionTypeIncome {
				changeType = "income"
				amtDelta = in.Amount
			} else {
				changeType = "expense"
				amtDelta = "-" + in.Amount
			}
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO location_balance_changes (rt_id, location_id, transaction_id, change_type, amount_delta, notes, created_by, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			rtID, *locID, t.ID, changeType, amtDelta, in.Description, createdBy, occurredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert location_balance_changes for transaction: %w", err)
		}
	}

	return &t, nil
}

func TransactionGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Transaction, error) {
	var t Transaction
	var catName, payNo, payMethod, createdBy, officerName, dueName, citizenName, houseNumber, locID, locName sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
		        t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
		        fc.name as category_name,
		        COALESCE(NULLIF(p.payment_no, ''), NULLIF(t.reference_no, '')) as payment_no,
		        COALESCE(NULLIF(t.payment_method, ''), p.method::text) as payment_method,
		        t.created_by::text,
		        COALESCE(u_creator.full_name, u_creator.email) as officer_name,
		        COALESCE(d.name, '') as due_name,
		        COALESCE(NULLIF(u_warga.full_name, ''), NULLIF(h.head_name, ''), 'Warga') as citizen_name,
		        COALESCE(ph.house_number, '') as house_number,
		        t.location_id::text,
		        COALESCE(fl.name, '') as location_name
		 FROM transactions t
		 JOIN financial_categories fc ON t.category_id = fc.id
		 LEFT JOIN users u_creator ON t.created_by = u_creator.id
		 LEFT JOIN payments p ON t.payment_id = p.id
		 LEFT JOIN bills b ON p.bill_id = b.id
		 LEFT JOIN dues d ON (p.due_id = d.id OR (p.due_id IS NULL AND b.due_id = d.id))
		 LEFT JOIN users u_warga ON p.submitted_by = u_warga.id
		 LEFT JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
		 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
		 LEFT JOIN households h ON ho.household_id = h.id
		 LEFT JOIN fund_locations fl ON t.location_id = fl.id
		 WHERE t.id = $1 AND t.rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
		&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
		&catName, &payNo, &payMethod, &createdBy, &officerName, &dueName, &citizenName, &houseNumber,
		&locID, &locName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get transaction by id: %w", err)
	}
	if catName.Valid {
		t.CategoryName = &catName.String
	}
	if payNo.Valid && payNo.String != "" {
		t.PaymentNo = &payNo.String
		t.ReferenceNo = &payNo.String
	}
	if payMethod.Valid {
		t.PaymentMethod = &payMethod.String
	}
	if createdBy.Valid {
		t.CreatedBy = &createdBy.String
	}
	if officerName.Valid && officerName.String != "" {
		t.OfficerName = &officerName.String
	}
	if dueName.Valid && dueName.String != "" {
		t.DueName = &dueName.String
	}
	if citizenName.Valid && citizenName.String != "" {
		t.CitizenName = &citizenName.String
	}
	if houseNumber.Valid && houseNumber.String != "" {
		t.HouseNumber = &houseNumber.String
	}
	if locID.Valid && locID.String != "" {
		t.LocationID = &locID.String
	}
	if locName.Valid && locName.String != "" {
		t.LocationName = &locName.String
	}
	return &t, nil
}

func TransactionList(ctx context.Context, tx *sql.Tx, rtID string, categoryID, transType, status, startDate, endDate *string, offset, limit int) ([]*Transaction, error) {
	query := `SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
	                 t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
	                 fc.name as category_name,
	                 COALESCE(NULLIF(p.payment_no, ''), NULLIF(t.reference_no, '')) as payment_no,
	                 COALESCE(NULLIF(t.payment_method, ''), p.method::text) as payment_method,
	                 t.created_by::text,
	                 COALESCE(u_creator.full_name, u_creator.email) as officer_name,
	                 COALESCE(d.name, '') as due_name,
	                 COALESCE(NULLIF(u_warga.full_name, ''), NULLIF(h.head_name, ''), 'Warga') as citizen_name,
	                 COALESCE(ph.house_number, '') as house_number,
	                 t.location_id::text,
	                 COALESCE(fl.name, '') as location_name
	          FROM transactions t
	          JOIN financial_categories fc ON t.category_id = fc.id
	          LEFT JOIN users u_creator ON t.created_by = u_creator.id
	          LEFT JOIN payments p ON t.payment_id = p.id
	          LEFT JOIN bills b ON p.bill_id = b.id
	          LEFT JOIN dues d ON (p.due_id = d.id OR (p.due_id IS NULL AND b.due_id = d.id))
	          LEFT JOIN users u_warga ON p.submitted_by = u_warga.id
	          LEFT JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
	          LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
	          LEFT JOIN households h ON ho.household_id = h.id
	          LEFT JOIN fund_locations fl ON t.location_id = fl.id
	          WHERE t.rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if categoryID != nil && *categoryID != "" {
		query += fmt.Sprintf(" AND t.category_id = $%d", pos)
		args = append(args, *categoryID)
		pos++
	}
	if transType != nil && *transType != "" {
		query += fmt.Sprintf(" AND t.type = $%d", pos)
		args = append(args, *transType)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND t.status = $%d", pos)
		args = append(args, *status)
		pos++
	}
	if startDate != nil && *startDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at >= $%d", pos)
		args = append(args, *startDate)
		pos++
	}
	if endDate != nil && *endDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at <= $%d", pos)
		args = append(args, *endDate)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY t.occurred_at DESC, t.created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var list []*Transaction
	for rows.Next() {
		var t Transaction
		var catName, payNo, payMethod, createdBy, officerName, dueName, citizenName, houseNumber, locID, locName sql.NullString
		if err := rows.Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
			&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
			&catName, &payNo, &payMethod, &createdBy, &officerName, &dueName, &citizenName, &houseNumber,
			&locID, &locName); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		if catName.Valid {
			t.CategoryName = &catName.String
		}
		if payNo.Valid && payNo.String != "" {
			t.PaymentNo = &payNo.String
			t.ReferenceNo = &payNo.String
		}
		if payMethod.Valid {
			t.PaymentMethod = &payMethod.String
		}
		if createdBy.Valid {
			t.CreatedBy = &createdBy.String
		}
		if officerName.Valid && officerName.String != "" {
			t.OfficerName = &officerName.String
		}
		if dueName.Valid && dueName.String != "" {
			t.DueName = &dueName.String
		}
		if citizenName.Valid && citizenName.String != "" {
			t.CitizenName = &citizenName.String
		}
		if houseNumber.Valid && houseNumber.String != "" {
			t.HouseNumber = &houseNumber.String
		}
		if locID.Valid && locID.String != "" {
			t.LocationID = &locID.String
		}
		if locName.Valid && locName.String != "" {
			t.LocationName = &locName.String
		}
		list = append(list, &t)
	}
	return list, nil
}

func TransactionCount(ctx context.Context, tx *sql.Tx, rtID string, categoryID, transType, status, startDate, endDate *string) (int, error) {
	query := `SELECT COUNT(*) FROM transactions t WHERE t.rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if categoryID != nil && *categoryID != "" {
		query += fmt.Sprintf(" AND t.category_id = $%d", pos)
		args = append(args, *categoryID)
		pos++
	}
	if transType != nil && *transType != "" {
		query += fmt.Sprintf(" AND t.type = $%d", pos)
		args = append(args, *transType)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND t.status = $%d", pos)
		args = append(args, *status)
		pos++
	}
	if startDate != nil && *startDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at >= $%d", pos)
		args = append(args, *startDate)
		pos++
	}
	if endDate != nil && *endDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at <= $%d", pos)
		args = append(args, *endDate)
		pos++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// TransactionCancelOnReversal marks the original transaction cancelled upon reversal.
func TransactionCancelOnReversal(ctx context.Context, tx *sql.Tx, id, rtID string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE transactions SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND rt_id = $2 AND status = 'posted'`,
		id, rtID,
	)
	if err != nil {
		return fmt.Errorf("cancel transaction on reversal: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("cancel transaction rows: %w", err)
	}
	if rows == 0 {
		return ErrCannotReverseNonPost
	}
	return nil
}

// CalculateBalance strictly derives current balance from ledger: Total Posted Income - Total Posted Expense.
// Zero floating-point types used.
func CalculateBalance(ctx context.Context, tx *sql.Tx, rtID string) (*BalanceSummary, error) {
	var incomeStr, expenseStr string
	var count int

	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text as total_income,
		        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text as total_expense,
		        COUNT(*) as tx_count
		 FROM transactions
		 WHERE rt_id = $1 AND status = 'posted'`,
		rtID,
	).Scan(&incomeStr, &expenseStr, &count)
	if err != nil {
		return nil, fmt.Errorf("derive balance: %w", err)
	}

	incCents, incCanon, err := ParseMoney(incomeStr)
	if err != nil {
		return nil, fmt.Errorf("parse income: %w", err)
	}
	expCents, expCanon, err := ParseMoney(expenseStr)
	if err != nil {
		return nil, fmt.Errorf("parse expense: %w", err)
	}

	netCents := incCents - expCents
	netCanon := FormatMoney(netCents)

	return &BalanceSummary{
		TotalIncome:      incCanon,
		TotalExpense:     expCanon,
		NetBalance:       netCanon,
		TransactionCount: count,
	}, nil
}

// FindOrCreateDefaultIncomeCategory ensures an active income category exists (e.g. for payments).
func FindOrCreateDefaultIncomeCategory(ctx context.Context, tx *sql.Tx, rtID, name string) (string, error) {
	var catID string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM financial_categories WHERE rt_id = $1 AND name = $2 LIMIT 1`,
		rtID, name,
	).Scan(&catID)
	if err == nil {
		return catID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("find income category: %w", err)
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO financial_categories (rt_id, name, type, is_active)
		 VALUES ($1, $2, 'income', true)
		 RETURNING id`,
		rtID, name,
	).Scan(&catID)
	if err != nil {
		return "", fmt.Errorf("insert default income category: %w", err)
	}
	return catID, nil
}

// --- Idempotency Repository ---

type CachedResponse struct {
	UserID       string
	RequestPath  string
	RequestHash  string
	ResponseCode int
	ResponseBody string
}

func IdempotencyGet(ctx context.Context, tx *sql.Tx, rtID, key string) (*CachedResponse, error) {
	var resp CachedResponse
	err := tx.QueryRowContext(ctx,
		`SELECT user_id::text, request_path, request_hash, response_code, response_body FROM idempotency_keys
		 WHERE rt_id = $1 AND key = $2 LIMIT 1`,
		rtID, key,
	).Scan(&resp.UserID, &resp.RequestPath, &resp.RequestHash, &resp.ResponseCode, &resp.ResponseBody)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}
	return &resp, nil
}

func IdempotencySave(ctx context.Context, tx *sql.Tx, rtID, key, userID, reqPath, reqHash string, code int, body string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO idempotency_keys (rt_id, key, user_id, request_path, request_hash, response_code, response_body)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		rtID, key, userID, reqPath, reqHash, code, body,
	)
	if err != nil {
		return fmt.Errorf("save idempotency key: %w", err)
	}
	return nil
}

// --- Audit Logs Repository ---

var AuditLogInsertHook func(context.Context, *sql.Tx, AuditLog) error

func AuditLogInsert(ctx context.Context, tx *sql.Tx, log AuditLog) error {
	if AuditLogInsertHook != nil {
		if err := AuditLogInsertHook(ctx, tx, log); err != nil {
			return err
		}
	}
	var uid *string
	if log.UserID != nil && *log.UserID != "" {
		var exists bool
		_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id::text = $1)`, *log.UserID).Scan(&exists)
		if exists {
			uid = log.UserID
		}
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO audit_logs (rt_id, user_id, action, entity_type, entity_id, old_values, new_values, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		log.RTID, uid, log.Action, log.EntityType, log.EntityID, log.OldValues, log.NewValues, log.IPAddress,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// ============================================================================
// System-wide (cross-RT) repository methods — SUPER_ADMIN only
// ============================================================================

// --- Categories ---

func CategoryListAll(ctx context.Context, tx *sql.Tx, isActive *bool) ([]*FinancialCategory, error) {
	query := `SELECT id, rt_id, name, type, is_active, created_at, updated_at
	          FROM financial_categories`
	var args []interface{}
	if isActive != nil {
		query += ` WHERE is_active = $1`
		args = append(args, *isActive)
	}
	query += ` ORDER BY rt_id, name ASC`

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all categories: %w", err)
	}
	defer rows.Close()

	var list []*FinancialCategory
	for rows.Next() {
		var c FinancialCategory
		if err := rows.Scan(&c.ID, &c.RTID, &c.Name, &c.Type, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		list = append(list, &c)
	}
	return list, nil
}

// --- Dues ---

func DueListAll(ctx context.Context, tx *sql.Tx, isActive *bool) ([]*Due, error) {
	query := `SELECT d.id, d.rt_id, rts.rt as rt_code, d.name, d.amount::text, d.period_type, d.payment_rule, d.is_voluntary, d.is_active, to_char(d.valid_until, 'YYYY-MM-DD'),
			COALESCE(ARRAY_AGG(DISTINCT b.period) FILTER (WHERE b.period IS NOT NULL), '{}') as periods,
			d.created_at, d.updated_at
		  FROM dues d JOIN rts ON rts.id = d.rt_id
		  LEFT JOIN bills b ON b.due_id = d.id AND b.rt_id = d.rt_id`
	var args []interface{}
	pos := 1
	if isActive != nil {
		query += fmt.Sprintf(" WHERE d.is_active = $%d", pos)
		args = append(args, *isActive)
		pos++
	}
	query += " GROUP BY d.id, rts.rt ORDER BY d.rt_id, d.name ASC"

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all dues: %w", err)
	}
	defer rows.Close()

	var list []*Due
	for rows.Next() {
		var d Due
		var validUntil sql.NullString
		var periodStr string
		if err := rows.Scan(&d.ID, &d.RTID, &d.RTCode, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.IsActive, &validUntil, &periodStr, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan due: %w", err)
		}
		setDueValidUntil(&d, validUntil)
		d.Periods = parsePGArray(periodStr)
		list = append(list, &d)
	}
	return list, nil
}

// --- Bills ---

func BillGetByIDAll(ctx context.Context, tx *sql.Tx, id string) (*Bill, error) {
	var b Bill
	var dueDate time.Time
	var dueName, houseNum, headName, dueType sql.NullString
	var dueYear sql.NullInt32
	var dueActive bool

	err := tx.QueryRowContext(ctx,
		`SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.is_active, b.created_at, b.updated_at,
		        d.name as due_name, d.is_active as due_is_active, ph.house_number, h.head_name, d.due_type, d.year
		 FROM bills b
		 JOIN dues d ON b.due_id = d.id
		 JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
		 JOIN households h ON ho.household_id = h.id
		 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
		 LEFT JOIN rts rt ON b.rt_id = rt.id
		 WHERE b.id = $1 LIMIT 1`,
		id,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
		&dueName, &dueActive, &houseNum, &headName, &dueType, &dueYear)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get bill by id (all RTs): %w", err)
	}
	b.DueDate = dueDate.Format("2006-01-02")
	if dueName.Valid {
		b.DueName = &dueName.String
	}
	b.DueIsActive = &dueActive
	if houseNum.Valid {
		b.HouseNumber = &houseNum.String
	}
	if headName.Valid {
		b.HeadName = &headName.String
	}
	if dueType.Valid {
		b.DueType = &dueType.String
		if dueType.String == string(DueTypeVoluntary) {
			b.IsVoluntary = true
		}
	}
	if dueYear.Valid {
		yr := int(dueYear.Int32)
		b.DueYear = &yr
	}
	return &b, nil
}

func BillListAll(ctx context.Context, tx *sql.Tx, occupancyID, dueID, status, paymentStatus, period, search, dueName *string, offset, limit int, order ...string) ([]*Bill, error) {
	query := `SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.is_active, b.created_at, b.updated_at,
	                 d.name as due_name, d.is_active as due_is_active, ph.house_number, h.head_name, d.is_voluntary,
	                 COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.bill_id = b.id AND p.status = 'APPROVED'), 0)::text as paid_amount,
	                 COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.bill_id = b.id AND p.status = 'PENDING'), 0)::text as pending_amount,
	                 d.due_type, d.year
	          FROM bills b
	          JOIN dues d ON b.due_id = d.id
	          JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
	          JOIN households h ON ho.household_id = h.id
	          LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
	          LEFT JOIN rts rt ON b.rt_id = rt.id`

	var whereParts []string
	var args []interface{}
	pos := 1

	if occupancyID != nil && *occupancyID != "" {
		whereParts = append(whereParts, fmt.Sprintf("b.household_occupancy_id = $%d", pos))
		args = append(args, *occupancyID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		whereParts = append(whereParts, fmt.Sprintf("b.due_id = $%d", pos))
		args = append(args, *dueID)
		pos++
	}
	if status != nil && *status != "" {
		whereParts = append(whereParts, fmt.Sprintf("b.status = $%d", pos))
		args = append(args, *status)
		pos++
	}
	if paymentStatus != nil && *paymentStatus != "" {
		whereParts = append(whereParts, fmt.Sprintf("EXISTS (SELECT 1 FROM payments p WHERE p.bill_id = b.id AND p.rt_id = b.rt_id AND p.status = $%d)", pos))
		args = append(args, *paymentStatus)
		pos++
	}
	if period != nil && *period != "" {
		whereParts = append(whereParts, fmt.Sprintf("b.period = $%d", pos))
		args = append(args, *period)
		pos++
	}
	if search != nil && *search != "" {
		whereParts = append(whereParts, fmt.Sprintf("(d.name ILIKE $%d OR h.head_name ILIKE $%d OR ph.house_number ILIKE $%d)", pos, pos, pos))
		args = append(args, "%"+*search+"%")
		pos++
	}
	if dueName != nil && *dueName != "" {
		whereParts = append(whereParts, fmt.Sprintf("d.name = $%d", pos))
		args = append(args, *dueName)
		pos++
	}

	query += " WHERE 1=1"
	if len(whereParts) > 0 {
		query += " AND " + strings.Join(whereParts, " AND ")
	}

	orderType := ""
	if len(order) > 0 {
		orderType = order[0]
	}
	if orderType == "unpaid_first" {
		query += fmt.Sprintf(" ORDER BY CASE WHEN b.status = 'paid' THEN 1 ELSE 0 END ASC, CASE WHEN b.status != 'paid' THEN b.period END ASC, CASE WHEN b.status = 'paid' THEN b.period END DESC, b.created_at ASC, b.id ASC OFFSET $%d LIMIT $%d", pos, pos+1)
	} else if occupancyID != nil && *occupancyID != "" {
		query += fmt.Sprintf(" ORDER BY b.period ASC, b.created_at ASC, b.id ASC OFFSET $%d LIMIT $%d", pos, pos+1)
	} else {
		query += fmt.Sprintf(" ORDER BY b.created_at DESC, b.id ASC OFFSET $%d LIMIT $%d", pos, pos+1)
	}
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all bills: %w", err)
	}
	defer rows.Close()

	var list []*Bill
	for rows.Next() {
		var b Bill
		var dueDate time.Time
		var dueName, houseNum, headName, paidAmt, pendingAmt, dueType sql.NullString
		var dueYear sql.NullInt32
		var dueActive bool
		if err := rows.Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
			&dueName, &dueActive, &houseNum, &headName, &b.IsVoluntary, &paidAmt, &pendingAmt, &dueType, &dueYear); err != nil {
			return nil, fmt.Errorf("scan bill: %w", err)
		}
		b.DueDate = dueDate.Format("2006-01-02")
		if dueName.Valid {
			b.DueName = &dueName.String
		}
		b.DueIsActive = &dueActive
		if houseNum.Valid {
			b.HouseNumber = &houseNum.String
		}
		if headName.Valid {
			b.HeadName = &headName.String
		}
		if paidAmt.Valid {
			b.PaidAmount = &paidAmt.String
		}
		if pendingAmt.Valid {
			b.PendingAmount = &pendingAmt.String
		}
		if dueType.Valid {
			b.DueType = &dueType.String
			if dueType.String == string(DueTypeVoluntary) {
				b.IsVoluntary = true
			}
		}
		if dueYear.Valid {
			yr := int(dueYear.Int32)
			b.DueYear = &yr
		}
		list = append(list, &b)
	}
	return list, nil
}

func BillCountAll(ctx context.Context, tx *sql.Tx, occupancyID, dueID, status, paymentStatus, period, search, dueName *string) (int, error) {
	query := `SELECT COUNT(*) FROM bills b
JOIN dues d ON b.due_id = d.id
JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
JOIN households h ON ho.household_id = h.id
LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
WHERE 1=1`
	var args []interface{}
	pos := 1

	if occupancyID != nil && *occupancyID != "" {
		query += fmt.Sprintf(" AND b.household_occupancy_id = $%d", pos)
		args = append(args, *occupancyID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		query += fmt.Sprintf(" AND b.due_id = $%d", pos)
		args = append(args, *dueID)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND b.status = $%d", pos)
		args = append(args, *status)
		pos++
	}
	if paymentStatus != nil && *paymentStatus != "" {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM payments p WHERE p.bill_id = b.id AND p.rt_id = b.rt_id AND p.status = $%d)", pos)
		args = append(args, *paymentStatus)
		pos++
	}
	if period != nil && *period != "" {
		query += fmt.Sprintf(" AND b.period = $%d", pos)
		args = append(args, *period)
		pos++
	}
	if search != nil && *search != "" {
		query += fmt.Sprintf(" AND (d.name ILIKE $%d OR h.head_name ILIKE $%d OR ph.house_number ILIKE $%d)", pos, pos, pos)
		args = append(args, "%"+*search+"%")
		pos++
	}
	if dueName != nil && *dueName != "" {
		query += fmt.Sprintf(" AND d.name = $%d", pos)
		args = append(args, *dueName)
		pos++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// --- Payments ---

func PaymentGetByIDAll(ctx context.Context, tx *sql.Tx, id string) (*Payment, error) {
	p, err := scanPaymentJoined(tx.QueryRowContext(ctx,
		`SELECT `+paymentJoinedColumns+` FROM payments p LEFT JOIN users u ON u.id = p.submitted_by LEFT JOIN bills b ON b.id = p.bill_id LEFT JOIN transactions t ON t.payment_id = p.id LEFT JOIN fund_locations fl ON t.location_id = fl.id WHERE p.id = $1 LIMIT 1`,
		id,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get payment by id (all RTs): %w", err)
	}
	return p, nil
}

func PaymentListAll(ctx context.Context, tx *sql.Tx, billID, dueID, status *string, offset, limit int) ([]*Payment, error) {
	query := `SELECT ` + paymentJoinedColumns + ` FROM payments p LEFT JOIN users u ON u.id = p.submitted_by LEFT JOIN bills b ON b.id = p.bill_id LEFT JOIN transactions t ON t.payment_id = p.id LEFT JOIN fund_locations fl ON t.location_id = fl.id WHERE 1=1`
	var args []interface{}
	pos := 1

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND bill_id = $%d", pos)
		args = append(args, *billID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		query += fmt.Sprintf(" AND due_id = $%d", pos)
		args = append(args, *dueID)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND p.status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY p.created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all payments: %w", err)
	}
	defer rows.Close()

	list := make([]*Payment, 0)
	for rows.Next() {
		p, err := scanPaymentJoined(rows)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		list = append(list, p)
	}
	return list, nil
}

func PaymentCountAll(ctx context.Context, tx *sql.Tx, billID, dueID, status *string) (int, error) {
	query := `SELECT COUNT(*) FROM payments WHERE 1=1`
	var args []interface{}
	pos := 1

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND bill_id = $%d", pos)
		args = append(args, *billID)
		pos++
	}
	if dueID != nil && *dueID != "" {
		query += fmt.Sprintf(" AND due_id = $%d", pos)
		args = append(args, *dueID)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// --- Transactions ---

func TransactionGetByIDAll(ctx context.Context, tx *sql.Tx, id string) (*Transaction, error) {
	var t Transaction
	var catName, payNo, payMethod, createdBy, officerName, dueName, citizenName, houseNumber, locID, locName sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
		        t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
		        fc.name as category_name,
		        COALESCE(NULLIF(p.payment_no, ''), NULLIF(t.reference_no, '')) as payment_no,
		        COALESCE(NULLIF(t.payment_method, ''), p.method::text) as payment_method,
		        t.created_by::text,
		        COALESCE(u_creator.full_name, u_creator.email) as officer_name,
		        COALESCE(d.name, '') as due_name,
		        COALESCE(NULLIF(u_warga.full_name, ''), NULLIF(h.head_name, ''), 'Warga') as citizen_name,
		        COALESCE(ph.house_number, '') as house_number,
		        t.location_id::text,
		        COALESCE(fl.name, '') as location_name
		 FROM transactions t
		 JOIN financial_categories fc ON t.category_id = fc.id
		 LEFT JOIN users u_creator ON t.created_by = u_creator.id
		 LEFT JOIN payments p ON t.payment_id = p.id
		 LEFT JOIN bills b ON p.bill_id = b.id
		 LEFT JOIN dues d ON (p.due_id = d.id OR (p.due_id IS NULL AND b.due_id = d.id))
		 LEFT JOIN users u_warga ON p.submitted_by = u_warga.id
		 LEFT JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
		 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
		 LEFT JOIN households h ON ho.household_id = h.id
		 LEFT JOIN fund_locations fl ON t.location_id = fl.id
		 WHERE t.id = $1 LIMIT 1`,
		id,
	).Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
		&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
		&catName, &payNo, &payMethod, &createdBy, &officerName, &dueName, &citizenName, &houseNumber,
		&locID, &locName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get transaction by id (all RTs): %w", err)
	}
	if catName.Valid {
		t.CategoryName = &catName.String
	}
	if payNo.Valid && payNo.String != "" {
		t.PaymentNo = &payNo.String
		t.ReferenceNo = &payNo.String
	}
	if payMethod.Valid {
		t.PaymentMethod = &payMethod.String
	}
	if createdBy.Valid {
		t.CreatedBy = &createdBy.String
	}
	if officerName.Valid && officerName.String != "" {
		t.OfficerName = &officerName.String
	}
	if dueName.Valid && dueName.String != "" {
		t.DueName = &dueName.String
	}
	if citizenName.Valid && citizenName.String != "" {
		t.CitizenName = &citizenName.String
	}
	if houseNumber.Valid && houseNumber.String != "" {
		t.HouseNumber = &houseNumber.String
	}
	if locID.Valid && locID.String != "" {
		t.LocationID = &locID.String
	}
	if locName.Valid && locName.String != "" {
		t.LocationName = &locName.String
	}
	return &t, nil
}

func TransactionListAll(ctx context.Context, tx *sql.Tx, categoryID, transType, status, startDate, endDate *string, offset, limit int) ([]*Transaction, error) {
	query := `SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
	                 t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
	                 fc.name as category_name,
	                 COALESCE(NULLIF(p.payment_no, ''), NULLIF(t.reference_no, '')) as payment_no,
	                 COALESCE(NULLIF(t.payment_method, ''), p.method::text) as payment_method,
	                 t.created_by::text,
	                 COALESCE(u_creator.full_name, u_creator.email) as officer_name,
	                 COALESCE(d.name, '') as due_name,
	                 COALESCE(NULLIF(u_warga.full_name, ''), NULLIF(h.head_name, ''), 'Warga') as citizen_name,
	                 COALESCE(ph.house_number, '') as house_number,
	                 t.location_id::text,
	                 COALESCE(fl.name, '') as location_name
	          FROM transactions t
	          JOIN financial_categories fc ON t.category_id = fc.id
	          LEFT JOIN users u_creator ON t.created_by = u_creator.id
	          LEFT JOIN payments p ON t.payment_id = p.id
	          LEFT JOIN bills b ON p.bill_id = b.id
	          LEFT JOIN dues d ON (p.due_id = d.id OR (p.due_id IS NULL AND b.due_id = d.id))
	          LEFT JOIN users u_warga ON p.submitted_by = u_warga.id
	          LEFT JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
	          LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
	          LEFT JOIN households h ON ho.household_id = h.id
	          LEFT JOIN fund_locations fl ON t.location_id = fl.id
	          WHERE 1=1`
	var args []interface{}
	pos := 1

	if categoryID != nil && *categoryID != "" {
		query += fmt.Sprintf(" AND t.category_id = $%d", pos)
		args = append(args, *categoryID)
		pos++
	}
	if transType != nil && *transType != "" {
		query += fmt.Sprintf(" AND t.type = $%d", pos)
		args = append(args, *transType)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND t.status = $%d", pos)
		args = append(args, *status)
		pos++
	}
	if startDate != nil && *startDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at >= $%d", pos)
		args = append(args, *startDate)
		pos++
	}
	if endDate != nil && *endDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at <= $%d", pos)
		args = append(args, *endDate)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY t.occurred_at DESC, t.created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions (all RTs): %w", err)
	}
	defer rows.Close()

	var list []*Transaction
	for rows.Next() {
		var t Transaction
		var catName, payNo, payMethod, createdBy, officerName, dueName, citizenName, houseNumber, locID, locName sql.NullString
		if err := rows.Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
			&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
			&catName, &payNo, &payMethod, &createdBy, &officerName, &dueName, &citizenName, &houseNumber,
			&locID, &locName); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		if catName.Valid {
			t.CategoryName = &catName.String
		}
		if payNo.Valid && payNo.String != "" {
			t.PaymentNo = &payNo.String
			t.ReferenceNo = &payNo.String
		}
		if payMethod.Valid {
			t.PaymentMethod = &payMethod.String
		}
		if createdBy.Valid {
			t.CreatedBy = &createdBy.String
		}
		if officerName.Valid && officerName.String != "" {
			t.OfficerName = &officerName.String
		}
		if dueName.Valid && dueName.String != "" {
			t.DueName = &dueName.String
		}
		if citizenName.Valid && citizenName.String != "" {
			t.CitizenName = &citizenName.String
		}
		if houseNumber.Valid && houseNumber.String != "" {
			t.HouseNumber = &houseNumber.String
		}
		if locID.Valid && locID.String != "" {
			t.LocationID = &locID.String
		}
		if locName.Valid && locName.String != "" {
			t.LocationName = &locName.String
		}
		list = append(list, &t)
	}
	return list, nil
}

func TransactionUpdate(ctx context.Context, tx *sql.Tx, rtID, id string, categoryID *string, amount *string, transType *TransactionType, description *string, occurredAt *time.Time, paymentMethod *string) (*Transaction, error) {
	query := `UPDATE transactions
	          SET category_id = COALESCE($1, category_id),
	              amount = COALESCE($2::numeric, amount),
	              type = COALESCE($3, type),
	              description = COALESCE($4, description),
	              occurred_at = COALESCE($5, occurred_at),
	              payment_method = COALESCE($6, payment_method),
	              updated_at = now()
	          WHERE id = $7 AND rt_id = $8
	          RETURNING id`
	var updatedID string
	err := tx.QueryRowContext(ctx, query, categoryID, amount, transType, description, occurredAt, paymentMethod, id, rtID).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update transaction: %w", err)
	}
	return TransactionGetByID(ctx, tx, updatedID, rtID)
}

func TransactionCountAll(ctx context.Context, tx *sql.Tx, categoryID, transType, status, startDate, endDate *string) (int, error) {
	query := `SELECT COUNT(*) FROM transactions t WHERE 1=1`
	var args []interface{}
	pos := 1

	if categoryID != nil && *categoryID != "" {
		query += fmt.Sprintf(" AND t.category_id = $%d", pos)
		args = append(args, *categoryID)
		pos++
	}
	if transType != nil && *transType != "" {
		query += fmt.Sprintf(" AND t.type = $%d", pos)
		args = append(args, *transType)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND t.status = $%d", pos)
		args = append(args, *status)
		pos++
	}
	if startDate != nil && *startDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at >= $%d", pos)
		args = append(args, *startDate)
		pos++
	}
	if endDate != nil && *endDate != "" {
		query += fmt.Sprintf(" AND t.occurred_at <= $%d", pos)
		args = append(args, *endDate)
		pos++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// --- Balance ---

// CalculateBalanceAll derives total balance across ALL RTs: Total Posted Income - Total Posted Expense.
func CalculateBalanceAll(ctx context.Context, tx *sql.Tx) (*BalanceSummary, error) {
	var incomeStr, expenseStr string
	var count int

	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text as total_income,
		        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text as total_expense,
		        COUNT(*) as tx_count
		 FROM transactions
		 WHERE status = 'posted'`,
	).Scan(&incomeStr, &expenseStr, &count)
	if err != nil {
		return nil, fmt.Errorf("derive balance (all RTs): %w", err)
	}

	incCents, incCanon, err := ParseMoney(incomeStr)
	if err != nil {
		return nil, fmt.Errorf("parse income: %w", err)
	}
	expCents, expCanon, err := ParseMoney(expenseStr)
	if err != nil {
		return nil, fmt.Errorf("parse expense: %w", err)
	}

	netCents := incCents - expCents
	netCanon := FormatMoney(netCents)

	return &BalanceSummary{
		TotalIncome:      incCanon,
		TotalExpense:     expCanon,
		NetBalance:       netCanon,
		TransactionCount: count,
	}, nil
}

// ─── Payment Allocation Repository ──────────────────────────────────

func PaymentAllocationCreate(ctx context.Context, tx *sql.Tx, paymentID, billID, amount string) (*PaymentAllocation, error) {
	var a PaymentAllocation
	err := tx.QueryRowContext(ctx,
		`INSERT INTO payment_allocations (payment_id, bill_id, amount)
		 VALUES ($1, $2, $3)
		 RETURNING id, payment_id, bill_id, amount, created_at`,
		paymentID, billID, amount,
	).Scan(&a.ID, &a.PaymentID, &a.BillID, &a.Amount, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create payment allocation: %w", err)
	}
	return &a, nil
}

func PaymentAllocationsByPaymentID(ctx context.Context, tx *sql.Tx, paymentID string) ([]*PaymentAllocation, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, payment_id, bill_id, amount, created_at
		 FROM payment_allocations
		 WHERE payment_id = $1
		 ORDER BY created_at ASC`,
		paymentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list payment allocations: %w", err)
	}
	defer rows.Close()

	var list []*PaymentAllocation
	for rows.Next() {
		var a PaymentAllocation
		if err := rows.Scan(&a.ID, &a.PaymentID, &a.BillID, &a.Amount, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan payment allocation: %w", err)
		}
		list = append(list, &a)
	}
	return list, nil
}

func PaymentAllocationsByBillID(ctx context.Context, tx *sql.Tx, billID string) ([]*PaymentAllocation, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, payment_id, bill_id, amount, created_at
		 FROM payment_allocations
		 WHERE bill_id = $1
		 ORDER BY created_at ASC`,
		billID,
	)
	if err != nil {
		return nil, fmt.Errorf("list payment allocations by bill: %w", err)
	}
	defer rows.Close()

	var list []*PaymentAllocation
	for rows.Next() {
		var a PaymentAllocation
		if err := rows.Scan(&a.ID, &a.PaymentID, &a.BillID, &a.Amount, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan payment allocation: %w", err)
		}
		list = append(list, &a)
	}
	return list, nil
}

// ─── Period Balance & Opening Balances ──────────────────────────────

// CalculatePeriodBalance derives monthly or range period balance sheet for an RT.
// Uses half-open interval [startTime, endTime) in Asia/Jakarta timezone.
// Opening balance respects locked manual corrections in period_opening_balances.
func CalculatePeriodBalance(ctx context.Context, tx *sql.Tx, rtID, periodStr string, startTime, endTime time.Time) (*PeriodBalanceSummary, error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	var openingCents int64

	// 1. Check if an exact manual record exists for periodStr
	var exactManual bool
	if periodStr != "" {
		var openingStr, source string
		err := tx.QueryRowContext(ctx,
			`SELECT opening_balance::text, source
			 FROM period_opening_balances
			 WHERE rt_id = $1 AND period = $2`,
			rtID, periodStr,
		).Scan(&openingStr, &source)
		if err == nil && source == "manual" {
			exactManual = true
			cents, _, err := ParseMoney(openingStr)
			if err != nil {
				return nil, fmt.Errorf("parse manual opening balance: %w", err)
			}
			openingCents = cents
		}
	}

	// 2. If no exact manual record, check if an earlier manual record exists
	if !exactManual {
		var earlierPeriod, earlierOpeningStr string
		err := tx.QueryRowContext(ctx,
			`SELECT period, opening_balance::text
			 FROM period_opening_balances
			 WHERE rt_id = $1 AND period < $2 AND source = 'manual'
			 ORDER BY period DESC
			 LIMIT 1`,
			rtID, startTime.In(loc).Format("2006-01"),
		).Scan(&earlierPeriod, &earlierOpeningStr)

		if err == nil {
			earlierCents, _, err := ParseMoney(earlierOpeningStr)
			if err != nil {
				return nil, fmt.Errorf("parse earlier manual opening balance: %w", err)
			}
			earlierTime, err := time.ParseInLocation("2006-01", earlierPeriod, loc)
			if err != nil {
				return nil, fmt.Errorf("parse earlier period time: %w", err)
			}
			earlierStart := time.Date(earlierTime.Year(), earlierTime.Month(), 1, 0, 0, 0, 0, loc)

			// Net movement from earlier manual base to startTime
			var intermediateInc, intermediateExp string
			err = tx.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text,
				        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text
				 FROM transactions
				 WHERE rt_id = $1 AND status = 'posted'
				   AND occurred_at >= $2 AND occurred_at < $3`,
				rtID, earlierStart, startTime,
			).Scan(&intermediateInc, &intermediateExp)
			if err != nil {
				return nil, fmt.Errorf("calculate intermediate net movement: %w", err)
			}

			intIncCents, _, _ := ParseMoney(intermediateInc)
			intExpCents, _, _ := ParseMoney(intermediateExp)
			openingCents = earlierCents + (intIncCents - intExpCents)
		} else {
			// No manual base exists anywhere before startTime: cumulative from genesis
			var priorInc, priorExp string
			err = tx.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text,
				        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text
				 FROM transactions
				 WHERE rt_id = $1 AND status = 'posted'
				   AND occurred_at < $2`,
				rtID, startTime,
			).Scan(&priorInc, &priorExp)
			if err != nil {
				return nil, fmt.Errorf("calculate prior balance: %w", err)
			}

			pIncCents, _, _ := ParseMoney(priorInc)
			pExpCents, _, _ := ParseMoney(priorExp)
			openingCents = pIncCents - pExpCents
		}
	}

	// 3. Calculate period movements and count within [startTime, endTime)
	var periodInc, periodExp string
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text,
		        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text,
		        COUNT(*)
		 FROM transactions
		 WHERE rt_id = $1 AND status = 'posted'
		   AND occurred_at >= $2 AND occurred_at < $3`,
		rtID, startTime, endTime,
	).Scan(&periodInc, &periodExp, &count)
	if err != nil {
		return nil, fmt.Errorf("calculate period movements: %w", err)
	}

	incCents, _, _ := ParseMoney(periodInc)
	expCents, _, _ := ParseMoney(periodExp)
	netMovementCents := incCents - expCents
	closingCents := openingCents + netMovementCents

	return &PeriodBalanceSummary{
		OpeningBalance:   FormatMoney(openingCents),
		TotalIncome:      FormatMoney(incCents),
		TotalExpense:     FormatMoney(expCents),
		NetMovement:      FormatMoney(netMovementCents),
		ClosingBalance:   FormatMoney(closingCents),
		TransactionCount: count,
	}, nil
}

// SetPeriodOpeningBalance records a manual correction of opening balance for a period.
func SetPeriodOpeningBalance(ctx context.Context, tx *sql.Tx, rtID, period string, newOpening string, reason string, userID string) (*PeriodOpeningBalanceResponse, error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	t, err := time.ParseInLocation("2006-01", period, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid period format: %w", err)
	}
	startTime := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	endTime := startTime.AddDate(0, 1, 0)

	// Calculate current opening balance before this update
	currentSummary, err := CalculatePeriodBalance(ctx, tx, rtID, period, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("calculate current opening balance: %w", err)
	}
	openingBefore := currentSummary.OpeningBalance

	_, canonicalNewOpening, err := ParseMoney(newOpening)
	if err != nil {
		return nil, fmt.Errorf("invalid opening balance: %w", err)
	}

	var correctedAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO period_opening_balances (
		    rt_id, period, opening_balance, source, previous_balance, reason, updated_by, updated_at
		 ) VALUES ($1, $2, $3, 'manual', $4, $5, $6, CURRENT_TIMESTAMP)
		 ON CONFLICT (rt_id, period) DO UPDATE SET
		    opening_balance = EXCLUDED.opening_balance,
		    source = 'manual',
		    previous_balance = period_opening_balances.opening_balance,
		    reason = EXCLUDED.reason,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = CURRENT_TIMESTAMP
		 RETURNING updated_at`,
		rtID, period, canonicalNewOpening, openingBefore, reason, userID,
	).Scan(&correctedAt)
	if err != nil {
		return nil, fmt.Errorf("set period opening balance: %w", err)
	}

	return &PeriodOpeningBalanceResponse{
		Period:               period,
		OpeningBalanceBefore: openingBefore,
		OpeningBalanceAfter:  canonicalNewOpening,
		Source:               "manual",
		CorrectedBy:          userID,
		CorrectedAt:          correctedAt.In(loc).Format(time.RFC3339),
		Reason:               reason,
	}, nil
}
