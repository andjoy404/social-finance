import { useState } from 'react'
import {
  CommunityAnnouncement,
  BannerTemplate,
  apiCreateAnnouncement,
  apiUpdateAnnouncement,
  apiUploadCommunityImage,
  getSessionPair,
} from '@/app/api'
import { LoadingOutlined, DeleteOutlined } from '@ant-design/icons'
import { CommunityImage } from '@/components/CommunityImage'

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
  const [selectedTemplateId, setSelectedTemplateId] = useState(
    announcement?.template_id || initialTemplate?.id || ''
  )
  const [title, setTitle] = useState(
    announcement?.title || initialTemplate?.title_template || ''
  )
  const [body, setBody] = useState(
    announcement?.body || initialTemplate?.body_template || ''
  )
  const [imageUrl, setImageUrl] = useState(announcement?.image_url || initialTemplate?.image_url || '')
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
  const [uploading, setUploading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleTemplateChange = (tmplId: string) => {
    setSelectedTemplateId(tmplId)
    const tmpl = templates.find((t) => t.id === tmplId)
    if (tmpl) {
      if (!title || title === initialTemplate?.title_template) setTitle(tmpl.title_template)
      if (!body || body === initialTemplate?.body_template) setBody(tmpl.body_template)
      if (tmpl.image_url && !imageUrl) setImageUrl(tmpl.image_url)
      setPriority(tmpl.default_priority)
    }
  }

  const handleImageUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (file.size > 5 * 1024 * 1024) {
      setError('Ukuran file maksimal 5 MB')
      return
    }
    const session = getSessionPair()
    if (!session?.accessToken) return

    setUploading(true)
    setError(null)
    try {
      const res = await apiUploadCommunityImage(session.accessToken, file, 'announcement')
      setImageUrl(res.image_url)
    } catch (err: any) {
      setError(err?.message || 'Gagal mengunggah gambar')
    } finally {
      setUploading(false)
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
      const payload: any = {
        title: title.trim(),
        body: body.trim(),
        image_url: imageUrl || null,
        deep_link: deepLink.trim() || null,
        template_id: selectedTemplateId || null,
        scheduled_start: scheduledStart ? new Date(scheduledStart).toISOString() : null,
        scheduled_end: scheduledEnd ? new Date(scheduledEnd).toISOString() : null,
        priority: Number(priority),
        display_order: Number(displayOrder),
      }

      if (isEditing) {
        await apiUpdateAnnouncement(session.accessToken, announcement.id, payload)
      } else {
        payload.publish_now = publishNow
        await apiCreateAnnouncement(session.accessToken, payload)
      }

      onSuccess()
    } catch (err: any) {
      setError(err?.message || 'Gagal menyimpan pengumuman')
    } finally {
      setSaving(false)
    }
  }

  return (
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
            onClick={onClose}
            style={{ background: 'none', border: 'none', fontSize: 20, cursor: 'pointer', color: 'var(--sf-text-muted)' }}
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

          {/* Banner Image Upload */}
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
              Gambar Banner (Opsional)
            </label>
            <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
              <input
                type="file"
                className="sf-form-file"
                accept="image/png,image/jpeg,image/webp"
                onChange={handleImageUpload}
                disabled={uploading}
                style={{ fontSize: 13 }}
              />
              {uploading && <LoadingOutlined style={{ color: 'var(--sf-accent)' }} />}
            </div>
            {imageUrl && (
              <div style={{ marginTop: 8, position: 'relative', display: 'inline-block' }}>
                <CommunityImage
                  src={imageUrl}
                  alt="Banner preview"
                  style={{ maxHeight: 100, borderRadius: 6, objectFit: 'cover', display: 'block' }}
                />
                <button
                  type="button"
                  onClick={() => setImageUrl('')}
                  aria-label="Hapus gambar"
                  title="Hapus gambar"
                  style={{
                    position: 'absolute',
                    top: 6,
                    right: 6,
                    width: 28,
                    height: 28,
                    borderRadius: 6,
                    border: '1px solid var(--sf-border)',
                    background: 'var(--sf-surface)',
                    color: 'var(--sf-danger)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    boxShadow: '0 1px 2px rgba(0,0,0,0.08)',
                  }}
                >
                  <DeleteOutlined />
                </button>
              </div>
            )}
          </div>

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
              onClick={onClose}
              disabled={saving}
            >
              Batal
            </button>
            <button
              type="submit"
              className="sf-btn sf-btn-primary"
              disabled={saving || uploading}
            >
              {saving ? 'Menyimpan...' : isEditing ? 'Simpan Perubahan' : 'Tambah Pengumuman'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
