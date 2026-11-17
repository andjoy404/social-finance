-- Add execution error detail and execution summary to notification_schedules
-- to provide transparent, safe error reasons and delivery statistics to RT administrators.
ALTER TABLE notification_schedules
    ADD COLUMN IF NOT EXISTS last_executed_error TEXT,
    ADD COLUMN IF NOT EXISTS last_executed_summary JSONB;

