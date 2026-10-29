import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { IuranList } from './IuranList'
import * as api from '@/app/api'

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof api>('@/app/api')
  return {
    ...actual,
    apiListBills: vi.fn(),
    apiListDues: vi.fn(),
    apiGetDueMonths: vi.fn(),
    apiUpdateDue: vi.fn(),
    apiUpdateDuePeriodStatus: vi.fn(),
    getSessionPair: vi.fn().mockReturnValue({ accessToken: 'test-token' }),
  }
})

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 'bendahara-1',
      role: 'bendahara',
      systemRole: null,
      name: 'Bendahara Test',
      rt: { id: 'rt-1', name: 'RT 01' },
      permissions: ['finance.read', 'finance.create', 'finance.update'],
    },
  }),
}))

const multiMonthDue: api.ApiDue = {
  id: 'due-mm-1',
  rt_id: 'rt-1',
  name: 'IPL 2026',
  amount: '100000.00',
  period_type: 'monthly',
  payment_rule: 'full',
  is_voluntary: false,
  is_active: true,
  due_type: 'multi_month',
  year: 2026,
  periods: ['2026-01', '2026-02'],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const mockMonths: api.DueMonthSummary[] = [
  {
    period: '2026-01',
    month: 1,
    year: 2026,
    total_bills: 5,
    active_bills: 5,
    paid_bills: 1,
    unpaid_bills: 4,
    is_active: true,
    can_deactivate: true,
  },
  {
    period: '2026-02',
    month: 2,
    year: 2026,
    total_bills: 5,
    active_bills: 5,
    paid_bills: 0,
    unpaid_bills: 5,
    is_active: true,
    can_deactivate: true,
  },
]

describe('IuranList — Multi-Month Period Status (Web Bug B)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.apiListBills).mockResolvedValue({
      data: [],
      pagination: { page: 1, page_size: 10, total: 0, total_pages: 0 },
    })
    vi.mocked(api.apiListDues).mockResolvedValue([multiMonthDue])
    vi.mocked(api.apiGetDueMonths).mockResolvedValue(mockMonths)
    vi.mocked(api.apiUpdateDue).mockResolvedValue(multiMonthDue)
    vi.mocked(api.apiUpdateDuePeriodStatus).mockResolvedValue(mockMonths)
  })

  it('clicking Nonaktifkan updates local UI state without calling API before Simpan', async () => {
    render(
      <MemoryRouter initialEntries={['/iuran?tab=kelola-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    // Open Edit modal
    fireEvent.click(await screen.findByText('IPL 2026'))
    expect(await screen.findByText('Januari')).toBeInTheDocument()

    // Find the Nonaktifkan button for Januari
    const deactBtns = screen.getAllByRole('button', { name: 'Nonaktifkan' })
    expect(deactBtns.length).toBeGreaterThan(0)

    // Click Nonaktifkan on Januari
    fireEvent.click(deactBtns[0])

    // API should NOT be called yet!
    expect(api.apiUpdateDuePeriodStatus).not.toHaveBeenCalled()

    // UI state should reflect the change locally
    expect(screen.getByRole('button', { name: 'Aktifkan' })).toBeInTheDocument()
  })

  it('clicking Batal discards local changes without calling API', async () => {
    render(
      <MemoryRouter initialEntries={['/iuran?tab=kelola-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    fireEvent.click(await screen.findByText('IPL 2026'))
    expect(await screen.findByText('Januari')).toBeInTheDocument()

    const deactBtns = screen.getAllByRole('button', { name: 'Nonaktifkan' })
    fireEvent.click(deactBtns[0])

    // Click Batal
    fireEvent.click(screen.getByRole('button', { name: 'Batal' }))

    // Modal closes without any status API calls
    await waitFor(() => {
      expect(screen.queryByText('Status Tagihan per Bulan')).toBeNull()
    })
    expect(api.apiUpdateDuePeriodStatus).not.toHaveBeenCalled()
    expect(api.apiUpdateDue).not.toHaveBeenCalled()
  })

  it('clicking Simpan sends pending month changes to API and refreshes server state', async () => {
    render(
      <MemoryRouter initialEntries={['/iuran?tab=kelola-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    fireEvent.click(await screen.findByText('IPL 2026'))
    expect(await screen.findByText('Januari')).toBeInTheDocument()

    const deactBtns = screen.getAllByRole('button', { name: 'Nonaktifkan' })
    // Toggle Januari to inactive
    fireEvent.click(deactBtns[0])

    // Click Simpan
    fireEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await waitFor(() => {
      expect(api.apiUpdateDuePeriodStatus).toHaveBeenCalledWith(
        'test-token',
        'due-mm-1',
        '2026-01',
        false
      )
      expect(api.apiUpdateDue).toHaveBeenCalled()
    })
  })

  it('displays API error when month status update fails and keeps modal open', async () => {
    vi.mocked(api.apiUpdateDuePeriodStatus).mockRejectedValueOnce(new Error('Network error on update'))

    render(
      <MemoryRouter initialEntries={['/iuran?tab=kelola-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    fireEvent.click(await screen.findByText('IPL 2026'))
    expect(await screen.findByText('Januari')).toBeInTheDocument()

    const deactBtns = screen.getAllByRole('button', { name: 'Nonaktifkan' })
    fireEvent.click(deactBtns[0])

    fireEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await waitFor(() => {
      expect(screen.getByText(/Gagal memperbarui status bulan/)).toBeInTheDocument()
    })
    // Main due update was NOT called because month update failed
    expect(api.apiUpdateDue).not.toHaveBeenCalled()
    // Modal is still open
    expect(screen.getByText(/Kelola Status 12 Bulan/)).toBeInTheDocument()
  })
})
