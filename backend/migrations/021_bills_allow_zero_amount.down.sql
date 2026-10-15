-- 021_bills_allow_zero_amount.down.sql
-- Revert: restore strict amount > 0 constraint on bills table.
-- If there are existing Rp0 bills, this rollback will fail — the down
-- migration must be run before any Rp0 bills are created in production.

ALTER TABLE bills DROP CONSTRAINT IF EXISTS bills_amount_non_negative;

ALTER TABLE bills ADD CONSTRAINT bills_amount_check CHECK (amount > 0);
