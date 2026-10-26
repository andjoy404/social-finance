export type IuranStatus = 'belum_bayar' | 'sebagian' | 'lunas' | 'dibatalkan' | 'sukarela'

// Backend BillStatus → Frontend IuranStatus mapping
export const BACKEND_STATUS_MAP: Record<string, IuranStatus> = {
  unpaid: 'belum_bayar',
  partial: 'sebagian',
  paid: 'lunas',
  cancelled: 'dibatalkan',
}

// Backend status to display variant
export function getStatusVariant(status: IuranStatus): 'red' | 'amber' | 'green' | 'default' | 'blue' {
  switch (status) {
    case 'belum_bayar':
      return 'red'
    case 'sebagian':
      return 'amber'
    case 'lunas':
      return 'green'
    case 'dibatalkan':
      return 'default'
    case 'sukarela':
      return 'blue'
    default:
      return 'default'
  }
}

// Re-export BILL_DATA from mock data so components can import everything from here
export { BILL_DATA } from './iuranMockData'

export interface IuranPaymentRecord {
  id: string
  nominal: number
  paidDate: string
  catatan?: string
  rejectionReason?: string
  verifiedBy?: string
  verifiedByName?: string
  officerNotes?: string
  paymentNo?: string
  status?: PaymentStatusDisplay
  method?: 'CASH' | 'TRANSFER'
  submitterName?: string
  submittedBy?: string | null
  proofPath?: string | null
  proofOriginalName?: string | null
  proofMimeType?: string | null
  proofSizeBytes?: number | null
}

export interface IuranBill {
  id: string
  householdId: string
  householdName: string
  houseNumber?: string
  rt: string
  iuranType: string
  periode: string
  nominal: number
  paidAmount: number
  pendingAmount: number
  status: IuranStatus
  paidDate?: string
  payments?: IuranPaymentRecord[]
  isVoluntary?: boolean
  dueId?: string
  dueIsActive?: boolean
}

export type IuranType = 'Keamanan' | 'Kebersihan' | 'Pembangunan'

export const IURAN_TYPE_OPTIONS: { value: IuranType; label: string }[] = [
  { value: 'Keamanan', label: 'Iuran Keamanan' },
  { value: 'Kebersihan', label: 'Iuran Kebersihan' },
  { value: 'Pembangunan', label: 'Iuran Pembangunan' },
]

// Backend DuePeriodType → frontend display
export type DuePeriodType = 'monthly' | 'yearly' | 'one_time'

export const DUE_PERIOD_TYPE_OPTIONS: { value: DuePeriodType; label: string }[] = [
  { value: 'monthly', label: 'Bulanan' },
  { value: 'yearly', label: 'Tahunan' },
  { value: 'one_time', label: 'Sekali Waktu' },
]

// Backend PaymentRule → frontend display
export type PaymentRuleType = 'full' | 'partial'

export const PAYMENT_RULE_OPTIONS: { value: PaymentRuleType; label: string }[] = [
  { value: 'full', label: 'Seluruh' },
  { value: 'partial', label: 'Sebagian' },
]

export function getIuranTypeLabel(value: string): string {
  return IURAN_TYPE_OPTIONS.find((o) => o.value === value)?.label ?? value
}

export function getStatusLabel(status: IuranStatus): string {
  switch (status) {
    case 'belum_bayar':
      return 'Belum Bayar'
    case 'sebagian':
      return 'Sebagian'
    case 'lunas':
      return 'Lunas'
    case 'dibatalkan':
      return 'Dibatalkan'
    case 'sukarela':
      return 'Sukarela'
    default:
      return status
  }
}

export function formatRupiah(n: number): string {
  return `Rp${new Intl.NumberFormat('id-ID').format(n)}`
}

// Payment status display helpers
export type PaymentStatusDisplay = 'PENDING' | 'APPROVED' | 'REJECTED'

export function getPaymentStatusLabel(status: PaymentStatusDisplay): string {
  switch (status) {
    case 'PENDING':
      return 'Menunggu Verifikasi'
    case 'APPROVED':
      return 'Terverifikasi'
    case 'REJECTED':
      return 'Ditolak'
    default:
      return status
  }
}

export function getPaymentStatusVariant(status: PaymentStatusDisplay): 'default' | 'green' | 'red' {
  switch (status) {
    case 'PENDING':
      return 'default'
    case 'APPROVED':
      return 'green'
    case 'REJECTED':
      return 'red'
    default:
      return 'default'
  }
}

export function formatRibuan(value: number | string): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'number') {
    if (isNaN(value)) return ''
    return new Intl.NumberFormat('id-ID').format(Math.floor(Math.abs(value)))
  }
  const trimmed = value.trim()
  if (!trimmed) return ''

  const absStr = trimmed.startsWith('-') ? trimmed.slice(1).trim() : trimmed
  const num = parseMoney(absStr)
  return new Intl.NumberFormat('id-ID').format(num)
}

