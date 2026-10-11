import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  apiUploadPaymentProof,
  apiGetPaymentProofBlob,
  apiCreatePayment,
  ApiError,
} from '@/app/api'

describe('Payment Proof API', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  describe('apiUploadPaymentProof', () => {
    it('should upload proof file via multipart formData to /api/v1/payments/:id/proof without manual Content-Type', async () => {
      let capturedUrl = ''
      let capturedInit: RequestInit | undefined

      globalThis.fetch = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
        capturedUrl = url
        capturedInit = init
        const bodyData = {
          id: 'pay-123',
          rt_id: 'rt-1',
          bill_id: 'bill-1',
          amount: '50000',
          method: 'TRANSFER',
          origin: 'SELF_SUBMITTED',
          status: 'PENDING',
          proof_path: 'proofs/rt-1/bill-1/proof.jpg',
          paid_at: '2026-10-12T00:00:00Z',
        }
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'content-type': 'application/json' }),
          text: async () => JSON.stringify(bodyData),
          json: async () => bodyData,
        } as unknown as Response
      })

      const file = new File(['fake content'], 'test-proof.jpg', { type: 'image/jpeg' })
      const res = await apiUploadPaymentProof('test-token', 'pay-123', file)

      expect(capturedUrl).toContain('/payments/pay-123/proof')
      expect(capturedInit?.method).toBe('POST')

      // Bearer token must be passed in header
      const headers = capturedInit?.headers as Record<string, string>
      expect(headers['Authorization']).toBe('Bearer test-token')

      // Must NOT manually specify Content-Type (browser/engine sets it with boundary)
      expect(headers['Content-Type']).toBeUndefined()

      // Body must be FormData containing the file
      expect(capturedInit?.body).toBeInstanceOf(FormData)
      const fd = capturedInit?.body as FormData
      expect(fd.get('file')).toBe(file)

      expect(res.id).toBe('pay-123')
      expect(res.proof_path).toBe('proofs/rt-1/bill-1/proof.jpg')
    })

    it('should throw ApiError if upload fails with error code', async () => {
      globalThis.fetch = vi.fn().mockImplementation(async () => {
        return {
          ok: false,
          status: 400,
          headers: new Headers({ 'content-type': 'application/json' }),
          text: async () => JSON.stringify({ error: { code: 'validation_error', message: 'file size exceeds 5MB limit' } }),
          json: async () => ({ error: { code: 'validation_error', message: 'file size exceeds 5MB limit' } }),
        } as unknown as Response
      })

      const file = new File(['oversized content'], 'large.png', { type: 'image/png' })
      await expect(apiUploadPaymentProof('token', 'pay-1', file)).rejects.toThrow(ApiError)
      await expect(apiUploadPaymentProof('token', 'pay-1', file)).rejects.toThrow('file size exceeds 5MB limit')
    })
  })

  describe('apiGetPaymentProofBlob', () => {
    it('should fetch binary proof using Authorization header and return blob', async () => {
      let capturedUrl = ''
      let capturedInit: RequestInit | undefined
      const mockBlob = new Blob(['image binary content'], { type: 'image/jpeg' })

      globalThis.fetch = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
        capturedUrl = url
        capturedInit = init
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'content-type': 'image/jpeg' }),
          blob: async () => mockBlob,
        } as unknown as Response
      })

      const result = await apiGetPaymentProofBlob('test-token', 'pay-123')

      expect(capturedUrl).toContain('/payments/pay-123/proof')
      expect(capturedInit?.method).toBe('GET')
      const headers = capturedInit?.headers as Record<string, string>
      expect(headers['Authorization']).toBe('Bearer test-token')
      expect(result.contentType).toBe('image/jpeg')
      expect(result.blob).toBe(mockBlob)
    })
  })

  describe('apiCreatePayment with multipart', () => {
    it('should use FormData when file is provided', async () => {
      let capturedInit: RequestInit | undefined

      globalThis.fetch = vi.fn().mockImplementation(async (_url: string, init?: RequestInit) => {
        capturedInit = init
        const bodyData = {
          id: 'pay-456',
          rt_id: 'rt-1',
          bill_id: 'bill-2',
          amount: '75000',
          method: 'TRANSFER',
          origin: 'SELF_SUBMITTED',
          status: 'PENDING',
          proof_path: 'proofs/rt-1/bill-2/proof.png',
          paid_at: '2026-10-12T00:00:00Z',
        }
        return {
          ok: true,
          status: 201,
          headers: new Headers({ 'content-type': 'application/json' }),
          text: async () => JSON.stringify(bodyData),
          json: async () => bodyData,
        } as unknown as Response
      })

      const file = new File(['image bytes'], 'receipt.png', { type: 'image/png' })
      const res = await apiCreatePayment('token-abc', {
        bill_id: 'bill-2',
        amount: '75000',
        method: 'TRANSFER',
        notes: 'Transfer via BCA',
        file,
      })

      expect(capturedInit?.body).toBeInstanceOf(FormData)
      const fd = capturedInit?.body as FormData
      expect(fd.get('bill_id')).toBe('bill-2')
      expect(fd.get('amount')).toBe('75000')
      expect(fd.get('method')).toBe('TRANSFER')
      expect(fd.get('notes')).toBe('Transfer via BCA')
      expect(fd.get('file')).toBe(file)

      expect(res.id).toBe('pay-456')
      expect(res.proof_path).toBe('proofs/rt-1/bill-2/proof.png')
    })
  })
})
