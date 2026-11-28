import React, { useState, useCallback, useEffect, useRef } from 'react'
import Cropper, { Area } from 'react-easy-crop'
import {
  BannerStyleConfig,
  apiUploadCommunityImage,
  apiSignCommunityImage,
  apiGetCommunityImageBlob,
  getSessionPair,
} from '@/app/api'
import {
  UploadOutlined,
  ReloadOutlined,
  LoadingOutlined,
  BgColorsOutlined,
  PictureOutlined,
  CheckOutlined,
} from '@ant-design/icons'

export const NEON_PRESETS = [
  { name: 'Neon Purple', hex: '#A970FF' },
  { name: 'Electric Cyan', hex: '#00F0FF' },
  { name: 'Neon Pink', hex: '#FF2A85' },
  { name: 'Neon Green', hex: '#00E599' },
  { name: 'Neon Amber', hex: '#FFB800' },
  { name: 'Sky Blue', hex: '#38BDF8' },
  { name: 'Indigo Glow', hex: '#6366F1' },
  { name: 'Coral Flare', hex: '#FF6B6B' },
]

export function computeNeonGradient(hexColor: string) {
  let hex = hexColor.trim().replace(/^#/, '').toUpperCase()
  if (hex.length === 3) {
    hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2]
  }
  const r = parseInt(hex.substring(0, 2), 16) || 0
  const g = parseInt(hex.substring(2, 4), 16) || 0
  const b = parseInt(hex.substring(4, 6), 16) || 0

  const toR = Math.max(12, Math.floor(r * 0.18))
  const toG = Math.max(8, Math.floor(g * 0.18))
  const toB = Math.max(20, Math.floor(b * 0.18))

  const toHex = `#${toR.toString(16).padStart(2, '0')}${toG.toString(16).padStart(2, '0')}${toB.toString(16).padStart(2, '0')}`.toUpperCase()

  return {
    from: `#${hex}`,
    to: toHex,
    angle: 135,
    glow: true,
  }
}

interface BannerStyleEditorProps {
  value?: BannerStyleConfig | null
  onChange: (val: BannerStyleConfig) => void
  titlePreview?: string
  bodyPreview?: string
  category?: 'announcement' | 'activity'
}

