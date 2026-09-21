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
import { UserOutlined, CloseOutlined, DownOutlined, EditOutlined } from '@ant-design/icons'
import { HouseholdEdit } from './HouseholdEdit'

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

function statusBadge(value: boolean): 'green' | 'red' {
  return value ? 'green' : 'red'
}

function statusLabel(value: boolean): string {
  return value ? 'Aktif' : 'Tidak Aktif'
}

function formatNull(value: string | null): string {
  return value ?? '\u2014'
}

export function HouseholdList() {
  const { user } = useAuth()
  if (!user) return null

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [householdList, setHouseholdList] = useState<ApiHousehold[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:warga', [10, 25, 50])
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

  const start = total === 0 ? 0 : (page - 1) * pageSize + 1

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
            <div className="sf-page-header-subtitle">Manajemen kepala keluarga dan rumah tangga</div>
          </div>
        </div>
      </div>

      {/* Content surface */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar">
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

          {/* Create button */}
          <Link to="/warga/baru" className="sf-create-btn" style={{ marginLeft: 'auto' }}>
            + Tambah
          </Link>
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
            <table>
              <thead>
                <tr>
                  <th>No</th>
                  <th>Nomor Rumah</th>
                  <th>Kepala Keluarga</th>
                  <th>NIK</th>
                  <th>Status Hunian</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {householdList.map((hh, i) => (
                  <HouseholdRow
                    key={hh.id}
                    index={start + i}
                    household={hh}
                    onEdit={handleEdit}
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
            pageSizeOptions={[10, 25, 50]}
          />
        )}
      </div>
    </>
  )
}

interface HouseholdRowProps {
  index: number
  household: ApiHousehold
  onEdit: (hh: ApiHousehold) => void
}

function HouseholdRow({ index, household, onEdit }: HouseholdRowProps) {
  return (
    <tr>
      <td>{index}</td>
      <td style={{ fontFamily: 'monospace' }}>{formatNull(household.house_number)}</td>
      <td style={{ fontWeight: 600 }}>{household.head_name}</td>
      <td style={{ fontFamily: 'monospace' }}>{formatNull(household.nik ?? household.head_resident?.nik ?? null)}</td>
      <td>{occupancyLabel(household.occupancy_status)}</td>
      <td style={{ textAlign: 'right', display: 'flex', gap: '4px', justifyContent: 'flex-end' }}>
        <Badge variant={statusBadge(household.is_active)}>
          {statusLabel(household.is_active)}
        </Badge>
        <button
          type="button"
          onClick={() => onEdit(household)}
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--sf-text-muted)',
            cursor: 'pointer',
            padding: '0 2px',
            display: 'inline-flex',
            alignItems: 'center',
          }}
        >
          <EditOutlined style={{ fontSize: 14 }} />
        </button>
      </td>
    </tr>
  )
}
