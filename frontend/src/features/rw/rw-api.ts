/**
 * API client untuk endpoint PR 4 — RW-scoped aggregated data.
 *
 * Semua endpoint mengembalikan response berbentuk { "data": {...} }.
 * Response STRUKTURNYA BERBEDA dengan API RT, jadi type-nya terpisah.
 *
 * Endpoints:
 *   GET /api/v1/rw/{rwId}/dashboard
 *   GET /api/v1/rw/{rwId}/kas?period=YYYY-MM
 *   GET /api/v1/rw/{rwId}/kas/transactions
 *   GET /api/v1/rw/{rwId}/locations?is_active=true
 */

// Re-export dari api.ts — dibutuhkan untuk fetch calls
import type { ApiTransaction } from '@/app/api'

const BASE = '/api/v1'

// ---------------------------------------------------------------------------
// Models — response dari PR 4 backend
// ---------------------------------------------------------------------------

/**
 * Dashboard summary dari seluruh RT di bawah RW.
 * STRUKTUR BERBEDA dari ApiDashboardSummary (RT):
 *   - TIDAK ada chart, dues_progress, payment_method_breakdown
 *   - Flat aggregate: total_rts, total_locations, total_balance, dll
 */
export interface ApiRWDashboardSummary {
  total_rts: number
  total_locations: number
  total_balance: string
  total_income: string
  total_expense: string
  transaction_count: number
}

/** KAS report aggregate dari seluruh RT di bawah RW. */
export interface ApiRWKASReport {
  total_income: string
  total_expense: string
  net_balance: string
  transaction_count: number
}

/** Satu transaksi dari RT tertentu di bawah RW. */
export interface ApiRWTransaction {
  id: string
  rt_id: string
  rt_name: string
  type: string // 'income' | 'expense'
  amount: string
  description: string
  category_id?: string | null
  category_name?: string | null
  location_id?: string | null
  location_name?: string | null
  source_rt_id?: string | null
  source_rt_name?: string | null
  status: string
  payment_id?: string | null
  occurred_at: string
  created_at: string
}

/** Pagination wrapper untuk transaction list. */
export interface ApiRWTxPaginated {
  data: ApiRWTransaction[]
  pagination: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/** Satu lokasi dana dari RT tertentu di bawah RW. */
export interface ApiRWLocation {
  id: string
  rt_id: string
  rt_name: string
  name: string
  category: string
  bank_name?: string | null
  account_number?: string | null
  account_holder?: string | null
  is_active: boolean
  current_balance: string
  created_at: string
}

export interface ApiRWFundTransfer {
  id: string
  rw_id: string
  from_location_id: string
  from_location_name?: string | null
  to_location_id: string
  to_location_name?: string | null
  amount: string
  transfer_date: string
  notes?: string | null
  created_by?: string | null
  reverses_transfer_id?: string | null
  is_reversal: boolean
  created_at: string
}

export interface ApiRWLocationMutation {
  id: string
  rw_id: string
  location_id: string
  transaction_id?: string | null
  transfer_id?: string | null
  change_type: string
  amount_delta: string
  notes?: string | null
  created_by?: string | null
  created_at: string
}

export interface ApiRWCategorySummary {
  category: string
  total_balance: string
  location_count: number
}

export interface ApiRWReconciliation {
  opening_balance: string
  total_income: string
  total_expense: string
  calculated_balance: string
  total_location_balances: string
  is_reconciled: boolean
  discrepancy: string
  category_breakdown: ApiRWCategorySummary[]
}

// ---------------------------------------------------------------------------
// API Functions
// ---------------------------------------------------------------------------

/**
 * Helper: authenticated fetch menggunakan session token.
 * Menggunakan pattern yang sama dengan api.ts internal fetch calls.
 */
async function authenticatedFetch(
  url: string,
  token: string,
  init?: RequestInit,
): Promise<Response> {
  const res = await fetch(url, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...init?.headers,
    },
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`API error ${res.status}: ${text}`)
  }
  return res
}

/**
 * GET /api/v1/rw/{rwId}/dashboard
 *
 * Returns aggregated financial overview for all RTs in the RW.
 */
