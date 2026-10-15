-- 021_bills_allow_zero_amount.up.sql
-- Allow bills with amount = 0 for voluntary iuran (tagihan sukarela Rp0).
-- Previously bills.amount had CHECK (amount > 0) which prevented Rp0 bills.
-- Voluntary iuran definitions with amount 0 now produce real per-household
-- bills instead of a single synthetic "Semua Warga (Sukarela)" row.

-- 1. Relax the CHECK constraint: amount >= 0 instead of amount > 0

ALTER TABLE bills DROP CONSTRAINT IF EXISTS bills_amount_check;

ALTER TABLE bills ADD CONSTRAINT bills_amount_non_negative
    CHECK (amount >= 0);
