-- 008_temporal_household_model.down.sql
--
-- RESTORE v7 Household/Resident schema.
--
-- This migration reconstructs the legacy columns from CURRENT temporal
-- state (end_date IS NULL).  Rows without a current temporal row will
-- have NULL restored.
--
-- Limitations:
--   - Historical occupancy/residency state (prior periods) CANNOT be
--     represented in the v7 schema.  That temporal history is lost.
--   - If a legacy household had NO current occupancy, its v7 fields
--     (house_number, address, occupancy_status) will be NULL.
--   - If a legacy resident had NO current residency, its v7 fields
--     (household_id, relationship_to_head) will be NULL.
--   - No fabricated dates are produced.

-- ================================================================
-- 1. ADD LEGACY COLUMNS (nullable for safe reconstruction)
-- ================================================================

ALTER TABLE households ADD COLUMN IF NOT EXISTS house_number text;
ALTER TABLE households ADD COLUMN IF NOT EXISTS address text;
ALTER TABLE households ADD COLUMN IF NOT EXISTS occupancy_status text;

ALTER TABLE residents ADD COLUMN IF NOT EXISTS household_id uuid;
ALTER TABLE residents ADD COLUMN IF NOT EXISTS relationship_to_head text;

-- ================================================================
-- 2. RECONSTRUCT HOUSEHOLD FIELDS
-- ================================================================

-- Priority 1: households with a current (end_date IS NULL) occupancy.
-- Restored from temporal tables for occupancy_status consistency.
-- All legacy text fields restored from the bridge for exact fidelity.
WITH resolved_households AS (
    SELECT ho.household_id, ho.occupancy_status, m.legacy_house_number, m.legacy_address
    FROM household_occupancies ho
    JOIN physical_houses ph
      ON  ph.id = ho.physical_house_id
      AND ph.is_active = true
    JOIN migration_008_household_physical_house_map m
      ON  m.household_id = ho.household_id
    WHERE ho.end_date IS NULL
)
UPDATE households h
SET
    house_number       = r.legacy_house_number,
    address            = r.legacy_address,
    occupancy_status   = r.occupancy_status
FROM resolved_households r
WHERE r.household_id = h.id;

-- Priority 2: legacy households not yet restored by Priority 1 (inactive
-- households without a temporal occupancy).  Restored from the migration
-- rollback bridge which stores exact original v7 values.
UPDATE households h
SET
    house_number       = m.legacy_house_number,
    address            = m.legacy_address,
    occupancy_status   = m.legacy_occupancy_status
FROM migration_008_household_physical_house_map m
WHERE m.household_id = h.id
  AND (
    h.house_number IS NULL
    OR h.address IS NULL
    OR h.occupancy_status IS NULL
  );

-- ================================================================
-- 3. RECONSTRUCT RESIDENT FIELDS
-- ================================================================

WITH resolved_residents AS (
    SELECT rp.resident_id, ho.household_id, rp.relationship_to_head
    FROM residency_periods rp
    JOIN household_occupancies ho
      ON ho.id = rp.household_occupancy_id
    WHERE rp.end_date IS NULL
)
UPDATE residents r
SET
    household_id           = rr.household_id,
    relationship_to_head   = rr.relationship_to_head
FROM resolved_residents rr
WHERE rr.resident_id = r.id;

-- Restore residents not resolved by Residency Period from provenance.
-- This covers INACTIVE residents who intentionally have no Residency Period.

UPDATE residents r
SET
    household_id           = m.legacy_household_id,
    relationship_to_head   = m.legacy_relationship_to_head
FROM migration_008_resident_legacy_map m
WHERE m.resident_id = r.id
  AND (
    r.household_id IS NULL
  );

-- ================================================================
-- 4. ROLLBACK GUARD — Resident not-null requirement
-- ================================================================

-- v7 schema requires residents.household_id NOT NULL.
-- If ANY resident (active or inactive) cannot be reconstructed from
-- current Residency Period state, rollback must fail.

DO $$
DECLARE
    unresolvable int;
BEGIN
    SELECT COUNT(*) INTO unresolvable
    FROM residents
    WHERE household_id IS NULL;

    IF unresolvable > 0 THEN
        RAISE EXCEPTION
            'Migration 008 rollback guard failed: % resident(s) cannot be mapped to a Household during rollback. Rollback from normalized state requires all residents to have a valid Residency Period or provenance mapping.',
            unresolvable;
    END IF;
END $$;

-- ================================================================
-- 5. RESTORE v7 CONSTRAINTS
-- ================================================================

ALTER TABLE residents ALTER COLUMN household_id SET NOT NULL;
ALTER TABLE residents ADD CONSTRAINT fk_residents_household FOREIGN KEY (household_id) REFERENCES households(id);

CREATE INDEX IF NOT EXISTS idx_residents_household_id
    ON residents (household_id);

CREATE INDEX IF NOT EXISTS idx_residents_rt_household_active
    ON residents (rt_id, household_id, is_active);

-- ================================================================
-- 6. DROP MIGRATION ROLLBACK BRIDGES (must survive until reconstruction complete)
-- ================================================================

DROP TABLE IF EXISTS migration_008_resident_legacy_map;

DROP TABLE IF EXISTS migration_008_household_physical_house_map;

-- ================================================================
-- 7. DROP TEMPORAL TABLES (reverse dependency order)
-- ================================================================

DROP INDEX IF EXISTS idx_residency_period_current_resident;

DROP TABLE IF EXISTS residency_periods;

ALTER TABLE household_occupancies DROP CONSTRAINT IF EXISTS no_occupancy_overlap_known;
DROP INDEX IF EXISTS idx_household_occupancy_current_house;

DROP TABLE IF EXISTS household_occupancies;

DROP INDEX IF EXISTS idx_physical_houses_active_rt_hn;

DROP TABLE IF EXISTS physical_houses;
