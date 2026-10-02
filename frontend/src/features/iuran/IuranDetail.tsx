import { useState, useEffect } from 'react'
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
import {
  BILL_DATA,
  formatRupiah,
  formatFullPeriodeLabel,
  getStatusVariant,
  getStatusLabel,
  type IuranBill,
} from './iuranTypes'

function findBill(id: string): IuranBill | undefined {
  return BILL_DATA.find((b) => b.id === id)
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
}

export function IuranDetail() {
  const { id } = useParams<{ id: string }>()

  const navigateBack = () => {
    window.history.back()
  }

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [bill, setBill] = useState<IuranBill | null>(null)
  const [notFound, setNotFound] = useState(false)

  useEffect(() => {
    setLoading(true)
    setTimeout(() => {
      if (!id) {
        setError('ID tagihan tidak valid')
        setLoading(false)
        return
      }
      const found = findBill(id)
      if (!found) {
        setNotFound(true)
      } else {
        setBill(found)
      }
      setLoading(false)
    }, 400)
  }, [id])

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
          onClick={navigateBack}
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
      {!loading && !notFound && !error && bill && (
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
                  <WarningOutlined style={{ marginRight: 4, color: bill.paidAmount >= bill.nominal ? 'var(--sf-success)' : 'var(--sf-warning)' }} />
                  Sisa Tagihan
                </div>
                <div style={{
                  fontSize: '14px',
                  color: bill.paidAmount >= bill.nominal ? 'var(--sf-success)' : 'var(--sf-warning)',
                  fontWeight: 600,
                }}>
                  {formatRupiah(bill.nominal - bill.paidAmount)}
                </div>
              </div>
            </div>

            {/* Actions */}
            {bill.paidAmount < bill.nominal && (
              <div style={{ marginTop: 'var(--sp-lg)', display: 'flex', gap: '8px' }}>
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
      )}
    </div>
  )
}
