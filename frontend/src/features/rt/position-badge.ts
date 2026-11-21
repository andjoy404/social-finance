const rtPositions = [
  ['ketua', 'Ketua'], ['wakil_ketua', 'Wakil Ketua'], ['sekretaris', 'Sekretaris'], ['bendahara', 'Bendahara'],
  ['keamanan', 'Keamanan'], ['sosial', 'Sosial'], ['operasional', 'Operasional'], ['kebersihan', 'Kebersihan'],
  ['anggota', 'Anggota RT'], ['anggota_rt', 'Anggota RT'], ['anggota_rw', 'Anggota RW'],
]

const rwPositions = [
  ['ketua', 'Ketua'], ['wakil_ketua', 'Wakil Ketua'], ['sekretaris', 'Sekretaris'], ['bendahara', 'Bendahara'],
  ['anggota', 'Anggota RW'], ['anggota_rw', 'Anggota RW'],
]

export const rtPositionLabel = (value?: string | null) =>
  rtPositions.find(([key]) => key === value)?.[1] ?? (value ? value : 'Belum ada jabatan')

export const rwPositionLabel = (value?: string | null) =>
  rwPositions.find(([key]) => key === value)?.[1] ?? (value ? value : 'Belum ada jabatan')

export type BadgeVariant = 'violet' | 'blue' | 'green' | 'amber' | 'red' | 'gray' | 'teal' | 'chartreuse' | 'pink' | 'default'

export function positionVariant(value?: string | null): BadgeVariant {
  switch (value) {
    case 'ketua': return 'violet'
    case 'wakil_ketua': return 'blue'
    case 'sekretaris': return 'green'
    case 'bendahara': return 'amber'
    case 'keamanan': return 'red'
    case 'sosial': return 'pink'
    case 'operasional': return 'teal'
    case 'kebersihan': return 'chartreuse'
    case 'anggota':
    case 'anggota_rt':
    case 'anggota_rw': return 'blue'
    default: return 'default'
  }
}
