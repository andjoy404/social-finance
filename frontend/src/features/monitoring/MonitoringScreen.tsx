import { useState, useCallback, useEffect, useRef } from 'react'
import {
  Button,
  message,
} from 'antd'
import { Modal } from '@/components/Modal'
import { isSnapshotDirty, useUnsavedChangesGuard } from '@/hooks/useUnsavedChangesGuard'
import {
  FileExcelOutlined,
  FilePdfOutlined,
  EditOutlined,
  PrinterOutlined,
  SafetyCertificateOutlined,
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
import { Pagination } from '@/components/Pagination'
import { createPortal } from 'react-dom'
import './MonitoringScreen.css'

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
  const [page, setPage] = useState(1)
  const pageSize = 20
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const [noteModalOpen, setNoteModalOpen] = useState(false)
  const [editingHousehold, setEditingHousehold] = useState<ApiMonitoringHousehold | null>(null)
  const [noteText, setNoteText] = useState('')
  const noteInitialRef = useRef('')
  const [savingNote, setSavingNote] = useState(false)
  const [printedAt, setPrintedAt] = useState(() => new Date())

  const noteGuard = useUnsavedChangesGuard({
    active: noteModalOpen,
    isDirty: noteModalOpen && isSnapshotDirty(noteText, noteInitialRef.current),
    isSaving: savingNote,
  })
  const closeNoteModal = () => {
    void noteGuard.guardedClose(() => {
      setNoteModalOpen(false)
      setEditingHousehold(null)
    })
  }

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

  const handlePageChange = (p: number) => {
    setPage(p)
    // Server-side pagination handled by API via offset
  }

  const handleUpdateNote = async (householdOccupancyId: string, note: string) => {
    setSavingNote(true)
    try {
      await apiUpdateMonitoringNote(token, rtId, householdOccupancyId, note)
      message.success('Catatan diperbarui')
      noteGuard.exitAfterSave(() => {
        setNoteModalOpen(false)
        setEditingHousehold(null)
      })
      await fetchHouseholds()
    } catch (err: any) {
      message.error(err?.message || 'Gagal memperbarui catatan')
    } finally {
      setSavingNote(false)
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

  const displayedHouseholds = households.slice((page - 1) * pageSize, page * pageSize)

  return (
    <>
      {/* Page Header */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <div className="sf-page-header-icon">
            <SafetyCertificateOutlined />
          </div>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Sampah</div>
            <div className="sf-page-header-subtitle">
              {rtLabel}{rwLabel ? ` | ${rwLabel}` : ''}
            </div>
          </div>
        </div>
        <div className="sf-page-header-actions">
          {canExport && (
            <Button
              type="primary"
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
              type="primary"
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
            type="primary"
            style={{ backgroundColor: '#6b7280', borderColor: '#6b7280', color: '#fff' }}
            icon={<PrinterOutlined />}
            onClick={handlePrint}
            disabled={loading || exporting !== null}
          >
            Cetak
          </Button>
        </div>
      </div>

      {/* Content Surface */}
      <div className="sf-content-surface sf-content-surface-padding">
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            <span className="sf-list-toolbar-label">Daftar pemantauan sampah</span>
          </div>
          <div className="sf-list-toolbar-right" />
        </div>

        <div className="table-wrapper">
          <table className="sf-warga-table">
            <thead>
              <tr>
                <th className="col-index" style={{ width: 60, minWidth: 60 }}>No.</th>
                <th className="col-house-number" style={{ width: 120, minWidth: 120 }}>No. Rumah</th>
                <th className="col-note" style={{ width: 200, minWidth: 200 }}>Catatan Operasional</th>
                <th className="sf-table-action" style={{ width: 100, minWidth: 100 }}>Aksi</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={4} style={{ textAlign: 'center', padding: 40, color: 'var(--sf-text-muted)' }}>
                    Memuat data...
                  </td>
                </tr>
              ) : displayedHouseholds.length === 0 ? (
                <tr>
                  <td colSpan={4} style={{ textAlign: 'center', padding: 40, color: 'var(--sf-text-muted)' }}>
                    Tidak ada rumah dalam monitoring
                  </td>
                </tr>
              ) : (
                displayedHouseholds.map((household, index) => (
                  <tr key={household.household_occupancy_id}>
                    <td>{(page - 1) * pageSize + index + 1}</td>
                    <td>{household.house_number}</td>
                    <td
                      onClick={() => {
                        const initial = household.note || ''
                        noteInitialRef.current = initial
                        setNoteText(initial)
                        setEditingHousehold(household)
                        setNoteModalOpen(true)
                      }}
                      style={{ cursor: 'pointer', color: 'var(--sf-accent)', textDecoration: 'underline' }}
                    >
                      {household.note || '—'}
                    </td>
                    <td>
                      <Button
                        type="text"
                        size="small"
                        icon={<EditOutlined />}
                        onClick={() => {
                          const initial = household.note || ''
                          noteInitialRef.current = initial
                          setNoteText(initial)
                          setEditingHousehold(household)
                          setNoteModalOpen(true)
                        }}
                      >
                        Edit Catatan
                      </Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {!loading && total > 0 && (
          <div className="sf-pagination-wrapper">
            <Pagination
              page={page}
              pageSize={pageSize}
              total={total}
              totalPages={totalPages}
              onPageChange={handlePageChange}
              onPageSizeChange={() => {}}
            />
          </div>
        )}
      </div>

      {/* Print Portal */}
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
      {noteModalOpen && (
        <Modal open onClose={closeNoteModal} width={520}>
            <div className="sf-modal-header">
              <div className="sf-modal-header-copy">
                <div className="sf-modal-title">Catatan Lapangan</div>
                <div className="sf-modal-subtitle">No. Rumah {editingHousehold?.house_number || ''}</div>
              </div>
              <button
                type="button"
                className="sf-modal-close"
                onClick={closeNoteModal}
                disabled={savingNote}
                aria-label="Tutup"
              >
                ×
              </button>
            </div>
            <div className="sf-modal-body">
              <textarea
                value={noteText}
                onChange={(e) => setNoteText(e.target.value)}
                rows={4}
                placeholder="Masukkan catatan lapangan..."
                disabled={savingNote}
                style={{ width: '100%', padding: 10, border: '1px solid var(--sf-border)', borderRadius: 8, fontFamily: 'inherit', fontSize: 14 }}
              />
              <div className="sf-modal-footer">
                <button
                  type="button"
                  className="sf-btn sf-btn-ghost"
                  onClick={closeNoteModal}
                  disabled={savingNote}
                >
                  Batal
                </button>
                <button
                  type="button"
                  className="sf-btn sf-btn-primary"
                  disabled={savingNote || noteText.trim() === '' || editingHousehold?.note === noteText}
                  onClick={() => {
                    if (!editingHousehold) return
                    if (noteText.trim() === '') return
                    if (editingHousehold.note === noteText) {
                      noteGuard.exitAfterSave(() => setNoteModalOpen(false))
                      return
                    }
                    handleUpdateNote(editingHousehold.household_occupancy_id, noteText)
                  }}
                >
                  {savingNote ? 'Menyimpan...' : 'Simpan'}
                </button>
              </div>
            </div>
        </Modal>
      )}
      {noteGuard.confirmModal}
    </>
  )
}
