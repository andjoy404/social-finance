-- 039_bills_is_active.up.sql
-- Add is_active column to bills table to support activating/deactivating individual monthly bills.
-- All existing bills default to active (is_active = true).

ALTER TABLE bills ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- Index for tenant-scoped active bill queries and FIFO lookups
CREATE INDEX IF NOT EXISTS idx_bills_rt_is_active ON bills (rt_id, is_active);
CREATE INDEX IF NOT EXISTS idx_bills_occ_due_active ON bills (household_occupancy_id, due_id, is_active);

