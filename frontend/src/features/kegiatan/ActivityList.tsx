import { useState, useEffect, useCallback } from 'react'
import { useAuth, canManageActivities } from '@/app/AuthContext'
import { useNavigate } from 'react-router-dom'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { Pagination } from '@/components/Pagination'
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
} from '@ant-design/icons'
import {
  CommunityActivity,
  apiListActivities,
  apiPublishActivity,
  apiCancelActivity,
  apiDeleteActivity,
  getSessionPair,
} from '@/app/api'
import { ActivityModal } from './ActivityModal'

export function ActivityList() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [activities, setActivities] = useState<CommunityActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [showModal, setShowModal] = useState(false)
  const [editingActivity, setEditingActivity] = useState<CommunityActivity | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)

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

  const handlePublish = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Publikasikan / terbitkan ulang kegiatan ini? Banner Beranda ikut diperbarui.')) return

    setActionError(null)
    try {
      await apiPublishActivity(session.accessToken, id)
      loadActivities()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal mempublikasikan kegiatan')
    }
  }

  const handleCancel = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Batalkan kegiatan ini? Banner di Beranda akan dinonaktifkan.')) return

    setActionError(null)
    try {
      await apiCancelActivity(session.accessToken, id)
      loadActivities()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal membatalkan kegiatan')
    }
  }

  const handleDelete = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Hapus kegiatan draf ini? Tindakan ini tidak dapat dibatalkan.')) return

    setActionError(null)
    try {
      await apiDeleteActivity(session.accessToken, id)
      loadActivities()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal menghapus kegiatan')
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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
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

      {/* Filter Tabs */}
      <div className="sf-list-toolbar" style={{ marginTop: 0, marginBottom: 0 }}>
        <div className="sf-list-toolbar-left" style={{ flexWrap: 'wrap', gap: 8 }}>
          {[
            { id: 'all', label: 'Semua Status' },
            { id: 'upcoming', label: 'Mendatang' },
            { id: 'ongoing', label: 'Berlangsung' },
            { id: 'completed', label: 'Selesai' },
            { id: 'cancelled', label: 'Dibatalkan' },
          ].map((tab) => (
            <button
              key={tab.id}
              type="button"
              className={`sf-btn ${statusFilter === tab.id ? 'sf-btn-primary' : 'sf-btn-outline'}`}
              onClick={() => handleStatusFilter(tab.id)}
              style={{ padding: '6px 14px', fontSize: 13 }}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {actionError && (
        <div
          role="alert"
          className="sf-card"
          style={{ padding: '10px 16px', borderColor: 'rgba(239, 68, 68, 0.45)', color: 'var(--sf-danger)', fontSize: 13 }}
        >
          {actionError}
        </div>
      )}

      {/* Activities Grid */}
      {loading ? (
        <div className="sf-card" style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
          Memuat daftar kegiatan...
        </div>
      ) : error ? (
        <div className="sf-card" style={{ padding: 48, textAlign: 'center' }}>
          <CalendarOutlined style={{ fontSize: 32, marginBottom: 12, color: 'var(--sf-text-muted)' }} />
          <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-text)' }}>Gagal memuat kegiatan</div>
          <div style={{ fontSize: 13, marginTop: 4, color: 'var(--sf-text-muted)' }}>{error}</div>
          <button type="button" className="sf-btn sf-btn-outline" style={{ marginTop: 16 }} onClick={loadActivities}>
            <ReloadOutlined /> Coba Lagi
          </button>
        </div>
      ) : activities.length > 0 ? (
        <>
        <div
          style={{
            overflowX: 'auto',
            background: 'var(--sf-card-bg)',
            border: '1px solid var(--sf-border)',
            borderRadius: 8,
          }}
        >
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--sf-border)', background: 'rgba(0,0,0,0.02)' }}>
                <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--sf-text-muted)' }}>Waktu & Lokasi</th>
                <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--sf-text-muted)' }}>Judul & Deskripsi</th>
                <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--sf-text-muted)' }}>Penyelenggara</th>
                <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--sf-text-muted)' }}>Status</th>
                {canManage && (
                  <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--sf-text-muted)', textAlign: 'right' }}>Aksi</th>
                )}
              </tr>
            </thead>
            <tbody>
              {activities.map((act) => {
                const start = new Date(act.starts_at)
                const end = act.ends_at ? new Date(act.ends_at) : null
                const isDraft = act.publication_status === 'draft'
                const isCancelled = act.status === 'cancelled'

                return (
                  <tr
                    key={act.id}
                    role="link"
                    aria-label={`Detail kegiatan ${act.title}`}
                    tabIndex={0}
                    onClick={() => navigate(`/kegiatan/${act.id}`)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') navigate(`/kegiatan/${act.id}`)
                    }}
                    style={{
                      borderBottom: '1px solid var(--sf-border)',
                      cursor: 'pointer',
                    }}
                  >
                    <td style={{ padding: '14px 16px', verticalAlign: 'top', width: '25%' }}>
                      <div style={{ fontWeight: 600, color: 'var(--sf-text)', marginBottom: 2 }}>
                        {start.toLocaleDateString('id-ID', { dateStyle: 'full' })}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', display: 'flex', alignItems: 'center', gap: 5 }}>
                        <ClockCircleOutlined style={{ color: 'var(--sf-accent)', fontSize: 12 }} />
                        <span>
                          {start.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB
                          {end && ` - ${end.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB`}
                        </span>
                      </div>
                      {act.location && (
                        <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', display: 'flex', alignItems: 'center', gap: 5, marginTop: 2 }}>
                          <EnvironmentOutlined style={{ color: 'var(--sf-accent)', fontSize: 12 }} />
                          <span>{act.location}</span>
                        </div>
                      )}
                    </td>

                    <td style={{ padding: '14px 16px', verticalAlign: 'top', maxWidth: 380 }}>
                      <div style={{ fontWeight: 600, color: 'var(--sf-text)', marginBottom: act.description ? 2 : 0, display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                        <span>{act.title}</span>
                        {act.poster_url && (
                          <PictureOutlined style={{ color: 'var(--sf-accent)', fontSize: 13 }} title="Memiliki lampiran gambar" />
                        )}
                      </div>
                      {act.description && (
                        <div style={{ color: 'var(--sf-text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {act.description}
                        </div>
                      )}
                    </td>

                    <td style={{ padding: '14px 16px', verticalAlign: 'top' }}>
                      {act.organizer ? (
                        <div style={{ fontSize: 12, color: 'var(--sf-text)', display: 'flex', alignItems: 'center', gap: 5 }}>
                          <UserOutlined style={{ color: 'var(--sf-accent)', fontSize: 12 }} />
                          <span>Penyelenggara: {act.organizer}</span>
                        </div>
                      ) : (
                        <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>—</span>
                      )}
                    </td>

                    <td style={{ padding: '14px 16px', verticalAlign: 'top', whiteSpace: 'nowrap' }}>
                      <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap' }}>
                        {getStatusBadge(act.status)}
                        {canManage && isDraft && <Badge variant="amber">DRAF</Badge>}
                        {act.show_on_banner && <Badge variant="blue">BANNER</Badge>}
                      </div>
                    </td>

                    {canManage && (
                      <td
                        style={{ padding: '14px 16px', verticalAlign: 'top', textAlign: 'right' }}
                        onClick={(e) => e.stopPropagation()}
                      >
                        <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
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
      ) : (
        <div className="sf-card" style={{ padding: 48, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
          <CalendarOutlined style={{ fontSize: 32, marginBottom: 12, color: 'var(--sf-text-muted)' }} />
          <div style={{ fontSize: 15, fontWeight: 600 }}>Belum ada kegiatan</div>
          <div style={{ fontSize: 13, marginTop: 4 }}>
            {canManage ? 'Klik tombol "Tambah Kegiatan" untuk membuat agenda baru.' : 'Belum ada agenda yang dijadwalkan oleh pengurus.'}
          </div>
        </div>
      )}

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
