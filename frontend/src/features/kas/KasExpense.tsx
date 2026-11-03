import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { DatePicker } from 'antd'
import dayjs from 'dayjs'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import { apiCreateTransaction, apiListFundLocations, getSessionPair, type FundLocation } from '@/app/api'
import { toJakartaIso8601, getJakartaDateString } from '@/utils/jakartaTime'
import { formatRibuan, parseMoney } from '../iuran/iuranTypes'
import { EXPENSE_CATEGORIES } from './kasTypes'
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  CalendarOutlined,
  WalletOutlined,
  FileImageOutlined,
  UploadOutlined,
  DeleteOutlined,
} from '@ant-design/icons'

interface FormState {
  tanggal: string
  nominal: string
  kategori: string
  keterangan: string
  paymentMethod: 'CASH' | 'TRANSFER'
  jenis: 'masuk' | 'keluar'
  locationId: string
  expenseCategory: string
}

const emptyForm: FormState = {
  tanggal: getJakartaDateString(),
  nominal: '',
  kategori: '',
  keterangan: '',
  paymentMethod: 'CASH',
  jenis: 'keluar',
  locationId: '',
  expenseCategory: '',
}

export function KasExpense() {
  const { user } = useAuth()
  const navigate = useNavigate()

  if (!user) return null

  const canWrite = hasWargaWriteAccess(user)

  const handleClose = () => navigate('/kas')

  // Category state
  interface Category {
    id: string
    name: string
    type: string
  }
  const [categories, setCategories] = useState<Category[]>([])

  useEffect(() => {
    const fetchCategories = async () => {
      try {
        const token = getSessionPair()?.accessToken ?? ''
        const res = await fetch('/api/v1/categories', {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (res.ok) {
          const data = await res.json()
          setCategories(Array.isArray(data) ? data : (data.data ?? []))
        }
      } catch {
        // ignore
      }
    }
    fetchCategories()
  }, [])

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
  const [fundLocations, setFundLocations] = useState<FundLocation[]>([])

  useEffect(() => {
    const token = getSessionPair()?.accessToken
    if (token) {
      apiListFundLocations(token, { is_active: true })
        .then(setFundLocations)
        .catch(() => {})
    }
  }, [])

  const handleChange = (field: string, value: string) => {
    if (field === 'locationId') {
      const loc = fundLocations.find((l) => l.id === value)
      let autoMethod: 'CASH' | 'TRANSFER' = 'CASH'
      if (loc) {
        if (loc.category === 'rekening') {
          autoMethod = 'TRANSFER'
        } else if (loc.category === 'coh' || loc.category === 'brankas') {
          autoMethod = 'CASH'
        }
      }
      setForm((prev) => ({ ...prev, locationId: value, paymentMethod: autoMethod }))
      if (validationErrors.length > 0) setValidationErrors([])
      return
    }
    setForm((prev) => ({ ...prev, [field]: value }))
    if (validationErrors.length > 0) setValidationErrors([])
  }

  const handleJenisChange = (jenis: 'masuk' | 'keluar') => {
    setForm((prev) => ({ ...prev, jenis }))
    if (jenis === 'masuk') {
      navigate('/kas/baru')
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
    const selectedCategory = form.kategori || form.expenseCategory
    if (!selectedCategory) {
      errs.push('Kategori harus dipilih.')
    }
    if (!form.keterangan.trim()) {
      errs.push('Keterangan harus diisi.')
    } else if (selectedCategory === 'Lain-lain' && !form.keterangan.trim()) {
      errs.push('Keterangan wajib diisi untuk kategori Lain-lain.')
    }
    if (form.jenis === 'keluar' && form.locationId) {
      const selected = fundLocations.find((l) => l.id === form.locationId)
      if (selected) {
        const bal = parseMoney(selected.current_balance) || 0
        const nom = parseMoney(form.nominal) || 0
        if (nom > bal) {
          errs.push(`Saldo pada ${selected.name} tidak mencukupi (Tersedia: Rp ${formatRibuan(bal)}).`)
        }
      }
    }
    if (errs.length > 0) {
      setValidationErrors(errs)
      return
    }

    setSaving(true)
    try {
      const token = getSessionPair()?.accessToken ?? ''
      const cat = categories.find(
        (c) => c.name.toLowerCase() === selectedCategory.toLowerCase(),
      ) || categories.find((c) => c.type === 'expense')

      let paymentMethod = form.paymentMethod
      if (form.locationId) {
        const loc = fundLocations.find((l) => l.id === form.locationId)
        if (loc) {
          if (loc.category === 'rekening') {
            paymentMethod = 'TRANSFER'
          } else if (loc.category === 'coh' || loc.category === 'brankas') {
            paymentMethod = 'CASH'
          }
        }
      }

      await apiCreateTransaction(token, {
        amount: String(parseMoney(form.nominal)),
        type: form.jenis === 'masuk' ? 'income' : 'expense',
        category_id: cat?.id,
        description: form.keterangan.trim(),
        occurred_at: toJakartaIso8601(form.tanggal),
        payment_method: paymentMethod,
        location_id: form.locationId || undefined,
        expense_category: selectedCategory || undefined,
      })
      navigate('/kas')
    } catch (err: any) {
      setValidationErrors([err?.message ?? 'Gagal menyimpan transaksi'])
    } finally {
      setSaving(false)
    }
  }

  const isJenisKeluar = form.jenis === 'keluar'

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title={isJenisKeluar ? 'Tambah Kas Keluar' : 'Tambah Kas Masuk'}
        subtitle={isJenisKeluar ? 'Catat pengeluaran kas RT' : 'Catat pemasukan kas RT'}
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
                background: isJenisKeluar ? 'var(--sf-surface)' : 'rgba(115, 191, 105, 0.16)',
                color: isJenisKeluar ? 'var(--sf-text-muted)' : 'var(--sf-success)',
                border: isJenisKeluar ? '1px solid var(--sf-border)' : '1px solid rgba(115, 191, 105, 0.6)',
                boxShadow: !isJenisKeluar ? '0 0 8px rgba(115, 191, 105, 0.15)' : 'none',
                fontWeight: 600,
                fontSize: 13,
                cursor: 'pointer',
                transition: 'all 150ms ease',
              }}
            >
              <ArrowDownOutlined style={{ fontSize: 12, color: 'inherit' }} />
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
                background: isJenisKeluar
                  ? 'rgba(242, 73, 92, 0.16)'
                  : 'var(--sf-surface)',
                color: isJenisKeluar ? 'var(--sf-danger)' : 'var(--sf-text-muted)',
                border: isJenisKeluar
                  ? '1px solid rgba(242, 73, 92, 0.6)'
                  : '1px solid var(--sf-border)',
                boxShadow: isJenisKeluar ? '0 0 8px rgba(242, 73, 92, 0.15)' : 'none',
                fontWeight: 600,
                fontSize: 13,
                cursor: 'pointer',
                transition: 'all 150ms ease',
              }}
            >
              <ArrowUpOutlined style={{ fontSize: 12, color: 'inherit' }} />
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
            <label className="sf-form-label" htmlFor="ke-tanggal">
              <CalendarOutlined style={{ marginRight: 4 }} />
              Tanggal <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <DatePicker
              id="ke-tanggal"
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

          {/* Lokasi Dana */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-location">
              Lokasi Dana
            </label>
            <select
              id="ke-location"
              className="sf-select"
              value={form.locationId}
              onChange={(e) => handleChange('locationId', e.target.value)}
              style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
            >
              <option value="">-- Tanpa Lokasi Dana (Kas Umum) --</option>
              {fundLocations.map((loc) => (
                <option key={loc.id} value={loc.id}>
                  {loc.name} ({loc.category.toUpperCase()}) - Saldo: Rp {formatRibuan(loc.current_balance)}
                </option>
              ))}
            </select>
            {form.jenis === 'keluar' && form.locationId && (() => {
              const sel = fundLocations.find((l) => l.id === form.locationId)
              if (sel && parseMoney(form.nominal) > (parseMoney(sel.current_balance) || 0)) {
                return (
                  <div style={{ color: 'var(--sf-danger)', fontSize: 12, marginTop: 4 }}>
                    Peringatan: Saldo {sel.name} tidak mencukupi (Rp {formatRibuan(sel.current_balance)}).
                  </div>
                )
              }
              return null
            })()}
          </div>

          {/* Kategori (Wajib, 15 Standar Kategori) */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-kategori">
              <WalletOutlined style={{ marginRight: 4 }} />
              Kategori <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <select
              id="ke-kategori"
              className="sf-form-input"
              value={form.kategori || form.expenseCategory}
              onChange={(e) => {
                const val = e.target.value
                setForm((prev) => ({ ...prev, kategori: val, expenseCategory: val }))
                setValidationErrors([])
              }}
              style={{
                cursor: 'pointer',
              }}
            >
              <option value="">-- Pilih Kategori Pengeluaran --</option>
              {EXPENSE_CATEGORIES.map((cat) => (
                <option key={cat} value={cat}>
                  {cat}
                </option>
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
              type="text"
              inputMode="numeric"
              value={form.nominal ? formatRibuan(form.nominal) : ''}
              onChange={(e) => {
                const rawDigits = e.target.value.replace(/\D/g, '')
                handleChange('nominal', rawDigits)
                setValidationErrors([])
              }}
              placeholder="Contoh: 150.000"
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
              placeholder="Deskripsi transaksi kas"
              required
              style={{ resize: 'vertical' }}
            />
          </div>

          {/* Bukti Bayar (Opsional) */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ke-proof">
              <FileImageOutlined style={{ marginRight: 4 }} />
              Bukti Bayar
            </label>

            {!proofFile ? (
              <div>
                <label
                  htmlFor="ke-proof"
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
                  id="ke-proof"
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