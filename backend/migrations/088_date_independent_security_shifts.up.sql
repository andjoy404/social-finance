-- Migration 088: Restructure Security Schedules & Attendance to Date-Independent Shifts
-- Shifts are the primary unit of continuous assignment and attendance.
-- Calendar dates are removed from assignment creation, initialization, and scheduling.
-- Dates are purely automatic timestamps on actual attendance check-in/out records.

-- 1. Security Schedules: continuous date-independent shift assignment
ALTER TABLE security_schedules
    ALTER COLUMN shift_date DROP NOT NULL,
    ALTER COLUMN scheduled_start_at DROP NOT NULL,
    ALTER COLUMN scheduled_end_at DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- Unique active continuous shift assignment per officer in an RT (date-independent)
DROP INDEX IF EXISTS uniq_security_schedules_active_assignment;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_security_schedules_active_assignment
    ON security_schedules(rt_id, user_id, shift_config_id)
    WHERE is_active = true AND status = 'scheduled' AND shift_date IS NULL;

-- Retain slot uniqueness for date-specified rows if present
CREATE UNIQUE INDEX IF NOT EXISTS uniq_security_schedules_active_slot
    ON security_schedules(rt_id, user_id, shift_date)
    WHERE is_active = true AND status = 'scheduled' AND shift_date IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_security_schedules_active
    ON security_schedules(rt_id, is_active, status);

-- 2. Security Attendance Records: decouple schedule_id uniqueness so multiple attendance records can link to continuous assignment
ALTER TABLE security_attendance_records
    DROP CONSTRAINT IF EXISTS security_attendance_records_schedule_id_key;

-- Make shift_date nullable if not yet checked in, though it is derived automatically on check-in
ALTER TABLE security_attendance_records
    ALTER COLUMN shift_date DROP NOT NULL;

-- Unique attendance session per assignment per calendar date
ALTER TABLE security_attendance_records
    ADD CONSTRAINT uniq_security_attendance_schedule_date
    UNIQUE (schedule_id, shift_date);

-- Efficient lookup index for open attendance (sedang bertugas)
CREATE INDEX IF NOT EXISTS idx_security_attendance_open_sessions
    ON security_attendance_records(user_id, status)
    WHERE checked_in_at IS NOT NULL AND checked_out_at IS NULL AND marked_missing_checkout_at IS NULL;

-- Strict database-level guarantee: an officer can NEVER have multiple open attendance sessions simultaneously
DROP INDEX IF EXISTS uniq_security_attendance_user_open_session;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_security_attendance_user_open_session
    ON security_attendance_records(user_id)
    WHERE checked_in_at IS NOT NULL AND checked_out_at IS NULL AND marked_missing_checkout_at IS NULL AND status IN ('hadir', 'terlambat');

CREATE INDEX IF NOT EXISTS idx_security_attendance_rt_checkin
    ON security_attendance_records(rt_id, checked_in_at DESC);

-- 3. Security Pos Guest Book: decouple hard dependency on physical device presence
ALTER TABLE security_guest_books
    ALTER COLUMN device_id DROP NOT NULL;

ALTER TABLE security_guest_books
    DROP CONSTRAINT IF EXISTS security_guest_books_device_id_fkey;

ALTER TABLE security_guest_books
    ADD CONSTRAINT security_guest_books_device_id_fkey
    FOREIGN KEY (device_id) REFERENCES security_devices(id) ON DELETE SET NULL;
