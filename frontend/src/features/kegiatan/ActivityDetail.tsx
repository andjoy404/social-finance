import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { CommunityImage } from '@/components/CommunityImage'
import {
  CalendarOutlined,
  ClockCircleOutlined,
  EnvironmentOutlined,
  UserOutlined,
  ArrowLeftOutlined,
} from '@ant-design/icons'
import {
  CommunityActivity,
  apiGetActivity,
  getSessionPair,
} from '@/app/api'

function getStatusBadge(status: string) {
  switch (status) {
    case 'upcoming':
      return <Badge variant="violet">Mendatang</Badge>
    case 'ongoing':
      return <Badge variant="green">Berlangsung</Badge>
    case 'completed':
      return <Badge variant="gray">Selesai</Badge>
    case 'cancelled':
      return <Badge variant="red">Dibatalkan</Badge>
    default:
      return <Badge variant="default">{status}</Badge>
  }
}

export function ActivityDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [activity, setActivity] = useState<CommunityActivity | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    const session = getSessionPair()
    if (!session?.accessToken || !id) {
      setLoading(false)
      setError('Sesi atau parameter kegiatan tidak valid.')
      return
    }
    setLoading(true)
    setError(null)
    apiGetActivity(session.accessToken, id)
      .then((res) => {
        setActivity(res)
        setLoading(false)
      })
      .catch((err) => {
        setError(err?.message || 'Kegiatan tidak ditemukan atau tidak dapat dibuka.')
        setActivity(null)
        setLoading(false)
      })
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  if (loading) {
    return (
      <div className="sf-card" style={{ padding: 48, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        Memuat detail kegiatan...
      </div>
    )
  }

  if (error || !activity) {
    return (
      <div className="sf-card" style={{ padding: 48, textAlign: 'center' }}>
        <CalendarOutlined style={{ fontSize: 32, marginBottom: 12, color: 'var(--sf-text-muted)' }} />
        <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-text)' }}>Gagal memuat detail kegiatan</div>
        <div style={{ fontSize: 13, marginTop: 4, color: 'var(--sf-text-muted)' }}>{error ?? 'Kegiatan tidak ditemukan.'}</div>
        <button type="button" className="sf-btn sf-btn-outline" style={{ marginTop: 16 }} onClick={load}>
          Coba Lagi
        </button>
      </div>
    )
  }

  const start = new Date(activity.starts_at)
  const end = activity.ends_at ? new Date(activity.ends_at) : null
  const isCancelled = activity.status === 'cancelled'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <PageHeader
        surface
        icon={<CalendarOutlined />}
        title={activity.title}
        subtitle={`${start.toLocaleDateString('id-ID', { dateStyle: 'full' })} • ${start.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB`}
        actions={
          <button type="button" className="sf-btn sf-btn-outline" onClick={() => navigate('/kegiatan')}>
            <ArrowLeftOutlined /> Kembali
          </button>
        }
      />

      {activity.poster_url && (
        <div className="sf-card" style={{ padding: 0, overflow: 'hidden', borderRadius: 12 }}>
          <CommunityImage
            src={activity.poster_url}
            alt={`Poster kegiatan ${activity.title}`}
            style={{ width: '100%', maxHeight: 360, objectFit: 'cover' }}
          />
        </div>
      )}

      <div className="sf-card" style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          {getStatusBadge(activity.status)}
          {isCancelled && <Badge variant="red">Kegiatan Dibatalkan</Badge>}
        </div>

        {activity.description && (
          <p style={{ margin: 0, fontSize: 14, color: 'var(--sf-text)', lineHeight: 1.7, whiteSpace: 'pre-line' }}>
            {activity.description}
          </p>
        )}

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13, color: 'var(--sf-text)' }}>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
            <ClockCircleOutlined style={{ color: 'var(--sf-accent)', marginTop: 2 }} />
            <div>
              <div style={{ fontWeight: 600 }}>Waktu Pelaksanaan</div>
              <div style={{ color: 'var(--sf-text-muted)' }}>
                {start.toLocaleDateString('id-ID', { dateStyle: 'full' })} •{' '}
                {start.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB
                {end && <> — {end.toLocaleDateString('id-ID', { dateStyle: 'medium' })} {end.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB</>}
              </div>
            </div>
          </div>

          {activity.location && (
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
              <EnvironmentOutlined style={{ color: 'var(--sf-accent)', marginTop: 2 }} />
              <div>
                <div style={{ fontWeight: 600 }}>Lokasi</div>
                <div style={{ color: 'var(--sf-text-muted)' }}>{activity.location}</div>
              </div>
            </div>
          )}

          {activity.organizer && (
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
              <UserOutlined style={{ color: 'var(--sf-accent)', marginTop: 2 }} />
              <div>
                <div style={{ fontWeight: 600 }}>Penyelenggara</div>
                <div style={{ color: 'var(--sf-text-muted)' }}>{activity.organizer}</div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
