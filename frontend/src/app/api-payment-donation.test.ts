import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiCreatePayment } from './api'

describe('voluntary donation API request', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => vi.restoreAllMocks())
  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('posts due_id, positive amount and idempotency key as JSON', async () => {
    let capturedUrl = ''
    let capturedInit: RequestInit | undefined
    globalThis.fetch = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      capturedUrl = url
      capturedInit = init
      return new Response(JSON.stringify({
        id: 'payment-1',
        rt_id: 'rt-1',
        bill_id: null,
        due_id: 'due-1',
        amount: '12500.00',
        method: 'TRANSFER',
        origin: 'SELF_SUBMITTED',
        status: 'PENDING',
      }), { status: 201, headers: { 'Content-Type': 'application/json' } })
    })

    const payment = await apiCreatePayment('token', {
      due_id: 'due-1',
      amount: '12500',
      method: 'TRANSFER',
      notes: 'Donasi',
    }, 'stable-retry-key')

    expect(capturedUrl).toContain('/payments')
    expect(capturedInit?.method).toBe('POST')
    expect((capturedInit?.headers as Record<string, string>)['Idempotency-Key']).toBe('stable-retry-key')
    expect(JSON.parse(String(capturedInit?.body))).toEqual({
      amount: '12500',
      due_id: 'due-1',
      method: 'TRANSFER',
      notes: 'Donasi',
    })
    expect(payment.bill_id).toBeNull()
    expect(payment.due_id).toBe('due-1')
    expect(payment.status).toBe('PENDING')
  })

  it('includes due_id in multipart requests and reports server validation errors', async () => {
    let formData: FormData | undefined
    let nextResponse: Response
    globalThis.fetch = vi.fn().mockImplementation(async (_url: string, init?: RequestInit) => {
      if (init?.body instanceof FormData) formData = init.body
      return nextResponse
    })

    const file = new File(['proof'], 'proof.png', { type: 'image/png' })
    nextResponse = new Response(JSON.stringify({
      id: 'payment-2',
      rt_id: 'rt-1',
      due_id: 'due-1',
      bill_id: null,
      amount: '1.00',
      method: 'CASH',
      origin: 'SELF_SUBMITTED',
      status: 'PENDING',
    }), { status: 201, headers: { 'Content-Type': 'application/json' } })

    await apiCreatePayment('token', {
      due_id: 'due-1',
      amount: '1',
      method: 'CASH',
      file,
    }, 'multipart-key')
    expect(formData?.get('due_id')).toBe('due-1')
    expect(formData?.get('bill_id')).toBeNull()

    nextResponse = new Response(JSON.stringify({
      error: { code: 'validation_error', message: 'Nominal harus positif' },
    }), { status: 400, headers: { 'Content-Type': 'application/json' } })
    await expect(apiCreatePayment('token', {
      due_id: 'due-1',
      amount: '0',
      method: 'TRANSFER',
    }, 'invalid-key')).rejects.toThrow('Nominal harus positif')
  })

  it('rejects requests with both or neither payment target', async () => {
    await expect(apiCreatePayment('token', {
      amount: '1000',
      method: 'CASH',
    })).rejects.toThrow('Pilih tepat satu target')
    await expect(apiCreatePayment('token', {
      bill_id: 'bill-1',
      due_id: 'due-1',
      amount: '1000',
      method: 'CASH',
    })).rejects.toThrow('Pilih tepat satu target')
  })
})
