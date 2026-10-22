-- 032_dues_valid_until.up.sql
-- Adds an optional validity end date for voluntary dues.
--
-- Semantics:
--   * The due remains active through the entire valid_until date in Asia/Jakarta.
--   * It expires at 00:00 WIB on the following date.
--   * Regular dues must keep valid_until NULL and never expire automatically.
--
-- This migration is additive-only and does not modify bills, payments,
-- payment allocations, transactions, balances, or historical reports.

ALTER TABLE dues
    ADD COLUMN IF NOT EXISTS valid_until DATE;

ALTER TABLE dues
    DROP CONSTRAINT IF EXISTS dues_valid_until_voluntary_check;
ALTER TABLE dues
    ADD CONSTRAINT dues_valid_until_voluntary_check
    CHECK (valid_until IS NULL OR is_voluntary);

-- Supports the daily expiry job: only active voluntary dues with a deadline.
CREATE INDEX IF NOT EXISTS idx_dues_voluntary_expiry
    ON dues(valid_until)
    WHERE is_voluntary AND is_active AND valid_until IS NOT NULL;
