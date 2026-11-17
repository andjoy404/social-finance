package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
)

func mustDateWIB(t *testing.T, raw string) time.Time {
	t.Helper()
	d, err := ParseDateJakarta(raw)
	require.NoError(t, err, "failed to parse date %s", raw)
	return d
}

// 1 & 2 & 3. Calendar month arithmetic: -3 months, +3 months, and end-of-month edge cases
func TestAddCalendarMonthsAndThresholds(t *testing.T) {
	tests := []struct {
		name           string
		dueDate        string
		expectedUpc3m  string
		expectedOver3m string
	}{
		{
			name:           "mid-month standard (15 Oct -> 15 Jul upcoming, 15 Jan next year overdue)",
			dueDate:        "2026-10-15",
			expectedUpc3m:  "2026-07-15",
			expectedOver3m: "2027-01-15",
		},
		{
			name:           "mid-month standard (15 Jul -> 15 Apr upcoming, 15 Oct overdue)",
			dueDate:        "2026-07-15",
			expectedUpc3m:  "2026-04-15",
			expectedOver3m: "2026-10-15",
		},
		{
			name:           "end-of-month 31 Jan -> 31 Oct prev year upcoming, 30 Apr overdue",
			dueDate:        "2026-01-31",
			expectedUpc3m:  "2025-10-31",
			expectedOver3m: "2026-04-30",
		},
		{
			name:           "end-of-month 31 May -> 28 Feb non-leap upcoming, 31 Aug overdue",
			dueDate:        "2026-05-31",
			expectedUpc3m:  "2026-02-28",
			expectedOver3m: "2026-08-31",
		},
		{
			name:           "end-of-month 31 May in leap year 2028 -> 29 Feb upcoming, 31 Aug overdue",
			dueDate:        "2028-05-31",
			expectedUpc3m:  "2028-02-29",
			expectedOver3m: "2028-08-31",
		},
		{
			name:           "end-of-month 30 Nov -> 30 Aug upcoming, 28 Feb next year overdue",
			dueDate:        "2026-11-30",
			expectedUpc3m:  "2026-08-30",
			expectedOver3m: "2027-02-28",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			due := mustDateWIB(t, tc.dueDate)
			upc := UpcomingDue3MonthsTriggerDate(due)
			over := Overdue3MonthsThresholdDate(due)

			require.Equal(t, tc.expectedUpc3m, upc.Format("2006-01-02"))
			require.Equal(t, tc.expectedOver3m, over.Format("2006-01-02"))
		})
	}
}

