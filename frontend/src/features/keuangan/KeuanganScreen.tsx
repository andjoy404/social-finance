import { useState, useEffect } from 'react'
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip as RechartsTooltip } from 'recharts'
import { useAuth } from '@/app/AuthContext'
import { formatRupiah } from '@/styles/format'
import { FinancialChart } from '@/components/FinancialChart'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { cleanKasDescription } from '@/features/kas/KasList'
import { formatRupiah as formatKasRupiah } from '@/features/kas/kasTypes'
import { DollarOutlined, LeftOutlined, RightOutlined, CopyOutlined, CheckOutlined, ReloadOutlined } from '@ant-design/icons'
import { apiGetDashboardSummary, ApiDashboardSummary, getSessionPair } from '@/app/api'

interface MethodTooltipProps {
  active?: boolean
  label?: string
  payload?: Array<{ name?: string; value?: number; payload?: { count?: number } }>
}

/** Clone of FinancialChart's custom tooltip (style + coloring), applied to the Metode Pembayaran donut. */
function CustomMethodTooltip({ active, payload }: MethodTooltipProps) {
  if (!active || !payload || payload.length === 0) return null

  const mk = payload.find((p) => p.name === 'Transfer')
  const mc = payload.find((p) => p.name === 'Tunai')
  const transferVal = Number(mk?.value ?? 0)
  const cashVal = Number(mc?.value ?? 0)
  const total = transferVal + cashVal
  const transferPct = total > 0 ? Math.round((transferVal / total) * 100) : 0
  const cashPct = total > 0 ? 100 - transferPct : 0
  const transferCount = Number((mk?.payload as { count?: number } | undefined)?.count ?? 0)
  const cashCount = Number((mc?.payload as { count?: number } | undefined)?.count ?? 0)

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
        Metode Transaksi
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, color: '#a970ff', fontWeight: 600 }}>
        <span>Transfer</span>
        <span style={{ fontFamily: 'monospace' }}>Rp.{transferVal.toLocaleString('id-ID')}</span>
      </div>
      <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
        {transferPct}% · {transferCount} transaksi
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, color: '#5794f2', fontWeight: 600, marginTop: 8 }}>
        <span>Tunai</span>
        <span style={{ fontFamily: 'monospace' }}>Rp.{cashVal.toLocaleString('id-ID')}</span>
      </div>
      <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
        {cashPct}% · {cashCount} transaksi
      </div>
    </div>
  )
}

const MONTH_NAMES = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

function roleLabel(sys?: string | null, tenant?: string): string {
  if (sys === 'super_admin') return 'Super Admin'
  const labels: Record<string, string> = { pengurus: 'Pengurus', bendahara: 'Bendahara', warga: 'Warga' }
  return labels[tenant ?? ''] ?? tenant ?? ''
}

