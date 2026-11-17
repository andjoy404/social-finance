package notification_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"
)

func TestGetMonitoringHouseholdsUsesEligibleOverdueBills(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	db := pool.Raw()
	n := time.Now().UnixNano()
	createRT := func(label string) string {
		t.Helper()
		var rtID string
		err := db.QueryRowContext(ctx, `
			INSERT INTO rts (name, rw, rt, is_active)
			VALUES ($1, $2, $3, true) RETURNING id
		`, fmt.Sprintf("Monitoring %s %d", label, n), n%1000000000, fmt.Sprintf("M-%s-%d", label, n)).Scan(&rtID)
		require.NoError(t, err)
		return rtID
	}
	rtID := createRT("eligible")
	emptyRTID := createRT("empty")
	otherRTID := createRT("other")
	defer cleanupMonitoringTestRT(t, pool, rtID)
	defer cleanupMonitoringTestRT(t, pool, emptyRTID)
	defer cleanupMonitoringTestRT(t, pool, otherRTID)

	evalDate := time.Date(2026, 11, 6, 12, 0, 0, 0, finance.JakartaLocation())
	threshold := finance.AddCalendarMonths(evalDate, -3).AddDate(0, 0, -1).Format("2006-01-02")

	createDue := func(tenantID, name string, voluntary bool) string {
		t.Helper()
		var dueID string
		err := db.QueryRowContext(ctx, `
			INSERT INTO dues (rt_id, name, amount, period_type, is_active, is_voluntary)
			VALUES ($1, $2, 100000, 'monthly', true, $3)
			RETURNING id
		`, tenantID, fmt.Sprintf("%s-%d", name, n), voluntary).Scan(&dueID)
		require.NoError(t, err)
		return dueID
	}

	createOccupancy := func(tenantID, number, startDate string) string {
		t.Helper()
		var houseID, householdID, occupancyID string
		err := db.QueryRowContext(ctx, `
			INSERT INTO physical_houses (rt_id, house_number, is_active)
			VALUES ($1, $2, true) RETURNING id
		`, tenantID, fmt.Sprintf("%s-%d", number, n)).Scan(&houseID)
		require.NoError(t, err)
		err = db.QueryRowContext(ctx, `
			INSERT INTO households (rt_id, head_name, is_active)
			VALUES ($1, $2, true) RETURNING id
		`, tenantID, fmt.Sprintf("Monitoring household %d", n)).Scan(&householdID)
		require.NoError(t, err)
		err = db.QueryRowContext(ctx, `
			INSERT INTO household_occupancies
				(household_id, physical_house_id, occupancy_status, start_date)
			VALUES ($1, $2, 'OWNER', $3::date) RETURNING id
		`, householdID, houseID, startDate).Scan(&occupancyID)
		require.NoError(t, err)
		return occupancyID
	}

	createBill := func(tenantID, occupancyID, dueID, period, dueDate string) {
		t.Helper()
		_, err := db.ExecContext(ctx, `
			INSERT INTO bills
				(rt_id, household_occupancy_id, due_id, amount, period, due_date,
				 status, is_active, is_obligatory)
			VALUES ($1, $2, $3, 100000, $4, $5::date, 'unpaid', true, true)
		`, tenantID, occupancyID, dueID, period, dueDate)
		require.NoError(t, err)
	}

	obligatoryDueID := createDue(rtID, "Obligatory", false)
	eligibleOccupancy := createOccupancy(rtID, "ELIGIBLE", "2025-01-01")
	createBill(rtID, eligibleOccupancy, obligatoryDueID, "2026-08", threshold)

	preMoveInOccupancy := createOccupancy(rtID, "PRE-MOVE", "2026-01-01")
	createBill(rtID, preMoveInOccupancy, obligatoryDueID, "2025-12", threshold)

	voluntaryDueID := createDue(rtID, "Voluntary", true)
	voluntaryOccupancy := createOccupancy(rtID, "VOLUNTARY", "2025-01-01")
	createBill(rtID, voluntaryOccupancy, voluntaryDueID, "2026-08", threshold)

	otherRTDueID := createDue(otherRTID, "Other RT", false)
	otherRTOccupancy := createOccupancy(otherRTID, "OTHER-RT", "2025-01-01")
	createBill(otherRTID, otherRTOccupancy, otherRTDueID, "2026-08", threshold)

	list, err := notification.GetMonitoringHouseholds(ctx, pool, rtID, evalDate)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, eligibleOccupancy, list[0].HouseholdOccupancyID)

	emptyList, err := notification.GetMonitoringHouseholds(ctx, pool, emptyRTID, evalDate)
	require.NoError(t, err)
	require.Empty(t, emptyList)
}

func cleanupMonitoringTestRT(t *testing.T, pool *database.Pool, rtID string) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	for _, query := range []string{
		`DELETE FROM bills WHERE rt_id = $1`,
		`DELETE FROM dues WHERE rt_id = $1`,
		`DELETE FROM household_occupancies WHERE household_id IN (SELECT id FROM households WHERE rt_id = $1)`,
		`DELETE FROM physical_houses WHERE rt_id = $1`,
		`DELETE FROM households WHERE rt_id = $1`,
		`DELETE FROM rts WHERE id = $1`,
	} {
		if _, err := db.ExecContext(ctx, query, rtID); err != nil {
			t.Errorf("cleanup monitoring RT %s: %v", rtID, err)
		}
	}
}
