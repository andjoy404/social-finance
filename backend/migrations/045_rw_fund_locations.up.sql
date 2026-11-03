-- 045_rw_fund_locations.up.sql
-- Lokasi Dana RW, Transfer Antar-Lokasi RW, Ledger Mutasi Append-Only RW, dan Integrasi Transaksi RW.

-- 1. Tabel Master Lokasi Dana RW
CREATE TABLE IF NOT EXISTS rw_fund_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rw_id UUID NOT NULL REFERENCES rws(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('rekening', 'brankas', 'coh')),
    bank_name TEXT,
    account_number TEXT,
    account_holder TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uniq_rw_fund_locations_id_rw UNIQUE (id, rw_id),
    CONSTRAINT chk_rw_fund_locations_bank_info CHECK (
        (category = 'rekening' AND bank_name IS NOT NULL AND TRIM(bank_name) != '') OR
        (category IN ('brankas', 'coh'))
    )
);

-- Indeks unik Case-Insensitive & Trimmed Name per RW
CREATE UNIQUE INDEX IF NOT EXISTS uniq_rw_fund_locations_rw_lower_name
ON rw_fund_locations (rw_id, LOWER(TRIM(name)));

CREATE INDEX IF NOT EXISTS idx_rw_fund_locations_rw_active ON rw_fund_locations (rw_id, is_active);
CREATE INDEX IF NOT EXISTS idx_rw_fund_locations_category ON rw_fund_locations (rw_id, category);

-- 2. Tabel Pemindahan Dana Antar-Lokasi RW (RW Fund Transfers)
CREATE TABLE IF NOT EXISTS rw_fund_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rw_id UUID NOT NULL REFERENCES rws(id) ON DELETE RESTRICT,
    from_location_id UUID NOT NULL,
    to_location_id UUID NOT NULL,
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    transfer_date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reverses_transfer_id UUID REFERENCES rw_fund_transfers(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_rw_fund_transfers_diff_locations CHECK (from_location_id != to_location_id),
    CONSTRAINT uniq_rw_fund_transfers_reversal UNIQUE (reverses_transfer_id),
    CONSTRAINT fk_rw_transfers_from_location_rw FOREIGN KEY (from_location_id, rw_id)
        REFERENCES rw_fund_locations(id, rw_id) ON DELETE RESTRICT,
    CONSTRAINT fk_rw_transfers_to_location_rw FOREIGN KEY (to_location_id, rw_id)
        REFERENCES rw_fund_locations(id, rw_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_rw_fund_transfers_rw_date ON rw_fund_transfers (rw_id, transfer_date);
CREATE INDEX IF NOT EXISTS idx_rw_fund_transfers_from_loc ON rw_fund_transfers (from_location_id);
CREATE INDEX IF NOT EXISTS idx_rw_fund_transfers_to_loc ON rw_fund_transfers (to_location_id);

-- 3. Modifikasi Tabel rw_transactions Existing (Penambahan location_id & source_rt_id)
ALTER TABLE rw_transactions
ADD COLUMN IF NOT EXISTS location_id UUID;

ALTER TABLE rw_transactions
ADD COLUMN IF NOT EXISTS source_rt_id UUID REFERENCES rts(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_rw_transactions_location_rw'
    ) THEN
        ALTER TABLE rw_transactions
        ADD CONSTRAINT fk_rw_transactions_location_rw
        FOREIGN KEY (location_id, rw_id) REFERENCES rw_fund_locations(id, rw_id) ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_rw_transactions_location_id ON rw_transactions(location_id);
CREATE INDEX IF NOT EXISTS idx_rw_transactions_source_rt_id ON rw_transactions(source_rt_id);

-- 4. Tabel Ledger Mutasi Lokasi Dana RW (RW Location Balance Changes - Append Only)
CREATE TABLE IF NOT EXISTS rw_location_balance_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rw_id UUID NOT NULL REFERENCES rws(id) ON DELETE RESTRICT,
    location_id UUID NOT NULL,
    transaction_id UUID REFERENCES rw_transactions(id) ON DELETE RESTRICT,
    transfer_id UUID REFERENCES rw_fund_transfers(id) ON DELETE RESTRICT,
    change_type TEXT NOT NULL,
    amount_delta NUMERIC(15, 2) NOT NULL CHECK (amount_delta != 0),
    notes TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_rw_lbc_location_rw FOREIGN KEY (location_id, rw_id)
        REFERENCES rw_fund_locations(id, rw_id) ON DELETE RESTRICT,
    CONSTRAINT chk_rw_lbc_change_type_source CHECK (
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

CREATE INDEX IF NOT EXISTS idx_rw_lbc_rw_loc ON rw_location_balance_changes (rw_id, location_id);
CREATE INDEX IF NOT EXISTS idx_rw_lbc_created_at ON rw_location_balance_changes (created_at);

-- Partial Unique Index Anti-Duplikasi: Satu transaksi kas hanya boleh menghasilkan 1 mutasi LBC
CREATE UNIQUE INDEX IF NOT EXISTS uniq_rw_lbc_transaction_id
ON rw_location_balance_changes (transaction_id)
WHERE transaction_id IS NOT NULL;

-- Partial Unique Index Anti-Duplikasi: Satu transfer hanya boleh menghasilkan 1 transfer_out dan 1 transfer_in
CREATE UNIQUE INDEX IF NOT EXISTS uniq_rw_lbc_transfer_change_type
ON rw_location_balance_changes (transfer_id, change_type)
WHERE transfer_id IS NOT NULL;

