import { describe, expect, it } from 'vitest'
import { normalizeMonitoringResponse } from './api'
import sharedMonitoringResponse from '../testdata/monitoring-api-response.json'

describe('Monitoring API response compatibility', () => {
  const household = {
    household_occupancy_id: 'occupancy-1',
    house_number: 'A-01',
  }

  it('normalizes the backend data wrapper used by Android', () => {
    expect(normalizeMonitoringResponse({
      data: [household],
      total: 1,
      eval_date: '2026-11-06',
    })).toEqual({
      households: [household],
      total: 1,
      eval_date: '2026-11-06',
    })
  })

  it('continues to accept the legacy households wrapper and empty lists', () => {
    expect(normalizeMonitoringResponse({ households: [household] }).households)
      .toEqual([household])
    expect(normalizeMonitoringResponse({ data: [], total: 0 }).total).toBe(0)
  })

  it('rejects malformed responses rather than showing a false empty state', () => {
    expect(() => normalizeMonitoringResponse({ total: 1 })).toThrow(
      'Monitoring list missing from response',
    )
  })

  it('uses the shared backend fixture for the same RT list and total as Android', () => {
    const parsed = normalizeMonitoringResponse(sharedMonitoringResponse)
    expect(parsed.total).toBe(2)
    expect(parsed.households.map((row) => [row.household_occupancy_id, row.house_number]))
      .toEqual([
        ['occupancy-a', 'A-01'],
        ['occupancy-b', 'B-07'],
      ])
    expect(sharedMonitoringResponse.data[0]).not.toHaveProperty('outstanding')
    expect(sharedMonitoringResponse.data[0]).not.toHaveProperty('bill_status')
    expect(sharedMonitoringResponse.data[0]).not.toHaveProperty('period')
  })
})
