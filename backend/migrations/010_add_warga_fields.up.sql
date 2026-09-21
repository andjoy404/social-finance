-- Migration 010: Remove kk_number, add resident NIK/email, use resident phone

-- =====================================================================
-- HOUSEHOLDS: remove kk_number and duplicated personal phone
-- =====================================================================

ALTER TABLE households DROP CONSTRAINT IF EXISTS uniq_households_rt_kk;
ALTER TABLE households DROP COLUMN IF EXISTS kk_number;
ALTER TABLE households DROP COLUMN IF EXISTS phone;

-- =====================================================================
-- RESIDENTS: add NIK, email
-- Keep phone nullable for legacy migration; app layer enforces required
-- =====================================================================

ALTER TABLE residents ADD COLUMN IF NOT EXISTS nik TEXT;
ALTER TABLE residents ADD COLUMN IF NOT EXISTS email TEXT;

-- UNIQUE per RT; partial so NULL nik rows are allowed for legacy

CREATE UNIQUE INDEX IF NOT EXISTS uniq_residents_rt_nik
    ON residents(rt_id, nik) WHERE nik IS NOT NULL;

-- Email lookup index

CREATE INDEX IF NOT EXISTS idx_residents_email_lookup
    ON residents(rt_id, lower(trim(email))) WHERE email IS NOT NULL;

-- NIK format constraints (allows NULL for legacy)

ALTER TABLE residents ADD CONSTRAINT chk_residents_nik_length
    CHECK (nik IS NULL OR char_length(nik) = 16);
ALTER TABLE residents ADD CONSTRAINT chk_residents_nik_digits
    CHECK (nik IS NULL OR nik ~ '^[0-9]{16}$');
