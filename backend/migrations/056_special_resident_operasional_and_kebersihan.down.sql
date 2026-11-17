-- Migration 056 DOWN: Revert 'operasional' back to 'kebersihan_pembangunan'
-- and remove 'kebersihan' position only if no membership data uses 'kebersihan'.
--
-- Safety properties:
--   * Refuses to rollback if any user_rt_memberships rows currently use jabatan = 'kebersihan',
--     preventing accidental data loss or constraint failure on active/historical accounts.
--   * Conflict-safely restores position_permissions from 'operasional' to 'kebersihan_pembangunan'.
--   * Restores user_rt_memberships rows from 'operasional' to 'kebersihan_pembangunan'.

DO $$
DECLARE
    kebersihan_count INTEGER;
    r RECORD;
BEGIN
    SELECT COUNT(*) INTO kebersihan_count
    FROM user_rt_memberships
    WHERE jabatan = 'kebersihan';

    IF kebersihan_count > 0 THEN
        RAISE EXCEPTION 'Cannot rollback migration 056: % user_rt_memberships row(s) still use jabatan=kebersihan', kebersihan_count;
    END IF;

    -- Drop current CHECK constraints before restoring old code 'kebersihan_pembangunan'
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

    -- Restore user_rt_memberships from 'operasional' to 'kebersihan_pembangunan'
    UPDATE user_rt_memberships
    SET jabatan = 'kebersihan_pembangunan',
        updated_at = now()
    WHERE jabatan = 'operasional';

    -- Restore position_permissions from 'operasional' to 'kebersihan_pembangunan'
    INSERT INTO position_permissions (position, permission_id)
    SELECT 'kebersihan_pembangunan', permission_id
    FROM position_permissions
    WHERE position = 'operasional'
    ON CONFLICT (position, permission_id) DO NOTHING;

    DELETE FROM position_permissions
    WHERE position IN ('operasional', 'kebersihan');

    -- Restore original CHECK constraints
    ALTER TABLE user_rt_memberships
        ADD CONSTRAINT user_rt_memberships_jabatan_check
        CHECK (jabatan IN (
            'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
            'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'
        ));

    ALTER TABLE position_permissions
        ADD CONSTRAINT position_permissions_position_check
        CHECK (position IN (
            'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
            'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'
        ));
END $$;
