import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { UserOutlined } from '@ant-design/icons'
import { apiSignCommunityImage, getSessionPair } from '@/app/api'
import { FullImageViewer } from './FullImageViewer'

type ResidentAvatarProps = {
  name: string
  /** Stored residents.avatar_path — storage key or full /api/v1/community/images/... path. */
  avatarPath?: string | null
  size?: number
  className?: string
  style?: React.CSSProperties
}

/** Deterministic up-to-2-letter initials from a display name. */
function initialsFor(name: string | null | undefined): string {
  const text = (name ?? '').trim()
  if (!text) return ''
  const parts = text.split(/\s+/).filter(Boolean)
  const first = parts[0]?.[0]?.toUpperCase() ?? ''
  const last = parts.length > 1 ? parts[parts.length - 1]?.[0]?.toUpperCase() ?? '' : ''
  return (first + last).slice(0, 2)
}

/**
 * Avatar for a resident (warga biasa or petugas keamanan).
 *
 * Renders the stored profile photo when available (resolved through the
 * existing authorized community-images pipeline), otherwise a deterministic
 * initials fallback. Built from plain div/img so it composes with any
 * theme context and stays robust in jsdom-based tests.
 */
export function ResidentAvatar({ name, avatarPath, size = 40, className, style }: ResidentAvatarProps) {
  const [resolved, setResolved] = useState<string | null>(null)
  const [viewerOpen, setViewerOpen] = useState<boolean>(false)

  useEffect(() => {
    setResolved(null)
    if (!avatarPath) return
    const session = getSessionPair()
    if (!session?.accessToken) return
    let cancelled = false
    apiSignCommunityImage(session.accessToken, avatarPath)
      .then((res) => {
        if (!cancelled) setResolved(res.url)
      })
      .catch(() => {
        if (!cancelled) setResolved(null)
      })
    return () => {
      cancelled = true
    }
  }, [avatarPath])

  const initials = initialsFor(name)
  const fontSize = Math.max(11, Math.round(size * 0.38))
  const common: React.CSSProperties = {
    width: size,
    height: size,
    borderRadius: '50%',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    overflow: 'hidden',
    ...style,
  }

  if (!resolved) {
    return (
      <span
        className={className}
        style={{
          ...common,
          backgroundColor: 'var(--sf-accent, #3b82f6)',
          color: '#ffffff',
          fontWeight: 600,
          fontSize,
          lineHeight: 1,
        }}
        aria-hidden="true"
      >
        {initials || <UserOutlined style={{ fontSize }} />}
      </span>
    )
  }

  const openViewer = resolved ? () => setViewerOpen(true) : undefined

  const avatar = (
    <img
      src={resolved}
      alt={name}
      className={className}
      role={openViewer ? 'button' : undefined}
      aria-label={openViewer ? `Lihat foto profil ${name} dalam ukuran besar` : undefined}
      tabIndex={openViewer ? 0 : undefined}
      onKeyDown={
        openViewer
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                openViewer()
              }
            }
          : undefined
      }
      onClick={openViewer}
      onError={() => setResolved(null)}
      style={{
        ...common,
        objectFit: 'cover',
        cursor: openViewer ? 'zoom-in' : undefined,
      }}
    />
  )

  if (!viewerOpen) return avatar

  return (
    <>
      {avatar}
      {createPortal(
        <FullImageViewer
          avatarPath={avatarPath ?? ''}
          alt={name}
          onClose={() => setViewerOpen(false)}
        />,
        document.body,
      )}
    </>
  )
}
