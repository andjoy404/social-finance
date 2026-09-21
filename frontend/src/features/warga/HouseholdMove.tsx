import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  getHousehold,
  moveHousehold,
  ApiError as ApiErrorType,
  clearSessionPair,
  getSessionPair,
  type ApiMoveHouseholdBody,
  type ApiHousehold,
} from '@/app/api'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { AppCard } from '@/components/AppCard'

type OccupancyStatus = 'OWNER' | 'TENANT'

export function HouseholdMove() {
  const { user } = useAuth()
  if (!user) return null

  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  if (!id) {
    navigate('/warga', { replace: true })
    return null
  }

  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<ApiErrorType | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [form, setForm] = useState({
    house_number: '',
    address: '',
    occupancy_status: 'OWNER' as OccupancyStatus,
    start_date: '',
  })
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const fetchHousehold = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError(new ApiErrorType('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
      navigate('/login')
      return
    }

    setLoading(true)
    setError(null)
    setNotFound(false)

    try {
      const data: ApiHousehold = await getHousehold(pair.accessToken, id)

      setForm({
        house_number: data.house_number ?? '',
        address: data.address ?? '',
        occupancy_status: (data.occupancy_status as OccupancyStatus) || 'OWNER',
        start_date: '',
      })
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiErrorType && err.code === 'not_found') {
        setNotFound(true)
      } else if (err instanceof ApiErrorType) {
        setError(new ApiErrorType(err.code, err.message || 'Gagal memuat data.'))
      } else {
        setError(new ApiErrorType('unexpected', 'Gagal memuat data.'))
      }
    } finally {
      setLoading(false)
    }
  }, [id, navigate])

  useEffect(() => {
    void fetchHousehold()
  }, [fetchHousehold])

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const validate = (): boolean => {
    const errs: string[] = []
    if (!form.house_number || !form.house_number.trim()) errs.push('Nomor rumah baru harus diisi.')
    if (!form.address || !form.address.trim()) errs.push('Alamat baru harus diisi.')
    if (!form.occupancy_status) errs.push('Status hunian harus dipilih.')
    if (!form.start_date) errs.push('Tanggal mulai hunian baru harus diisi.')
    if (errs.length > 0) {
      setValidationErrors(errs)
      return false
    }
    return true
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])
    setError(null)
    if (!validate()) return

    setSaving(true)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) {
        setError(new ApiErrorType('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
        navigate('/login')
        return
      }

      const body: ApiMoveHouseholdBody = {
        house_number: form.house_number.trim(),
        address: form.address.trim() || null,
        occupancy_status: form.occupancy_status,
        start_date: form.start_date,
      }

      await moveHousehold(pair.accessToken, id, body)
      navigate('/warga/' + id)
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiErrorType) {
        setError(new ApiErrorType(err.code, err.message || 'Gagal memindahkan rumah tangga.'))
      } else {
        setError(new ApiErrorType('unexpected', 'Gagal memindahkan rumah tangga.'))
      }
    } finally {
      setSaving(false)
    }
  }

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '8px 10px',
    borderRadius: 'var(--radius-sm)',
    border: '1px solid var(--border-color)',
    fontSize: '13px',
    color: 'var(--text-primary)',
    background: 'var(--bg-primary)',
  }

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: '13px',
    fontWeight: 500,
    color: 'var(--text-secondary)',
    marginBottom: 'var(--sp-xs)',
  }

  const fieldGap: React.CSSProperties = { marginBottom: 'var(--sp-md)' }

  const selectStyle: React.CSSProperties = {
    ...inputStyle,
    appearance: 'auto',
  }

  if (loading) {
    return (
      <div style={{ flex: 1, padding: 'var(--sp-md) var(--sp-xl)' }}>
        <AppCard>
          <div style={{
            display: 'flex', justifyContent: 'center', padding: 'var(--sp-xl)',
            color: 'var(--text-muted)', fontSize: '13px',
          }}>
            Memuat data rumah tangga...
          </div>
        </AppCard>
      </div>
    )
  }

  if (notFound) {
    return (
      <div style={{ flex: 1, padding: 'var(--sp-md) var(--sp-xl)' }}>
        <div style={{ marginBottom: 'var(--sp-md)' }}>
          <Link to="/warga" style={{
            fontSize: '13px', color: 'var(--primary)', display: 'inline-flex', alignItems: 'center', gap: 'var(--sp-xs)',
          }}>
            ← Kembali ke Daftar Warga
          </Link>
        </div>
        <AppCard>
          <div style={{
            textAlign: 'center', padding: 'var(--sp-xl) 0', color: 'var(--text-muted)', fontSize: '13px',
          }}>
            Rumah tangga tidak ditemukan.
          </div>
        </AppCard>
      </div>
    )
  }

  return (
    <div style={{ flex: 1, padding: 'var(--sp-md) var(--sp-xl)' }}>
      {/* Header */}
      <div style={{ marginBottom: 'var(--sp-lg)' }}>
        <Link to="/warga" style={{
          fontSize: '13px', color: 'var(--primary)', display: 'inline-flex', alignItems: 'center', gap: 'var(--sp-xs)', marginBottom: 'var(--sp-md)',
        }}>
          ← Kembali ke Daftar Warga
        </Link>
        <h2 style={{
          fontSize: '16px', fontWeight: 700, color: 'var(--text-primary)', margin: 0, lineHeight: 1.3,
        }}>
          Pindah Rumah Tangga
        </h2>
        <p style={{
          fontSize: '12px', color: 'var(--text-secondary)', margin: '2px 0 0',
        }}>
          Pindahkan rumah tangga ke alamat dan nomor rumah baru. Tindakan ini akan membuat periode hunian baru tanpa menghapus riwayat sebelumnya.
        </p>
      </div>

      {/* Errors */}
      {validationErrors.length > 0 && (
        <AppCard style={{ marginBottom: 'var(--sp-md)' }}>
          <div style={{
            background: 'var(--color-expense-subtle)',
            color: 'var(--color-expense)',
            border: '1px solid rgba(255,59,48,0.3)',
            borderRadius: 'var(--radius-sm)',
            padding: '10px 14px',
            fontSize: '13px',
          }}>
            <ul style={{ margin: 0, paddingLeft: 'var(--sp-md)' }}>
              {validationErrors.map((e, i) => <li key={i}>{e}</li>)}
            </ul>
          </div>
        </AppCard>
      )}

      {error && (
        <AppCard style={{ marginBottom: 'var(--sp-md)' }}>
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

      {/* Form */}
      <AppCard>
        <form onSubmit={handleSubmit}>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
            gap: 'var(--sp-md)',
          }}>
            {/* House number - required */}
            <div style={fieldGap}>
              <label style={labelStyle} htmlFor="move-house-number">
                Nomor Rumah Baru <span style={{ color: 'var(--color-expense)' }}>*</span>
              </label>
              <input
                id="move-house-number"
                type="text"
                value={form.house_number}
                onChange={(e) => handleChange('house_number', e.target.value)}
                placeholder="Contoh: 002"
                required
                disabled={saving}
                style={inputStyle}
              />
            </div>

            {/* Occupancy status - required */}
            <div style={fieldGap}>
              <label style={labelStyle} htmlFor="move-occupancy-status">
                Status Hunian <span style={{ color: 'var(--color-expense)' }}>*</span>
              </label>
              <select
                id="move-occupancy-status"
                value={form.occupancy_status}
                onChange={(e) => handleChange('occupancy_status', e.target.value)}
                disabled={saving}
                style={selectStyle}
              >
                <option value="OWNER">Pemilik</option>
                <option value="TENANT">Penyewa</option>
              </select>
            </div>

            {/* Start date - required */}
            <div style={fieldGap}>
              <label style={labelStyle} htmlFor="move-start-date">
                Tanggal Mulai Hunian Baru <span style={{ color: 'var(--color-expense)' }}>*</span>
              </label>
              <input
                id="move-start-date"
                type="date"
                value={form.start_date}
                onChange={(e) => handleChange('start_date', e.target.value)}
                required
                disabled={saving}
                style={inputStyle}
              />
            </div>
          </div>

          {/* Address */}
          <div style={{
            marginTop: 'var(--sp-sm)',
          }}>
            <div style={fieldGap}>
              <label style={labelStyle} htmlFor="move-address">
                Alamat Baru <span style={{ color: 'var(--color-expense)' }}>*</span>
              </label>
              <input
                id="move-address"
                type="text"
                value={form.address}
                onChange={(e) => handleChange('address', e.target.value)}
                placeholder="Alamat lengkap baru"
                required
                disabled={saving}
                style={{ ...inputStyle, gridColumn: '1 / -1' }}
              />
            </div>
          </div>

          {/* Actions */}
          <div style={{
            display: 'flex', gap: 'var(--sp-sm)', marginTop: 'var(--sp-lg)',
          }}>
            <button
              type="submit"
              disabled={saving}
              style={{
                padding: '8px 20px',
                background: 'var(--primary)',
                color: '#fff',
                border: 'none',
                borderRadius: 'var(--radius-sm)',
                fontSize: '13px',
                fontWeight: 600,
                cursor: saving ? 'not-allowed' : 'pointer',
                opacity: saving ? 0.7 : 1,
              }}
            >
              {saving ? 'Memindahkan...' : 'Pindahkan'}
            </button>
            <Link
              to="/warga"
              style={{
                padding: '8px 20px',
                background: 'transparent',
                color: 'var(--text-secondary)',
                border: '1px solid var(--border-color)',
                borderRadius: 'var(--radius-sm)',
                fontSize: '13px',
                textDecoration: 'none',
              }}
            >
              Batal
            </Link>
          </div>
        </form>
      </AppCard>
    </div>
  )
}
