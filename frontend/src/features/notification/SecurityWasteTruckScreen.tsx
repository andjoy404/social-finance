import { useEffect, useRef, useState, useCallback } from 'react'
import { Alert, Image, Input, Spin, Tabs, message } from 'antd'
import {
  CameraOutlined,
  ReloadOutlined,
  SendOutlined,
  HistoryOutlined,
  EyeOutlined,
} from '@ant-design/icons'
import { getSessionPair } from '@/app/api'
import { Badge } from '@/components/Badge'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { renderNotificationBody } from './notificationUrl'
import { formatJakartaDateTime } from '@/utils/jakartaTime'
import './SecurityWasteTruckScreen.css'
import {
  apiGetWasteTruckTemplate,
  apiGetWasteTruckHistory,
  apiTriggerWasteTruckArrival,
  apiUploadWasteTruckImage,
  type ApiWasteTruckDispatch,
  type ApiWasteTruckTemplate,
} from '@/app/api'

const { TextArea } = Input

function getDispatchStatusDisplay(status: string) {
  const normalized = status?.trim().toUpperCase() || 'UNKNOWN'
  switch (normalized) {
    case 'SENT':
      return { label: 'TERKIRIM', variant: 'green' as const }
    case 'PARTIAL':
      return { label: 'SEBAGIAN GAGAL', variant: 'amber' as const }
    case 'FAILED':
      return { label: 'GAGAL', variant: 'red' as const }
    case 'NO_DEVICE':
      return { label: 'PERANGKAT TIDAK TERSEDIA', variant: 'amber' as const }
    default:
      return { label: 'DIPROSES', variant: 'amber' as const }
  }
}

