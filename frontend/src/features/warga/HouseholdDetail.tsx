import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  getHousehold,
  deactivateHousehold,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiHousehold,
} from '@/app/api'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { AppCard } from '@/components/AppCard'
import { Badge } from '@/components/Badge'

function occupancyLabel(status: ApiHousehold['occupancy_status']): string {
  switch (status) {
    case 'OWNER':
      return 'Pemilik'
    case 'TENANT':
      return 'Penyewa'
    default:
      return '\u2014'
  }
}

function occupancyBadgeVariant(status: ApiHousehold['occupancy_status']): 'violet' | 'blue' | 'default' {
  switch (status) {
    case 'OWNER':
      return 'violet'
    case 'TENANT':
      return 'blue'
    default:
      return 'default'
  }
}

function statusBadge(value: boolean): 'green' | 'red' {
  return value ? 'green' : 'red'
}

function statusLabel(value: boolean): string {
  return value ? 'Aktif' : 'Tidak Aktif'
}

function formatDate(iso: string) {
  const d = new Date(iso)
  return d.toLocaleString('id-ID', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatNull(value: string | null): string {
  return value ?? '\u2014'
}

export function HouseholdDetail() {
  const { user } = useAuth()
  if (!user) return null

  const { id } = useParams<{ id: string }>()
  if (!id) {
    return <div style={{ flex: 1, padding: 'var(--sp-md) var(--sp-xl)' }}>ID tidak valid</div>
  }

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [household, setHousehold] = useState<ApiHousehold | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [confirmDeactivate, setConfirmDeactivate] = useState(false)
  const [deactivating, setDeactivating] = useState(false)
  const navigate = useNavigate()

  const fetchHousehold = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError(new ApiError('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
      return
    }

    setLoading(true)
    setError(null)
    setNotFound(false)

    try {
      const data = await getHousehold(pair.accessToken, id)
      setHousehold(data)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        window.location.href = '/login'
      } else if (err instanceof ApiError && (err.code === 'not_found' || err.code === 'unauthorized')) {
        setNotFound(true)
      } else if (err instanceof ApiError && err.code === 'forbidden') {
        setError(new ApiError('access_denied', 'Anda tidak memiliki akses ke halaman ini.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal memuat detail warga.'))
      }
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    void fetchHousehold()
  }, [fetchHousehold])

  const handleDeactivate = async () => {
    if (!household) return
    setDeactivating(true)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) {
        setError(new ApiError('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
        navigate('/login')
        return
      }
      await deactivateHousehold(pair.accessToken, id)
      navigate('/warga')
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiError) {
        setError(new ApiError(err.code, err.message || 'Gagal menonaktifkan rumah tangga.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal menonaktifkan rumah tangga.'))
      }
    } finally {
      setDeactivating(false)
      setConfirmDeactivate(false)
    }
  }

  const navigateBack = () => {
    window.history.back()
  }

  return (
    <div style={{ flex: 1, padding: 'var(--sp-md) var(--sp-xl)' }}>
      {/* Back button */}
      <div style={{ marginBottom: 'var(--sp-md)' }}>
        <Link
          to="/warga"
          style={{
            fontSize: '13px',
            color: 'var(--primary)',
            display: 'inline-flex',
            alignItems: 'center',
            gap: 'var(--sp-xs)',
          }}
          onClick={navigateBack}
        >
          ← Kembali ke Daftar Warga
        </Link>
      </div>

      {/* Loading */}
      {loading && (
        <AppCard>
          <div style={{
            display: 'flex',
            justifyContent: 'center',
            padding: 'var(--sp-xl)',
            color: 'var(--text-muted)',
            fontSize: '13px',
          }}>
            Memuat detail warga...
          </div>
        </AppCard>
      )}

      {/* 404 */}
      {!loading && notFound && (
        <AppCard>
          <div style={{
            textAlign: 'center',
            padding: 'var(--sp-xl) 0',
            color: 'var(--text-muted)',
            fontSize: '13px',
          }}>
            Warga tidak ditemukan.
          </div>
        </AppCard>
      )}

      {/* Error */}
      {!loading && !notFound && error && (
        <AppCard>
          <div style={{
            background: 'var(--color-expense-subtle)',
            color: 'var(--color-expense)',
            border: '1px solid rgba(255,59,48,0.3)',
            borderRadius: 'var(--radius-sm)',
            padding: '10px 14px',
            fontSize: '13px',
          }}>
            {error.message}
          </div>
        </AppCard>
      )}

      {/* Household Detail */}
      {!loading && !notFound && !error && household && (
        <AppCard>
          {/* Header */}
          <div style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            marginBottom: 'var(--sp-lg)',
            gap: 'var(--sp-md)',
            flexWrap: 'wrap',
          }}>
            <div>
              <h2 style={{
                fontSize: '16px',
                fontWeight: 700,
                color: 'var(--text-primary)',
                margin: 0,
                lineHeight: 1.3,
              }}>
                {household.head_name}
              </h2>
              <p style={{
                fontSize: '12px',
                color: 'var(--text-secondary)',
                margin: '2px 0 0',
              }}>
                Detail rumah tangga
              </p>
            </div>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <Badge variant={occupancyBadgeVariant(household.occupancy_status)}>
                {occupancyLabel(household.occupancy_status)}
              </Badge>
              <Badge variant={statusBadge(household.is_active)}>
                {statusLabel(household.is_active)}
              </Badge>
            </div>
          </div>

          {/* Actions */}
          <div style={{
            display: 'flex',
            gap: '8px',
            flexWrap: 'wrap',
            marginBottom: 'var(--sp-md)',
            paddingBottom: 'var(--sp-md)',
            borderBottom: '1px solid var(--border-subtle)',
          }}>
            {household.is_active && (
              <>
                <Link
                  to={`/warga/${household.id}/edit`}
                  style={{
                    padding: '6px 14px',
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--text-primary)',
                    background: 'var(--bg-primary)',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-sm)',
                    textDecoration: 'none',
                    cursor: 'pointer',
                  }}
                >
                  Edit
                </Link>
                <Link
                  to={`/warga/${household.id}/pindah`}
                  style={{
                    padding: '6px 14px',
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--text-primary)',
                    background: 'var(--bg-primary)',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-sm)',
                    textDecoration: 'none',
                    cursor: 'pointer',
                  }}
                >
                  Pindah
                </Link>
                <button
                  onClick={() => setConfirmDeactivate(true)}
                  style={{
                    padding: '6px 14px',
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--color-expense)',
                    background: 'transparent',
                    border: '1px solid var(--color-expense)',
                    borderRadius: 'var(--radius-sm)',
                    cursor: 'pointer',
                  }}
                >
                  Nonaktifkan
                </button>
              </>
            )}
          </div>

          {/* Deactivate confirmation modal */}
          {confirmDeactivate && (
            <div style={{
              background: 'var(--color-expense-subtle)',
              border: '1px solid rgba(255,59,48,0.3)',
              borderRadius: 'var(--radius-sm)',
              padding: '16px',
              marginBottom: 'var(--sp-md)',
            }}>
              <p style={{
                fontSize: '13px',
                color: 'var(--color-expense)',
                fontWeight: 600,
                margin: '0 0 8px',
              }}>
                Nonaktifkan rumah tangga {household.house_number} — {household.head_name}?
              </p>
              <p style={{
                fontSize: '12px',
                color: 'var(--color-expense)',
                margin: '0 0 12px',
                opacity: 0.8,
              }}>
                Rumah tangga ini akan ditandai tidak aktif dan tidak akan muncul di daftar aktif.
              </p>
              <div style={{ display: 'flex', gap: '8px' }}>
                <button
                  onClick={handleDeactivate}
                  disabled={deactivating}
                  style={{
                    padding: '6px 14px',
                    fontSize: '12px',
                    fontWeight: 600,
                    color: '#fff',
                    background: 'var(--color-expense)',
                    border: 'none',
                    borderRadius: 'var(--radius-sm)',
                    cursor: deactivating ? 'not-allowed' : 'pointer',
                    opacity: deactivating ? 0.7 : 1,
                  }}
                >
                  {deactivating ? 'Menonaktifkan...' : 'Ya, Nonaktifkan'}
                </button>
                <button
                  onClick={() => setConfirmDeactivate(false)}
                  disabled={deactivating}
                  style={{
                    padding: '6px 14px',
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--text-secondary)',
                    background: 'transparent',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-sm)',
                    cursor: deactivating ? 'not-allowed' : 'pointer',
                  }}
                >
                  Batal
                </button>
              </div>
            </div>
          )}

          {/* Info grid */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
            gap: 'var(--sp-md)',
          }}>
            <div>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Nomor Rumah
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
                fontWeight: 500,
                fontFamily: 'monospace',
              }}>
                {formatNull(household.house_number)}
              </div>
            </div>
            <div>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Kepala Keluarga
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
              }}>
                {household.head_name}
              </div>
            </div>
            <div>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Telepon
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
              }}>
                {formatNull(household.phone ?? household.head_resident?.phone ?? null)}
              </div>
            </div>
            <div>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Email
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
              }}>
                {formatNull(household.email ?? household.head_resident?.email ?? null)}
              </div>
            </div>
            <div>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Status Hunian
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
              }}>
                {occupancyLabel(household.occupancy_status)}
              </div>
            </div>
            <div style={{ gridColumn: '1 / -1' }}>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Alamat
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
              }}>
                {formatNull(household.address)}
              </div>
            </div>
          </div>

          {/* Meta */}
          <div style={{
            borderTop: '1px solid var(--border-subtle)',
            marginTop: 'var(--sp-md)',
            paddingTop: 'var(--sp-md)',
            display: 'flex',
            gap: 'var(--sp-lg)',
            flexWrap: 'wrap',
          }}>
            <div>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Dibuat
              </div>
              <div style={{
                fontSize: '12px',
                color: 'var(--text-secondary)',
              }}>
                {formatDate(household.created_at)}
              </div>
            </div>
            <div>
              <div style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                marginBottom: '2px',
                fontWeight: 500,
                textTransform: 'uppercase',
                letterSpacing: '0.03em',
              }}>
                Diperbarui
              </div>
              <div style={{
                fontSize: '12px',
                color: 'var(--text-secondary)',
              }}>
                {formatDate(household.updated_at)}
              </div>
            </div>
          </div>
        </AppCard>
      )}
    </div>
  )
}
