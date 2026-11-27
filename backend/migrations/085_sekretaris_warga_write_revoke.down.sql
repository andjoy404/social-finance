-- Migration 085 (DOWN):
-- Re-grant resident write permissions (warga.create, warga.update) to position 'sekretaris'.

INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types WHERE code IN ('warga.create', 'warga.update')
ON CONFLICT (position, permission_id) DO NOTHING;
