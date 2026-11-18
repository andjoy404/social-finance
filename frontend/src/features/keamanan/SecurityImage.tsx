import React, { useEffect, useState, useRef, useCallback } from 'react'
import {
  PictureOutlined,
  LoadingOutlined,
  CloseOutlined,
  LeftOutlined,
  RightOutlined,
} from '@ant-design/icons'
import { apiGetPatrolPhotoBlob, apiGetPaketPhotoBlob } from './securityApi'
import { formatJakartaDateTime } from '@/utils/jakartaTime'

export interface SecurityImageProps {
  photoId: string
  domain: 'patrol' | 'paket'
  alt?: string
  className?: string
  style?: React.CSSProperties
  onClick?: () => void
}

export function SecurityImage({
  photoId,
  domain,
  alt = 'Foto',
  className,
  style,
  onClick,
}: SecurityImageProps) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const objectUrlRef = useRef<string | null>(null)

  const cleanup = useCallback(() => {
    if (objectUrlRef.current) {
      URL.revokeObjectURL(objectUrlRef.current)
      objectUrlRef.current = null
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    cleanup()
    setLoading(true)
    setError(false)

    if (!photoId) {
      setLoading(false)
      setError(true)
      return
    }

    const fetcher = domain === 'patrol' ? apiGetPatrolPhotoBlob : apiGetPaketPhotoBlob
    fetcher(photoId)
      .then((blob) => {
        if (cancelled) return
        const url = URL.createObjectURL(blob)
        objectUrlRef.current = url
        setBlobUrl(url)
        setLoading(false)
      })
      .catch(() => {
        if (cancelled) return
        setError(true)
        setLoading(false)
      })

    return () => {
      cancelled = true
      cleanup()
    }
  }, [photoId, domain, cleanup])

  if (loading) {
    return (
      <div
        className={className}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: 'var(--sf-bg-surface-secondary, rgba(0,0,0,0.05))',
          borderRadius: 8,
          minHeight: 80,
          ...style,
        }}
        aria-label="Memuat foto..."
      >
        <LoadingOutlined style={{ fontSize: 20, color: 'var(--sf-text-muted, #8c8c8c)' }} />
      </div>
    )
  }

  if (error || !blobUrl) {
    return (
      <div
        className={className}
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: 'var(--sf-bg-surface-secondary, rgba(0,0,0,0.05))',
          borderRadius: 8,
          minHeight: 80,
          color: 'var(--sf-text-muted, #8c8c8c)',
          padding: 8,
          fontSize: 12,
          ...style,
        }}
      >
        <PictureOutlined style={{ fontSize: 24, marginBottom: 4 }} />
        <span>Gagal memuat foto</span>
      </div>
    )
  }

  return (
    <img
      src={blobUrl}
      alt={alt}
      className={className}
      style={{
        objectFit: 'cover',
        borderRadius: 8,
        cursor: onClick ? 'pointer' : 'default',
        ...style,
      }}
      onClick={onClick}
    />
  )
}

export interface LightboxPhoto {
  id: string
  title?: string
  location?: string
  timestamp?: string
  photoType?: string
}

export interface SecurityPhotoLightboxProps {
  photos: LightboxPhoto[]
  initialIndex?: number
  domain: 'patrol' | 'paket'
  open: boolean
  onClose: () => void
}

export function SecurityPhotoLightbox({
  photos,
  initialIndex = 0,
  domain,
  open,
  onClose,
}: SecurityPhotoLightboxProps) {
  const [currentIndex, setCurrentIndex] = useState(initialIndex)

  useEffect(() => {
    setCurrentIndex(initialIndex)
  }, [initialIndex, open])

  useEffect(() => {
    if (!open) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      else if (e.key === 'ArrowLeft') {
        setCurrentIndex((prev) => (prev > 0 ? prev - 1 : photos.length - 1))
      } else if (e.key === 'ArrowRight') {
        setCurrentIndex((prev) => (prev < photos.length - 1 ? prev + 1 : 0))
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open, photos.length, onClose])

  if (!open || photos.length === 0) return null

  const current = photos[currentIndex] || photos[0]

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Pratinjau Foto"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.85)',
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 16,
      }}
      onClick={onClose}
    >
      {/* Top Bar */}
      <div
        style={{
          position: 'absolute',
          top: 16,
          right: 16,
          left: 16,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          color: '#ffffff',
          zIndex: 10000,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ fontSize: 14, fontWeight: 500 }}>
          {photos.length > 1 && `${currentIndex + 1} / ${photos.length}`}
        </div>
        <button
          onClick={onClose}
          aria-label="Tutup"
          style={{
            background: 'rgba(255, 255, 255, 0.2)',
            border: 'none',
            borderRadius: '50%',
            color: '#ffffff',
            width: 36,
            height: 36,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            fontSize: 18,
          }}
        >
          <CloseOutlined />
        </button>
      </div>

      {/* Main Image Box */}
      <div
        style={{
          position: 'relative',
          maxWidth: '90vw',
          maxHeight: '75vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <SecurityImage
          photoId={current.id}
          domain={domain}
          alt={current.title || 'Foto'}
          style={{
            maxWidth: '90vw',
            maxHeight: '75vh',
            objectFit: 'contain',
            borderRadius: 8,
          }}
        />

        {/* Prev / Next Buttons */}
        {photos.length > 1 && (
          <>
            <button
              onClick={() => setCurrentIndex((prev) => (prev > 0 ? prev - 1 : photos.length - 1))}
              aria-label="Foto Sebelumnya"
              style={{
                position: 'absolute',
                left: -48,
                background: 'rgba(255, 255, 255, 0.2)',
                border: 'none',
                borderRadius: '50%',
                color: '#ffffff',
                width: 40,
                height: 40,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                cursor: 'pointer',
                fontSize: 18,
              }}
            >
              <LeftOutlined />
            </button>
            <button
              onClick={() => setCurrentIndex((prev) => (prev < photos.length - 1 ? prev + 1 : 0))}
              aria-label="Foto Selanjutnya"
              style={{
                position: 'absolute',
                right: -48,
                background: 'rgba(255, 255, 255, 0.2)',
                border: 'none',
                borderRadius: '50%',
                color: '#ffffff',
                width: 40,
                height: 40,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                cursor: 'pointer',
                fontSize: 18,
              }}
            >
              <RightOutlined />
            </button>
          </>
        )}
      </div>

      {/* Bottom Info Bar */}
      <div
        style={{
          marginTop: 16,
          color: '#ffffff',
          textAlign: 'center',
          maxWidth: 600,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {current.title && <div style={{ fontSize: 16, fontWeight: 600 }}>{current.title}</div>}
        {current.location && (
          <div style={{ fontSize: 13, color: '#e0e0e0', marginTop: 2 }}>{current.location}</div>
        )}
        {current.timestamp && (
          <div style={{ fontSize: 12, color: '#aaaaaa', marginTop: 4 }}>
            {formatJakartaDateTime(current.timestamp)}
          </div>
        )}
      </div>
    </div>
  )
}
