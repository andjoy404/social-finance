-- Migration 086 (DOWN):
-- Revert patrol and paket tables and permissions

DROP TABLE IF EXISTS paket_photos CASCADE;
DROP TABLE IF EXISTS paket_items CASCADE;
DROP TABLE IF EXISTS paket_reports CASCADE;

DROP TABLE IF EXISTS patrol_photos CASCADE;
DROP TABLE IF EXISTS patrol_bundles CASCADE;
DROP TABLE IF EXISTS patrol_templates CASCADE;

DELETE FROM position_permissions
WHERE permission_id IN (
    SELECT id FROM permission_types
    WHERE code IN ('patrol.create', 'patrol.read', 'patrol.template.manage', 'paket.create', 'paket.read', 'paket.status_change')
);

DELETE FROM permission_types
WHERE code IN ('patrol.create', 'patrol.read', 'patrol.template.manage', 'paket.create', 'paket.read', 'paket.status_change');
