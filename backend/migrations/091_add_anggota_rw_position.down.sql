-- Migration 091 (DOWN):
-- Revert constraints back to pre-091 state.
-- SAFETY: rollback is refused if jabatan/position 'anggota_rw' is already in use.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM user_rt_memberships WHERE jabatan = 'anggota_rw') THEN
        RAISE EXCEPTION 'ROLLBACK REFUSED: user_rt_memberships contains rows with jabatan=anggota_rw; cannot safely revert CHECK constraint.';
    END IF;

    IF EXISTS (SELECT 1 FROM position_permissions WHERE position = 'anggota_rw') THEN
        RAISE EXCEPTION 'ROLLBACK REFUSED: position_permissions contains rows with position=anggota_rw; cannot safely revert CHECK constraint.';
    END IF;

    IF EXISTS (SELECT 1 FROM user_rw_memberships WHERE jabatan = 'anggota_rw') THEN
        RAISE EXCEPTION 'ROLLBACK REFUSED: user_rw_memberships contains rows with jabatan=anggota_rw; cannot safely revert CHECK constraint.';
    END IF;
END $$;

ALTER TABLE user_rt_memberships
    DROP CONSTRAINT IF EXISTS user_rt_memberships_jabatan_check;

ALTER TABLE user_rt_memberships
    ADD CONSTRAINT user_rt_memberships_jabatan_check
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota', 'anggota_rt'
    ));

ALTER TABLE position_permissions
    DROP CONSTRAINT IF EXISTS position_permissions_position_check;

ALTER TABLE position_permissions
    ADD CONSTRAINT position_permissions_position_check
    CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota', 'anggota_rt'
    ));

ALTER TABLE user_rw_memberships
    DROP CONSTRAINT IF EXISTS user_rw_memberships_jabatan_check;

ALTER TABLE user_rw_memberships
    ADD CONSTRAINT user_rw_memberships_jabatan_check
    CHECK (jabatan IS NULL OR jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'anggota', 'pengurus'
    ));
