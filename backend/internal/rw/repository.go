package rw

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"social-finance/internal/storage"
)

// Repository provides RW-scoped queries.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

// =========================================================================
// Dashboard & KAS Reports
// =========================================================================

func (r *Repository) GetDashboardSummary(ctx context.Context, tx *sql.Tx, rwID string) (*DashboardSummary, error) {
	var summary DashboardSummary

	// 1. Count active RTs
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rts WHERE rw_id = $1 AND is_active = true`, rwID,
	).Scan(&summary.TotalRTs)
	if err != nil {
		return nil, fmt.Errorf("count rts: %w", err)
	}

	// 2. Count active RW fund locations (strictly isolated to rw_fund_locations)
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_fund_locations WHERE rw_id = $1 AND is_active = true`, rwID,
	).Scan(&summary.TotalLocations)
	if err != nil {
		return nil, fmt.Errorf("count locations: %w", err)
	}

	// 3. Total opening balance from RW's own ledger
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0)::numeric(15,2)::text
		 FROM rw_opening_balances
		 WHERE rw_id = $1`, rwID,
	).Scan(&summary.TotalOpeningBal)
	if err != nil {
		return nil, fmt.Errorf("opening balance: %w", err)
	}

	// 4. Total income from RW's independent ledger
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN type = 'income' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text
		 FROM rw_transactions
		 WHERE rw_id = $1 AND status = 'posted'`, rwID,
	).Scan(&summary.TotalIncome)
	if err != nil {
		return nil, fmt.Errorf("income aggregate: %w", err)
	}

	// 5. Total expense from RW's independent ledger
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN type = 'expense' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text
		 FROM rw_transactions
		 WHERE rw_id = $1 AND status = 'posted'`, rwID,
	).Scan(&summary.TotalExpense)
	if err != nil {
		return nil, fmt.Errorf("expense aggregate: %w", err)
	}

	// 6. Transaction count from RW's independent ledger
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_transactions
		 WHERE rw_id = $1 AND status = 'posted'`, rwID,
	).Scan(&summary.TransactionCount)
	if err != nil {
		return nil, fmt.Errorf("transaction count: %w", err)
	}

	// 7. Calculate net balance: opening + income - expense
	oc, _, err := ParseMoney(summary.TotalOpeningBal)
	if err != nil {
		return nil, fmt.Errorf("parse opening balance: %w", err)
	}
	incCents, _, err := ParseMoney(summary.TotalIncome)
	if err != nil {
		return nil, fmt.Errorf("parse income: %w", err)
	}
	expCents, _, err := ParseMoney(summary.TotalExpense)
	if err != nil {
		return nil, fmt.Errorf("parse expense: %w", err)
	}
	summary.NetBalance = FormatMoney(oc + incCents - expCents)

	return &summary, nil
}

