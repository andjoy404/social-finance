import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  apiCreateRT,
  apiUpdateRT,
  apiDeactivateRT,
  apiReactivateRT,
  ApiError,
  clearSessionPair,
  setTokenManager,
} from '@/app/api'

// ---- Test helpers ----

function okResponse<R>(data: R): Response {
  const body = JSON.stringify(data)
  return {
    ok: true,
    status: 200,
    text: () => Promise.resolve(body),
    json: () => Promise.resolve(data),
  } as Response
}

function createdResponse<R>(data: R): Response {
  const body = JSON.stringify(data)
  return {
    ok: true,
    status: 201,
    text: () => Promise.resolve(body),
    json: () => Promise.resolve(data),
  } as Response
}

function noContentResponse(): Response {
  return {
    ok: true,
    status: 204,
    text: () => Promise.resolve(''),
    json: () => Promise.resolve(null),
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

function setupFetchMock(
  fn: (input: string, init?: RequestInit) => Response | Promise<Response>,
) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => fn(typeof input === 'string' ? input : '', init))
}

// ---- W3.3 — apiCreateRT ----

describe('W3.3 — apiCreateRT', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('creates RT successfully and returns RT data', async () => {
    const rtData = {
      id: 'rt-new',
      name: 'RT 005',
      rw: 2,
      rt: '005',
      address: 'Jl. Pahlawan No. 5',
      head_name: 'Citra',
      is_active: true,
      created_at: '2024-06-01T00:00:00Z',
      updated_at: '2024-06-01T00:00:00Z',
    }
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts') && !input.includes('/deactivate')) {
        return createdResponse(rtData)
      }
      return failResponse(404, { code: 'not_found' })
    })
    const result = await apiCreateRT('test-jwt', {
      name: 'RT 005',
      rw: 2,
      rt: '005',
      address: 'Jl. Pahlawan No. 5',
      head_name: 'Citra',
    })
    expect(result?.id).toBe('rt-new')
    expect(result?.name).toBe('RT 005')
    expect(result?.rw).toBe(2)
    expect(result?.rt).toBe('005')
    expect(result?.is_active).toBe(true)
  })

  it('creates RT with optional fields as null when not provided', async () => {
    const rtData = {
      id: 'rt-minimal',
      name: 'RT 010',
      rw: 5,
      rt: '010',
      address: null,
      head_name: null,
      is_active: true,
      created_at: '2024-06-01T00:00:00Z',
      updated_at: '2024-06-01T00:00:00Z',
    }
    setupFetchMock(() => createdResponse(rtData))
    const result = await apiCreateRT('test-jwt', {
      name: 'RT 010',
      rw: 5,
      rt: '010',
    })
    expect(result?.address).toBeNull()
    expect(result?.head_name).toBeNull()
  })

  it('handles 204 No Content response gracefully by returning undefined', async () => {
    setupFetchMock(() => new Response(null, { status: 204 }))
    const result = await apiCreateRT('test-jwt', {
      name: 'RT 010',
      rw: 5,
      rt: '010',
    })
    expect(result).toBeUndefined()
  })

  it('sends correct headers when creating RT', async () => {
    let capturedHeaders: Record<string, string> | undefined
    setupFetchMock(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/api/v1/rts') && !input.includes('/deactivate')) {
        capturedHeaders = init?.headers as Record<string, string> || {}
        return createdResponse({
          id: 'rt-headers', name: 'RT 001', rw: 1, rt: '001',
          address: null, head_name: null, is_active: true,
          created_at: '2024-06-01T00:00:00Z', updated_at: '2024-06-01T00:00:00Z',
        })
      }
      return failResponse(404, { code: 'not_found' })
    })
    setTokenManager(
      () => undefined,
      () => undefined,
      () => {},
      () => {},
    )
    await apiCreateRT('w3-create-token', { name: 'RT 001', rw: 1, rt: '001' })
    expect(capturedHeaders).toHaveProperty('Authorization', 'Bearer w3-create-token')
    expect(capturedHeaders).toHaveProperty('Content-Type', 'application/json')
  })
})

// ---- W3.3 — apiCreateRT errors ----

