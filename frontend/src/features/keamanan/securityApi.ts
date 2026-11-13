import { ApiError, authenticatedApiRequest, getSessionPair } from '@/app/api'

const BASE = '/api/v1'

export interface SecurityDevice {
  id: string
  rt_id: string
  rt_name?: string
  rt_code?: string
  device_name: string
  location_label: string
  is_active: boolean
  is_paired: boolean
  activation_expires_at?: string | null
  activated_at?: string | null
  last_active_at?: string | null
  revoked_at?: string | null
  created_at: string
  updated_at: string
}

export interface DeviceProvisionResult {
  device: SecurityDevice
  pairing_token: string
  expires_at: string
}

export interface ShiftConfig {
  id: string
  rt_id: string
  name: string
  start_time: string
  end_time: string
  crosses_midnight: boolean
  lateness_tolerance_minutes: number
  check_in_window_before_minutes: number
  check_in_window_after_minutes: number
  absent_cutoff_minutes: number
  missing_checkout_cutoff_minutes: number
  version: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface SecurityOfficer {
  user_id: string
  membership_id: string
  rt_id: string
  resident_id?: string
  full_name: string
  email: string
  phone?: string | null
  is_active: boolean
  avatar_path?: string | null
  avatar_thumb_path?: string | null
}

export interface SecuritySchedule {
  id: string
  assignment_id: string
  assignment_start_date: string
  assignment_end_date: string
  rt_id: string
  user_id: string
  officer_name?: string
  membership_id: string
  shift_config_id: string
  shift_version: number
  shift_date: string
  shift_name: string
  start_time: string
  end_time: string
  crosses_midnight: boolean
  lateness_tolerance_minutes: number
  scheduled_start_at: string
  scheduled_end_at: string
  notes?: string | null
  status: string
  attendance_status?: string
  attendance_status_label?: string
  checked_in_at?: string | null
  checked_out_at?: string | null
}

export interface ShiftSwap {
  id: string
  rt_id: string
  requester_user_id: string
  requester_name?: string
  target_user_id: string
  target_name?: string
  requester_schedule_id: string
  requester_shift_date?: string
  requester_shift_name?: string
  target_schedule_id: string
  target_shift_date?: string
  target_shift_name?: string
  reason: string
  status: string
  target_responded_at?: string | null
  target_note?: string | null
  ketua_decided_at?: string | null
  ketua_note?: string | null
  created_at: string
}

export interface AttendanceRecord {
  id: string
  rt_id: string
  schedule_id: string
  user_id: string
  officer_name?: string
  shift_config_id: string
  shift_version: number
  shift_date: string
  shift_name: string
  start_time: string
  end_time: string
  crosses_midnight: boolean
  status: string
  status_label: string
  arrival_status?: string | null
  late_minutes: number
  check_in_device_name?: string | null
  check_out_device_name?: string | null
  checked_in_at?: string | null
  checked_out_at?: string | null
  marked_absent_at?: string | null
  marked_missing_checkout_at?: string | null
  correction_locked: boolean
}

export interface AttendanceCorrection {
  id: string
  rt_id: string
  attendance_record_id: string
  schedule_id: string
  officer_user_id: string
  officer_name?: string
  requested_by: string
  requested_by_name?: string
  shift_date?: string
  shift_name?: string
  request_type: string
  original_status: string
  original_checked_in_at?: string | null
  original_checked_out_at?: string | null
  requested_status: string
  requested_checked_in_at?: string | null
  requested_checked_out_at?: string | null
  reason: string
  status: string
  decided_by?: string | null
  decided_by_name?: string | null
  decided_at?: string | null
  decision_note?: string | null
  final_status?: string | null
  final_checked_in_at?: string | null
  final_checked_out_at?: string | null
  created_at: string
}

export interface GuestBookEntry {
  id: string
  rt_id: string
  device_id: string
  device_name?: string
  guest_name: string
  phone?: string | null
  phone_masked?: string | null
  purpose: string
  destination_label: string
  vehicle_plate?: string | null
  notes?: string | null
  checked_in_at: string
  checked_out_at?: string | null
}

export interface SecurityDashboardSummary {
  server_time: string
  today_date: string
  total_officers: number
  active_shifts_count: number
  scheduled_today_count: number
  present_today_count: number
  late_today_count: number
  checked_out_today_count: number
  absent_today_count: number
  missing_checkout_count: number
  pending_swaps_count: number
  pending_corrections: number
  active_devices_count: number
  active_guests_count: number
  today_attendance: AttendanceRecord[]
}

export interface PaginatedResult<T> {
  data: T[]
  pagination: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

async function parseOrThrow<T>(res: Response, fallbackMsg: string): Promise<T> {
  const text = await res.text()
  const parsed = text ? JSON.parse(text) : null
  if (!res.ok) {
    const err = parsed?.error
    throw new ApiError(err?.code ?? 'error', err?.message ?? fallbackMsg)
  }
  return parsed as T
}

function authHeaders(jsonBody = false): Record<string, string> {
  const tok = getSessionPair()?.accessToken ?? ''
  return {
    Authorization: `Bearer ${tok}`,
    ...(jsonBody ? { 'Content-Type': 'application/json' } : {}),
  }
}

export async function apiGetSecurityDashboard(): Promise<SecurityDashboardSummary> {
  const res = await authenticatedApiRequest(`${BASE}/security/dashboard`, {
    method: 'GET',
    headers: authHeaders(),
  })
  return parseOrThrow(res, 'Gagal memuat ringkasan keamanan')
}

export async function apiListSecurityOfficers(): Promise<SecurityOfficer[]> {
  const res = await authenticatedApiRequest(`${BASE}/security/officers`, {
    method: 'GET',
    headers: authHeaders(),
  })
  const d = await parseOrThrow<{ data: SecurityOfficer[] }>(res, 'Gagal memuat data petugas')
  return d.data ?? []
}

export async function apiListSecurityShifts(activeOnly = false): Promise<ShiftConfig[]> {
  const q = activeOnly ? '?active_only=true' : ''
  const res = await authenticatedApiRequest(`${BASE}/security/shifts${q}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  const d = await parseOrThrow<{ data: ShiftConfig[] }>(res, 'Gagal memuat pengaturan shift')
  return d.data ?? []
}

export async function apiCreateSecurityShift(payload: {
  name: string
  start_time: string
  end_time: string
  crosses_midnight: boolean
  lateness_tolerance_minutes: number
  check_in_window_before_minutes: number
  check_in_window_after_minutes: number
  absent_cutoff_minutes: number
  missing_checkout_cutoff_minutes: number
  is_active?: boolean
}): Promise<ShiftConfig> {
  const res = await authenticatedApiRequest(`${BASE}/security/shifts`, {
    method: 'POST',
    headers: authHeaders(true),
    body: JSON.stringify(payload),
  })
  return parseOrThrow(res, 'Gagal membuat shift')
}

export async function apiUpdateSecurityShift(
  id: string,
  payload: {
    name: string
    start_time: string
    end_time: string
    crosses_midnight: boolean
    lateness_tolerance_minutes: number
    check_in_window_before_minutes: number
    check_in_window_after_minutes: number
    absent_cutoff_minutes: number
    missing_checkout_cutoff_minutes: number
    is_active?: boolean
  },
): Promise<ShiftConfig> {
  const res = await authenticatedApiRequest(`${BASE}/security/shifts/${id}`, {
    method: 'PUT',
    headers: authHeaders(true),
    body: JSON.stringify(payload),
  })
  return parseOrThrow(res, 'Gagal memperbarui shift')
}

export async function apiListSecuritySchedules(params: {
  date_from?: string
  date_to?: string
  user_id?: string
  shift_config_id?: string
  page?: number
  page_size?: number
} = {}): Promise<PaginatedResult<SecuritySchedule>> {
  const qs = new URLSearchParams()
  if (params.date_from) qs.set('date_from', params.date_from)
  if (params.date_to) qs.set('date_to', params.date_to)
  if (params.user_id) qs.set('user_id', params.user_id)
  if (params.shift_config_id) qs.set('shift_config_id', params.shift_config_id)
  qs.set('page', String(params.page ?? 1))
  qs.set('page_size', String(params.page_size ?? 50))
  const res = await authenticatedApiRequest(`${BASE}/security/schedules?${qs.toString()}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  return parseOrThrow(res, 'Gagal memuat jadwal petugas')
}

export async function apiCreateSecuritySchedule(payload: {
  user_id: string
  shift_config_id: string
  start_date: string
  end_date: string
  notes?: string | null
}): Promise<SecuritySchedule> {
  const res = await authenticatedApiRequest(`${BASE}/security/schedules`, {
    method: 'POST',
    headers: authHeaders(true),
    body: JSON.stringify(payload),
  })
  return parseOrThrow(res, 'Gagal membuat jadwal')
}

export async function apiGetSecuritySchedule(id: string): Promise<SecuritySchedule[]> {
  const res = await authenticatedApiRequest(`${BASE}/security/schedules/${id}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  const result = await parseOrThrow<{ data: SecuritySchedule[] }>(res, 'Gagal memuat detail jadwal')
  return result.data ?? []
}

export async function apiUpdateSecuritySchedule(id: string, payload: {
  user_id: string
  shift_config_id: string
  start_date: string
  end_date: string
  notes?: string | null
}): Promise<SecuritySchedule> {
  const res = await authenticatedApiRequest(`${BASE}/security/schedules/${id}`, {
    method: 'PUT',
    headers: authHeaders(true),
    body: JSON.stringify(payload),
  })
  return parseOrThrow(res, 'Gagal memperbarui jadwal')
}

export async function apiCancelSecuritySchedule(id: string): Promise<void> {
  const res = await authenticatedApiRequest(`${BASE}/security/schedules/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) {
    await parseOrThrow(res, 'Gagal membatalkan jadwal')
  }
}

export async function apiListSecuritySwaps(status?: string): Promise<ShiftSwap[]> {
  const qs = status ? `?status=${encodeURIComponent(status)}` : ''
  const res = await authenticatedApiRequest(`${BASE}/security/swaps${qs}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  const d = await parseOrThrow<{ data: ShiftSwap[] }>(res, 'Gagal memuat pertukaran shift')
  return d.data ?? []
}

export async function apiDecideSecuritySwap(id: string, approve: boolean, note?: string): Promise<ShiftSwap> {
  const res = await authenticatedApiRequest(`${BASE}/security/swaps/${id}/decide`, {
    method: 'POST',
    headers: authHeaders(true),
    body: JSON.stringify({ approve, note: note || null }),
  })
  return parseOrThrow(res, 'Gagal memproses pertukaran shift')
}

export async function apiListSecurityAttendance(params: {
  date_from?: string
  date_to?: string
  user_id?: string
  shift_config_id?: string
  status?: string
  page?: number
  page_size?: number
} = {}): Promise<PaginatedResult<AttendanceRecord>> {
  const qs = new URLSearchParams()
  if (params.date_from) qs.set('date_from', params.date_from)
  if (params.date_to) qs.set('date_to', params.date_to)
  if (params.user_id) qs.set('user_id', params.user_id)
  if (params.shift_config_id) qs.set('shift_config_id', params.shift_config_id)
  if (params.status) qs.set('status', params.status)
  qs.set('page', String(params.page ?? 1))
  qs.set('page_size', String(params.page_size ?? 20))
  const res = await authenticatedApiRequest(`${BASE}/security/attendance?${qs.toString()}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  return parseOrThrow(res, 'Gagal memuat data absensi')
}

export async function apiExportSecurityAttendanceCSV(params: {
  date_from?: string
  date_to?: string
  user_id?: string
  shift_config_id?: string
  status?: string
} = {}): Promise<Blob> {
  const qs = new URLSearchParams()
  if (params.date_from) qs.set('date_from', params.date_from)
  if (params.date_to) qs.set('date_to', params.date_to)
  if (params.user_id) qs.set('user_id', params.user_id)
  if (params.shift_config_id) qs.set('shift_config_id', params.shift_config_id)
  if (params.status) qs.set('status', params.status)
  const res = await authenticatedApiRequest(`${BASE}/security/attendance/export?${qs.toString()}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  if (!res.ok) {
    await parseOrThrow(res, 'Gagal mengekspor laporan absensi')
  }
  return res.blob()
}

export async function apiListSecurityCorrections(status?: string): Promise<AttendanceCorrection[]> {
  const qs = status ? `?status=${encodeURIComponent(status)}` : ''
  const res = await authenticatedApiRequest(`${BASE}/security/corrections${qs}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  const d = await parseOrThrow<{ data: AttendanceCorrection[] }>(res, 'Gagal memuat koreksi absensi')
  return d.data ?? []
}

export async function apiCreateSecurityCorrection(payload: {
  attendance_record_id: string
  request_type: 'status_change' | 'time_adjustment' | 'manual_check_in' | 'manual_check_out'
  requested_status: string
  requested_checked_in_at?: string
  requested_checked_out_at?: string
  reason: string
}): Promise<AttendanceCorrection> {
  const res = await authenticatedApiRequest(`${BASE}/security/corrections`, {
    method: 'POST',
    headers: authHeaders(true),
    body: JSON.stringify(payload),
  })
  return parseOrThrow(res, 'Gagal membuat pengajuan koreksi')
}

export async function apiDecideSecurityCorrection(id: string, approve: boolean, note?: string): Promise<AttendanceCorrection> {
  const res = await authenticatedApiRequest(`${BASE}/security/corrections/${id}/decide`, {
    method: 'POST',
    headers: authHeaders(true),
    body: JSON.stringify({ approve, note: note || null }),
  })
  return parseOrThrow(res, 'Gagal memutuskan koreksi absensi')
}

export async function apiListSecurityDevices(): Promise<SecurityDevice[]> {
  const res = await authenticatedApiRequest(`${BASE}/security/devices`, {
    method: 'GET',
    headers: authHeaders(),
  })
  const d = await parseOrThrow<{ data: SecurityDevice[] }>(res, 'Gagal memuat perangkat pos')
  return d.data ?? []
}

export async function apiCreateSecurityDevice(payload: {
  device_name: string
  location_label: string
}): Promise<DeviceProvisionResult> {
  const res = await authenticatedApiRequest(`${BASE}/security/devices`, {
    method: 'POST',
    headers: authHeaders(true),
    body: JSON.stringify(payload),
  })
  return parseOrThrow(res, 'Gagal mendaftarkan perangkat pos')
}

export async function apiRotateDevicePairing(id: string): Promise<DeviceProvisionResult> {
  const res = await authenticatedApiRequest(`${BASE}/security/devices/${id}/rotate-pairing`, {
    method: 'POST',
    headers: authHeaders(),
  })
  return parseOrThrow(res, 'Gagal membuat ulang token pairing')
}

export async function apiRevokeSecurityDevice(id: string): Promise<SecurityDevice> {
  const res = await authenticatedApiRequest(`${BASE}/security/devices/${id}/revoke`, {
    method: 'POST',
    headers: authHeaders(),
  })
  return parseOrThrow(res, 'Gagal mencabut akses perangkat')
}

export async function apiListSecurityGuestBooks(params: {
  status?: string
  date_from?: string
  date_to?: string
  search?: string
  page?: number
  page_size?: number
} = {}): Promise<PaginatedResult<GuestBookEntry>> {
  const qs = new URLSearchParams()
  if (params.status) qs.set('status', params.status)
  if (params.date_from) qs.set('date_from', params.date_from)
  if (params.date_to) qs.set('date_to', params.date_to)
  if (params.search) qs.set('search', params.search)
  qs.set('page', String(params.page ?? 1))
  qs.set('page_size', String(params.page_size ?? 20))
  const res = await authenticatedApiRequest(`${BASE}/security/guest-books?${qs.toString()}`, {
    method: 'GET',
    headers: authHeaders(),
  })
  return parseOrThrow(res, 'Gagal memuat buku tamu')
}

