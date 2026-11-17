-- Migration 080 rollback: permanent system default announcement — "Pengingat Pembayaran Iuran Warga"
--
-- Removes only the permanent marker rows created by the up migration.
-- Regular announcements are left untouched.

BEGIN;

DELETE FROM announcements
WHERE is_permanent = true;

DROP INDEX IF EXISTS announcements_permanent_one_per_rt;
ALTER TABLE announcements
    DROP COLUMN IF EXISTS is_permanent;

COMMIT;
