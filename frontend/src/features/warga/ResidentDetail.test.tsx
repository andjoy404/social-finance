import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom'
import { ResidentDetail } from './ResidentDetail'
import { persistSessionPair, type ApiResident } from '@/app/api'
import * as AuthModule from '@/app/AuthContext'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import type { AuthUser } from '@/app/AuthContext'

// ---- Test helpers ----

const mockPengurusUser = {
  id: 'user-1',
  name: 'Test Pengurus',
  email: 'pengurus@example.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: null,
  rt: { id: 'rt-1', name: 'RT 001' },
}

const mockWargaUser = {
  id: 'user-2',
  name: 'Test Warga',
  email: 'warga@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
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

function renderResidentDetail(user: AuthUser) {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={['/warga/res-1']}>
        <Routes>
          <Route path="/warga/:id" element={<ResidentDetail />} />
        </Routes>
      </MemoryRouter>,
    ),
  }
}

// ---- Tests ----

describe('ResidentDetail — loading/error/not-found', () => {
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
    renderResidentDetail(mockPengurusUser)
    expect(screen.getByText(/Memuat detail warga/i)).toBeInTheDocument()
  })

  it('shows error state on API failure', async () => {
    mockFetch(() => {
      return errorResponse(500, 'internal_error', 'Server error')
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText(/Gagal memuat detail warga/i)).toBeInTheDocument()
    })
  })

  it('shows not-found state when resident does not exist', async () => {
    mockFetch(() => {
      return errorResponse(404, 'not_found', 'Resident not found')
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText(/Warga tidak ditemukan/i)).toBeInTheDocument()
    })
  })
})

describe('ResidentDetail — renders resident information', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders resident full name', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Budi Santoso')
    })
  })

  it('renders NIK for pengurus role', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('3201011234560001')).toBeInTheDocument()
    })
  })

  it('renders NIK for super_admin role', async () => {
    const superAdminUser = { ...mockPengurusUser, systemRole: 'super_admin', name: 'Super Admin' }
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(superAdminUser)

    await waitFor(() => {
      expect(screen.getByText('3201011234560001')).toBeInTheDocument()
    })
  })

  it('hides NIK for warga role', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockWargaUser)

    await waitFor(() => {
      expect(screen.queryByText('3201011234560001')).not.toBeInTheDocument()
    })
  })

  it('renders phone number', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('081234567890')).toBeInTheDocument()
    })
  })

  it('renders email', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('budi@example.com')).toBeInTheDocument()
    })
  })

  it('renders active status as Aktif', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Aktif')).toBeInTheDocument()
    })
  })

  it('renders inactive status as Tidak Aktif', async () => {
    const inactive: ApiResident = { ...mockResident, is_active: false }
    mockFetch(() => {
      return mockSuccessResponse(inactive)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Tidak Aktif')).toBeInTheDocument()
    })
  })

  it('renders RT/RW numbers', async () => {
    const withRt: ApiResident = {
      ...mockResident,
      rt_number: '03',
      rw: 16,
    }
    mockFetch(() => {
      return mockSuccessResponse(withRt)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('3')).toBeInTheDocument()
    })
  })

  it('renders back navigation link to /warga', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText(/Kembali ke Daftar Warga/)).toBeInTheDocument()
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
      rt_number: '03',
      rw: 16,
      rt_name: 'Wisma Rukun Tunggal',
      is_active: true,
    }
    mockFetch(() => {
      return mockSuccessResponse(positionResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Position Ketua RT 03')
    })
  })
})
