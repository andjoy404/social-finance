-- 020_payments_donations.down.sql
-- Rollback migration 020: restore strict bill_id NOT NULL constraint

-- 1. Drop the new constraints and columns
-- Refuse rollback before any schema changes if donation rows still exist.
DO $$
DECLARE
    null_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO null_count FROM payments WHERE bill_id IS NULL;
    IF null_count > 0 THEN
        RAISE EXCEPTION 'Cannot restore bill_id NOT NULL: % payment(s) have NULL bill_id', null_count;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_payments_due_id;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_bill_or_due;

ALTER TABLE payments DROP COLUMN IF EXISTS due_id;
ALTER TABLE payments DROP COLUMN IF EXISTS submitted_by;

-- 2. Restore bill_id to NOT NULL.
ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_bill_id_fkey;

ALTER TABLE payments
    ALTER COLUMN bill_id SET NOT NULL;

ALTER TABLE payments
    ADD CONSTRAINT payments_bill_id_fkey
    FOREIGN KEY (bill_id) REFERENCES bills(id) ON DELETE RESTRICT;
