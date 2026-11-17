import { useMemo } from 'react'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { ExclamationCircleOutlined } from '@ant-design/icons'
import type { ApiWargaImportObligationWarning } from '@/app/api'
import { formatRupiah } from '@/features/iuran/iuranTypes'

const MONTH_NAMES_ID = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

function monthLabel(period: string): string {
  const m = /^(\d{4})-(\d{2})$/.exec(period)
  if (!m) return period
  const year = Number(m[1])
  const month = Number(m[2])
  if (!Number.isFinite(year) || month < 1 || month > 12) return period
  return `${MONTH_NAMES_ID[month - 1]} ${year}`
}

function rangeLabel(periods: string[]): string {
  if (periods.length === 0) return '-'
  // Sort and collapse contiguous runs: "Jan–Jul 2026, Okt–Des 2026"
  const sorted = [...periods].sort()
  const groups: string[] = []
  let runStart = sorted[0]
  let prev = sorted[0]
  const advance = (p: string) => {
    const [y, m] = p.split('-').map(Number)
    const d = new Date(Date.UTC(y, m - 1, 1))
    d.setUTCMonth(d.getUTCMonth() + 1)
    return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`
  }
  for (let i = 1; i <= sorted.length; i++) {
    const cur = sorted[i]
    if (cur && advance(prev) === cur) {
      prev = cur
      continue
    }
    if (runStart === prev) {
      groups.push(monthLabel(runStart))
    } else {
      groups.push(`${monthLabel(runStart)} – ${monthLabel(prev)}`)
    }
    if (cur) {
      runStart = cur
      prev = cur
    }
  }
  return groups.join(', ')
}

function parseAmount(s: string | undefined): number {
  if (!s) return 0
  const n = Number(s)
  return Number.isFinite(n) ? n : 0
}

export interface ImportObligationWarningsPopupProps {
  open: boolean
  onClose: () => void
  warnings: ApiWargaImportObligationWarning[]
}

/**
 * Popup shown after a successful XLSX import when the import actually caused
 * new iuran obligations to become wajib (Belum Bayar). Mirrors the menu Warga
 * post-update notification behaviour, but shows per-warga detail produced by
 * the backend transactions — not frontend estimates.
 */
export function ImportObligationWarningsPopup({
  open,
  onClose,
  warnings,
}: ImportObligationWarningsPopupProps) {
  const totals = useMemo(() => {
    let bills = 0
    let amount = 0
    for (const w of warnings) {
      bills += w.activated_bill_count
      amount += parseAmount(w.total_activated_amount)
    }
    return { bills, amount }
  }, [warnings])

  const totalSemua = warnings.length

  return (
    <Modal open={open} onClose={onClose} width={620}>
      <ModalHeader
        title="Kewajiban Iuran Berubah"
        subtitle="Import berhasil. Periksa warga yang kewajibannya bertambah wajib dibayar."
        onClose={onClose}
        icon={<ExclamationCircleOutlined />}
      />
      <ModalBody>
        <div
          role="alert"
          style={{
            border: '1px solid var(--sf-warning, #ff9830)',
            background: 'var(--sf-warning-soft, rgba(255,152,48,0.10))',
            borderRadius: 8,
            padding: '12px 14px',
            fontSize: 13,
            color: 'var(--sf-text)',
            lineHeight: 1.45,
            marginBottom: 14,
          }}
        >
          <div style={{ fontWeight: 700, marginBottom: 2 }}>
            {totalSemua === 1
              ? '1 warga mengalami perubahan kewajiban'
              : `${totalSemua} warga mengalami perubahan kewajiban`}
          </div>
          <div style={{ color: 'var(--sf-text-muted, #8a9ab0)' }}>
            Bulan-bulan yang sebelumnya <strong>Tidak Wajib</strong> sekarang menjadi{' '}
            <strong>Belum Bayar</strong>. Rincian di bawah berasal dari sinkronisasi tagihan backend,
            bukan estimasi.
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {warnings.map((w) => (
            <section
              key={w.household_occupancy_id}
              data-testid="obligation-warning-card"
              style={{
                border: '1px solid var(--sf-warning, #ff9830)',
                borderStyle: 'solid',
                borderRadius: 8,
                padding: '12px 14px',
                background: 'transparent',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
                <div>
                  <div style={{ fontWeight: 700, fontSize: 13 }}>
                    {w.house_number ? `Rumah ${w.house_number}` : 'Warga'}
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted, #8a9ab0)' }}>
                    {(w.resident_names ?? []).join(', ') || '—'}
                  </div>
                </div>
                <div style={{ textAlign: 'right', fontSize: 12 }}>
                  <div style={{ fontWeight: 700, fontSize: 15 }}>
                    {formatRupiah(parseAmount(w.total_activated_amount))}
                  </div>
                  <div style={{ color: 'var(--sf-text-muted)' }}>
                    {w.activated_bill_count} tagihan
                  </div>
                </div>
              </div>

              <div style={{ display: 'flex', gap: 18, flexWrap: 'wrap', marginTop: 8, fontSize: 12 }}>
                <div>
                  <span style={{ color: 'var(--sf-text-muted)' }}>Bulan kembali wajib:</span>{' '}
                  <strong>{rangeLabel(w.activated_periods)}</strong>
                  <span style={{ color: 'var(--sf-text-muted)' }}> ({w.activated_periods.length} bulan)</span>
                </div>
              </div>
            </section>
          ))}
        </div>

        {warnings.length > 1 && (
          <div
            style={{
              marginTop: 14,
              paddingTop: 12,
              borderTop: '1px dashed var(--sf-border)',
              fontSize: 12,
              color: 'var(--sf-text-muted)',
              display: 'flex',
              justifyContent: 'space-between',
              gap: 12,
              flexWrap: 'wrap',
            }}
          >
            <span>Total keseluruhan: <strong>{totals.bills} tagihan</strong></span>
            <span><strong>{formatRupiah(totals.amount)}</strong></span>
          </div>
        )}
      </ModalBody>
      <ModalFooter>
        <button
          type="button"
          className="sf-btn-primary"
          onClick={onClose}
        >
          Mengerti
        </button>
      </ModalFooter>
    </Modal>
  )
}
