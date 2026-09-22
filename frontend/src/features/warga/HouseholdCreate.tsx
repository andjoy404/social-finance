import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  createHousehold,
  getHousehold,
  updateHousehold,
  apiListRTs,
  ApiError as ApiErrorType,
  clearSessionPair,
  getSessionPair,
  type ApiCreateHouseholdBody,
  type ApiUpdateHouseholdBody,
  type ApiHousehold,
  type ApiRT,
} from '@/app/api'
import { useNavigate, useParams } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { SearchableSelect } from '@/components/SearchableSelect'
import {
  HomeOutlined,
  UserOutlined,
  EnvironmentOutlined,
  PhoneOutlined,
  IdcardOutlined,
  MailOutlined,
  CalendarOutlined,
} from '@ant-design/icons'

type OccupancyStatus = 'OWNER' | 'TENANT'

/* ── Shared form fields ───────────────────────────────────────────────── */

interface HouseholdFormState {
  house_number: string
  head_name: string
  nik: string
  phone: string
  email: string
  address: string
  occupancy_status: OccupancyStatus
  is_active: boolean
}

const emptyHouseholdForm: HouseholdFormState = {
  house_number: '',
  head_name: '',
  nik: '',
  phone: '',
  email: '',
  address: '',
  occupancy_status: 'OWNER',
  is_active: true,
}

/* ── Status select field ────────────────────────────────────────────── */

function StatusField({
  value,
  onChange,
  disabled,
}: {
  value: boolean
  onChange: (v: boolean) => void
  disabled: boolean
}) {
  return (
    <div className="sf-form-field">
      <label className="sf-form-label" htmlFor="hf-status">
        Status <span style={{ color: 'var(--sf-danger)' }}>*</span>
      </label>
      <select
        id="hf-status"
        className="sf-form-select"
        value={value ? 'true' : 'false'}
        onChange={(e) => onChange(e.target.value === 'true')}
        disabled={disabled}
      >
        <option value="true">Aktif</option>
        <option value="false">Tidak Aktif</option>
      </select>
    </div>
  )
}

