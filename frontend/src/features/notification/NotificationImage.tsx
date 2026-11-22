import React, { useCallback, useEffect, useRef, useState } from 'react'
import { LoadingOutlined, PictureOutlined, ReloadOutlined } from '@ant-design/icons'
import { ApiError, apiGetNotificationImageBlob } from '@/app/api'

export interface NotificationImageProps {
  /** Stored notification image URL (e.g. /api/v1/notifications/images/...) or local blob:/data: preview URL. */
  src: string
  alt: string
  style?: React.CSSProperties
  className?: string
  onClick?: () => void
  title?: string
  /** Compact mode for small thumbnails (e.g. 44x44 table cells). */
  compact?: boolean
}

type ErrorKind = 'unauthorized' | 'forbidden' | 'failed'

interface ImageErrorState {
  kind: ErrorKind
  message: string
}

function isLocalPreviewUrl(url: string): boolean {
  return url.startsWith('blob:') || url.startsWith('data:')
}

function classifyImageError(err: unknown): ImageErrorState {
  if (err instanceof ApiError) {
    if (err.status === 401 || err.code === 'unauthorized' || err.code === 'auth_expired') {
      return { kind: 'unauthorized', message: 'Sesi tidak valid untuk memuat gambar' }
    }
    if (err.status === 403 || err.code === 'forbidden') {
      return { kind: 'forbidden', message: 'Akses gambar ditolak' }
    }
  }
  return { kind: 'failed', message: 'Gagal memuat gambar' }
}

/**
 * Renders a notification attachment image fetched through the authenticated API client.
 *
 * - Local `blob:` / `data:` previews render directly without network requests.
 * - Backend notification image URLs are fetched via `apiGetNotificationImageBlob`
 *   (Bearer token + active RT header + token-refresh handling) and converted into
 *   a managed object URL.
 * - Cancels in-flight requests via AbortController and revokes object URLs on
 *   unmount or when `src` changes to prevent memory leaks and race conditions.
 */
export function NotificationImage({
  src,
  alt,
  style,
  className,
  onClick,
  title,
  compact = false,
}: NotificationImageProps) {
  const [resolvedUrl, setResolvedUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ImageErrorState | null>(null)
  const [retryCount, setRetryCount] = useState(0)
  const objectUrlRef = useRef<string | null>(null)

  const revokeObjectUrl = useCallback(() => {
    if (objectUrlRef.current) {
      URL.revokeObjectURL(objectUrlRef.current)
      objectUrlRef.current = null
    }
  }, [])

  useEffect(() => {
    const trimmed = src.trim()

    // Immediately clear any previous preview so stale images never linger.
    revokeObjectUrl()
    setResolvedUrl(null)
    setError(null)

    if (!trimmed) {
      setLoading(false)
      setError({ kind: 'failed', message: 'Gambar tidak tersedia' })
      return
    }

    if (isLocalPreviewUrl(trimmed)) {
      setResolvedUrl(trimmed)
      setLoading(false)
      return
    }

    let cancelled = false
    const controller = new AbortController()
    setLoading(true)

    apiGetNotificationImageBlob(trimmed, controller.signal)
      .then((blob) => {
        if (cancelled || controller.signal.aborted) {
          return
        }
        const objectUrl = URL.createObjectURL(blob)
        revokeObjectUrl()
        objectUrlRef.current = objectUrl
        setResolvedUrl(objectUrl)
        setError(null)
        setLoading(false)
      })
      .catch((err: unknown) => {
        if (cancelled || controller.signal.aborted) {
          return
        }
        if (err instanceof DOMException && err.name === 'AbortError') {
          return
        }
        setResolvedUrl(null)
        setError(classifyImageError(err))
        setLoading(false)
      })

    return () => {
      cancelled = true
      controller.abort()
      revokeObjectUrl()
    }
  }, [src, retryCount, revokeObjectUrl])

  if (loading) {
    return (
      <div
        role="status"
        aria-busy="true"
        aria-label="Memuat gambar..."
        className={className}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'var(--sf-bg-surface-secondary, rgba(148, 163, 184, 0.12))',
          border: '1px solid var(--sf-border)',
          borderRadius: 6,
          minWidth: compact ? 44 : 96,
          minHeight: compact ? 44 : 72,
          color: 'var(--sf-text-muted)',
          boxSizing: 'border-box',
          ...style,
        }}
      >
        <LoadingOutlined style={{ fontSize: compact ? 14 : 18 }} />
      </div>
    )
  }

  if (error || !resolvedUrl) {
    const canRetry = error?.kind !== 'forbidden'
    return (
      <div
        role="alert"
        aria-label={error?.message || 'Gagal memuat gambar'}
        title={error?.message || 'Gagal memuat gambar'}
        className={className}
        style={{
          display: 'inline-flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: compact ? 2 : 6,
          background: 'var(--sf-bg-surface-secondary, rgba(148, 163, 184, 0.12))',
          border: '1px dashed var(--sf-border)',
          borderRadius: 6,
          minWidth: compact ? 44 : 120,
          minHeight: compact ? 44 : 72,
          padding: compact ? 4 : 10,
          color: 'var(--sf-text-muted)',
          fontSize: 12,
          textAlign: 'center',
          boxSizing: 'border-box',
          ...style,
        }}
      >
        <PictureOutlined style={{ fontSize: compact ? 16 : 20 }} />
        {!compact && (
          <span>{error?.message || 'Gagal memuat gambar'}</span>
        )}
        {!compact && canRetry && (
          <button
            type="button"
            className="sf-btn-outline"
            onClick={(e) => {
              e.stopPropagation()
              setRetryCount((prev) => prev + 1)
            }}
            style={{
              padding: '2px 8px',
              fontSize: 11,
              display: 'inline-flex',
              alignItems: 'center',
              gap: 4,
            }}
          >
            <ReloadOutlined /> Muat ulang
          </button>
        )}
      </div>
    )
  }

  return (
    <img
      src={resolvedUrl}
      alt={alt}
      title={title}
      className={className}
      style={style}
      onClick={onClick}
    />
  )
}
