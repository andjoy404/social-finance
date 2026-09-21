import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from '@/app/AuthContext'
import { Login } from '@/features/auth/Login'
import { Dashboard } from '@/features/dashboard/Dashboard'
import { AppLayout } from '@/layouts/AppLayout'
import { RtList } from '@/features/rt/RtList'
import { RtDetail } from '@/features/rt/RtDetail'
import { RtCreate } from '@/features/rt/RtCreate'
import { RtEdit } from '@/features/rt/RtEdit'
import { HouseholdList } from '@/features/warga/HouseholdList'
import { HouseholdDetail } from '@/features/warga/HouseholdDetail'
import { HouseholdCreate } from '@/features/warga/HouseholdCreate'
import { HouseholdEdit } from '@/features/warga/HouseholdEdit'
import { HouseholdMove } from '@/features/warga/HouseholdMove'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function SuperAdminRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  if (!user || user.systemRole !== 'super_admin') {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

export function App() {
  const { isAuthenticated, isInitializing } = useAuth()

  if (isInitializing) return null

  return (
    <Routes>
      <Route
        path="/login"
        element={isAuthenticated ? <Navigate to="/" replace /> : <Login />}
      />

      <Route
        path="/"
        element={
          <ProtectedRoute>
            <AppLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Dashboard />} />
        <Route
          path="rt"
          element={
            <SuperAdminRoute>
              <RtList />
            </SuperAdminRoute>
          }
        />
        <Route
          path="rt/:id"
          element={
            <SuperAdminRoute>
              <RtDetail />
            </SuperAdminRoute>
          }
        />
        <Route
          path="rt/new"
          element={
            <SuperAdminRoute>
              <RtCreate />
            </SuperAdminRoute>
          }
        />
        <Route
          path="rt/:id/edit"
          element={
            <SuperAdminRoute>
              <RtEdit />
            </SuperAdminRoute>
          }
        />
        <Route
          path="warga"
          element={<HouseholdList />}
        />
        <Route
          path="warga/:id"
          element={<HouseholdDetail />}
        />
        <Route
          path="warga/baru"
          element={<HouseholdCreate />}
        />
        <Route
          path="warga/:id/edit"
          element={<HouseholdEdit />}
        />
        <Route
          path="warga/:id/pindah"
          element={<HouseholdMove />}
        />
      </Route>

      <Route path="*" element={<Navigate to={isAuthenticated ? "/" : "/login"} replace />} />
    </Routes>
  )
}
