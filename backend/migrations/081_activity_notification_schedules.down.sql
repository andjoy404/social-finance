-- Migration 081 down: drop activity-linked one-shot scheduling columns and indexes.

DROP INDEX IF EXISTS idx_notification_schedules_execution_at;
DROP INDEX IF EXISTS idx_notification_schedules_activity;

ALTER TABLE notification_schedules
    DROP COLUMN IF EXISTS execution_at;

ALTER TABLE notification_schedules
    DROP COLUMN IF EXISTS activity_id;
