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

// Count returns the number of RTs matching the active filter and search.
func (s *RTManager) Count(ctx context.Context, tx *sql.Tx, isActive *bool, search *string) (int, error) {
	return Count(ctx, tx, isActive, search)
}

// Update partially updates an RT.
func (s *RTManager) Update(ctx context.Context, tx *sql.Tx, id string, in *UpdateRTInput) (*RT, error) {
	return Update(ctx, tx, id, in)
}

// Deactivates an RT (soft delete).
func (s *RTManager) Deactivate(ctx context.Context, tx *sql.Tx, id string) error {
	return Deactivate(ctx, tx, id)
}
