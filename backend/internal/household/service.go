package household

import (
	"context"
	"database/sql"

	"social-finance/internal/database"
)

// HouseholdManager wraps the repository for business logic operations over
// both households and residents.
//
// IMPORTANT: The rtID parameter must originate from JWT auth context, never
// from client input. Handlers extract it with auth.GetAuthContext(r).RTID
// and pass it here.
type HouseholdManager struct {
	p *database.Pool
}

// NewHouseholdManager creates a new HouseholdManager.
func NewHouseholdManager(p *database.Pool) *HouseholdManager {
	return &HouseholdManager{p: p}
}

// ─── Household ──────────────────────────────────────────────────────────────

// Create inserts a new household and its physical house / occupancy.
func (s *HouseholdManager) CreateHousehold(ctx context.Context, tx *sql.Tx, in CreateHouseholdInput) (*Household, error) {
	return HouseholdCreate(ctx, tx, in)
}

// GetByID finds a household by ID within the RT, projecting current occupancy.
func (s *HouseholdManager) GetHouseholdByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Household, error) {
	return HouseholdGetByID(ctx, tx, id, rtID)
}

// ListHouseholds returns households for the RT with optional filters and pagination.
func (s *HouseholdManager) ListHouseholds(ctx context.Context, tx *sql.Tx, rtID string, isActive *bool, search *string, offset, limit int) ([]*Household, error) {
	return HouseholdList(ctx, tx, rtID, isActive, search, offset, limit)
}

// CountHouseholds returns the count of households in the RT.
func (s *HouseholdManager) CountHouseholds(ctx context.Context, tx *sql.Tx, rtID string, isActive *bool, search *string) (int, error) {
	return HouseholdCount(ctx, tx, rtID, isActive, search)
}

// UpdateHousehold partially updates a household and physical house / occupancy corrections.
func (s *HouseholdManager) UpdateHousehold(ctx context.Context, tx *sql.Tx, id, rtID string, in *UpdateHouseholdInput) (*Household, error) {
	return HouseholdUpdate(ctx, tx, id, rtID, in)
}

// MoveHousehold moves a household to a new physical house starting at start_date.
func (s *HouseholdManager) MoveHousehold(ctx context.Context, tx *sql.Tx, id, rtID string, in MoveHouseholdInput) (*Household, error) {
	return HouseholdMove(ctx, tx, id, rtID, in)
}

// DeactivateHousehold soft-deletes a household.
func (s *HouseholdManager) DeactivateHousehold(ctx context.Context, tx *sql.Tx, id, rtID string) error {
	return HouseholdDeactivate(ctx, tx, id, rtID)
}

// ListAllHouseholds returns households across all RTs with optional filters and pagination.
func (s *HouseholdManager) ListAllHouseholds(ctx context.Context, tx *sql.Tx, isActive *bool, search *string, offset, limit int) ([]*Household, error) {
	return HouseholdListAll(ctx, tx, isActive, search, offset, limit)
}

// CountAllHouseholds returns the count of households across all RTs.
func (s *HouseholdManager) CountAllHouseholds(ctx context.Context, tx *sql.Tx, isActive *bool, search *string) (int, error) {
	return HouseholdCountAll(ctx, tx, isActive, search)
}

// GetAllHouseholdByID finds a household by ID across all RTs, projecting current occupancy.
func (s *HouseholdManager) GetAllHouseholdByID(ctx context.Context, tx *sql.Tx, id string) (*Household, error) {
	return HouseholdGetByIDAll(ctx, tx, id)
}

// ─── Resident ───────────────────────────────────────────────────────────────

// CreateResident inserts a new resident, validating that the household belongs to the RT.
func (s *HouseholdManager) CreateResident(ctx context.Context, tx *sql.Tx, in CreateResidentInput, rtID string) (*Resident, error) {
	return ResidentCreate(ctx, tx, in, rtID)
}

// GetResidentByID finds a resident by ID within the RT.
func (s *HouseholdManager) GetResidentByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Resident, error) {
	return ResidentGetByID(ctx, tx, id, rtID)
}

// ListResidents returns residents for the RT with optional filters and pagination.
func (s *HouseholdManager) ListResidents(ctx context.Context, tx *sql.Tx, rtID string, householdID *string, isActive *bool, search *string, offset, limit int) ([]*Resident, error) {
	return ResidentList(ctx, tx, rtID, householdID, isActive, search, offset, limit)
}

// CountResidents returns the count of residents in the RT.
func (s *HouseholdManager) CountResidents(ctx context.Context, tx *sql.Tx, rtID string, householdID *string, isActive *bool) (int, error) {
	return ResidentCount(ctx, tx, rtID, householdID, isActive)
}

// UpdateResident partially updates a resident.
func (s *HouseholdManager) UpdateResident(ctx context.Context, tx *sql.Tx, id, rtID string, in *UpdateResidentInput) (*Resident, error) {
	return ResidentUpdate(ctx, tx, id, rtID, in)
}

// MoveResident moves a resident to another household.
func (s *HouseholdManager) MoveResident(ctx context.Context, tx *sql.Tx, id, rtID string, in MoveResidentInput) (*Resident, error) {
	return ResidentMove(ctx, tx, id, rtID, in)
}

// DeactivateResident soft-deletes a resident.
func (s *HouseholdManager) DeactivateResident(ctx context.Context, tx *sql.Tx, id, rtID string) error {
	return ResidentDeactivate(ctx, tx, id, rtID)
}

// ListAllResidents returns residents across all RTs with optional filters and pagination.
func (s *HouseholdManager) ListAllResidents(ctx context.Context, tx *sql.Tx, householdID *string, isActive *bool, search *string, offset, limit int) ([]*Resident, error) {
	return ResidentListAll(ctx, tx, householdID, isActive, search, offset, limit)
}

// CountAllResidents returns the count of residents across all RTs.
func (s *HouseholdManager) CountAllResidents(ctx context.Context, tx *sql.Tx, householdID *string, isActive *bool) (int, error) {
	return ResidentCountAll(ctx, tx, householdID, isActive)
}
