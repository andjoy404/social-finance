import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth, isAllowedRT } from '@/app/AuthContext'
import { Login } from '@/features/auth/Login'
import { Dashboard } from '@/features/dashboard/Dashboard'
import { AppLayout } from '@/layouts/AppLayout'
import { RtDetail } from '@/features/rt/RtDetail'
import { RtList } from '@/features/rt/RtList'
import { RwList } from '@/features/rt/RwList'
import { RtCreate } from '@/features/rt/RtCreate'
import { RtEdit } from '@/features/rt/RtEdit'
import { WargaLayout } from '@/features/warga/WargaLayout'
import { IuranRoutes } from '@/features/iuran/iuranRoutes'
import { KasRoutes } from '@/features/kas/kasRoutes'
import { FundLocationRoutes } from '@/features/fund_location/fundLocationRoutes'
import { ResidentDetail } from '@/features/warga/ResidentDetail'
import { HouseholdCreate } from '@/features/warga/HouseholdCreate'
import { HouseholdEdit } from '@/features/warga/HouseholdEdit'
import { HouseholdMove } from '@/features/warga/HouseholdMove'
import { SpecialResidentCreate } from '@/features/warga/SpecialResidentCreate'
import { SpecialResidentEdit } from '@/features/warga/SpecialResidentEdit'
import { NotificationScreen } from '@/features/notification/NotificationScreen'

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

function AllowedRTRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  if (!isAllowedRT(user)) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

function NotificationAdminRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  const isSuperAdmin = user?.systemRole === 'super_admin'
  const isKetuaRT = user?.jabatan?.toLowerCase() === 'ketua'
  if (!isSuperAdmin && !isKetuaRT) {
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
          path="rw"
          element={<SuperAdminRoute><RwList /></SuperAdminRoute>}
        />
        <Route
          path="rt"
          element={<SuperAdminRoute><RtList /></SuperAdminRoute>}
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
          element={
            <AllowedRTRoute>
              <WargaLayout />
            </AllowedRTRoute>
          }
        >
          <Route
            path="baru"
            element={<Navigate to="/warga" replace />}
          />
          <Route
            path=":id"
            element={<ResidentDetail />}
          />
          <Route
            path="household/baru"
            element={<HouseholdCreate />}
          />
          <Route
            path=":id/edit"
            element={<HouseholdEdit />}
          />
          <Route
            path=":id/pindah"
            element={<HouseholdMove />}
          />
          <Route
            path="special/baru"
            element={<SpecialResidentCreate />}
          />
          <Route
            path="special/:id/edit"
            element={<SpecialResidentEdit />}
          />
        </Route>
        <Route
          path="iuran/*"
          element={
            <AllowedRTRoute>
              <IuranRoutes />
            </AllowedRTRoute>
          }
        />
        <Route
          path="kas/*"
          element={
            <AllowedRTRoute>
              <KasRoutes />
            </AllowedRTRoute>
          }
        />
        <Route
          path="lokasi-dana/*"
          element={
            <AllowedRTRoute>
              <FundLocationRoutes />
            </AllowedRTRoute>
          }
        />
        <Route
          path="notifikasi"
          element={
            <NotificationAdminRoute>
              <NotificationScreen />
            </NotificationAdminRoute>
          }
        />
      </Route>

      <Route path="*" element={<Navigate to={isAuthenticated ? "/" : "/login"} replace />} />
    </Routes>

  )
}
