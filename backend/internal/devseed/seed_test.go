package devseed_test

import (
	"context"
	"testing"

	"social-finance/internal/devseed"
	"social-finance/internal/testutil"
)

func TestDevSeed_ExecutionAndIdempotency(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// 1. First run: should populate all canonical seed data
	summary1, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("first seed run failed: %v", err)
	}

	if summary1.RTsCount != 2 {
		t.Errorf("expected 2 RTs, got %d", summary1.RTsCount)
	}
	if summary1.PhysicalHousesCount != 11 {
		t.Errorf("expected 11 physical houses, got %d", summary1.PhysicalHousesCount)
	}
	if summary1.HouseholdsCount != 11 {
		t.Errorf("expected 11 households, got %d", summary1.HouseholdsCount)
	}
	if summary1.HouseholdOccupanciesCount != 11 {
		t.Errorf("expected 11 occupancies, got %d", summary1.HouseholdOccupanciesCount)
	}
	if summary1.ResidentsCount != 10 {
		t.Errorf("expected 10 residents, got %d", summary1.ResidentsCount)
	}
	if summary1.ResidencyPeriodsCount != 12 {
		t.Errorf("expected 12 residency periods, got %d", summary1.ResidencyPeriodsCount)
	}
	if summary1.UsersCount != 8 {
		t.Errorf("expected 8 users, got %d", summary1.UsersCount)
	}
	if summary1.MembershipsCount != 8 {
		t.Errorf("expected 8 memberships, got %d", summary1.MembershipsCount)
	}

	if summary1.PositionUsersCount != 16 {
		t.Errorf("expected 16 position users, got %d", summary1.PositionUsersCount)
	}
	if summary1.PositionMembershipsCount != 16 {
		t.Errorf("expected 16 position memberships, got %d", summary1.PositionMembershipsCount)
	}
	if summary1.PermissionTypesCount != 12 {
		t.Errorf("expected 12 permission types, got %d", summary1.PermissionTypesCount)
	}
	if summary1.PositionPermissionsCount != 43 {
		t.Errorf("expected 43 position permissions, got %d", summary1.PositionPermissionsCount)
	}
	if summary1.BillsCount != 0 {
		t.Errorf("expected 0 bills, got %d", summary1.BillsCount)
	}
	if summary1.PaymentsCount != 0 {
		t.Errorf("expected 0 payments, got %d", summary1.PaymentsCount)
	}

	// 2. Second run: idempotency test - should succeed without error or duplicating rows
	summary2, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("second seed run failed: %v", err)
	}

	// Verify row counts in the database did not double
	var count int
	if err := pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rts WHERE id IN ('8dfa1e36-6727-45be-8fea-24d958efc155', 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b')").Scan(&count); err != nil {
		t.Fatalf("query rts count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 RTs from canonical set, found %d", count)
	}

	if err := pool.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM physical_houses WHERE id::text LIKE 'bbbb%'").Scan(&count); err != nil {
		t.Fatalf("query physical_houses count: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 bbbb physical houses, found %d", count)
	}

	if summary2.UsersCount != summary1.UsersCount {
		t.Errorf("expected idempotent user count %d, got %d", summary1.UsersCount, summary2.UsersCount)
	}
	if summary2.PositionUsersCount != summary1.PositionUsersCount {
		t.Errorf("expected idempotent position user count %d, got %d", summary1.PositionUsersCount, summary2.PositionUsersCount)
	}
}

