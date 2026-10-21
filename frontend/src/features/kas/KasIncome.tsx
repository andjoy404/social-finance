import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { DatePicker } from 'antd'
import dayjs from 'dayjs'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import { apiCreateTransaction, getSessionPair } from '@/app/api'
import { toJakartaIso8601, getJakartaDateString } from '@/utils/jakartaTime'
import { formatRibuan, parseMoney } from '../iuran/iuranTypes'
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  CalendarOutlined,
  WalletOutlined,
  BankOutlined,
  FileImageOutlined,
  UploadOutlined,
  DeleteOutlined,
} from '@ant-design/icons'

interface FormState {
  tanggal: string
  nominal: string
  keterangan: string
  paymentMethod: 'CASH' | 'TRANSFER'
  jenis: 'masuk' | 'keluar'
}

const emptyForm: FormState = {
  tanggal: getJakartaDateString(),
  nominal: '',
  keterangan: '',
  paymentMethod: 'CASH',
  jenis: 'masuk',
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

  // Bukti bayar (opsional, tidak dikirim ke backend karena contract API transaksi tidak mendukung file)
  const [proofFile, setProofFile] = useState<File | null>(null)
  const [proofPreviewUrl, setProofPreviewUrl] = useState<string | null>(null)

  const handleChange = (field: string, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const handleJenisChange = (jenis: 'masuk' | 'keluar') => {
    setForm((prev) => ({ ...prev, jenis }))
    if (jenis === 'keluar') {
      navigate('/kas/baru/keluar')
    }
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      setValidationErrors(['Format file bukti harus berupa gambar (JPG, PNG, atau WebP).'])
      e.target.value = ''
      return
    }

    if (file.size > 5 * 1024 * 1024) {
      setValidationErrors(['Ukuran file bukti tidak boleh melebihi 5 MB.'])
      e.target.value = ''
      return
    }

    if (proofPreviewUrl) {
      URL.revokeObjectURL(proofPreviewUrl)
    }

    setProofFile(file)
    setProofPreviewUrl(URL.createObjectURL(file))
    setValidationErrors([])
  }

  const handleRemoveFile = () => {
    if (proofPreviewUrl) {
      URL.revokeObjectURL(proofPreviewUrl)
    }
    setProofFile(null)
    setProofPreviewUrl(null)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])

    const errs: string[] = []
    if (!form.tanggal) errs.push('Tanggal harus diisi.')
    if (!form.nominal) {
      errs.push('Nominal harus diisi.')
    } else {
      const nominalNum = parseMoney(form.nominal)
      if (isNaN(nominalNum) || nominalNum <= 0) {
        errs.push('Nominal harus berupa angka lebih dari 0.')
      }
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
        amount: String(parseMoney(form.nominal)),
        type: form.jenis === 'masuk' ? 'income' : 'expense',
        description: form.keterangan.trim(),
        occurred_at: toJakartaIso8601(form.tanggal),
        payment_method: form.paymentMethod,
      })

      navigate('/kas')
    } catch (err: any) {
      setValidationErrors([err?.message ?? 'Gagal menyimpan transaksi'])
    } finally {
      setSaving(false)
    }
  }

  const isJenisMasuk = form.jenis === 'masuk'

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title={isJenisMasuk ? 'Tambah Kas Masuk' : 'Tambah Kas Keluar'}
        subtitle={isJenisMasuk ? 'Catat pemasukan kas RT' : 'Catat pengeluaran kas RT'}
        onClose={handleClose}
      />
      <form onSubmit={handleSubmit}>
        <ModalBody>
          {/* Type Toggle */}
          <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
            <button
              type="button"
              className="sf-btn"
              onClick={() => handleJenisChange('masuk')}
              style={{
                flex: 1,
                height: 38,
                minHeight: 38,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 6,
                background: isJenisMasuk
                  ? 'rgba(115, 191, 105, 0.16)'
                  : 'var(--sf-surface)',
                color: isJenisMasuk ? 'var(--sf-success)' : 'var(--sf-text-muted)',
                border: isJenisMasuk
                  ? '1px solid rgba(115, 191, 105, 0.6)'
                  : '1px solid var(--sf-border)',
                boxShadow: isJenisMasuk ? '0 0 8px rgba(115, 191, 105, 0.15)' : 'none',
                fontWeight: 600,
                fontSize: 13,
                cursor: 'pointer',
                transition: 'all 150ms ease',
              }}
            >
              <ArrowUpOutlined style={{ fontSize: 12, color: 'inherit' }} />
              Pemasukan
            </button>
            <button
              type="button"
              className="sf-btn"
              onClick={() => handleJenisChange('keluar')}
              style={{
                flex: 1,
                height: 38,
                minHeight: 38,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 6,
                background: !isJenisMasuk
                  ? 'rgba(242, 73, 92, 0.16)'
                  : 'var(--sf-surface)',
                color: !isJenisMasuk ? 'var(--sf-danger)' : 'var(--sf-text-muted)',
                border: !isJenisMasuk
                  ? '1px solid rgba(242, 73, 92, 0.6)'
                  : '1px solid var(--sf-border)',
                boxShadow: !isJenisMasuk ? '0 0 8px rgba(242, 73, 92, 0.15)' : 'none',
                fontWeight: 600,
                fontSize: 13,
                cursor: 'pointer',
                transition: 'all 150ms ease',
              }}
            >
              <ArrowDownOutlined style={{ fontSize: 12, color: 'inherit' }} />
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
            <DatePicker
              id="ki-tanggal"
              picker="date"
              value={form.tanggal ? dayjs(form.tanggal) : null}
              onChange={(date) => handleChange('tanggal', date?.format('YYYY-MM-DD') || '')}
              allowClear={false}
              placeholder="DD/MM/YYYY"
              format="DD/MM/YYYY"
              disabled={saving}
              style={{ width: '100%' }}
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
                  background: form.paymentMethod === 'CASH' ? 'rgba(87, 148, 242, 0.16)' : 'var(--sf-surface)',
                  color: form.paymentMethod === 'CASH' ? 'var(--sf-info)' : 'var(--sf-text-muted)',
                  border: form.paymentMethod === 'CASH' ? '1px solid rgba(87, 148, 242, 0.6)' : '1px solid var(--sf-border)',
                  boxShadow: form.paymentMethod === 'CASH' ? '0 0 8px rgba(87, 148, 242, 0.15)' : 'none',
                  transition: 'all 0.2s ease',
                }}
              >
                <WalletOutlined style={{ color: form.paymentMethod === 'CASH' ? 'var(--sf-info)' : 'var(--sf-text-muted)' }} /> Tunai (Cash)
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
                <BankOutlined style={{ color: form.paymentMethod === 'TRANSFER' ? 'var(--sf-accent)' : 'var(--sf-text-muted)' }} /> Transfer Bank
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
              type="text"
              inputMode="numeric"
              value={form.nominal ? formatRibuan(form.nominal) : ''}
              onChange={(e) => {
                const rawDigits = e.target.value.replace(/\D/g, '')
                handleChange('nominal', rawDigits)
                setValidationErrors([])
              }}
              placeholder="Contoh: 500.000"
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
              placeholder="Deskripsi transaksi kas"
              required
              style={{ resize: 'vertical' }}
            />
          </div>

          {/* Bukti Bayar (Opsional) */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ki-proof">
              <FileImageOutlined style={{ marginRight: 4 }} />
              Bukti Bayar
            </label>

            {!proofFile ? (
              <div>
                <label
                  htmlFor="ki-proof"
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    gap: '8px',
                    padding: '16px',
                    border: '1.5px dashed var(--sf-border, var(--border-color))',
                    borderRadius: 'var(--sf-radius-sm, 6px)',
                    background: 'var(--sf-surface)',
                    cursor: saving ? 'not-allowed' : 'pointer',
                    fontSize: '13px',
                    color: 'var(--sf-text-muted)',
                    transition: 'border-color 150ms ease, background 150ms ease',
                  }}
                >
                  <UploadOutlined style={{ fontSize: '20px', color: 'var(--sf-accent)' }} />
                  <span style={{ fontWeight: 500, color: 'var(--sf-text)' }}>
                    Pilih Berkas Bukti Bayar
                  </span>
                  <span style={{ fontSize: '11px', color: 'var(--sf-text-muted)' }}>
                    Format JPG, PNG, atau WebP (Maks. 5 MB)
                  </span>
                </label>
                <input
                  id="ki-proof"
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  onChange={handleFileChange}
                  disabled={saving}
                  style={{ display: 'none' }}
                  data-testid="kas-proof-file-input"
                />
                <p style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 6, marginLeft: 2 }}>
                  Bukti bayar bersifat opsional dan tidak disimpan ke sistem transaksi.
                </p>
              </div>
            ) : (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '12px',
                  padding: '12px 14px',
                  border: '1px solid var(--sf-border, var(--border-color))',
                  borderRadius: 'var(--sf-radius-sm, 6px)',
                  background: 'var(--sf-surface)',
                }}
              >
                {proofPreviewUrl && (
                  <img
                    src={proofPreviewUrl}
                    alt="Preview Bukti"
                    style={{
                      width: '48px',
                      height: '48px',
                      objectFit: 'cover',
                      borderRadius: '4px',
                      border: '1px solid var(--sf-border, var(--border-color))',
                    }}
                  />
                )}
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div
                    style={{
                      fontSize: '13px',
                      fontWeight: 500,
                      color: 'var(--sf-text)',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {proofFile.name}
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--sf-text-muted)' }}>
                    {(proofFile.size / 1024).toFixed(1)} KB
                  </div>
                </div>
                <button
                  type="button"
                  onClick={handleRemoveFile}
                  disabled={saving}
                  style={{
                    background: 'none',
                    border: 'none',
                    color: 'var(--sf-danger)',
                    cursor: 'pointer',
                    padding: '6px',
                    fontSize: '14px',
                    display: 'flex',
                    alignItems: 'center',
                  }}
                  title="Hapus file"
                >
                  <DeleteOutlined />
                </button>
              </div>
            )}
          </div>
        </ModalBody>
        <ModalFooter>
          <button
            type="button"
            className="sf-btn"
            onClick={handleClose}
            style={{
              background: 'rgba(242, 73, 92, 0.16)',
              border: '1px solid rgba(242, 73, 92, 0.6)',
              color: 'var(--sf-danger)',
              boxShadow: '0 0 12px rgba(242, 73, 92, 0.15)',
              fontWeight: 600,
              padding: '7px 16px',
              borderRadius: 'var(--sf-radius-md)',
              fontSize: 13,
              transition: 'all 0.2s ease',
            }}
          >
            Batal
          </button>
          <button
            type="submit"
            className="sf-btn"
            style={{
              background: 'rgba(169, 112, 255, 0.16)',
              border: '1px solid rgba(169, 112, 255, 0.6)',
              color: 'var(--sf-accent)',
              boxShadow: '0 0 12px rgba(169, 112, 255, 0.15)',
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