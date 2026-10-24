import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { KasIncome } from './KasIncome'
import { KasExpense } from './KasExpense'
import * as apiModule from '@/app/api'
import { formatRibuan, parseMoney } from '../iuran/iuranTypes'

// Mock AuthContext
const mockUseAuth = vi.fn()
const mockUseNavigate = vi.fn()
const mockApiCreateTransaction = vi.fn()

vi.mock('@/app/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}))

vi.mock('@/app/wargaAuth', () => ({
  hasWargaWriteAccess: vi.fn(() => true),
}))

vi.mock('@/utils/jakartaTime', () => ({
  toJakartaIso8601: vi.fn((d: string) => `${d}T10:00:00.000+07:00`),
  getJakartaDateString: vi.fn(() => '2026-10-22'),
}))

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => mockUseNavigate(),
  }
})

// Mock URL.createObjectURL / revokeObjectURL (jsdom does not support blob URLs)
const mockCreateObjectURL = vi.fn(() => 'mocked-object-url')
const mockRevokeObjectURL = vi.fn()

vi.stubGlobal('URL', {
  ...globalThis.URL,
  createObjectURL: mockCreateObjectURL,
  revokeObjectURL: mockRevokeObjectURL,
})

const mockUser = {
  id: 'user-1',
  email: 'bendahara@local.com',
  role: 'bendahara',
  jabatan: 'bendahara',
  system_role: null,
}

const mockNavigate = vi.fn()

beforeEach(() => {
  vi.restoreAllMocks()
  mockCreateObjectURL.mockReturnValue('mocked-object-url')
  mockRevokeObjectURL.mockReset()
  mockUseAuth.mockReturnValue({ user: mockUser } as any)
  mockNavigate.mockImplementation(() => {})
  mockUseNavigate.mockReturnValue(mockNavigate)
  mockApiCreateTransaction.mockResolvedValue({ id: 'tx-1' })
  vi.spyOn(apiModule, 'getSessionPair').mockReturnValue({
    accessToken: 'test-token',
    refreshToken: 'test-refresh',
  })
  vi.spyOn(apiModule, 'apiCreateTransaction').mockImplementation(mockApiCreateTransaction)
})

afterEach(() => {
  vi.restoreAllMocks()
})

// Helper: get inline style value from a button
function getBtnStyle(btn: HTMLElement): string {
  return btn.getAttribute('style') || ''
}

describe('KAS Tambah — PEMASUKKAN/PENGELUARAN button styling and state', () => {
  it('KasIncome: PEMASUKKAN button is selected (green) by default', () => {
    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    const pemasukanBtn = screen.getByRole('button', { name: /Pemasukan/ })
    expect(pemasukanBtn).toBeInTheDocument()
    const style = getBtnStyle(pemasukanBtn)
    expect(style).toContain('rgba(115, 191, 105, 0.16)')
    expect(style).toContain('var(--sf-success)')
  })

  it('KasIncome: PENGELUARAN button is not selected (muted) by default', () => {
    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    const pengeluaranBtn = screen.getByRole('button', { name: /Pengeluaran/ })
    expect(pengeluaranBtn).toBeInTheDocument()
    const style = getBtnStyle(pengeluaranBtn)
    expect(style).toContain('var(--sf-surface)')
    expect(style).toContain('var(--sf-text-muted)')
  })

  it('KasIncome: PEMASUKKAN button has ArrowUp icon', () => {
    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    const pemasukanBtn = screen.getByRole('button', { name: /Pemasukan/ })
    const icon = pemasukanBtn.querySelector('svg')
    expect(icon).toBeInTheDocument()
  })

  it('KasExpense: PENGELUARAN button is selected (red) by default', () => {
    render(
      <MemoryRouter>
        <KasExpense />
      </MemoryRouter>
    )

    const pengeluaranBtn = screen.getByRole('button', { name: /Pengeluaran/ })
    expect(pengeluaranBtn).toBeInTheDocument()
    const style = getBtnStyle(pengeluaranBtn)
    expect(style).toContain('rgba(242, 73, 92, 0.16)')
    expect(style).toContain('var(--sf-danger)')
  })

  it('KasExpense: PEMASUKKAN button is not selected by default', () => {
    render(
      <MemoryRouter>
        <KasExpense />
      </MemoryRouter>
    )

    const pemasukanBtn = screen.getByRole('button', { name: /Pemasukan/ })
    expect(pemasukanBtn).toBeInTheDocument()
    const style = getBtnStyle(pemasukanBtn)
    expect(style).toContain('var(--sf-surface)')
    expect(style).toContain('var(--sf-text-muted)')
  })

  it('KasExpense: PENGELUARAN button has ArrowDown icon', () => {
    render(
      <MemoryRouter>
        <KasExpense />
      </MemoryRouter>
    )

    const pengeluaranBtn = screen.getByRole('button', { name: /Pengeluaran/ })
    const icon = pengeluaranBtn.querySelector('svg')
    expect(icon).toBeInTheDocument()
  })
})

