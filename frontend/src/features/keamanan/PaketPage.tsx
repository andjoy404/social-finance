import React, { useState, useEffect, useCallback, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  InboxOutlined,
  PlusOutlined,
  CameraOutlined,
  CheckCircleOutlined,
  ReloadOutlined,
  UserOutlined,
  LoadingOutlined,
} from '@ant-design/icons'
import { useAuth, hasPermission } from '@/app/AuthContext'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { Pagination } from '@/components/Pagination'
import {
  PaketReport,
  PaketItem,
  MaskedWargaSearchResult,
  apiListPaketReports,
  apiGetPaketReport,
  apiUploadPaketPhoto,
  apiCreatePaketReport,
  apiUpdatePaketItemStatus,
  apiSearchWargaMasked,
  apiMarkSecurityReadReceipt,
} from './securityApi'
import { SecurityImage, SecurityPhotoLightbox, LightboxPhoto } from './SecurityImage'
import { formatJakartaDateTime } from '@/utils/jakartaTime'

interface DraftItem {
  id: string
  description: string
  file: File | null
  previewUrl: string | null
}

// Column width styles matching Warga's approach
const colStyles = {
  receiver: { minWidth: 180, textAlign: 'left' as const, whiteSpace: 'nowrap' as const },
  itemsCount: { width: 110, minWidth: 110, maxWidth: 110, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  status: { width: 130, minWidth: 130, maxWidth: 130, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  photo: { width: 130, minWidth: 130, maxWidth: 130, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
  createdAt: { width: 170, minWidth: 170, maxWidth: 170, textAlign: 'left' as const, fontFamily: 'monospace', whiteSpace: 'nowrap' as const },
  action: { width: 140, minWidth: 140, maxWidth: 140, textAlign: 'center' as const, whiteSpace: 'nowrap' as const },
}

export function PaketPage() {
  const { user } = useAuth()
  const [searchParams] = useSearchParams()

  const canRead = hasPermission(user, 'paket.read') || hasPermission(user, 'paket.create')
  const canCreate = hasPermission(user, 'paket.create')
  const canStatusChange = hasPermission(user, 'paket.status_change')

  // Tab: 'list' | 'create'
  const [activeTab, setActiveTab] = useState<'list' | 'create'>('list')

  // List States
  const [reports, setReports] = useState<PaketReport[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [totalCount, setTotalCount] = useState(0)
  const [statusFilter, setStatusFilter] = useState<'all' | 'dititipkan' | 'sudah_diambil' | 'dibatalkan'>('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Detail Modal
  const [selectedReport, setSelectedReport] = useState<PaketReport | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  // Status Change (Serah Terima) Modal
  const [statusTargetItem, setStatusTargetItem] = useState<{ reportId: string; item: PaketItem } | null>(null)
  const [proofFile, setProofFile] = useState<File | null>(null)
  const [proofPreviewUrl, setProofPreviewUrl] = useState<string | null>(null)
  const [statusSubmitting, setStatusSubmitting] = useState(false)
  const [statusError, setStatusError] = useState<string | null>(null)
  const proofInputRef = useRef<HTMLInputElement>(null)

  // Create Form States
  const [wargaQuery, setWargaQuery] = useState('')
  const [wargaResults, setWargaResults] = useState<MaskedWargaSearchResult[]>([])
  const [wargaSearching, setWargaSearching] = useState(false)
  const [selectedWarga, setSelectedWarga] = useState<MaskedWargaSearchResult | null>(null)
  const [draftItems, setDraftItems] = useState<DraftItem[]>([
    { id: 'item-0', description: '', file: null, previewUrl: null },
  ])
  const [createSubmitting, setCreateSubmitting] = useState(false)
  const [createProgress, setCreateProgress] = useState<string | null>(null)
  const [createError, setCreateError] = useState<string | null>(null)
  const [createSuccess, setCreateSuccess] = useState<string | null>(null)

  // Lightbox
  const [lightboxPhotos, setLightboxPhotos] = useState<LightboxPhoto[]>([])
  const [lightboxIndex, setLightboxIndex] = useState(0)
  const [lightboxOpen, setLightboxOpen] = useState(false)

  // Load Reports
  const loadReports = useCallback(async (targetPage = 1, targetPageSize = pageSize) => {
    try {
      setLoading(true)
      setError(null)
      const res = await apiListPaketReports({ page: targetPage, per_page: targetPageSize })
      setReports(res.data || [])
      setPage(res.pagination.page)
      setTotalCount(res.pagination.total)
    } catch (err: any) {
      setError(err?.message || 'Gagal memuat daftar paket')
    } finally {
      setLoading(false)
    }
  }, [pageSize])

  // Initial Load & Read Receipt
  useEffect(() => {
    if (canRead) {
      loadReports(1, pageSize)
      apiMarkSecurityReadReceipt('paket').catch(() => {})
    }
  }, [canRead, loadReports, pageSize])

  // Deep-link `?reportId=...`
  useEffect(() => {
    const reportId = searchParams.get('reportId')
    if (reportId) {
      setDetailLoading(true)
      apiGetPaketReport(reportId)
        .then((r) => setSelectedReport(r))
        .catch(() => setSelectedReport(null))
        .finally(() => setDetailLoading(false))
    }
  }, [searchParams])

  // Warga search debounce
  useEffect(() => {
    if (!wargaQuery.trim()) {
      setWargaResults([])
      return
    }
    const timer = setTimeout(async () => {
      try {
        setWargaSearching(true)
        const res = await apiSearchWargaMasked(wargaQuery.trim(), 10)
        setWargaResults(res.data || [])
      } catch {
        setWargaResults([])
      } finally {
        setWargaSearching(false)
      }
    }, 300)
    return () => clearTimeout(timer)
  }, [wargaQuery])

  // Open Detail
  const handleOpenDetail = async (report: PaketReport) => {
    try {
      setDetailLoading(true)
      const full = await apiGetPaketReport(report.id)
      setSelectedReport(full)
    } catch {
      setSelectedReport(report)
    } finally {
      setDetailLoading(false)
    }
  }

  // Lightbox
  const handleOpenLightbox = (photos: LightboxPhoto[], startIndex = 0) => {
    if (!photos || photos.length === 0) return
    setLightboxPhotos(photos)
    setLightboxIndex(startIndex)
    setLightboxOpen(true)
  }

  // Add Item in Create Form
  const handleAddDraftItem = () => {
    setDraftItems((prev) => [
      ...prev,
      { id: `item-${Date.now()}`, description: '', file: null, previewUrl: null },
    ])
  }

  const handleRemoveDraftItem = (id: string) => {
    if (draftItems.length <= 1) return
    setDraftItems((prev) => {
      const found = prev.find((item) => item.id === id)
      if (found && found.previewUrl) URL.revokeObjectURL(found.previewUrl)
      return prev.filter((item) => item.id !== id)
    })
  }

  const handleItemFileChange = (id: string, file: File | null) => {
    if (!file) return
    if (file.size > 5 * 1024 * 1024) {
      setCreateError('Foto melebihi ukuran maksimum 5MB')
      return
    }
    const previewUrl = URL.createObjectURL(file)
    setDraftItems((prev) =>
      prev.map((item) => {
        if (item.id === id) {
          if (item.previewUrl) URL.revokeObjectURL(item.previewUrl)
          return { ...item, file, previewUrl }
        }
        return item
      })
    )
  }

  // Submit Paket Report
  const handleSubmitPaket = async (e: React.FormEvent) => {
    e.preventDefault()
    if (createSubmitting) return

    if (!selectedWarga) {
      setCreateError('Pilih warga penerima paket')
      return
    }

    // Validate draft items
    for (let i = 0; i < draftItems.length; i++) {
      const item = draftItems[i]
      if (!item.file) {
        setCreateError(`Foto untuk paket ke-${i + 1} wajib diunggah`)
        return
      }
    }

    try {
      setCreateSubmitting(true)
      setCreateError(null)
      setCreateProgress('Mengunggah foto-foto paket...')

      const itemsPayload: { description?: string; image_keys: string[] }[] = []

      for (let i = 0; i < draftItems.length; i++) {
        const item = draftItems[i]
        setCreateProgress(`Mengunggah foto paket ${i + 1} dari ${draftItems.length}...`)
        const uploadRes = await apiUploadPaketPhoto(item.file!)
        itemsPayload.push({
          description: item.description.trim() || undefined,
          image_keys: [uploadRes.image_key],
        })
      }

      setCreateProgress('Menyimpan penerimaan paket...')
      await apiCreatePaketReport({
        receiver_warga_id: selectedWarga.id,
        items: itemsPayload,
      })

      // Clean up
      draftItems.forEach((it) => it.previewUrl && URL.revokeObjectURL(it.previewUrl))
      setDraftItems([{ id: 'item-0', description: '', file: null, previewUrl: null }])
      setSelectedWarga(null)
      setWargaQuery('')
      setCreateSuccess('Penerimaan paket berhasil dicatat!')
      setActiveTab('list')
      loadReports(1, pageSize)
    } catch (err: any) {
      setCreateError(err?.message || 'Gagal menyimpan penerimaan paket')
    } finally {
      setCreateSubmitting(false)
      setCreateProgress(null)
    }
  }

  // Serah Terima Submit (with Proof Photo)
  const handleConfirmPickup = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!statusTargetItem || statusSubmitting) return

    if (!proofFile) {
      setStatusError('Foto bukti pengambilan wajib disertakan saat penyerahan paket')
      return
    }

    try {
      setStatusSubmitting(true)
      setStatusError(null)

      const uploadRes = await apiUploadPaketPhoto(proofFile)
      const updatedReport = await apiUpdatePaketItemStatus(
        statusTargetItem.reportId,
        statusTargetItem.item.id,
        {
          status: 'sudah_diambil',
          proof_image_key: uploadRes.image_key,
        }
      )

      setSelectedReport(updatedReport)
      if (proofPreviewUrl) URL.revokeObjectURL(proofPreviewUrl)
      setProofFile(null)
      setProofPreviewUrl(null)
      setStatusTargetItem(null)
      loadReports(page, pageSize)
    } catch (err: any) {
      setStatusError(err?.message || 'Gagal memperbarui status penyerahan paket')
    } finally {
      setStatusSubmitting(false)
    }
  }

  const handleCancelItem = async (reportId: string, item: PaketItem) => {
    if (!window.confirm('Yakin ingin membatalkan status paket ini?')) return
    try {
      const updated = await apiUpdatePaketItemStatus(reportId, item.id, {
        status: 'dibatalkan',
      })
      setSelectedReport(updated)
      loadReports(page, pageSize)
    } catch (err: any) {
      alert(err?.message || 'Gagal membatalkan paket')
    }
  }

  // Filtered reports (client-side)
  const filteredReports = reports.filter((r) => {
    if (statusFilter !== 'all' && r.overall_status !== statusFilter) return false
    if (!searchQuery) return true
    const q = searchQuery.toLowerCase()
    const recName = (r.receiver_name || '').toLowerCase()
    const recHouse = (r.receiver_house_number || '').toLowerCase()
    const officer = (r.created_by_name || '').toLowerCase()
    const itemDesc = (r.items || []).some((it) => (it.description || '').toLowerCase().includes(q))
    return recName.includes(q) || recHouse.includes(q) || officer.includes(q) || itemDesc
  })

  const handleSearch = useCallback((value: string) => {
    setSearchQuery(value)
    setPage(1)
  }, [])

  const handlePageChange = useCallback((p: number) => {
    if (p >= 1) {
      setPage(p)
      void loadReports(p, pageSize)
    }
  }, [loadReports, pageSize])

  const handlePageSizeChange = useCallback((newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }, [])

  const handleRefresh = useCallback(() => {
    void loadReports(page, pageSize)
  }, [loadReports, page, pageSize])

  const handleStatusFilter = useCallback((s: 'all' | 'dititipkan' | 'sudah_diambil' | 'dibatalkan') => {
    setStatusFilter(s)
    setPage(1)
  }, [])

  const totalPages = Math.ceil(totalCount / pageSize) || 1

  if (!canRead && !canCreate) {
    return (
      <div style={{ padding: 32, textAlign: 'center' }}>
        <h3>Akses Ditolak</h3>
        <p style={{ color: 'var(--sf-text-muted)' }}>Anda tidak memiliki hak akses untuk membuka fitur Paket.</p>
      </div>
    )
  }

  return (
    <div>
      {/* ── Page Header ──────────────────────────────────────────────────── */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <InboxOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Titipan Paket Warga</div>
            <div className="sf-page-header-subtitle">
              Pencatatan titipan paket kurir dan serah terima paket di pos keamanan RT.
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
          onClick={() => setActiveTab('list')}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '10px 16px',
            border: 'none',
            borderBottom: activeTab === 'list' ? '2px solid var(--sf-accent)' : '2px solid transparent',
            background: activeTab === 'list' ? 'var(--sf-surface-hover)' : 'transparent',
            color: activeTab === 'list' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
            cursor: 'pointer',
            fontSize: 13,
            fontWeight: activeTab === 'list' ? 600 : 400,
            borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
            transition: 'all 0.2s ease',
          }}
        >
          <InboxOutlined aria-hidden="true" />
          DAFTAR PAKET
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
            <PlusOutlined aria-hidden="true" />
            TERIMA PAKET BARU
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
      {/* ── TAB 1: DAFTAR PAKET ──────────────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {activeTab === 'list' && (
        <div className="sf-content-surface">
          {/* Toolbar */}
          <div className="sf-list-toolbar">
            <div className="sf-list-toolbar-left">
              <SearchBox
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={handleSearch}
                placeholder="Cari penerima / deskripsi..."
              />
            </div>
            <div className="sf-list-toolbar-right" style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
              {/* Status filter buttons */}
              {[
                { value: 'all' as const, label: 'Semua' },
                { value: 'dititipkan' as const, label: 'Dititipkan' },
                { value: 'sudah_diambil' as const, label: 'Sudah Diambil' },
                { value: 'dibatalkan' as const, label: 'Dibatalkan' },
              ].map((opt) => (
                <button
                  key={opt.value}
                  className={`sf-btn ${statusFilter === opt.value ? 'sf-btn-primary' : 'sf-btn-outline'}`}
                  onClick={() => handleStatusFilter(opt.value)}
                  style={{ padding: '6px 14px', fontSize: '12px' }}
                >
                  {opt.label}
                </button>
              ))}
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
          {error && (
            <div className="sf-error-banner" role="alert">
              {error}
            </div>
          )}

          {/* Loading */}
          {loading && (
            <div className="sf-loading-banner">
              Memuat daftar paket...
            </div>
          )}

          {/* Empty */}
          {!loading && !error && filteredReports.length === 0 && (
            <div className="sf-empty-banner">
              Belum ada paket yang tercatat.
            </div>
          )}

          {/* Paket Table */}
          {!loading && filteredReports.length > 0 && (
            <div className="table-wrapper">
              <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
                <thead>
                  <tr>
                    <th className="col-receiver" style={colStyles.receiver}>PENERIMA</th>
                    <th className="col-items-count" style={colStyles.itemsCount}>ITEM</th>
                    <th className="col-status" style={colStyles.status}>STATUS</th>
                    <th className="col-photo" style={colStyles.photo}>FOTO</th>
                    <th className="col-created-at" style={colStyles.createdAt}>WAKTU MASUK</th>
                    <th className="col-action" style={colStyles.action}>AKSI</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredReports.map((row) => {
                    const isPickedUp = row.overall_status === 'sudah_diambil'
                    const isCancelled = row.overall_status === 'dibatalkan'
                    const statusBadge: 'green' | 'gray' | 'amber' = isPickedUp ? 'green' : isCancelled ? 'gray' : 'amber'
                    const statusLabel = isPickedUp ? 'Sudah Diambil' : isCancelled ? 'Dibatalkan' : 'Dititipkan'

                    const reportPhotos: LightboxPhoto[] = []
                    for (const item of row.items || []) {
                      for (const p of item.photos || []) {
                        reportPhotos.push({
                          id: p.id,
                          title: p.photo_type === 'bukti_pengambilan' ? 'Bukti Serah Terima' : 'Foto Paket',
                          location: item.description || undefined,
                          timestamp: p.captured_at,
                          photoType: p.photo_type,
                        })
                      }
                    }

                    return (
                      <tr key={row.id}>
                        {/* 1. PENERIMA */}
                        <td style={colStyles.receiver}>
                          <div>
                            <strong>{row.receiver_name || 'Warga RT'}</strong>{' '}
                            {row.receiver_house_number && (
                              <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                                (Rumah No. {row.receiver_house_number})
                              </span>
                            )}
                          </div>
                        </td>

                        {/* 2. ITEM */}
                        <td style={colStyles.itemsCount}>
                          <Badge variant="blue">{row.items?.length || 0} Item</Badge>
                        </td>

                        {/* 3. STATUS */}
                        <td style={colStyles.status}>
                          <Badge variant={statusBadge}>{statusLabel}</Badge>
                        </td>

                        {/* 4. FOTO */}
                        <td style={colStyles.photo}>
                          {reportPhotos.length === 0 ? (
                            '\u2014'
                          ) : (
                            <div
                              style={{
                                width: 44,
                                height: 44,
                                cursor: 'pointer',
                                borderRadius: 4,
                                overflow: 'hidden',
                                border: '1px solid var(--sf-border)',
                                margin: '0 auto',
                              }}
                              onClick={() => handleOpenLightbox(reportPhotos, 0)}
                              title="Klik untuk melihat foto penuh"
                            >
                              <SecurityImage
                                photoId={reportPhotos[0].id}
                                domain="paket"
                                style={{ width: 44, height: 44, objectFit: 'cover' }}
                              />
                            </div>
                          )}
                        </td>

                        {/* 5. WAKTU MASUK */}
                        <td style={colStyles.createdAt}>
                          {formatJakartaDateTime(row.created_at)}
                        </td>

                        {/* 6. AKSI */}
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
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}

          {/* Pagination */}
          {!loading && filteredReports.length > 0 && (
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
      {/* ── TAB 2: TERIMA PAKET BARU ─────────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {activeTab === 'create' && canCreate && (
        <div className="sf-content-surface sf-content-surface-padding" style={{ padding: 24 }}>
          <h2 style={{ fontSize: 18, fontWeight: 700, margin: '0 0 16px' }}>
            Formulir Penerimaan Titipan Paket
          </h2>

          {createError && (
            <div className="sf-error-banner" role="alert">
              {createError}
            </div>
          )}

          <form onSubmit={handleSubmitPaket}>
            {/* Warga Autocomplete */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', fontWeight: 600, fontSize: 14, marginBottom: 6 }}>
                Penerima / Pemilik Paket (Warga RT):
              </label>

              {selectedWarga ? (
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '10px 14px',
                    border: '1px solid var(--sf-success-border, #b7eb8f)',
                    backgroundColor: 'var(--sf-success-bg, #f6ffed)',
                    borderRadius: 6,
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <UserOutlined style={{ color: '#52c41a' }} />
                    <span style={{ fontWeight: 600 }}>{selectedWarga.full_name}</span>
                    <span style={{ color: 'var(--sf-text-muted)', fontSize: 13 }}>
                      (Rumah No. {selectedWarga.house_number})
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedWarga(null)
                      setWargaQuery('')
                    }}
                    style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#8c8c8c', fontSize: 16 }}
                  >
                    &times;
                  </button>
                </div>
              ) : (
                <div style={{ position: 'relative' }}>
                  <input
                    type="text"
                    placeholder="Ketik nama warga atau nomor rumah..."
                    value={wargaQuery}
                    onChange={(e) => setWargaQuery(e.target.value)}
                    style={{
                      width: '100%',
                      padding: '8px 12px',
                      borderRadius: 'var(--sf-radius-sm)',
                      border: '1px solid var(--sf-border)',
                      fontSize: 14,
                    }}
                  />
                  {wargaSearching && (
                    <div style={{ position: 'absolute', right: 12, top: 10 }}>
                      <LoadingOutlined spin />
                    </div>
                  )}

                  {wargaResults.length > 0 && (
                    <div
                      style={{
                        position: 'absolute',
                        top: '100%',
                        left: 0,
                        right: 0,
                        backgroundColor: 'var(--sf-bg-surface)',
                        border: '1px solid var(--sf-border)',
                        borderRadius: 6,
                        boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
                        zIndex: 100,
                        marginTop: 4,
                        maxHeight: 200,
                        overflowY: 'auto',
                      }}
                    >
                      {wargaResults.map((w) => (
                        <div
                          key={w.id}
                          onClick={() => {
                            setSelectedWarga(w)
                            setWargaResults([])
                            setWargaQuery('')
                          }}
                          style={{
                            padding: '8px 14px',
                            cursor: 'pointer',
                            borderBottom: '1px solid var(--sf-border, #f0f0f0)',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                          }}
                          onMouseEnter={(e) =>
                            (e.currentTarget.style.backgroundColor = 'var(--sf-bg-surface-secondary)')
                          }
                          onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                        >
                          <span style={{ fontWeight: 600, fontSize: 14 }}>{w.full_name}</span>
                          <span style={{ color: 'var(--sf-text-muted)', fontSize: 13 }}>
                            No. {w.house_number}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Items List */}
            <div style={{ marginBottom: 20 }}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 10,
                }}
              >
                <label style={{ fontWeight: 600, fontSize: 14, margin: 0 }}>
                  Item Paket Titipan:
                </label>
                <button
                  type="button"
                  className="sf-btn-secondary"
                  onClick={handleAddDraftItem}
                  style={{ padding: '6px 14px', fontSize: '12px', display: 'inline-flex', alignItems: 'center', gap: 4 }}
                >
                  <PlusOutlined style={{ fontSize: 12 }} />
                  Tambah Paket Lain
                </button>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
                {draftItems.map((item, idx) => (
                  <div
                    key={item.id}
                    style={{
                      padding: 14,
                      border: '1px solid var(--sf-border)',
                      borderRadius: 8,
                      backgroundColor: 'var(--sf-bg-surface-secondary)',
                    }}
                  >
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        marginBottom: 10,
                      }}
                    >
                      <span style={{ fontWeight: 600, fontSize: 13 }}>Paket #{idx + 1}</span>
                      {draftItems.length > 1 && (
                        <button
                          type="button"
                          onClick={() => handleRemoveDraftItem(item.id)}
                          style={{ background: 'none', border: 'none', color: 'var(--danger, #ff4d4f)', cursor: 'pointer', fontSize: 12 }}
                        >
                          Hapus Item
                        </button>
                      )}
                    </div>

                    <div style={{ marginBottom: 12 }}>
                      <input
                        type="text"
                        placeholder="Keterangan paket (Contoh: Kardus Coklat J&T, Resi JP...)"
                        value={item.description}
                        onChange={(e) => {
                          const val = e.target.value
                          setDraftItems((prev) =>
                            prev.map((it) => (it.id === item.id ? { ...it, description: val } : it))
                          )
                        }}
                        disabled={createSubmitting}
                        style={{
                          width: '100%',
                          padding: '8px 12px',
                          borderRadius: 'var(--sf-radius-sm)',
                          border: '1px solid var(--sf-border)',
                          fontSize: 13,
                        }}
                      />
                    </div>

                    {/* Photo Upload for Item */}
                    <div>
                      {item.previewUrl ? (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                          <img
                            src={item.previewUrl}
                            alt="Bukti Paket"
                            style={{ width: 80, height: 80, objectFit: 'cover', borderRadius: 6 }}
                          />
                          <div>
                            <button
                              type="button"
                              className="sf-btn-secondary"
                              onClick={() => {
                                if (item.previewUrl) URL.revokeObjectURL(item.previewUrl)
                                setDraftItems((prev) =>
                                  prev.map((it) =>
                                    it.id === item.id ? { ...it, file: null, previewUrl: null } : it
                                  )
                                )
                              }}
                              style={{ padding: '6px 14px', fontSize: '12px', color: 'var(--danger, #ff4d4f)' }}
                            >
                              Ganti Foto
                            </button>
                          </div>
                        </div>
                      ) : (
                        <div>
                          <label
                            className="sf-btn-secondary"
                            style={{
                              display: 'inline-flex',
                              alignItems: 'center',
                              gap: 6,
                              cursor: 'pointer',
                              padding: '8px 14px',
                              fontSize: 13,
                            }}
                          >
                            <CameraOutlined style={{ fontSize: 14 }} />
                            Foto Paket (Wajib)
                            <input
                              type="file"
                              accept="image/*"
                              capture="environment"
                              onChange={(e) => {
                                const f = e.target.files?.[0]
                                if (f) handleItemFileChange(item.id, f)
                              }}
                              style={{ display: 'none' }}
                            />
                          </label>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Progress */}
            {createProgress && (
              <div className="sf-loading-banner">
                <LoadingOutlined spin style={{ marginRight: 4 }} />
                <span>{createProgress}</span>
              </div>
            )}

            {/* Submit Actions */}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={() => {
                  draftItems.forEach((it) => it.previewUrl && URL.revokeObjectURL(it.previewUrl))
                  setDraftItems([{ id: 'item-0', description: '', file: null, previewUrl: null }])
                  setSelectedWarga(null)
                  setActiveTab('list')
                }}
                disabled={createSubmitting}
              >
                Batal
              </button>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={createSubmitting || !selectedWarga}
                style={{ minWidth: 160 }}
              >
                {createSubmitting ? 'Menyimpan...' : 'Simpan Penerimaan'}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* ═══════════════════════════════════════════════════════════════════ */}
      {/* ── MODAL: DETAIL PAKET ──────────────────────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {selectedReport && (
        <Modal open={!!selectedReport} onClose={() => setSelectedReport(null)}>
          <ModalHeader
            title="Detail Titipan Paket"
            onClose={() => setSelectedReport(null)}
          />
          <ModalBody>
            {detailLoading ? (
              <div style={{ padding: 32, textAlign: 'center' }}>
                <LoadingOutlined style={{ fontSize: 28 }} />
              </div>
            ) : (
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
                  <div>
                    <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Penerima / Pemilik:</div>
                    <div style={{ fontWeight: 700, fontSize: 16 }}>
                      {selectedReport.receiver_name || 'Warga RT'}
                    </div>
                    {selectedReport.receiver_house_number && (
                      <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
                        Nomor Rumah: {selectedReport.receiver_house_number}
                      </div>
                    )}
                  </div>
                  <div>
                    <Badge
                      variant={
                        selectedReport.overall_status === 'sudah_diambil'
                          ? 'green'
                          : selectedReport.overall_status === 'dibatalkan'
                          ? 'gray'
                          : 'amber'
                      }
                    >
                      {selectedReport.overall_status === 'sudah_diambil'
                        ? 'Sudah Diambil'
                        : selectedReport.overall_status === 'dibatalkan'
                        ? 'Dibatalkan'
                        : 'Dititipkan'}
                    </Badge>
                  </div>
                </div>

                <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginBottom: 16 }}>
                  Diterima pada: {formatJakartaDateTime(selectedReport.created_at)} oleh{' '}
                  <strong>{selectedReport.created_by_name || 'Petugas'}</strong>
                </div>

                <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 10 }}>
                  Daftar Paket ({selectedReport.items?.length || 0} Item):
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                  {(selectedReport.items || []).map((item, idx) => {
                    const isPending = item.status === 'dititipkan'
                    const isPickedUp = item.status === 'sudah_diambil'
                    const isCancelled = item.status === 'dibatalkan'

                    const allPhotos: LightboxPhoto[] = (item.photos || []).map((p) => ({
                      id: p.id,
                      title: p.photo_type === 'bukti_pengambilan' ? 'Bukti Serah Terima' : 'Foto Paket',
                      location: item.description || undefined,
                      timestamp: p.captured_at,
                      photoType: p.photo_type,
                    }))

                    return (
                      <div
                        key={item.id}
                        style={{
                          border: '1px solid var(--sf-border)',
                          borderRadius: 8,
                          padding: 12,
                          backgroundColor: 'var(--sf-bg-surface-secondary)',
                        }}
                      >
                        <div
                          style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center',
                            marginBottom: 8,
                          }}
                        >
                          <span style={{ fontWeight: 600, fontSize: 14 }}>
                            Item #{idx + 1}: {item.description || 'Paket Titipan'}
                          </span>
                          <Badge
                            variant={
                              isPickedUp ? 'green' : isCancelled ? 'gray' : 'amber'
                            }
                          >
                            {isPickedUp ? 'Sudah Diambil' : isCancelled ? 'Dibatalkan' : 'Dititipkan'}
                          </Badge>
                        </div>

                        {/* Status history info if picked up */}
                        {isPickedUp && item.status_changed_at && (
                          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 8 }}>
                            Diserahkan pada: {formatJakartaDateTime(item.status_changed_at)}
                            {item.changed_by_name && ` oleh ${item.changed_by_name}`}
                          </div>
                        )}

                        {/* Photos gallery */}
                        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 10 }}>
                          {(item.photos || []).map((photo, pIdx) => (
                            <div
                              key={photo.id}
                              style={{ cursor: 'pointer', textAlign: 'center' }}
                              onClick={() => handleOpenLightbox(allPhotos, pIdx)}
                            >
                              <SecurityImage
                                photoId={photo.id}
                                domain="paket"
                                alt="Foto Paket"
                                style={{ width: 90, height: 90, objectFit: 'cover' }}
                              />
                              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
                                {photo.photo_type === 'bukti_pengambilan' ? 'Bukti Ambil' : 'Foto Paket'}
                              </div>
                            </div>
                          ))}
                        </div>

                        {/* Operational Actions for Security Officer */}
                        {canStatusChange && isPending && (
                          <div
                            style={{
                              display: 'flex',
                              gap: 8,
                              justifyContent: 'flex-end',
                              borderTop: '1px solid var(--sf-border, #f0f0f0)',
                              paddingTop: 8,
                              marginTop: 4,
                            }}
                          >
                            <button
                              className="sf-btn-secondary"
                              style={{ padding: '4px 10px', fontSize: '12px', color: 'var(--danger, #ff4d4f)' }}
                              onClick={() => handleCancelItem(selectedReport.id, item)}
                            >
                              Batalkan
                            </button>
                            <button
                              className="sf-btn-primary"
                              style={{ padding: '4px 12px', fontSize: '12px' }}
                              onClick={() => {
                                setStatusTargetItem({ reportId: selectedReport.id, item })
                                setProofFile(null)
                                setProofPreviewUrl(null)
                                setStatusError(null)
                              }}
                            >
                              <CheckCircleOutlined style={{ fontSize: 12 }} /> Serah Terima (Diambil)
                            </button>
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>
            )}
          </ModalBody>
          <ModalFooter>
            <button className="sf-btn-primary" onClick={() => setSelectedReport(null)}>
              Tutup
            </button>
          </ModalFooter>
        </Modal>
      )}

      {/* ═══════════════════════════════════════════════════════════════════ */}
      {/* ── MODAL: SERAH TERIMA DENGAN FOTO BUKTI ────────────────────────── */}
      {/* ═══════════════════════════════════════════════════════════════════ */}
      {statusTargetItem && (
        <Modal open={!!statusTargetItem} onClose={() => setStatusTargetItem(null)}>
          <ModalHeader
            title="Konfirmasi Penyerahan Paket"
            onClose={() => setStatusTargetItem(null)}
          />
          <form onSubmit={handleConfirmPickup}>
            <ModalBody>
              <p style={{ margin: '0 0 12px', fontSize: 14 }}>
                Penyerahan paket <strong>{statusTargetItem.item.description || 'titipan'}</strong> kepada warga.
              </p>

              {statusError && (
                <div className="sf-error-banner" role="alert">
                  {statusError}
                </div>
              )}

              <div style={{ marginBottom: 16 }}>
                <label style={{ display: 'block', fontWeight: 600, fontSize: 13, marginBottom: 6 }}>
                  Foto Bukti Pengambilan / Serah Terima (Wajib):
                </label>

                <input
                  type="file"
                  ref={proofInputRef}
                  accept="image/*"
                  capture="environment"
                  onChange={(e) => {
                    const f = e.target.files?.[0]
                    if (!f) return
                    if (f.size > 5 * 1024 * 1024) {
                      setStatusError('Ukuran foto melebihi 5MB')
                      return
                    }
                    if (proofPreviewUrl) URL.revokeObjectURL(proofPreviewUrl)
                    setProofFile(f)
                    setProofPreviewUrl(URL.createObjectURL(f))
                  }}
                  style={{ display: 'none' }}
                />

                {proofPreviewUrl ? (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <img
                      src={proofPreviewUrl}
                      alt="Bukti Pengambilan"
                      style={{ width: 100, height: 100, objectFit: 'cover', borderRadius: 8 }}
                    />
                    <button
                      type="button"
                      className="sf-btn-secondary"
                      onClick={() => {
                        if (proofPreviewUrl) URL.revokeObjectURL(proofPreviewUrl)
                        setProofFile(null)
                        setProofPreviewUrl(null)
                      }}
                      style={{ padding: '6px 14px', fontSize: '12px', color: 'var(--danger, #ff4d4f)' }}
                    >
                      Ganti Foto
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    className="sf-btn-secondary"
                    onClick={() => proofInputRef.current?.click()}
                    style={{ display: 'inline-flex', alignItems: 'center', gap: 8, padding: '8px 14px' }}
                  >
                    <CameraOutlined style={{ fontSize: 14 }} />
                    Ambil Foto Bukti Pengambilan
                  </button>
                )}
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4 }}>
                  Foto warga saat mengambil paket atau tanda tangan serah terima.
                </div>
              </div>
            </ModalBody>
            <ModalFooter>
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={() => setStatusTargetItem(null)}
                disabled={statusSubmitting}
              >
                Batal
              </button>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={statusSubmitting || !proofFile}
              >
                {statusSubmitting ? 'Menyimpan...' : 'Konfirmasi Serah Terima'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* ── LIGHTBOX ──────────────────────────────────────────────────────── */}
      <SecurityPhotoLightbox
        photos={lightboxPhotos}
        initialIndex={lightboxIndex}
        domain="paket"
        open={lightboxOpen}
        onClose={() => setLightboxOpen(false)}
      />
    </div>
  )
}
