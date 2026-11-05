package notification

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"social-finance/internal/database"
	"social-finance/internal/testutil"
)

func testIntegrationPool(t *testing.T) *database.Pool {
	t.Helper()
	pool := testutil.GetTestPool(t)
	if pool == nil {
		t.Skip("no test database configured")
	}
	return pool
}

// TestGetIuranReminderRecipients_Integration tests the recipient selection query
// against a real database, covering payment allocation, partial payments,
// pending/rejected payments, non-obligatory bills, and move-in dates.
func TestGetIuranReminderRecipients_Integration(t *testing.T) {
	pool := testIntegrationPool(t)
	db := pool.Raw()
	repo := &pgRepository{pool: pool}
	ctx := context.Background()

	rtID := createTestRTIntegration(t, db)

	// Create physical house → household → occupancy → residency period → resident → user
	physID := createPhysicalHouseIntegration(t, db, rtID)
	hhID := createHouseholdIntegration(t, db, rtID)
	occID := createOccupancyIntegration(t, db, physID, hhID)
	resID := createResidentIntegration(t, db, occID, rtID)
	userID := createUserIntegration(t, db, resID)
	execIntegration(t, db, `UPDATE residents SET user_id = $1 WHERE id = $2`, userID, resID)
	execIntegration(t, db, `INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES ($1, $2, $3, 'warga', true)`,
		uniqueIDIntegration(), userID, rtID)

	// Second household/occupancy with a different user (for DISTINCT ON user_id)
	physID2 := createPhysicalHouseIntegration(t, db, rtID)
	hhID2 := createHouseholdIntegration(t, db, rtID)
	occID2 := createOccupancyIntegration(t, db, physID2, hhID2)
	resID2 := createResidentIntegration(t, db, occID2, rtID)
	userID2 := createUserIntegration(t, db, resID2)
	execIntegration(t, db, `UPDATE residents SET user_id = $1 WHERE id = $2`, userID2, resID2)
	execIntegration(t, db, `INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES ($1, $2, $3, 'warga', true)`,
		uniqueIDIntegration(), userID2, rtID)

	// Create dues (obligatory, non-voluntary)
	dueID := createDueIntegration(t, db, rtID, false)

	// Create two bills for TWO different households/occupancies (two different users)
	bill1ID := createBillIntegration(t, db, rtID, occID, dueID, "2026-01", "100000", "unpaid", true, "2026-01-10")
	bill2ID := createBillIntegration(t, db, rtID, occID2, dueID, "2026-02", "200000", "unpaid", true, "2026-02-10")

	asOf := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)

	// Scenario 1: No payments → both bills (different users) should return recipients
	recs, err := repo.GetIuranReminderRecipients(ctx, rtID, asOf)
	if err != nil {
		t.Fatalf("GetIuranReminderRecipients (no payments): %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 recipients (2 unpaid bills, different users), got %d", len(recs))
	}

	// Scenario 2: Full payment on bill1 → bill1 should NOT appear, bill2 should
	fullPayID := uniqueIDIntegration()
	execIntegration(t, db, `INSERT INTO payments (id, rt_id, bill_id, amount, method, origin, status, created_at, updated_at) VALUES ($1, $2, $3, $4, 'TRANSFER', 'SELF_SUBMITTED', 'APPROVED', NOW(), NOW())`,
		fullPayID, rtID, bill1ID, "100000")
	execIntegration(t, db, `INSERT INTO payment_allocations (id, payment_id, bill_id, amount, created_at) VALUES ($1, $2, $3, $4, NOW())`,
		uniqueIDIntegration(), fullPayID, bill1ID, "100000")

	recs, err = repo.GetIuranReminderRecipients(ctx, rtID, asOf)
	if err != nil {
		t.Fatalf("GetIuranReminderRecipients after full payment: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recipient (bill2 only) after full payment on bill1, got %d", len(recs))
	}
	if recs[0].BillID != bill2ID {
		t.Fatalf("expected bill2 as remaining recipient, got %s", recs[0].BillID)
	}

	// Scenario 3: Partial payment on bill2 → bill2 still has remaining balance
	partialPayID := uniqueIDIntegration()
	execIntegration(t, db, `INSERT INTO payments (id, rt_id, bill_id, amount, method, origin, status, created_at, updated_at) VALUES ($1, $2, $3, $4, 'TRANSFER', 'SELF_SUBMITTED', 'APPROVED', NOW(), NOW())`,
		partialPayID, rtID, bill2ID, "50000")
	execIntegration(t, db, `INSERT INTO payment_allocations (id, payment_id, bill_id, amount, created_at) VALUES ($1, $2, $3, $4, NOW())`,
		uniqueIDIntegration(), partialPayID, bill2ID, "50000")

	recs, err = repo.GetIuranReminderRecipients(ctx, rtID, asOf)
	if err != nil {
		t.Fatalf("GetIuranReminderRecipients after partial payment: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recipient (bill2 with remaining 150000), got %d", len(recs))
	}
	if recs[0].BillID != bill2ID {
		t.Fatalf("expected bill2 as remaining recipient, got %s", recs[0].BillID)
	}

	// Scenario 4: PENDING payment should NOT reduce balance (only APPROVED counts)
	pendingPayID := uniqueIDIntegration()
	execIntegration(t, db, `INSERT INTO payments (id, rt_id, bill_id, amount, method, origin, status, created_at, updated_at) VALUES ($1, $2, $3, $4, 'TRANSFER', 'SELF_SUBMITTED', 'PENDING', NOW(), NOW())`,
		pendingPayID, rtID, bill2ID, "150000")
	execIntegration(t, db, `INSERT INTO payment_allocations (id, payment_id, bill_id, amount, created_at) VALUES ($1, $2, $3, $4, NOW())`,
		uniqueIDIntegration(), pendingPayID, bill2ID, "150000")

	recs, err = repo.GetIuranReminderRecipients(ctx, rtID, asOf)
	if err != nil {
		t.Fatalf("GetIuranReminderRecipients after pending payment: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recipient (pending payment does not clear balance), got %d", len(recs))
	}

	// Scenario 5: REJECTED payment should NOT reduce balance
	execIntegration(t, db, `UPDATE payments SET status = 'REJECTED' WHERE id = $1`, pendingPayID)

	// Scenario 6: Bill non-obligatory should NOT appear
	nonObligDueID := createDueIntegration(t, db, rtID, true)
	nonObligBillID := createBillIntegration(t, db, rtID, occID, nonObligDueID, "2026-03", "50000", "unpaid", false, "2026-03-10")

	recs, err = repo.GetIuranReminderRecipients(ctx, rtID, asOf)
	if err != nil {
		t.Fatalf("GetIuranReminderRecipients with non-obligatory bill: %v", err)
	}
	for _, r := range recs {
		if r.BillID == nonObligBillID {
			t.Fatalf("non-obligatory bill should not appear in recipients")
		}
	}

	// Scenario 7: New resident (move_in_date after bill due date) should NOT appear for past months
	newPhysID := createPhysicalHouseIntegration(t, db, rtID)
	newOccID := createOccupancyIntegration(t, db, newPhysID, hhID)
	newResID := createResidentIntegration(t, db, newOccID, rtID)
	newUserID := createUserIntegration(t, db, newResID)
	execIntegration(t, db, `UPDATE residents SET user_id = $1 WHERE id = $2`, newUserID, newResID)
	execIntegration(t, db, `INSERT INTO user_rt_memberships (id, user_id, rt_id, role, is_active) VALUES ($1, $2, $3, 'warga', true)`,
		uniqueIDIntegration(), newUserID, rtID)
	// New occupancy starts AFTER bill due dates → should not be billed for past periods
	execIntegration(t, db, `UPDATE household_occupancies SET start_date = '2026-03-01' WHERE id = $1`, newOccID)

	recs, err = repo.GetIuranReminderRecipients(ctx, rtID, asOf)
	if err != nil {
		t.Fatalf("GetIuranReminderRecipients with new resident: %v", err)
	}
	for _, r := range recs {
		if r.UserID == newUserID {
			t.Fatalf("new resident (move_in_date after bill due date) should not receive reminder for past bills")
		}
	}
}

