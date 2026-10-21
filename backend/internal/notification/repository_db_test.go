package notification_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/database"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"
)

// These tests exercise the real PostgreSQL recipient queries behind
// notification delivery. They run against the isolated test database only
// (testutil.GetTestPool enforces the "social_finance_test" guard).
//
// Covered authorization policy for payment verifiers (cmd/api/main.go,
// POST /api/v1/payments/{id}/verify):
//   - RequirePermission("finance.approve"): jabatan holders are evaluated
//     against position_permissions; members without jabatan pass through.
//   - RequireAccess(roles=[bendahara, pengurus], jabatans=[ketua, bendahara]).

var dbRTCounter int64 = time.Now().UnixNano() % 100000

func notifTestPool(t *testing.T) *database.Pool {
	t.Helper()
	return testutil.GetTestPool(t)
}

// ensureFinanceApproveCatalog makes sure the canonical permission catalog
// rows required by the verifier policy exist. Idempotent; matches the
// canonical grants seeded by migration 031 (ketua, bendahara).
func ensureFinanceApproveCatalog(t *testing.T, pool *database.Pool) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO permission_types (code, domain, operation)
		VALUES ('finance.approve', 'finance', 'approve')
		ON CONFLICT (code) DO NOTHING
	`)
	if err != nil {
		t.Fatalf("ensure finance.approve permission type: %v", err)
	}

	for _, position := range []string{"ketua", "bendahara"} {
		_, err = db.ExecContext(ctx, `
			INSERT INTO position_permissions (position, permission_id)
			SELECT $1, id FROM permission_types WHERE code = 'finance.approve'
			ON CONFLICT (position, permission_id) DO NOTHING
		`, position)
		if err != nil {
			t.Fatalf("ensure %s finance.approve grant: %v", position, err)
		}
	}
}

func notifTestRT(t *testing.T, pool *database.Pool) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)
	var id string
	err := db.QueryRowContext(ctx,
		`INSERT INTO rts (name, rw, rt, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		fmt.Sprintf("Notif Repo Test RT %d", n), 9, fmt.Sprintf("NOTIF-%d", n),
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test RT: %v", err)
	}
	return id
}

func notifTestUser(t *testing.T, pool *database.Pool, label string, active bool) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)
	var id string
	err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'dummyhash', $3, $4) RETURNING id`,
		fmt.Sprintf("%d_%d_%s", time.Now().UnixNano(), n, label),
		fmt.Sprintf("+62811%08d", n), label, active,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return id
}

func notifTestMembership(t *testing.T, pool *database.Pool, userID, rtID, role string, jabatan *string, active bool) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, err := db.ExecContext(ctx,
		`INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, rtID, role, jabatan, active,
	)
	if err != nil {
		t.Fatalf("create test membership: %v", err)
	}
}

func notifTestDevice(t *testing.T, pool *database.Pool, userID, token string, active bool) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, err := db.ExecContext(ctx,
		`INSERT INTO user_devices (user_id, fcm_token, platform, is_active, last_seen_at, updated_at)
		 VALUES ($1, $2, 'android', $3, NOW(), NOW())`,
		userID, token, active,
	)
	if err != nil {
		t.Fatalf("create test device: %v", err)
	}
}

// notifTestOccupancyWithResident creates physical house → household →
// occupancy → resident (optionally linked to a user) → open residency period.
func notifTestOccupancyWithResident(t *testing.T, pool *database.Pool, rtID string, residentUserID *string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	var houseID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO physical_houses (rt_id, house_number, address) VALUES ($1, $2, 'Test Street') RETURNING id`,
		rtID, fmt.Sprintf("NR-%d", n),
	).Scan(&houseID); err != nil {
		t.Fatalf("create house: %v", err)
	}

	var hhID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO households (rt_id, head_name) VALUES ($1, 'Notif Test Head') RETURNING id`,
		rtID,
	).Scan(&hhID); err != nil {
		t.Fatalf("create household: %v", err)
	}

	var occID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date)
		 VALUES ($1, $2, 'OWNER', CURRENT_DATE) RETURNING id`,
		hhID, houseID,
	).Scan(&occID); err != nil {
		t.Fatalf("create occupancy: %v", err)
	}

	var resID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active, user_id)
		 VALUES ($1, 'Notif Test Resident', $2, true, $3) RETURNING id`,
		rtID, fmt.Sprintf("+62822%08d", n), residentUserID,
	).Scan(&resID); err != nil {
		t.Fatalf("create resident: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, CURRENT_DATE)`,
		resID, occID,
	); err != nil {
		t.Fatalf("create residency period: %v", err)
	}

	return occID
}

