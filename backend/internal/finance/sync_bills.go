package finance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SyncHouseholdOccupancyBills ensures that an active household occupancy in rtID
// has all required bills for all currently active dues in the RT:
//   - Multi-Month dues: generates all 12 calendar-year periods (Jan-Dec) for the due's year.
//     Eligible if occupancy overlaps with the calendar year (i.e. start_date <= yearEnd and (end_date is null or end_date >= yearStart)).
//   - Regular dues: generates bills for all periods already issued for this regular due in the RT.
//   - Voluntary dues: skipped (voluntary dues are never auto-generated).
//
// Idempotency: Uses ON CONFLICT (household_occupancy_id, due_id, period) DO NOTHING, ensuring:
//  1. Existing bills and their payment statuses ('paid', 'unpaid', 'cancelled') are NEVER overwritten.
//  2. Existing bill amounts are preserved.
//  3. Duplicate bills are never created.
//  4. Concurrent calls safely produce no duplicates.
func SyncHouseholdOccupancyBills(ctx context.Context, tx *sql.Tx, rtID, occupancyID string) (int, error) {
	if rtID == "" || occupancyID == "" {
		return 0, nil
	}

	// 1. Fetch occupancy details
	var occID string
	var occStartDate, occEndDate sql.NullTime
	err := tx.QueryRowContext(ctx, `
		SELECT ho.id, ho.start_date, ho.end_date
		FROM household_occupancies ho
		JOIN households h ON ho.household_id = h.id
		WHERE ho.id = $1 AND h.rt_id = $2 AND h.is_active = true
		LIMIT 1
	`, occupancyID, rtID).Scan(&occID, &occStartDate, &occEndDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil // Not an active occupancy in this RT
		}
		return 0, fmt.Errorf("lookup occupancy for sync: %w", err)
	}

	var moveInMonth, moveOutMonth sql.NullString
	if occStartDate.Valid {
		moveInMonth = sql.NullString{String: occStartDate.Time.Format("2006-01"), Valid: true}
	}
	if occEndDate.Valid {
		moveOutMonth = sql.NullString{String: occEndDate.Time.Format("2006-01"), Valid: true}
	}

	// 2. Fetch all active dues in this RT
	rows, err := tx.QueryContext(ctx, `
		SELECT id, due_type, year, amount::text, is_voluntary, payment_rule
		FROM dues
		WHERE rt_id = $1 AND is_active = true
	`, rtID)
	if err != nil {
		return 0, fmt.Errorf("lookup active dues for sync: %w", err)
	}
	defer rows.Close()

	type dueInfo struct {
		id          string
		dueType     string
		year        sql.NullInt32
		amount      string
		isVoluntary bool
		paymentRule string
	}
	var dues []dueInfo
	for rows.Next() {
		var d dueInfo
		if err := rows.Scan(&d.id, &d.dueType, &d.year, &d.amount, &d.isVoluntary, &d.paymentRule); err != nil {
			return 0, fmt.Errorf("scan active due for sync: %w", err)
		}
		dues = append(dues, d)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate active dues for sync: %w", err)
	}

	createdCount := 0

	for _, d := range dues {
		// Rule C: Voluntary dues are NEVER auto-generated
		if d.isVoluntary || d.dueType == DueTypeVoluntary {
			continue
		}

		if d.dueType == DueTypeMultiMonth {
			if !d.year.Valid || d.year.Int32 <= 0 {
				continue
			}
			year := int(d.year.Int32)
			yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			yearEnd := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)

			// Check overlap:
			// If occupancy strictly started AFTER this calendar year, skip.
			if occStartDate.Valid && occStartDate.Time.After(yearEnd) {
				continue
			}
			// If occupancy strictly ended BEFORE this calendar year, skip.
			if occEndDate.Valid && occEndDate.Time.Before(yearStart) {
				continue
			}

			// Generate all 12 calendar-year periods (Jan - Dec)
			targetDay := 10
			for m := 1; m <= 12; m++ {
				period := fmt.Sprintf("%04d-%02d", year, m)
				lastDay := time.Date(year, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Day()
				day := targetDay
				if day > lastDay {
					day = lastDay
				}
				monthDueDate := fmt.Sprintf("%04d-%02d-%02d", year, m, day)

				isObligatory := true
				if occStartDate.Valid {
					moveInMonth := occStartDate.Time.Format("2006-01")
					if period < moveInMonth {
						isObligatory = false
					}
				}
				if occEndDate.Valid {
					moveOutMonth := occEndDate.Time.Format("2006-01")
					if period > moveOutMonth {
						isObligatory = false
					}
				}

				var isInserted bool
				err := tx.QueryRowContext(ctx, `
					INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status, is_active, is_obligatory)
					VALUES ($1, $2, $3, $4, $5, $6, 'unpaid', true, $7)
					ON CONFLICT (household_occupancy_id, due_id, period) DO UPDATE
					SET is_obligatory = EXCLUDED.is_obligatory,
					    updated_at = CURRENT_TIMESTAMP
					WHERE bills.is_obligatory IS DISTINCT FROM EXCLUDED.is_obligatory
					  AND (
					    (EXCLUDED.is_obligatory = true AND bills.status IN ('unpaid', 'partial', 'paid'))
					    OR
					    (EXCLUDED.is_obligatory = false AND bills.status = 'unpaid' AND NOT EXISTS (
					      SELECT 1 FROM payments p
					      WHERE p.status IN ('APPROVED', 'PENDING')
					        AND (
					          p.bill_id = bills.id
					          OR EXISTS (
					            SELECT 1 FROM payment_allocations pa
					            WHERE pa.payment_id = p.id AND pa.bill_id = bills.id
					          )
					        )
					    ))
					  )
					RETURNING (xmax = 0)
				`, rtID, occID, d.id, d.amount, period, monthDueDate, isObligatory).Scan(&isInserted)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return createdCount, fmt.Errorf("sync multi-month bill (%s): %w", period, err)
				}
				if isInserted {
					createdCount++
				}
			}
		} else if d.dueType == DueTypeRegular {
			// Query periods already issued in the RT for this regular due
			pRows, err := tx.QueryContext(ctx, `
				SELECT DISTINCT period, to_char(due_date, 'YYYY-MM-DD')
				FROM bills
				WHERE rt_id = $1 AND due_id = $2
				ORDER BY period ASC
			`, rtID, d.id)
			if err != nil {
				return createdCount, fmt.Errorf("query issued regular periods: %w", err)
			}

			type periodInfo struct {
				period  string
				dueDate string
			}
			var issuedPeriods []periodInfo
			for pRows.Next() {
				var pi periodInfo
				if err := pRows.Scan(&pi.period, &pi.dueDate); err == nil {
					issuedPeriods = append(issuedPeriods, pi)
				}
			}
			pRows.Close()

			for _, pi := range issuedPeriods {
				isObligatory := true
				if occStartDate.Valid {
					moveInMonth := occStartDate.Time.Format("2006-01")
					if pi.period < moveInMonth {
						isObligatory = false
					}
				}
				if occEndDate.Valid {
					moveOutMonth := occEndDate.Time.Format("2006-01")
					if pi.period > moveOutMonth {
						isObligatory = false
					}
				}

				var isInserted bool
				err := tx.QueryRowContext(ctx, `
					INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status, is_active, is_obligatory)
					VALUES ($1, $2, $3, $4, $5, $6, 'unpaid', true, $7)
					ON CONFLICT (household_occupancy_id, due_id, period) DO UPDATE
					SET is_obligatory = EXCLUDED.is_obligatory,
					    updated_at = CURRENT_TIMESTAMP
					WHERE bills.is_obligatory IS DISTINCT FROM EXCLUDED.is_obligatory
					  AND (
					    (EXCLUDED.is_obligatory = true AND bills.status IN ('unpaid', 'partial', 'paid'))
					    OR
					    (EXCLUDED.is_obligatory = false AND bills.status = 'unpaid' AND NOT EXISTS (
					      SELECT 1 FROM payments p
					      WHERE p.status IN ('APPROVED', 'PENDING')
					        AND (
					          p.bill_id = bills.id
					          OR EXISTS (
					            SELECT 1 FROM payment_allocations pa
					            WHERE pa.payment_id = p.id AND pa.bill_id = bills.id
					          )
					        )
					    ))
					  )
					RETURNING (xmax = 0)
				`, rtID, occID, d.id, d.amount, pi.period, pi.dueDate, isObligatory).Scan(&isInserted)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return createdCount, fmt.Errorf("sync regular bill (%s): %w", pi.period, err)
				}
				if isInserted {
					createdCount++
				}
			}
		}
	}

	// Reconcile any remaining existing bills for this occupancy (e.g. bills in earlier/later
	// calendar years when start_date or end_date moves across years).
	_, err = tx.ExecContext(ctx, `
		UPDATE bills b
		SET is_obligatory = CASE
			WHEN $3::text IS NOT NULL AND b.period < $3 THEN false
			WHEN $4::text IS NOT NULL AND b.period > $4 THEN false
			ELSE true
		END,
		updated_at = CURRENT_TIMESTAMP
		WHERE b.household_occupancy_id = $1
		  AND b.rt_id = $2
		  AND b.is_obligatory IS DISTINCT FROM (
			CASE
				WHEN $3::text IS NOT NULL AND b.period < $3 THEN false
				WHEN $4::text IS NOT NULL AND b.period > $4 THEN false
				ELSE true
			END
		  )
		  AND (
			(
				NOT ($3::text IS NOT NULL AND b.period < $3)
				AND NOT ($4::text IS NOT NULL AND b.period > $4)
				AND b.status IN ('unpaid', 'partial', 'paid')
			)
			OR
			(
				(($3::text IS NOT NULL AND b.period < $3) OR ($4::text IS NOT NULL AND b.period > $4))
				AND b.status = 'unpaid'
				AND NOT EXISTS (
					SELECT 1 FROM payments p
					WHERE p.status IN ('APPROVED', 'PENDING')
					  AND (
					    p.bill_id = b.id
					    OR EXISTS (
					      SELECT 1 FROM payment_allocations pa
					      WHERE pa.payment_id = p.id AND pa.bill_id = b.id
					    )
					  )
				)
			)
		  )
	`, occID, rtID, moveInMonth, moveOutMonth)
	if err != nil {
		return createdCount, fmt.Errorf("reconcile existing bills obligation: %w", err)
	}

	return createdCount, nil
}

// SyncBillsForRT synchronizes all active household occupancies in an RT
// against all active dues.
func SyncBillsForRT(ctx context.Context, tx *sql.Tx, rtID string) (int, error) {
	if rtID == "" {
		return 0, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT ho.id
		FROM household_occupancies ho
		JOIN households h ON ho.household_id = h.id
		WHERE h.rt_id = $1 AND h.is_active = true AND ho.end_date IS NULL
	`, rtID)
	if err != nil {
		return 0, fmt.Errorf("lookup active occupancies for RT sync: %w", err)
	}
	defer rows.Close()

	var occIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			occIDs = append(occIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate active occupancies for RT sync: %w", err)
	}

	totalCreated := 0
	for _, occID := range occIDs {
		n, err := SyncHouseholdOccupancyBills(ctx, tx, rtID, occID)
		if err != nil {
			return totalCreated, err
		}
		totalCreated += n
	}
	return totalCreated, nil
}
