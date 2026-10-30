import { useState, useCallback } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth, isAllowedRT, isSuperAdminUser, canAccessNotificationMenu } from '@/app/AuthContext'
import {
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  DashboardOutlined,
  UserOutlined,
  ApartmentOutlined,
  AccountBookOutlined,
  DollarOutlined,
  BankOutlined,
  BellOutlined,
} from '@ant-design/icons'

function isActive(route: string, pathname: string): boolean {
  if (route === '/') return pathname === '/'
  return pathname.startsWith(route)
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

  const items: NavItem[] = [
    { to: '/', label: 'Beranda', icon: <DashboardOutlined />, id: 'dashboard' },
  ]

    if (allowedRT) {
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
}

  const canAccessNotifikasi = canAccessNotificationMenu(user)
  if (canAccessNotifikasi) {
    items.push({
      to: '/notifikasi',
      label: 'Notifikasi',
      icon: <BellOutlined />,
      id: 'notifikasi',
    })
  }

  if (isSuperAdmin) {
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
            const active = isActive(item.to, location.pathname)
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
