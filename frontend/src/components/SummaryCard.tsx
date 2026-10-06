import { formatRupiah } from '@/styles/format'

interface SummaryCardProps {
  label: string
  value: string | number
  accent?: 'income' | 'expense' | 'default'
  className?: string
}

export function SummaryCard({ label, value, accent = 'default', className = '' }: SummaryCardProps) {
  const colorMap: Record<string, string> = {
    income: 'var(--sf-success)',
    expense: 'var(--sf-danger)',
    default: 'var(--sf-accent)',
  }

  const color = colorMap[accent]
  const displayValue = typeof value === 'number' ? formatRupiah(value) : value

  return (
    <div className={className}>
      <p
        style={{
          fontSize: '13px',
          color: 'var(--sf-text-muted)',
          marginBottom: 'var(--sf-sp-xs)',
          fontWeight: 500,
        }}
      >
        {label}
      </p>
      <p
        style={{
          fontSize: '28px',
          fontWeight: 700,
          color,
          lineHeight: 1.2,
        }}
      >
        {displayValue}
      </p>
    </div>
  )
}
