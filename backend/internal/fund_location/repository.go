package fundlocation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"social-finance/internal/finance"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

func (r *Repository) CreateLocation(ctx context.Context, tx *sql.Tx, rtID string, in CreateFundLocationInput) (*FundLocation, error) {
	var loc FundLocation
	var bankName, accNo, accHolder sql.NullString
	err := tx.QueryRowContext(ctx,
		`INSERT INTO fund_locations (rt_id, name, category, bank_name, account_number, account_holder, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, true)
		 RETURNING id, rt_id, name, category, bank_name, account_number, account_holder, is_active, created_at, updated_at`,
		rtID, in.Name, in.Category, in.BankName, in.AccountNumber, in.AccountHolder,
	).Scan(&loc.ID, &loc.RTID, &loc.Name, &loc.Category, &bankName, &accNo, &accHolder, &loc.IsActive, &loc.CreatedAt, &loc.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "uniq_fund_locations_rt_lower_name") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("create fund location: %w", err)
	}
	if bankName.Valid {
		loc.BankName = &bankName.String
	}
	if accNo.Valid {
		loc.AccountNumber = &accNo.String
	}
	if accHolder.Valid {
		loc.AccountHolder = &accHolder.String
	}
	loc.CurrentBalance = "0.00"
	return &loc, nil
}

func (r *Repository) GetLocationByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*FundLocation, error) {
	var loc FundLocation
	var bankName, accNo, accHolder sql.NullString
	var bal string
	err := tx.QueryRowContext(ctx,
		`SELECT fl.id, fl.rt_id, fl.name, fl.category, fl.bank_name, fl.account_number, fl.account_holder, fl.is_active,
		        fl.created_at, fl.updated_at,
		        COALESCE(SUM(lbc.amount_delta), 0)::text as current_balance
		 FROM fund_locations fl
		 LEFT JOIN location_balance_changes lbc ON fl.id = lbc.location_id
		 WHERE fl.id = $1 AND fl.rt_id = $2
		 GROUP BY fl.id`,
		id, rtID,
	).Scan(&loc.ID, &loc.RTID, &loc.Name, &loc.Category, &bankName, &accNo, &accHolder, &loc.IsActive, &loc.CreatedAt, &loc.UpdatedAt, &bal)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get fund location by id: %w", err)
	}
	if bankName.Valid {
		loc.BankName = &bankName.String
	}
	if accNo.Valid {
		loc.AccountNumber = &accNo.String
	}
	if accHolder.Valid {
		loc.AccountHolder = &accHolder.String
	}
	if _, canon, err := finance.ParseMoney(bal); err == nil {
		loc.CurrentBalance = canon
	} else {
		loc.CurrentBalance = bal
	}
	return &loc, nil
}

func (r *Repository) ListLocationsByRT(ctx context.Context, tx *sql.Tx, rtID string, category *string, isActive *bool) ([]*FundLocation, error) {
	query := `SELECT fl.id, fl.rt_id, fl.name, fl.category, fl.bank_name, fl.account_number, fl.account_holder, fl.is_active,
	                 fl.created_at, fl.updated_at,
	                 COALESCE(SUM(lbc.amount_delta), 0)::text as current_balance
	          FROM fund_locations fl
	          LEFT JOIN location_balance_changes lbc ON fl.id = lbc.location_id
	          WHERE fl.rt_id = $1`
	args := []interface{}{rtID}
	pos := 2

	if category != nil && *category != "" {
		query += fmt.Sprintf(" AND fl.category = $%d", pos)
		args = append(args, *category)
		pos++
	}
	if isActive != nil {
		query += fmt.Sprintf(" AND fl.is_active = $%d", pos)
		args = append(args, *isActive)
		pos++
	}

	query += ` GROUP BY fl.id ORDER BY fl.category, fl.name ASC`

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list fund locations: %w", err)
	}
	defer rows.Close()

	var list []*FundLocation
	for rows.Next() {
		var loc FundLocation
		var bankName, accNo, accHolder sql.NullString
		var bal string
		if err := rows.Scan(&loc.ID, &loc.RTID, &loc.Name, &loc.Category, &bankName, &accNo, &accHolder, &loc.IsActive, &loc.CreatedAt, &loc.UpdatedAt, &bal); err != nil {
			return nil, err
		}
		if bankName.Valid {
			loc.BankName = &bankName.String
		}
		if accNo.Valid {
			loc.AccountNumber = &accNo.String
		}
		if accHolder.Valid {
			loc.AccountHolder = &accHolder.String
		}
		if _, canon, err := finance.ParseMoney(bal); err == nil {
			loc.CurrentBalance = canon
		} else {
			loc.CurrentBalance = bal
		}
		list = append(list, &loc)
	}
	return list, rows.Err()
}

