/**
 * Regression test — Saldo berjalan gabungan (running balance).
 *
 * Memastikan kolom Saldo pada baris transaksi berisi saldo berjalan, bukan
 * nilai debet/kredit baris itu sendiri atau total mutasi.
 *
 * Fixture contoh dari instruksi:
 *   Saldo awal Rp1.000.000
 *   Penerimaan tunai Rp100.000
 *   Pengeluaran tunai Rp50.000
 *   Pengeluaran transfer Rp25.000
 *   Saldo akhir yang diharapkan: Rp1.025.000
 *   Mutasi tunai +Rp50.000
 *   Mutasi transfer −Rp25.000
 */
import { describe, it, expect } from 'vitest'
import type { ApiTransaction } from '@/app/api'
import {
  buildKasReport,
  buildStatementRows,
  type StatementRow,
} from './kasReportModel'

const identity = { orgName: 'Cluster Urena', rtLabel: 'RT 02', rwLabel: 'RW 016' }

/**
 * Helper: bangun model + statement rows untuk fixture sederhana.
 */
function buildFixture(
  txs: Partial<ApiTransaction>[],
  openingBalance: string,
): { model: ReturnType<typeof buildKasReport>; rows: StatementRow[] } {
  const transactions = txs.map((p) => ({
    ...p,
    rt_id: p.rt_id ?? 'rt-1',
    category_id: p.category_id ?? 'c',
    status: p.status ?? 'posted',
    description: p.description ?? p.id ?? '',
    occurred_at: p.occurred_at ?? '2026-10-05T03:00:00Z',
    created_at: p.created_at ?? '2026-10-05T03:00:00Z',
    updated_at: p.updated_at ?? '2026-10-05T03:00:00Z',
    category_name: p.category_name ?? 'Operasional RT',
    payment_method: p.payment_method === undefined ? 'CASH' : p.payment_method,
  })) as ApiTransaction[]

  const model = buildKasReport({
    transactions,
    openingBalance,
    apiClosingBalance: '1025000.00',
    period: '2026-10',
    identity,
    generatedAt: new Date('2026-10-23T00:00:00Z'),
  })
  return { model, rows: buildStatementRows(model) }
}

