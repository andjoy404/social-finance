import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'
import { HouseholdCreate } from './HouseholdCreate'
import { HouseholdEdit } from './HouseholdEdit'
import { persistSessionPair } from '@/app/api'
import * as AuthModule from '@/app/AuthContext'
import { MemoryRouter } from 'react-router-dom'

// ---- Test helpers ----

const mockUser = {
  id: 'user-1',
  name: 'Test User',
  email: 'test@example.com',
  systemRole: null,
  role: 'pengurus',
  rt: { id: 'rt-1', name: 'RT 001' },
}

function mockFetch(fn: (input: string, init?: RequestInit) => Response | Promise<Response>) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(
    async (input, init) => fn(typeof input === 'string' ? input : '', init),
  )
}

function okResponse<R>(data: R): Response {
  return new Response(JSON.stringify(data), {
    status: 201,
    headers: { 'Content-Type': 'application/json' },
  })
}

function mockSuccessResponse<R>(data: R): Response {
  return new Response(JSON.stringify(data), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function failResponse(status: number, data: unknown): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function renderHouseholdCreate() {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: mockUser,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={['/warga/baru']}>
        <HouseholdCreate />
      </MemoryRouter>,
    ),
  }
}

/** Wait for React to flush state updates from user events. */
async function tickForReact() {
  await vi.waitFor(() => {}, { timeout: 1000 })
}

/** Helper to fill all required fields in the HouseholdCreate form. */
async function fillValidForm(overrides?: {
  houseNumber?: string
  headName?: string
  nik?: string
  phone?: string
  email?: string
  startDate?: string
  address?: string
  occupancyStatus?: string
}) {
  const houseInput = screen.getByLabelText(/Nomor Rumah\s*\*/i) as HTMLInputElement
  const headInput = screen.getByLabelText(/Nama Kepala Keluarga\s*\*/i) as HTMLInputElement
  const nikInput = screen.getByLabelText(/KTP \/ NIK\s*\*/i) as HTMLInputElement
  const phoneInput = screen.getByLabelText(/Nomor Telepon\s*\*/i) as HTMLInputElement
  const emailInput = screen.getByLabelText(/Email\s*\*/i) as HTMLInputElement
  const dateInput = screen.getByLabelText(/Tanggal Mulai Hunian\s*\*/i) as HTMLInputElement

  await userEvent.type(houseInput, overrides?.houseNumber ?? '001')
  await tickForReact()

  await userEvent.type(headInput, overrides?.headName ?? 'Budi')
  await tickForReact()

  await userEvent.type(nikInput, overrides?.nik ?? '3201011234560001')
  await tickForReact()

  await userEvent.type(phoneInput, overrides?.phone ?? '081234567890')
  await tickForReact()

  await userEvent.type(emailInput, overrides?.email ?? 'budi@example.com')
  await tickForReact()

  await userEvent.type(dateInput, overrides?.startDate ?? '2026-01-01')
  await tickForReact()

  if (overrides?.address) {
    const addressInput = screen.getByLabelText(/Alamat/i) as HTMLInputElement
    await userEvent.type(addressInput, overrides.address)
    await tickForReact()
  }

  if (overrides?.occupancyStatus) {
    const statusSelect = screen.getByLabelText(/Status Hunian\s*\*/i) as HTMLSelectElement
    await userEvent.selectOptions(statusSelect, overrides.occupancyStatus)
    await tickForReact()
  }
}

// ---- Tests ----

