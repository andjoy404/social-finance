-- Migration 092 (DOWN):
-- Revert constraints and permissions for 'humas'.
-- SAFETY: rollback is refused if jabatan/position 'humas' is actively in use in user_rt_memberships.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM user_rt_memberships WHERE jabatan = 'humas') THEN
        RAISE EXCEPTION 'ROLLBACK REFUSED: user_rt_memberships contains rows with jabatan=humas; cannot safely revert CHECK constraint.';
    END IF;
END $$;

-- Delete seeded permissions for 'humas'
DELETE FROM position_permissions WHERE position = 'humas';

-- Revert position_permissions.position CHECK constraint to 091 list
ALTER TABLE position_permissions
    DROP CONSTRAINT IF EXISTS position_permissions_position_check;

ALTER TABLE position_permissions
    ADD CONSTRAINT position_permissions_position_check
    CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan',
        'anggota', 'anggota_rt', 'anggota_rw'
    ));

-- Revert user_rt_memberships.jabatan CHECK constraint to 091 list
ALTER TABLE user_rt_memberships
    DROP CONSTRAINT IF EXISTS user_rt_memberships_jabatan_check;

ALTER TABLE user_rt_memberships
    ADD CONSTRAINT user_rt_memberships_jabatan_check
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan',
        'anggota', 'anggota_rt', 'anggota_rw'
    ));
