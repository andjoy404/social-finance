import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  apiLogout,
  persistSessionPair,
  clearSessionPair,
  getSessionPair,
  ApiError,
} from '@/app/api'

// ---- Pure unit tests (no fetch involved) ----

describe('W2.5 — sessionStorage helpers', () => {
  beforeEach(() => { clearSessionPair() })
  afterEach(() => { clearSessionPair() })

  it('saves access + refresh token to sessionStorage', () => {
    persistSessionPair({ accessToken: 'at', refreshToken: 'rt' })
    const stored = getSessionPair()
    expect(stored).toEqual({ accessToken: 'at', refreshToken: 'rt' })
  })

  it('retrieves previously saved pair', () => {
    persistSessionPair({ accessToken: 'tok1', refreshToken: 'tok2' })
    expect(getSessionPair()?.accessToken).toBe('tok1')
    expect(getSessionPair()?.refreshToken).toBe('tok2')
  })

  it('clearSessionPair removes stored pair', () => {
    persistSessionPair({ accessToken: 'at', refreshToken: 'rt' })
    clearSessionPair()
    expect(getSessionPair()).toBeNull()
  })

  it('is idempotent when called on empty storage', () => {
    clearSessionPair()
    expect(getSessionPair()).toBeNull()
    clearSessionPair()
    expect(getSessionPair()).toBeNull()
  })
})

describe('W2.5 — malformed storage', () => {
  beforeEach(() => { clearSessionPair() })

  it('returns null when stored JSON has wrong shape', () => {
    sessionStorage.setItem('social-finance-session', JSON.stringify({ bad: true }))
    expect(getSessionPair()).toBeNull()
  })

  it('returns null when stored JSON is garbage', () => {
    sessionStorage.setItem('social-finance-session', 'not-json-at-all')
    expect(getSessionPair()).toBeNull()
  })

  it('returns null when stored JSON is missing access token field', () => {
    sessionStorage.setItem('social-finance-session', JSON.stringify({ refreshToken: 'rt' }))
    expect(getSessionPair()).toBeNull()
  })
})

// ---- Fetch helpers ----
function noContentResponse(): Response {
  return { ok: true, status: 204, text: () => Promise.resolve(''), json: () => Promise.resolve(null) } as Response
}

function failResponse(status: number, data: unknown): Response {
  return {
    ok: false,
    status: status,
    text: () => Promise.resolve(JSON.stringify(data)),
    json: () => Promise.resolve(data),
  } as Response
}

function notFound(): Response {
  return { ok: false, status: 404, text: () => Promise.resolve(''), json: () => Promise.resolve(null) } as Response
}

describe('W2.5 — apiLogout', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns successfully on 204', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(noContentResponse())
    await expect(apiLogout('valid-jwt')).resolves.toBeUndefined()
  })

  it('includes Bearer access token in authorization header', async () => {
    const calls: string[] = []
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : ''
      calls.push(url)
      if (url.includes('/api/v1/auth/logout')) {
        expect(init?.headers).toHaveProperty('Authorization', 'Bearer test-jwt-xyz')
        return noContentResponse()
      }
      return notFound()
    })
    await apiLogout('test-jwt-xyz')
    expect(calls).toHaveLength(1)
  })

  it('throws ApiError on 401 backend failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      failResponse(401, { code: 'token_expired', message: 'expired' }),
    )
    await expect(apiLogout('expired-jwt')).rejects.toThrow(ApiError)
  })

  it('throws ApiError on 403 backend failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      failResponse(403, { code: 'forbidden', message: 'no access' }),
    )
    await expect(apiLogout('invalid-jwt')).rejects.toThrow(ApiError)
  })

  it('throws ApiError on 500 backend failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      failResponse(500, { code: 'internal_error', message: 'server error' }),
    )
    await expect(apiLogout('jwt')).rejects.toThrow(ApiError)
  })
})

describe('W2.5 — logout flow (integration via apiLogout + storage)', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    clearSessionPair()
  })

  afterEach(() => {
    clearSessionPair()
    vi.restoreAllMocks()
  })

  it('successful logout clears session after', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(noContentResponse())
    persistSessionPair({ accessToken: 'at', refreshToken: 'rt' })
    await apiLogout('at')
    expect(getSessionPair()).not.toBeNull()
    clearSessionPair()
    expect(getSessionPair()).toBeNull()
  })

  it('backend logout failure still allows local cleanup', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      failResponse(500, { code: 'internal_error', message: 'server error' }),
    )
    persistSessionPair({ accessToken: 'at', refreshToken: 'rt' })
    expect(getSessionPair()).not.toBeNull()
    try {
      await apiLogout('at')
      expect.fail('should have thrown')
    } catch {
      /* expected */
    }
    expect(getSessionPair()).not.toBeNull()
    clearSessionPair()
    expect(getSessionPair()).toBeNull()
  })

  it('network failure allows local cleanup', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('network down'))
    persistSessionPair({ accessToken: 'at', refreshToken: 'rt' })
    try {
      await apiLogout('at')
      expect.fail('should have thrown')
    } catch {
      /* expected */
    }
    clearSessionPair()
    expect(getSessionPair()).toBeNull()
  })
})
