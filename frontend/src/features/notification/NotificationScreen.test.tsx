import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { NotificationScreen } from './NotificationScreen'
import {
  isManualNotification,
  validateSafeHttpUrl,
  parseNotificationTextSegments,
} from './notificationUrl'
import * as api from '@/app/api'

import * as AuthModule from '@/app/AuthContext'

vi.mock('@/app/AuthContext', async () => {
  const actual = await vi.importActual<typeof AuthModule>('@/app/AuthContext')
  return {
    ...actual,
    useAuth: vi.fn().mockReturnValue({
      user: { id: 'usr-admin', email: 'admin@rt.local', systemRole: 'super_admin', role: 'pengurus', jabatan: null },
    }),
  }
})

vi.mock('@/app/api', async () => {
  const actual = await vi.importActual<typeof api>('@/app/api')
  return {
    ...actual,
    apiGetNotificationSystemTypes: vi.fn(),
    apiGetNotificationTemplates: vi.fn(),
    apiCreateNotificationTemplate: vi.fn(),
    apiUpdateNotificationTemplate: vi.fn(),
    apiToggleNotificationTemplateActive: vi.fn(),
    apiDeleteNotificationTemplate: vi.fn(),
    apiGetNotificationLogs: vi.fn(),
    apiGetNotificationManagementHistory: vi.fn(),
    apiUploadNotificationImage: vi.fn(),
    apiGetNotificationImageBlob: vi.fn(),
    apiTriggerManualPush: vi.fn().mockResolvedValue({ status: 'accepted', message: 'dispatched' }),
    apiTestPush: vi.fn().mockResolvedValue({ status: 'dispatched', message: 'dispatched', dispatched_count: 1 }),
    apiMarkNotificationAsRead: vi.fn().mockResolvedValue(undefined),
    apiMarkAllNotificationsAsRead: vi.fn().mockResolvedValue({ marked: 1 }),
    apiGetNotificationUnreadCount: vi.fn().mockResolvedValue({ unread_count: 1 }),
    apiGetNotificationSchedules: vi.fn().mockResolvedValue({
      data: [{ id: 'sched-1', name: 'Jadwal Tagihan', event_type: 'iuran_payment_reminder', is_active: true, send_days: [1], send_hour: 8, send_minute: 0, timezone: 'Asia/Jakarta' }],
      pagination: { page: 1, page_size: 15, total: 1, total_pages: 1 },
    }),
    getSessionPair: vi.fn().mockReturnValue({ accessToken: 'mock-token' }),
  }
})

