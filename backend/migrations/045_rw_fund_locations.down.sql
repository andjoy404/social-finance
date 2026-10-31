-- 045_rw_fund_locations.down.sql
DROP INDEX IF EXISTS uniq_rw_lbc_transfer_change_type;
DROP INDEX IF EXISTS uniq_rw_lbc_transaction_id;
DROP INDEX IF EXISTS idx_rw_lbc_created_at;
DROP INDEX IF EXISTS idx_rw_lbc_rw_loc;
DROP TABLE IF EXISTS rw_location_balance_changes;

DROP INDEX IF EXISTS idx_rw_transactions_source_rt_id;
DROP INDEX IF EXISTS idx_rw_transactions_location_id;
ALTER TABLE rw_transactions DROP CONSTRAINT IF EXISTS fk_rw_transactions_location_rw;
ALTER TABLE rw_transactions DROP COLUMN IF EXISTS source_rt_id;
ALTER TABLE rw_transactions DROP COLUMN IF EXISTS location_id;

DROP INDEX IF EXISTS idx_rw_fund_transfers_to_loc;
DROP INDEX IF EXISTS idx_rw_fund_transfers_from_loc;
DROP INDEX IF EXISTS idx_rw_fund_transfers_rw_date;
DROP TABLE IF EXISTS rw_fund_transfers;

DROP INDEX IF EXISTS idx_rw_fund_locations_category;
DROP INDEX IF EXISTS idx_rw_fund_locations_rw_active;
DROP INDEX IF EXISTS uniq_rw_fund_locations_rw_lower_name;
DROP TABLE IF EXISTS rw_fund_locations;

