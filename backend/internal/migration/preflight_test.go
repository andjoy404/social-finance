package migration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"social-finance/internal/testutil"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testutil.GetTestSQLDB(t)

	var hasV7Col bool
	_ = db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='households' AND column_name='occupancy_status')").Scan(&hasV7Col)
	if !hasV7Col {
		t.Skip("skipping preflight test: schema is on v8+ (preflight applies to v7 legacy schema)")
	}

	return db
}

func uniqueID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// createTestRT creates a test RT row and returns its ID.
func createTestRT(db *sql.DB, t *testing.T, name string) string {
	t.Helper()
	var id string

	// Derive unique rw/rt numbers to avoid uniq_rts_active conflicts.
	rtVal := uniqueID()[:8]

	// rw is int, rt is text — cannot reuse same $3 for both.
	err := db.QueryRow(
		"INSERT INTO rts (id, name, rw, rt, address, is_active) VALUES ($1, $2, $3, $4, '', true) RETURNING id",
		uniqueID(), name, int64(len(rtVal)), rtVal,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test RT %s: %v", name, err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM rts WHERE id = $1", id)
	})
	return id
}

// createTestHousehold creates a household.
func createTestHousehold(db *sql.DB, t *testing.T, rtID, houseNumber string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO households (id, rt_id, head_name, is_active, occupancy_status, house_number)
		 VALUES ($1, $2, $3, true, $4, $5) RETURNING id`,
		uniqueID(), rtID, "Test Head", "OWNER", houseNumber,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test household: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM households WHERE id = $1", id)
	})
	return id
}

// createTestResident creates a resident.
func createTestResident(db *sql.DB, t *testing.T, rtID, householdID, fullName string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO residents (id, rt_id, household_id, full_name, is_active)
		 VALUES ($1, $2, $3, $4, true) RETURNING id`,
		uniqueID(), rtID, householdID, fullName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test resident: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM residents WHERE id = $1", id)
	})
	return id
}

// createTestHouseholdWithHN creates a household with configurable house_number.
func createTestHouseholdWithHN(db *sql.DB, t *testing.T, rtID string, hn sql.NullString) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO households (id, rt_id, head_name, is_active, occupancy_status, house_number)
		 VALUES ($1, $2, 'Test Head', true, 'OWNER', $3) RETURNING id`,
		uniqueID(), rtID, hn,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test household with house_number: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM households WHERE id = $1", id)
	})
	return id
}

// createTestHouseholdWithStatus creates a household with configurable occupancy_status.
func createTestHouseholdWithStatus(db *sql.DB, t *testing.T, rtID string, os sql.NullString) string {
	t.Helper()
	id := uniqueID()
	// Use a unique, short house_number derived from the UUID to avoid cross-test collisions.
	hn := id[len(id)-8:]
	err := db.QueryRow(
		`INSERT INTO households (id, rt_id, head_name, is_active, occupancy_status, house_number)
		 VALUES ($1, $2, 'Test Head', true, $3, $4) RETURNING id`,
		id, rtID, os, hn,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test household with occupancy_status: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM households WHERE id = $1", id)
	})
	return id
}

// createTestResidentWithHousehold creates a resident referencing a specific household.
func createTestResidentWithHousehold(db *sql.DB, t *testing.T, rtID, householdID, fullName string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO residents (id, rt_id, household_id, full_name, is_active)
		 VALUES ($1, $2, $3, $4, true) RETURNING id`,
		uniqueID(), rtID, householdID, fullName,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test resident with household: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM residents WHERE id = $1", id)
	})
	return id
}

// assertCheckExists verifies a check with the given name and severity exists.
func assertCheckExists(t *testing.T, result *PreflightResult, name, severity string) {
	t.Helper()
	for _, c := range result.Checks {
		if c.Name == name && c.Severity == severity {
			return
		}
	}
	t.Fatalf("expected check %q severity=%q not found in result", name, severity)
}

func assertBlockingCount(t *testing.T, result *PreflightResult, expected int) {
	t.Helper()
	if result.BlockingCount != expected {
		t.Fatalf("expected %d blocking conflicts, got %d", expected, result.BlockingCount)
	}
}