export async function apiGetRWDashboard(
  token: string,
  rwId: string,
  period?: string,
): Promise<ApiRWDashboardSummary> {
  let url = `${BASE}/rw/${encodeURIComponent(rwId)}/dashboard`
  if (period) url += `?period=${encodeURIComponent(period)}`

  const res = await authenticatedFetch(url, token)
  const body = await res.json()
  if (!body?.data) throw new Error('RW dashboard response is empty')
  return body.data as ApiRWDashboardSummary
}

/**
 * GET /api/v1/rw/{rwId}/kas?period=YYYY-MM
 *
 * Returns KAS report (income/expense aggregate) for the RW.
 */
export async function apiGetRWKAS(
  token: string,
  rwId: string,
  period?: string,
): Promise<ApiRWKASReport> {
  let url = `${BASE}/rw/${encodeURIComponent(rwId)}/kas`
  if (period) url += `?period=${encodeURIComponent(period)}`

  const res = await authenticatedFetch(url, token)
  const body = await res.json()
  if (!body?.data) throw new Error('RW KAS response is empty')
  return body.data as ApiRWKASReport
}

/**
 * GET /api/v1/rw/{rwId}/kas/transactions
 *
 * Returns paginated list of posted transactions from all RTs in the RW.
 * Every entry includes rt_id and rt_name to identify the source RT.
 */
export async function apiListRWTransactions(
  token: string,
  rwId: string,
  params: {
    period?: string
    start_date?: string
    end_date?: string
    page?: number
    page_size?: number
  } = {},
): Promise<ApiRWTxPaginated> {
  const q = new URLSearchParams()
  if (params.period) q.set('period', params.period)
  if (params.start_date) q.set('start_date', params.start_date)
  if (params.end_date) q.set('end_date', params.end_date)
  if (params.page) q.set('page', String(params.page))
  if (params.page_size) q.set('page_size', String(params.page_size))

  let url = `${BASE}/rw/${encodeURIComponent(rwId)}/kas/transactions`
  if (q.toString()) url += '?' + q.toString()

  const res = await authenticatedFetch(url, token)
  const body = await res.json()
  if (!body?.data) return { data: [], pagination: { page: 1, per_page: 20, total: 0, total_pages: 0 } }
  return {
    data: body.data as ApiRWTransaction[],
    pagination: (body.pagination as { page: number; per_page: number; total: number; total_pages: number }) ?? { page: 1, per_page: 20, total: 0, total_pages: 0 },
  }
}

/**
 * GET /api/v1/rw/{rwId}/locations?is_active=true
 *
 * Returns fund locations from all RTs in the RW.
 * Read-only: every entry includes rt_id and rt_name.
 */
export async function apiListRWLocations(
  token: string,
  rwId: string,
  params?: {
    category?: string
    is_active?: boolean
  },
): Promise<ApiRWLocation[]> {
  const q = new URLSearchParams()
  if (params?.category) q.set('category', params.category)
  if (params?.is_active !== undefined) q.set('is_active', String(params.is_active))

  let url = `${BASE}/rw/${encodeURIComponent(rwId)}/locations`
  if (q.toString()) url += '?' + q.toString()

  const res = await authenticatedFetch(url, token)
  const body = await res.json()
  if (!body?.data) return []
  return body.data as ApiRWLocation[]
}

/** POST /api/v1/rw/{rwId}/locations */
export async function apiCreateRWLocation(
  token: string,
  rwId: string,
  data: {
    name: string
    category: 'rekening' | 'brankas' | 'coh'
    bank_name?: string
    account_number?: string
    account_holder?: string
  },
): Promise<ApiRWLocation> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/locations`
  const res = await authenticatedFetch(url, token, {
    method: 'POST',
    body: JSON.stringify(data),
  })
  const body = await res.json()
  return body.data as ApiRWLocation
}

/** PUT /api/v1/rw/{rwId}/locations/{id} */
export async function apiUpdateRWLocation(
  token: string,
  rwId: string,
  locationId: string,
  data: {
    name: string
    bank_name?: string
    account_number?: string
    account_holder?: string
  },
): Promise<ApiRWLocation> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/locations/${encodeURIComponent(locationId)}`
  const res = await authenticatedFetch(url, token, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
  const body = await res.json()
  return body.data as ApiRWLocation
}