// 4, 5, 6, 7. Upcoming 3 months window vs Overdue < 3m, == 3m, > 3m
func TestUpcomingAndOverdueDateEvaluation(t *testing.T) {
	// Due date: 2026-10-15 -> upcoming trigger is 2026-07-15
	dueOct15 := mustDateWIB(t, "2026-10-15")
	require.False(t, IsUpcomingDue3Months(dueOct15, mustDateWIB(t, "2026-07-14")), "1 day before 3-month window should be false")
	require.True(t, IsUpcomingDue3Months(dueOct15, mustDateWIB(t, "2026-07-15")), "exact 3 months before due date should be true")
	require.True(t, IsUpcomingDue3Months(dueOct15, mustDateWIB(t, "2026-08-01")), "within 3-month window for catch-up should be true")
	require.False(t, IsUpcomingDue3Months(dueOct15, mustDateWIB(t, "2026-10-15")), "on due date is no longer upcoming")
	require.False(t, IsUpcomingDue3Months(dueOct15, mustDateWIB(t, "2026-10-16")), "after due date is not upcoming")

	// Due date: 2026-07-15 -> overdue 3m threshold is 2026-10-15 (not 90 days which would be Oct 13)
	dueJul15 := mustDateWIB(t, "2026-07-15")
	require.False(t, IsOverdue3Months(dueJul15, mustDateWIB(t, "2026-10-13")), "90 days (Oct 13) is still < 3 calendar months")
	require.False(t, IsOverdue3Months(dueJul15, mustDateWIB(t, "2026-10-14")), "1 day before 3 calendar months should be false")
	require.False(t, IsOverdue3Months(dueJul15, mustDateWIB(t, "2026-10-15")), "exact 3 calendar months MUST be false (strictly > 3m)")
	require.True(t, IsOverdue3Months(dueJul15, mustDateWIB(t, "2026-10-16")), "3 calendar months + 1 day should be true")
	require.True(t, IsOverdue3Months(dueJul15, mustDateWIB(t, "2026-11-15")), "> 3 calendar months should be true")

	// Month-end clamping edge cases:
	// Due date 2026-01-31 -> 3 months is 2026-04-30 (April has 30 days)
	dueJan31 := mustDateWIB(t, "2026-01-31")
	require.False(t, IsOverdue3Months(dueJan31, mustDateWIB(t, "2026-04-29")), "before 3 calendar months should be false")
	require.False(t, IsOverdue3Months(dueJan31, mustDateWIB(t, "2026-04-30")), "exact month-end threshold (Apr 30) MUST be false")
	require.True(t, IsOverdue3Months(dueJan31, mustDateWIB(t, "2026-05-01")), "1 day after clamped month-end (May 1) should be true")

	// Leap year case:
	// Due date 2023-11-30 -> 3 months in leap year 2024 is 2024-02-29
	dueNov30Leap := mustDateWIB(t, "2023-11-30")
	require.False(t, IsOverdue3Months(dueNov30Leap, mustDateWIB(t, "2024-02-28")), "before 3 calendar months should be false")
	require.False(t, IsOverdue3Months(dueNov30Leap, mustDateWIB(t, "2024-02-29")), "exact leap year threshold (Feb 29) MUST be false")
	require.True(t, IsOverdue3Months(dueNov30Leap, mustDateWIB(t, "2024-03-01")), "1 day after leap year threshold (Mar 1) should be true")

	// Non-leap year case:
	// Due date 2025-11-30 -> 3 months in non-leap year 2026 is 2026-02-28
	dueNov30NonLeap := mustDateWIB(t, "2025-11-30")
	require.False(t, IsOverdue3Months(dueNov30NonLeap, mustDateWIB(t, "2026-02-27")), "before 3 calendar months should be false")
	require.False(t, IsOverdue3Months(dueNov30NonLeap, mustDateWIB(t, "2026-02-28")), "exact non-leap year threshold (Feb 28) MUST be false")
	require.True(t, IsOverdue3Months(dueNov30NonLeap, mustDateWIB(t, "2026-03-01")), "1 day after non-leap year threshold (Mar 1) should be true")

	// Year boundary transition case:
	dueNov15 := mustDateWIB(t, "2025-11-15")
	require.False(t, IsOverdue3Months(dueNov15, mustDateWIB(t, "2026-02-15")), "exact 3 calendar months across year boundary MUST be false")
	require.True(t, IsOverdue3Months(dueNov15, mustDateWIB(t, "2026-02-16")), "3 calendar months + 1 day across year boundary should be true")

	// Timezone boundary case (Asia/Jakarta):
	// Due date 2026-07-15 -> threshold 2026-10-15
	// 2026-10-15 16:59:00 UTC = 2026-10-15 23:59:00 WIB (same day in WIB -> exact 3m) -> false
	require.False(t, IsOverdue3Months(dueJul15, time.Date(2026, 10, 15, 16, 59, 0, 0, time.UTC)))
	// 2026-10-15 17:01:00 UTC = 2026-10-16 00:01:00 WIB (next day in WIB -> 3m + 1 day) -> true
	require.True(t, IsOverdue3Months(dueJul15, time.Date(2026, 10, 15, 17, 1, 0, 0, time.UTC)))
}

