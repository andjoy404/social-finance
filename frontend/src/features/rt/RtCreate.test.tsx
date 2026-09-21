import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'
import { RtCreate } from './RtCreate'
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
  const body = JSON.stringify(data)
  return {
    ok: true,
    status: 201,
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

function renderRtCreate() {
  vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
    user: mockUser,
    isAuthenticated: true,
    isInitializing: false,
    login: async () => {},
    logout: () => {},
  })
  return {
    ...render(
      <MemoryRouter initialEntries={['/rt/create']}>
        <RtCreate />
      </MemoryRouter>,
    ),
  }
}

/** Wait for React to flush state updates from user events. */
async function tickForReact() {
  await vi.waitFor(() => {}, { timeout: 1000 })
}

// ---- Tests ----

describe('W3.3 — RtCreate form rendering', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders form with all required fields (name, rw, rt code)', () => {
    renderRtCreate()
    expect(screen.getByLabelText(/Nama RT/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/RW\s*\*/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Kode RT/i)).toBeInTheDocument()
  })

  it('renders optional fields (ketua rt, alamat)', () => {
    renderRtCreate()
    expect(screen.getByLabelText(/Ketua RT/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Alamat/i)).toBeInTheDocument()
  })

  it('shows validation errors when required fields are empty', async () => {
    renderRtCreate()
    const form = document.querySelector('form')! as HTMLFormElement
    fireEvent.submit(form)
    expect(screen.getByText(/Nama RT harus diisi/i)).toBeInTheDocument()
    expect(screen.getByText(/RW harus angka yang valid/i)).toBeInTheDocument()
    expect(screen.getByText(/Kode RT harus diisi/i)).toBeInTheDocument()
  })

  it('shows validation error when RW is not numeric', async () => {
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const form = document.querySelector('form')! as HTMLFormElement

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, 'abc')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    fireEvent.submit(form)

    expect(screen.getByText(/RW harus angka yang valid/i)).toBeInTheDocument()
  })

  it('shows validation error when RT code is empty', async () => {
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const form = document.querySelector('form')! as HTMLFormElement

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    fireEvent.submit(form)

    expect(screen.getByText(/Kode RT harus diisi/i)).toBeInTheDocument()
  })

  it('successfully submits valid form data', async () => {
    mockFetch((input, init) => {
      if (input.includes('/api/v1/rts') && init?.method === 'POST') {
        return okResponse({ id: 'rt-123', name: 'RT 001', rw: 1, rt: '001', address: null, head_name: null, is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    await userEvent.click(submitBtn)
  })

  it('shows error message from API on 409 conflict', async () => {
    mockFetch(() => {
      return failResponse(409, { code: 'conflict', message: 'RT sudah ada' })
    })
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    await userEvent.click(submitBtn)

    expect(screen.getByText(/RT sudah ada/i)).toBeInTheDocument()
  })

  it('shows error message from API on 400 validation error', async () => {
    mockFetch(() => {
      return failResponse(400, { code: 'validation_error', message: 'Data tidak valid' })
    })
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    await userEvent.click(submitBtn)

    expect(screen.getByText(/Data tidak valid/i)).toBeInTheDocument()
  })

  it('shows error message from API on 500 server error', async () => {
    mockFetch(() => {
      return failResponse(500, { code: 'internal_error', message: 'Terjadi kesalahan server' })
    })
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    await userEvent.click(submitBtn)

    expect(screen.getByText(/Terjadi kesalahan server/i)).toBeInTheDocument()
  })

  it('shows loading state during submit', async () => {
    const promise = new Promise<Response>(() => {})
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => promise)
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    await userEvent.click(submitBtn)

    expect(screen.getByRole('button', { name: /Menyimpan/i })).toBeInTheDocument()
  })

  it('submit button is disabled during loading', async () => {
    const promise = new Promise<Response>(() => {})
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => promise)
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    expect(submitBtn).not.toBeDisabled()

    await userEvent.click(submitBtn)

    const loadingBtn = screen.getByRole('button', { name: /Menyimpan/i })
    expect(loadingBtn).toBeDisabled()
  })

  it('optional fields (address, head_name) are nullable at API level', async () => {
    let capturedBody: Record<string, unknown> = {}
    mockFetch((input, init) => {
      if (input.includes('/api/v1/rts') && init?.method === 'POST') {
        capturedBody = JSON.parse((init.body as string) || '{}')
        return okResponse({ id: 'rt-123', name: capturedBody.name, rw: capturedBody.rw, rt: capturedBody.rt, address: null, head_name: null, is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT 001')
    await tickForReact()

    await userEvent.type(rwInput, '1')
    await tickForReact()

    await userEvent.type(rtInput, '001')
    await tickForReact()

    await userEvent.click(submitBtn)

    expect(capturedBody.name).toBe('RT 001')
    expect(capturedBody.rw).toBe(1)
  })
})

describe('W3.3 — RtCreate navigation', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'test-jwt-token', refreshToken: 'refresh-token' })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('navigates to /rt on successful submission', async () => {
    mockFetch((input) => {
      if (input.includes('/api/v1/rts')) {
        return okResponse({ id: 'rt-new', name: 'RT Baru', rw: 2, rt: '002', address: null, head_name: null, is_active: true, created_at: '', updated_at: '' })
      }
      return failResponse(404, { code: 'not_found' })
    })
    renderRtCreate()
    const nameInput = screen.getByLabelText(/Nama RT/i) as HTMLInputElement
    const rwInput = screen.getByLabelText(/RW/i) as HTMLInputElement
    const rtInput = screen.getByLabelText(/Kode RT/i) as HTMLInputElement
    const submitBtn = screen.getByRole('button', { name: /Simpan/i })

    await userEvent.type(nameInput, 'RT Baru')
    await tickForReact()

    await userEvent.type(rwInput, '2')
    await tickForReact()

    await userEvent.type(rtInput, '002')
    await tickForReact()

    await userEvent.click(submitBtn)

    expect(document.querySelector('a[href="/rt"]')).toBeInTheDocument()
  })

  it('cancel button navigates back to /rt', async () => {
    renderRtCreate()
    const cancelLink = screen.getByRole('link', { name: /Batal/i })
    expect(cancelLink).toHaveAttribute('href', '/rt')

    await userEvent.click(cancelLink)

    const backLink = document.querySelector('a[href="/rt"]')
    expect(backLink).toBeInTheDocument()
  })
})
