package finance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// OverdueWarningSummary represents the global warning summary for a logged-in resident (warga).
// Monetary values use int64 Rupiah (derived from fixed-point cents / 100, never floating point).
type OverdueWarningSummary struct {
	HasOverdue3Months bool  `json:"has_overdue_3_months"`
	TotalOutstanding  int64 `json:"total_outstanding"`
	OverdueBillCount  int   `json:"overdue_bill_count"`
}

// BillEvaluationInput holds all fields required to evaluate whether a bill qualifies
// for upcoming due (3 months before due_date) or overdue (3+ months after due_date).
// Used as the single source of truth by both the Notification Scheduler and Warning Summary API.
type BillEvaluationInput struct {
	BillID               string
	RTID                 string
	UserID               string
	UserFullName         string
	HouseholdOccupancyID string
	DueID                string
	DueName              string
	Period               string
	AmountCents          int64
	ApprovedPaidCents    int64
	PendingPaidCents     int64
	RejectedPaidCents    int64
	Status               string
	IsActive             bool
	IsObligatory         bool
	IsVoluntary          bool
	DueIsActive          bool
	DueDate              *time.Time
	MoveInDate           *time.Time
	MoveOutDate          *time.Time
}

// EligibleBillResult represents a bill that passed all obligation and threshold checks,
// together with its calculated outstanding balance in cents and Rupiah.
type EligibleBillResult struct {
	Bill                 BillEvaluationInput
	OutstandingCents     int64
	OutstandingRupiah    int64
	FormattedOutstanding string
	DueDateFormatted     string
}

var phonePrefixRegex = regexp.MustCompile(`^(\+62|62|0)`)

// JakartaLocation returns the Asia/Jakarta timezone (UTC+7).
func JakartaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

// NormalizeDateJakarta converts a timestamp to Asia/Jakarta and truncates to midnight (YYYY-MM-DD 00:00:00 +07:00).
func NormalizeDateJakarta(t time.Time) time.Time {
	loc := JakartaLocation()
	inLoc := t.In(loc)
	return time.Date(inLoc.Year(), inLoc.Month(), inLoc.Day(), 0, 0, 0, 0, loc)
}

// ParseDateJakarta parses a YYYY-MM-DD or RFC3339 date string into midnight Asia/Jakarta.
func ParseDateJakarta(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	loc := JakartaLocation()
	if len(trimmed) >= 10 {
		if d, err := time.ParseInLocation("2006-01-02", trimmed[:10], loc); err == nil {
			return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc), nil
		}
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return NormalizeDateJakarta(t), nil
	}
	return time.Time{}, fmt.Errorf("invalid date format: %s", raw)
}

// AddCalendarMonths adds (or subtracts) calendar months to a date in Asia/Jakarta.
// If the original day-of-month exceeds the number of days in the target month,
// it clamps to the last valid calendar day of the target month (matching PostgreSQL's
// date + INTERVAL 'N months' behavior, e.g. May 31 - 3 months = Feb 28/29,
// Nov 30 + 3 months = Feb 28/29, Jul 15 + 3 months = Oct 15).
func AddCalendarMonths(date time.Time, months int) time.Time {
	norm := NormalizeDateJakarta(date)
	loc := norm.Location()

	// Step 1: Find the 1st of the target month (Go normalizes year/month overflow/underflow).
	firstOfTarget := time.Date(norm.Year(), norm.Month()+time.Month(months), 1, 0, 0, 0, 0, loc)

	// Step 2: Find the last day of the target month by taking day 0 of (targetMonth + 1).
	lastDayOfTarget := time.Date(firstOfTarget.Year(), firstOfTarget.Month()+1, 0, 0, 0, 0, 0, loc).Day()

	day := norm.Day()
	if day > lastDayOfTarget {
		day = lastDayOfTarget
	}

	return time.Date(firstOfTarget.Year(), firstOfTarget.Month(), day, 0, 0, 0, 0, loc)
}

