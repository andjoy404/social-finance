/**
 * RT/RW 3-digit formatting utilities for React web.
 *
 * All RT/RW codes displayed in the app should use these helpers to ensure
 * consistent 3-digit zero-padded format: "003", "016", "100", etc.
 *
 * Only pass actual RT/RW codes to these functions. Do NOT pass UUIDs,
 * NIKs, phone numbers, monetary amounts, or other non-wilayah identifiers.
 */

/**
 * Format an RT code to 3-digit zero-padded string.
 *
 * Accepts: string | number | null | undefined
 * Returns: "003", "016", "001", etc. Always 3 digits.
 * Preserves alphanumeric codes (e.g. "C01", "A12") as-is.
 * Returns "000" for null, empty, invalid, or negative values.
 */
export function formatRtCode(
  value: string | number | null | undefined,
): string {
  if (value == null) return '000';
  if (typeof value === 'number') {
    if (value < 0) return '000';
    return String(Math.floor(value)).padStart(3, '0');
  }
  const str = String(value).trim();
  if (str === '') return '000';
  // If the string is purely numeric, zero-pad it.
  // Otherwise preserve as-is (alphanumeric codes like "C01", "A12").
  const match = str.match(/^(-?\d+)$/);
  if (!match) return str;
  const n = parseInt(match[1], 10);
  if (n < 0) return '000';
  return String(n).padStart(3, '0');
}

/**
 * Format an RW code to 3-digit zero-padded string.
 *
 * Accepts: number | string | null | undefined
 * Returns: "016", "001", etc. Always 3 digits.
 * Preserves alphanumeric codes (e.g. "C01") as-is.
 * Returns "000" for null, invalid, or negative values.
 */
export function formatRwCode(
  value: number | string | null | undefined,
): string {
  if (value == null) return '000';
  if (typeof value === 'number') {
    if (value < 0) return '000';
    return String(Math.floor(value)).padStart(3, '0');
  }
  const str = String(value).trim();
  if (str === '') return '000';
  const match = str.match(/^(-?\d+)$/);
  if (!match) return str;
  const n = parseInt(match[1], 10);
  if (n < 0) return '000';
  return String(n).padStart(3, '0');
}

/**
 * Combined RT/RW formatter for display.
 *
 * Returns formatted string like "RT 003 · RW 016".
 * If both are "000", returns "—" (em dash placeholder).
 */
export function formatRtRw(
  rt: string | number | null | undefined,
  rw: number | string | null | undefined,
): string {
  const formattedRt = formatRtCode(rt);
  const formattedRw = formatRwCode(rw);
  if (formattedRt === '000' && formattedRw === '000') return '\u2014';
  return `RT ${formattedRt}${formattedRw !== '000' ? ' · RW ' + formattedRw : ''}`;
}
