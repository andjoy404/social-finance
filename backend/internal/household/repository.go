package household

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Standard domain errors.
var (
	ErrNotFound             = errors.New("household or resident not found")
	ErrDuplicateHouseNumber = errors.New("duplicate house_number within the same RT")
	ErrDuplicateNik         = errors.New("duplicate NIK within the same RT")
	ErrOccupiedHouse        = errors.New("physical house is already occupied by another household")
	ErrInvalidDate          = errors.New("invalid date format, expected YYYY-MM-DD")
	ErrNoCurrentOccupancy   = errors.New("household has no current occupancy")
)

// isPGUniqueViolation checks for PostgreSQL unique constraint violation.
func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}

// isPGNikViolation checks for PostgreSQL unique constraint violation on NIK.
func isPGNikViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "nik") || strings.Contains(pgErr.Message, "nik") || strings.Contains(pgErr.Detail, "nik") {
			return true
		}
	}
	return false
}

type updateField struct {
	column string
	value  interface{}
}

func setClause(fields []updateField, baseIdx int) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = fmt.Sprintf("%s = $%d", f.column, baseIdx+i+1)
	}
	return strings.Join(parts, ", ")
}

// scanHouseholdRow scans a household and its projected head resident from a row.
func scanHouseholdRow(scan func(...interface{}) error) (*Household, error) {
	var item Household
	var houseNumber, address, occStatus sql.NullString
	var resID, resRTID, resFullName, resPhone, resNik, resEmail sql.NullString
	var resIsActive sql.NullBool
	var resCreatedAt, resUpdatedAt sql.NullTime

	err := scan(
		&item.ID, &item.RTID, &item.HeadName,
		&item.IsActive, &item.CreatedAt, &item.UpdatedAt,
		&houseNumber, &address, &occStatus,
		&resID, &resRTID, &resFullName, &resPhone, &resNik, &resEmail, &resIsActive, &resCreatedAt, &resUpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if houseNumber.Valid {
		item.HouseNumber = &houseNumber.String
	}
	if address.Valid {
		item.Address = &address.String
	}
	if occStatus.Valid {
		st := OccupancyStatus(occStatus.String)
		item.OccupancyStatus = &st
	}

	if resID.Valid {
		headRes := Resident{
			ID:        resID.String,
			RTID:      resRTID.String,
			FullName:  resFullName.String,
			IsActive:  resIsActive.Bool,
			CreatedAt: resCreatedAt.Time,
			UpdatedAt: resUpdatedAt.Time,
		}
		hhID := item.ID
		headRes.HouseholdID = &hhID
		relHead := RelationshipHead
		headRes.RelationshipToHead = &relHead

		if resNik.Valid {
			headRes.Nik = &resNik.String
			item.Nik = &resNik.String
		}
		if resPhone.Valid {
			headRes.Phone = &resPhone.String
			item.Phone = &resPhone.String
		}
		if resEmail.Valid {
			headRes.Email = &resEmail.String
			item.Email = &resEmail.String
		}
		item.HeadResident = &headRes
	}

	return &item, nil
}

const householdBaseSelect = `
	SELECT h.id, h.rt_id, h.head_name, h.is_active, h.created_at, h.updated_at,
	       ph.house_number, ph.address, ho.occupancy_status,
	       r.id, r.rt_id, r.full_name, r.phone, r.nik, r.email, r.is_active, r.created_at, r.updated_at
	FROM households h
	LEFT JOIN household_occupancies ho ON ho.household_id = h.id AND ho.end_date IS NULL
	LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
	LEFT JOIN LATERAL (
	    SELECT r.id, r.rt_id, r.full_name, r.phone, r.nik, r.email, r.is_active, r.created_at, r.updated_at
	    FROM residency_periods rp
	    JOIN residents r ON r.id = rp.resident_id
	    WHERE rp.household_occupancy_id = ho.id
	      AND rp.end_date IS NULL
	      AND (rp.relationship_to_head = 'HEAD' OR rp.relationship_to_head = 'head' OR rp.relationship_to_head = 'Kepala Keluarga' OR rp.relationship_to_head = 'self')
	    ORDER BY rp.created_at ASC
	    LIMIT 1
	) r ON true
`

// householdBaseCount is used for counting households across all RTs.
const householdBaseCount = `
	SELECT COUNT(*)
	FROM households h
	LEFT JOIN household_occupancies ho ON ho.household_id = h.id AND ho.end_date IS NULL
	LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
	LEFT JOIN LATERAL (
	    SELECT r.id, r.rt_id, r.full_name, r.phone, r.nik, r.email, r.is_active, r.created_at, r.updated_at
	    FROM residency_periods rp
	    JOIN residents r ON r.id = rp.resident_id
	    WHERE rp.household_occupancy_id = ho.id
	      AND rp.end_date IS NULL
	      AND (rp.relationship_to_head = 'HEAD' OR rp.relationship_to_head = 'head' OR rp.relationship_to_head = 'Kepala Keluarga' OR rp.relationship_to_head = 'self')
	    ORDER BY rp.created_at ASC
	    LIMIT 1
	) r ON true
`

// ─── Household ──────────────────────────────────────────────────────────────

// HouseholdCreate creates a new physical house (or reuses an unoccupied one),
// inserts the household, creates current household occupancy, creates head resident,
// and links head resident via residency period.
func HouseholdCreate(ctx context.Context, tx *sql.Tx, in CreateHouseholdInput) (*Household, error) {
	trimmedHN := strings.TrimSpace(in.HouseNumber)
	if trimmedHN == "" {
		return nil, fmt.Errorf("house_number is required")
	}

	// 1. Check or create physical house in this RT
	var phID string
	var phAddress sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT id, address FROM physical_houses WHERE rt_id = $1 AND house_number = $2 AND is_active = true LIMIT 1`,
		in.RTID, trimmedHN,
	).Scan(&phID, &phAddress)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create physical house
			err = tx.QueryRowContext(ctx,
				`INSERT INTO physical_houses (rt_id, house_number, address, is_active)
				 VALUES ($1, $2, $3, true) RETURNING id`,
				in.RTID, trimmedHN, in.Address,
			).Scan(&phID)
			if err != nil {
				if isPGUniqueViolation(err) {
					return nil, ErrDuplicateHouseNumber
				}
				return nil, fmt.Errorf("create physical house: %w", err)
			}
		} else {
			return nil, fmt.Errorf("check physical house: %w", err)
		}
	} else {
		// Physical house already exists — check if it is occupied
		var existingOccID string
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM household_occupancies WHERE physical_house_id = $1 AND end_date IS NULL LIMIT 1`,
			phID,
		).Scan(&existingOccID)
		if err == nil {
			return nil, ErrOccupiedHouse
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("check occupancy collision: %w", err)
		}

		// Update address if provided and not yet set
		if in.Address != nil && *in.Address != "" && !phAddress.Valid {
			_, err = tx.ExecContext(ctx,
				`UPDATE physical_houses SET address = $1, updated_at = now() WHERE id = $2`,
				*in.Address, phID,
			)
			if err != nil {
				return nil, fmt.Errorf("update physical house address: %w", err)
			}
		}
	}

	// 2. Insert into households
	headName := in.HeadName
	if in.FullName != nil && strings.TrimSpace(*in.FullName) != "" {
		headName = strings.TrimSpace(*in.FullName)
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	var item Household
	err = tx.QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name, is_active)
		 VALUES ($1, $2, $3)
		 RETURNING id, rt_id, head_name, is_active, created_at, updated_at`,
		in.RTID, headName, isActive,
	).Scan(
		&item.ID, &item.RTID, &item.HeadName,
		&item.IsActive, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create household: %w", err)
	}

	// 3. Create household occupancy
	var startDate *time.Time
	if in.StartDate != nil && *in.StartDate != "" {
		d, parseErr := time.Parse("2006-01-02", *in.StartDate)
		if parseErr != nil {
			return nil, ErrInvalidDate
		}
		startDate = &d
	} else {
		now := time.Now().UTC().Truncate(24 * time.Hour)
		startDate = &now
	}

	var occID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (physical_house_id, household_id, occupancy_status, start_date, end_date)
		 VALUES ($1, $2, $3, $4, NULL)
		 RETURNING id`,
		phID, item.ID, in.OccupancyStatus, startDate,
	).Scan(&occID)
	if err != nil {
		return nil, fmt.Errorf("create household occupancy: %w", err)
	}

	item.HouseNumber = &trimmedHN
	item.Address = in.Address
	occStatus := in.OccupancyStatus
	item.OccupancyStatus = &occStatus

	// 4. Create head resident & residency period
	var resItem Resident
	err = tx.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, nik, email, is_active)
		 VALUES ($1, $2, $3, $4, $5, true)
		 RETURNING id, rt_id, full_name, phone, nik, email, is_active, created_at, updated_at`,
		in.RTID, headName, in.Phone, in.Nik, in.Email,
	).Scan(
		&resItem.ID, &resItem.RTID, &resItem.FullName, &resItem.Phone,
		&resItem.Nik, &resItem.Email, &resItem.IsActive, &resItem.CreatedAt, &resItem.UpdatedAt,
	)
	if err != nil {
		if isPGNikViolation(err) {
			return nil, ErrDuplicateNik
		}
		return nil, fmt.Errorf("create head resident: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
		 VALUES ($1, $2, $3, $4, NULL)`,
		resItem.ID, occID, RelationshipHead, startDate,
	)
	if err != nil {
		return nil, fmt.Errorf("create head residency period: %w", err)
	}

	resItem.HouseholdID = &item.ID
	relHead := RelationshipHead
	resItem.RelationshipToHead = &relHead

	item.HeadResident = &resItem
	item.Nik = resItem.Nik
	item.Phone = resItem.Phone
	item.Email = resItem.Email

	return &item, nil
}

