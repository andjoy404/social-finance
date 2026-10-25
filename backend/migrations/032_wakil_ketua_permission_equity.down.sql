-- Migration 032: Equate Wakil Ketua permissions to Ketua RT (down)
--
-- Restore wakil_ketua to the original 6 grants.

-- 1. Remove the 12 grants.
DELETE FROM position_permissions
WHERE position = 'wakil_ketua';

-- 2. Restore original 6 grants.
INSERT INTO position_permissions (position, permission_id)
SELECT 'wakil_ketua', id FROM permission_types WHERE code IN (
    'warga.read', 'warga.create', 'warga.update', 'finance.read', 'reports.read', 'rt.settings.read'
);
