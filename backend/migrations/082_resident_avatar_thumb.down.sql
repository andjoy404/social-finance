-- Migration 082 Rollback: Resident profile photo thumbnail
-- Drops only the newly added nullable column. Does NOT touch existing data
-- (avatar_path), photo objects (original or thumbnail), or other tables.

ALTER TABLE residents
    DROP COLUMN IF EXISTS avatar_thumb_path;

DROP INDEX IF EXISTS idx_residents_avatar_thumb_path;