function HouseholdFormFields({
  form,
  onChange,
  disabled,
  showStartDate,
  startDate,
  onStartDateChange,
}: {
  form: HouseholdFormState
  onChange: (field: string, value: string) => void
  disabled: boolean
  showStartDate?: boolean
  startDate?: string
  onStartDateChange?: (value: string) => void
}) {
  return (
    <>
      {/* Nomor Rumah + Status Hunian */}
      <div className="sf-form-grid-2">
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="hf-house-number">
            <HomeOutlined style={{ marginRight: 4 }} />
            Nomor Rumah <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <input
            id="hf-house-number"
            className="sf-form-input"
            type="text"
            value={form.house_number}
            onChange={(e) => onChange('house_number', e.target.value)}
            placeholder="Contoh: 001"
            required
            disabled={disabled}
            autoFocus
          />
        </div>

        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="hf-occupancy">
            Status Hunian <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <select
            id="hf-occupancy"
            className="sf-form-select"
            value={form.occupancy_status}
            onChange={(e) => onChange('occupancy_status', e.target.value)}
            disabled={disabled}
          >
            <option value="OWNER">Pemilik</option>
            <option value="TENANT">Penyewa</option>
          </select>
        </div>
      </div>

      {/* Nama Warga */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="hf-head-name">
          <UserOutlined style={{ marginRight: 4 }} />
          Nama Kepala Keluarga <span style={{ color: 'var(--sf-danger)' }}>*</span>
        </label>
        <input
          id="hf-head-name"
          className="sf-form-input"
          type="text"
          value={form.head_name}
          onChange={(e) => onChange('head_name', e.target.value)}
          placeholder="Nama lengkap kepala keluarga / warga"
          required
          disabled={disabled}
        />
      </div>

      {/* KTP / NIK + Nomor Telepon */}
      <div className="sf-form-grid-2">
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="hf-nik">
            <IdcardOutlined style={{ marginRight: 4 }} />
            KTP / NIK <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <input
            id="hf-nik"
            className="sf-form-input"
            type="text"
            value={form.nik}
            onChange={(e) => onChange('nik', e.target.value)}
            placeholder="16 digit NIK"
            maxLength={16}
            required
            disabled={disabled}
          />
        </div>

        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="hf-phone">
            <PhoneOutlined style={{ marginRight: 4 }} />
            Nomor Telepon <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <input
            id="hf-phone"
            className="sf-form-input"
            type="tel"
            value={form.phone}
            onChange={(e) => onChange('phone', e.target.value)}
            placeholder="Contoh: 08123456789"
            required
            disabled={disabled}
          />
        </div>
      </div>

      {/* Email + Tanggal Mulai Hunian */}
      <div className="sf-form-grid-2">
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="hf-email">
            <MailOutlined style={{ marginRight: 4 }} />
            Email <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <input
            id="hf-email"
            className="sf-form-input"
            type="email"
            value={form.email}
            onChange={(e) => onChange('email', e.target.value)}
            placeholder="nama@email.com"
            required
            disabled={disabled}
          />
        </div>

        {showStartDate && onStartDateChange ? (
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="hf-start-date">
              <CalendarOutlined style={{ marginRight: 4 }} />
              Tanggal Mulai Hunian <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="hf-start-date"
              className="sf-form-input"
              type="date"
              value={startDate ?? ''}
              onChange={(e) => onStartDateChange(e.target.value)}
              required
              disabled={disabled}
            />
          </div>
        ) : (
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="hf-address">
              <EnvironmentOutlined style={{ marginRight: 4 }} />
              Alamat
            </label>
            <input
              id="hf-address"
              className="sf-form-input"
              type="text"
              value={form.address}
              onChange={(e) => onChange('address', e.target.value)}
              placeholder="Tidak diisi jika kosong"
              disabled={disabled}
            />
          </div>
        )}
      </div>

      {/* Alamat if showStartDate */}
      {showStartDate && onStartDateChange && (
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="hf-address">
            <EnvironmentOutlined style={{ marginRight: 4 }} />
            Alamat
          </label>
          <input
            id="hf-address"
            className="sf-form-input"
            type="text"
            value={form.address}
            onChange={(e) => onChange('address', e.target.value)}
            placeholder="Tidak diisi jika kosong"
            disabled={disabled}
          />
        </div>
      )}
    </>
  )
}

/* ── Create Household modal ───────────────────────────────────────────── */

function isSuperAdmin(systemRole: string | null): boolean {
  return systemRole === 'super_admin'
}

export function HouseholdCreate() {
  const { user } = useAuth()
  const navigate = useNavigate()

  if (!user) return null

  const adminMode = isSuperAdmin(user.systemRole)

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<ApiErrorType | null>(null)
  const [form, setForm] = useState<HouseholdFormState>(emptyHouseholdForm)
  const [startDate, setStartDate] = useState('')
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  /* SUPER_ADMIN RT selector state */
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
    if (!form.house_number.trim()) errs.push('Nomor rumah harus diisi.')
    if (!form.head_name.trim()) errs.push('Nama kepala keluarga harus diisi.')
    if (!form.nik.trim()) {
      errs.push('KTP / NIK harus diisi.')
    } else if (!/^[0-9]{16}$/.test(form.nik.trim())) {
      errs.push('KTP / NIK harus berupa 16 digit angka.')
    }
    if (!form.phone.trim()) errs.push('Nomor telepon harus diisi.')
    if (!form.email.trim()) {
      errs.push('Email harus diisi.')
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
      errs.push('Format email tidak valid.')
    }
    if (!startDate) errs.push('Tanggal mulai hunian harus diisi.')
    if (adminMode && !selectedRtId) errs.push('RT Tujuan harus dipilih.')
    if (errs.length > 0) { setValidationErrors(errs); return }

    setLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) { navigate('/login'); return }

      const body: ApiCreateHouseholdBody = {
        house_number: form.house_number.trim(),
        head_name: form.head_name.trim(),
        nik: form.nik.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
        occupancy_status: form.occupancy_status,
        is_active: form.is_active,
        address: form.address.trim() || null,
        start_date: startDate,
      }

      if (adminMode && selectedRtId) {
        body.rt_id = selectedRtId
      }

      await createHousehold(pair.accessToken, body)
      navigate('/warga')
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiErrorType) {
        setError(new ApiErrorType(err.code, err.message || 'Gagal membuat rumah tangga.'))
      } else {
        setError(new ApiErrorType('unexpected', 'Gagal membuat rumah tangga.'))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title="Tambah Rumah Tangga Baru"
        subtitle="Buat data rumah tangga baru"
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

          {/* RT selector for SUPER_ADMIN */}
          {adminMode && !rtLoading && rtOptions.length > 0 && (
            <div className="sf-form-field">
              <SearchableSelect
                id="hf-rt"
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

          <HouseholdFormFields
            form={form}
            onChange={handleChange}
            disabled={loading}
            showStartDate
            startDate={startDate}
            onStartDateChange={setStartDate}
          />

          <StatusField
            value={form.is_active}
            onChange={(v) => setForm((prev) => ({ ...prev, is_active: v }))}
            disabled={loading}
          />
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

/* ── Edit Household modal ─────────────────────────────────────────────── */

export function HouseholdEdit({ id: extId, onClose: propOnClose, onSaved: propOnSaved }: { id?: string; onClose?: () => void; onSaved?: () => void }) {
  const { user } = useAuth()
  const { id: routeId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const id = extId ?? routeId

  if (!user) return null
  if (!id) {
    navigate('/warga', { replace: true })
    return null
  }

  const handleClose = propOnClose ?? (() => navigate('/warga/' + id))

  const [loadingData, setLoadingData] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<ApiErrorType | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [form, setForm] = useState<HouseholdFormState>(emptyHouseholdForm)
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const fetchHousehold = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) { navigate('/login'); return }

    setLoadingData(true)
    setError(null)
    setNotFound(false)

    try {
      const data: ApiHousehold = await getHousehold(pair.accessToken, id)
      setForm({
        house_number: data.house_number ?? '',
        head_name: data.head_name ?? '',
        nik: data.nik ?? data.head_resident?.nik ?? '',
        phone: data.phone ?? data.head_resident?.phone ?? '',
        email: data.email ?? data.head_resident?.email ?? '',
        address: data.address ?? '',
        occupancy_status: (data.occupancy_status as OccupancyStatus) || 'OWNER',
        is_active: data.is_active,
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
      setLoadingData(false)
    }
  }, [id, navigate])

  useEffect(() => { void fetchHousehold() }, [fetchHousehold])

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
  }

  const validate = (): boolean => {
    const errs: string[] = []
    if (!form.house_number.trim()) errs.push('Nomor rumah harus diisi.')
    if (!form.head_name.trim()) errs.push('Nama kepala keluarga harus diisi.')
    if (!form.nik.trim()) {
      errs.push('KTP / NIK harus diisi.')
    } else if (!/^[0-9]{16}$/.test(form.nik.trim())) {
      errs.push('KTP / NIK harus berupa 16 digit angka.')
    }
    if (!form.phone.trim()) {
      errs.push('Nomor telepon harus diisi.')
    }
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

      const body: ApiUpdateHouseholdBody = {
        house_number: form.house_number.trim(),
        head_name: form.head_name.trim(),
        nik: form.nik.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
        address: form.address.trim() || null,
        occupancy_status: form.occupancy_status,
        is_active: form.is_active,
      }

      await updateHousehold(pair.accessToken, id, body)
      propOnSaved?.()
      handleClose()
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        handleClose()
      } else if (err instanceof ApiErrorType) {
        setError(new ApiErrorType(err.code, err.message || 'Gagal memperbarui rumah tangga.'))
      } else {
        setError(new ApiErrorType('unexpected', 'Gagal memperbarui rumah tangga.'))
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title="Edit Rumah Tangga"
        subtitle="Ubah informasi rumah tangga"
        onClose={handleClose}
      />
      {loadingData ? (
        <>
          <ModalBody>
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
              Memuat data rumah tangga...
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
              Rumah tangga tidak ditemukan.
            </div>
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
            {error && <div className="sf-form-error-list">{error.message}</div>}
            <HouseholdFormFields
              form={form}
              onChange={handleChange}
              disabled={saving}
            />

            <StatusField
              value={form.is_active}
              onChange={(v) => setForm((prev) => ({ ...prev, is_active: v }))}
              disabled={saving}
            />
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
