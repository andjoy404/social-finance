import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import '@testing-library/jest-dom'
import * as AuthModule from '@/app/AuthContext'
import type { AuthUser } from '@/app/AuthContext'
import * as SecurityApi from './securityApi'
import { KelolaKeamananPage } from './KelolaKeamananPage'

vi.mock('./securityApi', async () => {
  const actual = await vi.importActual<typeof import('./securityApi')>('./securityApi')
  return {
    ...actual,
    apiGetSecurityDashboard: vi.fn(),
    apiListSecurityOfficers: vi.fn(),
    apiListSecurityShifts: vi.fn(),
    apiCreateSecurityShift: vi.fn(),
    apiUpdateSecurityShift: vi.fn(),
    apiListSecuritySchedules: vi.fn(),
    apiCreateSecuritySchedule: vi.fn(),
    apiGetSecuritySchedule: vi.fn(),
    apiUpdateSecuritySchedule: vi.fn(),
    apiListSecuritySwaps: vi.fn(),
    apiListSecurityAttendance: vi.fn(),
    apiListSecurityCorrections: vi.fn(),
    apiListSecurityDevices: vi.fn(),
    apiCreateSecurityDevice: vi.fn(),
    apiListSecurityGuestBooks: vi.fn(),
  }
})

const ketuaUser: AuthUser = {
  id: 'ketua-1',
  name: 'Ketua RT 002',
  email: 'ketua@example.com',
  systemRole: null,
  role: 'pengurus',
  jabatan: 'ketua',
  effectivePermissions: ['security.schedule.write', 'security.device.manage', 'security.correction'],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
}

const wargaUser: AuthUser = {
  id: 'warga-1',
  name: 'Warga Biasa',
  email: 'warga@example.com',
  systemRole: null,
  role: 'warga',
  jabatan: null,
  effectivePermissions: [],
  rt: { id: 'rt-002', name: 'RT 002' },
  rtCode: '002',
  rtId: 'rt-002',
}

