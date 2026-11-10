-- 070_community_delete_permissions
-- Add explicit activities.delete and announcements.delete permission types
-- and grant them to Ketua RT ('ketua') position.

BEGIN;

INSERT INTO permission_types (code, domain, operation) VALUES
    ('activities.delete', 'community', 'delete'),
    ('announcements.delete', 'community', 'delete')
ON CONFLICT (code) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types
WHERE code IN ('activities.delete', 'announcements.delete')
ON CONFLICT (position, permission_id) DO NOTHING;

COMMIT;

