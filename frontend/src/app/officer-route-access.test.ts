import { describe, expect, it } from 'vitest'
import { getOfficerRouteDestination } from './App'

describe('special officer route access', () => {
  it('redirects cleaning officers to Monitoring unless the route is allowed', () => {
    expect(getOfficerRouteDestination('kebersihan', '/')).toBe('/monitoring')
    expect(getOfficerRouteDestination('kebersihan', '/kas')).toBe('/monitoring')
    expect(getOfficerRouteDestination('kebersihan', '/iuran')).toBe('/monitoring')
    expect(getOfficerRouteDestination('kebersihan', '/notifikasi')).toBe('/monitoring')
    expect(getOfficerRouteDestination('kebersihan', '/monitoring')).toBeNull()
  })

  it('restricts security officers to Warga, Notifikasi, Monitoring, Patroli, and Paket', () => {
    expect(getOfficerRouteDestination('keamanan', '/')).toBe('/warga')
    expect(getOfficerRouteDestination('keamanan', '/kas')).toBe('/warga')
    expect(getOfficerRouteDestination('keamanan', '/notifikasi')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/monitoring')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/warga')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/warga/resident-1')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/patroli')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/patroli/bundle-1')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/paket')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/paket/report-1')).toBeNull()
  })

  it('redirects anggota and anggota_rt to RW Dashboard and allows notifikasi', () => {
    const rwMemberships = [{ rw_id: 'rw-002', rw_name: 'RW 002' }]
    for (const pos of ['anggota', 'anggota_rt']) {
      expect(getOfficerRouteDestination(pos, '/', rwMemberships)).toBe('/rw-view/rw-002/dashboard')
      expect(getOfficerRouteDestination(pos, '/kas', rwMemberships)).toBe('/rw-view/rw-002/dashboard')
      expect(getOfficerRouteDestination(pos, '/notifikasi', rwMemberships)).toBeNull()
      expect(getOfficerRouteDestination(pos, '/rw-view/rw-002/dashboard', rwMemberships)).toBeNull()
      expect(getOfficerRouteDestination(pos, '/rw-view/rw-002/kas', rwMemberships)).toBeNull()
      // Test userRwId fallback when rwMemberships is empty
      expect(getOfficerRouteDestination(pos, '/', [], 'rw-002')).toBe('/rw-view/rw-002/dashboard')
    }
  })

  it('blocks kas and iuran routes for sosial and operasional', () => {
    for (const pos of ['sosial', 'operasional']) {
      expect(getOfficerRouteDestination(pos, '/kas')).toBe('/')
      expect(getOfficerRouteDestination(pos, '/kas/masuk')).toBe('/')
      expect(getOfficerRouteDestination(pos, '/iuran')).toBe('/')
      expect(getOfficerRouteDestination(pos, '/iuran/123')).toBe('/')
      expect(getOfficerRouteDestination(pos, '/warga')).toBeNull()
      expect(getOfficerRouteDestination(pos, '/')).toBeNull()
    }
  })

  it('does not alter routes for other positions', () => {
    expect(getOfficerRouteDestination('ketua', '/kas')).toBeNull()
    expect(getOfficerRouteDestination(null, '/')).toBeNull()
  })
})
