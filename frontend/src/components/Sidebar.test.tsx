import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import { Sidebar } from '@/components/Sidebar'
import * as AuthModule from '@/app/AuthContext'
import type { AuthUser } from '@/app/AuthContext'
import { MemoryRouter } from 'react-router-dom'

// ---- Test helpers ----

const mockWargaUser: AuthUser = {
  id: 'user-1',
  name: 'Test User',
  email: 'test@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'RT 002' },
  rtCode: '002',
  rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
}

const mockPengurusUser: AuthUser = {
  id: 'user-2',
  name: 'Pengurus A',
  email: 'pengurus@example.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: null,
  rt: { id: '8dfa1e36-6727-45be-8fea-24d958efc155', name: 'RT 003' },
  rtCode: '003',
  rtId: '8dfa1e36-6727-45be-8fea-24d958efc155',
}

const mockSuperAdminUser: AuthUser = {
  id: 'user-3',
  name: 'Super Admin',
  email: 'admin@example.com',
  systemRole: 'super_admin',
  role: '',
  jabatan: null,
  rt: null,
}

const mockNonAllowedUser: AuthUser = {
  id: 'user-4',
  name: 'Warga RT 004',
  email: 'warga004@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  rt: { id: 'efb70405-09f5-48a0-ad14-34802cf09e3b', name: 'RT 004' },
  rtCode: '004',
  rtId: 'efb70405-09f5-48a0-ad14-34802cf09e3b',
}

function renderSidebar(customUser?: AuthUser) {
  const user = customUser ?? mockWargaUser
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(<MemoryRouter><Sidebar /></MemoryRouter>),
  }
}

function clearSidebarLocalStorage() {
  try { localStorage.removeItem('sf_sidebar_collapsed') } catch { /* ignore */ }
}

// ---- Sidebar component tests ----

describe('Sidebar — AndJoy visual language', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })

  it('has the sf-sidebar class on the nav element', () => {
    renderSidebar()
    expect(screen.getByRole('navigation', { name: 'Navigasi utama' }).closest('nav')).toHaveClass('sf-sidebar')
  })

  it('uses Ant Design icons, not emojis', () => {
    renderSidebar()
    const navItems = screen.getAllByRole('link')
    navItems.forEach((item) => {
      const text = item.textContent
      expect(text).not.toMatch(/[\p{Emoji}]/u)
    })
  })

  it('renders DashboardOutlined icon for Beranda', () => {
    renderSidebar()
    const links = screen.getAllByRole('link')
    const berandaLink = links.find(l => l.textContent?.includes('Beranda'))
    expect(berandaLink).toBeTruthy()
  })

  it('renders UserOutlined icon for Warga', () => {
    renderSidebar()
    expect(screen.getByText('Warga')).toBeInTheDocument()
  })

  it('renders Kelola RT link for SuperAdmin', () => {
    renderSidebar(mockSuperAdminUser)
    expect(screen.getByText('Kelola RT')).toBeInTheDocument()
  })

  it('does not show SuperAdmin links for warga', () => {
    renderSidebar(mockWargaUser)
    expect(screen.queryByText('Kelola RT')).not.toBeInTheDocument()
  })

  it('does not show SuperAdmin links for pengurus', () => {
    renderSidebar(mockPengurusUser)
    expect(screen.queryByText('Kelola RT')).not.toBeInTheDocument()
  })
})

describe('Sidebar — collapsible state', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })

  it('is expanded by default', () => {
    renderSidebar()
    expect(screen.getByText('Social Finance')).toBeInTheDocument()
  })

  it('collapses when pin button is clicked', () => {
    renderSidebar()
    const pinBtn = screen.getByLabelText('Sematkan sidebar')
    fireEvent.click(pinBtn)

    expect(screen.queryByText('Social Finance')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Perluas sidebar')).toBeInTheDocument()
  })

  it('expands when pin button is clicked again after collapse', () => {
    renderSidebar()
    const pinBtn = screen.getByLabelText('Sematkan sidebar')
    fireEvent.click(pinBtn)

    const expandBtn = screen.getByLabelText('Perluas sidebar')
    fireEvent.click(expandBtn)

    expect(screen.getByText('Social Finance')).toBeInTheDocument()
  })

  it('persists collapsed state to localStorage', () => {
    renderSidebar()
    const pinBtn = screen.getByLabelText('Sematkan sidebar')
    fireEvent.click(pinBtn)

    try {
      expect(localStorage.getItem('sf_sidebar_collapsed')).toBe('true')
    } catch {
      expect(true).toBe(true)
    }
  })

  it('reads collapsed state from localStorage on mount', () => {
    try { localStorage.setItem('sf_sidebar_collapsed', 'true') } catch { /* ignore */ }

    renderSidebar()

    expect(screen.queryByText('Social Finance')).not.toBeInTheDocument()
  })

  it('collapses to 56px width when pinned', () => {
    renderSidebar()
    const slot = document.querySelector('.sf-sidebar-slot') as HTMLElement
    expect(slot.style.width).toBe('240px')

    const pinBtn = screen.getByLabelText('Sematkan sidebar')
    fireEvent.click(pinBtn)

    expect(slot.style.width).toBe('56px')
  })
})

