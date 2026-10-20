import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom'
import { WargaImportModal } from './WargaImportModal'
import { persistSessionPair } from '@/app/api'
import * as apiModule from '@/app/api'

describe('WargaImportModal Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    persistSessionPair({ accessToken: 'valid-test-token', refreshToken: 'valid-refresh-token' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('does not render dialog when open is false', () => {
    render(<WargaImportModal open={false} onClose={() => {}} />)
    expect(screen.queryByText('Import Data Warga')).not.toBeInTheDocument()
  })

  it('renders initial upload screen when open is true', () => {
    render(<WargaImportModal open={true} onClose={() => {}} />)

    expect(screen.getByText('Import Data Warga')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Unduh Template/i })).toBeInTheDocument()
    expect(screen.getByText(/Pilih file Excel \(\.xlsx\) atau seret ke sini/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Pratinjau Data/i })).toBeDisabled()
  })

  it('handles template download successfully', async () => {
    const mockBlob = new Blob(['mock excel bytes'], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    const downloadSpy = vi.spyOn(apiModule, 'apiDownloadWargaTemplate').mockResolvedValue(mockBlob)

    const createObjectURLMock = vi.fn().mockReturnValue('blob:http://localhost/test-blob')
    const revokeObjectURLMock = vi.fn()
    window.URL.createObjectURL = createObjectURLMock
    window.URL.revokeObjectURL = revokeObjectURLMock

    render(<WargaImportModal open={true} onClose={() => {}} />)

    const downloadBtn = screen.getByRole('button', { name: /Unduh Template/i })
    fireEvent.click(downloadBtn)

    await waitFor(() => {
      expect(downloadSpy).toHaveBeenCalledWith('valid-test-token')
      expect(createObjectURLMock).toHaveBeenCalledWith(mockBlob)
    })
  })

  it('shows error if selected file is not an .xlsx file', () => {
    render(<WargaImportModal open={true} onClose={() => {}} />)

    const file = new File(['dummy'], 'residents.csv', { type: 'text/csv' })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(input).toBeInTheDocument()

    fireEvent.change(input, { target: { files: [file] } })

    expect(screen.getByText(/Format file tidak didukung/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Pratinjau Data/i })).toBeDisabled()
  })

  it('accepts valid .xlsx file and enables preview button', () => {
    render(<WargaImportModal open={true} onClose={() => {}} />)

    const file = new File(['valid content'], 'warga_data.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement

    fireEvent.change(input, { target: { files: [file] } })

    expect(screen.queryByText(/Format file tidak didukung/i)).not.toBeInTheDocument()
    expect(screen.getByText('warga_data.xlsx')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Pratinjau Data/i })).not.toBeDisabled()
  })

  it('displays validation errors if preview contains errors', async () => {
    const previewSpy = vi.spyOn(apiModule, 'apiPreviewWargaImportXLSX').mockResolvedValue({
      total_rows: 2,
      valid_rows: 1,
      invalid_rows: 1,
      errors: [
        {
          row: 3,
          field: 'nik',
          code: 'invalid_nik',
          message: 'NIK must be exactly 16 digits',
        },
      ],
      preview_data: [],
    })

    render(<WargaImportModal open={true} onClose={() => {}} />)

    const file = new File(['dummy'], 'test.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })

    const previewBtn = screen.getByRole('button', { name: /Pratinjau Data/i })
    fireEvent.click(previewBtn)

    await waitFor(() => {
      expect(previewSpy).toHaveBeenCalled()
      expect(screen.getByText('Total Baris')).toBeInTheDocument()
      expect(screen.getByText(/Ditemukan 1 masalah validasi/i)).toBeInTheDocument()
      expect(screen.getByText('NIK must be exactly 16 digits')).toBeInTheDocument()
    })

    // "Impor Sekarang" button must NOT be present when invalid_rows > 0
    expect(screen.queryByRole('button', { name: /Impor Sekarang/i })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Ganti File/i })).toBeInTheDocument()
  })

  it('completes full flow from valid preview to successful commit', async () => {
    const previewSpy = vi.spyOn(apiModule, 'apiPreviewWargaImportXLSX').mockResolvedValue({
      total_rows: 2,
      valid_rows: 2,
      invalid_rows: 0,
      errors: [],
      preview_data: [
        {
          row_number: 2,
          house_number: 'A-01',
          head_name: 'Budi Hartono',
          occupancy_status: 'OWNER',
          address: 'Jl. Merak No. 1',
          resident_name: 'Budi Hartono',
          nik: '3201011111110001',
          phone: '08123456789',
          email: 'budi@test.com',
        },
      ],
    })

    const commitSpy = vi.spyOn(apiModule, 'apiCommitWargaImportXLSX').mockResolvedValue({
      status: 'success',
      imported_households: 1,
      imported_residents: 1,
    })

    const onSuccess = vi.fn()
    const onClose = vi.fn()

    render(<WargaImportModal open={true} onClose={onClose} onSuccess={onSuccess} />)

    const file = new File(['valid'], 'warga.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })

    // Step 1 -> Click Preview
    fireEvent.click(screen.getByRole('button', { name: /Pratinjau Data/i }))

    await waitFor(() => {
      expect(previewSpy).toHaveBeenCalled()
      expect(screen.getByText(/Semua baris valid dan siap diimpor/i)).toBeInTheDocument()
      expect(screen.getAllByText('Budi Hartono').length).toBeGreaterThan(0)
      expect(screen.getByRole('button', { name: /Impor Sekarang/i })).toBeInTheDocument()
    })

    // Step 2 -> Click Commit
    fireEvent.click(screen.getByRole('button', { name: /Impor Sekarang/i }))

    await waitFor(() => {
      expect(commitSpy).toHaveBeenCalled()
      expect(screen.getByText('Import Data Berhasil!')).toBeInTheDocument()
      expect(screen.getByText(/1 Rumah Tangga/i)).toBeInTheDocument()
      expect(screen.getByText(/1 Warga/i)).toBeInTheDocument()
    })

    // Step 3 -> Click Selesai
    fireEvent.click(screen.getByRole('button', { name: /Selesai/i }))

    expect(onSuccess).toHaveBeenCalledTimes(1)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('allows returning to upload step via Ganti File button', async () => {
    vi.spyOn(apiModule, 'apiPreviewWargaImportXLSX').mockResolvedValue({
      total_rows: 1,
      valid_rows: 0,
      invalid_rows: 1,
      errors: [{ row: 2, field: 'resident_name', code: 'missing', message: 'required' }],
      preview_data: [],
    })

    render(<WargaImportModal open={true} onClose={() => {}} />)

    const file = new File(['content'], 'test.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })

    fireEvent.click(screen.getByRole('button', { name: /Pratinjau Data/i }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Ganti File/i })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: /Ganti File/i }))

    expect(screen.getByText(/Template Format Excel \(\.xlsx\)/i)).toBeInTheDocument()
  })
})
