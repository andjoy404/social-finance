package migration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Check represents a single preflight validation result.
type Check struct {
	Name             string  `json:"name"`
	Severity         string  `json:"severity"` // "blocking" or "info"
	Message          string  `json:"message"`
	HouseholdID      *string `json:"household_id,omitempty"`
	HouseNumber      *string `json:"house_number,omitempty"`
	ResidentID       *string `json:"resident_id,omitempty"`
	ResidentName     *string `json:"resident_name,omitempty"`
	RTID             *string `json:"rt_id,omitempty"`
	CrossRT          *string `json:"cross_rt,omitempty"`
	CurrentVal       *string `json:"current_value,omitempty"`
	AssociatedHHUUID *string `json:"associated_hh_uuid,omitempty"`
}

// PreflightResult holds all preflight check outcomes.
type PreflightResult struct {
	Checks             []Check
	BlockingCount      int
	TotalHouseholds    int
	ActiveHouseholds   int
	InactiveHouseholds int
	TotalResidents     int
	ActiveResidents    int
	InactiveResidents  int
	ActiveHouseNumbers int
}

// Run executes all preflight checks against the v7 schema.
// It is strictly read-only: no INSERT, UPDATE, DELETE, or schema mutation.
func Run(ctx context.Context, db *sql.DB) (*PreflightResult, error) {
	result := &PreflightResult{}

	checks, err := runBlockingChecks(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("preflight: %w", err)
	}

	result.Checks = checks
	for _, c := range checks {
		if c.Severity == "blocking" {
			result.BlockingCount++
		}
	}

	// Collect informational counts (non-blocking).
	infoChecks, err := runInfoChecks(ctx, db, result)
	if err != nil {
		return nil, fmt.Errorf("preflight info checks: %w", err)
	}
	result.Checks = append(result.Checks, infoChecks...)

	return result, nil
}

