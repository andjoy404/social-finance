import { useState, useEffect, useMemo, useCallback } from 'react'
import { Link, useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeftOutlined,
  DollarOutlined,
  FileTextOutlined,
  UserOutlined,
  CalendarOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  FileImageOutlined,
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
import {
  apiGetBill,
  apiListPayments,
  apiGetPaymentProofBlob,
  apiVerifyPayment,
  apiReversePayment,
  apiListFundLocations,
  getSessionPair,
  type FundLocation,
} from '@/app/api'
import { useAuth, type AuthUser } from '@/app/AuthContext'

export function canApprovePayment(user: AuthUser | null): boolean {
  if (!user) return false
  if (user.systemRole === 'super_admin') return true
  if (user.effectivePermissions?.includes('finance.approve')) return true
  const role = user.role?.toLowerCase()
  if (role === 'super_admin') return true
  const jabatan = user.jabatan?.toLowerCase()
  if (jabatan === 'ketua' || jabatan === 'bendahara') return true
  if (role === 'bendahara') return true
  return false
}

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
   Proof image thumbnail component
   ───────────────────────────────────────────── */

function ProofImageThumbnail({
  paymentId,
  onClick,
}: {
  paymentId: string
  onClick?: () => void
}) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    let cancelled = false
    let currentUrl: string | null = null

    async function fetchProof() {
      try {
        const token = getSessionPair()?.accessToken
        if (!token) {
          setError(true)
          setLoading(false)
          return
        }

        const { blob } = await apiGetPaymentProofBlob(token, paymentId)
        if (cancelled) return

        currentUrl = URL.createObjectURL(blob)
        setBlobUrl(currentUrl)
        setLoading(false)
      } catch {
        if (!cancelled) {
          setError(true)
          setLoading(false)
        }
      }
    }

    fetchProof()

    return () => {
      cancelled = true
      if (currentUrl) {
        URL.revokeObjectURL(currentUrl)
      }
    }
  }, [paymentId])

  if (loading) {
    return (
      <div
        style={{
          width: '32px',
          height: '32px',
          borderRadius: '4px',
          background: 'var(--sf-surface)',
          border: '1px solid var(--sf-border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '11px',
          color: 'var(--sf-text-muted)',
        }}
        title="Memuat bukti..."
      >
        ⏳
      </div>
    )
  }

  if (error || !blobUrl) {
    return (
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation()
          onClick?.()
        }}
        style={{
          padding: '4px 8px',
          fontSize: '11px',
          borderRadius: '4px',
          border: '1px solid var(--sf-border)',
          background: 'var(--sf-surface)',
          color: 'var(--sf-text-muted)',
          cursor: 'pointer',
          display: 'inline-flex',
          alignItems: 'center',
          gap: '4px',
        }}
        title="Lihat Bukti"
      >
        <FileImageOutlined /> Bukti
      </button>
    )
  }

  return (
    <img
      src={blobUrl}
      alt="Bukti Transfer"
      onClick={(e) => {
        e.stopPropagation()
        onClick?.()
      }}
      style={{
        width: '32px',
        height: '32px',
        objectFit: 'cover',
        borderRadius: '4px',
        border: '1px solid var(--sf-border)',
        cursor: 'pointer',
      }}
      title="Klik untuk memperbesar bukti"
    />
  )
}

/* ─────────────────────────────────────────────
   Proof preview modal
   ───────────────────────────────────────────── */

function ProofPreviewModal({
  paymentId,
  onClose,
}: {
  paymentId: string
  onClose: () => void
}) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    let currentUrl: string | null = null

    async function fetchProof() {
      try {
        const token = getSessionPair()?.accessToken
        if (!token) {
          setError('Sesi berakhir. Silakan login ulang.')
          setLoading(false)
          return
        }

        const { blob } = await apiGetPaymentProofBlob(token, paymentId)
        if (cancelled) return

        currentUrl = URL.createObjectURL(blob)
        setBlobUrl(currentUrl)
        setLoading(false)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Gagal memuat bukti pembayaran')
          setLoading(false)
        }
      }
    }

    fetchProof()

    return () => {
      cancelled = true
      if (currentUrl) {
        URL.revokeObjectURL(currentUrl)
      }
    }
  }, [paymentId])

  return (
    <Modal open onClose={onClose} width={600} overlayClassName="sf-modal-overlay-preview">
      <ModalHeader
        title="Bukti Pembayaran"
        subtitle=""
        onClose={onClose}
      />
      <ModalBody>
        <div style={{ textAlign: 'center', padding: '12px 0' }}>
          {loading && (
            <div style={{ padding: '40px 0', color: 'var(--sf-text-muted)' }}>
              Memuat gambar bukti pembayaran...
            </div>
          )}
          {error && (
            <div style={{ padding: '20px 0', color: 'var(--sf-danger)' }}>
              {error}
            </div>
          )}
          {blobUrl && (
            <img
              src={blobUrl}
              alt="Bukti Pembayaran Penuh"
              style={{
                maxWidth: '100%',
                maxHeight: '65vh',
                borderRadius: '6px',
                objectFit: 'contain',
                boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
              }}
            />
          )}
        </div>
      </ModalBody>
      <ModalFooter>
        <button type="button" className="sf-btn-primary" onClick={onClose}>
          Tutup
        </button>
      </ModalFooter>
    </Modal>
  )
}

/* ─────────────────────────────────────────────
   Payment history item
   ───────────────────────────────────────────── */

