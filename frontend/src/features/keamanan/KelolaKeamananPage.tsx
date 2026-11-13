import { useCallback, useEffect, useState } from 'react'
import dayjs from 'dayjs'
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  DatePicker,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  TimePicker,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  CopyOutlined,
  DownloadOutlined,
  EditOutlined,
  KeyOutlined,
  PlusOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  StopOutlined,
  SwapOutlined,
} from '@ant-design/icons'
import { hasPermission, isKetuaRTUser, isSuperAdminUser, useAuth } from '@/app/AuthContext'
import { ResidentAvatar } from '@/components/ResidentAvatar'
import { ResidentPhotoManage } from '@/components/ResidentPhotoManage'
import {
  apiCancelSecuritySchedule,
  apiCreateSecurityCorrection,
  apiCreateSecurityDevice,
  apiCreateSecuritySchedule,
  apiCreateSecurityShift,
  apiDecideSecurityCorrection,
  apiDecideSecuritySwap,
  apiExportSecurityAttendanceCSV,
  apiGetSecurityDashboard,
  apiGetSecuritySchedule,
  apiListSecurityAttendance,
  apiListSecurityCorrections,
  apiListSecurityDevices,
  apiListSecurityGuestBooks,
  apiListSecurityOfficers,
  apiListSecuritySchedules,
  apiListSecurityShifts,
  apiListSecuritySwaps,
  apiRevokeSecurityDevice,
  apiRotateDevicePairing,
  apiUpdateSecurityShift,
  apiUpdateSecuritySchedule,
  type AttendanceCorrection,
  type AttendanceRecord,
  type DeviceProvisionResult,
  type GuestBookEntry,
  type SecurityDashboardSummary,
  type SecurityDevice,
  type SecurityOfficer,
  type SecuritySchedule,
  type ShiftConfig,
  type ShiftSwap,
} from './securityApi'

const { Title, Text } = Typography

const ATTENDANCE_STATUS_TAG: Record<string, { color: string; label: string }> = {
  belum_hadir: { color: 'default', label: 'Belum Hadir' },
  hadir: { color: 'processing', label: 'Hadir' },
  terlambat: { color: 'warning', label: 'Terlambat' },
  sudah_absen_pulang: { color: 'success', label: 'Sudah Absen Pulang' },
  tidak_hadir: { color: 'error', label: 'Tidak Hadir' },
  belum_absen_pulang: { color: 'orange', label: 'Belum Absen Pulang' },
}

const SWAP_STATUS_TAG: Record<string, { color: string; label: string }> = {
  pending_target: { color: 'blue', label: 'Menunggu Petugas Pengganti' },
  pending_ketua: { color: 'gold', label: 'Menunggu Persetujuan Ketua RT' },
  approved: { color: 'green', label: 'Disetujui' },
  rejected: { color: 'red', label: 'Ditolak' },
  cancelled: { color: 'default', label: 'Dibatalkan' },
}

const CORRECTION_STATUS_TAG: Record<string, { color: string; label: string }> = {
  pending: { color: 'gold', label: 'Menunggu Persetujuan' },
  approved: { color: 'green', label: 'Disetujui' },
  rejected: { color: 'red', label: 'Ditolak' },
}

function formatDateTime(iso?: string | null): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Asia/Jakarta',
  })
}

function formatTimeOnly(iso?: string | null): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleTimeString('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Asia/Jakarta',
  })
}

function getErrMsg(err: unknown, fallback: string): string {
  if (err instanceof Error && err.message) return err.message
  return fallback
}

