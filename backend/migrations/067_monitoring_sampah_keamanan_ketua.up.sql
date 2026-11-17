-- Migration 067: Grant rw.monitoring permissions to 'keamanan' and 'ketua'
-- Permitted positions for RT-scoped waste monitoring: kebersihan (from migration 057), keamanan, and ketua.

INSERT INTO permission_types (code, domain, operation) VALUES
    ('rw.monitoring.read', 'rw', 'monitoring.read'),
    ('rw.monitoring.notes.write', 'rw', 'monitoring.notes.write'),
    ('rw.monitoring.export', 'rw', 'monitoring.export')
ON CONFLICT (code) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'keamanan', id FROM permission_types WHERE code IN ('rw.monitoring.read', 'rw.monitoring.notes.write', 'rw.monitoring.export')
ON CONFLICT (position, permission_id) DO NOTHING;

INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types WHERE code IN ('rw.monitoring.read', 'rw.monitoring.notes.write', 'rw.monitoring.export')
ON CONFLICT (position, permission_id) DO NOTHING;

