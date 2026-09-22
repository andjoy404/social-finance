import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'
import { HouseholdEdit } from './HouseholdEdit'
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

function renderHouseholdEdit(householdId: string = 'hh-1') {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: mockUser,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={[`/warga/${householdId}/edit`]}>
        <Routes>
          <Route path="/warga/:id/edit" element={<HouseholdEdit />} />
          <Route path="/warga/:id" element={<div>HOUSEHOLD_DETAIL_DESTINATION</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  }
}

/** Wait for React to flush state updates from user events. */
async function tickForReact() {
  await vi.waitFor(() => {}, { timeout: 1000 })
}

// ---- Tests ----

describe('W4.3 — HouseholdEdit form rendering', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads existing household data on mount', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByLabelText(/Nomor Rumah\s*\*/i)).toHaveValue('001')
    })
    await waitFor(() => {
      expect(screen.getByLabelText(/KTP \/ NIK\s*\*/i)).toHaveValue('3201011234560001')
    })
    await waitFor(() => {
      expect(screen.getByLabelText(/Nama Kepala Keluarga\s*\*/i)).toHaveValue('Budi Santoso')
    })
  })

  it('shows 404 when household not found', async () => {
    mockFetch(() => {
      return failResponse(404, { code: 'not_found', message: 'not found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByText(/Rumah tangga tidak ditemukan/i)).toBeInTheDocument()
    })
  })

  it('maps OWNER/Pemillis and TENANT/Penyewa occupancy correctly', async () => {
    const tenantHh = { ...mockHousehold, occupancy_status: 'TENANT', head_name: 'Siti Rahayu' }
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(tenantHh)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      const select = screen.getByLabelText(/Status Hunian\s*\*/i) as HTMLSelectElement
      expect(select).toHaveValue('TENANT')
    })
  })

  it('sends occupancy_status in PATCH body and does NOT send rt_id', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (init?.method === 'PATCH' && input.includes('/api/v1/households/hh-1')) {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ ...mockHousehold, occupancy_status: 'TENANT' })
      }
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByLabelText(/Status Hunian\s*\*/i)).toBeInTheDocument()
    })

    await tickForReact()

    const statusSelect = screen.getByLabelText(/Status Hunian\s*\*/i) as HTMLSelectElement
    await userEvent.selectOptions(statusSelect, 'TENANT')
    await tickForReact()

    const submitBtn = screen.getByRole('button', { name: /Simpan Perubahan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.occupancy_status).toBe('TENANT')
    expect(capturedBody.rt_id).toBeUndefined()
  })

  it('sends Authorization header', async () => {
    let capturedAuth = ''
    mockFetch((input, init) => {
      if (init?.method === 'PATCH' && input.includes('/api/v1/households/hh-1')) {
        const headers = init.headers as Record<string, string> | undefined
        capturedAuth = headers?.Authorization ?? ''
        return okResponse(mockHousehold)
      }
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByLabelText(/Nomor Rumah\s*\*/i)).toBeInTheDocument()
    })

    await tickForReact()

    const submitBtn = screen.getByRole('button', { name: /Simpan Perubahan/i })
    await userEvent.click(submitBtn)

    expect(capturedAuth).toBe('Bearer test-jwt-token')
  })

  it('shows loading state during save', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return new Promise<Response>(() => {})
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByLabelText(/Nomor Rumah\s*\*/i)).toBeInTheDocument()
    })

    await tickForReact()

    const submitBtn = screen.getByRole('button', { name: /Simpan Perubahan/i })
    await userEvent.click(submitBtn)

    expect(screen.getByRole('button', { name: /Menyimpan/i })).toBeInTheDocument()
  })

  it('shows 404 for missing household ID', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByLabelText(/Nomor Rumah\s*\*/i)).toBeInTheDocument()
    })
  })

  it('navigates back to detail on successful save', async () => {
    mockFetch((input) => {
      if (input.includes('/api/v1/households/hh-1') && input.includes('/api')) {
        return okResponse({ ...mockHousehold, head_name: 'Updated' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByLabelText(/Nama Kepala Keluarga\s*\*/i)).toBeInTheDocument()
    })

    await tickForReact()

    const submitBtn = screen.getByRole('button', { name: /Simpan Perubahan/i })
    await userEvent.click(submitBtn)

    await waitFor(() => {
      expect(screen.getByText(/HOUSEHOLD_DETAIL_DESTINATION/)).toBeInTheDocument()
    })
  })

  it('rejects blank NIK on edit', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    const nikInput = await screen.findByLabelText(/KTP \/ NIK\s*\*/i)
    await userEvent.clear(nikInput)
    await tickForReact()

    const form = document.querySelector('form')! as HTMLFormElement
    fireEvent.submit(form)

    expect(await screen.findByText('KTP / NIK harus diisi.')).toBeInTheDocument()
  })

  it('rejects malformed NIK (non-16 digits) on edit', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    const nikInput = await screen.findByLabelText(/KTP \/ NIK\s*\*/i)
    await userEvent.clear(nikInput)
    await userEvent.type(nikInput, '12345')
    await tickForReact()

    const form = document.querySelector('form')! as HTMLFormElement
    fireEvent.submit(form)

    expect(await screen.findByText('KTP / NIK harus berupa 16 digit angka.')).toBeInTheDocument()
  })

  it('rejects blank phone on edit', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    const phoneInput = await screen.findByLabelText(/Nomor Telepon\s*\*/i)
    await userEvent.clear(phoneInput)
    await tickForReact()

    const form = document.querySelector('form')! as HTMLFormElement
    fireEvent.submit(form)

    expect(await screen.findByText('Nomor telepon harus diisi.')).toBeInTheDocument()
  })

  it('rejects blank email on edit', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    const emailInput = await screen.findByLabelText(/Email\s*\*/i)
    await userEvent.clear(emailInput)
    await tickForReact()

    const form = document.querySelector('form')! as HTMLFormElement
    fireEvent.submit(form)

    expect(await screen.findByText('Email harus diisi.')).toBeInTheDocument()
  })

  it('rejects malformed email on edit', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    const emailInput = await screen.findByLabelText(/Email\s*\*/i)
    await userEvent.clear(emailInput)
    await userEvent.type(emailInput, 'invalid-email')
    await tickForReact()

    const form = document.querySelector('form')! as HTMLFormElement
    fireEvent.submit(form)

    expect(await screen.findByText('Format email tidak valid.')).toBeInTheDocument()
  })

  it('accepts valid NIK, phone, email and sends composite Edit request correctly', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (init?.method === 'PATCH' && input.includes('/api/v1/households/hh-1')) {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ ...mockHousehold, nik: '3201019999990001', phone: '081299998888', email: 'valid@example.com' })
      }
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    const nikInput = await screen.findByLabelText(/KTP \/ NIK\s*\*/i)
    const phoneInput = await screen.findByLabelText(/Nomor Telepon\s*\*/i)
    const emailInput = await screen.findByLabelText(/Email\s*\*/i)

    await userEvent.clear(nikInput)
    await userEvent.type(nikInput, '3201019999990001')
    await tickForReact()

    await userEvent.clear(phoneInput)
    await userEvent.type(phoneInput, '081299998888')
    await tickForReact()

    await userEvent.clear(emailInput)
    await userEvent.type(emailInput, 'valid@example.com')
    await tickForReact()

    const submitBtn = screen.getByRole('button', { name: /Simpan Perubahan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.nik).toBe('3201019999990001')
    expect(capturedBody.phone).toBe('081299998888')
    expect(capturedBody.email).toBe('valid@example.com')
    expect(capturedBody.house_number).toBe('001')
    expect(capturedBody.head_name).toBe('Budi Santoso')
  })

  it('cancel button closes the modal', async () => {
    mockFetch((input, init) => {
      if (init?.method === 'GET' && input.includes('/api/v1/households/hh-1')) {
        return okResponse(mockHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdEdit()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Batal/i })).toBeInTheDocument()
    })

    const cancelButton = screen.getByRole('button', { name: /Batal/i })
    expect(cancelButton).toBeInTheDocument()

    await userEvent.click(cancelButton)
  })
})
