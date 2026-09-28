import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  getResident,
  updateSpecialResident,
  ApiError as ApiErrorType,
  clearSessionPair,
  getSessionPair,
  type ApiResident,
} from '@/app/api'
import { useNavigate, useParams } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'

const validJabatans: Record<string, boolean> = {
  keamanan: true,
  kebersihan_pembangunan: true,
}

type JabatanValue = 'keamanan' | 'kebersihan_pembangunan'

const jabatanOptions: { value: JabatanValue; label: string }[] = [
  { value: 'keamanan', label: 'Keamanan' },
  { value: 'kebersihan_pembangunan', label: 'Kebersihan & Pembangunan' },
]

interface SpecialResidentFormState {
  full_name: string
  nik: string
  phone: string
  email: string
  jabatan: JabatanValue
  is_active: boolean
}

export function SpecialResidentEdit() {
  const { user } = useAuth()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  if (!user || !id) return null

  const [loadingData, setLoadingData] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<ApiErrorType | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [isSpecial, setIsSpecial] = useState(false)
  const [form, setForm] = useState<SpecialResidentFormState>({
    full_name: '',
    nik: '',
    phone: '',
    email: '',
    jabatan: 'keamanan',
    is_active: true,
  })
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const fetchResident = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      navigate('/login')
      return
    }

    setLoadingData(true)
    setError(null)
    setNotFound(false)

    try {
      const data: ApiResident = await getResident(pair.accessToken, id)

      if (data.jabatan && validJabatans[data.jabatan]) {
        setIsSpecial(true)
        setForm({
          full_name: data.full_name,
          nik: data.nik ?? '',
          phone: data.phone ?? '',
          email: data.email ?? '',
          jabatan: data.jabatan as JabatanValue,
          is_active: data.is_active,
        })
      } else {
        // Not a special resident — redirect to normal edit
        if (data.household_id) {
          navigate(`/warga/${data.household_id}/edit`, { replace: true })
        } else {
          setNotFound(true)
          setIsSpecial(false)
        }
      }
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
      setLoadingData(false)
    }
  }, [id, navigate])

  useEffect(() => { void fetchResident() }, [fetchResident])

  const handleClose = () => navigate('/warga')

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const validate = (): boolean => {
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
    if (errs.length > 0) { setValidationErrors(errs); return false }
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
      if (!pair?.accessToken) { handleClose(); return }

      const body = {
        full_name: form.full_name.trim(),
        nik: form.nik.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
        jabatan: form.jabatan,
        is_active: form.is_active,
      }

      await updateSpecialResident(pair.accessToken, id, body)
      navigate('/warga')
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        handleClose()
      } else if (err instanceof ApiErrorType) {
        setError(new ApiErrorType(err.code, err.message || 'Gagal memperbarui petugas khusus.'))
      } else {
        setError(new ApiErrorType('unexpected', 'Gagal memperbarui petugas khusus.'))
      }
    } finally {
      setSaving(false)
    }
  }

  if (!isSpecial) {
    return loadingData ? (
      <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
        Memuat data...
      </div>
    ) : notFound ? (
      <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
        Data tidak ditemukan.
      </div>
    ) : null
  }

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title="Ubah Petugas Khusus"
        subtitle={form.full_name || 'Memuat...'}
        onClose={handleClose}
      />
      {loadingData ? (
        <>
          <ModalBody>
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
              Memuat data petugas khusus...
            </div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Batal
            </button>
          </ModalFooter>
        </>
      ) : notFound ? (
        <>
          <ModalBody>
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
              Petugas khusus tidak ditemukan.
            </div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Batal
            </button>
          </ModalFooter>
        </>
      ) : error ? (
        <>
          <ModalBody>
            <div className="sf-form-error-list">{error.message}</div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Batal
            </button>
          </ModalFooter>
        </>
      ) : (
        <form onSubmit={handleSubmit}>
          <ModalBody>
            {validationErrors.length > 0 && (
              <div className="sf-form-error-list">
                <ul>{validationErrors.map((e, i) => <li key={i}>{e}</li>)}</ul>
              </div>
            )}

            {/* Jenis Petugas */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-e-jabatan">
                Jenis Petugas <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <select
                id="sr-e-jabatan"
                className="sf-form-select"
                value={form.jabatan}
                onChange={(e) => handleChange('jabatan', e.target.value)}
                disabled={saving}
              >
                {jabatanOptions.map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
              </select>
            </div>

            {/* Nama */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-e-name">
                Nama <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="sr-e-name"
                className="sf-form-input"
                type="text"
                value={form.full_name}
                onChange={(e) => handleChange('full_name', e.target.value)}
                placeholder="Nama lengkap"
                required
                disabled={saving}
              />
            </div>

            {/* NIK + Telepon */}
            <div className="sf-form-grid-2">
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="sr-e-nik">
                  NIK <span style={{ color: 'var(--sf-danger)' }}>*</span>
                </label>
                <input
                  id="sr-e-nik"
                  className="sf-form-input"
                  type="text"
                  value={form.nik}
                  onChange={(e) => handleChange('nik', e.target.value)}
                  placeholder="16 digit NIK"
                  maxLength={16}
                  required
                  disabled={saving}
                />
              </div>

              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="sr-e-phone">
                  Telepon <span style={{ color: 'var(--sf-danger)' }}>*</span>
                </label>
                <input
                  id="sr-e-phone"
                  className="sf-form-input"
                  type="tel"
                  value={form.phone}
                  onChange={(e) => handleChange('phone', e.target.value)}
                  placeholder="Contoh: 08123456789"
                  required
                  disabled={saving}
                />
              </div>
            </div>

            {/* Email */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-e-email">
                Email <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="sr-e-email"
                className="sf-form-input"
                type="email"
                value={form.email}
                onChange={(e) => handleChange('email', e.target.value)}
                placeholder="nama@email.com"
                required
                disabled={saving}
              />
            </div>

            {/* Status */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-e-status">
                Status <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <select
                id="sr-e-status"
                className="sf-form-select"
                value={form.is_active ? 'true' : 'false'}
                onChange={(e) => setForm((prev) => ({ ...prev, is_active: e.target.value === 'true' }))}
                disabled={saving}
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
            <button type="submit" className="sf-btn-primary" disabled={saving}>
              {saving ? 'Menyimpan...' : 'Simpan Perubahan'}
            </button>
          </ModalFooter>
        </form>
      )}
    </Modal>
  )
}
