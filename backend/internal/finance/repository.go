package finance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	err := tx.QueryRowContext(ctx,
		`INSERT INTO dues (rt_id, name, amount, period_type, payment_rule, is_voluntary, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, true)
		 RETURNING id, rt_id, name, amount::text, period_type, payment_rule, is_voluntary, is_active, created_at, updated_at`,
		rtID, in.Name, in.Amount, in.PeriodType, in.PaymentRule, in.IsVoluntary,
	).Scan(&d.ID, &d.RTID, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create dues: %w", err)
	}
	return &d, nil
}

func DueGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Due, error) {
	var d Due
	err := tx.QueryRowContext(ctx,
		`SELECT id, rt_id, name, amount::text, period_type, payment_rule, is_voluntary, is_active, created_at, updated_at
		 FROM dues WHERE id = $1 AND rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&d.ID, &d.RTID, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get due by id: %w", err)
	}
	return &d, nil
}

func DueList(ctx context.Context, tx *sql.Tx, rtID string, isActive *bool) ([]*Due, error) {
	query := `SELECT id, rt_id, name, amount::text, period_type, payment_rule, is_voluntary, is_active, created_at, updated_at
	          FROM dues WHERE rt_id = $1`
	var args []interface{}
	args = append(args, rtID)
	if isActive != nil {
		query += " AND is_active = $2"
		args = append(args, *isActive)
	}
	query += " ORDER BY name ASC"

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dues: %w", err)
	}
	defer rows.Close()

	var list []*Due
	for rows.Next() {
		var d Due
		if err := rows.Scan(&d.ID, &d.RTID, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan due: %w", err)
		}
		list = append(list, &d)
	}
	return list, nil
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

	var d Due
	err = tx.QueryRowContext(ctx,
		`UPDATE dues
		 SET name = $1, amount = $2, period_type = $3, payment_rule = $4, is_voluntary = $5, is_active = $6, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $7 AND rt_id = $8
		 RETURNING id, rt_id, name, amount::text, period_type, payment_rule, is_voluntary, is_active, created_at, updated_at`,
		name, amount, periodType, paymentRule, isVoluntary, isActive, id, rtID,
	).Scan(&d.ID, &d.RTID, &d.Name, &d.Amount, &d.PeriodType, &d.PaymentRule, &d.IsVoluntary, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update due: %w", err)
	}
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
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'unpaid')
		 RETURNING id, rt_id, (SELECT rts.rt FROM rts WHERE id = $1) as rt_code, household_occupancy_id, due_id, amount::text, period, due_date, status, created_at, updated_at`,
		rtID, in.HouseholdOccupancyID, in.DueID, in.Amount, in.Period, in.DueDate,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create bill: %w", err)
	}
	b.DueDate = dueDate.Format("2006-01-02")
	return &b, nil
}

func BillGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Bill, error) {
	var b Bill
	var dueDate time.Time
	var dueName, houseNum, headName sql.NullString

	err := tx.QueryRowContext(ctx,
		`SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.created_at, b.updated_at,
		        d.name as due_name, ph.house_number, h.head_name
		 FROM bills b
		 JOIN dues d ON b.due_id = d.id
		 JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
		 JOIN households h ON ho.household_id = h.id
		 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
		 LEFT JOIN rts rt ON b.rt_id = rt.id
		 WHERE b.id = $1 AND b.rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.CreatedAt, &b.UpdatedAt,
		&dueName, &houseNum, &headName)
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
	if houseNum.Valid {
		b.HouseNumber = &houseNum.String
	}
	if headName.Valid {
		b.HeadName = &headName.String
	}
	return &b, nil
}

func BillList(ctx context.Context, tx *sql.Tx, rtID string, occupancyID, dueID, status, period *string, offset, limit int) ([]*Bill, error) {
	query := `SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.created_at, b.updated_at,
	                 d.name as due_name, ph.house_number, h.head_name
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
	if period != nil && *period != "" {
		query += fmt.Sprintf(" AND b.period LIKE $%d || '%%'", pos)
		args = append(args, *period)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY b.created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
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
		var dueName, houseNum, headName sql.NullString
		if err := rows.Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.CreatedAt, &b.UpdatedAt,
			&dueName, &houseNum, &headName); err != nil {
			return nil, fmt.Errorf("scan bill: %w", err)
		}
		b.DueDate = dueDate.Format("2006-01-02")
		if dueName.Valid {
			b.DueName = &dueName.String
		}
		if houseNum.Valid {
			b.HouseNumber = &houseNum.String
		}
		if headName.Valid {
			b.HeadName = &headName.String
		}
		list = append(list, &b)
	}
	return list, nil
}

