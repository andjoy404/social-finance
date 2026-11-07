-- Migration 057: Add Petugas Keamanan and Petugas Kebersihan permissions + monitoring notes table.
--
-- This migration:
--   1. Registers new permission types:
--      - notification.send_waste_truck  (Keamanan: send waste truck arrival notification)
--      - rw.monitoring.read             (Kebersihan: read overdue 3+ months monitoring list)
--      - rw.monitoring.notes.write      (Kebersihan: create/update field notes)
--      - rw.monitoring.export           (Kebersihan: export Excel/PDF/print)
--   2. Grants these permissions to the corresponding jabatan positions.
--   3. Creates rw_monitoring_notes table for Kebersihan field notes.
--
-- All changes are idempotent where possible.

-- =====================================================================
-- Permission types
-- =====================================================================

INSERT INTO permission_types (code, domain, operation) VALUES
    ('notification.send_waste_truck', 'notification', 'send_waste_truck')
ON CONFLICT (code) DO NOTHING;

INSERT INTO permission_types (code, domain, operation) VALUES
    ('rw.monitoring.read', 'rw', 'monitoring.read')
ON CONFLICT (code) DO NOTHING;

INSERT INTO permission_types (code, domain, operation) VALUES
    ('rw.monitoring.notes.write', 'rw', 'monitoring.notes.write')
ON CONFLICT (code) DO NOTHING;

INSERT INTO permission_types (code, domain, operation) VALUES
    ('rw.monitoring.export', 'rw', 'monitoring.export')
ON CONFLICT (code) DO NOTHING;

-- =====================================================================
-- Position permissions: Keamanan (security)
-- =====================================================================

-- Keamanan can read warga (existing from migration 056) and send waste truck notifications.
INSERT INTO position_permissions (position, permission_id)
SELECT 'keamanan', id FROM permission_types WHERE code = 'notification.send_waste_truck'
ON CONFLICT (position, permission_id) DO NOTHING;

-- =====================================================================
-- Position permissions: Kebersihan (cleanliness)
-- =====================================================================

-- Kebersihan can read warga (existing from migration 056), read monitoring, write notes, export.
INSERT INTO position_permissions (position, permission_id)
SELECT 'kebersihan', id FROM permission_types WHERE code = 'rw.monitoring.read'
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'kebersihan', id FROM permission_types WHERE code = 'rw.monitoring.notes.write'
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'kebersihan', id FROM permission_types WHERE code = 'rw.monitoring.export'
ON CONFLICT (position, permission_id) DO NOTHING;

-- =====================================================================
-- rw_monitoring_notes table: Kebersihan field notes per household.
-- =====================================================================

-- One note row per household_occupancy. Notes persist even after the
-- household exits the active monitoring list (bill paid), so historical
-- notes remain available if the household re-qualifies later.
CREATE TABLE IF NOT EXISTS rw_monitoring_notes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id             UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    household_occupancy_id UUID NOT NULL REFERENCES household_occupancies(id) ON DELETE CASCADE,
    note              TEXT NOT NULL CHECK (char_length(note) > 0),
    created_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE rw_monitoring_notes IS
    'Kebersihan field notes per household occupancy. Notes persist after bill payment so they remain available if the household re-qualifies for monitoring.';

COMMENT ON COLUMN rw_monitoring_notes.note IS
    'Operational field note written by Petugas Kebersihan (e.g. sampah belum diangkut, sudah dikunjungi).';

CREATE INDEX IF NOT EXISTS idx_rw_monitoring_notes_rt ON rw_monitoring_notes(rt_id);
CREATE INDEX IF NOT EXISTS idx_rw_monitoring_notes_occupancy ON rw_monitoring_notes(household_occupancy_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_rw_monitoring_notes_occupancy ON rw_monitoring_notes(household_occupancy_id);

-- =====================================================================
-- Down
-- =====================================================================

-- DROP TABLE IF EXISTS rw_monitoring_notes;
-- DELETE FROM position_permissions WHERE position = 'keamanan' AND permission_id = (SELECT id FROM permission_types WHERE code = 'notification.send_waste_truck');
-- DELETE FROM position_permissions WHERE position = 'kebersihan' AND permission_id IN (SELECT id FROM permission_types WHERE code IN ('rw.monitoring.read','rw.monitoring.notes.write','rw.monitoring.export'));
-- DELETE FROM permission_types WHERE code IN ('notification.send_waste_truck','rw.monitoring.read','rw.monitoring.notes.write','rw.monitoring.export');
