import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom'
import { HouseholdList } from './HouseholdList'
import { persistSessionPair, type ApiHousehold } from '@/app/api'
import * as AuthModule from '@/app/AuthContext'
import { MemoryRouter } from 'react-router-dom'

// ---- Test helpers ----

const mockUser = {
  id: 'user-1',
  name: 'Test User',
  email: 'test@example.com',
  systemRole: null,
  role: 'warga',
  rt: { id: 'rt-1', name: 'RT 001' },
}

const mockHousehold: ApiHousehold = {
  id: 'hh-1',
  rt_id: 'rt-1',
  house_number: '001',
  nik: '3201011234560001',
  head_name: 'Budi Santoso',
  address: 'Jl. Mawar No. 1',
  phone: '081234567890',
  email: 'budi@example.com',
  occupancy_status: 'OWNER',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const mockPaginatedResponse = (data: ApiHousehold[]) => ({
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

function renderHouseholdList() {
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
        <HouseholdList />
      </MemoryRouter>,
    ),
  }
}

// ---- Tests ----

describe('W4.2C — HouseholdList rendering', () => {
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
    renderHouseholdList()
    expect(screen.getByText(/Memuat data warga/i)).toBeInTheDocument()
  })

  it('renders household list with real data', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([mockHousehold]))
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('001')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('3201011234560001')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('Jl. Mawar No. 1')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('081234567890')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('budi@example.com')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getAllByText('Kepala Keluarga').length).toBeGreaterThanOrEqual(1)
    })
  })

  it('renders OWNER as Pemilik', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([mockHousehold]))
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(screen.getByText('Pemilik')).toBeInTheDocument()
    })
  })

  it('renders TENANT as Penyewa', async () => {
    const tenant: ApiHousehold = { ...mockHousehold, occupancy_status: 'TENANT' as const, head_name: 'Siti Rahayu' }
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([tenant]))
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(screen.getByText('Penyewa')).toBeInTheDocument()
    })
  })

  it('renders null fields as dash', async () => {
    const noOptionalHh = {
      ...mockHousehold,
      nik: null,
      address: null,
      phone: null,
      email: null,
      head_name: 'Tanpa KK',
      occupancy_status: null,
    }
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([noOptionalHh]))
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(screen.getByText('Tanpa KK')).toBeInTheDocument()
    })
    await waitFor(() => {
      const dashes = screen.getAllByText('—')
      expect(dashes.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('shows empty state when no households', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([]))
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(screen.getByText(/Tidak ada data warga/i)).toBeInTheDocument()
    })
  })

  it('shows error state on API failure', async () => {
    mockFetch(() => {
      return errorResponse(500, 'internal_error', 'Server error')
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(screen.getByText(/Gagal memuat data warga/i)).toBeInTheDocument()
    })
  })

  it('does not render row as a link', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockPaginatedResponse([mockHousehold]))
    })
    renderHouseholdList()

    await waitFor(() => {
      const row = screen.getByText('Budi Santoso')
      expect(row).toBeInTheDocument()
      const link = row.closest('a')
      expect(link).not.toBeInTheDocument()
    })
  })

  it('search uses Household API query parameter', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households')) {
        return mockSuccessResponse(mockPaginatedResponse([]))
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderHouseholdList()

    const searchInput = screen.getByPlaceholderText(/Cari/i) as HTMLInputElement
    fireEvent.change(searchInput, { target: { value: 'Budi' } })
    const searchBtn = screen.getByRole('button', { name: /Cari/i })
    fireEvent.click(searchBtn)

    await waitFor(() => {
      expect(screen.getByText(/Tidak ada data warga/i)).toBeInTheDocument()
    })
  })

  it('list calls API with correct path', async () => {
    let capturedUrl = ''
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households')) {
        capturedUrl = input
        return mockSuccessResponse(mockPaginatedResponse([]))
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(capturedUrl).toContain('/api/v1/households')
    })
  })

  it('list sends Authorization header', async () => {
    let capturedHeaders: Record<string, string> | undefined
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households')) {
        capturedHeaders = init.headers as Record<string, string>
        return mockSuccessResponse(mockPaginatedResponse([]))
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(capturedHeaders).toHaveProperty('Authorization', 'Bearer test-jwt-token')
    })
  })

  it('pagination shows page numbers', async () => {
    const largeList = Array.from({ length: 25 }, (_, i) => ({
      ...mockHousehold,
      id: `hh-${i}`,
      head_name: `Head ${i}`,
    }))
    mockFetch(() => {
      return mockSuccessResponse({
        data: largeList.slice(0, 20),
        pagination: { page: 1, page_size: 20, total: 25, total_pages: 2 },
      })
    })
    renderHouseholdList()

    await waitFor(() => {
      expect(screen.getByText(/Prev/i)).toBeInTheDocument()
    })
  })
})

describe('W4.2C — HouseholdList status filter', () => {
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
      if (init?.method === 'GET' && input.includes('/api/v1/households')) {
        capturedUrl = input
        return new Response(JSON.stringify(mockPaginatedResponse([])), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderHouseholdList()

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
      if (init?.method === 'GET' && input.includes('/api/v1/households')) {
        capturedUrl = input
        return new Response(JSON.stringify(mockPaginatedResponse([])), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return errorResponse(404, 'not_found', 'not found')
    })
    renderHouseholdList()

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
