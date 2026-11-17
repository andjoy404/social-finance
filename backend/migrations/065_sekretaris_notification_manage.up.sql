-- Migration 065: Grant sekretaris RT notification.manage (idempotent).
-- Ensures existing databases (where migration 052 was applied before grant edit)
-- receive the permission. Scoped by RT position_permissions; RW routes use
-- RequireRWAccess/rw.* permissions and do not grant sekretaris RW write access.
INSERT INTO permission_types (code, domain, operation) VALUES
    ('notification.manage', 'notification', 'manage')
ON CONFLICT (code) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types WHERE code = 'notification.manage'
ON CONFLICT (position, permission_id) DO NOTHING;
