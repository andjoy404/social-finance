-- 038_device_refresh_tokens.up.sql
--
-- Add device_id to refresh_tokens for multi-device session tracking.
-- This allows users to see and revoke active sessions per device.

ALTER TABLE refresh_tokens
    ADD COLUMN device_id TEXT NULL;

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_device_id
    ON refresh_tokens (device_id) WHERE device_id IS NOT NULL;

COMMENT ON COLUMN refresh_tokens.device_id IS
    'Device identifier from the client (e.g. Android ID / Firebase Installations ID). NULL for legacy tokens.';
