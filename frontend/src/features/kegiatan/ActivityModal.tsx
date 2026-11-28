import { useState, useRef } from 'react'
import {
  CommunityActivity,
  BannerStyleConfig,
  apiCreateActivity,
  apiUpdateActivity,
  getSessionPair,
} from '@/app/api'
import { BannerStyleEditor, computeNeonGradient } from '@/components/BannerStyleEditor'
import { CancelConfirmModal } from '@/components/CancelConfirmModal'

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
  const [styleConfig, setStyleConfig] = useState<BannerStyleConfig | null>(
    activity?.style_config || {
      version: 1,
      background_type: activity?.poster_url ? 'image' : 'color',
      background_color: '#00F0FF',
      gradient: computeNeonGradient('#00F0FF'),
      image: activity?.poster_url
        ? { url: activity.poster_url, crop: { x: 0, y: 0, width: 1, height: 1 }, zoom: 1 }
        : null,
      show_detail: true,
    }
  )
  const [showOnBanner, setShowOnBanner] = useState(activity?.show_on_banner ?? true)
  const [publishNow, setPublishNow] = useState(activity?.publication_status === 'published')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showCancelConfirm, setShowCancelConfirm] = useState(false)

  const initialSnapshot = useRef({
    title: activity?.title || '',
    description: activity?.description || '',
    startsAt: activity?.starts_at ? new Date(activity.starts_at).toISOString().slice(0, 16) : '',
    endsAt: activity?.ends_at ? new Date(activity.ends_at).toISOString().slice(0, 16) : '',
    location: activity?.location || '',
    organizer: activity?.organizer || '',
    showOnBanner: activity?.show_on_banner ?? true,
    publishNow: activity?.publication_status === 'published',
    styleConfigStr: JSON.stringify(activity?.style_config || null),
  }).current

  const isDirty =
    title !== initialSnapshot.title ||
    description !== initialSnapshot.description ||
    startsAt !== initialSnapshot.startsAt ||
    endsAt !== initialSnapshot.endsAt ||
    location !== initialSnapshot.location ||
    organizer !== initialSnapshot.organizer ||
    showOnBanner !== initialSnapshot.showOnBanner ||
    publishNow !== initialSnapshot.publishNow ||
    JSON.stringify(styleConfig || null) !== initialSnapshot.styleConfigStr

  const handleRequestClose = () => {
    if (isDirty) {
      setShowCancelConfirm(true)
    } else {
      onClose()
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
      const finalPoster =
        styleConfig?.background_type === 'image' && styleConfig.image?.url
          ? styleConfig.image.url
          : null

      const payload: any = {
        title: title.trim(),
        description: description.trim() || null,
        poster_url: finalPoster,
        starts_at: new Date(startsAt).toISOString(),
        ends_at: endsAt ? new Date(endsAt).toISOString() : null,
        location: location.trim() || null,
        organizer: organizer.trim() || null,
        show_on_banner: showOnBanner,
        style_config: styleConfig,
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
          maxWidth: 540,
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
            {isEditing ? 'Ubah Kegiatan' : 'Tambah Kegiatan Baru'}
          </h2>
          <button
            type="button"
            onClick={handleRequestClose}
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
                className="sf-form-input"
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
                className="sf-form-input"
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

          {/* Banner & Poster Style Editor */}
          <BannerStyleEditor
            value={styleConfig}
            onChange={(cfg) => {
              setStyleConfig(cfg)
            }}
            titlePreview={title}
            bodyPreview={description}
            category="activity"
          />

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
              {saving ? 'Menyimpan...' : isEditing ? 'Simpan Perubahan' : 'Tambah Kegiatan'}
            </button>
          </div>
        </form>
      </div>

      <CancelConfirmModal
        open={showCancelConfirm}
        onStay={() => setShowCancelConfirm(false)}
        onConfirmCancel={() => {
          setShowCancelConfirm(false)
          onClose()
        }}
      />
    </div>
  )
}
