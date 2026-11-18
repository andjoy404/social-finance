-- Migration 083 (UP):
-- 1. Revoke write permissions (finance.create) from position 'sekretaris'
-- 2. Expand user_rt_memberships.jabatan and position_permissions.position CHECK constraints
--    to support the official 'anggota_rt' position.

-- Revoke write permissions (finance.create, finance.update, finance.approve) from sekretaris
DELETE FROM position_permissions
WHERE position = 'sekretaris'
  AND permission_id IN (
    SELECT id FROM permission_types WHERE code IN ('finance.create', 'finance.update', 'finance.approve')
  );

-- Update user_rt_memberships.jabatan CHECK constraint
ALTER TABLE user_rt_memberships
    DROP CONSTRAINT IF EXISTS user_rt_memberships_jabatan_check;

ALTER TABLE user_rt_memberships
    ADD CONSTRAINT user_rt_memberships_jabatan_check
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota', 'anggota_rt'
    ));

-- Update position_permissions.position CHECK constraint
ALTER TABLE position_permissions
    DROP CONSTRAINT IF EXISTS position_permissions_position_check;

ALTER TABLE position_permissions
    ADD CONSTRAINT position_permissions_position_check
    CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota', 'anggota_rt'
    ));
