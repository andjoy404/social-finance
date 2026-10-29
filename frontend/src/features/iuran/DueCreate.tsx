import { useState } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiCreateDue,
  apiGenerateBills,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiCreateDueBody,
} from '@/app/api'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { PlusOutlined } from '@ant-design/icons'
import { DatePicker } from 'antd'
import dayjs from 'dayjs'
import { DUE_PERIOD_TYPE_OPTIONS, type DuePeriodType, type DueType, formatRupiah, PAYMENT_RULE_OPTIONS, type PaymentRuleType } from './iuranTypes'

/* ── Form shape ─────────────────────────────────────────────────────── */

interface DueFormState {
  name: string
  amount: string
  period_type: DuePeriodType
  payment_rule: PaymentRuleType
  due_type: DueType
  year: number
  valid_until: string
}

const currentYear = new Date().getFullYear()

const emptyForm: DueFormState = {
  name: '',
  amount: '',
  period_type: 'monthly',
  payment_rule: 'full',
  due_type: 'regular',
  year: currentYear,
  valid_until: '',
}

/* ── Authorization gate ─────────────────────────────────────────────── */

function hasFinanceCreateAccess(user: {
  systemRole?: string | null
  role?: string
  jabatan?: string | null
  effectivePermissions?: string[]
}): boolean {
  if (user.systemRole === 'super_admin') return true
  if (Array.isArray(user.effectivePermissions) && user.effectivePermissions.includes('finance.create')) {
    return true
  }
  if (typeof user.jabatan === 'string' && user.jabatan.length > 0) {
    const writeJabatans = new Set([
      'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
    ])
    return writeJabatans.has(user.jabatan)
  }
  switch (user.role) {
    case 'pengurus':
    case 'bendahara':
    case 'super_admin':
      return true
    default:
      return false
  }
}

/* ── Badge toggle for Due Type ──────────────────────────────────────── */

function DueTypeBadge({
  selected,
  onSelect,
}: {
  selected: DueType
  onSelect: (type: DueType) => void
}) {
  const style = (type: DueType): React.CSSProperties => ({
    padding: '4px 14px',
    fontSize: '12px',
    fontWeight: 600,
    borderRadius: '6px',
    border: selected === type ? '1px solid var(--sf-accent)' : '1px solid var(--sf-border)',
    background: selected === type ? 'var(--sf-accent-soft)' : 'transparent',
    color: selected === type ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
    cursor: 'pointer',
    transition: 'all 150ms ease',
    outline: 'none',
    userSelect: 'none',
  })

  return (
    <div
      role="radiogroup"
      aria-label="Jenis Iuran"
      style={{ display: 'flex', gap: '8px' }}
    >
      <button
        type="button"
        role="radio"
        aria-checked={selected === 'regular'}
        tabIndex={selected === 'regular' ? 0 : -1}
        onClick={() => onSelect('regular')}
        onKeyDown={(e) => {
          if (e.key === ' ' || e.key === 'Enter') {
            e.preventDefault()
            onSelect('regular')
          }
        }}
        style={style('regular')}
      >
        Reguler
      </button>
      <button
        type="button"
        role="radio"
        aria-checked={selected === 'voluntary'}
        tabIndex={selected === 'voluntary' ? 0 : -1}
        onClick={() => onSelect('voluntary')}
        onKeyDown={(e) => {
          if (e.key === ' ' || e.key === 'Enter') {
            e.preventDefault()
            onSelect('voluntary')
          }
        }}
        style={style('voluntary')}
      >
        Sukarela
      </button>
      <button
        type="button"
        role="radio"
        aria-checked={selected === 'multi_month'}
        tabIndex={selected === 'multi_month' ? 0 : -1}
        onClick={() => onSelect('multi_month')}
        onKeyDown={(e) => {
          if (e.key === ' ' || e.key === 'Enter') {
            e.preventDefault()
            onSelect('multi_month')
          }
        }}
        style={style('multi_month')}
      >
        Multi-Bulan
      </button>
    </div>
  )
}

/* ── Shared inner form ──────────────────────────────────────────────── */

