import { useState, useEffect, useCallback, useMemo } from 'react'
import {
  DollarOutlined,
  PlusOutlined,
  CloseOutlined,
  DownOutlined,
} from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { RowActionMenu } from '@/components/RowActionMenu'
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
import { apiListBills, apiListPayments, type ApiBill, type ApiPayment, getSessionPair } from '@/app/api'
import { useAuth } from '@/app/AuthContext'

type FilterType = 'semua' | 'status' | 'periode' | 'jenis'

const STATUS_OPTIONS: { value: IuranStatus; label: string }[] = [
  { value: 'belum_bayar', label: 'Belum Bayar' },
  { value: 'sebagian', label: 'Sebagian' },
  { value: 'lunas', label: 'Lunas' },
  { value: 'dibatalkan', label: 'Dibatalkan' },
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
  status: { width: 120, minWidth: 120, maxWidth: 120, textAlign: 'center' as const },
  action: { width: 60, minWidth: 60, maxWidth: 60, textAlign: 'center' as const },
}

export function IuranList() {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [backendBills, setBackendBills] = useState<ApiBill[]>([])

  // Create Due modal state
  const [showCreateDue, setShowCreateDue] = useState(false)

  // Detail modal state
  const [showDetailBillId, setShowDetailBillId] = useState<string | null>(null)

  // Payment modal state
  const [showPaymentBillId, setShowPaymentBillId] = useState<string | null>(null)

  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [search, setSearch] = useState('')
  const [statusValue, setStatusValue] = useState<IuranStatus | null>(null)
  const [periodeValue, setPeriodeValue] = useState<string>('')
  const [jenisValue, setJenisValue] = useState<string>('')

  const [filteredBills, setFilteredBills] = useState<IuranBill[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  // Auth for role-based access control
  const { user } = useAuth()
  const canVerify = useMemo(() => {
    if (!user) return false
    return user.role === 'pengurus' || user.role === 'bendahara' || user.role === 'super_admin'
  }, [user])

  // Map backend bill to frontend IuranBill format
  const mapBill = useCallback((b: ApiBill): IuranBill => {
    const iuranType = b.due_name ?? 'Iuran'
    const householdName = b.head_name ?? b.house_number ?? 'Unknown'
    const rt = b.rt_code ? `RT ${b.rt_code}` : 'RT —'
    return {
      id: b.id,
      householdId: b.household_occupancy_id,
      householdName,
      rt,
      iuranType,
      periode: b.period,
      nominal: parseMoney(b.amount),
      paidAmount: 0, // calculated separately from payments
      pendingAmount: 0,
      status: BACKEND_STATUS_MAP[b.status] ?? 'belum_bayar',
      paidDate: undefined,
      payments: [], // populated in IuranDetail
    }
  }, [])

  const loadData = useCallback(async (p: number, size: number) => {
    setLoading(true)
    setError(null)

    try {
      const token = getSessionPair()?.accessToken
      if (!token) {
        setError('Sesi Anda telah berakhir. Silakan login ulang.')
        setLoading(false)
        return
      }

      const params: Record<string, string> = {
        page: String(p),
        page_size: String(size),
      }

      // Apply server-side filters
      if (statusValue) {
        // Map frontend status back to backend status
        const statusMap: Record<string, string> = {
          belum_bayar: 'unpaid',
          sebagian: 'partial',
          lunas: 'paid',
          dibatalkan: 'cancelled',
        }
        params.status = statusMap[statusValue] ?? ''
      }
      if (periodeValue) {
        params.period = periodeValue
      }

      const response = await apiListBills(token, params)

      const data = response?.data ?? []
      let bills = data.map(mapBill)

      // Load payments for all visible bills to calculate paidAmount
      // Only load for first page to avoid excessive API calls
      if (p === 1 && bills.length > 0) {
        try {
          // Load payments for each bill to calculate paidAmount AND pending
          if (token) {
            const paymentsPromises = bills.map(async (b: IuranBill) => {
              const res = await apiListPayments(token, { bill_id: b.id }).catch(() => null)
              return { billId: b.id, data: res }
            })
            const paymentsResults = await Promise.all(paymentsPromises)

            paymentsResults.forEach(({ billId: _billId, data }, idx: number) => {
              if (data && data.data.length > 0) {
                const approvedPayments = data.data.filter(
                  (p: { status: string; amount: string }) => p.status === 'APPROVED'
                )
                const paidAmount = approvedPayments.reduce(
                  (sum: number, p: { amount: string }) => sum + parseMoney(p.amount),
                  0
                )
                const pendingPayments = data.data.filter(
                  (p: { status: string; amount: string }) => p.status === 'PENDING'
                )
                const pendingAmount = pendingPayments.reduce(
                  (sum: number, p: { amount: string }) => sum + parseMoney(p.amount),
                  0
                )

                bills[idx] = { ...bills[idx], paidAmount, pendingAmount }
              }
            })
          }
        } catch {
          // Silent fail - show paidAmount=0 if payments cannot be loaded
        }
      }

      // Client-side search filter (backend doesn't support search)
      if (search.trim()) {
        const q = search.trim().toLowerCase()
        bills = bills.filter(
          (b) => b.householdName.toLowerCase().includes(q) || b.rt.toLowerCase().includes(q)
        )
      }

      // Client-side jenis filter (backend doesn't support it)
      if (jenisValue) {
        bills = bills.filter((b) => b.iuranType === jenisValue)
      }

      const totalPages = Math.max(1, Math.ceil(bills.length / size))

      setBackendBills(response?.data ?? [])
      setFilteredBills(bills)
      setTotal(bills.length)
      setTotalPages(totalPages)
      setPage(p)
      setPageSize(size)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal memuat data')
    } finally {
      setLoading(false)
    }
  }, [mapBill, statusValue, periodeValue, search, jenisValue])

  useEffect(() => {
    loadData(1, 10)
  }, [loadData])

  // Auto-refresh when user returns to tab (e.g., after paying from Android)
  useEffect(() => {
    function handleVisibilityChange() {
      if (document.visibilityState === 'visible') {
        loadData(1, pageSize)
      }
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [loadData, pageSize])

  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    if (filter === 'semua') {
      setStatusValue(null)
      setPeriodeValue('')
      setJenisValue('')
    }
  }

  const handleSearch = (value: string) => {
    setSearch(value)
    loadData(1, pageSize)
  }

  const handlePage = (p: number) => {
    if (p >= 1 && p <= totalPages) {
      loadData(p, pageSize)
    }
  }

  const handlePageSizeChange = (newSize: number) => {
    setPageSize(newSize)
    loadData(1, newSize)
  }

  const handleStatusSelect = (val: IuranStatus) => {
    setStatusValue(val)
    loadData(1, pageSize)
  }

  const handlePeriodeSelect = (val: string) => {
    setPeriodeValue(val)
    loadData(1, pageSize)
  }

  const handleJenisSelect = (val: string) => {
    setJenisValue(val)
    loadData(1, pageSize)
  }

  const statusLabel: string | null = statusValue ? getStatusLabel(statusValue) : null
  const periodeLabel: string | null = periodeValue ? formatPeriodeLabel(periodeValue) : null
  const jenisLabel: string | null = jenisValue
    ? IURAN_TYPE_OPTIONS.find((o) => o.value === jenisValue)?.label ?? null
    : null

  const uniquePeriodes = useMemo(() => {
    const s = new Set(backendBills.map((b: ApiBill) => b.period))
    return Array.from(s).sort().reverse()
  }, [backendBills])

  const uniquePeriodesOptions = useMemo(
    () => uniquePeriodes.map((p) => ({ value: p, label: formatPeriodeLabel(p) })),
    [uniquePeriodes]
  )

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
              onChange={(v) => setSearch(v)}
              onSearch={handleSearch}
              placeholder="Cari nama warga/RT..."
              statusControl={
                filterType === 'status' ? (
                  <div className="sf-status-selector">
                    {statusLabel ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {statusLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={() => { setStatusValue(null); loadData(1, pageSize) }}
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
                          onClick={() => { setPeriodeValue(''); loadData(1, pageSize) }}
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
                          onClick={() => { setJenisValue(''); loadData(1, pageSize) }}
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
          </div>

          {/* Create Due button */}
          <div className="sf-list-toolbar-right">
            <button
              type="button"
              className="sf-btn-primary"
              onClick={() => setShowCreateDue(true)}
            >
              <PlusOutlined style={{ marginRight: 4, fontSize: 12 }} />
              Tambah Iuran
            </button>
          </div>
        </div>

        {/* Create Due Modal */}
        {showCreateDue && (
          <DueCreate
            onClose={() => setShowCreateDue(false)}
            onSaved={() => loadData(1, pageSize)}
          />
        )}

        {/* Detail Modal */}
        {showDetailBillId && (
          <IuranDetail
            billId={showDetailBillId}
            open
            onClose={() => setShowDetailBillId(null)}
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
            onPaymentSuccess={() => loadData(1, pageSize)}
          />
        )}

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
            Tidak ada data iuran.
          </div>
        )}

        {/* Bill Table — dengan kolom VERIFIKASI untuk pembayaran pending */}
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
                  <th style={colStyles.status}>STATUS</th>
                  <th style={{ ...colStyles.action, width: 120, minWidth: 120 }}>VERIFIKASI</th>
                </tr>
              </thead>
              <tbody>
                {filteredBills.map((bill, idx) => {
                  const pendingForBill = (bill as IuranBill & { pendingPayments?: ApiPayment[] }).pendingPayments
                  return (
                    <IuranBillRow
                      key={bill.id}
                      bill={bill}
                      idx={idx}
                      canVerify={canVerify}
                      pendingPayments={pendingForBill ?? []}
                      onDetail={() => {
                        setShowDetailBillId(bill.id)
                      }}
                      onPayment={() => setShowPaymentBillId(bill.id)}
                    />
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
            total={total}
            totalPages={totalPages}
            onPageChange={handlePage}
            onPageSizeChange={handlePageSizeChange}
            pageSizeOptions={[10, 25, 50, 100]}
          />
        )}
      </div>
    </>
  )
}

interface IuranBillRowProps {
  bill: IuranBill
  idx: number
  canVerify: boolean
  pendingPayments: ApiPayment[]
  onDetail: () => void
  onPayment: () => void
}

function IuranBillRow({ bill, idx, canVerify, pendingPayments, onDetail, onPayment }: IuranBillRowProps) {
  // Show pending payment indicator in status cell if available
  const hasPending = pendingPayments.length > 0
  const pendingAmount = hasPending
    ? pendingPayments.reduce((sum, p) => sum + parseMoney(p.amount), 0)
    : 0

  return (
    <tr>
      <td style={colStyles.no}>
        {idx + 1}
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
        {formatPeriodeLabel(bill.periode)}
      </td>
      <td style={colStyles.nominal}>
        {formatRupiah(bill.nominal)}
      </td>
      <td style={colStyles.paid}>
        {formatRupiah(bill.paidAmount)}
        {hasPending && (
          <div style={{ fontSize: '10px', color: 'var(--sf-warning)', marginTop: '2px', fontWeight: 600 }}>
            +{formatRupiah(pendingAmount)} tertunda
          </div>
        )}
      </td>
      <td style={colStyles.status}>
        <Badge variant={getStatusVariant(bill.status)}>
          {getStatusLabel(bill.status)}
        </Badge>
        {hasPending && (
          <div style={{ fontSize: '10px', color: 'var(--sf-warning)', marginTop: '2px', fontWeight: 600 }}>
            ⏳ {pendingPayments.length} menunggu
          </div>
        )}
      </td>
      <td style={colStyles.action}>
        <RowActionMenu
          items={[
            { label: 'Detail', onClick: onDetail },
            { label: 'Pembayaran', onClick: onPayment },
            ...(hasPending && canVerify
              ? [{
                  label: `Verifikasi (${pendingPayments.length})`,
                  onClick: () => {
                    // Open detail modal with pending payment info
                    onDetail()
                  },
                }]
              : []),
          ]}
        />
      </td>
    </tr>
  )
}
