import { type ReactNode } from 'react'

interface BadgeProps {
  children: ReactNode
  variant?: 'violet' | 'green' | 'red' | 'amber' | 'blue' | 'default'
}

const variantStyles: Record<string, React.CSSProperties> = {
  violet: {
    background: 'var(--primary-subtle)',
    color: 'var(--primary)',
    borderColor: 'var(--primary-border)',
  },
  green: {
    background: 'var(--color-income-subtle)',
    color: 'var(--color-income)',
    borderColor: 'rgba(52,199,89,0.3)',
  },
  red: {
    background: 'var(--color-expense-subtle)',
    color: 'var(--color-expense)',
    borderColor: 'rgba(255,59,48,0.3)',
  },
  amber: {
    background: 'var(--color-warning-subtle)',
    color: 'var(--color-warning)',
    borderColor: 'rgba(255,159,10,0.3)',
  },
  blue: {
    background: 'rgba(57,160,255,0.12)',
    color: 'var(--dashboard-accent, #7c5ac7)',
    borderColor: 'rgba(57,160,255,0.35)',
  },
  default: {
    background: 'var(--bg-sidebar-active)',
    color: 'var(--text-secondary)',
    borderColor: 'var(--border-color)',
  },
}

export function Badge({ children, variant = 'default' }: BadgeProps) {
  const style = variantStyles[variant] ?? variantStyles.default

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        padding: '1px 8px',
        fontSize: '11px',
        fontWeight: 600,
        borderRadius: '5px',
        border: '1px solid',
        lineHeight: '20px',
        ...style,
      }}
    >
      {children}
    </span>
  )
}
