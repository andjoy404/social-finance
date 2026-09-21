-- 008_temporal_household_model.up.sql
--
-- Frozen Option B+ contract.
-- Phase 4: Physical House / Household Occupancy / Residency Period model.
--
-- Temporal semantics:
--   - [start_date, end_date) half-open interval.
--   - NULL start_date used for legacy-unknown starts.
--   - NULL end_date means "current".
--   - Temporal tables have NO is_active column.
--   - DO NOT fabricate or synthesize dates.

-- ================================================================
-- 0. MINIMAL EXTENSION (overlap proof)
-- ================================================================

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ================================================================
-- 1. PHYSICAL HOUSES
-- ================================================================

CREATE TABLE physical_houses (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id        uuid NOT NULL REFERENCES rts(id),
    house_number text NOT NULL,
    address      text,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE physical_houses IS
    'Physical dwelling unit. House number and address authority.';

CREATE UNIQUE INDEX idx_physical_houses_active_rt_hn
    ON physical_houses (rt_id, house_number)
    WHERE is_active = true;

-- ================================================================
-- 2. HOUSEHOLD OCCUPANCIES
-- ================================================================

CREATE TABLE household_occupancies (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    physical_house_id  uuid NOT NULL REFERENCES physical_houses(id),
    household_id       uuid NOT NULL REFERENCES households(id),
    occupancy_status   text NOT NULL,
    start_date         date,
    end_date           date,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE household_occupancies IS
    'Current or historical occupancy of a physical house.';

COMMENT ON COLUMN household_occupancies.occupancy_status IS
    'OWNER or TENANT.';
COMMENT ON COLUMN household_occupancies.start_date IS
    '[start_date, end_date). NULL = unknown start (legacy).';
COMMENT ON COLUMN household_occupancies.end_date IS
    'NULL = current. DO NOT fabricate dates.';

ALTER TABLE household_occupancies
    ADD CONSTRAINT chk_occupancy_status
        CHECK (occupancy_status IN ('OWNER', 'TENANT'));

ALTER TABLE household_occupancies
    ADD CONSTRAINT chk_occupancy_date
        CHECK (
            start_date IS NULL OR end_date IS NULL
            OR end_date > start_date
        );

CREATE UNIQUE INDEX idx_household_occupancy_current_house
    ON household_occupancies (physical_house_id)
    WHERE end_date IS NULL;

-- Known-period overlap protection: for known start dates,
-- [start_date, end_date) must not overlap on the same house.
-- NULL start_date rows are excluded from overlap enforcement.

ALTER TABLE household_occupancies
    ADD CONSTRAINT no_occupancy_overlap_known
        EXCLUDE USING gist (
            physical_house_id WITH =,
            daterange(start_date, end_date, '[)') WITH &&
        )
        WHERE (start_date IS NOT NULL);

-- ================================================================
-- 3. RESIDENCY PERIODS
-- ================================================================

CREATE TABLE residency_periods (
    id                         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    resident_id                uuid NOT NULL REFERENCES residents(id),
    household_occupancy_id     uuid NOT NULL REFERENCES household_occupancies(id),
    relationship_to_head       text,
    start_date                 date,
    end_date                   date,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE residency_periods IS
    'Current or historical residency linkage.';

COMMENT ON COLUMN residency_periods.start_date IS
    '[start_date, end_date). NULL = unknown start (legacy).';
COMMENT ON COLUMN residency_periods.end_date IS
    'NULL = current. DO NOT fabricate dates.';

ALTER TABLE residency_periods
    ADD CONSTRAINT chk_residency_date
        CHECK (
            start_date IS NULL OR end_date IS NULL
            OR end_date > start_date
        );

CREATE UNIQUE INDEX idx_residency_period_current_resident
    ON residency_periods (resident_id)
    WHERE end_date IS NULL;

-- ================================================================
-- 4. ROLLBACK BRIDGE: Household -> Physical House mapping
-- ================================================================

-- Migration-local bridge for DOWN rollback.
-- NOT an application/domain table.  Dropped during DOWN after
-- all legacy data reconstruction is complete.
-- One row per legacy Household that was mapped to a Physical House
-- during migration.  Needed for truthful rollback especially for
-- inactive Household instances that intentionally receive no
-- Household Occupancy.

CREATE TABLE migration_008_household_physical_house_map (
    household_id           uuid NOT NULL REFERENCES households(id),
    physical_house_id      uuid NOT NULL REFERENCES physical_houses(id),
    legacy_house_number    text,
    legacy_address         text,
    legacy_occupancy_status text
);

-- ================================================================
-- 4b. ROLLBACK PROVENANCE: Resident -> Legacy mapping
-- ================================================================

-- Migration-local provenance for DOWN rollback.
-- NOT an application/domain table.  Dropped during DOWN after
-- all legacy data reconstruction is complete.
-- One row per legacy Resident, including inactive Residents that
-- intentionally receive no Residency Period during UP.

CREATE TABLE migration_008_resident_legacy_map (
    resident_id               uuid NOT NULL REFERENCES residents(id),
    legacy_household_id       uuid NOT NULL REFERENCES households(id),
    legacy_relationship_to_head text,
    UNIQUE (resident_id)
);

-- ================================================================
-- 5. CONFLICT GUARD — LEGACY ADDRESS AMBIGUITY
-- ================================================================

-- Detect groups where Households mapping to the same Physical House
-- have conflicting non-NULL legacy addresses.  Must run BEFORE any
-- destructive Household normalization so the diagnosis is possible.

DO $$
DECLARE
    conflict_details TEXT;
BEGIN
    SELECT string_agg(
        format('rt_id=%s; house_number=%s', rt_id, house_number),
        '; ' ORDER BY rt_id::text, house_number
    )
    INTO conflict_details
    FROM (
        SELECT
            rt_id,
            TRIM(h.house_number) AS house_number
        FROM households h
        WHERE h.house_number IS NOT NULL
          AND TRIM(h.house_number) <> ''
        GROUP BY rt_id, TRIM(h.house_number)
        HAVING COUNT(DISTINCT h.address) > 1
    ) sub;

    IF conflict_details IS NOT NULL THEN
        RAISE EXCEPTION
            'Migration 008: %',
            'Conflicting legacy addresses detected groups: ' || conflict_details;
    END IF;
END $$;

-- ================================================================
-- 6. LEGACY MIGRATION — PHYSICAL HOUSES
-- ================================================================

-- One physical house per unique (rt_id, TRIM(house_number)).
-- is_active = true if any legacy household for this house is active.
-- Address is the single validated non-NULL value after the conflict
-- guard above guarantees at most one distinct non-NULL value.
-- Inactive-only groups get is_active = FALSE.

WITH house_groups AS (
    SELECT
        rt_id,
        TRIM(h.house_number) AS house_number,
        BOOL_OR(h.is_active) AS has_active,
        MAX(h.address) AS address
    FROM households h
    WHERE h.house_number IS NOT NULL
      AND TRIM(h.house_number) <> ''
    GROUP BY rt_id, TRIM(h.house_number)
)
INSERT INTO physical_houses (id, rt_id, house_number, address, is_active)
SELECT gen_random_uuid(), rt_id, house_number, address, has_active
FROM house_groups;

-- ================================================================
-- 7. LEGACY MIGRATION — HOUSEHOLD OCCUPANCIES (ACTIVE)
-- ================================================================

-- Each ACTIVE legacy household with a valid house_number gets a
-- CURRENT household occupancy.  Inactive households do NOT receive
-- a current occupancy (unknown historical end date).
--
-- Every active household that had a valid house_number MUST get a
-- household_occupancy; silently skipping would indicate a migration
-- bug or an un-fixed preflight blocker.  An explicit guard below
-- raises an error if counts do not match, causing an atomic ROLLBACK.

INSERT INTO household_occupancies (
    id, physical_house_id, household_id,
    occupancy_status, start_date, end_date
)
SELECT
    gen_random_uuid(),
    ph.id,
    h.id,
    h.occupancy_status,
    NULL,
    NULL
FROM households h
JOIN physical_houses ph
  ON  ph.rt_id       = h.rt_id
  AND ph.house_number = TRIM(h.house_number)
  AND ph.is_active    = true
WHERE h.is_active = true;

-- Guard: every active household with a valid house_number must have
-- received a household_occupancy.  Mismatch = un-fixed blocker or bug.

DO $$
DECLARE
    expected_active   int;
    migrated_count    int;
BEGIN
    SELECT COUNT(*) INTO expected_active
    FROM households
    WHERE is_active = true
      AND house_number IS NOT NULL
      AND TRIM(house_number) <> '';

    SELECT COUNT(*) INTO migrated_count FROM household_occupancies;

    IF expected_active != migrated_count THEN
        RAISE EXCEPTION
            'Migration 008 guard failed: expected % active households with valid house_number, migrated %',
            expected_active, migrated_count;
    END IF;
END $$;

-- ================================================================
-- 7. LEGACY MIGRATION — ROLLBACK BRIDGE MAPPING
-- ================================================================

-- Populate the rollback bridge with exact Household -> Physical House
-- mapping for ALL legacy households that received a Physical House.

INSERT INTO migration_008_household_physical_house_map (household_id, physical_house_id, legacy_house_number, legacy_address, legacy_occupancy_status)
SELECT
    h.id,
    ph.id,
    h.house_number,
    h.address,
    h.occupancy_status
FROM households h
JOIN physical_houses ph
  ON  ph.rt_id       = h.rt_id
  AND ph.house_number = TRIM(h.house_number)
WHERE h.house_number IS NOT NULL
  AND TRIM(h.house_number) <> '';

-- ================================================================
-- 8. LEGACY MIGRATION — RESIDENCY PERIODS (ACTIVE)
-- ================================================================

-- Each ACTIVE resident whose household has a current household_occupancy
-- gets a CURRENT residency period.  Inactive residents do NOT receive
-- a fake current residency period.  Active residents whose household has
-- no current household_occupancy (e.g., linked to an inactive household
-- that was skipped above) are not assigned a residency period.

INSERT INTO residency_periods (
    id, resident_id, household_occupancy_id,
    relationship_to_head, start_date, end_date
)
SELECT
    gen_random_uuid(),
    r.id,
    ho.id,
    r.relationship_to_head,
    NULL,
    NULL
FROM residents r
JOIN household_occupancies ho
  ON ho.household_id = r.household_id
WHERE r.is_active = true;

-- Guard: every active resident must resolve through a migrated
-- Household Occupancy.  The household occupancy guard (Section 6)
-- guarantees every active legacy Household has an occupancy;
-- therefore the JOIN above should cover every active resident.

DO $$
DECLARE
    expected_residents   int;
    migrated_residents   int;
BEGIN
    SELECT COUNT(*) INTO expected_residents
    FROM residents
    WHERE is_active = true;

    SELECT COUNT(*) INTO migrated_residents
    FROM residency_periods;

    IF expected_residents != migrated_residents THEN
        RAISE EXCEPTION
            'Migration 008 residency guard failed: expected % active residents with current occupancy, migrated %',
            expected_residents, migrated_residents;
    END IF;
END $$;

-- ================================================================
-- 8b. LEGACY MIGRATION — RESIDENT PROVENANCE MAPPING
-- ================================================================

-- Capture exact v7 fields for ALL Residents BEFORE destructive
-- normalization removes them.  This covers active and inactive
-- Residents equally; no Residency Period dependency.

INSERT INTO migration_008_resident_legacy_map (resident_id, legacy_household_id, legacy_relationship_to_head)
SELECT id, household_id, relationship_to_head FROM residents;

-- Guard: every legacy Resident must have provenance before columns are dropped.

DO $$
DECLARE
    expected_provenance        int;
    actual_provenance          int;
    distinct_provenance        int;
BEGIN
    SELECT COUNT(*) INTO expected_provenance FROM residents;
    SELECT COUNT(*) INTO actual_provenance FROM migration_008_resident_legacy_map;
    SELECT COUNT(DISTINCT resident_id) INTO distinct_provenance FROM migration_008_resident_legacy_map;

    IF expected_provenance != actual_provenance THEN
        RAISE EXCEPTION
            'Migration 008 provenance guard failed: expected % residents, captured %',
            expected_provenance, actual_provenance;
    END IF;

    IF expected_provenance != distinct_provenance THEN
        RAISE EXCEPTION
            'Migration 008 provenance distinct guard failed: expected % distinct resident_id, found %',
            expected_provenance, distinct_provenance;
    END IF;
END $$;

-- ================================================================
-- 9. NORMALIZE HOUSEHOLDS
-- ================================================================

-- House number, address, occupancy_status authority moves out.

ALTER TABLE households DROP COLUMN IF EXISTS house_number;
ALTER TABLE households DROP COLUMN IF EXISTS address;
ALTER TABLE households DROP COLUMN IF EXISTS occupancy_status;

-- ================================================================
-- 10. NORMALIZE RESIDENTS
-- ================================================================

-- Household linkage and relationship_to_head move out.

DROP INDEX IF EXISTS idx_residents_household_id;
DROP INDEX IF EXISTS idx_residents_rt_household_active;

ALTER TABLE residents DROP COLUMN IF EXISTS household_id;
ALTER TABLE residents DROP COLUMN IF EXISTS relationship_to_head;
