package devseed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

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
	FinancialCategoriesCount  int
	DuesCount                 int
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
		"financial_categories", summary.FinancialCategoriesCount,
		"dues", summary.DuesCount,
		"bills", summary.BillsCount,
		"payments", summary.PaymentsCount,
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

	// 11. Financial Seed — categories, dues, bills, payments
	if err := seedFinancialData(ctx, tx, summary); err != nil {
		return nil, fmt.Errorf("seed financial data: %w", err)
	}

	return summary, nil
}

// seedFinancialData seeds financial categories, dues, bills, and payments.
// All entities use deterministic UUIDs for idempotent upserts.
// Financial seed runs AFTER all organizational seed (RTs, households, occupants, users)
// so that occupancy IDs and bendahara user IDs are available.
func seedFinancialData(ctx context.Context, tx *sql.Tx, summary *SeedSummary) error {
	// --- 11a. Financial Categories ---
	for _, cat := range CanonicalFinancialCategories {
		q := `
			INSERT INTO financial_categories (id, rt_id, name, type, is_active)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				type = EXCLUDED.type,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, cat.ID, cat.RTID, cat.Name, cat.Type, cat.IsActive); err != nil {
			return fmt.Errorf("upsert financial_category %s: %w", cat.ID, err)
		}
	}
	summary.FinancialCategoriesCount = len(CanonicalFinancialCategories)
	slog.Info("seeded financial_categories", "count", summary.FinancialCategoriesCount)

	// --- 11b. Dues ---
	for _, due := range CanonicalDues {
		q := `
			INSERT INTO dues (id, rt_id, name, amount, period_type, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				amount = EXCLUDED.amount,
				period_type = EXCLUDED.period_type,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, due.ID, due.RTID, due.Name, due.Amount, due.PeriodType, due.IsActive); err != nil {
			return fmt.Errorf("upsert due %s: %w", due.ID, err)
		}
	}
	summary.DuesCount = len(CanonicalDues)
	slog.Info("seeded dues", "count", summary.DuesCount)

	// --- 11c. Bills ---
	for _, bill := range CanonicalBills {
		q := `
			INSERT INTO bills (id, rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO UPDATE SET
				household_occupancy_id = EXCLUDED.household_occupancy_id,
				due_id = EXCLUDED.due_id,
				amount = EXCLUDED.amount,
				period = EXCLUDED.period,
				due_date = EXCLUDED.due_date,
				status = EXCLUDED.status,
				updated_at = now()
		`
		if _, err := tx.ExecContext(ctx, q, bill.ID, bill.RTID, bill.HouseholdOccupancyID,
			bill.DueID, bill.Amount, bill.Period, bill.DueDate, bill.Status); err != nil {
			return fmt.Errorf("upsert bill %s: %w", bill.ID, err)
		}
	}
	summary.BillsCount = len(CanonicalBills)
	slog.Info("seeded bills", "count", summary.BillsCount)

	// --- 11d. Payments ---
	for _, pay := range CanonicalPayments {
		verifiedBy := &pay.VerifiedBy
		if pay.VerifiedBy == "" {
			verifiedBy = nil
		}
		var verificationReason, note *string
		if pay.RejectionReason != nil && *pay.RejectionReason != "" {
			verificationReason = pay.RejectionReason
		}
		if pay.Notes != nil && *pay.Notes != "" {
			note = pay.Notes
		}

		q := `
			INSERT INTO payments (id, rt_id, bill_id, amount, method, origin, status,
			                     verified_by, verified_at, rejection_reason, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (id) DO UPDATE SET
				amount = EXCLUDED.amount,
				method = EXCLUDED.method,
				origin = EXCLUDED.origin,
				status = EXCLUDED.status,
				verified_by = EXCLUDED.verified_by,
				verified_at = EXCLUDED.verified_at,
				rejection_reason = EXCLUDED.rejection_reason,
				notes = EXCLUDED.notes,
				updated_at = now()
		`
		var verifiedAt *time.Time
		if pay.VerifiedAt != "" {
			t, err := time.Parse(time.RFC3339, pay.VerifiedAt)
			if err != nil {
				return fmt.Errorf("parse verified_at for payment %s: %w", pay.ID, err)
			}
			verifiedAt = &t
		}

		if _, err := tx.ExecContext(ctx, q, pay.ID, pay.RTID, pay.BillID,
			pay.Amount, pay.Method, pay.Origin, pay.Status,
			verifiedBy, verifiedAt, verificationReason, note); err != nil {
			return fmt.Errorf("upsert payment %s: %w", pay.ID, err)
		}
	}
	summary.PaymentsCount = len(CanonicalPayments)
	slog.Info("seeded payments", "count", summary.PaymentsCount)

	// --- 11e. Update bill statuses based on approved payments ---
	// The backend computes bill status from payments:
	//   paid (lunas):    total APPROVED payments >= bill.amount
	//   partial (sebagian): 0 < total APPROVED payments < bill.amount
	//   unpaid:          0 approved payments
	// Since we seed payments directly (bypassing service layer), we must
	// update bill statuses manually to match.
	type billPaymentTotal struct {
		billID   string
		amount   string
		total    string
	}
	var totals []billPaymentTotal

	tRows, err := tx.QueryContext(ctx, `
		SELECT b.id, b.amount,
		       COALESCE(SUM(CASE WHEN p.status = 'APPROVED' THEN p.amount ELSE 0 END), '0')
		FROM bills b
		LEFT JOIN payments p ON p.bill_id = b.id
		GROUP BY b.id, b.amount
	`)
	if err != nil {
		return fmt.Errorf("query bill payment totals: %w", err)
	}

	for tRows.Next() {
		var t billPaymentTotal
		if err := tRows.Scan(&t.billID, &t.amount, &t.total); err != nil {
			tRows.Close()
			return fmt.Errorf("scan bill payment total: %w", err)
		}
		totals = append(totals, t)
	}
	tRows.Close()
	if err := tRows.Err(); err != nil {
		return fmt.Errorf("iterating bill payment totals: %w", err)
	}

	// Now update all bill statuses after closing the query cursor
	for _, t := range totals {
		var newStatus string
		if t.total == "0" {
			newStatus = "unpaid"
		} else if compareMoney(t.total, t.amount) >= 0 {
			newStatus = "paid"
		} else {
			newStatus = "partial"
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE bills SET status = $1, updated_at = now() WHERE id = $2`,
			newStatus, t.billID); err != nil {
			return fmt.Errorf("update bill %s status to %s: %w", t.billID, newStatus, err)
		}
	}

	return nil
}

// compareMoney compares two numeric(15,2) string values.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b.
// Parses to integer cents to avoid floating-point issues.
func compareMoney(a, b string) int {
	// Remove decimal point and parse as int64 (cents)
	pa := parseMoneyCents(a)
	pb := parseMoneyCents(b)
	if pa < pb {
		return -1
	}
	if pa > pb {
		return 1
	}
	return 0
}

// parseMoneyCents parses a numeric(15,2) string into int64 cents.
// e.g. "50000.00" → 5000000
func parseMoneyCents(s string) int64 {
	parts := strings.Split(s, ".")
	if len(parts) == 1 {
		v, _ := strconv.ParseInt(parts[0], 10, 64)
		return v * 100
	}
	intPart, _ := strconv.ParseInt(parts[0], 10, 64)
	decPart := parts[1]
	// Pad or truncate to exactly 2 decimal digits
	for len(decPart) < 2 {
		decPart += "0"
	}
	decPart = decPart[:2]
	dec, _ := strconv.ParseInt(decPart, 10, 64)
	return intPart*100 + dec
}
