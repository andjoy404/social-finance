import { useState, useEffect } from 'react'
import { Modal, ModalHeader, ModalBody } from '@/components/Modal'
import {
  apiGetPaketReport,
  type PaketReport,
} from './securityApi'
import { SecurityImage, SecurityPhotoLightbox } from './SecurityImage'
import { formatJakartaDateTime } from '@/utils/jakartaTime'

export interface PaketDetailModalProps {
  reportId: string | null
  open: boolean
  onClose: () => void
}

/**
 * Modal detail paket yang dapat digunakan di NotificationScreen
 * maupun sebagai fallback deep link. Menggunakan API terautentikasi
 * dan SecurityImage untuk thumbnail foto (menghindari 401).
 */
export function PaketDetailModal({ reportId, open, onClose }: PaketDetailModalProps) {
  const [report, setReport] = useState<PaketReport | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [lightboxPhotos, setLightboxPhotos] = useState<{ id: string; title?: string }[]>([])
  const [lightboxIndex, setLightboxIndex] = useState(0)
  const [showLightbox, setShowLightbox] = useState(false)

  useEffect(() => {
    if (!open || !reportId) return
    setLoading(true)
    setError(null)
    setReport(null)
    apiGetPaketReport(reportId)
      .then((r) => {
        setReport(r)
        setError(null)
      })
      .catch((err: any) => {
        setError(err?.message || 'Gagal memuat detail paket')
        setReport(null)
      })
      .finally(() => setLoading(false))
  }, [open, reportId])

  const handleClose = () => {
    setReport(null)
    setError(null)
    onClose()
  }

  const handleViewPhoto = (photos: Array<{ id: string; title?: string }>, index: number) => {
    setLightboxPhotos(photos)
    setLightboxIndex(index)
    setShowLightbox(true)
  }

  const overallBadge = (status: string) => {
    const isDiambil = status === 'sudah_diambil'
    return (
      <span
        style={{
          display: 'inline-block',
          padding: '2px 10px',
          borderRadius: 6,
          fontSize: 12,
          fontWeight: 700,
          background: isDiambil ? 'rgba(16, 185, 129, 0.12)' : 'rgba(245, 158, 11, 0.12)',
          color: isDiambil ? '#10b981' : '#f59e0b',
          border: `1px solid ${isDiambil ? 'rgba(16,185,129,0.3)' : 'rgba(245,158,11,0.3)'}`,
        }}
      >
        {isDiambil ? 'Sudah Diambil' : 'Dititipkan'}
      </span>
    )
  }

  return (
    <>
      <Modal
        open={open && !!reportId}
        onClose={handleClose}
      >
        <ModalHeader title="Detail Titipan Paket" onClose={handleClose} />
        <ModalBody>
          {loading && (
            <div style={{ textAlign: 'center', padding: 40, color: 'var(--sf-text-muted)' }}>
              Memuat detail paket...
            </div>
          )}

          {error && !loading && (
            <div style={{ textAlign: 'center', padding: 24 }}>
              <div style={{ color: 'var(--sf-danger)', marginBottom: 12 }}>
                {error || 'Data paket tidak ditemukan'}
              </div>
              <button className="sf-btn-outline" onClick={handleClose}>Tutup</button>
            </div>
          )}

          {report && !loading && (
            <>
              {/* Summary Card */}
              <div
                style={{
                  background: 'var(--sf-card-bg)',
                  border: '1px solid var(--sf-border)',
                  borderRadius: 12,
                  padding: 16,
                  marginBottom: 16,
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12 }}>
                  <div>
                    <div style={{ fontWeight: 700, fontSize: 16 }}>{report.receiver_name || 'Warga Penerima'}</div>
                    {report.receiver_house_number && (
                      <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginTop: 2 }}>
                        Rumah: {report.receiver_house_number}
                      </div>
                    )}
                  </div>
                  {overallBadge(report.overall_status)}
                </div>
                <div style={{ borderTop: '1px solid var(--sf-border)', paddingTop: 12, fontSize: 13, color: 'var(--sf-text)' }}>
                  <div style={{ marginBottom: 6 }}>🕐 Waktu Diterima: {formatJakartaDateTime(report.created_at)}</div>
                  <div style={{ marginBottom: 6 }}>🛡️ Petugas Penerima: {report.created_by_name || 'Petugas Keamanan'}</div>
                  <div style={{ marginBottom: 6 }}>
                    📦 Total Titipan: {report.total_items || (report.items?.length ?? 0)} Barang
                  </div>
                  {report.items && report.items.length > 0 && (
                    <div style={{ marginBottom: 6 }}>
                      📋 Status Penyerahan: {report.handed_over_items_count ?? 0} diserahkan, {report.pending_items_count ?? 0} belum diserahkan
                    </div>
                  )}
                  {(() => {
                    const handed = report.handed_over_items_count ?? 0

                    // Derive from item-level handed_over_by_name by comparing all handed items.
                    if (handed > 0 && report.items) {
                      const handedItems = report.items.filter(
                        (it) => it.status === 'sudah_diambil' || it.handed_over_at != null
                      )
                      const handedNames = new Set(
                        handedItems
                          .map((it) => it.handed_over_by_name ?? null)
                          .filter((name): name is string => name != null && name.length > 0)
                      )

                      if (handedNames.size === 1) {
                        return (
                          <div style={{ marginBottom: 6 }}>
                            👤 Petugas Penyerah: {Array.from(handedNames)[0]}
                          </div>
                        )
                      }
                      if (handedNames.size > 1) {
                        return (
                          <div style={{ marginBottom: 6 }}>
                            👤 Petugas Penyerah: Beberapa Petugas
                          </div>
                        )
                      }
                      // No names available from items — fall back to report-level
                      if (report.handed_over_by_name) {
                        return (
                          <div style={{ marginBottom: 6 }}>
                            👤 Petugas Penyerah: {report.handed_over_by_name}
                          </div>
                        )
                      }
                      return (
                        <div style={{ marginBottom: 6 }}>
                          👤 Petugas Penyerah: Data serah terima belum tersedia
                        </div>
                      )
                    }
                    if (handed > 0) {
                      return (
                        <div style={{ marginBottom: 6 }}>
                          👤 Petugas Penyerah: {report.handed_over_by_name || 'Data serah terima belum tersedia'}
                        </div>
                      )
                    }
                    return <div style={{ marginBottom: 6 }}>👤 Petugas Penyerah: Belum diserahkan</div>
                  })()}
                  {report.overall_status === 'sudah_diambil' && report.handed_over_at && (
                    <div style={{ marginBottom: 6 }}>✅ Waktu Diserahkan: {formatJakartaDateTime(report.handed_over_at)}</div>
                  )}
                </div>
              </div>

              {/* Items List */}
              <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 10 }}>
                Daftar Barang Titipan ({report.items?.length ?? 0})
              </h3>
              {report.items?.map((item, idx) => (
                <div
                  key={item.id}
                  style={{
                    background: 'var(--sf-card-bg)',
                    border: '1px solid var(--sf-border)',
                    borderRadius: 10,
                    padding: 12,
                    marginBottom: 10,
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div>
                      <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--sf-primary)' }}>
                        Barang #{idx + 1}
                      </div>
                      {(() => {
                        const raw = item.description?.trim()
                        if (!raw) {
                          return <div style={{ fontWeight: 700, fontSize: 14, marginTop: 2 }}>Paket / Titipan</div>
                        }
                        const parts = raw.split(' · ').map((s) => s.trim()).filter((s) => s.length > 0)
                        if (parts.length <= 1) {
                          return <div style={{ fontWeight: 700, fontSize: 14, marginTop: 2 }}>{raw}</div>
                        }
                        return (
                          <>
                            {parts.map((part, i) => (
                              <div
                                key={i}
                                style={{
                                  fontWeight: i === 0 ? 700 : 400,
                                  fontSize: i === 0 ? 14 : 13,
                                  marginTop: i === 0 ? 2 : 2,
                                  color: i === 0 ? 'inherit' : 'var(--sf-text-muted)',
                                }}
                              >
                                {part}
                              </div>
                            ))}
                          </>
                        )
                      })()}
                    </div>
                    <span
                      style={{
                        padding: '2px 8px',
                        borderRadius: 4,
                        fontSize: 11,
                        fontWeight: 700,
                        background: item.status === 'sudah_diambil'
                          ? 'rgba(16,185,129,0.12)'
                          : 'rgba(245,158,11,0.12)',
                        color: item.status === 'sudah_diambil' ? '#10b981' : '#f59e0b',
                      }}
                    >
                      {item.status === 'sudah_diambil' ? 'Sudah Diambil' : item.status === 'dibatalkan' ? 'Dibatalkan' : 'Dititipkan'}
                    </span>
                  </div>

                  {item.status === 'sudah_diambil' && (
                    <div
                      style={{
                        marginTop: 8,
                        padding: '6px 8px',
                        borderRadius: 6,
                        background: 'rgba(16,185,129,0.08)',
                        fontSize: 12,
                        color: '#10b981',
                      }}
                    >
                      {item.handed_over_by_name
                        ? <>
                            <div>Diserahkan oleh: {item.handed_over_by_name}</div>
                            {item.handed_over_at && <div style={{ marginTop: 2 }}>{formatJakartaDateTime(item.handed_over_at)}</div>}
                          </>
                        : item.status_changed_at
                          ? <>
                              <div>Diambil: {formatJakartaDateTime(item.status_changed_at)}</div>
                              {item.changed_by_name && <div style={{ marginTop: 2 }}>Petugas: {item.changed_by_name}</div>}
                            </>
                          : 'Data serah terima belum tersedia'
                      }
                    </div>
                  )}

                  {/* Photos — use SecurityImage to avoid 401 on authenticated endpoints */}
                  {item.photos && item.photos.length > 0 && (
                    <div style={{ marginTop: 10 }}>
                      <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 6 }}>
                        Foto ({item.photos.length}):
                      </div>
                      <div style={{ display: 'flex', gap: 8, overflowX: 'auto' }}>
                        {item.photos?.map((photo, photoIdx) => (
                          <SecurityImage
                            key={photo.id}
                            photoId={photo.id}
                            domain="paket"
                            alt={photo.photo_type === 'bukti_pengambilan' ? 'Bukti' : 'Paket'}
                            style={{
                              width: 72,
                              height: 72,
                              minWidth: 72,
                            }}
                            onClick={() => handleViewPhoto(
                              (item.photos ?? []).map((p) => ({ id: p.id, title: p.photo_type ?? 'Foto' })),
                              photoIdx,
                            )}
                          />
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </>
          )}
        </ModalBody>
      </Modal>

      {/* Photo Viewer Lightbox */}
      <SecurityPhotoLightbox
        photos={lightboxPhotos}
        initialIndex={lightboxIndex}
        domain="paket"
        open={showLightbox}
        onClose={() => setShowLightbox(false)}
      />
    </>
  )
}
