import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
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

  it('renders resident full name in modal subtitle', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })
  })

  it('renders NIK for pengurus role', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByDisplayValue('3201011234560001')).toBeInTheDocument()
    })
  })

  it('renders NIK for super_admin role', async () => {
    const superAdminUser = { ...mockPengurusUser, systemRole: 'super_admin', name: 'Super Admin' }
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(superAdminUser)

    await waitFor(() => {
      expect(screen.getByDisplayValue('3201011234560001')).toBeInTheDocument()
    })
  })

  it('hides NIK for warga role', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockWargaUser)

    await waitFor(() => {
      expect(screen.queryByDisplayValue('3201011234560001')).not.toBeInTheDocument()
    })
  })

  it('renders phone number', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByDisplayValue('081234567890')).toBeInTheDocument()
    })
  })

  it('renders email', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByDisplayValue('budi@example.com')).toBeInTheDocument()
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
      expect(screen.getByDisplayValue('3')).toBeInTheDocument()
    })
  })

  it('renders close button', async () => {
    mockFetch(() => {
      return mockSuccessResponse(mockResident)
    })
    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Tutup')).toBeInTheDocument()
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
      expect(screen.getByText('Position Ketua RT 03')).toBeInTheDocument()
    })
  })
})

// ---- Edit flow and warga:refresh tests ----

