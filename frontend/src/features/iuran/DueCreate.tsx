import { useState, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiCreateDue,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiCreateDueBody,
} from '@/app/api'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { PlusOutlined } from '@ant-design/icons'
import { DUE_PERIOD_TYPE_OPTIONS, type DuePeriodType } from './iuranTypes'

/* ── Form shape ─────────────────────────────────────────────────────── */

interface DueFormState {
  name: string
  amount: string
  period_type: DuePeriodType
}

const emptyForm: DueFormState = {
  name: '',
  amount: '',
  period_type: 'monthly',
}

/* ── Authorization gate ─────────────────────────────────────────────── */

function hasFinanceCreateAccess(user: {
  systemRole?: string | null
  role?: string
  jabatan?: string | null
}): boolean {
  if (user.systemRole === 'super_admin') return true
  if (typeof user.jabatan === 'string' && user.jabatan.length > 0) {
    const writeJabatans = new Set([
      'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
    ])
    return writeJabatans.has(user.jabatan)
  }
  switch (user.role) {
    case 'pengurus':
    case 'super_admin':
      return true
    default:
      return false
  }
}

/* ── Shared inner form ──────────────────────────────────────────────── */

function DueFormFields({
  form,
  onChange,
  disabled,
}: {
  form: DueFormState
  onChange: (field: string, value: string) => void
  disabled: boolean
}) {
  return (
    <>
      {/* Nama Iuran */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="df-name">
          <PlusOutlined style={{ marginRight: 4 }} />
          Nama Iuran <span style={{ color: 'var(--sf-danger)' }}>*</span>
        </label>
        <input
          id="df-name"
          className="sf-form-input"
          type="text"
          value={form.name}
          onChange={(e) => onChange('name', e.target.value)}
          placeholder="Contoh: Iuran Keamanan Bulanan"
          required
          disabled={disabled}
          autoFocus
        />
      </div>

      {/* Nominal */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="df-amount">
          Nominal <span style={{ color: 'var(--sf-danger)' }}>*</span>
        </label>
        <input
          id="df-amount"
          className="sf-form-input"
          type="number"
          value={form.amount}
          onChange={(e) => onChange('amount', e.target.value)}
          placeholder="Contoh: 50000"
          required
          min="1"
          inputMode="numeric"
          disabled={disabled}
        />
      </div>

      {/* Frekuensi */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="df-period">
          Frekuensi <span style={{ color: 'var(--sf-danger)' }}>*</span>
        </label>
        <select
          id="df-period"
          className="sf-form-select"
          value={form.period_type}
          onChange={(e) => onChange('period_type', e.target.value)}
          disabled={disabled}
        >
          {DUE_PERIOD_TYPE_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>
    </>
  )
}

/* ── Create Due Modal ───────────────────────────────────────────────── */

export function DueCreate({
  open: openProp,
  onClose: onCloseProp,
  onSaved: onSavedProp,
}: {
  open?: boolean
  onClose?: () => void
  onSaved?: () => void
} = {}) {
  const { user } = useAuth()
  const navigate = useNavigate()

  const open = openProp ?? true
  const handleClose = onCloseProp ?? (() => navigate('/iuran'))
  const handleSave = onSavedProp ?? (() => {})

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [form, setForm] = useState<DueFormState>(emptyForm)
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
    if (error) setError(null)
  }

  const validate = (): boolean => {
    const errs: string[] = []
    if (!form.name.trim()) errs.push('Nama iuran harus diisi.')
    if (!form.amount || !/^\d+(\.\d+)?$/.test(form.amount)) {
      errs.push('Nominal harus angka yang valid.')
    } else if (Number(form.amount) <= 0) {
      errs.push('Nominal harus lebih besar dari 0.')
    }
    if (!form.period_type) errs.push('Frekuensi harus dipilih.')
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

      const body: ApiCreateDueBody = {
        name: form.name.trim(),
        amount: form.amount,
        period_type: form.period_type,
      }

      await apiCreateDue(pair.accessToken, body)
      handleSave()
      handleClose()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        handleClose()
      } else if (err instanceof ApiError) {
        setError(new ApiError(err.code, err.message || 'Gagal membuat iuran.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal membuat iuran.'))
      }
    } finally {
      setLoading(false)
    }
  }

  // Authorization gate
  if (user && !hasFinanceCreateAccess(user)) {
    return (
      <Modal open onClose={handleClose}>
        <ModalHeader
          title="Akses Ditolak"
          subtitle="Anda tidak memiliki izin untuk menambah iuran."
          onClose={handleClose}
        />
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Tutup
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  return (
    <Modal open onClose={handleClose} width={520} overlayClassName="sf-modal-overlay-due">
      <ModalHeader
        title="Tambah Iuran"
        subtitle="Form pembuatan jenis iuran baru"
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
          <DueFormFields form={form} onChange={handleChange} disabled={loading} />
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose} disabled={loading}>
            Batal
          </button>
          <button type="submit" className="sf-btn-primary" disabled={loading}>
            {loading ? 'Menyimpan...' : 'Simpan Iuran'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
