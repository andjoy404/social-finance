-- Rollback migration 010

-- Restore households.kk_number and phone columns
ALTER TABLE households ADD COLUMN kk_number TEXT;
ALTER TABLE households ADD COLUMN phone TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_households_rt_kk
    ON households (rt_id, kk_number)
    WHERE is_active = true AND kk_number IS NOT NULL;

-- Drop resident NIK/email columns and constraints
ALTER TABLE residents DROP CONSTRAINT IF EXISTS chk_residents_nik_length;
ALTER TABLE residents DROP CONSTRAINT IF EXISTS chk_residents_nik_digits;
DROP INDEX IF EXISTS uniq_residents_rt_nik;
DROP INDEX IF EXISTS idx_residents_email_lookup;
ALTER TABLE residents DROP COLUMN IF EXISTS nik;
ALTER TABLE residents DROP COLUMN IF EXISTS email;
