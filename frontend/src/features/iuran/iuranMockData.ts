import type { IuranBill, IuranStatus } from './iuranTypes'

const HOUSEHOLDS = [
  { householdId: 'h001', name: 'Budi Santoso', rt: 'RT 03' },
  { householdId: 'h002', name: 'Siti Rahayu', rt: 'RT 03' },
  { householdId: 'h003', name: 'Agus Prasetyo', rt: 'RT 04' },
  { householdId: 'h004', name: 'Dewi Lestari', rt: 'RT 04' },
  { householdId: 'h005', name: 'Rudi Hermawan', rt: 'RT 05' },
  { householdId: 'h006', name: 'Ningsih Wulandari', rt: 'RT 05' },
  { householdId: 'h007', name: 'Hendra Gunawan', rt: 'RT 03' },
  { householdId: 'h008', name: 'Fitri Handayani', rt: 'RT 04' },
  { householdId: 'h009', name: 'Dedi Kurniawan', rt: 'RT 05' },
  { householdId: 'h010', name: 'Lina Marlina', rt: 'RT 03' },
]

const TYPES = ['Keamanan', 'Kebersihan', 'Pembangunan'] as const

const PERIODS = ['2026-05', '2026-06', '2026-07', '2026-08', '2026-09', '2026-10']

const NOMINALS: Record<string, number> = {
  Keamanan: 50000,
  Kebersihan: 30000,
  Pembangunan: 100000,
}

function makePayments(count: number, totalPaid: number): { id: string; nominal: number; paidDate: string; catatan?: string }[] {
  if (count === 0) return []
  if (count === 1) return [{ id: `pay-${totalPaid}`, nominal: totalPaid, paidDate: '2026-09-15' }]
  const parts: { id: string; nominal: number; paidDate: string; catatan?: string }[] = []
  let remaining = totalPaid
  for (let i = 0; i < count; i++) {
    if (i === count - 1) {
      parts.push({ id: `pay-${i}`, nominal: remaining, paidDate: '2026-09-15' })
    } else {
      const p = Math.floor(totalPaid / count)
      parts.push({ id: `pay-${i}`, nominal: p, paidDate: `2026-09-${String(10 + i).slice(-2)}`, catatan: i === 0 ? 'Transfer' : undefined })
      remaining -= p
    }
  }
  return parts
}

function makeBill(hhIndex: number, iuranType: string, periode: string, status: IuranStatus): IuranBill {
  const hh = HOUSEHOLDS[hhIndex]
  const nominal = NOMINALS[iuranType]
  const paidAmount = status === 'lunas'
    ? nominal
    : status === 'sebagian'
      ? Math.floor(nominal * 0.6)
      : 0

  return {
    id: `bill-${hh.householdId}-${iuranType}-${periode}`,
    householdId: hh.householdId,
    householdName: hh.name,
    rt: hh.rt,
    iuranType,
    periode,
    nominal,
    paidAmount,
    status,
    paidDate: status === 'lunas' ? '2026-09-15' : undefined,
    payments: makePayments(status === 'lunas' ? 1 : status === 'sebagian' ? 2 : 0, paidAmount),
  }
}

// Build ~20 bills with ~40% belum_bayar, ~30% sebagian, ~30% lunas
const BILL_DATA: IuranBill[] = [
  // Belum bayar (~40% = ~8 bills)
  makeBill(0, 'Keamanan', '2026-10', 'belum_bayar'),
  makeBill(1, 'Keamanan', '2026-10', 'belum_bayar'),
  makeBill(2, 'Kebersihan', '2026-10', 'belum_bayar'),
  makeBill(3, 'Pembangunan', '2026-10', 'belum_bayar'),
  makeBill(4, 'Keamanan', '2026-09', 'belum_bayar'),
  makeBill(5, 'Kebersihan', '2026-09', 'belum_bayar'),
  makeBill(6, 'Pembangunan', '2026-09', 'belum_bayar'),
  makeBill(7, 'Keamanan', '2026-10', 'belum_bayar'),

  // Sebagian (~30% = ~6 bills)
  makeBill(0, 'Kebersihan', '2026-10', 'sebagian'),
  makeBill(1, 'Pembangunan', '2026-09', 'sebagian'),
  makeBill(3, 'Keamanan', '2026-09', 'sebagian'),
  makeBill(4, 'Kebersihan', '2026-10', 'sebagian'),
  makeBill(6, 'Keamanan', '2026-08', 'sebagian'),
  makeBill(8, 'Kebersihan', '2026-10', 'sebagian'),

  // Lunas (~30% = ~6 bills)
  makeBill(0, 'Kebersihan', '2026-09', 'lunas'),
  makeBill(2, 'Keamanan', '2026-10', 'lunas'),
  makeBill(4, 'Pembangunan', '2026-09', 'lunas'),
  makeBill(5, 'Keamanan', '2026-10', 'lunas'),
  makeBill(7, 'Kebersihan', '2026-09', 'lunas'),
  makeBill(9, 'Pembangunan', '2026-09', 'lunas'),
]

export { BILL_DATA }
