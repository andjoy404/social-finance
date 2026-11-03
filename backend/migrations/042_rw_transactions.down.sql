-- 042_rw_transactions.down.sql
-- Drop rw_transactions table and all associated indexes.

DROP INDEX IF EXISTS idx_rw_transactions_created_by;
DROP INDEX IF EXISTS idx_rw_transactions_payment;
DROP INDEX IF EXISTS idx_rw_transactions_category;
DROP INDEX IF EXISTS idx_rw_transactions_reverses_id;
DROP INDEX IF EXISTS idx_rw_transactions_occurred_at;
DROP INDEX IF EXISTS idx_rw_transactions_type;
DROP INDEX IF EXISTS idx_rw_transactions_status;
DROP INDEX IF EXISTS idx_rw_transactions_rw_id;

DROP INDEX IF EXISTS uniq_rw_transactions_reference_no;

DROP TABLE IF EXISTS rw_transactions;