// assertFixtureHasNoConflict verifies that no blocking conflict references the given household/resident IDs.
func assertFixtureHasNoConflict(t *testing.T, result *PreflightResult, householdIDs, residentIDs map[string]bool) {
	t.Helper()
	for _, c := range result.Checks {
		if c.Severity != "blocking" {
			continue
		}
		if c.HouseholdID != nil && householdIDs[*c.HouseholdID] {
			t.Fatalf("fixture household %s produced unexpected blocking conflict: %s: %s", *c.HouseholdID, c.Name, c.Message)
		}
		if c.ResidentID != nil && residentIDs[*c.ResidentID] {
			t.Fatalf("fixture resident %s produced unexpected blocking conflict: %s: %s", *c.ResidentID, c.Name, c.Message)
		}
	}
}

// assertFixtureHasConflict verifies that at least one blocking conflict references the given household ID.
func assertFixtureHasConflict(t *testing.T, result *PreflightResult, checkName string, householdID string) {
	t.Helper()
	found := false
	for _, c := range result.Checks {
		if c.Name == checkName && c.Severity == "blocking" && c.HouseholdID != nil && *c.HouseholdID == householdID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fixture household %s in %s conflict", householdID, checkName)
	}
}

// assertFixtureHasResidentConflict verifies that at least one blocking conflict references the given resident ID.
func assertFixtureHasResidentConflict(t *testing.T, result *PreflightResult, checkName string, residentID string) {
	t.Helper()
	found := false
	for _, c := range result.Checks {
		if c.Name == checkName && c.Severity == "blocking" && c.ResidentID != nil && *c.ResidentID == residentID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fixture resident %s in %s conflict", residentID, checkName)
	}
}

// TestPreflightCleanDataset creates a minimal clean fixture and verifies
// no blocking conflicts are reported for the fixture itself.
func TestPreflightCleanDataset(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-clean-dataset-rt")
	hhID := createTestHousehold(db, t, rtA, "X-01")
	resID := createTestResident(db, t, rtA, hhID, "Clean Resident")

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertFixtureHasNoConflict(t, result, map[string]bool{hhID: true}, map[string]bool{resID: true})
}

// TestPreflightMissingHouseNumber verifies that an active household with
// a NULL or blank house_number is detected.
func TestPreflightMissingHouseNumber(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-missing-hn-rt")
	hhNil := createTestHouseholdWithHN(db, t, rtA, sql.NullString{Valid: false})
	t.Logf("household ID: %s", hhNil)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertCheckExists(t, result, "missing_active_house_number", "blocking")
	assertFixtureHasConflict(t, result, "missing_active_house_number", hhNil)
}

// TestPreflightBlankHouseNumber verifies that an active household with
// a whitespace-only house_number is detected.
func TestPreflightBlankHouseNumber(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-blank-hn-rt")
	hhBlank := createTestHouseholdWithHN(db, t, rtA, sql.NullString{Valid: true, String: "   "})
	t.Logf("household ID: %s", hhBlank)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertCheckExists(t, result, "missing_active_house_number", "blocking")
	assertFixtureHasConflict(t, result, "missing_active_house_number", hhBlank)
}

// TestPreflightDuplicateHouseNumber verifies that two active households
// in the same RT with house_numbers that collide after TRIM are detected.
func TestPreflightDuplicateHouseNumber(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-dup-hn-rt")
	hh1 := createTestHouseholdWithHN(db, t, rtA, sql.NullString{Valid: true, String: "U12/14"})
	hh2 := createTestHouseholdWithHN(db, t, rtA, sql.NullString{Valid: true, String: "  U12/14  "})
	t.Logf("household IDs: %s, %s", hh1, hh2)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertCheckExists(t, result, "duplicate_active_house_number", "blocking")
	assertFixtureHasConflict(t, result, "duplicate_active_house_number", hh1)
	assertFixtureHasConflict(t, result, "duplicate_active_house_number", hh2)
}

// TestPreflightValidHouseNumbers verifies that distinct house numbers
// in the same RT produce no duplicate conflict.
func TestPreflightValidHouseNumbers(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-valid-hn-rt")
	hh1 := createTestHouseholdWithHN(db, t, rtA, sql.NullString{Valid: true, String: "A-01"})
	hh2 := createTestHouseholdWithHN(db, t, rtA, sql.NullString{Valid: true, String: "A-02"})

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertFixtureHasNoConflict(t, result, map[string]bool{hh1: true, hh2: true}, map[string]bool{})
}

