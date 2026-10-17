-- Migration 026 down

DROP INDEX IF EXISTS idx_transactions_created_by;

ALTER TABLE transactions
    DROP COLUMN IF EXISTS created_by;

