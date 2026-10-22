-- 032_dues_valid_until.down.sql
-- Reverses only the schema addition. No financial/history table is touched.

DROP INDEX IF EXISTS idx_dues_voluntary_expiry;
ALTER TABLE dues DROP CONSTRAINT IF EXISTS dues_valid_until_voluntary_check;
ALTER TABLE dues DROP COLUMN IF EXISTS valid_until;
