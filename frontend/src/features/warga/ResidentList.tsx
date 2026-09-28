import { useState, useEffect, useCallback, useMemo } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  listResidents,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiResident,
} from '@/app/api'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { UserOutlined, CloseOutlined, DownOutlined } from '@ant-design/icons'
import { RowActionMenu } from '@/components/RowActionMenu'
import { DataChoiceModal } from './DataChoiceModal'

type FilterType = 'semua' | 'status' | 'occupancy'

type StatusValue = 'aktif' | 'tidak_aktif' | null
type OccupancyValue = 'semua' | 'OWNER' | 'TENANT'

const validSpecialJabatans: Record<string, boolean> = {
  keamanan: true,
  kebersihan_pembangunan: true,
}

function isSpecialResident(resident: ApiResident): boolean {
  return validSpecialJabatans[resident.jabatan ?? ''] === true
}

const statusFilterOptions: { value: FilterType; label: string }[] = [
  { value: 'semua', label: 'Semua' },
  { value: 'status', label: 'Status' },
  { value: 'occupancy', label: 'Status Hunian' },
]

const statusValueOptions: { value: StatusValue; label: string }[] = [
  { value: 'aktif', label: 'Aktif' },
  { value: 'tidak_aktif', label: 'Tidak Aktif' },
]

const occupancyOptions: { value: OccupancyValue; label: string }[] = [
  { value: 'semua', label: 'Semua' },
  { value: 'OWNER', label: 'Pemilik' },
  { value: 'TENANT', label: 'Penyewa' },
]

function statusBadge(value: boolean): 'green' | 'red' {
  return value ? 'green' : 'red'
}

function statusLabel(value: boolean): string {
  return value ? 'Aktif' : 'Tidak Aktif'
}

function formatIntegerOnly(value: string | number | null | undefined): string {
  if (value == null) return '\u2014'
  if (typeof value === 'number') {
    return isNaN(value) ? '\u2014' : String(Math.floor(value))
  }
  const str = String(value).trim()
  if (!str || str === '\u2014' || str === '-') return '\u2014'
  const match = str.replace(/^(rt|rw)\s*/i, '').match(/\d+/)
  if (match) {
    const num = parseInt(match[0], 10)
    return isNaN(num) ? '\u2014' : String(num)
  }
  return '\u2014'
}

function formatStrictDate(value?: string | null): string {
  if (!value) return '\u2014'
  const trimmed = value.trim()
  if (!trimmed || trimmed === '\u2014' || trimmed === '-') return '\u2014'
  const match = trimmed.match(/^(\d{4}-\d{2}-\d{2})/)
  if (match) {
    return match[1]
  }
  const d = new Date(trimmed)
  if (isNaN(d.getTime())) return '\u2014'
  return d.toISOString().substring(0, 10)
}

function formatNull(value: string | null | undefined): string {
  return value ?? '\u2014'
}

function formatOccupancyStatus(status: 'OWNER' | 'TENANT' | null | undefined): { label: string; variant: 'violet' | 'blue' | 'default' } {
  if (!status) return { label: '\u2014', variant: 'default' as const }
  return status === 'OWNER' ? { label: 'Pemilik', variant: 'violet' as const } : { label: 'Penyewa', variant: 'blue' as const }
}

function getJabatanColor(jabatan: string | null | undefined): 'violet' | 'deepPurple' | 'lightPurple' | 'amber' | 'green' | 'red' | 'blue' | 'gray' | 'default' {
  if (!jabatan || jabatan.trim() === '') return 'default'
  switch (jabatan) {
    case 'ketua':
      return 'violet'
    case 'wakil_ketua':
      return 'blue'
    case 'sekretaris':
      return 'green'
    case 'bendahara':
      return 'amber'
    case 'sosial':
      return 'gray'
    case 'keamanan':
      return 'red'
    case 'kebersihan_pembangunan':
      return 'blue'
    default:
      return 'default'
  }
}

function getJabatanLabel(jabatan: string | null | undefined): string {
  if (!jabatan || jabatan.trim() === '') return '\u2014'
  switch (jabatan) {
    case 'ketua':
      return 'Ketua RT'
    case 'wakil_ketua':
      return 'Wakil Ketua RT'
    case 'sekretaris':
      return 'Sekretaris RT'
    case 'bendahara':
      return 'Bendahara RT'
    case 'sosial':
      return 'Seksi Sosial'
    case 'keamanan':
      return 'Seksi Keamanan'
    case 'kebersihan_pembangunan':
      return 'Kebersihan & Pembangunan'
    default:
      return jabatan
  }
}

