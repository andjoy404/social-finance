import { useState, useEffect } from 'react'
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip as RechartsTooltip } from 'recharts'
import { useAuth } from '@/app/AuthContext'
import { formatRupiah } from '@/styles/format'
import { FinancialChart } from '@/components/FinancialChart'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { cleanKasDescription } from '@/features/kas/KasList'
import { formatRupiah as formatKasRupiah } from '@/features/kas/kasTypes'
import { DashboardOutlined, LeftOutlined, RightOutlined } from '@ant-design/icons'
import { apiGetDashboardSummary, ApiDashboardSummary, getSessionPair } from '@/app/api'

const MONTH_NAMES = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

function roleLabel(sys?: string | null, tenant?: string): string {
  if (sys === 'super_admin') return 'Super Admin'
  const labels: Record<string, string> = { pengurus: 'Pengurus', bendahara: 'Bendahara', warga: 'Warga' }
  return labels[tenant ?? ''] ?? tenant ?? ''
}

export function Dashboard() {
  const { user } = useAuth()
  const now = new Date()
  const [selectedMonth, setSelectedMonth] = useState<number>(now.getMonth() + 1)
  const [selectedYear, setSelectedYear] = useState<number>(now.getFullYear())
  const [isAllPeriods, setIsAllPeriods] = useState<boolean>(false)
  const [data, setData] = useState<ApiDashboardSummary | null>(null)
  const [loading, setLoading] = useState(true)

  const activePeriodStr = isAllPeriods
    ? 'all'
    : `${selectedYear}-${String(selectedMonth).padStart(2, '0')}`

  const handlePrevMonth = () => {
    setIsAllPeriods(false)
    if (selectedMonth === 1) {
      setSelectedMonth(12)
      setSelectedYear((y) => y - 1)
    } else {
      setSelectedMonth((m) => m - 1)
    }
  }

  const handleNextMonth = () => {
    setIsAllPeriods(false)
    if (selectedMonth === 12) {
      setSelectedMonth(1)
      setSelectedYear((y) => y + 1)
    } else {
      setSelectedMonth((m) => m + 1)
    }
  }

  useEffect(() => {
    let cancelled = false
    const session = getSessionPair()
    if (!session?.accessToken || !user?.rt?.id) {
      setLoading(false)
      return
    }

    setLoading(true)
    apiGetDashboardSummary(session.accessToken, activePeriodStr)
      .then((res) => {
        if (!cancelled) {
          setData(res)
          setLoading(false)
        }
      })
      .catch((err) => {
        console.error('Failed to load dashboard summary:', err)
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [user?.rt?.id, activePeriodStr])

  if (!user) return null

  const label = roleLabel(user.systemRole, user.role)
  const roleColors: Record<string, string> = {
    'Super Admin': 'var(--sf-danger)',
    'Pengurus': 'var(--sf-accent)',
    'Bendahara': 'var(--sf-info)',
    'Warga': 'var(--sf-success)',
  }

  const balanceNum = data ? parseFloat(data.balance) || 0 : 0
  const monthlyIncomeNum = data ? parseFloat(data.monthly_income) || 0 : 0
  const monthlyExpenseNum = data ? parseFloat(data.monthly_expense) || 0 : 0

  const chartData = (data?.chart ?? []).map((c) => ({
    month: c.month,
    income: c.income,
    expense: c.expense,
  }))

  const duesPaid = data?.dues_progress?.paid_households ?? 0
  const duesTotal = data?.dues_progress?.total_households ?? 0
  const duesPct = data?.dues_progress?.percentage ?? 0

  const recentTx = data?.recent_transactions ?? []
  const monthLabel = MONTH_NAMES[selectedMonth - 1]
  const periodDisplayLabel = isAllPeriods ? 'Semua Periode' : `${monthLabel} ${selectedYear}`
  const metricScopeLabel = isAllPeriods ? 'Semua' : monthLabel

  // Payment method breakdown (Donut chart)
  const breakdown = data?.payment_method_breakdown
  const transferAmount = breakdown ? parseFloat(breakdown.transfer_amount) || 0 : 0
  const cashAmount = breakdown ? parseFloat(breakdown.cash_amount) || 0 : 0
  const transferCount = breakdown?.transfer_count ?? 0
  const cashCount = breakdown?.cash_count ?? 0
  const totalMethodAmount = transferAmount + cashAmount
  const transferPct = totalMethodAmount > 0 ? Math.round((transferAmount / totalMethodAmount) * 100) : 0
  const cashPct = totalMethodAmount > 0 ? 100 - transferPct : 0

  const donutData = [
    { name: 'Transfer', value: transferAmount, count: transferCount, color: '#a970ff' },
    { name: 'Tunai', value: cashAmount, count: cashCount, color: '#5794f2' },
  ]

  return (
    <>
      {/* PageHeader surface */}
      <PageHeader
        surface
        icon={<DashboardOutlined />}
        title={`Selamat datang, ${user.name}`}
        subtitle={user.rt?.name ?? ''}
        actions={
          <span className="sf-badge" style={{ color: roleColors[label] || 'var(--sf-text)', background: 'var(--sf-surface-hover)' }}>
            {label}
          </span>
        }
      />

      {/* Period Filter Toolbar */}
      <div className="sf-list-toolbar" style={{ marginTop: 0, marginBottom: 0 }}>
        <div className="sf-list-toolbar-left" style={{ flexWrap: 'wrap', gap: 8 }}>
          {/* Period navigation */}
          <button
            type="button"
            className="sf-btn sf-btn-outline sf-kas-period-button"
            onClick={handlePrevMonth}
            disabled={isAllPeriods}
            style={{
              padding: '6px 10px',
              fontSize: 13,
              background: 'rgba(169, 112, 255, 0.06)',
              border: '1px solid rgba(169, 112, 255, 0.35)',
              color: 'var(--sf-accent)',
              borderRadius: 'var(--sf-radius-sm)',
              transition: 'all 0.25s ease',
              opacity: isAllPeriods ? 0.5 : 1,
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
            }}
            title="Bulan Sebelumnya"
            aria-label="Bulan Sebelumnya"
          >
            <LeftOutlined style={{ fontSize: 11 }} />
          </button>

          <select
            value={isAllPeriods ? '' : selectedMonth}
            disabled={isAllPeriods}
            onChange={(e) => {
              setIsAllPeriods(false)
              setSelectedMonth(parseInt(e.target.value, 10))
            }}
            style={{
              padding: '6px 12px',
              fontSize: 13,
              fontWeight: 600,
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid var(--sf-border)',
              background: 'var(--sf-bg)',
              color: 'var(--sf-text)',
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
            }}
            aria-label="Pilih Bulan"
          >
            {MONTH_NAMES.map((m, idx) => (
              <option key={m} value={idx + 1}>
                {m}
              </option>
            ))}
          </select>

          <select
            value={isAllPeriods ? '' : selectedYear}
            disabled={isAllPeriods}
            onChange={(e) => {
              setIsAllPeriods(false)
              setSelectedYear(parseInt(e.target.value, 10))
            }}
            style={{
              padding: '6px 12px',
              fontSize: 13,
              fontWeight: 600,
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid var(--sf-border)',
              background: 'var(--sf-bg)',
              color: 'var(--sf-text)',
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
            }}
            aria-label="Pilih Tahun"
          >
            {[2024, 2025, 2026, 2027].map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </select>

          <button
            type="button"
            className="sf-btn sf-btn-outline sf-kas-period-button"
            onClick={handleNextMonth}
            disabled={isAllPeriods}
            style={{
              padding: '6px 10px',
              fontSize: 13,
              background: 'rgba(169, 112, 255, 0.06)',
              border: '1px solid rgba(169, 112, 255, 0.35)',
              color: 'var(--sf-accent)',
              borderRadius: 'var(--sf-radius-sm)',
              transition: 'all 0.25s ease',
              opacity: isAllPeriods ? 0.5 : 1,
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
            }}
            title="Bulan Berikutnya"
            aria-label="Bulan Berikutnya"
          >
            <RightOutlined style={{ fontSize: 11 }} />
          </button>
        </div>

        {/* Right: Semua Periode button + Active Period Badge */}
        <div className="sf-list-toolbar-right" style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <button
            type="button"
            className={`sf-btn sf-kas-period-button ${isAllPeriods ? 'sf-btn-primary' : 'sf-btn-outline'}`}
            onClick={() => setIsAllPeriods(!isAllPeriods)}
            style={{
              fontSize: 12,
              padding: '6px 12px',
              background: isAllPeriods
                ? 'rgba(169, 112, 255, 0.14)'
                : 'rgba(169, 112, 255, 0.06)',
              border: isAllPeriods
                ? '1px solid rgba(169, 112, 255, 0.55)'
                : '1px solid rgba(169, 112, 255, 0.35)',
              color: isAllPeriods ? 'var(--sf-accent)' : 'var(--sf-accent)',
              borderRadius: 'var(--sf-radius-sm)',
              transition: 'all 0.25s ease',
              boxShadow: isAllPeriods
                ? '0 0 12px rgba(169, 112, 255, 0.18)'
                : '0 0 6px rgba(169, 112, 255, 0.06)',
            }}
          >
            Semua Periode
          </button>

          <div
            style={{
              padding: '5px 12px',
              borderRadius: 'var(--sf-radius-sm)',
              background: 'rgba(59, 130, 246, 0.1)',
              border: '1px solid rgba(59, 130, 246, 0.25)',
              fontSize: 12,
              fontWeight: 600,
              color: 'var(--sf-accent)',
              whiteSpace: 'nowrap',
            }}
          >
            {periodDisplayLabel}
          </div>
        </div>
      </div>

      {/* Metrics row */}
      <div className="sf-metrics-row">
        <div className="sf-metric-card">
          <div className="sf-metric-label">Saldo Kas</div>
          <div className="sf-metric-value sf-metric-default">
            {loading && !data ? '...' : formatRupiah(balanceNum)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Pemasukan ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-income">
            {loading && !data ? '...' : formatRupiah(monthlyIncomeNum)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Pengeluaran ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-expense">
            {loading && !data ? '...' : formatRupiah(monthlyExpenseNum)}
          </div>
        </div>
      </div>

      {/* Financial chart */}
      <div className="sf-section-card">
        <div className="sf-section-header">
          <div>
            <div className="sf-section-title">Ringkasan Keuangan</div>
            <div className="sf-section-subtitle">Tahun {selectedYear} (12 Bulan)</div>
          </div>
        </div>
        <div style={{ height: 240 }}>
          <FinancialChart data={chartData} />
        </div>
      </div>

      {/* Middle row: Iuran progress + Donut Panel Transfer & Tunai */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
          gap: 'var(--sf-sp-md)',
          marginBottom: '1px',
        }}
      >
        {/* Iuran progress */}
        <div className="sf-section-card" style={{ marginBottom: 0 }}>
          <div className="sf-section-title">Iuran ({metricScopeLabel})</div>
          <div style={{ marginTop: 20 }}>
            <div style={{
              height: 6,
              background: 'var(--sf-border)',
              borderRadius: 999,
              overflow: 'hidden',
              marginBottom: 8,
            }}>
              <div style={{
                width: `${duesPct}%`,
                height: '100%',
                background: 'var(--sf-accent)',
                borderRadius: 999,
                transition: 'width 0.3s ease',
              }} />
            </div>
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'baseline',
            }}>
              <span style={{
                fontSize: 22,
                fontWeight: 700,
              color: '#3b82f6',
                lineHeight: 1.2,
              }}>
                {duesPaid}/{duesTotal}
              </span>
              <span style={{
                fontSize: 12,
                color: 'var(--sf-text-muted)',
                fontWeight: 500,
              }}>
                {duesPct}% terkumpul
              </span>
            </div>
          </div>
        </div>

        {/* Donut Panel: Transfer vs Tunai */}
        <div className="sf-section-card" style={{ marginBottom: 0 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <div className="sf-section-title">Metode Transaksi ({metricScopeLabel})</div>
            <span style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Transfer vs Tunai</span>
          </div>

          {totalMethodAmount === 0 ? (
            <div style={{ padding: '24px 0', textAlign: 'center', fontSize: 13, color: 'var(--sf-text-muted)' }}>
              Belum ada transaksi pada periode ini
            </div>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
              <div style={{ width: 120, height: 120, flexShrink: 0 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={donutData}
                      dataKey="value"
                      nameKey="name"
                      cx="50%"
                      cy="50%"
                      innerRadius={34}
                      outerRadius={52}
                      paddingAngle={2}
                      stroke="none"
                    >
                      {donutData.map((entry) => (
                        <Cell key={entry.name} fill={entry.color} />
                      ))}
                    </Pie>
                    <RechartsTooltip
                      formatter={(val: number, name: string) => [`Rp ${Number(val).toLocaleString('id-ID')}`, name]}
                      contentStyle={{
                        background: 'var(--sf-surface)',
                        border: '1px solid var(--sf-border)',
                        borderRadius: 'var(--sf-radius-sm)',
                        fontSize: '12px',
                      }}
                    />
                  </PieChart>
                </ResponsiveContainer>
              </div>

              <div style={{ flex: 1, minWidth: 180, display: 'flex', flexDirection: 'column', gap: 10 }}>
                {/* Transfer */}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ width: 10, height: 10, borderRadius: '50%', background: '#a970ff', display: 'inline-block' }} />
                    <div>
                      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--sf-text)' }}>
                        Transfer ({transferPct}%)
                      </div>
                      <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>
                        {transferCount} transaksi
                      </div>
                    </div>
                  </div>
                  <div style={{ fontSize: 13, fontWeight: 700, fontFamily: 'monospace', color: '#a970ff' }}>
                    {formatRupiah(transferAmount)}
                  </div>
                </div>

                {/* Tunai */}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ width: 10, height: 10, borderRadius: '50%', background: '#5794f2', display: 'inline-block' }} />
                    <div>
                      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--sf-text)' }}>
                        Tunai ({cashPct}%)
                      </div>
                      <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>
                        {cashCount} transaksi
                      </div>
                    </div>
                  </div>
                  <div style={{ fontSize: 13, fontWeight: 700, fontFamily: 'monospace', color: '#5794f2' }}>
                    {formatRupiah(cashAmount)}
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Recent transactions */}
      <div className="sf-section-card">
        <div className="sf-section-title">Transaksi Terbaru</div>
        <div style={{ marginTop: 8, overflowX: 'auto' }}>
          <table className="sf-table" style={{ width: '100%', minWidth: 620 }}>
            <thead>
              <tr>
                <th style={{ textAlign: 'left' }}>Keterangan</th>
                <th style={{ textAlign: 'left', width: 130 }}>Kategori</th>
                <th style={{ textAlign: 'center', width: 90 }}>Metode</th>
                <th style={{ textAlign: 'left', width: 110 }}>Tanggal</th>
                <th style={{ textAlign: 'left', width: 95 }}>Arus</th>
                <th style={{ textAlign: 'right', width: 130 }}>Nominal</th>
              </tr>
            </thead>
            <tbody>
              {recentTx.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', color: 'var(--sf-text-muted)', padding: '16px' }}>
                    {loading ? 'Memuat transaksi...' : 'Belum ada transaksi'}
                  </td>
                </tr>
              ) : (
                recentTx.map((t) => {
                  const isIncome = t.type === 'income'
                  const metode = t.payment_method === 'TRANSFER' ? 'Transfer' : t.payment_method === 'CASH' ? 'Tunai' : null
                  const refNo = t.payment_no || t.reference_no || (t.payment_id ? `PAY-${t.payment_id.slice(0, 8)}` : '')
                  return (
                    <tr key={t.id}>
                      <td>
                        {t.due_name ? (
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                            <Badge variant="blue">{t.due_name}</Badge>
                            {refNo && (
                              <span style={{ fontWeight: 500, fontFamily: 'monospace', color: 'var(--sf-text-muted)', fontSize: 12 }}>
                                {refNo}
                              </span>
                            )}
                          </div>
                        ) : (
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                            <span style={{ maxWidth: 260, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'inline-block', fontSize: 13 }}>
                              {cleanKasDescription(t.description)}
                            </span>
                            {refNo && (
                              <span style={{ fontFamily: 'monospace', color: 'var(--sf-text-muted)', fontSize: 11 }}>
                                ({refNo})
                              </span>
                            )}
                          </div>
                        )}
                      </td>
                      <td style={{ whiteSpace: 'nowrap', fontSize: 13, color: 'var(--sf-text)' }}>
                        {t.category_name || (isIncome ? 'Pemasukan' : 'Pengeluaran')}
                      </td>
                      <td style={{ textAlign: 'center', whiteSpace: 'nowrap' }}>
                        {metode ? (
                          <Badge variant={metode === 'Transfer' ? 'violet' : 'blue'}>
                            {metode}
                          </Badge>
                        ) : (
                          <span style={{ color: 'var(--sf-text-muted)' }}>—</span>
                        )}
                      </td>
                      <td style={{ whiteSpace: 'nowrap', fontSize: 12, color: 'var(--sf-text-muted)' }}>
                        {formatDate(t.occurred_at || t.created_at)}
                      </td>
                      <td style={{ whiteSpace: 'nowrap' }}>
                        <Badge
                          variant={isIncome ? 'green' : 'red'}
                          style={{ height: 20, padding: '0 6px', fontSize: 10, fontWeight: 700, display: 'inline-flex', alignItems: 'center' }}
                        >
                          {isIncome ? '↓ Masuk' : '↑ Keluar'}
                        </Badge>
                      </td>
                      <td style={{
                        textAlign: 'right',
                        fontFamily: 'monospace',
                        fontSize: 13,
                        fontWeight: 600,
                        color: isIncome ? 'var(--sf-success)' : 'var(--sf-danger)',
                      }}>
                        {isIncome ? '+' : '\u2212'} Rp {formatKasRupiah(t.amount)}
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  )
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}


