import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { NotificationScreen } from './NotificationScreen'
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
})
