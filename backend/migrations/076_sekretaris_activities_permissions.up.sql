-- Migration 076: Grant activities management permissions to Sekretaris RT
-- Sekretaris manages Pengumuman and Kegiatan (write access) within RT scope.
-- Grants CRUD/publish/cancel/delete for activities, matching announcement scope
-- given in 072. Idempotent and scoped to sekretaris only.
BEGIN;

INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types
WHERE code IN (
    'activities.create',
    'activities.update',
    'activities.publish',
    'activities.cancel',
    'activities.delete'
)
ON CONFLICT (position, permission_id) DO NOTHING;

COMMIT;
