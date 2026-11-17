-- Migration 060: Add notification_schedule_executions table for schedule configuration versioning and same-day re-execution.
--
-- Tracks schedule executions by (schedule_id, exec_date, config_hash).
-- Enables safe re-execution when an RT administrator updates schedule time or
-- configuration on the same day, preventing duplicate deliveries while allowing
-- updated schedules to trigger at their newly configured times.

CREATE TABLE IF NOT EXISTS notification_schedule_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL REFERENCES notification_schedules(id) ON DELETE CASCADE,
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    exec_date DATE NOT NULL,
    config_hash VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'success',
    error_detail TEXT,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_schedule_execution UNIQUE (schedule_id, exec_date, config_hash)
);

CREATE INDEX IF NOT EXISTS idx_sched_exec_rt ON notification_schedule_executions(rt_id);
CREATE INDEX IF NOT EXISTS idx_sched_exec_date ON notification_schedule_executions(exec_date);
CREATE INDEX IF NOT EXISTS idx_sched_exec_lookup ON notification_schedule_executions(schedule_id, exec_date, config_hash);

-- Backfill legacy execution entries from notification_schedules where last_executed_at is not null
INSERT INTO notification_schedule_executions (
    schedule_id,
    rt_id,
    exec_date,
    config_hash,
    status,
    error_detail,
    summary,
    executed_at
)
SELECT
    s.id,
    s.rt_id,
    (s.last_executed_at AT TIME ZONE COALESCE(s.timezone, 'Asia/Jakarta'))::date,
    'legacy_initial',
    COALESCE(s.last_executed_status, 'success'),
    s.last_executed_error,
    COALESCE(s.last_executed_summary, '{}'::jsonb),
    s.last_executed_at
FROM notification_schedules s
WHERE s.last_executed_at IS NOT NULL
ON CONFLICT (schedule_id, exec_date, config_hash) DO NOTHING;

