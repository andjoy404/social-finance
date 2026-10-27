-- 038_device_refresh_tokens.down.sql
-- Remove device_id from refresh_tokens.

ALTER TABLE refresh_tokens
    DROP COLUMN IF EXISTS device_id;
