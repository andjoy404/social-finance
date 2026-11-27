import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as AuthModule from '@/app/AuthContext'
import * as api from '@/app/api'
import * as rwApi from './rw-api'
import { RwNotificationScreen } from './RwNotificationScreen'

vi.mock('@/app/AuthContext', async () => {
  const actual = await vi.importActual<typeof AuthModule>('@/app/AuthContext')
  return {
    ...actual,
    useAuth: vi.fn(),
  }
})

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof api>('@/app/api')
  return {
    ...actual,
    getSessionPair: vi.fn().mockReturnValue({ accessToken: 'rw-test-token', refreshToken: 'rw-refresh' }),
    apiGetNotificationImageBlob: vi.fn(),
  }
})

vi.mock('./rw-api', async () => {
  const actual = await vi.importActual<typeof rwApi>('./rw-api')
  return {
    ...actual,
    apiGetRWNotifications: vi.fn(),
    apiGetRWMyNotifications: vi.fn(),
    apiListRWNotificationTemplates: vi.fn(),
    apiCreateRWNotificationTemplate: vi.fn(),
    apiUpdateRWNotificationTemplate: vi.fn(),
    apiToggleRWNotificationTemplate: vi.fn(),
    apiDeleteRWNotificationTemplate: vi.fn(),
    apiUploadRWNotificationImage: vi.fn(),
    apiPreviewRWTemplate: vi.fn(),
    apiSendRWTemplate: vi.fn(),
    apiSendRWManualPush: vi.fn(),
  }
})

const RW_ID = 'rw-016-uuid'

const baseTemplate: rwApi.ApiRWNotificationTemplate = {
  id: 'tmpl-1',
  rw_id: RW_ID,
  name: 'Rapat RW Bulanan',
  event_type: 'manual_push',
  channel: 'push',
  title: 'Undangan Rapat RW',
  body: 'Mohon kehadiran seluruh pengurus RT.',
  image_url: '/api/v1/notifications/images/rw-manual/rw-016-uuid/saved.png',
  is_active: true,
  variables: [],
  created_at: '2026-11-22T08:00:00Z',
  updated_at: '2026-11-22T08:00:00Z',
}

