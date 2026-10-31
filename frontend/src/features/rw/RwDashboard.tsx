/**
 * RW-aware Dashboard — reuses the same visual structure as RT Dashboard
 * but fetches data from PR 4 RW endpoints.
 *
 * The PR 4 RW dashboard response is FLAT (no chart, no dues_progress,
 * no payment_method_breakdown). This component fills in sensible defaults
 * so the layout renders consistently.
 *
 * Read-only by design — RW dashboard shows aggregated data, no write actions.
 */

import { useState, useEffect } from 'react'
import { useAuth } from '@/app/AuthContext'
import { getSessionPair } from '@/app/api'
import { formatRupiah } from '@/styles/format'
import { PageHeader } from '@/components/PageHeader'
import { DashboardOutlined, LeftOutlined, RightOutlined } from '@ant-design/icons'
import { apiGetRWDashboard, type ApiRWDashboardSummary } from './rw-api'

const MONTH_NAMES = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

function formatRupiahShort(val: number): string {
  if (val >= 1000000000) return `Rp ${(val / 1000000000).toFixed(1)} M`
  if (val >= 1000000) return `Rp ${(val / 1000000).toFixed(1)} Jt`
  if (val >= 1000) return `Rp ${(val / 1000).toFixed(0)} Rb`
  return formatRupiah(val)
}

export function RwDashboard({
  rwId,
  rwName,
}: {
  rwId: string
  rwName?: string
}) {
  const { user } = useAuth()
  const now = new Date()
  const [selectedMonth, setSelectedMonth] = useState<number>(now.getMonth() + 1)
  const [selectedYear, setSelectedYear] = useState<number>(now.getFullYear())
  const [isAllPeriods, setIsAllPeriods] = useState<boolean>(false)
  const [data, setData] = useState<ApiRWDashboardSummary | null>(null)
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
    const accessToken = getSessionPair()?.accessToken
    if (!accessToken) {
      setLoading(false)
      return
    }

    setLoading(true)
    apiGetRWDashboard(accessToken, rwId, activePeriodStr)
      .then((res) => {
        if (!cancelled) {
          setData(res)
          setLoading(false)
        }
      })
      .catch((err) => {
        console.error('Failed to load RW dashboard summary:', err)
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [rwId, activePeriodStr])

  if (!user) return null

  const balanceNum = data ? parseFloat(data.total_balance) || 0 : 0
  const totalIncomeNum = data ? parseFloat(data.total_income) || 0 : 0
  const totalExpenseNum = data ? parseFloat(data.total_expense) || 0 : 0

  const metricScopeLabel = isAllPeriods ? 'Semua' : MONTH_NAMES[selectedMonth - 1]

  const periodDisplayLabel = isAllPeriods ? 'Semua Periode' : `${MONTH_NAMES[selectedMonth - 1]} ${selectedYear}`

  return (
    <>
      {/* PageHeader */}
      <PageHeader
        surface
        icon={<DashboardOutlined />}
        title={`Selamat datang, ${user.name}`}
        subtitle={rwName || `RW ${rwId}`}
        actions={
          <span style={{ padding: '4px 10px', fontSize: 12, fontWeight: 600, borderRadius: 'var(--sf-radius-sm)', color: 'var(--sf-accent)', background: 'rgba(169, 112, 255, 0.1)' }}>
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
              color: isAllPeriods ? 'var(--sf-accent)' : 'var(--sf-accent)',
              borderRadius: 'var(--sf-radius-sm)',
            }}
          >
            Semua Periode
          </button>

          <div style={{
            padding: '5px 12px',
            borderRadius: 'var(--sf-radius-sm)',
            background: 'rgba(59, 130, 246, 0.1)',
            border: '1px solid rgba(59, 130, 246, 0.25)',
            fontSize: 12,
            fontWeight: 600,
            color: 'var(--sf-accent)',
            whiteSpace: 'nowrap',
          }}>
            {periodDisplayLabel}
          </div>
        </div>
      </div>

      {/* Metrics row */}
      <div className="sf-metrics-row">
        <div className="sf-metric-card">
          <div className="sf-metric-label">Total Saldo ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-default">
            {loading && !data ? '...' : formatRupiahShort(balanceNum)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Total Pemasukan ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-income">
            {loading && !data ? '...' : formatRupiahShort(totalIncomeNum)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Total Pengeluaran ({metricScopeLabel})</div>
          <div className="sf-metric-value sf-metric-expense">
            {loading && !data ? '...' : formatRupiahShort(totalExpenseNum)}
          </div>
        </div>
      </div>

      {/* Info card: RW scope */}
      <div className="sf-section-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <div className="sf-section-title">Ringkasan RW</div>
            <div className="sf-section-subtitle">Pencatatan kas dan lokasi dana mandiri tingkat RW</div>
          </div>
          <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-accent)' }}>{data?.total_rts ?? '—'}</div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Total RT</div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-info)', fontFamily: 'monospace' }}>{data?.transaction_count ?? '—'}</div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Transaksi</div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-success)' }}>{data?.total_locations ?? '—'}</div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Lokasi Dana</div>
            </div>
          </div>
        </div>
      </div>

      {/* Financial chart (placeholder — RW API doesn't provide monthly breakdown yet) */}
      <div className="sf-section-card">
        <div className="sf-section-header">
          <div>
            <div className="sf-section-title">Ringkasan Keuangan</div>
            <div className="sf-section-subtitle">Tahun {selectedYear} (12 Bulan) — Data belum tersedia</div>
          </div>
        </div>
        <div style={{ height: 240, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ color: 'var(--sf-text-muted)', fontSize: 13 }}>
            Grafik bulanan belum tersedia untuk KAS RW
          </div>
        </div>
      </div>

      {/* Donut Panel — placeholder */}
      <div className="sf-section-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          <div className="sf-section-title">Metode Transaksi</div>
          <span style={{ fontSize: 11, color: 'var(--sf-text-muted)' }}>Data belum tersedia</span>
        </div>
        <div style={{ padding: '24px 0', textAlign: 'center', fontSize: 13, color: 'var(--sf-text-muted)' }}>
          Belum ada transaksi pada periode ini
        </div>
      </div>

      {/* Recent transactions — empty placeholder */}
      <div className="sf-section-card">
        <div className="sf-section-title">Transaksi Terbaru</div>
        <div style={{ marginTop: 8, textAlign: 'center', padding: '16px', color: 'var(--sf-text-muted)', fontSize: 13 }}>
          {loading ? 'Memuat transaksi...' : 'Daftar transaksi lengkap tersedia di menu KAS'}
        </div>
      </div>
    </>
  )
}
