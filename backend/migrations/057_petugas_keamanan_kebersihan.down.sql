-- Migration 057: Down (remove Petugas Keamanan and Petugas Kebersihan permissions + notes table).
--
-- WARNING: Dropping rw_monitoring_notes will delete field notes data.
-- This is intentional for rollback scenarios.
--
-- Precondition check: abort if monitoring notes contain data that would be lost.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM rw_monitoring_notes LIMIT 1) THEN
        RAISE EXCEPTION 'Cannot rollback migration 057: rw_monitoring_notes contains data. Manual data export required before rollback.';
    END IF;
END $$;

DROP TABLE IF EXISTS rw_monitoring_notes;

DELETE FROM position_permissions WHERE position = 'kebersihan' AND permission_id IN (
    SELECT id FROM permission_types WHERE code IN ('rw.monitoring.read','rw.monitoring.notes.write','rw.monitoring.export')
);

DELETE FROM position_permissions WHERE position = 'keamanan' AND permission_id = (
    SELECT id FROM permission_types WHERE code = 'notification.send_waste_truck'
);

DELETE FROM permission_types WHERE code IN (
    'notification.send_waste_truck',
    'rw.monitoring.read',
    'rw.monitoring.notes.write',
    'rw.monitoring.export'
);
