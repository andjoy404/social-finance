/**
 * RW Data View — wrapper component that provides the RW data navigation
 * and renders the appropriate RW data screen based on the current route.
 *
 * This component is mounted under /rw-view/:rwId/* routes.
 * It verifies RW membership using rw_memberships from the authenticated user.
 */

import { useState, useEffect, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth, hasRwWriteAccess, type RWMembership } from '@/app/AuthContext'
import { RwDashboard } from './RwDashboard'
import { RwKasList } from './RwKasList'
import { RwFundLocationList } from './RwFundLocationList'
import { LeftOutlined, DashboardOutlined, WalletOutlined, BankOutlined } from '@ant-design/icons'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/Badge'
import { Select } from 'antd'

type RwViewTab = 'dashboard' | 'kas' | 'lokasi-dana'

export function RwDataView({ rwId }: { rwId?: string }) {
  const { user } = useAuth()
  const params = useParams()
  const navigate = useNavigate()
  // Primary: URL param from useParams(). Fallback: prop (for tests/rendering).
  const urlRwId = params.rwId || rwId || ''
  const [selectedRwId, setSelectedRwId] = useState<string>('')
  const [selectedRwName, setSelectedRwName] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<RwViewTab>('dashboard')

  const isSuperAdmin = user?.role === 'super_admin' || (user as any)?.systemRole === 'super_admin' || (user as any)?.system_role === 'super_admin'
  const rawRwMemberships: RWMembership[] = user?.rwMemberships ?? []
  const rwMemberships = rawRwMemberships.filter(
    (m) => isSuperAdmin || (m.jabatan && ['ketua', 'wakil_ketua', 'sekretaris', 'bendahara'].includes(m.jabatan.toLowerCase()))
  )

  // Source-aware: RT inferred users cannot see the Lokasi Dana tab.
  const canWriteForSelected = hasRwWriteAccess(user, selectedRwId)
  const visibleTabs: { key: RwViewTab; label: string; icon: React.ReactNode }[] = useMemo(() => {
    const base: { key: RwViewTab; label: string; icon: React.ReactNode }[] = [
      {
        key: 'dashboard',
        label: 'BERANDA',
        icon: <DashboardOutlined />,
      },
      {
        key: 'kas',
        label: 'KAS',
        icon: <WalletOutlined />,
      },
    ]
    if (canWriteForSelected) {
      base.push({
        key: 'lokasi-dana',
        label: 'Lokasi Dana',
        icon: <BankOutlined />,
      })
    }
    return base
  }, [canWriteForSelected])
  const tabs = visibleTabs

  // Initialize selected RW from URL param or auto-select single membership
  useEffect(() => {
    if (!user) {
      setLoading(false)
      setError('User belum login')
      return
    }

    if (rwMemberships.length === 0) {
      setLoading(false)
      setError('Anda tidak memiliki akses ke RW mana pun')
      return
    }

    // Determine which RW to show
    let targetRwId = urlRwId
    let targetRwName: string | null = null

    if (targetRwId) {
      // Validate that the URL param RW is in user's memberships
      const membership = rwMemberships.find((m) => m.rw_id === targetRwId)
      if (!membership) {
        setLoading(false)
        setError('Anda tidak memiliki akses ke RW ini')
        return
      }
      targetRwName = membership.rw_name || null
    } else {
      // No URL param: auto-select if single membership
      if (rwMemberships.length === 1) {
        targetRwId = rwMemberships[0].rw_id
        targetRwName = rwMemberships[0].rw_name || null
        // Navigate to the RW URL
        navigate(`/rw-view/${targetRwId}/dashboard`, { replace: true })
      } else {
        // Multiple memberships: require explicit selection
        setLoading(false)
        setError('Silakan pilih RW yang ingin diakses')
        return
      }
    }

    setSelectedRwId(targetRwId)
    setSelectedRwName(targetRwName)
    setLoading(false)
    setError(null)
  }, [user, rwMemberships, urlRwId, navigate])

  const handleRwChange = (rwIdValue: string) => {
    const membership = rwMemberships.find((m) => m.rw_id === rwIdValue)
    if (!membership) return
    setSelectedRwId(rwIdValue)
    setSelectedRwName(membership.rw_name || null)
    setError(null)
    navigate(`/rw-view/${rwIdValue}/${activeTab}`, { replace: true })
  }

  const handleTabChange = (tab: RwViewTab) => {
    setActiveTab(tab)
    if (selectedRwId) {
      navigate(`/rw-view/${selectedRwId}/${tab}`, { replace: true })
    }
  }

  if (loading) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        Memuat data RW...
      </div>
    )
  }

  if (error || !selectedRwId) {
    return (
      <div style={{ padding: 24 }}>
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          padding: 16,
          borderRadius: 'var(--sf-radius-md)',
          background: 'rgba(255, 59, 48, 0.05)',
          border: '1px solid rgba(255, 59, 48, 0.2)',
          marginBottom: 16,
        }}>
          <span style={{ color: 'var(--sf-danger)' }}>
            <LeftOutlined style={{ fontSize: 16, cursor: 'pointer' }} />
          </span>
          <div>
            <div style={{ fontWeight: 600, color: 'var(--sf-danger)', marginBottom: 4 }}>{error ? 'Akses Ditolak' : 'Pilih RW'}</div>
            <div style={{ fontSize: 13, color: 'var(--sf-text)' }}>{error || 'Silakan pilih RW yang ingin diakses.'}</div>
          </div>
        </div>

        {rwMemberships.length > 0 && !error && (
          <div style={{ marginTop: 16 }}>
            <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 8 }}>
              Pilih RW:
            </div>
            {rwMemberships.map((m) => (
              <button
                key={m.rw_id}
                type="button"
                onClick={() => handleRwChange(m.rw_id)}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 8,
                  padding: '8px 16px',
                  marginRight: 8,
                  marginBottom: 8,
                  border: '1px solid var(--sf-border)',
                  borderRadius: 'var(--sf-radius-md)',
                  background: 'var(--sf-surface)',
                  color: 'var(--sf-text)',
                  cursor: 'pointer',
                  fontSize: 13,
                }}
              >
                {m.rw_name || `RW ${String(m.rw_number ?? '').padStart(3, '0')}`}
              </button>
            ))}
          </div>
        )}
      </div>
    )
  }

  const membership = rwMemberships.find((m) => m.rw_id === selectedRwId)
  const displayName = membership?.rw_name || selectedRwName || 'DATA RW'
  const jabatan = (membership?.jabatan || user?.jabatan || '').toLowerCase()
  // Source-aware: RT inferred users get read-only badge.
  // Checks the membership for the selected RW only.
  const canWrite = hasRwWriteAccess(user, selectedRwId)
  const canCreate = isSuperAdmin || (canWrite && ['ketua', 'bendahara', 'wakil_ketua'].includes(jabatan))
  const canUpdate = isSuperAdmin || (canWrite && ['ketua', 'bendahara'].includes(jabatan))
  const isReadOnly = !canCreate && !canUpdate

  return (
    <div>
      {/* RW Selector (when multiple memberships) */}
      {rwMemberships.length > 1 && (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '12px 16px',
          marginBottom: 16,
          borderRadius: 'var(--sf-radius-md)',
          background: 'rgba(169, 112, 255, 0.06)',
          border: '1px solid rgba(169, 112, 255, 0.2)',
        }}>
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--sf-text-muted)' }}>
            Konteks RW:
          </span>
          <Select
            defaultValue={selectedRwId}
            onChange={handleRwChange}
            style={{ width: 260 }}
            dropdownMatchSelectWidth={false}
          >
            {rwMemberships.map((m) => (
              <Select.Option key={m.rw_id} value={m.rw_id}>
                {m.rw_name || `RW ${String(m.rw_number ?? '').padStart(3, '0')}`} {m.jabatan ? `(${m.jabatan})` : ''}
              </Select.Option>
            ))}
          </Select>
        </div>
      )}

      {/* RW Data View Header */}
      <PageHeader
        surface
        icon={<BankOutlined />}
        title={`Keuangan ${displayName}`}
        subtitle="Pencatatan kas dan lokasi dana mandiri tingkat RW"
        actions={
          isReadOnly ? (
            <Badge variant="blue">
              Mode Read-Only
            </Badge>
          ) : undefined
        }
      />

      {/* RW View Tabs */}
      <div style={{
        display: 'flex',
        gap: 4,
        margin: '16px 0 0 0',
        borderBottom: '1px solid var(--sf-border)',
        paddingBottom: 0,
      }}>
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => handleTabChange(tab.key)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              padding: '10px 16px',
              border: 'none',
              borderBottom: tab.key === activeTab ? '2px solid var(--sf-accent)' : '2px solid transparent',
              background: tab.key === activeTab ? 'var(--sf-surface-hover)' : 'transparent',
              color: tab.key === activeTab ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
              cursor: 'pointer',
              fontSize: 13,
              fontWeight: tab.key === activeTab ? 600 : 400,
              borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
              transition: 'all 0.2s ease',
            }}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* RW View Content */}
      <div style={{ padding: 16 }}>
        {activeTab === 'dashboard' && (
          <RwDashboard
            rwId={selectedRwId}
            rwName={displayName || undefined}
            onNavigateToKas={() => handleTabChange('kas')}
          />
        )}
        {activeTab === 'kas' && <RwKasList rwId={selectedRwId} rwName={displayName || undefined} />}
        {activeTab === 'lokasi-dana' && <RwFundLocationList rwId={selectedRwId} rwName={displayName || undefined} />}
      </div>
    </div>
  )
}
