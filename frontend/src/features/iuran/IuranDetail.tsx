import { useState, useEffect, useMemo } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  ArrowLeftOutlined,
  CheckCircleOutlined,
  DollarOutlined,
  CalendarOutlined,
  FileTextOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { AppCard } from '@/components/AppCard'
import { Badge } from '@/components/Badge'
import { Modal, ModalHeader, ModalBody } from '@/components/Modal'
import {
  BACKEND_STATUS_MAP,
  formatRupiah,
  formatFullPeriodeLabel,
  getStatusVariant,
  getStatusLabel,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  parseMoney,
  type IuranBill,
  type PaymentStatusDisplay,
} from './iuranTypes'
import { apiGetBill, apiListPayments, getSessionPair } from '@/app/api'

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
}

export function IuranDetail({
  billId,
  open: openProp,
  onClose: onCloseProp,
  onPayment: onPaymentProp,
}: {
  billId?: string
  open?: boolean
  onClose?: () => void
  onPayment?: (billId: string) => void
} = {}) {
  const routeId = useParams<{ id: string }>()?.id
  const effectiveId = billId ?? routeId

  const isModal = billId !== undefined
  const open = openProp ?? true
  const handleClose = onCloseProp ?? (() => window.history.back())
  const onPayment = onPaymentProp ?? (() => { window.history.back(); window.location.href = `/iuran/${effectiveId}/pembayaran` })

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [bill, setBill] = useState<IuranBill | null>(null)
  const [notFound, setNotFound] = useState(false)

  // Load bill data
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

        // Map backend bill to frontend format
        const mappedBill: IuranBill = {
          id: apiBill.id,
          householdId: apiBill.household_occupancy_id,
          householdName: apiBill.head_name ?? apiBill.house_number ?? 'Unknown',
          rt: apiBill.rt_id ?? 'RT',
          iuranType: apiBill.due_name ?? 'Iuran',
          periode: apiBill.period,
          nominal: parseMoney(apiBill.amount),
          paidAmount: 0, // will be calculated from payments
          status: BACKEND_STATUS_MAP[apiBill.status] ?? 'belum_bayar',
          payments: [], // will be loaded separately
        }

        // Load payments for this bill
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

        // Map payments to frontend format
        const paymentRecords = paymentsResponse.data.map((p) => ({
          id: p.id,
          nominal: parseMoney(p.amount),
          paidDate: p.paid_at,
          catatan: p.notes ?? undefined,
          status: p.status as PaymentStatusDisplay,
          method: p.method,
        }))

        setBill({
          ...mappedBill,
          paidAmount,
          payments: paymentRecords,
        })
      } catch (err) {
        if (cancelled) return
        const apiErr = err as { code?: string }
        if (apiErr.code === 'not_found') {
          setNotFound(true)
        } else {
          setError(err instanceof Error ? err.message : 'Gagal memuat data')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadBill()
    return () => {
      cancelled = true
    }
  }, [effectiveId])

  const remaining = useMemo(() => {
    if (!bill) return 0
    return bill.nominal - bill.paidAmount
  }, [bill])

  /* ── Canonical detail content (reused by both modes) ─────────────────── */

  const detailContent = !loading && !notFound && !error && bill ? (
    <>
      <AppCard style={{ marginBottom: 'var(--sp-md)' }}>
        {/* Header */}
        <div style={{
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          marginBottom: 'var(--sp-lg)',
          gap: 'var(--sp-md)',
          flexWrap: 'wrap',
        }}>
          <div>
            <h2 style={{
              fontSize: '16px',
              fontWeight: 700,
              color: 'var(--sf-text)',
              margin: 0,
              lineHeight: 1.3,
            }}>
              {bill.householdName}
            </h2>
            <p style={{
              fontSize: '12px',
              color: 'var(--sf-text-muted)',
              margin: '2px 0 0',
            }}>
              Detail tagihan iuran
            </p>
          </div>
          <Badge variant={getStatusVariant(bill.status)}>
            {getStatusLabel(bill.status)}
          </Badge>
        </div>

        {/* Info grid */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
          gap: 'var(--sp-md)',
        }}>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              marginBottom: '2px',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
            }}>
              RT
            </div>
            <div style={{
              fontSize: '13px',
              color: 'var(--sf-text)',
              fontWeight: 500,
            }}>
              {bill.rt}
            </div>
          </div>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              marginBottom: '2px',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
            }}>
              <FileTextOutlined style={{ marginRight: 4 }} />
              Jenis Iuran
            </div>
            <div style={{
              fontSize: '13px',
              color: 'var(--sf-text)',
            }}>
              {bill.iuranType}
            </div>
          </div>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              marginBottom: '2px',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
            }}>
              <CalendarOutlined style={{ marginRight: 4 }} />
              Periode
            </div>
            <div style={{
              fontSize: '13px',
              color: 'var(--sf-text)',
            }}>
              {formatFullPeriodeLabel(bill.periode)}
            </div>
          </div>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              marginBottom: '2px',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
            }}>
              <DollarOutlined style={{ marginRight: 4 }} />
              Total Tagihan
            </div>
            <div style={{
              fontSize: '14px',
              color: 'var(--sf-text)',
              fontWeight: 600,
            }}>
              {formatRupiah(bill.nominal)}
            </div>
          </div>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              marginBottom: '2px',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
            }}>
              <CheckCircleOutlined style={{ marginRight: 4, color: 'var(--sf-success)' }} />
              Sudah Dibayar
            </div>
            <div style={{
              fontSize: '14px',
              color: 'var(--sf-success)',
              fontWeight: 600,
            }}>
              {formatRupiah(bill.paidAmount)}
            </div>
          </div>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              marginBottom: '2px',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
            }}>
              <WarningOutlined style={{ marginRight: 4, color: remaining <= 0 ? 'var(--sf-success)' : 'var(--sf-warning)' }} />
              Sisa Tagihan
            </div>
            <div style={{
              fontSize: '14px',
              color: remaining <= 0 ? 'var(--sf-success)' : 'var(--sf-warning)',
              fontWeight: 600,
            }}>
              {formatRupiah(remaining)}
            </div>
          </div>
        </div>

        {/* Actions */}
        {remaining > 0 && (
          <div style={{ marginTop: 'var(--sp-lg)', display: 'flex', gap: '8px' }}>
            {isModal ? (
              <button
                type="button"
                onClick={() => onPayment(bill.id)}
                style={{
                  padding: '8px 16px',
                  fontSize: '13px',
                  fontWeight: 500,
                  color: '#fff',
                  background: 'var(--sf-accent)',
                  border: 'none',
                  borderRadius: 'var(--radius-sm)',
                  cursor: 'pointer',
                }}
              >
                Bayar Sekarang
              </button>
            ) : (
              <Link
                to={`/iuran/${bill.id}/pembayaran`}
                style={{
                  padding: '8px 16px',
                  fontSize: '13px',
                  fontWeight: 500,
                  color: '#fff',
                  background: 'var(--sf-accent)',
                  border: 'none',
                  borderRadius: 'var(--radius-sm)',
                  textDecoration: 'none',
                  cursor: 'pointer',
                }}
              >
                Bayar Sekarang
              </Link>
            )}
          </div>
        )}
      </AppCard>

      {/* Payment history */}
      {bill.payments && bill.payments.length > 0 && (
        <AppCard>
          <div style={{ marginBottom: 'var(--sp-md)' }}>
            <h3 style={{
              fontSize: '14px',
              fontWeight: 600,
              color: 'var(--sf-text)',
              margin: 0,
            }}>
              Riwayat Pembayaran
            </h3>
            <p style={{
              fontSize: '12px',
              color: 'var(--sf-text-muted)',
              margin: '2px 0 0',
            }}>
              {bill.payments.length} pembayaran tercatat
            </p>
          </div>
          <table className="sf-table" style={{ width: '100%' }}>
            <thead>
              <tr>
                <th>Tanggal</th>
                <th>Nominal</th>
                <th>Status</th>
                <th>Metode</th>
                <th>Catatan</th>
              </tr>
            </thead>
            <tbody>
              {bill.payments.map((payment) => (
                <tr key={payment.id}>
                  <td>{formatDate(payment.paidDate)}</td>
                  <td style={{ fontFamily: 'monospace', textAlign: 'right' }}>
                    {formatRupiah(payment.nominal)}
                  </td>
                  <td>
                    <Badge variant={getPaymentStatusVariant(payment.status!)}>
                      {getPaymentStatusLabel(payment.status!)}
                    </Badge>
                  </td>
                  <td style={{ color: 'var(--sf-text-muted)' }}>
                    {payment.method === 'TRANSFER' ? 'Transfer' : 'Cash'}
                  </td>
                  <td style={{ color: 'var(--sf-text-muted)' }}>
                    {payment.catatan || '\u2014'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </AppCard>
      )}
    </>
  ) : null

  /* ── Loading / error / not-found (page mode only) ────────────────────── */

  if (isModal) {
    return (
      <Modal
        open={open}
        onClose={handleClose}
        width={800}
        overlayClassName="sf-modal-overlay-detail"
      >
        <ModalHeader
          title="Detail Iuran"
          subtitle={`ID: ${effectiveId}`}
          onClose={handleClose}
        />
        <ModalBody>
          {loading && (
            <div style={{
              display: 'flex',
              justifyContent: 'center',
              padding: 'var(--sp-xl)',
              color: 'var(--sf-text-muted)',
              fontSize: '13px',
            }}>
              Memuat detail tagihan...
            </div>
          )}
          {error && (
            <div style={{
              background: 'var(--color-expense-subtle)',
              color: 'var(--color-expense)',
              border: '1px solid rgba(255,59,48,0.3)',
              borderRadius: 'var(--radius-sm)',
              padding: '10px 14px',
              fontSize: '13px',
            }}>
              {error}
            </div>
          )}
          {notFound && (
            <div style={{
              textAlign: 'center',
              padding: 'var(--sp-xl) 0',
              color: 'var(--sf-text-muted)',
              fontSize: '13px',
            }}>
              Tagihan iuran tidak ditemukan.
            </div>
          )}
          {detailContent}
        </ModalBody>
      </Modal>
    )
  }

  /* ── Page mode (route: /iuran/:id) ──────────────────────────────────── */

  return (
    <div style={{ flex: 1, padding: 'var(--sp-md) var(--sp-xl)' }}>
      {/* Back button */}
      <div style={{ marginBottom: 'var(--sp-md)' }}>
        <Link
          to="/iuran"
          style={{
            fontSize: '13px',
            color: 'var(--sf-accent)',
            display: 'inline-flex',
            alignItems: 'center',
            gap: 'var(--sp-xs)',
          }}
          onClick={() => window.history.back()}
        >
          <ArrowLeftOutlined style={{ fontSize: 12 }} />
          Kembali ke Daftar Iuran
        </Link>
      </div>

      {/* Loading */}
      {loading && (
        <AppCard>
          <div style={{
            display: 'flex',
            justifyContent: 'center',
            padding: 'var(--sp-xl)',
            color: 'var(--sf-text-muted)',
            fontSize: '13px',
          }}>
            Memuat detail tagihan...
          </div>
        </AppCard>
      )}

      {/* Not found */}
      {!loading && notFound && (
        <AppCard>
          <div style={{
            textAlign: 'center',
            padding: 'var(--sp-xl) 0',
            color: 'var(--sf-text-muted)',
            fontSize: '13px',
          }}>
            Tagihan iuran tidak ditemukan.
          </div>
        </AppCard>
      )}

      {/* Error */}
      {!loading && !notFound && error && (
        <AppCard>
          <div style={{
            background: 'var(--color-expense-subtle)',
            color: 'var(--color-expense)',
            border: '1px solid rgba(255,59,48,0.3)',
            borderRadius: 'var(--radius-sm)',
            padding: '10px 14px',
            fontSize: '13px',
          }}>
            {error}
          </div>
        </AppCard>
      )}

      {/* Detail */}
      {detailContent}
    </div>
  )
}
