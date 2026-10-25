import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { FundLocationList } from './FundLocationList'
import * as AuthContext from '@/app/AuthContext'
import * as api from '@/app/api'

vi.mock('@/app/AuthContext', () => ({
  useAuth: vi.fn(),
}))

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof api>('@/app/api')
  return {
    ...actual,
    getSessionPair: vi.fn(() => ({ accessToken: 'test-token', refreshToken: 'test-ref' })),
    apiListFundLocations: vi.fn(),
    apiGetFundLocationReconciliation: vi.fn(),
    apiListFundTransfers: vi.fn(),
    apiCreateFundLocation: vi.fn(),
    apiCreateFundTransfer: vi.fn(),
    apiSetOpeningBalance: vi.fn(),
  }
})

describe('FundLocationList component', () => {
  const mockLocations: api.FundLocation[] = [
    {
      id: 'loc-1',
      rt_id: 'rt-1',
      name: 'Rekening BCA Operasional',
      category: 'rekening',
      bank_name: 'BCA',
      account_number: '1234567890',
      account_holder: 'RT 001',
      is_active: true,
      current_balance: '1500000.00',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
    },
    {
      id: 'loc-2',
      rt_id: 'rt-1',
      name: 'Brankas Utama',
      category: 'brankas',
      bank_name: null,
      account_number: null,
      account_holder: 'Bendahara',
      is_active: true,
      current_balance: '500000.00',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
    },
    {
      id: 'loc-3',
      rt_id: 'rt-1',
      name: 'COH Ketua RT',
      category: 'coh',
      bank_name: null,
      account_number: null,
      account_holder: 'Pak Ketua',
      is_active: true,
      current_balance: '250000.00',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
    },
  ]

  const mockReconciliation: api.ReconciliationReport = {
    kas_balance: '2500000.00',
    total_allocated_balance: '2250000.00',
    unallocated_balance: '250000.00',
    discrepancy: '0.00',
    status: 'MATCH_WITH_NOTES',
    status_label: 'Cocok dengan Catatan',
    locations_summary: [],
    anomalies_detected: [],
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(AuthContext.useAuth).mockReturnValue({
      user: {
        id: 'u-1',
        name: 'Bendahara Test',
        email: 'bendahara@test.com',
        systemRole: null,
        role: 'bendahara',
        jabatan: 'bendahara',
        rt: { id: 'rt-1', name: 'RT 001' },
      },
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
    })
    vi.mocked(api.apiListFundLocations).mockResolvedValue(mockLocations)
    vi.mocked(api.apiGetFundLocationReconciliation).mockResolvedValue(mockReconciliation)
    vi.mocked(api.apiListFundTransfers).mockResolvedValue({ data: [], pagination: {} })
  })

  it('renders category summary cards and reconciliation metrics accurately', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Lokasi Dana')).toBeInTheDocument()
    })

    // Check 3 category cards exist
    expect(screen.getByText('REKENING BANK')).toBeInTheDocument()
    expect(screen.getByText('BRANKAS KAS')).toBeInTheDocument()
    expect(screen.getByText('CASH ON HAND (COH)')).toBeInTheDocument()

    // Check balances formatted
    expect(screen.getAllByText('Rp 1.500.000').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('Rp 500.000').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('Rp 250.000').length).toBeGreaterThanOrEqual(1)

    // Check reconciliation status
    expect(screen.getByText(/Cocok dengan Catatan/)).toBeInTheDocument()

    // Check location table rows
    expect(screen.getByText('Rekening BCA Operasional')).toBeInTheDocument()
    expect(screen.getByText('Brankas Utama')).toBeInTheDocument()
    expect(screen.getByText('COH Ketua RT')).toBeInTheDocument()
  })

  it('filters locations by category', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Rekening BCA Operasional')).toBeInTheDocument()
    })

    const select = screen.getByDisplayValue('Semua Kategori')
    fireEvent.change(select, { target: { value: 'rekening' } })

    expect(screen.getByText('Rekening BCA Operasional')).toBeInTheDocument()
    expect(screen.queryByText('Brankas Utama')).not.toBeInTheDocument()
    expect(screen.queryByText('COH Ketua RT')).not.toBeInTheDocument()
  })

  it('opens create location modal and submits new location', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Tambah Lokasi'))
    expect(screen.getByPlaceholderText(/Contoh: Rekening BCA RT/)).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText(/Contoh: Rekening BCA RT/), {
      target: { value: 'Rekening Mandiri' },
    })
    fireEvent.change(screen.getByPlaceholderText(/Contoh: BCA, Mandiri/), {
      target: { value: 'Mandiri' },
    })

    vi.mocked(api.apiCreateFundLocation).mockResolvedValueOnce({
      id: 'loc-new',
      rt_id: 'rt-1',
      name: 'Rekening Mandiri',
      category: 'rekening',
      bank_name: 'Mandiri',
      account_number: null,
      account_holder: null,
      is_active: true,
      current_balance: '0.00',
      created_at: '2026-10-02T00:00:00Z',
      updated_at: '2026-10-02T00:00:00Z',
    })

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(api.apiCreateFundLocation).toHaveBeenCalledWith('test-token', {
        name: 'Rekening Mandiri',
        category: 'rekening',
        bank_name: 'Mandiri',
        account_number: undefined,
        account_holder: undefined,
      })
    })
  })
})
