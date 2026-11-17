import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ActivityList } from './ActivityList'
import * as api from '@/app/api'
import * as auth from '@/app/AuthContext'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof import('@/app/api')>('@/app/api')
  return {
    ...actual,
    getSessionPair: vi.fn(() => ({ accessToken: 'test-token', refreshToken: 'refresh-token' })),
    apiListActivities: vi.fn(),
    apiPublishActivity: vi.fn(),
    apiCancelActivity: vi.fn(),
    apiDeleteActivity: vi.fn(),
  }
})

const sampleActivities: api.CommunityActivity[] = [
  {
    id: 'act-1',
    rt_id: 'rt-1',
    title: 'Kerja Bakti Pembersihan Saluran Air',
    description: 'Gotong royong membersihkan drainase utama dan pemangkasan pohon.',
    poster_url: '/api/v1/community/media/poster-kerja-bakti.jpg',
    starts_at: '2026-11-15T00:00:00Z',
    ends_at: '2026-11-15T04:00:00Z',
    location: 'Balai Warga & Sepanjang Blok A-C',
    organizer: 'Seksi Kebersihan RT 002',
    status: 'upcoming',
    publication_status: 'published',
    show_on_banner: true,
    created_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-01T00:00:00Z',
  },
  {
    id: 'act-2',
    rt_id: 'rt-1',
    title: 'Rapat Koordinasi Persiapan Akhir Tahun',
    description: '',
    poster_url: null,
    starts_at: '2026-11-20T12:30:00Z',
    ends_at: null,
    location: 'Posko Sekretariat RT 002',
    organizer: null,
    status: 'upcoming',
    publication_status: 'draft',
    show_on_banner: false,
    created_at: '2026-11-02T00:00:00Z',
    updated_at: '2026-11-02T00:00:00Z',
  },
  {
    id: 'act-3',
    rt_id: 'rt-1',
    title: 'Turnamen Tenis Meja Antar Blok',
    description: 'Ditunda karena renovasi aula lapangan warga.',
    poster_url: '/api/v1/community/media/poster-tenis.jpg',
    starts_at: '2026-11-18T02:00:00Z',
    ends_at: null,
    location: 'Aula Serbaguna RT 002',
    organizer: 'Seksi Pemuda & Olahraga',
    status: 'cancelled',
    publication_status: 'published',
    show_on_banner: false,
    created_at: '2026-11-03T00:00:00Z',
    updated_at: '2026-11-03T00:00:00Z',
  },
]

describe('ActivityList (Standardized Notification Table Pattern)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.mocked(api.apiListActivities).mockResolvedValue({
      data: sampleActivities,
      pagination: { page: 1, per_page: 9, total: 3, total_pages: 1 },
    })
  })

  it('renders multiple items without image thumbnails in list and shows attachment icon instead', async () => {
    vi.spyOn(auth, 'useAuth').mockReturnValue({
      user: { id: 'u-1', role: 'pengurus', permissions: ['activities.manage', 'activities.delete'] } as any,
    } as any)
    vi.spyOn(auth, 'canManageActivities').mockReturnValue(true)

    const { container } = render(
      <MemoryRouter>
        <ActivityList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Kerja Bakti Pembersihan Saluran Air')).toBeInTheDocument()
    })

    // All 3 items rendered in table rows
    expect(screen.getByText('Rapat Koordinasi Persiapan Akhir Tahun')).toBeInTheDocument()
    expect(screen.getByText('Turnamen Tenis Meja Antar Blok')).toBeInTheDocument()

    // Full metadata vs partial metadata
    expect(screen.getByText('Penyelenggara: Seksi Kebersihan RT 002')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()

    // Status badges (plus filter tab buttons)
    expect(screen.getAllByText('Mendatang').length).toBeGreaterThanOrEqual(2)
    expect(screen.getAllByText('Dibatalkan').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('DRAF')).toBeInTheDocument()
    expect(screen.getByText('BANNER')).toBeInTheDocument()

    // No <img> thumbnail rendered in the list table
    expect(container.querySelectorAll('table img').length).toBe(0)
    // Attachment indicator icon rendered for items with poster_url (act-1 and act-3)
    expect(screen.getAllByTitle('Memiliki lampiran gambar').length).toBe(2)
  })

  it('preserves exact actions and callbacks for manager (publish/republish, cancel, edit, delete)', async () => {
    vi.spyOn(auth, 'useAuth').mockReturnValue({
      user: { id: 'u-1', role: 'pengurus', permissions: ['activities.manage', 'activities.delete'] } as any,
    } as any)
    vi.spyOn(auth, 'canManageActivities').mockReturnValue(true)
    vi.mocked(api.apiPublishActivity).mockResolvedValue({} as any)
    vi.mocked(api.apiCancelActivity).mockResolvedValue({} as any)
    vi.mocked(api.apiDeleteActivity).mockResolvedValue({} as any)

    render(
      <MemoryRouter>
        <ActivityList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Kerja Bakti Pembersihan Saluran Air')).toBeInTheDocument()
    })

    // Row click navigates to detail page
    fireEvent.click(screen.getByRole('link', { name: 'Detail kegiatan Kerja Bakti Pembersihan Saluran Air' }))
    expect(mockNavigate).toHaveBeenCalledWith('/kegiatan/act-1')

    // Publish action on draft (act-2)
    fireEvent.click(screen.getByRole('button', { name: 'Publikasikan' }))
    await waitFor(() => {
      expect(api.apiPublishActivity).toHaveBeenCalledWith('test-token', 'act-2')
    })

    // Republish action on published non-cancelled (act-1)
    fireEvent.click(screen.getByRole('button', { name: 'Terbitkan Ulang' }))
    await waitFor(() => {
      expect(api.apiPublishActivity).toHaveBeenCalledWith('test-token', 'act-1')
    })

    // Cancel action on non-cancelled (act-1, act-2)
    const cancelButtons = screen.getAllByRole('button', { name: 'Batalkan' })
    expect(cancelButtons.length).toBe(2)
    fireEvent.click(cancelButtons[0])
    await waitFor(() => {
      expect(api.apiCancelActivity).toHaveBeenCalledWith('test-token', 'act-1')
    })

    // Delete action ONLY available on draft (act-2)
    const deleteButtons = screen.getAllByRole('button', { name: 'Hapus' })
    expect(deleteButtons.length).toBe(1)
    fireEvent.click(deleteButtons[0])
    await waitFor(() => {
      expect(api.apiDeleteActivity).toHaveBeenCalledWith('test-token', 'act-2')
    })

    // Edit action available on all 3 items and opens modal
    const editButtons = screen.getAllByRole('button', { name: 'Ubah' })
    expect(editButtons.length).toBe(3)
    fireEvent.click(editButtons[0])
    expect(await screen.findByText('Ubah Kegiatan')).toBeInTheDocument()
  })

  it('hides management action column for warga (non-manager)', async () => {
    vi.spyOn(auth, 'useAuth').mockReturnValue({
      user: { id: 'u-2', role: 'warga', permissions: [] } as any,
    } as any)
    vi.spyOn(auth, 'canManageActivities').mockReturnValue(false)

    render(
      <MemoryRouter>
        <ActivityList />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Kerja Bakti Pembersihan Saluran Air')).toBeInTheDocument()
    })

    expect(screen.queryByRole('button', { name: 'Ubah' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Hapus' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Batalkan' })).not.toBeInTheDocument()
  })
})
