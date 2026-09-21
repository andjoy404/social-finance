import { describe, it, expect } from 'vitest'
import { MOCK_USERS } from '@/mocks/users'

describe('Mock Users', () => {
  it('Admin has SUPER_ADMIN role', () => {
    expect(MOCK_USERS.admin.role).toBe('SUPER_ADMIN')
    expect(MOCK_USERS.admin.name).toBe('Admin')
  })

  it('Heri Prastyo has BENDAHARA role with RT/RW', () => {
    expect(MOCK_USERS.heri.role).toBe('BENDAHARA')
    expect(MOCK_USERS.heri.name).toBe('Heri Prastyo')
    expect(MOCK_USERS.heri.rtNumber).toBe('002')
    expect(MOCK_USERS.heri.rwNumber).toBe('016')
  })

  it('Andriyan has WARGA role with house and OWNER occupancy', () => {
    expect(MOCK_USERS.andriyan.role).toBe('WARGA')
    expect(MOCK_USERS.andriyan.name).toBe('Andriyan')
    expect(MOCK_USERS.andriyan.rtNumber).toBe('002')
    expect(MOCK_USERS.andriyan.rwNumber).toBe('016')
    expect(MOCK_USERS.andriyan.house).toBe('U12/14')
    expect(MOCK_USERS.andriyan.occupancy).toBe('OWNER')
  })

  it('Andriyan is NOT BENDAHARA', () => {
    expect(MOCK_USERS.andriyan.role).not.toBe('BENDAHARA')
  })
})
