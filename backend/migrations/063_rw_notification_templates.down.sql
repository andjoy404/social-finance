-- Migration 063 down: Remove rw_id column from notification_templates

-- Remove check constraint first
ALTER TABLE notification_templates
DROP CONSTRAINT IF EXISTS chk_notification_templates_rt_or_rw;

-- Drop foreign key
ALTER TABLE notification_templates
DROP CONSTRAINT IF EXISTS fk_notification_templates_rw_id;

-- Drop index
DROP INDEX IF EXISTS idx_notification_templates_rw_id;

-- Drop column
ALTER TABLE notification_templates
DROP COLUMN IF EXISTS rw_id;

-- Restore rt_id NOT NULL constraint
ALTER TABLE notification_templates
ALTER COLUMN rt_id SET NOT NULL;
