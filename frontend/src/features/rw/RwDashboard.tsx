/**
 * RW-aware Dashboard (BERANDA RW)
 *
 * Mengagregasi data keuangan mandiri tingkat RW:
 *   - Ringkasan Saldo, Pemasukan, dan Pengeluaran berdasarkan periode filter
 *   - Ringkasan RW (Total RT, Transaksi, Lokasi Dana)
 *   - Ringkasan Keuangan 12 Bulan (Grafik Bulanan Pemasukan vs Pengeluaran KAS RW)
 *   - Metode Transaksi (Distribusi Transfer vs Tunai dari transaksi riil KAS RW)
 *   - Transaksi Terbaru (Daftar transaksi KAS RW terbaru dengan navigasi ke menu KAS)
 */

import { useState, useEffect, useMemo, useCallback } from 'react'
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip as RechartsTooltip } from 'recharts'
import { useAuth } from '@/app/AuthContext'
import { getSessionPair } from '@/app/api'
import { formatRupiah } from '@/styles/format'
import { PageHeader } from '@/components/PageHeader'
import { FinancialChart } from '@/components/FinancialChart'
import {
  DashboardOutlined,
  LeftOutlined,
  RightOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from '@ant-design/icons'
import { resolveRwLabel } from '@/utils/rwUtils'
import {
  apiGetRWDashboard,
  apiGetRWKAS,
  apiListRWTransactions,
  apiListRWLocations,
  type ApiRWDashboardSummary,
  type ApiRWKASReport,
  type ApiRWTransaction,
  type ApiRWLocation,
} from './rw-api'

const MONTH_NAMES = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

const MONTH_SHORT = [
  'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun',
  'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des',
]

function formatRupiahShort(val: number): string {
  return formatRupiah(val)
}

function formatTanggal(dateStr: string): string {
  if (!dateStr) return '—'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}

export function RwDashboard({
  rwId,
  rwName,
  onNavigateToKas,
}: {
  rwId: string
  rwName?: string
  onNavigateToKas?: () => void
}) {
  const { user } = useAuth()
  const now = new Date()
  const [selectedMonth, setSelectedMonth] = useState<number>(now.getMonth() + 1)
  const [selectedYear, setSelectedYear] = useState<number>(now.getFullYear())
  const [isAllPeriods, setIsAllPeriods] = useState<boolean>(false)

  const [dashboardData, setDashboardData] = useState<ApiRWDashboardSummary | null>(null)
  const [kasReport, setKasReport] = useState<ApiRWKASReport | null>(null)
  const [yearTransactions, setYearTransactions] = useState<ApiRWTransaction[]>([])
  const [locations, setLocations] = useState<ApiRWLocation[]>([])
  const [loading, setLoading] = useState(true)

  const activePeriodStr = isAllPeriods
    ? undefined
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

  const loadData = useCallback(async () => {
    const accessToken = getSessionPair()?.accessToken
    if (!accessToken) {
      setLoading(false)
      return
    }

    setLoading(true)
    try {
      const [dashRes, kasRes, txRes, locRes] = await Promise.all([
        apiGetRWDashboard(accessToken, rwId).catch((err) => {
          console.error('Failed to load RW dashboard:', err)
          return null
        }),
        apiGetRWKAS(accessToken, rwId, activePeriodStr).catch((err) => {
          console.error('Failed to load RW KAS report:', err)
          return null
        }),
        apiListRWTransactions(accessToken, rwId, {
          start_date: `${selectedYear}-01-01`,
          end_date: `${selectedYear}-12-31`,
          page: 1,
          page_size: 1000,
        }).catch((err) => {
          console.error('Failed to load RW transactions:', err)
          return { data: [], pagination: { page: 1, per_page: 1000, total: 0, total_pages: 0 } }
        }),
        apiListRWLocations(accessToken, rwId, { is_active: true }).catch((err) => {
          console.error('Failed to load RW locations:', err)
          return []
        }),
      ])

      if (dashRes) setDashboardData(dashRes)
      if (kasRes) setKasReport(kasRes)
      setYearTransactions(txRes?.data || [])
      setLocations(locRes || [])
    } finally {
      setLoading(false)
    }
  }, [rwId, activePeriodStr, selectedYear])

  useEffect(() => {
    loadData()
  }, [loadData])

  if (!user) return null

  // Metric values
  // When specific period is selected, income/expense come from period KAS report
  // When all periods is selected, cumulative income/expense come from dashboard/KAS
  const balanceNum = isAllPeriods
    ? parseFloat(dashboardData?.net_balance || dashboardData?.total_balance || kasReport?.net_balance || '0') || 0
    : parseFloat(kasReport?.net_balance || '0') || 0

  const totalIncomeNum = isAllPeriods
    ? parseFloat(dashboardData?.total_income || kasReport?.total_income || '0') || 0
    : parseFloat(kasReport?.total_income || '0') || 0

  const totalExpenseNum = isAllPeriods
    ? parseFloat(dashboardData?.total_expense || kasReport?.total_expense || '0') || 0
    : parseFloat(kasReport?.total_expense || '0') || 0

  const metricScopeLabel = isAllPeriods ? 'Semua' : MONTH_NAMES[selectedMonth - 1]
  const periodDisplayLabel = isAllPeriods ? 'Semua Periode' : `${MONTH_NAMES[selectedMonth - 1]} ${selectedYear}`

  const rwMemberships = (user as any)?.rwMemberships || (user as any)?.rw_memberships || []
  const displayRwName = resolveRwLabel(rwId, rwMemberships, rwName)

  // Chart data: 12 months for selectedYear
  const chartData = useMemo(() => {
    const monthly = MONTH_SHORT.map((m) => ({ month: m, income: 0, expense: 0 }))
    yearTransactions.forEach((tx) => {
      if (tx.status !== 'posted') return
      const d = new Date(tx.occurred_at)
      if (isNaN(d.getTime())) return
      if (d.getFullYear() !== selectedYear) return
      const m = d.getMonth()
      const amt = parseFloat(tx.amount) || 0
      if (tx.type === 'income') {
        monthly[m].income += amt
      } else if (tx.type === 'expense') {
        monthly[m].expense += amt
      }
    })
    return monthly
  }, [yearTransactions, selectedYear])

  // Location map for determining transaction payment method (rekening = Transfer, brankas/coh = Tunai)
  const locMap = useMemo(() => {
    const map = new Map<string, string>()
    locations.forEach((loc) => {
      map.set(loc.id, loc.category)
    })
    return map
  }, [locations])

  // Payment method breakdown for selected period (or year)
  const { transferAmount, cashAmount, transferCount, cashCount, totalMethodAmount, transferPct, cashPct } = useMemo(() => {
    let tAmt = 0
    let cAmt = 0
    let tCnt = 0
    let cCnt = 0

    yearTransactions.forEach((tx) => {
      if (tx.status !== 'posted') return
      const d = new Date(tx.occurred_at)
      if (isNaN(d.getTime())) return

      if (!isAllPeriods) {
        if (d.getFullYear() !== selectedYear || d.getMonth() + 1 !== selectedMonth) return
      } else {
        if (d.getFullYear() !== selectedYear) return
      }

      const amt = parseFloat(tx.amount) || 0
      const methodStr = (tx.payment_method || '').toUpperCase()
      const locCat = tx.location_id ? locMap.get(tx.location_id) : undefined

      let isTransfer = false
      if (methodStr === 'TRANSFER') {
        isTransfer = true
      } else if (methodStr === 'CASH') {
        isTransfer = false
      } else if (locCat === 'rekening') {
        isTransfer = true
      } else if (locCat === 'brankas' || locCat === 'coh') {
        isTransfer = false
      } else {
        isTransfer = true
      }

      if (isTransfer) {
        tAmt += amt
        tCnt += 1
      } else {
        cAmt += amt
        cCnt += 1
      }
    })

    const total = tAmt + cAmt
    const tPct = total > 0 ? Math.round((tAmt / total) * 100) : 0
    const cPct = total > 0 ? 100 - tPct : 0

    return {
      transferAmount: tAmt,
      cashAmount: cAmt,
      transferCount: tCnt,
      cashCount: cCnt,
      totalMethodAmount: total,
      transferPct: tPct,
      cashPct: cPct,
    }
  }, [yearTransactions, isAllPeriods, selectedYear, selectedMonth, locMap])

  const donutData = [
    { name: 'Transfer', value: transferAmount, count: transferCount, color: '#a970ff' },
    { name: 'Tunai', value: cashAmount, count: cashCount, color: '#5794f2' },
  ]

  // Recent transactions: top 5 from yearTransactions
  const recentTransactions = useMemo(() => {
    return [...yearTransactions]
      .filter((t) => t.status === 'posted')
      .sort((a, b) => new Date(b.occurred_at).getTime() - new Date(a.occurred_at).getTime())
      .slice(0, 5)
  }, [yearTransactions])

  return (
    <>
      {/* PageHeader */}
      <PageHeader
        surface
        icon={<DashboardOutlined />}
        title={`Selamat datang, ${user.name}`}
        subtitle={displayRwName}
        actions={
          <span
            style={{
              padding: '4px 10px',
              fontSize: 12,
              fontWeight: 600,
              borderRadius: 'var(--sf-radius-sm)',
              color: 'var(--sf-accent)',
              background: 'rgba(169, 112, 255, 0.1)',
            }}
          >
            Keuangan RW
          </span>
        }
      />

      {/* Period Filter Toolbar */}
      <div className="sf-list-toolbar" style={{ marginTop: 0, marginBottom: 0 }}>
        <div className="sf-list-toolbar-left" style={{ flexWrap: 'wrap', gap: 8 }}>
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
              opacity: isAllPeriods ? 0.5 : 1,
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
            }}
            title="Bulan Sebelumnya"
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
          >
            {MONTH_NAMES.map((m, idx) => (
              <option key={m} value={idx + 1}>{m}</option>
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
          >
            {[2024, 2025, 2026, 2027].map((y) => (
              <option key={y} value={y}>{y}</option>
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
              opacity: isAllPeriods ? 0.5 : 1,
              cursor: isAllPeriods ? 'not-allowed' : 'pointer',
            }}
            title="Bulan Berikutnya"
          >
            <RightOutlined style={{ fontSize: 11 }} />
          </button>
        </div>

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
              color: 'var(--sf-accent)',
              borderRadius: 'var(--sf-radius-sm)',
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
          <div className="sf-metric-label">Total Saldo ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-default">
            {loading && !dashboardData && !kasReport ? '...' : formatRupiahShort(balanceNum)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Total Pemasukan ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-income">
            {loading && !dashboardData && !kasReport ? '...' : formatRupiahShort(totalIncomeNum)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Total Pengeluaran ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-expense">
            {loading && !dashboardData && !kasReport ? '...' : formatRupiahShort(totalExpenseNum)}
          </div>
        </div>
      </div>

      {/* Info card: RW scope */}
      <div className="sf-section-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
          <div>
            <div className="sf-section-title">Ringkasan RW</div>
            <div className="sf-section-subtitle">Pencatatan kas dan lokasi dana mandiri tingkat RW</div>
          </div>
          <div style={{ display: 'flex', gap: 20, alignItems: 'center' }}>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-accent)' }}>
                {dashboardData?.total_rts ?? 0}
              </div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Total RT</div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-info)', fontFamily: 'monospace' }}>
                {dashboardData?.transaction_count ?? yearTransactions.length}
              </div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Transaksi</div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-success)' }}>
                {dashboardData?.total_locations ?? locations.length}
              </div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Lokasi Dana</div>
            </div>
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

      {/* Donut Panel: Transfer vs Tunai */}
      <div className="sf-section-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <div>
            <div className="sf-section-title">Metode Transaksi ({metricScopeLabel})</div>
            <div className="sf-section-subtitle">Distribusi Transfer Rekening Bank vs Kas Tunai</div>
          </div>
        </div>

        {totalMethodAmount === 0 ? (
          <div style={{ padding: '24px 0', textAlign: 'center', fontSize: 13, color: 'var(--sf-text-muted)' }}>
            Belum ada transaksi pada periode ini
          </div>
        ) : (
          <div style={{ display: 'flex', alignItems: 'center', gap: 24, flexWrap: 'wrap' }}>
            <div style={{ width: 130, height: 130, flexShrink: 0 }}>
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={donutData}
                    dataKey="value"
                    nameKey="name"
                    cx="50%"
                    cy="50%"
                    innerRadius={36}
                    outerRadius={56}
                    paddingAngle={2}
                    stroke="none"
                  >
                    {donutData.map((entry) => (
                      <Cell key={entry.name} fill={entry.color} />
                    ))}
                  </Pie>
                  <RechartsTooltip
                    formatter={(val: number, name: string) => [
                      `Rp ${Number(val).toLocaleString('id-ID')}`,
                      name,
                    ]}
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

            <div style={{ flex: 1, minWidth: 200, display: 'flex', flexDirection: 'column', gap: 12 }}>
              {/* Transfer */}
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span
                    style={{
                      width: 10,
                      height: 10,
                      borderRadius: '50%',
                      background: '#a970ff',
                      display: 'inline-block',
                    }}
                  />
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
                  <span
                    style={{
                      width: 10,
                      height: 10,
                      borderRadius: '50%',
                      background: '#5794f2',
                      display: 'inline-block',
                    }}
                  />
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

      {/* Recent transactions */}
      <div className="sf-section-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <div className="sf-section-title">Transaksi Terbaru</div>
          {onNavigateToKas && (
            <button
              type="button"
              className="sf-btn sf-btn-outline"
              onClick={onNavigateToKas}
              style={{
                fontSize: 12,
                padding: '4px 10px',
                display: 'flex',
                alignItems: 'center',
                gap: 4,
                cursor: 'pointer',
              }}
            >
              Lihat Semua di Menu KAS <RightOutlined style={{ fontSize: 10 }} />
            </button>
          )}
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table className="sf-table" style={{ width: '100%', minWidth: 600 }}>
            <thead>
              <tr>
                <th style={{ textAlign: 'left' }}>Keterangan</th>
                <th style={{ textAlign: 'left', width: 140 }}>Lokasi Dana</th>
                <th style={{ textAlign: 'center', width: 90 }}>Metode</th>
                <th style={{ textAlign: 'left', width: 110 }}>Tanggal</th>
                <th style={{ textAlign: 'left', width: 100 }}>Arus</th>
                <th style={{ textAlign: 'right', width: 130 }}>Nominal</th>
              </tr>
            </thead>
            <tbody>
              {recentTransactions.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    style={{
                      textAlign: 'center',
                      color: 'var(--sf-text-muted)',
                      padding: '24px',
                      fontSize: 13,
                    }}
                  >
                    {loading ? 'Memuat transaksi...' : 'Belum ada transaksi di KAS RW'}
                  </td>
                </tr>
              ) : (
                recentTransactions.map((tx) => {
                  const isIncome = tx.type === 'income'
                  const locCat = tx.location_id ? locMap.get(tx.location_id) : undefined
                  const methodStr = (tx.payment_method || '').toUpperCase()
                  const isTransfer =
                    methodStr === 'TRANSFER' ||
                    locCat === 'rekening' ||
                    (!methodStr && !locCat)
                  const metodeLabel = isTransfer ? 'Transfer' : 'Tunai'
                  const amountNum = parseFloat(tx.amount) || 0

                  return (
                    <tr key={tx.id}>
                      <td style={{ textAlign: 'left' }}>
                        <div style={{ fontWeight: 600, color: 'var(--sf-text)' }}>
                          {tx.description || '—'}
                        </div>
                        {tx.source_rt_name && (
                          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>
                            Sumber: {tx.source_rt_name}
                          </div>
                        )}
                      </td>
                      <td style={{ textAlign: 'left', fontSize: 12, color: 'var(--sf-text-muted)' }}>
                        {tx.location_name || '—'}
                      </td>
                      <td style={{ textAlign: 'center' }}>
                        <span
                          style={{
                            fontSize: 11,
                            padding: '2px 8px',
                            borderRadius: 'var(--sf-radius-sm)',
                            fontWeight: 600,
                            background: isTransfer
                              ? 'rgba(169, 112, 255, 0.12)'
                              : 'rgba(87, 148, 242, 0.12)',
                            color: isTransfer ? '#a970ff' : '#5794f2',
                          }}
                        >
                          {metodeLabel}
                        </span>
                      </td>
                      <td style={{ textAlign: 'left', fontSize: 12, color: 'var(--sf-text-muted)' }}>
                        {formatTanggal(tx.occurred_at)}
                      </td>
                      <td style={{ textAlign: 'left' }}>
                        <span
                          style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: 4,
                            fontSize: 12,
                            fontWeight: 600,
                            color: isIncome ? 'var(--sf-success)' : 'var(--sf-danger)',
                          }}
                        >
                          {isIncome ? (
                            <>
                              <ArrowDownOutlined style={{ fontSize: 11 }} />
                              Masuk
                            </>
                          ) : (
                            <>
                              <ArrowUpOutlined style={{ fontSize: 11 }} />
                              Keluar
                            </>
                          )}
                        </span>
                      </td>
                      <td
                        style={{
                          textAlign: 'right',
                          fontWeight: 700,
                          fontFamily: 'monospace',
                          fontSize: 13,
                          color: isIncome ? 'var(--sf-success)' : 'var(--sf-danger)',
                        }}
                      >
                        {isIncome ? `+ ${formatRupiah(amountNum)}` : `- ${formatRupiah(amountNum)}`}
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
