/**
 * RwNotificationScreen — RW Notification Management Screen.
 *
 * Displays RW notification templates and system/personal notification history
 * behind server-resolved per-RW permissions (`rw.notification.read` /
 * `rw.notification.write` / `notification.read` / `notification.manage`).
 */

import { useState, useEffect, useCallback, useRef } from 'react'
import {
  BellOutlined,
  PlusOutlined,
  SendOutlined,
  EditOutlined,
  DeleteOutlined,
  EyeOutlined,
  PictureOutlined,
  HistoryOutlined,
  AppstoreOutlined,
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
import {
  useAuth,
  canReadRwNotifications,
  canManageRwNotifications,
} from '@/app/AuthContext'
import { getSessionPair } from '@/app/api'
import { formatJakartaDateTime } from '@/utils/jakartaTime'
import { NotificationImage } from '@/features/notification/NotificationImage'
import {
  apiGetRWNotifications,
  apiGetRWMyNotifications,
  apiSendRWManualPush,
  apiListRWNotificationTemplates,
  apiCreateRWNotificationTemplate,
  apiUpdateRWNotificationTemplate,
  apiToggleRWNotificationTemplate,
  apiDeleteRWNotificationTemplate,
  apiUploadRWNotificationImage,
  apiPreviewRWTemplate,
  apiSendRWTemplate,
  type ApiRWNotificationTemplate,
  type ApiRWTemplatePreviewResponse,
} from './rw-api'
import { renderNotificationBody } from './url-utils'
import { isSnapshotDirty, useAntdModalGuard } from '@/hooks/useUnsavedChangesGuard'

const { Text } = Typography

const MAX_IMAGE_BYTES = 5 * 1024 * 1024

const IMAGE_SUPPORTED_EVENTS = new Set([
  'manual_push',
  'rw_cash_in',
  'rw_cash_out',
])

const EVENT_LABELS: Record<string, string> = {
  manual_push: 'Notifikasi Manual RW',
  rw_cash_in: 'Kas Masuk RW',
  rw_cash_out: 'Kas Keluar RW',
  iuran_payment_reminder: 'Pengingat Pembayaran Iuran',
}

interface RwNotificationScreenProps {
  rwId: string
  rwName?: string
}

export function RwNotificationScreen({ rwId, rwName }: RwNotificationScreenProps) {
  const { user } = useAuth()
  const [activeSubTab, setActiveSubTab] = useState<'templates' | 'history'>('templates')

  const [logs, setLogs] = useState<any[]>([])
  const [templates, setTemplates] = useState<ApiRWNotificationTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [templatesLoading, setTemplatesLoading] = useState(false)
  const [page, setPage] = useState(1)
  const pageSize = 20
  const [total, setTotal] = useState(0)

  // Sub-switcher inside Riwayat Notifikasi Sistem: Sistem vs Saya (Saya only for canManage)
  const [historyTab, setHistoryTab] = useState<'system' | 'mine'>('system')
  const [myLogs, setMyLogs] = useState<any[]>([])
  const [myTotal, setMyTotal] = useState(0)
  const [myPage, setMyPage] = useState(1)
  const [myLoading, setMyLoading] = useState(false)

  // Modals
  const [showManualPush, setShowManualPush] = useState(false)
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [templateModalMode, setTemplateModalMode] = useState<'create' | 'edit'>('create')
  const [templateModalLoading, setTemplateModalLoading] = useState(false)
  const [manualPushLoading, setManualPushLoading] = useState(false)

  // Delete confirmation modal
  const [deletingTemplate, setDeletingTemplate] = useState<ApiRWNotificationTemplate | null>(null)
  const [deletingLoading, setDeletingLoading] = useState(false)

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
  const [pushTemplateId, setPushTemplateId] = useState<string>('')
  const [pushImageUrl, setPushImageUrl] = useState('')
  const [pushUploadingImage, setPushUploadingImage] = useState(false)
  const pushImageInputRef = useRef<HTMLInputElement | null>(null)
  const [detailLog, setDetailLog] = useState<any | null>(null)
  const [fullImagePreviewSrc, setFullImagePreviewSrc] = useState<string | null>(null)

  // Template form
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null)
  const [tmplName, setTmplName] = useState('')
  const [tmplEventType, setTmplEventType] = useState('manual_push')
  const [tmplTitle, setTmplTitle] = useState('')
  const [tmplBody, setTmplBody] = useState('')
  const [tmplImageUrl, setTmplImageUrl] = useState('')
  const [tmplImageChanged, setTmplImageChanged] = useState(false)
  const [uploadingImage, setUploadingImage] = useState(false)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const tmplInitialRef = useRef({
    tmplName: '',
    tmplEventType: 'manual_push',
    tmplTitle: '',
    tmplBody: '',
    tmplImageUrl: '',
  })

  const manualPushGuard = useAntdModalGuard({
    open: showManualPush,
    isDirty: showManualPush && isSnapshotDirty(
      { pushTitle, pushBody, pushTemplateId, pushImageUrl },
      { pushTitle: '', pushBody: '', pushTemplateId: '', pushImageUrl: '' },
    ),
    isSaving: manualPushLoading || pushUploadingImage,
    onClose: () => {
      setShowManualPush(false)
      setPushTitle('')
      setPushBody('')
      setPushTemplateId('')
      setPushImageUrl('')
    },
  })

  const templateGuard = useAntdModalGuard({
    open: showTemplateModal,
    isDirty: showTemplateModal && isSnapshotDirty(
      { tmplName, tmplEventType, tmplTitle, tmplBody, tmplImageUrl },
      tmplInitialRef.current,
    ),
    isSaving: templateModalLoading || uploadingImage,
    onClose: () => {
      setShowTemplateModal(false)
      setEditingTemplateId(null)
    },
  })

  const sendTemplateGuard = useAntdModalGuard({
    open: Boolean(sendTemplate),
    isDirty: Boolean(sendTemplate) && sendTemplateTarget !== 'all_pengurus',
    isSaving: sendingTemplate,
    onClose: () => setSendTemplate(null),
  })

  const canRead = canReadRwNotifications(user, rwId)
  const canManage = canManageRwNotifications(user, rwId)

  const supportsImage = IMAGE_SUPPORTED_EVENTS.has(tmplEventType)

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
    setTemplatesLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair) return
      const tmpls = await apiListRWNotificationTemplates(pair.accessToken, rwId)
      setTemplates(tmpls)
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal memuat template notifikasi')
    } finally {
      setTemplatesLoading(false)
    }
  }, [rwId, canRead])

  const loadMyNotifications = useCallback(async () => {
    if (!canManage) return
    setMyLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair) return
      const res = await apiGetRWMyNotifications(pair.accessToken, rwId, myPage, pageSize)
      setMyLogs(res.data)
      setMyTotal(res.pagination.total)
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal memuat riwayat notifikasi saya')
    } finally {
      setMyLoading(false)
    }
  }, [rwId, myPage, pageSize, canManage])

  useEffect(() => {
    loadNotifications()
    loadTemplates()
  }, [loadNotifications, loadTemplates])

  useEffect(() => {
    if (canManage && historyTab === 'mine') {
      loadMyNotifications()
    }
  }, [loadMyNotifications, canManage, historyTab])

  const openCreateTemplateModal = () => {
    setTemplateModalMode('create')
    setEditingTemplateId(null)
    tmplInitialRef.current = {
      tmplName: '',
      tmplEventType: 'manual_push',
      tmplTitle: '',
      tmplBody: '',
      tmplImageUrl: '',
    }
    setTmplName('')
    setTmplEventType('manual_push')
    setTmplTitle('')
    setTmplBody('')
    setTmplImageUrl('')
    setTmplImageChanged(false)
    setShowTemplateModal(true)
  }

  const openEditTemplateModal = (tmpl: ApiRWNotificationTemplate) => {
    setTemplateModalMode('edit')
    setEditingTemplateId(tmpl.id)
    tmplInitialRef.current = {
      tmplName: tmpl.name,
      tmplEventType: tmpl.event_type,
      tmplTitle: tmpl.title,
      tmplBody: tmpl.body,
      tmplImageUrl: tmpl.image_url || '',
    }
    setTmplName(tmpl.name)
    setTmplEventType(tmpl.event_type)
    setTmplTitle(tmpl.title)
    setTmplBody(tmpl.body)
    setTmplImageUrl(tmpl.image_url || '')
    setTmplImageChanged(false)
    setShowTemplateModal(true)
  }

  const handleImageFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return

    if (!file.type.startsWith('image/')) {
      message.error('Pilih file dengan format gambar (PNG, JPG, WebP)')
      return
    }
    if (file.size > MAX_IMAGE_BYTES) {
      message.error('Ukuran gambar maksimal 5 MB')
      return
    }
    if (!IMAGE_SUPPORTED_EVENTS.has(tmplEventType)) {
      message.warning('Jenis event ini tidak mendukung lampiran gambar')
      return
    }

    const pair = getSessionPair()
    if (!pair) {
      message.error('Sesi autentikasi tidak tersedia')
      return
    }

    setUploadingImage(true)
    try {
      const resp = await apiUploadRWNotificationImage(pair.accessToken, rwId, tmplEventType, file)
      setTmplImageUrl(resp.image_url)
      setTmplImageChanged(true)
      message.success('Gambar berhasil diunggah')
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal mengunggah gambar')
    } finally {
      setUploadingImage(false)
    }
  }

  const handleManualPushImageChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    if (!file.type.startsWith('image/')) {
      message.error('Pilih file dengan format gambar (PNG, JPG, WebP)')
      return
    }
    if (file.size > MAX_IMAGE_BYTES) {
      message.error('Ukuran gambar maksimal 5 MB')
      return
    }
    const pair = getSessionPair()
    if (!pair) {
      message.error('Sesi autentikasi tidak tersedia')
      return
    }
    setPushUploadingImage(true)
    try {
      const resp = await apiUploadRWNotificationImage(pair.accessToken, rwId, 'manual_push', file)
      setPushImageUrl(resp.image_url)
      message.success('Gambar berhasil diunggah')
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal mengunggah gambar')
    } finally {
      setPushUploadingImage(false)
    }
  }

  const handleManualPush = async (e?: React.FormEvent) => {
    e?.preventDefault()
    if (pushUploadingImage) {
      message.warning('Tunggu proses unggah gambar selesai')
      return
    }
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
        ...(pushTemplateId ? { template_id: pushTemplateId } : {}),
        ...(pushImageUrl.trim() ? { image_url: pushImageUrl.trim() } : {}),
      })
      message.success('Notifikasi manual terkirim')
      manualPushGuard.exitAfterSave(() => {
        setShowManualPush(false)
        setPushTitle('')
        setPushBody('')
        setPushTemplateId('')
        setPushImageUrl('')
      })
      loadNotifications()
      loadMyNotifications()
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal mengirim notifikasi manual')
    } finally {
      setManualPushLoading(false)
    }
  }

  const handleSaveTemplate = async (e?: React.FormEvent) => {
    e?.preventDefault()
    if (uploadingImage) {
      message.warning('Tunggu proses unggah gambar selesai')
      return
    }
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
          ...(supportsImage && tmplImageUrl.trim() ? { image_url: tmplImageUrl.trim() } : {}),
        })
      } else if (editingTemplateId) {
        await apiUpdateRWNotificationTemplate(pair.accessToken, rwId, editingTemplateId, {
          name: tmplName.trim(),
          title: tmplTitle.trim(),
          body: tmplBody.trim(),
          ...(tmplImageChanged ? { image_url: supportsImage ? tmplImageUrl.trim() : '' } : {}),
        })
      }
      message.success(
        templateModalMode === 'create'
          ? 'Template berhasil dibuat'
          : 'Template berhasil diperbarui',
      )
      templateGuard.exitAfterSave(() => {
        setShowTemplateModal(false)
        setEditingTemplateId(null)
        setTmplName('')
        setTmplEventType('manual_push')
        setTmplTitle('')
        setTmplBody('')
        setTmplImageUrl('')
        setTmplImageChanged(false)
      })
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
      sendTemplateGuard.exitAfterSave(() => setSendTemplate(null))
      loadNotifications()
      loadMyNotifications()
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

  const handleConfirmDeleteTemplate = async () => {
    if (!deletingTemplate || deletingLoading) return
    setDeletingLoading(true)
    try {
      const pair = getSessionPair()
      if (!pair) return
      await apiDeleteRWNotificationTemplate(pair.accessToken, rwId, deletingTemplate.id)
      message.success('Template berhasil dihapus')
      setDeletingTemplate(null)
      loadTemplates()
    } catch (err: any) {
      message.error(err?.message ?? 'Gagal menghapus template')
    } finally {
      setDeletingLoading(false)
    }
  }

  if (!canRead) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
        <BellOutlined style={{ fontSize: 48, marginBottom: 16 }} />
        <div>
          <Text type="danger">Anda tidak memiliki akses ke Notifikasi RW.</Text>
        </div>
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
      case 'NO_DEVICE':
        return <Tag color="gold">Perangkat Tidak Tersedia</Tag>
      default:
        return <Tag>{status}</Tag>
    }
  }

  return (
    <div>
      {/* Sub-tabs flush below parent Data RW tabs (no extra page title/subtitle gap) */}
      <div
        role="tablist"
        aria-label="Sub-tab Notifikasi RW"
        style={{
          display: 'flex',
          gap: 4,
          borderBottom: '1px solid var(--sf-border)',
          marginBottom: 16,
        }}
      >
        <button
          type="button"
          role="tab"
          aria-selected={activeSubTab === 'templates'}
          onClick={() => setActiveSubTab('templates')}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '8px 16px',
            border: 'none',
            borderBottom: activeSubTab === 'templates' ? '2px solid var(--sf-accent)' : '2px solid transparent',
            background: activeSubTab === 'templates' ? 'var(--sf-surface-hover)' : 'transparent',
            color: activeSubTab === 'templates' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
            cursor: 'pointer',
            fontSize: 13,
            fontWeight: activeSubTab === 'templates' ? 600 : 500,
            borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
          }}
        >
          <AppstoreOutlined />
          Template
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeSubTab === 'history'}
          onClick={() => setActiveSubTab('history')}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '8px 16px',
            border: 'none',
            borderBottom: activeSubTab === 'history' ? '2px solid var(--sf-accent)' : '2px solid transparent',
            background: activeSubTab === 'history' ? 'var(--sf-surface-hover)' : 'transparent',
            color: activeSubTab === 'history' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
            cursor: 'pointer',
            fontSize: 13,
            fontWeight: activeSubTab === 'history' ? 600 : 500,
            borderRadius: 'var(--sf-radius-sm) var(--sf-radius-sm) 0 0',
          }}
        >
          <HistoryOutlined />
          Riwayat Notifikasi Sistem
        </button>
      </div>

      {/* TAB 1: TEMPLATE */}
      {activeSubTab === 'templates' && (
        <div
          style={{
            border: '1px solid var(--sf-border)',
            borderRadius: 'var(--sf-radius-md)',
            overflow: 'hidden',
            background: 'var(--sf-surface)',
          }}
        >
          <div
            style={{
              padding: '12px 16px',
              borderBottom: '1px solid var(--sf-border)',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              flexWrap: 'wrap',
              gap: 12,
            }}
          >
            <div>
              <div style={{ fontWeight: 600, fontSize: 15, color: 'var(--sf-text)' }}>
                Template Notifikasi ({templates.length})
              </div>
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                Kelola template pesan notifikasi RW
              </div>
            </div>
            {canManage && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={openCreateTemplateModal}
                >
                  Tambah Template
                </Button>
                <Button
                  icon={<SendOutlined />}
                  onClick={() => setShowManualPush(true)}
                  disabled={manualPushLoading}
                >
                  Kirim Notifikasi Manual
                </Button>
              </div>
            )}
          </div>

          <div>
            {templatesLoading && templates.length === 0 ? (
              <div style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                Memuat template...
              </div>
            ) : templates.length === 0 ? (
              <div style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                <div style={{ marginBottom: 8 }}>
                  {canManage
                    ? 'Belum ada template. Buat template baru untuk notifikasi RW.'
                    : 'Belum ada template yang tersedia.'}
                </div>
                {canManage && (
                  <Button type="primary" icon={<PlusOutlined />} onClick={openCreateTemplateModal}>
                    Tambah Template
                  </Button>
                )}
              </div>
            ) : (
              <div style={{ padding: 12, display: 'flex', flexDirection: 'column', gap: 10 }}>
                {templates.map((tmpl) => (
                  <div
                    key={tmpl.id}
                    style={{
                      border: '1px solid var(--sf-border)',
                      borderRadius: 'var(--sf-radius-sm)',
                      padding: 14,
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      flexWrap: 'wrap',
                      gap: 12,
                      background: 'var(--sf-card-bg, var(--sf-surface))',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, flex: '1 1 320px', minWidth: 0 }}>
                      {tmpl.image_url && (
                        <NotificationImage
                          src={tmpl.image_url}
                          alt={`Lampiran ${tmpl.name}`}
                          compact
                          style={{
                            width: 52,
                            height: 52,
                            borderRadius: 6,
                            objectFit: 'cover',
                            flexShrink: 0,
                            border: '1px solid var(--sf-border)',
                          }}
                        />
                      )}
                      <div style={{ minWidth: 0, flex: 1 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                          <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--sf-text)' }}>
                            {tmpl.name}
                          </span>
                          <Tag color="blue" style={{ marginInlineEnd: 0 }}>
                            {EVENT_LABELS[tmpl.event_type] || tmpl.event_type}
                          </Tag>
                          {tmpl.image_url && (
                            <Tag icon={<PictureOutlined />} color="purple" style={{ marginInlineEnd: 0 }}>
                              Gambar
                            </Tag>
                          )}
                        </div>
                        <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--sf-text)', marginTop: 4 }}>
                          {tmpl.title}
                        </div>
                        <div
                          style={{
                            fontSize: 12,
                            color: 'var(--sf-text-muted)',
                            marginTop: 2,
                            whiteSpace: 'pre-wrap',
                            wordBreak: 'break-word',
                          }}
                        >
                          {tmpl.body}
                        </div>
                      </div>
                    </div>

                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                      <div style={{ display: 'inline-flex', alignItems: 'center', gap: 6, marginRight: 4 }}>
                        <Switch
                          checked={tmpl.is_active}
                          onChange={() => handleToggleTemplate(tmpl)}
                          size="small"
                          disabled={!canManage}
                          aria-label={`Status aktif ${tmpl.name}`}
                        />
                        <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                          {tmpl.is_active ? 'Aktif' : 'Nonaktif'}
                        </span>
                      </div>

                      <Button
                        size="small"
                        icon={<EyeOutlined />}
                        onClick={() => handleOpenPreview(tmpl)}
                        title="Pratinjau tampilan notifikasi"
                      >
                        Preview
                      </Button>

                      {canManage && (
                        <>
                          <Button
                            type="primary"
                            size="small"
                            icon={<SendOutlined />}
                            disabled={!tmpl.is_active}
                            onClick={() => handleOpenSend(tmpl)}
                            title={
                              tmpl.is_active
                                ? 'Kirim notifikasi menggunakan template ini'
                                : 'Aktifkan template terlebih dahulu'
                            }
                          >
                            Kirim
                          </Button>
                          <Button
                            size="small"
                            icon={<EditOutlined />}
                            onClick={() => openEditTemplateModal(tmpl)}
                            title="Ubah template"
                          >
                            Ubah
                          </Button>
                          <Button
                            size="small"
                            danger
                            icon={<DeleteOutlined />}
                            onClick={() => setDeletingTemplate(tmpl)}
                            title="Hapus template"
                          >
                            Hapus
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 2: RIWAYAT NOTIFIKASI SISTEM */}
      {activeSubTab === 'history' && (
        <div
          style={{
            border: '1px solid var(--sf-border)',
            borderRadius: 'var(--sf-radius-md)',
            overflow: 'hidden',
            background: 'var(--sf-surface)',
          }}
        >
          <div
            style={{
              padding: '12px 16px',
              borderBottom: '1px solid var(--sf-border)',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              flexWrap: 'wrap',
              gap: 8,
            }}
          >
            <Text style={{ fontWeight: 600 }}>
              {historyTab === 'mine' ? 'Riwayat Notifikasi Saya' : 'Riwayat Notifikasi Sistem'}
            </Text>
            {canManage && (
              <div style={{ display: 'flex', gap: 6 }}>
                <Button
                  size="small"
                  type={historyTab === 'system' ? 'primary' : 'default'}
                  onClick={() => setHistoryTab('system')}
                >
                  Sistem
                </Button>
                <Button
                  size="small"
                  type={historyTab === 'mine' ? 'primary' : 'default'}
                  onClick={() => setHistoryTab('mine')}
                >
                  Saya
                </Button>
              </div>
            )}
          </div>

          <div>
            {historyTab === 'mine' && canManage ? (
              myLogs.length === 0 ? (
                <div style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
                  <Text>Belum ada notifikasi yang Anda kirimkan untuk RW ini.</Text>
                </div>
              ) : (
                <Table
                  dataSource={myLogs}
                  rowKey="id"
                  columns={[
                    {
                      title: 'Waktu',
                      dataIndex: 'created_at',
                      width: 180,
                      render: (v: string) => formatJakartaDateTime(v),
                    },
                    {
                      title: 'Judul',
                      dataIndex: 'title',
                    },
                    {
                      title: 'Isi',
                      dataIndex: 'body',
                      render: (body: string, r: any) => {
                        const img = r.image_url || r.payload?.image_url
                        return (
                          <div>
                            <div>{renderNotificationBody(body)}</div>
                            {img && (
                              <div style={{ marginTop: 6 }}>
                                <NotificationImage
                                  src={img}
                                  alt="Lampiran Notifikasi"
                                  compact
                                  style={{ width: 44, height: 44, objectFit: 'cover', borderRadius: 4, cursor: 'pointer' }}
                                  onClick={() => setFullImagePreviewSrc(img)}
                                />
                              </div>
                            )}
                          </div>
                        )
                      },
                    },
                    {
                      title: 'Penerima',
                      width: 120,
                      render: (_: any, r: any) =>
                        r.delivery_counts_available
                          ? `${r.sent_count ?? 0} / ${r.recipients_count ?? 0}`
                          : '-',
                    },
                    {
                      title: 'Status',
                      dataIndex: 'status',
                      width: 140,
                      render: (text: string) => statusBadge(text),
                    },
                    {
                      title: 'Aksi',
                      width: 100,
                      render: (_: any, r: any) => (
                        <Button
                          size="small"
                          icon={<EyeOutlined />}
                          onClick={() => setDetailLog(r)}
                        >
                          Detail
                        </Button>
                      ),
                    },
                  ]}
                  pagination={{
                    current: myPage,
                    pageSize,
                    total: myTotal,
                    onChange: (p) => setMyPage(p),
                  }}
                  loading={myLoading}
                />
              )
            ) : logs.length === 0 ? (
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
                    render: (v: string) => formatJakartaDateTime(v),
                  },
                  {
                    title: 'Judul',
                    dataIndex: 'title',
                  },
                  {
                    title: 'Isi',
                    dataIndex: 'body',
                    render: (body: string, r: any) => {
                      const img = r.image_url || r.payload?.image_url
                      return (
                        <div>
                          <div>{renderNotificationBody(body)}</div>
                          {img && (
                            <div style={{ marginTop: 6 }}>
                              <NotificationImage
                                src={img}
                                alt="Lampiran Notifikasi"
                                compact
                                style={{ width: 44, height: 44, objectFit: 'cover', borderRadius: 4, cursor: 'pointer' }}
                                onClick={() => setFullImagePreviewSrc(img)}
                              />
                            </div>
                          )}
                        </div>
                      )
                    },
                  },
                  {
                    title: 'Status',
                    dataIndex: 'status',
                    width: 160,
                    render: (text: string) => statusBadge(text),
                  },
                  {
                    title: 'Aksi',
                    width: 100,
                    render: (_: any, r: any) => (
                      <Button
                        size="small"
                        icon={<EyeOutlined />}
                        onClick={() => setDetailLog(r)}
                      >
                        Detail
                      </Button>
                    ),
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
      )}

      {/* Manual Push Modal */}
      {canManage && (
        <Modal
          title="Kirim Notifikasi Manual"
          open={showManualPush}
          onOk={() => handleManualPush()}
          okText="Kirim"
          cancelText="Batal"
          onCancel={manualPushGuard.onCancel}
          keyboard={manualPushGuard.keyboard}
          maskClosable={manualPushGuard.maskClosable}
          closable={manualPushGuard.closable}
          confirmLoading={manualPushLoading}
          okButtonProps={{ disabled: pushUploadingImage }}
          width={560}
        >
          <form onSubmit={handleManualPush} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {templates.some((t) => t.is_active && t.event_type === 'manual_push') && (
              <div>
                <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                  Gunakan Template (Opsional)
                </label>
                <Select
                  allowClear
                  placeholder="Pilih template notifikasi manual..."
                  value={pushTemplateId || undefined}
                  style={{ width: '100%' }}
                  onChange={(val) => {
                    const id = val || ''
                    setPushTemplateId(id)
                    if (id) {
                      const found = templates.find((t) => t.id === id)
                      if (found) {
                        setPushTitle(found.title)
                        setPushBody(found.body)
                        setPushImageUrl(found.image_url || '')
                      }
                    }
                  }}
                >
                  {templates
                    .filter((t) => t.is_active && t.event_type === 'manual_push')
                    .map((t) => (
                      <Select.Option key={t.id} value={t.id}>
                        {t.name}
                      </Select.Option>
                    ))}
                </Select>
              </div>
            )}
            <div>
              <label htmlFor="rw-manual-title" style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                Judul Notifikasi
              </label>
              <Input
                id="rw-manual-title"
                placeholder="Masukkan judul notifikasi"
                value={pushTitle}
                onChange={(e) => setPushTitle(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="rw-manual-body" style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                Isi Notifikasi
              </label>
              <Input.TextArea
                id="rw-manual-body"
                placeholder="Masukkan isi notifikasi"
                value={pushBody}
                onChange={(e) => setPushBody(e.target.value)}
                rows={4}
              />
            </div>
            <div>
              <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                Lampiran Gambar (Opsional)
              </label>
              <input
                ref={pushImageInputRef}
                type="file"
                accept="image/*"
                hidden
                onChange={handleManualPushImageChange}
              />
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <Button
                  icon={<PictureOutlined />}
                  loading={pushUploadingImage}
                  onClick={() => pushImageInputRef.current?.click()}
                >
                  {pushImageUrl ? 'Ganti Gambar' : 'Pilih Gambar'}
                </Button>
                {pushImageUrl && (
                  <Button
                    danger
                    size="small"
                    onClick={() => setPushImageUrl('')}
                    disabled={pushUploadingImage}
                  >
                    Hapus Gambar
                  </Button>
                )}
                <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                  PNG/JPG/WebP, maksimal 5 MB
                </span>
              </div>
              {pushImageUrl && (
                <div style={{ marginTop: 10 }}>
                  <NotificationImage
                    src={pushImageUrl}
                    alt="Preview Lampiran Manual RW"
                    style={{
                      maxWidth: '100%',
                      maxHeight: 160,
                      borderRadius: 8,
                      objectFit: 'contain',
                      border: '1px solid var(--sf-border)',
                    }}
                  />
                </div>
              )}
            </div>
            <Text type="warning" style={{ fontSize: 12 }}>
              Notifikasi akan dikirim ke seluruh pengurus RW dan RT di bawah RW ini.
            </Text>
          </form>
        </Modal>
      )}

      {/* Create / Edit Template Modal */}
      {canManage && (
        <Modal
          title={templateModalMode === 'create' ? 'Tambah Template Baru' : 'Ubah Template'}
          open={showTemplateModal}
          onOk={() => handleSaveTemplate()}
          okText={templateModalMode === 'create' ? 'Simpan Template' : 'Simpan Perubahan'}
          cancelText="Batal"
          onCancel={templateGuard.onCancel}
          keyboard={templateGuard.keyboard}
          maskClosable={templateGuard.maskClosable}
          closable={templateGuard.closable}
          confirmLoading={templateModalLoading}
          okButtonProps={{ disabled: uploadingImage }}
          width={560}
        >
          <form onSubmit={handleSaveTemplate} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div>
              <label htmlFor="rw-tmpl-name" style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                Nama Template
              </label>
              <Input
                id="rw-tmpl-name"
                placeholder="Masukkan nama template"
                value={tmplName}
                onChange={(e) => setTmplName(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="rw-tmpl-event" style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                Jenis Event
              </label>
              <Select
                id="rw-tmpl-event"
                placeholder="Pilih jenis event"
                value={tmplEventType}
                disabled={templateModalMode === 'edit'}
                style={{ width: '100%' }}
                onChange={(val) => {
                  const next = val || 'manual_push'
                  setTmplEventType(next)
                  if (!IMAGE_SUPPORTED_EVENTS.has(next)) {
                    setTmplImageUrl('')
                    setTmplImageChanged(true)
                  }
                }}
              >
                <Select.Option value="manual_push">Notifikasi Manual RW</Select.Option>
                <Select.Option value="rw_cash_in">Kas Masuk RW</Select.Option>
                <Select.Option value="rw_cash_out">Kas Keluar RW</Select.Option>
                <Select.Option value="iuran_payment_reminder">Pengingat Pembayaran Iuran</Select.Option>
              </Select>
            </div>
            <div>
              <label htmlFor="rw-tmpl-title" style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                Judul
              </label>
              <Input
                id="rw-tmpl-title"
                placeholder="Masukkan judul notifikasi"
                value={tmplTitle}
                onChange={(e) => setTmplTitle(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="rw-tmpl-body" style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                Isi Pesan
              </label>
              <Input.TextArea
                id="rw-tmpl-body"
                placeholder="Masukkan isi notifikasi"
                value={tmplBody}
                onChange={(e) => setTmplBody(e.target.value)}
                rows={4}
              />
            </div>

            {supportsImage && (
              <div>
                <label style={{ fontWeight: 600, fontSize: 13, marginBottom: 6, display: 'block' }}>
                  Lampiran Gambar (Opsional)
                </label>
                <input
                  ref={imageInputRef}
                  type="file"
                  accept="image/*"
                  hidden
                  onChange={handleImageFileChange}
                />
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <Button
                    icon={<PictureOutlined />}
                    loading={uploadingImage}
                    onClick={() => imageInputRef.current?.click()}
                  >
                    {tmplImageUrl ? 'Ganti Gambar' : 'Pilih Gambar'}
                  </Button>
                  {tmplImageUrl && (
                    <Button
                      danger
                      size="small"
                      onClick={() => {
                        setTmplImageUrl('')
                        setTmplImageChanged(true)
                      }}
                      disabled={uploadingImage}
                    >
                      Hapus Gambar
                    </Button>
                  )}
                  <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                    PNG/JPG/WebP, maksimal 5 MB
                  </span>
                </div>
                {tmplImageUrl && (
                  <div style={{ marginTop: 10 }}>
                    <NotificationImage
                      src={tmplImageUrl}
                      alt="Preview Lampiran Template RW"
                      style={{
                        maxWidth: '100%',
                        maxHeight: 160,
                        borderRadius: 8,
                        objectFit: 'contain',
                        border: '1px solid var(--sf-border)',
                      }}
                    />
                  </div>
                )}
              </div>
            )}
          </form>
        </Modal>
      )}

      {/* Delete Template Confirmation Modal */}
      {canManage && deletingTemplate && (
        <Modal
          title="Hapus Template"
          open={!!deletingTemplate}
          onOk={handleConfirmDeleteTemplate}
          okText="Hapus"
          okButtonProps={{ danger: true }}
          cancelText="Batal"
          onCancel={() => {
            if (!deletingLoading) setDeletingTemplate(null)
          }}
          confirmLoading={deletingLoading}
        >
          <p style={{ margin: 0 }}>
            Apakah Anda yakin ingin menghapus template{' '}
            <strong>{deletingTemplate.name}</strong>? Tindakan ini tidak dapat dibatalkan.
          </p>
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
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}>
                <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)' }}>Jenis Event:</span>
                <Tag color="blue">{EVENT_LABELS[previewTemplate.event_type] || previewTemplate.event_type}</Tag>
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
                {previewTemplate.image_url && (
                  <div style={{ marginTop: 12 }}>
                    <NotificationImage
                      src={previewTemplate.image_url}
                      alt="Banner Template RW"
                      style={{
                        width: '100%',
                        maxHeight: 200,
                        objectFit: 'contain',
                        borderRadius: 8,
                      }}
                    />
                  </div>
                )}
              </div>

              {previewData?.sample_data && Object.keys(previewData.sample_data).length > 0 && (
                <div style={{ background: 'rgba(148, 163, 184, 0.08)', padding: 12, borderRadius: 8, fontSize: 12 }}>
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
          onCancel={sendTemplateGuard.onCancel}
          keyboard={sendTemplateGuard.keyboard}
          maskClosable={sendTemplateGuard.maskClosable}
          closable={sendTemplateGuard.closable}
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
              {sendTemplate.image_url && (
                <div style={{ marginTop: 10 }}>
                  <NotificationImage
                    src={sendTemplate.image_url}
                    alt="Lampiran Kirim Template RW"
                    style={{ maxHeight: 140, borderRadius: 6, objectFit: 'contain' }}
                  />
                </div>
              )}
            </div>

            <div style={{ padding: 10, background: 'rgba(250, 173, 20, 0.1)', border: '1px solid #ffe58f', borderRadius: 6, fontSize: 12, color: '#d46b08' }}>
              Peringatan: Notifikasi push akan langsung dikirimkan ke perangkat aktif pengurus melalui FCM dan dicatat dalam riwayat notifikasi RW.
            </div>
          </div>
        </Modal>
      )}
      {/* Detail / Bubble Preview Modal for RW Notification History */}
      {detailLog && (() => {
        const logImage = detailLog.image_url || detailLog.payload?.image_url || detailLog.payload?.image || ''
        return (
          <Modal
            open={!!detailLog}
            title="Detail Notifikasi RW"
            onCancel={() => setDetailLog(null)}
            footer={[
              <Button key="close" onClick={() => setDetailLog(null)}>
                Tutup
              </Button>,
            ]}
            width={560}
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
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
                        background: '#7C3AED',
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
                  <span style={{ fontSize: 11, color: '#64748B' }}>
                    {formatJakartaDateTime(detailLog.created_at)}
                  </span>
                </div>
                <div style={{ fontWeight: 700, fontSize: 15, color: '#FFFFFF', marginBottom: 6 }}>
                  {detailLog.title}
                </div>
                <div style={{ fontSize: 13, color: '#CBD5E1', lineHeight: 1.5, whiteSpace: 'pre-wrap' }}>
                  {renderNotificationBody(detailLog.body || '')}
                </div>
                {logImage && (
                  <div style={{ marginTop: 12 }}>
                    <NotificationImage
                      src={logImage}
                      alt="Banner Detail Notifikasi RW"
                      style={{
                        width: '100%',
                        maxHeight: 220,
                        objectFit: 'contain',
                        borderRadius: 8,
                        cursor: 'pointer',
                      }}
                      onClick={() => setFullImagePreviewSrc(logImage)}
                    />
                  </div>
                )}
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '130px 1fr', gap: '8px 12px', fontSize: 13 }}>
                <span style={{ color: 'var(--sf-text-muted)' }}>Jenis Event</span>
                <span><Tag color="blue">{EVENT_LABELS[detailLog.event_type] || detailLog.event_type}</Tag></span>
                <span style={{ color: 'var(--sf-text-muted)' }}>Status</span>
                <span>{statusBadge(detailLog.status)}</span>
                {detailLog.delivery_counts_available && (
                  <>
                    <span style={{ color: 'var(--sf-text-muted)' }}>Penerima</span>
                    <span>{`${detailLog.sent_count ?? 0} / ${detailLog.recipients_count ?? 0}`}</span>
                  </>
                )}
              </div>
            </div>
          </Modal>
        )
      })()}
      {/* Full Image Preview Modal */}
      {fullImagePreviewSrc && (
        <Modal
          open={!!fullImagePreviewSrc}
          title="Lampiran Notifikasi"
          footer={null}
          onCancel={() => setFullImagePreviewSrc(null)}
          width={640}
        >
          <div style={{ textAlign: 'center', padding: '12px 0' }}>
            <NotificationImage
              src={fullImagePreviewSrc}
              alt="Lampiran Notifikasi"
              style={{ maxWidth: '100%', maxHeight: '70vh', objectFit: 'contain' }}
            />
          </div>
        </Modal>
      )}
      {manualPushGuard.confirmModal}
      {templateGuard.confirmModal}
      {sendTemplateGuard.confirmModal}
    </div>
  )
}
