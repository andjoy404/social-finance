-- 023_reconcile_iuran_payments.down.sql
-- Rollback migration 023: hapus transaksi yang dibuat oleh up migration ini.
--
-- Identifikasi transaksi yang dihapus menggunakan tracking table `sf023_tx_ids`
-- yang dibuat oleh up migration (bukan berdasarkan status payments yang bisa berubah).
--
-- Keamanan rollback:
--   - Menggunakan `sf023_tx_ids` tracking table untuk identifikasi presisi.
--   - Table ini di-drop setelah digunakan untuk mencegah leakage.
--   - Guard check memastikan tracking table ada dan berisi entri sebelum delete.
--
-- Eksekusi:
--   docker compose run --rm migrate down 23

BEGIN;

-- ============================================================
-- Step 1: Guard — pastikan tracking table ada dan berisi entri
-- ============================================================
DO $$
DECLARE
    migration_tx_count INT;
BEGIN
    -- Validate: sf023_tx_ids table harus ada (ditimbulkan oleh up migration)
    IF NOT EXISTS (
        SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'sf023_tx_ids'
    ) THEN
        RAISE EXCEPTION 'ROLLBACK GUARD FAILED: sf023_tx_ids table not found. Migration 023 may not have been applied or already rolled back.';
    END IF;

    -- Hitung jumlah transaksi yang akan dihapus dari tracking table
    SELECT COUNT(*)
    INTO migration_tx_count
    FROM sf023_tx_ids;

    IF migration_tx_count = 0 THEN
        RAISE EXCEPTION 'NO TRANSACTIONS TO ROLLBACK: sf023_tx_ids is empty. Migration 023 may not have created any transactions or already rolled back.';
    END IF;

    RAISE NOTICE 'Found % transactions created by migration 023 to rollback.', migration_tx_count;
END $$;

-- ============================================================
-- Step 2: Hapus transaksi yang dibuat migration 023
-- ============================================================
-- Menggunakan tracking table untuk presisi — tidak bergantung pada
-- status payments yang bisa berubah setelah migration.
DELETE FROM transactions
WHERE id IN (SELECT tx_id FROM sf023_tx_ids);

-- ============================================================
-- Step 3: Cleanup — drop tracking table setelah digunakan
-- ============================================================
DROP TABLE IF EXISTS sf023_tx_ids;

COMMIT;
