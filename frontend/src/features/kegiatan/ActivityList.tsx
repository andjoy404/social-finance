import { useState, useEffect, useCallback, useMemo } from 'react'
import { useAuth, canManageActivities } from '@/app/AuthContext'
import { Modal } from 'antd'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { Pagination } from '@/components/Pagination'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import {
  CalendarOutlined,
  PlusOutlined,
  ClockCircleOutlined,
  EnvironmentOutlined,
  UserOutlined,
  PictureOutlined,
  CheckOutlined,
  CloseOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  EyeOutlined,
} from '@ant-design/icons'
import {
  CommunityActivity,
  apiListActivities,
  apiPublishActivity,
  apiCancelActivity,
  apiDeleteActivity,
  getSessionPair,
} from '@/app/api'
import { CommunityImage } from '@/components/CommunityImage'
import { ActivityModal } from './ActivityModal'

function formatDateId(date: Date): string {
  return date.toLocaleDateString('id-ID', { dateStyle: 'full' })
}

function formatTimeId(date: Date): string {
  return date.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
}

export function ActivityList() {
  const { user } = useAuth()
  const [activities, setActivities] = useState<CommunityActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [editingActivity, setEditingActivity] = useState<CommunityActivity | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [viewingActivity, setViewingActivity] = useState<CommunityActivity | null>(null)

  // Permission-driven gating (not role-name based).
  const canManage = canManageActivities(user)

  const loadActivities = useCallback(() => {
    const session = getSessionPair()
    if (!session?.accessToken) {
      setLoading(false)
      setError('Sesi tidak ditemukan. Silakan login ulang.')
      return
    }

    setLoading(true)
    setError(null)
    const params: { status?: string; page: number; page_size: number } = { page, page_size: pageSize }
    if (statusFilter !== 'all') {
      params.status = statusFilter
    }

    apiListActivities(session.accessToken, params)
      .then((res) => {
        if ((res.data || []).length === 0 && page > 1 && (res.pagination?.total ?? 0) > 0) {
          setPage((p) => p - 1)
          return
        }
        setActivities(res.data || [])
        setTotal(res.pagination?.total ?? 0)
        setTotalPages(Math.max(1, res.pagination?.total_pages ?? 1))
        setLoading(false)
      })
      .catch((err) => {
        setError(err?.message || 'Gagal memuat kegiatan. Periksa koneksi Anda lalu coba lagi.')
        setActivities([])
        setTotal(0)
        setTotalPages(1)
        setLoading(false)
      })
  }, [statusFilter, page, pageSize])

  useEffect(() => {
    loadActivities()
  }, [loadActivities])

  const handleStatusFilter = (next: string) => {
    setPage(1)
    setStatusFilter(next)
  }

  const handleSearch = (query: string) => {
    setPage(1)
    setSearchQuery(query)
  }

  const handlePublish = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Publikasikan / terbitkan ulang kegiatan ini? Banner Beranda ikut diperbarui.')) return

    try {
      await apiPublishActivity(session.accessToken, id)
      loadActivities()
    } catch (err: any) {
      setError(err?.message || 'Gagal mempublikasikan kegiatan')
    }
  }

  const handleCancel = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Batalkan kegiatan ini? Banner di Beranda akan dinonaktifkan.')) return

    try {
      await apiCancelActivity(session.accessToken, id)
      loadActivities()
    } catch (err: any) {
      setError(err?.message || 'Gagal membatalkan kegiatan')
    }
  }

  const handleDelete = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Hapus kegiatan draf ini? Tindakan ini tidak dapat dibatalkan.')) return

    try {
      await apiDeleteActivity(session.accessToken, id)
      loadActivities()
    } catch (err: any) {
      setError(err?.message || 'Gagal menghapus kegiatan')
    }
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'upcoming':
        return <Badge variant="violet">Mendatang</Badge>
      case 'ongoing':
        return <Badge variant="green">Berlangsung</Badge>
      case 'completed':
        return <Badge variant="gray">Selesai</Badge>
      case 'cancelled':
        return <Badge variant="red">Dibatalkan</Badge>
      default:
        return <Badge variant="default">{status}</Badge>
    }
  }

  const filteredActivities = useMemo(() => {
    let result = activities
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase()
      result = result.filter(
        (a) =>
          a.title.toLowerCase().includes(q) ||
          (a.description && a.description.toLowerCase().includes(q)) ||
          (a.organizer && a.organizer.toLowerCase().includes(q)),
      )
    }
    return result
  }, [activities, searchQuery])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
      <PageHeader
        surface
        icon={<CalendarOutlined />}
        title="Kegiatan & Agenda RT"
        subtitle="Jadwal kegiatan masyarakat, kerja bakti, dan musyawarah warga"
        actions={
          canManage ? (
            <button
              type="button"
              className="sf-btn sf-btn-primary"
              onClick={() => {
                setEditingActivity(null)
                setShowModal(true)
              }}
            >
              <PlusOutlined /> Tambah Kegiatan
            </button>
          ) : undefined
        }
      />

      {/* Content surface */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            {/* Status Filter Dropdown */}
            <FilterDropdown
              value={statusFilter === 'all' ? 'semua' : statusFilter}
              options={[
                { value: 'semua', label: 'Semua Status' },
                { value: 'upcoming', label: 'Mendatang' },
                { value: 'ongoing', label: 'Berlangsung' },
                { value: 'completed', label: 'Selesai' },
                { value: 'cancelled', label: 'Dibatalkan' },
              ]}
              onChange={(v) => handleStatusFilter(v === 'semua' ? 'all' : v)}
              ariaLabel="Status kegiatan"
            />

            {/* Search */}
            <SearchBox
              value={searchQuery}
              onChange={setSearchQuery}
              onSearch={handleSearch}
              placeholder="Cari judul kegiatan..."
            />
          </div>

          <div className="sf-list-toolbar-right">
            <button
              type="button"
              className="sf-btn-secondary"
              onClick={loadActivities}
              style={{ padding: '6px 14px', fontSize: 12, display: 'inline-flex', alignItems: 'center', gap: '6px' }}
            >
              <ReloadOutlined style={{ fontSize: 13 }} />
              Refresh
            </button>
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
            Memuat daftar kegiatan...
          </div>
        )}

        {/* Empty */}
        {!loading && !error && filteredActivities.length === 0 && (
          <div className="sf-empty-banner">
            {searchQuery || statusFilter !== 'all'
              ? 'Tidak ada kegiatan yang sesuai filter.'
              : 'Belum ada kegiatan'}
          </div>
        )}

        {/* Activities Table */}
        {!loading && filteredActivities.length > 0 && (
          <>
            <div className="table-wrapper">
              <table className="sf-warga-table sf-activity-table">
                <thead>
                  <tr>
                    <th className="col-title">WAKTU & LOKASI</th>
                    <th className="col-title">JUDUL & DESKRIPSI</th>
                    <th className="col-organizer">PENYELENGGARA</th>
                    <th className="col-status">STATUS</th>
                    {canManage && <th className="col-action">AKSI</th>}
                  </tr>
                </thead>
                <tbody>
                  {filteredActivities.map((act) => {
                    const start = new Date(act.starts_at)
                    const end = act.ends_at ? new Date(act.ends_at) : null
                    const isDraft = act.publication_status === 'draft'
                    const isCancelled = act.status === 'cancelled'

                    return (
                      <tr
                        key={act.id}
                        onClick={(e) => {
                          if ((e.target as HTMLElement).closest('.sf-btn-outline, .sf-btn-primary')) return
                          setViewingActivity(act)
                        }}
                        style={{ cursor: 'pointer' }}
                      >
                        {/* 1. WAKTU & LOKASI */}
                        <td className="col-time">
                          <div style={{ fontWeight: 600, color: 'var(--sf-text)', marginBottom: 2 }}>
                            {formatDateId(start)}
                          </div>
                          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', display: 'flex', alignItems: 'center', gap: 5 }}>
                            <ClockCircleOutlined style={{ color: 'var(--sf-accent)', fontSize: 12 }} />
                            <span>
                              {formatTimeId(start)} WIB
                              {end && ` - ${formatTimeId(end)} WIB`}
                            </span>
                          </div>
                          {act.location && (
                            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', display: 'flex', alignItems: 'center', gap: 5, marginTop: 2 }}>
                              <EnvironmentOutlined style={{ color: 'var(--sf-accent)', fontSize: 12 }} />
                              <span>{act.location}</span>
                            </div>
                          )}
                        </td>

                        {/* 2. JUDUL & DESKRIPSI */}
                        <td className="col-title">
                          <div style={{ fontWeight: 600, color: 'var(--sf-text)', marginBottom: act.description ? 2 : 0, display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                            <span>{act.title}</span>
                            {act.poster_url && (
                              <PictureOutlined style={{ color: 'var(--sf-accent)', fontSize: 13 }} title="Memiliki lampiran gambar" />
                            )}
                          </div>
                          {act.description && (
                            <div style={{ color: 'var(--sf-text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', fontSize: 12 }}>
                              {act.description}
                            </div>
                          )}
                        </td>

                        {/* 3. PENYELENGGARA */}
                        <td className="col-organizer">
                          {act.organizer ? (
                            <div style={{ fontSize: 12, color: 'var(--sf-text)', display: 'flex', alignItems: 'center', gap: 5, overflow: 'hidden' }}>
                              <UserOutlined style={{ color: 'var(--sf-accent)', fontSize: 12, flexShrink: 0 }} />
                              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>Penyelenggara: {act.organizer}</span>
                            </div>
                          ) : (
                            <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>—</span>
                          )}
                        </td>

                        {/* 4. STATUS */}
                        <td className="col-status">
                          <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap' }}>
                            {getStatusBadge(act.status)}
                            {canManage && isDraft && <Badge variant="amber">DRAF</Badge>}
                            {act.show_on_banner && <Badge variant="blue">BANNER</Badge>}
                          </div>
                        </td>

                        {/* 5. AKSI */}
                        {canManage && (
                          <td className="col-action" onClick={(e) => e.stopPropagation()} style={{ whiteSpace: 'nowrap' }}>
                            <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                              <button
                                type="button"
                                className="sf-btn-outline"
                                onClick={() => setViewingActivity(act)}
                                style={{ padding: '4px 8px', fontSize: 12 }}
                                title="Lihat Detail"
                                aria-label="Lihat Detail"
                              >
                                <EyeOutlined />
                              </button>
                              {(isDraft || (!isCancelled)) && (
                                <button
                                  type="button"
                                  className="sf-btn-outline"
                                  onClick={() => handlePublish(act.id)}
                                  style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-success)' }}
                                  title={isDraft ? 'Publikasikan' : 'Terbitkan ulang (perbarui snapshot banner)'}
                                  aria-label={isDraft ? 'Publikasikan' : 'Terbitkan Ulang'}
                                >
                                  <CheckOutlined />
                                </button>
                              )}
                              {!isCancelled && (
                                <button
                                  type="button"
                                  className="sf-btn-outline"
                                  onClick={() => handleCancel(act.id)}
                                  style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-warning)' }}
                                  title="Batalkan"
                                  aria-label="Batalkan"
                                >
                                  <CloseOutlined />
                                </button>
                              )}
                              <button
                                type="button"
                                className="sf-btn-outline"
                                onClick={() => {
                                  setEditingActivity(act)
                                  setShowModal(true)
                                }}
                                style={{ padding: '4px 8px', fontSize: 12 }}
                                title="Ubah"
                                aria-label="Ubah"
                              >
                                <EditOutlined />
                              </button>
                              {isDraft && (
                                <button
                                  type="button"
                                  className="sf-btn-outline"
                                  onClick={() => handleDelete(act.id)}
                                  style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-danger)' }}
                                  title="Hapus"
                                  aria-label="Hapus"
                                >
                                  <DeleteOutlined />
                                </button>
                              )}
                            </div>
                          </td>
                        )}
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
            {totalPages > 1 && (
              <div className="sf-card" style={{ padding: 0, overflow: 'hidden', borderRadius: 12 }}>
                <Pagination
                  page={page}
                  pageSize={pageSize}
                  total={total}
                  totalPages={totalPages}
                  onPageChange={setPage}
                />
              </div>
            )}
          </>
        )}
      </div>

      {/* Detail Modal */}
      <Modal
        open={!!viewingActivity}
        onCancel={() => setViewingActivity(null)}
        footer={null}
        closable
        width={600}
        styles={{ body: { padding: 0 } }}
        destroyOnClose
      >
        {viewingActivity && (
          <div style={{ padding: '0 24px 24px 24px', color: 'var(--sf-text)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
              <div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                  {getStatusBadge(viewingActivity.status)}
                  {viewingActivity.publication_status === 'draft' && <Badge variant="amber">DRAF</Badge>}
                  {viewingActivity.show_on_banner && <Badge variant="blue">BANNER</Badge>}
                </div>
                <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--sf-text)' }}>
                  {viewingActivity.title}
                </h2>
              </div>
            </div>

            {viewingActivity.poster_url && (
              <div
                style={{
                  width: '100%',
                  maxHeight: 280,
                  borderRadius: 8,
                  overflow: 'hidden',
                  background: 'var(--sf-surface-subtle)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  marginBottom: 16,
                  border: '1px solid var(--sf-border-subtle)',
                }}
              >
                <CommunityImage
                  src={viewingActivity.poster_url}
                  alt={viewingActivity.title}
                  style={{ width: '100%', maxHeight: 280, objectFit: 'contain' }}
                />
              </div>
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 16 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--sf-text)' }}>
                <ClockCircleOutlined style={{ color: 'var(--sf-accent)', fontSize: 14 }} />
                <span>
                  {formatDateId(new Date(viewingActivity.starts_at))} —{' '}
                  {formatTimeId(new Date(viewingActivity.starts_at))} WIB
                  {viewingActivity.ends_at && ` — ${formatTimeId(new Date(viewingActivity.ends_at))} WIB`}
                </span>
              </div>
              {viewingActivity.location && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--sf-text)' }}>
                  <EnvironmentOutlined style={{ color: 'var(--sf-accent)', fontSize: 14 }} />
                  <span>{viewingActivity.location}</span>
                </div>
              )}
              {viewingActivity.organizer && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--sf-text)' }}>
                  <UserOutlined style={{ color: 'var(--sf-accent)', fontSize: 14 }} />
                  <span>Penyelenggara: {viewingActivity.organizer}</span>
                </div>
              )}
            </div>

            {viewingActivity.description && (
              <div style={{ marginBottom: 16 }}>
                <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 4 }}>
                  DESKRIPSI
                </div>
                <p style={{ margin: 0, fontSize: 14, color: 'var(--sf-text)', lineHeight: 1.6, whiteSpace: 'pre-line' }}>
                  {viewingActivity.description}
                </p>
              </div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', paddingTop: 16, borderTop: '1px solid var(--sf-border)' }}>
              <button
                type="button"
                className="sf-btn sf-btn-outline"
                onClick={() => setViewingActivity(null)}
              >
                Tutup
              </button>
            </div>
          </div>
        )}
      </Modal>

      {/* Create / Edit Modal */}
      {showModal && (
        <ActivityModal
          activity={editingActivity}
          onClose={() => {
            setShowModal(false)
            setEditingActivity(null)
          }}
          onSuccess={() => {
            setShowModal(false)
            setEditingActivity(null)
            if (page !== 1) setPage(1)
            else loadActivities()
          }}
        />
      )}
    </div>
  )
}
