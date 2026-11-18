const rtPositions = [
  ['ketua', 'Ketua'], ['wakil_ketua', 'Wakil Ketua'], ['sekretaris', 'Sekretaris'], ['bendahara', 'Bendahara'],
  ['keamanan', 'Keamanan'], ['sosial', 'Sosial'], ['operasional', 'Operasional'], ['kebersihan', 'Kebersihan'],
]

const rwPositions = [
  ['ketua', 'Ketua'], ['wakil_ketua', 'Wakil Ketua'], ['sekretaris', 'Sekretaris'], ['bendahara', 'Bendahara'],
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
    default: return 'default'
  }
}
