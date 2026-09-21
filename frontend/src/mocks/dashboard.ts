export interface FinancialPeriod {
  month: string
  income: number
  expense: number
}

export const financialData: FinancialPeriod[] = [
  { month: 'Apr', income: 3200000, expense: 1800000 },
  { month: 'Mei', income: 4100000, expense: 2300000 },
  { month: 'Jun', income: 3800000, expense: 2100000 },
  { month: 'Jul', income: 4600000, expense: 2800000 },
  { month: 'Agu', income: 3900000, expense: 2450000 },
  { month: 'Sep', income: 4250000, expense: 2175000 },
]

export const recentTransactions = [
  { id: 1, description: 'Iuran Bulanan', date: '2026-09-15', type: 'income', amount: 450000 },
  { id: 2, description: 'Kebersihan Jalan', date: '2026-09-14', type: 'expense', amount: 350000 },
  { id: 3, description: 'Kas Masuk', date: '2026-09-13', type: 'income', amount: 800000 },
  { id: 4, description: 'Perbaikan Fasilitas', date: '2026-09-12', type: 'expense', amount: 725000 },
  { id: 5, description: 'Iuran Bulanan', date: '2026-09-10', type: 'income', amount: 420000 },
]

export const iuranData = {
  paid: 42,
  total: 50,
  percentage: 84,
}

export const dashboard = {
  balance: 24750000,
  monthlyIncome: 4250000,
  monthlyExpense: 2175000,
}
