-- Migration 072: Down
-- Rollback revokes the announcement management grants from sekretaris.
-- banner_templates.manage is deliberately never granted by 072; no revoke needed.
BEGIN;

DELETE FROM position_permissions
WHERE position = 'sekretaris'
  AND permission_id IN (
    SELECT id FROM permission_types
    WHERE code IN (
        'announcements.create',
        'announcements.update',
        'announcements.publish',
        'announcements.archive',
        'announcements.delete'
    )
);

COMMIT;
