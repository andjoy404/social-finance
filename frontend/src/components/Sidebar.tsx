import { useState, useCallback } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth, isAllowedRT, isSuperAdminUser, canAccessNotificationAdmin, hasPermission, type RWMembership } from '@/app/AuthContext'
import { DownOutlined } from '@ant-design/icons'
import {
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  DashboardOutlined,
  HomeOutlined,
  CalendarOutlined,
  NotificationOutlined,
  UserOutlined,
  ApartmentOutlined,
  AccountBookOutlined,
  DollarOutlined,
  BankOutlined,
  BellOutlined,
  AreaChartOutlined,
  SafetyCertificateOutlined,
  BookOutlined,
  ScheduleOutlined,
  SecurityScanOutlined,
  InboxOutlined,
} from '@ant-design/icons'

function isActive(route: string, pathname: string, itemId?: string): boolean {
  if (route === '/') return pathname === '/'
  if (itemId === 'kas') {
    return pathname === route || pathname.startsWith(route + '/') || (pathname.includes('/kas') && !pathname.includes('lokasi'))
  }
  if (itemId === 'keuangan') {
    return pathname === route || (pathname === '/keuangan' || pathname.startsWith('/keuangan/'))
  }
  if (itemId === 'rw-data') {
    return pathname.startsWith('/rw-view') && !pathname.includes('/kas')
  }
  return pathname === route || pathname.startsWith(route + '/')
}

type NavItem = {
  to: string
  label: string
  icon: React.ReactNode
  id: string
  onlySuperAdmin?: boolean
}

