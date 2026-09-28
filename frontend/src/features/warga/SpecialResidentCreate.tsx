import { useState, useEffect } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  createSpecialResident,
  apiListRTs,
  ApiError as ApiErrorType,
  clearSessionPair,
  getSessionPair,
  type ApiRT,
} from '@/app/api'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { SearchableSelect } from '@/components/SearchableSelect'
import {
  IdcardOutlined,
  MailOutlined,
  PhoneOutlined,
  UserOutlined,
} from '@ant-design/icons'

type JabatanValue = 'keamanan' | 'kebersihan_pembangunan'

const jabatanOptions: { value: JabatanValue; label: string }[] = [
  { value: 'keamanan', label: 'Keamanan' },
  { value: 'kebersihan_pembangunan', label: 'Kebersihan & Pembangunan' },
]

interface SpecialResidentFormState {
  jabatan: JabatanValue
  full_name: string
  nik: string
  phone: string
  email: string
  is_active: boolean
}

const emptyForm: SpecialResidentFormState = {
  jabatan: 'keamanan',
  full_name: '',
  nik: '',
  phone: '',
  email: '',
  is_active: true,
}

function isSuperAdmin(systemRole: string | null): boolean {
  return systemRole === 'super_admin'
}

export function SpecialResidentCreate() {
  const { user } = useAuth()
  const navigate = useNavigate()
  if (!user) return null

  const adminMode = isSuperAdmin(user.systemRole)

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<ApiErrorType | null>(null)
  const [form, setForm] = useState<SpecialResidentFormState>(emptyForm)
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  // SUPER_ADMIN RT selector state
  const [rtOptions, setRtOptions] = useState<ApiRT[]>([])
  const [selectedRtId, setSelectedRtId] = useState<string>('')
  const [rtLoading, setRtLoading] = useState(false)
  const [rtError, setRtError] = useState<ApiErrorType | null>(null)

  useEffect(() => {
    if (!adminMode) return

    let cancelled = false
    ;(async () => {
      setRtLoading(true)
      setRtError(null)
      try {
        const pair = getSessionPair()
        if (!pair?.accessToken) { navigate('/login'); return }
        const allRts: ApiRT[] = []
        let page = 1
        while (true) {
          const resp = await apiListRTs(pair.accessToken, { page, page_size: 100, is_active: true })
          if (!cancelled) {
            allRts.push(...resp.data)
            if (page >= resp.pagination.total_pages) break
            page++
          }
        }
        if (!cancelled) {
          setRtOptions(allRts)
          setRtLoading(false)
        }
      } catch (err) {
        if (!cancelled) {
          setRtError(err instanceof ApiErrorType ? err : new ApiErrorType('unexpected', 'Gagal memuat daftar RT.'))
          setRtLoading(false)
        }
      }
    })()
    return () => { cancelled = true }
  }, [adminMode, navigate])

  const handleClose = () => navigate('/warga')

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])
    setError(null)

    const errs: string[] = []
    if (!form.full_name.trim()) errs.push('Nama harus diisi.')
    if (!form.nik.trim()) {
      errs.push('NIK harus diisi.')
    } else if (!/^[0-9]{16}$/.test(form.nik.trim())) {
      errs.push('NIK harus berupa 16 digit angka.')
    }
    if (!form.phone.trim()) errs.push('Telepon harus diisi.')
    if (!form.email.trim()) {
      errs.push('Email harus diisi.')
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
      errs.push('Format email tidak valid.')
    }
    if (adminMode && !selectedRtId) errs.push('RT Tujuan harus dipilih.')
    if (errs.length > 0) { setValidationErrors(errs); return }

    setLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) { navigate('/login'); return }

      const body = {
        rt_id: adminMode ? selectedRtId : user.rt?.id ?? '',
        jabatan: form.jabatan,
        full_name: form.full_name.trim(),
        nik: form.nik.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
      }

      await createSpecialResident(pair.accessToken, body)
      // Navigate back with refresh param; if on /warga/baru (child route),
      // navigate to parent /warga instead.
      if (window.location.pathname === '/warga/baru') {
        navigate('/warga?refresh=1')
      } else {
        navigate('/warga')
      }
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiErrorType) {
        setError(new ApiErrorType(err.code, err.message || 'Gagal menambah petugas khusus.'))
      } else {
        setError(new ApiErrorType('unexpected', 'Gagal menambah petugas khusus.'))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title="Tambah Petugas Khusus"
        subtitle="Data petugas keamanan / kebersihan pembangunan"
        onClose={handleClose}
      />
      <form onSubmit={handleSubmit}>
        <ModalBody>
          {validationErrors.length > 0 && (
            <div className="sf-form-error-list">
              <ul>{validationErrors.map((e, i) => <li key={i}>{e}</li>)}</ul>
            </div>
          )}
          {error && <div className="sf-form-error-list">{error.message}</div>}
          {rtError && <div className="sf-form-error-list">{rtError.message}</div>}

          {/* Jenis Petugas */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-jabatan">
              Jenis Petugas <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <select
              id="sr-jabatan"
              className="sf-form-select"
              value={form.jabatan}
              onChange={(e) => handleChange('jabatan', e.target.value)}
              disabled={loading}
            >
              {jabatanOptions.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>

          {/* Nama */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-name">
              <UserOutlined style={{ marginRight: 4 }} />
              Nama <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="sr-name"
              className="sf-form-input"
              type="text"
              value={form.full_name}
              onChange={(e) => handleChange('full_name', e.target.value)}
              placeholder="Nama lengkap"
              required
              disabled={loading}
              autoFocus
            />
          </div>

          {/* NIK + Telepon */}
          <div className="sf-form-grid-2">
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-nik">
                <IdcardOutlined style={{ marginRight: 4 }} />
                NIK <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="sr-nik"
                className="sf-form-input"
                type="text"
                value={form.nik}
                onChange={(e) => handleChange('nik', e.target.value)}
                placeholder="16 digit NIK"
                maxLength={16}
                required
                disabled={loading}
              />
            </div>

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-phone">
                <PhoneOutlined style={{ marginRight: 4 }} />
                Telepon <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="sr-phone"
                className="sf-form-input"
                type="tel"
                value={form.phone}
                onChange={(e) => handleChange('phone', e.target.value)}
                placeholder="Contoh: 08123456789"
                required
                disabled={loading}
              />
            </div>
          </div>

          {/* Email */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-email">
              <MailOutlined style={{ marginRight: 4 }} />
              Email <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="sr-email"
              className="sf-form-input"
              type="email"
              value={form.email}
              onChange={(e) => handleChange('email', e.target.value)}
              placeholder="nama@email.com"
              required
              disabled={loading}
            />
          </div>

          {/* RT selector for SUPER_ADMIN */}
          {adminMode && !rtLoading && rtOptions.length > 0 && (
            <div className="sf-form-field">
              <SearchableSelect
                id="sr-rt"
                label={
                  <>RT Tujuan <span style={{ color: 'var(--sf-danger)' }}>*</span></>
                }
                required
                options={rtOptions.map((rt) => ({
                  id: rt.id,
                  display: `RT ${String(rt.rt).padStart(3, '0')} / RW ${String(rt.rw).padStart(3, '0')} — ${rt.name}`,
                }))}
                value={selectedRtId}
                emptyMessage="Tidak ada RT yang cocok."
                placeholder="Cari RT..."
                disabled={loading || rtLoading}
                onChange={(v) => {
                  setSelectedRtId(v)
                  if (validationErrors.length > 0) setValidationErrors([])
                }}
              />
            </div>
          )}
          {adminMode && rtLoading && (
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 12 }}>
              Memuat daftar RT...
            </div>
          )}

          {/* Status */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-status">
              Status <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <select
              id="sr-status"
              className="sf-form-select"
              value={form.is_active ? 'true' : 'false'}
              onChange={(e) => setForm((prev) => ({ ...prev, is_active: e.target.value === 'true' }))}
              disabled={loading}
            >
              <option value="true">Aktif</option>
              <option value="false">Tidak Aktif</option>
            </select>
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Batal
          </button>
          <button
            type="submit"
            className="sf-btn-primary"
            disabled={loading || (adminMode && !selectedRtId)}
          >
            {adminMode && !selectedRtId ? 'Pilih RT' : loading ? 'Menyimpan...' : 'Simpan'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
