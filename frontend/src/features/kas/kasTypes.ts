export type KasJenis = 'masuk' | 'keluar'

export interface KasTransaction {
  id: string
  tanggal: string
  jenis: KasJenis
  kategori: string
  keterangan: string
  referensi?: string
  nominal: number
  saldo: number
}

export type KategoriKas =
  | 'Iuran Warga'
  | 'Iuran Paksa'
  | 'Perbaikan Fasilitas'
  | 'Kebersihan'
  | 'Keamanan'
  | 'Konsumsi Rapat'
  | 'Perlengkapan'
  | 'Lainnya'

export const KAS_KATEGORI_OPTIONS: { value: KategoriKas; label: string }[] = [
  { value: 'Iuran Warga', label: 'Iuran Warga' },
  { value: 'Iuran Paksa', label: 'Iuran Paksa' },
  { value: 'Perbaikan Fasilitas', label: 'Perbaikan Fasilitas' },
  { value: 'Kebersihan', label: 'Kebersihan' },
  { value: 'Keamanan', label: 'Keamanan' },
  { value: 'Konsumsi Rapat', label: 'Konsumsi Rapat' },
  { value: 'Perlengkapan', label: 'Perlengkapan' },
  { value: 'Lainnya', label: 'Lainnya' },
]

export function formatRupiah(n: number): string {
  return new Intl.NumberFormat('id-ID').format(n)
}

export function formatTanggal(dateStr: string): string {
  if (!dateStr) return '\u2014'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  const months = [
    'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun',
    'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des',
  ]
  return `${d.getDate()} ${months[d.getMonth()]} ${d.getFullYear()}`
}

export function getJenisVariant(jenis: KasJenis): 'green' | 'red' {
  return jenis === 'masuk' ? 'green' : 'red'
}

export function getJenisLabel(jenis: KasJenis): string {
  return jenis === 'masuk' ? 'Kas Masuk' : 'Kas Keluar'
}

export function formatKeteranganKas(nominal: number, jenis: KasJenis): string {
  const formatted = formatRupiah(nominal)
  return jenis === 'masuk' ? `+${formatted}` : `-${formatted}`
}