func BillCount(ctx context.Context, tx *sql.Tx, rtID string, occupancyID, dueID, status, period *string) (int, error) {
	query := `SELECT COUNT(*) FROM bills b WHERE b.rt_id = $1`
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
	if period != nil && *period != "" {
		query += fmt.Sprintf(" AND b.period LIKE $%d || '%%'", pos)
		args = append(args, *period)
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
		`SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.created_at, b.updated_at
		 FROM bills b
		 JOIN dues d ON b.due_id = d.id
		 JOIN rts rt ON b.rt_id = rt.id
		 WHERE b.id = $1 AND b.rt_id = $2 LIMIT 1 FOR UPDATE`,
		id, rtID,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get bill by id for update: %w", err)
	}
	b.DueDate = dueDate.Format("2006-01-02")
	return &b, nil
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

// GetOccupancyIDForUser resolves the current active household occupancy for a warga user.
func GetOccupancyIDForUser(ctx context.Context, tx *sql.Tx, rtID, userID string) (string, error) {
	var occupancyID string
	err := tx.QueryRowContext(ctx,
		`SELECT ho.id
		 FROM users u
		 JOIN residents r ON r.phone = u.phone AND r.rt_id = $1 AND r.is_active = true
		 JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
		 JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id AND ho.end_date IS NULL
		 WHERE u.id = $2
		 LIMIT 1`,
		rtID, userID,
	).Scan(&occupancyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil // No occupancy tied to user phone
		}
		return "", fmt.Errorf("lookup user occupancy: %w", err)
	}
	return occupancyID, nil
}

// --- Payments Repository ---

func PaymentCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreatePaymentInput, origin PaymentOrigin, status PaymentStatus, verifiedBy *string, verifiedAt *time.Time) (*Payment, error) {
	var p Payment
	var uploadedAt *time.Time
	if in.ProofPath != nil {
		now := time.Now()
		uploadedAt = &now
	}

	err := tx.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, proof_path, notes, officer_notes, verified_by, verified_at, verified_by_name, payment_no,
		                      proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		 RETURNING id, rt_id, bill_id, amount::text, method, origin, status, proof_path, paid_at,
		           verified_by, verified_at, verified_by_name, rejection_reason, notes, officer_notes, payment_no,
		           created_at, updated_at, proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by`,
		rtID, in.BillID, in.Amount, in.Method, origin, status, in.ProofPath, in.Notes, nil, verifiedBy, verifiedAt, nil, nil,
		in.ProofOriginalName, in.ProofMimeType, in.ProofSizeBytes, uploadedAt, in.ProofUploadedBy,
	).Scan(&p.ID, &p.RTID, &p.BillID, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
		&p.VerifiedBy, &p.VerifiedAt, &p.VerifiedByName, &p.RejectionReason, &p.Notes, &p.OfficerNotes, &p.PaymentNo,
		&p.CreatedAt, &p.UpdatedAt,
		&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy)
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return &p, nil
}

func PaymentGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Payment, error) {
	var p Payment
	err := tx.QueryRowContext(ctx,
		`SELECT id, rt_id, bill_id, amount::text, method, origin, status, proof_path, paid_at,
		        verified_by, verified_at, verified_by_name, rejection_reason, notes, officer_notes, payment_no, created_at, updated_at,
		        proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by
		 FROM payments WHERE id = $1 AND rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&p.ID, &p.RTID, &p.BillID, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
		&p.VerifiedBy, &p.VerifiedAt, &p.VerifiedByName, &p.RejectionReason, &p.Notes, &p.OfficerNotes, &p.PaymentNo, &p.CreatedAt, &p.UpdatedAt,
		&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get payment by id: %w", err)
	}
	return &p, nil
}

