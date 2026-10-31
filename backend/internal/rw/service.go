package rw

import (
	"context"
	"fmt"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/finance"
)

// Service provides RW-scoped operations with authorization enforcement.
// Every public method validates that the user has active RW membership
// matching the requested rwID before executing any query.
type Service struct {
	pool       *database.Pool
	repo       *Repository
	scopingCfg *auth.ScopingConfig
}

func NewService(pool *database.Pool, repo *Repository, scopingCfg *auth.ScopingConfig) *Service {
	return &Service{
		pool:       pool,
		repo:       repo,
		scopingCfg: scopingCfg,
	}
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
// based on their RW membership jabatan.
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
			if auth.HasRWScopePermission(rm.Jabatan, requiredPermission) {
				return nil
			}
			return fmt.Errorf("%w: required %q for jabatan %q", ErrPermissionDenied, requiredPermission, rm.Jabatan)
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

// =========================================================================
// Dashboard & KAS Reports
// =========================================================================

// GetDashboard returns an aggregated financial overview for the RW.
func (s *Service) GetDashboard(ctx context.Context, ac *auth.AuthContext, rwID string) (*DashboardSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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

// GetTransactions returns a paginated list of posted transactions from RW's independent ledger.
func (s *Service) GetTransactions(ctx context.Context, ac *auth.AuthContext, rwID string,
	period string, page, pageSize int, startDate, endDate string,
) ([]*TransactionEntry, int, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
		return nil, 0, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	list, total, err := s.repo.GetTransactions(ctx, tx, rwID, period, page, pageSize, startDate, endDate)
	if err != nil {
		return nil, 0, fmt.Errorf("get transactions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit transaction: %w", err)
	}

	return list, total, nil
}

// GetTransactionByID returns a single transaction by ID.
func (s *Service) GetTransactionByID(ctx context.Context, ac *auth.AuthContext, rwID, id string) (*TransactionEntry, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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
	if err := s.requirePermission(ctx, ac, rwID, "finance.create"); err != nil {
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

	return entry, nil
}

// =========================================================================
// RW Fund Locations
// =========================================================================

// GetLocations returns all RW fund locations with current balances.
func (s *Service) GetLocations(ctx context.Context, ac *auth.AuthContext, rwID string, isActive *bool) ([]*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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

	return locations, nil
}

// GetLocationByID returns details of a single RW fund location.
func (s *Service) GetLocationByID(ctx context.Context, ac *auth.AuthContext, rwID, locID string) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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

// CreateLocation creates a new RW fund location (requires finance.create).
func (s *Service) CreateLocation(ctx context.Context, ac *auth.AuthContext, rwID string, in CreateRWFundLocationInput) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.create"); err != nil {
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

// UpdateLocation updates an existing RW fund location (requires finance.update).
func (s *Service) UpdateLocation(ctx context.Context, ac *auth.AuthContext, rwID, locID string, in UpdateRWFundLocationInput) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.update"); err != nil {
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

// ToggleLocationStatus activates or deactivates an RW fund location (requires finance.update).
func (s *Service) ToggleLocationStatus(ctx context.Context, ac *auth.AuthContext, rwID, locID string, isActive bool) (*LocationSummary, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.update"); err != nil {
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
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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

// CreateTransfer executes a transfer between two locations within the same RW (requires finance.update).
func (s *Service) CreateTransfer(ctx context.Context, ac *auth.AuthContext, rwID string, in CreateFundTransferInput) (*FundTransfer, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.update"); err != nil {
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

// ReverseTransfer reverses a completed transfer within the same RW (requires finance.update).
func (s *Service) ReverseTransfer(ctx context.Context, ac *auth.AuthContext, rwID, transferID, reason string) (*FundTransfer, error) {
	if err := s.requirePermission(ctx, ac, rwID, "finance.update"); err != nil {
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
	if err := s.requirePermission(ctx, ac, rwID, "finance.read"); err != nil {
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

	return transfers, total, nil
}

// ---- Money utility functions (delegates to finance package) ----

func ParseMoney(s string) (int64, string, error) {
	return finance.ParseMoney(s)
}

func FormatMoney(cents int64) string {
	return finance.FormatMoney(cents)
}
