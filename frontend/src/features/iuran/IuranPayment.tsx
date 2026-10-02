import { useState, useEffect, useMemo, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  CheckCircleOutlined,
  DollarOutlined,
  CalendarOutlined,
  FileTextOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'
import {
  formatRupiah,
  getStatusVariant,
  getStatusLabel,
  parseMoney,
  type IuranBill,
} from './iuranTypes'
import { apiGetBill, apiCreatePayment, type ApiPayment, type ApiError, getSessionPair } from '@/app/api'

// Payment method options
const PAYMENT_METHODS = [
  { value: 'CASH', label: 'Cash' },
  { value: 'TRANSFER', label: 'Transfer' },
] as const

function IuranPaymentContent({
  bill,
  remaining,
  handleClose,
  onPaymentSuccess,
}: {
  bill: IuranBill
  remaining: number
  handleClose: () => void
  onPaymentSuccess: (payment: ApiPayment) => void
}) {
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [nominal, setNominal] = useState('')
  const [payDate, setPayDate] = useState('')
  const [catatan, setCatatan] = useState('')
  const [method, setMethod] = useState<'CASH' | 'TRANSFER' | ''>('')
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [successPayment, setSuccessPayment] = useState<ApiPayment | null>(null)

  const nominalNum = nominal.trim() ? parseMoney(nominal) : 0

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])
    setError(null)

    const errs: string[] = []
    if (!nominal.trim()) {
      errs.push('Nominal pembayaran harus diisi.')
    } else if (isNaN(nominalNum) || nominalNum <= 0) {
      errs.push('Nominal pembayaran harus lebih dari 0.')
    } else if (nominalNum > remaining) {
      errs.push(`Nominal pembayaran melebihi sisa tagihan (${formatRupiah(remaining)}).`)
    }
    if (!payDate) {
      errs.push('Tanggal pembayaran harus diisi.')
    }
    if (!method) {
      errs.push('Metode pembayaran harus dipilih.')
    }
    if (errs.length > 0) {
      setValidationErrors(errs)
      return
    }

    setSubmitting(true)

    try {
      const token = localStorage.getItem('social-finance-session')
      const session = token ? JSON.parse(token) : null
      const accessToken = session?.accessToken

      if (!accessToken) {
        setError('Sesi Anda telah berakhir. Silakan login ulang.')
        setSubmitting(false)
        return
      }

      const payment = await apiCreatePayment(accessToken, {
        bill_id: bill.id,
        amount: String(nominalNum),
        method: method as 'CASH' | 'TRANSFER',
        notes: catatan || undefined,
      })

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
      <form onSubmit={handleSubmit}>
        <ModalBody>
          {validationErrors.length > 0 && (
            <div className="sf-form-error-list">
              <ul>{validationErrors.map((e, i) => <li key={i}>{e}</li>)}</ul>
            </div>
          )}
          {error && <div className="sf-form-error-list">{error}</div>}

          {/* Informasi warga/KK */}
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="ip-household">
              Informasi Warga/KK
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
              value={bill.periode}
              disabled
            />
          </div>

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
                value={formatRupiah(bill.nominal)}
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
              <WarningOutlined style={{ marginRight: 4, color: 'var(--sf-warning)' }} />
              Sisa Tagihan
            </label>
            <input
              id="ip-sisa"
              className="sf-form-input"
              style={{ fontWeight: 700, color: 'var(--sf-warning)' }}
              value={formatRupiah(remaining)}
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
              {PAYMENT_METHODS.map((m) => (
                <button
                  key={m.value}
                  type="button"
                  className={`sf-form-input ${method === m.value ? 'sf-form-input--active' : ''}`}
                  style={{
                    flex: 1,
                    textAlign: 'center',
                    cursor: 'pointer',
                    border: method === m.value ? '2px solid var(--sf-accent)' : '1px solid var(--border-color)',
                    background: method === m.value ? 'var(--sf-accent-subtle)' : 'transparent',
                    fontWeight: method === m.value ? 600 : 400,
                  }}
                  onClick={() => {
                    setMethod(m.value as 'CASH' | 'TRANSFER')
                    setValidationErrors([])
                  }}
                >
                  {m.label}
                </button>
              ))}
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
              type="number"
              value={nominal}
              onChange={(e) => { setNominal(e.target.value); setValidationErrors([]) }}
              placeholder={`Maksimal ${formatRupiah(remaining)}`}
              max={remaining}
              min={1}
              autoFocus
            />
            {remaining <= 0 && (
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
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose} disabled={submitting}>
            Batal
          </button>
          <button
            type="submit"
            className="sf-btn-primary"
            disabled={submitting || remaining <= 0 || !method}
          >
            {submitting ? 'Menyimpan...' : 'Simpan Pembayaran'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}

export function IuranPayment() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const handleClose = useCallback(() => navigate('/iuran'), [navigate])

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [bill, setBill] = useState<IuranBill | null>(null)

  const remaining = useMemo(() => {
    if (!bill) return 0
    return bill.nominal - bill.paidAmount
  }, [bill])

  // Load bill data from API
  useEffect(() => {
    let cancelled = false
    async function loadBill() {
      setLoading(true)
      setError(null)

      if (!id) {
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

        const apiBill = await apiGetBill(token, id)

        if (cancelled) return

        // Map backend bill to frontend format
        setBill({
          id: apiBill.id,
          householdId: apiBill.household_occupancy_id,
          householdName: apiBill.head_name ?? apiBill.house_number ?? 'Unknown',
          rt: apiBill.rt_id ?? 'RT',
          iuranType: apiBill.due_name ?? 'Iuran',
          periode: apiBill.period,
          nominal: parseMoney(apiBill.amount),
          paidAmount: 0, // will be loaded with payments
          status: 'belum_bayar', // default, will be mapped
          payments: [],
        })
      } catch (err) {
        if (cancelled) return
        const apiErr = err as ApiError
        if (apiErr.code === 'not_found') {
          setError('Tagihan tidak ditemukan')
        } else {
          setError(err instanceof Error ? err.message : 'Gagal memuat data tagihan')
        }
        setLoading(false)
      }
    }

    loadBill()
    return () => {
      cancelled = true
    }
  }, [id])

  // Handle payment success - refresh bill data
  const handlePaymentSuccess = useCallback(async (_payment: ApiPayment) => {
    // Refresh bill data after successful payment to update paidAmount
    try {
      const token = getSessionPair()?.accessToken
      if (token && id) {
        await apiGetBill(token, id)
        // We don't update bill state here to avoid UI disruption
        // The user will see the success modal and close it
      }
    } catch {
      // Silent fail - don't disrupt the success flow
    }
  }, [id])

  if (loading) {
    return (
      <Modal open onClose={handleClose} width={520}>
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
      </Modal>
    )
  }

  if (error && !bill) {
    return (
      <Modal open onClose={handleClose} width={520}>
        <ModalHeader
          title="Error"
          subtitle=""
          onClose={handleClose}
        />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
            {error}
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

  if (!bill) {
    return (
      <Modal open onClose={handleClose} width={520}>
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
            Kembali
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  return (
    <IuranPaymentContent
      bill={bill}
      remaining={remaining}
      handleClose={handleClose}
      onPaymentSuccess={handlePaymentSuccess}
    />
  )
}
