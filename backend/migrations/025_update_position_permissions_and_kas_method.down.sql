-- 025_update_position_permissions_and_kas_method.down.sql

DELETE FROM position_permissions
WHERE position = 'bendahara' AND permission_id IN (
    SELECT id FROM permission_types WHERE code IN ('warga.export', 'warga.import')
);

DELETE FROM position_permissions
WHERE position = 'sekretaris' AND permission_id IN (
    SELECT id FROM permission_types WHERE code IN ('finance.read', 'finance.create', 'rt.settings.read')
);

ALTER TABLE transactions DROP COLUMN IF EXISTS payment_method;

