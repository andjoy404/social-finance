import React, { useState } from 'react'
import { InfoCircleOutlined, DollarOutlined } from '@ant-design/icons'
import { parseMoney, formatRupiah } from './iuranTypes'
import type { ApiDue } from '@/app/api'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'

interface DueDetailModalProps {
  due: ApiDue
  open: boolean
  onClose: () => void
}

export function DueDetailModal({ due, open, onClose }: DueDetailModalProps) {
  const [sumbangan, setSumbangan] = useState('')
  const [catatan, setCatatan] = useState('')

  if (!open) return null

  const handleClose = () => {
    setSumbangan('')
    setCatatan('')
    onClose()
  }

  const statusBadge = due.is_voluntary
    ? { variant: 'blue', label: 'Sukarela' }
    : { variant: 'green', label: 'Wajib' }

  return (
    <Modal open={open} onClose={handleClose} width={520}>
      <ModalHeader
        title={due.name}
        subtitle="Definisi Iuran Sukarela"
        onClose={handleClose}
      />
      <ModalBody>
        <div style={{ overflowY: 'auto' }}>
          {/* Info Grid */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12, marginBottom: 20 }}>
            <InfoBox icon={<DollarOutlined />} label="Nominal" value={formatRupiah(parseMoney(due.amount))} highlight />
            <InfoBox icon={<InfoCircleOutlined />} label="Status" value={statusBadge.label} badge={<Badge variant={due.is_voluntary ? 'blue' : 'green'}>{statusBadge.label}</Badge>} />
            <InfoBox icon={<InfoCircleOutlined />} label="Periode" value={formatPeriodeType(due.period_type)} />
            <InfoBox icon={<InfoCircleOutlined />} label="RT" value={due.rt_code ? `RT ${due.rt_code}` : '—'} />
          </div>

          {/* Payment Rule */}
          {due.payment_rule && (
            <InfoBox
              icon={<InfoCircleOutlined />}
              label="Aturan Pembayaran"
              value={due.payment_rule === 'full' ? 'Seluruh Saldo' : 'Sebagian'}
              style={{ marginBottom: 16 }}
            />
          )}

          {/* Sumbangan Input */}
          <div style={{ height: 20 }} />
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="sumbangan-input">Nominal Sumbangan (Rp)</label>
            <input
              id="sumbangan-input"
              className="sf-form-input"
              type="text"
              placeholder="Masukkan nominal sumbangan"
              value={formatSumbanganInput(sumbangan)}
              onChange={(e) => setSumbangan(parseSumbanganInput(e.target.value))}
            />
          </div>

          {/* Catatan Input */}
          <div style={{ height: 12 }} />
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="catatan-input">Catatan (opsional)</label>
            <textarea
              id="catatan-input"
              className="sf-form-input"
              rows={3}
              placeholder="Tambahkan catatan jika diperlukan"
              value={catatan}
              onChange={(e) => setCatatan(e.target.value)}
              style={{ resize: 'vertical' }}
            />
          </div>

          {/* Info Note — moved to bottom */}
          <div style={{ height: 20 }} />
          <div
            style={{
              background: 'var(--sf-bg-tertiary)',
              border: '1px solid var(--sf-border)',
              borderRadius: 'var(--sf-radius-md)',
              padding: '12px 14px',
              display: 'flex',
              alignItems: 'flex-start',
              gap: 10,
            }}
          >
            <InfoCircleOutlined style={{ color: 'var(--sf-info)', fontSize: 16, marginTop: 1, flexShrink: 0 }} />
            <div style={{ flex: 1, fontSize: 12, color: 'var(--sf-text-muted)', lineHeight: 1.5 }}>
              Ini adalah definisi iuran sukarela (sumbangan) dengan nominal Rp0.
              {' '}
              {due.is_voluntary
                ? 'Tagihan hanya terbentuk ketika pengurus membuat tagihan manual untuk periode tertentu.'
                : 'Item ini tidak menghasilkan tagihan otomatis. Tagihan dibuat secara manual oleh pengurus sesuai kebutuhan.'}
            </div>
          </div>
        </div>
      </ModalBody>
      <ModalFooter>
        <button type="button" className="sf-btn-secondary" onClick={handleClose}>
          Batal
        </button>
        <button type="button" className="sf-btn-primary" disabled>
          Bayar
        </button>
      </ModalFooter>
    </Modal>
  )
}

