import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import { RwKasList } from './RwKasList'
import * as rwApi from './rw-api'
import * as AuthContext from '@/app/AuthContext'
import { getRwKasReportAccess } from './RwKasReport'
import { apiListCategories } from '@/app/api'

vi.mock('@/app/AuthContext', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/app/AuthContext')>()
  return {
    ...actual,
    useAuth: vi.fn(),
  }
})

vi.mock('@/app/api', () => ({
  getSessionPair: () => ({ accessToken: 'test-token' }),
  apiListCategories: vi.fn().mockResolvedValue([]),
}))

vi.mock('@/features/kas/kasReportExport', () => ({
  exportKasReportPdf: vi.fn().mockResolvedValue(undefined),
  exportKasReportXlsx: vi.fn().mockResolvedValue(undefined),
}))

const explicitBenh = {
  id: 'user-1',
  name: 'Bendahara RW 002',
  role: 'bendahara',
  jabatan: 'bendahara',
  rwMemberships: [{ rw_id: 'rw-002', rw_name: 'RW 002 Sukamaju', rw_number: 2, jabatan: 'bendahara', source: 'explicit' }],
}

const inferredKetua = {
  id: 'user-rt',
  name: 'Ketua RT 002',
  role: 'warga',
  jabatan: 'ketua',
  rwMemberships: [{ rw_id: 'rw-002', rw_name: 'RW 002', rw_number: 2, jabatan: 'ketua', source: 'rt_inferred' }],
}

const emptyMemberships = {
  id: 'user-out',
  name: 'Outsider',
  role: 'warga',
  jabatan: null,
  rwMemberships: [] as any[],
}

const detail = {
  opening_balance: '2000000.00',
  total_income: '1875000.00',
  total_expense: '550000.00',
  net_balance: '1325000.00',
  total_balance: '3325000.00',
  transaction_count: 2,
}

const txList = {
  data: [
    {
      id: 'i1',
      rw_id: 'rw-002',
      rt_id: 'rt-1',
      rt_name: 'RT 001',
      type: 'income',
      amount: '1000000.00',
      description: 'Penerimaan Iuran Warga',
      category_name: 'Iuran Warga',
      location_name: 'Rekening BCA Operasional RW',
      status: 'posted',
      payment_method: 'CASH',
      occurred_at: '2026-10-10T10:00:00Z',
      created_at: '2026-10-10T10:00:00Z',
    },
    {
      id: 'i2',
      rw_id: 'rw-002',
      rt_id: 'rt-1',
      rt_name: 'RT 001',
      type: 'income',
      amount: '250000.00',
      description: 'Infaq Warga',
      category_name: 'Infaq',
      location_name: 'Rekening BCA Operasional RW',
      status: 'posted',
      payment_method: 'CASH',
      occurred_at: '2026-10-11T10:00:00Z',
      created_at: '2026-10-11T10:00:00Z',
    },
    {
      id: 'i3',
      rw_id: 'rw-002',
      rt_id: 'rt-1',
      rt_name: 'RT 001',
      type: 'income',
      amount: '500000.00',
      description: 'Iuran Transfer Warga',
      category_name: 'Iuran Warga',
      location_name: 'Rekening BCA Operasional RW',
      status: 'posted',
      payment_method: 'TRANSFER',
      occurred_at: '2026-10-12T10:00:00Z',
      created_at: '2026-10-12T10:00:00Z',
    },
    {
      id: 'i4',
      rw_id: 'rw-002',
      rt_id: 'rt-1',
      rt_name: 'RT 001',
      type: 'income',
      amount: '999999.00',
      description: 'Draft income',
      category_name: 'Draft',
      location_name: 'Rekening BCA Operasional RW',
      status: 'draft',
      payment_method: 'CASH',
      occurred_at: '2026-10-13T10:00:00Z',
      created_at: '2026-10-13T10:00:00Z',
    },
    {
      id: 's1',
      rw_id: 'rw-002',
      rt_id: 'rt-3',
      rt_name: 'RT 003',
      type: 'income',
      amount: '125000.00',
      description: 'Setoran RT 003 ke Kas RW',
      category_name: 'Setoran RT',
      location_name: 'Rekening BCA Operasional RW',
      status: 'posted',
      payment_method: 'CASH',
      source_rt_id: 'rt-003',
      occurred_at: '2026-10-14T10:00:00Z',
      created_at: '2026-10-14T10:00:00Z',
    },
    {
      id: 'e1',
      rw_id: 'rw-002',
      rt_id: 'rt-1',
      rt_name: 'RT 001',
      type: 'expense',
      amount: '400000.00',
      description: 'Kebersihan',
      category_name: 'Kebersihan',
      location_name: 'Brankas RW',
      status: 'posted',
      payment_method: 'CASH',
      occurred_at: '2026-10-15T10:00:00Z',
      created_at: '2026-10-15T10:00:00Z',
    },
    {
      id: 'e3',
      rw_id: 'rw-002',
      rt_id: 'rt-1',
      rt_name: 'RT 001',
      type: 'expense',
      amount: '150000.00',
      description: 'Keamanan Transfer',
      category_name: 'Keamanan',
      location_name: 'Rekening BCA Operasional RW',
      status: 'posted',
      payment_method: 'TRANSFER',
      occurred_at: '2026-10-16T10:00:00Z',
      created_at: '2026-10-16T10:00:00Z',
    },
    {
      id: 'e4',
      rw_id: 'rw-002',
      rt_id: 'rt-1',
      rt_name: 'RT 001',
      type: 'expense',
      amount: '888888.00',
      description: 'Cancelled expense',
      category_name: 'Kebersihan',
      location_name: 'Brankas RW',
      status: 'cancelled',
      payment_method: 'CASH',
      occurred_at: '2026-10-17T10:00:00Z',
      created_at: '2026-10-17T10:00:00Z',
    },
  ],
  pagination: { page: 1, per_page: 100, total: 8, total_pages: 1 },
}

