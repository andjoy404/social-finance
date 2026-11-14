import { useState, useRef } from 'react'
import {
  Modal,
  Button,
  Space,
  Alert,
  Card,
  Row,
  Col,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  UploadOutlined,
  FileExcelOutlined,
  DownloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  ArrowLeftOutlined,
} from '@ant-design/icons'
import {
  apiPreviewSecuritySchedulesXLSX,
  apiCommitSecuritySchedulesXLSX,
  type ScheduleExcelRow,
  type ScheduleImportPreviewResponse,
} from './securityApi'

const { Text, Paragraph } = Typography

export interface ScheduleImportModalProps {
  open: boolean
  onClose: () => void
  onSuccess: () => void
}

export function ScheduleImportModal({ open, onClose, onSuccess }: ScheduleImportModalProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [loadingPreview, setLoadingPreview] = useState(false)
  const [previewData, setPreviewData] = useState<ScheduleImportPreviewResponse | null>(null)
  const [committing, setCommitting] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleReset = () => {
    setSelectedFile(null)
    setPreviewData(null)
    setErrorMsg(null)
    setLoadingPreview(false)
    setCommitting(false)
    setConfirmOpen(false)
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleModalClose = () => {
    handleReset()
    onClose()
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.name.endsWith('.xlsx')) {
      message.error('File harus berformat Excel (.xlsx)')
      return
    }
    setSelectedFile(file)
    setErrorMsg(null)
  }

  const handleProcessPreview = async () => {
    if (!selectedFile) return
    setLoadingPreview(true)
    setErrorMsg(null)
    try {
      const res = await apiPreviewSecuritySchedulesXLSX(selectedFile)
      setPreviewData(res)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Gagal memproses pratinjau file'
      setErrorMsg(msg)
    } finally {
      setLoadingPreview(false)
    }
  }

  const handleDownloadValidationReport = () => {
    if (!previewData) return
    const rejectedRows = previewData.rows.filter((r) => !r.is_valid)
    if (rejectedRows.length === 0) {
      message.info('Tidak ada baris yang ditolak pada file ini.')
      return
    }

    const csvLines = [
      'Baris,ID Jadwal,Nama Petugas,Jenis Shift,Tanggal Mulai,Tanggal Selesai,Alasan Penolakan',
      ...rejectedRows.map((r) => {
        const errs = r.errors.join('; ').replace(/"/g, '""')
        return `${r.row_number},"${r.schedule_id}","${r.officer_name}","${r.shift_name}","${r.start_date}","${r.end_date}","${errs}"`
      }),
    ]

    const blob = new Blob([csvLines.join('\n')], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `laporan_penolakan_jadwal_${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  const handleCommitValidRows = async () => {
    if (!previewData || previewData.valid_rows === 0) return
    const validRows = previewData.rows.filter((r) => r.is_valid)
    setCommitting(true)
    try {
      const res = await apiCommitSecuritySchedulesXLSX(validRows)
      message.success(res.message || 'Jadwal berhasil disimpan!')
      handleReset()
      onSuccess()
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Gagal menyimpan jadwal'
      message.error(msg)
    } finally {
      setCommitting(false)
      setConfirmOpen(false)
    }
  }

  const columns: ColumnsType<ScheduleExcelRow> = [
    {
      title: 'Baris',
      dataIndex: 'row_number',
      key: 'row_number',
      width: 70,
      align: 'center',
    },
    {
      title: 'Status',
      key: 'status',
      width: 100,
      align: 'center',
      render: (_, record) =>
        record.is_valid ? (
          <Tag color="success" icon={<CheckCircleOutlined />}>
            Valid
          </Tag>
        ) : (
          <Tag color="error" icon={<CloseCircleOutlined />}>
            Ditolak
          </Tag>
        ),
    },
    {
      title: 'Aksi',
      dataIndex: 'action',
      key: 'action',
      width: 110,
      align: 'center',
      render: (action: string) =>
        action === 'update' ? (
          <Tag color="purple">Ubah</Tag>
        ) : (
          <Tag color="blue">Tambah Baru</Tag>
        ),
    },
    {
      title: 'Nama Petugas',
      dataIndex: 'officer_name',
      key: 'officer_name',
      width: 160,
      render: (text) => text || <Text type="secondary">-</Text>,
    },
    {
      title: 'Jenis Shift',
      dataIndex: 'shift_name',
      key: 'shift_name',
      width: 140,
      render: (text) => text || <Text type="secondary">-</Text>,
    },
    {
      title: 'Periode Tugas',
      key: 'period',
      width: 190,
      render: (_, record) => (
        <span>
          {record.start_date || '-'} <Text type="secondary">s/d</Text> {record.end_date || '-'}
        </span>
      ),
    },
    {
      title: 'Informasi Jadwal',
      dataIndex: 'notes',
      key: 'notes',
      width: 160,
      ellipsis: true,
      render: (text) => text || <Text type="secondary">-</Text>,
    },
    {
      title: 'Keterangan / Alasan Penolakan',
      key: 'errors',
      render: (_, record) => {
        if (record.is_valid) {
          return <Text type="secondary">Siap disimpan</Text>
        }
        return (
          <ul style={{ margin: 0, paddingLeft: 16, color: '#dc2626' }}>
            {record.errors.map((e, idx) => (
              <li key={idx}>{e}</li>
            ))}
          </ul>
        )
      },
    },
  ]

  return (
    <Modal
      open={open}
      title={
        <Space>
          <FileExcelOutlined style={{ color: '#16a34a', fontSize: 20 }} />
          <span>Import Jadwal Petugas Keamanan (Excel .xlsx)</span>
        </Space>
      }
      onCancel={handleModalClose}
      width={1000}
      footer={
        !previewData ? (
          <Space>
            <Button onClick={handleModalClose}>Batal</Button>
            <Button
              type="primary"
              onClick={() => void handleProcessPreview()}
              disabled={!selectedFile}
              loading={loadingPreview}
            >
              Proses Pratinjau
            </Button>
          </Space>
        ) : (
          <Space wrap style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Button icon={<ArrowLeftOutlined />} onClick={() => setPreviewData(null)} disabled={committing}>
                Pilih File Lain
              </Button>
              {previewData.rejected_rows > 0 && (
                <Button icon={<DownloadOutlined />} onClick={handleDownloadValidationReport} disabled={committing}>
                  Download Laporan Validasi ({previewData.rejected_rows} Baris)
                </Button>
              )}
            </Space>
            <Space>
              <Button onClick={handleModalClose} disabled={committing}>
                Batal
              </Button>
              <Button
                type="primary"
                onClick={() => setConfirmOpen(true)}
                loading={committing}
                disabled={previewData.valid_rows === 0}
              >
                Simpan {previewData.valid_rows} Jadwal Valid
              </Button>
            </Space>
          </Space>
        )
      }
    >
      {errorMsg && (
        <Alert
          type="error"
          showIcon
          message="Terjadi Kesalahan"
          description={errorMsg}
          style={{ marginBottom: 16 }}
        />
      )}

      {!previewData ? (
        <div style={{ padding: '16px 0' }}>
          <Alert
            type="info"
            showIcon
            message="Petunjuk Format File Excel"
            description={
              <div>
                <p style={{ margin: '4px 0' }}>
                  Gunakan format file Excel (<strong>.xlsx</strong>) yang sesuai template atau hasil Export Jadwal.
                </p>
                <ul style={{ margin: 0, paddingLeft: 18 }}>
                  <li>Satu file dapat berisi penambahan jadwal baru dan pembaruan jadwal lama sekaligus.</li>
                  <li>
                    Kosongkan kolom <strong>ID Jadwal</strong> untuk menambah jadwal baru.
                  </li>
                  <li>
                    Biarkan kolom <strong>ID Jadwal</strong> tetap terisi untuk memperbarui jadwal yang sudah ada.
                  </li>
                  <li>
                    Format tanggal harus <strong>YYYY-MM-DD</strong> (contoh: 2026-11-15).
                  </li>
                </ul>
              </div>
            }
            style={{ marginBottom: 20 }}
          />

          <div
            style={{
              border: '2px dashed #d1d5db',
              borderRadius: 8,
              padding: '36px 20px',
              textAlign: 'center',
              backgroundColor: selectedFile ? '#f0fdf4' : '#fafafa',
              cursor: 'pointer',
            }}
            onClick={() => fileInputRef.current?.click()}
          >
            <input
              type="file"
              ref={fileInputRef}
              onChange={handleFileChange}
              accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              style={{ display: 'none' }}
              aria-label="Upload file Excel"
            />
            <UploadOutlined style={{ fontSize: 40, color: selectedFile ? '#16a34a' : '#6b7280', marginBottom: 12 }} />
            {selectedFile ? (
              <div>
                <Paragraph strong style={{ fontSize: 16, color: '#16a34a', margin: '4px 0' }}>
                  {selectedFile.name}
                </Paragraph>
                <Text type="secondary">
                  Ukuran: {(selectedFile.size / 1024).toFixed(1)} KB — Klik untuk mengganti file
                </Text>
              </div>
            ) : (
              <div>
                <Paragraph strong style={{ fontSize: 16, margin: '4px 0' }}>
                  Klik di sini untuk memilih file Excel (.xlsx)
                </Paragraph>
                <Text type="secondary">Pastikan file berisi lembar kerja Jadwal dengan kolom yang sesuai</Text>
              </div>
            )}
          </div>
        </div>
      ) : (
        <div style={{ padding: '8px 0' }}>
          {/* Summary Cards */}
          <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
            <Col xs={12} sm={6}>
              <Card size="small">
                <Statistic title="Total Baris" value={previewData.total_rows} />
              </Card>
            </Col>
            <Col xs={12} sm={6}>
              <Card size="small">
                <Statistic
                  title="Baris Valid"
                  value={previewData.valid_rows}
                  valueStyle={{ color: '#16a34a' }}
                  prefix={<CheckCircleOutlined />}
                />
              </Card>
            </Col>
            <Col xs={12} sm={6}>
              <Card size="small">
                <Statistic
                  title="Baris Ditolak"
                  value={previewData.rejected_rows}
                  valueStyle={{ color: previewData.rejected_rows > 0 ? '#dc2626' : undefined }}
                  prefix={previewData.rejected_rows > 0 ? <CloseCircleOutlined /> : undefined}
                />
              </Card>
            </Col>
            <Col xs={12} sm={6}>
              <Card size="small">
                <Statistic title="Baris Kosong" value={previewData.empty_rows} />
              </Card>
            </Col>
          </Row>

          {/* Warning / Notification Alert */}
          {previewData.rejected_rows > 0 && (
            <Alert
              type="warning"
              showIcon
              message={`Perhatian: ${previewData.rejected_rows} baris jadwal ditolak`}
              description="Baris yang ditolak memiliki masalah validasi (bentrok waktu, format salah, petugas tidak valid, dll) dan akan diabaikan saat penyimpanan. Anda dapat mengunduh laporan penolakan di bawah."
              style={{ marginBottom: 16 }}
            />
          )}

          {previewData.valid_rows === 0 && (
            <Alert
              type="error"
              showIcon
              message="Tidak ada data valid untuk disimpan"
              description="Seluruh baris jadwal pada file ditolak. Harap perbaiki kesalahan dan unggah ulang file."
              style={{ marginBottom: 16 }}
            />
          )}

          {/* Table Preview */}
          <Table
            rowKey="row_number"
            dataSource={previewData.rows}
            columns={columns}
            pagination={{ pageSize: 10 }}
            scroll={{ x: 900 }}
            size="small"
          />
        </div>
      )}

      {/* Confirmation Modal */}
      <Modal
        open={confirmOpen}
        title={
          <Space>
            <ExclamationCircleOutlined style={{ color: '#d97706' }} />
            <span>Konfirmasi Simpan Jadwal</span>
          </Space>
        }
        onCancel={() => setConfirmOpen(false)}
        onOk={() => void handleCommitValidRows()}
        confirmLoading={committing}
        okText="Ya, Simpan Jadwal"
        cancelText="Batal"
      >
        <p>
          Anda akan menyimpan <strong>{previewData?.valid_rows ?? 0} baris jadwal yang valid</strong> ke database.
        </p>
        {(previewData?.rejected_rows ?? 0) > 0 && (
          <p style={{ color: '#d97706' }}>
            Catatan: <strong>{previewData?.rejected_rows} baris yang ditolak</strong> tidak akan dimasukkan.
          </p>
        )}
        <p>Apakah Anda yakin ingin melanjutkan?</p>
      </Modal>
    </Modal>
  )
}