// 8, 9, 10, 11, 12, 13, 14. Bill eligibility, payment statuses (APPROVED/PENDING/REJECTED/partial), is_obligatory, move_in_date, and Asia/Jakarta timezone
func TestBillEligibilityAndPaymentFormula(t *testing.T) {
	dueJul15 := mustDateWIB(t, "2026-07-15")
	evalOct15 := mustDateWIB(t, "2026-10-15") // Exact 3 calendar months
	evalOct16 := mustDateWIB(t, "2026-10-16") // Strictly > 3 calendar months (3m + 1 day)

	baseBill := BillEvaluationInput{
		BillID:               "bill-1",
		RTID:                 "rt-1",
		HouseholdOccupancyID: "occ-1",
		DueID:                "due-1",
		DueName:              "Iuran Kebersihan",
		Period:               "2026-07",
		AmountCents:          15000000, // Rp 150.000,00
		ApprovedPaidCents:    0,
		Status:               "unpaid",
		DueDate:              &dueJul15,
		IsObligatory:         true,
		IsActive:             true,
		IsVoluntary:          false,
		DueIsActive:          true,
	}

	evalOverdue := func(b BillEvaluationInput, eval time.Time) bool {
		_, ok := EvaluateBillForOverdue3Months(b, eval)
		return ok
	}

	// Boundary check: On exact 3 calendar months (evalOct15), even unpaid bill MUST be false
	require.False(t, evalOverdue(baseBill, evalOct15), "exact 3 calendar months MUST be false")
	// On 3 calendar months + 1 day (evalOct16), unpaid bill is true
	require.True(t, evalOverdue(baseBill, evalOct16), "3 calendar months + 1 day MUST be true")

	// 8. Full APPROVED payment -> not eligible on evalOct16
	paidBill := baseBill
	paidBill.ApprovedPaidCents = 15000000
	paidBill.Status = "paid"
	require.Equal(t, int64(0), CalculateOutstandingCents(paidBill))
	require.False(t, evalOverdue(paidBill, evalOct16))

	// Even if status wasn't flipped yet, 0 outstanding means not eligible
	zeroOutBill := baseBill
	zeroOutBill.ApprovedPaidCents = 15000000
	require.Equal(t, int64(0), CalculateOutstandingCents(zeroOutBill))
	require.False(t, evalOverdue(zeroOutBill, evalOct16))

	// 9. Partial APPROVED payment -> eligible with reduced outstanding
	partialBill := baseBill
	partialBill.ApprovedPaidCents = 5000000 // Rp 50.000,00 paid
	partialBill.Status = "partial"
	require.Equal(t, int64(10000000), CalculateOutstandingCents(partialBill))
	require.False(t, evalOverdue(partialBill, evalOct15), "exact 3m partial bill is false")
	require.True(t, evalOverdue(partialBill, evalOct16), "> 3m partial bill is true")

	// 10 & 11. PENDING and REJECTED payments do not increase ApprovedPaidCents -> full balance remains
	unverifiedBill := baseBill
	unverifiedBill.ApprovedPaidCents = 0
	unverifiedBill.PendingPaidCents = 5000000
	unverifiedBill.RejectedPaidCents = 5000000
	require.Equal(t, int64(15000000), CalculateOutstandingCents(unverifiedBill))
	require.True(t, evalOverdue(unverifiedBill, evalOct16))

	// 12. Non-obligatory or voluntary bill -> excluded
	nonObligBill := baseBill
	nonObligBill.IsObligatory = false
	require.False(t, evalOverdue(nonObligBill, evalOct16))

	voluntaryBill := baseBill
	voluntaryBill.IsVoluntary = true
	require.False(t, evalOverdue(voluntaryBill, evalOct16))

	// 13. Bill before move_in_date -> excluded
	moveInAug := mustDateWIB(t, "2026-08-01")
	beforeMoveInBill := baseBill
	beforeMoveInBill.MoveInDate = &moveInAug // period is 2026-07, due is 2026-07-15, move-in is 2026-08-01
	require.False(t, evalOverdue(beforeMoveInBill, evalOct16))

	// Bill on/after move_in_date -> included on evalOct16
	moveInJul1 := mustDateWIB(t, "2026-07-01")
	afterMoveInBill := baseBill
	afterMoveInBill.MoveInDate = &moveInJul1
	require.True(t, evalOverdue(afterMoveInBill, evalOct16))

	// 14. Asia/Jakarta timezone boundary:
	// Due date 2026-07-15 -> threshold date is 2026-10-15.
	// 2026-10-15 16:59:00 UTC = 2026-10-15 23:59:00 WIB (exact 3m in WIB) -> false
	utcLateOct15Exact := time.Date(2026, 10, 15, 16, 59, 0, 0, time.UTC)
	require.False(t, evalOverdue(baseBill, NormalizeDateJakarta(utcLateOct15Exact)))
	// 2026-10-15 17:30:00 UTC = 2026-10-16 00:30:00 WIB (3m + 1 day in WIB) -> true
	utcLateOct15Over := time.Date(2026, 10, 15, 17, 30, 0, 0, time.UTC)
	normalizedWIB := NormalizeDateJakarta(utcLateOct15Over)
	require.Equal(t, "2026-10-16", normalizedWIB.Format("2006-01-02"))
	require.True(t, evalOverdue(baseBill, normalizedWIB))
}

