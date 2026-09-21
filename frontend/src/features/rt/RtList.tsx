import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import { apiListRTs, ApiError, clearSessionPair, getSessionPair, type ApiRT } from '@/app/api'
import { Link } from 'react-router-dom'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { ApartmentOutlined, DownOutlined, CloseOutlined, EditOutlined } from '@ant-design/icons'
import { RtEdit } from './RtEdit'

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

function statusBadgeForRow(value: boolean): 'green' | 'red' {
  return value ? 'green' : 'red'
}

function statusLabel(value: boolean): string {
  return value ? 'Aktif' : 'Tidak Aktif'
}

export function RtList() {
  const { user } = useAuth()
  if (!user) return null

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [rtList, setRtList] = useState<ApiRT[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [search, setSearch] = useState('')
  const [statusValue, setStatusValue] = useState<StatusValue>(null)
  const [statusOpen, setStatusOpen] = useState(false)

  const [editingRt, setEditingRt] = useState<ApiRT | null>(null)

  const fetchRts = useCallback(async (p: number, size: number) => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError(new ApiError('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
      return
    }

    setLoading(true)
    setError(null)

    try {
      const params: { page: number; page_size: number; is_active?: boolean; search?: string } = {
        page: p,
        page_size: size,
      }

      if (filterType === 'status' && statusValue === 'aktif') params.is_active = true
      else if (filterType === 'status' && statusValue === 'tidak_aktif') params.is_active = false
      if (search.trim()) params.search = search.trim()

      const resp = await apiListRTs(pair.accessToken, params)

      setRtList(resp.data)
      setTotalPages(resp.pagination.total_pages)
      setTotal(resp.pagination.total)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        window.location.href = '/login'
      } else if (err instanceof ApiError && (err.code === 'forbidden' || err.code === 'unauthorized')) {
        setError(new ApiError('access_denied', 'Anda tidak memiliki akses ke halaman ini.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal memuat data RT.'))
      }
    } finally {
      setLoading(false)
    }
  }, [filterType, statusValue, search])

  useEffect(() => {
    void fetchRts(page, pageSize)
  }, [page, pageSize, fetchRts])

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

  const handleEdit = (rt: ApiRT) => {
    setEditingRt(rt)
  }

  const handleStatusSelect = (val: StatusValue) => {
    setStatusValue(val)
    setStatusOpen(false)
    setPage(1)
  }

  const handleStatusClear = () => {
    setStatusValue(null)
  }

  const start = total === 0 ? 0 : (page - 1) * pageSize + 1

  const statusValueLabel: string | null = statusValue === 'aktif' ? 'Aktif' : statusValue === 'tidak_aktif' ? 'Tidak Aktif' : null

  return (
    <>
      {editingRt && (
        <RtEdit
          id={editingRt.id}
          onClose={() => setEditingRt(null)}
          onSaved={() => {
            setEditingRt(null)
            void fetchRts(page, pageSize)
          }}
        />
      )}

      {/* PageHeader — separate floating surface */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <ApartmentOutlined style={{ fontSize: 16, color: 'var(--sf-accent)' }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Daftar RT</div>
            <div className="sf-page-header-subtitle">Manajemen unit wilayah administratif</div>
          </div>
        </div>
      </div>

      {/* Content surface — separate floating surface with gap from PageHeader */}
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
            onChange={handleSearch}
            placeholder="Cari RT..."
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

          {/* Add RT button */}
          <Link
            to="/rt/new"
            className="sf-create-btn"
            style={{ marginLeft: 'auto' }}
          >
            + Tambah RT
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
            Memuat data RT...
          </div>
        )}

        {/* Empty */}
        {!loading && !error && rtList.length === 0 && (
          <div className="sf-empty-banner">
            Tidak ada data RT.
          </div>
        )}

        {/* RT Table */}
        {!loading && rtList.length > 0 && (
          <div className="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>No</th>
                  <th>Nama RT</th>
                  <th>RW</th>
                  <th>Kode RT</th>
                  <th>Ketua RT</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {rtList.map((rt, i) => (
                  <RtRow
                    key={rt.id}
                    index={start + i}
                    rt={rt}
                    onEdit={handleEdit}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && rtList.length > 0 && (
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

interface RtRowProps {
  index: number
  rt: ApiRT
  onEdit: (rt: ApiRT) => void
}

function RtRow({ index, rt, onEdit }: RtRowProps) {
  return (
    <tr>
      <td>{index}</td>
      <td>{rt.name}</td>
      <td style={{ textAlign: 'right' }}>{rt.rw}</td>
      <td style={{ fontFamily: 'monospace', textAlign: 'right' }}>{rt.rt}</td>
      <td>{rt.head_name ?? '\u2014'}</td>
      <td style={{ textAlign: 'right', display: 'flex', gap: '4px', justifyContent: 'flex-end' }}>
        <Badge variant={statusBadgeForRow(rt.is_active)}>
          {statusLabel(rt.is_active)}
        </Badge>
        <button
          type="button"
          onClick={() => onEdit(rt)}
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
