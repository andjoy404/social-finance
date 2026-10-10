// Regression tests for nested error parsing (Bug B).
//
// Backend error response format:
//   { "error": { "code": "validation_error", "message": "..." } }
//
// authenticatedApiRequest must unwrap body.error before reading code/message.

import { describe, it, expect, vi, beforeEach } from 'vitest'

function failResponse(status: number, data: unknown): Response {
  return {
    ok: false,
    status,
    text: () => Promise.resolve(JSON.stringify(data)),
    json: () => Promise.resolve(data),
  } as Response
}

function okResponse(data: unknown): Response {
  const body = JSON.stringify(data)
  return {
    ok: true,
    status: 201,
    text: () => Promise.resolve(body),
    json: () => Promise.resolve(data),
  } as Response
}

function setupFetchMock(fn: (input: string | URL | Request, init?: RequestInit) => Response | Promise<Response>) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => fn(input as string, init))
}

describe('authenticatedApiRequest — nested error parsing (Bug B regression)', () => {
  let mod: typeof import('./api')

  beforeEach(async () => {
    mod = await import('./api')
    vi.clearAllMocks()
  })

  it('parses nested { error: { code, message } } on 400 validation error', async () => {
    setupFetchMock((_input, _init) =>
      failResponse(400, {
        error: { code: 'validation_error', message: 'amount must be greater than zero' },
      })
    )

    await expect(
      mod.apiCreateDue('fake-token', { name: 'x', amount: '-1', period_type: 'monthly' })
    ).rejects.toThrow('amount must be greater than zero')
  })

  it('throws ApiError with code from nested error on 400', async () => {
    setupFetchMock((_input, _init) =>
      failResponse(400, {
        error: { code: 'validation_error', message: 'name is required' },
      })
    )

    try {
      await mod.apiCreateDue('fake-token', { name: '', amount: '1000', period_type: 'monthly' })
      expect.fail('should have thrown')
    } catch (err: unknown) {
      expect(err).toBeInstanceOf(Error)
      const apiErr = err as import('./api').ApiError
      expect(apiErr.code).toBe('validation_error')
    }
  })

  it('throws "Unexpected response" when backend returns error without nested structure', async () => {
    setupFetchMock((_input, _init) =>
      failResponse(400, { code: 'bad', message: 'direct' })
    )

    try {
      await mod.apiCreateDue('fake-token', { name: 'x', amount: '1', period_type: 'monthly' })
      expect.fail('should have thrown')
    } catch (err: unknown) {
      const apiErr = err as import('./api').ApiError
      expect(apiErr.code).toBe('unexpected')
    }
  })

  it('throws "Unexpected response" when error body is not JSON', async () => {
    setupFetchMock((_input, _init) => ({
      ok: false,
      status: 500,
      text: () => Promise.resolve('Internal Server Error'),
    } as Response))

    try {
      await mod.apiCreateDue('fake-token', { name: 'x', amount: '1', period_type: 'monthly' })
      expect.fail('should have thrown')
    } catch (err: unknown) {
      const apiErr = err as import('./api').ApiError
      expect(apiErr.code).toBe('unexpected')
    }
  })

  it('throws "Unexpected response" when error body is empty', async () => {
    setupFetchMock((_input, _init) => ({
      ok: false,
      status: 500,
      text: () => Promise.resolve(''),
    } as Response))

    try {
      await mod.apiCreateDue('fake-token', { name: 'x', amount: '1', period_type: 'monthly' })
      expect.fail('should have thrown')
    } catch (err: unknown) {
      const apiErr = err as import('./api').ApiError
      expect(apiErr.code).toBe('unexpected')
    }
  })

  it('returns data successfully for 201 created', async () => {
    setupFetchMock((_input, _init) =>
      okResponse({
        id: '123',
        name: 'Iuran Keamanan',
        amount: '50000.00',
        period_type: 'monthly',
        is_active: true,
        created_at: '2026-10-01T00:00:00Z',
        updated_at: '2026-10-01T00:00:00Z',
      })
    )

    const result = await mod.apiCreateDue('fake-token', {
      name: 'Iuran Keamanan',
      amount: '50000',
      period_type: 'monthly',
    })
    expect(result).toBeDefined()
    expect(result.id).toBe('123')
  })
})
