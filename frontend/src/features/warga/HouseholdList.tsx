import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  listHouseholds,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiHousehold,
} from '@/app/api'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { Link } from 'react-router-dom'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { UserOutlined, CloseOutlined, DownOutlined } from '@ant-design/icons'
import { HouseholdEdit } from './HouseholdEdit'
import { RowActionMenu } from '@/components/RowActionMenu'

type FilterType = 'semua' | 'status'

type StatusValue = 'aktif' | 'tidak_aktif' | null

const statusFilterOptions: { value: FilterType; label: string }[] = [
  { value: 'semua', label: 'Semua' },
  { value: 'status', label: 'Status' },
]

const statusValueOptions: { value: StatusValue; label: string }[] = [
  { value: 'aktif', label: 'Aktif' },
  { value: 'tidak_aktif', label: 'Tidak Aktif' },
]

function occupancyLabel(status: ApiHousehold['occupancy_status']): string {
  switch (status) {
    case 'OWNER':
      return 'Pemilik'
    case 'TENANT':
      return 'Penyewa'
    default:
      return '\u2014'
  }
}

function occupancyBadgeVariant(status: ApiHousehold['occupancy_status']): 'violet' | 'blue' | 'default' {
  switch (status) {
    case 'OWNER':
      return 'violet'
    case 'TENANT':
      return 'blue'
    default:
      return 'default'
  }
}

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

