import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import { apiCreateTransaction, getSessionPair } from '@/app/api'
import {
  ArrowUpOutlined,
  CalendarOutlined,
} from '@ant-design/icons'

interface FormState {
  tanggal: string
  nominal: string
  keterangan: string
  referensi: string
  paymentMethod: 'CASH' | 'TRANSFER'
}

const emptyForm: FormState = {
  tanggal: new Date().toISOString().split('T')[0],
  nominal: '',
  keterangan: '',
  referensi: '',
  paymentMethod: 'CASH',
}

export function KasIncome() {
  const { user } = useAuth()
  const navigate = useNavigate()

  if (!user) return null

  const canWrite = hasWargaWriteAccess(user)

  const handleClose = () => navigate('/kas')

  if (!canWrite) {
    return (
      <Modal open onClose={handleClose} width={560}>
        <ModalHeader
          title="Akses Ditolak"
          subtitle="Halaman ini hanya untuk pengurus yang berwenang"
          onClose={handleClose}
        />
        <ModalBody>
          <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', textAlign: 'center', padding: '24px 0' }}>
            Anda tidak memiliki izin untuk menambah transaksi kas masuk.
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>Kembali</button>
        </ModalFooter>
      </Modal>
    )
  }

  const [form, setForm] = useState<FormState>({ ...emptyForm })
  const [saving, setSaving] = useState(false)
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])

    const errs: string[] = []
    if (!form.tanggal) errs.push('Tanggal harus diisi.')
    if (!form.nominal) {
      errs.push('Nominal harus diisi.')
    } else if (isNaN(Number(form.nominal)) || Number(form.nominal) <= 0) {
      errs.push('Nominal harus berupa angka lebih dari 0.')
    }
    if (!form.keterangan.trim()) errs.push('Keterangan harus diisi.')
    if (errs.length > 0) {
      setValidationErrors(errs)
      return
    }

    setSaving(true)
    try {
      const token = getSessionPair()?.accessToken ?? ''
      await apiCreateTransaction(token, {
        amount: form.nominal,
        type: 'income',
        description: form.referensi.trim() ? `${form.keterangan.trim()} (Ref: ${form.referensi.trim()})` : form.keterangan.trim(),
        occurred_at: form.tanggal,
        payment_method: form.paymentMethod,
      })

      navigate('/kas')
    } catch (err: any) {
      setValidationErrors([err?.message ?? 'Gagal menyimpan transaksi'])
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title="Tambah Kas Masuk"
        subtitle="Catat pemasukan kas RT"
        onClose={handleClose}
      />
      <form onSubmit={handleSubmit}>
        <ModalBody>
          {/* Type Toggle Tabs */}
          <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
            <button
              type="button"
              className="sf-btn"
              style={{
                flex: 1,
                height: 38,
                minHeight: 38,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: 'var(--sf-success)',
                color: '#fff',
                fontWeight: 600,
                fontSize: 13,
                cursor: 'default',
              }}
            >
              Pemasukan
            </button>
            <button
              type="button"
              className="sf-btn sf-btn-outline"
              onClick={() => navigate('/kas/baru/keluar')}
              style={{
                flex: 1,
                height: 38,
                minHeight: 38,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 13,
                fontWeight: 600,
              }}
            >
              Pengeluaran
            </button>
          </div>

          {validationErrors.length > 0 && (
            <div className="sf-form-error-list">
              <ul>{validationErrors.map((e, i) => <li key={i}>{e}</li>)}</ul>
            </div>
          )}

          {/* Tanggal */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ki-tanggal">
              <CalendarOutlined style={{ marginRight: 4 }} />
              Tanggal <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="ki-tanggal"
              className="sf-form-input"
              type="date"
              value={form.tanggal}
              onChange={(e) => handleChange('tanggal', e.target.value)}
              required
              autoFocus
            />
          </div>

          {/* Metode Pembayaran */}
          <div className="sf-form-field">
            <label className="sf-form-label">
              Metode Pembayaran <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <div style={{ display: 'flex', gap: 8 }}>
              <button
                type="button"
                className="sf-btn"
                onClick={() => handleChange('paymentMethod', 'CASH')}
                style={{
                  flex: 1,
                  height: 38,
                  minHeight: 38,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  padding: '6px 12px',
                  fontSize: 13,
                  fontWeight: 600,
                  background: form.paymentMethod === 'CASH' ? 'rgba(169, 112, 255, 0.12)' : 'var(--sf-surface)',
                  color: form.paymentMethod === 'CASH' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                  border: form.paymentMethod === 'CASH' ? '1px solid rgba(169, 112, 255, 0.5)' : '1px solid var(--sf-border)',
                  boxShadow: form.paymentMethod === 'CASH' ? '0 0 8px rgba(169, 112, 255, 0.15)' : 'none',
                  transition: 'all 0.2s ease',
                }}
              >
                💵 Tunai (Cash)
              </button>
              <button
                type="button"
                className="sf-btn"
                onClick={() => handleChange('paymentMethod', 'TRANSFER')}
                style={{
                  flex: 1,
                  height: 38,
                  minHeight: 38,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  padding: '6px 12px',
                  fontSize: 13,
                  fontWeight: 600,
                  background: form.paymentMethod === 'TRANSFER' ? 'rgba(169, 112, 255, 0.12)' : 'var(--sf-surface)',
                  color: form.paymentMethod === 'TRANSFER' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                  border: form.paymentMethod === 'TRANSFER' ? '1px solid rgba(169, 112, 255, 0.5)' : '1px solid var(--sf-border)',
                  boxShadow: form.paymentMethod === 'TRANSFER' ? '0 0 8px rgba(169, 112, 255, 0.15)' : 'none',
                  transition: 'all 0.2s ease',
                }}
              >
                🏦 Transfer Bank
              </button>
            </div>
          </div>

          {/* Nominal */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ki-nominal">
              <ArrowUpOutlined style={{ marginRight: 4 }} />
              Nominal <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="ki-nominal"
              className="sf-form-input"
              type="number"
              value={form.nominal}
              onChange={(e) => handleChange('nominal', e.target.value)}
              placeholder="Contoh: 500000"
              required
              min="1"
            />
          </div>

          {/* Keterangan */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ki-keterangan">
              Keterangan <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <textarea
              id="ki-keterangan"
              className="sf-form-input"
              rows={3}
              value={form.keterangan}
              onChange={(e) => handleChange('keterangan', e.target.value)}
              placeholder="Deskripsi transaksi kas masuk"
              required
              style={{ resize: 'vertical' }}
            />
          </div>

          {/* Referensi (optional) — stored in description for simplicity */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ki-referensi">
              Referensi
            </label>
            <input
              id="ki-referensi"
              className="sf-form-input"
              type="text"
              value={form.referensi}
              onChange={(e) => handleChange('referensi', e.target.value)}
              placeholder="Referensi pembayaran (opsional)"
            />
          </div>
        </ModalBody>
        <ModalFooter>
          <button
            type="button"
            className="sf-btn"
            onClick={handleClose}
            style={{
              background: 'transparent',
              border: '1px solid var(--sf-border)',
              color: 'var(--sf-text-muted)',
              padding: '7px 16px',
              borderRadius: 'var(--sf-radius-md)',
              fontSize: 13,
              fontWeight: 500,
            }}
          >
            Batal
          </button>
          <button
            type="submit"
            className="sf-btn"
            style={{
              background: 'rgba(115, 191, 105, 0.16)',
              border: '1px solid rgba(115, 191, 105, 0.6)',
              color: 'var(--sf-success)',
              boxShadow: '0 0 12px rgba(115, 191, 105, 0.15)',
              fontWeight: 600,
              padding: '7px 18px',
              borderRadius: 'var(--sf-radius-md)',
              fontSize: 13,
              transition: 'all 0.2s ease',
            }}
            disabled={saving}
          >
            {saving ? 'Menyimpan...' : 'Simpan'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
