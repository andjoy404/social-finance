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
    apiUploadNotificationImage: vi.fn(),
    apiTriggerManualPush: vi.fn().mockResolvedValue({ status: 'accepted', message: 'dispatched' }),
    apiTestPush: vi.fn().mockResolvedValue({ status: 'dispatched', message: 'dispatched', dispatched_count: 1 }),
    apiMarkNotificationAsRead: vi.fn().mockResolvedValue(undefined),
    apiMarkAllNotificationsAsRead: vi.fn().mockResolvedValue({ marked: 1 }),
    apiGetNotificationUnreadCount: vi.fn().mockResolvedValue({ unread_count: 1 }),
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
      recipients_rule: 'Warga kepala keluarga dan anggota KK yang bersangkutan',
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
      event_type: 'iuran_bill_created',
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
    vi.mocked(api.apiGetNotificationSystemTypes).mockResolvedValue(mockSystemTypes)
    vi.mocked(api.apiGetNotificationTemplates).mockResolvedValue(mockTemplates)
    vi.mocked(api.apiGetNotificationLogs).mockResolvedValue(mockLogs)
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
    expect(screen.getByRole('button', { name: /Riwayat/i })).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByText('Penerbitan Tagihan Iuran')).toBeInTheDocument()
      expect(screen.getByText('Kedatangan Mobil Sampah')).toBeInTheDocument()
      expect(screen.getByText('Diterbitkan saat bendahara men-generate tagihan iuran baru')).toBeInTheDocument()
    })
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

  it('navigates to Riwayat tab and displays logs', async () => {
    render(
      <MemoryRouter>
        <NotificationScreen />
      </MemoryRouter>
    )

    fireEvent.click(screen.getByRole('button', { name: /Riwayat/i }))

    await waitFor(() => {
      expect(screen.getByText('Total riwayat tersimpan: 1')).toBeInTheDocument()
      expect(screen.getByText('Segera bayar tagihan nomor 101 ya.')).toBeInTheDocument()
      expect(screen.getByText('TERKIRIM')).toBeInTheDocument()
    })
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

    const previewButtons = screen.getAllByRole('button', { name: /preview banner/i })
    fireEvent.click(previewButtons[0])

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

    // Ketua RT can broadcast and manage templates
    expect(screen.getByRole('button', { name: /Kirim Notifikasi/i })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Tambah Template/i }).length).toBeGreaterThan(0)
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
      expect(screen.getByText('Riwayat Notifikasi')).toBeInTheDocument()
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
