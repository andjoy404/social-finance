import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { useAuth } from '@/app/AuthContext'
import { Sidebar } from '@/components/Sidebar'
import { OverdueWarningCard } from '@/components/OverdueWarningCard'
import { ThemeMenu } from '@/components/ThemeMenu'
import { ResidentAvatar } from '@/components/ResidentAvatar'
import { LogoutOutlined, UserOutlined } from '@ant-design/icons'
import { Dropdown } from 'antd'
import logoImg from '@/assets/logo.png'
import { ProfileModal } from '@/features/profile/ProfileModal'

export function AppLayout() {
  const { user, logout } = useAuth()
  const [isProfileOpen, setIsProfileOpen] = useState(false)

  if (!user) return null

  const handleLogout = () => {
    logout()
  }

  const menuItems = [
    {
      key: 'profile',
      icon: <UserOutlined style={{ fontSize: 14 }} />,
      label: 'Profil',
    },
    {
      key: 'logout',
      icon: <LogoutOutlined style={{ fontSize: 14 }} />,
      label: 'Keluar',
    },
  ]

  const handleMenuClick = (e: { key: string }) => {
    if (e.key === 'profile') {
      setIsProfileOpen(true)
    } else if (e.key === 'logout') {
      handleLogout()
    }
  }

  return (
    <div className="sf-shell-layout">
      {/* Full-width global header */}
      <header className="sf-header">
        <div className="sf-header-left">
          <img
            src={logoImg}
            alt="Social Finance Logo"
            className="sf-header-logo"
            style={{ width: 28, height: 28, borderRadius: 6, objectFit: 'contain' }}
          />
          <span className="sf-header-brand">Social Finance</span>
        </div>
        <div className="sf-header-right">
          <ThemeMenu />

          <Dropdown
            className="sf-user-dropdown"
            menu={{ items: menuItems, onClick: handleMenuClick }}
            placement="bottomRight"
            overlayClassName="sf-user-dropdown-menu"
          >
            <div className="header-user-btn">
              <ResidentAvatar
                name={user.name}
                avatarPath={user.avatarPath ?? null}
                size={28}
                className="header-user-avatar"
              />
              <div className="header-user-info">
                <span className="header-user-name">{user.name}</span>
                <span className="header-user-role">{user.role}</span>
              </div>
              <span className="header-user-chevron" />
            </div>
          </Dropdown>
        </div>
      </header>

      {/* Sidebar + Content below header */}
      <div className="sf-shell-body">
        <Sidebar />

        <main className="sf-content">
          <OverdueWarningCard />
          <Outlet />
        </main>
      </div>

      <ProfileModal open={isProfileOpen} onClose={() => setIsProfileOpen(false)} />
    </div>
  )
}
