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
    apiGenerateBills: vi.fn(),
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

describe('IuranList — Sinkronkan Tagihan KK', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.apiListBills).mockResolvedValue({
      data: [],
      pagination: { page: 1, page_size: 10, total: 0, total_pages: 0 },
    })
    vi.mocked(api.apiListDues).mockResolvedValue([
      {
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
      },
    ])
    vi.mocked(api.apiGetDueMonths).mockResolvedValue([
      {
        period: '2026-01',
        month: 1,
        year: 2026,
        total_bills: 5,
        active_bills: 5,
        paid_bills: 0,
        unpaid_bills: 5,
        is_active: true,
        can_deactivate: true,
      },
    ])
    vi.mocked(api.apiGenerateBills).mockResolvedValue({
      generated_count: 12,
      bills: [],
    })
  })

  it('renders "Sinkronkan Tagihan KK" button in Multi-Month edit modal and triggers sync', async () => {
    render(
      <MemoryRouter initialEntries={['/iuran?tab=kelola-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    // Wait for dues list to load
    await waitFor(() => {
      expect(screen.getByText('IPL 2026')).toBeInTheDocument()
    })

    // Click row to open Edit Modal
    fireEvent.click(screen.getByText('IPL 2026'))

    // Expect modal to show "Kelola Status 12 Bulan" and "Sinkronkan Tagihan KK" button
    await waitFor(() => {
      expect(screen.getByText(/Kelola Status 12 Bulan/i)).toBeInTheDocument()
      expect(screen.getByText(/Sinkronkan Tagihan KK/i)).toBeInTheDocument()
    })

    // Click "Sinkronkan Tagihan KK"
    fireEvent.click(screen.getByText(/Sinkronkan Tagihan KK/i))

    // Expect Confirmation Modal to appear
    await waitFor(() => {
      expect(screen.getByText(/Konfirmasi Sinkronisasi Tagihan KK/i)).toBeInTheDocument()
    })

    // Click "Ya, Sinkronkan"
    fireEvent.click(screen.getByText('Ya, Sinkronkan'))

    // Verify apiGenerateBills called with due_id and year 2026
    await waitFor(() => {
      expect(api.apiGenerateBills).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({
          due_id: 'due-mm-1',
          period: '2026',
        }),
        expect.any(String)
      )
    })

    // Verify success feedback message displayed
    await waitFor(() => {
      expect(screen.getByText(/Sinkronisasi berhasil: 12 tagihan baru diterbitkan/i)).toBeInTheDocument()
    })
  })
})
