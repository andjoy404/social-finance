interface SectionHeaderProps {
  title: string
  subtitle?: string
}

export function SectionHeader({ title, subtitle }: SectionHeaderProps) {
  return (
    <div>
      <h3
        style={{
          fontSize: '15px',
          fontWeight: 600,
          color: 'var(--text-primary)',
          marginBottom: '2px',
        }}
      >
        {title}
      </h3>
      {subtitle && (
        <p
          style={{
            fontSize: '12px',
            color: 'var(--text-muted)',
          }}
        >
          {subtitle}
        </p>
      )}
    </div>
  )
}
