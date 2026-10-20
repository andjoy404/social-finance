-- Migration 031: Restore canonical permission catalog (permission_types + position_permissions)
--
-- Production runs with the dev seed intentionally disabled
-- (backend/internal/devseed/seed.go is never executed there). As a result the
-- permission_types and position_permissions tables — created by migration 011
-- but never populated in production — are empty, and jabatan-based users such
-- as Ketua RT (role=warga, jabatan=ketua) receive HTTP 403 on
-- GET /api/v1/residents, /api/v1/bills, and /api/v1/transactions because
-- RequirePermission evaluates position_permissions and finds no matching grant.
--
-- This migration forward-only restores the canonical permission master data
-- defined in backend/internal/devseed/seed.go (12 permission types, 43
-- position grants) so that migration 025's grants and the ketua position policy
-- are consistent with the designed authorization model.
--
-- Safety properties:
--   * Forward-only: this file only INSERTs. It never UPDATEs/DELETEs existing
--     rows and never touches users, memberships, RTs, households, residents,
--     categories, dues, bills, payments, or transactions.
--   * Idempotent: every statement uses ON CONFLICT ... DO NOTHING, so re-running
--     the migration is safe and never duplicates rows.
--   * Additive-only: because production tables are currently empty, this restores
--     the intended design without overriding an existing production policy.
--   * Order respects foreign keys: permission_types is populated before
--     position_permissions (which references permission_types.id).

-- 1. Permission types — 12 canonical codes (must come first due to FK).
INSERT INTO permission_types (code, domain, operation) VALUES
    ('warga.read', 'warga', 'read'),
    ('warga.create', 'warga', 'create'),
    ('warga.update', 'warga', 'update'),
    ('warga.import', 'warga', 'import'),
    ('warga.export', 'warga', 'export'),
    ('finance.read', 'finance', 'read'),
    ('finance.create', 'finance', 'create'),
    ('finance.update', 'finance', 'update'),
    ('finance.approve', 'finance', 'approve'),
    ('reports.read', 'reports', 'read'),
    ('rt.settings.read', 'rt.settings', 'read'),
    ('rt.settings.manage', 'rt.settings', 'manage')
ON CONFLICT (code) DO NOTHING;

-- 2. Position permissions — 43 canonical grants (resolves permission_id by code).
--    ketua: full warga + finance + reports + rt.settings access.
INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types WHERE code IN (
    'warga.read', 'warga.create', 'warga.update', 'warga.import', 'warga.export',
    'finance.read', 'finance.create', 'finance.update', 'finance.approve',
    'reports.read', 'rt.settings.read', 'rt.settings.manage'
)
ON CONFLICT (position, permission_id) DO NOTHING;

-- wakil_ketua
INSERT INTO position_permissions (position, permission_id)
SELECT 'wakil_ketua', id FROM permission_types WHERE code IN (
    'warga.read', 'warga.create', 'warga.update', 'finance.read', 'reports.read', 'rt.settings.read'
)
ON CONFLICT (position, permission_id) DO NOTHING;

-- sekretaris
INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types WHERE code IN (
    'warga.read', 'warga.create', 'warga.update', 'finance.read', 'finance.create', 'reports.read', 'rt.settings.read'
)
ON CONFLICT (position, permission_id) DO NOTHING;

-- bendahara
INSERT INTO position_permissions (position, permission_id)
SELECT 'bendahara', id FROM permission_types WHERE code IN (
    'warga.read', 'warga.create', 'warga.update', 'warga.import', 'warga.export',
    'finance.read', 'finance.create', 'finance.update', 'finance.approve', 'reports.read'
)
ON CONFLICT (position, permission_id) DO NOTHING;

-- keamanan
INSERT INTO position_permissions (position, permission_id)
SELECT 'keamanan', id FROM permission_types WHERE code IN ('warga.read', 'reports.read')
ON CONFLICT (position, permission_id) DO NOTHING;

-- sosial
INSERT INTO position_permissions (position, permission_id)
SELECT 'sosial', id FROM permission_types WHERE code IN ('warga.read', 'reports.read')
ON CONFLICT (position, permission_id) DO NOTHING;

-- kebersihan_pembangunan
INSERT INTO position_permissions (position, permission_id)
SELECT 'kebersihan_pembangunan', id FROM permission_types WHERE code IN ('warga.read', 'reports.read')
ON CONFLICT (position, permission_id) DO NOTHING;

-- anggota
INSERT INTO position_permissions (position, permission_id)
SELECT 'anggota', id FROM permission_types WHERE code IN ('warga.read', 'reports.read')
ON CONFLICT (position, permission_id) DO NOTHING;