describe('NotificationScreen', () => {
  const mockSystemTypes = [
    {
      event_type: 'iuran_bill_created',
      name: 'Penerbitan Tagihan Iuran',
      trigger: 'Diterbitkan saat bendahara men-generate tagihan iuran baru',
      channel: 'push',
      recipients_rule: 'Warga yang menempati rumah tangga terkait tagihan',
      default_title: 'Tagihan Iuran Baru Tersedia',
      default_body: 'Tagihan iuran Anda untuk periode terkait telah diterbitkan.',
      allowed_variables: ['bill_id'],
      is_configurable: true,
    },
    {
      event_type: 'waste_truck_arrival',
      name: 'Kedatangan Mobil Sampah',
      trigger: 'Diterbitkan saat petugas sampah tiba di lingkungan RT',
      channel: 'push',
      recipients_rule: 'Seluruh warga RT',
      default_title: 'Mobil Sampah Tiba',
      default_body: 'Truk pengangkut sampah sudah berada di lingkungan RT.',
      allowed_variables: [],
      is_configurable: true,
    },
  ]

  const mockTemplates = [
    {
      id: 'tmpl-1',
      rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      event_type: 'iuran_payment_reminder',
      name: 'Tagihan Bulanan RT 02',
      channel: 'push',
      title: 'Halo Warga, Tagihan Terbit',
      body: 'Segera bayar tagihan nomor {{bill_id}} ya.',
      is_active: true,
      variables: ['bill_id'],
      created_at: '2026-10-29T00:00:00Z',
      updated_at: '2026-10-29T00:00:00Z',
    },
  ]

  const mockLogs = {
    data: [
      {
        id: 'log-1',
        rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        user_id: 'usr-1',
        event_type: 'iuran_bill_created',
        dedup_key: 'dedup-1',
        title: 'Halo Warga, Tagihan Terbit',
        body: 'Segera bayar tagihan nomor 101 ya.',
        status: 'sent',
        created_at: '2026-10-29T01:00:00Z',
      },
    ],
    pagination: {
      page: 1,
      page_size: 15,
      total: 1,
      total_pages: 1,
    },
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'usr-admin',
        name: 'Super Admin',
        email: 'admin@rt.local',
        systemRole: 'super_admin',
        role: 'pengurus',
        jabatan: null,
        rt: null,
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    vi.mocked(api.apiGetNotificationSystemTypes).mockResolvedValue(mockSystemTypes)
    vi.mocked(api.apiGetNotificationTemplates).mockResolvedValue(mockTemplates)
    vi.mocked(api.apiGetNotificationLogs).mockResolvedValue(mockLogs)
    vi.mocked(api.apiGetNotificationManagementHistory).mockResolvedValue(mockLogs)
    vi.mocked(api.apiGetNotificationImageBlob).mockResolvedValue(
      new Blob(['mock-png'], { type: 'image/png' }),
    )
  })

  it('renders correctly with tabs and loads system notification cards', async () => {
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    expect(screen.getByText('Kelola Notifikasi')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Notifikasi Sistem/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Template Kustom/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Riwayat Sistem/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Riwayat Saya/i })).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByText('Penerbitan Tagihan Iuran')).toBeInTheDocument()
      expect(screen.getByText('Kedatangan Mobil Sampah')).toBeInTheDocument()
      expect(screen.getByText('Diterbitkan saat bendahara men-generate tagihan iuran baru')).toBeInTheDocument()
    })
  })

  it('shows only the authenticated warga notification history and preserves SENT', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'warga-1',
        name: 'Warga',
        email: 'warga@example.com',
        systemRole: null,
        role: 'warga',
        jabatan: null,
        rt: { id: 'rt-1', name: 'RT 01' },
        rtId: 'rt-1',
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>,
    )

    expect(await screen.findByText('RIWAYAT NOTIFIKASI')).toBeInTheDocument()
    expect(await screen.findByText('TERKIRIM')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Riwayat Saya/ })).not.toBeInTheDocument()
    expect(api.apiGetNotificationLogs).toHaveBeenCalledWith(
      'mock-token',
      1,
      15,
      undefined,
      undefined,
    )
    expect(api.apiGetNotificationSystemTypes).not.toHaveBeenCalled()
    expect(api.apiGetNotificationTemplates).not.toHaveBeenCalled()
    expect(api.apiGetNotificationManagementHistory).not.toHaveBeenCalled()
  })

  it('navigates to Template Kustom tab and displays templates', async () => {
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Template Kustom/i })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: /Template Kustom/i }))

    await waitFor(() => {
      expect(screen.getByText('Tagihan Bulanan RT 02')).toBeInTheDocument()
      expect(screen.getByText('Halo Warga, Tagihan Terbit')).toBeInTheDocument()
      expect(screen.getByText('Aktif')).toBeInTheDocument()
    })
  })

  it('navigates to Riwayat Sistem and displays system history', async () => {
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    fireEvent.click(screen.getByRole('button', { name: /Riwayat Sistem/i }))

    await waitFor(() => {
      expect(screen.getByText('Total riwayat tersimpan: 1')).toBeInTheDocument()
      expect(screen.getByText('Segera bayar tagihan nomor 101 ya.')).toBeInTheDocument()
      expect(screen.getByText('TERKIRIM')).toBeInTheDocument()
    })
    expect(api.apiGetNotificationManagementHistory).toHaveBeenCalledWith(
      'mock-token',
      'system',
      1,
      15,
    )
  })

  it('loads personal history from a distinct backend category', async () => {
    vi.mocked(api.apiGetNotificationManagementHistory).mockImplementation(
      async (_token, category) => ({
        data: [
          {
            ...mockLogs.data[0],
            id: `log-${category}`,
            body: category === 'mine' ? 'Aktivitas pengurus saya' : 'Notifikasi sistem',
          },
        ],
        pagination: { page: 1, page_size: 15, total: 1, total_pages: 1 },
      }),
    )
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )
    fireEvent.click(screen.getByRole('button', { name: /Riwayat Saya/i }))

    await waitFor(() => {
      expect(screen.getByText('Aktivitas pengurus saya')).toBeInTheDocument()
      expect(screen.queryByText('Notifikasi sistem')).not.toBeInTheDocument()
    })
    expect(api.apiGetNotificationManagementHistory).toHaveBeenCalledWith(
      'mock-token',
      'mine',
      1,
      15,
    )
  })

  it('opens preview banner popup', async () => {
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Penerbitan Tagihan Iuran')).toBeInTheDocument()
    })

    // By-System events are placed at the bottom of the list. Pick the preview
    // button whose card subtree contains the expected system type name.
    const previewButtons = screen.getAllByRole('button', { name: /preview banner/i })
    const billPreviewButton = previewButtons.find((button) => {
      let el: HTMLElement | null = button
      while (el) {
        const text = el.textContent ?? ''
        // Stop at the nearest ancestor that mentions either event name;
        // the shared grid ancestor contains both, so walking past the card would be a false match.
        if (text.includes('Penerbitan Tagihan Iuran') || text.includes('Kedatangan Mobil Sampah')) {
          return text.includes('Penerbitan Tagihan Iuran')
        }
        el = el.parentElement
      }
      return false
    })
    expect(billPreviewButton).toBeDefined()
    fireEvent.click(billPreviewButton as HTMLButtonElement)

    await waitFor(() => {
      expect(screen.getByText(/Preview Push Notification: Penerbitan Tagihan Iuran/i)).toBeInTheDocument()
      expect(screen.getAllByText('Tagihan Iuran Baru Tersedia').length).toBeGreaterThan(0)
    })
  })

  it('triggers test push for Super Admin on waste truck event', async () => {
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Kedatangan Mobil Sampah')).toBeInTheDocument()
    })

    const testButton = screen.getByRole('button', { name: /Test Notifikasi/i })
    expect(testButton).toBeInTheDocument()
    fireEvent.click(testButton)

    await waitFor(() => {
      expect(api.apiTestPush).toHaveBeenCalledWith(
        'mock-token',
        expect.objectContaining({
          event_type: 'waste_truck_arrival',
          title: 'Mobil Sampah Tiba',
          body: 'Truk pengangkut sampah sudah berada di lingkungan RT.',
        })
      )
      expect(screen.getByText(/Uji notifikasi berhasil dikirim/i)).toBeInTheDocument()
    })
  })

  it('opens broadcast confirmation modal and triggers manual push', async () => {
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Kedatangan Mobil Sampah')).toBeInTheDocument()
    })

    const kirimButton = screen.getByRole('button', { name: /Kirim Notifikasi/i })
    fireEvent.click(kirimButton)

    await waitFor(() => {
      expect(screen.getByText(/Konfirmasi Kirim Notifikasi Siaran/i)).toBeInTheDocument()
      expect(screen.getByText(/Apakah Anda yakin ingin menyiarkan notifikasi ini/i)).toBeInTheDocument()
    })

    const confirmButton = screen.getByRole('button', { name: /Ya, Kirim Sekarang/i })
    fireEvent.click(confirmButton)

    await waitFor(() => {
      expect(api.apiTriggerManualPush).toHaveBeenCalledWith(
        'mock-token',
        expect.objectContaining({
          title: 'Mobil Sampah Tiba',
          body: 'Truk pengangkut sampah sudah berada di lingkungan RT.',
        })
      )
      expect(screen.getByText(/Notifikasi berhasil disiarkan ke seluruh warga RT!/i)).toBeInTheDocument()
    })
  })

  it('allows Ketua RT to manage templates and broadcast but hides SuperAdmin-only Test Notifikasi button', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'usr-ketua',
        name: 'Ketua RT 002',
        email: 'ketua-002@local.com',
        systemRole: null,
        role: 'pengurus',
        jabatan: 'ketua',
        effectivePermissions: ['notification.manage'],
        rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'Urena' },
        rtCode: '002',
        rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Kedatangan Mobil Sampah')).toBeInTheDocument()
    })

    // Ketua RT with the RBAC permission retains the full management tabs.
    expect(screen.getByRole('button', { name: /Notifikasi Sistem/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Template Kustom/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Riwayat Sistem/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Riwayat Saya/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Kirim Notifikasi/i })).toBeInTheDocument()
    // Top-right toolbar buttons removed; inside "Template Kustom" tab, the button is present
    fireEvent.click(screen.getByRole('button', { name: /Template Kustom/i }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Tambah Template/i })).toBeInTheDocument()
    })
    // SuperAdmin-only test push button must NOT be shown to Ketua RT
    expect(screen.queryByRole('button', { name: /Test Notifikasi/i })).not.toBeInTheDocument()
  })

  it('renders ONLY Riwayat for Warga role without Sistem or Template tabs', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'usr-warga',
        name: 'Warga Contoh',
        email: 'warga@local.com',
        systemRole: null,
        role: 'warga',
        jabatan: null,
        rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'Urena' },
        rtCode: '002',
        rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    // Header title for warga is "Notifikasi", subtitle is "Riwayat Notifikasi dan Pengumuman Warga RT"
    expect(screen.getByText('Notifikasi')).toBeInTheDocument()
    expect(screen.getByText('Riwayat Notifikasi dan Pengumuman Warga RT')).toBeInTheDocument()

    // Administrative tabs must NOT be present
    expect(screen.queryByRole('button', { name: /Notifikasi Sistem/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Template Kustom/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Tambah Template/i })).not.toBeInTheDocument()

    // Riwayat content is rendered directly
    await waitFor(() => {
      expect(screen.getByText('RIWAYAT NOTIFIKASI')).toBeInTheDocument()
      expect(screen.getByText('Total riwayat tersimpan: 1')).toBeInTheDocument()
      expect(screen.getByText('Segera bayar tagihan nomor 101 ya.')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Tandai Semua Dibaca/i })).toBeInTheDocument()
    })

    // System types and templates endpoints MUST NOT be called for warga
    expect(api.apiGetNotificationSystemTypes).not.toHaveBeenCalled()
    expect(api.apiGetNotificationTemplates).not.toHaveBeenCalled()
    expect(api.apiGetNotificationLogs).toHaveBeenCalled()

    // Test clicking "Tandai Semua Dibaca"
    const markAllBtn = screen.getByRole('button', { name: /Tandai Semua Dibaca/i })
    fireEvent.click(markAllBtn)
    await waitFor(() => {
      expect(api.apiMarkAllNotificationsAsRead).toHaveBeenCalledWith('mock-token')
    })
  })

  it('renders HTTP/HTTPS URLs as clickable external links ONLY for manual_push notifications', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'usr-warga',
        name: 'Warga Contoh',
        email: 'warga@local.com',
        systemRole: null,
        role: 'warga',
        jabatan: null,
        rt: { id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', name: 'Urena' },
        rtCode: '002',
        rtId: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    vi.mocked(api.apiGetNotificationLogs).mockResolvedValue({
      data: [
        {
          id: 'log-manual-https',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'manual_push',
          dedup_key: 'manual_push:a95c1cb5:123',
          title: 'Unduhan Laporan RT',
          body: 'Silakan unduh di https://urena.id/files/laporan-rt.pdf?dl=1&ver=2, atau cek http://rt02.urena.id/info.',
          status: 'sent',
          created_at: '2026-10-29T01:00:00Z',
        },
        {
          id: 'log-manual-plain',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'manual_push',
          dedup_key: 'manual_push:a95c1cb5:124',
          title: 'Kerja Bakti Minggu Ini',
          body: 'Kerja bakti dilaksanakan pukul 07.00 WIB di lapangan RT.',
          status: 'sent',
          created_at: '2026-10-29T02:00:00Z',
        },
        {
          id: 'log-system-url',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'iuran_bill_created',
          dedup_key: 'bill:101',
          title: 'Tagihan Baru',
          body: 'Tagihan terbit, cek https://urena.id/system-should-not-link',
          status: 'sent',
          created_at: '2026-10-29T03:00:00Z',
        },
        {
          id: 'log-scheduler-overdue',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'iuran_overdue_3_months',
          dedup_key: 'schedule:sched-1:2026-10-29:bill-1:usr-warga',
          title: 'Pengingat Tunggakan Iuran',
          body: 'Segera lunasi tunggakan di https://urena.id/scheduler-should-not-link',
          status: 'sent',
          created_at: '2026-10-29T04:00:00Z',
        },
        {
          id: 'log-scheduler-upcoming',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'iuran_upcoming_due_3_months',
          dedup_key: 'schedule:sched-2:2026-10-29:bill-2:usr-warga',
          title: 'Pengingat Jatuh Tempo Iuran',
          body: 'Info lengkap di https://urena.id/upcoming-should-not-link',
          status: 'sent',
          created_at: '2026-10-29T05:00:00Z',
        },
        {
          id: 'log-manual-unsafe',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'manual_push',
          dedup_key: 'manual_push:a95c1cb5:125',
          title: 'Tautan Tidak Valid',
          body: 'Jangan buka javascript:alert(1) atau data:text/html,hi atau file:///etc/passwd atau intent://scan atau market://details atau https://.',
          status: 'sent',
          created_at: '2026-10-29T06:00:00Z',
        },
      ],
      pagination: {
        page: 1,
        page_size: 15,
        total: 6,
        total_pages: 1,
      },
    })

    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Unduhan Laporan RT')).toBeInTheDocument()
    })

    // 1 & 2. Manual notification with https:// and http:// renders clickable links with target="_blank" and rel="noopener noreferrer"
    const httpsLink = screen.getByRole('link', {
      name: 'https://urena.id/files/laporan-rt.pdf?dl=1&ver=2',
    })
    expect(httpsLink).toHaveAttribute('href', 'https://urena.id/files/laporan-rt.pdf?dl=1&ver=2')
    expect(httpsLink).toHaveAttribute('target', '_blank')
    expect(httpsLink).toHaveAttribute('rel', 'noopener noreferrer')

    const httpLink = screen.getByRole('link', {
      name: 'http://rt02.urena.id/info',
    })
    expect(httpLink).toHaveAttribute('href', 'http://rt02.urena.id/info')
    expect(httpLink).toHaveAttribute('target', '_blank')
    expect(httpLink).toHaveAttribute('rel', 'noopener noreferrer')

    // 3. Manual notification without URL renders plain text normally
    expect(
      screen.getByText('Kerja bakti dilaksanakan pukul 07.00 WIB di lapangan RT.')
    ).toBeInTheDocument()

    // 4 & 5 & 6. System notifications & scheduler notifications MUST NOT render links even if body has URL
    expect(
      screen.queryByRole('link', { name: 'https://urena.id/system-should-not-link' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: 'https://urena.id/scheduler-should-not-link' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: 'https://urena.id/upcoming-should-not-link' })
    ).not.toBeInTheDocument()

    // 6. Non-HTTP/HTTPS schemes and malformed URLs MUST NOT become links; only the 2 valid manual links exist
    const allLinks = screen.getAllByRole('link')
    expect(allLinks).toHaveLength(2)
  })

  it('strictly separates Tambah Template and Tambah Jadwal buttons per tab', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: {
        id: 'usr-admin-1',
        name: 'Ketua RT',
        email: 'ketua@local.com',
        systemRole: 'super_admin',
        jabatan: 'ketua',
        permissions: ['notification.manage'],
      } as any,
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Kedatangan Mobil Sampah')).toBeInTheDocument()
    })

    // 1. Navigate to Template Kustom tab
    const templateTabBtn = screen.getByRole('button', { name: /Template Kustom/i })
    fireEvent.click(templateTabBtn)

    await waitFor(() => {
      expect(screen.getByText('Tagihan Bulanan RT 02')).toBeInTheDocument()
    })

    // On tab Template: "Tambah Template" MUST be present, "Tambah Jadwal" MUST NOT be present
    expect(screen.getAllByRole('button', { name: /Tambah Template/i }).length).toBeGreaterThan(0)
    expect(screen.queryByRole('button', { name: /Tambah Jadwal/i })).not.toBeInTheDocument()

    // Clicking Tambah Template opens template creation modal
    const addTemplateBtn = screen.getAllByRole('button', { name: /Tambah Template/i })[0]
    fireEvent.click(addTemplateBtn)
    expect(screen.getByText('Tambah Template Notifikasi')).toBeInTheDocument()

    // Close template modal
    const cancelBtn = screen.getByRole('button', { name: /Batal/i })
    fireEvent.click(cancelBtn)

    // 2. Navigate to Penjadwalan tab
    const scheduleTabBtn = screen.getByRole('button', { name: /Penjadwalan/i })
    fireEvent.click(scheduleTabBtn)

    await waitFor(() => {
      expect(screen.getByText('Jadwal Tagihan')).toBeInTheDocument()
    })

    // On tab Penjadwalan: "Tambah Jadwal" MUST be present, "Tambah Template" MUST NOT be present
    expect(screen.getAllByRole('button', { name: /Tambah Jadwal/i }).length).toBeGreaterThan(0)
    expect(screen.queryByRole('button', { name: /Tambah Template/i })).not.toBeInTheDocument()

    // Clicking Tambah Jadwal opens schedule creation modal
    const addScheduleBtn = screen.getAllByRole('button', { name: /Tambah Jadwal/i })[0]
    fireEvent.click(addScheduleBtn)
    expect(screen.getByText('Buat Jadwal Baru')).toBeInTheDocument()
  })

  it('deduplicates in-flight mark-read requests for the same log id and still allows different ids in parallel', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: { id: 'usr-warga', name: 'Warga RT', email: 'warga@rt.local', systemRole: 'member', role: 'warga', jabatan: null, rt: null },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })

    vi.mocked(api.apiGetNotificationLogs).mockResolvedValue({
      data: [
        {
          id: 'log-1',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'iuran_bill_created',
          dedup_key: 'dedup-1',
          title: 'Tagihan Satu',
          body: 'Body satu',
          status: 'sent',
          is_read: false,
          created_at: '2026-10-29T01:00:00Z',
        },
        {
          id: 'log-2',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'iuran_bill_created',
          dedup_key: 'dedup-2',
          title: 'Tagihan Dua',
          body: 'Body dua',
          status: 'sent',
          is_read: false,
          created_at: '2026-10-29T02:00:00Z',
        },
      ],
      pagination: { page: 1, page_size: 15, total: 2, total_pages: 1 },
    } as any)

    let resolveFirst: (() => void) | null = null
    let resolveSecond: (() => void) | null = null
    let callCount = 0
    vi.mocked(api.apiMarkNotificationAsRead).mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          callCount += 1
          if (callCount === 1) resolveFirst = resolve
          else resolveSecond = resolve
        }),
    )

    render(
      <MemoryRouter>
        <AuthModule.AuthProvider>
          <NotificationScreen />
        </AuthModule.AuthProvider>
      </MemoryRouter>,
    )

    const detailButtons = await screen.findAllByRole('button', { name: /Detail/i })
    expect(detailButtons.length).toBeGreaterThanOrEqual(2)

    fireEvent.click(detailButtons[0])
    fireEvent.click(detailButtons[0])
    await waitFor(() => {
      expect(vi.mocked(api.apiMarkNotificationAsRead)).toHaveBeenCalledTimes(1)
    })

    fireEvent.click(detailButtons[1])
    await waitFor(() => {
      expect(vi.mocked(api.apiMarkNotificationAsRead)).toHaveBeenCalledTimes(2)
    })

    expect(resolveFirst).toBeTruthy()
    expect(resolveSecond).toBeTruthy()
    resolveFirst!()
    resolveSecond!()

    await waitFor(() => {
      expect(screen.getAllByText('Sudah Dibaca').length).toBeGreaterThanOrEqual(2)
    })
  })

  it('keeps notification unread when mark-as-read API fails and clears in-flight guard for retry', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: { id: 'usr-warga', name: 'Warga RT', email: 'warga@rt.local', systemRole: 'member', role: 'warga', jabatan: null, rt: null },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    vi.mocked(api.apiGetNotificationLogs).mockResolvedValue({
      data: [
        {
          id: 'log-fail',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'iuran_bill_created',
          dedup_key: 'dedup-fail',
          title: 'Tagihan Gagal',
          body: 'Body gagal',
          status: 'sent',
          is_read: false,
          created_at: '2026-10-29T01:00:00Z',
        },
      ],
      pagination: { page: 1, page_size: 15, total: 1, total_pages: 1 },
    } as any)
    vi.mocked(api.apiMarkNotificationAsRead)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(undefined as any)

    render(
      <MemoryRouter>
        <AuthModule.AuthProvider>
          <NotificationScreen />
        </AuthModule.AuthProvider>
      </MemoryRouter>,
    )

    const detailButton = await screen.findByRole('button', { name: /Detail/i })
    fireEvent.click(detailButton)
    await waitFor(() => {
      expect(vi.mocked(api.apiMarkNotificationAsRead)).toHaveBeenCalledTimes(1)
    })
    expect(screen.getAllByText('Belum Dibaca').length).toBeGreaterThanOrEqual(1)

    fireEvent.click(detailButton)
    await waitFor(() => {
      expect(vi.mocked(api.apiMarkNotificationAsRead)).toHaveBeenCalledTimes(2)
    })
    await waitFor(() => {
      expect(screen.getAllByText('Sudah Dibaca').length).toBeGreaterThanOrEqual(1)
    })
  })

  it('does not update state or throw after unmount while mark-as-read is in flight', async () => {
    vi.mocked(AuthModule.useAuth).mockReturnValue({
      user: { id: 'usr-warga', name: 'Warga RT', email: 'warga@rt.local', systemRole: 'member', role: 'warga', jabatan: null, rt: null },
      isAuthenticated: true,
      isInitializing: false,
      login: async () => {},
      logout: () => {},
    })
    vi.mocked(api.apiGetNotificationLogs).mockResolvedValue({
      data: [
        {
          id: 'log-unmount',
          rt_id: 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          user_id: 'usr-warga',
          event_type: 'iuran_bill_created',
          dedup_key: 'dedup-unmount',
          title: 'Tagihan Unmount',
          body: 'Body unmount',
          status: 'sent',
          is_read: false,
          created_at: '2026-10-29T01:00:00Z',
        },
      ],
      pagination: { page: 1, page_size: 15, total: 1, total_pages: 1 },
    } as any)

    let resolveRead: (() => void) | undefined
    vi.mocked(api.apiMarkNotificationAsRead).mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveRead = resolve
        }),
    )

    const view = render(
      <MemoryRouter>
        <AuthModule.AuthProvider>
          <NotificationScreen />
        </AuthModule.AuthProvider>
      </MemoryRouter>,
    )

    const detailButton = await screen.findByRole('button', { name: /Detail/i })
    fireEvent.click(detailButton)
    await waitFor(() => {
      expect(vi.mocked(api.apiMarkNotificationAsRead)).toHaveBeenCalledTimes(1)
      expect(resolveRead).toBeTruthy()
    })

    view.unmount()
    resolveRead?.()
    await Promise.resolve()
    await Promise.resolve()
  })

  it('loads uploaded create-template image via authenticated blob loader and revokes on remove', async () => {
    const uploadedUrl =
      '/api/v1/notifications/images/notifications/manual/43d1ed42-d982-4299-9b29-30080591fcf9/upload.png'
    vi.mocked(api.apiUploadNotificationImage).mockResolvedValue({
      status: 'uploaded',
      image_url: uploadedUrl,
      key: 'notifications/manual/43d1ed42-d982-4299-9b29-30080591fcf9/upload.png',
    })
    const createUrlSpy = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:http://localhost/template-preview-1')
    const revokeUrlSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})

    try {
      render(
        <MemoryRouter>
          <NotificationScreen />
        </MemoryRouter>,
      )

      fireEvent.click(screen.getByRole('button', { name: /Template Kustom/i }))
      const createBtn = await screen.findByRole('button', { name: /Tambah Template/i })
      fireEvent.click(createBtn)

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
      expect(fileInput).toBeTruthy()
      const file = new File(['png'], 'banner.png', { type: 'image/png' })
      fireEvent.change(fileInput, { target: { files: [file] } })

      const previewImg = await screen.findByRole('img', { name: 'Preview Lampiran' })
      expect(previewImg).toHaveAttribute('src', 'blob:http://localhost/template-preview-1')
      expect(previewImg.getAttribute('src')).not.toBe(uploadedUrl)
      expect(vi.mocked(api.apiGetNotificationImageBlob)).toHaveBeenCalledWith(
        uploadedUrl,
        expect.any(AbortSignal),
      )

      // Removing image immediately removes preview and revokes object URL
      fireEvent.click(screen.getByTitle('Hapus gambar'))
      await waitFor(() => {
        expect(screen.queryByRole('img', { name: 'Preview Lampiran' })).not.toBeInTheDocument()
      })
      expect(revokeUrlSpy).toHaveBeenCalledWith('blob:http://localhost/template-preview-1')
    } finally {
      createUrlSpy.mockRestore()
      revokeUrlSpy.mockRestore()
    }
  })

  it('renders existing saved template image via authenticated loader in edit and preview modals', async () => {
    const savedImageUrl =
      '/api/v1/notifications/images/notifications/manual/43d1ed42-d982-4299-9b29-30080591fcf9/saved.png'
    vi.mocked(api.apiGetNotificationTemplates).mockResolvedValue([
      {
        ...mockTemplates[0],
        id: 'tmpl-img-1',
        event_type: 'manual_push',
        name: 'Pengumuman Kerja Bakti',
        title: 'Kerja Bakti Minggu',
        body: 'Mari ikut kerja bakti jam 07.00',
        image_url: savedImageUrl,
      },
    ])
    const createUrlSpy = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:http://localhost/saved-template-img')
    const revokeUrlSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})

    try {
      render(
        <MemoryRouter>
          <NotificationScreen />
        </MemoryRouter>,
      )

      fireEvent.click(screen.getByRole('button', { name: /Template Kustom/i }))
      await screen.findByText('Pengumuman Kerja Bakti')

      // Open Preview modal
      fireEvent.click(screen.getByTitle('Lihat Tampilan Banner'))
      const bannerImg = await screen.findByRole('img', { name: 'Banner Notifikasi' })
      expect(bannerImg).toHaveAttribute('src', 'blob:http://localhost/saved-template-img')
      expect(bannerImg.getAttribute('src')).not.toBe(savedImageUrl)
      expect(vi.mocked(api.apiGetNotificationImageBlob)).toHaveBeenCalledWith(
        savedImageUrl,
        expect.any(AbortSignal),
      )

      fireEvent.click(screen.getByRole('button', { name: /Tutup Preview/i }))
      expect(revokeUrlSpy).toHaveBeenCalledWith('blob:http://localhost/saved-template-img')

      // Open Edit modal without replacing image
      fireEvent.click(screen.getByTitle('Edit template'))
      const editImg = await screen.findByRole('img', { name: 'Preview Lampiran' })
      expect(editImg).toHaveAttribute('src', 'blob:http://localhost/saved-template-img')
      expect(editImg.getAttribute('src')).not.toBe(savedImageUrl)
    } finally {
      createUrlSpy.mockRestore()
      revokeUrlSpy.mockRestore()
    }
  })
})


