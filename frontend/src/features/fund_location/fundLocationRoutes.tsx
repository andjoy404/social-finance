import { Routes, Route, Navigate } from 'react-router-dom'
import { FundLocationList } from './FundLocationList'
import { useAuth } from '@/app/AuthContext'

export function FundLocationRoutes() {
  const { user } = useAuth()

  // Warga is blocked from viewing Fund Locations
  if (user?.role === 'warga' && !user?.jabatan) {
    return <Navigate to="/" replace />
  }

  return (
    <Routes>
      <Route index element={<FundLocationList />} />
      <Route path="*" element={<Navigate to="" replace />} />
    </Routes>
  )
}
