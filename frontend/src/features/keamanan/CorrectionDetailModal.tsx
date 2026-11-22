import { Modal, Descriptions, Tag, Space, Button, Input, message } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, SwapOutlined, SendOutlined, HourglassOutlined, CheckCircleFilled, CloseOutlined } from '@ant-design/icons'
import { apiDecideSecurityCorrection } from './securityApi'
import type { AttendanceCorrection } from './securityApi'
import { useState, type CSSProperties } from 'react'

type StatusTone = 'success' | 'warning' | 'danger' | 'info' | 'neutral'

const ATTENDANCE_STATUS_TAG: Record<string, { tone: StatusTone; label: string }> = {
  hadir: { tone: 'success', label: 'Hadir' },
  terlambat: { tone: 'warning', label: 'Terlambat' },
  sudah_absen_pulang: { tone: 'info', label: 'Sudah Absen Pulang' },
  tidak_hadir: { tone: 'danger', label: 'Tidak Hadir' },
  belum_hadir: { tone: 'neutral', label: 'Belum Hadir' },
  belum_absen_pulang: { tone: 'warning', label: 'Belum Absen Pulang' },
}

const CORRECTION_STATUS_TAG: Record<string, { tone: StatusTone; label: string }> = {
  pending: { tone: 'warning', label: 'Menunggu Persetujuan' },
  approved: { tone: 'success', label: 'Disetujui' },
  rejected: { tone: 'danger', label: 'Ditolak' },
}

function getToneTagStyle(tone: StatusTone): CSSProperties {
  switch (tone) {
    case 'success':
      return {
        background: 'var(--sf-success-soft)',
        color: 'var(--sf-success)',
        borderColor: 'color-mix(in srgb, var(--sf-success) 40%, transparent)',
        fontWeight: 600,
      }
    case 'warning':
      return {
        background: 'var(--sf-warning-soft)',
        color: 'var(--sf-warning)',
        borderColor: 'color-mix(in srgb, var(--sf-warning) 40%, transparent)',
        fontWeight: 600,
      }
    case 'danger':
      return {
        background: 'var(--sf-danger-soft)',
        color: 'var(--sf-danger)',
        borderColor: 'color-mix(in srgb, var(--sf-danger) 40%, transparent)',
        fontWeight: 600,
      }
    case 'info':
      return {
        background: 'var(--sf-info-soft)',
        color: 'var(--sf-info)',
        borderColor: 'color-mix(in srgb, var(--sf-info) 40%, transparent)',
        fontWeight: 600,
      }
    default:
      return {
        background: 'var(--sf-surface-subtle)',
        color: 'var(--sf-text-muted)',
        borderColor: 'var(--sf-border)',
        fontWeight: 600,
      }
  }
}

const CORRECTION_TYPE_LABEL: Record<string, string> = {
  status_change: 'Perubahan Status',
  time_adjustment: 'Penyesuaian Jam',
  manual_check_in: 'Check-In Manual',
  manual_check_out: 'Check-Out Manual',
}

function formatDateTime(iso?: string | null): string {
  if (!iso) return '-'
  try {
    return new Date(iso).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return iso
  }
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  try {
    const parts = dateStr.split('-')
    if (parts.length === 3) {
      return new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2])).toLocaleDateString('id-ID', {
        day: '2-digit',
        month: 'short',
        year: 'numeric',
      })
    }
  } catch {}
  return dateStr
}

interface CorrectionDetailModalProps {
  open: boolean
  correction: AttendanceCorrection | null
  onClose: () => void
  onRefresh: () => void
  canDecide: boolean
  currentUserId?: string
}

