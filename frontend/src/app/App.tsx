import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { useAuth, isAllowedRT, isSuperAdminUser } from '@/app/AuthContext'
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
import { MonitoringScreen } from '@/features/monitoring/MonitoringScreen'
import { RwDataView } from '@/features/rw/RwDataView'
import { SecurityWasteTruckScreen } from '@/features/notification/SecurityWasteTruckScreen'
import { KeuanganScreen } from '@/features/keuangan/KeuanganScreen'
import { ActivityList } from '@/features/kegiatan/ActivityList'
import { ActivityDetail } from '@/features/kegiatan/ActivityDetail'
import { AnnouncementManagement } from '@/features/pengumuman/AnnouncementManagement'

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
  if (!isSuperAdminUser(user)) {
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

export function getOfficerRouteDestination(
  jabatan: string | null | undefined,
  pathname: string,
): string | null {
  const normalized = (jabatan ?? '').trim().toLowerCase()
  if (normalized !== 'keamanan' && normalized !== 'kebersihan') return null
  const defaultPath = normalized === 'keamanan' ? '/warga' : '/monitoring'
  if (pathname === '/') return defaultPath
  const allowed = normalized === 'keamanan'
    ? pathname === '/warga' ||
      pathname.startsWith('/warga/') ||
      pathname === '/notifikasi' ||
      pathname === '/monitoring'
    : pathname === '/monitoring'
  return allowed ? null : defaultPath
}

function OfficerAccessRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  const location = useLocation()
  if (!isAuthenticated) return <>{children}</>

  const destination = getOfficerRouteDestination(user?.jabatan, location.pathname)
  if (destination) return <Navigate to={destination} replace />
  return <>{children}</>
}

function NotificationEntryRoute() {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) return <Navigate to="/login" replace />
  const jabatan = (user?.jabatan ?? '').trim().toLowerCase()
  if (jabatan === 'kebersihan') {
    return <Navigate to="/monitoring" replace />
  }
  if (jabatan === 'keamanan') {
    const canDispatch = (user?.effectivePermissions ?? []).includes('notification.send_waste_truck')
    return <SecurityWasteTruckScreen canDispatch={canDispatch} />
  }
  return <NotificationScreen />
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
            <OfficerAccessRoute>
              <AppLayout />
            </OfficerAccessRoute>
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
          element={<NotificationEntryRoute />}
        />
        <Route
          path="rw-view/:rwId/*"
          element={
            <ProtectedRoute>
              <RwDataView />
            </ProtectedRoute>
          }
        />
        <Route
          path="monitoring"
          element={
            <AllowedRTRoute>
              <MonitoringScreen />
            </AllowedRTRoute>
          }
        />
        <Route
          path="keuangan"
          element={
            <AllowedRTRoute>
              <KeuanganScreen />
            </AllowedRTRoute>
          }
        />
        <Route
          path="kegiatan"
          element={
            <AllowedRTRoute>
              <ActivityList />
            </AllowedRTRoute>
          }
        />
        <Route
          path="kegiatan/:id"
          element={
            <AllowedRTRoute>
              <ActivityDetail />
            </AllowedRTRoute>
          }
        />
        <Route
          path="pengumuman"
          element={
            <AllowedRTRoute>
              <AnnouncementManagement />
            </AllowedRTRoute>
          }
        />
      </Route>

      <Route path="*" element={<Navigate to={isAuthenticated ? "/" : "/login"} replace />} />
    </Routes>

  )
}