// TestPreflightNullOccupancyStatus verifies that an active household
// with NULL occupancy_status is detected as invalid.
func TestPreflightNullOccupancyStatus(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-null-os-rt")
	hh := createTestHouseholdWithStatus(db, t, rtA, sql.NullString{Valid: false})
	t.Logf("household ID: %s", hh)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertCheckExists(t, result, "invalid_active_occupancy_status", "blocking")
	assertFixtureHasConflict(t, result, "invalid_active_occupancy_status", hh)

	// Verify the specific household reports <NULL> as current value.
	for _, c := range result.Checks {
		if c.Name == "invalid_active_occupancy_status" && c.HouseholdID != nil && *c.HouseholdID == hh {
			if c.CurrentVal == nil || *c.CurrentVal != "<NULL>" {
				t.Fatalf("expected current_value=<NULL>, got %v", c.CurrentVal)
			}
			return
		}
	}
	t.Fatalf("expected fixture household %s in invalid_active_occupancy_status check", hh)
}

// TestPreflightValidOccupancyStatus verifies that OWNER and TENANT
// do not produce conflicts.
func TestPreflightValidOccupancyStatus(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-valid-os-rt")
	hh1 := createTestHouseholdWithStatus(db, t, rtA, sql.NullString{Valid: true, String: "OWNER"})
	hh2 := createTestHouseholdWithStatus(db, t, rtA, sql.NullString{Valid: true, String: "TENANT"})

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertFixtureHasNoConflict(t, result, map[string]bool{hh1: true, hh2: true}, map[string]bool{})
}

// TestPreflightCrossRTResidentHousehold verifies that a resident whose
// rt_id differs from the rt_id of the household they reference triggers
// the cross_rt_resident_household check.
func TestPreflightCrossRTResidentHousehold(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-cxrta")
	rtB := createTestRT(db, t, "-cxrtb")
	hhID := createTestHouseholdWithStatus(db, t, rtA, sql.NullString{Valid: true, String: "OWNER"})

	resID := createTestResidentWithHousehold(db, t, rtB, hhID, "CrossRT Resident")
	t.Logf("resident ID: %s, household ID: %s, resident RT: %s, household RT: %s", resID, hhID, rtB, rtA)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertCheckExists(t, result, "cross_rt_resident_household", "blocking")
	assertFixtureHasResidentConflict(t, result, "cross_rt_resident_household", resID)

	// Verify the specific resident's cross_RT reference is correct.
	for _, c := range result.Checks {
		if c.Name == "cross_rt_resident_household" && c.ResidentID != nil && *c.ResidentID == resID {
			if c.CrossRT == nil || *c.CrossRT != hhID {
				t.Fatalf("expected cross_rt=%s, got %v", hhID, c.CrossRT)
			}
			return
		}
	}
	t.Fatalf("expected fixture resident %s in cross_rt_resident_household check", resID)
}

