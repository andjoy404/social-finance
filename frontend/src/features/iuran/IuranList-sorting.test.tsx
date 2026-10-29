import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
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

const pengurusUser: AuthUser = {
  id: 'user-ketua',
  name: 'Ketua RT',
  email: 'ketua@test.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'ketua',
  rt: { id: 'rt-1', name: 'RT 01' },
}

const wargaUser: AuthUser = {
  id: 'user-warga',
  name: 'Warga Test',
  email: 'warga@test.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  rt: { id: 'rt-1', name: 'RT 01' },
}

let mockCurrentUser: AuthUser | null = pengurusUser

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => ({
    user: mockCurrentUser,
    isAuthenticated: mockCurrentUser !== null,
    isInitializing: false,
    login: vi.fn(),
    logout: vi.fn(),
  }),
}))

function makeBill(over: Partial<api.ApiBill> & { id: string }): api.ApiBill {
  return {
    rt_id: 'rt-1',
    due_id: 'due-1',
    household_occupancy_id: 'occ-' + over.id,
    amount: '100000.00',
    period: '2026-01',
    due_date: '2026-01-31',
    status: 'unpaid',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    due_name: 'IPL 2026',
    house_number: over.id,
    head_name: 'Warga ' + over.id,
    paid_amount: '0',
    pending_amount: '0',
    ...over,
  }
}

// Deliberately shuffled server order (what the API returns without `order`).
// Tier 1 (pending) : bill-pending  → period 2026-06
// Tier 2 (unpaid)  : 2026-01, 2026-03, 2026-12 (ascending)
// Tier 3 (paid)    : 2026-02, 2026-07
const shuffledBills: api.ApiBill[] = [
  makeBill({ id: 'bill-paid-07', period: '2026-07', status: 'paid', paid_amount: '100000.00' }),
  makeBill({ id: 'bill-unpaid-12', period: '2026-12', status: 'unpaid' }),
  makeBill({ id: 'bill-pending-06', period: '2026-06', status: 'unpaid', pending_amount: '50000.00' }),
  makeBill({ id: 'bill-paid-02', period: '2026-02', status: 'paid', paid_amount: '100000.00' }),
  makeBill({ id: 'bill-unpaid-03', period: '2026-03', status: 'unpaid' }),
  makeBill({ id: 'bill-unpaid-01', period: '2026-01', status: 'unpaid' }),
]

/** Read the rendered table body in visual order. */
function renderedRowOrder(): string[] {
  const table = document.querySelector('table')
  if (!table) return []
  return Array.from(table.querySelectorAll('tbody tr')).map((tr) => {
    const text = tr.textContent || ''
    const match = /bill-(?:pending|unpaid|paid)-\d+/.exec(text)
    return match ? match[0] : ''
  })
}

