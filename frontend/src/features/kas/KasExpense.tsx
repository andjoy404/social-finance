import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import {
  SearchOutlined,
  ArrowDownOutlined,
  CalendarOutlined,
} from '@ant-design/icons'
import type { KasTransaction } from './kasTypes'
import {
  KAS_KATEGORI_OPTIONS,
} from './kasTypes'

interface FormState {
  tanggal: string
  kategori: string
  nominal: string
  keterangan: string
  referensi: string
}

const emptyForm: FormState = {
  tanggal: new Date().toISOString().split('T')[0],
  kategori: '',
  nominal: '',
  keterangan: '',
  referensi: '',
}

export function KasExpense() {
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
            Anda tidak memiliki izin untuk menambah transaksi kas keluar.
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Kembali
          </button>
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])

    const errs: string[] = []
    if (!form.tanggal) errs.push('Tanggal harus diisi.')
    if (!form.kategori) errs.push('Kategori harus dipilih.')
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
    // Mock save — simulate delay
    setTimeout(() => {
      const newTransaction: KasTransaction = {
        id: `kas-new-${Date.now()}`,
        tanggal: form.tanggal,
        jenis: 'keluar' as const,
        kategori: form.kategori,
        keterangan: form.keterangan.trim(),
        referensi: form.referensi.trim() || undefined,
        nominal: Number(form.nominal),
        saldo: 0,
      }
      console.log('Mock save KasExpense:', newTransaction)
      setSaving(false)
      navigate('/kas')
    }, 300)
  }

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title="Tambah Kas Keluar"
        subtitle="Catat pengeluaran kas RT"
        onClose={handleClose}
      />
      <form onSubmit={handleSubmit}>
        <ModalBody>
          {validationErrors.length > 0 && (
            <div className="sf-form-error-list">
              <ul>{validationErrors.map((e, i) => <li key={i}>{e}</li>)}</ul>
            </div>
          )}

          {/* Tanggal */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-tanggal">
              <CalendarOutlined style={{ marginRight: 4 }} />
              Tanggal <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="ke-tanggal"
              className="sf-form-input"
              type="date"
              value={form.tanggal}
              onChange={(e) => handleChange('tanggal', e.target.value)}
              required
              autoFocus
            />
          </div>

          {/* Kategori */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-kategori">
              <SearchOutlined style={{ marginRight: 4 }} />
              Kategori <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <select
              id="ke-kategori"
              className="sf-form-select"
              value={form.kategori}
              onChange={(e) => handleChange('kategori', e.target.value)}
              required
            >
              <option value="">Pilih kategori...</option>
              {KAS_KATEGORI_OPTIONS.filter((o) => o.value !== 'Iuran Warga' && o.value !== 'Iuran Paksa').map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>

          {/* Nominal */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-nominal">
              <ArrowDownOutlined style={{ marginRight: 4 }} />
              Nominal <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="ke-nominal"
              className="sf-form-input"
              type="number"
              value={form.nominal}
              onChange={(e) => handleChange('nominal', e.target.value)}
              placeholder="Contoh: 150000"
              required
              min="1"
            />
          </div>

          {/* Keterangan */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-keterangan">
              Keterangan <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <textarea
              id="ke-keterangan"
              className="sf-form-input"
              rows={3}
              value={form.keterangan}
              onChange={(e) => handleChange('keterangan', e.target.value)}
              placeholder="Deskripsi transaksi kas keluar"
              required
              style={{ resize: 'vertical' }}
            />
          </div>

          {/* Referensi (optional) */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-referensi">
              Referensi
            </label>
            <input
              id="ke-referensi"
              className="sf-form-input"
              type="text"
              value={form.referensi}
              onChange={(e) => handleChange('referensi', e.target.value)}
              placeholder="Referensi pembayaran (opsional)"
            />
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Batal
          </button>
          <button
            type="submit"
            className="sf-btn-primary"
            disabled={saving}
          >
            {saving ? 'Menyimpan...' : 'Simpan'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
