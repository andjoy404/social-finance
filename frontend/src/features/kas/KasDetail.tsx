import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeftOutlined, CalendarOutlined, TagOutlined, FileTextOutlined } from '@ant-design/icons'
import { useAuth } from '@/app/AuthContext'
import { apiGetTransaction, type ApiTransaction, getSessionPair } from '@/app/api'
import { formatTanggal, getJenisVariant, getJenisLabel, formatKeteranganKas } from './kasTypes'
import { Badge } from '@/components/Badge'

export function KasDetail() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  if (!user) return null
  if (!id) {
    navigate('/kas', { replace: true })
    return null
  }

  const [transaction, setTransaction] = useState<ApiTransaction | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchTransaction = useCallback(async () => {
    if (!id) return
    setLoading(true)
    setError(null)
    try {
      const token = getSessionPair()?.accessToken ?? ''
      const data = await apiGetTransaction(token, id)
      setTransaction(data)
    } catch (e: any) {
      setError(e?.message ?? 'Gagal memuat detail')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    void fetchTransaction()
  }, [fetchTransaction])

  const handleClose = () => navigate('/kas')

  if (loading) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        Memuat detail transaksi...
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-danger)' }}>
        {error}
      </div>
    )
  }

  if (!transaction) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        Transaksi tidak ditemukan.
      </div>
    )
  }

  const isMasuk = transaction.type === 'income'

  return (
    <>
      {/* Back button header */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
        <button type="button" className="sf-btn-ghost" onClick={handleClose} style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '6px 12px' }}>
          <ArrowLeftOutlined />
          Kembali
        </button>
      </div>

      {/* Detail card */}
      <div className="sf-content-surface" style={{ padding: 0 }}>
        {/* Type badge + title */}
        <div style={{ padding: '20px 24px', borderBottom: '1px solid var(--sf-border)' }}>
          <Badge variant={getJenisVariant(transaction.type === 'income' ? 'masuk' : 'keluar')} style={{ height: 24, padding: '0 10px', fontSize: 12, marginBottom: 8 }}>
            {getJenisLabel(transaction.type === 'income' ? 'masuk' : 'keluar')}
          </Badge>
          <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--sf-text)' }}>
            {transaction.due_name && transaction.citizen_name ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <Badge variant="blue">{transaction.due_name}</Badge>
                <span>{transaction.citizen_name}{transaction.house_number ? ` · ${transaction.house_number}` : ''}</span>
              </div>
            ) : (
              transaction.description.replace(/\s*\(Bill:\s*[a-f0-9-]+\)/gi, '').replace(/Payment for bill [a-f0-9-]+\s*\(Period:\s*([^)]+)\)/i, 'Pembayaran Iuran ($1)')
            )}
          </div>
        </div>

        {/* Details grid */}
        <div style={{ padding: 24 }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 }}>
            {/* Tanggal */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>Tanggal</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 14, color: 'var(--sf-text)' }}>
                <CalendarOutlined style={{ color: 'var(--sf-text-muted)', fontSize: 14 }} />
                {formatTanggal(transaction.occurred_at)}
              </div>
            </div>

            {/* Nominal */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>Nominal</div>
              <div style={{ fontSize: 20, fontWeight: 700, fontFamily: 'monospace', color: isMasuk ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                {formatKeteranganKas(transaction.amount, transaction.type === 'income' ? 'masuk' : 'keluar')}
              </div>
            </div>

            {/* Kategori */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>Kategori</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 14, color: 'var(--sf-text)' }}>
                <TagOutlined style={{ color: 'var(--sf-text-muted)', fontSize: 14 }} />
                {transaction.category_name || '\u2014'}
              </div>
            </div>

            {/* Status */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>Status</div>
              <Badge variant={transaction.status === 'posted' ? 'green' : transaction.status === 'cancelled' ? 'red' : 'amber'}>
                {transaction.status === 'posted' ? 'Diposting' : transaction.status === 'cancelled' ? 'Dibatalkan' : 'Draft'}
              </Badge>
            </div>

            {/* Referensi Pembayaran */}
            {(transaction.payment_no || transaction.payment_id) && (
              <div>
                <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>Referensi Pembayaran</div>
                <div style={{ fontSize: 14, fontFamily: 'monospace', fontWeight: 600, color: 'var(--sf-accent)' }}>
                  {transaction.payment_no || `PAY-${transaction.payment_id?.slice(0, 8)}`}
                </div>
              </div>
            )}

            {/* Keterangan */}
            <div style={{ gridColumn: '1 / -1' }}>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>Keterangan</div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 6, fontSize: 14, color: 'var(--sf-text)' }}>
                <FileTextOutlined style={{ color: 'var(--sf-text-muted)', fontSize: 14, marginTop: 2 }} />
                {transaction.description.replace(/\s*\(Bill:\s*[a-f0-9-]+\)/gi, '').replace(/Payment for bill [a-f0-9-]+\s*\(Period:\s*([^)]+)\)/i, 'Pembayaran Iuran ($1)')}
              </div>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
