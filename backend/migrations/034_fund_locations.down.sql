-- 033_fund_locations.down.sql
-- PERINGATAN: Hanya untuk automated testing / disposable environment!

-- 1. Kembalikan constraint status payments ke bentuk semula
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED'));

-- 2. Hapus kolom location_id dari transactions
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS fk_transactions_location_rt;
DROP INDEX IF EXISTS idx_transactions_location_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS location_id;

-- 3. Hapus tabel ledger mutasi lokasi dana
DROP INDEX IF EXISTS uniq_lbc_transfer_change_type;
DROP INDEX IF EXISTS uniq_lbc_transaction_id;
DROP INDEX IF EXISTS idx_lbc_created_at;
DROP INDEX IF EXISTS idx_lbc_rt_loc;
DROP TABLE IF EXISTS location_balance_changes;

-- 4. Hapus tabel transfer antar-lokasi
DROP INDEX IF EXISTS idx_fund_transfers_to_loc;
DROP INDEX IF EXISTS idx_fund_transfers_from_loc;
DROP INDEX IF EXISTS idx_fund_transfers_rt_date;
DROP TABLE IF EXISTS fund_transfers;

-- 5. Hapus tabel master lokasi dana
DROP INDEX IF EXISTS idx_fund_locations_category;
DROP INDEX IF EXISTS idx_fund_locations_rt_active;
DROP INDEX IF EXISTS uniq_fund_locations_rt_lower_name;
DROP TABLE IF EXISTS fund_locations;
