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
  }
})

const pengurusUser: AuthUser = {
  id: 'user-pengurus',
  name: 'Bendahara RT',
  email: 'bendahara@test.com',
  systemRole: null,
  role: 'bendahara',
  jabatan: 'bendahara',
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

describe('IuranList - Payment Status Filter', () => {
  const sampleBills: api.ApiBill[] = [
    {
      id: 'b-1',
      rt_id: 'rt-1',
      household_occupancy_id: 'ho-1',
      due_id: 'due-1',
      amount: '50000.00',
      period: '2026-10',
      due_date: '2026-10-31',
      status: 'unpaid',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
      due_name: 'Kebersihan',
      head_name: 'Budi Santoso',
      house_number: 'A-1',
      is_voluntary: false,
      paid_amount: '0.00',
      pending_amount: '50000.00',
    },
    {
      id: 'b-2',
      rt_id: 'rt-1',
      household_occupancy_id: 'ho-2',
      due_id: 'due-1',
      amount: '50000.00',
      period: '2026-10',
      due_date: '2026-10-31',
      status: 'paid',
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
      due_name: 'Kebersihan',
      head_name: 'Siti Aminah',
      house_number: 'A-2',
      is_voluntary: false,
      paid_amount: '50000.00',
      pending_amount: '0.00',
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    mockCurrentUser = pengurusUser
    vi.mocked(api.getSessionPair).mockReturnValue({
      accessToken: 'test-token',
      refreshToken: 'test-refresh-token',
    })
    vi.mocked(api.apiListBills).mockResolvedValue({
      data: sampleBills,
      pagination: {
        page: 1,
        page_size: 10,
        total: sampleBills.length,
        total_pages: 1,
      },
    })
    vi.mocked(api.apiListPayments).mockResolvedValue({
      data: [],
      pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 },
    })
  })

  it('provides "Status Pembayaran" option in filter dropdown', async () => {
    render(
      <MemoryRouter>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    const statusPembayaranBtn = screen.getByRole('button', { name: 'Status Pembayaran' })
    expect(statusPembayaranBtn).toBeInTheDocument()
  })

  it('selecting Status Pembayaran displays trigger with Menunggu Verifikasi, Disetujui, and Ditolak', async () => {
    render(
      <MemoryRouter>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    // Click 'Status Pembayaran' item in FilterDropdown menu
    const statusPembayaranItem = screen.getByRole('button', { name: 'Status Pembayaran' })
    fireEvent.click(statusPembayaranItem)

    const trigger = screen.getByRole('button', { name: /Pilih Status Pembayaran/i })
    expect(trigger).toBeInTheDocument()

    // Click trigger to open dropdown
    fireEvent.click(trigger)

    expect(screen.getByRole('button', { name: 'Menunggu Verifikasi' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disetujui' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Ditolak' })).toBeInTheDocument()
  })

  it('selecting "Menunggu Verifikasi" calls apiListBills with payment_status=pending', async () => {
    render(
      <MemoryRouter>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalled()
    })

    // Click 'Status Pembayaran' item in FilterDropdown menu
    const statusPembayaranItem = screen.getByRole('button', { name: 'Status Pembayaran' })
    fireEvent.click(statusPembayaranItem)

    // Open dropdown and select 'Menunggu Verifikasi'
    const trigger = screen.getByRole('button', { name: /Pilih Status Pembayaran/i })
    fireEvent.click(trigger)

    const pendingOption = screen.getByRole('button', { name: 'Menunggu Verifikasi' })
    fireEvent.click(pendingOption)

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({
          payment_status: 'pending',
        })
      )
    })

    // Active badge is shown
    expect(screen.getByText('Menunggu Verifikasi')).toBeInTheDocument()
  })

  it('clearing payment status filter removes payment_status from apiListBills query', async () => {
    render(
      <MemoryRouter initialEntries={['/?filter=pembayaran&pembayaran=pending']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({
          payment_status: 'pending',
        })
      )
    })

    // Click clear button
    const clearBtn = screen.getByRole('button', { name: 'Hapus filter pembayaran' })
    fireEvent.click(clearBtn)

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenLastCalledWith(
        'test-token',
        expect.objectContaining({
          payment_status: undefined,
        })
      )
    })
  })

  it('works for Warga role viewing their bills with payment filter', async () => {
    mockCurrentUser = wargaUser

    render(
      <MemoryRouter initialEntries={['/?filter=pembayaran&pembayaran=pending']}>
        <IuranList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(api.apiListBills).toHaveBeenCalledWith(
        'test-token',
        expect.objectContaining({
          payment_status: 'pending',
        })
      )
    })
  })
})