/** PATCH /api/v1/rw/{rwId}/locations/{id}/status */
export async function apiToggleRWLocationStatus(
  token: string,
  rwId: string,
  locationId: string,
  isActive: boolean,
): Promise<ApiRWLocation> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/locations/${encodeURIComponent(locationId)}/status`
  const res = await authenticatedFetch(url, token, {
    method: 'PATCH',
    body: JSON.stringify({ is_active: isActive }),
  })
  const body = await res.json()
  return body.data as ApiRWLocation
}

/** GET /api/v1/rw/{rwId}/locations/reconcile */
export async function apiGetRWReconciliation(
  token: string,
  rwId: string,
): Promise<ApiRWReconciliation> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/locations/reconcile`
  const res = await authenticatedFetch(url, token)
  const body = await res.json()
  return body.data as ApiRWReconciliation
}

/** GET /api/v1/rw/{rwId}/locations/{id}/mutations */
export async function apiGetRWLocationMutations(
  token: string,
  rwId: string,
  locationId: string,
  limit = 50,
): Promise<ApiRWLocationMutation[]> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/locations/${encodeURIComponent(locationId)}/mutations?limit=${limit}`
  const res = await authenticatedFetch(url, token)
  const body = await res.json()
  return (body.data as ApiRWLocationMutation[]) || []
}

/** GET /api/v1/rw/{rwId}/transfers */
export async function apiListRWTransfers(
  token: string,
  rwId: string,
  page = 1,
  pageSize = 20,
): Promise<ApiRWFundTransfer[]> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/transfers?page=${page}&page_size=${pageSize}`
  const res = await authenticatedFetch(url, token)
  const body = await res.json()
  return (body.data as ApiRWFundTransfer[]) || []
}

/** POST /api/v1/rw/{rwId}/transfers */
export async function apiCreateRWTransfer(
  token: string,
  rwId: string,
  data: {
    from_location_id: string
    to_location_id: string
    amount: string
    transfer_date: string
    notes?: string
  },
): Promise<ApiRWFundTransfer> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/transfers`
  const res = await authenticatedFetch(url, token, {
    method: 'POST',
    body: JSON.stringify(data),
  })
  const body = await res.json()
  return body.data as ApiRWFundTransfer
}

/** POST /api/v1/rw/{rwId}/transfers/{id}/reverse */
export async function apiReverseRWTransfer(
  token: string,
  rwId: string,
  transferId: string,
  reason: string,
): Promise<ApiRWFundTransfer> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/transfers/${encodeURIComponent(transferId)}/reverse`
  const res = await authenticatedFetch(url, token, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  })
  const body = await res.json()
  return body.data as ApiRWFundTransfer
}

/** POST /api/v1/rw/{rwId}/kas/transactions */
export async function apiCreateRWTransaction(
  token: string,
  rwId: string,
  data: {
    type: 'income' | 'expense'
    amount: string
    location_id: string
    description: string
    payment_method: 'transfer' | 'cash'
    source_rt_id?: string
  },
): Promise<ApiRWTransaction> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/kas/transactions`
  const res = await authenticatedFetch(url, token, {
    method: 'POST',
    body: JSON.stringify(data),
  })
  const body = await res.json()
  return body.data as ApiRWTransaction
}

// ---------------------------------------------------------------------------
// Adapter: Map RW dashboard/KAS data into RT-compatible formats
// ---------------------------------------------------------------------------

/**
 * The PR 4 RW dashboard response is flat — no chart, no dues_progress,
 * no payment_method_breakdown. This adapter fills in sensible defaults
 * so the existing Dashboard UI can render without changes.
 */
export function mapRWDashboardToRTFormat(
  rwData: ApiRWDashboardSummary,
): {
  balance: string
  monthly_income: string
  monthly_expense: string
  chart: Array<{ month: string; income: string; expense: string }>
  dues_progress: { paid_households: number; total_households: number; percentage: number }
  payment_method_breakdown: {
    transfer_amount: string
    cash_amount: string
    transfer_count: number
    cash_count: number
  }
  recent_transactions: ApiTransaction[]
} {
  return {
    balance: rwData.total_balance || '0.00',
    monthly_income: rwData.total_income || '0.00',
    monthly_expense: rwData.total_expense || '0.00',
    chart: [],
    dues_progress: {
      paid_households: 0,
      total_households: 0,
      percentage: 0,
    },
    payment_method_breakdown: {
      transfer_amount: '0.00',
      cash_amount: '0.00',
      transfer_count: 0,
      cash_count: 0,
    },
    recent_transactions: [],
  }
}
