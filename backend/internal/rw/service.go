package rw

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/finance"
	"social-finance/internal/notification"
	"social-finance/internal/storage"
)

// RWNotifier defines the notification operations for the RW domain.
type RWNotifier interface {
	NotifyRWCashIn(ctx context.Context, rwID, txID, amount, description, locationName string)
	NotifyRWCashOut(ctx context.Context, rwID, txID, amount, description, category, locationName string)
	SendRWManualPush(ctx context.Context, rwID, senderUserID, title, body string, target ...string) (int, error)
	ListRWLogs(ctx context.Context, rwID, eventType string, page, pageSize int) ([]notification.NotificationLogEntry, int, error)
}

// Service provides RW-scoped operations with authorization enforcement.
// Every public method validates that the user has active RW membership
// matching the requested rwID before executing any query.
type Service struct {
	pool       *database.Pool
	repo       *Repository
	scopingCfg *auth.ScopingConfig
	notifier   RWNotifier
}

func NewService(pool *database.Pool, repo *Repository, scopingCfg *auth.ScopingConfig) *Service {
	return &Service{
		pool:       pool,
		repo:       repo,
		scopingCfg: scopingCfg,
	}
}

// SetNotifier sets the RWNotifier instance.
func (s *Service) SetNotifier(n RWNotifier) {
	s.notifier = n
}

// GetNotifier returns the RWNotifier instance.
func (s *Service) GetNotifier() RWNotifier {
	return s.notifier
}

// checkRWAccess verifies the AuthContext has authorization to access the given rwID.
func (s *Service) checkRWAccess(ctx context.Context, ac *auth.AuthContext, rwID string) error {
	if ac == nil {
		return fmt.Errorf("no authentication context")
	}
	// Super Admin: validate scope and active status via shared validator.
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return auth.ValidateSAForRW(ctx, s.scopingCfg, s.pool, rwID)
	}
	for _, rm := range ac.RWMemberships {
		if rm.RWID == rwID {
			return nil
		}
	}
	return fmt.Errorf("%w: rw %s", ErrRWAccessDenied, rwID)
}

// checkRWPermission verifies the AuthContext has the required permission code
// based on their RW membership jabatan and source.
//
// The source distinguishes between:
//   - explicit membership (RW officials) — full permissions per rwPermissionMap.
//   - inferred membership (RT officials) — read-only access only.
func (s *Service) checkRWPermission(ac *auth.AuthContext, rwID string, requiredPermission string) error {
	if ac == nil {
		return fmt.Errorf("no authentication context")
	}
	// Super Admin bypass: validated by RequireRWScope middleware (scope + active DB check).
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return nil
	}
	for _, rm := range ac.RWMemberships {
		if rm.RWID == rwID {
			// Pass the membership source so the permission layer can
			// distinguish RT officials (rt_inferred → read-only) from
			// RW officials (explicit → full permissions).
			if auth.HasRWScopePermission(rm.Jabatan, rm.Source, requiredPermission) {
				return nil
			}
			return fmt.Errorf("%w: required %q for jabatan %q (source: %s)",
				ErrPermissionDenied, requiredPermission, rm.Jabatan, rm.Source)
		}
	}
	return fmt.Errorf("%w: no RW membership for %s", ErrRWAccessDenied, rwID)
}

// requirePermission wraps access and permission checks.
func (s *Service) requirePermission(ctx context.Context, ac *auth.AuthContext, rwID string, permissionCode string) error {
	if err := s.checkRWAccess(ctx, ac, rwID); err != nil {
		return err
	}
	return s.checkRWPermission(ac, rwID, permissionCode)
}

// RequirePermission checks whether the AuthContext has the required permission code for the given rwID.
func (s *Service) RequirePermission(ctx context.Context, ac *auth.AuthContext, rwID string, permissionCode string) error {
	return s.requirePermission(ctx, ac, rwID, permissionCode)
}

// =========================================================================
// Dashboard & KAS Reports
// =========================================================================

