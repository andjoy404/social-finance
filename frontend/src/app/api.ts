export interface ApiAuthResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  user: ApiUser
}

export interface ApiUser {
  id: string
  full_name: string
  email: string
  phone?: string | null
  system_role?: string
  role: string
  jabatan?: string | null
  effective_permissions?: string[]
  rt: { id: string; name: string } | null
  rt_id?: string
  rt_name?: string
  rt_rw?: number
  rt_rt?: string
  rw_id?: string
  rw_name?: string
}

export interface ApiRefreshResponse {
  access_token: string
  refresh_token: string
}

export interface ApiRW {
  id: string
  rw_number: number
  name: string
  address: string | null
  head_name: string | null
  is_active: boolean
  rt_count?: number
  active_rt_count?: number
  created_at: string
  updated_at: string
}

export interface ApiRWPaginated {
  data: ApiRW[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
}

export interface ApiRT {
  id: string
  name: string
  rw_id?: string | null
  rw_name?: string | null
  rw: number
  rt: string
  address: string | null
  head_name: string | null
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface ApiRTPaginated {
  data: ApiRT[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
}

export interface ApiError {
  code: string
  message: string
  details?: unknown
}

export interface SessionPair {
  accessToken: string
  refreshToken: string
}

const BASE = '/api/v1'
const SESSION_KEY = 'social-finance-session'
const TIMEOUT_MS = 10_000

async function json<T>(res: Response): Promise<T | null> {
  const text = await res.text()
  if (!text) return null
  return JSON.parse(text) as T
}

export function persistSessionPair(pair: SessionPair): void {
  try {
    sessionStorage.setItem(SESSION_KEY, JSON.stringify(pair))
  } catch {
    /* storage full - silent */
  }
}

export function clearSessionPair(): void {
  sessionStorage.removeItem(SESSION_KEY)
}

export function getSessionPair(): SessionPair | null {
  try {
    const raw = sessionStorage.getItem(SESSION_KEY)
    if (!raw) return null
    const p = JSON.parse(raw)
    if (!p || typeof p.accessToken !== 'string' || typeof p.refreshToken !== 'string') {
      clearSessionPair()
      return null
    }
    return p as SessionPair
  } catch {
    clearSessionPair()
    return null
  }
}

let _getToken: () => string | undefined = () => undefined
let _getRefreshToken: () => string | undefined = () => undefined
let _setToken: (pair: SessionPair) => void = () => {}
let _clearToken: () => void = () => {}

export function setTokenManager(
  getToken: () => string | undefined,
  getRefreshToken: () => string | undefined,
  setToken: (pair: SessionPair) => void,
  clearToken: () => void,
): void {
  _getToken = getToken
  _getRefreshToken = getRefreshToken
  _setToken = setToken
  _clearToken = clearToken
}

let refreshPromise: Promise<void> | null = null

export async function apiLogin(identifier: string, password: string): Promise<ApiAuthResponse> {
  const trimmed = identifier.trim()
  const payload: Record<string, string> = trimmed.includes('@')
    ? { email: trimmed, password }
    : { phone: trimmed, password }

  const res = await fetch(BASE + '/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiAuthResponse>(res)
  if (!res.ok || !data) {
    const body = await json<ApiError>(res).catch(() => null)
    throw new ApiError(body?.code ?? 'unexpected', body?.message ?? 'Login failed')
  }
  return data
}

export interface ApiUpdateProfileBody {
  full_name: string
  phone?: string
  email?: string
}

export async function apiUpdateProfile(accessToken: string, body: ApiUpdateProfileBody): Promise<ApiUser> {
  const res = await fetch(BASE + '/auth/profile', {
    method: 'PUT',
    headers: {
      Authorization: `Bearer ${accessToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiUser>(res)
  if (!res.ok || !data) {
    const errBody = await json<ApiError>(res).catch(() => null)
    throw new ApiError(errBody?.code ?? 'unexpected', errBody?.message ?? 'Gagal memperbarui profil')
  }
  return data
}

export interface ApiChangePasswordBody {
  current_password: string
  new_password: string
}

export async function apiChangePassword(accessToken: string, body: ApiChangePasswordBody): Promise<void> {
  const res = await fetch(BASE + '/auth/password', {
    method: 'PUT',
    headers: {
      Authorization: `Bearer ${accessToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok) {
    const errBody = await json<ApiError>(res).catch(() => null)
    throw new ApiError(errBody?.code ?? 'unexpected', errBody?.message ?? 'Gagal mengubah kata sandi')
  }
}

export async function apiLogout(accessToken: string): Promise<void> {
  const res = await fetch(BASE + '/auth/logout', {
    method: 'POST',
    headers: { Authorization: `Bearer ${accessToken}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok && res.status !== 204) {
    const body = await json<ApiError>(res).catch(() => null)
    throw new ApiError(body?.code ?? 'unexpected', body?.message ?? 'Logout failed')
  }
}

export async function apiRefresh(refreshToken: string): Promise<SessionPair> {
  if (!refreshToken || refreshToken.length === 0) {
    throw new ApiError('no_refresh_token', 'No refresh token')
  }
  const res = await fetch(BASE + '/auth/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<ApiRefreshResponse>(res).catch(() => null)
  if (!res.ok || !body) {
    const errBody = await json<ApiError>(res).catch(() => null)
    if (errBody) throw new ApiError(errBody.code, errBody.message)
    throw new ApiError('unexpected', 'Refresh response malformed')
  }
  return { accessToken: body.access_token, refreshToken: body.refresh_token }
}

async function performRefresh(): Promise<void> {
  const currentRt = _getRefreshToken()
  if (!currentRt || currentRt.length === 0) {
    _clearToken()
    return
  }
  try {
    const pair = await apiRefresh(currentRt)
    _setToken(pair)
  } catch (err) {
    _clearToken()
    throw err
  }
}

async function authenticatedApiRequest(
  url: string,
  opts: RequestInit,
): Promise<Response> {
  const method = (opts.method || 'GET').toUpperCase()
  const isGet = method === 'GET'
  const activeRT = typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('sf_active_rt_id') : null
  const headers: Record<string, string> = {
    ...(opts.headers as Record<string, string> || {}),
    ...(isGet ? { 'Cache-Control': 'no-store', Pragma: 'no-cache', Expires: '0' } : {}),
    ...(activeRT && !(opts.headers as Record<string, string>)?.['X-RT-ID'] ? { 'X-RT-ID': activeRT } : {}),
  }
  const fetchOpts: RequestInit = {
    ...opts,
    ...(isGet ? { cache: 'no-store' } : {}),
    headers,
  }
  let currentRes = await fetch(url, fetchOpts)
  let attempts = 0

  while (currentRes.status === 401 && attempts < 1) {
    attempts++

    await json<ApiError>(currentRes).catch(() => null)

    if (!refreshPromise) {
      refreshPromise = performRefresh()
    }

    try {
      await refreshPromise
    } catch {
      throw new ApiError('auth_expired', 'Session expired')
    } finally {
      refreshPromise = null
    }

    const token = _getToken()
    if (!token) {
      throw new ApiError('auth_expired', 'Session expired')
    }
    const retryHeaders: Record<string, string> = {
      ...headers,
    }
    retryHeaders['Authorization'] = `Bearer ${token}`
    const retryOpts: RequestInit = { ...fetchOpts, headers: retryHeaders as HeadersInit }
    currentRes = await fetch(url, retryOpts)
  }

  if (!currentRes.ok) {
    const raw = await currentRes.text()
    let body: unknown = null
    try { body = JSON.parse(raw) } catch { /* ignore */ }
    // Support both nested { error: { code, message } } and top-level { code, message } formats
    const nested = body && typeof body === 'object' ? (body as { error?: { code?: string; message?: string } }).error : undefined
    const apiErr = nested ?? (body && typeof body === 'object' ? body as { code?: string; message?: string } : null)
    if (currentRes.status >= 400 && apiErr?.code) {
      throw new ApiError(apiErr.code, apiErr.message ?? 'Unexpected response')
    }
    throw new ApiError('unexpected', 'Unexpected response')
  }

  return currentRes
}

export async function apiMe(token: string): Promise<ApiUser> {
  const res = await authenticatedApiRequest(BASE + '/auth/me', {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiUser>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiListRTs(
  token: string,
  params: { page?: number; page_size?: number; is_active?: boolean; search?: string; rw_id?: string },
): Promise<ApiRTPaginated> {
  const q = new URLSearchParams()
  if (params.page != null) q.set('page', String(params.page))
  if (params.page_size != null) q.set('page_size', String(params.page_size))
  if (params.search != null) q.set('search', params.search)
  if (params.rw_id != null && params.rw_id !== '') q.set('rw_id', params.rw_id)
  if (!('is_active' in params)) q.set('is_active', 'all')
  else if (params.is_active) q.set('is_active', 'true')
  else q.set('is_active', 'false')
  const res = await authenticatedApiRequest(BASE + '/rts?' + q.toString(), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiRTPaginated>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export interface ApiCreateRWBody {
  rw_number: number
  name?: string
  address?: string | null
  head_name?: string | null
}

export interface ApiUpdateRWBody {
  rw_number?: number
  name?: string
  address?: string | null
  head_name?: string | null
  is_active?: boolean
}

export async function apiListRWs(
  token: string,
  params?: { page?: number; page_size?: number; is_active?: boolean; search?: string },
): Promise<ApiRWPaginated> {
  const q = new URLSearchParams()
  if (params?.page != null) q.set('page', String(params.page))
  if (params?.page_size != null) q.set('page_size', String(params.page_size))
  if (params?.search != null) q.set('search', params.search)
  if (!params || !('is_active' in params) || params.is_active === undefined) q.set('is_active', 'all')
  else if (params.is_active) q.set('is_active', 'true')
  else q.set('is_active', 'false')
  const res = await authenticatedApiRequest(BASE + '/rws?' + q.toString(), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiRWPaginated>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiGetRW(token: string, id: string): Promise<ApiRW> {
  const res = await authenticatedApiRequest(BASE + '/rws/' + id, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiRW>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiCreateRW(token: string, body: ApiCreateRWBody): Promise<ApiRW> {
  const res = await authenticatedApiRequest(BASE + '/rws', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiRW>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiUpdateRW(token: string, id: string, body: ApiUpdateRWBody): Promise<ApiRW> {
  const res = await authenticatedApiRequest(BASE + '/rws/' + id, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiRW>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiDeactivateRW(token: string, id: string): Promise<void> {
  const res = await authenticatedApiRequest(BASE + '/rws/' + id + '/deactivate', {
    method: 'PATCH',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status !== 204 && !res.ok) {
    const body = await json<ApiError>(res).catch(() => null)
    if (body?.code) throw new ApiError(body.code, body.message)
    throw new ApiError('unexpected', 'Unexpected response')
  }
}

export interface ApiRTMembership {
  membership_id: string
  user_id: string
  full_name: string
  email?: string | null
  phone?: string | null
  role: string
  jabatan?: string | null
  is_active: boolean
}

export async function apiListRTMemberships(token: string, rtId: string): Promise<ApiRTMembership[]> {
  const res = await authenticatedApiRequest(BASE + `/rts/${encodeURIComponent(rtId)}/memberships`, { method: 'GET', headers: { Authorization: `Bearer ${token}` } })
  const body = await json<{ data: ApiRTMembership[] }>(res)
  if (!body || !Array.isArray(body.data)) throw new ApiError('unexpected', 'Response data missing')
  return body.data
}

export async function apiSetRTJabatan(token: string, rtId: string, userId: string, jabatan: string): Promise<ApiRTMembership> {
  const res = await authenticatedApiRequest(BASE + `/rts/${encodeURIComponent(rtId)}/memberships/${encodeURIComponent(userId)}/jabatan`, {
    method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }, body: JSON.stringify({ jabatan }),
  })
  const body = await json<ApiRTMembership>(res)
  if (!body) throw new ApiError('unexpected', 'Response data missing')
  return body
}

export async function apiRemoveRTJabatan(token: string, rtId: string, userId: string): Promise<void> {
  await authenticatedApiRequest(BASE + `/rts/${encodeURIComponent(rtId)}/memberships/${encodeURIComponent(userId)}/jabatan`, {
    method: 'DELETE', headers: { Authorization: `Bearer ${token}` },
  })
}

export interface ApiRWMembership {
  membership_id: string
  user_id: string
  full_name: string
  email?: string | null
  phone?: string | null
  role: string
  jabatan?: string | null
  rt_id?: string | null
  rt_name?: string | null
  is_active: boolean
}

export async function apiListRWMemberships(token: string, rwId: string): Promise<ApiRWMembership[]> {
  const res = await authenticatedApiRequest(BASE + `/rws/${encodeURIComponent(rwId)}/memberships`, { method: 'GET', headers: { Authorization: `Bearer ${token}` } })
  const body = await json<{ data: ApiRWMembership[] }>(res)
  if (!body || !Array.isArray(body.data)) throw new ApiError('unexpected', 'Response data missing')
  return body.data
}

export async function apiSetRWJabatan(token: string, rwId: string, userId: string, rtId: string, jabatan: string): Promise<ApiRWMembership> {
  const res = await authenticatedApiRequest(BASE + `/rws/${encodeURIComponent(rwId)}/memberships/${encodeURIComponent(userId)}/jabatan`, {
    method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }, body: JSON.stringify({ jabatan, rt_id: rtId }),
  })
  const body = await json<ApiRWMembership>(res)
  if (!body) throw new ApiError('unexpected', 'Response data missing')
  return body
}

export async function apiRemoveRWJabatan(token: string, rwId: string, userId: string): Promise<void> {
  await authenticatedApiRequest(BASE + `/rws/${encodeURIComponent(rwId)}/memberships/${encodeURIComponent(userId)}/jabatan`, {
    method: 'DELETE', headers: { Authorization: `Bearer ${token}` },
  })
}

export async function apiGetRT(token: string, id: string): Promise<ApiRT> {
  const res = await authenticatedApiRequest(BASE + '/rts/' + id, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiRT>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export interface ApiCreateRTBody {
  name: string
  rw_id?: string | null
  rw: number
  rt: string
  address?: string | null
  head_name?: string | null
}

export interface ApiUpdateRTBody {
  name?: string
  rw_id?: string | null
  rw?: number
  rt?: string
  address?: string | null
  head_name?: string | null
  is_active?: boolean
}

export async function apiCreateRT(token: string, body: ApiCreateRTBody): Promise<ApiRT | void> {
  const res = await authenticatedApiRequest(BASE + '/rts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status === 204) return
  const data = await json<ApiRT>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiUpdateRT(token: string, id: string, body: ApiUpdateRTBody): Promise<ApiRT> {
  const res = await authenticatedApiRequest(BASE + '/rts/' + id, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiRT>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiDeactivateRT(token: string, id: string): Promise<void> {
  const res = await authenticatedApiRequest(BASE + '/rts/' + id + '/deactivate', {
    method: 'PATCH',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status !== 204 && !res.ok) {
    const body = await json<ApiError>(res).catch(() => null)
    if (body?.code) throw new ApiError(body.code, body.message)
    throw new ApiError('unexpected', 'Unexpected response')
  }
}

export async function apiReactivateRT(token: string, id: string): Promise<ApiRT> {
  return apiUpdateRT(token, id, { is_active: true })
}

export class ApiError extends Error {
  code: string
  details?: unknown

  constructor(code: string, message: string, details?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.details = details
  }
}

// ── Household ────────────────────────────────────────────────────────────────

export interface ApiHousehold {
  id: string
  rt_id: string
  house_number: string | null
  head_name: string
  nik?: string | null
  phone: string | null
  email?: string | null
  address: string | null
  occupancy_status: 'OWNER' | 'TENANT' | null
  is_active: boolean
  head_resident?: ApiResident | null
  start_date?: string | null
  end_date?: string | null
  created_at: string
  updated_at: string
}

export interface ApiPaginated<T> {
  data: T[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
}

export interface ApiCreateHouseholdBody {
  house_number: string
  head_name: string
  nik: string
  phone: string
  email: string
  occupancy_status: 'OWNER' | 'TENANT'
  address?: string | null
  start_date?: string
  end_date?: string
  rt_id?: string
  is_active?: boolean
  password?: string
}

export interface ApiUpdateHouseholdBody {
  house_number?: string
  head_name?: string
  nik?: string
  phone?: string
  email?: string
  address?: string | null
  occupancy_status?: 'OWNER' | 'TENANT'
  start_date?: string
  end_date?: string | null
  is_active?: boolean
  password?: string
}

export interface ApiMoveHouseholdBody {
  house_number: string
  start_date: string
  address?: string | null
  occupancy_status?: 'OWNER' | 'TENANT'
}

export interface ApiListHouseholdsParams {
  page?: number
  page_size?: number
  is_active?: boolean
  search?: string
}

export async function listHouseholds(token: string, params: ApiListHouseholdsParams): Promise<ApiPaginated<ApiHousehold>> {
  const q = new URLSearchParams()
  if (params.page != null) q.set('page', String(params.page))
  if (params.page_size != null) q.set('per_page', String(params.page_size))
  if (params.is_active != null) q.set('is_active', String(params.is_active))
  if (params.search != null) q.set('search', params.search)
  const res = await authenticatedApiRequest(BASE + '/households?' + q.toString(), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiPaginated<ApiHousehold>>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function getHousehold(token: string, id: string): Promise<ApiHousehold> {
  const res = await authenticatedApiRequest(BASE + '/households/' + id, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiHousehold>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function createHousehold(token: string, body: ApiCreateHouseholdBody): Promise<ApiHousehold | void> {
  const res = await authenticatedApiRequest(BASE + '/households', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status === 204) return
  const data = await json<ApiHousehold>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function updateHousehold(token: string, id: string, body: ApiUpdateHouseholdBody): Promise<ApiHousehold> {
  const res = await authenticatedApiRequest(BASE + '/households/' + id, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiHousehold>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function deactivateHousehold(token: string, id: string): Promise<void> {
  const res = await authenticatedApiRequest(BASE + '/households/' + id, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status !== 204 && !res.ok) {
    const body = await json<ApiError>(res).catch(() => null)
    if (body?.code) throw new ApiError(body.code, body.message)
    throw new ApiError('unexpected', 'Unexpected response')
  }
}

export async function moveHousehold(token: string, id: string, body: ApiMoveHouseholdBody): Promise<ApiHousehold> {
  const res = await authenticatedApiRequest(BASE + '/households/' + id + '/move', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiHousehold>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

// ── Resident ─────────────────────────────────────────────────────────────────

export interface ApiResident {
  id: string
  rt_id: string
  household_id: string | null
  full_name: string
  nik?: string | null
  phone: string | null
  email?: string | null
  is_active: boolean
  created_at: string
  updated_at: string
  rt_number?: string | null
  rw?: number | null
  rt_name?: string | null
  start_date?: string | null
  end_date?: string | null
  jabatan?: string | null
  occupancy_status?: 'OWNER' | 'TENANT' | null
  house_number?: string | null
}

export interface ApiCreateResidentBody {
  household_id: string
  full_name: string
  nik: string
  phone?: string | null
  email?: string | null
  start_date?: string
  end_date?: string
  password?: string
}

export interface ApiUpdateResidentBody {
  full_name?: string
  nik?: string
  phone?: string | null
  email?: string | null
  start_date?: string
  end_date?: string | null
  is_active?: boolean
  password?: string
}

export interface ApiMoveResidentBody {
  destination_household_id: string
  start_date: string
}

export interface ApiListResidentsParams {
  page?: number
  page_size?: number
  is_active?: boolean
  search?: string
  household_id?: string
  signal?: AbortSignal
}

export async function listResidents(token: string, params: ApiListResidentsParams): Promise<ApiPaginated<ApiResident>> {
  const q = new URLSearchParams()
  if (params.page != null) q.set('page', String(params.page))
  if (params.page_size != null) q.set('page_size', String(params.page_size))
  if (params.is_active != null) q.set('is_active', String(params.is_active))
  if (params.search != null) q.set('search', params.search)
  if (params.household_id != null) q.set('household_id', params.household_id)
  const res = await authenticatedApiRequest(BASE + '/residents?' + q.toString(), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    signal: params.signal ?? (typeof AbortSignal !== 'undefined' ? AbortSignal.timeout(TIMEOUT_MS) : undefined),
  })
  const data = await json<ApiPaginated<ApiResident>>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function getResident(token: string, id: string): Promise<ApiResident> {
  const res = await authenticatedApiRequest(BASE + '/residents/' + id, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiResident>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function createResident(token: string, body: ApiCreateResidentBody): Promise<ApiResident | void> {
  const res = await authenticatedApiRequest(BASE + '/residents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status === 204) return
  const data = await json<ApiResident>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function updateResident(token: string, id: string, body: ApiUpdateResidentBody): Promise<ApiResident> {
  const res = await authenticatedApiRequest(BASE + '/residents/' + id, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiResident>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function deactivateResident(token: string, id: string): Promise<void> {
  const res = await authenticatedApiRequest(BASE + '/residents/' + id, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status !== 204 && !res.ok) {
    const body = await json<ApiError>(res).catch(() => null)
    if (body?.code) throw new ApiError(body.code, body.message)
    throw new ApiError('unexpected', 'Unexpected response')
  }
}

export async function moveResident(token: string, id: string, body: ApiMoveResidentBody): Promise<ApiResident> {
  const res = await authenticatedApiRequest(BASE + '/residents/' + id + '/move', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiResident>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

// ── Special Resident (Petugas Khusus) ────────────────────────────────────────

export interface ApiSpecialResidentCreateBody {
  rt_id: string
  jabatan: 'keamanan' | 'kebersihan_pembangunan'
  full_name: string
  nik: string
  phone: string
  email: string
}

export interface ApiSpecialResidentUpdateBody {
  full_name?: string
  nik?: string
  phone?: string
  email?: string
  jabatan?: 'keamanan' | 'kebersihan_pembangunan'
  is_active?: boolean
}

export async function createSpecialResident(
  token: string,
  body: ApiSpecialResidentCreateBody,
): Promise<ApiResident | void> {
  const res = await authenticatedApiRequest(BASE + '/residents/special', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (res.status === 204) return
  const data = await json<ApiResident>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function updateSpecialResident(
  token: string,
  id: string,
  body: ApiSpecialResidentUpdateBody,
): Promise<ApiResident> {
  const res = await authenticatedApiRequest(BASE + '/residents/special/' + id, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiResident>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

// ── Finance (Dues) ────────────────────────────────────────────────────────

export interface ApiDue {
  id: string
  rt_id: string
  rt_code?: string
  name: string
  amount: string
  period_type: 'monthly' | 'yearly' | 'one_time'
  payment_rule?: 'full' | 'partial'
  is_voluntary?: boolean
  due_type?: 'regular' | 'voluntary' | 'multi_month'
  year?: number | null
  is_active: boolean
  valid_until?: string | null
  periods?: string[]
  created_at: string
  updated_at: string
}

export interface ApiCreateDueBody {
  name: string
  amount: string
  period_type: 'monthly' | 'yearly' | 'one_time'
  payment_rule?: 'full' | 'partial'
  is_voluntary?: boolean
  due_type?: 'regular' | 'voluntary' | 'multi_month'
  year?: number
  due_date?: string
  valid_until?: string
}

export async function apiListDues(
  token: string,
  params?: { is_active?: boolean },
): Promise<ApiDue[]> {
  const q = new URLSearchParams()
  if (params?.is_active != null) q.set('is_active', String(params.is_active))
  const res = await authenticatedApiRequest(BASE + '/dues?' + q.toString(), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiDue[]>(res)
  if (!Array.isArray(data)) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiCreateDue(
  token: string,
  body: ApiCreateDueBody,
  idempotencyKey?: string,
): Promise<ApiDue> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey
  }
  const res = await authenticatedApiRequest(BASE + '/dues', {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiDue>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export interface ApiUpdateDueBody {
  name?: string
  amount?: string
  period_type?: 'monthly' | 'yearly' | 'one_time'
  payment_rule?: 'full' | 'partial'
  is_voluntary?: boolean
  is_active?: boolean
  valid_until?: string
}

export async function apiUpdateDue(
  token: string,
  id: string,
  body: ApiUpdateDueBody,
): Promise<ApiDue> {
  const res = await authenticatedApiRequest(BASE + `/dues/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiDue>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiDeactivateDue(
  token: string,
  id: string,
): Promise<void> {
  const res = await authenticatedApiRequest(BASE + `/dues/${id}/deactivate`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok && res.status !== 204) {
    const bodyErr = await json<ApiError>(res).catch(() => null)
    throw new ApiError(bodyErr?.code ?? 'unexpected', bodyErr?.message ?? 'Gagal menonaktifkan iuran.')
  }
}

export interface DueMonthSummary {
  period: string
  month: number
  year: number
  total_bills: number
  active_bills: number
  paid_bills: number
  unpaid_bills: number
  is_active: boolean
  can_deactivate: boolean
}

export async function apiGetDueMonths(
  token: string,
  dueId: string,
): Promise<DueMonthSummary[]> {
  const res = await authenticatedApiRequest(BASE + `/dues/${dueId}/months`, {
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const jsonResp = await json<{ data: DueMonthSummary[] }>(res)
  return jsonResp?.data ?? []
}

export async function apiUpdateDuePeriodStatus(
  token: string,
  dueId: string,
  period: string,
  isActive: boolean,
): Promise<DueMonthSummary[]> {
  const res = await authenticatedApiRequest(BASE + `/dues/${dueId}/periods/${period}/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ is_active: isActive }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const jsonResp = await json<{ data: DueMonthSummary[] }>(res)
  return jsonResp?.data ?? []
}

export async function apiUpdateBillStatus(
  token: string,
  billId: string,
  isActive: boolean,
): Promise<ApiBill> {
  const res = await authenticatedApiRequest(BASE + `/bills/${billId}/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ is_active: isActive }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiBill>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export interface ApiGenerateBillsBody {
  due_id: string
  period?: string
  due_date?: string
}

export interface ApiGenerateBillsResponse {
  generated_count: number
  bills: ApiBill[]
}

export async function apiGenerateBills(
  token: string,
  body: ApiGenerateBillsBody,
  idempotencyKey: string,
): Promise<ApiGenerateBillsResponse> {
  const res = await authenticatedApiRequest(BASE + '/bills/generate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      'Idempotency-Key': idempotencyKey,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiGenerateBillsResponse>(res)
  if (!res.ok || !data) {
    const bodyErr = await json<ApiError>(res).catch(() => null)
    throw new ApiError(bodyErr?.code ?? 'unexpected', bodyErr?.message ?? 'Gagal membuat tagihan.')
  }
  if (!Array.isArray(data.bills) || data.generated_count !== data.bills.length) {
    throw new ApiError('malformed_response', 'Respons pembuatan tagihan tidak valid.')
  }
  return data
}


export interface ApiBill {
  id: string
  rt_id: string
  rt_code?: string | null
  household_occupancy_id: string
  due_id: string
  amount: string
  period: string
  due_date: string
  status: 'unpaid' | 'partial' | 'paid' | 'cancelled'
  is_active?: boolean
  created_at: string
  updated_at: string
  due_name?: string | null
  due_is_active?: boolean | null
  due_type?: 'regular' | 'voluntary' | 'multi_month' | null
  due_year?: number | null
  house_number?: string | null
  head_name?: string | null
  is_voluntary?: boolean
  paid_amount?: string | null
  pending_amount?: string | null
}

export interface ApiPayment {
  id: string
  rt_id: string
  bill_id?: string | null
  due_id?: string | null
  submitted_by?: string | null
  submitter_name?: string | null
  amount: string
  method: 'CASH' | 'TRANSFER'
  origin: 'SELF_SUBMITTED' | 'STAFF_RECORDED'
  status: 'PENDING' | 'APPROVED' | 'REJECTED'
  proof_path?: string | null
  proof_original_name?: string | null
  proof_mime_type?: string | null
  proof_size_bytes?: number | null
  proof_uploaded_at?: string | null
  proof_uploaded_by?: string | null
  paid_at: string
  verified_by?: string | null
  verified_at?: string | null
  rejection_reason?: string | null
  notes?: string | null
  officer_notes?: string | null
  verified_by_name?: string | null
  payment_no?: string | null
  actual_received_date?: string | null
  created_at: string
  updated_at: string
}

export interface ApiCreatePaymentBody {
  bill_id?: string | null
  due_id?: string | null
  amount: string
  method: 'CASH' | 'TRANSFER'
  proof_path?: string | null
  notes?: string | null
  file?: File
}

export async function apiListBills(
  token: string,
  params?: {
    page?: number
    page_size?: number
    household_occupancy_id?: string
    due_id?: string
    status?: string
    period?: string
    search?: string
    due_name?: string
    my_bills?: boolean
    payment_status?: string
    is_active?: boolean
    // Backend-supported ordering: 'unpaid_first' → unpaid/partial oldest-first,
    // then paid newest-first (see docs/api.md).
    order?: string
  },
): Promise<ApiPaginated<ApiBill>> {
  const q = new URLSearchParams()
  if (params?.page != null) q.set('page', String(params.page))
  if (params?.page_size != null) q.set('page_size', String(params.page_size))
  if (params?.household_occupancy_id) q.set('household_occupancy_id', params.household_occupancy_id)
  if (params?.due_id) q.set('due_id', params.due_id)
  if (params?.status) q.set('status', params.status)
  if (params?.payment_status) q.set('payment_status', params.payment_status)
  if (params?.period) q.set('period', params.period)
  if (params?.search) q.set('search', params.search)
  if (params?.due_name) q.set('due_name', params.due_name)
  if (params?.my_bills) q.set('my_bills', 'true')
  if (params?.is_active != null) q.set('is_active', String(params.is_active))
  if (params?.order) q.set('order', params.order)
  const res = await authenticatedApiRequest(BASE + '/bills?' + q.toString(), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiPaginated<ApiBill>>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiGetBill(token: string, id: string): Promise<ApiBill> {
  const res = await authenticatedApiRequest(BASE + '/bills/' + id, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiBill>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiListPayments(
  token: string,
  params?: {
    page?: number
    page_size?: number
    bill_id?: string
    due_id?: string
    status?: string
    submitted_by?: string
  },
): Promise<ApiPaginated<ApiPayment>> {
  const q = new URLSearchParams()
  if (params?.page != null) q.set('page', String(params.page))
  if (params?.page_size != null) q.set('page_size', String(params.page_size))
  if (params?.bill_id) q.set('bill_id', params.bill_id)
  if (params?.due_id) q.set('due_id', params.due_id)
  if (params?.status) q.set('status', params.status)
  if (params?.submitted_by) q.set('submitted_by', params.submitted_by)
  const res = await authenticatedApiRequest(BASE + '/payments?' + q.toString(), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiPaginated<ApiPayment>>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiGetPayment(token: string, id: string): Promise<ApiPayment> {
  const res = await authenticatedApiRequest(BASE + '/payments/' + id, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiPayment>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiCreatePayment(
  token: string,
  body: ApiCreatePaymentBody,
  idempotencyKey: string = crypto.randomUUID(),
): Promise<ApiPayment> {
  if (Boolean(body.bill_id) === Boolean(body.due_id)) {
    throw new ApiError('validation_error', 'Pilih tepat satu target tagihan atau iuran sukarela.')
  }
  let res: Response
  if (body.file) {
    const formData = new FormData()
    if (body.bill_id) formData.append('bill_id', body.bill_id)
    if (body.due_id) formData.append('due_id', body.due_id)
    formData.append('amount', body.amount)
    formData.append('method', body.method)
    if (body.notes) formData.append('notes', body.notes)
    formData.append('file', body.file)

    res = await authenticatedApiRequest(BASE + '/payments', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Idempotency-Key': idempotencyKey,
      },
      body: formData,
      ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
    })
  } else {
    res = await authenticatedApiRequest(BASE + '/payments', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
        'Idempotency-Key': idempotencyKey,
      },
      body: JSON.stringify({
        bill_id: body.bill_id,
        due_id: body.due_id,
        amount: body.amount,
        method: body.method,
        notes: body.notes,
        proof_path: body.proof_path,
      }),
      ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
    })
  }

  const data = await json<unknown>(res)
  if (!res.ok || !data || typeof data !== 'object' || !('id' in data)) {
    const bodyErr = data && typeof data === 'object' && 'error' in data
      ? (data as { error?: ApiError }).error
      : undefined
    if (res.status >= 400 && bodyErr?.code) {
      throw new ApiError(bodyErr.code, bodyErr.message)
    }
    throw new ApiError('unexpected', 'Unexpected response')
  }
  return data as ApiPayment
}

export interface ApiPaymentAllocation {
  id: string
  payment_id: string
  bill_id: string
  amount: string
  created_at?: string
}

export interface ApiMultiPeriodPaymentResult {
  payment: ApiPayment
  allocations: ApiPaymentAllocation[]
}

export interface ApiCreateMultiPeriodPaymentBody {
  bill_ids: string[]
  allocations: { bill_id: string; amount: string }[]
  amount?: string
  method: 'CASH' | 'TRANSFER'
  notes?: string
  proof_path?: string
  file?: File
}

export async function apiCreateMultiPeriodPayment(
  token: string,
  body: ApiCreateMultiPeriodPaymentBody,
  idempotencyKey: string = crypto.randomUUID(),
): Promise<ApiMultiPeriodPaymentResult> {
  if (!body.bill_ids || body.bill_ids.length === 0) {
    throw new ApiError('validation_error', 'Pilih minimal satu tagihan untuk pembayaran multi-bulan.')
  }
  if (body.bill_ids.length > 12) {
    throw new ApiError('validation_error', 'Maksimal 12 tagihan dalam satu transaksi pembayaran.')
  }
  let res: Response
  if (body.file) {
    const formData = new FormData()
    formData.append('bill_ids', JSON.stringify(body.bill_ids))
    formData.append('allocations', JSON.stringify(body.allocations))
    if (body.amount) formData.append('amount', body.amount)
    formData.append('method', body.method)
    if (body.notes) formData.append('notes', body.notes)
    formData.append('file', body.file)

    res = await authenticatedApiRequest(BASE + '/payments/multi-period', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Idempotency-Key': idempotencyKey,
      },
      body: formData,
      ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
    })
  } else {
    res = await authenticatedApiRequest(BASE + '/payments/multi-period', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
        'Idempotency-Key': idempotencyKey,
      },
      body: JSON.stringify({
        bill_ids: body.bill_ids,
        allocations: body.allocations,
        amount: body.amount,
        method: body.method,
        notes: body.notes,
        proof_path: body.proof_path,
      }),
      ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
    })
  }

  const data = await json<unknown>(res)
  if (!res.ok || !data || typeof data !== 'object' || !('payment' in data)) {
    const bodyErr = data && typeof data === 'object' && 'error' in data
      ? (data as { error?: ApiError }).error
      : undefined
    if (res.status >= 400 && bodyErr?.code) {
      throw new ApiError(bodyErr.code, bodyErr.message)
    }
    throw new ApiError('unexpected', 'Unexpected response')
  }
  return data as ApiMultiPeriodPaymentResult
}

export async function apiUploadPaymentProof(
  token: string,
  paymentId: string,
  file: File,
): Promise<ApiPayment> {
  const formData = new FormData()
  formData.append('file', file)

  const res = await authenticatedApiRequest(BASE + `/payments/${paymentId}/proof`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: formData,
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })

  const data = await json<ApiPayment>(res)
  if (!res.ok || !data) {
    const bodyErr = await json<ApiError>(res).catch(() => null)
    if (res.status >= 400 && bodyErr?.code) {
      throw new ApiError(bodyErr.code, bodyErr.message)
    }
    throw new ApiError('unexpected', 'Unexpected response')
  }
  return data
}

export async function apiGetPaymentProofBlob(
  token: string,
  paymentId: string,
): Promise<{ blob: Blob; contentType: string }> {
  const res = await authenticatedApiRequest(BASE + `/payments/${paymentId}/proof`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })

  if (!res.ok) {
    const bodyErr = await json<ApiError>(res).catch(() => null)
    if (res.status >= 400 && bodyErr?.code) {
      throw new ApiError(bodyErr.code, bodyErr.message)
    }
    throw new ApiError('unexpected', 'Failed to fetch payment proof')
  }

  const blob = await res.blob()
  const contentType = res.headers.get('content-type') || 'image/jpeg'
  return { blob, contentType }
}

export async function apiVerifyPayment(
  token: string,
  paymentId: string,
  action: 'approve' | 'reject',
  options?: { rejectionReason?: string; officerNotes?: string; locationId?: string; actualReceivedDate?: string },
): Promise<ApiPayment> {
  const res = await authenticatedApiRequest(BASE + '/payments/' + paymentId + '/verify', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      action,
      ...(options?.actualReceivedDate ? { actual_received_date: options.actualReceivedDate } : {}),
      ...(options?.rejectionReason ? { rejection_reason: options.rejectionReason } : {}),
      ...(options?.officerNotes ? { officer_notes: options.officerNotes } : {}),
      ...(options?.locationId ? { location_id: options.locationId } : {}),
    }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiPayment>(res)
  if (!res.ok || !data) {
    const body = await json<ApiError>(res).catch(() => null)
    if (res.status >= 400 && body?.code) {
      throw new ApiError(body.code, body.message)
    }
    throw new ApiError('unexpected', 'Unexpected response')
  }
  return data
}

export async function apiReversePayment(
  token: string,
  paymentId: string,
  reason: string,
): Promise<{ status: string; message: string; transaction: ApiTransaction }> {
  const res = await authenticatedApiRequest(BASE + `/payments/${paymentId}/reverse`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ reason }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<{ status: string; message: string; transaction: ApiTransaction }>(res)
  if (!res.ok || !data) {
    const body = await json<ApiError>(res).catch(() => null)
    if (res.status >= 400 && body?.code) {
      throw new ApiError(body.code, body.message)
    }
    throw new ApiError('unexpected', 'Unexpected response')
  }
  return data
}

export async function apiRegisterDevice(
  token: string,
  fcmToken: string,
  platform: 'web' | 'android' | 'ios' = 'web',
  deviceId?: string,
): Promise<void> {
  const res = await authenticatedApiRequest(BASE + '/notifications/devices', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      fcm_token: fcmToken,
      platform,
      ...(deviceId ? { device_id: deviceId } : {}),
    }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok) {
    const body = await json<ApiError>(res).catch(() => null)
    if (res.status >= 400 && body?.code) {
      throw new ApiError(body.code, body.message)
    }
    throw new ApiError('unexpected', 'Failed to register device')
  }
}

export async function apiUnregisterDevice(token: string, fcmToken: string): Promise<void> {
  const res = await authenticatedApiRequest(BASE + '/notifications/devices', {
    method: 'DELETE',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ fcm_token: fcmToken }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok) {
    const body = await json<ApiError>(res).catch(() => null)
    if (res.status >= 400 && body?.code) {
      throw new ApiError(body.code, body.message)
    }
    throw new ApiError('unexpected', 'Failed to unregister device')
  }
}

export async function apiTriggerWasteTruckArrival(
  token: string,
  message?: string,
): Promise<{ status: string; message: string }> {
  const res = await authenticatedApiRequest(BASE + '/notifications/waste-truck', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      ...(message ? { message } : {}),
    }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<{ status: string; message: string }>(res)
  if (!res.ok || !data) {
    const body = await json<ApiError>(res).catch(() => null)
    if (res.status >= 400 && body?.code) {
      throw new ApiError(body.code, body.message)
    }
    throw new ApiError('unexpected', 'Failed to trigger waste truck arrival notification')
  }
  return data
}

// --- Notification Management API ---

export interface ApiSystemNotificationType {
  event_type: string
  name: string
  trigger: string
  channel: string
  recipients_rule: string
  default_title: string
  default_body: string
  allowed_variables: string[]
  is_configurable: boolean
  supports_image?: boolean
  supports_test?: boolean
}

export interface ApiNotificationTemplate {
  id: string
  rt_id: string
  name: string
  event_type: string
  channel: string
  title: string
  body: string
  image_url?: string | null
  is_active: boolean
  variables: string[]
  created_at: string
  updated_at: string
}

export interface ApiCreateNotificationTemplateInput {
  name: string
  event_type: string
  title: string
  body: string
  channel?: string
  image_url?: string | null
}

export interface ApiUpdateNotificationTemplateInput {
  name?: string
  title?: string
  body?: string
  image_url?: string | null
  is_active?: boolean
}

export interface ApiNotificationLog {
  id: string
  rt_id: string
  user_id?: string | null
  event_type: string
  dedup_key: string
  title: string
  body: string
  image_url?: string | null
  status: string
  created_at: string
}

export async function apiGetNotificationSystemTypes(token: string): Promise<ApiSystemNotificationType[]> {
  const res = await authenticatedApiRequest(BASE + '/notifications/system-types', {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<{ data: ApiSystemNotificationType[] }>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to load system notification types')
  }
  return body.data ?? []
}

export async function apiGetNotificationTemplates(
  token: string,
  eventType?: string,
  isActive?: boolean,
): Promise<ApiNotificationTemplate[]> {
  const params = new URLSearchParams()
  if (eventType) params.set('event_type', eventType)
  if (isActive !== undefined) params.set('is_active', String(isActive))
  const qs = params.toString() ? `?${params.toString()}` : ''

  const res = await authenticatedApiRequest(BASE + `/notifications/templates${qs}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<{ data: ApiNotificationTemplate[] }>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to load notification templates')
  }
  return body.data ?? []
}

export async function apiCreateNotificationTemplate(
  token: string,
  input: ApiCreateNotificationTemplateInput,
): Promise<ApiNotificationTemplate> {
  const res = await authenticatedApiRequest(BASE + '/notifications/templates', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(input),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<ApiNotificationTemplate>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to create notification template')
  }
  return body
}

export async function apiGetNotificationTemplate(token: string, id: string): Promise<ApiNotificationTemplate> {
  const res = await authenticatedApiRequest(BASE + `/notifications/templates/${id}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<ApiNotificationTemplate>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to get notification template')
  }
  return body
}

export async function apiUpdateNotificationTemplate(
  token: string,
  id: string,
  input: ApiUpdateNotificationTemplateInput,
): Promise<ApiNotificationTemplate> {
  const res = await authenticatedApiRequest(BASE + `/notifications/templates/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(input),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<ApiNotificationTemplate>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to update notification template')
  }
  return body
}

export async function apiToggleNotificationTemplateActive(
  token: string,
  id: string,
): Promise<ApiNotificationTemplate> {
  const res = await authenticatedApiRequest(BASE + `/notifications/templates/${id}/toggle-active`, {
    method: 'PATCH',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<ApiNotificationTemplate>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to toggle notification template status')
  }
  return body
}

export async function apiDeleteNotificationTemplate(token: string, id: string): Promise<void> {
  const res = await authenticatedApiRequest(BASE + `/notifications/templates/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok && res.status !== 204) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to delete notification template')
  }
}

export async function apiGetNotificationLogs(
  token: string,
  page = 1,
  pageSize = 20,
  eventType?: string,
): Promise<{ data: ApiNotificationLog[]; pagination: { page: number; page_size: number; total: number; total_pages: number } }> {
  const params = new URLSearchParams()
  params.set('page', String(page))
  params.set('page_size', String(pageSize))
  if (eventType) params.set('event_type', eventType)

  const res = await authenticatedApiRequest(BASE + `/notifications/logs?${params.toString()}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<{ data: ApiNotificationLog[]; pagination: { page: number; page_size: number; total: number; total_pages: number } }>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Failed to load notification logs')
  }
  return body
}

export async function apiUploadNotificationImage(
  token: string,
  eventType: string,
  file: File,
): Promise<{ status: string; image_url: string; key: string }> {
  const formData = new FormData()
  formData.append('event_type', eventType)
  formData.append('file', file)

  const res = await authenticatedApiRequest(BASE + '/notifications/upload-image', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: formData,
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS * 2) } : {}),
  })
  const body = await json<{ status: string; image_url: string; key: string }>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Gagal mengunggah gambar notifikasi')
  }
  return body
}

export async function apiTriggerManualPush(
  token: string,
  input: { title: string; body: string; image_url?: string; deep_link?: string },
): Promise<{ status: string; message: string }> {
  const res = await authenticatedApiRequest(BASE + '/notifications/manual', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(input),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<{ status: string; message: string }>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Gagal mengirim notifikasi manual')
  }
  return body
}

export async function apiTestPush(
  token: string,
  input: { event_type: string; title: string; body: string; image_url?: string },
): Promise<{ status: string; message: string; dispatched_count: number }> {
  const res = await authenticatedApiRequest(BASE + '/notifications/test-push', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(input),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const body = await json<{ status: string; message: string; dispatched_count: number }>(res)
  if (!res.ok || !body) {
    const err = await json<ApiError>(res).catch(() => null)
    throw new ApiError(err?.code ?? 'unexpected', err?.message ?? 'Gagal mengirim uji notifikasi')
  }
  return body
}

// --- KAS / Transaction API ---

export interface ApiFinancialCategory {
  id: string
  rt_id: string
  name: string
  type: 'income' | 'expense'
  is_active: boolean
  created_at: string
  updated_at: string
}

export async function apiListCategories(token: string, isActive?: boolean): Promise<ApiFinancialCategory[]> {
  const q = new URLSearchParams()
  if (isActive != null) q.set('is_active', String(isActive))
  const res = await authenticatedApiRequest(BASE + '/categories' + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiFinancialCategory[]>(res)
  if (!Array.isArray(data)) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export interface ApiTransaction {
  id: string
  rt_id: string
  category_id: string
  amount: string
  type: 'income' | 'expense'
  status: 'draft' | 'posted' | 'cancelled'
  reverses_transaction_id?: string | null
  payment_id?: string | null
  created_by?: string | null
  officer_name?: string | null
  due_name?: string | null
  citizen_name?: string | null
  description: string
  occurred_at: string
  created_at: string
  updated_at: string
  category_name?: string | null
  payment_no?: string | null
  reference_no?: string | null
  payment_method?: 'CASH' | 'TRANSFER' | string | null
  house_number?: string | null
  location_id?: string | null
  location_name?: string | null
}

export interface ApiTransactionPaginated {
  data: ApiTransaction[]
  pagination: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

export interface ApiBalanceSummary {
  total_income: string
  total_expense: string
  net_balance: string
  transaction_count: number
}

export interface ApiPeriodBalanceSummary {
  opening_balance: string
  total_income: string
  total_expense: string
  net_movement: string
  closing_balance: string
  transaction_count: number
}

export interface ApiCreateTransactionInput {
  category_id?: string
  amount: string
  type: 'income' | 'expense'
  description: string
  occurred_at?: string
  payment_method?: 'CASH' | 'TRANSFER'
  location_id?: string
}

export async function apiListTransactions(
  token: string,
  params?: {
    page?: number
    page_size?: number
    category_id?: string
    type?: string
    status?: string
    period?: string
    start_date?: string
    end_date?: string
  },
): Promise<ApiTransactionPaginated> {
  const q = new URLSearchParams()
  if (params?.page != null) q.set('page', String(params.page))
  if (params?.page_size != null) q.set('page_size', String(params.page_size))
  if (params?.category_id) q.set('category_id', params.category_id)
  if (params?.type) q.set('type', params.type)
  if (params?.status) q.set('status', params.status)
  if (params?.period) q.set('period', params.period)
  if (params?.start_date) q.set('start_date', params.start_date)
  if (params?.end_date) q.set('end_date', params.end_date)
  const res = await authenticatedApiRequest(BASE + '/transactions' + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiTransactionPaginated>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiGetTransaction(token: string, id: string): Promise<ApiTransaction> {
  const res = await authenticatedApiRequest(BASE + '/transactions/' + id, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiTransaction>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiCreateTransaction(
  token: string,
  body: ApiCreateTransactionInput,
): Promise<ApiTransaction> {
  const res = await authenticatedApiRequest(BASE + '/transactions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      'Idempotency-Key': `kas-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiTransaction>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiGetBalance(
  token: string,
  params?: {
    period?: string
    start_date?: string
    end_date?: string
  },
): Promise<ApiBalanceSummary & Partial<ApiPeriodBalanceSummary>> {
  const q = new URLSearchParams()
  if (params?.period) q.set('period', params.period)
  if (params?.start_date) q.set('start_date', params.start_date)
  if (params?.end_date) q.set('end_date', params.end_date)
  const res = await authenticatedApiRequest(BASE + '/reports/balance' + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiBalanceSummary & Partial<ApiPeriodBalanceSummary>>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiUpdateTransaction(
  token: string,
  id: string,
  body: Partial<ApiCreateTransactionInput>,
): Promise<ApiTransaction> {
  const res = await authenticatedApiRequest(BASE + `/transactions/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiTransaction>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export async function apiUpdateOpeningBalance(
  token: string,
  period: string,
  openingBalance: string,
  reason?: string,
): Promise<{ rt_id: string; period: string; opening_balance: string; source: string }> {
  const res = await authenticatedApiRequest(BASE + `/reports/balance/opening/${encodeURIComponent(period)}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ opening_balance: openingBalance, reason: reason ?? 'Koreksi saldo awal' }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<{ rt_id: string; period: string; opening_balance: string; source: string }>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

export interface ApiDashboardDuesProgress {
  paid_households: number
  total_households: number
  percentage: number
}

export interface ApiDashboardChartPoint {
  month: string
  period: string
  income: number
  expense: number
}

export interface ApiDashboardPaymentMethodBreakdown {
  cash_amount: string
  transfer_amount: string
  cash_count: number
  transfer_count: number
}

export interface ApiDashboardSummary {
  balance: string
  monthly_income: string
  monthly_expense: string
  net_movement: string
  dues_progress: ApiDashboardDuesProgress
  payment_method_breakdown?: ApiDashboardPaymentMethodBreakdown
  chart: ApiDashboardChartPoint[]
  recent_transactions: ApiTransaction[]
}

export async function apiGetDashboardSummary(token: string, period?: string): Promise<ApiDashboardSummary> {
  const url = period ? `${BASE}/dashboard/summary?period=${encodeURIComponent(period)}` : `${BASE}/dashboard/summary`
  const res = await authenticatedApiRequest(url, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiDashboardSummary>(res)
  if (!data) throw new ApiError('unexpected', 'Response data missing')
  return data
}

// ── Warga Import / Template API ──────────────────────────────────────────────

export interface ApiWargaImportRowError {
  row: number
  field: string
  code: string
  message: string
}

export interface ApiWargaImportRowData {
  row_number: number
  house_number: string
  head_name: string
  occupancy_status: string
  address: string
  resident_name: string
  nik: string
  phone: string
  email: string
}

export interface ApiWargaImportPreviewResponse {
  total_rows: number
  valid_rows: number
  invalid_rows: number
  errors: ApiWargaImportRowError[]
  preview_data: ApiWargaImportRowData[]
}

export interface ApiWargaImportCommitResponse {
  status: string
  imported_households: number
  imported_residents: number
}

export async function apiDownloadWargaTemplate(token: string): Promise<Blob> {
  const res = await authenticatedApiRequest(BASE + '/warga/template', {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok) {
    const raw = await res.text().catch(() => '')
    let errCode = 'unexpected'
    let errMsg = 'Gagal mengunduh template warga'
    if (raw) {
      try {
        const body = JSON.parse(raw)
        errCode = body?.error?.code || body?.code || errCode
        errMsg = body?.error?.message || body?.message || errMsg
      } catch {
        /* non-json */
      }
    }
    throw new ApiError(errCode, errMsg)
  }
  return res.blob()
}

export async function apiDownloadWargaExport(token: string, rtId?: string): Promise<Blob> {
  const url = BASE + '/warga/export/xlsx' + (rtId ? `?rt_id=${encodeURIComponent(rtId)}` : '')
  const res = await authenticatedApiRequest(url, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  if (!res.ok) {
    const raw = await res.text().catch(() => '')
    let errCode = 'unexpected'
    let errMsg = 'Gagal mengunduh export warga'
    if (raw) {
      try {
        const body = JSON.parse(raw)
        errCode = body?.error?.code || body?.code || errCode
        errMsg = body?.error?.message || body?.message || errMsg
      } catch {
        /* non-json */
      }
    }
    throw new ApiError(errCode, errMsg)
  }
  return res.blob()
}

export async function apiPreviewWargaImportXLSX(
  token: string,
  file: File,
  rtId?: string,
): Promise<ApiWargaImportPreviewResponse> {
  const formData = new FormData()
  formData.append('file', file)

  const url = BASE + '/warga/import/preview/xlsx' + (rtId ? `?rt_id=${encodeURIComponent(rtId)}` : '')
  const res = await authenticatedApiRequest(url, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: formData,
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })

  const raw = await res.text()
  let body: any = null
  if (raw) {
    try {
      body = JSON.parse(raw)
    } catch {
      /* non-json */
    }
  }

  if (!res.ok) {
    const errCode = body?.error?.code || body?.code || 'unexpected'
    const errMsg = body?.error?.message || body?.message || 'Gagal memproses pratinjau import warga'
    const details = body?.error?.details || body?.details
    throw new ApiError(errCode, errMsg, details)
  }

  return body as ApiWargaImportPreviewResponse
}

export async function apiCommitWargaImportXLSX(
  token: string,
  file: File,
  rtId?: string,
): Promise<ApiWargaImportCommitResponse> {
  const formData = new FormData()
  formData.append('file', file)

  const url = BASE + '/warga/import/commit/xlsx' + (rtId ? `?rt_id=${encodeURIComponent(rtId)}` : '')
  const res = await authenticatedApiRequest(url, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: formData,
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })

  const raw = await res.text()
  let body: any = null
  if (raw) {
    try {
      body = JSON.parse(raw)
    } catch {
      /* non-json */
    }
  }

  if (!res.ok) {
    const errCode = body?.error?.code || body?.code || 'unexpected'
    const errMsg = body?.error?.message || body?.message || 'Gagal menyimpan data import warga'
    const details = body?.error?.details || body?.details
    throw new ApiError(errCode, errMsg, details)
  }

  return body as ApiWargaImportCommitResponse
}

// ─────────────────────────────────────────────────────────────
// Fund Locations & Transfers (Lokasi Dana: Rekening, Brankas, COH)
// ─────────────────────────────────────────────────────────────

export type FundLocationCategory = 'rekening' | 'brankas' | 'coh'

export interface FundLocation {
  id: string
  rt_id: string
  name: string
  category: FundLocationCategory
  bank_name?: string | null
  account_number?: string | null
  account_holder?: string | null
  is_active: boolean
  current_balance: string
  created_at: string
  updated_at: string
}

export interface CreateFundLocationInput {
  name: string
  category: FundLocationCategory
  bank_name?: string
  account_number?: string
  account_holder?: string
}

export interface UpdateFundLocationInput {
  name?: string
  bank_name?: string
  account_number?: string
  account_holder?: string
}

export interface LocationBalanceChange {
  id: string
  rt_id: string
  location_id: string
  transaction_id?: string | null
  transfer_id?: string | null
  change_type: string
  amount_delta: string
  notes?: string | null
  created_by?: string | null
  created_at: string
}

export interface FundTransfer {
  id: string
  rt_id: string
  from_location_id: string
  to_location_id: string
  amount: string
  transfer_date: string
  notes?: string | null
  created_by?: string | null
  reverses_transfer_id?: string | null
  created_at: string
  from_location_name?: string | null
  to_location_name?: string | null
}

export interface CreateFundTransferInput {
  from_location_id: string
  to_location_id: string
  amount: string
  transfer_date?: string
  notes?: string
}

export interface ReconciliationReport {
  kas_balance: string
  total_allocated_balance: string
  unallocated_balance: string
  discrepancy: string
  status: 'MATCH' | 'MATCH_WITH_NOTES' | 'DISCREPANCY_ANOMALY'
  status_label: string
  locations_summary: Array<{
    category: string
    total_balance: string
    locations_count: number
  }>
  anomalies_detected: string[]
}

function unpackFundLocation(resData: any): FundLocation {
  if (!resData) throw new ApiError('unexpected', 'Response data missing')
  let payload = resData.data !== undefined ? resData.data : resData
  if (Array.isArray(payload)) {
    if (payload.length !== 1) throw new ApiError('unexpected', 'Response data missing')
    payload = payload[0]
  }
  if (!payload || typeof payload !== 'object' || !payload.id || typeof payload.id !== 'string') {
    throw new ApiError('unexpected', 'Response data missing')
  }
  return payload as FundLocation
}

function unpackFundLocationList(resData: any): FundLocation[] {
  if (!resData) throw new ApiError('unexpected', 'Response data missing')
  const payload = resData.data !== undefined ? resData.data : resData
  if (!Array.isArray(payload)) throw new ApiError('unexpected', 'Response data missing')
  return payload as FundLocation[]
}

function unpackFundTransfer(resData: any): FundTransfer {
  if (!resData) throw new ApiError('unexpected', 'Response data missing')
  let payload = resData.data !== undefined ? resData.data : resData
  if (Array.isArray(payload)) {
    if (payload.length !== 1) throw new ApiError('unexpected', 'Response data missing')
    payload = payload[0]
  }
  if (!payload || typeof payload !== 'object' || !payload.id || typeof payload.id !== 'string') {
    throw new ApiError('unexpected', 'Response data missing')
  }
  return payload as FundTransfer
}

function unpackReconciliation(resData: any): ReconciliationReport {
  if (!resData) throw new ApiError('unexpected', 'Response data missing')
  const payload = resData.data !== undefined ? resData.data : resData
  if (!payload || typeof payload !== 'object' || payload.kas_balance === undefined) {
    throw new ApiError('unexpected', 'Response data missing')
  }
  return payload as ReconciliationReport
}

export async function apiListFundLocations(
  token: string,
  params?: { category?: string; is_active?: boolean },
): Promise<FundLocation[]> {
  const q = new URLSearchParams()
  if (params?.category) q.set('category', params.category)
  if (params?.is_active != null) q.set('is_active', String(params.is_active))
  const res = await authenticatedApiRequest(BASE + '/fund-locations' + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundLocationList(resData)
}

export async function apiCreateFundLocation(
  token: string,
  body: CreateFundLocationInput,
): Promise<FundLocation> {
  const res = await authenticatedApiRequest(BASE + '/fund-locations', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundLocation(resData)
}

export async function apiGetFundLocation(token: string, id: string): Promise<FundLocation> {
  const res = await authenticatedApiRequest(BASE + `/fund-locations/${encodeURIComponent(id)}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundLocation(resData)
}

export async function apiUpdateFundLocation(
  token: string,
  id: string,
  body: UpdateFundLocationInput,
): Promise<FundLocation> {
  const res = await authenticatedApiRequest(BASE + `/fund-locations/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundLocation(resData)
}

export async function apiDeactivateFundLocation(token: string, id: string): Promise<FundLocation> {
  const res = await authenticatedApiRequest(BASE + `/fund-locations/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundLocation(resData)
}

export async function apiGetFundLocationHistory(
  token: string,
  id: string,
  params?: { page?: number; page_size?: number },
): Promise<{ data: LocationBalanceChange[]; pagination: any }> {
  const q = new URLSearchParams()
  if (params?.page != null) q.set('page', String(params.page))
  if (params?.page_size != null) q.set('page_size', String(params.page_size))
  const res = await authenticatedApiRequest(BASE + `/fund-locations/${encodeURIComponent(id)}/history` + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  if (!resData) throw new ApiError('unexpected', 'Response data missing')
  const items = resData.data !== undefined ? resData.data : (Array.isArray(resData) ? resData : [])
  const pagination = resData.pagination !== undefined ? resData.pagination : {}
  return { data: Array.isArray(items) ? items : [], pagination }
}

export async function apiSetOpeningBalance(
  token: string,
  id: string,
  body: { amount: string; notes?: string },
): Promise<FundLocation> {
  const res = await authenticatedApiRequest(BASE + `/fund-locations/${encodeURIComponent(id)}/opening-balance`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundLocation(resData)
}

export async function apiListFundTransfers(
  token: string,
  params?: { page?: number; page_size?: number },
): Promise<{ data: FundTransfer[]; pagination: any }> {
  const q = new URLSearchParams()
  if (params?.page != null) q.set('page', String(params.page))
  if (params?.page_size != null) q.set('page_size', String(params.page_size))
  const res = await authenticatedApiRequest(BASE + '/fund-transfers' + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  if (!resData) throw new ApiError('unexpected', 'Response data missing')
  const items = resData.data !== undefined ? resData.data : (Array.isArray(resData) ? resData : [])
  const pagination = resData.pagination !== undefined ? resData.pagination : {}
  return { data: Array.isArray(items) ? items : [], pagination }
}

export async function apiCreateFundTransfer(
  token: string,
  body: CreateFundTransferInput,
): Promise<FundTransfer> {
  const res = await authenticatedApiRequest(BASE + '/fund-transfers', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      'Idempotency-Key': `transfer-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundTransfer(resData)
}

export async function apiReverseFundTransfer(
  token: string,
  id: string,
  body: { reason: string },
): Promise<FundTransfer> {
  const res = await authenticatedApiRequest(BASE + `/fund-transfers/${encodeURIComponent(id)}/reverse`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackFundTransfer(resData)
}

export async function apiGetFundLocationReconciliation(token: string): Promise<ReconciliationReport> {
  const res = await authenticatedApiRequest(BASE + '/reports/fund-locations/reconciliation', {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const resData = await json<any>(res)
  return unpackReconciliation(resData)
}
