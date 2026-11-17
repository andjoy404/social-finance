import { useState, useCallback, useEffect } from 'react'
import {
  Table,
  Button,
  Modal,
  message,
  TableProps,
  Space,
  Typography,
  Input,
  Empty,
} from 'antd'
import {
  FileExcelOutlined,
  FilePdfOutlined,
  EditOutlined,
  PrinterOutlined,
} from '@ant-design/icons'
import { useAuth, hasPermission } from '@/app/AuthContext'
import { getSessionPair } from '@/app/api'
import {
  apiGetMonitoringHouseholds,
  apiUpdateMonitoringNote,
  apiExportMonitoringExcel,
  apiExportMonitoringPDF,
  type ApiMonitoringHousehold,
} from '@/app/api'
import { createPortal } from 'react-dom'
import './MonitoringScreen.css'

const { TextArea } = Input

export function MonitoringScreen() {
  const { user } = useAuth()
  const rtId = user?.rt?.id || ''
  const rtCode = user?.rtCode?.trim()
  const rtName = user?.rt?.name?.trim()
  const rwName = user?.rwName?.trim()
  const rwNumber = user?.rwNumber
  const formattedRtCode = rtCode ? rtCode.padStart(3, '0') : ''
  const formattedRwNumber = typeof rwNumber === 'number'
    ? String(rwNumber).padStart(3, '0')
    : ''
  const rtLabel = [
    formattedRtCode ? `RT ${formattedRtCode}` : '',
    rtName && rtName.toLowerCase() !== `rt ${formattedRtCode}`.toLowerCase() ? rtName : '',
  ].filter(Boolean).join(' — ') || 'RT'
  const rwLabel = formattedRwNumber
    ? `RW ${formattedRwNumber}${rwName && !rwName.toLowerCase().includes(formattedRwNumber) ? ` — ${rwName}` : ''}`
    : rwName && !/^[0-9a-f-]{36}$/i.test(rwName)
      ? rwName
      : ''

  const canExport = hasPermission(user, 'rw.monitoring.export')
  const [households, setHouseholds] = useState<ApiMonitoringHousehold[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState<'excel' | 'pdf' | null>(null)
  const [noteModalOpen, setNoteModalOpen] = useState(false)
  const [editingHousehold, setEditingHousehold] = useState<ApiMonitoringHousehold | null>(null)
  const [noteText, setNoteText] = useState('')
  const [printedAt, setPrintedAt] = useState(() => new Date())

  const token = getSessionPair()?.accessToken ?? ''

  const fetchHouseholds = useCallback(async () => {
    if (!rtId) return
    setLoading(true)
    try {
      const data = await apiGetMonitoringHouseholds(token, rtId)
      setHouseholds(data.households)
      setTotal(data.total)
    } catch (err: any) {
      message.error(err?.message || 'Gagal memuat daftar monitoring')
    } finally {
      setLoading(false)
    }
  }, [rtId, token])

  useEffect(() => {
    fetchHouseholds()
  }, [fetchHouseholds])

  useEffect(() => {
    const updatePrintTime = () => setPrintedAt(new Date())
    window.addEventListener('beforeprint', updatePrintTime)
    return () => window.removeEventListener('beforeprint', updatePrintTime)
  }, [])

  const handleUpdateNote = async (householdOccupancyId: string, note: string) => {
    try {
      await apiUpdateMonitoringNote(token, rtId, householdOccupancyId, note)
      message.success('Catatan diperbarui')
      await fetchHouseholds()
    } catch (err: any) {
      message.error(err?.message || 'Gagal memperbarui catatan')
    }
  }

  const handleExportExcel = async () => {
    if (exporting) return
    setExporting('excel')
    try {
      const blob = await apiExportMonitoringExcel(token, rtId)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `monitoring_kebersihan_${new Date().toISOString().slice(0, 10)}.xlsx`
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.setTimeout(() => window.URL.revokeObjectURL(url), 1000)
    } catch (err: any) {
      message.error(err?.message || 'Gagal mengekspor Excel')
    } finally {
      setExporting(null)
    }
  }

  const handleExportPDF = async () => {
    if (exporting) return
    setExporting('pdf')
    try {
      const blob = await apiExportMonitoringPDF(token, rtId)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `monitoring_kebersihan_${new Date().toISOString().slice(0, 10)}.pdf`
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.setTimeout(() => window.URL.revokeObjectURL(url), 1000)
    } catch (err: any) {
      message.error(err?.message || 'Gagal mengekspor PDF')
    } finally {
      setExporting(null)
    }
  }

  const handlePrint = () => window.print()

  const columns: TableProps<ApiMonitoringHousehold>['columns'] = [
    {
      title: 'No.',
      key: 'index',
      width: 60,
      render: (_: any, _row: any, index: number) => index + 1,
      align: 'center',
    },
    {
      title: 'No. Rumah',
      dataIndex: 'house_number',
      key: 'house_number',
      width: 120,
    },
    {
      title: 'Catatan Operasional',
      dataIndex: 'note',
      key: 'note',
      width: 200,
      render: (note: string, record: ApiMonitoringHousehold) => (
        <span
          onClick={() => {
            setNoteText(note || '')
            setEditingHousehold(record)
            setNoteModalOpen(true)
          }}
          style={{ cursor: 'pointer', textDecoration: 'underline', color: 'var(--sf-accent)' }}
        >
          {note || '—'}
        </span>
      ),
    },
    {
      title: 'Aksi',
      key: 'action',
      width: 100,
      render: (_: any, record: ApiMonitoringHousehold) => (
        <Space>
          <Button
            type="text"
            icon={<EditOutlined />}
            onClick={() => {
              setNoteText(record.note || '')
              setEditingHousehold(record)
              setNoteModalOpen(true)
            }}
          >
            Edit Catatan
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <>
      <div className="monitoring-page">
        <div className="monitoring-screen-content">
      <div className="monitoring-header">
        <div>
          <Typography.Title level={2}>Sampah</Typography.Title>
          <Typography.Text style={{ color: 'var(--sf-text-muted)' }}>
            {rtLabel}{rwLabel ? ` | ${rwLabel}` : ''}
          </Typography.Text>
        </div>
        <Space className="monitoring-actions">
          {canExport && (
            <Button
              style={{ backgroundColor: 'var(--sf-success)', borderColor: 'var(--sf-success)', color: '#111' }}
              icon={<FileExcelOutlined />}
              onClick={handleExportExcel}
              disabled={loading || exporting !== null}
              loading={exporting === 'excel'}
            >
              Export Excel
            </Button>
          )}
          {canExport && (
            <Button
              style={{ backgroundColor: 'var(--sf-danger)', borderColor: 'var(--sf-danger)', color: '#111' }}
              icon={<FilePdfOutlined />}
              onClick={handleExportPDF}
              disabled={loading || exporting !== null}
              loading={exporting === 'pdf'}
            >
              Export PDF
            </Button>
          )}
          <Button
            style={{ backgroundColor: '#6b7280', borderColor: '#6b7280', color: '#fff' }}
            icon={<PrinterOutlined />}
            onClick={handlePrint}
            disabled={loading || exporting !== null}
          >
            Cetak
          </Button>
        </Space>
      </div>

      <Table
        dataSource={households}
        columns={columns}
        rowKey="household_occupancy_id"
        loading={loading}
        pagination={{
          pageSize: 20,
          total,
          showTotal: () => `Total ${total} rumah`,
        }}
        scroll={{ x: 1200 }}
        locale={{ emptyText: <Empty description="Tidak ada rumah dalam monitoring" /> }}
      />
        </div>
      </div>
      {createPortal(
        <section className="monitoring-print-report" aria-label="Laporan Monitoring Kebersihan">
        <header className="monitoring-report-header">
          <h1>LAPORAN MONITORING KEBERSIHAN</h1>
          <p>{rtLabel}{rwLabel ? ` | ${rwLabel}` : ''}</p>
          <p className="monitoring-report-date">
            Dicetak: {printedAt.toLocaleString('id-ID')}
          </p>
        </header>
        <p className="monitoring-report-summary">
          Total rumah yang memiliki tunggakan: <strong>{total}</strong>
        </p>
        <table className="monitoring-report-table">
          <thead>
            <tr>
              <th scope="col">No.</th>
              <th scope="col">No. Rumah</th>
              <th scope="col">Catatan Operasional</th>
            </tr>
          </thead>
          <tbody>
            {households.length === 0 ? (
              <tr>
                <td colSpan={3} className="monitoring-report-empty">
                  Tidak ada rumah dalam monitoring.
                </td>
              </tr>
            ) : households.map((household, index) => (
              <tr key={household.household_occupancy_id}>
                <td className="monitoring-report-number">{index + 1}</td>
                <td>{household.house_number}</td>
                <td>{household.note || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <footer className="monitoring-report-footer">
          Dokumen laporan Monitoring Kebersihan
        </footer>
        </section>,
        document.body,
      )}

      {/* Note Modal */}
      <Modal
        title={`Catatan Lapangan - ${editingHousehold?.house_number || ''}`}
        open={noteModalOpen}
        onCancel={() => setNoteModalOpen(false)}
        confirmLoading={false}
        width={500}
      >
        <TextArea
          value={noteText}
          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setNoteText(e.target.value)}
          rows={4}
          placeholder="Masukkan catatan lapangan..."
          autoSize={{ minRows: 4, maxRows: 6 }}
        />
        <div style={{ marginTop: 16, textAlign: 'right' }}>
          <Button onClick={() => setNoteModalOpen(false)}>Batal</Button>
          <Button
            type="primary"
            onClick={() => {
              if (!editingHousehold) return
              if (noteText.trim() === '') return
              if (editingHousehold.note === noteText) {
                setNoteModalOpen(false)
                return
              }
              handleUpdateNote(editingHousehold.household_occupancy_id, noteText)
            }}
          >
            Simpan
          </Button>
        </div>
      </Modal>
    </>
  )
}
