import { useState, useEffect, useMemo } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  ArrowLeftOutlined,
  DollarOutlined,
  FileTextOutlined,
  UserOutlined,
  CalendarOutlined,
} from '@ant-design/icons'
import { AppCard } from '@/components/AppCard'
import { Badge } from '@/components/Badge'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { SummaryCard } from '@/components/SummaryCard'
import type { IuranStatus } from './iuranTypes'
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
  type IuranPaymentRecord,
  type PaymentStatusDisplay,
} from './iuranTypes'
import { apiGetBill, apiListPayments, getSessionPair } from '@/app/api'

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

/* ─────────────────────────────────────────────
   Status badge (modal header)
   ───────────────────────────────────────────── */

function StatusBadge({ status }: { status: string }) {
  const typed = status as unknown as IuranStatus
  const variant = getStatusVariant(typed)
  return (
    <Badge variant={variant} style={{ fontSize: '12px', padding: '0 12px', height: '26px', fontWeight: 600 }}>
      {getStatusLabel(typed)}
    </Badge>
  )
}

/* ─────────────────────────────────────────────
   Progress bar
   ───────────────────────────────────────────── */

function PaymentProgress({ paid, nominal }: { paid: number; nominal: number }) {
  const pct = nominal > 0 ? Math.min((paid / nominal) * 100, 100) : 0
  const isPaid = paid >= nominal
  const barColor = isPaid
    ? 'var(--sf-success)'
    : 'var(--sf-accent)'

  return (
    <div>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        fontSize: '12px',
        color: 'var(--sf-text-muted)',
        marginBottom: '6px',
      }}>
        <span>{formatRupiah(paid)} dari {formatRupiah(nominal)}</span>
        <span style={{ fontWeight: 600, color: isPaid ? 'var(--sf-success)' : 'var(--sf-accent)' }}>
          {pct.toFixed(0)}%
        </span>
      </div>
      <div style={{
        height: '6px',
        background: 'var(--sf-surface-subtle)',
        borderRadius: '3px',
        overflow: 'hidden',
      }}>
        <div style={{
          height: '100%',
          width: `${pct}%`,
          background: barColor,
          borderRadius: '3px',
          transition: 'width 400ms ease',
        }} />
      </div>
    </div>
  )
}

/* ─────────────────────────────────────────────
   Info row helper
   ───────────────────────────────────────────── */

function InfoRow({ icon, label, value }: { icon?: React.ReactNode; label: string; value: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: '10px' }}>
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: '28px',
        height: '28px',
        borderRadius: '6px',
        background: 'var(--sf-accent-soft)',
        color: 'var(--sf-accent)',
        fontSize: '13px',
        flexShrink: 0,
        marginTop: '1px',
      }}>
        {icon || <FileTextOutlined />}
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{
          fontSize: '11px',
          color: 'var(--sf-text-muted)',
          fontWeight: 500,
          marginBottom: '2px',
          textTransform: 'uppercase',
          letterSpacing: '0.03em',
        }}>
          {label}
        </div>
        <div style={{
          fontSize: '13px',
          color: 'var(--sf-text)',
          fontWeight: 500,
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }}>
          {value}
        </div>
      </div>
    </div>
  )
}

/* ─────────────────────────────────────────────
   Payment history item
   ───────────────────────────────────────────── */

function PaymentHistoryItem({ payment }: { payment: IuranPaymentRecord }) {
  const isApproved = payment.status === 'APPROVED'
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '10px 12px',
      borderRadius: '8px',
      border: `1px solid ${isApproved ? 'var(--sf-success)' : 'var(--sf-border)'}`,
      background: isApproved ? 'var(--sf-success-soft)' : 'transparent',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
        <div style={{
          width: '36px',
          height: '36px',
          borderRadius: '50%',
          background: 'var(--sf-surface)',
          border: '1px solid var(--sf-border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'var(--sf-text-muted)',
          fontSize: '14px',
          flexShrink: 0,
        }}>
          <DollarOutlined />
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{
            fontSize: '13px',
            fontWeight: 600,
            fontFamily: 'var(--sf-font-mono)',
            color: 'var(--sf-text)',
          }}>
            {formatRupiah(payment.nominal)}
          </div>
          <div style={{
            fontSize: '11px',
            color: 'var(--sf-text-muted)',
            marginTop: '1px',
          }}>
            {formatDate(payment.paidDate)}
            {payment.method ? ' · ' + (payment.method === 'TRANSFER' ? 'Transfer' : 'Cash') : ''}
          </div>
        </div>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
        {payment.catatan && (
          <span style={{
            fontSize: '11px',
            color: 'var(--sf-text-muted)',
            fontStyle: 'italic',
            maxWidth: '120px',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {payment.catatan}
          </span>
        )}
        <Badge variant={getPaymentStatusVariant(payment.status!)}>
          {getPaymentStatusLabel(payment.status!)}
        </Badge>
      </div>
    </div>
  )
}

