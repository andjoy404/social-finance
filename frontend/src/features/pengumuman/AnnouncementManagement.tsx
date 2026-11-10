import { useState, useEffect, useCallback } from 'react'
import { useAuth, canManageAnnouncements } from '@/app/AuthContext'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { CommunityImage } from '@/components/CommunityImage'
import { Pagination } from '@/components/Pagination'
import {
  NotificationOutlined,
  PlusOutlined,
  AppstoreOutlined,
  CheckOutlined,
  InboxOutlined,
  EditOutlined,
  DeleteOutlined,
  LinkOutlined,
  ThunderboltOutlined,
  ReloadOutlined,
  EyeOutlined,
} from '@ant-design/icons'
import {
  CommunityAnnouncement,
  BannerTemplate,
  apiListAnnouncements,
  apiListBannerTemplates,
  apiPublishAnnouncement,
  apiArchiveAnnouncement,
  apiDeleteAnnouncement,
  getSessionPair,
} from '@/app/api'
import { AnnouncementModal } from './AnnouncementModal'

function formatScheduleWindow(start?: string | null, end?: string | null): string | null {
  const fmt = (v?: string | null) =>
    v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : null
  const s = fmt(start)
  const e = fmt(end)
  if (s && e) return `${s} — ${e}`
  if (s) return `Mulai ${s}`
  if (e) return `Sampai ${e}`
  return null
}

