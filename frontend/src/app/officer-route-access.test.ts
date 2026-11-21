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

  it('blocks kas routes for operasional and humas, and allows iuran personal bills', () => {
    expect(getOfficerRouteDestination('operasional', '/kas')).toBe('/')
    expect(getOfficerRouteDestination('operasional', '/kas/masuk')).toBe('/')
    expect(getOfficerRouteDestination('operasional', '/iuran')).toBeNull()
    expect(getOfficerRouteDestination('operasional', '/iuran/123')).toBeNull()
    expect(getOfficerRouteDestination('operasional', '/warga')).toBeNull()
    expect(getOfficerRouteDestination('operasional', '/')).toBeNull()

    expect(getOfficerRouteDestination('humas', '/kas')).toBe('/')
    expect(getOfficerRouteDestination('humas', '/kas/masuk')).toBe('/')
    expect(getOfficerRouteDestination('humas', '/kas', [{ rw_id: 'rw-002', rw_name: 'RW 002' }])).toBe('/rw-view/rw-002/kas')
    expect(getOfficerRouteDestination('humas', '/keuangan', [{ rw_id: 'rw-002', rw_name: 'RW 002' }])).toBe('/rw-view/rw-002/dashboard')
    expect(getOfficerRouteDestination('humas', '/iuran')).toBeNull()
    expect(getOfficerRouteDestination('humas', '/iuran/123')).toBeNull()
    expect(getOfficerRouteDestination('humas', '/warga')).toBeNull()
    expect(getOfficerRouteDestination('humas', '/')).toBeNull()

    expect(getOfficerRouteDestination('sosial', '/kas')).toBe('/')
    expect(getOfficerRouteDestination('sosial', '/kas/masuk')).toBe('/')
    expect(getOfficerRouteDestination('sosial', '/iuran')).toBeNull()
    expect(getOfficerRouteDestination('sosial', '/iuran/123')).toBeNull()
    expect(getOfficerRouteDestination('sosial', '/warga')).toBeNull()
    expect(getOfficerRouteDestination('sosial', '/')).toBeNull()
  })

  it('does not alter routes for other positions', () => {
    expect(getOfficerRouteDestination('ketua', '/kas')).toBeNull()
    expect(getOfficerRouteDestination(null, '/')).toBeNull()
  })
})
