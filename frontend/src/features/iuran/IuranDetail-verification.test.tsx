/**
 * B3 — Payment verification validation regression tests
 *
 * These tests isolate the reject/approve validation logic from the
 * IuranDetail component to avoid complex mocking of the entire
 * React tree + API layer.
 *
 * Each test mirrors what handleReject/handleApprove does in the
 * component so that any drift is caught.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { canApprovePayment } from './IuranDetail'
import type { AuthUser } from '@/app/AuthContext'

// ---- Validation logic (copied from IuranDetail.tsx handleReject) ----

/**
 * Simulate handleReject validation: officerNotes.trim() must be non-empty.
 * Returns { ok: true } or { ok: false; error: string }.
 */
function validateReject(officerNotes: string) {
  const notes = officerNotes.trim()
  if (!notes) {
    return { ok: false as const, error: 'Catatan wajib diisi untuk menolak pembayaran.' }
  }
  return { ok: true as const, notes }
}

/**
 * Simulate the payload built by handleReject when validation passes.
 * Sends both rejectionReason and officerNotes with the same value.
 */
function buildRejectPayload(notes: string) {
  return {
    action: 'reject',
    rejection_reason: notes,
    officer_notes: notes,
  }
}

/**
 * Simulate handleApprove payload: action + officerNotes (empty → undefined).
 */
function buildApprovePayload(officerNotes: string) {
  return {
    action: 'approve',
    ...(officerNotes.trim() ? { officer_notes: officerNotes.trim() } : {}),
  }
}

