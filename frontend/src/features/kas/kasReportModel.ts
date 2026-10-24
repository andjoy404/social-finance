/**
 * Laporan KAS — satu model agregasi yang menjadi sumber data untuk tampilan
 * Web, PDF dan XLSX. Kontrak data dan rumus yang sama diimplementasikan di
 * Android (`mobile/lib/features/kas/export/kas_report_model.dart`) dan
 * diverifikasi dengan fixture yang sama.
 *
 * ATURAN PERHITUNGAN (sesuai laporan formal `laporan-ipl-urena.jpeg`):
 *  - Debet  = penerimaan (transaksi `income`); Kredit = pengeluaran (`expense`).
 *  - Hanya transaksi berstatus `posted` yang dihitung (konsisten dengan
 *    `GET /reports/balance`). Reversal adalah transaksi `posted` bertipe
 *    berlawanan, sehingga otomatis mengurangi total efektif; transaksi asli
 *    tetap dihitung (ledger immutable).
 *  - Metode pembayaran: `CASH` → cash, `TRANSFER` → transfer, selain itu
 *    (kosong/tidak dikenal) → "tanpa metode", ditampilkan hanya bila nonzero.
 *  - Saldo akhir = saldo awal + total penerimaan - total pengeluaran.
 *  - Saldo awal berasal dari API (tidak bisa dipecah per metode), sehingga
 *    laporan menampilkan *mutasi* kas tunai dan *mutasi* rekening, bukan
 *    "cash on hand"/"saldo rekening" absolut yang butuh saldo awal per metode.
 *  - Tarik tunai: tidak ada model transaksi perpindahan antar-pos di backend,
 *    jadi baris tarik tunai tidak ditampilkan. Jika kelak ada, ia harus
 *    diperlakukan sebagai perpindahan antar-pos, bukan pengeluaran.
 *  - Seluruh perhitungan memakai sen (integer), tanpa floating point.
 */
import type { ApiTransaction } from '@/app/api'

export type PaymentBucket = 'cash' | 'transfer' | 'other'

export interface BucketAmounts {
  cash: number
  transfer: number
  other: number
}

export interface CategoryRow {
  category: string
  cash: number
  transfer: number
  other: number
  total: number
}

export interface KasReportIdentity {
  orgName: string
  rtLabel?: string | null
  rwLabel?: string | null
  address?: string | null
  logoDataUrl?: string | null
}

export interface KasReportInput {
  transactions: ApiTransaction[]
  /** Saldo awal dari API (string NUMERIC). */
  openingBalance: string
  /** Saldo akhir dari API, untuk rekonsiliasi (opsional). */
  apiClosingBalance?: string | null
  /** YYYY-MM atau null untuk semua periode. */
  period: string | null
  identity: KasReportIdentity
  generatedAt?: Date
}

export interface KasReportModel {
  identity: KasReportIdentity
  period: string | null
  periodLabel: string
  generatedAt: Date
  openingCents: number
  income: BucketAmounts
  expense: BucketAmounts
  incomeCategories: { cash: CategoryRow[]; transfer: CategoryRow[]; other: CategoryRow[] }
  expenseCategories: CategoryRow[]
  totalIncomeCents: number
  totalExpenseCents: number
  closingCents: number
  cashMovementCents: number
  transferMovementCents: number
  /** true bila saldo akhir hasil hitung sama dengan saldo akhir API. */
  reconciles: boolean | null
  apiClosingCents: number | null
  transactionCount: number
}

const MONTHS = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

export function periodLabel(period: string | null): string {
  if (!period) return 'Semua Periode'
  const [year, month] = period.split('-')
  const idx = parseInt(month, 10) - 1
  if (!year || Number.isNaN(idx) || idx < 0 || idx > 11) return period
  return `${MONTHS[idx]} ${year}`
}

/** Parse string NUMERIC ("1234.50", "-5") ke sen tanpa floating point. */
export function parseCents(value: string | number | null | undefined): number {
  if (value === null || value === undefined) return 0
  const s = String(value).trim()
  if (s === '') return 0
  const m = /^(-?)(\d+)(?:\.(\d{1,2})\d*)?$/.exec(s)
  if (!m) throw new Error(`Nilai nominal tidak valid: "${s}"`)
  const cents = parseInt(m[2], 10) * 100 + parseInt((m[3] ?? '').padEnd(2, '0') || '0', 10)
  return m[1] === '-' ? -cents : cents
}

export function bucketOf(method: string | null | undefined): PaymentBucket {
  const v = (method ?? '').toString().trim().toUpperCase()
  if (v === 'CASH') return 'cash'
  if (v === 'TRANSFER') return 'transfer'
  return 'other'
}

function addTo(target: BucketAmounts, bucket: PaymentBucket, cents: number) {
  target[bucket] += cents
}

function toRows(map: Map<string, BucketAmounts>): CategoryRow[] {
  return Array.from(map.entries())
    .map(([category, a]) => ({ category, ...a, total: a.cash + a.transfer + a.other }))
    .sort((x, y) => x.category.localeCompare(y.category, 'id'))
}