function mockBase() {
  vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue({
    opening_balance: '1000000.00',
    total_income: '5000000.00',
    total_expense: '2000000.00',
    net_balance: '3000000.00',
    total_balance: '4000000.00',
    transaction_count: 8,
  } as any)
  vi.spyOn(rwApi, 'apiListRWRTRs').mockResolvedValue([])
  vi.spyOn(rwApi, 'apiListRWLocations').mockResolvedValue([])
}

describe('getRwKasReportAccess', () => {
  it('allows explicit bendahara to view and export', () => {
    const a = getRwKasReportAccess(explicitBenh as any, 'rw-002')
    expect(a.canView).toBe(true)
    expect(a.canExportPdf).toBe(true)
    expect(a.canExportXlsx).toBe(true)
    expect(a.canPrint).toBe(true)
  })

  it('allows wakil_ketua explicit member to view and export xlsx/print', () => {
    const u = {
      ...explicitBenh,
      rwMemberships: [{ rw_id: 'rw-002', rw_name: 'RW 002', rw_number: 2, jabatan: 'wakil_ketua', source: 'explicit' }] ,
    }
    const a = getRwKasReportAccess(u as any, 'rw-002')
    expect(a.canView).toBe(true)
    expect(a.canExportXlsx).toBe(true)
  })

  it('allows anggota explicit member to view but not export xlsx/print', () => {
    const u = {
      ...explicitBenh,
      rwMemberships: [{ rw_id: 'rw-002', rw_name: 'RW 002', rw_number: 2, jabatan: 'anggota', source: 'explicit' }],
    }
    const a = getRwKasReportAccess(u as any, 'rw-002')
    expect(a.canView).toBe(true)
    expect(a.canExportPdf).toBe(true)
    expect(a.canExportXlsx).toBe(false)
    expect(a.canPrint).toBe(false)
  })

  it('denies rt_inferred members (RT only)', () => {
    const a = getRwKasReportAccess(inferredKetua as any, 'rw-002')
    expect(a.canView).toBe(false)
    expect(a.canExportPdf).toBe(false)
  })

  it('denies non members', () => {
    const a = getRwKasReportAccess(emptyMemberships as any, 'rw-002')
    expect(a.canView).toBe(false)
  })

  it('allows super_admin without membership', () => {
    const a = getRwKasReportAccess({ ...explicitBenh, systemRole: 'super_admin', rwMemberships: [] } as any, 'rw-002')
    expect(a.canView).toBe(true)
    expect(a.canExportXlsx).toBe(true)
  })
})

