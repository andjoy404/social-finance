-- Migration 082: Resident profile photo thumbnail (avatar_thumb_path)
-- Adds nullable avatar_thumb_path column to residents table for the optimized
-- thumbnail-based avatar pipeline. All image binaries remain in object storage;
-- only storage keys are persisted. Existing photos are untouched; thumbnail
-- population is handled by a separate async backfill tool.

ALTER TABLE residents
    ADD COLUMN IF NOT EXISTS avatar_thumb_path text;

CREATE INDEX IF NOT EXISTS idx_residents_avatar_thumb_path
    ON residents USING BTREE (avatar_thumb_path)
    WHERE avatar_thumb_path IS NOT NULL;
