import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom'
import { ResidentList } from './ResidentList'
import { persistSessionPair, type ApiResident } from '@/app/api'
import * as AuthModule from '@/app/AuthContext'
import { MemoryRouter, useLocation } from 'react-router-dom'

// ---- Test helpers ----

const mockUser = {
  id: 'user-1',
  name: 'Test User',
  email: 'test@example.com',
  systemRole: null,
  role: 'pengurus',
  rt: { id: 'rt-1', name: 'RT 001' },
}

const mockResident: ApiResident = {
  id: 'res-1',
  rt_id: 'rt-1',
  household_id: 'hh-1',
  full_name: 'Budi Santoso',
  nik: '3201011234560001',
  phone: '081234567890',
  email: 'budi@example.com',
  relationship_to_head: 'HEAD',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const mockPaginatedResponse = (data: ApiResident[]) => ({
  data,
  pagination: { page: 1, page_size: 20, total: data.length, total_pages: 1 },
})

function mockFetch(
  fn: (input: string, init?: RequestInit) => Response | Promise<Response>,
) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(
    async (input, init) => fn(typeof input === 'string' ? input : '', init),
  )
}

function mockSuccessResponse(data: unknown): Response {
  return new Response(JSON.stringify(data), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function errorResponse(status: number, code: string, message: string): Response {
  return new Response(JSON.stringify({ code, message }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function renderResidentList() {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: mockUser,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={['/warga']}>
        <ResidentList />
      </MemoryRouter>,
    ),
  }
}

// ---- Tests ----

describe('W4.2C — ResidentList rendering', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows loading state on initial render', () => {
    const promise = new Promise<Response>(() => {})
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => promise)
    renderResidentList()
    expect(screen.getByText(/Memuat data warga/i)).toBeInTheDocument()
  })

  it('renders resident list with real data', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([mockResident]))
    })
    renderResidentList()

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('3201011234560001')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('081234567890')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('budi@example.com')).toBeInTheDocument()
    })
  })

  it('renders null fields as dash', async () => {
    const noOptional = {
      ...mockResident,
      nik: null,
      phone: null,
      email: null,
      full_name: 'Tanpa Data',
      relationship_to_head: null,
    }
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([noOptional]))
    })
    renderResidentList()

    await waitFor(() => {
      expect(screen.getByText('Tanpa Data')).toBeInTheDocument()
    })
    await waitFor(() => {
      const dashes = screen.getAllByText('—')
      expect(dashes.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('shows empty state when no residents', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([]))
    })
    renderResidentList()

    await waitFor(() => {
      expect(screen.getByText(/Tidak ada data warga/i)).toBeInTheDocument()
    })
  })

  it('shows error state on API failure', async () => {
    mockFetch(() => {
      return errorResponse(500, 'internal_error', 'Server error')
    })
    renderResidentList()

    await waitFor(() => {
      expect(screen.getByText(/Gagal memuat data warga/i)).toBeInTheDocument()
    })
  })

  it('list calls API with correct path', async () => {
    let capturedUrl = ''
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/residents')) {
        capturedUrl = input
        return mockSuccessResponse(mockPaginatedResponse([]))
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderResidentList()

    await waitFor(() => {
      expect(capturedUrl).toContain('/api/v1/residents')
    })
  })

  it('list sends Authorization header', async () => {
    let capturedHeaders: Record<string, string> | undefined
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/residents')) {
        capturedHeaders = init.headers as Record<string, string>
        return mockSuccessResponse(mockPaginatedResponse([]))
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderResidentList()

    await waitFor(() => {
      expect(capturedHeaders).toHaveProperty('Authorization', 'Bearer test-jwt-token')
    })
  })

  it('pagination shows page numbers', async () => {
    const largeList = Array.from({ length: 25 }, (_, i) => ({
      ...mockResident,
      id: `res-${i}`,
      full_name: `Resident ${i}`,
    }))
    mockFetch(() => {
      return mockSuccessResponse({
        data: largeList.slice(0, 20),
        pagination: { page: 1, page_size: 20, total: 25, total_pages: 2 },
      })
    })
    renderResidentList()

    await waitFor(() => {
      expect(screen.getByText(/Prev/i)).toBeInTheDocument()
    })
  })

  it('renders position resident naturally from API data', async () => {
    const positionResident: ApiResident = {
      ...mockResident,
      id: 'position-1',
      full_name: 'Position Ketua RT 03',
      nik: '9999999999990001',
      phone: null,
      email: 'position.ketua.rt03@example.com',
      relationship_to_head: 'HEAD',
      is_active: true,
      rt_number: '03',
      rw: 16,
      rt_name: 'Wisma Rukun Tunggal',
    }
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([positionResident]))
    })
    renderResidentList()

    await waitFor(() => {
      expect(screen.getByText('Position Ketua RT 03')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('9999999999990001')).toBeInTheDocument()
    })
  })
})