function InfoBox({
  icon,
  label,
  value,
  highlight,
  badge,
  style,
}: {
  icon: React.ReactNode
  label: string
  value: string
  highlight?: boolean
  badge?: React.ReactNode
  style?: React.CSSProperties
}) {
  const badgeColors = {
    blue: { bg: '#e3f2fd', color: '#1565c0', border: '#90caf9' },
    green: { bg: '#e8f5e9', color: '#2e7d32', border: '#a5d6a7' },
    amber: { bg: '#fff8e1', color: '#f57f17', border: '#ffe082' },
    red: { bg: '#ffebee', color: '#c62828', border: '#ef9a9a' },
  }

  // If badge is a ReactNode (e.g. <Badge>), render it directly
  if (React.isValidElement(badge)) {
    return (
      <div
        style={{
          background: highlight ? 'var(--sf-accent-soft)' : 'var(--sf-bg-secondary)',
          border: highlight ? '1px solid var(--sf-accent)' : '1px solid var(--sf-border)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '10px 12px',
          display: 'flex',
          flexDirection: 'column',
          gap: 4,
          ...style,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{ fontSize: 12, opacity: 0.5 }}>{icon}</span>
          <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--sf-text-muted)', textTransform: 'uppercase', letterSpacing: '0.03em' }}>
            {label}
          </span>
        </div>
        <div style={{ fontSize: 14, fontWeight: 600, color: highlight ? 'var(--sf-accent)' : 'var(--sf-text)' }}>
          {badge}
        </div>
      </div>
    )
  }

  // Legacy: string badge with inline colors
  return (
    <div
      style={{
        background: highlight ? 'var(--sf-accent-soft)' : 'var(--sf-bg-secondary)',
        border: highlight ? '1px solid var(--sf-accent)' : '1px solid var(--sf-border)',
        borderRadius: 'var(--sf-radius-md)',
        padding: '10px 12px',
        display: 'flex',
        flexDirection: 'column',
        gap: 4,
        ...style,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ fontSize: 12, opacity: 0.5 }}>{icon}</span>
        <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--sf-text-muted)', textTransform: 'uppercase', letterSpacing: '0.03em' }}>
          {label}
        </span>
      </div>
      <div style={{ fontSize: 14, fontWeight: 600, color: highlight ? 'var(--sf-accent)' : 'var(--sf-text)' }}>
        {badge ? (
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              padding: '2px 8px',
              borderRadius: 'var(--sf-radius-xs)',
              background: badgeColors[badge as keyof typeof badgeColors]?.bg,
              color: badgeColors[badge as keyof typeof badgeColors]?.color,
              fontSize: 12,
              fontWeight: 600,
              border: `1px solid ${badgeColors[badge as keyof typeof badgeColors]?.border}`,
            }}
          >
            {value}
          </span>
        ) : (
          value
        )}
      </div>
    </div>
  )
}

function formatPeriodeType(periodType: string): string {
  switch (periodType) {
    case 'monthly':
      return 'Bulanan'
    case 'yearly':
      return 'Tahunan'
    case 'one_time':
      return 'Sekali Waktu'
    default:
      return periodType || '—'
  }
}

function formatSumbanganInput(value: string): string {
  const digits = value.replace(/\D/g, '')
  if (!digits) return ''
  return digits.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

function parseSumbanganInput(value: string): string {
  return value.replace(/\D/g, '')
}
