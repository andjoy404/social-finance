-- Migration 061: Support RW notification logs by allowing rt_id to be nullable and adding rw_id
ALTER TABLE notification_logs ALTER COLUMN rt_id DROP NOT NULL;
ALTER TABLE notification_logs ADD COLUMN IF NOT EXISTS rw_id UUID REFERENCES rws(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_notification_logs_rw_id ON notification_logs(rw_id) WHERE rw_id IS NOT NULL;