// HouseholdGetByID finds a household by UUID within the given RT,
// projecting its current occupancy, physical house, and head resident.
func HouseholdGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Household, error) {
	row := tx.QueryRowContext(ctx, householdBaseSelect+` WHERE h.id = $1 AND h.rt_id = $2 LIMIT 1`, id, rtID)
	item, err := scanHouseholdRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get household by id: %w", err)
	}
	return item, nil
}

// HouseholdList returns households filtered by RT with optional active filter and search.
func HouseholdList(ctx context.Context, tx *sql.Tx, rtID string, isActive *bool, search *string, offset, limit int) ([]*Household, error) {
	query := householdBaseSelect + ` WHERE h.rt_id = $1`

	var args []interface{}
	args = append(args, rtID)
	argIndex := 2

	if isActive != nil {
		query += fmt.Sprintf(" AND h.is_active = $%d", argIndex)
		args = append(args, *isActive)
		argIndex++
	}

	if search != nil && *search != "" {
		searchStr := "%" + *search + "%"
		query += fmt.Sprintf(" AND (h.head_name ILIKE $%d OR ph.address ILIKE $%d OR ph.house_number ILIKE $%d OR r.full_name ILIKE $%d OR r.nik ILIKE $%d OR r.phone ILIKE $%d OR r.email ILIKE $%d)",
			argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex)
		args = append(args, searchStr)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY h.created_at ASC OFFSET $%d LIMIT $%d", argIndex, argIndex+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list households: %w", err)
	}
	defer rows.Close()

	result := make([]*Household, 0)
	for rows.Next() {
		item, err := scanHouseholdRow(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan household: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate households: %w", err)
	}
	return result, nil
}

