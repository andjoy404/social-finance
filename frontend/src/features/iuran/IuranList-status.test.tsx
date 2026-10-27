import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { IuranList } from './IuranList'
import * as api from '../../app/api'
import type { AuthUser } from '@/app/AuthContext'

vi.mock('../../app/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../app/api')>()
  return {
    ...actual,
    getSessionPair: vi.fn(),
    apiListBills: vi.fn(),
    apiListPayments: vi.fn(),
    apiGetBill: vi.fn(),
    apiListDues: vi.fn(),
    apiUpdateDue: vi.fn(),
  }
})

const bendaharaUser: AuthUser = {
  id: 'user-bendahara',
  name: 'Bendahara',
  email: 'bendahara@test.com',
  systemRole: null,
  role: 'bendahara',
  jabatan: 'bendahara',
  rt: { id: 'rt-1', name: 'RT 01' },
}

let mockCurrentUser: AuthUser | null = bendaharaUser

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: mockCurrentUser,
    isAuthenticated: mockCurrentUser !== null,
    isInitializing: false,
    login: vi.fn(),
    logout: vi.fn(),
  }),
}))

const activeDue: api.ApiDue = {
  id: 'due-active',
  rt_id: 'rt-1',
  rt_code: '001',
  name: 'Iuran Keamanan',
  amount: '50000.00',
  period_type: 'monthly',
  payment_rule: 'full',
  is_voluntary: false,
  is_active: true,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
}

const inactiveDue: api.ApiDue = {
  id: 'due-inactive',
  rt_id: 'rt-1',
  rt_code: '001',
  name: 'Iuran Lama',
  amount: '30000.00',
  period_type: 'monthly',
  payment_rule: 'full',
  is_voluntary: false,
  is_active: false,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
}

function renderKelolaIuran() {
  return render(
    <MemoryRouter initialEntries={['/iuran?tab=kelola-iuran']}>
      <IuranList />
    </MemoryRouter>
  )
}

describe('IuranList - Kelola Iuran status management', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mockCurrentUser = bendaharaUser
    ;(api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    ;(api.apiGetBill as any).mockResolvedValue(null)
    ;(api.apiListPayments as any).mockResolvedValue({ data: [] })
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })
  })

  it('loads all dues (including inactive) and shows the saved status', async () => {
    ;(api.apiListDues as any).mockResolvedValue([activeDue, inactiveDue])

    renderKelolaIuran()

    await waitFor(() => {
      expect(api.apiListDues).toHaveBeenCalledWith('test-token')
    })

    expect(await screen.findByText('Iuran Keamanan')).toBeInTheDocument()
    expect(screen.getByText('Iuran Lama')).toBeInTheDocument()
    expect(screen.getByText('Tidak Aktif')).toBeInTheDocument()
  })

  it('shows the current status in Edit Iuran and persists Aktif -> Tidak Aktif', async () => {
    ;(api.apiListDues as any)
      .mockResolvedValueOnce([activeDue])
      .mockResolvedValueOnce([{ ...activeDue, is_active: false }])
    ;(api.apiUpdateDue as any).mockResolvedValue({ ...activeDue, is_active: false })

    renderKelolaIuran()

    fireEvent.click(await screen.findByText('Iuran Keamanan'))

    const statusSelect = (await screen.findByLabelText('Status')) as HTMLSelectElement
    expect(statusSelect.value).toBe('true')

    fireEvent.change(statusSelect, { target: { value: 'false' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await waitFor(() => {
      expect(api.apiUpdateDue).toHaveBeenCalledWith(
        'test-token',
        'due-active',
        expect.objectContaining({ is_active: false })
      )
    })

    // Reloaded data reflects the persisted status.
    await waitFor(() => {
      expect(screen.queryByLabelText('Status')).toBeNull()
      expect(screen.getByText('Tidak Aktif')).toBeInTheDocument()
    })
  })

  it('allows reactivating an inactive due from Edit Iuran', async () => {
    ;(api.apiListDues as any).mockResolvedValue([inactiveDue])
    ;(api.apiUpdateDue as any).mockResolvedValue({ ...inactiveDue, is_active: true })

    renderKelolaIuran()

    fireEvent.click(await screen.findByText('Iuran Lama'))

    const statusSelect = (await screen.findByLabelText('Status')) as HTMLSelectElement
    expect(statusSelect.value).toBe('false')

    fireEvent.change(statusSelect, { target: { value: 'true' } })
    fireEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await waitFor(() => {
      expect(api.apiUpdateDue).toHaveBeenCalledWith(
        'test-token',
        'due-inactive',
        expect.objectContaining({ is_active: true })
      )
    })
  })
})

describe('IuranList - Kelola Iuran PERIODE column consistency', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mockCurrentUser = bendaharaUser
    ;(api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    ;(api.apiGetBill as any).mockResolvedValue(null)
    ;(api.apiListPayments as any).mockResolvedValue({ data: [] })
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })
  })

  it('menampilkan periode yang diformat (bulan + tahun) di kolom PERIODE Kelola Iuran', async () => {
    ;(api.apiListDues as any).mockResolvedValue([
      { ...activeDue, id: 'due-okt', period: '2026-10' },
      { ...activeDue, id: 'due-nov', period: '2026-11' },
    ])

    renderKelolaIuran()

    await waitFor(() => {
      expect(screen.getByText('Okt 2026')).toBeInTheDocument()
      expect(screen.getByText('Nov 2026')).toBeInTheDocument()
    })
  })

  it('menampilkan — ketika periode tidak tersedia di kolom PERIODE Kelola Iuran', async () => {
    ;(api.apiListDues as any).mockResolvedValue([
      // voluntary agar sel kolom BERLAKU SAMPAI tidak juga render '—'
      { ...activeDue, id: 'due-noperiod', is_voluntary: true },
    ])

    renderKelolaIuran()

    await waitFor(() => {
      expect(screen.getByText('—')).toBeInTheDocument()
    })
  })

  it('menampilkan nilai mentah saat periode tidak memiliki pemisah strip', async () => {
    ;(api.apiListDues as any).mockResolvedValue([
      { ...activeDue, id: 'due-raw', period: '2026' },
    ])

    renderKelolaIuran()

    await waitFor(() => {
      expect(screen.getByText('2026')).toBeInTheDocument()
    })
  })

  it('format PERIODE konsisten antara tab Iuran Saya dan Kelola Iuran', async () => {
    const billOkt: api.ApiBill = {
      id: 'bill-okt',
      rt_id: 'rt-1',
      due_id: 'due-1',
      household_occupancy_id: 'occ-1',
      period: '2026-10',
      due_date: '2026-10-31',
      amount: '50000.00',
      status: 'unpaid',
      head_name: 'Budi Santoso',
      due_name: 'Iuran Sampah',
      rt_code: '001',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
    }

    // Tab Iuran Saya (default)
    ;(api.apiListBills as any).mockResolvedValue({
      data: [billOkt],
      pagination: { page: 1, per_page: 10, total: 1, total_pages: 1 },
    })

    const saya = render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )
    await waitFor(() => expect(saya.getByText('Okt 2026')).toBeInTheDocument())
    saya.unmount()

    // Tab Kelola Iuran
    ;(api.apiListDues as any).mockResolvedValue([
      { ...activeDue, id: 'due-okt', period: '2026-10' },
    ])
    const kelola = renderKelolaIuran()
    await waitFor(() => expect(kelola.getByText('Okt 2026')).toBeInTheDocument())
    kelola.unmount()
  })
})
