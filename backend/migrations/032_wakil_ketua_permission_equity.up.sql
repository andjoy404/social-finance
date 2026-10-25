-- Migration 032: Equate Wakil Ketua permissions to Ketua RT
--
-- Previously, wakil_ketua had only 6 permissions (warga.read/create/update,
-- finance.read, reports.read, rt.settings.read). This migration replaces the
-- 6 grants with 12 grants identical to ketua: full warga, finance, reports,
-- and rt.settings access.

-- 1. Remove existing 6 grants for wakil_ketua.
DELETE FROM position_permissions
WHERE position = 'wakil_ketua';

-- 2. Insert full 12 grants (equivalent to ketua).
INSERT INTO position_permissions (position, permission_id)
SELECT 'wakil_ketua', id FROM permission_types WHERE code IN (
    'warga.read', 'warga.create', 'warga.update', 'warga.import', 'warga.export',
    'finance.read', 'finance.create', 'finance.update', 'finance.approve',
    'reports.read', 'rt.settings.read', 'rt.settings.manage'
);
