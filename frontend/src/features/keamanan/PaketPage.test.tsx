import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import { MemoryRouter } from 'react-router-dom'
import * as AuthModule from '@/app/AuthContext'
import type { AuthUser } from '@/app/AuthContext'
import * as SecurityApi from './securityApi'
import { PaketPage } from './PaketPage'

vi.mock('./securityApi', async () => {
  const actual = await vi.importActual<typeof import('./securityApi')>('./securityApi')
  return {
    ...actual,
    apiListPaketReports: vi.fn(),
    apiGetPaketReport: vi.fn(),
    apiUploadPaketPhoto: vi.fn(),
    apiCreatePaketReport: vi.fn(),
    apiUpdatePaketItemStatus: vi.fn(),
    apiGetPaketPhotoBlob: vi.fn(),
    apiSearchWargaMasked: vi.fn(),
    apiMarkSecurityReadReceipt: vi.fn(),
  }
})

beforeEach(() => {
  if (!URL.createObjectURL) {
    URL.createObjectURL = vi.fn().mockReturnValue('blob:mock-paket-url')
  }
  if (!URL.revokeObjectURL) {
    URL.revokeObjectURL = vi.fn()
  }
})

const securityOfficerUser: AuthUser = {
  id: 'officer-1',
  name: 'Budi Petugas Keamanan',
  email: 'keamanan@local.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'keamanan',
  effectivePermissions: ['patrol.read', 'patrol.create', 'paket.read', 'paket.create', 'paket.status_change'],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
}

const ketuaRTUser: AuthUser = {
  id: 'ketua-1',
  name: 'Ketua RT 002',
  email: 'ketua@local.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'ketua',
  effectivePermissions: ['patrol.read', 'patrol.template.manage', 'paket.read'],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
}

const sekretarisUser: AuthUser = {
  id: 'sekretaris-1',
  name: 'Sekretaris RT 002',
  email: 'sekretaris@local.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'sekretaris',
  effectivePermissions: ['notification.read', 'notification.manage', 'activities.read', 'announcements.read'],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
}

const wargaUser: AuthUser = {
  id: 'warga-1',
  name: 'Warga Biasa',
  email: 'warga@local.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  effectivePermissions: [],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
}

const mockReports: SecurityApi.PaketReport[] = [
  {
    id: 'report-201',
    rt_id: 'rt-002',
    receiver_warga_id: 'resident-1',
    receiver_name: 'Ahmad Dahlan',
    receiver_house_number: '14B',
    created_by: 'officer-1',
    created_by_name: 'Budi Petugas Keamanan',
    overall_status: 'dititipkan',
    items: [
      {
        id: 'item-1',
        paket_report_id: 'report-201',
        description: 'Kardus Sedang SiCepat',
        status: 'dititipkan',
        photos: [
          {
            id: 'photo-paket-1',
            paket_item_id: 'item-1',
            paket_report_id: 'report-201',
            image_key: 'paket/box.jpg',
            image_url: '/api/v1/security/paket/photos/photo-paket-1',
            photo_type: 'paket',
            captured_at: '2026-11-17T09:00:00Z',
            created_at: '2026-11-17T09:00:00Z',
          },
        ],
        created_at: '2026-11-17T09:00:00Z',
        updated_at: '2026-11-17T09:00:00Z',
      },
    ],
    created_at: '2026-11-17T09:00:00Z',
    updated_at: '2026-11-17T09:00:00Z',
  },
]