func TestDevSeed_SafetyGuard_RejectsProduction(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	_, err := devseed.Run(ctx, pool, "production")
	if err == nil {
		t.Fatalf("expected devseed to fail for APP_ENV=production, but it succeeded")
	}
	if err := pool.Raw().PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestDevSeed_NeverCreatesSuperAdmin(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	if _, err := devseed.Run(ctx, pool, "test"); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	var superAdminCount int
	q := `SELECT COUNT(*) FROM users WHERE system_role = 'super_admin' AND (email LIKE 'rt%.rw16.%@example.com' OR email LIKE '%@rt%.rw16.local')`
	if err := pool.Raw().QueryRowContext(ctx, q).Scan(&superAdminCount); err != nil {
		t.Fatalf("query super_admin count: %v", err)
	}
	if superAdminCount != 0 {
		t.Errorf("devseed must NEVER create super_admin, found %d", superAdminCount)
	}
}

// TestDevSeed_RestoredRT002Residents verifies that the three RT 002 seed residents
// (Aqueen, Azis, Namah) are present with correct data and valid household/residency
// periods after seed execution.
func TestDevSeed_RestoredRT002Residents(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Run seed first
	_, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	// Helper to count rows matching a query
	countResidents := func(query string, args ...any) int {
		var count int
		if err := pool.Raw().QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
			t.Fatalf("query: %v", err)
		}
		return count
	}

	// 1. All three residents must exist exactly once
	expectedResidents := []struct {
		ID       string
		FullName string
		Phone    string
		NIK      string
		Email    *string
	}{
		{
			ID:       "aaaa0002-0001-4f66-8000-000000000001",
			FullName: "Aqueen Alsava Khaylila",
			Phone:    "+6281200000003",
			NIK:      "3175000000000003",
			Email:    strPtr("aqueen.alsava@local.com"),
		},
		{
			ID:       "aaaa0002-0002-4f66-8000-000000000002",
			FullName: "Muhamad Azis",
			Phone:    "+6281200000004",
			NIK:      "3175000000000004",
			Email:    strPtr("muhamad.azis@local.com"),
		},
		{
			ID:       "aaaa0002-0003-4f66-8000-000000000003",
			FullName: "Namah",
			Phone:    "+6281200000005",
			NIK:      "3175000000000005",
			Email:    nil,
		},
	}

	rt002ID := "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"

	for _, exp := range expectedResidents {
		var actualID, actualName, actualPhone, actualNIK string
		var actualEmail *string
		var actualRTID, actualUserID *string
		var actualIsActive bool

		q := `SELECT id, full_name, phone, nik, email, rt_id, user_id, is_active FROM residents WHERE id = $1`
		row := pool.Raw().QueryRowContext(ctx, q, exp.ID)
		if err := row.Scan(&actualID, &actualName, &actualPhone, &actualNIK, &actualEmail, &actualRTID, &actualUserID, &actualIsActive); err != nil {
			t.Errorf("resident %s not found: %v", exp.FullName, err)
			continue
		}

		if actualName != exp.FullName {
			t.Errorf("resident %s: expected name %q, got %q", exp.ID, exp.FullName, actualName)
		}
		if actualPhone != exp.Phone {
			t.Errorf("resident %s: expected phone %q, got %q", exp.ID, exp.Phone, actualPhone)
		}
		if actualNIK != exp.NIK {
			t.Errorf("resident %s: expected NIK %q, got %q", exp.ID, exp.NIK, actualNIK)
		}
		if exp.Email == nil {
			if actualEmail != nil && *actualEmail != "" {
				t.Errorf("resident %s: expected NULL/empty email, got %q", exp.ID, *actualEmail)
			}
		} else {
			if actualEmail == nil || *actualEmail != *exp.Email {
				t.Errorf("resident %s: expected email %q, got %v", exp.ID, *exp.Email, actualEmail)
			}
		}
		if actualRTID == nil || *actualRTID != rt002ID {
			t.Errorf("resident %s: expected RT %s, got %v", exp.ID, rt002ID, actualRTID)
		}
		if actualUserID != nil {
			t.Errorf("resident %s: expected user_id NULL (seed does not link), got %s", exp.ID, *actualUserID)
		}
		if !actualIsActive {
			t.Errorf("resident %s: expected is_active = true, got false", exp.ID)
		}
	}

	// 2. No duplicate residents with these IDs
	for _, exp := range expectedResidents {
		count := countResidents(`SELECT COUNT(*) FROM residents WHERE id = $1`, exp.ID)
		if count != 1 {
			t.Errorf("expected exactly 1 resident with ID %s, found %d", exp.ID, count)
		}
	}

	// 3. Each resident must have exactly one active residency period that resolves
	//    to a valid household in RT 002 with a non-null house_number.
	for _, exp := range expectedResidents {
		var residencyCount int
		row := pool.Raw().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM residency_periods rp
			 JOIN residents r ON r.id = rp.resident_id
			 JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
			 JOIN households h ON h.id = ho.household_id
			 JOIN physical_houses ph ON ph.id = ho.physical_house_id
			 WHERE r.id = $1 AND rp.end_date IS NULL AND h.rt_id = $2 AND ph.house_number IS NOT NULL`,
			exp.ID, rt002ID,
		)
		if err := row.Scan(&residencyCount); err != nil {
			t.Fatalf("query residency for %s: %v", exp.FullName, err)
		}
		if residencyCount != 1 {
			t.Errorf("resident %s: expected exactly 1 active residency in RT 002 with house_number, found %d", exp.FullName, residencyCount)
		}
	}

	// 4. No user account created for Azis (not in CanonicalUsers)
	azisUserCount := countResidents(
		`SELECT COUNT(*) FROM users WHERE email = 'muhamad.azis@local.com'`,
	)
	if azisUserCount != 0 {
		t.Errorf("expected 0 users with email muhamad.azis@local.com, found %d", azisUserCount)
	}

	// 5. Verify idempotency: running seed again preserves all three residents
	_, err = devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("second seed run failed: %v", err)
	}

	for _, exp := range expectedResidents {
		count := countResidents(`SELECT COUNT(*) FROM residents WHERE id = $1`, exp.ID)
		if count != 1 {
			t.Errorf("after second seed: expected 1 resident with ID %s, found %d", exp.ID, count)
		}

		// Verify residency periods are also idempotent
		var residencyCount int
		row := pool.Raw().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM residency_periods rp
			 JOIN residents r ON r.id = rp.resident_id
			 WHERE r.id = $1 AND rp.end_date IS NULL`,
			exp.ID,
		)
		if err := row.Scan(&residencyCount); err != nil {
			t.Fatalf("query residency after idempotency for %s: %v", exp.FullName, err)
		}
		if residencyCount != 1 {
			t.Errorf("after second seed: resident %s expected 1 active residency, found %d", exp.FullName, residencyCount)
		}
	}
}

