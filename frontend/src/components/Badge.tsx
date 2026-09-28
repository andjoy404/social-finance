import { type ReactNode } from 'react'

interface BadgeProps {
  children: ReactNode
  variant?: 'violet' | 'deepPurple' | 'lightPurple' | 'green' | 'red' | 'amber' | 'blue' | 'gray' | 'default'
  style?: React.CSSProperties
}

const variantStyles: Record<string, React.CSSProperties> = {
  violet: {
    color: `color-mix(in srgb, #a970ff ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #a970ff ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #a970ff ${17}%, var(--sf-bg)), color-mix(in srgb, #a970ff ${9}%, var(--sf-bg)))`,
  },
  deepPurple: {
    color: `color-mix(in srgb, #7c5ac7 ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #7c5ac7 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #7c5ac7 ${17}%, var(--sf-bg)), color-mix(in srgb, #7c5ac7 ${9}%, var(--sf-bg)))`,
  },
  lightPurple: {
    color: `color-mix(in srgb, #b388ff ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #b388ff ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #b388ff ${17}%, var(--sf-bg)), color-mix(in srgb, #b388ff ${9}%, var(--sf-bg)))`,
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
    color: `color-mix(in srgb, #3b82f6 ${82}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #3b82f6 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #3b82f6 ${17}%, var(--sf-bg)), color-mix(in srgb, #3b82f6 ${9}%, var(--sf-bg)))`,
  },
  gray: {
    color: `color-mix(in srgb, #6B7280 ${65}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #969BA5 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #6B7280 ${50}%, var(--sf-bg)), color-mix(in srgb, #6B7280 ${30}%, var(--sf-bg)))`,
  },
  default: {
    color: `var(--sf-text-muted)`,
    borderColor: `var(--sf-border)`,
    background: `var(--sf-bg)`,
  },
}

export function Badge({ children, variant = 'default', style }: BadgeProps) {
  const variantStyle = variantStyles[variant] ?? variantStyles.default

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        padding: '0 8px',
        fontSize: '11px',
        fontWeight: 400,
        borderRadius: '5px',
        border: '1px solid',
        lineHeight: 1.2,
        height: '22px',
        boxSizing: 'border-box',
        fontFamily: 'inherit',
        ...variantStyle,
        ...style,
      }}
    >
      {children}
    </span>
  )
}