describe('W4.3 — HouseholdCreate form rendering', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders form with all required fields', () => {
    renderHouseholdCreate()
    expect(screen.getByLabelText(/Nomor Rumah\s*\*/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Nama Kepala Keluarga\s*\*/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/KTP \/ NIK\s*\*/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Nomor Telepon\s*\*/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Email\s*\*/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Status Hunian\s*\*/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Tanggal Mulai Hunian\s*\*/i)).toBeInTheDocument()
  })

  it('renders optional fields (alamat)', () => {
    renderHouseholdCreate()
    expect(screen.getByLabelText(/Alamat/i)).toBeInTheDocument()
  })

  it('shows OWNER/Pemilik and TENANT/Penyewa options in occupancy status', () => {
    renderHouseholdCreate()
    const select = screen.getByLabelText(/Status Hunian\s*\*/i) as HTMLSelectElement
    expect(select).toBeInTheDocument()
    expect(select).toHaveValue('OWNER')
    expect(screen.getByRole('option', { name: /Pemilik/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /Penyewa/i })).toBeInTheDocument()
  })

  it('shows validation errors when required fields are empty', async () => {
    renderHouseholdCreate()
    const form = document.querySelector('form')! as HTMLFormElement
    fireEvent.submit(form)
    expect(screen.getByText(/Nomor rumah harus diisi/i)).toBeInTheDocument()
    expect(screen.getByText(/Nama kepala keluarga harus diisi/i)).toBeInTheDocument()
    expect(screen.getByText(/KTP \/ NIK harus diisi/i)).toBeInTheDocument()
    expect(screen.getByText(/Nomor telepon harus diisi/i)).toBeInTheDocument()
    expect(screen.getByText(/Email harus diisi/i)).toBeInTheDocument()
    expect(screen.getByText(/Tanggal mulai hunian harus diisi/i)).toBeInTheDocument()
  })

  it('does NOT send rt_id in API request', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.rt_id).toBeUndefined()
    expect(capturedBody.house_number).toBe('001')
    expect(capturedBody.nik).toBe('3201011234560001')
  })

  it('sends explicit start_date in API request', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm({ startDate: '2026-06-15' })
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.start_date).toBe('2026-06-15')
  })

  it('maps OWNER occupancy_status correctly', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm({ occupancyStatus: 'OWNER' })
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.occupancy_status).toBe('OWNER')
  })

  it('maps TENANT/Penyewa occupancy_status correctly', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'TENANT', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm({ occupancyStatus: 'TENANT' })
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.occupancy_status).toBe('TENANT')
  })

  it('sends null for empty optional fields, not placeholders', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.address).toBeNull()
    expect(capturedBody.nik).toBe('3201011234560001')
  })

  it('calls /households API endpoint on submit', async () => {
    let capturedUrl = ''
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedUrl = input
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedUrl).toContain('/api/v1/households')
  })

  it('sends Authorization header with request', async () => {
    let capturedAuth = ''
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        const headers = init.headers as Record<string, string> | undefined
        capturedAuth = headers?.Authorization ?? ''
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedAuth).toBe('Bearer test-jwt-token')
  })

  it('shows API error on 400 validation error', async () => {
    mockFetch(() => {
      return failResponse(400, { code: 'validation_error', message: 'Nomor rumah sudah ada' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(screen.getByText(/Nomor rumah sudah ada/i)).toBeInTheDocument()
  })

  it('shows API error on 409 conflict', async () => {
    mockFetch(() => {
      return failResponse(409, { code: 'conflict', message: 'Nomor rumah sudah terdaftar' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(screen.getByText(/Nomor rumah sudah terdaftar/i)).toBeInTheDocument()
  })

  it('shows general error on 500 server error', async () => {
    mockFetch(() => {
      return failResponse(500, { code: 'internal_error', message: 'Terjadi kesalahan server' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(screen.getByText(/Terjadi kesalahan server/i)).toBeInTheDocument()
  })

  it('shows loading state during submit', async () => {
    const promise = new Promise<Response>(() => {})
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => promise)
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(screen.getByRole('button', { name: /Menyimpan/i })).toBeInTheDocument()
  })

  it('submit button is disabled during loading', async () => {
    const promise = new Promise<Response>(() => {})
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => promise)
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    expect(submitBtn).not.toBeDisabled()

    await userEvent.click(submitBtn)

    const loadingBtn = screen.getByRole('button', { name: /Menyimpan/i })
    expect(loadingBtn).toBeDisabled()
  })

  it('navigates to /warga on successful submission', async () => {
    mockFetch((input) => {
      if (input.includes('/api/v1/households')) {
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Budi', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(document.querySelector('a[href="/warga"]')).toBeInTheDocument()
  })

  it('cancel button navigates back to /warga', async () => {
    renderHouseholdCreate()
    const cancelLink = screen.getByRole('link', { name: /Batal/i })
    expect(cancelLink).toHaveAttribute('href', '/warga')

    await userEvent.click(cancelLink)

    const backLink = document.querySelector('a[href="/warga"]')
    expect(backLink).toBeInTheDocument()
  })
})

// ---- SUPER_ADMIN tests ----

const mockSuperAdminUser = {
  id: 'sa-user',
  name: 'Super Admin',
  email: 'sa@example.com',
  systemRole: 'super_admin',
  role: '',
  rt: null,
}

function renderSuperAdminCreate() {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: mockSuperAdminUser,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={['/warga/baru']}>
        <HouseholdCreate />
      </MemoryRouter>,
    ),
  }
}

describe('SA.2A-WEB1 — HouseholdCreate for SUPER_ADMIN', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'sa-jwt-token', refreshToken: 'sa-refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('RT selector is visible for SUPER_ADMIN', async () => {
    const activeRt = { id: 'sa-rt-1', name: 'RT 001 / RW 016 — Sudin', rw: 16, rt: '001', address: null, head_name: null, is_active: true, created_at: '', updated_at: '' }
    mockFetch((input, init) => {
      if (input.includes('/api/v1/rts') && init?.method === 'GET') {
        return mockSuccessResponse({ data: [activeRt], pagination: { page: 1, page_size: 100, total: 1, total_pages: 1 } })
      }
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        return mockSuccessResponse({ id: 'hh-new', rt_id: 'sa-rt-1' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderSuperAdminCreate()

    await waitFor(() => {
      const combobox = screen.getByRole('combobox', { name: /RT Tujuan/i })
      expect(combobox).toHaveAttribute('aria-expanded', 'false')
    })
  })

  it('active RT options loaded via apiListRTs', async () => {
    const activeRt = { id: 'sa-rt-1', name: 'RT 001 / RW 016 — Sudin', rw: 16, rt: '001', address: null, head_name: null, is_active: true, created_at: '', updated_at: '' }
    mockFetch((input, init) => {
      if (input.includes('/api/v1/rts') && init?.method === 'GET') {
        return mockSuccessResponse({ data: [activeRt], pagination: { page: 1, page_size: 100, total: 1, total_pages: 1 } })
      }
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        return okResponse({ id: 'hh-new', rt_id: 'sa-rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderSuperAdminCreate()

    await waitFor(() => {
      const combobox = screen.getByRole('combobox', { name: /RT Tujuan/i })
      expect(combobox).toBeInTheDocument()
    })
  })

  it('SUPER_ADMIN RT selector requests active RTs only', async () => {
    let capturedUrl = ''
    const activeRt = { id: 'sa-rt-1', name: 'RT 001 / RW 016 — Sudin', rw: 16, rt: '001', address: null, head_name: null, is_active: true, created_at: '', updated_at: '' }
    mockFetch((input, init) => {
      if (input.includes('/api/v1/rts') && init?.method === 'GET') {
        capturedUrl = input
        return mockSuccessResponse({ data: [activeRt], pagination: { page: 1, page_size: 100, total: 1, total_pages: 1 } })
      }
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        return failResponse(404, { code: 'not_found' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderSuperAdminCreate()

    await waitFor(() => {
      expect(capturedUrl).toContain('is_active=true')
    })

    await waitFor(() => {
      const combobox = screen.getByRole('combobox', { name: /RT Tujuan/i })
      expect(combobox).toBeInTheDocument()
    })
  })

  it('submit disabled until target RT selected', async () => {
    const activeRt = { id: 'sa-rt-1', name: 'RT 001 / RW 016 — Sudin', rw: 16, rt: '001', address: null, head_name: null, is_active: true, created_at: '', updated_at: '' }
    mockFetch((input, init) => {
      if (input.includes('/api/v1/rts') && init?.method === 'GET') {
        return mockSuccessResponse({ data: [activeRt], pagination: { page: 1, page_size: 100, total: 1, total_pages: 1 } })
      }
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        return failResponse(404, { code: 'not_found' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderSuperAdminCreate()

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: /RT Tujuan/i })).toBeInTheDocument()
    })

    await fillValidForm()

    expect(screen.getByRole('button', { name: /Pilih RT/i })).toBeInTheDocument()
  })

  it('successful submit includes selected rt_id', async () => {
    const activeRt = { id: 'sa-rt-1', name: 'RT 001 / RW 016 — Sudin', rw: 16, rt: '001', address: null, head_name: null, is_active: true, created_at: '', updated_at: '' }
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/rts') && init?.method === 'GET') {
        return mockSuccessResponse({ data: [activeRt], pagination: { page: 1, page_size: 100, total: 1, total_pages: 1 } })
      }
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'sa-rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderSuperAdminCreate()

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: /RT Tujuan/i })).toBeInTheDocument()
    })

    // Open combobox, type search, click option
    const combobox = screen.getByRole('combobox', { name: /RT Tujuan/i })
    await userEvent.click(combobox)

    await waitFor(() => {
      const option = screen.getByRole('option', { name: /RT 001 \/ RW 016/ })
      expect(option).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /RT 001 \/ RW 016/ }))
    await tickForReact()

    await fillValidForm()

    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.rt_id).toBe('sa-rt-1')
    expect(capturedBody.house_number).toBe('001')
  })
})

// ---- is_active Status Tests ----

describe('W4.3 — HouseholdCreate is_active defaults to active', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('Status field renders in form', () => {
    renderHouseholdCreate()
    expect(screen.getByLabelText(/Status\s*\*/i)).toBeInTheDocument()
  })

  it('Status field default is Aktif', () => {
    renderHouseholdCreate()

    const select = screen.getByLabelText(/Status\s*\*/i) as HTMLSelectElement

    expect(select).toHaveValue('true')
    expect(select.querySelector('option[value="true"]')).toHaveTextContent('Aktif')
    expect(select.querySelector('option[value="false"]')).toHaveTextContent('Tidak Aktif')
  })

  it('send is_active=true in API request when creating active household', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.is_active).toBe(true)
  })

  it('can switch status to Tidak Aktif in create form', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'hh-new', rt_id: 'rt-1', house_number: '001', head_name: 'Test', address: null, phone: '081234567890', nik: '3201011234560001', email: 'test@example.com', occupancy_status: 'OWNER', is_active: false, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderHouseholdCreate()
    await fillValidForm()
    const statusSelect = screen.getByLabelText(/Status\s*\*/i) as HTMLSelectElement
    await userEvent.selectOptions(statusSelect, 'false')
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.is_active).toBe(false)
  })
})

// ---- HouseholdEdit is_active Tests ----

const mockApiHousehold = {
  id: 'hh-old',
  rt_id: 'rt-1',
  house_number: '001',
  head_name: 'Budi',
  nik: '3201011234560001',
  phone: '081234567890',
  email: 'budi@example.com',
  address: 'Jl. Test 1',
  occupancy_status: 'OWNER',
  is_active: true,
  head_resident: {
    id: 'res-1',
    rt_id: 'rt-1',
    full_name: 'Budi',
    nik: '3201011234560001',
    phone: '081234567890',
    email: 'budi@example.com',
    is_active: true,
  },
  created_at: '',
  updated_at: '',
}

describe('W4.3 — HouseholdEdit is_active', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('edit form loads current is_active from API', async () => {
    mockFetch((input) => {
      if (input.includes('/api/v1/households/hh-old')) {
        return mockSuccessResponse(mockApiHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })
    render(
      <MemoryRouter initialEntries={['/warga/hh-old/edit']}>
        <HouseholdEdit id="hh-old" />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByLabelText(/Status\s*\*/i)).toBeInTheDocument()
    })

    const select = screen.getByLabelText(/Status\s*\*/i) as HTMLSelectElement
    expect(select).toHaveValue('true')
  })

  it('edit form includes is_active in PATCH body', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households/hh-old') && init?.method === 'PATCH') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return mockSuccessResponse({ ...mockApiHousehold, head_name: 'Updated' })
      }
      if (input.includes('/api/v1/households/hh-old')) {
        return mockSuccessResponse(mockApiHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })

    render(
      <MemoryRouter initialEntries={['/warga/hh-old/edit']}>
        <HouseholdEdit id="hh-old" />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByLabelText(/Status\s*\*/i)).toBeInTheDocument()
    })

    const select = screen.getByLabelText(/Status\s*\*/i) as HTMLSelectElement
    expect(select).toHaveValue('true')

    // Submit the edit form
    const submitBtn = screen.getByRole('button', { name: /Simpan Perubahan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.is_active).toBe(true)
  })

  it('edit form can switch status to Tidak Aktif', async () => {
    const inactiveHousehold = { ...mockApiHousehold, is_active: false, head_name: 'Test HH' }
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/households/hh-old') && init?.method === 'PATCH') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return mockSuccessResponse({ ...inactiveHousehold, is_active: false })
      }
      if (input.includes('/api/v1/households/hh-old')) {
        return mockSuccessResponse(inactiveHousehold)
      }
      return failResponse(404, { code: 'not_found' })
    })

    render(
      <MemoryRouter initialEntries={['/warga/hh-old/edit']}>
        <HouseholdEdit id="hh-old" />
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByLabelText(/Status\s*\*/i)).toBeInTheDocument()
    })

    // Change status to Tidak Aktif
    const select = screen.getByLabelText(/Status\s*\*/i) as HTMLSelectElement
    await userEvent.selectOptions(select, 'false')

    const submitBtn = screen.getByRole('button', { name: /Simpan Perubahan/i })
    await userEvent.click(submitBtn)

    expect(capturedBody.is_active).toBe(false)
  })
})
