-- 018_dues_is_voluntary.up.sql
-- Allow dues with amount = 0 for voluntary donations (sumbangan sukarela)

-- 1. Add is_voluntary column (default false to keep existing dues unchanged)
ALTER TABLE dues
    ADD COLUMN IF NOT EXISTS is_voluntary BOOLEAN NOT NULL DEFAULT false;

-- 2. Drop the old CHECK constraint amount > 0
ALTER TABLE dues DROP CONSTRAINT IF EXISTS dues_amount_check;

-- 3. Add new CHECK: amount >= 0 when voluntary, amount > 0 when not
ALTER TABLE dues ADD CONSTRAINT dues_amount_non_negative
    CHECK (amount >= 0);

-- 4. Index on is_voluntary for filtering voluntary dues
CREATE INDEX IF NOT EXISTS idx_dues_is_voluntary ON dues(is_voluntary);