// HouseholdCount returns the number of households in the RT matching the filters.
func HouseholdCount(ctx context.Context, tx *sql.Tx, rtID string, isActive *bool, search *string) (int, error) {
	query := householdBaseCount + ` WHERE h.rt_id = $1`

	var args []interface{}
	args = append(args, rtID)
	argIndex := 2

	if isActive != nil {
		query += fmt.Sprintf(" AND h.is_active = $%d", argIndex)
		args = append(args, *isActive)
		argIndex++
	}

	if search != nil && *search != "" {
		searchStr := "%" + *search + "%"
		query += fmt.Sprintf(" AND (h.head_name ILIKE $%d OR ph.address ILIKE $%d OR ph.house_number ILIKE $%d OR r.full_name ILIKE $%d OR r.nik ILIKE $%d OR r.phone ILIKE $%d OR r.email ILIKE $%d)",
			argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex)
		args = append(args, searchStr)
		argIndex++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count households: %w", err)
	}
	return count, nil
}

// HouseholdListAll returns households across all RTs with optional active filter and search.
// Used by SUPER_ADMIN for global read.
func HouseholdListAll(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, offset, limit int) ([]*Household, error) {
	query := householdBaseSelect + ` WHERE 1=1`

	var args []interface{}
	argIndex := 1

	if isActive != nil {
		query += fmt.Sprintf(" AND h.is_active = $%d", argIndex)
		args = append(args, *isActive)
		argIndex++
	}

	if search != nil && *search != "" {
		searchStr := "%" + *search + "%"
		query += fmt.Sprintf(" AND (h.head_name ILIKE $%d OR ph.address ILIKE $%d OR ph.house_number ILIKE $%d OR r.full_name ILIKE $%d OR r.nik ILIKE $%d OR r.phone ILIKE $%d OR r.email ILIKE $%d)",
			argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex)
		args = append(args, searchStr)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY h.created_at ASC OFFSET $%d LIMIT $%d", argIndex, argIndex+1)
	args = append(args, offset, limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all households: %w", err)
	}
	defer rows.Close()

	result := make([]*Household, 0)
	for rows.Next() {
		item, err := scanHouseholdRow(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan household: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate households: %w", err)
	}
	return result, nil
}

// HouseholdCountAll returns the number of households across all RTs
// matching the active filter and search. Used by SUPER_ADMIN for global read.
func HouseholdCountAll(ctx context.Context, tx *sql.Tx, isActive *bool, search *string) (int, error) {
	query := householdBaseCount + ` WHERE 1=1`

	var args []interface{}
	argIndex := 1

	if isActive != nil {
		query += fmt.Sprintf(" AND h.is_active = $%d", argIndex)
		args = append(args, *isActive)
		argIndex++
	}

	if search != nil && *search != "" {
		searchStr := "%" + *search + "%"
		query += fmt.Sprintf(" AND (h.head_name ILIKE $%d OR ph.address ILIKE $%d OR ph.house_number ILIKE $%d OR r.full_name ILIKE $%d OR r.nik ILIKE $%d OR r.phone ILIKE $%d OR r.email ILIKE $%d)",
			argIndex, argIndex, argIndex, argIndex, argIndex, argIndex, argIndex)
		args = append(args, searchStr)
		argIndex++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count all households: %w", err)
	}
	return count, nil
}

// HouseholdGetByIDAll finds a household by UUID across all RTs,
// projecting its current occupancy, physical house, and head resident.
func HouseholdGetByIDAll(ctx context.Context, tx *sql.Tx, id string) (*Household, error) {
	row := tx.QueryRowContext(ctx, householdBaseSelect+` WHERE h.id = $1 LIMIT 1`, id)
	item, err := scanHouseholdRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get household by id all: %w", err)
	}
	return item, nil
}

// HouseholdGetRTByID returns the rt_id of a household by id without filtering by tenant.
// This is used by system super_admin handlers to derive the target RT from the resource itself.
// Only active households are returned.
func HouseholdGetRTByID(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var rtID string
	err := tx.QueryRowContext(ctx,
		`SELECT rt_id FROM households WHERE id = $1 AND is_active = true`, id,
	).Scan(&rtID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get household rt_id: %w", err)
	}
	return rtID, nil
}

// HouseholdGetRTByIDAnyStatus returns the rt_id of a household by id regardless of active status.
// This is used by update operations that must allow editing inactive households.
func HouseholdGetRTByIDAnyStatus(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var rtID string
	err := tx.QueryRowContext(ctx,
		`SELECT rt_id FROM households WHERE id = $1`, id,
	).Scan(&rtID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get household rt_id any status: %w", err)
	}
	return rtID, nil
}

// ResidentGetRTByID returns the rt_id of a resident by id without filtering by tenant.
// This is used by system super_admin handlers to derive the target RT from the resident itself.
func ResidentGetRTByID(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var rtID string
	err := tx.QueryRowContext(ctx,
		`SELECT rt_id FROM residents WHERE id = $1 AND is_active = true`, id,
	).Scan(&rtID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get resident rt_id: %w", err)
	}
	return rtID, nil
}

// HouseholdUpdate partially updates a household and its current physical house/occupancy/head resident.
// Correcting house_number updates the same physical house. Collision -> ErrDuplicateHouseNumber.
func HouseholdUpdate(ctx context.Context, tx *sql.Tx, id, rtID string, in *UpdateHouseholdInput) (*Household, error) {
	// First fetch current household info with occupancy
	var hhID, currentHeadName, phID, hoID sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT h.id, h.head_name, ho.physical_house_id, ho.id
		 FROM households h
		 LEFT JOIN household_occupancies ho ON ho.household_id = h.id AND ho.end_date IS NULL
		 WHERE h.id = $1 AND h.rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(&hhID, &currentHeadName, &phID, &hoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find household for update: %w", err)
	}

	hasChanges := false

	// 1. Physical house corrections
	if phID.Valid {
		if in.HouseNumber != nil {
			trimmed := strings.TrimSpace(*in.HouseNumber)
			if trimmed == "" {
				return nil, fmt.Errorf("house_number cannot be empty")
			}
			_, err = tx.ExecContext(ctx,
				`UPDATE physical_houses SET house_number = $1, updated_at = now() WHERE id = $2 AND rt_id = $3`,
				trimmed, phID.String, rtID,
			)
			if err != nil {
				if isPGUniqueViolation(err) {
					return nil, ErrDuplicateHouseNumber
				}
				return nil, fmt.Errorf("update house_number: %w", err)
			}
			hasChanges = true
		}
		if in.Address != nil {
			_, err = tx.ExecContext(ctx,
				`UPDATE physical_houses SET address = $1, updated_at = now() WHERE id = $2 AND rt_id = $3`,
				*in.Address, phID.String, rtID,
			)
			if err != nil {
				return nil, fmt.Errorf("update physical house address: %w", err)
			}
			hasChanges = true
		}
	}

	// 2. Current occupancy corrections
	if hoID.Valid && in.OccupancyStatus != nil {
		if err := in.OccupancyStatus.Validate(); err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE household_occupancies SET occupancy_status = $1, updated_at = now() WHERE id = $2`,
			*in.OccupancyStatus, hoID.String,
		)
		if err != nil {
			return nil, fmt.Errorf("update occupancy status: %w", err)
		}
		hasChanges = true
	}

	// 3. Head Resident updates or repair legacy household
	if hoID.Valid {
		var headResID sql.NullString
		err = tx.QueryRowContext(ctx,
			`SELECT r.id
			 FROM residency_periods rp
			 JOIN residents r ON r.id = rp.resident_id
			 WHERE rp.household_occupancy_id = $1
			   AND rp.end_date IS NULL
			   AND (rp.relationship_to_head = 'HEAD' OR rp.relationship_to_head = 'head' OR rp.relationship_to_head = 'Kepala Keluarga' OR rp.relationship_to_head = 'self')
			 ORDER BY rp.created_at ASC
			 LIMIT 1`,
			hoID.String,
		).Scan(&headResID)

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find head resident: %w", err)
		}

		if headResID.Valid {
			// Update existing head resident
			var rFields []updateField
			var rArgs []interface{}
			rIdx := 1

			effectiveName := in.FullName
			if effectiveName == nil {
				effectiveName = in.HeadName
			}
			if effectiveName != nil {
				rFields = append(rFields, updateField{"full_name", *effectiveName})
				rArgs = append(rArgs, *effectiveName)
				rIdx++
			}
			if in.Nik != nil {
				rFields = append(rFields, updateField{"nik", *in.Nik})
				rArgs = append(rArgs, *in.Nik)
				rIdx++
			}
			if in.Phone != nil {
				rFields = append(rFields, updateField{"phone", *in.Phone})
				rArgs = append(rArgs, *in.Phone)
				rIdx++
			}
			if in.Email != nil {
				rFields = append(rFields, updateField{"email", *in.Email})
				rArgs = append(rArgs, *in.Email)
				rIdx++
			}

			if len(rFields) > 0 {
				q := `UPDATE residents SET ` + setClause(rFields, 0) +
					`, updated_at = now() WHERE id = $` + fmt.Sprint(rIdx) +
					` AND rt_id = $` + fmt.Sprint(rIdx+1) + ` AND is_active = true`
				rArgs = append(rArgs, headResID.String, rtID)
				_, err = tx.ExecContext(ctx, q, rArgs...)
				if err != nil {
					if isPGNikViolation(err) {
						return nil, ErrDuplicateNik
					}
					return nil, fmt.Errorf("update head resident: %w", err)
				}
				hasChanges = true
			}
		} else if in.Nik != nil || in.Phone != nil || in.Email != nil || in.FullName != nil || in.HeadName != nil {
			// Legacy household without head resident: create head resident + residency period
			name := currentHeadName.String
			if in.FullName != nil && strings.TrimSpace(*in.FullName) != "" {
				name = strings.TrimSpace(*in.FullName)
			} else if in.HeadName != nil && strings.TrimSpace(*in.HeadName) != "" {
				name = strings.TrimSpace(*in.HeadName)
			}
			var newResID string
			err = tx.QueryRowContext(ctx,
				`INSERT INTO residents (rt_id, full_name, phone, nik, email, is_active)
				 VALUES ($1, $2, $3, $4, $5, true)
				 RETURNING id`,
				rtID, name, in.Phone, in.Nik, in.Email,
			).Scan(&newResID)
			if err != nil {
				if isPGNikViolation(err) {
					return nil, ErrDuplicateNik
				}
				return nil, fmt.Errorf("create legacy head resident: %w", err)
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
				 VALUES ($1, $2, 'HEAD', CURRENT_DATE, NULL)`,
				newResID, hoID.String,
			)
			if err != nil {
				return nil, fmt.Errorf("create legacy head residency period: %w", err)
			}
			hasChanges = true
		}
	}

	// 4. Household entity updates (head_name, is_active)
	var fields []updateField
	var args []interface{}
	argIndex := 1

	effectiveHeadName := in.FullName
	if effectiveHeadName == nil {
		effectiveHeadName = in.HeadName
	}
	if effectiveHeadName != nil {
		fields = append(fields, updateField{"head_name", *effectiveHeadName})
		args = append(args, *effectiveHeadName)
		argIndex++
	}
	if in.IsActive != nil {
		fields = append(fields, updateField{"is_active", *in.IsActive})
		args = append(args, *in.IsActive)
		argIndex++
	}

	if len(fields) > 0 {
		query := `UPDATE households SET ` + setClause(fields, 0) +
			`, updated_at = now() WHERE id = $` + fmt.Sprint(len(fields)+1) +
			` AND rt_id = $` + fmt.Sprint(len(fields)+2)
		args = append(args, id, rtID)
		_, err = tx.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("update household: %w", err)
		}
		hasChanges = true
	}

	if !hasChanges {
		return nil, ErrNotFound
	}

	return HouseholdGetByID(ctx, tx, id, rtID)
}

