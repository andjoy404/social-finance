import { useState, useMemo } from 'react'
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
  BILL_DATA,
  formatRupiah,
  getStatusVariant,
  getStatusLabel,
  type IuranBill,
} from './iuranTypes'

function findBill(id: string): IuranBill | undefined {
  return BILL_DATA.find((b) => b.id === id)
}

export function IuranPayment() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const bill = useMemo(() => (id ? findBill(id) : null), [id])

  const handleClose = () => navigate('/iuran')

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [nominal, setNominal] = useState('')
  const [payDate, setPayDate] = useState('')
  const [catatan, setCatatan] = useState('')
  const [validationErrors, setValidationErrors] = useState<string[]>([])

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

  const sisa = bill.nominal - bill.paidAmount
  const nominalNum = nominal.trim() ? parseInt(nominal, 10) : 0

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationErrors([])
    setError(null)

    const errs: string[] = []
    if (!nominal.trim()) {
      errs.push('Nominal pembayaran harus diisi.')
    } else if (isNaN(nominalNum) || nominalNum <= 0) {
      errs.push('Nominal pembayaran harus lebih dari 0.')
    } else if (nominalNum > sisa) {
      errs.push(`Nominal pembayaran melebihi sisa tagihan (${formatRupiah(sisa)}).`)
    }
    if (!payDate) {
      errs.push('Tanggal pembayaran harus diisi.')
    }
    if (errs.length > 0) {
      setValidationErrors(errs)
      return
    }

    setLoading(true)
    setTimeout(() => {
      console.log('Pembayaran:', {
        billId: bill.id,
        nominal: nominalNum,
        paidDate: payDate,
        catatan,
      })
      setLoading(false)
      navigate('/iuran')
    }, 500)
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
              value={`${bill.periode}`}
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
              value={formatRupiah(sisa)}
              disabled
            />
          </div>

          <hr style={{ border: 'none', borderTop: '1px solid var(--border-color)', margin: 'var(--sp-md) 0' }} />

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
              placeholder={`Maksimal ${formatRupiah(sisa)}`}
              max={sisa}
              min={1}
              autoFocus
            />
            {sisa <= 0 && (
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
              disabled={loading}
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
            disabled={loading || sisa <= 0}
          >
            {loading ? 'Menyimpan...' : 'Simpan Pembayaran'}
          </button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
