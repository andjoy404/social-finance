import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { RwDashboard } from './RwDashboard'
import * as rwApi from './rw-api'

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 'user-1',
      name: 'Heri Prastyo',
      role: 'bendahara',
      rwMemberships: [{ rw_id: 'rw-002', rw_name: 'RW 016', jabatan: 'bendahara' }],
    },
  }),
}))

vi.mock('@/app/api', () => ({
  getSessionPair: () => ({ accessToken: 'mock-access-token' }),
}))

// Mock Recharts ResponsiveContainer to avoid sizing issues in JSDOM
vi.mock('recharts', async () => {
  const original = await vi.importActual<any>('recharts')
  return {
    ...original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 400, height: 200 }}>{children}</div>
    ),
  }
})

describe('RwDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders Beranda RW summary, 12-month chart, payment methods, and recent transactions', async () => {
    vi.spyOn(rwApi, 'apiGetRWDashboard').mockResolvedValue({
      total_rts: 5,
      total_locations: 3,
      total_opening_balance: '1000000.00',
      total_income: '15000000.00',
      total_expense: '5000000.00',
      net_balance: '11000000.00',
      transaction_count: 12,
    })

    vi.spyOn(rwApi, 'apiGetRWKAS').mockResolvedValue({
      opening_balance: '1000000.00',
      total_income: '2500000.00',
      total_expense: '1000000.00',
      net_balance: '1500000.00',
      transaction_count: 4,
    })

    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [
        {
          id: 'tx-1',
          type: 'income',
          amount: '1500000.00',
          description: 'Iuran RT 01',
          location_id: 'loc-1',
          location_name: 'Rekening BCA RW',
          status: 'posted',
          payment_method: 'TRANSFER',
          occurred_at: '2026-10-10T10:00:00Z',
          created_at: '2026-10-10T10:00:00Z',
        },
        {
          id: 'tx-2',
          type: 'expense',
          amount: '500000.00',
          description: 'Beli ATK',
          location_id: 'loc-2',
          location_name: 'Brankas RW',
          status: 'posted',
          payment_method: 'CASH',
          occurred_at: '2026-10-12T14:00:00Z',
          created_at: '2026-10-12T14:00:00Z',
        },
      ],
      pagination: {
        page: 1,
        per_page: 1000,
        total: 2,
        total_pages: 1,
      },
    })

    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([
      {
        id: 'loc-1',
        rt_id: '',
        rt_name: '',
        name: 'Rekening BCA RW',
        category: 'rekening',
        is_active: true,
        current_balance: '10000000.00',
        created_at: '2026-01-01T00:00:00Z',
      },
      {
        id: 'loc-2',
        rt_id: '',
        rt_name: '',
        name: 'Brankas RW',
        category: 'brankas',
        is_active: true,
        current_balance: '1000000.00',
        created_at: '2026-01-01T00:00:00Z',
      },
    ])

    const onNavigateToKas = vi.fn()

    render(
      <RwDashboard
        rwId="rw-002"
        rwName="RW 016"
        onNavigateToKas={onNavigateToKas}
      />
    )

    // Check Header
    expect(screen.getByText(/Selamat datang, Heri Prastyo/i)).toBeInTheDocument()
    expect(screen.getByText('Keuangan RW')).toBeInTheDocument()

    // Wait for metrics to load
    await waitFor(() => {
      expect(screen.getByText(/Ringkasan RW/i)).toBeInTheDocument()
    })

    // Scope cards: Total RT, Transaksi, Lokasi Dana
    expect(screen.getByText('Total RT')).toBeInTheDocument()
    expect(screen.getByText('Transaksi')).toBeInTheDocument()
    expect(screen.getAllByText('Lokasi Dana').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('5')).toBeInTheDocument() // Total RTs
    expect(screen.getByText('12')).toBeInTheDocument() // Transaction count

    // Financial Chart section
    expect(screen.getByText('Ringkasan Keuangan')).toBeInTheDocument()
    expect(screen.getByText(/12 Bulan/i)).toBeInTheDocument()

    // Payment method breakdown
    expect(screen.getByText(/Metode Transaksi/i)).toBeInTheDocument()
    expect(screen.getAllByText(/Transfer/i).length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/Tunai/i).length).toBeGreaterThanOrEqual(1)

    // Recent transactions table
    expect(screen.getByText('Transaksi Terbaru')).toBeInTheDocument()
    expect(screen.getByText('Iuran RT 01')).toBeInTheDocument()
    expect(screen.getByText('Beli ATK')).toBeInTheDocument()
    expect(screen.getByText('Rekening BCA RW')).toBeInTheDocument()
    expect(screen.getByText('Brankas RW')).toBeInTheDocument()

    // Navigate to KAS button
    const navigateBtn = screen.getByText(/Lihat Semua di Menu KAS/i)
    expect(navigateBtn).toBeInTheDocument()
    fireEvent.click(navigateBtn)
    expect(onNavigateToKas).toHaveBeenCalledTimes(1)
  })
})
