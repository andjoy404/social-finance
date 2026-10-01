import { describe, it, expect } from 'vitest'
import { hasWargaWriteAccess, getWargaWriteRoleType, K_WARGA_WRITE_JABATANS } from '@/app/wargaAuth'

describe('wargaAuth — K_WARGA_WRITE_JABATANS', () => {
  it('contains exactly the four authorized positions', () => {
    const expected = new Set(['ketua', 'wakil_ketua', 'sekretaris', 'bendahara'])
    expect(K_WARGA_WRITE_JABATANS).toEqual(expected)
    expect(K_WARGA_WRITE_JABATANS.size).toBe(4)
  })
})

describe('wargaAuth — hasWargaWriteAccess', () => {
  it('super_admin always has write access regardless of role/jabatan', () => {
    expect(hasWargaWriteAccess({ systemRole: 'super_admin' })).toBe(true)
    expect(hasWargaWriteAccess({ systemRole: 'super_admin', role: 'warga', jabatan: null })).toBe(true)
    expect(hasWargaWriteAccess({ systemRole: 'super_admin', role: 'warga', jabatan: 'ketua' })).toBe(true)
    expect(hasWargaWriteAccess({ systemRole: 'super_admin', role: 'warga', jabatan: 'kas_keuangan' })).toBe(true)
  })

  it('authorized positions grant write access', () => {
    for (const jabatan of ['ketua', 'wakil_ketua', 'sekretaris', 'bendahara']) {
      expect(hasWargaWriteAccess({ role: 'warga', jabatan })).toBe(true)
    }
  })

  it('unknown non-empty jabatan denies access', () => {
    const unknownJabatans = ['kas_keuangan', 'koordinator_kegiatan', 'sekbid_media']
    for (const jabatan of unknownJabatans) {
      expect(hasWargaWriteAccess({ role: 'warga', jabatan })).toBe(false)
    }
  })

  it('empty string jabatan falls through to legacy role-based check', () => {
    expect(hasWargaWriteAccess({ role: 'pengurus', jabatan: '' })).toBe(true)
    expect(hasWargaWriteAccess({ role: 'warga', jabatan: '' })).toBe(false)
  })

  it('null jabatan falls through to legacy role-based check', () => {
    expect(hasWargaWriteAccess({ role: 'pengurus', jabatan: null })).toBe(true)
    expect(hasWargaWriteAccess({ role: 'warga', jabatan: null })).toBe(false)
  })

  it('undefined jabatan falls through to legacy role-based check', () => {
    expect(hasWargaWriteAccess({ role: 'pengurus', jabatan: undefined })).toBe(true)
    expect(hasWargaWriteAccess({ role: 'warga', jabatan: undefined })).toBe(false)
  })

  it('legacy role-based: pengurus allowed', () => {
    expect(hasWargaWriteAccess({ role: 'pengurus' })).toBe(true)
  })

  it('legacy role-based: warga/bendahara/perangkat denied', () => {
    expect(hasWargaWriteAccess({ role: 'warga' })).toBe(false)
    expect(hasWargaWriteAccess({ role: 'bendahara' })).toBe(false)
    expect(hasWargaWriteAccess({ role: 'perangkat' })).toBe(false)
  })

  it('undefined user fields default to deny', () => {
    expect(hasWargaWriteAccess({})).toBe(false)
  })
})

describe('wargaAuth — getWargaWriteRoleType', () => {
  it('returns super_admin for super_admin users', () => {
    expect(getWargaWriteRoleType({ systemRole: 'super_admin' })).toBe('super_admin')
    expect(getWargaWriteRoleType({ systemRole: 'super_admin', role: 'warga' })).toBe('super_admin')
  })

  it('returns position for authorized jabatan', () => {
    for (const jabatan of ['ketua', 'wakil_ketua', 'sekretaris', 'bendahara']) {
      expect(getWargaWriteRoleType({ role: 'warga', jabatan })).toBe('position')
    }
  })

  it('returns none for unauthorized jabatan', () => {
    expect(getWargaWriteRoleType({ role: 'warga', jabatan: 'kas_keuangan' })).toBe('none')
    expect(getWargaWriteRoleType({ role: 'warga', jabatan: 'koordinator_kegiatan' })).toBe('none')
  })

  it('returns role_legacy for authorized legacy roles', () => {
    expect(getWargaWriteRoleType({ role: 'pengurus' })).toBe('role_legacy')
  })

  it('returns none for unauthorized legacy roles', () => {
    expect(getWargaWriteRoleType({ role: 'warga' })).toBe('none')
    expect(getWargaWriteRoleType({ role: 'bendahara' })).toBe('none')
    expect(getWargaWriteRoleType({ role: 'perangkat' })).toBe('none')
  })

  it('falls through to legacy check for empty jabatan', () => {
    expect(getWargaWriteRoleType({ role: 'pengurus', jabatan: '' })).toBe('role_legacy')
    expect(getWargaWriteRoleType({ role: 'warga', jabatan: '' })).toBe('none')
  })

  it('returns none for empty user object', () => {
    expect(getWargaWriteRoleType({})).toBe('none')
  })
})