// UpcomingDue3MonthsTriggerDate returns the calendar date 3 months before dueDate in Asia/Jakarta.
func UpcomingDue3MonthsTriggerDate(dueDate time.Time) time.Time {
	return AddCalendarMonths(dueDate, -3)
}

// Overdue3MonthsThresholdDate returns the calendar date 3 months after dueDate in Asia/Jakarta.
func Overdue3MonthsThresholdDate(dueDate time.Time) time.Time {
	return AddCalendarMonths(dueDate, 3)
}

// IsUpcomingDue3Months returns true when evalDate has reached the 3-calendar-month window
// before dueDate (evalDate >= dueDate - 3 calendar months) and is still before dueDate.
func IsUpcomingDue3Months(dueDate, evalDate time.Time) bool {
	dueNorm := NormalizeDateJakarta(dueDate)
	evalNorm := NormalizeDateJakarta(evalDate)
	triggerDate := UpcomingDue3MonthsTriggerDate(dueNorm)
	return !evalNorm.Before(triggerDate) && evalNorm.Before(dueNorm)
}

// IsOverdue3Months returns true when evalDate is strictly more than 3 calendar months
// after dueDate (evalDate > dueDate + 3 calendar months).
// Exact 3 calendar months is NOT eligible.
func IsOverdue3Months(dueDate, evalDate time.Time) bool {
	dueNorm := NormalizeDateJakarta(dueDate)
	evalNorm := NormalizeDateJakarta(evalDate)
	thresholdDate := Overdue3MonthsThresholdDate(dueNorm)
	return evalNorm.After(thresholdDate)
}

// CalculateOutstandingCents returns the remaining unpaid obligation in cents:
//
//	Outstanding = Bill amount - Approved payment allocations
//
// Pending and Rejected payments do not reduce the outstanding balance.
func CalculateOutstandingCents(bill BillEvaluationInput) int64 {
	if bill.AmountCents <= 0 {
		return 0
	}
	approved := bill.ApprovedPaidCents
	if approved < 0 {
		approved = 0
	}
	rem := bill.AmountCents - approved
	if rem < 0 {
		return 0
	}
	return rem
}

// FormatRupiahDisplay formats cents (int64) into Indonesian Rupiah display string (e.g. "Rp 450.000").
func FormatRupiahDisplay(cents int64) string {
	rupiah := cents / 100
	if rupiah <= 0 {
		return "Rp 0"
	}
	s := fmt.Sprintf("%d", rupiah)
	n := len(s)
	if n <= 3 {
		return "Rp " + s
	}
	var b strings.Builder
	b.WriteString("Rp ")
	rem := n % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		if n > rem {
			b.WriteByte('.')
		}
	}
	for i := rem; i < n; i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < n {
			b.WriteByte('.')
		}
	}
	return b.String()
}

// IsBillEligibleForObligation checks all non-date-threshold business rules for a bill:
//   - Active bill (is_active = true)
//   - Obligatory bill (is_obligatory = true) and non-voluntary active due
//   - Unpaid or partial status (paid and cancelled bills are excluded)
//   - Valid due_date (not nil/zero)
//   - Active occupancy (MoveOutDate == nil or after evalDate)
//   - Within resident's obligation period (move_in_date / start_date rules)
//   - Positive outstanding balance after subtracting APPROVED payment allocations
func IsBillEligibleForObligation(bill BillEvaluationInput, evalDate time.Time) bool {
	if !bill.IsActive {
		return false
	}
	if !bill.IsObligatory {
		return false
	}
	if bill.IsVoluntary || !bill.DueIsActive {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(bill.Status))
	if status != string(BillStatusUnpaid) && status != string(BillStatusPartial) {
		return false
	}
	if bill.DueDate == nil || bill.DueDate.IsZero() {
		return false
	}
	evalNorm := NormalizeDateJakarta(evalDate)
	dueNorm := NormalizeDateJakarta(*bill.DueDate)

	if bill.MoveOutDate != nil && !bill.MoveOutDate.IsZero() {
		moveOutNorm := NormalizeDateJakarta(*bill.MoveOutDate)
		if evalNorm.After(moveOutNorm) {
			return false
		}
	}

	if bill.MoveInDate != nil && !bill.MoveInDate.IsZero() {
		moveInNorm := NormalizeDateJakarta(*bill.MoveInDate)
		// Resident has not moved in yet on evalDate.
		if evalNorm.Before(moveInNorm) {
			return false
		}
		// Check billing period against move-in period.
		period := strings.TrimSpace(bill.Period)
		if len(period) == 7 {
			if period < moveInNorm.Format("2006-01") {
				return false
			}
		} else if len(period) == 4 {
			if period < moveInNorm.Format("2006") {
				return false
			}
		} else if dueNorm.Before(moveInNorm) {
			// Fallback only when period is absent
			return false
		}
	}

	return CalculateOutstandingCents(bill) > 0
}

