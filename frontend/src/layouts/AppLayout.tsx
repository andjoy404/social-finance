import { Outlet } from 'react-router-dom'
import { useAuth } from '@/app/AuthContext'
import { Sidebar } from '@/components/Sidebar'
import { ThemeMenu } from '@/components/ThemeMenu'
import { LogoutOutlined, UserOutlined } from '@ant-design/icons'
import { Dropdown, Avatar } from 'antd'

function getInitials(name?: string): string {
  if (!name || !name.trim()) return ''
  const parts = name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

export function AppLayout() {
  const { user, logout } = useAuth()

  if (!user) return null

  const handleLogout = () => {
    logout()
  }

  const menuItems = [
    {
      key: 'logout',
      icon: <LogoutOutlined style={{ fontSize: 14 }} />,
      label: 'Keluar',
    },
  ]

  const handleMenuClick = (e: { key: string }) => {
    if (e.key === 'logout') {
      handleLogout()
    }
  }

  return (
    <div className="sf-shell-layout">
      {/* Full-width global header */}
      <header className="sf-header">
        <div className="sf-header-left">
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
              <Avatar
                size={28}
                style={{
                  backgroundColor: 'var(--sf-accent)',
                  fontSize: 12,
                  fontWeight: 600,
                }}
                icon={<UserOutlined />}
              >
                {getInitials(user.name)}
              </Avatar>
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
          <Outlet />
        </main>
      </div>
    </div>
  )
}
