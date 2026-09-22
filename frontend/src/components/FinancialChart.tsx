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
          formatter={(value: number) => [`Rp ${value.toLocaleString('id-ID')}`, '']}
          contentStyle={{
            background: 'var(--sf-surface)',
            border: '1px solid var(--sf-border)',
            borderRadius: 'var(--sf-radius-sm)',
            fontSize: '12px',
            boxShadow: 'var(--sf-shadow-sm)',
            padding: '6px 10px',
          }}
          itemStyle={{ color: 'var(--sf-text)' }}
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
