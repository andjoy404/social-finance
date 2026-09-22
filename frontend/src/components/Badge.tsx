import { type ReactNode } from 'react'

interface BadgeProps {
  children: ReactNode
  variant?: 'violet' | 'green' | 'red' | 'amber' | 'blue' | 'default'
}

const variantStyles: Record<string, React.CSSProperties> = {
  violet: {
    color: `color-mix(in srgb, #a970ff ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #a970ff ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #a970ff ${17}%, var(--sf-bg)), color-mix(in srgb, #a970ff ${9}%, var(--sf-bg)))`,
  },
  green: {
    color: `color-mix(in srgb, #34c759 ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #34c759 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #34c759 ${17}%, var(--sf-bg)), color-mix(in srgb, #34c759 ${9}%, var(--sf-bg)))`,
  },
  red: {
    color: `color-mix(in srgb, #ff3b30 ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #ff3b30 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #ff3b30 ${17}%, var(--sf-bg)), color-mix(in srgb, #ff3b30 ${9}%, var(--sf-bg)))`,
  },
  amber: {
    color: `color-mix(in srgb, #ff9f0a ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #ff9f0a ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #ff9f0a ${17}%, var(--sf-bg)), color-mix(in srgb, #ff9f0a ${9}%, var(--sf-bg)))`,
  },
  blue: {
    color: `color-mix(in srgb, #60a5fa ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #3b82f6 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #3b82f6 ${17}%, var(--sf-bg)), color-mix(in srgb, #3b82f6 ${9}%, var(--sf-bg)))`,
  },
  default: {
    color: `var(--sf-text-muted)`,
    borderColor: `var(--sf-border)`,
    background: `var(--sf-bg)`,
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
        fontWeight: 400,
        borderRadius: '5px',
        border: '1px solid',
        lineHeight: '20px',
        textTransform: 'capitalize',
        fontFamily: 'inherit',
        ...style,
      }}
    >
      {children}
    </span>
  )
}
