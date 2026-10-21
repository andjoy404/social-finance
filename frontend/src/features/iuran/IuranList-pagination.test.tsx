import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react'
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
  }
})

// ─── AUTH CONTEXT MOCK ─────────────────────────────────────────────────
// Fixtures defined BEFORE the mutable ref so references are valid.
// The vi.mock closure reads mockCurrentUser at call time, allowing tests
// to override it between renders.

const wargaUser: AuthUser = {
  id: 'user-warga',
  name: 'Warga Test',
  email: 'warga@test.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  rt: { id: 'rt-1', name: 'RT 01' },
}

const pengurusKetuaUser: AuthUser = {
  id: 'user-ketua',
  name: 'Ketua RT',
  email: 'ketua@test.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'ketua',
  rt: { id: 'rt-1', name: 'RT 01' },
}

const superAdminUser: AuthUser = {
  id: 'user-super',
  name: 'Super Admin',
  email: 'super@test.com',
  systemRole: 'super_admin',
  role: 'super_admin',
  jabatan: null,
  rt: null,
}

// Mutable reference — vi.mock closure reads this at call time.
let mockCurrentUser: AuthUser | null = null

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: mockCurrentUser,
    isAuthenticated: mockCurrentUser !== null,
    isInitializing: false,
    login: vi.fn(),
    logout: vi.fn(),
  }),
}))

