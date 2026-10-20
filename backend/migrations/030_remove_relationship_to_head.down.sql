-- Migration 030: Remove relationship_to_head from residency_periods (down)

-- Restore the removed column and provenance table for rollback.
-- These are recreated so migration 008's DOWN can still operate.

ALTER TABLE residency_periods ADD COLUMN IF NOT EXISTS relationship_to_head text;

CREATE TABLE IF NOT EXISTS migration_008_resident_legacy_map (
    resident_id               uuid NOT NULL REFERENCES residents(id),
    legacy_household_id       uuid NOT NULL REFERENCES households(id),
    legacy_relationship_to_head text,
    UNIQUE (resident_id)
);