export function KeuanganScreen() {
  const { user } = useAuth()
  const now = new Date()
  const [selectedMonth, setSelectedMonth] = useState<number>(now.getMonth() + 1)
  const [selectedYear, setSelectedYear] = useState<number>(now.getFullYear())
  const [isAllPeriods, setIsAllPeriods] = useState<boolean>(false)
  const [data, setData] = useState<ApiDashboardSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [copiedRef, setCopiedRef] = useState<string | null>(null)

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
    setError(null)
    const doLoad = () =>
      apiGetDashboardSummary(session.accessToken, activePeriodStr)
        .then((res) => {
          if (!cancelled) {
            setData(res)
            setLoading(false)
          }
        })
        .catch((err) => {
          if (!cancelled) {
            setError(err?.message || 'Gagal memuat ringkasan keuangan. Periksa koneksi lalu coba lagi.')
            // Keep data = null so stale/zero placeholders are never shown as if real.
            setData(null)
            setLoading(false)
          }
        })
    void doLoad()

    return () => {
      cancelled = true
    }
  }, [user?.rt?.id, activePeriodStr])

  if (!user) return null

  if (error && !data && !loading) {
    return (
      <>
        <PageHeader
          surface
          icon={<DollarOutlined />}
          title="Keuangan RT"
          subtitle={user.rt?.name ?? ''}
        />
        <div className="sf-card" style={{ padding: 48, textAlign: 'center', marginTop: 16 }}>
          <DollarOutlined style={{ fontSize: 32, marginBottom: 12, color: 'var(--sf-text-muted)' }} />
          <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-text)' }}>Gagal memuat ringkasan keuangan</div>
          <div style={{ fontSize: 13, marginTop: 4, color: 'var(--sf-text-muted)' }}>{error}</div>
          <button
            type="button"
            className="sf-btn sf-btn-outline"
            style={{ marginTop: 16 }}
            onClick={() => {
              setLoading(true)
              setError(null)
              apiGetDashboardSummary(getSessionPair()!.accessToken, activePeriodStr)
                .then((res) => { setData(res); setLoading(false) })
                .catch((err) => { setError(err?.message || 'Gagal memuat ringkasan keuangan.'); setData(null); setLoading(false) })
            }}
          >
            <ReloadOutlined /> Coba Lagi
          </button>
        </div>
      </>
    )
  }

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
        icon={<DollarOutlined />}
        title="Keuangan RT"
        subtitle={user.rt?.name ?? ''}
        actions={
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <Badge variant="violet" style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
              Periode {periodDisplayLabel}{isAllPeriods ? '' : ' (Aktif)'}
            </Badge>
            <span className="sf-badge" style={{ color: roleColors[label] || 'var(--sf-text)', background: 'var(--sf-surface-hover)' }}>
              {label}
            </span>
          </div>
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
              padding: '8px 20px',
              fontSize: 13,
              fontWeight: 600,
              lineHeight: '1.2',
              background: 'rgba(169, 112, 255, 0.06)',
              border: '1px solid rgba(169, 112, 255, 0.35)',
              color: 'var(--sf-accent)',
              borderRadius: 'var(--sf-radius-sm)',
              transition: 'all 0.25s ease',
              boxSizing: 'border-box',
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
              padding: '8px 20px',
              fontSize: 13,
              fontWeight: 600,
              lineHeight: '1.2',
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid var(--sf-border)',
              background: 'var(--sf-bg)',
              color: 'var(--sf-text)',
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
              boxSizing: 'border-box',
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
              padding: '8px 20px',
              fontSize: 13,
              fontWeight: 600,
              lineHeight: '1.2',
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid var(--sf-border)',
              background: 'var(--sf-bg)',
              color: 'var(--sf-text)',
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
              boxSizing: 'border-box',
            }}
            aria-label="Pilih Tahun"
          >
            {[now.getFullYear() - 2, now.getFullYear() - 1, now.getFullYear(), now.getFullYear() + 1].map((y) => (
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
              padding: '8px 20px',
              fontSize: 13,
              fontWeight: 600,
              lineHeight: '1.2',
              background: 'rgba(169, 112, 255, 0.06)',
              border: '1px solid rgba(169, 112, 255, 0.35)',
              color: 'var(--sf-accent)',
              borderRadius: 'var(--sf-radius-sm)',
              transition: 'all 0.25s ease',
              boxSizing: 'border-box',
              opacity: isAllPeriods ? 0.5 : 1,
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
            }}
            title="Bulan Berikutnya"
            aria-label="Bulan Berikutnya"
          >
            <RightOutlined style={{ fontSize: 11 }} />
          </button>

          <button
            type="button"
            className={`sf-btn sf-kas-period-button ${isAllPeriods ? 'sf-btn-primary' : 'sf-btn-outline'}`}
            onClick={() => setIsAllPeriods((prev) => !prev)}
            style={{
              padding: '8px 20px',
              fontSize: 13,
              fontWeight: 600,
              lineHeight: '1.2',
              borderRadius: 'var(--sf-radius-sm)',
              boxSizing: 'border-box',
              ...(isAllPeriods
                ? {
                    background: 'var(--sf-accent)',
                    borderColor: 'var(--sf-accent)',
                    color: '#fff',
                  }
                : {
                    background: 'rgba(169, 112, 255, 0.06)',
                    borderColor: 'rgba(169, 112, 255, 0.35)',
                    color: 'var(--sf-accent)',
                  }),
            }}
          >
            Semua Periode
          </button>
        </div>
      </div>

      {/* Main Content Area */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 20, marginTop: 16 }}>
        {/* Metric Cards Grid */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
            gap: 16,
          }}
        >
          {/* Card: Total Saldo Kas (Cumulative) */}
          <div className="sf-card" style={{ padding: 20 }}>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginBottom: 8, fontWeight: 500 }}>
              Total Saldo Kas (Kumulatif)
            </div>
            <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--sf-text)' }}>
              {loading ? '...' : formatRupiah(balanceNum)}
            </div>
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 8 }}>
              Saldo riil per saat ini
            </div>
          </div>

          {/* Card: Total Pemasukan */}
          <div className="sf-card" style={{ padding: 20 }}>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginBottom: 8, fontWeight: 500 }}>
              Pemasukan ({metricScopeLabel})
            </div>
            <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--sf-success)' }}>
              {loading ? '...' : formatRupiah(monthlyIncomeNum)}
            </div>
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 8 }}>
              {periodDisplayLabel}
            </div>
          </div>

          {/* Card: Total Pengeluaran */}
          <div className="sf-card" style={{ padding: 20 }}>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginBottom: 8, fontWeight: 500 }}>
              Pengeluaran ({metricScopeLabel})
            </div>
            <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--sf-danger)' }}>
              {loading ? '...' : formatRupiah(monthlyExpenseNum)}
            </div>
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 8 }}>
              {periodDisplayLabel}
            </div>
          </div>

          {/* Card: Progres Pembayaran Iuran */}
          <div className="sf-card" style={{ padding: 20 }}>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginBottom: 8, fontWeight: 500 }}>
              Progres Iuran ({metricScopeLabel})
            </div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
              <span style={{ fontSize: 24, fontWeight: 700, color: 'var(--sf-accent)' }}>
                {loading ? '...' : `${duesPct}%`}
              </span>
              <span style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
                ({duesPaid}/{duesTotal} Rumah)
              </span>
            </div>
            <div
              style={{
                width: '100%',
                height: 6,
                background: 'var(--sf-border)',
                borderRadius: 3,
                marginTop: 12,
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  width: `${Math.min(duesPct, 100)}%`,
                  height: '100%',
                  background: 'var(--sf-accent)',
                  borderRadius: 3,
                  transition: 'width 0.3s ease',
                }}
              />
            </div>
          </div>
        </div>

        {/* Charts & Breakdown Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 20 }}>
          {/* Financial Trend Chart */}
          <div className="sf-card" style={{ padding: 20 }}>
            <h3 style={{ margin: '0 0 16px 0', fontSize: 16, fontWeight: 600 }}>Tren Keuangan (12 Bulan Terakhir)</h3>
            <div style={{ height: 280 }}>
              {chartData.length > 0 ? (
                <FinancialChart data={chartData} />
              ) : (
                <div style={{ display: 'flex', height: '100%', alignItems: 'center', justifyContent: 'center', color: 'var(--sf-text-muted)' }}>
                  {loading ? 'Memuat grafik...' : 'Belum ada data grafik'}
                </div>
              )}
            </div>
          </div>

          {/* Payment Method Breakdown Donut Chart */}
          <div className="sf-card" style={{ padding: 20, display: 'flex', flexDirection: 'column' }}>
            <div style={{ margin: '0 0 16px 0', fontSize: 16, fontWeight: 600 }}>Metode Transaksi ({metricScopeLabel})</div>
            {totalMethodAmount > 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column', height: '100%', justifyContent: 'space-between' }}>
                <div style={{ height: 180 }}>
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={donutData}
                        cx="50%"
                        cy="50%"
                        innerRadius={50}
                        outerRadius={75}
                        paddingAngle={4}
                        dataKey="value"
                      >
                        {donutData.map((entry, index) => (
                          <Cell key={`cell-${index}`} fill={entry.color} />
                        ))}
                      </Pie>
                      <RechartsTooltip
                        content={<CustomMethodTooltip />}
                        cursor={{ fill: 'var(--sf-accent)', fillOpacity: 0.10 }}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-around', marginTop: 12, paddingTop: 12, borderTop: '1px solid var(--sf-border)' }}>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, justifyContent: 'center', marginBottom: 4 }}>
                      <span style={{ width: 10, height: 10, borderRadius: '50%', background: '#a970ff' }} />
                      <span style={{ fontSize: 13, fontWeight: 500 }}>Transfer ({transferPct}%)</span>
                    </div>
                    <div style={{ fontSize: 14, fontWeight: 700 }}>{formatRupiah(transferAmount)}</div>
                    <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>{transferCount} transaksi</div>
                  </div>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, justifyContent: 'center', marginBottom: 4 }}>
                      <span style={{ width: 10, height: 10, borderRadius: '50%', background: '#5794f2' }} />
                      <span style={{ fontSize: 13, fontWeight: 500 }}>Tunai ({cashPct}%)</span>
                    </div>
                    <div style={{ fontSize: 14, fontWeight: 700 }}>{formatRupiah(cashAmount)}</div>
                    <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>{cashCount} transaksi</div>
                  </div>
                </div>
              </div>
            ) : (
              <div style={{ display: 'flex', flex: 1, alignItems: 'center', justifyContent: 'center', color: 'var(--sf-text-muted)', minHeight: 200 }}>
                {loading ? 'Memuat data...' : 'Belum ada transaksi pembayaran'}
              </div>
            )}
          </div>
        </div>

        {/* Recent Transactions List */}
        <div className="sf-card" style={{ padding: 20 }}>
          <h3 style={{ margin: '0 0 16px 0', fontSize: 16, fontWeight: 600 }}>5 Transaksi Terbaru ({metricScopeLabel})</h3>
          {recentTx.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {recentTx.map((tx) => {
                const isIncome = tx.type === 'income'
                const amountNum = parseFloat(tx.amount) || 0
                const methodRaw = (tx.payment_method || '').toUpperCase()
                const methodLabel = methodRaw === 'CASH' ? 'Tunai' : methodRaw === 'TRANSFER' ? 'Transfer' : (methodRaw || '')
                const refNo = tx.payment_no || tx.reference_no || null
                const copied = copiedRef === tx.id
                return (
                  <div
                    key={tx.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      gap: 12,
                      padding: '12px 16px',
                      background: 'var(--sf-bg)',
                      borderRadius: 8,
                      border: '1px solid var(--sf-border)',
                      flexWrap: 'wrap',
                    }}
                  >
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--sf-text)' }}>
                        {cleanKasDescription(tx.description || 'Transaksi')}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4, display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
                        <span>
                          {(tx.occurred_at || tx.created_at) ? new Date(tx.occurred_at || tx.created_at).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '-'} • {tx.due_name || 'Umum'}
                        </span>
                        {tx.category_name && (
                          <Badge variant="blue" style={{ display: 'inline-flex', alignItems: 'center' }}>
                            {tx.category_name}
                          </Badge>
                        )}
                        {methodLabel && (
                          <Badge variant="gray" style={{ display: 'inline-flex', alignItems: 'center' }}>
                            {methodLabel}
                          </Badge>
                        )}
                        {refNo && (
                          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                            <span style={{ fontFamily: 'monospace' }}>{refNo}</span>
                            <button
                              type="button"
                              aria-label="Salin nomor referensi"
                              title="Salin nomor referensi"
                              onClick={() => {
                                const done = () => {
                                  setCopiedRef(tx.id)
                                  window.setTimeout(() => setCopiedRef((prev) => (prev === tx.id ? null : prev)), 1500)
                                }
                                if (navigator.clipboard?.writeText) {
                                  navigator.clipboard.writeText(refNo).then(done).catch(done)
                                } else {
                                  done()
                                }
                              }}
                              style={{
                                border: '1px solid var(--sf-border)',
                                background: 'var(--sf-surface-hover)',
                                color: 'var(--sf-text-muted)',
                                borderRadius: 4,
                                cursor: 'pointer',
                                padding: '2px 6px',
                                fontSize: 11,
                                display: 'inline-flex',
                                alignItems: 'center',
                                gap: 4,
                              }}
                            >
                              {copied ? <CheckOutlined style={{ fontSize: 11 }} /> : <CopyOutlined style={{ fontSize: 11 }} />}
                              {copied ? 'Tersalin' : 'Salin'}
                            </button>
                          </span>
                        )}
                      </div>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div
                        style={{
                          fontWeight: 700,
                          fontSize: 15,
                          color: isIncome ? 'var(--sf-success)' : 'var(--sf-danger)',
                        }}
                      >
                        {isIncome ? '+ ' : '- '}
                        {formatKasRupiah(amountNum)}
                      </div>
                      <Badge variant={isIncome ? 'green' : 'red'} style={{ marginTop: 4, display: 'inline-block' }}>
                        {isIncome ? 'Pemasukan' : 'Pengeluaran'}
                      </Badge>
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)' }}>
              {loading ? 'Memuat transaksi...' : 'Belum ada transaksi pada periode ini'}
            </div>
          )}
        </div>
      </div>
    </>
  )
}
