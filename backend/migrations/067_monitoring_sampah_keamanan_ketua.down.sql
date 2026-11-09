-- Migration 067 Rollback: Revoke rw.monitoring permissions from 'keamanan' and 'ketua'
DELETE FROM position_permissions
WHERE position IN ('keamanan', 'ketua')
  AND permission_id IN (
    SELECT id FROM permission_types WHERE code IN ('rw.monitoring.read', 'rw.monitoring.notes.write', 'rw.monitoring.export')
  );

