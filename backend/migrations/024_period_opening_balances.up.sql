-- 024_period_opening_balances.up.sql
-- Table for monthly period opening balance tracking and manual correction audit trail.

CREATE TABLE IF NOT EXISTS period_opening_balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    period text NOT NULL,
    opening_balance NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
    source text NOT NULL DEFAULT 'automatic' CHECK (source IN ('automatic', 'manual')),
    previous_balance NUMERIC(15, 2) NULL,
    reason text NULL,
    updated_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_period_opening_balances_rt_period UNIQUE (rt_id, period)
);

CREATE INDEX IF NOT EXISTS idx_period_opening_balances_rt_id ON period_opening_balances(rt_id);