describe('notificationUrl helpers', () => {
  it('identifies manual notifications strictly from trusted event_type and dedup_key', () => {
    expect(isManualNotification('manual_push', 'manual_push:rt1:1')).toBe(true)
    expect(isManualNotification('manual_push', null)).toBe(true)
    expect(isManualNotification('manual_push', 'schedule:sched-1:2026-10-29')).toBe(false)
    expect(isManualNotification('iuran_bill_created', 'dedup-1')).toBe(false)
    expect(isManualNotification('iuran_upcoming_due_3_months', 'schedule:1')).toBe(false)
    expect(isManualNotification('iuran_overdue_3_months', 'schedule:2')).toBe(false)
    expect(isManualNotification('waste_truck_arrival', null)).toBe(false)
  })

  it('validates only safe http/https URLs and preserves query strings intact', () => {
    expect(
      validateSafeHttpUrl('https://example.com/download?file=dokumen_rt.pdf&token=abc-123')
    ).toBe('https://example.com/download?file=dokumen_rt.pdf&token=abc-123')
    expect(validateSafeHttpUrl('http://192.168.1.10:8080/status')).toBe(
      'http://192.168.1.10:8080/status'
    )
    expect(validateSafeHttpUrl('javascript:alert(1)')).toBeNull()
    expect(validateSafeHttpUrl('data:text/html;base64,PHNjcmlwdD4=')).toBeNull()
    expect(validateSafeHttpUrl('file:///sdcard/Download/a.pdf')).toBeNull()
    expect(validateSafeHttpUrl('intent://scan/#Intent;scheme=zxing;end')).toBeNull()
    expect(validateSafeHttpUrl('market://details?id=com.example')).toBeNull()
    expect(validateSafeHttpUrl('https://')).toBeNull()
    expect(validateSafeHttpUrl('http://')).toBeNull()
    expect(validateSafeHttpUrl('https://.')).toBeNull()
    expect(validateSafeHttpUrl('http:///path')).toBeNull()
    expect(validateSafeHttpUrl('')).toBeNull()
  })

  it('parses text segments accurately with balanced parentheses and trailing punctuation', () => {
    const segments = parseNotificationTextSegments(
      'Unduh (https://example.com/doc_(v1).pdf?dl=1). Jangan klik javascript:alert(1)!',
      true
    )
    expect(segments).toEqual([
      { type: 'text', text: 'Unduh (' },
      {
        type: 'link',
        text: 'https://example.com/doc_(v1).pdf?dl=1',
        url: 'https://example.com/doc_(v1).pdf?dl=1',
      },
      { type: 'text', text: '). Jangan klik javascript:alert(1)!' },
    ])
  })


})