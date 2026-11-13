import { describe, it, expect, vi, beforeEach } from 'vitest'
import dayjs from 'dayjs'
import customParseFormat from 'dayjs/plugin/customParseFormat'
import {
  formatTimeHHMM,
  apiCreateSecurityShift,
  apiUpdateSecurityShift,
  type ShiftConfig,
} from './securityApi'
import * as apiModule from '@/app/api'

dayjs.extend(customParseFormat)

describe('Security Shift Time Normalization (formatTimeHHMM)', () => {
  it('normalizes Dayjs objects to HH:mm string', () => {
    expect(formatTimeHHMM(dayjs('08:00', 'HH:mm'))).toBe('08:00')
    expect(formatTimeHHMM(dayjs('16:00', 'HH:mm'))).toBe('16:00')
    expect(formatTimeHHMM(dayjs('23:00', 'HH:mm'))).toBe('23:00')
    expect(formatTimeHHMM(dayjs('00:00', 'HH:mm'))).toBe('00:00')
  })

  it('normalizes native JS Date objects using local time without ISO conversion', () => {
    const dMorning = new Date(2026, 10, 14, 8, 0, 0)
    const dAfternoon = new Date(2026, 10, 14, 16, 0, 0)
    const dNight = new Date(2026, 10, 14, 23, 0, 0)
    const dMidnight = new Date(2026, 10, 14, 0, 0, 0)

    expect(formatTimeHHMM(dMorning)).toBe('08:00')
    expect(formatTimeHHMM(dAfternoon)).toBe('16:00')
    expect(formatTimeHHMM(dNight)).toBe('23:00')
    expect(formatTimeHHMM(dMidnight)).toBe('00:00')
  })

  it('normalizes backend HH:MM:SS strings to HH:MM', () => {
    expect(formatTimeHHMM('08:00:00')).toBe('08:00')
    expect(formatTimeHHMM('16:00:00')).toBe('16:00')
    expect(formatTimeHHMM('23:00:00')).toBe('23:00')
    expect(formatTimeHHMM('00:00:00')).toBe('00:00')
  })

  it('preserves valid HH:MM strings', () => {
    expect(formatTimeHHMM('08:00')).toBe('08:00')
    expect(formatTimeHHMM('16:00')).toBe('16:00')
    expect(formatTimeHHMM('23:00')).toBe('23:00')
    expect(formatTimeHHMM('00:00')).toBe('00:00')
  })

  it('handles single digit hours by zero-padding', () => {
    expect(formatTimeHHMM('8:00')).toBe('08:00')
    expect(formatTimeHHMM('7:30:00')).toBe('07:30')
  })

  it('handles null, undefined, and empty string safely', () => {
    expect(formatTimeHHMM(null)).toBe('')
    expect(formatTimeHHMM(undefined)).toBe('')
    expect(formatTimeHHMM('')).toBe('')
  })

  it('prevents timezone shifting: local 23:00 remains 23:00, 00:00 remains 00:00', () => {
    const local23 = dayjs().hour(23).minute(0).second(0)
    const local00 = dayjs().hour(0).minute(0).second(0)

    expect(formatTimeHHMM(local23)).toBe('23:00')
    expect(formatTimeHHMM(local00)).toBe('00:00')

    // Confirms no ISO-8601 formatting or UTC conversion occurred
    const result23 = formatTimeHHMM(local23)
    expect(result23).not.toContain('T')
    expect(result23).not.toContain('Z')
    expect(result23).not.toMatch(/^\d{4}-\d{2}-\d{2}/)
  })
})