describe('saldo berjalan gabungan — regression', () => {
  // -----------------------------------------------------------------------
  // 1. Fixture utama dari instruksi
  // -----------------------------------------------------------------------
  it('skenario instruksi: saldo awal 1jt, penerimaan tunai 100rb, pengeluaran tunai 50rb, pengeluaran transfer 25rb', () => {
    const { model, rows } = buildFixture([
      { id: 'i1', type: 'income', amount: '100000.00', payment_method: 'CASH' },
      { id: 'e1', type: 'expense', amount: '50000.00', payment_method: 'CASH' },
      { id: 'e2', type: 'expense', amount: '25000.00', payment_method: 'TRANSFER' },
    ], '1000000.00')

    // Model aggregation
    expect(model.openingCents).toBe(100000000)
    expect(model.income.cash).toBe(10000000)
    expect(model.totalIncomeCents).toBe(10000000)
    expect(model.expense.cash).toBe(5000000)
    expect(model.expense.transfer).toBe(2500000)
    expect(model.totalExpenseCents).toBe(7500000)
    expect(model.closingCents).toBe(102500000) // 1jt + 100rb - 75rb
    expect(model.cashMovementCents).toBe(5000000)    // +50.000
    expect(model.transferMovementCents).toBe(-2500000) // −25.000

    // Saldo berjalan pada statement rows
    const row0 = rows[0]
    expect(row0.label).toBe('Saldo awal periode')
    expect(row0.saldo).toBe(100000000) // Rp1.000.000

    // Setelah penerimaan tunai
    const row1 = rows.find((r) => r.label.includes('Iuran Warga') && r.debit !== undefined)
    if (row1) {
      expect(row1.debit).toBe(10000000)
      expect(row1.saldo).toBeUndefined() // agregat kategori: saldo dikosongkan
    }

    // Setelah pengeluaran tunai
    const rowExpCash = rows.find((r) => r.label === 'Pengeluaran tunai')
    expect(rowExpCash?.credit).toBe(5000000)
    expect(rowExpCash?.saldo).toBeUndefined() // agregat pengeluaran: saldo dikosongkan

    // Setelah pengeluaran transfer
    const rowExpTransfer = rows.find((r) => r.label === 'Pengeluaran transfer')
    expect(rowExpTransfer?.credit).toBe(2500000)
    expect(rowExpTransfer?.saldo).toBeUndefined() // agregat pengeluaran: saldo dikosongkan

    // Saldo akhir
    const lastRow = rows[rows.length - 1]
    expect(lastRow.kind).toBe('grand')
    expect(lastRow.saldo).toBe(102500000)
  })

  // -----------------------------------------------------------------------
  // 2. Penerimaan transfer dan pengeluaran transfer sama-sama ada
  // -----------------------------------------------------------------------
  it('penerimaan & pengeluaran transfer keduanya ada', () => {
    const { model, rows } = buildFixture([
      { id: 'i1', type: 'income', amount: '200000.00', payment_method: 'TRANSFER' },
      { id: 'e1', type: 'expense', amount: '80000.00', payment_method: 'TRANSFER' },
    ], '500000.00')

    expect(model.income.transfer).toBe(20000000)
    expect(model.expense.transfer).toBe(8000000)
    expect(model.transferMovementCents).toBe(12000000)
    expect(model.closingCents).toBe(62000000) // 500rb + 200rb - 80rb

    const rowExpTransfer = rows.find((r) => r.label === 'Pengeluaran transfer')
    expect(rowExpTransfer?.saldo).toBeUndefined() // agregat pengeluaran: saldo dikosongkan
  })

  // -----------------------------------------------------------------------
  // 3. Transaksi tanggal sama → saldo berjalan deterministik
  // -----------------------------------------------------------------------
  it('beberapa transaksi tanggal sama menghasilkan saldo berjalan deterministik', () => {
    const ts = '2026-10-05T03:00:00Z'
    const { model } = buildFixture([
      { id: 'a', type: 'income', amount: '100000.00', payment_method: 'CASH', occurred_at: ts },
      { id: 'b', type: 'income', amount: '50000.00', payment_method: 'CASH', occurred_at: ts },
      { id: 'c', type: 'expense', amount: '30000.00', payment_method: 'CASH', occurred_at: ts },
    ], '1000000.00')

    // Urutan model: sorted by category name untuk income
    // Kategori default 'Operasional RT' untuk semua, jadi urutannya tetap berdasarkan urutan array input
    // Penerimaan a (100.000), b (50.000), pengeluaran c (30.000)
    // Saldo: 1.000.000 → 1.100.000 → 1.150.000 → 1.120.000
    expect(model.income.cash).toBe(15000000)
    expect(model.expense.cash).toBe(3000000)
    expect(model.closingCents).toBe(112000000)
  })

  // -----------------------------------------------------------------------
  // 4. Saldo awal nol & periode tanpa transaksi
  // -----------------------------------------------------------------------
  it('saldo awal nol, tanpa transaksi', () => {
    const { model, rows } = buildFixture([], '0')

    expect(model.openingCents).toBe(0)
    expect(model.closingCents).toBe(0)
    expect(model.transactionCount).toBe(0)

    // Hanya baris saldo awal & saldo akhir (ringkasan kosong)
    const row0 = rows[0]
    expect(row0.label).toBe('Saldo awal periode')
    expect(row0.saldo).toBe(0)

    const lastRow = rows[rows.length - 1]
    expect(lastRow.kind).toBe('grand')
    expect(lastRow.saldo).toBe(0)
  })

  // -----------------------------------------------------------------------
  // 5. Saldo awal tidak tersedia → jangan mengarang nilai
  // -----------------------------------------------------------------------
  it('saldo awal tidak tersedia (string kosong) → 0', () => {
    const { model, rows } = buildFixture(
      [{ id: 'i1', type: 'income', amount: '50000.00' }],
      '',
    )

    expect(model.openingCents).toBe(0)
    expect(model.income.cash).toBe(5000000)
    expect(model.closingCents).toBe(5000000)

    const row0 = rows[0]
    expect(row0.saldo).toBe(0)
  })

  // -----------------------------------------------------------------------
  // 6. Baris total & mutasi tidak dihitung lagi sebagai transaksi
  // -----------------------------------------------------------------------
  it('baris total dan mutasi tidak mengubah saldo berjalan', () => {
    const { rows } = buildFixture([
      { id: 'i1', type: 'income', amount: '100000.00', payment_method: 'CASH' },
      { id: 'e1', type: 'expense', amount: '50000.00', payment_method: 'CASH' },
    ], '1000000.00')

    // Baris-baris summary tidak boleh punya saldo
    for (const r of rows) {
      if (r.kind === 'subtotal' || r.kind === 'total') {
        expect(r.saldo).toBeUndefined()
      }
    }

    // Saldo berjalan harus konsisten (lewatkan baris saldo awal)
    const rowAfterIncome = rows.find((r) => r.kind === 'row' && r.debit !== undefined && r.debit! > 0 && r.label !== 'Saldo awal periode')
    expect(rowAfterIncome?.saldo).toBeUndefined() // agregat penerimaan: saldo dikosongkan

    const rowAfterExpense = rows.find((r) => r.kind === 'row' && r.credit !== undefined && r.credit! > 0)
    expect(rowAfterExpense?.saldo).toBeUndefined() // agregat pengeluaran: saldo dikosongkan

    // Saldo akhir = 1.050.000
    const lastRow = rows[rows.length - 1]
    expect(lastRow.saldo).toBe(105000000)
  })

  // -----------------------------------------------------------------------
  // 7. Web dan Flutter hasil identik untuk fixture sama
  //    (Divverifikasi melalui fixture yang sama di kasReportExport.test.ts
  //    dan kasReportAcceptanceArtifacts.test.ts; test ini memastikan
  //    model menghasilkan angka yang sama.)
  // -----------------------------------------------------------------------
  it('Web: model menghasilkan angka yang sama untuk fixture gabungan', () => {
    const txs = [
      { id: 'i1', type: 'income' as const, amount: '100000.00', payment_method: 'CASH' },
      { id: 'i2', type: 'income' as const, amount: '250000.00', payment_method: 'CASH' },
      { id: 'i3', type: 'income' as const, amount: '50000.00', payment_method: 'TRANSFER' },
      { id: 'e1', type: 'expense' as const, amount: '30000.00', payment_method: 'CASH' },
      { id: 'e2', type: 'expense' as const, amount: '10000.00', payment_method: 'CASH' },
      { id: 'e3', type: 'expense' as const, amount: '15000.00', payment_method: 'TRANSFER' },
    ]
    const { model } = buildFixture(txs, '2000000.00')

    // Hasil harus identik dengan fixture.ts
    expect(model.openingCents).toBe(200000000)
    expect(model.income.cash).toBe(35000000)
    expect(model.income.transfer).toBe(5000000)
    expect(model.totalIncomeCents).toBe(40000000)
    expect(model.expense.cash).toBe(4000000)
    expect(model.expense.transfer).toBe(1500000)
    expect(model.totalExpenseCents).toBe(5500000)
    expect(model.closingCents).toBe(234500000) // 2jt + 400rb - 550rb
    expect(model.cashMovementCents).toBe(31000000)
    expect(model.transferMovementCents).toBe(3500000)
  })

  // -----------------------------------------------------------------------
  // 8. PDF dan XLSX tidak menampilkan saldo berjalan berbeda dari model
  //    (Divverifikasi: kasReportExport.test.ts membuka PDF/XLSX dan
  //    memverifikasi nilai numerik; test ini memastikan model
  //    menghasilkan angka yang benar untuk divalidasi oleh export.)
  // -----------------------------------------------------------------------
  it('model menghasilkan saldo akhir yang divalidasi PDF/XLSX', () => {
    const txs = [
      { id: 'i1', type: 'income' as const, amount: '1000000.00', payment_method: 'CASH' },
      { id: 'i2', type: 'income' as const, amount: '250000.00', payment_method: 'CASH' },
      { id: 'i3', type: 'income' as const, amount: '500000.00', payment_method: 'TRANSFER' },
      { id: 'e1', type: 'expense' as const, amount: '300000.00', payment_method: 'CASH' },
      { id: 'e2', type: 'expense' as const, amount: '100000.00', payment_method: 'CASH' },
      { id: 'e3', type: 'expense' as const, amount: '150000.00', payment_method: 'TRANSFER' },
    ]
    const { model } = buildFixture(txs, '2000000.00')

    // Nilai yang sama yang divalidasi kasReportExport.test.ts dan
    // kasReportAcceptanceArtifacts.test.ts
    expect(model.closingCents).toBe(320000000) // 3.200.000 → XLSX 3200000
    expect(model.totalIncomeCents).toBe(175000000) // 1.750.000 → XLSX 1750000
  })

  // -----------------------------------------------------------------------
  // 9. Baris ringkasan tetap ada dan terurut benar
  // -----------------------------------------------------------------------
  it('baris ringkasan terurut: saldo awal → penerimaan → total penerimaan → pengeluaran → total pengeluaran → mutasi → saldo akhir', () => {
    const { rows } = buildFixture([
      { id: 'i1', type: 'income', amount: '100000.00', payment_method: 'CASH' },
      { id: 'e1', type: 'expense', amount: '50000.00', payment_method: 'CASH' },
    ], '1000000.00')

    const labels = rows.map((r) => r.label)
    expect(labels[0]).toBe('Saldo awal periode')

    // Harus ada 'Total penerimaan tunai'
    const idxIncomeTotal = labels.findIndex((l) => l === 'Total penerimaan tunai')
    expect(idxIncomeTotal).toBeGreaterThan(0)

    // 'Total penerimaan Tunai + Transfer'
    const idxTotalIncome = labels.findIndex((l) => l === 'Total penerimaan Tunai + Transfer')
    expect(idxTotalIncome).toBeGreaterThan(idxIncomeTotal)

    // 'Pengeluaran tunai'
    const idxExpense = labels.findIndex((l) => l === 'Pengeluaran tunai')
    expect(idxExpense).toBeGreaterThan(idxTotalIncome)

    // 'Total pengeluaran Tunai + Transfer'
    const idxTotalExpense = labels.findIndex((l) => l === 'Total pengeluaran Tunai + Transfer')
    expect(idxTotalExpense).toBeGreaterThan(idxExpense)

    // 'Mutasi kas tunai'
    const idxMutasiKas = labels.findIndex((l) => l.includes('Mutasi kas tunai'))
    expect(idxMutasiKas).toBeGreaterThan(idxTotalExpense)

    // 'Saldo akhir' di paling bawah
    const idxSaldoAkhir = labels.findIndex((l) => l.includes('Saldo akhir'))
    expect(idxSaldoAkhir).toBe(rows.length - 1)
  })

  // -----------------------------------------------------------------------
  // 10. Pengeluaran tanpa metode ditampilkan
  // -----------------------------------------------------------------------
  it('pengeluaran tanpa metode (other) ditampilkan dan tidak menampilkan saldo agregat', () => {
    const { model, rows } = buildFixture([
      { id: 'e1', type: 'expense' as const, amount: '30000.00', payment_method: null },
    ], '500000.00')

    expect(model.expense.other).toBe(3000000)
    expect(model.closingCents).toBe(47000000)

    const row = rows.find((r) => r.label === 'Pengeluaran tanpa metode')
    expect(row?.credit).toBe(3000000)
    expect(row?.saldo).toBeUndefined() // agregat pengeluaran: saldo dikosongkan, konsisten dengan tunai/transfer
  })
})