func (r *Repository) GetKASReport(ctx context.Context, tx *sql.Tx, rwID, period string) (*KASReport, error) {
	var report KASReport
	var txCount int

	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0)::numeric(15,2)::text
		 FROM rw_opening_balances
		 WHERE rw_id = $1`, rwID,
	).Scan(&report.OpeningBalance)
	if err != nil {
		return nil, fmt.Errorf("kas opening balance: %w", err)
	}

	if period != "" {
		if _, err := ValidatePeriod(period); err != nil {
			return nil, fmt.Errorf("validate period: %w", ErrValidation)
		}
		startTime := period + "-01T00:00:00+07:00"
		year := period[:4]
		month := period[5:7]
		endTime := nextMonth(startTime, year, month)

		err = tx.QueryRowContext(ctx,
			`SELECT
				COALESCE(SUM(CASE WHEN type = 'income' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text,
				COALESCE(SUM(CASE WHEN type = 'expense' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text,
				COUNT(CASE WHEN status = 'posted' THEN 1 END)
			FROM rw_transactions
			WHERE rw_id = $1
			  AND status = 'posted'
			  AND occurred_at >= $2 AND occurred_at < $3`,
			rwID, startTime, endTime,
		).Scan(&report.TotalIncome, &report.TotalExpense, &txCount)
		if err != nil {
			return nil, fmt.Errorf("kas period report: %w", err)
		}
	} else {
		err = tx.QueryRowContext(ctx,
			`SELECT
				COALESCE(SUM(CASE WHEN type = 'income' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text,
				COALESCE(SUM(CASE WHEN type = 'expense' AND status = 'posted' THEN amount ELSE 0 END), 0)::numeric(15,2)::text,
				COUNT(CASE WHEN status = 'posted' THEN 1 END)
			FROM rw_transactions
			WHERE rw_id = $1
			  AND status = 'posted'`,
			rwID,
		).Scan(&report.TotalIncome, &report.TotalExpense, &txCount)
		if err != nil {
			return nil, fmt.Errorf("kas all-time report: %w", err)
		}
	}

	report.TransactionCount = int64(txCount)

	oc, _, err := ParseMoney(report.OpeningBalance)
	if err != nil {
		return nil, fmt.Errorf("parse opening balance: %w", err)
	}
	incCents, _, err := ParseMoney(report.TotalIncome)
	if err != nil {
		return nil, fmt.Errorf("parse income: %w", err)
	}
	expCents, _, err := ParseMoney(report.TotalExpense)
	if err != nil {
		return nil, fmt.Errorf("parse expense: %w", err)
	}
	report.NetBalance = FormatMoney(oc + incCents - expCents)

	return &report, nil
}

// =========================================================================
// RW Fund Locations CRUD & Queries
// =========================================================================

func (r *Repository) CreateLocation(ctx context.Context, tx *sql.Tx, rwID string, in CreateRWFundLocationInput) (*LocationSummary, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	// 1. Check duplicate name per RW (case-insensitive & trimmed)
	var dupExists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM rw_fund_locations
			WHERE rw_id = $1 AND LOWER(TRIM(name)) = LOWER(TRIM($2))
		)`, rwID, in.Name,
	).Scan(&dupExists)
	if err != nil {
		return nil, fmt.Errorf("check duplicate name: %w", err)
	}
	if dupExists {
		return nil, ErrDuplicateName
	}

	// 2. Insert into rw_fund_locations
	var loc LocationSummary
	var bankName, accNo, accHolder sql.NullString
	var createdAt, updatedAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO rw_fund_locations (rw_id, name, category, bank_name, account_number, account_holder)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, rw_id, name, category, bank_name, account_number, account_holder, is_active, created_at, updated_at`,
		rwID, in.Name, string(in.Category), in.BankName, in.AccountNumber, in.AccountHolder,
	).Scan(&loc.ID, &loc.RWID, &loc.Name, &loc.Category, &bankName, &accNo, &accHolder, &loc.IsActive, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert rw_fund_location: %w", err)
	}

	loc.CurrentBalance = "0.00"
	loc.CreatedAt = createdAt.Format(time.RFC3339)
	loc.UpdatedAt = updatedAt.Format(time.RFC3339)
	if bankName.Valid {
		loc.BankName = &bankName.String
	}
	if accNo.Valid {
		loc.AccountNumber = &accNo.String
	}
	if accHolder.Valid {
		loc.AccountHolder = &accHolder.String
	}

	return &loc, nil
}

func (r *Repository) GetLocations(ctx context.Context, tx *sql.Tx, rwID string, isActive *bool) ([]*LocationSummary, error) {
	return r.GetLocationsWithFilter(ctx, tx, rwID, isActive, nil)
}

func (r *Repository) GetLocationsWithFilter(ctx context.Context, tx *sql.Tx, rwID string, isActive *bool, category *string) ([]*LocationSummary, error) {
	// Query strictly dedicated rw_fund_locations for this RW
	query := `
		SELECT fl.id, fl.rw_id, fl.name, fl.category,
			fl.bank_name, fl.account_number, fl.account_holder, fl.is_active,
			COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text as current_balance,
			fl.created_at::text, fl.updated_at::text
		FROM rw_fund_locations fl
		LEFT JOIN rw_location_balance_changes lbc ON fl.id = lbc.location_id
		WHERE fl.rw_id = $1`
	args := []interface{}{rwID}
	pos := 2

	if isActive != nil {
		query += fmt.Sprintf(" AND fl.is_active = $%d", pos)
		args = append(args, *isActive)
		pos++
	}
	if category != nil && *category != "" && *category != "all" {
		query += fmt.Sprintf(" AND fl.category = $%d", pos)
		args = append(args, *category)
		pos++
	}

	query += ` GROUP BY fl.id ORDER BY fl.category, fl.name ASC`

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query locations: %w", err)
	}
	defer rows.Close()

	var list []*LocationSummary
	for rows.Next() {
		var loc LocationSummary
		var bankName, accNo, accHolder sql.NullString
		var bal, createdAt, updatedAt string
		err := rows.Scan(&loc.ID, &loc.RWID, &loc.Name,
			&loc.Category, &bankName, &accNo, &accHolder, &loc.IsActive, &bal, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan location: %w", err)
		}
		loc.CurrentBalance = bal
		loc.CreatedAt = createdAt
		loc.UpdatedAt = updatedAt
		if bankName.Valid {
			loc.BankName = &bankName.String
		}
		if accNo.Valid {
			loc.AccountNumber = &accNo.String
		}
		if accHolder.Valid {
			loc.AccountHolder = &accHolder.String
		}
		list = append(list, &loc)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("locations iteration: %w", err)
	}
	return list, nil
}

func (r *Repository) GetLocationByID(ctx context.Context, tx *sql.Tx, rwID, locID string) (*LocationSummary, error) {
	query := `
		SELECT fl.id, fl.rw_id, fl.name, fl.category,
			fl.bank_name, fl.account_number, fl.account_holder, fl.is_active,
			COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text as current_balance,
			fl.created_at::text, fl.updated_at::text
		FROM rw_fund_locations fl
		LEFT JOIN rw_location_balance_changes lbc ON fl.id = lbc.location_id
		WHERE fl.rw_id = $1 AND fl.id = $2
		GROUP BY fl.id`

	var loc LocationSummary
	var bankName, accNo, accHolder sql.NullString
	var bal, createdAt, updatedAt string
	err := tx.QueryRowContext(ctx, query, rwID, locID).Scan(
		&loc.ID, &loc.RWID, &loc.Name, &loc.Category,
		&bankName, &accNo, &accHolder, &loc.IsActive, &bal, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get location by id: %w", err)
	}

	loc.CurrentBalance = bal
	loc.CreatedAt = createdAt
	loc.UpdatedAt = updatedAt
	if bankName.Valid {
		loc.BankName = &bankName.String
	}
	if accNo.Valid {
		loc.AccountNumber = &accNo.String
	}
	if accHolder.Valid {
		loc.AccountHolder = &accHolder.String
	}

	return &loc, nil
}

func (r *Repository) UpdateLocation(ctx context.Context, tx *sql.Tx, rwID, locID string, in UpdateRWFundLocationInput) (*LocationSummary, error) {
	existing, err := r.GetLocationByID(ctx, tx, rwID, locID)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		trimmedName := strings.TrimSpace(*in.Name)
		if trimmedName == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		if !strings.EqualFold(trimmedName, existing.Name) {
			var dupExists bool
			err = tx.QueryRowContext(ctx,
				`SELECT EXISTS(
					SELECT 1 FROM rw_fund_locations
					WHERE rw_id = $1 AND id != $2 AND LOWER(TRIM(name)) = LOWER(TRIM($3))
				)`, rwID, locID, trimmedName,
			).Scan(&dupExists)
			if err != nil {
				return nil, fmt.Errorf("check duplicate name on update: %w", err)
			}
			if dupExists {
				return nil, ErrDuplicateName
			}
		}
		existing.Name = trimmedName
	}

	if existing.Category == CategoryRekening {
		if in.BankName != nil {
			trimmedBank := strings.TrimSpace(*in.BankName)
			if trimmedBank == "" {
				return nil, ErrMissingBankName
			}
			existing.BankName = &trimmedBank
		}
	} else {
		existing.BankName = nil
	}

	if in.AccountNumber != nil {
		trimmed := strings.TrimSpace(*in.AccountNumber)
		existing.AccountNumber = &trimmed
	}
	if in.AccountHolder != nil {
		trimmed := strings.TrimSpace(*in.AccountHolder)
		existing.AccountHolder = &trimmed
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE rw_fund_locations
		 SET name = $1, bank_name = $2, account_number = $3, account_holder = $4, updated_at = CURRENT_TIMESTAMP
		 WHERE rw_id = $5 AND id = $6`,
		existing.Name, existing.BankName, existing.AccountNumber, existing.AccountHolder, rwID, locID,
	)
	if err != nil {
		return nil, fmt.Errorf("update rw_fund_location: %w", err)
	}

	return r.GetLocationByID(ctx, tx, rwID, locID)
}

func (r *Repository) ToggleLocationStatus(ctx context.Context, tx *sql.Tx, rwID, locID string, isActive bool) (*LocationSummary, error) {
	loc, err := r.GetLocationByID(ctx, tx, rwID, locID)
	if err != nil {
		return nil, err
	}

	if !isActive {
		balCents, _, err := ParseMoney(loc.CurrentBalance)
		if err != nil {
			return nil, fmt.Errorf("parse current balance: %w", err)
		}
		if balCents > 0 {
			return nil, ErrCannotDeactivateWithBalance
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE rw_fund_locations
		 SET is_active = $1, updated_at = CURRENT_TIMESTAMP
		 WHERE rw_id = $2 AND id = $3`,
		isActive, rwID, locID,
	)
	if err != nil {
		return nil, fmt.Errorf("toggle location status: %w", err)
	}

	return r.GetLocationByID(ctx, tx, rwID, locID)
}

func (r *Repository) GetLocationMutations(ctx context.Context, tx *sql.Tx, rwID, locID string, page, pageSize int) ([]LocationBalanceChange, int, error) {
	_, err := r.GetLocationByID(ctx, tx, rwID, locID)
	if err != nil {
		return nil, 0, err
	}

	var total int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_location_balance_changes WHERE rw_id = $1 AND location_id = $2`,
		rwID, locID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count mutations: %w", err)
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, rw_id, location_id, transaction_id, transfer_id, change_type,
		        amount_delta::numeric(15,2)::text, notes, created_by, created_at
		 FROM rw_location_balance_changes
		 WHERE rw_id = $1 AND location_id = $2
		 ORDER BY created_at DESC, id DESC
		 LIMIT $3 OFFSET $4`,
		rwID, locID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query mutations: %w", err)
	}
	defer rows.Close()

	var list []LocationBalanceChange
	for rows.Next() {
		var m LocationBalanceChange
		var txID, transferID, notes, createdBy sql.NullString
		err := rows.Scan(&m.ID, &m.RWID, &m.LocationID, &txID, &transferID, &m.ChangeType,
			&m.AmountDelta, &notes, &createdBy, &m.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan mutation: %w", err)
		}
		if txID.Valid {
			m.TransactionID = &txID.String
		}
		if transferID.Valid {
			m.TransferID = &transferID.String
		}
		if notes.Valid {
			m.Notes = &notes.String
		}
		if createdBy.Valid {
			m.CreatedBy = &createdBy.String
		}
		list = append(list, m)
	}

	return list, total, nil
}

func (r *Repository) GetReconciliationReport(ctx context.Context, tx *sql.Tx, rwID string) (*ReconciliationReport, error) {
	kasReport, err := r.GetKASReport(ctx, tx, rwID, "")
	if err != nil {
		return nil, err
	}

	var totalAllocatedStr string
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text
		 FROM rw_location_balance_changes lbc
		 JOIN rw_fund_locations fl ON lbc.location_id = fl.id
		 WHERE fl.rw_id = $1`, rwID,
	).Scan(&totalAllocatedStr)
	if err != nil {
		return nil, fmt.Errorf("query total allocated balance: %w", err)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT fl.category,
		        COALESCE(SUM(lbc.amount_delta), 0)::numeric(15,2)::text as cat_balance,
		        COUNT(DISTINCT fl.id) as loc_count
		 FROM rw_fund_locations fl
		 LEFT JOIN rw_location_balance_changes lbc ON fl.id = lbc.location_id
		 WHERE fl.rw_id = $1
		 GROUP BY fl.category
		 ORDER BY fl.category ASC`, rwID,
	)
	if err != nil {
		return nil, fmt.Errorf("query category summaries: %w", err)
	}
	defer rows.Close()

	var summaries []CategorySummary
	for rows.Next() {
		var cs CategorySummary
		if err := rows.Scan(&cs.Category, &cs.TotalBalance, &cs.LocationsCount); err != nil {
			return nil, fmt.Errorf("scan category summary: %w", err)
		}
		summaries = append(summaries, cs)
	}

	kasCents, _, _ := ParseMoney(kasReport.NetBalance)
	allocCents, _, _ := ParseMoney(totalAllocatedStr)
	discCents := kasCents - allocCents

	status := "matched"
	statusLabel := "Saldo Sesuai"
	var anomalies []string

	if discCents != 0 {
		status = "discrepant"
		statusLabel = "Selisih Terdeteksi"
		anomalies = append(anomalies, fmt.Sprintf("Selisih antara saldo kas RW dan total alokasi lokasi dana: %s", FormatMoney(discCents)))
	}

	unallocatedCents := discCents
	if unallocatedCents < 0 {
		unallocatedCents = 0
	}

	return &ReconciliationReport{
		KasBalance:            kasReport.NetBalance,
		TotalAllocatedBalance: totalAllocatedStr,
		UnallocatedBalance:    FormatMoney(unallocatedCents),
		Discrepancy:           FormatMoney(discCents),
		Status:                status,
		StatusLabel:           statusLabel,
		LocationsSummary:      summaries,
		AnomaliesDetected:     anomalies,
	}, nil
}

// =========================================================================
// RW Fund Transfers
// =========================================================================

func (r *Repository) CreateTransfer(ctx context.Context, tx *sql.Tx, rwID, userID string, in CreateFundTransferInput) (*FundTransfer, error) {
	if in.FromLocationID == in.ToLocationID {
		return nil, ErrSameLocationTransfer
	}

	amtCents, _, err := ParseMoney(in.Amount)
	if err != nil || amtCents <= 0 {
		return nil, ErrInvalidAmount
	}

	// 1. Lock both locations in consistent ascending order to serialize and eliminate deadlock
	firstID, secondID := in.FromLocationID, in.ToLocationID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	if _, err := tx.ExecContext(ctx, `SELECT id FROM rw_fund_locations WHERE rw_id = $1 AND id IN ($2, $3) ORDER BY id FOR UPDATE`, rwID, firstID, secondID); err != nil {
		return nil, fmt.Errorf("lock fund locations: %w", err)
	}

	// Validate source location (must belong to rwID and be active)
	fromLoc, err := r.GetLocationByID(ctx, tx, rwID, in.FromLocationID)
	if err != nil {
		return nil, fmt.Errorf("source location: %w", err)
	}
	if !fromLoc.IsActive {
		return nil, fmt.Errorf("%w: source location %s is inactive", ErrInactiveLocation, fromLoc.Name)
	}

	// 2. Validate destination location (must belong to SAME rwID and be active)
	toLoc, err := r.GetLocationByID(ctx, tx, rwID, in.ToLocationID)
	if err != nil {
		return nil, fmt.Errorf("destination location: %w", err)
	}
	if !toLoc.IsActive {
		return nil, fmt.Errorf("%w: destination location %s is inactive", ErrInactiveLocation, toLoc.Name)
	}

	// Double-check cross-RW enforcement (defense-in-depth)
	if fromLoc.RWID != rwID || toLoc.RWID != rwID {
		return nil, ErrCrossRWTransfer
	}

	// 3. Balance check on source location
	fromBalCents, _, err := ParseMoney(fromLoc.CurrentBalance)
	if err != nil {
		return nil, fmt.Errorf("parse source balance: %w", err)
	}
	if fromBalCents < amtCents {
		return nil, ErrInsufficientFunds
	}

	// 4. Insert transfer row
	var tDate time.Time
	if in.TransferDate != nil && *in.TransferDate != "" {
		parsed, err := time.Parse(time.RFC3339, *in.TransferDate)
		if err == nil {
			tDate = parsed
		} else {
			tDate = time.Now()
		}
	} else {
		tDate = time.Now()
	}

	var tr FundTransfer
	var notes, createdBy sql.NullString
	var reversesID sql.NullString
	err = tx.QueryRowContext(ctx,
		`INSERT INTO rw_fund_transfers (rw_id, from_location_id, to_location_id, amount, transfer_date, notes, created_by)
		 VALUES ($1, $2, $3, $4::numeric(15,2), $5, $6, $7)
		 RETURNING id, rw_id, from_location_id, to_location_id, amount::numeric(15,2)::text,
		           transfer_date, notes, created_by, reverses_transfer_id, created_at`,
		rwID, in.FromLocationID, in.ToLocationID, in.Amount, tDate, in.Notes, userID,
	).Scan(&tr.ID, &tr.RWID, &tr.FromLocationID, &tr.ToLocationID, &tr.Amount,
		&tr.TransferDate, &notes, &createdBy, &reversesID, &tr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert rw_fund_transfer: %w", err)
	}

	tr.FromLocationName = &fromLoc.Name
	tr.ToLocationName = &toLoc.Name
	if notes.Valid {
		tr.Notes = &notes.String
	}
	if createdBy.Valid {
		tr.CreatedBy = &createdBy.String
	}

	// 5. Insert 2 immutable mutations in rw_location_balance_changes
	// Source location: transfer_out (-amount)
	transferNotesOut := fmt.Sprintf("Transfer ke %s", toLoc.Name)
	if in.Notes != nil && *in.Notes != "" {
		transferNotesOut = fmt.Sprintf("Transfer ke %s: %s", toLoc.Name, *in.Notes)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (rw_id, location_id, transfer_id, change_type, amount_delta, notes, created_by)
		 VALUES ($1, $2, $3, 'transfer_out', -1 * $4::numeric(15,2), $5, $6)`,
		rwID, in.FromLocationID, tr.ID, in.Amount, transferNotesOut, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert transfer_out mutation: %w", err)
	}

	// Destination location: transfer_in (+amount)
	transferNotesIn := fmt.Sprintf("Transfer dari %s", fromLoc.Name)
	if in.Notes != nil && *in.Notes != "" {
		transferNotesIn = fmt.Sprintf("Transfer dari %s: %s", fromLoc.Name, *in.Notes)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (rw_id, location_id, transfer_id, change_type, amount_delta, notes, created_by)
		 VALUES ($1, $2, $3, 'transfer_in', $4::numeric(15,2), $5, $6)`,
		rwID, in.ToLocationID, tr.ID, in.Amount, transferNotesIn, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert transfer_in mutation: %w", err)
	}

	return &tr, nil
}

func (r *Repository) ReverseTransfer(ctx context.Context, tx *sql.Tx, rwID, userID, transferID, reason string) (*FundTransfer, error) {
	// 1. Fetch and lock original transfer row to serialize concurrent reversals
	var orig FundTransfer
	var origNotes, origCreatedBy, origReversesID sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT id, rw_id, from_location_id, to_location_id, amount::numeric(15,2)::text,
		        transfer_date, notes, created_by, reverses_transfer_id, created_at
		 FROM rw_fund_transfers
		 WHERE rw_id = $1 AND id = $2
		 FOR UPDATE`,
		rwID, transferID,
	).Scan(&orig.ID, &orig.RWID, &orig.FromLocationID, &orig.ToLocationID, &orig.Amount,
		&orig.TransferDate, &origNotes, &origCreatedBy, &origReversesID, &orig.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transfer to reverse: %w", err)
	}

	if origReversesID.Valid && origReversesID.String != "" {
		return nil, ErrCannotReverseReversal
	}

	// Check if already reversed
	var alreadyReversed bool
	err = tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM rw_fund_transfers WHERE rw_id = $1 AND reverses_transfer_id = $2)`,
		rwID, transferID,
	).Scan(&alreadyReversed)
	if err != nil {
		return nil, fmt.Errorf("check already reversed: %w", err)
	}
	if alreadyReversed {
		return nil, ErrAlreadyReversed
	}

	// 2. Lock both locations in consistent ascending order to serialize and eliminate deadlock
	firstID, secondID := orig.FromLocationID, orig.ToLocationID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	if _, err := tx.ExecContext(ctx, `SELECT id FROM rw_fund_locations WHERE rw_id = $1 AND id IN ($2, $3) ORDER BY id FOR UPDATE`, rwID, firstID, secondID); err != nil {
		return nil, fmt.Errorf("lock fund locations: %w", err)
	}

	// Fetch recipient location and check that it has sufficient funds to refund
	toLoc, err := r.GetLocationByID(ctx, tx, rwID, orig.ToLocationID)
	if err != nil {
		return nil, fmt.Errorf("recipient location for reversal: %w", err)
	}
	fromLoc, err := r.GetLocationByID(ctx, tx, rwID, orig.FromLocationID)
	if err != nil {
		return nil, fmt.Errorf("source location for reversal: %w", err)
	}

	toBalCents, _, err := ParseMoney(toLoc.CurrentBalance)
	if err != nil {
		return nil, fmt.Errorf("parse recipient balance: %w", err)
	}
	amtCents, _, err := ParseMoney(orig.Amount)
	if err != nil {
		return nil, fmt.Errorf("parse transfer amount: %w", err)
	}
	if toBalCents < amtCents {
		return nil, ErrInsufficientReversal
	}

	// 3. Insert reversal transfer row
	revNotes := fmt.Sprintf("Reversal of %s. Reason: %s", orig.ID, strings.TrimSpace(reason))
	var rev FundTransfer
	var revNotesOut, revCreatedBy sql.NullString
	var revReversesID sql.NullString
	err = tx.QueryRowContext(ctx,
		`INSERT INTO rw_fund_transfers (rw_id, from_location_id, to_location_id, amount, transfer_date, notes, created_by, reverses_transfer_id)
		 VALUES ($1, $2, $3, $4::numeric(15,2), CURRENT_TIMESTAMP, $5, $6, $7)
		 RETURNING id, rw_id, from_location_id, to_location_id, amount::numeric(15,2)::text,
		           transfer_date, notes, created_by, reverses_transfer_id, created_at`,
		rwID, orig.ToLocationID, orig.FromLocationID, orig.Amount, revNotes, userID, orig.ID,
	).Scan(&rev.ID, &rev.RWID, &rev.FromLocationID, &rev.ToLocationID, &rev.Amount,
		&rev.TransferDate, &revNotesOut, &revCreatedBy, &revReversesID, &rev.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert reversal transfer: %w", err)
	}

	rev.FromLocationName = &toLoc.Name
	rev.ToLocationName = &fromLoc.Name
	if revNotesOut.Valid {
		rev.Notes = &revNotesOut.String
	}
	if revCreatedBy.Valid {
		rev.CreatedBy = &revCreatedBy.String
	}
	if revReversesID.Valid {
		rev.ReversesTransferID = &revReversesID.String
	}

	// 4. Insert 2 reversal mutations in rw_location_balance_changes
	// Source was credited originally (transfer_in); now reverse it: reversal_transfer_in (-amount) on recipient
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (rw_id, location_id, transfer_id, change_type, amount_delta, notes, created_by)
		 VALUES ($1, $2, $3, 'reversal_transfer_in', -1 * $4::numeric(15,2), $5, $6)`,
		rwID, orig.ToLocationID, rev.ID, orig.Amount, fmt.Sprintf("Pembatalan transfer: kembalikan dana ke %s", fromLoc.Name), userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert reversal_transfer_in mutation: %w", err)
	}

	// Source was debited originally (transfer_out); now restore funds: reversal_transfer_out (+amount) on sender
	_, err = tx.ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (rw_id, location_id, transfer_id, change_type, amount_delta, notes, created_by)
		 VALUES ($1, $2, $3, 'reversal_transfer_out', $4::numeric(15,2), $5, $6)`,
		rwID, orig.FromLocationID, rev.ID, orig.Amount, fmt.Sprintf("Pembatalan transfer: dana dikembalikan dari %s", toLoc.Name), userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert reversal_transfer_out mutation: %w", err)
	}

	return &rev, nil
}

func (r *Repository) GetTransfers(ctx context.Context, tx *sql.Tx, rwID string, page, pageSize int) ([]FundTransfer, int, error) {
	var total int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rw_fund_transfers WHERE rw_id = $1`, rwID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count transfers: %w", err)
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT t.id, t.rw_id, t.from_location_id, fl1.name as from_name,
		        t.to_location_id, fl2.name as to_name,
		        t.amount::numeric(15,2)::text, t.transfer_date, t.notes, t.created_by,
		        t.reverses_transfer_id, t.created_at
		 FROM rw_fund_transfers t
		 JOIN rw_fund_locations fl1 ON t.from_location_id = fl1.id
		 JOIN rw_fund_locations fl2 ON t.to_location_id = fl2.id
		 WHERE t.rw_id = $1
		 ORDER BY t.transfer_date DESC, t.id DESC
		 LIMIT $2 OFFSET $3`,
		rwID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query transfers: %w", err)
	}
	defer rows.Close()

	var list []FundTransfer
	for rows.Next() {
		var tr FundTransfer
		var fromName, toName, notes, createdBy, revID sql.NullString
		err := rows.Scan(&tr.ID, &tr.RWID, &tr.FromLocationID, &fromName,
			&tr.ToLocationID, &toName, &tr.Amount, &tr.TransferDate,
			&notes, &createdBy, &revID, &tr.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan transfer: %w", err)
		}
		if fromName.Valid {
			tr.FromLocationName = &fromName.String
		}
		if toName.Valid {
			tr.ToLocationName = &toName.String
		}
		if notes.Valid {
			tr.Notes = &notes.String
		}
		if createdBy.Valid {
			tr.CreatedBy = &createdBy.String
		}
		if revID.Valid {
			tr.ReversesTransferID = &revID.String
		}
		list = append(list, tr)
	}

	return list, total, nil
}

// =========================================================================
// RW Transactions (KAS Masuk & Keluar)
// =========================================================================

func (r *Repository) CreateTransaction(ctx context.Context, tx *sql.Tx, rwID, userID string, in CreateTransactionInput) (*TransactionEntry, error) {
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))
	if in.Type != "income" && in.Type != "expense" {
		return nil, fmt.Errorf("%w: invalid transaction type (must be income or expense)", ErrValidation)
	}

	amtCents, _, err := ParseMoney(in.Amount)
	if err != nil || amtCents <= 0 {
		return nil, ErrInvalidAmount
	}

	in.Description = strings.TrimSpace(in.Description)
	if in.Description == "" {
		return nil, fmt.Errorf("%w: description is required", ErrValidation)
	}

	// 1. Lock location row to serialize concurrent mutations and verify existence
	var lockDummy int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM rw_fund_locations WHERE rw_id = $1 AND id = $2 FOR UPDATE`, rwID, in.LocationID).Scan(&lockDummy)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock fund location: %w", err)
	}

	// Verify fund location belongs to rwID and is active
	loc, err := r.GetLocationByID(ctx, tx, rwID, in.LocationID)
	if err != nil {
		return nil, fmt.Errorf("fund location: %w", err)
	}
	if !loc.IsActive {
		return nil, fmt.Errorf("%w: location %s is inactive", ErrInactiveLocation, loc.Name)
	}

	// 2. Validate payment method match with category
	var paymentMethod *string
	if in.PaymentMethod != nil && *in.PaymentMethod != "" {
		pm := strings.ToUpper(strings.TrimSpace(*in.PaymentMethod))
		if pm != "CASH" && pm != "TRANSFER" {
			return nil, fmt.Errorf("%w: invalid payment method (must be CASH or TRANSFER)", ErrValidation)
		}
		if loc.Category == CategoryRekening && pm != "TRANSFER" {
			return nil, fmt.Errorf("%w: rekening location requires TRANSFER payment method", ErrInvalidMethodCategoryMatch)
		}
		if (loc.Category == CategoryBrankas || loc.Category == CategoryCOH) && pm != "CASH" {
			return nil, fmt.Errorf("%w: cash/brankas location requires CASH payment method", ErrInvalidMethodCategoryMatch)
		}
		paymentMethod = &pm
	} else {
		// Auto-derive payment method from location category
		if loc.Category == CategoryRekening {
			pm := "TRANSFER"
			paymentMethod = &pm
		} else {
			pm := "CASH"
			paymentMethod = &pm
		}
	}

	// 3. For expense: check that location has sufficient balance
	if in.Type == "expense" {
		locBalCents, _, err := ParseMoney(loc.CurrentBalance)
		if err != nil {
			return nil, fmt.Errorf("parse location balance: %w", err)
		}
		if locBalCents < amtCents {
			return nil, ErrInsufficientFunds
		}
	}

	// 4. Validate source RT if provided (penerimaan dana dari RT ke RW)
	if in.SourceRTID != nil && *in.SourceRTID != "" {
		var rtExists bool
		err = tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM rts WHERE id = $1 AND is_active = true)`,
			*in.SourceRTID,
		).Scan(&rtExists)
		if err != nil {
			return nil, fmt.Errorf("validate source rt: %w", err)
		}
		if !rtExists {
			return nil, fmt.Errorf("%w: source RT not found or inactive", ErrValidation)
		}
	}

	var occDate time.Time
	if in.OccurredAt != nil && *in.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, *in.OccurredAt)
		if err == nil {
			occDate = parsed
		} else {
			occDate = time.Now()
		}
	} else {
		occDate = time.Now()
	}

	// 5. Insert transaction into rw_transactions
	var entry TransactionEntry
	var catID, payID, srcRTID sql.NullString
	var occAt, createdAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO rw_transactions (
			rw_id, location_id, source_rt_id, category_id, amount, type, status,
			description, occurred_at, payment_method, created_by
		) VALUES (
			$1, $2, $3, $4, $5::numeric(15,2), $6, 'posted',
			$7, $8, $9, $10
		) RETURNING id, rw_id, location_id, source_rt_id, category_id, amount::numeric(15,2)::text,
		            type, status, payment_id, description, occurred_at, created_at`,
		rwID, in.LocationID, in.SourceRTID, in.CategoryID, in.Amount, in.Type,
		in.Description, occDate, paymentMethod, userID,
	).Scan(&entry.ID, &entry.RWID, &entry.LocationID, &srcRTID, &catID, &entry.Amount,
		&entry.Type, &entry.Status, &payID, &entry.Description, &occAt, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert rw_transaction: %w", err)
	}

	entry.OccurredAt = occAt.Format(time.RFC3339)
	entry.CreatedAt = createdAt.Format(time.RFC3339)
	entry.LocationName = &loc.Name
	if catID.Valid {
		entry.CategoryID = &catID.String
	}
	if payID.Valid {
		entry.PaymentID = &payID.String
	}
	if srcRTID.Valid {
		entry.SourceRTID = &srcRTID.String
	}

	// 6. Insert corresponding balance change into rw_location_balance_changes
	changeType := in.Type
	var amountDelta string
	if in.Type == "income" {
		amountDelta = in.Amount
	} else {
		amountDelta = fmt.Sprintf("-%s", in.Amount)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rw_location_balance_changes (
			rw_id, location_id, transaction_id, change_type, amount_delta, notes, created_by
		) VALUES ($1, $2, $3, $4, $5::numeric(15,2), $6, $7)`,
		rwID, in.LocationID, entry.ID, changeType, amountDelta, in.Description, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert location_balance_change: %w", err)
	}

	return &entry, nil
}

func (r *Repository) GetTransactions(ctx context.Context, tx *sql.Tx, rwID string, period string, page, pageSize int, startDate, endDate string) ([]*TransactionEntry, int, error) {
	baseQuery := `
		SELECT t.id, t.rw_id, t.location_id, fl.name as location_name,
		       t.source_rt_id, rt.name as source_rt_name,
		       t.type, t.amount::numeric(15,2)::text, t.description,
		       t.category_id, c.name as category_name, t.status,
		       t.payment_id, t.occurred_at::text, t.created_at::text,
		       t.proof_path
		FROM rw_transactions t
		LEFT JOIN rw_fund_locations fl ON t.location_id = fl.id
		LEFT JOIN rts rt ON t.source_rt_id = rt.id
		LEFT JOIN financial_categories c ON t.category_id = c.id
		WHERE t.status = 'posted' AND t.rw_id = $1`
	args := []interface{}{rwID}
	pos := 2

	if period != "" {
		if _, err := ValidatePeriod(period); err != nil {
			return nil, 0, fmt.Errorf("validate period: %w", ErrValidation)
		}
		startTime := period + "-01T00:00:00+07:00"
		year := period[:4]
		month := period[5:7]
		endTime := nextMonth(startTime, year, month)
		baseQuery += fmt.Sprintf(" AND t.occurred_at >= $%d AND t.occurred_at < $%d", pos, pos+1)
		args = append(args, startTime, endTime)
		pos += 2
	}

	if startDate != "" {
		if _, err := ValidateDate(startDate); err != nil {
			return nil, 0, fmt.Errorf("validate start_date: %w", ErrValidation)
		}
		baseQuery += fmt.Sprintf(" AND DATE(t.occurred_at) >= $%d", pos)
		args = append(args, startDate)
		pos++
	}
	if endDate != "" {
		if _, err := ValidateDate(endDate); err != nil {
			return nil, 0, fmt.Errorf("validate end_date: %w", ErrValidation)
		}
		baseQuery += fmt.Sprintf(" AND DATE(t.occurred_at) <= $%d", pos)
		args = append(args, endDate)
		pos++
	}

	baseQuery += " ORDER BY t.occurred_at DESC, t.id DESC"

	countQuery := `SELECT COUNT(*) FROM rw_transactions t WHERE t.status = 'posted' AND t.rw_id = $1`
	countArgs := []interface{}{rwID}
	cpos := 2

	if period != "" {
		startTime := period + "-01T00:00:00+07:00"
		year := period[:4]
		month := period[5:7]
		endTime := nextMonth(startTime, year, month)
		countQuery += fmt.Sprintf(" AND t.occurred_at >= $%d AND t.occurred_at < $%d", cpos, cpos+1)
		countArgs = append(countArgs, startTime, endTime)
		cpos += 2
	}
	if startDate != "" {
		countQuery += fmt.Sprintf(" AND DATE(t.occurred_at) >= $%d", cpos)
		countArgs = append(countArgs, startDate)
		cpos++
	}
	if endDate != "" {
		countQuery += fmt.Sprintf(" AND DATE(t.occurred_at) <= $%d", cpos)
		countArgs = append(countArgs, endDate)
		cpos++
	}

	var total int
	err := tx.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", pos, pos+1)
	args = append(args, pageSize, offset)

	rows, err := tx.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var list []*TransactionEntry
	for rows.Next() {
		var entry TransactionEntry
		var locID, locName, srcRTID, srcRTName sql.NullString
		var categoryID, categoryName sql.NullString
		var paymentID, proofPath sql.NullString
		err := rows.Scan(
			&entry.ID, &entry.RWID, &locID, &locName, &srcRTID, &srcRTName,
			&entry.Type, &entry.Amount, &entry.Description,
			&categoryID, &categoryName, &entry.Status, &paymentID, &entry.OccurredAt, &entry.CreatedAt,
			&proofPath,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		if locID.Valid {
			entry.LocationID = &locID.String
		}
		if locName.Valid {
			entry.LocationName = &locName.String
		}
		if srcRTID.Valid {
			entry.SourceRTID = &srcRTID.String
		}
		if srcRTName.Valid {
			entry.SourceRTName = &srcRTName.String
		}
		if categoryID.Valid {
			entry.CategoryID = &categoryID.String
		}
		if categoryName.Valid {
			entry.CategoryName = &categoryName.String
		}
		if paymentID.Valid {
			entry.PaymentID = &paymentID.String
		}
		if proofPath.Valid && proofPath.String != "" {
			entry.ProofPath = &proofPath.String
			url := fmt.Sprintf("/api/v1/rw/%s/kas/transactions/%s/proof", rwID, entry.ID)
			entry.ProofURL = &url
		}
		list = append(list, &entry)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("transactions iteration: %w", err)
	}

	return list, total, nil
}

func (r *Repository) GetTransactionByID(ctx context.Context, tx *sql.Tx, rwID, id string) (*TransactionEntry, error) {
	query := `
		SELECT t.id, t.rw_id, t.location_id, fl.name as location_name,
		       t.source_rt_id, rt.name as source_rt_name,
		       t.type, t.amount::numeric(15,2)::text, t.description,
		       t.category_id, c.name as category_name, t.status,
		       t.payment_id, t.occurred_at::text, t.created_at::text,
		       t.proof_path
		FROM rw_transactions t
		LEFT JOIN rw_fund_locations fl ON t.location_id = fl.id
		LEFT JOIN rts rt ON t.source_rt_id = rt.id
		LEFT JOIN financial_categories c ON t.category_id = c.id
		WHERE t.rw_id = $1 AND t.id = $2`

	var entry TransactionEntry
	var locID, locName, srcRTID, srcRTName sql.NullString
	var categoryID, categoryName sql.NullString
	var paymentID, proofPath sql.NullString
	err := tx.QueryRowContext(ctx, query, rwID, id).Scan(
		&entry.ID, &entry.RWID, &locID, &locName, &srcRTID, &srcRTName,
		&entry.Type, &entry.Amount, &entry.Description,
		&categoryID, &categoryName, &entry.Status, &paymentID, &entry.OccurredAt, &entry.CreatedAt,
		&proofPath,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction by id: %w", err)
	}

	if locID.Valid {
		entry.LocationID = &locID.String
	}
	if locName.Valid {
		entry.LocationName = &locName.String
	}
	if srcRTID.Valid {
		entry.SourceRTID = &srcRTID.String
	}
	if srcRTName.Valid {
		entry.SourceRTName = &srcRTName.String
	}
	if categoryID.Valid {
		entry.CategoryID = &categoryID.String
	}
	if categoryName.Valid {
		entry.CategoryName = &categoryName.String
	}
	if paymentID.Valid {
		entry.PaymentID = &paymentID.String
	}
	if proofPath.Valid && proofPath.String != "" {
		entry.ProofPath = &proofPath.String
		url := fmt.Sprintf("/api/v1/rw/%s/kas/transactions/%s/proof", rwID, entry.ID)
		entry.ProofURL = &url
	}

	return &entry, nil
}

// AttachTransactionProof saves the uploaded proof metadata for an RW transaction.
func (r *Repository) AttachTransactionProof(ctx context.Context, tx *sql.Tx, rwID, txID, userID string, meta *storage.ObjectMetadata) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE rw_transactions SET
			proof_path = $1,
			proof_original_name = $2,
			proof_mime_type = $3,
			proof_size_bytes = $4,
			proof_uploaded_at = NOW(),
			proof_uploaded_by = $5
		WHERE rw_id = $6 AND id = $7
	`, meta.Key, meta.OriginalName, meta.ContentType, meta.Size, userID, rwID, txID)
	if err != nil {
		return fmt.Errorf("update transaction proof: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetTransactionProofInfo returns the proof path and mime type for an RW transaction.
func (r *Repository) GetTransactionProofInfo(ctx context.Context, tx *sql.Tx, rwID, txID string) (proofPath, mimeType string, err error) {
	var pPath, mMime sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT proof_path, proof_mime_type
		FROM rw_transactions
		WHERE rw_id = $1 AND id = $2
	`, rwID, txID).Scan(&pPath, &mMime)
	if err == sql.ErrNoRows {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("get transaction proof: %w", err)
	}
	if !pPath.Valid || pPath.String == "" {
		return "", "", ErrNotFound
	}
	return pPath.String, mMime.String, nil
}

func nextMonth(_ string, year, month string) string {
	var y, m int
	_, err := fmt.Sscanf(year, "%d", &y)
	if err != nil {
		y = 0
	}
	_, err = fmt.Sscanf(month, "%d", &m)
	if err != nil {
		m = 0
	}
	if m >= 12 {
		return fmt.Sprintf("%d-01-01T00:00:00+07:00", y+1)
	}
	return fmt.Sprintf("%d-%02d-01T00:00:00+07:00", y, m+1)
}