/**
 * Sort transaksi secara kronologis sebelum agregasi.
 * Urutan memastikan saldo berjalan (running balance) pada baris statement
 * reliabel untuk transaksi yang berasal dari tanggal/waktu berbeda.
 *
 * Tie-breaker deterministik: id (string). Fallback: created_at.
 */
function sortTransactions(txs: ApiTransaction[]): ApiTransaction[] {
  return [...txs].sort((a, b) => {
    const atA = a.occurred_at ?? a.created_at ?? ''
    const atB = b.occurred_at ?? b.created_at ?? ''
    const cmp = atA.localeCompare(atB)
    if (cmp !== 0) return cmp
    return (a.id ?? '').localeCompare(b.id ?? '')
  })
}

export function buildKasReport(input: KasReportInput): KasReportModel {
  // Sort kronologis ascending sebelum agregasi → saldo berjalan reliabel
  const sorted = sortTransactions(input.transactions)

  const income: BucketAmounts = { cash: 0, transfer: 0, other: 0 }
  const expense: BucketAmounts = { cash: 0, transfer: 0, other: 0 }
  const incByCat: Record<PaymentBucket, Map<string, BucketAmounts>> = {
    cash: new Map(), transfer: new Map(), other: new Map(),
  }
  const expByCat = new Map<string, BucketAmounts>()
  let count = 0

  for (const t of sorted) {
    if (t.status !== 'posted') continue
    const cents = parseCents(t.amount)
    const bucket = bucketOf(t.payment_method)
    const cat = (t.category_name ?? '').trim() || 'Lainnya'
    count++
    if (t.type === 'income') {
      addTo(income, bucket, cents)
      const m = incByCat[bucket]
      const cur = m.get(cat) ?? { cash: 0, transfer: 0, other: 0 }
      cur[bucket] += cents
      m.set(cat, cur)
    } else {
      addTo(expense, bucket, cents)
      const cur = expByCat.get(cat) ?? { cash: 0, transfer: 0, other: 0 }
      cur[bucket] += cents
      expByCat.set(cat, cur)
    }
  }

  const sum = (a: BucketAmounts) => a.cash + a.transfer + a.other
  const openingCents = parseCents(input.openingBalance)
  const totalIncomeCents = sum(income)
  const totalExpenseCents = sum(expense)
  const closingCents = openingCents + totalIncomeCents - totalExpenseCents
  const apiClosingCents =
    input.apiClosingBalance === null || input.apiClosingBalance === undefined
      ? null
      : parseCents(input.apiClosingBalance)

  return {
    identity: input.identity,
    period: input.period,
    periodLabel: periodLabel(input.period),
    generatedAt: input.generatedAt ?? new Date(),
    openingCents,
    income,
    expense,
    incomeCategories: {
      cash: toRows(incByCat.cash),
      transfer: toRows(incByCat.transfer),
      other: toRows(incByCat.other),
    },
    expenseCategories: toRows(expByCat),
    totalIncomeCents,
    totalExpenseCents,
    closingCents,
    cashMovementCents: income.cash - expense.cash,
    transferMovementCents: income.transfer - expense.transfer,
    reconciles: apiClosingCents === null ? null : apiClosingCents === closingCents,
    apiClosingCents,
    transactionCount: count,
  }
}

/** Format "Rp1.023.000" (tanpa spasi, sesuai dokumen referensi). */
export function formatRp(cents: number): string {
  const neg = cents < 0
  const abs = Math.abs(cents)
  const whole = Math.floor(abs / 100)
  const frac = abs % 100
  const w = new Intl.NumberFormat('id-ID').format(whole)
  return `${neg ? '-' : ''}Rp${w}${frac ? ',' + String(frac).padStart(2, '0') : ''}`
}

/** Angka tanpa simbol (untuk tabel rekap kategori). */
export function formatNum(cents: number): string {
  if (cents === 0) return ''
  return formatRp(cents).replace('Rp', '')
}

export interface StatementRow {
  no?: number | null
  label: string
  debit?: number | null
  credit?: number | null
  saldo?: number | null
  kind: 'row' | 'subtotal' | 'total' | 'grand' | 'section' | 'note'
}

/**
 * Baris tabel utama (No, Uraian, Debet, Kredit, Saldo) — satu sumber untuk
 * tampilan, PDF dan XLSX.
 *
 * PERHITUNGAN SALDO BERJALAN:
 *   saldo = saldo_sebelumnya + debet - kredit
 * Baris ringkasan (subtotal / total) tidak mengubah saldo berjalan — kolom
 * Saldo tetap dikosongkan agar tidak tertukar dengan nilai mutasi atau total.
 *
 * Perbaikan 2026-10-23: baris agregat (Total penerimaan/pengeluaran Tunai+Transfer)
 * dan baris mutasi (kas tunai, rekening) sekarang menampilkan nilai di kolom
 * Debet/Kredit sehingga tidak lagi kosong di tampilan Web, PDF, dan XLSX.
 */