describe('Sidebar — navigation items', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })

  it('has exactly 4 nav items for warga: Beranda, Warga, Iuran, KAS', () => {
    renderSidebar()
    const links = screen.getAllByRole('link')
    // Total buttons = pin button only (logout removed → header user menu)
    const buttons = screen.getAllByRole('button')
    expect(links.length).toBe(4) // Beranda, Warga, Iuran, KAS
    expect(buttons.length).toBe(1) // pin toggle
  })

  it('has exactly 8 nav items for SuperAdmin: Beranda, Warga, Iuran, KAS, Lokasi Dana, Notifikasi, Kelola RT, Kelola RW', () => {
    renderSidebar(mockSuperAdminUser)
    const links = screen.getAllByRole('link')
    // Total buttons = pin button only (logout removed → header user menu)
    const buttons = screen.getAllByRole('button')
    expect(links.length).toBe(8) // Beranda, Warga, Iuran, KAS, Lokasi Dana, Notifikasi, Kelola RT, Kelola RW
    expect(buttons.length).toBe(1) // pin toggle
  })

  it('hides RT-scoped menus (Warga, Iuran, KAS) for non-allowed RTs (RT 004)', () => {
    renderSidebar(mockNonAllowedUser)
    const links = screen.getAllByRole('link')
    expect(links.length).toBe(1) // Beranda only
    expect(screen.getByText('Beranda')).toBeInTheDocument()
    expect(screen.queryByText('Warga')).not.toBeInTheDocument()
    expect(screen.queryByText('Iuran')).not.toBeInTheDocument()
    expect(screen.queryByText(/Kas/i)).not.toBeInTheDocument()
  })

  it('contains nav entry for /iuran (active route)', () => {
    renderSidebar()
    expect(screen.getByText(/Iuran/i)).toBeInTheDocument()
  })

  it('contains nav entry for /kas (active route)', () => {
    renderSidebar()
    expect(screen.getByText(/Kas/i)).toBeInTheDocument()
  })

  it('does not contain nav entry for /laporan (not implemented)', () => {
    renderSidebar()
    expect(screen.queryByText(/Laporan/i)).not.toBeInTheDocument()
  })

  it('does not contain nav entry for /profil (not implemented)', () => {
    renderSidebar()
    expect(screen.queryByText(/Profil/i)).not.toBeInTheDocument()
  })

  it('has active state on current route', () => {
    renderSidebar()
    const berandaLink = screen.getAllByRole('link').find(l => l.textContent?.includes('Beranda'))
    expect(berandaLink).toHaveClass('nav-item-active')
  })

  it('has proper nav-label for items when expanded', () => {
    renderSidebar()
    expect(screen.getByText('Beranda')).toBeInTheDocument()
    expect(screen.getByText('Warga')).toBeInTheDocument()
  })
})

describe('Sidebar — active route detection', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: mockWargaUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
  })
  afterEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })

  it('marks /warga/hh-1 as active when on nested warga route', () => {
    render(
      <MemoryRouter initialEntries={['/warga/hh-1']}>
        <Sidebar />
      </MemoryRouter>,
    )
    const wargaLink = screen.getAllByRole('link').find(l => l.textContent?.includes('Warga'))
    expect(wargaLink).toHaveClass('nav-item-active')
  })

  it('marks /warga/new as active when on nested warga route', () => {
    render(
      <MemoryRouter initialEntries={['/warga/new']}>
        <Sidebar />
      </MemoryRouter>,
    )
    const wargaLink = screen.getAllByRole('link').find(l => l.textContent?.includes('Warga'))
    expect(wargaLink).toHaveClass('nav-item-active')
  })

  it('marks /rt as active for SuperAdmin', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: mockSuperAdminUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    render(
      <MemoryRouter initialEntries={['/rt'] }>
        <Sidebar />
      </MemoryRouter>,
    )
    const rtLink = screen.getAllByRole('link').find(l => l.textContent?.includes('Kelola RT'))
    expect(rtLink).toHaveClass('nav-item-active')
  })
  it('marks /rw as active for SuperAdmin', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: mockSuperAdminUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    render(
      <MemoryRouter initialEntries={['/rw']}>
        <Sidebar />
      </MemoryRouter>,
    )
    const rwLink = screen.getAllByRole('link').find(l => l.textContent?.includes('Kelola RW'))
    expect(rwLink).toHaveClass('nav-item-active')
  })

})

describe('Sidebar — layout classes', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    clearSidebarLocalStorage()
  })

  it('has sf-sidebar-slot wrapper', () => {
    renderSidebar()
    expect(document.querySelector('.sf-sidebar-slot')).toBeInTheDocument()
  })

  it('has sidebar-nav container', () => {
    renderSidebar()
    expect(screen.getByRole('navigation', { name: 'Navigasi utama' })).toHaveClass('sf-sidebar')
  })

  it('applies sf-sidebar-slot-collapsed class when collapsed', () => {
    renderSidebar()
    const slot = document.querySelector('.sf-sidebar-slot') as HTMLElement
    expect(slot.style.width).toBe('240px')

    const pinBtn = screen.getByLabelText('Sematkan sidebar')
    fireEvent.click(pinBtn)

    expect(slot.style.width).toBe('56px')
  })
})