export function Sidebar() {
  const { user } = useAuth()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState<boolean>(() => {
    try {
      const stored = localStorage.getItem('sf_sidebar_collapsed')
      if (stored !== null) return stored === 'true'
      // Default to collapsed on small screens so the sidebar overlay
      // does not cover content before the user pins/unpins it.
      return typeof window !== 'undefined' && window.innerWidth <= 768
    } catch {
      return false
    }
  })

  const toggleCollapsed = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev
      try {
        localStorage.setItem('sf_sidebar_collapsed', String(next))
      } catch { /* ignore */ }
      return next
    })
  }, [])

  const isSuperAdmin = isSuperAdminUser(user)
  const allowedRT = isAllowedRT(user)
  const jabatan = (user?.jabatan ?? '').trim().toLowerCase()
  const isSecurityOfficer = jabatan === 'keamanan'
  const isCleaningOfficer = jabatan === 'kebersihan'
  const isResident =
    user?.role.trim().toLowerCase() === 'warga' && !jabatan && allowedRT
  const isPengurus = user?.role.trim().toLowerCase() === 'pengurus' && allowedRT

  const isAnggota = jabatan === 'anggota' || jabatan === 'anggota_rt' || jabatan === 'anggota_rw'

  const rawRwMemberships: RWMembership[] = user?.rwMemberships ?? []
  // Include anggota, anggota_rt, anggota_rw, operasional, humas, and sosial in RW membership filter so they can access RW Dashboard and RW Kas READ.
  const rwMemberships = rawRwMemberships.filter(
    (m) => isSuperAdmin || (m.jabatan && ['ketua', 'wakil_ketua', 'sekretaris', 'bendahara', 'anggota', 'anggota_rt', 'anggota_rw', 'operasional', 'humas', 'sosial'].includes(m.jabatan.toLowerCase()))
  )

  const isReadOnlyRW = isAnggota || jabatan === 'operasional' || jabatan === 'humas' || jabatan === 'sosial'
  const effectiveRwMemberships = rwMemberships.length > 0
    ? rwMemberships
    : (isReadOnlyRW && user?.rwId
        ? [{
            rw_id: user.rwId,
            rw_name: user.rwName ?? undefined,
            rw_number: user.rwNumber ?? undefined,
            jabatan: user.jabatan ?? (jabatan === 'anggota_rw' ? 'anggota_rw' : (jabatan === 'operasional' ? 'operasional' : (jabatan === 'humas' ? 'humas' : (jabatan === 'sosial' ? 'sosial' : 'anggota_rt')))),
            source: 'rt_inferred' as const,
          }]
        : [])
  const userRwId = effectiveRwMemberships[0]?.rw_id || user?.rwId || ''

  const items: NavItem[] = isSecurityOfficer || isCleaningOfficer || isAnggota
    ? []
    : [{ to: '/', label: 'Beranda', icon: <HomeOutlined />, id: 'dashboard' }]

  if (isSecurityOfficer) {
    items.push(
      { to: '/warga', label: 'Warga', icon: <UserOutlined />, id: 'warga' },
      { to: '/patroli', label: 'Patroli', icon: <SecurityScanOutlined />, id: 'patroli' },
      {
        to: '/monitoring',
        label: 'Sampah',
        icon: <ScheduleOutlined />,
        id: 'monitoring',
      },
      { to: '/paket', label: 'Paket', icon: <InboxOutlined />, id: 'paket' },
    )
  } else if (isCleaningOfficer) {
    items.push({
      to: '/monitoring',
      label: 'Sampah',
      icon: <ScheduleOutlined />,
      id: 'monitoring',
    })
  } else if (isAnggota) {
    // Anggota RT & Anggota RW: simplified sidebar — only Data RW and Notifikasi.
    // Kas RT, Iuran, Keamanan, and other RT features are hidden.
  } else if (allowedRT) {
    const isRestrictedFinance = jabatan === 'sosial' || jabatan === 'operasional'
    const isHumas = jabatan === 'humas'
    if (!isRestrictedFinance) {
      items.push({
        to: isHumas && userRwId ? `/rw-view/${userRwId}/dashboard` : '/keuangan',
        label: 'Keuangan',
        icon: <DashboardOutlined />,
        id: 'keuangan',
      })
    }
    const canAccessLokasiDana =
        (user?.role !== 'warga' || (user?.jabatan?.trim() ?? '') !== '') &&
        !isRestrictedFinance &&
        !isHumas
    // Sekretaris RT can access Kas (transactions) AND Iuran personal view (own bills/payments).
    // Admin Iuran operations (Kelola Iuran, Iuran Warga) are blocked by the backend.
    // Sosial and Operasional can access Iuran (own bills/payments) but NOT Kas.
    // Humas accesses Kas RW (read-only) via /rw-view/${userRwId}/kas.
    if (!isRestrictedFinance) {
      items.push({
        to: isHumas && userRwId ? `/rw-view/${userRwId}/kas` : '/kas',
        label: 'Kas',
        icon: <DollarOutlined />,
        id: 'kas',
      })
    }
    items.push({ to: '/iuran', label: 'Iuran', icon: <AccountBookOutlined />, id: 'iuran' })
    if (canAccessLokasiDana) {
        items.push({
            to: '/lokasi-dana',
            label: 'Lokasi Dana',
            icon: <BankOutlined />,
            id: 'lokasi-dana',
        })
    }
    items.push(
        { to: '/warga', label: 'Warga', icon: <UserOutlined />, id: 'warga' },
    )
    if (jabatan === 'ketua') {
      items.push({
        to: '/monitoring',
        label: 'Sampah',
        icon: <ScheduleOutlined />,
        id: 'monitoring',
      })
    }
    const canManageSecurity =
      isSuperAdmin ||
      jabatan === 'ketua' ||
      jabatan === 'humas' ||
      hasPermission(user, 'security.schedule.write') ||
      hasPermission(user, 'security.device.manage')
    if (canManageSecurity) {
      items.push({
        to: '/keamanan-rt',
        label: 'Keamanan',
        icon: <SafetyCertificateOutlined />,
        id: 'keamanan-rt',
      })
    }
    const canAccessGuestBook =
      jabatan !== 'humas' &&
      (isSuperAdmin ||
      jabatan === 'ketua' ||
      hasPermission(user, 'security.guest_book'))
    if (canAccessGuestBook) {
      items.push({
        to: '/buku-tamu',
        label: 'Buku Tamu',
        icon: <BookOutlined />,
        id: 'buku-tamu',
      })
    }
    const isSekretaris = jabatan === 'sekretaris'
    const canAccessPatrol = !isSuperAdmin && !isSekretaris && (hasPermission(user, 'patrol.read') || hasPermission(user, 'patrol.create'))
    const canAccessPaket = !isSuperAdmin && !isSekretaris && (hasPermission(user, 'paket.read') || hasPermission(user, 'paket.create'))

    if (canAccessPatrol) {
      items.push({
        to: '/patroli',
        label: 'Patroli',
        icon: <SecurityScanOutlined />,
        id: 'patroli',
      })
    }
    if (canAccessPaket) {
      items.push({
        to: '/paket',
        label: 'Paket',
        icon: <InboxOutlined />,
        id: 'paket',
      })
    }
  }

  // Add RW data view link(s)
  if (!isSecurityOfficer && !isCleaningOfficer && effectiveRwMemberships.length > 0) {
    const rwIdForData = effectiveRwMemberships[0].rw_id
    // Build a label showing RW count if multiple
    const rwLabel = effectiveRwMemberships.length === 1
      ? 'Data RW'
      : `Data RW (${effectiveRwMemberships.length})`

    items.push({
      to: `/rw-view/${rwIdForData}/dashboard`,
      label: rwLabel,
      icon: <AreaChartOutlined />,
      id: 'rw-data',
      onlySuperAdmin: false,
    })
  }

  if (isSuperAdmin && !isSecurityOfficer && !isCleaningOfficer) {
    items.push(
      { to: '/rt', label: 'Kelola RT', icon: <ApartmentOutlined />, id: 'rt' },
      { to: '/rw', label: 'Kelola RW', icon: <ApartmentOutlined />, id: 'rw' },
    )
  }

  const canAccessNotifikasi =
    !isCleaningOfficer &&
    (isSecurityOfficer || isResident || isPengurus || isAnggota || jabatan === 'humas' || jabatan === 'operasional' || user?.role?.trim().toLowerCase() === 'bendahara' || canAccessNotificationAdmin(user))

  // Place Pengumuman, Kegiatan, Notifikasi as the three bottom-most menu items
  // in that exact order.
  // Non-allowed/non-officer views only have Beranda; do not append the
  // community pair there (preserves prior restricted visibility).
  const showCommunityNav = !isSecurityOfficer && !isCleaningOfficer && !isAnggota && allowedRT
  if (showCommunityNav) {
    items.push(
      { to: '/pengumuman', label: 'Pengumuman', icon: <NotificationOutlined />, id: 'pengumuman' },
      { to: '/kegiatan', label: 'Kegiatan', icon: <CalendarOutlined />, id: 'kegiatan' },
    )
  }
  if (canAccessNotifikasi) {
    items.push({
      to: '/notifikasi',
      label: 'Notifikasi',
      icon: <BellOutlined />,
      id: 'notifikasi',
    })
  }

  return (
    <div
      className="sf-sidebar-slot"
      style={{ width: collapsed ? 56 : 240 }}
    >
      <nav className="sf-sidebar" aria-label="Navigasi utama">
        {/* Brand */}
        <div className="sidebar-brand">
          <span
            className="sidebar-brand-title"
            style={{ flex: 1, overflow: 'hidden', whiteSpace: 'nowrap', minWidth: 0 }}
          >
            {!collapsed && 'Social Finance'}
          </span>
          <button
            className="sidebar-pin-btn"
            onClick={toggleCollapsed}
            aria-label={collapsed ? 'Perluas sidebar' : 'Sematkan sidebar'}
            title={collapsed ? 'Perluas sidebar' : 'Sematkan sidebar'}
          >
            {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          </button>
        </div>

        {/* Nav */}
        <div className="sidebar-nav">
          {items.map((item) => {
            const active = isActive(item.to, location.pathname, item.id)

            // Render RW memberships dropdown for multi-RW users
            const isRWDataItem = item.id === 'rw-data'
            if (isRWDataItem && rwMemberships.length > 1) {
              return (
                <div key={item.id}>
                  <Link
                    to={item.to}
                    className={`nav-item${active ? ' nav-item-active' : ''}`}
                    style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}
                  >
                    <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span className="nav-item-icon">{item.icon}</span>
                      {!collapsed && <span className="nav-item-label">{item.label}</span>}
                    </span>
                    {!collapsed && <DownOutlined style={{ fontSize: 12 }} />}
                  </Link>
                  <div style={{ paddingLeft: 24, marginTop: 4 }}>
                    {rwMemberships.map((m) => {
                      const rwPath = `/rw-view/${m.rw_id}/dashboard`
                      const rwActive = location.pathname.startsWith(`/rw-view/${m.rw_id}`)
                      return (
                        <Link
                          key={m.rw_id}
                          to={rwPath}
                          className={`nav-item${rwActive ? ' nav-item-active' : ''}`}
                          style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, padding: '6px 8px' }}
                        >
                          <span className="nav-item-icon" style={{ fontSize: 12 }}>{item.icon}</span>
                          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {m.rw_name || `RW ${String(m.rw_number ?? '').padStart(3, '0')}`}{' '}
                            {m.jabatan && <span style={{ color: 'var(--sf-text-muted)', fontSize: 11 }}>({m.jabatan})</span>}
                          </span>
                        </Link>
                      )
                    })}
                  </div>
                </div>
              )
            }

            return (
              <Link
                key={item.id}
                to={item.to}
                className={`nav-item${active ? ' nav-item-active' : ''}`}
              >
                <span className="nav-item-icon">{item.icon}</span>
                {!collapsed && <span className="nav-item-label">{item.label}</span>}
              </Link>
            )
          })}
        </div>
      </nav>
    </div>
  )
}
