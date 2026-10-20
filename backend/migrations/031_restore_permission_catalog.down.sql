-- Migration 031: Restore canonical permission catalog (permission_types + position_permissions)
--
-- DOWN migration — INTENTIONALLY A NO-OP (forward-only).
--
-- This migration only INSERTs permission master data. Reversing it would require
-- DELETEing permission_types and position_permissions rows, which is destructive
-- and may remove permission grants that are actively used by authorization
-- (RequirePermission) for jabatan-based users such as Ketua RT.
--
-- Per project forward-only policy, no destructive rollback procedure is added
-- here. To undo this migration in an emergency, operators must manually review
-- and remove only the specific rows they intend to revoke, after confirming no
-- running authorization depends on them.

SELECT 1;
