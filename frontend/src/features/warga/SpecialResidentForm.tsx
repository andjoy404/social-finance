import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiListRTs,
  ApiError as ApiErrorType,
  clearSessionPair,
  getSessionPair,
  type ApiRT,
} from '@/app/api'
import { useNavigate, useParams } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { SearchableSelect } from '@/components/SearchableSelect'
import { ResidentAvatar } from '@/components/ResidentAvatar'
import { ResidentPhotoManage } from '@/components/ResidentPhotoManage'
import {
  IdcardOutlined,
  MailOutlined,
  PhoneOutlined,
  UserOutlined,
  LockOutlined,
} from '@ant-design/icons'


export type JabatanValue = 'keamanan' | 'kebersihan' | 'anggota_rw' | 'operasional'

const createJabatanOptions: { value: JabatanValue; label: string }[] = [
  { value: 'keamanan', label: 'Keamanan' },
  { value: 'kebersihan', label: 'Kebersihan' },
  { value: 'anggota_rw', label: 'Anggota RW' },
]

const allJabatanOptions: { value: JabatanValue; label: string }[] = [
  { value: 'keamanan', label: 'Keamanan' },
  { value: 'kebersihan', label: 'Kebersihan' },
  { value: 'anggota_rw', label: 'Anggota RW' },
  { value: 'operasional', label: 'Operasional' },
]

export interface SpecialResidentFormValues {
  jabatan: JabatanValue
  full_name: string
  nik: string
  phone: string
  email: string
  password: string
  is_active: boolean
  rt_id: string
}

type Mode = 'create' | 'edit' | 'detail'

interface Props {
  mode: Mode
  canEdit?: boolean
  residentId?: string
  rtDisplay?: string
  initialValues?: Partial<SpecialResidentFormValues>
  onSubmit?: (values: SpecialResidentFormValues) => Promise<void>
  submitLabel?: string
  loading?: boolean
  error?: ApiErrorType | null
  subtitle?: string
  avatarPath?: string | null
  avatarThumbPath?: string | null
  onAvatarChange?: (newAvatarPath: string | null, newThumbPath: string | null) => void
}

function isSuperAdmin(systemRole: string | null): boolean {
  return systemRole === 'super_admin'
}

