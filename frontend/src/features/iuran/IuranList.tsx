import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  DollarOutlined,
  CloseOutlined,
  DownOutlined,
  AlertOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { useAuth } from '@/app/AuthContext'
import {
  BACKEND_STATUS_MAP,
  formatRupiah,
  formatPeriodeLabel,
  getStatusVariant,
  getStatusLabel,
  IURAN_TYPE_OPTIONS,
  parseMoney,
} from './iuranTypes'
import { DueCreate } from './DueCreate'
import { IuranDetail } from './IuranDetail'
import { IuranPayment } from './IuranPayment'
import type { IuranBill, IuranStatus } from './iuranTypes'
import { apiListBills, apiListPayments, apiListDues, apiUpdateDue, apiDeactivateDue, apiTriggerWasteTruckArrival, type ApiBill, type ApiDue, getSessionPair } from '@/app/api'

type FilterType = 'semua' | 'status' | 'periode' | 'jenis'

const STATUS_OPTIONS: { value: IuranStatus; label: string }[] = [
  { value: 'belum_bayar', label: 'Belum Bayar' },
  { value: 'sebagian', label: 'Sebagian' },
  { value: 'lunas', label: 'Lunas' },
  { value: 'dibatalkan', label: 'Dibatalkan' },
  { value: 'sukarela', label: 'Sukarela' },
]

const statusFilterOptions: { value: FilterType; label: string }[] = [
  { value: 'semua', label: 'Semua' },
  { value: 'status', label: 'Status' },
  { value: 'periode', label: 'Periode' },
  { value: 'jenis', label: 'Jenis Iuran' },
]

