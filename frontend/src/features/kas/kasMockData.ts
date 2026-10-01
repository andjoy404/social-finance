import type { KasTransaction, KasJenis, KategoriKas } from './kasTypes'

const KATEGORI_MASUK: KategoriKas[] = ['Iuran Warga', 'Iuran Paksa']
const KATEGORI_KELUAR: KategoriKas[] = ['Perbaikan Fasilitas', 'Kebersihan', 'Keamanan', 'Konsumsi Rapat', 'Perlengkapan', 'Lainnya']

const REFERENSI_MASUK = [
  'Pembayaran Iuran Okt 2026 - KK Budi Santoso',
  'Pembayaran Iuran Sep 2026 - KK Siti Rahayu',
  'Iuran Paksa Agustus 2026 - KK Agus Prasetyo',
  'Pembayaran Iuran Agu 2026 - KK Dewi Lestari',
  'Pembayaran Iuran Jul 2026 - KK Hendra Gunawan',
  'Iuran Paksa Juli 2026 - KK Lina Marlina',
  'Pembayaran Iuran Jun 2026 - KK Rudi Hermawan',
  'Pembayaran Iuran Mei 2026 - KK Ningsih Wulandari',
  'Iuran Warga Mei 2026 - KK Fitri Handayani',
  'Pembayaran Iuran Apr 2026 - KK Dedi Kurniawan',
]

const REFERENSI_KELUAR = [
  'Pembelian cat dan kuas untuk pagar RT',
  'Konsumsi rapat koordinasi bulan Oktober',
  'Pembayaran petugas kebersihan bulan Oktober',
  'Penggantian lampu taman RT 03',
  'Pembelian sapu dan peralatan kebersihan',
  'Konsumsi rapat rutin bulanan September',
  'Perbaikan gapura masuk RT 03',
  'Pembayaran iuran keamanan bulan September',
  'Perbaikan selokan gang belakang',
  'Konsumsi arisan bulan Agustus',
  'Pembelian sampah plastik untuk sampah RT',
  'Perbaikan pos jaga RT 03',
  'Pembayaran honor petugas kebersihan Agustus',
  'Konsumsi rapat RT Agustus',
  'Penggantian kunci pos jaga',
  'Pembelian ember dan kain pel',
  'Konsumsi rapat koordinasi Juli',
  'Perbaikan jalan gang dalam',
  'Pembayaran iuran keamanan Juli',
  'Pembelian paku dan lem untuk papan informasi',
  'Konsumsi arisan bulan Juli',
  'Perbaikan tiang bendera RT',
  'Pembayaran petugas kebersihan Juli',
  'Pembelian tisu dan sabun untuk WC umum',
]

function generateSaldo(transactions: KasTransaction[]): KasTransaction[] {
  let saldo = 5000000
  const result: KasTransaction[] = []
  for (const t of transactions) {
    if (t.jenis === 'masuk') {
      saldo += t.nominal
    } else {
      saldo -= t.nominal
    }
    result.push({ ...t, saldo })
  }
  return result
}