export function buildStatementRows(m: KasReportModel): StatementRow[] {
  const rows: StatementRow[] = []
  let n = 0
  let runningBalance = 0
  const next = () => ++n

  // Saldo awal
  rows.push({ no: next(), label: 'Saldo awal periode', debit: m.openingCents, saldo: m.openingCents, kind: 'row' })
  runningBalance = m.openingCents

  const addIncomeSection = (title: string, list: CategoryRow[], key: PaymentBucket, subtotalLabel: string) => {
    if (list.length === 0 && m.income[key] === 0) return
    for (const r of list) {
      const val = r[key]
      if (val !== 0) {
        runningBalance += val
        // Kosongkan saldo: baris agregat kategori tidak dapat mewakili
        // titik kronologis yang valid (kategori diurutkan alfabetis,
        // bukan tanggal; satu kategori bisa mencakup transaksi dari banyak tanggal).
        rows.push({ no: next(), label: `${r.category} (${title})`, debit: val, kind: 'row' })
      } else {
        rows.push({ no: next(), label: `${r.category} (${title})`, kind: 'row' })
      }
    }
    // Baris subtotal tidak mengubah saldo berjalan — kolom Saldo dikosongkan
    rows.push({ label: subtotalLabel, kind: 'subtotal' })
  }
  addIncomeSection('tunai', m.incomeCategories.cash, 'cash', 'Total penerimaan tunai')
  addIncomeSection('transfer', m.incomeCategories.transfer, 'transfer', 'Total penerimaan transfer')
  addIncomeSection('tanpa metode', m.incomeCategories.other, 'other', 'Total penerimaan tanpa metode')

  // Baris agregasi Tunai + Transfer — Debet = total penerimaan (cash + transfer)
  rows.push({
    label: 'Total penerimaan Tunai + Transfer',
    debit: m.totalIncomeCents,
    kind: 'total',
  })

  // Baris pengeluaran individual
  const addExpenseRow = (label: string, credit: number) => {
    if (credit === 0) return
    runningBalance -= credit
    // Kosongkan saldo: baris agregat pengeluaran tidak dapat mewakili
    // titik kronologis yang valid (sama dengan penerimaan agregat).
    rows.push({ no: next(), label, credit, kind: 'row' })
  }
  addExpenseRow('Pengeluaran tunai', m.expense.cash)
  addExpenseRow('Pengeluaran transfer', m.expense.transfer)
  if (m.expense.other !== 0) {
    runningBalance -= m.expense.other
    rows.push({ no: next(), label: 'Pengeluaran tanpa metode', credit: m.expense.other, kind: 'row' })
  }

  // Baris agregasi pengeluaran (tidak mengubah saldo berjalan) — Kredit = total pengeluaran (cash + transfer)
  rows.push({
    label: 'Total pengeluaran Tunai + Transfer',
    credit: m.totalExpenseCents,
    kind: 'total',
  })

  // Baris mutasi (tidak mengubah saldo berjalan — bukan transaksi)
  // Mutasi kas tunai: neto penerimaan tunai. Jika positif → Debet, jika negatif → Kredit.
  const cashMovementNeto = m.income.cash - m.expense.cash
  if (cashMovementNeto > 0) {
    rows.push({
      label: 'Mutasi kas tunai (penerimaan tunai - pengeluaran tunai)',
      debit: cashMovementNeto,
      kind: 'subtotal',
    })
  } else if (cashMovementNeto < 0) {
    rows.push({
      label: 'Mutasi kas tunai (penerimaan tunai - pengeluaran tunai)',
      credit: Math.abs(cashMovementNeto),
      kind: 'subtotal',
    })
  } else {
    rows.push({
      label: 'Mutasi kas tunai (penerimaan tunai - pengeluaran tunai)',
      kind: 'subtotal',
    })
  }

  // Mutasi rekening: neto penerimaan transfer. Jika positif → Debet, jika negatif → Kredit.
  const transferMovementNeto = m.income.transfer - m.expense.transfer
  if (transferMovementNeto > 0) {
    rows.push({
      label: 'Mutasi rekening (penerimaan transfer - pengeluaran transfer)',
      debit: transferMovementNeto,
      kind: 'subtotal',
    })
  } else if (transferMovementNeto < 0) {
    rows.push({
      label: 'Mutasi rekening (penerimaan transfer - pengeluaran transfer)',
      credit: Math.abs(transferMovementNeto),
      kind: 'subtotal',
    })
  } else {
    rows.push({
      label: 'Mutasi rekening (penerimaan transfer - pengeluaran transfer)',
      kind: 'subtotal',
    })
  }

  // Saldo akhir — satu-satunya baris yang menampilkan nilai akhir sebenarnya
  rows.push({ label: 'Saldo akhir (saldo awal + penerimaan - pengeluaran)', saldo: runningBalance, kind: 'grand' })
  return rows
}

/** Rekap pengeluaran per kategori + grand total. */
export function buildCategoryRecap(m: KasReportModel) {
  return {
    rows: m.expenseCategories,
    grand: {
      cash: m.expense.cash,
      transfer: m.expense.transfer,
      other: m.expense.other,
      total: m.totalExpenseCents,
    },
    hasOther: m.expense.other !== 0,
  }
}

export function reportFileBase(m: KasReportModel): string {
  return `Laporan-KAS-${m.period ?? 'semua-periode'}`
}
