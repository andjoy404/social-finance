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
  ScheduleOutlined,
} from '@ant-design/icons'

function isActive(route: string, pathname: string, itemId?: string): boolean {
  if (route === '/') return pathname === '/'
  if (itemId === 'rw-data') {
    return pathname.startsWith('/rw-view')
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
      return localStorage.getItem('sf_sidebar_collapsed') === 'true'
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

  // Permission-driven, not role-name driven: only holders of the relevant
  // management permission (or super admin) see the management entry.
  const canManageCommunity =
    hasPermission(user, 'announcements.create') ||
    hasPermission(user, 'announcements.update')

  const items: NavItem[] = isSecurityOfficer || isCleaningOfficer
    ? []
    : [{ to: '/', label: 'Beranda', icon: <HomeOutlined />, id: 'dashboard' }]

  if (isSecurityOfficer) {
    items.push(
      { to: '/warga', label: 'Warga', icon: <UserOutlined />, id: 'warga' },
      {
        to: '/monitoring',
        label: 'Monitoring',
        icon: <ScheduleOutlined />,
        id: 'monitoring',
      },
    )
  } else if (isCleaningOfficer) {
    items.push({
      to: '/monitoring',
      label: 'Monitoring',
      icon: <ScheduleOutlined />,
      id: 'monitoring',
    })
  } else if (allowedRT) {
    items.push(
      { to: '/keuangan', label: 'Keuangan', icon: <DashboardOutlined />, id: 'keuangan' },
      { to: '/kegiatan', label: 'Kegiatan', icon: <CalendarOutlined />, id: 'kegiatan' },
    )
    if (canManageCommunity) {
      items.push({
        to: '/pengumuman',
        label: 'Kelola Pengumuman',
        icon: <NotificationOutlined />,
        id: 'pengumuman',
      })
    }
    const canAccessLokasiDana =
        user?.role !== 'warga' || (user?.jabatan?.trim() ?? '') !== ''
    items.push(
        { to: '/kas', label: 'Kas', icon: <DollarOutlined />, id: 'kas' },
        { to: '/iuran', label: 'Iuran', icon: <AccountBookOutlined />, id: 'iuran' },
    )
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
        label: 'Monitoring',
        icon: <ScheduleOutlined />,
        id: 'monitoring',
      })
    }
}

  const canAccessNotifikasi =
    !isCleaningOfficer &&
    (isSecurityOfficer || isResident || isPengurus || canAccessNotificationAdmin(user))
  if (canAccessNotifikasi) {
    items.push({
      to: '/notifikasi',
      label: 'Notifikasi',
      icon: <BellOutlined />,
      id: 'notifikasi',
    })
  }

  const rawRwMemberships: RWMembership[] = user?.rwMemberships ?? []
  const rwMemberships = rawRwMemberships.filter(
    (m) => isSuperAdmin || (m.jabatan && ['ketua', 'wakil_ketua', 'sekretaris', 'bendahara'].includes(m.jabatan.toLowerCase()))
  )

  // Add RW data view link(s)
  if (!isSecurityOfficer && !isCleaningOfficer && rwMemberships.length > 0) {
    const rwIdForData = rwMemberships[0].rw_id
    // Build a label showing RW count if multiple
    const rwLabel = rwMemberships.length === 1
      ? 'Data RW'
      : `Data RW (${rwMemberships.length})`

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
                key={item.to}
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
