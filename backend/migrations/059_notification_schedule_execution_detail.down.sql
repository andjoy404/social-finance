ALTER TABLE notification_schedules
    DROP COLUMN IF EXISTS last_executed_summary,
    DROP COLUMN IF EXISTS last_executed_error;

