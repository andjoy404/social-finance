import type { ApiTransaction } from '@/app/api'

/**
 * Fixture bersama Web & Android (lihat mobile/test/.../kas_report_model_test.dart).
 * Expected values dihitung manual dari aturan bisnis:
 *   Penerimaan cash     = 1.000.000 + 250.000            = 1.250.000
 *   Penerimaan transfer = 500.000                        =   500.000
 *   Penerimaan tanpa metode = 0
 *   Total penerimaan                                      = 1.750.000
 *   Pengeluaran cash     = 300.000 + 100.000              =   400.000
 *   Pengeluaran transfer = 150.000                        =   150.000
 *   Total pengeluaran                                     =   550.000
 *   Saldo awal 2.000.000 → saldo akhir 2.000.000+1.750.000-550.000 = 3.200.000
 *   Mutasi kas tunai   = 1.250.000-400.000 = 850.000
 *   Mutasi rekening    =   500.000-150.000 = 350.000
 * Transaksi draft/cancelled TIDAK dihitung.
 */
export function tx(p: Partial<ApiTransaction> & Pick<ApiTransaction, 'id' | 'type' | 'amount'>): ApiTransaction {
  return {
    rt_id: 'rt-1', category_id: 'c', status: 'posted', description: p.id,
    occurred_at: '2026-10-05T03:00:00Z', created_at: '2026-10-05T03:00:00Z', updated_at: '2026-10-05T03:00:00Z',
    category_name: 'Lainnya', payment_method: 'CASH', ...p,
  } as ApiTransaction
}

export const FIXTURE: ApiTransaction[] = [
  tx({ id: 'i1', type: 'income', amount: '1000000.00', category_name: 'Iuran Warga', payment_method: 'CASH' }),
  tx({ id: 'i2', type: 'income', amount: '250000.00', category_name: 'Infaq', payment_method: 'CASH' }),
  tx({ id: 'i3', type: 'income', amount: '500000.00', category_name: 'Iuran Warga', payment_method: 'TRANSFER' }),
  tx({ id: 'e1', type: 'expense', amount: '300000.00', category_name: 'Keamanan', payment_method: 'CASH' }),
  tx({ id: 'e2', type: 'expense', amount: '100000.00', category_name: 'Kebersihan', payment_method: 'CASH' }),
  tx({ id: 'e3', type: 'expense', amount: '150000.00', category_name: 'Keamanan', payment_method: 'TRANSFER' }),
  tx({ id: 'd1', type: 'income', amount: '999999.00', status: 'draft' }),
  tx({ id: 'c1', type: 'expense', amount: '888888.00', status: 'cancelled' }),
]

