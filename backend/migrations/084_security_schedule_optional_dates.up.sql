-- Migration 084 (UP): Make security schedule assignment dates nullable.
-- Start Date and End Date are truly optional. NULL values are preserved as NULL.
-- Existing rows that already have dates are unaffected.
ALTER TABLE security_schedules
    ALTER COLUMN assignment_start_date DROP NOT NULL,
    ALTER COLUMN assignment_end_date DROP NOT NULL;