const colStyles = {
  rt: { width: 60, minWidth: 60, maxWidth: 60, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  rw: { width: 60, minWidth: 60, maxWidth: 60, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  rtName: { minWidth: 140, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  name: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  nik: { width: 170, minWidth: 170, maxWidth: 170, textAlign: 'center' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  phone: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'center' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  email: { minWidth: 180, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  occupancyStatus: { width: 110, minWidth: 110, maxWidth: 110, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  startDate: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'left' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  endDate: { width: 150, minWidth: 150, maxWidth: 150, textAlign: 'left' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  jabatan: { width: 200, minWidth: 200, maxWidth: 200, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  status: { width: 100, minWidth: 100, maxWidth: 100, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  action: { width: 80, minWidth: 80, maxWidth: 80, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
}

export function ResidentList() {
  const { user } = useAuth()
  const navigate = useNavigate()
  if (!user) return null

  const isSuperAdmin = user.systemRole === 'super_admin' || user.role === 'super_admin'
  const isPengurus = user.role === 'pengurus'
  const isReadOnly = user.role === 'warga' && !isSuperAdmin
  const showNik = isPengurus || isSuperAdmin
  const showAksi = !isReadOnly

  const [choiceOpen, setChoiceOpen] = useState(false)

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [residentList, setResidentList] = useState<ApiResident[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:warga', [10, 25, 50, 100])
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [search, setSearch] = useState('')
  const [statusValue, setStatusValue] = useState<StatusValue>(null)
  const [statusOpen, setStatusOpen] = useState(false)
  const [occupancyFilter, setOccupancyFilter] = useState<OccupancyValue>('semua')
  const [occupancyOpen, setOccupancyOpen] = useState(false)
  const [searchParams] = useSearchParams()

  const fetchResidents = useCallback(async (p: number, size: number) => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError(new ApiError('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
      return
    }

    setLoading(true)
    setError(null)

    try {
      const params: Record<string, string | number | boolean | undefined> = {
        page: p,
        page_size: size,
      }

      if (filterType === 'status' && statusValue === 'aktif') params.is_active = true
      else if (filterType === 'status' && statusValue === 'tidak_aktif') params.is_active = false

      if (search.trim()) params.search = search.trim()

      const resp = await listResidents(pair.accessToken, params as import('@/app/api').ApiListResidentsParams)

      setResidentList(resp.data)
      setTotalPages(resp.pagination.total_pages)
      setTotal(resp.pagination.total)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        window.location.href = '/login'
      } else if (err instanceof ApiError && (err.code === 'forbidden' || err.code === 'unauthorized')) {
        setError(new ApiError('access_denied', 'Anda tidak memiliki akses ke halaman ini.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal memuat data warga.'))
      }
    } finally {
      setLoading(false)
    }
  }, [filterType, statusValue, search])

  useEffect(() => {
    void fetchResidents(page, pageSize)
  }, [page, pageSize, fetchResidents])

  const refresh = searchParams.get('refresh')
  useEffect(() => {
    if (refresh !== '1') return
    void fetchResidents(page, pageSize)
    const url = new URL(window.location.href)
    url.searchParams.delete('refresh')
    window.history.replaceState({}, '', url.pathname + url.search)
  }, [refresh, fetchResidents, page, pageSize])

  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    if (filter === 'semua') {
      setStatusValue(null)
      setOccupancyFilter('semua')
    }
  }

  const handleSearch = (value: string) => {
    setSearch(value)
    setPage(1)
  }

  const handlePage = (p: number) => {
    if (p >= 1 && p <= totalPages) setPage(p)
  }

  const handlePageSizeChange = (newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }

  const handleStatusSelect = (val: StatusValue) => {
    setStatusValue(val)
    setStatusOpen(false)
    setPage(1)
  }

  const handleStatusClear = () => {
    setStatusValue(null)
  }

  const handleOccupancySelect = (val: OccupancyValue) => {
    setOccupancyFilter(val)
    setOccupancyOpen(false)
    setPage(1)
  }

  const handleOccupancyClear = () => {
    setOccupancyFilter('semua')
  }

  const occupancyFilterLabel: string | null = occupancyFilter === 'semua' ? null : occupancyFilter === 'OWNER' ? 'Pemilik' : 'Penyewa'

  const filteredResidents = useMemo(() => {
    if (occupancyFilter === 'semua') return residentList
    return residentList.filter((r) => r.occupancy_status === occupancyFilter)
  }, [residentList, occupancyFilter])

  const handleDetail = useCallback((id: string) => {
    navigate(`/warga/${id}`)
  }, [navigate])

  const handleEdit = useCallback((id: string) => {
    navigate(`/warga/${id}/edit`)
  }, [navigate])

  const handleEditSpecial = useCallback((id: string) => {
    navigate(`/warga/special/${id}/edit`)
  }, [navigate])

  const statusValueLabel: string | null = statusValue === 'aktif' ? 'Aktif' : statusValue === 'tidak_aktif' ? 'Tidak Aktif' : null

  return (
    <>
      {/* PageHeader — separate floating surface */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <UserOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Daftar Warga</div>
            <div className="sf-page-header-subtitle">
              {user.rt?.name ? `Manajemen warga dan kepala keluarga \u2014 ${user.rt.name}` : 'Manajemen warga dan kepala keluarga'}
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
              placeholder="Cari nama warga/NIK..."
              statusControl={
                filterType === 'status' ? (
                  <div className="sf-status-selector">
                    {statusValueLabel ? (
                      <span
                        className={`sf-status-badge ${statusValue === 'aktif' ? 'sf-status-badge__active' : 'sf-status-badge__inactive'}`}
                      >
                        {statusValueLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={handleStatusClear}
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
                          onClick={() => setStatusOpen(!statusOpen)}
                          aria-haspopup="listbox"
                          aria-expanded={statusOpen}
                        >
                          Pilih Status
                          <DownOutlined style={{ fontSize: 9, color: 'var(--sf-text-muted)' }} />
                        </button>
                        {statusOpen && (
                          <div className="sf-status-dropdown-menu" role="listbox">
                            {statusValueOptions.map((opt) => (
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
                        )}
                      </div>
                    )}
                  </div>
                ) : filterType === 'occupancy' ? (
                  <div className="sf-status-selector">
                    {occupancyFilterLabel ? (
                      <span
                        className={`sf-status-badge sf-status-badge__active`}
                      >
                        {occupancyFilterLabel}
                        <button
                          type="button"
                          className="sf-status-badge-clear"
                          onClick={handleOccupancyClear}
                          aria-label="Hapus filter status hunian"
                          title="Hapus filter status hunian"
                        >
                          <CloseOutlined style={{ fontSize: 10 }} />
                        </button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button
                          type="button"
                          className="sf-status-selector-trigger"
                          onClick={() => setOccupancyOpen(!occupancyOpen)}
                          aria-haspopup="listbox"
                          aria-expanded={occupancyOpen}
                        >
                          Pilih Status Hunian
                          <DownOutlined style={{ fontSize: 9, color: 'var(--sf-text-muted)' }} />
                        </button>
                        {occupancyOpen && (
                          <div className="sf-status-dropdown-menu" role="listbox">
                            {occupancyOptions.map((opt) => (
                              <button
                                key={opt.value}
                                type="button"
                                className={`sf-status-dropdown-item ${occupancyFilter === opt.value ? 'sf-status-dropdown-item-active' : ''}`}
                                onClick={() => handleOccupancySelect(opt.value)}
                                role="button"
                              >
                                {opt.label}
                              </button>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ) : null
              }
            />
          </div>

          {/* Create button — hidden for warga (read-only) */}
          {!isReadOnly && (
            <button className="sf-create-btn" onClick={() => setChoiceOpen(true)}>
              + Tambah
            </button>
          )}
        </div>

        {/* Data Choice Modal */}
        <DataChoiceModal open={choiceOpen} onClose={() => setChoiceOpen(false)} />

        {/* Error */}
        {error && (
          <div className="sf-error-banner" role="alert">
            {error.message}
          </div>
        )}

        {/* Loading */}
        {loading && (
          <div className="sf-loading-banner">
            Memuat data warga...
          </div>
        )}

        {/* Empty */}
        {!loading && !error && filteredResidents.length === 0 && (
          <div className="sf-empty-banner">
            Tidak ada data warga.
          </div>
        )}

        {/* Resident Table */}
        {!loading && filteredResidents.length > 0 && (
          <div className="table-wrapper">
            <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th className="col-rt" style={colStyles.rt}>RT</th>
                  <th className="col-rw" style={colStyles.rw}>RW</th>
                  <th className="col-rt-name" style={colStyles.rtName}>NAMA RT</th>
                  <th className="col-name" style={colStyles.name}>NAMA</th>
                  {showNik && <th className="col-nik" style={colStyles.nik}>NIK</th>}
                  <th className="col-phone" style={colStyles.phone}>TELEPON</th>
                  <th className="col-email" style={colStyles.email}>EMAIL</th>
                  <th className="col-occupancy-status" style={colStyles.occupancyStatus}>STATUS HUNIAN</th>
                  <th className="col-start-date" style={colStyles.startDate}>TANGGAL MULAI</th>
                  <th className="col-end-date" style={colStyles.endDate}>TANGGAL SELESAI</th>
                  <th className="col-jabatan" style={colStyles.jabatan}>JABATAN</th>
                  <th className="col-status" style={colStyles.status}>STATUS</th>
                  {showAksi && <th className="col-action" style={colStyles.action}>AKSI</th>}
                </tr>
              </thead>
              <tbody>
                {filteredResidents.map((resident) => (
                  <ResidentRow
                    key={resident.id}
                    resident={resident}
                    showNik={showNik}
                    showAksi={showAksi}
                    onDetail={handleDetail}
                    onEdit={handleEdit}
                    onEditSpecial={handleEditSpecial}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && filteredResidents.length > 0 && (
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

interface ResidentRowProps {
  resident: ApiResident
  showNik: boolean
  showAksi: boolean
  onDetail: (id: string) => void
  onEdit: (id: string) => void
  onEditSpecial: (id: string) => void
}

function ResidentRow({ resident, showNik, showAksi, onDetail, onEdit, onEditSpecial }: ResidentRowProps) {
  const startDate = formatStrictDate(resident.start_date)
  const endDate = formatStrictDate(resident.end_date)

  const occupancy = formatOccupancyStatus(resident.occupancy_status)
  const jabatanLabel = getJabatanLabel(resident.jabatan)
  const jabatanColor = getJabatanColor(resident.jabatan)

  return (
    <tr>
      {/* 1. RT */}
      <td className="col-rt" style={colStyles.rt}>
        {formatIntegerOnly(resident.rt_number ?? null)}
      </td>

      {/* 2. RW */}
      <td className="col-rw" style={colStyles.rw}>
        {formatIntegerOnly(resident.rw ?? null)}
      </td>

      {/* 3. NAMA RT */}
      <td className="col-rt-name" style={colStyles.rtName}>
        {formatNull(resident.rt_name ?? null)}
      </td>

      {/* 4. NAMA */}
      <td className="col-name" style={{ ...colStyles.name, fontWeight: 600 }}>
        {resident.full_name}
      </td>

      {/* 5. NIK */}
      {showNik && (
        <td className="col-nik" style={colStyles.nik}>
          {formatNull(resident.nik ?? null)}
        </td>
      )}

      {/* 6. TELEPON */}
      <td className="col-phone" style={colStyles.phone}>
        {formatNull(resident.phone ?? null)}
      </td>

      {/* 7. EMAIL */}
      <td className="col-email" style={colStyles.email}>
        {formatNull(resident.email ?? null)}
      </td>

      {/* 8. STATUS HUNIAN */}
      <td className="col-occupancy-status" style={colStyles.occupancyStatus}>
        <Badge variant={occupancy.variant}>
          {occupancy.label}
        </Badge>
      </td>

      {/* 9. TANGGAL MULAI */}
      <td className="col-start-date" style={colStyles.startDate}>
        {startDate}
      </td>

      {/* 10. TANGGAL SELESAI */}
      <td className="col-end-date" style={colStyles.endDate}>
        {endDate}
      </td>

      {/* 11. JABATAN */}
      <td className="col-jabatan" style={colStyles.jabatan}>
        <Badge variant={jabatanColor}>
          {jabatanLabel}
        </Badge>
      </td>

      {/* 12. STATUS */}
      <td className="col-status" style={colStyles.status}>
        <Badge variant={statusBadge(resident.is_active)}>
          {statusLabel(resident.is_active)}
        </Badge>
      </td>

      {/* 13. AKSI */}
      {showAksi && (
        <td className="col-action" style={colStyles.action}>
          <RowActionMenu
            items={[
              { label: 'Detail', onClick: () => onDetail(resident.id) },
              ...(resident.household_id
                ? [{ label: 'Ubah', onClick: () => onEdit(resident.household_id!) }]
                : isSpecialResident(resident)
                  ? [{ label: 'Ubah', onClick: () => onEditSpecial(resident.id) }]
                  : []),
            ]}
          />
        </td>
      )}
    </tr>
  )
}
