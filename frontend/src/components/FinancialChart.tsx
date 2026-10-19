import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'

interface FinancialDataItem {
  month: string
  income: number
  expense: number
}

interface FinancialChartProps {
  data: FinancialDataItem[]
}

const FULL_MONTH_MAP: Record<string, string> = {
  Jan: 'Januari',
  Feb: 'Februari',
  Mar: 'Maret',
  Apr: 'April',
  Mei: 'Mei',
  Jun: 'Juni',
  Jul: 'Juli',
  Agu: 'Agustus',
  Sep: 'September',
  Okt: 'Oktober',
  Nov: 'November',
  Des: 'Desember',
}

function CustomFinancialTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean
  payload?: Array<{ dataKey?: string; value?: number }>
  label?: string
}) {
  if (!active || !payload || payload.length === 0) return null

  const incomeItem = payload.find((p) => p.dataKey === 'income')
  const expenseItem = payload.find((p) => p.dataKey === 'expense')
  const incomeVal = Number(incomeItem?.value ?? 0)
  const expenseVal = Number(expenseItem?.value ?? 0)
  const fullMonth = (label && FULL_MONTH_MAP[label]) ? FULL_MONTH_MAP[label] : (label ?? '')

  return (
    <div
      style={{
        background: 'var(--sf-surface)',
        border: '1px solid var(--sf-border)',
        borderRadius: 'var(--sf-radius-sm)',
        fontSize: '12px',
        boxShadow: 'var(--sf-shadow-sm)',
        padding: '8px 12px',
        color: 'var(--sf-text)',
        minWidth: 160,
      }}
    >
      <div style={{ fontWeight: 700, marginBottom: 4, color: 'var(--sf-text)' }}>
        {fullMonth}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, color: 'var(--sf-success)', fontWeight: 600 }}>
        <span>Masuk</span>
        <span style={{ fontFamily: 'monospace' }}>Rp.{incomeVal.toLocaleString('id-ID')}</span>
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, color: 'var(--sf-danger)', fontWeight: 600, marginTop: 2 }}>
        <span>Keluar</span>
        <span style={{ fontFamily: 'monospace' }}>Rp.{expenseVal.toLocaleString('id-ID')}</span>
      </div>
    </div>
  )
}

export function FinancialChart({ data }: FinancialChartProps) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <BarChart
        data={data}
        margin={{ top: 4, right: 16, bottom: 4, left: 0 }}
      >
        <CartesianGrid
          strokeDasharray="3 3"
          vertical={false}
          style={{ stroke: 'var(--sf-border-subtle)' }}
        />
        <XAxis
          dataKey="month"
          tick={{ fill: 'var(--sf-text-muted)', fontSize: 11, fontWeight: 500 }}
          axisLine={{ stroke: 'var(--sf-border)' }}
          tickLine={false}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          tick={{ fill: 'var(--sf-text-muted)', fontSize: 11 }}
          tickCount={6}
          tickFormatter={(v: number) => {
            const millions = v / 1000000
            if (millions >= 1) return `${millions} jt`
            if (v > 0) return `${(v / 1000).toFixed(0)} rbh`
            return '0'
          }}
        />
        <Tooltip
          content={<CustomFinancialTooltip />}
          cursor={{ fill: 'var(--sf-accent)', fillOpacity: 0.10 }}
        />
        <Legend
          wrapperStyle={{ fontSize: '11px', paddingTop: '4px' }}
          iconType="circle"
          iconSize={6}
        />
        <Bar
          dataKey="income"
          name="Pemasukan"
          fill="var(--sf-chart-success)"
          radius={[4, 4, 0, 0]}
          maxBarSize={36}
        />
        <Bar
          dataKey="expense"
          name="Pengeluaran"
          fill="var(--sf-chart-danger)"
          radius={[4, 4, 0, 0]}
          maxBarSize={36}
        />
      </BarChart>
    </ResponsiveContainer>
  )
}

