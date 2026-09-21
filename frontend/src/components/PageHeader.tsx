import React from 'react'
import { Typography } from 'antd'

const { Text } = Typography

interface PageHeaderProps {
  icon?: React.ReactNode
  title: string
  subtitle?: string
  actions?: React.ReactNode
  surface?: boolean
}

export function PageHeader({ icon, title, subtitle, actions, surface = false }: PageHeaderProps) {
  const rootClassName = surface ? 'sf-page-header-surface' : 'sf-page-header'

  return (
    <div className={rootClassName}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        {icon && <span className="sf-page-header-icon">{icon}</span>}
        <div>
          <div className={surface ? 'sf-page-header-title-text' : 'sf-page-header-title'}>{title}</div>
          {subtitle && <Text type="secondary" className={surface ? 'sf-page-header-subtitle' : 'sf-page-header-subtitle'}>{subtitle}</Text>}
        </div>
      </div>
      {actions && <div className="sf-page-header-actions">{actions}</div>}
    </div>
  )
}
