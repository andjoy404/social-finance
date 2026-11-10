import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '@/app/AuthContext'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import {
  HomeOutlined,
  CalendarOutlined,
  NotificationOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
  RightOutlined,
  PhoneOutlined,
  ClockCircleOutlined,
  EnvironmentOutlined,
  AlertOutlined,
} from '@ant-design/icons'
import {
  apiGetHomeDashboard,
  HomeDashboardData,
  CommunityBanner,
  getSessionPair,
} from '@/app/api'
import { CommunityImage } from '@/components/CommunityImage'

export function Dashboard() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [data, setData] = useState<HomeDashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [activeBannerIdx, setActiveBannerIdx] = useState(0)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    const session = getSessionPair()
    if (!session?.accessToken) {
      setLoading(false)
      setError('Sesi tidak ditemukan. Silakan login ulang.')
      return
    }

    setLoading(true)
    setError(null)
    apiGetHomeDashboard(session.accessToken)
      .then((res) => {
        if (!cancelled) {
          setData(res)
          setLoading(false)
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err?.message || 'Gagal memuat Beranda. Periksa koneksi lalu coba lagi.')
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [])

  // Auto-rotate banner every 6 seconds if multiple banners
  const banners = data?.banners ?? []
  useEffect(() => {
    if (banners.length <= 1) return
    const timer = setInterval(() => {
      setActiveBannerIdx((prev) => (prev + 1) % banners.length)
    }, 6000)
    return () => clearInterval(timer)
  }, [banners.length])

  if (!user) return null

  const greeting = data?.greeting || 'Selamat Datang'
  const displayName = data?.user_full_name || user.name || 'Warga'
  const rtInfo = data?.rt_number ? `RT ${data.rt_number} / RW ${data.rw_number}` : (user.rt?.name ?? '')
  const activities = data?.upcoming_activities ?? []
  const orgNodes = data?.organization?.nodes ?? []
  const security = data?.security_on_duty

  const handleBannerClick = (b: CommunityBanner) => {
    if (b.deep_link) {
      navigate(b.deep_link)
    }
  }

  if (error && !data) {
    return (
      <div className="sf-card" style={{ padding: 48, textAlign: 'center' }}>
        <AlertOutlined style={{ fontSize: 32, marginBottom: 12, color: 'var(--sf-danger)' }} />
        <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-text)' }}>Gagal memuat Beranda</div>
        <div style={{ fontSize: 13, marginTop: 4, color: 'var(--sf-text-muted)' }}>{error}</div>
        <button type="button" className="sf-btn sf-btn-outline" style={{ marginTop: 16 }} onClick={() => window.location.reload()}>
          Coba Lagi
        </button>
      </div>
    )
  }

  if (loading && !data) {
    return (
      <div className="sf-card" style={{ padding: 48, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        Memuat Beranda...
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      {/* Header Greeting Surface */}
      <PageHeader
        surface
        icon={<HomeOutlined />}
        title={`${greeting}, ${displayName}`}
        subtitle={`${data?.rt_name || user.rt?.name || 'Social Finance'} • ${rtInfo}`}
        actions={
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            {data?.user_jabatan && (
              <Badge variant="violet">
                {data.user_jabatan.toUpperCase()}
              </Badge>
            )}
            <Badge variant="gray">
              {(data?.user_role || user.role || 'warga').toUpperCase()}
            </Badge>
          </div>
        }
      />

      {/* Active Banners Carousel / Grid */}
      {banners.length > 0 && (
        <div style={{ position: 'relative', overflow: 'hidden', borderRadius: 12 }}>
          {banners.map((banner, idx) => {
            if (idx !== activeBannerIdx && banners.length > 1) return null
            const isIuranReminder = banner.source_type === 'iuran_reminder'
            const bannerLabel = banner.source_type === 'iuran_reminder'
              ? 'Pengingat Iuran'
              : banner.source_type === 'activity'
                ? 'Kegiatan'
                : banner.source_type === 'announcement'
                  ? 'Pengumuman'
                  : 'Pengumuman RT'
            const bannerAlt = banner.source_type === 'activity' ? 'Banner kegiatan' : 'Banner pengumuman'
            return (
              <div
                key={banner.id}
                onClick={() => handleBannerClick(banner)}
                role={banner.deep_link ? 'link' : undefined}
                style={{
                  background: isIuranReminder
                    ? 'linear-gradient(135deg, rgba(235, 94, 40, 0.15) 0%, rgba(200, 70, 30, 0.25) 100%)'
                    : 'linear-gradient(135deg, rgba(169, 112, 255, 0.12) 0%, rgba(87, 148, 242, 0.2) 100%)',
                  border: isIuranReminder
                    ? '1px solid rgba(235, 94, 40, 0.4)'
                    : '1px solid rgba(169, 112, 255, 0.35)',
                  borderRadius: 12,
                  padding: '24px 28px',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  gap: 16,
                  cursor: banner.deep_link ? 'pointer' : 'default',
                  transition: 'all 0.3s ease',
                  minHeight: 110,
                  boxSizing: 'border-box',
                  overflow: 'hidden',
                }}
              >
                {banner.image_url && (
                  <div style={{ flex: '0 0 auto', width: 120, height: 80, borderRadius: 10, overflow: 'hidden', border: '1px solid rgba(255,255,255,0.18)' }}>
                    <CommunityImage
                      src={banner.image_url}
                      alt={bannerAlt}
                      style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                    />
                  </div>
                )}
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 18, maxWidth: '85%', minWidth: 0 }}>
                  <div
                    style={{
                      width: 48,
                      height: 48,
                      borderRadius: 10,
                      background: isIuranReminder ? 'var(--sf-danger)' : 'var(--sf-accent)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      color: '#fff',
                      fontSize: 22,
                      flexShrink: 0,
                    }}
                  >
                    {isIuranReminder ? <AlertOutlined /> : <NotificationOutlined />}
                  </div>
                  <div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 }}>
                      <span
                        style={{
                          fontSize: 11,
                          fontWeight: 700,
                          textTransform: 'uppercase',
                          letterSpacing: 0.5,
                          padding: '2px 8px',
                          borderRadius: 4,
                          background: isIuranReminder ? 'rgba(235, 94, 40, 0.2)' : 'rgba(169, 112, 255, 0.2)',
                          color: isIuranReminder ? 'var(--sf-danger)' : 'var(--sf-accent)',
                        }}
                      >
                        {bannerLabel}
                      </span>
                      {banner.is_pinned && <Badge variant="amber">PENTING</Badge>}
                    </div>
                    <h3 style={{ margin: '0 0 6px 0', fontSize: 18, fontWeight: 700, color: 'var(--sf-text)' }}>
                      {banner.title}
                    </h3>
                    {banner.body && (
                      <p style={{ margin: 0, fontSize: 13, color: 'var(--sf-text-muted)', lineHeight: 1.5 }}>
                        {banner.body}
                      </p>
                    )}
                  </div>
                </div>

                {banner.deep_link && (
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      fontSize: 13,
                      fontWeight: 600,
                      color: isIuranReminder ? 'var(--sf-danger)' : 'var(--sf-accent)',
                      flexShrink: 0,
                    }}
                  >
                    <span>{isIuranReminder ? 'Buka Keuangan' : 'Lihat Detail'}</span>
                    <RightOutlined style={{ fontSize: 11 }} />
                  </div>
                )}
              </div>
            )
          })}

          {/* Carousel indicators if multiple banners */}
          {banners.length > 1 && (
            <div
              style={{
                display: 'flex',
                justifyContent: 'center',
                gap: 6,
                marginTop: 8,
              }}
            >
              {banners.map((_, i) => (
                <button
                  key={i}
                  type="button"
                  onClick={() => setActiveBannerIdx(i)}
                  style={{
                    width: i === activeBannerIdx ? 20 : 6,
                    height: 6,
                    borderRadius: 3,
                    background: i === activeBannerIdx ? 'var(--sf-accent)' : 'var(--sf-border)',
                    border: 'none',
                    padding: 0,
                    cursor: 'pointer',
                    transition: 'all 0.25s ease',
                  }}
                  aria-label={`Banner slide ${i + 1}`}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {/* Two Column Layout: Security On-Duty & Upcoming Activities */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 20 }}>
        {/* Security Officer On-Duty Card */}
        <div className="sf-card" style={{ padding: 22 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div
                style={{
                  width: 36,
                  height: 36,
                  borderRadius: 8,
                  background: 'rgba(87, 148, 242, 0.15)',
                  color: 'var(--sf-info)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 18,
                }}
              >
                <SafetyCertificateOutlined />
              </div>
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>Petugas Keamanan Bertugas</h3>
            </div>
            <Badge variant={security?.is_available ? 'green' : 'gray'}>
              {security?.is_available ? 'AKTIF' : 'STANDBY'}
            </Badge>
          </div>

          {security?.officers && security.officers.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {security.officers.map((officer) => (
                <div
                  key={officer.user_id}
                  style={{
                    padding: '12px 14px',
                    borderRadius: 8,
                    background: 'var(--sf-bg)',
                    border: '1px solid var(--sf-border)',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                  }}
                >
                  <div>
                    <div style={{ fontWeight: 600, fontSize: 14 }}>{officer.full_name}</div>
                    <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 2 }}>
                      Shift: {officer.shift_name} ({officer.shift_start} - {officer.shift_end})
                    </div>
                  </div>
                  {officer.phone_number && (
                    <a
                      href={`tel:${officer.phone_number}`}
                      style={{
                        padding: '6px 10px',
                        borderRadius: 6,
                        background: 'rgba(46, 213, 115, 0.12)',
                        color: 'var(--sf-success)',
                        display: 'flex',
                        alignItems: 'center',
                        gap: 4,
                        fontSize: 12,
                        textDecoration: 'none',
                        fontWeight: 600,
                      }}
                    >
                      <PhoneOutlined /> Hubungi
                    </a>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <div
              style={{
                padding: '24px 16px',
                textAlign: 'center',
                borderRadius: 8,
                background: 'var(--sf-bg)',
                border: '1px dashed var(--sf-border)',
              }}
            >
              <div style={{ fontSize: 24, color: 'var(--sf-text-muted)', marginBottom: 8 }}>
                <ClockCircleOutlined />
              </div>
              <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--sf-text)' }}>
                {security?.message || 'Belum ada petugas aktif / jadwal shift saat ini'}
              </div>
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4 }}>
                Data shift terintegrasi langsung saat Modul Keamanan berjalan
              </div>
            </div>
          )}
        </div>

        {/* Upcoming Activities Preview Card */}
        <div className="sf-card" style={{ padding: 22 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div
                style={{
                  width: 36,
                  height: 36,
                  borderRadius: 8,
                  background: 'rgba(169, 112, 255, 0.15)',
                  color: 'var(--sf-accent)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 18,
                }}
              >
                <CalendarOutlined />
              </div>
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>Agenda & Kegiatan Mendatang</h3>
            </div>
            <button
              type="button"
              onClick={() => navigate('/kegiatan')}
              style={{
                background: 'none',
                border: 'none',
                color: 'var(--sf-accent)',
                fontSize: 13,
                fontWeight: 600,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 4,
                padding: 0,
              }}
            >
              Lihat Semua <RightOutlined style={{ fontSize: 10 }} />
            </button>
          </div>

          {activities.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {activities.slice(0, 4).map((activity) => {
                const startDate = new Date(activity.starts_at)
                return (
                  <div
                    key={activity.id}
                    onClick={() => navigate(`/kegiatan/${activity.id}`)}
                    style={{
                      padding: '12px 14px',
                      borderRadius: 8,
                      background: 'var(--sf-bg)',
                      border: '1px solid var(--sf-border)',
                      display: 'flex',
                      gap: 12,
                      alignItems: 'center',
                      cursor: 'pointer',
                    }}
                  >
                    {/* Date badge */}
                    <div
                      style={{
                        width: 44,
                        height: 48,
                        borderRadius: 6,
                        background: 'rgba(169, 112, 255, 0.1)',
                        display: 'flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                        justifyContent: 'center',
                        flexShrink: 0,
                      }}
                    >
                      <span style={{ fontSize: 16, fontWeight: 700, color: 'var(--sf-accent)', lineHeight: 1 }}>
                        {startDate.getDate()}
                      </span>
                      <span style={{ fontSize: 10, fontWeight: 600, color: 'var(--sf-text-muted)', textTransform: 'uppercase', marginTop: 2 }}>
                        {startDate.toLocaleDateString('id-ID', { month: 'short' })}
                      </span>
                    </div>

                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div
                        style={{
                          fontWeight: 600,
                          fontSize: 14,
                          color: 'var(--sf-text)',
                          whiteSpace: 'nowrap',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                        }}
                      >
                        {activity.title}
                      </div>
                      <div
                        style={{
                          fontSize: 12,
                          color: 'var(--sf-text-muted)',
                          marginTop: 3,
                          display: 'flex',
                          alignItems: 'center',
                          gap: 10,
                        }}
                      >
                        <span>
                          <ClockCircleOutlined style={{ fontSize: 11, marginRight: 3 }} />
                          {startDate.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })} WIB
                        </span>
                        {activity.location && (
                          <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                            <EnvironmentOutlined style={{ fontSize: 11, marginRight: 3 }} />
                            {activity.location}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div
              style={{
                padding: '24px 16px',
                textAlign: 'center',
                borderRadius: 8,
                background: 'var(--sf-bg)',
                border: '1px dashed var(--sf-border)',
                color: 'var(--sf-text-muted)',
                fontSize: 13,
              }}
            >
              Belum ada kegiatan mendatang
            </div>
          )}
        </div>
      </div>

      {/* Organization Structure Diagram */}
      <div className="sf-card" style={{ padding: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 20 }}>
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: 8,
              background: 'rgba(46, 213, 115, 0.15)',
              color: 'var(--sf-success)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 18,
            }}
          >
            <TeamOutlined />
          </div>
          <div>
            <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>Struktur Organisasi RT</h3>
            <p style={{ margin: 0, fontSize: 12, color: 'var(--sf-text-muted)' }}>
              Pengurus dan Penanggung Jawab Lingkungan {data?.rt_name || ''}
            </p>
          </div>
        </div>

        {orgNodes.length > 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 20, alignItems: 'center' }}>
            {/* Group nodes by level — render every level the backend provides */}
            {[...new Set(orgNodes.map((n) => n.level))].sort((a, b) => a - b).map((lvl) => {
              const nodesAtLevel = orgNodes.filter((n) => n.level === lvl)
              if (nodesAtLevel.length === 0) return null
              return (
                <div
                  key={lvl}
                  style={{
                    display: 'flex',
                    flexWrap: 'wrap',
                    justifyContent: 'center',
                    gap: 16,
                    width: '100%',
                  }}
                >
                  {nodesAtLevel.map((node) => (
                    <div
                      key={node.position}
                      style={{
                        flex: '1 1 240px',
                        maxWidth: 320,
                        padding: '16px 18px',
                        borderRadius: 10,
                        background: 'var(--sf-bg)',
                        border: '1px solid var(--sf-border)',
                        boxSizing: 'border-box',
                      }}
                    >
                      <div
                        style={{
                          fontSize: 11,
                          fontWeight: 700,
                          textTransform: 'uppercase',
                          color: lvl === 1 ? 'var(--sf-accent)' : lvl === 2 ? 'var(--sf-info)' : lvl === 3 ? 'var(--sf-success)' : 'var(--sf-text-muted)',
                          letterSpacing: 0.5,
                          marginBottom: 6,
                        }}
                      >
                        {node.position_label}
                      </div>

                      {node.members.length > 0 ? (
                        node.members.map((member) => (
                          <div key={member.user_id} style={{ marginTop: 4 }}>
                            <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--sf-text)' }}>
                              {member.full_name}
                            </div>
                            {member.phone && (
                              <div
                                style={{
                                  fontSize: 12,
                                  color: 'var(--sf-text-muted)',
                                  marginTop: 2,
                                  display: 'flex',
                                  alignItems: 'center',
                                  gap: 4,
                                }}
                              >
                                <PhoneOutlined style={{ fontSize: 11 }} /> {member.phone}
                              </div>
                            )}
                          </div>
                        ))
                      ) : (
                        <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', fontStyle: 'italic' }}>
                          Belum ditentukan
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )
            })}
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: '20px 0', color: 'var(--sf-text-muted)' }}>
            Belum ada struktur organisasi
          </div>
        )}
      </div>
    </div>
  )
}
