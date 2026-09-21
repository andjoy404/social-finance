import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import { apiGetRT, apiDeactivateRT, apiReactivateRT, ApiError, clearSessionPair, getSessionPair, type ApiRT } from '@/app/api'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { AppCard } from '@/components/AppCard'
import { Badge } from '@/components/Badge'

function statusBadge(value: boolean) {
  return value ? 'green' : 'red'
}

function statusLabel(value: boolean) {
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

function formatNull(value: string | null) {
  return value ?? '\u2014'
}

export function RtDetail() {
  const { user } = useAuth()
  if (!user) return null

  const { id } = useParams<{ id: string }>()
  if (!id) {
    return <div style={{ flex: 1, padding: 'var(--sp-md) var(--sp-xl)' }}>ID tidak valid</div>
  }

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [rt, setRt] = useState<ApiRT | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [actionProgress, setActionProgress] = useState<string | null>(null)
  const navigate = useNavigate()

  const fetchRt = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError(new ApiError('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
      return
    }

    setLoading(true)
    setError(null)
    setNotFound(false)

    try {
      const data = await apiGetRT(pair.accessToken, id)
      setRt(data)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        window.location.href = '/login'
      } else if (err instanceof ApiError && (err.code === 'not_found' || err.code === 'unauthorized')) {
        setNotFound(true)
      } else if (err instanceof ApiError && (err.code === 'forbidden' || err.code === 'unauthorized')) {
        setError(new ApiError('access_denied', 'Anda tidak memiliki akses ke halaman ini.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal memuat detail RT.'))
      }
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    void fetchRt()
  }, [fetchRt])

  const handleDeactivate = async () => {
    if (!rt) return
    setActionProgress('deactivate')
    setError(null)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) {
        clearSessionPair()
        navigate('/login')
        return
      }
      await apiDeactivateRT(pair.accessToken, rt.id)
      setRt(prev => prev ? { ...prev, is_active: false } : null)
    } catch (err) {
      if (err instanceof ApiError && (err.code === 'auth_expired' || err.code === 'unauthorized')) {
        if (err.code === 'auth_expired') clearSessionPair()
        navigate('/login')
      } else {
        setError(new ApiError('unexpected', 'Gagal menonaktifkan RT.'))
      }
    } finally {
      setActionProgress(null)
    }
  }

  const handleReactivate = async () => {
    if (!rt) return
    setActionProgress('reactivate')
    setError(null)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) {
        clearSessionPair()
        navigate('/login')
        return
      }
      const updated = await apiReactivateRT(pair.accessToken, rt.id)
      setRt(updated)
    } catch (err) {
      if (err instanceof ApiError && (err.code === 'auth_expired' || err.code === 'unauthorized')) {
        if (err.code === 'auth_expired') clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiError && err.code === 'not_found') {
        setNotFound(true)
      } else {
        setError(new ApiError('unexpected', 'Gagal mengaktifkan kembali RT.'))
      }
    } finally {
      setActionProgress(null)
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
          to="/rt"
          style={{
            fontSize: '13px',
            color: 'var(--primary)',
            display: 'inline-flex',
            alignItems: 'center',
            gap: 'var(--sp-xs)',
          }}
          onClick={navigateBack}
        >
          ← Kembali ke Daftar RT
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
            Memuat detail RT...
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
            RT tidak ditemukan.
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

      {/* RT Detail */}
      {!loading && !notFound && !error && rt && (
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
                {rt.name}
              </h2>
              <p style={{
                fontSize: '12px',
                color: 'var(--text-secondary)',
                margin: '2px 0 0',
              }}>
                Detail administratif RT
              </p>
            </div>
            <Badge variant={statusBadge(rt.is_active)}>
              {statusLabel(rt.is_active)}
            </Badge>
          </div>

          {/* Actions */}
          <div style={{
            display: 'flex',
            gap: 'var(--sp-sm)',
            marginTop: 'var(--sp-md)',
            paddingTop: 'var(--sp-md)',
            borderTop: '1px solid var(--border-subtle)',
            flexWrap: 'wrap',
          }}>
            <Link
              to={'/rt/' + rt.id + '/edit'}
              style={{
                padding: '6px 14px',
                fontSize: '12px',
                fontWeight: 600,
                color: 'var(--text-primary)',
                background: 'var(--bg-primary)',
                border: '1px solid var(--border-color)',
                borderRadius: 'var(--radius-sm)',
                cursor: 'pointer',
                textDecoration: 'none',
                whiteSpace: 'nowrap',
              }}
            >
              Edit RT
            </Link>
            {rt.is_active ? (
              <button
                onClick={handleDeactivate}
                disabled={actionProgress === 'deactivate'}
                style={{
                  padding: '6px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: 'var(--color-expense)',
                  background: 'var(--color-expense-subtle)',
                  border: '1px solid rgba(255,59,48,0.3)',
                  borderRadius: 'var(--radius-sm)',
                  cursor: actionProgress === 'deactivate' ? 'not-allowed' : 'pointer',
                  whiteSpace: 'nowrap',
                  opacity: actionProgress === 'deactivate' ? 0.7 : 1,
                }}
              >
                {actionProgress === 'deactivate' ? 'Menonaktifkan...' : 'Nonaktifkan RT'}
              </button>
            ) : (
              <button
                onClick={handleReactivate}
                disabled={actionProgress === 'reactivate'}
                style={{
                  padding: '6px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: 'var(--success)',
                  background: 'var(--success-subtle)',
                  border: '1px solid rgba(52,199,89,0.3)',
                  borderRadius: 'var(--radius-sm)',
                  cursor: actionProgress === 'reactivate' ? 'not-allowed' : 'pointer',
                  whiteSpace: 'nowrap',
                  opacity: actionProgress === 'reactivate' ? 0.7 : 1,
                }}
              >
                {actionProgress === 'reactivate' ? 'Mengaktifkan...' : 'Aktifkan Kembali RT'}
              </button>
            )}
          </div>

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
                RT
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
                fontWeight: 500,
                fontFamily: 'monospace',
              }}>
                {rt.rt}
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
                RW
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
                fontWeight: 500,
              }}>
                {rt.rw}
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
                Ketua RT
              </div>
              <div style={{
                fontSize: '13px',
                color: 'var(--text-primary)',
              }}>
                {formatNull(rt.head_name)}
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
                {formatNull(rt.address)}
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
                {formatDate(rt.created_at)}
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
                {formatDate(rt.updated_at)}
              </div>
            </div>
          </div>
        </AppCard>
      )}
    </div>
  )
}