function PaymentHistoryItem({
  payment,
  canApprove = true,
  onVerify,
  onViewProof,
  onTap,
}: {
  payment: IuranPaymentRecord
  canApprove?: boolean
  onVerify?: () => void
  onViewProof?: (paymentId: string) => void
  onTap?: () => void
}) {
  const isApproved = payment.status === 'APPROVED'
  const isPending = payment.status === 'PENDING'
  const allowVerify = isPending && canApprove
  return (
    <div
      role={onTap || allowVerify ? 'button' : undefined}
      tabIndex={onTap || allowVerify ? 0 : undefined}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          if (allowVerify) onVerify?.()
          else onTap?.()
        }
      }}
      onClick={() => {
        if (allowVerify) onVerify?.()
        else onTap?.()
      }}
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: isPending ? '10px 12px' : '10px 12px',
        borderRadius: '8px',
        border: isApproved
          ? '1px solid var(--sf-success)'
          : isPending
            ? '1px solid var(--sf-warning)'
            : '1px solid var(--sf-border)',
        background: isApproved
          ? 'var(--sf-success-soft)'
          : isPending
            ? 'var(--sf-warning-soft)'
            : 'transparent',
        cursor: allowVerify || onTap ? 'pointer' : 'default',
        outline: 'none',
        transition: 'background 150ms ease',
      }}
      onMouseEnter={(e) => {
        if (onTap || allowVerify) {
          (e.currentTarget as HTMLDivElement).style.background = isApproved
            ? 'var(--sf-success-soft)'
            : isPending
              ? 'var(--sf-warning-soft)'
              : 'var(--sf-surface-subtle)'
        }
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLDivElement).style.background = isApproved
          ? 'var(--sf-success-soft)'
          : isPending
            ? 'var(--sf-warning-soft)'
            : 'transparent'
      }}
    >
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
            {payment.paymentNo && (
              <span style={{ fontFamily: 'var(--sf-font-mono)', marginLeft: '4px' }}>
                {payment.paymentNo}
              </span>
            )}
          </div>
        </div>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
        {payment.proofPath && (
          <ProofImageThumbnail
            paymentId={payment.id}
            onClick={() => onViewProof?.(payment.id)}
          />
        )}
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
        {allowVerify && (
          <span style={{
            fontSize: '10px',
            color: 'var(--sf-warning)',
            fontWeight: 500,
          }}>
            Klik untuk verifikasi
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

function DetailContent({
  bill,
  remaining,
  canApprove = true,
  onPendingVerify,
  onViewProof,
  onPaymentDetail,
}: {
  bill: IuranBill
  remaining: number
  canApprove?: boolean
  onPendingVerify?: (payment: IuranPaymentRecord) => void
  onViewProof?: (paymentId: string) => void
  onPaymentDetail?: (payment: IuranPaymentRecord) => void
}) {
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
            label="Sisa Tagihan"
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
          <div style={{ marginTop: '10px' }}>
            <InfoRow
              icon={<UserOutlined />}
              label="No. Rumah"
              value={bill.houseNumber ? `No. ${bill.houseNumber}` : '—'}
            />
          </div>
          <div style={{ marginTop: '10px' }}>
            <InfoRow
              icon={<UserOutlined />}
              label="Wilayah"
              value={bill.rt || '—'}
            />
          </div>
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
          <div style={{ marginTop: '10px' }}>
            <InfoRow
              icon={<DollarOutlined />}
              label="Sifat Iuran"
              value={bill.isVoluntary ? 'Sukarela' : 'Wajib'}
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
              <PaymentHistoryItem
                key={payment.id}
                payment={payment}
                canApprove={canApprove}
                onVerify={() => onPendingVerify?.(payment)}
                onViewProof={onViewProof}
                onTap={() => onPaymentDetail?.(payment)}
              />
            ))}
          </div>
        </AppCard>
      )}
    </>
  )
}

/* ─────────────────────────────────────────────
   Payment detail modal — shows full payment info
   ───────────────────────────────────────────── */

