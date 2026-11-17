import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as AuthModule from '@/app/AuthContext'
import type { AuthUser } from '@/app/AuthContext'
import { App } from './App'

vi.mock('@/layouts/AppLayout', async () => {
  const { Outlet } = await import('react-router-dom')
  const { Sidebar } = await import('@/components/Sidebar')
  return {
    AppLayout: () => (
      <>
        <Sidebar />
        <Outlet />
      </>
    ),
  }
})

vi.mock('@/features/notification/SecurityWasteTruckScreen', () => ({
  SecurityWasteTruckScreen: ({ canDispatch }: { canDispatch: boolean }) => (
    <div>
      Notifikasi Keamanan
      {!canDispatch && <span>Akses hanya-baca</span>}
    </div>
  ),
}))

vi.mock('@/features/notification/NotificationScreen', () => ({
  NotificationScreen: () => <div>Riwayat Notifikasi</div>,
}))

vi.mock('@/features/dashboard/Dashboard', () => ({
  Dashboard: () => <div>Beranda</div>,
}))

vi.mock('@/features/warga/WargaLayout', () => ({
  WargaLayout: () => <div>Halaman Warga</div>,
}))

const securityUser: AuthUser = {
  id: 'security-1',
  name: 'Petugas Keamanan',
  email: 'security@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: 'keamanan',
  effectivePermissions: ['notification.send_waste_truck'],
  rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'Urena' },
  rtCode: '002',
  rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
}

const wargaUser: AuthUser = {
  id: 'warga-1',
  name: 'Warga RT 002',
  email: 'warga@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'Urena' },
  rtCode: '002',
  rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
}

const pengurusUser: AuthUser = {
  id: 'pengurus-1',
  name: 'Pengurus RT',
  email: 'pengurus@example.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'sekretaris',
  effectivePermissions: [],
  rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'Urena' },
  rtCode: '002',
  rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
}

function renderApp(path: string, user: AuthUser = securityUser) {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

describe('Notification route access', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('navigates from the security officer menu to the Security notification page', async () => {
    renderApp('/warga')
    fireEvent.click(screen.getByRole('link', { name: /Notifikasi/ }))
    expect(await screen.findByText('Notifikasi Keamanan')).toBeInTheDocument()
    expect(screen.queryByText('Beranda')).not.toBeInTheDocument()
  })

  it('keeps a direct notification URL on the Security page after session restoration', async () => {
    const useAuth = vi.spyOn(AuthModule, 'useAuth')
    useAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isInitializing: true,
      login: async () => {},
      logout: () => {},
    })
    const view = render(
      <MemoryRouter initialEntries={['/notifikasi']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.queryByText('Beranda')).not.toBeInTheDocument()

    useAuth.mockReturnValue({
      user: securityUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    view.rerender(
      <MemoryRouter initialEntries={['/notifikasi']}>
        <App />
      </MemoryRouter>,
    )

    expect(await screen.findByText('Notifikasi Keamanan')).toBeInTheDocument()
  })

  it('allows Security to read history without granting dispatch permission', async () => {
    renderApp('/notifikasi', { ...securityUser, effectivePermissions: [] })
    expect(await screen.findByText('Notifikasi Keamanan')).toBeInTheDocument()
    expect(screen.getByText('Akses hanya-baca')).toBeInTheDocument()
    expect(screen.queryByText('Beranda')).not.toBeInTheDocument()
  })

  it('does not grant the Security feature to a warga without the Security position', async () => {
    renderApp('/notifikasi', { ...securityUser, jabatan: null })
    expect(await screen.findByText('Riwayat Notifikasi')).toBeInTheDocument()
    expect(screen.queryByText('Notifikasi Keamanan')).not.toBeInTheDocument()
  })

  it('allows ordinary warga to open the personal notification history page', async () => {
    renderApp('/notifikasi', wargaUser)
    expect(await screen.findByText('Riwayat Notifikasi')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Notifikasi/ })).toBeInTheDocument()
  })

  it('allows Pengurus without notification.manage to open personal history only', async () => {
    renderApp('/notifikasi', pengurusUser)
    expect(await screen.findByText('Riwayat Notifikasi')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Notifikasi/ })).toBeInTheDocument()
    expect(screen.queryByText('Notifikasi Keamanan')).not.toBeInTheDocument()
  })
})