describe('IuranList - Pagination and Bill Name Search', () => {
  const sampleBills: api.ApiBill[] = [
    {
      id: 'bill-1',
      rt_id: 'rt-1',
      due_id: 'due-1',
      household_occupancy_id: 'occ-1',
      period: '2026-10',
      due_date: '2026-10-31',
      amount: '50000.00',
      status: 'unpaid',
      head_name: 'Budi Santoso',
      house_number: 'A-10',
      due_name: 'Iuran Keamanan',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
      rt_code: '001',
    },
    {
      id: 'bill-2',
      rt_id: 'rt-1',
      due_id: 'due-2',
      household_occupancy_id: 'occ-2',
      period: '2026-10',
      due_date: '2026-10-31',
      amount: '30000.00',
      status: 'unpaid',
      head_name: 'Ahmad Dahlan',
      house_number: 'B-20',
      due_name: 'Iuran Kebersihan',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
      rt_code: '001',
    },
    {
      id: 'bill-3',
      rt_id: 'rt-1',
      due_id: 'due-3',
      household_occupancy_id: 'occ-3',
      period: '2026-10',
      due_date: '2026-10-31',
      amount: '0.00',
      status: 'paid',
      head_name: 'Siti Aminah',
      house_number: 'C-30',
      due_name: 'Iuran Sukarela',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
      rt_code: '001',
      is_voluntary: true,
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    ;(api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    ;(api.apiGetBill as any).mockResolvedValue(null)
    ;(api.apiListPayments as any).mockResolvedValue({ data: [] })
  })

  // ── 1. PAGINATION TESTS ──

  it('persists selected page_size from URL query param across refresh and sends to API', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: sampleBills,
      pagination: { page: 1, per_page: 25, total: 3, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?page_size=25']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalledWith('test-token', expect.objectContaining({
        page: 1,
        page_size: 25,
      }))
    })

    // localStorage should now store the selected page size
    expect(localStorage.getItem('social-finance:table-page-size:iuran')).toBe('25')
  })

  it('preserves page number across refresh and sends page: 2 to API', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [sampleBills[1]],
      pagination: { page: 2, per_page: 10, total: 20, total_pages: 2 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?page=2&page_size=10']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalledWith('test-token', expect.objectContaining({
        page: 2,
        page_size: 10,
      }))
    })

    expect(screen.getByText('Ahmad Dahlan')).toBeDefined()
  })

  it('changing page size when on page > 1 resets page to 1', async () => {
    ;(api.apiListBills as any).mockImplementation((_token: string, params?: { page?: number; page_size?: number }) => {
      return Promise.resolve({
        data: sampleBills,
        pagination: { page: params?.page ?? 1, per_page: params?.page_size ?? 10, total: 30, total_pages: 3 },
      })
    })

    render(
      <MemoryRouter initialEntries={['/iuran?page=2&page_size=10']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined()
    })

    // Find page size select trigger or dropdown and change it to 25
    const pageSizeSelect = screen.getByRole('combobox') as HTMLSelectElement
    fireEvent.change(pageSizeSelect, { target: { value: '25' } })

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalledWith('test-token', expect.objectContaining({
        page: 1,
        page_size: 25,
      }))
    })
  })

  // ── 2. SEARCH BY BILL NAME (NAMA TAGIHAN) TESTS ──

  it('filters specifically by bill name (nama tagihan), NOT citizen name or RT', async () => {
    ;(api.apiListBills as any).mockImplementation((_token: string, params?: { search?: string }) => {
      if (params?.search) {
        const q = params.search.toLowerCase()
        const filtered = sampleBills.filter(b => (b.due_name ?? '').toLowerCase().includes(q))
        return Promise.resolve({
          data: filtered,
          pagination: { page: 1, per_page: 10, total: filtered.length, total_pages: 1 },
        })
      }
      return Promise.resolve({
        data: sampleBills,
        pagination: { page: 1, per_page: 10, total: sampleBills.length, total_pages: 1 },
      })
    })

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined()
      expect(screen.getByText('Ahmad Dahlan')).toBeDefined()
    })

    const searchInput = screen.getByPlaceholderText('Cari nama tagihan...') as HTMLInputElement

    // 1. Search by bill name "Keamanan" -> matches Iuran Keamanan
    fireEvent.change(searchInput, { target: { value: 'Keamanan' } })

    await waitFor(() => {
      expect(screen.getByText('Iuran Keamanan')).toBeDefined()
      expect(screen.queryByText('Iuran Kebersihan')).toBeNull()
    })

    // 2. Search by citizen name "Budi" -> should NOT match any bill name
    fireEvent.change(searchInput, { target: { value: 'Budi' } })

    await waitFor(() => {
      expect(screen.getByText('Tidak ada data iuran.')).toBeDefined()
      expect(screen.queryByText('Iuran Keamanan')).toBeNull()
    })

    // 3. Search by RT code "001" -> should NOT match
    fireEvent.change(searchInput, { target: { value: '001' } })

    await waitFor(() => {
      expect(screen.getByText('Tidak ada data iuran.')).toBeDefined()
    })
  })

  it('debounces rapid keystrokes to prevent duplicate requests and race conditions', async () => {
    let callCount = 0
    ;(api.apiListBills as any).mockImplementation(() => {
      callCount++
      return Promise.resolve({
        data: sampleBills,
        pagination: { page: 1, per_page: 10, total: 3, total_pages: 1 },
      })
    })

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined()
    })

    const initialCalls = callCount
    const searchInput = screen.getByPlaceholderText('Cari nama tagihan...')

    // Rapidly type characters
    act(() => {
      fireEvent.change(searchInput, { target: { value: 'K' } })
      fireEvent.change(searchInput, { target: { value: 'Ke' } })
      fireEvent.change(searchInput, { target: { value: 'Kea' } })
      fireEvent.change(searchInput, { target: { value: 'Keamanan' } })
    })

    // Wait past debounce duration (300ms)
    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenLastCalledWith('test-token', expect.objectContaining({
        search: 'Keamanan',
      }))
    })

    // Debounce should coalesce rapid keystrokes
    expect(callCount - initialCalls).toBeLessThanOrEqual(2)
  })

  it('clearing search query restores full list without stale filter', async () => {
    ;(api.apiListBills as any).mockImplementation((_token: string, params?: { search?: string }) => {
      if (params?.search) {
        const q = params.search.toLowerCase()
        const filtered = sampleBills.filter(b => (b.due_name ?? '').toLowerCase().includes(q))
        return Promise.resolve({
          data: filtered,
          pagination: { page: 1, per_page: 10, total: filtered.length, total_pages: 1 },
        })
      }
      return Promise.resolve({
        data: sampleBills,
        pagination: { page: 1, per_page: 10, total: sampleBills.length, total_pages: 1 },
      })
    })

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined()
    })

    const searchInput = screen.getByPlaceholderText('Cari nama tagihan...')
    fireEvent.change(searchInput, { target: { value: 'Kebersihan' } })

    await waitFor(() => {
      expect(screen.getByText('Iuran Kebersihan')).toBeDefined()
      expect(screen.queryByText('Iuran Keamanan')).toBeNull()
    })

    // Clear search
    fireEvent.change(searchInput, { target: { value: '' } })

    await waitFor(() => {
      expect(screen.getByText('Iuran Keamanan')).toBeDefined()
      expect(screen.getByText('Iuran Kebersihan')).toBeDefined()
      expect(screen.getByText('Iuran Sukarela')).toBeDefined()
    })
  })

  it('resets page to 1 whenever search query changes', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: sampleBills,
      pagination: { page: 2, per_page: 10, total: 20, total_pages: 2 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?page=2']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeDefined()
    })

    const searchInput = screen.getByPlaceholderText('Cari nama tagihan...')
    fireEvent.change(searchInput, { target: { value: 'Keamanan' } })

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenLastCalledWith('test-token', expect.objectContaining({
        page: 1,
        search: 'Keamanan',
      }))
    })
  })

  it('restores search query from URL search parameter on refresh/load', async () => {
    ;(api.apiListBills as any).mockImplementation((_token: string, params?: { search?: string }) => {
      if (params?.search) {
        const q = params.search.toLowerCase()
        const filtered = sampleBills.filter(b => (b.due_name ?? '').toLowerCase().includes(q))
        return Promise.resolve({
          data: filtered,
          pagination: { page: 1, per_page: 10, total: filtered.length, total_pages: 1 },
        })
      }
      return Promise.resolve({
        data: sampleBills,
        pagination: { page: 1, per_page: 10, total: sampleBills.length, total_pages: 1 },
      })
    })

    render(
      <MemoryRouter initialEntries={['/iuran?q=Sukarela']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      const searchInput = screen.getByPlaceholderText('Cari nama tagihan...') as HTMLInputElement
      expect(searchInput.value).toBe('Sukarela')
      expect(screen.getByText('Iuran Sukarela')).toBeDefined()
      expect(screen.queryByText('Iuran Keamanan')).toBeNull()
    })
  })
})