const baseTransactions: Omit<KasTransaction, 'saldo'>[] = [
  // Oktober 2026
  { id: 'kas-001', tanggal: '2026-10-01', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Oktober', referensi: REFERENSI_MASUK[0], nominal: 500000 },
  { id: 'kas-002', tanggal: '2026-10-03', jenis: 'keluar', kategori: 'Kebersihan', keterangan: 'Pembayaran petugas kebersihan bulan Oktober', referensi: REFERENSI_KELUAR[2], nominal: 300000 },
  { id: 'kas-003', tanggal: '2026-10-05', jenis: 'keluar', kategori: 'Konsumsi Rapat', keterangan: 'Konsumsi rapat koordinasi bulan Oktober', referensi: REFERENSI_KELUAR[1], nominal: 150000 },
  { id: 'kas-004', tanggal: '2026-10-08', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Pembayaran iuran tambahan - KK Siti Rahayu', referensi: 'Pembayaran Iuran Sep 2026 - KK Siti Rahayu', nominal: 500000 },
  { id: 'kas-005', tanggal: '2026-10-10', jenis: 'keluar', kategori: 'Perbaikan Fasilitas', keterangan: 'Penggantian lampu taman RT 03', referensi: REFERENSI_KELUAR[3], nominal: 200000 },
  { id: 'kas-006', tanggal: '2026-10-12', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Oktober', referensi: 'Pembayaran Iuran Okt 2026 - KK Hendra Gunawan', nominal: 500000 },
  { id: 'kas-007', tanggal: '2026-10-15', jenis: 'keluar', kategori: 'Perlengkapan', keterangan: 'Pembelian sapu dan peralatan kebersihan', referensi: REFERENSI_KELUAR[4], nominal: 75000 },
  { id: 'kas-008', tanggal: '2026-10-18', jenis: 'masuk', kategori: 'Iuran Paksa', keterangan: 'Iuran paksa - KK Agus Prasetyo', referensi: REFERENSI_MASUK[2], nominal: 1000000 },

  // September 2026
  { id: 'kas-009', tanggal: '2026-09-02', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan September', referensi: 'Pembayaran Iuran Sep 2026 - KK Dewi Lestari', nominal: 500000 },
  { id: 'kas-010', tanggal: '2026-09-05', jenis: 'keluar', kategori: 'Konsumsi Rapat', keterangan: 'Konsumsi rapat rutin bulanan September', referensi: REFERENSI_KELUAR[5], nominal: 150000 },
  { id: 'kas-011', tanggal: '2026-09-08', jenis: 'keluar', kategori: 'Perbaikan Fasilitas', keterangan: 'Perbaikan gapura masuk RT 03', referensi: REFERENSI_KELUAR[6], nominal: 500000 },
  { id: 'kas-012', tanggal: '2026-09-10', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan September', referensi: 'Pembayaran Iuran Sep 2026 - KK Budi Santoso', nominal: 500000 },
  { id: 'kas-013', tanggal: '2026-09-12', jenis: 'keluar', kategori: 'Keamanan', keterangan: 'Pembayaran iuran keamanan bulan September', referensi: REFERENSI_KELUAR[7], nominal: 350000 },
  { id: 'kas-014', tanggal: '2026-09-15', jenis: 'keluar', kategori: 'Perbaikan Fasilitas', keterangan: 'Perbaikan selokan gang belakang', referensi: REFERENSI_KELUAR[8], nominal: 400000 },
  { id: 'kas-015', tanggal: '2026-09-20', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan September', referensi: 'Pembayaran Iuran Sep 2026 - KK Lina Marlina', nominal: 500000 },

  // Agustus 2026
  { id: 'kas-016', tanggal: '2026-08-01', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Agustus', referensi: 'Pembayaran Iuran Agu 2026 - KK Rudi Hermawan', nominal: 500000 },
  { id: 'kas-017', tanggal: '2026-08-05', jenis: 'keluar', kategori: 'Perbaikan Fasilitas', keterangan: 'Pembelian cat dan kuas untuk pagar RT', referensi: REFERENSI_KELUAR[0], nominal: 250000 },
  { id: 'kas-018', tanggal: '2026-08-08', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Agustus', referensi: REFERENSI_MASUK[3], nominal: 500000 },
  { id: 'kas-019', tanggal: '2026-08-10', jenis: 'keluar', kategori: 'Konsumsi Rapat', keterangan: 'Konsumsi arisan bulan Agustus', referensi: REFERENSI_KELUAR[9], nominal: 150000 },
  { id: 'kas-020', tanggal: '2026-08-12', jenis: 'keluar', kategori: 'Kebersihan', keterangan: 'Pembayaran honor petugas kebersihan Agustus', referensi: REFERENSI_KELUAR[12], nominal: 300000 },
  { id: 'kas-021', tanggal: '2026-08-15', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Agustus', referensi: 'Pembayaran Iuran Agu 2026 - KK Ningsih Wulandari', nominal: 500000 },
  { id: 'kas-022', tanggal: '2026-08-18', jenis: 'keluar', kategori: 'Perlengkapan', keterangan: 'Perbaikan pos jaga RT 03', referensi: REFERENSI_KELUAR[11], nominal: 350000 },
  { id: 'kas-023', tanggal: '2026-08-20', jenis: 'masuk', kategori: 'Iuran Paksa', keterangan: 'Iuran warga Mei 2026 - KK Fitri Handayani', referensi: REFERENSI_MASUK[8], nominal: 500000 },
  { id: 'kas-024', tanggal: '2026-08-25', jenis: 'keluar', kategori: 'Keamanan', keterangan: 'Penggantian kunci pos jaga', referensi: REFERENSI_KELUAR[14], nominal: 100000 },

  // Juli 2026
  { id: 'kas-025', tanggal: '2026-07-02', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Juli', referensi: 'Pembayaran Iuran Jul 2026 - KK Dedi Kurniawan', nominal: 500000 },
  { id: 'kas-026', tanggal: '2026-07-05', jenis: 'keluar', kategori: 'Kebersihan', keterangan: 'Pembayaran petugas kebersihan bulan Juli', referensi: REFERENSI_KELUAR[22], nominal: 300000 },
  { id: 'kas-027', tanggal: '2026-07-08', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Juli', referensi: 'Iuran Warga Mei 2026 - KK Budi Santoso', nominal: 500000 },
  { id: 'kas-028', tanggal: '2026-07-10', jenis: 'keluar', kategori: 'Konsumsi Rapat', keterangan: 'Konsumsi rapat koordinasi Juli', referensi: REFERENSI_KELUAR[16], nominal: 150000 },
  { id: 'kas-029', tanggal: '2026-07-12', jenis: 'keluar', kategori: 'Perbaikan Fasilitas', keterangan: 'Perbaikan jalan gang dalam', referensi: REFERENSI_KELUAR[17], nominal: 600000 },
  { id: 'kas-030', tanggal: '2026-07-15', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Juli', referensi: 'Pembayaran Iuran Jul 2026 - KK Siti Rahayu', nominal: 500000 },
  { id: 'kas-031', tanggal: '2026-07-18', jenis: 'keluar', kategori: 'Keamanan', keterangan: 'Pembayaran iuran keamanan Juli', referensi: REFERENSI_KELUAR[18], nominal: 350000 },
  { id: 'kas-032', tanggal: '2026-07-20', jenis: 'keluar', kategori: 'Perlengkapan', keterangan: 'Pembelian paku dan lem untuk papan informasi', referensi: REFERENSI_KELUAR[19], nominal: 50000 },
  { id: 'kas-033', tanggal: '2026-07-22', jenis: 'masuk', kategori: 'Iuran Warga', keterangan: 'Iuran warga bulan Juli', referensi: 'Iuran Warga Mei 2026 - KK Agus Prasetyo', nominal: 500000 },
  { id: 'kas-034', tanggal: '2026-07-25', jenis: 'keluar', kategori: 'Konsumsi Rapat', keterangan: 'Konsumsi arisan bulan Juli', referensi: REFERENSI_KELUAR[20], nominal: 150000 },
  { id: 'kas-035', tanggal: '2026-07-28', jenis: 'keluar', kategori: 'Perbaikan Fasilitas', keterangan: 'Perbaikan tiang bendera RT', referensi: REFERENSI_KELUAR[21], nominal: 200000 },
  { id: 'kas-036', tanggal: '2026-07-30', jenis: 'keluar', kategori: 'Kebersihan', keterangan: 'Pembayaran petugas kebersihan Juli', referensi: REFERENSI_KELUAR[22], nominal: 300000 },
  { id: 'kas-037', tanggal: '2026-07-30', jenis: 'keluar', kategori: 'Perlengkapan', keterangan: 'Pembelian tisu dan sabun untuk WC umum', referensi: REFERENSI_KELUAR[23], nominal: 80000 },
]

// Filter to 25 transactions, sort by date descending, compute running balance
const allTransactions = baseTransactions.slice(0, 25)
allTransactions.sort((a, b) => new Date(b.tanggal).getTime() - new Date(a.tanggal).getTime())

export const KAS_TRANSACTIONS: KasTransaction[] = generateSaldo(allTransactions)

export function getTotalKasMasuk(): number {
  return KAS_TRANSACTIONS.filter((t) => t.jenis === 'masuk').reduce((sum, t) => sum + t.nominal, 0)
}

export function getTotalKasKeluar(): number {
  return KAS_TRANSACTIONS.filter((t) => t.jenis === 'keluar').reduce((sum, t) => sum + t.nominal, 0)
}

export function getFilteredTransactions(
  search: string,
  jenisFilter: string | null,
  kategoriFilter: string | null,
  periodeFilter: string | null,
  transactions: KasTransaction[]
): KasTransaction[] {
  return transactions.filter((t) => {
    if (jenisFilter && t.jenis !== jenisFilter) return false
    if (kategoriFilter && t.kategori !== kategoriFilter) return false
    if (periodeFilter) {
      const [year, month] = periodeFilter.split('-')
      const tMonth = t.tanggal.substring(0, 7)
      if (tMonth !== periodeFilter) return false
    }
    if (search) {
      const s = search.toLowerCase()
      if (
        !t.keterangan.toLowerCase().includes(s) &&
        !t.kategori.toLowerCase().includes(s) &&
        !t.referensi?.toLowerCase().includes(s)
      ) return false
    }
    return true
  })
}

export function getFilteredCategories(transactions: KasTransaction[], jenisFilter: string | null, periodeFilter: string | null): string[] {
  const filtered = transactions.filter((t) => {
    if (jenisFilter && t.jenis !== jenisFilter) return false
    if (periodeFilter && t.tanggal.substring(0, 7) !== periodeFilter) return false
    return true
  })
  const set = new Set<string>()
  for (const t of filtered) set.add(t.kategori)
  return Array.from(set).sort()
}
