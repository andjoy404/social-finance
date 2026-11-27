-- Migration 091 (UP):
-- 1. Expand user_rt_memberships.jabatan and position_permissions.position CHECK constraints
--    to support the official 'anggota_rw' position.
-- 2. Expand user_rw_memberships.jabatan CHECK constraint to include 'anggota_rw'.

-- Update user_rt_memberships.jabatan CHECK constraint
ALTER TABLE user_rt_memberships
    DROP CONSTRAINT IF EXISTS user_rt_memberships_jabatan_check;

ALTER TABLE user_rt_memberships
    ADD CONSTRAINT user_rt_memberships_jabatan_check
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota', 'anggota_rt', 'anggota_rw'
    ));

-- Update position_permissions.position CHECK constraint
ALTER TABLE position_permissions
    DROP CONSTRAINT IF EXISTS position_permissions_position_check;

ALTER TABLE position_permissions
    ADD CONSTRAINT position_permissions_position_check
    CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota', 'anggota_rt', 'anggota_rw'
    ));

-- Update user_rw_memberships.jabatan CHECK constraint
ALTER TABLE user_rw_memberships
    DROP CONSTRAINT IF EXISTS user_rw_memberships_jabatan_check;

ALTER TABLE user_rw_memberships
    ADD CONSTRAINT user_rw_memberships_jabatan_check
    CHECK (jabatan IS NULL OR jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'anggota', 'pengurus', 'anggota_rw'
    ));
