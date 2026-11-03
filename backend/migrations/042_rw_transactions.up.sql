-- 042_rw_transactions.up.sql
-- Create rw_transactions table for RW-level independent accounting.
-- Brief 2 (Koreksi Fundamental): KAS RT and KAS RW are completely independent
-- ledgers — no aggregation in either direction.

CREATE TABLE rw_transactions (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rw_id                   uuid NOT NULL REFERENCES rws(id) ON DELETE CASCADE,
    category_id             uuid REFERENCES financial_categories(id) ON DELETE SET NULL,
    account_id              uuid REFERENCES fund_locations(id) ON DELETE SET NULL,
    amount                  numeric(15,2) NOT NULL CHECK (amount > 0),
    type                    text NOT NULL CHECK (type IN ('income', 'expense')),
    status                  text NOT NULL DEFAULT 'posted' CHECK (status IN ('draft', 'posted', 'cancelled')),
    reverses_transaction_id uuid REFERENCES rw_transactions(id) ON DELETE RESTRICT,
    payment_id              uuid REFERENCES payments(id) ON DELETE SET NULL,
    description             text NOT NULL,
    occurred_at             timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at              timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    payment_method          text CHECK (payment_method IS NULL OR payment_method IN ('CASH', 'TRANSFER')),
    created_by              uuid REFERENCES users(id) ON DELETE SET NULL,
    reference_no            text
);

-- Partial unique index for reference_no (unique where NOT NULL)
CREATE UNIQUE INDEX IF NOT EXISTS uniq_rw_transactions_reference_no ON rw_transactions (reference_no) WHERE reference_no IS NOT NULL;

-- Indexes
CREATE INDEX idx_rw_transactions_rw_id ON rw_transactions(rw_id);
CREATE INDEX idx_rw_transactions_status ON rw_transactions(status);
CREATE INDEX idx_rw_transactions_type ON rw_transactions(type);
CREATE INDEX idx_rw_transactions_occurred_at ON rw_transactions(occurred_at);
CREATE INDEX idx_rw_transactions_reverses_id ON rw_transactions(reverses_transaction_id);
CREATE INDEX idx_rw_transactions_category ON rw_transactions(category_id);
CREATE INDEX idx_rw_transactions_payment ON rw_transactions(payment_id);
CREATE INDEX idx_rw_transactions_created_by ON rw_transactions(created_by);