export function BannerStyleEditor({
  value,
  onChange,
  titlePreview = 'Judul Banner Pengumuman',
  bodyPreview = 'Deskripsi atau ringkasan informasi yang akan tampil pada banner Beranda.',
  category = 'announcement',
}: BannerStyleEditorProps) {
  const currentType = value?.background_type === 'image' ? 'image' : 'color'
  const currentColor = value?.background_color || '#A970FF'
  const currentImageUrl = value?.image?.url || ''
  const currentZoom = value?.image?.zoom ?? 1
  const currentShowDetail = value?.show_detail !== false
  const currentOpacity = value?.overlay_opacity ?? 0.35

  const [crop, setCrop] = useState({ x: 0, y: 0 })
  const [zoom, setZoom] = useState(currentZoom)
  const [croppedAreaNormalized, setCroppedAreaNormalized] = useState<Area | null>(
    value?.image?.crop
      ? {
          x: value.image.crop.x * 100,
          y: value.image.crop.y * 100,
          width: value.image.crop.width * 100,
          height: value.image.crop.height * 100,
        }
      : null
  )
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [resolvedCropperUrl, setResolvedCropperUrl] = useState<string>('')
  const localBlobUrlRef = useRef<string | null>(null)
  const resolvedBlobUrlRef = useRef<string | null>(null)

  useEffect(() => {
    return () => {
      if (localBlobUrlRef.current) {
        URL.revokeObjectURL(localBlobUrlRef.current)
        localBlobUrlRef.current = null
      }
      if (resolvedBlobUrlRef.current) {
        URL.revokeObjectURL(resolvedBlobUrlRef.current)
        resolvedBlobUrlRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    if (!currentImageUrl) {
      if (!localBlobUrlRef.current) {
        setResolvedCropperUrl('')
      }
      return
    }

    if (currentImageUrl.startsWith('blob:') || currentImageUrl.startsWith('data:')) {
      setResolvedCropperUrl(currentImageUrl)
      return
    }

    let cancelled = false
    const session = getSessionPair()
    if (!session?.accessToken) {
      setResolvedCropperUrl(currentImageUrl)
      return
    }

    apiSignCommunityImage(session.accessToken, currentImageUrl)
      .then((res) => {
        if (!cancelled) setResolvedCropperUrl(res.url)
      })
      .catch(async () => {
        if (cancelled) return
        try {
          const { blob } = await apiGetCommunityImageBlob(session.accessToken, currentImageUrl)
          if (cancelled) return
          if (resolvedBlobUrlRef.current) {
            URL.revokeObjectURL(resolvedBlobUrlRef.current)
          }
          const objUrl = URL.createObjectURL(blob)
          resolvedBlobUrlRef.current = objUrl
          setResolvedCropperUrl(objUrl)
        } catch {
          if (!cancelled) setResolvedCropperUrl(currentImageUrl)
        }
      })

    return () => {
      cancelled = true
    }
  }, [currentImageUrl])

  const handleTypeSelect = (type: 'color' | 'image') => {
    if (type === 'color') {
      const grad = computeNeonGradient(currentColor)
      onChange({
        version: 1,
        background_type: 'color',
        background_color: currentColor,
        gradient: grad,
        image: null,
        overlay_opacity: currentOpacity,
        show_detail: currentShowDetail,
      })
    } else {
      onChange({
        version: 1,
        background_type: 'image',
        background_color: undefined,
        gradient: {
          from: '#2A124D',
          to: '#130924',
          angle: 135,
          glow: true,
        },
        image: {
          url: currentImageUrl,
          crop: croppedAreaNormalized
            ? {
                x: croppedAreaNormalized.x / 100,
                y: croppedAreaNormalized.y / 100,
                width: croppedAreaNormalized.width / 100,
                height: croppedAreaNormalized.height / 100,
              }
            : { x: 0, y: 0, width: 1, height: 1 },
          focal_point: { x: 0.5, y: 0.5 },
          zoom,
        },
        overlay_opacity: currentOpacity,
        show_detail: currentShowDetail,
      })
    }
  }

  const handleColorChange = (hex: string) => {
    const grad = computeNeonGradient(hex)
    onChange({
      version: 1,
      background_type: 'color',
      background_color: hex.toUpperCase(),
      gradient: grad,
      image: null,
      overlay_opacity: currentOpacity,
      show_detail: currentShowDetail,
    })
  }

  const handleCropComplete = useCallback(
    (_: Area, croppedAreaPixelsNorm: Area) => {
      setCroppedAreaNormalized(croppedAreaPixelsNorm)
      if (currentType === 'image') {
        onChange({
          version: 1,
          background_type: 'image',
          gradient: {
            from: '#2A124D',
            to: '#130924',
            angle: 135,
            glow: true,
          },
          image: {
            url: currentImageUrl,
            crop: {
              x: croppedAreaPixelsNorm.x / 100,
              y: croppedAreaPixelsNorm.y / 100,
              width: croppedAreaPixelsNorm.width / 100,
              height: croppedAreaPixelsNorm.height / 100,
            },
            focal_point: {
              x: (croppedAreaPixelsNorm.x + croppedAreaPixelsNorm.width / 2) / 100,
              y: (croppedAreaPixelsNorm.y + croppedAreaPixelsNorm.height / 2) / 100,
            },
            zoom,
          },
          overlay_opacity: currentOpacity,
          show_detail: currentShowDetail,
        })
      }
    },
    [currentType, currentImageUrl, zoom, currentOpacity, currentShowDetail, onChange]
  )

  const handleZoomChange = (newZoom: number) => {
    setZoom(newZoom)
    if (currentType === 'image' && value?.image) {
      onChange({
        ...value,
        image: {
          ...value.image,
          zoom: newZoom,
        },
      })
    }
  }

  const handleResetCrop = () => {
    setCrop({ x: 0, y: 0 })
    setZoom(1)
    if (currentType === 'image' && value?.image) {
      onChange({
        ...value,
        image: {
          ...value.image,
          crop: { x: 0, y: 0, width: 1, height: 1 },
          focal_point: { x: 0.5, y: 0.5 },
          zoom: 1,
        },
      })
    }
  }

  const handleImageUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (file.size > 5 * 1024 * 1024) {
      setUploadError('Ukuran file maksimal 5 MB')
      return
    }

    // Instant local preview in cropper
    if (localBlobUrlRef.current) {
      URL.revokeObjectURL(localBlobUrlRef.current)
    }
    const localUrl = URL.createObjectURL(file)
    localBlobUrlRef.current = localUrl
    setResolvedCropperUrl(localUrl)

    const session = getSessionPair()
    if (!session?.accessToken) return

    setUploading(true)
    setUploadError(null)
    try {
      const res = await apiUploadCommunityImage(session.accessToken, file, category)
      setCrop({ x: 0, y: 0 })
      setZoom(1)
      onChange({
        version: 1,
        background_type: 'image',
        gradient: {
          from: '#2A124D',
          to: '#130924',
          angle: 135,
          glow: true,
        },
        image: {
          url: res.image_url,
          crop: { x: 0, y: 0, width: 1, height: 1 },
          focal_point: { x: 0.5, y: 0.5 },
          zoom: 1,
        },
        overlay_opacity: currentOpacity,
        show_detail: currentShowDetail,
      })
    } catch (err: any) {
      setUploadError(err?.message || 'Gagal mengunggah gambar')
    } finally {
      setUploading(false)
    }
  }

  const handleShowDetailToggle = (enabled: boolean) => {
    onChange({
      ...(value || {
        version: 1,
        background_type: 'color',
        background_color: currentColor,
        gradient: computeNeonGradient(currentColor),
      }),
      show_detail: enabled,
    })
  }

  // Calculate crop coordinates for preview
  const cropData = value?.image?.crop || { x: 0, y: 0, width: 1, height: 1 }
  const imgWidthPercent = (1 / (cropData.width || 1)) * 100
  const imgHeightPercent = (1 / (cropData.height || 1)) * 100
  const imgLeftPercent = -((cropData.x || 0) / (cropData.width || 1)) * 100
  const imgTopPercent = -((cropData.y || 0) / (cropData.height || 1)) * 100

  return (
    <div style={{ marginTop: 16, marginBottom: 16 }}>
      <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: 'var(--sf-text)', marginBottom: 8 }}>
        Kustomisasi Tampilan Background Banner
      </label>

      {/* Mode Switcher Tabs */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 8,
          marginBottom: 16,
          background: 'var(--sf-surface-subtle)',
          padding: 4,
          borderRadius: 8,
          border: '1px solid var(--sf-border)',
        }}
      >
        <button
          type="button"
          onClick={() => handleTypeSelect('color')}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 8,
            padding: '8px 14px',
            borderRadius: 6,
            fontSize: 13,
            fontWeight: 600,
            border: 'none',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            background: currentType === 'color' ? 'var(--sf-surface)' : 'transparent',
            color: currentType === 'color' ? 'var(--sf-text)' : 'var(--sf-text-muted)',
            boxShadow: currentType === 'color' ? 'var(--sf-shadow-sm)' : 'none',
          }}
        >
          <BgColorsOutlined />
          <span>Warna Custom (Neon Glow)</span>
        </button>

        <button
          type="button"
          onClick={() => handleTypeSelect('image')}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 8,
            padding: '8px 14px',
            borderRadius: 6,
            fontSize: 13,
            fontWeight: 600,
            border: 'none',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            background: currentType === 'image' ? 'var(--sf-surface)' : 'transparent',
            color: currentType === 'image' ? 'var(--sf-text)' : 'var(--sf-text-muted)',
            boxShadow: currentType === 'image' ? 'var(--sf-shadow-sm)' : 'none',
          }}
        >
          <PictureOutlined />
          <span>Gambar Background (16:9)</span>
        </button>
      </div>

      {/* Color Mode Controls */}
      {currentType === 'color' && (
        <div style={{ marginBottom: 16, padding: 12, borderRadius: 8, border: '1px solid var(--sf-border)', background: 'var(--sf-surface-subtle)' }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 8 }}>
            Pilih Warna Utama (Sistem otomatis menghasilkan efek neon glow & gradien halus):
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, alignItems: 'center', marginBottom: 12 }}>
            {NEON_PRESETS.map((p) => {
              const isSelected = currentColor.toUpperCase() === p.hex.toUpperCase()
              return (
                <button
                  key={p.hex}
                  type="button"
                  title={p.name}
                  onClick={() => handleColorChange(p.hex)}
                  style={{
                    width: 32,
                    height: 32,
                    borderRadius: 8,
                    backgroundColor: p.hex,
                    border: isSelected ? '2px solid #fff' : '1px solid rgba(255,255,255,0.2)',
                    boxShadow: isSelected ? `0 0 12px ${p.hex}` : 'none',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#fff',
                    transition: 'transform 0.15s ease',
                    transform: isSelected ? 'scale(1.1)' : 'scale(1)',
                  }}
                >
                  {isSelected && <CheckOutlined style={{ fontSize: 13, filter: 'drop-shadow(0 1px 2px rgba(0,0,0,0.8))' }} />}
                </button>
              )
            })}
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginLeft: 'auto' }}>
              <input
                type="color"
                value={currentColor.startsWith('#') && currentColor.length === 7 ? currentColor : '#A970FF'}
                onChange={(e) => handleColorChange(e.target.value)}
                style={{
                  width: 34,
                  height: 34,
                  padding: 0,
                  borderRadius: 6,
                  border: '1px solid var(--sf-border)',
                  cursor: 'pointer',
                  background: 'none',
                }}
              />
              <span style={{ fontSize: 12, fontFamily: 'monospace', color: 'var(--sf-text-muted)' }}>
                {currentColor}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* Image Mode Controls */}
      {currentType === 'image' && (
        <div style={{ marginBottom: 16, padding: 12, borderRadius: 8, border: '1px solid var(--sf-border)', background: 'var(--sf-surface-subtle)' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
            <label
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 8,
                padding: '6px 14px',
                borderRadius: 6,
                background: 'var(--sf-surface)',
                border: '1px solid var(--sf-border)',
                color: 'var(--sf-text)',
                fontSize: 13,
                fontWeight: 600,
                cursor: uploading ? 'not-allowed' : 'pointer',
              }}
            >
              {uploading ? <LoadingOutlined /> : <UploadOutlined />}
              <span>{currentImageUrl ? 'Ganti Gambar' : 'Pilih & Unggah Gambar'}</span>
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                onChange={handleImageUpload}
                disabled={uploading}
                style={{ display: 'none' }}
              />
            </label>

            {currentImageUrl && (
              <button
                type="button"
                onClick={handleResetCrop}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '6px 12px',
                  borderRadius: 6,
                  background: 'transparent',
                  border: '1px solid var(--sf-border)',
                  color: 'var(--sf-text-muted)',
                  fontSize: 12,
                  cursor: 'pointer',
                }}
              >
                <ReloadOutlined />
                <span>Reset Posisi & Zoom</span>
              </button>
            )}
          </div>

          {uploadError && (
            <div style={{ color: 'var(--sf-danger)', fontSize: 12, marginBottom: 8 }}>{uploadError}</div>
          )}

          {currentImageUrl ? (
            <div>
              {/* Interactive Cropper 16:9 */}
              <div
                style={{
                  position: 'relative',
                  width: '100%',
                  height: 220,
                  borderRadius: 8,
                  overflow: 'hidden',
                  background: 'linear-gradient(135deg, #2A124D 0%, #130924 100%)',
                  marginBottom: 10,
                }}
              >
                <Cropper
                  image={resolvedCropperUrl || currentImageUrl}
                  crop={crop}
                  zoom={zoom}
                  aspect={16 / 9}
                  onCropChange={setCrop}
                  onZoomChange={handleZoomChange}
                  onCropComplete={handleCropComplete}
                  showGrid={true}
                  style={{
                    containerStyle: { background: 'transparent' },
                    cropAreaStyle: { border: '2px solid #00F0FF', boxShadow: '0 0 10px rgba(0, 240, 255, 0.4)' },
                  }}
                />
              </div>

              {/* Zoom Slider */}
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <span style={{ fontSize: 12, color: 'var(--sf-text-muted)', minWidth: 40 }}>Zoom:</span>
                <input
                  type="range"
                  min={1}
                  max={3}
                  step={0.05}
                  value={zoom}
                  onChange={(e) => handleZoomChange(Number(e.target.value))}
                  style={{ flex: 1, accentColor: '#00F0FF' }}
                />
                <span style={{ fontSize: 12, fontFamily: 'monospace', color: 'var(--sf-text)', minWidth: 35 }}>
                  {zoom.toFixed(1)}x
                </span>
              </div>
            </div>
          ) : (
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', textAlign: 'center', padding: '16px 0' }}>
              Belum ada gambar background yang diunggah. Silakan klik tombol di atas untuk mengunggah gambar (PNG/JPG/WebP).
            </div>
          )}
        </div>
      )}

      {/* Show Detail Toggle */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 14px',
          borderRadius: 8,
          border: '1px solid var(--sf-border)',
          background: 'var(--sf-surface-subtle)',
          marginBottom: 16,
        }}
      >
        <div>
          <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--sf-text)' }}>
            Tampilkan Detail Teks pada Banner
          </div>
          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
            Jika dimatikan, banner hanya menampilkan judul (cocok jika gambar sudah memuat info lengkap).
          </div>
        </div>
        <label style={{ position: 'relative', display: 'inline-block', width: 44, height: 24, cursor: 'pointer' }}>
          <input
            type="checkbox"
            checked={currentShowDetail}
            onChange={(e) => handleShowDetailToggle(e.target.checked)}
            style={{ opacity: 0, width: 0, height: 0 }}
          />
          <span
            style={{
              position: 'absolute',
              inset: 0,
              backgroundColor: currentShowDetail ? 'var(--sf-success)' : 'var(--sf-border)',
              borderRadius: 24,
              transition: '0.2s',
            }}
          >
            <span
              style={{
                position: 'absolute',
                content: '""',
                height: 18,
                width: 18,
                left: currentShowDetail ? 23 : 3,
                bottom: 3,
                backgroundColor: '#fff',
                borderRadius: '50%',
                transition: '0.2s',
              }}
            />
          </span>
        </label>
      </div>

      {/* Live 16:9 Preview */}
      <div>
        <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 6 }}>
          Live Preview Banner (Rasio 16:9 seperti di Beranda):
        </div>
        <div
          style={{
            position: 'relative',
            width: '100%',
            aspectRatio: '16 / 9',
            borderRadius: 12,
            overflow: 'hidden',
            boxShadow:
              currentType === 'color'
                ? `0 4px 20px -2px ${currentColor}40, 0 0 15px ${currentColor}25`
                : '0 4px 20px rgba(0, 0, 0, 0.4)',
            border:
              currentType === 'color'
                ? `1px solid ${currentColor}60`
                : '1px solid rgba(255, 255, 255, 0.15)',
          }}
        >
          {/* Base Background: either neon ambient glow or violet gradient for image */}
          {currentType === 'color' ? (
            <div
              style={{
                position: 'absolute',
                inset: 0,
                backgroundColor: '#0C0916',
                overflow: 'hidden',
              }}
            >
              {/* Layer 1: Soft diagonal ambient color wash */}
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  background: `linear-gradient(135deg, ${currentColor}28 0%, rgba(12, 9, 22, 0.95) 75%)`,
                }}
              />
              {/* Layer 2: Primary broad diffused neon bloom at top-right */}
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  background: `radial-gradient(circle at 75% 25%, ${currentColor}99 0%, ${currentColor}33 35%, transparent 70%)`,
                }}
              />
              {/* Layer 3: Secondary subtle accent/reflection at bottom-left */}
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  background: `radial-gradient(circle at 15% 85%, ${currentColor}38 0%, transparent 55%)`,
                }}
              />
              {/* Layer 4: Content contrast scrim (subtle bottom dark gradient) */}
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  background: 'linear-gradient(to top, rgba(0, 0, 0, 0.7) 0%, rgba(0, 0, 0, 0.2) 40%, transparent 100%)',
                }}
              />
            </div>
          ) : (
            <div
              style={{
                position: 'absolute',
                inset: 0,
                background: 'linear-gradient(135deg, #2A124D 0%, #130924 100%)',
              }}
            >
              {currentImageUrl && (
                <div
                  style={{
                    position: 'absolute',
                    inset: 0,
                    overflow: 'hidden',
                  }}
                >
                  <img
                    src={resolvedCropperUrl || currentImageUrl}
                    alt="Preview"
                    style={{
                      position: 'absolute',
                      width: `${imgWidthPercent}%`,
                      height: `${imgHeightPercent}%`,
                      left: `${imgLeftPercent}%`,
                      top: `${imgTopPercent}%`,
                      maxWidth: 'none',
                      maxHeight: 'none',
                      objectFit: 'cover',
                    }}
                  />
                </div>
              )}
              {/* Soft bottom dark gradient overlay */}
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  background: `linear-gradient(to top, rgba(0, 0, 0, ${currentOpacity}) 0%, rgba(0, 0, 0, ${currentOpacity * 0.4}) 50%, rgba(0, 0, 0, 0) 100%)`,
                }}
              />
            </div>
          )}

          {/* Banner Overlaid Content (Always 16:9 responsive layout) */}
          <div
            style={{
              position: 'absolute',
              inset: 0,
              padding: '16px 20px',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
              zIndex: 2,
              boxSizing: 'border-box',
            }}
          >
            {/* Top row: Category Badge */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 700,
                  textTransform: 'uppercase',
                  letterSpacing: 0.5,
                  padding: '3px 8px',
                  borderRadius: 4,
                  background: 'rgba(0, 0, 0, 0.4)',
                  color: '#fff',
                  backdropFilter: 'blur(4px)',
                  border: '1px solid rgba(255, 255, 255, 0.2)',
                }}
              >
                {category === 'activity' ? 'Kegiatan' : 'Pengumuman'}
              </span>
            </div>

            {/* Bottom area: Title & optional detail */}
            <div style={{ marginTop: 'auto' }}>
              <h3
                style={{
                  margin: '0 0 4px 0',
                  fontSize: 16,
                  fontWeight: 700,
                  color: '#fff',
                  textShadow: '0 1px 3px rgba(0,0,0,0.8)',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
              >
                {titlePreview || 'Judul Banner'}
              </h3>

              {currentShowDetail && (
                <p
                  style={{
                    margin: 0,
                    fontSize: 12,
                    color: 'rgba(255, 255, 255, 0.85)',
                    lineHeight: 1.4,
                    textShadow: '0 1px 2px rgba(0,0,0,0.8)',
                    display: '-webkit-box',
                    WebkitLineClamp: 2,
                    WebkitBoxOrient: 'vertical',
                    overflow: 'hidden',
                  }}
                >
                  {bodyPreview || 'Deskripsi banner...'}
                </p>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