// GetDashboard returns an aggregated financial overview for the RW.
func (s *Service) GetDashboard(ctx context.Context, ac *auth.AuthContext, rwID string) (*DashboardSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.dashboard.read"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	summary, err := s.repo.GetDashboardSummary(ctx, tx, rwID)
	if err != nil {
		return nil, fmt.Errorf("dashboard summary: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return summary, nil
}

// GetKASReport returns the KAS report (opening/income/expense) for the RW.
func (s *Service) GetKASReport(ctx context.Context, ac *auth.AuthContext, rwID, period string) (*KASReport, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.read"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	report, err := s.repo.GetKASReport(ctx, tx, rwID, period)
	if err != nil {
		return nil, fmt.Errorf("kas report: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return report, nil
}

// GetKASReportDetail returns the period-consistent RW kas report with a
// carry-forward opening balance (RW opening balance + net posted movements
// strictly before the window start). Window resolution:
//   - no period and no dates -> cumulative all-time balance,
//   - period (YYYY-MM) -> [first of month, first of next month),
//   - start_date and end_date (YYYY-MM-DD, both required together, start <= end)
//     -> [start 00:00, end+1day 00:00) Asia/Jakarta, i.e. both endpoints
//     denote whole days and are fully included. This deliberately differs
//     from the RT /reports/balance half-open quirk (which excludes the end
//     day and transactions on it) and is consistent with the RW transactions
//     endpoint, which already filters whole days via DATE().
//   - period combined with dates is rejected.
func (s *Service) GetKASReportDetail(ctx context.Context, ac *auth.AuthContext, rwID, period, startDate, endDate string) (*KASReportResponse, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.read"); err != nil {
		return nil, err
	}

	period = strings.TrimSpace(period)
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)

	if period == "" && startDate == "" && endDate == "" {
		return s.computeKASReportDetail(ctx, ac, rwID, "", "", "", nil, nil)
	}
	if period != "" && (startDate != "" || endDate != "") {
		return nil, fmt.Errorf("%w: cannot combine period with start_date or end_date", ErrValidation)
	}
	if (startDate != "") != (endDate != "") {
		return nil, fmt.Errorf("%w: start_date and end_date must be provided together", ErrValidation)
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil || loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	if period != "" {
		if _, err := ValidatePeriod(period); err != nil {
			return nil, fmt.Errorf("%w: invalid period format, expected YYYY-MM", ErrValidation)
		}
		t, err := time.ParseInLocation("2006-01", period, loc)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid period format, expected YYYY-MM", ErrValidation)
		}
		st := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		et := st.AddDate(0, 1, 0)
		return s.computeKASReportDetail(ctx, ac, rwID, period, "", "", &st, &et)
	}

	if _, err := ValidateDate(startDate); err != nil {
		return nil, fmt.Errorf("%w: invalid start_date format, expected YYYY-MM-DD", ErrValidation)
	}
	if _, err := ValidateDate(endDate); err != nil {
		return nil, fmt.Errorf("%w: invalid end_date format, expected YYYY-MM-DD", ErrValidation)
	}
	st, err := time.ParseInLocation("2006-01-02", startDate, loc)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid start_date format, expected YYYY-MM-DD", ErrValidation)
	}
	et, err := time.ParseInLocation("2006-01-02", endDate, loc)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid end_date format, expected YYYY-MM-DD", ErrValidation)
	}
	if st.After(et) {
		return nil, fmt.Errorf("%w: start_date must not be after end_date", ErrValidation)
	}
	// Whole-day inclusive window: [start 00:00, end+1day 00:00). A single-day
	// range (start == end) covers that whole day.
	et = et.AddDate(0, 0, 1)
	return s.computeKASReportDetail(ctx, ac, rwID, "", startDate, endDate, &st, &et)
}

