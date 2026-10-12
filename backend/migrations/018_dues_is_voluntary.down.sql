-- 018_dues_is_voluntary.down.sql
-- Rollback: remove is_voluntary, restore strict amount > 0 constraint

ALTER TABLE dues DROP CONSTRAINT IF EXISTS dues_amount_non_negative;
ALTER TABLE dues ADD CONSTRAINT dues_amount_check CHECK (amount > 0);
ALTER TABLE dues DROP COLUMN IF EXISTS is_voluntary;
DROP INDEX IF EXISTS idx_dues_is_voluntary;