export type MoneyParseContract = 'auto' | 'api' | 'rupiah'

export interface ParseMoneyOptions {
  format?: MoneyParseContract
}

export function parseMoney(
  value: string | number | null | undefined,
  options?: MoneyParseContract | ParseMoneyOptions
): number {
  if (value === null || value === undefined) return 0

  if (typeof value === 'number') {
    if (isNaN(value) || !isFinite(value) || value < 0) return 0
    return Math.floor(value)
  }

  if (typeof value !== 'string') return 0

  const trimmed = value.trim()
  if (!trimmed) return 0

  // Explicit handling for negative values: return 0 (safe financial rule)
  if (trimmed.startsWith('-') || /-\s*Rp/i.test(trimmed) || /Rp\s*-/i.test(trimmed)) {
    return 0
  }

  // Must contain at least one digit
  if (!/\d/.test(trimmed)) {
    return 0
  }

  const contract: MoneyParseContract =
    typeof options === 'string'
      ? options
      : options?.format || 'auto'

  if (contract === 'api') {
    // API contract: dot (.) is decimal separator (NUMERIC(15,2)).
    // Extract integer part before decimal separator.
    const integerPart = trimmed.split('.')[0].replace(/\D/g, '')
    if (!integerPart) return 0
    const num = parseInt(integerPart, 10)
    return isNaN(num) || num < 0 ? 0 : num
  }

  if (contract === 'rupiah') {
    // Rupiah contract: dot (.) is thousands separator, comma (,) is decimal cents.
    // Drop decimal cents if comma is present, then strip all non-digits (dots, Rp, spaces).
    const beforeComma = trimmed.split(',')[0]
    const digits = beforeComma.replace(/\D/g, '')
    if (!digits) return 0
    const num = parseInt(digits, 10)
    return isNaN(num) || num < 0 ? 0 : num
  }

  // 'auto' contract:
  // 1. Explicit Rupiah indicator (Rp / rp / IDR) or Indonesian comma decimal
  const hasCurrencyPrefix = /^(?:Rp|IDR)\b/i.test(trimmed) || /Rp/i.test(trimmed)
  const hasDecimalComma = /,\d{1,2}$/.test(trimmed)

  if (hasCurrencyPrefix || hasDecimalComma) {
    const beforeComma = trimmed.split(',')[0]
    const digits = beforeComma.replace(/\D/g, '')
    if (!digits) return 0
    const num = parseInt(digits, 10)
    return isNaN(num) || num < 0 ? 0 : num
  }

  // 2. Multiple dots: definitely thousands separators (e.g. "1.500.000", "10.000.000")
  const dotCount = (trimmed.match(/\./g) || []).length
  if (dotCount > 1) {
    const digits = trimmed.replace(/\D/g, '')
    if (!digits) return 0
    const num = parseInt(digits, 10)
    return isNaN(num) || num < 0 ? 0 : num
  }

  // 3. Exactly one dot:
  if (dotCount === 1) {
    const [partBefore, partAfter] = trimmed.split('.')
    // If partAfter has 3 digits and partBefore has 1-3 digits (e.g. "50.000", "1.000", "100.000"):
    // In Indonesian currency context (and matching formatRibuan output), this is thousands separator.
    if (/^\d{1,3}$/.test(partBefore) && /^\d{3}$/.test(partAfter)) {
      const digits = (partBefore + partAfter).replace(/\D/g, '')
      const num = parseInt(digits, 10)
      return isNaN(num) || num < 0 ? 0 : num
    }

    // Otherwise (e.g. "50000.00", "25000.00", "100000.00", "50.00", "123.45"):
    // It is standard decimal format from PostgreSQL NUMERIC(15, 2) / API. Take partBefore!
    const digits = partBefore.replace(/\D/g, '')
    if (!digits) return 0
    const num = parseInt(digits, 10)
    return isNaN(num) || num < 0 ? 0 : num
  }

  // 4. No dots (e.g. "50000", "1000000", "0"):
  const digits = trimmed.replace(/\D/g, '')
  if (!digits) return 0
  const num = parseInt(digits, 10)
  return isNaN(num) || num < 0 ? 0 : num
}


export function formatPeriodeLabel(periode: string): string {
  const [year, month] = periode.split('-')
  const monthNames = [
    'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun',
    'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des',
  ]
  const m = parseInt(month, 10)
  return `${monthNames[m - 1]} ${year}`
}

export function formatFullPeriodeLabel(periode: string): string {
  // Guard for non-period values (e.g. voluntary Rp0 donations with periode '—')
  if (!periode || periode === '—' || !periode.includes('-')) {
    return periode ?? ''
  }
  const [year, month] = periode.split('-')
  const monthNames = [
    'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
    'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
  ]
  const m = parseInt(month, 10)
  return `${monthNames[m - 1]} ${year}`
}