// EvaluateBillForUpcomingDue3Months evaluates if a bill qualifies for the 3-month upcoming due reminder.
func EvaluateBillForUpcomingDue3Months(bill BillEvaluationInput, evalDate time.Time) (EligibleBillResult, bool) {
	if !IsBillEligibleForObligation(bill, evalDate) {
		return EligibleBillResult{}, false
	}
	if !IsUpcomingDue3Months(*bill.DueDate, evalDate) {
		return EligibleBillResult{}, false
	}
	outCents := CalculateOutstandingCents(bill)
	return EligibleBillResult{
		Bill:                 bill,
		OutstandingCents:     outCents,
		OutstandingRupiah:    outCents / 100,
		FormattedOutstanding: FormatRupiahDisplay(outCents),
		DueDateFormatted:     NormalizeDateJakarta(*bill.DueDate).Format("2006-01-02"),
	}, true
}

// EvaluateBillForOverdue3Months evaluates if a bill qualifies for the 3+ month overdue reminder and warning card.
func EvaluateBillForOverdue3Months(bill BillEvaluationInput, evalDate time.Time) (EligibleBillResult, bool) {
	if !IsBillEligibleForObligation(bill, evalDate) {
		return EligibleBillResult{}, false
	}
	if !IsOverdue3Months(*bill.DueDate, evalDate) {
		return EligibleBillResult{}, false
	}
	outCents := CalculateOutstandingCents(bill)
	return EligibleBillResult{
		Bill:                 bill,
		OutstandingCents:     outCents,
		OutstandingRupiah:    outCents / 100,
		FormattedOutstanding: FormatRupiahDisplay(outCents),
		DueDateFormatted:     NormalizeDateJakarta(*bill.DueDate).Format("2006-01-02"),
	}, true
}

// SummarizeOverdue3Months aggregates all bills that meet the 3+ calendar month overdue criteria
// into the OverdueWarningSummary response model.
func SummarizeOverdue3Months(bills []BillEvaluationInput, evalDate time.Time) OverdueWarningSummary {
	var totalCents int64
	count := 0
	for _, b := range bills {
		if res, ok := EvaluateBillForOverdue3Months(b, evalDate); ok {
			totalCents += res.OutstandingCents
			count++
		}
	}
	totalRupiah := totalCents / 100
	if count == 0 || totalRupiah <= 0 {
		return OverdueWarningSummary{
			HasOverdue3Months: false,
			TotalOutstanding:  0,
			OverdueBillCount:  0,
		}
	}
	return OverdueWarningSummary{
		HasOverdue3Months: true,
		TotalOutstanding:  totalRupiah,
		OverdueBillCount:  count,
	}
}

