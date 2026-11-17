/**
 * RwNotificationScreen — RW Notification Management Screen.
 *
 * Displays notification logs for the RW and allows users with
 * notification.manage permission to send manual push notifications
 * and manage templates.
 *
 * Access rules:
 * - Ketua RW, Wakil Ketua RW, Bendahara RW: notification.manage (view logs + send manual push + manage templates)
 * - Sekretaris RW: notification.read (view logs only)
 * - Others: denied
 */

import { useState, useEffect, useCallback } from 'react'
import {
  BellOutlined,
  PlusOutlined,
  SendOutlined,
  EditOutlined,
  DeleteOutlined,
  EyeOutlined,
} from '@ant-design/icons'
import {
  Button,
  Input,
  Modal,
  message,
  Typography,
  Table,
  Tag,
  Select,
  Switch,
} from 'antd'
import { useAuth } from '@/app/AuthContext'
import { getSessionPair } from '@/app/api'
import {
  apiGetRWNotifications,
  apiSendRWManualPush,
  apiListRWNotificationTemplates,
  apiCreateRWNotificationTemplate,
  apiUpdateRWNotificationTemplate,
  apiToggleRWNotificationTemplate,
  apiDeleteRWNotificationTemplate,
  apiPreviewRWTemplate,
  apiSendRWTemplate,
  type ApiRWNotificationTemplate,
  type ApiRWTemplatePreviewResponse,
} from './rw-api'
import { renderNotificationBody } from './url-utils'

const { Text } = Typography

interface RwNotificationScreenProps {
  rwId: string
  rwName?: string
}

