import { useState, useEffect, useCallback } from 'react'
import { ReloadOutlined, BookOutlined } from '@ant-design/icons'
import {
  GuestBookEntry,
  apiListSecurityGuestBooks,
} from './securityApi'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { Badge } from '@/components/Badge'

type FilterStatus = 'semua' | 'active' | 'checked_out'

const guestStatusFilterOptions: { value: FilterStatus; label: string }[] = [
  { value: 'semua', label: 'Semua' },
  { value: 'active', label: 'Masih Berkunjung' },
  { value: 'checked_out', label: 'Sudah Keluar' },
]

function formatDateTime(iso?: string | null): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function getErrMsg(err: unknown, fallback: string): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const r = (err as { response?: { data?: { error?: { message?: string } } } }).response
    if (r?.data?.error?.message) return r.data.error.message
  }
  if (err instanceof Error) return err.message
  return fallback
}

// Column width styles matching Warga's approach
const colStyles = {
  guestName: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  phone: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'center' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  purpose: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  destination: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  vehicle: { width: 130, minWidth: 130, maxWidth: 130, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  checkedIn: { width: 150, minWidth: 150, maxWidth: 150, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  checkedOut: { width: 150, minWidth: 150, maxWidth: 150, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  action: { width: 80, minWidth: 80, maxWidth: 80, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
}

export function BukuTamuPage() {
  const [loading, setLoading] = useState(false)
  const [guestBooks, setGuestBooks] = useState<GuestBookEntry[]>([])
  const [guestTotal, setGuestTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [guestFilter, setGuestFilter] = useState<{
    status?: string
    search?: string
  }>({})
  const [filterType, setFilterType] = useState<FilterStatus>('semua')
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const loadGuestBooks = useCallback(
    async (p = page, size = pageSize) => {
      setLoading(true)
      setErrorMessage(null)
      try {
        const res = await apiListSecurityGuestBooks({
          ...guestFilter,
          page: p,
          page_size: size,
        })
        setGuestBooks(res.data ?? [])
        setGuestTotal(res.pagination?.total ?? 0)
      } catch (err) {
        setErrorMessage(getErrMsg(err, 'Gagal memuat buku tamu'))
      } finally {
        setLoading(false)
      }
    },
    [guestFilter, page, pageSize],
  )

  useEffect(() => {
    void loadGuestBooks(1, pageSize)
  }, [loadGuestBooks, pageSize])

  const handleSearch = useCallback((value: string) => {
    setGuestFilter((prev) => ({ ...prev, search: value || undefined }))
    setPage(1)
  }, [])

  const handleFilterChange = useCallback((value: FilterStatus) => {
    setFilterType(value)
    setPage(1)
    setGuestFilter((prev) => ({
      ...prev,
      status: value === 'semua' ? undefined : value,
    }))
  }, [])

  const handlePageChange = useCallback((p: number) => {
    if (p >= 1) {
      setPage(p)
      void loadGuestBooks(p, pageSize)
    }
  }, [loadGuestBooks, pageSize])

  const handlePageSizeChange = useCallback((newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }, [])

  const handleRefresh = useCallback(() => {
    void loadGuestBooks(page, pageSize)
  }, [loadGuestBooks, page, pageSize])

  const total = guestTotal
  const totalPages = Math.ceil(total / pageSize) || 1

  return (
    <>
      {/* PageHeader — same pattern as Warga */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <BookOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Buku Tamu RT</div>
            <div className="sf-page-header-subtitle">
              Daftar kunjungan tamu dan status checkout di lingkungan RT
            </div>
          </div>
        </div>
      </div>

      {/* Content surface */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            {/* Filter dropdown */}
            <FilterDropdown
              value={filterType}
              options={guestStatusFilterOptions}
              onChange={handleFilterChange}
              ariaLabel="Status kunjungan"
            />

            {/* Search */}
            <SearchBox
              value={guestFilter.search ?? ''}
              onChange={(v) => setGuestFilter((p) => ({ ...p, search: v || undefined }))}
              onSearch={handleSearch}
              placeholder="Cari nama / tujuan / plat..."
            />
          </div>

          {/* Action buttons */}
          <div className="sf-list-toolbar-right" style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
            <button
              type="button"
              className="sf-btn-secondary"
              onClick={handleRefresh}
              style={{
                padding: '6px 14px',
                fontSize: '12px',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <ReloadOutlined style={{ fontSize: 13 }} />
              Refresh
            </button>
          </div>
        </div>

        {/* Error */}
        {errorMessage && (
          <div className="sf-error-banner" role="alert">
            {errorMessage}
          </div>
        )}

        {/* Loading */}
        {loading && (
          <div className="sf-loading-banner">
            Memuat data buku tamu...
          </div>
        )}

        {/* Empty */}
        {!loading && !errorMessage && guestBooks.length === 0 && (
          <div className="sf-empty-banner">
            Tidak ada data buku tamu.
          </div>
        )}

        {/* Guest Book Table */}
        {!loading && guestBooks.length > 0 && (
          <div className="table-wrapper">
            <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th className="col-name" style={colStyles.guestName}>NAMA TAMU</th>
                  <th className="col-phone" style={colStyles.phone}>NO. HP</th>
                  <th className="col-purpose" style={colStyles.purpose}>KEPERLUAN</th>
                  <th className="col-destination" style={colStyles.destination}>TUJUAN</th>
                  <th className="col-vehicle" style={colStyles.vehicle}>KENDARAAN</th>
                  <th className="col-checked-in" style={colStyles.checkedIn}>MASUK</th>
                  <th className="col-checked-out" style={colStyles.checkedOut}>KELUAR</th>
                </tr>
              </thead>
              <tbody>
                {guestBooks.map((entry) => (
                  <tr key={entry.id}>
                    {/* 1. NAMA TAMU */}
                    <td className="col-name" style={{ ...colStyles.guestName, fontWeight: 600 }}>
                      {entry.guest_name}
                    </td>

                    {/* 2. NO. HP */}
                    <td className="col-phone" style={colStyles.phone}>
                      {entry.phone_masked || entry.phone || '\u2014'}
                    </td>

                    {/* 3. KEPERLUAN */}
                    <td className="col-purpose" style={colStyles.purpose}>
                      {entry.purpose || '\u2014'}
                    </td>

                    {/* 4. TUJUAN */}
                    <td className="col-destination" style={colStyles.destination}>
                      {entry.destination_label || '\u2014'}
                    </td>

                    {/* 5. KENDARAAN */}
                    <td className="col-vehicle" style={colStyles.vehicle}>
                      {entry.vehicle_plate || '\u2014'}
                    </td>

                    {/* 6. MASUK */}
                    <td className="col-checked-in" style={colStyles.checkedIn}>
                      {formatDateTime(entry.checked_in_at)}
                    </td>

                    {/* 7. KELUAR — badge for still inside */}
                    <td className="col-checked-out" style={colStyles.checkedOut}>
                      {entry.checked_out_at ? (
                        formatDateTime(entry.checked_out_at)
                      ) : (
                        <Badge variant="blue">Masih di RT</Badge>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && guestBooks.length > 0 && (
          <Pagination
            page={page}
            pageSize={pageSize}
            total={total}
            totalPages={totalPages}
            onPageChange={handlePageChange}
            onPageSizeChange={handlePageSizeChange}
            pageSizeOptions={[20, 50, 100]}
          />
        )}
      </div>
    </>
  )
}
