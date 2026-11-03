-- 043_rw_opening_balance.up.sql
-- RW KAS independence fix (Brief 3): Saldo KAS RW dihitung dari pembukuan RW,
-- bukan agregasi lokasi dana RT.
--
-- Rumus: Saldo Akhir KAS RW = Saldo Awal RW + Total Pemasukan RW - Total Pengeluaran RW

-- Tabel opening balance per RW
CREATE TABLE rw_opening_balances (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rw_id       uuid NOT NULL REFERENCES rws(id) ON DELETE CASCADE,
    amount      numeric(15,2) NOT NULL,
    description text NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at  timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_rw_opening_balances_rw_id ON rw_opening_balances(rw_id);
