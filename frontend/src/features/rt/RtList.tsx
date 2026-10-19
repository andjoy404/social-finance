import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiListRTs,
  apiListRWs,
  apiCreateRW,
  apiUpdateRW,
  apiDeactivateRW,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiRT,
  type ApiRW,
} from '@/app/api'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { ApartmentOutlined, CloseOutlined, DownOutlined } from '@ant-design/icons'
import { RtCreate } from './RtCreate'
import { RtDetailModal } from './RtDetailModal'

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

const dangerBtnStyle: React.CSSProperties = {
  background: 'rgba(242, 73, 92, 0.16)',
  border: '1px solid rgba(242, 73, 92, 0.6)',
  color: 'var(--sf-danger)',
}

function statusBadgeForRow(value: boolean): 'green' | 'red' {
  return value ? 'green' : 'red'
}

function statusLabel(value: boolean): string {
  return value ? 'Aktif' : 'Tidak Aktif'
}

function formatRwDisplay(rt: ApiRT): string {
  if (rt.rw_name && rt.rw_name.trim()) return rt.rw_name
  return `RW ${String(rt.rw).padStart(3, '0')}`
}

export function RtList() {
  const { user } = useAuth()
  if (!user) return null

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [rtList, setRtList] = useState<ApiRT[]>([])
  const [rwList, setRwList] = useState<ApiRW[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:rt', [10, 25, 50, 100])
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [search, setSearch] = useState('')
  const [statusValue, setStatusValue] = useState<StatusValue>(null)
  const [statusOpen, setStatusOpen] = useState(false)

  const [detailRt, setDetailRt] = useState<ApiRT | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [rwManageOpen, setRwManageOpen] = useState(false)

  const handleCreate = () => {
    setCreateOpen(true)
  }

  const fetchRws = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) return
    try {
      const resp = await apiListRWs(pair.accessToken, { page_size: 100 })
      if (Array.isArray(resp?.data)) {
        setRwList(resp.data)
      }
    } catch {
      /* ignore if RW endpoint unavailable */
    }
  }, [])

  const fetchRts = useCallback(async (p: number, size: number) => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError(new ApiError('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
      return
    }

    setLoading(true)
    setError(null)

    try {
      const params: { page: number; page_size: number; is_active?: boolean; search?: string; rw_id?: string } = {
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
    void fetchRws()
  }, [fetchRws])

  useEffect(() => {
    void fetchRts(page, pageSize)
  }, [page, pageSize, fetchRts])

  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    if (filter !== 'status') setStatusValue(null)
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

  const statusValueLabel: string | null = statusValue === 'aktif' ? 'Aktif' : statusValue === 'tidak_aktif' ? 'Tidak Aktif' : null

  return (
    <>
      {detailRt && (
        <RtDetailModal
          open={!!detailRt}
          rt={detailRt}
          onClose={() => setDetailRt(null)}
          onUpdated={(updatedRt) => {
            setDetailRt(updatedRt)
            void fetchRts(page, pageSize)
          }}
        />
      )}

      {rwManageOpen && (
        <RwManageModal
          open={rwManageOpen}
          rwList={rwList}
          onClose={() => setRwManageOpen(false)}
          onChanged={() => {
            void fetchRws()
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
            <div className="sf-page-header-subtitle">Manajemen unit wilayah administratif RT &amp; RW</div>
          </div>
        </div>
      </div>

      {/* Content surface — separate floating surface with gap from PageHeader */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            <FilterDropdown
              value={filterType}
              options={statusFilterOptions}
              onChange={handleFilter}
              ariaLabel="Tipe filter"
            />

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
          </div>

          <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
            <button
              type="button"
              className="sf-create-btn"
              onClick={handleCreate}
            >
              + RT
            </button>
          </div>
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
            <table style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th style={{ width: '1%', whiteSpace: 'nowrap' }}>Kode RT</th>
                  <th>Nama RT</th>
                  <th>RW</th>
                  <th>Ketua RT</th>
                  <th style={{ width: '1%', whiteSpace: 'nowrap' }}>Status</th>
                </tr>
              </thead>
              <tbody>
                {rtList.map((rt) => (
                  <RtRow
                    key={rt.id}
                    rt={rt}
                    onDetail={setDetailRt}
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
            pageSizeOptions={[10, 25, 50, 100]}
          />
        )}

        {/* RtCreate Modal — after table and pagination */}
        {createOpen && (
          <RtCreate
            open={createOpen}
            onClose={() => setCreateOpen(false)}
            onSaved={() => {
              setCreateOpen(false)
              void fetchRts(page, pageSize)
              void fetchRws()
            }}
          />
        )}
      </div>
    </>
  )
}

interface RtRowProps {
  rt: ApiRT
  onDetail: (rt: ApiRT) => void
}

function RtRow({ rt, onDetail }: RtRowProps) {
  return (
    <tr
      onClick={() => onDetail(rt)}
      style={{ cursor: 'pointer' }}
      title={`Klik untuk melihat detail ${rt.name}`}
    >
      <td style={{ width: '1%', whiteSpace: 'nowrap', fontFamily: 'monospace' }}>{rt.rt}</td>
      <td>{rt.name}</td>
      <td style={{ fontFamily: 'monospace' }}>{formatRwDisplay(rt)}</td>
      <td>{rt.head_name ?? '\u2014'}</td>
      <td style={{ width: '1%', whiteSpace: 'nowrap' }}>
        <Badge variant={statusBadgeForRow(rt.is_active)}>
          {statusLabel(rt.is_active)}
        </Badge>
      </td>
    </tr>
  )
}

/* ── RW Management Modal ──────────────────────────────────────────────── */

function RwManageModal({
  open,
  rwList,
  onClose,
  onChanged,
}: {
  open: boolean
  rwList: ApiRW[]
  onClose: () => void
  onChanged: () => void
}) {
  const [editingRw, setEditingRw] = useState<ApiRW | null>(null)
  const [rwNumber, setRwNumber] = useState('')
  const [rwName, setRwName] = useState('')
  const [rwHead, setRwHead] = useState('')
  const [rwAddress, setRwAddress] = useState('')
  const [saving, setSaving] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const resetForm = () => {
    setEditingRw(null)
    setRwNumber('')
    setRwName('')
    setRwHead('')
    setRwAddress('')
    setErrorMsg(null)
  }

  const handleSelectEdit = (rw: ApiRW) => {
    setEditingRw(rw)
    setRwNumber(String(rw.rw_number))
    setRwName(rw.name)
    setRwHead(rw.head_name ?? '')
    setRwAddress(rw.address ?? '')
    setErrorMsg(null)
  }

  const handleSaveRw = async (e: React.FormEvent) => {
    e.preventDefault()
    setErrorMsg(null)
    const num = Number(rwNumber)
    if (!rwNumber || !Number.isInteger(num) || num <= 0) {
      setErrorMsg('Nomor RW harus angka bulat positif.')
      return
    }
    const pair = getSessionPair()
    if (!pair?.accessToken) return

    setSaving(true)
    try {
      if (editingRw) {
        await apiUpdateRW(pair.accessToken, editingRw.id, {
          rw_number: num,
          name: rwName.trim() || `RW ${String(num).padStart(3, '0')}`,
          head_name: rwHead.trim() || null,
          address: rwAddress.trim() || null,
        })
      } else {
        await apiCreateRW(pair.accessToken, {
          rw_number: num,
          name: rwName.trim() || `RW ${String(num).padStart(3, '0')}`,
          head_name: rwHead.trim() || null,
          address: rwAddress.trim() || null,
        })
      }
      resetForm()
      onChanged()
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorMsg(err.message || 'Gagal menyimpan RW.')
      } else {
        setErrorMsg('Gagal menyimpan RW.')
      }
    } finally {
      setSaving(false)
    }
  }

  const handleToggleRwActive = async (rw: ApiRW) => {
    setErrorMsg(null)
    const pair = getSessionPair()
    if (!pair?.accessToken) return
    setSaving(true)
    try {
      if (rw.is_active) {
        await apiDeactivateRW(pair.accessToken, rw.id)
      } else {
        await apiUpdateRW(pair.accessToken, rw.id, { is_active: true })
      }
      onChanged()
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorMsg(err.message || 'Gagal mengubah status RW.')
      } else {
        setErrorMsg('Gagal mengubah status RW.')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} width={580} overlayClassName="sf-modal-overlay-rt">
      <ModalHeader
        title="Kelola RW"
        subtitle="Daftar dan pengaturan Rukun Warga (RW)"
        onClose={onClose}
      />
      <ModalBody>
        {errorMsg && (
          <div className="sf-form-error-list" style={{ marginBottom: 12 }}>
            {errorMsg}
          </div>
        )}

        <form onSubmit={handleSaveRw} style={{ marginBottom: 16, paddingBottom: 16, borderBottom: '1px solid var(--sf-border)' }}>
          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10, color: 'var(--sf-text-primary)' }}>
            {editingRw ? `Ubah ${editingRw.name}` : 'Tambah RW Baru'}
          </div>
          <div className="sf-form-grid-2">
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="rw-number">
                Nomor RW <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="rw-number"
                className="sf-form-input"
                type="number"
                min="1"
                value={rwNumber}
                onChange={(e) => setRwNumber(e.target.value)}
                placeholder="Contoh: 16"
                required
                disabled={saving}
              />
            </div>
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="rw-name">
                Nama RW
              </label>
              <input
                id="rw-name"
                className="sf-form-input"
                type="text"
                value={rwName}
                onChange={(e) => setRwName(e.target.value)}
                placeholder="Contoh: RW 016"
                disabled={saving}
              />
            </div>
          </div>
          <div className="sf-form-grid-2">
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="rw-head">
                Ketua RW
              </label>
              <input
                id="rw-head"
                className="sf-form-input"
                type="text"
                value={rwHead}
                onChange={(e) => setRwHead(e.target.value)}
                placeholder="Nama Ketua RW"
                disabled={saving}
              />
            </div>
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="rw-address">
                Alamat / Wilayah
              </label>
              <input
                id="rw-address"
                className="sf-form-input"
                type="text"
                value={rwAddress}
                onChange={(e) => setRwAddress(e.target.value)}
                placeholder="Wilayah RW"
                disabled={saving}
              />
            </div>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 }}>
            {editingRw && (
              <button
                type="button"
                className="sf-btn-ghost"
                style={dangerBtnStyle}
                onClick={resetForm}
                disabled={saving}
              >
                Batal Ubah
              </button>
            )}
            <button type="submit" className="sf-btn-primary" disabled={saving}>
              {saving ? 'Menyimpan...' : editingRw ? 'Simpan RW' : '+ Tambah RW'}
            </button>
          </div>
        </form>

        <div className="table-wrapper" style={{ maxHeight: 240, overflowY: 'auto' }}>
          <table>
            <thead>
              <tr>
                <th>RW</th>
                <th>Ketua RW</th>
                <th>Jumlah RT</th>
                <th>Status</th>
                <th>Aksi</th>
              </tr>
            </thead>
            <tbody>
              {rwList.length === 0 ? (
                <tr>
                  <td colSpan={5} style={{ textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                    Belum ada data RW.
                  </td>
                </tr>
              ) : (
                rwList.map((rw) => (
                  <tr key={rw.id}>
                    <td style={{ fontFamily: 'monospace', fontWeight: 600 }}>
                      {rw.name || `RW ${String(rw.rw_number).padStart(3, '0')}`}
                    </td>
                    <td>{rw.head_name || '—'}</td>
                    <td>{rw.active_rt_count ?? rw.rt_count ?? 0} RT</td>
                    <td>
                      <Badge variant={statusBadgeForRow(rw.is_active)}>
                        {statusLabel(rw.is_active)}
                      </Badge>
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 6 }}>
                        <button
                          type="button"
                          className="sf-btn-ghost"
                          style={{ padding: '2px 8px', fontSize: 12 }}
                          onClick={() => handleSelectEdit(rw)}
                          disabled={saving}
                        >
                          Ubah
                        </button>
                        <button
                          type="button"
                          className="sf-btn-ghost"
                          style={{
                            padding: '2px 8px',
                            fontSize: 12,
                            ...(rw.is_active ? dangerBtnStyle : {}),
                          }}
                          onClick={() => handleToggleRwActive(rw)}
                          disabled={saving}
                        >
                          {rw.is_active ? 'Nonaktifkan' : 'Aktifkan'}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </ModalBody>
      <ModalFooter>
        <button type="button" className="sf-btn-ghost" style={dangerBtnStyle} onClick={onClose}>
          Tutup
        </button>
      </ModalFooter>
    </Modal>
  )
}
