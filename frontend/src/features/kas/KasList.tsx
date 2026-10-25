import { useState, useEffect, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { DatePicker } from 'antd'
import dayjs from 'dayjs'
import {
  BarChartOutlined,
  CheckCircleOutlined,
  WalletOutlined,
  PlusOutlined,
  EditOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
} from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { Pagination } from '@/components/Pagination'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import {
  getSessionPair,
  type ApiTransaction,
  type ApiFinancialCategory,
  apiListCategories,
  apiUpdateTransaction,
  apiUpdateOpeningBalance,
} from '@/app/api'
import { fetchPeriodBalance, fetchKasTransactions, type KasPeriodSummary } from './kas-api'
import { formatRupiah, formatTanggal } from './kasTypes'
import { KasReport } from './KasReport'

const MONTH_NAMES = [
  'Januari',
  'Februari',
  'Maret',
  'April',
  'Mei',
  'Juni',
  'Juli',
  'Agustus',
  'September',
  'Oktober',
  'November',
  'Desember',
]

function formatPeriodLabel(year: number, month: number): string {
  return `${MONTH_NAMES[month - 1]} ${year}`
}

export function cleanKasDescription(desc: string): string {
  if (!desc) return ''
  const voluntaryMatch = desc.match(/Voluntary contribution for due (.+?)\s*\(/i)
  if (voluntaryMatch) {
    return `Sumbangan Sukarela - ${voluntaryMatch[1].trim()}`
  }
  const paymentPeriodMatch = desc.match(/Payment for bill [a-f0-9-]+\s*\(Period:\s*([^)]+)\)/i)
  if (paymentPeriodMatch) {
    return `Pembayaran Iuran (${paymentPeriodMatch[1].trim()})`
  }
  return desc.replace(/\s*\(Bill:\s*[a-f0-9-]+\)/gi, '').replace(/bill\s+[a-f0-9-]{36}/gi, 'tagihan').trim()
}

interface TableRowItem {
  id: string
  rawTx?: ApiTransaction
  arus?: 'income' | 'expense'
  isIncome?: boolean
  tanggal: string
  keterangan: string
  dueName?: string | null
  citizenName?: string | null
  kategori: string
  metode: string
  pengurus: string
  referensi: string
  pemasukan: string
  pengeluaran: string
  saldo: string
  isSaldoAwal?: boolean
}

export function KasList() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [reportModalOpen, setReportModalOpen] = useState(false)

  const canViewKasReport = user?.systemRole === 'super_admin'
    || (user?.jabatan !== 'keamanan' && user?.jabatan !== 'kebersihan_pembangunan')

  // Month filter state — default to current month
  const [selectedMonth, setSelectedMonth] = useState<{ year: number; month: number }>(() => {
    const now = new Date()
    return { year: now.getFullYear(), month: now.getMonth() + 1 }
  })
  const [isAllPeriods, setIsAllPeriods] = useState<boolean>(false)

  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState<string>('')
  const [filterArus, setFilterArus] = useState<'all' | 'income' | 'expense'>('all')
  const [filterMetode, setFilterMetode] = useState<'all' | 'CASH' | 'TRANSFER'>('all')

  const [selectedTx, setSelectedTx] = useState<ApiTransaction | null>(null)

  // Edit opening balance modal state
  const [showEditOpening, setShowEditOpening] = useState(false)
  const [openingBalanceInput, setOpeningBalanceInput] = useState('')
  const [openingBalanceReason, setOpeningBalanceReason] = useState('')
  const [savingOpening, setSavingOpening] = useState(false)
  const [openingError, setOpeningError] = useState<string | null>(null)

  // Edit manual transaction modal state
  const [editingTx, setEditingTx] = useState<ApiTransaction | null>(null)
  const [categories, setCategories] = useState<ApiFinancialCategory[]>([])
  const [editCategoryId, setEditCategoryId] = useState('')
  const [editAmount, setEditAmount] = useState('')
  const [editType, setEditType] = useState<'income' | 'expense'>('income')
  const [editMethod, setEditMethod] = useState<'CASH' | 'TRANSFER'>('CASH')
  const [editDate, setEditDate] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [savingEdit, setSavingEdit] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)

  const [page, setPage] = useState<number>(1)
  const pageSize = 20

  const [summary, setSummary] = useState<KasPeriodSummary>({
    openingBalance: '0.00',
    totalIncome: '0.00',
    totalExpense: '0.00',
    netMovement: '0.00',
    closingBalance: '0.00',
    transactionCount: 0,
  })

  const [rawTransactions, setRawTransactions] = useState<ApiTransaction[]>([])
  const [totalCount, setTotalCount] = useState<number>(0)
  const [totalPages, setTotalPages] = useState<number>(1)

  const canWrite = user ? hasWargaWriteAccess(user) : false

  const activePeriodStr = useMemo(() => {
    if (isAllPeriods) return undefined
    return `${selectedMonth.year}-${String(selectedMonth.month).padStart(2, '0')}`
  }, [isAllPeriods, selectedMonth])

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
      const [sumData, txData] = await Promise.all([
        fetchPeriodBalance(token, activePeriodStr),
        fetchKasTransactions(token, {
          page,
          pageSize,
          period: activePeriodStr,
          status: 'posted',
        }),
      ])

      setSummary(sumData)
      setRawTransactions(txData.data || [])
      setTotalCount(txData.pagination?.total || 0)
      setTotalPages(Math.max(1, txData.pagination?.total_pages || 1))
    } catch (err: any) {
      setError(err?.message || 'Gagal memuat data kas')
    } finally {
      setLoading(false)
    }
  }, [activePeriodStr, page, pageSize])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Set initial values when opening edit modal
  useEffect(() => {
    if (editingTx) {
      setEditCategoryId(editingTx.category_id || '')
      const rawNum = Math.round(Number(editingTx.amount) || 0)
      setEditAmount(rawNum > 0 ? String(rawNum) : '')
      setEditType(editingTx.type)
      setEditMethod((editingTx.payment_method?.toUpperCase() === 'TRANSFER') ? 'TRANSFER' : 'CASH')
      const d = (editingTx.occurred_at || editingTx.created_at || '').split('T')[0]
      setEditDate(d || new Date().toISOString().split('T')[0])
      setEditDescription(editingTx.description || '')
      setEditError(null)

      const token = getSessionPair()?.accessToken
      if (token) {
        apiListCategories(token, true).then(setCategories).catch(() => {})
      }
    }
  }, [editingTx])

  const handleAllPeriods = () => {
    setIsAllPeriods((prev) => !prev)
    setPage(1)
  }

  const handleCreate = useCallback(() => {
    navigate('/kas/baru')
  }, [navigate])

  const handleSaveOpeningBalance = async (e: React.FormEvent) => {
    e.preventDefault()
    const targetPeriod = activePeriodStr || `${selectedMonth.year}-${String(selectedMonth.month).padStart(2, '0')}`
    const token = getSessionPair()?.accessToken
    if (!token) return

    setSavingOpening(true)
    setOpeningError(null)
    try {
      await apiUpdateOpeningBalance(token, targetPeriod, openingBalanceInput.trim(), openingBalanceReason.trim() || undefined)
      setShowEditOpening(false)
      loadData()
    } catch (err: any) {
      setOpeningError(err?.message || 'Gagal mengubah saldo awal')
    } finally {
      setSavingOpening(false)
    }
  }

  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingTx) return
    const token = getSessionPair()?.accessToken
    if (!token) return

    setSavingEdit(true)
    setEditError(null)
    try {
      await apiUpdateTransaction(token, editingTx.id, {
        category_id: editCategoryId || undefined,
        amount: editAmount.trim(),
        type: editType,
        payment_method: editMethod,
        occurred_at: editDate,
        description: editDescription.trim(),
      })
      setEditingTx(null)
      loadData()
    } catch (err: any) {
      setEditError(err?.message || 'Gagal mengubah transaksi')
    } finally {
      setSavingEdit(false)
    }
  }

  // Running balance calculation
  const tableRows: TableRowItem[] = useMemo(() => {
    let txs = [...rawTransactions]
    if (filterArus !== 'all') {
      txs = txs.filter((t) => t.type === filterArus)
    }
    if (filterMetode !== 'all') {
      txs = txs.filter((t) => ((t.payment_method || 'CASH').toUpperCase()) === filterMetode)
    }
    if (search.trim()) {
      const q = search.trim().toLowerCase()
      txs = txs.filter((t) =>
        t.description.toLowerCase().includes(q) ||
        (t.category_name && t.category_name.toLowerCase().includes(q)) ||
        (t.payment_no && t.payment_no.toLowerCase().includes(q)) ||
        (t.reference_no && t.reference_no.toLowerCase().includes(q)) ||
        (t.citizen_name && t.citizen_name.toLowerCase().includes(q)) ||
        (t.due_name && t.due_name.toLowerCase().includes(q))
      )
    }

    const rows: TableRowItem[] = []
    const isFiltered = Boolean(search.trim() || filterArus !== 'all' || filterMetode !== 'all')
    const openingNum = Number(summary.openingBalance) || 0

    if (page === 1 && !isFiltered) {
      rows.push({
        id: '__saldo_awal__',
        tanggal: !isAllPeriods
          ? `01 ${MONTH_NAMES[selectedMonth.month - 1]?.slice(0, 3)} ${selectedMonth.year}`
          : '—',
        keterangan: 'Saldo Awal',
        kategori: '—',
        metode: '—',
        pengurus: '—',
        referensi: '—',
        pemasukan: '—',
        pengeluaran: '—',
        saldo: formatRupiah(summary.openingBalance),
        isSaldoAwal: true,
      })
    }

    if (txs.length === 0) {
      return rows
    }

    const chronological = [...txs].sort((a, b) => {
      const dateA = new Date(a.occurred_at || a.created_at).getTime()
      const dateB = new Date(b.occurred_at || b.created_at).getTime()
      if (dateA !== dateB) return dateA - dateB
      // Deterministic tie-breaker by created_at — not by type (income/expense)
      // and not by random UUID id. Ensures IN and OUT with the same timestamp
      // have a stable order based on creation time.
      return (a.created_at || '').localeCompare(b.created_at || '')
    })

    let currentBalance = Math.round(openingNum * 100)
    const withBalance = chronological.map((t) => {
      const amtCents = Math.round((Number(t.amount) || 0) * 100)
      if (t.type === 'income') {
        currentBalance += amtCents
      } else {
        currentBalance -= amtCents
      }
      return {
        ...t,
        runningBalance: (currentBalance / 100).toFixed(2),
      }
    })

    const displayList = [...withBalance].reverse()

    displayList.forEach((t) => {
      const arus: 'income' | 'expense' | undefined =
        t.type === 'income' ? 'income' : t.type === 'expense' ? 'expense' : undefined
      const isIncome = arus === 'income'
      const isExpense = arus === 'expense'
      const rawMetode = (t.payment_method || 'CASH').toUpperCase()
      const metode = rawMetode === 'TRANSFER' ? 'Transfer' : 'Tunai'
      const resolvedRef = t.payment_no || t.reference_no || (t.payment_id ? `PAY-${t.payment_id.slice(0, 8)}` : (t.reverses_transaction_id ? 'Reversal' : '—'))
      rows.push({
        id: t.id,
        rawTx: t,
        arus,
        isIncome,
        tanggal: formatTanggal(t.occurred_at || t.created_at),
        keterangan: t.description,
        dueName: t.due_name,
        citizenName: t.citizen_name,
        kategori: t.category_name || '—',
        metode,
        pengurus: (isIncome && Boolean(t.payment_id || t.due_name || (t.payment_no && t.payment_no.startsWith('PAY-')))) ? 'Sistem' : (t.officer_name || '—'),
        referensi: resolvedRef,
        pemasukan: isIncome ? `+Rp ${formatRupiah(t.amount)}` : '—',
        pengeluaran: isExpense ? `-Rp ${formatRupiah(t.amount)}` : '—',
        saldo: `Rp ${formatRupiah(t.runningBalance)}`,
      })
    })

    return rows
  }, [rawTransactions, search, filterArus, filterMetode, summary.openingBalance, page, isAllPeriods])

  if (!user) return null

  const displayPeriodLabel = isAllPeriods
    ? 'Semua Periode'
    : formatPeriodLabel(selectedMonth.year, selectedMonth.month)

  const hasNoTransactions = !loading && rawTransactions.length === 0

  return (
    <>
      {/* PageHeader */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <WalletOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">KAS RT</div>
            <div className="sf-page-header-subtitle">
              Neraca kas dan catatan mutasi keuangan RT
            </div>
          </div>
        </div>
      </div>

      {/* Summary Cards: Saldo Awal, Pemasukan, Pengeluaran, Mutasi Bersih, Saldo Akhir */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
          gap: 12,
          marginTop: 8,
          marginBottom: 16,
        }}
      >
        {/* Card 1: Saldo Awal */}
        <div
          style={{
            background: 'rgba(100, 116, 139, 0.05)',
            borderRadius: 'var(--sf-radius-md)',
            border: '1px solid rgba(100, 116, 139, 0.25)',
            padding: '14px 16px',
            boxShadow: '0 0 10px rgba(100, 116, 139, 0.06)',
            position: 'relative',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
            <span style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600 }}>
              Saldo Awal
            </span>
            {canWrite && !isAllPeriods && (
              <button
                type="button"
                className="sf-btn sf-btn-ghost"
                onClick={() => {
                  setOpeningBalanceInput(summary.openingBalance)
                  setOpeningBalanceReason('Koreksi saldo awal')
                  setOpeningError(null)
                  setShowEditOpening(true)
                }}
                style={{ padding: '2px 8px', fontSize: 11, height: 'auto', color: 'var(--sf-accent)' }}
                title="Ubah Saldo Awal"
              >
                <EditOutlined style={{ marginRight: 3 }} /> Ubah
              </button>
            )}
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-text)' }}>
            Rp {formatRupiah(summary.openingBalance)}
          </div>
        </div>

        {/* Card 2: Total Pemasukan */}
        <div
          style={{
            background: 'color-mix(in srgb, var(--sf-success) 8%, var(--sf-bg))',
            borderRadius: 'var(--sf-radius-md)',
            border: '1px solid color-mix(in srgb, var(--sf-success) 35%, var(--sf-border))',
            padding: '14px 16px',
            boxShadow: '0 0 10px rgba(115, 191, 105, 0.08)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11, color: 'var(--sf-success)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            <ArrowDownOutlined style={{ fontSize: 11 }} /> Total Pemasukan
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-success)' }}>
            +Rp {formatRupiah(summary.totalIncome)}
          </div>
        </div>

        {/* Card 3: Total Pengeluaran */}
        <div
          style={{
            background: 'color-mix(in srgb, var(--sf-danger) 8%, var(--sf-bg))',
            borderRadius: 'var(--sf-radius-md)',
            border: '1px solid color-mix(in srgb, var(--sf-danger) 35%, var(--sf-border))',
            padding: '14px 16px',
            boxShadow: '0 0 10px rgba(242, 73, 92, 0.08)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11, color: 'var(--sf-danger)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            <ArrowUpOutlined style={{ fontSize: 11 }} /> Total Pengeluaran
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-danger)' }}>
            -Rp {formatRupiah(summary.totalExpense)}
          </div>
        </div>

        {/* Card 4: Mutasi Bersih */}
        <div
          style={{
            background: 'color-mix(in srgb, var(--sf-info) 8%, var(--sf-bg))',
            borderRadius: 'var(--sf-radius-md)',
            border: '1px solid color-mix(in srgb, var(--sf-info) 35%, var(--sf-border))',
            padding: '14px 16px',
            boxShadow: '0 0 10px rgba(87, 148, 242, 0.08)',
          }}
        >
          <div style={{ fontSize: 11, color: 'var(--sf-info)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Mutasi Bersih
          </div>
          <div
            style={{
              fontSize: 20,
              fontWeight: 700,
              color: 'var(--sf-info)',
            }}
          >
            {Number(summary.netMovement) >= 0 ? '+' : '-'}Rp {formatRupiah(Math.abs(Number(summary.netMovement) || 0))}
          </div>
        </div>

        {/* Card 5: Saldo Akhir */}
        <div
          style={{
            background: 'rgba(169, 112, 255, 0.06)',
            borderRadius: 'var(--sf-radius-md)',
            border: '1px solid rgba(169, 112, 255, 0.25)',
            padding: '14px 16px',
            boxShadow: '0 0 10px rgba(169, 112, 255, 0.06)',
          }}
        >
          <div style={{ fontSize: 11, color: 'var(--sf-accent)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Saldo Akhir
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-accent)' }}>
            Rp {formatRupiah(summary.closingBalance)}
          </div>
        </div>
      </div>

      {/* Toolbar & Filters */}
      <div className="sf-list-toolbar" style={{ marginTop: 0, marginBottom: 0 }}>
        {/* Row 1: Period Navigation & Filters (left), Period Action (right) */}
        <div className="sf-list-toolbar-left" style={{ flexWrap: 'wrap', gap: 8 }}>
          {/* Filter & Pencarian (default periode: bulan ini) */}

          {/* Filter Bulan */}
          <DatePicker
            picker="month"
            value={selectedMonth ? dayjs(`${selectedMonth.year}-${String(selectedMonth.month).padStart(2, '0')}-01`) : null}
            onChange={(date) => {
              if (date) {
                setSelectedMonth({ year: date.year(), month: date.month() + 1 })
                setPage(1)
              }
            }}
            allowClear={false}
            format="YYYY-MM"
            style={{ fontSize: 13, width: 140 }}
            aria-label="Filter Bulan"
          />

          {/* Filter Arus Kas */}
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

          {/* Filter Metode */}
          <select
            className="sf-form-select"
            value={filterMetode}
            onChange={(e) => setFilterMetode(e.target.value as any)}
            style={{ fontSize: 13, height: 34, padding: '4px 10px', width: 'auto', minWidth: 130 }}
            aria-label="Filter Metode Pembayaran"
          >
            <option value="all">Semua Metode</option>
            <option value="CASH">💵 Tunai</option>
            <option value="TRANSFER">🏦 Transfer</option>
          </select>

          {/* Search Box */}
          <div style={{ flex: 1, minWidth: 200, maxWidth: 420 }}>
            <SearchBox
              value={search}
              onChange={(v) => setSearch(v)}
              placeholder="Cari keterangan, kategori, ref, warga..."
            />
          </div>
        </div>

        {/* Right: Laporan KAS button + Semua Periode button + Active Period Badge */}
        <div className="sf-list-toolbar-right" style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          {canViewKasReport && (
            <button
              type="button"
              className="sf-btn sf-btn-outline"
              onClick={() => setReportModalOpen(true)}
              aria-label="Laporan KAS"
            >
              <BarChartOutlined style={{ fontSize: 12, marginRight: 6 }} />
              Laporan KAS
            </button>
          )}

          <button
            type="button"
            className={`sf-btn sf-kas-period-button ${isAllPeriods ? 'sf-btn-primary' : 'sf-btn-outline'}`}
            onClick={handleAllPeriods}
            aria-pressed={isAllPeriods}
            style={{
              padding: '8px 20px',
              fontSize: 13,
              fontWeight: 600,
              lineHeight: '1.2',
              borderRadius: 'var(--sf-radius-sm)',
              transition: 'all 0.25s ease',
              boxSizing: 'border-box',
            }}
          >
            {isAllPeriods && (
              <CheckCircleOutlined style={{ fontSize: 11, marginRight: 5 }} />
            )}
            Semua Periode
          </button>

          {canWrite && (
            <button className="sf-create-btn" onClick={handleCreate} aria-label="+ Tambah Transaksi">
              <PlusOutlined style={{ marginRight: 0, fontSize: 12 }} />
            </button>
          )}
        </div>
      </div>

      {/* Main Table Card */}
      <div className="sf-card">
        {error && (
          <div className="sf-error-banner" role="alert">
            {error}
          </div>
        )}

        {loading && (
          <div className="sf-loading-banner">
            Memuat data neraca kas...
          </div>
        )}

        {hasNoTransactions && !error && (
          <div className="sf-empty-banner">
            Belum ada transaksi pada periode ini.
          </div>
        )}

        {/* Neraca KAS Table (Without NO / # column) */}
        {!loading && (
          <div className="table-wrapper">
            <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th style={{ width: 110, minWidth: 110, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    TANGGAL
                  </th>
                  <th style={{ minWidth: 220, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    KETERANGAN
                  </th>
                  <th style={{ width: 140, minWidth: 140, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    KATEGORI
                  </th>
                  <th style={{ width: 130, minWidth: 130, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    REFERENSI
                  </th>
                  <th style={{ width: 130, minWidth: 130, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    PENCATAT
                  </th>
                  <th style={{ width: 90, minWidth: 90, textAlign: 'center', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    METODE
                  </th>
                  <th style={{ width: 120, minWidth: 120, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    ARUS
                  </th>
                  <th className="sf-kas-col-income" style={{ width: 130, minWidth: 130, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'color-mix(in srgb, #34c759 85%, var(--sf-text))', borderBottom: '1px solid var(--sf-border)', background: 'var(--sf-kas-income-header-bg)' }}>
                    PEMASUKAN (+)
                  </th>
                  <th className="sf-kas-col-expense" style={{ width: 130, minWidth: 130, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'color-mix(in srgb, #ff3b30 85%, var(--sf-text))', borderBottom: '1px solid var(--sf-border)', background: 'var(--sf-kas-expense-header-bg)' }}>
                    PENGELUARAN (-)
                  </th>
                  <th style={{ width: 140, minWidth: 140, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    SALDO
                  </th>
                </tr>
              </thead>
              <tbody>
                {tableRows.map((r) => {
                  if (r.isSaldoAwal) {
                    return (
                      <tr key={r.id} style={{ background: 'rgba(100, 116, 139, 0.04)', fontWeight: 600 }}>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', color: 'var(--sf-text-muted)', fontSize: 13 }}>
                          {r.tanggal}
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', color: 'var(--sf-text)', fontSize: 13 }}>
                          <Badge variant="blue">{r.keterangan}</Badge>
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text-muted)' }}>
                          —
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', color: 'var(--sf-text-muted)' }}>
                          —
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', color: 'var(--sf-text-muted)' }}>
                          —
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                          —
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                          —
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', color: 'var(--sf-text-muted)' }}>
                          —
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', color: 'var(--sf-text-muted)' }}>
                          —
                        </td>
                        <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', fontSize: 13, color: 'var(--sf-text)', fontWeight: 700 }}>
                          {r.saldo}
                        </td>
                      </tr>
                    )
                  }

                  return (
                    <tr
                      key={r.id}
                      onClick={() => r.rawTx && setSelectedTx(r.rawTx)}
                      style={{ cursor: 'pointer' }}
                      title="Klik untuk melihat detail transaksi"
                    >
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)' }}>
                        {r.tanggal}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                          {r.dueName ? (
                            <>
                              <Badge variant="blue">{r.dueName}</Badge>
                              <span style={{ fontWeight: 500, color: 'var(--sf-text)' }}>Warga</span>
                            </>
                          ) : (
                            <span style={{ maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'inline-block' }}>
                              {cleanKasDescription(r.keterangan)}
                            </span>
                          )}
                        </div>
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', whiteSpace: 'nowrap' }}>
                        {r.kategori}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 12, fontFamily: 'monospace', color: 'var(--sf-text-muted)' }}>
                        {r.referensi}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text-muted)', whiteSpace: 'nowrap' }}>
                        {r.pengurus}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'center', whiteSpace: 'nowrap' }}>
                        <Badge variant={r.metode === 'Transfer' ? 'violet' : 'blue'}>
                          {r.metode}
                        </Badge>
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)' }}>
                        <Badge
                          variant={r.isIncome ? 'green' : 'red'}
                          style={{ height: 20, padding: '0 6px', fontSize: 10, fontWeight: 700, display: 'inline-flex', alignItems: 'center' }}
                        >
                          {r.isIncome ? '↓ Masuk' : '↑ Keluar'}
                        </Badge>
                      </td>
                      <td
                        className={r.arus === 'income' ? 'sf-kas-cell-income' : undefined}
                        style={{
                          padding: '10px 12px',
                          borderBottom: '1px solid var(--sf-border)',
                          textAlign: 'right',
                          fontFamily: 'monospace',
                          fontSize: 13,
                          color: r.arus === 'income' ? 'color-mix(in srgb, #34c759 85%, var(--sf-text))' : 'var(--sf-text-muted)',
                          fontWeight: r.arus === 'income' ? 600 : 400,
                          background: r.arus === 'income' ? 'var(--sf-kas-income-bg)' : 'transparent',
                        }}
                      >
                        {r.pemasukan}
                      </td>
                      <td
                        className={r.arus === 'expense' ? 'sf-kas-cell-expense' : undefined}
                        style={{
                          padding: '10px 12px',
                          borderBottom: '1px solid var(--sf-border)',
                          textAlign: 'right',
                          fontFamily: 'monospace',
                          fontSize: 13,
                          color: r.arus === 'expense' ? 'color-mix(in srgb, #ff3b30 85%, var(--sf-text))' : 'var(--sf-text-muted)',
                          fontWeight: r.arus === 'expense' ? 600 : 400,
                          background: r.arus === 'expense' ? 'var(--sf-kas-expense-bg)' : 'transparent',
                        }}
                      >
                        {r.pengeluaran}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', fontSize: 13, color: 'var(--sf-text)', fontWeight: 600 }}>
                        {r.saldo}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && (
          <Pagination
            page={page}
            pageSize={pageSize}
            total={totalCount}
            totalPages={totalPages}
            onPageChange={(p) => setPage(p)}
          />
        )}
      </div>

      {/* Detail Transaksi Modal */}
      {selectedTx && (
        <div className="sf-modal-overlay" onClick={() => setSelectedTx(null)}>
          <div
            className="sf-modal"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: 520, width: '92%' }}
            role="dialog"
            aria-modal="true"
          >
            <div className="sf-modal-header">
              <div className="sf-modal-header-copy">
                <div className="sf-modal-title">Detail Transaksi Kas</div>
                <div className="sf-modal-subtitle">
                  {formatTanggal(selectedTx.occurred_at || selectedTx.created_at)}
                </div>
              </div>
              <button
                type="button"
                className="sf-modal-close"
                onClick={() => setSelectedTx(null)}
                aria-label="Tutup"
              >
                ×
              </button>
            </div>

            <div className="sf-modal-body" style={{ padding: 20 }}>
              {/* Type and Amount Card */}
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 16,
                  padding: '12px 16px',
                  background: 'var(--sf-bg)',
                  borderRadius: 8,
                  border: '1px solid var(--sf-border)',
                }}
              >
                <div>
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
                    Nominal
                  </div>
                  <div
                    style={{
                      fontSize: 22,
                      fontWeight: 700,
                      fontFamily: 'monospace',
                      color: selectedTx.type === 'income' ? 'var(--sf-success)' : 'var(--sf-danger)',
                    }}
                  >
                    {selectedTx.type === 'income' ? '+' : '-'}Rp {formatRupiah(selectedTx.amount)}
                  </div>
                </div>
                <Badge
                  variant={selectedTx.type === 'income' ? 'green' : 'red'}
                  style={{ height: 28, padding: '0 12px', fontSize: 13, fontWeight: 600 }}
                >
                  {selectedTx.type === 'income' ? 'Pemasukan' : 'Pengeluaran'}
                </Badge>
              </div>

              {/* Detail fields grid */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 14 }}>
                <div>
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
                    Kategori
                  </div>
                  <div style={{ fontSize: 14, color: 'var(--sf-text)', fontWeight: 500 }}>
                    {selectedTx.category_name || '—'}
                  </div>
                </div>

                <div>
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
                    Metode Pembayaran
                  </div>
                  <div style={{ fontSize: 14, color: 'var(--sf-text)' }}>
                    <Badge variant={(selectedTx.payment_method || 'CASH').toUpperCase() === 'TRANSFER' ? 'violet' : 'blue'}>
                      {(selectedTx.payment_method || 'CASH').toUpperCase() === 'TRANSFER' ? 'Transfer' : 'Tunai'}
                    </Badge>
                  </div>
                </div>

                <div>
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
                    Pencatat
                  </div>
                  <div style={{ fontSize: 14, color: 'var(--sf-text)', fontWeight: 500 }}>
                    {selectedTx.type === 'income' && (selectedTx.payment_id || selectedTx.due_name || (selectedTx.payment_no && selectedTx.payment_no.startsWith('PAY-')))
                      ? 'Sistem'
                      : (selectedTx.officer_name || '—')}
                  </div>
                </div>

                <div>
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
                    Referensi
                  </div>
                  <div style={{ fontSize: 13, fontFamily: 'monospace', color: 'var(--sf-text-muted)' }}>
                    {selectedTx.payment_no || selectedTx.reference_no || (selectedTx.payment_id ? `PAY-${selectedTx.payment_id.slice(0, 8)}` : (selectedTx.reverses_transaction_id ? 'Reversal' : '—'))}
                  </div>
                </div>

                <div style={{ gridColumn: '1 / -1' }}>
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
                    Keterangan
                  </div>
                  {selectedTx.due_name ? (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                        <Badge variant="blue">{selectedTx.due_name}</Badge>
                        <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--sf-text)' }}>
                          Warga
                        </span>
                      </div>
                      {selectedTx.description && (
                        <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 2 }}>
                          {cleanKasDescription(selectedTx.description)}
                        </div>
                      )}
                    </div>
                  ) : (
                    <div style={{ fontSize: 14, color: 'var(--sf-text)' }}>
                      {cleanKasDescription(selectedTx.description)}
                    </div>
                  )}
                </div>
              </div>
            </div>

            <div
              className="sf-modal-footer"
              style={{
                display: 'flex',
                justifyContent: 'flex-end',
                alignItems: 'center',
                gap: 10,
                padding: '12px 20px',
              }}
            >
              {canWrite && !selectedTx.payment_id && !selectedTx.reverses_transaction_id && selectedTx.status === 'posted' && (
                <button
                  type="button"
                  className="sf-btn"
                  style={{
                    padding: '7px 16px',
                    fontSize: 13,
                    fontWeight: 600,
                    background: 'rgba(169, 112, 255, 0.16)',
                    border: '1px solid rgba(169, 112, 255, 0.6)',
                    color: 'var(--sf-accent)',
                    boxShadow: '0 0 10px rgba(169, 112, 255, 0.15)',
                    borderRadius: 'var(--sf-radius-md)',
                    transition: 'all 0.2s ease',
                  }}
                  onClick={() => {
                    setEditingTx(selectedTx)
                    setSelectedTx(null)
                  }}
                >
                  Ubah Transaksi
                </button>
              )}
              <button
                type="button"
                className="sf-btn"
                style={{
                  padding: '7px 16px',
                  fontSize: 13,
                  fontWeight: 500,
                  background: 'transparent',
                  border: '1px solid var(--sf-border)',
                  color: 'var(--sf-text-muted)',
                  borderRadius: 'var(--sf-radius-md)',
                }}
                onClick={() => setSelectedTx(null)}
              >
                Tutup
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal Ubah Saldo Awal */}
      {showEditOpening && (
        <div className="sf-modal-overlay" onClick={() => setShowEditOpening(false)}>
          <div
            className="sf-modal"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: 440, width: '92%' }}
            role="dialog"
            aria-modal="true"
          >
            <div className="sf-modal-header">
              <div className="sf-modal-header-copy">
                <div className="sf-modal-title">Ubah Saldo Awal</div>
                <div className="sf-modal-subtitle">
                  Periode {displayPeriodLabel}
                </div>
              </div>
              <button
                type="button"
                className="sf-modal-close"
                onClick={() => setShowEditOpening(false)}
                aria-label="Tutup"
              >
                ×
              </button>
            </div>

            <form onSubmit={handleSaveOpeningBalance}>
              <div className="sf-modal-body" style={{ padding: 20 }}>
                {openingError && (
                  <div className="sf-error-banner" style={{ marginBottom: 14 }}>
                    {openingError}
                  </div>
                )}

                <div className="sf-form-field">
                  <label className="sf-form-label" htmlFor="input-opening-balance">
                    Nominal Saldo Awal (Rp) <span style={{ color: 'var(--sf-danger)' }}>*</span>
                  </label>
                  <input
                    id="input-opening-balance"
                    type="number"
                    step="any"
                    className="sf-form-input"
                    value={openingBalanceInput}
                    onChange={(e) => setOpeningBalanceInput(e.target.value)}
                    required
                    placeholder="Contoh: 15000000"
                    autoFocus
                  />
                  <span style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 4, display: 'block' }}>
                    Nilai saldo awal yang menjadi basis perhitungan neraca berjalan periode ini.
                  </span>
                </div>

                <div className="sf-form-field" style={{ marginTop: 14 }}>
                  <label className="sf-form-label" htmlFor="input-opening-reason">
                    Alasan Perubahan
                  </label>
                  <input
                    id="input-opening-reason"
                    type="text"
                    className="sf-form-input"
                    value={openingBalanceReason}
                    onChange={(e) => setOpeningBalanceReason(e.target.value)}
                    placeholder="Contoh: Koreksi pembukuan awal tahun"
                  />
                </div>
              </div>

              <div
                className="sf-modal-footer"
                style={{
                  display: 'flex',
                  justifyContent: 'flex-end',
                  gap: 10,
                  padding: '12px 20px',
                }}
              >
                <button
                  type="button"
                  className="sf-btn"
                  onClick={() => setShowEditOpening(false)}
                  disabled={savingOpening}
                  style={{
                    background: 'transparent',
                    border: '1px solid var(--sf-border)',
                    color: 'var(--sf-text-muted)',
                    padding: '7px 16px',
                    borderRadius: 'var(--sf-radius-md)',
                    fontSize: 13,
                    fontWeight: 500,
                  }}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="sf-btn"
                  disabled={savingOpening}
                  style={{
                    background: 'rgba(87, 148, 242, 0.16)',
                    border: '1px solid rgba(87, 148, 242, 0.6)',
                    color: 'var(--sf-info)',
                    boxShadow: '0 0 12px rgba(87, 148, 242, 0.15)',
                    fontWeight: 600,
                    padding: '7px 18px',
                    borderRadius: 'var(--sf-radius-md)',
                    fontSize: 13,
                    transition: 'all 0.2s ease',
                  }}
                >
                  {savingOpening ? 'Menyimpan...' : 'Simpan Saldo Awal'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Ubah Transaksi Manual */}
      {editingTx && (
        <div className="sf-modal-overlay" onClick={() => setEditingTx(null)}>
          <div
            className="sf-modal"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: 520, width: '92%' }}
            role="dialog"
            aria-modal="true"
          >
            <div className="sf-modal-header">
              <div className="sf-modal-header-copy">
                <div className="sf-modal-title">Ubah Transaksi Kas</div>
                <div className="sf-modal-subtitle">
                  Perbarui rincian transaksi manual
                </div>
              </div>
              <button
                type="button"
                className="sf-modal-close"
                onClick={() => setEditingTx(null)}
                aria-label="Tutup"
              >
                ×
              </button>
            </div>

            <form onSubmit={handleSaveEdit}>
              <div className="sf-modal-body" style={{ padding: 20 }}>
                {editError && (
                  <div className="sf-error-banner" style={{ marginBottom: 14 }}>
                    {editError}
                  </div>
                )}

                {/* Arus Kas Toggle */}
                <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
                  <button
                    type="button"
                    className="sf-btn"
                    onClick={() => setEditType('income')}
                    style={{
                      flex: 1,
                      height: 38,
                      minHeight: 38,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      background: editType === 'income' ? 'rgba(115, 191, 105, 0.14)' : 'transparent',
                      color: editType === 'income' ? 'var(--sf-success)' : 'var(--sf-text-muted)',
                      border: editType === 'income' ? '1px solid rgba(115, 191, 105, 0.5)' : '1px solid var(--sf-border)',
                      boxShadow: editType === 'income' ? '0 0 10px rgba(115, 191, 105, 0.15)' : 'none',
                      fontWeight: 600,
                      fontSize: 13,
                      transition: 'all 0.2s ease',
                    }}
                  >
                    Pemasukan
                  </button>
                  <button
                    type="button"
                    className="sf-btn"
                    onClick={() => setEditType('expense')}
                    style={{
                      flex: 1,
                      height: 38,
                      minHeight: 38,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      background: editType === 'expense' ? 'rgba(242, 73, 92, 0.14)' : 'transparent',
                      color: editType === 'expense' ? 'var(--sf-danger)' : 'var(--sf-text-muted)',
                      border: editType === 'expense' ? '1px solid rgba(242, 73, 92, 0.5)' : '1px solid var(--sf-border)',
                      boxShadow: editType === 'expense' ? '0 0 10px rgba(242, 73, 92, 0.15)' : 'none',
                      fontWeight: 600,
                      fontSize: 13,
                      transition: 'all 0.2s ease',
                    }}
                  >
                    Pengeluaran
                  </button>
                </div>

                {/* Tanggal */}
                <div className="sf-form-field">
                  <label className="sf-form-label" htmlFor="edit-tanggal">
                    Tanggal <span style={{ color: 'var(--sf-danger)' }}>*</span>
                  </label>
                  <DatePicker
                    id="edit-tanggal"
                    picker="date"
                    value={editDate ? dayjs(editDate) : null}
                    onChange={(date) => setEditDate(date?.format('YYYY-MM-DD') || '')}
                    allowClear={false}
                    placeholder="DD/MM/YYYY"
                    format="DD/MM/YYYY"
                    disabled={savingEdit}
                    style={{ width: '100%' }}
                  />
                </div>

                {/* Metode Pembayaran */}
                <div className="sf-form-field" style={{ marginTop: 14 }}>
                  <label className="sf-form-label">
                    Metode Pembayaran <span style={{ color: 'var(--sf-danger)' }}>*</span>
                  </label>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <button
                      type="button"
                      className="sf-btn"
                      onClick={() => setEditMethod('CASH')}
                      style={{
                        flex: 1,
                        height: 38,
                        minHeight: 38,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: 13,
                        fontWeight: 600,
                        background: editMethod === 'CASH' ? 'rgba(169, 112, 255, 0.12)' : 'var(--sf-surface)',
                        color: editMethod === 'CASH' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                        border: editMethod === 'CASH' ? '1px solid rgba(169, 112, 255, 0.5)' : '1px solid var(--sf-border)',
                        boxShadow: editMethod === 'CASH' ? '0 0 8px rgba(169, 112, 255, 0.15)' : 'none',
                        transition: 'all 0.2s ease',
                      }}
                    >
                      💵 Tunai (Cash)
                    </button>
                    <button
                      type="button"
                      className="sf-btn"
                      onClick={() => setEditMethod('TRANSFER')}
                      style={{
                        flex: 1,
                        height: 38,
                        minHeight: 38,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: 13,
                        fontWeight: 600,
                        background: editMethod === 'TRANSFER' ? 'rgba(169, 112, 255, 0.12)' : 'var(--sf-surface)',
                        color: editMethod === 'TRANSFER' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                        border: editMethod === 'TRANSFER' ? '1px solid rgba(169, 112, 255, 0.5)' : '1px solid var(--sf-border)',
                        boxShadow: editMethod === 'TRANSFER' ? '0 0 8px rgba(169, 112, 255, 0.15)' : 'none',
                        transition: 'all 0.2s ease',
                      }}
                    >
                      🏦 Transfer Bank
                    </button>
                  </div>
                </div>

                {/* Kategori */}
                <div className="sf-form-field" style={{ marginTop: 14 }}>
                  <label className="sf-form-label" htmlFor="edit-kategori">
                    Kategori
                  </label>
                  <select
                    id="edit-kategori"
                    className="sf-form-select"
                    value={editCategoryId}
                    onChange={(e) => setEditCategoryId(e.target.value)}
                  >
                    <option value="">Pilih Kategori</option>
                    {categories
                      .filter((c) => c.type === editType)
                      .map((c) => (
                        <option key={c.id} value={c.id}>
                          {c.name}
                        </option>
                      ))}
                  </select>
                </div>

                {/* Nominal */}
                <div className="sf-form-field" style={{ marginTop: 14 }}>
                  <label className="sf-form-label" htmlFor="edit-nominal">
                    Nominal (Rp) <span style={{ color: 'var(--sf-danger)' }}>*</span>
                  </label>
                  <input
                    id="edit-nominal"
                    type="number"
                    className="sf-form-input"
                    value={editAmount}
                    onChange={(e) => setEditAmount(e.target.value)}
                    placeholder="Contoh: 500000"
                    required
                    min="1"
                  />
                </div>

                {/* Keterangan */}
                <div className="sf-form-field" style={{ marginTop: 14 }}>
                  <label className="sf-form-label" htmlFor="edit-keterangan">
                    Keterangan <span style={{ color: 'var(--sf-danger)' }}>*</span>
                  </label>
                  <textarea
                    id="edit-keterangan"
                    className="sf-form-input"
                    rows={2}
                    value={editDescription}
                    onChange={(e) => setEditDescription(e.target.value)}
                    placeholder="Deskripsi transaksi..."
                    required
                  />
                </div>
              </div>

              <div
                className="sf-modal-footer"
                style={{
                  display: 'flex',
                  justifyContent: 'flex-end',
                  gap: 10,
                  padding: '12px 20px',
                }}
              >
                <button
                  type="button"
                  className="sf-btn"
                  onClick={() => setEditingTx(null)}
                  disabled={savingEdit}
                  style={{
                    background: 'transparent',
                    border: '1px solid var(--sf-border)',
                    color: 'var(--sf-text-muted)',
                    padding: '7px 16px',
                    borderRadius: 'var(--sf-radius-md)',
                    fontSize: 13,
                    fontWeight: 500,
                  }}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="sf-btn"
                  disabled={savingEdit}
                  style={{
                    background: 'rgba(169, 112, 255, 0.16)',
                    border: '1px solid rgba(169, 112, 255, 0.6)',
                    color: 'var(--sf-accent)',
                    boxShadow: '0 0 12px rgba(169, 112, 255, 0.15)',
                    fontWeight: 600,
                    padding: '7px 18px',
                    borderRadius: 'var(--sf-radius-md)',
                    fontSize: 13,
                    transition: 'all 0.2s ease',
                  }}
                >
                  {savingEdit ? 'Menyimpan...' : 'Simpan Perubahan'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      {/* Modal Laporan KAS */}
      {reportModalOpen && (
        <div className="sf-modal-overlay" onClick={() => setReportModalOpen(false)}>
          <div
            className="sf-modal"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: 960, width: '95%', maxHeight: '90vh', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}
            role="dialog"
            aria-modal="true"
            aria-label="Laporan KAS"
          >
            <div className="sf-modal-header">
              <div className="sf-modal-header-copy">
                <div className="sf-modal-title">Laporan KAS</div>
                <div className="sf-modal-subtitle">
                  {isAllPeriods
                    ? `Semua Periode`
                    : `${MONTH_NAMES[selectedMonth.month - 1]?.slice(0, 3)} ${selectedMonth.year}`}
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
              <KasReport onClose={() => setReportModalOpen(false)} onNavigateBack={() => setReportModalOpen(false)} period={activePeriodStr} />
            </div>
          </div>
        </div>
      )}
    </>
  )
}
