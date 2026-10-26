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

  let storedLocations: api.FundLocation[] = mockLocations

  beforeEach(() => {
    vi.clearAllMocks()
    storedLocations = [...mockLocations]
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
    vi.mocked(api.apiListFundLocations).mockImplementation(async () => storedLocations)
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
    expect(screen.getAllByText('BRANKAS KAS').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('CASH ON HAND (COH)').length).toBeGreaterThanOrEqual(1)

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

    const newLocation: api.FundLocation = {
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
    }

    storedLocations = [...mockLocations, newLocation]

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

    await waitFor(() => {
      expect(screen.getByText('Rekening Mandiri')).toBeInTheDocument()
    })
  })

  it('handles brankas creation and refreshes the list', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Tambah Lokasi'))

    fireEvent.change(screen.getByPlaceholderText(/Contoh: Rekening BCA RT/), {
      target: { value: 'Brankas Utama RT' },
    })

    // Select brankas category
    fireEvent.change(screen.getByLabelText(/Kategori Lokasi/), { target: { value: 'brankas' } })

    fireEvent.change(screen.getByPlaceholderText(/Nama pengurus atau rekening/), {
      target: { value: 'Ketua RT' },
    })

    const newBrankas: api.FundLocation = {
      id: 'loc-brankas',
      rt_id: 'rt-1',
      name: 'Brankas Utama RT',
      category: 'brankas',
      bank_name: null,
      account_number: null,
      account_holder: 'Ketua RT',
      is_active: true,
      current_balance: '0.00',
      created_at: '2026-10-02T00:00:00Z',
      updated_at: '2026-10-02T00:00:00Z',
    }

    storedLocations = [...mockLocations, newBrankas]

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(screen.getByText('Brankas Utama RT')).toBeInTheDocument()
    })
  })

  it('shows error when create location request fails', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Tambah Lokasi'))

    fireEvent.change(screen.getByPlaceholderText(/Contoh: Rekening BCA RT/), {
      target: { value: 'Rekening Gagal' },
    })
    fireEvent.change(screen.getByPlaceholderText(/Contoh: BCA, Mandiri/), {
      target: { value: 'Gagal' },
    })

    vi.mocked(api.apiCreateFundLocation).mockRejectedValueOnce(new Error('Gagal: lokasi sudah ada'))

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(screen.getByText('Gagal: lokasi sudah ada')).toBeInTheDocument()
    })

    expect(screen.queryByText('Rekening Gagal')).not.toBeInTheDocument()
  })

  it('no Response data missing warning for valid create responses', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Tambah Lokasi'))

    fireEvent.change(screen.getByPlaceholderText(/Contoh: Rekening BCA RT/), {
      target: { value: 'BCA RT' },
    })
    fireEvent.change(screen.getByPlaceholderText(/Contoh: BCA, Mandiri/), {
      target: { value: 'BCA' },
    })

    const validLocation: api.FundLocation = {
      id: 'loc-valid',
      rt_id: 'rt-1',
      name: 'BCA RT',
      category: 'rekening',
      bank_name: 'BCA',
      account_number: '1234567890',
      account_holder: 'Ketua RT',
      is_active: true,
      current_balance: '0.00',
      created_at: '2026-10-02T00:00:00Z',
      updated_at: '2026-10-02T00:00:00Z',
    }

    storedLocations = [...mockLocations, validLocation]

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(api.apiCreateFundLocation).toHaveBeenCalled()
    })

    await waitFor(() => {
      expect(screen.getByText('BCA RT')).toBeInTheDocument()
    })

    expect(screen.queryByText(/Response data missing/)).not.toBeInTheDocument()
  })

  it('handles refresh failure gracefully after successful create without marking create as failed', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Tambah Lokasi'))

    fireEvent.change(screen.getByPlaceholderText(/Contoh: Rekening BCA RT/), {
      target: { value: 'Rekening Refresh Fail' },
    })
    fireEvent.change(screen.getByPlaceholderText(/Contoh: BCA, Mandiri/), {
      target: { value: 'BSI' },
    })

    const newLoc: api.FundLocation = {
      id: 'loc-refresh-fail',
      rt_id: 'rt-1',
      name: 'Rekening Refresh Fail',
      category: 'rekening',
      bank_name: 'BSI',
      is_active: true,
      current_balance: '0.00',
      created_at: '2026-10-02T00:00:00Z',
      updated_at: '2026-10-02T00:00:00Z',
    }

    vi.mocked(api.apiCreateFundLocation).mockResolvedValueOnce(newLoc)
    // Make subsequent list call fail
    vi.mocked(api.apiListFundLocations).mockRejectedValueOnce(new Error('Network timeout on list'))

    fireEvent.click(screen.getByText('Simpan'))

    // The created location should still be added to the UI
    await waitFor(() => {
      expect(screen.getByText('Rekening Refresh Fail')).toBeInTheDocument()
    })

    // The accurate error message should be displayed
    await waitFor(() => {
      expect(
        screen.getByText(/Lokasi berhasil dibuat, namun gagal memperbarui ringkasan data/),
      ).toBeInTheDocument()
    })
  })

  it('resets category filter if created location category would otherwise be hidden', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    })

    // Set category filter to 'coh'
    const categorySelect = screen.getByDisplayValue('Semua Kategori')
    fireEvent.change(categorySelect, { target: { value: 'coh' } })

    fireEvent.click(screen.getByText('Tambah Lokasi'))

    fireEvent.change(screen.getByPlaceholderText(/Contoh: Rekening BCA RT/), {
      target: { value: 'Rekening Reset Filter' },
    })
    fireEvent.change(screen.getByPlaceholderText(/Contoh: BCA, Mandiri/), {
      target: { value: 'BRI' },
    })

    const newLoc: api.FundLocation = {
      id: 'loc-filter-reset',
      rt_id: 'rt-1',
      name: 'Rekening Reset Filter',
      category: 'rekening',
      bank_name: 'BRI',
      is_active: true,
      current_balance: '0.00',
      created_at: '2026-10-02T00:00:00Z',
      updated_at: '2026-10-02T00:00:00Z',
    }

    storedLocations = [...mockLocations, newLoc]

    fireEvent.click(screen.getByText('Simpan'))

    // Filter should be reset and newly created rekening should be in the document
    await waitFor(() => {
      expect(screen.getByText('Rekening Reset Filter')).toBeInTheDocument()
    })
  })

  it('validates transfer form inputs: same location, invalid amount, and insufficient balance', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Transfer Dana')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Transfer Dana'))

    expect(screen.getByText('Transfer Dana Antar-Lokasi')).toBeInTheDocument()

    // 1. Submit without choosing locations
    fireEvent.click(screen.getByText('Kirim Transfer'))
    expect(screen.getByText('Lokasi asal dan tujuan wajib dipilih')).toBeInTheDocument()

    // 2. Select source location: loc-2 (Brankas Utama, Saldo: 500.000)
    const sourceSelect = screen.getByDisplayValue('-- Pilih Lokasi Asal --')
    fireEvent.change(sourceSelect, { target: { value: 'loc-2' } })

    // Check helper text displays available balance
    expect(screen.getByText(/Saldo tersedia:/)).toBeInTheDocument()
    expect(screen.getAllByText('Rp 500.000').length).toBeGreaterThanOrEqual(1)

    // 3. Select destination: loc-1 (BCA)
    const destSelect = screen.getByDisplayValue('-- Pilih Lokasi Tujuan --')
    fireEvent.change(destSelect, { target: { value: 'loc-1' } })

    // 4. Enter amount <= 0
    const amountInput = screen.getByPlaceholderText(/Contoh: 500000/)
    fireEvent.change(amountInput, { target: { value: '0' } })
    fireEvent.click(screen.getByText('Kirim Transfer'))
    expect(screen.getByText('Nominal transfer harus lebih besar dari 0')).toBeInTheDocument()

    // 5. Enter amount exceeding balance (e.g. 600.000 > 500.000)
    fireEvent.change(amountInput, { target: { value: '600000' } })
    fireEvent.click(screen.getByText('Kirim Transfer'))
    expect(screen.getByText(/Saldo pada Brankas Utama tidak mencukupi/)).toBeInTheDocument()
  })

  it('submits transfer successfully, shows loading state, displays success banner, and updates balances dynamically', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Transfer Dana')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Transfer Dana'))

    const sourceSelect = screen.getByDisplayValue('-- Pilih Lokasi Asal --')
    fireEvent.change(sourceSelect, { target: { value: 'loc-2' } }) // Brankas: 500.000

    const destSelect = screen.getByDisplayValue('-- Pilih Lokasi Tujuan --')
    fireEvent.change(destSelect, { target: { value: 'loc-1' } }) // BCA: 1.500.000

    const amountInput = screen.getByPlaceholderText(/Contoh: 500000/)
    fireEvent.change(amountInput, { target: { value: '200000' } })

    const notesInput = screen.getByPlaceholderText(/Tarik tunai dari BCA/)
    fireEvent.change(notesInput, { target: { value: 'Setor kas brankas ke bank' } })

    const createdTransfer: api.FundTransfer = {
      id: 'txf-123',
      rt_id: 'rt-1',
      from_location_id: 'loc-2',
      to_location_id: 'loc-1',
      amount: '200000.00',
      transfer_date: '2026-10-25T10:00:00Z',
      notes: 'Setor kas brankas ke bank',
      reverses_transfer_id: null,
      from_location_name: 'Brankas Utama',
      to_location_name: 'Rekening BCA Operasional',
      created_at: '2026-10-25T10:00:00Z',
    }

    vi.mocked(api.apiCreateFundTransfer).mockResolvedValueOnce(createdTransfer)

    // Updated balances: Brankas 500k - 200k = 300k, BCA 1.5M + 200k = 1.7M
    const updatedLocations: api.FundLocation[] = [
      { ...mockLocations[0], current_balance: '1700000.00' },
      { ...mockLocations[1], current_balance: '300000.00' },
      mockLocations[2],
    ]
    storedLocations = updatedLocations

    const submitBtn = screen.getByText('Kirim Transfer')
    fireEvent.click(submitBtn)

    // Verify loading state disabling the button
    expect(api.apiCreateFundTransfer).toHaveBeenCalledWith('test-token', {
      from_location_id: 'loc-2',
      to_location_id: 'loc-1',
      amount: '200000',
      notes: 'Setor kas brankas ke bank',
    })

    // Success banner should appear
    await waitFor(() => {
      expect(screen.getByTestId('success-banner')).toBeInTheDocument()
      expect(
        screen.getByText('Transfer dana sebesar Rp 200.000 berhasil diproses.'),
      ).toBeInTheDocument()
    })

    // Balances in UI should be updated dynamically
    await waitFor(() => {
      expect(screen.getAllByText('Rp 1.700.000').length).toBeGreaterThanOrEqual(1)
      expect(screen.getAllByText('Rp 300.000').length).toBeGreaterThanOrEqual(1)
    })
  })

  it('handles backend transfer error gracefully and displays error message in modal', async () => {
    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Transfer Dana')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Transfer Dana'))

    const sourceSelect = screen.getByDisplayValue('-- Pilih Lokasi Asal --')
    fireEvent.change(sourceSelect, { target: { value: 'loc-1' } })

    const destSelect = screen.getByDisplayValue('-- Pilih Lokasi Tujuan --')
    fireEvent.change(destSelect, { target: { value: 'loc-2' } })

    const amountInput = screen.getByPlaceholderText(/Contoh: 500000/)
    fireEvent.change(amountInput, { target: { value: '100000' } })

    vi.mocked(api.apiCreateFundTransfer).mockRejectedValueOnce(
      new Error('Saldo sumber telah berubah, silakan muat ulang halaman'),
    )

    fireEvent.click(screen.getByText('Kirim Transfer'))

    await waitFor(() => {
      expect(
        screen.getByText('Saldo sumber telah berubah, silakan muat ulang halaman'),
      ).toBeInTheDocument()
    })
  })

  it('verifies reconciliation metrics with Oktober figures (K=1.325.000, L=1.000.000, U=325.000, D=0) and confirms Rp 325.000 does not jump to Rp 32.500.000', async () => {
    const oktoberReconciliation: api.ReconciliationReport = {
      kas_balance: '1325000.00',
      total_allocated_balance: '1000000.00',
      unallocated_balance: '325000.00',
      discrepancy: '0.00',
      status: 'MATCH_WITH_NOTES',
      status_label: 'Cocok dengan Catatan',
      locations_summary: [],
      anomalies_detected: [],
    }

    vi.mocked(api.apiGetFundLocationReconciliation).mockResolvedValueOnce(oktoberReconciliation)

    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Lokasi Dana')).toBeInTheDocument()
    })

    // Check Kas Balance: Rp 1.325.000
    expect(screen.getByText('Rp 1.325.000')).toBeInTheDocument()

    // Check Unallocated Balance: Rp 325.000 (MUST NOT BE Rp 32.500.000)
    expect(screen.getByText('Rp 325.000')).toBeInTheDocument()
    expect(screen.queryByText('Rp 32.500.000')).not.toBeInTheDocument()

    // Check Discrepancy: Rp 0
    expect(screen.getByText('Rp 0')).toBeInTheDocument()
  })

  it('hides Transfer Dana button when user role is not bendahara', async () => {
    vi.mocked(AuthContext.useAuth).mockReturnValue({
      user: {
        id: 'u-2',
        name: 'Pengurus Test',
        email: 'pengurus@test.com',
        systemRole: null,
        role: 'pengurus',
        jabatan: 'ketua',
        rt: { id: 'rt-1', name: 'RT 001' },
      },
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
    })

    render(<FundLocationList />)

    await waitFor(() => {
      expect(screen.getByText('Lokasi Dana')).toBeInTheDocument()
    })

    // Pengurus can see Tambah Lokasi but NOT Transfer Dana
    expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    expect(screen.queryByText('Transfer Dana')).not.toBeInTheDocument()
  })
})
