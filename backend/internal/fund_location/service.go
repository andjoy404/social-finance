package fundlocation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"social-finance/internal/database"
	"social-finance/internal/finance"
)

type Service struct {
	pool *database.Pool
	repo *Repository
}

func NewService(pool *database.Pool, repo *Repository) *Service {
	return &Service{
		pool: pool,
		repo: repo,
	}
}

func (s *Service) CreateLocation(ctx context.Context, rtID, userID string, in CreateFundLocationInput) (*FundLocation, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	loc, err := s.repo.CreateLocation(ctx, tx, rtID, in)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(loc)
	_ = finance.AuditLogInsert(ctx, tx, finance.AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_fund_location",
		EntityType: "fund_location",
		EntityID:   loc.ID,
		NewValues:  auditPayload,
	})

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return loc, nil
}

func (s *Service) GetLocationByID(ctx context.Context, rtID, id string) (*FundLocation, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return s.repo.GetLocationByID(ctx, tx, id, rtID)
}

func (s *Service) ListLocationsByRT(ctx context.Context, rtID string, category *string, isActive *bool) ([]*FundLocation, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return s.repo.ListLocationsByRT(ctx, tx, rtID, category, isActive)
}

func (s *Service) UpdateLocation(ctx context.Context, rtID, userID, id string, in UpdateFundLocationInput) (*FundLocation, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	loc, err := s.repo.UpdateLocation(ctx, tx, id, rtID, in)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(loc)
	_ = finance.AuditLogInsert(ctx, tx, finance.AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "update_fund_location",
		EntityType: "fund_location",
		EntityID:   loc.ID,
		NewValues:  auditPayload,
	})

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return loc, nil
}

func (s *Service) DeactivateLocation(ctx context.Context, rtID, userID, id string) (*FundLocation, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	loc, err := s.repo.DeactivateLocation(ctx, tx, id, rtID)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(loc)
	_ = finance.AuditLogInsert(ctx, tx, finance.AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "deactivate_fund_location",
		EntityType: "fund_location",
		EntityID:   loc.ID,
		NewValues:  auditPayload,
	})

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return loc, nil
}

func (s *Service) CreateTransfer(ctx context.Context, rtID, userID string, in CreateFundTransferInput) (*FundTransfer, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	tr, err := s.repo.CreateTransfer(ctx, tx, rtID, userID, in)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(tr)
	_ = finance.AuditLogInsert(ctx, tx, finance.AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "create_fund_transfer",
		EntityType: "fund_transfer",
		EntityID:   tr.ID,
		NewValues:  auditPayload,
	})

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tr, nil
}

func (s *Service) ListTransfersByRT(ctx context.Context, rtID string, offset, limit int) ([]*FundTransfer, int, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	return s.repo.ListTransfersByRT(ctx, tx, rtID, offset, limit)
}

func (s *Service) GetTransferByID(ctx context.Context, rtID, id string) (*FundTransfer, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return s.repo.GetTransferByID(ctx, tx, id, rtID)
}

func (s *Service) ReverseTransfer(ctx context.Context, rtID, userID, transferID string, in ReverseFundTransferInput) (*FundTransfer, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	tr, err := s.repo.ReverseTransfer(ctx, tx, rtID, userID, transferID, in)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(tr)
	_ = finance.AuditLogInsert(ctx, tx, finance.AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "reverse_fund_transfer",
		EntityType: "fund_transfer",
		EntityID:   tr.ID,
		NewValues:  auditPayload,
	})

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tr, nil
}

func (s *Service) SetOpeningBalance(ctx context.Context, rtID, userID, locationID string, in SetOpeningBalanceInput) (*FundLocation, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	loc, err := s.repo.SetOpeningBalance(ctx, tx, rtID, userID, locationID, in)
	if err != nil {
		return nil, err
	}

	auditPayload, _ := json.Marshal(loc)
	_ = finance.AuditLogInsert(ctx, tx, finance.AuditLog{
		RTID:       rtID,
		UserID:     &userID,
		Action:     "set_opening_balance",
		EntityType: "fund_location",
		EntityID:   loc.ID,
		NewValues:  auditPayload,
	})

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return loc, nil
}

func (s *Service) GetReconciliationReport(ctx context.Context, rtID string) (*ReconciliationReport, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return s.repo.GetReconciliationReport(ctx, tx, rtID)
}

func (s *Service) GetLocationHistory(ctx context.Context, rtID, locationID string, offset, limit int) ([]*LocationBalanceChange, int, error) {
	tx, err := s.pool.BeginTx(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	var total int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM location_balance_changes WHERE location_id = $1 AND rt_id = $2`, locationID, rtID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, rt_id, location_id, transaction_id, transfer_id, change_type, amount_delta::text, notes, created_by::text, created_at
		 FROM location_balance_changes
		 WHERE location_id = $1 AND rt_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		locationID, rtID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list location history: %w", err)
	}
	defer rows.Close()

	var list []*LocationBalanceChange
	for rows.Next() {
		var lbc LocationBalanceChange
		var txID, trID, notesStr, createdBy sql.NullString
		if err := rows.Scan(&lbc.ID, &lbc.RTID, &lbc.LocationID, &txID, &trID, &lbc.ChangeType, &lbc.AmountDelta, &notesStr, &createdBy, &lbc.CreatedAt); err != nil {
			return nil, 0, err
		}
		if txID.Valid {
			lbc.TransactionID = &txID.String
		}
		if trID.Valid {
			lbc.TransferID = &trID.String
		}
		if notesStr.Valid {
			lbc.Notes = &notesStr.String
		}
		if createdBy.Valid {
			lbc.CreatedBy = &createdBy.String
		}
		list = append(list, &lbc)
	}
	return list, total, rows.Err()
}