// strPtr returns a pointer to the given string value.
// Used in tests since strPtr from devseed package is unexported.
func strPtr(s string) *string {
	return &s
}

// TestDevSeed_HouseholdIsolation verifies that updating a house number for one
// household does NOT affect other households. This regression test ensures that
// Namah, Azis, and Aqueen each have independent household chains.
func TestDevSeed_HouseholdIsolation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	// Run seed to establish clean state
	_, err := devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	// Helper to get resident's house number via their active residency
	getHouseNumber := func(residentID string) (string, error) {
		var hn string
		q := `
			SELECT ph.house_number
			FROM residency_periods rp
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
			JOIN physical_houses ph ON ph.id = ho.physical_house_id
			WHERE rp.resident_id = $1 AND rp.end_date IS NULL
			LIMIT 1
		`
		if err := pool.Raw().QueryRowContext(ctx, q, residentID).Scan(&hn); err != nil {
			return "", err
		}
		return hn, nil
	}

	// Helper to get resident's household ID via their active residency
	getHouseholdID := func(residentID string) (string, error) {
		var hid string
		q := `
			SELECT ho.household_id
			FROM residency_periods rp
			JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
			WHERE rp.resident_id = $1 AND rp.end_date IS NULL
			LIMIT 1
		`
		if err := pool.Raw().QueryRowContext(ctx, q, residentID).Scan(&hid); err != nil {
			return "", err
		}
		return hid, nil
	}

	// Verify each resident has a valid active residency
	for _, id := range []string{
		"aaaa0002-0001-4f66-8000-000000000001", // Aqueen
		"aaaa0002-0002-4f66-8000-000000000002", // Azis
		"aaaa0002-0003-4f66-8000-000000000003", // Namah
	} {
		hn, err := getHouseNumber(id)
		if err != nil {
			t.Fatalf("resident %s: failed to get house number: %v", id, err)
		}
		if hn == "" {
			t.Fatalf("resident %s: no active residency found", id)
		}
	}

	// 1. Verify each resident has a DIFFERENT house number initially
	hnAqueen, _ := getHouseNumber("aaaa0002-0001-4f66-8000-000000000001")
	hnAzis, _ := getHouseNumber("aaaa0002-0002-4f66-8000-000000000002")
	hnNamah, _ := getHouseNumber("aaaa0002-0003-4f66-8000-000000000003")

	t.Logf("Initial: Aqueen=%s, Azis=%s, Namah=%s", hnAqueen, hnAzis, hnNamah)

	// All three must have unique house numbers
	if hnAqueen == hnAzis {
		t.Errorf("Aqueen and Azis share house number %s — they should be independent", hnAqueen)
	}
	if hnAqueen == hnNamah {
		t.Errorf("Aqueen and Namah share house number %s — they should be independent", hnAqueen)
	}
	if hnAzis == hnNamah {
		t.Errorf("Azis and Namah share house number %s — they should be independent", hnAzis)
	}

	// 2. Verify each resident has a DIFFERENT household ID
	hhAqueen, _ := getHouseholdID("aaaa0002-0001-4f66-8000-000000000001")
	hhAzis, _ := getHouseholdID("aaaa0002-0002-4f66-8000-000000000002")
	hhNamah, _ := getHouseholdID("aaaa0002-0003-4f66-8000-000000000003")

	if hhAqueen == hhAzis {
		t.Errorf("Aqueen and Azis share household ID — they should be independent")
	}
	if hhAqueen == hhNamah {
		t.Errorf("Aqueen and Namah share household ID — they should be independent")
	}
	if hhAzis == hhNamah {
		t.Errorf("Azis and Namah share household ID — they should be independent")
	}

	// 3. Update Namah's house number through the correct API path
	// This updates the physical_houses.house_number for Namah's physical house
	updateQ := `
		UPDATE physical_houses ph
		SET house_number = 'U12/99'
		FROM residency_periods rp
		JOIN household_occupancies ho ON ho.id = rp.household_occupancy_id
		WHERE rp.resident_id = 'aaaa0002-0003-4f66-8000-000000000003'
		  AND ph.id = ho.physical_house_id
		  AND rp.end_date IS NULL
	`
	result, err := pool.Raw().ExecContext(ctx, updateQ)
	if err != nil {
		t.Fatalf("update Namah's house number: %v", err)
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		t.Errorf("expected 1 row updated, got %d", rows)
	}

	// 4. Verify Namah's house number changed
	newHnNamah, err := getHouseNumber("aaaa0002-0003-4f66-8000-000000000003")
	if err != nil {
		t.Fatalf("get Namah's house number after update: %v", err)
	}
	if newHnNamah != "U12/99" {
		t.Errorf("expected Namah's house number to be U12/99, got %s", newHnNamah)
	}

	// 5. Verify Azis and Aqueen are UNCHANGED
	newHnAzis, err := getHouseNumber("aaaa0002-0002-4f66-8000-000000000002")
	if err != nil {
		t.Fatalf("get Azis's house number after update: %v", err)
	}
	if newHnAzis == "U12/99" {
		t.Errorf("Azis's house number was changed to U12/99 — household isolation broken!")
	}
	if newHnAzis == hnAzis {
		t.Logf("Azis house number unchanged: %s (PASS)", newHnAzis)
	}

	newHnAqueen, err := getHouseNumber("aaaa0002-0001-4f66-8000-000000000001")
	if err != nil {
		t.Fatalf("get Aqueen's house number after update: %v", err)
	}
	if newHnAqueen == "U12/99" {
		t.Errorf("Aqueen's house number was changed to U12/99 — household isolation broken!")
	}
	if newHnAqueen == hnAqueen {
		t.Logf("Aqueen house number unchanged: %s (PASS)", newHnAqueen)
	}

	// 6. Verify Azis and Aqueen are not only unchanged but also still independent
	if newHnAzis == newHnAqueen {
		t.Errorf("After Namah update: Azis and Aqueen share house number %s", newHnAzis)
	}

	// 7. Verify household IDs remain unchanged
	newHhNamah, _ := getHouseholdID("aaaa0002-0003-4f66-8000-000000000003")
	newHhAzis, _ := getHouseholdID("aaaa0002-0002-4f66-8000-000000000002")
	newHhAqueen, _ := getHouseholdID("aaaa0002-0001-4f66-8000-000000000001")

	if newHhNamah != hhNamah {
		t.Errorf("Namah's household ID changed from %s to %s", hhNamah, newHhNamah)
	}
	if newHhAzis != hhAzis {
		t.Errorf("Azis's household ID changed from %s to %s", hhAzis, newHhAzis)
	}
	if newHhAqueen != hhAqueen {
		t.Errorf("Aqueen's household ID changed from %s to %s", hhAqueen, newHhAqueen)
	}

	// 8. Test idempotency: running seed again preserves independence
	_, err = devseed.Run(ctx, pool, "test")
	if err != nil {
		t.Fatalf("second seed run failed: %v", err)
	}

	hnAqueen2, _ := getHouseNumber("aaaa0002-0001-4f66-8000-000000000001")
	hnAzis2, _ := getHouseNumber("aaaa0002-0002-4f66-8000-000000000002")
	hnNamah2, _ := getHouseNumber("aaaa0002-0003-4f66-8000-000000000003")

	t.Logf("After reseed: Aqueen=%s, Azis=%s, Namah=%s", hnAqueen2, hnAzis2, hnNamah2)

	if hnAqueen2 == hnAzis2 {
		t.Errorf("After reseed: Aqueen and Azis share house number")
	}
	if hnAqueen2 == hnNamah2 {
		t.Errorf("After reseed: Aqueen and Namah share house number")
	}
	if hnAzis2 == hnNamah2 {
		t.Errorf("After reseed: Azis and Namah share house number")
	}
}