/* ─────────────────────────────────────────────
   Shared detail content (used by both modes)
   ───────────────────────────────────────────── */

function DetailContent({ bill, remaining }: { bill: IuranBill; remaining: number }) {
  const payments = bill.payments ?? []
  const hasHistory = payments.length > 0

  return (
    <>
      {/* ── Payment summary ── */}
      <AppCard style={{ padding: '20px 24px' }}>
        {/* Top row: big amount */}
        <div style={{ textAlign: 'center', marginBottom: '16px' }}>
          <div style={{
            fontSize: '12px',
            color: 'var(--sf-text-muted)',
            fontWeight: 500,
            textTransform: 'uppercase',
            letterSpacing: '0.04em',
            marginBottom: '4px',
          }}>
            Total Tagihan
          </div>
          <div style={{
            fontSize: '32px',
            fontWeight: 700,
            color: 'var(--sf-text)',
            lineHeight: 1.1,
            fontVariantNumeric: 'tabular-nums',
            fontFamily: 'var(--sf-font-mono)',
          }}>
            {formatRupiah(bill.nominal)}
          </div>
        </div>

        {/* Progress bar */}
        {bill.nominal > 0 && (
          <PaymentProgress paid={bill.paidAmount} nominal={bill.nominal} />
        )}

        {/* Paid / Remaining row */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: '12px',
          marginTop: '16px',
        }}>
          <SummaryCard
            label="Sudah Dibayar"
            value={bill.paidAmount > 0 ? formatRupiah(bill.paidAmount) : 'Rp0'}
            accent="income"
          />
          <SummaryCard
            label={remaining <= 0 ? 'Sisa Tagihan' : 'Sisa Tagihan'}
            value={formatRupiah(Math.max(remaining, 0))}
            accent={remaining <= 0 ? 'income' : 'expense'}
          />
        </div>
      </AppCard>

      {/* ── Info grid: 2-column ── */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: '1fr 1fr',
        gap: '12px',
        marginTop: '12px',
      }}>
        {/* Household info */}
        <AppCard style={{ padding: '16px' }}>
          <div style={{
            fontSize: '12px',
            fontWeight: 600,
            color: 'var(--sf-text)',
            marginBottom: '12px',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}>
            <UserOutlined style={{ color: 'var(--sf-accent)' }} />
            Data Warga
          </div>
          <InfoRow
            icon={<UserOutlined />}
            label="Kepala Keluarga"
            value={bill.householdName}
          />
        </AppCard>

        {/* Bill info */}
        <AppCard style={{ padding: '16px' }}>
          <div style={{
            fontSize: '12px',
            fontWeight: 600,
            color: 'var(--sf-text)',
            marginBottom: '12px',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}>
            <FileTextOutlined style={{ color: 'var(--sf-accent)' }} />
            Detail Tagihan
          </div>
          <InfoRow
            icon={<FileTextOutlined />}
            label="Jenis Iuran"
            value={bill.iuranType}
          />
          <div style={{ marginTop: '10px' }}>
            <InfoRow
              icon={<CalendarOutlined />}
              label="Periode"
              value={formatFullPeriodeLabel(bill.periode)}
            />
          </div>
        </AppCard>
      </div>

      {/* ── Payment history ── */}
      {hasHistory && (
        <AppCard style={{ marginTop: '12px', padding: '16px' }}>
          <div style={{
            fontSize: '13px',
            fontWeight: 600,
            color: 'var(--sf-text)',
            marginBottom: '12px',
          }}>
            Riwayat Pembayaran
            <span style={{
              fontSize: '12px',
              fontWeight: 400,
              color: 'var(--sf-text-muted)',
              marginLeft: '8px',
            }}>
              ({payments.length} tercatat)
            </span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {payments.map((payment) => (
              <PaymentHistoryItem key={payment.id} payment={payment} />
            ))}
          </div>
        </AppCard>
      )}
    </>
  )
}

