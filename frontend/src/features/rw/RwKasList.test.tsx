import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { RwKasList } from './RwKasList'
import * as rwApi from './rw-api'
import * as AuthContext from '@/app/AuthContext'

vi.mock('@/app/AuthContext', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/app/AuthContext')>()
  return {
    ...actual,
    useAuth: () => ({
      user: {
        id: 'user-1',
        name: 'Bendahara RW 002',
        role: 'bendahara',
        jabatan: 'bendahara',
        rwMemberships: [{ rw_id: 'rw-002', jabatan: 'bendahara', source: 'explicit' }],
      },
    }),
  }
})

vi.mock('@/app/api', () => ({
  getSessionPair: () => ({ accessToken: 'test-token' }),
  apiListCategories: vi.fn().mockResolvedValue([]),
}))

describe('RwKasList', () => {
  beforeEach(() => {
    vi.spyOn(rwApi, 'apiListRWRTRs').mockResolvedValue([])
  })
  it('renders KAS summary cards and action buttons for authorized officer', async () => {
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue({
      opening_balance: '1000000.00',
      total_income: '5000000.00',
      total_expense: '2000000.00',
      net_balance: '3000000.00',
      total_balance: '4000000.00',
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
      expect(screen.getByText('Penerimaan Iuran RT 01')).toBeInTheDocument()
    })

    // Header actions
    expect(screen.getByText('Catat Kas Masuk')).toBeInTheDocument()
    expect(screen.getByText('Catat Kas Keluar')).toBeInTheDocument()

    // Summary cards
    expect(screen.getByText('Total Pemasukan')).toBeInTheDocument()
    expect(screen.getByText('Total Pengeluaran')).toBeInTheDocument()
    expect(screen.getByText('Saldo Bersih')).toBeInTheDocument()

    // Table should render transaction
    expect(screen.getByText('Rekening BCA Operasional RW')).toBeInTheDocument()
  })

  it('shows read-only UI when membership source is rt_inferred', async () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-rt-inferred',
        name: 'Ketua RT 002',
        role: 'warga',
        jabatan: 'ketua',
        rwMemberships: [
          { rw_id: 'rw-016', rw_name: 'RW 016', jabatan: 'ketua', source: 'rt_inferred' },
        ],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue({
      opening_balance: '1000000.00',
      total_income: '5000000.00',
      total_expense: '2000000.00',
      net_balance: '3000000.00',
      total_balance: '4000000.00',
      transaction_count: 8,
    } as any)

    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    } as any)

    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([])

    render(<RwKasList rwId="rw-016" />)

    await waitFor(() => {
      expect(screen.getByText(/KAS RW — (rw-016|RW 016)/i)).toBeInTheDocument()
    })

    // Header should show read-only subtitle
    expect(screen.getByText('Buku kas RW (Mode Read-Only)')).toBeInTheDocument()

    // Action buttons must NOT appear for inferred user
    expect(screen.queryByText('Catat Kas Masuk')).not.toBeInTheDocument()
    expect(screen.queryByText('Catat Kas Keluar')).not.toBeInTheDocument()
  })

  it('multi-RW: shows read-only for inferred RW even when explicit RW exists', async () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-multi',
        name: 'Ketua RT',
        role: 'warga',
        jabatan: 'ketua',
        rwMemberships: [
          { rw_id: 'rw-016', rw_name: 'RW 016', jabatan: 'ketua', source: 'rt_inferred' },
          { rw_id: 'rw-017', rw_name: 'RW 017', jabatan: 'bendahara', source: 'explicit' },
        ],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue({
      opening_balance: '1000000.00',
      total_income: '5000000.00',
      total_expense: '2000000.00',
      net_balance: '3000000.00',
      total_balance: '4000000.00',
      transaction_count: 8,
    } as any)

    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    } as any)

    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([])

    // Viewing inferred RW → read-only
    render(<RwKasList rwId="rw-016" />)

    await waitFor(() => {
      expect(screen.getByText(/KAS RW — (rw-016|RW 016)/i)).toBeInTheDocument()
    })

    expect(screen.getByText('Buku kas RW (Mode Read-Only)')).toBeInTheDocument()
    expect(screen.queryByText('Catat Kas Masuk')).not.toBeInTheDocument()
    expect(screen.queryByText('Catat Kas Keluar')).not.toBeInTheDocument()
  })

  it('multi-RW: shows full controls for explicit RW even when inferred RW also exists', async () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-multi',
        name: 'Ketua RT',
        role: 'warga',
        jabatan: 'ketua',
        rwMemberships: [
          { rw_id: 'rw-016', rw_name: 'RW 016', jabatan: 'ketua', source: 'rt_inferred' },
          { rw_id: 'rw-017', rw_name: 'RW 017', jabatan: 'bendahara', source: 'explicit' },
        ],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue({
      opening_balance: '1000000.00',
      total_income: '5000000.00',
      total_expense: '2000000.00',
      net_balance: '3000000.00',
      total_balance: '4000000.00',
      transaction_count: 8,
    } as any)

    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    } as any)

    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([])

    // Viewing explicit RW → full controls
    render(<RwKasList rwId="rw-017" />)

    await waitFor(() => {
      expect(screen.getByText(/KAS RW — (rw-017|RW 017)/i)).toBeInTheDocument()
    })

    expect(screen.queryByText('Buku kas RW (Mode Read-Only)')).not.toBeInTheDocument()
    expect(screen.getByText('Catat Kas Masuk')).toBeInTheDocument()
    expect(screen.getByText('Catat Kas Keluar')).toBeInTheDocument()
  })

  it('Anggota RT read-only: does not call apiListRWRTRs or apiListCategories and hides category filter', async () => {
    const listRTRsSpy = vi.spyOn(rwApi, 'apiListRWRTRs').mockResolvedValue([])
    const apiModule = await import('@/app/api')
    const listCategoriesSpy = vi.spyOn(apiModule, 'apiListCategories').mockResolvedValue([])

    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-anggota-rt',
        name: 'Anggota RT 002',
        role: 'pengurus',
        jabatan: 'anggota_rt',
        rwId: 'rw-002',
        rwMemberships: [
          { rw_id: 'rw-002', rw_name: 'RW 002', jabatan: 'anggota_rt', source: 'rt_inferred' },
        ],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue({
      opening_balance: '500000.00',
      total_income: '2000000.00',
      total_expense: '1000000.00',
      net_balance: '1000000.00',
      total_balance: '1500000.00',
      transaction_count: 5,
    } as any)

    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [
        {
          id: 'tx-10',
          rw_id: 'rw-002',
          type: 'income',
          amount: '500000.00',
          description: 'Iuran RT 01',
          status: 'posted',
          occurred_at: '2026-11-01T10:00:00Z',
          created_at: '2026-11-01T10:00:00Z',
        },
      ],
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    } as any)

    render(<RwKasList rwId="rw-002" />)

    await waitFor(() => {
      expect(screen.getByText('Iuran RT 01')).toBeInTheDocument()
    })

    // Assert that unnecessary write endpoints are NOT called in read-only mode
    expect(listRTRsSpy).not.toHaveBeenCalled()
    expect(listCategoriesSpy).not.toHaveBeenCalled()

    // Assert read-only UI: no write buttons, no category filter
    expect(screen.queryByText('Catat Kas Masuk')).not.toBeInTheDocument()
    expect(screen.queryByText('Catat Kas Keluar')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Filter Kategori')).not.toBeInTheDocument()
    expect(screen.getByText('Buku kas RW (Mode Read-Only)')).toBeInTheDocument()
  })
})

