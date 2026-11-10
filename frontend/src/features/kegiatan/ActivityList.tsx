import { useState, useEffect, useCallback } from 'react'
import { useAuth, canManageActivities } from '@/app/AuthContext'
import { useNavigate } from 'react-router-dom'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { CommunityImage } from '@/components/CommunityImage'
import { Pagination } from '@/components/Pagination'
import {
  CalendarOutlined,
  PlusOutlined,
  ClockCircleOutlined,
  EnvironmentOutlined,
  UserOutlined,
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
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 320px), 1fr))',
            gap: 20,
          }}
        >
          {activities.map((act) => {
            const start = new Date(act.starts_at)
            const end = act.ends_at ? new Date(act.ends_at) : null
            const isDraft = act.publication_status === 'draft'
            const isCancelled = act.status === 'cancelled'

            return (
              <div
                key={act.id}
                className="sf-card"
                role="link"
                aria-label={`Detail kegiatan ${act.title}`}
                tabIndex={0}
                onClick={() => navigate(`/kegiatan/${act.id}`)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') navigate(`/kegiatan/${act.id}`)
                }}
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  overflow: 'hidden',
                  padding: 0,
                  borderRadius: 12,
                  cursor: 'pointer',
                }}
              >
                {/* Poster / Image header with consistent height & placeholder */}
                <div style={{ height: 180, overflow: 'hidden', position: 'relative', background: 'var(--sf-surface-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'center', borderBottom: '1px solid var(--sf-border-subtle, rgba(0,0,0,0.06))' }}>
                  {act.poster_url ? (
                    <CommunityImage
                      src={act.poster_url}
                      alt={`Poster kegiatan ${act.title}`}
                      style={{ width: '100%', height: '100%', objectFit: 'contain' }}
                    />
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, color: 'var(--sf-text-muted)', opacity: 0.6 }}>
                      <CalendarOutlined style={{ fontSize: 36 }} />
                      <span style={{ fontSize: 11, fontWeight: 500 }}>Kegiatan RT</span>
                    </div>
                  )}
                </div>

                <div style={{ padding: 20, display: 'flex', flexDirection: 'column', flex: 1, gap: 12 }}>
                  {/* Status Badges */}
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    {getStatusBadge(act.status)}
                    {canManage && isDraft && <Badge variant="amber">DRAF</Badge>}
                    {act.show_on_banner && <Badge variant="blue">BANNER</Badge>}
                  </div>

                  {/* Title & Description */}
                  <div>
                    <h3 style={{ margin: '0 0 6px 0', fontSize: 16, fontWeight: 700, color: 'var(--sf-text)' }}>
                      {act.title}
                    </h3>
                    {act.description && (
                      <p style={{ margin: 0, fontSize: 13, color: 'var(--sf-text-muted)', lineHeight: 1.5, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                        {act.description}
                      </p>
                    )}
                  </div>

                  {/* Meta Details */}
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 'auto', paddingTop: 8 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <ClockCircleOutlined style={{ color: 'var(--sf-accent)' }} />
                      <span>
                        {start.toLocaleDateString('id-ID', { dateStyle: 'full' })} •{' '}
                        {start.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB
                        {end && ` - ${end.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB`}
                      </span>
                    </div>

                    {act.location && (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <EnvironmentOutlined style={{ color: 'var(--sf-accent)' }} />
                        <span>{act.location}</span>
                      </div>
                    )}

                    {act.organizer && (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <UserOutlined style={{ color: 'var(--sf-accent)' }} />
                        <span>Penyelenggara: {act.organizer}</span>
                      </div>
                    )}
                  </div>

                  {/* Manager Action Bar */}
                  {canManage && (
                    <div
                      onClick={(e) => e.stopPropagation()}
                      style={{
                        display: 'flex',
                        gap: 8,
                        marginTop: 8,
                        paddingTop: 12,
                        borderTop: '1px solid var(--sf-border)',
                        justifyContent: 'flex-end',
                      }}
                    >
                      {(isDraft || (!isCancelled)) && (
                        <button
                          type="button"
                          className="sf-btn sf-btn-outline"
                          onClick={() => handlePublish(act.id)}
                          style={{ padding: '4px 10px', fontSize: 12 }}
                          title={isDraft ? 'Publikasikan' : 'Terbitkan ulang (perbarui snapshot banner)'}
                        >
                          <CheckOutlined /> {isDraft ? 'Publikasikan' : 'Terbitkan Ulang'}
                        </button>
                      )}
                      {!isCancelled && (
                        <button
                          type="button"
                          className="sf-btn sf-btn-outline"
                          onClick={() => handleCancel(act.id)}
                          style={{ padding: '4px 10px', fontSize: 12, color: 'var(--sf-danger)' }}
                          title="Batalkan"
                        >
                          <CloseOutlined /> Batalkan
                        </button>
                      )}
                      <button
                        type="button"
                        className="sf-btn sf-btn-outline"
                        onClick={() => {
                          setEditingActivity(act)
                          setShowModal(true)
                        }}
                        style={{ padding: '4px 10px', fontSize: 12 }}
                        title="Ubah"
                      >
                        <EditOutlined /> Ubah
                      </button>
                      {isDraft && (
                        <button
                          type="button"
                          className="sf-btn sf-btn-outline"
                          onClick={() => handleDelete(act.id)}
                          style={{ padding: '4px 10px', fontSize: 12, color: 'var(--sf-danger)' }}
                          title="Hapus"
                        >
                          <DeleteOutlined /> Hapus
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            )
          })}
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
