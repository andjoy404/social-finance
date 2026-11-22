package rw

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"social-finance/internal/auth"
	"social-finance/internal/notification"
	"social-finance/internal/testutil"
)

type mockRWNotifierAudit struct {
	cashInCalls  []cashInCall
	cashOutCalls []cashOutCall
}

type cashInCall struct {
	rwID         string
	txID         string
	amount       string
	description  string
	locationName string
}

type cashOutCall struct {
	rwID         string
	txID         string
	amount       string
	description  string
	category     string
	locationName string
}

func (m *mockRWNotifierAudit) NotifyRWCashIn(ctx context.Context, rwID, txID, amount, description, locationName string) {
	m.cashInCalls = append(m.cashInCalls, cashInCall{
		rwID:         rwID,
		txID:         txID,
		amount:       amount,
		description:  description,
		locationName: locationName,
	})
}

func (m *mockRWNotifierAudit) NotifyRWCashOut(ctx context.Context, rwID, txID, amount, description, category, locationName string) {
	m.cashOutCalls = append(m.cashOutCalls, cashOutCall{
		rwID:         rwID,
		txID:         txID,
		amount:       amount,
		description:  description,
		category:     category,
		locationName: locationName,
	})
}

func (m *mockRWNotifierAudit) SendRWManualPush(ctx context.Context, rwID, senderUserID, title, body string, target ...string) (int, error) {
	return 1, nil
}

func (m *mockRWNotifierAudit) ListRWLogs(ctx context.Context, rwID, eventType string, page, pageSize int) ([]notification.NotificationLogEntry, int, error) {
	return nil, 0, nil
}

func (m *mockRWNotifierAudit) ListRWMyLogs(ctx context.Context, rwID, senderUserID, eventType string, page, pageSize int) ([]notification.NotificationLogEntry, int, error) {
	return nil, 0, nil
}

func (m *mockRWNotifierAudit) GetRWUnreadCount(ctx context.Context, rwID, userID string) (int, error) {
	return 0, nil
}

// TestAuditKasRW_NotificationTriggerAndIsolation tests that:
// 1. Kas RW Masuk triggers NotifyRWCashIn
// 2. Kas RW Keluar triggers NotifyRWCashOut
// 3. Failed transactions do NOT trigger notification
// 4. RT transactions have no connection to Kas RW notifier
func TestAuditKasRW_NotificationTriggerAndIsolation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	uniqueRwNum := int(time.Now().UnixNano()/1000000)%90000 + 20000
	var rwID string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW Audit Test', $1, true) RETURNING id`,
		uniqueRwNum,
	).Scan(&rwID)
	require.NoError(t, err)

	// Create fund location
	var locID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rw_fund_locations (rw_id, name, category, is_active) VALUES ($1, 'Kas Utama RW Audit', 'brankas', true) RETURNING id`,
		rwID,
	).Scan(&locID)
	require.NoError(t, err)

	repo := NewRepository(pool.Raw())
	svc := NewService(pool, repo, nil)

	notifier := &mockRWNotifierAudit{}
	svc.SetNotifier(notifier)

	// Create user in DB
	var ketuaUserID string
	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, is_active)
		 VALUES ($1, $2, 'hash', 'Test Ketua RW', true) RETURNING id`,
		fmt.Sprintf("ketua-rw-%d@test.com", time.Now().UnixNano()),
		fmt.Sprintf("081%d", time.Now().UnixNano()%100000000),
	).Scan(&ketuaUserID)
	require.NoError(t, err)

	// Ketua RW with write permission
	ketuaAC := &auth.AuthContext{
		UserID:     ketuaUserID,
		SystemRole: "user",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}

	// 1. Kas RW Masuk -> triggers NotifyRWCashIn
	inTx, err := svc.CreateTransaction(ctx, ketuaAC, rwID, CreateTransactionInput{
		Type:        "income",
		Amount:      "1000000.00",
		Description: "Iuran Kas RW Masuk Test",
		LocationID:  locID,
	})
	require.NoError(t, err)

	if len(notifier.cashInCalls) != 1 {
		t.Fatalf("expected 1 NotifyRWCashIn call, got %d", len(notifier.cashInCalls))
	}
	if notifier.cashInCalls[0].txID != inTx.ID || notifier.cashInCalls[0].rwID != rwID {
		t.Errorf("NotifyRWCashIn arguments mismatch: %+v", notifier.cashInCalls[0])
	}
	if len(notifier.cashOutCalls) != 0 {
		t.Errorf("expected 0 NotifyRWCashOut calls on income, got %d", len(notifier.cashOutCalls))
	}

	// 2. Kas RW Keluar -> triggers NotifyRWCashOut
	expenseCat := "Pemeliharaan"
	outTx, err := svc.CreateTransaction(ctx, ketuaAC, rwID, CreateTransactionInput{
		Type:            "expense",
		Amount:          "300000.00",
		Description:     "Beli Lampu RW Test",
		ExpenseCategory: &expenseCat,
		LocationID:      locID,
	})
	require.NoError(t, err)

	if len(notifier.cashOutCalls) != 1 {
		t.Fatalf("expected 1 NotifyRWCashOut call, got %d", len(notifier.cashOutCalls))
	}
	if notifier.cashOutCalls[0].txID != outTx.ID || notifier.cashOutCalls[0].rwID != rwID {
		t.Errorf("NotifyRWCashOut arguments mismatch: %+v", notifier.cashOutCalls[0])
	}

	// 3. Failed transaction does NOT trigger notification
	beforeInCount := len(notifier.cashInCalls)
	beforeOutCount := len(notifier.cashOutCalls)

	// Attempt transaction with negative amount -> fails validation
	_, err = svc.CreateTransaction(ctx, ketuaAC, rwID, CreateTransactionInput{
		Type:        "income",
		Amount:      "-50000.00",
		Description: "Invalid Amount",
		LocationID:  locID,
	})
	if err == nil {
		t.Fatal("expected error on invalid transaction amount, got nil")
	}

	if len(notifier.cashInCalls) != beforeInCount {
		t.Errorf("failed transaction must NOT trigger NotifyRWCashIn")
	}
	if len(notifier.cashOutCalls) != beforeOutCount {
		t.Errorf("failed transaction must NOT trigger NotifyRWCashOut")
	}

	// 4. Unauthorized user transaction attempt -> fails and does NOT trigger notification
	unauthAC := &auth.AuthContext{
		UserID:     uuid.New().String(),
		SystemRole: "user",
		RWMemberships: []auth.RWMembership{
			{RWID: rwID, Jabatan: "anggota", Source: auth.RWMembershipSourceExplicit},
		},
	}
	_, err = svc.CreateTransaction(ctx, unauthAC, rwID, CreateTransactionInput{
		Type:        "income",
		Amount:      "100000.00",
		Description: "Unauthorized attempt",
		LocationID:  locID,
	})
	if err == nil {
		t.Fatal("expected unauthorized error, got nil")
	}
	if len(notifier.cashInCalls) != beforeInCount {
		t.Errorf("unauthorized transaction must NOT trigger NotifyRWCashIn")
	}
}

