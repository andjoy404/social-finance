import React, { useState, useEffect, useCallback, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  SecurityScanOutlined,
  CameraOutlined,
  PlusOutlined,
  DeleteOutlined,
  ReloadOutlined,
  LoadingOutlined,
  CheckCircleOutlined,
  SettingOutlined,
  HistoryOutlined,
} from '@ant-design/icons'
import { useAuth, hasPermission } from '@/app/AuthContext'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { Pagination } from '@/components/Pagination'
import {
  PatrolBundle,
  PatrolTemplate,
  apiListPatrolBundles,
  apiGetPatrolBundle,
  apiListPatrolTemplates,
  apiCreatePatrolTemplate,
  apiUpdatePatrolTemplate,
  apiDeletePatrolTemplate,
  apiUploadPatrolPhoto,
  apiCreatePatrolBundle,
  apiMarkSecurityReadReceipt,
} from './securityApi'
import { SecurityImage, SecurityPhotoLightbox, LightboxPhoto } from './SecurityImage'
import { formatJakartaDateTime } from '@/utils/jakartaTime'

interface StagedPhoto {
  id: string
  file: File
  previewUrl: string
  templateId: string
  capturedAt: string
}

// Column width styles matching Warga's approach
const colStyles = {
  officer: { minWidth: 160, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  submittedAt: { width: 170, minWidth: 170, maxWidth: 170, textAlign: 'left' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  photos: { width: 180, minWidth: 180, maxWidth: 180, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  notes: { minWidth: 200, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  action: { width: 130, minWidth: 130, maxWidth: 130, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
}

const tplColStyles = {
  name: { minWidth: 180, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  location: { minWidth: 200, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  status: { width: 120, minWidth: 120, maxWidth: 120, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  action: { width: 200, minWidth: 200, maxWidth: 200, textAlign: 'right' as const, whiteSpace: 'nowrap' as const },
}

export function PatrolPage() {
  const { user } = useAuth()
  const [searchParams] = useSearchParams()

  const canRead = hasPermission(user, 'patrol.read') || hasPermission(user, 'patrol.create')
  const canCreate = hasPermission(user, 'patrol.create')
  const canManageTemplates = hasPermission(user, 'patrol.template.manage')

  // Active Tab: 'history' | 'create' | 'templates'
  const [activeTab, setActiveTab] = useState<'history' | 'create' | 'templates'>('history')

  // Bundles (History)
  const [bundles, setBundles] = useState<PatrolBundle[]>([])
  const [bundlesLoading, setBundlesLoading] = useState(false)
  const [bundlesError, setBundlesError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [totalCount, setTotalCount] = useState(0)
  const [searchQuery, setSearchQuery] = useState('')

  // Detail Modal
  const [selectedBundle, setSelectedBundle] = useState<PatrolBundle | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  // Lightbox
  const [lightboxPhotos, setLightboxPhotos] = useState<LightboxPhoto[]>([])
  const [lightboxIndex, setLightboxIndex] = useState(0)
  const [lightboxOpen, setLightboxOpen] = useState(false)

  // Templates
  const [templates, setTemplates] = useState<PatrolTemplate[]>([])

  // Staged Photos for Create
  const [stagedPhotos, setStagedPhotos] = useState<StagedPhoto[]>([])
  const [notes, setNotes] = useState('')
  const [selectedTemplateId, setSelectedTemplateId] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<string | null>(null)
  const [createError, setCreateError] = useState<string | null>(null)
  const [createSuccess, setCreateSuccess] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Template Management Modals
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<PatrolTemplate | null>(null)
  const [templateForm, setTemplateForm] = useState({ name: '', location_label: '', is_active: true })
  const [templateSubmitting, setTemplateSubmitting] = useState(false)
  const [templateError, setTemplateError] = useState<string | null>(null)

  // Load Templates
  const loadTemplates = useCallback(async () => {
    try {
      const list = await apiListPatrolTemplates()
      setTemplates(list)
      if (list.length > 0 && !selectedTemplateId) {
        const active = list.find((t) => t.is_active) || list[0]
        setSelectedTemplateId(active.id)
      }
    } catch {
      // ignore
    }
  }, [selectedTemplateId])

  // Load Bundles
  const loadBundles = useCallback(async (targetPage = 1, targetPageSize = pageSize) => {
    try {
      setBundlesLoading(true)
      setBundlesError(null)
      const res = await apiListPatrolBundles({ page: targetPage, per_page: targetPageSize })
      setBundles(res.data || [])
      setPage(res.pagination.page)
      setTotalCount(res.pagination.total)
    } catch (err: any) {
      setBundlesError(err?.message || 'Gagal memuat riwayat patroli')
    } finally {
      setBundlesLoading(false)
    }
  }, [pageSize])

  // Initial Load & Read Receipt
  useEffect(() => {
    if (canRead) {
      loadBundles(1, pageSize)
      loadTemplates()
      apiMarkSecurityReadReceipt('patrol').catch(() => {})
    }
  }, [canRead, loadBundles, loadTemplates, pageSize])

  // Handle URL deep-link query `?bundleId=...`
  useEffect(() => {
    const bundleId = searchParams.get('bundleId')
    if (bundleId) {
      setDetailLoading(true)
      apiGetPatrolBundle(bundleId)
        .then((b) => setSelectedBundle(b))
        .catch(() => {
          setSelectedBundle(null)
        })
        .finally(() => setDetailLoading(false))
    }
  }, [searchParams])

  // File Selection
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files || files.length === 0) return

    setCreateError(null)
    const newStaged: StagedPhoto[] = []

    for (let i = 0; i < files.length; i++) {
      const file = files[i]
      if (file.size > 5 * 1024 * 1024) {
        setCreateError(`File "${file.name}" melebihi ukuran maksimum 5MB`)
        continue
      }
      if (!file.type.startsWith('image/')) {
        setCreateError(`File "${file.name}" bukan gambar yang valid`)
        continue
      }
      const previewUrl = URL.createObjectURL(file)
      newStaged.push({
        id: `${Date.now()}-${i}-${Math.random()}`,
        file,
        previewUrl,
        templateId: selectedTemplateId || (templates[0]?.id ?? ''),
        capturedAt: new Date().toISOString(),
      })
    }

    setStagedPhotos((prev) => [...prev, ...newStaged])
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const handleRemoveStagedPhoto = (id: string) => {
    setStagedPhotos((prev) => {
      const found = prev.find((p) => p.id === id)
      if (found) URL.revokeObjectURL(found.previewUrl)
      return prev.filter((p) => p.id !== id)
    })
  }

  const handleUpdatePhotoTemplate = (photoId: string, templateId: string) => {
    setStagedPhotos((prev) =>
      prev.map((p) => (p.id === photoId ? { ...p, templateId } : p))
    )
  }

  // Submit Patrol
  const handleSubmitPatrol = async (e: React.FormEvent) => {
    e.preventDefault()
    if (submitting) return
    if (stagedPhotos.length === 0) {
      setCreateError('Minimal satu foto patroli harus disertakan')
      return
    }

    // Verify all photos have a template selected
    for (const p of stagedPhotos) {
      if (!p.templateId) {
        setCreateError('Pilih lokasi gang untuk setiap foto yang diambil')
        return
      }
    }

    try {
      setSubmitting(true)
      setCreateError(null)
      setUploadProgress('Mengunggah foto-foto patroli...')

      const uploadedPhotos: { patrol_template_id: string; image_key: string; captured_at: string }[] = []

      for (let i = 0; i < stagedPhotos.length; i++) {
        const p = stagedPhotos[i]
        setUploadProgress(`Mengunggah foto ${i + 1} dari ${stagedPhotos.length}...`)
        const uploadRes = await apiUploadPatrolPhoto(p.file)
        uploadedPhotos.push({
          patrol_template_id: p.templateId,
          image_key: uploadRes.image_key,
          captured_at: p.capturedAt,
        })
      }

      setUploadProgress('Menyimpan laporan patroli...')
      await apiCreatePatrolBundle({
        notes: notes.trim() || undefined,
        photos: uploadedPhotos,
      })

      // Clean up preview URLs
      stagedPhotos.forEach((p) => URL.revokeObjectURL(p.previewUrl))
      setStagedPhotos([])
      setNotes('')
      setCreateSuccess('Laporan patroli berhasil dikirim!')
      setActiveTab('history')
      loadBundles(1, pageSize)
    } catch (err: any) {
      setCreateError(err?.message || 'Gagal mengirim laporan patroli')
    } finally {
      setSubmitting(false)
      setUploadProgress(null)
    }
  }

  // Open Detail
  const handleOpenDetail = async (bundle: PatrolBundle) => {
    try {
      setDetailLoading(true)
      const full = await apiGetPatrolBundle(bundle.id)
      setSelectedBundle(full)
    } catch {
      setSelectedBundle(bundle)
    } finally {
      setDetailLoading(false)
    }
  }

  // Open Lightbox from bundle photos
  const handleOpenLightbox = (photos: PatrolBundle['photos'], startIndex = 0) => {
    if (!photos || photos.length === 0) return
    const formatted: LightboxPhoto[] = photos.map((p) => ({
      id: p.id,
      title: p.template_name || 'Foto Patroli',
      location: p.location_label || undefined,
      timestamp: p.captured_at,
    }))
    setLightboxPhotos(formatted)
    setLightboxIndex(startIndex)
    setLightboxOpen(true)
  }

  // Template Save
  const handleSaveTemplate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!templateForm.name.trim() || !templateForm.location_label.trim()) {
      setTemplateError('Nama dan label lokasi wajib diisi')
      return
    }

    try {
      setTemplateSubmitting(true)
      setTemplateError(null)
      if (editingTemplate) {
        await apiUpdatePatrolTemplate(editingTemplate.id, {
          name: templateForm.name.trim(),
          location_label: templateForm.location_label.trim(),
          is_active: templateForm.is_active,
        })
      } else {
        await apiCreatePatrolTemplate({
          name: templateForm.name.trim(),
          location_label: templateForm.location_label.trim(),
        })
      }
      setShowTemplateModal(false)
      setEditingTemplate(null)
      setTemplateForm({ name: '', location_label: '', is_active: true })
      loadTemplates()
    } catch (err: any) {
      setTemplateError(err?.message || 'Gagal menyimpan template')
    } finally {
      setTemplateSubmitting(false)
    }
  }

  const handleDeleteTemplate = async (template: PatrolTemplate) => {
    if (!window.confirm(`Yakin ingin menghapus template "${template.name}"?`)) return
    try {
      await apiDeletePatrolTemplate(template.id)
      loadTemplates()
    } catch (err: any) {
      alert(err?.message || 'Gagal menghapus template')
    }
  }

  // Filtered bundles (client-side search)
  const filteredBundles = bundles.filter((b) => {
    if (!searchQuery) return true
    const q = searchQuery.toLowerCase()
    return (
      (b.created_by_name && b.created_by_name.toLowerCase().includes(q)) ||
      (b.duty_officer_name && b.duty_officer_name.toLowerCase().includes(q)) ||
      (b.notes && b.notes.toLowerCase().includes(q))
    )
  })

  const handleSearch = useCallback((value: string) => {
    setSearchQuery(value)
    setPage(1)
  }, [])

  const handlePageChange = useCallback((p: number) => {
    if (p >= 1) {
      setPage(p)
      void loadBundles(p, pageSize)
    }
  }, [loadBundles, pageSize])

  const handlePageSizeChange = useCallback((newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }, [])

  const handleRefresh = useCallback(() => {
    void loadBundles(page, pageSize)
  }, [loadBundles, page, pageSize])

  const totalPages = Math.ceil(totalCount / pageSize) || 1

  if (!canRead && !canCreate) {
    return (
      <div style={{ padding: 32, textAlign: 'center' }}>
        <h3>Akses Ditolak</h3>
        <p style={{ color: 'var(--sf-text-muted)' }}>Anda tidak memiliki hak akses untuk membuka fitur Patroli.</p>
      </div>
    )
  }

  return (
    <div>
      {/* ── Page Header ──────────────────────────────────────────────────── */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <SecurityScanOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Patroli Wilayah</div>
            <div className="sf-page-header-subtitle">
              Pencatatan dan pemantauan aktivitas patroli keamanan lingkungan RT.
            </div>
          </div>
        </div>
      </div>

      {/* ── Tab Navigation ───────────────────────────────────────────────── */}
      <div style={{
        display: 'flex',
        gap: 4,
        margin: '16px 0 0 0',
        borderBottom: '1px solid var(--sf-border)',
        paddingBottom: 0,
      }}>
        <button
          type="button"
          onClick={() => setActiveTab('history')}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '10px 16px',
            border: 'none',
            borderBottom: activeTab === 'history' ? '2px solid var(--sf-accent)' : '2px solid transparent',
            background: activeTab === 'history' ? 'var(--sf-surface-hover)' : 'transparent',
            color: activeTab === 'history' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
            cursor: 'pointer',
            fontSize: 13,
            fontWeight: activeTab === 'history' ? 600 : 400,
            borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
            transition: 'all 0.2s ease',
          }}
        >
          <HistoryOutlined aria-hidden="true" />
          RIWAYAT PATROLI
        </button>
        {canCreate && (
          <button
            type="button"
            onClick={() => setActiveTab('create')}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              padding: '10px 16px',
              border: 'none',
              borderBottom: activeTab === 'create' ? '2px solid var(--sf-accent)' : '2px solid transparent',
              background: activeTab === 'create' ? 'var(--sf-surface-hover)' : 'transparent',
              color: activeTab === 'create' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
              cursor: 'pointer',
              fontSize: 13,
              fontWeight: activeTab === 'create' ? 600 : 400,
              borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
              transition: 'all 0.2s ease',
            }}
          >
            <CameraOutlined aria-hidden="true" />
            KIRIM LAPORAN
          </button>
        )}
        {canManageTemplates && (
          <button
            type="button"
            onClick={() => setActiveTab('templates')}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              padding: '10px 16px',
              border: 'none',
              borderBottom: activeTab === 'templates' ? '2px solid var(--sf-accent)' : '2px solid transparent',
              background: activeTab === 'templates' ? 'var(--sf-surface-hover)' : 'transparent',
              color: activeTab === 'templates' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
              cursor: 'pointer',
              fontSize: 13,
              fontWeight: activeTab === 'templates' ? 600 : 400,
              borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
              transition: 'all 0.2s ease',
            }}
          >
            <SettingOutlined aria-hidden="true" />
            TEMPLATE LOKASI
          </button>
        )}
      </div>

      {/* ── Success Banner ───────────────────────────────────────────────── */}
      {createSuccess && (
        <div className="sf-success-banner" role="alert">
          <CheckCircleOutlined />
          <span>{createSuccess}</span>
          <button
            onClick={() => setCreateSuccess(null)}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--sf-text-muted)' }}
          >
            &times;
          </button>
        </div>
      )}

      {/* ═══════════════════════════════════════════════════════════════════ */}
      {/* ── TAB 1: RIWAYAT PATROLI ───────────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {activeTab === 'history' && (
        <div className="sf-content-surface">
          {/* Toolbar */}
          <div className="sf-list-toolbar">
            <div className="sf-list-toolbar-left">
              <SearchBox
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={handleSearch}
                placeholder="Cari nama petugas atau catatan..."
              />
            </div>
            <div className="sf-list-toolbar-right" style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={handleRefresh}
                style={{ padding: '6px 14px', fontSize: '12px', display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              >
                <ReloadOutlined style={{ fontSize: 13 }} />
                Refresh
              </button>
            </div>
          </div>

          {/* Error */}
          {bundlesError && (
            <div className="sf-error-banner" role="alert">
              {bundlesError}
            </div>
          )}

          {/* Loading */}
          {bundlesLoading && (
            <div className="sf-loading-banner">
              Memuat riwayat patroli...
            </div>
          )}

          {/* Empty */}
          {!bundlesLoading && !bundlesError && filteredBundles.length === 0 && (
            <div className="sf-empty-banner">
              Tidak ada laporan patroli.
            </div>
          )}

          {/* Patrol History Table */}
          {!bundlesLoading && filteredBundles.length > 0 && (
            <div className="table-wrapper">
              <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
                <thead>
                  <tr>
                    <th className="col-officer" style={colStyles.officer}>PETUGAS</th>
                    <th className="col-submitted-at" style={colStyles.submittedAt}>WAKTU KIRIM</th>
                    <th className="col-photos" style={colStyles.photos}>FOTO</th>
                    <th className="col-notes" style={colStyles.notes}>CATATAN</th>
                    <th className="col-action" style={colStyles.action}>AKSI</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredBundles.map((row) => (
                    <tr key={row.id}>
                      {/* 1. PETUGAS */}
                      <td style={{ ...colStyles.officer, fontWeight: 600 }}>
                        {row.duty_officer_name || row.created_by_name || 'Petugas Keamanan'}
                      </td>

                      {/* 2. WAKTU KIRIM */}
                      <td style={colStyles.submittedAt}>
                        {formatJakartaDateTime(row.submitted_at || row.created_at)}
                      </td>

                      {/* 3. FOTO — thumbnail + count badge */}
                      <td style={colStyles.photos}>
                        {(() => {
                          const photos = row.photos
                          if (!photos || photos.length === 0) {
                            return <Badge variant="blue">{row.photo_count} Foto</Badge>
                          }
                          const firstPhoto = photos[0]
                          return (
                            <div style={{ display: 'flex', alignItems: 'center', gap: 8, justifyContent: 'center' }}>
                              <div
                                style={{
                                  width: 44,
                                  height: 44,
                                  cursor: 'pointer',
                                  borderRadius: 4,
                                  overflow: 'hidden',
                                  border: '1px solid var(--sf-border)',
                                }}
                                onClick={() => handleOpenLightbox(photos, 0)}
                                title="Klik untuk melihat foto penuh"
                              >
                                <SecurityImage
                                  photoId={firstPhoto.id}
                                  domain="patrol"
                                  style={{ width: 44, height: 44, objectFit: 'cover' }}
                                />
                              </div>
                              <Badge variant="blue">{row.photo_count} Foto</Badge>
                            </div>
                          )
                        })()}
                      </td>

                      {/* 4. CATATAN */}
                      <td style={colStyles.notes}>
                        {row.notes || '\u2014'}
                      </td>

                      {/* 5. AKSI */}
                      <td style={colStyles.action}>
                        <button
                          className="sf-btn-secondary"
                          style={{ padding: '4px 10px', fontSize: '12px' }}
                          onClick={() => handleOpenDetail(row)}
                        >
                          Detail
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Pagination */}
          {!bundlesLoading && filteredBundles.length > 0 && (
            <Pagination
              page={page}
              pageSize={pageSize}
              total={totalCount}
              totalPages={totalPages}
              onPageChange={handlePageChange}
              onPageSizeChange={handlePageSizeChange}
              pageSizeOptions={[15, 25, 50]}
            />
          )}
        </div>
      )}

      {/* ═══════════════════════════════════════════════════════════════════ */}
      {/* ── TAB 2: KIRIM LAPORAN PATROLI ─────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {activeTab === 'create' && canCreate && (
        <div className="sf-content-surface sf-content-surface-padding" style={{ padding: 24 }}>
          <h2 style={{ fontSize: 18, fontWeight: 700, margin: '0 0 16px' }}>
            Kirim Laporan Patroli Wilayah
          </h2>

          {createError && (
            <div className="sf-error-banner" role="alert">
              {createError}
            </div>
          )}

          <form onSubmit={handleSubmitPatrol}>
            {/* Template Gang / Lokasi Selector */}
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontWeight: 600, fontSize: 14, marginBottom: 6 }}>
                Pilih Lokasi Gang / Pos Patroli:
              </label>
              <select
                value={selectedTemplateId}
                onChange={(e) => setSelectedTemplateId(e.target.value)}
                disabled={submitting}
                style={{
                  width: '100%',
                  padding: '8px 12px',
                  borderRadius: 'var(--sf-radius-sm)',
                  border: '1px solid var(--sf-border)',
                  fontSize: 14,
                  backgroundColor: 'var(--sf-bg-surface)',
                }}
              >
                {templates.filter((t) => t.is_active).map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name} ({t.location_label})
                  </option>
                ))}
              </select>
            </div>

            {/* Photo Capture / Upload Button */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', fontWeight: 600, fontSize: 14, marginBottom: 6 }}>
                Ambil Foto Dokumentasi (Maks. 5MB per foto):
              </label>
              <input
                type="file"
                ref={fileInputRef}
                accept="image/*"
                capture="environment"
                multiple
                onChange={handleFileChange}
                style={{ display: 'none' }}
              />
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={() => fileInputRef.current?.click()}
                disabled={submitting}
                style={{ display: 'inline-flex', alignItems: 'center', gap: 8, padding: '10px 18px', fontSize: 14 }}
              >
                <CameraOutlined style={{ fontSize: 18 }} />
                Ambil Foto atau Pilih Gambar
              </button>
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4 }}>
                Bisa mengambil langsung melalui kamera HP atau mengunggah dari galeri/komputer.
              </div>
            </div>

            {/* Staged Photos Preview Grid */}
            {stagedPhotos.length > 0 && (
              <div style={{ marginBottom: 20 }}>
                <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 8 }}>
                  Foto yang akan dikirim ({stagedPhotos.length}):
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 12 }}>
                  {stagedPhotos.map((item, index) => (
                    <div
                      key={item.id}
                      style={{
                        border: '1px solid var(--sf-border)',
                        borderRadius: 8,
                        overflow: 'hidden',
                        backgroundColor: 'var(--sf-bg-surface-secondary)',
                      }}
                    >
                      <div style={{ position: 'relative', height: 120 }}>
                        <img
                          src={item.previewUrl}
                          alt={`Foto ${index + 1}`}
                          style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                        />
                        <button
                          type="button"
                          onClick={() => handleRemoveStagedPhoto(item.id)}
                          aria-label="Hapus foto"
                          style={{
                            position: 'absolute',
                            top: 6,
                            right: 6,
                            backgroundColor: 'rgba(0, 0, 0, 0.65)',
                            color: '#fff',
                            border: 'none',
                            borderRadius: '50%',
                            width: 26,
                            height: 26,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            cursor: 'pointer',
                          }}
                        >
                          <DeleteOutlined style={{ fontSize: 12 }} />
                        </button>
                      </div>
                      <div style={{ padding: 8 }}>
                        <select
                          value={item.templateId}
                          onChange={(e) => handleUpdatePhotoTemplate(item.id, e.target.value)}
                          disabled={submitting}
                          style={{
                            width: '100%',
                            fontSize: 12,
                            padding: '4px 6px',
                            borderRadius: 4,
                            border: '1px solid var(--sf-border)',
                          }}
                        >
                          {templates.filter((t) => t.is_active).map((t) => (
                            <option key={t.id} value={t.id}>
                              {t.name}
                            </option>
                          ))}
                        </select>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Notes */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', fontWeight: 600, fontSize: 14, marginBottom: 6 }}>
                Catatan Laporan (Opsional):
              </label>
              <textarea
                rows={3}
                placeholder="Tuliskan keterangan situasi wilayah, kondisi portal, dll..."
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                disabled={submitting}
                style={{
                  width: '100%',
                  padding: '8px 12px',
                  borderRadius: 'var(--sf-radius-sm)',
                  border: '1px solid var(--sf-border)',
                  fontSize: 14,
                  resize: 'vertical',
                }}
              />
            </div>

            {/* Progress / Status */}
            {uploadProgress && (
              <div className="sf-loading-banner">
                <LoadingOutlined spin />
                <span style={{ marginLeft: 8 }}>{uploadProgress}</span>
              </div>
            )}

            {/* Submit Buttons */}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={() => {
                  stagedPhotos.forEach((p) => URL.revokeObjectURL(p.previewUrl))
                  setStagedPhotos([])
                  setNotes('')
                  setActiveTab('history')
                }}
                disabled={submitting}
              >
                Batal
              </button>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={submitting || stagedPhotos.length === 0}
                style={{ minWidth: 160 }}
              >
                {submitting ? 'Mengirim...' : 'Kirim Laporan'}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* ═══════════════════════════════════════════════════════════════════ */}
      {/* ── TAB 3: KELOLA TEMPLATE LOKASI ────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {activeTab === 'templates' && canManageTemplates && (
        <div>
          {/* Toolbar */}
          <div className="sf-list-toolbar" style={{ justifyContent: 'space-between' }}>
            <div className="sf-list-toolbar-left">
              <h2 style={{ fontSize: 16, fontWeight: 700, margin: 0 }}>
                Daftar Titik / Gang Patroli
              </h2>
            </div>
            <div className="sf-list-toolbar-right">
              <button
                className="sf-btn-primary"
                onClick={() => {
                  setEditingTemplate(null)
                  setTemplateForm({ name: '', location_label: '', is_active: true })
                  setTemplateError(null)
                  setShowTemplateModal(true)
                }}
                style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '6px 14px', fontSize: '12px' }}
              >
                <PlusOutlined style={{ fontSize: 12 }} />
                Tambah Titik Patroli
              </button>
            </div>
          </div>

          {/* Template Table */}
          <div className="sf-content-surface sf-content-surface-padding" style={{ padding: 0 }}>
            {templates.length === 0 && (
              <div className="sf-empty-banner">
                Belum ada titik patroli. Tambahkan titik patroli untuk memulai.
              </div>
            )}

            {templates.length > 0 && (
              <div className="table-wrapper">
                <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
                  <thead>
                    <tr>
                      <th className="col-name" style={tplColStyles.name}>NAMA TITIK</th>
                      <th className="col-destination" style={tplColStyles.location}>LABEL LOKASI</th>
                      <th className="col-status" style={tplColStyles.status}>STATUS</th>
                      <th className="col-action" style={tplColStyles.action}>AKSI</th>
                    </tr>
                  </thead>
                  <tbody>
                    {templates.map((tpl) => (
                      <tr key={tpl.id}>
                        <td style={{ ...tplColStyles.name, fontWeight: 600 }}>
                          {tpl.name}
                        </td>
                        <td style={tplColStyles.location}>
                          {tpl.location_label}
                        </td>
                        <td style={{ ...tplColStyles.status, textAlign: 'center' }}>
                          <Badge variant={tpl.is_active ? 'green' : 'gray'}>
                            {tpl.is_active ? 'Aktif' : 'Non-Aktif'}
                          </Badge>
                        </td>
                        <td style={{ ...tplColStyles.action, textAlign: 'right' }}>
                          <button
                            className="sf-btn-secondary"
                            style={{ padding: '4px 10px', fontSize: '12px', marginRight: 8 }}
                            onClick={() => {
                              setEditingTemplate(tpl)
                              setTemplateForm({
                                name: tpl.name,
                                location_label: tpl.location_label,
                                is_active: tpl.is_active,
                              })
                              setTemplateError(null)
                              setShowTemplateModal(true)
                            }}
                          >
                            Ubah
                          </button>
                          <button
                            className="sf-btn-secondary"
                            style={{ padding: '4px 10px', fontSize: '12px', color: 'var(--danger, #ff4d4f)' }}
                            onClick={() => handleDeleteTemplate(tpl)}
                          >
                            Hapus
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ═══════════════════════════════════════════════════════════════════ */}
      {/* ── MODAL: DETAIL PATROLI ────────────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {selectedBundle && (
        <Modal open={!!selectedBundle} onClose={() => setSelectedBundle(null)}>
          <ModalHeader
            title={`Detail Laporan Patroli`}
            onClose={() => setSelectedBundle(null)}
          />
          <ModalBody>
            {detailLoading ? (
              <div style={{ padding: 32, textAlign: 'center' }}>
                <LoadingOutlined style={{ fontSize: 28 }} />
              </div>
            ) : (
              <div>
                <div style={{ marginBottom: 16 }}>
                  <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Petugas Pelapor:</div>
                  <div style={{ fontWeight: 600, fontSize: 15 }}>
                    {selectedBundle.duty_officer_name || selectedBundle.created_by_name || 'Petugas Keamanan'}
                  </div>
                </div>

                <div style={{ marginBottom: 16 }}>
                  <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Waktu Laporan:</div>
                  <div style={{ fontSize: 14 }}>
                    {formatJakartaDateTime(selectedBundle.submitted_at || selectedBundle.created_at)}
                  </div>
                </div>

                {selectedBundle.notes && (
                  <div style={{ marginBottom: 20 }}>
                    <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Catatan:</div>
                    <div style={{ fontSize: 14, backgroundColor: 'var(--sf-bg-surface-secondary)', padding: 10, borderRadius: 6 }}>
                      {selectedBundle.notes}
                    </div>
                  </div>
                )}

                <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 10 }}>
                  Foto Dokumentasi ({selectedBundle.photos?.length || 0}):
                </div>

                {(!selectedBundle.photos || selectedBundle.photos.length === 0) ? (
                  <div style={{ color: 'var(--sf-text-muted)', fontSize: 13 }}>Tidak ada foto.</div>
                ) : (
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: 12 }}>
                    {selectedBundle.photos.map((photo, pIdx) => (
                      <div
                        key={photo.id}
                        style={{
                          border: '1px solid var(--sf-border)',
                          borderRadius: 8,
                          overflow: 'hidden',
                          cursor: 'pointer',
                        }}
                        onClick={() => handleOpenLightbox(selectedBundle.photos, pIdx)}
                      >
                        <SecurityImage
                          photoId={photo.id}
                          domain="patrol"
                          alt={photo.template_name || 'Foto'}
                          style={{ width: '100%', height: 110, objectFit: 'cover' }}
                        />
                        <div style={{ padding: 6, fontSize: 11, textAlign: 'center' }}>
                          <div style={{ fontWeight: 600 }}>{photo.template_name || 'Gang'}</div>
                          <div style={{ color: 'var(--sf-text-muted)' }}>{photo.location_label}</div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </ModalBody>
          <ModalFooter>
            <button className="sf-btn-primary" onClick={() => setSelectedBundle(null)}>
              Tutup
            </button>
          </ModalFooter>
        </Modal>
      )}

      {/* ═══════════════════════════════════════════════════════════════════ */}
      {/* ── MODAL: TAMBAH/UBAH TEMPLATE ──────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {showTemplateModal && (
        <Modal open={showTemplateModal} onClose={() => setShowTemplateModal(false)}>
          <ModalHeader
            title={editingTemplate ? 'Ubah Titik Patroli' : 'Tambah Titik Patroli'}
            onClose={() => setShowTemplateModal(false)}
          />
          <form onSubmit={handleSaveTemplate}>
            <ModalBody>
              {templateError && (
                <div className="sf-error-banner" role="alert">
                  {templateError}
                </div>
              )}
              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontWeight: 600, fontSize: 13, marginBottom: 4 }}>
                  Nama Titik (Contoh: Gang Mawar, Pos Ronda 1):
                </label>
                <input
                  type="text"
                  required
                  value={templateForm.name}
                  onChange={(e) => setTemplateForm({ ...templateForm, name: e.target.value })}
                  disabled={templateSubmitting}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    borderRadius: 'var(--sf-radius-sm)',
                    border: '1px solid var(--sf-border)',
                    fontSize: 14,
                  }}
                />
              </div>

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontWeight: 600, fontSize: 13, marginBottom: 4 }}>
                  Label Lokasi (Contoh: RT 002 Barat, Blok C):
                </label>
                <input
                  type="text"
                  required
                  value={templateForm.location_label}
                  onChange={(e) => setTemplateForm({ ...templateForm, location_label: e.target.value })}
                  disabled={templateSubmitting}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    borderRadius: 'var(--sf-radius-sm)',
                    border: '1px solid var(--sf-border)',
                    fontSize: 14,
                  }}
                />
              </div>

              {editingTemplate && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}>
                  <input
                    type="checkbox"
                    id="template_is_active"
                    checked={templateForm.is_active}
                    onChange={(e) => setTemplateForm({ ...templateForm, is_active: e.target.checked })}
                  />
                  <label htmlFor="template_is_active" style={{ fontSize: 14 }}>
                    Aktif digunakan untuk laporan patroli
                  </label>
                </div>
              )}
            </ModalBody>
            <ModalFooter>
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={() => setShowTemplateModal(false)}
                disabled={templateSubmitting}
              >
                Batal
              </button>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={templateSubmitting}
              >
                {templateSubmitting ? 'Menyimpan...' : 'Simpan'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* ── LIGHTBOX ──────────────────────────────────────────────────────── */}
      <SecurityPhotoLightbox
        photos={lightboxPhotos}
        initialIndex={lightboxIndex}
        domain="patrol"
        open={lightboxOpen}
        onClose={() => setLightboxOpen(false)}
      />
    </div>
  )
}
