import { useCallback, useEffect, useRef, useState } from 'react'
import { Button, Spin } from 'antd'
import { CloseOutlined, LoadingOutlined, PictureOutlined } from '@ant-design/icons'
import { apiSignCommunityImage, getSessionPair } from '@/app/api'

type FullImageViewerProps = {
  /** Stored residents.avatar_path or full /api/v1/community/images/... path. */
  avatarPath: string
  alt: string
  onClose: () => void
}

/**
 * Full-screen pinch-to-zoom / pan-friendly image viewer.
 *
 * Reuses the authorized community-images pipeline (signed URL issued by
 * the backend after it validates the caller's JWT + RT membership and
 * the resident.avatar_path reference). No brand-new endpoint or public
 * object exposure is introduced.
 */
export function FullImageViewer({ avatarPath, alt, onClose }: FullImageViewerProps) {
  const [src, setSrc] = useState<string | null>(null)
  const [failed, setFailed] = useState<boolean>(false)
  const [scale, setScale] = useState(1)
  const [translation, setTranslation] = useState({ x: 0, y: 0 })

  const containerRef = useRef<HTMLDivElement | null>(null)
  const dragState = useRef<{ startX: number; startY: number; baseX: number; baseY: number } | null>(null)

  useEffect(() => {
    const session = getSessionPair()
    if (!session?.accessToken) {
      setFailed(true)
      return
    }
    let cancelled = false
    apiSignCommunityImage(session.accessToken, avatarPath)
      .then((res) => {
        if (!cancelled) setSrc(res.url)
      })
      .catch(() => {
        if (!cancelled) setFailed(true)
      })
    return () => {
      cancelled = true
    }
  }, [avatarPath])

  // ESC closes.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      if (e.key === '+' || e.key === '=') setScale((s) => Math.min(4, +(s + 0.5).toFixed(1)))
      if (e.key === '-') setScale((s) => Math.max(0.5, +(s - 0.5).toFixed(1)))
      if (e.key === '0') {
        setScale(1)
        setTranslation({ x: 0, y: 0 })
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const onPointerDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    if (scale <= 1) return
    ;(e.currentTarget as HTMLDivElement).setPointerCapture(e.pointerId)
    dragState.current = { startX: e.clientX, startY: e.clientY, baseX: translation.x, baseY: translation.y }
  }, [scale, translation])

  const onPointerMove = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    if (!dragState.current) return
    const dx = e.clientX - dragState.current.startX
    const dy = e.clientY - dragState.current.startY
    setTranslation({ x: dragState.current.baseX + dx, y: dragState.current.baseY + dy })
  }, [])

  const onPointerUp = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    if (!dragState.current) return
    try {
      ;(e.currentTarget as HTMLDivElement).releasePointerCapture(e.pointerId)
    } catch {
      /* best effort */
    }
    dragState.current = null
  }, [])

  const onWheel = useCallback((e: React.WheelEvent<HTMLDivElement>) => {
    if (e.deltaY === 0) return
    const next = e.deltaY < 0 ? scale + 0.25 : scale - 0.25
    const clamped = Math.max(0.5, Math.min(4, +next.toFixed(2)))
    if (clamped <= 1) setTranslation({ x: 0, y: 0 })
    setScale(clamped)
  }, [scale])

  const resetView = useCallback(() => {
    setScale(1)
    setTranslation({ x: 0, y: 0 })
  }, [])

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Foto profil ${alt}`}
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 1000,
        background: 'rgba(0, 0, 0, 0.92)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        touchAction: 'none',
        overscrollBehavior: 'contain',
      }}
      onClick={onClose}
    >
      {/* Close button */}
      <Button
        shape="circle"
        icon={<CloseOutlined />}
        size="large"
        aria-label="Tutup viewer"
        onClick={onClose}
        style={{
          position: 'absolute',
          top: 16,
          right: 16,
          background: 'rgba(255,255,255,0.15)',
          color: '#fff',
          borderColor: 'rgba(255,255,255,0.3)',
          zIndex: 1001,
        }}
      />

      <div
        ref={containerRef}
        style={{
          position: 'relative',
          width: '100%',
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          overflow: 'hidden',
          cursor: scale > 1 ? 'grab' : 'default',
        }}
        onClick={(e) => e.stopPropagation()}
        onWheel={onWheel}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
      >
        {failed ? (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 12,
              color: '#fff',
              padding: 24,
              textAlign: 'center',
            }}
          >
            <PictureOutlined style={{ fontSize: 56, opacity: 0.8 }} />
            <div style={{ fontSize: 16, fontWeight: 600 }}>Gagal memuat foto profil.</div>
            <div style={{ fontSize: 13, opacity: 0.75 }}>Pastikan koneksi aktif dan coba kembali.</div>
          </div>
        ) : !src ? (
          <Spin
            indicator={<LoadingOutlined spin style={{ fontSize: 40, color: '#fff' }} />}
          />
        ) : (
          <img
            src={src}
            alt={alt}
            draggable={false}
            style={{
              maxWidth: '95vw',
              maxHeight: '95vh',
              objectFit: 'contain',
              transform: `translate(${translation.x}px, ${translation.y}px) scale(${scale})`,
              transformOrigin: 'center center',
              transition: dragState.current ? 'none' : 'transform 0.15s ease',
              userSelect: 'none',
              touchAction: 'none',
            }}
            onDoubleClick={() => {
              if (scale === 1) setScale(2)
              else resetView()
            }}
            onError={() => setFailed(true)}
          />
        )}
      </div>

      {/* Hint bar */}
      {!failed && (
        <div
          style={{
            position: 'absolute',
            bottom: 20,
            left: 0,
            right: 0,
            display: 'flex',
            justifyContent: 'center',
            color: 'rgba(255,255,255,0.65)',
            fontSize: 12,
            pointerEvents: 'none',
          }}
        >
          Gunakan scroll / cubit untuk zoom · Esc untuk tutup
        </div>
      )}
    </div>
  )
}
