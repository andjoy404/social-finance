-- Migration 094: Add style_config to announcements, activities, and banners
-- Supports custom background (neon color gradient / glow or cropped image)
-- across Pengumuman, Kegiatan, and active Beranda banner snapshots.

BEGIN;

ALTER TABLE announcements
    ADD COLUMN IF NOT EXISTS style_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE activities
    ADD COLUMN IF NOT EXISTS style_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE banners
    ADD COLUMN IF NOT EXISTS style_config JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMIT;