export function KelolaKeamananPage() {
  const { user } = useAuth()
  const canManage =
    isSuperAdminUser(user) ||
    isKetuaRTUser(user) ||
    hasPermission(user, 'security.schedule.write') ||
    hasPermission(user, 'security.device.manage')

  const [activeTab, setActiveTab] = useState('dashboard')
  const [msgApi, contextHolder] = message.useMessage()

  // Core data states
  const [dashboard, setDashboard] = useState<SecurityDashboardSummary | null>(null)
  const [officers, setOfficers] = useState<SecurityOfficer[]>([])
  const [shifts, setShifts] = useState<ShiftConfig[]>([])
  const [schedules, setSchedules] = useState<SecuritySchedule[]>([])
  const [scheduleTotal, setScheduleTotal] = useState(0)
  const [schedulePage, setSchedulePage] = useState(1)
  const [swaps, setSwaps] = useState<ShiftSwap[]>([])
  const [attendance, setAttendance] = useState<AttendanceRecord[]>([])
  const [attendanceTotal, setAttendanceTotal] = useState(0)
  const [attendancePage, setAttendancePage] = useState(1)
  const [corrections, setCorrections] = useState<AttendanceCorrection[]>([])
  const [devices, setDevices] = useState<SecurityDevice[]>([])
  const [guestBooks, setGuestBooks] = useState<GuestBookEntry[]>([])
  const [guestTotal, setGuestTotal] = useState(0)
  const [guestPage, setGuestPage] = useState(1)

  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)

  // Filters
  const [scheduleFilter, setScheduleFilter] = useState<{
    date_from?: string
    date_to?: string
    user_id?: string
    shift_config_id?: string
  }>({})
  const [swapFilterStatus, setSwapFilterStatus] = useState<string | undefined>(undefined)
  const [attendanceFilter, setAttendanceFilter] = useState<{
    date_from?: string
    date_to?: string
    user_id?: string
    shift_config_id?: string
    status?: string
  }>({})
  const [correctionFilterStatus, setCorrectionFilterStatus] = useState<string | undefined>(undefined)
  const [guestFilter, setGuestFilter] = useState<{
    status?: string
    date_from?: string
    date_to?: string
    search?: string
  }>({})

  // Modals
  const [shiftModalOpen, setShiftModalOpen] = useState(false)
  const [editingShift, setEditingShift] = useState<ShiftConfig | null>(null)
  const [shiftForm] = Form.useForm()

  const [scheduleModalOpen, setScheduleModalOpen] = useState(false)
  const [editingSchedule, setEditingSchedule] = useState<SecuritySchedule | null>(null)
  const [scheduleDetail, setScheduleDetail] = useState<SecuritySchedule[] | null>(null)
  const [scheduleForm] = Form.useForm()

  const [deviceModalOpen, setDeviceModalOpen] = useState(false)
  const [deviceForm] = Form.useForm()
  const [provisionResult, setProvisionResult] = useState<DeviceProvisionResult | null>(null)

  const [correctionModalRecord, setCorrectionModalRecord] = useState<AttendanceRecord | null>(null)
  const [correctionForm] = Form.useForm()

  const [decisionModal, setDecisionModal] = useState<{
    kind: 'swap' | 'correction'
    id: string
    approve: boolean
    title: string
  } | null>(null)
  const [decisionNote, setDecisionNote] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const loadBaseLists = useCallback(async () => {
    const [off, sh] = await Promise.all([apiListSecurityOfficers(), apiListSecurityShifts(false)])
    setOfficers(off)
    setShifts(sh)
  }, [])

  const loadDashboard = useCallback(async () => {
    setLoading(true)
    try {
      const [dash] = await Promise.all([apiGetSecurityDashboard(), loadBaseLists()])
      setDashboard(dash)
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal memuat dashboard keamanan'))
    } finally {
      setLoading(false)
    }
  }, [loadBaseLists, msgApi])

  const loadSchedules = useCallback(
    async (page = schedulePage) => {
      setLoading(true)
      try {
        const res = await apiListSecuritySchedules({
          ...scheduleFilter,
          page,
          page_size: 20,
        })
        setSchedules(res.data ?? [])
        setScheduleTotal(res.pagination?.total ?? 0)
      } catch (err) {
        msgApi.error(getErrMsg(err, 'Gagal memuat jadwal petugas'))
      } finally {
        setLoading(false)
      }
    },
    [msgApi, scheduleFilter, schedulePage],
  )

  const loadSwaps = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiListSecuritySwaps(swapFilterStatus)
      setSwaps(data)
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal memuat daftar pertukaran shift'))
    } finally {
      setLoading(false)
    }
  }, [msgApi, swapFilterStatus])

  const loadAttendance = useCallback(
    async (page = attendancePage) => {
      setLoading(true)
      try {
        const res = await apiListSecurityAttendance({
          ...attendanceFilter,
          page,
          page_size: 20,
        })
        setAttendance(res.data ?? [])
        setAttendanceTotal(res.pagination?.total ?? 0)
      } catch (err) {
        msgApi.error(getErrMsg(err, 'Gagal memuat laporan absensi'))
      } finally {
        setLoading(false)
      }
    },
    [attendanceFilter, attendancePage, msgApi],
  )

  const loadCorrections = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiListSecurityCorrections(correctionFilterStatus)
      setCorrections(data)
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal memuat koreksi absensi'))
    } finally {
      setLoading(false)
    }
  }, [correctionFilterStatus, msgApi])

  const loadDevices = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiListSecurityDevices()
      setDevices(data)
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal memuat perangkat pos'))
    } finally {
      setLoading(false)
    }
  }, [msgApi])

  const loadGuestBooks = useCallback(
    async (page = guestPage) => {
      setLoading(true)
      try {
        const res = await apiListSecurityGuestBooks({
          ...guestFilter,
          page,
          page_size: 20,
        })
        setGuestBooks(res.data ?? [])
        setGuestTotal(res.pagination?.total ?? 0)
      } catch (err) {
        msgApi.error(getErrMsg(err, 'Gagal memuat buku tamu'))
      } finally {
        setLoading(false)
      }
    },
    [guestFilter, guestPage, msgApi],
  )

  useEffect(() => {
    if (!canManage) return
    void loadDashboard()
  }, [canManage, loadDashboard])

  useEffect(() => {
    if (!canManage) return
    if (activeTab === 'dashboard') void loadDashboard()
    else if (activeTab === 'officers' || activeTab === 'shifts') void loadBaseLists()
    else if (activeTab === 'schedules') void loadSchedules(1)
    else if (activeTab === 'swaps') void loadSwaps()
    else if (activeTab === 'attendance') void loadAttendance(1)
    else if (activeTab === 'corrections') void loadCorrections()
    else if (activeTab === 'devices') void loadDevices()
    else if (activeTab === 'guest_books') void loadGuestBooks(1)
  }, [
    activeTab,
    canManage,
    loadAttendance,
    loadBaseLists,
    loadCorrections,
    loadDashboard,
    loadDevices,
    loadGuestBooks,
    loadSchedules,
    loadSwaps,
  ])

  if (!canManage) {
    return (
      <div style={{ padding: 24 }}>
        <Alert
          type="error"
          showIcon
          message="Akses Ditolak"
          description="Halaman pengelolaan keamanan RT hanya dapat diakses oleh Ketua RT atau pengelola berwenang."
        />
      </div>
    )
  }

  const handleOpenCreateShift = () => {
    setEditingShift(null)
    shiftForm.setFieldsValue({
      name: '',
      start_time: dayjs('20:00', 'HH:mm'),
      end_time: dayjs('06:00', 'HH:mm'),
      crosses_midnight: true,
      lateness_tolerance_minutes: 15,
      check_in_window_before_minutes: 30,
      check_in_window_after_minutes: 60,
      absent_cutoff_minutes: 60,
      missing_checkout_cutoff_minutes: 120,
      is_active: true,
    })
    setShiftModalOpen(true)
  }

  const handleOpenEditShift = (shift: ShiftConfig) => {
    setEditingShift(shift)
    shiftForm.setFieldsValue({
      ...shift,
      start_time: dayjs(shift.start_time.slice(0, 5), 'HH:mm'),
      end_time: dayjs(shift.end_time.slice(0, 5), 'HH:mm'),
    })
    setShiftModalOpen(true)
  }

  const handleSaveShift = async () => {
    try {
      const vals = await shiftForm.validateFields()
      setSubmitting(true)
      if (editingShift) {
        await apiUpdateSecurityShift(editingShift.id, vals)
        msgApi.success('Konfigurasi shift diperbarui (versi baru tercatat)')
      } else {
        await apiCreateSecurityShift(vals)
        msgApi.success('Shift keamanan berhasil dibuat')
      }
      setShiftModalOpen(false)
      await loadBaseLists()
    } catch (err) {
      if (err && typeof err === 'object' && 'errorFields' in err) return
      msgApi.error(getErrMsg(err, 'Gagal menyimpan konfigurasi shift'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleCreateSchedule = async () => {
    try {
      const vals = await scheduleForm.validateFields()
      setSubmitting(true)
      if (editingSchedule) {
        await apiUpdateSecuritySchedule(editingSchedule.id, {
          user_id: vals.user_id,
          shift_config_id: vals.shift_config_id,
          start_date: vals.start_date,
          end_date: vals.end_date,
          notes: vals.notes || null,
        })
        msgApi.success('Jadwal petugas keamanan diperbarui')
      } else {
        await apiCreateSecuritySchedule({
          user_id: vals.user_id,
          shift_config_id: vals.shift_config_id,
          start_date: vals.start_date,
          end_date: vals.end_date,
          notes: vals.notes || null,
        })
        msgApi.success('Jadwal petugas keamanan ditambahkan')
      }
      setScheduleModalOpen(false)
      if (scheduleDetail) setScheduleDetail(null)
      setEditingSchedule(null)
      scheduleForm.resetFields()
      await Promise.all([loadSchedules(1), loadDashboard(), loadAttendance(1)])
    } catch (err) {
      if (err && typeof err === 'object' && 'errorFields' in err) return
      msgApi.error(getErrMsg(err, 'Gagal menambahkan jadwal petugas'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleEditSchedule = async (row: SecuritySchedule) => {
    try {
      const rows = await apiGetSecuritySchedule(row.id)
      const selected = rows.find((item) => item.id === row.id) ?? row
      setEditingSchedule(selected)
      scheduleForm.setFieldsValue({
        user_id: selected.user_id,
        shift_config_id: selected.shift_config_id,
        start_date: selected.assignment_start_date || selected.shift_date,
        end_date: selected.assignment_end_date || selected.shift_date,
      })
      setScheduleModalOpen(true)
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal memuat detail jadwal'))
    }
  }

  const handleScheduleDetail = async (row: SecuritySchedule) => {
    try {
      setScheduleDetail(await apiGetSecuritySchedule(row.id))
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal memuat detail jadwal'))
    }
  }

  const handleCancelSchedule = async (id: string) => {
    try {
      await apiCancelSecuritySchedule(id)
      msgApi.success('Jadwal dibatalkan')
      await loadSchedules(schedulePage)
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal membatalkan jadwal'))
    }
  }

  const handleExportCSV = async () => {
    setExporting(true)
    try {
      const blob = await apiExportSecurityAttendanceCSV(attendanceFilter)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `laporan-absensi-keamanan-${new Date().toISOString().slice(0, 10)}.csv`
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      msgApi.success('Laporan CSV berhasil diunduh')
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal mengekspor laporan CSV'))
    } finally {
      setExporting(false)
    }
  }

  const handleCreateCorrection = async () => {
    if (!correctionModalRecord) return
    try {
      const vals = await correctionForm.validateFields()
      setSubmitting(true)
      await apiCreateSecurityCorrection({
        attendance_record_id: correctionModalRecord.id,
        request_type: vals.request_type,
        requested_status: vals.requested_status,
        requested_checked_in_at: vals.requested_checked_in_at
          ? new Date(vals.requested_checked_in_at).toISOString()
          : undefined,
        requested_checked_out_at: vals.requested_checked_out_at
          ? new Date(vals.requested_checked_out_at).toISOString()
          : undefined,
        reason: vals.reason,
      })
      msgApi.success('Pengajuan koreksi absensi berhasil dibuat')
      setCorrectionModalRecord(null)
      correctionForm.resetFields()
      await loadCorrections()
    } catch (err) {
      if (err && typeof err === 'object' && 'errorFields' in err) return
      msgApi.error(getErrMsg(err, 'Gagal mengajukan koreksi absensi'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleSubmitDecision = async () => {
    if (!decisionModal) return
    setSubmitting(true)
    try {
      if (decisionModal.kind === 'swap') {
        await apiDecideSecuritySwap(decisionModal.id, decisionModal.approve, decisionNote)
        msgApi.success(
          decisionModal.approve ? 'Pertukaran shift disetujui' : 'Pertukaran shift ditolak',
        )
        await loadSwaps()
      } else {
        await apiDecideSecurityCorrection(decisionModal.id, decisionModal.approve, decisionNote)
        msgApi.success(
          decisionModal.approve ? 'Koreksi absensi disetujui' : 'Koreksi absensi ditolak',
        )
        await loadCorrections()
      }
      setDecisionModal(null)
      setDecisionNote('')
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal memproses keputusan'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleCreateDevice = async () => {
    try {
      const vals = await deviceForm.validateFields()
      setSubmitting(true)
      const res = await apiCreateSecurityDevice(vals)
      setDeviceModalOpen(false)
      deviceForm.resetFields()
      setProvisionResult(res)
      msgApi.success('Perangkat pos terdaftar. Salin token pairing sekarang.')
      await loadDevices()
    } catch (err) {
      if (err && typeof err === 'object' && 'errorFields' in err) return
      msgApi.error(getErrMsg(err, 'Gagal mendaftarkan perangkat pos'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleRotatePairing = async (id: string) => {
    try {
      const res = await apiRotateDevicePairing(id)
      setProvisionResult(res)
      msgApi.success('Token pairing baru berhasil dibuat')
      await loadDevices()
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal membuat ulang token pairing'))
    }
  }

  const handleRevokeDevice = async (id: string) => {
    try {
      await apiRevokeSecurityDevice(id)
      msgApi.success('Akses perangkat pos dicabut')
      await loadDevices()
    } catch (err) {
      msgApi.error(getErrMsg(err, 'Gagal mencabut perangkat pos'))
    }
  }

  const copyText = async (val: string) => {
    try {
      await navigator.clipboard.writeText(val)
      msgApi.success('Token pairing disalin ke clipboard')
    } catch {
      msgApi.info('Silakan salin token secara manual dari kotak di atas')
    }
  }

  const officerColumns: ColumnsType<SecurityOfficer> = [
    {
      title: 'Nama Petugas',
      dataIndex: 'full_name',
      key: 'full_name',
      render: (v: string, record) => (
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <ResidentAvatar
            name={record.full_name}
            avatarPath={record.avatar_path ?? null}
            avatarThumbPath={record.avatar_thumb_path ?? null}
            size={36}
          />
          <strong>{v}</strong>
        </div>
      ),
    },
    { title: 'Email', dataIndex: 'email', key: 'email', render: (v) => v || '-' },
    { title: 'Telepon', dataIndex: 'phone_number', key: 'phone', render: (v) => v || '-' },
    {
      title: 'Status',
      dataIndex: 'is_active',
      key: 'is_active',
      render: (active: boolean | undefined) => (
        <Tag color={active ? 'green' : 'default'}>{active ? 'Aktif' : 'Nonaktif'}</Tag>
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 200,
      render: (_v: unknown, record: SecurityOfficer) =>
        record.resident_id ? (
          <ResidentPhotoManage
            residentId={record.resident_id}
            fullName={record.full_name}
            avatarPath={record.avatar_path ?? null}
            compact
            onChanged={(newPath, newThumb) => {
              setOfficers((prev) =>
                prev.map((o) =>
                  o.user_id === record.user_id
                    ? { ...o, avatar_path: newPath, avatar_thumb_path: newThumb }
                    : o,
                ),
              )
              void loadBaseLists()
            }}
          />
        ) : null,
    },
  ]

  const shiftColumns: ColumnsType<ShiftConfig> = [
    {
      title: 'Nama Shift',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, row) => (
        <Space>
          <strong>{name}</strong>
          <Tag color="blue">v{row.version}</Tag>
          {row.crosses_midnight && <Tag color="purple">Lintas Hari</Tag>}
        </Space>
      ),
    },
    {
      title: 'Jam Tugas',
      key: 'hours',
      render: (_, row) => `${row.start_time.slice(0, 5)} - ${row.end_time.slice(0, 5)} WIB`,
    },
    {
      title: 'Toleransi / Cutoff',
      key: 'rules',
      render: (_, row) => (
        <Text type="secondary">
          Telat {row.lateness_tolerance_minutes} mnt · Window -{row.check_in_window_before_minutes}/+
          {row.check_in_window_after_minutes} mnt · Alpa {row.absent_cutoff_minutes} mnt · Lupa Checkout{' '}
          {row.missing_checkout_cutoff_minutes} mnt
        </Text>
      ),
    },
    {
      title: 'Status',
      dataIndex: 'is_active',
      key: 'is_active',
      render: (active: boolean) => <Tag color={active ? 'green' : 'default'}>{active ? 'Aktif' : 'Nonaktif'}</Tag>,
    },
    {
      title: 'Aksi',
      key: 'actions',
      render: (_, row) => (
        <Button size="small" icon={<EditOutlined />} onClick={() => handleOpenEditShift(row)}>
          Ubah
        </Button>
      ),
    },
  ]

  const scheduleColumns: ColumnsType<SecuritySchedule> = [
    { title: 'Periode Tugas', key: 'period', render: (_, row) => `${row.assignment_start_date || row.shift_date} – ${row.assignment_end_date || row.shift_date}` },
    { title: 'Petugas', dataIndex: 'officer_name', key: 'officer_name', render: (v) => v || '-' },
    {
      title: 'Shift',
      key: 'shift',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <span>{row.shift_name}</span>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {row.start_time.slice(0, 5)} - {row.end_time.slice(0, 5)} (v{row.shift_version})
          </Text>
        </Space>
      ),
    },
    {
      title: 'Status Jadwal',
      dataIndex: 'status',
      key: 'status',
      render: (st: string) => (
        <Tag color={st === 'scheduled' ? 'blue' : st === 'cancelled' ? 'red' : 'default'}>
          {st === 'scheduled' ? 'Aktif' : st === 'cancelled' ? 'Dibatalkan' : st}
        </Tag>
      ),
    },
    {
      title: 'Absensi',
      key: 'attendance',
      render: (_, row) => {
        const meta = ATTENDANCE_STATUS_TAG[row.attendance_status ?? 'belum_hadir'] ?? {
          color: 'default',
          label: row.attendance_status_label || 'Belum Hadir',
        }
        return <Tag color={meta.color}>{row.attendance_status_label || meta.label}</Tag>
      },
    },
    {
      title: 'Aksi',
      key: 'actions',
      render: (_, row) => (
        <Space>
          <Button size="small" onClick={() => void handleScheduleDetail(row)}>Detail</Button>
          {row.status === 'scheduled' && row.attendance_status === 'belum_hadir' && row.shift_date >= (dashboard?.today_date ?? '') && (
            <Button size="small" icon={<EditOutlined />} onClick={() => void handleEditSchedule(row)}>Ubah</Button>
          )}
          {row.status === 'scheduled' && !row.checked_in_at ? (
          <Popconfirm
            title="Batalkan jadwal petugas ini?"
            okText="Ya, Batalkan"
            cancelText="Tidak"
            onConfirm={() => handleCancelSchedule(row.id)}
          >
            <Button size="small" danger icon={<StopOutlined />}>
              Batalkan
            </Button>
          </Popconfirm>
          ) : null}
        </Space>
      ),
    },
  ]

  const swapColumns: ColumnsType<ShiftSwap> = [
    {
      title: 'Pemohon',
      key: 'requester',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <strong>{row.requester_name || row.requester_user_id}</strong>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {row.requester_shift_date} · {row.requester_shift_name}
          </Text>
        </Space>
      ),
    },
    {
      title: 'Pengganti',
      key: 'target',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <strong>{row.target_name || row.target_user_id}</strong>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {row.target_shift_date} · {row.target_shift_name}
          </Text>
        </Space>
      ),
    },
    { title: 'Alasan', dataIndex: 'reason', key: 'reason' },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (st: string) => {
        const meta = SWAP_STATUS_TAG[st] ?? { color: 'default', label: st }
        return <Tag color={meta.color}>{meta.label}</Tag>
      },
    },
    {
      title: 'Aksi Ketua RT',
      key: 'actions',
      render: (_, row) =>
        row.status === 'pending_ketua' ? (
          <Space>
            <Button
              size="small"
              type="primary"
              icon={<CheckCircleOutlined />}
              onClick={() =>
                setDecisionModal({
                  kind: 'swap',
                  id: row.id,
                  approve: true,
                  title: `Setujui Pertukaran ${row.requester_name ?? ''} ↔ ${row.target_name ?? ''}`,
                })
              }
            >
              Setujui
            </Button>
            <Button
              size="small"
              danger
              icon={<CloseCircleOutlined />}
              onClick={() =>
                setDecisionModal({
                  kind: 'swap',
                  id: row.id,
                  approve: false,
                  title: `Tolak Pertukaran ${row.requester_name ?? ''} ↔ ${row.target_name ?? ''}`,
                })
              }
            >
              Tolak
            </Button>
          </Space>
        ) : (
          <Text type="secondary">{row.ketua_note || '-'}</Text>
        ),
    },
  ]

  const attendanceColumns: ColumnsType<AttendanceRecord> = [
    { title: 'Tanggal', dataIndex: 'shift_date', key: 'shift_date' },
    { title: 'Petugas', dataIndex: 'officer_name', key: 'officer_name', render: (v) => v || '-' },
    {
      title: 'Shift',
      key: 'shift',
      render: (_, row) => `${row.shift_name} (${row.start_time.slice(0, 5)}-${row.end_time.slice(0, 5)})`,
    },
    {
      title: 'Status',
      key: 'status',
      render: (_, row) => {
        const meta = ATTENDANCE_STATUS_TAG[row.status] ?? { color: 'default', label: row.status_label }
        return <Tag color={meta.color}>{row.status_label || meta.label}</Tag>
      },
    },
    {
      title: 'Terlambat',
      dataIndex: 'late_minutes',
      key: 'late_minutes',
      render: (m: number) => (m > 0 ? <Tag color="warning">{m} menit</Tag> : '0 menit'),
    },
    {
      title: 'Check-In',
      key: 'in',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <span>{formatTimeOnly(row.checked_in_at)}</span>
          {row.check_in_device_name && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              {row.check_in_device_name}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: 'Check-Out',
      key: 'out',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <span>{formatTimeOnly(row.checked_out_at)}</span>
          {row.check_out_device_name && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              {row.check_out_device_name}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: 'Aksi',
      key: 'actions',
      render: (_, row) => (
        <Button
          size="small"
          onClick={() => {
            setCorrectionModalRecord(row)
            correctionForm.setFieldsValue({
              request_type: 'status_change',
              requested_status: 'sudah_absen_pulang',
              reason: '',
            })
          }}
        >
          Koreksi
        </Button>
      ),
    },
  ]

  const correctionColumns: ColumnsType<AttendanceCorrection> = [
    {
      title: 'Jadwal',
      key: 'schedule',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <strong>{row.officer_name || row.officer_user_id}</strong>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {row.shift_date} · {row.shift_name}
          </Text>
        </Space>
      ),
    },
    { title: 'Pengaju', dataIndex: 'requested_by_name', key: 'requested_by_name' },
    {
      title: 'Perubahan Status',
      key: 'diff',
      render: (_, row) => (
        <Space>
          <Tag>{ATTENDANCE_STATUS_TAG[row.original_status]?.label ?? row.original_status}</Tag>
          <span>→</span>
          <Tag color="blue">{ATTENDANCE_STATUS_TAG[row.requested_status]?.label ?? row.requested_status}</Tag>
        </Space>
      ),
    },
    { title: 'Alasan', dataIndex: 'reason', key: 'reason' },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (st: string) => {
        const meta = CORRECTION_STATUS_TAG[st] ?? { color: 'default', label: st }
        return <Tag color={meta.color}>{meta.label}</Tag>
      },
    },
    {
      title: 'Aksi',
      key: 'actions',
      render: (_, row) => {
        if (row.status !== 'pending') {
          return (
            <Text type="secondary">
              {row.decided_by_name ? `Oleh ${row.decided_by_name}` : '-'}
              {row.decision_note ? ` (${row.decision_note})` : ''}
            </Text>
          )
        }
        const isSelfRequested = Boolean(user?.id && row.requested_by === user.id)
        if (isSelfRequested) {
          return <Tag color="default">Tidak bisa menyetujui pengajuan sendiri</Tag>
        }
        return (
          <Space>
            <Button
              size="small"
              type="primary"
              icon={<CheckCircleOutlined />}
              onClick={() =>
                setDecisionModal({
                  kind: 'correction',
                  id: row.id,
                  approve: true,
                  title: `Setujui Koreksi ${row.officer_name ?? ''}`,
                })
              }
            >
              Setujui
            </Button>
            <Button
              size="small"
              danger
              icon={<CloseCircleOutlined />}
              onClick={() =>
                setDecisionModal({
                  kind: 'correction',
                  id: row.id,
                  approve: false,
                  title: `Tolak Koreksi ${row.officer_name ?? ''}`,
                })
              }
            >
              Tolak
            </Button>
          </Space>
        )
      },
    },
  ]

  const deviceColumns: ColumnsType<SecurityDevice> = [
    {
      title: 'Perangkat Pos',
      key: 'device',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <strong>{row.device_name}</strong>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {row.location_label}
          </Text>
        </Space>
      ),
    },
    {
      title: 'Status',
      key: 'status',
      render: (_, row) => {
        if (!row.is_active || row.revoked_at) return <Tag color="red">Dicabut</Tag>
        if (row.is_paired) return <Tag color="green">Aktif & Terpasang</Tag>
        return <Tag color="gold">Menunggu Aktivasi</Tag>
      },
    },
    {
      title: 'Terakhir Aktif',
      dataIndex: 'last_active_at',
      key: 'last_active_at',
      render: (v) => formatDateTime(v),
    },
    {
      title: 'Aksi',
      key: 'actions',
      render: (_, row) =>
        row.is_active && !row.revoked_at ? (
          <Space>
            <Popconfirm
              title="Buat token pairing baru? Token perangkat lama akan diganti."
              okText="Ya, Buat Ulang"
              cancelText="Batal"
              onConfirm={() => handleRotatePairing(row.id)}
            >
              <Button size="small" icon={<KeyOutlined />}>
                Token Baru
              </Button>
            </Popconfirm>
            <Popconfirm
              title="Cabut akses perangkat pos ini?"
              okText="Ya, Cabut"
              cancelText="Batal"
              onConfirm={() => handleRevokeDevice(row.id)}
            >
              <Button size="small" danger icon={<StopOutlined />}>
                Cabut
              </Button>
            </Popconfirm>
          </Space>
        ) : (
          <Text type="secondary">Nonaktif ({formatDateTime(row.revoked_at)})</Text>
        ),
    },
  ]

  const guestColumns: ColumnsType<GuestBookEntry> = [
    { title: 'Nama Tamu', dataIndex: 'guest_name', key: 'guest_name', render: (v) => <strong>{v}</strong> },
    {
      title: 'No. HP',
      key: 'phone',
      render: (_, row) => row.phone_masked || row.phone || '-',
    },
    { title: 'Keperluan', dataIndex: 'purpose', key: 'purpose' },
    { title: 'Tujuan', dataIndex: 'destination_label', key: 'destination_label' },
    { title: 'Kendaraan', dataIndex: 'vehicle_plate', key: 'vehicle_plate', render: (v) => v || '-' },
    { title: 'Masuk', dataIndex: 'checked_in_at', key: 'checked_in_at', render: (v) => formatDateTime(v) },
    {
      title: 'Keluar',
      dataIndex: 'checked_out_at',
      key: 'checked_out_at',
      render: (v) => (v ? formatDateTime(v) : <Tag color="processing">Masih di RT</Tag>),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      {contextHolder}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space align="center">
          <SafetyCertificateOutlined style={{ fontSize: 24, color: '#0f766e' }} />
          <div>
            <Title level={4} style={{ margin: 0 }}>
              Kelola Keamanan RT
            </Title>
            <Text type="secondary">
              Manajemen shift, jadwal petugas, absensi QR dinamis, koreksi, perangkat pos, dan buku tamu
            </Text>
          </div>
        </Space>
        <Button icon={<ReloadOutlined />} onClick={() => setActiveTab((prev) => prev)}>
          Refresh
        </Button>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'dashboard',
            label: 'Dashboard Monitoring',
            children: (
              <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                <Row gutter={[16, 16]}>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic title="Petugas Aktif" value={dashboard?.total_officers ?? 0} />
                    </Card>
                  </Col>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic title="Dijadwalkan Hari Ini" value={dashboard?.scheduled_today_count ?? 0} />
                    </Card>
                  </Col>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic
                        title="Hadir / Selesai"
                        value={(dashboard?.present_today_count ?? 0) + (dashboard?.checked_out_today_count ?? 0)}
                      />
                    </Card>
                  </Col>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic
                        title="Terlambat / Alpa / Lupa Out"
                        value={
                          (dashboard?.late_today_count ?? 0) +
                          (dashboard?.absent_today_count ?? 0) +
                          (dashboard?.missing_checkout_count ?? 0)
                        }
                      />
                    </Card>
                  </Col>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic title="Perangkat Pos Aktif" value={dashboard?.active_devices_count ?? 0} />
                    </Card>
                  </Col>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic title="Tamu Masih Berkunjung" value={dashboard?.active_guests_count ?? 0} />
                    </Card>
                  </Col>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic
                        title="Pertukaran Pending"
                        value={dashboard?.pending_swaps_count ?? 0}
                        prefix={<SwapOutlined />}
                      />
                    </Card>
                  </Col>
                  <Col xs={12} md={6}>
                    <Card size="small">
                      <Statistic title="Koreksi Pending" value={dashboard?.pending_corrections ?? 0} />
                    </Card>
                  </Col>
                </Row>

                <Card
                  title={`Absensi Hari Ini (${dashboard?.today_date ?? '-'})`}
                  extra={
                    <Button size="small" icon={<ReloadOutlined />} onClick={() => void loadDashboard()}>
                      Muat Ulang
                    </Button>
                  }
                >
                  <Table
                    rowKey="id"
                    loading={loading}
                    dataSource={dashboard?.today_attendance ?? []}
                    columns={attendanceColumns.filter((c) => c.key !== 'actions')}
                    pagination={false}
                    locale={{ emptyText: <Empty description="Belum ada jadwal bertugas hari ini" /> }}
                  />
                </Card>
              </Space>
            ),
          },
          {
            key: 'officers',
            label: `Data Petugas (${officers.length})`,
            children: (
              <Card>
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 16 }}
                  message="Daftar petugas keamanan diambil otomatis dari pengguna RT aktif dengan jabatan Keamanan."
                />
                <Table rowKey="user_id" loading={loading} dataSource={officers} columns={officerColumns} />
              </Card>
            ),
          },
          {
            key: 'shifts',
            label: 'Pengaturan Shift',
            children: (
              <Card
                extra={
                  <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreateShift}>
                    Tambah Shift
                  </Button>
                }
              >
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 16 }}
                  message="Mengubah aturan shift akan menaikkan nomor versi dan menjaga snapshot absensi historis tetap akurat."
                />
                <Table rowKey="id" loading={loading} dataSource={shifts} columns={shiftColumns} pagination={false} />
              </Card>
            ),
          },
          {
            key: 'schedules',
            label: 'Jadwal Petugas',
            children: (
              <Card
                extra={
                  <Button
                    type="primary"
                    icon={<PlusOutlined />}
                    onClick={() => {
                      setEditingSchedule(null)
                      scheduleForm.resetFields()
                      setScheduleModalOpen(true)
                    }}
                  >
                    Tambah Jadwal
                  </Button>
                }
              >
                <Space wrap style={{ marginBottom: 16 }}>
                  <Input
                    type="date"
                    aria-label="Filter tanggal mulai"
                    value={scheduleFilter.date_from ?? ''}
                    onChange={(e) => setScheduleFilter((p) => ({ ...p, date_from: e.target.value || undefined }))}
                    style={{ width: 160 }}
                  />
                  <Input
                    type="date"
                    aria-label="Filter tanggal akhir"
                    value={scheduleFilter.date_to ?? ''}
                    onChange={(e) => setScheduleFilter((p) => ({ ...p, date_to: e.target.value || undefined }))}
                    style={{ width: 160 }}
                  />
                  <Select
                    allowClear
                    placeholder="Semua Petugas"
                    style={{ width: 200 }}
                    value={scheduleFilter.user_id}
                    onChange={(v) => setScheduleFilter((p) => ({ ...p, user_id: v }))}
                    options={officers.map((o) => ({ value: o.user_id, label: o.full_name }))}
                  />
                  <Select
                    allowClear
                    placeholder="Semua Shift"
                    style={{ width: 180 }}
                    value={scheduleFilter.shift_config_id}
                    onChange={(v) => setScheduleFilter((p) => ({ ...p, shift_config_id: v }))}
                    options={shifts.map((s) => ({ value: s.id, label: s.name }))}
                  />
                  <Button type="primary" onClick={() => void loadSchedules(1)}>
                    Terapkan Filter
                  </Button>
                </Space>
                <Table
                  rowKey="id"
                  loading={loading}
                  dataSource={schedules}
                  columns={scheduleColumns}
                  pagination={{
                    current: schedulePage,
                    pageSize: 20,
                    total: scheduleTotal,
                    onChange: (p) => {
                      setSchedulePage(p)
                      void loadSchedules(p)
                    },
                  }}
                />
              </Card>
            ),
          },
          {
            key: 'swaps',
            label: (
              <Badge count={dashboard?.pending_swaps_count ?? 0} offset={[10, 0]} size="small">
                <span>Pertukaran Shift</span>
              </Badge>
            ),
            children: (
              <Card>
                <Space style={{ marginBottom: 16 }}>
                  <Select
                    allowClear
                    placeholder="Filter status"
                    style={{ width: 260 }}
                    value={swapFilterStatus}
                    onChange={(v) => setSwapFilterStatus(v)}
                    options={Object.entries(SWAP_STATUS_TAG).map(([k, v]) => ({ value: k, label: v.label }))}
                  />
                  <Button onClick={() => void loadSwaps()} icon={<ReloadOutlined />}>
                    Muat Ulang
                  </Button>
                </Space>
                <Table rowKey="id" loading={loading} dataSource={swaps} columns={swapColumns} />
              </Card>
            ),
          },
          {
            key: 'attendance',
            label: 'Monitoring & Laporan Absensi',
            children: (
              <Card
                extra={
                  <Button icon={<DownloadOutlined />} loading={exporting} onClick={() => void handleExportCSV()}>
                    Export CSV
                  </Button>
                }
              >
                <Space wrap style={{ marginBottom: 16 }}>
                  <Input
                    type="date"
                    aria-label="Tanggal mulai absensi"
                    value={attendanceFilter.date_from ?? ''}
                    onChange={(e) => setAttendanceFilter((p) => ({ ...p, date_from: e.target.value || undefined }))}
                    style={{ width: 160 }}
                  />
                  <Input
                    type="date"
                    aria-label="Tanggal akhir absensi"
                    value={attendanceFilter.date_to ?? ''}
                    onChange={(e) => setAttendanceFilter((p) => ({ ...p, date_to: e.target.value || undefined }))}
                    style={{ width: 160 }}
                  />
                  <Select
                    allowClear
                    placeholder="Semua Petugas"
                    style={{ width: 180 }}
                    value={attendanceFilter.user_id}
                    onChange={(v) => setAttendanceFilter((p) => ({ ...p, user_id: v }))}
                    options={officers.map((o) => ({ value: o.user_id, label: o.full_name }))}
                  />
                  <Select
                    allowClear
                    placeholder="Semua Status"
                    style={{ width: 180 }}
                    value={attendanceFilter.status}
                    onChange={(v) => setAttendanceFilter((p) => ({ ...p, status: v }))}
                    options={Object.entries(ATTENDANCE_STATUS_TAG).map(([k, v]) => ({ value: k, label: v.label }))}
                  />
                  <Button type="primary" onClick={() => void loadAttendance(1)}>
                    Cari
                  </Button>
                </Space>
                <Table
                  rowKey="id"
                  loading={loading}
                  dataSource={attendance}
                  columns={attendanceColumns}
                  pagination={{
                    current: attendancePage,
                    pageSize: 20,
                    total: attendanceTotal,
                    onChange: (p) => {
                      setAttendancePage(p)
                      void loadAttendance(p)
                    },
                  }}
                />
              </Card>
            ),
          },
          {
            key: 'corrections',
            label: (
              <Badge count={dashboard?.pending_corrections ?? 0} offset={[10, 0]} size="small">
                <span>Koreksi Absensi</span>
              </Badge>
            ),
            children: (
              <Card>
                <Space style={{ marginBottom: 16 }}>
                  <Select
                    allowClear
                    placeholder="Filter status koreksi"
                    style={{ width: 220 }}
                    value={correctionFilterStatus}
                    onChange={(v) => setCorrectionFilterStatus(v)}
                    options={Object.entries(CORRECTION_STATUS_TAG).map(([k, v]) => ({ value: k, label: v.label }))}
                  />
                  <Button onClick={() => void loadCorrections()} icon={<ReloadOutlined />}>
                    Muat Ulang
                  </Button>
                </Space>
                <Table rowKey="id" loading={loading} dataSource={corrections} columns={correctionColumns} />
              </Card>
            ),
          },
          {
            key: 'devices',
            label: 'Perangkat Pos',
            children: (
              <Card
                extra={
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => setDeviceModalOpen(true)}>
                    Daftarkan Perangkat Pos
                  </Button>
                }
              >
                <Table rowKey="id" loading={loading} dataSource={devices} columns={deviceColumns} />
              </Card>
            ),
          },
          {
            key: 'guest_books',
            label: 'Buku Tamu',
            children: (
              <Card>
                <Space wrap style={{ marginBottom: 16 }}>
                  <Select
                    allowClear
                    placeholder="Status Kunjungan"
                    style={{ width: 180 }}
                    value={guestFilter.status}
                    onChange={(v) => setGuestFilter((p) => ({ ...p, status: v }))}
                    options={[
                      { value: 'active', label: 'Masih Berkunjung' },
                      { value: 'checked_out', label: 'Sudah Keluar' },
                    ]}
                  />
                  <Input
                    placeholder="Cari nama / tujuan / plat..."
                    style={{ width: 240 }}
                    value={guestFilter.search ?? ''}
                    onChange={(e) => setGuestFilter((p) => ({ ...p, search: e.target.value || undefined }))}
                  />
                  <Button type="primary" onClick={() => void loadGuestBooks(1)}>
                    Cari
                  </Button>
                </Space>
                <Table
                  rowKey="id"
                  loading={loading}
                  dataSource={guestBooks}
                  columns={guestColumns}
                  pagination={{
                    current: guestPage,
                    pageSize: 20,
                    total: guestTotal,
                    onChange: (p) => {
                      setGuestPage(p)
                      void loadGuestBooks(p)
                    },
                  }}
                />
              </Card>
            ),
          },
        ]}
      />

      {/* Shift Config Modal */}
      <Modal
        open={shiftModalOpen}
        title={editingShift ? `Ubah Shift: ${editingShift.name}` : 'Tambah Shift Keamanan'}
        onCancel={() => setShiftModalOpen(false)}
        onOk={() => void handleSaveShift()}
        confirmLoading={submitting}
        okText="Simpan"
        cancelText="Batal"
      >
        <Form form={shiftForm} layout="vertical">
          <Form.Item name="name" label="Nama Shift" rules={[{ required: true, message: 'Nama shift wajib diisi' }]}>
            <Input placeholder="Contoh: Shift Malam" />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="start_time" label="Jam Mulai (WIB)" rules={[{ required: true }]}>
                <TimePicker
                  style={{ width: '100%' }}
                  format="HH:mm"
                  placeholder="Jam mulai"
                  allowClear={false}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="end_time" label="Jam Selesai (WIB)" rules={[{ required: true }]}>
                <TimePicker
                  style={{ width: '100%' }}
                  format="HH:mm"
                  placeholder="Jam selesai"
                  allowClear={false}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="crosses_midnight" label="Shift Lintas Tengah Malam" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="lateness_tolerance_minutes" label="Toleransi Telat (menit)" rules={[{ required: true }]}>
                <InputNumber min={0} max={240} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="absent_cutoff_minutes" label="Batas Alpa (menit)" rules={[{ required: true }]}>
                <InputNumber min={15} max={720} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                name="check_in_window_before_minutes"
                label="Buka Check-In Sebelum (menit)"
                rules={[{ required: true }]}
              >
                <InputNumber min={0} max={240} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="check_in_window_after_minutes"
                label="Tutup Check-In Sesudah (menit)"
                rules={[{ required: true }]}
              >
                <InputNumber min={10} max={480} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                name="missing_checkout_cutoff_minutes"
                label="Batas Lupa Checkout (menit)"
                rules={[{ required: true }]}
              >
                <InputNumber min={15} max={720} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="is_active" label="Aktif" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      {/* Create Schedule Modal */}
      <Modal
        open={scheduleModalOpen}
        title={editingSchedule ? 'Ubah Jadwal Petugas Keamanan' : 'Tambah Jadwal Petugas Keamanan'}
        onCancel={() => { setScheduleModalOpen(false); setEditingSchedule(null) }}
        onOk={() => void handleCreateSchedule()}
        confirmLoading={submitting}
        okText={editingSchedule ? 'Simpan Perubahan' : 'Simpan Jadwal'}
        cancelText="Batal"
      >
        <Form form={scheduleForm} layout="vertical">
          <Form.Item name="user_id" label="Petugas Keamanan" rules={[{ required: true, message: 'Pilih petugas' }]}>
            <Select
              placeholder="Pilih petugas keamanan"
              options={officers.map((o) => ({ value: o.user_id, label: o.full_name }))}
            />
          </Form.Item>
          <Form.Item name="shift_config_id" label="Shift" rules={[{ required: true, message: 'Pilih shift' }]}>
            <Select
              placeholder="Pilih shift"
              options={shifts
                .filter((s) => s.is_active)
                .map((s) => ({
                  value: s.id,
                  label: `${s.name} (${s.start_time.slice(0, 5)}-${s.end_time.slice(0, 5)})`,
                }))}
            />
          </Form.Item>
          <Form.Item name="start_date" label="Tanggal Mulai Tugas" rules={[{ required: true, message: 'Pilih tanggal mulai' }]}>
            <DatePicker style={{ width: '100%' }} format="DD/MM/YYYY" />
          </Form.Item>
          <Form.Item name="end_date" label="Tanggal Berakhir Tugas" dependencies={['start_date']} rules={[{ required: true, message: 'Pilih tanggal berakhir' }, ({ getFieldValue }) => ({ validator(_, value) { const start = getFieldValue('start_date'); return !start || !value || value >= start ? Promise.resolve() : Promise.reject(new Error('Tanggal berakhir tidak boleh sebelum tanggal mulai')) } })]}>
            <DatePicker style={{ width: '100%' }} format="DD/MM/YYYY" />
          </Form.Item>
          <Form.Item name="notes" label="Catatan (Opsional)">
            <Input.TextArea rows={2} maxLength={300} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal open={Boolean(scheduleDetail)} title="Detail Jadwal Petugas Keamanan" footer={<Button onClick={() => setScheduleDetail(null)}>Tutup</Button>} onCancel={() => setScheduleDetail(null)}>
        {scheduleDetail && scheduleDetail.length > 0 && (
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="Nama Petugas">{scheduleDetail[0].officer_name}</Descriptions.Item>
            <Descriptions.Item label="Jenis Petugas">Keamanan</Descriptions.Item>
            <Descriptions.Item label="Shift">{scheduleDetail[0].shift_name}</Descriptions.Item>
            <Descriptions.Item label="Tanggal Mulai">{scheduleDetail[0].assignment_start_date}</Descriptions.Item>
            <Descriptions.Item label="Tanggal Berakhir">{scheduleDetail[0].assignment_end_date}</Descriptions.Item>
            <Descriptions.Item label="Status Jadwal">{scheduleDetail[0].status}</Descriptions.Item>
            <Descriptions.Item label="Tanggal Pelaksanaan">{scheduleDetail.map((item) => `${item.shift_date} (${item.attendance_status_label || 'Belum Hadir'})`).join(', ')}</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      {/* Register Pos Device Modal */}
      <Modal
        open={deviceModalOpen}
        title="Daftarkan Perangkat Pos Keamanan"
        onCancel={() => setDeviceModalOpen(false)}
        onOk={() => void handleCreateDevice()}
        confirmLoading={submitting}
        okText="Buat Token Pairing"
        cancelText="Batal"
      >
        <Form form={deviceForm} layout="vertical">
          <Form.Item
            name="device_name"
            label="Nama Perangkat"
            rules={[{ required: true, message: 'Nama perangkat wajib diisi' }]}
          >
            <Input placeholder="Contoh: Tablet Pos Gerbang Utama" />
          </Form.Item>
          <Form.Item
            name="location_label"
            label="Lokasi Pos"
            rules={[{ required: true, message: 'Lokasi pos wajib diisi' }]}
          >
            <Input placeholder="Contoh: Gerbang Utama RT 002" />
          </Form.Item>
        </Form>
      </Modal>

      {/* One-time Pairing Token Modal */}
      <Modal
        open={Boolean(provisionResult)}
        title="Token Pairing Perangkat Pos"
        onCancel={() => setProvisionResult(null)}
        footer={[
          <Button
            key="copy"
            type="primary"
            icon={<CopyOutlined />}
            onClick={() => provisionResult && void copyText(provisionResult.pairing_token)}
          >
            Salin Token
          </Button>,
          <Button key="close" onClick={() => setProvisionResult(null)}>
            Tutup
          </Button>,
        ]}
      >
        {provisionResult && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert
              type="warning"
              showIcon
              message="Token ini hanya ditampilkan satu kali."
              description="Masukkan token ini di aplikasi Android pada mode Perangkat Pos sebelum masa berlaku habis."
            />
            <Descriptions bordered column={1} size="small">
              <Descriptions.Item label="Perangkat">{provisionResult.device.device_name}</Descriptions.Item>
              <Descriptions.Item label="Lokasi">{provisionResult.device.location_label}</Descriptions.Item>
              <Descriptions.Item label="Berlaku Sampai">
                {formatDateTime(provisionResult.expires_at)}
              </Descriptions.Item>
              <Descriptions.Item label="Token Pairing">
                <Text code copyable>
                  {provisionResult.pairing_token}
                </Text>
              </Descriptions.Item>
            </Descriptions>
          </Space>
        )}
      </Modal>

      {/* Create Attendance Correction Modal */}
      <Modal
        open={Boolean(correctionModalRecord)}
        title={`Ajukan Koreksi Absensi: ${correctionModalRecord?.officer_name ?? ''}`}
        onCancel={() => setCorrectionModalRecord(null)}
        onOk={() => void handleCreateCorrection()}
        confirmLoading={submitting}
        okText="Kirim Pengajuan"
        cancelText="Batal"
      >
        <Form form={correctionForm} layout="vertical">
          <Form.Item name="request_type" label="Jenis Koreksi" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'status_change', label: 'Perubahan Status' },
                { value: 'time_adjustment', label: 'Penyesuaian Jam Masuk/Pulang' },
                { value: 'manual_check_in', label: 'Check-In Manual' },
                { value: 'manual_check_out', label: 'Check-Out Manual' },
              ]}
            />
          </Form.Item>
          <Form.Item name="requested_status" label="Status yang Diajukan" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'hadir', label: 'Hadir' },
                { value: 'terlambat', label: 'Terlambat' },
                { value: 'sudah_absen_pulang', label: 'Sudah Absen Pulang' },
                { value: 'tidak_hadir', label: 'Tidak Hadir' },
              ]}
            />
          </Form.Item>
          <Form.Item name="requested_checked_in_at" label="Jam Check-In Baru (Opsional)">
            <Input type="datetime-local" />
          </Form.Item>
          <Form.Item name="requested_checked_out_at" label="Jam Check-Out Baru (Opsional)">
            <Input type="datetime-local" />
          </Form.Item>
          <Form.Item name="reason" label="Alasan Koreksi" rules={[{ required: true, message: 'Alasan wajib diisi' }]}>
            <Input.TextArea rows={3} maxLength={500} placeholder="Jelaskan alasan koreksi secara ringkas..." />
          </Form.Item>
        </Form>
      </Modal>

      {/* Approve / Reject Decision Modal */}
      <Modal
        open={Boolean(decisionModal)}
        title={decisionModal?.title ?? ''}
        onCancel={() => {
          setDecisionModal(null)
          setDecisionNote('')
        }}
        onOk={() => void handleSubmitDecision()}
        confirmLoading={submitting}
        okText={decisionModal?.approve ? 'Setujui' : 'Tolak'}
        okButtonProps={{ danger: !decisionModal?.approve }}
        cancelText="Batal"
      >
        <Form layout="vertical">
          <Form.Item label="Catatan Keputusan (Opsional)">
            <Input.TextArea
              rows={3}
              value={decisionNote}
              onChange={(e) => setDecisionNote(e.target.value)}
              placeholder="Tambahkan catatan jika diperlukan..."
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