func PaymentList(ctx context.Context, tx *sql.Tx, rtID string, billID, status *string, offset, limit int) ([]*Payment, error) {
	query := `SELECT id, rt_id, bill_id, amount::text, method, origin, status, proof_path, paid_at,
	                 verified_by, verified_at, verified_by_name, rejection_reason, notes, officer_notes, payment_no, created_at, updated_at,
	                 proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by
	          FROM payments WHERE rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND bill_id = $%d", pos)
		args = append(args, *billID)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	list := make([]*Payment, 0)
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.RTID, &p.BillID, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
			&p.VerifiedBy, &p.VerifiedAt, &p.VerifiedByName, &p.RejectionReason, &p.Notes, &p.OfficerNotes, &p.PaymentNo, &p.CreatedAt, &p.UpdatedAt,
			&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		list = append(list, &p)
	}
	return list, nil
}

func PaymentCount(ctx context.Context, tx *sql.Tx, rtID string, billID, status *string) (int, error) {
	query := `SELECT COUNT(*) FROM payments WHERE rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND bill_id = $%d", pos)
		args = append(args, *billID)
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

func PaymentVerify(ctx context.Context, tx *sql.Tx, id, rtID, verifierID string, status PaymentStatus, reason, officerNotes string) (*Payment, error) {
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
		// Under row lock: safe to read MAX and write back.
		var seq int64
		err = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(CAST(SUBSTRING(payment_no FROM 15) AS INTEGER)), 0)
			 FROM payments
			 WHERE payment_no LIKE 'PAY-' || TO_CHAR(CURRENT_DATE, 'YYYYMMDD') || '-%'`,
		).Scan(&seq)
		if err != nil {
			seq = 0
		}
		seq++
		v := fmt.Sprintf("PAY-%s-%05d", time.Now().Format("20060102"), seq)
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

	err = tx.QueryRowContext(ctx,
		`UPDATE payments
		 SET status = $1, verified_by = $2, verified_at = CURRENT_TIMESTAMP, rejection_reason = $3,
		     officer_notes = $4, verified_by_name = $5, payment_no = $6, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $7 AND rt_id = $8
		 RETURNING id, rt_id, bill_id, amount::text, method, origin, status, proof_path, paid_at,
		           verified_by, verified_at, verified_by_name, rejection_reason, notes, officer_notes, payment_no,
		           created_at, updated_at,
		           proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by`,
		status, verifierID, reasonPtr, officerNotesPtr, verifierName, p.PaymentNo, id, rtID,
	).Scan(&p.ID, &p.RTID, &p.BillID, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
		&p.VerifiedBy, &p.VerifiedAt, &p.VerifiedByName, &p.RejectionReason, &p.Notes, &p.OfficerNotes, &p.PaymentNo,
		&p.CreatedAt, &p.UpdatedAt,
		&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("verify payment: %w", err)
	}
	return &p, nil
}

func PaymentAttachProof(ctx context.Context, tx *sql.Tx, paymentID, rtID, proofPath, originalName, mimeType string, size int64, uploadedBy string) (*Payment, error) {
	var p Payment
	err := tx.QueryRowContext(ctx,
		`UPDATE payments
		 SET proof_path = $1, proof_original_name = $2, proof_mime_type = $3, proof_size_bytes = $4,
		     proof_uploaded_at = CURRENT_TIMESTAMP, proof_uploaded_by = $5, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $6 AND rt_id = $7
		 RETURNING id, rt_id, bill_id, amount::text, method, origin, status, proof_path, paid_at,
		           verified_by, verified_at, rejection_reason, notes, created_at, updated_at,
		           proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by`,
		proofPath, originalName, mimeType, size, uploadedBy, paymentID, rtID,
	).Scan(&p.ID, &p.RTID, &p.BillID, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
		&p.VerifiedBy, &p.VerifiedAt, &p.RejectionReason, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
		&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("attach proof to payment: %w", err)
	}
	return &p, nil
}

// --- Transactions (Immutable Ledger) Repository ---

func TransactionCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateTransactionInput, status TransactionStatus, reversesID, paymentID *string, occurredAt time.Time) (*Transaction, error) {
	var t Transaction
	err := tx.QueryRowContext(ctx,
		`INSERT INTO transactions (rt_id, category_id, amount, type, status, reverses_transaction_id, payment_id, description, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, rt_id, category_id, amount::text, type, status, reverses_transaction_id, payment_id, description, occurred_at, created_at, updated_at`,
		rtID, in.CategoryID, in.Amount, in.Type, status, reversesID, paymentID, in.Description, occurredAt,
	).Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status, &t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	return &t, nil
}

func TransactionGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Transaction, error) {
	var t Transaction
	var catName sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
		        t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
		        fc.name as category_name
		 FROM transactions t
		 JOIN financial_categories fc ON t.category_id = fc.id
		 WHERE t.id = $1 AND t.rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
		&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
		&catName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get transaction by id: %w", err)
	}
	if catName.Valid {
		t.CategoryName = &catName.String
	}
	return &t, nil
}

func TransactionList(ctx context.Context, tx *sql.Tx, rtID string, categoryID, transType, status, startDate, endDate *string, offset, limit int) ([]*Transaction, error) {
	query := `SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
	                 t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
	                 fc.name as category_name
	          FROM transactions t
	          JOIN financial_categories fc ON t.category_id = fc.id
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
		var catName sql.NullString
		if err := rows.Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
			&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
			&catName); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		if catName.Valid {
			t.CategoryName = &catName.String
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
	ResponseCode int
	ResponseBody string
}

func IdempotencyGet(ctx context.Context, tx *sql.Tx, rtID, key string) (*CachedResponse, error) {
	var resp CachedResponse
	err := tx.QueryRowContext(ctx,
		`SELECT response_code, response_body FROM idempotency_keys
		 WHERE rt_id = $1 AND key = $2 LIMIT 1`,
		rtID, key,
	).Scan(&resp.ResponseCode, &resp.ResponseBody)
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
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (rt_id, key) DO UPDATE
		 SET response_code = EXCLUDED.response_code, response_body = EXCLUDED.response_body`,
		rtID, key, userID, reqPath, reqHash, code, body,
	)
	if err != nil {
		return fmt.Errorf("save idempotency key: %w", err)
	}
	return nil
}

// --- Audit Logs Repository ---

func AuditLogInsert(ctx context.Context, tx *sql.Tx, log AuditLog) error {
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
	query := `SELECT id, rt_id, name, amount::text, period_type, is_active, created_at, updated_at
	          FROM dues`
	var args []interface{}
	if isActive != nil {
		query += ` WHERE is_active = $1`
		args = append(args, *isActive)
	}
	query += ` ORDER BY rt_id, name ASC`

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all dues: %w", err)
	}
	defer rows.Close()

	var list []*Due
	for rows.Next() {
		var d Due
		if err := rows.Scan(&d.ID, &d.RTID, &d.Name, &d.Amount, &d.PeriodType, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan due: %w", err)
		}
		list = append(list, &d)
	}
	return list, nil
}

// --- Bills ---

func BillGetByIDAll(ctx context.Context, tx *sql.Tx, id string) (*Bill, error) {
	var b Bill
	var dueDate time.Time
	var dueName, houseNum, headName sql.NullString

	err := tx.QueryRowContext(ctx,
		`SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.created_at, b.updated_at,
		        d.name as due_name, ph.house_number, h.head_name
		 FROM bills b
		 JOIN dues d ON b.due_id = d.id
		 JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
		 JOIN households h ON ho.household_id = h.id
		 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
		 LEFT JOIN rts rt ON b.rt_id = rt.id
		 WHERE b.id = $1 LIMIT 1`,
		id,
	).Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.CreatedAt, &b.UpdatedAt,
		&dueName, &houseNum, &headName)
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
	if houseNum.Valid {
		b.HouseNumber = &houseNum.String
	}
	if headName.Valid {
		b.HeadName = &headName.String
	}
	return &b, nil
}

func BillListAll(ctx context.Context, tx *sql.Tx, occupancyID, dueID, status, period *string, offset, limit int) ([]*Bill, error) {
	query := `SELECT b.id, b.rt_id, rt.rt as rt_code, b.household_occupancy_id, b.due_id, b.amount::text, b.period, b.due_date, b.status, b.created_at, b.updated_at,
	                 d.name as due_name, ph.house_number, h.head_name
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
	if period != nil && *period != "" {
		whereParts = append(whereParts, fmt.Sprintf("b.period = $%d", pos))
		args = append(args, *period)
		pos++
	}

	query += " WHERE 1=1"
	if len(whereParts) > 0 {
		query += " AND " + strings.Join(whereParts, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY b.created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
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
		var dueName, houseNum, headName sql.NullString
		if err := rows.Scan(&b.ID, &b.RTID, &b.RTCode, &b.HouseholdOccupancyID, &b.DueID, &b.Amount, &b.Period, &dueDate, &b.Status, &b.CreatedAt, &b.UpdatedAt,
			&dueName, &houseNum, &headName); err != nil {
			return nil, fmt.Errorf("scan bill: %w", err)
		}
		b.DueDate = dueDate.Format("2006-01-02")
		if dueName.Valid {
			b.DueName = &dueName.String
		}
		if houseNum.Valid {
			b.HouseNumber = &houseNum.String
		}
		if headName.Valid {
			b.HeadName = &headName.String
		}
		list = append(list, &b)
	}
	return list, nil
}

func BillCountAll(ctx context.Context, tx *sql.Tx, occupancyID, dueID, status, period *string) (int, error) {
	query := `SELECT COUNT(*) FROM bills b WHERE 1=1`
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
	if period != nil && *period != "" {
		query += fmt.Sprintf(" AND b.period = $%d", pos)
		args = append(args, *period)
		pos++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// --- Payments ---

func PaymentGetByIDAll(ctx context.Context, tx *sql.Tx, id string) (*Payment, error) {
	var p Payment
	err := tx.QueryRowContext(ctx,
		`SELECT id, rt_id, bill_id, amount::text, method, origin, status, proof_path, paid_at,
		        verified_by, verified_at, rejection_reason, notes, created_at, updated_at,
		        proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by
		 FROM payments WHERE id = $1 LIMIT 1`,
		id,
	).Scan(&p.ID, &p.RTID, &p.BillID, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
		&p.VerifiedBy, &p.VerifiedAt, &p.RejectionReason, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
		&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get payment by id (all RTs): %w", err)
	}
	return &p, nil
}

func PaymentListAll(ctx context.Context, tx *sql.Tx, billID, status *string, offset, limit int) ([]*Payment, error) {
	query := `SELECT id, rt_id, bill_id, amount::text, method, origin, status, proof_path, paid_at,
	                 verified_by, verified_at, rejection_reason, notes, created_at, updated_at,
	                 proof_original_name, proof_mime_type, proof_size_bytes, proof_uploaded_at, proof_uploaded_by
	          FROM payments WHERE 1=1`
	var args []interface{}
	pos := 1

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND bill_id = $%d", pos)
		args = append(args, *billID)
		pos++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND status = $%d", pos)
		args = append(args, *status)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC OFFSET $%d LIMIT $%d", pos, pos+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all payments: %w", err)
	}
	defer rows.Close()

	list := make([]*Payment, 0)
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.RTID, &p.BillID, &p.Amount, &p.Method, &p.Origin, &p.Status, &p.ProofPath, &p.PaidAt,
			&p.VerifiedBy, &p.VerifiedAt, &p.RejectionReason, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
			&p.ProofOriginalName, &p.ProofMimeType, &p.ProofSizeBytes, &p.ProofUploadedAt, &p.ProofUploadedBy); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		list = append(list, &p)
	}
	return list, nil
}

