-- 017_dues_payment_rule.down.sql
-- Remove payment_rule column and related artifacts from dues table

-- Drop CHECK constraint first
ALTER TABLE dues DROP CONSTRAINT IF EXISTS dues_payment_rule_check;

-- Drop index
DROP INDEX IF EXISTS idx_dues_payment_rule;

-- Drop column last
ALTER TABLE dues DROP COLUMN IF EXISTS payment_rule;
