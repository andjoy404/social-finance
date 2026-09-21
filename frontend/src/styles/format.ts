export function formatRupiah(n: number): string {
  return new Intl.NumberFormat('id-ID').format(n)
}