describe('W3.3 — apiCreateRT errors', () => {
  beforeEach(vi.restoreAllMocks)
  afterEach(clearSessionPair)

  it('throws ApiError on validation error (400)', async () => {
    setupFetchMock(() => {
      return failResponse(400, { code: 'validation_error', message: 'RW harus angka yang valid.' })
    })
    try {
      await apiCreateRT('test-jwt', { name: 'RT 005', rw: 0, rt: '' })
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
      if (err instanceof ApiError) {
        expect(err.code).toBe('validation_error')
      }
    }
  })

  it('throws ApiError on duplicate RT (409)', async () => {
    setupFetchMock(() => {
      return failResponse(409, { code: 'conflict', message: 'RT combination already exists' })
    })
    try {
      await apiCreateRT('test-jwt', { name: 'RT 001', rw: 1, rt: '001' })
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
      if (err instanceof ApiError) {
        expect(err.code).toBe('conflict')
      }
    }
  })

  it('throws ApiError on server error (500)', async () => {
    setupFetchMock(() => {
      return failResponse(500, { code: 'internal_error', message: 'server error' })
    })
    try {
      await apiCreateRT('jwt', { name: 'RT 001', rw: 1, rt: '001' })
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })

  it('throws ApiError on auth failure (401)', async () => {
    setupFetchMock(() => {
      return failResponse(401, { code: 'unauthorized', message: 'invalid token' })
    })
    try {
      await apiCreateRT('bad-jwt', { name: 'RT 001', rw: 1, rt: '001' })
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })
})

// ---- W3.3 — apiUpdateRT ----

describe('W3.3 — apiUpdateRT', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('updates RT successfully and returns updated data', async () => {
    const rtData = {
      id: 'rt-update',
      name: 'RT 001 Updated',
      rw: 1,
      rt: '001',
      address: 'Jl. Baru No. 1',
      head_name: 'Doni',
      is_active: true,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-07-01T00:00:00Z',
    }
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts/rt-update')) {
        return okResponse(rtData)
      }
      return failResponse(404, { code: 'not_found' })
    })
    const result = await apiUpdateRT('test-jwt', 'rt-update', {
      name: 'RT 001 Updated',
      address: 'Jl. Baru No. 1',
      head_name: 'Doni',
    })
    expect(result.name).toBe('RT 001 Updated')
    expect(result.head_name).toBe('Doni')
    expect(result.updated_at).toBe('2024-07-01T00:00:00Z')
  })

  it('sends PATCH method for update', async () => {
    let capturedMethod: string | undefined
    setupFetchMock(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/api/v1/rts/rt-update')) {
        capturedMethod = init?.method
        return okResponse({
          id: 'rt-update', name: 'RT 001', rw: 1, rt: '001',
          address: null, head_name: null, is_active: true,
          created_at: '2024-01-01T00:00:00Z', updated_at: '2024-07-01T00:00:00Z',
        })
      }
      return failResponse(404, { code: 'not_found' })
    })
    await apiUpdateRT('test-jwt', 'rt-update', { name: 'RT 001' })
    expect(capturedMethod).toBe('PATCH')
  })

  it('updates partial fields only', async () => {
    let capturedBody: Record<string, unknown> | undefined
    setupFetchMock(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/api/v1/rts/rt-partial')) {
        const text = await init?.body?.toString() || ''
        capturedBody = JSON.parse(text)
        return okResponse({
          id: 'rt-partial', name: 'RT 002', rw: 2, rt: '002',
          address: 'Jl. Lama', head_name: 'Lama', is_active: true,
          created_at: '2024-01-01T00:00:00Z', updated_at: '2024-07-01T00:00:00Z',
        })
      }
      return failResponse(404, { code: 'not_found' })
    })
    await apiUpdateRT('test-jwt', 'rt-partial', { head_name: 'Baru' })
    expect(capturedBody).toEqual({ head_name: 'Baru' })
  })

  it('throws ApiError on RT not found (404)', async () => {
    setupFetchMock(() => {
      return failResponse(404, { code: 'not_found', message: 'RT not found' })
    })
    try {
      await apiUpdateRT('test-jwt', 'nonexistent', { name: 'RT 999' })
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })

  it('throws ApiError on validation error (400)', async () => {
    setupFetchMock(() => {
      return failResponse(400, { code: 'validation_error', message: 'RW harus angka yang valid.' })
    })
    try {
      await apiUpdateRT('test-jwt', 'rt-123', { rw: -1 })
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })
})

// ---- W3.3 — apiDeactivateRT ----

