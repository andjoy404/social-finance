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
  effectivePermissions: ['finance.read'],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
  rwId: 'rw-001',
  rwMemberships: [],
}

describe('OverdueWarningCard (Web Global Warning Card)', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(ApiModule, 'getSessionPair').mockReturnValue({
      accessToken: 'test-access-token',
      refreshToken: 'test-refresh-token',
    })
  })

  it('1. renders warning card for plain warga with >= 3 months overdue bills', async () => {
    vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
      user: baseWargaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as unknown as ReturnType<typeof AuthContextModule.useAuth>)

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

  it('2 & 3. does not render for warga with no overdue or < 3 months overdue', async () => {
    vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
      user: baseWargaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as unknown as ReturnType<typeof AuthContextModule.useAuth>)

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

  it('4a. renders RT-wide card for sekretaris (jabatan + finance.read) with RT summary', async () => {
    const sekretarisUser: AuthContextModule.AuthUser = {
      ...baseWargaUser,
      role: 'warga',
      jabatan: 'sekretaris',
      effectivePermissions: ['finance.read', 'finance.create', 'organization.contact.read'],
      rtId: 'rt-002',
      rtCode: '002',
    }
    vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
      user: sekretarisUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as unknown as ReturnType<typeof AuthContextModule.useAuth>)

    vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary').mockResolvedValue({
      has_overdue_3_months: true,
      total_outstanding: 3900000,
      overdue_bill_count: 39,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByTestId('overdue-warning-card')).toBeInTheDocument()
    })
    expect(screen.getByText(/Peringatan Tunggakan Iuran Warga/)).toBeInTheDocument()
    expect(
      screen.getByText(/Terdapat tagihan wajib iuran warga di RT Anda/i),
    ).toBeInTheDocument()
    expect(screen.getByText(/3\.900\.000/)).toBeInTheDocument()
    expect(screen.getByText('39 tagihan')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Lihat Tunggakan Warga/i })).toBeInTheDocument()
  })

  it('4b. does not render and does not fetch for pengurus without finance.read, RW-only ketua, or Super Admin', () => {
    const noAccessUsers: AuthContextModule.AuthUser[] = [
      { ...baseWargaUser, role: 'pengurus', jabatan: null, effectivePermissions: ['warga.read'] },
      { ...baseWargaUser, role: 'warga', jabatan: 'ketua', effectivePermissions: ['organization.read'] },
      { ...baseWargaUser, role: 'warga', jabatan: 'bendahara', effectivePermissions: [] },
      { ...baseWargaUser, systemRole: 'super_admin', role: 'super_admin', effectivePermissions: ['finance.read'] },
      {
        ...baseWargaUser,
        role: 'warga',
        jabatan: null,
        effectivePermissions: ['organization.read'],
        rwMemberships: [{ rw_id: 'rw-1', rw_name: 'RW 01', jabatan: 'ketua' }],
      },
    ]

    const apiSpy = vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary')

    for (const u of noAccessUsers) {
      vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
        user: u,
        loading: false,
        login: vi.fn(),
        logout: vi.fn(),
        refreshUser: vi.fn(),
      } as unknown as ReturnType<typeof AuthContextModule.useAuth>)

      const { unmount } = render(
        <MemoryRouter>
          <OverdueWarningCard />
        </MemoryRouter>,
      )
      expect(screen.queryByTestId('overdue-warning-card')).not.toBeInTheDocument()
      unmount()
    }

    expect(apiSpy).not.toHaveBeenCalled()
  })

  it('4c. does not render and does not fetch for role-only pengurus without explicit jabatan/permission', () => {
    const roleOnlyPengurus: AuthContextModule.AuthUser = {
      ...baseWargaUser,
      jabatan: null,
      role: 'pengurus',
      effectivePermissions: ['warga.read'],
    }
    vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
      user: roleOnlyPengurus,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as unknown as ReturnType<typeof AuthContextModule.useAuth>)

    const apiSpy = vi.spyOn(ApiModule, 'apiGetOverdueWarningSummary')

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <OverdueWarningCard />
      </MemoryRouter>,
    )

    expect(screen.queryByTestId('overdue-warning-card')).not.toBeInTheDocument()
    expect(apiSpy).not.toHaveBeenCalled()
  })

  it('6. navigates to /iuran?tab=tagihan when clicking Lihat Tagihan Saya', async () => {
    vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
      user: baseWargaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as unknown as ReturnType<typeof AuthContextModule.useAuth>)

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
    vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
      user: baseWargaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as unknown as ReturnType<typeof AuthContextModule.useAuth>)

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
