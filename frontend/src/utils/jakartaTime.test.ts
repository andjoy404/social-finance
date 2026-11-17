import { describe, expect, it } from 'vitest'
import {
  getJakartaTimeParts,
  JAKARTA_OFFSET,
  JAKARTA_TIMEZONE,
  parseIso8601ToEpoch,
  toJakartaIso8601,
  formatJakartaDateTime,
} from './jakartaTime'

describe('Jakarta timezone utility', () => {
  it('exports the correct IANA timezone identifier', () => {
    expect(JAKARTA_TIMEZONE).toBe('Asia/Jakarta')
  })

  it('exports the correct fixed offset for Jakarta (no DST)', () => {
    expect(JAKARTA_OFFSET).toBe('+07:00')
  })

  it('preserves the selected calendar date in the output', () => {
    const result = toJakartaIso8601('2026-10-22')
    expect(result.startsWith('2026-10-22T')).toBe(true)
  })

  it('always includes explicit +07:00 offset', () => {
    const result = toJakartaIso8601('2026-01-15')
    expect(result.endsWith('+07:00')).toBe(true)
  })

  it('produces a valid parseable ISO 8601 timestamp', () => {
    const result = toJakartaIso8601('2026-10-22')
    const parsed = new Date(result)
    expect(parsed.toISOString()).toBeDefined()
    expect(isNaN(parsed.getTime())).toBe(false)
  })

  it('does not shift the date when the selected date is the first of month', () => {
    // Edge case: if browser were in UTC-12, old `new Date('YYYY-MM-DD') + setHours`
    // could shift to the previous day. Our implementation must not.
    const result = toJakartaIso8601('2026-10-01')
    expect(result.startsWith('2026-10-01T')).toBe(true)
  })

  it('does not shift the date when the selected date is the last of month', () => {
    const result = toJakartaIso8601('2026-01-31')
    expect(result.startsWith('2026-01-31T')).toBe(true)
  })

  it('produces different instants for different dates', () => {
    const a = parseIso8601ToEpoch(toJakartaIso8601('2026-10-01'))
    const b = parseIso8601ToEpoch(toJakartaIso8601('2026-10-02'))
    // Same Jakarta time (approximately), later date should be later instant
    expect(b).toBeGreaterThan(a)
  })

  it('getJakartaTimeParts returns valid components', () => {
    const parts = getJakartaTimeParts()
    expect(parts.year).toBeGreaterThan(2000)
    expect(parts.month).toBeGreaterThanOrEqual(1)
    expect(parts.month).toBeLessThanOrEqual(12)
    expect(parts.day).toBeGreaterThanOrEqual(1)
    expect(parts.day).toBeLessThanOrEqual(31)
    expect(parts.hour).toBeGreaterThanOrEqual(0)
    expect(parts.hour).toBeLessThanOrEqual(23)
    expect(parts.minute).toBeGreaterThanOrEqual(0)
    expect(parts.minute).toBeLessThanOrEqual(59)
    expect(parts.second).toBeGreaterThanOrEqual(0)
    expect(parts.second).toBeLessThanOrEqual(59)
    expect(parts.millisecond).toBeGreaterThanOrEqual(0)
    expect(parts.millisecond).toBeLessThanOrEqual(999)
  })

  it('parseIso8601ToEpoch handles +07:00 offset and equivalent Z timestamp', () => {
    // 2026-10-22T10:30:00+07:00 == 2026-10-22T03:30:00Z
    const withOffset = parseIso8601ToEpoch('2026-10-22T10:30:00.000+07:00')
    const withZ = parseIso8601ToEpoch('2026-10-22T03:30:00.000Z')
    expect(withOffset).toBe(withZ)
    expect(isNaN(withOffset)).toBe(false)
  })

  it('parseIso8601ToEpoch returns NaN for invalid input', () => {
    expect(isNaN(parseIso8601ToEpoch(null))).toBe(true)
    expect(isNaN(parseIso8601ToEpoch(undefined))).toBe(true)
    expect(isNaN(parseIso8601ToEpoch(''))).toBe(true)
    expect(isNaN(parseIso8601ToEpoch('not-a-date'))).toBe(true)
  })

  describe('formatJakartaDateTime', () => {
    it('formats UTC ISO timestamp to Jakarta wall-clock time (UTC+7)', () => {
      // 01:05:45 UTC is 08:05:45 WIB
      const formatted = formatJakartaDateTime('2026-11-07T01:05:45Z')
      expect(formatted).toContain('2026')
      expect(formatted).toMatch(/08[.:]05/)
    })

    it('crosses midnight date boundary correctly from UTC to Jakarta', () => {
      // 2026-11-06T18:30:00Z is 2026-11-07 01:30:00 WIB
      const formatted = formatJakartaDateTime('2026-11-06T18:30:00Z')
      expect(formatted).toContain('7')
      expect(formatted).toMatch(/01[.:]30/)
    })

    it('handles explicit +07:00 offset ISO timestamp identically', () => {
      const fromUtc = formatJakartaDateTime('2026-11-07T01:05:45Z')
      const fromWib = formatJakartaDateTime('2026-11-07T08:05:45+07:00')
      expect(fromUtc).toBe(fromWib)
    })

    it('returns fallback string for null, undefined, empty, or invalid date', () => {
      expect(formatJakartaDateTime(null)).toBe('-')
      expect(formatJakartaDateTime(undefined)).toBe('-')
      expect(formatJakartaDateTime('')).toBe('-')
      expect(formatJakartaDateTime('invalid-date')).toBe('-')
      expect(formatJakartaDateTime(null, {}, 'N/A')).toBe('N/A')
    })
  })
})