func (r *Repository) UpdateLocation(ctx context.Context, tx *sql.Tx, id, rtID string, in UpdateFundLocationInput) (*FundLocation, error) {
	cur, err := r.GetLocationByID(ctx, tx, id, rtID)
	if err != nil {
		return nil, err
	}

	name := cur.Name
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		name = strings.TrimSpace(*in.Name)
	}
	bankName := cur.BankName
	if in.BankName != nil {
		trimmed := strings.TrimSpace(*in.BankName)
		bankName = &trimmed
	}
	accNo := cur.AccountNumber
	if in.AccountNumber != nil {
		trimmed := strings.TrimSpace(*in.AccountNumber)
		accNo = &trimmed
	}
	accHolder := cur.AccountHolder
	if in.AccountHolder != nil {
		trimmed := strings.TrimSpace(*in.AccountHolder)
		accHolder = &trimmed
	}

	var updated FundLocation
	var bName, aNo, aHolder sql.NullString
	err = tx.QueryRowContext(ctx,
		`UPDATE fund_locations
		 SET name = $1, bank_name = $2, account_number = $3, account_holder = $4, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $5 AND rt_id = $6
		 RETURNING id, rt_id, name, category, bank_name, account_number, account_holder, is_active, created_at, updated_at`,
		name, bankName, accNo, accHolder, id, rtID,
	).Scan(&updated.ID, &updated.RTID, &updated.Name, &updated.Category, &bName, &aNo, &aHolder, &updated.IsActive, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "uniq_fund_locations_rt_lower_name") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update fund location: %w", err)
	}
	if bName.Valid {
		updated.BankName = &bName.String
	}
	if aNo.Valid {
		updated.AccountNumber = &aNo.String
	}
	if aHolder.Valid {
		updated.AccountHolder = &aHolder.String
	}
	updated.CurrentBalance = cur.CurrentBalance
	return &updated, nil
}

func (r *Repository) DeactivateLocation(ctx context.Context, tx *sql.Tx, id, rtID string) (*FundLocation, error) {
	loc, err := r.GetLocationByID(ctx, tx, id, rtID)
	if err != nil {
		return nil, err
	}

	cents, _, err := finance.ParseMoney(loc.CurrentBalance)
	if err != nil {
		return nil, err
	}
	if cents > 0 {
		return nil, ErrCannotDeactivateWithBalance
	}

	var updated FundLocation
	var bName, aNo, aHolder sql.NullString
	err = tx.QueryRowContext(ctx,
		`UPDATE fund_locations
		 SET is_active = false, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND rt_id = $2
		 RETURNING id, rt_id, name, category, bank_name, account_number, account_holder, is_active, created_at, updated_at`,
		id, rtID,
	).Scan(&updated.ID, &updated.RTID, &updated.Name, &updated.Category, &bName, &aNo, &aHolder, &updated.IsActive, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("deactivate fund location: %w", err)
	}
	if bName.Valid {
		updated.BankName = &bName.String
	}
	if aNo.Valid {
		updated.AccountNumber = &aNo.String
	}
	if aHolder.Valid {
		updated.AccountHolder = &aHolder.String
	}
	updated.CurrentBalance = loc.CurrentBalance
	return &updated, nil
}

