/**
 * Warga module write-access authorization helper.
 *
 * Authorization flow (evaluated top-to-bottom):
 * 1. Super-admin bypass — any user with `systemRole === 'super_admin'` has full write access.
 * 2. Position-based — if the user has a non-empty `jabatan` (role position within the RT),
 *    write access is granted ONLY for the four authorized positions:
 *    ketua, wakil_ketua, sekretaris, bendahara.
 *    **Unknown or unrecognized non-empty jabatan MUST deny access.**
 * 3. Legacy role-based fallback — if `jabatan` is null, undefined, or empty,
 *    fall back to checking the legacy `role` field:
 *    - `pengurus` → allowed
 *    - `super_admin` → allowed (redundant safety net)
 *    - All other roles (`warga`, `bendahara`, `perangkat`, etc.) → denied
 *
 * This allows the Warga list screen to show the "Tambah Warga" action
 * only for users who are authorized to create or modify warga records.
 */

/** Set of RT positions authorized to write warga data. */
export const K_WARGA_WRITE_JABATANS = new Set<string>([
  'ketua',
  'wakil_ketua',
  'sekretaris',
  'bendahara',
])

export type WargaUser = {
  systemRole?: string | null
  role?: string
  jabatan?: string | null
}

/**
 * Returns `true` if the user has write access to the Warga module.
 */
export function hasWargaWriteAccess(user: WargaUser): boolean {
  // 1. Super-admin bypass
  if (user.systemRole === 'super_admin') {
    return true
  }

  // 2. Position-based authorization
  if (typeof user.jabatan === 'string' && user.jabatan.length > 0) {
    return K_WARGA_WRITE_JABATANS.has(user.jabatan)
  }

  // 3. Legacy role-based fallback
  switch (user.role) {
    case 'pengurus':
    case 'super_admin':
      return true
    default:
      return false
  }
}

/**
 * Returns a string describing WHY access was granted or denied.
 * Useful for debugging and logging who is allowed through which path.
 */
export function getWargaWriteRoleType(user: WargaUser): 'super_admin' | 'position' | 'role_legacy' | 'none' {
  if (user.systemRole === 'super_admin') {
    return 'super_admin'
  }

  if (typeof user.jabatan === 'string' && user.jabatan.length > 0) {
    return K_WARGA_WRITE_JABATANS.has(user.jabatan) ? 'position' : 'none'
  }

  if (user.role === 'pengurus' || user.role === 'super_admin') {
    return 'role_legacy'
  }

  return 'none'
}

// Note: Unit tests are in wargaAuth.test.ts
