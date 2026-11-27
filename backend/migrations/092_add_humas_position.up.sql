-- Migration 092 (UP):
-- 1. Expand user_rt_memberships.jabatan and position_permissions.position CHECK constraints
--    to support the official 'humas' position.
-- 2. Seed default position_permissions for 'humas':
--    - Equivalent to operasional: activities.read, announcements.read, banner_templates.read,
--      organization.read, reports.read, warga.read
--    - Plus Kelola Keamanan (ReadWrite): security.schedule.read, security.schedule.write,
--      security.correction, security.device.manage

-- Update user_rt_memberships.jabatan CHECK constraint
ALTER TABLE user_rt_memberships
    DROP CONSTRAINT IF EXISTS user_rt_memberships_jabatan_check;

ALTER TABLE user_rt_memberships
    ADD CONSTRAINT user_rt_memberships_jabatan_check
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan',
        'anggota', 'anggota_rt', 'anggota_rw', 'humas'
    ));

-- Update position_permissions.position CHECK constraint
ALTER TABLE position_permissions
    DROP CONSTRAINT IF EXISTS position_permissions_position_check;

ALTER TABLE position_permissions
    ADD CONSTRAINT position_permissions_position_check
    CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan',
        'anggota', 'anggota_rt', 'anggota_rw', 'humas'
    ));

-- Seed permissions for 'humas'
INSERT INTO position_permissions (id, position, permission_id, created_at)
SELECT gen_random_uuid(), 'humas', pt.id, now()
FROM permission_types pt
WHERE pt.code IN (
    'activities.read',
    'announcements.read',
    'banner_templates.read',
    'organization.read',
    'reports.read',
    'warga.read',
    'security.schedule.read',
    'security.schedule.write',
    'security.correction',
    'security.device.manage'
)
ON CONFLICT ("position", permission_id) DO NOTHING;
