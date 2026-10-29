import { useState, useEffect, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  CheckCircleOutlined,
  DollarOutlined,
  CalendarOutlined,
  FileTextOutlined,
  WarningOutlined,
  UploadOutlined,
  DeleteOutlined,
  FileImageOutlined,
} from '@ant-design/icons'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'
import {
  BACKEND_STATUS_MAP,
  formatRupiah,
  formatRibuan,
  getStatusVariant,
  getStatusLabel,
  parseMoney,
  type IuranBill,
  type IuranStatus,
  type DueType,
} from './iuranTypes'
import {
  apiGetBill,
  apiCreatePayment,
  apiCreateMultiPeriodPayment,
  apiUploadPaymentProof,
  apiListPayments,
  apiListBills,
  type ApiPayment,
  type ApiError,
  getSessionPair,
} from '@/app/api'

// Payment method options
const PAYMENT_METHODS = [
  { value: 'CASH', label: 'Tunai' },
  { value: 'TRANSFER', label: 'Transfer' },
] as const

const MAX_PROOF_SIZE = 5 * 1024 * 1024 // 5 MB
const ALLOWED_MIME_TYPES = ['image/jpeg', 'image/png', 'image/webp']

export interface PayableBillInfo {
  id: string
  period: string
  amount: string
}