const colStyles = {
  rt: { width: 60, minWidth: 60, maxWidth: 60, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  rw: { width: 60, minWidth: 60, maxWidth: 60, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  rtName: { minWidth: 140, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  houseNumber: { minWidth: 120, textAlign: 'left' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  address: { minWidth: 180, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  occupancy: { width: 130, minWidth: 130, maxWidth: 130, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  name: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  nik: { width: 170, minWidth: 170, maxWidth: 170, textAlign: 'center' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  phone: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'center' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  email: { minWidth: 180, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  startDate: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'left' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  endDate: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'left' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  status: { width: 100, minWidth: 100, maxWidth: 100, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  action: { width: 80, minWidth: 80, maxWidth: 80, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
}

export function HouseholdList() {
  const { user } = useAuth()
  if (!user) return null

  const isSuperAdmin = user.systemRole === 'super_admin' || user.role === 'super_admin'
  const isPengurus = user.role === 'pengurus'
  const isReadOnly = user.role === 'warga' && !isSuperAdmin
  const showNik = isPengurus || isSuperAdmin
  const showAksi = !isReadOnly

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [householdList, setHouseholdList] = useState<ApiHousehold[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:warga', [10, 25, 50, 100])
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [search, setSearch] = useState('')
  const [statusValue, setStatusValue] = useState<StatusValue>(null)
  const [statusOpen, setStatusOpen] = useState(false)
  const [editingHh, setEditingHh] = useState<ApiHousehold | null>(null)

  const fetchHouseholds = useCallback(async (p: number, size: number) => {
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

      const resp = await listHouseholds(pair.accessToken, params as import('@/app/api').ApiListHouseholdsParams)

      setHouseholdList(resp.data)
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
    void fetchHouseholds(page, pageSize)
  }, [page, pageSize, fetchHouseholds])

  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    if (filter === 'semua') {
      setStatusValue(null)
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

  const handleEdit = (hh: ApiHousehold) => {
    setEditingHh(hh)
  }


  const statusValueLabel: string | null = statusValue === 'aktif' ? 'Aktif' : statusValue === 'tidak_aktif' ? 'Tidak Aktif' : null

  return (
    <>
      {editingHh && (
        <HouseholdEdit
          id={editingHh.id}
          onClose={() => setEditingHh(null)}
          onSaved={() => {
            setEditingHh(null)
            void fetchHouseholds(page, pageSize)
          }}
        />
      )}

      {/* PageHeader — separate floating surface */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <UserOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Daftar Warga</div>
            <div className="sf-page-header-subtitle">
              {user.rt?.name ? `Manajemen kepala keluarga dan rumah tangga \u2014 ${user.rt.name}` : 'Manajemen kepala keluarga dan rumah tangga'}
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
              placeholder="Cari kepala keluarga/alamat..."
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
                ) : null
              }
            />
          </div>

          {/* Create button — hidden for warga (read-only) */}
          {!isReadOnly && (
            <Link to="/warga/baru" className="sf-create-btn">
              + Tambah
            </Link>
          )}
        </div>

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
        {!loading && !error && householdList.length === 0 && (
          <div className="sf-empty-banner">
            Tidak ada data warga.
          </div>
        )}

        {/* Household Table */}
        {!loading && householdList.length > 0 && (
          <div className="table-wrapper">
            <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th className="col-rt" style={colStyles.rt}>RT</th>
                  <th className="col-rw" style={colStyles.rw}>RW</th>
                  <th className="col-rt-name" style={colStyles.rtName}>NAMA RT</th>
                  <th className="col-house-number" style={colStyles.houseNumber}>NOMOR RUMAH</th>
                  <th className="col-address" style={colStyles.address}>ALAMAT</th>
                  <th className="col-occupancy" style={colStyles.occupancy}>STATUS HUNIAN</th>
                  <th className="col-name" style={colStyles.name}>NAMA</th>
                  {showNik && <th className="col-nik" style={colStyles.nik}>NIK</th>}
                  <th className="col-phone" style={colStyles.phone}>TELEPON</th>
                  <th className="col-email" style={colStyles.email}>EMAIL</th>
                  <th className="col-start-date" style={colStyles.startDate}>TANGGAL MULAI</th>
                  <th className="col-end-date" style={colStyles.endDate}>TANGGAL SELESAI</th>
                  <th className="col-status" style={colStyles.status}>STATUS</th>
                  {showAksi && <th className="col-action" style={colStyles.action}>AKSI</th>}
                </tr>
              </thead>
              <tbody>
                {householdList.map((hh) => (
                  <HouseholdRow
                    key={hh.id}
                    household={hh}
                    onEdit={handleEdit}
                    showNik={showNik}
                    showAksi={showAksi}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && householdList.length > 0 && (
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

interface HouseholdRowProps {
  household: ApiHousehold
  onEdit: (hh: ApiHousehold) => void
  showNik: boolean
  showAksi: boolean
}

function HouseholdRow({ household, onEdit, showNik, showAksi }: HouseholdRowProps) {
  const startDateRaw = household.start_date ?? household.head_resident?.start_date ?? household.created_at
  const startDate = formatStrictDate(startDateRaw)

  const endDateRaw = household.end_date ?? household.head_resident?.end_date ?? null
  const endDate = formatStrictDate(endDateRaw)

  return (
    <tr>
      {/* 1. RT */}
      <td className="col-rt" style={colStyles.rt}>
        {formatIntegerOnly(household.head_resident?.rt_number ?? null)}
      </td>

      {/* 2. RW */}
      <td className="col-rw" style={colStyles.rw}>
        {formatIntegerOnly(household.head_resident?.rw ?? null)}
      </td>

      {/* 3. NAMA RT */}
      <td className="col-rt-name" style={colStyles.rtName}>
        {formatNull(household.head_resident?.rt_name ?? null)}
      </td>

      {/* 4. NOMOR RUMAH */}
      <td className="col-house-number" style={colStyles.houseNumber}>
        {formatNull(household.house_number)}
      </td>

      {/* 5. ALAMAT */}
      <td className="col-address" style={colStyles.address}>
        {formatNull(household.address)}
      </td>

      {/* 6. STATUS HUNIAN */}
      <td className="col-occupancy" style={colStyles.occupancy}>
        <Badge variant={occupancyBadgeVariant(household.occupancy_status)}>
          {occupancyLabel(household.occupancy_status)}
        </Badge>
      </td>

      {/* 7. NAMA */}
      <td className="col-name" style={{ ...colStyles.name, fontWeight: 600 }}>
        {household.head_name}
      </td>

      {/* 8. NIK */}
      {showNik && (
        <td className="col-nik" style={colStyles.nik}>
          {formatNull(household.nik ?? household.head_resident?.nik ?? null)}
        </td>
      )}

      {/* 9. TELEPON */}
      <td className="col-phone" style={colStyles.phone}>
        {formatNull(household.phone ?? household.head_resident?.phone ?? null)}
      </td>

      {/* 10. EMAIL */}
      <td className="col-email" style={colStyles.email}>
        {formatNull(household.email ?? household.head_resident?.email ?? null)}
      </td>

      {/* 11. TANGGAL MULAI */}
      <td className="col-start-date" style={colStyles.startDate}>
        {startDate}
      </td>

      {/* 12. TANGGAL SELESAI */}
      <td className="col-end-date" style={colStyles.endDate}>
        {endDate}
      </td>

      {/* 13. STATUS */}
      <td className="col-status" style={colStyles.status}>
        <Badge variant={statusBadge(household.is_active)}>
          {statusLabel(household.is_active)}
        </Badge>
      </td>

      {/* 14. AKSI */}
      {showAksi && (
        <td className="col-action" style={colStyles.action}>
          <RowActionMenu
            items={[
              { label: 'Ubah', onClick: () => onEdit(household) },
            ]}
          />
        </td>
      )}
    </tr>
  )
}