describe('W4.2C — ResidentList status filter', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('includes is_active=query when selecting Status then Aktif', async () => {
    let capturedUrl = ''
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/residents')) {
        capturedUrl = input
        return new Response(JSON.stringify(mockPaginatedResponse([])), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderResidentList()

    // First, open the type filter dropdown and select "Status"
    const filterTrigger = screen.getByRole('button', { name: /Tipe filter/i })
    fireEvent.click(filterTrigger)
    const statusOption = screen.getByRole('button', { name: 'Status' })
    fireEvent.click(statusOption)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Tipe filter: Status/i })).toBeInTheDocument()
    })

    // Wait for status control to appear, then find the status selector trigger
    await waitFor(() => {
      const selector = screen.getByRole('button', { name: /Pilih Status/i })
      expect(selector).toBeInTheDocument()
    })

    const selectorTrigger = screen.getByRole('button', { name: /Pilih Status/i })
    fireEvent.click(selectorTrigger)

    const aktifBtn = screen.getByRole('button', { name: 'Aktif' })
    fireEvent.click(aktifBtn)

    await waitFor(() => {
      expect(capturedUrl).toContain('is_active=true')
    })
  })

  it('includes is_active=false when selecting Status then Tidak Aktif', async () => {
    let capturedUrl = ''
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/residents')) {
        capturedUrl = input
        return new Response(JSON.stringify(mockPaginatedResponse([])), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderResidentList()

    // Select "Status" from type filter
    const filterTrigger = screen.getByRole('button', { name: /Tipe filter/i })
    fireEvent.click(filterTrigger)
    const statusOption = screen.getByRole('button', { name: 'Status' })
    fireEvent.click(statusOption)

    // Open status selector
    const selectorTrigger = screen.getByRole('button', { name: /Pilih Status/i })
    fireEvent.click(selectorTrigger)

    const tidakAktifBtn = screen.getByRole('button', { name: /Tidak Aktif/i })
    fireEvent.click(tidakAktifBtn)

    await waitFor(() => {
      expect(capturedUrl).toContain('is_active=false')
    })
  })
})

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname}</div>
}

function renderResidentListWithRouter() {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: mockUser,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={['/warga']}>
        <ResidentList />
        <LocationProbe />
      </MemoryRouter>,
    ),
  }
}

describe('W4.2C — Aksi row navigation', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('Detail action navigates to /warga/<resident-id>', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([mockResident]))
    })
    renderResidentListWithRouter()

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })

    // Open Aksi menu
    const aksiBtn = screen.getByRole('button', { name: /Aksi/i })
    fireEvent.click(aksiBtn)

    // Click Detail
    const detailLink = screen.getByRole('menuitem', { name: 'Detail' })
    fireEvent.click(detailLink)

    // Verify navigation to resident detail page
    await waitFor(() => {
      expect(screen.getByTestId('location')).toHaveTextContent('/warga/res-1')
    })
  })

  it('Ubah action navigates to /warga/<household-id>/edit', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([mockResident]))
    })
    renderResidentListWithRouter()

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })

    // Open Aksi menu
    const aksiBtn = screen.getByRole('button', { name: /Aksi/i })
    fireEvent.click(aksiBtn)

    // Click Ubah
    const ubahLink = screen.getByRole('menuitem', { name: 'Ubah' })
    fireEvent.click(ubahLink)

    // Verify navigation to edit page
    await waitFor(() => {
      expect(screen.getByTestId('location')).toHaveTextContent('/warga/hh-1/edit')
    })
  })
})