// 15. Integration test for Shared DB Queries & GET /api/v1/bills/overdue-summary
func TestOverdueWarningSummaryEndpointAndDatabaseIntegration(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()
	db := pool.Raw()

	rtA := setupTestRT(t, pool, "RT Overdue A", 1, "OVA")
	rtB := setupTestRT(t, pool, "RT Overdue B", 1, "OVB")
	defer cleanupTestRT(t, pool, rtA)
	defer cleanupTestRT(t, pool, rtB)

	n := atomic.AddInt64(&rtCounter, 1)

	createWargaWithOccupancy := func(rtID, suffix, role, jabatan, moveIn string) (userID, mid, occID string) {
		phone := fmt.Sprintf("0812%04d%s", n%10000, suffix)
		var houseID, hhID, resID string
		err := db.QueryRowContext(ctx,
			`INSERT INTO physical_houses (rt_id, house_number, address) VALUES ($1, $2, 'Jl Test') RETURNING id`,
			rtID, fmt.Sprintf("H-%s-%d", suffix, n),
		).Scan(&houseID)
		require.NoError(t, err)

		err = db.QueryRowContext(ctx,
			`INSERT INTO households (rt_id, head_name) VALUES ($1, $2) RETURNING id`,
			rtID, "Warga "+suffix,
		).Scan(&hhID)
		require.NoError(t, err)

		err = db.QueryRowContext(ctx,
			`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
			 VALUES ($1, $2, 'OWNER', $3::date) RETURNING id`,
			hhID, houseID, moveIn,
		).Scan(&occID)
		require.NoError(t, err)

		err = db.QueryRowContext(ctx,
			`INSERT INTO residents (rt_id, full_name, phone, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
			rtID, "Warga "+suffix, phone,
		).Scan(&resID)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx,
			`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
			 VALUES ($1, $2, $3::date)`,
			resID, occID, moveIn,
		)
		require.NoError(t, err)

		userID, mid = setupTestUserWithMembership(t, pool, fmt.Sprintf("warga-%s-%d@test.local", suffix, n), "Warga "+suffix, phone, rtID, role)
		if jabatan != "" {
			_, err = db.ExecContext(ctx, `UPDATE user_rt_memberships SET jabatan = $1 WHERE id = $2`, jabatan, mid)
			require.NoError(t, err)
		}
		return userID, mid, occID
	}

	warga1UserID, warga1MID, warga1OccID := createWargaWithOccupancy(rtA, "11", "warga", "", "2025-01-01")
	warga2UserID, warga2MID, _ := createWargaWithOccupancy(rtA, "22", "warga", "", "2025-01-01")
	ketuaUserID, ketuaMID, ketuaOccID := createWargaWithOccupancy(rtA, "33", "warga", "ketua", "2025-01-01")
	sekretarisUserID, sekretarisMID, sekretarisOccID := createWargaWithOccupancy(rtA, "55", "warga", "sekretaris", "2025-01-01")
	sosialUserID, sosialMID, _ := createWargaWithOccupancy(rtA, "66", "warga", "sosial", "2025-01-01")
	rtBSekretarisUserID, rtBSekretarisMID, _ := createWargaWithOccupancy(rtB, "77", "warga", "sekretaris", "2025-01-01")
	rtBWargaUserID, _, rtBOccID := createWargaWithOccupancy(rtB, "44", "warga", "", "2025-01-01")

	defer func() {
		for _, uid := range []string{warga1UserID, warga2UserID, ketuaUserID, sekretarisUserID, sosialUserID, rtBSekretarisUserID, rtBWargaUserID} {
			_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE user_id = $1`, uid)
			_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, uid)
		}
	}()

	createTestDue := func(rtID string) string {
		var dueID string
		err := db.QueryRowContext(ctx,
			`INSERT INTO dues (rt_id, name, amount, period_type, is_active, is_voluntary)
			 VALUES ($1, 'Iuran Bulanan', 150000.00, 'monthly', true, false) RETURNING id`,
			rtID,
		).Scan(&dueID)
		require.NoError(t, err)
		return dueID
	}

	dueID := createTestDue(rtA)
	dueIDB := createTestDue(rtB)
	// Clear any bills auto-generated by the dues trigger so test assertions are deterministic
	_, err := db.ExecContext(ctx, `DELETE FROM bills WHERE rt_id IN ($1, $2)`, rtA, rtB)
	require.NoError(t, err)

	// Today in Asia/Jakarta
	today := NormalizeDateJakarta(time.Now())
	due4MonthsAgo := AddCalendarMonths(today, -4).Format("2006-01-02")
	due3MonthsPlus1DayAgo := AddCalendarMonths(today, -3).AddDate(0, 0, -1).Format("2006-01-02")
	dueExact3MonthsAgo := AddCalendarMonths(today, -3).Format("2006-01-02")
	due2MonthsAgo := AddCalendarMonths(today, -2).Format("2006-01-02")

	// Bill 1 for Warga 1: overdue 4 months, Rp 150.000, partial APPROVED Rp 50.000 + PENDING Rp 25.000 + REJECTED Rp 25.000 -> remaining Rp 100.000
	var bill1ID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2025-05', 150000.00, 'partial', $4::date, true, true) RETURNING id`,
		rtA, warga1OccID, dueID, due4MonthsAgo,
	).Scan(&bill1ID)
	require.NoError(t, err)

	// Approved payment 50.000
	var payApprovedID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, submitted_by)
		 VALUES ($1, $2, 50000.00, 'TRANSFER', 'SELF_SUBMITTED', 'APPROVED', $3) RETURNING id`,
		rtA, bill1ID, warga1UserID,
	).Scan(&payApprovedID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO payment_allocations (payment_id, bill_id, amount) VALUES ($1, $2, 50000.00)`,
		payApprovedID, bill1ID,
	)
	require.NoError(t, err)

	// Pending payment 25.000 (must NOT reduce balance)
	var payPendingID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, submitted_by)
		 VALUES ($1, $2, 25000.00, 'TRANSFER', 'SELF_SUBMITTED', 'PENDING', $3) RETURNING id`,
		rtA, bill1ID, warga1UserID,
	).Scan(&payPendingID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO payment_allocations (payment_id, bill_id, amount) VALUES ($1, $2, 25000.00)`,
		payPendingID, bill1ID,
	)
	require.NoError(t, err)

	// Rejected payment 25.000 (must NOT reduce balance)
	var payRejectedID string
	err = db.QueryRowContext(ctx,
		`INSERT INTO payments (rt_id, bill_id, amount, method, origin, status, submitted_by)
		 VALUES ($1, $2, 25000.00, 'TRANSFER', 'SELF_SUBMITTED', 'REJECTED', $3) RETURNING id`,
		rtA, bill1ID, warga1UserID,
	).Scan(&payRejectedID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO payment_allocations (payment_id, bill_id, amount) VALUES ($1, $2, 25000.00)`,
		payRejectedID, bill1ID,
	)
	require.NoError(t, err)

	// Bill 2 for Warga 1: overdue strictly > 3 months (3 months + 1 day), Rp 200.000 unpaid -> remaining Rp 200.000 (ELIGIBLE)
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2025-06', 200000.00, 'unpaid', $4::date, true, true)`,
		rtA, warga1OccID, dueID, due3MonthsPlus1DayAgo,
	)
	require.NoError(t, err)

	// Bill 2B for Warga 1: overdue EXACT 3 calendar months, Rp 180.000 unpaid -> must NOT be counted in 3m overdue summary
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2025-07', 180000.00, 'unpaid', $4::date, true, true)`,
		rtA, warga1OccID, dueID, dueExact3MonthsAgo,
	)
	require.NoError(t, err)

	// Bill 3 for Warga 1: overdue 2 months (< 3 months), Rp 100.000 -> must NOT be counted in 3m overdue summary
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2025-08', 100000.00, 'unpaid', $4::date, true, true)`,
		rtA, warga1OccID, dueID, due2MonthsAgo,
	)
	require.NoError(t, err)

	// Bill 4 for Warga 1: overdue 4 months but is_obligatory = false -> must NOT be counted
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2024-12', 300000.00, 'unpaid', $4::date, false, true)`,
		rtA, warga1OccID, dueID, due4MonthsAgo,
	)
	require.NoError(t, err)

	// Bill for Ketua RT (overdue 4 months) — ketua's own occupancy.
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2025-05', 500000.00, 'unpaid', $4::date, true, true)`,
		rtA, ketuaOccID, dueID, due4MonthsAgo,
	)
	require.NoError(t, err)

	// Bill for Sekretaris RT A (overdue 4 months) — personal occupancy.
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2025-05', 250000.00, 'unpaid', $4::date, true, true)`,
		rtA, sekretarisOccID, dueID, due4MonthsAgo,
	)
	require.NoError(t, err)

	// Bill for RT B Warga (overdue 4 months, Rp 750.000) -> must not leak into RT A
	_, err = db.ExecContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, due_date, is_obligatory, is_active)
		 VALUES ($1, $2, $3, '2025-05', 750000.00, 'unpaid', $4::date, true, true)`,
		rtB, rtBOccID, dueIDB, due4MonthsAgo,
	)
	require.NoError(t, err)

	router := setupFinanceRouter(pool)

	// Case A: Unauthenticated -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/bills/overdue-summary", nil)
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)
	require.Equal(t, http.StatusUnauthorized, recUnauth.Code)

	// Case B: Warga 1 (has 2 qualifying overdue >= 3m bills: Rp 100.000 + Rp 200.000 = Rp 300.000)
	// Also pass spoofed user_id and rt_id query params to verify they are ignored!
	warga1Token := makeToken(t, auth.TokenClaims{
		UserID: warga1UserID,
		MID:    warga1MID,
		RTID:   rtA,
		Role:   string(auth.RoleWarga),
	})
	reqW1 := httptest.NewRequest(http.MethodGet, "/api/v1/bills/overdue-summary?user_id="+rtBWargaUserID+"&rt_id="+rtB, nil)
	reqW1.Header.Set("Authorization", "Bearer "+warga1Token)
	recW1 := httptest.NewRecorder()
	router.ServeHTTP(recW1, reqW1)
	require.Equal(t, http.StatusOK, recW1.Code)

	var sumW1 OverdueWarningSummary
	require.NoError(t, json.Unmarshal(recW1.Body.Bytes(), &sumW1))
	require.True(t, sumW1.HasOverdue3Months)
	require.Equal(t, int64(300000), sumW1.TotalOutstanding)
	require.Equal(t, 2, sumW1.OverdueBillCount)

	// Case C: Warga 2 (no overdue bills) -> has_overdue_3_months: false
	warga2Token := makeToken(t, auth.TokenClaims{
		UserID: warga2UserID,
		MID:    warga2MID,
		RTID:   rtA,
		Role:   string(auth.RoleWarga),
	})
	reqW2 := httptest.NewRequest(http.MethodGet, "/api/v1/bills/overdue-summary", nil)
	reqW2.Header.Set("Authorization", "Bearer "+warga2Token)
	recW2 := httptest.NewRecorder()
	router.ServeHTTP(recW2, reqW2)
	require.Equal(t, http.StatusOK, recW2.Code)

	var sumW2 OverdueWarningSummary
	require.NoError(t, json.Unmarshal(recW2.Body.Bytes(), &sumW2))
	require.False(t, sumW2.HasOverdue3Months)
	require.Equal(t, int64(0), sumW2.TotalOutstanding)
	require.Equal(t, 0, sumW2.OverdueBillCount)

	// Case D: Ketua RT 002 (jabatan=ketua with finance.read) -> PERSONAL ONLY:
	// only sees own occupancy (Rp 500.000 / 1 bill), not Warga 1's Rp 300.000.
	ketuaToken := makeToken(t, auth.TokenClaims{
		UserID:  ketuaUserID,
		MID:     ketuaMID,
		RTID:    rtA,
		Role:    string(auth.RoleWarga),
		Jabatan: "ketua",
	})
	reqKetua := httptest.NewRequest(http.MethodGet, "/api/v1/bills/overdue-summary", nil)
	reqKetua.Header.Set("Authorization", "Bearer "+ketuaToken)
	recKetua := httptest.NewRecorder()
	router.ServeHTTP(recKetua, reqKetua)
	require.Equal(t, http.StatusOK, recKetua.Code)

	var sumKetua OverdueWarningSummary
	require.NoError(t, json.Unmarshal(recKetua.Body.Bytes(), &sumKetua))
	require.True(t, sumKetua.HasOverdue3Months)
	require.Equal(t, int64(500000), sumKetua.TotalOutstanding)
	require.Equal(t, 1, sumKetua.OverdueBillCount)

	// Case E: Sekretaris RT A (jabatan=sekretaris with finance.read) -> PERSONAL ONLY:
	// only sees own occupancy (Rp 250.000 / 1 bill), not Warga 1's or Ketua's bills.
	sekretarisToken := makeToken(t, auth.TokenClaims{
		UserID:  sekretarisUserID,
		MID:     sekretarisMID,
		RTID:    rtA,
		Role:    string(auth.RoleWarga),
		Jabatan: "sekretaris",
	})
	reqSek := httptest.NewRequest(http.MethodGet, "/api/v1/bills/overdue-summary", nil)
	reqSek.Header.Set("Authorization", "Bearer "+sekretarisToken)
	recSek := httptest.NewRecorder()
	router.ServeHTTP(recSek, reqSek)
	require.Equal(t, http.StatusOK, recSek.Code)

	var sumSek OverdueWarningSummary
	require.NoError(t, json.Unmarshal(recSek.Body.Bytes(), &sumSek))
	require.True(t, sumSek.HasOverdue3Months)
	require.Equal(t, int64(250000), sumSek.TotalOutstanding)
	require.Equal(t, 1, sumSek.OverdueBillCount)

	// Case F: Pengurus RT A WITHOUT finance.read permission (jabatan=sosial) ->
	// personal scope only (no exception for permissions).
	sosialToken := makeToken(t, auth.TokenClaims{
		UserID:  sosialUserID,
		MID:     sosialMID,
		RTID:    rtA,
		Role:    string(auth.RoleWarga),
		Jabatan: "sosial",
	})
	reqSos := httptest.NewRequest(http.MethodGet, "/api/v1/bills/overdue-summary", nil)
	reqSos.Header.Set("Authorization", "Bearer "+sosialToken)
	recSos := httptest.NewRecorder()
	router.ServeHTTP(recSos, reqSos)
	require.Equal(t, http.StatusOK, recSos.Code)

	var sumSos OverdueWarningSummary
	require.NoError(t, json.Unmarshal(recSos.Body.Bytes(), &sumSos))
	require.False(t, sumSos.HasOverdue3Months)
	require.Equal(t, int64(0), sumSos.TotalOutstanding)
	require.Equal(t, 0, sumSos.OverdueBillCount)

	// Case G: Sekretaris RT B (jabatan=sekretaris) -> PERSONAL ONLY: RT A bills
	// (Warga 1's Rp 300.000, Ketua's Rp 500.000, Sekretaris RT A's Rp 250.000, jadilah)
	// are invisible. RT B's occupancy has no overdue bills -> false.
	rtBSekretarisToken := makeToken(t, auth.TokenClaims{
		UserID:  rtBSekretarisUserID,
		MID:     rtBSekretarisMID,
		RTID:    rtB,
		Role:    string(auth.RoleWarga),
		Jabatan: "sekretaris",
	})
	reqRtBSek := httptest.NewRequest(http.MethodGet, "/api/v1/bills/overdue-summary", nil)
	reqRtBSek.Header.Set("Authorization", "Bearer "+rtBSekretarisToken)
	recRtBSek := httptest.NewRecorder()
	router.ServeHTTP(recRtBSek, reqRtBSek)
	require.Equal(t, http.StatusOK, recRtBSek.Code)

	var sumRtBSek OverdueWarningSummary
	require.NoError(t, json.Unmarshal(recRtBSek.Body.Bytes(), &sumRtBSek))
	require.False(t, sumRtBSek.HasOverdue3Months)
	require.Equal(t, int64(0), sumRtBSek.TotalOutstanding)
	require.Equal(t, 0, sumRtBSek.OverdueBillCount)
}
