-- 070_community_delete_permissions rollback
-- Safely undoes migration 070 without affecting permissions granted by
-- other sources (canonical devseed, manual admin grants, or future migrations).
--
-- Key safety properties:
--   1. Only removes grants from 'ketua' (the position migration 070 added to).
--   2. Does NOT delete permission_types, because these are canonical permissions
--      that may also be seeded by devseed or used by other processes.
--   3. If someone needs to remove the permission types, that is an administrative
--      action outside the scope of this migration's rollback.
--   4. Idempotent — safe to run multiple times.

BEGIN;

-- Remove only the grant to 'ketua' that migration 070 UP added.
-- If any other position also holds these permissions, we leave the grant
-- intact because we cannot prove it came from this migration.
DELETE FROM position_permissions
WHERE position = 'ketua'
  AND permission_id IN (
    SELECT id FROM permission_types
    WHERE code IN ('activities.delete', 'announcements.delete')
  );

COMMIT;
