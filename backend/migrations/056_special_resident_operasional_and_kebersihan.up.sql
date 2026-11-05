-- Migration 056: Migrate special resident position 'kebersihan_pembangunan' to 'operasional'
-- and add new independent special resident position 'kebersihan' (Petugas Kebersihan).
--
-- Safety properties:
--   1. Drops old CHECK constraints on user_rt_memberships.jabatan and
--      position_permissions.position before migrating data.
--   2. Migrates existing user_rt_memberships rows with jabatan = 'kebersihan_pembangunan'
--      to 'operasional' without deleting or modifying any accounts, roles, or RT/RW memberships.
--   3. Copies existing position_permissions grants from 'kebersihan_pembangunan' to
--      'operasional' using ON CONFLICT DO NOTHING (preventing unique constraint errors),
--      ensures canonical grants ('warga.read', 'reports.read') for 'operasional',
--      and removes the obsolete 'kebersihan_pembangunan' rows.
--   4. Seeds independent position_permissions grants ('warga.read', 'reports.read')
--      for 'kebersihan' without altering 'keamanan' or 'operasional'.
--   5. Enforces updated CHECK constraints allowing:
--      ('ketua', 'wakil_ketua', 'sekretaris', 'bendahara', 'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota').

-- 1. Drop existing CHECK constraints on user_rt_memberships.jabatan and position_permissions.position
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT c.conname
        FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = current_schema()
          AND t.relname = 'user_rt_memberships'
          AND c.contype = 'c'
          AND pg_get_constraintdef(c.oid) ILIKE '%jabatan%'
    LOOP
        EXECUTE format('ALTER TABLE user_rt_memberships DROP CONSTRAINT IF EXISTS %I', r.conname);
    END LOOP;

    FOR r IN
        SELECT c.conname
        FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = current_schema()
          AND t.relname = 'position_permissions'
          AND c.contype = 'c'
          AND pg_get_constraintdef(c.oid) ILIKE '%position%'
    LOOP
        EXECUTE format('ALTER TABLE position_permissions DROP CONSTRAINT IF EXISTS %I', r.conname);
    END LOOP;
END $$;

-- 2. Ensure canonical permission_types exist before granting position_permissions
INSERT INTO permission_types (code, domain, operation) VALUES
    ('warga.read', 'warga', 'read'),
    ('reports.read', 'reports', 'read')
ON CONFLICT (code) DO NOTHING;

-- 3. Migrate existing user_rt_memberships rows from 'kebersihan_pembangunan' to 'operasional'
UPDATE user_rt_memberships
SET jabatan = 'operasional',
    updated_at = now()
WHERE jabatan = 'kebersihan_pembangunan';

-- 4. Migrate position_permissions from 'kebersihan_pembangunan' to 'operasional' safely
INSERT INTO position_permissions (position, permission_id)
SELECT 'operasional', permission_id
FROM position_permissions
WHERE position = 'kebersihan_pembangunan'
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'operasional', id
FROM permission_types
WHERE code IN ('warga.read', 'reports.read')
ON CONFLICT (position, permission_id) DO NOTHING;

DELETE FROM position_permissions
WHERE position = 'kebersihan_pembangunan';

-- 5. Add independent position_permissions grants for 'kebersihan'
INSERT INTO position_permissions (position, permission_id)
SELECT 'kebersihan', id
FROM permission_types
WHERE code IN ('warga.read', 'reports.read')
ON CONFLICT (position, permission_id) DO NOTHING;

-- 6. Add updated CHECK constraints
ALTER TABLE user_rt_memberships
    ADD CONSTRAINT user_rt_memberships_jabatan_check
    CHECK (jabatan IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota'
    ));

ALTER TABLE position_permissions
    ADD CONSTRAINT position_permissions_position_check
    CHECK (position IN (
        'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
        'keamanan', 'sosial', 'operasional', 'kebersihan', 'anggota'
    ));
