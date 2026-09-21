import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiCreateRT,
  apiGetRT,
  apiUpdateRT,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiCreateRTBody,
  type ApiUpdateRTBody,
} from '@/app/api'
import { useNavigate, useParams, Link } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import {
  ApartmentOutlined,
  NumberOutlined,
  HomeOutlined,
  UserOutlined,
  EnvironmentOutlined,
} from '@ant-design/icons'

/* ── Shared form shape ────────────────────────────────────────────────── */

interface RtFormState {
  name: string
  rw: string
  rt: string
  address: string
  head_name: string
}

const emptyForm: RtFormState = {
  name: '',
  rw: '',
  rt: '',
  address: '',
  head_name: '',
}

/* ── Shared inner form (used by both create and edit) ─────────────────── */

function RtFormFields({
  form,
  onChange,
  disabled,
}: {
  form: RtFormState
  onChange: (field: string, value: string) => void
  disabled: boolean
}) {
  return (
    <>
      {/* Name */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="rf-name">
          <ApartmentOutlined style={{ marginRight: 4 }} />
          Nama RT <span style={{ color: 'var(--sf-danger)' }}>*</span>
        </label>
        <input
          id="rf-name"
          className="sf-form-input"
          type="text"
          value={form.name}
          onChange={(e) => onChange('name', e.target.value)}
          placeholder="Contoh: RT 001"
          required
          disabled={disabled}
          autoFocus
        />
      </div>

      {/* RW + RT code */}
      <div className="sf-form-grid-2">
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="rf-rw">
            <NumberOutlined style={{ marginRight: 4 }} />
            RW <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <input
            id="rf-rw"
            className="sf-form-input"
            type="number"
            value={form.rw}
            onChange={(e) => onChange('rw', e.target.value)}
            placeholder="Contoh: 1"
            required
            min="0"
            inputMode="numeric"
            disabled={disabled}
          />
        </div>

        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="rf-rt">
            <HomeOutlined style={{ marginRight: 4 }} />
            Kode RT <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <input
            id="rf-rt"
            className="sf-form-input"
            type="text"
            value={form.rt}
            onChange={(e) => onChange('rt', e.target.value)}
            placeholder="Contoh: 001"
            required
            disabled={disabled}
          />
        </div>
      </div>

      {/* Ketua RT */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="rf-head">
          <UserOutlined style={{ marginRight: 4 }} />
          Ketua RT
        </label>
        <input
          id="rf-head"
          className="sf-form-input"
          type="text"
          value={form.head_name}
          onChange={(e) => onChange('head_name', e.target.value)}
          placeholder="Tidak diisi jika kosong"
          disabled={disabled}
        />
      </div>

      {/* Alamat */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="rf-address">
          <EnvironmentOutlined style={{ marginRight: 4 }} />
          Alamat
        </label>
        <input
          id="rf-address"
          className="sf-form-input"
          type="text"
          value={form.address}
          onChange={(e) => onChange('address', e.target.value)}
          placeholder="Tidak diisi jika kosong"
          disabled={disabled}
        />
      </div>
    </>
  )
}

/* ── Create RT modal page ─────────────────────────────────────────────── */

export function RtCreate() {
  const { user } = useAuth()
  const navigate = useNavigate()

  if (!user) return null

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [form, setForm] = useState<RtFormState>(emptyForm)
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const handleClose = () => navigate('/rt')

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const validate = (): boolean => {
    const errs: string[] = []
    if (!form.name.trim()) errs.push('Nama RT harus diisi.')
    if (!form.rw || !/^\d+$/.test(form.rw)) errs.push('RW harus angka yang valid.')
    if (form.rw && /^\d+$/.test(form.rw) && Number(form.rw) < 0) errs.push('RW tidak boleh negatif.')
    if (!form.rt.trim()) errs.push('Kode RT harus diisi.')
    if (errs.length > 0) { setValidationErrors(errs); return false }
    return true
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])
    setError(null)
    if (!validate()) return

    setLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) {
        navigate('/login')
        return
      }

      const body: ApiCreateRTBody = {
        name: form.name.trim(),
        rw: Number(form.rw),
        rt: form.rt.trim(),
        address: form.address.trim() || null,
        head_name: form.head_name.trim() || null,
      }

      await apiCreateRT(pair.accessToken, body)
      navigate('/rt')
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiError) {
        setError(new ApiError(err.code, err.message || 'Gagal membuat RT.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal membuat RT.'))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal open onClose={handleClose} width={520}>
      <ModalHeader
        title="Tambah RT Baru"
        subtitle="Form pembuatan unit wilayah administratif"
        onClose={handleClose}
      />
      <form onSubmit={handleSubmit}>
        <ModalBody>
          {validationErrors.length > 0 && (
            <div className="sf-form-error-list">
              <ul>
                {validationErrors.map((e, i) => <li key={i}>{e}</li>)}
              </ul>
            </div>
          )}
          {error && (
            <div className="sf-form-error-list">
              {error.message}
            </div>
          )}
          <RtFormFields form={form} onChange={handleChange} disabled={loading} />
        </ModalBody>
        <ModalFooter>
          <Link to="/rt" className="sf-btn-ghost">
            Batal
          </Link>
          <button type="submit" className="sf-btn-primary" disabled={loading}>
            {loading ? 'Menyimpan...' : 'Simpan'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}

/* ── Edit RT modal page ───────────────────────────────────────────────── */

export function RtEdit({ id: propId, onClose: propOnClose, onSaved: propOnSaved }: { id?: string, onClose?: () => void, onSaved?: () => void }) {
  const { user } = useAuth()
  const { id: routeId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const id = propId ?? routeId

  if (!user) return null
  if (!id) {
    navigate('/rt', { replace: true })
    return null
  }

  const handleClose = propOnClose ?? (() => navigate('/rt/' + id))
  const onCloseSuccess = propOnSaved ?? (() => {})

  const [loadingData, setLoadingData] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [form, setForm] = useState<RtFormState>(emptyForm)
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const fetchRt = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) { handleClose(); return }

    setLoadingData(true)
    setError(null)
    setNotFound(false)

    try {
      const data = await apiGetRT(pair.accessToken, id)
      setForm({
        name: data.name,
        rw: String(data.rw),
        rt: data.rt,
        address: data.address ?? '',
        head_name: data.head_name ?? '',
      })
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        handleClose()
      } else if (err instanceof ApiError && err.code === 'not_found') {
        setNotFound(true)
      } else {
        setError(new ApiError('unexpected', 'Gagal memuat data RT.'))
      }
    } finally {
      setLoadingData(false)
    }
  }, [id, navigate, handleClose])

  useEffect(() => { void fetchRt() }, [fetchRt])

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const validate = (): boolean => {
    const errs: string[] = []
    if (!form.name.trim()) errs.push('Nama RT harus diisi.')
    if (!form.rw || !/^\d+$/.test(form.rw)) errs.push('RW harus angka yang valid.')
    if (form.rw && /^\d+$/.test(form.rw) && Number(form.rw) < 0) errs.push('RW tidak boleh negatif.')
    if (!form.rt.trim()) errs.push('Kode RT harus diisi.')
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

      const body: ApiUpdateRTBody = {
        name: form.name.trim(),
        rw: Number(form.rw),
        rt: form.rt.trim(),
        address: form.address.trim() || null,
        head_name: form.head_name.trim() || null,
      }

      await apiUpdateRT(pair.accessToken, id, body)
      onCloseSuccess()
      handleClose()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        handleClose()
      } else if (err instanceof ApiError) {
        setError(new ApiError(err.code, err.message || 'Gagal memperbarui RT.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal memperbarui RT.'))
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open onClose={handleClose} width={520}>
      <ModalHeader
        title="Edit RT"
        subtitle="Ubah informasi unit wilayah administratif"
        onClose={handleClose}
      />
      {loadingData ? (
        <>
          <ModalBody>
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
              {notFound ? 'RT tidak ditemukan.' : 'Memuat data RT...'}
            </div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Tutup
            </button>
          </ModalFooter>
        </>
      ) : (
        <form onSubmit={handleSubmit}>
          <ModalBody>
            {validationErrors.length > 0 && (
              <div className="sf-form-error-list">
                <ul>
                  {validationErrors.map((e, i) => <li key={i}>{e}</li>)}
                </ul>
              </div>
            )}
            {error && (
              <div className="sf-form-error-list">
                {error.message}
              </div>
            )}
            <RtFormFields form={form} onChange={handleChange} disabled={saving} />
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose} disabled={saving}>
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