// TestPreflightActiveResidentInactiveHousehold verifies that an active
// resident assigned to an inactive household triggers the blocker.
func TestPreflightActiveResidentInactiveHousehold(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-arihib")

	// Create an inactive household.
	var inactiveHH string
	err := db.QueryRow(
		`INSERT INTO households (id, rt_id, head_name, is_active, occupancy_status, house_number)
		 VALUES ($1, $2, 'Inactive Head', false, 'OWNER', 'INN-01') RETURNING id`,
		uniqueID(), rtA,
	).Scan(&inactiveHH)
	if err != nil {
		t.Fatalf("create inactive household: %v", err)
	}
	t.Logf("inactive household ID: %s", inactiveHH)

	// Create an active resident assigned to the inactive household.
	resID := createTestResidentWithHousehold(db, t, rtA, inactiveHH, "Active Resident on Inactive HH")
	t.Logf("resident ID: %s", resID)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertCheckExists(t, result, "active_resident_inactive_household", "blocking")

	// Verify the specific resident is flagged.
	found := false
	for _, c := range result.Checks {
		if c.Name == "active_resident_inactive_household" && c.ResidentID != nil && *c.ResidentID == resID {
			found = true
			// Verify diagnostic fields are populated.
			if c.ResidentName == nil || *c.ResidentName != "Active Resident on Inactive HH" {
				t.Fatalf("expected resident_name=%q, got %v", "Active Resident on Inactive HH", c.ResidentName)
			}
			if c.AssociatedHHUUID == nil || *c.AssociatedHHUUID != inactiveHH {
				t.Fatalf("expected associated_hh_uuid=%s, got %v", inactiveHH, c.AssociatedHHUUID)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected fixture resident %s in active_resident_inactive_household check", resID)
	}
}

// TestPreflightActiveResidentNoInactiveHouseholdBlock verifies that
// active residents on active households do NOT trigger blocker #8.
func TestPreflightActiveResidentNoInactiveHouseholdBlock(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-arnihh")

	// Create an active household.
	hhID := createTestHouseholdWithStatus(db, t, rtA, sql.NullString{Valid: true, String: "OWNER"})

	// Create an active resident assigned to the active household.
	resID := createTestResidentWithHousehold(db, t, rtA, hhID, "Active Resident on Active HH")

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	// Ensure the fixture did NOT trigger blocker #8.
	for _, c := range result.Checks {
		if c.Name == "active_resident_inactive_household" && c.ResidentID != nil && *c.ResidentID == resID {
			t.Fatalf("fixture resident %s should NOT trigger active_resident_inactive_household: %s: %s",
				resID, c.Name, c.Message)
		}
	}
}

// TestPreflightInactiveResidentInactiveHousehold verifies that inactive
// residents on inactive households do NOT trigger blocker #8.
func TestPreflightInactiveResidentInactiveHousehold(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-irihh")

	// Create an inactive household.
	var inactiveHH string
	err := db.QueryRow(
		`INSERT INTO households (id, rt_id, head_name, is_active, occupancy_status, house_number)
		 VALUES ($1, $2, 'Inactive Head', false, 'OWNER', 'INRH-01') RETURNING id`,
		uniqueID(), rtA,
	).Scan(&inactiveHH)
	if err != nil {
		t.Fatalf("create inactive household: %v", err)
	}

	// Create an INACTIVE resident assigned to the inactive household.
	var resID string
	err = db.QueryRow(
		`INSERT INTO residents (id, rt_id, household_id, full_name, is_active)
		 VALUES ($1, $2, $3, 'Inactive Resident', false) RETURNING id`,
		uniqueID(), rtA, inactiveHH,
	).Scan(&resID)
	if err != nil {
		t.Fatalf("create inactive resident: %v", err)
	}
	t.Logf("resident ID: %s, household ID: %s", resID, inactiveHH)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	// Ensure the fixture did NOT trigger blocker #8.
	for _, c := range result.Checks {
		if c.Name == "active_resident_inactive_household" && c.ResidentID != nil && *c.ResidentID == resID {
			t.Fatalf("inactive fixture resident %s should NOT trigger active_resident_inactive_household", resID)
		}
	}
}

// TestPreflightInactiveResidentActiveHousehold verifies that inactive
// residents on active households do NOT trigger blocker #8.
func TestPreflightInactiveResidentActiveHousehold(t *testing.T) {
	db := testDB(t)
	rtA := createTestRT(db, t, "-iriah")

	// Create an active household.
	hhID := createTestHouseholdWithStatus(db, t, rtA, sql.NullString{Valid: true, String: "OWNER"})

	// Create an INACTIVE resident assigned to the active household.
	var resID string
	err := db.QueryRow(
		`INSERT INTO residents (id, rt_id, household_id, full_name, is_active)
		 VALUES ($1, $2, $3, 'Inactive Resident on Active HH', false) RETURNING id`,
		uniqueID(), rtA, hhID,
	).Scan(&resID)
	if err != nil {
		t.Fatalf("create inactive resident on active household: %v", err)
	}
	t.Logf("resident ID: %s, household ID: %s", resID, hhID)

	result, err := Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	// Ensure the fixture did NOT trigger blocker #8.
	for _, c := range result.Checks {
		if c.Name == "active_resident_inactive_household" && c.ResidentID != nil && *c.ResidentID == resID {
			t.Fatalf("fixture resident %s should NOT trigger active_resident_inactive_household: %s: %s",
				resID, c.Name, c.Message)
		}
	}
}
