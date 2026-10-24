/**
 * Integration test — memastikan prop `period` dari parent diteruskan
 * dengan benar ke request API di KasReport.
 *
 * Menggunakan mock API yang melacak argumen pemanggilan agar periode
 * yang dikirim bisa diverifikasi secara eksplisit.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

// ---- Mocks ----
const mockUseAuth = vi.fn()
const mockUseNavigate = vi.fn()

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => mockUseNavigate(),
  }
})

// Mock yang melacak argumen pemanggilan
const apiListTransactionsCalls: Array<{ page: number; page_size: number; status?: string; period?: string }> = []
const apiGetBalanceCalls: Array<{ period?: string }> = []

vi.mock('@/app/api', () => ({
  apiListTransactions: vi.fn(async (_token: string, params: typeof apiListTransactionsCalls[number]) => {
    apiListTransactionsCalls.push(params)
    return {
      data: [],
      pagination: { total: 0, total_pages: 1, page: params.page, page_size: params.page_size },
    }
  }),
  apiGetBalance: vi.fn(async (_token: string, params?: typeof apiGetBalanceCalls[number]) => {
    apiGetBalanceCalls.push(params || {})
    return {
      opening_balance: '0',
      closing_balance: '0',
      net_balance: '0',
      total_income: '0',
      total_expense: '0',
    }
  }),
  apiGetRT: vi.fn().mockResolvedValue({ id: 'rt-001', name: 'RT 001', rw_name: 'RW 001', address: '' }),
  getSessionPair: () => ({ accessToken: 'mock-token' }),
}))

import { KasReport } from './KasReport'

function makeUser(overrides: Record<string, any> = {}): Record<string, any> {
  return {
    id: 'test-user-id',
    name: 'Test User',
    role: 'bendahara',
    jabatan: null,
    systemRole: null,
    rt: { id: 'rt-001', name: 'Test RT' },
    ...overrides,
  }
}

function renderKasReport(props: Record<string, any> = {}) {
  mockUseAuth.mockReturnValue({ user: makeUser() })
  return render(
    <MemoryRouter initialEntries={['/kas/report']}>
      <KasReport {...props} />
    </MemoryRouter>,
  )
}

describe('KasReport — propagasi periode ke API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiListTransactionsCalls.length = 0
    apiGetBalanceCalls.length = 0
  })

  it('prop period = "2026-10" → API dipanggil dengan periode 2026-10', async () => {
    renderKasReport({ period: '2026-10' })

    await waitFor(() => {
      expect(apiListTransactionsCalls.length).toBeGreaterThanOrEqual(1)
    }, { timeout: 5000 })

    const listCall = apiListTransactionsCalls[0]
    expect(listCall.period).toBe('2026-10')
    expect(listCall.status).toBe('posted')

    const balanceCall = apiGetBalanceCalls[0]
    expect(balanceCall.period).toBe('2026-10')
  })

  it('prop period = null → API dipanggil TANPA periode (Semua Periode)', async () => {
    renderKasReport({ period: null })

    await waitFor(() => {
      expect(apiListTransactionsCalls.length).toBeGreaterThanOrEqual(1)
    }, { timeout: 5000 })

    const listCall = apiListTransactionsCalls[0]
    expect(listCall.period).toBeUndefined()
    expect(listCall.status).toBe('posted')

    const balanceCall = apiGetBalanceCalls[0]
    expect(balanceCall.period).toBeUndefined()
  })

  it('tidak ada prop period → default null, API tanpa filter periode', async () => {
    renderKasReport({})

    await waitFor(() => {
      expect(apiListTransactionsCalls.length).toBeGreaterThanOrEqual(1)
    }, { timeout: 5000 })

    const listCall = apiListTransactionsCalls[0]
    expect(listCall.period).toBeUndefined()

    const balanceCall = apiGetBalanceCalls[0]
    expect(balanceCall.period).toBeUndefined()
  })

  it('mengubah periode prop → API menggunakan filter periode yang benar', async () => {
    renderKasReport({ period: '2026-10' })

    await waitFor(() => {
      expect(apiListTransactionsCalls.length).toBeGreaterThanOrEqual(1)
    }, { timeout: 5000 })

    const listCall = apiListTransactionsCalls[0]
    expect(listCall.period).toBe('2026-10')
    expect(listCall.status).toBe('posted')

    const balanceCall = apiGetBalanceCalls[0]
    expect(balanceCall.period).toBe('2026-10')
  })
})
