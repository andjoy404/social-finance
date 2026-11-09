-- Migration 061 down: Remove rw_id from notification_logs
DROP INDEX IF EXISTS idx_notification_logs_rw_id;
ALTER TABLE notification_logs DROP COLUMN IF EXISTS rw_id;

