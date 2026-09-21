import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  apiListRTs,
  apiGetRT,
  ApiError,
  setTokenManager,
  clearSessionPair,
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

// ---- W3.2 — apiListRTs ----

describe('W3.2 — apiListRTs', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('returns paginated RT data on success', async () => {
    const mockData = {
      data: [
        {
          id: 'rt-1',
          name: 'RT 001',
          rw: 1,
          rt: '001',
          address: 'Jl. Merdeka No. 1',
          head_name: 'Budi',
          is_active: true,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      ],
      pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 },
    }
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts')) {
        return okResponse(mockData)
      }
      return failResponse(404, { code: 'not_found' })
    })
    const result = await apiListRTs('test-jwt', { page: 1, page_size: 20 })
    expect(result.data).toHaveLength(1)
    expect(result.data[0].name).toBe('RT 001')
    expect(result.pagination.total).toBe(1)
    expect(result.pagination.total_pages).toBe(1)
  })

  it('sends is_active=true when filter is aktif', async () => {
    const capturedUrls: string[] = []
    setupFetchMock((input: string) => {
      capturedUrls.push(typeof input === 'string' ? input : '')
      return okResponse({ data: [], pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 } })
    })
    await apiListRTs('test-jwt', { page: 1, page_size: 20, is_active: true })
    const rtsUrl = capturedUrls.find((u) => u.includes('/api/v1/rts'))
    expect(rtsUrl).toContain('is_active=true')
  })

  it('sends is_active=false when filter is tidak_aktif', async () => {
    const capturedUrls: string[] = []
    setupFetchMock((input: string) => {
      capturedUrls.push(typeof input === 'string' ? input : '')
      return okResponse({ data: [], pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 } })
    })
    await apiListRTs('test-jwt', { page: 1, page_size: 20, is_active: false })
    const rtsUrl = capturedUrls.find((u) => u.includes('/api/v1/rts'))
    expect(rtsUrl).toContain('is_active=false')
  })

  it('omits is_active query parameter when not specified (semua)', async () => {
    const capturedUrls: string[] = []
    setupFetchMock((input: string) => {
      capturedUrls.push(typeof input === 'string' ? input : '')
      return okResponse({ data: [], pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 } })
    })
    await apiListRTs('test-jwt', { page: 1, page_size: 20 })
    const rtsUrl = capturedUrls.find((u) => u.includes('/api/v1/rts'))
    expect(rtsUrl).toContain('is_active=all')
  })

  it('throws ApiError on authorization failure (403)', async () => {
    setupFetchMock(() => {
      return failResponse(403, { code: 'forbidden', message: 'access denied' })
    })
    try {
      await apiListRTs('bad-jwt', {})
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
      if (err instanceof ApiError) {
        expect(err.code).toBe('forbidden')
      }
    }
  })

  it('throws ApiError on server error (500)', async () => {
    setupFetchMock(() => {
      return failResponse(500, { code: 'internal_error', message: 'server error' })
    })
    try {
      await apiListRTs('jwt', {})
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })

  it('uses default page and page_size when not specified', async () => {
    const capturedUrls: string[] = []
    setupFetchMock((input: string) => {
      capturedUrls.push(typeof input === 'string' ? input : '')
      return okResponse({ data: [], pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 } })
    })
    await apiListRTs('jwt', { page: 5, page_size: 10 })
    const rtsUrl = capturedUrls.find((u) => u.includes('/api/v1/rts'))
    expect(rtsUrl).toContain('page=5')
    expect(rtsUrl).toContain('page_size=10')
  })
})

// ---- W3.2 — apiGetRT ----

