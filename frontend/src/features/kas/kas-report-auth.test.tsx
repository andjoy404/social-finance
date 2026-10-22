import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

// ---- Mocks (hoisted by Vitest before any imports) ----

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

vi.mock('@/app/api', () => ({
  apiListTransactions: vi.fn().mockResolvedValue({ data: [], pagination: { total: 0 } }),
  apiGetBalance: vi.fn().mockResolvedValue({ total_income: '0', total_expense: '0', net_balance: '0' }),
  getSessionPair: () => ({ accessToken: 'mock-token' }),
}))

// ---- Import after mocks ----

import { KasReport } from './KasReport'

// ---- Helpers ----

function makeUser(overrides: Record<string, any> = {}): Record<string, any> {
  return {
    id: 'test-user-id',
    name: 'Test User',
    email: 'test@example.com',
    systemRole: null,
    role: 'warga',
    jabatan: null,
    effectivePermissions: [],
    rt: { id: 'rt-001', name: 'RT 001' },
    ...overrides,
  }
}

function renderKasReport(user: Record<string, any> | null) {
  mockUseAuth.mockReturnValue({ user })
  mockUseNavigate.mockImplementation(() => {})
  return render(
    <MemoryRouter initialEntries={['/kas/report']}>
      <KasReport />
    </MemoryRouter>
  )
}

// ---- Tests ----

describe('KasReport authorization', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('Skenario 1: Bendahara + RT sesuai = boleh akses', async () => {
    const user = makeUser({ role: 'bendahara', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText('Akses Ditolak')).not.toBeInTheDocument()
    })
    expect(screen.getByText('Laporan KAS')).toBeInTheDocument()
  })

  it('Skenario 2: Ketua RT (jabatan) + RT sesuai = boleh akses', async () => {
    const user = makeUser({ role: 'warga', jabatan: 'ketua' })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText('Akses Ditolak')).not.toBeInTheDocument()
    })
    expect(screen.getByText('Laporan KAS')).toBeInTheDocument()
  })

  it('Skenario 3: role === "ketua" tapi jabatan !== "ketua" = ditolak', async () => {
    const user = makeUser({ role: 'ketua', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
      expect(screen.getByText('Anda tidak memiliki izin untuk mengakses Laporan KAS.')).toBeInTheDocument()
    })
  })

  it('Skenario 4: Warga = ditolak', async () => {
    const user = makeUser({ role: 'warga', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    })
  })

  it('Skenario 5: User memenuhi role/jabatan tapi RT beda = frontend passes, backend filter', async () => {
    const user = makeUser({ role: 'bendahara', rt: { id: 'wrong-rt-id', name: 'RT 999' } })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText('Akses Ditolak')).not.toBeInTheDocument()
    }, { timeout: 2000 })
    // Frontend authorization passes (role = bendahara)
    // Backend ListTransactions/GetBalance menggunakan getAuthContext(r) + tenant isolation
    // sehingga data RT lain tidak akan dikembalikan
  })

  it('Skenario 6: User tidak login (null) = ditolak', async () => {
    renderKasReport(null)

    await waitFor(() => {
      expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    })
  })

  it('Skenario 7: SUPER_ADMIN tanpa role bendahara atau jabatan ketua = ditolak', async () => {
    const user = makeUser({ systemRole: 'super_admin', role: 'warga', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    })
  })

  it('Bonus: role bendahara + jabatan ketua = tetap boleh akses', async () => {
    const user = makeUser({ role: 'bendahara', jabatan: 'ketua' })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText('Akses Ditolak')).not.toBeInTheDocument()
    })
    expect(screen.getByText('Laporan KAS')).toBeInTheDocument()
  })
})
