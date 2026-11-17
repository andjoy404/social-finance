-- Migration 054: Add is_default column to distinguish seeded default data
-- from user-customised data in notification_templates and notification_schedules.
--
-- This allows rollback (migration 053 down) to delete only seeded default
-- rows without touching user-customised templates or schedules.

-- notification_templates: mark default templates
ALTER TABLE notification_templates
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;

-- notification_schedules: mark default schedules
ALTER TABLE notification_schedules
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;

-- Indexes for efficient lookup of default rows
CREATE INDEX IF NOT EXISTS idx_notif_templates_is_default ON notification_templates(is_default);
CREATE INDEX IF NOT EXISTS idx_notif_schedules_is_default ON notification_schedules(is_default);
