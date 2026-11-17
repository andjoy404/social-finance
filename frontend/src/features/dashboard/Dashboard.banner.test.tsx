// Banner thumbnail rendering integration test for the dashboard Beranda.
// Verifies that image_url from the backend API is rendered through the
// CommunityImage component (signed URL / Bearer blob resolution).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom'
import { MemoryRouter } from 'react-router-dom'
import { Dashboard } from '@/features/dashboard/Dashboard'
import * as AuthModule from '@/app/AuthContext'
import * as ApiModule from '@/app/api'
import type { AuthUser } from '@/app/AuthContext'
import type { HomeDashboardData } from '@/app/api'

const mockWargaUser: AuthUser = {
  id: 'user-warga',
  name: 'Warga Test',
  email: 'warga@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'RT 002' },
  rtCode: '002',
  rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
}

function makeDashboard(imageUrl: string | null): HomeDashboardData {
  return {
    rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
    date: '2026-11-11',
    total_tunggakan: 0,
    statistik: { hinggaHariIni: 0, mingguIni: 0, bulanIni: 0 },
    activities: [],
    banners: [
      {
        id: 'banner-1',
        rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        source_type: 'announcement',
        source_id: 'announcement-1',
        title: 'Pengumuman Penting',
        body: 'Isi pengumuman penting',
        image_url: imageUrl,
        deep_link: '/pengumuman',
        template_id: null,
        priority: 5,
        display_order: 1,
        published_at: '2026-11-11T00:00:00Z',
        created_at: '2026-11-11T00:00:00Z',
        updated_at: '2026-11-11T00:00:00Z',
      } as unknown as HomeDashboardData['banners'][number],
    ],
    recent_activity: [],
  } as unknown as HomeDashboardData
}

// Confirm the banner image is wired through CommunityImage.
vi.mock('@/components/CommunityImage', () => ({
  CommunityImage: ({ src, alt }: { src: string; alt: string }) => (
    <img data-testid="community-image" alt={alt} src={src} />
  ),
}))

describe('Dashboard — Banner thumbnail wiring (Web)', () => {
  beforeEach(() => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: mockWargaUser,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    } as unknown as ReturnType<typeof AuthModule.useAuth>)

    vi.spyOn(ApiModule, 'getSessionPair').mockReturnValue({
      accessToken: 'tok-test',
      refreshToken: 'r-test',
    } as unknown as ReturnType<typeof ApiModule.getSessionPair>)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders banner image_url through CommunityImage when present', async () => {
    const imageURL = '/api/v1/community/images/community/rt-1/announcement/tpl.png'
    vi.spyOn(ApiModule, 'apiGetHomeDashboard').mockResolvedValue(makeDashboard(imageURL))

    render(
      <MemoryRouter>
        <Dashboard />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByTestId('community-image')).toBeInTheDocument())
    expect(screen.getByTestId('community-image')).toHaveAttribute('src', imageURL)
  })

  it('hides thumbnail when banner has no image_url', async () => {
    vi.spyOn(ApiModule, 'apiGetHomeDashboard').mockResolvedValue(makeDashboard(null))

    render(
      <MemoryRouter>
        <Dashboard />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Pengumuman Penting')).toBeInTheDocument())
    expect(screen.queryByTestId('community-image')).not.toBeInTheDocument()
  })
})
