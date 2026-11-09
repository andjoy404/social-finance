-- Migration 063: Add rw_id column to notification_templates for RW-scoped templates
ALTER TABLE notification_templates ALTER COLUMN rt_id DROP NOT NULL;
ALTER TABLE notification_templates ADD COLUMN IF NOT EXISTS rw_id UUID;
ALTER TABLE notification_templates DROP CONSTRAINT IF EXISTS fk_notification_templates_rw_id;
ALTER TABLE notification_templates ADD CONSTRAINT fk_notification_templates_rw_id FOREIGN KEY (rw_id) REFERENCES rws(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_notification_templates_rw_id ON notification_templates(rw_id);
ALTER TABLE notification_templates DROP CONSTRAINT IF EXISTS chk_notification_templates_rt_or_rw;
ALTER TABLE notification_templates ADD CONSTRAINT chk_notification_templates_rt_or_rw CHECK (
    (rt_id IS NOT NULL AND rw_id IS NULL) OR
    (rt_id IS NULL AND rw_id IS NOT NULL)
);
