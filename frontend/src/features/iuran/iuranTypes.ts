export type IuranStatus = 'belum_bayar' | 'sebagian' | 'lunas'

// Re-export BILL_DATA from mock data so components can import everything from here
export { BILL_DATA } from './iuranMockData'

export interface IuranPaymentRecord {
  id: string
  nominal: number
  paidDate: string
  catatan?: string
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

export function getIuranTypeLabel(value: string): string {
  return IURAN_TYPE_OPTIONS.find((o) => o.value === value)?.label ?? value
}

export function getStatusVariant(status: IuranStatus): 'red' | 'amber' | 'green' {
  switch (status) {
    case 'belum_bayar':
      return 'red'
    case 'sebagian':
      return 'amber'
    case 'lunas':
      return 'green'
  }
}

export function getStatusLabel(status: IuranStatus): string {
  switch (status) {
    case 'belum_bayar':
      return 'Belum Bayar'
    case 'sebagian':
      return 'Sebagian'
    case 'lunas':
      return 'Lunas'
  }
}

export function formatRupiah(n: number): string {
  return new Intl.NumberFormat('id-ID').format(n)
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
