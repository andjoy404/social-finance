import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { UserOutlined } from '@ant-design/icons'
import { apiSignCommunityImage, getSessionPair } from '@/app/api'
import { FullImageViewer } from './FullImageViewer'

type ResidentAvatarProps = {
  name: string
  /** Stored residents.avatar_path — storage key or full /api/v1/community/images/... path. */
  avatarPath?: string | null
  /** Stored residents.avatar_thumb_path — thumbnail storage key or full URL. */
  avatarThumbPath?: string | null
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
 * Renders the stored profile photo thumbnail (200x200 JPEG) when available,
 * falling back to the original photo and then initials. Built from plain
 * div/img so it composes with any theme context and stays robust in tests.
 * Full image viewer always loads the original full-resolution photo.
 */
export function ResidentAvatar({ name, avatarPath, avatarThumbPath, size = 40, className, style }: ResidentAvatarProps) {
  const [resolved, setResolved] = useState<string | null>(null)
  const [viewerOpen, setViewerOpen] = useState<boolean>(false)

  useEffect(() => {
    setResolved(null)
    const primary = (avatarThumbPath ?? '').trim() || (avatarPath ?? '').trim()
    if (!primary) return
    const session = getSessionPair()
    if (!session?.accessToken) return
    let cancelled = false
    apiSignCommunityImage(session.accessToken, primary)
      .then((res) => {
        if (!cancelled) setResolved(res.url)
      })
      .catch(() => {
        // Fallback to original avatarPath if primary was thumbnail and failed
        const fallback = (avatarPath ?? '').trim()
        if (fallback && fallback !== primary) {
          apiSignCommunityImage(session.accessToken, fallback)
            .then((res) => {
              if (!cancelled) setResolved(res.url)
            })
            .catch(() => {
              if (!cancelled) setResolved(null)
            })
        } else {
          if (!cancelled) setResolved(null)
        }
      })
    return () => {
      cancelled = true
    }
  }, [avatarPath, avatarThumbPath])

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

  const handleError = () => {
    const fallback = (avatarPath ?? '').trim()
    const primary = (avatarThumbPath ?? '').trim()
    const session = getSessionPair()
    if (primary && fallback && primary !== fallback && session?.accessToken) {
      apiSignCommunityImage(session.accessToken, fallback)
        .then((res) => setResolved(res.url))
        .catch(() => setResolved(null))
    } else {
      setResolved(null)
    }
  }

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
      onError={handleError}
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
          avatarPath={(avatarPath ?? '').trim() || (avatarThumbPath ?? '').trim()}
          alt={name}
          onClose={() => setViewerOpen(false)}
        />,
        document.body,
      )}
    </>
  )
}
