import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  apiListFundLocations,
  apiCreateFundLocation,
  apiGetFundLocationReconciliation,
  apiCreateFundTransfer,
} from './api'

const win = globalThis as unknown as { fetch: typeof fetch }

describe('Fund Location API Response Parsing', () => {
  const originalFetch = win.fetch

  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    win.fetch = originalFetch
  })

  it('apiListFundLocations: correctly unpacks { data: [...] }', async () => {
    const mockList = [
      {
        id: 'loc-1',
        rt_id: 'rt-1',
        name: 'BCA RT',
        category: 'rekening',
        is_active: true,
        current_balance: '100000.00',
      },
    ]

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ data: mockList }),
    } as Response)

    const result = await apiListFundLocations('token-123')
    expect(result).toHaveLength(1)
    expect(result[0].name).toBe('BCA RT')
  })

  it('apiListFundLocations: correctly handles raw array [...]', async () => {
    const mockList = [
      {
        id: 'loc-2',
        rt_id: 'rt-1',
        name: 'Brankas RT',
        category: 'brankas',
        is_active: true,
        current_balance: '50000.00',
      },
    ]

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify(mockList),
    } as Response)

    const result = await apiListFundLocations('token-123')
    expect(result).toHaveLength(1)
    expect(result[0].name).toBe('Brankas RT')
  })

  it('apiListFundLocations: throws ApiError Response data missing when body is invalid', async () => {
    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ invalid: 'structure' }),
    } as Response)

    await expect(apiListFundLocations('token-123')).rejects.toThrow('Response data missing')
  })

  it('apiCreateFundLocation: correctly unpacks Create Rekening response with { data: [...] }', async () => {
    const mockPayload = {
      data: [
        {
          id: '236d11af-cea4-4459-a572-eea678396fb1',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          name: 'BCA RT',
          category: 'rekening',
          bank_name: 'BCA',
          account_number: '1234567890',
          account_holder: 'Ketua RT',
          is_active: true,
          current_balance: '0.00',
          created_at: '2026-10-25T20:18:27.493442+07:00',
          updated_at: '2026-10-25T20:18:27.493442+07:00',
        },
      ],
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () => JSON.stringify(mockPayload),
    } as Response)

    const result = await apiCreateFundLocation('token-123', {
      name: 'BCA RT',
      category: 'rekening',
      bank_name: 'BCA',
    })

    expect(result.id).toBe('236d11af-cea4-4459-a572-eea678396fb1')
    expect(result.name).toBe('BCA RT')
    expect(result.category).toBe('rekening')
  })

  it('apiCreateFundLocation: correctly unpacks Create Brankas response with { data: {...} }', async () => {
    const mockPayload = {
      data: {
        id: 'd681926f-1aed-4630-b333-4252ea519812',
        rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        name: 'Brankas RT',
        category: 'brankas',
        account_holder: 'Ketua RT',
        is_active: true,
        current_balance: '0.00',
        created_at: '2026-10-25T20:19:09.863455+07:00',
        updated_at: '2026-10-25T20:19:09.863455+07:00',
      },
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () => JSON.stringify(mockPayload),
    } as Response)

    const result = await apiCreateFundLocation('token-123', {
      name: 'Brankas RT',
      category: 'brankas',
    })

    expect(result.id).toBe('d681926f-1aed-4630-b333-4252ea519812')
    expect(result.name).toBe('Brankas RT')
    expect(result.category).toBe('brankas')
  })

  it('apiCreateFundLocation: handles raw object without data wrapper', async () => {
    const mockPayload = {
      id: 'loc-raw',
      rt_id: 'rt-1',
      name: 'COH RT',
      category: 'coh',
      is_active: true,
      current_balance: '0.00',
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () => JSON.stringify(mockPayload),
    } as Response)

    const result = await apiCreateFundLocation('token-123', {
      name: 'COH RT',
      category: 'coh',
    })

    expect(result.id).toBe('loc-raw')
    expect(result.name).toBe('COH RT')
  })

  it('apiCreateFundLocation: throws ApiError when response data is empty or missing id', async () => {
    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () => JSON.stringify({ data: [] }),
    } as Response)

    await expect(
      apiCreateFundLocation('token-123', { name: 'Empty', category: 'brankas' }),
    ).rejects.toThrow('Response data missing')
  })

  it('apiCreateFundLocation: throws ApiError when array has multiple items', async () => {
    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () =>
        JSON.stringify({
          data: [
            { id: 'loc-1', name: 'Item 1' },
            { id: 'loc-2', name: 'Item 2' },
          ],
        }),
    } as Response)

    await expect(
      apiCreateFundLocation('token-123', { name: 'Multiple', category: 'rekening' }),
    ).rejects.toThrow('Response data missing')
  })

  it('apiCreateFundLocation: throws ApiError when payload has no valid id string', async () => {
    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () => JSON.stringify({ data: { name: 'No ID' } }),
    } as Response)

    await expect(
      apiCreateFundLocation('token-123', { name: 'No ID', category: 'brankas' }),
    ).rejects.toThrow('Response data missing')
  })

  it('apiGetFundLocationReconciliation: correctly unpacks { data: {...} }', async () => {
    const mockPayload = {
      data: {
        kas_balance: '325000.00',
        total_allocated_balance: '0.00',
        unallocated_balance: '325000.00',
        discrepancy: '0.00',
        status: 'MATCH_WITH_NOTES',
        status_label: 'Cocok dengan Catatan',
        locations_summary: [
          {
            category: 'rekening',
            total_balance: '0.00',
            locations_count: 1,
          },
        ],
        anomalies_detected: null,
      },
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify(mockPayload),
    } as Response)

    const result = await apiGetFundLocationReconciliation('token-123')
    expect(result.kas_balance).toBe('325000.00')
    expect(result.unallocated_balance).toBe('325000.00')
    expect(result.locations_summary).toHaveLength(1)
  })

  it('apiCreateFundTransfer: correctly unpacks { data: {...} }', async () => {
    const mockPayload = {
      data: {
        id: 'tr-1',
        from_location_id: 'loc-1',
        to_location_id: 'loc-2',
        amount: '50000.00',
        transfer_date: '2026-10-25T20:20:00Z',
      },
    }

    win.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () => JSON.stringify(mockPayload),
    } as Response)

    const result = await apiCreateFundTransfer('token-123', {
      from_location_id: 'loc-1',
      to_location_id: 'loc-2',
      amount: '50000.00',
    })

    expect(result.id).toBe('tr-1')
    expect(result.amount).toBe('50000.00')
  })
})
