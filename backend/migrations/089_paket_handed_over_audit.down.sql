-- 089_paket_handed_over_audit.down.sql
DROP INDEX IF EXISTS idx_paket_items_handed_over_by;
ALTER TABLE paket_items
    DROP COLUMN IF EXISTS handed_over_at,
    DROP COLUMN IF EXISTS handed_over_by_user_id;

