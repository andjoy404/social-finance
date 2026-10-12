-- 017_dues_payment_rule.up.sql
-- Add payment_rule to dues table: 'full' (lunas semua) or 'partial' (lunas sebagian)

-- 1. Add payment_rule column with default
ALTER TABLE dues
    ADD COLUMN IF NOT EXISTS payment_rule TEXT NOT NULL DEFAULT 'full';

-- 2. Add CHECK constraint (idempotent via DO block)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'dues_payment_rule_check'
    ) THEN
        ALTER TABLE dues ADD CONSTRAINT dues_payment_rule_check CHECK (payment_rule IN ('full', 'partial'));
    END IF;
END $$;

-- 3. Index on payment_rule for filtering active dues by payment type
CREATE INDEX IF NOT EXISTS idx_dues_payment_rule ON dues(payment_rule);

-- 4. Backfill: all existing dues get 'full' (covered by DEFAULT above; UPDATE is a no-op)
UPDATE dues SET payment_rule = 'full' WHERE payment_rule IS NULL OR payment_rule = '';
