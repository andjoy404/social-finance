import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'
import { HouseholdDetail } from './HouseholdDetail'
import { persistSessionPair } from '@/app/api'
import * as AuthModule from '@/app/AuthContext'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

// ---- Test helpers ----

const mockUser = {
  id: 'user-1',
  name: 'Test User',
  email: 'test@example.com',
  systemRole: null,
  role: 'pengurus',
  rt: { id: 'rt-1', name: 'RT 001' },
}

const mockHousehold = {
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

function mockFetch(fn: (input: string, init?: RequestInit) => Response | Promise<Response>) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(
    async (input, init) => fn(typeof input === 'string' ? input : '', init),
  )
}

function okResponse<R>(data: R): Response {
  const body = JSON.stringify(data)
  return {
    ok: true,
    status: 200,
    text: () => Promise.resolve(body),
    json: () => Promise.resolve(data),
  } as Response
}

function failResponse(status: number, data: unknown): Response {
  return {
    ok: false,
    status,
    text: () => Promise.resolve(JSON.stringify(data)),
    json: () => Promise.resolve(data),
  } as Response
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

describe('W4.3 — HouseholdDetail action buttons', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('EDIT button navigates to edit page', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('link', { name: /Edit/i })).toBeInTheDocument()
    })

    const editLink = screen.getByRole('link', { name: /Edit/i })
    expect(editLink).toHaveAttribute('href', '/warga/hh-1/edit')
  })

  it('MOVE button navigates to move page', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('link', { name: /Pindah/i })).toBeInTheDocument()
    })

    const moveLink = screen.getByRole('link', { name: /Pindah/i })
    expect(moveLink).toHaveAttribute('href', '/warga/hh-1/pindah')
  })

  it('DEACTIVATE button shows confirmation dialog', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Nonaktifkan/i })).toBeInTheDocument()
    })

    const deactBtn = screen.getByRole('button', { name: /Nonaktifkan/i })

    // Before click: no confirmation
    expect(screen.queryByText(/Nonaktifkan rumah tangga/i)).not.toBeInTheDocument()

    // Click to show confirmation
    await userEvent.click(deactBtn)

    // Find the confirmation text within the confirmation card
    const confirmCard = document.querySelector('[style*="var(--color-expense-subtle)"]')
    expect(confirmCard).toBeInTheDocument()
    expect(confirmCard).toHaveTextContent('001')
    expect(confirmCard).toHaveTextContent('Budi Santoso')
  })

  it('deactivate confirmation modal shows confirmation heading', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Nonaktifkan/i })).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('button', { name: /Nonaktifkan/i }))

    // The confirmation string is unique — only appears in the confirmation card
    expect(screen.getByText(/Nonaktifkan rumah tangga 001/i)).toBeInTheDocument()
    expect(screen.getByText(/— Budi Santoso\?/i)).toBeInTheDocument()
  })

  it('NO API call is made until confirm in modal', async () => {
    let deleteCalled = false
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      if (init?.method === 'DELETE' && input.includes('/api/v1/households/hh-1')) {
        deleteCalled = true
        return { ok: true, status: 204, text: () => Promise.resolve('') } as Response
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Nonaktifkan/i })).toBeInTheDocument()
    })

    // Click deactivate button
    await userEvent.click(screen.getByRole('button', { name: /Nonaktifkan/i }))

    expect(deleteCalled).toBe(false)

    // Click cancel
    await userEvent.click(screen.getByRole('button', { name: /Batal/i }))

    expect(deleteCalled).toBe(false)
  })

  it('calls deactivate API on confirm', async () => {
    let deleteCalled = false
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      if (init?.method === 'DELETE' && input.includes('/api/v1/households/hh-1')) {
        deleteCalled = true
        return { ok: true, status: 204, text: () => Promise.resolve('') } as Response
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Nonaktifkan/i })).toBeInTheDocument()
    })

    // Click the deactivate button
    await userEvent.click(screen.getByRole('button', { name: /Nonaktifkan/i }))

    // Find confirm button
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Ya, Nonaktifkan/i })).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('button', { name: /Ya, Nonaktifkan/i }))

    expect(deleteCalled).toBe(true)
  })

  it('navigates to /warga on successful deactivate', async () => {
    // Use wrapper routes so navigation succeeds
    function Wrapper() {
      return (
        <Routes>
          <Route path="/warga/:id" element={<HouseholdDetail />} />
          <Route path="/warga" element={<div>Daftar Warga</div>} />
        </Routes>
      )
    }

    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      if (init?.method === 'DELETE' && input.includes('/api/v1/households/hh-1')) {
        return { ok: true, status: 204, text: () => Promise.resolve('') } as Response
      }
      return failResponse(404, { code: 'not_found' })
    })

    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    render(
      <MemoryRouter initialEntries={['/warga/hh-1']}>
        <Wrapper />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Nonaktifkan/i })).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('button', { name: /Nonaktifkan/i }))
    await userEvent.click(screen.getByRole('button', { name: /Ya, Nonaktifkan/i }))

    expect(screen.getByText(/Daftar Warga/i)).toBeInTheDocument()
  })

  it('shows loading during deactivate', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return new Promise<Response>(() => {})
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Nonaktifkan/i })).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('button', { name: /Nonaktifkan/i }))
    await userEvent.click(screen.getByRole('button', { name: /Ya, Nonaktifkan/i }))

    expect(screen.getByRole('button', { name: /Menonaktifkan/i })).toBeInTheDocument()
  })

  it('does NOT show action buttons for inactive household', async () => {
    const inactiveHh = { ...mockHousehold, is_active: false }
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(inactiveHh)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByText(/Tidak Aktif/i)).toBeInTheDocument()
    })

    expect(screen.queryByRole('link', { name: /Edit/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Pindah/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Nonaktifkan/i })).not.toBeInTheDocument()
  })

  it('deactivate exists for active households', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdDetail()

    await waitFor(() => {
      expect(screen.getByText(/Detail rumah tangga/i)).toBeInTheDocument()
    })

    // Verify deactivate exists
    expect(screen.getByRole('button', { name: /Nonaktifkan/i })).toBeInTheDocument()
  })
})