const colStyles = {
  no: { width: 40, minWidth: 40, maxWidth: 40, textAlign: 'center' as const },
  name: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  rt: { width: 70, minWidth: 70, maxWidth: 70, textAlign: 'center' as const },
  type: { minWidth: 130, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  periode: { width: 90, minWidth: 90, maxWidth: 90, textAlign: 'center' as const },
  nominal: { width: 120, minWidth: 120, maxWidth: 120, textAlign: 'right' as const, whiteSpace: 'nowrap' as const, fontFamily: 'monospace' },
  paid: { width: 120, minWidth: 120, maxWidth: 120, textAlign: 'right' as const, whiteSpace: 'nowrap' as const, fontFamily: 'monospace' },
  pending: { width: 120, minWidth: 120, maxWidth: 120, textAlign: 'right' as const, whiteSpace: 'nowrap' as const, fontFamily: 'monospace' },
  status: { width: 120, minWidth: 120, maxWidth: 120, textAlign: 'center' as const },
  action: { width: 60, minWidth: 60, maxWidth: 60, textAlign: 'center' as const },
}

export function IuranList() {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [allBills, setAllBills] = useState<IuranBill[]>([])

  // Create Due modal state
  const [showCreateDue, setShowCreateDue] = useState(false)

  // Detail modal state
  const [showDetailBillId, setShowDetailBillId] = useState<string | null>(null)

  // Payment modal state
  const [showPaymentBillId, setShowPaymentBillId] = useState<string | null>(null)

  // URL search params (must be read before any code that uses searchParams)
  const [searchParams, setSearchParams] = useSearchParams()

  // Auth context (must be before any code that uses `user`)
  const { user } = useAuth()

  // Tab state: 'list-iuran' = tagihan RT, 'tagihan' = tagihan pribadi pengurus, 'kelola-iuran' = konfigurasi iuran
  type ActiveTab = 'list-iuran' | 'tagihan' | 'kelola-iuran'
  const initialTab = (() => {
    const raw = searchParams.get('tab')
    if (raw === 'kelola-iuran') return 'kelola-iuran' as const
    if (raw === 'tagihan') return 'tagihan' as const
    return 'list-iuran' as const
  })()

  // ═══════════════════════════════════════════════════
  // Iuran Tab Access Control
  // ═══════════════════════════════════════════════════

  function canAccessIuranTabs(u: {
    systemRole?: string | null
    role?: string
    jabatan?: string | null
  }): boolean {
    if (u.systemRole === 'super_admin') return true
    if (typeof u.jabatan === 'string' && u.jabatan.length > 0) {
      const accessJabatans = new Set(['ketua', 'wakil_ketua', 'sekretaris', 'bendahara'])
      return accessJabatans.has(u.jabatan)
    }
    switch (u.role) {
      case 'pengurus':
      case 'super_admin':
        return true
      default:
        return false
    }
  }

  const hasIuranAccess = user ? canAccessIuranTabs(user) : false

  // Force default tab for users without Iuran access
  const effectiveInitialTab: ActiveTab = hasIuranAccess
    ? initialTab
    : 'list-iuran'
  const [activeTab, setActiveTab] = useState<ActiveTab>(effectiveInitialTab)

  // Dues data for "Kelola Iuran" tab
  const [dues, setDues] = useState<ApiDue[]>([])
  const [duesLoading, setDuesLoading] = useState(false)
  const [duesError, setDuesError] = useState<string | null>(null)

  // Edit Due modal state
  const [editDueId, setEditDueId] = useState<string | null>(null)
  const [editDueForm, setEditDueForm] = useState<{ name: string; amount: string; period_type: 'monthly' | 'yearly' | 'one_time'; payment_rule: 'full' | 'partial'; is_voluntary: boolean } | null>(null)
  const [editDueLoading, setEditDueLoading] = useState(false)

  // Deactivate due state
  const [deactivateDueId, setDeactivateDueId] = useState<string | null>(null)
  const [deactivateLoading, setDeactivateLoading] = useState(false)

  const initialSearch = searchParams.get('q') || searchParams.get('search') || ''
  const initialFilter = (searchParams.get('filter') as FilterType) ?? 'semua'
  const initialStatus = (searchParams.get('status') as IuranStatus) || null
  const initialPeriode = searchParams.get('periode') ?? ''
  const initialJenis = searchParams.get('jenis') ?? ''
  const initialPage = parseInt(searchParams.get('page') || '1', 10) || 1
  const initialPageSize = parseInt(searchParams.get('page_size') || searchParams.get('per_page') || '', 10) || undefined

  const [filterType, setFilterType] = useState<FilterType>(initialFilter)
  const [search, setSearch] = useState(initialSearch)
  const [debouncedSearch, setDebouncedSearch] = useState(initialSearch)
  const [statusValue, setStatusValue] = useState<IuranStatus | null>(initialStatus)
  const [periodeValue, setPeriodeValue] = useState<string>(initialPeriode)
  const [jenisValue, setJenisValue] = useState<string>(initialJenis)

  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:iuran', [10, 25, 50, 100], initialPageSize)

  // Debounce search input by 300ms to prevent duplicate in-flight requests
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(search)
    }, 300)
    return () => clearTimeout(timer)
  }, [search])

  // Sync activeTab + filter state to URL for persistence across refreshes
  useEffect(() => {
    const params = new URLSearchParams()
    if (activeTab === 'kelola-iuran') params.set('tab', 'kelola-iuran')
    else if (activeTab === 'tagihan') params.set('tab', 'tagihan')
    if (debouncedSearch.trim()) params.set('q', debouncedSearch.trim())
    if (filterType !== 'semua') params.set('filter', filterType)
    if (statusValue) params.set('status', statusValue)
    if (periodeValue) params.set('periode', periodeValue)
    if (jenisValue) params.set('jenis', jenisValue)
    if (page > 1) params.set('page', String(page))
    if (pageSize) params.set('page_size', String(pageSize))

    setSearchParams(params, { replace: true })
  }, [searchParams, debouncedSearch, filterType, statusValue, periodeValue, jenisValue, page, pageSize, activeTab, setSearchParams])

  // Notification state
  const [wasteTruckStatus, setWasteTruckStatus] = useState<string | null>(null)
  const [wasteTruckLoading, setWasteTruckLoading] = useState(false)

  const canTriggerWasteTruck = user?.role === 'pengurus' || user?.systemRole === 'super_admin'

  const handleWasteTruckArrival = async () => {
    const token = getSessionPair()?.accessToken
    if (!token) return
    setWasteTruckLoading(true)
    setWasteTruckStatus(null)
    try {
      const res = await apiTriggerWasteTruckArrival(token)
      setWasteTruckStatus(res.message || 'Notifikasi kedatangan truk sampah berhasil dikirim')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Gagal mengirim notifikasi'
      setWasteTruckStatus(msg)
    } finally {
      setWasteTruckLoading(false)
    }
  }

  const fetchedPaymentBillIds = useRef<Set<string>>(new Set())
  const requestIdRef = useRef(0)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)

  // Map backend bill to frontend IuranBill format
  const mapBill = useCallback((b: ApiBill): IuranBill => {
    const iuranType = b.due_name ?? 'Iuran'
    const householdName = b.head_name ?? b.house_number ?? 'Unknown'
    const rt = b.rt_code ? b.rt_code.replace(/^RT\s*/i, '') : '—'
    const nominal = parseMoney(b.amount)
    const isVoluntary = Boolean(b.is_voluntary || nominal === 0)
    const status: IuranStatus = (isVoluntary && nominal === 0)
      ? 'sukarela'
      : (BACKEND_STATUS_MAP[b.status] ?? 'belum_bayar')

    return {
      id: b.id,
      householdId: b.household_occupancy_id,
      householdName,
      houseNumber: b.house_number ?? undefined,
      rt,
      iuranType,
      periode: b.period,
      nominal,
      paidAmount: 0,
      pendingAmount: 0,
      status,
      isVoluntary,
      dueId: b.due_id,
      payments: [],
    }
  }, [])

  const loadData = useCallback(async () => {
    const currentReq = ++requestIdRef.current
    setLoading(true)
    setError(null)
    fetchedPaymentBillIds.current.clear()

    try {
      const token = getSessionPair()?.accessToken
      if (!token) {
        setError('Sesi Anda telah berakhir. Silakan login ulang.')
        setLoading(false)
        return
      }

      let apiStatus: string | undefined
      if (statusValue === 'lunas') apiStatus = 'paid'
      else if (statusValue === 'belum_bayar') apiStatus = 'unpaid'
      else if (statusValue === 'dibatalkan') apiStatus = 'cancelled'
      else if (statusValue === 'sebagian') apiStatus = 'partial'

      const resp = await apiListBills(token, {
        page,
        page_size: pageSize,
        search: debouncedSearch.trim() || undefined,
        status: apiStatus,
        period: periodeValue || undefined,
        due_name: jenisValue || undefined,
        my_bills: activeTab === 'tagihan' ? true : undefined,
      })

      if (currentReq !== requestIdRef.current) {
        return
      }

      if (!resp?.data) {
        throw new Error('Gagal memuat data tagihan')
      }

      const bills = resp.data.map(mapBill)
      setAllBills(bills)

      const serverTotal = resp.pagination?.total ?? bills.length
      const serverTotalPages = resp.pagination?.total_pages ?? Math.max(1, Math.ceil(serverTotal / pageSize))
      setTotal(serverTotal)
      setTotalPages(serverTotalPages)

      if (serverTotalPages > 0 && page > serverTotalPages) {
        setPage(serverTotalPages)
      }
    } catch (err) {
      if (currentReq === requestIdRef.current) {
        setAllBills([])
        setError(err instanceof Error ? err.message : 'Gagal memuat data')
      }
    } finally {
      if (currentReq === requestIdRef.current) {
        setLoading(false)
      }
    }
  }, [page, pageSize, debouncedSearch, statusValue, periodeValue, jenisValue, activeTab, mapBill])

  useEffect(() => {
    void loadData()
  }, [loadData])

  // Auto-refresh when user returns to tab
  useEffect(() => {
    function handleVisibilityChange() {
      if (document.visibilityState === 'visible') {
        void loadData()
      }
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [loadData])

  // Load dues for "Kelola Iuran" tab
  const loadDues = useCallback(async () => {
    const currentReq = ++requestIdRef.current
    setDuesLoading(true)
    setDuesError(null)
    try {
      const token = getSessionPair()?.accessToken
      if (!token) {
        setDuesError('Sesi Anda telah berakhir. Silakan login ulang.')
        setDuesLoading(false)
        return
      }
      const resp = await apiListDues(token, { is_active: true })
      if (currentReq !== requestIdRef.current) return
      if (!resp || !Array.isArray(resp) || resp.length === 0) {
        if (resp && resp.length === 0) {
          setDues([])
          setDuesLoading(false)
          return
        }
        throw new Error('Gagal memuat konfigurasi iuran')
      }
      setDues(resp)
    } catch (err) {
      if (currentReq === requestIdRef.current) {
        setDues([])
        setDuesError(err instanceof Error ? err.message : 'Gagal memuat data')
      }
    } finally {
      if (currentReq === requestIdRef.current) {
        setDuesLoading(false)
      }
    }
  }, [])

  // Load dues when "Kelola Iuran" tab is activated
  useEffect(() => {
    if (activeTab === 'kelola-iuran') {
      void loadDues()
    }
  }, [activeTab, loadDues])

  // Client-side filter for Kelola Iuran: search by name, filter by period_type
  // (Due model lacks server-side search/filter — backend API only supports { is_active })
  const filteredDues = useMemo(() => {
    if (dues.length === 0) return dues
    return dues.filter((due) => {
      // Search: match against due name (case-insensitive)
      if (debouncedSearch.trim()) {
        const q = debouncedSearch.toLowerCase()
        if (!due.name.toLowerCase().includes(q)) return false
      }
      // Filter by period_type (FREKUENSI)
      if (filterType === 'jenis' && jenisValue) {
        const expectedType = jenisValue === 'bulanan' ? 'monthly'
          : jenisValue === 'tahunan' ? 'yearly'
          : 'one_time'
        if (due.period_type !== expectedType) return false
      }
      return true
    })
  }, [dues, debouncedSearch, filterType, jenisValue])

  // No client-side filtering needed — backend handles status/periode/jenis/search.
  // filteredBills = allBills which already comes from server with applied filters.
  const filteredBills = useMemo(() => allBills, [allBills])

  const displayTotal = total > 0 ? total : allBills.length
  const displayTotalPages = totalPages > 0 ? totalPages : Math.max(1, Math.ceil(displayTotal / pageSize))
  const paginatedBills = useMemo(() => {
    if (allBills.length > pageSize) {
      return allBills.slice((page - 1) * pageSize, page * pageSize)
    }
    return allBills
  }, [allBills, page, pageSize])

  // Load payments for visible bills on active page to calculate paidAmount & pendingAmount
  useEffect(() => {
    let cancelled = false
    const token = getSessionPair()?.accessToken
    if (!token || paginatedBills.length === 0) return

    const unfetched = paginatedBills.filter((b) => !fetchedPaymentBillIds.current.has(b.id))
    if (unfetched.length === 0) return

    unfetched.forEach((b) => fetchedPaymentBillIds.current.add(b.id))

    Promise.all(
      unfetched.map(async (b) => {
        const res = await apiListPayments(token, { bill_id: b.id }).catch(() => null)
        return { billId: b.id, data: res }
      })
    ).then((results) => {
      if (cancelled) return
      setAllBills((prev) => {
        const paymentMap = new Map<string, { paid: number; pending: number }>()
        for (const { billId, data } of results) {
          if (data && Array.isArray(data.data) && data.data.length > 0) {
            const approved = data.data.filter((p: { status: string }) => p.status === 'APPROVED')
            const paid = approved.reduce((sum: number, p: { amount: string }) => sum + parseMoney(p.amount), 0)
            const pending = data.data.filter((p: { status: string }) => p.status === 'PENDING')
            const pendingAmt = pending.reduce((sum: number, p: { amount: string }) => sum + parseMoney(p.amount), 0)
            paymentMap.set(billId, { paid, pending: pendingAmt })
          }
        }
        if (paymentMap.size === 0) return prev
        return prev.map((b) => {
          const pm = paymentMap.get(b.id)
          if (pm) {
            return { ...b, paidAmount: pm.paid, pendingAmount: pm.pending }
          }
          return b
        })
      })
    })

    return () => {
      cancelled = true
    }
  }, [paginatedBills])

  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    if (filter === 'semua') {
      setStatusValue(null)
      setPeriodeValue('')
      setJenisValue('')
    }
  }

  const handleSearchChange = (value: string) => {
    setSearch(value)
    setPage(1)
  }

  const handleSearch = (value: string) => {
    setSearch(value)
    setDebouncedSearch(value)
    setPage(1)
  }

  const handlePage = (p: number) => {
    if (p >= 1 && p <= displayTotalPages) {
      setPage(p)
    }
  }

  const handlePageSizeChange = (newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }

  const handleStatusSelect = (val: IuranStatus) => {
    setStatusValue(val)
    setPage(1)
  }

  const handlePeriodeSelect = (val: string) => {
    setPeriodeValue(val)
    setPage(1)
  }

  const handleJenisSelect = (val: string) => {
    setJenisValue(val)
    setPage(1)
  }

  const statusLabel: string | null = statusValue ? getStatusLabel(statusValue) : null
  const periodeLabel: string | null = periodeValue ? formatPeriodeLabel(periodeValue) : null
  const jenisLabel: string | null = jenisValue
    ? IURAN_TYPE_OPTIONS.find((o) => o.value === jenisValue)?.label ?? null
    : null

  const uniquePeriodes = useMemo(() => {
    const s = new Set<string>()
    allBills.forEach((b) => {
      if (b.periode) s.add(b.periode)
    })
    return Array.from(s).sort().reverse()
  }, [allBills])

  const uniquePeriodesOptions = useMemo(
    () => uniquePeriodes.map((p) => ({ value: p, label: formatPeriodeLabel(p) })),
    [uniquePeriodes]
  )

  const handleRowClick = useCallback((billId: string) => {
    setShowDetailBillId(billId)
  }, [])

  // ═══════════════════════════════════════════════════
  // Edit Due handlers
  // ═══════════════════════════════════════════════════
  const handleOpenEdit = useCallback((due: ApiDue) => {
    setEditDueId(due.id)
    setEditDueForm({
      name: due.name,
      amount: due.amount,
      period_type: due.period_type,
      payment_rule: due.payment_rule ?? 'full',
      is_voluntary: due.is_voluntary ?? false,
    })
  }, [])

  const handleCloseEdit = useCallback(() => {
    setEditDueId(null)
    setEditDueForm(null)
    setEditDueLoading(false)
  }, [])

  const handleSaveEdit = useCallback(async () => {
    if (!editDueId || !editDueForm) return
    const token = getSessionPair()?.accessToken
    if (!token) return
    setEditDueLoading(true)
    try {
      await apiUpdateDue(token, editDueId, editDueForm)
      handleCloseEdit()
      void loadDues()
    } catch (err) {
      // keep modal open — error shown in duesError
      setDuesError(err instanceof Error ? err.message : 'Gagal menyimpan perubahan')
    } finally {
      setEditDueLoading(false)
    }
  }, [editDueId, editDueForm, loadDues, handleCloseEdit])

  // ═══════════════════════════════════════════════════
  // Deactivate due handlers
  // ═══════════════════════════════════════════════════
  const handleConfirmDeactivate = useCallback(async () => {
    const token = getSessionPair()?.accessToken
    if (!token || !deactivateDueId) return
    setDeactivateLoading(true)
    try {
      await apiDeactivateDue(token, deactivateDueId)
      setDeactivateDueId(null)
      void loadDues()
    } catch (err) {
      setDuesError(err instanceof Error ? err.message : 'Gagal menonaktifkan iuran')
    } finally {
      setDeactivateLoading(false)
    }
  }, [deactivateDueId, loadDues])

  const handleCloseDeactivate = useCallback(() => {
    setDeactivateDueId(null)
    setDeactivateLoading(false)
  }, [])

  return (
    <>
      {/* PageHeader — separate floating surface */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <DollarOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Iuran</div>
            <div className="sf-page-header-subtitle">
              Manajemen tagihan dan pembayaran iuran warga
            </div>
          </div>
        </div>
      </div>

      {/* Content surface */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            {/* Filter type dropdown */}
            <FilterDropdown
              value={filterType}
              options={statusFilterOptions}
              onChange={handleFilter}
              ariaLabel="Tipe filter"
            />

            {/* Search */}
            <SearchBox
              value={search}
              onChange={handleSearchChange}
              onSearch={handleSearch}
              placeholder="Cari nama tagihan..."
              statusControl={
                filterType === 'status' ? (
                  <div className="sf-status-selector">
                    {statusLabel ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {statusLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={() => { setStatusValue(null); setPage(1); }}
                          aria-label="Hapus filter status"
                          title="Hapus filter status"
                        >
                          <CloseOutlined style={{ fontSize: 10 }} />
                        </button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button
                          type="button"
                          className="sf-status-selector-trigger"
                          onClick={() => {}}
                          aria-haspopup="listbox"
                          aria-expanded={false}
                        >
                          Pilih Status
                          <DownOutlined style={{ fontSize: 9, color: 'var(--sf-text-muted)' }} />
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {STATUS_OPTIONS.map((opt) => (
                            <button
                              key={opt.value}
                              type="button"
                              className={`sf-status-dropdown-item ${statusValue === opt.value ? 'sf-status-dropdown-item-active' : ''}`}
                              onClick={() => handleStatusSelect(opt.value)}
                              role="button"
                            >
                              {opt.label}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : filterType === 'periode' ? (
                  <div className="sf-status-selector">
                    {periodeLabel ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {periodeLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={() => { setPeriodeValue(''); setPage(1); }}
                          aria-label="Hapus filter periode"
                          title="Hapus filter periode"
                        >
                          <CloseOutlined style={{ fontSize: 10 }} />
                        </button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button
                          type="button"
                          className="sf-status-selector-trigger"
                          onClick={() => {}}
                          aria-haspopup="listbox"
                          aria-expanded={false}
                        >
                          Pilih Periode
                          <DownOutlined style={{ fontSize: 9, color: 'var(--sf-text-muted)' }} />
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {uniquePeriodesOptions.map((opt) => (
                            <button
                              key={opt.value}
                              type="button"
                              className={`sf-status-dropdown-item ${periodeValue === opt.value ? 'sf-status-dropdown-item-active' : ''}`}
                              onClick={() => handlePeriodeSelect(opt.value)}
                              role="button"
                            >
                              {opt.label}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : filterType === 'jenis' ? (
                  <div className="sf-status-selector">
                    {jenisLabel ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {jenisLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={() => { setJenisValue(''); setPage(1); }}
                          aria-label="Hapus filter jenis"
                          title="Hapus filter jenis"
                        >
                          <CloseOutlined style={{ fontSize: 10 }} />
                        </button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button
                          type="button"
                          className="sf-status-selector-trigger"
                          onClick={() => {}}
                          aria-haspopup="listbox"
                          aria-expanded={false}
                        >
                          Pilih Jenis
                          <DownOutlined style={{ fontSize: 9, color: 'var(--sf-text-muted)' }} />
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {IURAN_TYPE_OPTIONS.map((opt) => (
                            <button
                              key={opt.value}
                              type="button"
                              className={`sf-status-dropdown-item ${jenisValue === opt.value ? 'sf-status-dropdown-item-active' : ''}`}
                              onClick={() => handleJenisSelect(opt.value)}
                              role="button"
                            >
                              {opt.label}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : null
              }
            />

            {/* No filter — Kelola Iuran shows all active dues */}
          </div>

          {/* Create Due and Waste Truck buttons */}
          <div className="sf-list-toolbar-right" style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            {/* "+ Iuran" only shown in "Kelola Iuran" tab */}
            {activeTab === 'kelola-iuran' && (
              <button
                type="button"
                className="sf-create-btn"
                onClick={() => setShowCreateDue(true)}
                aria-label="+ Iuran"
              >
                <PlusOutlined style={{ marginRight: 0, fontSize: 12 }} />
              </button>
            )}
            {/* "Truk Sampah Tiba" always visible in "List Iuran" tab */}
            {activeTab === 'list-iuran' && canTriggerWasteTruck && (
              <button
                type="button"
                className="sf-btn-secondary"
                disabled={wasteTruckLoading}
                onClick={() => void handleWasteTruckArrival()}
                title="Kirim notifikasi kedatangan truk sampah ke seluruh warga RT"
              >
                <AlertOutlined style={{ marginRight: 4, fontSize: 12 }} />
                {wasteTruckLoading ? 'Mengirim...' : 'Truk Sampah Tiba'}
              </button>
            )}
          </div>
        </div>

        {/* Tab Navigation */}
        {hasIuranAccess && (
          <div style={{ display: 'flex', gap: 0, borderBottom: '1px solid var(--sf-border)', marginBottom: '16px' }}>
            <button
              type="button"
              onClick={() => { setActiveTab('list-iuran'); setPage(1); }}
              style={{
                padding: '10px 20px',
                fontSize: '14px',
                fontWeight: activeTab === 'list-iuran' ? 600 : 400,
                color: activeTab === 'list-iuran' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                borderBottom: activeTab === 'list-iuran' ? '2px solid var(--sf-accent)' : '2px solid transparent',
                background: 'none',
                borderLeft: 'none',
                borderRight: 'none',
                borderTop: 'none',
                cursor: 'pointer',
                transition: 'all 150ms ease',
              }}
            >
              List Iuran
            </button>
            <button
              type="button"
              onClick={() => { setActiveTab('tagihan'); setPage(1); }}
              style={{
                padding: '10px 20px',
                fontSize: '14px',
                fontWeight: activeTab === 'tagihan' ? 600 : 400,
                color: activeTab === 'tagihan' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                borderBottom: activeTab === 'tagihan' ? '2px solid var(--sf-accent)' : '2px solid transparent',
                background: 'none',
                borderLeft: 'none',
                borderRight: 'none',
                borderTop: 'none',
                cursor: 'pointer',
                transition: 'all 150ms ease',
              }}
            >
              Tagihan
            </button>
            <button
              type="button"
              onClick={() => { setActiveTab('kelola-iuran'); setPage(1); }}
              style={{
                padding: '10px 20px',
                fontSize: '14px',
                fontWeight: activeTab === 'kelola-iuran' ? 600 : 400,
                color: activeTab === 'kelola-iuran' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                borderBottom: activeTab === 'kelola-iuran' ? '2px solid var(--sf-accent)' : '2px solid transparent',
                background: 'none',
                borderLeft: 'none',
                borderRight: 'none',
                borderTop: 'none',
                cursor: 'pointer',
                transition: 'all 150ms ease',
              }}
            >
              Kelola Iuran
            </button>
          </div>
        )}

        {wasteTruckStatus && activeTab === 'list-iuran' && (
          <div
            className="sf-alert-banner"
            style={{
              margin: '8px 0',
              padding: '8px 12px',
              borderRadius: '6px',
              background: 'var(--sf-success-light, #e6f7ed)',
              color: 'var(--sf-success, #1a7f37)',
              fontSize: '13px',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}
          >
            <span>{wasteTruckStatus}</span>
            <button
              type="button"
              onClick={() => setWasteTruckStatus(null)}
              style={{ border: 'none', background: 'none', cursor: 'pointer', color: 'inherit', fontWeight: 'bold' }}
              aria-label="Tutup notifikasi"
            >
              ✕
            </button>
          </div>
        )}

        {/* Create Due Modal */}
        {showCreateDue && (
          <DueCreate
            onClose={() => setShowCreateDue(false)}
            onSaved={() => { void loadData(); void loadDues(); }}
          />
        )}

        {/* Detail Modal */}
        {showDetailBillId && (
          <IuranDetail
            billId={showDetailBillId}
            open
            onClose={() => setShowDetailBillId(null)}
            onStatusChange={loadData}
            onPayment={(billId) => {
              setShowDetailBillId(null)
              setShowPaymentBillId(billId)
            }}
          />
        )}

        {/* Payment Modal */}
        {showPaymentBillId && (
          <IuranPayment
            billId={showPaymentBillId}
            open={true}
            onClose={() => setShowPaymentBillId(null)}
            onPaymentSuccess={() => { void loadData(); void loadDues(); }}
          />
        )}

        {/* Edit Due Modal */}
        {editDueId && editDueForm && (
          <div className="sf-modal-overlay" role="dialog" aria-modal="true">
            <div className="sf-modal" style={{ width: 480, maxWidth: 'calc(100vw - 32px)' }} onClick={(e) => e.stopPropagation()}>
              <ModalHeader
                title="Edit Iuran"
                subtitle="Perubahan akan berlaku untuk tagihan baru yang dibuat setelah ini."
                onClose={handleCloseEdit}
              />
              <ModalBody>
                <div className="sf-form-field">
                  <label className="sf-form-label" htmlFor="edit-due-name">Nama Iuran</label>
                  <input
                    id="edit-due-name"
                    className="sf-form-input"
                    type="text"
                    value={editDueForm.name}
                    onChange={(e) => setEditDueForm({ ...editDueForm, name: e.target.value })}
                    placeholder="Nama iuran"
                  />
                </div>
                <div className="sf-form-field">
                  <label className="sf-form-label" htmlFor="edit-due-amount">Nominal</label>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <input
                      id="edit-due-amount"
                      className="sf-form-input"
                      type="text"
                      value={editDueForm.amount}
                      onChange={(e) => {
                        const raw = e.target.value.replace(/\D/g, '')
                        setEditDueForm({ ...editDueForm, amount: raw || '0' })
                      }}
                      disabled={editDueForm.is_voluntary}
                      placeholder="0"
                    />
                    <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={editDueForm.is_voluntary}
                        onChange={(e) => setEditDueForm({ ...editDueForm, is_voluntary: e.target.checked })}
                        style={{ accentColor: 'var(--sf-accent)' }}
                      />
                      Sukarela (Rp0)
                    </label>
                  </div>
                </div>
                <div className="sf-form-field">
                  <label className="sf-form-label" htmlFor="edit-due-period">Frekuensi</label>
                  <select
                    id="edit-due-period"
                    className="sf-form-select"
                    value={editDueForm.period_type}
                    onChange={(e) => setEditDueForm({ ...editDueForm, period_type: e.target.value as 'monthly' | 'yearly' | 'one_time' })}
                  >
                    <option value="monthly">Bulanan</option>
                    <option value="yearly">Tahunan</option>
                    <option value="one_time">Sekali Waktu</option>
                  </select>
                </div>
                <div className="sf-form-field">
                  <label className="sf-form-label" htmlFor="edit-due-rule">Aturan Pembayaran</label>
                  <select
                    id="edit-due-rule"
                    className="sf-form-select"
                    value={editDueForm.payment_rule}
                    onChange={(e) => setEditDueForm({ ...editDueForm, payment_rule: e.target.value as 'full' | 'partial' })}
                  >
                    <option value="full">Seluruh</option>
                    <option value="partial">Sebagian</option>
                  </select>
                </div>
              </ModalBody>
              <ModalFooter>
                <button
                  type="button"
                  className="sf-btn-secondary"
                  onClick={handleCloseEdit}
                  disabled={editDueLoading}
                  style={{ marginRight: 'auto' }}
                >
                  Batal
                </button>
                <button
                  type="button"
                  className="sf-btn-primary"
                  onClick={handleSaveEdit}
                  disabled={editDueLoading || !editDueForm.name.trim() || !editDueForm.amount}
                >
                  {editDueLoading ? 'Menyimpan...' : 'Simpan'}
                </button>
              </ModalFooter>
            </div>
          </div>
        )}

        {/* Deactivate Confirmation */}
        {deactivateDueId && (
          <div className="sf-modal-overlay" role="dialog" aria-modal="true">
            <div className="sf-modal" style={{ width: 400, maxWidth: 'calc(100vw - 32px)' }} onClick={(e) => e.stopPropagation()}>
              <ModalHeader
                title="Konfirmasi"
                onClose={handleCloseDeactivate}
              />
              <ModalBody>
                <p style={{ fontSize: '14px', lineHeight: 1.6 }}>
                  Apakah Anda yakin ingin menonaktifkan iuran ini? Iuran tidak akan membuat tagihan baru setelah ditonaktifkan.
                </p>
              </ModalBody>
              <ModalFooter>
                <button
                  type="button"
                  className="sf-btn-secondary"
                  onClick={handleCloseDeactivate}
                  disabled={deactivateLoading}
                  style={{ marginRight: 'auto' }}
                >
                  Batal
                </button>
                <button
                  type="button"
                  className="sf-btn-danger"
                  onClick={handleConfirmDeactivate}
                  disabled={deactivateLoading}
                >
                  {deactivateLoading ? 'Menonaktifkan...' : 'Ya, Nonaktifkan'}
                </button>
              </ModalFooter>
            </div>
          </div>
        )}

        {/* ═══ TAB: LIST IURAN & TAGIHAN ═══ */}
        {(activeTab === 'list-iuran' || activeTab === 'tagihan') && (
          <>
            {/* Error */}
            {error && (
              <div className="sf-error-banner" role="alert">
                {error}
              </div>
            )}

            {/* Loading */}
            {loading && (
              <div className="sf-loading-banner">
                Memuat data iuran...
              </div>
            )}

            {/* Empty */}
            {!loading && !error && filteredBills.length === 0 && (
              <div className="sf-empty-banner">
                {search.trim() ? 'Tidak ada data iuran.' : 'Belum ada tagihan.'}
              </div>
            )}

            {/* Bill Table */}
            {!loading && filteredBills.length > 0 && (
              <div className="table-wrapper">
                <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
                  <thead>
                    <tr>
                      <th style={colStyles.no}>No</th>
                      <th style={colStyles.name}>WARGA/KK</th>
                      <th style={colStyles.rt}>RT</th>
                      <th style={colStyles.type}>JENIS IURAN</th>
                      <th style={colStyles.periode}>PERIODE</th>
                      <th style={colStyles.nominal}>NOMINAL</th>
                      <th style={colStyles.paid}>DIBAYAR</th>
                      <th style={colStyles.pending}>TERTUNDA</th>
                      <th style={colStyles.status}>STATUS</th>
                    </tr>
                  </thead>
                  <tbody>
                    {paginatedBills.map((bill, idx) => {
                      const hasPending = (bill.pendingAmount ?? 0) > 0

                      return (
                        <tr
                          key={bill.id}
                          onClick={() => handleRowClick(bill.id)}
                          style={{ cursor: 'pointer' }}
                          role="button"
                          tabIndex={0}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                              e.preventDefault()
                              handleRowClick(bill.id)
                            }
                          }}
                        >
                          <td style={colStyles.no}>
                            {(page - 1) * pageSize + idx + 1}
                          </td>
                          <td style={colStyles.name}>
                            <span style={{ fontWeight: 600 }}>{bill.householdName}</span>
                          </td>
                          <td style={colStyles.rt}>
                            {bill.rt}
                          </td>
                          <td style={colStyles.type}>
                            {bill.iuranType}
                          </td>
                          <td style={colStyles.periode}>
                            {bill.periode.includes('-') ? formatPeriodeLabel(bill.periode) : bill.periode}
                          </td>
                          <td style={colStyles.nominal}>
                            {formatRupiah(bill.nominal)}
                          </td>
                          <td style={colStyles.paid}>
                            <span style={{ color: (bill.paidAmount ?? 0) > 0 ? 'var(--sf-success)' : undefined, fontWeight: (bill.paidAmount ?? 0) > 0 ? 600 : 400 }}>
                              {formatRupiah(bill.paidAmount)}
                            </span>
                          </td>
                          <td style={colStyles.pending}>
                            {hasPending ? (
                              <span style={{ color: 'var(--sf-warning)', fontWeight: 600 }}>
                                {formatRupiah(bill.pendingAmount)}
                              </span>
                            ) : (
                              '—'
                            )}
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
            )}

            {/* Pagination */}
            {!loading && filteredBills.length > 0 && (
              <Pagination
                page={page}
                pageSize={pageSize}
                total={displayTotal}
                totalPages={displayTotalPages}
                onPageChange={handlePage}
                onPageSizeChange={handlePageSizeChange}
                pageSizeOptions={[10, 25, 50, 100]}
              />
            )}
          </>
        )}

        {/* ═══ TAB: KELOLA IURAN ═══ */}
        {activeTab === 'kelola-iuran' && (
          <>
            {/* Loading */}
            {duesLoading && (
              <div className="sf-loading-banner">
                Memuat konfigurasi iuran...
              </div>
            )}

            {/* Error */}
            {duesError && (
              <div className="sf-error-banner" role="alert">
                {duesError}
              </div>
            )}

            {/* Empty */}
            {!duesLoading && !duesError && filteredDues.length === 0 && (
              <div className="sf-empty-banner">
                Belum ada konfigurasi iuran. Klik "Tambah Iuran" untuk membuat.
              </div>
            )}

            {/* Dues Table — row click opens Edit Due modal */}
            {!duesLoading && filteredDues.length > 0 && (
              <div className="table-wrapper">
                <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
                  <thead>
                    <tr>
                      <th style={{ ...colStyles.name, minWidth: 200, textAlign: 'left' }}>NAMA IURAN</th>
                      <th style={{ width: 130, minWidth: 130, textAlign: 'right', fontFamily: 'monospace' }}>NOMINAL</th>
                      <th style={{ width: 130, minWidth: 130, textAlign: 'left' }}>FREKUENSI</th>
                      <th style={{ width: 130, minWidth: 130, textAlign: 'left' }}>ATURAN</th>
                      <th style={{ width: 110, minWidth: 110, textAlign: 'center' }}>TIPE</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredDues.map((due) => {
                      const isVoluntary = due.is_voluntary ?? false
                      const periodLabel = due.period_type === 'monthly' ? 'Bulanan'
                        : due.period_type === 'yearly' ? 'Tahunan'
                        : 'Sekali Waktu'
                      const ruleLabel = (due.payment_rule ?? 'full') === 'full' ? 'Seluruh' : 'Sebagian'

                      return (
                        <tr
                          key={due.id}
                          onClick={() => handleOpenEdit(due)}
                          style={{ cursor: 'pointer' }}
                          role="button"
                          tabIndex={0}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                              e.preventDefault()
                              handleOpenEdit(due)
                            }
                          }}
                        >
                          <td style={colStyles.name}>
                            <span style={{ fontWeight: 600 }}>{due.name}</span>
                          </td>
                          <td style={colStyles.nominal}>
                            {isVoluntary ? (
                              <span style={{ color: 'var(--sf-info)' }}>Sukarela</span>
                            ) : (
                              formatRupiah(parseMoney(due.amount))
                            )}
                          </td>
                          <td style={{ textAlign: 'left' }}>
                            {periodLabel}
                          </td>
                          <td style={{ textAlign: 'left' }}>
                            {ruleLabel}
                          </td>
                          <td style={{ textAlign: 'center' }}>
                            {isVoluntary ? (
                              <Badge variant="blue">Sukarela</Badge>
                            ) : (
                              <span style={{
                                display: 'inline-flex',
                                alignItems: 'center',
                                padding: '0 8px',
                                fontSize: '11px',
                                fontWeight: 400,
                                borderRadius: '5px',
                                border: '1px solid rgba(16, 185, 129, 0.30)',
                                background: 'rgba(16, 185, 129, 0.08)',
                                color: '#059669',
                                lineHeight: 1.2,
                                height: '22px',
                                boxSizing: 'border-box',
                                boxShadow: '0 0 8px rgba(16, 185, 129, 0.08)',
                                fontFamily: 'inherit',
                              }}>
                                Reguler
                              </span>
                            )}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}