func PaymentCountAll(ctx context.Context, tx *sql.Tx, billID, status *string) (int, error) {
	query := `SELECT COUNT(*) FROM payments WHERE 1=1`
	var args []interface{}
	pos := 1

	if billID != nil && *billID != "" {
		query += fmt.Sprintf(" AND bill_id = $%d", pos)
		args = append(args, *billID)
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
	var catName sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
		        t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
		        fc.name as category_name
		 FROM transactions t
		 JOIN financial_categories fc ON t.category_id = fc.id
		 WHERE t.id = $1 LIMIT 1`,
		id,
	).Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
		&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
		&catName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get transaction by id (all RTs): %w", err)
	}
	if catName.Valid {
		t.CategoryName = &catName.String
	}
	return &t, nil
}

func TransactionListAll(ctx context.Context, tx *sql.Tx, categoryID, transType, status, startDate, endDate *string, offset, limit int) ([]*Transaction, error) {
	query := `SELECT t.id, t.rt_id, t.category_id, t.amount::text, t.type, t.status,
	                 t.reverses_transaction_id, t.payment_id, t.description, t.occurred_at, t.created_at, t.updated_at,
	                 fc.name as category_name
	          FROM transactions t
	          JOIN financial_categories fc ON t.category_id = fc.id
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
		return nil, fmt.Errorf("list all transactions: %w", err)
	}
	defer rows.Close()

	var list []*Transaction
	for rows.Next() {
		var t Transaction
		var catName sql.NullString
		if err := rows.Scan(&t.ID, &t.RTID, &t.CategoryID, &t.Amount, &t.Type, &t.Status,
			&t.ReversesTransactionID, &t.PaymentID, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt,
			&catName); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		if catName.Valid {
			t.CategoryName = &catName.String
		}
		list = append(list, &t)
	}
	return list, nil
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