/* ─────────────────────────────────────────────
   IuranDetail — entry component
   ───────────────────────────────────────────── */

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

  /* ── Modal mode ───────────────────────────── */

  if (isModal) {
    return (
      <Modal
        open={open}
        onClose={handleClose}
        width={720}
        overlayClassName="sf-modal-overlay-detail"
      >
        <ModalHeader
          title="Detail Tagihan"
          subtitle={
            bill
              ? `${bill.iuranType} · ${formatFullPeriodeLabel(bill.periode)}`
              : `ID: ${effectiveId}`
          }
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
              background: 'var(--sf-danger-soft)',
              color: 'var(--sf-danger)',
              border: '1px solid rgba(242,73,92,0.3)',
              borderRadius: 'var(--sf-radius-sm)',
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
          {!loading && !error && !notFound && bill && (
            <>
              {/* Header row with status badge */}
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginBottom: '16px',
              }}>
                <div>
                  <div style={{
                    fontSize: '15px',
                    fontWeight: 700,
                    color: 'var(--sf-text)',
                  }}>
                    {bill.householdName}
                  </div>
                  <div style={{
                    fontSize: '12px',
                    color: 'var(--sf-text-muted)',
                    marginTop: '2px',
                  }}>
                    {bill.iuranType} · {formatFullPeriodeLabel(bill.periode)}
                  </div>
                </div>
                <StatusBadge status={bill.status} />
              </div>

              <DetailContent bill={bill} remaining={remaining} />
            </>
          )}
        </ModalBody>

        {/* Footer actions */}
        {!loading && !error && !notFound && bill && (
          <ModalFooter>
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              width: '100%',
            }}>
              <button
                type="button"
                onClick={handleClose}
                style={{
                  padding: '8px 20px',
                  fontSize: '13px',
                  fontWeight: 500,
                  color: 'var(--sf-text)',
                  background: 'var(--sf-surface)',
                  border: '1px solid var(--sf-border)',
                  borderRadius: 'var(--sf-radius-sm)',
                  cursor: 'pointer',
                }}
              >
                Tutup
              </button>

              {remaining > 0 && (
                <button
                  type="button"
                  onClick={() => onPayment(bill.id)}
                  style={{
                    padding: '8px 20px',
                    fontSize: '13px',
                    fontWeight: 600,
                    color: '#fff',
                    background: 'var(--sf-accent)',
                    border: 'none',
                    borderRadius: 'var(--sf-radius-sm)',
                    cursor: 'pointer',
                  }}
                >
                  Bayar Sekarang
                </button>
              )}
            </div>
          </ModalFooter>
        )}
      </Modal>
    )
  }

  /* ── Page mode (route: /iuran/:id) ────────── */

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
            background: 'var(--sf-danger-soft)',
            color: 'var(--sf-danger)',
            border: '1px solid rgba(242,73,92,0.3)',
            borderRadius: 'var(--sf-radius-sm)',
            padding: '10px 14px',
            fontSize: '13px',
          }}>
            {error}
          </div>
        </AppCard>
      )}

      {/* Detail */}
      {!loading && !error && !notFound && bill && (
        <>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            marginBottom: '16px',
          }}>
            <div>
              <div style={{
                fontSize: '16px',
                fontWeight: 700,
                color: 'var(--sf-text)',
              }}>
                {bill.householdName}
              </div>
              <div style={{
                fontSize: '12px',
                color: 'var(--sf-text-muted)',
                marginTop: '2px',
              }}>
                {bill.iuranType} · {formatFullPeriodeLabel(bill.periode)}
              </div>
            </div>
            <StatusBadge status={bill.status} />
          </div>

          <DetailContent bill={bill} remaining={remaining} />
        </>
      )}
    </div>
  )
}
