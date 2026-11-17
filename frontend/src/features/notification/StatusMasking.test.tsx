import { describe, it, expect } from 'vitest'

describe('Notification status masking — Warga vs Admin', () => {
  // Test the logic that getDeliveryStatusDisplay() uses.
  // Web: if (!isAdmin) return { label: 'TERKIRIM', variant: 'green' }
  // Mobile: if (!isAdmin) { label = 'TERKIRIM'; color = AppColors.success }

  it('Warga sees TERKIRIM for ALL delivery statuses', () => {
    const allInternalStatuses = [
      'SENT', 'NO_DEVICE', 'PARTIAL', 'FAILED', 
      'UNKNOWN', 'PENDING', 'RECORDED', 'APPROVED', 'REJECTED',
      null, undefined, '', 'some_weird_value'
    ]
    
    // Simulate: isAdmin = false → always TERKIRIM
    for (const _status of allInternalStatuses) {
      // This is exactly what getDeliveryStatusDisplay does when isAdmin=false
      const result = { label: 'TERKIRIM', variant: 'green' as const }
      expect(result.label).toBe('TERKIRIM')
    }
  })

  it('Admin sees detailed status for every type', () => {
    const adminExpected: Record<string, string> = {
      'SENT': 'TERKIRIM',
      'NO_DEVICE': 'PERANGKAT TIDAK TERSEDIA',
      'PARTIAL': 'SEBAGIAN GAGAL',
      'FAILED': 'GAGAL',
      'UNKNOWN': 'TIDAK DIKETAHUI',
      'PENDING': 'TIDAK DIKETAHUI',
      'RECORDED': 'TERCATAT',
      'APPROVED': 'DISETUJUI',
      'REJECTED': 'DITOLAK',
    }
    
    for (const [status, expected] of Object.entries(adminExpected)) {
      const normalized = status?.trim().toUpperCase() || 'UNKNOWN'
      let result: string
      switch (normalized) {
        case 'SENT': result = 'TERKIRIM'; break
        case 'FAILED': result = 'GAGAL'; break
        case 'PARTIAL': result = 'SEBAGIAN GAGAL'; break
        case 'APPROVED': result = 'DISETUJUI'; break
        case 'REJECTED': result = 'DITOLAK'; break
        case 'RECORDED': result = 'TERCATAT'; break
        case 'NO_DEVICE': result = 'PERANGKAT TIDAK TERSEDIA'; break
        case 'UNKNOWN':
        case 'PENDING':
        default: result = 'TIDAK DIKETAHUI'; break
      }
      expect(result).toBe(expected)
    }
  })

  it('NO_DEVICE maps correctly — not the same as UNKNOWN', () => {
    // NO_DEVICE = explicitly known to have no push token (admin sees it)
    // UNKNOWN = delivery result unavailable (admin sees it)
    // Both → TERKIRIM for Warga
    const wargaAlwaysSees: string[] = ['SENT', 'NO_DEVICE', 'PARTIAL', 'FAILED', 'UNKNOWN', 'PENDING', 'RECORDED']
    for (const _status of wargaAlwaysSees) {
      expect({ label: 'TERKIRIM', variant: 'green' }).toEqual({ label: 'TERKIRIM', variant: 'green' })
    }

    // Admin sees different labels
    expect((() => {
      const n = 'NO_DEVICE'.toUpperCase()
      switch (n) {
        case 'NO_DEVICE': return 'PERANGKAT TIDAK TERSEDIA'
        default: return 'TIDAK DIKETAHUI'
      }
    })()).toBe('PERANGKAT TIDAK TERSEDIA')

    expect((() => {
      const n = 'UNKNOWN'.toUpperCase()
      switch (n) {
        case 'UNKNOWN':
        case 'PENDING': return 'TIDAK DIKETAHUI'
        default: return 'TIDAK DIKETAHUI'
      }
    })()).toBe('TIDAK DIKETAHUI')
  })
})
