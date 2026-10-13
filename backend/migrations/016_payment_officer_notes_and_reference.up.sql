-- 016_payment_officer_notes_and_reference.up.sql
-- Add officer notes for approval, verified_by_name for display, and payment_no for audit reference

-- 1. Officer notes: written by bendahara/pengurus when APPROVING a payment
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS officer_notes TEXT;

-- 2. Human-readable name of the officer who verified (lookup from users table)
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS verified_by_name TEXT;

-- 3. Unique payment reference number for audit (e.g. PAY-20261012-00001)
-- Column uses ADD COLUMN IF NOT EXISTS to be idempotent;
-- constraint uses DO block so it survives partial-retry scenarios.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS payment_no TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'payments_payment_no_unique'
    ) THEN
        ALTER TABLE payments ADD CONSTRAINT payments_payment_no_unique UNIQUE (payment_no);
    END IF;
END $$;

-- Generate payment_no for existing records
UPDATE payments
SET payment_no = 'PAY-' || TO_CHAR(created_at, 'YYYYMMDD') || '-' || LPAD(row_number::TEXT, 5, '0')
FROM (
    SELECT id, ROW_NUMBER() OVER (ORDER BY created_at, id) AS row_number
    FROM payments
) AS sub
WHERE payments.id = sub.id
  AND payments.payment_no IS NULL;
