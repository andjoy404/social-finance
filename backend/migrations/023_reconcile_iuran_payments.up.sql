-- 023_reconcile_iuran_payments.up.sql
-- Rekonsiliasi approved payments (seed) yang belum memiliki entry ledger ke transaksi KAS income.
--
-- Migration ini mengisi tabel `transactions` dari approved payments yang
-- tidak memiliki entry ledger. Desain atomik — semua atau gagal seluruhnya.
--
-- Identitas transaksi migration (untuk rollback yang presisi):
--   - Up migration membuat table `sf023_tx_ids` yang menyimpan UUID transaksi yang dibuat.
--   - Description format: 'sf:023:tx:' || payment_uuid — juga untuk audit trail.
--   - Table `sf023_tx_ids` di-drop otomatis di down migration setelah digunakan.
--
-- Idempotensi: guarded oleh `NOT IN (SELECT payment_id FROM transactions ...)`,
-- sehingga rerun aman (jika migration sudah pernah berjalan, SELECT akan empty
-- dan INSERT tidak akan menambah duplikat).
--
-- BATASAN KONKURENSI: Guard `NOT IN` hanya serial-safe, TIDAK mengimplementasikan
-- `FOR UPDATE SKIP LOCKED` atau partial unique index pada payment_id. Migrasi paralel
-- (multi-proses `docker compose run --rm migrate up` bersamaan) DAPAT menciptakan
-- duplikat transaksi jika kedua proses melihat payment_id yang sama masih
-- belum ada di transactions sebelum salah satunya INSERT.
--
-- Praktik deployment yang aman: jalankan migration secara serial (satu per
-- satu), jangan paralel. Gunakan `docker compose run --rm migrate up` tanpa
-- proses migrate lain yang berjalan bersamaan.
--
-- Precondition check:
--   - Setiap payment eligible HARUS punya kategori income "Iuran Warga" yang aktif di RT yang sama.
--   - Jika precondition gagal, DO $$ akan RAISE EXCEPTION dan rollback seluruh transaksi.
--
-- Validasi akhir (postflight):
--   - Payment IDs eligible DISIMPAN di temp table `sf023_eligible_payments` di Step 0.
--   - UUID transaksi yang dibuat DISIMPAN di `sf023_tx_ids` table di Step 1.
--   - Setelah INSERT, Step 2 memverifikasi SEMUA payment IDs di temp table
--     sekarang punya transaksi terkait.
--   - Jika ada payment ID di temp table yang TIDAK punya transaksi → RAISE EXCEPTION.
--   - Jika ada transaksi baru yang TIDAK ada di sf023_tx_ids → RAISE EXCEPTION.
--
-- Eksekusi:
--   docker compose run --rm migrate up 23

BEGIN;

-- ============================================================
-- Step 0: Preflight — verifikasi dan SIMPAN daftar payment IDs
--             yang eligible untuk migrasi
-- ============================================================
-- Temp table bersifat session-scoped, di-drop saat transaction commit/rollback.
CREATE TEMPORARY TABLE sf023_eligible_payments (
    payment_id UUID PRIMARY KEY
) ON COMMIT DROP;

DO $$
DECLARE
    preflight_count INT;
    missing_count   INT;
    pay_id          UUID;
BEGIN
    -- Simpan daftar payment IDs yang eligible (belum punya transaksi)
    FOR pay_id IN
        SELECT p.id
        FROM payments p
        INNER JOIN financial_categories fc
            ON p.rt_id = fc.rt_id
           AND fc.name = 'Iuran Warga'
           AND fc.type = 'income'
           AND fc.is_active = true
        WHERE p.status = 'APPROVED'
          AND p.bill_id IS NOT NULL
          AND p.id NOT IN (
              SELECT payment_id FROM transactions WHERE payment_id IS NOT NULL
          )
    LOOP
        INSERT INTO sf023_eligible_payments (payment_id) VALUES (pay_id);
    END LOOP;

    SELECT COUNT(*)
    INTO preflight_count
    FROM sf023_eligible_payments;

    RAISE NOTICE 'Preflight: % approved payment(s) eligible for migration 023.', preflight_count;

    -- Verifikasi kategori tersedia untuk semua eligible payments
    SELECT COUNT(*)
    INTO missing_count
    FROM payments p
    LEFT JOIN financial_categories fc
        ON p.rt_id = fc.rt_id
       AND fc.name = 'Iuran Warga'
       AND fc.type = 'income'
       AND fc.is_active = true
    WHERE p.status = 'APPROVED'
      AND p.bill_id IS NOT NULL
      AND fc.id IS NULL;

    IF missing_count > 0 THEN
        RAISE EXCEPTION 'PREFLIGHT FAILED: % approved payment(s) tanpa kategori "Iuran Warga" aktif di RT-nya. Buat kategori terlebih dahulu.', missing_count;
    END IF;

    IF preflight_count = 0 THEN
        RAISE NOTICE 'Preflight: No eligible payments found. Migration is a no-op.';
    END IF;
