interface AppCardProps {
  children: React.ReactNode
  style?: React.CSSProperties
}

export function AppCard({ children, style }: AppCardProps) {
  return (
    <div
      style={{
        background: 'var(--bg-surface)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        padding: 'var(--sp-md)',
        boxShadow: 'var(--shadow-xs)',
        ...style,
      }}
    >
      {children}
    </div>
  )
}