func (r *Repository) GetLocationBalance(ctx context.Context, tx *sql.Tx, locationID, rtID string) (string, int64, error) {
	var bal string
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_delta), 0)::text
		 FROM location_balance_changes
		 WHERE location_id = $1 AND rt_id = $2`,
		locationID, rtID,
	).Scan(&bal)
	if err != nil {
		return "0.00", 0, err
	}
	cents, canon, err := finance.ParseMoney(bal)
	if err != nil {
		return bal, 0, err
	}
	return canon, cents, nil
}

func (r *Repository) CreateTransfer(ctx context.Context, tx *sql.Tx, rtID, createdBy string, in CreateFundTransferInput) (*FundTransfer, error) {
	if in.FromLocationID == in.ToLocationID {
		return nil, ErrSameLocationTransfer
	}

	amtCents, canonAmt, err := finance.ParseMoney(in.Amount)
	if err != nil || amtCents <= 0 {
		return nil, ErrInvalidAmount
	}

	// Ordered locking to guarantee deadlock-free execution
	firstID, secondID := in.FromLocationID, in.ToLocationID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, is_active FROM fund_locations WHERE id IN ($1, $2) AND rt_id = $3 ORDER BY id FOR UPDATE`,
		firstID, secondID, rtID,
	)
	if err != nil {
		return nil, fmt.Errorf("lock locations: %w", err)
	}
	defer rows.Close()

	activeMap := make(map[string]bool)
	count := 0
	for rows.Next() {
		var id string
		var act bool
		if err := rows.Scan(&id, &act); err != nil {
			return nil, err
		}
		activeMap[id] = act
		count++
	}
	if count < 2 {
		return nil, errors.New("one or both locations not found in RT")
	}
	if !activeMap[in.FromLocationID] || !activeMap[in.ToLocationID] {
		return nil, ErrInactiveLocation
	}

	// Verify balance of source location
	_, fromCents, err := r.GetLocationBalance(ctx, tx, in.FromLocationID, rtID)
	if err != nil {
		return nil, err
	}
	if fromCents < amtCents {
		return nil, ErrInsufficientFunds
	}

	transferDate := time.Now()
	if in.TransferDate != nil && strings.TrimSpace(*in.TransferDate) != "" {
		if parsed, pErr := time.Parse(time.RFC3339, *in.TransferDate); pErr == nil {
			transferDate = parsed
		} else if parsedDate, dErr := time.Parse("2006-01-02", *in.TransferDate); dErr == nil {
			transferDate = parsedDate
		}
	}

	var tr FundTransfer
	var notesStr sql.NullString
	err = tx.QueryRowContext(ctx,
		`INSERT INTO fund_transfers (rt_id, from_location_id, to_location_id, amount, transfer_date, notes, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, rt_id, from_location_id, to_location_id, amount::text, transfer_date, notes, created_by::text, reverses_transfer_id, created_at`,
		rtID, in.FromLocationID, in.ToLocationID, canonAmt, transferDate, in.Notes, createdBy,
	).Scan(&tr.ID, &tr.RTID, &tr.FromLocationID, &tr.ToLocationID, &tr.Amount, &tr.TransferDate, &notesStr, &tr.CreatedBy, &tr.ReversesTransferID, &tr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert transfer: %w", err)
	}
	if notesStr.Valid {
		tr.Notes = &notesStr.String
	}

	// Insert LBC entries (out and in)
	negAmt := "-" + canonAmt
	_, err = tx.ExecContext(ctx,
		`INSERT INTO location_balance_changes (rt_id, location_id, transfer_id, change_type, amount_delta, notes, created_by, created_at)
		 VALUES ($1, $2, $3, 'transfer_out', $4, $5, $6, $7)`,
		rtID, in.FromLocationID, tr.ID, negAmt, in.Notes, createdBy, transferDate,
	)
	if err != nil {
		return nil, fmt.Errorf("insert transfer_out lbc: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO location_balance_changes (rt_id, location_id, transfer_id, change_type, amount_delta, notes, created_by, created_at)
		 VALUES ($1, $2, $3, 'transfer_in', $4, $5, $6, $7)`,
		rtID, in.ToLocationID, tr.ID, canonAmt, in.Notes, createdBy, transferDate,
	)
	if err != nil {
		return nil, fmt.Errorf("insert transfer_in lbc: %w", err)
	}

	return &tr, nil
}

func (r *Repository) ListTransfersByRT(ctx context.Context, tx *sql.Tx, rtID string, offset, limit int) ([]*FundTransfer, int, error) {
	var total int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM fund_transfers WHERE rt_id = $1`, rtID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT ft.id, ft.rt_id, ft.from_location_id, ft.to_location_id, ft.amount::text, ft.transfer_date,
		        ft.notes, ft.created_by::text, ft.reverses_transfer_id, ft.created_at,
		        fl_from.name as from_location_name, fl_to.name as to_location_name
		 FROM fund_transfers ft
		 JOIN fund_locations fl_from ON ft.from_location_id = fl_from.id
		 JOIN fund_locations fl_to ON ft.to_location_id = fl_to.id
		 WHERE ft.rt_id = $1
		 ORDER BY ft.transfer_date DESC, ft.created_at DESC
		 LIMIT $2 OFFSET $3`,
		rtID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list transfers: %w", err)
	}
	defer rows.Close()

	var list []*FundTransfer
	for rows.Next() {
		var tr FundTransfer
		var notesStr, fromName, toName sql.NullString
		if err := rows.Scan(&tr.ID, &tr.RTID, &tr.FromLocationID, &tr.ToLocationID, &tr.Amount, &tr.TransferDate,
			&notesStr, &tr.CreatedBy, &tr.ReversesTransferID, &tr.CreatedAt, &fromName, &toName); err != nil {
			return nil, 0, err
		}
		if notesStr.Valid {
			tr.Notes = &notesStr.String
		}
		if fromName.Valid {
			tr.FromLocationName = &fromName.String
		}
		if toName.Valid {
			tr.ToLocationName = &toName.String
		}
		list = append(list, &tr)
	}
	return list, total, rows.Err()
}

func (r *Repository) GetTransferByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*FundTransfer, error) {
	var tr FundTransfer
	var notesStr, fromName, toName sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT ft.id, ft.rt_id, ft.from_location_id, ft.to_location_id, ft.amount::text, ft.transfer_date,
		        ft.notes, ft.created_by::text, ft.reverses_transfer_id, ft.created_at,
		        fl_from.name as from_location_name, fl_to.name as to_location_name
		 FROM fund_transfers ft
		 JOIN fund_locations fl_from ON ft.from_location_id = fl_from.id
		 JOIN fund_locations fl_to ON ft.to_location_id = fl_to.id
		 WHERE ft.id = $1 AND ft.rt_id = $2`,
		id, rtID,
	).Scan(&tr.ID, &tr.RTID, &tr.FromLocationID, &tr.ToLocationID, &tr.Amount, &tr.TransferDate,
		&notesStr, &tr.CreatedBy, &tr.ReversesTransferID, &tr.CreatedAt, &fromName, &toName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get transfer: %w", err)
	}
	if notesStr.Valid {
		tr.Notes = &notesStr.String
	}
	if fromName.Valid {
		tr.FromLocationName = &fromName.String
	}
	if toName.Valid {
		tr.ToLocationName = &toName.String
	}
	return &tr, nil
}