// ResolveUserOccupancyID resolves the active household_occupancy_id for a user in an RT
// without requiring an open transaction.
func ResolveUserOccupancyID(ctx context.Context, db *sql.DB, rtID, userID string) (string, error) {
	if strings.TrimSpace(rtID) == "" || strings.TrimSpace(userID) == "" {
		return "", nil
	}
	var occupancyID string
	err := db.QueryRowContext(ctx,
		`SELECT ho.id
		 FROM users u
		 JOIN user_rt_memberships urm ON urm.user_id = u.id AND urm.rt_id = $1 AND urm.is_active = true
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
		 JOIN households h ON h.id = ho.household_id AND h.rt_id = $1 AND h.is_active = true
		 WHERE u.id = $2 AND u.is_active = true
		 ORDER BY CASE WHEN r.user_id = u.id THEN 0 ELSE 1 END, ho.created_at DESC
		 LIMIT 1`,
		rtID, userID,
	).Scan(&occupancyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var userPhone sql.NullString
			err2 := db.QueryRowContext(ctx,
				`SELECT u.phone
				 FROM users u
				 JOIN user_rt_memberships urm ON urm.user_id = u.id AND urm.rt_id = $1 AND urm.is_active = true
				 WHERE u.id = $2 AND u.is_active = true`,
				rtID, userID,
			).Scan(&userPhone)
			if err2 == nil && userPhone.Valid && strings.TrimSpace(userPhone.String) != "" {
				normalizedPhone := phonePrefixRegex.ReplaceAllString(strings.TrimSpace(userPhone.String), "")
				err3 := db.QueryRowContext(ctx,
					`SELECT ho.id
					 FROM residents res
					 JOIN household_occupancies ho ON ho.household_id = res.household_id AND ho.end_date IS NULL
					 JOIN households h ON h.id = ho.household_id AND h.rt_id = $1 AND h.is_active = true
					 WHERE res.rt_id = $1 AND res.is_active = true AND res.phone IS NOT NULL
					   AND regexp_replace(res.phone, '^(\+62|62|0)', '') = $2
					 ORDER BY ho.created_at DESC
					 LIMIT 1`,
					rtID, normalizedPhone,
				).Scan(&occupancyID)
				if err3 != nil && !errors.Is(err3, sql.ErrNoRows) {
					return "", fmt.Errorf("fallback lookup user occupancy: %w", err3)
				}
			}
			return occupancyID, nil
		}
		return "", fmt.Errorf("resolve user occupancy: %w", err)
	}
	return occupancyID, nil
}