describe('KelolaKeamananPage', () => {
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
    vi.mocked(SecurityApi.apiGetSecurityDashboard).mockResolvedValue({
      server_time: '2026-11-12T10:00:00Z',
      today_date: '2026-11-12',
      total_officers: 2,
      active_shifts_count: 2,
      scheduled_today_count: 1,
      present_today_count: 1,
      late_today_count: 0,
      checked_out_today_count: 0,
      absent_today_count: 0,
      missing_checkout_count: 0,
      pending_swaps_count: 1,
      pending_corrections: 1,
      active_devices_count: 1,
      active_guests_count: 2,
      today_attendance: [],
    })
    vi.mocked(SecurityApi.apiListSecurityOfficers).mockResolvedValue([
      {
        user_id: 'sec-1',
        membership_id: 'mem-1',
        rt_id: 'rt-002',
        full_name: 'Pak Budi Satpam',
        email: 'budi@example.com',
        phone: '081234567890',
        is_active: true,
      },
    ])
    vi.mocked(SecurityApi.apiListSecurityShifts).mockResolvedValue([])
    vi.mocked(SecurityApi.apiCreateSecurityShift).mockResolvedValue({} as SecurityApi.ShiftConfig)
    vi.mocked(SecurityApi.apiUpdateSecurityShift).mockResolvedValue({} as SecurityApi.ShiftConfig)
    vi.mocked(SecurityApi.apiCreateSecuritySchedule).mockResolvedValue({} as SecurityApi.SecuritySchedule)
    vi.mocked(SecurityApi.apiGetSecuritySchedule).mockResolvedValue([])
    vi.mocked(SecurityApi.apiUpdateSecuritySchedule).mockResolvedValue({} as SecurityApi.SecuritySchedule)
    vi.mocked(SecurityApi.apiListSecuritySchedules).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    })
    vi.mocked(SecurityApi.apiListSecuritySwaps).mockResolvedValue([])
    vi.mocked(SecurityApi.apiListSecurityAttendance).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    })
    vi.mocked(SecurityApi.apiListSecurityCorrections).mockResolvedValue([
      {
        id: 'corr-self',
        rt_id: 'rt-002',
        attendance_record_id: 'att-1',
        schedule_id: 'sch-1',
        officer_user_id: 'sec-1',
        officer_name: 'Pak Budi Satpam',
        requested_by: 'ketua-1',
        requested_by_name: 'Ketua RT 002',
        shift_date: '2026-11-12',
        shift_name: 'Shift Malam',
        request_type: 'status_change',
        original_status: 'absent',
        requested_status: 'present',
        reason: 'Lupa scan saat serah terima',
        status: 'pending',
        created_at: '2026-11-12T08:00:00Z',
      },
    ])
    vi.mocked(SecurityApi.apiListSecurityDevices).mockResolvedValue([])
    vi.mocked(SecurityApi.apiListSecurityGuestBooks).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    })
  })

  it('opens schedule form with inclusive start and end date fields', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({ user: ketuaUser, loading: false, login: vi.fn(), logout: vi.fn(), refreshUser: vi.fn() } as never)
    render(<KelolaKeamananPage />)
    fireEvent.click(await screen.findByText('Jadwal Petugas'))
    fireEvent.click(await screen.findByText('Tambah Jadwal'))
    expect(screen.getByText('Tanggal Mulai Tugas')).toBeInTheDocument()
    expect(screen.getByText('Tanggal Berakhir Tugas')).toBeInTheDocument()
    expect(screen.queryByText('Ubah Warga')).not.toBeInTheDocument()
  })

  it('denies access for unauthorized warga user', () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: wargaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as never)

    render(<KelolaKeamananPage />)
    expect(screen.getByText('Akses Ditolak')).toBeInTheDocument()
    expect(SecurityApi.apiGetSecurityDashboard).not.toHaveBeenCalled()
  })

  it('renders dashboard KPIs and prevents self-approval on own correction request', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as never)

    render(<KelolaKeamananPage />)
    await waitFor(() => {
      expect(screen.getByText('Kelola Keamanan RT')).toBeInTheDocument()
      expect(screen.getByText('Tamu Masih Berkunjung')).toBeInTheDocument()
    })

    const koreksiTab = screen.getByText('Koreksi Absensi')
    fireEvent.click(koreksiTab)

    await waitFor(() => {
      expect(screen.getByText('Tidak bisa menyetujui pengajuan sendiri')).toBeInTheDocument()
    })
  })

  it('registers pos device and shows one-time pairing token', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as never)
    vi.mocked(SecurityApi.apiCreateSecurityDevice).mockResolvedValue({
      device: {
        id: 'dev-1',
        rt_id: 'rt-002',
        device_name: 'Tablet Pos Gerbang',
        location_label: 'Gerbang Utama',
        is_active: true,
        is_paired: false,
        created_at: '2026-11-12T08:00:00Z',
        updated_at: '2026-11-12T08:00:00Z',
      },
      pairing_token: 'PAIR-SECRET-XYZ',
      expires_at: '2026-11-13T08:00:00Z',
    })

    render(<KelolaKeamananPage />)
    fireEvent.click(screen.getByText('Perangkat Pos'))
    await waitFor(() => {
      expect(screen.getByText('Daftarkan Perangkat Pos')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Daftarkan Perangkat Pos'))
    fireEvent.change(screen.getByPlaceholderText('Contoh: Tablet Pos Gerbang Utama'), {
      target: { value: 'Tablet Pos Gerbang' },
    })
    fireEvent.change(screen.getByPlaceholderText('Contoh: Gerbang Utama RT 002'), {
      target: { value: 'Gerbang Utama' },
    })
    fireEvent.click(screen.getByText('Buat Token Pairing'))

    await waitFor(() => {
      expect(screen.getByText('PAIR-SECRET-XYZ')).toBeInTheDocument()
      expect(screen.getByText('Token ini hanya ditampilkan satu kali.')).toBeInTheDocument()
    })
  })

  it('displays backend HH:MM:SS as HH:MM in shift list table without seconds', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as never)
    vi.mocked(SecurityApi.apiListSecurityShifts).mockResolvedValue([
      {
        id: 'shift-1',
        rt_id: 'rt-002',
        name: 'Shift Pagi',
        start_time: '08:00:00',
        end_time: '16:00:00',
        crosses_midnight: false,
        lateness_tolerance_minutes: 15,
        check_in_window_before_minutes: 30,
        check_in_window_after_minutes: 60,
        absent_cutoff_minutes: 60,
        missing_checkout_cutoff_minutes: 120,
        is_active: true,
        version: 1,
        created_at: '2026-11-12T00:00:00Z',
        updated_at: '2026-11-12T00:00:00Z',
      },
      {
        id: 'shift-2',
        rt_id: 'rt-002',
        name: 'Shift Malam',
        start_time: '23:00:00',
        end_time: '07:00:00',
        crosses_midnight: true,
        lateness_tolerance_minutes: 15,
        check_in_window_before_minutes: 30,
        check_in_window_after_minutes: 60,
        absent_cutoff_minutes: 60,
        missing_checkout_cutoff_minutes: 120,
        is_active: true,
        version: 1,
        created_at: '2026-11-12T00:00:00Z',
        updated_at: '2026-11-12T00:00:00Z',
      },
    ])

    render(<KelolaKeamananPage />)
    fireEvent.click(await screen.findByText('Pengaturan Shift'))

    await waitFor(() => {
      expect(screen.getByText('Shift Pagi')).toBeInTheDocument()
      expect(screen.getByText('08:00 - 16:00 WIB')).toBeInTheDocument()
      expect(screen.getByText('Shift Malam')).toBeInTheDocument()
      expect(screen.getByText('23:00 - 07:00 WIB')).toBeInTheDocument()
    })
  })

  it('Ubah Shift normalizes backend HH:MM:SS and sends HH:MM payload without ISO-8601 timestamp', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as never)
    vi.mocked(SecurityApi.apiListSecurityShifts).mockResolvedValue([
      {
        id: 'shift-1',
        rt_id: 'rt-002',
        name: 'Shift Pagi',
        start_time: '08:00:00',
        end_time: '16:00:00',
        crosses_midnight: false,
        lateness_tolerance_minutes: 15,
        check_in_window_before_minutes: 30,
        check_in_window_after_minutes: 60,
        absent_cutoff_minutes: 60,
        missing_checkout_cutoff_minutes: 120,
        is_active: true,
        version: 1,
        created_at: '2026-11-12T00:00:00Z',
        updated_at: '2026-11-12T00:00:00Z',
      },
    ])

    render(<KelolaKeamananPage />)
    fireEvent.click(await screen.findByText('Pengaturan Shift'))
    await waitFor(() => {
      expect(screen.getByText('Shift Pagi')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Ubah'))
    await waitFor(() => {
      expect(screen.getByText('Ubah Shift: Shift Pagi')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(SecurityApi.apiUpdateSecurityShift).toHaveBeenCalledTimes(1)
    })

    const [id, payload] = vi.mocked(SecurityApi.apiUpdateSecurityShift).mock.calls[0]
    expect(id).toBe('shift-1')
    expect(payload.name).toBe('Shift Pagi')
    expect(payload.start_time).toBe('08:00')
    expect(payload.end_time).toBe('16:00')

    // Verify payload serialization has no ISO-8601 timestamps
    const jsonStr = JSON.stringify(payload)
    expect(jsonStr).not.toContain('T')
    expect(jsonStr).not.toContain('Z')
    expect(jsonStr).not.toMatch(/\d{4}-\d{2}-\d{2}T/)
  })

  it('Tambah Shift sends HH:MM payload without ISO-8601 timestamp', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({
      user: ketuaUser,
      loading: false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    } as never)

    render(<KelolaKeamananPage />)
    fireEvent.click(await screen.findByText('Pengaturan Shift'))
    await waitFor(() => {
      expect(screen.getByText('Tambah Shift')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Tambah Shift'))
    await waitFor(() => {
      expect(screen.getByText('Tambah Shift Keamanan')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByPlaceholderText('Contoh: Shift Malam'), {
      target: { value: 'Shift Baru' },
    })

    fireEvent.click(screen.getByText('Simpan'))

    await waitFor(() => {
      expect(SecurityApi.apiCreateSecurityShift).toHaveBeenCalledTimes(1)
    })

    const payload = vi.mocked(SecurityApi.apiCreateSecurityShift).mock.calls[0][0]
    expect(payload.name).toBe('Shift Baru')
    expect(payload.start_time).toBe('20:00')
    expect(payload.end_time).toBe('06:00')

    // Verify payload serialization has no ISO-8601 timestamps
    const jsonStr = JSON.stringify(payload)
    expect(jsonStr).not.toContain('T')
    expect(jsonStr).not.toContain('Z')
    expect(jsonStr).not.toMatch(/\d{4}-\d{2}-\d{2}T/)
  })
})

describe('KelolaKeamananPage - Ubah Jadwal date normalization', () => {
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
    vi.mocked(SecurityApi.apiGetSecurityDashboard).mockResolvedValue({
      server_time: '2026-11-12T10:00:00Z',
      today_date: '2026-11-12',
      total_officers: 1,
      active_shifts_count: 1,
      scheduled_today_count: 1,
      present_today_count: 0,
      late_today_count: 0,
      checked_out_today_count: 0,
      absent_today_count: 0,
      missing_checkout_count: 0,
      pending_swaps_count: 0,
      pending_corrections: 0,
      active_devices_count: 0,
      active_guests_count: 0,
      today_attendance: [],
    })
    vi.mocked(SecurityApi.apiListSecurityOfficers).mockResolvedValue([
      {
        user_id: 'sec-1',
        membership_id: 'mem-1',
        rt_id: 'rt-002',
        full_name: 'Pak Budi Satpam',
        email: 'budi@example.com',
        phone: '081234567890',
        is_active: true,
      },
    ])
    vi.mocked(SecurityApi.apiListSecurityShifts).mockResolvedValue([
      {
        id: 'shift-1',
        rt_id: 'rt-002',
        name: 'Shift Pagi',
        start_time: '08:00:00',
        end_time: '16:00:00',
        crosses_midnight: false,
        lateness_tolerance_minutes: 15,
        check_in_window_before_minutes: 30,
        check_in_window_after_minutes: 60,
        absent_cutoff_minutes: 60,
        missing_checkout_cutoff_minutes: 120,
        is_active: true,
        version: 1,
        created_at: '2026-11-12T00:00:00Z',
        updated_at: '2026-11-12T00:00:00Z',
      },
    ])
    vi.mocked(SecurityApi.apiListSecuritySchedules).mockResolvedValue({
      data: [
        {
          id: 'sch-1',
          assignment_id: 'asg-1',
          assignment_start_date: '2026-11-13',
          assignment_end_date: '2026-11-20',
          rt_id: 'rt-002',
          user_id: 'sec-1',
          officer_name: 'Pak Budi Satpam',
          membership_id: 'mem-1',
          shift_config_id: 'shift-1',
          shift_version: 1,
          shift_date: '2026-11-13',
          shift_name: 'Shift Pagi',
          start_time: '08:00',
          end_time: '16:00',
          crosses_midnight: false,
          lateness_tolerance_minutes: 15,
          scheduled_start_at: '2026-11-13T01:00:00Z',
          scheduled_end_at: '2026-11-13T09:00:00Z',
          status: 'scheduled',
          attendance_status: 'belum_hadir',
        },
      ],
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    })
    vi.mocked(SecurityApi.apiGetSecuritySchedule).mockResolvedValue([
      {
        id: 'sch-1',
        assignment_id: 'asg-1',
        assignment_start_date: '2026-11-13',
        assignment_end_date: '2026-11-20',
        rt_id: 'rt-002',
        user_id: 'sec-1',
        officer_name: 'Pak Budi Satpam',
        membership_id: 'mem-1',
        shift_config_id: 'shift-1',
        shift_version: 1,
        shift_date: '2026-11-13',
        shift_name: 'Shift Pagi',
        start_time: '08:00',
        end_time: '16:00',
        crosses_midnight: false,
        lateness_tolerance_minutes: 15,
        scheduled_start_at: '2026-11-13T01:00:00Z',
        scheduled_end_at: '2026-11-13T09:00:00Z',
        status: 'scheduled',
        attendance_status: 'belum_hadir',
      },
    ])
    vi.mocked(SecurityApi.apiUpdateSecuritySchedule).mockResolvedValue({} as SecurityApi.SecuritySchedule)
    vi.mocked(SecurityApi.apiListSecuritySwaps).mockResolvedValue([])
    vi.mocked(SecurityApi.apiListSecurityAttendance).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    })
    vi.mocked(SecurityApi.apiListSecurityCorrections).mockResolvedValue([])
    vi.mocked(SecurityApi.apiListSecurityDevices).mockResolvedValue([])
    vi.mocked(SecurityApi.apiListSecurityGuestBooks).mockResolvedValue({
      data: [],
      pagination: { page: 1, per_page: 20, total: 0, total_pages: 1 },
    })
  })

  it('opens Ubah Jadwal with YYYY-MM-DD dates and saves unchanged dates as YYYY-MM-DD', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({ user: ketuaUser, loading: false, login: vi.fn(), logout: vi.fn(), refreshUser: vi.fn() } as never)
    render(<KelolaKeamananPage />)
    fireEvent.click(await screen.findByText('Jadwal Petugas'))
    await waitFor(() => expect(screen.getByText('Pak Budi Satpam')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Ubah'))
    await waitFor(() => expect(screen.getByText('Ubah Jadwal Petugas Keamanan')).toBeInTheDocument())
    // Dates should render without crash in formatted inputs
    expect(screen.getByDisplayValue('13/11/2026')).toBeInTheDocument()
    expect(screen.getByDisplayValue('20/11/2026')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Simpan Perubahan'))
    await waitFor(() => expect(SecurityApi.apiUpdateSecuritySchedule).toHaveBeenCalledTimes(1))
    const [id, payload] = vi.mocked(SecurityApi.apiUpdateSecuritySchedule).mock.calls[0]
    expect(id).toBe('sch-1')
    expect(payload.start_date).toBe('2026-11-13')
    expect(payload.end_date).toBe('2026-11-20')
    expect(JSON.stringify(payload)).not.toMatch(/\d{4}-\d{2}-\d{2}T/)
  })

  it('opens Ubah Jadwal with ISO timestamp dates without crash and keeps calendar day', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({ user: ketuaUser, loading: false, login: vi.fn(), logout: vi.fn(), refreshUser: vi.fn() } as never)
    vi.mocked(SecurityApi.apiListSecuritySchedules).mockResolvedValue({
      data: [{
        id: 'sch-iso', assignment_id: 'asg-iso', assignment_start_date: '2026-11-13T17:00:00.000Z', assignment_end_date: '2026-11-20T17:00:00.000Z',
        rt_id: 'rt-002', user_id: 'sec-1', officer_name: 'Pak Budi Satpam', membership_id: 'mem-1', shift_config_id: 'shift-1', shift_version: 1,
        shift_date: '2026-11-13', shift_name: 'Shift Pagi', start_time: '08:00', end_time: '16:00', crosses_midnight: false, lateness_tolerance_minutes: 15,
        scheduled_start_at: '2026-11-13T01:00:00Z', scheduled_end_at: '2026-11-13T09:00:00Z', status: 'scheduled', attendance_status: 'belum_hadir',
      } as SecurityApi.SecuritySchedule],
      pagination: { page: 1, per_page: 20, total: 1, total_pages: 1 },
    })
    vi.mocked(SecurityApi.apiGetSecuritySchedule).mockResolvedValue([{
      id: 'sch-iso', assignment_id: 'asg-iso', assignment_start_date: '2026-11-13T17:00:00.000Z', assignment_end_date: '2026-11-20T17:00:00.000Z',
      rt_id: 'rt-002', user_id: 'sec-1', officer_name: 'Pak Budi Satpam', membership_id: 'mem-1', shift_config_id: 'shift-1', shift_version: 1,
      shift_date: '2026-11-13', shift_name: 'Shift Pagi', start_time: '08:00', end_time: '16:00', crosses_midnight: false, lateness_tolerance_minutes: 15,
      scheduled_start_at: '2026-11-13T01:00:00Z', scheduled_end_at: '2026-11-13T09:00:00Z', status: 'scheduled', attendance_status: 'belum_hadir',
    } as SecurityApi.SecuritySchedule])
    render(<KelolaKeamananPage />)
    fireEvent.click(await screen.findByText('Jadwal Petugas'))
    await waitFor(() => expect(screen.getByText('Pak Budi Satpam')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Ubah'))
    await waitFor(() => expect(screen.getByText('Ubah Jadwal Petugas Keamanan')).toBeInTheDocument())
    // Calendar date must not shift
    expect(screen.getByDisplayValue('13/11/2026')).toBeInTheDocument()
    expect(screen.getByDisplayValue('20/11/2026')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Simpan Perubahan'))
    await waitFor(() => expect(SecurityApi.apiUpdateSecuritySchedule).toHaveBeenCalled())
    const [, payload] = vi.mocked(SecurityApi.apiUpdateSecuritySchedule).mock.calls[0]
    expect(payload.start_date).toBe('2026-11-13')
    expect(payload.end_date).toBe('2026-11-20')
  })

  it('opens Ubah Jadwal safely when dates are empty/null and does not crash', async () => {
    vi.spyOn(AuthModule, 'useAuth').mockReturnValue({ user: ketuaUser, loading: false, login: vi.fn(), logout: vi.fn(), refreshUser: vi.fn() } as never)
    vi.mocked(SecurityApi.apiGetSecuritySchedule).mockResolvedValue([{
      id: 'sch-null', assignment_id: 'asg-null', assignment_start_date: '' as unknown as string, assignment_end_date: null as unknown as string,
      rt_id: 'rt-002', user_id: 'sec-1', officer_name: 'Pak Budi Satpam', membership_id: 'mem-1', shift_config_id: 'shift-1', shift_version: 1,
      shift_date: '', shift_name: 'Shift Pagi', start_time: '08:00', end_time: '16:00', crosses_midnight: false, lateness_tolerance_minutes: 15,
      scheduled_start_at: '', scheduled_end_at: '', status: 'scheduled',
    } as SecurityApi.SecuritySchedule])
    render(<KelolaKeamananPage />)
    fireEvent.click(await screen.findByText('Jadwal Petugas'))
    await waitFor(() => expect(screen.getByText('Pak Budi Satpam')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Ubah'))
    await waitFor(() => expect(screen.getByText('Ubah Jadwal Petugas Keamanan')).toBeInTheDocument())
    // No crash and modal open; required validation will block save but form is usable
  })
})
