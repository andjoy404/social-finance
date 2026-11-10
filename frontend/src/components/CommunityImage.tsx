import { useEffect, useState } from 'react'
import { PictureOutlined } from '@ant-design/icons'
import { useAuth, canManageAnnouncements, canManageActivities } from '@/app/AuthContext'
import { apiSignCommunityImage, apiGetCommunityImageBlob, getSessionPair } from '@/app/api'

type CommunityImageProps = {
  /** Stored reference (full API path or raw storage key). */
  src: string
  alt: string
  style?: React.CSSProperties
  className?: string
}

/**
 * Renders a community image that lives behind the authorized image endpoint.
 *
 * Resolution order:
 *  1. Short-lived signed URL (warga-visible content).
 *  2. Bearer blob fetch — only when the caller may manage activities or
 *     announcements (draft/archived content for managers). The object URL
 *     is revoked on cleanup and when it stops being used.
 *  3. Neutral placeholder otherwise.
 *
 * Token never enters the URL nor the logs. The component re-resolves when
 * the asset, the permission set, or the user identity/RT changes, and a
 * stale async result can never overwrite a newer one.
 */
export function CommunityImage({ src, alt, style, className }: CommunityImageProps) {
  const { user } = useAuth()
  const [resolved, setResolved] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)

  const canManage = canManageAnnouncements(user) || canManageActivities(user)
  const userId = user?.id ?? null
  const rtId = user?.rt?.id ?? null

  useEffect(() => {
    let cancelled = false
    let createdObjectUrl: string | null = null

    setResolved(null)
    setFailed(false)

    const session = getSessionPair()
    if (!session?.accessToken || !src) {
      setFailed(true)
      return
    }

    const token = session.accessToken

    apiSignCommunityImage(token, src)
      .then((res) => {
        if (!cancelled) setResolved(res.url)
      })
      .catch(async () => {
        if (!canManage) {
          if (!cancelled) setFailed(true)
          return
        }
        try {
          const { blob } = await apiGetCommunityImageBlob(token, src)
          if (cancelled) return
          createdObjectUrl = URL.createObjectURL(blob)
          if (!cancelled) {
            setResolved(createdObjectUrl)
          } else {
            URL.revokeObjectURL(createdObjectUrl)
          }
        } catch {
          if (!cancelled) setFailed(true)
        }
      })

    return () => {
      cancelled = true
      if (createdObjectUrl) {
        URL.revokeObjectURL(createdObjectUrl)
      }
    }
  }, [src, canManage, userId, rtId])

  if (failed) {
    return (
      <div
        role="img"
        aria-label={alt}
        className={className}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'var(--sf-bg)',
          border: '1px dashed var(--sf-border)',
          color: 'var(--sf-text-muted)',
          fontSize: 18,
          boxSizing: 'border-box',
          ...style,
        }}
      >
        <PictureOutlined />
      </div>
    )
  }

  if (!resolved) {
    return (
      <div
        aria-hidden="true"
        className={className}
        style={{
          background: 'var(--sf-bg)',
          border: '1px solid var(--sf-border)',
          boxSizing: 'border-box',
          ...style,
        }}
      />
    )
  }

  return (
    <img
      src={resolved}
      alt={alt}
      className={className}
      style={style}
      onError={() => setFailed(true)}
    />
  )
}
