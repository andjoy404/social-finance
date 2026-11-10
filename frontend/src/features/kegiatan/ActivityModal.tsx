import { useState } from 'react'
import { CommunityActivity, apiCreateActivity, apiUpdateActivity, apiUploadCommunityImage, getSessionPair } from '@/app/api'
import { LoadingOutlined } from '@ant-design/icons'

interface ActivityModalProps {
  activity?: CommunityActivity | null
  onClose: () => void
  onSuccess: () => void
}

export function ActivityModal({ activity, onClose, onSuccess }: ActivityModalProps) {
  const isEditing = !!activity
  const [title, setTitle] = useState(activity?.title || '')
  const [description, setDescription] = useState(activity?.description || '')
  const [startsAt, setStartsAt] = useState(
    activity?.starts_at ? new Date(activity.starts_at).toISOString().slice(0, 16) : ''
  )
  const [endsAt, setEndsAt] = useState(
    activity?.ends_at ? new Date(activity.ends_at).toISOString().slice(0, 16) : ''
  )
  const [location, setLocation] = useState(activity?.location || '')
  const [organizer, setOrganizer] = useState(activity?.organizer || '')
  const [posterUrl, setPosterUrl] = useState(activity?.poster_url || '')
  const [showOnBanner, setShowOnBanner] = useState(activity?.show_on_banner ?? true)
  const [publishNow, setPublishNow] = useState(activity?.publication_status === 'published')
  const [uploading, setUploading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleImageUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const session = getSessionPair()
    if (!session?.accessToken) return

    setUploading(true)
    setError(null)
    try {
      const res = await apiUploadCommunityImage(session.accessToken, file, 'activity')
      setPosterUrl(res.image_url)
    } catch (err: any) {
      setError(err?.message || 'Gagal mengunggah poster')
    } finally {
      setUploading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !startsAt) {
      setError('Judul dan waktu mulai wajib diisi')
      return
    }

    const session = getSessionPair()
    if (!session?.accessToken) return

    setSaving(true)
    setError(null)

    try {
      const payload: any = {
        title: title.trim(),
        description: description.trim() || null,
        poster_url: posterUrl || null,
        starts_at: new Date(startsAt).toISOString(),
        ends_at: endsAt ? new Date(endsAt).toISOString() : null,
        location: location.trim() || null,
        organizer: organizer.trim() || null,
        show_on_banner: showOnBanner,
      }

      if (isEditing) {
        await apiUpdateActivity(session.accessToken, activity.id, payload)
      } else {
        payload.publish_now = publishNow
        await apiCreateActivity(session.accessToken, payload)
      }

      onSuccess()
    } catch (err: any) {
      setError(err?.message || 'Gagal menyimpan kegiatan')
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
          maxWidth: 540,
          maxHeight: '90vh',
          overflowY: 'auto',
          padding: 24,
          borderRadius: 12,
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700 }}>
            {isEditing ? 'Ubah Kegiatan' : 'Tambah Kegiatan Baru'}
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
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
              Judul Kegiatan <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Contoh: Kerja Bakti Lingkungan"
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
              Deskripsi
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              placeholder="Jelaskan detail kegiatan..."
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

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                Waktu Mulai <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                type="datetime-local"
                value={startsAt}
                onChange={(e) => setStartsAt(e.target.value)}
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
                Waktu Selesai (Opsional)
              </label>
              <input
                type="datetime-local"
                value={endsAt}
                onChange={(e) => setEndsAt(e.target.value)}
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

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                Lokasi
              </label>
              <input
                type="text"
                value={location}
                onChange={(e) => setLocation(e.target.value)}
                placeholder="Contoh: Balai Warga"
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
                Penyelenggara
              </label>
              <input
                type="text"
                value={organizer}
                onChange={(e) => setOrganizer(e.target.value)}
                placeholder="Contoh: Pengurus RT"
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

          {/* Poster Image Upload */}
          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
              Poster Kegiatan (Gambar)
            </label>
            <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                onChange={handleImageUpload}
                disabled={uploading}
                style={{ fontSize: 13 }}
              />
              {uploading && <LoadingOutlined style={{ color: 'var(--sf-accent)' }} />}
            </div>
            {posterUrl && (
              <div style={{ marginTop: 8 }}>
                <img
                  src={posterUrl}
                  alt="Poster preview"
                  style={{ maxHeight: 120, borderRadius: 6, objectFit: 'cover' }}
                />
              </div>
            )}
          </div>

          {/* Options: show on banner & publish now */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, paddingTop: 4 }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={showOnBanner}
                onChange={(e) => setShowOnBanner(e.target.checked)}
              />
              <span>Tampilkan sebagai banner di Beranda</span>
            </label>

            {!isEditing && (
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={publishNow}
                  onChange={(e) => setPublishNow(e.target.checked)}
                />
                <span>Publikasikan langsung (dapat dilihat oleh semua warga)</span>
              </label>
            )}
          </div>

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
              {saving ? 'Menyimpan...' : isEditing ? 'Simpan Perubahan' : 'Tambah Kegiatan'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
