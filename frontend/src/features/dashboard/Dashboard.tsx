import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '@/app/AuthContext'
import { ResidentAvatar } from '@/components/ResidentAvatar'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import {
  HomeOutlined,
  CalendarOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
  IdcardOutlined,
  RightOutlined,
  PhoneOutlined,
  ClockCircleOutlined,
  EnvironmentOutlined,
  AlertOutlined,
  CloseOutlined,
  LinkOutlined,
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
  const [selectedAnnouncementBanner, setSelectedAnnouncementBanner] = useState<CommunityBanner | null>(null)

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

  // Map RT position to Badge variant — matches Android _getJabatanColor
  const getJabatanBadgeVariant = (position: string): 'violet' | 'deepPurple' | 'lightPurple' | 'amber' | 'green' | 'red' | 'blue' | 'gray' | 'teal' | 'chartreuse' | 'pink' | 'default' => {
    const key = (position ?? '').trim().toLowerCase().replace(/\s+/g, '_')
    switch (key) {
      case 'ketua':
      case 'ketua_rt': return 'violet'
      case 'wakil_ketua':
      case 'wakil_ketua_rt': return 'blue'
      case 'sekretaris':
      case 'sekretaris_rt': return 'green'
      case 'bendahara':
      case 'bendahara_rt': return 'amber'
      case 'humas':
      case 'humas_rt':
      case 'seksi_humas':
      case 'seksi_humas_rt': return 'amber'
      case 'keamanan':
      case 'keamanan_rt':
      case 'seksi_keamanan':
      case 'seksi_keamanan_rt': return 'red'
      case 'operasional':
      case 'operasional_rt': return 'teal'
      case 'kebersihan':
      case 'kebersihan_rt': return 'chartreuse'
      case 'sosial':
      case 'sosial_rt':
      case 'seksi_sosial': return 'pink'
      default: return 'default'
    }
  }

  const handleBannerClick = (b: CommunityBanner) => {
    const title = (b.title ?? '').trim()
    if (title === 'Pengingat Iuran Warga' || b.source_type === 'iuran_reminder') {
      navigate('/iuran')
      return
    }
    if (b.source_type === 'activity') {
      if (b.source_id) {
        navigate(`/kegiatan/${b.source_id}`)
        return
      }
      if (b.deep_link) {
        navigate(b.deep_link)
        return
      }
      navigate('/kegiatan')
      return
    }
    if (b.source_type === 'announcement') {
      setSelectedAnnouncementBanner(b)
      return
    }
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

      {/* Active Banners Carousel / Grid (Responsive 16:9 Aspect Ratio) */}
      {banners.length > 0 && (
        <div style={{ position: 'relative', overflow: 'hidden', borderRadius: 12 }}>
          {banners.map((banner, idx) => {
            if (idx !== activeBannerIdx && banners.length > 1) return null
            const isIuranReminder = banner.source_type === 'iuran_reminder'
            const bannerLabel = isIuranReminder
              ? 'Pengingat Iuran'
              : banner.source_type === 'activity'
                ? 'Kegiatan'
                : banner.source_type === 'announcement'
                  ? 'Pengumuman'
                  : 'Pengumuman RT'
            const bannerAlt = banner.source_type === 'activity' ? 'Banner kegiatan' : 'Banner pengumuman'

            const styleCfg = banner.style_config
            const isColorBg = !isIuranReminder && styleCfg?.background_type === 'color'
            const isImageBg = !isIuranReminder && (styleCfg?.background_type === 'image' || (!styleCfg && !!banner.image_url))
            const customColor = styleCfg?.background_color || '#A970FF'
            const showDetail = styleCfg?.show_detail !== false

            // Mathematical 16:9 crop calculations
            const crop = styleCfg?.image?.crop
            const imgUrl = styleCfg?.image?.url || banner.image_url
            const overlayOpacity = styleCfg?.overlay_opacity ?? 0.35

            return (
              <div
                key={banner.id}
                onClick={() => handleBannerClick(banner)}
                role={banner.deep_link ? 'link' : undefined}
                style={{
                  position: 'relative',
                  width: '100%',
                  aspectRatio: '16 / 9',
                  borderRadius: 12,
                  overflow: 'hidden',
                  cursor: banner.deep_link ? 'pointer' : 'default',
                  boxSizing: 'border-box',
                  border: isIuranReminder
                    ? '1px solid rgba(235, 94, 40, 0.4)'
                    : isColorBg
                      ? `1px solid ${customColor}60`
                      : '1px solid rgba(255, 255, 255, 0.15)',
                  boxShadow: isColorBg
                    ? `0 6px 24px -2px ${customColor}35, 0 0 16px ${customColor}20`
                    : isIuranReminder
                      ? '0 6px 20px -2px rgba(235, 94, 40, 0.25)'
                      : '0 6px 24px rgba(0, 0, 0, 0.4)',
                  transition: 'all 0.3s ease',
                }}
              >
                {/* 1. Background Layers */}
                {isIuranReminder ? (
                  <div
                    style={{
                      position: 'absolute',
                      inset: 0,
                      background: 'linear-gradient(135deg, rgba(235, 94, 40, 0.25) 0%, rgba(200, 70, 30, 0.4) 100%)',
                    }}
                  />
                ) : isColorBg ? (
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
                        background: `linear-gradient(${styleCfg?.gradient?.angle || 135}deg, ${customColor}28 0%, rgba(12, 9, 22, 0.95) 75%)`,
                      }}
                    />
                    {/* Layer 2: Primary broad diffused neon bloom at top-right */}
                    <div
                      style={{
                        position: 'absolute',
                        inset: 0,
                        background: `radial-gradient(circle at 75% 25%, ${customColor}99 0%, ${customColor}33 35%, transparent 70%)`,
                      }}
                    />
                    {/* Layer 3: Secondary subtle accent/reflection at bottom-left */}
                    <div
                      style={{
                        position: 'absolute',
                        inset: 0,
                        background: `radial-gradient(circle at 15% 85%, ${customColor}38 0%, transparent 55%)`,
                      }}
                    />
                    {/* Layer 4: Content contrast scrim */}
                    <div
                      style={{
                        position: 'absolute',
                        inset: 0,
                        background: 'linear-gradient(to top, rgba(0, 0, 0, 0.7) 0%, rgba(0, 0, 0, 0.2) 40%, transparent 100%)',
                      }}
                    />
                  </div>
                ) : isImageBg && imgUrl ? (
                  <div
                    style={{
                      position: 'absolute',
                      inset: 0,
                      background: 'linear-gradient(135deg, #2A124D 0%, #130924 100%)',
                    }}
                  >
                    <div style={{ position: 'absolute', inset: 0, overflow: 'hidden' }}>
                      {crop ? (
                        <CommunityImage
                          src={imgUrl}
                          alt={bannerAlt}
                          style={{
                            position: 'absolute',
                            width: `${(1 / (crop.width || 1)) * 100}%`,
                            height: `${(1 / (crop.height || 1)) * 100}%`,
                            left: `${-((crop.x || 0) / (crop.width || 1)) * 100}%`,
                            top: `${-((crop.y || 0) / (crop.height || 1)) * 100}%`,
                            maxWidth: 'none',
                            maxHeight: 'none',
                            objectFit: 'cover',
                          }}
                        />
                      ) : (
                        <CommunityImage
                          src={imgUrl}
                          alt={bannerAlt}
                          style={{
                            width: '100%',
                            height: '100%',
                            objectFit: 'cover',
                          }}
                        />
                      )}
                    </div>
                    {/* Soft dark gradient overlay for text readability */}
                    <div
                      style={{
                        position: 'absolute',
                        inset: 0,
                        background: `linear-gradient(to top, rgba(0, 0, 0, ${overlayOpacity}) 0%, rgba(0, 0, 0, ${overlayOpacity * 0.4}) 50%, rgba(0, 0, 0, 0) 100%)`,
                      }}
                    />
                  </div>
                ) : (
                  /* Legacy Fallback without image */
                  <div
                    style={{
                      position: 'absolute',
                      inset: 0,
                      background: 'linear-gradient(135deg, rgba(169, 112, 255, 0.25) 0%, rgba(87, 148, 242, 0.35) 100%)',
                    }}
                  />
                )}

                {/* 2. Banner Foreground Content */}
                <div
                  style={{
                    position: 'absolute',
                    inset: 0,
                    padding: '18px 24px',
                    display: 'flex',
                    flexDirection: 'column',
                    justifyContent: 'space-between',
                    zIndex: 2,
                    boxSizing: 'border-box',
                  }}
                >
                  {/* Top Bar: Badges */}
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span
                        style={{
                          fontSize: 11,
                          fontWeight: 700,
                          textTransform: 'uppercase',
                          letterSpacing: 0.5,
                          padding: '3px 8px',
                          borderRadius: 4,
                          background: isIuranReminder ? 'rgba(235, 94, 40, 0.3)' : 'rgba(0, 0, 0, 0.4)',
                          color: '#fff',
                          backdropFilter: 'blur(4px)',
                          border: isIuranReminder ? '1px solid rgba(235, 94, 40, 0.5)' : '1px solid rgba(255, 255, 255, 0.2)',
                        }}
                      >
                        {bannerLabel}
                      </span>
                      {banner.is_pinned && <Badge variant="amber">PENTING</Badge>}
                    </div>

                    {banner.deep_link && (
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 6,
                          fontSize: 12,
                          fontWeight: 600,
                          padding: '4px 10px',
                          borderRadius: 6,
                          background: 'rgba(0, 0, 0, 0.35)',
                          color: '#fff',
                          backdropFilter: 'blur(4px)',
                          border: '1px solid rgba(255, 255, 255, 0.2)',
                        }}
                      >
                        <span>{isIuranReminder ? 'Buka Keuangan' : 'Lihat Detail'}</span>
                        <RightOutlined style={{ fontSize: 10 }} />
                      </div>
                    )}
                  </div>

                  {/* Bottom Content Area: Title & optional Detail */}
                  <div style={{ marginTop: 'auto' }}>
                    <h3
                      style={{
                        margin: '0 0 4px 0',
                        fontSize: 20,
                        fontWeight: 700,
                        color: '#fff',
                        textShadow: '0 2px 4px rgba(0,0,0,0.85)',
                        whiteSpace: 'nowrap',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                      }}
                    >
                      {banner.title}
                    </h3>
                    {showDetail && banner.body && (
                      <p
                        style={{
                          margin: 0,
                          fontSize: 13,
                          color: 'rgba(255, 255, 255, 0.9)',
                          lineHeight: 1.45,
                          textShadow: '0 1px 3px rgba(0,0,0,0.85)',
                          display: '-webkit-box',
                          WebkitLineClamp: 2,
                          WebkitBoxOrient: 'vertical',
                          overflow: 'hidden',
                        }}
                      >
                        {banner.body}
                      </p>
                    )}
                  </div>
                </div>
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
        {/* Petugas Keamanan Sedang Bertugas */}
        <div className="sf-card" style={{ padding: 22 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
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
            <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>
              Petugas Keamanan Sedang Bertugas
            </h3>
          </div>

          {security?.is_available && (security.officers?.length ?? 0) > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {security.officers.map((officer) => {
                const name = officer.full_name ?? ''
                const phoneRow = officer.phone_number && officer.phone_number.trim() !== ''
                return (
                  <div
                    key={officer.user_id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 12,
                      padding: '10px 12px',
                      borderRadius: 10,
                      background: 'var(--sf-bg)',
                      border: '1px solid var(--sf-border)',
                    }}
                  >
                    <ResidentAvatar
                      name={name}
                      avatarPath={officer.avatar_path ?? null}
                      avatarThumbPath={officer.avatar_thumb_path ?? null}
                      size={44}
                    />
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 2, flex: 1, minWidth: 0 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
                        <IdcardOutlined style={{ fontSize: 12, color: 'var(--sf-text-muted)', flexShrink: 0 }} />
                        <span style={{ fontWeight: 600, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {name}
                        </span>
                      </div>
                      {phoneRow && (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <PhoneOutlined style={{ fontSize: 12, color: '#16A34A', flexShrink: 0 }} />
                          <span style={{ fontSize: 12, color: 'var(--sf-text-secondary, #94a3b8)', letterSpacing: '0.02em' }}>
                            {officer.phone_number}
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div
              style={{
                padding: '16px',
                borderRadius: 8,
                background: 'var(--sf-bg)',
                border: '1px dashed var(--sf-border)',
                fontWeight: 600,
                fontSize: 14,
                color: 'var(--sf-text)',
              }}
            >
              Tidak ada petugas keamanan yang sedang bertugas.
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
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, alignItems: 'center' }}>
            <style>{`
              @keyframes sfOrgFlowDash {
                to { stroke-dashoffset: -16; }
              }
              .sf-org-connector-line {
                animation: sfOrgFlowDash 1.2s linear infinite;
              }
              @media (prefers-reduced-motion: reduce) {
                .sf-org-connector-line {
                  animation: none !important;
                }
              }
            `}</style>
            {/* Group nodes by level — render every level the backend provides with animated connectors */}
            {[...new Set(orgNodes.map((n) => n.level))].sort((a, b) => a - b).map((lvl, idx) => {
              const nodesAtLevel = orgNodes.filter((n) => n.level === lvl)
              if (nodesAtLevel.length === 0) return null
              return (
                <div key={lvl} style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                  {idx > 0 && (
                    <div
                      style={{
                        display: 'flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                        margin: '2px 0 10px 0',
                        width: '100%',
                        maxWidth: 240,
                      }}
                    >
                      <svg
                        width="100%"
                        height="26"
                        viewBox="0 0 100 26"
                        preserveAspectRatio="none"
                        style={{ overflow: 'visible' }}
                      >
                        <line
                          x1="50"
                          y1="0"
                          x2="50"
                          y2="20"
                          stroke="var(--sf-accent)"
                          strokeWidth="2"
                          strokeDasharray="4 4"
                          className="sf-org-connector-line"
                        />
                        <polygon points="46,20 54,20 50,26" fill="var(--sf-accent)" />
                      </svg>
                    </div>
                  )}
                  <div
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
                      {node.members.length > 0 ? (
                        node.members.map((member) => {
                          const posLower = (member.jabatan ?? '').toLowerCase()
                          const isKetuaPos = posLower === 'ketua' || posLower === 'ketua_rt'
                          const isKeamananPos = ['keamanan', 'keamanan_rt', 'seksi_keamanan', 'seksi_keamanan_rt'].includes(posLower)
                          // Backend sends phone for Ketua RT and Keamanan positions to all authenticated RT members
                          const showPhone = isKetuaPos || isKeamananPos
                          return (
                          <div
                            key={member.user_id}
                            style={{
                              marginTop: 8,
                              display: 'flex',
                              alignItems: 'flex-start',
                              gap: 12,
                            }}
                          >
                            {/* Profile Avatar */}
                            <ResidentAvatar
                              name={member.full_name}
                              avatarThumbPath={member.avatar_url}
                              size={40}
                            />

                            {/* Member Info */}
                            <div style={{ flex: 1, minWidth: 0 }}>
                              {member.jabatan && (
                                <div style={{ marginBottom: 4 }}>
                                  <Badge variant={getJabatanBadgeVariant(member.jabatan)}>
                                    {member.jabatan.toUpperCase()}
                                  </Badge>
                                </div>
                              )}
                              <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--sf-text)' }}>
                                {member.full_name}
                              </div>
                              {member.house_number && (
                                <div
                                  style={{
                                    fontSize: 12,
                                    color: 'var(--sf-text-muted)',
                                    marginTop: 4,
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 4,
                                  }}
                                >
                                  <HomeOutlined style={{ fontSize: 11, color: 'var(--sf-accent)' }} />
                                  <span>{member.house_number}</span>
                                </div>
                              )}
                              {showPhone && member.phone && (
                                <div
                                  style={{
                                    fontSize: 12,
                                    color: 'var(--sf-text-muted)',
                                    marginTop: 4,
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 4,
                                  }}
                                >
                                  <PhoneOutlined style={{ fontSize: 11, color: '#16A34A' }} />
                                  <span>{member.phone}</span>
                                </div>
                              )}
                              {/* Divider selalu di bawah informasi terakhir (house_number/phone) */}
                              <div style={{ height: 1, background: 'var(--sf-border)', margin: '8px 0' }} />
                            </div>
                          </div>
                          )
                        })
                      ) : (
                        <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', fontStyle: 'italic' }}>
                          Belum ditentukan
                        </div>
                      )}
                    </div>
                  ))}
                </div>
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

      {/* Announcement Detail Modal from Banner */}
      {selectedAnnouncementBanner && (
        <div
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
          onClick={() => setSelectedAnnouncementBanner(null)}
        >
          <div
            className="sf-card"
            style={{
              width: '100%',
              maxWidth: 600,
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
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
              <div>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                  <Badge variant="blue">PENGUMUMAN</Badge>
                  {selectedAnnouncementBanner.is_pinned && <Badge variant="amber">PENTING</Badge>}
                </div>
                <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--sf-text)' }}>
                  {selectedAnnouncementBanner.title}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setSelectedAnnouncementBanner(null)}
                style={{ background: 'none', border: 'none', fontSize: 22, cursor: 'pointer', color: 'var(--sf-text-muted)' }}
              >
                <CloseOutlined style={{ fontSize: 18 }} />
              </button>
            </div>

            {selectedAnnouncementBanner.image_url && (
              <div
                style={{
                  width: '100%',
                  maxHeight: 280,
                  borderRadius: 8,
                  overflow: 'hidden',
                  background: 'var(--sf-surface-subtle)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  marginBottom: 16,
                  border: '1px solid var(--sf-border-subtle)',
                }}
              >
                <CommunityImage
                  src={selectedAnnouncementBanner.image_url}
                  alt={selectedAnnouncementBanner.title}
                  style={{ width: '100%', maxHeight: 280, objectFit: 'contain' }}
                />
              </div>
            )}

            {selectedAnnouncementBanner.body && (
              <div style={{ marginBottom: 16 }}>
                <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 4 }}>
                  ISI PENGUMUMAN
                </div>
                <p style={{ margin: 0, fontSize: 14, color: 'var(--sf-text)', lineHeight: 1.6, whiteSpace: 'pre-line' }}>
                  {selectedAnnouncementBanner.body}
                </p>
              </div>
            )}

            {selectedAnnouncementBanner.deep_link && selectedAnnouncementBanner.deep_link !== '/pengumuman' && (
              <div style={{ fontSize: 13, marginBottom: 16 }}>
                <button
                  type="button"
                  className="sf-btn sf-btn-outline"
                  onClick={() => {
                    const link = selectedAnnouncementBanner.deep_link!
                    setSelectedAnnouncementBanner(null)
                    navigate(link)
                  }}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
                >
                  <LinkOutlined />
                  <span>Buka Menu Terkait</span>
                </button>
              </div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', paddingTop: 16, borderTop: '1px solid var(--sf-border)' }}>
              <button
                type="button"
                className="sf-btn sf-btn-outline"
                onClick={() => setSelectedAnnouncementBanner(null)}
              >
                Tutup
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
