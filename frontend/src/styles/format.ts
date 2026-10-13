export function formatRupiah(n: number): string {
  return `Rp${new Intl.NumberFormat('id-ID').format(n)}`
}
