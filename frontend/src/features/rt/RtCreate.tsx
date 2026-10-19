import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiCreateRT,
  apiGetRT,
  apiUpdateRT,
  apiDeactivateRT,
  apiReactivateRT,
  apiListRWs,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiCreateRTBody,
  type ApiUpdateRTBody,
  type ApiRW,
} from '@/app/api'
import { useNavigate, useParams } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import {
  ApartmentOutlined,
  NumberOutlined,
  HomeOutlined,
  UserOutlined,
  EnvironmentOutlined,
} from '@ant-design/icons'

/* ── Shared form shape ────────────────────────────────────────────────── */

export interface RtFormState {
  name: string
  rw_id: string
  rw: string
  rt: string
  address: string
  head_name: string
}

const emptyForm: RtFormState = {
  name: '',
  rw_id: '',
  rw: '',
  rt: '',
  address: '',
  head_name: '',
}

const dangerBtnStyle: React.CSSProperties = {
  background: 'rgba(242, 73, 92, 0.16)',
  border: '1px solid rgba(242, 73, 92, 0.6)',
  color: 'var(--sf-danger)',
}

/* ── Shared inner form (used by both create and edit) ─────────────────── */

export function RtFormFields({
  form,
  onChange,
  disabled,
  rwList,
}: {
  form: RtFormState
  onChange: (field: string, value: string) => void
  disabled: boolean
  rwList: ApiRW[]
}) {
  const handleRwSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const selectedId = e.target.value
    onChange('rw_id', selectedId)
    const found = rwList.find((r) => r.id === selectedId)
    if (found) {
      onChange('rw', String(found.rw_number))
    }
  }

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
          {rwList.length > 0 ? (
            <select
              id="rf-rw"
              className="sf-form-input"
              value={form.rw_id}
              onChange={handleRwSelectChange}
              required
              disabled={disabled}
            >
              <option value="">Pilih RW...</option>
              {rwList.map((rwItem) => (
                <option key={rwItem.id} value={rwItem.id}>
                  {rwItem.name || `RW ${String(rwItem.rw_number).padStart(3, '0')}`} (Nomor {rwItem.rw_number})
                </option>
              ))}
            </select>
          ) : (
            <input
              id="rf-rw"
              className="sf-form-input"
              type="number"
              value={form.rw}
              onChange={(e) => onChange('rw', e.target.value)}
              placeholder="Contoh: 16"
              required
              min="0"
              inputMode="numeric"
              disabled={disabled}
            />
          )}
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

export function RtCreate({
  open: openProp,
  onClose: onCloseProp,
  onSaved: onSavedProp,
}: {
  open?: boolean
  onClose?: () => void
  onSaved?: () => void
} = {}) {
  const navigate = useNavigate()

  const open = openProp ?? true
  const handleClose = onCloseProp ?? (() => navigate('/rt'))
  const handleSave = onSavedProp ?? (() => {})

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [form, setForm] = useState<RtFormState>(emptyForm)
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [rwList, setRwList] = useState<ApiRW[]>([])

  useEffect(() => {
    const pair = getSessionPair()
    if (!pair?.accessToken) return
    apiListRWs(pair.accessToken, { is_active: true, page_size: 100 })
      .then((res) => {
        if (Array.isArray(res?.data)) {
          setRwList(res.data)
          if (res.data.length === 1) {
            setForm((prev) =>
              prev.rw_id
                ? prev
                : { ...prev, rw_id: res.data[0].id, rw: String(res.data[0].rw_number) },
            )
          }
        }
      })
      .catch(() => {
        /* fallback to numeric input if RW endpoint not mocked in unit tests */
      })
  }, [])

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
        handleClose()
        return
      }

      const body: ApiCreateRTBody = {
        name: form.name.trim(),
        ...(form.rw_id ? { rw_id: form.rw_id } : {}),
        rw: Number(form.rw),
        rt: form.rt.trim(),
        address: form.address.trim() || null,
        head_name: form.head_name.trim() || null,
      }

      await apiCreateRT(pair.accessToken, body)
      handleSave()
      handleClose()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        handleClose()
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
    <Modal open={open} onClose={handleClose} width={520} overlayClassName="sf-modal-overlay-rt">
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
          <RtFormFields form={form} onChange={handleChange} disabled={loading} rwList={rwList} />
        </ModalBody>
        <ModalFooter>
          <button
            type="button"
            className="sf-btn-ghost"
            style={dangerBtnStyle}
            onClick={handleClose}
            disabled={loading}
          >
            Batal
          </button>
          <button type="submit" className="sf-btn-primary" disabled={loading}>
            {loading ? 'Menyimpan...' : 'Simpan'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}

