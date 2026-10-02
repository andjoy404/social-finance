import { useState, useMemo, useCallback, useEffect } from 'react'
import {
  WarningOutlined,
  DownOutlined,
  CloseOutlined,
} from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import {
  BILL_DATA,
  formatRupiah,
  formatPeriodeLabel,
  getStatusLabel,
} from './iuranTypes'
import type { IuranBill } from './iuranTypes'

const colStyles = {
  no: { width: 40, minWidth: 40, maxWidth: 40, textAlign: 'center' as const },
  name: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  rt: { width: 70, minWidth: 70, maxWidth: 70, textAlign: 'center' as const },
  type: { minWidth: 130, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  periode: { width: 90, minWidth: 90, maxWidth: 90, textAlign: 'center' as const },
  amount: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'right' as const, whiteSpace: 'nowrap' as const, fontFamily: 'monospace' },
  months: { width: 120, minWidth: 120, maxWidth: 120, textAlign: 'center' as const },
}

export function IuranArrears() {
  const arrearsBills = useMemo(() => {
    return BILL_DATA.filter((b) => b.status === 'belum_bayar' || b.status === 'sebagian')
  }, [])

  const [loading, setLoading] = useState(true)
  const [filterType, setFilterType] = useState<'semua' | 'periode' | 'rt'>('semua')
  const [search, setSearch] = useState('')
  const [periodeValue, setPeriodeValue] = useState('')
  const [rtValue, setRtValue] = useState('')

  const [filteredBills, setFilteredBills] = useState<IuranBill[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  // Summary stats
  const totalTunggakan = useMemo(() => {
    return arrearsBills.reduce((sum, b) => sum + (b.nominal - b.paidAmount), 0)
  }, [arrearsBills])

  const jumlahTagihanTertunggak = arrearsBills.length

  const uniqueRts = useMemo(() => {
    const s = new Set(arrearsBills.map((b) => b.rt))
    return Array.from(s)
  }, [arrearsBills])

  const uniquePeriodes = useMemo(() => {
    const s = new Set(arrearsBills.map((b) => b.periode))
    return Array.from(s).sort().reverse()
  }, [arrearsBills])

  const loadData = useCallback(async (p: number, size: number) => {
    setLoading(true)
    await new Promise((r) => setTimeout(r, 400))

    let filtered = arrearsBills

    if (search.trim()) {
      const q = search.trim().toLowerCase()
      filtered = filtered.filter(
        (b) => b.householdName.toLowerCase().includes(q) || b.rt.toLowerCase().includes(q)
      )
    }

    if (filterType === 'periode' && periodeValue) {
      filtered = filtered.filter((b) => b.periode === periodeValue)
    }

    if (filterType === 'rt' && rtValue) {
      filtered = filtered.filter((b) => b.rt === rtValue)
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
  }, [arrearsBills, filterType, search, periodeValue, rtValue])

  useEffect(() => {
    loadData(1, 10)
  }, [loadData])

  const handleSearch = (value: string) => {
    setSearch(value)
    loadData(1, pageSize)
  }

  const handlePage = (p: number) => {
    if (p >= 1 && p <= totalPages) loadData(p, pageSize)
  }

  const handlePageSizeChange = (newSize: number) => {
    setPageSize(newSize)
    loadData(1, newSize)
  }

  const handlePeriodeSelect = (val: string) => {
    setPeriodeValue(val)
    loadData(1, pageSize)
  }

  const handleRtSelect = (val: string) => {
    setRtValue(val)
    loadData(1, pageSize)
  }

  const periodeLabel: string | null = periodeValue ? formatPeriodeLabel(periodeValue) : null
  const rtLabel: string | null = rtValue ? rtValue : null

  return (
    <>
      {/* PageHeader */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <WarningOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Tunggakan Iuran</div>
            <div className="sf-page-header-subtitle">
              Daftar tagihan yang belum atau belum lunas dibayar
            </div>
          </div>
        </div>
      </div>

      <div className="sf-content-surface">
        {/* Summary cards */}
        <div className="sf-metrics-row" style={{ marginBottom: 'var(--sp-md)' }}>
          <div className="sf-metric-card">
            <div className="sf-metric-label">Total Tunggakan</div>
            <div className="sf-metric-value" style={{ color: 'var(--sf-danger)' }}>
              {formatRupiah(totalTunggakan)}
            </div>
          </div>
          <div className="sf-metric-card">
            <div className="sf-metric-label">Jumlah Tagihan Tertunggak</div>
            <div className="sf-metric-value">
              {jumlahTagihanTertunggak}
            </div>
          </div>
        </div>

        {/* Toolbar */}
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            <FilterDropdown
              value={filterType}
              options={[
                { value: 'semua', label: 'Semua' },
                { value: 'periode', label: 'Periode' },
                { value: 'rt', label: 'RT' },
              ]}
              onChange={(v) => {
                setFilterType(v)
                if (v === 'semua') { setPeriodeValue(''); setRtValue(''); loadData(1, pageSize) }
              }}
              ariaLabel="Tipe filter"
            />
            <SearchBox
              value={search}
              onChange={setSearch}
              onSearch={handleSearch}
              placeholder="Cari nama warga/RT..."
              statusControl={
                filterType === 'periode' ? (
                  <div className="sf-status-selector">
                    {periodeLabel ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {periodeLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={() => { setPeriodeValue(''); loadData(1, pageSize) }}
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
                        >
                          Pilih Periode
                          <DownOutlined style={{ fontSize: 9, color: 'var(--sf-text-muted)' }} />
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {uniquePeriodes.map((p) => (
                            <button
                              key={p}
                              type="button"
                              className={`sf-status-dropdown-item ${periodeValue === p ? 'sf-status-dropdown-item-active' : ''}`}
                              onClick={() => handlePeriodeSelect(p)}
                            >
                              {formatPeriodeLabel(p)}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : filterType === 'rt' ? (
                  <div className="sf-status-selector">
                    {rtLabel ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {rtLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={() => { setRtValue(''); loadData(1, pageSize) }}
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
                        >
                          Pilih RT
                          <DownOutlined style={{ fontSize: 9, color: 'var(--sf-text-muted)' }} />
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {uniqueRts.map((rt) => (
                            <button
                              key={rt}
                              type="button"
                              className={`sf-status-dropdown-item ${rtValue === rt ? 'sf-status-dropdown-item-active' : ''}`}
                              onClick={() => handleRtSelect(rt)}
                            >
                              {rt}
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

        {/* Loading */}
        {loading && (
          <div className="sf-loading-banner">
            Memuat data tunggakan...
          </div>
        )}

        {/* Empty */}
        {!loading && filteredBills.length === 0 && (
          <div className="sf-empty-banner">
            Tidak ada data tunggakan.
          </div>
        )}

        {/* Table */}
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
                  <th style={colStyles.amount}>NOMINAL TERTUNGGAK</th>
                  <th style={{ ...colStyles.months, textAlign: 'center' }}>STATUS</th>
                </tr>
              </thead>
              <tbody>
                {filteredBills.map((bill, idx) => {
                  const sisa = bill.nominal - bill.paidAmount
                  return (
                    <tr key={bill.id}>
                      <td style={colStyles.no}>{idx + 1}</td>
                      <td style={colStyles.name}>
                        <span style={{ fontWeight: 600 }}>{bill.householdName}</span>
                      </td>
                      <td style={colStyles.rt}>{bill.rt}</td>
                      <td style={colStyles.type}>{bill.iuranType}</td>
                      <td style={colStyles.periode}>{formatPeriodeLabel(bill.periode)}</td>
                      <td style={{ ...colStyles.amount, color: 'var(--sf-danger)', fontWeight: 600 }}>
                        {formatRupiah(sisa)}
                      </td>
                      <td style={colStyles.months}>
                        <Badge variant={bill.status === 'belum_bayar' ? 'red' : 'amber'}>
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