// ─── ROLE-BASED IURAN TAB RESTRICTION TESTS ───────────────────────────

describe('IuranList - Role-Based Tab Restriction', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    ;(api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    ;(api.apiGetBill as any).mockResolvedValue(null)
    ;(api.apiListPayments as any).mockResolvedValue({ data: [] })
    ;(api.apiListDues as any).mockResolvedValue([])
    // Reset to warga (no access) so tests explicitly set the user they want
    mockCurrentUser = wargaUser
  })

  it('WARGA user does not see Iuran Warga or Kelola Iuran tabs', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = wargaUser

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    // WARGA tabs should NOT be visible
    expect(screen.queryByRole('button', { name: 'Iuran Warga' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Kelola Iuran' })).toBeNull()
  })

  it('WARGA user cannot access ?tab=kelola-iuran via URL', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = wargaUser

    render(
      <MemoryRouter initialEntries={['/iuran?tab=kelola-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    // WARGA cannot see any tab buttons even with URL param
    expect(screen.queryByRole('button', { name: 'Kelola Iuran' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Iuran Warga' })).toBeNull()
  })

  it('Bendahara user sees Iuran Saya, Iuran Warga, and Kelola Iuran tabs', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
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
    mockCurrentUser = bendaharaUser

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    expect(screen.getByRole('button', { name: 'Iuran Saya' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Iuran Warga' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Kelola Iuran' })).toBeVisible()
  })

  it('defaults to Iuran Saya (personal bills) for managers and orders tabs correctly', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = pengurusKetuaUser

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    // Personal scope is the default landing page, even for managers.
    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({ my_bills: true })
      )
    })

    const saya = screen.getByRole('button', { name: 'Iuran Saya' })
    const warga = screen.getByRole('button', { name: 'Iuran Warga' })
    const kelola = screen.getByRole('button', { name: 'Kelola Iuran' })

    expect(saya.compareDocumentPosition(warga) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(warga.compareDocumentPosition(kelola) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('keeps the ?tab=list-iuran deep link working for Iuran Warga', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = pengurusKetuaUser

    render(
      <MemoryRouter initialEntries={['/iuran?tab=list-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    const calls = (api.apiListBills as any).mock.calls
    const lastParams = calls[calls.length - 1]?.[1]
    expect(lastParams?.my_bills).not.toBe(true)
    expect(screen.getByRole('button', { name: 'Iuran Warga' })).toBeVisible()
  })

  it('Pengurus (ketua) user sees Iuran Saya, Iuran Warga, and Kelola Iuran tabs', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = pengurusKetuaUser

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    expect(screen.getByRole('button', { name: 'Iuran Warga' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Kelola Iuran' })).toBeVisible()
  })

  it('Super-admin user sees Iuran Saya, Iuran Warga, and Kelola Iuran tabs', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = superAdminUser

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    expect(screen.getByRole('button', { name: 'Iuran Warga' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Kelola Iuran' })).toBeVisible()
  })

  it('Ketua RT with role=warga and jabatan=ketua sees Iuran Saya, Iuran Warga, and Kelola Iuran tabs', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = {
      id: 'user-ketua-warga',
      name: 'Ketua Warga',
      email: 'ketua.warga@test.com',
      systemRole: null,
      role: 'warga',
      jabatan: 'ketua',
      rt: { id: 'rt-1', name: 'RT 01' },
    }

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    expect(screen.getByRole('button', { name: 'Iuran Warga' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Kelola Iuran' })).toBeVisible()
  })

  it('Bendahara RT with role=bendahara and jabatan=null sees Iuran Saya, Iuran Warga, and Kelola Iuran tabs', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = {
      id: 'user-bendahara-pure',
      name: 'Bendahara Pure',
      email: 'bendahara.pure@test.com',
      systemRole: null,
      role: 'bendahara',
      jabatan: null,
      rt: { id: 'rt-1', name: 'RT 01' },
    }

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    expect(screen.getByRole('button', { name: 'Iuran Warga' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Kelola Iuran' })).toBeVisible()
  })

  it('User with effectivePermissions containing finance.read sees Iuran Saya, Iuran Warga, and Kelola Iuran tabs', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    mockCurrentUser = {
      id: 'user-perm',
      name: 'User With Perm',
      email: 'perm@test.com',
      systemRole: null,
      role: 'warga',
      jabatan: null,
      effectivePermissions: ['finance.read'],
      rt: { id: 'rt-1', name: 'RT 01' },
    }

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    expect(screen.getByRole('button', { name: 'Iuran Warga' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Kelola Iuran' })).toBeVisible()
  })
})

// ─── EMPTY VS ERROR STATE TESTS ──────────────────────────────────────────

describe('IuranList - Empty vs Error States', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    ;(api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    ;(api.apiGetBill as any).mockResolvedValue(null)
    ;(api.apiListPayments as any).mockResolvedValue({ data: [] })
  })

  it('shows empty state "Belum ada tagihan." when API returns empty data list', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 10, total: 0, total_pages: 0 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByText('Belum ada tagihan.')).toBeInTheDocument()
    })
    expect(screen.queryByText('Gagal memuat data tagihan')).toBeNull()
  })

  it('shows bills when API returns data', async () => {
    const oneBill = [
      {
        id: 'bill-1',
        rt_id: 'rt-1',
        due_id: 'due-1',
        household_occupancy_id: 'occ-1',
        period: '2026-10',
        due_date: '2026-10-31',
        amount: '50000.00',
        status: 'unpaid',
        head_name: 'Budi Santoso',
        house_number: 'A-10',
        due_name: 'Iuran Keamanan',
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
        rt_code: '001',
      },
    ]

    ;(api.apiListBills as any).mockResolvedValue({
      data: oneBill,
      pagination: { page: 1, per_page: 10, total: 1, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })
    expect(screen.queryByText('Belum ada tagihan.')).toBeNull()
  })

  it('shows error message when API fails, not empty state', async () => {
    ;(api.apiListBills as any).mockRejectedValue(new Error('Gagal memuat data tagihan'))

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByText('Gagal memuat data tagihan')).toBeInTheDocument()
    })
    expect(screen.queryByText('Belum ada tagihan.')).toBeNull()
  })
})