END $$;

-- ============================================================
-- Step 1: Insert transactions untuk payments yang belum punya ledger
-- ============================================================
-- Menggunakan INNER JOIN untuk memverifikasi kategori sekaligus
-- memasukkan kategori_id ke dalam INSERT.
--
-- Description format: 'sf:023:tx:' || payment_uuid
-- Prefix 'sf:023:tx:' unik untuk migration ini dan tidak akan pernah
-- dihasilkan oleh service layer (CreatePayment selalu menggunakan description
-- dari domain logic, bukan prefix internal seperti ini).
--
-- UUID transaksi yang dibuat disimpan ke table tracking `sf023_tx_ids`
-- untuk referensi rollback presisi di down migration.
--
-- [TABLE TRACKING]
-- `sf023_tx_ids`: regular table, di-drop oleh down migration setelah digunakan.
CREATE TABLE IF NOT EXISTS sf023_tx_ids (
    tx_id      UUID PRIMARY KEY,
    payment_id UUID NOT NULL
);

INSERT INTO transactions (
    id,
    rt_id,
    category_id,
    amount,
    type,
    status,
    reverses_transaction_id,
    payment_id,
    description,
    occurred_at,
    created_at,
    updated_at
)
SELECT
    gen_random_uuid(),                          -- id (auto-uuid)
    p.rt_id,                                    -- RT yang sama dengan payment
    fc.id,                                      -- kategori "Iuran Warga" untuk RT ini
    p.amount,                                   -- nominal sama dengan payment
    'income',                                   -- jenis pemasukan
    'posted',                                   -- langsung posted (计入 balance)
    NULL,                                        -- belum ada reversal
    p.id,                                        -- FK ke source payment
    'sf:023:tx:' || p.id::text,                 -- audit trail: prefix unik + payment_uuid
    p.created_at,                               -- timestamp dari payment creation
    CURRENT_TIMESTAMP,                           -- created_at (waktu migrasi berjalan)
    CURRENT_TIMESTAMP                            -- updated_at (waktu migrasi berjalan)
FROM payments p
INNER JOIN financial_categories fc
    ON p.rt_id = fc.rt_id
   AND fc.name = 'Iuran Warga'
   AND fc.type = 'income'
   AND fc.is_active = true
WHERE p.status = 'APPROVED'
  AND p.bill_id IS NOT NULL                    -- hanya bill payments (exclude donations via due_id)
  AND p.id NOT IN (
      SELECT payment_id FROM transactions WHERE payment_id IS NOT NULL
  );

-- Simpan UUID transaksi yang baru dibuat ke table tracking untuk rollback presisi
INSERT INTO sf023_tx_ids (tx_id, payment_id)
SELECT t.id, t.payment_id
FROM transactions t
INNER JOIN sf023_eligible_payments e ON e.payment_id = t.payment_id
WHERE t.description LIKE 'sf:023:tx:%';

-- ============================================================
-- Step 2: Postflight — verifikasi SEMUA eligible payments diproses
-- ============================================================
-- Validasi dilakukan dengan join ke temp table `sf023_eligible_payments`
-- dan table tracking `sf023_tx_ids`.
DO $$
DECLARE
    missing_tx_count INT;
    expected_count   INT;
    actual_tx_count  INT;
BEGIN
    -- expected_count = jumlah payment IDs yang seharusnya diproses
    SELECT COUNT(*)
    INTO expected_count
    FROM sf023_eligible_payments;

    -- actual_tx_count = jumlah transaksi yang dibuat migration (dari tracking table)
    SELECT COUNT(*)
    INTO actual_tx_count
    FROM sf023_tx_ids;

    -- Validasi 1: expected == actual
    IF expected_count <> actual_tx_count THEN
        RAISE EXCEPTION
            'POSTFLIGHT FAILED: expected % transaction(s) to be created, but tracking table shows %. Migration may have partially failed.',
            expected_count, actual_tx_count;
    END IF;

    -- Validasi 2: Semua payment IDs di temp table HARUS punya transaksi terkait
    SELECT COUNT(*)
    INTO missing_tx_count
    FROM sf023_eligible_payments e
    WHERE NOT EXISTS (
        SELECT 1 FROM sf023_tx_ids x WHERE x.payment_id = e.payment_id
    );

    IF missing_tx_count > 0 THEN
        RAISE EXCEPTION
            'POSTFLIGHT FAILED: % eligible payment(s) did not produce a tracked transaction. Rollback and investigate.',
            missing_tx_count;
    END IF;

    RAISE NOTICE 'Postflight OK: % transaction(s) created and tracked for % eligible payment(s).', actual_tx_count, expected_count;
END $$;

COMMIT;
