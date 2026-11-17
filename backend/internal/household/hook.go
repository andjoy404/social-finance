package household

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"social-finance/internal/finance"
)

// SyncOccupancyBillsFn is invoked when a household occupancy is created or updated
// to synchronize active dues/bills for the occupancy.
// Defaults to finance.SyncHouseholdOccupancyBills so all callers (HTTP handlers,
// XLSX/CSV imports, and integration tests) share the same single source of truth.
var SyncOccupancyBillsFn = func(ctx context.Context, tx *sql.Tx, rtID, occupancyID string) error {
	_, err := finance.SyncHouseholdOccupancyBills(ctx, tx, rtID, occupancyID)
	return err
}

func triggerSyncBills(ctx context.Context, tx *sql.Tx, rtID, occupancyID string) {
	_ = syncOccupancyBills(ctx, tx, rtID, occupancyID)
}

func syncOccupancyBills(ctx context.Context, tx *sql.Tx, rtID, occupancyID string) error {
	if SyncOccupancyBillsFn != nil && rtID != "" && occupancyID != "" {
		return SyncOccupancyBillsFn(ctx, tx, rtID, occupancyID)
	}
	return nil
}

// findHeadResidencyPeriod locates the primary (head) residency period for an occupancy.
// It prefers an active residency period whose resident's full_name matches the household's
// head_name, falling back to the earliest created active residency period.
func findHeadResidencyPeriod(ctx context.Context, tx *sql.Tx, occupancyID string) (rpID string, residentID string, startDate sql.NullString) {
	_ = tx.QueryRowContext(ctx, `
		SELECT rp.id, rp.resident_id, to_char(rp.start_date, 'YYYY-MM-DD')
		FROM residency_periods rp
		JOIN residents r ON r.id = rp.resident_id
		JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
		JOIN households h ON h.id = ho.household_id
		WHERE rp.household_occupancy_id = $1
		  AND r.is_active = true
		ORDER BY (rp.end_date IS NULL) DESC,
		         (lower(trim(r.full_name)) = lower(trim(h.head_name))) DESC,
		         rp.created_at ASC
		LIMIT 1
	`, occupancyID).Scan(&rpID, &residentID, &startDate)
	return rpID, residentID, startDate
}

// SyncOccupancyDatesAndBills updates household_occupancies start_date/end_date
// (and optionally the head resident's residency_period when called from HouseholdUpdate)
// and reconciles monthly bill obligations via SyncOccupancyBillsFn inside the same transaction.
//
// Historical residency periods (end_date IS NOT NULL) and non-head household members'
// residency periods are never overwritten.
func SyncOccupancyDatesAndBills(
	ctx context.Context,
	tx *sql.Tx,
	rtID, occupancyID string,
	startDate, endDate *string,
	syncHeadResidencyPeriod bool,
) error {
	if occupancyID == "" {
		return nil
	}

	headRPID, _, headStart := findHeadResidencyPeriod(ctx, tx, occupancyID)
	dateUpdated := false

	// Determine effective occupancy start_date:
	// - When called from HouseholdUpdate (syncHeadResidencyPeriod == true), use the explicit startDate.
	// - When called from ResidentUpdate / XLSX import (syncHeadResidencyPeriod == false),
	//   derive the occupancy start_date from the head resident's active residency period
	//   so editing a non-head family member never overwrites the household's start_date.
	effectiveStart := ""
	if startDate != nil && strings.TrimSpace(*startDate) != "" {
		if syncHeadResidencyPeriod {
			effectiveStart = strings.TrimSpace(*startDate)
		} else if headStart.Valid && headStart.String != "" {
			effectiveStart = headStart.String
		} else {
			effectiveStart = strings.TrimSpace(*startDate)
		}
	}

	if effectiveStart != "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE household_occupancies
			 SET start_date = $1::date, updated_at = now()
			 WHERE id = $2 AND start_date IS DISTINCT FROM $1::date`,
			effectiveStart, occupancyID,
		); err != nil {
			return fmt.Errorf("update occupancy start_date: %w", err)
		}
		if syncHeadResidencyPeriod && headRPID != "" {
			if _, err := tx.ExecContext(ctx,
				`UPDATE residency_periods
				 SET start_date = $1::date, updated_at = now()
				 WHERE id = $2 AND start_date IS DISTINCT FROM $1::date`,
				effectiveStart, headRPID,
			); err != nil {
				return fmt.Errorf("update head residency_period start_date: %w", err)
			}
		}
		dateUpdated = true
	}

	if endDate != nil && syncHeadResidencyPeriod {
		trimmedEnd := strings.TrimSpace(*endDate)
		if trimmedEnd == "" {
			if _, err := tx.ExecContext(ctx,
				`UPDATE household_occupancies
				 SET end_date = NULL, updated_at = now()
				 WHERE id = $1 AND end_date IS NOT NULL`,
				occupancyID,
			); err != nil {
				return fmt.Errorf("clear occupancy end_date: %w", err)
			}
			if headRPID != "" {
				if _, err := tx.ExecContext(ctx,
					`UPDATE residency_periods
					 SET end_date = NULL, updated_at = now()
					 WHERE id = $1 AND end_date IS NOT NULL`,
					headRPID,
				); err != nil {
					return fmt.Errorf("clear head residency_period end_date: %w", err)
				}
			}
		} else {
			if _, err := tx.ExecContext(ctx,
				`UPDATE household_occupancies
				 SET end_date = $1::date, updated_at = now()
				 WHERE id = $2 AND end_date IS DISTINCT FROM $1::date`,
				trimmedEnd, occupancyID,
			); err != nil {
				return fmt.Errorf("update occupancy end_date: %w", err)
			}
			if headRPID != "" {
				if _, err := tx.ExecContext(ctx,
					`UPDATE residency_periods
					 SET end_date = $1::date, updated_at = now()
					 WHERE id = $2 AND end_date IS DISTINCT FROM $1::date`,
					trimmedEnd, headRPID,
				); err != nil {
					return fmt.Errorf("update head residency_period end_date: %w", err)
				}
			}
		}
		dateUpdated = true
	} else if endDate != nil && !syncHeadResidencyPeriod {
		dateUpdated = true
	}

	if dateUpdated {
		if err := syncOccupancyBills(ctx, tx, rtID, occupancyID); err != nil {
			return fmt.Errorf("sync occupancy bills: %w", err)
		}
	}
	return nil
}
