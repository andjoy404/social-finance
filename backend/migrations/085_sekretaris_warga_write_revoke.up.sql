-- Migration 085 (UP):
-- Revoke resident write permissions (warga.create, warga.update) from position 'sekretaris'.
-- Sekretaris RT retains warga.read (to view resident lists) but cannot create or edit warga records.

DELETE FROM position_permissions
WHERE position = 'sekretaris'
  AND permission_id IN (
    SELECT id FROM permission_types WHERE code IN ('warga.create', 'warga.update')
  );
