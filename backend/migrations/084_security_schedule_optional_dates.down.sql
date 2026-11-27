-- Migration 084 (DOWN): Restore NOT NULL constraint on security schedule assignment dates.
-- Existing NULL rows are backfilled to shift_date before re-applying the constraint.
UPDATE security_schedules
SET assignment_start_date = COALESCE(assignment_start_date, shift_date),
    assignment_end_date = COALESCE(assignment_end_date, shift_date)
WHERE assignment_start_date IS NULL OR assignment_end_date IS NULL;

ALTER TABLE security_schedules
    ALTER COLUMN assignment_start_date SET NOT NULL,
    ALTER COLUMN assignment_end_date SET NOT NULL;
