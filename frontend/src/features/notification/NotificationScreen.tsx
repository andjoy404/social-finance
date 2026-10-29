import { useState, useEffect, useCallback, useMemo } from 'react'
import {
  BellOutlined,
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  StopOutlined,
  HistoryOutlined,
  AppstoreOutlined,
  MobileOutlined,
  ReloadOutlined,
  SendOutlined,
  ExperimentOutlined,
  PictureOutlined,
} from '@ant-design/icons'
import { useAuth } from '@/app/AuthContext'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'
import {
  getSessionPair,
  apiGetNotificationSystemTypes,
  apiGetNotificationTemplates,
  apiCreateNotificationTemplate,
  apiUpdateNotificationTemplate,
  apiToggleNotificationTemplateActive,
  apiDeleteNotificationTemplate,
  apiGetNotificationLogs,
  apiUploadNotificationImage,
  apiTriggerManualPush,
  apiTestPush,
  type ApiSystemNotificationType,
  type ApiNotificationTemplate,
  type ApiNotificationLog,
} from '@/app/api'

export function NotificationScreen() {
  const { user } = useAuth()
  const isSuperAdmin = user?.systemRole === 'super_admin'

  const [activeTab, setActiveTab] = useState<'system' | 'templates' | 'logs'>('system')

  // Data states
  const [systemTypes, setSystemTypes] = useState<ApiSystemNotificationType[]>([])
  const [templates, setTemplates] = useState<ApiNotificationTemplate[]>([])
  const [logs, setLogs] = useState<ApiNotificationLog[]>([])
  const [logsTotal, setLogsTotal] = useState(0)
  const [logsPage, setLogsPage] = useState(1)
  const [logsPageSize] = useState(15)
  const [logsEventType, setLogsEventType] = useState('')

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [createForm, setCreateForm] = useState({
    name: '',
    event_type: '',
    title: '',
    body: '',
    image_url: '',
  })
  const [editingTemplate, setEditingTemplate] = useState<ApiNotificationTemplate | null>(null)
  const [editForm, setEditForm] = useState({
    name: '',
    title: '',
    body: '',
    image_url: '',
    is_active: true,
  })
  const [deletingTemplate, setDeletingTemplate] = useState<ApiNotificationTemplate | null>(null)
  const [previewItem, setPreviewItem] = useState<{
    title: string
    body: string
    eventName: string
    imageUrl?: string | null
  } | null>(null)

  const [uploadingImage, setUploadingImage] = useState(false)
  const [testPushSubmitting, setTestPushSubmitting] = useState(false)
  const [broadcastSubmitting, setBroadcastSubmitting] = useState(false)
  const [confirmBroadcastItem, setConfirmBroadcastItem] = useState<{
    event_type: string
    title: string
    body: string
    image_url?: string
  } | null>(null)

  const [formSubmitting, setFormSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  // Load Data
  const loadData = useCallback(async () => {
    setLoading(true)
    setError(null)
    const token = getSessionPair()?.accessToken
    if (!token) {
      setLoading(false)
      return
    }

    try {
      const [typesData, tmplData] = await Promise.all([
        apiGetNotificationSystemTypes(token).catch(() => []),
        apiGetNotificationTemplates(token).catch(() => []),
      ])
      setSystemTypes(typesData)
      setTemplates(tmplData)

      // Also load logs
      const logsResp = await apiGetNotificationLogs(token, logsPage, logsPageSize, logsEventType || undefined).catch(() => ({
        data: [],
        pagination: { page: 1, page_size: 15, total: 0, total_pages: 1 },
      }))
      setLogs(logsResp.data || [])
      setLogsTotal(logsResp.pagination?.total || 0)
    } catch (err: any) {
      setError(err?.message || 'Gagal memuat data notifikasi')
    } finally {
      setLoading(false)
    }
  }, [logsPage, logsPageSize, logsEventType])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Clear toast after 4s
  useEffect(() => {
    if (successMsg) {
      const timer = setTimeout(() => setSuccessMsg(null), 4000)
      return () => clearTimeout(timer)
    }
  }, [successMsg])

  // Open Create Modal
  const handleOpenCreate = (presetEventType?: string) => {
    setFormError(null)
    const ev = presetEventType || systemTypes[0]?.event_type || 'iuran_bill_created'
    const found = systemTypes.find((t) => t.event_type === ev)
    setCreateForm({
      name: found ? `Template ${found.name}` : '',
      event_type: ev,
      title: found?.default_title || '',
      body: found?.default_body || '',
      image_url: '',
    })
    setShowCreateModal(true)
  }

  // Handle Create Submit
  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    const token = getSessionPair()?.accessToken
    if (!token) return

    if (!createForm.name.trim()) {
      setFormError('Nama template wajib diisi')
      return
    }
    if (!createForm.title.trim()) {
      setFormError('Judul notifikasi wajib diisi')
      return
    }
    if (!createForm.body.trim()) {
      setFormError('Isi pesan notifikasi wajib diisi')
      return
    }

    setFormSubmitting(true)
    try {
      const created = await apiCreateNotificationTemplate(token, {
        name: createForm.name.trim(),
        event_type: createForm.event_type,
        title: createForm.title.trim(),
        body: createForm.body.trim(),
        channel: 'push',
        image_url: createForm.image_url ? createForm.image_url.trim() : undefined,
      })
      setTemplates((prev) => [created, ...prev])
      setShowCreateModal(false)
      setSuccessMsg(`Template "${created.name}" berhasil dibuat`)
    } catch (err: any) {
      setFormError(err?.message || 'Gagal membuat template notifikasi')
    } finally {
      setFormSubmitting(false)
    }
  }

  // Open Edit Modal
  const handleOpenEdit = (t: ApiNotificationTemplate) => {
    setFormError(null)
    setEditingTemplate(t)
    setEditForm({
      name: t.name,
      title: t.title,
      body: t.body,
      image_url: t.image_url || '',
      is_active: t.is_active,
    })
  }

  // Handle Edit Submit
  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingTemplate) return
    setFormError(null)
    const token = getSessionPair()?.accessToken
    if (!token) return

    if (!editForm.name.trim()) {
      setFormError('Nama template wajib diisi')
      return
    }
    if (!editForm.title.trim()) {
      setFormError('Judul notifikasi wajib diisi')
      return
    }
    if (!editForm.body.trim()) {
      setFormError('Isi pesan notifikasi wajib diisi')
      return
    }

    setFormSubmitting(true)
    try {
      const updated = await apiUpdateNotificationTemplate(token, editingTemplate.id, {
        name: editForm.name.trim(),
        title: editForm.title.trim(),
        body: editForm.body.trim(),
        image_url: editForm.image_url.trim() || null,
        is_active: editForm.is_active,
      })
      setTemplates((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
      setEditingTemplate(null)
      setSuccessMsg(`Template "${updated.name}" berhasil diperbarui`)
    } catch (err: any) {
      setFormError(err?.message || 'Gagal memperbarui template notifikasi')
    } finally {
      setFormSubmitting(false)
    }
  }

  // Handle Image File Upload to RustFS
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>, targetForm: 'create' | 'edit') => {
    const file = e.target.files?.[0]
    if (!file) return
    const token = getSessionPair()?.accessToken
    if (!token) return

    const eventType = targetForm === 'create' ? createForm.event_type : editingTemplate?.event_type
    if (!eventType) return

    setUploadingImage(true)
    setFormError(null)
    try {
      const resp = await apiUploadNotificationImage(token, eventType, file)
      if (targetForm === 'create') {
        setCreateForm((prev) => ({ ...prev, image_url: resp.image_url }))
      } else {
        setEditForm((prev) => ({ ...prev, image_url: resp.image_url }))
      }
    } catch (err: any) {
      setFormError(err?.message || 'Gagal mengunggah gambar')
    } finally {
      setUploadingImage(false)
      e.target.value = ''
    }
  }

  // Handle Test Push (Super Admin Only)
  const handleTestPush = async (eventType: string, title: string, body: string, imageUrl?: string) => {
    const token = getSessionPair()?.accessToken
    if (!token) return
    setTestPushSubmitting(true)
    setError(null)
    setSuccessMsg(null)
    try {
      const resp = await apiTestPush(token, {
        event_type: eventType,
        title,
        body,
        image_url: imageUrl || undefined,
      })
      setSuccessMsg(`Uji notifikasi berhasil dikirim (${resp.dispatched_count} perangkat aktif Anda).`)
    } catch (err: any) {
      setError(err?.message || 'Gagal mengirim uji notifikasi')
    } finally {
      setTestPushSubmitting(false)
    }
  }

  // Handle Broadcast Push to all RT residents
  const handleBroadcastPush = async () => {
    if (!confirmBroadcastItem) return
    const token = getSessionPair()?.accessToken
    if (!token) return
    setBroadcastSubmitting(true)
    setError(null)
    setSuccessMsg(null)
    try {
      await apiTriggerManualPush(token, {
        title: confirmBroadcastItem.title,
        body: confirmBroadcastItem.body,
        image_url: confirmBroadcastItem.image_url || undefined,
      })
      setSuccessMsg('Notifikasi berhasil disiarkan ke seluruh warga RT!')
      setConfirmBroadcastItem(null)
      loadData()
    } catch (err: any) {
      setError(err?.message || 'Gagal menyiarkan notifikasi')
    } finally {
      setBroadcastSubmitting(false)
    }
  }

  // Handle Toggle Active
  const handleToggleActive = async (t: ApiNotificationTemplate) => {
    const token = getSessionPair()?.accessToken
    if (!token) return

    try {
      const updated = await apiToggleNotificationTemplateActive(token, t.id)
      setTemplates((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
      setSuccessMsg(`Status template "${updated.name}" diubah menjadi ${updated.is_active ? 'Aktif' : 'Nonaktif'}`)
    } catch (err: any) {
      setError(err?.message || 'Gagal mengubah status template')
    }
  }

  // Handle Delete Submit
  const handleDeleteSubmit = async () => {
    if (!deletingTemplate) return
    const token = getSessionPair()?.accessToken
    if (!token) return

    setFormSubmitting(true)
    try {
      await apiDeleteNotificationTemplate(token, deletingTemplate.id)
      setTemplates((prev) => prev.filter((item) => item.id !== deletingTemplate.id))
      setSuccessMsg(`Template "${deletingTemplate.name}" berhasil dihapus`)
      setDeletingTemplate(null)
    } catch (err: any) {
      setError(err?.message || 'Gagal menghapus template')
    } finally {
      setFormSubmitting(false)
    }
  }

  // Active variables for create form
  const createAllowedVars = useMemo(() => {
    const found = systemTypes.find((t) => t.event_type === createForm.event_type)
    return found?.allowed_variables || []
  }, [systemTypes, createForm.event_type])

  // Active variables for edit form
  const editAllowedVars = useMemo(() => {
    if (!editingTemplate) return []
    const found = systemTypes.find((t) => t.event_type === editingTemplate.event_type)
    return found?.allowed_variables || []
  }, [systemTypes, editingTemplate])

  return (
    <div style={{ padding: '1px 1px' }}>
      {/* Header */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <BellOutlined style={{ fontSize: 16 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Kelola Notifikasi</div>
            <div className="sf-page-header-subtitle">
              Pusat Pengelolaan Notifikasi Sistem, Template Pesan, dan Riwayat Pengiriman RT
            </div>
          </div>
        </div>
        <div className="sf-page-header-actions">
          <button
            type="button"
            className="sf-btn-outline"
            onClick={loadData}
            title="Muat ulang data"
          >
            <ReloadOutlined style={{ marginRight: 6 }} /> Segarkan
          </button>
          <button
            type="button"
            className="sf-btn-primary"
            onClick={() => handleOpenCreate()}
          >
            <PlusOutlined style={{ marginRight: 6 }} /> Tambah Template
          </button>
        </div>
      </div>

      {/* Alert Banners */}
      {error && (
        <div style={{ padding: 12, background: 'rgba(242, 73, 92, 0.1)', border: '1px solid var(--sf-danger)', borderRadius: 6, color: 'var(--sf-danger)', marginBottom: 16 }}>
          {error}
        </div>
      )}
      {successMsg && (
        <div style={{ padding: 12, background: 'rgba(115, 191, 105, 0.12)', border: '1px solid var(--sf-success)', borderRadius: 6, color: 'var(--sf-success)', marginBottom: 16 }}>
          {successMsg}
        </div>
      )}

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 0, borderBottom: '1px solid var(--sf-border)', marginBottom: 16 }}>
        <button
          type="button"
          onClick={() => setActiveTab('system')}
          style={{
            padding: '10px 16px',
            border: 'none',
            background: 'none',
            fontWeight: activeTab === 'system' ? 600 : 400,
            fontSize: 14,
            cursor: 'pointer',
            borderBottom: activeTab === 'system' ? '2px solid var(--sf-accent)' : '2px solid transparent',
            color: activeTab === 'system' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
            display: 'flex',
            alignItems: 'center',
            gap: 6,
          }}
        >
          <AppstoreOutlined /> Notifikasi Sistem ({systemTypes.length})
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('templates')}
          style={{
            padding: '10px 16px',
            border: 'none',
            background: 'none',
            fontWeight: activeTab === 'templates' ? 600 : 400,
            fontSize: 14,
            cursor: 'pointer',
            borderBottom: activeTab === 'templates' ? '2px solid var(--sf-accent)' : '2px solid transparent',
            color: activeTab === 'templates' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
            display: 'flex',
            alignItems: 'center',
            gap: 6,
          }}
        >
          <BellOutlined /> Template Kustom ({templates.length})
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('logs')}
          style={{
            padding: '10px 16px',
            border: 'none',
            background: 'none',
            fontWeight: activeTab === 'logs' ? 600 : 400,
            fontSize: 14,
            cursor: 'pointer',
            borderBottom: activeTab === 'logs' ? '2px solid var(--sf-accent)' : '2px solid transparent',
            color: activeTab === 'logs' ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
            display: 'flex',
            alignItems: 'center',
            gap: 6,
          }}
        >
          <HistoryOutlined /> Riwayat Pengiriman ({logsTotal})
        </button>
      </div>

      {loading ? (
        <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--sf-text-muted)' }}>
          Memuat data notifikasi...
        </div>
      ) : (
        <>
          {/* TAB 1: NOTIFIKASI SISTEM */}
          {activeTab === 'system' && (
            <div>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 16 }}>
                {systemTypes.map((st) => (
                  <div
                    key={st.event_type}
                    style={{
                      background: 'var(--sf-card-bg)',
                      border: '1px solid var(--sf-border)',
                      borderRadius: 'var(--sf-radius-md)',
                      padding: 16,
                      display: 'flex',
                      flexDirection: 'column',
                      justifyContent: 'space-between',
                    }}
                  >
                    <div>
                      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 8, marginBottom: 8 }}>
                        <div style={{ fontWeight: 600, fontSize: 16, color: 'var(--sf-text)' }}>
                          {st.name}
                        </div>
                        <Badge variant="blue">FCM PUSH</Badge>
                      </div>

                      <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginBottom: 12 }}>
                        {st.trigger}
                      </div>

                      <div style={{ fontSize: 12, background: 'rgba(0,0,0,0.03)', padding: 10, borderRadius: 6, marginBottom: 12 }}>
                        <div style={{ fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 2 }}>Penerima:</div>
                        <div style={{ color: 'var(--sf-text)' }}>{st.recipients_rule}</div>
                      </div>

                      <div style={{ fontSize: 12, borderLeft: '3px solid var(--sf-accent)', paddingLeft: 8, marginBottom: 12 }}>
                        <div style={{ fontWeight: 600, color: 'var(--sf-text)' }}>{st.default_title}</div>
                        <div style={{ color: 'var(--sf-text-muted)' }}>{st.default_body}</div>
                      </div>

                      {st.allowed_variables.length > 0 && (
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginBottom: 12 }}>
                          {st.allowed_variables.map((v) => (
                            <span key={v} style={{ fontSize: 11, background: 'var(--sf-tag-bg, rgba(255,255,255,0.08))', padding: '2px 6px', borderRadius: 4, fontFamily: 'monospace' }}>
                              {'{{' + v + '}}'}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 8 }}>
                      <div style={{ display: 'flex', gap: 8 }}>
                        <button
                          type="button"
                          className="sf-btn-outline"
                          style={{ flex: 1 }}
                          onClick={() => setPreviewItem({ title: st.default_title, body: st.default_body, eventName: st.name })}
                        >
                          Preview Banner
                        </button>
                        <button
                          type="button"
                          className="sf-btn-primary"
                          style={{ flex: 1 }}
                          onClick={() => handleOpenCreate(st.event_type)}
                        >
                          Kustomisasi
                        </button>
                      </div>
                      {(st.event_type === 'waste_truck_arrival' || st.event_type === 'manual_push') && (
                        <div style={{ display: 'flex', gap: 8, marginTop: 4 }}>
                          {isSuperAdmin && (
                            <button
                              type="button"
                              className="sf-btn-outline"
                              style={{ flex: 1, fontSize: 12, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 4 }}
                              onClick={() => handleTestPush(st.event_type, st.default_title, st.default_body)}
                              disabled={testPushSubmitting}
                              title="Uji kirim hanya ke perangkat Super Admin"
                            >
                              <ExperimentOutlined /> {testPushSubmitting ? 'Menguji...' : 'Test Notifikasi'}
                            </button>
                          )}
                          <button
                            type="button"
                            className="sf-btn-primary"
                            style={{ flex: 1, fontSize: 12, background: 'var(--sf-success)', borderColor: 'var(--sf-success)', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 4 }}
                            onClick={() => setConfirmBroadcastItem({ event_type: st.event_type, title: st.default_title, body: st.default_body })}
                            disabled={broadcastSubmitting}
                            title="Kirim notifikasi siaran ke seluruh warga RT"
                          >
                            <SendOutlined /> Kirim Notifikasi
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* TAB 2: TEMPLATE KUSTOM */}
          {activeTab === 'templates' && (
            <div>
              {templates.length === 0 ? (
                <div style={{ textAlign: 'center', padding: '60px 20px', background: 'var(--sf-card-bg)', border: '1px dashed var(--sf-border)', borderRadius: 8 }}>
                  <BellOutlined style={{ fontSize: 36, color: 'var(--sf-text-muted)', marginBottom: 12 }} />
                  <div style={{ fontSize: 16, fontWeight: 600, color: 'var(--sf-text)', marginBottom: 4 }}>
                    Belum Ada Template Kustom
                  </div>
                  <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', marginBottom: 16, maxWidth: 440, margin: '0 auto 16px' }}>
                    Sistem saat ini menggunakan pesan default standar Urena. Anda dapat menambahkan template kustom untuk mengubah judul dan isi pesan notifikasi push.
                  </div>
                  <button
                    type="button"
                    className="sf-btn-primary"
                    onClick={() => handleOpenCreate()}
                  >
                    <PlusOutlined style={{ marginRight: 6 }} /> Buat Template Pertama
                  </button>
                </div>
              ) : (
                <div style={{ overflowX: 'auto', background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8 }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--sf-border)', background: 'rgba(0,0,0,0.02)' }}>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Nama Template</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Jenis Notifikasi</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Judul & Pesan</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Status</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, textAlign: 'right' }}>Aksi</th>
                      </tr>
                    </thead>
                    <tbody>
                      {templates.map((t) => (
                        <tr key={t.id} style={{ borderBottom: '1px solid var(--sf-border)' }}>
                          <td style={{ padding: '14px 16px', fontWeight: 600, color: 'var(--sf-text)' }}>
                            {t.name}
                          </td>
                          <td style={{ padding: '14px 16px' }}>
                            <Badge variant="blue">
                              {systemTypes.find((st) => st.event_type === t.event_type)?.name || t.event_type}
                            </Badge>
                          </td>
                          <td style={{ padding: '14px 16px', maxWidth: 360 }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 2 }}>
                              <span style={{ fontWeight: 600, color: 'var(--sf-text)' }}>{t.title}</span>
                              {t.image_url && <PictureOutlined style={{ color: 'var(--sf-accent)', fontSize: 13 }} title="Memiliki lampiran gambar" />}
                            </div>
                            <div style={{ color: 'var(--sf-text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                              {t.body}
                            </div>
                          </td>
                          <td style={{ padding: '14px 16px' }}>
                            {t.is_active ? (
                              <Badge variant="green">Aktif</Badge>
                            ) : (
                              <Badge variant="gray">Tidak Aktif</Badge>
                            )}
                          </td>
                          <td style={{ padding: '14px 16px', textAlign: 'right' }}>
                            <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                              <button
                                type="button"
                                className="sf-btn-outline"
                                style={{ padding: '4px 8px', fontSize: 12 }}
                                onClick={() => setPreviewItem({ title: t.title, body: t.body, eventName: t.name, imageUrl: t.image_url })}
                                title="Lihat Tampilan Banner"
                              >
                                <MobileOutlined />
                              </button>
                              {(t.event_type === 'waste_truck_arrival' || t.event_type === 'manual_push') && isSuperAdmin && (
                                <button
                                  type="button"
                                  className="sf-btn-outline"
                                  style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-accent)' }}
                                  onClick={() => handleTestPush(t.event_type, t.title, t.body, t.image_url || undefined)}
                                  disabled={testPushSubmitting}
                                  title="Test Notifikasi (Hanya Super Admin)"
                                >
                                  <ExperimentOutlined />
                                </button>
                              )}
                              {(t.event_type === 'waste_truck_arrival' || t.event_type === 'manual_push') && (
                                <button
                                  type="button"
                                  className="sf-btn-outline"
                                  style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-success)' }}
                                  onClick={() => setConfirmBroadcastItem({ event_type: t.event_type, title: t.title, body: t.body, image_url: t.image_url || undefined })}
                                  disabled={broadcastSubmitting}
                                  title="Kirim Notifikasi Siaran ke Warga"
                                >
                                  <SendOutlined />
                                </button>
                              )}
                              <button
                                type="button"
                                className="sf-btn-outline"
                                style={{ padding: '4px 8px', fontSize: 12 }}
                                onClick={() => handleToggleActive(t)}
                                title={t.is_active ? 'Nonaktifkan template' : 'Aktifkan template'}
                              >
                                {t.is_active ? <StopOutlined style={{ color: 'var(--sf-warning)' }} /> : <CheckCircleOutlined style={{ color: 'var(--sf-success)' }} />}
                              </button>
                              <button
                                type="button"
                                className="sf-btn-outline"
                                style={{ padding: '4px 8px', fontSize: 12 }}
                                onClick={() => handleOpenEdit(t)}
                                title="Edit template"
                              >
                                <EditOutlined />
                              </button>
                              <button
                                type="button"
                                className="sf-btn-outline"
                                style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-danger)' }}
                                onClick={() => setDeletingTemplate(t)}
                                title="Hapus template"
                              >
                                <DeleteOutlined />
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {/* TAB 3: RIWAYAT PENGIRIMAN */}
          {activeTab === 'logs' && (
            <div>
              {/* Filter bar */}
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <span style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Filter Jenis:</span>
                  <select
                    value={logsEventType}
                    onChange={(e) => {
                      setLogsEventType(e.target.value)
                      setLogsPage(1)
                    }}
                    style={{
                      padding: '6px 12px',
                      borderRadius: 6,
                      border: '1px solid var(--sf-border)',
                      background: 'var(--sf-card-bg)',
                      color: 'var(--sf-text)',
                      fontSize: 13,
                    }}
                  >
                    <option value="">Semua Jenis Notifikasi</option>
                    {systemTypes.map((st) => (
                      <option key={st.event_type} value={st.event_type}>
                        {st.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                  Total riwayat tersimpan: {logsTotal}
                </div>
              </div>

              {logs.length === 0 ? (
                <div style={{ textAlign: 'center', padding: '60px 20px', background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8 }}>
                  <HistoryOutlined style={{ fontSize: 36, color: 'var(--sf-text-muted)', marginBottom: 12 }} />
                  <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-text)', marginBottom: 4 }}>
                    Belum Ada Riwayat Notifikasi
                  </div>
                  <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
                    Riwayat akan tercatat secara otomatis setiap kali sistem mengirimkan notifikasi ke perangkat warga.
                  </div>
                </div>
              ) : (
                <div style={{ overflowX: 'auto', background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8 }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--sf-border)', background: 'rgba(0,0,0,0.02)' }}>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Waktu Kirim</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Event</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Judul</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Isi Pesan</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600 }}>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {logs.map((l) => (
                        <tr key={l.id} style={{ borderBottom: '1px solid var(--sf-border)' }}>
                          <td style={{ padding: '12px 16px', whiteSpace: 'nowrap', color: 'var(--sf-text-muted)', fontSize: 12 }}>
                            {new Date(l.created_at).toLocaleString('id-ID', {
                              dateStyle: 'medium',
                              timeStyle: 'short',
                            })}
                          </td>
                          <td style={{ padding: '12px 16px' }}>
                            <Badge variant="gray">
                              {systemTypes.find((st) => st.event_type === l.event_type)?.name || l.event_type}
                            </Badge>
                          </td>
                          <td style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--sf-text)' }}>
                            {l.title}
                          </td>
                          <td style={{ padding: '12px 16px', maxWidth: 380, color: 'var(--sf-text-muted)' }}>
                            {l.body}
                          </td>
                          <td style={{ padding: '12px 16px' }}>
                            <Badge variant="green">TERKIRIM</Badge>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </>
      )}

      {/* MODAL: Tambah Template */}
      {showCreateModal && (
        <Modal open={showCreateModal} onClose={() => setShowCreateModal(false)}>
          <ModalHeader title="Tambah Template Notifikasi" onClose={() => setShowCreateModal(false)} />
          <form onSubmit={handleCreateSubmit}>
            <ModalBody>
              {formError && (
                <div style={{ padding: 10, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 6, marginBottom: 14, fontSize: 13 }}>
                  {formError}
                </div>
              )}

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                  Jenis Event Notifikasi
                </label>
                <select
                  value={createForm.event_type}
                  onChange={(e) => {
                    const ev = e.target.value
                    const found = systemTypes.find((t) => t.event_type === ev)
                    setCreateForm((prev) => ({
                      ...prev,
                      event_type: ev,
                      title: found?.default_title || prev.title,
                      body: found?.default_body || prev.body,
                    }))
                  }}
                  style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-bg)', color: 'var(--sf-text)' }}
                >
                  {systemTypes.map((st) => (
                    <option key={st.event_type} value={st.event_type}>
                      {st.name} ({st.event_type})
                    </option>
                  ))}
                </select>
              </div>

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                  Nama Template
                </label>
                <input
                  type="text"
                  value={createForm.name}
                  onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                  placeholder="Misal: Tagihan Bulanan Resmi"
                  style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-bg)', color: 'var(--sf-text)' }}
                />
              </div>

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                  Judul Notifikasi
                </label>
                <input
                  type="text"
                  value={createForm.title}
                  onChange={(e) => setCreateForm({ ...createForm, title: e.target.value })}
                  placeholder="Judul banner push notification"
                  style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-bg)', color: 'var(--sf-text)' }}
                />
              </div>

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                  Isi Pesan Notifikasi
                </label>
                <textarea
                  rows={3}
                  value={createForm.body}
                  onChange={(e) => setCreateForm({ ...createForm, body: e.target.value })}
                  placeholder="Isi pesan notifikasi"
                  style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-bg)', color: 'var(--sf-text)' }}
                />
              </div>

              {(createForm.event_type === 'waste_truck_arrival' || createForm.event_type === 'manual_push') ? (
                <div style={{ marginBottom: 14 }}>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                    Gambar Notifikasi (Opsional - JPEG, PNG, WebP)
                  </label>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 6 }}>
                    <input
                      type="file"
                      accept="image/jpeg,image/png,image/webp"
                      onChange={(e) => handleFileUpload(e, 'create')}
                      disabled={uploadingImage}
                      style={{ fontSize: 12 }}
                    />
                    {uploadingImage && <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>Mengunggah...</span>}
                  </div>
                  {createForm.image_url && (
                    <div style={{ position: 'relative', display: 'inline-block', marginTop: 4 }}>
                      <img
                        src={createForm.image_url}
                        alt="Preview Lampiran"
                        style={{ maxHeight: 90, borderRadius: 6, border: '1px solid var(--sf-border)' }}
                      />
                      <button
                        type="button"
                        onClick={() => setCreateForm((prev) => ({ ...prev, image_url: '' }))}
                        style={{
                          position: 'absolute',
                          top: -6,
                          right: -6,
                          background: 'var(--sf-danger)',
                          color: '#fff',
                          border: 'none',
                          borderRadius: '50%',
                          width: 20,
                          height: 20,
                          cursor: 'pointer',
                          fontSize: 10,
                          lineHeight: '20px',
                          textAlign: 'center',
                        }}
                        title="Hapus gambar"
                      >
                        ×
                      </button>
                    </div>
                  )}
                </div>
              ) : (
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 14, fontStyle: 'italic' }}>
                  * Notifikasi keuangan tidak mendukung penyertaan gambar.
                </div>
              )}

              {createAllowedVars.length > 0 && (
                <div style={{ background: 'rgba(0,0,0,0.03)', padding: 10, borderRadius: 6, marginBottom: 14 }}>
                  <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 4 }}>
                    Variabel Dinamis yang Diizinkan (klik untuk menyisipkan ke pesan):
                  </div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {createAllowedVars.map((v) => (
                      <button
                        key={v}
                        type="button"
                        onClick={() => setCreateForm({ ...createForm, body: createForm.body + ` {{${v}}}` })}
                        style={{ background: 'var(--sf-tag-bg, #3884ff)', color: '#fff', border: 'none', borderRadius: 4, padding: '3px 8px', fontSize: 12, cursor: 'pointer' }}
                      >
                        {`{{${v}}}`}
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </ModalBody>
            <ModalFooter>
              <button
                type="button"
                className="sf-btn-outline"
                onClick={() => setShowCreateModal(false)}
                disabled={formSubmitting}
              >
                Batal
              </button>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={formSubmitting}
              >
                {formSubmitting ? 'Menyimpan...' : 'Simpan Template'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* MODAL: Edit Template */}
      {editingTemplate && (
        <Modal open={!!editingTemplate} onClose={() => setEditingTemplate(null)}>
          <ModalHeader title={`Edit Template: ${editingTemplate.name}`} onClose={() => setEditingTemplate(null)} />
          <form onSubmit={handleEditSubmit}>
            <ModalBody>
              {formError && (
                <div style={{ padding: 10, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 6, marginBottom: 14, fontSize: 13 }}>
                  {formError}
                </div>
              )}

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                  Nama Template
                </label>
                <input
                  type="text"
                  value={editForm.name}
                  onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                  style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-bg)', color: 'var(--sf-text)' }}
                />
              </div>

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                  Judul Notifikasi
                </label>
                <input
                  type="text"
                  value={editForm.title}
                  onChange={(e) => setEditForm({ ...editForm, title: e.target.value })}
                  style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-bg)', color: 'var(--sf-text)' }}
                />
              </div>

              <div style={{ marginBottom: 14 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                  Isi Pesan Notifikasi
                </label>
                <textarea
                  rows={3}
                  value={editForm.body}
                  onChange={(e) => setEditForm({ ...editForm, body: e.target.value })}
                  style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-bg)', color: 'var(--sf-text)' }}
                />
              </div>

              {(editingTemplate.event_type === 'waste_truck_arrival' || editingTemplate.event_type === 'manual_push') ? (
                <div style={{ marginBottom: 14 }}>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                    Gambar Notifikasi (Opsional - JPEG, PNG, WebP)
                  </label>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 6 }}>
                    <input
                      type="file"
                      accept="image/jpeg,image/png,image/webp"
                      onChange={(e) => handleFileUpload(e, 'edit')}
                      disabled={uploadingImage}
                      style={{ fontSize: 12 }}
                    />
                    {uploadingImage && <span style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>Mengunggah...</span>}
                  </div>
                  {editForm.image_url && (
                    <div style={{ position: 'relative', display: 'inline-block', marginTop: 4 }}>
                      <img
                        src={editForm.image_url}
                        alt="Preview Lampiran"
                        style={{ maxHeight: 90, borderRadius: 6, border: '1px solid var(--sf-border)' }}
                      />
                      <button
                        type="button"
                        onClick={() => setEditForm((prev) => ({ ...prev, image_url: '' }))}
                        style={{
                          position: 'absolute',
                          top: -6,
                          right: -6,
                          background: 'var(--sf-danger)',
                          color: '#fff',
                          border: 'none',
                          borderRadius: '50%',
                          width: 20,
                          height: 20,
                          cursor: 'pointer',
                          fontSize: 10,
                          lineHeight: '20px',
                          textAlign: 'center',
                        }}
                        title="Hapus gambar"
                      >
                        ×
                      </button>
                    </div>
                  )}
                </div>
              ) : (
                <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 14, fontStyle: 'italic' }}>
                  * Notifikasi keuangan tidak mendukung penyertaan gambar.
                </div>
              )}

              <div style={{ marginBottom: 14, display: 'flex', alignItems: 'center', gap: 8 }}>
                <input
                  type="checkbox"
                  id="editIsActive"
                  checked={editForm.is_active}
                  onChange={(e) => setEditForm({ ...editForm, is_active: e.target.checked })}
                />
                <label htmlFor="editIsActive" style={{ fontSize: 13, fontWeight: 600, cursor: 'pointer' }}>
                  Gunakan template ini sebagai template aktif
                </label>
              </div>

              {editAllowedVars.length > 0 && (
                <div style={{ background: 'rgba(0,0,0,0.03)', padding: 10, borderRadius: 6, marginBottom: 14 }}>
                  <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--sf-text-muted)', marginBottom: 4 }}>
                    Variabel Dinamis yang Diizinkan:
                  </div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {editAllowedVars.map((v) => (
                      <button
                        key={v}
                        type="button"
                        onClick={() => setEditForm({ ...editForm, body: editForm.body + ` {{${v}}}` })}
                        style={{ background: 'var(--sf-tag-bg, #3884ff)', color: '#fff', border: 'none', borderRadius: 4, padding: '3px 8px', fontSize: 12, cursor: 'pointer' }}
                      >
                        {`{{${v}}}`}
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </ModalBody>
            <ModalFooter>
              <button
                type="button"
                className="sf-btn-outline"
                onClick={() => setEditingTemplate(null)}
                disabled={formSubmitting}
              >
                Batal
              </button>
              {(editingTemplate.event_type === 'waste_truck_arrival' || editingTemplate.event_type === 'manual_push') && isSuperAdmin && (
                <button
                  type="button"
                  className="sf-btn-outline"
                  onClick={() => handleTestPush(editingTemplate.event_type, editForm.title, editForm.body, editForm.image_url || undefined)}
                  disabled={testPushSubmitting || formSubmitting}
                  title="Kirim uji notifikasi ke perangkat Anda"
                >
                  <ExperimentOutlined /> {testPushSubmitting ? 'Menguji...' : 'Test Notifikasi'}
                </button>
              )}
              {(editingTemplate.event_type === 'waste_truck_arrival' || editingTemplate.event_type === 'manual_push') && (
                <button
                  type="button"
                  className="sf-btn-primary"
                  style={{ background: 'var(--sf-success)', borderColor: 'var(--sf-success)' }}
                  onClick={() => {
                    setConfirmBroadcastItem({
                      event_type: editingTemplate.event_type,
                      title: editForm.title,
                      body: editForm.body,
                      image_url: editForm.image_url || undefined,
                    })
                  }}
                  disabled={formSubmitting}
                >
                  <SendOutlined /> Kirim Notifikasi
                </button>
              )}
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={formSubmitting}
              >
                {formSubmitting ? 'Menyimpan...' : 'Simpan Template'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* MODAL: Hapus Template */}
      {deletingTemplate && (
        <Modal open={!!deletingTemplate} onClose={() => setDeletingTemplate(null)}>
          <ModalHeader title="Hapus Template" onClose={() => setDeletingTemplate(null)} />
          <ModalBody>
            <div style={{ fontSize: 14, color: 'var(--sf-text)', marginBottom: 12 }}>
              Apakah Anda yakin ingin menghapus template <strong>"{deletingTemplate.name}"</strong>?
            </div>
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
              Jika template dihapus, pengiriman notifikasi untuk event ini akan otomatis menggunakan pesan default bawaan sistem.
            </div>
          </ModalBody>
          <ModalFooter>
            <button
              type="button"
              className="sf-btn-outline"
              onClick={() => setDeletingTemplate(null)}
              disabled={formSubmitting}
            >
              Batal
            </button>
            <button
              type="button"
              className="sf-btn-primary"
              style={{ background: 'var(--sf-danger)', borderColor: 'var(--sf-danger)' }}
              onClick={handleDeleteSubmit}
              disabled={formSubmitting}
            >
              {formSubmitting ? 'Menghapus...' : 'Ya, Hapus'}
            </button>
          </ModalFooter>
        </Modal>
      )}

      {/* MODAL: Konfirmasi Kirim Notifikasi Siaran */}
      {confirmBroadcastItem && (
        <Modal open={!!confirmBroadcastItem} onClose={() => setConfirmBroadcastItem(null)}>
          <ModalHeader title="Konfirmasi Kirim Notifikasi Siaran" onClose={() => setConfirmBroadcastItem(null)} />
          <ModalBody>
            <div style={{ fontSize: 14, color: 'var(--sf-text)', marginBottom: 12 }}>
              Apakah Anda yakin ingin menyiarkan notifikasi ini ke <strong>SELURUH</strong> warga terdaftar di RT Anda?
            </div>
            <div style={{ background: 'var(--sf-card-bg)', border: '1px solid var(--sf-border)', borderRadius: 8, padding: 12, marginBottom: 12 }}>
              <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>{confirmBroadcastItem.title}</div>
              <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>{confirmBroadcastItem.body}</div>
              {confirmBroadcastItem.image_url && (
                <div style={{ marginTop: 8 }}>
                  <img src={confirmBroadcastItem.image_url} alt="Lampiran" style={{ maxHeight: 120, borderRadius: 6, objectFit: 'cover' }} />
                </div>
              )}
            </div>
            <div style={{ fontSize: 12, color: 'var(--sf-warning)' }}>
              Peringatan: Pesan ini akan langsung dikirimkan ke perangkat aktif warga tanpa dapat dibatalkan.
            </div>
          </ModalBody>
          <ModalFooter>
            <button
              type="button"
              className="sf-btn-outline"
              onClick={() => setConfirmBroadcastItem(null)}
              disabled={broadcastSubmitting}
            >
              Batal
            </button>
            <button
              type="button"
              className="sf-btn-primary"
              style={{ background: 'var(--sf-success)', borderColor: 'var(--sf-success)' }}
              onClick={handleBroadcastPush}
              disabled={broadcastSubmitting}
            >
              {broadcastSubmitting ? 'Mengirim...' : 'Ya, Kirim Sekarang'}
            </button>
          </ModalFooter>
        </Modal>
      )}

      {/* MODAL: Preview Banner Notification */}
      {previewItem && (
        <Modal open={!!previewItem} onClose={() => setPreviewItem(null)}>
          <ModalHeader title={`Preview Push Notification: ${previewItem.eventName}`} onClose={() => setPreviewItem(null)} />
          <ModalBody>
            <div style={{ textAlign: 'center', marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginBottom: 16 }}>
                Simulasi tampilan banner push notification di layar perangkat Android warga:
              </div>

              {/* Realistic Android Notification Card */}
              <div
                style={{
                  maxWidth: 380,
                  margin: '0 auto',
                  background: '#1E293B',
                  color: '#F8FAFC',
                  borderRadius: 16,
                  padding: 14,
                  boxShadow: '0 8px 24px rgba(0,0,0,0.3)',
                  border: '1px solid rgba(255,255,255,0.1)',
                  textAlign: 'left',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <div style={{ width: 18, height: 18, borderRadius: 4, background: '#3884ff', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <BellOutlined style={{ fontSize: 10, color: '#fff' }} />
                    </div>
                    <span style={{ fontSize: 11, fontWeight: 600, color: '#94A3B8', letterSpacing: 0.5 }}>
                      SOCIAL FINANCE URENA
                    </span>
                  </div>
                  <span style={{ fontSize: 11, color: '#64748B' }}>baru saja</span>
                </div>

                <div style={{ fontWeight: 700, fontSize: 14, color: '#FFFFFF', marginBottom: 4 }}>
                  {previewItem.title}
                </div>
                <div style={{ fontSize: 13, color: '#CBD5E1', lineHeight: 1.4 }}>
                  {previewItem.body}
                </div>
                {previewItem.imageUrl && (
                  <div style={{ marginTop: 10 }}>
                    <img
                      src={previewItem.imageUrl}
                      alt="Banner Notifikasi"
                      style={{ width: '100%', maxHeight: 180, objectFit: 'cover', borderRadius: 8 }}
                    />
                  </div>
                )}
              </div>
            </div>
          </ModalBody>
          <ModalFooter>
            <button
              type="button"
              className="sf-btn-primary"
              onClick={() => setPreviewItem(null)}
            >
              Tutup Preview
            </button>
          </ModalFooter>
        </Modal>
      )}
    </div>
  )
}
