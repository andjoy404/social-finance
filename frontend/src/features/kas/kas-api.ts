import {
  apiListTransactions,
  apiGetTransaction,
  apiCreateTransaction,
  apiGetBalance,
  type ApiTransaction,
  type ApiTransactionPaginated,
  type ApiCreateTransactionInput,
} from '@/app/api'

export interface KasPeriodSummary {
  openingBalance: string
  totalIncome: string
  totalExpense: string
  netMovement: string
  closingBalance: string
  transactionCount: number
}

export async function fetchPeriodBalance(
  token: string,
  period?: string,
): Promise<KasPeriodSummary> {
  const res = await apiGetBalance(token, period ? { period } : undefined)
  return {
    openingBalance: res.opening_balance ?? '0.00',
    totalIncome: res.total_income ?? '0.00',
    totalExpense: res.total_expense ?? '0.00',
    netMovement: res.net_movement ?? '0.00',
    closingBalance: res.closing_balance ?? res.net_balance ?? '0.00',
    transactionCount: res.transaction_count ?? 0,
  }
}

export async function fetchKasTransactions(
  token: string,
  params?: {
    page?: number
    pageSize?: number
    period?: string
    type?: string
    status?: string
    categoryId?: string
    startDate?: string
    endDate?: string
  },
): Promise<ApiTransactionPaginated> {
  return apiListTransactions(token, {
    page: params?.page,
    page_size: params?.pageSize,
    period: params?.period,
    type: params?.type,
    status: params?.status,
    category_id: params?.categoryId,
    start_date: params?.startDate,
    end_date: params?.endDate,
  })
}

export {
  apiGetTransaction,
  apiCreateTransaction,
  type ApiTransaction,
  type ApiTransactionPaginated,
  type ApiCreateTransactionInput,
}

