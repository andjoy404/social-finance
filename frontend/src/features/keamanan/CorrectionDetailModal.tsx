import { Modal, Descriptions, Tag, Space, Button, Input, message } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, SwapOutlined, SendOutlined, HourglassOutlined, CheckCircleFilled, CloseOutlined } from '@ant-design/icons'
import { apiDecideSecurityCorrection } from './securityApi'
import type { AttendanceCorrection } from './securityApi'
import { useState } from 'react'

const ATTENDANCE_STATUS_TAG: Record<string, { color: string; label: string }> = {
  hadir: { color: 'green', label: 'Hadir' },
  terlambat: { color: 'orange', label: 'Terlambat' },
  sudah_absen_pulang: { color: 'blue', label: 'Sudah Absen Pulang' },
  tidak_hadir: { color: 'red', label: 'Tidak Hadir' },
  belum_hadir: { color: 'default', label: 'Belum Hadir' },
  belum_absen_pulang: { color: 'gold', label: 'Belum Absen Pulang' },
}

const CORRECTION_STATUS_TAG: Record<string, { color: string; label: string }> = {
  pending: { color: 'gold', label: 'Menunggu Persetujuan' },
  approved: { color: 'green', label: 'Disetujui' },
  rejected: { color: 'red', label: 'Ditolak' },
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

  const statusMeta = CORRECTION_STATUS_TAG[correction.status] ?? { color: 'default', label: correction.status }
  const origMeta = ATTENDANCE_STATUS_TAG[correction.original_status] ?? { color: 'default', label: correction.original_status }
  const reqMeta = ATTENDANCE_STATUS_TAG[correction.requested_status] ?? { color: 'blue', label: correction.requested_status }

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
      <Descriptions bordered column={1} size="small" style={{ marginBottom: 16 }}>
        <Descriptions.Item label="Petugas">{correction.officer_name || correction.officer_user_id}</Descriptions.Item>
        <Descriptions.Item label="Jadwal">
          {formatDate(correction.shift_date)} · {correction.shift_name}
        </Descriptions.Item>
        <Descriptions.Item label="Jenis Koreksi">
          {CORRECTION_TYPE_LABEL[correction.request_type] || correction.request_type}
        </Descriptions.Item>
        <Descriptions.Item label="Status">
          <Tag color={statusMeta.color}>{statusMeta.label}</Tag>
        </Descriptions.Item>
      </Descriptions>

      {/* Perubahan Status */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontWeight: 700, fontSize: 13, color: 'var(--sf-primary, #1677ff)', marginBottom: 8 }}>
          <SwapOutlined /> Perubahan Status
        </div>
        <div style={{
          padding: 12,
          background: 'var(--sf-surface-2, #f5f5f5)',
          borderRadius: 8,
          border: '1px solid #d9d9d9',
        }}>
          <Space>
            <Tag color={origMeta.color}>{origMeta.label}</Tag>
            <span>→</span>
            <Tag color={reqMeta.color}>{reqMeta.label}</Tag>
          </Space>
          {(correction.requested_checked_in_at || correction.requested_checked_out_at) && (
            <div style={{ marginTop: 8, fontSize: 12 }}>
              {correction.requested_checked_in_at && <div>Check-In Baru: {formatDateTime(correction.requested_checked_in_at)}</div>}
              {correction.requested_checked_out_at && <div>Check-Out Baru: {formatDateTime(correction.requested_checked_out_at)}</div>}
            </div>
          )}
        </div>
      </div>

      {/* Diajukan */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ fontWeight: 700, fontSize: 13, color: 'var(--sf-primary, #1677ff)', marginBottom: 8 }}>
          <SendOutlined /> Diajukan
        </div>
        <div style={{
          padding: 12,
          background: 'var(--sf-surface-2, #f5f5f5)',
          borderRadius: 8,
          border: '1px solid #d9d9d9',
        }}>
          <div style={{ fontSize: 13 }}>Alasan: {correction.reason}</div>
          <div style={{ fontSize: 12, color: '#888', marginTop: 4 }}>
            Oleh: {correction.requested_by_name || correction.requested_by}
          </div>
          <div style={{ fontSize: 11, color: '#999', marginTop: 2 }}>
            Waktu: {formatDateTime(correction.created_at)}
          </div>
        </div>
      </div>

      {/* Keputusan */}
      <div>
        <div style={{ fontWeight: 700, fontSize: 13, color: 'var(--sf-primary, #1677ff)', marginBottom: 8 }}>
          {isPending ? <HourglassOutlined /> : statusMeta.color === 'green' ? <CheckCircleFilled /> : <CloseOutlined />}
          {' '}Keputusan
        </div>
        <div style={{
          padding: 12,
          background: isPending ? '#fffbe6' : (statusMeta.color === 'green' ? '#f6ffed' : '#fff2f0'),
          borderRadius: 8,
          border: `1px solid ${isPending ? '#ffe58f' : (statusMeta.color === 'green' ? '#b7eb8f' : '#ffccc7')}`,
        }}>
          {isPending ? (
            <div style={{ fontSize: 12, fontStyle: 'italic', color: '#888' }}>Menunggu Konfirmasi</div>
          ) : (
            <>
              {correction.decided_by_name && <div style={{ fontSize: 12 }}>Peninjau: {correction.decided_by_name}</div>}
              {correction.decision_note && <div style={{ fontSize: 12, marginTop: 2 }}>Catatan: {correction.decision_note}</div>}
              {correction.decided_at && <div style={{ fontSize: 11, color: '#999', marginTop: 2 }}>Diputuskan: {formatDateTime(correction.decided_at)}</div>}
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}
