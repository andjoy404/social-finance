export type IuranStatus = 'belum_bayar' | 'sebagian' | 'lunas' | 'dibatalkan'

// Backend BillStatus → Frontend IuranStatus mapping
export const BACKEND_STATUS_MAP: Record<string, IuranStatus> = {
  unpaid: 'belum_bayar',
  partial: 'sebagian',
  paid: 'lunas',
  cancelled: 'dibatalkan',
}

// Backend status to display variant
export function getStatusVariant(status: IuranStatus): 'red' | 'amber' | 'green' | 'default' {
  switch (status) {
    case 'belum_bayar':
      return 'red'
    case 'sebagian':
      return 'amber'
    case 'lunas':
      return 'green'
    case 'dibatalkan':
      return 'default'
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
  status?: PaymentStatusDisplay
  method?: 'CASH' | 'TRANSFER'
}

export interface IuranBill {
  id: string
  householdId: string
  householdName: string
  rt: string
  iuranType: string
  periode: string
  nominal: number
  paidAmount: number
  status: IuranStatus
  paidDate?: string
  payments?: IuranPaymentRecord[]
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
    default:
      return status
  }
}

export function formatRupiah(n: number): string {
  return new Intl.NumberFormat('id-ID').format(n)
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

export function parseMoney(value: string): number {
  const num = parseInt(value, 10)
  return isNaN(num) ? 0 : num
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
  const [year, month] = periode.split('-')
  const monthNames = [
    'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
    'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
  ]
  const m = parseInt(month, 10)
  return `${monthNames[m - 1]} ${year}`
}
