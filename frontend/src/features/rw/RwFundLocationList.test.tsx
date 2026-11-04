import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { RwFundLocationList } from './RwFundLocationList'
import * as rwApi from './rw-api'
import * as AuthContext from '@/app/AuthContext'

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 'user-1',
      name: 'Bendahara RW',
      role: 'bendahara',
      jabatan: 'bendahara',
      rwMemberships: [{ rw_id: 'rw-123', jabatan: 'bendahara', source: 'explicit' }],
    },
  }),
}))

vi.mock('@/app/api', () => ({
  getSessionPair: () => ({ accessToken: 'test-token' }),
}))

describe('RwFundLocationList', () => {
  it('renders 3 categories and completely eliminates Dana Hibah/Donasi', async () => {
    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([
      {
        id: 'loc-1',
        rt_id: '',
        rt_name: 'RW',
        name: 'Rekening Operasional RW',
        category: 'rekening',
        bank_name: 'BCA',
        account_number: '1234567890',
        account_holder: 'Pengurus RW',
        is_active: true,
        current_balance: '10000000.00',
        created_at: '2026-01-01T00:00:00Z',
      },
      {
        id: 'loc-2',
        rt_id: '',
        rt_name: 'RW',
        name: 'Brankas RW',
        category: 'brankas',
        is_active: true,
        current_balance: '2000000.00',
        created_at: '2026-01-01T00:00:00Z',
      },
      {
        id: 'loc-3',
        rt_id: '',
        rt_name: 'RW',
        name: 'Kas Di Tangan RW',
        category: 'coh',
        is_active: true,
        current_balance: '1000000.00',
        created_at: '2026-01-01T00:00:00Z',
      },
    ])

    render(<RwFundLocationList rwId="rw-123" />)

    await waitFor(() => {
      expect(screen.getByText('Lokasi Dana RW — rw-123')).toBeInTheDocument()
    })

    // Verify 3 categories are present with correct terminology
    expect(screen.getAllByText('Rekening Bank').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Brankas Fisik').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Kas Di Tangan (COH)').length).toBeGreaterThan(0)

    // Strictly verify Dana Hibah/Donasi does NOT exist
    expect(screen.queryByText(/Kas Hibah/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Dana Hibah/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Hibah\/Donasi/i)).not.toBeInTheDocument()

    // Verify action buttons for authorized role
    expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()
    expect(screen.getByText('Transfer Antar-Lokasi')).toBeInTheDocument()
    expect(screen.getByText('Riwayat Transfer')).toBeInTheDocument()
    expect(screen.getByText('Rekonsiliasi')).toBeInTheDocument()
  })

  it('respects granular permissions: wakil_ketua can add location but CANNOT transfer', async () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-wk',
        name: 'Wakil Ketua RW',
        role: 'warga',
        jabatan: 'wakil_ketua',
        rwMemberships: [{ rw_id: 'rw-123', jabatan: 'wakil_ketua', source: 'explicit' }],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([
      {
        id: 'loc-1',
        rt_id: '',
        rt_name: 'RW',
        name: 'Rekening Operasional RW',
        category: 'rekening',
        bank_name: 'BCA',
        account_number: '1234567890',
        account_holder: 'Pengurus RW',
        is_active: true,
        current_balance: '10000000.00',
        created_at: '2026-01-01T00:00:00Z',
      },
    ])

    render(<RwFundLocationList rwId="rw-123" />)

    await waitFor(() => {
      expect(screen.getByText('Lokasi Dana RW — rw-123')).toBeInTheDocument()
    })

    // wakil_ketua has finance.create -> CAN see "Tambah Lokasi"
    expect(screen.getByText('Tambah Lokasi')).toBeInTheDocument()

    // wakil_ketua lacks finance.update -> CANNOT see "Transfer Antar-Lokasi"
    expect(screen.queryByText('Transfer Antar-Lokasi')).not.toBeInTheDocument()

    // Can still view Riwayat Transfer and Rekonsiliasi
    expect(screen.getByText('Riwayat Transfer')).toBeInTheDocument()
    expect(screen.getByText('Rekonsiliasi')).toBeInTheDocument()
  })

  it('opens Riwayat Transfer modal and handles reversal for authorized users', async () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-bendahara',
        name: 'Bendahara RW',
        role: 'bendahara',
        jabatan: 'bendahara',
        rwMemberships: [{ rw_id: 'rw-123', jabatan: 'bendahara', source: 'explicit' }],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([])
    const mockListTransfers = vi.spyOn(rwApi, 'apiListRWTransfers').mockResolvedValue([
      {
        id: 'tr-1',
        rw_id: 'rw-123',
        from_location_id: 'loc-1',
        from_location_name: 'Rekening BCA',
        to_location_id: 'loc-2',
        to_location_name: 'Kas Di Tangan',
        amount: '500000.00',
        transfer_date: '2026-10-20T00:00:00Z',
        notes: 'Uang operasional',
        created_by: 'user-bendahara',
        reverses_transfer_id: null,
        is_reversal: false,
        created_at: '2026-10-20T00:00:00Z',
      },
    ])

    const { userEvent } = await import('@testing-library/user-event')
    const user = userEvent.setup()

    render(<RwFundLocationList rwId="rw-123" />)

    await waitFor(() => {
      expect(screen.getByText('Lokasi Dana RW — rw-123')).toBeInTheDocument()
    })

    // Click "Riwayat Transfer" button
    const historyBtn = screen.getByText('Riwayat Transfer')
    await user.click(historyBtn)

    // Modal title should appear
    await waitFor(() => {
      expect(screen.getByText('Riwayat Transfer Antar-Lokasi RW')).toBeInTheDocument()
    })

    expect(mockListTransfers).toHaveBeenCalledWith('test-token', 'rw-123')
    expect(screen.getByText('Rekening BCA')).toBeInTheDocument()
    expect(screen.getByText('Kas Di Tangan')).toBeInTheDocument()
    expect(screen.getByText('Uang operasional')).toBeInTheDocument()

    // Authorized user (bendahara) sees "Batalkan (Reversal)"
    const reverseBtn = screen.getByText('Batalkan (Reversal)')
    expect(reverseBtn).toBeInTheDocument()
  })
})