describe('IuranList - "Iuran Warga" 3-tier sorting', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mockCurrentUser = pengurusUser
    ;(api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    ;(api.apiGetBill as any).mockResolvedValue(null)
    ;(api.apiListPayments as any).mockResolvedValue({ data: [] })
    ;(api.apiListDues as any).mockResolvedValue([])
  })

  it('renders pending first, then unpaid oldest-first, then paid', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: shuffledBills,
      pagination: { page: 1, per_page: 100, total: shuffledBills.length, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?tab=list-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    await waitFor(() => {
      expect(renderedRowOrder().length).toBeGreaterThan(0)
    })

    // The rendered rows must reflect the sort (regression: rendering used to
    // come straight from the unsorted `allBills` array).
    const periods = Array.from(document.querySelectorAll('table tbody tr')).map(
      (tr) => tr.textContent || ''
    )
    expect(periods.length).toBe(shuffledBills.length)

    const order = Array.from(document.querySelectorAll('table tbody tr')).map((tr) => {
      const text = tr.textContent || ''
      if (text.includes('Jun 2026')) return 'pending-2026-06'
      if (text.includes('Jan 2026')) return 'unpaid-2026-01'
      if (text.includes('Mar 2026')) return 'unpaid-2026-03'
      if (text.includes('Feb 2026')) return 'paid-2026-02'
      if (text.includes('Jul 2026')) return 'paid-2026-07'
      if (text.includes('Des 2026')) return 'unpaid-2026-12'
      return 'unknown'
    })

    expect(order).toEqual([
      'pending-2026-06', // tier 1
      'unpaid-2026-01', // tier 2, oldest first
      'unpaid-2026-03',
      'unpaid-2026-12',
      'paid-2026-02', // tier 3
      'paid-2026-07',
    ])
  })

  it('orders multiple pending bills by oldest period first', async () => {
    // Two pending payments at different months + one older unpaid bill.
    // Tier beats period: pending(2026-09) must outrank unpaid(2026-01),
    // and within the pending tier the OLDEST month (2026-05) comes first.
    const bills: api.ApiBill[] = [
      makeBill({ id: 'bill-pending-09', period: '2026-09', status: 'unpaid', pending_amount: '50000.00' }),
      makeBill({ id: 'bill-unpaid-01', period: '2026-01', status: 'unpaid' }),
      makeBill({ id: 'bill-pending-05', period: '2026-05', status: 'unpaid', pending_amount: '25000.00' }),
    ]
    ;(api.apiListBills as any).mockResolvedValue({
      data: bills,
      pagination: { page: 1, per_page: 100, total: bills.length, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?tab=list-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(renderedRowOrder().length).toBeGreaterThan(0)
    })

    const order = Array.from(document.querySelectorAll('table tbody tr')).map((tr) => {
      const text = tr.textContent || ''
      if (text.includes('Mei 2026')) return 'pending-2026-05'
      if (text.includes('Sep 2026')) return 'pending-2026-09'
      if (text.includes('Jan 2026')) return 'unpaid-2026-01'
      return 'unknown'
    })

    expect(order).toEqual([
      'pending-2026-05', // pending, oldest month
      'pending-2026-09', // pending, newer month
      'unpaid-2026-01', // arrears despite being the oldest period overall
    ])
  })

  it('puts the newest paid month at the very bottom', async () => {
    const bills: api.ApiBill[] = [
      makeBill({ id: 'bill-paid-12', period: '2026-12', status: 'paid', paid_amount: '100000.00' }),
      makeBill({ id: 'bill-paid-02', period: '2026-02', status: 'paid', paid_amount: '100000.00' }),
      makeBill({ id: 'bill-unpaid-03', period: '2026-03', status: 'unpaid' }),
    ]
    ;(api.apiListBills as any).mockResolvedValue({
      data: bills,
      pagination: { page: 1, per_page: 100, total: bills.length, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?tab=list-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(renderedRowOrder().length).toBeGreaterThan(0)
    })

    const order = Array.from(document.querySelectorAll('table tbody tr')).map((tr) => {
      const text = tr.textContent || ''
      if (text.includes('Mar 2026')) return 'unpaid-2026-03'
      if (text.includes('Feb 2026')) return 'paid-2026-02'
      if (text.includes('Des 2026')) return 'paid-2026-12'
      return 'unknown'
    })

    expect(order).toEqual(['unpaid-2026-03', 'paid-2026-02', 'paid-2026-12'])
    expect(order[order.length - 1]).toBe('paid-2026-12')
  })

  it('asks the backend for a global order on the Iuran Warga tab', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: shuffledBills,
      pagination: { page: 1, per_page: 100, total: shuffledBills.length, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?tab=list-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    const callArgs = (api.apiListBills as any).mock.calls[0][1]
    expect(callArgs.order).toBe('unpaid_first')
    // list-iuran is the RT-wide tab: must NOT be scoped to my_bills
    expect(callArgs.my_bills).toBeUndefined()
  })

  it('does not send order=unpaid_first on the "Iuran Saya" tab', async () => {
    mockCurrentUser = wargaUser
    ;(api.apiListBills as any).mockResolvedValue({
      data: shuffledBills,
      pagination: { page: 1, per_page: 100, total: shuffledBills.length, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    const callArgs = (api.apiListBills as any).mock.calls[0][1]
    expect(callArgs.order).toBeUndefined()
    expect(callArgs.my_bills).toBe(true)
  })
})

describe('IuranList - warga cannot reach manager tabs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mockCurrentUser = wargaUser
    ;(api.getSessionPair as any).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh',
    })
    ;(api.apiGetBill as any).mockResolvedValue(null)
    ;(api.apiListPayments as any).mockResolvedValue({ data: [] })
    ;(api.apiListDues as any).mockResolvedValue([])
  })

  it('clamps ?tab=list-iuran back to personal bills for a warga user', async () => {
    ;(api.apiListBills as any).mockResolvedValue({
      data: shuffledBills,
      pagination: { page: 1, per_page: 100, total: shuffledBills.length, total_pages: 1 },
    })

    render(
      <MemoryRouter initialEntries={['/iuran?tab=list-iuran']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    // No manager tabs, and the data request stays scoped to the warga's own bills.
    expect(screen.queryByRole('button', { name: 'Iuran Warga' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Kelola Iuran' })).toBeNull()
    await waitFor(() => {
      const lastCall = (api.apiListBills as any).mock.calls.at(-1)[1]
      expect(lastCall.my_bills).toBe(true)
      expect(lastCall.order).toBeUndefined()
    })
  })
})
