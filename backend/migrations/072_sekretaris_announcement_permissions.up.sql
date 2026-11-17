-- Migration 072: Grant announcements management permissions to Sekretaris RT
-- Announcements management: create, update, publish, archive, delete.
-- NOTE: banner_templates.manage is intentionally NOT granted. Sekretaris only
-- manages announcements; the banner template library is reserved for Ketua RT.
-- Sekretaris keeps the read-only banner_templates.read grant via the canonical seed.
BEGIN;

INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types
WHERE code IN (
    'announcements.create',
    'announcements.update',
    'announcements.publish',
    'announcements.archive',
    'announcements.delete'
)
ON CONFLICT (position, permission_id) DO NOTHING;

COMMIT;
