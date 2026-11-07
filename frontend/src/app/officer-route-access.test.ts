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

  it('restricts security officers to Warga and Notifikasi', () => {
    expect(getOfficerRouteDestination('keamanan', '/')).toBe('/warga')
    expect(getOfficerRouteDestination('keamanan', '/kas')).toBe('/warga')
    expect(getOfficerRouteDestination('keamanan', '/notifikasi')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/warga')).toBeNull()
    expect(getOfficerRouteDestination('keamanan', '/warga/resident-1')).toBeNull()
  })

  it('does not alter routes for other positions', () => {
    expect(getOfficerRouteDestination('ketua', '/kas')).toBeNull()
    expect(getOfficerRouteDestination(null, '/')).toBeNull()
  })
})
