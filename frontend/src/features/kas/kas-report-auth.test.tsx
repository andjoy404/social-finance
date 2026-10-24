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
  apiListTransactions: vi.fn().mockResolvedValue({
    data: [],
    pagination: { total: 0, total_pages: 1, page: 1, page_size: 100 },
  }),
  apiGetBalance: vi.fn().mockResolvedValue({
    total_income: '0',
    total_expense: '0',
    net_balance: '0',
    opening_balance: '0',
    closing_balance: '0',
  }),
  apiGetRT: vi.fn().mockResolvedValue({ id: 'rt-001', name: 'RT 001', rw_name: 'RW 001', address: '' }),
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

    const el = await screen.findByText(/Rekap Pengeluaran per Kategori/, {}, { timeout: 5000 })
    expect(el).toBeInTheDocument()
  })

  it('Skenario 2: Ketua RT (jabatan) + RT sesuai = boleh akses', async () => {
    const user = makeUser({ role: 'warga', jabatan: 'ketua' })
    renderKasReport(user)

    const el = await screen.findByText(/Rekap Pengeluaran per Kategori/, {}, { timeout: 5000 })
    expect(el).toBeInTheDocument()
  })

  it('Skenario 3: role === "ketua" tanpa jabatan = boleh akses (bukan warga biasa)', async () => {
    // role !== warga dan bukan jabatan terlarang => boleh akses
    const user = makeUser({ role: 'ketua', jabatan: null })
    renderKasReport(user)

    const el = await screen.findByText(/Rekap Pengeluaran per Kategori/, {}, { timeout: 5000 })
    expect(el).toBeInTheDocument()
  })

  it('Skenario 4: Warga = boleh akses (lihat laporan, tidak bisa export)', async () => {
    // Warga biasa (jabatan null) boleh melihat laporan sesuai aturan bisnis final.
    // Only jabatan Keamanan dan Operasional yang ditolak.
    const user = makeUser({ role: 'warga', jabatan: null })
    renderKasReport(user)

    const el = await screen.findByText(/Rekap Pengeluaran per Kategori/, {}, { timeout: 5000 })
    expect(el).toBeInTheDocument()
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

  it('Skenario 7: SUPER_ADMIN = boleh akses tanpa kecuali', async () => {
    // SUPER_ADMIN selalu boleh akses Laporan KAS.
    const user = makeUser({ systemRole: 'super_admin', role: 'warga', jabatan: null })
    renderKasReport(user)

    const el = await screen.findByText(/Rekap Pengeluaran per Kategori/, {}, { timeout: 5000 })
    expect(el).toBeInTheDocument()
  })

  it('Skenario 8: Wakil Ketua RT (jabatan) + RT sesuai = boleh akses', async () => {
    const user = makeUser({ role: 'warga', jabatan: 'wakil_ketua' })
    renderKasReport(user)

    const el = await screen.findByText(/Rekap Pengeluaran per Kategori/, {}, { timeout: 5000 })
    expect(el).toBeInTheDocument()
  })

  it('Skenario 9: Warga + jabatan anggota = boleh akses (anggota bukan jabatan terlarang)', async () => {
    // Hanya Keamanan (keamanan) dan Kebersihan/Pembangunan (kebersihan_pembangunan) yang ditolak.
    // Anggota adalah jabatan biasa yang tidak ada larangan.
    const user = makeUser({ role: 'warga', jabatan: 'anggota' })
    renderKasReport(user)

    const el = await screen.findByText(/Rekap Pengeluaran per Kategori/, {}, { timeout: 5000 })
    expect(el).toBeInTheDocument()
  })

  // ---- Export Permission Tests ----

  it('Bendahara (role) = boleh export PDF dan XLSX', async () => {
    const user = makeUser({ role: 'bendahara', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText(/Rekap Pengeluaran per Kategori/)).toBeInTheDocument()
    })
    // Tombol Export PDF dan XLSX hanya muncul jika canExport === true.
    // canExport = jabatan === 'ketua' || jabatan === 'wakil_ketua' || role === 'bendahara'
    expect(screen.queryByText(/Ekspor PDF/i)).toBeInTheDocument()
    expect(screen.queryByText(/Ekspor XLSX/i)).toBeInTheDocument()
  })

  it('Ketua (jabatan) = boleh export PDF dan XLSX', async () => {
    const user = makeUser({ role: 'warga', jabatan: 'ketua' })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText(/Rekap Pengeluaran per Kategori/)).toBeInTheDocument()
    })
    expect(screen.queryByText(/Ekspor PDF/i)).toBeInTheDocument()
    expect(screen.queryByText(/Ekspor XLSX/i)).toBeInTheDocument()
  })

  it('Wakil Ketua (jabatan) = boleh export PDF dan XLSX', async () => {
    const user = makeUser({ role: 'warga', jabatan: 'wakil_ketua' })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText(/Rekap Pengeluaran per Kategori/)).toBeInTheDocument()
    })
    expect(screen.queryByText(/Ekspor PDF/i)).toBeInTheDocument()
    expect(screen.queryByText(/Ekspor XLSX/i)).toBeInTheDocument()
  })

  it('Warga = tidak bisa export (tombol tidak muncul)', async () => {
    const user = makeUser({ role: 'warga', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText(/Rekap Pengeluaran per Kategori/)).toBeInTheDocument()
    })
    // canExport = false untuk warga biasa, jadi tombol export tidak muncul.
    expect(screen.queryByText(/Ekspor PDF/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Ekspor XLSX/i)).not.toBeInTheDocument()
  })

  it('SUPER_ADMIN = tidak bisa export (tidak ada jabatan/bendahara)', async () => {
    // SUPER_ADMIN bukan jabatan ketua/wakil_ketua dan bukan role bendahara.
    // canExport = jabatan === 'ketua' || jabatan === 'wakil_ketua' || role === 'bendahara'
    // Maka SUPER_ADMIN tanpa jabatan tersebut TIDAK bisa export.
    const user = makeUser({ systemRole: 'super_admin', role: 'warga', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText(/Rekap Pengeluaran per Kategori/)).toBeInTheDocument()
    })
    expect(screen.queryByText(/Ekspor PDF/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Ekspor XLSX/i)).not.toBeInTheDocument()
  })

  // ---- Unauthorized Jabatan Tests ----

  it('Jabatan Keamanan (keamanan) = ditolak akses laporan', async () => {
    const user = makeUser({ role: 'warga', jabatan: 'keamanan' })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    })
  })

  it('Jabatan Operasional (operasional) = ditolak akses laporan', async () => {
    const user = makeUser({ role: 'warga', jabatan: 'operasional' })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    })
  })

  it('Pengurus biasa (role pengurus, jabatan null) = boleh akses, tidak bisa export', async () => {
    const user = makeUser({ role: 'pengurus', jabatan: null })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText('Akses Ditolak')).not.toBeInTheDocument()
    })
    expect(screen.queryByText(/Rekap Pengeluaran per Kategori/)).toBeInTheDocument()
    // Pengurus biasa bukan ketua/wakil_ketua jabatan dan bukan bendahara role.
    expect(screen.queryByText(/Ekspor PDF/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Ekspor XLSX/i)).not.toBeInTheDocument()
  })

  it('Bonus: role bendahara + jabatan ketua = tetap boleh akses', async () => {
    const user = makeUser({ role: 'bendahara', jabatan: 'ketua' })
    renderKasReport(user)

    await waitFor(() => {
      expect(screen.queryByText('Akses Ditolak')).not.toBeInTheDocument()
    })
    expect(screen.queryByText(/Rekap Pengeluaran per Kategori/)).toBeInTheDocument()
  })
})
