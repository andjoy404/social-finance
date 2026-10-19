-- 027_rw_domain_and_transaction_reference.down.sql

DROP INDEX IF EXISTS idx_transactions_reference_no_unique;

ALTER TABLE transactions
    DROP COLUMN IF EXISTS reference_no;

DROP INDEX IF EXISTS idx_rts_rw_id;

ALTER TABLE rts
    DROP COLUMN IF EXISTS rw_id;

DROP INDEX IF EXISTS uniq_rws_active;

DROP TABLE IF EXISTS rws;
