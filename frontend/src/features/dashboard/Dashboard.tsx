import { useState, useEffect } from 'react'
import { useAuth } from '@/app/AuthContext'
import { formatRupiah } from '@/styles/format'
import { FinancialChart } from '@/components/FinancialChart'
import { PageHeader } from '@/components/PageHeader'
import { DashboardOutlined } from '@ant-design/icons'
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
  const [data, setData] = useState<ApiDashboardSummary | null>(null)
  const [loading, setLoading] = useState(true)

  const activePeriodStr = `${selectedYear}-${String(selectedMonth).padStart(2, '0')}`

  const handlePrevMonth = () => {
    if (selectedMonth === 1) {
      setSelectedMonth(12)
      setSelectedYear((y) => y - 1)
    } else {
      setSelectedMonth((m) => m - 1)
    }
  }

  const handleNextMonth = () => {
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

      {/* Month & Year Navigation Toolbar for periodic panels */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          background: 'var(--sf-surface)',
          border: '1px solid var(--sf-border)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '8px 12px',
          marginTop: 12,
          marginBottom: 12,
          gap: 8,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <button
            type="button"
            className="sf-btn"
            onClick={handlePrevMonth}
            style={{
              height: 32,
              padding: '0 10px',
              fontSize: 13,
              fontWeight: 600,
              background: 'transparent',
              border: '1px solid var(--sf-border)',
              color: 'var(--sf-text)',
            }}
            title="Bulan Sebelumnya"
          >
            ←
          </button>
          <select
            className="sf-form-select"
            value={selectedMonth}
            onChange={(e) => setSelectedMonth(parseInt(e.target.value, 10))}
            style={{ height: 32, padding: '0 10px', fontSize: 13, fontWeight: 600 }}
          >
            {MONTH_NAMES.map((m, idx) => (
              <option key={m} value={idx + 1}>
                {m}
              </option>
            ))}
          </select>
          <select
            className="sf-form-select"
            value={selectedYear}
            onChange={(e) => setSelectedYear(parseInt(e.target.value, 10))}
            style={{ height: 32, padding: '0 10px', fontSize: 13, fontWeight: 600 }}
          >
            {[2024, 2025, 2026, 2027].map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </select>
          <button
            type="button"
            className="sf-btn"
            onClick={handleNextMonth}
            style={{
              height: 32,
              padding: '0 10px',
              fontSize: 13,
              fontWeight: 600,
              background: 'transparent',
              border: '1px solid var(--sf-border)',
              color: 'var(--sf-text)',
            }}
            title="Bulan Berikutnya"
          >
            →
          </button>
        </div>
        <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', fontWeight: 500 }}>
          Periode: <strong style={{ color: 'var(--sf-accent)' }}>{monthLabel} {selectedYear}</strong>
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
          <div className="sf-metric-label">Pemasukan ({monthLabel})</div>
          <div className="sf-metric-value sf-metric-income">
            {loading && !data ? '...' : formatRupiah(monthlyIncomeNum)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Pengeluaran ({monthLabel})</div>
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

      {/* Bottom row */}
      <div className="sf-data-grid sf-data-grid-compact">
        {/* Iuran progress */}
        <div className="sf-section-card">
          <div className="sf-section-title">Iuran ({monthLabel})</div>
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
                color: 'var(--sf-accent)',
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

        {/* Recent transactions */}
        <div className="sf-section-card">
          <div className="sf-section-title">Transaksi Terbaru</div>
          <div style={{ marginTop: 4 }}>
            <table className="sf-table">
              <thead>
                <tr>
                  <th>Keterangan</th>
                  <th>Tanggal</th>
                  <th>Jumlah</th>
                </tr>
              </thead>
              <tbody>
                {recentTx.length === 0 ? (
                  <tr>
                    <td colSpan={3} style={{ textAlign: 'center', color: 'var(--sf-text-muted)', padding: '16px' }}>
                      {loading ? 'Memuat transaksi...' : 'Belum ada transaksi'}
                    </td>
                  </tr>
                ) : (
                  recentTx.map((t) => {
                    const desc = t.due_name && t.citizen_name
                      ? `[${t.due_name}] ${t.citizen_name}${t.house_number ? ' · ' + t.house_number : ''}`
                      : t.description
                    const amountVal = parseFloat(t.amount) || 0
                    return (
                      <tr key={t.id}>
                        <td>{desc}</td>
                        <td>{formatDate(t.occurred_at || t.created_at)}</td>
                        <td style={{
                          color: t.type === 'income' ? 'var(--sf-success)' : 'var(--sf-danger)',
                        }}>
                          {t.type === 'income' ? '+' : '\u2212'} {formatRupiah(amountVal)}
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </>
  )
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}

