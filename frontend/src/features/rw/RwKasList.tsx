/**
 * RW KAS List — Buku Kas & Transaksi Finansial RW.
 *
 * Mendukung:
 *   - Laporan ringkasan KAS (Pemasukan, Pengeluaran, Saldo Bersih, Transaksi)
 *   - Catat KAS Masuk (penerimaan iuran dari RT / kas masuk lainnya ke lokasi dana tujuan)
 *   - Catat KAS Keluar (pengeluaran RW dari lokasi dana sumber dengan balance check)
 *   - Filter periode bulan/tahun atau semua periode
 *   - Filter arus kas (Semua, Masuk, Keluar) & pencarian
 *   - Detail lokasi dana per transaksi
 */

import { useState, useEffect, useMemo, useCallback } from 'react'
import { CheckCircleOutlined, WalletOutlined, ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { Pagination } from '@/components/Pagination'
import { useAuth } from '@/app/AuthContext'
import { getSessionPair } from '@/app/api'
import { formatRupiah } from '@/styles/format'
import { resolveRwLabel } from '@/utils/rwUtils'
import {
  apiListRWTransactions,
  apiGetRWKAS,
  apiListRWLocations,
  apiCreateRWTransaction,
  type ApiRWTransaction,
  type ApiRWKASReport,
  type ApiRWLocation,
} from './rw-api'

const MONTH_NAMES = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

function formatTanggal(dateStr: string): string {
  if (!dateStr) return '—'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}

export function RwKasList({ rwId, rwName }: { rwId: string; rwName?: string }) {
  const { user } = useAuth()

  const isSuperAdmin = user?.systemRole === 'super_admin' || user?.role === 'super_admin'
  const rwMemberships = (user as any)?.rw_memberships || (user as any)?.rwMemberships || []
  const currentRw = rwMemberships.find((m: any) => m.rw_id === rwId)
  const displayRwLabel = resolveRwLabel(rwId, rwMemberships, rwName)
  const jabatan = (currentRw?.jabatan || user?.jabatan || '').toLowerCase()
  const canCreate = isSuperAdmin || ['ketua', 'bendahara', 'wakil_ketua'].includes(jabatan)
  const canUpdate = isSuperAdmin || ['ketua', 'bendahara'].includes(jabatan)
  const isReadOnly = !canCreate && !canUpdate

  // Month filter state
  const [selectedMonth, setSelectedMonth] = useState<number>(new Date().getMonth() + 1)
  const [selectedYear, setSelectedYear] = useState<number>(new Date().getFullYear())
  const [isAllPeriods, setIsAllPeriods] = useState<boolean>(false)

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState<string>('')
  const [filterArus, setFilterArus] = useState<'all' | 'income' | 'expense'>('all')

  const [page, setPage] = useState<number>(1)
  const pageSize = 20

  const [kasReport, setKasReport] = useState<ApiRWKASReport | null>(null)
  const [rawTransactions, setRawTransactions] = useState<ApiRWTransaction[]>([])
  const [totalCount, setTotalCount] = useState<number>(0)
  const [totalPages, setTotalPages] = useState<number>(1)

  // Transaction Create Modal
  const [showTxModal, setShowTxModal] = useState(false)
  const [txType, setTxType] = useState<'income' | 'expense'>('income')
  const [locations, setLocations] = useState<ApiRWLocation[]>([])
  const [txForm, setTxForm] = useState<{
    location_id: string
    amount: string
    description: string
    payment_method: 'transfer' | 'cash'
    source_rt_name?: string
  }>({
    location_id: '',
    amount: '',
    description: '',
    payment_method: 'transfer',
    source_rt_name: '',
  })
  const [txError, setTxError] = useState<string | null>(null)
  const [submittingTx, setSubmittingTx] = useState(false)

  const activePeriodStr = isAllPeriods
    ? undefined
    : `${selectedYear}-${String(selectedMonth).padStart(2, '0')}`

  const loadData = useCallback(async () => {
    setLoading(true)
    setError(null)
    const session = getSessionPair()
    const token = session?.accessToken
    if (!token) {
      setLoading(false)
      return
    }

    try {
      const [report, txData, locData] = await Promise.all([
        apiGetRWKAS(token, rwId, activePeriodStr),
        apiListRWTransactions(token, rwId, {
          page,
          page_size: pageSize,
          period: activePeriodStr,
        }),
        apiListRWLocations(token, rwId, { is_active: true }),
      ])

      setKasReport(report)
      setRawTransactions(txData.data || [])
      setTotalCount(txData.pagination?.total || 0)
      setTotalPages(Math.max(1, txData.pagination?.total_pages || 1))
      setLocations(locData || [])
    } catch (err: any) {
      setError(err?.message || 'Gagal memuat data KAS RW')
    } finally {
      setLoading(false)
    }
  }, [rwId, activePeriodStr, page, pageSize])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleAllPeriods = () => {
    setIsAllPeriods((prev) => !prev)
    setPage(1)
  }

  const openCreateTx = (type: 'income' | 'expense') => {
    setTxType(type)
    setTxError(null)
    setTxForm({
      location_id: locations.length > 0 ? locations[0].id : '',
      amount: '',
      description: '',
      payment_method: 'transfer',
      source_rt_name: '',
    })
    setShowTxModal(true)
  }

  const handleCreateTx = async (e: React.FormEvent) => {
    e.preventDefault()
    const token = getSessionPair()?.accessToken
    if (!token) return

    if (!txForm.location_id) {
      setTxError('Pilih lokasi dana')
      return
    }
    const amt = parseFloat(txForm.amount) || 0
    if (amt <= 0) {
      setTxError('Nominal harus lebih dari 0')
      return
    }
    if (!txForm.description.trim()) {
      setTxError('Keterangan transaksi wajib diisi')
      return
    }

    if (txType === 'expense') {
      const selectedLoc = locations.find((l) => l.id === txForm.location_id)
      const locBal = parseFloat(selectedLoc?.current_balance || '0') || 0
      if (amt > locBal) {
        setTxError(`Saldo lokasi dana sumber (${formatRupiah(locBal)}) tidak mencukupi`)
        return
      }
    }

    setSubmittingTx(true)
    setTxError(null)

    try {
      await apiCreateRWTransaction(token, rwId, {
        type: txType,
        amount: `${amt.toFixed(2)}`,
        location_id: txForm.location_id,
        description: txForm.description.trim(),
        payment_method: txForm.payment_method,
      })
      setShowTxModal(false)
      loadData()
    } catch (err: any) {
      setTxError(err?.message || 'Gagal menyimpan transaksi KAS RW')
    } finally {
      setSubmittingTx(false)
    }
  }

  // Filtered transactions
  const filteredTransactions = useMemo(() => {
    let txs = [...rawTransactions]
    if (filterArus !== 'all') {
      txs = txs.filter((t) => t.type === filterArus)
    }
    if (search.trim()) {
      const q = search.trim().toLowerCase()
      txs = txs.filter((t) =>
        t.description.toLowerCase().includes(q) ||
        (t.rt_name && t.rt_name.toLowerCase().includes(q)) ||
        (t.location_name && t.location_name.toLowerCase().includes(q))
      )
    }
    return txs
  }, [rawTransactions, search, filterArus])

  if (!user) return null

  return (
    <>
      {/* PageHeader */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <WalletOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">KAS RW — {displayRwLabel}</div>
            <div className="sf-page-header-subtitle">
              {isReadOnly ? 'Buku kas RW (Mode Read-Only)' : 'Pencatatan dan monitoring kas keuangan tingkat RW'}
            </div>
          </div>
        </div>
      </div>

      {/* KAS Summary Cards */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: 12,
        marginTop: 8,
        marginBottom: 2,
      }}>
        <div style={{
          background: 'rgba(59, 130, 246, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(59, 130, 246, 0.1)',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Pemasukan
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-success)' }}>
            {loading ? '...' : formatRupiah(parseFloat(kasReport?.total_income || '0') || 0)}
          </div>
        </div>

        <div style={{
          background: 'rgba(255, 59, 48, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(255, 59, 48, 0.1)',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Pengeluaran
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-danger)' }}>
            {loading ? '...' : formatRupiah(parseFloat(kasReport?.total_expense || '0') || 0)}
          </div>
        </div>

        <div style={{
          background: 'rgba(169, 112, 255, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(169, 112, 255, 0.1)',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Saldo Bersih
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-accent)' }}>
            {loading ? '...' : formatRupiah(parseFloat(kasReport?.net_balance || '0') || 0)}
          </div>
        </div>

        <div style={{
          background: 'rgba(100, 116, 139, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(100, 116, 139, 0.1)',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Transaksi
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-text)' }}>
            {loading ? '...' : kasReport?.transaction_count ?? 0}
          </div>
        </div>
      </div>

      {/* Main Content Surface */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar" style={{ flexWrap: 'wrap', gap: 8 }}>
          <div className="sf-list-toolbar-left" style={{ flexWrap: 'wrap', gap: 8 }}>
            {/* Period Navigation */}
            <select
              className="sf-form-select"
              value={selectedMonth}
              onChange={(e) => {
                setIsAllPeriods(false)
                setSelectedMonth(parseInt(e.target.value, 10))
                setPage(1)
              }}
              style={{ fontSize: 13, height: 34, padding: '4px 10px', width: 140 }}
              aria-label="Pilih Bulan"
            >
              {MONTH_NAMES.map((m, idx) => (
                <option key={m} value={idx + 1}>{m}</option>
              ))}
            </select>

            <select
              className="sf-form-select"
              value={selectedYear}
              onChange={(e) => {
                setIsAllPeriods(false)
                setSelectedYear(parseInt(e.target.value, 10))
                setPage(1)
              }}
              style={{ fontSize: 13, height: 34, padding: '4px 10px', width: 100 }}
              aria-label="Pilih Tahun"
            >
              {[2024, 2025, 2026, 2027].map((y) => (
                <option key={y} value={y}>{y}</option>
              ))}
            </select>

            {/* Filter Arus */}
            <select
              className="sf-form-select"
              value={filterArus}
              onChange={(e) => setFilterArus(e.target.value as any)}
              style={{ fontSize: 13, height: 34, padding: '4px 10px', width: 'auto', minWidth: 130 }}
              aria-label="Filter Arus Kas"
            >
              <option value="all">Semua Arus Kas</option>
              <option value="income">Pemasukan (+)</option>
              <option value="expense">Pengeluaran (-)</option>
            </select>

            {/* Search */}
            <div style={{ minWidth: 200, maxWidth: 320 }}>
              <SearchBox
                value={search}
                onChange={(v) => setSearch(v)}
                placeholder="Cari keterangan, RT, lokasi..."
              />
            </div>
          </div>

          <div className="sf-list-toolbar-right" style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <button
              type="button"
              className={`sf-btn sf-kas-period-button ${isAllPeriods ? 'sf-btn-primary' : 'sf-btn-outline'}`}
              onClick={handleAllPeriods}
              style={{
                padding: '6px 16px',
                fontSize: 13,
                fontWeight: 600,
                borderRadius: 'var(--sf-radius-sm)',
              }}
            >
              {isAllPeriods && <CheckCircleOutlined style={{ fontSize: 11, marginRight: 5 }} />}
              Semua Periode
            </button>

            {canCreate && (
              <>
                <button
                  type="button"
                  className="sf-btn sf-btn-primary"
                  onClick={() => openCreateTx('income')}
                  style={{
                    fontSize: 13,
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 6,
                    background: 'var(--sf-success)',
                    borderColor: 'var(--sf-success)',
                  }}
                >
                  <ArrowDownOutlined /> Catat Kas Masuk
                </button>
                <button
                  type="button"
                  className="sf-btn sf-btn-primary"
                  onClick={() => openCreateTx('expense')}
                  style={{
                    fontSize: 13,
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 6,
                    background: 'var(--sf-danger)',
                    borderColor: 'var(--sf-danger)',
                  }}
                >
                  <ArrowUpOutlined /> Catat Kas Keluar
                </button>
              </>
            )}
          </div>
        </div>

        {error && (
          <div className="sf-error-banner" role="alert">{error}</div>
        )}

        {loading && (
          <div className="sf-loading-banner">Memuat data KAS RW...</div>
        )}

        {!loading && rawTransactions.length === 0 && !error && (
          <div className="sf-empty-banner">
            Belum ada transaksi pada periode ini.
          </div>
        )}

        {/* Table */}
        {!loading && filteredTransactions.length > 0 && (
          <div className="table-wrapper">
            <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th style={{ width: 110, minWidth: 110, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    TANGGAL
                  </th>
                  <th style={{ width: 100, minWidth: 100, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    RT/SUMBER
                  </th>
                  <th style={{ width: 140, minWidth: 140, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    LOKASI DANA
                  </th>
                  <th style={{ minWidth: 220, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    KETERANGAN
                  </th>
                  <th style={{ width: 120, minWidth: 120, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    ARUS
                  </th>
                  <th style={{ width: 130, minWidth: 130, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)', background: 'var(--sf-kas-income-header-bg)' }}>
                    PEMASUKAN (+)
                  </th>
                  <th style={{ width: 130, minWidth: 130, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)', background: 'var(--sf-kas-expense-header-bg)' }}>
                    PENGELUARAN (-)
                  </th>
                </tr>
              </thead>
              <tbody>
                {filteredTransactions.map((t) => {
                  const isIncome = t.type === 'income'
                  return (
                    <tr key={t.id}>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text-muted)', whiteSpace: 'nowrap' }}>
                        {formatTanggal(t.occurred_at || t.created_at)}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 12, color: 'var(--sf-text)' }}>
                        {t.source_rt_name || t.rt_name || t.rt_id || 'RW'}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 12, color: 'var(--sf-text)', fontWeight: 500 }}>
                        {t.location_name || '—'}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {t.description}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)' }}>
                        <Badge
                          variant={isIncome ? 'green' : 'red'}
                          style={{ height: 20, padding: '0 6px', fontSize: 10, fontWeight: 700, display: 'inline-flex', alignItems: 'center' }}
                        >
                          {isIncome ? '↓ Masuk' : '↑ Keluar'}
                        </Badge>
                      </td>
                      <td style={{
                        padding: '10px 12px',
                        borderBottom: '1px solid var(--sf-border)',
                        textAlign: 'right',
                        fontFamily: 'monospace',
                        fontSize: 13,
                        color: isIncome ? 'var(--sf-success)' : 'var(--sf-text-muted)',
                        fontWeight: isIncome ? 600 : 400,
                        background: isIncome ? 'var(--sf-kas-income-bg)' : 'transparent',
                      }}>
                        {isIncome ? formatRupiah(parseFloat(t.amount) || 0) : '—'}
                      </td>
                      <td style={{
                        padding: '10px 12px',
                        borderBottom: '1px solid var(--sf-border)',
                        textAlign: 'right',
                        fontFamily: 'monospace',
                        fontSize: 13,
                        color: !isIncome ? 'var(--sf-danger)' : 'var(--sf-text-muted)',
                        fontWeight: !isIncome ? 600 : 400,
                        background: !isIncome ? 'var(--sf-kas-expense-bg)' : 'transparent',
                      }}>
                        {!isIncome ? formatRupiah(parseFloat(t.amount) || 0) : '—'}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && totalPages > 1 && (
          <Pagination
            page={page}
            pageSize={pageSize}
            total={totalCount}
            totalPages={totalPages}
            onPageChange={(p) => setPage(p)}
          />
        )}
      </div>

      {/* Modal: Catat Kas Masuk / Keluar */}
      {showTxModal && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 480 }}>
            <div className="sf-modal-header">
              <h3>{txType === 'income' ? 'Catat KAS Masuk RW' : 'Catat KAS Keluar RW'}</h3>
              <button type="button" className="sf-modal-close" onClick={() => setShowTxModal(false)}>✕</button>
            </div>
            <form onSubmit={handleCreateTx}>
              <div className="sf-modal-body">
                {txError && <div className="sf-error-banner" style={{ marginBottom: 12 }}>{txError}</div>}
                
                {locations.length === 0 ? (
                  <div className="sf-empty-banner">
                    Tidak ada lokasi dana aktif. Silakan tambahkan lokasi dana terlebih dahulu.
                  </div>
                ) : (
                  <>
                    <div className="sf-form-group">
                      <label>{txType === 'income' ? 'Lokasi Dana Tujuan *' : 'Lokasi Dana Sumber *'}</label>
                      <select
                        className="sf-form-select"
                        value={txForm.location_id}
                        onChange={(e) => setTxForm({ ...txForm, location_id: e.target.value })}
                        required
                      >
                        {locations.map((loc) => {
                          const bal = parseFloat(loc.current_balance) || 0
                          return (
                            <option key={loc.id} value={loc.id}>
                              {loc.name} ({formatRupiah(bal)})
                            </option>
                          )
                        })}
                      </select>
                    </div>

                    <div className="sf-form-group">
                      <label>Nominal (Rp) *</label>
                      <input
                        type="number"
                        min="1"
                        step="1"
                        className="sf-form-input"
                        value={txForm.amount}
                        onChange={(e) => setTxForm({ ...txForm, amount: e.target.value })}
                        placeholder="Contoh: 1000000"
                        required
                      />
                    </div>

                    <div className="sf-form-group">
                      <label>Metode Pembayaran *</label>
                      <select
                        className="sf-form-select"
                        value={txForm.payment_method}
                        onChange={(e) => setTxForm({ ...txForm, payment_method: e.target.value as any })}
                      >
                        <option value="transfer">Transfer Bank</option>
                        <option value="cash">Tunai (Cash)</option>
                      </select>
                    </div>

                    <div className="sf-form-group">
                      <label>Keterangan *</label>
                      <textarea
                        className="sf-form-input"
                        rows={3}
                        value={txForm.description}
                        onChange={(e) => setTxForm({ ...txForm, description: e.target.value })}
                        placeholder={txType === 'income' ? 'Contoh: Iuran RW dari RT 01...' : 'Contoh: Pembelian perlengkapan keamanan RW...'}
                        required
                      />
                    </div>
                  </>
                )}
              </div>
              <div className="sf-modal-footer">
                <button type="button" className="sf-btn sf-btn-outline" onClick={() => setShowTxModal(false)}>Batal</button>
                <button
                  type="submit"
                  className="sf-btn sf-btn-primary"
                  disabled={submittingTx || locations.length === 0}
                  style={{
                    background: txType === 'income' ? 'var(--sf-success)' : 'var(--sf-danger)',
                    borderColor: txType === 'income' ? 'var(--sf-success)' : 'var(--sf-danger)',
                  }}
                >
                  {submittingTx ? 'Menyimpan...' : 'Simpan Transaksi'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