function DueFormFields({
  form,
  onChange,
  disabled,
  period,
  dueDate,
  onPeriodChange,
  onDueDateChange,
}: {
  form: DueFormState
  onChange: (field: string, value: string) => void
  disabled: boolean
  period: string
  dueDate: string
  onPeriodChange: (value: string) => void
  onDueDateChange: (value: string) => void
}) {
  return (
    <>
      {/* Jenis Iuran — badge toggle */}
      <div className="sf-form-field">
        <label className="sf-form-label">
          Jenis Iuran <span style={{ color: 'var(--sf-danger)' }}>*</span>
        </label>
        <DueTypeBadge
          selected={form.due_type}
          onSelect={(type) => onChange('due_type', type)}
        />
      </div>

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
          placeholder={form.due_type === 'multi_month' ? "Contoh: Iuran Kebersihan 2027" : "Contoh: Iuran Keamanan Bulanan"}
          required
          disabled={disabled}
          autoFocus
        />
      </div>

      {/* Tahun Tagihan — Multi-Bulan only */}
      {form.due_type === 'multi_month' && (
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="df-year">
            Tahun Tagihan Kalender <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <select
            id="df-year"
            className="sf-form-select"
            value={form.year}
            onChange={(e) => onChange('year', e.target.value)}
            disabled={disabled}
          >
            {[currentYear - 1, currentYear, currentYear + 1, currentYear + 2].map((y) => (
              <option key={y} value={y}>
                Tahun {y} (Januari - Desember {y})
              </option>
            ))}
          </select>
        </div>
      )}

      {/* Nominal */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="df-amount">
          {form.due_type === 'multi_month' ? 'Nominal per Bulan' : 'Nominal'} {form.due_type === 'voluntary' ? '' : <span style={{ color: 'var(--sf-danger)' }}>*</span>}
        </label>
        <input
          id="df-amount"
          className="sf-form-input"
          type="text"
          value={form.due_type === 'voluntary' ? 'Rp0' : (form.amount ? formatRupiah(Number(form.amount)) : 'Rp0')}
          onChange={(e) => {
            const raw = e.target.value.replace(/\D/g, '')
            onChange('amount', raw)
          }}
          placeholder={form.due_type === 'voluntary' ? 'Contoh: 0 untuk sumbangan sukarela' : 'Contoh: 50000'}
          required={form.due_type !== 'voluntary'}
          disabled={disabled}
        />
      </div>



      {/* Informasi Multi-Bulan — after Nominal Per Bulan */}
      {form.due_type === 'multi_month' && (
        <div style={{
          padding: '10px 14px',
          background: 'var(--sf-info-soft, rgba(37,99,235,0.08))',
          border: '1px solid var(--sf-info, #2563eb)',
          borderRadius: 'var(--sf-radius-sm, 6px)',
          fontSize: '12px',
          color: 'var(--sf-text)',
          marginBottom: '12px',
        }}>
          Sistem akan otomatis menerbitkan 12 tagihan bulanan (Januari s/d Desember {form.year}) untuk seluruh warga yang memenuhi syarat.
        </div>
      )}

      {/* Masa berlaku — voluntary only */}
      {form.due_type === 'voluntary' && (
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="df-valid-until">
            Berlaku Sampai (opsional)
          </label>
          <DatePicker
            id="df-valid-until"
            picker="date"
            value={form.valid_until ? dayjs(form.valid_until) : null}
            onChange={(date) => onChange('valid_until', date?.format('YYYY-MM-DD') || '')}
            allowClear
            placeholder="Tanpa batas waktu"
            format="DD/MM/YYYY"
            disabled={disabled}
            disabledDate={(date) => date.isBefore(dayjs().startOf('day'))}
            style={{ width: '100%' }}
          />
          <p style={{ margin: '6px 0 0', fontSize: '12px', color: 'var(--sf-text-muted)' }}>
            Iuran tetap aktif sampai tanggal ini (inklusif, WIB), lalu otomatis tidak aktif pada hari berikutnya.
          </p>
        </div>
      )}

      {/* Frekuensi — only for regular & voluntary */}
      {form.due_type !== 'multi_month' && (
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
      )}

      {/* Rule Pembayaran — only for reguler */}
      {form.due_type === 'regular' && (
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="df-payment-rule">
            Aturan Pembayaran <span style={{ color: 'var(--sf-danger)' }}>*</span>
          </label>
          <select
            id="df-payment-rule"
            className="sf-form-select"
            value={form.payment_rule}
            onChange={(e) => onChange('payment_rule', e.target.value)}
            disabled={disabled}
          >
            {PAYMENT_RULE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
      )}

      {/* Periode — only for regular & voluntary (NOT multi_month) */}
      {form.due_type !== 'multi_month' && (
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="df-period-value">Periode</label>
          <DatePicker
            id="df-period-value"
            picker="month"
            format="MMMM YYYY"
            value={period ? dayjs(period, 'YYYY-MM') : null}
            onChange={(date) => onPeriodChange(date?.format('YYYY-MM') || '')}
            allowClear={false}
            placeholder="Bulan dan Tahun"
            disabled={disabled}
            style={{ width: '100%' }}
          />
        </div>
      )}

      {/* Tanggal Jatuh Tempo — only for regular & voluntary (NOT multi_month) */}
      {form.due_type !== 'multi_month' && (
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="df-due-date">Tanggal Jatuh Tempo</label>
          <DatePicker
            id="df-due-date"
            picker="date"
            value={dueDate ? dayjs(dueDate) : null}
            onChange={(date) => onDueDateChange(date?.format('YYYY-MM-DD') || '')}
            allowClear={false}
            placeholder="DD/MM/YYYY"
            format="DD/MM/YYYY"
            disabled={disabled}
            style={{ width: '100%' }}
          />
        </div>
      )}

      {/* Informasi Sukarela — after Tanggal Jatuh Tempo */}
      {form.due_type === 'voluntary' && (
        <div style={{
          padding: '10px 14px',
          background: 'var(--sf-info-soft)',
          border: '1px solid var(--sf-info)',
          borderRadius: 'var(--sf-radius-sm)',
          fontSize: '12px',
          color: 'var(--sf-text)',
          marginBottom: '0',
        }}>
          Iuran sukarela bersifat opsional. Tidak ada tunggakan atau penalti jika tidak berkontribusi.
        </div>
      )}
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
  const [period, setPeriod] = useState('')
  const [dueDate, setDueDate] = useState('')
  const handlePeriodChange = (value: string) => setPeriod(value)
  const handleDueDateChange = (value: string) => setDueDate(value)

  const handleChange = (field: string, value: string) => {
    setForm((prev) => {
      const next = { ...prev, [field]: value }
      // Reset dependent fields when type changes
      if (field === 'due_type') {
        if (value === 'voluntary') {
          next.amount = '0'
          next.payment_rule = 'full'
        } else {
          next.amount = ''
          next.valid_until = ''
        }
      }
      return next
    })
    if (validationErrors.length > 0) setValidationErrors([])
    if (error) setError(null)
  }

  const validate = (): boolean => {
    const errs: string[] = []
    if (!form.name.trim()) errs.push('Nama iuran harus diisi.')

    if (form.due_type === 'voluntary') {
      if (!form.amount || !/^\d+(\.\d+)?$/.test(form.amount)) {
        errs.push('Nominal harus angka yang valid.')
      } else if (Number(form.amount) < 0) {
        errs.push('Nominal harus nol atau lebih.')
      }
    } else {
      if (!form.amount || !/^\d+(\.\d+)?$/.test(form.amount)) {
        errs.push('Nominal harus angka yang valid.')
      } else if (Number(form.amount) <= 0) {
        errs.push('Nominal harus lebih besar dari 0.')
      }
    }

    if (form.due_type === 'multi_month') {
      if (!form.year || form.year < 2000 || form.year > 2100) errs.push('Tahun tagihan harus antara 2000 dan 2100.')
    }

    if (form.due_type === 'voluntary' && form.valid_until && !/^\d{4}-\d{2}-\d{2}$/.test(form.valid_until)) {
      errs.push('Tanggal berlaku harus YYYY-MM-DD.')
    }
    if (errs.length > 0) { setValidationErrors(errs); return false }
    return true
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (loading) return
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

      const effectivePeriod = form.due_type === 'multi_month' ? String(form.year) : period
      const effectiveDueDate = form.due_type === 'multi_month' ? `${form.year}-01-10` : (dueDate || '')

      const body: ApiCreateDueBody = {
        name: form.name.trim(),
        amount: form.due_type === 'voluntary' ? '0' : (form.amount || '0'),
        period_type: form.due_type === 'multi_month' ? 'monthly' : form.period_type,
        payment_rule: form.due_type === 'voluntary' ? 'partial' : form.payment_rule,
        is_voluntary: form.due_type === 'voluntary',
        due_type: form.due_type,
        ...(form.due_type === 'multi_month' ? { year: form.year, due_date: effectiveDueDate } : {}),
        ...(form.due_type === 'voluntary' && form.valid_until
          ? { valid_until: form.valid_until }
          : {}),
      }

      const due = await apiCreateDue(pair.accessToken, body, crypto.randomUUID())
      if (form.due_type !== 'multi_month') {
        await apiGenerateBills(pair.accessToken, { due_id: due.id, period: effectivePeriod, due_date: effectiveDueDate }, crypto.randomUUID())
      }
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
      <Modal open={open} onClose={handleClose}>
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
    <Modal open={open} onClose={handleClose} width={520} overlayClassName="sf-modal-overlay-due">
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
          <DueFormFields
            form={form}
            onChange={handleChange}
            disabled={loading}
            period={period}
            dueDate={dueDate}
            onPeriodChange={handlePeriodChange}
            onDueDateChange={handleDueDateChange}
          />
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
