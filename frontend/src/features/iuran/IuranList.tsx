import { useState, useEffect, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  DollarOutlined,
  CloseOutlined,
  DownOutlined,
} from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { RowActionMenu } from '@/components/RowActionMenu'
import {
  BILL_DATA,
  formatRupiah,
  formatPeriodeLabel,
  getStatusVariant,
  getStatusLabel,
  IURAN_TYPE_OPTIONS,
} from './iuranTypes'
import type { IuranBill, IuranStatus } from './iuranTypes'

type FilterType = 'semua' | 'status' | 'periode' | 'jenis'

const statusFilterOptions: { value: FilterType; label: string }[] = [
  { value: 'semua', label: 'Semua' },
  { value: 'status', label: 'Status' },
  { value: 'periode', label: 'Periode' },
  { value: 'jenis', label: 'Jenis Iuran' },
]

const STATUS_OPTIONS: { value: IuranStatus; label: string }[] = [
  { value: 'belum_bayar', label: 'Belum Bayar' },
  { value: 'sebagian', label: 'Sebagian' },
  { value: 'lunas', label: 'Lunas' },
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
  const navigate = useNavigate()

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [allBills] = useState<IuranBill[]>(BILL_DATA)

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

  const loadData = useCallback(async (p: number, size: number) => {
    setLoading(true)
    setError(null)
    await new Promise((r) => setTimeout(r, 400))

    let filtered = allBills

    if (search.trim()) {
      const q = search.trim().toLowerCase()
      filtered = filtered.filter(
        (b) => b.householdName.toLowerCase().includes(q) || b.rt.toLowerCase().includes(q)
      )
    }

    if (filterType === 'status' && statusValue) {
      filtered = filtered.filter((b) => b.status === statusValue)
    }

    if (filterType === 'periode' && periodeValue) {
      filtered = filtered.filter((b) => b.periode === periodeValue)
    }

    if (filterType === 'jenis' && jenisValue) {
      filtered = filtered.filter((b) => b.iuranType === jenisValue)
    }

    const start = (p - 1) * size
    const paginated = filtered.slice(start, start + size)
    const totalPages = Math.max(1, Math.ceil(filtered.length / size))

    setFilteredBills(paginated)
    setTotal(filtered.length)
    setTotalPages(totalPages)
    setPage(p)
    setPageSize(size)
    setLoading(false)
  }, [allBills, filterType, statusValue, search, periodeValue, jenisValue])

  useEffect(() => {
    loadData(1, 10)
  }, [loadData])

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
    const s = new Set(allBills.map((b) => b.periode))
    return Array.from(s).sort().reverse()
  }, [allBills])

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
        </div>

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
                  <th style={colStyles.status}>STATUS</th>
                  <th style={colStyles.action}>AKSI</th>
                </tr>
              </thead>
              <tbody>
                {filteredBills.map((bill, idx) => (
                  <IuranBillRow
                    key={bill.id}
                    bill={bill}
                    idx={idx}
                    onDetail={() => navigate(`/iuran/${bill.id}`)}
                    onPayment={() => navigate(`/iuran/${bill.id}/pembayaran`)}
                  />
                ))}
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
  onDetail: () => void
  onPayment: () => void
}

function IuranBillRow({ bill, idx, onDetail, onPayment }: IuranBillRowProps) {
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
      </td>
      <td style={colStyles.status}>
        <Badge variant={getStatusVariant(bill.status)}>
          {getStatusLabel(bill.status)}
        </Badge>
      </td>
      <td style={colStyles.action}>
        <RowActionMenu
          items={[
            { label: 'Detail', onClick: onDetail },
            { label: 'Pembayaran', onClick: onPayment },
          ]}
        />
      </td>
    </tr>
  )
}
