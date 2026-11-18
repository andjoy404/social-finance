import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import { MemoryRouter } from 'react-router-dom'
import * as AuthModule from '@/app/AuthContext'
import type { AuthUser } from '@/app/AuthContext'
import * as SecurityApi from './securityApi'
import { PatrolPage } from './PatrolPage'

vi.mock('./securityApi', async () => {
  const actual = await vi.importActual<typeof import('./securityApi')>('./securityApi')
  return {
    ...actual,
    apiListPatrolBundles: vi.fn(),
    apiGetPatrolBundle: vi.fn(),
    apiListPatrolTemplates: vi.fn(),
    apiCreatePatrolTemplate: vi.fn(),
    apiUpdatePatrolTemplate: vi.fn(),
    apiDeletePatrolTemplate: vi.fn(),
    apiUploadPatrolPhoto: vi.fn(),
    apiCreatePatrolBundle: vi.fn(),
    apiGetPatrolPhotoBlob: vi.fn(),
    apiMarkSecurityReadReceipt: vi.fn(),
  }
})

// Mock URL createObjectURL / revokeObjectURL for jsdom
beforeEach(() => {
  if (!URL.createObjectURL) {
    URL.createObjectURL = vi.fn().mockReturnValue('blob:mock-url')
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

const mockTemplates: SecurityApi.PatrolTemplate[] = [
  {
    id: 'tpl-1',
    rt_id: 'rt-002',
    name: 'Gang Mawar',
    location_label: 'RT 002 Blok A',
    is_active: true,
    created_at: '2026-11-17T00:00:00Z',
    updated_at: '2026-11-17T00:00:00Z',
  },
  {
    id: 'tpl-2',
    rt_id: 'rt-002',
    name: 'Pos Ronda 1',
    location_label: 'Gerbang Utama',
    is_active: true,
    created_at: '2026-11-17T00:00:00Z',
    updated_at: '2026-11-17T00:00:00Z',
  },
]

const mockBundles: SecurityApi.PatrolBundle[] = [
  {
    id: 'bundle-101',
    rt_id: 'rt-002',
    created_by: 'officer-1',
    created_by_name: 'Budi Petugas Keamanan',
    submitted_at: '2026-11-17T08:30:00Z',
    notes: 'Kondisi aman terkendali, pintu portal tertutup.',
    photo_count: 2,
    photos: [
      {
        id: 'photo-1',
        patrol_bundle_id: 'bundle-101',
        patrol_template_id: 'tpl-1',
        template_name: 'Gang Mawar',
        location_label: 'RT 002 Blok A',
        image_key: 'patrol/photo1.jpg',
        image_url: '/api/v1/security/patrol/photos/photo-1',
        captured_at: '2026-11-17T08:25:00Z',
        created_at: '2026-11-17T08:30:00Z',
      },
    ],
    created_at: '2026-11-17T08:30:00Z',
    updated_at: '2026-11-17T08:30:00Z',
  },
]

describe('PatrolPage Frontend Tests', () => {
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
    vi.mocked(SecurityApi.apiListPatrolTemplates).mockResolvedValue(mockTemplates)
    vi.mocked(SecurityApi.apiListPatrolBundles).mockResolvedValue({
      data: mockBundles,
      pagination: { page: 1, per_page: 15, total: 1, total_pages: 1 },
    })
    vi.mocked(SecurityApi.apiGetPatrolBundle).mockResolvedValue(mockBundles[0])
    vi.mocked(SecurityApi.apiGetPatrolPhotoBlob).mockResolvedValue(new Blob(['test'], { type: 'image/jpeg' }))
    vi.mocked(SecurityApi.apiMarkSecurityReadReceipt).mockResolvedValue(undefined)
  })

  it('1. Rejects access when user lacks patrol permissions (e.g. Warga)', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: wargaUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    expect(screen.queryByText('Kirim Laporan')).not.toBeInTheDocument()
  })

  it('2. Rejects access for Sekretaris who only has notification permission', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: sekretarisUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    expect(screen.queryByText('Kirim Laporan')).not.toBeInTheDocument()
  })

  it('3. Petugas Keamanan can see tabs and open "Kirim Laporan" form', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Patroli Wilayah')).toBeInTheDocument()
    })

    const kirimTab = screen.getByRole('button', { name: /KIRIM LAPORAN/i })
    expect(kirimTab).toBeInTheDocument()

    // Switch to create form tab
    fireEvent.click(kirimTab)
    expect(screen.getByRole('heading', { name: /Kirim Laporan Patroli Wilayah/i })).toBeInTheDocument()
    expect(screen.getByText(/Pilih Lokasi Gang/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Ambil Foto atau Pilih Gambar/i })).toBeInTheDocument()
  })

  it('4. Petugas Keamanan can submit patrol report with photo', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    vi.mocked(SecurityApi.apiUploadPatrolPhoto).mockResolvedValue({ image_key: 'patrol/new_photo.jpg' })
    vi.mocked(SecurityApi.apiCreatePatrolBundle).mockResolvedValue({
      id: 'bundle-new',
      rt_id: 'rt-002',
      created_by: 'officer-1',
      photo_count: 1,
      submitted_at: new Date().toISOString(),
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    })

    const { container } = render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    // Wait for templates and initial data to load
    await waitFor(() => {
      expect(SecurityApi.apiListPatrolTemplates).toHaveBeenCalled()
    })

    // Open Kirim Laporan tab
    fireEvent.click(screen.getByRole('button', { name: /KIRIM LAPORAN/i }))

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /Kirim Laporan Patroli Wilayah/i })).toBeInTheDocument()
    })

    // Upload a photo
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement
    expect(fileInput).toBeInTheDocument()

    const mockFile = new File(['mock content'], 'gang_mawar.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [mockFile] } })

    // Check preview appears
    await waitFor(() => {
      expect(screen.getByText(/Foto yang akan dikirim/i)).toBeInTheDocument()
    })

    // Fill notes
    const textarea = screen.getByPlaceholderText(/Tuliskan keterangan situasi/i)
    fireEvent.change(textarea, { target: { value: 'Patroli putaran pertama aman.' } })

    // Submit form — target submit button by class, not by text (tab button has same text)
    const submitBtn = container.querySelector('.sf-btn-primary[type="submit"]')
    expect(submitBtn).toBeInTheDocument()
    fireEvent.click(submitBtn!)

    await waitFor(() => {
      expect(SecurityApi.apiUploadPatrolPhoto).toHaveBeenCalled()
      expect(SecurityApi.apiCreatePatrolBundle).toHaveBeenCalledWith(
        expect.objectContaining({
          notes: 'Patroli putaran pertama aman.',
          photos: expect.arrayContaining([
            expect.objectContaining({
              image_key: 'patrol/new_photo.jpg',
            }),
          ]),
        })
      )
    })
  })

  it('5. Riwayat Patroli displays bundle list and officer details', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaRTUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Petugas Keamanan')).toBeInTheDocument()
      expect(screen.getByText('2 Foto')).toBeInTheDocument()
      expect(screen.getByText(/Kondisi aman terkendali/i)).toBeInTheDocument()
    })
  })

  it('6. Detail Patroli modal opens and renders photo gallery', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaRTUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Budi Petugas Keamanan')).toBeInTheDocument()
    })

    const detailBtn = screen.getByRole('button', { name: /Detail/i })
    fireEvent.click(detailBtn)

    await waitFor(() => {
      expect(screen.getByText('Detail Laporan Patroli')).toBeInTheDocument()
      expect(screen.getByText('Gang Mawar')).toBeInTheDocument()
      expect(screen.getByText('RT 002 Blok A')).toBeInTheDocument()
    })
  })

  it('7. Ketua RT is strictly monitoring/read-only: no "Kirim Laporan" tab', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaRTUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    // Has Riwayat and Template Lokasi (patrol.template.manage), but NO Kirim Laporan tab
    expect(screen.getByText('RIWAYAT PATROLI')).toBeInTheDocument()
    expect(screen.getByText('TEMPLATE LOKASI')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /KIRIM LAPORAN/i })).not.toBeInTheDocument()
  })

  it('8. Deep link ?bundleId=bundle-101 opens detail modal automatically', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter initialEntries={['/patroli?bundleId=bundle-101']}>
        <PatrolPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(SecurityApi.apiGetPatrolBundle).toHaveBeenCalledWith('bundle-101')
      expect(screen.getByText('Detail Laporan Patroli')).toBeInTheDocument()
    })
  })

  it('9. Empty state is shown cleanly when no patrol records exist', async () => {
    vi.mocked(SecurityApi.apiListPatrolBundles).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 15, total: 0, total_pages: 1 },
    })

    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    await waitFor(
      () => {
        expect(screen.getByText('Tidak ada laporan patroli.')).toBeInTheDocument()
      },
      { timeout: 5000 },
    )
  })

  it('10. API error is displayed cleanly in alert banner', async () => {
    vi.mocked(SecurityApi.apiListPatrolBundles).mockRejectedValue(new Error('Koneksi server gagal'))

    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: securityOfficerUser,
      isAuthenticated: true,
      token: 'test-token',
    } as any)

    render(
      <MemoryRouter>
        <PatrolPage />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Koneksi server gagal')).toBeInTheDocument()
    })
  })
})