func notifTestBill(t *testing.T, pool *database.Pool, rtID, occID, period string) string {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	n := atomic.AddInt64(&dbRTCounter, 1)

	var dueID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO dues (rt_id, name, amount, period_type, is_active)
		 VALUES ($1, $2, 50000, 'monthly', true) RETURNING id`,
		rtID, fmt.Sprintf("Notif Due %d", n),
	).Scan(&dueID); err != nil {
		t.Fatalf("create due: %v", err)
	}

	var billID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
		 VALUES ($1, $2, $3, 50000, $4, CURRENT_DATE + INTERVAL '7 days', 'unpaid') RETURNING id`,
		rtID, occID, dueID, period,
	).Scan(&billID); err != nil {
		t.Fatalf("create bill: %v", err)
	}

	return billID
}

func cleanupNotifTestRT(t *testing.T, pool *database.Pool, rtID string) {
	t.Helper()
	db := pool.Raw()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM notification_logs WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM transactions WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM payments WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM bills WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM dues WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM financial_categories WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `
		DELETE FROM residency_periods WHERE household_occupancy_id IN (
			SELECT id FROM household_occupancies WHERE household_id IN (
				SELECT id FROM households WHERE rt_id = $1))`, rtID)
	_, _ = db.ExecContext(ctx, `
		DELETE FROM household_occupancies WHERE household_id IN (
			SELECT id FROM households WHERE rt_id = $1)`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM physical_houses WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM residents WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM households WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM user_rt_memberships WHERE rt_id = $1`, rtID)
	_, _ = db.ExecContext(ctx, `DELETE FROM rts WHERE id = $1`, rtID)
}

// --- GetTokensForPaymentVerifiers ---

// Reported regression case: role=pengurus, jabatan=bendahara with
// finance.approve must receive payment-submission notifications.
func TestGetTokensForPaymentVerifiers_PengurusWithBendaharaJabatan(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "pengurus-bendahara", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, userID, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-pengurus-bendahara", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "tok-pengurus-bendahara" {
		t.Fatalf("expected [tok-pengurus-bendahara], got %v", tokens)
	}
}

// Documented pattern from migration 031: Ketua RT has role=warga + jabatan=ketua.
func TestGetTokensForPaymentVerifiers_KetuaWithWargaRole(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "warga-ketua", true)
	jabatan := "ketua"
	notifTestMembership(t, pool, userID, rtID, "warga", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-warga-ketua", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "tok-warga-ketua" {
		t.Fatalf("expected [tok-warga-ketua], got %v", tokens)
	}
}

// Role-based verifier without jabatan passes RequirePermission by pass-through
// and RequireAccess by role → must be notified.
func TestGetTokensForPaymentVerifiers_PengurusWithoutJabatan(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "pengurus-no-jabatan", true)
	notifTestMembership(t, pool, userID, rtID, "pengurus", nil, true)
	notifTestDevice(t, pool, userID, "tok-pengurus-no-jabatan", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "tok-pengurus-no-jabatan" {
		t.Fatalf("expected [tok-pengurus-no-jabatan], got %v", tokens)
	}
}

// Ordinary warga without jabatan cannot verify → must NOT be notified.
func TestGetTokensForPaymentVerifiers_WargaExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "warga-no-jabatan", true)
	notifTestMembership(t, pool, userID, rtID, "warga", nil, true)
	notifTestDevice(t, pool, userID, "tok-warga", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens for warga, got %v", tokens)
	}
}

// A pengurus whose jabatan lacks finance.approve (e.g. anggota) is blocked by
// RequirePermission on the verify route → must NOT be notified.
func TestGetTokensForPaymentVerifiers_JabatanWithoutFinanceApproveExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "pengurus-anggota", true)
	jabatan := "anggota"
	notifTestMembership(t, pool, userID, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-pengurus-anggota", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens for jabatan without finance.approve, got %v", tokens)
	}
}

// An authorized verifier in another RT must not receive the notification.
func TestGetTokensForPaymentVerifiers_OtherRTExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	targetRT := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, targetRT)
	otherRT := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, otherRT)

	otherUserID := notifTestUser(t, pool, "other-rt-verifier", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, otherUserID, otherRT, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, otherUserID, "tok-other-rt", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, targetRT)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens from other RT, got %v", tokens)
	}
}

// Inactive users, inactive memberships, and inactive devices are excluded.
func TestGetTokensForPaymentVerifiers_InactiveRowsExcluded(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	jabatan := "bendahara"

	// Inactive user with active membership and device.
	inactiveUser := notifTestUser(t, pool, "inactive-user", false)
	notifTestMembership(t, pool, inactiveUser, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, inactiveUser, "tok-inactive-user", true)

	// Active user with inactive membership.
	noMembershipUser := notifTestUser(t, pool, "inactive-membership", true)
	notifTestMembership(t, pool, noMembershipUser, rtID, "pengurus", &jabatan, false)
	notifTestDevice(t, pool, noMembershipUser, "tok-inactive-membership", true)

	// Active user and membership but inactive device.
	noDeviceUser := notifTestUser(t, pool, "inactive-device", true)
	notifTestMembership(t, pool, noDeviceUser, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, noDeviceUser, "tok-inactive-device", false)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no tokens (all inactive), got %v", tokens)
	}
}

// A member matching multiple recipient conditions (role=pengurus AND
// jabatan=bendahara) with several devices must yield each token exactly once.
func TestGetTokensForPaymentVerifiers_NoDuplicateTokensForMultipleMatches(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)

	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "multi-match", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, userID, rtID, "pengurus", &jabatan, true)
	notifTestDevice(t, pool, userID, "tok-multi-a", true)
	notifTestDevice(t, pool, userID, "tok-multi-b", true)

	tokens, err := repo.GetTokensForPaymentVerifiers(ctx, rtID)
	if err != nil {
		t.Fatalf("GetTokensForPaymentVerifiers: %v", err)
	}
	counts := make(map[string]int)
	for _, tok := range tokens {
		counts[tok]++
	}
	if len(tokens) != 2 {
		t.Fatalf("expected exactly 2 unique tokens, got %d: %v", len(tokens), tokens)
	}
	if counts["tok-multi-a"] != 1 || counts["tok-multi-b"] != 1 {
		t.Fatalf("expected each token exactly once, got %v", counts)
	}
}

// --- GetRecipientUserIDsForBill ---

func TestGetRecipientUserIDsForBill_IncludesOrdinaryResident(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "resident-ordinary", true)
	notifTestMembership(t, pool, userID, rtID, "warga", nil, true)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &userID)
	billID := notifTestBill(t, pool, rtID, occID, "2027-01")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 1 || ids[0] != userID {
		t.Fatalf("expected [%s], got %v", userID, ids)
	}
}

// A resident who also holds a management role (pengurus) must keep receiving
// resident-addressed notifications for their own household.
func TestGetRecipientUserIDsForBill_IncludesPengurusResident(t *testing.T) {
	pool := notifTestPool(t)
	ensureFinanceApproveCatalog(t, pool)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	officerID := notifTestUser(t, pool, "resident-pengurus", true)
	jabatan := "bendahara"
	notifTestMembership(t, pool, officerID, rtID, "pengurus", &jabatan, true)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &officerID)
	billID := notifTestBill(t, pool, rtID, occID, "2027-02")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 1 || ids[0] != officerID {
		t.Fatalf("expected pengurus resident [%s] to receive bill notification, got %v", officerID, ids)
	}
}

func TestGetRecipientUserIDsForBill_ExcludesInactiveUser(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	inactiveID := notifTestUser(t, pool, "resident-inactive", false)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &inactiveID)
	billID := notifTestBill(t, pool, rtID, occID, "2027-03")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no recipients for inactive user, got %v", ids)
	}
}

// One person matching multiple recipient conditions (same user resolved via
// user_id and via email fallback) must appear only once.
func TestGetRecipientUserIDsForBill_NoDuplicateRecipients(t *testing.T) {
	pool := notifTestPool(t)
	repo := notification.NewRepository(pool)
	ctx := context.Background()

	rtID := notifTestRT(t, pool)
	defer cleanupNotifTestRT(t, pool, rtID)

	userID := notifTestUser(t, pool, "resident-dup", true)
	occID := notifTestOccupancyWithResident(t, pool, rtID, &userID)

	// Second resident row for the SAME user (email fallback path would also
	// resolve to this user); the unique user_id index prevents duplicate
	// user_id rows, so simulate the multi-condition match via an email-only
	// resident whose email equals the user's email.
	db := pool.Raw()
	var email string
	if err := db.QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
		t.Fatalf("load user email: %v", err)
	}
	var resID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO residents (rt_id, full_name, phone, is_active, email)
		 VALUES ($1, 'Email Fallback Resident', '+628999999999', true, $2) RETURNING id`,
		rtID, email,
	).Scan(&resID); err != nil {
		t.Fatalf("create email-fallback resident: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO residency_periods (resident_id, household_occupancy_id, start_date)
		 VALUES ($1, $2, CURRENT_DATE)`,
		resID, occID,
	); err != nil {
		t.Fatalf("create email-fallback residency period: %v", err)
	}

	billID := notifTestBill(t, pool, rtID, occID, "2027-04")

	ids, err := repo.GetRecipientUserIDsForBill(ctx, rtID, billID)
	if err != nil {
		t.Fatalf("GetRecipientUserIDsForBill: %v", err)
	}
	if len(ids) != 1 || ids[0] != userID {
		t.Fatalf("expected exactly one recipient [%s], got %v", userID, ids)
	}
}
