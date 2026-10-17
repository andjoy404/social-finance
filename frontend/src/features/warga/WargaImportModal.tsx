import { useState, useRef } from 'react'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'
import {
  getSessionPair,
  apiDownloadWargaTemplate,
  apiPreviewWargaImportXLSX,
  apiCommitWargaImportXLSX,
  type ApiWargaImportPreviewResponse,
  type ApiWargaImportCommitResponse,
  ApiError,
} from '@/app/api'
import {
  FileExcelOutlined,
  DownloadOutlined,
  UploadOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  ArrowLeftOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons'

export interface WargaImportModalProps {
  open: boolean
  onClose: () => void
  onSuccess?: () => void
  targetRtId?: string
}

type Step = 'upload' | 'preview' | 'success'

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

export function WargaImportModal({
  open,
  onClose,
  onSuccess,
  targetRtId,
}: WargaImportModalProps) {
  const [step, setStep] = useState<Step>('upload')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [downloadingTemplate, setDownloadingTemplate] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [previewResult, setPreviewResult] = useState<ApiWargaImportPreviewResponse | null>(null)
  const [commitResult, setCommitResult] = useState<ApiWargaImportCommitResponse | null>(null)
  const [isDragOver, setIsDragOver] = useState(false)

  const fileInputRef = useRef<HTMLInputElement>(null)

  const resetState = () => {
    setStep('upload')
    setSelectedFile(null)
    setPreviewResult(null)
    setCommitResult(null)
    setError(null)
    setIsDragOver(false)
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleClose = () => {
    if (loading) return
    resetState()
    onClose()
  }

  const handleFinish = () => {
    resetState()
    onClose()
    onSuccess?.()
  }

  const handleDownloadTemplate = async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }

    setDownloadingTemplate(true)
    setError(null)
    try {
      const blob = await apiDownloadWargaTemplate(pair.accessToken)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'warga_template.xlsx'
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.URL.revokeObjectURL(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal mengunduh template Excel')
    } finally {
      setDownloadingTemplate(false)
    }
  }

  const validateAndSetFile = (f: File) => {
    setError(null)
    if (!f.name.toLowerCase().endsWith('.xlsx')) {
      setError('Format file tidak didukung. Harap unggah file spreadsheet Excel (.xlsx)')
      return
    }
    // 20 MB limit
    if (f.size > 20 * 1024 * 1024) {
      setError('Ukuran file melebihi batas maksimal 20 MB.')
      return
    }
    setSelectedFile(f)
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (f) {
      validateAndSetFile(f)
    }
  }

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragOver(false)
    const f = e.dataTransfer.files?.[0]
    if (f) {
      validateAndSetFile(f)
    }
  }

  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setIsDragOver(true)
  }

  const handleDragLeave = () => {
    setIsDragOver(false)
  }

  const handlePreview = async () => {
    if (!selectedFile) return
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }

    setLoading(true)
    setError(null)
    try {
      const res = await apiPreviewWargaImportXLSX(pair.accessToken, selectedFile, targetRtId)
      setPreviewResult(res)
      setStep('preview')
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Gagal memproses pratinjau file.')
      }
    } finally {
      setLoading(false)
    }
  }

  const handleCommit = async () => {
    if (!selectedFile) return
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }

    setLoading(true)
    setError(null)
    try {
      const res = await apiCommitWargaImportXLSX(pair.accessToken, selectedFile, targetRtId)
      setCommitResult(res)
      setStep('success')
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Gagal menyimpan data import warga.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal open={open} onClose={handleClose} width={700}>
      <ModalHeader
        title="Import Data Warga"
        subtitle="Unggah spreadsheet Excel (.xlsx) untuk menambahkan data warga dan rumah tangga sekaligus"
        onClose={handleClose}
      />

      <ModalBody>
        {error && (
          <div
            className="sf-error-banner"
            role="alert"
            style={{ marginBottom: 16 }}
          >
            {error}
          </div>
        )}

        {/* STEP 1: Upload & Template */}
        {step === 'upload' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
            {/* Template Info Card */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '14px 18px',
                borderRadius: '8px',
                background: 'var(--sf-bg-surface-elevated, var(--sf-bg))',
                border: '1px solid var(--sf-border)',
                gap: 16,
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <span
                  style={{
                    fontSize: 24,
                    color: 'color-mix(in srgb, #34c759 85%, var(--sf-text))',
                    display: 'flex',
                    alignItems: 'center',
                  }}
                >
                  <FileExcelOutlined />
                </span>
                <div>
                  <div style={{ fontWeight: 600, fontSize: 13, color: 'var(--sf-text)' }}>
                    Template Format Excel (.xlsx)
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)', marginTop: 2 }}>
                    Gunakan template standar agar kolom sesuai dengan format sistem
                  </div>
                </div>
              </div>

              <button
                type="button"
                className="sf-btn-secondary"
                onClick={handleDownloadTemplate}
                disabled={downloadingTemplate}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 6,
                  whiteSpace: 'nowrap',
                  padding: '6px 14px',
                  fontSize: 12,
                }}
              >
                <DownloadOutlined />
                {downloadingTemplate ? 'Mengunduh...' : 'Unduh Template'}
              </button>
            </div>

            {/* Instruction Summary */}
            <div
              style={{
                fontSize: 12,
                lineHeight: 1.5,
                color: 'var(--sf-text-muted)',
                background: 'var(--sf-bg)',
                padding: '12px 16px',
                borderRadius: '8px',
                border: '1px solid var(--sf-border)',
              }}
            >
              <div style={{ fontWeight: 600, color: 'var(--sf-text)', marginBottom: 4, display: 'flex', alignItems: 'center', gap: 6 }}>
                <InfoCircleOutlined style={{ fontSize: 13, color: 'var(--sf-accent)' }} />
                Ketentuan Import Data:
              </div>
              <ul style={{ margin: 0, paddingLeft: 18, display: 'flex', flexDirection: 'column', gap: 3 }}>
                <li>
                  <strong>Kolom Wajib:</strong> <code>house_number</code>, <code>head_name</code>, <code>occupancy_status</code> (OWNER/TENANT), <code>resident_name</code>
                </li>
                <li>
                  <strong>Kolom Opsional:</strong> <code>address</code>, <code>nik</code> (16 digit), <code>phone</code>, <code>email</code>, <code>relationship_to_head</code>
                </li>
                <li>
                  Beberapa baris dengan <code>house_number</code> yang sama akan dikelompokkan ke dalam satu Rumah Tangga.
                </li>
              </ul>
            </div>

            {/* Drag and Drop Zone */}
            <div
              onDrop={handleDrop}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onClick={() => fileInputRef.current?.click()}
              style={{
                border: `2px dashed ${isDragOver ? 'var(--sf-accent)' : 'var(--sf-border)'}`,
                borderRadius: '10px',
                padding: '32px 20px',
                textAlign: 'center',
                cursor: 'pointer',
                background: isDragOver ? 'var(--sf-accent-soft, rgba(99, 102, 241, 0.05))' : 'transparent',
                transition: 'all 150ms ease',
              }}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".xlsx"
                style={{ display: 'none' }}
                onChange={handleFileChange}
              />
              <UploadOutlined
                style={{
                  fontSize: 36,
                  color: isDragOver ? 'var(--sf-accent)' : 'var(--sf-text-muted)',
                  marginBottom: 10,
                }}
              />
              <div style={{ fontWeight: 600, fontSize: 14, color: 'var(--sf-text)', marginBottom: 4 }}>
                {selectedFile ? selectedFile.name : 'Pilih file Excel (.xlsx) atau seret ke sini'}
              </div>
              <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                {selectedFile
                  ? `Ukuran: ${formatFileSize(selectedFile.size)} \u2014 Klik untuk ganti file`
                  : 'Maksimal ukuran file 20 MB'}
              </div>
            </div>
          </div>
        )}

        {/* STEP 2: Preview & Validation */}
        {step === 'preview' && previewResult && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
            {/* Summary Counters */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(3, 1fr)',
                gap: 12,
              }}
            >
              <div
                style={{
                  padding: '12px 16px',
                  borderRadius: '8px',
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  textAlign: 'center',
                }}
              >
                <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Total Baris
                </div>
                <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-text)', marginTop: 2 }}>
                  {previewResult.total_rows}
                </div>
              </div>

              <div
                style={{
                  padding: '12px 16px',
                  borderRadius: '8px',
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  textAlign: 'center',
                }}
              >
                <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Baris Valid
                </div>
                <div
                  style={{
                    fontSize: 22,
                    fontWeight: 700,
                    color: 'color-mix(in srgb, #34c759 85%, var(--sf-text))',
                    marginTop: 2,
                  }}
                >
                  {previewResult.valid_rows}
                </div>
              </div>

              <div
                style={{
                  padding: '12px 16px',
                  borderRadius: '8px',
                  border: '1px solid var(--sf-border)',
                  background: 'var(--sf-bg)',
                  textAlign: 'center',
                }}
              >
                <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Baris Bermasalah
                </div>
                <div
                  style={{
                    fontSize: 22,
                    fontWeight: 700,
                    color: previewResult.invalid_rows > 0 ? 'var(--sf-danger)' : 'var(--sf-text-muted)',
                    marginTop: 2,
                  }}
                >
                  {previewResult.invalid_rows}
                </div>
              </div>
            </div>

            {/* If there are validation errors */}
            {previewResult.errors && previewResult.errors.length > 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '10px 14px',
                    borderRadius: '6px',
                    background: 'color-mix(in srgb, #ff3b30 10%, var(--sf-bg))',
                    border: '1px solid color-mix(in srgb, #ff3b30 30%, var(--sf-border))',
                    fontSize: 12,
                    color: 'var(--sf-danger)',
                  }}
                >
                  <ExclamationCircleOutlined />
                  <span>
                    Ditemukan {previewResult.errors.length} masalah validasi. Harap perbaiki file Excel Anda sebelum melakukan import.
                  </span>
                </div>

                <div
                  style={{
                    maxHeight: 220,
                    overflowY: 'auto',
                    border: '1px solid var(--sf-border)',
                    borderRadius: '8px',
                  }}
                >
                  <table className="sf-table" style={{ width: '100%', fontSize: 12 }}>
                    <thead>
                      <tr>
                        <th style={{ width: 60, textAlign: 'center' }}>Baris</th>
                        <th style={{ width: 130 }}>Kolom</th>
                        <th>Keterangan Kesalahan</th>
                      </tr>
                    </thead>
                    <tbody>
                      {previewResult.errors.map((err, idx) => (
                        <tr key={idx}>
                          <td style={{ textAlign: 'center', fontFamily: 'monospace' }}>
                            {err.row}
                          </td>
                          <td style={{ fontFamily: 'monospace' }}>
                            {err.field}
                          </td>
                          <td style={{ color: 'var(--sf-danger)' }}>
                            {err.message}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : (
              /* Valid state: preview table of rows */
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '10px 14px',
                    borderRadius: '6px',
                    background: 'color-mix(in srgb, #34c759 10%, var(--sf-bg))',
                    border: '1px solid color-mix(in srgb, #34c759 30%, var(--sf-border))',
                    fontSize: 12,
                    color: 'color-mix(in srgb, #34c759 85%, var(--sf-text))',
                  }}
                >
                  <CheckCircleOutlined />
                  <span>Semua baris valid dan siap diimpor ke database!</span>
                </div>

                {previewResult.preview_data && previewResult.preview_data.length > 0 && (
                  <div
                    style={{
                      maxHeight: 220,
                      overflowY: 'auto',
                      border: '1px solid var(--sf-border)',
                      borderRadius: '8px',
                    }}
                  >
                    <table className="sf-table" style={{ width: '100%', fontSize: 12 }}>
                      <thead>
                        <tr>
                          <th style={{ width: 80 }}>No. Rumah</th>
                          <th>Kepala Keluarga</th>
                          <th>Status</th>
                          <th>Nama Warga</th>
                          <th>NIK</th>
                          <th>Hubungan</th>
                        </tr>
                      </thead>
                      <tbody>
                        {previewResult.preview_data.slice(0, 10).map((row, idx) => (
                          <tr key={idx}>
                            <td style={{ fontFamily: 'monospace' }}>{row.house_number}</td>
                            <td>{row.head_name}</td>
                            <td>
                              <Badge variant={row.occupancy_status === 'OWNER' ? 'violet' : 'blue'}>
                                {row.occupancy_status === 'OWNER' ? 'Pemilik' : 'Penyewa'}
                              </Badge>
                            </td>
                            <td style={{ fontWeight: 600 }}>{row.resident_name}</td>
                            <td style={{ fontFamily: 'monospace' }}>{row.nik || '\u2014'}</td>
                            <td>{row.relationship_to_head || 'Anggota'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                {previewResult.preview_data && previewResult.preview_data.length > 10 && (
                  <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textAlign: 'center' }}>
                    Menampilkan 10 dari {previewResult.preview_data.length} baris pratinjau
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* STEP 3: Success State */}
        {step === 'success' && (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '30px 20px',
              textAlign: 'center',
              gap: 12,
            }}
          >
            <CheckCircleOutlined
              style={{
                fontSize: 48,
                color: 'color-mix(in srgb, #34c759 85%, var(--sf-text))',
              }}
            />
            <div style={{ fontSize: 18, fontWeight: 700, color: 'var(--sf-text)' }}>
              Import Data Berhasil!
            </div>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', maxWidth: 420 }}>
              {commitResult ? (
                <>
                  Berhasil menambahkan{' '}
                  <strong style={{ color: 'var(--sf-text)' }}>
                    {commitResult.imported_households} Rumah Tangga
                  </strong>{' '}
                  dan{' '}
                  <strong style={{ color: 'var(--sf-text)' }}>
                    {commitResult.imported_residents} Warga
                  </strong>{' '}
                  ke dalam sistem.
                </>
              ) : (
                'Data warga berhasil diimpor ke sistem.'
              )}
            </div>
          </div>
        )}
      </ModalBody>

      <ModalFooter>
        {step === 'upload' && (
          <>
            <button
              type="button"
              className="sf-btn-secondary"
              onClick={handleClose}
              disabled={loading}
            >
              Batal
            </button>
            <button
              type="button"
              className="sf-btn-primary"
              onClick={handlePreview}
              disabled={!selectedFile || loading}
            >
              {loading ? 'Memvalidasi...' : 'Pratinjau Data'}
            </button>
          </>
        )}

        {step === 'preview' && (
          <>
            <button
              type="button"
              className="sf-btn-secondary"
              onClick={() => {
                setStep('upload')
                setError(null)
              }}
              disabled={loading}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}
            >
              <ArrowLeftOutlined />
              Ganti File
            </button>

            {previewResult && previewResult.invalid_rows === 0 && previewResult.valid_rows > 0 ? (
              <button
                type="button"
                className="sf-btn-primary"
                onClick={handleCommit}
                disabled={loading}
              >
                {loading ? 'Mengimpor Data...' : 'Impor Sekarang'}
              </button>
            ) : (
              <button
                type="button"
                className="sf-btn-secondary"
                onClick={handleClose}
                disabled={loading}
              >
                Tutup
              </button>
            )}
          </>
        )}

        {step === 'success' && (
          <button
            type="button"
            className="sf-btn-primary"
            onClick={handleFinish}
          >
            Selesai
          </button>
        )}
      </ModalFooter>
    </Modal>
  )
}
