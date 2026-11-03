/**
 * Utilities for formatting and resolving RW identifiers.
 * Displays human-readable RW numbers (e.g. "RW 016") instead of raw UUIDs in the presentation layer.
 */

export function looksLikeUuid(id?: string | null): boolean {
  if (!id) return false
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)
}

export function formatRwNumber(rwNumber?: number | string | null): string | null {
  if (rwNumber === undefined || rwNumber === null || rwNumber === '') return null
  const num = typeof rwNumber === 'number' ? rwNumber : parseInt(String(rwNumber), 10)
  if (isNaN(num)) return null
  return `RW ${String(num).padStart(3, '0')}`
}

export function resolveRwLabel(
  rwId?: string | null,
  rwMemberships?: Array<{ rw_id: string; rw_name?: string; rw_number?: number }>,
  customName?: string | null
): string {
  if (customName && !looksLikeUuid(customName)) {
    return customName
  }
  if (rwMemberships && rwId) {
    const mem = rwMemberships.find((m) => m.rw_id === rwId)
    if (mem) {
      if (mem.rw_name && !looksLikeUuid(mem.rw_name)) return mem.rw_name
      if (mem.rw_number !== undefined && mem.rw_number !== null) {
        return `RW ${String(mem.rw_number).padStart(3, '0')}`
      }
    }
  }
  if (rwId && looksLikeUuid(rwId)) {
    return 'DATA RW'
  }
  return rwId || 'DATA RW'
}