func (r *Repository) ReverseTransfer(ctx context.Context, tx *sql.Tx, rtID, createdBy, transferID string, in ReverseFundTransferInput) (*FundTransfer, error) {
	orig, err := r.GetTransferByID(ctx, tx, transferID, rtID)
	if err != nil {
		return nil, err
	}
	if orig.ReversesTransferID != nil {
		return nil, ErrCannotReverseReversal
	}

	var alreadyCount int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM fund_transfers WHERE reverses_transfer_id = $1 AND rt_id = $2`, orig.ID, rtID).Scan(&alreadyCount)
	if err != nil {
		return nil, err
	}
	if alreadyCount > 0 {
		return nil, ErrAlreadyReversed
	}

	amtCents, canonAmt, err := finance.ParseMoney(orig.Amount)
	if err != nil {
		return nil, err
	}

	// Ordered lock on both locations
	firstID, secondID := orig.FromLocationID, orig.ToLocationID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM fund_locations WHERE id IN ($1, $2) AND rt_id = $3 ORDER BY id FOR UPDATE`,
		firstID, secondID, rtID,
	)
	if err != nil {
		return nil, err
	}
	rows.Close()

	// Recipient of original transfer (orig.ToLocationID) must return the money. Check its balance.
	_, toCents, err := r.GetLocationBalance(ctx, tx, orig.ToLocationID, rtID)
	if err != nil {
		return nil, err
	}
	if toCents < amtCents {
		return nil, ErrInsufficientReversal
	}

	now := time.Now()
	notes := fmt.Sprintf("Reversal of transfer %s: %s", orig.ID, in.Reason)

	var rev FundTransfer
	var notesStr sql.NullString
	err = tx.QueryRowContext(ctx,
		`INSERT INTO fund_transfers (rt_id, from_location_id, to_location_id, amount, transfer_date, notes, created_by, reverses_transfer_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, rt_id, from_location_id, to_location_id, amount::text, transfer_date, notes, created_by::text, reverses_transfer_id, created_at`,
		rtID, orig.ToLocationID, orig.FromLocationID, canonAmt, now, notes, createdBy, orig.ID,
	).Scan(&rev.ID, &rev.RTID, &rev.FromLocationID, &rev.ToLocationID, &rev.Amount, &rev.TransferDate, &notesStr, &rev.CreatedBy, &rev.ReversesTransferID, &rev.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert reversal transfer: %w", err)
	}
	if notesStr.Valid {
		rev.Notes = &notesStr.String
	}

	// LBC entries
	negAmt := "-" + canonAmt
	_, err = tx.ExecContext(ctx,
		`INSERT INTO location_balance_changes (rt_id, location_id, transfer_id, change_type, amount_delta, notes, created_by, created_at)
		 VALUES ($1, $2, $3, 'reversal_transfer_in', $4, $5, $6, $7)`,
		rtID, orig.ToLocationID, rev.ID, negAmt, notes, createdBy, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert reversal_transfer_in: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO location_balance_changes (rt_id, location_id, transfer_id, change_type, amount_delta, notes, created_by, created_at)
		 VALUES ($1, $2, $3, 'reversal_transfer_out', $4, $5, $6, $7)`,
		rtID, orig.FromLocationID, rev.ID, canonAmt, notes, createdBy, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert reversal_transfer_out: %w", err)
	}

	return &rev, nil
}

func (r *Repository) SetOpeningBalance(ctx context.Context, tx *sql.Tx, rtID, createdBy, locationID string, in SetOpeningBalanceInput) (*FundLocation, error) {
	amtCents, canonAmt, err := finance.ParseMoney(in.Amount)
	if err != nil || amtCents <= 0 {
		return nil, ErrInvalidAmount
	}

	// Lock location
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT true FROM fund_locations WHERE id = $1 AND rt_id = $2 FOR UPDATE`, locationID, rtID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Calculate unallocated cash: U = K - L
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	now := time.Now().In(loc)
	currentPeriod := now.Format("2006-01")
	tPeriod, _ := time.ParseInLocation("2006-01", currentPeriod, loc)
	startOfMonth := time.Date(tPeriod.Year(), tPeriod.Month(), 1, 0, 0, 0, 0, loc)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	var maxTxTime sql.NullTime
	_ = tx.QueryRowContext(ctx, `SELECT MAX(occurred_at) FROM transactions WHERE rt_id = $1`, rtID).Scan(&maxTxTime)
	endTime := startOfNextMonth
	if maxTxTime.Valid && maxTxTime.Time.After(endTime) {
		endTime = maxTxTime.Time.AddDate(0, 1, 0)
	}

	pb, err := finance.CalculatePeriodBalance(ctx, tx, rtID, currentPeriod, startOfMonth, endTime)
	if err != nil {
		return nil, err
	}
	kCents, _, err := finance.ParseMoney(pb.ClosingBalance)
	if err != nil {
		return nil, err
	}

	var lbcStr string
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_delta), 0)::text
		 FROM location_balance_changes
		 WHERE rt_id = $1`,
		rtID,
	).Scan(&lbcStr)
	if err != nil {
		return nil, err
	}
	lCents, _, _ := finance.ParseMoney(lbcStr)

	availUCents := kCents - lCents
	if amtCents > availUCents {
		return nil, ErrInsufficientUnallocated
	}

	notes := "Opening balance allocation"
	if in.Notes != nil && strings.TrimSpace(*in.Notes) != "" {
		notes = strings.TrimSpace(*in.Notes)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO location_balance_changes (rt_id, location_id, change_type, amount_delta, notes, created_by)
		 VALUES ($1, $2, 'opening_balance', $3, $4, $5)`,
		rtID, locationID, canonAmt, notes, createdBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert opening balance: %w", err)
	}

	return r.GetLocationByID(ctx, tx, locationID, rtID)
}

