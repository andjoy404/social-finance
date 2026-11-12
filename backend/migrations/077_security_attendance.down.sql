-- Migration 077 Rollback: Security Attendance Module

DELETE FROM position_permissions
WHERE permission_id IN (
    SELECT id FROM permission_types WHERE code IN (
        'security.attendance',
        'security.schedule.read',
        'security.schedule.write',
        'security.device.manage',
        'security.correction',
        'security.guest_book'
    )
);

DELETE FROM permission_types
WHERE code IN (
    'security.attendance',
    'security.schedule.read',
    'security.schedule.write',
    'security.device.manage',
    'security.correction',
    'security.guest_book'
);

DROP TABLE IF EXISTS security_guest_books;
DROP TABLE IF EXISTS security_attendance_corrections;
DROP TABLE IF EXISTS security_attendance_records;
DROP TABLE IF EXISTS security_qr_tokens;
DROP TABLE IF EXISTS security_shift_swaps;
DROP TABLE IF EXISTS security_schedules;
DROP TABLE IF EXISTS security_shift_configs;
DROP TABLE IF EXISTS security_devices;

