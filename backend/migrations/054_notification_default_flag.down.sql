-- Migration 054: Down
DROP INDEX IF EXISTS idx_notif_templates_is_default;
DROP INDEX IF EXISTS idx_notif_schedules_is_default;
ALTER TABLE notification_templates DROP COLUMN IF EXISTS is_default;
ALTER TABLE notification_schedules DROP COLUMN IF EXISTS is_default;
