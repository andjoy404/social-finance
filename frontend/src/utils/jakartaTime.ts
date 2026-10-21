/**
 * Jakarta (Asia/Jakarta, WIB, UTC+07:00) timezone utilities for React web.
 *
 * Indonesia does NOT observe daylight saving time, so the offset is always +07:00.
 * All manual KAS transactions must use Jakarta calendar date and Jakarta wall-clock time,
 * then be converted to an unambiguous ISO 8601 timestamp for the backend.
 */

/** IANA timezone identifier for Jakarta. */
export const JAKARTA_TIMEZONE = 'Asia/Jakarta';

/** Fixed UTC offset for Jakarta (no DST in Indonesia). */
export const JAKARTA_OFFSET = '+07:00';

/**
 * Extract wall-clock time components from a Date in the Asia/Jakarta timezone.
 *
 * Uses Intl.DateTimeFormat with timeZone: 'Asia/Jakarta' so the result is correct
 * regardless of the browser's local timezone.
 */
export function getJakartaTimeParts(date: Date = new Date()): {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
  millisecond: number;
} {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone: JAKARTA_TIMEZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });

  const parts = formatter.formatToParts(date);
  const get = (type: string) => {
    const p = parts.find((x) => x.type === type);
    return p ? parseInt(p.value, 10) : 0;
  };

  return {
    year: get('year'),
    month: get('month'),
    day: get('day'),
    hour: get('hour'),
    minute: get('minute'),
    second: get('second'),
    // Milliseconds are not available via Intl.DateTimeFormat options in all
    // TS versions; fall back to the underlying Date's UTC milliseconds, then
    // adjust for the Jakarta offset. Since we only need milliseconds for the
    // ISO 8601 fractional part and Jakarta is fixed at +07:00 (no DST), the
    // UTC milliseconds modulo 1000 equals the Jakarta milliseconds.
    millisecond: date.getUTCMilliseconds(),
  };
}

/**
 * Convert a Jakarta calendar date string ("YYYY-MM-DD") plus current Jakarta wall-clock
 * time into an ISO 8601 timestamp with explicit +07:00 offset.
 *
 * This avoids the pitfall of `new Date('YYYY-MM-DD')`, which parses as UTC midnight
 * and can shift the date when the browser runs in a timezone behind UTC.
 *
 * @param dateStr - Jakarta calendar date in "YYYY-MM-DD" format (e.g. "2026-10-22")
 * @returns ISO 8601 string like "2026-10-22T10:30:00.000+07:00"
 *
 * @example
 * // User picks 2026-10-22, current Jakarta time is 10:30:00 WIB
 * toJakartaIso8601("2026-10-22") // "2026-10-22T10:30:00.000+07:00"
 */
export function toJakartaIso8601(dateStr: string): string {
  const [year, month, day] = dateStr.split('-').map(Number);

  // Get current wall-clock time in Jakarta (independent of browser timezone)
  const { hour, minute, second, millisecond } = getJakartaTimeParts();

  const pad = (n: number) => String(n).padStart(2, '0');
  const pad3 = (n: number) => String(n).padStart(3, '0');

  return (
    `${String(year).padStart(4, '0')}-${pad(month)}-${pad(day)}T` +
    `${pad(hour)}:${pad(minute)}:${pad(second)}.${pad3(millisecond)}+07:00`
  );
}

/**
 * Get the Jakarta calendar date string ("YYYY-MM-DD") for the current Jakarta wall-clock time.
 *
 * This avoids the pitfall of `new Date().toISOString().split('T')[0]`, which parses
 * as UTC midnight and can shift the date when the browser runs in a timezone
 * behind UTC (e.g. browsers in Asia/Jakarta at night).
 *
 * @returns Jakarta calendar date in "YYYY-MM-DD" format (e.g. "2026-10-22")
 *
 * @example
 * // Current Jakarta wall-clock time is 22 Oct 2026 02:00 WIB
 * getJakartaDateString() // "2026-10-22"
 */
export function getJakartaDateString(): string {
  const { year, month, day } = getJakartaTimeParts()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${String(year).padStart(4, '0')}-${pad(month)}-${pad(day)}`
}

/**
 * Parse an ISO 8601 timestamp (with or without offset) into epoch milliseconds.
 *
 * This is used for sorting: all timestamps are compared as instant values,
 * not as local date strings.
 *
 * @param iso - ISO 8601 string (e.g. "2026-10-22T10:30:00.000+07:00" or "...Z")
 * @returns Epoch milliseconds, or NaN if unparseable
 */
export function parseIso8601ToEpoch(iso: string | null | undefined): number {
  if (!iso) return NaN;
  const ms = Date.parse(iso);
  return Number.isFinite(ms) ? ms : NaN;
}
