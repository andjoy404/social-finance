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
  system_role?: string
  role: string
  rt: { id: string; name: string } | null
}

export interface ApiRefreshResponse {
  access_token: string
  refresh_token: string
}

export interface ApiRT {
  id: string
  name: string
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
  details?: string[]
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

export async function apiLogin(email: string, password: string): Promise<ApiAuthResponse> {
  const res = await fetch(BASE + '/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
  })
  const data = await json<ApiAuthResponse>(res)
  if (!res.ok || !data) {
    const body = await json<ApiError>(res).catch(() => null)
    throw new ApiError(body?.code ?? 'unexpected', body?.message ?? 'Login failed')
  }
  return data
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
  let currentRes = await fetch(url, opts)
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
      ...(opts.headers as Record<string, string> || {}),
    }
    retryHeaders['Authorization'] = `Bearer ${token}`
    const retryOpts: RequestInit = { ...opts, headers: retryHeaders as HeadersInit }
    currentRes = await fetch(url, retryOpts)
  }

  if (!currentRes.ok) {
    const body = await json<ApiError>(currentRes).catch(() => null)
    if (currentRes.status >= 400 && body?.code) {
      throw new ApiError(body.code, body.message)
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
  params: { page?: number; page_size?: number; is_active?: boolean; search?: string },
): Promise<ApiRTPaginated> {
  const q = new URLSearchParams()
  if (params.page != null) q.set('page', String(params.page))
  if (params.page_size != null) q.set('page_size', String(params.page_size))
  if (params.search != null) q.set('search', params.search)
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
  rw: number
  rt: string
  address?: string | null
  head_name?: string | null
}

export interface ApiUpdateRTBody {
  name?: string
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

  constructor(code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
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
  rt_id?: string
  is_active?: boolean
}

export interface ApiUpdateHouseholdBody {
  house_number?: string
  head_name?: string
  nik?: string
  phone?: string
  email?: string
  address?: string | null
  occupancy_status?: 'OWNER' | 'TENANT'
  is_active?: boolean
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
  relationship_to_head: string | null
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface ApiCreateResidentBody {
  household_id: string
  full_name: string
  nik: string
  phone?: string | null
  email?: string | null
  relationship_to_head?: string | null
  start_date?: string
}

export interface ApiUpdateResidentBody {
  full_name?: string
  nik?: string
  phone?: string | null
  email?: string | null
  relationship_to_head?: string | null
  is_active?: boolean
}

export interface ApiMoveResidentBody {
  destination_household_id: string
  start_date: string
  relationship_to_head?: string | null
}

export interface ApiListResidentsParams {
  page?: number
  page_size?: number
  is_active?: boolean
  search?: string
  household_id?: string
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
    ...(typeof AbortSignal !== 'undefined' ? { signal: AbortSignal.timeout(TIMEOUT_MS) } : {}),
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
