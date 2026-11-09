-- Migration 062: Add notification.read permission and notification.manage grant for Ketua RW only
--
-- Extends migration 052 which only created notification.manage and granted it to Ketua RT.
-- This migration adds:
--   1. notification.read permission type
--   2. notification.read grants for ketua, wakil_ketua, sekretaris, bendahara positions
--   3. notification.manage is granted ONLY to ketua (already in migration 052)
--
-- These grants support the RW notification tab access rules:
--   - Ketua RW: notification.read + notification.manage (manage templates, schedules, manual push)
--   - Wakil Ketua RW: notification.read only (read-only access)
--   - Sekretaris RW: notification.read only (view logs only)
--   - Bendahara RW: notification.read only (read-only access)
--   - All other jabatan: no notification permissions

-- =====================================================================
-- Permission: notification.read
-- =====================================================================

INSERT INTO permission_types (code, domain, operation) VALUES
    ('notification.read', 'notification', 'read')
ON CONFLICT (code) DO NOTHING;

-- =====================================================================
-- Grants for notification.read (all RW management positions)
-- notification.manage is granted ONLY to ketua (from migration 052).
-- wakil_ketua, sekretaris, bendahara get notification.read ONLY.
-- =====================================================================

INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types WHERE code = 'notification.read'
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'wakil_ketua', id FROM permission_types WHERE code = 'notification.read'
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types WHERE code = 'notification.read'
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'bendahara', id FROM permission_types WHERE code = 'notification.read'
ON CONFLICT (position, permission_id) DO NOTHING;