function PaymentDetailModal({
  payment,
  onClose,
  onViewProof,
  canApprove,
  onReverse,
}: {
  payment: IuranPaymentRecord
  onClose: () => void
  onViewProof?: (paymentId: string) => void
  canApprove?: boolean
  onReverse?: () => void
}) {
  const isApproved = payment.status === 'APPROVED'
  const isRejected = payment.status === 'REJECTED'
  const statusColor = isApproved
    ? 'var(--sf-success)'
    : isRejected
      ? 'var(--sf-danger)'
      : 'var(--sf-warning)'
  const statusIcon = isApproved
    ? '✓'
    : isRejected
      ? '✕'
      : '⏳'

  return (
    <Modal open onClose={onClose} width={560}>
      <ModalHeader
        title="Detail Pembayaran"
        subtitle=""
        onClose={onClose}
      />
      <ModalBody>
        {/* Amount + status header */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
          marginBottom: '20px',
        }}>
          <div style={{
            width: '40px',
            height: '40px',
            borderRadius: '50%',
            background: `${statusColor}18`,
            border: `1px solid ${statusColor}40`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '18px',
            fontWeight: 700,
            color: statusColor,
            flexShrink: 0,
          }}>
            {statusIcon}
          </div>
          <div>
            <div style={{
              fontSize: '18px',
              fontWeight: 700,
              fontFamily: 'var(--sf-font-mono)',
              color: 'var(--sf-text)',
            }}>
              {formatRupiah(payment.nominal)}
            </div>
            <div style={{
              fontSize: '12px',
              color: 'var(--sf-text-muted)',
              marginTop: '2px',
            }}>
              {formatDate(payment.paidDate)}
              {payment.method ? ` · ${payment.method === 'TRANSFER' ? 'Transfer' : 'Cash'}` : ''}
            </div>
          </div>
        </div>

        {/* Details grid */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr 1fr',
          gap: '16px',
          marginBottom: '20px',
        }}>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
              marginBottom: '4px',
            }}>
              Status
            </div>
            <div style={{
              fontSize: '13px',
              fontWeight: 600,
              color: statusColor,
            }}>
              {getPaymentStatusLabel(payment.status!)}
            </div>
          </div>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
              marginBottom: '4px',
            }}>
              Metode
            </div>
            <div style={{
              fontSize: '13px',
              fontWeight: 600,
              color: 'var(--sf-text)',
            }}>
              {payment.method === 'TRANSFER' ? 'Transfer' : payment.method === 'CASH' ? 'Cash' : '—'}
            </div>
          </div>
          <div>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
              marginBottom: '4px',
            }}>
              No. Pembayaran
            </div>
            <div style={{
              fontSize: '13px',
              fontWeight: 600,
              color: 'var(--sf-text)',
              fontFamily: 'var(--sf-font-mono)',
            }}>
              {payment.paymentNo || '—'}
            </div>
          </div>
        </div>

        {/* Catatan Warga */}
        {payment.catatan && (
          <div style={{ marginBottom: '16px' }}>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
              marginBottom: '6px',
            }}>
              Catatan Warga
            </div>
            <div style={{
              fontSize: '13px',
              color: 'var(--sf-text)',
              padding: '10px 12px',
              background: 'var(--sf-surface)',
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid var(--sf-border)',
            }}>
              {payment.catatan}
            </div>
          </div>
        )}

        {/* Catatan Pengurus (approval/rejection notes) */}
        {payment.officerNotes && (
          <div style={{ marginBottom: '16px' }}>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
              marginBottom: '6px',
            }}>
              Catatan Pengurus
            </div>
            <div style={{
              fontSize: '13px',
              color: 'var(--sf-text)',
              padding: '10px 12px',
              background: 'var(--sf-surface)',
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid var(--sf-border)',
            }}>
              {payment.officerNotes}
            </div>
          </div>
        )}

        {/* Catatan Penolakan (legacy) */}
        {payment.rejectionReason && !payment.officerNotes && (
          <div style={{ marginBottom: '16px' }}>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
              marginBottom: '6px',
            }}>
              Alasan Penolakan
            </div>
            <div style={{
              fontSize: '13px',
              color: 'var(--sf-danger)',
              padding: '10px 12px',
              background: 'var(--sf-danger-soft)',
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid rgba(242,73,92,0.3)',
            }}>
              {payment.rejectionReason}
            </div>
          </div>
        )}

        {/* Diverifikasi Oleh */}
        {payment.verifiedBy && (
          <div style={{ marginBottom: '16px' }}>
            <div style={{
              fontSize: '11px',
              color: 'var(--sf-text-muted)',
              fontWeight: 500,
              textTransform: 'uppercase',
              letterSpacing: '0.03em',
              marginBottom: '6px',
            }}>
              Diverifikasi Oleh
            </div>
            <div style={{
              fontSize: '13px',
              color: 'var(--sf-success)',
              fontWeight: 600,
            }}>
              {payment.verifiedByName ?? payment.verifiedBy}
            </div>
          </div>
        )}

        {/* Bukti Transfer */}
        <div>
          <div style={{
            fontSize: '11px',
            color: 'var(--sf-text-muted)',
            fontWeight: 500,
            textTransform: 'uppercase',
            letterSpacing: '0.03em',
            marginBottom: '6px',
          }}>
            Bukti Transfer
          </div>
          {payment.proofPath ? (
            <div>
              <ProofImageThumbnail
                paymentId={payment.id}
                onClick={() => onViewProof?.(payment.id)}
              />
              <button
                type="button"
                onClick={() => onViewProof?.(payment.id)}
                style={{
                  marginLeft: '8px',
                  padding: '4px 10px',
                  fontSize: '12px',
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-surface)',
                  borderRadius: '4px',
                  cursor: 'pointer',
                  color: 'var(--sf-text)',
                }}
              >
                Lihat Penuh
              </button>
            </div>
          ) : (
            <div style={{
              fontSize: '12px',
              color: 'var(--sf-text-muted)',
              fontStyle: 'italic',
              padding: '12px',
              background: 'var(--sf-surface)',
              borderRadius: 'var(--sf-radius-sm)',
              border: '1px solid var(--sf-border)',
            }}>
              Tidak ada bukti transfer
            </div>
          )}
        </div>
      </ModalBody>
      <ModalFooter>
        <div style={{ display: 'flex', justifyContent: 'space-between', width: '100%' }}>
          {canApprove && isApproved && onReverse ? (
            <button
              type="button"
              className="sf-btn"
              onClick={onReverse}
              style={{ color: 'var(--sf-danger)', border: '1px solid var(--sf-danger)', background: 'transparent', cursor: 'pointer' }}
            >
              Batalkan Pembayaran (Reversal)
            </button>
          ) : <div />}
          <button type="button" className="sf-btn-primary" onClick={onClose}>
            Tutup
          </button>
        </div>
      </ModalFooter>
    </Modal>
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
  onStatusChange,
}: {
  billId?: string
  open?: boolean
  onClose?: () => void
  onPayment?: (billId: string) => void
  onStatusChange?: () => void
} = {}) {
  const routeId = useParams<{ id: string }>()?.id
  const effectiveId = billId ?? routeId

  const isModal = billId !== undefined
  const open = openProp ?? true
  const navigate = useNavigate()
  const handleClose = onCloseProp ?? (() => navigate('/iuran'))
  const onPayment = onPaymentProp ?? (() => { navigate(`/iuran/${effectiveId}/pembayaran`) })

  const { user } = useAuth()
  const canApprove = canApprovePayment(user)

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [bill, setBill] = useState<IuranBill | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [verifyingId, setVerifyingId] = useState<string | null>(null)
  const [officerNotes, setOfficerNotes] = useState('')
  const [rejectError, setRejectError] = useState('')
  const [verifyingPayment, setVerifyingPayment] = useState<IuranPaymentRecord | null>(null)
  const [previewingProofId, setPreviewingProofId] = useState<string | null>(null)
  const [selectedPaymentDetail, setSelectedPaymentDetail] = useState<IuranPaymentRecord | null>(null)
  const [fundLocations, setFundLocations] = useState<FundLocation[]>([])
  const [reversingPayment, setReversingPayment] = useState<IuranPaymentRecord | null>(null)
  const [reversalReason, setReversalReason] = useState('')
  const [reversingLoading, setReversingLoading] = useState(false)
  const [reversalError, setReversalError] = useState('')

  useEffect(() => {
    const token = getSessionPair()?.accessToken
    if (token) {
      apiListFundLocations(token, { is_active: true })
        .then(setFundLocations)
        .catch(() => {})
    }
  }, [])

  // Load bill data
  const loadBill = useCallback(async () => {
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

      const nominal = parseMoney(apiBill.amount)
      const isVoluntary = Boolean(apiBill.is_voluntary || nominal === 0)
      const status: IuranStatus = (isVoluntary && nominal === 0)
        ? 'sukarela'
        : (BACKEND_STATUS_MAP[apiBill.status] ?? 'belum_bayar')

      // Map backend bill to frontend format
      const mappedBill: IuranBill = {
        id: apiBill.id,
        householdId: apiBill.household_occupancy_id,
        householdName: apiBill.head_name ?? apiBill.house_number ?? 'Unknown',
        houseNumber: apiBill.house_number ?? undefined,
        rt: apiBill.rt_code ? `RT ${apiBill.rt_code}` : 'RT —',
        iuranType: apiBill.due_name ?? 'Iuran',
        periode: apiBill.period,
        nominal,
        paidAmount: 0, // will be calculated from payments
        pendingAmount: 0, // will be calculated from PENDING payments
        status,
        isVoluntary,
        dueId: apiBill.due_id,
        dueIsActive: apiBill.due_is_active ?? true,
        payments: [], // will be loaded separately
      }

      // Load payments for this bill
      const paymentsResponse = await apiListPayments(token, { bill_id: effectiveId })

      // Calculate paidAmount from APPROVED payments only
      const approvedPayments = paymentsResponse.data.filter(
        (p) => p.status === 'APPROVED'
      )
      const paidAmount = approvedPayments.reduce(
        (sum, p) => sum + parseMoney(p.amount),
        0
      )

      // Calculate pendingAmount for remaining balance calculation
      const pendingPayments = paymentsResponse.data.filter(
        (p) => p.status === 'PENDING'
      )
      const pendingAmount = pendingPayments.reduce(
        (sum, p) => sum + parseMoney(p.amount),
        0
      )

      // Map payments to frontend format
      const paymentRecords = paymentsResponse.data.map((p) => ({
        id: p.id,
        nominal: parseMoney(p.amount),
        paidDate: p.paid_at,
        catatan: p.notes ?? undefined,
        rejectionReason: p.rejection_reason ?? undefined,
        verifiedBy: p.verified_by ?? undefined,
        verifiedByName: p.verified_by_name ?? undefined,
        officerNotes: p.officer_notes ?? undefined,
        paymentNo: p.payment_no ?? undefined,
        status: p.status as PaymentStatusDisplay,
        method: p.method,
        proofPath: p.proof_path,
        proofOriginalName: p.proof_original_name,
        proofMimeType: p.proof_mime_type,
        proofSizeBytes: p.proof_size_bytes,
      }))

      setBill({
        ...mappedBill,
        paidAmount,
        pendingAmount,
        payments: paymentRecords,
      })
    } catch (err) {
      const apiErr = err as { code?: string }
      if (apiErr.code === 'not_found') {
        setNotFound(true)
      } else {
        setError(err instanceof Error ? err.message : 'Gagal memuat data')
      }
    } finally {
      setLoading(false)
    }
  }, [effectiveId])

  useEffect(() => {
    loadBill()
  }, [loadBill])

  const remaining = useMemo(() => {
    if (!bill) return 0
    return bill.nominal - bill.paidAmount - bill.pendingAmount
  }, [bill])

  // ── Verification handlers ────────────────────────────────────────

  async function handleApprove(paymentId: string, locationId?: string) {
    if (!window.confirm('Terima pembayaran ini?')) return
    setVerifyingId(paymentId)
    setRejectError('')
    try {
      const token = getSessionPair()?.accessToken
      if (!token) {
        setError('Sesi telah berakhir. Silakan login ulang.')
        return
      }
      await apiVerifyPayment(token, paymentId, 'approve', { officerNotes: officerNotes.trim() || undefined, locationId })
      setOfficerNotes('')
      setVerifyingPayment(null)
      await loadBill()
      onStatusChange?.()
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Gagal memverifikasi'
      setRejectError(msg)
    } finally {
      setVerifyingId(null)
    }
  }

  async function handleReversePayment(e: React.FormEvent) {
    e.preventDefault()
    if (!reversingPayment) return
    if (!reversalReason.trim()) {
      setReversalError('Alasan pembatalan (reversal) wajib diisi.')
      return
    }
    setReversingLoading(true)
    setReversalError('')
    try {
      const token = getSessionPair()?.accessToken
      if (!token) throw new Error('Sesi telah berakhir. Silakan login ulang.')
      await apiReversePayment(token, reversingPayment.id, reversalReason.trim())
      setReversingPayment(null)
      setReversalReason('')
      await loadBill()
      onStatusChange?.()
    } catch (err: any) {
      setReversalError(err?.message || 'Gagal membatalkan pembayaran')
    } finally {
      setReversingLoading(false)
    }
  }

  async function handleReject(paymentId: string) {
    const notes = officerNotes.trim()
    if (!notes) {
      setRejectError('Catatan wajib diisi untuk menolak pembayaran.')
      return
    }
    setRejectError('')
    setVerifyingId(paymentId)
    try {
      const token = getSessionPair()?.accessToken
      if (!token) {
        setRejectError('Sesi telah berakhir. Silakan login ulang.')
        return
      }
      await apiVerifyPayment(token, paymentId, 'reject', {
        rejectionReason: notes,
        officerNotes: notes,
      })
      setOfficerNotes('')
      setVerifyingPayment(null)
      await loadBill()
      onStatusChange?.()
    } catch (err) {
      setRejectError(err instanceof Error ? err.message : 'Gagal menolak pembayaran')
    } finally {
      setVerifyingId(null)
    }
  }

  // ── Modal mode ───────────────────────────────────────

  if (isModal) {
    return (
      <>
        <Modal
          open={open}
          onClose={handleClose}
          width={720}
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

                <DetailContent
                  bill={bill}
                  remaining={remaining}
                  canApprove={canApprove}
                  onPendingVerify={(payment) => setVerifyingPayment(payment)}
                  onViewProof={(paymentId) => setPreviewingProofId(paymentId)}
                  onPaymentDetail={(payment) => setSelectedPaymentDetail(payment)}
                />
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

                {(remaining > 0 || bill.isVoluntary || bill.nominal === 0) && (
                  <button
                    type="button"
                    onClick={() => !bill.dueIsActive && onPayment(bill.id)}
                    disabled={!bill.dueIsActive}
                    style={{
                      padding: '8px 20px',
                      fontSize: '13px',
                      fontWeight: 600,
                      color: bill.dueIsActive ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                      background: bill.dueIsActive ? 'var(--sf-accent-soft)' : 'var(--sf-bg)',
                      border: `1px solid ${bill.dueIsActive ? 'var(--sf-accent)' : 'var(--sf-border)'}`,
                      borderRadius: 'var(--sf-radius-sm)',
                      cursor: bill.dueIsActive ? 'pointer' : 'not-allowed',
                      transition: 'background 200ms, border-color 200ms',
                      opacity: bill.dueIsActive ? 1 : 0.5,
                    }}
                  >
                    {bill.dueIsActive ? 'Bayar Sekarang' : 'Iuran Tidak Aktif'}
                  </button>
                )}
              </div>
            </ModalFooter>
          )}
        </Modal>

        {/* ── Verification popup ─────────────────────────────────── */}
        {verifyingPayment && (
          <VerificationDialog
            verifyingPayment={verifyingPayment}
            verifyingId={verifyingId}
            officerNotes={officerNotes}
            rejectError={rejectError}
            canApprove={canApprove}
            fundLocations={fundLocations}
            onClose={() => setVerifyingPayment(null)}
            onNotesChange={setOfficerNotes}
            onPreviewProof={(paymentId) => setPreviewingProofId(paymentId)}
            onReject={handleReject}
            onApprove={handleApprove}
          />
        )}

        {/* Payment Detail Modal in Modal Mode */}
        {selectedPaymentDetail && (
          <PaymentDetailModal
            payment={selectedPaymentDetail}
            canApprove={canApprove}
            onReverse={() => {
              setReversingPayment(selectedPaymentDetail)
              setSelectedPaymentDetail(null)
            }}
            onClose={() => setSelectedPaymentDetail(null)}
            onViewProof={(paymentId) => setPreviewingProofId(paymentId)}
          />
        )}

        {/* Reversal Confirmation Modal in Modal Mode */}
        {reversingPayment && (
          <Modal open onClose={() => setReversingPayment(null)} width={460}>
            <ModalHeader title="Batalkan Pembayaran (Reversal)" onClose={() => setReversingPayment(null)} />
            <form onSubmit={handleReversePayment}>
              <ModalBody>
                <div style={{ marginBottom: 12, fontSize: 13, color: 'var(--sf-text)' }}>
                  Apakah Anda yakin ingin membatalkan verifikasi pembayaran sebesar <strong>{formatRupiah(reversingPayment.nominal)}</strong>?
                </div>
                <div style={{ marginBottom: 12, fontSize: 12, color: 'var(--sf-text-muted)' }}>
                  Tagihan warga akan kembali berstatus Belum Lunas dan transaksi pengeluaran penyesuaian akan dicatat pada buku kas dan lokasi dana terkait.
                </div>
                {reversalError && (
                  <div style={{ padding: 8, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 4, marginBottom: 12, fontSize: 13 }}>
                    {reversalError}
                  </div>
                )}
                <div style={{ marginBottom: 12 }}>
                  <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Alasan Pembatalan *</label>
                  <input
                    type="text"
                    placeholder="Contoh: Bukti transfer palsu, salah konfirmasi"
                    value={reversalReason}
                    onChange={(e) => setReversalReason(e.target.value)}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                  />
                </div>
              </ModalBody>
              <ModalFooter>
                <button type="button" className="sf-btn-ghost" onClick={() => setReversingPayment(null)}>Batal</button>
                <button type="submit" className="sf-btn" disabled={reversingLoading} style={{ background: 'var(--sf-danger)', color: '#fff' }}>
                  {reversingLoading ? 'Memproses...' : 'Konfirmasi Reversal'}
                </button>
              </ModalFooter>
            </form>
          </Modal>
        )}

        {/* Proof Preview Modal — rendered after PaymentDetailModal so it stacks on top */}
        {previewingProofId && (
          <ProofPreviewModal
            paymentId={previewingProofId}
            onClose={() => setPreviewingProofId(null)}
          />
        )}
      </>
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

          <DetailContent
            bill={bill}
            remaining={remaining}
            canApprove={canApprove}
            onPendingVerify={(payment) => setVerifyingPayment(payment)}
            onViewProof={(paymentId) => setPreviewingProofId(paymentId)}
            onPaymentDetail={(payment) => setSelectedPaymentDetail(payment)}
          />

          {/* Verification Dialog in Page Mode */}
          {verifyingPayment && (
            <VerificationDialog
              verifyingPayment={verifyingPayment}
              verifyingId={verifyingId}
              officerNotes={officerNotes}
              rejectError={rejectError}
              canApprove={canApprove}
              fundLocations={fundLocations}
              onClose={() => setVerifyingPayment(null)}
              onNotesChange={setOfficerNotes}
              onPreviewProof={(paymentId) => setPreviewingProofId(paymentId)}
              onReject={handleReject}
              onApprove={handleApprove}
            />
          )}

          {/* Payment Detail Modal in Page Mode */}
          {selectedPaymentDetail && (
            <PaymentDetailModal
              payment={selectedPaymentDetail}
              canApprove={canApprove}
              onReverse={() => {
                setReversingPayment(selectedPaymentDetail)
                setSelectedPaymentDetail(null)
              }}
              onClose={() => setSelectedPaymentDetail(null)}
              onViewProof={(paymentId) => setPreviewingProofId(paymentId)}
            />
          )}

          {/* Reversal Confirmation Modal in Page Mode */}
          {reversingPayment && (
            <Modal open onClose={() => setReversingPayment(null)} width={460}>
              <ModalHeader title="Batalkan Pembayaran (Reversal)" onClose={() => setReversingPayment(null)} />
              <form onSubmit={handleReversePayment}>
                <ModalBody>
                  <div style={{ marginBottom: 12, fontSize: 13, color: 'var(--sf-text)' }}>
                    Apakah Anda yakin ingin membatalkan verifikasi pembayaran sebesar <strong>{formatRupiah(reversingPayment.nominal)}</strong>?
                  </div>
                  <div style={{ marginBottom: 12, fontSize: 12, color: 'var(--sf-text-muted)' }}>
                    Tagihan warga akan kembali berstatus Belum Lunas dan transaksi pengeluaran penyesuaian akan dicatat pada buku kas dan lokasi dana terkait.
                  </div>
                  {reversalError && (
                    <div style={{ padding: 8, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 4, marginBottom: 12, fontSize: 13 }}>
                      {reversalError}
                    </div>
                  )}
                  <div style={{ marginBottom: 12 }}>
                    <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Alasan Pembatalan *</label>
                    <input
                      type="text"
                      placeholder="Contoh: Bukti transfer palsu, salah konfirmasi"
                      value={reversalReason}
                      onChange={(e) => setReversalReason(e.target.value)}
                      style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                    />
                  </div>
                </ModalBody>
                <ModalFooter>
                  <button type="button" className="sf-btn-ghost" onClick={() => setReversingPayment(null)}>Batal</button>
                  <button type="submit" className="sf-btn" disabled={reversingLoading} style={{ background: 'var(--sf-danger)', color: '#fff' }}>
                    {reversingLoading ? 'Memproses...' : 'Konfirmasi Reversal'}
                  </button>
                </ModalFooter>
              </form>
            </Modal>
          )}

          {/* Proof Preview Modal in Page Mode — rendered after PaymentDetailModal so it stacks on top */}
          {previewingProofId && (
            <ProofPreviewModal
              paymentId={previewingProofId}
              onClose={() => setPreviewingProofId(null)}
            />
          )}
        </>
      )}
    </div>
  )
}