func (s *Service) computeKASReportDetail(ctx context.Context, ac *auth.AuthContext, rwID, period, startDate, endDate string, startBound, endBound *time.Time) (*KASReportResponse, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	report, err := s.repo.GetKASReportCarry(ctx, tx, rwID, startBound, endBound)
	if err != nil {
		return nil, fmt.Errorf("kas report: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &KASReportResponse{KASReport: report, Period: period, StartDate: startDate, EndDate: endDate}, nil
}

// SetOpeningBalance sets or updates the RW opening balance.
func (s *Service) SetOpeningBalance(ctx context.Context, ac *auth.AuthContext, rwID, amount string) error {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.write"); err != nil {
		return err
	}
	cents, _, err := ParseMoney(amount)
	if err != nil || cents < 0 {
		return fmt.Errorf("invalid opening balance: %w", ErrValidation)
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.repo.SetOpeningBalance(ctx, tx, rwID, FormatMoney(cents), ac.UserID); err != nil {
		return err
	}

	return tx.Commit()
}

// AllocateOpeningBalance allocates opening balance from unallocated cash (U)
// to a specific RW fund location. It does not create a Kas Masuk transaction
// and does not change the total Kas RW (K).
func (s *Service) AllocateOpeningBalance(ctx context.Context, ac *auth.AuthContext, rwID string, in AllocateOpeningBalanceInput) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.write"); err != nil {
		return nil, err
	}

	amtCents, _, err := ParseMoney(in.Amount)
	if err != nil || amtCents <= 0 {
		return nil, fmt.Errorf("invalid amount: %w", ErrInvalidAmount)
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	loc, err := s.repo.AllocateOpeningBalance(ctx, tx, rwID, in.LocationID, amtCents, in.Notes, ac.UserID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return loc, nil
}

// GetTransactions returns a paginated list of posted transactions from RW's independent ledger.
func (s *Service) GetTransactions(ctx context.Context, ac *auth.AuthContext, rwID string,
	period string, page, pageSize int, startDate, endDate string, search string,
	opts ...TransactionFilter,
) ([]*TransactionEntry, int, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.read"); err != nil {
		return nil, 0, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	list, total, err := s.repo.GetTransactions(ctx, tx, rwID, period, page, pageSize, startDate, endDate, search, opts...)
	if err != nil {
		return nil, 0, fmt.Errorf("get transactions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit transaction: %w", err)
	}

	if list == nil {
		list = []*TransactionEntry{}
	}
	return list, total, nil
}

// GetTransactionByID returns a single transaction by ID.
func (s *Service) GetTransactionByID(ctx context.Context, ac *auth.AuthContext, rwID, id string) (*TransactionEntry, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.read"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	entry, err := s.repo.GetTransactionByID(ctx, tx, rwID, id)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return entry, nil
}

// CreateTransaction creates a new KAS Masuk or KAS Keluar transaction linked to an RW fund location.
func (s *Service) CreateTransaction(ctx context.Context, ac *auth.AuthContext, rwID string, in CreateTransactionInput) (*TransactionEntry, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.write"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	entry, err := s.repo.CreateTransaction(ctx, tx, rwID, ac.UserID, in)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	if s.notifier != nil {
		if entry.Type == "income" {
			locName := ""
			if entry.LocationName != nil {
				locName = *entry.LocationName
			}
			s.notifier.NotifyRWCashIn(ctx, rwID, entry.ID, entry.Amount, entry.Description, locName)
		} else if entry.Type == "expense" {
			locName := ""
			if entry.LocationName != nil {
				locName = *entry.LocationName
			}
			cat := ""
			if entry.ExpenseCategory != nil {
				cat = *entry.ExpenseCategory
			}
			s.notifier.NotifyRWCashOut(ctx, rwID, entry.ID, entry.Amount, entry.Description, cat, locName)
		}
	}

	return entry, nil
}

// SendManualPush broadcasts a manual push notification to RW officials and RT officials under the RW.
func (s *Service) SendManualPush(ctx context.Context, ac *auth.AuthContext, rwID, title, body string, target ...string) (int, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.notification.write"); err != nil {
		return 0, err
	}
	if strings.TrimSpace(title) == "" {
		return 0, errors.New("judul notifikasi wajib diisi")
	}
	if strings.TrimSpace(body) == "" {
		return 0, errors.New("isi pesan notifikasi wajib diisi")
	}
	if s.notifier == nil {
		return 0, nil
	}
	return s.notifier.SendRWManualPush(ctx, rwID, ac.UserID, title, body, target...)
}

// ListNotifications returns notification logs for the RW.
func (s *Service) ListNotifications(ctx context.Context, ac *auth.AuthContext, rwID, eventType string, page, pageSize int) ([]notification.NotificationLogEntry, int, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.notification.read"); err != nil {
		return nil, 0, err
	}
	if s.notifier == nil {
		return nil, 0, nil
	}
	return s.notifier.ListRWLogs(ctx, rwID, eventType, page, pageSize)
}

// AttachTransactionProof saves the uploaded proof metadata for an RW transaction.
func (s *Service) AttachTransactionProof(ctx context.Context, ac *auth.AuthContext, rwID, txID string, meta *storage.ObjectMetadata) error {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.write"); err != nil {
		return err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.repo.AttachTransactionProof(ctx, tx, rwID, txID, ac.UserID, meta); err != nil {
		return err
	}

	return tx.Commit()
}

// GetTransactionProofInfo returns the proof path and mime type for an RW transaction.
func (s *Service) GetTransactionProofInfo(ctx context.Context, ac *auth.AuthContext, rwID, txID string) (proofPath, mimeType string, err error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.kas.read"); err != nil {
		return "", "", err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return "", "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	proofPath, mimeType, err = s.repo.GetTransactionProofInfo(ctx, tx, rwID, txID)
	if err != nil {
		return "", "", err
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("commit transaction: %w", err)
	}

	return proofPath, mimeType, nil
}

