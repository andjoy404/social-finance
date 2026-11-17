-- Migration 077: Security Attendance Module (Pos Devices, Shifts, Schedules, Swaps, QR Tokens, Attendance, Corrections, Guest Book, Permissions)

-- 1. Registered Security Pos Devices
CREATE TABLE IF NOT EXISTS security_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    device_name TEXT NOT NULL,
    location_label TEXT NOT NULL DEFAULT 'Pos Keamanan Utama',
    activation_code_hash TEXT,
    activation_expires_at TIMESTAMPTZ,
    token_hash TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    activated_at TIMESTAMPTZ,
    last_active_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoked_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_devices_rt_active
    ON security_devices(rt_id, is_active);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_security_devices_token_hash
    ON security_devices(token_hash)
    WHERE token_hash IS NOT NULL;

-- 2. Flexible Shift Configurations (N shifts per RT, versioned snapshots)
CREATE TABLE IF NOT EXISTS security_shift_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    crosses_midnight BOOLEAN NOT NULL DEFAULT false,
    lateness_tolerance_minutes INT NOT NULL DEFAULT 15 CHECK (lateness_tolerance_minutes >= 0 AND lateness_tolerance_minutes <= 180),
    check_in_window_before_minutes INT NOT NULL DEFAULT 60 CHECK (check_in_window_before_minutes >= 0 AND check_in_window_before_minutes <= 360),
    check_in_window_after_minutes INT NOT NULL DEFAULT 120 CHECK (check_in_window_after_minutes >= 0 AND check_in_window_after_minutes <= 480),
    absent_cutoff_minutes INT NOT NULL DEFAULT 120 CHECK (absent_cutoff_minutes >= 0 AND absent_cutoff_minutes <= 480),
    missing_checkout_cutoff_minutes INT NOT NULL DEFAULT 180 CHECK (missing_checkout_cutoff_minutes >= 0 AND missing_checkout_cutoff_minutes <= 720),
    version INT NOT NULL DEFAULT 1 CHECK (version >= 1),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_security_shift_configs_rt_name_active
    ON security_shift_configs(rt_id, LOWER(name))
    WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_security_shift_configs_rt_active
    ON security_shift_configs(rt_id, is_active);

-- 3. Officer Shift Schedules (stores immutable snapshot & computed Asia/Jakarta instants)
CREATE TABLE IF NOT EXISTS security_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    membership_id UUID NOT NULL REFERENCES user_rt_memberships(id) ON DELETE RESTRICT,
    shift_config_id UUID NOT NULL REFERENCES security_shift_configs(id) ON DELETE RESTRICT,
    shift_version INT NOT NULL DEFAULT 1 CHECK (shift_version >= 1),
    shift_date DATE NOT NULL,
    shift_name_snapshot TEXT NOT NULL,
    start_time_snapshot TIME NOT NULL,
    end_time_snapshot TIME NOT NULL,
    crosses_midnight_snapshot BOOLEAN NOT NULL DEFAULT false,
    lateness_tolerance_minutes INT NOT NULL,
    check_in_window_before_minutes INT NOT NULL,
    check_in_window_after_minutes INT NOT NULL,
    absent_cutoff_minutes INT NOT NULL,
    missing_checkout_cutoff_minutes INT NOT NULL,
    scheduled_start_at TIMESTAMPTZ NOT NULL,
    scheduled_end_at TIMESTAMPTZ NOT NULL,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'cancelled')),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_security_schedules_active_slot
    ON security_schedules(rt_id, user_id, shift_date, shift_config_id)
    WHERE status = 'scheduled';

CREATE INDEX IF NOT EXISTS idx_security_schedules_rt_date
    ON security_schedules(rt_id, shift_date, status);

CREATE INDEX IF NOT EXISTS idx_security_schedules_user_window
    ON security_schedules(user_id, scheduled_start_at, scheduled_end_at);

-- 4. Two-Step Shift Swap Requests
CREATE TABLE IF NOT EXISTS security_shift_swaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    requester_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    target_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    requester_schedule_id UUID NOT NULL REFERENCES security_schedules(id) ON DELETE RESTRICT,
    target_schedule_id UUID NOT NULL REFERENCES security_schedules(id) ON DELETE RESTRICT,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending_target'
        CHECK (status IN ('pending_target', 'rejected_target', 'pending_ketua', 'approved', 'rejected_ketua', 'cancelled')),
    target_responded_at TIMESTAMPTZ,
    target_note TEXT,
    ketua_decided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ketua_decided_at TIMESTAMPTZ,
    ketua_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_security_swap_diff_users CHECK (requester_user_id <> target_user_id),
    CONSTRAINT chk_security_swap_diff_schedules CHECK (requester_schedule_id <> target_schedule_id)
);

CREATE INDEX IF NOT EXISTS idx_security_swaps_rt_status
    ON security_shift_swaps(rt_id, status);

CREATE INDEX IF NOT EXISTS idx_security_swaps_users
    ON security_shift_swaps(requester_user_id, target_user_id);

