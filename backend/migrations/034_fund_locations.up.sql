-- 033_fund_locations.up.sql
-- Fitur Lokasi Dana: Master Lokasi, Transfer Antar-Lokasi, Ledger Append-Only, dan Integrasi Transaksi/Payment

-- 1. Tabel Master Lokasi Dana
CREATE TABLE IF NOT EXISTS fund_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('rekening', 'brankas', 'coh')),
    bank_name TEXT,
    account_number TEXT,
    account_holder TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uniq_fund_locations_id_rt UNIQUE (id, rt_id),
    CONSTRAINT chk_fund_locations_bank_info CHECK (
        (category = 'rekening' AND bank_name IS NOT NULL AND TRIM(bank_name) != '') OR
        (category IN ('brankas', 'coh'))
    )
);

-- Indeks unik berbasis ekspresi PostgreSQL yang valid (Case-Insensitive & Trimmed Name per RT)
CREATE UNIQUE INDEX IF NOT EXISTS uniq_fund_locations_rt_lower_name
ON fund_locations (rt_id, LOWER(TRIM(name)));

CREATE INDEX IF NOT EXISTS idx_fund_locations_rt_active ON fund_locations (rt_id, is_active);
CREATE INDEX IF NOT EXISTS idx_fund_locations_category ON fund_locations (rt_id, category);

-- 2. Tabel Pemindahan Dana Antar-Lokasi (Fund Transfers)
CREATE TABLE IF NOT EXISTS fund_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    from_location_id UUID NOT NULL,
    to_location_id UUID NOT NULL,
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    transfer_date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reverses_transfer_id UUID REFERENCES fund_transfers(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_fund_transfers_diff_locations CHECK (from_location_id != to_location_id),
    CONSTRAINT uniq_fund_transfers_reversal UNIQUE (reverses_transfer_id),
    CONSTRAINT fk_transfers_from_location_rt FOREIGN KEY (from_location_id, rt_id)
        REFERENCES fund_locations(id, rt_id) ON DELETE RESTRICT,
    CONSTRAINT fk_transfers_to_location_rt FOREIGN KEY (to_location_id, rt_id)
        REFERENCES fund_locations(id, rt_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_fund_transfers_rt_date ON fund_transfers (rt_id, transfer_date);
CREATE INDEX IF NOT EXISTS idx_fund_transfers_from_loc ON fund_transfers (from_location_id);
CREATE INDEX IF NOT EXISTS idx_fund_transfers_to_loc ON fund_transfers (to_location_id);

-- 3. Tabel Ledger Mutasi Lokasi Dana (Location Balance Changes - Append Only)
CREATE TABLE IF NOT EXISTS location_balance_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE RESTRICT,
    location_id UUID NOT NULL,
    transaction_id UUID REFERENCES transactions(id) ON DELETE RESTRICT,
    transfer_id UUID REFERENCES fund_transfers(id) ON DELETE RESTRICT,
    change_type TEXT NOT NULL,
    amount_delta NUMERIC(15, 2) NOT NULL CHECK (amount_delta != 0),
    notes TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_lbc_location_rt FOREIGN KEY (location_id, rt_id)
        REFERENCES fund_locations(id, rt_id) ON DELETE RESTRICT,
    CONSTRAINT chk_lbc_change_type_source CHECK (
        (change_type = 'opening_balance' AND transaction_id IS NULL AND transfer_id IS NULL AND amount_delta > 0) OR
        (change_type = 'income' AND transaction_id IS NOT NULL AND transfer_id IS NULL AND amount_delta > 0) OR
        (change_type = 'expense' AND transaction_id IS NOT NULL AND transfer_id IS NULL AND amount_delta < 0) OR
        (change_type = 'transfer_out' AND transaction_id IS NULL AND transfer_id IS NOT NULL AND amount_delta < 0) OR
        (change_type = 'transfer_in' AND transaction_id IS NULL AND transfer_id IS NOT NULL AND amount_delta > 0) OR
        (change_type = 'reversal_income' AND transaction_id IS NOT NULL AND transfer_id IS NULL AND amount_delta < 0) OR
        (change_type = 'reversal_expense' AND transaction_id IS NOT NULL AND transfer_id IS NULL AND amount_delta > 0) OR
        (change_type = 'reversal_transfer_out' AND transaction_id IS NULL AND transfer_id IS NOT NULL AND amount_delta > 0) OR
        (change_type = 'reversal_transfer_in' AND transaction_id IS NULL AND transfer_id IS NOT NULL AND amount_delta < 0)
    )
);

CREATE INDEX IF NOT EXISTS idx_lbc_rt_loc ON location_balance_changes (rt_id, location_id);
CREATE INDEX IF NOT EXISTS idx_lbc_created_at ON location_balance_changes (created_at);

-- Partial Unique Index Anti-Duplikasi: Satu transaksi kas hanya boleh menghasilkan 1 mutasi LBC
CREATE UNIQUE INDEX IF NOT EXISTS uniq_lbc_transaction_id
ON location_balance_changes (transaction_id)
WHERE transaction_id IS NOT NULL;

-- Partial Unique Index Anti-Duplikasi: Satu transfer hanya boleh menghasilkan 1 transfer_out dan 1 transfer_in
CREATE UNIQUE INDEX IF NOT EXISTS uniq_lbc_transfer_change_type
ON location_balance_changes (transfer_id, change_type)
WHERE transfer_id IS NOT NULL;

-- 4. Modifikasi Tabel transactions Existing (Penambahan location_id & Composite FK Isolasi Tenant)
ALTER TABLE transactions
ADD COLUMN IF NOT EXISTS location_id UUID;

ALTER TABLE transactions
DROP CONSTRAINT IF EXISTS fk_transactions_location_rt;

ALTER TABLE transactions
ADD CONSTRAINT fk_transactions_location_rt
FOREIGN KEY (location_id, rt_id) REFERENCES fund_locations(id, rt_id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_transactions_location_id ON transactions (location_id);

-- 5. Modifikasi Tabel payments Existing (Dukungan Status REVERSED)
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'REVERSED'));