// =========================================================================
// RW Fund Locations
// =========================================================================

// GetLocations returns all RW fund locations with current balances.
func (s *Service) GetLocations(ctx context.Context, ac *auth.AuthContext, rwID string, isActive *bool) ([]*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.read"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	locations, err := s.repo.GetLocations(ctx, tx, rwID, isActive)
	if err != nil {
		return nil, fmt.Errorf("get locations: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	if locations == nil {
		locations = []*LocationSummary{}
	}
	return locations, nil
}

// GetLocationByID returns details of a single RW fund location.
func (s *Service) GetLocationByID(ctx context.Context, ac *auth.AuthContext, rwID, locID string) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.read"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	loc, err := s.repo.GetLocationByID(ctx, tx, rwID, locID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return loc, nil
}

// CreateLocation creates a new RW fund location (requires rw.fund_location.write).
func (s *Service) CreateLocation(ctx context.Context, ac *auth.AuthContext, rwID string, in CreateRWFundLocationInput) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.write"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	loc, err := s.repo.CreateLocation(ctx, tx, rwID, in)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return loc, nil
}

// UpdateLocation updates an existing RW fund location (requires rw.fund_location.write).
func (s *Service) UpdateLocation(ctx context.Context, ac *auth.AuthContext, rwID, locID string, in UpdateRWFundLocationInput) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.write"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	loc, err := s.repo.UpdateLocation(ctx, tx, rwID, locID, in)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return loc, nil
}

// ToggleLocationStatus activates or deactivates an RW fund location (requires rw.fund_location.write).
func (s *Service) ToggleLocationStatus(ctx context.Context, ac *auth.AuthContext, rwID, locID string, isActive bool) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.write"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	loc, err := s.repo.ToggleLocationStatus(ctx, tx, rwID, locID, isActive)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return loc, nil
}

// GetLocationMutations returns paginated balance changes for a location.
func (s *Service) GetLocationMutations(ctx context.Context, ac *auth.AuthContext, rwID, locID string, page, pageSize int) ([]LocationBalanceChange, int, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.read"); err != nil {
		return nil, 0, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	mutations, total, err := s.repo.GetLocationMutations(ctx, tx, rwID, locID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit transaction: %w", err)
	}

	return mutations, total, nil
}

// GetReconciliation returns reconciliation report between KAS and fund locations.
func (s *Service) GetReconciliation(ctx context.Context, ac *auth.AuthContext, rwID string) (*ReconciliationReport, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.read"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	rep, err := s.repo.GetReconciliationReport(ctx, tx, rwID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return rep, nil
}

// =========================================================================
// RW Fund Transfers
// =========================================================================

// CreateTransfer executes a transfer between two locations within the same RW (requires rw.fund_location.write).
func (s *Service) CreateTransfer(ctx context.Context, ac *auth.AuthContext, rwID string, in CreateFundTransferInput) (*FundTransfer, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.write"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	tr, err := s.repo.CreateTransfer(ctx, tx, rwID, ac.UserID, in)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return tr, nil
}

// ReverseTransfer reverses a completed transfer within the same RW (requires rw.fund_location.write).
func (s *Service) ReverseTransfer(ctx context.Context, ac *auth.AuthContext, rwID, transferID, reason string) (*FundTransfer, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.write"); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	rev, err := s.repo.ReverseTransfer(ctx, tx, rwID, ac.UserID, transferID, reason)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return rev, nil
}

// GetTransfers returns transfer history for an RW.
func (s *Service) GetTransfers(ctx context.Context, ac *auth.AuthContext, rwID string, page, pageSize int) ([]FundTransfer, int, error) {
	if err := s.requirePermission(ctx, ac, rwID, "rw.fund_location.read"); err != nil {
		return nil, 0, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	transfers, total, err := s.repo.GetTransfers(ctx, tx, rwID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit transaction: %w", err)
	}

	if transfers == nil {
		transfers = []FundTransfer{}
	}
	return transfers, total, nil
}

// ---- Money utility functions (delegates to finance package) ----

// ParseMoney parses a money string into cents and a canonical representation.
// It delegates to the finance package for consistent money handling across modules.
func ParseMoney(s string) (int64, string, error) {
	return finance.ParseMoney(s)
}

// FormatMoney formats cents into a human-readable money string.
// It delegates to the finance package for consistent money handling across modules.
func FormatMoney(cents int64) string {
	return finance.FormatMoney(cents)
}