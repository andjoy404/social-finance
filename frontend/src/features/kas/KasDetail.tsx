import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeftOutlined, CalendarOutlined, TagOutlined, FileTextOutlined, LinkOutlined } from '@ant-design/icons'
import { useAuth } from '@/app/AuthContext'
import { KAS_TRANSACTIONS } from './kasMockData'
import type { KasTransaction } from './kasTypes'
import { formatRupiah, formatTanggal, getJenisVariant, getJenisLabel, formatKeteranganKas } from './kasTypes'
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

  const [transaction, setTransaction] = useState<KasTransaction | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchTransaction = useCallback(() => {
    setLoading(true)
    const found = KAS_TRANSACTIONS.find((t) => t.id === id)
    setTransaction(found ?? null)
    setLoading(false)
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

  if (!transaction) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        Transaksi tidak ditemukan.
      </div>
    )
  }

  const isMasuk = transaction.jenis === 'masuk'

  return (
    <>
      {/* Back button header */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
        <button
          type="button"
          className="sf-btn-ghost"
          onClick={handleClose}
          style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '6px 12px' }}
        >
          <ArrowLeftOutlined />
          Kembali
        </button>
      </div>

      {/* Detail card */}
      <div className="sf-content-surface" style={{ padding: 0 }}>
        {/* Type badge + title */}
        <div style={{ padding: '20px 24px', borderBottom: '1px solid var(--sf-border)' }}>
          <Badge variant={getJenisVariant(transaction.jenis)} style={{ height: 24, padding: '0 10px', fontSize: 12, marginBottom: 8 }}>
            {getJenisLabel(transaction.jenis)}
          </Badge>
          <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--sf-text)' }}>
            {transaction.keterangan}
          </div>
        </div>

        {/* Details grid */}
        <div style={{ padding: 24 }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 }}>
            {/* Tanggal */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>
                Tanggal
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 14, color: 'var(--sf-text)' }}>
                <CalendarOutlined style={{ color: 'var(--sf-text-muted)', fontSize: 14 }} />
                {formatTanggal(transaction.tanggal)}
              </div>
            </div>

            {/* Nominal */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>
                Nominal
              </div>
              <div style={{ fontSize: 20, fontWeight: 700, fontFamily: 'monospace', color: isMasuk ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                {formatKeteranganKas(transaction.nominal, transaction.jenis)}
              </div>
            </div>

            {/* Kategori */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>
                Kategori
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 14, color: 'var(--sf-text)' }}>
                <TagOutlined style={{ color: 'var(--sf-text-muted)', fontSize: 14 }} />
                {transaction.kategori}
              </div>
            </div>

            {/* Saldo */}
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>
                Saldo Berjalan
              </div>
              <div style={{ fontSize: 20, fontWeight: 700, fontFamily: 'monospace', color: transaction.saldo >= 0 ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                {formatRupiah(transaction.saldo)}
              </div>
            </div>

            {/* Keterangan */}
            <div style={{ gridColumn: '1 / -1' }}>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>
                Keterangan
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 6, fontSize: 14, color: 'var(--sf-text)' }}>
                <FileTextOutlined style={{ color: 'var(--sf-text-muted)', fontSize: 14, marginTop: 2 }} />
                {transaction.keterangan}
              </div>
            </div>

            {/* Referensi */}
            <div style={{ gridColumn: '1 / -1' }}>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 6 }}>
                Referensi
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 6, fontSize: 14, color: 'var(--sf-text-muted)' }}>
                <LinkOutlined style={{ color: 'var(--sf-text-muted)', fontSize: 14, marginTop: 2 }} />
                {transaction.referensi || 'Tidak ada referensi'}
              </div>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
