export type KasJenis = 'masuk' | 'keluar'

export interface KasTransaction {
  id: string
  tanggal: string
  jenis: KasJenis
  kategori: string
  keterangan: string
  referensi?: string
  nominal: string
  saldo: string
}

export type KategoriKas =
  | 'Operasional RT'
  | 'Iuran Warga'
  | 'Iuran Paksa'
  | 'Perbaikan Fasilitas'
  | 'Kebersihan'
  | 'Keamanan'
  | 'Konsumsi Rapat'
  | 'Perlengkapan'
  | 'Lainnya'

export const KAS_KATEGORI_OPTIONS: { value: KategoriKas; label: string }[] = [
  { value: 'Operasional RT', label: 'Operasional RT' },
  { value: 'Iuran Warga', label: 'Iuran Warga' },
  { value: 'Iuran Paksa', label: 'Iuran Paksa' },
  { value: 'Perbaikan Fasilitas', label: 'Perbaikan Fasilitas' },
  { value: 'Kebersihan', label: 'Kebersihan' },
  { value: 'Keamanan', label: 'Keamanan' },
  { value: 'Konsumsi Rapat', label: 'Konsumsi Rapat' },
  { value: 'Perlengkapan', label: 'Perlengkapan' },
  { value: 'Lainnya', label: 'Lainnya' },
]

export function formatRupiah(n: string | number): string {
  const num = Number(n)
  if (isNaN(num)) return String(n)
  return new Intl.NumberFormat('id-ID').format(num)
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

export function formatKeteranganKas(nominal: string, jenis: KasJenis): string {
  const formatted = formatRupiah(nominal)
  return jenis === 'masuk' ? `+${formatted}` : `-${formatted}`
}
