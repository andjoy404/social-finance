-- 039_bills_is_active.down.sql
-- Revert is_active column on bills table.

DROP INDEX IF EXISTS idx_bills_occ_due_active;
DROP INDEX IF EXISTS idx_bills_rt_is_active;
ALTER TABLE bills DROP COLUMN IF EXISTS is_active;