describe('RwNotificationScreen (Web)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
    vi.mocked(rwApi.apiGetRWNotifications).mockResolvedValue({
      data: [],
      pagination: { page: 1, page_size: 20, total: 0, total_pages: 1 },
    })
    vi.mocked(rwApi.apiGetRWMyNotifications).mockResolvedValue({
      data: [],
      pagination: { page: 1, page_size: 20, total: 0, total_pages: 1 },
    })
    vi.mocked(rwApi.apiListRWNotificationTemplates).mockResolvedValue([baseTemplate])
    vi.mocked(rwApi.apiCreateRWNotificationTemplate).mockResolvedValue({ ...baseTemplate, id: 'tmpl-new' })
    vi.mocked(rwApi.apiUpdateRWNotificationTemplate).mockResolvedValue(baseTemplate)
    vi.mocked(rwApi.apiDeleteRWNotificationTemplate).mockResolvedValue({ message: 'deleted' })
    vi.mocked(rwApi.apiToggleRWNotificationTemplate).mockResolvedValue({ ...baseTemplate, is_active: false })
    vi.mocked(rwApi.apiPreviewRWTemplate).mockResolvedValue({
      template_id: baseTemplate.id,
      name: baseTemplate.name,
      event_type: baseTemplate.event_type,
      original_title: baseTemplate.title,
      original_body: baseTemplate.body,
      rendered_title: baseTemplate.title,
      rendered_body: baseTemplate.body,
      placeholders: [],
      sample_data: {},
      is_active: true,
    })
    vi.mocked(api.apiGetNotificationImageBlob).mockResolvedValue(new Blob(['png'], { type: 'image/png' }))
  })

  it('removes extra header wording and shows Template + Riwayat Notifikasi Sistem sub-tabs', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'u-ketua',
        name: 'Mety',
        email: 'mety@local.com',
        systemRole: null,
        role: 'warga',
        jabatan: null,
        effectivePermissions: ['finance.read', 'activities.read', 'announcements.read'],
        rt: null,
        rwMemberships: [
          {
            rw_id: RW_ID,
            rw_name: 'RW 016',
            rw_number: 16,
            jabatan: 'ketua',
            source: 'explicit',
            permissions: ['rw.notification.read', 'rw.notification.write', 'notification.read', 'notification.manage'],
          },
        ],
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:http://localhost/rw-thumb')
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    try {
      render(<RwNotificationScreen rwId={RW_ID} rwName="RW 016" />)

      await screen.findByText('Rapat RW Bulanan')

      expect(screen.queryByText('Notifikasi RW')).not.toBeInTheDocument()
      expect(screen.queryByText('Riwayat notifikasi dan template untuk RW ini.')).not.toBeInTheDocument()
      expect(screen.queryByText(/Daftar notifikasi dan riwayat pengiriman/i)).not.toBeInTheDocument()

      expect(screen.getByRole('tab', { name: /Template/i })).toBeInTheDocument()
      expect(screen.getByRole('tab', { name: /Riwayat Notifikasi Sistem/i })).toBeInTheDocument()

      // Ketua RW who is RT warga with RT-only effectivePermissions still sees WRITE controls
      expect(screen.getByRole('button', { name: /Tambah Template/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Ubah/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Hapus/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Preview/i })).toBeInTheDocument()
    } finally {
      createSpy.mockRestore()
      revokeSpy.mockRestore()
    }
  })

  it('keeps Sekretaris RW who is also Ketua RT strictly READ-ONLY in RW notifications', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'u-sek',
        name: 'Sekretaris RW',
        email: 'sek@local.com',
        systemRole: null,
        role: 'pengurus',
        jabatan: 'ketua',
        effectivePermissions: ['finance.read', 'notification.read', 'notification.manage'],
        rt: null,
        rwMemberships: [
          {
            rw_id: RW_ID,
            rw_name: 'RW 016',
            rw_number: 16,
            jabatan: 'sekretaris',
            source: 'explicit',
            permissions: ['rw.notification.read', 'notification.read'],
          },
        ],
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:http://localhost/rw-ro')
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    try {
      render(<RwNotificationScreen rwId={RW_ID} rwName="RW 016" />)
      await screen.findByText('Rapat RW Bulanan')

      expect(screen.getByRole('button', { name: /Preview/i })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /Tambah Template/i })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /Ubah/i })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /Hapus/i })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /^Kirim$/i })).not.toBeInTheDocument()
    } finally {
      createSpy.mockRestore()
      revokeSpy.mockRestore()
    }
  })

  it('supports Create with image upload + authenticated preview, Edit preserving image, and Confirm Delete', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'u-sa',
        name: 'Super Admin',
        email: 'admin@local.com',
        systemRole: 'super_admin',
        role: 'super_admin',
        jabatan: null,
        effectivePermissions: ['notification.manage', 'notification.read'],
        rt: null,
        rwMemberships: [
          {
            rw_id: RW_ID,
            rw_name: 'RW 016',
            rw_number: 16,
            jabatan: 'super_admin',
            source: 'explicit',
            permissions: ['rw.notification.read', 'rw.notification.write', 'notification.manage'],
          },
        ],
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    const uploadedUrl = `/api/v1/notifications/images/rw-manual/${RW_ID}/new-banner.png`
    vi.mocked(rwApi.apiUploadRWNotificationImage).mockResolvedValue({
      status: 'uploaded',
      image_url: uploadedUrl,
      key: `rw-manual/${RW_ID}/new-banner.png`,
    })

    const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:http://localhost/rw-preview')
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    try {
      render(<RwNotificationScreen rwId={RW_ID} rwName="RW 016" />)
      await screen.findByText('Rapat RW Bulanan')

      // 1. Create template with image upload
      fireEvent.click(screen.getByRole('button', { name: /Tambah Template/i }))
      fireEvent.change(screen.getByLabelText(/Nama Template/i), { target: { value: 'Kerja Bakti RW' } })
      fireEvent.change(screen.getByLabelText(/^Judul$/i), { target: { value: 'Kerja Bakti Minggu' } })
      fireEvent.change(screen.getByLabelText(/Isi Pesan/i), { target: { value: 'Dimulai jam 07.00 WIB' } })

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
      expect(fileInput).toBeTruthy()
      fireEvent.change(fileInput, {
        target: { files: [new File(['png'], 'banner.png', { type: 'image/png' })] },
      })

      const formPreview = await screen.findByRole('img', { name: 'Preview Lampiran Template RW' })
      expect(formPreview).toHaveAttribute('src', 'blob:http://localhost/rw-preview')
      expect(formPreview.getAttribute('src')).not.toBe(uploadedUrl)

      fireEvent.click(screen.getByRole('button', { name: /Simpan Template/i }))
      await waitFor(() => {
        expect(vi.mocked(rwApi.apiCreateRWNotificationTemplate)).toHaveBeenCalledWith(
          'rw-test-token',
          RW_ID,
          expect.objectContaining({
            name: 'Kerja Bakti RW',
            event_type: 'manual_push',
            title: 'Kerja Bakti Minggu',
            body: 'Dimulai jam 07.00 WIB',
            image_url: uploadedUrl,
          }),
        )
      })

      // 2. Edit without replacing image omits image_url so backend preserves saved image
      fireEvent.click(screen.getByRole('button', { name: /Ubah/i }))
      await screen.findByRole('img', { name: 'Preview Lampiran Template RW' })
      fireEvent.change(screen.getByLabelText(/^Judul$/i), { target: { value: 'Undangan Rapat RW (Revisi)' } })
      fireEvent.click(screen.getByRole('button', { name: /Simpan Perubahan/i }))

      await waitFor(() => {
        const callArgs = vi.mocked(rwApi.apiUpdateRWNotificationTemplate).mock.calls[0]
        expect(callArgs[2]).toBe('tmpl-1')
        expect(callArgs[3]).toEqual({
          name: 'Rapat RW Bulanan',
          title: 'Undangan Rapat RW (Revisi)',
          body: 'Mohon kehadiran seluruh pengurus RT.',
        })
        expect('image_url' in callArgs[3]).toBe(false)
      })

      // 3. Delete opens confirmation modal and deletes on confirm
      fireEvent.click(screen.getByTitle('Hapus template'))
      await screen.findByText(/Apakah Anda yakin ingin menghapus template/i)
      const deleteButtons = screen.getAllByRole('button', { name: /^Hapus$/i })
      fireEvent.click(deleteButtons[deleteButtons.length - 1])

      await waitFor(() => {
        expect(vi.mocked(rwApi.apiDeleteRWNotificationTemplate)).toHaveBeenCalledWith(
          'rw-test-token',
          RW_ID,
          'tmpl-1',
        )
      })
    } finally {
      createSpy.mockRestore()
      revokeSpy.mockRestore()
    }
  })

  it('opens Detail Notifikasi RW bubble modal with authenticated image and handles old logs without image safely', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'u-sa',
        name: 'Super Admin',
        email: 'admin@local.com',
        systemRole: 'super_admin',
        role: 'super_admin',
        jabatan: null,
        effectivePermissions: ['notification.manage', 'notification.read'],
        rt: null,
        rwMemberships: [
          {
            rw_id: RW_ID,
            rw_name: 'RW 016',
            rw_number: 16,
            jabatan: 'super_admin',
            source: 'explicit',
            permissions: ['rw.notification.read', 'rw.notification.write', 'notification.manage'],
          },
        ],
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    const logImg = `/api/v1/notifications/images/notifications/rw-kas/${RW_ID}/kas-in.png`
    vi.mocked(rwApi.apiGetRWNotifications).mockResolvedValue({
      data: [
        {
          id: 'log-with-img',
          event_type: 'rw_cash_in',
          title: 'Kas Masuk RW Rp 500.000',
          body: 'Penerimaan iuran RT',
          status: 'SENT',
          image_url: logImg,
          payload: { image_url: logImg },
          created_at: '2026-11-23T01:00:00Z',
        },
        {
          id: 'log-no-img',
          event_type: 'manual_push',
          title: 'Notifikasi Lama Tanpa Gambar',
          body: 'Pesan lama',
          status: 'SENT',
          image_url: null,
          payload: {},
          created_at: '2026-11-22T01:00:00Z',
        },
      ],
      pagination: { page: 1, page_size: 20, total: 2, total_pages: 1 },
    })

    const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:http://localhost/detail-bubble-img')
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    try {
      render(<RwNotificationScreen rwId={RW_ID} rwName="RW 016" />)
      await screen.findByText('Rapat RW Bulanan')

      fireEvent.click(screen.getByRole('tab', { name: /Riwayat Notifikasi Sistem/i }))
      await screen.findByText('Kas Masuk RW Rp 500.000')
      await screen.findByText('Notifikasi Lama Tanpa Gambar')

      const detailButtons = screen.getAllByRole('button', { name: /Detail/i })
      expect(detailButtons.length).toBeGreaterThanOrEqual(2)

      // Open log WITH image -> renders authenticated blob image inside Detail bubble modal
      fireEvent.click(detailButtons[0])
      const bannerImg = await screen.findByRole('img', { name: 'Banner Detail Notifikasi RW' })
      expect(bannerImg.getAttribute('src')).toMatch(/^blob:/)
      expect(bannerImg.getAttribute('src')).not.toBe(logImg)

      fireEvent.click(screen.getByRole('button', { name: /^Tutup$/i }))

      // Open legacy log WITHOUT image -> renders cleanly with zero crash and no banner img
      fireEvent.click(detailButtons[1])
      await screen.findByText('Detail Notifikasi RW')
      expect(screen.queryByRole('img', { name: 'Banner Detail Notifikasi RW' })).not.toBeInTheDocument()
    } finally {
      createSpy.mockRestore()
      revokeSpy.mockRestore()
    }
  })
})
