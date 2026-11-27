-- Migration 083 (DOWN):
-- 1. Restore finance write permissions (finance.create, finance.update, finance.approve) for sekretaris
-- 2. Revert user_rt_memberships.jabatan and position_permissions.position CHECK constraints
--
-- SAFETY: rollback is refused if jabatan/position 'anggota_rt' is already in use,
-- because reverting CHECK constraints would violate existing data. Resolve/migrate
-- those rows first, then retry rollback.

-- Precondition guard: fail fast before any DDL/DML if anggota_rt data exists.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM user_rt_memberships WHERE jabatan = 'anggota_rt') THEN
        RAISE EXCEPTION 'ROLLBACK REFUSED: user_rt_memberships contains rows with jabatan=anggota_rt; cannot safely revert CHECK constraint. Remove/migrate anggota_rt memberships before rolling back migration 083.';
    END IF;
    IF EXISTS (SELECT 1 FROM position_permissions WHERE position = 'anggota_rt') THEN
        RAISE EXCEPTION 'ROLLBACK REFUSED: position_permissions contains rows with position=anggota_rt; cannot safely revert CHECK constraint. Remove/migrate anggota_rt rows before rolling back migration 083.';
    END IF;
END $$;

-- Re-grant finance write permissions to position 'sekretaris' (as they existed before 083 UP).
INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id
FROM permission_types
WHERE code IN ('finance.create', 'finance.update', 'finance.approve')
ON CONFLICT (position, permission_id) DO NOTHING;

-- Revert user_rt_memberships.jabatan CHECK constraint
ALTER TABLE user_rt_memberships
    DROP CONSTRAINT IF EXISTS user_rt_memberships_jabatan_check;

ALTER TABLE user_rt_memberships
    ADD CONSTRAINT user_rt_memberships_jabatan_check
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota'
    ));

-- Revert position_permissions.position CHECK constraint
ALTER TABLE position_permissions
    DROP CONSTRAINT IF EXISTS position_permissions_position_check;

ALTER TABLE position_permissions
    ADD CONSTRAINT position_permissions_position_check
    CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota'
    ));