// Helper functions

func createTestRTIntegration(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	uid := uniqueIDIntegration()
	err := db.QueryRowContext(context.Background(),
		"INSERT INTO rts (id, name, rw, rt, address, is_active) VALUES ($1, $2, $3, $4, '', true) RETURNING id",
		uid, "test-rt-"+uid[:8], 1, uid[:8],
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test RT: %v", err)
	}
	return id
}

func createPhysicalHouseIntegration(t *testing.T, db *sql.DB, rtID string) string {
	t.Helper()
	var id string
	hn := "H" + uniqueIDIntegration()[:6]
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO physical_houses (id, rt_id, house_number, address, is_active) VALUES ($1, $2, $3, '', true) RETURNING id`,
		uniqueIDIntegration(), rtID, hn,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create physical house: %v", err)
	}
	return id
}

func createHouseholdIntegration(t *testing.T, db *sql.DB, rtID string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO households (id, rt_id, head_name, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		uniqueIDIntegration(), rtID, "Test Head",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	return id
}

func createOccupancyIntegration(t *testing.T, db *sql.DB, physID string, hhID string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO household_occupancies (id, physical_house_id, household_id, occupancy_status, start_date, end_date) VALUES ($1, $2, $3, 'OWNER', '2026-01-01', NULL) RETURNING id`,
		uniqueIDIntegration(), physID, hhID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create occupancy: %v", err)
	}
	return id
}

