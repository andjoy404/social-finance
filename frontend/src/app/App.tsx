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
import { KelolaKeamananPage } from '@/features/keamanan/KelolaKeamananPage'
import { BukuTamuPage } from '@/features/keamanan/BukuTamuPage'
import { PatrolPage } from '@/features/keamanan/PatrolPage'
import { PaketPage } from '@/features/keamanan/PaketPage'
import { hasPermission } from '@/app/AuthContext'

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

// Iuran access route — all authenticated RT users can access Iuran personal view.
// Sekretaris RT can view their own bills and payments.
// Admin Iuran operations (Kelola Iuran, Iuran Warga) are blocked by the backend.
// Sosial and Operasional cannot access Kas or Iuran.
function IuranAccessRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  const jabatan = (user?.jabatan ?? '').trim().toLowerCase()
  if (jabatan === 'sosial' || jabatan === 'operasional') {
    return <Navigate to="/" replace />
  }
  if (!isAllowedRT(user)) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

function KasAccessRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  const jabatan = (user?.jabatan ?? '').trim().toLowerCase()
  if (jabatan === 'sosial' || jabatan === 'operasional') {
    return <Navigate to="/" replace />
  }
  if (!isAllowedRT(user)) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

function GuestBookRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  const jabatan = (user?.jabatan ?? '').trim().toLowerCase()
  const isSuperAdmin = user?.role === 'super_admin'
  const canAccess =
    isSuperAdmin ||
    jabatan === 'ketua' ||
    hasPermission(user, 'security.guest_book') ||
    hasPermission(user, 'security.device.manage') ||
    hasPermission(user, 'security.schedule.write')
  if (!canAccess) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

export function getOfficerRouteDestination(
  jabatan: string | null | undefined,
  pathname: string,
  rwMemberships?: { rw_id: string; rw_name: string }[],
  userRwId?: string | null,
): string | null {
  const normalized = (jabatan ?? '').trim().toLowerCase()

  // Sosial and Operasional: hide/block Kas and Iuran
  if (normalized === 'sosial' || normalized === 'operasional') {
    if (pathname.startsWith('/kas') || pathname.startsWith('/iuran')) {
      return '/'
    }
  }

  // Anggota RT: redirect to RW Dashboard, not Beranda.
  if (normalized === 'anggota' || normalized === 'anggota_rt') {
    const rwId = (rwMemberships && rwMemberships.length > 0)
      ? rwMemberships[0].rw_id
      : (userRwId ?? '')
    if (rwId) {
      const rwPath = `/rw-view/${rwId}/dashboard`
      if (pathname === '/' || pathname === '/login') return rwPath
      // Allow staying on rw-view, notifikasi; redirect away from other paths
      if (pathname.startsWith('/rw-view/') || pathname === '/notifikasi') return null
      return rwPath
    }
    return null // No RW membership — let RwDataView handle the error
  }

  if (normalized !== 'keamanan' && normalized !== 'kebersihan') return null
  const defaultPath = normalized === 'keamanan' ? '/warga' : '/monitoring'
  if (pathname === '/') return defaultPath
  const allowed = normalized === 'keamanan'
    ? pathname === '/warga' ||
      pathname.startsWith('/warga/') ||
      pathname === '/notifikasi' ||
      pathname === '/monitoring' ||
      pathname === '/patroli' ||
      pathname.startsWith('/patroli/') ||
      pathname === '/paket' ||
      pathname.startsWith('/paket/')
    : pathname === '/monitoring'
  return allowed ? null : defaultPath
}

function PatrolRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  const jabatan = (user?.jabatan ?? '').trim().toLowerCase()
  const isSuperAdmin = user?.role === 'super_admin'
  const isSekretaris = jabatan === 'sekretaris'
  if (isSuperAdmin || isSekretaris) {
    return <Navigate to="/" replace />
  }
  const canAccess = hasPermission(user, 'patrol.read') || hasPermission(user, 'patrol.create')
  if (!canAccess) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

function PaketRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  const jabatan = (user?.jabatan ?? '').trim().toLowerCase()
  const isSuperAdmin = user?.role === 'super_admin'
  const isSekretaris = jabatan === 'sekretaris'
  if (isSuperAdmin || isSekretaris) {
    return <Navigate to="/" replace />
  }
  const canAccess = hasPermission(user, 'paket.read') || hasPermission(user, 'paket.create')
  if (!canAccess) {
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

function OfficerAccessRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  const location = useLocation()
  if (!isAuthenticated) return <>{children}</>

  const destination = getOfficerRouteDestination(
    user?.jabatan,
    location.pathname,
    user?.rwMemberships,
    user?.rwId,
  )
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
          <Route
            path="special/:id"
            element={<SpecialResidentEdit mode="detail" />}
          />
        </Route>
        <Route
          path="iuran/*"
          element={
            <IuranAccessRoute>
              <IuranRoutes />
            </IuranAccessRoute>
          }
        />
        <Route
          path="kas/*"
          element={
            <KasAccessRoute>
              <KasRoutes />
            </KasAccessRoute>
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
          path="patroli"
          element={
            <PatrolRoute>
              <PatrolPage />
            </PatrolRoute>
          }
        />
        <Route
          path="paket"
          element={
            <PaketRoute>
              <PaketPage />
            </PaketRoute>
          }
        />
        <Route
          path="keamanan-rt"
          element={
            <AllowedRTRoute>
              <KelolaKeamananPage />
            </AllowedRTRoute>
          }
        />
        <Route
          path="buku-tamu"
          element={
            <GuestBookRoute>
              <BukuTamuPage />
            </GuestBookRoute>
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
