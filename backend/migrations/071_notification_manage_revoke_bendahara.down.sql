-- Rollback for 071_notification_manage_revoke_bendahara.
-- Re-grants notification.manage to the bendahara position ONLY IF the
-- permission does not already exist (idempotent).
--
-- WARNING: This reinstates the pre-fix behavior where Bendahara has
-- administrative notification access. Deploy only after explicit sign-off.
BEGIN;

INSERT INTO position_permissions (position, permission_id)
SELECT 'bendahara', id FROM permission_types
WHERE code = 'notification.manage'
ON CONFLICT DO NOTHING;

COMMIT;