export function AnnouncementManagement() {
  const { user } = useAuth()
  const [activeTab, setActiveTab] = useState<'announcements' | 'templates'>('announcements')
  const [announcements, setAnnouncements] = useState<CommunityAnnouncement[]>([])
  const [templates, setTemplates] = useState<BannerTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [showModal, setShowModal] = useState(false)
  const [editingAnnouncement, setEditingAnnouncement] = useState<CommunityAnnouncement | null>(null)
  const [selectedTemplateForCreate, setSelectedTemplateForCreate] = useState<BannerTemplate | null>(null)
  const [viewingAnnouncement, setViewingAnnouncement] = useState<CommunityAnnouncement | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)

  // Permission-driven gating (not role-name based).
  const canManage = canManageAnnouncements(user)

  const loadData = useCallback(() => {
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

    Promise.all([
      apiListAnnouncements(session.accessToken, params),
      apiListBannerTemplates(session.accessToken),
    ])
      .then(([annRes, tmplRes]) => {
        setTemplates(tmplRes || [])
        if ((annRes.data || []).length === 0 && page > 1 && (annRes.pagination?.total ?? 0) > 0) {
          setPage((p) => p - 1)
          return
        }
        setAnnouncements(annRes.data || [])
        setTotal(annRes.pagination?.total ?? 0)
        setTotalPages(Math.max(1, annRes.pagination?.total_pages ?? 1))
        setLoading(false)
      })
      .catch((err) => {
        setError(err?.message || 'Gagal memuat pengumuman. Periksa koneksi Anda lalu coba lagi.')
        setAnnouncements([])
        setTotal(0)
        setTotalPages(1)
        setLoading(false)
      })
  }, [statusFilter, page, pageSize])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleStatusFilter = (next: string) => {
    setPage(1)
    setStatusFilter(next)
  }

  const handlePublish = async (id: string, republish: boolean) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    const confirmed = republish
      ? confirm('Terbitkan ulang pengumuman ini? Konten banner di Beranda ikut diperbarui.')
      : confirm('Publikasikan pengumuman ini sekarang? Pengumuman akan langsung tampil di Beranda.')
    if (!confirmed) return

    setActionError(null)
    try {
      await apiPublishAnnouncement(session.accessToken, id)
      loadData()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal mempublikasikan pengumuman')
    }
  }

  const handleArchive = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Arsipkan pengumuman ini? Banner di Beranda akan dinonaktifkan.')) return

    setActionError(null)
    try {
      await apiArchiveAnnouncement(session.accessToken, id)
      loadData()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal mengarsipkan pengumuman')
    }
  }

  const handleDelete = async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    if (!confirm('Hapus pengumuman draf ini? Tindakan ini tidak dapat dibatalkan.')) return

    setActionError(null)
    try {
      await apiDeleteAnnouncement(session.accessToken, id)
      loadData()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal menghapus pengumuman')
    }
  }

  const handleUseTemplate = (tmpl: BannerTemplate) => {
    setSelectedTemplateForCreate(tmpl)
    setEditingAnnouncement(null)
    setShowModal(true)
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active':
        return <Badge variant="green">Aktif</Badge>
      case 'scheduled':
        return <Badge variant="blue">Terjadwal</Badge>
      case 'draft':
        return <Badge variant="amber">Draf</Badge>
      case 'archived':
        return <Badge variant="gray">Diarsipkan</Badge>
      case 'expired':
        return <Badge variant="red">Kedaluwarsa</Badge>
      default:
        return <Badge variant="default">{status}</Badge>
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <PageHeader
        surface
        icon={<NotificationOutlined />}
        title="Pengumuman"
        subtitle="Publikasikan pengumuman warga dan kelola banner informasi di Beranda"
        actions={
          canManage ? (
            <button
              type="button"
              className="sf-btn sf-btn-primary"
              onClick={() => {
                setSelectedTemplateForCreate(null)
                setEditingAnnouncement(null)
                setShowModal(true)
              }}
            >
              <PlusOutlined /> Tambah Pengumuman
            </button>
          ) : undefined
        }
      />

      {/* Main Tabs: Pengumuman vs Template Banner */}
      <div style={{ display: 'flex', gap: 12, borderBottom: '1px solid var(--sf-border)', paddingBottom: 12 }}>
        <button
          type="button"
          onClick={() => setActiveTab('announcements')}
          className={`sf-btn ${activeTab === 'announcements' ? 'sf-btn-primary' : 'sf-btn-outline'}`}
        >
          <NotificationOutlined /> Daftar Pengumuman ({total})
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('templates')}
          className={`sf-btn ${activeTab === 'templates' ? 'sf-btn-primary' : 'sf-btn-outline'}`}
        >
          <AppstoreOutlined /> Template Banner ({templates.length})
        </button>
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

      {activeTab === 'announcements' ? (
        <>
          {/* Status Filters (visible to managers; warga only ever sees in-window active announcements) */}
          {canManage && (
            <div className="sf-list-toolbar" style={{ marginTop: 0, marginBottom: 0 }}>
              <div className="sf-list-toolbar-left" style={{ flexWrap: 'wrap', gap: 8 }}>
                {[
                  { id: 'all', label: 'Semua Status' },
                  { id: 'active', label: 'Aktif' },
                  { id: 'draft', label: 'Draf' },
                  { id: 'scheduled', label: 'Terjadwal' },
                  { id: 'archived', label: 'Diarsipkan' },
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
          )}

          {/* Announcements List */}
          {loading ? (
            <div className="sf-card" style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
              Memuat pengumuman...
            </div>
          ) : error ? (
            <div className="sf-card" style={{ padding: 48, textAlign: 'center' }}>
              <NotificationOutlined style={{ fontSize: 32, marginBottom: 12, color: 'var(--sf-text-muted)' }} />
              <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-text)' }}>Gagal memuat pengumuman</div>
              <div style={{ fontSize: 13, marginTop: 4, color: 'var(--sf-text-muted)' }}>{error}</div>
              <button type="button" className="sf-btn sf-btn-outline" style={{ marginTop: 16 }} onClick={loadData}>
                <ReloadOutlined /> Coba Lagi
              </button>
            </div>
          ) : announcements.length > 0 ? (
            <>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              {announcements.map((ann) => {
                const windowLabel = formatScheduleWindow(ann.scheduled_start, ann.scheduled_end)
                const isDraft = ann.status === 'draft'
                const canRepublish = ann.status === 'active' || ann.status === 'scheduled' || ann.status === 'archived'
                return (
                <div
                  key={ann.id}
                  className="sf-card"
                  style={{
                    padding: 20,
                    borderRadius: 12,
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 12,
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12 }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 6, flexWrap: 'wrap' }}>
                        {getStatusBadge(ann.status)}
                        <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                          Prioritas: {ann.priority}
                        </span>
                        {ann.deep_link && (
                          <span
                            style={{
                              fontSize: 11,
                              padding: '2px 8px',
                              borderRadius: 4,
                              background: 'var(--sf-surface-hover)',
                              color: 'var(--sf-accent)',
                              display: 'flex',
                              alignItems: 'center',
                              gap: 4,
                            }}
                          >
                            <LinkOutlined /> {ann.deep_link}
                          </span>
                        )}
                      </div>
                      <h3 style={{ margin: '0 0 6px 0', fontSize: 17, fontWeight: 700, color: 'var(--sf-text)' }}>
                        {ann.title}
                      </h3>
                      <p style={{ margin: 0, fontSize: 13, color: 'var(--sf-text-muted)', lineHeight: 1.5, whiteSpace: 'pre-line' }}>
                        {ann.body}
                      </p>
                      {windowLabel && (
                        <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 6 }}>
                          Tayang: {windowLabel}
                        </div>
                      )}
                    </div>

                    {ann.image_url && (
                      <div
                        style={{
                          width: 90,
                          height: 65,
                          borderRadius: 8,
                          overflow: 'hidden',
                          background: 'var(--sf-surface-subtle)',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          flexShrink: 0,
                          border: '1px solid var(--sf-border-subtle)',
                        }}
                      >
                        <CommunityImage
                          src={ann.image_url}
                          alt={`Gambar pengumuman ${ann.title}`}
                          style={{ width: '100%', height: '100%', objectFit: 'contain' }}
                        />
                      </div>
                    )}
                  </div>

                  {/* Actions Bar */}
                  <div
                    style={{
                      display: 'flex',
                      gap: 8,
                      paddingTop: 12,
                      borderTop: '1px solid var(--sf-border)',
                      justifyContent: 'flex-end',
                      alignItems: 'center',
                      flexWrap: 'wrap',
                    }}
                  >
                    <button
                      type="button"
                      className="sf-btn sf-btn-outline"
                      onClick={() => setViewingAnnouncement(ann)}
                      style={{ padding: '4px 10px', fontSize: 12 }}
                    >
                      <EyeOutlined /> Lihat Detail
                    </button>
                    {canManage && (
                      <>
                        {isDraft && (
                          <button
                            type="button"
                            className="sf-btn sf-btn-outline"
                            onClick={() => handlePublish(ann.id, false)}
                            style={{ padding: '4px 10px', fontSize: 12 }}
                          >
                            <CheckOutlined /> Publikasikan
                          </button>
                        )}
                        {canRepublish && (
                          <button
                            type="button"
                            className="sf-btn sf-btn-outline"
                            onClick={() => handlePublish(ann.id, true)}
                            style={{ padding: '4px 10px', fontSize: 12 }}
                            title="Terbitkan ulang (perbarui snapshot konten & banner)"
                          >
                            <CheckOutlined /> Terbitkan Ulang
                          </button>
                        )}
                        {(ann.status === 'active' || ann.status === 'scheduled') && (
                          <button
                            type="button"
                            className="sf-btn sf-btn-outline"
                            onClick={() => handleArchive(ann.id)}
                            style={{ padding: '4px 10px', fontSize: 12 }}
                          >
                            <InboxOutlined /> Arsipkan
                          </button>
                        )}
                        <button
                          type="button"
                          className="sf-btn sf-btn-outline"
                          onClick={() => {
                            setEditingAnnouncement(ann)
                            setSelectedTemplateForCreate(null)
                            setShowModal(true)
                          }}
                          style={{ padding: '4px 10px', fontSize: 12 }}
                        >
                          <EditOutlined /> Ubah
                        </button>
                        {isDraft && (
                          <button
                            type="button"
                            className="sf-btn sf-btn-outline"
                            onClick={() => handleDelete(ann.id)}
                            style={{ padding: '4px 10px', fontSize: 12, color: 'var(--sf-danger)' }}
                          >
                            <DeleteOutlined /> Hapus
                          </button>
                        )}
                      </>
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
              <NotificationOutlined style={{ fontSize: 32, marginBottom: 12 }} />
              <div style={{ fontSize: 15, fontWeight: 600 }}>Belum ada pengumuman</div>
              <div style={{ fontSize: 13, marginTop: 4 }}>
                {canManage
                  ? 'Gunakan tombol "Tambah Pengumuman" atau pilih dari tab "Template Banner" untuk memulai.'
                  : 'Belum ada pengumuman yang dipublikasikan oleh pengurus.'}
              </div>
            </div>
          )}
        </>
      ) : (
        /* Template Banner Tab */
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(min(100%, 300px), 1fr))',
            gap: 20,
          }}
        >
          {templates.map((tmpl) => (
            <div
              key={tmpl.id}
              className="sf-card"
              style={{
                padding: 20,
                borderRadius: 12,
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between',
                gap: 16,
              }}
            >
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <span
                    style={{
                      fontSize: 11,
                      fontWeight: 700,
                      textTransform: 'uppercase',
                      padding: '2px 8px',
                      borderRadius: 4,
                      background: 'rgba(169, 112, 255, 0.15)',
                      color: 'var(--sf-accent)',
                    }}
                  >
                    {tmpl.category}
                  </span>
                  {tmpl.is_system && <Badge variant="gray">SISTEM</Badge>}
                </div>
                <h3 style={{ margin: '0 0 6px 0', fontSize: 16, fontWeight: 700, color: 'var(--sf-text)' }}>
                  {tmpl.name}
                </h3>
                <div
                  style={{
                    fontSize: 13,
                    fontWeight: 600,
                    color: 'var(--sf-text)',
                    background: 'var(--sf-bg)',
                    padding: '8px 10px',
                    borderRadius: 6,
                    border: '1px solid var(--sf-border)',
                    marginBottom: 8,
                  }}
                >
                  {tmpl.title_template}
                </div>
                <p style={{ margin: 0, fontSize: 12, color: 'var(--sf-text-muted)', lineHeight: 1.5 }}>
                  {tmpl.body_template}
                </p>
              </div>

              {canManage && (
                <button
                  type="button"
                  className="sf-btn sf-btn-primary"
                  onClick={() => handleUseTemplate(tmpl)}
                  style={{ width: '100%', justifyContent: 'center' }}
                >
                  <ThunderboltOutlined /> Gunakan Template
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Modal for create/edit */}
      {showModal && (
        <AnnouncementModal
          announcement={editingAnnouncement}
          initialTemplate={selectedTemplateForCreate}
          templates={templates}
          onClose={() => {
            setShowModal(false)
            setEditingAnnouncement(null)
            setSelectedTemplateForCreate(null)
          }}
          onSuccess={() => {
            setShowModal(false)
            setEditingAnnouncement(null)
            setSelectedTemplateForCreate(null)
            if (page !== 1) setPage(1)
            else loadData()
          }}
        />
      )}

      {/* Detail Modal */}
      {viewingAnnouncement && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.6)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            padding: 16,
          }}
          onClick={() => setViewingAnnouncement(null)}
        >
          <div
            className="sf-card"
            style={{
              width: '100%',
              maxWidth: 600,
              maxHeight: '90vh',
              overflowY: 'auto',
              padding: 24,
              borderRadius: 12,
              background: 'var(--sf-surface)',
              backgroundColor: 'var(--sf-surface)',
              color: 'var(--sf-text)',
              border: '1px solid var(--sf-border)',
              boxShadow: 'var(--sf-shadow-md)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
              <div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                  {getStatusBadge(viewingAnnouncement.status)}
                  <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                    Prioritas {viewingAnnouncement.priority}
                  </span>
                </div>
                <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--sf-text)' }}>
                  {viewingAnnouncement.title}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setViewingAnnouncement(null)}
                style={{ background: 'none', border: 'none', fontSize: 22, cursor: 'pointer', color: 'var(--sf-text-muted)' }}
              >
                ×
              </button>
            </div>

            {viewingAnnouncement.image_url && (
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
                  src={viewingAnnouncement.image_url}
                  alt={viewingAnnouncement.title}
                  style={{ width: '100%', maxHeight: 280, objectFit: 'contain' }}
                />
              </div>
            )}

            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 4 }}>
                ISI PENGUMUMAN
              </div>
              <p style={{ margin: 0, fontSize: 14, color: 'var(--sf-text)', lineHeight: 1.6, whiteSpace: 'pre-line' }}>
                {viewingAnnouncement.body}
              </p>
            </div>

            {formatScheduleWindow(viewingAnnouncement.scheduled_start, viewingAnnouncement.scheduled_end) && (
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 12 }}>
                <strong>Jadwal Tayang:</strong> {formatScheduleWindow(viewingAnnouncement.scheduled_start, viewingAnnouncement.scheduled_end)}
              </div>
            )}

            {viewingAnnouncement.deep_link && (
              <div style={{ fontSize: 12, color: 'var(--sf-accent)', marginBottom: 16, display: 'flex', alignItems: 'center', gap: 6 }}>
                <LinkOutlined /> Menuju menu: <strong>{viewingAnnouncement.deep_link}</strong>
              </div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', paddingTop: 16, borderTop: '1px solid var(--sf-border)' }}>
              <button
                type="button"
                className="sf-btn sf-btn-outline"
                onClick={() => setViewingAnnouncement(null)}
              >
                Tutup
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