// HouseholdMove atomically ends current occupancy at start_date, finds or creates
// the target physical house, and creates a new current occupancy starting at start_date.
// It also transitions active resident residency periods to the new occupancy.
func HouseholdMove(ctx context.Context, tx *sql.Tx, id, rtID string, in MoveHouseholdInput) (*Household, error) {
	startDate, err := time.Parse("2006-01-02", in.StartDate)
	if err != nil {
		return nil, ErrInvalidDate
	}
	trimmedHN := strings.TrimSpace(in.HouseNumber)
	if trimmedHN == "" {
		return nil, fmt.Errorf("house_number is required")
	}

	// 1. Get current occupancy
	var currHOID, currPHID string
	var currStatus string
	var currStart sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT ho.id, ho.physical_house_id, ho.occupancy_status, ho.start_date
		 FROM household_occupancies ho
		 JOIN households h ON ho.household_id = h.id
		 WHERE h.id = $1 AND h.rt_id = $2 AND h.is_active = true AND ho.end_date IS NULL
		 LIMIT 1`,
		id, rtID,
	).Scan(&currHOID, &currPHID, &currStatus, &currStart)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoCurrentOccupancy
		}
		return nil, fmt.Errorf("find current occupancy: %w", err)
	}

	if currStart.Valid && startDate.Before(currStart.Time) {
		return nil, fmt.Errorf("move start_date (%s) cannot be before current occupancy start_date (%s)",
			in.StartDate, currStart.Time.Format("2006-01-02"))
	}

	// 2. Find or create target physical house
	var targetPHID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM physical_houses WHERE rt_id = $1 AND house_number = $2 AND is_active = true LIMIT 1`,
		rtID, trimmedHN,
	).Scan(&targetPHID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create physical house
			err = tx.QueryRowContext(ctx,
				`INSERT INTO physical_houses (rt_id, house_number, address, is_active)
				 VALUES ($1, $2, $3, true) RETURNING id`,
				rtID, trimmedHN, in.Address,
			).Scan(&targetPHID)
			if err != nil {
				if isPGUniqueViolation(err) {
					return nil, ErrDuplicateHouseNumber
				}
				return nil, fmt.Errorf("create physical house: %w", err)
			}
		} else {
			return nil, fmt.Errorf("check target physical house: %w", err)
		}
	} else {
		// Verify target physical house is not currently occupied by another household
		var occupiedByHH string
		err = tx.QueryRowContext(ctx,
			`SELECT household_id FROM household_occupancies
			 WHERE physical_house_id = $1 AND end_date IS NULL AND household_id != $2 LIMIT 1`,
			targetPHID, id,
		).Scan(&occupiedByHH)
		if err == nil {
			return nil, ErrOccupiedHouse
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("check target occupancy: %w", err)
		}

		if in.Address != nil && *in.Address != "" {
			_, err = tx.ExecContext(ctx,
				`UPDATE physical_houses SET address = $1, updated_at = now() WHERE id = $2`,
				*in.Address, targetPHID,
			)
			if err != nil {
				return nil, fmt.Errorf("update target physical house address: %w", err)
			}
		}
	}

	// 3. Close current occupancy
	_, err = tx.ExecContext(ctx,
		`UPDATE household_occupancies SET end_date = $1, updated_at = now() WHERE id = $2`,
		startDate, currHOID,
	)
	if err != nil {
		return nil, fmt.Errorf("close current occupancy: %w", err)
	}

	// 4. Create new occupancy
	newStatus := currStatus
	if in.OccupancyStatus != nil && in.OccupancyStatus.IsValid() {
		newStatus = string(*in.OccupancyStatus)
	}

	var newHOID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (physical_house_id, household_id, occupancy_status, start_date, end_date)
		 VALUES ($1, $2, $3, $4, NULL) RETURNING id`,
		targetPHID, id, newStatus, startDate,
	).Scan(&newHOID)
	if err != nil {
		return nil, fmt.Errorf("create new occupancy: %w", err)
	}

	// 5. Transition active residents to new occupancy
	type residentPeriod struct {
		id                 string
		residentID         string
		relationshipToHead sql.NullString
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, resident_id, relationship_to_head FROM residency_periods
		 WHERE household_occupancy_id = $1 AND end_date IS NULL`,
		currHOID,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch active residency periods: %w", err)
	}
	defer rows.Close()

	var periods []residentPeriod
	for rows.Next() {
		var rp residentPeriod
		if err := rows.Scan(&rp.id, &rp.residentID, &rp.relationshipToHead); err != nil {
			return nil, fmt.Errorf("scan residency period: %w", err)
		}
		periods = append(periods, rp)
	}
	rows.Close()

	for _, rp := range periods {
		_, err = tx.ExecContext(ctx,
			`UPDATE residency_periods SET end_date = $1, updated_at = now() WHERE id = $2`,
			startDate, rp.id,
		)
		if err != nil {
			return nil, fmt.Errorf("close residency period: %w", err)
		}

		var rel *string
		if rp.relationshipToHead.Valid {
			rel = &rp.relationshipToHead.String
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
			 VALUES ($1, $2, $3, $4, NULL)`,
			rp.residentID, newHOID, rel, startDate,
		)
		if err != nil {
			return nil, fmt.Errorf("create new residency period: %w", err)
		}
	}

	return HouseholdGetByID(ctx, tx, id, rtID)
}

// HouseholdDeactivate soft-deletes a household and terminates its current occupancy and residency periods.
func HouseholdDeactivate(ctx context.Context, tx *sql.Tx, id, rtID string) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE households SET is_active = false, updated_at = now()
		 WHERE id = $1 AND rt_id = $2 AND is_active = true`,
		id, rtID,
	)
	if err != nil {
		return fmt.Errorf("deactivate household: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check household deactivate rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Close current occupancy and residency periods
	_, err = tx.ExecContext(ctx,
		`UPDATE residency_periods
		 SET end_date = CASE
		     WHEN start_date IS NULL THEN CURRENT_DATE
		     WHEN CURRENT_DATE > start_date THEN CURRENT_DATE
		     ELSE start_date + INTERVAL '1 day'
		 END,
		 updated_at = now()
		 WHERE household_occupancy_id IN (
		     SELECT id FROM household_occupancies WHERE household_id = $1 AND end_date IS NULL
		 ) AND end_date IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("close residency periods: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE household_occupancies
		 SET end_date = CASE
		     WHEN start_date IS NULL THEN CURRENT_DATE
		     WHEN CURRENT_DATE > start_date THEN CURRENT_DATE
		     ELSE start_date + INTERVAL '1 day'
		 END,
		 updated_at = now()
		 WHERE household_id = $1 AND end_date IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("close household occupancies: %w", err)
	}

	return nil
}

// ─── Resident ───────────────────────────────────────────────────────────────

// ResidentCreate creates a new resident attached to the current occupancy of the specified household.
func ResidentCreate(ctx context.Context, tx *sql.Tx, in CreateResidentInput, rtID string) (*Resident, error) {
	// 1. Verify household exists, is active, and belongs to RT
	var hhID string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM households WHERE id = $1 AND rt_id = $2 AND is_active = true`,
		in.HouseholdID, rtID,
	).Scan(&hhID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("validate household: %w", err)
	}

	// 2. Find current occupancy for household
	var hoID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM household_occupancies WHERE household_id = $1 AND end_date IS NULL LIMIT 1`,
		hhID,
	).Scan(&hoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoCurrentOccupancy
		}
		return nil, fmt.Errorf("find current occupancy: %w", err)
	}

	// 3. Insert resident
	var item Resident
	err = tx.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, nik, email, is_active)
		 VALUES ($1, $2, $3, $4, $5, true)
		 RETURNING id, rt_id, full_name, phone, nik, email, is_active, created_at, updated_at`,
		rtID, in.FullName, in.Phone, in.Nik, in.Email,
	).Scan(
		&item.ID, &item.RTID, &item.FullName, &item.Phone,
		&item.Nik, &item.Email, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if isPGNikViolation(err) {
			return nil, ErrDuplicateNik
		}
		return nil, fmt.Errorf("create resident: %w", err)
	}

	// 4. Create current residency period
	var startDate *time.Time
	if in.StartDate != nil && *in.StartDate != "" {
		d, parseErr := time.Parse("2006-01-02", *in.StartDate)
		if parseErr != nil {
			return nil, ErrInvalidDate
		}
		startDate = &d
	} else {
		now := time.Now().UTC().Truncate(24 * time.Hour)
		startDate = &now
	}

	var rpID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
		 VALUES ($1, $2, $3, $4, NULL) RETURNING id`,
		item.ID, hoID, in.RelationshipToHead, startDate,
	).Scan(&rpID)
	if err != nil {
		return nil, fmt.Errorf("create residency period: %w", err)
	}

	item.HouseholdID = &in.HouseholdID
	item.RelationshipToHead = in.RelationshipToHead

	return &item, nil
}

// ResidentGetByID finds an active resident by UUID within the given RT,
// projecting current household and relationship.
func ResidentGetByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Resident, error) {
	var item Resident
	var hhID, rel sql.NullString

	err := tx.QueryRowContext(ctx,
		`SELECT r.id, r.rt_id, r.full_name, r.phone, r.nik, r.email, r.is_active, r.created_at, r.updated_at,
		        ho.household_id, rp.relationship_to_head
		 FROM residents r
		 LEFT JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
		 LEFT JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id
		 WHERE r.id = $1 AND r.rt_id = $2 LIMIT 1`,
		id, rtID,
	).Scan(
		&item.ID, &item.RTID, &item.FullName, &item.Phone, &item.Nik, &item.Email, &item.IsActive,
		&item.CreatedAt, &item.UpdatedAt,
		&hhID, &rel,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get resident by id: %w", err)
	}

	if hhID.Valid {
		item.HouseholdID = &hhID.String
	}
	if rel.Valid {
		item.RelationshipToHead = &rel.String
	}

	return &item, nil
}

// ResidentList returns residents filtered by RT with optional household_id, active filter, and search.
func ResidentList(ctx context.Context, tx *sql.Tx, rtID string, householdID *string, isActive *bool, search *string, offset, limit int) ([]*Resident, error) {
	query := `SELECT r.id, r.rt_id, r.full_name, r.phone, r.nik, r.email, r.is_active, r.created_at, r.updated_at,
	                 ho.household_id, rp.relationship_to_head
	          FROM residents r
	          LEFT JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
	          LEFT JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id
	          WHERE r.rt_id = $1`

	var allArgs []interface{}
	allArgs = append(allArgs, rtID)
	pos := 2

	if isActive != nil {
		query += fmt.Sprintf(" AND r.is_active = $%d", pos)
		allArgs = append(allArgs, *isActive)
		pos++
	} else {
		// Default to active
		query += " AND r.is_active = true"
	}

	if householdID != nil {
		query += fmt.Sprintf(" AND ho.household_id = $%d", pos)
		allArgs = append(allArgs, *householdID)
		pos++
	}

	if search != nil && *search != "" {
		searchStr := "%" + *search + "%"
		query += fmt.Sprintf(" AND (r.full_name ILIKE $%d OR r.nik ILIKE $%d OR r.phone ILIKE $%d OR r.email ILIKE $%d)", pos, pos, pos, pos)
		allArgs = append(allArgs, searchStr)
		pos++
	}

	query += fmt.Sprintf(" ORDER BY r.created_at ASC OFFSET $%d LIMIT $%d", pos, pos+1)
	allArgs = append(allArgs, offset, limit)

	rows, err := tx.QueryContext(ctx, query, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("list residents: %w", err)
	}
	defer rows.Close()

	result := make([]*Resident, 0)
	for rows.Next() {
		var item Resident
		var hhID, rel sql.NullString
		if err := rows.Scan(
			&item.ID, &item.RTID, &item.FullName, &item.Phone, &item.Nik, &item.Email,
			&item.IsActive, &item.CreatedAt, &item.UpdatedAt,
			&hhID, &rel,
		); err != nil {
			return nil, fmt.Errorf("scan resident: %w", err)
		}
		if hhID.Valid {
			item.HouseholdID = &hhID.String
		}
		if rel.Valid {
			item.RelationshipToHead = &rel.String
		}
		result = append(result, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate residents: %w", err)
	}
	return result, nil
}

// ResidentCount returns the number of residents in the RT matching the filters.
func ResidentCount(ctx context.Context, tx *sql.Tx, rtID string, householdID *string, isActive *bool) (int, error) {
	query := `SELECT COUNT(*)
	          FROM residents r
	          LEFT JOIN residency_periods rp ON rp.resident_id = r.id AND rp.end_date IS NULL
	          LEFT JOIN household_occupancies ho ON rp.household_occupancy_id = ho.id
	          WHERE r.rt_id = $1`

	var args []interface{}
	args = append(args, rtID)
	argIndex := 2

	if isActive != nil {
		query += fmt.Sprintf(" AND r.is_active = $%d", argIndex)
		args = append(args, *isActive)
		argIndex++
	} else {
		query += " AND r.is_active = true"
	}

	if householdID != nil {
		query += fmt.Sprintf(" AND ho.household_id = $%d", argIndex)
		args = append(args, *householdID)
		argIndex++
	}

	var count int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count residents: %w", err)
	}
	return count, nil
}

// ResidentUpdate partially updates a resident and/or their current relationship_to_head.
func ResidentUpdate(ctx context.Context, tx *sql.Tx, id, rtID string, in *UpdateResidentInput) (*Resident, error) {
	hasChanges := false

	if in.RelationshipToHead != nil {
		_, err := tx.ExecContext(ctx,
			`UPDATE residency_periods SET relationship_to_head = $1, updated_at = now()
			 WHERE resident_id = $2 AND end_date IS NULL`,
			*in.RelationshipToHead, id,
		)
		if err != nil {
			return nil, fmt.Errorf("update residency period relationship: %w", err)
		}
		hasChanges = true
	}

	var fields []updateField
	var args []interface{}
	argIndex := 1

	if in.FullName != nil {
		fields = append(fields, updateField{"full_name", *in.FullName})
		args = append(args, *in.FullName)
		argIndex++
	}
	if in.Phone != nil {
		fields = append(fields, updateField{"phone", *in.Phone})
		args = append(args, *in.Phone)
		argIndex++
	}
	if in.Nik != nil {
		fields = append(fields, updateField{"nik", *in.Nik})
		args = append(args, *in.Nik)
		argIndex++
	}
	if in.Email != nil {
		fields = append(fields, updateField{"email", *in.Email})
		args = append(args, *in.Email)
		argIndex++
	}
	if in.IsActive != nil {
		fields = append(fields, updateField{"is_active", *in.IsActive})
		args = append(args, *in.IsActive)
		argIndex++
	}

	if len(fields) > 0 {
		query := `UPDATE residents SET ` + setClause(fields, 0) +
			`, updated_at = now() WHERE id = $` + fmt.Sprint(len(fields)+1) +
			` AND rt_id = $` + fmt.Sprint(len(fields)+2) + ` AND is_active = true`
		args = append(args, id, rtID)
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			if isPGNikViolation(err) {
				return nil, ErrDuplicateNik
			}
			return nil, fmt.Errorf("update resident: %w", err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 && !hasChanges {
			return nil, ErrNotFound
		}
		hasChanges = true
	}

	if !hasChanges {
		return nil, ErrNotFound
	}

	return ResidentGetByID(ctx, tx, id, rtID)
}

// ResidentMove moves a resident from their current household to another household within the same RT.
func ResidentMove(ctx context.Context, tx *sql.Tx, id, rtID string, in MoveResidentInput) (*Resident, error) {
	startDate, err := time.Parse("2006-01-02", in.StartDate)
	if err != nil {
		return nil, ErrInvalidDate
	}

	// 1. Verify target household belongs to same RT and is active
	var targetHHID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM households WHERE id = $1 AND rt_id = $2 AND is_active = true`,
		in.DestinationHouseholdID, rtID,
	).Scan(&targetHHID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find destination household: %w", err)
	}

	// 2. Find target household's current occupancy
	var targetHOID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM household_occupancies WHERE household_id = $1 AND end_date IS NULL LIMIT 1`,
		targetHHID,
	).Scan(&targetHOID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoCurrentOccupancy
		}
		return nil, fmt.Errorf("find target occupancy: %w", err)
	}

	// 3. Find resident's current residency period
	var currRPID string
	var currStart sql.NullTime
	var currRel sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT rp.id, rp.start_date, rp.relationship_to_head
		 FROM residency_periods rp
		 JOIN residents r ON rp.resident_id = r.id
		 WHERE r.id = $1 AND r.rt_id = $2 AND r.is_active = true AND rp.end_date IS NULL
		 LIMIT 1`,
		id, rtID,
	).Scan(&currRPID, &currStart, &currRel)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find current residency period: %w", err)
	}

	if currStart.Valid && startDate.Before(currStart.Time) {
		return nil, fmt.Errorf("move start_date (%s) cannot be before current residency start_date (%s)",
			in.StartDate, currStart.Time.Format("2006-01-02"))
	}

	// 4. Close current residency period
	_, err = tx.ExecContext(ctx,
		`UPDATE residency_periods SET end_date = $1, updated_at = now() WHERE id = $2`,
		startDate, currRPID,
	)
	if err != nil {
		return nil, fmt.Errorf("close current residency period: %w", err)
	}

	// 5. Create new residency period
	newRel := currRel.String
	if in.RelationshipToHead != nil && *in.RelationshipToHead != "" {
		newRel = *in.RelationshipToHead
	}

	var newRPID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, relationship_to_head, start_date, end_date)
		 VALUES ($1, $2, $3, $4, NULL) RETURNING id`,
		id, targetHOID, newRel, startDate,
	).Scan(&newRPID)
	if err != nil {
		return nil, fmt.Errorf("create new residency period: %w", err)
	}

	return ResidentGetByID(ctx, tx, id, rtID)
}

// ResidentDeactivate soft-deletes a resident and closes their current residency period.
func ResidentDeactivate(ctx context.Context, tx *sql.Tx, id, rtID string) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE residents SET is_active = false, updated_at = now()
		 WHERE id = $1 AND rt_id = $2 AND is_active = true`,
		id, rtID,
	)
	if err != nil {
		return fmt.Errorf("deactivate resident: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check resident deactivate rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE residency_periods
		 SET end_date = CASE
		     WHEN start_date IS NULL THEN CURRENT_DATE
		     WHEN CURRENT_DATE > start_date THEN CURRENT_DATE
		     ELSE start_date + INTERVAL '1 day'
		 END,
		 updated_at = now()
		 WHERE resident_id = $1 AND end_date IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("close resident periods: %w", err)
	}

	return nil
}