export function RwNotificationScreen({ rwId, rwName }: RwNotificationScreenProps) {
  const { user } = useAuth()
  const [logs, setLogs] = useState<any[]>([])
  const [templates, setTemplates] = useState<ApiRWNotificationTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const pageSize = 20
  const [total, setTotal] = useState(0)

  // Modals
  const [showManualPush, setShowManualPush] = useState(false)
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [templateModalMode, setTemplateModalMode] = useState<'create' | 'edit'>('create')
  const [templateModalLoading, setTemplateModalLoading] = useState(false)
  const [manualPushLoading, setManualPushLoading] = useState(false)

  // Preview template states
  const [previewTemplate, setPreviewTemplate] = useState<ApiRWNotificationTemplate | null>(null)
  const [previewData, setPreviewData] = useState<ApiRWTemplatePreviewResponse | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)

  // Send template states
  const [sendTemplate, setSendTemplate] = useState<ApiRWNotificationTemplate | null>(null)
  const [sendTemplateTarget, setSendTemplateTarget] = useState<'all_pengurus' | 'rw_only'>('all_pengurus')
  const [sendingTemplate, setSendingTemplate] = useState(false)

  // Manual push form
  const [pushTitle, setPushTitle] = useState('')
  const [pushBody, setPushBody] = useState('')

  // Template form
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null)
  const [tmplName, setTmplName] = useState('')
  const [tmplEventType, setTmplEventType] = useState('')
  const [tmplTitle, setTmplTitle] = useState('')
  const [tmplBody, setTmplBody] = useState('')

  const isSuperAdmin = user?.systemRole === 'super_admin' || user?.role === 'super_admin'
  const rwMemberships = (user as any)?.rw_memberships || (user as any)?.rwMemberships || []
  const currentRw = rwMemberships.find((m: any) => m.rw_id === rwId)
  const isExplicit = currentRw?.source === 'explicit'
  const jabatan = (currentRw?.jabatan || '').trim().toLowerCase()

  // Final Matrix permissions:
  // Pengurus RT (rt_inferred): NO access (canRead = false, canManage = false)
  // Pengurus RW (explicit, e.g. wakil_ketua, sekretaris): READ access (canRead = true, canManage = false)
  // Ketua RW & Bendahara RW: WRITE access (canRead = true, canManage = true)
  // Super Admin: Full bypass (canRead = true, canManage = true)
  const canRead = isSuperAdmin || isExplicit
  const canManage = isSuperAdmin || (isExplicit && ['ketua', 'bendahara'].includes(jabatan))

  const loadNotifications = useCallback(async () => {
    if (!canRead) return
    setLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair) return
      const res = await apiGetRWNotifications(pair.accessToken, rwId, page, pageSize)
      setLogs(res.data)
      setTotal(res.pagination.total)
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal memuat notifikasi')
    } finally {
      setLoading(false)
    }
  }, [rwId, page, pageSize, canRead])

  const loadTemplates = useCallback(async () => {
    if (!canRead) return
    try {
      const pair = getSessionPair()
      if (!pair) return
      const tmpls = await apiListRWNotificationTemplates(pair.accessToken, rwId)
      setTemplates(tmpls)
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal memuat template notifikasi')
    }
  }, [rwId, canRead])

  useEffect(() => {
    loadNotifications()
    loadTemplates()
  }, [loadNotifications, loadTemplates])

  const handleManualPush = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!pushTitle.trim() || !pushBody.trim()) {
      message.warning('Judul dan isi notifikasi wajib diisi')
      return
    }
    setManualPushLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair) return
      await apiSendRWManualPush(pair.accessToken, rwId, {
        title: pushTitle.trim(),
        body: pushBody.trim(),
      })
      message.success('Notifikasi manual terkirim')
      setShowManualPush(false)
      setPushTitle('')
      setPushBody('')
      loadNotifications()
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal mengirim notifikasi manual')
    } finally {
      setManualPushLoading(false)
    }
  }

  const handleSaveTemplate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!tmplName.trim() || !tmplEventType.trim() || !tmplTitle.trim() || !tmplBody.trim()) {
      message.warning('Semua field wajib diisi')
      return
    }
    setTemplateModalLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair) return
      if (templateModalMode === 'create') {
        await apiCreateRWNotificationTemplate(pair.accessToken, rwId, {
          name: tmplName.trim(),
          event_type: tmplEventType.trim(),
          title: tmplTitle.trim(),
          body: tmplBody.trim(),
        })
      } else if (editingTemplateId) {
        await apiUpdateRWNotificationTemplate(pair.accessToken, rwId, editingTemplateId, {
          name: tmplName.trim(),
          title: tmplTitle.trim(),
          body: tmplBody.trim(),
        })
      }
      message.success('Template berhasil disimpan')
      setShowTemplateModal(false)
      setEditingTemplateId(null)
      setTmplName('')
      setTmplEventType('')
      setTmplTitle('')
      setTmplBody('')
      loadTemplates()
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal menyimpan template')
    } finally {
      setTemplateModalLoading(false)
    }
  }

  const handleOpenPreview = async (tmpl: ApiRWNotificationTemplate) => {
    setPreviewTemplate(tmpl)
    setPreviewLoading(true)
    setPreviewData(null)
    try {
      const pair = getSessionPair()
      if (pair) {
        const data = await apiPreviewRWTemplate(pair.accessToken, rwId, tmpl.id)
        setPreviewData(data)
      }
    } catch (_err: any) {
      // Safe fallback preview
      const fallbackTitle = tmpl.title
        .replace(/\{\{rw_name\}\}/g, rwName || 'RW 05')
        .replace(/\{\{nama\}\}/g, 'Budi Santoso')
        .replace(/\{\{period\}\}/g, 'November 2026')
        .replace(/\{\{amount\}\}/g, 'Rp 50.000')
      const fallbackBody = tmpl.body
        .replace(/\{\{rw_name\}\}/g, rwName || 'RW 05')
        .replace(/\{\{nama\}\}/g, 'Budi Santoso')
        .replace(/\{\{period\}\}/g, 'November 2026')
        .replace(/\{\{amount\}\}/g, 'Rp 50.000')
      setPreviewData({
        template_id: tmpl.id,
        name: tmpl.name,
        event_type: tmpl.event_type,
        original_title: tmpl.title,
        original_body: tmpl.body,
        rendered_title: fallbackTitle,
        rendered_body: fallbackBody,
        placeholders: [],
        sample_data: { nama: 'Budi Santoso', period: 'November 2026', amount: 'Rp 50.000', rw_name: rwName || 'RW 05' },
        is_active: tmpl.is_active,
      })
    } finally {
      setPreviewLoading(false)
    }
  }

  const handleOpenSend = (tmpl: ApiRWNotificationTemplate) => {
    if (!canManage) {
      message.error('Anda tidak memiliki izin untuk mengirim notifikasi RW')
      return
    }
    if (!tmpl.is_active) {
      message.warning('Template ini sedang nonaktif. Aktifkan template terlebih dahulu untuk mengirim.')
      return
    }
    setSendTemplate(tmpl)
    setSendTemplateTarget('all_pengurus')
  }

  const handleExecuteSendTemplate = async () => {
    if (!sendTemplate || sendingTemplate) return
    setSendingTemplate(true)
    try {
      const pair = getSessionPair()
      if (!pair) return
      const res = await apiSendRWTemplate(pair.accessToken, rwId, sendTemplate.id, {
        target: sendTemplateTarget,
      })
      message.success(`Notifikasi berhasil dikirim (${res.sent_count} penerima)`)
      setSendTemplate(null)
      loadNotifications()
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal mengirim notifikasi dari template')
    } finally {
      setSendingTemplate(false)
    }
  }

  const handleToggleTemplate = async (tmpl: ApiRWNotificationTemplate) => {
    try {
      const pair = getSessionPair()
      if (!pair) return
      await apiToggleRWNotificationTemplate(pair.accessToken, rwId, tmpl.id)
      message.success(
        tmpl.is_active ? 'Template dinonaktifkan' : 'Template diaktifkan',
      )
      loadTemplates()
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal mengubah status template')
    }
  }

  const handleDeleteTemplate = async (tmpl: ApiRWNotificationTemplate) => {
    if (!confirm('Hapus template ini?')) return
    try {
      const pair = getSessionPair()
      if (!pair) return
      await apiDeleteRWNotificationTemplate(pair.accessToken, rwId, tmpl.id)
      message.success('Template dihapus')
      loadTemplates()
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal menghapus template')
    }
  }

  if (!canRead) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        <BellOutlined style={{ fontSize: 48, marginBottom: 16 }} />
        <Text type="danger">Anda tidak memiliki akses ke Notifikasi RW.</Text>
      </div>
    )
  }

  const statusBadge = (status: string) => {
    switch (status) {
      case 'SENT':
        return <Tag color="green">Terkirim</Tag>
      case 'FAILED':
        return <Tag color="red">Gagal</Tag>
      case 'PENDING':
        return <Tag color="blue">Menunggu</Tag>
      default:
        return <Tag>{status}</Tag>
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Text style={{ fontSize: 20, fontWeight: 600 }}>
            Notifikasi {rwName ?? 'RW'}
          </Text>
          <Text style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginTop: 4 }}>
            Daftar notifikasi dan riwayat pengiriman untuk RW ini.
          </Text>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {canManage && (
            <>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => setShowManualPush(true)}
                disabled={manualPushLoading}
              >
                Kirim Notifikasi Manual
              </Button>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setTemplateModalMode('create')
                  setTmplName('')
                  setTmplEventType('')
                  setTmplTitle('')
                  setTmplBody('')
                  setShowTemplateModal(true)
                }}
              >
                Kelola Template
              </Button>
            </>
          )}
        </div>
      </div>

      <div style={{ display: 'flex', gap: 16, marginBottom: 24 }}>
        <div style={{ flex: 1, border: '1px solid var(--sf-border)', borderRadius: 'var(--sf-radius-md)', overflow: 'hidden' }}>
          <div style={{ padding: '12px 16px', background: 'var(--sf-surface)', borderBottom: '1px solid var(--sf-border)' }}>
            <Text style={{ fontWeight: 600 }}>
              Template Notifikasi{canManage ? ' (Kelola)' : ''}
            </Text>
          </div>
          <div style={{ borderTop: '1px solid var(--sf-border)' }}>
            {templates.length === 0 ? (
              <div style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                {canManage ? (
                  <>
                    <Text>Belum ada template.</Text>
                    <Text style={{ fontSize: 12, marginTop: 8, color: 'var(--sf-text-muted)' }}>
                      Klik "Kelola Template" untuk membuat template baru.
                    </Text>
                  </>
                ) : (
                  <Text>Belum ada template yang tersedia.</Text>
                )}
              </div>
            ) : (
              <div style={{ padding: 12 }}>
                {templates.map((tmpl) => (
                  <div
                    key={tmpl.id}
                    style={{
                      border: '1px solid var(--sf-border)',
                      borderRadius: 'var(--sf-radius-sm)',
                      padding: 12,
                      marginBottom: 8,
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <div style={{ flex: 1 }}>
                      <Text style={{ fontWeight: 600 }}>{tmpl.name}</Text>
                      <Text style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                        {tmpl.event_type}
                      </Text>
                      <Text style={{ fontSize: 13, marginTop: 4 }}>{tmpl.title}</Text>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <Switch
                        checked={tmpl.is_active}
                        onChange={() => handleToggleTemplate(tmpl)}
                        size="small"
                        disabled={!canManage}
                      />
                      {tmpl.is_active ? (
                        <Text style={{ fontSize: 12, marginLeft: 8, color: 'var(--sf-text-muted)' }}>
                          Aktif
                        </Text>
                      ) : (
                        <Text style={{ fontSize: 12, marginLeft: 8, color: 'var(--sf-text-muted)' }}>
                          Nonaktif
                        </Text>
                      )}
                      {/* Preview Button (Read Permission) */}
                      <Button
                        size="small"
                        icon={<EyeOutlined />}
                        onClick={() => handleOpenPreview(tmpl)}
                        title="Pratinjau tampilan notifikasi"
                      >
                        Preview
                      </Button>

                      {/* Kirim Button (Write Permission only) */}
                      {canManage && (
                        <Button
                          type="primary"
                          size="small"
                          icon={<SendOutlined />}
                          disabled={!tmpl.is_active}
                          onClick={() => handleOpenSend(tmpl)}
                          title={tmpl.is_active ? 'Kirim notifikasi menggunakan template ini' : 'Aktifkan template terlebih dahulu'}
                        >
                          Kirim
                        </Button>
                      )}

                      {canManage && (
                        <>
                          <Button
                            type="text"
                            size="small"
                            icon={<EditOutlined />}
                            onClick={() => {
                              setTemplateModalMode('edit')
                              setEditingTemplateId(tmpl.id)
                              setTmplName(tmpl.name)
                              setTmplEventType(tmpl.event_type)
                              setTmplTitle(tmpl.title)
                              setTmplBody(tmpl.body)
                              setShowTemplateModal(true)
                            }}
                          />
                          <Button
                            type="text"
                            size="small"
                            icon={<DeleteOutlined />}
                            danger
                            onClick={() => handleDeleteTemplate(tmpl)}
                          />
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div style={{ flex: 1, border: '1px solid var(--sf-border)', borderRadius: 'var(--sf-radius-md)', overflow: 'hidden' }}>
          <div style={{ padding: '12px 16px', background: 'var(--sf-surface)', borderBottom: '1px solid var(--sf-border)' }}>
            <Text style={{ fontWeight: 600 }}>Riwayat Notifikasi</Text>
          </div>
          <div style={{ borderTop: '1px solid var(--sf-border)' }}>
            {logs.length === 0 ? (
              <div style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                <Text>Belum ada notifikasi untuk RW ini.</Text>
              </div>
            ) : (
              <Table
                dataSource={logs}
                rowKey="id"
                columns={[
                  {
                    title: 'Waktu',
                    dataIndex: 'created_at',
                    width: 180,
                  },
                  {
                    title: 'Judul',
                    dataIndex: 'title',
                  },
                  {
                    title: 'Isi',
                    dataIndex: 'body',
                    render: (body: string) => renderNotificationBody(body),
                  },
                  {
                    title: 'Status',
                    dataIndex: 'status',
                    render: (text: string) => statusBadge(text),
                  },
                ]}
                pagination={{
                  current: page,
                  pageSize,
                  total,
                  onChange: (p) => setPage(p),
                }}
                loading={loading}
              />
            )}
          </div>
        </div>
      </div>

      {/* Manual Push Modal */}
      {canManage && (
        <Modal
          title="Kirim Notifikasi Manual"
          open={showManualPush}
          onOk={handleManualPush}
          okText="Kirim"
          cancelText="Batal"
          onCancel={() => setShowManualPush(false)}
          confirmLoading={manualPushLoading}
          width={520}
        >
          <form onSubmit={handleManualPush} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div>
              <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>Judul Notifikasi</label>
              <Input
                placeholder="Masukkan judul notifikasi"
                value={pushTitle}
                onChange={(e) => setPushTitle(e.target.value)}
              />
            </div>
            <div>
              <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>Isi Notifikasi</label>
              <Input.TextArea
                placeholder="Masukkan isi notifikasi"
                value={pushBody}
                onChange={(e) => setPushBody(e.target.value)}
                rows={4}
              />
            </div>
            <Text type="warning" style={{ fontSize: 12 }}>
              Notifikasi akan dikirim ke seluruh pengurus RW dan RT di bawah RW ini.
            </Text>
          </form>
        </Modal>
      )}

      {/* Template Modal */}
      {canManage && (
        <Modal
          title={templateModalMode === 'create' ? 'Buat Template Baru' : 'Ubah Template'}
          open={showTemplateModal}
          onOk={handleSaveTemplate}
          okText={templateModalMode === 'create' ? 'Buat' : 'Simpan'}
          cancelText="Batal"
          onCancel={() => setShowTemplateModal(false)}
          confirmLoading={templateModalLoading}
          width={520}
        >
          <form onSubmit={handleSaveTemplate} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div>
              <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>Nama Template</label>
              <Input
                placeholder="Masukkan nama template"
                value={tmplName}
                onChange={(e) => setTmplName(e.target.value)}
              />
            </div>
            <div>
              <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>Jenis Event</label>
              <Select
                placeholder="Pilih jenis event"
                value={tmplEventType}
                onChange={(val) => setTmplEventType(val || '')}
              >
                <Select.Option value="iuran_payment_reminder">Pengingat Pembayaran Iuran</Select.Option>
                <Select.Option value="manual_push">Notifikasi Manual</Select.Option>
              </Select>
            </div>
            <div>
              <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>Judul</label>
              <Input
                placeholder="Masukkan judul notifikasi"
                value={tmplTitle}
                onChange={(e) => setTmplTitle(e.target.value)}
              />
            </div>
            <div>
              <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>Isi Pesan</label>
              <Input.TextArea
                placeholder="Masukkan isi notifikasi"
                value={tmplBody}
                onChange={(e) => setTmplBody(e.target.value)}
                rows={4}
              />
            </div>
          </form>
        </Modal>
      )}

      {/* Preview Template Modal */}
      {previewTemplate && (
        <Modal
          title={`Preview Template: ${previewTemplate.name}`}
          open={!!previewTemplate}
          onCancel={() => {
            setPreviewTemplate(null)
            setPreviewData(null)
          }}
          footer={[
            <Button
              key="close"
              onClick={() => {
                setPreviewTemplate(null)
                setPreviewData(null)
              }}
            >
              Tutup Preview
            </Button>,
            canManage && previewTemplate.is_active ? (
              <Button
                key="send"
                type="primary"
                icon={<SendOutlined />}
                onClick={() => {
                  const tmpl = previewTemplate
                  setPreviewTemplate(null)
                  setPreviewData(null)
                  handleOpenSend(tmpl)
                }}
              >
                Lanjut Kirim
              </Button>
            ) : null,
          ].filter(Boolean)}
          width={560}
        >
          {previewLoading ? (
            <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--sf-text-muted)' }}>
              Memuat pratinjau template...
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* Event & Placeholders badge */}
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}>
                <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)' }}>Jenis Event:</span>
                <Tag color="blue">{previewTemplate.event_type}</Tag>
                <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginLeft: 8 }}>Variabel:</span>
                {previewData?.placeholders && previewData.placeholders.length > 0 ? (
                  previewData.placeholders.map((p) => (
                    <Tag key={p} color="default" style={{ fontFamily: 'monospace' }}>
                      {`{{${p}}}`}
                    </Tag>
                  ))
                ) : (
                  <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>Tidak ada variabel dinamis</span>
                )}
              </div>

              {/* Notification Banner Simulator */}
              <div
                style={{
                  background: 'linear-gradient(135deg, #0F172A 0%, #1E293B 100%)',
                  borderRadius: 12,
                  padding: 16,
                  color: '#FFFFFF',
                  border: '1px solid #334155',
                  boxShadow: '0 8px 24px rgba(0,0,0,0.25)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <div
                      style={{
                        width: 22,
                        height: 22,
                        borderRadius: 6,
                        background: '#2563EB',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: 12,
                      }}
                    >
                      <BellOutlined style={{ color: '#fff' }} />
                    </div>
                    <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: 0.5, color: '#94A3B8' }}>
                      SOCIAL FINANCE URENA • NOTIFIKASI RW
                    </span>
                  </div>
                  <span style={{ fontSize: 11, color: '#64748B' }}>Baru saja</span>
                </div>
                <div style={{ fontWeight: 700, fontSize: 15, color: '#FFFFFF', marginBottom: 6 }}>
                  {previewData?.rendered_title || previewTemplate.title}
                </div>
                <div style={{ fontSize: 13, color: '#CBD5E1', lineHeight: 1.5, whiteSpace: 'pre-wrap' }}>
                  {renderNotificationBody(previewData?.rendered_body || previewTemplate.body || '')}
                </div>
              </div>

              {/* Sample Data Explanation */}
              {previewData?.sample_data && Object.keys(previewData.sample_data).length > 0 && (
                <div style={{ background: 'rgba(0,0,0,0.03)', padding: 12, borderRadius: 8, fontSize: 12 }}>
                  <div style={{ fontWeight: 600, marginBottom: 4, color: 'var(--sf-text)' }}>
                    Data Contoh yang Digunakan:
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 6 }}>
                    {Object.entries(previewData.sample_data).map(([k, v]) => (
                      <div key={k} style={{ color: 'var(--sf-text-muted)' }}>
                        <code style={{ fontSize: 11 }}>{`{{${k}}}`}</code>: <strong>{v}</strong>
                      </div>
                    ))}
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 8 }}>
                    * Pada pengiriman aktual, variabel akan terisi otomatis dengan data sesuai konteks penerima.
                  </div>
                </div>
              )}
            </div>
          )}
        </Modal>
      )}

      {/* Send Template Modal */}
      {canManage && sendTemplate && (
        <Modal
          title={`Kirim Notifikasi: ${sendTemplate.name}`}
          open={!!sendTemplate}
          onOk={handleExecuteSendTemplate}
          okText={sendingTemplate ? 'Mengirim...' : 'Kirim Sekarang'}
          cancelText="Batal"
          onCancel={() => {
            if (!sendingTemplate) setSendTemplate(null)
          }}
          confirmLoading={sendingTemplate}
          width={540}
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div>
              <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 4, color: 'var(--sf-text)' }}>
                Target Penerima
              </div>
              <Select
                value={sendTemplateTarget}
                onChange={(val) => setSendTemplateTarget(val as 'all_pengurus' | 'rw_only')}
                style={{ width: '100%' }}
              >
                <Select.Option value="all_pengurus">
                  Semua Pengurus (Pengurus RW & Seluruh Pengurus RT di bawah RW)
                </Select.Option>
                <Select.Option value="rw_only">Pengurus RW Saja</Select.Option>
              </Select>
              <Text style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 4, display: 'block' }}>
                Hanya pengurus dalam cakupan RW aktif yang akan menerima notifikasi. Tidak ada pengiriman lintas RW.
              </Text>
            </div>

            {/* Ringkasan Template */}
            <div style={{ background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8, padding: 12 }}>
              <div style={{ fontSize: 11, fontWeight: 700, color: 'var(--sf-text-muted)', textTransform: 'uppercase', marginBottom: 6 }}>
                Ringkasan Pesan yang Akan Dikirim:
              </div>
              <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--sf-text)', marginBottom: 4 }}>
                {sendTemplate.title}
              </div>
              <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', whiteSpace: 'pre-wrap' }}>
                {sendTemplate.body}
              </div>
            </div>

            <div style={{ padding: 10, background: 'rgba(250, 173, 20, 0.1)', border: '1px solid #ffe58f', borderRadius: 6, fontSize: 12, color: '#d46b08' }}>
              Peringatan: Notifikasi push akan langsung dikirimkan ke perangkat aktif pengurus melalui FCM dan dicatat dalam riwayat notifikasi RW.
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
