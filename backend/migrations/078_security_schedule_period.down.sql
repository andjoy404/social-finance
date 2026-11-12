-- Migration 078 Rollback: daily rows remain valid as one-day assignments.
DROP INDEX IF EXISTS idx_security_schedules_assignment_period;
DROP INDEX IF EXISTS idx_security_schedules_assignment;
ALTER TABLE security_schedules
    DROP COLUMN IF EXISTS assignment_end_date,
    DROP COLUMN IF EXISTS assignment_start_date,
    DROP COLUMN IF EXISTS assignment_id;