describe('PaketPage Frontend Tests', () => {
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
    vi.mocked(SecurityApi.apiListPaketReports).mockResolvedValue({
      data: mockReports,
      pagination: { page: 1, per_page: 15, total: 1, total_pages: 1 },
    })
    vi.mocked(SecurityApi.apiGetPaketReport).mockResolvedValue(mockReports[0])
    vi.mocked(SecurityApi.apiGetPaketPhotoBlob).mockResolvedValue(new Blob(['test-paket'], { type: 'image/jpeg' }))
    vi.mocked(SecurityApi.apiMarkSecurityReadReceipt).mockResolvedValue(undefined)
  })

  it('1. Rejects access when user lacks paket permissions (e.g. Warga)', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: wargaUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    expect(screen.queryByText('TERIMA PAKET BARU')).not.toBeInTheDocument()
  })

  it('2. Rejects access for Sekretaris who only has notification permission', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: sekretarisUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    expect(screen.queryByText('TERIMA PAKET BARU')).not.toBeInTheDocument()
  })

  it('3. Petugas Keamanan can see tabs and open "Terima Paket Baru" form', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    expect(screen.getByText('Titipan Paket Warga')).toBeInTheDocument()
    const terimaTab = screen.getByRole('button', { name: /TERIMA PAKET BARU/i })
    expect(terimaTab).toBeInTheDocument()

    fireEvent.click(terimaTab)
    expect(screen.getByText(/Formulir Penerimaan Titipan Paket/i)).toBeInTheDocument()
    expect(screen.getByText(/Penerima \/ Pemilik Paket/i)).toBeInTheDocument()
    expect(screen.getByText(/Item Paket Titipan/i)).toBeInTheDocument()
  })

  it('4. Petugas Keamanan can search warga, add photo, and submit paket report', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    vi.mocked(SecurityApi.apiSearchWargaMasked).mockResolvedValue({
      data: [{ id: 'res-99', full_name: 'Bambang Sudirman', house_number: '20' }],
    })
    vi.mocked(SecurityApi.apiUploadPaketPhoto).mockResolvedValue({ image_key: 'paket/uploaded_box.jpg' })
    vi.mocked(SecurityApi.apiCreatePaketReport).mockResolvedValue({
      id: 'report-new',
      rt_id: 'rt-002',
      receiver_warga_id: 'res-99',
      receiver_name: 'Bambang Sudirman',
      created_by: 'officer-1',
      overall_status: 'dititipkan',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    })

    const { container } = render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    // Switch to create form
    fireEvent.click(screen.getByRole('button', { name: /TERIMA PAKET BARU/i }))

    // Search warga
    const searchInput = screen.getByPlaceholderText(/Ketik nama warga/i)
    fireEvent.change(searchInput, { target: { value: 'Bambang' } })

    // Wait for search result dropdown
    await waitFor(() => {
      expect(screen.getByText('Bambang Sudirman')).toBeInTheDocument()
    })

    // Select warga
    fireEvent.click(screen.getByText('Bambang Sudirman'))
    expect(screen.getByText('(Rumah No. 20)')).toBeInTheDocument()

    // Enter item description
    const descInput = screen.getByPlaceholderText(/Keterangan paket/i)
    fireEvent.change(descInput, { target: { value: 'Kardus J&T Express' } })

    // Upload photo
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement
    const mockFile = new File(['mock content'], 'paket.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [mockFile] } })

    // Submit
    const submitBtn = screen.getByRole('button', { name: /Simpan Penerimaan/i })
    fireEvent.click(submitBtn)

    await waitFor(() => {
      expect(SecurityApi.apiUploadPaketPhoto).toHaveBeenCalled()
      expect(SecurityApi.apiCreatePaketReport).toHaveBeenCalledWith({
        receiver_warga_id: 'res-99',
        items: [
          {
            description: 'Kardus J&T Express',
            image_keys: ['paket/uploaded_box.jpg'],
          },
        ],
      })
    })
  })

  it('5. Daftar Paket displays reports and handles status filters', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaRTUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Ahmad Dahlan')).toBeInTheDocument()
      expect(screen.getByText('(Rumah No. 14B)')).toBeInTheDocument()
      expect(screen.getAllByText('Dititipkan').length).toBeGreaterThan(0)
    })

    // Filter status
    fireEvent.click(screen.getByRole('button', { name: 'Sudah Diambil' }))
    // Ahmad Dahlan is 'dititipkan' so it should not match 'sudah_diambil'
    expect(screen.queryByText('Ahmad Dahlan')).not.toBeInTheDocument()
  })

  it('6. Detail Paket modal opens and shows item details & photos', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaRTUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Ahmad Dahlan')).toBeInTheDocument()
    })

    const detailBtn = screen.getByRole('button', { name: /Detail/i })
    fireEvent.click(detailBtn)

    await waitFor(() => {
      expect(screen.getByText('Detail Titipan Paket')).toBeInTheDocument()
      expect(screen.getByText('Item #1: Kardus Sedang SiCepat')).toBeInTheDocument()
    })
  })

  it('7. Ketua RT is strictly monitoring/read-only: no "Terima Paket Baru" tab and no "Serah Terima" buttons', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaRTUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    // No operational tab
    expect(screen.queryByRole('button', { name: /TERIMA PAKET BARU/i })).not.toBeInTheDocument()

    // Open detail
    await waitFor(() => expect(screen.getByText('Ahmad Dahlan')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Detail/i }))

    await waitFor(() => {
      expect(screen.getByText('Detail Titipan Paket')).toBeInTheDocument()
    })

    // No operational action buttons in detail modal
    expect(screen.queryByRole('button', { name: /Serah Terima/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Batalkan$/ })).not.toBeInTheDocument()
  })

  it('8. Petugas Keamanan can perform Serah Terima with proof photo', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    vi.mocked(SecurityApi.apiUploadPaketPhoto).mockResolvedValue({ image_key: 'paket/proof_received.jpg' })
    vi.mocked(SecurityApi.apiUpdatePaketItemStatus).mockResolvedValue({
      ...mockReports[0],
      overall_status: 'sudah_diambil',
      items: [
        {
          ...mockReports[0].items![0],
          status: 'sudah_diambil',
        },
      ],
    })

    const { container } = render(
      <MemoryRouter>
        <PaketPage />
      </MemoryRouter>
    )

    await waitFor(() => expect(screen.getByText('Ahmad Dahlan')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Detail/i }))

    await waitFor(() => {
      expect(screen.getByText('Detail Titipan Paket')).toBeInTheDocument()
    })

    // Serah Terima button is available for Petugas Keamanan
    const serahTerimaBtn = screen.getByRole('button', { name: /Serah Terima/i })
    expect(serahTerimaBtn).toBeInTheDocument()
    fireEvent.click(serahTerimaBtn)

    // Modal konfirmasi opens
    expect(screen.getByText('Konfirmasi Penyerahan Paket')).toBeInTheDocument()

    // Upload proof photo
    const fileInputs = container.querySelectorAll('input[type="file"]')
    const proofInput = fileInputs[fileInputs.length - 1] as HTMLInputElement
    const mockProofFile = new File(['proof image'], 'proof.jpg', { type: 'image/jpeg' })
    fireEvent.change(proofInput, { target: { files: [mockProofFile] } })

    // Confirm pickup
    const confirmBtn = screen.getByRole('button', { name: /Konfirmasi Serah Terima/i })
    fireEvent.click(confirmBtn)

    await waitFor(() => {
      expect(SecurityApi.apiUploadPaketPhoto).toHaveBeenCalled()
      expect(SecurityApi.apiUpdatePaketItemStatus).toHaveBeenCalledWith(
        'report-201',
        'item-1',
        expect.objectContaining({
          status: 'sudah_diambil',
          proof_image_key: 'paket/proof_received.jpg',
        })
      )
    })
  })

  it('9. Deep link ?reportId=report-201 opens detail modal directly', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter initialEntries={['/paket?reportId=report-201']}>
        <PaketPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(SecurityApi.apiGetPaketReport).toHaveBeenCalledWith('report-201')
      expect(screen.getByText('Detail Titipan Paket')).toBeInTheDocument()
    })
  })
})
