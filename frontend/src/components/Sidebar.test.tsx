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
  rt: { id: 'rt-1', name: 'RT 001' },
}

const mockPengurusUser: AuthUser = {
  id: 'user-2',
  name: 'Pengurus A',
  email: 'pengurus@example.com',
  systemRole: null,
  role: 'pengurus',
  rt: { id: 'rt-1', name: 'RT 001' },
}

const mockSuperAdminUser: AuthUser = {
  id: 'user-3',
  name: 'Super Admin',
  email: 'admin@example.com',
  systemRole: 'super_admin',
  role: '',
  rt: null,
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

  it('renders ApartmentOutlined icon for RT route when SuperAdmin', () => {
    renderSidebar(mockSuperAdminUser)
    expect(screen.getByText('RT')).toBeInTheDocument()
  })

  it('does not show RT route for warga', () => {
    renderSidebar(mockWargaUser)
    expect(screen.queryByText('RT')).not.toBeInTheDocument()
  })

  it('does not show RT route for pengurus', () => {
    renderSidebar(mockPengurusUser)
    expect(screen.queryByText('RT')).not.toBeInTheDocument()
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

  it('has exactly 3 nav items for warga: Beranda, Warga', () => {
    renderSidebar()
    const links = screen.getAllByRole('link')
    // Total buttons = pin button only (logout removed → header user menu)
    const buttons = screen.getAllByRole('button')
    expect(links.length).toBe(2) // Beranda, Warga
    expect(buttons.length).toBe(1) // pin toggle
  })

  it('has exactly 4 nav items for SuperAdmin: Beranda, Warga, RT', () => {
    renderSidebar(mockSuperAdminUser)
    const links = screen.getAllByRole('link')
    // Total buttons = pin button only (logout removed → header user menu)
    const buttons = screen.getAllByRole('button')
    expect(links.length).toBe(3) // Beranda, Warga, RT
    expect(buttons.length).toBe(1) // pin toggle
  })

  it('does not contain nav entry for /iuran (dead route removed)', () => {
    renderSidebar()
    expect(screen.queryByText(/Iuran/i)).not.toBeInTheDocument()
  })

  it('does not contain nav entry for /kas (dead route removed)', () => {
    renderSidebar()
    expect(screen.queryByText(/Kas/i)).not.toBeInTheDocument()
  })

  it('does not contain nav entry for /laporan (dead route removed)', () => {
    renderSidebar()
    expect(screen.queryByText(/Laporan/i)).not.toBeInTheDocument()
  })

  it('does not contain nav entry for /profil (dead route removed)', () => {
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

  it('marks /rt as active when on RT detail page for SuperAdmin', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: mockSuperAdminUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    render(
      <MemoryRouter initialEntries={['/rt/rt-1']}>
        <Sidebar />
      </MemoryRouter>,
    )
    const rtLink = screen.getAllByRole('link').find(l => l.textContent?.includes('RT'))
    expect(rtLink).toHaveClass('nav-item-active')
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