describe('ResidentDetail — edit flow dispatches warga:refresh on success', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('dispatches warga:refresh after successful resident update', async () => {
    let eventDispatched = false
    const handler = () => { eventDispatched = true }
    window.addEventListener('warga:refresh', handler)

    let residentPatchCount = 0
    let getCallCount = 0
    mockFetch((url, init) => {
      if (init?.method === 'PATCH' && url.includes('/residents/')) {
        residentPatchCount++
        return mockSuccessResponse({ ...mockResident, full_name: 'Updated Name' })
      }
      if (init?.method === 'PATCH' && url.includes('/households/')) {
        return mockSuccessResponse({ id: 'hh-1', house_number: '001' })
      }
      if (init?.method === 'GET' && url.includes('/residents/')) {
        getCallCount++
        // First GET = initial load, subsequent GETs = post-update refetch
        if (getCallCount === 1) {
          return mockSuccessResponse(mockResident)
        }
        return mockSuccessResponse({ ...mockResident, full_name: 'Updated Name' })
      }
      // getHousehold for address
      return mockSuccessResponse({ id: 'hh-1', house_number: '001', head_name: 'Budi', address: null })
    })

    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })

    // Click edit
    screen.getByText('Ubah').click()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Simpan Perubahan/i })).toBeInTheDocument()
    })

    // Fill form fields
    const nameInput = screen.getByLabelText(/Nama Lengkap\s*\*/i) as HTMLInputElement
    fireEvent.change(nameInput, { target: { value: 'Updated Name' } })

    const houseInput = screen.getByLabelText(/Nomor Rumah\s*\*/i) as HTMLInputElement
    fireEvent.change(houseInput, { target: { value: '001' } })

    const startDateInput = screen.getByLabelText(/Tanggal Mulai Hunian\s*\*/i) as HTMLInputElement
    fireEvent.change(startDateInput, { target: { value: '2026-01-01' } })

    // Submit
    screen.getByRole('button', { name: /Simpan Perubahan/i }).click()

    await waitFor(() => {
      expect(residentPatchCount).toBeGreaterThanOrEqual(1)
    })

    await waitFor(() => {
      expect(eventDispatched).toBe(true)
    })

    // UI should switch back from edit mode and show the refreshed data
    await waitFor(() => {
      expect(screen.getByDisplayValue('Updated Name')).toBeInTheDocument()
    })

    window.removeEventListener('warga:refresh', handler)
  })

  it('does NOT dispatch warga:refresh when update fails', async () => {
    let eventDispatched = false
    const handler = () => { eventDispatched = true }
    window.addEventListener('warga:refresh', handler)

    mockFetch((url, init) => {
      if (init?.method === 'PATCH' && url.includes('/residents/')) {
        return errorResponse(400, 'validation_error', 'Validation failed')
      }
      if (init?.method === 'GET' && url.includes('/residents/')) {
        return mockSuccessResponse(mockResident)
      }
      return mockSuccessResponse({ id: 'hh-1', house_number: '001', head_name: 'Budi', address: null })
    })

    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
    })

    screen.getByText('Ubah').click()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Simpan Perubahan/i })).toBeInTheDocument()
    })

    // Fill required fields
    const nameInput = screen.getByLabelText(/Nama Lengkap\s*\*/i) as HTMLInputElement
    fireEvent.change(nameInput, { target: { value: 'Updated Name' } })

    const houseInput = screen.getByLabelText(/Nomor Rumah\s*\*/i) as HTMLInputElement
    fireEvent.change(houseInput, { target: { value: '001' } })

    const startDateInput = screen.getByLabelText(/Tanggal Mulai Hunian\s*\*/i) as HTMLInputElement
    fireEvent.change(startDateInput, { target: { value: '2026-01-01' } })

    // Submit
    screen.getByRole('button', { name: /Simpan Perubahan/i }).click()

    // Wait for error to appear (error message comes from API response)
    await waitFor(() => {
      expect(screen.getByText('Validation failed')).toBeInTheDocument()
    })

    await new Promise(r => setTimeout(r, 100))

    expect(eventDispatched).toBe(false)

    window.removeEventListener('warga:refresh', handler)
  })

  it('allows update for residents without household (no household_id)', async () => {
    // Regression test: residents without household_id (e.g., special/unattached residents)
    // must not be blocked by household-level validation (houseNumber, startDate).
    let patchCallCount = 0
    mockFetch((url, init) => {
      if (init?.method === 'PATCH' && url.includes('/residents/')) {
        patchCallCount++
        return mockSuccessResponse({
          ...mockResident,
          id: 'no-hh-resident-1',
          full_name: 'Warga Tanpa Rumah',
          household_id: null,
        })
      }
      if (init?.method === 'GET' && url.includes('/residents/')) {
        // Initial load + post-update refetch
        return mockSuccessResponse({
          ...mockResident,
          id: 'no-hh-resident-1',
          full_name: 'Warga Tanpa Rumah',
          household_id: null,
          house_number: null,
          start_date: null,
          occupancy_status: null,
        })
      }
      return mockSuccessResponse({ id: 'hh-x', house_number: null, head_name: null, address: null })
    })

    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Warga Tanpa Rumah')).toBeInTheDocument()
    })

    // Click edit
    screen.getByText('Ubah').click()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Simpan Perubahan/i })).toBeInTheDocument()
    })

    // Fill required resident fields
    const nameInput = screen.getByLabelText(/Nama Lengkap\s*\*/i) as HTMLInputElement
    fireEvent.change(nameInput, { target: { value: 'Warga Tanpa Rumah Updated' } })

    // Household fields (house number, start date) should NOT be rendered
    // for residents without a household_id.
    expect(screen.queryByLabelText(/Nomor Rumah/i)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Tanggal Mulai/i)).not.toBeInTheDocument()

    // Submit — should succeed without any household fields
    screen.getByRole('button', { name: /Simpan Perubahan/i }).click()

    // Should succeed — PATCH is called and no validation errors appear
    await waitFor(() => {
      expect(patchCallCount).toBeGreaterThanOrEqual(1)
    })
  })

  it('allows update for non-resident personnel (Keamanan) with jabatan but no household', async () => {
    // Test: non-resident personnel (Keamanan/Operasional) have jabatan and no household_id.
    // The form must not show household fields, and must call updateSpecialResident.
    let specialEndpointCalled = false
    let regularEndpointCalled = false
    let householdEndpointCalled = false

    const mockNonResident: ApiResident = {
      id: 'special-res-1',
      rt_id: 'rt-1',
      household_id: null,
      full_name: 'Ahmad Keamanan',
      nik: null,
      phone: '081234567891',
      email: 'ahmad@example.com',
      is_active: true,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
      jabatan: 'keamanan',
      house_number: null,
      start_date: null,
      occupancy_status: null,
    }

    mockFetch((url, init) => {
      if (init?.method === 'PATCH' && url.includes('/residents/special/')) {
        specialEndpointCalled = true
        return mockSuccessResponse({
          ...mockNonResident,
          full_name: 'Ahmad Keamanan Updated',
        })
      }
      if (init?.method === 'PATCH' && url.includes('/residents/') && !url.includes('/residents/special/')) {
        regularEndpointCalled = true
      }
      if (init?.method === 'PATCH' && url.includes('/households/')) {
        householdEndpointCalled = true
      }
      if (init?.method === 'GET' && url.includes('/residents/')) {
        return mockSuccessResponse(mockNonResident)
      }
      return mockSuccessResponse({ id: 'hh-x', house_number: null, head_name: null, address: null })
    })

    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Ahmad Keamanan')).toBeInTheDocument()
    })

    // Click edit
    screen.getByText('Ubah').click()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Simpan Perubahan/i })).toBeInTheDocument()
    })

    // Household fields must NOT be rendered for non-resident personnel
    expect(screen.queryByLabelText(/Nomor Rumah/i)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Tanggal Mulai/i)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Status Hunian/i)).not.toBeInTheDocument()

    // Fill only the name field
    const nameInput = screen.getByLabelText(/Nama Lengkap\s*\*/i) as HTMLInputElement
    fireEvent.change(nameInput, { target: { value: 'Ahmad Keamanan Updated' } })

    // Submit
    screen.getByRole('button', { name: /Simpan Perubahan/i }).click()

    // Special endpoint must be called for non-resident personnel
    await waitFor(() => {
      expect(specialEndpointCalled).toBe(true)
    })

    // Regular resident endpoint must NOT be called
    expect(regularEndpointCalled).toBe(false)

    // Household endpoint must NOT be called
    expect(householdEndpointCalled).toBe(false)
  })

  it('uses regular resident endpoint (not special) for residents with jabatan AND household_id', async () => {
    // Regression test: residents with jabatan AND household_id must use the regular
    // endpoint (/api/v1/residents/:id), not the special endpoint.
    // The special endpoint requires NO active residency period, which this resident has.
    let specialEndpointCalled = false
    let regularEndpointCalled = false
    let householdEndpointCalled = false

    mockFetch((url, init) => {
      if (init?.method === 'PATCH' && url.includes('/residents/special/')) {
        specialEndpointCalled = true
      }
      if (init?.method === 'PATCH' && url.includes('/residents/') && !url.includes('/residents/special/')) {
        regularEndpointCalled = true
        return mockSuccessResponse({
          id: 'position-1',
          rt_id: 'rt-1',
          household_id: 'hh-1',
          full_name: 'Position Ketua RT 03',
          nik: '9999999999990001',
          phone: '081234567890',
          email: 'ketua.rt03@example.com',
          is_active: true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
          jabatan: 'wakil_ketua',
        })
      }
      if (init?.method === 'PATCH' && url.includes('/households/')) {
        householdEndpointCalled = true
        return mockSuccessResponse({ id: 'hh-1', house_number: '001' })
      }
      if (init?.method === 'GET' && url.includes('/residents/')) {
        // Initial load
        return mockSuccessResponse({
          id: 'position-1',
          rt_id: 'rt-1',
          household_id: 'hh-1',
          full_name: 'Position Ketua RT 03',
          nik: '9999999999990001',
          phone: '081234567890',
          email: 'ketua.rt03@example.com',
          is_active: true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
          jabatan: 'wakil_ketua',
        })
      }
      return mockSuccessResponse({ id: 'hh-1', house_number: '001', head_name: 'Budi', address: null })
    })

    renderResidentDetail(mockPengurusUser)

    await waitFor(() => {
      expect(screen.getByText('Position Ketua RT 03')).toBeInTheDocument()
    })

    // Click edit
    screen.getByText('Ubah').click()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Simpan Perubahan/i })).toBeInTheDocument()
    })

    // Fill required form fields
    const nameInput = screen.getByLabelText(/Nama Lengkap\s*\*/i) as HTMLInputElement
    fireEvent.change(nameInput, { target: { value: 'Position Ketua RT 03 Updated' } })

    const houseInput = screen.getByLabelText(/Nomor Rumah\s*\*/i) as HTMLInputElement
    fireEvent.change(houseInput, { target: { value: '001' } })

    const startDateInput = screen.getByLabelText(/Tanggal Mulai Hunian\s*\*/i) as HTMLInputElement
    fireEvent.change(startDateInput, { target: { value: '2026-01-01' } })

    // Submit
    screen.getByRole('button', { name: /Simpan Perubahan/i }).click()

    // Wait for save to complete
    await waitFor(() => {
      expect(regularEndpointCalled).toBe(true)
    })

    // Special endpoint must NOT be called for residents with household_id
    await waitFor(() => {
      expect(specialEndpointCalled).toBe(false)
    })

    // Household endpoint should also be called
    expect(householdEndpointCalled).toBe(true)
  })
})