function IuranPaymentContent({
  bill,
  payableBills = [],
  remaining,
  handleClose,
  onPaymentSuccess,
}: {
  bill: IuranBill
  payableBills?: PayableBillInfo[]
  remaining: number
  handleClose: () => void
  onPaymentSuccess: (payment: ApiPayment) => void
}) {
  const isMultiMonth = bill.dueType === 'multi_month'
  const isVoluntary = Boolean(bill.isVoluntary || (!isMultiMonth && bill.nominal === 0))
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [warningMessage, setWarningMessage] = useState<string | null>(null)
  const [monthCount, setMonthCount] = useState<number>(() => (isMultiMonth ? 0 : 1))

  const effectiveBills: PayableBillInfo[] = useMemo(() => {
    if (isMultiMonth) {
      return payableBills ?? []
    }
    if (payableBills && payableBills.length > 0) return payableBills
    return [{ id: bill.id, period: bill.periode, amount: String(bill.nominal) }]
  }, [isMultiMonth, payableBills, bill])

  const maxSelectableMonths = isMultiMonth ? Math.min(12, effectiveBills.length) : 1
  const selectedBills = useMemo(() => {
    if (!isMultiMonth) {
      return [{ id: bill.id, period: bill.periode, amount: String(bill.nominal) }]
    }
    if (monthCount === 0 || effectiveBills.length === 0) return []
    return effectiveBills.slice(0, Math.min(monthCount, maxSelectableMonths))
  }, [isMultiMonth, monthCount, effectiveBills, maxSelectableMonths, bill])

  const calculatedTotal = useMemo(() => {
    if (isVoluntary) return remaining
    if (isMultiMonth) {
      if (monthCount === 0 || selectedBills.length === 0) return 0
      return selectedBills.reduce((sum, b) => sum + parseMoney(b.amount), 0)
    }
    return remaining > 0 ? remaining : bill.nominal
  }, [isVoluntary, isMultiMonth, monthCount, selectedBills, remaining, bill.nominal])

  const [nominal, setNominal] = useState(() => {
    if (isMultiMonth) return '0'
    return remaining > 0 ? String(remaining) : ''
  })
  const [payDate, setPayDate] = useState(() => new Date().toISOString().split('T')[0])
  const [catatan, setCatatan] = useState('')
  const [method, setMethod] = useState<'CASH' | 'TRANSFER' | ''>('')
  const [proofFile, setProofFile] = useState<File | null>(null)
  const [proofPreviewUrl, setProofPreviewUrl] = useState<string | null>(null)
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [successPayment, setSuccessPayment] = useState<ApiPayment | null>(null)

  useEffect(() => {
    if (isMultiMonth) {
      setNominal(String(calculatedTotal))
    }
  }, [calculatedTotal, isMultiMonth])

  // Clean up object URL on unmount or file change
  useEffect(() => {
    return () => {
      if (proofPreviewUrl) {
        URL.revokeObjectURL(proofPreviewUrl)
      }
    }
  }, [proofPreviewUrl])

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (!ALLOWED_MIME_TYPES.includes(file.type)) {
      setValidationErrors(['Format file bukti harus berupa gambar (JPG, PNG, atau WebP).'])
      e.target.value = ''
      return
    }

    if (file.size > MAX_PROOF_SIZE) {
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

  const nominalNum = nominal.trim() ? parseMoney(nominal) : 0

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])
    setError(null)
    setWarningMessage(null)

    const errs: string[] = []
    if (isMultiMonth) {
      if (effectiveBills.length === 0) {
        errs.push('Tidak ada tagihan yang dapat dibayar untuk iuran ini.')
      } else if (monthCount === 0) {
        errs.push('Silakan pilih jumlah bulan yang ingin dibayar (minimal 1 bulan).')
      } else if (nominalNum <= 0) {
        errs.push('Nominal pembayaran harus lebih dari 0.')
      } else if (nominalNum !== calculatedTotal) {
        errs.push(`Nominal pembayaran untuk ${selectedBills.length} bulan harus sesuai (${formatRupiah(calculatedTotal)}).`)
      }
    } else {
      if (!nominal.trim()) {
        errs.push('Nominal pembayaran harus diisi.')
      } else if (isNaN(nominalNum) || nominalNum <= 0) {
        errs.push('Nominal pembayaran harus lebih dari 0.')
      } else if (!isVoluntary && nominalNum > remaining) {
        errs.push(`Nominal pembayaran melebihi sisa tagihan (${formatRupiah(remaining)}).`)
      }
    }
    if (!payDate) {
      errs.push('Tanggal pembayaran harus diisi.')
    }
    if (!method) {
      errs.push('Metode pembayaran harus dipilih.')
    }
    if (proofFile) {
      if (!ALLOWED_MIME_TYPES.includes(proofFile.type)) {
        errs.push('Format file bukti harus berupa gambar (JPG, PNG, atau WebP).')
      }
      if (proofFile.size > MAX_PROOF_SIZE) {
        errs.push('Ukuran file bukti tidak boleh melebihi 5 MB.')
      }
    }
    if (errs.length > 0) {
      setValidationErrors(errs)
      return
    }

    setSubmitting(true)

    try {
      const pair = getSessionPair()
      if (!pair?.accessToken) {
        setError('Sesi Anda telah berakhir. Silakan login ulang.')
        setSubmitting(false)
        return
      }

      // Create payment (multi-period if multi_month and > 1 bill, single payment otherwise)
      let payment: ApiPayment
      if (isMultiMonth) {
        if (selectedBills.length > 1) {
          const multiRes = await apiCreateMultiPeriodPayment(pair.accessToken, {
            bill_ids: selectedBills.map((b) => b.id),
            allocations: selectedBills.map((b) => ({ bill_id: b.id, amount: b.amount })),
            amount: String(nominalNum),
            method: method as 'CASH' | 'TRANSFER',
            notes: catatan || undefined,
            file: proofFile || undefined,
          })
          payment = multiRes.payment
        } else {
          payment = await apiCreatePayment(pair.accessToken, {
            bill_id: selectedBills[0].id,
            amount: String(nominalNum),
            method: method as 'CASH' | 'TRANSFER',
            notes: catatan || undefined,
            file: proofFile || undefined,
          })
        }
      } else {
        payment = await apiCreatePayment(pair.accessToken, {
          bill_id: bill.id,
          amount: String(nominalNum),
          method: method as 'CASH' | 'TRANSFER',
          notes: catatan || undefined,
          file: proofFile || undefined,
        })
      }

      // If proof file was attached but payment was created without file (or as separate upload)
      if (proofFile && !payment.proof_path) {
        try {
          payment = await apiUploadPaymentProof(pair.accessToken, payment.id, proofFile)
        } catch {
          setWarningMessage('Pembayaran berhasil dicatat, namun bukti pembayaran gagal diunggah.')
        }
      }

      // Success!
      setSuccessPayment(payment)

      // Notify parent to refresh data
      onPaymentSuccess(payment)
    } catch (err) {
      const apiErr = err as ApiError
      if (apiErr.message.includes('exceeds remaining') || apiErr.message.includes('exceeds')) {
        setError('Jumlah pembayaran melebihi sisa tagihan. Pembayaran akan ditolak secara otomatis.')
      } else if (apiErr.message.includes('already paid') || apiErr.message.includes('cancelled')) {
        setError(apiErr.message)
      } else {
        setError(err instanceof Error ? err.message : 'Gagal memproses pembayaran')
      }
      setSubmitting(false)
    }
  }

  // Show success state after payment
  if (successPayment) {
    const isPending = successPayment.status === 'PENDING'
    return (
      <Modal open onClose={handleClose} width={520}>
        <ModalHeader
          title={isPending ? 'Pembayaran Berhasil Dikirim' : 'Pembayaran Diterima'}
          subtitle={`Tagihan ${bill.householdName}`}
          onClose={handleClose}
        />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '20px 0' }}>
            {isPending ? (
              <>
                <CheckCircleOutlined style={{ fontSize: 48, color: 'var(--sf-warning)', marginBottom: 16 }} />
                <p style={{ fontSize: 14, color: 'var(--sf-text)', marginBottom: 8 }}>
                  Pembayaran sebesar <strong>{formatRupiah(parseMoney(successPayment.amount))}</strong> berhasil dikirim.
                </p>
                <p style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
                  Pembayaran Anda sedang menunggu verifikasi bendahara. Status tagihan akan diperbarui setelah pembayaran diverifikasi.
                </p>
              </>
            ) : (
              <>
                <CheckCircleOutlined style={{ fontSize: 48, color: 'var(--sf-success)', marginBottom: 16 }} />
                <p style={{ fontSize: 14, color: 'var(--sf-text)', marginBottom: 8 }}>
                  Pembayaran sebesar <strong>{formatRupiah(parseMoney(successPayment.amount))}</strong> telah diverifikasi.
                </p>
              </>
            )}
            {warningMessage && (
              <p style={{ fontSize: 12, color: 'var(--sf-warning)', marginTop: 8 }}>
                {warningMessage}
              </p>
            )}
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-primary" onClick={handleClose}>
            Tutup
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  return (
    <Modal open onClose={handleClose} width={520}>
      <ModalHeader
        title="Pembayaran Iuran"
        subtitle={`Tagihan ${bill.householdName}`}
        onClose={handleClose}
      />
      <form
        onSubmit={handleSubmit}
        style={{
          display: 'flex',
          flexDirection: 'column',
          flex: 1,
          minHeight: 0,
          overflow: 'hidden',
        }}
      >
        <ModalBody>
          {validationErrors.length > 0 && (
            <div className="sf-form-error-list">
              <ul>{validationErrors.map((e, i) => <li key={i}>{e}</li>)}</ul>
            </div>
          )}
          {error && <div className="sf-form-error-list">{error}</div>}

          {/* Informasi warga */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-household">
              Informasi Warga
            </label>
            <input
              id="ip-household"
              className="sf-form-input"
              value={bill.householdName}
              disabled
            />
          </div>

          {/* RT */}
          <div className="sf-form-field">
            <label className="sf-form-label">RT</label>
            <input className="sf-form-input" value={bill.rt} disabled />
          </div>

          {/* Jenis Iuran */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-type">
              <FileTextOutlined style={{ marginRight: 4 }} />
              Jenis Iuran
            </label>
            <input
              id="ip-type"
              className="sf-form-input"
              value={bill.iuranType}
              disabled
            />
          </div>

          {/* Periode */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-periode">
              <CalendarOutlined style={{ marginRight: 4 }} />
              Periode
            </label>
            <input
              id="ip-periode"
              className="sf-form-input"
              value={
                isMultiMonth
                  ? selectedBills.length > 1
                    ? `${selectedBills[0].period} s/d ${selectedBills[selectedBills.length - 1].period} (${selectedBills.length} Bulan)`
                    : selectedBills.length === 1
                    ? selectedBills[0].period
                    : bill.year ? `Tahun ${bill.year}` : bill.periode
                  : bill.periode
              }
              disabled
            />
          </div>

          {/* Pilihan Jumlah Bulan untuk Multi-Bulan */}
          {isMultiMonth && (
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="ip-month-count">
                <CalendarOutlined style={{ marginRight: 4 }} />
                Jumlah Bulan yang Dibayar
              </label>
              {effectiveBills.length === 0 ? (
                <div
                  style={{
                    padding: '10px 12px',
                    background: 'var(--sf-surface, rgba(0,0,0,0.02))',
                    border: '1px solid var(--sf-border, var(--border-color))',
                    borderRadius: 'var(--sf-radius-sm, 6px)',
                    color: 'var(--sf-text-muted)',
                    fontSize: 13,
                  }}
                >
                  Semua tagihan untuk iuran ini telah lunas atau tidak ada tagihan tertunggak.
                </div>
              ) : (
                <>
                  <select
                    id="ip-month-count"
                    aria-label="Jumlah Bulan yang Dibayar"
                    className="sf-form-input"
                    value={monthCount}
                    onChange={(e) => {
                      const count = Number(e.target.value)
                      setMonthCount(count)
                      setValidationErrors([])
                    }}
                  >
                    <option value={0}>0 Bulan — Pilih jumlah bulan (1-{maxSelectableMonths})</option>
                    {Array.from({ length: maxSelectableMonths }, (_, i) => i + 1).map((num) => (
                      <option key={num} value={num}>
                        {num} Bulan ({num} Tagihan)
                      </option>
                    ))}
                  </select>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4 }}>
                    {monthCount === 0
                      ? 'Silakan pilih jumlah bulan yang ingin dibayar (minimal 1 bulan).'
                      : `Pembayaran ${monthCount} bulan diproses berurutan sesuai aturan FIFO (${selectedBills[0]?.period} s/d ${selectedBills[selectedBills.length - 1]?.period}).`}
                  </div>
                </>
              )}
            </div>
          )}

          {/* Rincian Alokasi jika Multi-Bulan dan > 0 bulan */}
          {isMultiMonth && selectedBills.length > 0 && (
            <div
              style={{
                marginBottom: 'var(--sp-sm)',
                padding: '10px 12px',
                background: 'var(--sf-surface, rgba(0,0,0,0.02))',
                border: '1px solid var(--sf-border, var(--border-color))',
                borderRadius: 'var(--sf-radius-sm, 6px)',
              }}
            >
              <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text)', marginBottom: 6 }}>
                Rincian Tagihan yang Dibayar ({selectedBills.length} Bulan):
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxHeight: 180, overflowY: 'auto' }}>
                {selectedBills.map((b) => (
                  <div key={b.id} style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                    <span style={{ color: 'var(--sf-text-muted)' }}>Periode {b.period}</span>
                    <span style={{ fontWeight: 500, color: 'var(--sf-text)' }}>
                      {formatRupiah(parseMoney(b.amount))}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Status */}
          <div className="sf-form-field">
            <label className="sf-form-label">Status</label>
            <div style={{ padding: '8px 0' }}>
              <Badge variant={getStatusVariant(bill.status)}>
                {getStatusLabel(bill.status)}
              </Badge>
            </div>
          </div>

          {/* Nominal tagihan */}
          <div className="sf-form-grid-2">
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="ip-nominal">
                <DollarOutlined style={{ marginRight: 4 }} />
                Total Tagihan
              </label>
              <input
                id="ip-nominal"
                className="sf-form-input"
                value={formatRupiah(isMultiMonth ? calculatedTotal : remaining > 0 ? remaining : bill.nominal)}
                disabled
              />
            </div>

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="ip-paid">
                <CheckCircleOutlined style={{ marginRight: 4 }} />
                Sudah Dibayar
              </label>
              <input
                id="ip-paid"
                className="sf-form-input"
                value={formatRupiah(bill.paidAmount)}
                disabled
              />
            </div>
          </div>

          {/* Sisa */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-sisa">
              <WarningOutlined style={{ marginRight: 4, color: isVoluntary ? 'var(--sf-accent)' : 'var(--sf-warning)' }} />
              Sisa Tagihan
            </label>
            <input
              id="ip-sisa"
              className="sf-form-input"
              style={{ fontWeight: 700, color: isVoluntary ? 'var(--sf-accent)' : 'var(--sf-warning)' }}
              value={isVoluntary ? 'Sukarela' : formatRupiah(isMultiMonth ? calculatedTotal : remaining)}
              disabled
            />
          </div>

          <hr style={{ border: 'none', borderTop: '1px solid var(--border-color)', margin: 'var(--sp-md) 0' }} />

          {/* Metode pembayaran */}
          <div className="sf-form-field">
            <label className="sf-form-label">
              Metode Pembayaran <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <div style={{ display: 'flex', gap: 'var(--sp-sm)' }}>
              {PAYMENT_METHODS.map((m) => {
                const isSelected = method === m.value
                return (
                  <button
                    key={m.value}
                    type="button"
                    className={`sf-form-input ${isSelected ? 'sf-form-input--active' : ''}`}
                    style={{
                      flex: 1,
                      textAlign: 'center',
                      cursor: 'pointer',
                      border: isSelected
                        ? '1.5px solid var(--sf-accent)'
                        : '1px solid var(--sf-border, var(--border-color))',
                      background: isSelected
                        ? 'var(--sf-accent-subtle, rgba(59, 130, 246, 0.08))'
                        : 'var(--sf-surface, transparent)',
                      color: isSelected ? 'var(--sf-accent)' : 'var(--sf-text)',
                      fontWeight: isSelected ? 600 : 400,
                      borderRadius: 'var(--sf-radius-sm, 6px)',
                      padding: '8px 16px',
                      transition: 'all 150ms ease',
                    }}
                    onClick={() => {
                      setMethod(m.value as 'CASH' | 'TRANSFER')
                      setValidationErrors([])
                    }}
                  >
                    {m.label}
                  </button>
                )
              })}
            </div>
          </div>

          {/* Nominal pembayaran */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-amount">
              Nominal Pembayaran <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="ip-amount"
              className="sf-form-input"
              type="text"
              inputMode="numeric"
              value={formatRibuan(nominal)}
              disabled={isMultiMonth}
              onChange={(e) => {
                if (isMultiMonth) return
                const rawDigits = e.target.value.replace(/\D/g, '')
                setNominal(rawDigits)
                setValidationErrors([])
              }}
              placeholder={isVoluntary ? 'Masukkan nominal sumbangan' : isMultiMonth ? '0' : `Maksimal ${formatRibuan(remaining)}`}
              autoFocus
            />
            {remaining <= 0 && !isVoluntary && !isMultiMonth && (
              <div style={{ fontSize: 12, color: 'var(--sf-success)', marginTop: 4 }}>
                Tagihan sudah lunas.
              </div>
            )}
          </div>

          {/* Tanggal pembayaran */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-date">
              Tanggal Pembayaran <span style={{ color: 'var(--sf-danger)' }}>*</span>
            </label>
            <input
              id="ip-date"
              className="sf-form-input"
              type="date"
              value={payDate}
              onChange={(e) => { setPayDate(e.target.value); setValidationErrors([]) }}
              onClick={(e) => {
                try {
                  (e.currentTarget as HTMLInputElement).showPicker?.()
                } catch {
                  // Fallback for browsers that do not support showPicker or restrict invocation
                }
              }}
              style={{ cursor: 'pointer' }}
              max={new Date().toISOString().split('T')[0]}
            />
          </div>

          {/* Catatan */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-note">
              Catatan
            </label>
            <textarea
              id="ip-note"
              className="sf-form-input"
              rows={3}
              value={catatan}
              onChange={(e) => setCatatan(e.target.value)}
              placeholder="Catatan tambahan (opsional)"
              disabled={submitting}
            />
          </div>

          {/* Bukti Pembayaran */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-proof">
              <FileImageOutlined style={{ marginRight: 4 }} />
              Bukti Pembayaran (Opsional)
            </label>

            {!proofFile ? (
              <div>
                <label
                  htmlFor="ip-proof"
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
                    cursor: submitting ? 'not-allowed' : 'pointer',
                    fontSize: '13px',
                    color: 'var(--sf-text-muted)',
                    transition: 'border-color 150ms ease, background 150ms ease',
                  }}
                >
                  <UploadOutlined style={{ fontSize: '20px', color: 'var(--sf-accent)' }} />
                  <span style={{ fontWeight: 500, color: 'var(--sf-text)' }}>
                    Pilih Berkas Bukti Pembayaran
                  </span>
                  <span style={{ fontSize: '11px', color: 'var(--sf-text-muted)' }}>
                    Format JPG, PNG, atau WebP (Maks. 5 MB)
                  </span>
                </label>
                <input
                  id="ip-proof"
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  onChange={handleFileChange}
                  disabled={submitting}
                  style={{ display: 'none' }}
                />
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
                  disabled={submitting}
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
            className="sf-btn-ghost"
            onClick={handleClose}
            disabled={submitting}
          >
            Batal
          </button>
          <button
            type="submit"
            className="sf-btn-primary"
            disabled={
              submitting ||
              (isMultiMonth && (monthCount === 0 || effectiveBills.length === 0)) ||
              (!isMultiMonth && !isVoluntary && remaining <= 0)
            }
          >
            {submitting ? 'Menyimpan...' : 'Bayar'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}

export function IuranPayment({
  billId,
  open: openProp,
  onClose: onCloseProp,
  onPaymentSuccess: onPaymentSuccessProp,
}: {
  billId?: string
  open?: boolean
  onClose?: () => void
  onPaymentSuccess?: () => void
} = {}) {
  const navigate = useNavigate()
  const routeId = useParams<{ id: string }>()?.id
  const effectiveId = billId ?? routeId
  const isModal = billId !== undefined
  const open = openProp ?? true
  const handleClose = onCloseProp ?? (() => navigate('/iuran'))

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [bill, setBill] = useState<IuranBill | null>(null)
  const [payableBills, setPayableBills] = useState<PayableBillInfo[]>([])
  const [retryCount, setRetryCount] = useState(0)

  const handleRetry = () => {
    setRetryCount((c) => c + 1)
  }

  // Load bill and payments data from API
  useEffect(() => {
    let cancelled = false
    async function loadBill() {
      setLoading(true)
      setError(null)
      setNotFound(false)

      if (!effectiveId) {
        setError('ID tagihan tidak valid')
        setLoading(false)
        return
      }

      try {
        const token = getSessionPair()?.accessToken
        if (!token) {
          setError('Sesi Anda telah berakhir. Silakan login ulang.')
          setLoading(false)
          return
        }

        const apiBill = await apiGetBill(token, effectiveId)
        if (cancelled) return

        const nominal = parseMoney(apiBill.amount)
        const isMultiMonth = apiBill.due_type === 'multi_month'
        const isVoluntary = Boolean(apiBill.is_voluntary || (!isMultiMonth && nominal === 0))
        const status: IuranStatus = (isVoluntary && nominal === 0)
          ? 'sukarela'
          : (BACKEND_STATUS_MAP[apiBill.status] ?? 'belum_bayar')

        // Map backend bill to frontend format
        const mappedBill: IuranBill = {
          id: apiBill.id,
          householdId: apiBill.household_occupancy_id,
          householdName: apiBill.head_name ?? apiBill.house_number ?? 'Unknown',
          rt: apiBill.rt_code ? `RT ${apiBill.rt_code}` : 'RT —',
          iuranType: apiBill.due_name ?? 'Iuran',
          periode: apiBill.period,
          nominal,
          paidAmount: 0, // calculated separately from payments
          pendingAmount: 0, // calculated separately from PENDING payments
          status,
          isVoluntary,
          dueId: apiBill.due_id,
          dueIsActive: apiBill.due_is_active ?? true,
          isActive: apiBill.is_active ?? true,
          dueType: (apiBill.due_type as DueType) ?? (isVoluntary ? 'voluntary' : 'regular'),
          year: apiBill.due_year ?? undefined,
          payments: [],
        }

        // Load payments for this bill to calculate paidAmount + pendingAmount
        try {
          const paymentsResponse = await apiListPayments(token, { bill_id: effectiveId })
          if (cancelled) return

          // Calculate paidAmount from APPROVED payments only
          const approvedPayments = paymentsResponse.data.filter(
            (p) => p.status === 'APPROVED'
          )
          const paidAmount = approvedPayments.reduce(
            (sum, p) => sum + parseMoney(p.amount),
            0
          )

          // Calculate pendingAmount from PENDING payments only
          const pendingPayments = paymentsResponse.data.filter(
            (p) => p.status === 'PENDING'
          )
          const pendingAmount = pendingPayments.reduce(
            (sum, p) => sum + parseMoney(p.amount),
            0
          )

          mappedBill.paidAmount = paidAmount
          mappedBill.pendingAmount = pendingAmount
        } catch {
          // Silent fail — amounts tetap 0 jika payments API gagal
        }

        setBill(mappedBill)

        // Load payable bills for this household and due for multi-period selection
        let payable: PayableBillInfo[] = []
        if (isMultiMonth) {
          if (apiBill.household_occupancy_id && apiBill.due_id) {
            try {
              const listRes = await apiListBills(token, {
                household_occupancy_id: apiBill.household_occupancy_id,
                due_id: apiBill.due_id,
                status: 'unpaid',
                is_active: true,
                page_size: 50,
              })
              if (!cancelled && listRes?.data) {
                const matched = listRes.data.filter(
                  (b) =>
                    b.household_occupancy_id === apiBill.household_occupancy_id &&
                    b.due_id === apiBill.due_id &&
                    b.status === 'unpaid' &&
                    b.is_active !== false
                )
                if (apiBill.status === 'unpaid' && apiBill.is_active !== false && !matched.some((b) => b.id === apiBill.id)) {
                  matched.push(apiBill)
                }
                matched.sort((a, b) => a.period.localeCompare(b.period))
                payable = matched.map((b) => ({
                  id: b.id,
                  period: b.period,
                  amount: b.amount,
                }))
              }
            } catch {
              // Silent fail — fallback to empty list
            }
          }
        } else {
          payable = [{ id: apiBill.id, period: apiBill.period, amount: apiBill.amount }]
        }
        if (!cancelled) {
          setPayableBills(payable)
        }
      } catch (err) {
        if (cancelled) return
        const apiErr = err as { code?: string }
        if (apiErr.code === 'not_found') {
          setNotFound(true)
        } else {
          setError(err instanceof Error ? err.message : 'Gagal memuat data tagihan')
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    loadBill()
    return () => {
      cancelled = true
    }
  }, [effectiveId, retryCount])

  const remaining = useMemo(() => {
    if (!bill) return 0
    return bill.nominal - bill.paidAmount - bill.pendingAmount
  }, [bill])

  const paymentContent = !error && bill ? (
    <IuranPaymentContent
      key={bill.id}
      bill={bill}
      payableBills={payableBills}
      remaining={remaining}
      handleClose={handleClose}
      onPaymentSuccess={async (payment: ApiPayment) => {
        // Refresh bill detail state locally to update paidAmount + pendingAmount + status
        if (onPaymentSuccessProp) {
          void payment // used for typing; parent reloads from API
          onPaymentSuccessProp()
        }
        // Notify parent (IuranList) to reload data from API
        try {
          const token = getSessionPair()?.accessToken
          if (token && effectiveId) {
            const apiBill = await apiGetBill(token, effectiveId)
            try {
              const paymentsResponse = await apiListPayments(token, { bill_id: effectiveId })
              const approvedPayments = paymentsResponse.data.filter(
                (p) => p.status === 'APPROVED'
              )
              const paidAmount = approvedPayments.reduce(
                (sum, p) => sum + parseMoney(p.amount),
                0
              )
              const pendingPayments = paymentsResponse.data.filter(
                (p) => p.status === 'PENDING'
              )
              const pendingAmount = pendingPayments.reduce(
                (sum, p) => sum + parseMoney(p.amount),
                0
              )
              setBill((prev) =>
                prev ? {
                  ...prev,
                  paidAmount,
                  pendingAmount,
                  status: approvedPayments.length > 0 && paidAmount >= parseMoney(apiBill.amount) ? 'lunas' : 'sebagian',
                } : prev
              )
            } catch {
              // Silent fail — don't disrupt the success flow
            }
          }
        } catch {
          // Silent fail — don't disrupt the success flow
        }
      }}
    />
  ) : null

  if (isModal && !open) {
    return null
  }

  // Modal mode or page route
  const modalOpen = isModal ? open : true

  if (loading) {
    return (
      <Modal open={modalOpen} onClose={handleClose} width={520}>
        <ModalHeader
          title="Memuat Form Pembayaran"
          subtitle=""
          onClose={handleClose}
        />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
            Memuat data tagihan...
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Batal
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  if (notFound) {
    return (
      <Modal open={modalOpen} onClose={handleClose} width={520}>
        <ModalHeader
          title="Tagihan Tidak Ditemukan"
          subtitle="Data iuran tidak ditemukan"
          onClose={handleClose}
        />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
            Tagihan iuran yang Anda buka tidak ditemukan.
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Tutup
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  if (error && !bill) {
    return (
      <Modal open={modalOpen} onClose={handleClose} width={520}>
        <ModalHeader
          title="Gagal Memuat Tagihan"
          subtitle=""
          onClose={handleClose}
        />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-danger)', fontSize: 13 }}>
            {error}
          </div>
        </ModalBody>
        <ModalFooter>
          <div style={{ display: 'flex', justifyContent: 'space-between', width: '100%' }}>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Tutup
            </button>
            <button type="button" className="sf-btn-primary" onClick={handleRetry}>
              Coba Lagi
            </button>
          </div>
        </ModalFooter>
      </Modal>
    )
  }

  if (!bill) {
    return (
      <Modal open={modalOpen} onClose={handleClose} width={520}>
        <ModalHeader
          title="Tagihan Tidak Ditemukan"
          subtitle="Data iuran tidak ditemukan"
          onClose={handleClose}
        />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
            Tagihan iuran yang Anda buka tidak ditemukan.
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Tutup
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  return paymentContent
}
