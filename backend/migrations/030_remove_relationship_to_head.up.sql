-- Migration 030: Remove relationship_to_head from residency_periods

-- The relationship_to_head column is no longer part of the domain model.
-- Legacy data was preserved during migration 008 in
-- migration_008_resident_legacy_map.legacy_relationship_to_head.

DROP TABLE IF EXISTS migration_008_resident_legacy_map;

ALTER TABLE residency_periods DROP COLUMN IF EXISTS relationship_to_head;
