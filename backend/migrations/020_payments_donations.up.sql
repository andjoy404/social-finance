-- 020_payments_donations.up.sql
-- Allow payments without bills for voluntary donations (sumbangan sukarela)
-- Extends CreatePayment to accept due_id as alternative to bill_id.

-- 1. Make bill_id nullable so donation payments can exist without a bill
ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_bill_id_fkey;

ALTER TABLE payments
    ALTER COLUMN bill_id DROP NOT NULL;

-- Keep bill history restrictive; payments must not lose their target.
ALTER TABLE payments
    ADD CONSTRAINT payments_bill_id_fkey
    FOREIGN KEY (bill_id) REFERENCES bills(id) ON DELETE RESTRICT;

-- 2. Add due_id column to link donation payments back to the due definition
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS due_id UUID REFERENCES dues(id) ON DELETE RESTRICT;

-- Keep a donation tied to its submitter so warga access can be owner-scoped.
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS submitted_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Index for querying donations by due
CREATE INDEX IF NOT EXISTS idx_payments_due_id ON payments(due_id);

-- 3. Enforce exactly one target: bill payments or voluntary donations.
ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_bill_or_due;
ALTER TABLE payments ADD CONSTRAINT chk_payments_bill_or_due
    CHECK ((bill_id IS NOT NULL) <> (due_id IS NOT NULL));
