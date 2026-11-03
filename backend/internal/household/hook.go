package household

import (
	"context"
	"database/sql"
)

// SyncOccupancyBillsFn is an optional hook invoked when a household occupancy is created or updated
// to synchronize active dues/bills for the occupancy.
var SyncOccupancyBillsFn func(ctx context.Context, tx *sql.Tx, rtID, occupancyID string) error

func triggerSyncBills(ctx context.Context, tx *sql.Tx, rtID, occupancyID string) {
	if SyncOccupancyBillsFn != nil && rtID != "" && occupancyID != "" {
		_ = SyncOccupancyBillsFn(ctx, tx, rtID, occupancyID)
	}
}

