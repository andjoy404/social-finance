-- 009_finance.up.sql
-- Financial Management Domain: Categories, Dues, Bills, Payments, Immutable Transactions Ledger, Idempotency, and Audit Logs

-- 1. Financial Categories (Master income/expense categories per RT)
CREATE TABLE IF NOT EXISTS financial_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uniq_financial_categories_rt_name UNIQUE (rt_id, name)
);

CREATE INDEX IF NOT EXISTS idx_financial_categories_rt_id ON financial_categories(rt_id);
CREATE INDEX IF NOT EXISTS idx_financial_categories_rt_active ON financial_categories(rt_id, is_active);

-- 2. Dues Configuration
CREATE TABLE IF NOT EXISTS dues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    period_type TEXT NOT NULL CHECK (period_type IN ('monthly', 'yearly', 'one_time')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_dues_rt_id ON dues(rt_id);
CREATE INDEX IF NOT EXISTS idx_dues_rt_active ON dues(rt_id, is_active);

-- 3. Bills (Linked to household_occupancies and dues)
CREATE TABLE IF NOT EXISTS bills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    household_occupancy_id UUID NOT NULL REFERENCES household_occupancies(id) ON DELETE RESTRICT,
    due_id UUID NOT NULL REFERENCES dues(id) ON DELETE RESTRICT,
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    period TEXT NOT NULL,
    due_date DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'unpaid' CHECK (status IN ('unpaid', 'paid', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uniq_bills_occupancy_due_period UNIQUE (household_occupancy_id, due_id, period)
);

CREATE INDEX IF NOT EXISTS idx_bills_rt_id ON bills(rt_id);
CREATE INDEX IF NOT EXISTS idx_bills_occupancy_id ON bills(household_occupancy_id);
CREATE INDEX IF NOT EXISTS idx_bills_due_id ON bills(due_id);
CREATE INDEX IF NOT EXISTS idx_bills_status ON bills(status);
CREATE INDEX IF NOT EXISTS idx_bills_period ON bills(period);

-- 4. Payments
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    bill_id UUID NOT NULL REFERENCES bills(id) ON DELETE RESTRICT,
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    method TEXT NOT NULL CHECK (method IN ('CASH', 'TRANSFER')),
    origin TEXT NOT NULL CHECK (origin IN ('SELF_SUBMITTED', 'STAFF_RECORDED')),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    proof_path TEXT,
    paid_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    verified_by UUID REFERENCES users(id) ON DELETE SET NULL,
    verified_at TIMESTAMPTZ,
    rejection_reason TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payments_rt_id ON payments(rt_id);
CREATE INDEX IF NOT EXISTS idx_payments_bill_id ON payments(bill_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);

-- 5. Transactions Ledger (Immutable)
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES financial_categories(id) ON DELETE RESTRICT,
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    status TEXT NOT NULL DEFAULT 'posted' CHECK (status IN ('draft', 'posted', 'cancelled')),
    reverses_transaction_id UUID REFERENCES transactions(id) ON DELETE RESTRICT,
    payment_id UUID REFERENCES payments(id) ON DELETE RESTRICT,
    description TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_transactions_rt_id ON transactions(rt_id);
CREATE INDEX IF NOT EXISTS idx_transactions_category_id ON transactions(category_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_reverses_id ON transactions(reverses_transaction_id);
CREATE INDEX IF NOT EXISTS idx_transactions_payment_id ON transactions(payment_id);
CREATE INDEX IF NOT EXISTS idx_transactions_occurred_at ON transactions(occurred_at);

-- 6. Idempotency Keys (Deduplication for financial writes)
CREATE TABLE IF NOT EXISTS idempotency_keys (
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_path TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    response_code INT NOT NULL,
    response_body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (rt_id, key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_created_at ON idempotency_keys(created_at);

-- 7. Audit Logs (Append-only audit trail)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    old_values JSONB,
    new_values JSONB,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_rt_id ON audit_logs(rt_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id);