-- 5. Dynamic Single-Use QR Tokens (token stored SHA-256 hashed only)
CREATE TABLE IF NOT EXISTS security_qr_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    device_id UUID NOT NULL REFERENCES security_devices(id) ON DELETE RESTRICT,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    used_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    used_for_action TEXT CHECK (used_for_action IN ('check_in', 'check_out')),
    invalidated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_qr_tokens_device_created
    ON security_qr_tokens(device_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_qr_tokens_rt_expires
    ON security_qr_tokens(rt_id, expires_at);

-- 6. Immutable Attendance Ledger
CREATE TABLE IF NOT EXISTS security_attendance_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    schedule_id UUID NOT NULL UNIQUE REFERENCES security_schedules(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    shift_config_id UUID NOT NULL REFERENCES security_shift_configs(id) ON DELETE RESTRICT,
    shift_version INT NOT NULL DEFAULT 1,
    shift_date DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'belum_hadir'
        CHECK (status IN ('belum_hadir', 'hadir', 'terlambat', 'sudah_absen_pulang', 'tidak_hadir', 'belum_absen_pulang')),
    arrival_status TEXT CHECK (arrival_status IN ('hadir', 'terlambat')),
    late_minutes INT NOT NULL DEFAULT 0 CHECK (late_minutes >= 0),
    check_in_device_id UUID REFERENCES security_devices(id) ON DELETE SET NULL,
    check_out_device_id UUID REFERENCES security_devices(id) ON DELETE SET NULL,
    check_in_qr_token_id UUID REFERENCES security_qr_tokens(id) ON DELETE SET NULL,
    check_out_qr_token_id UUID REFERENCES security_qr_tokens(id) ON DELETE SET NULL,
    checked_in_at TIMESTAMPTZ,
    checked_out_at TIMESTAMPTZ,
    marked_absent_at TIMESTAMPTZ,
    marked_missing_checkout_at TIMESTAMPTZ,
    correction_locked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_attendance_rt_date_status
    ON security_attendance_records(rt_id, shift_date, status);

CREATE INDEX IF NOT EXISTS idx_security_attendance_user_date
    ON security_attendance_records(user_id, shift_date DESC);

-- 7. Maker-Checker Attendance Corrections
CREATE TABLE IF NOT EXISTS security_attendance_corrections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    attendance_record_id UUID NOT NULL REFERENCES security_attendance_records(id) ON DELETE RESTRICT,
    schedule_id UUID NOT NULL REFERENCES security_schedules(id) ON DELETE RESTRICT,
    officer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    requested_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    request_type TEXT NOT NULL
        CHECK (request_type IN ('status_change', 'time_adjustment', 'manual_check_in', 'manual_check_out')),
    original_status TEXT NOT NULL,
    original_checked_in_at TIMESTAMPTZ,
    original_checked_out_at TIMESTAMPTZ,
    requested_status TEXT NOT NULL
        CHECK (requested_status IN ('belum_hadir', 'hadir', 'terlambat', 'sudah_absen_pulang', 'tidak_hadir', 'belum_absen_pulang')),
    requested_checked_in_at TIMESTAMPTZ,
    requested_checked_out_at TIMESTAMPTZ,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    decided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ,
    decision_note TEXT,
    final_status TEXT,
    final_checked_in_at TIMESTAMPTZ,
    final_checked_out_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_corrections_rt_status
    ON security_attendance_corrections(rt_id, status);

CREATE INDEX IF NOT EXISTS idx_security_corrections_record
    ON security_attendance_corrections(attendance_record_id);

-- 8. Security Pos Guest Book
CREATE TABLE IF NOT EXISTS security_guest_books (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    device_id UUID NOT NULL REFERENCES security_devices(id) ON DELETE RESTRICT,
    guest_name TEXT NOT NULL,
    phone TEXT,
    purpose TEXT NOT NULL,
    host_household_id UUID REFERENCES household_occupancies(id) ON DELETE SET NULL,
    destination_label TEXT NOT NULL,
    vehicle_plate TEXT,
    notes TEXT,
    checked_in_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    checked_out_at TIMESTAMPTZ,
    checkout_device_id UUID REFERENCES security_devices(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_guest_books_rt_checkin
    ON security_guest_books(rt_id, checked_in_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_guest_books_rt_active
    ON security_guest_books(rt_id, checked_out_at);

-- 9. RBAC Permissions for Security Module
INSERT INTO permission_types (code, domain, operation) VALUES
    ('security.attendance', 'security', 'attendance'),
    ('security.schedule.read', 'security', 'read'),
    ('security.schedule.write', 'security', 'write'),
    ('security.device.manage', 'security', 'manage'),
    ('security.correction', 'security', 'correction'),
    ('security.guest_book', 'security', 'guest_book')
ON CONFLICT (code) DO UPDATE SET
    domain = EXCLUDED.domain,
    operation = EXCLUDED.operation;

-- Ketua RT receives full management permissions for RT Security
INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types
WHERE code IN (
    'security.schedule.read',
    'security.schedule.write',
    'security.device.manage',
    'security.correction',
    'security.guest_book'
)
ON CONFLICT (position, permission_id) DO NOTHING;

-- Security Officer ('keamanan') receives personal attendance & schedule read permissions
INSERT INTO position_permissions (position, permission_id)
SELECT 'keamanan', id FROM permission_types
WHERE code IN (
    'security.attendance',
    'security.schedule.read'
)
ON CONFLICT (position, permission_id) DO NOTHING;