func createResidentIntegration(t *testing.T, db *sql.DB, occID string, rtID string) string {
	t.Helper()
	var b [16]byte
	_, _ = rand.Read(b[:])
	for i := range b {
		b[i] = '0' + b[i]%10
	}
	nik := string(b[:])
	email := "test-" + uniqueIDIntegration()[:8] + "@example.com"
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO residents (id, rt_id, full_name, nik, email, is_active) VALUES ($1, $2, $3, $4, $5, true) RETURNING id`,
		uniqueIDIntegration(), rtID, "Test Resident", nik, email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create resident: %v", err)
	}
	execIntegration(t, db, `INSERT INTO residency_periods (id, household_occupancy_id, resident_id, start_date, end_date) VALUES ($1, $2, $3, '2026-01-01', NULL)`,
		uniqueIDIntegration(), occID, id)
	return id
}

func createUserIntegration(t *testing.T, db *sql.DB, resID string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, phone, password_hash, full_name, is_active) VALUES ($1, $2, $3, '', $4, true) RETURNING id`,
		uniqueIDIntegration(), "test-"+uniqueIDIntegration()[:8]+"@example.com", "", "Test User",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func createDueIntegration(t *testing.T, db *sql.DB, rtID string, voluntary bool) string {
	t.Helper()
	var id string
	voluntaryStr := "false"
	if voluntary {
		voluntaryStr = "true"
	}
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO dues (id, rt_id, name, amount, is_voluntary, is_active, period_type, due_type, year) VALUES ($1, $2, $3, $4, $5, true, 'monthly', 'regular', 2026) RETURNING id`,
		uniqueIDIntegration(), rtID, "Test Dues", "100000", voluntaryStr,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create dues: %v", err)
	}
	return id
}

func createBillIntegration(t *testing.T, db *sql.DB, rtID string, occID string, dueID string, period string, amount string, status string, obligatory bool, dueDate string) string {
	t.Helper()
	var id string
	oblStr := "false"
	if obligatory {
		oblStr = "true"
	}
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO bills (id, rt_id, household_occupancy_id, due_id, period, amount, status, is_obligatory, is_active, due_date, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, $9, NOW(), NOW()) RETURNING id`,
		uniqueIDIntegration(), rtID, occID, dueID, period, amount, status, oblStr, dueDate,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}
	return id
}

func execIntegration(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec: %v\nquery: %s", err, query)
	}
}

func uniqueIDIntegration() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
