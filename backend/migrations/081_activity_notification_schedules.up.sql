-- Migration 081: Link notification_schedules to activities for scheduled reminders (one-shot).
--
-- notification_schedules supports both recurring (day-of-month) schedules and, after this
-- migration, one-shot schedules bound to a specific activity with an absolute execution time
-- (e.g. H-3, H-1, Hari H reminder). Binding to activity_id + execution_at allows the scheduler
-- to enforce lifecycle guards (cancelled/completed activities never trigger delivery) and to
-- cancel all pending executions when the activity ends or is cancelled.

ALTER TABLE notification_schedules
    ADD COLUMN IF NOT EXISTS activity_id UUID NULL REFERENCES activities(id) ON DELETE SET NULL;

ALTER TABLE notification_schedules
    ADD COLUMN IF NOT EXISTS execution_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_notification_schedules_activity
    ON notification_schedules(activity_id)
    WHERE activity_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notification_schedules_execution_at
    ON notification_schedules(execution_at)
    WHERE execution_at IS NOT NULL AND is_active = true;
