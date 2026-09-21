import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { apiLogin, ApiError } from '@/app/api'

const win = globalThis as unknown as { fetch: typeof fetch }

describe('ApiError', () => {
  it('should have correct name', () => {
    const err = new ApiError('test_code', 'test message')
    expect(err.name).toBe('ApiError')
    expect(err.code).toBe('test_code')
    expect(err.message).toBe('test message')
  })

  it('should extend Error', () => {
    const err = new ApiError('code', 'msg')
    expect(err).toBeInstanceOf(Error)
  })
})

describe('apiLogin', () => {
  const originalFetch = win.fetch

  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    win.fetch = originalFetch
  })

  it('should return user data on successful login', async () => {
    const mockResponse = {
      access_token: 'mock-jwt-token',
      refresh_token: 'mock-refresh-token',
      token_type: 'Bearer',
      expires_in: 900,
      user: {
        id: 'user-123',
        full_name: 'Test User',
        email: 'test@example.com',
        system_role: 'super_admin',
        role: '',
        rt: null,
      },
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      text: () => Promise.resolve(JSON.stringify(mockResponse)),
    })

    const result = await apiLogin('test@example.com', 'password')

    expect(result.access_token).toBe('mock-jwt-token')
    expect(result.refresh_token).toBe('mock-refresh-token')
    expect(result.user.full_name).toBe('Test User')
    expect(result.user.email).toBe('test@example.com')

    const callArgs = (win.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit
    expect(callArgs?.method).toBe('POST')
    expect(callArgs?.body).toBe(JSON.stringify({ email: 'test@example.com', password: 'password' }))
  })

  it('should throw ApiError with invalid_credentials on 401', async () => {
    const errorResponse = { code: 'invalid_credentials', message: 'email or password is incorrect' }

    win.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve(errorResponse),
      text: () => Promise.resolve(JSON.stringify(errorResponse)),
    })

    await expect(apiLogin('wrong@example.com', 'wrong'))
      .rejects
      .toThrow(ApiError)

    try {
      await apiLogin('wrong@example.com', 'wrong')
    } catch (err) {
      if (err instanceof ApiError) {
        expect(err.code).toBe('invalid_credentials')
        expect(err.message).toBe('email or password is incorrect')
      } else {
        throw err
      }
    }
  })

  it('should throw ApiError with not_authorized on 401', async () => {
    const errorResponse = { code: 'not_authorized', message: 'no active memberships' }

    win.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve(errorResponse),
      text: () => Promise.resolve(JSON.stringify(errorResponse)),
    })

    try {
      await apiLogin('inactive@example.com', 'password')
    } catch (err) {
      if (err instanceof ApiError) {
        expect(err.code).toBe('not_authorized')
      } else {
        throw err
      }
    }
  })

  it('should throw ApiError with multiple_memberships on 409', async () => {
    const errorResponse = {
      code: 'multiple_memberships',
      message: 'multiple active memberships found',
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: () => Promise.resolve(errorResponse),
      text: () => Promise.resolve(JSON.stringify(errorResponse)),
    })

    try {
      await apiLogin('multi@example.com', 'password')
    } catch (err) {
      if (err instanceof ApiError) {
        expect(err.code).toBe('multiple_memberships')
      } else {
        throw err
      }
    }
  })

  it('should throw ApiError with unexpected code on network error', async () => {
    win.fetch = vi.fn().mockRejectedValue(new TypeError('Network failed'))

    try {
      await apiLogin('offline@example.com', 'password')
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(TypeError)
    }
  })

  it('should throw ApiError with default message when response has no body', async () => {
    win.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve(null),
      text: () => Promise.resolve(''),
    })

    try {
      await apiLogin('error@example.com', 'password')
    } catch (err) {
      if (err instanceof ApiError) {
        expect(err.code).toBe('unexpected')
        expect(err.message).toBe('Login failed')
      } else {
        throw err
      }
    }
  })

  it('should include super_admin system_role when returned by backend', async () => {
    const mockResponse = {
      access_token: 'jwt',
      refresh_token: '',
      token_type: 'Bearer',
      expires_in: 900,
      user: {
        id: 'sys-admin-1',
        full_name: 'System Admin',
        email: 'admin@system.local',
        system_role: 'super_admin',
        role: '',
        rt: null,
      },
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      text: () => Promise.resolve(JSON.stringify(mockResponse)),
    })

    const result = await apiLogin('admin@system.local', 'password')

    expect(result.user.system_role).toBe('super_admin')
    expect(result.user.role).toBe('')
    expect(result.user.rt).toBeNull()
  })

  it('should return empty system_role when not provided by backend', async () => {
    const mockResponse = {
      access_token: 'jwt',
      refresh_token: 'refresh',
      token_type: 'Bearer',
      expires_in: 900,
      user: {
        id: 'user-abc',
        full_name: 'Bendahara User',
        email: 'bendahara@rt.local',
        role: 'bendahara',
        rt: { id: 'rt-1', name: 'RT 001' },
      },
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      text: () => Promise.resolve(JSON.stringify(mockResponse)),
    })

    const result = await apiLogin('bendahara@rt.local', 'password')

    expect(result.user.system_role).toBeUndefined()
    expect(result.user.role).toBe('bendahara')
    expect(result.user.rt?.name).toBe('RT 001')
  })
})
