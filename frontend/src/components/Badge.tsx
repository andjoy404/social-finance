import { type ReactNode } from 'react'

interface BadgeProps {
  children: ReactNode
  variant?: 'violet' | 'green' | 'red' | 'amber' | 'blue' | 'default'
}

const variantStyles: Record<string, React.CSSProperties> = {
  violet: {
    color: `color-mix(in srgb, var(--sf-accent) ${78}, var(--sf-text))`,
    borderColor: `color-mix(in srgb, var(--sf-accent) ${72}, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, var(--sf-accent) ${16}, var(--sf-bg)), color-mix(in srgb, var(--sf-accent) ${9}, var(--sf-bg)))`,
  },
  green: {
    color: `color-mix(in srgb, #34c759 ${76}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #34c759 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #34c759 ${14}%, var(--sf-bg)), color-mix(in srgb, #34c759 ${9}%, var(--sf-bg)))`,
  },
  red: {
    color: `color-mix(in srgb, #ff3b30 ${76}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #ff3b30 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #ff3b30 ${14}%, var(--sf-bg)), color-mix(in srgb, #ff3b30 ${9}%, var(--sf-bg)))`,
  },
  amber: {
    color: `color-mix(in srgb, #ff9f0a ${76}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #ff9f0a ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #ff9f0a ${14}%, var(--sf-bg)), color-mix(in srgb, #ff9f0a ${9}%, var(--sf-bg)))`,
  },
  blue: {
    color: `color-mix(in srgb, #60a5fa ${76}%, var(--sf-text))`,
    borderColor: `color-mix(in srgb, #3b82f6 ${70}%, var(--sf-border))`,
    background: `linear-gradient(145deg, color-mix(in srgb, #3b82f6 ${14}%, var(--sf-bg)), color-mix(in srgb, #3b82f6 ${9}%, var(--sf-bg)))`,
  },
  default: {
    color: `color-mix(in srgb, var(--sf-text-muted) ${72}%, transparent)`,
    borderColor: `color-mix(in srgb, var(--sf-text-muted) ${30}%, transparent)`,
    background: `color-mix(in srgb, var(--sf-text-muted) ${10}%, var(--sf-bg))`,
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
