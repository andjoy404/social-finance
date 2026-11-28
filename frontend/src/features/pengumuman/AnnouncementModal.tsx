import { useState, useRef, useEffect, useCallback } from 'react'
import {
  CommunityAnnouncement,
  BannerTemplate,
  BannerStyleConfig,
  apiCreateAnnouncement,
  apiUpdateAnnouncement,
  getSessionPair,
} from '@/app/api'
import { BannerStyleEditor, computeNeonGradient } from '@/components/BannerStyleEditor'
import { isSnapshotDirty, useUnsavedChangesGuard } from '@/hooks/useUnsavedChangesGuard'

interface AnnouncementModalProps {
  announcement?: CommunityAnnouncement | null
  initialTemplate?: BannerTemplate | null
  templates: BannerTemplate[]
  onClose: () => void
  onSuccess: () => void
}

export function AnnouncementModal({
  announcement,
  initialTemplate,
  templates,
  onClose,
  onSuccess,
}: AnnouncementModalProps) {
  const isEditing = !!announcement
  const initialStyleConfig = useRef<BannerStyleConfig>(
    announcement?.style_config || {
      version: 1,
      background_type: announcement?.image_url ? 'image' : 'color',
      background_color: '#A970FF',
      gradient: computeNeonGradient('#A970FF'),
      image: announcement?.image_url
        ? { url: announcement.image_url, crop: { x: 0, y: 0, width: 1, height: 1 }, zoom: 1 }
        : null,
      show_detail: true,
    }
  ).current

  const [selectedTemplateId, setSelectedTemplateId] = useState(
    announcement?.template_id || initialTemplate?.id || ''
  )
  const [title, setTitle] = useState(
    announcement?.title || initialTemplate?.title_template || ''
  )
  const [body, setBody] = useState(
    announcement?.body || initialTemplate?.body_template || ''
  )
  const [styleConfig, setStyleConfig] = useState<BannerStyleConfig | null>(initialStyleConfig)
  const [deepLink, setDeepLink] = useState(announcement?.deep_link || '')
  const [priority, setPriority] = useState(announcement?.priority ?? initialTemplate?.default_priority ?? 10)
  const [displayOrder, setDisplayOrder] = useState(announcement?.display_order ?? 0)
  const [scheduledStart, setScheduledStart] = useState(
    announcement?.scheduled_start ? new Date(announcement.scheduled_start).toISOString().slice(0, 16) : ''
  )
  const [scheduledEnd, setScheduledEnd] = useState(
    announcement?.scheduled_end ? new Date(announcement.scheduled_end).toISOString().slice(0, 16) : ''
  )
  const [publishNow, setPublishNow] = useState(announcement?.status === 'active')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const initialSnapshot = useRef({
    selectedTemplateId: announcement?.template_id || initialTemplate?.id || '',
    title: announcement?.title || initialTemplate?.title_template || '',
    body: announcement?.body || initialTemplate?.body_template || '',
    deepLink: announcement?.deep_link || '',
    priority: announcement?.priority ?? initialTemplate?.default_priority ?? 10,
    displayOrder: announcement?.display_order ?? 0,
    scheduledStart: announcement?.scheduled_start ? new Date(announcement.scheduled_start).toISOString().slice(0, 16) : '',
    scheduledEnd: announcement?.scheduled_end ? new Date(announcement.scheduled_end).toISOString().slice(0, 16) : '',
    publishNow: announcement?.status === 'active',
    styleConfig: initialStyleConfig,
  }).current

  const isDirty = isSnapshotDirty(
    {
      selectedTemplateId,
      title,
      body,
      deepLink,
      priority,
      displayOrder,
      scheduledStart,
      scheduledEnd,
      publishNow,
      styleConfig,
    },
    initialSnapshot,
  )

  const { guardedClose, exitAfterSave, confirmModal } = useUnsavedChangesGuard({
    isDirty,
    isSaving: saving,
    active: true,
  })

  const handleRequestClose = useCallback(() => {
    void guardedClose(onClose)
  }, [guardedClose, onClose])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        handleRequestClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [handleRequestClose])

  const handleTemplateChange = (tmplId: string) => {
    setSelectedTemplateId(tmplId)
    const tmpl = templates.find((t) => t.id === tmplId)
    if (tmpl) {
      if (!title || title === initialTemplate?.title_template) setTitle(tmpl.title_template)
      if (!body || body === initialTemplate?.body_template) setBody(tmpl.body_template)
      if (tmpl.image_url) {
        setStyleConfig({
          version: 1,
          background_type: 'image',
          image: { url: tmpl.image_url, crop: { x: 0, y: 0, width: 1, height: 1 }, zoom: 1 },
          show_detail: styleConfig?.show_detail !== false,
        })
      }
      setPriority(tmpl.default_priority)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !body.trim()) {
      setError('Judul dan isi pengumuman wajib diisi')
      return
    }

    // Validate schedule window
    if (scheduledStart && scheduledEnd) {
      if (new Date(scheduledEnd).getTime() <= new Date(scheduledStart).getTime()) {
        setError('Jadwal berakhir harus lebih besar dari jadwal mulai')
        return
      }
    }

    // Validate deep link if provided
    if (deepLink.trim()) {
      const link = deepLink.trim()
      const allowed = ['/keuangan', '/iuran', '/kas', '/kegiatan', '/warga', '/notifikasi', '/pengumuman', '/profil']
      const isValid = link.startsWith('/') && allowed.some((p) => link === p || link.startsWith(p + '/'))
      if (!isValid) {
        setError('Tautan harus berupa path internal yang diizinkan (contoh: /keuangan, /kegiatan, dsb.)')
        return
      }
    }

    const session = getSessionPair()
    if (!session?.accessToken) return

    setSaving(true)
    setError(null)

    try {
      const finalImage =
        styleConfig?.background_type === 'image' && styleConfig.image?.url
          ? styleConfig.image.url
          : null

      const payload: any = {
        title: title.trim(),
        body: body.trim(),
        image_url: finalImage,
        deep_link: deepLink.trim() || null,
        template_id: selectedTemplateId || null,
        scheduled_start: scheduledStart ? new Date(scheduledStart).toISOString() : null,
        scheduled_end: scheduledEnd ? new Date(scheduledEnd).toISOString() : null,
        priority: Number(priority),
        display_order: Number(displayOrder),
        style_config: styleConfig,
      }

      if (isEditing) {
        await apiUpdateAnnouncement(session.accessToken, announcement.id, payload)
      } else {
        payload.publish_now = publishNow
        await apiCreateAnnouncement(session.accessToken, payload)
      }

      exitAfterSave(onSuccess)
    } catch (err: any) {
      setError(err?.message || 'Gagal menyimpan pengumuman')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      onClick={(e) => {
        if (e.target === e.currentTarget) handleRequestClose()
      }}
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
    >
      <div
        className="sf-card"
        style={{
          width: '100%',
          maxWidth: 580,
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
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700 }}>
            {isEditing ? 'Ubah Pengumuman' : 'Tambah Pengumuman Baru'}
          </h2>
          <button
            type="button"
            onClick={handleRequestClose}
            disabled={saving}
            aria-label="Tutup"
            style={{ background: 'none', border: 'none', fontSize: 20, cursor: saving ? 'not-allowed' : 'pointer', color: 'var(--sf-text-muted)' }}
          >
            ×
          </button>
        </div>

        {error && (
          <div
            style={{
              padding: '10px 14px',
              borderRadius: 8,
              background: 'rgba(235, 94, 40, 0.12)',
              border: '1px solid rgba(235, 94, 40, 0.3)',
              color: 'var(--sf-danger)',
              fontSize: 13,
              marginBottom: 16,
            }}
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Template Dropdown */}
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
              Pilih Template Banner (Opsional)
            </label>
            <select
              value={selectedTemplateId}
              onChange={(e) => handleTemplateChange(e.target.value)}
              style={{
                width: '100%',
                padding: '10px 12px',
                borderRadius: 8,
                border: '1px solid var(--sf-border)',
                background: 'var(--sf-bg)',
                color: 'var(--sf-text)',
                boxSizing: 'border-box',
              }}
            >
              <option value="">-- Kustom (Tanpa Template) --</option>
              {templates.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.category})
                </option>
              ))}
            </select>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
              Judul Pengumuman <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Contoh: Kerja Bakti Akbar Menyambut HUT RI"
              required
              style={{
                width: '100%',
                padding: '10px 12px',
                borderRadius: 8,
                border: '1px solid var(--sf-border)',
                background: 'var(--sf-bg)',
                color: 'var(--sf-text)',
                boxSizing: 'border-box',
              }}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
              Isi Pengumuman <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              rows={4}
              placeholder="Tuliskan pesan lengkap pengumuman untuk warga..."
              required
              style={{
                width: '100%',
                padding: '10px 12px',
                borderRadius: 8,
                border: '1px solid var(--sf-border)',
                background: 'var(--sf-bg)',
                color: 'var(--sf-text)',
                boxSizing: 'border-box',
                resize: 'vertical',
              }}
            />
          </div>

          {/* Deep link input with suggestions */}
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
              Tautan Internal (Deep Link)
            </label>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <input
                type="text"
                value={deepLink}
                onChange={(e) => setDeepLink(e.target.value)}
                placeholder="Contoh: /keuangan, /kegiatan"
                style={{
                  flex: 1,
                  padding: '10px 12px',
                  borderRadius: 8,
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  color: 'var(--sf-text)',
                  boxSizing: 'border-box',
                }}
              />
            </div>
            <div style={{ display: 'flex', gap: 6, marginTop: 6, flexWrap: 'wrap' }}>
              {['/keuangan', '/iuran', '/kegiatan', '/pengumuman', '/warga', '/notifikasi'].map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() => setDeepLink(p)}
                  style={{
                    padding: '3px 8px',
                    fontSize: 11,
                    borderRadius: 4,
                    background: 'var(--sf-surface-hover)',
                    border: '1px solid var(--sf-border)',
                    color: 'var(--sf-text-muted)',
                    cursor: 'pointer',
                  }}
                >
                  {p}
                </button>
              ))}
            </div>
          </div>

          {/* Priority & Order */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                Prioritas (1 - 100)
              </label>
              <input
                type="number"
                min={1}
                max={100}
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value))}
                style={{
                  width: '100%',
                  padding: '10px 12px',
                  borderRadius: 8,
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  color: 'var(--sf-text)',
                  boxSizing: 'border-box',
                }}
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                Urutan Tampilan
              </label>
              <input
                type="number"
                value={displayOrder}
                onChange={(e) => setDisplayOrder(Number(e.target.value))}
                style={{
                  width: '100%',
                  padding: '10px 12px',
                  borderRadius: 8,
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  color: 'var(--sf-text)',
                  boxSizing: 'border-box',
                }}
              />
            </div>
          </div>

          {/* Scheduling */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                Jadwal Mulai Tampil (Opsional)
              </label>
              <input
                type="datetime-local"
                className="sf-form-input"
                value={scheduledStart}
                onChange={(e) => setScheduledStart(e.target.value)}
                style={{
                  width: '100%',
                  padding: '10px 12px',
                  borderRadius: 8,
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  color: 'var(--sf-text)',
                  boxSizing: 'border-box',
                }}
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                Jadwal Berakhir Tampil (Opsional)
              </label>
              <input
                type="datetime-local"
                className="sf-form-input"
                value={scheduledEnd}
                onChange={(e) => setScheduledEnd(e.target.value)}
                style={{
                  width: '100%',
                  padding: '10px 12px',
                  borderRadius: 8,
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  color: 'var(--sf-text)',
                  boxSizing: 'border-box',
                }}
              />
            </div>
          </div>

          {/* Banner Style Editor (Color Neon Glow or 16:9 Image Crop) */}
          <BannerStyleEditor
            value={styleConfig}
            onChange={(cfg) => {
              setStyleConfig(cfg)
            }}
            titlePreview={title}
            bodyPreview={body}
            category="announcement"
          />

          {isEditing && announcement?.status === 'active' && (
            <div
              style={{
                fontSize: 12,
                padding: '8px 12px',
                borderRadius: 8,
                border: '1px solid var(--sf-border)',
                background: 'var(--sf-bg)',
                color: 'var(--sf-text-muted)',
              }}
            >
              Pengumuman ini sedang tampil di Beranda. Perubahan konten tidak otomatis mengubah banner —
              gunakan tombol <strong>Terbitkan Ulang</strong> pada daftar pengumuman untuk memperbarui snapshot banner.
            </div>
          )}

          {!isEditing && (
            <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={publishNow}
                onChange={(e) => setPublishNow(e.target.checked)}
              />
              <span>Publikasikan langsung (muncul di banner Beranda)</span>
            </label>
          )}

          {/* Actions */}
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 12 }}>
            <button
              type="button"
              className="sf-btn sf-btn-outline"
              onClick={handleRequestClose}
              disabled={saving}
            >
              Batal
            </button>
            <button
              type="submit"
              className="sf-btn sf-btn-primary"
              disabled={saving}
            >
              {saving ? 'Menyimpan...' : isEditing ? 'Simpan Perubahan' : 'Tambah Pengumuman'}
            </button>
          </div>
        </form>
      </div>

      {confirmModal}
    </div>
  )
}
