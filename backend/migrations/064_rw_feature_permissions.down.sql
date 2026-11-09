-- Migration 064 down: Remove feature-separated RW permission types

DELETE FROM permission_types WHERE code IN (
    'rw.dashboard.read',
    'rw.kas.read',
    'rw.kas.write',
    'rw.fund_location.read',
    'rw.fund_location.write',
    'rw.notification.read',
    'rw.notification.write'
);

ALTER TABLE user_rw_memberships DROP CONSTRAINT IF EXISTS user_rw_memberships_jabatan_check;
ALTER TABLE user_rw_memberships ADD CONSTRAINT user_rw_memberships_jabatan_check
    CHECK (jabatan IS NULL OR jabatan IN ('ketua', 'wakil_ketua', 'sekretaris', 'bendahara'));
