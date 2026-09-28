package devseed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

const (
	// DefaultSeedPassword is the single known password for all seeded mock users.
	DefaultSeedPassword = "TestUser123!"
)

// SeedSummary contains counts of all entities inserted/upserted by the seed operation.
type SeedSummary struct {
	RTsCount                  int
	PhysicalHousesCount       int
	HouseholdsCount           int
	HouseholdOccupanciesCount int
	ResidentsCount            int
	ResidencyPeriodsCount     int
	UsersCount                int
	MembershipsCount          int
	PositionUsersCount        int
	PositionMembershipsCount  int
	PositionResidentsCount    int
	PermissionTypesCount      int
	PositionPermissionsCount  int
}

// Run executes the development seed against the database pool.
// It verifies that the environment is strictly 'development' or 'test'.
func Run(ctx context.Context, pool *database.Pool, appEnv string) (*SeedSummary, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is required")
	}

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback()

	summary, err := RunTx(ctx, tx, appEnv)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit seed transaction: %w", err)
	}

	slog.Info("devseed completed successfully",
		"rts", summary.RTsCount,
		"physical_houses", summary.PhysicalHousesCount,
		"households", summary.HouseholdsCount,
		"occupancies", summary.HouseholdOccupanciesCount,
		"residents", summary.ResidentsCount,
		"residency_periods", summary.ResidencyPeriodsCount,
		"users", summary.UsersCount,
		"memberships", summary.MembershipsCount,
		"position_users", summary.PositionUsersCount,
		"position_memberships", summary.PositionMembershipsCount,
		"position_residents", summary.PositionResidentsCount,
		"permission_types", summary.PermissionTypesCount,
		"position_permissions", summary.PositionPermissionsCount,
	)

	return summary, nil
}

