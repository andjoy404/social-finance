import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { HouseholdDetail } from './HouseholdDetail'
import { persistSessionPair, type ApiHousehold } from '@/app/api'
import * as AuthModule from '@/app/AuthContext'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

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

function mockFetch(
  fn: (input: string, init?: RequestInit) => Response | Promise<Response>,
) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(
    async (input, init) => fn(typeof input === 'string' ? input : '', init),
  )
}

function renderHouseholdDetail(householdId: string = 'hh-1') {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: mockUser,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={[`/warga/${householdId}`]}>
        <Routes>
          <Route path="/warga/:id" element={<HouseholdDetail />} />
        </Routes>
      </MemoryRouter>,
    ),
  }
}

// ---- Tests ----

describe('W4.2C — HouseholdDetail rendering', () => {
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
    renderHouseholdDetail()
    expect(screen.getByText(/Memuat detail warga/i)).toBeInTheDocument()
  })

  it('renders all household fields correctly', async () => {
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-1')) {
        return new Response(JSON.stringify(mockHousehold), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByText('001')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.queryByText('3201011234560001')).not.toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('Jl. Mawar No. 1')).toBeInTheDocument()
    })
    await waitFor(() => {
      expect(screen.getByText('081234567890')).toBeInTheDocument()
    })
    await waitFor(() => {
      const label = screen.getByText('Kepala Keluarga')
      const container = label.parentElement
      expect(container?.querySelector('div:nth-child(2)')?.textContent).toBe('Budi Santoso')
    })
    await waitFor(() => {
      const label = screen.getByText('Status Hunian')
      const container = label.parentElement
      expect(container?.querySelector('div:nth-child(2)')?.textContent).toBe('Pemilik')
    })
  })

  it('renders null fields as dash', async () => {
    const noOptionalHh = {
      ...mockHousehold,
      nik: null,
      address: null,
      phone: null,
      email: null,
      occupancy_status: null,
    }
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-1')) {
        return new Response(JSON.stringify(noOptionalHh), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      const label = screen.getByText('Telepon')
      const container = label.parentElement
      expect(container?.querySelector('div:nth-child(2)')?.textContent).toBe('\u2014')
    })
  })

  it('shows 404 when household not found', async () => {
    mockFetch(() => {
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByText(/Warga tidak ditemukan/i)).toBeInTheDocument()
    })
  })

  it('shows error message on API failure', async () => {
    mockFetch(() => {
      return new Response(JSON.stringify({ code: 'internal_error', message: 'Server error' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByText('Gagal memuat detail warga.')).toBeInTheDocument()
    })
  })

  it('navigates back with back button', async () => {
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-1')) {
        return new Response(JSON.stringify(mockHousehold), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('link', { name: /Kembali ke Daftar Warga/i })).toBeInTheDocument()
    })

    const backLink = screen.getByRole('link', { name: /Kembali/i })
    expect(backLink).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Kembali/i })).toHaveAttribute('href', '/warga')
  })

  it('renders OWNER as Pemilik', async () => {
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-2')) {
        return new Response(JSON.stringify(mockHousehold), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail('hh-2')

    await waitFor(() => {
      const label = screen.getByText('Status Hunian')
      const container = label.parentElement
      expect(container).toBeTruthy()
      const valueDiv = container?.querySelector('div:nth-child(2)')
      expect(valueDiv?.textContent).toBe('Pemilik')
    })
  })

  it('renders TENANT as Penyewa', async () => {
    const tenant = { ...mockHousehold, occupancy_status: 'TENANT' as const }
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-3')) {
        return new Response(JSON.stringify(tenant), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail('hh-3')

    await waitFor(() => {
      const label = screen.getByText('Status Hunian')
      const container = label.parentElement
      expect(container?.querySelector('div:nth-child(2)')?.textContent).toBe('Penyewa')
    })
  })

  it('calls API with correct path for household ID from route', async () => {
    let capturedId = ''
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-custom')) {
        capturedId = 'hh-custom'
        return new Response(JSON.stringify(mockHousehold), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail('hh-custom')

    await waitFor(() => {
      expect(capturedId).toBe('hh-custom')
    })
  })

  it('sends Authorization header with request', async () => {
    let capturedHeaders: Record<string, string> | undefined
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-1')) {
        capturedHeaders = init.headers as Record<string, string>
        return new Response(JSON.stringify(mockHousehold), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(capturedHeaders).toHaveProperty('Authorization', 'Bearer test-jwt-token')
    })
  })

  it('shows created_at and updated_at dates', async () => {
    mockFetch((_, init) => {
      if (init?.method === 'GET' && _.includes('/api/v1/households/hh-1')) {
        return new Response(JSON.stringify(mockHousehold), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ code: 'not_found' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByText(/Dibuat/i)).toBeInTheDocument()
      expect(screen.getByText(/Diperbarui/i)).toBeInTheDocument()
    })
  })
})
