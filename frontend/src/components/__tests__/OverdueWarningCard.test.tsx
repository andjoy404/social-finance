import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { OverdueWarningCard } from '../OverdueWarningCard'
import * as AuthContextModule from '@/app/AuthContext'
import * as ApiModule from '@/app/api'

function LocationDisplay() {
  const location = useLocation()
  return <div data-testid="current-location">{location.pathname + location.search}</div>
}

const baseWargaUser: AuthContextModule.AuthUser = {
  id: 'user-warga-1',
  name: 'Budi Warga',
  email: 'budi@warga.local',
  phone: '081234567890',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  effectivePermissions: [],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
  rwId: 'rw-001',
  rwMemberships: [],
}

function mockAuth(user: AuthContextModule.AuthUser) {
  vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
    user,
    loading: false,
    login: vi.fn(),
    logout: vi.fn(),
    refreshUser: vi.fn(),
  } as unknown as ReturnType<typeof AuthContextModule.useAuth>)
}

describe('OverdueWarningCard (Web Global Warning Card — personal scope)', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(ApiModule, 'getSessionPair').mockReturnValue({
      accessToken: 'test-access-token',
      refreshToken: 'test-refresh-token',
    })
  })

  it('1. renders warning card for warga with >= 3 months overdue bills (personal data)', async () => {
    mockAuth(baseWargaUser)

    const apiSpy = vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: true,
      total_outstanding: 450000,
      overdue_bill_count: 3,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByTestId('overdue-warning-card')).toBeInTheDocument()
    })

    expect(apiSpy).toHaveBeenCalledTimes(1)
    expect(screen.getByText('Peringatan Tunggakan Iuran')).toBeInTheDocument()
    expect(screen.getByText('Tunggakan 3 Bulan+')).toBeInTheDocument()
    expect(
      screen.getByText(
        /Anda memiliki tagihan iuran yang belum lunas selama 3 bulan atau lebih/i,
      ),
    ).toBeInTheDocument()
    expect(screen.getByText(/450\.000/)).toBeInTheDocument()
    expect(screen.getByText('3 tagihan')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Lihat Tagihan Saya/i })).toBeInTheDocument()
  })

  it('1b. ketua RT sees personal summary with their own bills (not RT-wide)', async () => {
    const ketuaUser: AuthContextModule.AuthUser = {
      ...baseWargaUser,
      role: 'warga',
      jabatan: 'ketua',
      effectivePermissions: ['finance.read', 'finance.create'],
    }
    mockAuth(ketuaUser)

    vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: true,
      total_outstanding: 700000,
      overdue_bill_count: 7,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByTestId('overdue-warning-card')).toBeInTheDocument()
    })
    // Same warga copy for pengurus — never RT-wide wording
    expect(screen.getByText('Peringatan Tunggakan Iuran')).toBeInTheDocument()
    expect(screen.queryByText('Peringatan Tunggakan Iuran Warga')).not.toBeInTheDocument()
    expect(
      screen.getByText(/Anda memiliki tagihan iuran yang belum lunas/),
    ).toBeInTheDocument()
    expect(screen.queryByText(/RT Anda/)).not.toBeInTheDocument()
    expect(screen.getByText(/700\.000/)).toBeInTheDocument()
    expect(screen.getByText('7 tagihan')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Lihat Tagihan Saya/i })).toBeInTheDocument()
  })

  it('1c. sekretaris RT with no personal overdue does not see card even if RT has overdue elsewhere', async () => {
    const sekretarisUser: AuthContextModule.AuthUser = {
      ...baseWargaUser,
      role: 'warga',
      jabatan: 'sekretaris',
      effectivePermissions: ['finance.read', 'finance.create'],
    }
    mockAuth(sekretarisUser)

    // Backend returns personal summary (0 overdue for this occupancy) even though RT has overdue bills
    vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: false,
      total_outstanding: 0,
      overdue_bill_count: 0,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(ApiModule.apiGetOverdueWarningSummary).toHaveBeenCalledTimes(1)
    })
    expect(screen.queryByTestId('overdue-warning-card')).not.toBeInTheDocument()
  })

  it('1d. bendahara personal overdue renders with personal copy', async () => {
    const bendaharaUser: AuthContextModule.AuthUser = {
      ...baseWargaUser,
      role: 'warga',
      jabatan: 'bendahara',
      effectivePermissions: ['finance.read', 'finance.create'],
    }
    mockAuth(bendaharaUser)

    vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: true,
      total_outstanding: 800000,
      overdue_bill_count: 8,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByTestId('overdue-warning-card')).toBeInTheDocument()
    })
    expect(screen.getByText(/800\.000/)).toBeInTheDocument()
    expect(screen.getByText('8 tagihan')).toBeInTheDocument()
    expect(screen.getByText('Peringatan Tunggakan Iuran')).toBeInTheDocument()
  })

  it('1e. pengurus tanpa finance.read dengan personal overdue tetap melihat card (tidak ada pengecualian role)', async () => {
    const pengurusNoPerm: AuthContextModule.AuthUser = {
      ...baseWargaUser,
      role: 'warga',
      jabatan: 'sosial',
      effectivePermissions: ['warga.read'],
    }
    mockAuth(pengurusNoPerm)

    vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: true,
      total_outstanding: 250000,
      overdue_bill_count: 2,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByTestId('overdue-warning-card')).toBeInTheDocument()
    })
  })

  it('2 & 3. does not render for user with no overdue or < 3 months overdue', async () => {
    mockAuth(baseWargaUser)

    vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: false,
      total_outstanding: 0,
      overdue_bill_count: 0,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(ApiModule.apiGetOverdueWarningSummary).toHaveBeenCalledTimes(1)
    })
    expect(screen.queryByTestId('overdue-warning-card')).not.toBeInTheDocument()
  })

  it('4. pengurus, Ketua, RW pengurus, dan Super Admin tetap dipanggil Personal (tanpa RT-wide)', () => {
    const users: AuthContextModule.AuthUser[] = [
      { ...baseWargaUser, role: 'pengurus' },
      { ...baseWargaUser, role: 'bendahara' },
      { ...baseWargaUser, systemRole: 'super_admin', role: 'super_admin' },
      { ...baseWargaUser, role: 'warga', jabatan: 'ketua', effectivePermissions: ['finance.read'] },
      { ...baseWargaUser, role: 'warga', jabatan: 'bendahara', effectivePermissions: ['finance.read'] },
      {
        ...baseWargaUser,
        role: 'warga',
        jabatan: null,
        effectivePermissions: ['finance.read'],
        rwMemberships: [{ rw_id: 'rw-1', rw_name: 'RW 01', jabatan: 'ketua' }],
      },
    ]

    const apiSpy = vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: false,
      total_outstanding: 0,
      overdue_bill_count: 0,
    })

    for (const u of users) {
      mockAuth(u)

      const { unmount } = render(
        <MemoryRouter>
          <OverdueWarningCard />
        </MemoryRouter>,
      )
      expect(screen.queryByTestId('overdue-warning-card')).not.toBeInTheDocument()
      unmount()
    }

    expect(apiSpy).toHaveBeenCalledTimes(users.length)
  })

  it('6. navigates to /iuran?tab=tagihan when clicking Lihat Tagihan Saya', async () => {
    mockAuth(baseWargaUser)

    vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: true,
      total_outstanding: 150000,
      overdue_bill_count: 1,
    })

    render(
      <MemoryRouter initialEntries={['/kas']}>
        <OverdueWarningCard />
        <LocationDisplay />
      </MemoryRouter>,
    )

    const btn = await screen.findByRole('button', { name: /Lihat Tagihan Saya/i })
    fireEvent.click(btn)

    expect(screen.getByTestId('current-location').textContent).toBe('/iuran?tab=tagihan')
  })

  it('7. hides card automatically after payment update event resolves overdue balance', async () => {
    mockAuth(baseWargaUser)

    const apiSpy = vi
      .spyOn(ApiModule, 'apiGetOverdueWarningSummary')
      .mockResolvedValueOnce({
        has_overdue_3_months: true,
        total_outstanding: 300000,
        overdue_bill_count: 2,
      })
      .mockResolvedValueOnce({
        has_overdue_3_months: false,
        total_outstanding: 0,
        overdue_bill_count: 0,
      })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByTestId('overdue-warning-card')).toBeInTheDocument()
    })

    act(() => {
      ApiModule.notifyIuranDataUpdated()
    })

    await waitFor(() => {
      expect(screen.queryByTestId('overdue-warning-card')).not.toBeInTheDocument()
    })
    expect(apiSpy).toHaveBeenCalledTimes(2)
  })
})
