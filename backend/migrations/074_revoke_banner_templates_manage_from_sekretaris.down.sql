-- Migration 074: Down
-- Re-grants banner_templates.manage to sekretaris (undoing 074).
-- No-op if already granted.
BEGIN;

INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types
WHERE code = 'banner_templates.manage'
ON CONFLICT (position, permission_id) DO NOTHING;

COMMIT;