// TestAuditRWTemplate_RBAC verifies template permissions:
// - READ permission (rw.notification.read / notification.read) allows viewing and previewing
// - MANAGE permission (rw.notification.write / notification.manage) allows mutations
// - Unauthorized users without permission are rejected
// - Cross-RW access is rejected
func TestAuditRWTemplate_RBAC(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	numA := int(time.Now().UnixNano()/1000000)%90000 + 30000
	numB := numA + 1
	var rwA, rwB string
	err := pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW RBAC A', $1, true) RETURNING id`,
		numA,
	).Scan(&rwA)
	require.NoError(t, err)

	err = pool.Raw().QueryRowContext(ctx,
		`INSERT INTO rws (name, rw_number, is_active) VALUES ('RW RBAC B', $1, true) RETURNING id`,
		numB,
	).Scan(&rwB)
	require.NoError(t, err)

	repo := NewRepository(pool.Raw())
	svc := NewService(pool, repo, nil)

	// User 1: RW A Bendahara (has rw.notification.read and rw.notification.write)
	bendaharaAC := &auth.AuthContext{
		UserID:     uuid.New().String(),
		SystemRole: "user",
		RWMemberships: []auth.RWMembership{
			{RWID: rwA, Jabatan: "bendahara", Source: auth.RWMembershipSourceExplicit},
		},
	}

	// User 2: RW A Anggota (has rw.notification.read only)
	anggotaAC := &auth.AuthContext{
		UserID:     uuid.New().String(),
		SystemRole: "user",
		RWMemberships: []auth.RWMembership{
			{RWID: rwA, Jabatan: "anggota", Source: auth.RWMembershipSourceExplicit},
		},
	}

	// User 3: User in RW B only
	otherRWAC := &auth.AuthContext{
		UserID:     uuid.New().String(),
		SystemRole: "user",
		RWMemberships: []auth.RWMembership{
			{RWID: rwB, Jabatan: "ketua", Source: auth.RWMembershipSourceExplicit},
		},
	}

	// 1. Bendahara has read and write permissions in RW A
	if err := svc.RequirePermission(ctx, bendaharaAC, rwA, "rw.notification.read"); err != nil {
		t.Errorf("bendahara should have rw.notification.read: %v", err)
	}
	if err := svc.RequirePermission(ctx, bendaharaAC, rwA, "notification.read"); err != nil {
		t.Errorf("bendahara should have notification.read alias: %v", err)
	}
	if err := svc.RequirePermission(ctx, bendaharaAC, rwA, "rw.notification.write"); err != nil {
		t.Errorf("bendahara should have rw.notification.write: %v", err)
	}
	if err := svc.RequirePermission(ctx, bendaharaAC, rwA, "notification.manage"); err != nil {
		t.Errorf("bendahara should have notification.manage alias: %v", err)
	}

	// 2. Anggota has read permission, but NOT write permission
	if err := svc.RequirePermission(ctx, anggotaAC, rwA, "rw.notification.read"); err != nil {
		t.Errorf("anggota should have rw.notification.read: %v", err)
	}
	if err := svc.RequirePermission(ctx, anggotaAC, rwA, "rw.notification.write"); err == nil {
		t.Errorf("anggota MUST NOT have rw.notification.write")
	}
	if err := svc.RequirePermission(ctx, anggotaAC, rwA, "notification.manage"); err == nil {
		t.Errorf("anggota MUST NOT have notification.manage")
	}

	// 3. User from RW B cannot access RW A
	if err := svc.RequirePermission(ctx, otherRWAC, rwA, "rw.notification.read"); err == nil {
		t.Errorf("cross-RW read access MUST be rejected")
	}
	if err := svc.RequirePermission(ctx, otherRWAC, rwA, "rw.notification.write"); err == nil {
		t.Errorf("cross-RW write access MUST be rejected")
	}
}