describe('W3.3 — apiDeactivateRT', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('deactivates RT successfully with 204', async () => {
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts/rt-deact/deactivate')) {
        return noContentResponse()
      }
      return failResponse(404, { code: 'not_found' })
    })
    await expect(apiDeactivateRT('test-jwt', 'rt-deact')).resolves.toBeUndefined()
  })

  it('deactivates RT using PATCH method', async () => {
    let capturedMethod: string | undefined
    setupFetchMock(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/api/v1/rts/rt-deact/deactivate')) {
        capturedMethod = init?.method
        return noContentResponse()
      }
      return failResponse(404, { code: 'not_found' })
    })
    await apiDeactivateRT('test-jwt', 'rt-deact')
    expect(capturedMethod).toBe('PATCH')
  })

  it('does NOT call deactivate endpoint for create or update', async () => {
    let deactivateCalled = false
    setupFetchMock(async (input) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/deactivate')) {
        deactivateCalled = true
      }
      if (url.includes('/api/v1/rts') && !url.includes('/deactivate')) {
        return createdResponse({
          id: 'rt-not-deact', name: 'RT 001', rw: 1, rt: '001',
          address: null, head_name: null, is_active: true,
          created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z',
        })
      }
      return failResponse(404, { code: 'not_found' })
    })
    await apiCreateRT('test-jwt', { name: 'RT 001', rw: 1, rt: '001' })
    expect(deactivateCalled).toBe(false)
  })

  it('throws ApiError on not found (404)', async () => {
    setupFetchMock(() => {
      return failResponse(404, { code: 'not_found', message: 'RT not found' })
    })
    try {
      await apiDeactivateRT('test-jwt', 'nonexistent')
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })

  it('throws ApiError on server error (500)', async () => {
    setupFetchMock(() => {
      return failResponse(500, { code: 'internal_error', message: 'server error' })
    })
    try {
      await apiDeactivateRT('jwt', 'rt-123')
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })
})

// ---- W3.3 — apiReactivateRT ----

describe('W3.3 — apiReactivateRT', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('reactivates RT and returns updated data', async () => {
    const rtData = {
      id: 'rt-react',
      name: 'RT 002',
      rw: 1,
      rt: '002',
      address: 'Jl. Reaktivasi',
      head_name: 'Eka',
      is_active: true,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-08-01T00:00:00Z',
    }
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts/rt-react')) {
        return okResponse(rtData)
      }
      return failResponse(404, { code: 'not_found' })
    })
    const result = await apiReactivateRT('test-jwt', 'rt-react')
    expect(result.id).toBe('rt-react')
    expect(result.is_active).toBe(true)
  })

  it('reactivates by calling update with is_active=true', async () => {
    let capturedBody: Record<string, unknown> | undefined
    setupFetchMock(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/api/v1/rts/rt-react')) {
        const text = await init?.body?.toString() || ''
        capturedBody = JSON.parse(text)
        return okResponse({
          id: 'rt-react', name: 'RT 002', rw: 1, rt: '002',
          address: null, head_name: null, is_active: true,
          created_at: '2024-01-01T00:00:00Z', updated_at: '2024-08-01T00:00:00Z',
        })
      }
      return failResponse(404, { code: 'not_found' })
    })
    await apiReactivateRT('test-jwt', 'rt-react')
    expect(capturedBody).toEqual({ is_active: true })
  })
})

// ---- W3.3 — Lifecycle: create → deactivate → reactivate → update ----

describe('W3.3 — RT mutation lifecycle', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('create, deactivate, reactivate in sequence', async () => {
    const created = {
      id: 'rt-333',
      name: 'RT Baru',
      rw: 3,
      rt: '003',
      address: null,
      head_name: null,
      is_active: true,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }
    setupFetchMock(async (input, _unusedInit) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/deactivate')) {
        return noContentResponse()
      }
      if (url.includes('/api/v1/rts/rt-333')) {
        return okResponse(created)
      }
      if (url.includes('/api/v1/rts')) {
        return createdResponse(created)
      }
      return failResponse(404, { code: 'not_found' })
    })

    const createResult = await apiCreateRT('test-jwt', { name: 'RT Baru', rw: 3, rt: '003' })
    expect(createResult?.is_active).toBe(true)

    await apiDeactivateRT('test-jwt', 'rt-333')

    const updateResult = await apiReactivateRT('test-jwt', 'rt-333')
    expect(updateResult.is_active).toBe(true)
  })
})