export function SpecialResidentForm({
  mode,
  canEdit = true,
  residentId,
  rtDisplay,
  initialValues,
  onSubmit,
  submitLabel,
  loading: externalLoading,
  error: externalError,
  subtitle,
  avatarPath,
  avatarThumbPath,
  onAvatarChange: externalOnAvatarChange,
}: Props) {
  const { user } = useAuth()
  const navigate = useNavigate()
  const { id: routeId } = useParams<{ id: string }>()
  const effectiveResidentId = residentId ?? routeId
  const adminMode = isSuperAdmin(user?.systemRole ?? null)
  const readOnly = mode === 'detail'
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState<SpecialResidentFormValues>({
    jabatan: (initialValues?.jabatan as JabatanValue) || 'keamanan',
    full_name: initialValues?.full_name || '',
    nik: initialValues?.nik || '',
    phone: initialValues?.phone || '',
    email: initialValues?.email || '',
    password: '',
    is_active: initialValues?.is_active ?? true,
    rt_id:
      initialValues?.rt_id ||
      (typeof sessionStorage !== 'undefined'
        ? sessionStorage.getItem('sf_active_rt_id') || ''
        : ''),
  })
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [localError, setLocalError] = useState<string | null>(null)

  const [rtOptions, setRtOptions] = useState<ApiRT[]>([])
  const [rtLoading, setRtLoading] = useState(false)
  const [rtError, setRtError] = useState<ApiErrorType | null>(null)

  useEffect(() => {
    if (!initialValues) return
    setForm((prev) => ({
      ...prev,
      jabatan: (initialValues.jabatan as JabatanValue) || prev.jabatan,
      full_name: initialValues.full_name ?? prev.full_name,
      nik: initialValues.nik ?? prev.nik,
      phone: initialValues.phone ?? prev.phone,
      email: initialValues.email ?? prev.email,
      is_active: initialValues.is_active ?? prev.is_active,
      rt_id: initialValues.rt_id ?? prev.rt_id,
      password: '',
    }))
  }, [
    initialValues?.jabatan,
    initialValues?.full_name,
    initialValues?.nik,
    initialValues?.phone,
    initialValues?.email,
    initialValues?.is_active,
    initialValues?.rt_id,
  ])

  useEffect(() => {
    if (!adminMode || mode === 'edit' || mode === 'detail') return
    let cancelled = false
    ;(async () => {
      setRtLoading(true)
      setRtError(null)
      try {
        const pair = getSessionPair()
        if (!pair?.accessToken) {
          navigate('/login')
          return
        }
        const allRts: ApiRT[] = []
        let page = 1
        while (true) {
          const resp = await apiListRTs(pair.accessToken, {
            page,
            page_size: 100,
            is_active: true,
          })
          if (cancelled) return
          allRts.push(...resp.data)
          if (page >= resp.pagination.total_pages) break
          page++
        }
        if (!cancelled) {
          setRtOptions(allRts)
          setRtLoading(false)
        }
      } catch (err) {
        if (!cancelled) {
          setRtError(
            err instanceof ApiErrorType
              ? err
              : new ApiErrorType('unexpected', 'Gagal memuat daftar RT.'),
          )
          setRtLoading(false)
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [adminMode, mode, navigate])

  const handleClose = () => navigate('/warga')

  const handleAvatarChange = useCallback(
    (newPath: string | null, newThumb: string | null | undefined) => {
      if (externalOnAvatarChange) {
        externalOnAvatarChange(newPath, newThumb ?? null)
      }
    },
    [externalOnAvatarChange],
  )

  const handleChange = (field: keyof SpecialResidentFormValues, value: string | boolean) => {
    setForm((prev) => ({ ...prev, [field]: value as never }))
    if (validationErrors.length > 0) setValidationErrors([])
    if (localError) setLocalError(null)
  }

  const title =
    mode === 'create' ? 'Tambah Petugas' : mode === 'detail' ? 'Detail Petugas' : 'Ubah Petugas'

  // In edit mode, non-ketua callers (including SuperAdmin) receive redacted NIK/phone/email (empty).
  // Require those fields only on create, or on edit when the caller originally received a non-empty value.
  const nikRequired = mode === 'create' || Boolean(initialValues?.nik?.trim())
  const phoneRequired = mode === 'create' || Boolean(initialValues?.phone?.trim())
  const emailRequired = mode === 'create' || Boolean(initialValues?.email?.trim())

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (readOnly || !onSubmit) return
    setValidationErrors([])
    setLocalError(null)

    const errs: string[] = []
    if (!form.full_name.trim()) errs.push('Nama harus diisi.')
    if (!form.nik.trim()) {
      if (nikRequired) errs.push('NIK harus diisi.')
    } else if (!/^[0-9]{16}$/.test(form.nik.trim())) {
      errs.push('NIK harus berupa 16 digit angka.')
    }
    if (!form.phone.trim() && phoneRequired) errs.push('Telepon harus diisi.')
    if (!form.email.trim()) {
      if (emailRequired) errs.push('Email harus diisi.')
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
      errs.push('Format email tidak valid.')
    }
    if (mode === 'create') {
      if (!form.password.trim()) {
        errs.push('Kata sandi harus diisi.')
      } else if (form.password.trim().length < 8) {
        errs.push('Kata sandi minimal 8 karakter.')
      }
      if (adminMode && !form.rt_id) errs.push('RT Tujuan harus dipilih.')
    } else if (mode === 'edit') {
      if (form.password.trim() && form.password.trim().length < 8) {
        errs.push('Kata sandi baru minimal 8 karakter.')
      }
    }
    if (errs.length > 0) {
      setValidationErrors(errs)
      return
    }

    setSaving(true)
    try {
      await onSubmit({
        ...form,
        full_name: form.full_name.trim(),
        nik: form.nik.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
        password: form.password.trim(),
      })
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiErrorType) {
        setLocalError(err.message || 'Terjadi kesalahan.')
      } else {
        setLocalError('Terjadi kesalahan.')
      }
    } finally {
      setSaving(false)
    }
  }

  const busy = saving || !!externalLoading
  const shownError = localError || externalError?.message || rtError?.message

  const showAvatar = mode !== 'create' && effectiveResidentId != null

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title={title}
        subtitle={subtitle || (mode === 'create' ? 'Data petugas keamanan / operasional / kebersihan' : form.full_name || '')}
        onClose={handleClose}
      />
      {showAvatar && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 16,
            padding: '16px 20px 0',
          }}
        >
          <ResidentAvatar
            name={form.full_name}
            avatarPath={avatarPath ?? null}
            avatarThumbPath={avatarThumbPath ?? null}
            size={84}
          />
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <ResidentPhotoManage
              residentId={effectiveResidentId!}
              fullName={form.full_name}
              avatarPath={avatarPath ?? null}
              onChanged={handleAvatarChange}
            />
          </div>
        </div>
      )}
      <form onSubmit={handleSubmit}>
        <ModalBody>
          {validationErrors.length > 0 && (
            <div className="sf-form-error-list">
              <ul>
                {validationErrors.map((e, i) => (
                  <li key={i}>{e}</li>
                ))}
              </ul>
            </div>
          )}
          {shownError && <div className="sf-form-error-list">{shownError}</div>}

          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-jabatan">
              Jenis Petugas <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <select
              id="sr-jabatan"
              className="sf-form-select"
              value={form.jabatan}
              onChange={(e) => handleChange('jabatan', e.target.value)}
              disabled={busy || readOnly}
            >
              {(mode === 'create' ? createJabatanOptions : allJabatanOptions).map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>

          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-name">
              <UserOutlined style={{ marginRight: 4 }} />
              Nama Lengkap <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="sr-name"
              className="sf-form-input"
              type="text"
              value={form.full_name}
              onChange={(e) => handleChange('full_name', e.target.value)}
              placeholder="Nama lengkap"
              required={!readOnly}
              disabled={busy || readOnly}
              autoFocus={mode === 'create'}
              readOnly={readOnly}
            />
          </div>

          <div className="sf-form-grid-2">
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-nik">
                <IdcardOutlined style={{ marginRight: 4 }} />
                NIK {nikRequired && <span style={{ color: 'var(--sf-danger)' }}>*</span>}
              </label>
              <input
                id="sr-nik"
                className="sf-form-input"
                type="text"
                value={form.nik}
                onChange={(e) => handleChange('nik', e.target.value.replace(/\D/g, ''))}
                placeholder={!nikRequired && mode === 'edit' ? 'Kosongkan jika tidak mengubah' : '16 digit NIK'}
                maxLength={16}
                required={!readOnly && nikRequired}
                disabled={busy || readOnly}
                readOnly={readOnly}
              />
            </div>
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-phone">
                <PhoneOutlined style={{ marginRight: 4 }} />
                Telepon {phoneRequired && <span style={{ color: 'var(--sf-danger)' }}>*</span>}
              </label>
              <input
                id="sr-phone"
                className="sf-form-input"
                type="tel"
                value={form.phone}
                onChange={(e) => handleChange('phone', e.target.value)}
                placeholder={!phoneRequired && mode === 'edit' ? 'Kosongkan jika tidak mengubah' : 'Contoh: 08123456789'}
                required={!readOnly && phoneRequired}
                disabled={busy || readOnly}
                readOnly={readOnly}
              />
            </div>
          </div>

          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-email">
              <MailOutlined style={{ marginRight: 4 }} />
              Email {emailRequired && <span style={{ color: 'var(--sf-danger)' }}>*</span>}
            </label>
            <input
              id="sr-email"
              className="sf-form-input"
              type="email"
              value={form.email}
              onChange={(e) => handleChange('email', e.target.value)}
              placeholder={!emailRequired && mode === 'edit' ? 'Kosongkan jika tidak mengubah' : 'nama@email.com'}
              required={!readOnly && emailRequired}
              disabled={busy || readOnly}
              readOnly={readOnly}
            />
          </div>

          {mode !== 'create' && rtDisplay && (
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-rt-readonly">
                RT / RW
              </label>
              <input
                id="sr-rt-readonly"
                className="sf-form-input"
                type="text"
                value={rtDisplay}
                disabled
                readOnly
              />
            </div>
          )}

          {mode === 'create' && adminMode && !rtLoading && rtOptions.length > 0 && (
            <div className="sf-form-field">
              <SearchableSelect
                id="sr-rt"
                label={
                  <>
                    RT Tujuan <span style={{ color: 'var(--sf-danger)' }}>*</span>
                  </>
                }
                required
                options={rtOptions.map((rt) => ({
                  id: rt.id,
                  display: `RT ${String(rt.rt).padStart(3, '0')} / RW ${String(rt.rw).padStart(3, '0')} — ${rt.name}`,
                }))}
                value={form.rt_id}
                emptyMessage="Tidak ada RT yang cocok."
                placeholder="Cari RT..."
                disabled={busy}
                onChange={(v) => {
                  handleChange('rt_id', v)
                  if (v && typeof sessionStorage !== 'undefined') {
                    try {
                      sessionStorage.setItem('sf_active_rt_id', v)
                    } catch {
                      /* ignore */
                    }
                  }
                }}
              />
            </div>
          )}
          {mode === 'create' && adminMode && rtLoading && (
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 12 }}>
              Memuat daftar RT...
            </div>
          )}

          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sr-status">
              Status <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <select
              id="sr-status"
              className="sf-form-select"
              value={form.is_active ? 'true' : 'false'}
              onChange={(e) => handleChange('is_active', e.target.value === 'true')}
              disabled={busy || readOnly}
            >
              <option value="true">Aktif</option>
              <option value="false">Tidak Aktif</option>
            </select>
          </div>

          {mode !== 'detail' && (
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="sr-password">
                <LockOutlined style={{ marginRight: 4 }} />
                {mode === 'create' ? (
                  <>
                    Kata Sandi <span style={{ color: 'var(--sf-danger)' }}>*</span>
                  </>
                ) : (
                  'Kata Sandi Baru (Opsional)'
                )}
              </label>
              <input
                id="sr-password"
                className="sf-form-input"
                type="password"
                value={form.password}
                onChange={(e) => handleChange('password', e.target.value)}
                placeholder={
                  mode === 'create'
                    ? 'Minimal 8 karakter'
                    : 'Kosongkan jika tidak ingin mengubah kata sandi'
                }
                required={mode === 'create'}
                disabled={busy}
                autoComplete="new-password"
              />
            </div>
          )}
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost sf-btn-cancel-red" onClick={handleClose}>
            {readOnly ? 'Tutup' : 'Batal'}
          </button>
          {mode === 'detail' && canEdit && residentId && (
            <button type="button" className="sf-btn-primary" onClick={() => navigate(`/warga/special/${residentId}/edit`)}>
              Ubah Petugas
            </button>
          )}
          {!readOnly && (
            <button
              type="submit"
              className="sf-btn-primary"
              disabled={busy || (mode === 'create' && adminMode && !form.rt_id)}
            >
              {mode === 'create' && adminMode && !form.rt_id
                ? 'Pilih RT'
                : busy
                  ? 'Menyimpan...'
                  : submitLabel || (mode === 'create' ? 'Simpan' : 'Simpan Perubahan')}
            </button>
          )}
        </ModalFooter>
      </form>
    </Modal>
  )
}
