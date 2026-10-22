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