func (r *Repository) GetReconciliationReport(ctx context.Context, tx *sql.Tx, rtID string) (*ReconciliationReport, error) {
	// 1. Kas Balance K
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	now := time.Now().In(loc)
	currentPeriod := now.Format("2006-01")
	tPeriod, _ := time.ParseInLocation("2006-01", currentPeriod, loc)
	startOfMonth := time.Date(tPeriod.Year(), tPeriod.Month(), 1, 0, 0, 0, 0, loc)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	var maxTxTime sql.NullTime
	_ = tx.QueryRowContext(ctx, `SELECT MAX(occurred_at) FROM transactions WHERE rt_id = $1`, rtID).Scan(&maxTxTime)
	endTime := startOfNextMonth
	if maxTxTime.Valid && maxTxTime.Time.After(endTime) {
		endTime = maxTxTime.Time.AddDate(0, 1, 0)
	}

	pb, err := finance.CalculatePeriodBalance(ctx, tx, rtID, currentPeriod, startOfMonth, endTime)
	if err != nil {
		return nil, err
	}
	kCents, _, err := finance.ParseMoney(pb.ClosingBalance)
	if err != nil {
		return nil, err
	}
	kStr := pb.ClosingBalance

	// 2. Total Allocated Balance L (sum of all LBC)
	var lbcStr string
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_delta), 0)::text
		 FROM location_balance_changes
		 WHERE rt_id = $1`,
		rtID,
	).Scan(&lbcStr)
	if err != nil {
		return nil, err
	}
	lCents, _, _ := finance.ParseMoney(lbcStr)
	lStr := finance.FormatMoney(lCents)

	// 3. Unallocated balance U = K - L
	uCents := kCents - lCents
	if uCents < 0 {
		uCents = 0
	}
	uStr := finance.FormatMoney(uCents)

	// 4. Discrepancy D = K - (L + U)
	dCents := kCents - (lCents + uCents)
	dStr := finance.FormatMoney(dCents)

	// 5. Category breakdown
	rows, err := tx.QueryContext(ctx,
		`SELECT fl.category, COALESCE(SUM(lbc.amount_delta), 0)::text, COUNT(DISTINCT fl.id)
		 FROM fund_locations fl
		 LEFT JOIN location_balance_changes lbc ON fl.id = lbc.location_id
		 WHERE fl.rt_id = $1
		 GROUP BY fl.category`,
		rtID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var catSummaries []CategorySummary
	for rows.Next() {
		var cs CategorySummary
		var rawBal string
		if err := rows.Scan(&cs.Category, &rawBal, &cs.LocationsCount); err != nil {
			return nil, err
		}
		cents, canon, _ := finance.ParseMoney(rawBal)
		_ = cents
		cs.TotalBalance = canon
		catSummaries = append(catSummaries, cs)
	}

	// 6. Anomalies check query
	anomalyRows, err := tx.QueryContext(ctx,
		`WITH
		orphan_transactions AS (
		    SELECT t.id::text as id, 'TRANSACTION_WITHOUT_LBC' AS issue
		    FROM transactions t
		    LEFT JOIN location_balance_changes lbc ON t.id = lbc.transaction_id
		    WHERE t.rt_id = $1 AND t.status = 'posted' AND t.location_id IS NOT NULL AND lbc.id IS NULL
		),
		mismatched_amounts AS (
		    SELECT lbc.id::text as id, 'AMOUNT_MISMATCH_WITH_TRANSACTION' AS issue
		    FROM location_balance_changes lbc
		    JOIN transactions t ON lbc.transaction_id = t.id
		    WHERE lbc.rt_id = $1 AND ABS(lbc.amount_delta) != t.amount
		),
		unbalanced_transfers AS (
		    SELECT ft.id::text as id, 'UNBALANCED_TRANSFER' AS issue
		    FROM fund_transfers ft
		    LEFT JOIN location_balance_changes lbc_out ON ft.id = lbc_out.transfer_id AND lbc_out.change_type IN ('transfer_out', 'reversal_transfer_out')
		    LEFT JOIN location_balance_changes lbc_in ON ft.id = lbc_in.transfer_id AND lbc_in.change_type IN ('transfer_in', 'reversal_transfer_in')
		    WHERE ft.rt_id = $1 AND (lbc_out.id IS NULL OR lbc_in.id IS NULL OR (ABS(lbc_out.amount_delta) != ABS(lbc_in.amount_delta)))
		),
		cross_tenant_lbc AS (
		    SELECT lbc.id::text as id, 'CROSS_TENANT_LEAK' AS issue
		    FROM location_balance_changes lbc
		    JOIN fund_locations fl ON lbc.location_id = fl.id
		    WHERE lbc.rt_id = $1 AND lbc.rt_id != fl.rt_id
		)
		SELECT issue || ': ' || id FROM orphan_transactions
		UNION ALL
		SELECT issue || ': ' || id FROM mismatched_amounts
		UNION ALL
		SELECT issue || ': ' || id FROM unbalanced_transfers
		UNION ALL
		SELECT issue || ': ' || id FROM cross_tenant_lbc`,
		rtID,
	)
	var anomalies []string
	if err == nil {
		defer anomalyRows.Close()
		for anomalyRows.Next() {
			var issue string
			if err := anomalyRows.Scan(&issue); err == nil {
				anomalies = append(anomalies, issue)
			}
		}
	}

	status := "MATCH"
	label := "Sempurna"
	if dCents != 0 || len(anomalies) > 0 {
		status = "DISCREPANCY_ANOMALY"
		label = "Selisih / Anomali"
	} else if uCents > 0 {
		status = "MATCH_WITH_NOTES"
		label = "Cocok dengan Catatan"
	}

	return &ReconciliationReport{
		KasBalance:            kStr,
		TotalAllocatedBalance: lStr,
		UnallocatedBalance:    uStr,
		Discrepancy:           dStr,
		Status:                status,
		StatusLabel:           label,
		LocationsSummary:      catSummaries,
		AnomaliesDetected:     anomalies,
	}, nil
}
