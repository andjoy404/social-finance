-- Migration 026: Add created_by to transactions and support transaction creator tracking

ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_transactions_created_by ON transactions(created_by);

-- Backfill created_by from audit_logs for existing manual transactions
UPDATE transactions t
SET created_by = al.user_id
FROM audit_logs al
WHERE al.entity_id = t.id::text
  AND al.entity_type = 'transaction'
  AND al.action = 'create_transaction'
  AND t.created_by IS NULL
  AND al.user_id IS NOT NULL;