function VerificationDialog({
  verifyingPayment,
  verifyingId,
  officerNotes,
  rejectError,
  canApprove = true,
  fundLocations = [],
  onClose,
  onNotesChange,
  onPreviewProof,
  onReject,
  onApprove,
}: {
  verifyingPayment: IuranPaymentRecord
  verifyingId: string | null
  officerNotes: string
  rejectError: string
  canApprove?: boolean
  fundLocations?: FundLocation[]
  onClose: () => void
  onNotesChange: (notes: string) => void
  onPreviewProof: (paymentId: string) => void
  onReject: (paymentId: string) => void
  onApprove: (paymentId: string, locationId?: string) => void
}) {
  const eligibleLocations = useMemo(() => {
    const method = verifyingPayment.method?.toUpperCase() || 'CASH'
    if (method === 'CASH') {
      return fundLocations.filter((l) => (l.category === 'coh' || l.category === 'brankas') && l.is_active)
    }
    if (method === 'TRANSFER') {
      return fundLocations.filter((l) => l.category === 'rekening' && l.is_active)
    }
    return fundLocations.filter((l) => l.is_active)
  }, [fundLocations, verifyingPayment.method])

  const [selectedLocationId, setSelectedLocationId] = useState<string>(() => {
    if (eligibleLocations.length === 1) {
      return eligibleLocations[0].id
    }
    return ''
  })
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0, 0, 0, 0.65)',
        backdropFilter: 'blur(3px)',
        WebkitBackdropFilter: 'blur(3px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1100,
        padding: '16px',
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: 'var(--sf-surface)',
          borderRadius: '12px',
          border: '1px solid var(--sf-border)',
          width: '100%',
          maxWidth: '480px',
          maxHeight: 'calc(100vh - 48px)',
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '0 24px 64px rgba(0, 0, 0, 0.7), 0 0 0 1px rgba(0, 0, 0, 0.3)',
          overflow: 'hidden',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 20px',
            borderBottom: '1px solid var(--sf-border)',
            flexShrink: 0,
          }}
        >
          <div
            style={{
              fontSize: '15px',
              fontWeight: 700,
              color: 'var(--sf-text)',
            }}
          >
            Verifikasi Pembayaran
          </div>
          <button
            type="button"
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--sf-text-muted)',
              cursor: 'pointer',
              fontSize: '18px',
              padding: '4px 8px',
            }}
          >
            ✕
          </button>
        </div>

        {/* Scrollable Body */}
        <div
          style={{
            padding: '16px 20px',
            overflowY: 'auto',
            flex: '1 1 auto',
          }}
        >
          {/* Payment info */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '12px',
              borderRadius: 'var(--sf-radius-sm)',
              background: 'var(--sf-warning-soft)',
              border: '1px solid var(--sf-warning)',
              marginBottom: '16px',
            }}
          >
            <div
              style={{
                width: '36px',
                height: '36px',
                borderRadius: '50%',
                background: 'var(--sf-warning)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#fff',
                fontSize: '18px',
                flexShrink: 0,
              }}
            >
              ⏳
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div
                style={{
                  fontSize: '16px',
                  fontWeight: 700,
                  fontFamily: 'var(--sf-font-mono)',
                  color: 'var(--sf-text)',
                }}
              >
                {formatRupiah(verifyingPayment.nominal)}
              </div>
              <div
                style={{
                  fontSize: '12px',
                  color: 'var(--sf-text-muted)',
                  marginTop: '2px',
                  lineHeight: 1.5,
                }}
              >
                {formatDate(verifyingPayment.paidDate)}
                {verifyingPayment.method ? ` · ${verifyingPayment.method === 'TRANSFER' ? 'Transfer' : 'Cash'}` : ''}
                {verifyingPayment.catatan && ` · ${verifyingPayment.catatan}`}
              </div>
            </div>
          </div>

          {/* Proof of payment */}
          <div style={{ marginBottom: '16px' }}>
            <div
              style={{
                fontSize: '12px',
                color: 'var(--sf-text-muted)',
                marginBottom: '6px',
                fontWeight: 500,
              }}
            >
              Bukti Pembayaran
            </div>
            {verifyingPayment.proofPath ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <ProofImageThumbnail
                  paymentId={verifyingPayment.id}
                  onClick={() => onPreviewProof(verifyingPayment.id)}
                />
                <button
                  type="button"
                  onClick={() => onPreviewProof(verifyingPayment.id)}
                  style={{
                    padding: '4px 8px',
                    fontSize: '12px',
                    border: '1px solid var(--sf-border)',
                    background: 'var(--sf-surface)',
                    borderRadius: '4px',
                    cursor: 'pointer',
                    color: 'var(--sf-text)',
                  }}
                >
                  Lihat Bukti Ukuran Penuh
                </button>
              </div>
            ) : (
              <div
                style={{
                  fontSize: '12px',
                  color: 'var(--sf-text-muted)',
                  fontStyle: 'italic',
                }}
              >
                Belum ada bukti pembayaran
              </div>
            )}
          </div>

          {/* Lokasi Dana (GAP B) */}
          {canApprove && (
            <div style={{ marginBottom: '16px' }}>
              <label
                style={{
                  fontSize: '12px',
                  fontWeight: 600,
                  color: 'var(--sf-text)',
                  marginBottom: '6px',
                  display: 'block',
                }}
              >
                Lokasi Dana Penampungan <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>

              {eligibleLocations.length === 0 ? (
                <div
                  style={{
                    padding: '8px 12px',
                    fontSize: '12px',
                    color: 'var(--sf-danger)',
                    background: 'var(--sf-danger-soft)',
                    border: '1px solid rgba(242,73,92,0.3)',
                    borderRadius: 'var(--sf-radius-sm)',
                  }}
                >
                  Tidak ada lokasi dana aktif yang sesuai untuk metode pembayaran{' '}
                  <strong>{verifyingPayment.method || 'CASH'}</strong>.
                  Harap tambahkan lokasi dana ({verifyingPayment.method === 'TRANSFER' ? 'Rekening Bank' : 'Brankas / COH'}) di menu Lokasi Dana terlebih dahulu.
                </div>
              ) : (
                <>
                  <select
                    value={selectedLocationId}
                    onChange={(e) => setSelectedLocationId(e.target.value)}
                    style={{
                      width: '100%',
                      padding: '8px 12px',
                      fontSize: '13px',
                      color: 'var(--sf-text)',
                      background: 'var(--sf-surface)',
                      border: '1px solid var(--sf-border)',
                      borderRadius: 'var(--sf-radius-sm)',
                    }}
                  >
                    {eligibleLocations.length > 1 && (
                      <option value="">-- Pilih Rekening / Brankas Tujuan --</option>
                    )}
                    {eligibleLocations.map((loc) => (
                      <option key={loc.id} value={loc.id}>
                        {loc.name} ({loc.category.toUpperCase()})
                        {loc.bank_name ? ` - ${loc.bank_name}` : ''}
                        {loc.account_number ? ` (${loc.account_number})` : ''}
                      </option>
                    ))}
                  </select>
                  {eligibleLocations.length === 1 && (
                    <div style={{ fontSize: '11px', color: 'var(--sf-text-muted)', marginTop: '4px' }}>
                      Otomatis memilih satu-satunya lokasi yang sesuai untuk metode {verifyingPayment.method || 'CASH'}.
                    </div>
                  )}
                </>
              )}
            </div>
          )}

          {/* Officer notes */}
          {canApprove && (
            <div style={{ marginBottom: '16px' }}>
              <label
                style={{
                  fontSize: '12px',
                  color: 'var(--sf-text-muted)',
                  marginBottom: '6px',
                  display: 'block',
                }}
              >
                Catatan petugas (wajib untuk Tolak, opsional untuk Terima)
              </label>
              <textarea
                value={officerNotes}
                onChange={(e) => onNotesChange(e.target.value)}
                placeholder="Catatan verifikasi atau persetujuan..."
                rows={2}
                style={{
                  width: '100%',
                  padding: '8px 12px',
                  fontSize: '12px',
                  color: 'var(--sf-text)',
                  background: 'var(--sf-surface-subtle)',
                  border: '1px solid var(--sf-border)',
                  borderRadius: 'var(--sf-radius-sm)',
                  resize: 'vertical',
                  fontFamily: 'inherit',
                  outline: 'none',
                  boxSizing: 'border-box',
                }}
                onFocus={(e) => {
                  e.target.style.borderColor = 'var(--sf-accent)'
                }}
                onBlur={(e) => {
                  e.target.style.borderColor = 'var(--sf-border)'
                }}
              />
            </div>
          )}

          {/* Reject error */}
          {rejectError && (
            <div
              style={{
                marginBottom: '8px',
                padding: '8px 12px',
                fontSize: '12px',
                color: 'var(--sf-danger)',
                background: 'var(--sf-danger-soft)',
                border: '1px solid rgba(242,73,92,0.2)',
                borderRadius: 'var(--sf-radius-sm)',
              }}
            >
              {rejectError}
            </div>
          )}
        </div>

        {/* Fixed Footer Actions */}
        <div
          style={{
            display: 'flex',
            gap: '8px',
            justifyContent: 'space-between',
            padding: '14px 20px',
            borderTop: '1px solid var(--sf-border)',
            background: 'var(--sf-surface-subtle, var(--sf-surface))',
            flexShrink: 0,
          }}
        >
          <button
            type="button"
            onClick={onClose}
            style={{
              padding: '8px 16px',
              fontSize: '13px',
              fontWeight: 500,
              color: 'var(--sf-text)',
              background: 'var(--sf-surface)',
              border: '1px solid var(--sf-border)',
              borderRadius: 'var(--sf-radius-sm)',
              cursor: 'pointer',
            }}
          >
            {canApprove ? 'Batal' : 'Tutup'}
          </button>
          {canApprove && (
            <div style={{ display: 'flex', gap: '8px' }}>
              <button
                type="button"
                onClick={() => onReject(verifyingPayment.id)}
                disabled={verifyingId === verifyingPayment.id}
                style={{
                  padding: '8px 16px',
                  fontSize: '13px',
                  fontWeight: 600,
                  color: 'var(--sf-danger)',
                  background: 'var(--sf-danger-soft)',
                  border: '1px solid rgba(242,73,92,0.3)',
                  borderRadius: 'var(--sf-radius-sm)',
                  cursor: verifyingId === verifyingPayment.id ? 'not-allowed' : 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '4px',
                  transition: 'background 200ms, border-color 200ms',
                }}
              >
                {verifyingId === verifyingPayment.id ? (
                  'Memproses...'
                ) : (
                  <>
                    <CloseCircleOutlined style={{ fontSize: 12 }} />
                    Tolak
                  </>
                )}
              </button>
              <button
                type="button"
                onClick={() => onApprove(verifyingPayment.id, selectedLocationId || undefined)}
                disabled={verifyingId === verifyingPayment.id || eligibleLocations.length === 0 || !selectedLocationId}
                style={{
                  padding: '8px 16px',
                  fontSize: '13px',
                  fontWeight: 600,
                  color: 'var(--sf-success)',
                  background: 'var(--sf-success-soft)',
                  border: '1px solid rgba(46,204,113,0.3)',
                  borderRadius: 'var(--sf-radius-sm)',
                  cursor: (verifyingId === verifyingPayment.id || eligibleLocations.length === 0 || !selectedLocationId) ? 'not-allowed' : 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '4px',
                  transition: 'background 200ms, border-color 200ms',
                }}
              >
                {verifyingId === verifyingPayment.id ? (
                  'Memproses...'
                ) : (
                  <>
                    <CheckCircleOutlined style={{ fontSize: 12 }} />
                    Terima
                  </>
                )}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
