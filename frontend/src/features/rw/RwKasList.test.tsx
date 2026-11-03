import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { RwKasList } from './RwKasList'
import * as rwApi from './rw-api'

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 'user-1',
      name: 'Bendahara RW 002',
      role: 'bendahara',
      jabatan: 'bendahara',
      rw_memberships: [{ rw_id: 'rw-002', jabatan: 'bendahara' }],
    },
  }),
}))

vi.mock('@/app/api', () => ({
  getSessionPair: () => ({ accessToken: 'test-token' }),
}))

describe('RwKasList', () => {
  it('renders KAS summary cards and action buttons for authorized officer', async () => {
    vi.spyOn(rwApi, 'apiGetRWKAS').mockResolvedValue({
      opening_balance: '1000000.00',
      total_income: '5000000.00',
      total_expense: '2000000.00',
      net_balance: '3000000.00',
      transaction_count: 8,
    } as any)

    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [
        {
          id: 'tx-1',
          rw_id: 'rw-002',
          rt_id: 'rt-1',
          rt_name: 'RT 001',
          type: 'income',
          amount: '1000000.00',
          description: 'Penerimaan Iuran RT 01',
          location_name: 'Rekening BCA Operasional RW',
          status: 'posted',
          occurred_at: '2026-10-15T10:00:00Z',
          created_at: '2026-10-15T10:00:00Z',
        },
      ],
      pagination: {
        page: 1,
        per_page: 20,
        total: 1,
        total_pages: 1,
      },
    } as any)

    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([])

    render(<RwKasList rwId="rw-002" />)

    await waitFor(() => {
      expect(screen.getByText('KAS RW — rw-002')).toBeInTheDocument()
    })

    // Header actions
    expect(screen.getByText('Catat Kas Masuk')).toBeInTheDocument()
    expect(screen.getByText('Catat Kas Keluar')).toBeInTheDocument()

    // Summary cards
    expect(screen.getByText('Total Pemasukan')).toBeInTheDocument()
    expect(screen.getByText('Total Pengeluaran')).toBeInTheDocument()
    expect(screen.getByText('Saldo Bersih')).toBeInTheDocument()

    // Table should render transaction
    expect(screen.getByText('Penerimaan Iuran RT 01')).toBeInTheDocument()
    expect(screen.getByText('Rekening BCA Operasional RW')).toBeInTheDocument()
  })
})