describe('RwKasList - Laporan KAS RW', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockBase()
    vi.mocked(AuthContext.useAuth as any).mockReturnValue({
      user: explicitBenh,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })
  })

  it('shows Laporan KAS button for authorized member and opens modal with backend data', async () => {
    const detailSpy = vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue(txList as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Penerimaan Iuran Warga')).toBeInTheDocument())

    const reportBtn = screen.getByRole('button', { name: /Laporan KAS/i })
    expect(reportBtn).toBeInTheDocument()
    fireEvent.click(reportBtn)

    const doc = await screen.findByTestId('kas-report-document')
    const withinDoc = within(doc)
    expect(withinDoc.getByText('Saldo awal periode')).toBeInTheDocument()
    expect(withinDoc.getByText('LAPORAN KEUANGAN RW 002')).toBeInTheDocument()
    expect(withinDoc.getAllByText('Rp2.000.000').length).toBeGreaterThan(0)
    expect(withinDoc.getByText('Rp1.875.000')).toBeInTheDocument()
    expect(withinDoc.getByText('Rp550.000')).toBeInTheDocument()
    expect(withinDoc.getByText('Rp3.325.000')).toBeInTheDocument()
    // RT identity must not leak
    expect(withinDoc.queryByText('LAPORAN KEUANGAN RT 002')).not.toBeInTheDocument()
    expect(detailSpy).toHaveBeenCalled()
  })

  it('does NOT render Laporan KAS button for rt_inferred members', async () => {
    vi.mocked(AuthContext.useAuth as any).mockReturnValue({
      user: inferredKetua,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({ data: [], pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 } } as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText(/KAS RW — RW 002/)).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: /Laporan KAS/i })).not.toBeInTheDocument()
  })

  it('opens modal even for super_admin without membership', async () => {
    vi.mocked(AuthContext.useAuth as any).mockReturnValue({
      user: { ...explicitBenh, systemRole: 'super_admin', rwMemberships: [] },
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })
    const detailSpy = vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue(txList as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByRole('button', { name: /Laporan KAS/i })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Laporan KAS/i }))
    await waitFor(() => expect(screen.getByText('Saldo awal periode')).toBeInTheDocument())
    expect(detailSpy).toHaveBeenCalledWith(expect.anything(), 'rw-002', expect.objectContaining({ period: expect.any(String) }))
  })

  it('exports PDF with RW identity and current data when exporting', async () => {
    const { exportKasReportPdf } = await import('@/features/kas/kasReportExport')
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue(txList as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Penerimaan Iuran Warga')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Laporan KAS/i }))
    await waitFor(() => expect(screen.getByText('Saldo awal periode')).toBeInTheDocument())

    const pdfBtn = screen.getByRole('button', { name: /Ekspor PDF/i })
    fireEvent.click(pdfBtn)
    await waitFor(() => expect(vi.mocked(exportKasReportPdf)).toHaveBeenCalled())
    const [model] = vi.mocked(exportKasReportPdf).mock.calls[0]
    expect(model.identity.rwLabel).toBe('RW 002')
    expect(model.identity.rtLabel).toBeNull()
    expect(model.closingCents).toBe(332500000)
  })

  it('shows error state when backend fails and provides retry', async () => {
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail')
      .mockResolvedValueOnce(detail as any) // page load
      .mockRejectedValue(new Error('Server sedang down')) // modal
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue(txList as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Penerimaan Iuran Warga')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Laporan KAS/i }))
    await waitFor(() => expect(screen.getByText(/Server sedang down/)).toBeInTheDocument())
    expect(screen.getByRole('button', { name: /Coba lagi/i })).toBeInTheDocument()
  })

  it('passes no period param when Semua Periode is active', async () => {
    const detailSpy = vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)
    const listSpy = vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({ ...txList, data: [], pagination: { page: 1, per_page: 100, total: 0, total_pages: 1 } } as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Semua Periode')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Semua Periode'))
    fireEvent.click(screen.getByRole('button', { name: /Laporan KAS/i }))
    await waitFor(() => expect(screen.getByText('Saldo awal periode')).toBeInTheDocument())
    expect(detailSpy).toHaveBeenCalledWith(expect.anything(), 'rw-002', {})
    const listCalls = listSpy.mock.calls.map((c) => c[2])
    expect(listCalls.some((p) => p && p.period === undefined)).toBe(true)
  })
})

