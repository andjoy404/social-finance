-- 004_households_and_residents.up.sql
--
-- Phase 3: Household and Resident tables.

-- ================================================================
-- 1. Households
-- ================================================================
CREATE TABLE IF NOT EXISTS households (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id           uuid NOT NULL REFERENCES rts(id),
    kk_number       text UNIQUE,
    head_name       text NOT NULL,
    address         text,
    phone           text,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

COMMENT ON COLUMN households.kk_number IS
    'Kartu Keluarga number. Unique among active households within an RT.';

CREATE INDEX idx_households_rt_id ON households (rt_id);
CREATE INDEX idx_households_rt_is_active ON households (rt_id, is_active);
CREATE UNIQUE INDEX uniq_households_rt_kk
    ON households (rt_id, kk_number)
    WHERE is_active = true AND kk_number IS NOT NULL;

-- ================================================================
-- 2. Residents
-- ================================================================
CREATE TABLE IF NOT EXISTS residents (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id              uuid NOT NULL REFERENCES rts(id),
    household_id       uuid NOT NULL REFERENCES households(id),
    full_name          text NOT NULL,
    phone              text,
    relationship_to_head text,
    is_active          boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_residents_rt_id ON residents (rt_id);
CREATE INDEX idx_residents_household_id ON residents (household_id);
CREATE INDEX idx_residents_rt_household_active
    ON residents (rt_id, household_id, is_active);
