import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  FundOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { AppCard } from '@/components/AppCard'
import { Pagination } from '@/components/Pagination'
import {
  BILL_DATA,
  formatRupiah,
  formatPeriodeLabel,
  formatFullPeriodeLabel,
  getStatusVariant,
  getStatusLabel,
  type IuranBill,
} from './iuranTypes'

const colStyles = {
  no: { width: 40, minWidth: 40, maxWidth: 40, textAlign: 'center' as const },
  household: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  type: { minWidth: 120, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  periode: { width: 100, minWidth: 100, maxWidth: 100, textAlign: 'center' as const },
  amount: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'right' as const, fontFamily: 'monospace' },
  status: { width: 110, minWidth: 110, maxWidth: 110, textAlign: 'center' as const },
}

export function IuranReport() {
  const navigate = useNavigate()

  const allBills = BILL_DATA

  // Summary stats
  const totalTagihan = useMemo(() => allBills.reduce((sum, b) => sum + b.nominal, 0), [allBills])
  const totalDibayar = useMemo(() => allBills.reduce((sum, b) => sum + b.paidAmount, 0), [allBills])
  const totalTunggakan = useMemo(() => totalTagihan - totalDibayar, [totalTagihan, totalDibayar])
  const jumlahLunas = useMemo(() => allBills.filter((b) => b.status === 'lunas').length, [allBills])
  const jumlahBelumLunas = useMemo(() => allBills.filter((b) => b.status !== 'lunas').length, [allBills])
  const jumlahSebagian = useMemo(() => allBills.filter((b) => b.status === 'sebagian').length, [allBills])
  const jumlahBelumBayar = useMemo(() => allBills.filter((b) => b.status === 'belum_bayar').length, [allBills])

  const [loading, setLoading] = useState(true)
  const [filterPeriode, setFilterPeriode] = useState('')
  const [recentBills, setRecentBills] = useState<IuranBill[]>([])
  const [recentTotal, setRecentTotal] = useState(0)
  const [recentPage, setRecentPage] = useState(1)
  const [recentPageSize, setRecentPageSize] = useState(10)
  const [recentTotalPages, setRecentTotalPages] = useState(1)

  // Unique periods for filter
  const uniquePeriodes = useMemo(() => {
    const s = new Set(allBills.map((b) => b.periode))
    return Array.from(s).sort().reverse()
  }, [allBills])

  const loadRecent = async (p: number, size: number) => {
    setLoading(true)
    await new Promise((r) => setTimeout(r, 400))

    let filtered = allBills
    if (filterPeriode) {
      filtered = filtered.filter((b) => b.periode === filterPeriode)
    }

    // Sort by most recent first
    filtered = [...filtered].sort((a, b) => {
      const periodCompare = b.periode.localeCompare(a.periode)
      if (periodCompare !== 0) return periodCompare
      return a.householdName.localeCompare(b.householdName)
    })

    const start = (p - 1) * size
    const paginated = filtered.slice(start, start + size)
    const totalPages = Math.max(1, Math.ceil(filtered.length / size))

    setRecentBills(paginated)
    setRecentTotal(filtered.length)
    setRecentTotalPages(totalPages)
    setRecentPage(p)
    setRecentPageSize(size)
    setLoading(false)
  }

  useEffect(() => {
    loadRecent(1, 10)
  }, [filterPeriode])

  const handlePage = (p: number) => {
    if (p >= 1 && p <= recentTotalPages) loadRecent(p, recentPageSize)
  }

  const handlePageSizeChange = (size: number) => {
    setRecentPageSize(size)
    loadRecent(1, size)
  }

  return (
    <>
      {/* PageHeader */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <FundOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Laporan Iuran</div>
            <div className="sf-page-header-subtitle">
              Ringkasan dan laporan pembayaran iuran warga
            </div>
          </div>
        </div>
      </div>

      <div className="sf-content-surface">
        {/* Summary cards */}
        <div className="sf-metrics-row" style={{ marginBottom: 'var(--sp-md)' }}>
          <div className="sf-metric-card">
            <div className="sf-metric-label">Total Tagihan</div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-text)' }}>
              {formatRupiah(totalTagihan)}
            </div>
          </div>
          <div className="sf-metric-card">
            <div className="sf-metric-label">Total Dibayar</div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-success)' }}>
              {formatRupiah(totalDibayar)}
            </div>
          </div>
          <div className="sf-metric-card">
            <div className="sf-metric-label">Total Tunggakan</div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-danger)' }}>
              {formatRupiah(totalTunggakan)}
            </div>
          </div>
        </div>

        <div className="sf-metrics-row" style={{ marginBottom: 'var(--sp-md)' }}>
          <div className="sf-metric-card">
            <div className="sf-metric-label">
              <CheckCircleOutlined style={{ marginRight: 4, color: 'var(--sf-success)' }} />
              Jumlah Lunas
            </div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-success)' }}>
              {jumlahLunas}
            </div>
          </div>
          <div className="sf-metric-card">
            <div className="sf-metric-label">
              <WarningOutlined style={{ marginRight: 4, color: 'var(--sf-danger)' }} />
              Belum Lunas
            </div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-danger)' }}>
              {jumlahBelumLunas}
            </div>
          </div>
          <div className="sf-metric-card">
            <div className="sf-metric-label">Sebagian</div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-warning)' }}>
              {jumlahSebagian}
            </div>
          </div>
          <div className="sf-metric-card">
            <div className="sf-metric-label">
              <ClockCircleOutlined style={{ marginRight: 4, color: 'var(--sf-text-muted)' }} />
              Belum Bayar
            </div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-text-muted)' }}>
              {jumlahBelumBayar}
            </div>
          </div>
        </div>

        {/* Filter */}
        <div style={{ marginBottom: 'var(--sp-md)' }}>
          <select
            className="sf-form-select"
            value={filterPeriode}
            onChange={(e) => setFilterPeriode(e.target.value)}
            style={{ width: 'auto', minWidth: 200 }}
          >
            <option value="">Semua Periode</option>
            {uniquePeriodes.map((p) => (
              <option key={p} value={p}>
                {formatFullPeriodeLabel(p)}
              </option>
            ))}
          </select>
        </div>

        {/* Loading */}
        {loading && (
          <div className="sf-loading-banner">
            Memuat laporan...
          </div>
        )}

        {/* Recent transactions */}
        {!loading && recentBills.length > 0 && (
          <AppCard>
            <div style={{ marginBottom: 'var(--sp-md)' }}>
              <h3 style={{ fontSize: '14px', fontWeight: 600, color: 'var(--sf-text)', margin: 0 }}>
                Data Iuran
              </h3>
              <p style={{ fontSize: '12px', color: 'var(--sf-text-muted)', margin: '2px 0 0' }}>
                {recentTotal} tagihan ditemukan
              </p>
            </div>
            <div className="table-wrapper">
              <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
                <thead>
                  <tr>
                    <th style={colStyles.no}>No</th>
                    <th style={colStyles.household}>WARGA/KK</th>
                    <th style={colStyles.type}>JENIS IURAN</th>
                    <th style={colStyles.periode}>PERIODE</th>
                    <th style={colStyles.amount}>TAGIHAN</th>
                    <th style={{ ...colStyles.amount, textAlign: 'right' }}>DIBAYAR</th>
                    <th style={colStyles.status}>STATUS</th>
                  </tr>
                </thead>
                <tbody>
                  {recentBills.map((bill, idx) => {
                    const sisa = bill.nominal - bill.paidAmount
                    return (
                      <tr key={bill.id}>
                        <td style={colStyles.no}>{idx + 1}</td>
                        <td style={colStyles.household}>
                          <span style={{ fontWeight: 600 }}>{bill.householdName}</span>
                        </td>
                        <td style={colStyles.type}>{bill.iuranType}</td>
                        <td style={colStyles.periode}>{formatPeriodeLabel(bill.periode)}</td>
                        <td style={{ ...colStyles.amount, textAlign: 'right' }}>
                          {formatRupiah(bill.nominal)}
                        </td>
                        <td style={{ ...colStyles.amount, textAlign: 'right', color: 'var(--sf-success)' }}>
                          {formatRupiah(bill.paidAmount)}
                        </td>
                        <td style={colStyles.status}>
                          <Badge variant={getStatusVariant(bill.status)}>
                            {getStatusLabel(bill.status)}
                          </Badge>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </AppCard>
        )}

        {!loading && recentBills.length === 0 && (
          <AppCard>
            <div style={{ textAlign: 'center', padding: 'var(--sp-xl) 0', color: 'var(--sf-text-muted)', fontSize: '13px' }}>
              Tidak ada data iuran untuk periode yang dipilih.
            </div>
          </AppCard>
        )}

        {/* Pagination */}
        {!loading && recentBills.length > 0 && (
          <Pagination
            page={recentPage}
            pageSize={recentPageSize}
            total={recentTotal}
            totalPages={recentTotalPages}
            onPageChange={handlePage}
            onPageSizeChange={handlePageSizeChange}
            pageSizeOptions={[10, 25, 50, 100]}
          />
        )}
      </div>
    </>
  )
}