describe('RwKasList - date range filter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockBase()
    vi.mocked(AuthContext.useAuth as any).mockReturnValue({
      user: explicitBenh,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })
  })

  it('sends start_date/end_date to the list and the report detail endpoint', async () => {
    const detailSpy = vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)
    const listSpy = vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 100, total: 0, total_pages: 1 },
    } as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Catat Kas Masuk')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('Tanggal mulai'), { target: { value: '2026-10-05' } })
    fireEvent.change(screen.getByLabelText('Tanggal selesai'), { target: { value: '2026-10-31' } })

    await waitFor(() => {
      const listCalledWithRange = listSpy.mock.calls.some(
        (c) => c[2]?.start_date === '2026-10-05' && c[2]?.end_date === '2026-10-31',
      )
      expect(listCalledWithRange).toBe(true)
    })
    await waitFor(() => {
      const detailCalledWithRange = detailSpy.mock.calls.some(
        (c) => c[2]?.startDate === '2026-10-05' && c[2]?.endDate === '2026-10-31',
      )
      expect(detailCalledWithRange).toBe(true)
    })
    // No period (month) is sent alongside a date range
    const rangeCalls = listSpy.mock.calls.filter((c) => c[2]?.start_date === '2026-10-05')
    expect(rangeCalls.every((c) => c[2]?.period === undefined)).toBe(true)
  })

  it('open report modal with the active range', async () => {
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 100, total: 0, total_pages: 1 },
    } as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Catat Kas Masuk')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('Tanggal mulai'), { target: { value: '2026-01-01' } })
    fireEvent.change(screen.getByLabelText('Tanggal selesai'), { target: { value: '2026-12-31' } })
    fireEvent.click(screen.getByRole('button', { name: /Laporan KAS/i }))
    await waitFor(() => expect(screen.getByText(/Tahun 2026/)).toBeInTheDocument())
  })

  it('clearing the range restores padam month-mode', async () => {
    const listSpy = vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 100, total: 0, total_pages: 1 },
    } as any)
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Catat Kas Masuk')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('Tanggal mulai'), { target: { value: '2026-10-05' } })
    fireEvent.change(screen.getByLabelText('Tanggal selesai'), { target: { value: '2026-10-31' } })
    await waitFor(() => {
      expect(listSpy.mock.calls.some((c) => c[2]?.start_date === '2026-10-05')).toBe(true)
    })

    fireEvent.click(screen.getByRole('button', { name: /Hapus rentang tanggal/i }))
    await waitFor(() => {
      expect(listSpy.mock.calls.slice(-1)[0]?.[2]?.period).toMatch(/^\d{4}-\d{2}$/)
    })
  })
})

describe('RwKasList - category filter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockBase()
    vi.mocked(AuthContext.useAuth as any).mockReturnValue({
      user: explicitBenh,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })
    vi.mocked(apiListCategories).mockResolvedValue([
      { id: 'cat-konsumsi', name: 'Konsumsi', type: 'expense', group: 'operasional', is_active: true },
    ] as any)
  })

  it('sends category_id to the list API when a category filter is selected', async () => {
    const listSpy = vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 100, total: 0, total_pages: 1 },
    } as any)
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByLabelText('Filter Kategori')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('Filter Kategori'), { target: { value: 'cat-konsumsi' } })

    await waitFor(() => {
      expect(listSpy.mock.calls.some((c) => c[2]?.categoryId === 'cat-konsumsi')).toBe(true)
    })
  })
})

describe('RwKasList - report edges', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockBase()
    vi.mocked(AuthContext.useAuth as any).mockReturnValue({
      user: explicitBenh,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })
  })

  it('shows empty state when the period has no transactions', async () => {
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue({
      ...detail,
      total_income: '0.00',
      total_expense: '0.00',
      net_balance: '0.00',
      total_balance: detail.opening_balance,
      transaction_count: 0,
    } as any)
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 100, total: 0, total_pages: 1 },
    } as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByRole('button', { name: /Laporan KAS/i })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Laporan KAS/i }))
    await waitFor(() => expect(screen.getByText(/Belum ada transaksi KAS pada/)).toBeInTheDocument())
  })

  it('exports XLSX with RW identity and current data', async () => {
    const { exportKasReportXlsx } = await import('@/features/kas/kasReportExport')
    vi.spyOn(rwApi, 'apiGetRWKASReportDetail').mockResolvedValue(detail as any)
    vi.spyOn(rwApi, 'apiListRWTransactions').mockResolvedValue(txList as any)

    render(<RwKasList rwId="rw-002" />)
    await waitFor(() => expect(screen.getByText('Penerimaan Iuran Warga')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Laporan KAS/i }))
    await waitFor(() => expect(screen.getByTestId('kas-report-document')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Ekspor XLSX/i }))
    await waitFor(() => expect(vi.mocked(exportKasReportXlsx)).toHaveBeenCalled())
    const [model] = vi.mocked(exportKasReportXlsx).mock.calls[0]
    expect(model.identity.rwLabel).toBe('RW 002')
    expect(model.identity.rtLabel).toBeNull()
    expect(model.identity.orgName).toBe('RW 002 Sukamaju')
  })
})
