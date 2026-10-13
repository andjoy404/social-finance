-- 016_payment_officer_notes_and_reference.down.sql
-- Remove officer notes, verified_by_name, and payment_no columns

-- Drop unique constraint first (if it exists from migration or manual ALTER)
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_payment_no_unique;

-- Then drop columns in reverse order
ALTER TABLE payments DROP COLUMN IF EXISTS officer_notes;
ALTER TABLE payments DROP COLUMN IF EXISTS verified_by_name;
ALTER TABLE payments DROP COLUMN IF EXISTS payment_no;
