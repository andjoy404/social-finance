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
import { CheckCircleOutlined, WalletOutlined, ArrowDownOutlined, ArrowUpOutlined, EditOutlined, BarChartOutlined } from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { Pagination } from '@/components/Pagination'
import { useAuth, hasRwWriteAccess } from '@/app/AuthContext'
import { getSessionPair, apiListCategories, type ApiFinancialCategory } from '@/app/api'
import { formatRupiah } from '@/styles/format'
import { resolveRwLabel } from '@/utils/rwUtils'
import { RwKasReport, getRwKasReportAccess } from './RwKasReport'
import {
  apiListRWTransactions,
  apiGetRWKASReportDetail,
  apiListRWLocations,
  apiCreateRWTransaction,
  apiUpdateRWOpeningBalance,
  apiListRWRTRs,
  type ApiRWTransaction,
  type ApiRWKASReportDetail,
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

  const rwMemberships = (user as any)?.rw_memberships || (user as any)?.rwMemberships || []
  const displayRwLabel = resolveRwLabel(rwId, rwMemberships, rwName)

  // Source-aware write permission: Only Ketua RW and Bendahara RW have write access.
  // Pengurus RW (wakil_ketua, sekretaris) and Pengurus RT get read-only.
  const canWrite = hasRwWriteAccess(user, rwId)
  const canCreate = canWrite
  const canUpdate = canWrite
  const canEditSaldoAwal = canWrite
  const canViewReport = getRwKasReportAccess(user, rwId).canView
  const isReadOnly = !canCreate && !canUpdate

  const [reportModalOpen, setReportModalOpen] = useState<boolean>(false)

  // Month filter state
  const [selectedMonth, setSelectedMonth] = useState<number>(new Date().getMonth() + 1)
  const [selectedYear, setSelectedYear] = useState<number>(new Date().getFullYear())
  const [isAllPeriods, setIsAllPeriods] = useState<boolean>(false)
  const [dateRange, setDateRange] = useState<{ start: string; end: string } | null>(null)

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState<string>('')
  const [filterArus, setFilterArus] = useState<'all' | 'income' | 'expense'>('all')
  const [filterCategoryId, setFilterCategoryId] = useState<string>('all')

  const [page, setPage] = useState<number>(1)
  const pageSize = 20

  const [kasReport, setKasReport] = useState<ApiRWKASReportDetail | null>(null)
  const [rawTransactions, setRawTransactions] = useState<ApiRWTransaction[]>([])
  const [totalCount, setTotalCount] = useState<number>(0)
  const [totalPages, setTotalPages] = useState<number>(1)

  // Saldo Awal Edit Modal state
  const [showOpeningBalanceModal, setShowOpeningBalanceModal] = useState(false)
  const [openingBalanceInput, setOpeningBalanceInput] = useState('')
  const [openingBalanceError, setOpeningBalanceError] = useState<string | null>(null)
  const [submittingOpeningBalance, setSubmittingOpeningBalance] = useState(false)

  // Transaction Create Modal
  const [showTxModal, setShowTxModal] = useState(false)
  const [txType, setTxType] = useState<'income' | 'expense'>('income')
  const [txIncomeSubType, setTxIncomeSubType] = useState<'kas_masuk' | 'setoran_rt'>('kas_masuk')
  const [locations, setLocations] = useState<ApiRWLocation[]>([])
  const [rts, setRts] = useState<{ id: string; name: string; rt: string; label: string }[]>([])
  const [expenseCategories, setExpenseCategories] = useState<ApiFinancialCategory[]>([])
  const [txForm, setTxForm] = useState<{
    location_id: string
    amount: string
    description: string
    payment_method: 'transfer' | 'cash'
    source_rt_name?: string
    source_rt_id?: string
    expense_category?: string
  }>({
    location_id: '',
    amount: '',
    description: '',
    payment_method: 'transfer',
    source_rt_name: '',
    source_rt_id: '',
    expense_category: '',
  })
  const [txError, setTxError] = useState<string | null>(null)
  const [submittingTx, setSubmittingTx] = useState(false)

  const isFullYearRange =
    !!dateRange &&
    dateRange.start.endsWith('-01-01') &&
    dateRange.end.endsWith('-12-31') &&
    dateRange.start.slice(0, 4) === dateRange.end.slice(0, 4)

  // Month-mode period for the report/list calls (YYYY-MM). Range mode and all-periods pass none.
  const monthPeriodParam = !isAllPeriods && !dateRange
    ? `${selectedYear}-${String(selectedMonth).padStart(2, '0')}`
    : undefined

  // Params for the period-consistent /kas/report endpoint (summary cards + modal document).
  const reportDetailParams = useMemo(
    () =>
      dateRange
        ? { startDate: dateRange.start, endDate: dateRange.end }
        : monthPeriodParam
          ? { period: monthPeriodParam }
          : {},
    [dateRange, monthPeriodParam],
  )

  // Report period prop for the document header ('YYYY' for a full-year range).
  const reportPeriodProp = isFullYearRange
    ? (dateRange!.start.slice(0, 4) as string)
    : monthPeriodParam ?? null

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
      const [report, txData, locData, rtData, catData] = await Promise.all([
        apiGetRWKASReportDetail(token, rwId, reportDetailParams),
        apiListRWTransactions(token, rwId, {
          page,
          page_size: pageSize,
          period: monthPeriodParam,
          start_date: dateRange ? dateRange.start : undefined,
          end_date: dateRange ? dateRange.end : undefined,
          search: search.trim() || undefined,
          categoryId: filterCategoryId === 'all' ? undefined : filterCategoryId,
        }),
        apiListRWLocations(token, rwId, { is_active: true }),
        apiListRWRTRs(token, rwId),
        apiListCategories(token, true),
      ])

      setKasReport(report)
      setRawTransactions(txData.data || [])
      setTotalCount(txData.pagination?.total || 0)
      setTotalPages(Math.max(1, txData.pagination?.total_pages || 1))
      setLocations(locData || [])
      setRts(rtData || [])
      setExpenseCategories(catData || [])
    } catch (err: any) {
      setError(err?.message || 'Gagal memuat data KAS RW')
    } finally {
      setLoading(false)
    }
  }, [rwId, monthPeriodParam, page, pageSize, search, dateRange, reportDetailParams, filterCategoryId])

  useEffect(() => {
    loadData()
  }, [loadData])

  const formatShortDate = (iso: string): string => {
    const d = new Date(`${iso}T00:00:00`)
    if (Number.isNaN(d.getTime())) return iso
    return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
  }

  const modalSubtitle = isAllPeriods
    ? 'Semua Periode'
    : dateRange
      ? `${formatShortDate(dateRange.start)} – ${formatShortDate(dateRange.end)}`
      : `${MONTH_NAMES[selectedMonth - 1]?.slice(0, 3) ?? ''} ${selectedYear}`

  const handleAllPeriods = () => {
    setIsAllPeriods((prev) => !prev)
    setDateRange(null)
    setPage(1)
  }

  const getMethodForLocation = useCallback((locId: string): 'transfer' | 'cash' => {
    const loc = locations.find((l) => l.id === locId)
    if (!loc) return 'transfer'
    const cat = (loc.category || '').toLowerCase()
    if (cat === 'rekening') return 'transfer'
    return 'cash' // brankas, coh
  }, [locations])

  const openEditOpeningBalance = () => {
    const currentVal = kasReport?.opening_balance ? parseFloat(kasReport.opening_balance) : 0
    setOpeningBalanceInput(isNaN(currentVal) ? '0' : String(currentVal))
    setOpeningBalanceError(null)
    setShowOpeningBalanceModal(true)
  }

  const handleSaveOpeningBalance = async (e: React.FormEvent) => {
    e.preventDefault()
    const token = getSessionPair()?.accessToken
    if (!token) return

    const num = parseFloat(openingBalanceInput)
    if (isNaN(num) || num < 0) {
      setOpeningBalanceError('Saldo awal harus berupa angka dan tidak boleh negatif')
      return
    }

    setSubmittingOpeningBalance(true)
    setOpeningBalanceError(null)
    try {
      await apiUpdateRWOpeningBalance(token, rwId, num.toFixed(2))
      setShowOpeningBalanceModal(false)
      await loadData()
    } catch (err: any) {
      setOpeningBalanceError(err?.message || 'Gagal mengubah saldo awal')
    } finally {
      setSubmittingOpeningBalance(false)
    }
  }

  const openCreateTx = (type: 'income' | 'expense') => {
    setTxType(type)
    setTxError(null)
    if (type === 'income') {
      setTxIncomeSubType('kas_masuk')
    }
    const initialLocId = locations.length > 0 ? locations[0].id : ''
    setTxForm({
      location_id: initialLocId,
      amount: '',
      description: '',
      payment_method: getMethodForLocation(initialLocId),
      source_rt_name: '',
      source_rt_id: '',
      expense_category: '',
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

    const derivedMethod = getMethodForLocation(txForm.location_id)

    setSubmittingTx(true)
    setTxError(null)

    try {
      await apiCreateRWTransaction(token, rwId, {
        type: txType,
        amount: `${amt.toFixed(2)}`,
        location_id: txForm.location_id,
        description: txForm.description.trim(),
        payment_method: derivedMethod,
        source_rt_id: txIncomeSubType === 'setoran_rt' && txForm.source_rt_id ? txForm.source_rt_id : undefined,
        expense_category: txType === 'expense' && txForm.expense_category ? txForm.expense_category : undefined,
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
        (t.reference_no && t.reference_no.toLowerCase().includes(q)) ||
        (t.rt_name && t.rt_name.toLowerCase().includes(q)) ||
        (t.location_name && t.location_name.toLowerCase().includes(q))
      )
    }
    return txs
  }, [rawTransactions, search, filterArus])

  if (!user) return null

  const openingBalNum = parseFloat(kasReport?.opening_balance || '0') || 0
  const incomeBalNum = parseFloat(kasReport?.total_income || '0') || 0
  const expenseBalNum = parseFloat(kasReport?.total_expense || '0') || 0
  const netBalNum = incomeBalNum - expenseBalNum
  // Server-computed closing (carry-forward semantics) takes precedence when present.
  const totalSaldoNum = kasReport?.total_balance != null
    ? parseFloat(kasReport.total_balance) || 0
    : openingBalNum + netBalNum

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
        {canViewReport && (
          <div className="sf-page-header-right" style={{ marginLeft: 'auto' }}>
            <button
              type="button"
              className="sf-btn sf-btn-outline"
              onClick={() => setReportModalOpen(true)}
              aria-label="Laporan KAS"
            >
              <BarChartOutlined style={{ fontSize: 12, marginRight: 6 }} />
              Laporan KAS
            </button>
          </div>
        )}
      </div>

      {/* KAS Summary Cards */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: 12,
        marginTop: 8,
        marginBottom: 2,
      }}>
        {/* TOTAL SALDO */}
        <div style={{
          background: 'rgba(59, 130, 246, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(59, 130, 246, 0.1)',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Saldo
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-accent)' }}>
            {loading ? '...' : formatRupiah(totalSaldoNum)}
          </div>
        </div>

        {/* SALDO AWAL */}
        <div style={{
          background: 'rgba(100, 116, 139, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(100, 116, 139, 0.1)',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
            <span style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600 }}>
              Saldo Awal
            </span>
            {canEditSaldoAwal && (
              <button
                type="button"
                onClick={openEditOpeningBalance}
                style={{
                  background: 'none',
                  border: 'none',
                  color: 'var(--sf-accent)',
                  cursor: 'pointer',
                  fontSize: 12,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 4,
                  padding: 0,
                  fontWeight: 600,
                }}
                title="Ubah Saldo Awal"
              >
                <EditOutlined style={{ fontSize: 12 }} /> Ubah
              </button>
            )}
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-text)' }}>
            {loading ? '...' : formatRupiah(openingBalNum)}
          </div>
        </div>

        {/* TOTAL PEMASUKAN */}
        <div style={{
          background: 'rgba(34, 197, 94, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(34, 197, 94, 0.1)',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Pemasukan
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-success)' }}>
            {loading ? '...' : formatRupiah(incomeBalNum)}
          </div>
        </div>

        {/* TOTAL PENGELUARAN */}
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
            {loading ? '...' : formatRupiah(expenseBalNum)}
          </div>
        </div>

        {/* SALDO BERSIH */}
        <div style={{
          background: 'rgba(169, 112, 255, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(169, 112, 255, 0.1)',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Saldo Bersih
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: netBalNum >= 0 ? 'var(--sf-accent)' : 'var(--sf-danger)' }}>
            {loading ? '...' : formatRupiah(netBalNum)}
          </div>
        </div>

        {/* TRANSAKSI */}
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
                setDateRange(null)
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
                setDateRange(null)
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

            {/* Filter Rentang Tanggal */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <input
                type="date"
                className="sf-form-input"
                value={dateRange?.start ?? ''}
                onChange={(e) => {
                  const start = e.target.value
                  setIsAllPeriods(false)
                  setPage(1)
                  setDateRange((prev) => {
                    if (!start) return null
                    const end = prev?.end && start <= prev.end ? prev.end : start
                    return { start, end }
                  })
                }}
                style={{ fontSize: 13, height: 34, padding: '4px 8px', width: 150 }}
                aria-label="Tanggal mulai"
              />
              <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>s/d</span>
              <input
                type="date"
                className="sf-form-input"
                value={dateRange?.end ?? ''}
                onChange={(e) => {
                  const end = e.target.value
                  setIsAllPeriods(false)
                  setPage(1)
                  setDateRange((prev) => {
                    if (!end) return null
                    const start = prev?.start && prev.start <= end ? prev.start : end
                    return { start, end }
                  })
                }}
                style={{ fontSize: 13, height: 34, padding: '4px 8px', width: 150 }}
                aria-label="Tanggal selesai"
              />
              {dateRange && (
                <button
                  type="button"
                  className="sf-btn sf-btn-outline"
                  onClick={() => {
                    setDateRange(null)
                    setPage(1)
                  }}
                  style={{ padding: '6px 10px', fontSize: 13 }}
                  aria-label="Hapus rentang tanggal"
                >
                  ×
                </button>
              )}
            </div>

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

            {/* Filter Kategori */}
            <select
              className="sf-form-select"
              value={filterCategoryId}
              onChange={(e) => {
                setFilterCategoryId(e.target.value)
                setPage(1)
              }}
              style={{ fontSize: 13, height: 34, padding: '4px 10px', width: 'auto', minWidth: 150 }}
              aria-label="Filter Kategori"
            >
              <option value="all">Semua Kategori</option>
              {expenseCategories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
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
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', maxWidth: 300 }}>
                        {t.reference_no && (
                          <div style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--sf-accent)', fontWeight: 600, marginBottom: 2 }}>
                            {t.reference_no}
                          </div>
                        )}
                        <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={t.description}>
                          {t.description}
                        </div>
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
                        onChange={(e) => {
                          const newLocId = e.target.value
                          setTxForm({
                            ...txForm,
                            location_id: newLocId,
                            payment_method: getMethodForLocation(newLocId),
                          })
                        }}
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

                    {txType === 'income' && (
                      <>
                        <div className="sf-form-group">
                          <label>Jenis Kas Masuk *</label>
                          <select
                            className="sf-form-select"
                            value={txIncomeSubType}
                            onChange={(e) => setTxIncomeSubType(e.target.value as 'kas_masuk' | 'setoran_rt')}
                            required
                          >
                            <option value="kas_masuk">Kas Masuk</option>
                            <option value="setoran_rt">Setoran RT</option>
                          </select>
                        </div>
                        {txIncomeSubType === 'setoran_rt' && (
                          <div className="sf-form-group">
                            <label>RT Sumber *</label>
                            <select
                              className="sf-form-select"
                              value={txForm.source_rt_id || ''}
                              onChange={(e) => {
                                const rtId = e.target.value
                                const rt = rts.find((r) => r.id === rtId)
                                setTxForm({
                                  ...txForm,
                                  source_rt_id: rtId,
                                  source_rt_name: rt?.name || '',
                                })
                              }}
                              required
                            >
                              <option value="">-- Pilih RT Sumber --</option>
                              {rts.map((rt) => (
                                <option key={rt.id} value={rt.id}>
                                  {rt.label}
                                </option>
                              ))}
                            </select>
                            {rts.length === 0 && (
                              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
                                Tidak ada RT yang tersedia di RW ini.
                              </div>
                            )}
                          </div>
                        )}
                      </>
                    )}

                    {txType === 'expense' && (
                      <div className="sf-form-group">
                        <label>Kategori Kas Keluar *</label>
                        <select
                          className="sf-form-select"
                          value={txForm.expense_category || ''}
                          onChange={(e) => setTxForm({ ...txForm, expense_category: e.target.value })}
                          required
                        >
                          <option value="">-- Pilih Kategori Pengeluaran --</option>
                          {expenseCategories
                            .filter((c) => c.type === 'expense')
                            .map((c) => (
                              <option key={c.id} value={c.id}>
                                {c.name}
                              </option>
                            ))}
                        </select>
                        {expenseCategories.filter((c) => c.type === 'expense').length === 0 && (
                          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
                            Tidak ada kategori pengeluaran yang tersedia.
                          </div>
                        )}
                      </div>
                    )}

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

      {/* Modal: Ubah Saldo Awal */}
      {showOpeningBalanceModal && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 440 }}>
            <div className="sf-modal-header">
              <h3>Ubah Saldo Awal RW</h3>
              <button
                type="button"
                className="sf-modal-close"
                onClick={() => !submittingOpeningBalance && setShowOpeningBalanceModal(false)}
                disabled={submittingOpeningBalance}
              >
                ✕
              </button>
            </div>
            <form onSubmit={handleSaveOpeningBalance}>
              <div className="sf-modal-body">
                {openingBalanceError && (
                  <div className="sf-error-banner" style={{ marginBottom: 12 }}>
                    {openingBalanceError}
                  </div>
                )}
                <div className="sf-form-group">
                  <label>Saldo Awal (Rp) *</label>
                  <input
                    type="number"
                    min="0"
                    step="1"
                    className="sf-form-input"
                    value={openingBalanceInput}
                    onChange={(e) => setOpeningBalanceInput(e.target.value)}
                    placeholder="0"
                    required
                    disabled={submittingOpeningBalance}
                  />
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4 }}>
                    Nilai saat ini: {formatRupiah(parseFloat(openingBalanceInput) || 0)}
                  </div>
                </div>
              </div>
              <div className="sf-modal-footer">
                <button
                  type="button"
                  className="sf-btn sf-btn-outline"
                  onClick={() => setShowOpeningBalanceModal(false)}
                  disabled={submittingOpeningBalance}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="sf-btn sf-btn-primary"
                  disabled={submittingOpeningBalance}
                >
                  {submittingOpeningBalance ? 'Menyimpan...' : 'Simpan Saldo Awal'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      {/* Modal Laporan KAS RW */}
      {reportModalOpen && (
        <div className="sf-modal-overlay" onClick={() => setReportModalOpen(false)}>
          <div
            className="sf-modal"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: 960, width: '95%', maxHeight: '90vh', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}
            role="dialog"
            aria-modal="true"
            aria-label="Laporan KAS RW"
          >
            <div className="sf-modal-header">
              <div className="sf-modal-header-copy">
                <div className="sf-modal-title">Laporan KAS RW</div>
                <div className="sf-modal-subtitle">
                  {modalSubtitle}
                </div>
              </div>
              <button
                type="button"
                className="sf-modal-close"
                onClick={() => setReportModalOpen(false)}
                aria-label="Tutup"
              >
                ×
              </button>
            </div>
            <div className="sf-modal-body" style={{ flex: 1, overflow: 'auto', padding: 0 }}>
              <RwKasReport
                rwId={rwId}
                rwName={displayRwLabel}
                period={reportPeriodProp}
                startDate={dateRange?.start ?? null}
                endDate={dateRange?.end ?? null}
                onClose={() => setReportModalOpen(false)}
                onNavigateBack={() => setReportModalOpen(false)}
              />
            </div>
          </div>
        </div>
      )}
    </>
  )
}