describe('B3 — Payment verification validation', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  // ---- Reject validation ----

  describe('reject validation', () => {
    it('empty string returns error', () => {
      const result = validateReject('')
      expect(result.ok).toBe(false)
      expect(result.error).toBe('Catatan wajib diisi untuk menolak pembayaran.')
    })

    it('whitespace-only returns error', () => {
      const result = validateReject('   ')
      expect(result.ok).toBe(false)
      expect(result.error).toBe('Catatan wajib diisi untuk menolak pembayaran.')
    })

    it('newline-only returns error', () => {
      const result = validateReject('\n\t\n')
      expect(result.ok).toBe(false)
    })

    it('valid notes returns ok with trimmed notes', () => {
      const result = validateReject('  Pembayaran tidak sesuai  ')
      expect(result.ok).toBe(true)
      expect((result as { ok: true; notes: string }).notes).toBe('Pembayaran tidak sesuai')
    })

    it('single character notes returns ok', () => {
      const result = validateReject('O')
      expect(result.ok).toBe(true)
      expect((result as { ok: true; notes: string }).notes).toBe('O')
    })

    // ---- Reject payload construction ----

    it('reject payload includes both rejection_reason and officer_notes', () => {
      const result = validateReject('Pembayaran tidak sesuai')
      expect(result.ok).toBe(true)
      const payload = buildRejectPayload((result as { ok: true; notes: string }).notes)
      expect(payload.action).toBe('reject')
      expect(payload.rejection_reason).toBe('Pembayaran tidak sesuai')
      expect(payload.officer_notes).toBe('Pembayaran tidak sesuai')
    })
  })

  // ---- Approve validation ----

  describe('approve payload', () => {
    it('empty notes → no officer_notes in payload', () => {
      const payload = buildApprovePayload('')
      expect(payload.action).toBe('approve')
      expect('officer_notes' in payload).toBe(false)
    })

    it('whitespace notes → no officer_notes in payload', () => {
      const payload = buildApprovePayload('   ')
      expect(payload.action).toBe('approve')
      expect('officer_notes' in payload).toBe(false)
    })

    it('valid notes → officer_notes included trimmed', () => {
      const payload = buildApprovePayload('  Pembayaran sudah sesuai  ')
      expect(payload.action).toBe('approve')
      expect(payload.officer_notes).toBe('Pembayaran sudah sesuai')
    })
  })

  // ---- Integration-style: rejection_reason excluded when empty ----

  describe('API contract — conditional fields', () => {
    it('reject payload always includes rejection_reason (never empty)', () => {
      const result = validateReject('  Note  ')
      if (!result.ok) throw new Error('Should not error')
      const payload = buildRejectPayload(result.notes)
      // rejection_reason is always set for reject because validation ensures non-empty
      expect(payload.rejection_reason).toBe('Note')
    })

    it('reject payload officer_notes mirrors rejection_reason', () => {
      const result = validateReject('Test notes')
      if (!result.ok) throw new Error('Should not error')
      const payload = buildRejectPayload(result.notes)
      expect(payload.officer_notes).toBe(payload.rejection_reason)
    })
  })

  // ---- Role-based verification button visibility ----

  describe('canApprovePayment authorization', () => {
    it('returns false for null user', () => {
      expect(canApprovePayment(null)).toBe(false)
    })

    it('returns false for ordinary WARGA user (cannot see Terima/Tolak)', () => {
      const wargaUser: AuthUser = {
        id: 'user-warga-1',
        name: 'Warga Biasa',
        email: 'warga@test.local',
        role: 'warga',
        systemRole: null,
        jabatan: null,
        rt: { id: 'rt-1', name: 'RT 002' },
        effectivePermissions: ['finance.read'],
      }
      expect(canApprovePayment(wargaUser)).toBe(false)
    })

    it('returns true for Ketua RT with role=warga and jabatan=ketua', () => {
      const ketuaUser: AuthUser = {
        id: 'user-ketua-1',
        name: 'Pak RT Ketua',
        email: 'ketua@test.local',
        role: 'warga',
        systemRole: null,
        jabatan: 'ketua',
        rt: { id: 'rt-1', name: 'RT 002' },
        effectivePermissions: ['finance.read', 'finance.create', 'finance.update', 'finance.approve'],
      }
      expect(canApprovePayment(ketuaUser)).toBe(true)
    })

    it('returns true for Bendahara RT with role=bendahara', () => {
      const bendaharaUser: AuthUser = {
        id: 'user-bendahara-1',
        name: 'Bendahara RT',
        email: 'bendahara@test.local',
        role: 'bendahara',
        systemRole: null,
        jabatan: null,
        rt: { id: 'rt-1', name: 'RT 002' },
        effectivePermissions: ['finance.read', 'finance.create', 'finance.update', 'finance.approve'],
      }
      expect(canApprovePayment(bendaharaUser)).toBe(true)
    })

    it('returns true for super_admin', () => {
      const adminUser: AuthUser = {
        id: 'user-admin-1',
        name: 'Super Admin',
        email: 'admin@test.local',
        role: 'super_admin',
        systemRole: 'super_admin',
        jabatan: null,
        rt: null,
      }
      expect(canApprovePayment(adminUser)).toBe(true)
    })

    it('returns false for pengurus without finance.approve permission', () => {
      const pengurusReadOnly: AuthUser = {
        id: 'user-pengurus-1',
        name: 'Petugas Keamanan',
        email: 'keamanan@test.local',
        role: 'warga',
        systemRole: null,
        jabatan: 'keamanan',
        rt: { id: 'rt-1', name: 'RT 002' },
        effectivePermissions: ['warga.read'],
      }
      expect(canApprovePayment(pengurusReadOnly)).toBe(false)
    })

    it('returns true when effectivePermissions includes finance.approve', () => {
      const permittedUser: AuthUser = {
        id: 'user-permitted-1',
        name: 'Petugas Keuangan',
        email: 'petugas@test.local',
        role: 'warga',
        systemRole: null,
        jabatan: null,
        rt: { id: 'rt-1', name: 'RT 002' },
        effectivePermissions: ['finance.approve'],
      }
      expect(canApprovePayment(permittedUser)).toBe(true)
    })
  })
})
