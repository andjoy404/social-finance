package rt

import (
	"context"
	"database/sql"
	"fmt"
)

// RTManager wraps the repository for business logic operations.
type RTManager struct {
	repo *repository
}

// repository is the internal database interface.
type repository struct{}

func newRepository() *repository {
	return &repository{}
}

// NewService creates an RTManager with a configured repository.
func NewService() *RTManager {
	return &RTManager{repo: newRepository()}
}

// Create inserts a new RT.
func (s *RTManager) Create(ctx context.Context, tx *sql.Tx, in CreateRTInput) (*RT, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}
	return Create(ctx, tx, in)
}

// GetByID finds an RT by UUID.
func (s *RTManager) GetByID(ctx context.Context, tx *sql.Tx, id string) (*RT, error) {
	return GetByID(ctx, tx, id)
}

// ListAll returns RTs with optional active filter, search, and pagination.
func (s *RTManager) ListAll(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, offset, limit int) ([]*RT, error) {
	return ListAll(ctx, tx, isActive, search, offset, limit)
}

// ListAllWithFilter returns RTs with optional active filter, search, rw_id filter, and pagination.
func (s *RTManager) ListAllWithFilter(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, rwID *string, offset, limit int) ([]*RT, error) {
	return ListAllWithFilter(ctx, tx, isActive, search, rwID, offset, limit)
}

// Count returns the number of RTs matching the active filter and search.
func (s *RTManager) Count(ctx context.Context, tx *sql.Tx, isActive *bool, search *string) (int, error) {
	return Count(ctx, tx, isActive, search)
}

// CountWithFilter returns the number of RTs matching the active filter, search, and rw_id.
func (s *RTManager) CountWithFilter(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, rwID *string) (int, error) {
	return CountWithFilter(ctx, tx, isActive, search, rwID)
}

// Update partially updates an RT.
func (s *RTManager) Update(ctx context.Context, tx *sql.Tx, id string, in *UpdateRTInput) (*RT, error) {
	return Update(ctx, tx, id, in)
}

// Deactivates an RT (soft delete).
func (s *RTManager) Deactivate(ctx context.Context, tx *sql.Tx, id string) error {
	return Deactivate(ctx, tx, id)
}

// CreateRW inserts a new RW.
func (s *RTManager) CreateRW(ctx context.Context, tx *sql.Tx, in CreateRWInput) (*RW, error) {
	return CreateRW(ctx, tx, in)
}

// GetRWByID finds an RW by UUID.
func (s *RTManager) GetRWByID(ctx context.Context, tx *sql.Tx, id string) (*RW, error) {
	return GetRWByID(ctx, tx, id)
}

// ListAllRWs returns RWs with optional active filter, search, and pagination.
func (s *RTManager) ListAllRWs(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, offset, limit int) ([]*RW, error) {
	return ListAllRWs(ctx, tx, isActive, search, offset, limit)
}

// CountRWs returns the number of RWs matching the active filter and search.
func (s *RTManager) CountRWs(ctx context.Context, tx *sql.Tx, isActive *bool, search *string) (int, error) {
	return CountRWs(ctx, tx, isActive, search)
}

// UpdateRW partially updates an RW.
func (s *RTManager) UpdateRW(ctx context.Context, tx *sql.Tx, id string, in *UpdateRWInput) (*RW, error) {
	return UpdateRW(ctx, tx, id, in)
}

// DeactivateRW soft-deletes an RW.
func (s *RTManager) DeactivateRW(ctx context.Context, tx *sql.Tx, id string) error {
	return DeactivateRW(ctx, tx, id)
}