describe('KAS Tambah — Nominal Rupiah formatting', () => {
  it('formatRibuan formats digits with dots', () => {
    expect(formatRibuan('500000')).toBe('500.000')
    expect(formatRibuan('1500000')).toBe('1.500.000')
    expect(formatRibuan('1000')).toBe('1.000')
  })

  it('parseMoney extracts digits from formatted string', () => {
    expect(parseMoney('500.000')).toBe(500000)
    expect(parseMoney('1.500.000')).toBe(1500000)
    expect(parseMoney('1000')).toBe(1000)
  })

  it('formatRibuan returns empty for empty input', () => {
    expect(formatRibuan('')).toBe('')
  })
})

describe('KAS Tambah — BUKTI BAYAR component', () => {
  it('KasIncome: shows upload zone when no file selected', () => {
    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    expect(screen.getByText('Pilih Berkas Bukti Bayar')).toBeInTheDocument()
    expect(screen.getByText('Format JPG, PNG, atau WebP (Maks. 5 MB)')).toBeInTheDocument()
  })

  it('KasIncome: shows file info when file selected', () => {
    const file = new File(['test-content'], 'bukti.jpg', { type: 'image/jpeg' })

    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    // Directly set files on the hidden input via data-testid
    const input = screen.getByTestId('kas-proof-file-input')
    fireEvent.change(input, { target: { files: [file] } })

    expect(screen.getByText('bukti.jpg')).toBeInTheDocument()
    expect(screen.getByText(/KB/)).toBeInTheDocument()
  })

  it('KasIncome: rejects non-image file', () => {
    const file = new File(['test-content'], 'document.pdf', { type: 'application/pdf' })

    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    const input = screen.getByTestId('kas-proof-file-input')
    fireEvent.change(input, { target: { files: [file] } })

    expect(screen.getByText(/Format file bukti harus berupa gambar/)).toBeInTheDocument()
  })

  it('KasIncome: removes file when delete button clicked', () => {
    const file = new File(['test-content'], 'bukti.jpg', { type: 'image/jpeg' })

    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    const input = screen.getByTestId('kas-proof-file-input')
    fireEvent.change(input, { target: { files: [file] } })
    expect(screen.getByText('bukti.jpg')).toBeInTheDocument()

    fireEvent.click(screen.getByTitle('Hapus file'))
    expect(screen.queryByText('bukti.jpg')).not.toBeInTheDocument()
    expect(screen.getByText('Pilih Berkas Bukti Bayar')).toBeInTheDocument()
  })

  it('KasExpense: shows upload zone when no file selected', () => {
    render(
      <MemoryRouter>
        <KasExpense />
      </MemoryRouter>
    )

    expect(screen.getByText('Pilih Berkas Bukti Bayar')).toBeInTheDocument()
  })
})

describe('KAS Tambah — Payload submit contract', () => {
  it('KasIncome: sends raw numeric amount (not formatted) to API', async () => {
    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    // Type formatted nominal
    const nominalInput = screen.getByRole('textbox', { name: /Nominal/ })
    fireEvent.change(nominalInput, { target: { value: '500.000' } })

    // Fill required fields
    fireEvent.change(screen.getByRole('textbox', { name: /Keterangan/ }), { target: { value: 'Test pemasukan' } })

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(mockApiCreateTransaction).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({
          amount: '500000',
          type: 'income',
        })
      )
    })
  })

  it('KasExpense: sends raw numeric amount (not formatted) to API', async () => {
    render(
      <MemoryRouter>
        <KasExpense />
      </MemoryRouter>
    )

    const nominalInput = screen.getByRole('textbox', { name: /Nominal/ })
    fireEvent.change(nominalInput, { target: { value: '150.000' } })

    fireEvent.change(screen.getByRole('textbox', { name: /Keterangan/ }), { target: { value: 'Test pengeluaran' } })

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(mockApiCreateTransaction).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({
          amount: '150000',
          type: 'expense',
        })
      )
    })
  })

  it('KasIncome: rejects nominal <= 0', async () => {
    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByRole('textbox', { name: /Nominal/ }), { target: { value: '0' } })
    fireEvent.change(screen.getByRole('textbox', { name: /Keterangan/ }), { target: { value: 'Test' } })

    fireEvent.click(screen.getByText('Simpan'))

    expect(screen.getByText(/Nominal harus berupa angka lebih dari 0/)).toBeInTheDocument()
    expect(mockApiCreateTransaction).not.toHaveBeenCalled()
  })

  it('KasIncome: navigation to expense page when PENGELUARAN clicked', () => {
    render(
      <MemoryRouter>
        <KasIncome />
      </MemoryRouter>
    )

    fireEvent.click(screen.getByRole('button', { name: /Pengeluaran/ }))
    expect(mockNavigate).toHaveBeenCalledWith('/kas/baru/keluar')
  })

  it('KasExpense: navigation to income page when PEMASUKKAN clicked', () => {
    render(
      <MemoryRouter>
        <KasExpense />
      </MemoryRouter>
    )

    fireEvent.click(screen.getByRole('button', { name: /Pemasukan/ }))
    expect(mockNavigate).toHaveBeenCalledWith('/kas/baru')
  })
})
