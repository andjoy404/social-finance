export type MockUserRole = 'SUPER_ADMIN' | 'BENDAHARA' | 'WARGA'

export interface MockUser {
  id: string
  username: string
  name: string
  role: MockUserRole
  rtNumber?: string
  rwNumber?: string
  house?: string
  occupancy?: 'OWNER' | 'TENANT'
}

export const MOCK_USERS: Record<string, MockUser> = {
  admin: {
    id: 'mock-superadmin',
    username: 'admin',
    name: 'Admin',
    role: 'SUPER_ADMIN',
  },
  heri: {
    id: 'mock-heri',
    username: 'heri',
    name: 'Heri Prastyo',
    role: 'BENDAHARA',
    rtNumber: '002',
    rwNumber: '016',
  },
  andriyan: {
    id: 'mock-andriyan',
    username: 'andriyan',
    name: 'Andriyan',
    role: 'WARGA',
    rtNumber: '002',
    rwNumber: '016',
    house: 'U12/14',
    occupancy: 'OWNER',
  },
}

export type MockUserByKey = keyof typeof MOCK_USERS
