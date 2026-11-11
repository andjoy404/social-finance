import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { AnnouncementManagement } from './AnnouncementManagement'
import * as api from '@/app/api'
import * as auth from '@/app/AuthContext'

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof import('@/app/api')>('@/app/api')
  return {
    ...actual,
    getSessionPair: vi.fn(() => ({ accessToken: 'test-token', refreshToken: 'refresh-token' })),
    apiListAnnouncements: vi.fn(),
    apiListBannerTemplates: vi.fn(() => Promise.resolve([])),
    apiPublishAnnouncement: vi.fn(),
    apiArchiveAnnouncement: vi.fn(),
    apiDeleteAnnouncement: vi.fn(),
    apiGetCommunityMediaUrl: vi.fn(() => Promise.resolve({ url: 'https://example.com/signed-banner.jpg' })),
  }
})

const sampleAnnouncements: api.CommunityAnnouncement[] = [
  {
    id: 'ann-1',
    rt_id: 'rt-1',
    title: 'Jadwal Pemadaman Listrik Bergilir Blok A & B',
    body: 'PLN akan melakukan pemeliharaan gardu distribusi pada hari Sabtu.',
    image_url: '/api/v1/community/media/banner-pln.jpg',
    deep_link: '/kegiatan',
    scheduled_start: '2026-11-10T00:00:00Z',
    scheduled_end: '2026-11-16T16:59:00Z',
    priority: 10,
    display_order: 1,
    status: 'active',
    created_at: '2026-11-10T00:00:00Z',
    updated_at: '2026-11-10T00:00:00Z',
  },
  {
    id: 'ann-2',
    rt_id: 'rt-1',
    title: 'Rancangan Penyesuaian Iuran Keamanan 2027',
    body: '',
    image_url: null,
    deep_link: null,
    scheduled_start: null,
    scheduled_end: null,
    priority: 5,
    display_order: 2,
    status: 'draft',
    created_at: '2026-11-11T00:00:00Z',
    updated_at: '2026-11-11T00:00:00Z',
  },
  {
    id: 'ann-3',
    rt_id: 'rt-1',
    title: 'Laporan Pelaksanaan Fogging Nyamuk DBD',
    body: 'Kegiatan fogging telah selesai dilaksanakan di seluruh blok RT 002.',
    image_url: null,
    deep_link: null,
    scheduled_start: null,
    scheduled_end: null,
    priority: 1,
    display_order: 3,
    status: 'archived',
    created_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-05T00:00:00Z',
  },
]

describe('AnnouncementManagement (Standardized Notification Table Pattern)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.mocked(api.apiListAnnouncements).mockResolvedValue({
      data: sampleAnnouncements,
      pagination: { page: 1, per_page: 10, total: 3, total_pages: 1 },
    })
  })

  it('renders announcements without thumbnails in list, shows attachment icon, and keeps image in detail modal', async () => {
    vi.spyOn(auth, 'useAuth').mockReturnValue({
      user: { id: 'u-1', role: 'pengurus', permissions: ['announcements.manage', 'announcements.delete'] } as any,
    } as any)
    vi.spyOn(auth, 'canManageAnnouncements').mockReturnValue(true)

    const { container } = render(
      <MemoryRouter>
        <AnnouncementManagement />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Jadwal Pemadaman Listrik Bergilir Blok A & B')).toBeInTheDocument()
    })

    // No <img> thumbnail inside the list table
    expect(container.querySelectorAll('table img').length).toBe(0)
    // Attachment icon shown for ann-1
    expect(screen.getAllByTitle('Memiliki lampiran gambar').length).toBe(1)

    // Click Lihat Detail on ann-1 -> opens detail modal where image IS rendered
    const detailButtons = screen.getAllByRole('button', { name: 'Lihat Detail' })
    expect(detailButtons.length).toBe(3)
    fireEvent.click(detailButtons[0])

    expect(await screen.findByText('ISI PENGUMUMAN')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.getByRole('img', { name: 'Jadwal Pemadaman Listrik Bergilir Blok A & B' })).toBeInTheDocument()
    })
  })

  it('preserves all 6 existing actions and callbacks for manager', async () => {
    vi.spyOn(auth, 'useAuth').mockReturnValue({
      user: { id: 'u-1', role: 'pengurus', permissions: ['announcements.manage', 'announcements.delete'] } as any,
    } as any)
    vi.spyOn(auth, 'canManageAnnouncements').mockReturnValue(true)
    vi.mocked(api.apiPublishAnnouncement).mockResolvedValue({} as any)
    vi.mocked(api.apiArchiveAnnouncement).mockResolvedValue({} as any)
    vi.mocked(api.apiDeleteAnnouncement).mockResolvedValue({} as any)

    render(
      <MemoryRouter>
        <AnnouncementManagement />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Jadwal Pemadaman Listrik Bergilir Blok A & B')).toBeInTheDocument()
    })

    // Publish (draft only: ann-2)
    const publishButtons = screen.getAllByRole('button', { name: 'Publikasikan' })
    expect(publishButtons.length).toBe(1)
    fireEvent.click(publishButtons[0])
    await waitFor(() => {
      expect(api.apiPublishAnnouncement).toHaveBeenCalledWith('test-token', 'ann-2')
    })

    // Republish (active/scheduled/archived: ann-1 and ann-3)
    const republishButtons = screen.getAllByRole('button', { name: 'Terbitkan Ulang' })
    expect(republishButtons.length).toBe(2)
    fireEvent.click(republishButtons[0])
    await waitFor(() => {
      expect(api.apiPublishAnnouncement).toHaveBeenCalledWith('test-token', 'ann-1')
    })

    // Archive (active/scheduled: ann-1)
    const archiveButtons = screen.getAllByRole('button', { name: 'Arsipkan' })
    expect(archiveButtons.length).toBe(1)
    fireEvent.click(archiveButtons[0])
    await waitFor(() => {
      expect(api.apiArchiveAnnouncement).toHaveBeenCalledWith('test-token', 'ann-1')
    })

    // Delete (draft only: ann-2)
    const deleteButtons = screen.getAllByRole('button', { name: 'Hapus' })
    expect(deleteButtons.length).toBe(1)
    fireEvent.click(deleteButtons[0])
    await waitFor(() => {
      expect(api.apiDeleteAnnouncement).toHaveBeenCalledWith('test-token', 'ann-2')
    })

    // Edit (all 3 items)
    const editButtons = screen.getAllByRole('button', { name: 'Ubah' })
    expect(editButtons.length).toBe(3)
    fireEvent.click(editButtons[0])
    expect(await screen.findByText('Ubah Pengumuman')).toBeInTheDocument()
  })
})
