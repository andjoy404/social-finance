import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiListRWs,
  apiCreateRW,
  apiUpdateRW,
  apiDeactivateRW,
  ApiError,
  getSessionPair,
  type ApiRW,
} from '@/app/api'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { CloseOutlined, DownOutlined } from '@ant-design/icons'
import { RwDetailModal } from './RwDetailModal'

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

// ─── RwManageModal (extracted from RtList) ───────────────────────────

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
                placeholder="Nama ketua RW"
                disabled={saving}
              />
            </div>
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="rw-address">
                Alamat
              </label>
              <input
                id="rw-address"
                className="sf-form-input"
                type="text"
                value={rwAddress}
                onChange={(e) => setRwAddress(e.target.value)}
                placeholder="Alamat RW"
                disabled={saving}
              />
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
            <button
              type="submit"
              className="sf-btn sf-btn-primary sf-btn-sm"
              disabled={saving}
            >
              {saving ? 'Menyimpan...' : editingRw ? 'Simpan Perubahan' : 'Tambah RW'}
            </button>
            {editingRw && (
              <button
                type="button"
                className="sf-btn sf-btn-ghost sf-btn-sm"
                onClick={resetForm}
                disabled={saving}
              >
                Batal Edit
              </button>
            )}
          </div>
        </form>

        <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, color: 'var(--sf-text-primary)' }}>
          Daftar RW ({rwList.length})
        </div>
        <div style={{ maxHeight: 320, overflowY: 'auto' }}>
          {rwList.length === 0 ? (
            <div style={{ padding: 16, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
              Belum ada RW terdaftar.
            </div>
          ) : (
            rwList.map((rw) => (
              <div
                key={rw.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '8px 0',
                  borderBottom: '1px solid var(--sf-border)',
                }}
              >
                <div>
                  <div style={{ fontWeight: 600, fontSize: 13 }}>
                    {rw.name} (RW {String(rw.rw_number).padStart(3, '0')})
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                    {rw.head_name ? `Ketua: ${rw.head_name}` : 'Belum ada ketua'}
                    {rw.address ? ` · ${rw.address}` : ''}
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                  <Badge variant={rw.is_active ? 'green' : 'red'}>
                    {rw.is_active ? 'Aktif' : 'Nonaktif'}
                  </Badge>
                  <button
                    className="sf-btn sf-btn-ghost sf-btn-sm"
                    onClick={() => handleSelectEdit(rw)}
                    disabled={saving}
                  >
                    Edit
                  </button>
                  <button
                    className="sf-btn sf-btn-ghost sf-btn-sm"
                    onClick={() => void handleToggleRwActive(rw)}
                    disabled={saving}
                  >
                    {rw.is_active ? 'Nonaktifkan' : 'Aktifkan'}
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      </ModalBody>
      <ModalFooter>
        <button className="sf-btn sf-btn-ghost" onClick={onClose}>
          Tutup
        </button>
      </ModalFooter>
    </Modal>
  )
}

// ─── Main RwList Page ────────────────────────────────────────────────

export function RwList() {
  const { user } = useAuth()
  const isSuperAdmin = user?.systemRole === 'super_admin'

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [rwList, setRwList] = useState<ApiRW[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:rw', [10, 25, 50, 100])
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [search, setSearch] = useState('')
  const [statusValue, setStatusValue] = useState<StatusValue>(null)
  const [statusOpen, setStatusOpen] = useState(false)
  const [rwManageOpen, setRwManageOpen] = useState(false)
  const [detailRw, setDetailRw] = useState<ApiRW | null>(null)

  const fetchRws = useCallback(async (p: number, size: number) => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError('Sesi telah berakhir. Silakan masuk kembali.')
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

      const resp = await apiListRWs(pair.accessToken, params)

      setRwList(resp.data ?? [])
      setTotalPages(resp.pagination?.total_pages ?? 1)
      setTotal(resp.pagination?.total ?? 0)
    } catch (err) {
      if (err instanceof ApiError && (err.code === 'forbidden' || err.code === 'unauthorized')) {
        setError('Anda tidak memiliki akses ke halaman ini.')
      } else {
        setError('Gagal memuat data RW.')
      }
    } finally {
      setLoading(false)
    }
  }, [filterType, statusValue, search])

  useEffect(() => {
    void fetchRws(page, pageSize)
  }, [page, pageSize, fetchRws])

  const handleSearch = (value: string) => {
    setSearch(value)
    setPage(1)
  }

  const handleStatusSelect = (val: 'aktif' | 'tidak_aktif' | null) => {
    setStatusValue(val)
    setStatusOpen(false)
    setPage(1)
  }

  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    if (filter !== 'status') setStatusValue(null)
  }

  const handleStatusClear = () => {
    setStatusValue(null)
  }

  const handlePage = (p: number) => {
    if (p >= 1 && p <= totalPages) setPage(p)
  }

  const handlePageSizeChange = (newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }

  const statusValueLabel: string | null =
    statusValue === 'aktif' ? 'Aktif' : statusValue === 'tidak_aktif' ? 'Tidak Aktif' : null

  if (!isSuperAdmin) {
    return (
      <div className="sf-page">
        <div className="sf-card" style={{ padding: 24 }}>
          <h2>Akses Ditolak</h2>
          <p>Halaman ini hanya dapat diakses oleh Super Admin.</p>
        </div>
      </div>
    )
  }

  return (
    <div className="sf-page">
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Daftar RW</div>
            <div className="sf-page-header-subtitle">Manajemen unit wilayah administratif RW</div>
          </div>
        </div>
        <button
          className="sf-btn sf-btn-primary"
          onClick={() => setRwManageOpen(true)}
        >
          Kelola RW
        </button>
      </div>

      <div className="sf-content-surface">
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
              placeholder="Cari RW..."
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
                            {statusValueOptions.map((option) => (
                              <button
                                key={option.value}
                                type="button"
                                className={`sf-status-dropdown-item ${statusValue === option.value ? 'sf-status-dropdown-item-active' : ''}`}
                                onClick={() => handleStatusSelect(option.value)}
                                role="button"
                              >
                                {option.label}
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
        </div>

        {loading && (
          <div className="sf-loading-banner">Memuat data RW...</div>
        )}

        {error && (
          <div className="sf-error-banner" role="alert">{error}</div>
        )}

        {!loading && !error && rwList.length === 0 && (
          <div className="sf-empty-banner">Belum ada data RW yang terdaftar.</div>
        )}

        {!loading && !error && rwList.length > 0 && (
          <>
            <div className="table-wrapper">
              <table style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
                <thead>
                  <tr>
                    <th style={{ width: '1%', whiteSpace: 'nowrap' }}>Kode RW</th>
                    <th>Nama</th>
                    <th>Ketua</th>
                    <th>Alamat</th>
                    <th style={{ width: '1%', whiteSpace: 'nowrap' }}>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {rwList.map((rw) => (
                    <tr
                      key={rw.id}
                      onClick={() => setDetailRw(rw)}
                      style={{ cursor: 'pointer' }}
                      title={`Klik untuk melihat detail ${rw.name}`}
                    >
                      <td style={{ width: '1%', whiteSpace: 'nowrap', fontFamily: 'monospace' }}>
                        {String(rw.rw_number).padStart(3, '0')}
                      </td>
                      <td>{rw.name}</td>
                      <td>{rw.head_name || '-'}</td>
                      <td>{rw.address || '-'}</td>
                      <td style={{ width: '1%', whiteSpace: 'nowrap' }}>
                        <Badge variant={rw.is_active ? 'green' : 'red'}>
                          {rw.is_active ? 'Aktif' : 'Nonaktif'}
                        </Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <Pagination
              page={page}
              totalPages={totalPages}
              total={total}
              pageSize={pageSize}
              onPageChange={handlePage}
              onPageSizeChange={handlePageSizeChange}
            />
          </>
        )}
      </div>

      {detailRw && (
        <RwDetailModal
          open={!!detailRw}
          rw={detailRw}
          onClose={() => setDetailRw(null)}
          onUpdated={(updatedRw) => {
            setDetailRw(updatedRw)
            void fetchRws(page, pageSize)
          }}
        />
      )}

      {rwManageOpen && (
        <RwManageModal
          open={rwManageOpen}
          rwList={rwList}
          onClose={() => setRwManageOpen(false)}
          onChanged={() => void fetchRws(page, pageSize)}
        />
      )}
    </div>
  )
}