describe('Security Shift API Payloads (apiCreateSecurityShift & apiUpdateSecurityShift)', () => {
  let capturedBody: string | undefined

  beforeEach(() => {
    capturedBody = undefined
    vi.spyOn(apiModule, 'authenticatedApiRequest').mockImplementation(
      async (_url, init) => {
        capturedBody = init?.body as string | undefined
        return {
          ok: true,
          status: 200,
          text: async () =>
            JSON.stringify({
              id: 'shift-123',
              name: 'Shift Test',
              start_time: '08:00:00',
              end_time: '16:00:00',
            } as ShiftConfig),
        } as unknown as Response
      },
    )
  })

  it('apiCreateSecurityShift sends 08:00 and 16:00 when given Dayjs instances without ISO-8601 timestamp', async () => {
    await apiCreateSecurityShift({
      name: 'Shift Pagi',
      start_time: dayjs('08:00', 'HH:mm'),
      end_time: dayjs('16:00', 'HH:mm'),
      crosses_midnight: false,
      lateness_tolerance_minutes: 15,
      check_in_window_before_minutes: 30,
      check_in_window_after_minutes: 60,
      absent_cutoff_minutes: 60,
      missing_checkout_cutoff_minutes: 120,
      is_active: true,
    })

    expect(capturedBody).toBeDefined()
    const parsed = JSON.parse(capturedBody!)

    expect(parsed.start_time).toBe('08:00')
    expect(parsed.end_time).toBe('16:00')
    expect(capturedBody).not.toContain('T')
    expect(capturedBody).not.toContain('Z')
    expect(capturedBody).not.toContain('.000Z')
    expect(capturedBody).not.toMatch(/\d{4}-\d{2}-\d{2}T/)
  })

  it('apiUpdateSecurityShift sends 08:00 and 16:00 without ISO-8601 timestamp', async () => {
    await apiUpdateSecurityShift('shift-1', {
      name: 'Shift Pagi Revisi',
      start_time: dayjs('08:00', 'HH:mm'),
      end_time: dayjs('16:00', 'HH:mm'),
      crosses_midnight: false,
      lateness_tolerance_minutes: 15,
      check_in_window_before_minutes: 30,
      check_in_window_after_minutes: 60,
      absent_cutoff_minutes: 60,
      missing_checkout_cutoff_minutes: 120,
      is_active: true,
    })

    expect(capturedBody).toBeDefined()
    const parsed = JSON.parse(capturedBody!)

    expect(parsed.start_time).toBe('08:00')
    expect(parsed.end_time).toBe('16:00')
    expect(capturedBody).not.toContain('T')
    expect(capturedBody).not.toContain('Z')
    expect(capturedBody).not.toContain('.000Z')
    expect(capturedBody).not.toMatch(/\d{4}-\d{2}-\d{2}T/)
  })

  it('apiCreateSecurityShift preserves 23:00 and 00:00 across midnight without timezone shift', async () => {
    await apiCreateSecurityShift({
      name: 'Shift Malam',
      start_time: dayjs('23:00', 'HH:mm'),
      end_time: dayjs('07:00', 'HH:mm'),
      crosses_midnight: true,
      lateness_tolerance_minutes: 15,
      check_in_window_before_minutes: 30,
      check_in_window_after_minutes: 60,
      absent_cutoff_minutes: 60,
      missing_checkout_cutoff_minutes: 120,
      is_active: true,
    })

    expect(capturedBody).toBeDefined()
    const parsed = JSON.parse(capturedBody!)

    expect(parsed.start_time).toBe('23:00')
    expect(parsed.end_time).toBe('07:00')
    expect(capturedBody).not.toMatch(/\d{4}-\d{2}-\d{2}T/)
  })

  it('apiUpdateSecurityShift normalizes native Date objects to 00:00 and 08:00', async () => {
    const dStart = new Date(2026, 10, 14, 0, 0, 0)
    const dEnd = new Date(2026, 10, 14, 8, 0, 0)

    await apiUpdateSecurityShift('shift-2', {
      name: 'Shift Dini Hari',
      start_time: dStart,
      end_time: dEnd,
      crosses_midnight: false,
      lateness_tolerance_minutes: 15,
      check_in_window_before_minutes: 30,
      check_in_window_after_minutes: 60,
      absent_cutoff_minutes: 60,
      missing_checkout_cutoff_minutes: 120,
      is_active: true,
    })

    expect(capturedBody).toBeDefined()
    const parsed = JSON.parse(capturedBody!)

    expect(parsed.start_time).toBe('00:00')
    expect(parsed.end_time).toBe('08:00')
    expect(capturedBody).not.toContain('T')
    expect(capturedBody).not.toContain('Z')
  })
})