// RunTx executes the development seed within an existing transaction.
func RunTx(ctx context.Context, tx *sql.Tx, appEnv string) (*SeedSummary, error) {
	env := strings.ToLower(strings.TrimSpace(appEnv))
	if env != "development" && env != "test" {
		return nil, fmt.Errorf("HARD SAFETY GUARD BLOCKED: devseed can only run in development or test environment, current env is %q", appEnv)
	}

	summary := &SeedSummary{}

	// 1. RTs
	for _, rt := range CanonicalRTs {
		q := `
			INSERT INTO rts (id, name, rw, rt, address, head_name, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				rw = EXCLUDED.rw,
				rt = EXCLUDED.rt,
				address = EXCLUDED.address,
				head_name = EXCLUDED.head_name,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, rt.ID, rt.Name, rt.RW, rt.RT, rt.Address, rt.HeadName, rt.IsActive); err != nil {
			return nil, fmt.Errorf("upsert rt %s: %w", rt.ID, err)
		}
		summary.RTsCount++
	}

	// 2. Physical Houses
	for _, ph := range CanonicalPhysicalHouses {
		q := `
			INSERT INTO physical_houses (id, rt_id, house_number, address, is_active)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				rt_id = EXCLUDED.rt_id,
				house_number = EXCLUDED.house_number,
				address = EXCLUDED.address,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, ph.ID, ph.RTID, ph.HouseNumber, ph.Address, ph.IsActive); err != nil {
			return nil, fmt.Errorf("upsert physical_house %s: %w", ph.ID, err)
		}
		summary.PhysicalHousesCount++
	}

	// 3. Households
	for _, hh := range CanonicalHouseholds {
		q := `
			INSERT INTO households (id, rt_id, head_name, is_active)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET
				rt_id = EXCLUDED.rt_id,
				head_name = EXCLUDED.head_name,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, hh.ID, hh.RTID, hh.HeadName, hh.IsActive); err != nil {
			return nil, fmt.Errorf("upsert household %s: %w", hh.ID, err)
		}
		summary.HouseholdsCount++
	}

	// 4. Household Occupancies
	for _, ho := range CanonicalOccupancies {
		q := `
			INSERT INTO household_occupancies (id, physical_house_id, household_id, occupancy_status, start_date, end_date)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				physical_house_id = EXCLUDED.physical_house_id,
				household_id = EXCLUDED.household_id,
				occupancy_status = EXCLUDED.occupancy_status,
				start_date = EXCLUDED.start_date,
				end_date = EXCLUDED.end_date,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, ho.ID, ho.PhysicalHouseID, ho.HouseholdID, ho.OccupancyStatus, ho.StartDate, ho.EndDate); err != nil {
			return nil, fmt.Errorf("upsert occupancy %s: %w", ho.ID, err)
		}
		summary.HouseholdOccupanciesCount++
	}

	// 5. Residents
	for _, res := range CanonicalResidents {
		q := `
			INSERT INTO residents (id, rt_id, full_name, phone, is_active, nik, email)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				rt_id = EXCLUDED.rt_id,
				full_name = EXCLUDED.full_name,
				phone = EXCLUDED.phone,
				is_active = EXCLUDED.is_active,
				nik = EXCLUDED.nik,
				email = EXCLUDED.email,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, res.ID, res.RTID, res.FullName, res.Phone, res.IsActive, res.NIK, res.Email); err != nil {
			return nil, fmt.Errorf("upsert resident %s: %w", res.ID, err)
		}
		summary.ResidentsCount++
	}

	// 6. Residency Periods
	for _, rp := range CanonicalResidencyPeriods {
		q := `
			INSERT INTO residency_periods (id, resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				resident_id = EXCLUDED.resident_id,
				household_occupancy_id = EXCLUDED.household_occupancy_id,
				relationship_to_head = EXCLUDED.relationship_to_head,
				start_date = EXCLUDED.start_date,
				end_date = EXCLUDED.end_date,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, rp.ID, rp.ResidentID, rp.HouseholdOccupancyID, rp.RelationshipToHead, rp.StartDate, rp.EndDate); err != nil {
			return nil, fmt.Errorf("upsert residency_period %s: %w", rp.ID, err)
		}
		summary.ResidencyPeriodsCount++
	}

	// 7. Users and Memberships
	passwordHash, err := auth.Hash(DefaultSeedPassword)
	if err != nil {
		return nil, fmt.Errorf("hash seed password: %w", err)
	}

	for _, u := range CanonicalUsers {
		// HARD CONSTRAINT: Never allow super_admin to be seeded automatically
		// Super Admin must be created solely via --bootstrap CLI
		userQ := `
			INSERT INTO users (id, email, phone, password_hash, full_name, is_active, system_role)
			VALUES ($1, $2, $3, $4, $5, true, NULL)
			ON CONFLICT ((lower(TRIM(BOTH FROM email)))) DO UPDATE SET
				phone = EXCLUDED.phone,
				password_hash = EXCLUDED.password_hash,
				full_name = EXCLUDED.full_name,
				is_active = EXCLUDED.is_active,
				updated_at = now()
			RETURNING id
		`
		var userID string
		err := tx.QueryRowContext(ctx, userQ, u.ID, u.Email, u.Phone, passwordHash, u.FullName).Scan(&userID)
		if err != nil {
			return nil, fmt.Errorf("upsert user %s (%s): %w", u.Email, u.ID, err)
		}
		summary.UsersCount++

		// Upsert active RT membership
		memQ := `
			INSERT INTO user_rt_memberships (user_id, rt_id, role, is_active)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (user_id, rt_id) WHERE is_active = true DO UPDATE SET
				role = EXCLUDED.role,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, memQ, userID, u.RTID, u.Role); err != nil {
			return nil, fmt.Errorf("upsert membership user=%s rt=%s role=%s: %w", userID, u.RTID, u.Role, err)
		}
		summary.MembershipsCount++
	}

	// 8. Position Users (jabatan)
	// All position users get system_role=NULL and role="pengurus" with jabatan assigned.
	// Idempotent via email conflict.
	for _, p := range CanonicalPositionUsers {
		userQ := `
			INSERT INTO users (id, email, phone, password_hash, full_name, is_active, system_role)
			VALUES ($1, $2, $3, $4, $5, true, NULL)
			ON CONFLICT ((lower(TRIM(BOTH FROM email)))) DO UPDATE SET
				phone = EXCLUDED.phone,
				password_hash = EXCLUDED.password_hash,
				full_name = EXCLUDED.full_name,
				is_active = EXCLUDED.is_active,
				updated_at = now()
			RETURNING id
		`
		var userID string
		err := tx.QueryRowContext(ctx, userQ, p.ID, p.Email, p.Phone, passwordHash, p.FullName).Scan(&userID)
		if err != nil {
			return nil, fmt.Errorf("upsert position user %s: %w", p.Email, err)
		}
		summary.PositionUsersCount++

		// Upsert active RT membership with jabatan
		memQ := `
			INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (user_id, rt_id) WHERE is_active = true DO UPDATE SET
				role = EXCLUDED.role,
				jabatan = EXCLUDED.jabatan,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, memQ, userID, p.RTID, "pengurus", p.Jabatan); err != nil {
			return nil, fmt.Errorf("upsert position membership user=%s rt=%s jabatan=%s: %w", userID, p.RTID, p.Jabatan, err)
		}
		summary.PositionMembershipsCount++
	}

	// 8b. Position Residents — link eligible position users to resident records.
	// Only certain jabatan are eligible: ketua, wakil_ketua, sekretaris, bendahara, sosial.
	// Excluded: keamanan, kebersihan_pembangunan (no resident record at all).
	// Each eligible position gets its own household chain: physical_house → household → occupancy → residency_period.
	for _, prs := range positionResidentData {
		// 1. Create physical house (idempotent via ON CONFLICT)
		houseQ := `
			INSERT INTO physical_houses (id, rt_id, house_number, address, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (id) DO UPDATE SET
				rt_id = EXCLUDED.rt_id,
				house_number = EXCLUDED.house_number,
				address = EXCLUDED.address,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, houseQ, prs.HouseID, prs.RTID, prs.HouseNumber, prs.HouseAddress); err != nil {
			return nil, fmt.Errorf("upsert position house %s: %w", prs.HouseID, err)
		}

		// 2. Create household (idempotent via ON CONFLICT)
		householdQ := `
			INSERT INTO households (id, rt_id, head_name, is_active)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (id) DO UPDATE SET
				rt_id = EXCLUDED.rt_id,
				head_name = EXCLUDED.head_name,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, householdQ, prs.HouseholdID, prs.RTID, prs.HeadName); err != nil {
			return nil, fmt.Errorf("upsert position household %s: %w", prs.HouseholdID, err)
		}

		// 3. Create household occupancy linking physical house → household
		occQ := `
			INSERT INTO household_occupancies (id, physical_house_id, household_id, occupancy_status, start_date, end_date)
			VALUES ($1, $2, $3, 'OWNER', CURRENT_DATE, NULL)
			ON CONFLICT (id) DO UPDATE SET
				physical_house_id = EXCLUDED.physical_house_id,
				household_id = EXCLUDED.household_id,
				occupancy_status = EXCLUDED.occupancy_status,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, occQ, prs.OccupancyID, prs.HouseID, prs.HouseholdID); err != nil {
			return nil, fmt.Errorf("upsert position occupancy %s: %w", prs.OccupancyID, err)
		}

		// 4. Create resident record
		residentID := ""
		residentQ := `
			INSERT INTO residents (id, rt_id, user_id, full_name, phone, nik, email, is_active)
			SELECT gen_random_uuid(), $1, $2, $3, $4, $5, $6, true
			ON CONFLICT DO NOTHING
			RETURNING id
		`
		err := tx.QueryRowContext(ctx, residentQ, prs.RTID, prs.UserID, prs.FullName, prs.Phone, prs.NIK, prs.Email).Scan(&residentID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("create position resident: %w", err)
		}

		// If resident was not created (should not happen with gen_random_uuid), check if one already exists for this user
		if residentID == "" {
			var existingID string
			err = tx.QueryRowContext(ctx,
				`SELECT id FROM residents WHERE user_id = $1 LIMIT 1`, prs.UserID).Scan(&existingID)
			if err != nil && err != sql.ErrNoRows {
				return nil, fmt.Errorf("check existing position resident: %w", err)
			}
			residentID = existingID
		}

		// 5. Create residency period for this resident
		rpQ := `
			INSERT INTO residency_periods (id, resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
			VALUES (gen_random_uuid(), $1, $2, $3, CURRENT_DATE, NULL)
			ON CONFLICT DO NOTHING
		`
		if _, err := tx.ExecContext(ctx, rpQ, residentID, prs.OccupancyID, prs.RelationshipToHead); err != nil {
			return nil, fmt.Errorf("create position resident occupancy: %w", err)
		}

		summary.PositionResidentsCount++
	}

	// 8c. Special Position Residents — create resident records for non-eligible
	// position users (keamanan, kebersihan_pembangunan). These residents have no
	// household chain, but do have a user_id link and are active.
	for _, sprs := range specialPositionResidentData {
		residentID := ""
		residentQ := `
			INSERT INTO residents (id, rt_id, user_id, full_name, is_active, nik, email)
			SELECT gen_random_uuid(), $1, $2, $3, true, $4, $5
			ON CONFLICT DO NOTHING
			RETURNING id
		`
		err := tx.QueryRowContext(ctx, residentQ, sprs.RTID, sprs.UserID, sprs.FullName, sprs.NIK, sprs.Email).Scan(&residentID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("create special position resident %s: %w", sprs.FullName, err)
		}
		// Resident may already exist from a previous seed run; check if one exists for this user
		if residentID == "" {
			var existingID string
			err = tx.QueryRowContext(ctx,
				`SELECT id FROM residents WHERE user_id = $1 LIMIT 1`, sprs.UserID).Scan(&existingID)
			if err != nil && err != sql.ErrNoRows {
				return nil, fmt.Errorf("check existing special position resident: %w", err)
			}
			residentID = existingID
		}
		if residentID != "" {
			summary.PositionResidentsCount++
		}
	}

	// 9. Permission Types — seed once (let DB generate UUIDs)
	permissionCodes := []struct {
		Code      string
		Domain    string
		Operation string
	}{
		{"warga.read", "warga", "read"},
		{"warga.create", "warga", "create"},
		{"warga.update", "warga", "update"},
		{"warga.import", "warga", "import"},
		{"warga.export", "warga", "export"},
		{"finance.read", "finance", "read"},
		{"finance.create", "finance", "create"},
		{"finance.update", "finance", "update"},
		{"finance.approve", "finance", "approve"},
		{"reports.read", "reports", "read"},
		{"rt.settings.read", "rt.settings", "read"},
		{"rt.settings.manage", "rt.settings", "manage"},
	}

	for _, pc := range permissionCodes {
		q := `
			INSERT INTO permission_types (code, domain, operation)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO UPDATE SET
				domain = EXCLUDED.domain,
				operation = EXCLUDED.operation
		`
		if _, err := tx.ExecContext(ctx, q, pc.Code, pc.Domain, pc.Operation); err != nil {
			return nil, fmt.Errorf("upsert permission_type %s: %w", pc.Code, err)
		}
		summary.PermissionTypesCount++
	}

	// 10. Position Permissions — resolve permission_id by code, then insert
	positionPermissions := map[string][]string{
		"ketua":                  {"warga.read", "warga.create", "warga.update", "warga.import", "warga.export", "finance.read", "finance.create", "finance.update", "finance.approve", "reports.read", "rt.settings.read", "rt.settings.manage"},
		"wakil_ketua":            {"warga.read", "warga.create", "warga.update", "finance.read", "reports.read", "rt.settings.read"},
		"sekretaris":             {"warga.read", "warga.create", "warga.update", "reports.read"},
		"bendahara":              {"warga.read", "warga.create", "warga.update", "finance.read", "finance.create", "finance.update", "finance.approve", "reports.read"},
		"keamanan":               {"warga.read", "reports.read"},
		"sosial":                 {"warga.read", "reports.read"},
		"kebersihan_pembangunan": {"warga.read", "reports.read"},
	}

	for position, codes := range positionPermissions {
		for _, code := range codes {
			q := `
				INSERT INTO position_permissions (position, permission_id)
				SELECT $1, id FROM permission_types WHERE code = $2
				ON CONFLICT (position, permission_id) DO NOTHING
			`
			if _, err := tx.ExecContext(ctx, q, position, code); err != nil {
				return nil, fmt.Errorf("upsert position_permission %s -> %s: %w", position, code, err)
			}
			summary.PositionPermissionsCount++
		}
	}

	return summary, nil
}
