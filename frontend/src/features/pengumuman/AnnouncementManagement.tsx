import { useState, useEffect, useCallback, useMemo } from 'react'
import { useAuth, canManageAnnouncements } from '@/app/AuthContext'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { CommunityImage } from '@/components/CommunityImage'
import { Pagination } from '@/components/Pagination'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
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
  PictureOutlined,
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
  const [searchQuery, setSearchQuery] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [editingAnnouncement, setEditingAnnouncement] = useState<CommunityAnnouncement | null>(null)
  const [selectedTemplateForCreate, setSelectedTemplateForCreate] = useState<BannerTemplate | null>(null)
  const [viewingAnnouncement, setViewingAnnouncement] = useState<CommunityAnnouncement | null>(null)
  const sortedAnnouncements = useMemo(() => {
    let filtered = [...announcements].sort((a, b) => {
      const pa = a.is_permanent ? 1 : 0
      const pb = b.is_permanent ? 1 : 0
      if (pa !== pb) return pb - pa
      return b.priority - a.priority
    })

    // Client-side search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase()
      filtered = filtered.filter((a) =>
        a.title.toLowerCase().includes(q) ||
        (a.body && a.body.toLowerCase().includes(q)),
      )
    }

    return filtered
  }, [announcements, searchQuery])

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

    const fetchTemplates = canManage
      ? apiListBannerTemplates(session.accessToken)
      : Promise.resolve([] as BannerTemplate[])

    Promise.all([
      apiListAnnouncements(session.accessToken, params),
      fetchTemplates,
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

  const handleSearch = (query: string) => {
    setPage(1)
    setSearchQuery(query)
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
    <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
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

      {/* Main Tabs: Daftar Pengumuman vs Template Banner */}
      {canManage && (
        <div
          style={{
            display: 'flex',
            gap: 4,
            margin: '0px 0 0 0',
            borderBottom: '1px solid var(--sf-border)',
          }}
        >
          <button
            type="button"
            onClick={() => setActiveTab('announcements')}
            style={{
              display: 'flex',
              gap: 6,
              padding: '10px 16px',
              fontSize: 13,
              fontWeight: activeTab === 'announcements' ? 600 : 400,
              fontStyle: 'normal',
              color: activeTab === 'announcements' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
              border: 'none',
              borderBottom: activeTab === 'announcements' ? '2px solid var(--sf-accent)' : '2px solid transparent',
              borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
              background: activeTab === 'announcements' ? 'var(--sf-surface-hover)' : 'transparent',
              cursor: 'pointer',
              transition: 'all 0.2s ease',
              whiteSpace: 'nowrap',
            }}
            onMouseEnter={(e) => {
              if (activeTab !== 'announcements') {
                e.currentTarget.style.background = 'var(--sf-surface-hover)'
              }
            }}
            onMouseLeave={(e) => {
              if (activeTab !== 'announcements') {
                e.currentTarget.style.background = 'transparent'
              }
            }}
            aria-label="Tab Daftar Pengumuman"
          >
            <NotificationOutlined aria-hidden="true" /> Daftar Pengumuman ({total})
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('templates')}
            style={{
              display: 'flex',
              gap: 6,
              padding: '10px 16px',
              fontSize: 13,
              fontWeight: activeTab === 'templates' ? 600 : 400,
              fontStyle: 'normal',
              color: activeTab === 'templates' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
              border: 'none',
              borderBottom: activeTab === 'templates' ? '2px solid var(--sf-accent)' : '2px solid transparent',
              borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
              background: activeTab === 'templates' ? 'var(--sf-surface-hover)' : 'transparent',
              cursor: 'pointer',
              transition: 'all 0.2s ease',
              whiteSpace: 'nowrap',
            }}
            onMouseEnter={(e) => {
              if (activeTab !== 'templates') {
                e.currentTarget.style.background = 'var(--sf-surface-hover)'
              }
            }}
            onMouseLeave={(e) => {
              if (activeTab !== 'templates') {
                e.currentTarget.style.background = 'transparent'
              }
            }}
            aria-label="Tab Template Banner"
          >
            <AppstoreOutlined aria-hidden="true" /> Template Banner ({templates.length})
          </button>
        </div>
      )}

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
        <div className="sf-content-surface">
          {/* Toolbar */}
          <div className="sf-list-toolbar">
            <div className="sf-list-toolbar-left">
              {/* Status Filter Dropdown */}
              <FilterDropdown
                value={statusFilter === 'all' ? 'semua' : statusFilter}
                options={canManage
                  ? [
                      { value: 'semua', label: 'Semua Status' },
                      { value: 'active', label: 'Aktif' },
                      { value: 'draft', label: 'Draf' },
                      { value: 'scheduled', label: 'Terjadwal' },
                      { value: 'archived', label: 'Diarsipkan' },
                    ]
                  : [
                      { value: 'semua', label: 'Semua' },
                      { value: 'active', label: 'Aktif' },
                      { value: 'history', label: 'Riwayat' },
                    ]}
                onChange={(v) => handleStatusFilter(v === 'semua' ? 'all' : v)}
                ariaLabel="Status pengumuman"
              />

              {/* Search */}
              <SearchBox
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={handleSearch}
                placeholder="Cari judul pengumuman..."
              />
            </div>

            <div className="sf-list-toolbar-right">
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={loadData}
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
              Memuat pengumuman...
            </div>
          )}

          {/* Empty */}
          {!loading && !error && sortedAnnouncements.length === 0 && (
            <div className="sf-empty-banner">
              {searchQuery || statusFilter !== 'all'
                ? 'Tidak ada pengumuman yang sesuai filter.'
                : 'Belum ada pengumuman'}
            </div>
          )}

          {/* Announcements Table */}
          {!loading && sortedAnnouncements.length > 0 && (
            <>
              <div className="table-wrapper">
              <table className="sf-warga-table sf-announcement-table">
                <thead>
                  <tr>
                    <th className="col-ann-title">JUDUL & RINGKASAN</th>
                    <th className="col-ann-schedule">JADWAL & TAUTAN</th>
                    <th className="col-ann-priority">PRIORITAS</th>
                    <th className="col-status">STATUS</th>
                    <th className="col-action">AKSI</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedAnnouncements.map((ann) => {
                    const windowLabel = formatScheduleWindow(ann.scheduled_start, ann.scheduled_end)
                    const isDraft = ann.status === 'draft'
                    const canRepublish = ann.status === 'active' || ann.status === 'scheduled' || ann.status === 'archived'
                    const hasScheduleOrLink = Boolean(windowLabel || ann.deep_link)
                    return (
                      <tr key={ann.id}>
                        {/* 1. JUDUL & RINGKASAN — click to preview */}
                        <td className="col-ann-title" style={{ cursor: 'pointer' }} onClick={() => setViewingAnnouncement(ann)}>
                          <div style={{ fontWeight: 600, color: 'var(--sf-text)', marginBottom: ann.body ? 2 : 0, display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                            <span>{ann.title}</span>
                            {ann.image_url && (
                              <PictureOutlined style={{ color: 'var(--sf-accent)', fontSize: 13 }} title="Memiliki lampiran gambar" />
                            )}
                            {ann.is_permanent && (
                              <span
                                style={{
                                  fontSize: 10,
                                  fontWeight: 700,
                                  letterSpacing: 0.4,
                                  textTransform: 'uppercase',
                                  color: 'var(--sf-warning)',
                                  background: 'rgba(255, 152, 48, 0.15)',
                                  border: '1px solid rgba(255, 152, 48, 0.35)',
                                  borderRadius: 4,
                                  padding: '2px 6px',
                                }}
                                title="Pengumuman sistem default — bersifat permanen, tidak dapat dihapus atau diarsipkan"
                              >
                                Sistem · Permanen
                              </span>
                            )}
                          </div>
                          {ann.body && (
                            <div style={{ color: 'var(--sf-text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', fontSize: 12 }}>
                              {ann.body}
                            </div>
                          )}
                        </td>

                        {/* 2. JADWAL & TAUTAN */}
                        <td className="col-ann-schedule">
                          {hasScheduleOrLink ? (
                            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                              {windowLabel && (
                                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                                  Tayang: {windowLabel}
                                </div>
                              )}
                              {ann.deep_link && (
                                <span
                                  style={{
                                    fontSize: 11,
                                    padding: '2px 8px',
                                    borderRadius: 4,
                                    background: 'var(--sf-surface-hover)',
                                    color: 'var(--sf-accent)',
                                    display: 'inline-flex',
                                    alignItems: 'center',
                                    gap: 4,
                                    width: 'fit-content',
                                  }}
                                >
                                  <LinkOutlined /> {ann.deep_link}
                                </span>
                              )}
                            </div>
                          ) : (
                            <span style={{ color: 'var(--sf-text-muted)' }}>-</span>
                          )}
                        </td>

                        {/* 3. PRIORITAS */}
                        <td className="col-ann-priority">
                          <span style={{ fontSize: 12, color: 'var(--sf-text-muted)', fontWeight: 600 }}>
                            {ann.priority}
                          </span>
                        </td>

                        {/* 4. STATUS */}
                        <td className="col-status">
                          {getStatusBadge(ann.status)}
                        </td>

                        {/* 5. AKSI */}
                        <td className="col-action">
                          <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                            <button
                              type="button"
                              className="sf-btn-outline"
                              onClick={() => setViewingAnnouncement(ann)}
                              title="Lihat Detail"
                              aria-label="Lihat Detail"
                              style={{ padding: '4px 8px', fontSize: 12 }}
                            >
                              <EyeOutlined />
                            </button>
                            {canManage && (
                              <>
                                {isDraft && (
                                  <button
                                    type="button"
                                    className="sf-btn-outline"
                                    onClick={() => handlePublish(ann.id, false)}
                                    title="Publikasikan"
                                    aria-label="Publikasikan"
                                    style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-success)' }}
                                  >
                                    <CheckOutlined />
                                  </button>
                                )}
                                {canRepublish && (
                                  <button
                                    type="button"
                                    className="sf-btn-outline"
                                    onClick={() => handlePublish(ann.id, true)}
                                    title={ann.is_permanent ? 'Terbitkan / Terbitkan Ulang (perbarui snapshot konten & banner)' : 'Terbitkan ulang (perbarui snapshot konten & banner)'}
                                    aria-label={ann.is_permanent ? 'Terbitkan / Terbitkan Ulang' : 'Terbitkan Ulang'}
                                    style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-success)' }}
                                  >
                                    <CheckOutlined />
                                  </button>
                                )}
                                {(ann.status === 'active' || ann.status === 'scheduled') && !ann.is_permanent && (
                                  <button
                                    type="button"
                                    className="sf-btn-outline"
                                    onClick={() => handleArchive(ann.id)}
                                    title="Arsipkan"
                                    aria-label="Arsipkan"
                                    style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-warning)' }}
                                  >
                                    <InboxOutlined />
                                  </button>
                                )}
                                {ann.is_permanent && (
                                  <button
                                    type="button"
                                    className="sf-btn-outline"
                                    onClick={() => handleArchive(ann.id)}
                                    title="Nonaktifkan (tidak tampil di Beranda; dapat diaktifkan kembali)"
                                    aria-label="Nonaktifkan"
                                    style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-warning)' }}
                                  >
                                    <InboxOutlined />
                                  </button>
                                )}
                                <button
                                  type="button"
                                  className="sf-btn-outline"
                                  onClick={() => {
                                    setEditingAnnouncement(ann)
                                    setSelectedTemplateForCreate(null)
                                    setShowModal(true)
                                  }}
                                  title="Ubah"
                                  aria-label="Ubah"
                                  style={{ padding: '4px 8px', fontSize: 12 }}
                                >
                                  <EditOutlined />
                                </button>
                                {isDraft && !ann.is_permanent && (
                                  <button
                                    type="button"
                                    className="sf-btn-outline"
                                    onClick={() => handleDelete(ann.id)}
                                    title="Hapus"
                                    aria-label="Hapus"
                                    style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-danger)' }}
                                  >
                                    <DeleteOutlined />
                                  </button>
                                )}
                              </>
                            )}
                          </div>
                        </td>
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
