-- Migration 088 Down: Revert Date-Independent Security Shifts

ALTER TABLE security_attendance_records
    DROP CONSTRAINT IF EXISTS uniq_security_attendance_schedule_date;

DROP INDEX IF EXISTS uniq_security_attendance_user_open_session;
DROP INDEX IF EXISTS idx_security_attendance_rt_checkin;
DROP INDEX IF EXISTS idx_security_attendance_open_sessions;
DROP INDEX IF EXISTS idx_security_schedules_active;
DROP INDEX IF EXISTS uniq_security_schedules_active_assignment;
DROP INDEX IF EXISTS uniq_security_schedules_active_slot;

ALTER TABLE security_schedules
    DROP COLUMN IF EXISTS is_active;
