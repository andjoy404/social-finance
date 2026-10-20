export function formatRupiah(n: number): string {
  return `Rp${new Intl.NumberFormat('id-ID').format(n)}`
}

export function formatPhoneDisplay(raw?: string | null): string {
  if (!raw) return '—'
  const clean = raw.trim().replace(/[\s-]/g, '')
  if (!clean || clean === '—' || clean === '-') return '—'
  if (clean.startsWith('+62')) {
    return '0' + clean.slice(3)
  }
  if (clean.startsWith('62')) {
    return '0' + clean.slice(2)
  }
  return clean
}

