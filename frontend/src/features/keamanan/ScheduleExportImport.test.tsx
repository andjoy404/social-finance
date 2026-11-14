import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ScheduleImportModal } from './ScheduleImportModal'
import * as securityApi from './securityApi'

vi.mock('./securityApi', async () => {
  const actual = await vi.importActual<typeof securityApi>('./securityApi')
  return {
    ...actual,
    apiPreviewSecuritySchedulesXLSX: vi.fn(),
    apiCommitSecuritySchedulesXLSX: vi.fn(),
    apiExportSecuritySchedulesXLSX: vi.fn(),
  }
})

describe('ScheduleImportModal', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
    vi.clearAllMocks()
  })

  it('renders upload instructions and file input initially', () => {
    render(<ScheduleImportModal open={true} onClose={vi.fn()} onSuccess={vi.fn()} />)
    expect(screen.getByText('Import Jadwal Petugas Keamanan (Excel .xlsx)')).toBeInTheDocument()
    expect(screen.getByText('Petunjuk Format File Excel')).toBeInTheDocument()
    expect(screen.getByText('Klik di sini untuk memilih file Excel (.xlsx)')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Proses Pratinjau' })).toBeDisabled()
  })

  it('handles preview parsing with valid and rejected rows', async () => {
    const mockPreviewResponse: securityApi.ScheduleImportPreviewResponse = {
      total_rows: 3,
      valid_rows: 2,
      rejected_rows: 1,
      empty_rows: 0,
      rows: [
        {
          row_number: 2,
          schedule_id: '',
          officer_name: 'Budi Santoso',
          officer_user_id: 'user-1',
          shift_name: 'Shift Pagi',
          shift_config_id: 'shift-1',
          start_date: '2026-11-15',
          end_date: '2026-11-16',
          notes: 'Pos Utama',
          action: 'create',
          is_valid: true,
          errors: [],
        },
        {
          row_number: 3,
          schedule_id: 'a0000000-0000-0000-0000-000000000001',
          officer_name: 'Agus Setiawan',
          officer_user_id: 'user-2',
          shift_name: 'Shift Malam',
          shift_config_id: 'shift-2',
          start_date: '2026-11-17',
          end_date: '2026-11-18',
          notes: 'Patroli',
          action: 'update',
          is_valid: true,
          errors: [],
        },
        {
          row_number: 4,
          schedule_id: '',
          officer_name: 'Petugas Gaib',
          officer_user_id: '',
          shift_name: 'Shift Siang',
          shift_config_id: 'shift-3',
          start_date: '2026-11-19',
          end_date: '2026-11-20',
          notes: '',
          action: 'create',
          is_valid: false,
          errors: ['Petugas "Petugas Gaib" tidak ditemukan dalam data RT ini'],
        },
      ],
    }

    vi.mocked(securityApi.apiPreviewSecuritySchedulesXLSX).mockResolvedValueOnce(mockPreviewResponse)

    render(<ScheduleImportModal open={true} onClose={vi.fn()} onSuccess={vi.fn()} />)

    const fileInput = screen.getByLabelText('Upload file Excel') as HTMLInputElement
    const file = new File(['mock content'], 'jadwal_test.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })

    fireEvent.change(fileInput, { target: { files: [file] } })

    const previewBtn = screen.getByRole('button', { name: 'Proses Pratinjau' })
    expect(previewBtn).not.toBeDisabled()
    fireEvent.click(previewBtn)

    await waitFor(() => {
      expect(screen.getByText('Budi Santoso')).toBeInTheDocument()
      expect(screen.getByText('Agus Setiawan')).toBeInTheDocument()
      expect(screen.getByText('Petugas Gaib')).toBeInTheDocument()
      expect(screen.getByText(/Petugas "Petugas Gaib" tidak ditemukan/)).toBeInTheDocument()
    })

    // Verify statistics cards
    expect(screen.getByText('Total Baris')).toBeInTheDocument()
    expect(screen.getByText('Baris Valid')).toBeInTheDocument()
    expect(screen.getByText('Baris Ditolak')).toBeInTheDocument()

    // Verify commit button enabled with valid row count
    expect(screen.getByRole('button', { name: 'Simpan 2 Jadwal Valid' })).toBeInTheDocument()
  })

  it('commits valid rows upon confirmation', async () => {
    const mockPreviewResponse: securityApi.ScheduleImportPreviewResponse = {
      total_rows: 1,
      valid_rows: 1,
      rejected_rows: 0,
      empty_rows: 0,
      rows: [
        {
          row_number: 2,
          schedule_id: '',
          officer_name: 'Budi Santoso',
          officer_user_id: 'user-1',
          shift_name: 'Shift Pagi',
          shift_config_id: 'shift-1',
          start_date: '2026-11-15',
          end_date: '2026-11-16',
          notes: 'Pos Utama',
          action: 'create',
          is_valid: true,
          errors: [],
        },
      ],
    }

    vi.mocked(securityApi.apiPreviewSecuritySchedulesXLSX).mockResolvedValueOnce(mockPreviewResponse)
    vi.mocked(securityApi.apiCommitSecuritySchedulesXLSX).mockResolvedValueOnce({
      message: 'Berhasil menyimpan 1 jadwal',
      created_count: 1,
      updated_count: 0,
      total_committed: 1,
    })

    const onSuccessMock = vi.fn()
    const onCloseMock = vi.fn()

    render(<ScheduleImportModal open={true} onClose={onCloseMock} onSuccess={onSuccessMock} />)

    const fileInput = screen.getByLabelText('Upload file Excel') as HTMLInputElement
    const file = new File(['mock content'], 'jadwal_test.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    fireEvent.change(fileInput, { target: { files: [file] } })
    fireEvent.click(screen.getByRole('button', { name: 'Proses Pratinjau' }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Simpan 1 Jadwal Valid' })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Simpan 1 Jadwal Valid' }))

    // Modal confirmation opens
    await waitFor(() => {
      expect(screen.getByText('Konfirmasi Simpan Jadwal')).toBeInTheDocument()
    })

    const confirmBtn = screen.getByRole('button', { name: 'Ya, Simpan Jadwal' })
    fireEvent.click(confirmBtn)

    await waitFor(() => {
      expect(securityApi.apiCommitSecuritySchedulesXLSX).toHaveBeenCalledWith([
        mockPreviewResponse.rows[0],
      ])
      expect(onSuccessMock).toHaveBeenCalled()
      expect(onCloseMock).toHaveBeenCalled()
    })
  })
})

