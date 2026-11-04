import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { RwDataView } from './RwDataView'
import * as AuthContext from '@/app/AuthContext'

vi.mock('./RwDashboard', () => ({
  RwDashboard: () => <div data-testid="rw-dashboard">Dashboard Component</div>,
}))
vi.mock('./RwKasList', () => ({
  RwKasList: () => <div data-testid="rw-kas">Kas Component</div>,
}))
vi.mock('./RwFundLocationList', () => ({
  RwFundLocationList: () => <div data-testid="rw-locations">Locations Component</div>,
}))

describe('RwDataView', () => {
  it('renders Keuangan RW title, correct subtitle, and eliminates aggregate disclaimer banner for managers', () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-bendahara',
        name: 'Bendahara RW 002',
        role: 'bendahara',
        rwMemberships: [
          { rw_id: 'rw-002', rw_name: 'RW 002 Urena', jabatan: 'bendahara', source: 'explicit' },
        ],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    render(
      <MemoryRouter initialEntries={['/rw-view/rw-002/dashboard']}>
        <Routes>
          <Route path="/rw-view/:rwId/*" element={<RwDataView />} />
        </Routes>
      </MemoryRouter>
    )

    // Title should be "Keuangan RW 002 Urena"
    expect(screen.getByText('Keuangan RW 002 Urena')).toBeInTheDocument()

    // Subtitle should be "Pencatatan kas dan lokasi dana mandiri tingkat RW"
    expect(
      screen.getByText('Pencatatan kas dan lokasi dana mandiri tingkat RW')
    ).toBeInTheDocument()

    // Aggregate warning box must NOT exist
    expect(screen.queryByText(/Anda sedang menampilkan data agregat untuk/i)).not.toBeInTheDocument()

    // Manager role should NOT see "Mode Read-Only" badge
    expect(screen.queryByText('Mode Read-Only')).not.toBeInTheDocument()

    // Dashboard tab content rendered
    expect(screen.getByTestId('rw-dashboard')).toBeInTheDocument()
  })

  it('renders Mode Read-Only badge for read-only role (Sekretaris RW)', () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-sekretaris',
        name: 'Sekretaris RW 002',
        role: 'sekretaris',
        rwMemberships: [
          { rw_id: 'rw-002', rw_name: 'RW 002 Urena', jabatan: 'sekretaris', source: 'explicit' },
        ],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    render(
      <MemoryRouter initialEntries={['/rw-view/rw-002/dashboard']}>
        <Routes>
          <Route path="/rw-view/:rwId/*" element={<RwDataView />} />
        </Routes>
      </MemoryRouter>
    )

    expect(screen.getByText('Keuangan RW 002 Urena')).toBeInTheDocument()
    // Read-only badge should appear for sekretaris
    expect(screen.getByText('Mode Read-Only')).toBeInTheDocument()
  })

  it('rejects access for non-manager (Warga) without RW position', () => {
    vi.spyOn(AuthContext, 'useAuth').mockReturnValue({
      user: {
        id: 'user-warga',
        name: 'Warga RW 002',
        role: 'warga',
        rwMemberships: [
          { rw_id: 'rw-002', rw_name: 'RW 002 Urena', jabatan: 'anggota', source: 'explicit' },
        ],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })

    render(
      <MemoryRouter initialEntries={['/rw-view/rw-002/dashboard']}>
        <Routes>
          <Route path="/rw-view/:rwId/*" element={<RwDataView />} />
        </Routes>
      </MemoryRouter>
    )

    // Warga without valid RW position must see access denied message
    expect(screen.getByText('Anda tidak memiliki akses ke RW mana pun')).toBeInTheDocument()
    expect(screen.queryByText('Keuangan RW 002 Urena')).not.toBeInTheDocument()
  })
})