func runBlockingChecks(ctx context.Context, db *sql.DB) ([]Check, error) {
	var checks []Check

	// C1: Active households with NULL or blank house_number.
	rows, err := db.QueryContext(ctx, `
		SELECT id, rt_id, COALESCE(house_number, '')
		FROM households
		WHERE is_active = true
		  AND (house_number IS NULL OR TRIM(house_number) = '')
	`)
	if err != nil {
		return nil, fmt.Errorf("check missing house number: %w", err)
	}
	var missingHN []struct {
		ID   string
		RTID string
		HN   string
	}
	for rows.Next() {
		var r struct {
			ID   string
			RTID string
			HN   sql.NullString
		}
		if err := rows.Scan(&r.ID, &r.RTID, &r.HN); err != nil {
			return nil, fmt.Errorf("scan missing house number: %w", err)
		}
		missingHN = append(missingHN, struct {
			ID   string
			RTID string
			HN   string
		}{r.ID, r.RTID, r.HN.String})
	}
	rows.Close()

	for _, r := range missingHN {
		checks = append(checks, Check{
			Name:        "missing_active_house_number",
			Severity:    "blocking",
			Message:     fmt.Sprintf("Active household has NULL/blank house_number: %q", r.HN),
			HouseholdID: &r.ID,
			RTID:        &r.RTID,
		})
	}

	// C2: Duplicate active house numbers within the same RT.
	rows, err = db.QueryContext(ctx, `
 		SELECT rt_id, TRIM(house_number) AS hn, array_agg(id) AS hids
 		FROM households
 		WHERE is_active = true AND house_number IS NOT NULL
 		GROUP BY rt_id, TRIM(house_number)
 		HAVING COUNT(*) > 1
 	`)
	if err != nil {
		return nil, fmt.Errorf("check duplicate house number: %w", err)
	}
	type dupRow struct {
		RTID  string
		HN    string
		HIDsS string
	}
	var dups []dupRow
	for rows.Next() {
		var r dupRow
		if err := rows.Scan(&r.RTID, &r.HN, &r.HIDsS); err != nil {
			return nil, fmt.Errorf("scan duplicate house number: %w", err)
		}
		dups = append(dups, r)
	}
	rows.Close()

	for _, r := range dups {
		hids := parsePgArray(r.HIDsS)
		for _, hid := range hids {
			checks = append(checks, Check{
				Name:        "duplicate_active_house_number",
				Severity:    "blocking",
				Message:     fmt.Sprintf("Duplicate active house_number %q in RT %s: household %s", r.HN, r.RTID, hid),
				HouseholdID: &hid,
				HouseNumber: &r.HN,
			})
		}
	}

	// C3: Active households with invalid occupancy_status.
	rows, err = db.QueryContext(ctx, `
		SELECT id, rt_id, COALESCE(occupancy_status, '<NULL>') AS os
		FROM households
		WHERE is_active = true
		  AND (occupancy_status IS NULL
		       OR occupancy_status NOT IN ('OWNER', 'TENANT'))
	`)
	if err != nil {
		return nil, fmt.Errorf("check invalid occupancy status: %w", err)
	}
	var invalidOS []struct {
		ID   string
		RTID string
		OS   string
	}
	for rows.Next() {
		var r struct {
			ID   string
			RTID string
			OS   string
		}
		if err := rows.Scan(&r.ID, &r.RTID, &r.OS); err != nil {
			return nil, fmt.Errorf("scan invalid occupancy status: %w", err)
		}
		invalidOS = append(invalidOS, r)
	}
	rows.Close()

	for _, r := range invalidOS {
		checks = append(checks, Check{
			Name:        "invalid_active_occupancy_status",
			Severity:    "blocking",
			Message:     fmt.Sprintf("Active household has invalid occupancy_status: %q", r.OS),
			HouseholdID: &r.ID,
			RTID:        &r.RTID,
			CurrentVal:  &r.OS,
		})
	}

	// C4: Orphan Resident (household_id references non-existent household).
	// FK constraint may prevent this, but preflight explicitly proves the invariant.
	rows, err = db.QueryContext(ctx, `
		SELECT r.id, r.rt_id, r.household_id
		FROM residents r
		LEFT JOIN households h ON r.household_id = h.id
		WHERE h.id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("check orphan resident household: %w", err)
	}
	var orphans []struct {
		RID  string
		RTID string
		HHID string
	}
	for rows.Next() {
		var r struct {
			RID  string
			RTID string
			HHID sql.NullString
		}
		if err := rows.Scan(&r.RID, &r.RTID, &r.HHID); err != nil {
			return nil, fmt.Errorf("scan orphan resident: %w", err)
		}
		orphans = append(orphans, struct {
			RID  string
			RTID string
			HHID string
		}{r.RID, r.RTID, r.HHID.String})
	}
	rows.Close()

	for _, r := range orphans {
		checks = append(checks, Check{
			Name:       "orphan_resident_household",
			Severity:   "blocking",
			Message:    fmt.Sprintf("Resident %s references non-existent household %s", r.RID, r.HHID),
			ResidentID: &r.RID,
			RTID:       &r.RTID,
			CrossRT:    &r.HHID,
		})
	}

	// C5: Invalid Household RT (rt_id references non-existent RT).
	rows, err = db.QueryContext(ctx, `
		SELECT h.id, h.rt_id
		FROM households h
		LEFT JOIN rts r ON h.rt_id = r.id
		WHERE r.id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("check invalid household rt: %w", err)
	}
	type invalidHouseholdRT struct {
		ID   string
		RTID string
	}
	var invalidHRt []invalidHouseholdRT
	for rows.Next() {
		var r invalidHouseholdRT
		if err := rows.Scan(&r.ID, &r.RTID); err != nil {
			return nil, fmt.Errorf("scan invalid household rt: %w", err)
		}
		invalidHRt = append(invalidHRt, r)
	}
	rows.Close()

	for _, r := range invalidHRt {
		checks = append(checks, Check{
			Name:        "invalid_household_rt",
			Severity:    "blocking",
			Message:     fmt.Sprintf("Household %s references non-existent RT %s", r.ID, r.RTID),
			HouseholdID: &r.ID,
			RTID:        &r.RTID,
		})
	}

	// C6: Invalid Resident RT.
	rows, err = db.QueryContext(ctx, `
		SELECT r.id, r.rt_id
		FROM residents r
		LEFT JOIN rts t ON r.rt_id = t.id
		WHERE t.id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("check invalid resident rt: %w", err)
	}
	type invalidResidentRT struct {
		ID   string
		RTID string
	}
	var invalidRRt []invalidResidentRT
	for rows.Next() {
		var r invalidResidentRT
		if err := rows.Scan(&r.ID, &r.RTID); err != nil {
			return nil, fmt.Errorf("scan invalid resident rt: %w", err)
		}
		invalidRRt = append(invalidRRt, r)
	}
	rows.Close()

	for _, r := range invalidRRt {
		checks = append(checks, Check{
			Name:       "invalid_resident_rt",
			Severity:   "blocking",
			Message:    fmt.Sprintf("Resident %s references non-existent RT %s", r.ID, r.RTID),
			ResidentID: &r.ID,
			RTID:       &r.RTID,
		})
	}

	// C7: Cross-RT Resident -> Household (resident.rt_id != household.rt_id).
	rows, err = db.QueryContext(ctx, `
		SELECT r.id, r.rt_id, r.household_id, h.rt_id AS hh_rt_id
		FROM residents r
		JOIN households h ON r.household_id = h.id
		WHERE r.rt_id != h.rt_id
	`)
	if err != nil {
		return nil, fmt.Errorf("check cross-rt resident household: %w", err)
	}
	var crossRT []struct {
		RID  string
		RRT  string
		HHID string
		HHRT string
	}
	for rows.Next() {
		var r struct {
			RID  string
			RRT  string
			HHID string
			HHRT sql.NullString
		}
		if err := rows.Scan(&r.RID, &r.RRT, &r.HHID, &r.HHRT); err != nil {
			return nil, fmt.Errorf("scan cross-rt: %w", err)
		}
		crossRT = append(crossRT, struct {
			RID  string
			RRT  string
			HHID string
			HHRT string
		}{r.RID, r.RRT, r.HHID, r.HHRT.String})
	}
	rows.Close()

	for _, r := range crossRT {
		checks = append(checks, Check{
			Name:       "cross_rt_resident_household",
			Severity:   "blocking",
			Message:    fmt.Sprintf("Resident %s (RT %s) -> Household %s (RT %s)", r.RID, r.RRT, r.HHID, r.HHRT),
			ResidentID: &r.RID,
			RTID:       &r.RRT,
			CrossRT:    &r.HHID,
		})
	}

	// C8: Active Resident -> inactive Household.
	// Migration 008 residency creation only joins active residents whose
	// household has a valid current occupancy. Inactive households receive
	// no occupancy, so active residents assigned to them cannot be migrated.
	rows, err = db.QueryContext(ctx, `
		SELECT r.id, r.full_name, r.household_id, h.house_number, h.rt_id
		FROM residents r
		JOIN households h ON r.household_id = h.id
		WHERE r.is_active = true
		  AND h.is_active = false
	`)
	if err != nil {
		return nil, fmt.Errorf("check active resident inactive household: %w", err)
	}
	var activeResInactiveHH []struct {
		RID   string
		RName string
		HHID  string
		HN    string
		HHRT  string
	}
	for rows.Next() {
		var r struct {
			RID   string
			RName string
			HHID  string
			HN    sql.NullString
			HHRT  string
		}
		if err := rows.Scan(&r.RID, &r.RName, &r.HHID, &r.HN, &r.HHRT); err != nil {
			return nil, fmt.Errorf("scan active resident inactive household: %w", err)
		}
		hn := r.HN.String
		activeResInactiveHH = append(activeResInactiveHH, struct {
			RID   string
			RName string
			HHID  string
			HN    string
			HHRT  string
		}{r.RID, r.RName, r.HHID, hn, r.HHRT})
	}
	rows.Close()

	for _, r := range activeResInactiveHH {
		checks = append(checks, Check{
			Name:             "active_resident_inactive_household",
			Severity:         "blocking",
			Message:          fmt.Sprintf("Active resident %s (%s) assigned to inactive household %s (house_number=%q)", r.RID, r.RName, r.HHID, r.HN),
			ResidentID:       &r.RID,
			ResidentName:     &r.RName,
			AssociatedHHUUID: &r.HHID,
			HouseNumber:      &r.HN,
			RTID:             &r.HHRT,
		})
	}

	return checks, nil
}

func parsePgArray(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil
	}
	inner := s[1 : len(s)-1]
	if inner == "" {
		return nil
	}
	var result []string
	var current strings.Builder
	inQuotes := false
	for _, ch := range inner {
		switch ch {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				val := strings.TrimSpace(current.String())
				val = strings.Trim(val, `"`)
				if val != "" {
					result = append(result, val)
				}
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if val := strings.TrimSpace(current.String()); val != "" {
		result = append(result, strings.Trim(val, `"`))
	}
	return result
}

func runInfoChecks(ctx context.Context, db *sql.DB, result *PreflightResult) ([]Check, error) {
	var checks []Check

	// Total RTs.
	var rtCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rts").Scan(&rtCount); err != nil {
		return nil, fmt.Errorf("info: count rts: %w", err)
	}

	// Total Households.
	var hhCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM households").Scan(&hhCount); err != nil {
		return nil, fmt.Errorf("info: count households: %w", err)
	}
	result.TotalHouseholds = hhCount

	// Active Households.
	var hhActiveCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM households WHERE is_active = true").Scan(&hhActiveCount); err != nil {
		return nil, fmt.Errorf("info: count active households: %w", err)
	}
	result.ActiveHouseholds = hhActiveCount

	// Inactive Households.
	var hhInactiveCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM households WHERE is_active = false").Scan(&hhInactiveCount); err != nil {
		return nil, fmt.Errorf("info: count inactive households: %w", err)
	}
	result.InactiveHouseholds = hhInactiveCount

	// Total Residents.
	var resCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM residents").Scan(&resCount); err != nil {
		return nil, fmt.Errorf("info: count residents: %w", err)
	}
	result.TotalResidents = resCount

	// Active Residents.
	var resActiveCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM residents WHERE is_active = true").Scan(&resActiveCount); err != nil {
		return nil, fmt.Errorf("info: count active residents: %w", err)
	}
	result.ActiveResidents = resActiveCount

	// Inactive Residents.
	var resInactiveCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM residents WHERE is_active = false").Scan(&resInactiveCount); err != nil {
		return nil, fmt.Errorf("info: count inactive residents: %w", err)
	}
	result.InactiveResidents = resInactiveCount

	// Distinct active house numbers.
	var distHNCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT TRIM(house_number))
		FROM households
		WHERE is_active = true AND house_number IS NOT NULL
	`).Scan(&distHNCount); err != nil {
		return nil, fmt.Errorf("info: count active house numbers: %w", err)
	}
	result.ActiveHouseNumbers = distHNCount

	if rtCount > 0 {
		checks = append(checks, Check{
			Name:     "info_total_rts",
			Severity: "info",
			Message:  fmt.Sprintf("Total RTs: %d", rtCount),
		})
	}
	if hhCount > 0 {
		checks = append(checks, Check{
			Name:     "info_total_households",
			Severity: "info",
			Message:  fmt.Sprintf("Total households: %d (active: %d, inactive: %d)", hhCount, hhActiveCount, hhInactiveCount),
		})
	}
	if resCount > 0 {
		checks = append(checks, Check{
			Name:     "info_total_residents",
			Severity: "info",
			Message:  fmt.Sprintf("Total residents: %d (active: %d, inactive: %d)", resCount, resActiveCount, resInactiveCount),
		})
	}
	if distHNCount > 0 {
		checks = append(checks, Check{
			Name:     "info_active_house_numbers",
			Severity: "info",
			Message:  fmt.Sprintf("Distinct active house numbers: %d", distHNCount),
		})
	}

	return checks, nil
}
