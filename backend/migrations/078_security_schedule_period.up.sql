-- Migration 078: Group daily security schedule rows into editable inclusive date periods.
ALTER TABLE security_schedules
    ADD COLUMN IF NOT EXISTS assignment_id UUID,
    ADD COLUMN IF NOT EXISTS assignment_start_date DATE,
    ADD COLUMN IF NOT EXISTS assignment_end_date DATE;

-- Existing schedules are independent one-day assignments.
UPDATE security_schedules
SET assignment_id = COALESCE(assignment_id, id),
    assignment_start_date = COALESCE(assignment_start_date, shift_date),
    assignment_end_date = COALESCE(assignment_end_date, shift_date);

ALTER TABLE security_schedules
    ALTER COLUMN assignment_id SET NOT NULL,
    ALTER COLUMN assignment_start_date SET NOT NULL,
    ALTER COLUMN assignment_end_date SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_security_schedules_assignment
    ON security_schedules(rt_id, assignment_id, shift_date, status);
CREATE INDEX IF NOT EXISTS idx_security_schedules_assignment_period
    ON security_schedules(rt_id, assignment_start_date, assignment_end_date, status);