export function CorrectionDetailModal({ open, correction, onClose, onRefresh, canDecide, currentUserId }: CorrectionDetailModalProps) {
  const [deciding, setDeciding] = useState<'approve' | 'reject' | null>(null)
  const [decisionNote, setDecisionNote] = useState('')

  if (!correction) return null

  const isPending = correction.status === 'pending'
  const isSelfRequested = Boolean(currentUserId && correction.requested_by === currentUserId)
  const canDecideThis = canDecide && isPending && !isSelfRequested

  const statusMeta = CORRECTION_STATUS_TAG[correction.status] ?? { tone: 'neutral' as StatusTone, label: correction.status }
  const origMeta = ATTENDANCE_STATUS_TAG[correction.original_status] ?? { tone: 'neutral' as StatusTone, label: correction.original_status }
  const reqMeta = ATTENDANCE_STATUS_TAG[correction.requested_status] ?? { tone: 'info' as StatusTone, label: correction.requested_status }

  const handleDecide = async (approve: boolean) => {
    setDeciding(approve ? 'approve' : 'reject')
    try {
      await apiDecideSecurityCorrection(correction.id, approve, decisionNote || undefined)
      message.success(approve ? 'Koreksi disetujui' : 'Koreksi ditolak')
      setDecisionNote('')
      onRefresh()
      onClose()
    } catch (e: any) {
      message.error(e?.message || 'Gagal memproses keputusan')
    } finally {
      setDeciding(null)
    }
  }

  return (
    <Modal
      open={open}
      title="Detail Koreksi Absensi"
      onCancel={onClose}
      width={720}
      className="sf-correction-detail-modal"
      styles={{
        content: {
          background: 'var(--sf-surface)',
          color: 'var(--sf-text)',
          border: '1px solid var(--sf-border)',
        },
        header: {
          background: 'var(--sf-surface)',
          color: 'var(--sf-text)',
          borderBottom: '1px solid var(--sf-border-subtle)',
        },
        body: {
          background: 'transparent',
          color: 'var(--sf-text)',
        },
        footer: {
          background: 'transparent',
          borderTop: '1px solid var(--sf-border-subtle)',
        },
      }}
      footer={
        canDecideThis ? (
          <>
            <Button onClick={onClose}>Tutup</Button>
            <Button
              danger
              icon={<CloseCircleOutlined />}
              loading={deciding === 'reject'}
              onClick={() => {
                setDecisionNote('')
                Modal.confirm({
                  title: 'Tolak Koreksi',
                  className: 'sf-correction-confirm-modal',
                  content: (
                    <Input.TextArea
                      rows={3}
                      placeholder="Catatan keputusan (opsional)"
                      onPressEnter={(e) => {
                        const target = e.currentTarget as HTMLTextAreaElement
                        handleDecide(false).then(() => {
                          target.value = ''
                        })
                      }}
                      onChange={(e) => setDecisionNote(e.target.value)}
                    />
                  ),
                  okText: 'Tolak',
                  cancelText: 'Batal',
                  okButtonProps: { danger: true },
                  onOk: () => handleDecide(false),
                })
              }}
            >
              Tolak
            </Button>
            <Button
              type="primary"
              icon={<CheckCircleOutlined />}
              loading={deciding === 'approve'}
              onClick={() => {
                setDecisionNote('')
                Modal.confirm({
                  title: 'Setujui Koreksi',
                  className: 'sf-correction-confirm-modal',
                  content: (
                    <Input.TextArea
                      rows={3}
                      placeholder="Catatan keputusan (opsional)"
                      onPressEnter={(e) => {
                        const target = e.currentTarget as HTMLTextAreaElement
                        handleDecide(true).then(() => {
                          target.value = ''
                        })
                      }}
                      onChange={(e) => setDecisionNote(e.target.value)}
                    />
                  ),
                  okText: 'Setujui',
                  cancelText: 'Batal',
                  onOk: () => handleDecide(true),
                })
              }}
            >
              Setujui
            </Button>
          </>
        ) : (
          <Button onClick={onClose}>Tutup</Button>
        )
      }
    >
      <Descriptions
        bordered
        column={1}
        size="small"
        style={{ marginBottom: 16 }}
        styles={{
          label: {
            background: 'var(--sf-surface-subtle)',
            color: 'var(--sf-text-muted)',
            borderColor: 'var(--sf-border)',
            fontWeight: 600,
          },
          content: {
            background: 'var(--sf-surface)',
            color: 'var(--sf-text)',
            borderColor: 'var(--sf-border)',
          },
        }}
      >
        <Descriptions.Item label="Petugas">{correction.officer_name || correction.officer_user_id}</Descriptions.Item>
        <Descriptions.Item label="Jadwal">
          {formatDate(correction.shift_date)} · {correction.shift_name}
        </Descriptions.Item>
        <Descriptions.Item label="Jenis Koreksi">
          {CORRECTION_TYPE_LABEL[correction.request_type] || correction.request_type}
        </Descriptions.Item>
        <Descriptions.Item label="Status">
          <Tag style={getToneTagStyle(statusMeta.tone)}>{statusMeta.label}</Tag>
        </Descriptions.Item>
      </Descriptions>

      {/* Perubahan Status */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontWeight: 700, fontSize: 13, color: 'var(--sf-accent)', marginBottom: 8 }}>
          <SwapOutlined /> Perubahan Status
        </div>
        <div
          data-testid="correction-status-panel"
          style={{
            padding: 12,
            background: 'var(--sf-surface-subtle)',
            borderRadius: 'var(--sf-radius-md)',
            border: '1px solid var(--sf-border)',
          }}
        >
          <Space>
            <Tag style={getToneTagStyle(origMeta.tone)}>{origMeta.label}</Tag>
            <span style={{ color: 'var(--sf-text)' }}>→</span>
            <Tag style={getToneTagStyle(reqMeta.tone)}>{reqMeta.label}</Tag>
          </Space>
          {(correction.requested_checked_in_at || correction.requested_checked_out_at) && (
            <div style={{ marginTop: 8, fontSize: 12, color: 'var(--sf-text)' }}>
              {correction.requested_checked_in_at && <div>Check-In Baru: {formatDateTime(correction.requested_checked_in_at)}</div>}
              {correction.requested_checked_out_at && <div>Check-Out Baru: {formatDateTime(correction.requested_checked_out_at)}</div>}
            </div>
          )}
        </div>
      </div>

      {/* Diajukan */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontWeight: 700, fontSize: 13, color: 'var(--sf-accent)', marginBottom: 8 }}>
          <SendOutlined /> Diajukan
        </div>
        <div
          data-testid="correction-submitted-panel"
          style={{
            padding: 12,
            background: 'var(--sf-surface-subtle)',
            borderRadius: 'var(--sf-radius-md)',
            border: '1px solid var(--sf-border)',
          }}
        >
          <div style={{ fontSize: 13, color: 'var(--sf-text)' }}>Alasan: {correction.reason}</div>
          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4 }}>
            Oleh: {correction.requested_by_name || correction.requested_by}
          </div>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
            Waktu: {formatDateTime(correction.created_at)}
          </div>
        </div>
      </div>

      {/* Keputusan */}
      <div>
        <div style={{ fontWeight: 700, fontSize: 13, color: 'var(--sf-accent)', marginBottom: 8 }}>
          {isPending ? <HourglassOutlined /> : statusMeta.tone === 'success' ? <CheckCircleFilled /> : <CloseOutlined />}
          {' '}Keputusan
        </div>
        <div
          data-testid="correction-decision-panel"
          style={{
            padding: 12,
            background: isPending
              ? 'var(--sf-warning-soft)'
              : (statusMeta.tone === 'success'
                  ? 'var(--sf-success-soft)'
                  : 'var(--sf-danger-soft)'),
            borderRadius: 'var(--sf-radius-md)',
            border: `1px solid ${
              isPending
                ? 'color-mix(in srgb, var(--sf-warning) 40%, transparent)'
                : (statusMeta.tone === 'success'
                    ? 'color-mix(in srgb, var(--sf-success) 40%, transparent)'
                    : 'color-mix(in srgb, var(--sf-danger) 40%, transparent)')
            }`,
          }}
        >
          {isPending ? (
            <div style={{ fontSize: 12, fontStyle: 'italic', color: 'var(--sf-text-muted)' }}>Menunggu Konfirmasi</div>
          ) : (
            <>
              {correction.decided_by_name && <div style={{ fontSize: 12, color: 'var(--sf-text)' }}>Peninjau: {correction.decided_by_name}</div>}
              {correction.decision_note && <div style={{ fontSize: 12, marginTop: 2, color: 'var(--sf-text)' }}>Catatan: {correction.decision_note}</div>}
              {correction.decided_at && <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>Diputuskan: {formatDateTime(correction.decided_at)}</div>}
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}