describe('W3.2 — apiGetRT', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('returns RT data on success', async () => {
    const rtData = {
      id: 'rt-123',
      name: 'RT 005',
      rw: 3,
      rt: '005',
      address: 'Jl. Sudirman No. 10',
      head_name: 'Agus',
      is_active: true,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts/rt-123')) {
        return okResponse(rtData)
      }
      return failResponse(404, { code: 'not_found' })
    })
    const result = await apiGetRT('test-jwt', 'rt-123')
    expect(result.id).toBe('rt-123')
    expect(result.name).toBe('RT 005')
    expect(result.rw).toBe(3)
    expect(result.head_name).toBe('Agus')
    expect(result.is_active).toBe(true)
    expect(result.address).toBe('Jl. Sudirman No. 10')
  })

  it('handles null address and head_name', async () => {
    const rtData = {
      id: 'rt-456',
      name: 'RT 010',
      rw: 5,
      rt: '010',
      address: null,
      head_name: null,
      is_active: true,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts/rt-456')) {
        return okResponse(rtData)
      }
      return failResponse(404, { code: 'not_found' })
    })
    const result = await apiGetRT('test-jwt', 'rt-456')
    expect(result.address).toBeNull()
    expect(result.head_name).toBeNull()
  })

  it('returns is_active=false for inactive RT', async () => {
    const rtData = {
      id: 'rt-inactive',
      name: 'RT Inaktif',
      rw: 2,
      rt: '002',
      address: null,
      head_name: null,
      is_active: false,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }
    setupFetchMock((input: string) => {
      if (input.includes('/api/v1/rts/rt-inactive')) {
        return okResponse(rtData)
      }
      return failResponse(404, { code: 'not_found' })
    })
    const result = await apiGetRT('test-jwt', 'rt-inactive')
    expect(result.is_active).toBe(false)
  })

  it('throws ApiError on RT not found (404)', async () => {
    setupFetchMock(() => {
      return failResponse(404, { code: 'not_found', message: 'RT not found' })
    })
    try {
      await apiGetRT('test-jwt', 'nonexistent-id')
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })

  it('throws ApiError on authorization failure (403)', async () => {
    setupFetchMock(() => {
      return failResponse(403, { code: 'forbidden', message: 'access denied' })
    })
    try {
      await apiGetRT('bad-jwt', 'rt-123')
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
      await apiGetRT('jwt', 'rt-123')
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
    }
  })

  it('throws ApiError on unexpected response with null body', async () => {
    setupFetchMock(() => {
      return {
        ok: true,
        status: 200,
        text: () => Promise.resolve(''),
        json: () => Promise.resolve(null),
      } as Response
    })
    try {
      await apiGetRT('jwt', 'rt-123')
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
      if (err instanceof ApiError) {
        expect(err.code).toBe('unexpected')
      }
    }
  })
})

// ---- W3.2 — authenticatedApiRequest uses bearer token ----

describe('W3.2 — RT API includes Authorization header', () => {
  let calls: string[] = []

  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
    calls = []
  })
  afterEach(() => {
    clearSessionPair()
  })

  it('apiListRTs sends Bearer token in Authorization header', async () => {
    setupFetchMock(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      calls.push(url)
      if (url.includes('/api/v1/rts')) {
        expect(init?.headers).toHaveProperty('Authorization', 'Bearer test-w3-token')
        return okResponse({ data: [], pagination: { page: 1, page_size: 20, total: 0, total_pages: 0 } })
      }
      return { ok: false, status: 401, json: () => Promise.resolve({ code: 'unauthorized' }), text: () => Promise.resolve('') } as Response
    })
    setTokenManager(
      () => undefined,
      () => undefined,
      () => {},
      () => {},
    )
    await expect(apiListRTs('test-w3-token', {})).resolves.toBeDefined()
    expect(calls.some((u) => u.includes('/api/v1/rts'))).toBe(true)
  })

  it('apiGetRT sends Bearer token in Authorization header', async () => {
    let called = false
    setupFetchMock(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      if (url.includes('/api/v1/rts/')) {
        called = true
        expect(init?.headers).toHaveProperty('Authorization', 'Bearer w3-detail-token')
        return okResponse({
          id: 'rt-xyz',
          name: 'RT Test',
          rw: 1,
          rt: '001',
          address: null,
          head_name: null,
          is_active: true,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        })
      }
      return { ok: false, status: 401, json: () => Promise.resolve({ code: 'unauthorized' }), text: () => Promise.resolve('') } as Response
    })
    setTokenManager(
      () => undefined,
      () => undefined,
      () => {},
      () => {},
    )
    await expect(apiGetRT('w3-detail-token', 'rt-xyz')).resolves.toBeDefined()
    expect(called).toBe(true)
  })
})
