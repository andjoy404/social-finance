package devseed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

const (
	// DefaultSeedPassword is the single known password for all seeded mock users.
	DefaultSeedPassword = "123qweasdzxc"
	// CanonicalUrenaRTID is the deterministic UUID of RT 002 (Urena).
	CanonicalUrenaRTID = "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"
)

// RTQueryExecutor abstracts *sql.Tx or *sql.DB for running queries.
type RTQueryExecutor interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ResolveCanonicalUrenaRT deterministically finds the canonical RT 002 (Urena) tenant ID.
// Resolution priority:
// 1. Exact canonical ID match: a95c1cb5-2f67-4ea5-9197-bfec537b6d7b (with is_active = true)
// 2. Exact name 'Urena' and RT '002' (with is_active = true)
// 3. Name ILIKE '%urena%' and RT '002' (with is_active = true)
//
// If multiple ambiguous active candidates exist at step 2 or 3, it returns an explicit
// diagnostic error instead of arbitrarily choosing one.
// If no matching RT exists (e.g. fresh database before migrations/seed), it returns ("", sql.ErrNoRows).
func ResolveCanonicalUrenaRT(ctx context.Context, q RTQueryExecutor) (string, error) {
	// 1. Prioritize stable canonical ID
	var id string
	err := q.QueryRowContext(ctx, `SELECT id FROM rts WHERE id = $1 AND is_active = true`, CanonicalUrenaRTID).Scan(&id)
	if err == nil && id != "" {
		return id, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("query canonical rt by id: %w", err)
	}

	// 2. Exact match: lower(name) = 'urena' AND rt = '002' AND is_active = true
	rows, err := q.QueryContext(ctx, `SELECT id FROM rts WHERE lower(name) = 'urena' AND rt = '002' AND is_active = true`)
	if err != nil {
		return "", fmt.Errorf("query rt by name 'urena' and code '002': %w", err)
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return "", fmt.Errorf("scan candidate rt: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate candidate rts: %w", err)
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("ambiguous canonical tenant: found %d active RTs matching name 'urena' and rt '002': %v", len(candidates), candidates)
	}

	// 3. Pattern match: name ILIKE '%urena%' AND rt = '002' AND is_active = true
	rows, err = q.QueryContext(ctx, `SELECT id FROM rts WHERE name ILIKE '%urena%' AND rt = '002' AND is_active = true`)
	if err != nil {
		return "", fmt.Errorf("query rt by pattern '%%urena%%' and code '002': %w", err)
	}
	defer rows.Close()

	candidates = nil
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return "", fmt.Errorf("scan candidate rt: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate candidate rts: %w", err)
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("ambiguous canonical tenant: found %d active RTs matching pattern '%%urena%%' and rt '002': %v", len(candidates), candidates)
	}

	return "", sql.ErrNoRows
}

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
	BillsCount                int
	PaymentsCount             int
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

	// NOTE: Super Admin creation moved to bootstrap (main.go --bootstrap).
	// The admin account is created from BOOTSTRAP_EMAIL / BOOTSTRAP_PASSWORD
	// in .env, keeping credentials out of source code.
	// seed --seed-dev intentionally skips creating super_admin users.

	// 1c. If a Super Admin user already exists, idempotently ensure active membership in canonical RT 002.
	var saUserID string
	errSA := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE system_role = 'super_admin' ORDER BY created_at ASC LIMIT 1`).Scan(&saUserID)
	if errSA == nil && saUserID != "" {
		rt002ID, errRT := ResolveCanonicalUrenaRT(ctx, tx)
		if errRT != nil && !errors.Is(errRT, sql.ErrNoRows) {
			return nil, fmt.Errorf("resolve canonical rt 002: %w", errRT)
		}
		if rt002ID != "" {
			_, errMem := tx.ExecContext(ctx, `
				INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active)
				VALUES (gen_random_uuid(), $1, $2, 'pengurus', true)
				ON CONFLICT (user_id, rt_id) WHERE is_active = true DO NOTHING
			`, saUserID, rt002ID)
			if errMem != nil {
				return nil, fmt.Errorf("link super admin to canonical rt 002: %w", errMem)
			}
		}
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
			INSERT INTO residency_periods (id, resident_id, household_occupancy_id, start_date, end_date)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				resident_id = EXCLUDED.resident_id,
				household_occupancy_id = EXCLUDED.household_occupancy_id,
				start_date = EXCLUDED.start_date,
				end_date = EXCLUDED.end_date,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, rp.ID, rp.ResidentID, rp.HouseholdOccupancyID, rp.StartDate, rp.EndDate); err != nil {
			return nil, fmt.Errorf("upsert residency_period %s: %w", rp.ID, err)
		}
		summary.ResidencyPeriodsCount++
	}

	// 7. Users and Memberships
	// Pre-hash the seed password once — all mock users share the same password.
	passwordHash, err := auth.Hash(DefaultSeedPassword)
	if err != nil {
		return nil, fmt.Errorf("hash seed password: %w", err)
	}

	for _, u := range CanonicalUsers {

		// HARD CONSTRAINT: Never allow super_admin to be seeded automatically
		// Super Admin must be created solely via --bootstrap CLI
		//
		// Dual-match upsert: match by seed ID first, fall back to email.
		// If seed ID and seed email point to two different accounts, abort
		// with a clear diagnostic error rather than silently updating the
		// wrong user.
		resolveQ := `
			SELECT
				id_by_id::text,
				id_by_email::text
			FROM (
				SELECT
					(SELECT id FROM users WHERE id = $1) AS id_by_id,
					(SELECT id FROM users
					 WHERE lower(TRIM(BOTH FROM email)) = lower(TRIM(BOTH FROM $2))) AS id_by_email
			) sub
		`
		var idByID, idByEmail *string
		if err := tx.QueryRowContext(ctx, resolveQ, u.ID, u.Email).Scan(&idByID, &idByEmail); err != nil {
			return nil, fmt.Errorf("resolve user %s (%s): %w", u.Email, u.ID, err)
		}

		var userID string

		switch {
		case idByID != nil && idByEmail != nil && *idByID != *idByEmail:
			// Seed ID and seed email point to two different accounts.
			// This is ambiguous and must be resolved before proceeding.
			return nil, fmt.Errorf(
				"seed user %s (%s): ID matched account %s but email matched account %s — ambiguous identity, aborting to prevent silent update of the wrong user",
				u.Email, u.ID, *idByID, *idByEmail,
			)

		case idByID != nil:
			// Seed ID matched an existing user — update it
			updateQ := `
				UPDATE users SET
					email = $2,
					phone = $3,
					password_hash = $4,
					full_name = $5,
					is_active = true,
					updated_at = now()
				WHERE id = $1
				RETURNING id
			`
			if err := tx.QueryRowContext(ctx, updateQ, u.ID, u.Email, u.Phone, passwordHash, u.FullName).Scan(&userID); err != nil {
				return nil, fmt.Errorf("update user %s (%s): %w", u.Email, u.ID, err)
			}

		default:
			// No seed ID match — check if email matched
			if idByEmail != nil {
				// Email matched an existing user with different ID — update it
				updateByEmailQ := `
					UPDATE users SET
						phone = $2,
						password_hash = $3,
						full_name = $4,
						is_active = true,
						updated_at = now()
					WHERE id = $1
					RETURNING id
				`
				if err := tx.QueryRowContext(ctx, updateByEmailQ, *idByEmail, u.Phone, passwordHash, u.FullName).Scan(&userID); err != nil {
					return nil, fmt.Errorf("update user by email %s: %w", u.Email, err)
				}
			} else {
				// Neither ID nor email matched — insert new user
				insertQ := `
					INSERT INTO users (id, email, phone, password_hash, full_name, is_active, system_role)
					VALUES ($1, $2, $3, $4, $5, true, NULL)
					RETURNING id
				`
				if err := tx.QueryRowContext(ctx, insertQ, u.ID, u.Email, u.Phone, passwordHash, u.FullName).Scan(&userID); err != nil {
					return nil, fmt.Errorf("insert user %s (%s): %w", u.Email, u.ID, err)
				}
			}
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
	// Dual-match resolution by deterministic ID and normalized email:
	//   - If both ID and email match different accounts, abort with a diagnostic error.
	//   - If ID matches, verify the existing email on that account does not belong to another seed user, then update.
	//   - If ID does not match but normalized email exists, reuse the existing account ID without creating duplicates.
	//   - If neither matches, insert a new user.
	//   - Track resolvedPositionUserID so Sections 8b and 8c use the resolved userID consistently.
	canonicalEmailOwner := make(map[string]string, len(CanonicalUsers)+len(CanonicalPositionUsers))
	for _, u := range CanonicalUsers {
		canonicalEmailOwner[strings.ToLower(strings.TrimSpace(u.Email))] = u.ID
	}
	for _, p := range CanonicalPositionUsers {
		canonicalEmailOwner[strings.ToLower(strings.TrimSpace(p.Email))] = p.ID
	}

	resolvedPositionUserID := make(map[string]string, len(CanonicalPositionUsers))

	for _, p := range CanonicalPositionUsers {
		resolvePosQ := `
			SELECT
				id_by_id::text,
				email_by_id,
				id_by_email::text
			FROM (
				SELECT
					(SELECT id FROM users WHERE id = $1) AS id_by_id,
					(SELECT email FROM users WHERE id = $1) AS email_by_id,
					(SELECT id FROM users
					 WHERE lower(TRIM(BOTH FROM email)) = lower(TRIM(BOTH FROM $2))) AS id_by_email
			) sub
		`
		var idByID, emailByID, idByEmail *string
		if err := tx.QueryRowContext(ctx, resolvePosQ, p.ID, p.Email).Scan(&idByID, &emailByID, &idByEmail); err != nil {
			return nil, fmt.Errorf("resolve position user %s (%s): %w", p.Email, p.ID, err)
		}

		var userID string

		switch {
		case idByID != nil && idByEmail != nil && *idByID != *idByEmail:
			return nil, fmt.Errorf(
				"seed position user %s (%s): ID matched account %s but email matched account %s — ambiguous identity, aborting to prevent silent update of the wrong user",
				p.Email, p.ID, *idByID, *idByEmail,
			)

		case idByID != nil:
			if emailByID != nil {
				normExisting := strings.ToLower(strings.TrimSpace(*emailByID))
				normTarget := strings.ToLower(strings.TrimSpace(p.Email))
				if normExisting != normTarget {
					if ownerID, ok := canonicalEmailOwner[normExisting]; ok && ownerID != p.ID {
						return nil, fmt.Errorf(
							"seed position user %s (%s): account %s currently holds email %s belonging to seed user %s — aborting to prevent overwriting another account",
							p.Email, p.ID, *idByID, *emailByID, ownerID,
						)
					}
				}
			}
			updatePosQ := `
				UPDATE users SET
					email = $2,
					phone = $3,
					password_hash = $4,
					full_name = $5,
					is_active = true,
					updated_at = now()
				WHERE id = $1
				RETURNING id
			`
			if err := tx.QueryRowContext(ctx, updatePosQ, p.ID, p.Email, p.Phone, passwordHash, p.FullName).Scan(&userID); err != nil {
				return nil, fmt.Errorf("update position user %s (%s): %w", p.Email, p.ID, err)
			}

		case idByEmail != nil:
			updatePosByEmailQ := `
				UPDATE users SET
					phone = $2,
					password_hash = $3,
					full_name = $4,
					is_active = true,
					updated_at = now()
				WHERE id = $1
				RETURNING id
			`
			if err := tx.QueryRowContext(ctx, updatePosByEmailQ, *idByEmail, p.Phone, passwordHash, p.FullName).Scan(&userID); err != nil {
				return nil, fmt.Errorf("update position user by email %s: %w", p.Email, err)
			}

		default:
			insertPosQ := `
				INSERT INTO users (id, email, phone, password_hash, full_name, is_active, system_role)
				VALUES ($1, $2, $3, $4, $5, true, NULL)
				RETURNING id
			`
			if err := tx.QueryRowContext(ctx, insertPosQ, p.ID, p.Email, p.Phone, passwordHash, p.FullName).Scan(&userID); err != nil {
				return nil, fmt.Errorf("insert position user %s (%s): %w", p.Email, p.ID, err)
			}
		}

		resolvedPositionUserID[p.ID] = userID
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
	// Excluded: keamanan, operasional, kebersihan (no household chain).
	// Each eligible position gets its own household chain: physical_house → household → occupancy → residency_period.
	for _, prs := range positionResidentData {
		resolvedUserID := resolvedPositionUserID[prs.UserID]
		if resolvedUserID == "" {
			resolvedUserID = prs.UserID
		}

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
		err := tx.QueryRowContext(ctx, residentQ, prs.RTID, resolvedUserID, prs.FullName, prs.Phone, prs.NIK, prs.Email).Scan(&residentID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("create position resident: %w", err)
		}

		// If resident was not created (e.g. existing NIK or user_id), find existing resident
		if residentID == "" {
			var existingID string
			err = tx.QueryRowContext(ctx,
				`SELECT id FROM residents WHERE user_id = $1 OR (rt_id = $2 AND nik = $3) LIMIT 1`,
				resolvedUserID, prs.RTID, prs.NIK).Scan(&existingID)
			if err != nil && err != sql.ErrNoRows {
				return nil, fmt.Errorf("check existing position resident: %w", err)
			}
			residentID = existingID
			if residentID != "" {
				// Ensure user_id and active status are linked
				_, _ = tx.ExecContext(ctx, `UPDATE residents SET user_id = $1, is_active = true WHERE id = $2`, resolvedUserID, residentID)
			}
		}

		// 5. Create residency period for this resident if residentID is resolved
		if residentID != "" {
			rpQ := `
				INSERT INTO residency_periods (id, resident_id, household_occupancy_id, start_date, end_date)
				VALUES (gen_random_uuid(), $1, $2, CURRENT_DATE, NULL)
				ON CONFLICT DO NOTHING
			`
			if _, err := tx.ExecContext(ctx, rpQ, residentID, prs.OccupancyID); err != nil {
				return nil, fmt.Errorf("create position resident occupancy: %w", err)
			}
		}

		summary.PositionResidentsCount++
	}

	// 8c. Special Position Residents — create resident records for non-eligible
	// position users (keamanan, operasional, kebersihan). These residents have no
	// household chain, but do have a user_id link and are active.
	for _, sprs := range specialPositionResidentData {
		resolvedUserID := resolvedPositionUserID[sprs.UserID]
		if resolvedUserID == "" {
			resolvedUserID = sprs.UserID
		}

		residentID := ""
		residentQ := `
			INSERT INTO residents (id, rt_id, user_id, full_name, is_active, nik, email)
			SELECT gen_random_uuid(), $1, $2, $3, true, $4, $5
			ON CONFLICT DO NOTHING
			RETURNING id
		`
		err := tx.QueryRowContext(ctx, residentQ, sprs.RTID, resolvedUserID, sprs.FullName, sprs.NIK, sprs.Email).Scan(&residentID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("create special position resident %s: %w", sprs.FullName, err)
		}
		// Resident may already exist from a previous seed run; find and relink to resolved user.
		if residentID == "" {
			var existingID string
			err = tx.QueryRowContext(ctx,
				`SELECT id FROM residents WHERE user_id = $1 OR (rt_id = $2 AND nik = $3) LIMIT 1`,
				resolvedUserID, sprs.RTID, sprs.NIK).Scan(&existingID)
			if err != nil && err != sql.ErrNoRows {
				return nil, fmt.Errorf("check existing special position resident: %w", err)
			}
			residentID = existingID
			if residentID != "" {
				_, _ = tx.ExecContext(ctx, `UPDATE residents SET user_id = $1, full_name = $2, email = $3, is_active = true WHERE id = $4`,
					resolvedUserID, sprs.FullName, sprs.Email, residentID)
			}
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
		{"notification.read", "notification", "read"},
		{"notification.manage", "notification", "manage"},
		{"reports.read", "reports", "read"},
		{"rt.settings.read", "rt.settings", "read"},
		{"rt.settings.manage", "rt.settings", "manage"},
		{"activities.read", "community", "read"},
		{"activities.create", "community", "create"},
		{"activities.update", "community", "update"},
		{"activities.publish", "community", "publish"},
		{"activities.cancel", "community", "cancel"},
		{"activities.delete", "community", "delete"},
		{"announcements.read", "community", "read"},
		{"announcements.create", "community", "create"},
		{"announcements.update", "community", "update"},
		{"announcements.publish", "community", "publish"},
		{"announcements.archive", "community", "archive"},
		{"announcements.delete", "community", "delete"},
		{"banner_templates.read", "community", "read"},
		{"banner_templates.manage", "community", "manage"},
		{"organization.read", "community", "read"},
		{"organization.contact.read", "community", "read"},
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
		"ketua": {
			"warga.read", "warga.create", "warga.update", "warga.import", "warga.export",
			"finance.read", "finance.create", "finance.update", "finance.approve",
			"notification.read", "notification.manage", "reports.read", "rt.settings.read", "rt.settings.manage",
			"activities.read", "activities.create", "activities.update", "activities.publish", "activities.cancel", "activities.delete",
			"announcements.read", "announcements.create", "announcements.update", "announcements.publish", "announcements.archive", "announcements.delete",
			"banner_templates.read", "banner_templates.manage",
			"organization.read", "organization.contact.read",
		},
		"wakil_ketua": {
			"warga.read", "warga.create", "warga.update", "finance.read", "notification.read", "reports.read", "rt.settings.read",
			"activities.read", "announcements.read", "banner_templates.read", "organization.read",
		},
		"sekretaris": {
			"warga.read", "warga.create", "warga.update", "finance.read", "finance.create", "notification.read", "reports.read", "rt.settings.read",
			"activities.read", "announcements.read", "banner_templates.read", "organization.read", "organization.contact.read",
		},
		"bendahara": {
			"warga.read", "warga.create", "warga.update", "warga.import", "warga.export", "finance.read", "finance.create", "finance.update", "finance.approve", "notification.read", "reports.read",
			"activities.read", "announcements.read", "banner_templates.read", "organization.read",
		},
		"keamanan":    {"warga.read", "reports.read", "activities.read", "announcements.read", "banner_templates.read", "organization.read"},
		"sosial":      {"warga.read", "reports.read", "activities.read", "announcements.read", "banner_templates.read", "organization.read"},
		"operasional": {"warga.read", "reports.read", "activities.read", "announcements.read", "banner_templates.read", "organization.read"},
		"kebersihan":  {"warga.read", "reports.read", "activities.read", "announcements.read", "banner_templates.read", "organization.read"},
		"anggota":     {"warga.read", "reports.read", "activities.read", "announcements.read", "banner_templates.read", "organization.read"},
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
