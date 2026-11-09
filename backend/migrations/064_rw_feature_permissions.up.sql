-- Migration 064: Explicit feature-separated RW permission types
--
-- Registers the explicit canonical permission codes for RW domain features:
--   - rw.dashboard.read      (Beranda RW - Read)
--   - rw.kas.read            (KAS RW - Read)
--   - rw.kas.write           (KAS RW - Write)
--   - rw.fund_location.read  (Lokasi Dana RW - Read)
--   - rw.fund_location.write (Lokasi Dana RW - Write)
--   - rw.notification.read   (Notifikasi RW - Read)
--   - rw.notification.write  (Notifikasi RW - Write)
--
-- RT positions in position_permissions are NOT modified here.
-- RT permissions are preserved unchanged.

INSERT INTO permission_types (code, domain, operation) VALUES
    ('rw.dashboard.read', 'rw', 'read'),
    ('rw.kas.read', 'rw', 'read'),
    ('rw.kas.write', 'rw', 'write'),
    ('rw.fund_location.read', 'rw', 'read'),
    ('rw.fund_location.write', 'rw', 'write'),
    ('rw.notification.read', 'rw', 'read'),
    ('rw.notification.write', 'rw', 'write')
ON CONFLICT (code) DO NOTHING;

-- Expand user_rw_memberships check constraint to support non-officer pengurus RW (anggota, pengurus)
ALTER TABLE user_rw_memberships DROP CONSTRAINT IF EXISTS user_rw_memberships_jabatan_check;
ALTER TABLE user_rw_memberships ADD CONSTRAINT user_rw_memberships_jabatan_check
    CHECK (jabatan IS NULL OR jabatan IN ('ketua', 'wakil_ketua', 'sekretaris', 'bendahara', 'anggota', 'pengurus'));