export function SecurityWasteTruckScreen({ canDispatch = true }: { canDispatch?: boolean }) {
  const [activeTab, setActiveTab] = useState('template')
  const [template, setTemplate] = useState<ApiWasteTruckTemplate | null>(null)
  const [templateLoading, setTemplateLoading] = useState(true)
  const [templateError, setTemplateError] = useState<string | null>(null)
  const [photo, setPhoto] = useState<File | null>(null)
  const [photoPreview, setPhotoPreview] = useState<string | null>(null)
  const [messageText, setMessageText] = useState('')
  const [sending, setSending] = useState(false)

  // Dispatch history state for Security Officer
  const [entries, setEntries] = useState<ApiWasteTruckDispatch[]>([])
  const [entriesTotal, setEntriesTotal] = useState(0)
  const [entriesPage, setEntriesPage] = useState(1)
  const [entriesPageSize] = useState(15)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyError, setHistoryError] = useState<string | null>(null)
  const [previewEntry, setPreviewEntry] = useState<ApiWasteTruckDispatch | null>(null)

  const fileInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    let active = true
    const token = getSessionPair()?.accessToken
    if (!token) {
      setTemplateError('Sesi autentikasi tidak tersedia.')
      setTemplateLoading(false)
      return
    }
    apiGetWasteTruckTemplate(token)
      .then((data) => {
        if (active) setTemplate(data)
      })
      .catch((error: unknown) => {
        if (active) setTemplateError(error instanceof Error ? error.message : 'Gagal memuat template.')
      })
      .finally(() => {
        if (active) setTemplateLoading(false)
      })

    return () => { active = false }
  }, [])

  useEffect(() => {
    if (!photo) {
      setPhotoPreview(null)
      return
    }
    const url = URL.createObjectURL(photo)
    setPhotoPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [photo])

  const loadHistory = useCallback(async (targetPage = 1) => {
    const token = getSessionPair()?.accessToken
    if (!token) {
      setHistoryError('Sesi autentikasi tidak tersedia.')
      return
    }
    setHistoryLoading(true)
    setHistoryError(null)
    try {
      const response = await apiGetWasteTruckHistory(token, targetPage, entriesPageSize)
      setEntries(response.data)
      setEntriesPage(response.pagination.page)
      setEntriesTotal(response.pagination.total)
    } catch (error) {
      setHistoryError(error instanceof Error ? error.message : 'Gagal memuat riwayat pengiriman.')
    } finally {
      setHistoryLoading(false)
    }
  }, [entriesPageSize])

  useEffect(() => {
    if (activeTab === 'history') {
      void loadHistory(entriesPage)
    }
  }, [activeTab, loadHistory, entriesPage])

  const handleTabChange = (key: string) => {
    setActiveTab(key)
    if (key === 'history') {
      void loadHistory(1)
    }
  }

  const handlePhotoChange = (file?: File) => {
    if (!file) return
    if (!file.type.startsWith('image/')) {
      message.error('Pilih file foto dengan format gambar.')
      return
    }
    if (file.size > 5 * 1024 * 1024) {
      message.error('Ukuran foto maksimal 5 MB')
      return
    }
    setPhoto(file)
  }

  const handleSend = async () => {
    if (sending) return
    if (!photo) {
      message.error('Ambil atau pilih foto truk sampah terlebih dahulu.')
      return
    }
    const token = getSessionPair()?.accessToken
    if (!token) {
      message.error('Sesi autentikasi tidak tersedia.')
      return
    }

    setSending(true)
    try {
      const upload = await apiUploadWasteTruckImage(token, 'waste_truck_arrival', photo)
      const response = await apiTriggerWasteTruckArrival(token, messageText.trim(), upload.image_url)
      message.success(response.message || 'Permintaan diterima. Periksa status pengiriman pada Riwayat.')
      setPhoto(null)
      setMessageText('')
      await loadHistory(1)
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Gagal mengirim notifikasi truk sampah.')
    } finally {
      setSending(false)
    }
  }

  const templateTab = (
    <div className="security-waste-truck-panel">
      {templateLoading ? (
        <div style={{ textAlign: 'center', padding: '40px 0' }}><Spin /></div>
      ) : templateError ? (
        <Alert type="error" showIcon message={templateError} />
      ) : (
        <>
          <div className="sf-card" style={{ padding: 20, marginBottom: 16 }}>
            <div style={{ fontWeight: 700, marginBottom: 8 }}>{template?.name || 'Kedatangan Truk Sampah'}</div>
            <div style={{ fontSize: 18, fontWeight: 600, marginBottom: 8 }}>{template?.title || 'Truk Sampah Tiba'}</div>
            <div style={{ color: 'var(--sf-text-muted)', whiteSpace: 'pre-wrap' }}>{template?.body || 'Petugas kebersihan telah tiba.'}</div>
            <div style={{ marginTop: 12, fontSize: 12, color: 'var(--sf-text-muted)' }}>
              Event: waste_truck_arrival · Template hanya-baca
            </div>
          </div>
          {canDispatch ? (
            <div className="sf-card" style={{ padding: 20 }}>
              <h3 style={{ marginTop: 0 }}>Foto aktual truk sampah</h3>
              <input
                ref={fileInput}
                type="file"
                accept="image/*"
                capture="environment"
                hidden
                onChange={(event) => handlePhotoChange(event.target.files?.[0])}
              />
              <button
                type="button"
                className="sf-btn-outline"
                onClick={() => fileInput.current?.click()}
                disabled={sending}
              >
                <CameraOutlined style={{ marginRight: 4 }} /> Ambil / pilih foto
              </button>
              {photoPreview && (
                <div style={{ marginTop: 16 }}>
                  <Image src={photoPreview} alt="Pratinjau foto truk sampah" style={{ maxWidth: '100%', maxHeight: 320, objectFit: 'contain' }} />
                </div>
              )}

              <h3 style={{ marginTop: 20 }}>Pesan tambahan (opsional)</h3>
              <TextArea
                rows={3}
                placeholder="Tambahkan pesan kustom jika diperlukan..."
                value={messageText}
                onChange={(e) => setMessageText(e.target.value)}
                maxLength={500}
                showCount
                disabled={sending}
              />

              <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end' }}>
                <button
                  type="button"
                  className="sf-btn-primary"
                  onClick={handleSend}
                  disabled={sending || !photo}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}
                >
                  <SendOutlined /> {sending ? 'Mengirim...' : 'Kirim Notifikasi'}
                </button>
              </div>
            </div>
          ) : (
            <Alert
              type="warning"
              showIcon
              message="Anda tidak memiliki hak akses (notification.send_waste_truck) untuk memicu notifikasi ini."
            />
          )}
        </>
      )}
    </div>
  )

  const historyTab = (
    <div className="security-waste-truck-panel">
      {/* Header bar */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 12 }}>
        <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--sf-text)' }}>
          Riwayat Pengiriman Truk Sampah
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <button
            type="button"
            className="sf-btn-outline"
            onClick={() => void loadHistory(entriesPage)}
            disabled={historyLoading}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 13 }}
          >
            <ReloadOutlined /> Segarkan
          </button>
          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
            Total riwayat: {entriesTotal}
          </div>
        </div>
      </div>

      {historyError && <Alert type="error" showIcon message={historyError} style={{ marginBottom: 12 }} />}

      {historyLoading && entries.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '60px 20px', background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8 }}>
          <Spin />
        </div>
      ) : entries.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '60px 20px', background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8 }}>
          <HistoryOutlined style={{ fontSize: 36, color: 'var(--sf-text-muted)', marginBottom: 12 }} />
          <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-text)', marginBottom: 4 }}>
            Belum Ada Riwayat Pengiriman
          </div>
          <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
            Riwayat akan tercatat secara otomatis setiap kali Anda mengirimkan notifikasi truk sampah.
          </div>
        </div>
      ) : (
        <div style={{ overflowX: 'auto', background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--sf-border)', background: 'rgba(0,0,0,0.02)' }}>
                <th style={{ padding: '12px 16px', fontWeight: 600 }}>Waktu Kirim</th>
                <th style={{ padding: '12px 16px', fontWeight: 600 }}>Jenis Notifikasi</th>
                <th style={{ padding: '12px 16px', fontWeight: 600 }}>Judul & Pesan</th>
                <th style={{ padding: '12px 16px', fontWeight: 600 }}>Status Pengiriman</th>
                <th style={{ padding: '12px 16px', fontWeight: 600 }}>Lampiran Foto</th>
                <th style={{ padding: '12px 16px', fontWeight: 600 }}>Aksi</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => {
                const statusDisplay = getDispatchStatusDisplay(entry.status)
                return (
                  <tr
                    key={entry.id}
                    onClick={() => setPreviewEntry(entry)}
                    style={{
                      borderBottom: '1px solid var(--sf-border)',
                      cursor: 'pointer',
                    }}
                  >
                    <td style={{ padding: '12px 16px', whiteSpace: 'nowrap', color: 'var(--sf-text-muted)', fontSize: 12 }}>
                      {formatJakartaDateTime(entry.created_at)}
                    </td>
                    <td style={{ padding: '12px 16px', whiteSpace: 'nowrap' }}>
                      <Badge variant="gray">Kedatangan Truk Sampah</Badge>
                    </td>
                    <td style={{ padding: '12px 16px' }}>
                      <div style={{ fontWeight: 600, color: 'var(--sf-text)', marginBottom: 2 }}>
                        {entry.title || 'Kedatangan Truk Sampah'}
                      </div>
                      <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', maxWidth: 460 }}>
                        {renderNotificationBody(entry.body, entry.event_type, '')}
                      </div>
                    </td>
                    <td style={{ padding: '12px 16px', whiteSpace: 'nowrap' }}>
                      <Badge variant={statusDisplay.variant}>{statusDisplay.label}</Badge>
                    </td>
                    <td style={{ padding: '12px 16px', whiteSpace: 'nowrap' }}>
                      {entry.image_url ? (
                        <div onClick={(e) => e.stopPropagation()}>
                          <Image
                            src={entry.image_url}
                            alt="Foto Truk Sampah"
                            width={44}
                            height={44}
                            style={{ objectFit: 'cover', borderRadius: 4, border: '1px solid var(--sf-border)' }}
                          />
                        </div>
                      ) : (
                        <span style={{ color: 'var(--sf-text-muted)', fontSize: 12 }}>-</span>
                      )}
                    </td>
                    <td style={{ padding: '12px 16px', whiteSpace: 'nowrap' }}>
                      <button
                        type="button"
                        className="sf-btn-outline"
                        style={{ padding: '4px 10px', fontSize: 12 }}
                        onClick={(e) => {
                          e.stopPropagation()
                          setPreviewEntry(entry)
                        }}
                      >
                        <EyeOutlined style={{ marginRight: 4 }} /> Detail
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
      {Math.ceil(entriesTotal / entriesPageSize) > 1 && (
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 16, gap: 8 }}>
          <button
            type="button"
            className="sf-btn-outline"
            disabled={entriesPage <= 1}
            onClick={() => setEntriesPage((p) => Math.max(1, p - 1))}
            style={{ fontSize: 13 }}
          >
            Sebelumnya
          </button>
          <span style={{ fontSize: 13, display: 'flex', alignItems: 'center' }}>
            Halaman {entriesPage} dari {Math.ceil(entriesTotal / entriesPageSize)}
          </span>
          <button
            type="button"
            className="sf-btn-outline"
            disabled={entriesPage >= Math.ceil(entriesTotal / entriesPageSize)}
            onClick={() => setEntriesPage((p) => p + 1)}
            style={{ fontSize: 13 }}
          >
            Berikutnya
          </button>
        </div>
      )}
    </div>
  )

  return (
    <div className="security-waste-truck-page">
      <h1 style={{ marginTop: 0 }}>Notifikasi</h1>
      <Tabs activeKey={activeTab} onChange={handleTabChange} items={[
        { key: 'template', label: 'Template', children: templateTab },
        { key: 'history', label: 'Riwayat', children: historyTab },
      ]} />

      {/* Preview modal for Waste Truck Dispatch */}
      {previewEntry && (
        <Modal
          open={previewEntry !== null}
          onClose={() => setPreviewEntry(null)}
        >
          <ModalHeader
            title="Detail Pengiriman Notifikasi"
            onClose={() => setPreviewEntry(null)}
          />
          <ModalBody>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              <div>
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 2 }}>Jenis Notifikasi</div>
                <div style={{ fontSize: 13, fontWeight: 600 }}>Kedatangan Truk Sampah</div>
              </div>
              <div>
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 2 }}>Waktu Pengiriman</div>
                <div style={{ fontSize: 13 }}>
                  {formatJakartaDateTime(previewEntry.created_at, {
                    dateStyle: 'full',
                    timeStyle: 'medium',
                  })}
                </div>
              </div>
              <div>
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 2 }}>Status Pengiriman</div>
                <div>
                  <Badge variant={getDispatchStatusDisplay(previewEntry.status).variant}>
                    {getDispatchStatusDisplay(previewEntry.status).label}
                  </Badge>
                </div>
              </div>
              {previewEntry.failure_reason && (
                <div>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 2 }}>Informasi Status / Kendala</div>
                  <div style={{ fontSize: 13, color: 'var(--sf-danger, #ef4444)', background: 'rgba(239, 68, 68, 0.08)', padding: 10, borderRadius: 6 }}>
                    {previewEntry.failure_reason}
                  </div>
                </div>
              )}
              <div>
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 2 }}>Judul</div>
                <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--sf-text)' }}>
                  {previewEntry.title || 'Kedatangan Truk Sampah'}
                </div>
              </div>
              <div>
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 2 }}>Isi Pesan</div>
                <div style={{ fontSize: 13, color: 'var(--sf-text)', background: 'var(--sf-bg)', padding: 12, borderRadius: 6, whiteSpace: 'pre-wrap', lineHeight: 1.5 }}>
                  {previewEntry.body}
                </div>
              </div>
              {previewEntry.image_url && (
                <div>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 4 }}>Lampiran Foto Truk Sampah</div>
                  <Image
                    src={previewEntry.image_url}
                    alt="Lampiran Notifikasi Truk Sampah"
                    style={{ maxWidth: '100%', maxHeight: 300, objectFit: 'contain', borderRadius: 6 }}
                  />
                </div>
              )}
            </div>
          </ModalBody>
          <ModalFooter>
            <button
              type="button"
              className="sf-btn-outline"
              onClick={() => setPreviewEntry(null)}
            >
              Tutup
            </button>
          </ModalFooter>
        </Modal>
      )}
    </div>
  )
}
