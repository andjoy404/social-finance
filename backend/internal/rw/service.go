package rw

import (
	"context"
	"fmt"

	"social-finance/internal/auth"
	"social-finance/internal/database"
	"social-finance/internal/finance"
)

// Service provides RW-scoped read operations with authorization enforcement.
// Every public method validates that the user has active RW membership
// matching the requested rwID before executing any query.
//
// The scopingConfig is used to enforce deployment-level scope and active RW
// checks for Super Admin (SA) access, preventing SA from bypassing scope
// validation when the service is called directly (not via HTTP middleware).
type Service struct {
	pool        *database.Pool
	repo        *Repository
	scopingCfg  *auth.ScopingConfig
}

func NewService(pool *database.Pool, repo *Repository, scopingCfg *auth.ScopingConfig) *Service {
	return &Service{
		pool:       pool,
		repo:       repo,
		scopingCfg: scopingCfg,
	}
}

// checkRWAccess verifies the AuthContext has authorization to access the given rwID.
// Returns nil if authorized, error otherwise.
//
// System-level SUPER_ADMIN is authorized by their system role, subject to:
//   - The RW ID must be in the configured SCOPED_RW_IDS.
//   - The RW must exist and be active in the database.
//   - If scopingCfg is nil or empty, no SA can access any RW (fail-closed).
//
// Regular users must have an active RW membership whose rwID matches the requested scope.
func (s *Service) checkRWAccess(ac *auth.AuthContext, rwID string) error {
	if ac == nil {
		return fmt.Errorf("no authentication context")
	}
	// Super Admin: validate scope and active status via shared validator.
	if ac.SystemRole == auth.SystemRoleSuperAdmin {
		return auth.ValidateSAForRW(context.Background(), s.scopingCfg, s.pool, rwID)
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
//
// System-level SUPER_ADMIN always passes — they are authorized by their system
// role and have already been validated by RequireRWScope middleware or
// ValidateSAForRW (for direct service calls).
//
// For regular users: finds the RW membership matching rwID and checks if the
// jabatan grants the required permission via hasRWScopePermission.
// If the user has no matching membership, returns ErrRWAccessDenied.
// If the jabatan has no required permission, returns ErrPermissionDenied.
// If the jabatan is not recognized, returns ErrPermissionDenied (deny by default).
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

// requirePermission is a helper that wraps both access and permission checks.
// It is the standard entry point for all public service methods.
func (s *Service) requirePermission(ctx context.Context, ac *auth.AuthContext, rwID string, permissionCode string) error {
	if err := s.checkRWAccess(ac, rwID); err != nil {
		return err
	}
	return s.checkRWPermission(ac, rwID, permissionCode)
}

// GetDashboard returns an aggregated financial overview for the RW.
// Balance is computed from RW's own independent ledger:
// NetBalance = OpeningBalance + Income - Expense
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
// NetBalance = OpeningBalance + Income - Expense
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

// GetLocations returns a read-only list of fund locations from all RTs in the RW.
// Locations remain RT-level — shown for reference only, NOT used for KAS RW balance.
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

// ---- Money utility functions (delegates to finance package) ----

// ParseMoney delegates to finance.ParseMoney for consistent monetary parsing.
func ParseMoney(s string) (int64, string, error) {
	return finance.ParseMoney(s)
}

// FormatMoney delegates to finance.FormatMoney for consistent money formatting.
func FormatMoney(cents int64) string {
	return finance.FormatMoney(cents)
}