// GetOccupancyBillsForEvaluation loads all active candidate bills for a given RT and occupancy,
// including approved payment allocation totals, ready for shared evaluation.
func GetOccupancyBillsForEvaluation(ctx context.Context, db *sql.DB, rtID, occupancyID, userID string) ([]BillEvaluationInput, error) {
	if strings.TrimSpace(rtID) == "" || strings.TrimSpace(occupancyID) == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT
			b.id,
			b.rt_id,
			b.household_occupancy_id,
			b.due_id,
			COALESCE(d.name, ''),
			b.period,
			b.amount::text,
			COALESCE((
				SELECT SUM(v.amount)
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
			), 0)::text AS approved_paid,
			b.status,
			b.is_active,
			b.is_obligatory,
			COALESCE(d.is_voluntary, false),
			COALESCE(d.is_active, true),
			to_char(b.due_date, 'YYYY-MM-DD'),
			to_char(ho.start_date, 'YYYY-MM-DD'),
			to_char(ho.end_date, 'YYYY-MM-DD')
		 FROM bills b
		 JOIN dues d ON d.id = b.due_id
		 JOIN household_occupancies ho ON ho.id = b.household_occupancy_id
		 JOIN households h ON h.id = ho.household_id
		 WHERE b.rt_id = $1
		   AND b.household_occupancy_id = $2
		   AND b.is_active = true
		   AND b.is_obligatory = true
		   AND b.status IN ('unpaid', 'partial')
		   AND b.due_date IS NOT NULL
		   AND d.is_active = true
		   AND d.is_voluntary = false
		   AND h.is_active = true
		   AND ho.end_date IS NULL
		 ORDER BY b.due_date ASC, b.period ASC, b.id ASC`,
		rtID, occupancyID,
	)
	if err != nil {
		return nil, fmt.Errorf("query occupancy bills for evaluation: %w", err)
	}
	defer rows.Close()

	var result []BillEvaluationInput
	for rows.Next() {
		var (
			item                              BillEvaluationInput
			amountStr, approvedStr            string
			dueDateStr, moveInStr, moveOutStr sql.NullString
		)
		item.UserID = userID
		if err := rows.Scan(
			&item.BillID,
			&item.RTID,
			&item.HouseholdOccupancyID,
			&item.DueID,
			&item.DueName,
			&item.Period,
			&amountStr,
			&approvedStr,
			&item.Status,
			&item.IsActive,
			&item.IsObligatory,
			&item.IsVoluntary,
			&item.DueIsActive,
			&dueDateStr,
			&moveInStr,
			&moveOutStr,
		); err != nil {
			return nil, fmt.Errorf("scan occupancy bill for evaluation: %w", err)
		}
		if cents, _, err := ParseMoney(amountStr); err == nil {
			item.AmountCents = cents
		}
		if cents, _, err := ParseMoney(approvedStr); err == nil {
			item.ApprovedPaidCents = cents
		}
		if dueDateStr.Valid && dueDateStr.String != "" {
			if dt, err := ParseDateJakarta(dueDateStr.String); err == nil {
				item.DueDate = &dt
			}
		}
		if moveInStr.Valid && moveInStr.String != "" {
			if dt, err := ParseDateJakarta(moveInStr.String); err == nil {
				item.MoveInDate = &dt
			}
		}
		if moveOutStr.Valid && moveOutStr.String != "" {
			if dt, err := ParseDateJakarta(moveOutStr.String); err == nil {
				item.MoveOutDate = &dt
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate occupancy bills for evaluation: %w", err)
	}
	return result, nil
}

// GetOverdueWarningSummaryForUser calculates the OverdueWarningSummary for the authenticated user in their RT.
func GetOverdueWarningSummaryForUser(ctx context.Context, db *sql.DB, rtID, userID string, evalDate time.Time) (OverdueWarningSummary, error) {
	if strings.TrimSpace(rtID) == "" || strings.TrimSpace(userID) == "" {
		return OverdueWarningSummary{}, nil
	}
	occID, err := ResolveUserOccupancyID(ctx, db, rtID, userID)
	if err != nil {
		return OverdueWarningSummary{}, err
	}
	if occID == "" {
		return OverdueWarningSummary{}, nil
	}
	bills, err := GetOccupancyBillsForEvaluation(ctx, db, rtID, occID, userID)
	if err != nil {
		return OverdueWarningSummary{}, err
	}
	return SummarizeOverdue3Months(bills, evalDate), nil
}

// GetRTUserBillsForEvaluation loads all active candidate bills mapped to active resident users in an RT
// for scheduled notification evaluation.
func GetRTUserBillsForEvaluation(ctx context.Context, db *sql.DB, rtID string) ([]BillEvaluationInput, error) {
	if strings.TrimSpace(rtID) == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT ON (u.id, b.id)
			b.id,
			b.rt_id,
			u.id,
			COALESCE(u.full_name, ''),
			b.household_occupancy_id,
			b.due_id,
			COALESCE(d.name, ''),
			b.period,
			b.amount::text,
			COALESCE((
				SELECT SUM(v.amount)
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
			), 0)::text AS approved_paid,
			b.status,
			b.is_active,
			b.is_obligatory,
			COALESCE(d.is_voluntary, false),
			COALESCE(d.is_active, true),
			to_char(b.due_date, 'YYYY-MM-DD'),
			to_char(ho.start_date, 'YYYY-MM-DD'),
			to_char(ho.end_date, 'YYYY-MM-DD')
		 FROM bills b
		 JOIN dues d ON d.id = b.due_id
		 JOIN household_occupancies ho ON ho.id = b.household_occupancy_id
		 JOIN households h ON h.id = ho.household_id
		 JOIN residency_periods rp ON rp.household_occupancy_id = ho.id AND rp.end_date IS NULL
		 JOIN residents r ON r.id = rp.resident_id AND r.rt_id = b.rt_id AND r.is_active = true
		 JOIN users u ON (
			r.user_id = u.id
			OR (u.email IS NOT NULL AND u.email != '' AND lower(trim(r.email)) = lower(trim(u.email)))
			OR (u.phone IS NOT NULL AND u.phone != '' AND (
				r.phone = u.phone
				OR regexp_replace(r.phone, '^(\+62|62|0)', '') = regexp_replace(u.phone, '^(\+62|62|0)', '')
			))
		 ) AND u.is_active = true
		 JOIN user_rt_memberships urm ON urm.user_id = u.id AND urm.rt_id = b.rt_id AND urm.is_active = true
		 WHERE b.rt_id = $1
		   AND b.is_active = true
		   AND b.is_obligatory = true
		   AND b.status IN ('unpaid', 'partial')
		   AND b.due_date IS NOT NULL
		   AND d.is_active = true
		   AND d.is_voluntary = false
		   AND h.is_active = true
		   AND ho.end_date IS NULL
		 ORDER BY u.id, b.id, CASE WHEN r.user_id = u.id THEN 0 ELSE 1 END`,
		rtID,
	)
	if err != nil {
		return nil, fmt.Errorf("query rt user bills for evaluation: %w", err)
	}
	defer rows.Close()

	var result []BillEvaluationInput
	for rows.Next() {
		var (
			item                              BillEvaluationInput
			amountStr, approvedStr            string
			dueDateStr, moveInStr, moveOutStr sql.NullString
		)
		if err := rows.Scan(
			&item.BillID,
			&item.RTID,
			&item.UserID,
			&item.UserFullName,
			&item.HouseholdOccupancyID,
			&item.DueID,
			&item.DueName,
			&item.Period,
			&amountStr,
			&approvedStr,
			&item.Status,
			&item.IsActive,
			&item.IsObligatory,
			&item.IsVoluntary,
			&item.DueIsActive,
			&dueDateStr,
			&moveInStr,
			&moveOutStr,
		); err != nil {
			return nil, fmt.Errorf("scan rt user bill for evaluation: %w", err)
		}
		if cents, _, err := ParseMoney(amountStr); err == nil {
			item.AmountCents = cents
		}
		if cents, _, err := ParseMoney(approvedStr); err == nil {
			item.ApprovedPaidCents = cents
		}
		if dueDateStr.Valid && dueDateStr.String != "" {
			if dt, err := ParseDateJakarta(dueDateStr.String); err == nil {
				item.DueDate = &dt
			}
		}
		if moveInStr.Valid && moveInStr.String != "" {
			if dt, err := ParseDateJakarta(moveInStr.String); err == nil {
				item.MoveInDate = &dt
			}
		}
		if moveOutStr.Valid && moveOutStr.String != "" {
			if dt, err := ParseDateJakarta(moveOutStr.String); err == nil {
				item.MoveOutDate = &dt
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rt user bills for evaluation: %w", err)
	}

	// Sort deterministically by UserID, DueDate ASC, Period ASC, BillID ASC.
	sort.Slice(result, func(i, j int) bool {
		if result[i].UserID != result[j].UserID {
			return result[i].UserID < result[j].UserID
		}
		di, dj := time.Time{}, time.Time{}
		if result[i].DueDate != nil {
			di = *result[i].DueDate
		}
		if result[j].DueDate != nil {
			dj = *result[j].DueDate
		}
		if !di.Equal(dj) {
			return di.Before(dj)
		}
		if result[i].Period != result[j].Period {
			return result[i].Period < result[j].Period
		}
		return result[i].BillID < result[j].BillID
	})

	return result, nil
}