/* ── Edit RT modal page ───────────────────────────────────────────────── */

export function RtEdit({ id: propId, onClose: propOnClose, onSaved: propOnSaved, overlayClassName }: { id?: string, onClose?: () => void, onSaved?: () => void, overlayClassName?: string }) {
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
  const [togglingActive, setTogglingActive] = useState(false)
  const [isActive, setIsActive] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [form, setForm] = useState<RtFormState>(emptyForm)
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [rwList, setRwList] = useState<ApiRW[]>([])

  const fetchRt = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) { handleClose(); return }

    setLoadingData(true)
    setError(null)
    setNotFound(false)

    try {
      const [data, rwsResp] = await Promise.all([
        apiGetRT(pair.accessToken, id),
        apiListRWs(pair.accessToken, { page_size: 100 }).catch(() => null),
      ])
      const loadedRws = Array.isArray(rwsResp?.data) ? rwsResp!.data : []
      setRwList(loadedRws)
      const matchedRwId = data.rw_id || loadedRws.find((r) => r.rw_number === data.rw)?.id || ''
      setIsActive(data.is_active)
      setForm({
        name: data.name,
        rw_id: matchedRwId,
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
  }, [id, handleClose])

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

  const handleToggleActive = async () => {
    setTogglingActive(true)
    setError(null)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) { handleClose(); return }
      if (isActive) {
        await apiDeactivateRT(pair.accessToken, id)
        setIsActive(false)
      } else {
        const updated = await apiReactivateRT(pair.accessToken, id)
        setIsActive(updated.is_active)
      }
      onCloseSuccess()
      handleClose()
    } catch (err) {
      if (err instanceof ApiError) {
        setError(new ApiError(err.code, err.message || 'Gagal mengubah status RT.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal mengubah status RT.'))
      }
    } finally {
      setTogglingActive(false)
    }
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
        ...(form.rw_id ? { rw_id: form.rw_id } : {}),
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
    <Modal open onClose={handleClose} width={520} overlayClassName={overlayClassName}>
      <ModalHeader
        title="Detail & Ubah RT"
        subtitle="Informasi dan pengaturan unit wilayah administratif"
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
            <button type="button" className="sf-btn-ghost" style={dangerBtnStyle} onClick={handleClose}>
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
            <RtFormFields form={form} onChange={handleChange} disabled={saving || togglingActive} rwList={rwList} />
          </ModalBody>
          <ModalFooter>
            <button
              type="button"
              className="sf-btn-ghost"
              onClick={handleToggleActive}
              disabled={saving || togglingActive}
              style={{
                marginRight: 'auto',
                ...(isActive
                  ? dangerBtnStyle
                  : {
                      background: 'rgba(115, 191, 105, 0.16)',
                      border: '1px solid rgba(115, 191, 105, 0.6)',
                      color: 'var(--sf-success)',
                    }),
              }}
            >
              {togglingActive ? 'Memproses...' : isActive ? 'Nonaktifkan RT' : 'Aktifkan RT'}
            </button>
            <button
              type="button"
              className="sf-btn-ghost"
              style={dangerBtnStyle}
              onClick={handleClose}
              disabled={saving || togglingActive}
            >
              Batal
            </button>
            <button type="submit" className="sf-btn-primary" disabled={saving || togglingActive}>
              {saving ? 'Menyimpan...' : 'Simpan Perubahan'}
            </button>
          </ModalFooter>
        </form>
      )}
    </Modal>
  )
}
