-- Rollback 094: Drop style_config from banners, activities, and announcements

BEGIN;

ALTER TABLE banners
    DROP COLUMN IF EXISTS style_config;

ALTER TABLE activities
    DROP COLUMN IF EXISTS style_config;

ALTER TABLE announcements
    DROP COLUMN IF EXISTS style_config;

COMMIT;
